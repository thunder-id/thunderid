// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// getStoreMode reports the template store mode. An explicit notification.template.store wins and an
// unrecognized explicit value is a hard error so the server cannot boot silently with a mistyped mode;
// an unset value follows the global declarative setting.
func getStoreMode() (serverconst.StoreMode, error) {
	cfg := config.GetServerRuntime().Config
	if cfg.Notification.Template.Store != "" {
		mode := serverconst.StoreMode(strings.ToLower(strings.TrimSpace(cfg.Notification.Template.Store)))
		switch mode {
		case serverconst.StoreModeMutable, serverconst.StoreModeDeclarative, serverconst.StoreModeComposite:
			return mode, nil
		default:
			return "", fmt.Errorf("invalid notification.template.store %q: must be one of %q, %q, or %q",
				cfg.Notification.Template.Store,
				serverconst.StoreModeMutable, serverconst.StoreModeDeclarative, serverconst.StoreModeComposite)
		}
	}

	if declarativeresource.IsDeclarativeModeEnabled() {
		return serverconst.StoreModeDeclarative, nil
	}

	return serverconst.StoreModeMutable, nil
}

// isDeclarativeModeEnabled reports whether templates run in declarative-only mode, where the store is
// read-only and templates cannot be created through the API.
func isDeclarativeModeEnabled() bool {
	mode, err := getStoreMode()
	return err == nil && mode == serverconst.StoreModeDeclarative
}
