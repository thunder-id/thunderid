// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"context"
	"net/http"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// revocationHandler handles OAuth 2.0 token revocation requests (RFC 7009).
type revocationHandler struct {
	service RevocationServiceInterface
	bus     eventbus.EventBusInterface
	logger  *log.Logger
}

// newRevocationHandler creates a new token revocation handler (internal use).
func newRevocationHandler(service RevocationServiceInterface, bus eventbus.EventBusInterface) *revocationHandler {
	return &revocationHandler{
		service: service,
		bus:     bus,
		logger:  log.GetLogger().With(log.String(log.LoggerKeyComponentName, "RevocationHandler")),
	}
}

// HandleRevoke handles token revocation requests. Client authentication is enforced upstream by the
// clientauth middleware (401 invalid_client on failure).
func (h *revocationHandler) HandleRevoke(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		h.publishRevoke(ctx, oauth2utils.AuditOutcome(constants.ErrorInvalidRequest))
		sysutils.WriteJSONError(ctx, w, constants.ErrorInvalidRequest, "Failed to decode request body",
			http.StatusBadRequest, nil)
		return
	}

	token := r.FormValue(constants.RequestParamToken)
	if token == "" {
		h.publishRevoke(ctx, oauth2utils.AuditOutcome(constants.ErrorInvalidRequest))
		sysutils.WriteJSONError(ctx, w, constants.ErrorInvalidRequest, "Token parameter is required",
			http.StatusBadRequest, nil)
		return
	}
	tokenTypeHint := r.FormValue(constants.RequestParamTokenTypeHint)

	clientID := ""
	if client := clientauth.GetOAuthClient(ctx); client != nil {
		clientID = client.ClientID
	}

	revokeOutcome, err := h.service.RevokeToken(ctx, token, tokenTypeHint, clientID)
	if err != nil {
		h.logger.Error(ctx, "Failed to revoke token", log.Error(err))
		h.publishRevoke(ctx, oauth2utils.AuditOutcome(constants.ErrorServerError))
		sysutils.WriteJSONError(ctx, w, constants.ErrorServerError,
			"An unexpected error occurred while processing the request",
			http.StatusInternalServerError, nil)
		return
	}

	switch revokeOutcome {
	case RevokeOutcomeNotOwned:
		// Refusing another client's token is an ownership decision, whatever the error code says.
		h.publishRevoke(ctx, audit.OutcomeDenied(constants.ErrorInvalidGrant))
		sysutils.WriteJSONError(ctx, w, constants.ErrorInvalidGrant,
			"The token was not issued to the authenticated client", http.StatusBadRequest, nil)
	default:
		// RFC 7009 §2.2: success is HTTP 200 with an empty body.
		h.publishRevoke(ctx, audit.OutcomeSucceeded)
		w.WriteHeader(http.StatusOK)
	}
}

// publishRevoke publishes the audit event of a revocation request by the authenticated client. An
// unknown or invalid token succeeds, as RFC 7009 answers it, so the event never tells whether a token
// existed, and it never carries the token.
func (h *revocationHandler) publishRevoke(ctx context.Context, outcome audit.Outcome) {
	var oauthApp *providers.OAuthClient
	if client := clientauth.GetOAuthClient(ctx); client != nil {
		oauthApp = client.OAuthApp
	}
	payload := audit.NewPayload(ctx, audit.Ref{Type: audit.TargetTypeToken}, audit.ActionRevoke, outcome).
		WithActor(oauth2utils.AuditClientActor(oauthApp))
	eventbus.TryPublish(ctx, h.bus, eventbus.NewEvent(ctx, audit.EventTokenRevoke).With(payload))
}
