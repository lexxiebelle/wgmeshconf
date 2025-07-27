package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/lexxiebelle/wgmeshconf/internal/db"
	"github.com/lexxiebelle/wgmeshconf/internal/utils"
	"github.com/olekukonko/tablewriter"
)

func (a *App) statusCommand(showKeys bool) error {
	// 1) Load all clusters with nodes
	var clusters []db.Cluster
	if err := a.db.Conn.Preload("Nodes").Find(&clusters).Error; err != nil {
		return fmt.Errorf("load clusters: %w", err)
	}
	if len(clusters) == 0 {
		fmt.Println("No clusters found in database.")
		return nil
	}

	// Node dictionary by ID for tunnel lookups
	nodeMap := map[uint]db.Node{}
	for _, c := range clusters {
		for _, n := range c.Nodes {
			nodeMap[n.ID] = n
		}
	}

	// 2) Table "Clusters & Nodes"
	fmt.Println("\nClusters and Nodes:")
	tbl := tablewriter.NewWriter(os.Stdout)
	tbl.Header([]string{"Cluster", "Mode", "Node", "Endpoint", "Address", "Port", "Allowed", "PrivKey", "PubKey"})
	for ci, c := range clusters {
		for ni, n := range c.Nodes {
			row := []string{}
			if ni == 0 {
				row = append(row, c.Name, c.Mode)
			} else {
				row = append(row, "", "")
			}
			priv := n.PrivKey
			pub := n.PubKey
			if !showKeys {
				if len(priv) > 10 {
					priv = priv[:10] + "..."
				}
				if len(pub) > 10 {
					pub = pub[:10] + "..."
				}
			}
			row = append(row, n.Name, n.Endpoint, n.Address, fmt.Sprintf("%d", n.Port), strings.Join(n.AllowedIPs, ", "), priv, pub)
			if n.Address == "<nil>" || n.Address == "" {
				row[4] = "--"
			}

			if n.Port == 0 {
				row[5] = "--"
			}
			if len(n.AllowedIPs) == 0 {
				row[6] = "--"
			}
			if n.PrivKey == "" {
				row[7] = "--"
			}
			if n.PubKey == "" {
				row[8] = "--"
			}
			tbl.Append(row)
		}
		if ci < len(clusters)-1 {
			tbl.Append([]string{"", "", "", "", "", ""})
		}
	}
	tbl.Render()

	// 3) Table "Tunnels (PTP mode)"
	fmt.Println("\nTunnels (PTP mode):")
	tt := tablewriter.NewWriter(os.Stdout)
	tt.Header([]string{
		"Cluster", "From→To",
		"Local IP", "Peer IP",
		"Local Port", "Peer Port", "Allowed",
		"PrivKey", "PubKey", "PeerPubKey",
	})

	found := false
	for ci, c := range clusters {
		if c.Mode != "ptp" {
			continue
		}
		var tunnels []db.Tunnel
		if err := a.db.Conn.Where("cluster_id = ?", c.ID).Find(&tunnels).Error; err != nil {
			return fmt.Errorf("load tunnels for %s: %w", c.Name, err)
		}
		for _, t := range tunnels {
			fromNode := nodeMap[t.FromNodeID]
			toNode := nodeMap[t.ToNodeID]
			name := fmt.Sprintf("%s→%s", fromNode.Name, toNode.Name)

			priv := t.PrivKey
			pub := t.PubKey
			ppub := t.PeerPubKey
			if !showKeys {
				if len(priv) > 10 {
					priv = priv[:10] + "..."
				}
				if len(pub) > 10 {
					pub = pub[:10] + "..."
				}
				if len(ppub) > 10 {
					ppub = ppub[:10] + "..."
				}
			}

			allowed := []string{}
			if len(toNode.AllowedIPs) > 0 {
				allowed = toNode.AllowedIPs
			}

			if !c.RemoveLocalIPFromAllowed {
				allowed = append(allowed, t.PeerIP)
			}

			tt.Append([]string{
				c.Name,
				name,
				t.InterfaceIP,
				t.PeerIP,
				fmt.Sprintf("%d", t.Port),
				fmt.Sprintf("%d", t.PeerPort),
				strings.Join(allowed, ", "),
				priv,
				pub,
				ppub,
			})
			found = true
		}
		// need select only ptp clusters
		ptpClusters := utils.Filter(clusters, func(c db.Cluster) bool {
			return c.Mode == "ptp"
		})
		if ci < len(ptpClusters)-1 {
			tt.Append([]string{"", "", "", "", "", "", "", "", ""})
		}
	}
	if !found {
		tt.Append([]string{"No tunnels", "", "", "", "", "", "", "", ""})
	}
	tt.Render()

	return nil
}
