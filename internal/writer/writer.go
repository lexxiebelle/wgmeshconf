package writer

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lexxiebelle/wgmeshconf/internal/db"
)

func GenerateConfigs(clusters []db.Cluster) (map[string][]byte, error) {
	files := make(map[string][]byte)

	for _, cl := range clusters {
		switch cl.Mode {
		case "network":
			// For each node one config with [Interface] section + multiple [Peer] sections
			for _, node := range cl.Nodes {
				buf := bytes.Buffer{}

				// Tunnel name and public key
				fmt.Fprintf(&buf, "# tun_%s\n", node.Name)
				fmt.Fprintf(&buf, "# public: %s\n", node.PubKey)

				// [Interface]
				fmt.Fprintln(&buf, "[Interface]")
				// CIDR from cluster, take only the mask
				mask := strings.SplitN(cl.CIDR, "/", 2)[1]
				fmt.Fprintf(&buf, "Address = %s/%s\n", node.Address, mask)
				fmt.Fprintf(&buf, "ListenPort = %d\n", node.Port)
				fmt.Fprintf(&buf, "PrivateKey = %s\n\n", node.PrivKey)
				if cl.RemoveRoutes {
					fmt.Fprintf(&buf, "Table = off\n")
				}

				// all other nodes are peers
				for _, peer := range cl.Nodes {
					if peer.Name == node.Name {
						continue
					}

					allowed := []string{}
					if len(peer.AllowedIPs) > 0 {
						allowed = peer.AllowedIPs
					}
					if !cl.RemoveLocalIPFromAllowed {
						allowed = append(allowed, node.Address+"/32")
					}
					// insert comment above peers
					fmt.Fprintf(&buf, "# %s\n", peer.Name)
					fmt.Fprintln(&buf, "[Peer]")
					fmt.Fprintf(&buf, "PublicKey = %s\n", peer.PubKey)
					fmt.Fprintf(&buf, "Endpoint = %s:%d\n", peer.Endpoint, peer.Port)
					fmt.Fprintf(&buf, "AllowedIPs = %s\n", strings.Join(allowed, ", "))
					if cl.PersistentKeepalive > 0 {
						fmt.Fprintf(&buf, "PersistentKeepalive = %d\n", cl.PersistentKeepalive)
					}
					fmt.Fprintf(&buf, "\n")
				}

				rel := filepath.Join(cl.Name, fmt.Sprintf("tun_%s.conf", node.Name))
				files[rel] = buf.Bytes()
			}

		case "ptp":
			// For each Tunnel record generate a separate file
			// But it's convenient to group by FromNodeID
			nodeByID := make(map[uint]db.Node, len(cl.Nodes))
			for _, n := range cl.Nodes {
				nodeByID[n.ID] = n
			}
			for _, t := range cl.Tunnels {
				from := nodeByID[t.FromNodeID]
				to := nodeByID[t.ToNodeID]

				allowed := []string{}
				if len(to.AllowedIPs) > 0 {
					allowed = to.AllowedIPs
				}
				if !cl.RemoveLocalIPFromAllowed {
					allowed = append(allowed, t.PeerIP+"/32")
				}

				buf := bytes.Buffer{}

				// Tunnel name and public key
				fmt.Fprintf(&buf, "# tun_%s\n", to.Name)
				fmt.Fprintf(&buf, "# public: %s\n", t.PubKey)

				// [Interface]
				fmt.Fprintln(&buf, "[Interface]")
				fmt.Fprintf(&buf, "Address = %s/31\n", t.InterfaceIP)
				fmt.Fprintf(&buf, "ListenPort = %d\n", t.Port)
				fmt.Fprintf(&buf, "PrivateKey = %s\n", t.PrivKey)
				if cl.RemoveRoutes {
					fmt.Fprintf(&buf, "Table = off\n\n")
				}

				// [Peer]
				fmt.Fprintf(&buf, "# %s\n", to.Name)
				fmt.Fprintln(&buf, "[Peer]")
				fmt.Fprintf(&buf, "PublicKey = %s\n", t.PeerPubKey)
				fmt.Fprintf(&buf, "Endpoint = %s:%d\n", to.Endpoint, t.PeerPort)
				fmt.Fprintf(&buf, "AllowedIPs = %s\n", strings.Join(allowed, ", "))
				if cl.PersistentKeepalive > 0 {
					fmt.Fprintf(&buf, "PersistentKeepalive = %d\n", cl.PersistentKeepalive)
				}
				fmt.Fprintf(&buf, "\n")
				rel := filepath.Join(cl.Name, from.Name, fmt.Sprintf("tun_%s.conf", to.Name))
				files[rel] = buf.Bytes()
			}
		default:
			return nil, fmt.Errorf("unknown mode %q", cl.Mode)
		}
	}

	return files, nil
}

func WriteConfigs(baseDir string, files map[string][]byte) error {
	if err := os.RemoveAll(baseDir); err != nil {
		return fmt.Errorf("remove old configs: %w", err)
	}
	for rel, content := range files {
		full := filepath.Join(baseDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", full, err)
		}
		if err := os.WriteFile(full, content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", full, err)
		}
	}
	return nil
}
