# Sentinel

A modern, cloud-ready MySQL failover and high availability tool designed for flexibility, scalability, and operational simplicity.

## Overview

Sentinel provides automatic failover capabilities for MySQL replication topologies with fine-grained control over configuration and behavior. It supports tree-structured replication (primary, intermediates, replicas) and offers both automatic monitoring and manual operation modes.

## Key Features

- **Automatic Failover**: Detects primary failures and promotes the best replica candidate
- **Tree Topology Support**: Handles complex replication hierarchies with intermediate nodes
- **Flexible Operations**: Step-based architecture allows custom integrations via shell scripts and webhooks
- **Multiple Modes**: Run as a central control plane (multi-cluster) or per-cluster sidecar
- **REST API + Web UI**: Programmatic access and user-friendly interface for manual operations
- **Version Compatibility**: Supports MySQL 5.7+ and 8.0+ with automatic syntax adaptation
- **Comprehensive Audit**: Full operation history and user tracking for compliance

## Architecture

Sentinel uses a pluggable architecture with interfaces for:
- **State Store**: MySQL, PostgreSQL, SQLite, or in-memory
- **Leader Election**: etcd, Consul, Zookeeper, Kubernetes, or in-memory
- **Discovery**: Seed-based topology discovery with automatic intermediate detection

See [`ARCHITECTURE.md`](./ARCHITECTURE.md) for detailed design documentation.

## Development Status

🚧 **In Development** - Phase 0 (Project Setup) complete.

See [`PHASES.md`](./PHASES.md) for the implementation roadmap and [`CLAUDE.md`](./CLAUDE.md) for development strategy.

## Quick Start

### Prerequisites

- Go 1.22.3 or later
- Make (optional, but recommended)

### Building

```bash
# Clone the repository
git clone https://github.com/pourtorabehsan/sentinel.git
cd sentinel

# Download dependencies
make deps

# Build the binary
make build
```

The binary will be created at `bin/sentinel`.

### Configuration

1. Copy the example configuration:
```bash
cp config.example.yaml config.yaml
```

2. Edit `config.yaml` to match your environment. For local testing, the default `config.yaml` works out of the box with an in-memory state store.

### Running

```bash
# Run with default config (config.yaml)
make run

# Or run the binary directly with a custom config
./bin/sentinel --config=path/to/config.yaml

# Show version
./bin/sentinel --version

# Adjust log level
./bin/sentinel --config=config.yaml --log-level=debug
```

### Development

```bash
# Run tests
make test

# Run tests with coverage
make coverage

# Format code
make fmt

# Run go vet
make vet

# Run all checks (fmt, vet, test)
make check

# Clean build artifacts
make clean
```

### Configuration File Structure

```yaml
mode: multi              # Operation mode: multi or single
log_level: info          # Log level: debug, info, warn, error

state_store:
  type: sqlite           # mysql, postgresql, or sqlite
  dsn: ":memory:"        # Database connection string

leader_election:
  backend: in-memory     # etcd, consul, zookeeper, database, or in-memory
  endpoints: []          # Backend endpoints (not needed for in-memory)

groups:
  - id: test-group
    clusters:
      - id: test-cluster-1
        seed_nodes:
          - host: localhost
            port: 3306
```

See `config.example.yaml` for a production-ready configuration example.

## Documentation

- [`ARCHITECTURE.md`](./ARCHITECTURE.md) - Comprehensive architectural design document
- [`PHASES.md`](./PHASES.md) - Phased implementation plan
- [`CLAUDE.md`](./CLAUDE.md) - Implementation strategy and current status

## License

TBD
