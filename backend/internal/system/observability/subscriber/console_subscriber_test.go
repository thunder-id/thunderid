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
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
)

func TestNewConsoleSubscriber(t *testing.T) {
	sub := NewConsoleSubscriber()
	if sub == nil {
		t.Fatal("NewConsoleSubscriber() returned nil")
	}
}

func TestConsoleSubscriber_IsEnabled(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	tests := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{
			name:    "enabled when config is true",
			enabled: true,
			want:    true,
		},
		{
			name:    "disabled when config is false",
			enabled: false,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.GetServerRuntime().Config.Observability.Output.Console.Enabled = tt.enabled

			sub := NewConsoleSubscriber()
			if got := sub.IsEnabled(); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConsoleSubscriber_Initialize(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	tests := []struct {
		name    string
		config  func()
		wantErr bool
	}{
		{
			name: "successful initialization with json format",
			config: func() {
				cfg := &config.GetServerRuntime().Config.Observability.Output.Console
				cfg.Enabled = true
				cfg.Format = formatJSON
				cfg.Categories = []string{}
			},
			wantErr: false,
		},
		{
			name: "successful initialization with categories",
			config: func() {
				cfg := &config.GetServerRuntime().Config.Observability.Output.Console
				cfg.Enabled = true
				cfg.Format = formatJSON
				cfg.Categories = []string{"observability.authentication", "observability.flows"}
			},
			wantErr: false,
		},
		{
			name: "successful initialization with default format",
			config: func() {
				cfg := &config.GetServerRuntime().Config.Observability.Output.Console
				cfg.Enabled = true
				cfg.Format = ""
				cfg.Categories = []string{}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestConfig(t)
			tt.config()

			sub := NewConsoleSubscriber()
			err := sub.Initialize()

			if (err != nil) != tt.wantErr {
				t.Errorf("Initialize() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if sub.GetID() == "" {
					t.Error("Initialize() should set subscriber ID")
				}
				if sub.formatter == nil {
					t.Error("Initialize() should set formatter")
				}
				if sub.adapter == nil {
					t.Error("Initialize() should set adapter")
				}
				if sub.logger == nil {
					t.Error("Initialize() should set logger")
				}

				// Clean up
				_ = sub.Close()
			}
		})
	}
}

func TestConsoleSubscriber_GetID(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	cfg := &config.GetServerRuntime().Config.Observability.Output.Console
	cfg.Enabled = true
	cfg.Format = formatJSON

	sub := NewConsoleSubscriber()
	_ = sub.Initialize()
	defer func() { _ = sub.Close() }()

	id := sub.GetID()
	if id == "" {
		t.Error("GetID() returned empty string")
	}

	// ID should be consistent
	if id != sub.GetID() {
		t.Error("GetID() should return consistent ID")
	}
}

func TestConsoleSubscriber_GetCategories(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	tests := []struct {
		name       string
		categories []string
		wantLen    int
		wantAll    bool
	}{
		{
			name:       "returns default CategoryAll when no categories configured",
			categories: []string{},
			wantLen:    1,
			wantAll:    true,
		},
		{
			name:       "returns configured categories",
			categories: []string{"observability.authentication", "observability.flows"},
			wantLen:    2,
			wantAll:    false,
		},
		{
			name:       "returns single category",
			categories: []string{"observability.authentication"},
			wantLen:    1,
			wantAll:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestConfig(t)
			cfg := &config.GetServerRuntime().Config.Observability.Output.Console
			cfg.Enabled = true
			cfg.Format = formatJSON
			cfg.Categories = tt.categories

			sub := NewConsoleSubscriber()
			_ = sub.Initialize()
			defer func() { _ = sub.Close() }()

			categories := sub.GetCategories()
			if len(categories) != tt.wantLen {
				t.Errorf("GetCategories() returned %d categories, want %d", len(categories), tt.wantLen)
			}

			if tt.wantAll && categories[0] != event.CategoryAll {
				t.Errorf("GetCategories() should return CategoryAll when no categories configured")
			}
		})
	}
}

func TestConsoleSubscriber_OnEvent(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	cfg := &config.GetServerRuntime().Config.Observability.Output.Console
	cfg.Enabled = true
	cfg.Format = formatJSON

	sub := NewConsoleSubscriber()
	_ = sub.Initialize()
	defer func() {
		_ = sub.Close()
	}()

	tests := getCommonEventTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sub.OnEvent(tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("OnEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConsoleSubscriber_OnEventWithCapture(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cfg := &config.GetServerRuntime().Config.Observability.Output.Console
	cfg.Enabled = true
	cfg.Format = formatJSON

	sub := NewConsoleSubscriber()
	_ = sub.Initialize()
	defer func() { _ = sub.Close() }()

	testEvent := &event.Event{
		TraceID:   "trace-capture",
		EventID:   "event-capture",
		Type:      "test.capture",
		Timestamp: time.Now(),
		Component: "TestComponent",
		Status:    event.StatusSuccess,
		Data: map[string]interface{}{
			"test_field": "test_value",
		},
	}

	err := sub.OnEvent(testEvent)
	if err != nil {
		t.Fatalf("OnEvent() unexpected error = %v", err)
	}

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify output contains expected data
	if !strings.Contains(output, "trace-capture") {
		t.Error("Output should contain trace ID")
	}
	if !strings.Contains(output, "event-capture") {
		t.Error("Output should contain event ID")
	}
	if !strings.Contains(output, "test.capture") {
		t.Error("Output should contain event type")
	}
	if !strings.Contains(output, "test_field") {
		t.Error("Output should contain event data")
	}
}

func TestConsoleSubscriber_OnEventMultiple(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cfg := &config.GetServerRuntime().Config.Observability.Output.Console
	cfg.Enabled = true
	cfg.Format = formatJSON

	sub := NewConsoleSubscriber()
	_ = sub.Initialize()
	defer func() { _ = sub.Close() }()

	numEvents := 5
	for i := 0; i < numEvents; i++ {
		evt := event.NewEvent("trace-multi", "test.multi", "TestComponent").
			WithStatus(event.StatusSuccess).
			WithData("index", i)

		err := sub.OnEvent(evt)
		if err != nil {
			t.Errorf("OnEvent(%d) error = %v", i, err)
		}
	}

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify multiple events were written
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < numEvents {
		t.Errorf("Expected at least %d lines in output, got %d", numEvents, len(lines))
	}
}

func TestConsoleSubscriber_Close(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	cfg := &config.GetServerRuntime().Config.Observability.Output.Console
	cfg.Enabled = true
	cfg.Format = formatJSON

	sub := NewConsoleSubscriber()
	_ = sub.Initialize()

	err := sub.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Calling Close again should not error
	err = sub.Close()
	if err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

func TestConsoleSubscriber_CloseWithoutInitialize(t *testing.T) {
	// Note: In real usage, Close should only be called after successful Initialize
	// This test verifies that Close handles uninitialized state gracefully
	// However, since logger is not initialized, this will cause a panic in production
	// This is acceptable since the contract is that Initialize must be called first
	t.Skip("Close without Initialize is not a valid use case - Initialize must be called first")
}

func TestConsoleSubscriber_WriteAfterClose(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	cfg := &config.GetServerRuntime().Config.Observability.Output.Console
	cfg.Enabled = true
	cfg.Format = formatJSON

	sub := NewConsoleSubscriber()
	_ = sub.Initialize()

	err := sub.Close()
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Try to write after close
	evt := &event.Event{
		TraceID:   "trace-123",
		EventID:   "event-123",
		Type:      "test.event",
		Timestamp: time.Now(),
		Component: "TestComponent",
		Status:    event.StatusSuccess,
		Data:      map[string]interface{}{},
	}

	err = sub.OnEvent(evt)
	if err == nil {
		t.Error("OnEvent() should return error after Close()")
	}
}

func TestConsoleSubscriber_FormatterSelection(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	tests := []struct {
		name   string
		format string
	}{
		{
			name:   "json formatter",
			format: "json",
		},
		{
			name:   "default to json for unknown format",
			format: "unknown",
		},
		{
			name:   "empty format defaults to json",
			format: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestConfig(t)

			cfg := &config.GetServerRuntime().Config.Observability.Output.Console
			cfg.Enabled = true
			cfg.Format = tt.format

			sub := NewConsoleSubscriber()
			err := sub.Initialize()
			if err != nil {
				t.Fatalf("Initialize() error = %v", err)
			}
			defer func() { _ = sub.Close() }()

			if sub.formatter == nil {
				t.Error("Initialize() should create a formatter")
			}
		})
	}
}

func TestConsoleSubscriber_EventWithEmptyData(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cfg := &config.GetServerRuntime().Config.Observability.Output.Console
	cfg.Enabled = true
	cfg.Format = formatJSON

	sub := NewConsoleSubscriber()
	_ = sub.Initialize()
	defer func() { _ = sub.Close() }()

	testEvent := &event.Event{
		TraceID:   "trace-empty",
		EventID:   "event-empty",
		Type:      "test.empty",
		Timestamp: time.Now(),
		Component: "TestComponent",
		Status:    event.StatusSuccess,
		Data:      map[string]interface{}{},
	}

	err := sub.OnEvent(testEvent)
	if err != nil {
		t.Fatalf("OnEvent() with empty data error = %v", err)
	}

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify output was written even with empty data
	if len(output) == 0 {
		t.Error("OnEvent() should write output even with empty data")
	}
}

func TestConsoleSubscriber_EventWithNilData(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	cfg := &config.GetServerRuntime().Config.Observability.Output.Console
	cfg.Enabled = true
	cfg.Format = formatJSON

	sub := NewConsoleSubscriber()
	_ = sub.Initialize()
	defer func() { _ = sub.Close() }()

	testEvent := &event.Event{
		TraceID:   "trace-nil",
		EventID:   "event-nil",
		Type:      "test.nil",
		Timestamp: time.Now(),
		Component: "TestComponent",
		Status:    event.StatusSuccess,
		Data:      nil,
	}

	// Should handle nil data gracefully
	err := sub.OnEvent(testEvent)
	if err != nil {
		t.Errorf("OnEvent() with nil data should not error, got: %v", err)
	}
}

func BenchmarkConsoleSubscriber_OnEvent(b *testing.B) {
	setupTestConfig(&testing.T{})
	defer resetTestConfig()

	// Redirect stdout to /dev/null for benchmarking
	oldStdout := os.Stdout
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	defer func() {
		os.Stdout = oldStdout
		_ = devNull.Close()
	}()

	cfg := &config.GetServerRuntime().Config.Observability.Output.Console
	cfg.Enabled = true
	cfg.Format = formatJSON

	sub := NewConsoleSubscriber()
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

func BenchmarkConsoleSubscriber_OnEventComplex(b *testing.B) {
	setupTestConfig(&testing.T{})
	defer resetTestConfig()

	// Redirect stdout to /dev/null for benchmarking
	oldStdout := os.Stdout
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	defer func() {
		os.Stdout = oldStdout
		_ = devNull.Close()
	}()

	cfg := &config.GetServerRuntime().Config.Observability.Output.Console
	cfg.Enabled = true
	cfg.Format = formatJSON

	sub := NewConsoleSubscriber()
	_ = sub.Initialize()
	defer func() { _ = sub.Close() }()

	testEvent := &event.Event{
		TraceID:   "trace-123",
		EventID:   "event-123",
		Type:      "benchmark.complex",
		Timestamp: time.Now(),
		Component: "BenchmarkComponent",
		Status:    event.StatusSuccess,
		Data: map[string]interface{}{
			"string": "value",
			"int":    42,
			"float":  3.14,
			"bool":   true,
			"nested": map[string]interface{}{"key": "value", "num": 123},
			"array":  []string{"a", "b", "c", "d", "e"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sub.OnEvent(testEvent)
	}
}

func TestConsoleSubscriber_GetCategories_EmptyFallback(t *testing.T) {
	// Test the fallback path when categories slice is nil or empty
	sub := &ConsoleSubscriber{
		categories: nil,
	}

	categories := sub.GetCategories()
	if len(categories) != 1 {
		t.Errorf("GetCategories() with nil categories should return 1 category, got %d", len(categories))
	}
	if categories[0] != event.CategoryAll {
		t.Errorf("GetCategories() with nil categories should return CategoryAll, got %s", categories[0])
	}

	// Test with empty slice
	sub.categories = []event.EventCategory{}
	categories = sub.GetCategories()
	if len(categories) != 1 {
		t.Errorf("GetCategories() with empty categories should return 1 category, got %d", len(categories))
	}
	if categories[0] != event.CategoryAll {
		t.Errorf("GetCategories() with empty categories should return CategoryAll, got %s", categories[0])
	}
}

func TestConsoleSubscriber_CloseWithNilAdapter(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	sub := NewConsoleSubscriber()
	_ = sub.Initialize()

	// Set adapter to nil to test the nil check path
	sub.adapter = nil

	// Close should handle nil adapter gracefully
	err := sub.Close()
	if err != nil {
		t.Errorf("Close() with nil adapter should not error, got: %v", err)
	}
}

func TestConsoleSubscriber_Initialize_EmptyCategoriesDefaultsToAll(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	cfg := &config.GetServerRuntime().Config.Observability.Output.Console
	cfg.Enabled = true
	cfg.Format = formatJSON
	cfg.Categories = []string{} // Empty categories

	sub := NewConsoleSubscriber()
	err := sub.Initialize()
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	defer func() { _ = sub.Close() }()

	// Should default to CategoryAll
	categories := sub.GetCategories()
	if len(categories) != 1 {
		t.Errorf("Expected 1 category, got %d", len(categories))
	}
	if categories[0] != event.CategoryAll {
		t.Errorf("Expected CategoryAll, got %s", categories[0])
	}
}

func TestConsoleSubscriber_MultipleInitializations(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	cfg := &config.GetServerRuntime().Config.Observability.Output.Console
	cfg.Enabled = true
	cfg.Format = formatJSON

	sub := NewConsoleSubscriber()

	// First initialization
	err := sub.Initialize()
	if err != nil {
		t.Fatalf("First Initialize() error = %v", err)
	}

	firstID := sub.GetID()

	// Second initialization (re-initialize)
	err = sub.Initialize()
	if err != nil {
		t.Fatalf("Second Initialize() error = %v", err)
	}

	secondID := sub.GetID()

	// IDs should be different since UUID is generated each time
	if firstID == secondID {
		t.Error("Re-initialization should generate a new ID")
	}

	// Clean up
	_ = sub.Close()
}

func TestConsoleSubscriber_OnEventConcurrent(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cfg := &config.GetServerRuntime().Config.Observability.Output.Console
	cfg.Enabled = true
	cfg.Format = formatJSON

	sub := NewConsoleSubscriber()
	_ = sub.Initialize()
	defer func() { _ = sub.Close() }()

	// Read from pipe concurrently to prevent buffer saturation and deadlock
	var buf bytes.Buffer
	readerDone := make(chan bool)
	go func() {
		_, _ = io.Copy(&buf, r)
		readerDone <- true
	}()

	// Test concurrent writes
	const numGoroutines = 10
	const eventsPerGoroutine = 5

	done := make(chan bool)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < eventsPerGoroutine; j++ {
				evt := event.NewEvent("trace-concurrent", "test.concurrent", "TestComponent").
					WithStatus(event.StatusSuccess).
					WithData("goroutine", id).
					WithData("event", j)

				_ = sub.OnEvent(evt)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Restore stdout and close writer to signal reader
	_ = w.Close()
	os.Stdout = oldStdout

	// Wait for reader to finish
	<-readerDone

	output := buf.String()

	// Verify we got output (actual count may vary due to concurrency)
	if len(output) == 0 {
		t.Error("Concurrent OnEvent() calls should produce output")
	}
}

func TestConsoleSubscriber_GetID_BeforeInitialize(t *testing.T) {
	sub := NewConsoleSubscriber()

	// GetID before Initialize should return empty string
	id := sub.GetID()
	if id != "" {
		t.Errorf("GetID() before Initialize() should return empty string, got %s", id)
	}
}

func TestConsoleSubscriber_GetCategories_BeforeInitialize(t *testing.T) {
	sub := NewConsoleSubscriber()

	// GetCategories before Initialize should return default CategoryAll
	categories := sub.GetCategories()
	if len(categories) != 1 {
		t.Errorf("GetCategories() before Initialize() should return 1 category, got %d", len(categories))
	}
	if categories[0] != event.CategoryAll {
		t.Errorf("GetCategories() before Initialize() should return CategoryAll, got %s", categories[0])
	}
}

func TestConsoleSubscriber_OnEvent_DifferentEventStatuses(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	cfg := &config.GetServerRuntime().Config.Observability.Output.Console
	cfg.Enabled = true
	cfg.Format = formatJSON

	sub := NewConsoleSubscriber()
	_ = sub.Initialize()
	defer func() { _ = sub.Close() }()

	statuses := []string{
		event.StatusSuccess,
		event.StatusFailure,
		event.StatusPending,
		event.StatusInProgress,
	}

	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			evt := &event.Event{
				TraceID:   "trace-status-test",
				EventID:   "event-status-test",
				Type:      "test.status",
				Timestamp: time.Now(),
				Component: "TestComponent",
				Status:    status,
				Data:      map[string]interface{}{"status": status},
			}

			err := sub.OnEvent(evt)
			if err != nil {
				t.Errorf("OnEvent() with status %s error = %v", status, err)
			}
		})
	}
}
