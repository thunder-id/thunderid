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

package subscriber

import (
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/adapter"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/observability/formatter"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const consoleSubscriberComponentName = "ConsoleSubscriber"

// ConsoleSubscriber writes observability events to the console.
// It supports category-based filtering and JSON formatting.
type ConsoleSubscriber struct {
	id         string
	categories []event.EventCategory
	formatter  formatter.FormatterInterface
	adapter    adapter.OutputAdapterInterface
	logger     *log.Logger
}

var _ SubscriberInterface = (*ConsoleSubscriber)(nil)

// init registers the console subscriber factory with the global registry.
// This runs before main() and only registers the factory function.
// No configuration access or instance creation happens here.
func init() {
	RegisterSubscriberFactory("console", func() SubscriberInterface {
		return NewConsoleSubscriber()
	})
}

// NewConsoleSubscriber creates a new console subscriber instance.
func NewConsoleSubscriber() *ConsoleSubscriber {
	return &ConsoleSubscriber{}
}

// IsEnabled checks if the console subscriber should be activated based on configuration.
func (cs *ConsoleSubscriber) IsEnabled() bool {
	return config.GetServerRuntime().Config.Observability.Output.Console.Enabled
}

// Initialize sets up the console subscriber with the provided configuration.
func (cs *ConsoleSubscriber) Initialize() error {
	// Get config directly from config package (avoid import cycle)
	consoleConfig := config.GetServerRuntime().Config.Observability.Output.Console

	// Create formatter based on config using the Initialize pattern
	fmtr := formatter.Initialize(consoleConfig.Format)

	// Create console adapter using the Initialize pattern
	adptr := adapter.InitializeConsoleAdapter()

	// Set categories - convert strings to EventCategory
	cs.categories = convertCategories(consoleConfig.Categories)
	if len(cs.categories) == 0 {
		cs.categories = []event.EventCategory{event.CategoryAll}
	}

	cs.logger = log.GetLogger().With(log.String(log.LoggerKeyComponentName, consoleSubscriberComponentName))

	id, err := utils.GenerateUUIDv7()
	if err != nil {
		cs.logger.Error("failed to generate UUID for console subscriber", log.Error(err))
		return err
	}
	cs.id = id

	cs.formatter = fmtr
	cs.adapter = adptr

	cs.logger.Debug("Console subscriber initialized",
		log.String("format", consoleConfig.Format),
		log.Int("categories", len(cs.categories)))

	return nil
}

// GetID returns the unique identifier for this subscriber.
func (cs *ConsoleSubscriber) GetID() string {
	return cs.id
}

// GetCategories returns the categories this subscriber is interested in.
func (cs *ConsoleSubscriber) GetCategories() []event.EventCategory {
	if len(cs.categories) > 0 {
		return cs.categories
	}
	// Default: all categories
	return []event.EventCategory{event.CategoryAll}
}

// OnEvent is called when a new event is published.
func (cs *ConsoleSubscriber) OnEvent(evt *event.Event) error {
	return processEvent(evt, cs.formatter, cs.adapter, cs.logger, "console")
}

// Close closes the subscriber and releases resources.
func (cs *ConsoleSubscriber) Close() error {
	cs.logger.Info("Closing console subscriber", log.String("subscriberID", cs.id))

	// Flush and close adapter
	if cs.adapter != nil {
		if err := cs.adapter.Flush(); err != nil {
			cs.logger.Error("Failed to flush console adapter", log.Error(err))
		}

		if err := cs.adapter.Close(); err != nil {
			cs.logger.Error("Failed to close console adapter", log.Error(err))
			return err
		}
	}

	cs.logger.Info("Console subscriber closed", log.String("subscriberID", cs.id))
	return nil
}
