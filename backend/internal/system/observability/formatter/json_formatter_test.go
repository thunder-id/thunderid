/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package formatter

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/internal/system/observability/event"
)

func TestNewJSONFormatter(t *testing.T) {
	f := newJSONFormatter()
	if f == nil {
		t.Fatal("NewJSONFormatter() returned nil")
	}

	// Verify it implements the Formatter interface
	var _ FormatterInterface = f
}

func TestJSONFormatter_GetName(t *testing.T) {
	f := newJSONFormatter()
	name := f.GetName()

	if name == "" {
		t.Error("GetName() returned empty string")
	}

	if name != "JSONFormatter" {
		t.Errorf("GetName() = %s, want JSONFormatter", name)
	}
}

func TestJSONFormatter_Format(t *testing.T) {
	f := newJSONFormatter()

	timestamp := time.Date(2025, 11, 3, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		event   *event.Event
		wantErr bool
	}{
		{
			name: "simple event",
			event: &event.Event{
				TraceID:   "trace-123",
				EventID:   "event-456",
				Type:      string(event.EventTypeTokenIssuanceStarted),
				Component: "AuthHandler",
				Timestamp: timestamp,
				Status:    event.StatusInProgress,
				Data:      make(map[string]interface{}),
			},
			wantErr: false,
		},
		{
			name: "event with data",
			event: &event.Event{
				TraceID:   "trace-123",
				EventID:   "event-456",
				Type:      string(event.EventTypeFlowStarted),
				Component: "TokenHandler",
				Timestamp: timestamp,
				Status:    event.StatusSuccess,
				Data: map[string]interface{}{
					"user_id":     "user-789",
					"client_id":   "client-abc",
					"duration_ms": 100,
					"scopes":      []string{"openid", "profile"},
				},
			},
			wantErr: false,
		},
		{
			name: "event with nil data",
			event: &event.Event{
				TraceID:   "trace-123",
				EventID:   "event-456",
				Type:      "test.event",
				Component: "TestComponent",
				Timestamp: timestamp,
				Status:    event.StatusSuccess,
				Data:      nil,
			},
			wantErr: false,
		},
		{
			name: "event with nested data",
			event: &event.Event{
				TraceID:   "trace-123",
				EventID:   "event-456",
				Type:      "test.event",
				Component: "TestComponent",
				Timestamp: timestamp,
				Status:    event.StatusSuccess,
				Data: map[string]interface{}{
					"user": map[string]interface{}{
						"id":    "user-123",
						"email": "test@example.com",
					},
					"metadata": map[string]interface{}{
						"version": "1.0",
						"source":  "api",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := f.Format(tt.event)

			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(data) == 0 {
					t.Error("Format() returned empty data")
				}

				// Verify it's valid JSON by unmarshaling
				var result map[string]interface{}
				if err := json.Unmarshal(data, &result); err != nil {
					t.Errorf("Format() produced invalid JSON: %v", err)
				}

				// Verify key fields are present (JSON uses snake_case)
				if result["trace_id"] != tt.event.TraceID {
					t.Errorf("trace_id = %v, want %v", result["trace_id"], tt.event.TraceID)
				}
				if result["event_id"] != tt.event.EventID {
					t.Errorf("event_id = %v, want %v", result["event_id"], tt.event.EventID)
				}
				if result["type"] != tt.event.Type {
					t.Errorf("type = %v, want %v", result["type"], tt.event.Type)
				}
				if result["component"] != tt.event.Component {
					t.Errorf("component = %v, want %v", result["component"], tt.event.Component)
				}
				if result["status"] != tt.event.Status {
					t.Errorf("status = %v, want %v", result["status"], tt.event.Status)
				}
			}
		})
	}
}

func TestJSONFormatter_FormatMultipleEvents(t *testing.T) {
	f := newJSONFormatter()

	timestamp := time.Date(2025, 11, 3, 10, 0, 0, 0, time.UTC)

	events := []*event.Event{
		{
			TraceID:   "trace-1",
			EventID:   "event-1",
			Type:      string(event.EventTypeTokenIssuanceStarted),
			Component: "test",
			Timestamp: timestamp,
			Status:    event.StatusInProgress,
			Data:      nil, // nil data map
		},
		{
			TraceID:   "trace-1",
			EventID:   "event-2",
			Type:      string(event.EventTypeTokenIssued),
			Component: "AuthHandler",
			Timestamp: timestamp.Add(time.Second),
			Status:    event.StatusSuccess,
			Data:      map[string]interface{}{"user_id": "user-123"},
		},
		{
			TraceID:   "trace-1",
			EventID:   "event-3",
			Type:      string(event.EventTypeFlowStarted),
			Component: "TokenHandler",
			Timestamp: timestamp.Add(2 * time.Second),
			Status:    event.StatusSuccess,
			Data:      map[string]interface{}{"token_type": "Bearer"},
		},
	}

	for i, evt := range events {
		data, err := f.Format(evt)
		if err != nil {
			t.Errorf("Format() event %d error = %v", i, err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Errorf("Format() event %d produced invalid JSON: %v", i, err)
		}
	}
}

func TestJSONFormatter_FormatPreservesDataTypes(t *testing.T) {
	f := newJSONFormatter()

	timestamp := time.Date(2025, 11, 3, 10, 0, 0, 0, time.UTC)

	evt := &event.Event{
		TraceID:   "trace-123",
		EventID:   "event-456",
		Type:      "test.event",
		Component: "TestComponent",
		Timestamp: timestamp,
		Status:    event.StatusSuccess,
		Data: map[string]interface{}{
			"string_value": "test",
			"int_value":    42,
			"float_value":  3.14,
			"bool_value":   true,
			"null_value":   nil,
			"array_value":  []interface{}{"a", "b", "c"},
		},
	}

	data, err := f.Format(evt)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify data types are preserved (JSON field is "data" lowercase)
	eventData, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data field is not a map[string]interface{}, got type %T", result["data"])
	}

	if eventData["string_value"] != "test" {
		t.Errorf("string_value = %v, want test", eventData["string_value"])
	}

	if eventData["int_value"].(float64) != 42 {
		t.Errorf("int_value = %v, want 42", eventData["int_value"])
	}

	if eventData["float_value"].(float64) != 3.14 {
		t.Errorf("float_value = %v, want 3.14", eventData["float_value"])
	}

	if eventData["bool_value"] != true {
		t.Errorf("bool_value = %v, want true", eventData["bool_value"])
	}

	if eventData["null_value"] != nil {
		t.Errorf("null_value = %v, want nil", eventData["null_value"])
	}
}

func TestJSONFormatter_FormatEmptyEvent(t *testing.T) {
	f := newJSONFormatter()

	evt := &event.Event{}
	data, err := f.Format(evt)

	if err != nil {
		t.Errorf("Format() error = %v, want nil", err)
	}

	if len(data) == 0 {
		t.Error("Format() returned empty data for empty event")
	}

	// Should still be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Errorf("Format() produced invalid JSON for empty event: %v", err)
	}
}

func BenchmarkJSONFormatter_Format(b *testing.B) {
	f := newJSONFormatter()

	timestamp := time.Now()
	evt := &event.Event{
		TraceID:   "trace-123",
		EventID:   "event-456",
		Type:      string(event.EventTypeTokenIssuanceStarted),
		Component: "AuthHandler",
		Timestamp: timestamp,
		Status:    event.StatusSuccess,
		Data: map[string]interface{}{
			"user_id":   "user-789",
			"client_id": "client-abc",
			"scopes":    []string{"openid", "profile", "email"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = f.Format(evt)
	}
}

func BenchmarkJSONFormatter_FormatLargeData(b *testing.B) {
	f := newJSONFormatter()

	timestamp := time.Now()

	// Create event with large data payload
	largeData := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		largeData[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_%d", i)
	}

	evt := &event.Event{
		TraceID:   "trace-123",
		EventID:   "event-456",
		Type:      string(event.EventTypeTokenIssuanceStarted),
		Component: "AuthHandler",
		Timestamp: timestamp,
		Status:    event.StatusSuccess,
		Data:      largeData,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = f.Format(evt)
	}
}
