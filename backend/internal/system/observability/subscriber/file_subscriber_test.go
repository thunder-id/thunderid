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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/tests/mocks/observability/adaptermock"
)

func TestNewFileSubscriber(t *testing.T) {
	sub := NewFileSubscriber()
	if sub == nil {
		t.Fatal("NewFileSubscriber() returned nil")
	}
}

func TestFileSubscriber_IsEnabled(t *testing.T) {
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
			config.GetServerRuntime().Config.Observability.Output.File.Enabled = tt.enabled

			sub := NewFileSubscriber()
			if got := sub.IsEnabled(); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFileSubscriber_Initialize(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	tests := []struct {
		name        string
		config      func() string
		wantErr     bool
		errContains string
	}{
		{
			name: "successful initialization with explicit path",
			config: func() string {
				tmpDir := t.TempDir()
				filePath := filepath.Join(tmpDir, "test.log")
				cfg := &config.GetServerRuntime().Config.Observability.Output.File
				cfg.Enabled = true
				cfg.FilePath = filePath
				cfg.Format = formatJSON
				cfg.Categories = []string{}
				return filePath
			},
			wantErr: false,
		},
		{
			name: "successful initialization with default path",
			config: func() string {
				cfg := &config.GetServerRuntime().Config.Observability.Output.File
				cfg.Enabled = true
				cfg.FilePath = ""
				cfg.Format = formatJSON
				cfg.Categories = []string{"observability.authentication"}
				return ""
			},
			wantErr: false,
		},
		{
			name: "successful initialization with CSV format",
			config: func() string {
				tmpDir := t.TempDir()
				filePath := filepath.Join(tmpDir, "test.csv")
				cfg := &config.GetServerRuntime().Config.Observability.Output.File
				cfg.Enabled = true
				cfg.FilePath = filePath
				cfg.Format = "csv"
				return filePath
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestConfig(t)
			tt.config()

			sub := NewFileSubscriber()
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

func TestFileSubscriber_MultipleInitializations_ClosesExistingAdapterWithError(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test-reinit.log")

	cfg := &config.GetServerRuntime().Config.Observability.Output.File
	cfg.Enabled = true
	cfg.FilePath = filePath
	cfg.Format = formatJSON

	sub := NewFileSubscriber()

	// First initialization to properly set up the subscriber (logger, adapter, etc.)
	err := sub.Initialize()
	if err != nil {
		t.Fatalf("First Initialize() error = %v", err)
	}

	firstID := sub.GetID()

	// Close the real adapter to release file handles, then replace with a mock
	// that returns an error on Close
	_ = sub.adapter.Close()
	mockAdapter := adaptermock.NewOutputAdapterInterfaceMock(t)
	mockAdapter.EXPECT().Close().Return(errors.New("close error")).Once()
	sub.adapter = mockAdapter

	// Second Initialize - should attempt to close the existing (mock) adapter,
	// log the close error, and continue creating a new adapter
	err = sub.Initialize()

	// Initialization should succeed even if closing the old adapter fails
	if err != nil {
		t.Errorf("Second Initialize() unexpected error = %v", err)
	}

	// Verify that a new adapter was created (not the mock anymore)
	if sub.adapter == mockAdapter {
		t.Error("Initialize() should create a new adapter, but still has the old mock")
	}

	// Verify a new ID was generated
	if sub.GetID() == firstID {
		t.Error("Re-initialization should generate a new ID")
	}

	// Mock expectations (Close called once) are verified automatically by mockery

	// Clean up
	if sub.adapter != nil {
		_ = sub.Close()
	}
}

func TestFileSubscriber_GetID(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")

	cfg := &config.GetServerRuntime().Config.Observability.Output.File
	cfg.Enabled = true
	cfg.FilePath = filePath
	cfg.Format = formatJSON

	sub := NewFileSubscriber()
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

func TestFileSubscriber_GetCategories(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")

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
			cfg := &config.GetServerRuntime().Config.Observability.Output.File
			cfg.Enabled = true
			cfg.FilePath = filePath
			cfg.Categories = tt.categories

			sub := NewFileSubscriber()
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

func TestFileSubscriber_OnEvent(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")

	cfg := &config.GetServerRuntime().Config.Observability.Output.File
	cfg.Enabled = true
	cfg.FilePath = filePath
	cfg.Format = formatJSON

	sub := NewFileSubscriber()
	_ = sub.Initialize()
	defer func() { _ = sub.Close() }()

	tests := getCommonEventTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sub.OnEvent(tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("OnEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	// Verify file was written to
	_ = sub.adapter.Flush()
	content, err := os.ReadFile(filePath) // #nosec G304 - Test file path is controlled
	if err != nil {
		t.Fatalf("Failed to read test log file: %v", err)
	}

	if len(content) == 0 {
		t.Error("OnEvent() should write data to file")
	}

	// Verify JSON format
	if !strings.Contains(string(content), `"event_id"`) {
		t.Error("File content should contain JSON formatted events")
	}
}

func TestFileSubscriber_OnEventMultiple(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test-multiple.log")

	cfg := &config.GetServerRuntime().Config.Observability.Output.File
	cfg.Enabled = true
	cfg.FilePath = filePath
	cfg.Format = formatJSON

	sub := NewFileSubscriber()
	_ = sub.Initialize()
	defer func() { _ = sub.Close() }()

	// Write multiple events
	numEvents := 10
	for i := 0; i < numEvents; i++ {
		evt := event.NewEvent("trace-multi", "test.multi", "TestComponent").
			WithStatus(event.StatusSuccess).
			WithData("index", i)

		err := sub.OnEvent(evt)
		if err != nil {
			t.Errorf("OnEvent(%d) error = %v", i, err)
		}
	}

	// Flush and verify
	_ = sub.adapter.Flush()
	content, err := os.ReadFile(filePath) // #nosec G304 - Test file path is controlled
	if err != nil {
		t.Fatalf("Failed to read test log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != numEvents {
		t.Errorf("Expected %d events in file, got %d", numEvents, len(lines))
	}
}

func TestFileSubscriber_Close(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test-close.log")

	cfg := &config.GetServerRuntime().Config.Observability.Output.File
	cfg.Enabled = true
	cfg.FilePath = filePath
	cfg.Format = formatJSON

	sub := NewFileSubscriber()
	_ = sub.Initialize()

	// Write some data
	evt := &event.Event{
		TraceID:   "trace-123",
		EventID:   "event-123",
		Type:      "test.event",
		Timestamp: time.Now(),
		Component: "TestComponent",
		Status:    event.StatusSuccess,
		Data:      map[string]interface{}{},
	}
	_ = sub.OnEvent(evt)

	err := sub.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Verify file was flushed and closed
	content, err := os.ReadFile(filePath) // #nosec G304 - Test file path is controlled
	if err != nil {
		t.Fatalf("Failed to read test log file after close: %v", err)
	}

	if len(content) == 0 {
		t.Error("Close() should flush data before closing")
	}

	// Calling Close again should not error
	err = sub.Close()
	if err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

func TestFileSubscriber_CloseWithoutInitialize(t *testing.T) {
	// Note: In real usage, Close should only be called after successful Initialize
	// This test verifies that Close handles uninitialized state gracefully
	// However, since logger is not initialized, this will cause a panic in production
	// This is acceptable since the contract is that Initialize must be called first
	t.Skip("Close without Initialize is not a valid use case - Initialize must be called first")
}

func TestFileSubscriber_WriteAfterClose(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test-after-close.log")

	cfg := &config.GetServerRuntime().Config.Observability.Output.File
	cfg.Enabled = true
	cfg.FilePath = filePath
	cfg.Format = formatJSON

	sub := NewFileSubscriber()
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

func TestFileSubscriber_DefaultPathCreation(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	// Set up config with empty file path
	cfg := &config.GetServerRuntime().Config.Observability.Output.File
	cfg.Enabled = true
	cfg.FilePath = ""
	cfg.Format = formatJSON

	sub := NewFileSubscriber()
	err := sub.Initialize()

	// Should not error - should use default path
	if err != nil {
		// It's OK if it fails due to permissions, but it should attempt to create
		if !strings.Contains(err.Error(), "failed to create file adapter") {
			t.Errorf("Initialize() with empty path should attempt default path creation, got error: %v", err)
		}
	} else {
		// If successful, clean up
		_ = sub.Close()
	}
}

func TestFileSubscriber_FormatterSelection(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	tmpDir := t.TempDir()

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
			filePath := filepath.Join(tmpDir, "test-"+tt.name+".log")

			cfg := &config.GetServerRuntime().Config.Observability.Output.File
			cfg.Enabled = true
			cfg.FilePath = filePath
			cfg.Format = tt.format

			sub := NewFileSubscriber()
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

func BenchmarkFileSubscriber_OnEvent(b *testing.B) {
	setupTestConfig(&testing.T{})
	defer resetTestConfig()

	tmpDir := b.TempDir()
	filePath := filepath.Join(tmpDir, "benchmark.log")

	cfg := &config.GetServerRuntime().Config.Observability.Output.File
	cfg.Enabled = true
	cfg.FilePath = filePath
	cfg.Format = formatJSON

	sub := NewFileSubscriber()
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

func BenchmarkFileSubscriber_OnEventComplex(b *testing.B) {
	setupTestConfig(&testing.T{})
	defer resetTestConfig()

	tmpDir := b.TempDir()
	filePath := filepath.Join(tmpDir, "benchmark-complex.log")

	cfg := &config.GetServerRuntime().Config.Observability.Output.File
	cfg.Enabled = true
	cfg.FilePath = filePath
	cfg.Format = formatJSON

	sub := NewFileSubscriber()
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

func TestFileSubscriber_GetCategories_EmptyFallback(t *testing.T) {
	// Test the fallback path when categories slice is nil or empty
	sub := &FileSubscriber{
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

func TestFileSubscriber_CloseWithNilAdapter(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test-nil-adapter.log")

	cfg := &config.GetServerRuntime().Config.Observability.Output.File
	cfg.Enabled = true
	cfg.FilePath = filePath
	cfg.Format = formatJSON

	sub := NewFileSubscriber()
	_ = sub.Initialize()

	// Close the adapter first to release file handles and stop background goroutines
	_ = sub.adapter.Close()
	// Set adapter to nil to test the nil check path
	sub.adapter = nil

	// Close should handle nil adapter gracefully
	err := sub.Close()
	if err != nil {
		t.Errorf("Close() with nil adapter should not error, got: %v", err)
	}
}

func TestFileSubscriber_Initialize_EmptyCategoriesDefaultsToAll(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test-empty-categories.log")

	cfg := &config.GetServerRuntime().Config.Observability.Output.File
	cfg.Enabled = true
	cfg.FilePath = filePath
	cfg.Format = formatJSON
	cfg.Categories = []string{} // Empty categories

	sub := NewFileSubscriber()
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

func TestFileSubscriber_MultipleInitializations(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test-multi-init.log")

	cfg := &config.GetServerRuntime().Config.Observability.Output.File
	cfg.Enabled = true
	cfg.FilePath = filePath
	cfg.Format = formatJSON

	sub := NewFileSubscriber()

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

func TestFileSubscriber_OnEventConcurrent(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test-concurrent.log")

	cfg := &config.GetServerRuntime().Config.Observability.Output.File
	cfg.Enabled = true
	cfg.FilePath = filePath
	cfg.Format = formatJSON

	sub := NewFileSubscriber()
	_ = sub.Initialize()
	defer func() { _ = sub.Close() }()

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

	// Flush and verify data was written
	_ = sub.adapter.Flush()
	content, err := os.ReadFile(filePath) // #nosec G304 - Test file path is controlled
	if err != nil {
		t.Fatalf("Failed to read test log file: %v", err)
	}

	// Verify we got output (actual count may vary due to concurrency)
	if len(content) == 0 {
		t.Error("Concurrent OnEvent() calls should produce output")
	}
}

func TestFileSubscriber_GetID_BeforeInitialize(t *testing.T) {
	sub := NewFileSubscriber()

	// GetID before Initialize should return empty string
	id := sub.GetID()
	if id != "" {
		t.Errorf("GetID() before Initialize() should return empty string, got %s", id)
	}
}

func TestFileSubscriber_GetCategories_BeforeInitialize(t *testing.T) {
	sub := NewFileSubscriber()

	// GetCategories before Initialize should return default CategoryAll
	categories := sub.GetCategories()
	if len(categories) != 1 {
		t.Errorf("GetCategories() before Initialize() should return 1 category, got %d", len(categories))
	}
	if categories[0] != event.CategoryAll {
		t.Errorf("GetCategories() before Initialize() should return CategoryAll, got %s", categories[0])
	}
}

func TestFileSubscriber_OnEvent_DifferentEventStatuses(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test-statuses.log")

	cfg := &config.GetServerRuntime().Config.Observability.Output.File
	cfg.Enabled = true
	cfg.FilePath = filePath
	cfg.Format = formatJSON

	sub := NewFileSubscriber()
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

func TestFileSubscriber_OnEvent_EmptyData(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test-empty-data.log")

	cfg := &config.GetServerRuntime().Config.Observability.Output.File
	cfg.Enabled = true
	cfg.FilePath = filePath
	cfg.Format = formatJSON

	sub := NewFileSubscriber()
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

	// Flush and verify output was written even with empty data
	_ = sub.adapter.Flush()
	content, err := os.ReadFile(filePath) // #nosec G304 - Test file path is controlled
	if err != nil {
		t.Fatalf("Failed to read test log file: %v", err)
	}

	if len(content) == 0 {
		t.Error("OnEvent() should write output even with empty data")
	}
}

func TestFileSubscriber_OnEvent_NilData(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test-nil-data.log")

	cfg := &config.GetServerRuntime().Config.Observability.Output.File
	cfg.Enabled = true
	cfg.FilePath = filePath
	cfg.Format = formatJSON

	sub := NewFileSubscriber()
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

func TestFileSubscriber_Initialize_InvalidPath(t *testing.T) {
	setupTestConfig(t)
	defer resetTestConfig()

	// Use a path with null character that will fail on all operating systems
	// Both Windows and Unix-like systems reject null character in file paths
	invalidPath := t.TempDir() + string(filepath.Separator) + "invalid\x00file.log"

	cfg := &config.GetServerRuntime().Config.Observability.Output.File
	cfg.Enabled = true
	cfg.FilePath = invalidPath
	cfg.Format = formatJSON

	sub := NewFileSubscriber()
	err := sub.Initialize()

	// Should return an error
	if err == nil {
		t.Error("Initialize() with invalid path should return error")
		_ = sub.Close()
	}

	// Error should mention file adapter creation failure
	if err != nil && !strings.Contains(err.Error(), "failed to create file adapter") {
		t.Errorf("Initialize() error should mention file adapter creation failure, got: %v", err)
	}
}
