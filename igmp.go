package recstation

import (
	"log"
	"net"
	"time"

	"golang.org/x/net/ipv4"
)

const (
	IGMP_V2_MEMBERSHIP_REPORT = 0x16
)

type IgmpMembership struct {
	Iface           *net.Interface
	Conn            *ipv4.RawConn
	GroupAddr       net.IP
	DestAddr        net.IP
	MaxResponseTime time.Duration
}

func (m *IgmpMembership) SendMembershipReport() error {
	const ONE_TENTH_SECOND = 100 * time.Millisecond

	maxResponseTime := uint8(m.MaxResponseTime / ONE_TENTH_SECOND)

	pkt := [8]byte{
		IGMP_V2_MEMBERSHIP_REPORT,
		maxResponseTime,
		0,
		0,
		m.GroupAddr[0],
		m.GroupAddr[1],
		m.GroupAddr[2],
		m.GroupAddr[3],
	}

	iph := &ipv4.Header{
		Version:  ipv4.Version,
		Len:      ipv4.HeaderLen,
		TOS:      0xc0, // DSCP CS6
		TotalLen: ipv4.HeaderLen + len(pkt),
		TTL:      10,
		Protocol: 2,
		Dst:      m.DestAddr.To4(),
	}

	checksum := ChecksumRfc1071(pkt[:], 0)
	pkt[2] = byte((checksum & 0xff00) >> 8)
	pkt[3] = byte(checksum & 0x00ff)

	cm := ipv4.ControlMessage{
		IfIndex: m.Iface.Index,
	}

	return m.Conn.WriteTo(iph, pkt[:], &cm)
}

func ChecksumRfc1071(buf []byte, checksum uint32) uint16 {
	l := len(buf) - 1

	for i := 0; i < l; i += 2 {
		checksum += (uint32(buf[i]) << 8) + uint32(buf[i+1])
	}

	if len(buf)%2 == 1 {
		checksum += uint32(buf[l]) << 8
	}

	for checksum > 0xffff {
		checksum = (checksum & 0xffff) + (checksum >> 16)
	}

	return ^uint16((checksum >> 16) + checksum)
}

func SendPeriodicIgmpMembershipReports(iface *net.Interface, groups []net.IP) error {
	maxResponseTime := 1 * time.Second
	groupOffset := 250 * time.Millisecond
	sendPeriod := 5 * time.Second

	l, err := net.ListenPacket("ip4:2", "0.0.0.0")
	if err != nil {
		return err
	}

	for i, group := range groups {
		c, err := ipv4.NewRawConn(l)
		if err != nil {
			return err
		}

		go (func(i int, group net.IP, c *ipv4.RawConn) {

			im := IgmpMembership{
				Iface:           iface,
				Conn:            c,
				GroupAddr:       group.To4(),
				DestAddr:        group.To4(),
				MaxResponseTime: maxResponseTime,
			}

			time.Sleep(time.Duration(i+1) * groupOffset)

			tick := time.NewTicker(sendPeriod)

			for {
				err := im.SendMembershipReport()
				if err != nil {
					log.Printf("Failed to send IGMP membership for %s: %v", group, err)
					break
				}

				<-tick.C
			}
		})(i, group, c)
	}

	return nil
}
