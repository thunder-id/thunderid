// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// getGatewayStoreMode reports where gateways are read from and written to.
//
// An explicit gateway.store wins. Without one this follows the global declarative setting, so a
// deployment that turned declarative resources on does not have to name every resource type again.
func getGatewayStoreMode() serverconst.StoreMode {
	cfg := config.GetServerRuntime().Config
	if cfg.Gateway.Store != "" {
		mode := serverconst.StoreMode(strings.ToLower(strings.TrimSpace(cfg.Gateway.Store)))
		switch mode {
		case serverconst.StoreModeMutable, serverconst.StoreModeDeclarative, serverconst.StoreModeComposite:
			return mode
		}
	}

	if declarativeresource.IsDeclarativeModeEnabled() {
		return serverconst.StoreModeDeclarative
	}

	return serverconst.StoreModeMutable
}
