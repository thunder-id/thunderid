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

package application

import (
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// getApplicationStoreMode determines the store mode for applications.
//
// Resolution order:
//  1. If Application.Store is explicitly configured, use it
//  2. Otherwise, fall back to global DeclarativeResources.Enabled:
//     - If enabled: return "declarative"
//     - If disabled: return "mutable"
//
// Returns normalized store mode: "mutable", "declarative", or "composite"
func getApplicationStoreMode() serverconst.StoreMode {
	cfg := config.GetServerRuntime().Config
	// Check if service-level configuration is explicitly set
	if cfg.Application.Store != "" {
		mode := serverconst.StoreMode(strings.ToLower(strings.TrimSpace(cfg.Application.Store)))
		// Validate and normalize
		switch mode {
		case serverconst.StoreModeMutable, serverconst.StoreModeDeclarative, serverconst.StoreModeComposite:
			return mode
		}
	}

	// Fall back to global declarative resources setting
	if declarativeresource.IsDeclarativeModeEnabled() {
		return serverconst.StoreModeDeclarative
	}

	return serverconst.StoreModeMutable
}

// isDeclarativeModeEnabled checks if immutable-only store mode is enabled for applications.
func isDeclarativeModeEnabled() bool {
	return getApplicationStoreMode() == serverconst.StoreModeDeclarative
}

// TODO: Move this to application config (similar to user indexed attributes)
// getAppIndexedAttributes returns the entity attribute names that applications need indexed
// for fast lookups.
func getAppIndexedAttributes() []string {
	return []string{"name", "clientId"}
}
