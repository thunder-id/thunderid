/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

// Package event provides event models and types for the observability system.
package event

import (
	"fmt"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// EventCategory represents a category for grouping related events.
// Used by the event bus for efficient routing - subscribers declare which
// categories they're interested in, and events are only delivered to
// subscribers that match the event's category.
//
// Benefits:
// - Performance: Skip events with no interested subscribers
// - Flexibility: Subscribers can filter by category, event type, or both
// - Organization: Logical grouping of related events
//
// Example usage:
//
//	subscriber.GetCategories() returns [CategoryAuthentication, CategoryTokens]
//	Event with category "authentication" → only delivered to auth subscribers
type EventCategory string

const (
	// CategoryAuthentication groups all authentication-related events.
	CategoryAuthentication EventCategory = "observability.authentication"

	// CategoryAuthorization groups all authorization-related events.
	CategoryAuthorization EventCategory = "observability.authorization"

	// CategoryFlows groups all flow orchestration events for tracing end-to-end flows.
	CategoryFlows EventCategory = "observability.flows"

	// CategoryAll is a special category that matches all events.
	// Subscribers to this category receive all events regardless of type.
	CategoryAll EventCategory = "observability.all"
)

// UnmappedEventTypeError represents an error when an event type is not mapped to a category.
type UnmappedEventTypeError struct {
	EventType providers.EventType
}

func (e *UnmappedEventTypeError) Error() string {
	return fmt.Sprintf(
		"event type not mapped to category: %s - all event types must be explicitly mapped",
		string(e.EventType),
	)
}

// eventTypeToCategory maps each event type to its category.
// This enables automatic routing of events to appropriate categories.
var eventTypeToCategory = map[providers.EventType]EventCategory{
	// Authentication events
	EventTypeTokenIssuanceStarted:           CategoryAuthentication,
	EventTypeTokenIssued:                    CategoryAuthentication,
	EventTypeTokenIssuanceFailed:            CategoryAuthentication,
	EventTypeTokenRevoked:                   CategoryAuthentication,
	EventTypeRuntimePersistentDBUnavailable: CategoryAuthentication,

	// Flow events
	EventTypeFlowStarted:                CategoryFlows,
	EventTypeFlowNodeExecutionStarted:   CategoryFlows,
	EventTypeFlowNodeExecutionCompleted: CategoryFlows,
	EventTypeFlowNodeExecutionFailed:    CategoryFlows,
	EventTypeFlowUserInputRequired:      CategoryFlows,
	EventTypeFlowCompleted:              CategoryFlows,
	EventTypeFlowFailed:                 CategoryFlows,
}

// GetCategory returns the category for a given event type.
// If the event type is not mapped, it returns an error to prevent unintentional use
// of unmapped event types. All event types must be explicitly mapped to categories.
func GetCategory(eventType providers.EventType) (EventCategory, error) {
	if category, exists := eventTypeToCategory[eventType]; exists {
		return category, nil
	}
	// Return error for unmapped event types
	return "", &UnmappedEventTypeError{EventType: eventType}
}

// GetAllCategories returns all defined event categories (excluding CategoryAll).
func GetAllCategories() []EventCategory {
	return []EventCategory{
		CategoryAuthentication,
		CategoryAuthorization,
		CategoryFlows,
	}
}

// IsValidCategory checks if a category is valid.
func IsValidCategory(category EventCategory) bool {
	if category == CategoryAll {
		return true
	}

	for _, validCategory := range GetAllCategories() {
		if category == validCategory {
			return true
		}
	}

	return false
}
