package event

import (
	"time"
)

// BaseEvent provides a basic implementation of the Event interface
type BaseEvent struct {
	id        string
	eventType EventType
	timestamp time.Time
	payload   interface{}
}

// NewEvent creates a new BaseEvent
func NewEvent(id string, eventType EventType, payload interface{}) *BaseEvent {
	return &BaseEvent{
		id:        id,
		eventType: eventType,
		timestamp: time.Now(),
		payload:   payload,
	}
}

func (e *BaseEvent) ID() string {
	return e.id
}

func (e *BaseEvent) Type() string {
	return string(e.eventType)
}

func (e *BaseEvent) Timestamp() time.Time {
	return e.timestamp
}

func (e *BaseEvent) Payload() interface{} {
	return e.payload
}
