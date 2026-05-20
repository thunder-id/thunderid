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

package layoutmgt

import (
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// getLayoutStoreMode determines the store mode for layouts.
//
// Resolution order:
//  1. If Layout.Store is explicitly configured, use it
//  2. Otherwise, fall back to global DeclarativeResources.Enabled:
//     - If enabled: return "declarative"
//     - If disabled: return "mutable"
//
// Returns normalized store mode: "mutable", "declarative", or "composite"
func getLayoutStoreMode() serverconst.StoreMode {
	cfg := config.GetServerRuntime().Config
	// Check if service-level configuration is explicitly set
	if cfg.Layout.Store != "" {
		mode := serverconst.StoreMode(strings.ToLower(strings.TrimSpace(cfg.Layout.Store)))
		// Validate and normalize
		switch mode {
		case serverconst.StoreModeMutable, serverconst.StoreModeDeclarative, serverconst.StoreModeComposite:
			return mode
		default:
			// Warn about unrecognized value and fall back to default
			logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "LayoutMgtConfig"))
			logger.Warn("Unrecognized layout store configuration value",
				log.String("raw_value", cfg.Layout.Store),
				log.String("normalized_value", string(mode)),
				log.String("fallback", "global declarative_resources setting"))
		}
	}

	// Fall back to global declarative resources setting
	if declarativeresource.IsDeclarativeModeEnabled() {
		return serverconst.StoreModeDeclarative
	}

	return serverconst.StoreModeMutable
}

// isDeclarativeModeEnabled checks if immutable-only store mode is enabled for layouts.
func isDeclarativeModeEnabled() bool {
	return getLayoutStoreMode() == serverconst.StoreModeDeclarative
}
