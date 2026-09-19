// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

// Package authzenpdp manages AuthZEN PDP connections.

import (
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// Initialize creates the AuthZEN PDP connection service with its production store.
func Initialize(defaults config.AuthZENPDPConfig, deploymentID string) AuthZENPDPServiceInterface {
	return NewAuthZENPDPService(initializeStore(defaults, deploymentID), defaults)
}

func initializeStore(defaults config.AuthZENPDPConfig, deploymentID string) Store {
	mode := strings.ToLower(strings.TrimSpace(defaults.Store))
	if mode == "" {
		if declarativeresource.IsDeclarativeModeEnabled() {
			mode = string(serverconst.StoreModeDeclarative)
		} else {
			mode = string(serverconst.StoreModeMutable)
		}
	}
	switch serverconst.StoreMode(mode) {
	case serverconst.StoreModeDeclarative:
		return newFileBasedStore()
	case serverconst.StoreModeComposite:
		return newCompositeStore(newFileBasedStore(), NewStore(deploymentID))
	default:
		return NewStore(deploymentID)
	}
}
