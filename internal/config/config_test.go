package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Create a temporary test config file
	testConfig := `clusters:
  - name: example
    mode: ptp
    cidr: 172.16.20.0/24
    removeLocalIPFromAllowed: false
    removeRoutes: false
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

	tmpFile := "test_config.yaml"
	err := os.WriteFile(tmpFile, []byte(testConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}
	defer os.Remove(tmpFile)

	// Test loading valid config
	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if len(cfg.Clusters) == 0 {
		t.Fatal("Expected clusters to be loaded")
	}

	// Test first cluster
	cluster := cfg.Clusters[0]
	if cluster.Name != "example" {
		t.Errorf("Expected cluster name 'example', got '%s'", cluster.Name)
	}

	if cluster.Mode != "ptp" {
		t.Errorf("Expected cluster mode 'ptp', got '%s'", cluster.Mode)
	}

	if cluster.CIDR != "172.16.20.0/24" {
		t.Errorf("Expected CIDR '172.16.20.0/24', got '%s'", cluster.CIDR)
	}

	// Test nodes
	if len(cluster.Nodes) == 0 {
		t.Fatal("Expected nodes to be loaded")
	}

	node := cluster.Nodes[0]
	if node.Name != "node1" {
		t.Errorf("Expected node name 'node1', got '%s'", node.Name)
	}

	if node.Endpoint != "1.2.3.4" {
		t.Errorf("Expected endpoint '1.2.3.4', got '%s'", node.Endpoint)
	}

	if node.Port != 51820 {
		t.Errorf("Expected port 51820, got %d", node.Port)
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	_, err := Load("non_existent_file.yaml")
	if err == nil {
		t.Fatal("Expected error when loading non-existent file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	// Create temporary file with invalid YAML
	tmpFile := "test_invalid.yaml"
	content := `invalid: yaml: content: [`
	defer os.Remove(tmpFile)

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err = Load(tmpFile)
	if err == nil {
		t.Fatal("Expected error when loading invalid YAML")
	}
}

func TestLoadNetworkConfig(t *testing.T) {
	// Create a temporary network config file
	testConfig := `clusters:
  - name: network_example
    mode: network
    cidr: 172.16.20.0/24
    removePeerFromAllowed: false
    nodes:
      - name: node1
        endpoint: 1.2.3.4
        port: 51820
        allowedIps: [10.0.1.0/24]
      - name: node2
        endpoint: 5.6.7.8
        port: 51820
        allowedIps: [10.0.2.0/24]
      - name: node3
        endpoint: 9.10.11.12
        port: 51820
        allowedIps: [10.0.3.0/24]
`

	tmpFile := "test_network_config.yaml"
	err := os.WriteFile(tmpFile, []byte(testConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to create test network config file: %v", err)
	}
	defer os.Remove(tmpFile)

	// Test loading network mode config
	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load network config: %v", err)
	}

	if len(cfg.Clusters) == 0 {
		t.Fatal("Expected clusters to be loaded")
	}

	cluster := cfg.Clusters[0]
	if cluster.Mode != "network" {
		t.Errorf("Expected cluster mode 'network', got '%s'", cluster.Mode)
	}

	if len(cluster.Nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(cluster.Nodes))
	}
}

func TestValidateConfig(t *testing.T) {
	// Create a temporary test config file
	testConfig := `clusters:
  - name: example
    mode: ptp
    cidr: 172.16.20.0/24
    removePeerFromAllowed: false
    nodes:
      - name: node1
        endpoint: 1.2.3.4
        port: 51820
        allowedIps: [10.0.1.0/24]
      - name: node2
        endpoint: 5.6.7.8
        port: 51820
        allowedIps: [10.0.2.0/24]
`

	tmpFile := "test_validate_config.yaml"
	err := os.WriteFile(tmpFile, []byte(testConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}
	defer os.Remove(tmpFile)

	// Test valid config
	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// This should not error
	// Note: actual validation is done in validator package
	_ = cfg
}
