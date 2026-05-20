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

// Package subscriber provides the subscriber interface and implementations for the analytics system.
package subscriber

import (
	"github.com/thunder-id/thunderid/internal/system/observability/event"
)

// SubscriberInterface is the interface that all event subscribers must implement.
// Subscribers are now responsible for their own activation and configuration.
type SubscriberInterface interface {
	// GetID returns the unique identifier for this subscriber.
	GetID() string

	// GetCategories returns the categories this subscriber is interested in.
	// Return empty slice or slice containing event.CategoryAll to receive all events.
	GetCategories() []event.EventCategory

	// OnEvent is called when a new event is published.
	// Subscribers are responsible for filtering events they don't want to process.
	OnEvent(evt *event.Event) error

	// Close is called during shutdown to allow cleanup.
	Close() error

	// IsEnabled checks if the subscriber should be activated based on configuration.
	// The config parameter will be *observability.Config.
	// Returns true if the subscriber should be initialized and activated.
	IsEnabled() bool

	// Initialize sets up the subscriber with the provided configuration.
	// This is called after IsEnabled returns true.
	// Returns error if initialization fails.
	Initialize() error
}
