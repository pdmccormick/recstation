package recstation

import (
	"io"
	"log"
	"os/exec"
	"strconv"
	"time"

	"recstation/mpeg"
)

const (
	AUDIO_RAW_BUF_SIZE = 100 * mpeg.TS_PACKET_LENGTH
)

const (
	CAPTURE_PROG = "arecord"
	ENCODE_PROG  = "ffmpeg"
)

const (
	AUDIO_EVENT_STARTUP  = 1
	AUDIO_EVENT_SHUTDOWN = 2
)

type AudioSource struct {
	Device      string
	NumChannels int
	Bitrate     int

	Output      io.Reader
	Capture     *exec.Cmd
	CaptureArgs []string
	Encode      *exec.Cmd
	EncodeArgs  []string

	RecvPacket chan AudioRecvPacket
	Exits      chan CmdExit
	Event      chan int

	Sink *Sink
}

type AudioRecvPacket struct {
	Buf         []byte
	Err         error
	KeepRunning chan bool
}

func MakeAudioSource(device string, num_channels, bitrate int) *AudioSource {
	source := &AudioSource{
		Device:      device,
		NumChannels: num_channels,
		Bitrate:     bitrate,
		CaptureArgs: []string{
			"--file-type=raw",
			"--format=S32_LE",
			"--channels=" + strconv.Itoa(num_channels),
			"--rate=" + strconv.Itoa(bitrate),
			"--device=" + device,
		},
		EncodeArgs: []string{
			"-f", "s32le",
			"-ar", strconv.Itoa(bitrate),
			"-ac", strconv.Itoa(num_channels),
			"-i", "/dev/stdin",
			"-f", "mpegts",
			"-strict", "-2",
			"-c:a", "aac",
			"-b:a", "2048k",
			"-",
		},
		Event: make(chan int),
	}

	go source.RunLoop()

	return source
}

func (source *AudioSource) RunLoop() {
	ticker := time.NewTicker(1 * time.Second)
	ticker.Stop()

	startup := make(chan bool)
	const BACKOFF_PERIOD = 3 * time.Second

	active := false

	go func() {
		startup <- true
	}()

	var buf [AUDIO_RAW_BUF_SIZE]byte
	wait := make(chan bool)

	for {
		select {
		case <-startup:
			if !active {
				ticker.Stop()
				ticker = time.NewTicker(1 * time.Second)

				log.Printf("Starting audio capture")
				source.start()
				active = true

				source.Event <- AUDIO_EVENT_STARTUP
			}

		case exit := <-source.Exits:
			switch exit.Cmd {
			case source.Capture:
				log.Printf("Audio capture died with error: %v", exit.Err)
				p := source.Encode.Process
				if p != nil {
					p.Kill()
				}

			case source.Encode:
				log.Printf("Audio encoder died with error: %v", exit.Err)
				p := source.Capture.Process
				if p != nil {
					p.Kill()
				}

			default:
				continue
			}

			active = false
			source.shutdown()
			source.Event <- AUDIO_EVENT_SHUTDOWN

			time.AfterFunc(BACKOFF_PERIOD, func() {
				startup <- true
			})

		case pkt := <-source.RecvPacket:
			if pkt.Err != nil {
				continue
			}

			if source.Sink != nil {
				n := len(pkt.Buf)
				copy(buf[:n], pkt.Buf[:n])
				source.Sink.RawWrite(buf[:n], wait)
				<-wait
			}

			pkt.KeepRunning <- true
		}
	}
}

func (source *AudioSource) start() error {
	source.Capture = exec.Command(CAPTURE_PROG, source.CaptureArgs...)
	source.Encode = exec.Command(ENCODE_PROG, source.EncodeArgs...)

	PipeCmds(source.Capture, source.Encode)

	output, err := source.Encode.StdoutPipe()
	if err != nil {
		return err
	}

	source.Output = output

	source.Exits = make(chan CmdExit)

	RunAndReportCmd(source.Capture, source.Exits)
	RunAndReportCmd(source.Encode, source.Exits)

	source.RecvPacket = make(chan AudioRecvPacket)

	go source.recvLoop(source.Output, source.RecvPacket)

	return nil
}

func (source *AudioSource) shutdown() {
	go func(exit chan CmdExit) {
		<-exit
	}(source.Exits)

	go func(recv chan AudioRecvPacket) {
		pkt := <-recv

		if pkt.Err != nil {
			//log.Printf("Receive loop shutdown with error: %s", pkt.Err)
			return
		}

		pkt.KeepRunning <- false
	}(source.RecvPacket)

	source.Exits = make(chan CmdExit)
	source.RecvPacket = make(chan AudioRecvPacket)
}

func (source *AudioSource) recvLoop(output io.Reader, recv chan AudioRecvPacket) {
	var raw [AUDIO_RAW_BUF_SIZE]byte

	start := 0

	keepRunning := make(chan bool)

	running := true

	for running {
		buf := raw[start:]
		n, err := output.Read(buf)

		if err != nil {
			recv <- AudioRecvPacket{
				Err: err,
			}

			break
		}

		buf = raw[:start+n]
		m := len(buf)
		end := start + m

		residual := m % mpeg.TS_PACKET_LENGTH

		if residual != 0 {
			buf = raw[:(end - residual)]
		}

		recv <- AudioRecvPacket{
			Buf:         buf,
			KeepRunning: keepRunning,
		}

		running = <-keepRunning

		if residual != 0 {
			tail := raw[(end - residual):end]

			copy(raw[:residual], tail)
			start = residual
		}
	}
}
