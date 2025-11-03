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
api:
  enabled: true
  port: 8080
monitor:
  enabled: true
  interval_seconds: 10
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
	if config.API.Port != 8080 {
		t.Errorf("expected api.port=8080, got: %d", config.API.Port)
	}
	if config.Monitor.IntervalSeconds != 10 {
		t.Errorf("expected monitor.interval_seconds=10, got: %d", config.Monitor.IntervalSeconds)
	}
}

func TestLoadFromFile_Defaults(t *testing.T) {
	// Config without API and Monitor sections should get defaults
	minimalYAML := `
mode: multi
state_store:
  type: sqlite
  dsn: ":memory:"
leader_election:
  backend: in-memory
  endpoints: []
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(minimalYAML), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	config, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	// Check defaults are applied
	if config.API.Port != 8080 {
		t.Errorf("expected default api.port=8080, got: %d", config.API.Port)
	}
	if config.Monitor.IntervalSeconds != 10 {
		t.Errorf("expected default monitor.interval_seconds=10, got: %d", config.Monitor.IntervalSeconds)
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
				API: APIConfig{
					Enabled: true,
					Port:    8080,
				},
				Monitor: MonitorConfig{
					Enabled:         true,
					IntervalSeconds: 10,
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
				API: APIConfig{Port: 8080},
				Monitor: MonitorConfig{IntervalSeconds: 10},
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
				API: APIConfig{Port: 8080},
				Monitor: MonitorConfig{IntervalSeconds: 10},
			},
			expectErr: true,
			errMsg:    "mode must be",
		},
		{
			name: "invalid api port",
			config: Config{
				Mode: "multi",
				StateStore: StateStoreConfig{
					Type: "sqlite",
					DSN:  ":memory:",
				},
				LeaderElection: LeaderElectionConfig{
					Backend: "in-memory",
				},
				API: APIConfig{
					Port: 0,
				},
				Monitor: MonitorConfig{IntervalSeconds: 10},
			},
			expectErr: true,
			errMsg:    "api.port must be between",
		},
		{
			name: "invalid monitor interval",
			config: Config{
				Mode: "multi",
				StateStore: StateStoreConfig{
					Type: "sqlite",
					DSN:  ":memory:",
				},
				LeaderElection: LeaderElectionConfig{
					Backend: "in-memory",
				},
				API: APIConfig{Port: 8080},
				Monitor: MonitorConfig{
					IntervalSeconds: 0,
				},
			},
			expectErr: true,
			errMsg:    "monitor.interval_seconds must be positive",
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
		API: APIConfig{
			Port: 8080,
		},
		Monitor: MonitorConfig{
			IntervalSeconds: 10,
		},
	}

	result := config.String()
	if !strings.Contains(result, "Mode=multi") {
		t.Errorf("expected Mode=multi in output, got: %s", result)
	}
	if !strings.Contains(result, "API.Port=8080") {
		t.Errorf("expected API.Port=8080 in output, got: %s", result)
	}
	if !strings.Contains(result, "Monitor.Interval=10s") {
		t.Errorf("expected Monitor.Interval=10s in output, got: %s", result)
	}
}
