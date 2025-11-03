package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // SQLite driver

	"github.com/pourtorabehsan/sentinel/internal/types"
)

// SQLiteStore implements Store using SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite store.
// The dsn can be ":memory:" for in-memory database or "file:path" for file-based.
func NewSQLiteStore(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(1) // SQLite works best with single connection
	db.SetMaxIdleConns(1)

	// Initialize schema
	if err := initSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// initSchema creates tables if they don't exist.
func initSchema(db *sql.DB) error {
	_, err := db.Exec(sqliteSchema)
	return err
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// Groups

// CreateGroup creates a new group.
func (s *SQLiteStore) CreateGroup(ctx context.Context, group *types.Group) error {
	if group.CreatedAt.IsZero() {
		group.CreatedAt = time.Now()
	}
	if group.UpdatedAt.IsZero() {
		group.UpdatedAt = time.Now()
	}

	query := `INSERT INTO groups (id, name, description, created_at, updated_at)
	          VALUES (?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, query, group.ID, group.Name, group.Description,
		group.CreatedAt, group.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create group: %w", err)
	}
	return nil
}

// GetGroup retrieves a group by ID.
func (s *SQLiteStore) GetGroup(ctx context.Context, id string) (*types.Group, error) {
	query := `SELECT id, name, description, created_at, updated_at FROM groups WHERE id = ?`

	var group types.Group
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&group.ID, &group.Name, &group.Description, &group.CreatedAt, &group.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: group %s", ErrNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get group: %w", err)
	}
	return &group, nil
}

// ListGroups returns all groups.
func (s *SQLiteStore) ListGroups(ctx context.Context) ([]*types.Group, error) {
	query := `SELECT id, name, description, created_at, updated_at FROM groups ORDER BY name`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}
	defer rows.Close()

	var groups []*types.Group
	for rows.Next() {
		var group types.Group
		if err := rows.Scan(&group.ID, &group.Name, &group.Description,
			&group.CreatedAt, &group.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan group: %w", err)
		}
		groups = append(groups, &group)
	}
	return groups, rows.Err()
}

// DeleteGroup deletes a group and all its clusters/nodes.
func (s *SQLiteStore) DeleteGroup(ctx context.Context, id string) error {
	query := `DELETE FROM groups WHERE id = ?`
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("group not found: %s", id)
	}
	return nil
}

// Clusters

// CreateCluster creates a new cluster.
func (s *SQLiteStore) CreateCluster(ctx context.Context, cluster *types.Cluster) error {
	if cluster.CreatedAt.IsZero() {
		cluster.CreatedAt = time.Now()
	}
	if cluster.UpdatedAt.IsZero() {
		cluster.UpdatedAt = time.Now()
	}

	query := `INSERT INTO clusters (id, group_id, name, description, created_at, updated_at)
	          VALUES (?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, query, cluster.ID, cluster.GroupID, cluster.Name,
		cluster.Description, cluster.CreatedAt, cluster.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create cluster: %w", err)
	}
	return nil
}

// GetCluster retrieves a cluster by ID.
func (s *SQLiteStore) GetCluster(ctx context.Context, id string) (*types.Cluster, error) {
	query := `SELECT id, group_id, name, description, created_at, updated_at
	          FROM clusters WHERE id = ?`

	var cluster types.Cluster
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&cluster.ID, &cluster.GroupID, &cluster.Name, &cluster.Description,
		&cluster.CreatedAt, &cluster.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: cluster %s", ErrNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}
	return &cluster, nil
}

// ListClusters returns all clusters in a group.
func (s *SQLiteStore) ListClusters(ctx context.Context, groupID string) ([]*types.Cluster, error) {
	query := `SELECT id, group_id, name, description, created_at, updated_at
	          FROM clusters WHERE group_id = ? ORDER BY name`

	rows, err := s.db.QueryContext(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}
	defer rows.Close()

	var clusters []*types.Cluster
	for rows.Next() {
		var cluster types.Cluster
		if err := rows.Scan(&cluster.ID, &cluster.GroupID, &cluster.Name,
			&cluster.Description, &cluster.CreatedAt, &cluster.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan cluster: %w", err)
		}
		clusters = append(clusters, &cluster)
	}
	return clusters, rows.Err()
}

// ListAllClusters returns all clusters across all groups.
func (s *SQLiteStore) ListAllClusters(ctx context.Context) ([]*types.Cluster, error) {
	query := `SELECT id, group_id, name, description, created_at, updated_at
	          FROM clusters ORDER BY group_id, name`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list all clusters: %w", err)
	}
	defer rows.Close()

	var clusters []*types.Cluster
	for rows.Next() {
		var cluster types.Cluster
		if err := rows.Scan(&cluster.ID, &cluster.GroupID, &cluster.Name,
			&cluster.Description, &cluster.CreatedAt, &cluster.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan cluster: %w", err)
		}
		clusters = append(clusters, &cluster)
	}
	return clusters, rows.Err()
}

// DeleteCluster deletes a cluster and all its nodes.
func (s *SQLiteStore) DeleteCluster(ctx context.Context, id string) error {
	query := `DELETE FROM clusters WHERE id = ?`
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete cluster: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("cluster not found: %s", id)
	}
	return nil
}

// Nodes

// CreateNode creates a new node.
func (s *SQLiteStore) CreateNode(ctx context.Context, node *types.Node) error {
	if node.CreatedAt.IsZero() {
		node.CreatedAt = time.Now()
	}
	if node.UpdatedAt.IsZero() {
		node.UpdatedAt = time.Now()
	}

	query := `INSERT INTO nodes
	          (id, cluster_id, host, port, role, source_node_id, can_be_promoted,
	           is_intermediate, username, password, server_id, mysql_version,
	           gtid_executed, created_at, updated_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, query,
		node.ID, node.ClusterID, node.Host, node.Port, int(node.Role),
		node.SourceNodeID, node.CanBePromoted, node.IsIntermediate,
		node.Username, node.Password, node.ServerID, node.MySQLVersion,
		node.GTIDExecuted, node.CreatedAt, node.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create node: %w", err)
	}
	return nil
}

// GetNode retrieves a node by ID.
func (s *SQLiteStore) GetNode(ctx context.Context, id string) (*types.Node, error) {
	query := `SELECT id, cluster_id, host, port, role, source_node_id, can_be_promoted,
	          is_intermediate, username, password, server_id, mysql_version,
	          gtid_executed, created_at, updated_at
	          FROM nodes WHERE id = ?`

	var node types.Node
	var role int
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&node.ID, &node.ClusterID, &node.Host, &node.Port, &role,
		&node.SourceNodeID, &node.CanBePromoted, &node.IsIntermediate,
		&node.Username, &node.Password, &node.ServerID, &node.MySQLVersion,
		&node.GTIDExecuted, &node.CreatedAt, &node.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: node %s", ErrNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	node.Role = types.NodeRole(role)

	// Load downstream nodes
	downstreamNodes, err := s.GetDownstreamNodes(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get downstream nodes: %w", err)
	}
	node.DownstreamNodes = downstreamNodes

	return &node, nil
}

// GetNodeByHostPort retrieves a node by its natural key (cluster_id, host, port).
func (s *SQLiteStore) GetNodeByHostPort(ctx context.Context, clusterID, host string, port int) (*types.Node, error) {
	query := `SELECT id, cluster_id, host, port, role, source_node_id, can_be_promoted,
	          is_intermediate, username, password, server_id, mysql_version,
	          gtid_executed, created_at, updated_at
	          FROM nodes WHERE cluster_id = ? AND host = ? AND port = ?`

	var node types.Node
	var role int
	err := s.db.QueryRowContext(ctx, query, clusterID, host, port).Scan(
		&node.ID, &node.ClusterID, &node.Host, &node.Port, &role,
		&node.SourceNodeID, &node.CanBePromoted, &node.IsIntermediate,
		&node.Username, &node.Password, &node.ServerID, &node.MySQLVersion,
		&node.GTIDExecuted, &node.CreatedAt, &node.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: node %s:%s:%d", ErrNotFound, clusterID, host, port)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get node by host:port: %w", err)
	}
	node.Role = types.NodeRole(role)

	// Load downstream nodes
	downstreamNodes, err := s.GetDownstreamNodes(ctx, node.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get downstream nodes: %w", err)
	}
	node.DownstreamNodes = downstreamNodes

	return &node, nil
}

// UpdateNode updates an existing node.
func (s *SQLiteStore) UpdateNode(ctx context.Context, node *types.Node) error {
	node.UpdatedAt = time.Now()

	query := `UPDATE nodes SET
	          cluster_id = ?, host = ?, port = ?, role = ?, source_node_id = ?,
	          can_be_promoted = ?, is_intermediate = ?, username = ?, password = ?,
	          server_id = ?, mysql_version = ?, gtid_executed = ?, updated_at = ?
	          WHERE id = ?`

	result, err := s.db.ExecContext(ctx, query,
		node.ClusterID, node.Host, node.Port, int(node.Role), node.SourceNodeID,
		node.CanBePromoted, node.IsIntermediate, node.Username, node.Password,
		node.ServerID, node.MySQLVersion, node.GTIDExecuted, node.UpdatedAt,
		node.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update node: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("node not found: %s", node.ID)
	}
	return nil
}

// ListNodes returns all nodes in a cluster.
func (s *SQLiteStore) ListNodes(ctx context.Context, clusterID string) ([]*types.Node, error) {
	query := `SELECT id, cluster_id, host, port, role, source_node_id, can_be_promoted,
	          is_intermediate, username, password, server_id, mysql_version,
	          gtid_executed, created_at, updated_at
	          FROM nodes WHERE cluster_id = ? ORDER BY role, host`

	rows, err := s.db.QueryContext(ctx, query, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	defer rows.Close()

	var nodes []*types.Node
	for rows.Next() {
		var node types.Node
		var role int
		if err := rows.Scan(&node.ID, &node.ClusterID, &node.Host, &node.Port, &role,
			&node.SourceNodeID, &node.CanBePromoted, &node.IsIntermediate,
			&node.Username, &node.Password, &node.ServerID, &node.MySQLVersion,
			&node.GTIDExecuted, &node.CreatedAt, &node.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}
		node.Role = types.NodeRole(role)
		nodes = append(nodes, &node)
	}

	// Load downstream nodes for all nodes in a second pass
	// This prevents issues with concurrent queries on the same connection
	for _, node := range nodes {
		downstreamNodes, err := s.GetDownstreamNodes(ctx, node.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get downstream nodes: %w", err)
		}
		node.DownstreamNodes = downstreamNodes
	}

	return nodes, rows.Err()
}

// DeleteNode deletes a node.
func (s *SQLiteStore) DeleteNode(ctx context.Context, id string) error {
	query := `DELETE FROM nodes WHERE id = ?`
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("node not found: %s", id)
	}
	return nil
}

// Node Downstream Relationships

// AddDownstreamNode adds a downstream relationship.
func (s *SQLiteStore) AddDownstreamNode(ctx context.Context, nodeID, downstreamNodeID string) error {
	query := `INSERT OR IGNORE INTO node_downstream (node_id, downstream_node_id) VALUES (?, ?)`
	_, err := s.db.ExecContext(ctx, query, nodeID, downstreamNodeID)
	if err != nil {
		return fmt.Errorf("failed to add downstream node: %w", err)
	}
	return nil
}

// RemoveDownstreamNode removes a downstream relationship.
func (s *SQLiteStore) RemoveDownstreamNode(ctx context.Context, nodeID, downstreamNodeID string) error {
	query := `DELETE FROM node_downstream WHERE node_id = ? AND downstream_node_id = ?`
	_, err := s.db.ExecContext(ctx, query, nodeID, downstreamNodeID)
	if err != nil {
		return fmt.Errorf("failed to remove downstream node: %w", err)
	}
	return nil
}

// GetDownstreamNodes returns all downstream node IDs for a given node.
func (s *SQLiteStore) GetDownstreamNodes(ctx context.Context, nodeID string) ([]string, error) {
	query := `SELECT downstream_node_id FROM node_downstream WHERE node_id = ?`

	rows, err := s.db.QueryContext(ctx, query, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get downstream nodes: %w", err)
	}
	defer rows.Close()

	var nodeIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan downstream node: %w", err)
		}
		nodeIDs = append(nodeIDs, id)
	}
	return nodeIDs, rows.Err()
}

// Health

// UpdateNodeHealth updates or inserts health information for a node.
func (s *SQLiteStore) UpdateNodeHealth(ctx context.Context, health *types.NodeHealth) error {
	query := `INSERT OR REPLACE INTO node_health
	          (node_id, reachable, last_check_time, replication_lag, io_thread_running,
	           sql_thread_running, last_io_error, last_sql_error, read_only,
	           source_host, source_port)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, query,
		health.NodeID, health.Reachable, health.LastCheckTime, health.ReplicationLag,
		health.IOThreadRunning, health.SQLThreadRunning, health.LastIOError,
		health.LastSQLError, health.ReadOnly, health.SourceHost, health.SourcePort,
	)
	if err != nil {
		return fmt.Errorf("failed to update node health: %w", err)
	}
	return nil
}

// GetNodeHealth retrieves health information for a node.
func (s *SQLiteStore) GetNodeHealth(ctx context.Context, nodeID string) (*types.NodeHealth, error) {
	query := `SELECT node_id, reachable, last_check_time, replication_lag, io_thread_running,
	          sql_thread_running, last_io_error, last_sql_error, read_only,
	          source_host, source_port
	          FROM node_health WHERE node_id = ?`

	var health types.NodeHealth
	err := s.db.QueryRowContext(ctx, query, nodeID).Scan(
		&health.NodeID, &health.Reachable, &health.LastCheckTime, &health.ReplicationLag,
		&health.IOThreadRunning, &health.SQLThreadRunning, &health.LastIOError,
		&health.LastSQLError, &health.ReadOnly, &health.SourceHost, &health.SourcePort,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: node health for %s", ErrNotFound, nodeID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get node health: %w", err)
	}
	return &health, nil
}

// GetClusterTopology builds a complete topology for a cluster.
func (s *SQLiteStore) GetClusterTopology(ctx context.Context, clusterID string) (*types.ClusterTopology, error) {
	// Get all nodes in the cluster
	nodes, err := s.ListNodes(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	if len(nodes) == 0 {
		return &types.ClusterTopology{
			ClusterID:    clusterID,
			Nodes:        nodes,
			DiscoveredAt: time.Now(),
		}, nil
	}

	// Find primary (node with no source)
	var primary *types.Node
	nodeMap := make(map[string]*types.Node)
	for _, node := range nodes {
		nodeMap[node.ID] = node
		if node.SourceNodeID == nil {
			primary = node
		}
	}

	// Build tree structure
	var tree *types.TopologyNode
	if primary != nil {
		tree = buildTopologyTree(primary, nodeMap, 0)
	}

	return &types.ClusterTopology{
		ClusterID:    clusterID,
		Primary:      primary,
		Nodes:        nodes,
		Tree:         tree,
		DiscoveredAt: time.Now(),
	}, nil
}

// buildTopologyTree recursively builds the topology tree.
func buildTopologyTree(node *types.Node, nodeMap map[string]*types.Node, depth int) *types.TopologyNode {
	topologyNode := &types.TopologyNode{
		Node:     node,
		Depth:    depth,
		Children: make([]*types.TopologyNode, 0),
	}

	// Find and add children
	for _, downstreamID := range node.DownstreamNodes {
		if childNode, ok := nodeMap[downstreamID]; ok {
			childTopologyNode := buildTopologyTree(childNode, nodeMap, depth+1)
			childTopologyNode.Parent = topologyNode
			topologyNode.Children = append(topologyNode.Children, childTopologyNode)
		}
	}

	return topologyNode
}
