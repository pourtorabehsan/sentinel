package event

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// MockSubscriber implements Subscriber interface for testing
type MockSubscriber struct {
	id        string
	handleCnt int
	mu        sync.Mutex
	delay     time.Duration
	shouldErr bool
}

func NewMockSubscriber(id string) *MockSubscriber {
	return &MockSubscriber{id: id}
}

func (s *MockSubscriber) ID() string {
	return s.id
}

func (s *MockSubscriber) Handle(ctx context.Context, event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.delay > 0 {
		time.Sleep(s.delay)
	}

	s.handleCnt++
	if s.shouldErr {
		return fmt.Errorf("mock error from subscriber %s", s.id)
	}
	return nil
}

func (s *MockSubscriber) GetHandleCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.handleCnt
}

func TestEventBus(t *testing.T) {
	ctx := context.Background()
	bus := NewEventBus()

	t.Run("Subscribe and publish", func(t *testing.T) {
		sub1 := NewMockSubscriber("sub1")
		sub2 := NewMockSubscriber("sub2")

		// Subscribe both to MasterFailure events
		if err := bus.Subscribe(MasterFailure, sub1); err != nil {
			t.Fatalf("failed to subscribe sub1: %v", err)
		}
		if err := bus.Subscribe(MasterFailure, sub2); err != nil {
			t.Fatalf("failed to subscribe sub2: %v", err)
		}

		// Publish event
		event := NewEvent("test1", MasterFailure, nil)
		if err := bus.Publish(ctx, event); err != nil {
			t.Fatalf("failed to publish event: %v", err)
		}

		// Check both subscribers received the event
		if cnt := sub1.GetHandleCount(); cnt != 1 {
			t.Errorf("sub1: expected handle count 1, got %d", cnt)
		}
		if cnt := sub2.GetHandleCount(); cnt != 1 {
			t.Errorf("sub2: expected handle count 1, got %d", cnt)
		}
	})

	t.Run("Unsubscribe", func(t *testing.T) {
		sub := NewMockSubscriber("sub3")

		// Subscribe and verify
		if err := bus.Subscribe(ReplicationLag, sub); err != nil {
			t.Fatalf("failed to subscribe: %v", err)
		}

		// Publish first event
		event1 := NewEvent("test2", ReplicationLag, nil)
		if err := bus.Publish(ctx, event1); err != nil {
			t.Fatalf("failed to publish first event: %v", err)
		}

		// Unsubscribe
		if err := bus.Unsubscribe(ReplicationLag, sub.ID()); err != nil {
			t.Fatalf("failed to unsubscribe: %v", err)
		}

		// Publish second event
		event2 := NewEvent("test3", ReplicationLag, nil)
		if err := bus.Publish(ctx, event2); err != nil {
			t.Fatalf("failed to publish second event: %v", err)
		}

		// Check only first event was received
		if cnt := sub.GetHandleCount(); cnt != 1 {
			t.Errorf("expected handle count 1, got %d", cnt)
		}
	})

	t.Run("Error handling", func(t *testing.T) {
		sub := NewMockSubscriber("sub4")
		sub.shouldErr = true

		if err := bus.Subscribe(TopologyChange, sub); err != nil {
			t.Fatalf("failed to subscribe: %v", err)
		}

		event := NewEvent("test4", TopologyChange, nil)
		err := bus.Publish(ctx, event)
		if err == nil {
			t.Error("expected error from publish, got nil")
		}
	})

	t.Run("Concurrent subscribers", func(t *testing.T) {
		sub1 := NewMockSubscriber("sub5")
		sub2 := NewMockSubscriber("sub6")
		sub1.delay = 100 * time.Millisecond
		sub2.delay = 50 * time.Millisecond

		if err := bus.Subscribe(InstanceRecovery, sub1); err != nil {
			t.Fatalf("failed to subscribe sub1: %v", err)
		}
		if err := bus.Subscribe(InstanceRecovery, sub2); err != nil {
			t.Fatalf("failed to subscribe sub2: %v", err)
		}

		start := time.Now()
		event := NewEvent("test5", InstanceRecovery, nil)
		if err := bus.Publish(ctx, event); err != nil {
			t.Fatalf("failed to publish event: %v", err)
		}
		duration := time.Since(start)

		// Should take around 100ms, not 150ms
		if duration >= 140*time.Millisecond {
			t.Errorf("concurrent execution took too long: %v", duration)
		}

		if cnt := sub1.GetHandleCount(); cnt != 1 {
			t.Errorf("sub1: expected handle count 1, got %d", cnt)
		}
		if cnt := sub2.GetHandleCount(); cnt != 1 {
			t.Errorf("sub2: expected handle count 1, got %d", cnt)
		}
	})
}
