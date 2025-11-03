package store

// SQLite schema for Sentinel state storage.
// This schema supports storing groups, clusters, nodes, and health information.
const sqliteSchema = `
-- Groups table
CREATE TABLE IF NOT EXISTS groups (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Clusters table
CREATE TABLE IF NOT EXISTS clusters (
    id TEXT PRIMARY KEY,
    group_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE
);

-- Nodes table
CREATE TABLE IF NOT EXISTS nodes (
    id TEXT PRIMARY KEY,
    cluster_id TEXT NOT NULL,
    host TEXT NOT NULL,
    port INTEGER NOT NULL,
    role INTEGER NOT NULL,  -- 0=primary, 1=replica, 2=intermediate
    source_node_id TEXT,    -- NULL for primary
    can_be_promoted BOOLEAN NOT NULL DEFAULT 1,
    is_intermediate BOOLEAN NOT NULL DEFAULT 0,
    username TEXT NOT NULL,
    password TEXT NOT NULL,  -- TODO: Encrypt in production
    server_id INTEGER,
    mysql_version TEXT,
    gtid_executed TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (cluster_id) REFERENCES clusters(id) ON DELETE CASCADE,
    FOREIGN KEY (source_node_id) REFERENCES nodes(id) ON DELETE SET NULL,
    UNIQUE (cluster_id, host, port)
);

-- Node health table (latest health status per node)
CREATE TABLE IF NOT EXISTS node_health (
    node_id TEXT PRIMARY KEY,
    reachable BOOLEAN NOT NULL,
    last_check_time TIMESTAMP NOT NULL,
    replication_lag INTEGER,  -- Seconds, NULL if not applicable
    io_thread_running BOOLEAN,
    sql_thread_running BOOLEAN,
    last_io_error TEXT,
    last_sql_error TEXT,
    read_only BOOLEAN,
    source_host TEXT,
    source_port INTEGER,
    FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
);

-- Downstream relationships (which nodes replicate from this node)
CREATE TABLE IF NOT EXISTS node_downstream (
    node_id TEXT NOT NULL,
    downstream_node_id TEXT NOT NULL,
    PRIMARY KEY (node_id, downstream_node_id),
    FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE,
    FOREIGN KEY (downstream_node_id) REFERENCES nodes(id) ON DELETE CASCADE
);

-- Indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_clusters_group_id ON clusters(group_id);
CREATE INDEX IF NOT EXISTS idx_nodes_cluster_id ON nodes(cluster_id);
CREATE INDEX IF NOT EXISTS idx_nodes_source ON nodes(source_node_id);
CREATE INDEX IF NOT EXISTS idx_node_health_check_time ON node_health(last_check_time);
CREATE INDEX IF NOT EXISTS idx_node_downstream_downstream_id ON node_downstream(downstream_node_id);
`
