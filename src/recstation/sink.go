package recstation

import (
	"log"
	"os"
	"path"

	"mpeg"

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
}

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

	for online {
		select {
		case <-sink.StopRequest:
			sink.closeFile()

			sink.Running = false

		case <-sink.OfflineRequest:
			log.Printf("Sink '%s' going offline", sink.Name)

			sink.closeFile()

			sink.Running = false

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
			if sink.Running && sink.File != nil {
				n, err := sink.File.Write(msg.Buf)
				if err != nil {
					log.Printf("Error raw writing to sink (%d bytes): %s", n, err)
					sink.closeFile()
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
		}
	}

	if sink.File != nil {
		sink.File.Close()
		sink.File = nil
	}
}
