package backup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	MaxBackups = 10 // Maximum number of backups to keep
)

// BackupManager manages backup operations
type BackupManager struct {
	backupDir string
}

// BackupMetadata contains information about a backup
type BackupMetadata struct {
	Timestamp   time.Time `json:"timestamp"`
	BackupID    string    `json:"backup_id"`
	Command     string    `json:"command"`
	Description string    `json:"description"`
	Files       []string  `json:"files"`
	Size        int64     `json:"size"`
}

// NewBackupManager creates a new backup manager
func NewBackupManager(backupDir string) *BackupManager {
	return &BackupManager{
		backupDir: backupDir,
	}
}

// CreateBackup creates a new backup
func (bm *BackupManager) CreateBackup(dbPath, configPath, configsDir string, description string) (string, error) {
	// Generate backup ID with timestamp
	timestamp := time.Now()

	// Determine command type based on description
	commandType := "apply"
	if description != "Auto backup after apply" {
		commandType = "manual"
	}

	backupID := fmt.Sprintf("%s_%s",
		timestamp.Format("2006-01-02_15-04-05"),
		commandType)

	backupPath := filepath.Join(bm.backupDir, backupID)

	// Create backup directory
	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Copy database
	if err := bm.copyFile(dbPath, filepath.Join(backupPath, "database.db")); err != nil {
		return "", fmt.Errorf("failed to backup database: %w", err)
	}

	// Copy config file
	if err := bm.copyFile(configPath, filepath.Join(backupPath, "config.yaml")); err != nil {
		return "", fmt.Errorf("failed to backup config: %w", err)
	}

	// Copy configs directory if it exists
	configsBackupPath := filepath.Join(backupPath, "configs")
	if _, err := os.Stat(configsDir); err == nil {
		if err := bm.copyDirectory(configsDir, configsBackupPath); err != nil {
			return "", fmt.Errorf("failed to backup configs: %w", err)
		}
	}

	// Create metadata
	metadata := BackupMetadata{
		Timestamp:   timestamp,
		BackupID:    backupID,
		Command:     commandType,
		Description: description,
		Files:       []string{"database.db", "config.yaml", "configs/"},
		Size:        bm.calculateBackupSize(backupPath),
	}

	if err := bm.writeMetadata(backupPath, metadata); err != nil {
		return "", fmt.Errorf("failed to write metadata: %w", err)
	}

	// Update latest symlink
	if err := bm.updateLatestSymlink(backupID); err != nil {
		return "", fmt.Errorf("failed to update latest symlink: %w", err)
	}

	// Cleanup old backups if we exceed the limit
	if err := bm.cleanupOldBackups(); err != nil {
		return "", fmt.Errorf("failed to cleanup old backups: %w", err)
	}

	return backupID, nil
}

// RestoreBackup restores a backup
func (bm *BackupManager) RestoreBackup(backupID, dbPath, configPath, configsDir string) error {
	backupPath := filepath.Join(bm.backupDir, backupID)

	// Check if backup exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup %s does not exist", backupID)
	}

	// Restore database
	if err := bm.copyFile(filepath.Join(backupPath, "database.db"), dbPath); err != nil {
		return fmt.Errorf("failed to restore database: %w", err)
	}

	// Restore config
	if err := bm.copyFile(filepath.Join(backupPath, "config.yaml"), configPath); err != nil {
		return fmt.Errorf("failed to restore config: %w", err)
	}

	// Restore configs directory
	configsBackupPath := filepath.Join(backupPath, "configs")
	if _, err := os.Stat(configsBackupPath); err == nil {
		// Remove existing configs directory
		if err := os.RemoveAll(configsDir); err != nil {
			return fmt.Errorf("failed to remove existing configs: %w", err)
		}

		// Copy backup configs
		if err := bm.copyDirectory(configsBackupPath, configsDir); err != nil {
			return fmt.Errorf("failed to restore configs: %w", err)
		}
	}

	return nil
}

// ListBackups lists all available backups
func (bm *BackupManager) ListBackups() ([]BackupMetadata, error) {
	var backups []BackupMetadata

	entries, err := os.ReadDir(bm.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return backups, nil // No backups yet
		}
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			backupPath := filepath.Join(bm.backupDir, entry.Name())
			metadata, err := bm.readMetadata(backupPath)
			if err != nil {
				continue // Skip invalid backups
			}
			backups = append(backups, metadata)
		}
	}

	return backups, nil
}

// GetLatestBackup returns the latest backup ID
func (bm *BackupManager) GetLatestBackup() (string, error) {
	latestPath := filepath.Join(bm.backupDir, "latest")

	link, err := os.Readlink(latestPath)
	if err != nil {
		return "", fmt.Errorf("no latest backup found: %w", err)
	}

	return filepath.Base(link), nil
}

// DeleteBackup deletes a backup
func (bm *BackupManager) DeleteBackup(backupID string) error {
	backupPath := filepath.Join(bm.backupDir, backupID)

	if err := os.RemoveAll(backupPath); err != nil {
		return fmt.Errorf("failed to delete backup: %w", err)
	}

	// Update latest symlink if this was the latest backup
	latestBackup, err := bm.GetLatestBackup()
	if err == nil && latestBackup == backupID {
		if err := bm.updateLatestSymlinkFromList(); err != nil {
			return fmt.Errorf("failed to update latest symlink: %w", err)
		}
	}

	return nil
}

// CleanupOldBackups removes backups older than specified days
func (bm *BackupManager) CleanupOldBackups(days int) error {
	backups, err := bm.ListBackups()
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -days)

	for _, backup := range backups {
		if backup.Timestamp.Before(cutoff) {
			if err := bm.DeleteBackup(backup.BackupID); err != nil {
				return fmt.Errorf("failed to delete old backup %s: %w", backup.BackupID, err)
			}
		}
	}

	return nil
}

// Helper methods

func (bm *BackupManager) copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, input, 0644)
}

func (bm *BackupManager) copyDirectory(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		return bm.copyFile(path, dstPath)
	})
}

func (bm *BackupManager) writeMetadata(backupPath string, metadata BackupMetadata) error {
	metadataPath := filepath.Join(backupPath, "metadata.json")

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metadataPath, data, 0644)
}

func (bm *BackupManager) readMetadata(backupPath string) (BackupMetadata, error) {
	metadataPath := filepath.Join(backupPath, "metadata.json")

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return BackupMetadata{}, err
	}

	var metadata BackupMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return BackupMetadata{}, err
	}

	return metadata, nil
}

func (bm *BackupManager) calculateBackupSize(backupPath string) int64 {
	var size int64

	filepath.Walk(backupPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size
}

func (bm *BackupManager) updateLatestSymlink(backupID string) error {
	latestPath := filepath.Join(bm.backupDir, "latest")

	// Remove existing symlink if it exists
	if _, err := os.Lstat(latestPath); err == nil {
		os.Remove(latestPath)
	}

	// Create new symlink
	return os.Symlink(backupID, latestPath)
}

func (bm *BackupManager) updateLatestSymlinkFromList() error {
	backups, err := bm.ListBackups()
	if err != nil {
		return err
	}

	if len(backups) == 0 {
		return nil
	}

	// Find the most recent backup
	var latest BackupMetadata
	for _, backup := range backups {
		if backup.Timestamp.After(latest.Timestamp) {
			latest = backup
		}
	}

	return bm.updateLatestSymlink(latest.BackupID)
}

func (bm *BackupManager) cleanupOldBackups() error {
	backups, err := bm.ListBackups()
	if err != nil {
		return fmt.Errorf("failed to list backups for cleanup: %w", err)
	}

	// If we have more backups than the limit, remove the oldest ones
	if len(backups) > MaxBackups {
		// Sort backups by creation time (oldest first)
		sort.Slice(backups, func(i, j int) bool {
			return backups[i].Timestamp.Before(backups[j].Timestamp)
		})

		// Remove oldest backups
		backupsToRemove := len(backups) - MaxBackups
		for i := 0; i < backupsToRemove; i++ {
			if err := bm.DeleteBackup(backups[i].BackupID); err != nil {
				return fmt.Errorf("failed to remove old backup %s: %w", backups[i].BackupID, err)
			}
		}
	}

	return nil
}
