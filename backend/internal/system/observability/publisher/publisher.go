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

// Package publisher provides the event bus mechanism for the observability system.
package publisher

import (
	"context"
	"sync"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/observability/subscriber"
)

// CategoryPublisherInterface is an event bus interface with category-based routing.
// Events are published synchronously to all interested subscribers.
type CategoryPublisherInterface interface {
	// Publish publishes an event immediately to all interested subscribers.
	Publish(event *event.Event)

	// Subscribe adds a subscriber (subscriber decides what categories it wants).
	Subscribe(sub subscriber.SubscriberInterface)

	// Unsubscribe removes a subscriber.
	Unsubscribe(sub subscriber.SubscriberInterface)

	// GetActiveCategories returns categories that have at least one subscriber.
	GetActiveCategories() []event.EventCategory

	// GetSubscribers returns a list of all currently subscribed subscribers.
	// Returns a copy to prevent external modification.
	GetSubscribers() []subscriber.SubscriberInterface

	// Shutdown gracefully shuts down the publisher.
	Shutdown()
}

// categoryEventPublisher implements category-based event bus with smart routing.
// This is a simple synchronous event bus - no queuing, no async processing.
// Events are published directly to subscribers who can decide whether to process them.
type categoryEventPublisher struct {
	subscribers           map[string]subscriber.SubscriberInterface // subscriberID -> subscriber
	subscribersByCategory map[event.EventCategory][]string          // category -> []subscriberIDs
	mu                    sync.RWMutex
	isShutdown            bool
	wg                    sync.WaitGroup // Tracks active goroutines for graceful shutdown
	ctx                   context.Context
	cancel                context.CancelFunc
	logger                *log.Logger // Stored logger to avoid repeated creation
}

const loggerComponentName = "CategoryEventPublisher"

// NewCategoryPublisher creates a new category-based event bus.
func NewCategoryPublisher() CategoryPublisherInterface {
	ctx, cancel := context.WithCancel(context.Background())
	return &categoryEventPublisher{
		subscribers:           make(map[string]subscriber.SubscriberInterface),
		subscribersByCategory: make(map[event.EventCategory][]string),
		isShutdown:            false,
		ctx:                   ctx,
		cancel:                cancel,
		logger:                log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
	}
}

// Publish publishes an event to all interested subscribers.
// This method returns immediately without blocking the caller.
// The entire publishing process (validation, routing, and dispatching) runs asynchronously.
func (p *categoryEventPublisher) Publish(evt *event.Event) {
	if evt == nil {
		return
	}

	// Validate event
	if err := evt.Validate(); err != nil {
		p.logger.Warn("Invalid event, skipping publish", log.Error(err))
		return
	}

	p.mu.RLock()
	if p.isShutdown {
		p.mu.RUnlock()
		p.logger.Warn("Attempted to publish event after shutdown",
			log.String("eventType", evt.Type))
		return
	}
	p.mu.RUnlock()

	// Increment WaitGroup for the dispatcher goroutine
	p.wg.Add(1)

	// Process event asynchronously to avoid blocking the caller
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				p.logger.Error("Panic in event publisher dispatcher",
					log.String("eventType", evt.Type),
					log.Any("panic", r))
			}
		}()

		// Get event category
		category, err := evt.GetCategory()
		if err != nil {
			p.logger.Error("Failed to get event category, skipping publish",
				log.String("eventType", evt.Type),
				log.Error(err))
			return
		}

		p.mu.RLock()
		// Smart Publishing: Check if anyone cares about this category
		hasSubscribers := false
		if subscribers, exists := p.subscribersByCategory[category]; exists && len(subscribers) > 0 {
			hasSubscribers = true
		}
		// Also check for CategoryAll subscribers
		if allSubscribers, exists := p.subscribersByCategory[event.CategoryAll]; exists && len(allSubscribers) > 0 {
			hasSubscribers = true
		}

		if !hasSubscribers {
			// No subscribers for this category - skip it!
			p.mu.RUnlock()
			if p.logger.IsDebugEnabled() {
				p.logger.Debug("No subscribers for event category, skipping",
					log.String("eventType", evt.Type),
					log.String("category", string(category)))
			}
			return
		}

		// Get ALL subscribers (they will filter themselves)
		allSubscribers := make([]subscriber.SubscriberInterface, 0, len(p.subscribers))
		for _, sub := range p.subscribers {
			allSubscribers = append(allSubscribers, sub)
		}
		p.mu.RUnlock()

		// Broadcast to all subscribers asynchronously
		// Each subscriber runs in its own goroutine to avoid blocking
		for _, sub := range allSubscribers {
			// Increment WaitGroup before spawning goroutine
			p.wg.Add(1)
			go func(s subscriber.SubscriberInterface) {
				defer p.wg.Done() // Ensure WaitGroup is decremented when goroutine completes
				defer func() {
					if r := recover(); r != nil {
						p.logger.Error("Subscriber panicked while handling event",
							log.String("subscriberID", s.GetID()),
							log.Any("panic", r))
					}
				}()

				// Subscriber will filter the event itself based on its interests
				if err := s.OnEvent(evt); err != nil {
					p.logger.Error("Subscriber failed to handle event",
						log.String("subscriberID", s.GetID()),
						log.String("eventType", evt.Type),
						log.Error(err))
				}
			}(sub)
		}
	}()
}

// Subscribe adds a subscriber.
// The subscriber's GetCategories() method determines what categories it receives.
func (p *categoryEventPublisher) Subscribe(sub subscriber.SubscriberInterface) {
	if sub == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	subscriberID := sub.GetID()
	p.subscribers[subscriberID] = sub

	// Register subscriber for its interested categories
	categories := sub.GetCategories()
	for _, category := range categories {
		if p.subscribersByCategory[category] == nil {
			p.subscribersByCategory[category] = make([]string, 0)
		}
		p.subscribersByCategory[category] = append(p.subscribersByCategory[category], subscriberID)
	}

	p.logger.Debug("Subscriber registered",
		log.String("subscriberID", subscriberID),
		log.Int("categoryCount", len(categories)),
		log.Any("categories", categories))
}

// Unsubscribe removes a subscriber.
func (p *categoryEventPublisher) Unsubscribe(sub subscriber.SubscriberInterface) {
	if sub == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	subscriberID := sub.GetID()

	// Remove from subscribers map
	delete(p.subscribers, subscriberID)

	// Remove from all category lists
	for category, subscriberList := range p.subscribersByCategory {
		newList := make([]string, 0, len(subscriberList))
		for _, id := range subscriberList {
			if id != subscriberID {
				newList = append(newList, id)
			}
		}
		p.subscribersByCategory[category] = newList

		// Clean up empty category lists
		if len(newList) == 0 {
			delete(p.subscribersByCategory, category)
		}
	}

	p.logger.Debug("Subscriber unregistered", log.String("subscriberID", subscriberID))
}

// GetActiveCategories returns categories that have at least one subscriber.
func (p *categoryEventPublisher) GetActiveCategories() []event.EventCategory {
	p.mu.RLock()
	defer p.mu.RUnlock()

	categories := make([]event.EventCategory, 0, len(p.subscribersByCategory))
	for category, subscribers := range p.subscribersByCategory {
		if len(subscribers) > 0 {
			categories = append(categories, category)
		}
	}

	return categories
}

// GetSubscribers returns a list of all currently subscribed subscribers.
// Returns a copy to prevent external modification.
func (p *categoryEventPublisher) GetSubscribers() []subscriber.SubscriberInterface {
	p.mu.RLock()
	defer p.mu.RUnlock()

	subscribers := make([]subscriber.SubscriberInterface, 0, len(p.subscribers))
	for _, sub := range p.subscribers {
		subscribers = append(subscribers, sub)
	}

	return subscribers
}

// Shutdown gracefully shuts down the publisher and all subscribers.
func (p *categoryEventPublisher) Shutdown() {
	p.mu.Lock()
	if p.isShutdown {
		p.mu.Unlock()
		return
	}
	p.isShutdown = true

	p.logger.Debug("Shutting down event publisher")

	// Signal all goroutines to stop accepting new events
	p.cancel()

	// Get all subscribers
	subscribers := make([]subscriber.SubscriberInterface, 0, len(p.subscribers))
	for _, sub := range p.subscribers {
		subscribers = append(subscribers, sub)
	}
	p.mu.Unlock()

	// Wait for all in-flight event processing to complete
	p.logger.Debug("Waiting for in-flight event processing to complete")
	p.wg.Wait()
	p.logger.Debug("All in-flight events processed")

	// Close all subscribers
	for _, sub := range subscribers {
		if err := sub.Close(); err != nil {
			p.logger.Error("Error closing subscriber",
				log.String("subscriberID", sub.GetID()),
				log.Error(err))
		}
	}

	p.logger.Debug("Event bus shutdown complete")
}
