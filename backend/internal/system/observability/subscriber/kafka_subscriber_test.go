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
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/observability/formatter"
)

// stubAdapter captures Write calls so tests can verify republishing without a real Kafka broker.
type stubAdapter struct {
	mu      sync.Mutex
	writes  [][]byte
	writeFn func([]byte) error
	flushFn func() error
	closeFn func() error
	closed  bool
	flushed bool
}

func (s *stubAdapter) Write(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.writeFn != nil {
		return s.writeFn(data)
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	s.writes = append(s.writes, cp)
	return nil
}

func (s *stubAdapter) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flushed = true
	if s.flushFn != nil {
		return s.flushFn()
	}
	return nil
}

func (s *stubAdapter) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.closeFn != nil {
		return s.closeFn()
	}
	return nil
}

func (s *stubAdapter) GetName() string { return "stubAdapter" }

func (s *stubAdapter) Writes() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([][]byte, len(s.writes))
	copy(out, s.writes)
	return out
}

func newStubKafkaSubscriber(stub *stubAdapter) *KafkaSubscriber {
	return &KafkaSubscriber{
		id:         "kafka-sub-test",
		categories: []event.EventCategory{event.CategoryAll},
		formatter:  formatter.Initialize(formatJSON),
		adapter:    stub,
		logger:     log.GetLogger().With(log.String(log.LoggerKeyComponentName, kafkaSubscriberComponentName)),
	}
}

func TestNewKafkaSubscriber(t *testing.T) {
	if sub := NewKafkaSubscriber(); sub == nil {
		t.Fatal("NewKafkaSubscriber() returned nil")
	}
}

func TestKafkaSubscriber_IsEnabled(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	tests := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{"enabled when config is true", true, true},
		{"disabled when config is false", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.GetServerRuntime().Config.Observability.Output.Kafka.Enabled = tt.enabled
			sub := NewKafkaSubscriber()
			if got := sub.IsEnabled(); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKafkaSubscriber_Initialize_RejectsEmptyBrokers(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	cfg := &config.GetServerRuntime().Config.Observability.Output.Kafka
	cfg.Enabled = true
	cfg.Brokers = nil
	cfg.Topic = "events"

	if err := NewKafkaSubscriber().Initialize(); err == nil {
		t.Error("expected Initialize() to error when brokers are empty")
	}
}

func TestKafkaSubscriber_Initialize_RejectsEmptyTopic(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	cfg := &config.GetServerRuntime().Config.Observability.Output.Kafka
	cfg.Enabled = true
	cfg.Brokers = []string{"localhost:9092"}
	cfg.Topic = ""

	if err := NewKafkaSubscriber().Initialize(); err == nil {
		t.Error("expected Initialize() to error when topic is empty")
	}
}

func TestKafkaSubscriber_GetCategories_DefaultsToAll(t *testing.T) {
	sub := &KafkaSubscriber{categories: nil}
	cats := sub.GetCategories()
	if len(cats) != 1 || cats[0] != event.CategoryAll {
		t.Errorf("expected [CategoryAll], got %v", cats)
	}
}

func TestKafkaSubscriber_GetID(t *testing.T) {
	sub := newStubKafkaSubscriber(&stubAdapter{})
	if sub.GetID() != "kafka-sub-test" {
		t.Errorf("GetID() = %s, want kafka-sub-test", sub.GetID())
	}
}

func TestKafkaSubscriber_OnEvent_DelegatesToAdapter(t *testing.T) {
	stub := &stubAdapter{}
	sub := newStubKafkaSubscriber(stub)

	evt := &event.Event{
		TraceID:   "trace-1",
		EventID:   "event-1",
		Type:      "test.kafka",
		Timestamp: time.Now(),
		Component: "TestComponent",
		Status:    event.StatusSuccess,
		Data:      map[string]interface{}{"k": "v"},
	}

	if err := sub.OnEvent(evt); err != nil {
		t.Fatalf("OnEvent() error = %v", err)
	}

	writes := stub.Writes()
	if len(writes) != 1 {
		t.Fatalf("expected 1 write, got %d", len(writes))
	}
	if len(writes[0]) == 0 {
		t.Error("expected non-empty payload")
	}
}

func TestKafkaSubscriber_OnEvent_NilEventErrors(t *testing.T) {
	sub := newStubKafkaSubscriber(&stubAdapter{})
	if err := sub.OnEvent(nil); err == nil {
		t.Error("expected OnEvent(nil) to return error")
	}
}

func TestKafkaSubscriber_OnEvent_PropagatesAdapterError(t *testing.T) {
	stub := &stubAdapter{writeFn: func(_ []byte) error { return fmt.Errorf("boom") }}
	sub := newStubKafkaSubscriber(stub)

	evt := &event.Event{
		TraceID:   "trace-1",
		EventID:   "event-1",
		Type:      "test.kafka",
		Timestamp: time.Now(),
		Component: "TestComponent",
		Status:    event.StatusSuccess,
		Data:      map[string]interface{}{},
	}

	if err := sub.OnEvent(evt); err == nil {
		t.Error("expected adapter error to propagate")
	}
}

func TestKafkaSubscriber_Close(t *testing.T) {
	stub := &stubAdapter{}
	sub := newStubKafkaSubscriber(stub)

	if err := sub.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !stub.closed {
		t.Error("expected underlying adapter to be closed")
	}
}

func TestKafkaSubscriber_IsRegistered(t *testing.T) {
	if GetFactory("kafka") == nil {
		t.Error("expected kafka factory to be registered")
	}
}

func TestKafkaSubscriber_Factory_ReturnsKafkaSubscriber(t *testing.T) {
	factory := GetFactory("kafka")
	if factory == nil {
		t.Fatal("expected kafka factory to be registered")
	}
	if _, ok := factory().(*KafkaSubscriber); !ok {
		t.Error("expected factory to return *KafkaSubscriber")
	}
}

func TestKafkaSubscriber_IsEnabled_BeforeInitialize(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	config.GetServerRuntime().Config.Observability.Output.Kafka.Enabled = true

	sub := NewKafkaSubscriber()
	if !sub.IsEnabled() {
		t.Error("IsEnabled() should reflect config even before Initialize()")
	}
}

func TestKafkaSubscriber_GetID_BeforeInitialize(t *testing.T) {
	if id := NewKafkaSubscriber().GetID(); id != "" {
		t.Errorf("GetID() before Initialize() should be empty, got %q", id)
	}
}

func TestKafkaSubscriber_GetCategories_Configured(t *testing.T) {
	configured := []event.EventCategory{
		event.EventCategory("observability.authentication"),
		event.EventCategory("observability.flows"),
	}
	sub := &KafkaSubscriber{categories: configured}

	got := sub.GetCategories()
	if len(got) != len(configured) {
		t.Fatalf("GetCategories() len = %d, want %d", len(got), len(configured))
	}
	for i, c := range configured {
		if got[i] != c {
			t.Errorf("GetCategories()[%d] = %s, want %s", i, got[i], c)
		}
	}
}

func TestKafkaSubscriber_OnEvent_DifferentStatuses(t *testing.T) {
	stub := &stubAdapter{}
	sub := newStubKafkaSubscriber(stub)

	statuses := []string{
		event.StatusSuccess,
		event.StatusFailure,
		event.StatusPending,
		event.StatusInProgress,
	}

	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			evt := &event.Event{
				TraceID:   "trace-status",
				EventID:   "event-" + status,
				Type:      "test.status",
				Timestamp: time.Now(),
				Component: "TestComponent",
				Status:    status,
				Data:      map[string]interface{}{"status": status},
			}

			if err := sub.OnEvent(evt); err != nil {
				t.Errorf("OnEvent(status=%s) error = %v", status, err)
			}
		})
	}

	if got := len(stub.Writes()); got != len(statuses) {
		t.Errorf("expected %d writes, got %d", len(statuses), got)
	}
}

func TestKafkaSubscriber_OnEvent_EmptyData(t *testing.T) {
	stub := &stubAdapter{}
	sub := newStubKafkaSubscriber(stub)

	evt := &event.Event{
		TraceID:   "trace-empty",
		EventID:   "event-empty",
		Type:      "test.empty",
		Timestamp: time.Now(),
		Component: "TestComponent",
		Status:    event.StatusSuccess,
		Data:      map[string]interface{}{},
	}

	if err := sub.OnEvent(evt); err != nil {
		t.Fatalf("OnEvent() with empty data error = %v", err)
	}
	if len(stub.Writes()) != 1 {
		t.Fatalf("expected 1 write, got %d", len(stub.Writes()))
	}
}

func TestKafkaSubscriber_OnEvent_NilData(t *testing.T) {
	stub := &stubAdapter{}
	sub := newStubKafkaSubscriber(stub)

	evt := &event.Event{
		TraceID:   "trace-nil",
		EventID:   "event-nil",
		Type:      "test.nil",
		Timestamp: time.Now(),
		Component: "TestComponent",
		Status:    event.StatusSuccess,
		Data:      nil,
	}

	if err := sub.OnEvent(evt); err != nil {
		t.Errorf("OnEvent() with nil data should not error, got %v", err)
	}
}

func TestKafkaSubscriber_OnEvent_PayloadIsValidJSON(t *testing.T) {
	stub := &stubAdapter{}
	sub := newStubKafkaSubscriber(stub)

	evt := &event.Event{
		TraceID:   "trace-json",
		EventID:   "event-json",
		Type:      "test.json",
		Timestamp: time.Now(),
		Component: "TestComponent",
		Status:    event.StatusSuccess,
		Data:      map[string]interface{}{"k": "v", "n": 1},
	}

	if err := sub.OnEvent(evt); err != nil {
		t.Fatalf("OnEvent() error = %v", err)
	}

	writes := stub.Writes()
	if len(writes) != 1 {
		t.Fatalf("expected 1 write, got %d", len(writes))
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(writes[0], &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v\npayload=%s", err, string(writes[0]))
	}
}

func TestKafkaSubscriber_OnEvent_MultipleEvents(t *testing.T) {
	stub := &stubAdapter{}
	sub := newStubKafkaSubscriber(stub)

	const count = 5
	for i := 0; i < count; i++ {
		evt := &event.Event{
			TraceID:   fmt.Sprintf("trace-%d", i),
			EventID:   fmt.Sprintf("event-%d", i),
			Type:      "test.multi",
			Timestamp: time.Now(),
			Component: "TestComponent",
			Status:    event.StatusSuccess,
			Data:      map[string]interface{}{"i": i},
		}
		if err := sub.OnEvent(evt); err != nil {
			t.Fatalf("OnEvent(%d) error = %v", i, err)
		}
	}

	writes := stub.Writes()
	if len(writes) != count {
		t.Fatalf("expected %d writes, got %d", count, len(writes))
	}
}

func TestKafkaSubscriber_OnEvent_Concurrent(t *testing.T) {
	stub := &stubAdapter{}
	sub := newStubKafkaSubscriber(stub)

	const goroutines = 8
	const perGoroutine = 4

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				evt := &event.Event{
					TraceID:   fmt.Sprintf("trace-%d-%d", id, j),
					EventID:   fmt.Sprintf("event-%d-%d", id, j),
					Type:      "test.concurrent",
					Timestamp: time.Now(),
					Component: "TestComponent",
					Status:    event.StatusSuccess,
					Data:      map[string]interface{}{"g": id, "j": j},
				}
				_ = sub.OnEvent(evt)
			}
		}(g)
	}
	wg.Wait()

	if got := len(stub.Writes()); got != goroutines*perGoroutine {
		t.Errorf("expected %d writes, got %d", goroutines*perGoroutine, got)
	}
}

func TestKafkaSubscriber_Close_PropagatesCloseError(t *testing.T) {
	stub := &stubAdapter{closeFn: func() error { return fmt.Errorf("close failed") }}
	sub := newStubKafkaSubscriber(stub)

	if err := sub.Close(); err == nil {
		t.Error("expected Close() to propagate adapter close error")
	}
}

func TestKafkaSubscriber_Close_ContinuesWhenFlushFails(t *testing.T) {
	stub := &stubAdapter{flushFn: func() error { return fmt.Errorf("flush failed") }}
	sub := newStubKafkaSubscriber(stub)

	if err := sub.Close(); err != nil {
		t.Errorf("Close() should not return flush error, got %v", err)
	}
	if !stub.closed {
		t.Error("expected adapter to still be closed after flush failure")
	}
}

func TestKafkaSubscriber_Close_WithoutAdapter(t *testing.T) {
	sub := &KafkaSubscriber{
		id:     "kafka-sub-noadapter",
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, kafkaSubscriberComponentName)),
	}

	if err := sub.Close(); err != nil {
		t.Errorf("Close() with nil adapter should not error, got %v", err)
	}
}
