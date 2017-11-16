package recstation

import (
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
)

const (
	DECODE_PROG             = "ffmpeg"
	PIX_FMT                 = "bgr32"
	PREVIEW_BYTES_PER_PIXEL = 4
)

type Preview struct {
	Framerate int
	Width     int
	Height    int
	Pixbuf    []uint8
	PixbufLen int

	Input       *os.File
	Output      io.Reader
	Decoder     *exec.Cmd
	DecoderArgs []string
	Image       *image.RGBA

	NextPreviews []PreviewJpegRequest

	StartRequest chan bool
	Exits        chan CmdExit
	StopRequest  chan bool
	RecvBuf      chan PreviewRecvBuf
	JpegRequest  chan PreviewJpegRequest
}

type PreviewRecvBuf struct {
	Frame       []uint8
	Bufsize     int
	Err         error
	KeepRunning chan bool
}

type PreviewJpegRequest struct {
	Next   bool
	Writer io.Writer
	Ready  chan error
}

func MakePreview(framerate, width, height int) *Preview {
	buf_len := width * height * PREVIEW_BYTES_PER_PIXEL
	buf := make([]uint8, buf_len)

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	p := &Preview{
		Framerate: framerate,
		Width:     width,
		Height:    height,
		PixbufLen: len(img.Pix),
		Pixbuf:    buf,
		Image:     img,
		DecoderArgs: []string{
			"-i", "pipe:",
			"-r", strconv.Itoa(framerate),
			"-s", fmt.Sprintf("%dx%d", width, height),
			"-pix_fmt", PIX_FMT,
			"-f", "rawvideo",
			"pipe:",
		},
		StartRequest: make(chan bool),
		Exits:        make(chan CmdExit),
		StopRequest:  make(chan bool),
		RecvBuf:      make(chan PreviewRecvBuf),
		JpegRequest:  make(chan PreviewJpegRequest),
	}

	go p.RunLoop()

	p.StartRequest <- true

	return p
}

func (p *Preview) RunLoop() {
	running := true

	for running {
		select {
		case <-p.StartRequest:
			err := p.start()
			if err != nil {
				log.Printf("Error starting preview: %s", err)
			}

		case pix := <-p.RecvBuf:
			if pix.Err != nil {
				pix.KeepRunning <- false
				continue
			}

			copy(p.Image.Pix, pix.Frame)

			pix.KeepRunning <- true

			if len(p.NextPreviews) > 0 {
				for _, req := range p.NextPreviews {
					err := jpeg.Encode(req.Writer, p.Image, nil)

					go func(err error, ch chan error) {
						ch <- err
					}(err, req.Ready)
				}

				p.NextPreviews = []PreviewJpegRequest{}
			}

		case req := <-p.JpegRequest:
			if req.Next {
				p.NextPreviews = append(p.NextPreviews, req)
			} else {
				err := jpeg.Encode(req.Writer, p.Image, nil)
				req.Ready <- err
			}

		case exit := <-p.Exits:
			log.Printf("Preview decoder exited: %s", exit.Err)

			p.Input.Close()
			p.Input = nil
		}
	}
}

func (p *Preview) start() error {
	p.Decoder = exec.Command(DECODE_PROG, p.DecoderArgs...)

	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}

	p.Input = pw
	p.Decoder.Stdin = pr
	p.Decoder.Stderr = nil

	output, err := p.Decoder.StdoutPipe()
	if err != nil {
		return err
	}

	p.Output = output
	p.Exits = make(chan CmdExit)

	RunAndReportCmd(p.Decoder, p.Exits)

	p.RecvBuf = make(chan PreviewRecvBuf)
	go p.recvLoop(p.Output, p.PixbufLen, p.RecvBuf)

	return nil
}

func (p *Preview) recvLoop(output io.Reader, bufsize int, recv chan PreviewRecvBuf) {
	buf := make([]uint8, bufsize)
	keepRunning := make(chan bool)

	running := true
	for running {
		start := 0

		var err error

		for {
			var n int
			n, err = output.Read(buf[start:])

			if err != nil {
				break
			}

			start += n

			if start == bufsize {
				break
			}
		}

		send := PreviewRecvBuf{
			KeepRunning: keepRunning,
		}

		if err != nil {
			send.Err = err
		} else {
			send.Frame = buf
		}

		recv <- send

		running = <-keepRunning
	}
}
