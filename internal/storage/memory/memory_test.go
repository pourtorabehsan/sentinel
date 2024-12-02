package memory

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pourtorabehsan/sentinel/internal/domain"
)

func TestMemoryStorage_Cluster(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()

	// Test cluster operations
	cluster := domain.Cluster{
		ID:          uuid.New().String(),
		Name:        "test-cluster",
		Description: "Test cluster",
		CreatedAt:   time.Now(),
	}

	// Test AddCluster
	t.Run("AddCluster", func(t *testing.T) {
		if err := storage.AddCluster(ctx, cluster); err != nil {
			t.Errorf("Failed to add cluster: %v", err)
		}

		// Try adding same cluster again
		if err := storage.AddCluster(ctx, cluster); err == nil {
			t.Error("Expected error when adding duplicate cluster")
		}
	})

	// Test GetCluster
	t.Run("GetCluster", func(t *testing.T) {
		got, err := storage.GetCluster(ctx, cluster.ID)
		if err != nil {
			t.Errorf("Failed to get cluster: %v", err)
		}

		if got.ID != cluster.ID || got.Name != cluster.Name {
			t.Errorf("Got cluster %v, want %v", got, cluster)
		}

		// Test non-existent cluster
		_, err = storage.GetCluster(ctx, "non-existent")
		if err == nil {
			t.Error("Expected error when getting non-existent cluster")
		}
	})

	// Test ListClusters
	t.Run("ListClusters", func(t *testing.T) {
		clusters, err := storage.ListClusters(ctx)
		if err != nil {
			t.Errorf("Failed to list clusters: %v", err)
		}

		if len(clusters) != 1 {
			t.Errorf("Got %d clusters, want 1", len(clusters))
		}
	})

	// Test DeleteCluster
	t.Run("DeleteCluster", func(t *testing.T) {
		if err := storage.DeleteCluster(ctx, cluster.ID); err != nil {
			t.Errorf("Failed to delete cluster: %v", err)
		}

		// Verify cluster is deleted
		_, err := storage.GetCluster(ctx, cluster.ID)
		if err == nil {
			t.Error("Expected error when getting deleted cluster")
		}
	})
}

func TestMemoryStorage_Instance(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()

	// Create a test cluster first
	cluster := domain.Cluster{
		ID:          uuid.New().String(),
		Name:        "test-cluster",
		Description: "Test cluster",
		CreatedAt:   time.Now(),
	}
	if err := storage.AddCluster(ctx, cluster); err != nil {
		t.Fatalf("Failed to create test cluster: %v", err)
	}

	// Test instance operations
	instance := domain.Instance{
		ID:        uuid.New().String(),
		Host:      "localhost",
		Port:      3306,
		Role:      domain.RolePrimary,
		ClusterID: cluster.ID,
	}

	creds := domain.Credentials{
		Username: "test-user",
		Password: "test-pass",
	}

	// Test AddInstance
	t.Run("AddInstance", func(t *testing.T) {
		if err := storage.AddInstance(ctx, instance, creds); err != nil {
			t.Errorf("Failed to add instance: %v", err)
		}

		// Try adding same instance again
		if err := storage.AddInstance(ctx, instance, creds); err == nil {
			t.Error("Expected error when adding duplicate instance")
		}

		// Try adding instance with non-existent cluster
		invalidInstance := instance
		invalidInstance.ID = uuid.New().String()
		invalidInstance.ClusterID = "non-existent"
		if err := storage.AddInstance(ctx, invalidInstance, creds); err == nil {
			t.Error("Expected error when adding instance with non-existent cluster")
		}
	})

	// Test GetInstance
	t.Run("GetInstance", func(t *testing.T) {
		got, err := storage.GetInstance(ctx, instance.ID)
		if err != nil {
			t.Errorf("Failed to get instance: %v", err)
		}

		if got.ID != instance.ID || got.Host != instance.Host {
			t.Errorf("Got instance %v, want %v", got, instance)
		}
	})

	// Test ListInstances
	t.Run("ListInstances", func(t *testing.T) {
		instances, err := storage.ListInstances(ctx, cluster.ID)
		if err != nil {
			t.Errorf("Failed to list instances: %v", err)
		}

		if len(instances) != 1 {
			t.Errorf("Got %d instances, want 1", len(instances))
		}
	})

	// Test UpdateInstanceHealth
	t.Run("UpdateInstanceHealth", func(t *testing.T) {
		health := domain.InstanceHealth{
			IsAlive:       true,
			LastChecked:   time.Now(),
			ReplicationLag: 0,
		}

		if err := storage.UpdateInstanceHealth(ctx, instance.ID, health); err != nil {
			t.Errorf("Failed to update instance health: %v", err)
		}

		got, err := storage.GetInstanceHealth(ctx, instance.ID)
		if err != nil {
			t.Errorf("Failed to get instance health: %v", err)
		}

		if !got.IsAlive || got.ReplicationLag != health.ReplicationLag {
			t.Errorf("Got health %v, want %v", got, health)
		}
	})

	// Test GetInstanceCredentials
	t.Run("GetInstanceCredentials", func(t *testing.T) {
		got, err := storage.GetInstanceCredentials(ctx, instance.ID)
		if err != nil {
			t.Errorf("Failed to get instance credentials: %v", err)
		}

		if got.Username != creds.Username || got.Password != creds.Password {
			t.Errorf("Got credentials %v, want %v", got, creds)
		}
	})

	// Test DeleteInstance
	t.Run("DeleteInstance", func(t *testing.T) {
		if err := storage.DeleteInstance(ctx, instance.ID); err != nil {
			t.Errorf("Failed to delete instance: %v", err)
		}

		// Verify instance is deleted
		_, err := storage.GetInstance(ctx, instance.ID)
		if err == nil {
			t.Error("Expected error when getting deleted instance")
		}
	})
}
