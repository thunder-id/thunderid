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
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// Initialize creates and initializes a new observability service instance.
// This function follows the dependency injection pattern used throughout the server.
// It reads configuration, initializes subscribers via the registry pattern,
// and returns a ready-to-use observability service.
//
// The service can be nil-safe injected into other services - if observability is disabled,
// the returned service will gracefully handle all operations as no-ops.
//
// Example usage:
//
//	observabilitySvc := observability.Initialize()
//	flowExecSvc := flowexec.Initialize(mux, flowMgt, app, execRegistry, observabilitySvc)
//
// Returns:
//   - ObservabilityServiceInterface: A new observability service instance
func Initialize() ObservabilityServiceInterface {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	logger.Debug("Initializing observability service")

	// Get configuration
	cfg := config.GetServerRuntime().Config.Observability

	if !cfg.Enabled {
		logger.Debug("Observability is disabled in configuration")
		// Return a disabled service (handles all operations as no-ops)
		return &Service{
			logger: logger,
			config: cfg,
		}
	}

	// Create the service with full initialization
	svc := newServiceWithConfig()

	// Log initialization status
	activeSubscribers := svc.GetActiveSubscribers()
	logger.Debug("Observability service initialized successfully",
		log.Int("activeSubscribers", len(activeSubscribers)))

	return svc
}
