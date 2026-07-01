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

package serverconfig

import (
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// getServerConfigStoreMode determines the store mode for server config.
//
// Resolution order:
//  1. If ServerConfig.Store is explicitly configured, use it; an unrecognized value is an error so a
//     misconfiguration fails fast at startup rather than silently changing store behavior.
//  2. Otherwise, fall back to the global declarative-resources setting: enabled → composite (declarative
//     defaults plus runtime overrides), disabled → mutable.
//
// The writable layer is present in both fallback modes (mutable and composite), so PUT is always allowed
// there; an explicit "declarative" makes the section read-only.
func getServerConfigStoreMode() (serverconst.StoreMode, error) {
	if store := config.GetServerRuntime().Config.ServerConfig.Store; store != "" {
		switch serverconst.StoreMode(strings.ToLower(strings.TrimSpace(store))) {
		case serverconst.StoreModeMutable:
			return serverconst.StoreModeMutable, nil
		case serverconst.StoreModeDeclarative:
			return serverconst.StoreModeDeclarative, nil
		case serverconst.StoreModeComposite:
			return serverconst.StoreModeComposite, nil
		default:
			return "", fmt.Errorf("serverconfig: invalid server_config.store %q", store)
		}
	}
	if declarativeresource.IsDeclarativeModeEnabled() {
		return serverconst.StoreModeComposite, nil
	}
	return serverconst.StoreModeMutable, nil
}
