/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/adapter"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/observability/formatter"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const kafkaSubscriberComponentName = "KafkaSubscriber"

// KafkaSubscriber republishes observability events to a configured Kafka topic.
type KafkaSubscriber struct {
	id         string
	categories []event.EventCategory
	formatter  formatter.FormatterInterface
	adapter    adapter.OutputAdapterInterface
	logger     *log.Logger
}

var _ SubscriberInterface = (*KafkaSubscriber)(nil)

func init() {
	RegisterSubscriberFactory("kafka", func() SubscriberInterface {
		return NewKafkaSubscriber()
	})
}

// NewKafkaSubscriber creates a new kafka subscriber instance.
func NewKafkaSubscriber() *KafkaSubscriber {
	return &KafkaSubscriber{}
}

// IsEnabled checks if the kafka subscriber should be activated based on configuration.
func (ks *KafkaSubscriber) IsEnabled() bool {
	return config.GetServerRuntime().Config.Observability.Output.Kafka.Enabled
}

// Initialize sets up the kafka subscriber with the provided configuration.
func (ks *KafkaSubscriber) Initialize() error {
	kafkaConfig := config.GetServerRuntime().Config.Observability.Output.Kafka

	if len(kafkaConfig.Brokers) == 0 {
		return fmt.Errorf("kafka subscriber requires at least one broker")
	}
	if kafkaConfig.Topic == "" {
		return fmt.Errorf("kafka subscriber requires a non-empty topic")
	}

	fmtr := formatter.Initialize(kafkaConfig.Format)

	if ks.adapter != nil {
		if err := ks.adapter.Close(); err != nil && ks.logger != nil {
			ks.logger.Warn("failed to close existing kafka adapter", log.Error(err))
		}
	}

	adptr, err := adapter.InitializeKafkaAdapter(kafkaConfig)
	if err != nil {
		return fmt.Errorf("failed to create kafka adapter: %w", err)
	}

	ks.categories = convertCategories(kafkaConfig.Categories)
	if len(ks.categories) == 0 {
		ks.categories = []event.EventCategory{event.CategoryAll}
	}

	id, err := utils.GenerateUUIDv7()
	if err != nil {
		return fmt.Errorf("failed to generate kafka subscriber ID: %w", err)
	}

	ks.id = id
	ks.formatter = fmtr
	ks.adapter = adptr
	ks.logger = log.GetLogger().With(log.String(log.LoggerKeyComponentName, kafkaSubscriberComponentName))

	ks.logger.Debug("Kafka subscriber initialized",
		log.String("topic", kafkaConfig.Topic),
		log.Int("brokers", len(kafkaConfig.Brokers)),
		log.String("format", kafkaConfig.Format),
		log.Int("categories", len(ks.categories)))

	return nil
}

// GetID returns the unique identifier for this subscriber.
func (ks *KafkaSubscriber) GetID() string {
	return ks.id
}

// GetCategories returns the categories this subscriber is interested in.
func (ks *KafkaSubscriber) GetCategories() []event.EventCategory {
	if len(ks.categories) > 0 {
		return ks.categories
	}
	return []event.EventCategory{event.CategoryAll}
}

// OnEvent is called when a new event is published.
func (ks *KafkaSubscriber) OnEvent(evt *event.Event) error {
	return processEvent(evt, ks.formatter, ks.adapter, ks.logger, "kafka")
}

// Close closes the subscriber and releases resources.
func (ks *KafkaSubscriber) Close() error {
	ks.logger.Info("Closing kafka subscriber", log.String("subscriberID", ks.id))

	if ks.adapter != nil {
		if err := ks.adapter.Flush(); err != nil {
			ks.logger.Error("Failed to flush kafka adapter", log.Error(err))
		}

		if err := ks.adapter.Close(); err != nil {
			ks.logger.Error("Failed to close kafka adapter", log.Error(err))
			return err
		}
	}

	ks.logger.Info("Kafka subscriber closed", log.String("subscriberID", ks.id))
	return nil
}
