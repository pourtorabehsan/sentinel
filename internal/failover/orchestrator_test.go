package failover

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pourtorabehsan/sentinel/internal/domain"
	"github.com/pourtorabehsan/sentinel/internal/storage/memory"
)

func setupTestCluster(t *testing.T) (storage *memory.MemoryStorage, clusterID string, primaryID string, replicaIDs []string) {
	storage = memory.NewMemoryStorage()
	ctx := context.Background()

	// Create test cluster
	cluster := domain.Cluster{
		ID:          uuid.New().String(),
		Name:        "test-cluster",
		Description: "Test cluster",
		CreatedAt:   time.Now(),
	}
	if err := storage.AddCluster(ctx, cluster); err != nil {
		t.Fatalf("Failed to create test cluster: %v", err)
	}

	// Create primary instance
	primary := domain.Instance{
		ID:        uuid.New().String(),
		Host:      "primary.local",
		Port:      3306,
		Role:      domain.RolePrimary,
		ClusterID: cluster.ID,
	}
	primaryCreds := domain.Credentials{
		Username: "primary",
		Password: "password",
	}
	if err := storage.AddInstance(ctx, primary, primaryCreds); err != nil {
		t.Fatalf("Failed to create primary instance: %v", err)
	}

	// Create replica instances
	replicaIDs = make([]string, 2)
	for i := 0; i < 2; i++ {
		replica := domain.Instance{
			ID:        uuid.New().String(),
			Host:      fmt.Sprintf("replica%d.local", i),
			Port:      3306,
			Role:      domain.RoleReplica,
			ClusterID: cluster.ID,
		}
		replicaCreds := domain.Credentials{
			Username: fmt.Sprintf("replica%d", i),
			Password: "password",
		}
		if err := storage.AddInstance(ctx, replica, replicaCreds); err != nil {
			t.Fatalf("Failed to create replica instance: %v", err)
		}
		replicaIDs[i] = replica.ID
	}

	return storage, cluster.ID, primary.ID, replicaIDs
}

func TestOrchestrator_NeedsFailover(t *testing.T) {
	storage, _, primaryID, _ := setupTestCluster(t)
	ctx := context.Background()

	orchestrator := NewOrchestrator(storage, 30*time.Second, 10*time.Second, true)

	tests := []struct {
		name     string
		health   domain.InstanceHealth
		expected bool
	}{
		{
			name: "healthy instance",
			health: domain.InstanceHealth{
				IsAlive:     true,
				LastChecked: time.Now(),
			},
			expected: false,
		},
		{
			name: "recently failed instance",
			health: domain.InstanceHealth{
				IsAlive:     false,
				LastChecked: time.Now(),
			},
			expected: false,
		},
		{
			name: "instance failed long ago",
			health: domain.InstanceHealth{
				IsAlive:     false,
				LastChecked: time.Now().Add(-60 * time.Second),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := storage.UpdateInstanceHealth(ctx, primaryID, tt.health); err != nil {
				t.Fatalf("Failed to update health: %v", err)
			}

			health, err := storage.GetInstanceHealth(ctx, primaryID)
			if err != nil {
				t.Fatalf("Failed to get health: %v", err)
			}

			if got := orchestrator.needsFailover(health); got != tt.expected {
				t.Errorf("needsFailover() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestOrchestrator_SelectNewPrimary(t *testing.T) {
	storage, clusterID, _, replicaIDs := setupTestCluster(t)
	ctx := context.Background()

	orchestrator := NewOrchestrator(storage, 30*time.Second, 10*time.Second, true)

	tests := []struct {
		name          string
		healthStates  map[string]domain.InstanceHealth
		expectError   bool
		expectedID    string
	}{
		{
			name: "select replica with least lag",
			healthStates: map[string]domain.InstanceHealth{
				replicaIDs[0]: {
					IsAlive:       true,
					LastChecked:   time.Now(),
					ReplicationLag: 5,
				},
				replicaIDs[1]: {
					IsAlive:       true,
					LastChecked:   time.Now(),
					ReplicationLag: 2,
				},
			},
			expectError: false,
			expectedID:  replicaIDs[1],
		},
		{
			name: "no healthy replicas",
			healthStates: map[string]domain.InstanceHealth{
				replicaIDs[0]: {
					IsAlive:     false,
					LastChecked: time.Now(),
				},
				replicaIDs[1]: {
					IsAlive:     false,
					LastChecked: time.Now(),
				},
			},
			expectError: true,
			expectedID:  "",
		},
		{
			name: "replicas with too much lag",
			healthStates: map[string]domain.InstanceHealth{
				replicaIDs[0]: {
					IsAlive:       true,
					LastChecked:   time.Now(),
					ReplicationLag: 20,
				},
				replicaIDs[1]: {
					IsAlive:       true,
					LastChecked:   time.Now(),
					ReplicationLag: 15,
				},
			},
			expectError: true,
			expectedID:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set health states
			for id, health := range tt.healthStates {
				if err := storage.UpdateInstanceHealth(ctx, id, health); err != nil {
					t.Fatalf("Failed to update health: %v", err)
				}
			}

			// Get replicas
			instances, err := storage.ListInstances(ctx, clusterID)
			if err != nil {
				t.Fatalf("Failed to list instances: %v", err)
			}

			var replicas []domain.Instance
			for _, instance := range instances {
				if instance.Role == domain.RoleReplica {
					replicas = append(replicas, instance)
				}
			}

			newPrimary, err := orchestrator.selectNewPrimary(ctx, replicas)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if newPrimary.ID != tt.expectedID {
				t.Errorf("Got primary ID %s, want %s", newPrimary.ID, tt.expectedID)
			}
		})
	}
}
