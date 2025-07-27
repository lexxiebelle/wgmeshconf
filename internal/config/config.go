package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Node describes a single WireGuard endpoint
type Node struct {
	Name         string   `yaml:"name"`
	Endpoint     string   `yaml:"endpoint"`
	Port         int      `yaml:"port,omitempty"` // Optional, will be auto-assigned if not specified
	BindAddress  string   `yaml:"bindAddress,omitempty"`
	AllowedIPs   []string `yaml:"allowedIps,omitempty"`
	ExcludePeers []string `yaml:"excludePeers,omitempty"`
}

// Cluster defines one cluster section in config.yaml
type Cluster struct {
	Name                     string `yaml:"name"`
	Mode                     string `yaml:"mode"` // "network" or "ptp"
	CIDR                     string `yaml:"cidr"`
	PortsRange               string `yaml:"portsRange,omitempty"`    // e.g., "20000-21000"
	PortsAllocate            string `yaml:"portsAllocate,omitempty"` // "linear" or "random", default "random"
	RemoveLocalIPFromAllowed bool   `yaml:"removeLocalIPFromAllowed,omitempty"`
	PersistentKeepalive      int    `yaml:"persistentKeepalive,omitempty"`
	Nodes                    []Node `yaml:"nodes"`
}

// Config is the top-level config structure
type Config struct {
	Clusters []Cluster `yaml:"clusters"`
	Path     string
}

// Load reads and parses the YAML config
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	cfg.Path = path
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Set defaults for all clusters
	for i := range cfg.Clusters {
		cfg.Clusters[i].SetDefaults()
	}

	return &cfg, nil
}

func (c *Cluster) SetDefaults() {
	if c.PortsRange == "" {
		c.PortsRange = "20000-22000"
	}
	if c.PortsAllocate == "" {
		c.PortsAllocate = "random"
	}
}

func CreateSample(path string) error {
	sampleConfig := `clusters:
	  - name: example
		mode: ptp
		cidr: 172.16.20.0/24
		removeLocalIPFromAllowed: false
		nodes:
		  - name: node1
			endpoint: 1.2.3.4
			port: 51820
			allowedIps: [10.0.1.0/24]
			excludePeers: [node2]
		  - name: node2
			endpoint: 5.6.7.8
			port: 51820
			allowedIps: [10.0.2.0/24]
			excludePeers: [node1]
	`

	return os.WriteFile(path, []byte(sampleConfig), 0644)
}
