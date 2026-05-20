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

package observability

import (
	"testing"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
)

// setupTestService creates a test service with controlled configuration.
func setupTestService(enabled bool) ObservabilityServiceInterface {
	// Reset the global runtime config
	config.ResetServerRuntime()

	// Create a test config
	cfg := &config.Config{
		Observability: config.ObservabilityConfig{
			Enabled:     enabled,
			FailureMode: "lenient",
			Output: config.ObservabilityOutputConfig{
				Console: config.ObservabilityConsoleConfig{
					Enabled: true,
					Format:  "json",
				},
			},
		},
	}

	// Initialize the global runtime
	err := config.InitializeServerRuntime("/tmp/test", cfg)
	if err != nil {
		panic("failed to initialize test runtime: " + err.Error())
	}

	// Use Initialize to create a new instance (no singleton)
	return Initialize()
}

func TestInitialize(t *testing.T) {
	// Setup test environment first
	svc := setupTestService(true)
	defer svc.Shutdown()

	if svc == nil {
		t.Fatal("Initialize() returned nil")
	}

	// Verify service is enabled
	if !svc.IsEnabled() {
		t.Error("Service should be enabled when configured as enabled")
	}

	// Verify we can create multiple independent instances (no singleton)
	svc2 := setupTestService(true)
	defer svc2.Shutdown()

	if svc2 == nil {
		t.Error("Initialize() should return a new instance")
	}
}

func TestInitializeWithDisabled(t *testing.T) {
	// Test with disabled configuration
	svc := setupTestService(false)
	defer svc.Shutdown()

	if svc == nil {
		t.Fatal("Initialize() returned nil even when disabled")
	}

	if svc.IsEnabled() {
		t.Error("Service should be disabled when configured as disabled")
	}
}

func TestService_DisabledConfig(t *testing.T) {
	svc := setupTestService(false)

	if svc.IsEnabled() {
		t.Error("Service should be disabled when config.Enabled = false")
	}

	if svc.GetPublisher() != nil {
		t.Error("Publisher should be nil when service is disabled")
	}
}

func TestService_EnabledConfig(t *testing.T) {
	svc := setupTestService(true)

	if !svc.IsEnabled() {
		t.Error("Service should be enabled when config.Enabled = true")
	}

	if svc.GetPublisher() == nil {
		t.Error("Publisher should not be nil when service is enabled")
	}
}

func TestService_PublishEvent(t *testing.T) {
	svc := setupTestService(true)
	defer svc.Shutdown()

	// Note: Subscribers are now auto-registered via registry pattern
	// This test just verifies PublishEvent doesn't panic

	evt := event.NewEvent("trace-123", string(event.EventTypeTokenIssued), "test")
	evt.WithStatus(event.StatusSuccess)

	// Should not panic
	svc.PublishEvent(evt)

	// Give it time to process (async processing)
	time.Sleep(100 * time.Millisecond)

	// Verify service is operational
	if !svc.IsEnabled() {
		t.Error("Service should be enabled")
	}
}

func TestService_PublishEventDisabled(t *testing.T) {
	svc := setupTestService(false)

	// Test creating a new event
	evt := event.NewEvent("trace-123", string(event.EventTypeTokenIssuanceStarted), "test")

	// Should not panic even when disabled
	svc.PublishEvent(evt)
}

func TestService_PublishNilEvent(t *testing.T) {
	svc := setupTestService(true)
	defer svc.Shutdown()

	// Should not panic
	svc.PublishEvent(nil)
}

func TestService_GetConfig(t *testing.T) {
	svc := setupTestService(true)
	defer svc.Shutdown()

	retrievedCfg := svc.GetConfig()
	if retrievedCfg == nil {
		t.Error("GetConfig() returned nil")
		return
	}

	if !retrievedCfg.Enabled {
		t.Error("Config should be enabled")
	}

	if retrievedCfg.Output.Console.Format != "json" {
		t.Errorf("Expected format 'json', got '%s'", retrievedCfg.Output.Console.Format)
	}
}

func TestService_GetPublisher(t *testing.T) {
	t.Run("enabled service", func(t *testing.T) {
		svc := setupTestService(true)
		defer svc.Shutdown()

		pub := svc.GetPublisher()
		if pub == nil {
			t.Error("GetPublisher() should return non-nil publisher for enabled service")
		}
	})

	t.Run("disabled service", func(t *testing.T) {
		svc := setupTestService(false)

		pub := svc.GetPublisher()
		if pub != nil {
			t.Error("GetPublisher() should return nil publisher for disabled service")
		}
	})
}

// TestService_RegisterSubscriber is skipped - RegisterSubscriber functionality
// is now handled automatically via the registry pattern. Subscribers self-register
// via init() functions and are auto-discovered during service initialization.
func TestService_RegisterSubscriber(t *testing.T) {
	t.Skip("RegisterSubscriber is deprecated - subscribers now auto-register via registry pattern")
}

func TestService_GetActiveSubscribers(t *testing.T) {
	t.Skip("Test uses RegisterSubscriber - needs update for registry pattern")
}
func TestService_Shutdown(t *testing.T) {
	t.Skip("Test uses RegisterSubscriber - needs update for registry pattern")
}

func TestService_ShutdownDisabled(t *testing.T) {
	svc := setupTestService(false)

	// Shutdown should not panic even when disabled
	svc.Shutdown()
}

func TestService_MultipleSubscribers(t *testing.T) {
	t.Skip("Test uses RegisterSubscriber - needs update for registry pattern")
}

func TestService_MultipleEvents(t *testing.T) {
	t.Skip("Test uses RegisterSubscriber - needs update for registry pattern")
}

func TestService_IsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{
			name:    "enabled service",
			enabled: true,
			want:    true,
		},
		{
			name:    "disabled service",
			enabled: false,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := setupTestService(tt.enabled)
			defer svc.Shutdown()

			if got := svc.IsEnabled(); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestService_CategoryBasedRouting(t *testing.T) {
	t.Skip("Test uses RegisterSubscriber - needs update for registry pattern")
}

func TestService_ConcurrentPublish(t *testing.T) {
	t.Skip("Test uses RegisterSubscriber - needs update for registry pattern")
}

func TestService_SubscriberPanic(t *testing.T) {
	t.Skip("Test uses RegisterSubscriber - needs update for registry pattern")
}
