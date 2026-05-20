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

package observability

import (
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/observability/publisher"
	"github.com/thunder-id/thunderid/internal/system/observability/subscriber"
)

// ObservabilityServiceInterface defines the contract for the observability service.
// This interface enables dependency injection and facilitates testing.
// Services that need observability should accept this interface as a parameter.
type ObservabilityServiceInterface interface {
	// PublishEvent publishes an event to the observability system.
	// This is a no-op if observability is disabled.
	PublishEvent(evt *event.Event)

	// IsEnabled returns true if observability is enabled and operational.
	IsEnabled() bool

	// GetConfig returns the current observability configuration.
	GetConfig() *config.ObservabilityConfig

	// GetPublisher returns the underlying publisher for advanced use cases.
	// Most users should use PublishEvent() instead.
	// Returns nil if observability is disabled.
	GetPublisher() publisher.CategoryPublisherInterface

	// GetActiveSubscribers returns the list of active subscribers.
	// This is useful for testing or querying subscriber state.
	// Returns empty slice if no subscribers are active or observability is disabled.
	GetActiveSubscribers() []subscriber.SubscriberInterface

	// Shutdown gracefully shuts down the observability service.
	// The publisher handles unsubscribing and closing all subscribers.
	Shutdown()
}
