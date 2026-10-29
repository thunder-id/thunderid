// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernanceprovider

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// disabledIdentityGovernanceProvider permits access without storing governance state.
type disabledIdentityGovernanceProvider struct{}

// newDisabledIdentityGovernanceProvider creates a provider for deployments without governance.
func newDisabledIdentityGovernanceProvider() providers.IdentityGovernanceProvider {
	return &disabledIdentityGovernanceProvider{}
}

// EvaluateAccess admits every check.
func (p *disabledIdentityGovernanceProvider) EvaluateAccess(_ context.Context, _ string,
	_ providers.AccessPlane, _ providers.AuthenticationMethod) (providers.AccessDecision, *tidcommon.ServiceError) {
	return providers.AccessDecision{Admitted: true}, nil
}

// AdmitSignIn admits and clears nothing, as EvaluateAccess does.
func (p *disabledIdentityGovernanceProvider) AdmitSignIn(_ context.Context,
	_ string) (providers.AccessDecision, *tidcommon.ServiceError) {
	return providers.AccessDecision{Admitted: true}, nil
}

// RecordLogin records nothing.
func (p *disabledIdentityGovernanceProvider) RecordLogin(_ context.Context, _ string,
) *tidcommon.ServiceError {
	return nil
}

// Unlock is unavailable without governance.
func (p *disabledIdentityGovernanceProvider) Unlock(_ context.Context, _ string,
) *tidcommon.ServiceError {
	return &errGovernanceUnavailable
}

// Suspend is unavailable without governance.
func (p *disabledIdentityGovernanceProvider) Suspend(_ context.Context, _, _ string,
) *tidcommon.ServiceError {
	return &errGovernanceUnavailable
}

// ValidateSuspend is unavailable without governance.
func (p *disabledIdentityGovernanceProvider) ValidateSuspend(_ context.Context, _, _ string,
) *tidcommon.ServiceError {
	return &errGovernanceUnavailable
}

// Unsuspend is unavailable without governance.
func (p *disabledIdentityGovernanceProvider) Unsuspend(_ context.Context, _ string,
) *tidcommon.ServiceError {
	return &errGovernanceUnavailable
}
