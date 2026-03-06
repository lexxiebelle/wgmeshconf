package writer

import (
	"strings"
	"testing"

	"github.com/lexxiebelle/wgmeshconf/internal/db"
)

func TestGenerateConfigs_Network(t *testing.T) {
	cluster := db.Cluster{
		Name: "net1",
		Mode: "network",
		CIDR: "10.0.0.0/24",
		Nodes: []db.Node{
			{ID: 1, Name: "a", Endpoint: "1.1.1.1", Address: "10.0.0.1", Port: 51820, PrivKey: "privA", PubKey: "pubA", AllowedIPs: []string{"10.10.0.0/24"}},
			{ID: 2, Name: "b", Endpoint: "2.2.2.2", Address: "10.0.0.2", Port: 51821, PrivKey: "privB", PubKey: "pubB", AllowedIPs: []string{"10.20.0.0/24"}},
		},
	}
	files, err := GenerateConfigs([]db.Cluster{cluster})
	if err != nil {
		t.Fatal(err)
	}

	// Should be two files: net1/tun_a.conf and net1/tun_b.conf
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}

	for _, node := range cluster.Nodes {
		rel := "net1/tun_" + node.Name + ".conf"
		buf, ok := files[rel]
		if !ok {
			t.Errorf("missing file %s", rel)
			continue
		}
		s := string(buf)

		if !strings.Contains(s, "# tun_"+node.Name) {
			t.Errorf("%s: missing header", rel)
		}
		if !strings.Contains(s, "PrivateKey = "+node.PrivKey) {
			t.Errorf("%s: missing private key", rel)
		}
		// Address/mask
		if !strings.Contains(s, "Address = "+node.Address+"/24") {
			t.Errorf("%s: wrong address line", rel)
		}
		// peer bιlock
		for _, peer := range cluster.Nodes {
			if peer.Name == node.Name {
				continue
			}
			if !strings.Contains(s, "# "+peer.Name) {
				t.Errorf("%s: missing peer header %s", rel, peer.Name)
			}
			if !strings.Contains(s, "PublicKey = "+peer.PubKey) {
				t.Errorf("%s: missing peer publickey", rel)
			}
			// allowedIPs includes peer.AllowedIPs[0] and node/32
			want := peer.AllowedIPs[0] + ", " + node.Address + "/32"
			if !strings.Contains(s, "AllowedIPs = "+want) {
				t.Errorf("%s: wrong AllowedIPs, got:\n%s", rel, s)
			}
		}
	}
}

func TestGenerateConfigs_PTP(t *testing.T) {
	cluster := db.Cluster{
		Name: "ptp1", Mode: "ptp",
		Nodes: []db.Node{
			{ID: 1, Name: "x", Endpoint: "1.1.1.1", AllowedIPs: []string{"10.10.1.0/24"}},
			{ID: 2, Name: "y", Endpoint: "2.2.2.2", AllowedIPs: []string{"10.20.1.0/24"}},
		},
		Tunnels: []db.Tunnel{
			{FromNodeID: 1, ToNodeID: 2, InterfaceIP: "10.1.0.1", PeerIP: "10.1.0.0", Port: 30000, PeerPort: 30001, PrivKey: "privXY", PubKey: "pubXY", PeerPubKey: "peerXY"},
			{FromNodeID: 2, ToNodeID: 1, InterfaceIP: "10.1.0.0", PeerIP: "10.1.0.1", Port: 30001, PeerPort: 30000, PrivKey: "privYX", PubKey: "pubYX", PeerPubKey: "peerYX"},
		},
	}

	files, err := GenerateConfigs([]db.Cluster{cluster})
	if err != nil {
		t.Fatal(err)
	}
	// should be 2 files: ptp1/x/tun_y.conf and ptp1/y/tun_x.conf
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}

	tests := []struct {
		from, to, priv, pub, peerPub, ifIP, peerIP string
		ifPort, peerPort                           int
	}{
		{"x", "y", "privXY", "pubXY", "peerXY", "10.1.0.1", "10.1.0.0", 30000, 30001},
		{"y", "x", "privYX", "pubYX", "peerYX", "10.1.0.0", "10.1.0.1", 30001, 30000},
	}

	for _, tc := range tests {
		rel := "ptp1/" + tc.from + "/tun_" + tc.to + ".conf"
		buf, ok := files[rel]
		if !ok {
			t.Errorf("missing %s", rel)
			continue
		}
		s := string(buf)
		if !strings.Contains(s, "PrivateKey = "+tc.priv) {
			t.Errorf("%s: missing priv", rel)
		}
		if !strings.Contains(s, "Address = "+tc.ifIP+"/31") {
			t.Errorf("%s: wrong address", rel)
		}
		if !strings.Contains(s, "PublicKey = "+tc.peerPub) {
			t.Errorf("%s: missing peerPub", rel)
		}
		wantAllowed := cluster.Nodes[1].AllowedIPs[0] + ", " + tc.peerIP + "/32"
		if tc.from == "y" {
			wantAllowed = cluster.Nodes[0].AllowedIPs[0] + ", " + tc.peerIP + "/32"
		}
		if !strings.Contains(s, "AllowedIPs = "+wantAllowed) {
			t.Errorf("%s: wrong AllowedIPs", rel)
		}
	}
}
