// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package identitygovernanceprovider adapts the governance service to the runtime-facing governance
// port. It grants runtime privilege for access checks, and supplies a disabled provider for
// deployments that turn identity governance off.
package identitygovernanceprovider

import (
	"context"

	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the identity governance provider of the configured type.
func Initialize(
	governance identitygovernance.IdentityGovernanceServiceInterface,
	cfg config.IdentityGovernanceProviderConfig,
) providers.IdentityGovernanceProvider {
	switch cfg.Type {
	case providerTypeDisabled:
		logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "IdentityGovernanceProvider"))
		logger.Debug(context.Background(), "Identity governance provider is disabled")
		return newDisabledIdentityGovernanceProvider()
	default:
		return newDefaultIdentityGovernanceProvider(governance)
	}
}
