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

// Package subscriber manages the lifecycle and registration of observability subscribers.
// It implements a factory registry pattern to allow subscribers to self-register
// and be dynamically initialized based on configuration.
package subscriber

import (
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const initComponentName = "SubscriberInit"

// Initialize discovers and initializes all registered subscribers based on configuration.
// This function is called by the observability service during application startup,
// specifically after the configuration has been loaded.
//
// Lifecycle & Registry Pattern:
//  1. Discovery: Retrieves all registered subscriber factories from the global registry.
//     Subscribers register themselves via init() functions using RegisterFactory().
//  2. Creation: For each registered factory, a new subscriber instance is created.
//  3. Configuration Check (IsEnabled): The instance is checked against the provided
//     configuration to see if it should be active. This allows subscribers to be
//     conditionally enabled/disabled via config (e.g., deployment.yaml).
//  4. Initialization (Initialize): If enabled, the subscriber's Initialize() method
//     is called to set up necessary resources (connections, files, etc.).
//  5. Collection: Successfully initialized subscribers are returned to be attached
//     to the Event Bus.
//
// Parameters:
//   - observabilityConfig: The observability configuration from deployment.yaml
//
// Returns:
//   - []SubscriberInterface: List of successfully initialized and enabled subscribers
//   - error: Error if initialization fails in strict mode, nil otherwise
//
// Error Handling:
//   - In "strict" failure mode: Returns error on first initialization failure
//   - In "lenient" failure mode: Logs warning and continues with remaining subscribers
func Initialize(observabilityConfig config.ObservabilityConfig) ([]SubscriberInterface, error) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, initComponentName))

	// Get all registered factories
	factories := getAllFactories()

	if len(factories) == 0 {
		logger.Warn("No subscriber factories registered, observability will have no outputs")
		return []SubscriberInterface{}, nil
	}

	logger.Debug("Initializing subscribers from registry",
		log.Int("factoryCount", len(factories)))

	activeSubscribers := make([]SubscriberInterface, 0, len(factories))
	var initErrors []error

	// Iterate through all registered factories
	for name, factory := range factories {
		logger.Debug("Processing subscriber factory",
			log.String("subscriberType", name))

		// Create subscriber instance
		instance := factory()
		if instance == nil {
			logger.Error("Factory returned nil instance",
				log.String("subscriberType", name))
			initErrors = append(initErrors, fmt.Errorf("factory %s returned nil instance", name))
			continue
		}

		// Check if subscriber is enabled in configuration
		if !instance.IsEnabled() {
			logger.Debug("Subscriber disabled in configuration, skipping",
				log.String("subscriberType", name))
			continue
		}

		// Initialize the subscriber (setup resources, validate config, etc.)
		if err := instance.Initialize(); err != nil {
			logger.Error("Failed to initialize subscriber",
				log.String("subscriberType", name),
				log.Error(err))

			initErrors = append(initErrors, fmt.Errorf("failed to initialize %s subscriber: %w", name, err))

			// In strict mode, fail fast on first error
			if observabilityConfig.FailureMode == "strict" {
				return nil, fmt.Errorf("subscriber initialization failed in strict mode: %w", err)
			}

			// In lenient mode, log and continue
			logger.Warn("Continuing subscriber initialization despite error (lenient mode)",
				log.String("subscriberType", name))
			continue
		}

		// Successfully initialized - add to active list
		activeSubscribers = append(activeSubscribers, instance)
		logger.Debug("Subscriber initialized successfully",
			log.String("subscriberType", name),
			log.String("subscriberID", instance.GetID()))
	}

	logger.Debug("Subscriber initialization complete",
		log.Int("total", len(factories)),
		log.Int("active", len(activeSubscribers)),
		log.Int("errors", len(initErrors)))

	// If we have errors but reached here, we're in lenient mode
	if len(initErrors) > 0 && observabilityConfig.FailureMode != "strict" {
		logger.Warn("Some subscribers failed to initialize but continuing in lenient mode",
			log.Int("failedCount", len(initErrors)))
	}

	return activeSubscribers, nil
}
