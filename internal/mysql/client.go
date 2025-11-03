// Package mysql provides MySQL client operations for Sentinel.
package mysql

import (
	"context"
)

// Client provides MySQL operations for topology discovery and management.
// Phase 1 focuses on read-only operations for topology discovery.
type Client interface {
	// Connection management
	Connect(ctx context.Context) error
	Close() error
	Ping(ctx context.Context) error

	// Version detection
	GetVersion(ctx context.Context) (string, error)

	// Replication status (read operations only for Phase 1)
	GetReplicationStatus(ctx context.Context) (*ReplicationStatus, error)
	GetReplicas(ctx context.Context) ([]*ReplicaInfo, error)

	// Health checks
	IsReadOnly(ctx context.Context) (bool, error)
	GetServerID(ctx context.Context) (uint32, error)
	GetGTIDExecuted(ctx context.Context) (string, error)
}

// ReplicationStatus represents the output of SHOW REPLICA STATUS / SHOW SLAVE STATUS.
// This contains replication state when this MySQL instance is a replica.
type ReplicationStatus struct {
	SourceHost          string  // Source_Host / Master_Host
	SourcePort          int     // Source_Port / Master_Port
	IOThreadRunning     bool    // Replica_IO_Running / Slave_IO_Running
	SQLThreadRunning    bool    // Replica_SQL_Running / Slave_SQL_Running
	SecondsBehindSource *int64  // Seconds_Behind_Source / Seconds_Behind_Master (nil if not applicable)
	LastIOError         string  // Last_IO_Error
	LastSQLError        string  // Last_SQL_Error
	ExecutedGTIDSet     string  // Executed_Gtid_Set
	RetrievedGTIDSet    string  // Retrieved_Gtid_Set
}

// ReplicaInfo represents a replica from SHOW REPLICAS / SHOW SLAVE HOSTS.
// This contains information about replicas connected to this MySQL instance.
type ReplicaInfo struct {
	ServerID uint32 // Server_id
	Host     string // Host
	Port     int    // Port
	UUID     string // Slave_UUID (if available)
}
