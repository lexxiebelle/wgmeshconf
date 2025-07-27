package app

import (
	"fmt"
	"os"

	"github.com/lexxiebelle/wgmeshconf/internal/backup"
	"github.com/olekukonko/tablewriter"
)

// backupCommand creates a backup of current state
func (a *App) backupCommand() error {
	fmt.Printf("Creating backup...\n")

	// Create backup manager
	backupManager := backup.NewBackupManager("backups")

	// Create backup
	backupPath, err := backupManager.CreateBackup(a.dbPath, a.configPath, "configs", "Manual backup")
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	fmt.Printf("Backup created successfully: %s\n", backupPath)
	return nil
}

// restoreCommand restores from a backup
func (a *App) restoreCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("backup name required. Use 'wgmeshconf backups' to list available backups")
	}

	backupName := args[0]
	fmt.Printf("Restoring from backup: %s\n", backupName)

	// Create backup manager
	backupManager := backup.NewBackupManager("backups")

	// Restore backup
	if err := backupManager.RestoreBackup(backupName, a.dbPath, a.configPath, "configs"); err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	fmt.Printf("Backup restored successfully!\n")
	return nil
}

// listBackupsCommand lists available backups
func (a *App) listBackupsCommand() error {
	fmt.Printf("Available backups:\n")

	// Create backup manager
	backupManager := backup.NewBackupManager("backups")

	// List backups
	backups, err := backupManager.ListBackups()
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	if len(backups) == 0 {
		fmt.Printf("No backups found.\n")
		return nil
	}

	// Create table
	table := tablewriter.NewWriter(os.Stdout)
	table.Header("Backup Name", "Created", "Size")

	// Add rows
	for _, backup := range backups {
		table.Append(backup.BackupID, backup.Timestamp.Format("2006-01-02 15:04:05"), fmt.Sprintf("%d", backup.Size))
	}

	table.Render()
	return nil
}
