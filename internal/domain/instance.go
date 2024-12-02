package domain

import (
	"time"
)

type InstanceRole string

const (
	RolePrimary InstanceRole = "primary"
	RoleReplica InstanceRole = "replica"
)

type Instance struct {
	ID        string       `json:"id"`
	Host      string       `json:"host"`
	Port      int         `json:"port"`
	Role      InstanceRole `json:"role"`
	ClusterID string      `json:"cluster_id"`
}

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type InstanceHealth struct {
	IsAlive       bool      `json:"is_alive"`
	LastChecked   time.Time `json:"last_checked"`
	ReplicationLag int64    `json:"replication_lag"`
	Error         string    `json:"error,omitempty"`
}

type Cluster struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}
