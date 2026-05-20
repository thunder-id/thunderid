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
	"errors"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"

	"github.com/thunder-id/thunderid/internal/system/config"
)

func newMockKafkaAdapter(t *testing.T, mp *mocks.AsyncProducer, cfg config.ObservabilityKafkaConfig) *kafkaAdapter {
	t.Helper()
	ka, err := newKafkaAdapter(cfg, func(_ []string, _ *sarama.Config) (sarama.AsyncProducer, error) {
		return mp, nil
	})
	if err != nil {
		t.Fatalf("newKafkaAdapter() error = %v", err)
	}
	return ka
}

func TestKafkaAdapter_RequiresBrokers(t *testing.T) {
	_, err := newKafkaAdapter(config.ObservabilityKafkaConfig{Topic: "t"}, func(_ []string, _ *sarama.Config) (sarama.AsyncProducer, error) {
		t.Fatal("producer factory should not be called when validation fails")
		return nil, nil
	})
	if err == nil {
		t.Error("expected error when brokers are empty")
	}
}

func TestKafkaAdapter_RequiresTopic(t *testing.T) {
	_, err := newKafkaAdapter(config.ObservabilityKafkaConfig{Brokers: []string{"localhost:9092"}}, func(_ []string, _ *sarama.Config) (sarama.AsyncProducer, error) {
		t.Fatal("producer factory should not be called when validation fails")
		return nil, nil
	})
	if err == nil {
		t.Error("expected error when topic is empty")
	}
}

func TestKafkaAdapter_WritePublishesToTopic(t *testing.T) {
	mp := mocks.NewAsyncProducer(t, nil)
	mp.ExpectInputWithMessageCheckerFunctionAndSucceed(func(msg *sarama.ProducerMessage) error {
		if msg.Topic != "events-topic" {
			t.Errorf("unexpected topic: %s", msg.Topic)
		}
		val, err := msg.Value.Encode()
		if err != nil {
			return err
		}
		if string(val) != `{"foo":"bar"}` {
			t.Errorf("unexpected payload: %s", string(val))
		}
		return nil
	})

	ka := newMockKafkaAdapter(t, mp, config.ObservabilityKafkaConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "events-topic",
		Retries: 1,
		Timeout: time.Second,
	})

	if err := ka.Write([]byte(`{"foo":"bar"}`)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if err := ka.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestKafkaAdapter_WriteAfterCloseErrors(t *testing.T) {
	mp := mocks.NewAsyncProducer(t, nil)
	ka := newMockKafkaAdapter(t, mp, config.ObservabilityKafkaConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "events-topic",
	})

	if err := ka.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if err := ka.Write([]byte("data")); err == nil {
		t.Error("expected Write() to error after Close()")
	}
}

func TestKafkaAdapter_CloseIsIdempotent(t *testing.T) {
	mp := mocks.NewAsyncProducer(t, nil)
	ka := newMockKafkaAdapter(t, mp, config.ObservabilityKafkaConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "events-topic",
	})

	if err := ka.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	if err := ka.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestKafkaAdapter_GetName(t *testing.T) {
	mp := mocks.NewAsyncProducer(t, nil)
	ka := newMockKafkaAdapter(t, mp, config.ObservabilityKafkaConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "events-topic",
	})
	defer func() { _ = ka.Close() }()

	if ka.GetName() != "KafkaAdapter" {
		t.Errorf("GetName() = %s, want KafkaAdapter", ka.GetName())
	}
}

func TestKafkaAdapter_ProducerFactoryError(t *testing.T) {
	factoryErr := errors.New("producer init failed")
	_, err := newKafkaAdapter(config.ObservabilityKafkaConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "events-topic",
	}, func(_ []string, _ *sarama.Config) (sarama.AsyncProducer, error) {
		return nil, factoryErr
	})
	if err == nil {
		t.Fatal("expected error when producer factory fails")
	}
	if !errors.Is(err, factoryErr) {
		t.Errorf("expected factory error to be wrapped, got %v", err)
	}
}

func TestKafkaAdapter_WriteMultiple(t *testing.T) {
	mp := mocks.NewAsyncProducer(t, nil)
	const count = 5
	for i := 0; i < count; i++ {
		mp.ExpectInputAndSucceed()
	}

	ka := newMockKafkaAdapter(t, mp, config.ObservabilityKafkaConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "events-topic",
	})

	for i := 0; i < count; i++ {
		if err := ka.Write([]byte("payload")); err != nil {
			t.Fatalf("Write(%d) error = %v", i, err)
		}
	}

	if err := ka.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestKafkaAdapter_ProducerErrorIsDrained(t *testing.T) {
	mp := mocks.NewAsyncProducer(t, nil)
	mp.ExpectInputAndFail(errors.New("broker rejected"))

	ka := newMockKafkaAdapter(t, mp, config.ObservabilityKafkaConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "events-topic",
	})

	if err := ka.Write([]byte("payload")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	closeDone := make(chan error, 1)
	go func() { closeDone <- ka.Close() }()

	select {
	case err := <-closeDone:
		if err != nil {
			t.Errorf("Close() error = %v", err)
		}
	case <-time.After(kafkaShutdownTimeout + time.Second):
		t.Fatal("Close() blocked despite producer error being emitted")
	}
}

func TestKafkaAdapter_ConfiguresSaramaConfig(t *testing.T) {
	var captured *sarama.Config

	mp := mocks.NewAsyncProducer(t, nil)
	ka, err := newKafkaAdapter(config.ObservabilityKafkaConfig{
		Brokers:  []string{"localhost:9092"},
		Topic:    "events-topic",
		ClientID: "abcd-client",
		Retries:  7,
		Timeout:  2 * time.Second,
	}, func(_ []string, cfg *sarama.Config) (sarama.AsyncProducer, error) {
		captured = cfg
		return mp, nil
	})
	if err != nil {
		t.Fatalf("newKafkaAdapter() error = %v", err)
	}
	defer func() { _ = ka.Close() }()

	if captured == nil {
		t.Fatal("expected sarama config to be captured")
	}
	if captured.ClientID != "abcd-client" {
		t.Errorf("ClientID = %s, want abcd-client", captured.ClientID)
	}
	if captured.Producer.Retry.Max != 7 {
		t.Errorf("Retry.Max = %d, want 7", captured.Producer.Retry.Max)
	}
	if captured.Producer.RequiredAcks != sarama.WaitForLocal {
		t.Errorf("RequiredAcks = %v, want WaitForLocal", captured.Producer.RequiredAcks)
	}
	if !captured.Producer.Return.Errors {
		t.Error("Return.Errors should be true")
	}
	if captured.Producer.Return.Successes {
		t.Error("Return.Successes should be false")
	}
	if captured.Net.DialTimeout != 2*time.Second {
		t.Errorf("DialTimeout = %v, want 2s", captured.Net.DialTimeout)
	}
	if captured.Net.ReadTimeout != 2*time.Second {
		t.Errorf("ReadTimeout = %v, want 2s", captured.Net.ReadTimeout)
	}
	if captured.Net.WriteTimeout != 2*time.Second {
		t.Errorf("WriteTimeout = %v, want 2s", captured.Net.WriteTimeout)
	}
}

func TestKafkaAdapter_ZeroTimeoutKeepsDefaults(t *testing.T) {
	defaults := sarama.NewConfig()
	var captured *sarama.Config

	mp := mocks.NewAsyncProducer(t, nil)
	ka, err := newKafkaAdapter(config.ObservabilityKafkaConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "events-topic",
	}, func(_ []string, cfg *sarama.Config) (sarama.AsyncProducer, error) {
		captured = cfg
		return mp, nil
	})
	if err != nil {
		t.Fatalf("newKafkaAdapter() error = %v", err)
	}
	defer func() { _ = ka.Close() }()

	if captured.Net.DialTimeout != defaults.Net.DialTimeout {
		t.Errorf("DialTimeout overridden to %v despite zero config, want default %v",
			captured.Net.DialTimeout, defaults.Net.DialTimeout)
	}
	if captured.ClientID != defaults.ClientID {
		t.Errorf("ClientID overridden to %q despite empty config, want default %q",
			captured.ClientID, defaults.ClientID)
	}
}

func TestKafkaAdapter_Flush_IsNoop(t *testing.T) {
	mp := mocks.NewAsyncProducer(t, nil)
	ka := newMockKafkaAdapter(t, mp, config.ObservabilityKafkaConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "events-topic",
	})
	defer func() { _ = ka.Close() }()

	if err := ka.Flush(); err != nil {
		t.Errorf("Flush() error = %v", err)
	}
}
