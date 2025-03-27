package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// ConnectionManager handles MySQL connections and their lifecycle
type ConnectionManager struct {
	mu          sync.RWMutex
	connections map[string]*sql.DB
	config      ConnectionConfig
}

// ConnectionConfig holds configuration for MySQL connections
type ConnectionConfig struct {
	Username        string
	Password        string
	ConnectTimeout  time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(config ConnectionConfig) *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]*sql.DB),
		config:     config,
	}
}

// GetConnection returns a connection to the specified instance
func (m *ConnectionManager) GetConnection(ctx context.Context, instance *Instance) (*sql.DB, error) {
	key := instance.Key()

	m.mu.RLock()
	db, exists := m.connections[key]
	m.mu.RUnlock()

	if exists {
		if err := db.PingContext(ctx); err == nil {
			return db, nil
		}
		// Connection is invalid, remove it
		m.closeConnection(key)
	}

	return m.createConnection(ctx, instance)
}

func (m *ConnectionManager) createConnection(ctx context.Context, instance *Instance) (*sql.DB, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double check if connection was created while waiting for lock
	if db, exists := m.connections[instance.Key()]; exists {
		return db, nil
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?timeout=%s&readTimeout=%s&writeTimeout=%s",
		m.config.Username,
		m.config.Password,
		instance.HostName,
		instance.Port,
		m.config.ConnectTimeout,
		m.config.ReadTimeout,
		m.config.WriteTimeout,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxIdleConns(m.config.MaxIdleConns)
	db.SetMaxOpenConns(m.config.MaxOpenConns)
	db.SetConnMaxLifetime(m.config.ConnMaxLifetime)

	// Verify connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to verify connection: %w", err)
	}

	m.connections[instance.Key()] = db
	return db, nil
}

func (m *ConnectionManager) closeConnection(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if db, exists := m.connections[key]; exists {
		db.Close()
		delete(m.connections, key)
	}
}

// Close closes all managed connections
func (m *ConnectionManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, db := range m.connections {
		db.Close()
		delete(m.connections, key)
	}
}
