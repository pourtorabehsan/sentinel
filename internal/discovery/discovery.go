// Package discovery provides topology discovery for MySQL replication clusters.
package discovery

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/pourtorabehsan/sentinel/internal/mysql"
	"github.com/pourtorabehsan/sentinel/internal/store"
	"github.com/pourtorabehsan/sentinel/internal/types"
)

// Manager handles topology discovery.
type Manager struct {
	store  store.Store
	logger *slog.Logger
}

// NewManager creates a new discovery manager.
func NewManager(store store.Store, logger *slog.Logger) *Manager {
	return &Manager{
		store:  store,
		logger: logger,
	}
}

// DiscoverFromSeed discovers the complete topology starting from a seed node.
// It crawls the replication topology by connecting to the seed, finding its
// source and replicas, and recursively discovering all nodes.
func (m *Manager) DiscoverFromSeed(
	ctx context.Context,
	clusterID string,
	seedHost string,
	seedPort int,
	username, password string,
) (*types.ClusterTopology, error) {
	m.logger.Info("starting topology discovery",
		"cluster_id", clusterID,
		"seed_host", seedHost,
		"seed_port", seedPort,
	)

	// Track discovered nodes to avoid cycles
	discovered := make(map[string]*types.Node)

	// Track node relationships for building the tree
	relationships := make(map[string][]string) // nodeID -> downstream node IDs

	// Start discovery from seed
	if err := m.discoverNode(ctx, seedHost, seedPort, username, password,
		clusterID, discovered, relationships); err != nil {
		return nil, fmt.Errorf("failed to discover from seed: %w", err)
	}

	// Convert map to slice
	var nodes []*types.Node
	for _, node := range discovered {
		nodes = append(nodes, node)
	}

	// Find primary (node with no source)
	var primary *types.Node
	for _, node := range nodes {
		if node.SourceNodeID == nil {
			primary = node
			break
		}
	}

	// Build topology tree
	var tree *types.TopologyNode
	if primary != nil {
		tree = m.buildTopologyTree(primary, discovered, 0)
	}

	topology := &types.ClusterTopology{
		ClusterID:    clusterID,
		Primary:      primary,
		Nodes:        nodes,
		Tree:         tree,
		DiscoveredAt: time.Now(),
	}

	m.logger.Info("topology discovery completed",
		"cluster_id", clusterID,
		"nodes_count", len(nodes),
		"primary_found", primary != nil,
	)

	return topology, nil
}

// discoverNode discovers a single node and recursively discovers its upstream and downstream nodes.
func (m *Manager) discoverNode(
	ctx context.Context,
	host string,
	port int,
	username, password string,
	clusterID string,
	discovered map[string]*types.Node,
	relationships map[string][]string,
) error {
	// Check if already discovered
	nodeKey := fmt.Sprintf("%s:%d", host, port)
	if _, exists := discovered[nodeKey]; exists {
		m.logger.Debug("node already discovered, skipping", "node", nodeKey)
		return nil
	}

	m.logger.Debug("discovering node", "host", host, "port", port)

	// Connect to node
	client, err := mysql.NewClient(host, port, username, password)
	if err != nil {
		return fmt.Errorf("failed to create client for %s:%d: %w", host, port, err)
	}

	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to %s:%d: %w", host, port, err)
	}
	defer client.Close()

	// Get node information
	version, err := client.GetVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get version: %w", err)
	}

	serverID, err := client.GetServerID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get server_id: %w", err)
	}

	gtidExecuted, err := client.GetGTIDExecuted(ctx)
	if err != nil {
		m.logger.Warn("failed to get gtid_executed", "error", err)
		gtidExecuted = ""
	}

	readOnly, err := client.IsReadOnly(ctx)
	if err != nil {
		m.logger.Warn("failed to get read_only", "error", err)
	}

	// Create node
	node := &types.Node{
		ID:            uuid.New().String(),
		ClusterID:     clusterID,
		Host:          host,
		Port:          port,
		Role:          types.RoleReplica, // Default, will be updated
		SourceNodeID:  nil,
		CanBePromoted: true,
		IsIntermediate: false,
		Username:      username,
		Password:      password,
		ServerID:      serverID,
		MySQLVersion:  version,
		GTIDExecuted:  gtidExecuted,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Get replication status to find upstream
	replStatus, err := client.GetReplicationStatus(ctx)
	if err != nil {
		m.logger.Warn("failed to get replication status", "error", err)
	}

	// Get replicas to find downstream
	replicas, err := client.GetReplicas(ctx)
	if err != nil {
		m.logger.Warn("failed to get replicas", "error", err)
		replicas = nil
	}

	// Determine role
	if replStatus == nil {
		// No replication status = this is the primary
		node.Role = types.RolePrimary
	} else {
		// Has upstream source
		if len(replicas) > 0 {
			node.Role = types.RoleIntermediate
			node.IsIntermediate = true
		} else {
			node.Role = types.RoleReplica
		}
	}

	// Mark as not promotable if read_only is off and it's a replica
	// (Primary should be writable)
	if node.Role != types.RolePrimary && !readOnly {
		node.CanBePromoted = false
	}

	// Add to discovered nodes
	discovered[nodeKey] = node
	m.logger.Info("discovered node",
		"node_id", node.ID,
		"host", host,
		"port", port,
		"role", node.Role.String(),
		"server_id", serverID,
		"version", version,
	)

	// Discover upstream if this is a replica
	if replStatus != nil && replStatus.SourceHost != "" {
		sourceKey := fmt.Sprintf("%s:%d", replStatus.SourceHost, replStatus.SourcePort)

		// Discover upstream node
		if err := m.discoverNode(ctx, replStatus.SourceHost, replStatus.SourcePort,
			username, password, clusterID, discovered, relationships); err != nil {
			m.logger.Warn("failed to discover upstream node",
				"source_host", replStatus.SourceHost,
				"source_port", replStatus.SourcePort,
				"error", err,
			)
		} else {
			// Link to upstream
			if sourceNode, ok := discovered[sourceKey]; ok {
				node.SourceNodeID = &sourceNode.ID

				// Add to relationships for tree building
				relationships[sourceNode.ID] = append(relationships[sourceNode.ID], node.ID)
			}
		}
	}

	// Discover downstream replicas
	for _, replica := range replicas {
		replicaKey := fmt.Sprintf("%s:%d", replica.Host, replica.Port)

		// Skip if already discovered
		if _, exists := discovered[replicaKey]; exists {
			continue
		}

		if err := m.discoverNode(ctx, replica.Host, replica.Port,
			username, password, clusterID, discovered, relationships); err != nil {
			m.logger.Warn("failed to discover replica",
				"replica_host", replica.Host,
				"replica_port", replica.Port,
				"error", err,
			)
			continue
		}

		// Link downstream
		if replicaNode, ok := discovered[replicaKey]; ok {
			relationships[node.ID] = append(relationships[node.ID], replicaNode.ID)
		}
	}

	// Update downstream nodes in the node object
	if downstreamIDs, ok := relationships[node.ID]; ok {
		node.DownstreamNodes = downstreamIDs
	}

	return nil
}

// buildTopologyTree recursively builds the topology tree structure.
func (m *Manager) buildTopologyTree(
	node *types.Node,
	nodeMap map[string]*types.Node,
	depth int,
) *types.TopologyNode {
	topologyNode := &types.TopologyNode{
		Node:     node,
		Depth:    depth,
		Children: make([]*types.TopologyNode, 0),
	}

	// Find children by looking for nodes with this node as source
	for _, downstreamID := range node.DownstreamNodes {
		// Find the downstream node in the map
		for _, candidateNode := range nodeMap {
			if candidateNode.ID == downstreamID {
				childTopologyNode := m.buildTopologyTree(candidateNode, nodeMap, depth+1)
				childTopologyNode.Parent = topologyNode
				topologyNode.Children = append(topologyNode.Children, childTopologyNode)
				break
			}
		}
	}

	return topologyNode
}

// SaveTopology saves a discovered topology to the store.
func (m *Manager) SaveTopology(ctx context.Context, topology *types.ClusterTopology) error {
	m.logger.Info("saving topology to store", "cluster_id", topology.ClusterID, "nodes_count", len(topology.Nodes))

	// Save all nodes
	for _, node := range topology.Nodes {
		// Check if node already exists by its natural key (cluster_id, host, port)
		existingNode, err := m.store.GetNodeByHostPort(ctx, node.ClusterID, node.Host, node.Port)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("failed to check existing node: %w", err)
		}

		if existingNode != nil {
			// Node exists - update it with the new data but keep the existing ID
			node.ID = existingNode.ID
			node.CreatedAt = existingNode.CreatedAt
			if err := m.store.UpdateNode(ctx, node); err != nil {
				return fmt.Errorf("failed to update node %s: %w", node.ID, err)
			}
		} else {
			// Create new node
			if err := m.store.CreateNode(ctx, node); err != nil {
				return fmt.Errorf("failed to create node %s: %w", node.ID, err)
			}
		}

		// Save downstream relationships
		for _, downstreamID := range node.DownstreamNodes {
			if err := m.store.AddDownstreamNode(ctx, node.ID, downstreamID); err != nil {
				m.logger.Warn("failed to add downstream relationship",
					"node_id", node.ID,
					"downstream_id", downstreamID,
					"error", err,
				)
			}
		}
	}

	m.logger.Info("topology saved successfully", "cluster_id", topology.ClusterID)
	return nil
}
