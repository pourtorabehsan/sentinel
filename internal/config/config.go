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
// Infrastructure settings only - groups/clusters are managed in the database.
type Config struct {
	Mode           string               `yaml:"mode"`
	LogLevel       string               `yaml:"log_level,omitempty"`
	StateStore     StateStoreConfig     `yaml:"state_store"`
	LeaderElection LeaderElectionConfig `yaml:"leader_election"`
	API            APIConfig            `yaml:"api"`
	Monitor        MonitorConfig        `yaml:"monitor"`
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

// APIConfig configures the HTTP API server.
type APIConfig struct {
	Enabled bool `yaml:"enabled"` // Default: true
	Port    int  `yaml:"port"`    // Default: 8080
}

// MonitorConfig configures the monitoring behavior.
type MonitorConfig struct {
	Enabled         bool `yaml:"enabled"`          // Default: true
	IntervalSeconds int  `yaml:"interval_seconds"` // Default: 10
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

	// Apply defaults
	config.applyDefaults()

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// applyDefaults sets default values for optional fields.
func (c *Config) applyDefaults() {
	// API defaults
	if c.API.Port == 0 {
		c.API.Port = 8080
	}
	// API is enabled by default unless explicitly disabled
	// (zero value for bool is false, so we can't distinguish between unset and false)
	// In practice, users should explicitly set enabled: false if they want to disable it

	// Monitor defaults
	if c.Monitor.IntervalSeconds == 0 {
		c.Monitor.IntervalSeconds = 10
	}
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

	// Validate API config
	if c.API.Port <= 0 || c.API.Port > 65535 {
		return fmt.Errorf("api.port must be between 1 and 65535, got: %d", c.API.Port)
	}

	// Validate Monitor config
	if c.Monitor.IntervalSeconds <= 0 {
		return fmt.Errorf("monitor.interval_seconds must be positive, got: %d", c.Monitor.IntervalSeconds)
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
	return fmt.Sprintf("Config{Mode=%s, StateStore=%s, LeaderElection=%s, API.Port=%d, Monitor.Interval=%ds}",
		c.Mode, c.StateStore.Type, c.LeaderElection.Backend, c.API.Port, c.Monitor.IntervalSeconds)
}
