// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package defaultprovider

import (
	"context"

	"github.com/thunder-id/thunderid/internal/authn/passkey"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// attributedFailure identifies the subject of a verified proof mismatch.
type attributedFailure struct {
	// entityID is the subject the authentication method compared against. Preferred: it needs no
	// second lookup and is never re-derived from caller input.
	entityID string

	// recipient names the subject indirectly, for a method that holds an identifier, not an entity.
	// Read only when entityID is empty, and never built from a caller-supplied identifier.
	recipient map[string]interface{}
}

// recordAuthenticationFailure counts one failed proof against the identity that presented it.
func (p *defaultAuthnProvider) recordAuthenticationFailure(ctx context.Context,
	scope providers.AccessScope, failure *attributedFailure, svcErr *tidcommon.ServiceError) {
	if p.governance == nil || failure == nil {
		return
	}
	if !isAttributableAuthenticationError(svcErr) {
		// A server fault, an invalid request or an unknown subject is not a failed proof, whatever
		// evidence accompanied it.
		return
	}

	if scope == scopeNotAnAuthenticationEvent {
		return
	}

	entityID := p.resolveFailedSubject(ctx, failure)
	if entityID == "" {
		return
	}

	// A failure to count is logged by the service and never changes the authentication answer.
	_ = p.governance.RecordFailure(security.WithRuntimeContext(ctx), entityID, scope)
}

// isAttributableAuthenticationError reports whether the refusal is one a failed proof produces.
func isAttributableAuthenticationError(svcErr *tidcommon.ServiceError) bool {
	return svcErr != nil &&
		svcErr.Type == tidcommon.ClientErrorType &&
		svcErr.Code == authnprovidercm.ErrorCodeAuthenticationFailed
}

// resolveFailedSubject turns failure evidence into the identity to count against.
func (p *defaultAuthnProvider) resolveFailedSubject(ctx context.Context,
	failure *attributedFailure) string {
	if failure.entityID != "" {
		return failure.entityID
	}
	if failure.recipient == nil {
		return ""
	}

	entityID, err := p.entityIDFromToken(ctx, failure.recipient)
	if err != nil {
		p.logger.Debug(ctx, "Not counting an authentication failure: its subject did not resolve")
		return ""
	}
	return entityID
}

// checkAuthenticationAccess submits a verified identity to governance.
func (p *defaultAuthnProvider) checkAuthenticationAccess(ctx context.Context,
	scope providers.AccessScope, result *providers.AuthnResult) *tidcommon.ServiceError {
	if p.governance == nil {
		return nil
	}
	if scope == scopeNotAnAuthenticationEvent {
		return nil
	}
	if result.EntityReference == nil || result.EntityReference.EntityID == "" {
		return nil
	}
	entityID := result.EntityReference.EntityID

	// A recovery flow's proof is admitted at the recovery plane, so a lock does not refuse the step
	// that leads to its own release.
	authorityCtx := security.WithRuntimeContext(ctx)
	var decision providers.AccessDecision
	var svcErr *tidcommon.ServiceError
	if authnprovidercm.IsRecoveryFlow(ctx) {
		decision, svcErr = p.governance.EvaluateAccess(authorityCtx, entityID, providers.AccessPlaneRecovery, scope)
	} else {
		decision, svcErr = p.governance.AdmitAuthenticationStep(authorityCtx, entityID, scope)
	}
	if svcErr != nil {
		// An unreadable authority refuses with the server error an unreadable entity gets here. The
		// service has logged the cause.
		serverErr := tidcommon.InternalServerError
		return &serverErr
	}
	if !decision.Admitted {
		return refusal(decision)
	}

	return nil
}

// refusal maps a governance decision to the authentication error contract. The reason picks the
// suspended or locked code; a refusal that names none gets the wrong-secret code. The messages are
// replaced by the manager when it normalizes the code.
func refusal(decision providers.AccessDecision) *tidcommon.ServiceError {
	switch decision.Reason {
	case providers.AccessRefusalReasonSuspended:
		return newClientError(authnprovidercm.ErrorCodeAccountSuspended,
			"Account suspended", "The account is suspended")
	case providers.AccessRefusalReasonLocked:
		return newClientError(authnprovidercm.ErrorCodeAccountLocked,
			"Account locked", "The account is locked")
	default:
		return newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
			"Authentication failed", "Invalid credentials provided")
	}
}

// passkeyEnrollmentRefusal keeps a stated suspension's code through the enrolment error mapping, or
// returns nil for any other error.
func passkeyEnrollmentRefusal(svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	if svcErr.Code != passkey.ErrorAccountSuspended.Code {
		return nil
	}
	return refusal(providers.AccessDecision{Reason: providers.AccessRefusalReasonSuspended})
}
