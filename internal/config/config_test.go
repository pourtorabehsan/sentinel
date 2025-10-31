package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFromFile(t *testing.T) {
	validYAML := `
mode: multi
log_level: info
state_store:
  type: sqlite
  dsn: ":memory:"
leader_election:
  backend: in-memory
  endpoints: []
groups:
  - id: test-group
    clusters:
      - id: test-cluster
        seed_nodes:
          - host: localhost
            port: 3306
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(validYAML), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	config, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if config.Mode != "multi" {
		t.Errorf("expected mode='multi', got: %s", config.Mode)
	}
	if config.LogLevel != "info" {
		t.Errorf("expected log_level='info', got: %s", config.LogLevel)
	}
	if len(config.Groups) != 1 {
		t.Errorf("expected 1 group, got: %d", len(config.Groups))
	}
}

func TestLoadFromFile_InvalidPath(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "failed to read config file") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoadFromFile_InvalidYAML(t *testing.T) {
	invalidYAML := `
mode: multi
invalid yaml syntax: [
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := LoadFromFile(configPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid config",
			config: Config{
				Mode: "multi",
				StateStore: StateStoreConfig{
					Type: "sqlite",
					DSN:  ":memory:",
				},
				LeaderElection: LeaderElectionConfig{
					Backend:   "in-memory",
					Endpoints: []string{},
				},
				Groups: []Group{
					{
						ID: "test-group",
						Clusters: []Cluster{
							{
								ID: "test-cluster",
								SeedNodes: []SeedNode{
									{Host: "localhost", Port: 3306},
								},
							},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "missing mode",
			config: Config{
				StateStore: StateStoreConfig{
					Type: "sqlite",
					DSN:  ":memory:",
				},
			},
			expectErr: true,
			errMsg:    "mode is required",
		},
		{
			name: "invalid mode",
			config: Config{
				Mode: "invalid",
				StateStore: StateStoreConfig{
					Type: "sqlite",
					DSN:  ":memory:",
				},
			},
			expectErr: true,
			errMsg:    "mode must be",
		},
		{
			name: "no groups",
			config: Config{
				Mode: "multi",
				StateStore: StateStoreConfig{
					Type: "sqlite",
					DSN:  ":memory:",
				},
				LeaderElection: LeaderElectionConfig{
					Backend: "in-memory",
				},
				Groups: []Group{},
			},
			expectErr: true,
			errMsg:    "at least one group is required",
		},
		{
			name: "duplicate group IDs",
			config: Config{
				Mode: "multi",
				StateStore: StateStoreConfig{
					Type: "sqlite",
					DSN:  ":memory:",
				},
				LeaderElection: LeaderElectionConfig{
					Backend: "in-memory",
				},
				Groups: []Group{
					{
						ID: "group1",
						Clusters: []Cluster{
							{
								ID: "cluster1",
								SeedNodes: []SeedNode{
									{Host: "localhost", Port: 3306},
								},
							},
						},
					},
					{
						ID: "group1",
						Clusters: []Cluster{
							{
								ID: "cluster2",
								SeedNodes: []SeedNode{
									{Host: "localhost", Port: 3307},
								},
							},
						},
					},
				},
			},
			expectErr: true,
			errMsg:    "duplicate group ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.expectErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
			}
		})
	}
}

func TestStateStoreConfigValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    StateStoreConfig
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid mysql",
			config:    StateStoreConfig{Type: "mysql", DSN: "user:pass@tcp(host:3306)/db"},
			expectErr: false,
		},
		{
			name:      "valid postgresql",
			config:    StateStoreConfig{Type: "postgresql", DSN: "postgres://user:pass@host/db"},
			expectErr: false,
		},
		{
			name:      "valid sqlite",
			config:    StateStoreConfig{Type: "sqlite", DSN: ":memory:"},
			expectErr: false,
		},
		{
			name:      "missing type",
			config:    StateStoreConfig{DSN: ":memory:"},
			expectErr: true,
			errMsg:    "type is required",
		},
		{
			name:      "invalid type",
			config:    StateStoreConfig{Type: "redis", DSN: "localhost:6379"},
			expectErr: true,
			errMsg:    "invalid type",
		},
		{
			name:      "missing DSN",
			config:    StateStoreConfig{Type: "sqlite"},
			expectErr: true,
			errMsg:    "dsn is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.expectErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
			}
		})
	}
}

func TestLeaderElectionConfigValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    LeaderElectionConfig
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid etcd",
			config:    LeaderElectionConfig{Backend: "etcd", Endpoints: []string{"http://localhost:2379"}},
			expectErr: false,
		},
		{
			name:      "valid in-memory",
			config:    LeaderElectionConfig{Backend: "in-memory"},
			expectErr: false,
		},
		{
			name:      "missing backend",
			config:    LeaderElectionConfig{},
			expectErr: true,
			errMsg:    "backend is required",
		},
		{
			name:      "invalid backend",
			config:    LeaderElectionConfig{Backend: "redis"},
			expectErr: true,
			errMsg:    "invalid backend",
		},
		{
			name:      "etcd missing endpoints",
			config:    LeaderElectionConfig{Backend: "etcd", Endpoints: []string{}},
			expectErr: true,
			errMsg:    "at least one endpoint is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.expectErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
			}
		})
	}
}

func TestGroupValidate(t *testing.T) {
	tests := []struct {
		name      string
		group     Group
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid group",
			group: Group{
				ID: "test-group",
				Clusters: []Cluster{
					{
						ID: "cluster1",
						SeedNodes: []SeedNode{
							{Host: "localhost", Port: 3306},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name:      "missing ID",
			group:     Group{},
			expectErr: true,
			errMsg:    "id is required",
		},
		{
			name: "no clusters",
			group: Group{
				ID:       "test-group",
				Clusters: []Cluster{},
			},
			expectErr: true,
			errMsg:    "at least one cluster is required",
		},
		{
			name: "duplicate cluster IDs",
			group: Group{
				ID: "test-group",
				Clusters: []Cluster{
					{
						ID: "cluster1",
						SeedNodes: []SeedNode{
							{Host: "localhost", Port: 3306},
						},
					},
					{
						ID: "cluster1",
						SeedNodes: []SeedNode{
							{Host: "localhost", Port: 3307},
						},
					},
				},
			},
			expectErr: true,
			errMsg:    "duplicate cluster ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.group.Validate()
			if tt.expectErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.expectErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
			}
		})
	}
}

func TestClusterValidate(t *testing.T) {
	tests := []struct {
		name      string
		cluster   Cluster
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid cluster",
			cluster: Cluster{
				ID: "test-cluster",
				SeedNodes: []SeedNode{
					{Host: "localhost", Port: 3306},
				},
			},
			expectErr: false,
		},
		{
			name:      "missing ID",
			cluster:   Cluster{},
			expectErr: true,
			errMsg:    "id is required",
		},
		{
			name: "no seed nodes",
			cluster: Cluster{
				ID:        "test-cluster",
				SeedNodes: []SeedNode{},
			},
			expectErr: true,
			errMsg:    "at least one seed node is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cluster.Validate()
			if tt.expectErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.expectErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
			}
		})
	}
}

func TestSeedNodeValidate(t *testing.T) {
	tests := []struct {
		name      string
		node      SeedNode
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid node",
			node:      SeedNode{Host: "localhost", Port: 3306},
			expectErr: false,
		},
		{
			name:      "missing host",
			node:      SeedNode{Port: 3306},
			expectErr: true,
			errMsg:    "host is required",
		},
		{
			name:      "invalid port zero",
			node:      SeedNode{Host: "localhost", Port: 0},
			expectErr: true,
			errMsg:    "port must be between",
		},
		{
			name:      "invalid port negative",
			node:      SeedNode{Host: "localhost", Port: -1},
			expectErr: true,
			errMsg:    "port must be between",
		},
		{
			name:      "invalid port too large",
			node:      SeedNode{Host: "localhost", Port: 65536},
			expectErr: true,
			errMsg:    "port must be between",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.node.Validate()
			if tt.expectErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.expectErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
			}
		})
	}
}

func TestSanitizedDSN(t *testing.T) {
	tests := []struct {
		name     string
		dsn      string
		expected string
	}{
		{
			name:     "mysql with password",
			dsn:      "user:secret@tcp(localhost:3306)/database",
			expected: "user:[REDACTED]@tcp(localhost:3306)/database",
		},
		{
			name:     "postgresql with password",
			dsn:      "postgres://user:secret@localhost:5432/database",
			expected: "postgres://user:[REDACTED]@localhost:5432/database",
		},
		{
			name:     "sqlite in-memory",
			dsn:      ":memory:",
			expected: ":memory:",
		},
		{
			name:     "sqlite file",
			dsn:      "file:/path/to/database.db",
			expected: "file:/path/to/database.db",
		},
		{
			name:     "no password",
			dsn:      "user@tcp(localhost:3306)/database",
			expected: "[REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := StateStoreConfig{DSN: tt.dsn}
			result := config.SanitizedDSN()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestConfigString(t *testing.T) {
	config := Config{
		Mode: "multi",
		StateStore: StateStoreConfig{
			Type: "sqlite",
			DSN:  ":memory:",
		},
		LeaderElection: LeaderElectionConfig{
			Backend: "in-memory",
		},
		Groups: []Group{
			{ID: "group1"},
			{ID: "group2"},
		},
	}

	result := config.String()
	if !strings.Contains(result, "Mode=multi") {
		t.Errorf("expected Mode=multi in output, got: %s", result)
	}
	if !strings.Contains(result, "Groups=2") {
		t.Errorf("expected Groups=2 in output, got: %s", result)
	}
}
