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

package adapter

import (
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const kafkaAdapterComponentName = "KafkaAdapter"
const kafkaShutdownTimeout = 5 * time.Second

// kafkaAdapter publishes formatted events to a Kafka topic via sarama's AsyncProducer.
type kafkaAdapter struct {
	producer sarama.AsyncProducer
	topic    string
	logger   *log.Logger
	done     chan struct{}
	mu       sync.Mutex
	closed   bool
}

var _ OutputAdapterInterface = (*kafkaAdapter)(nil)

// newKafkaAdapter constructs a kafkaAdapter using a producer factory so tests can inject a mock.
func newKafkaAdapter(cfg config.ObservabilityKafkaConfig,
	newProducer func([]string, *sarama.Config) (sarama.AsyncProducer, error)) (*kafkaAdapter, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("kafka adapter requires at least one broker")
	}
	if cfg.Topic == "" {
		return nil, fmt.Errorf("kafka adapter requires a non-empty topic")
	}

	saramaCfg := sarama.NewConfig()
	saramaCfg.Producer.RequiredAcks = sarama.WaitForLocal
	saramaCfg.Producer.Retry.Max = cfg.Retries
	saramaCfg.Producer.Return.Errors = true
	saramaCfg.Producer.Return.Successes = false
	if cfg.Timeout > 0 {
		saramaCfg.Net.DialTimeout = cfg.Timeout
		saramaCfg.Net.ReadTimeout = cfg.Timeout
		saramaCfg.Net.WriteTimeout = cfg.Timeout
	}
	if cfg.ClientID != "" {
		saramaCfg.ClientID = cfg.ClientID
	}

	producer, err := newProducer(cfg.Brokers, saramaCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka producer: %w", err)
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, kafkaAdapterComponentName))
	ka := &kafkaAdapter{
		producer: producer,
		topic:    cfg.Topic,
		logger:   logger,
		done:     make(chan struct{}),
	}

	go ka.drainErrors()

	return ka, nil
}

// drainErrors consumes the producer's Errors() channel until it is closed by AsyncClose.
func (ka *kafkaAdapter) drainErrors() {
	defer close(ka.done)
	for prodErr := range ka.producer.Errors() {
		if prodErr == nil {
			continue
		}
		ka.logger.Error("Kafka producer error",
			log.String("topic", ka.topic),
			log.Error(prodErr.Err))
	}
}

// Write enqueues the data on the producer's Input channel without blocking on broker latency.
func (ka *kafkaAdapter) Write(data []byte) error {
	ka.mu.Lock()
	if ka.closed {
		ka.mu.Unlock()
		return fmt.Errorf("kafka adapter is closed")
	}
	ka.mu.Unlock()

	ka.producer.Input() <- &sarama.ProducerMessage{
		Topic: ka.topic,
		Value: sarama.ByteEncoder(data),
	}
	return nil
}

// Flush is a no-op; AsyncProducer.Close drains in-flight messages.
func (ka *kafkaAdapter) Flush() error {
	return nil
}

// Close drains in-flight messages and shuts down the error-drain goroutine.
func (ka *kafkaAdapter) Close() error {
	ka.mu.Lock()
	if ka.closed {
		ka.mu.Unlock()
		return nil
	}
	ka.closed = true
	ka.mu.Unlock()

	ka.producer.AsyncClose()

	select {
	case <-ka.done:
	case <-time.After(kafkaShutdownTimeout):
		ka.logger.Warn("Timed out waiting for kafka producer to drain",
			log.String("topic", ka.topic))
	}
	return nil
}

// GetName returns the name of this adapter.
func (ka *kafkaAdapter) GetName() string {
	return "KafkaAdapter"
}
