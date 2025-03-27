package event

import (
	"context"
	"sync"
)

// Subscriber represents an event subscriber
type Subscriber interface {
	// ID returns unique identifier of the subscriber
	ID() string

	// Handle processes the event
	Handle(ctx context.Context, event Event) error
}

// EventBus handles the distribution of events to subscribers
type EventBus interface {
	// Publish sends an event to all subscribers
	Publish(ctx context.Context, event Event) error

	// Subscribe registers a subscriber for specific event types
	Subscribe(eventType EventType, subscriber Subscriber) error

	// Unsubscribe removes a subscriber for the given event type
	Unsubscribe(eventType EventType, subscriberID string) error
}

type defaultEventBus struct {
	subscribers map[EventType]map[string]Subscriber
	mu          sync.RWMutex
}

// NewEventBus creates a new instance of the default event bus
func NewEventBus() EventBus {
	return &defaultEventBus{
		subscribers: make(map[EventType]map[string]Subscriber),
	}
}

func (b *defaultEventBus) Publish(ctx context.Context, event Event) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	eventType := EventType(event.Type())
	subscribers := b.subscribers[eventType]

	// Execute subscribers concurrently
	var wg sync.WaitGroup
	errCh := make(chan error, len(subscribers))

	for _, subscriber := range subscribers {
		wg.Add(1)
		go func(s Subscriber) {
			defer wg.Done()
			if err := s.Handle(ctx, event); err != nil {
				errCh <- err
			}
		}(subscriber)
	}

	// Wait for all subscribers to complete
	wg.Wait()
	close(errCh)

	// Return the first error if any
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func (b *defaultEventBus) Subscribe(eventType EventType, subscriber Subscriber) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.subscribers[eventType] == nil {
		b.subscribers[eventType] = make(map[string]Subscriber)
	}

	b.subscribers[eventType][subscriber.ID()] = subscriber
	return nil
}

func (b *defaultEventBus) Unsubscribe(eventType EventType, subscriberID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if subscribers, ok := b.subscribers[eventType]; ok {
		delete(subscribers, subscriberID)
	}
	return nil
}
