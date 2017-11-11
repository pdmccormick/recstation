package recstation

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"golang.org/x/net/ipv4"
)

// TODO FIXME Make multicast group addresses configurable
var groups = []string{
	"239.255.42.42",
	"239.255.43.43",
	"239.255.44.44",
	"239.255.45.45",
	"239.255.46.46",
	"239.255.47.47",
	"239.255.48.48",
	"239.255.49.49",
}

type Config struct {
	IfName     string
	Iface      *net.Interface
	Groups     []net.IP
	ListenAddr string
	Conn       net.PacketConn
	V4Conn     *ipv4.PacketConn
}

func Usage() {
	fmt.Fprint(os.Stderr, "Usage of ", os.Args[0], ":\n")
	flag.PrintDefaults()
	fmt.Fprint(os.Stderr, "\n")
}

func RunMain() {
	flag.Usage = Usage
	ifaceName := flag.String("iface", "", "Interface name")

	flag.Parse()

	if ifaceName == nil {
		Usage()
		os.Exit(1)
	}

	iface, err := net.InterfaceByName(*ifaceName)
	if err != nil {
		panic(err)
	}

	source, err := MakeUdpSource(iface, "0.0.0.0:5004")
	if err != nil {
		panic(err)
	}

	var groupAddrs []net.IP

	for _, groupAddr := range groups {
		group := net.ParseIP(groupAddr)
		groupAddrs = append(groupAddrs, group)
	}

	if err := SendPeriodicIgmpMembershipReports(iface, groupAddrs); err != nil {
		panic(err)
	}

	conn, err := net.ListenPacket("udp4", "0.0.0.0:6000")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	v4Conn := ipv4.NewPacketConn(conn)

	for _, group := range groupAddrs {
		if err := v4Conn.JoinGroup(iface, &net.UDPAddr{IP: group}); err != nil {
			panic(err)
		}
	}

	if err := v4Conn.SetControlMessage(ipv4.FlagDst, true); err != nil {
		panic(err)
	}

	events := make(chan HeartbeatEvent)

	timeout := 3 * time.Second
	_ = timeout
	go RunHeartbeat(v4Conn, timeout, events)

	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	MakeFilenameMaker := func(stream string) func() string {
		return func() string {
			now := time.Now()
			f := now.Format("20060102_150405.00000-0700")

			return fmt.Sprintf("%s-%s-%s.ts", hostname, stream, f)
		}
	}

	type MySink struct {
		Sink  *Sink
		Namer func() string
	}

	sinks := make(map[string]*MySink)

	const REOPEN_TIME = 1 * time.Minute
	reopen_tick := time.NewTicker(REOPEN_TIME)

	for {
		select {
		case ev := <-events:
			switch ev.Event {
			case HEARTBEAT_ONLINE:
				log.Printf("Online %s => %s", ev.Src, ev.Dst)

				key := ev.Dst.String()

				sink := MakeSink()

				mysink := &MySink{
					Sink:  sink,
					Namer: MakeFilenameMaker(key),
				}

				sink.OpenFile <- mysink.Namer
				source.AddSink(ev.Dst, sink)

				sinks[key] = mysink

			case HEARTBEAT_OFFLINE:
				log.Printf("OFFLINE %s => %s", ev.Src, ev.Dst)
				source.leaveGroup <- ev.Dst

				delete(sinks, ev.Dst.String())
			}

		case <-reopen_tick.C:
			for _, mysink := range sinks {
				mysink.Sink.OpenFile <- mysink.Namer
			}
		}

	}
}
