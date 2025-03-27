package event

import (
	"reflect"
	"testing"
	"time"
)

func TestBaseEvent(t *testing.T) {
	id := "test-event-1"
	eventType := MasterFailure
	payload := map[string]string{"key": "value"}

	event := NewEvent(id, eventType, payload)

	t.Run("ID should match", func(t *testing.T) {
		if event.ID() != id {
			t.Errorf("expected ID %s, got %s", id, event.ID())
		}
	})

	t.Run("Type should match", func(t *testing.T) {
		if event.Type() != string(eventType) {
			t.Errorf("expected Type %s, got %s", eventType, event.Type())
		}
	})

	t.Run("Timestamp should be recent", func(t *testing.T) {
		now := time.Now()
		eventTime := event.Timestamp()
		if eventTime.After(now) || eventTime.Before(now.Add(-time.Second)) {
			t.Errorf("event timestamp %v is not within expected range", eventTime)
		}
	})

	t.Run("Payload should match", func(t *testing.T) {
		eventPayload := event.Payload()
		if !reflect.DeepEqual(eventPayload, payload) {
			t.Errorf("expected payload %v, got %v", payload, eventPayload)
		}
	})
}
