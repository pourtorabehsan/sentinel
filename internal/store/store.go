// Package store provides state persistence for Sentinel.
package store

import (
	"context"
	"errors"

	"github.com/pourtorabehsan/sentinel/internal/types"
)

// ErrNotFound is returned when a requested resource is not found.
var ErrNotFound = errors.New("not found")

// Store defines the interface for state persistence.
// Implementations must be safe for concurrent use.
type Store interface {
	// Groups
	CreateGroup(ctx context.Context, group *types.Group) error
	GetGroup(ctx context.Context, id string) (*types.Group, error)
	ListGroups(ctx context.Context) ([]*types.Group, error)
	DeleteGroup(ctx context.Context, id string) error

	// Clusters
	CreateCluster(ctx context.Context, cluster *types.Cluster) error
	GetCluster(ctx context.Context, id string) (*types.Cluster, error)
	ListClusters(ctx context.Context, groupID string) ([]*types.Cluster, error)
	ListAllClusters(ctx context.Context) ([]*types.Cluster, error)
	DeleteCluster(ctx context.Context, id string) error

	// Nodes
	CreateNode(ctx context.Context, node *types.Node) error
	GetNode(ctx context.Context, id string) (*types.Node, error)
	GetNodeByHostPort(ctx context.Context, clusterID, host string, port int) (*types.Node, error)
	UpdateNode(ctx context.Context, node *types.Node) error
	ListNodes(ctx context.Context, clusterID string) ([]*types.Node, error)
	DeleteNode(ctx context.Context, id string) error

	// Node downstream relationships
	AddDownstreamNode(ctx context.Context, nodeID, downstreamNodeID string) error
	RemoveDownstreamNode(ctx context.Context, nodeID, downstreamNodeID string) error
	GetDownstreamNodes(ctx context.Context, nodeID string) ([]string, error)

	// Health
	UpdateNodeHealth(ctx context.Context, health *types.NodeHealth) error
	GetNodeHealth(ctx context.Context, nodeID string) (*types.NodeHealth, error)

	// Topology (convenience method to get full cluster topology)
	GetClusterTopology(ctx context.Context, clusterID string) (*types.ClusterTopology, error)

	// Lifecycle
	Close() error
}
