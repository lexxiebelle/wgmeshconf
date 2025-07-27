package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewBackupManager(t *testing.T) {
	backupDir := "test_backups"
	manager := NewBackupManager(backupDir)

	if manager.backupDir != backupDir {
		t.Errorf("Expected backup directory %s, got %s", backupDir, manager.backupDir)
	}
}

func TestCreateBackup(t *testing.T) {
	// Setup test directories
	testDir := t.TempDir()
	backupDir := filepath.Join(testDir, "backups")
	configDir := filepath.Join(testDir, "configs")

	// Create test files
	dbPath := filepath.Join(testDir, "test.db")
	configPath := filepath.Join(testDir, "test.yaml")

	// Create test database file
	if err := os.WriteFile(dbPath, []byte("test database content"), 0644); err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create test config file
	if err := os.WriteFile(configPath, []byte("test config content"), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Create configs directory
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create configs directory: %v", err)
	}

	// Create test config file in configs
	configFile := filepath.Join(configDir, "test.conf")
	if err := os.WriteFile(configFile, []byte("test config file"), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	manager := NewBackupManager(backupDir)

	// Create backup
	backupID, err := manager.CreateBackup(dbPath, configPath, configDir, "Test backup")
	if err != nil {
		t.Fatalf("Failed to create backup: %v", err)
	}

	if backupID == "" {
		t.Error("Expected backup ID, got empty string")
	}

	// Check if backup directory exists
	backupPath := filepath.Join(backupDir, backupID)
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Errorf("Backup directory does not exist: %s", backupPath)
	}

	// Check if files were copied
	backupDB := filepath.Join(backupPath, "database.db")
	if _, err := os.Stat(backupDB); os.IsNotExist(err) {
		t.Errorf("Backup database file does not exist: %s", backupDB)
	}

	backupConfig := filepath.Join(backupPath, "config.yaml")
	if _, err := os.Stat(backupConfig); os.IsNotExist(err) {
		t.Errorf("Backup config file does not exist: %s", backupConfig)
	}

	backupConfigs := filepath.Join(backupPath, "configs")
	if _, err := os.Stat(backupConfigs); os.IsNotExist(err) {
		t.Errorf("Backup configs directory does not exist: %s", backupConfigs)
	}

	// Check metadata
	metadata, err := manager.readMetadata(backupPath)
	if err != nil {
		t.Fatalf("Failed to read metadata: %v", err)
	}

	if metadata.BackupID != backupID {
		t.Errorf("Expected backup ID %s, got %s", backupID, metadata.BackupID)
	}

	if metadata.Description != "Test backup" {
		t.Errorf("Expected description 'Test backup', got %s", metadata.Description)
	}

	// Cleanup
	os.RemoveAll(backupDir)
}

func TestRestoreBackup(t *testing.T) {
	// Setup test directories
	testDir := t.TempDir()
	backupDir := filepath.Join(testDir, "backups")
	configDir := filepath.Join(testDir, "configs")

	// Create test files
	dbPath := filepath.Join(testDir, "test.db")
	configPath := filepath.Join(testDir, "test.yaml")

	// Create test database file
	if err := os.WriteFile(dbPath, []byte("test database content"), 0644); err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create test config file
	if err := os.WriteFile(configPath, []byte("test config content"), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Create configs directory
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create configs directory: %v", err)
	}

	// Create test config file in configs
	configFile := filepath.Join(configDir, "test.conf")
	if err := os.WriteFile(configFile, []byte("test config file"), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	manager := NewBackupManager(backupDir)

	// Create backup
	backupID, err := manager.CreateBackup(dbPath, configPath, configDir, "Test backup")
	if err != nil {
		t.Fatalf("Failed to create backup: %v", err)
	}

	// Remove original files
	os.Remove(dbPath)
	os.Remove(configPath)
	os.RemoveAll(configDir)

	// Restore backup
	if err := manager.RestoreBackup(backupID, dbPath, configPath, configDir); err != nil {
		t.Fatalf("Failed to restore backup: %v", err)
	}

	// Check if files were restored
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("Restored database file does not exist: %s", dbPath)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("Restored config file does not exist: %s", configPath)
	}

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Errorf("Restored configs directory does not exist: %s", configDir)
	}

	// Check content of restored files
	dbContent, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("Failed to read restored database: %v", err)
	}
	if string(dbContent) != "test database content" {
		t.Errorf("Restored database content does not match")
	}

	configContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read restored config: %v", err)
	}
	if string(configContent) != "test config content" {
		t.Errorf("Restored config content does not match")
	}

	// Cleanup
	os.RemoveAll(backupDir)
}

func TestListBackups(t *testing.T) {
	// Setup test directories
	testDir := t.TempDir()
	backupDir := filepath.Join(testDir, "backups")

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatalf("Failed to create backup directory: %v", err)
	}

	manager := NewBackupManager(backupDir)

	// Create test files for backup
	dbPath := filepath.Join(testDir, "test.db")
	configPath := filepath.Join(testDir, "test.yaml")
	configDir := filepath.Join(testDir, "configs")

	if err := os.WriteFile(dbPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test files: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test files: %v", err)
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create test files: %v", err)
	}

	// Create multiple backups
	backup1, err := manager.CreateBackup(dbPath, configPath, configDir, "First backup")
	if err != nil {
		t.Fatalf("Failed to create first backup: %v", err)
	}

	time.Sleep(1 * time.Second) // Ensure different timestamps

	backup2, err := manager.CreateBackup(dbPath, configPath, configDir, "Second backup")
	if err != nil {
		t.Fatalf("Failed to create second backup: %v", err)
	}

	// List backups
	backups, err := manager.ListBackups()
	if err != nil {
		t.Fatalf("Failed to list backups: %v", err)
	}

	if len(backups) != 2 {
		t.Errorf("Expected 2 backups, got %d", len(backups))
	}

	// Check if both backups are in the list
	foundBackup1 := false
	foundBackup2 := false
	for _, backup := range backups {
		if backup.BackupID == backup1 {
			foundBackup1 = true
		}
		if backup.BackupID == backup2 {
			foundBackup2 = true
		}
	}

	if !foundBackup1 {
		t.Error("First backup not found in list")
	}
	if !foundBackup2 {
		t.Error("Second backup not found in list")
	}

	// Cleanup
	os.RemoveAll(backupDir)
}

func TestDeleteBackup(t *testing.T) {
	// Setup test directories
	testDir := t.TempDir()
	backupDir := filepath.Join(testDir, "backups")

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatalf("Failed to create backup directory: %v", err)
	}

	manager := NewBackupManager(backupDir)

	// Create test files for backup
	dbPath := filepath.Join(testDir, "test.db")
	configPath := filepath.Join(testDir, "test.yaml")
	configDir := filepath.Join(testDir, "configs")

	if err := os.WriteFile(dbPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test files: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test files: %v", err)
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create test files: %v", err)
	}

	// Create backup
	backupID, err := manager.CreateBackup(dbPath, configPath, configDir, "Test backup")
	if err != nil {
		t.Fatalf("Failed to create backup: %v", err)
	}

	// Verify backup exists
	backupPath := filepath.Join(backupDir, backupID)
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Errorf("Backup directory does not exist before deletion: %s", backupPath)
	}

	// Delete backup
	if err := manager.DeleteBackup(backupID); err != nil {
		t.Fatalf("Failed to delete backup: %v", err)
	}

	// Verify backup is deleted
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Errorf("Backup directory still exists after deletion: %s", backupPath)
	}

	// Cleanup
	os.RemoveAll(backupDir)
}

func TestGetLatestBackup(t *testing.T) {
	// Setup test directories
	testDir := t.TempDir()
	backupDir := filepath.Join(testDir, "backups")

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatalf("Failed to create backup directory: %v", err)
	}

	manager := NewBackupManager(backupDir)

	// Create test files for backup
	dbPath := filepath.Join(testDir, "test.db")
	configPath := filepath.Join(testDir, "test.yaml")
	configDir := filepath.Join(testDir, "configs")

	if err := os.WriteFile(dbPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test files: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test files: %v", err)
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create test files: %v", err)
	}

	// Create backup
	backupID, err := manager.CreateBackup(dbPath, configPath, configDir, "Test backup")
	if err != nil {
		t.Fatalf("Failed to create backup: %v", err)
	}

	// Get latest backup
	latestBackup, err := manager.GetLatestBackup()
	if err != nil {
		t.Fatalf("Failed to get latest backup: %v", err)
	}

	if latestBackup != backupID {
		t.Errorf("Expected latest backup %s, got %s", backupID, latestBackup)
	}

	// Cleanup
	os.RemoveAll(backupDir)
}
