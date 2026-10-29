// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"cmp"
	"context"

	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// accessGuard asks the identity governance provider whether an identity may be used, and maps a
// refusal to the flow error that states its hold. A nil provider disables the checks. It logs through
// the calling executor's logger.
type accessGuard struct {
	governance providers.IdentityGovernanceProvider
}

// newAccessGuard builds the guard over the governance provider.
func newAccessGuard(governance providers.IdentityGovernanceProvider) *accessGuard {
	return &accessGuard{governance: governance}
}

// heldAt reports whether the identity is held at the plane, plus the refusal stating the hold, or nil
// when the decision names none. It never counts, and an unreadable authority returns a fault.
func (g *accessGuard) heldAt(ctx context.Context, entityID string, plane providers.AccessPlane,
	method providers.AuthenticationMethod, logger *log.Logger) (bool, *tidcommon.ServiceError, error) {
	if g.governance == nil || entityID == "" {
		return false, nil, nil
	}

	decision, svcErr := g.governance.EvaluateAccess(ctx, entityID, plane, method)
	if svcErr != nil {
		return false, nil, governanceFault(svcErr)
	}
	if decision.Admitted {
		return false, nil, nil
	}
	logger.Debug(ctx, "Refused a held identity", log.MaskedString(log.LoggerKeyEntityID, entityID),
		log.String("plane", string(plane)), log.String("authenticationMethod", string(method)))
	return true, statedDecision(decision), nil
}

// refuseIfHeld returns the refusal for a held identity, or nil when it may be used. A refusal that
// names no hold answers as an invalid credential.
func (g *accessGuard) refuseIfHeld(ctx context.Context, entityID string, plane providers.AccessPlane,
	method providers.AuthenticationMethod, logger *log.Logger) (*tidcommon.ServiceError, error) {
	held, stated, err := g.heldAt(ctx, entityID, plane, method, logger)
	if err != nil || !held {
		return nil, err
	}
	return cmp.Or(stated, &ErrInvalidCredentials), nil
}

// refuseSignInIfHeld is refuseIfHeld at the session plane for a completing sign-in, through
// AdmitSignIn, which also clears a lapsed account-wide failure record when admitted.
func (g *accessGuard) refuseSignInIfHeld(ctx context.Context, entityID string,
	logger *log.Logger) (*tidcommon.ServiceError, error) {
	if g.governance == nil || entityID == "" {
		return nil, nil
	}

	decision, svcErr := g.governance.AdmitSignIn(ctx, entityID)
	if svcErr != nil {
		return nil, governanceFault(svcErr)
	}
	if decision.Admitted {
		return nil, nil
	}
	logger.Debug(ctx, "Refused a held identity", log.MaskedString(log.LoggerKeyEntityID, entityID),
		log.String("plane", string(providers.AccessPlaneSession)))
	return cmp.Or(statedDecision(decision), &ErrInvalidCredentials), nil
}

// recordLogin stamps when the identity was last granted access.
func (g *accessGuard) recordLogin(ctx context.Context, entityID string) {
	if g.governance == nil || entityID == "" {
		return
	}
	_ = g.governance.RecordLogin(ctx, entityID)
}

// heldAtDispatch is heldAt for a challenge about to be sent, checked against the challenge's own
// method at the plane its flow type maps to.
func (g *accessGuard) heldAtDispatch(ctx context.Context, entityID string, method providers.AuthenticationMethod,
	flowType providers.FlowType, logger *log.Logger) (bool, *tidcommon.ServiceError, error) {
	return g.heldAt(ctx, entityID, dispatchPlaneFor(flowType), method, logger)
}
