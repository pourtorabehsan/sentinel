package monitor

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/pourtorabehsan/sentinel/internal/domain"
	"github.com/pourtorabehsan/sentinel/internal/storage/memory"
)

type mockDB struct {
	db   *sql.DB
	mock sqlmock.Sqlmock
}

func setupMockDB(t *testing.T) *mockDB {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}

	return &mockDB{
		db:   db,
		mock: mock,
	}
}

func setupTestMonitor(t *testing.T) (*Monitor, *memory.MemoryStorage, string, *mockDB) {
	storage := memory.NewMemoryStorage()
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

	// Create test instance
	instance := domain.Instance{
		ID:        uuid.New().String(),
		Host:      "test.local",
		Port:      3306,
		Role:      domain.RolePrimary,
		ClusterID: cluster.ID,
	}
	creds := domain.Credentials{
		Username: "test",
		Password: "password",
	}
	if err := storage.AddInstance(ctx, instance, creds); err != nil {
		t.Fatalf("Failed to create test instance: %v", err)
	}

	mockDB := setupMockDB(t)

	monitor := New(storage, 5*time.Second, 3, time.Second)

	return monitor, storage, instance.ID, mockDB
}

func TestMonitor_PerformHealthCheck(t *testing.T) {
	monitor, _, _, mockDB := setupTestMonitor(t)
	defer mockDB.db.Close()

	tests := []struct {
		name        string
		setupMock   func(sqlmock.Sqlmock)
		expectAlive bool
		expectLag   int64
	}{
		{
			name: "healthy primary",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectPing().WillReturnError(nil)
				mock.ExpectQuery("SELECT @@read_only").WillReturnRows(sqlmock.NewRows([]string{"@@read_only"}).AddRow("0"))
				mock.ExpectQuery("SELECT.*SECONDS_BEHIND_MASTER").WillReturnRows(sqlmock.NewRows([]string{"Seconds_Behind_Master"}).AddRow(0))
			},
			expectAlive: true,
			expectLag:   0,
		},
		{
			name: "healthy replica with lag",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectPing().WillReturnError(nil)
				mock.ExpectQuery("SELECT @@read_only").WillReturnRows(sqlmock.NewRows([]string{"@@read_only"}).AddRow("1"))
				mock.ExpectQuery("SELECT.*SECONDS_BEHIND_MASTER").WillReturnRows(sqlmock.NewRows([]string{"Seconds_Behind_Master"}).AddRow(5))
			},
			expectAlive: true,
			expectLag:   5,
		},
		{
			name: "unreachable instance",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectPing().WillReturnError(sql.ErrConnDone)
			},
			expectAlive: false,
			expectLag:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock(mockDB.mock)

			health := domain.InstanceHealth{}
			err := monitor.performHealthCheck(context.Background(), mockDB.db, &health)

			if tt.expectAlive {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
				if !health.IsAlive {
					t.Error("Expected instance to be alive")
				}
				if health.ReplicationLag != tt.expectLag {
					t.Errorf("Got replication lag %d, want %d", health.ReplicationLag, tt.expectLag)
				}
			} else {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if health.IsAlive {
					t.Error("Expected instance to be dead")
				}
			}

			if err := mockDB.mock.ExpectationsWereMet(); err != nil {
				t.Errorf("There were unfulfilled expectations: %v", err)
			}
		})
	}
}
