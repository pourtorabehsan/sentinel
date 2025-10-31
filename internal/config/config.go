// Package config provides configuration loading and validation for Sentinel.
// It supports hierarchical configuration: Global → Group → Cluster → Node
package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the root configuration for Sentinel.
type Config struct {
	Mode           string               `yaml:"mode"`
	LogLevel       string               `yaml:"log_level,omitempty"`
	StateStore     StateStoreConfig     `yaml:"state_store"`
	LeaderElection LeaderElectionConfig `yaml:"leader_election"`
	Groups         []Group              `yaml:"groups"`
}

// StateStoreConfig configures the state storage backend.
type StateStoreConfig struct {
	Type string `yaml:"type"` // mysql, postgresql, sqlite
	DSN  string `yaml:"dsn"`  // Data Source Name
}

// LeaderElectionConfig configures the leader election backend.
type LeaderElectionConfig struct {
	Backend   string   `yaml:"backend"`   // etcd, consul, zookeeper, database, in-memory
	Endpoints []string `yaml:"endpoints"` // Backend endpoints
}

// Group represents a logical grouping of clusters (e.g., production, staging).
type Group struct {
	ID       string    `yaml:"id"`
	Clusters []Cluster `yaml:"clusters"`
}

// Cluster represents a MySQL replication topology.
type Cluster struct {
	ID        string     `yaml:"id"`
	SeedNodes []SeedNode `yaml:"seed_nodes"`
}

// SeedNode represents an initial MySQL node to discover the topology.
type SeedNode struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// LoadFromFile loads configuration from a YAML file.
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// Validate performs validation on the configuration.
func (c *Config) Validate() error {
	// Validate mode
	if c.Mode == "" {
		return fmt.Errorf("mode is required")
	}
	mode := strings.ToLower(c.Mode)
	if mode != "multi" && mode != "single" {
		return fmt.Errorf("mode must be 'multi' or 'single', got: %s", c.Mode)
	}

	// Validate state store
	if err := c.StateStore.Validate(); err != nil {
		return fmt.Errorf("state_store: %w", err)
	}

	// Validate leader election
	if err := c.LeaderElection.Validate(); err != nil {
		return fmt.Errorf("leader_election: %w", err)
	}

	// Validate groups
	if len(c.Groups) == 0 {
		return fmt.Errorf("at least one group is required")
	}

	groupIDs := make(map[string]bool)
	for i, group := range c.Groups {
		if err := group.Validate(); err != nil {
			return fmt.Errorf("groups[%d]: %w", i, err)
		}
		if groupIDs[group.ID] {
			return fmt.Errorf("duplicate group ID: %s", group.ID)
		}
		groupIDs[group.ID] = true
	}

	return nil
}

// Validate performs validation on StateStoreConfig.
func (s *StateStoreConfig) Validate() error {
	if s.Type == "" {
		return fmt.Errorf("type is required")
	}

	validTypes := map[string]bool{
		"mysql":      true,
		"postgresql": true,
		"sqlite":     true,
	}
	if !validTypes[strings.ToLower(s.Type)] {
		return fmt.Errorf("invalid type: %s (must be mysql, postgresql, or sqlite)", s.Type)
	}

	if s.DSN == "" {
		return fmt.Errorf("dsn is required")
	}

	return nil
}

// Validate performs validation on LeaderElectionConfig.
func (l *LeaderElectionConfig) Validate() error {
	if l.Backend == "" {
		return fmt.Errorf("backend is required")
	}

	validBackends := map[string]bool{
		"etcd":      true,
		"consul":    true,
		"zookeeper": true,
		"database":  true,
		"in-memory": true,
	}
	backend := strings.ToLower(l.Backend)
	if !validBackends[backend] {
		return fmt.Errorf("invalid backend: %s", l.Backend)
	}

	// in-memory backend doesn't need endpoints
	if backend != "in-memory" && len(l.Endpoints) == 0 {
		return fmt.Errorf("at least one endpoint is required for backend: %s", l.Backend)
	}

	return nil
}

// Validate performs validation on Group.
func (g *Group) Validate() error {
	if g.ID == "" {
		return fmt.Errorf("id is required")
	}

	if len(g.Clusters) == 0 {
		return fmt.Errorf("at least one cluster is required in group: %s", g.ID)
	}

	clusterIDs := make(map[string]bool)
	for i, cluster := range g.Clusters {
		if err := cluster.Validate(); err != nil {
			return fmt.Errorf("clusters[%d]: %w", i, err)
		}
		if clusterIDs[cluster.ID] {
			return fmt.Errorf("duplicate cluster ID: %s", cluster.ID)
		}
		clusterIDs[cluster.ID] = true
	}

	return nil
}

// Validate performs validation on Cluster.
func (c *Cluster) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("id is required")
	}

	if len(c.SeedNodes) == 0 {
		return fmt.Errorf("at least one seed node is required in cluster: %s", c.ID)
	}

	for i, node := range c.SeedNodes {
		if err := node.Validate(); err != nil {
			return fmt.Errorf("seed_nodes[%d]: %w", i, err)
		}
	}

	return nil
}

// Validate performs validation on SeedNode.
func (s *SeedNode) Validate() error {
	if s.Host == "" {
		return fmt.Errorf("host is required")
	}

	if s.Port <= 0 || s.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got: %d", s.Port)
	}

	return nil
}

// SanitizedDSN returns a sanitized version of the DSN with password hidden.
// This is useful for logging configuration without exposing sensitive data.
func (s *StateStoreConfig) SanitizedDSN() string {
	dsn := s.DSN

	// Handle common DSN formats:
	// MySQL: user:password@tcp(host:port)/database
	// PostgreSQL: postgres://user:password@host:port/database
	// SQLite: file:path or :memory:

	if strings.Contains(dsn, ":memory:") {
		return dsn
	}

	if strings.HasPrefix(dsn, "file:") {
		return dsn
	}

	// Handle PostgreSQL URL format: postgres://user:password@host/db
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		// Find the position after the scheme
		schemeEnd := strings.Index(dsn, "://") + 3
		rest := dsn[schemeEnd:]

		// Find password in the rest of the URL
		if idx := strings.Index(rest, ":"); idx != -1 {
			if idx2 := strings.Index(rest[idx+1:], "@"); idx2 != -1 {
				return dsn[:schemeEnd+idx+1] + "[REDACTED]" + rest[idx+1+idx2:]
			}
		}
		return "[REDACTED]"
	}

	// Handle MySQL format: user:password@tcp(host:port)/database
	if idx := strings.Index(dsn, ":"); idx != -1 {
		if idx2 := strings.Index(dsn[idx+1:], "@"); idx2 != -1 {
			return dsn[:idx+1] + "[REDACTED]" + dsn[idx+1+idx2:]
		}
	}

	return "[REDACTED]"
}

// String returns a string representation of the config with sensitive data redacted.
func (c *Config) String() string {
	return fmt.Sprintf("Config{Mode=%s, StateStore=%s, LeaderElection=%s, Groups=%d}",
		c.Mode, c.StateStore.Type, c.LeaderElection.Backend, len(c.Groups))
}
