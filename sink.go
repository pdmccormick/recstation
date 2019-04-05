package recstation

import (
	"log"
	"os"
	"path"
	"time"

	"recstation/mpeg"

	"github.com/google/vectorio"
)

type Sink struct {
	Name    string
	Running bool

	pat     mpeg.TsFrame
	pmt     mpeg.TsFrame
	pmt_pid mpeg.PID

	File     *os.File
	Filename string
	Namer    func(start bool) string

	Preview *Preview

	StopRequest     chan bool
	OfflineRequest  chan bool
	OpenFileRequest chan bool
	Packets         chan []mpeg.TsBuffer
	rawWrites       chan sinkRawWrite
	StatusRequest   chan chan *SinkStatusMessage
}

type SinkStatusMessage struct {
	Name              string `json:"name"`
	Running           bool   `json:"running"`
	BytesIn           uint64 `json:"bytes_in"`
	BytesInPerSecond  uint64 `json:"bytes_in_per_second"`
	BytesOut          uint64 `json:"bytes_out"`
	BytesOutPerSecond uint64 `json:"bytes_out_per_second"`
}

type SinkStatusMessage_ByName []*SinkStatusMessage

func (a SinkStatusMessage_ByName) Len() int           { return len(a) }
func (a SinkStatusMessage_ByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a SinkStatusMessage_ByName) Less(i, j int) bool { return a[i].Name < a[j].Name }

type sinkRawWrite struct {
	Buf  []byte
	Done chan bool
}

func MakeSink(name string, namer func(start bool) string) *Sink {
	sink := &Sink{
		Name:            name,
		Namer:           namer,
		StopRequest:     make(chan bool),
		OfflineRequest:  make(chan bool),
		OpenFileRequest: make(chan bool),
		Packets:         make(chan []mpeg.TsBuffer),
		rawWrites:       make(chan sinkRawWrite),
		StatusRequest:   make(chan chan *SinkStatusMessage),
	}

	go sink.Runloop()

	return sink
}

func (sink *Sink) RawWrite(buf []byte, done chan bool) {
	sink.rawWrites <- sinkRawWrite{
		Buf:  buf,
		Done: done,
	}
}

func ensureExists(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}

	if os.IsNotExist(err) {
		return os.MkdirAll(path, os.ModePerm)
	}

	return nil
}

func (sink *Sink) closeFile() bool {
	if sink.File == nil {
		return false
	}

	log.Printf("Closing %s file %s", sink.Name, sink.Filename)

	sink.File.Close()
	sink.File = nil

	fi, err := os.Stat(sink.Filename)
	if err == nil {
		if fi.Size() == 0 {
			os.Remove(sink.Filename)
		}
	}

	sink.Filename = ""

	return true
}

func (sink *Sink) openFile(filename string) (bool, error) {
	if sink.File != nil || filename == "" {
		return false, nil
	}

	log.Printf("Opening %s file %s", sink.Name, filename)

	dirname := path.Dir(filename)
	if err := ensureExists(dirname); err != nil {
		log.Printf("Unable to create directory %s: %s", dirname, err)
		return false, err
	}

	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Print("Unable to open: ", err)
		return false, err
	}

	sink.File = f
	sink.Filename = filename

	return true, nil
}

func (sink *Sink) Runloop() {
	online := true
	multiple := make([][]byte, 10)

	bytes_in := uint64(0)
	last_bytes_in := uint64(0)
	bytes_in_per := uint64(0)

	bytes_out := uint64(0)
	last_bytes_out := uint64(0)
	bytes_out_per := uint64(0)

	ticker := time.NewTicker(1 * time.Second)

	for online {
		select {
		case <-sink.StopRequest:
			sink.closeFile()

			sink.Running = false
			bytes_out = 0
			last_bytes_out = 0
			bytes_out_per = 0

		case <-sink.OfflineRequest:
			log.Printf("Sink '%s' going offline", sink.Name)

			sink.closeFile()

			sink.Running = false
			bytes_out = 0
			last_bytes_out = 0
			bytes_out_per = 0

			bytes_in = 0
			last_bytes_in = 0
			bytes_in_per = 0

			online = false

		case <-sink.OpenFileRequest:
			start := !sink.closeFile()

			filename := sink.Namer(start)

			ok, err := sink.openFile(filename)
			if err != nil {
				panic(err)
			}

			sink.Running = ok

		case msg := <-sink.rawWrites:
			bytes_in += uint64(len(msg.Buf))

			if sink.Running && sink.File != nil {
				n, err := sink.File.Write(msg.Buf)

				if err != nil {
					log.Printf("Error raw writing to sink (%d bytes): %s", n, err)
					sink.closeFile()
				} else {
					bytes_out += uint64(n)
				}
			}

			msg.Done <- true

		case pkts := <-sink.Packets:
			npkts := len(pkts)
			nbytes := 0
			for i, pkt := range pkts {
				multiple[i] = pkt[:mpeg.TS_PACKET_LENGTH]
				nbytes += len(multiple[i])

				pid := pkt.GetPid()
				if pid == mpeg.PID_PAT {
					pkt.ToFrame(&sink.pat)

					var pat mpeg.PAT
					if pat.ParsePAT(pkt) {
						for _, entry := range pat.Entry {
							pmt_pid := entry.ProgramMapPID

							if pmt_pid != sink.pmt_pid {
								sink.pmt_pid = pmt_pid

								log.Printf("Found PMT PID %v", pmt_pid)
							}

							break
						}
					}
				} else if sink.pmt_pid != 0 && pid == sink.pmt_pid {
					pkt.ToFrame(&sink.pmt)
				}
			}

			bytes_in += uint64(nbytes)

			if !sink.Running || sink.File == nil {
				continue
			}

			n, err := vectorio.Writev(sink.File, multiple[:npkts])
			if err != nil {
				panic(err)
			}

			if n != nbytes {
				log.Printf("nbytes=%d n=%d", nbytes, n)
			}

			bytes_out += uint64(n)

		case resp := <-sink.StatusRequest:
			resp <- &SinkStatusMessage{
				Name:              sink.Name,
				Running:           sink.Running,
				BytesIn:           bytes_in,
				BytesInPerSecond:  bytes_in_per,
				BytesOut:          bytes_out,
				BytesOutPerSecond: bytes_out_per,
			}

		case <-ticker.C:
			bytes_in_per = bytes_in - last_bytes_in
			last_bytes_in = bytes_in

			bytes_out_per = bytes_out - last_bytes_out
			last_bytes_out = bytes_out
		}
	}

	if sink.File != nil {
		sink.File.Close()
		sink.File = nil
	}
}
