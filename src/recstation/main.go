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
	iface := flag.String("iface", "", "Interface name")

	flag.Parse()

	if iface == nil {
		Usage()
		os.Exit(1)
	}

	cfg := Config{
		IfName:     *iface,
		ListenAddr: "0.0.0.0:6000",
	}

	var err error

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

	for _, group := range cfg.Groups {
		if err := cfg.V4Conn.JoinGroup(cfg.Iface, &net.UDPAddr{IP: group}); err != nil {
			panic(err)
		}
	}

	if err := cfg.V4Conn.SetControlMessage(ipv4.FlagDst, true); err != nil {
		panic(err)
	}

	events := make(chan HeartbeatEvent)

	timeout := 3 * time.Second
	go RunHeartbeat(cfg.V4Conn, timeout, events)

	for {
		ev := <-events

		switch ev.Event {
		case HEARTBEAT_ONLINE:
			log.Printf("Online %s => %s", ev.Src, ev.Dst)

		case HEARTBEAT_OFFLINE:
			log.Printf("OFFLINE %s => %s", ev.Src, ev.Dst)
		}
	}
}
