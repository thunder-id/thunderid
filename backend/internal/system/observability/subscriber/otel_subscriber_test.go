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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

const (
	exporterTypeStdout = "stdout"

	// Test constants for context propagation tests
	testTraceIDWithHyphens = "4bf92f35-77b3-4da6-a3ce-929d0e0e4736"
	testTraceIDSanitized   = "4bf92f3577b34da6a3ce929d0e0e4736"
)

// OTelSubscriberTestSuite is the test suite for OTelSubscriber
type OTelSubscriberTestSuite struct {
	suite.Suite
}

// TestOTelSubscriberTestSuite runs the test suite
func TestOTelSubscriberTestSuite(t *testing.T) {
	suite.Run(t, new(OTelSubscriberTestSuite))
}

// SetupTest runs before each test
func (suite *OTelSubscriberTestSuite) SetupTest() {
	setupTestConfig(suite.T())
}

// TearDownTest runs after each test
func (suite *OTelSubscriberTestSuite) TearDownTest() {
	resetTestConfig()
}

// setupTestSubscriber creates a test subscriber with default stdout exporter config
func (suite *OTelSubscriberTestSuite) setupTestSubscriber() *OTelSubscriber {
	cfg := &config.GetServerRuntime().Config.Observability.Output.OpenTelemetry
	cfg.Enabled = true
	cfg.ExporterType = exporterTypeStdout
	cfg.ServiceName = "test-service"
	cfg.ServiceVersion = "1.0.0"
	cfg.SampleRate = 1.0

	sub := NewOTelSubscriber()
	_ = sub.Initialize()
	return sub
}

// Test suite for basic subscriber operations

func (suite *OTelSubscriberTestSuite) TestNewOTelSubscriber() {
	sub := NewOTelSubscriber()
	assert.NotNil(suite.T(), sub, "NewOTelSubscriber() returned nil")
}

func (suite *OTelSubscriberTestSuite) TestIsEnabled_WhenConfigTrue() {
	config.GetServerRuntime().Config.Observability.Output.OpenTelemetry.Enabled = true

	sub := NewOTelSubscriber()
	assert.True(suite.T(), sub.IsEnabled(), "IsEnabled() should return true when config is enabled")
}

func (suite *OTelSubscriberTestSuite) TestIsEnabled_WhenConfigFalse() {
	config.GetServerRuntime().Config.Observability.Output.OpenTelemetry.Enabled = false

	sub := NewOTelSubscriber()
	assert.False(suite.T(), sub.IsEnabled(), "IsEnabled() should return false when config is disabled")
}

func (suite *OTelSubscriberTestSuite) TestInitialize_WithStdoutExporter() {
	cfg := &config.GetServerRuntime().Config.Observability.Output.OpenTelemetry
	cfg.Enabled = true
	cfg.ExporterType = exporterTypeStdout
	cfg.ServiceName = "test-service"
	cfg.ServiceVersion = "1.0.0"
	cfg.SampleRate = 1.0

	sub := NewOTelSubscriber()
	err := sub.Initialize()

	assert.NoError(suite.T(), err, "Initialize() should not return error")
	assert.NotEmpty(suite.T(), sub.GetID(), "Initialize() should set subscriber ID")
	assert.NotNil(suite.T(), sub.tracer, "Initialize() should set tracer")
	assert.NotNil(suite.T(), sub.tracerProvider, "Initialize() should set tracer provider")

	_ = sub.Close()
}

func (suite *OTelSubscriberTestSuite) TestInitialize_WithDefaultCategories() {
	cfg := &config.GetServerRuntime().Config.Observability.Output.OpenTelemetry
	cfg.Enabled = true
	cfg.ExporterType = exporterTypeStdout
	cfg.Categories = []string{}

	sub := NewOTelSubscriber()
	err := sub.Initialize()

	assert.NoError(suite.T(), err, "Initialize() should not return error")
	_ = sub.Close()
}

func (suite *OTelSubscriberTestSuite) TestGetID() {
	sub := suite.setupTestSubscriber()
	defer func() { _ = sub.Close() }()

	id := sub.GetID()
	assert.NotEmpty(suite.T(), id, "GetID() returned empty string")
	assert.Equal(suite.T(), id, sub.GetID(), "GetID() should return consistent ID")
}

func (suite *OTelSubscriberTestSuite) TestGetCategories_WithDefaultCategories() {
	cfg := &config.GetServerRuntime().Config.Observability.Output.OpenTelemetry
	cfg.Enabled = true
	cfg.ExporterType = exporterTypeStdout
	cfg.Categories = []string{}

	sub := NewOTelSubscriber()
	_ = sub.Initialize()
	defer func() { _ = sub.Close() }()

	categories := sub.GetCategories()
	assert.Len(suite.T(), categories, 1, "GetCategories() should return default CategoryAll")
}

func (suite *OTelSubscriberTestSuite) TestGetCategories_WithConfiguredCategories() {
	cfg := &config.GetServerRuntime().Config.Observability.Output.OpenTelemetry
	cfg.Enabled = true
	cfg.ExporterType = exporterTypeStdout
	cfg.Categories = []string{"observability.authentication", "observability.flows"}

	sub := NewOTelSubscriber()
	_ = sub.Initialize()
	defer func() { _ = sub.Close() }()

	categories := sub.GetCategories()
	assert.Len(suite.T(), categories, 2, "GetCategories() should return configured categories")
}

// Test suite for OnEvent

func (suite *OTelSubscriberTestSuite) TestOnEvent_NilEvent() {
	sub := suite.setupTestSubscriber()
	defer func() { _ = sub.Close() }()

	err := sub.OnEvent(nil)
	assert.Error(suite.T(), err, "OnEvent() should return error for nil event")
}

func (suite *OTelSubscriberTestSuite) TestOnEvent_ValidEvent() {
	sub := suite.setupTestSubscriber()
	defer func() { _ = sub.Close() }()

	testEvent := &event.Event{
		TraceID:   "trace-123",
		EventID:   "event-123",
		Type:      "test.event",
		Timestamp: time.Now(),
		Component: "TestComponent",
		Status:    event.StatusSuccess,
		Data: map[string]interface{}{
			"key1": "value1",
			"key2": 123,
		},
	}

	err := sub.OnEvent(testEvent)
	assert.NoError(suite.T(), err, "OnEvent() should not return error for valid event")
}

func (suite *OTelSubscriberTestSuite) TestOnEvent_FailureEvent() {
	sub := suite.setupTestSubscriber()
	defer func() { _ = sub.Close() }()

	testEvent := &event.Event{
		TraceID:   "trace-456",
		EventID:   "event-456",
		Type:      "test.failure",
		Timestamp: time.Now(),
		Component: "TestComponent",
		Status:    event.StatusFailure,
		Data: map[string]interface{}{
			event.DataKey.Error: "test error message",
		},
	}

	err := sub.OnEvent(testEvent)
	assert.NoError(suite.T(), err, "OnEvent() should not return error for failure event")
}

func (suite *OTelSubscriberTestSuite) TestOnEvent_VariousDataTypes() {
	sub := suite.setupTestSubscriber()
	defer func() { _ = sub.Close() }()

	testEvent := &event.Event{
		TraceID:   "trace-789",
		EventID:   "event-789",
		Type:      "test.datatypes",
		Timestamp: time.Now(),
		Component: "OAuth2Server",
		Status:    event.StatusSuccess,
		Data: map[string]interface{}{
			"string":  "value",
			"int":     42,
			"int64":   int64(123456),
			"float64": 3.14,
			"bool":    true,
			"nil":     nil,
		},
	}

	err := sub.OnEvent(testEvent)
	assert.NoError(suite.T(), err, "OnEvent() should handle various data types")
}

func (suite *OTelSubscriberTestSuite) TestOnEvent_WithSpanRecorder() {
	// Create a span recorder to capture spans
	spanRecorder := tracetest.NewSpanRecorder()

	// Create a tracer provider with the span recorder
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(spanRecorder),
	)

	// Create logger for the subscriber
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "TestOTelSubscriber"))

	sub := &OTelSubscriber{
		id:             "test-sub",
		tracerProvider: tracerProvider,
		tracer:         tracerProvider.Tracer("thunderid-observability"),
		categories:     []event.EventCategory{event.CategoryAll},
		logger:         logger,
	}

	testEvent := &event.Event{
		TraceID:   "trace-123",
		EventID:   "event-123",
		Type:      "test.event",
		Timestamp: time.Now(),
		Component: "TestComponent",
		Status:    event.StatusSuccess,
		Data: map[string]interface{}{
			"test_key": "test_value",
		},
	}

	err := sub.OnEvent(testEvent)
	assert.NoError(suite.T(), err, "OnEvent() should not return error")

	// Force flush spans
	_ = tracerProvider.ForceFlush(context.Background())

	// Verify span was created
	spans := spanRecorder.Ended()
	assert.Len(suite.T(), spans, 1, "Expected 1 span to be created")

	if len(spans) == 1 {
		span := spans[0]
		assert.Equal(suite.T(), testEvent.Type, span.Name(), "Span name should match event type")

		// Verify span attributes include both metadata and event data
		attrs := span.Attributes()
		expectedAttrs := map[string]bool{
			"event.id":     false,
			"trace.id":     false,
			"component":    false,
			"event.status": false,
			"test_key":     false, // Event data should now be in tags
		}

		for _, attr := range attrs {
			if _, exists := expectedAttrs[string(attr.Key)]; exists {
				expectedAttrs[string(attr.Key)] = true
			}
			// Verify test_key value
			if string(attr.Key) == "test_key" {
				assert.Equal(suite.T(), "test_value", attr.Value.AsString(),
					"Event data 'test_key' should have correct value in span attributes")
			}
		}

		for key, found := range expectedAttrs {
			assert.True(suite.T(), found, "Expected attribute %s not found in span", key)
		}

		// Verify span events (logs) also contain the event data for backward compatibility
		events := span.Events()
		assert.Len(suite.T(), events, 1, "Expected 1 span event to be created")

		if len(events) == 1 {
			spanEvent := events[0]
			assert.Equal(suite.T(), testEvent.Type, spanEvent.Name, "Span event name should match event type")

			// Verify span event also has the test_key attribute
			testKeyFound := false
			for _, attr := range spanEvent.Attributes {
				if string(attr.Key) == "test_key" {
					testKeyFound = true
					assert.Equal(suite.T(), "test_value", attr.Value.AsString(),
						"Event data 'test_key' should have correct value in span event")
				}
			}
			assert.True(suite.T(), testKeyFound,
				"Span event should contain 'test_key' attribute for backward compatibility")
		}
	}

	// Clean up
	_ = sub.Close()
}

// Test suite for context propagation

func (suite *OTelSubscriberTestSuite) TestContextPropagation_SameTraceID() {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)

	cfg := config.GetServerRuntime()
	cfg.Config.Observability.Output.OpenTelemetry.Enabled = true
	cfg.Config.Observability.Output.OpenTelemetry.ExporterType = "memory"

	sub := &OTelSubscriber{
		id:             "test-subscriber",
		tracerProvider: tp,
		tracer:         tp.Tracer("test-tracer"),
		categories:     []event.EventCategory{event.CategoryAll},
		logger:         log.GetLogger(),
	}

	// Use UUID format (with hyphens) to verify sanitization
	traceID := testTraceIDWithHyphens
	expectedTraceID := testTraceIDSanitized

	evt1 := event.NewEvent(traceID, "event.one", "test-component")
	evt2 := event.NewEvent(traceID, "event.two", "test-component")

	err := sub.OnEvent(evt1)
	assert.NoError(suite.T(), err)

	err = sub.OnEvent(evt2)
	assert.NoError(suite.T(), err)

	// Verify spans
	spans := exporter.GetSpans()
	assert.Len(suite.T(), spans, 2, "Expected 2 spans")

	if len(spans) == 2 {
		assert.Equal(suite.T(), expectedTraceID, spans[0].SpanContext.TraceID().String(),
			"First span should have sanitized TraceID")
		assert.Equal(suite.T(), expectedTraceID, spans[1].SpanContext.TraceID().String(),
			"Second span should have same TraceID")
		assert.NotEqual(suite.T(), spans[0].SpanContext.SpanID(), spans[1].SpanContext.SpanID(),
			"Spans should have different SpanIDs")
	}
}

func (suite *OTelSubscriberTestSuite) TestContextPropagation_ParentSpanID() {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)

	sub := &OTelSubscriber{
		id:             "test-subscriber",
		tracerProvider: tp,
		tracer:         tp.Tracer("test-tracer"),
		categories:     []event.EventCategory{event.CategoryAll},
		logger:         log.GetLogger(),
	}

	traceID := "4bf92f35-77b3-4da6-a3ce-929d0e0e4736"
	expectedTraceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	parentSpanID := "00f067aa0ba902b7"

	evt := event.NewEvent(traceID, "event.child", "test-component").
		WithData(event.DataKey.TraceParent, parentSpanID)

	err := sub.OnEvent(evt)
	assert.NoError(suite.T(), err)

	spans := exporter.GetSpans()
	assert.Len(suite.T(), spans, 1, "Expected 1 span")

	if len(spans) == 1 {
		assert.Equal(suite.T(), expectedTraceID, spans[0].SpanContext.TraceID().String(),
			"Span should have correct TraceID")
		assert.Equal(suite.T(), parentSpanID, spans[0].Parent.SpanID().String(),
			"Span should have correct parent SpanID")
	}
}

func (suite *OTelSubscriberTestSuite) TestContextPropagation_InvalidParentSpanID() {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)

	sub := &OTelSubscriber{
		id:             "test-subscriber",
		tracerProvider: tp,
		tracer:         tp.Tracer("test-tracer"),
		categories:     []event.EventCategory{event.CategoryAll},
		logger:         log.GetLogger(),
	}

	traceID := "4bf92f35-77b3-4da6-a3ce-929d0e0e4736"
	expectedTraceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	invalidParentID := "invalid-parent-id"

	evt := event.NewEvent(traceID, "event.invalid.parent", "test-component").
		WithData(event.DataKey.TraceParent, invalidParentID)

	err := sub.OnEvent(evt)
	assert.NoError(suite.T(), err, "OnEvent should not error with invalid parent ID")

	spans := exporter.GetSpans()
	assert.Len(suite.T(), spans, 1, "Expected 1 span")

	if len(spans) == 1 {
		assert.Equal(suite.T(), expectedTraceID, spans[0].SpanContext.TraceID().String(),
			"Span should have correct TraceID even with invalid parent")
		assert.False(suite.T(), spans[0].Parent.IsValid(),
			"Parent SpanID should be invalid when parent ID is malformed")
	}
}

func (suite *OTelSubscriberTestSuite) TestContextPropagation_InvalidTraceID() {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)

	sub := &OTelSubscriber{
		id:             "test-subscriber",
		tracerProvider: tp,
		tracer:         tp.Tracer("test-tracer"),
		categories:     []event.EventCategory{event.CategoryAll},
		logger:         log.GetLogger(),
	}

	invalidTraceID := "invalid-trace-id"
	evt := event.NewEvent(invalidTraceID, "event.invalid.trace", "test-component")

	err := sub.OnEvent(evt)
	assert.NoError(suite.T(), err, "OnEvent should not error with invalid trace ID")

	spans := exporter.GetSpans()
	assert.Len(suite.T(), spans, 1, "Expected 1 span")

	if len(spans) == 1 {
		assert.NotEqual(suite.T(), invalidTraceID, spans[0].SpanContext.TraceID().String(),
			"TraceID should not be the invalid one")
		assert.True(suite.T(), spans[0].SpanContext.TraceID().IsValid(),
			"TraceID should be valid (generated by OTel)")
	}
}

// Test suite for helper methods

func (suite *OTelSubscriberTestSuite) TestConvertDataToAttributes_EmptyData() {
	sub := &OTelSubscriber{}
	data := map[string]interface{}{}

	attrs := sub.convertDataToAttributes(data)
	assert.Len(suite.T(), attrs, 0, "Empty data should produce no attributes")
}

func (suite *OTelSubscriberTestSuite) TestConvertDataToAttributes_StringData() {
	sub := &OTelSubscriber{}
	data := map[string]interface{}{
		"key": "value",
	}

	attrs := sub.convertDataToAttributes(data)
	assert.Len(suite.T(), attrs, 1, "Should convert string data")
	assert.Equal(suite.T(), attribute.Key("key"), attrs[0].Key)
	assert.Equal(suite.T(), "value", attrs[0].Value.AsString())
}

func (suite *OTelSubscriberTestSuite) TestConvertDataToAttributes_IntData() {
	sub := &OTelSubscriber{}
	data := map[string]interface{}{
		"count": 42,
	}

	attrs := sub.convertDataToAttributes(data)
	assert.Len(suite.T(), attrs, 1, "Should convert int data")
	assert.Equal(suite.T(), int64(42), attrs[0].Value.AsInt64())
}

func (suite *OTelSubscriberTestSuite) TestConvertDataToAttributes_Int64Data() {
	sub := &OTelSubscriber{}
	data := map[string]interface{}{
		"big": int64(123456),
	}

	attrs := sub.convertDataToAttributes(data)
	assert.Len(suite.T(), attrs, 1, "Should convert int64 data")
}

func (suite *OTelSubscriberTestSuite) TestConvertDataToAttributes_Float64Data() {
	sub := &OTelSubscriber{}
	data := map[string]interface{}{
		"ratio": 3.14,
	}

	attrs := sub.convertDataToAttributes(data)
	assert.Len(suite.T(), attrs, 1, "Should convert float64 data")
}

func (suite *OTelSubscriberTestSuite) TestConvertDataToAttributes_BoolData() {
	sub := &OTelSubscriber{}
	data := map[string]interface{}{
		"flag": true,
	}

	attrs := sub.convertDataToAttributes(data)
	assert.Len(suite.T(), attrs, 1, "Should convert bool data")
}

func (suite *OTelSubscriberTestSuite) TestConvertDataToAttributes_NilValuesSkipped() {
	sub := &OTelSubscriber{}
	data := map[string]interface{}{
		"valid": "value",
		"null":  nil,
	}

	attrs := sub.convertDataToAttributes(data)
	assert.Len(suite.T(), attrs, 1, "Nil values should be skipped")
}

func (suite *OTelSubscriberTestSuite) TestConvertDataToAttributes_EmptyStringSkipped() {
	sub := &OTelSubscriber{}
	data := map[string]interface{}{
		"valid": "value",
		"empty": "",
	}

	attrs := sub.convertDataToAttributes(data)
	assert.Len(suite.T(), attrs, 1, "Empty strings should be skipped")
}

func (suite *OTelSubscriberTestSuite) TestConvertDataToAttributes_MixedDataTypes() {
	sub := &OTelSubscriber{}
	data := map[string]interface{}{
		"string":  "value",
		"int":     42,
		"float":   3.14,
		"bool":    true,
		"nil":     nil,
		"empty":   "",
		"complex": map[string]string{"nested": "value"},
	}

	attrs := sub.convertDataToAttributes(data)
	assert.Len(suite.T(), attrs, 5, "Should convert mixed types, skip nil and empty")
}

func (suite *OTelSubscriberTestSuite) TestGetStringData_ValidString() {
	sub := &OTelSubscriber{}
	evt := &event.Event{
		Data: map[string]interface{}{
			"key": "value",
		},
	}

	result := sub.getStringData(evt, "key")
	assert.Equal(suite.T(), "value", result, "Should return string value")
}

func (suite *OTelSubscriberTestSuite) TestGetStringData_MissingKey() {
	sub := &OTelSubscriber{}
	evt := &event.Event{
		Data: map[string]interface{}{},
	}

	result := sub.getStringData(evt, "missing")
	assert.Equal(suite.T(), "", result, "Should return empty string for missing key")
}

func (suite *OTelSubscriberTestSuite) TestGetStringData_NonStringValue() {
	sub := &OTelSubscriber{}
	evt := &event.Event{
		Data: map[string]interface{}{
			"number": 123,
		},
	}

	result := sub.getStringData(evt, "number")
	assert.Equal(suite.T(), "", result, "Should return empty string for non-string value")
}

// Test suite for Close operations

func (suite *OTelSubscriberTestSuite) TestClose_Success() {
	sub := suite.setupTestSubscriber()

	err := sub.Close()
	assert.NoError(suite.T(), err, "Close() should not return error")
	assert.Nil(suite.T(), sub.tracerProvider, "Close() should set tracerProvider to nil")
}

func (suite *OTelSubscriberTestSuite) TestClose_CalledTwice() {
	sub := suite.setupTestSubscriber()

	err := sub.Close()
	assert.NoError(suite.T(), err, "First Close() should not return error")

	err = sub.Close()
	assert.NoError(suite.T(), err, "Second Close() should not return error")
}

// Benchmark tests

func BenchmarkOTelSubscriber_OnEvent(b *testing.B) {
	setupTestConfig(&testing.T{})
	defer resetTestConfig()

	config.GetServerRuntime().Config.Observability.Output.OpenTelemetry.Enabled = true
	config.GetServerRuntime().Config.Observability.Output.OpenTelemetry.ExporterType = exporterTypeStdout

	sub := NewOTelSubscriber()
	_ = sub.Initialize()
	defer func() { _ = sub.Close() }()

	testEvent := &event.Event{
		TraceID:   "trace-123",
		EventID:   "event-123",
		Type:      "benchmark.event",
		Timestamp: time.Now(),
		Component: "BenchmarkComponent",
		Status:    event.StatusSuccess,
		Data: map[string]interface{}{
			"key1": "value1",
			"key2": 123,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sub.OnEvent(testEvent)
	}
}

func BenchmarkOTelSubscriber_convertDataToAttributes(b *testing.B) {
	sub := &OTelSubscriber{}
	data := map[string]interface{}{
		"string":  "value",
		"int":     42,
		"int64":   int64(123456),
		"float64": 3.14,
		"bool":    true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sub.convertDataToAttributes(data)
	}
}

// Helper functions for testing

func setupTestConfig(t *testing.T) {
	// Reset any existing runtime first
	config.ResetServerRuntime()

	// Create a test config
	testConfig := &config.Config{
		Observability: config.ObservabilityConfig{
			Enabled: true,
			Output: config.ObservabilityOutputConfig{
				OpenTelemetry: config.ObservabilityOTelConfig{
					Enabled:        false,
					ExporterType:   "stdout",
					ServiceName:    "test-service",
					ServiceVersion: "1.0.0",
					Environment:    "test",
					SampleRate:     1.0,
					Insecure:       true,
					Categories:     []string{},
				},
				File: config.ObservabilityFileConfig{
					Enabled: false,
				},
				Console: config.ObservabilityConsoleConfig{
					Enabled: false,
				},
			},
		},
	}

	// Initialize server runtime with test config
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	if err != nil {
		t.Fatalf("Failed to initialize server runtime: %v", err)
	}
}

func resetTestConfig() {
	// Reset to default disabled state
	config.ResetServerRuntime()
}
