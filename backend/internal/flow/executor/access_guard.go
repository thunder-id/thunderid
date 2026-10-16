// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"fmt"

	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// accessGuard is the flow executors' shared way of asking whether an identity may be used. Each
// placement differs only in the plane it names; what a plane withholds is the authority's decision.
type accessGuard struct {
	governance providers.IdentityGovernanceProvider
	logger     *log.Logger
}

// newAccessGuard builds the guard. A nil provider disables governance checks.
func newAccessGuard(governance providers.IdentityGovernanceProvider,
	logger *log.Logger) *accessGuard {
	return &accessGuard{governance: governance, logger: logger}
}

// accountHold is the hold a refusal states, or holdNone when it names none.
type accountHold int

const (
	holdNone accountHold = iota
	holdLocked
	holdSuspended
)

// statedHold classifies an error raised by the authentication provider manager. Every executor uses
// it, so none compares hold codes itself.
func statedHold(svcErr *tidcommon.ServiceError) accountHold {
	if svcErr == nil {
		return holdNone
	}
	switch svcErr.Code {
	case authnprovidermgr.ErrorAccountLocked.Code:
		return holdLocked
	case authnprovidermgr.ErrorAccountSuspended.Code:
		return holdSuspended
	default:
		return holdNone
	}
}

// refusal is the flow error a stated hold carries, and nil when no hold is named.
func (h accountHold) refusal() *tidcommon.ServiceError {
	switch h {
	case holdLocked:
		return &ErrAccountLocked
	case holdSuspended:
		return &ErrAccountSuspended
	default:
		return nil
	}
}

// statedDecision is accountHold for a decision the guard holds directly. A decision states its hold
// through its reason.
func statedDecision(decision providers.AccessDecision) accountHold {
	switch decision.Reason {
	case providers.AccessRefusalReasonSuspended:
		return holdSuspended
	case providers.AccessRefusalReasonLocked:
		return holdLocked
	default:
		return holdNone
	}
}

// refuseIfHeld returns the error a held identity produces, or nil when it may be used. The refusal
// names the hold, or answers as an invalid credential when the decision names none. An unreadable
// authority returns a fault, not a hold.
func (g *accessGuard) refuseIfHeld(ctx context.Context, entityID string,
	plane providers.AccessPlane, scope providers.AccessScope) (*tidcommon.ServiceError, error) {
	if g == nil || g.governance == nil || entityID == "" {
		return nil, nil
	}

	decision, svcErr := g.governance.EvaluateAccess(ctx, entityID, plane, scope)
	return g.refusal(ctx, entityID, string(plane), decision, svcErr)
}

// refuseSignInIfHeld is refuseIfHeld at the session plane for a sign-in that is completing. When
// admitted, the authority also clears a lapsed entity-wide failure record.
func (g *accessGuard) refuseSignInIfHeld(ctx context.Context,
	entityID string) (*tidcommon.ServiceError, error) {
	if g == nil || g.governance == nil || entityID == "" {
		return nil, nil
	}

	decision, svcErr := g.governance.AdmitSignIn(ctx, entityID)
	return g.refusal(ctx, entityID, string(providers.AccessPlaneSession), decision, svcErr)
}

// refusal maps a decision to the guard's refusal, or a fault when the authority could not decide.
func (g *accessGuard) refusal(ctx context.Context, entityID, plane string, decision providers.AccessDecision,
	svcErr *tidcommon.ServiceError) (*tidcommon.ServiceError, error) {
	if svcErr != nil {
		return nil, governanceFault(svcErr)
	}
	if decision.Admitted {
		return nil, nil
	}
	g.logger.Debug(ctx, "Refused a held identity",
		log.MaskedString("entityId", entityID), log.String("plane", plane))
	if stated := statedDecision(decision).refusal(); stated != nil {
		return stated, nil
	}
	return &ErrInvalidCredentials, nil
}

// isStatedRefusal reports whether a guard refusal states the hold rather than answering as an invalid
// credential.
func isStatedRefusal(svcErr *tidcommon.ServiceError) bool {
	return svcErr != nil && (svcErr.Code == ErrAccountLocked.Code || svcErr.Code == ErrAccountSuspended.Code)
}

// recordLogin stamps when the identity was last granted access. A failure is logged by the authority
// and changes nothing here.
func (g *accessGuard) recordLogin(ctx context.Context, entityID string) {
	if g == nil || g.governance == nil || entityID == "" {
		return
	}
	_ = g.governance.RecordLogin(ctx, entityID)
}

// governanceFault is the error a gate raises when the authority could not decide.
func governanceFault(svcErr *tidcommon.ServiceError) error {
	return fmt.Errorf("identity governance check failed: %s", svcErr.Error.DefaultValue)
}

// heldAtDispatch reports whether a challenge must not be sent, checked against the challenge's own
// scope, plus the refusal stating the hold. With no refusal (a decision that names no hold) the caller
// answers as for an unknown user. It never counts, and an unreadable authority returns a fault.
func (g *accessGuard) heldAtDispatch(ctx context.Context, entityID string, scope providers.AccessScope,
	flowType providers.FlowType) (bool, *tidcommon.ServiceError, error) {
	if g == nil || g.governance == nil || entityID == "" {
		return false, nil, nil
	}

	decision, svcErr := g.governance.EvaluateAccess(ctx, entityID, dispatchPlaneFor(flowType), scope)
	if svcErr != nil {
		return false, nil, governanceFault(svcErr)
	}
	if !decision.Admitted {
		g.logger.Debug(ctx, "Withheld a challenge for a held identity",
			log.MaskedString("entityId", entityID), log.String("scope", string(scope)))
		return true, statedDecision(decision).refusal(), nil
	}
	return false, nil, nil
}

// dispatchPlaneFor maps the flow the dispatch belongs to onto the plane it is checked at. A lock
// withholds nothing at the recovery plane, so recovery and registration challenges still go out.
func dispatchPlaneFor(flowType providers.FlowType) providers.AccessPlane {
	if flowType == providers.FlowTypeAuthentication {
		return providers.AccessPlaneDispatch
	}
	return providers.AccessPlaneRecovery
}
