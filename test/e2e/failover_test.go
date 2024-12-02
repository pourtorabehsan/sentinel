package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pourtorabehsan/sentinel/internal/api"
	"github.com/pourtorabehsan/sentinel/internal/domain"
	"github.com/pourtorabehsan/sentinel/internal/failover"
	"github.com/pourtorabehsan/sentinel/internal/monitor"
	"github.com/pourtorabehsan/sentinel/internal/storage/memory"
)

const (
	sandboxDir   = "/Users/ehsanpourtorab/sandboxes"
	mysqlVersion = "8.0.33"
	basePort     = 5700
	replicaCount = 2
)

type testEnvironment struct {
	storage      *memory.MemoryStorage
	monitor      *monitor.Monitor
	orchestrator *failover.Orchestrator
	handler      *api.Handler
	cleanup      func()
}

func setupMySQLCluster(t *testing.T) (primaryPort int, replicaPorts []int, cleanup func()) {
	// Check if dbdeployer is installed
	if _, err := exec.LookPath("dbdeployer"); err != nil {
		t.Fatal("dbdeployer is not installed. Please install it first.")
	}

	// Create sandbox directory if not exists
	if _, err := os.Stat(sandboxDir); os.IsNotExist(err) {
		cmd := exec.Command("dbdeployer", "init")
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to initialize deployer: %v", err)
		}
	}

	// Deploy MySQL replication topology with force
	cmd := exec.Command("dbdeployer", "deploy", "replication",
		"--base-port", fmt.Sprintf("%d", basePort),
		"--nodes", fmt.Sprintf("%d", replicaCount+1),
		"--force",
		mysqlVersion,
	)
	// if err := cmd.Run(); err != nil {
	// 	t.Fatalf("Failed to deploy MySQL cluster: %v", err)
	// }

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to deploy MySQL cluster: %v\nOutput: %s", err, string(output))
	}

	primaryPort = basePort
	replicaPorts = make([]int, replicaCount)
	for i := 0; i < replicaCount; i++ {
		replicaPorts[i] = basePort + i + 1
	}

	cleanup = func() {
		cmd := exec.Command("dbdeployer", "delete", fmt.Sprintf("rsandbox_%s", mysqlVersion))
		if err := cmd.Run(); err != nil {
			t.Logf("Failed to cleanup MySQL cluster: %v", err)
		}
	}

	// Wait for MySQL to start
	time.Sleep(5 * time.Second)
	return
}

func setupTestEnvironment(t *testing.T) *testEnvironment {
	primaryPort, replicaPorts, mysqlCleanup := setupMySQLCluster(t)

	storage := memory.NewMemoryStorage()
	ctx := context.Background()

	// Create test cluster
	cluster := domain.Cluster{
		ID:          uuid.New().String(),
		Name:        "test-cluster",
		Description: "E2E test cluster",
		CreatedAt:   time.Now(),
	}
	if err := storage.AddCluster(ctx, cluster); err != nil {
		t.Fatalf("Failed to create cluster: %v", err)
	}

	// Add primary instance
	primary := domain.Instance{
		ID:        uuid.New().String(),
		Host:      "127.0.0.1",
		Port:      primaryPort,
		Role:      domain.RolePrimary,
		ClusterID: cluster.ID,
	}
	primaryCreds := domain.Credentials{
		Username: "root",
		Password: "sandbox",
	}
	if err := storage.AddInstance(ctx, primary, primaryCreds); err != nil {
		t.Fatalf("Failed to add primary instance: %v", err)
	}

	// Add replica instances
	for i, port := range replicaPorts {
		replica := domain.Instance{
			ID:        uuid.New().String(),
			Host:      "127.0.0.1",
			Port:      port,
			Role:      domain.RoleReplica,
			ClusterID: cluster.ID,
		}
		replicaCreds := domain.Credentials{
			Username: "root",
			Password: "sandbox",
		}
		if err := storage.AddInstance(ctx, replica, replicaCreds); err != nil {
			t.Fatalf("Failed to add replica instance %d: %v", i, err)
		}
	}

	// Initialize components
	mon := monitor.New(storage, 1*time.Second, 3, 500*time.Millisecond)
	orchestrator := failover.NewOrchestrator(storage, 5*time.Second, 2*time.Second, true)
	handler := api.NewHandler(storage)

	return &testEnvironment{
		storage:      storage,
		monitor:      mon,
		orchestrator: orchestrator,
		handler:      handler,
		cleanup:      mysqlCleanup,
	}
}

func TestFailover(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start monitoring
	go env.monitor.Start(ctx)

	// Wait for initial health checks
	time.Sleep(2 * time.Second)

	// Get primary instance
	clusters, err := env.storage.ListClusters(ctx)
	if err != nil {
		t.Fatalf("Failed to list clusters: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatal("Expected one cluster")
	}

	instances, err := env.storage.ListInstances(ctx, clusters[0].ID)
	if err != nil {
		t.Fatalf("Failed to list instances: %v", err)
	}

	var primary domain.Instance
	for _, instance := range instances {
		if instance.Role == domain.RolePrimary {
			primary = instance
			break
		}
	}

	// Verify all instances are healthy
	for _, instance := range instances {
		t.Logf("Checking health for instance %v", instance)
		health, err := env.storage.GetInstanceHealth(ctx, instance.ID)
		t.Logf("Health: %v", health)
		if err != nil {
			t.Errorf("Failed to get health for instance %s: %v", instance.ID, err)
			continue
		}
		if !health.IsAlive {
			t.Errorf("Instance %s is not alive: %v", instance.ID, health.Error)
		}
	}

	// Simulate primary failure by stopping MySQL
	cmd := exec.Command("dbdeployer", "sandboxes", fmt.Sprintf("repl_%s", mysqlVersion), "stop_node", "1")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to stop primary: %v", err)
	}

	// Wait for failover
	time.Sleep(10 * time.Second)

	// Verify failover occurred
	newInstances, err := env.storage.ListInstances(ctx, clusters[0].ID)
	if err != nil {
		t.Fatalf("Failed to list instances after failover: %v", err)
	}

	var newPrimary *domain.Instance
	for _, instance := range newInstances {
		if instance.Role == domain.RolePrimary && instance.ID != primary.ID {
			newPrimary = &instance
			break
		}
	}

	if newPrimary == nil {
		t.Fatal("No new primary found after failover")
	}

	// Verify new primary is healthy
	health, err := env.storage.GetInstanceHealth(ctx, newPrimary.ID)
	if err != nil {
		t.Fatalf("Failed to get new primary health: %v", err)
	}
	if !health.IsAlive {
		t.Errorf("New primary is not alive: %v", health.Error)
	}

	// Verify replicas are following new primary
	for _, instance := range newInstances {
		if instance.Role == domain.RoleReplica {
			health, err := env.storage.GetInstanceHealth(ctx, instance.ID)
			if err != nil {
				t.Errorf("Failed to get replica health: %v", err)
				continue
			}
			if !health.IsAlive {
				t.Errorf("Replica %s is not alive: %v", instance.ID, health.Error)
			}
			if health.ReplicationLag > 10 {
				t.Errorf("Replica %s has high replication lag: %d", instance.ID, health.ReplicationLag)
			}
		}
	}
}
