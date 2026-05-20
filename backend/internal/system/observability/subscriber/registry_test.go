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
	"testing"

	"github.com/thunder-id/thunderid/internal/system/observability/event"
)

// mockSubscriber is a mock implementation of SubscriberInterface for testing
type mockSubscriber struct {
	id         string
	enabled    bool
	initError  error
	categories []event.EventCategory
}

func (m *mockSubscriber) GetID() string {
	return m.id
}

func (m *mockSubscriber) GetCategories() []event.EventCategory {
	return m.categories
}

func (m *mockSubscriber) OnEvent(evt *event.Event) error {
	return nil
}

func (m *mockSubscriber) Close() error {
	return nil
}

func (m *mockSubscriber) IsEnabled() bool {
	return m.enabled
}

func (m *mockSubscriber) Initialize() error {
	return m.initError
}

// TestRegisterSubscriberFactory tests registering a subscriber factory
func TestRegisterSubscriberFactory(t *testing.T) {
	// Clear registry before test
	ClearRegistry()

	// Register a factory
	factory := func() SubscriberInterface {
		return &mockSubscriber{
			id:      "test-subscriber",
			enabled: true,
		}
	}

	RegisterSubscriberFactory("test", factory)

	// Verify it was registered
	registeredFactory := GetFactory("test")
	if registeredFactory == nil {
		t.Fatal("Expected factory to be registered, got nil")
	}

	// Verify the factory creates the correct instance
	instance := registeredFactory()
	if instance == nil {
		t.Fatal("Expected factory to create instance, got nil")
	}

	if instance.GetID() != "test-subscriber" {
		t.Errorf("Expected subscriber ID 'test-subscriber', got '%s'", instance.GetID())
	}
}

// TestRegisterMultipleFactories tests registering multiple subscriber factories
func TestRegisterMultipleFactories(t *testing.T) {
	// Clear registry before test
	ClearRegistry()

	// Register multiple factories
	RegisterSubscriberFactory("console", func() SubscriberInterface {
		return &mockSubscriber{id: "console-1", enabled: true}
	})

	RegisterSubscriberFactory("file", func() SubscriberInterface {
		return &mockSubscriber{id: "file-1", enabled: true}
	})

	RegisterSubscriberFactory("otel", func() SubscriberInterface {
		return &mockSubscriber{id: "otel-1", enabled: true}
	})

	// Verify all were registered
	factories := getAllFactories()
	if len(factories) != 3 {
		t.Errorf("Expected 3 factories, got %d", len(factories))
	}

	// Verify we can get each one
	if GetFactory("console") == nil {
		t.Error("Expected console factory to be registered")
	}
	if GetFactory("file") == nil {
		t.Error("Expected file factory to be registered")
	}
	if GetFactory("otel") == nil {
		t.Error("Expected otel factory to be registered")
	}
}

// TestGetRegisteredNames tests getting all registered factory names
func TestGetRegisteredNames(t *testing.T) {
	// Clear registry before test
	ClearRegistry()

	// Register some factories
	RegisterSubscriberFactory("console", func() SubscriberInterface {
		return &mockSubscriber{id: "console-1"}
	})

	RegisterSubscriberFactory("file", func() SubscriberInterface {
		return &mockSubscriber{id: "file-1"}
	})

	names := GetRegisteredNames()
	if len(names) != 2 {
		t.Errorf("Expected 2 registered names, got %d", len(names))
	}

	// Verify names are present (order doesn't matter)
	hasConsole := false
	hasFile := false
	for _, name := range names {
		if name == "console" {
			hasConsole = true
		}
		if name == "file" {
			hasFile = true
		}
	}

	if !hasConsole {
		t.Error("Expected 'console' to be in registered names")
	}
	if !hasFile {
		t.Error("Expected 'file' to be in registered names")
	}
}

// TestGetFactoryNotFound tests getting a factory that doesn't exist
func TestGetFactoryNotFound(t *testing.T) {
	// Clear registry before test
	ClearRegistry()

	factory := GetFactory("nonexistent")
	if factory != nil {
		t.Error("Expected nil for nonexistent factory, got a factory")
	}
}

// TestClearRegistry tests clearing the registry
func TestClearRegistry(t *testing.T) {
	// Note: The actual subscriber init() functions have already registered factories
	// (console, file, otel), so we need to account for those
	initialCount := len(getAllFactories())

	// Register some test factories
	RegisterSubscriberFactory("test1", func() SubscriberInterface {
		return &mockSubscriber{id: "test1"}
	})

	RegisterSubscriberFactory("test2", func() SubscriberInterface {
		return &mockSubscriber{id: "test2"}
	})

	// Verify they are registered (initial + 2 new ones)
	if len(getAllFactories()) != initialCount+2 {
		t.Errorf("Expected %d factories before clear, got %d", initialCount+2, len(getAllFactories()))
	}

	// Clear the registry
	ClearRegistry()

	// Verify they are gone
	if len(getAllFactories()) != 0 {
		t.Errorf("Expected 0 factories after clear, got %d", len(getAllFactories()))
	}
}

// TestGetAllFactoriesReturnsCopy tests that getAllFactories returns a copy
func TestGetAllFactoriesReturnsCopy(t *testing.T) {
	// Clear registry before test
	ClearRegistry()

	// Register a factory
	RegisterSubscriberFactory("test", func() SubscriberInterface {
		return &mockSubscriber{id: "test"}
	})

	// Get all factories
	factories1 := getAllFactories()

	// Modify the returned map
	factories1["modified"] = func() SubscriberInterface {
		return &mockSubscriber{id: "modified"}
	}

	// Get all factories again
	factories2 := getAllFactories()

	// Verify the modification didn't affect the registry
	if len(factories2) != 1 {
		t.Errorf("Expected 1 factory in registry, got %d (modification leaked)", len(factories2))
	}

	if _, exists := factories2["modified"]; exists {
		t.Error("Expected 'modified' factory to not exist in registry (modification leaked)")
	}
}

// TestReplaceFactory tests replacing an existing factory
func TestReplaceFactory(t *testing.T) {
	// Clear registry before test
	ClearRegistry()

	// Register initial factory
	RegisterSubscriberFactory("test", func() SubscriberInterface {
		return &mockSubscriber{id: "original"}
	})

	// Verify original factory
	instance1 := GetFactory("test")()
	if instance1.GetID() != "original" {
		t.Errorf("Expected original ID, got %s", instance1.GetID())
	}

	// Replace with new factory
	RegisterSubscriberFactory("test", func() SubscriberInterface {
		return &mockSubscriber{id: "replaced"}
	})

	// Verify replaced factory
	instance2 := GetFactory("test")()
	if instance2.GetID() != "replaced" {
		t.Errorf("Expected replaced ID, got %s", instance2.GetID())
	}
}
