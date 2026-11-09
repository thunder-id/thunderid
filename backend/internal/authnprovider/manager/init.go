// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize creates a new AuthnProviderManager. defaultProvider is required. bus is required: the
// manager publishes the audit event of every credential it enrolls on it.
func Initialize(defaultProvider providers.AuthnProviderInterface,
	others map[string]providers.CustomAuthnProvider, bus eventbus.EventBusInterface) (
	providers.AuthnProviderManager, error) {
	if bus == nil {
		return nil, errNilEventBus
	}
	return newAuthnProviderManager(defaultProvider, others, bus)
}
