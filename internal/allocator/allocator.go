package allocator

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"
)

// IPAllocator allocates IP addresses from a given CIDR block.
// Supports single-ip allocation (network mode) and /31-pair allocation (ptp mode).
type IPAllocator struct {
	netIP      *net.IPNet
	base       uint32          // base IP as uint32
	size       uint32          // total number of addresses in the block
	allocated1 map[uint32]bool // allocated single addresses
	allocated2 map[uint32]bool // allocated pair starts (offsets)
	next1      uint32          // next offset candidate for single
	next2      uint32          // next offset candidate for pair (increment by 2)
}

// NewIPAllocator creates an allocator for the given CIDR.
// existing is a list of already-used IP strings (with or without /32 mask).
func NewIPAllocator(cidr string, existing []string) (*IPAllocator, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR '%s': %w", cidr, err)
	}
	maskSize, bits := ipnet.Mask.Size()
	hostBits := bits - maskSize
	if hostBits < 1 {
		return nil, fmt.Errorf("CIDR '%s' too small for allocation", cidr)
	}

	// compute base and total size
	base := binary.BigEndian.Uint32(ip.To4())
	size := uint32(1) << hostBits

	a := &IPAllocator{
		netIP:      ipnet,
		base:       base,
		size:       size,
		allocated1: make(map[uint32]bool),
		allocated2: make(map[uint32]bool),
		next1:      1, // skip network address at offset 0
		next2:      0, // will step by 2
	}

	// reserve existing addresses
	for _, s := range existing {
		// strip mask if present
		addr := s
		if strings.Contains(s, "/") {
			addr = strings.SplitN(s, "/", 2)[0]
		}
		parsed := net.ParseIP(addr)
		if parsed == nil {
			continue
		}
		u := binary.BigEndian.Uint32(parsed.To4())
		o := u - base
		// if in range, mark allocated
		if o < size {
			a.allocated1[o] = true
		}
	}

	return a, nil
}

// AllocateOne returns a single unused IP (as net.IP) within the CIDR, with /32 mask.
func (a *IPAllocator) AllocateOne() (net.IP, error) {
	for o := a.next1; o < a.size; o++ {
		if a.allocated1[o] {
			continue
		}
		// skip broadcast of the whole block
		if o == a.size-1 {
			continue
		}
		// calculate absolute address
		ipInt := a.base + o
		// skip network and broadcast of each /24
		low := byte(ipInt & 0xFF)
		if low == 0 || low == 255 {
			continue
		}
		// reserve this IP
		a.allocated1[o] = true
		a.next1 = o + 1
		// collect net.IP and return
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, ipInt)
		return ip, nil
	}
	return nil, errors.New("no available IP addresses in network mode")
}

// AllocatePair returns two IPs forming a /31 subnet: first and second addresses.
func (a *IPAllocator) AllocatePair() (net.IP, net.IP, error) {
	totalPairs := a.size / 2
	for i := a.next2 / 2; i < totalPairs; i++ {
		o := uint32(i) * 2
		if !a.allocated2[o] {
			// allocate this pair
			a.allocated2[o] = true
			a.next2 = o + 2

			ip1 := make(net.IP, 4)
			ip2 := make(net.IP, 4)
			binary.BigEndian.PutUint32(ip1, a.base+o)
			binary.BigEndian.PutUint32(ip2, a.base+o+1)
			return ip1, ip2, nil
		}
	}
	return nil, nil, errors.New("no available /31 subnets in ptp mode")
}

// PortAllocator allocates UDP ports from a given inclusive range, in linear or random mode.
type PortAllocator struct {
	min       int
	max       int
	allocated map[int]bool
	mode      string
}

// NewPortAllocator creates a PortAllocator. rangeStr is "min-max". mode is "linear" or "random".
// existing is a slice of already-used ports.
func NewPortAllocator(rangeStr, mode string, existing []int) (*PortAllocator, error) {
	parts := strings.Split(rangeStr, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid portsRange '%s', expected 'min-max'", rangeStr)
	}
	min, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid port min '%s': %w", parts[0], err)
	}
	max, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid port max '%s': %w", parts[1], err)
	}
	if min > max {
		return nil, errors.New("portsRange min must be <= max")
	}
	pa := &PortAllocator{
		min:       min,
		max:       max,
		allocated: make(map[int]bool),
		mode:      mode,
	}
	// reserve existing
	for _, p := range existing {
		if p >= min && p <= max {
			pa.allocated[p] = true
		}
	}
	return pa, nil
}

// Allocate returns a new port. In "linear" mode, returns the next free port in sequence;
// in "random" mode, picks a random free port.
func (p *PortAllocator) Allocate() (int, error) {
	switch strings.ToLower(p.mode) {
	case "linear":
		for port := p.min; port <= p.max; port++ {
			if !p.allocated[port] {
				p.allocated[port] = true
				return port, nil
			}
		}
		return 0, errors.New("no available ports in linear allocation")

	case "random":
		rangeSize := p.max - p.min + 1
		for range rangeSize {
			// crypto random int in [0, rangeSize)
			nBig, err := rand.Int(rand.Reader, big.NewInt(int64(rangeSize)))
			if err != nil {
				continue
			}
			off := int(nBig.Int64())
			port := p.min + off
			if !p.allocated[port] {
				p.allocated[port] = true
				return port, nil
			}
		}
		return 0, errors.New("no available ports in random allocation")

	default:
		return 0, fmt.Errorf("unknown allocation mode: %s", p.mode)
	}
}
