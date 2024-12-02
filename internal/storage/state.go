package storage

import (
	"context"

	"github.com/pourtorabehsan/sentinel/internal/domain"
)

type Storage interface {
	// Cluster operations
	AddCluster(ctx context.Context, cluster domain.Cluster) error
	GetCluster(ctx context.Context, id string) (*domain.Cluster, error)
	ListClusters(ctx context.Context) ([]domain.Cluster, error)
	DeleteCluster(ctx context.Context, id string) error

	// Instance operations
	AddInstance(ctx context.Context, instance domain.Instance, creds domain.Credentials) error
	GetInstance(ctx context.Context, id string) (*domain.Instance, error)
	ListInstances(ctx context.Context, clusterID string) ([]domain.Instance, error)
	UpdateInstanceHealth(ctx context.Context, id string, health domain.InstanceHealth) error
	DeleteInstance(ctx context.Context, id string) error

	// Health and credentials operations
	GetInstanceHealth(ctx context.Context, id string) (*domain.InstanceHealth, error)
	GetInstanceCredentials(ctx context.Context, id string) (*domain.Credentials, error)
}
