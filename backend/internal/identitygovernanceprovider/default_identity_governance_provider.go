// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernanceprovider

import (
	"context"

	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// defaultIdentityGovernanceProvider forwards to the governance service. Access checks and RecordLogin
// run on a runtime context; lifecycle operations keep the caller's.
type defaultIdentityGovernanceProvider struct {
	governance identitygovernance.IdentityGovernanceServiceInterface
}

// newDefaultIdentityGovernanceProvider wraps the governance service as the runtime-facing port.
func newDefaultIdentityGovernanceProvider(
	governance identitygovernance.IdentityGovernanceServiceInterface,
) providers.IdentityGovernanceProvider {
	return &defaultIdentityGovernanceProvider{
		governance: governance,
	}
}

// EvaluateAccess forwards the check.
func (p *defaultIdentityGovernanceProvider) EvaluateAccess(ctx context.Context, entityID string,
	plane providers.AccessPlane,
	method providers.AuthenticationMethod) (providers.AccessDecision, *tidcommon.ServiceError) {
	return p.governance.EvaluateAccess(security.WithRuntimeContext(ctx), entityID, plane, method)
}

// AdmitSignIn admits a completing sign-in and clears a lapsed account-wide lock when admitted.
func (p *defaultIdentityGovernanceProvider) AdmitSignIn(ctx context.Context,
	entityID string) (providers.AccessDecision, *tidcommon.ServiceError) {
	return p.governance.AdmitSignIn(security.WithRuntimeContext(ctx), entityID)
}

// RecordLogin stamps when access was last granted.
func (p *defaultIdentityGovernanceProvider) RecordLogin(ctx context.Context,
	entityID string) *tidcommon.ServiceError {
	return p.governance.RecordLogin(security.WithRuntimeContext(ctx), entityID)
}

// Unlock releases every automatic lock.
func (p *defaultIdentityGovernanceProvider) Unlock(ctx context.Context,
	entityID string) *tidcommon.ServiceError {
	return p.governance.Unlock(ctx, entityID)
}

// Suspend places the hold, carrying the operator note through unchanged.
func (p *defaultIdentityGovernanceProvider) Suspend(ctx context.Context,
	entityID, operatorNote string) *tidcommon.ServiceError {
	return p.governance.Suspend(ctx, entityID, operatorNote)
}

// ValidateSuspend runs the suspend checks without writing.
func (p *defaultIdentityGovernanceProvider) ValidateSuspend(ctx context.Context,
	entityID, operatorNote string) *tidcommon.ServiceError {
	return p.governance.ValidateSuspend(ctx, entityID, operatorNote)
}

// Unsuspend releases the hold.
func (p *defaultIdentityGovernanceProvider) Unsuspend(ctx context.Context,
	entityID string) *tidcommon.ServiceError {
	return p.governance.Unsuspend(ctx, entityID)
}
