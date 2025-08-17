package db

import (
	"fmt"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var dbConn *gorm.DB

type DB struct {
	Conn *gorm.DB
}

type Cluster struct {
	ID                       uint   `gorm:"primaryKey"`
	Name                     string `gorm:"unique;not null"`
	Mode                     string `gorm:"not null;index"` // network | ptp
	CIDR                     string `gorm:"not null"`
	PortsRange               string `gorm:"not null"`
	PortsAllocate            string `gorm:"not null"` // linear | random
	RemoveLocalIPFromAllowed bool   `gorm:"default:false"`
	RemoveRoutes             bool   `gorm:"default:false"`
	PersistentKeepalive      int    `gorm:"default:0"`
	CreatedAt, UpdatedAt     time.Time
	Nodes                    []Node   `gorm:"constraint:OnDelete:CASCADE;"`
	Tunnels                  []Tunnel `gorm:"constraint:OnDelete:CASCADE;"`
}

type Node struct {
	ID                   uint     `gorm:"primaryKey"`
	ClusterID            uint     `gorm:"not null;index;uniqueIndex:idx_cluster_node"`
	Name                 string   `gorm:"not null;uniqueIndex:idx_cluster_node"`
	Endpoint             string   `gorm:"not null"`
	Address              string   `gorm:"not null"`
	Port                 int      `gorm:"not null"`
	PrivKey              string   `gorm:"not null"`
	PubKey               string   `gorm:"not null"`
	AllowedIPs           []string `gorm:"type:json;serializer:json"`
	CreatedAt, UpdatedAt time.Time
}

type Tunnel struct {
	ID                   uint   `gorm:"primaryKey"`
	ClusterID            uint   `gorm:"not null;index"`
	FromNodeID           uint   `gorm:"not null;index"`
	ToNodeID             uint   `gorm:"not null;index"`
	InterfaceIP          string `gorm:"not null"`
	PeerIP               string `gorm:"not null"`
	Port                 int    `gorm:"not null"`
	PeerPort             int    `gorm:"not null"`
	PrivKey              string `gorm:"not null"`
	PubKey               string `gorm:"not null"`
	PeerPubKey           string `gorm:"not null"`
	CreatedAt, UpdatedAt time.Time
}

func New(dbPath string) (*DB, error) {
	dbConn, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // Disable all SQL logging
	})
	if err != nil {
		return nil, fmt.Errorf("opening db: %w", err)
	}
	return &DB{Conn: dbConn}, nil
}

func Initialize(dbPath string) error {
	var err error
	dbConn, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("opening db: %w", err)
	}

	return dbConn.AutoMigrate(&Cluster{}, &Node{}, &Tunnel{})
}

// Cleanup removes all data from database and generated files
func (db *DB) Cleanup() error {
	tables := []string{"clusters", "nodes", "tunnels"}
	for _, table := range tables {
		if err := db.Conn.Exec(fmt.Sprintf("DELETE FROM %s", table)).Error; err != nil {
			return fmt.Errorf("failed to delete from %s: %w", table, err)
		}
	}

	return nil
}
