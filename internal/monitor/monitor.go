package monitor

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pourtorabehsan/sentinel/internal/domain"
	"github.com/pourtorabehsan/sentinel/internal/storage"
)

type Monitor struct {
	storage  storage.Storage
	interval time.Duration
	retry    int
	delay    time.Duration
	done     chan struct{}
}

func New(storage storage.Storage, interval time.Duration, retry int, delay time.Duration) *Monitor {
	return &Monitor{
		storage:  storage,
		interval: interval,
		retry:    retry,
		delay:    delay,
		done:     make(chan struct{}),
	}
}

func (m *Monitor) Start(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkInstances(ctx)
		}
	}
}

func (m *Monitor) Stop() {
	close(m.done)
}

func (m *Monitor) checkInstances(ctx context.Context) {
	clusters, err := m.storage.ListClusters(ctx)
	if err != nil {
		log.Printf("Error listing clusters: %v", err)
		return
	}

	for _, cluster := range clusters {
		instances, err := m.storage.ListInstances(ctx, cluster.ID)
		if err != nil {
			log.Printf("Error listing instances for cluster %s: %v", cluster.ID, err)
			continue
		}

		for _, instance := range instances {
			go m.checkInstance(ctx, instance)
		}
	}
}

func (m *Monitor) checkInstance(ctx context.Context, instance domain.Instance) {
	health := domain.InstanceHealth{
		LastChecked: time.Now(),
	}

	creds, err := m.storage.GetInstanceCredentials(ctx, instance.ID)
	if err != nil {
		health.Error = fmt.Sprintf("Failed to get credentials: %v", err)
		m.updateHealth(ctx, instance.ID, health)
		return
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?timeout=5s",
		creds.Username,
		creds.Password,
		instance.Host,
		instance.Port,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		health.Error = fmt.Sprintf("Failed to create DB connection: %v", err)
		m.updateHealth(ctx, instance.ID, health)
		return
	}
	defer db.Close()

	// Perform health checks with retry
	var lastErr error
	for i := 0; i < m.retry; i++ {
		if err := m.performHealthCheck(ctx, db, &health); err != nil {
			lastErr = err
			time.Sleep(m.delay)
			continue
		}
		lastErr = nil
		break
	}

	if lastErr != nil {
		health.Error = lastErr.Error()
		health.IsAlive = false
	}

	m.updateHealth(ctx, instance.ID, health)
}

func (m *Monitor) performHealthCheck(ctx context.Context, db *sql.DB, health *domain.InstanceHealth) error {
	// Check if server is responding
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping failed: %v", err)
	}

	// Check read/write status
	var (
		readOnly     string
		secondsBehind int64
	)

	// Check if instance is read-only
	err := db.QueryRowContext(ctx, "SELECT @@read_only").Scan(&readOnly)
	if err != nil {
		return fmt.Errorf("failed to check read_only status: %v", err)
	}

	// Check replication lag for replicas
	err = db.QueryRowContext(ctx, `
		SELECT IFNULL(CAST(SECONDS_BEHIND_MASTER AS SIGNED), -1)
		FROM performance_schema.replication_applier_status_by_worker
		ORDER BY CHANNEL_NAME LIMIT 1
	`).Scan(&secondsBehind)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to check replication lag: %v", err)
	}

	health.IsAlive = true
	health.ReplicationLag = secondsBehind
	return nil
}

func (m *Monitor) updateHealth(ctx context.Context, instanceID string, health domain.InstanceHealth) {
	if err := m.storage.UpdateInstanceHealth(ctx, instanceID, health); err != nil {
		log.Printf("Failed to update health for instance %s: %v", instanceID, err)
	}
}
