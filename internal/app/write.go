package app

import (
	"fmt"

	"github.com/lexxiebelle/wgmeshconf/internal/db"
	"github.com/lexxiebelle/wgmeshconf/internal/writer"
)

const configsDir = "configs"

// writeCommand generates wg-config files under configs/
func (a *App) writeCommand() error {
	var clusters []db.Cluster
	if err := a.db.Conn.
		Preload("Nodes").
		Preload("Tunnels").
		Find(&clusters).Error; err != nil {
		return fmt.Errorf("load clusters: %w", err)
	}

	files, err := writer.GenerateConfigs(clusters)
	if err != nil {
		return fmt.Errorf("generate configs: %w", err)
	}

	if err := writer.WriteConfigs(configsDir, files); err != nil {
		return fmt.Errorf("write configs: %w", err)
	}

	if len(clusters) == 0 {
		fmt.Println("No clusters found in database, delete configs directory")
	} else {
		fmt.Printf("Configs successfully generated in directory %q\n", configsDir)
	}
	return nil
}
