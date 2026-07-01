/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package revocation

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// revocationHandler handles OAuth 2.0 token revocation requests (RFC 7009).
type revocationHandler struct {
	service RevocationServiceInterface
	logger  *log.Logger
}

// newRevocationHandler creates a new token revocation handler (internal use).
func newRevocationHandler(service RevocationServiceInterface) *revocationHandler {
	return &revocationHandler{
		service: service,
		logger:  log.GetLogger().With(log.String(log.LoggerKeyComponentName, "RevocationHandler")),
	}
}

// HandleRevoke handles token revocation requests. Client authentication is enforced upstream by the
// clientauth middleware (401 invalid_client on failure).
func (h *revocationHandler) HandleRevoke(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		sysutils.WriteJSONError(ctx, w, constants.ErrorInvalidRequest, "Failed to decode request body",
			http.StatusBadRequest, nil)
		return
	}

	token := r.FormValue(constants.RequestParamToken)
	if token == "" {
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
		sysutils.WriteJSONError(ctx, w, constants.ErrorServerError,
			"An unexpected error occurred while processing the request",
			http.StatusInternalServerError, nil)
		return
	}

	switch revokeOutcome {
	case RevokeOutcomeNotOwned:
		sysutils.WriteJSONError(ctx, w, constants.ErrorInvalidGrant,
			"The token was not issued to the authenticated client", http.StatusBadRequest, nil)
	default:
		// RFC 7009 §2.2: success is HTTP 200 with an empty body.
		w.WriteHeader(http.StatusOK)
	}
}
