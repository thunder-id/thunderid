// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package httpconnection manages HTTP endpoint connections: a named outbound HTTP endpoint with its
// URL, timeout, plain headers and authentication, whose secrets are stored encrypted.
package httpconnection

import (
	"github.com/thunder-id/thunderid/internal/connection/connectionou"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
)

// Initialize creates the HTTP connection service over the store its configured mode selects.
func Initialize(cfg config.HTTPConnectionConfig, ouService ou.OrganizationUnitServiceInterface,
	authzService sysauthz.SystemAuthorizationServiceInterface) (HTTPConnectionServiceInterface, error) {
	mode, err := resolvedStoreMode(cfg)
	if err != nil {
		return nil, err
	}
	ouResolver := connectionou.Initialize(ouService, authzService, connectionou.Errors{
		Required: connectionou.ErrorOURequired,
		NotFound: connectionou.ErrorInvalidOU,
		Changed:  connectionou.ErrorOUUpdateNotAllowed,
	})
	return newHTTPConnectionService(initializeStore(mode), ouResolver), nil
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
