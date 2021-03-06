package recstation

import (
	"log"
	"net"
	"time"

	"golang.org/x/net/ipv4"
)

type Heartbeat struct {
	Conn    net.PacketConn
	V4Conn  *ipv4.PacketConn
	Events  chan HeartbeatEvent
	Timeout time.Duration
}

const (
	HEARTBEAT_ONLINE = iota
	HEARTBEAT_OFFLINE
)

type HeartbeatEvent struct {
	Event int
	Src   net.IP
	Dst   net.IP
}

const HEARTBEAT_PKTLEN = 14

type listenMessage struct {
	src net.IP
	dst net.IP
}

const (
	WATCHDOG_HEARTBEAT = iota
	WATCHDOG_TIMEOUT
	WATCHDOG_STOP
)

type activeNode struct {
	src net.IP
	dst net.IP

	control chan int
}

func (node *activeNode) watchdog(timeout time.Duration, stop chan<- *activeNode) {
	running := true
	stopping := false
	tick := time.NewTimer(timeout)

	for running {
		select {
		case <-tick.C:
			stopping = true
			stop <- node

		case m := <-node.control:
			switch m {
			case WATCHDOG_HEARTBEAT:
				if !stopping {
					if !tick.Stop() {
						<-tick.C
					}
					tick.Reset(timeout)
				}

			case WATCHDOG_STOP:
				running = false
			}
		}
	}
}

func listenLoop(conn *ipv4.PacketConn, msg chan<- listenMessage) error {
	buf := make([]byte, 2048)

	for {
		n, cm, _, err := conn.ReadFrom(buf)

		if err != nil {
			return err
		}

		if n != HEARTBEAT_PKTLEN {
			continue
		}

		// TODO FIXME: Parse content of heartbeat packet

		msg <- listenMessage{
			src: cm.Src,
			dst: cm.Dst,
		}
	}
}

func RunHeartbeat(conn *ipv4.PacketConn, timeout time.Duration, events chan<- HeartbeatEvent) {

	live := make(map[uint32]*activeNode)
	stop := make(chan *activeNode)
	incoming := make(chan listenMessage)

	go (func() {
		err := listenLoop(conn, incoming)

		log.Printf("Listen loop failed: %s", err)
	})()

	for {
		select {
		case msg := <-incoming:
			key := IPtoU32(msg.src)

			if node, found := live[key]; found {
				node.control <- WATCHDOG_HEARTBEAT
			} else {
				node := &activeNode{
					src:     msg.src,
					dst:     msg.dst,
					control: make(chan int),
				}

				events <- HeartbeatEvent{
					Event: HEARTBEAT_ONLINE,
					Src:   node.src,
					Dst:   node.dst,
				}

				live[key] = node

				go node.watchdog(timeout, stop)
			}

		case node := <-stop:
			delete(live, IPtoU32(node.src))

			node.control <- WATCHDOG_STOP

			events <- HeartbeatEvent{
				Event: HEARTBEAT_OFFLINE,
				Src:   node.src,
				Dst:   node.dst,
			}
		}
	}
}

func MakeHeartbeat(iface *net.Interface, listenAddr string, timeout time.Duration, groupAddrs []net.IP) (*Heartbeat, error) {
	conn, err := net.ListenPacket("udp4", listenAddr)
	if err != nil {
		return nil, err
	}

	v4conn := ipv4.NewPacketConn(conn)

	for _, group := range groupAddrs {
		if err := v4conn.JoinGroup(iface, &net.UDPAddr{IP: group}); err != nil {
			return nil, err
		}
	}

	if err := v4conn.SetControlMessage(ipv4.FlagDst, true); err != nil {
		return nil, err
	}

	heartbeat := &Heartbeat{
		Conn:    conn,
		V4Conn:  v4conn,
		Events:  make(chan HeartbeatEvent),
		Timeout: timeout,
	}

	go RunHeartbeat(heartbeat.V4Conn, heartbeat.Timeout, heartbeat.Events)

	return heartbeat, nil
}
