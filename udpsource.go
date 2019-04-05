package recstation

import (
	"log"
	"net"

	"recstation/mpeg"

	"github.com/google/vectorio"
	"golang.org/x/net/ipv4"
)

const (
	NUM_INFLIGHT_PACKETS = 2048
	NUM_TS_PER_PACKET    = 10
)

type RecvPacket struct {
	Buf []byte

	Src net.IP
	Dst net.IP
}

type RecvBuf struct {
	Stop   bool
	Err    error
	Flags  int
	RawOob [1024]byte
	RawBuf [2048]byte
	Oob    []byte
	Buf    []byte
	Src    net.IP
	Dst    net.IP
	Pkts   []mpeg.TsBuffer
}

type UdpSource struct {
	Iface   *net.Interface
	UdpConn *net.UDPConn
	PktConn *ipv4.PacketConn

	SinkMap map[uint32]*Sink

	ListenError       chan error
	RxBufReady        chan *RecvBuf
	RxBufPending      chan *RecvBuf
	RecvPackets       chan *RecvPacket
	leaveGroup        chan net.IP
	addSink           chan addSinkMsg
	removeSinkRequest chan net.IP
}

type addSinkMsg struct {
	Group net.IP
	Sink  *Sink
}

func (source *UdpSource) AddSink(group net.IP, sink *Sink) {
	source.addSink <- addSinkMsg{
		Group: group,
		Sink:  sink,
	}
}

func (source *UdpSource) RemoveSink(group net.IP) {
	source.removeSinkRequest <- group
}

func MakeUdpSource(iface *net.Interface, listenAddr string) (*UdpSource, error) {
	source := &UdpSource{
		Iface:             iface,
		ListenError:       make(chan error),
		RxBufReady:        make(chan *RecvBuf, NUM_INFLIGHT_PACKETS),
		RxBufPending:      make(chan *RecvBuf, NUM_INFLIGHT_PACKETS),
		leaveGroup:        make(chan net.IP),
		addSink:           make(chan addSinkMsg),
		removeSinkRequest: make(chan net.IP),
		SinkMap:           make(map[uint32]*Sink),
	}

	laddr, err := net.ResolveUDPAddr("udp4", listenAddr)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		return nil, err
	}

	source.UdpConn = conn

	source.PktConn = ipv4.NewPacketConn(source.UdpConn)

	if err := source.PktConn.SetControlMessage(ipv4.FlagDst, true); err != nil {
		return nil, err
	}

	for i := 0; i < NUM_INFLIGHT_PACKETS; i++ {
		source.RxBufReady <- &RecvBuf{
			Pkts: make([]mpeg.TsBuffer, 0, NUM_TS_PER_PACKET),
		}
	}

	go source.RunLoop()
	go source.RecvLoop(source.UdpConn, source.RxBufReady, source.RxBufPending)

	return source, nil
}

func (source *UdpSource) RunLoop() {
	running := true

	npkts := 0
	for running {
		select {
		case msg := <-source.addSink:
			log.Printf("Adding %s", msg.Group)

			if err := source.PktConn.JoinGroup(source.Iface, &net.UDPAddr{IP: msg.Group}); err != nil {
				panic(err)
			}

			key := IPtoU32(msg.Group)
			source.SinkMap[key] = msg.Sink

		case addr := <-source.removeSinkRequest:
			log.Printf("Removing sink for %s", addr)

			key := IPtoU32(addr)
			delete(source.SinkMap, key)

		case group := <-source.leaveGroup:
			if err := source.PktConn.LeaveGroup(source.Iface, &net.UDPAddr{IP: group}); err != nil {
				panic(err)
			}

		case err := <-source.ListenError:
			panic(err)

		case rx := <-source.RxBufPending:
			n := len(rx.Buf)
			rx.Pkts = rx.Pkts[:0]

			for offs := 0; offs < n; offs += mpeg.TS_PACKET_LENGTH {
				pkt := mpeg.TsBuffer(rx.Buf[offs:(offs + mpeg.TS_PACKET_LENGTH)])

				if pkt.IsValid() && pkt.GetPid() != mpeg.PID_PADDING {
					rx.Pkts = append(rx.Pkts, pkt)
				}
			}

			if len(rx.Pkts) > 0 {
				npkts += len(rx.Pkts)

				key := IPtoU32(rx.Dst)

				if sink, found := source.SinkMap[key]; found {
					//log.Printf("%s => %s: n=%d", rp.Src, rp.Dst, len(rp.Pkts))
					sink.Packets <- rx.Pkts

					if sink.Preview != nil && sink.Preview.Input != nil {
						var multiple [10][]byte

						npkts := len(rx.Pkts)
						nbytes := 0
						for i, pkt := range rx.Pkts {
							multiple[i] = pkt[:mpeg.TS_PACKET_LENGTH]
							nbytes += len(multiple[i])

							if multiple[i][0] != 'G' {
								log.Printf("bad TS preview")
							}
						}

						n, err := vectorio.Writev(sink.Preview.Input, multiple[:npkts])
						if err != nil {
							log.Printf("Failed to write into preview (%d bytes): %s", n, err)
							sink.Preview.Input = nil
						}

						if n != nbytes {
							log.Printf("Bad number of preview bytes: %d vs %d", n, nbytes)
						}
					}
				}
			}

			rx.Stop = false
			source.RxBufReady <- rx
		}
	}
}

func (_ *UdpSource) RecvLoop(conn *net.UDPConn, source chan *RecvBuf, sink chan *RecvBuf) {
	for {
		rx := <-source

		if rx.Stop {
			break
		}

		var n, oobn int
		var src *net.UDPAddr

		for n == 0 {
			n, oobn, rx.Flags, src, rx.Err = conn.ReadMsgUDP(rx.RawBuf[:], rx.RawOob[:])
		}

		if rx.Err == nil {
			rx.Oob = rx.RawOob[:oobn]
			rx.Buf = rx.RawBuf[:n]
			rx.Src = src.IP.To4()

			var cm ipv4.ControlMessage

			if err := cm.Parse(rx.Oob); err != nil {
				rx.Err = err
			} else {
				rx.Dst = cm.Dst.To4()
			}
		}

		sink <- rx
	}
}
