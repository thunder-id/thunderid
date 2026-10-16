// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package identitygovernanceprovider adapts the governance service to the runtime-facing governance
// port. It grants runtime privilege for access checks, and supplies a disabled provider when no
// service is configured.
package identitygovernanceprovider

import (
	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// InitializeIdentityGovernanceProvider returns the runtime-facing governance port. A nil service
// disables access checks, and lifecycle operations are unavailable.
func InitializeIdentityGovernanceProvider(
	governance identitygovernance.IdentityGovernanceServiceInterface,
) providers.IdentityGovernanceProvider {
	if governance == nil {
		return newDisabledIdentityGovernanceProvider()
	}
	return newDefaultIdentityGovernanceProvider(governance)
}
