package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
)

// mysqlClient implements the Client interface.
type mysqlClient struct {
	dsn     string
	db      *sql.DB
	version string // Cached MySQL version
}

// NewClient creates a new MySQL client.
// The DSN format is: "username:password@tcp(host:port)/database?parseTime=true"
func NewClient(host string, port int, username, password string) (Client, error) {
	// Build DSN without database (we don't need to select a database for our queries)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?parseTime=true&timeout=5s",
		username, password, host, port)

	return &mysqlClient{
		dsn: dsn,
	}, nil
}

// Connect establishes a connection to MySQL.
func (c *mysqlClient) Connect(ctx context.Context) error {
	db, err := sql.Open("mysql", c.dsn)
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}

	// Test connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)

	c.db = db

	// Cache version on connect
	version, err := c.GetVersion(ctx)
	if err != nil {
		db.Close()
		return fmt.Errorf("failed to get version: %w", err)
	}
	c.version = version

	return nil
}

// Close closes the MySQL connection.
func (c *mysqlClient) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// Ping tests the connection to MySQL.
func (c *mysqlClient) Ping(ctx context.Context) error {
	if c.db == nil {
		return fmt.Errorf("not connected")
	}
	return c.db.PingContext(ctx)
}

// GetVersion returns the MySQL version string.
func (c *mysqlClient) GetVersion(ctx context.Context) (string, error) {
	if c.db == nil {
		return "", fmt.Errorf("not connected")
	}

	var version string
	err := c.db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version)
	if err != nil {
		return "", fmt.Errorf("failed to query version: %w", err)
	}
	return version, nil
}

// GetReplicationStatus returns the replication status of this MySQL instance.
// Returns nil if this instance is not a replica.
func (c *mysqlClient) GetReplicationStatus(ctx context.Context) (*ReplicationStatus, error) {
	if c.db == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Try MySQL 8.0+ syntax first (SHOW REPLICA STATUS)
	query := "SHOW REPLICA STATUS"
	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		// Fall back to legacy syntax for MySQL 5.7 and earlier
		query = "SHOW SLAVE STATUS"
		rows, err = c.db.QueryContext(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("failed to query replication status: %w", err)
		}
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	// Check if there are any rows (no rows means not a replica)
	if !rows.Next() {
		return nil, nil
	}

	// Create a map to scan results
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, fmt.Errorf("failed to scan row: %w", err)
	}

	// Build column map for easier access
	colMap := make(map[string]interface{})
	for i, col := range columns {
		colMap[col] = values[i]
	}

	status := &ReplicationStatus{}

	// Extract fields (handling both MySQL 8.0+ and 5.7 column names)
	status.SourceHost = getStringValue(colMap, "Source_Host", "Master_Host")
	status.SourcePort = getIntValue(colMap, "Source_Port", "Master_Port")
	status.IOThreadRunning = getStringValue(colMap, "Replica_IO_Running", "Slave_IO_Running") == "Yes"
	status.SQLThreadRunning = getStringValue(colMap, "Replica_SQL_Running", "Slave_SQL_Running") == "Yes"
	status.LastIOError = getStringValue(colMap, "Last_IO_Error")
	status.LastSQLError = getStringValue(colMap, "Last_SQL_Error")
	status.ExecutedGTIDSet = getStringValue(colMap, "Executed_Gtid_Set")
	status.RetrievedGTIDSet = getStringValue(colMap, "Retrieved_Gtid_Set")

	// Handle Seconds_Behind_Source / Seconds_Behind_Master (can be NULL)
	if sbsVal := getInt64ValuePtr(colMap, "Seconds_Behind_Source", "Seconds_Behind_Master"); sbsVal != nil {
		status.SecondsBehindSource = sbsVal
	}

	return status, nil
}

// GetReplicas returns a list of replicas connected to this MySQL instance.
func (c *mysqlClient) GetReplicas(ctx context.Context) ([]*ReplicaInfo, error) {
	if c.db == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Try MySQL 8.0.22+ syntax first (SHOW REPLICAS)
	query := "SHOW REPLICAS"
	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		// Fall back to legacy syntax
		query = "SHOW SLAVE HOSTS"
		rows, err = c.db.QueryContext(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("failed to query replicas: %w", err)
		}
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var replicas []*ReplicaInfo
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Build column map
		colMap := make(map[string]interface{})
		for i, col := range columns {
			colMap[col] = values[i]
		}

		replica := &ReplicaInfo{
			ServerID: uint32(getIntValue(colMap, "Server_id")),
			Host:     getStringValue(colMap, "Host"),
			Port:     getIntValue(colMap, "Port"),
			UUID:     getStringValue(colMap, "Slave_UUID", "Replica_UUID"),
		}
		replicas = append(replicas, replica)
	}

	return replicas, rows.Err()
}

// IsReadOnly checks if the MySQL instance is in read-only mode.
func (c *mysqlClient) IsReadOnly(ctx context.Context) (bool, error) {
	if c.db == nil {
		return false, fmt.Errorf("not connected")
	}

	var variable, value string
	err := c.db.QueryRowContext(ctx, "SHOW VARIABLES LIKE 'read_only'").Scan(&variable, &value)
	if err != nil {
		return false, fmt.Errorf("failed to query read_only: %w", err)
	}

	return strings.ToLower(value) == "on" || value == "1", nil
}

// GetServerID returns the MySQL server_id.
func (c *mysqlClient) GetServerID(ctx context.Context) (uint32, error) {
	if c.db == nil {
		return 0, fmt.Errorf("not connected")
	}

	var serverID uint32
	err := c.db.QueryRowContext(ctx, "SELECT @@server_id").Scan(&serverID)
	if err != nil {
		return 0, fmt.Errorf("failed to query server_id: %w", err)
	}
	return serverID, nil
}

// GetGTIDExecuted returns the gtid_executed set.
func (c *mysqlClient) GetGTIDExecuted(ctx context.Context) (string, error) {
	if c.db == nil {
		return "", fmt.Errorf("not connected")
	}

	var gtidExecuted string
	err := c.db.QueryRowContext(ctx, "SELECT @@gtid_executed").Scan(&gtidExecuted)
	if err != nil {
		return "", fmt.Errorf("failed to query gtid_executed: %w", err)
	}
	return gtidExecuted, nil
}

// Helper functions to safely extract values from column map

func getStringValue(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val, ok := m[key]; ok && val != nil {
			if b, ok := val.([]byte); ok {
				return string(b)
			}
			if s, ok := val.(string); ok {
				return s
			}
		}
	}
	return ""
}

func getIntValue(m map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		if val, ok := m[key]; ok && val != nil {
			switch v := val.(type) {
			case int64:
				return int(v)
			case int32:
				return int(v)
			case int:
				return v
			case []byte:
				// Try to parse string to int
				var i int
				fmt.Sscanf(string(v), "%d", &i)
				return i
			}
		}
	}
	return 0
}

func getInt64ValuePtr(m map[string]interface{}, keys ...string) *int64 {
	for _, key := range keys {
		if val, ok := m[key]; ok && val != nil {
			switch v := val.(type) {
			case int64:
				return &v
			case int32:
				val64 := int64(v)
				return &val64
			case int:
				val64 := int64(v)
				return &val64
			case []byte:
				// Try to parse string to int64
				var i int64
				if _, err := fmt.Sscanf(string(v), "%d", &i); err == nil {
					return &i
				}
			}
		}
	}
	return nil
}
