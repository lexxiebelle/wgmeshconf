# WireGuard Mesh Configuration Tool

A powerful tool for managing WireGuard VPN mesh networks with support for both Point-to-Point (PTP) and Network modes. Automatically generates WireGuard configuration files, manages IP allocations, and provides backup/restore functionality.

## Features

- **Dual Mode Support**: PTP (Point-to-Point) and Network modes
- **Dynamic Port Management**: Automatic port allocation with conflict avoidance
- **Port Allocation Strategies**: Linear and random allocation strategies
- **Automatic IP Allocation**: Preserves existing IP assignments
- **Key Management**: Automatic generation and management of WireGuard keys
- **Backup System**: Automatic and manual backup/restore functionality
- **Database Storage**: SQLite-based configuration persistence
- **Configuration Management**: Automatic detection of added/removed clusters and nodes
- **CLI Interface**: Easy-to-use command-line interface

## Installation

```bash
git clone <repository-url>
cd wgmeshconf
go build -o wgmeshconf cmd/wgmeshconf/main.go
```

## Quick Start

1. **Initialize the application:**
   ```bash
   ./wgmeshconf init
   ```

2. **Edit the configuration file:**
   ```bash
   nano config.yaml
   ```

3. **Apply configuration:**
   ```bash
   ./wgmeshconf apply
   # or with custom config file:
   ./wgmeshconf apply -f myconfig.yaml
   ```

4. **Write WireGuard configs:**
   ```bash
   ./wgmeshconf write
   ```

## Configuration

### Configuration File Structure

The tool uses YAML configuration files with the following structure:

```yaml
clusters:
  - name: example
    mode: ptp                    # or "network"
    cidr: 172.16.20.0/24
    removePeerFromAllowed: false
    # persistentKeepalive: 25
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
```

### Cluster Configuration Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | Yes | Unique cluster identifier |
| `mode` | string | Yes | Cluster mode: `ptp` or `network` |
| `cidr` | string | Yes | IP range for tunnel allocation (e.g., `172.16.20.0/24`) |
| `portsRange` | string | No | Port range for dynamic allocation (e.g., `20000-22000`, default: `20000-22000`) |
| `portsAllocate` | string | No | Port allocation strategy: `linear` or `random` (default: `random`) |
| `removePeerFromAllowed` | boolean | No | Remove peer IP from AllowedIPs (default: false) |
| `persistentKeepalive` | int | No | PersistentKeepalive peer option (default: 0) |
| `nodes` | array | Yes | Array of node configurations |

### Node Configuration Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | Yes | Unique node identifier within cluster |
| `endpoint` | string | Yes | Public IP address or hostname |
| `port` | integer | No | WireGuard port (if omitted, will be allocated dynamically) |
| `allowedIps` | array | Yes | IP ranges this node can route |
| `excludePeers` | array | No | Nodes to exclude from peer list |
| `bindAddress` | string | No | Interface bind address (e.g., "172.16.20.100") (only in network mode) |

## Modes

### PTP (Point-to-Point) Mode

In PTP mode, each node pair gets its own tunnel with dedicated IP addresses.

**Characteristics:**
- Each node pair has a dedicated tunnel
- Automatic IP allocation for each tunnel
- Individual configuration files for each tunnel
- Support for peer exclusions
- IP preservation across configuration changes

**Generated Files:**
```
configs/
├── example/
│   ├── node1/
│   │   ├── tun_node2.conf
│   │   └── tun_node3.conf
│   ├── node2/
│   │   ├── tun_node3.conf
│   │   └── tun_node1.conf
│   └── node3/
│       ├── tun_node1.conf
│       └── tun_node2.conf
```

**IP Allocation:**
- Uses the cluster CIDR for tunnel IP allocation
- Preserves existing IP assignments
- Automatically assigns new IPs for new tunnels

### Network Mode

In Network mode, each node gets a single configuration file with all peers.

**Characteristics:**
- Single configuration file per node
- All nodes in one mesh network
- Simplified peer management
- No individual tunnel IPs

**Generated Files:**
```
configs/
├── example/
│   ├── node1.conf
│   ├── node2.conf
│   └── node3.conf
```

## Commands

### Basic Commands

```bash
# Initialize the application
wgmeshconf init

# Apply configuration changes
wgmeshconf apply

# Generate and write WireGuard configuration files
wgmeshconf write

# Show current status
wgmeshconf status

# Clean up database and generated files
wgmeshconf cleanup
```

### Configuration Management

The tool automatically detects and handles configuration changes:

- **New Clusters**: Automatically added to the database
- **Removed Clusters**: Detected and removed with confirmation
- **New Nodes**: Added to existing clusters
- **Removed Nodes**: Removed from clusters automatically
- **Modified Configurations**: Updated in the database

**Example Output:**
```
 * Clusters changes: +2 add, -1 delete. Proceed? [N/y]:
 * Cluster ptp nodes changes: +5 add. Proceed? [N/y]:
```

### Backup Commands

```bash
# Create a manual backup
wgmeshconf backup

# List available backups
wgmeshconf backups

# Restore from a backup
wgmeshconf restore <backup-name>
```

### Command Options

```bash
# Use custom config file
wgmeshconf apply -f /path/to/config.yaml

# Use custom database file
wgmeshconf apply -d /path/to/database.db

# Combine options
wgmeshconf apply -f config.yaml -d wgmeshconf.db 
wgmeshconf apply --file config.yaml --database wgmeshconf.db

```

**Available flags:**
- `-f, --file string` - Path to config file (default "config.yaml")
- `-d, --database string` - Path to database file (default "wgmeshconf.db")  
- `-h, --help` - Show help message
- `-k, --showKeys` - Show full keys in status output

## Backup System

The tool includes a comprehensive backup system that automatically creates backups after each `apply` command and supports manual backups.

### Features

- **Automatic Backups**: Created after successful `apply` operations
- **Manual Backups**: Created via `backup` command
- **Automatic Cleanup**: Keeps only the last 10 backups
- **Metadata Storage**: Backup information including timestamps and sizes
- **Restore Functionality**: Complete state restoration

### Backup Structure

```
backups/
├── 2025-07-20_15-30-45_apply/
│   ├── database.db
│   ├── config.yaml
│   ├── configs/
│   └── metadata.json
└── latest -> 2025-07-20_15-30-45_apply/
```

### Backup Naming Convention

- **Automatic backups**: `YYYY-MM-DD_HH-MM-SS_apply`
- **Manual backups**: `YYYY-MM-DD_HH-MM-SS_manual`

## Database Schema

The tool uses SQLite to store configuration data:

### Tables

- **clusters**: Cluster information (name, mode, CIDR)
- **nodes**: Node information (name, endpoint, port, allowed IPs)
- **tunnels**: Tunnel information (PTP mode only) 

### Simple PTP Configuration

```yaml
clusters:
  - name: office
    mode: ptp
    cidr: 172.16.10.0/24
    portsRange: 20000-22000
    portsAllocate: random
    nodes:
      - name: server1
        endpoint: 203.0.113.1
        allowedIps: [10.0.1.0/24]
      - name: server2
        endpoint: 203.0.113.2
        allowedIps: [10.0.2.0/24]
      - name: server3
        endpoint: 203.0.113.3
        allowedIps: [10.0.3.0/24]
```

### Network Mode with Exclusions

```yaml
clusters:
  - name: exclusions
    mode: network
    cidr: 172.16.20.0/24
    portsAllocate: linear
    removePeerFromAllowed: true
    nodes:
      - name: gateway
        endpoint: 203.0.113.10
        # port omitted - will be allocated automatically
        allowedIps: [10.0.0.0/8]
        bindAddress: "172.16.20.100"
      - name: client1
        endpoint: 203.0.113.11
        port: 20234
        allowedIps: [10.0.1.0/24]
        excludePeers: [client2]
      - name: client2
        endpoint: 203.0.113.12
        port: 20237
        allowedIps: [10.0.2.0/24]
        excludePeers: [client1]
```

### Multiple Clusters

```yaml
clusters:
  - name: office
    mode: ptp
    cidr: 172.16.10.0/24
    nodes:
      - name: office1
        endpoint: 203.0.113.1
        allowedIps: [10.0.1.0/24]
      - name: office2
        endpoint: 203.0.113.2
        allowedIps: [10.0.2.0/24]
  
  - name: remote
    mode: network
    cidr: 172.16.20.0/24
    nodes:
      - name: remote1
        endpoint: 203.0.113.10
        port: 51820
        allowedIps: [10.1.0.0/24]
      - name: remote2
        endpoint: 203.0.113.11
        port: 51820
        allowedIps: [10.1.1.0/24]
```


## Backup Management

### Listing Backups

```bash
wgmeshconf backups
```

Output:
```
Available backups:
┌─────────────────────────────┬─────────────────────┬───────┐
│        BACKUP NAME          │       CREATED       │ SIZE  │
├─────────────────────────────┼─────────────────────┼───────┤
│ 2025-07-20_15-30-45_apply   │ 2025-07-20 15:30:45 │ 94920 │
│ 2025-07-20_15-35-12_manual  │ 2025-07-20 15:35:12 │ 94920 │
└─────────────────────────────┴─────────────────────┴───────┘
```

### Restoring from Backup

```bash
wgmeshconf restore 2025-07-20_15-30-45_apply
```

## File Structure

```
.
├── cmd/wgmeshconf/main.go    # Main CLI application
├── internal/
│   ├── allocator/            # IP allocation logic
│   ├── app/                  # App commands logic
│   ├── backup/               # Backup/restore functionality
│   ├── config/               # Configuration loading
│   ├── db/                   # Database operations
│   ├── writer/               # WG configs generation
│   ├── utils/                # Some functions
├── config.yaml               # Configuration file
├── wgmeshconf.db            # SQLite database
├── configs/                  # Generated WireGuard configs
└── backups/                  # Backup storage
```