package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	logger := New(LevelInfo)
	if logger == nil {
		t.Fatal("expected logger to be created")
	}
}

func TestNewWithWriter(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter(&buf, LevelInfo)

	logger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("expected output to contain 'test message', got: %s", output)
	}

	// Verify JSON format
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Errorf("expected valid JSON output, got error: %v", err)
	}
}

func TestWithCluster(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter(&buf, LevelInfo)
	logger = WithCluster(logger, "test-cluster")

	logger.Info("test message")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	cluster, ok := logEntry["cluster"]
	if !ok {
		t.Error("expected 'cluster' field in log output")
	}
	if cluster != "test-cluster" {
		t.Errorf("expected cluster='test-cluster', got: %v", cluster)
	}
}

func TestWithNode(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter(&buf, LevelInfo)
	logger = WithNode(logger, "10.0.1.5:3306")

	logger.Info("test message")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	node, ok := logEntry["node"]
	if !ok {
		t.Error("expected 'node' field in log output")
	}
	if node != "10.0.1.5:3306" {
		t.Errorf("expected node='10.0.1.5:3306', got: %v", node)
	}
}

func TestWithOperation(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter(&buf, LevelInfo)
	logger = WithOperation(logger, "failover-123")

	logger.Info("test message")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	opID, ok := logEntry["operation_id"]
	if !ok {
		t.Error("expected 'operation_id' field in log output")
	}
	if opID != "failover-123" {
		t.Errorf("expected operation_id='failover-123', got: %v", opID)
	}
}

func TestWithGroup(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter(&buf, LevelInfo)
	logger = WithGroup(logger, "production")

	logger.Info("test message")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	group, ok := logEntry["group"]
	if !ok {
		t.Error("expected 'group' field in log output")
	}
	if group != "production" {
		t.Errorf("expected group='production', got: %v", group)
	}
}

func TestWithContext(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter(&buf, LevelInfo)
	logger = WithContext(logger, "production", "prod-mysql-1", "10.0.1.5:3306")

	logger.Info("test message")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if logEntry["group"] != "production" {
		t.Errorf("expected group='production', got: %v", logEntry["group"])
	}
	if logEntry["cluster"] != "prod-mysql-1" {
		t.Errorf("expected cluster='prod-mysql-1', got: %v", logEntry["cluster"])
	}
	if logEntry["node"] != "10.0.1.5:3306" {
		t.Errorf("expected node='10.0.1.5:3306', got: %v", logEntry["node"])
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", LevelDebug},
		{"info", LevelInfo},
		{"warn", LevelWarn},
		{"warning", LevelWarn},
		{"error", LevelError},
		{"invalid", LevelInfo},
		{"", LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseLevel(tt.input)
			if result != tt.expected {
				t.Errorf("ParseLevel(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLogLevels(t *testing.T) {
	tests := []struct {
		name      string
		level     Level
		logLevel  Level
		shouldLog bool
	}{
		{"debug logs at debug level", LevelDebug, LevelDebug, true},
		{"info logs at debug level", LevelDebug, LevelInfo, true},
		{"debug doesn't log at info level", LevelInfo, LevelDebug, false},
		{"info logs at info level", LevelInfo, LevelInfo, true},
		{"warn logs at info level", LevelInfo, LevelWarn, true},
		{"error logs at info level", LevelInfo, LevelError, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewWithWriter(&buf, tt.level)

			switch tt.logLevel {
			case LevelDebug:
				logger.Debug("test message")
			case LevelInfo:
				logger.Info("test message")
			case LevelWarn:
				logger.Warn("test message")
			case LevelError:
				logger.Error("test message")
			}

			hasOutput := buf.Len() > 0
			if hasOutput != tt.shouldLog {
				t.Errorf("expected shouldLog=%v, but hasOutput=%v", tt.shouldLog, hasOutput)
			}
		})
	}
}
