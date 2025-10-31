# Sentinel Single Mode (V1.1+ Future Enhancement)

> **Status**: This is a **future enhancement** planned for v1.1+. If it adds complexity to v1.0, it will be deferred.

## Overview

Single mode is designed for cloud-native deployments where one Sentinel instance monitors exactly one MySQL cluster. It's optimized for sidecar patterns, Kubernetes deployments, and containerized environments.

## Use Case

One Sentinel instance per cluster, deployed as sidecar or alongside the cluster.

## Characteristics

- Monitors exactly one cluster
- **Always stateless** (in-memory state using SQLite with `:memory:` DSN)
- Logs to stdout (JSON format for log aggregation)
- Configuration from environment variables (v1.1), file/discovery later
- Lightweight, cloud-native (Kubernetes, Nomad, Docker Compose)
- Optional HA via shared leader election backend

## Configuration

### Environment Variables

```bash
# Required
export SENTINEL_MODE=single
export SENTINEL_CLUSTER_ID=mysql-cluster-a
export SENTINEL_SEED_NODES="10.0.1.5:3306,10.0.1.6:3306,10.0.1.7:3306"
export SENTINEL_MYSQL_USER=sentinel
export SENTINEL_MYSQL_PASSWORD=secret

# Optional: HA with leader election
export SENTINEL_LEADER_ELECTION_BACKEND=etcd
export SENTINEL_LEADER_ELECTION_ENDPOINTS=http://etcd:2379

# Start Sentinel
sentinel
```

## Deployment Examples

### Docker Compose

```yaml
services:
  mysql-primary:
    image: mysql:8.0
    # ... MySQL configuration

  mysql-replica:
    image: mysql:8.0
    # ... MySQL configuration

  sentinel:
    image: sentinel:latest
    environment:
      SENTINEL_MODE: single
      SENTINEL_CLUSTER_ID: mysql-cluster
      SENTINEL_SEED_NODES: "mysql-primary:3306,mysql-replica:3306"
      SENTINEL_MYSQL_USER: sentinel
      SENTINEL_MYSQL_PASSWORD: secret
    ports:
      - "8080:8080"  # API + UI
      - "9090:9090"  # Metrics
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sentinel-cluster-a
spec:
  replicas: 2  # HA with multiple instances
  selector:
    matchLabels:
      app: sentinel
      cluster: mysql-cluster-a
  template:
    metadata:
      labels:
        app: sentinel
        cluster: mysql-cluster-a
    spec:
      containers:
      - name: sentinel
        image: sentinel:latest
        env:
        - name: SENTINEL_MODE
          value: "single"
        - name: SENTINEL_CLUSTER_ID
          value: "mysql-cluster-a"
        - name: SENTINEL_SEED_NODES
          value: "mysql-0.mysql:3306,mysql-1.mysql:3306"
        - name: SENTINEL_MYSQL_USER
          valueFrom:
            secretKeyRef:
              name: sentinel-credentials
              key: username
        - name: SENTINEL_MYSQL_PASSWORD
          valueFrom:
            secretKeyRef:
              name: sentinel-credentials
              key: password
        - name: SENTINEL_LEADER_ELECTION_BACKEND
          value: "kubernetes"
        ports:
        - name: api
          containerPort: 8080
          protocol: TCP
        - name: metrics
          containerPort: 9090
          protocol: TCP
        livenessProbe:
          httpGet:
            path: /api/v1/health
            port: api
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /api/v1/health
            port: api
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: sentinel
spec:
  selector:
    app: sentinel
  ports:
  - name: api
    port: 8080
    targetPort: 8080
  - name: metrics
    port: 9090
    targetPort: 9090
```

### Kubernetes with Sidecar Pattern

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: mysql
spec:
  serviceName: mysql
  replicas: 3
  template:
    spec:
      containers:
      # MySQL container
      - name: mysql
        image: mysql:8.0
        # ... MySQL configuration

      # Sentinel sidecar
      - name: sentinel
        image: sentinel:latest
        env:
        - name: SENTINEL_MODE
          value: "single"
        - name: SENTINEL_CLUSTER_ID
          value: "mysql-cluster"
        - name: SENTINEL_SEED_NODES
          value: "mysql-0.mysql:3306,mysql-1.mysql:3306,mysql-2.mysql:3306"
        # ... other environment variables
```

## State Management

| Aspect | Single Mode |
|--------|-------------|
| State Storage | SQLite with `:memory:` DSN (in-memory, ephemeral) |
| Operation History | Lost on restart, not persisted |
| Audit Log | Structured logs to stdout (JSON), captured by log aggregator |
| Cluster Topology | Discovered on startup, maintained in memory |
| Configuration | Environment variables (v1.1), file/etcd later |

## High Availability

Single mode supports HA through:
- **Multiple instances**: Deploy 2+ Sentinel instances for the same cluster
- **Leader election**: Shared backend (etcd, Consul, Kubernetes leases)
- **Stateless operation**: Any instance can take over instantly
- **No data loss**: No persistent state to sync

## Observability

### Structured Logging

All events logged as JSON to stdout:

```json
{
  "timestamp": "2024-01-20T16:05:00Z",
  "level": "info",
  "message": "failover_started",
  "cluster": "mysql-cluster-a",
  "old_primary": "10.0.1.5:3306",
  "candidate": "10.0.1.6:3306"
}
```

Captured by log aggregators: Fluent Bit, Logstash, Vector, CloudWatch Logs, etc.

### Prometheus Metrics

Exposed on port 9090:

```
sentinel_failovers_total{cluster="mysql-cluster-a",result="success"} 5
sentinel_cluster_health{cluster="mysql-cluster-a"} 1
sentinel_replication_lag_seconds{cluster="mysql-cluster-a",node="replica-1"} 2.5
```

## Comparison with Multi Mode

| Feature | Multi Mode (V1.0) | Single Mode (V1.1+) |
|---------|-------------------|---------------------|
| Clusters Monitored | Multiple | Single |
| Configuration Source | YAML file | Environment variables |
| State Storage | Persistent (MySQL/PostgreSQL/SQLite file) | In-memory (SQLite `:memory:`) |
| Operation History | Persistent in database | Lost on restart |
| Audit Log | Database table | Structured logs (stdout) |
| Deployment Pattern | Central control plane | Sidecar, per-cluster |
| Best For | Traditional infrastructure | Kubernetes, cloud-native |

## Implementation Notes

### ConfigProvider for Single Mode

```go
// EnvConfigProvider reads from environment variables (single mode)
type EnvConfigProvider struct{}

func (p *EnvConfigProvider) GetMode() Mode {
    if os.Getenv("SENTINEL_MODE") == "single" {
        return ModeSingle
    }
    return ModeMulti
}

func (p *EnvConfigProvider) GetClusters() ([]*ClusterConfig, error) {
    mode := p.GetMode()
    if mode != ModeSingle {
        return nil, fmt.Errorf("EnvConfigProvider only supports single mode")
    }

    clusterID := os.Getenv("SENTINEL_CLUSTER_ID")
    if clusterID == "" {
        return nil, fmt.Errorf("SENTINEL_CLUSTER_ID is required in single mode")
    }

    seedNodesStr := os.Getenv("SENTINEL_SEED_NODES")
    if seedNodesStr == "" {
        return nil, fmt.Errorf("SENTINEL_SEED_NODES is required in single mode")
    }

    seedNodes := parseSeedNodes(seedNodesStr) // Parse comma-separated "host:port" list

    return []*ClusterConfig{
        {
            ID:        clusterID,
            SeedNodes: seedNodes,
            Username:  os.Getenv("SENTINEL_MYSQL_USER"),
            Password:  os.Getenv("SENTINEL_MYSQL_PASSWORD"),
        },
    }, nil
}
```

### State Store for Single Mode

```go
// Use SQLite with :memory: DSN for in-memory storage
stateStore := NewSQLiteStateStore(":memory:")
```

## Open Questions for V1.1

1. **Service discovery**: Should we support DNS-based seed node discovery? (e.g., `_mysql._tcp.cluster.local`)
2. **Configuration updates**: How to update configuration without restart in stateless mode?
3. **Kubernetes CRDs**: Should we provide a CRD for declarative cluster management?
4. **Operator pattern**: Should Sentinel have a Kubernetes operator mode?
5. **State sharing**: In HA mode, should instances share any state? (Currently fully independent)

## Migration Path from Multi to Single

For users starting with multi mode who want to adopt single mode:

1. **Deploy Sentinel in single mode** per cluster (sidecar or standalone)
2. **Keep multi mode** running for other clusters
3. **Gradually migrate** clusters one by one
4. **No data migration needed**: Sentinel is stateless, just configuration change

---

**Back to**: [ARCHITECTURE.md](./ARCHITECTURE.md)
