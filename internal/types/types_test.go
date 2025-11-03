package types

import (
	"testing"
)

func TestNodeRoleString(t *testing.T) {
	tests := []struct {
		role     NodeRole
		expected string
	}{
		{RolePrimary, "primary"},
		{RoleReplica, "replica"},
		{RoleIntermediate, "intermediate"},
		{NodeRole(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.role.String()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestNodeStructure(t *testing.T) {
	node := Node{
		ID:             "test-node-1",
		ClusterID:      "test-cluster",
		Host:           "localhost",
		Port:           3306,
		Role:           RolePrimary,
		SourceNodeID:   nil,
		CanBePromoted:  true,
		IsIntermediate: false,
		Username:       "root",
		Password:       "secret",
		ServerID:       1,
		MySQLVersion:   "8.0.32-MySQL",
		GTIDExecuted:   "uuid:1-100",
	}

	if node.ID != "test-node-1" {
		t.Errorf("unexpected node ID: %s", node.ID)
	}
	if node.Role != RolePrimary {
		t.Errorf("unexpected role: %s", node.Role)
	}
	if node.SourceNodeID != nil {
		t.Errorf("expected nil source node ID for primary")
	}
}

func TestClusterStructure(t *testing.T) {
	cluster := Cluster{
		ID:          "test-cluster",
		GroupID:     "test-group",
		Name:        "Test Cluster",
		Description: "A test cluster",
	}

	if cluster.ID != "test-cluster" {
		t.Errorf("unexpected cluster ID: %s", cluster.ID)
	}
	if cluster.GroupID != "test-group" {
		t.Errorf("unexpected group ID: %s", cluster.GroupID)
	}
}

func TestTopologyNode(t *testing.T) {
	primary := &Node{
		ID:   "primary",
		Role: RolePrimary,
	}

	replica1 := &Node{
		ID:           "replica1",
		Role:         RoleReplica,
		SourceNodeID: &primary.ID,
	}

	replica2 := &Node{
		ID:           "replica2",
		Role:         RoleReplica,
		SourceNodeID: &primary.ID,
	}

	// Build topology tree
	primaryTopology := &TopologyNode{
		Node:  primary,
		Depth: 0,
		Children: []*TopologyNode{
			{Node: replica1, Depth: 1},
			{Node: replica2, Depth: 1},
		},
	}

	if primaryTopology.Depth != 0 {
		t.Errorf("expected primary depth 0, got %d", primaryTopology.Depth)
	}
	if len(primaryTopology.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(primaryTopology.Children))
	}
	if primaryTopology.Children[0].Depth != 1 {
		t.Errorf("expected replica depth 1, got %d", primaryTopology.Children[0].Depth)
	}
}
