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

🚧 **In Development** - Currently in planning phase.

See [`PHASES.md`](./PHASES.md) for the implementation roadmap and [`CLAUDE.md`](./CLAUDE.md) for development strategy.

## Quick Start

Coming soon after Phase 0 (Project Setup) is complete.

## Documentation

- [`ARCHITECTURE.md`](./ARCHITECTURE.md) - Comprehensive architectural design document
- [`PHASES.md`](./PHASES.md) - Phased implementation plan
- [`CLAUDE.md`](./CLAUDE.md) - Implementation strategy and current status

## License

TBD
