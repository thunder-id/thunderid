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

// refuseSignInIfHeld returns the refusal for a held identity at the session plane for a completing
// sign-in, or nil when it may be used. A refusal that names no hold answers as an invalid credential.
// AdmitSignIn also clears a lapsed entity-wide failure record when admitted.
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
