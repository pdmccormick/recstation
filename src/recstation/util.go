package recstation

import (
	"net"
)

func IPtoU32(ip net.IP) uint32 {
	return uint32(ip[0]) | (uint32(ip[1]) << 8) | (uint32(ip[2]) << 16) | (uint32(ip[3]) << 24)
}
