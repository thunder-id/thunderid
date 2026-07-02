package event

import (
	"testing"
	"time"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

func TestNewEvent(t *testing.T) {
	traceID := "trace-123"
	eventType := string(EventTypeTokenIssuanceStarted)
	component := "TestComponent"

	evt := NewEvent(traceID, eventType, component)

	if evt == nil {
		t.Fatal("NewEvent returned nil")
		return
	}

	if evt.TraceID != traceID {
		t.Errorf("Expected TraceID %s, got %s", traceID, evt.TraceID)
	}

	if evt.Type != eventType {
		t.Errorf("Expected Type %s, got %s", eventType, evt.Type)
	}

	if evt.Component != component {
		t.Errorf("Expected Component %s, got %s", component, evt.Component)
	}

	if evt.EventID == "" {
		t.Error("EventID should not be empty")
	}

	if evt.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}

	if evt.Status != providers.StatusInProgress {
		t.Errorf("Expected Status %s, got %s", providers.StatusInProgress, evt.Status)
	}

	if evt.Data == nil {
		t.Error("Data map should be initialized")
	}
}

func TestEventBuilderPattern(t *testing.T) {
	evt := NewEvent("trace-123", string(EventTypeTokenIssuanceStarted), "test-component")

	result := evt.
		WithStatus(providers.StatusSuccess).
		WithData(DataKey.UserID, "user-456").
		WithData(DataKey.ClientID, "client-789").
		WithData(DataKey.Message, "Authentication completed successfully").
		WithData(DataKey.DurationMs, 500)

	if result != evt {
		t.Error("Builder methods should return the same event instance")
	}

	if evt.Status != providers.StatusSuccess {
		t.Errorf("Expected Status %s, got %s", providers.StatusSuccess, evt.Status)
	}

	if evt.Data["user_id"] != "user-456" {
		t.Errorf("Expected Data[user_id] %s, got %v", "user-456", evt.Data["user_id"])
	}

	if evt.Data["client_id"] != "client-789" {
		t.Errorf("Expected Data[client_id] %s, got %v", "client-789", evt.Data["client_id"])
	}

	if evt.Data["message"] != "Authentication completed successfully" {
		t.Errorf("Expected Data[message] %s, got %v", "Authentication completed successfully", evt.Data["message"])
	}

	if evt.Data["duration_ms"] != 500 {
		t.Errorf("Expected Data[duration_ms] %d, got %v", 500, evt.Data["duration_ms"])
	}
}

func TestEventWithDataMap(t *testing.T) {
	evt := NewEvent("trace-123", "user.created", "UserService")

	data := map[string]interface{}{
		"user_id":    "user-123",
		"email":      "user@example.com",
		"created_at": "2025-10-23T10:00:00Z",
		"roles":      []string{"admin", "user"},
	}

	evt.WithDataMap(data)

	if evt.Data["user_id"] != "user-123" {
		t.Error("WithDataMap should set user_id")
	}

	if evt.Data["email"] != "user@example.com" {
		t.Error("WithDataMap should set email")
	}
}

func TestEventValidate(t *testing.T) {
	tests := []struct {
		name    string
		event   *providers.Event
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid event",
			event: &providers.Event{
				TraceID:   "trace-123",
				EventID:   "event-456",
				Type:      string(EventTypeTokenIssuanceStarted),
				Component: "TestComponent",
				Timestamp: time.Now(),
			},
			wantErr: false,
		},
		{
			name:    "nil event",
			event:   nil,
			wantErr: true,
			errMsg:  "event is nil",
		},
		{
			name: "missing trace ID",
			event: &providers.Event{
				EventID:   "event-456",
				Type:      string(EventTypeTokenIssuanceStarted),
				Component: "TestComponent",
				Timestamp: time.Now(),
			},
			wantErr: true,
			errMsg:  "trace_id is required",
		},
		{
			name: "missing event ID",
			event: &providers.Event{
				TraceID:   "trace-123",
				Type:      string(EventTypeTokenIssuanceStarted),
				Component: "TestComponent",
				Timestamp: time.Now(),
			},
			wantErr: true,
			errMsg:  "event_id is required",
		},
		{
			name: "missing event type",
			event: &providers.Event{
				TraceID:   "trace-123",
				EventID:   "event-456",
				Component: "TestComponent",
				Timestamp: time.Now(),
			},
			wantErr: true,
			errMsg:  "type is required",
		},
		{
			name: "missing component",
			event: &providers.Event{
				TraceID:   "trace-123",
				EventID:   "event-456",
				Type:      string(EventTypeTokenIssuanceStarted),
				Timestamp: time.Now(),
			},
			wantErr: true,
			errMsg:  "component is required",
		},
		{
			name: "missing timestamp",
			event: &providers.Event{
				TraceID:   "trace-123",
				EventID:   "event-456",
				Type:      string(EventTypeTokenIssuanceStarted),
				Component: "TestComponent",
			},
			wantErr: true,
			errMsg:  "timestamp is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("Validate() error message = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestEventDataNilSafety(t *testing.T) {
	evt := &providers.Event{
		TraceID:   "trace-123",
		EventID:   "event-456",
		Type:      string(EventTypeTokenIssuanceStarted),
		Component: "TestComponent",
		Timestamp: time.Now(),
		Data:      nil, // Explicitly nil
	}

	// Should initialize map if nil
	evt.WithData(DataKey.Key, "value")

	if evt.Data == nil {
		t.Error("WithData should initialize nil Data map")
	}

	if evt.Data["key"] != "value" {
		t.Errorf("Expected Data[key] %s, got %v", "value", evt.Data["key"])
	}
}

func TestEventTypeConstants(t *testing.T) {
	// Just verify some key constants exist
	if EventTypeTokenIssuanceStarted == "" {
		t.Error("EventTypeTokenIssuanceStarted should not be empty")
	}

	if EventTypeTokenIssued == "" {
		t.Error("EventTypeTokenIssued should not be empty")
	}

	if EventTypeTokenIssuanceFailed == "" {
		t.Error("EventTypeTokenIssuanceFailed should not be empty")
	}
}

func TestStatusConstants(t *testing.T) {
	// Verify status constants exist
	if providers.StatusSuccess == "" {
		t.Error("StatusSuccess should not be empty")
	}

	if providers.StatusFailure == "" {
		t.Error("StatusFailure should not be empty")
	}

	if providers.StatusInProgress == "" {
		t.Error("StatusInProgress should not be empty")
	}

	if providers.StatusPending == "" {
		t.Error("StatusPending should not be empty")
	}
}
