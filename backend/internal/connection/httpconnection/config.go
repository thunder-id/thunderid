// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// ShouldLoadDeclarativeHTTPConnectionResources reports whether the file store is enabled.
func ShouldLoadDeclarativeHTTPConnectionResources() bool {
	mode, err := resolvedStoreMode(config.GetServerRuntime().Config.HTTPConnection)
	if err != nil {
		return false
	}
	return mode == serverconst.StoreModeDeclarative || mode == serverconst.StoreModeComposite
}

// resolvedStoreMode returns http_connection.store, or its declarative-mode default when unset. An
// unrecognized value is an error, so the server cannot start with a mistyped mode.
func resolvedStoreMode(cfg config.HTTPConnectionConfig) (serverconst.StoreMode, error) {
	mode := serverconst.StoreMode(strings.ToLower(strings.TrimSpace(cfg.Store)))
	switch mode {
	case "":
		if declarativeresource.IsDeclarativeModeEnabled() {
			return serverconst.StoreModeDeclarative, nil
		}
		return serverconst.StoreModeMutable, nil
	case serverconst.StoreModeMutable, serverconst.StoreModeDeclarative, serverconst.StoreModeComposite:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid http_connection store mode %q: must be one of %q, %q, or %q",
			cfg.Store, serverconst.StoreModeMutable, serverconst.StoreModeDeclarative,
			serverconst.StoreModeComposite)
	}
}
