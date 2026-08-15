// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package introspect

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// tokenIntrospectionHandler handles OAuth 2.0 token introspection requests.
type tokenIntrospectionHandler struct {
	service TokenIntrospectionServiceInterface
	logger  *log.Logger
}

// newTokenIntrospectionHandler creates a new token introspection handler (internal use).
func newTokenIntrospectionHandler(introspectionService TokenIntrospectionServiceInterface) *tokenIntrospectionHandler {
	return &tokenIntrospectionHandler{
		service: introspectionService,
		logger:  log.GetLogger().With(log.String(log.LoggerKeyComponentName, "TokenIntrospectionHandler")),
	}
}

// HandleIntrospect handles token introspection requests
func (h *tokenIntrospectionHandler) HandleIntrospect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		sysutils.WriteJSONError(ctx, w, constants.ErrorInvalidRequest, "Failed to decode request body",
			http.StatusBadRequest, nil)
		return
	}

	// Extract request parameters
	token := r.FormValue(constants.RequestParamToken)
	if token == "" {
		sysutils.WriteJSONError(ctx, w, constants.ErrorInvalidRequest, "Token parameter is required",
			http.StatusBadRequest, nil)
		return
	}
	// token_type_hint parameter is not supported due to non persistent tokens in the server
	tokenTypeHint := r.FormValue(constants.RequestParamTokenTypeHint)

	// ClientAuthMiddleware has already authenticated the caller; the client it resolved decides which
	// tokens this request may introspect.
	clientID := ""
	if client := clientauth.GetOAuthClient(ctx); client != nil {
		clientID = client.ClientID
	}

	response, err := h.service.IntrospectToken(ctx, token, tokenTypeHint, clientID)
	if err != nil {
		h.logger.Error(ctx, "Failed to introspect token", log.Error(err))
		sysutils.WriteJSONError(ctx, w, constants.ErrorServerError,
			"An unexpected error occurred while processing the request",
			http.StatusInternalServerError, nil)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, response)
}
