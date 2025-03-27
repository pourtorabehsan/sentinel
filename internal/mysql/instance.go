package mysql

import (
	"fmt"
	"net"
	"time"
)

// Role represents the role of a MySQL instance
type Role string

const (
	RoleMaster  Role = "master"
	RoleSlave   Role = "slave"
	RoleUnknown Role = "unknown"
)

// Instance represents a MySQL database instance
type Instance struct {
	// Basic instance information
	HostName string
	Port     int
	Role     Role

	// Replication status
	ReadOnly        bool
	SlaveIORunning  bool
	SlaveSQLRunning bool
	LastIOError     string
	LastSQLError    string

	// Master information
	MasterHost string
	MasterPort int

	// Replication position
	BinlogFile     string
	BinlogPosition uint32
	GTID           string

	// Instance status
	LastSeen        time.Time
	SecondsBehind   int64
	ReplicationLag  time.Duration
	IsReachable    bool
	LastError      error
	Version        string
}

// DSN returns the MySQL connection string for this instance
func (i *Instance) DSN(user, password string) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/?timeout=5s&readTimeout=5s&writeTimeout=5s",
		user, password, i.HostName, i.Port)
}

// Key returns a unique identifier for this instance
func (i *Instance) Key() string {
	return net.JoinHostPort(i.HostName, fmt.Sprintf("%d", i.Port))
}

// IsMaster returns true if this instance is a master
func (i *Instance) IsMaster() bool {
	return i.Role == RoleMaster
}

// IsValid returns true if the instance is in a valid state
func (i *Instance) IsValid() bool {
	return i.IsReachable && i.LastError == nil
}

// HasValidSlaveStatus returns true if the slave is running correctly
func (i *Instance) HasValidSlaveStatus() bool {
	if i.IsMaster() {
		return true
	}
	return i.SlaveIORunning && i.SlaveSQLRunning
}
