package mock

import (
	"encoding/binary"
	"errors"
	"net"

	"github.com/cybozu-go/sabakan"
)

// NewLeaser returns a mocked sabakan.Leaser
func NewLeaser(begin, end net.IP) sabakan.Leaser {
	return &assignment{
		begin:  begin,
		end:    end,
		leases: make(map[uint32]struct{}),
	}
}

// TODO this is a temporary on-memory DHCP leases for the mock
type assignment struct {
	begin net.IP
	end   net.IP
	//TODO: leases must be committed on DHCPREQUEST
	leases map[uint32]struct{}
}

func (a *assignment) Lease() (net.IP, error) {
	ibegin := ip2int(a.begin)
	iend := ip2int(a.end)
	for n := ibegin; n <= iend; n++ {
		if _, ok := a.leases[n]; ok {
			continue
		}

		a.leases[n] = struct{}{}

		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, n)
		return ip, nil
	}
	return nil, errors.New("leases are full")
}

func ip2int(ip net.IP) uint32 {
	if len(ip) == 16 {
		return binary.BigEndian.Uint32(ip[12:16])
	}
	return binary.BigEndian.Uint32(ip)
}