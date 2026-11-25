// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
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

// unusedValueGrace is how long a value nothing refers to is kept on a gateway after it was last set,
// and whether such values are removed at all. Without a server runtime nothing is removed. A grace
// below the minimum is raised to it, with a warning, since the gateway needs a real age to compare.
func unusedValueGrace(ctx context.Context) (time.Duration, bool) {
	if !config.IsServerRuntimeInitialized() {
		return 0, false
	}
	gateway := config.GetServerRuntime().Config.Gateway
	grace, on := gateway.UnusedValueGrace()
	if on && gateway.UnusedValueGraceRaised() {
		log.GetLogger().Warn(ctx, "gateway.unused_value_grace_seconds is below the minimum and was raised to it; "+
			"set a negative number to turn the removal of unused values off",
			log.Int("minimumSeconds", config.MinUnusedValueGraceSeconds))
	}
	return grace, on
}

// unusedValueSweepInterval is the least time between two sweeps of the same gateway's store; zero
// sweeps after every apply.
func unusedValueSweepInterval() time.Duration {
	if !config.IsServerRuntimeInitialized() {
		return 0
	}
	return config.GetServerRuntime().Config.Gateway.UnusedValueSweepInterval()
}
