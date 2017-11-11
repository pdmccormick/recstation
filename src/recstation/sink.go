package recstation

import (
	"log"
	"os"
	"path"

	"mpeg"

	"github.com/google/vectorio"
)

type Sink struct {
	pat     mpeg.TsFrame
	pmt     mpeg.TsFrame
	pmt_pid mpeg.PID

	File *os.File

	Stop     chan bool
	OpenFile chan func(start bool) string
	Packets  chan []mpeg.TsBuffer
}

func MakeSink() *Sink {
	sink := &Sink{
		OpenFile: make(chan func(start bool) string),
		Packets:  make(chan []mpeg.TsBuffer),
	}

	go sink.Runloop()

	return sink
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

func (sink *Sink) Runloop() {
	running := true
	multiple := make([][]byte, 10)

	for running {
		select {
		case <-sink.Stop:
			running = false

		case makeFilename := <-sink.OpenFile:
			start := true

			if sink.File != nil {
				sink.File.Close()
				sink.File = nil
				start = false
			}

			filename := makeFilename(start)
			log.Printf("Opening %s (start %v)", filename, start)

			if filename != "" {
				dirname := path.Dir(filename)
				if err := ensureExists(dirname); err != nil {
					log.Printf("Unable to create directory %s: %s", dirname, err)
					continue
				}

				f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
				if err != nil {
					panic(err)
				}

				sink.File = f
			}

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

			if sink.File == nil {
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
