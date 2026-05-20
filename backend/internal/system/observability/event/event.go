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

// Package event provides event models and types for the analytics system.
package event

import (
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/system/utils"
)

// Event represents a generic analytics or audit event in the system.
// This is a minimal, generic structure that can represent any type of event.
// Event-specific data should be stored in the Data map.
type Event struct {
	// TraceID is the correlation ID for tracking related events across the system.
	TraceID string `json:"trace_id"`

	// EventID is the unique identifier for this specific event.
	EventID string `json:"event_id"`

	// Type indicates the type/name of the event (e.g., "user.created", "order.completed").
	Type string `json:"type"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// Component is the source component/service that generated the event.
	Component string `json:"component"`

	// Status indicates the outcome of the event (e.g., "success", "failure", "in_progress").
	Status string `json:"status"`

	// Data contains event-specific structured data.
	// Use this to store any additional information relevant to the event type.
	// Examples:
	//   - user_id, client_id, session_id
	//   - error details, duration, IP address
	//   - business-specific fields
	Data map[string]interface{} `json:"data,omitempty"`
}

// EventType is a type alias for event type strings.
// This allows for type-safe event type constants while keeping the Event struct generic.
type EventType string

// Common status values (use these for consistency, but not enforced)
const (
	StatusSuccess    = "success"
	StatusFailure    = "failure"
	StatusInProgress = "in_progress"
	StatusPending    = "pending"
)

// NewEvent creates a new Event with required fields.
// Additional data should be added using WithData().
func NewEvent(traceID string, eventType string, component string) *Event {
	eventID, err := utils.GenerateUUIDv7()
	if err != nil {
		return &Event{}
	}

	return &Event{
		TraceID:   traceID,
		EventID:   eventID,
		Type:      eventType,
		Timestamp: time.Now(),
		Component: component,
		Status:    StatusInProgress,
		Data:      make(map[string]interface{}),
	}
}

// WithStatus sets the status and returns the event for chaining.
func (e *Event) WithStatus(status string) *Event {
	e.Status = status
	return e
}

// WithData sets a data field and returns the event for chaining.
// Use this to add event-specific information like user_id, client_id, error details, etc.
func (e *Event) WithData(key string, value interface{}) *Event {
	if e.Data == nil {
		e.Data = make(map[string]interface{})
	}
	e.Data[key] = value
	return e
}

// WithDataMap sets multiple data fields at once and returns the event for chaining.
func (e *Event) WithDataMap(data map[string]interface{}) *Event {
	if e.Data == nil {
		e.Data = make(map[string]interface{})
	}
	for k, v := range data {
		e.Data[k] = v
	}
	return e
}

// Validate validates the event and returns an error if invalid.
func (e *Event) Validate() error {
	if e == nil {
		return fmt.Errorf("event is nil")
	}

	if e.TraceID == "" {
		return fmt.Errorf("trace_id is required")
	}

	if e.EventID == "" {
		return fmt.Errorf("event_id is required")
	}

	if e.Type == "" {
		return fmt.Errorf("type is required")
	}

	if e.Component == "" {
		return fmt.Errorf("component is required")
	}

	if e.Timestamp.IsZero() {
		return fmt.Errorf("timestamp is required")
	}

	return nil
}
