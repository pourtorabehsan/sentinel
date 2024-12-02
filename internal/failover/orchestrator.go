package failover

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/pourtorabehsan/sentinel/internal/domain"
	"github.com/pourtorabehsan/sentinel/internal/storage"
)

type Orchestrator struct {
	storage        storage.Storage
	maxFailureTime time.Duration
	minReplicaLag  time.Duration
	automatic      bool
}

func NewOrchestrator(storage storage.Storage, maxFailureTime, minReplicaLag time.Duration, automatic bool) *Orchestrator {
	return &Orchestrator{
		storage:        storage,
		maxFailureTime: maxFailureTime,
		minReplicaLag:  minReplicaLag,
		automatic:      automatic,
	}
}

// CheckAndFailover monitors primary instances and triggers failover if needed
func (o *Orchestrator) CheckAndFailover(ctx context.Context) error {
	clusters, err := o.storage.ListClusters(ctx)
	if err != nil {
		return fmt.Errorf("failed to list clusters: %w", err)
	}

	for _, cluster := range clusters {
		if err := o.checkCluster(ctx, cluster); err != nil {
			log.Printf("Error checking cluster %s: %v", cluster.ID, err)
		}
	}

	return nil
}

func (o *Orchestrator) checkCluster(ctx context.Context, cluster domain.Cluster) error {
	instances, err := o.storage.ListInstances(ctx, cluster.ID)
	if err != nil {
		return fmt.Errorf("failed to list instances: %w", err)
	}

	var primary *domain.Instance
	var replicas []domain.Instance

	// Separate primary and replicas
	for _, instance := range instances {
		if instance.Role == domain.RolePrimary {
			primary = &instance
		} else {
			replicas = append(replicas, instance)
		}
	}

	if primary == nil {
		return fmt.Errorf("no primary instance found for cluster %s", cluster.ID)
	}

	// Check primary health
	health, err := o.storage.GetInstanceHealth(ctx, primary.ID)
	if err != nil {
		return fmt.Errorf("failed to get primary health: %w", err)
	}

	// Check if failover is needed
	if o.needsFailover(health) {
		if o.automatic {
			return o.performAutomaticFailover(ctx, cluster.ID, *primary, replicas)
		}
		log.Printf("Primary %s needs failover but automatic failover is disabled", primary.ID)
	}

	return nil
}

func (o *Orchestrator) needsFailover(health *domain.InstanceHealth) bool {
	if health.IsAlive {
		return false
	}

	return time.Since(health.LastChecked) > o.maxFailureTime
}

func (o *Orchestrator) performAutomaticFailover(ctx context.Context, clusterID string, primary domain.Instance, replicas []domain.Instance) error {
	if len(replicas) == 0 {
		return fmt.Errorf("no replicas available for failover")
	}

	// Find the best replica for promotion
	newPrimary, err := o.selectNewPrimary(ctx, replicas)
	if err != nil {
		return fmt.Errorf("failed to select new primary: %w", err)
	}

	return o.executePrimaryPromotion(ctx, clusterID, primary, *newPrimary)
}

func (o *Orchestrator) selectNewPrimary(ctx context.Context, replicas []domain.Instance) (*domain.Instance, error) {
	var bestReplica *domain.Instance
	var minLag int64 = -1

	for _, replica := range replicas {
		health, err := o.storage.GetInstanceHealth(ctx, replica.ID)
		if err != nil {
			continue
		}

		if !health.IsAlive {
			continue
		}

		// Skip replicas with too much lag
		if health.ReplicationLag > int64(o.minReplicaLag.Seconds()) {
			continue
		}

		// Select replica with minimum lag
		if minLag == -1 || health.ReplicationLag < minLag {
			minLag = health.ReplicationLag
			bestReplica = &replica
		}
	}

	if bestReplica == nil {
		return nil, fmt.Errorf("no suitable replica found for promotion")
	}

	return bestReplica, nil
}

func (o *Orchestrator) executePrimaryPromotion(ctx context.Context, clusterID string, oldPrimary domain.Instance, newPrimary domain.Instance) error {
	// Get credentials for both instances
	oldPrimaryCreds, err := o.storage.GetInstanceCredentials(ctx, oldPrimary.ID)
	if err != nil {
		return fmt.Errorf("failed to get old primary credentials: %w", err)
	}

	newPrimaryCreds, err := o.storage.GetInstanceCredentials(ctx, newPrimary.ID)
	if err != nil {
		return fmt.Errorf("failed to get new primary credentials: %w", err)
	}

	// Execute promotion steps
	if err := o.promoteReplica(ctx, newPrimary, *newPrimaryCreds); err != nil {
		return fmt.Errorf("failed to promote replica: %w", err)
	}

	// Get all other replicas in the cluster
	instances, err := o.storage.ListInstances(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("failed to list cluster instances: %w", err)
	}

	// Reconfigure other replicas to follow the new primary
	for _, instance := range instances {
		// Skip the old primary and new primary
		if instance.ID == oldPrimary.ID || instance.ID == newPrimary.ID {
			continue
		}

		creds, err := o.storage.GetInstanceCredentials(ctx, instance.ID)
		if err != nil {
			log.Printf("Failed to get credentials for replica %s: %v", instance.ID, err)
			continue
		}

		if err := o.reconfigureReplica(ctx, instance, *creds, newPrimary, *newPrimaryCreds); err != nil {
			log.Printf("Failed to reconfigure replica %s: %v", instance.ID, err)
		}
	}

	// Update roles in storage
	oldPrimary.Role = domain.RoleReplica
	if err := o.storage.DeleteInstance(ctx, oldPrimary.ID); err != nil {
		log.Printf("Failed to delete old primary: %v", err)
	}
	if err := o.storage.AddInstance(ctx, oldPrimary, *oldPrimaryCreds); err != nil {
		log.Printf("Failed to add old primary as replica: %v", err)
	}

	newPrimary.Role = domain.RolePrimary
	if err := o.storage.DeleteInstance(ctx, newPrimary.ID); err != nil {
		return fmt.Errorf("failed to update new primary role: %w", err)
	}
	if err := o.storage.AddInstance(ctx, newPrimary, *newPrimaryCreds); err != nil {
		return fmt.Errorf("failed to update new primary role: %w", err)
	}

	log.Printf("Successfully promoted instance %s to primary for cluster %s", newPrimary.ID, clusterID)
	return nil
}

func (o *Orchestrator) reconfigureReplica(ctx context.Context, replica domain.Instance, replicaCreds domain.Credentials, newPrimary domain.Instance, primaryCreds domain.Credentials) error {
	db, err := connectToMySQL(replica, replicaCreds)
	if err != nil {
		return fmt.Errorf("failed to connect to replica: %w", err)
	}
	defer db.Close()

	// Stop current replication
	if _, err := db.ExecContext(ctx, "STOP SLAVE"); err != nil {
		return fmt.Errorf("failed to stop replication: %w", err)
	}

	// Reset slave configuration
	if _, err := db.ExecContext(ctx, "RESET SLAVE ALL"); err != nil {
		return fmt.Errorf("failed to reset slave configuration: %w", err)
	}

	// Configure replication from new primary
	query := fmt.Sprintf("CHANGE MASTER TO MASTER_HOST='%s', MASTER_PORT=%d, MASTER_USER='%s', MASTER_PASSWORD='%s'",
		newPrimary.Host,
		newPrimary.Port,
		primaryCreds.Username,
		primaryCreds.Password,
	)
	if _, err := db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("failed to change master: %w", err)
	}

	// Start replication
	if _, err := db.ExecContext(ctx, "START SLAVE"); err != nil {
		return fmt.Errorf("failed to start replication: %w", err)
	}

	log.Printf("Successfully reconfigured replica %s to follow new primary %s", replica.ID, newPrimary.ID)
	return nil
}

func (o *Orchestrator) promoteReplica(ctx context.Context, instance domain.Instance, creds domain.Credentials) error {
	// Connect to the replica
	db, err := connectToMySQL(instance, creds)
	if err != nil {
		return fmt.Errorf("failed to connect to replica: %w", err)
	}
	defer db.Close()

	// Stop replication
	if _, err := db.ExecContext(ctx, "STOP SLAVE"); err != nil {
		return fmt.Errorf("failed to stop replication: %w", err)
	}

	// Reset slave configuration completely
	if _, err := db.ExecContext(ctx, "RESET SLAVE ALL"); err != nil {
		return fmt.Errorf("failed to reset slave configuration: %w", err)
	}

	// Reset read_only flag
	if _, err := db.ExecContext(ctx, "SET GLOBAL read_only = OFF"); err != nil {
		return fmt.Errorf("failed to disable read_only: %w", err)
	}

	// Reset super_read_only flag if exists
	if _, err := db.ExecContext(ctx, "SET GLOBAL super_read_only = OFF"); err != nil {
		log.Printf("Note: super_read_only not supported on this instance")
	}

	log.Printf("Successfully reset slave configuration and promoted instance %s", instance.ID)
	return nil
}
