// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package httpconnection manages HTTP endpoint connections: a named outbound HTTP endpoint with its
// URL, timeout, plain headers and authentication, whose secrets are stored encrypted.
package httpconnection

import (
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
)

// Initialize creates the HTTP connection service over the store its configured mode selects.
func Initialize(cfg config.HTTPConnectionConfig,
	ouService ou.OrganizationUnitServiceInterface) (HTTPConnectionServiceInterface, error) {
	mode, err := resolvedStoreMode(cfg)
	if err != nil {
		return nil, err
	}
	return newHTTPConnectionService(initializeStore(mode), ouService), nil
}

// initializeStore selects the mutable, declarative, or composite connection store.
func initializeStore(mode serverconst.StoreMode) httpConnectionStoreInterface {
	switch mode {
	case serverconst.StoreModeDeclarative:
		return newFileBasedStore()
	case serverconst.StoreModeComposite:
		return newCompositeStore(newFileBasedStore(), newHTTPConnectionStore())
	default:
		return newHTTPConnectionStore()
	}
}
