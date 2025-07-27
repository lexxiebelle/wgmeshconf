package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/lexxiebelle/wgmeshconf/internal/backup"
	"github.com/lexxiebelle/wgmeshconf/internal/config"
	"github.com/lexxiebelle/wgmeshconf/internal/db"
	"github.com/lexxiebelle/wgmeshconf/internal/generator"
	"github.com/lexxiebelle/wgmeshconf/internal/utils"
)

// applyCommand applies configuration changes to the database
func (a *App) applyCommand() error {
	// 1) Load existing clusters (with nodes and tunnels) from DB
	var existing []db.Cluster
	if err := a.db.Conn.
		Preload("Nodes").
		Preload("Tunnels").
		Find(&existing).Error; err != nil {
		return fmt.Errorf("load existing clusters: %w", err)
	}

	// 2) Generate desired state, reusing DB state
	genCfg, err := generator.Generate(a.config, existing)
	if err != nil {
		return fmt.Errorf("generation failed: %w", err)
	}

	// 3) Begin transaction
	tx := a.db.Conn.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	// 4) Load clusters currently in DB
	var dbClusters []db.Cluster
	if err := tx.Preload("Nodes").Preload("Tunnels").Find(&dbClusters).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("load clusters: %w", err)
	}

	// 5) Compare clusters
	dbByName := make(map[string]db.Cluster, len(dbClusters))
	for _, c := range dbClusters {
		dbByName[c.Name] = c
	}
	genByName := make(map[string]generator.GeneratedCluster, len(genCfg.Clusters))
	for _, gc := range genCfg.Clusters {
		genByName[gc.Name] = gc
	}

	var toAddC, toUpdateC []generator.GeneratedCluster
	var toDeleteC []db.Cluster

	for name, genC := range genByName {
		if _, ok := dbByName[name]; !ok {
			toAddC = append(toAddC, genC)
		} else {
			dbC := dbByName[name]
			cfg := a.findClusterConfig(name)
			// only these fields may change
			if dbC.Mode != genC.Mode || dbC.CIDR != genC.CIDR {
				tx.Rollback()
				return fmt.Errorf(
					"cluster %q: only PortsRange, PortsAllocate and RemoveLocalIPFromAllowed can be changed; mode/CIDR require recreate or cleanup",
					name,
				)
			}
			if dbC.PortsRange != cfg.PortsRange ||
				dbC.PortsAllocate != cfg.PortsAllocate ||
				dbC.PersistentKeepalive != cfg.PersistentKeepalive ||
				dbC.RemoveLocalIPFromAllowed != cfg.RemoveLocalIPFromAllowed {
				toUpdateC = append(toUpdateC, genC)
			}
		}
	}
	for _, dbC := range dbClusters {
		if _, ok := genByName[dbC.Name]; !ok {
			toDeleteC = append(toDeleteC, dbC)
		}
	}

	// 6) Confirm cluster changes
	var parts []string
	if len(toAddC) > 0 {
		parts = append(parts, fmt.Sprintf("+%d add", len(toAddC)))
	}
	if len(toUpdateC) > 0 {
		parts = append(parts, fmt.Sprintf("~%d update", len(toUpdateC)))
	}
	if len(toDeleteC) > 0 {
		parts = append(parts, fmt.Sprintf("-%d delete", len(toDeleteC)))
	}
	if len(parts) > 0 {
		prompt := fmt.Sprintf("Clusters changes: %s. Proceed?", strings.Join(parts, ", "))
		if !a.askConfirmation(prompt) {
			tx.Rollback()
			fmt.Println("Operation cancelled")
			return nil
		}
	}

	fmt.Printf("Creating backup before apply...\n")
	backupManager := backup.NewBackupManager("backups")
	backupPath, err := backupManager.CreateBackup(a.dbPath, a.configPath, "configs", "Auto backup after apply")
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	fmt.Printf("Backup created successfully: %s\n", backupPath)

	// 7) Apply cluster additions/updates/deletions
	for _, genC := range toAddC {
		cfg := a.findClusterConfig(genC.Name)
		newC := db.Cluster{
			Name:                     genC.Name,
			Mode:                     genC.Mode,
			CIDR:                     genC.CIDR,
			PortsRange:               cfg.PortsRange,
			PortsAllocate:            cfg.PortsAllocate,
			RemoveLocalIPFromAllowed: cfg.RemoveLocalIPFromAllowed,
			PersistentKeepalive:      cfg.PersistentKeepalive,
			CreatedAt:                time.Now(),
			UpdatedAt:                time.Now(),
		}
		if err := tx.Create(&newC).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("create cluster %s: %w", genC.Name, err)
		}
	}
	for _, genC := range toUpdateC {
		cfg := a.findClusterConfig(genC.Name)
		upd := db.Cluster{
			PortsRange:               cfg.PortsRange,
			PortsAllocate:            cfg.PortsAllocate,
			RemoveLocalIPFromAllowed: cfg.RemoveLocalIPFromAllowed,
			PersistentKeepalive:      cfg.PersistentKeepalive,
			UpdatedAt:                time.Now(),
		}
		if err := tx.Model(&db.Cluster{}).
			Where("name = ?", genC.Name).
			Select("ports_range", "ports_allocate", "remove_local_ip_from_allowed", "persistent_keepalive", "updated_at").
			Updates(upd).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("update cluster %s: %w", genC.Name, err)
		}
	}
	for _, delC := range toDeleteC {
		if err := tx.Delete(&db.Cluster{}, delC.ID).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("delete cluster %s: %w", delC.Name, err)
		}
	}

	// 8) Synchronize nodes per cluster
	for _, genC := range genCfg.Clusters {
		// load this cluster
		var dbC db.Cluster
		if err := tx.Where("name = ?", genC.Name).First(&dbC).Error; err != nil {
			tx.Rollback()
			return err
		}
		// load existing nodes
		var dbNodes []db.Node
		if err := tx.Where("cluster_id = ?", dbC.ID).Find(&dbNodes).Error; err != nil {
			tx.Rollback()
			return err
		}
		dbN := make(map[string]db.Node, len(dbNodes))
		for _, n := range dbNodes {
			dbN[n.Name] = n
		}

		// compute differences
		genN := make(map[string]generator.GeneratedNode, len(genC.Nodes))
		for _, gn := range genC.Nodes {
			genN[gn.Name] = gn
		}
		var toAddN, toUpdateN []generator.GeneratedNode
		var toDeleteN []db.Node

		for name, gn := range genN {
			if dn, ok := dbN[name]; !ok {
				toAddN = append(toAddN, gn)
			} else {
				need := dn.Endpoint != gn.Endpoint ||
					!utils.EqualStringSlices(dn.AllowedIPs, gn.AllowedIPs)
				if genC.Mode == "network" {
					need = need || dn.Port != gn.Port || dn.Address != gn.Address.String()
				}
				if need {
					toUpdateN = append(toUpdateN, gn)
				}
			}
		}
		for _, dn := range dbNodes {
			if _, ok := genN[dn.Name]; !ok {
				toDeleteN = append(toDeleteN, dn)
			}
		}

		// confirm node changes
		parts = parts[:0]
		if len(toAddN) > 0 {
			parts = append(parts, fmt.Sprintf("+%d add", len(toAddN)))
		}
		if len(toUpdateN) > 0 {
			parts = append(parts, fmt.Sprintf("~%d update", len(toUpdateN)))
		}
		if len(toDeleteN) > 0 {
			parts = append(parts, fmt.Sprintf("-%d delete", len(toDeleteN)))
		}
		if len(parts) > 0 {
			prompt := fmt.Sprintf("Cluster %s nodes changes: %s. Proceed?",
				genC.Name, strings.Join(parts, ", "))
			if !a.askConfirmation(prompt) {
				tx.Rollback()
				fmt.Println("Operation cancelled")
				return nil
			}
		}

		// apply node additions
		for _, gn := range toAddN {
			nodeRec := db.Node{
				ClusterID:  dbC.ID,
				Name:       gn.Name,
				Endpoint:   gn.Endpoint,
				Address:    gn.Address.String(),
				Port:       gn.Port,
				PrivKey:    gn.PrivKey,
				PubKey:     gn.PubKey,
				AllowedIPs: gn.AllowedIPs,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}
			if err := tx.Create(&nodeRec).Error; err != nil {
				tx.Rollback()
				return err
			}
		}
		// apply node updates
		for _, gn := range toUpdateN {
			upd := db.Node{
				Endpoint:   gn.Endpoint,
				AllowedIPs: gn.AllowedIPs,
				UpdatedAt:  time.Now(),
			}
			fields := []string{"endpoint", "allowed_ips", "updated_at"}
			if genC.Mode == "network" {
				upd.Address = gn.Address.String()
				upd.Port = gn.Port
				fields = append(fields, "address", "port")
			}
			if err := tx.Model(&db.Node{}).
				Where("cluster_id = ? AND name = ?", dbC.ID, gn.Name).
				Select(fields).
				Updates(upd).Error; err != nil {
				tx.Rollback()
				return err
			}
		}
		// apply node deletions
		for _, dn := range toDeleteN {
			if err := tx.Delete(&db.Node{}, dn.ID).Error; err != nil {
				tx.Rollback()
				return err
			}
		}

		// prepare set of node names that actually changed (to control tunnel updates)
		changed := make(map[string]struct{}, len(toUpdateN))
		for _, gn := range toUpdateN {
			changed[gn.Name] = struct{}{}
		}

		// 9) synchronize tunnels (PTP only)
		if genC.Mode == "ptp" {
			// load existing tunnels
			var existingT []db.Tunnel
			if err := tx.Where("cluster_id = ?", dbC.ID).Find(&existingT).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("load tunnels: %w", err)
			}
			exMap := make(map[string]db.Tunnel, len(existingT))
			for _, t := range existingT {
				key := fmt.Sprintf("%d>%d", t.FromNodeID, t.ToNodeID)
				exMap[key] = t
			}

			// for each generated tunnel
			for _, gt := range genC.Tunnels {
				// skip update if neither endpoint nor allowedIPs changed on either node
				if _, fromCh := changed[gt.From]; !fromCh {
					if _, toCh := changed[gt.To]; !toCh {
						// if it already exists, just consume it and continue
						// (we don't want to update unchanged tunnels)
						// use name to ID mapping
						var fromN, toN db.Node
						tx.Where("cluster_id = ? AND name = ?", dbC.ID, gt.From).First(&fromN)
						tx.Where("cluster_id = ? AND name = ?", dbC.ID, gt.To).First(&toN)
						key := fmt.Sprintf("%d>%d", fromN.ID, toN.ID)
						if _, ok := exMap[key]; ok {
							delete(exMap, key)
							continue
						}
					}
				}

				// lookup database node IDs
				var fromN, toN db.Node
				if err := tx.Where("cluster_id = ? AND name = ?", dbC.ID, gt.From).First(&fromN).Error; err != nil {
					tx.Rollback()
					return err
				}
				if err := tx.Where("cluster_id = ? AND name = ?", dbC.ID, gt.To).First(&toN).Error; err != nil {
					tx.Rollback()
					return err
				}

				key := fmt.Sprintf("%d>%d", fromN.ID, toN.ID)
				if old, ok := exMap[key]; ok {
					// only update if IPs or ports actually differ
					newIf := gt.InterfaceIP.String()
					newPeer := gt.PeerIP.String()
					if old.InterfaceIP != newIf ||
						old.PeerIP != newPeer ||
						old.Port != gt.Port ||
						old.PeerPort != gt.PeerPort {
						upd := db.Tunnel{
							InterfaceIP: newIf,
							PeerIP:      newPeer,
							Port:        gt.Port,
							PeerPort:    gt.PeerPort,
							UpdatedAt:   time.Now(),
						}
						if err := tx.Model(&db.Tunnel{}).
							Where("id = ?", old.ID).
							Select("interface_ip", "peer_ip", "port", "peer_port", "updated_at").
							Updates(upd).Error; err != nil {
							tx.Rollback()
							return err
						}
					}
					delete(exMap, key)
				} else {
					// insert new tunnel
					newT := db.Tunnel{
						ClusterID:   dbC.ID,
						FromNodeID:  fromN.ID,
						ToNodeID:    toN.ID,
						InterfaceIP: gt.InterfaceIP.String(),
						PeerIP:      gt.PeerIP.String(),
						Port:        gt.Port,
						PeerPort:    gt.PeerPort,
						PrivKey:     gt.PrivKey,
						PubKey:      gt.PubKey,
						PeerPubKey:  gt.PeerPubKey,
						CreatedAt:   time.Now(),
						UpdatedAt:   time.Now(),
					}
					if err := tx.Create(&newT).Error; err != nil {
						tx.Rollback()
						return err
					}
				}
			}

			// delete any stale tunnels
			for _, stale := range exMap {
				if err := tx.Delete(&db.Tunnel{}, stale.ID).Error; err != nil {
					tx.Rollback()
					return err
				}
			}
		}
	}

	// 10) Commit transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("apply commit failed: %w", err)
	}
	fmt.Println("Apply complete.")
	return nil
}

func (a *App) findClusterConfig(name string) *config.Cluster {
	for i := range a.config.Clusters {
		if a.config.Clusters[i].Name == name {
			return &a.config.Clusters[i]
		}
	}
	return nil
}
