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

// EvaluateAccess forwards the check. A failure refuses, since the caller cannot tell it from a hold.
func (p *defaultIdentityGovernanceProvider) EvaluateAccess(ctx context.Context, entityID string,
	plane providers.AccessPlane, scope providers.AccessScope) (providers.AccessDecision, *tidcommon.ServiceError) {
	decision, svcErr := p.governance.EvaluateAccess(security.WithRuntimeContext(ctx), entityID, plane, scope)
	return refuseOnFailure(decision, svcErr)
}

// AdmitSignIn admits a completing sign-in and clears a lapsed entity-wide lock when admitted. A
// failure refuses, as for EvaluateAccess.
func (p *defaultIdentityGovernanceProvider) AdmitSignIn(ctx context.Context,
	entityID string) (providers.AccessDecision, *tidcommon.ServiceError) {
	decision, svcErr := p.governance.AdmitSignIn(security.WithRuntimeContext(ctx), entityID)
	return refuseOnFailure(decision, svcErr)
}

// RecordLogin stamps when access was last granted. It always returns nil: a failed stamp must never
// refuse a grant. The service logs the failure.
func (p *defaultIdentityGovernanceProvider) RecordLogin(ctx context.Context,
	entityID string) *tidcommon.ServiceError {
	_ = p.governance.RecordLogin(security.WithRuntimeContext(ctx), entityID)
	return nil
}

// Unlock releases every automatic lock. Lifecycle operations run on the caller's context, so the
// service can check the caller's organization-unit boundary.
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
	entityID string) *tidcommon.ServiceError {
	return p.governance.ValidateSuspend(ctx, entityID)
}

// Unsuspend releases the hold.
func (p *defaultIdentityGovernanceProvider) Unsuspend(ctx context.Context,
	entityID string) *tidcommon.ServiceError {
	return p.governance.Unsuspend(ctx, entityID)
}

// refuseOnFailure turns a service failure into a refusal, returning the failure unchanged.
func refuseOnFailure(decision providers.AccessDecision,
	svcErr *tidcommon.ServiceError) (providers.AccessDecision, *tidcommon.ServiceError) {
	if svcErr == nil {
		return decision, nil
	}
	return providers.AccessDecision{}, svcErr
}
