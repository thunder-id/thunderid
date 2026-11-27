// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// getGroupStoreMode determines the store mode for groups.
//
// Resolution order:
//  1. If Group.Store is explicitly configured, validate and use it — an
//     unrecognized value is a hard error so the server cannot boot silently
//     with a mistyped mode.
//  2. Otherwise, fall back to global DeclarativeResources.Enabled:
//     - If enabled: return "declarative"
//     - If disabled: return "mutable"
//
// Returns normalized store mode: "mutable", "declarative", or "composite".
func getGroupStoreMode() (serverconst.StoreMode, error) {
	cfg := config.GetServerRuntime().Config
	if cfg.Group.Store != "" {
		mode := serverconst.StoreMode(strings.ToLower(strings.TrimSpace(cfg.Group.Store)))
		switch mode {
		case serverconst.StoreModeMutable, serverconst.StoreModeDeclarative, serverconst.StoreModeComposite:
			return mode, nil
		default:
			return "", fmt.Errorf("invalid group store mode %q: must be one of %q, %q, or %q",
				cfg.Group.Store,
				serverconst.StoreModeMutable,
				serverconst.StoreModeDeclarative,
				serverconst.StoreModeComposite,
			)
		}
	}

	if declarativeresource.IsDeclarativeModeEnabled() {
		return serverconst.StoreModeDeclarative, nil
	}

	return serverconst.StoreModeMutable, nil
}

// isGroupDeclarativeModeEnabled checks if immutable-only store mode is enabled for groups.
// Returns false on invalid configuration — startup validation in initializeGroupStore will
// have already caught it.
func isGroupDeclarativeModeEnabled() bool {
	mode, err := getGroupStoreMode()
	if err != nil {
		return false
	}
	return mode == serverconst.StoreModeDeclarative
}
