  Phase 0: Project Setup (1-2 days)

  - Project structure
  - Go modules
  - Basic logging
  - Config loading (simple YAML)

  Phase 1: Discovery Core (3-5 days)

  Goal: Discover and understand a single cluster's topology
  - Define core data structures (Node, Cluster, Topology)
  - MySQL client interface (read operations only)
  - Version detection
  - Topology discovery from seed nodes
  - Simple CLI: sentinel discover --cluster=prod-mysql-1
  - Output: Pretty-print the topology tree

  Phase 2: Basic Failover (5-7 days)

  Goal: Manually trigger a failover (hard-coded steps)
  - Promotion candidate selection algorithm
  - MySQL client write operations (STOP REPLICA, CHANGE SOURCE, etc.)
  - Hard-coded failover sequence (no step abstraction yet)
  - CLI: sentinel failover --cluster=prod-mysql-1 [--candidate=node-2]
  - Test: Actually fail over a test cluster!

  Phase 3: Step Architecture (3-4 days)

  Goal: Make operations extensible
  - Refactor Phase 2 failover into discrete steps
  - Step interface, OperationContext, OperationExecutor
  - Built-in steps: acquire-lock, select-candidate, promote-primary, repoint-replicas
  - Custom shell script step
  - Test: Same failover, but now with configurable steps

  Phase 4: Monitoring & Auto-Failover (4-6 days)

  Goal: Automatic response to failures
  - Health checking
  - Monitoring loop
  - Leader election (in-memory first)
  - Auto-trigger failover when primary fails
  - Run in background: sentinel watch --cluster=prod-mysql-1
  - Test: Kill primary, watch auto-failover

  Phase 5: REST API (4-5 days)

  Goal: Programmatic control
  - API server setup (routing, middleware)
  - Core endpoints: topology, operations, trigger failover
  - Async operation tracking
  - Test: curl commands to view topology and trigger failover

  Phase 6: Basic UI (5-7 days)

  Goal: User-friendly interface
  - React setup with Vite
  - Topology visualization
  - Manual failover dialog
  - Embed in Go binary
  - Test: Open browser, view cluster, click failover button

  Phase 7+: Advanced Features (ongoing)

  - Intermediate failover strategies
  - Graceful takeover
  - Configuration system (hierarchy, pending/active)
  - Persistent state store (MySQL/PostgreSQL)
  - Distributed leader election (etcd/Consul)
  - Multi-cluster support
  - Audit logging
  - Authorization

  Key Principles

  1. Always have something runnable: Each phase produces a working artifact
  2. Test with real MySQL: Set up a test replication cluster early, use it throughout
  3. Defer complexity: Start with in-memory, single-cluster, file config - add complexity later
  4. Interfaces early, implementations incremental: Define interfaces in Phase 1-2, swap implementations later
  5. Document as you go: Each phase should update docs with what's implemented and what's stubbed
