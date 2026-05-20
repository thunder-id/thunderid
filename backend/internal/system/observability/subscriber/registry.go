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
	"sync"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// SubscriberFactory is a function that creates a new subscriber instance.
// Factories are registered during package initialization (init()) and called
// later during service initialization when configuration is available.
type SubscriberFactory func() SubscriberInterface

// subscriberRegistry holds all registered subscriber factories.
// This is a package-level registry that stores factory functions (not instances).
// The registry is populated during init() and queried during service initialization.
// Initialized inline to ensure it's ready before any init() functions run.
var (
	factoryRegistry = make(map[string]SubscriberFactory)
	registryMu      sync.RWMutex
)

// RegisterSubscriberFactory registers a subscriber factory in the global registry.
// This should be called from each subscriber's init() function.
// The factory function will be called later when configuration is available.
//
// Parameters:
//   - name: Unique identifier for the subscriber type (e.g., "console", "file", "otel")
//   - factory: Function that creates a new subscriber instance
//
// Example:
//
//	func init() {
//	    RegisterSubscriberFactory("console", NewConsoleSubscriber)
//	}
func RegisterSubscriberFactory(name string, factory SubscriberFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()

	if _, exists := factoryRegistry[name]; exists {
		logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "SubscriberRegistry"))
		logger.Warn("Subscriber factory already registered, replacing",
			log.String("subscriberType", name))
	}

	factoryRegistry[name] = factory
}

// getAllFactories returns a copy of all registered subscriber factories.
// This is called during service initialization to instantiate subscribers.
// Returns a map of subscriber name -> factory function.
func getAllFactories() map[string]SubscriberFactory {
	registryMu.RLock()
	defer registryMu.RUnlock()

	// Return a copy to prevent external modification
	factories := make(map[string]SubscriberFactory, len(factoryRegistry))
	for name, factory := range factoryRegistry {
		factories[name] = factory
	}

	return factories
}

// GetFactory returns the factory for a specific subscriber type.
// Returns nil if the subscriber type is not registered.
func GetFactory(name string) SubscriberFactory {
	registryMu.RLock()
	defer registryMu.RUnlock()

	return factoryRegistry[name]
}

// GetRegisteredNames returns the names of all registered subscriber types.
// Useful for debugging and testing.
func GetRegisteredNames() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	names := make([]string, 0, len(factoryRegistry))
	for name := range factoryRegistry {
		names = append(names, name)
	}

	return names
}

// ClearRegistry removes all registered factories.
// This is primarily for testing purposes.
func ClearRegistry() {
	registryMu.Lock()
	defer registryMu.Unlock()

	factoryRegistry = make(map[string]SubscriberFactory)
}
