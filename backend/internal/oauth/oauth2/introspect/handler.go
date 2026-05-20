/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package introspect

import (
	"net/http"

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
		sysutils.WriteJSONError(w, constants.ErrorInvalidRequest, "Failed to decode request body",
			http.StatusBadRequest, nil)
		return
	}

	// Extract request parameters
	token := r.FormValue(constants.RequestParamToken)
	if token == "" {
		sysutils.WriteJSONError(w, constants.ErrorInvalidRequest, "Token parameter is required",
			http.StatusBadRequest, nil)
		return
	}
	// token_type_hint parameter is not supported due to non persistent tokens in the server
	tokenTypeHint := r.FormValue(constants.RequestParamTokenTypeHint)

	response, err := h.service.IntrospectToken(ctx, token, tokenTypeHint)
	if err != nil {
		h.logger.Error("Failed to introspect token", log.Error(err))
		sysutils.WriteJSONError(w, constants.ErrorServerError,
			"An unexpected error occurred while processing the request",
			http.StatusInternalServerError, nil)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, response)
}
