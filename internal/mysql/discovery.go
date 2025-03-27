package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// DiscoveryService handles MySQL topology discovery and monitoring
type DiscoveryService struct {
	connMgr *ConnectionManager
	topology *Topology
}

// NewDiscoveryService creates a new discovery service
func NewDiscoveryService(connMgr *ConnectionManager, topology *Topology) *DiscoveryService {
	return &DiscoveryService{
		connMgr:  connMgr,
		topology: topology,
	}
}

// DiscoverInstance probes an instance and returns its current state
func (s *DiscoveryService) DiscoverInstance(ctx context.Context, hostname string, port int) (*Instance, error) {
	instance := &Instance{
		HostName:    hostname,
		Port:        port,
		Role:        RoleUnknown,
		LastSeen:    time.Now(),
		IsReachable: false,
	}

	db, err := s.connMgr.GetConnection(ctx, instance)
	if err != nil {
		instance.LastError = err
		return instance, err
	}

	if err := s.probeInstance(ctx, db, instance); err != nil {
		instance.LastError = err
		return instance, err
	}

	instance.IsReachable = true
	return instance, nil
}

func (s *DiscoveryService) probeInstance(ctx context.Context, db *sql.DB, instance *Instance) error {
	// Get read_only status
	var readOnly sql.NullInt64
	if err := db.QueryRowContext(ctx, "SELECT @@read_only").Scan(&readOnly); err != nil {
		return fmt.Errorf("failed to get read_only status: %w", err)
	}
	instance.ReadOnly = readOnly.Int64 == 1

	// Get replication status
	rows, err := db.QueryContext(ctx, "SHOW SLAVE STATUS")
	if err != nil {
		return fmt.Errorf("failed to get slave status: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		// Parse slave status
		if err := s.parseSlaveStatus(rows, instance); err != nil {
			return err
		}
		instance.Role = RoleSlave
	} else {
		instance.Role = RoleMaster
	}

	return nil
}

func (s *DiscoveryService) parseSlaveStatus(rows *sql.Rows, instance *Instance) error {
	// Implementation will depend on MySQL version and available fields
	// This is a simplified version
	var masterHost, masterPort, ioRunning, sqlRunning sql.NullString
	var secondsBehind sql.NullInt64

	if err := rows.Scan(&masterHost, &masterPort, &ioRunning, &sqlRunning, &secondsBehind); err != nil {
		return fmt.Errorf("failed to parse slave status: %w", err)
	}

	instance.MasterHost = masterHost.String
	instance.SlaveIORunning = ioRunning.String == "Yes"
	instance.SlaveSQLRunning = sqlRunning.String == "Yes"
	instance.SecondsBehind = secondsBehind.Int64
	instance.ReplicationLag = time.Duration(secondsBehind.Int64) * time.Second

	return nil
}
