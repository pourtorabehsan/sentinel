package event

import "time"

// Event represents a basic event in the system
type Event interface {
	// ID returns the unique identifier of the event
	ID() string

	// Type returns the type of the event
	Type() string

	// Timestamp returns when the event was created
	Timestamp() time.Time

	// Payload returns the event data
	Payload() interface{}
}

// EventType represents different types of events in the system
type EventType string

const (
	// Define common event types
	MasterFailure    EventType = "master_failure"
	ReplicationLag   EventType = "replication_lag"
	TopologyChange   EventType = "topology_change"
	InstanceRecovery EventType = "instance_recovery"
)
