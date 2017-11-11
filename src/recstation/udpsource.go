package recstation

import (
	"log"
	"net"

	"mpeg"

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

	ListenError  chan error
	RxBufReady   chan *RecvBuf
	RxBufPending chan *RecvBuf
	RecvPackets  chan *RecvPacket
	leaveGroup   chan net.IP
	addSink      chan addSinkMsg
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

func MakeUdpSource(iface *net.Interface, listenAddr string) (*UdpSource, error) {

	source := &UdpSource{
		Iface:        iface,
		ListenError:  make(chan error),
		RxBufReady:   make(chan *RecvBuf, NUM_INFLIGHT_PACKETS),
		RxBufPending: make(chan *RecvBuf, NUM_INFLIGHT_PACKETS),
		leaveGroup:   make(chan net.IP),
		addSink:      make(chan addSinkMsg),
		SinkMap:      make(map[uint32]*Sink),
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

	//nrecvs := NUM_INFLIGHT_PACKETS

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
				}
			}

			rx.Stop = false
			source.RxBufReady <- rx

		}
	}
}

/*
	cfg.Iface, err = net.InterfaceByName(cfg.IfName)
	if err != nil {
		panic(err)
	}

	cfg.Conn, err = net.ListenPacket("udp4", cfg.ListenAddr)
	if err != nil {
		panic(err)
	}
	defer cfg.Conn.Close()

	cfg.V4Conn = ipv4.NewPacketConn(cfg.Conn)

	for _, groupAddr := range groups {
		group := net.ParseIP(groupAddr)
		cfg.Groups = append(cfg.Groups, group)
	}

	if err := SendPeriodicIgmpMembershipReports(cfg.Iface, cfg.Groups); err != nil {
		panic(err)
	}

	for _, group := range cfg.Groups {
		if err := cfg.V4Conn.JoinGroup(cfg.Iface, &net.UDPAddr{IP: group}); err != nil {
			panic(err)
		}
	}

*/

/*
func (source *UdpSource) ListenLoop() {
	nrecvs := 2048
	eachbuflen := 2048
	masterbuf := make([]byte, nrecvs*eachbuflen)
	npkts := 10

	recvs := make([]*RecvPacket, nrecvs)
	offs := 0

	for i := 0; i < nrecvs; i++ {
		recvs[i] = &RecvBuf{
			Buf:  masterbuf[offs : offs+eachbuflen],
			Pkts: make([]mpeg.TsBuffer, 0, npkts),
		}

		offs += eachbuflen
	}

	oob := make([]byte, 1024)

	for i := 0; ; i = (i + 1) % nrecvs {
		recv := recvs[i]

		var n, oobn, flags int
		var src *net.UDPAddr

		for {
			var err error
			n, oobn, flags, src, err = source.UdpConn.ReadMsgUDP(recv.Buf, oob)

			if err != nil {
				panic(err)
				source.ListenError <- err
				continue
			}

			if n == 0 {
				continue
			}

			break
		}

		if flags != 0 || oobn != 0 || oob[0] != 0 {
		}

		recv.Pkts = recv.Pkts[:0]

		for offs := 0; offs < n; offs += mpeg.TS_PACKET_LENGTH {
			pkt := mpeg.TsBuffer(recv.Buf[offs:(offs + mpeg.TS_PACKET_LENGTH)])

			if pkt.IsValid() && pkt.GetPid() != mpeg.PID_PADDING {
				recv.Pkts = append(recv.Pkts, pkt)
			}
		}

		if len(recv.Pkts) == 0 {
			continue
		}

		var cm ipv4.ControlMessage
		err := cm.Parse(oob[:oobn])
		if err != nil {
			panic(err)
			continue
		}

		recv.Src = src.IP.To4()
		recv.Dst = cm.Dst.To4()

		source.RecvPackets <- recv
	}
}
*/

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
