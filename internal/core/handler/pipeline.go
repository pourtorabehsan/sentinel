package handler

import (
	"context"
	"fmt"
	"sync"

	"github.com/pourtorabehsan/sentinel/internal/core/event"
)

// defaultPipeline implements the Pipeline interface
type defaultPipeline struct {
	handlers []Handler
	mu       sync.RWMutex
}

// NewPipeline creates a new handler pipeline
func NewPipeline() Pipeline {
	return &defaultPipeline{
		handlers: make([]Handler, 0),
	}
}

func (p *defaultPipeline) AddHandler(handler Handler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers = append(p.handlers, handler)
}

func (p *defaultPipeline) Process(ctx context.Context, evt event.Event) error {
	p.mu.RLock()
	handlers := make([]Handler, len(p.handlers))
	copy(handlers, p.handlers) // Create a copy to avoid holding the lock during processing
	p.mu.RUnlock()

	eventType := event.EventType(evt.Type())
	var errs []error

	// Process through all supporting handlers
	for _, h := range handlers {
		if !h.Supports(eventType) {
			continue
		}

		if err := h.Handle(ctx, evt); err != nil {
			errs = append(errs, fmt.Errorf("handler error: %w", err))
		}
	}

	// If any handlers failed, return a combined error
	if len(errs) > 0 {
		return fmt.Errorf("pipeline errors: %v", errs)
	}

	return nil
}
