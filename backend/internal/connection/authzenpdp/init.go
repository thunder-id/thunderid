// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package authzenpdp manages AuthZEN PDP connections.
package authzenpdp

import (
	"strings"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// Initialize creates the AuthZEN PDP connection service with its production store.
func Initialize(
	defaults config.AuthZENPDPConfig,
	entityTypes entitytype.EntityTypeServiceInterface,
) (AuthZENPDPServiceInterface, error) {
	transactioner, err := provider.GetDBProvider().GetConfigDBTransactioner()
	if err != nil {
		return nil, err
	}
	return newAuthZENPDPService(initializeStore(defaults), defaults, transactioner, entityTypes), nil
}

// ShouldLoadDeclarativeAuthZENPDPResources reports whether the file store is enabled.
func ShouldLoadDeclarativeAuthZENPDPResources() bool {
	mode := resolvedStoreMode(config.GetServerRuntime().Config.AuthZENPDP)
	return mode == serverconst.StoreModeDeclarative || mode == serverconst.StoreModeComposite
}

// initializeStore selects the mutable, declarative, or composite connection store.
func initializeStore(defaults config.AuthZENPDPConfig) authZENPDPStoreInterface {
	mode := resolvedStoreMode(defaults)
	switch mode {
	case serverconst.StoreModeDeclarative:
		return newFileBasedStore()
	case serverconst.StoreModeComposite:
		return newCompositeStore(newFileBasedStore(), newAuthZENPDPStore())
	default:
		return newAuthZENPDPStore()
	}
}

// resolvedStoreMode returns the configured store mode or its declarative-mode default.
func resolvedStoreMode(defaults config.AuthZENPDPConfig) serverconst.StoreMode {
	mode := strings.ToLower(strings.TrimSpace(defaults.Store))
	if mode == "" {
		if declarativeresource.IsDeclarativeModeEnabled() {
			mode = string(serverconst.StoreModeDeclarative)
		} else {
			mode = string(serverconst.StoreModeMutable)
		}
	}
	return serverconst.StoreMode(mode)
}
