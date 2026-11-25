// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package httpconnection manages HTTP endpoint connections: a named outbound HTTP endpoint with its
// URL, timeout, plain headers and authentication, whose secrets are stored encrypted.
package httpconnection

import (
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
)

// Initialize creates the HTTP connection service over the store its configured mode selects.
//
// bus is required: the service publishes the audit event of every connection it creates, updates or
// deletes on it.
func Initialize(cfg config.HTTPConnectionConfig,
	ouService ou.OrganizationUnitServiceInterface, bus eventbus.EventBusInterface) (
	HTTPConnectionServiceInterface, error) {
	if bus == nil {
		return nil, errNilEventBus
	}
	mode, err := resolvedStoreMode(cfg)
	if err != nil {
		return nil, err
	}
	return newHTTPConnectionService(initializeStore(mode), ouService, bus), nil
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
