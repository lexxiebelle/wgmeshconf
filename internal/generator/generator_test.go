package generator

import (
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/lexxiebelle/wgmeshconf/internal/config"
	"github.com/lexxiebelle/wgmeshconf/internal/db"
)

func hasDupStrings(ss []string) bool {
	seen := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		if _, ok := seen[s]; ok {
			return true
		}
		seen[s] = struct{}{}
	}
	return false
}

func toDBClusterFromGeneratedNetwork(name, cidr, portsRange, portsAllocate string, nodes []GeneratedNode) db.Cluster {
	out := db.Cluster{
		Name:          name,
		Mode:          "network",
		CIDR:          cidr,
		PortsRange:    portsRange,
		PortsAllocate: portsAllocate,
	}
	for i, n := range nodes {
		out.Nodes = append(out.Nodes, db.Node{
			ID:         uint(i + 1),
			ClusterID:  1,
			Name:       n.Name,
			Endpoint:   n.Endpoint,
			Address:    n.Address.String(),
			Port:       n.Port,
			PrivKey:    n.PrivKey,
			PubKey:     n.PubKey,
			AllowedIPs: append([]string{}, n.AllowedIPs...),
		})
	}
	return out
}

func toDBClusterFromGeneratedPTP(name, cidr, portsRange, portsAllocate string, nodes []GeneratedNode, tunnels []GeneratedTunnel) db.Cluster {
	out := db.Cluster{
		Name:          name,
		Mode:          "ptp",
		CIDR:          cidr,
		PortsRange:    portsRange,
		PortsAllocate: portsAllocate,
	}

	nodeIDByName := make(map[string]uint, len(nodes))
	for i, n := range nodes {
		id := uint(i + 1)
		nodeIDByName[n.Name] = id
		out.Nodes = append(out.Nodes, db.Node{
			ID:         id,
			ClusterID:  1,
			Name:       n.Name,
			Endpoint:   n.Endpoint,
			AllowedIPs: append([]string{}, n.AllowedIPs...),
		})
	}

	for _, t := range tunnels {
		out.Tunnels = append(out.Tunnels, db.Tunnel{
			ClusterID:   1,
			FromNodeID:  nodeIDByName[t.From],
			ToNodeID:    nodeIDByName[t.To],
			InterfaceIP: t.InterfaceIP.String(),
			PeerIP:      t.PeerIP.String(),
			Port:        t.Port,
			PeerPort:    t.PeerPort,
			PrivKey:     t.PrivKey,
			PubKey:      t.PubKey,
			PeerPubKey:  t.PeerPubKey,
		})
	}

	return out
}

func mapGeneratedNodesByName(nodes []GeneratedNode) map[string]GeneratedNode {
	out := make(map[string]GeneratedNode, len(nodes))
	for _, n := range nodes {
		out[n.Name] = n
	}
	return out
}

func mapGeneratedTunnelsByPair(tunnels []GeneratedTunnel) map[string]GeneratedTunnel {
	out := make(map[string]GeneratedTunnel, len(tunnels))
	for _, t := range tunnels {
		out[t.From+">"+t.To] = t
	}
	return out
}

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

func TestGenerateNetworkDuplicateStaticPortsError(t *testing.T) {
	cfg := &config.Config{
		Clusters: []config.Cluster{{
			Name:          "net-dup-port",
			Mode:          "network",
			CIDR:          "10.1.0.0/29",
			PortsRange:    "20000-20010",
			PortsAllocate: "linear",
			Nodes: []config.Node{
				{Name: "a", Endpoint: "1.1.1.1", Port: 20001},
				{Name: "b", Endpoint: "2.2.2.2", Port: 20001},
			},
		}},
	}

	_, err := Generate(cfg, nil)
	if err == nil {
		t.Fatal("expected duplicate static port error, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate static port") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateNetworkStaticPortReservedFromPoolOnFirstGeneration(t *testing.T) {
	cfg := &config.Config{
		Clusters: []config.Cluster{{
			Name:          "net-static-reserve",
			Mode:          "network",
			CIDR:          "10.66.0.0/29",
			PortsRange:    "24000-24002",
			PortsAllocate: "linear",
			Nodes: []config.Node{
				{Name: "a", Endpoint: "1.1.1.1", Port: 24001},
				{Name: "b", Endpoint: "2.2.2.2"},
				{Name: "c", Endpoint: "3.3.3.3"},
			},
		}},
	}

	gen, err := Generate(cfg, nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	c := gen.Clusters[0]
	byName := mapGeneratedNodesByName(c.Nodes)

	if byName["a"].Port != 24001 {
		t.Fatalf("node a static port = %d; want 24001", byName["a"].Port)
	}
	if byName["b"].Port == 24001 || byName["c"].Port == 24001 {
		t.Fatalf("allocator reused static port 24001: b=%d c=%d", byName["b"].Port, byName["c"].Port)
	}
	if byName["b"].Port == byName["c"].Port {
		t.Fatalf("auto-allocated ports must differ: b=%d c=%d", byName["b"].Port, byName["c"].Port)
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

// TestGeneratePTPAddNodeNoAddressOrPortCollisions verifies that when new
// nodes are added to an existing ptp cluster, newly allocated tunnel IPs/ports
// do not collide with already persisted tunnel values.
func TestGeneratePTPAddNodeNoAddressOrPortCollisions(t *testing.T) {
	cfg := &config.Config{
		Clusters: []config.Cluster{{
			Name:          "ptp3",
			Mode:          "ptp",
			CIDR:          "10.2.0.0/24",
			PortsRange:    "30000-30050",
			PortsAllocate: "linear",
			Nodes: []config.Node{
				{Name: "x", Endpoint: "1.1.1.1"},
				{Name: "y", Endpoint: "2.2.2.2"},
				{Name: "z", Endpoint: "3.3.3.3"}, // newly added node
			},
		}},
	}

	// Existing DB state has only x<->y tunnels.
	existing := []db.Cluster{{
		Name: "ptp3",
		Mode: "ptp",
		CIDR: "10.2.0.0/24",
		Nodes: []db.Node{
			{ID: 1, Name: "x", Endpoint: "1.1.1.1"},
			{ID: 2, Name: "y", Endpoint: "2.2.2.2"},
		},
		Tunnels: []db.Tunnel{
			{
				FromNodeID: 1, ToNodeID: 2,
				InterfaceIP: "10.2.0.0", PeerIP: "10.2.0.1",
				Port: 30000, PeerPort: 30001,
				PrivKey: "privXY", PubKey: "pubXY", PeerPubKey: "pubYX",
			},
			{
				FromNodeID: 2, ToNodeID: 1,
				InterfaceIP: "10.2.0.1", PeerIP: "10.2.0.0",
				Port: 30001, PeerPort: 30000,
				PrivKey: "privYX", PubKey: "pubYX", PeerPubKey: "pubXY",
			},
		},
	}}

	genCfg, err := Generate(cfg, existing)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	c := genCfg.Clusters[0]

	// x-y plus x-z plus y-z, each in both directions => 6 tunnels total
	if len(c.Tunnels) != 6 {
		t.Fatalf("len(Tunnels) = %d; want 6", len(c.Tunnels))
	}

	var allIPs []string
	var allPorts []string
	for _, tnl := range c.Tunnels {
		allIPs = append(allIPs, tnl.InterfaceIP.String())
		allPorts = append(allPorts, fmt.Sprintf("%d", tnl.Port))
	}
	if hasDupStrings(allIPs) {
		t.Fatalf("duplicate interface IPs found: %v", allIPs)
	}
	if hasDupStrings(allPorts) {
		t.Fatalf("duplicate ports found: %v", allPorts)
	}
}

// TestGenerateNetworkAddRemoveKeepsExistingNodeIdentity ensures that in network
// mode add/remove operations keep address/port/keys stable for remaining nodes.
func TestGenerateNetworkAddRemoveKeepsExistingNodeIdentity(t *testing.T) {
	baseCfg := &config.Config{
		Clusters: []config.Cluster{{
			Name:          "net-stable",
			Mode:          "network",
			CIDR:          "10.44.0.0/24",
			PortsRange:    "25000-25050",
			PortsAllocate: "linear",
			Nodes: []config.Node{
				{Name: "a", Endpoint: "1.1.1.1", AllowedIPs: []string{"10.10.0.0/16"}},
				{Name: "b", Endpoint: "2.2.2.2", AllowedIPs: []string{"10.20.0.0/16"}},
				{Name: "c", Endpoint: "3.3.3.3", AllowedIPs: []string{"10.30.0.0/16"}},
			},
		}},
	}

	gen1, err := Generate(baseCfg, nil)
	if err != nil {
		t.Fatalf("Generate(base) error: %v", err)
	}
	base := gen1.Clusters[0]
	existing1 := []db.Cluster{
		toDBClusterFromGeneratedNetwork(base.Name, base.CIDR, base.PortsRange, base.PortsAllocate, base.Nodes),
	}

	addCfg := &config.Config{
		Clusters: []config.Cluster{{
			Name:          "net-stable",
			Mode:          "network",
			CIDR:          "10.44.0.0/24",
			PortsRange:    "25000-25050",
			PortsAllocate: "linear",
			Nodes: []config.Node{
				{Name: "a", Endpoint: "1.1.1.1", AllowedIPs: []string{"10.10.0.0/16"}},
				{Name: "b", Endpoint: "2.2.2.2", AllowedIPs: []string{"10.20.0.0/16"}},
				{Name: "c", Endpoint: "3.3.3.3", AllowedIPs: []string{"10.30.0.0/16"}},
				{Name: "d", Endpoint: "4.4.4.4", AllowedIPs: []string{"10.40.0.0/16"}},
				{Name: "e", Endpoint: "5.5.5.5", AllowedIPs: []string{"10.50.0.0/16"}},
			},
		}},
	}
	gen2, err := Generate(addCfg, existing1)
	if err != nil {
		t.Fatalf("Generate(add) error: %v", err)
	}
	added := gen2.Clusters[0]

	baseByName := mapGeneratedNodesByName(base.Nodes)
	addedByName := mapGeneratedNodesByName(added.Nodes)
	for _, name := range []string{"a", "b", "c"} {
		bn := baseByName[name]
		an := addedByName[name]
		if bn.Address.String() != an.Address.String() || bn.Port != an.Port || bn.PrivKey != an.PrivKey || bn.PubKey != an.PubKey {
			t.Fatalf("node %s changed after add: before(%s,%d,%s,%s) after(%s,%d,%s,%s)",
				name, bn.Address.String(), bn.Port, bn.PrivKey, bn.PubKey, an.Address.String(), an.Port, an.PrivKey, an.PubKey)
		}
	}

	existing2 := []db.Cluster{
		toDBClusterFromGeneratedNetwork(added.Name, added.CIDR, added.PortsRange, added.PortsAllocate, added.Nodes),
	}
	removeCfg := &config.Config{
		Clusters: []config.Cluster{{
			Name:          "net-stable",
			Mode:          "network",
			CIDR:          "10.44.0.0/24",
			PortsRange:    "25000-25050",
			PortsAllocate: "linear",
			Nodes: []config.Node{
				{Name: "a", Endpoint: "1.1.1.1", AllowedIPs: []string{"10.10.0.0/16"}},
				{Name: "b", Endpoint: "2.2.2.2", AllowedIPs: []string{"10.20.0.0/16"}},
				{Name: "c", Endpoint: "3.3.3.3", AllowedIPs: []string{"10.30.0.0/16"}},
				{Name: "d", Endpoint: "4.4.4.4", AllowedIPs: []string{"10.40.0.0/16"}},
			},
		}},
	}
	gen3, err := Generate(removeCfg, existing2)
	if err != nil {
		t.Fatalf("Generate(remove) error: %v", err)
	}
	removed := gen3.Clusters[0]
	removedByName := mapGeneratedNodesByName(removed.Nodes)
	addedByName = mapGeneratedNodesByName(added.Nodes)
	for _, name := range []string{"a", "b", "c", "d"} {
		an := addedByName[name]
		rn := removedByName[name]
		if an.Address.String() != rn.Address.String() || an.Port != rn.Port || an.PrivKey != rn.PrivKey || an.PubKey != rn.PubKey {
			t.Fatalf("node %s changed after remove: before(%s,%d,%s,%s) after(%s,%d,%s,%s)",
				name, an.Address.String(), an.Port, an.PrivKey, an.PubKey, rn.Address.String(), rn.Port, rn.PrivKey, rn.PubKey)
		}
	}
}

// TestGeneratePTPAddRemoveKeepsExistingTunnelIdentity ensures that in ptp mode
// add/remove operations keep tunnel IPs/ports/keys stable for remaining pairs.
func TestGeneratePTPAddRemoveKeepsExistingTunnelIdentity(t *testing.T) {
	baseCfg := &config.Config{
		Clusters: []config.Cluster{{
			Name:          "ptp-stable",
			Mode:          "ptp",
			CIDR:          "10.55.0.0/24",
			PortsRange:    "26000-26100",
			PortsAllocate: "linear",
			Nodes: []config.Node{
				{Name: "x", Endpoint: "1.1.1.1"},
				{Name: "y", Endpoint: "2.2.2.2"},
				{Name: "z", Endpoint: "3.3.3.3"},
			},
		}},
	}
	gen1, err := Generate(baseCfg, nil)
	if err != nil {
		t.Fatalf("Generate(base) error: %v", err)
	}
	base := gen1.Clusters[0]
	existing1 := []db.Cluster{
		toDBClusterFromGeneratedPTP(base.Name, base.CIDR, base.PortsRange, base.PortsAllocate, base.Nodes, base.Tunnels),
	}

	addCfg := &config.Config{
		Clusters: []config.Cluster{{
			Name:          "ptp-stable",
			Mode:          "ptp",
			CIDR:          "10.55.0.0/24",
			PortsRange:    "26000-26100",
			PortsAllocate: "linear",
			Nodes: []config.Node{
				{Name: "x", Endpoint: "1.1.1.1"},
				{Name: "y", Endpoint: "2.2.2.2"},
				{Name: "z", Endpoint: "3.3.3.3"},
				{Name: "w", Endpoint: "4.4.4.4"},
			},
		}},
	}
	gen2, err := Generate(addCfg, existing1)
	if err != nil {
		t.Fatalf("Generate(add) error: %v", err)
	}
	added := gen2.Clusters[0]

	baseT := mapGeneratedTunnelsByPair(base.Tunnels)
	addedT := mapGeneratedTunnelsByPair(added.Tunnels)
	for _, key := range []string{"x>y", "y>x", "x>z", "z>x", "y>z", "z>y"} {
		bt := baseT[key]
		at := addedT[key]
		if bt.InterfaceIP.String() != at.InterfaceIP.String() ||
			bt.PeerIP.String() != at.PeerIP.String() ||
			bt.Port != at.Port ||
			bt.PeerPort != at.PeerPort ||
			bt.PrivKey != at.PrivKey ||
			bt.PubKey != at.PubKey ||
			bt.PeerPubKey != at.PeerPubKey {
			t.Fatalf("tunnel %s changed after add", key)
		}
	}

	existing2 := []db.Cluster{
		toDBClusterFromGeneratedPTP(added.Name, added.CIDR, added.PortsRange, added.PortsAllocate, added.Nodes, added.Tunnels),
	}
	removeCfg := &config.Config{
		Clusters: []config.Cluster{{
			Name:          "ptp-stable",
			Mode:          "ptp",
			CIDR:          "10.55.0.0/24",
			PortsRange:    "26000-26100",
			PortsAllocate: "linear",
			Nodes: []config.Node{
				{Name: "x", Endpoint: "1.1.1.1"},
				{Name: "y", Endpoint: "2.2.2.2"},
				{Name: "z", Endpoint: "3.3.3.3"},
			},
		}},
	}
	gen3, err := Generate(removeCfg, existing2)
	if err != nil {
		t.Fatalf("Generate(remove) error: %v", err)
	}
	removed := gen3.Clusters[0]
	removedT := mapGeneratedTunnelsByPair(removed.Tunnels)
	addedT = mapGeneratedTunnelsByPair(added.Tunnels)
	for _, key := range []string{"x>y", "y>x", "x>z", "z>x", "y>z", "z>y"} {
		at := addedT[key]
		rt := removedT[key]
		if at.InterfaceIP.String() != rt.InterfaceIP.String() ||
			at.PeerIP.String() != rt.PeerIP.String() ||
			at.Port != rt.Port ||
			at.PeerPort != rt.PeerPort ||
			at.PrivKey != rt.PrivKey ||
			at.PubKey != rt.PubKey ||
			at.PeerPubKey != rt.PeerPubKey {
			t.Fatalf("tunnel %s changed after remove", key)
		}
	}
}
