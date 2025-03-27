package handler

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/pourtorabehsan/sentinel/internal/core/event"
)

// MockHandler implements Handler interface for testing
type MockHandler struct {
	mu          sync.Mutex
	id          string
	supportType event.EventType
	shouldErr   bool
	handleCount int
}

func NewMockHandler(id string, supportType event.EventType) *MockHandler {
	return &MockHandler{
		id:          id,
		supportType: supportType,
	}
}

func (h *MockHandler) Handle(ctx context.Context, evt event.Event) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.handleCount++
	if h.shouldErr {
		return errors.New("mock handler error")
	}
	return nil
}

func (h *MockHandler) Supports(eventType event.EventType) bool {
	return h.supportType == eventType
}

func TestPipeline(t *testing.T) {
	ctx := context.Background()
	pipeline := NewPipeline()

	t.Run("Single handler success", func(t *testing.T) {
		handler := NewMockHandler("h1", event.MasterFailure)
		pipeline.AddHandler(handler)

		evt := event.NewEvent("test1", event.MasterFailure, nil)
		err := pipeline.Process(ctx, evt)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		if handler.handleCount != 1 {
			t.Errorf("expected handle count 1, got %d", handler.handleCount)
		}
	})

	t.Run("Multiple handlers different types", func(t *testing.T) {
		handler1 := NewMockHandler("h2", event.MasterFailure)
		handler2 := NewMockHandler("h3", event.ReplicationLag)
		pipeline.AddHandler(handler1)
		pipeline.AddHandler(handler2)

		evt := event.NewEvent("test2", event.MasterFailure, nil)
		err := pipeline.Process(ctx, evt)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		if handler1.handleCount != 1 {
			t.Errorf("handler1: expected handle count 1, got %d", handler1.handleCount)
		}
		if handler2.handleCount != 0 {
			t.Errorf("handler2: expected handle count 0, got %d", handler2.handleCount)
		}
	})

	t.Run("Handler error propagation", func(t *testing.T) {
		handler := NewMockHandler("h4", event.TopologyChange)
		handler.shouldErr = true
		pipeline.AddHandler(handler)

		evt := event.NewEvent("test3", event.TopologyChange, nil)
		err := pipeline.Process(ctx, evt)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("Concurrent safety", func(t *testing.T) {
		pipeline := NewPipeline() // Fresh pipeline for this test
		handler := NewMockHandler("h5", event.InstanceRecovery)

		// Add handler and process events concurrently
		done := make(chan bool)
		go func() {
			for i := 0; i < 100; i++ {
				pipeline.AddHandler(handler)
			}
			done <- true
		}()

		go func() {
			evt := event.NewEvent("test4", event.InstanceRecovery, nil)
			for i := 0; i < 100; i++ {
				_ = pipeline.Process(ctx, evt)
			}
			done <- true
		}()

		// Wait for both goroutines to complete
		<-done
		<-done
	})
}
