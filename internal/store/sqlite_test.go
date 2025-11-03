package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pourtorabehsan/sentinel/internal/types"
)

func TestSQLiteStore_Groups(t *testing.T) {
	// Create in-memory store
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Test CreateGroup
	group := &types.Group{
		ID:          "test-group",
		Name:        "Test Group",
		Description: "A test group",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := store.CreateGroup(ctx, group); err != nil {
		t.Fatalf("failed to create group: %v", err)
	}

	// Test GetGroup
	retrieved, err := store.GetGroup(ctx, "test-group")
	if err != nil {
		t.Fatalf("failed to get group: %v", err)
	}
	if retrieved.ID != group.ID {
		t.Errorf("expected group ID %s, got %s", group.ID, retrieved.ID)
	}
	if retrieved.Name != group.Name {
		t.Errorf("expected group name %s, got %s", group.Name, retrieved.Name)
	}

	// Test ListGroups
	groups, err := store.ListGroups(ctx)
	if err != nil {
		t.Fatalf("failed to list groups: %v", err)
	}
	if len(groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(groups))
	}

	// Test DeleteGroup
	if err := store.DeleteGroup(ctx, "test-group"); err != nil {
		t.Fatalf("failed to delete group: %v", err)
	}

	// Verify deletion
	_, err = store.GetGroup(ctx, "test-group")
	if err == nil {
		t.Error("expected error when getting deleted group")
	}
}

func TestSQLiteStore_Clusters(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create a group first
	group := &types.Group{
		ID:   "test-group",
		Name: "Test Group",
	}
	if err := store.CreateGroup(ctx, group); err != nil {
		t.Fatalf("failed to create group: %v", err)
	}

	// Test CreateCluster
	cluster := &types.Cluster{
		ID:          "test-cluster",
		GroupID:     "test-group",
		Name:        "Test Cluster",
		Description: "A test cluster",
	}

	if err := store.CreateCluster(ctx, cluster); err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}

	// Test GetCluster
	retrieved, err := store.GetCluster(ctx, "test-cluster")
	if err != nil {
		t.Fatalf("failed to get cluster: %v", err)
	}
	if retrieved.ID != cluster.ID {
		t.Errorf("expected cluster ID %s, got %s", cluster.ID, retrieved.ID)
	}

	// Test ListClusters
	clusters, err := store.ListClusters(ctx, "test-group")
	if err != nil {
		t.Fatalf("failed to list clusters: %v", err)
	}
	if len(clusters) != 1 {
		t.Errorf("expected 1 cluster, got %d", len(clusters))
	}

	// Test ListAllClusters
	allClusters, err := store.ListAllClusters(ctx)
	if err != nil {
		t.Fatalf("failed to list all clusters: %v", err)
	}
	if len(allClusters) != 1 {
		t.Errorf("expected 1 cluster, got %d", len(allClusters))
	}
}

func TestSQLiteStore_Nodes(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create group and cluster
	group := &types.Group{ID: "test-group", Name: "Test Group"}
	if err := store.CreateGroup(ctx, group); err != nil {
		t.Fatalf("failed to create group: %v", err)
	}

	cluster := &types.Cluster{ID: "test-cluster", GroupID: "test-group", Name: "Test Cluster"}
	if err := store.CreateCluster(ctx, cluster); err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}

	// Test CreateNode
	node := &types.Node{
		ID:             "node-1",
		ClusterID:      "test-cluster",
		Host:           "localhost",
		Port:           3306,
		Role:           types.RolePrimary,
		SourceNodeID:   nil,
		CanBePromoted:  true,
		IsIntermediate: false,
		Username:       "root",
		Password:       "secret",
		ServerID:       1,
		MySQLVersion:   "8.0.32",
		GTIDExecuted:   "uuid:1-100",
	}

	if err := store.CreateNode(ctx, node); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	// Test GetNode
	retrieved, err := store.GetNode(ctx, "node-1")
	if err != nil {
		t.Fatalf("failed to get node: %v", err)
	}
	if retrieved.ID != node.ID {
		t.Errorf("expected node ID %s, got %s", node.ID, retrieved.ID)
	}
	if retrieved.Role != types.RolePrimary {
		t.Errorf("expected role primary, got %s", retrieved.Role)
	}

	// Test UpdateNode
	retrieved.Role = types.RoleReplica
	retrieved.CanBePromoted = false
	if err := store.UpdateNode(ctx, retrieved); err != nil {
		t.Fatalf("failed to update node: %v", err)
	}

	updated, err := store.GetNode(ctx, "node-1")
	if err != nil {
		t.Fatalf("failed to get updated node: %v", err)
	}
	if updated.Role != types.RoleReplica {
		t.Errorf("expected role replica, got %s", updated.Role)
	}

	// Test ListNodes
	nodes, err := store.ListNodes(ctx, "test-cluster")
	if err != nil {
		t.Fatalf("failed to list nodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(nodes))
	}
}

func TestSQLiteStore_DownstreamNodes(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Setup group, cluster, and nodes
	group := &types.Group{ID: "test-group", Name: "Test Group"}
	store.CreateGroup(ctx, group)

	cluster := &types.Cluster{ID: "test-cluster", GroupID: "test-group", Name: "Test Cluster"}
	store.CreateCluster(ctx, cluster)

	primary := &types.Node{
		ID:        "primary",
		ClusterID: "test-cluster",
		Host:      "host1",
		Port:      3306,
		Role:      types.RolePrimary,
		Username:  "root",
		Password:  "secret",
	}
	store.CreateNode(ctx, primary)

	replica := &types.Node{
		ID:        "replica",
		ClusterID: "test-cluster",
		Host:      "host2",
		Port:      3306,
		Role:      types.RoleReplica,
		Username:  "root",
		Password:  "secret",
	}
	store.CreateNode(ctx, replica)

	// Test AddDownstreamNode
	if err := store.AddDownstreamNode(ctx, "primary", "replica"); err != nil {
		t.Fatalf("failed to add downstream node: %v", err)
	}

	// Test GetDownstreamNodes
	downstream, err := store.GetDownstreamNodes(ctx, "primary")
	if err != nil {
		t.Fatalf("failed to get downstream nodes: %v", err)
	}
	if len(downstream) != 1 {
		t.Errorf("expected 1 downstream node, got %d", len(downstream))
	}
	if downstream[0] != "replica" {
		t.Errorf("expected downstream node 'replica', got %s", downstream[0])
	}

	// Test RemoveDownstreamNode
	if err := store.RemoveDownstreamNode(ctx, "primary", "replica"); err != nil {
		t.Fatalf("failed to remove downstream node: %v", err)
	}

	downstream, err = store.GetDownstreamNodes(ctx, "primary")
	if err != nil {
		t.Fatalf("failed to get downstream nodes after removal: %v", err)
	}
	if len(downstream) != 0 {
		t.Errorf("expected 0 downstream nodes after removal, got %d", len(downstream))
	}
}

func TestSQLiteStore_NodeHealth(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Setup
	group := &types.Group{ID: "test-group", Name: "Test Group"}
	store.CreateGroup(ctx, group)

	cluster := &types.Cluster{ID: "test-cluster", GroupID: "test-group", Name: "Test Cluster"}
	store.CreateCluster(ctx, cluster)

	node := &types.Node{
		ID:        "node-1",
		ClusterID: "test-cluster",
		Host:      "localhost",
		Port:      3306,
		Username:  "root",
		Password:  "secret",
	}
	store.CreateNode(ctx, node)

	// Test UpdateNodeHealth
	lag := int64(5)
	health := &types.NodeHealth{
		NodeID:           "node-1",
		Reachable:        true,
		LastCheckTime:    time.Now(),
		ReplicationLag:   &lag,
		IOThreadRunning:  true,
		SQLThreadRunning: true,
		ReadOnly:         true,
		SourceHost:       "master",
		SourcePort:       3306,
	}

	if err := store.UpdateNodeHealth(ctx, health); err != nil {
		t.Fatalf("failed to update node health: %v", err)
	}

	// Test GetNodeHealth
	retrieved, err := store.GetNodeHealth(ctx, "node-1")
	if err != nil {
		t.Fatalf("failed to get node health: %v", err)
	}
	if retrieved.NodeID != "node-1" {
		t.Errorf("expected node ID node-1, got %s", retrieved.NodeID)
	}
	if !retrieved.Reachable {
		t.Error("expected node to be reachable")
	}
	if retrieved.ReplicationLag == nil || *retrieved.ReplicationLag != 5 {
		t.Errorf("expected replication lag 5, got %v", retrieved.ReplicationLag)
	}
}

func TestSQLiteStore_GetClusterTopology(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Setup
	group := &types.Group{ID: "test-group", Name: "Test Group"}
	store.CreateGroup(ctx, group)

	cluster := &types.Cluster{ID: "test-cluster", GroupID: "test-group", Name: "Test Cluster"}
	store.CreateCluster(ctx, cluster)

	// Create primary
	primary := &types.Node{
		ID:        "primary",
		ClusterID: "test-cluster",
		Host:      "host1",
		Port:      3306,
		Role:      types.RolePrimary,
		Username:  "root",
		Password:  "secret",
	}
	store.CreateNode(ctx, primary)

	// Create replica
	replica := &types.Node{
		ID:           "replica",
		ClusterID:    "test-cluster",
		Host:         "host2",
		Port:         3306,
		Role:         types.RoleReplica,
		SourceNodeID: &primary.ID,
		Username:     "root",
		Password:     "secret",
	}
	store.CreateNode(ctx, replica)

	// Add downstream relationship
	store.AddDownstreamNode(ctx, primary.ID, replica.ID)

	// Test GetClusterTopology
	topology, err := store.GetClusterTopology(ctx, "test-cluster")
	if err != nil {
		t.Fatalf("failed to get cluster topology: %v", err)
	}

	if topology.ClusterID != "test-cluster" {
		t.Errorf("expected cluster ID test-cluster, got %s", topology.ClusterID)
	}
	if len(topology.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(topology.Nodes))
	}
	if topology.Primary == nil {
		t.Fatal("expected primary to be set")
	}
	if topology.Primary.ID != "primary" {
		t.Errorf("expected primary ID 'primary', got %s", topology.Primary.ID)
	}
	if topology.Tree == nil {
		t.Fatal("expected tree to be set")
	}
	if len(topology.Tree.Children) != 1 {
		t.Errorf("expected 1 child in tree, got %d", len(topology.Tree.Children))
	}
}

// TestSQLiteStore_NotFoundErrors tests that Get methods return ErrNotFound for non-existent resources.
func TestSQLiteStore_NotFoundErrors(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	t.Run("GetGroup_NotFound", func(t *testing.T) {
		_, err := store.GetGroup(ctx, "non-existent-group")
		if err == nil {
			t.Fatal("expected error when getting non-existent group")
		}
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got: %v", err)
		}
	})

	t.Run("GetCluster_NotFound", func(t *testing.T) {
		_, err := store.GetCluster(ctx, "non-existent-cluster")
		if err == nil {
			t.Fatal("expected error when getting non-existent cluster")
		}
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got: %v", err)
		}
	})

	t.Run("GetNode_NotFound", func(t *testing.T) {
		_, err := store.GetNode(ctx, "non-existent-node")
		if err == nil {
			t.Fatal("expected error when getting non-existent node")
		}
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got: %v", err)
		}
	})

	t.Run("GetNodeHealth_NotFound", func(t *testing.T) {
		_, err := store.GetNodeHealth(ctx, "non-existent-node")
		if err == nil {
			t.Fatal("expected error when getting non-existent node health")
		}
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got: %v", err)
		}
	})
}

// TestSQLiteStore_CreateOrUpdate tests the create-or-update pattern used in discovery.
func TestSQLiteStore_CreateOrUpdate(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Setup
	group := &types.Group{ID: "test-group", Name: "Test Group"}
	if err := store.CreateGroup(ctx, group); err != nil {
		t.Fatalf("failed to create group: %v", err)
	}

	cluster := &types.Cluster{ID: "test-cluster", GroupID: "test-group", Name: "Test Cluster"}
	if err := store.CreateCluster(ctx, cluster); err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}

	// Test the create-or-update pattern
	nodeID := "test-node-123"

	// First check - node doesn't exist
	existingNode, err := store.GetNode(ctx, nodeID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		t.Fatalf("unexpected error checking for node: %v", err)
	}

	if existingNode != nil {
		t.Fatal("expected node to not exist initially")
	}

	// Create the node
	node := &types.Node{
		ID:        nodeID,
		ClusterID: "test-cluster",
		Host:      "localhost",
		Port:      3306,
		Role:      types.RolePrimary,
		Username:  "root",
		Password:  "secret",
	}

	if err := store.CreateNode(ctx, node); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	// Second check - node exists, update it
	existingNode, err = store.GetNode(ctx, nodeID)
	if err != nil {
		t.Fatalf("failed to get node: %v", err)
	}

	if existingNode == nil {
		t.Fatal("expected node to exist after creation")
	}

	// Update the node
	existingNode.Role = types.RoleReplica
	existingNode.Port = 3307

	if err := store.UpdateNode(ctx, existingNode); err != nil {
		t.Fatalf("failed to update node: %v", err)
	}

	// Verify update
	updated, err := store.GetNode(ctx, nodeID)
	if err != nil {
		t.Fatalf("failed to get updated node: %v", err)
	}

	if updated.Role != types.RoleReplica {
		t.Errorf("expected role to be RoleReplica, got %s", updated.Role)
	}
	if updated.Port != 3307 {
		t.Errorf("expected port to be 3307, got %d", updated.Port)
	}
}
