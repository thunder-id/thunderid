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

// Package observability provides observability capabilities for the server including
// event logging and distributed tracing.
package observability

import (
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/observability/publisher"
	"github.com/thunder-id/thunderid/internal/system/observability/subscriber"
)

const loggerComponentName = "ObservabilityService"

// Service provides observability event publishing functionality.
// This is the main entry point for the observability system.
// It manages the lifecycle of the publisher, which in turn manages subscribers.
//
// Architecture:
//   - Service (High-level): Manages lifecycle, configuration, subscriber registration
//   - CategoryPublisher (Low-level): Implements event publishing logic
//   - Subscribers (Low-level): Consume events, self-register, and self-configure
//
// The service implements ObservabilityServiceInterface and is created via Initialize().
// Subscribers register themselves via the registry pattern during package initialization.
type Service struct {
	publisher publisher.CategoryPublisherInterface
	logger    *log.Logger
	config    config.ObservabilityConfig
}

// Ensure Service implements ObservabilityServiceInterface
var _ ObservabilityServiceInterface = (*Service)(nil)

// newServiceWithConfig creates and initializes a new observability service.
func newServiceWithConfig() *Service {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	// Check if observability is disabled

	logger.Debug("Initializing observability service")
	config := config.GetServerRuntime().Config.Observability
	if !config.Enabled {
		logger.Debug("Observability is disabled in configuration")
		return &Service{
			logger: logger,
			config: config,
		}
	}

	// Create event bus (no queue needed)
	pub := publisher.NewCategoryPublisher()

	svc := &Service{
		publisher: pub,
		logger:    logger,
		config:    config,
	}

	// Initialize and register all subscribers using the registry pattern
	subscribers, err := subscriber.Initialize(config)
	if err != nil {
		// This only happens in strict mode
		logger.Error("Failed to initialize subscribers in strict mode", log.Error(err))
		// In strict mode, we could return an error service, but for now we continue
		// with no subscribers (observability will be disabled)
		return svc
	}

	// Subscribe all initialized subscribers to the publisher
	for _, sub := range subscribers {
		pub.Subscribe(sub)
		logger.Debug("Subscriber subscribed to publisher",
			log.String("subscriberID", sub.GetID()))
	}

	logger.Debug("Observability service initialized successfully",
		log.Int("activeSubscribers", len(subscribers)))
	return svc
}

// RegisterSubscriber allows subscribers to self-register with the service.
// This is called by subscribers in their init() functions.
// The subscriber will only be activated if IsEnabled returns true and Initialize succeeds.
func (s *Service) RegisterSubscriber(sub subscriber.SubscriberInterface) {
	// Skip registration if service is not enabled or has no publisher
	if !s.config.Enabled || s.publisher == nil {
		s.logger.Debug("Subscriber registration skipped - service not enabled",
			log.String("subscriberType", fmt.Sprintf("%T", sub)))
		return
	}

	// Check if subscriber is enabled in config
	if !sub.IsEnabled() {
		s.logger.Debug("Subscriber registration skipped - disabled by config",
			log.String("subscriberType", fmt.Sprintf("%T", sub)))
		return
	}

	// Initialize the subscriber
	if err := sub.Initialize(); err != nil {
		if s.config.FailureMode == "strict" {
			s.logger.Error("Failed to initialize subscriber in strict mode",
				log.String("subscriberType", fmt.Sprintf("%T", sub)),
				log.Error(err))
			// In strict mode, we could panic or store error for later retrieval
		} else {
			s.logger.Warn("Failed to initialize subscriber, skipping",
				log.String("subscriberType", fmt.Sprintf("%T", sub)),
				log.Error(err))
		}
		return
	}

	// Subscribe to publisher (publisher now owns the subscriber list)
	s.publisher.Subscribe(sub)

	s.logger.Debug("Subscriber registered and activated successfully",
		log.String("subscriberType", fmt.Sprintf("%T", sub)),
		log.String("subscriberID", sub.GetID()))
}

// PublishEvent publishes an event to the observability system.
// This is a no-op if observability is disabled.
// The publisher will validate the event, so no need to check here.
func (s *Service) PublishEvent(evt *event.Event) {
	// Quick exit if observability is disabled
	if !s.config.Enabled || s.publisher == nil {
		return
	}

	// Publisher handles nil check and validation
	s.publisher.Publish(evt)

	// Only log if event is not nil (avoid panic on evt.Type access)
	if evt != nil {
		s.logger.Debug("Event published",
			log.String("eventType", evt.Type),
			log.String("eventID", evt.EventID),
			log.String("traceID", evt.TraceID))
	}
}

// IsEnabled returns true if observability is enabled and operational.
func (s *Service) IsEnabled() bool {
	return s.config.Enabled && s.publisher != nil
}

// GetConfig returns the current configuration.
func (s *Service) GetConfig() *config.ObservabilityConfig {
	return &s.config
}

// GetPublisher returns the underlying publisher for advanced use cases.
// Most users should use PublishEvent() instead.
// This is provided for cases where you need direct access to:
// - Subscribe/Unsubscribe subscribers programmatically
// - Query active categories
// Returns nil if observability is disabled.
func (s *Service) GetPublisher() publisher.CategoryPublisherInterface {
	return s.publisher
}

// GetActiveSubscribers returns the list of active subscribers.
// This is useful for testing or querying subscriber state.
// Returns empty slice if no subscribers are active or observability is disabled.
func (s *Service) GetActiveSubscribers() []subscriber.SubscriberInterface {
	// Delegate to publisher (single source of truth)
	if s.publisher == nil {
		return []subscriber.SubscriberInterface{}
	}
	return s.publisher.GetSubscribers()
}

// Shutdown gracefully shuts down the observability service.
// The publisher handles unsubscribing and closing all subscribers.
func (s *Service) Shutdown() {
	s.logger.Debug("Shutting down observability service")

	if s.publisher != nil {
		// Publisher handles all subscriber cleanup (unsubscribe, close)
		// This avoids duplicate Close() calls
		s.publisher.Shutdown()
		// Set publisher to nil to mark service as disabled
		s.publisher = nil
	}
	s.logger.Debug("Observability service shutdown complete")
}
