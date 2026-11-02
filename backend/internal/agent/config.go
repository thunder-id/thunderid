// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// getAgentStoreMode determines the store mode for agents from config.
//
// Resolution order:
//  1. If Agent.Store is explicitly configured, use it.
//  2. Otherwise fall back to global DeclarativeResources.Enabled:
//     - If enabled: return "declarative"
//     - If disabled: return "mutable"
func getAgentStoreMode() serverconst.StoreMode {
	store := strings.ToLower(strings.TrimSpace(config.GetServerRuntime().Config.Agent.Store))
	switch serverconst.StoreMode(store) {
	case serverconst.StoreModeMutable, serverconst.StoreModeDeclarative, serverconst.StoreModeComposite:
		return serverconst.StoreMode(store)
	}
	if declarativeresource.IsDeclarativeModeEnabled() {
		return serverconst.StoreModeDeclarative
	}
	return serverconst.StoreModeMutable
}

// getAgentIndexedAttributes returns the entity attribute names that agents need indexed for fast
// lookups and schema uniqueness checks.
func getAgentIndexedAttributes() []string {
	return []string{"name"}
}
