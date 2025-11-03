// Package types provides core data structures for Sentinel.
package types

import "time"

// NodeRole represents the role of a MySQL node in replication.
type NodeRole int

const (
	RolePrimary      NodeRole = iota // Primary node (writable)
	RoleReplica                       // Read-only replica
	RoleIntermediate                  // Intermediate node with downstream replicas
)

// String returns the string representation of a NodeRole.
func (r NodeRole) String() string {
	switch r {
	case RolePrimary:
		return "primary"
	case RoleReplica:
		return "replica"
	case RoleIntermediate:
		return "intermediate"
	default:
		return "unknown"
	}
}

// Node represents a single MySQL instance in a replication topology.
type Node struct {
	ID          string    // Unique identifier (e.g., UUID)
	ClusterID   string    // Parent cluster ID
	Host        string    // Hostname or IP
	Port        int       // MySQL port
	Role        NodeRole  // Node role in replication

	// Replication relationships
	SourceNodeID    *string  // nil for primary, points to upstream node
	DownstreamNodes []string // IDs of nodes replicating from this node

	// Capabilities
	CanBePromoted  bool // Can this node be promoted to primary?
	IsIntermediate bool // Does this node have downstream replicas?

	// Connection credentials
	Username string // MySQL username
	Password string // MySQL password (encrypted in database)

	// Metadata from MySQL
	ServerID     uint32 // MySQL server_id
	MySQLVersion string // MySQL version string
	GTIDExecuted string // GTID executed set

	// Timestamps
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Cluster represents a MySQL replication topology.
type Cluster struct {
	ID          string
	GroupID     string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Group represents a logical grouping of clusters.
type Group struct {
	ID          string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TopologyNode represents a node in the topology tree structure.
type TopologyNode struct {
	Node     *Node
	Parent   *TopologyNode
	Children []*TopologyNode
	Depth    int // Distance from primary (0 = primary, 1 = direct replica, etc.)
}

// ClusterTopology represents a complete topology snapshot.
type ClusterTopology struct {
	ClusterID    string
	Primary      *Node          // Primary node (nil if no primary found)
	Nodes        []*Node        // All nodes in the cluster
	Tree         *TopologyNode  // Tree structure starting at primary
	DiscoveredAt time.Time      // When this topology was discovered
}

// NodeHealth represents health metrics for a node.
type NodeHealth struct {
	NodeID           string
	Reachable        bool
	LastCheckTime    time.Time
	ReplicationLag   *int64  // Seconds behind source, nil if not replicating
	IOThreadRunning  bool    // I/O thread status
	SQLThreadRunning bool    // SQL thread status
	LastIOError      string  // Last I/O error message
	LastSQLError     string  // Last SQL error message
	ReadOnly         bool    // read_only system variable
	SourceHost       string  // Source host this node replicates from
	SourcePort       int     // Source port
}
