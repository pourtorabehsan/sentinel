package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pourtorabehsan/sentinel/internal/domain"
)

type MemoryStorage struct {
	mu sync.RWMutex

	clusters  map[string]domain.Cluster
	instances map[string]domain.Instance
	health    map[string]domain.InstanceHealth
	creds     map[string]domain.Credentials
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		clusters:  make(map[string]domain.Cluster),
		instances: make(map[string]domain.Instance),
		health:    make(map[string]domain.InstanceHealth),
		creds:     make(map[string]domain.Credentials),
	}
}

// Cluster operations
func (m *MemoryStorage) AddCluster(ctx context.Context, cluster domain.Cluster) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clusters[cluster.ID]; exists {
		return fmt.Errorf("cluster with ID %s already exists", cluster.ID)
	}

	cluster.CreatedAt = time.Now()
	m.clusters[cluster.ID] = cluster
	return nil
}

func (m *MemoryStorage) GetCluster(ctx context.Context, id string) (*domain.Cluster, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cluster, exists := m.clusters[id]
	if !exists {
		return nil, fmt.Errorf("cluster with ID %s not found", id)
	}

	return &cluster, nil
}

func (m *MemoryStorage) ListClusters(ctx context.Context) ([]domain.Cluster, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clusters := make([]domain.Cluster, 0, len(m.clusters))
	for _, cluster := range m.clusters {
		clusters = append(clusters, cluster)
	}

	return clusters, nil
}

func (m *MemoryStorage) DeleteCluster(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clusters[id]; !exists {
		return fmt.Errorf("cluster with ID %s not found", id)
	}

	// Delete all instances belonging to this cluster
	for instanceID, instance := range m.instances {
		if instance.ClusterID == id {
			delete(m.instances, instanceID)
			delete(m.health, instanceID)
			delete(m.creds, instanceID)
		}
	}

	delete(m.clusters, id)
	return nil
}

// Instance operations
func (m *MemoryStorage) AddInstance(ctx context.Context, instance domain.Instance, creds domain.Credentials) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.instances[instance.ID]; exists {
		return fmt.Errorf("instance with ID %s already exists", instance.ID)
	}

	if _, exists := m.clusters[instance.ClusterID]; !exists {
		return fmt.Errorf("cluster with ID %s not found", instance.ClusterID)
	}

	m.instances[instance.ID] = instance
	m.creds[instance.ID] = creds
	m.health[instance.ID] = domain.InstanceHealth{
		IsAlive:     false,
		LastChecked: time.Now(),
	}

	return nil
}

func (m *MemoryStorage) GetInstance(ctx context.Context, id string) (*domain.Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instance, exists := m.instances[id]
	if !exists {
		return nil, fmt.Errorf("instance with ID %s not found", id)
	}

	return &instance, nil
}

func (m *MemoryStorage) ListInstances(ctx context.Context, clusterID string) ([]domain.Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var instances []domain.Instance
	for _, instance := range m.instances {
		if instance.ClusterID == clusterID {
			instances = append(instances, instance)
		}
	}

	return instances, nil
}

func (m *MemoryStorage) UpdateInstanceHealth(ctx context.Context, id string, health domain.InstanceHealth) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.instances[id]; !exists {
		return fmt.Errorf("instance with ID %s not found", id)
	}

	m.health[id] = health
	return nil
}

func (m *MemoryStorage) DeleteInstance(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.instances[id]; !exists {
		return fmt.Errorf("instance with ID %s not found", id)
	}

	delete(m.instances, id)
	delete(m.health, id)
	delete(m.creds, id)
	return nil
}

// Helper methods for credentials
func (m *MemoryStorage) GetInstanceCredentials(ctx context.Context, id string) (*domain.Credentials, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	creds, exists := m.creds[id]
	if !exists {
		return nil, fmt.Errorf("credentials for instance %s not found", id)
	}

	return &creds, nil
}

func (m *MemoryStorage) GetInstanceHealth(ctx context.Context, id string) (*domain.InstanceHealth, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health, exists := m.health[id]
	if !exists {
		return nil, fmt.Errorf("health status for instance %s not found", id)
	}

	return &health, nil
}
