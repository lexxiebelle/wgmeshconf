package generator

import (
	"fmt"
	"net"
	"testing"

	"github.com/lexxiebelle/wgmeshconf/internal/config"
	"github.com/lexxiebelle/wgmeshconf/internal/db"
)

// TestGenerateNetworkThreeNodes asserts correct generation in network mode for three nodes
func TestGenerateNetworkThreeNodes(t *testing.T) {
	cfg := &config.Config{
		Clusters: []config.Cluster{{
			Name:          "net3",
			Mode:          "network",
			CIDR:          "10.1.0.0/29",
			PortsRange:    "20000-20005",
			PortsAllocate: "linear",
			Nodes: []config.Node{
				{Name: "a", Endpoint: "1.1.1.1", AllowedIPs: []string{"10.10.1.0/24"}},
				{Name: "b", Endpoint: "2.2.2.2", AllowedIPs: []string{"10.20.1.0/24"}},
				{Name: "c", Endpoint: "3.3.3.3", AllowedIPs: []string{"10.30.1.0/24"}},
			},
		}},
	}

	gen, err := Generate(cfg, []db.Cluster{})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	c := gen.Clusters[0]
	if c.Mode != "network" {
		t.Errorf("Mode = %q; want \"network\"", c.Mode)
	}
	if len(c.Nodes) != 3 {
		t.Fatalf("len(Nodes) = %d; want 3", len(c.Nodes))
	}

	ports := []int{20000, 20001, 20002}
	for i, node := range c.Nodes {
		expName := fmt.Sprintf("%c", 'a'+i)
		if node.Name != expName {
			t.Errorf("Node[%d].Name = %q; want %q", i, node.Name, expName)
		}

		expPort := ports[i]
		if node.Port != expPort {
			t.Errorf("Node[%d].Port = %d; want %d", i, node.Port, expPort)
		}

		// Address should be sequential from .1
		expAddr := fmt.Sprintf("10.1.0.%d", i+1)
		if node.Address.String() != expAddr {
			t.Errorf("Node[%d].Address = %s; want %s", i, node.Address.String(), expAddr)
		}

		// AllowedIPs includes original then local /32
		if len(node.AllowedIPs) != 2 {
			t.Errorf("Node[%d].AllowedIPs length = %d; want 2", i, len(node.AllowedIPs))
		}
		if node.AllowedIPs[1] != expAddr+"/32" {
			t.Errorf("Node[%d].AllowedIPs[1] = %q; want %q", i, node.AllowedIPs[1], expAddr+"/32")
		}
	}
}

// TestGeneratePTPThreeNodes asserts correct generation in ptp mode for three nodes and key mapping
func TestGeneratePTPThreeNodes(t *testing.T) {
	cfg := &config.Config{
		Clusters: []config.Cluster{{
			Name:          "ptp3",
			Mode:          "ptp",
			CIDR:          "10.2.0.0/24",
			PortsRange:    "30000-30005",
			PortsAllocate: "linear",
			Nodes: []config.Node{
				{Name: "x", Endpoint: "1.1.1.1", AllowedIPs: []string{}},
				{Name: "y", Endpoint: "2.2.2.2", AllowedIPs: []string{}},
				{Name: "z", Endpoint: "3.3.3.3", AllowedIPs: []string{}},
			},
		}},
	}

	gen, err := Generate(cfg, []db.Cluster{})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	c := gen.Clusters[0]
	if c.Mode != "ptp" {
		t.Errorf("Mode = %q; want \"ptp\"", c.Mode)
	}

	// Expect 3 pairs × 2 tunnels = 6
	if len(c.Tunnels) != 6 {
		t.Fatalf("len(Tunnels) = %d; want 6", len(c.Tunnels))
	}

	// Map tunnels by from->to
	tmap := make(map[string]GeneratedTunnel)
	for _, tnl := range c.Tunnels {
		key := tnl.From + ">" + tnl.To
		tmap[key] = tnl
	}

	pairs := [][2]string{{"x", "y"}, {"x", "z"}, {"y", "z"}}
	port := 30000
	for i, p := range pairs {
		src, dst := p[0], p[1]
		key1, key2 := src+">"+dst, dst+">"+src

		t1, ok1 := tmap[key1]
		t2, ok2 := tmap[key2]
		if !ok1 || !ok2 {
			t.Fatalf("tunnels for pair %s-%s missing", src, dst)
		}

		// IP pairs: offsets 0-1, 2-3, 4-5
		offset := i * 2
		exp1 := net.ParseIP(fmt.Sprintf("10.2.0.%d", offset))
		exp2 := net.ParseIP(fmt.Sprintf("10.2.0.%d", offset+1))

		if !t1.InterfaceIP.Equal(exp1) || !t1.PeerIP.Equal(exp2) {
			t.Errorf("%s IPs = %v/%v; want %v/%v", key1, t1.InterfaceIP, t1.PeerIP, exp1, exp2)
		}
		if !t2.InterfaceIP.Equal(exp2) || !t2.PeerIP.Equal(exp1) {
			t.Errorf("%s IPs = %v/%v; want %v/%v", key2, t2.InterfaceIP, t2.PeerIP, exp2, exp1)
		}

		// Ports sequential
		if t1.Port != port || t1.PeerPort != port+1 {
			t.Errorf("%s Ports = %d/%d; want %d/%d", key1, t1.Port, t1.PeerPort, port, port+1)
		}
		if t2.Port != port+1 || t2.PeerPort != port {
			t.Errorf("%s Ports = %d/%d; want %d/%d", key2, t2.Port, t2.PeerPort, port+1, port)
		}

		// Key correctness
		if t1.PeerPubKey != t2.PubKey || t2.PeerPubKey != t1.PubKey {
			t.Errorf("key mismatch for pair %s/%s", key1, key2)
		}
		if t1.PubKey == "" || t2.PubKey == "" {
			t.Errorf("empty public key for %s or %s", key1, key2)
		}

		port += 2
	}
}

// TestGenerateNetworkReuse verifies that when existing nodes are passed in,
// the generator reuses their addresses and ports instead of allocating new ones.
func TestGenerateNetworkReuse(t *testing.T) {
	cfg := &config.Config{
		Clusters: []config.Cluster{{
			Name:          "net3",
			Mode:          "network",
			CIDR:          "10.1.0.0/29",
			PortsRange:    "20000-20005",
			PortsAllocate: "linear",
			Nodes: []config.Node{
				{Name: "a", Endpoint: "1.1.1.1", AllowedIPs: []string{"10.10.1.0/24"}},
				{Name: "b", Endpoint: "2.2.2.2", AllowedIPs: []string{"10.20.1.0/24"}},
				{Name: "c", Endpoint: "3.3.3.3", AllowedIPs: []string{"10.30.1.0/24"}},
			},
		}},
	}

	// Simulate existing DB state with pre-allocated addresses and ports
	existing := []db.Cluster{{
		Name:          "net3",
		Mode:          "network",
		CIDR:          "10.1.0.0/29",
		PortsRange:    "20000-20005",
		PortsAllocate: "linear",
		Nodes: []db.Node{
			{ID: 1, Name: "a", Endpoint: "1.1.1.1", Address: "10.1.0.5", Port: 20003, AllowedIPs: []string{"10.10.1.0/24", "10.1.0.5/32"}},
			{ID: 2, Name: "b", Endpoint: "2.2.2.2", Address: "10.1.0.6", Port: 20004, AllowedIPs: []string{"10.20.1.0/24", "10.1.0.6/32"}},
			{ID: 3, Name: "c", Endpoint: "3.3.3.3", Address: "10.1.0.7", Port: 20005, AllowedIPs: []string{"10.30.1.0/24", "10.1.0.7/32"}},
		},
	}}

	genCfg, err := Generate(cfg, existing)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	c := genCfg.Clusters[0]

	// Verify reuse of addresses and ports
	expAddrs := []string{"10.1.0.5", "10.1.0.6", "10.1.0.7"}
	expPorts := []int{20003, 20004, 20005}
	for i, node := range c.Nodes {
		if node.Address.String() != expAddrs[i] {
			t.Errorf("Node %s Address = %s; want %s", node.Name, node.Address, expAddrs[i])
		}
		if node.Port != expPorts[i] {
			t.Errorf("Node %s Port = %d; want %d", node.Name, node.Port, expPorts[i])
		}
	}
}

// TestGeneratePTPReuse verifies that existing tunnels are reused and new ones
// are created only for missing pairs.
func TestGeneratePTPReuse(t *testing.T) {
	cfg := &config.Config{
		Clusters: []config.Cluster{{
			Name:          "ptp3",
			Mode:          "ptp",
			CIDR:          "10.2.0.0/24",
			PortsRange:    "30000-30005",
			PortsAllocate: "linear",
			Nodes: []config.Node{
				{Name: "x", Endpoint: "1.1.1.1"},
				{Name: "y", Endpoint: "2.2.2.2"},
				{Name: "z", Endpoint: "3.3.3.3"},
			},
		}},
	}

	// Existing DB state: tunnel x->y and y->x
	existing := []db.Cluster{{
		Name: "ptp3",
		Mode: "ptp",
		CIDR: "10.2.0.0/24",
		Nodes: []db.Node{
			{ID: 1, Name: "x", Endpoint: "1.1.1.1"},
			{ID: 2, Name: "y", Endpoint: "2.2.2.2"},
			{ID: 3, Name: "z", Endpoint: "3.3.3.3"},
		},
		Tunnels: []db.Tunnel{
			{
				FromNodeID: 1, ToNodeID: 2,
				InterfaceIP: "10.2.0.0", PeerIP: "10.2.0.1",
				Port: 30003, PeerPort: 30004,
				PrivKey: "privXY", PubKey: "pubXY", PeerPubKey: "pubYX",
			},
			{
				FromNodeID: 2, ToNodeID: 1,
				InterfaceIP: "10.2.0.1", PeerIP: "10.2.0.0",
				Port: 30004, PeerPort: 30003,
				PrivKey: "privYX", PubKey: "pubYX", PeerPubKey: "pubXY",
			},
		},
	}}

	genCfg, err := Generate(cfg, existing)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	c := genCfg.Clusters[0]

	// Expect 6 tunnels in total (existing 2 + new 4)
	if len(c.Tunnels) != 6 {
		t.Fatalf("len(Tunnels) = %d; want 6", len(c.Tunnels))
	}

	// Find the reused tunnel x->y
	var reused GeneratedTunnel
	for _, tnl := range c.Tunnels {
		if tnl.From == "x" && tnl.To == "y" {
			reused = tnl
		}
	}

	// Check reuse of IPs, ports and keys
	expIP1 := net.ParseIP("10.2.0.0")
	expIP2 := net.ParseIP("10.2.0.1")
	if !reused.InterfaceIP.Equal(expIP1) || !reused.PeerIP.Equal(expIP2) {
		t.Errorf("Reused IPs = %v/%v; want %v/%v", reused.InterfaceIP, reused.PeerIP, expIP1, expIP2)
	}
	if reused.Port != 30003 || reused.PeerPort != 30004 {
		t.Errorf("Reused ports = %d/%d; want %d/%d", reused.Port, reused.PeerPort, 30003, 30004)
	}
	if reused.PubKey != "pubXY" || reused.PeerPubKey != "pubYX" {
		t.Errorf("Reused keys = %s/%s; want pubXY/pubYX", reused.PubKey, reused.PeerPubKey)
	}
}
