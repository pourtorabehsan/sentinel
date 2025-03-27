package failover

import (
	"context"
	"time"
)

// Step represents a single step in the failover process
type Step interface {
	// Execute performs the step's action
	Execute(ctx context.Context, state *FailoverState) error

	// Rollback reverts the step's action in case of failure
	Rollback(ctx context.Context, state *FailoverState) error
}

// FailoverState holds the current state of the failover process
type FailoverState struct {
	FailedMaster    string
	CandidateMaster string
	Replicas        []string
	StartTime       time.Time
	Status          Status
}

// Status represents the current status of a failover operation
type Status string

const (
	StatusStarted   Status = "started"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)
