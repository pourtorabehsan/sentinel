package handler

import (
	"context"
	"github.com/pourtorabehsan/sentinel/internal/core/event"
)

// Handler defines the interface for event handlers
type Handler interface {
	// Handle processes an event
	Handle(ctx context.Context, evt event.Event) error

	// Supports returns true if the handler can process the given event type
	Supports(eventType event.EventType) bool
}

// Pipeline represents a series of handlers that process events in order
type Pipeline interface {
	// AddHandler adds a new handler to the pipeline
	AddHandler(handler Handler)

	// Process runs the event through all supporting handlers
	Process(ctx context.Context, evt event.Event) error
}
