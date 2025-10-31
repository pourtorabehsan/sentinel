# Sentinel Architecture

> **Architectural Document**: This document describes the **concepts, patterns, and design decisions** for Sentinel. It intentionally avoids prescriptive code to allow developers flexibility in implementation choices. All descriptions provide sufficient detail for developers to make informed decisions about data structures, interfaces, algorithms, and patterns while maintaining the architectural intent.

## Overview

Sentinel is a modern, cloud-ready MySQL failover and high availability tool designed for flexibility, scalability, and operational simplicity. It provides automatic failover capabilities while offering fine-grained control over configuration and behavior.

## Design Goals

- **Flexibility**: Support multiple operational modes (single/multi-cluster, with/without API/UI/watch)
- **High Availability**: Distributed architecture with per-operation leader election
- **Configurability**: Hierarchical configuration with global → group → cluster → node precedence
- **Pluggability**: Interface-based design for storage, leader election, and discovery mechanisms
- **Observability**: Comprehensive auditing and user tracking
- **Simplicity**: Support both sophisticated and simple deployment scenarios

## Core Concepts

### Hierarchy

```
Global
  └── Group
       └── Cluster (MySQL Replication Group)
            └── Node (Individual MySQL Instance)
                 ├── Role: Primary/Replica
                 ├── Replication Status
                 └── Health Status
```

### Organizational Model

- **Node**: A single MySQL instance with connection details, role, and health status
  - Role can be: Primary, Replica, or Intermediate (a replica that acts as source for other replicas)
- **Cluster**: A MySQL replication topology (tree structure: 1 primary, intermediates, replicas)
- **Group**: A logical grouping of clusters (e.g., "production", "staging", "region-us-east")
- **Global**: System-wide settings and defaults

### Replication Topology Structure

Sentinel supports **tree-structured replication topologies**, not just flat primary-replica:

```
Primary (depth 0)
  ├── Replica A (depth 1)
  ├── Intermediate B (depth 1) ─── has downstream replicas
  │   ├── Replica B1 (depth 2)
  │   └── Replica B2 (depth 2)
  ├── Intermediate C (depth 1)
  │   └── Intermediate C1 (depth 2) ─── nested intermediate
  │       └── Replica C1.1 (depth 3)
  └── Replica D (depth 1)
```

**Key Concepts:**
- **Intermediate**: A replica that serves as the replication source for other nodes
- **Replication Depth**: Distance from primary (0 = primary, 1 = direct replica, 2+ = cascading)
- **Subtree**: An intermediate and all its downstream nodes
- **Parent/Children**: Direct replication relationships

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Sentinel Instance                         │
├─────────────────────────────────────────────────────────────┤
│  ┌────────────┐  ┌──────────┐  ┌─────────────┐            │
│  │   API      │  │    UI    │  │   Watcher   │            │
│  │  (REST)    │  │ (Web UI) │  │  (Monitor)  │            │
│  └─────┬──────┘  └────┬─────┘  └──────┬──────┘            │
│        │              │                │                    │
│        └──────────────┴────────────────┘                    │
│                       │                                     │
│        ┌──────────────▼───────────────────┐                │
│        │       Core Engine                │                │
│        │  - Topology Management           │                │
│        │  - Failover Decision             │                │
│        │  - Config Management             │                │
│        │  - Audit Logging                 │                │
│        └──────────┬───────────────────────┘                │
│                   │                                         │
│    ┌──────────────┼──────────────────┐                     │
│    │              │                  │                      │
│    ▼              ▼                  ▼                      │
│ ┌─────┐      ┌─────────┐      ┌──────────┐                │
│ │State│      │ Leader  │      │Discovery │                │
│ │Store│      │ Election│      │ Manager  │                │
│ │     │      │         │      │          │                │
│ └──┬──┘      └────┬────┘      └────┬─────┘                │
└────┼─────────────┼──────────────────┼──────────────────────┘
     │             │                  │
     ▼             ▼                  ▼
┌─────────┐  ┌─────────┐      ┌──────────────┐
│ MySQL   │  │  etcd   │      │MySQL Clusters│
│  or     │  │  Zk     │      │(Monitoring)  │
│ KV Store│  │  or DB  │      │              │
└─────────┘  └─────────┘      └──────────────┘
```

## Core Interfaces

### 1. State Store Interface

The state store abstracts persistence of configuration, cluster topology, and operational state. It provides a clean separation between business logic and storage implementation.

**Responsibilities:**

**Configuration Management:**
- Retrieve and persist configuration at all hierarchy levels (global, group, cluster, node)
- Support both active and pending configuration states
- Handle configuration versioning for optimistic locking

**Topology Management:**
- CRUD operations for groups, clusters, and nodes
- List operations with filtering by parent resource (e.g., list clusters in a group)
- Support for hierarchical queries (get all nodes in a cluster)

**Operation Tracking:**
- Create and update operation records for async operations (failover, recovery, etc.)
- Query operations by ID or filters (cluster, status, time range, user)
- Track operation lifecycle: created, running, completed, failed
- Store operation context, steps completed, and error information

**Audit Trail:**
- Persist audit events with full context (user, timestamp, action, result)
- Query audit logs with flexible filters (cluster, user, operation type, date range)
- Support pagination for large result sets

**Health & Status:**
- Update and retrieve node health metrics (reachability, replication lag, errors)
- Track temporal health data for trending

**Resource Deletion:**
- Safe deletion of groups, clusters, and nodes
- Handle cascading deletes or validation (e.g., cannot delete cluster with active nodes)

**Implementation Options:**
- MySQL/PostgreSQL backend for multi-mode (persistent, distributed)
- SQLite with file DSN for multi-mode (persistent, single-instance)
- SQLite with `:memory:` DSN for single-mode (ephemeral, stateless)

**Design Considerations:**
- Interface should support context for cancellation and timeouts
- Return structured errors for not-found, conflict, validation failures
- Consider transaction support for complex operations

### 2. Leader Election Interface

Leader election ensures only one Sentinel instance performs critical write operations (failover, topology changes, configuration updates) at a time, preventing conflicts and race conditions.

**Core Concept:**
Leader election is **per-operation** and **per-resource**, not global. Multiple operations on different clusters can run concurrently, but only one instance can perform a specific operation on a specific resource.

**Lock Acquisition:**
- Instances attempt to acquire a lock identified by: resource ID
- Lock acquisition requires: unique session ID (instance identifier) + TTL (time-to-live)
- If lock is already held, acquisition fails (another instance is handling it)
- If successful, returns a lock handle for management

**Lock Management:**
- **Release**: Explicitly give up lock ownership (called when operation completes)
- **Renew**: Extend lock TTL for long-running operations (prevents timeout)
- **Auto-expiry**: If holder crashes, lock expires after TTL

**Operation Types Requiring Coordination:**
1. **Failover**: Primary or intermediate node failover
2. **Recovery**: Intermediate recovery, graceful takeover
3. **Topology Change**: Manual node repointing, promotion, detachment
4. **Config Apply**: Applying pending configuration changes

**Health Checking:**
- Interface should provide health check for leader election backend
- Used to determine if distributed coordination is available
- Fallback: if backend unhealthy and single instance, can proceed without election

**Implementation Options:**
- **etcd**: Distributed key-value store with TTL and compare-and-swap
- **Consul**: Service mesh with session-based locking
- **Zookeeper**: Distributed coordination service with ephemeral nodes
- **Database-based**: Use SELECT FOR UPDATE or advisory locks in StateStore database
- **Kubernetes**: Use Lease API for cloud-native deployments
- **In-memory**: No coordination (single-instance fallback)

**Design Considerations:**
- Session ID must be unique and stable per Sentinel instance
- TTL should be long enough for operations but short enough for fast failover on crashes
- Lock interface should be released even if operation fails (use defer patterns)
- Consider lock renewal for operations exceeding initial TTL

## Configuration System

### Configuration Hierarchy

Configuration follows a hierarchical merge strategy:
- **Global** config provides system-wide defaults
- **Group** config overrides global for all clusters in group
- **Cluster** config overrides group for all nodes in cluster
- **Node** config overrides cluster for specific node

### Configuration Levels

Some configuration keys can be restricted to specific hierarchy levels to enforce architectural constraints.

**Level Restriction Mechanism:**
Use bitwise flags to represent allowed levels, enabling combinations:
- **Global-only**: Settings that affect entire system (e.g., leader election backend, state store configuration)
- **Any level**: Settings that can be overridden anywhere (e.g., auto-failover enabled, retry counts)
- **Cluster and below**: Settings specific to clusters/nodes (e.g., replication lag thresholds)
- **Node-only**: Connection-specific settings (e.g., MySQL connection timeout, credentials)

**Example Level Restrictions:**
- `auto_failover_enabled`: Any level (can enable/disable per cluster or globally)
- `failover_timeout_seconds`: Global, Group, or Cluster (not per-node)
- `replication_lag_threshold`: Cluster or Node (workload-specific)
- `mysql_connection_timeout`: Node only (connection-specific)
- `leader_election_backend`: Global only (system-wide infrastructure)

**Validation:**
Configuration schema should validate that keys are only set at allowed levels, rejecting invalid configurations at load/apply time.

### Pending Configuration

Configuration changes enter a "pending" state before being applied, allowing review and approval workflows.

**Configuration States:**
- **Active**: Currently applied and in effect
- **Pending**: Changed but not yet applied, awaiting approval/review

**Configuration Versioning:**
Each configuration resource (global, group, cluster, node) maintains:
- **Active version**: Current effective configuration
- **Pending version**: Proposed changes (may be null if no pending changes)
- **Metadata**: Who changed it, when changed, version number for optimistic locking

**Apply Process:**
1. User makes configuration changes via API/UI
2. Changes saved as "pending" version
3. User explicitly applies pending configuration (or auto-apply if configured)
4. Pending becomes active, old active is archived/discarded
5. Audit event logged

**Use Cases:**
- Prevent accidental configuration changes
- Review changes before applying to production
- Coordinate configuration changes with maintenance windows
- Track who changed what and when

---

## Topology Discovery and Monitoring

### Discovery Strategy

Sentinel uses **seed-based discovery** to learn MySQL replication topology without requiring full cluster inventory upfront.

**Discovery Process:**
1. **Seed Nodes**: User provides one or more initial nodes per cluster (typically the primary or any known node)
2. **Crawl Topology**: Connect to seed node, query its replication relationships
3. **Recursive Discovery**: Follow replication links to discover all nodes in the cluster
4. **Build Tree Structure**: Construct parent-child relationships to represent the replication tree
5. **Auto-Detect Roles**: Identify primary, replicas, and intermediates based on replication relationships
6. **Periodic Refresh**: Re-discover topology at configured intervals to detect changes

**Seed Nodes Are Temporary:**
Seed nodes are only used for initial discovery. Once topology is known, Sentinel tracks all nodes directly. Seed nodes don't need to be stored in state—they're configuration-time hints.

### Intermediate Detection

**Auto-Detection Algorithm:**
Intermediates are automatically identified during topology discovery—no manual configuration needed.

**Detection Logic:**
- Query each node for:
  - **Upstream**: Which node is this replicating from? (via replication status commands)
  - **Downstream**: Which nodes are replicating from this? (via replica hosts commands)
- Build parent-child map from this information
- **Role Assignment**:
  - **Primary**: Node with no upstream source (SourceNodeID = null)
  - **Intermediate**: Node with both upstream source AND downstream replicas (has children)
  - **Replica**: Node with upstream source but no downstream replicas (leaf node)

**MySQL Commands Used:**
- **Upstream detection**: `SHOW REPLICA STATUS` / `SHOW SLAVE STATUS`
- **Downstream detection**: `SHOW REPLICAS` / `SHOW SLAVE HOSTS`

**Benefits:**
- Zero configuration—topology emerges from actual replication relationships
- Adapts to external changes (if DBA manually changes replication topology)
- Simplifies initial setup and ongoing management
- Automatically recognizes cascading replication hierarchies

### MySQL Version Compatibility

MySQL's replication command syntax changed across versions, requiring version-aware SQL generation.

**Version Differences:**
- **MySQL 5.7 and earlier**: `MASTER`/`SLAVE` terminology
  - Commands: `SHOW MASTER STATUS`, `SHOW SLAVE STATUS`, `CHANGE MASTER TO`, `START SLAVE`, etc.
- **MySQL 8.0.22+**: `SOURCE`/`REPLICA` terminology (new inclusive naming)
  - Commands: `SHOW BINARY LOG STATUS`, `SHOW REPLICA STATUS`, `CHANGE REPLICATION SOURCE TO`, `START REPLICA`, etc.
- **MySQL 8.0.0 - 8.0.21**: Transition period, both syntaxes work
- **MariaDB/Percona**: May have own variants, need separate handling

**Compatibility Strategy:**
Use a **version-aware client abstraction** that hides version differences from business logic.

### MySQL Client Abstraction

**Purpose:**
Provide a high-level interface for MySQL operations that automatically adapts to server version, hiding SQL syntax differences.

**Client Responsibilities:**

**Connection Management:**
- Establish and maintain MySQL connections
- Handle connection pooling, timeouts, retries
- Detect and cache MySQL version on connect

**Topology Discovery:**
- Query replication status (upstream source, lag, thread status, errors)
- Query binary log position (for GTID-less replication)
- List replicas (downstream nodes)
- All queries return normalized data structures regardless of MySQL version

**Replication Control:**
- Start/stop replication threads
- Reset replication configuration
- Change replication source (repoint to new primary/intermediate)
- Support both GTID and file:position based replication

**Health and Status:**
- Connection health checks (ping)
- Query/set read-only mode
- Verify replication thread status

**Version Detection:**
- Query `SELECT VERSION()` on connection
- Parse version string (e.g., "8.0.32-MySQL", "5.7.40", "10.6.10-MariaDB")
- Extract: major, minor, patch, flavor (MySQL/MariaDB/Percona)
- Use version to determine command syntax

**Implementation Pattern:**
Use internal **SQL Builder** and **Result Parser** components:
- **SQL Builder**: Generate version-appropriate SQL commands based on detected version
- **Result Parser**: Handle version-specific column names in result sets (e.g., `Master_Host` vs `Source_Host`)
- Client interface remains constant; implementation adapts internally

### Discovery Manager Responsibilities

The Discovery Manager orchestrates topology discovery and health monitoring operations.

**Core Operations:**

**Initial Discovery:**
- Accept cluster ID and seed node list (host:port pairs)
- Connect to seed nodes, crawl replication topology recursively
- Build complete cluster topology structure (tree + flat node list)
- Persist discovered topology to StateStore

**Topology Refresh:**
- Re-query all known nodes for current replication status
- Detect topology changes (new nodes, removed nodes, repointed replicas)
- Update StateStore with current topology
- Return refreshed topology snapshot

**Node Health Monitoring:**
- Execute health checks on individual nodes (connection test, replication status)
- Return health metrics for storage and alerting

### Data Structures

**ClusterTopology:**
Represents a complete snapshot of cluster replication topology at a point in time.
- **ClusterID**: Unique identifier for the cluster
- **Primary**: Reference to primary node (may be null during failover)
- **Nodes**: Flat list of all nodes for easy iteration/filtering
- **Tree**: Tree structure with parent/child pointers for traversal
- **Timestamp**: When topology was discovered/refreshed

**TopologyNode (Tree Structure):**
Wraps a Node with tree traversal pointers for recursive operations.
- **Node**: The actual MySQL node data
- **Parent**: Pointer to upstream node (null for primary)
- **Children**: List of downstream nodes (empty for leaf replicas)
- **Depth**: Distance from primary (0 = primary, 1 = direct replica, 2+ = cascaded)

**Node (MySQL Instance):**
Represents a single MySQL server in the replication topology.

*Identity:*
- ID, Host, Port
- Role: Primary (int enum 0), Replica (enum 1), or Intermediate (enum 2)

*Replication Relationships:*
- SourceNodeID: Which node is this replicating from? (null for primary)
- DownstreamNodes: List of node IDs replicating from this node

*Capabilities:*
- CanBePromoted: Boolean flag—can this node become primary?
- IsIntermediate: Boolean flag—does this node have downstream replicas? (auto-detected)

*Connection Info:*
- Username, Password (password MUST be encrypted at rest with AES-256-GCM or similar)
- Passwords decrypted only when establishing MySQL connections
- Never expose passwords in JSON responses or logs

*Metadata:*
- MySQL Version (major, minor, patch, flavor)
- ServerID (MySQL server_id)
- GTID executed set (for GTID-based replication)

**NodeHealth (Health Metrics):**
Health and replication status for a node at a point in time.
- **Reachability**: Can Sentinel connect to this node?
- **Last Check Time**: When was this health check performed?
- **Replication Status**: IO/SQL thread running, lag duration, errors
- **Source Info**: Which host/port is this replicating from?
- **Read-Only Mode**: Is MySQL in read-only mode?

### Monitoring Loop

Each Sentinel instance runs a continuous monitoring loop for every managed cluster.

**Loop Cycle (repeats at configured interval, e.g., every 10 seconds):**

1. **Refresh Topology:**
   - Re-discover current replication structure
   - Auto-detect intermediates (any node with downstream replicas)
   - Update StateStore with latest topology

2. **Health Check All Nodes (in parallel):**
   - Test connectivity to each node
   - Query replication status
   - Measure replication lag
   - Store health metrics in StateStore

3. **Analyze for Issues:**
   - **Primary unreachable?** → Trigger failover if auto-failover enabled
   - **Intermediate unreachable?** → Trigger intermediate recovery based on strategy
   - **Replica lag too high?** → Alert but don't fail over (lag threshold configurable)
   - **Replication broken?** (IO/SQL thread stopped with errors) → Alert
   - **Replication depth > warning level?** → Alert but don't block (default threshold: 5)

4. **If Issue Detected → Trigger Action:**
   - Attempt to acquire leader lock for the operation
   - If lock acquired, execute operation (failover, recovery, etc.)
   - If lock held by another instance, stand down

5. **Sleep Until Next Interval:**
   - Wait for configured monitoring interval
   - Repeat cycle

**Depth Monitoring:**
Sentinel tracks replication depth (distance from primary) and warns if depth exceeds `replication_depth_warning` (default: 5). However, it does **not enforce** any maximum depth—deep chains are allowed but discouraged because lag multiplies at each level.

### Multi-Instance Coordination

Multiple Sentinel instances can monitor the same clusters for high availability, but **only one instance acts** when failures are detected.

**Coordination Model:**

**Independent Monitoring:**
- Each Sentinel instance monitors independently
- Each maintains its own view of topology and health
- No shared state during monitoring (read-only from StateStore)

**Leader Election on Action:**
- When an instance detects a failure requiring intervention (failover, recovery):
  1. Instance attempts to acquire a leader lock for that specific operation + resource
  2. Lock identified by: operation type + cluster ID (e.g., "failover:prod-mysql-1")
  3. If lock acquired successfully → instance becomes leader and performs operation
  4. If lock already held → another instance is handling it, stand down

**Failure Confirmation:**
Before triggering failover, instance confirms failure from multiple sources:
- Sentinel cannot connect to primary (TCP failure)
- **AND** at least N replicas report their replication IO thread stopped with connection errors
- This prevents false positives from network splits affecting only Sentinel

**Lock Management:**
- Lock has TTL (e.g., 30 seconds for quick operations, 5 minutes for longer ones)
- If lock holder crashes, lock expires automatically after TTL
- Lock should be renewed for long-running operations
- Lock released explicitly when operation completes

---

## Failover Decision and Execution

### Failure Detection Criteria

A primary is considered failed when:
1. **Sentinel cannot connect** to primary (TCP connection fails or times out)
2. **AND** replicas confirm primary is down:
   - At least N replicas report IO thread stopped (`Replica_IO_Running: No` or `Slave_IO_Running: No`) with connection error
   - This prevents false positives from network splits affecting only Sentinel

### Replica Selection Algorithm

When selecting a new primary (or promoting an intermediate's child), Sentinel evaluates all eligible replicas and ranks them.

**Promotion Candidate Data:**
For each eligible replica, calculate:
- **Score**: Numeric ranking (higher is better)
- **Reason**: Human-readable explanation for why this candidate ranks high/low
- **Replication Lag**: Current lag duration
- **Data Loss Estimate**: Estimated MB of data not yet replicated
- **Priority**: User-configured priority value (if set in node configuration)

**Selection Criteria (in priority order):**

1. **Reachability**: Replica must be reachable and responding to connections
2. **Promotion Eligibility**: Must have `CanBePromoted: true` flag set
3. **User Priority**: If configured, prefer replica with highest priority value
4. **Data Loss**: Prefer replica closest to primary (least estimated data loss)
5. **Replication Lag**: Prefer replica with lowest lag duration
6. **Tie-Breaker**: Lexicographical ordering by node ID for deterministic selection

**Scoring Algorithm:**
Assign points for each criterion, multiply by weights, sum to get final score. Highest score wins.

**Usage:**
This algorithm is used for both primary promotion (selecting from all replicas) and intermediate promotion (selecting from an intermediate's children).

### Failover Execution Steps

Failover executes as a sequence of steps, each building on the previous step's results.

**1. Acquire Leader Lock**
- Attempt to acquire distributed lock for failover operation on this cluster
- Lock identifier: operation type + cluster ID (e.g., "failover:prod-mysql-1")
- If lock acquisition fails, another instance is handling failover—abort

**2. Validate Current State**
- Re-confirm primary is still unreachable (may have recovered since detection)
- Check no other failover is already in progress for this cluster
- Verify cluster configuration allows auto-failover (may be disabled)
- If validation fails, release lock and abort

**3. Select Best Replica Candidate**
- Gather list of all healthy replicas
- Apply replica selection algorithm (see previous section)
- If no suitable candidate found, alert and abort

**4. Pre-Promotion Validation**
- Verify candidate is still reachable (health may have changed)
- Wait for candidate's relay log to be fully applied (minimize data loss)
- Confirm candidate's replication lag is within acceptable threshold
- If candidate fails validation, try next best candidate

**5. Promote New Primary**
- Stop replication on candidate node
- Reset replication configuration (clear source info)
- Disable read-only mode (allow writes)
- Uses version-aware MySQL client to execute appropriate SQL commands

**6. Repoint Remaining Replicas**
- For each remaining replica (excluding promoted node):
  - Stop replication threads
  - Change replication source to new primary
  - Start replication threads
- **Replication Method** (configurable):
  - **Auto** (default): Use GTID if MySQL >= 5.6 AND GTID_MODE=ON, otherwise file:position
  - **Force GTID**: Always use GTID auto-positioning
  - **Force Binlog**: Always use binary log file:position
- Execute in parallel where possible to minimize downtime

**7. Update StateStore**
- Mark new node's role as Primary in topology
- Mark old primary's state as Failed
- Update all replication relationships in cluster topology
- Store timestamp of failover completion

**8. Log Audit Event**
- Record comprehensive audit event with:
  - Who triggered it (user or "system" for auto-failover)
  - What happened (old primary → new primary)
  - When it occurred (timestamps)
  - Why it was triggered (reason/cause)
  - Result (success/failure with details)

**9. Send Notifications**
- If notification hooks configured, send alerts to:
  - Slack channels
  - PagerDuty incidents
  - Email distribution lists
  - Webhook endpoints
  - Custom notification scripts

**10. Release Leader Lock**
- Explicitly release distributed lock
- Allow other instances to perform operations if needed

### Handling Failure During Failover

If failover fails partway through:
- **Idempotency**: Each step should be idempotent where possible
- **State tracking**: Store current failover phase in StateStore
- **Recovery**: Next instance can resume or rollback based on phase
- **Manual intervention**: If automatic recovery fails, provide clear status for ops team

### Preventing Split-Brain

Critical protections:
1. **Fencing old primary**: Before promoting new primary, attempt to set old primary to `read_only=ON`
2. **GTID validation**: Ensure new primary's GTID set is ahead or equal to all replicas
3. **Explicit promotion**: Never automatically promote without operator intent (human or auto-failover config)

---

## Intermediate Failure Handling

### Failure Scenarios

When an intermediate node fails, its downstream nodes (children) lose their replication source. Sentinel must decide how to recover.

### Intermediate Failure Strategies

Configurable per cluster/group via `intermediate_failover_strategy`:

#### 1. **Promote Child** (Default)
Select one of the failed intermediate's children and promote it to become the new intermediate.

**Before/After Topology:**
- Before: Primary → Intermediate (FAILED) → Child Replicas (Replica 1, Replica 2)
- After: Primary → Replica 1 (promoted to intermediate) → Replica 2 (repointed to Replica 1)

**How It Works:**
- Apply promotion candidate selection algorithm to the intermediate's children
- Promote best child to intermediate role (can now accept downstream replicas)
- Repoint remaining children to the promoted node
- Preserves same replication depth structure

**Selection Criteria** (similar to primary promotion):
- Child must have `CanBePromoted: true` flag
- Prefer child with least replication lag
- Prefer child with higher user-configured priority

**Pros**: Preserves topology structure; maintains geographic/logical separation; good for distributed topologies
**Cons**: More complex than simple repoint strategies; requires eligible child

**Configuration Options:**
- `prefer_same_datacenter`: Prefer children in same region/datacenter
- `max_lag_seconds`: Maximum acceptable lag for promotion candidate

#### 2. **Repoint to Parent**
Flatten the topology by reconnecting all children directly to the intermediate's parent (source).

**Before/After Topology:**
- Before: Primary → Intermediate (FAILED) → Children
- After: Primary → Children (all repointed directly to Primary)

**How It Works:**
- Identify the failed intermediate's parent node (its replication source)
- Repoint all children directly to that parent
- Reduces replication depth by one level

**Pros**: Simple strategy; maintains replication from highest authority; reduces replication depth
**Cons**: May overload parent with more direct children; loses geographic/logical separation; changes topology structure

**Configuration Options:**
- Basic strategy with no additional parameters

#### 3. **Repoint to Sibling**
Repoint children to another intermediate at the same depth (a sibling node).

**Before/After Topology:**
- Before: Primary → [Intermediate A, Intermediate B (FAILED) with children]
- After: Primary → [Intermediate A with additional children from B, Intermediate C]

**How It Works:**
- Find sibling intermediates (nodes at same depth, sharing same parent)
- Select best sibling based on criteria (load, proximity, health)
- Repoint failed intermediate's children to selected sibling
- Maintains replication depth

**Pros**: Good for load balancing; maintains replication depth; preserves multi-intermediate topology
**Cons**: Requires available sibling; may cross logical boundaries; can overload sibling

**Configuration Options:**
- `prefer_same_group`: Prefer siblings in same datacenter/region/group
- `max_child_count`: Don't select siblings that already have too many children

#### 4. **Manual** (No Auto-Recovery)
Do not automatically recover. Alert operators and wait for manual intervention.

**How It Works:**
- Detect intermediate failure
- Send alerts/notifications to operators
- Do not modify topology automatically
- Wait for human operator to decide recovery strategy

**Pros**: Safe for critical topologies; prevents unexpected changes; allows human judgment
**Cons**: Requires human intervention; slower recovery; children remain without replication source

**Configuration Options:**
- `alert_on_intermediate_failure`: Enable/disable alerting
- Alert targets: Slack, PagerDuty, email, etc.

### Configuration Hierarchy for Intermediate Failover

Intermediate-specific configurations follow the same hierarchical model as other settings.

**Configuration Keys and Allowed Levels:**

- **`intermediate_failover_strategy`**: Global, Group, or Cluster level
  - Determines which strategy to use when intermediate fails
  - Values: promote_child (int 0, default), repoint_to_parent (int 1), repoint_to_sibling (int 2), manual (int 3)

- **`intermediate_failover_enabled`**: Any level (including per-node)
  - Boolean flag to enable/disable automatic intermediate failover
  - Can disable for specific nodes if needed

- **`replication_depth_warning`**: Global, Group, or Cluster level
  - Threshold for alerting on deep replication chains
  - Default: 5 (warn if depth > 5, but don't enforce maximum)

- **`auto_repoint_on_recovery`**: Any level
  - Boolean flag: if intermediate recovers, automatically repoint children back to it
  - Default: false (leave topology as-is after recovery)

### Intermediate Recovery Process

```
┌────────────────────────────────────────────────────────────┐
│         Intermediate Failure Recovery Process              │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  1. Detect Intermediate Failure                            │
│     ├─> Intermediate unreachable                          │
│     └─> Children report IO thread stopped                 │
│                                                            │
│  2. Acquire Leader Lock                                    │
│     └─> Resource: "intermediate_failover:cluster-{id}"       │
│                                                            │
│  3. Validate Configuration                                 │
│     └─> Check intermediate_failover_strategy              │
│                                                            │
│  4. Identify Affected Subtree                              │
│     └─> Find all downstream nodes (children)              │
│                                                            │
│  5. Execute Strategy                                       │
│     ├─> repoint_to_parent: Point children to intermediate's source
│     ├─> promote_child: Select & promote best child        │
│     ├─> repoint_to_sibling: Find sibling & repoint        │
│     └─> manual: Alert and wait                            │
│                                                            │
│  6. For Each Child (parallel if possible)                  │
│     ├─> STOP REPLICA                                      │
│     ├─> CHANGE REPLICATION SOURCE TO new_source           │
│     └─> START REPLICA                                     │
│                                                            │
│  7. Update Topology in StateStore                          │
│     ├─> Mark intermediate as Failed                       │
│     ├─> Update children's SourceNodeID                    │
│     └─> Update roles (if promotion occurred)              │
│                                                            │
│  8. Log Audit Event                                        │
│     └─> Record strategy used and affected nodes           │
│                                                            │
│  9. Send Notifications                                     │
│                                                            │
│ 10. Release Leader Lock                                    │
│                                                            │
└────────────────────────────────────────────────────────────┘
```

### Primary Failover with Intermediates

When the **primary** fails and has intermediate nodes, Sentinel **always preserves the intermediate structure**:

```
Before:                After:
Primary (FAILED)       New Primary (promoted)
  ├── Replica A          ├── Replica A
  ├── Intermediate B     ├── Intermediate B (repointed to new primary)
  │   └── Replica B1     │   └── Replica B1 (unchanged)
  └── Replica C          └── Replica C
```

**Strategy:**
1. Promote best candidate to new primary (could be direct replica or even an intermediate)
2. Repoint all direct children of old primary to new primary
3. Intermediates retain their downstream replicas
4. Downstream replicas keep replicating from their intermediates

This preserves the topology structure, which is important for:
- Geographic distribution (e.g., regional intermediates)
- Network topology (e.g., datacenter-specific intermediates)
- Load distribution across the replication tree

---

## Topology Manipulation API

Sentinel provides programmatic APIs for manual topology modifications, useful for maintenance, testing, and recovery scenarios.

**Manual Operations Supported:**

**Promote to Intermediate:**
- Convert a regular replica into an intermediate node
- Allows downstream replicas to be pointed at it
- Use case: Adding a new replication layer for geographic distribution

**Demote to Replica:**
- Convert an intermediate back to regular replica
- Requires first repointing or removing its children
- Use case: Simplifying topology, removing unnecessary layers

**Move Node:**
- Change a node's replication source (repoint to different parent)
- Option to move entire subtree or just the node
- Use case: Rebalancing topology, testing failover scenarios

**Detach Node:**
- Disconnect node from replication (make standalone)
- Node becomes independent MySQL instance
- Use case: Decommissioning, testing, creating new cluster seed

**Get Promotion Candidates:**
- Query system for ranked list of candidates for a given node
- Returns scored candidates with reasons (uses replica selection algorithm)
- Use case: UI showing options before manual failover, decision support

**Usage Contexts:**
- **Web UI**: Drag-and-drop topology editor with visual feedback
- **CLI Tools**: Manual recovery and maintenance commands
- **Internal**: Called by automatic failover logic during recovery

---

## Step-Based Operation Architecture

Sentinel uses a **step-based architecture** for all operations (failover, intermediate failover, graceful takeover, etc.). This provides:
- **Testability**: Each step is independently testable
- **Flexibility**: Users can inject custom steps anywhere in the operation
- **Type Safety**: All configuration is validated at operation creation time (when constructing the operation from YAML)
- **Simplicity**: One mental model for all operations

### Core Step Concept

Each operation (failover, recovery, topology change) is decomposed into a sequence of discrete steps. Each step is a self-contained unit of work with clearly defined inputs, outputs, and failure handling.

**Step Interface Requirements:**

**Identity:**
- Human-readable name for logging and UI display

**Execution:**
- Execute method that performs the step's action
- Receives operation context containing shared state
- Returns error if step fails

**Failure Handling:**
- Declare failure mode: Abort (stop operation) or Continue (log error, proceed)
- Async steps MUST use Continue mode (fire-and-forget, cannot abort)

**Dependencies:**
- Declare required data (what must exist in context before this step runs)
- Declare provided data (what this step adds to context)
- Used for validation at operation creation time to ensure steps are in correct order

**Async Capability:**
- Flag indicating if step can run asynchronously (fire-and-forget)
- Async steps: notifications, webhooks, non-critical logging
- Sync steps: database writes, MySQL commands, critical state changes

**Failure Mode Types:**
- **Abort** (int 0): Stop entire operation immediately on error, no subsequent steps execute
- **Continue** (int 1): Log error but continue to next step, used for non-critical steps

**Design Principles:**
- **No Rollback**: Intentionally omitted for v1.0 simplicity, can be added later if needed
- **Idempotency**: Steps should be idempotent where possible (safe to retry)
- **Testability**: Each step independently testable in isolation
- **Dependency Validation**: Steps declare dependencies, validated at operation construction time

### Operation Context

The **Operation Context** is a shared data structure that flows through all steps during an operation's execution. It serves as the communication mechanism between steps.

**Context Components:**

1. **Immutable Metadata** (set at operation start):
   - Operation type (failover, intermediate_failover, etc.) represented as int enum
   - Cluster and group identifiers
   - Timestamp of when operation was initiated
   - Triggering user (for manual operations, used in audit trail)

2. **Mutable State** (built up during execution):
   - Node references: Old primary, new primary, candidate, failed node
   - Collections of affected nodes (repointed replicas, promoted intermediates)
   - Dynamically populated based on operation progress

3. **Execution Tracking**:
   - List of completed step names
   - Failed step identifier (if any)
   - Start and end timestamps for operation duration tracking

4. **Inter-Step Communication**:
   - Generic metadata map for custom data exchange between steps
   - Steps write data that subsequent steps may consume
   - Flexible schema allows custom steps to communicate arbitrary information

**Context Usage Pattern:**
- Steps **read** from context to access information from previous steps
- Steps **write** to context to provide information for subsequent steps
- Metadata map enables custom steps to pass domain-specific data

### Built-In Steps

Sentinel provides built-in steps for common operations. Each built-in step has specific responsibilities:

**1. Acquire Lock Step** (`acquire-lock`)
- Acquires distributed lock via leader election backend
- Configurable TTL (time-to-live) for lock duration
- Stores lock reference in context metadata for later release
- **Failure mode**: Abort (operation cannot proceed without lock)
- **Provides**: "lock" in metadata
- **Requires**: Nothing (first step)

**2. Select Candidate Step** (`select-candidate`)
- Runs promotion candidate selection algorithm (described in Failover section)
- Evaluates replicas based on: replication lag, GTID position, health, priority
- Writes selected candidate to context
- **Failure mode**: Abort (no suitable candidate = operation impossible)
- **Provides**: "Candidate" in context
- **Requires**: Nothing (uses current topology state)

**3. Promote Primary Step** (`promote-primary`)
- Executes MySQL commands to promote candidate to primary:
  1. Stop replication on candidate (`STOP REPLICA` or `STOP SLAVE`)
  2. Reset replication state (`RESET REPLICA ALL` or `RESET SLAVE ALL`)
  3. Disable read-only mode (`SET GLOBAL read_only = OFF`)
- Updates context with new primary reference
- **Failure mode**: Abort (critical operation)
- **Provides**: "NewPrimary" in context
- **Requires**: "Candidate" from previous step

**4. Repoint Replicas Step** (`repoint-replicas`)
- Iterates through all replicas (excluding promoted node)
- For each replica:
  1. Stop replication
  2. Configure new source (promoted primary)
  3. Start replication
- Uses version-aware MySQL client for syntax compatibility
- Updates context with list of successfully repointed nodes
- **Failure mode**: Configurable (Abort or Continue)
- **Provides**: "RepointedNodes" in context
- **Requires**: "NewPrimary" from promote step

**5. Release Lock Step** (`release-lock`)
- Releases distributed lock acquired at operation start
- Retrieves lock reference from context metadata
- **Failure mode**: Continue (lock will eventually expire via TTL)
- **Provides**: Nothing
- **Requires**: "lock" from acquire-lock step

**6. Intermediate Failover Steps:**
- **Identify Subtree** (`identify-subtree`): Determines all nodes in failed intermediate's subtree
- **Select Child Candidate** (`select-child-candidate`): Selects best child to promote to intermediate
- **Promote to Intermediate** (`promote-to-intermediate`): Configures child as new intermediate
- **Repoint Children** (`repoint-children`): Repoints remaining children to new intermediate or parent

Each step is independently testable and can be composed into various operation workflows via configuration.

### Custom Steps

Users can inject custom steps for integrations with external systems. Custom steps follow the same interface contract as built-in steps.

#### 1. Shell Script Step

**⚠️ Security Warning**: Shell script execution poses security risks including command injection, arbitrary code execution, and privilege escalation. Implementation must:
- Sanitize all template variables before expansion
- Validate and allowlist permitted commands/scripts
- Run scripts with minimal privileges (dedicated sentinel user)
- Avoid passing sensitive data (passwords, tokens) as arguments
- Consider using a restricted shell environment

**Capabilities:**
- Execute arbitrary shell commands or scripts during operation workflow
- Template expansion: Replace placeholders like `{{.NewPrimary.Host}}` with actual values from operation context
- Environment variable injection: Operation metadata passed as environment variables (e.g., `SENTINEL_OPERATION`, `SENTINEL_CLUSTER_ID`, `SENTINEL_NEW_PRIMARY_HOST`)
- Timeout enforcement: Command execution bounded by configurable timeout
- Async execution: Optional fire-and-forget mode for non-critical operations (notifications, cleanup)

**Configuration Properties:**
- Name (step identifier)
- Command/script path with template variables
- Timeout duration
- Failure mode (Abort or Continue)
- Async flag (true for fire-and-forget)
- Working directory

**Use Cases:**
- DNS updates to point application to new primary
- External monitoring system notifications
- Custom validation checks (capacity, network connectivity)
- Legacy system integration

#### 2. HTTP Webhook Step

**Capabilities:**
- POST operation context as JSON to external HTTP endpoint
- Configurable HTTP method (POST, PUT, PATCH)
- Custom headers (authentication tokens, content negotiation)
- Timeout enforcement
- Async execution for notifications
- HTTP status code validation (>=400 = failure)

**Configuration Properties:**
- Name (step identifier)
- URL endpoint
- HTTP method
- Custom headers (map of key-value pairs)
- Timeout duration
- Failure mode (Abort or Continue)
- Async flag

**Payload:**
- Entire operation context serialized as JSON
- Includes operation type, cluster ID, old/new primary details, timestamps
- Recipient can parse and process failover metadata

**Use Cases:**
- Slack/PagerDuty/incident management notifications
- Triggering downstream automation workflows
- Updating external configuration management systems
- Alerting and audit trail integration

### Configuration-Driven Step Injection

Users compose operations by declaring steps in YAML configuration. Steps execute sequentially in defined order.

**Configuration Schema:**

Each operation (failover, intermediate_failover, etc.) has a `steps` list where each step defines:
- **type**: Step type identifier (e.g., `builtin.acquire_lock`, `custom.shell_script`, `custom.http_webhook`)
- **config**: Step-specific configuration (map of key-value pairs)

**Example Failover Configuration:**

1. `builtin.acquire_lock` with TTL configuration
2. `builtin.validate_state` (checks cluster health)
3. `custom.shell_script` for capacity validation (Abort on failure)
4. `builtin.select_candidate`
5. `custom.shell_script` for DNS update (Abort on failure, critical)
6. `builtin.promote_primary`
7. `builtin.repoint_replicas`
8. `custom.http_webhook` for Slack notification (Continue on failure, async)
9. `builtin.release_lock`

**Example Intermediate Failover Configuration:**

1. `builtin.acquire_lock`
2. `builtin.identify_subtree`
3. `builtin.select_child_candidate`
4. `builtin.promote_to_intermediate`
5. `builtin.repoint_children`
6. `custom.shell_script` for notification (Continue on failure, async)
7. `builtin.release_lock`

**Design Principles:**
- Steps execute in declared order
- Critical steps (lock acquisition, promotion) use Abort failure mode
- Non-critical steps (notifications) use Continue failure mode
- Async steps suitable for fire-and-forget operations
- Step dependencies validated at operation load time

### Step Factory

The **Step Factory** instantiates step implementations from YAML configuration. It acts as the bridge between declarative configuration and concrete step objects.

**Responsibilities:**

1. **Step Type Routing**: Maps step type strings (e.g., `builtin.acquire_lock`) to concrete step implementations
2. **Configuration Parsing**: Extracts type-specific configuration from generic key-value map
3. **Dependency Injection**: Provides steps with required dependencies:
   - MySQL client (for promotion, replication operations)
   - Leader election backend (for lock acquisition)
   - State store (for state persistence)
   - Topology reference (for candidate selection)
4. **Default Values**: Applies sensible defaults for optional configuration (e.g., 60s TTL, 30s timeout)
5. **Validation**: Ensures required configuration properties present
6. **Extensibility**: Plugin mechanism for registering custom step types

**Factory Interface Pattern:**
- Single method: `CreateStep(stepType string, config map[string]interface{}) (OperationStep, error)`
- Returns error for unknown step types or invalid configuration
- Supports both built-in and custom step types

**Built-In Step Creation:**
- Parses type-specific configuration (TTL for locks, timeout for operations)
- Injects shared dependencies (clients, state stores)
- Returns fully initialized step ready for execution

**Custom Step Creation:**
- Shell script steps: Parse command, timeout, failure mode, async flag
- HTTP webhook steps: Parse URL, method, headers, timeout, failure mode, async flag
- Validates required properties present

**Extensibility:**
- Registerable step types for third-party extensions
- Custom factories can extend or override default step creation

### Operation Executor

The **Operation Executor** orchestrates step execution, handles failures according to step policies, and tracks operation progress.

**Operation Structure:**
- Operation name (identifier, e.g., "failover", "intermediate_failover")
- Ordered list of steps to execute

**Validation (Pre-Execution):**

Before executing an operation, the executor validates the step sequence:

1. **Async/Abort Conflict Check**: Async steps cannot have Abort failure mode (async = fire-and-forget, cannot abort parent operation)
2. **Dependency Resolution**: For each step:
   - Check that all required context fields are provided by previous steps
   - Track what each step provides for subsequent steps
   - Fail fast if dependency chain broken (e.g., step requires "Candidate" but no prior step provides it)

Validation ensures operation configuration is consistent before execution begins.

**Execution Flow:**

1. **Initialize Operation Context**: Set start time, operation metadata
2. **Sequential Step Execution**: For each step in order:
   - Log step execution start
   - Execute step with context and operation context
   - If step succeeds:
     - Add step name to completed steps list
     - Continue to next step
   - If step fails:
     - Log error with step name and error details
     - Record failed step in operation context
     - Check failure mode:
       - **Abort (0)**: Return error immediately, stop operation, mark operation as failed
       - **Continue (1)**: Log warning, mark step as completed, continue to next step
3. **Finalize Context**: Set end time after all steps complete or operation aborts
4. **Return Result**: Success if all critical steps passed, failure if any Abort-mode step failed

**Error Handling Strategy:**
- Critical steps (locks, promotions, state changes) use Abort mode
- Non-critical steps (notifications, optional cleanup) use Continue mode
- Async steps implicitly use Continue mode (cannot wait for result)
- Failed step name preserved in context for audit and debugging

**Operation Lifecycle:**

1. **Construction**: Factory creates operation from YAML configuration
2. **Validation**: Validate step dependencies and configuration
3. **Execution**: Execute steps with failure handling
4. **Persistence**: Operation context persisted to state store for audit
5. **Cleanup**: Resources released (locks, connections)

---

## API Architecture

Sentinel provides a **REST API** for all management operations. The UI is a thin client that communicates exclusively through the API.

### Design Principles

1. **REST-based**: Standard HTTP methods, JSON payloads
2. **Stateless**: API instances don't maintain session state
3. **Leader-aware**: Write operations use leader election internally
4. **Versioned**: `/api/v1/...` for backward compatibility
5. **Delegated Auth**: Authentication handled by external service (sidecar, reverse proxy)
6. **TLS/HTTPS**: Transport security is infrastructure responsibility (reverse proxy, load balancer, service mesh). Sentinel API serves plain HTTP by default, with optional TLS support via `--tls-cert` and `--tls-key` flags.

### API Structure

```
/api/v1
├── /groups                    # Group management
├── /clusters                  # Cluster management
├── /nodes                     # Node management
├── /topology                  # Topology operations
├── /operations                # Operation execution & history
├── /config                    # Configuration management
├── /health                    # Health & status
└── /audit                     # Audit log
```

### Core Endpoints

#### Groups

```
GET    /api/v1/groups
GET    /api/v1/groups/{group_id}
POST   /api/v1/groups
PUT    /api/v1/groups/{group_id}
DELETE /api/v1/groups/{group_id}
```

**Example Response:**
```json
{
  "id": "production",
  "name": "Production Environment",
  "description": "Production MySQL clusters",
  "cluster_count": 5,
  "created_at": "2024-01-15T10:00:00Z",
  "updated_at": "2024-01-20T15:30:00Z"
}
```

#### Clusters

```
GET    /api/v1/groups/{group_id}/clusters
GET    /api/v1/clusters/{cluster_id}
POST   /api/v1/groups/{group_id}/clusters
PUT    /api/v1/clusters/{cluster_id}
DELETE /api/v1/clusters/{cluster_id}

# Cluster topology & health
GET    /api/v1/clusters/{cluster_id}/topology
GET    /api/v1/clusters/{cluster_id}/health
```

**Topology Response:**
```json
{
  "cluster_id": "prod-mysql-1",
  "group_id": "production",
  "discovered_at": "2024-01-20T16:00:00Z",
  "primary": {
    "id": "node-1",
    "host": "10.0.1.5",
    "port": 3306,
    "role": "primary",
    "version": "8.0.32",
    "read_only": false,
    "replication_lag": null
  },
  "nodes": [
    {
      "id": "node-2",
      "host": "10.0.1.6",
      "port": 3306,
      "role": "intermediate",
      "source_node_id": "node-1",
      "downstream_count": 2,
      "replication_lag": "0s",
      "io_thread_running": true,
      "sql_thread_running": true
    },
    {
      "id": "node-3",
      "host": "10.0.1.7",
      "port": 3306,
      "role": "replica",
      "source_node_id": "node-2",
      "replication_lag": "2s",
      "io_thread_running": true,
      "sql_thread_running": true
    }
  ],
  "tree": {
    "node_id": "node-1",
    "depth": 0,
    "children": [
      {
        "node_id": "node-2",
        "depth": 1,
        "children": [
          {
            "node_id": "node-3",
            "depth": 2,
            "children": []
          }
        ]
      }
    ]
  }
}
```

#### Nodes

```
GET    /api/v1/nodes/{node_id}
POST   /api/v1/clusters/{cluster_id}/nodes
PUT    /api/v1/nodes/{node_id}
DELETE /api/v1/nodes/{node_id}

# Node health
GET    /api/v1/nodes/{node_id}/health
```

#### Topology Operations

```
# Manual failover
POST   /api/v1/clusters/{cluster_id}/operations/failover
{
  "reason": "Planned maintenance",
  "candidate_node_id": "node-2"  // Optional, auto-select if not provided
}

# Graceful takeover (no failure, planned switchover)
POST   /api/v1/clusters/{cluster_id}/operations/graceful-takeover
{
  "new_primary_node_id": "node-2",
  "reason": "Planned switchover"
}

# Move node (repoint to different source)
POST   /api/v1/nodes/{node_id}/operations/move
{
  "new_source_node_id": "node-5",
  "move_subtree": true  // Move downstream nodes too?
}

# Promote to intermediate
POST   /api/v1/nodes/{node_id}/operations/promote-to-intermediate

# Detach from replication
POST   /api/v1/nodes/{node_id}/operations/detach

# Get promotion candidates
GET    /api/v1/clusters/{cluster_id}/promotion-candidates
GET    /api/v1/nodes/{node_id}/promotion-candidates  // For intermediate
```

**Operation Response (Async):**
```json
{
  "operation_id": "op-abc123",
  "operation_type": "failover",
  "cluster_id": "prod-mysql-1",
  "status": "running",
  "started_at": "2024-01-20T16:05:00Z",
  "triggered_by": "john.doe@company.com",
  "current_step": "promote-primary",
  "completed_steps": ["acquire-lock", "validate-state", "select-candidate"],
  "message": "Promoting node-2 to primary"
}
```

#### Operations History

```
# List operations
GET    /api/v1/operations?cluster_id={id}&status={status}&limit={n}

# Get operation details
GET    /api/v1/operations/{operation_id}

# Get operation logs
GET    /api/v1/operations/{operation_id}/logs
```

**Operation Detail Response:**
```json
{
  "operation_id": "op-abc123",
  "operation_type": "failover",
  "cluster_id": "prod-mysql-1",
  "status": "completed",
  "started_at": "2024-01-20T16:05:00Z",
  "completed_at": "2024-01-20T16:06:30Z",
  "duration": "90s",
  "triggered_by": "john.doe@company.com",
  "context": {
    "old_primary": {"id": "node-1", "host": "10.0.1.5"},
    "new_primary": {"id": "node-2", "host": "10.0.1.6"},
    "affected_nodes": ["node-3", "node-4"]
  },
  "steps": [
    {
      "name": "acquire-lock",
      "status": "completed",
      "started_at": "2024-01-20T16:05:00Z",
      "completed_at": "2024-01-20T16:05:01Z",
      "duration": "1s"
    },
    {
      "name": "promote-primary",
      "status": "completed",
      "started_at": "2024-01-20T16:05:15Z",
      "completed_at": "2024-01-20T16:05:20Z",
      "duration": "5s"
    }
  ]
}
```

#### Configuration Management

```
# Get configuration (with hierarchy)
GET    /api/v1/config/global
GET    /api/v1/config/groups/{group_id}
GET    /api/v1/config/clusters/{cluster_id}
GET    /api/v1/config/nodes/{node_id}

# Get effective configuration (merged hierarchy)
GET    /api/v1/config/effective/clusters/{cluster_id}
GET    /api/v1/config/effective/nodes/{node_id}

# Update configuration
PUT    /api/v1/config/global
PUT    /api/v1/config/groups/{group_id}
PUT    /api/v1/config/clusters/{cluster_id}
PUT    /api/v1/config/nodes/{node_id}

# Get pending configuration
GET    /api/v1/config/pending/clusters/{cluster_id}

# Apply pending configuration
POST   /api/v1/config/clusters/{cluster_id}/apply
```

**Configuration Response:**
```json
{
  "level": "cluster",
  "cluster_id": "prod-mysql-1",
  "active": {
    "auto_failover_enabled": true,
    "failover_timeout_seconds": 300,
    "intermediate_failover_strategy": "promote_child"
  },
  "pending": {
    "auto_failover_enabled": false,
    "failover_timeout_seconds": 600
  },
  "pending_since": "2024-01-20T15:00:00Z",
  "changed_by": "jane.doe@company.com"
}
```

#### Health & Status

```
# Sentinel instance health
GET    /api/v1/health

# Cluster health summary
GET    /api/v1/clusters/{cluster_id}/health

# All clusters health
GET    /api/v1/health/clusters
```

**Health Response:**
```json
{
  "status": "healthy",
  "instance_id": "sentinel-1",
  "is_leader": false,
  "leader_instance_id": "sentinel-2",
  "version": "1.0.0",
  "uptime": "2h30m",
  "leader_election_backend": "etcd",
  "state_store_backend": "mysql",
  "monitored_clusters": 5,
  "components": {
    "leader_election": {"status": "healthy", "latency_ms": 5},
    "state_store": {"status": "healthy", "latency_ms": 10},
    "mysql_connections": {"status": "healthy", "active": 15}
  }
}
```

#### Audit Log

```
GET    /api/v1/audit?cluster_id={id}&user={user}&operation={type}&from={time}&to={time}&limit={n}
```

**Audit Response:**
```json
{
  "events": [
    {
      "id": "audit-xyz789",
      "timestamp": "2024-01-20T16:05:00Z",
      "operation_type": "failover",
      "cluster_id": "prod-mysql-1",
      "user": "john.doe@company.com",
      "action": "initiated",
      "details": {
        "reason": "Planned maintenance",
        "old_primary": "node-1",
        "new_primary": "node-2"
      },
      "result": "success"
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 50
}
```

### Leader Election for Write Operations

Write operations (failover, topology changes, config changes) require leader election for coordination across multiple Sentinel instances.

**Write Operation Pattern:**

1. **Request Parsing**: Extract operation parameters from JSON request body
2. **Leader Lock Acquisition**: Attempt to acquire distributed lock for this specific operation
   - Lock key includes: operation type + cluster ID (per-resource locking)
   - Lock TTL: Typically 5 minutes (configurable)
   - If lock already held: Return HTTP 409 Conflict ("Another instance is processing this operation")
3. **Operation Record Creation**: Create operation record in state store with:
   - Unique operation ID (generated)
   - Operation type (int enum)
   - Cluster ID
   - Status: "running"
   - Triggered by user (from `X-Authenticated-User` header)
   - Start timestamp
4. **Async Execution**: Launch operation in background goroutine
5. **Immediate Response**: Return HTTP 202 Accepted with operation ID and status
6. **Lock Release**: Defer lock release after operation completion (or TTL expiration)

**Operation Tracking:**

Background goroutine updates operation status in state store as it progresses:
- Update current step name
- Append completed steps
- Record failures and error messages
- Set final status (completed/failed)
- Set end timestamp

Clients poll `GET /api/v1/operations/{id}` to monitor progress.

**Read Operations:**

Read operations (GET endpoints) served by any Sentinel instance without leader election:
- Topology queries
- Health checks
- Configuration retrieval
- Operation history
- Audit logs

All instances have consistent view via shared state store.

### Authentication & Authorization

Sentinel **delegates** authentication to external services:

```
┌─────────┐      ┌──────────────┐      ┌──────────────┐
│ Client  │─────>│ Auth Proxy   │─────>│ Sentinel API │
│  (UI)   │      │ (Sidecar)    │      │              │
└─────────┘      └──────────────┘      └──────────────┘
                       │
                       ▼
                 ┌──────────────┐
                 │ Auth Provider│
                 │ (LDAP/OAuth) │
                 └──────────────┘
```

**API receives authenticated requests with headers:**
- `X-Authenticated-User`: User identifier (e.g., `john.doe@company.com`)
- `X-User-Groups`: Comma-separated group memberships (e.g., `admins,mysql-operators`)
- `X-User-Roles`: Comma-separated role assignments (e.g., `cluster-admin`)

**Simple Authorization (optional):**

Sentinel can optionally enforce authorization rules via middleware:

**Authorization Rule Structure:**
- **Resource**: Resource identifier (e.g., `cluster:prod-mysql-1`, `group:production`)
- **Action**: Operation type (e.g., `failover`, `read`, `write`, `config_change`)
- **Roles**: List of roles permitted to perform action on resource

**Authorization Middleware:**
- Extracts user and roles from request headers (set by auth proxy)
- Checks if user's roles match rule requirements for requested resource/action
- Returns HTTP 403 Forbidden if unauthorized
- Bypasses check if authorization disabled (delegated auth only mode)

**Authorization Scope:**
- Per-cluster permissions (e.g., only allow failover on specific clusters)
- Per-group permissions (e.g., only allow access to production group)
- Global permissions (e.g., admin can access all resources)

**Note:** Full RBAC can be added later if needed. Start simple with delegated auth. Authentication validation middleware just checks that `X-Authenticated-User` header exists (user identity verified by auth proxy).

### API Versioning

- Current: `/api/v1/...`
- Future: `/api/v2/...` for breaking changes
- Version in URL for simplicity
- Maintain backward compatibility within major version

### Error Handling

**Standard error response:**
```json
{
  "error": {
    "code": "CLUSTER_NOT_FOUND",
    "message": "Cluster 'prod-mysql-1' not found",
    "details": {
      "cluster_id": "prod-mysql-1"
    },
    "request_id": "req-123456"
  }
}
```

**HTTP Status Codes:**
- `200 OK` - Success
- `201 Created` - Resource created
- `202 Accepted` - Async operation started
- `400 Bad Request` - Invalid input
- `401 Unauthorized` - Missing authentication
- `403 Forbidden` - Insufficient permissions
- `404 Not Found` - Resource not found
- `409 Conflict` - Resource conflict (e.g., another operation in progress)
- `422 Unprocessable Entity` - Validation error
- `500 Internal Server Error` - Server error
- `503 Service Unavailable` - Service unavailable (e.g., leader election failed)

### API Implementation Structure

**API Server Components:**

The API server requires the following dependencies:
- **HTTP Router**: Request routing and middleware chaining (e.g., chi, gorilla/mux, standard library)
- **State Store**: Interface for persistent storage access
- **Leader Election**: Interface for distributed lock coordination
- **MySQL Client**: Interface for cluster operations

**Middleware Stack:**

Applied to all `/api/v1/*` routes in order:
1. **Logging Middleware**: Request/response logging with timing, status codes, request IDs
2. **Authentication Middleware**: Validates `X-Authenticated-User` header present (set by auth proxy)
3. **CORS Middleware**: Cross-origin resource sharing headers for browser clients
4. **Authorization Middleware** (optional): Role-based access control

**Route Organization:**

Routes organized hierarchically by resource:

**Groups:**
- `GET /groups` - List all groups
- `POST /groups` - Create new group
- `GET /groups/{group_id}` - Get group details
- `PUT /groups/{group_id}` - Update group
- `DELETE /groups/{group_id}` - Delete group

**Clusters:**
- `GET /clusters/{cluster_id}` - Get cluster details
- `GET /clusters/{cluster_id}/topology` - Get topology tree
- `GET /clusters/{cluster_id}/health` - Get health summary
- `POST /clusters/{cluster_id}/operations/failover` - Trigger failover
- `POST /clusters/{cluster_id}/operations/graceful-takeover` - Trigger takeover

**Operations:**
- `GET /operations` - List operations (with filters)
- `GET /operations/{operation_id}` - Get operation details
- `GET /operations/{operation_id}/logs` - Get operation logs

Similar patterns for nodes, configuration, health, and audit endpoints.

**Handler Pattern:**
- Handlers parse request (URL params, query params, JSON body)
- Validate input
- Check authorization (if enabled)
- Acquire leader lock for write operations
- Execute business logic (via service layer)
- Return JSON response with appropriate HTTP status

### UI Architecture

The UI is a **single-page application (SPA)** embedded in the Go binary, served from the same process as the API.

#### Technology Stack

- **Framework**: React 18+ with TypeScript
- **Build Tool**: Vite (fast dev server, optimized production builds)
- **Styling**: Tailwind CSS (utility-first CSS framework)
- **Components**: Radix UI (unstyled, accessible primitives)
- **Data Fetching**: TanStack Query (React Query) for API state management
- **Routing**: React Router
- **Topology Visualization**: D3.js or Cytoscape.js for interactive tree graphs
- **Forms**: React Hook Form + Zod for validation
- **Icons**: Lucide React or Heroicons

#### Deployment Architecture

```
┌─────────────────────────────────────────┐
│         Sentinel Binary                  │
│                                          │
│  ┌────────────────────────────────────┐ │
│  │  Embedded UI (React SPA)           │ │
│  │  Served at:  /                     │ │
│  │              /clusters              │ │
│  │              /operations            │ │
│  │              /* (SPA fallback)      │ │
│  └────────────────────────────────────┘ │
│                                          │
│  ┌────────────────────────────────────┐ │
│  │  REST API                          │ │
│  │  Served at:  /api/v1/*             │ │
│  └────────────────────────────────────┘ │
│                                          │
│         Single Port: 8080                │
└─────────────────────────────────────────┘
```

#### Go Embedding

The UI is embedded into the Go binary using Go's `embed` package (standard library feature).

**Embedding Strategy:**

1. **Build-time Embedding**: Use `//go:embed` directive to include UI dist files at compile time
2. **Virtual File System**: Embedded files accessible via `embed.FS` interface
3. **Subdirectory Extraction**: Extract UI subdirectory for clean path handling
4. **HTTP Serving**: Serve embedded files via standard `http.FileServer`

**Routing Strategy:**

- **API Routes** (`/api/v1/*`): Served by API handlers with middleware
- **Static Files**: Served from embedded filesystem if file exists
- **SPA Fallback**: Serve `index.html` for client-side routes (any non-file path)

**SPA Handler Pattern:**

The SPA handler enables client-side routing:
1. Check if requested path corresponds to a static file in embedded filesystem
2. If file exists (JS, CSS, images): Serve the file
3. If file doesn't exist (client-side route like `/clusters/prod-mysql-1`): Serve `index.html`
4. React Router handles navigation client-side

This allows the UI to use browser history API for routing without server-side configuration.

#### Build Process

**Development Mode:**
- **UI Development**: Start Vite dev server on `:5173` with hot module replacement
- **API Development**: Start Go API server on `:8080` (watching for changes optional)
- **API Proxy**: Vite proxies `/api` requests to Go server for seamless development
- **Fast Iteration**: Changes to UI instantly reflected, no rebuild needed

**Production Build:**
1. **Build UI**: Run Vite build to produce optimized static files in `ui/dist/`
   - Minification, tree-shaking, code splitting
   - Asset hashing for cache busting
2. **Build Go Binary**: Compile Go with UI embedded
   - Strip debug symbols (`-ldflags="-s -w"`) for smaller binary
   - Output single executable to `bin/sentinel`

**Build Targets:**
- `dev-ui`: Launch Vite development server
- `dev-api`: Launch Go API server
- `build-ui`: Build React app to production bundle
- `build`: Build UI, then build Go binary with embedded UI
- `clean`: Remove build artifacts (dist, node_modules, binary)

**Vite Configuration:**

Vite build tool configured with:
- **React Plugin**: JSX/TSX transformation, Fast Refresh
- **Build Output**: `dist/` directory, clean before build
- **Dev Server**:
  - Port: 5173
  - API Proxy: Forward `/api/*` requests to `http://localhost:8080` (Go server)
  - Enables local development without CORS issues

**Deployment Comparison:**
- **Development**: UI runs on Vite dev server (`:5173`), API calls proxied to Go server (`:8080`), fast iteration with instant updates
- **Production**: UI built to static files, embedded in Go binary via `//go:embed`, single binary deployment, no external dependencies

#### UI Features

1. **Dashboard**
   - Overview of all groups and clusters
   - Health status summary (healthy, warning, critical)
   - Recent operations
   - Alert notifications

2. **Topology View**
   - Interactive tree visualization using D3.js
   - Drag-and-drop node repointing
   - Real-time health indicators (green/yellow/red)
   - Node details on hover/click
   - Replication lag display

3. **Operations**
   - Trigger failover with candidate selection
   - Graceful takeover
   - Manual topology changes (move node, promote to intermediate)
   - Operation progress tracking (polling)
   - Operation history with filters

4. **Configuration Management**
   - Hierarchical config editor (Global → Group → Cluster → Node)
   - Diff view showing pending vs active config
   - Apply button with confirmation
   - Config validation

5. **Audit Log**
   - Searchable operation history
   - Filter by user, cluster, operation type, date range
   - Detailed operation context and results

6. **Node Management**
   - Add/remove nodes
   - Edit node credentials
   - View node health metrics

#### Component Structure

```
ui/src/
├── components/
│   ├── ui/               # Radix UI + Tailwind components
│   │   ├── Button.tsx
│   │   ├── Dialog.tsx
│   │   ├── Select.tsx
│   │   └── ...
│   ├── topology/
│   │   ├── TopologyTree.tsx
│   │   ├── NodeCard.tsx
│   │   └── ...
│   ├── operations/
│   │   ├── FailoverDialog.tsx
│   │   ├── OperationProgress.tsx
│   │   └── ...
│   └── layout/
│       ├── Sidebar.tsx
│       ├── Header.tsx
│       └── ...
├── pages/
│   ├── Dashboard.tsx
│   ├── ClusterTopology.tsx
│   ├── Operations.tsx
│   ├── Configuration.tsx
│   └── AuditLog.tsx
├── hooks/
│   ├── useTopology.ts
│   ├── useOperations.ts
│   └── useConfig.ts
├── lib/
│   ├── api.ts            # API client
│   ├── types.ts          # TypeScript types
│   └── utils.ts
└── main.tsx
```

#### API Client Pattern

**Data Fetching Strategy:**

Use TanStack Query (React Query) for declarative API state management:

**Query Hooks (GET operations):**
- Wrap API fetch calls in `useQuery` hooks
- Define query keys for caching (e.g., `['topology', clusterId]`)
- Implement fetch function with error handling
- Configure polling interval for real-time updates
- Example: `useClusterTopology(clusterId)` polls topology every 5 seconds

**Mutation Hooks (POST/PUT/DELETE operations):**
- Wrap write operations in `useMutation` hooks
- Define mutation function with typed parameters
- Handle success/error states
- Invalidate related queries on success for cache consistency
- Example: `useTriggerFailover()` posts failover request, invalidates topology query

**Polling Configuration:**

Configurable intervals via environment variables (compile-time):
- `REACT_APP_TOPOLOGY_POLL_INTERVAL_MS`: Topology refresh rate (default: 5000ms)
- `REACT_APP_OPERATION_POLL_INTERVAL_MS`: Operation progress polling (default: 2000ms)

**API Client Structure:**
- Base URL: `/api/v1` (relative to current origin)
- Fetch API for HTTP requests
- JSON content type for all requests
- Throw errors on non-OK responses for query/mutation error handling
- TypeScript interfaces for request/response types

#### Example UI Flow - Manual Failover

**User Journey:**

1. **Navigate**: User opens cluster topology page (`/clusters/prod-mysql-1`)
2. **Load Topology**: UI fetches `GET /api/v1/clusters/prod-mysql-1/topology` via React Query
3. **Initiate Failover**: User clicks "Failover" button
4. **Candidate Selection**: Modal dialog shows:
   - Option to auto-select candidate (leave empty)
   - Dropdown of available promotion candidates
   - Reason text field (required for audit)
5. **Execute**: User clicks "Execute" button
6. **API Request**: UI posts `POST /api/v1/clusters/prod-mysql-1/operations/failover` with payload
7. **Async Response**: API returns `{"operation_id": "op-123", "status": "running"}` (HTTP 202 Accepted)
8. **Progress Tracking**: UI shows progress modal with:
   - Operation ID
   - Current step name
   - Progress bar (completed_steps / total_steps)
   - Real-time log updates
9. **Polling**: UI polls `GET /api/v1/operations/op-123` every 2 seconds (configurable)
10. **Progress Updates**: Progress bar and current step update on each poll response
11. **Completion**: Operation status changes to "completed" or "failed"
12. **Notification**: UI shows success toast notification or error alert
13. **Auto-Refresh**: React Query automatically refetches topology to show new primary

#### Styling with Tailwind + Radix UI

**UI Component Pattern:**

Sentinel uses a combination of Tailwind CSS (utility classes) and Radix UI (unstyled primitives) for building accessible, customizable components.

**Component Structure:**

1. **Base Components** (in `components/ui/`):
   - Wrap Radix UI primitives with Tailwind styling
   - Use class-variance-authority (CVA) for variant management
   - Support props for variant (default, destructive, outline, ghost, etc.)
   - Support props for size (sm, default, lg)
   - Forward refs for proper DOM access
   - TypeScript interfaces for type safety

2. **Variant System**:
   - Define base styles applied to all variants
   - Define variant-specific styles (colors, borders, backgrounds)
   - Define size variants (height, padding, text size)
   - Specify default variants
   - Compose variants via props

3. **Example Component Categories**:
   - **Button**: Multiple variants (primary, destructive, outline, ghost), sizes
   - **Dialog**: Modal overlays with Radix Dialog primitive
   - **Select**: Dropdown menus with Radix Select primitive
   - **Toast**: Notification system with Radix Toast primitive
   - **Form**: Input fields, labels, error messages with validation

**Benefits:**
- **Accessibility**: Radix UI provides ARIA attributes, keyboard navigation, focus management out-of-the-box
- **Consistency**: CVA ensures variant application is type-safe and consistent
- **Customization**: Tailwind utilities allow rapid styling adjustments
- **Type Safety**: TypeScript interfaces ensure prop correctness

---

**Benefits of This Stack:**
- ✅ **Fast development**: Vite hot reload, Tailwind utility classes
- ✅ **Type-safe**: TypeScript end-to-end
- ✅ **Accessible**: Radix UI components are WCAG compliant
- ✅ **Modern**: Latest React patterns, hooks, suspense
- ✅ **Small bundle**: Vite optimizes, tree-shakes, code-splits
- ✅ **Single binary**: Simple deployment
- ✅ **Beautiful UI**: Tailwind + Radix UI make it easy to build polished interfaces

---

## Operational Modes

Sentinel supports different operational modes for various deployment scenarios. The mode affects **only** configuration source and cluster discovery - the core logic remains the same.

### Mode Types

Sentinel defines operational modes as an int enum:
- **ModeMulti (0)**: Multiple clusters mode (default, v1.0 focus)
- **ModeSingle (1)**: Single cluster mode, cloud-native (v1.1+ future enhancement)

### Multi Mode (Default - V1.0 Focus)

**Use Case:** Central control plane monitoring multiple MySQL clusters.

**Characteristics:**
- Monitors multiple clusters and groups
- Configuration from file or StateStore
- Persistent state (topology, operations, audit)
- Recommended for traditional infrastructure

**Configuration:**

YAML file (`config.yaml`) structure for multi mode:
- **mode**: Set to `multi` (default)
- **state_store**:
  - **type**: Storage backend (mysql, postgresql, sqlite)
  - **dsn**: Data source name (connection string with credentials)
- **leader_election**:
  - **backend**: Coordination backend (etcd, consul, zookeeper, database)
  - **endpoints**: List of backend server URLs
- **groups**: List of cluster groups
  - Each group has **id** and list of **clusters**
  - Each cluster has **id** and **seed_nodes** (initial discovery points)
  - Seed nodes specified with **host** and **port**

Example: Production group with 2 clusters, each with initial seed node for topology discovery.

### Single Mode (Cloud-Native - V1.1+)

**Use Case:** One Sentinel instance per cluster, deployed as sidecar or alongside cluster.

**Key Differences from Multi Mode:**
- Monitors exactly **one cluster** (vs multiple)
- **Stateless**: SQLite `:memory:` storage (vs persistent database)
- **Environment variable** configuration (vs YAML file)
- Logs to stdout as JSON (vs database audit table)
- Cloud-native friendly (Kubernetes, Docker, sidecar pattern)

**Note:** Single mode is a **future enhancement** (v1.1+). If it adds complexity to v1.0, it will be deferred.

**📄 Full documentation**: See [SINGLE_MODE_V1.1.md](./SINGLE_MODE_V1.1.md) for detailed specifications, deployment examples, and implementation notes.

### Component Toggles

All modes support selectively disabling components via command-line flags:

**Component Types:**
1. **Watch**: Automatic monitoring and failover (background worker)
2. **API**: REST API server for manual operations
3. **UI**: Web-based user interface (requires API)

**Deployment Modes:**
- **Full (default)**: Watch + API + UI enabled
  - Command: `sentinel` (no flags)
- **Watch-only**: No API or UI, autonomous failover agent
  - Command: `sentinel --no-api --no-ui`
  - Use case: Lightweight agent that only watches and fails over
- **API-only**: Manual operations only, no automatic monitoring
  - Command: `sentinel --no-watch`
  - Use case: Control plane for manual operations via API/UI
- **Headless**: API + Watch, no UI
  - Command: `sentinel --no-ui`
  - Use case: Programmatic access via API, no web interface

**Component Configuration:**

Configuration structure tracks which components are enabled (boolean flags):
- **API**: Serve REST API
- **UI**: Serve embedded UI
- **Watch**: Monitor and auto-failover

**Defaults**: All components enabled (API=true, UI=true, Watch=true)

**Validation Rules:**
- At least one component must be enabled (cannot disable all)
- UI requires API (UI is a client of the API, cannot run standalone)

### Configuration Interface (Abstraction)

To support multiple configuration sources, Sentinel defines a **ConfigProvider** interface that abstracts where configuration comes from.

**Interface Responsibilities:**

1. **GetMode()**: Return operational mode (Multi or Single)
2. **GetClusters()**: Return list of clusters to monitor
   - Single mode: Returns 1 cluster
   - Multi mode: Returns N clusters
3. **GetGlobalConfig()**: Return global configuration settings
4. **Watch()** (optional): Subscribe to configuration changes for dynamic updates

**Provider Implementations:**

- **EnvConfigProvider** (v1.0): Reads from environment variables (single mode)
- **FileConfigProvider** (v1.0): Reads from YAML file (multi mode)
- **EtcdConfigProvider** (v1.1+): Reads from etcd K/V store
- **ConsulConfigProvider** (v1.1+): Reads from Consul K/V store

**Environment Variable Provider (Single Mode):**

Reads configuration from environment variables:
- **GetMode()**: Returns `ModeSingle` if `SENTINEL_MODE=single`, otherwise `ModeMulti`
- **GetClusters()**: In single mode, parses:
  - `SENTINEL_CLUSTER_ID`: Cluster identifier
  - `SENTINEL_SEED_NODES`: Comma-separated "host:port" list
  - `SENTINEL_MYSQL_USER`, `SENTINEL_MYSQL_PASSWORD`: Credentials
  - Returns single-item cluster list
- In multi mode: Returns error (multi mode requires file config)

**File Provider (Multi Mode):**

Reads configuration from YAML file:
- File path specified via `--config` flag
- Parses YAML into configuration structs
- Returns all groups and clusters defined in file

### Leader Election Abstraction

Leader election backend is shared between modes. The LeaderElection interface (defined earlier) has multiple implementations.

**Interface Methods:**
- **AcquireLock()**: Acquire distributed lock for operation
- **IsHealthy()**: Check backend health

**Implementation Options:**

1. **InMemoryLeaderElection** (default, single instance):
   - No distributed coordination
   - Always succeeds (lock acquisition never fails)
   - Use case: Single Sentinel instance, no HA

2. **EtcdLeaderElection** (distributed):
   - Uses etcd lease mechanism for locking
   - Automatic lock expiry via TTL
   - Use case: Multi-instance HA deployments

3. **ConsulLeaderElection** (distributed):
   - Uses Consul session and K/V locks
   - Session invalidation on failure

4. **DatabaseLeaderElection** (distributed):
   - Uses database advisory locks or SELECT FOR UPDATE
   - Works with existing database infrastructure

5. **KubernetesLeaderElection** (v1.1+):
   - Uses Kubernetes Lease API
   - Native Kubernetes integration

**Configuration:**

Multi mode YAML:
- `leader_election.backend`: Backend type (etcd, consul, zookeeper, database, in-memory)
- `leader_election.endpoints`: List of backend server URLs

Single mode environment variables:
- `SENTINEL_LEADER_ELECTION_BACKEND`: Backend type
- `SENTINEL_LEADER_ELECTION_ENDPOINTS`: Comma-separated endpoints

**Default Behavior:**

If no backend specified, use InMemoryLeaderElection:
- Suitable for single-instance deployments
- No external dependencies
- Lock acquisition always succeeds immediately

### State Storage by Mode

| Mode | State Storage | Operation History | Audit Log |
|------|---------------|-------------------|-----------|
| Multi | Configurable (MySQL, PostgreSQL, SQLite file) | Persistent | Persistent |
| Single | In-memory SQLite (`:memory:`) | Lost on restart | Logs to stdout (JSON) |

**State Store Interface:**

The StateStore interface (defined earlier in Core Interfaces section) works for both modes, with different implementations selected based on mode.

**Implementation Options:**

1. **MySQLStateStore** (multi mode):
   - Uses MySQL database for persistent storage
   - Configured via DSN: `user:pass@tcp(host:port)/database`
   - Full persistence of topology, operations, audit logs

2. **SQLiteStateStore** (both modes):
   - Multi mode: File-based SQLite with disk persistence
     - DSN: `file:/path/to/sentinel.db`
   - Single mode: In-memory SQLite (ephemeral)
     - DSN: `:memory:`
     - State lost on restart (acceptable for cloud-native deployments)

Mode selection determines which implementation to instantiate and which DSN to use.

### Observability (All Modes)

Both modes support identical observability features.

**Structured Logging:**

- Always enabled, JSON format to stdout
- Fields: timestamp, level, message, contextual attributes (cluster, node, operation)
- Example fields: `cluster`, `old_primary`, `candidate`, `operation_id`
- Captured by log aggregators (Fluent Bit, Logstash, CloudWatch, etc.)

**Prometheus Metrics:**

Always enabled, exposed on `/metrics` endpoint (port 9090 by default, separate from API port 8080).

**Key Metrics:**
- `sentinel_failovers_total{cluster,result}`: Counter of failovers (labeled by result: success/failure)
- `sentinel_cluster_health{cluster}`: Gauge of cluster health (1=healthy, 0=unhealthy)
- `sentinel_replication_lag_seconds{cluster,node}`: Gauge of replication lag per replica
- `sentinel_operation_duration_seconds{operation}`: Histogram of operation durations

Labels enable filtering and aggregation by cluster, node, operation type, result.

### Deployment Examples

#### Multi Mode (Traditional)

**Single Instance:**
- Command: `sentinel --config=/etc/sentinel/config.yaml`
- Reads configuration from YAML file
- Connects to configured state store and leader election backend

**High Availability (Multiple Instances):**
- Start multiple Sentinel instances with same configuration
- Command: `sentinel --config=/etc/sentinel/config.yaml --instance-id=sentinel-1`
- Each instance has unique ID for identification in logs/metrics
- Leader election coordinates write operations across instances
- Any instance can handle read operations

#### Single Mode (Cloud-Native) - V1.1+

See [SINGLE_MODE_V1.1.md](./SINGLE_MODE_V1.1.md) for Docker Compose, Kubernetes, and sidecar deployment examples.

### Mode Detection

Sentinel initialization uses ConfigProvider to determine mode and configure appropriate backends.

**Initialization Flow:**

1. **Mode Detection**: Query ConfigProvider.GetMode()
2. **State Store Selection**:
   - Single mode: Instantiate SQLiteStateStore with `:memory:` DSN (ephemeral)
   - Multi mode: Instantiate chosen state store (MySQL/PostgreSQL/SQLite file) with configured DSN
3. **Core Component Construction**: Same for both modes
   - Discovery Manager: Handles seed-based topology discovery
   - Monitor: Watches cluster health, triggers auto-failover
   - Failover Manager: Executes failover operations
4. **Leader Election**: Backend configured based on mode settings (in-memory vs distributed)

**Key Design Principle:**

Mode affects **only**:
- Configuration source (environment variables vs YAML file)
- State storage backend (in-memory vs persistent)
- Leader election backend selection

Mode does **not** affect:
- Discovery logic (same seed-based discovery)
- Monitoring logic (same health checks and lag tracking)
- Failover logic (same promotion algorithm and step execution)
- API endpoints (same REST API)

This ensures minimal code duplication and consistent behavior across modes.

### V1.0 Scope

For v1.0, focus on **multi mode** with:
- ✅ File-based configuration
- ✅ Multiple clusters and groups
- ✅ Persistent state (MySQL/PostgreSQL StateStore)
- ✅ Distributed leader election (etcd/Consul/Zookeeper/DB)
- ✅ Full API + UI + Watch
- ✅ Component toggles (--no-api, --no-ui, --no-watch)
- ✅ Structured logs + Prometheus metrics

**Defer to v1.1+ if complex:**
- Single mode (environment variable config)
- In-memory stateless operation
- Cloud-native deployment patterns
- DNS-based discovery
- Kubernetes-native features (CRDs, operators)

---

## Audit and User Tracking

### Audit Event Model

Every significant operation is logged as an audit event for compliance and troubleshooting.

**Audit Event Structure:**

- **ID**: Unique event identifier
- **Timestamp**: When event occurred
- **OperationType**: Type of operation (string: "failover", "config_change", "topology_change")
- **Action**: Event lifecycle stage (string: "initiated", "completed", "failed")
- **ClusterID**: Affected cluster identifier
- **User**: Who triggered the operation (from `X-Authenticated-User` header, or "system" for auto-failover)
- **Source**: Origin of operation (string: "api", "auto", "cli")
- **Result**: Outcome (string: "success", "failure")
- **Details**: Operation-specific metadata (generic map for flexibility)
- **Duration**: How long operation took (nullable, only set for completed/failed actions)
- **Error**: Error message if operation failed (nullable)

### Audit Events

**Failover Events:**

Two events logged for each failover:

1. **Failover Initiated**:
   - OperationType: "failover"
   - Action: "initiated"
   - User: Triggering user (e.g., "john.doe@company.com")
   - Source: "api" (manual) or "auto" (automatic)
   - Details include: reason (user-provided), old_primary, candidate, trigger_type

2. **Failover Completed**:
   - Action: "completed" (or "failed")
   - Result: "success" or "failure"
   - Duration: Total operation time (e.g., 45 seconds)
   - Details include: old_primary, new_primary, repointed_nodes (list), steps_completed (count)

**Configuration Change Events:**

Logged when configuration is applied:
- OperationType: "config_change"
- Action: "applied"
- User: Who made the change
- Details include:
  - level: Configuration level (global, group, cluster, node)
  - changes: Map of changed fields with before/after values

**Topology Change Events:**

Logged for manual topology operations (move node, promote to intermediate, detach):
- OperationType: "topology_change"
- Action: "completed" (or "failed")
- User: Who executed the operation
- Details include:
  - operation: Specific operation name (e.g., "move_node")
  - node_id: Target node
  - old_source, new_source: Replication source changes
  - move_subtree: Whether downstream nodes also moved

### Audit Storage

**Multi Mode (Persistent):**

Audit events stored in state store database for long-term retention and querying.

**StateStore Interface Methods:**
- **LogAuditEvent()**: Insert audit event into database
- **GetAuditEvents()**: Query audit events with filters (cluster, user, operation type, date range, result)

**Database Schema:**

Audit events table with columns:
- `id`: Unique event identifier (primary key)
- `timestamp`: Event occurrence time (indexed)
- `operation_type`: Operation type (indexed)
- `action`: Event action (initiated/completed/failed)
- `cluster_id`: Affected cluster (indexed)
- `user`: Triggering user (indexed)
- `source`: Origin (api/auto/cli)
- `result`: Outcome (success/failure)
- `details`: JSON column for operation-specific metadata
- `duration_ms`: Operation duration in milliseconds
- `error`: Error message if failed

**Indexes:**
- `(cluster_id, timestamp)`: Fast queries by cluster
- `(user, timestamp)`: Fast queries by user (compliance)
- `(operation_type, timestamp)`: Fast queries by operation type

**Single Mode (Stateless):**

Audit events logged as structured JSON to stdout:
- Log level: Info
- Log message: "audit_event"
- Fields: event_id, operation_type, action, cluster_id, user, result, details
- Captured by log aggregation systems (Fluent Bit, Logstash, CloudWatch, etc.)
- Searchable via log aggregator query interface

### User Attribution

**From API Requests:**

Manual operations triggered via API:
1. **Extract User**: Read `X-Authenticated-User` header (set by auth proxy)
2. **Default Handling**: If header missing, set user to "unknown"
3. **Operation Context**: Populate `TriggeredBy` field with user identifier
4. **Audit Event**: Include user in audit event for compliance tracking

User identity flows through:
- API request header → Operation context → Audit event → Database/logs

**From Auto-Failover:**

Automatic operations triggered by monitoring:
1. **System User**: Set user to "system" or "auto-failover"
2. **Source Field**: Set source to "auto" to distinguish from manual operations
3. **Operation Context**: Populate with system identity
4. **Audit Event**: Include system user for audit trail completeness

This allows filtering audit logs to show:
- All manual operations (source="api")
- All automatic operations (source="auto")
- Operations by specific user (user="john.doe@company.com")

### Audit Query API

**Endpoint:** `GET /api/v1/audit`

**Query Parameters:**
- `cluster_id`: Filter by cluster
- `user`: Filter by user who triggered operation
- `operation`: Filter by operation type (failover, config_change, topology_change)
- `from`: Start of time range (RFC3339 timestamp)
- `to`: End of time range (RFC3339 timestamp)
- `limit`: Max results to return (pagination)

**Response Structure:**

JSON object with:
- **events**: Array of audit events
  - Each event includes: id, timestamp, operation_type, action, cluster_id, user, source, result, duration_ms, details
- **total**: Total matching events (for pagination)
- **page**: Current page number
- **page_size**: Events per page

**Example Event:**
- ID: "evt-abc123"
- Operation type: "failover"
- Action: "completed"
- User: "john.doe@company.com"
- Result: "success"
- Duration: 45000ms (45 seconds)
- Details: old_primary, new_primary, reason

### Audit UI Features

1. **Audit Log Page**
   - Searchable table of all events
   - Filters: cluster, user, operation type, date range, result
   - Drill-down to operation details

2. **Per-Cluster Audit**
   - Show audit history for specific cluster
   - Timeline visualization

3. **Per-User Audit**
   - What operations did this user perform?
   - Useful for compliance

4. **Operation Detail View**
   - Full operation context
   - Step-by-step execution log
   - Before/after state comparison

### Compliance & Retention

**Configuration Options:**

**Audit Settings:**
- **enabled**: Enable/disable audit logging (boolean, default: true)
- **retention_days**: Auto-delete events older than N days (default: 90)

**External Audit Sinks (optional):**

Forward audit events to external systems for centralized compliance:
- **Syslog Sink**: Send events to syslog server
  - Type: "syslog"
  - Endpoint: Syslog server address with port
- **HTTP Sink**: POST events to HTTP endpoint
  - Type: "http"
  - Endpoint: HTTPS URL for webhook receiver

**Retention Job:**

Periodic background job cleans up old audit events:
- **Trigger**: Scheduled task (e.g., daily at midnight)
- **Cutoff Calculation**: Current time minus retention_days
- **Operation**: Delete all events with timestamp before cutoff
- **Implementation**: Single SQL DELETE statement with timestamp filter
- **Performance**: Indexed timestamp column ensures fast deletion

This ensures:
- Compliance with data retention policies
- Database size remains manageable
- Audit history available for configurable period

---

## Open Questions

- **Configuration apply coordination**: When a user makes config changes and clicks "apply", should this trigger a leader election to ensure only one instance applies the changes? Or is it safe for any instance to apply config changes?
