package generator

import (
	"fmt"
	"net"

	"github.com/lexxiebelle/wgmeshconf/internal/allocator"
	"github.com/lexxiebelle/wgmeshconf/internal/config"
	"github.com/lexxiebelle/wgmeshconf/internal/db"
	"github.com/lexxiebelle/wgmeshconf/internal/utils"
)

// GeneratedConfig holds the full generated state for all clusters
type GeneratedConfig struct {
	Clusters []GeneratedCluster
}

// GeneratedCluster contains generated data for one cluster
type GeneratedCluster struct {
	Name                     string
	Mode                     string
	CIDR                     string
	PortsRange               string
	PortsAllocate            string
	RemoveLocalIPFromAllowed bool
	RemoveRoutes             bool
	PersistentKeepalive      int
	Nodes                    []GeneratedNode
	Tunnels                  []GeneratedTunnel // for ptp
}

// GeneratedNode holds info for a network‐mode node
type GeneratedNode struct {
	Name       string
	Endpoint   string
	Port       int
	Address    net.IP
	PrivKey    string
	PubKey     string
	AllowedIPs []string
}

// GeneratedTunnel holds one tunnel (ptp)
type GeneratedTunnel struct {
	From        string // node name
	To          string
	InterfaceIP net.IP // local IP (/31 start)
	PeerIP      net.IP // remote IP (/31 second)
	Port        int
	PeerPort    int
	PrivKey     string
	PubKey      string
	PeerPubKey  string
	AllowedIPs  []string // merge of node.AllowedIPs + InterfaceIP/32
}

// Generate processes the config and returns GeneratedConfig.
// existing must come from DB with Preload("Nodes") and Preload("Tunnels").
func Generate(cfg *config.Config, existing []db.Cluster) (*GeneratedConfig, error) {
	gen := &GeneratedConfig{}
	// Build lookup: cluster name → existing record
	existingByCluster := make(map[string]db.Cluster, len(existing))
	for _, c := range existing {
		existingByCluster[c.Name] = c
	}

	for _, cl := range cfg.Clusters {
		gc := GeneratedCluster{
			Name:                     cl.Name,
			Mode:                     cl.Mode,
			CIDR:                     cl.CIDR,
			PortsRange:               cl.PortsRange,
			PortsAllocate:            cl.PortsAllocate,
			RemoveLocalIPFromAllowed: cl.RemoveLocalIPFromAllowed,
			RemoveRoutes:             cl.RemoveRoutes,
		}
		dbC, hasDB := existingByCluster[cl.Name]

		// Prepare allocators seeded from DB state
		existingAddrs := []string{}
		existingPorts := []int{}
		if hasDB {
			for _, dn := range dbC.Nodes {
				existingAddrs = append(existingAddrs, dn.Address)
				existingPorts = append(existingPorts, dn.Port)
			}
		}
		ipAlloc, err := allocator.NewIPAllocator(cl.CIDR, existingAddrs)
		if err != nil {
			return nil, fmt.Errorf("cluster %s: %w", cl.Name, err)
		}
		portAlloc, err := allocator.NewPortAllocator(cl.PortsRange, cl.PortsAllocate, existingPorts)
		if err != nil {
			return nil, fmt.Errorf("cluster %s: %w", cl.Name, err)
		}

		if cl.Mode == "network" {
			// One interface per node
			for _, node := range cl.Nodes {
				// reuse or allocate address & port
				var addr net.IP
				var port int
				var priv string
				var pub string
				if hasDB {
					for _, dn := range dbC.Nodes {
						if dn.Name == node.Name {
							addr = net.ParseIP(dn.Address)
							port = dn.Port
							priv = dn.PrivKey
							pub = dn.PubKey
							break
						}
					}
				}

				if node.BindAddress != "" {
					addr = net.ParseIP(node.BindAddress)
				}

				if node.Port != 0 {
					port = node.Port
				}

				if addr == nil {
					addr, err = ipAlloc.AllocateOne()
					if err != nil {
						return nil, fmt.Errorf("node %s: %w", node.Name, err)
					}
				}
				if port == 0 {
					port, err = portAlloc.Allocate()
					if err != nil {
						return nil, fmt.Errorf("node %s: %w", node.Name, err)
					}
				}
				if priv == "" || pub == "" {
					kp, err := utils.GenerateWireGuardKeyPair()
					if err != nil {
						return nil, err
					}
					priv = kp.Private
					pub = kp.Public
				}

				// merge allowed IPs
				allowed := append([]string{}, node.AllowedIPs...)
				if !cl.RemoveLocalIPFromAllowed {
					allowed = append(allowed, fmt.Sprintf("%s/32", addr.String()))
				}

				gc.Nodes = append(gc.Nodes, GeneratedNode{
					Name:       node.Name,
					Endpoint:   node.Endpoint,
					Port:       port,
					Address:    addr,
					PrivKey:    priv,
					PubKey:     pub,
					AllowedIPs: allowed,
				})
			}

		} else if cl.Mode == "ptp" {
			// Prepare reuse map for tunnels
			for _, cfgNode := range cl.Nodes {
				gc.Nodes = append(gc.Nodes, GeneratedNode{
					Name:       cfgNode.Name,
					Endpoint:   cfgNode.Endpoint,
					AllowedIPs: cfgNode.AllowedIPs,
				})
			}
			nodeIDToName := make(map[uint]string, len(dbC.Nodes))
			for _, dn := range dbC.Nodes {
				nodeIDToName[dn.ID] = dn.Name
			}
			tunnelMap := make(map[string]db.Tunnel, len(dbC.Tunnels))
			for _, t := range dbC.Tunnels {
				from := nodeIDToName[t.FromNodeID]
				to := nodeIDToName[t.ToNodeID]
				tunnelMap[fmt.Sprintf("%s>%s", from, to)] = t
			}

			// Iterate pairs
			n := len(cl.Nodes)
			for i := 0; i < n; i++ {
				for j := i + 1; j < n; j++ {
					src := cl.Nodes[i]
					dst := cl.Nodes[j]
					if contains(src.ExcludePeers, dst.Name) || contains(dst.ExcludePeers, src.Name) {
						continue
					}

					// reuse or allocate
					key := fmt.Sprintf("%s>%s", src.Name, dst.Name)
					var (
						ip1, ip2 net.IP
						p1, p2   int
						kp1, kp2 *utils.KeyPair
					)
					if old, ok := tunnelMap[key]; ok {
						ip1 = net.ParseIP(old.InterfaceIP)
						ip2 = net.ParseIP(old.PeerIP)
						p1 = old.Port
						p2 = old.PeerPort
						kp1 = &utils.KeyPair{Private: old.PrivKey, Public: old.PubKey}
						kp2 = &utils.KeyPair{Private: "", Public: old.PeerPubKey}
						delete(tunnelMap, key)
					} else {
						ip1, ip2, err = ipAlloc.AllocatePair()
						if err != nil {
							return nil, fmt.Errorf("ptp %s-%s: %w", src.Name, dst.Name, err)
						}
						p1, err = portAlloc.Allocate()
						if err != nil {
							return nil, fmt.Errorf("ptp %s: %w", src.Name, err)
						}
						p2, err = portAlloc.Allocate()
						if err != nil {
							return nil, fmt.Errorf("ptp %s: %w", dst.Name, err)
						}
						kp1, err = utils.GenerateWireGuardKeyPair()
						if err != nil {
							return nil, err
						}
						kp2, err = utils.GenerateWireGuardKeyPair()
						if err != nil {
							return nil, err
						}
					}

					// merge allowed IPs
					allowed1 := append([]string{}, src.AllowedIPs...)
					if !cl.RemoveLocalIPFromAllowed {
						allowed1 = append(allowed1, fmt.Sprintf("%s/32", ip1.String()))
					}
					allowed2 := append([]string{}, dst.AllowedIPs...)
					if !cl.RemoveLocalIPFromAllowed {
						allowed2 = append(allowed2, fmt.Sprintf("%s/32", ip2.String()))
					}

					// add two directions
					gc.Tunnels = append(gc.Tunnels, GeneratedTunnel{
						From:        src.Name,
						To:          dst.Name,
						InterfaceIP: ip1, PeerIP: ip2,
						Port: p1, PeerPort: p2,
						PrivKey: kp1.Private, PubKey: kp1.Public, PeerPubKey: kp2.Public,
						AllowedIPs: allowed1,
					}, GeneratedTunnel{
						From:        dst.Name,
						To:          src.Name,
						InterfaceIP: ip2, PeerIP: ip1,
						Port: p2, PeerPort: p1,
						PrivKey: kp2.Private, PubKey: kp2.Public, PeerPubKey: kp1.Public,
						AllowedIPs: allowed2,
					})
				}
			}

		} else {
			return nil, fmt.Errorf("unknown mode %s", cl.Mode)
		}

		gen.Clusters = append(gen.Clusters, gc)
	}

	return gen, nil
}

func contains(slice []string, v string) bool {
	for _, s := range slice {
		if s == v {
			return true
		}
	}
	return false
}
