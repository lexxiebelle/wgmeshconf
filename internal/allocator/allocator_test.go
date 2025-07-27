package allocator

import (
	"fmt"
	"net"
	"testing"
)

func TestIPAllocator_AllocateOne(t *testing.T) {
	// CIDR with 8 addresses: 192.168.0.0/29 → usable .1–.6
	existing := []string{"192.168.0.2"} // reserve .2
	a, err := NewIPAllocator("192.168.0.0/29", existing)
	if err != nil {
		t.Fatalf("failed to create IPAllocator: %v", err)
	}

	seen := make(map[string]bool)
	for range 5 { // 6 usable minus 1 existing = 5
		ip, err := a.AllocateOne()
		if err != nil {
			t.Fatalf("unexpected error allocating IP: %v", err)
		}
		if !a.netIP.Contains(ip) {
			t.Errorf("allocated IP %v not in network", ip)
		}
		s := ip.String()
		if s == "192.168.0.2" {
			t.Errorf("allocated reserved IP: %s", s)
		}
		if seen[s] {
			t.Errorf("duplicate IP allocated: %s", s)
		}
		seen[s] = true
	}

	// next allocation should fail
	if _, err := a.AllocateOne(); err == nil {
		t.Error("expected error when IPs exhausted, got none")
	}
}

func TestIPAllocator_AllocatePair(t *testing.T) {
	// CIDR with 4 addresses: 10.0.0.0/30 → pairs [0,1] and [2,3]
	a, err := NewIPAllocator("10.0.0.0/30", nil)
	if err != nil {
		t.Fatalf("failed to create IPAllocator: %v", err)
	}

	// First pair
	ip1, ip2, err := a.AllocatePair()
	if err != nil {
		t.Fatalf("first pair allocation failed: %v", err)
	}
	if ip1.String() != "10.0.0.0" || ip2.String() != "10.0.0.1" {
		t.Errorf("unexpected first pair: %v, %v", ip1, ip2)
	}

	// Second pair
	ip3, ip4, err := a.AllocatePair()
	if err != nil {
		t.Fatalf("second pair allocation failed: %v", err)
	}
	if ip3.String() != "10.0.0.2" || ip4.String() != "10.0.0.3" {
		t.Errorf("unexpected second pair: %v, %v", ip3, ip4)
	}

	// no more pairs
	if _, _, err := a.AllocatePair(); err == nil {
		t.Error("expected error when pairs exhausted, got none")
	}
}

func TestIPAllocator_Segmented24(t *testing.T) {
	// /23 covers 10.0.0.0 - 10.0.1.255. We expect 2 segments of 254 hosts each: .0.1-.0.254 then .1.1-.1.254
	a, err := NewIPAllocator("10.0.0.0/23", nil)
	if err != nil {
		t.Fatalf("failed to create allocator: %v", err)
	}
	// First segment
	for i := 1; i <= 254; i++ {
		ip, err := a.AllocateOne()
		if err != nil {
			t.Fatalf("allocation #%d failed: %v", i, err)
		}
		want := net.ParseIP(fmt.Sprintf("10.0.0.%d", i))
		if !ip.Equal(want) {
			t.Errorf("#%d = %v; want %v", i, ip, want)
		}
	}
	// Skip .0.255 (broadcast for first /24)
	// Next allocation should be second segment at .1.1
	ip, err := a.AllocateOne()
	if err != nil {
		t.Fatalf("allocation after first segment failed: %v", err)
	}
	want := net.ParseIP("10.0.1.1")
	if !ip.Equal(want) {
		t.Errorf("after segment: got %v; want %v", ip, want)
	}
}

func TestPortAllocator_Linear(t *testing.T) {
	// Range 1000-1002, no existing
	p, err := NewPortAllocator("1000-1002", "linear", nil)
	if err != nil {
		t.Fatalf("failed to create PortAllocator: %v", err)
	}

	expected := []int{1000, 1001, 1002}
	for _, exp := range expected {
		port, err := p.Allocate()
		if err != nil {
			t.Fatalf("unexpected error allocating port: %v", err)
		}
		if port != exp {
			t.Errorf("expected port %d, got %d", exp, port)
		}
	}

	// exhausted
	if _, err := p.Allocate(); err == nil {
		t.Error("expected error when ports exhausted, got none")
	}
}

func TestPortAllocator_Random(t *testing.T) {
	// Range 2000-2002, no existing
	p, err := NewPortAllocator("2000-2003", "random", nil)
	if err != nil {
		t.Fatalf("failed to create PortAllocator: %v", err)
	}

	seen := make(map[int]bool)
	for range 4 {
		port, err := p.Allocate()
		if err != nil {
			t.Fatalf("unexpected error allocating random port: %v", err)
		}
		if port < 2000 || port > 2003 {
			t.Errorf("allocated port %d out of range", port)
		}
		if seen[port] {
			t.Errorf("duplicate random port %d", port)
		}
		seen[port] = true
	}

	// exhausted
	if _, err := p.Allocate(); err == nil {
		t.Error("expected error when random ports exhausted, got none")
	}
}

func TestPortAllocator_Existing(t *testing.T) {
	// Range 3000-3002, existing [3001]
	existing := []int{3001}
	p, err := NewPortAllocator("3000-3002", "linear", existing)
	if err != nil {
		t.Fatalf("failed to create PortAllocator: %v", err)
	}

	// should allocate 3000 then 3002
	port, err := p.Allocate()
	if err != nil || port != 3000 {
		t.Fatalf("expected 3000, got %d, err %v", port, err)
	}
	port, err = p.Allocate()
	if err != nil || port != 3002 {
		t.Fatalf("expected 3002, got %d, err %v", port, err)
	}
	// exhausted
	if _, err := p.Allocate(); err == nil {
		t.Error("expected error when ports exhausted, got none")
	}
}
