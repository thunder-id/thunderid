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

package ciba

import (
	"context"
	"net/http"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	sysconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// CIBAHandlerInterface defines the interface for handling CIBA backchannel authentication requests.
type CIBAHandlerInterface interface {
	HandleBackchannelAuthRequest(w http.ResponseWriter, r *http.Request)
}

// cibaHandler implements the CIBAHandlerInterface.
type cibaHandler struct {
	cibaService CIBAServiceInterface
	logger      *log.Logger
}

// newCIBAHandler creates a new instance of cibaHandler.
func newCIBAHandler(cibaService CIBAServiceInterface) CIBAHandlerInterface {
	return &cibaHandler{
		cibaService: cibaService,
		logger:      log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CIBAHandler")),
	}
}

// HandleBackchannelAuthRequest handles a POST /oauth2/bc-authorize request.
func (h *cibaHandler) HandleBackchannelAuthRequest(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		utils.WriteJSONError(r.Context(), w, oauth2const.ErrorInvalidRequest, "Failed to parse request body",
			http.StatusBadRequest, nil)
		return
	}

	// Get authenticated client from context (set by ClientAuthMiddleware).
	clientInfo := clientauth.GetOAuthClient(r.Context())
	if clientInfo == nil {
		h.logger.Error(r.Context(),
			"OAuth client not found in context - ClientAuthMiddleware must be applied")
		utils.WriteJSONError(r.Context(), w, oauth2const.ErrorServerError, "Something went wrong",
			http.StatusInternalServerError, nil)
		return
	}

	loginHint := r.FormValue(oauth2const.RequestParamLoginHint)
	idTokenHint := r.FormValue(oauth2const.RequestParamIDTokenHint)
	loginHintToken := r.FormValue(oauth2const.RequestParamLoginHintToken)

	// Enforce exactly-one hint rule (CIBA Core 1.0 §7.1).
	// The spec states the client MUST include exactly one of login_hint, id_token_hint,
	// or login_hint_token. Zero or multiple hints are both invalid_request.
	hintsProvided := countNonEmpty(loginHint, idTokenHint, loginHintToken)
	if hintsProvided == 0 {
		utils.WriteJSONError(r.Context(), w, oauth2const.ErrorInvalidRequest,
			"One of login_hint, id_token_hint, or login_hint_token is required",
			http.StatusBadRequest, nil)
		return
	}
	if hintsProvided > 1 {
		utils.WriteJSONError(r.Context(), w, oauth2const.ErrorInvalidRequest,
			"Only one of login_hint, id_token_hint, or login_hint_token may be provided",
			http.StatusBadRequest, nil)
		return
	}

	// login_hint_token is valid per the CIBA spec but not yet implemented by this OP.
	// TODO: implement login_hint_token — resolve user from a signed hint JWT
	if loginHintToken != "" {
		utils.WriteJSONError(r.Context(), w, oauth2const.ErrorInvalidRequest,
			"login_hint_token is not supported, use login_hint",
			http.StatusBadRequest, nil)
		return
	}

	request := &BackchannelAuthRequest{
		LoginHint:       loginHint,
		IDTokenHint:     idTokenHint,
		Scope:           r.FormValue(oauth2const.RequestParamScope),
		Resources:       r.Form[oauth2const.RequestParamResource],
		BindingMessage:  r.FormValue(oauth2const.RequestParamBindingMessage),
		RequestedExpiry: r.FormValue(oauth2const.RequestParamRequestedExpiry),
		ACRValues:       r.FormValue(oauth2const.RequestParamAcrValues),
		Headers:         utils.SanitizeRawMultiValueStringMap(r.Header),
		QueryParams:     utils.SanitizeRawMultiValueStringMap(r.URL.Query()),
	}

	response, cibaErr := h.cibaService.InitiateBackchannelAuth(r.Context(), request, clientInfo.OAuthApp)
	if cibaErr != nil {
		writeCIBAError(r.Context(), w, cibaErr)
		return
	}

	w.Header().Set(sysconst.CacheControlHeaderName, sysconst.CacheControlNoStore)
	w.Header().Set(sysconst.PragmaHeaderName, sysconst.PragmaNoCache)
	utils.WriteSuccessResponse(r.Context(), w, http.StatusOK, response)
}

// writeCIBAError maps a CIBAError to the appropriate HTTP status code and writes the JSON response.
func writeCIBAError(ctx context.Context, w http.ResponseWriter, cibaErr *CIBAError) {
	statusCode := http.StatusBadRequest
	if cibaErr.Code == oauth2const.ErrorServerError {
		statusCode = http.StatusInternalServerError
	}
	utils.WriteJSONError(ctx, w, cibaErr.Code, cibaErr.Message, statusCode, nil)
}

// countNonEmpty returns the number of non-empty strings in the provided values.
func countNonEmpty(values ...string) int {
	count := 0
	for _, v := range values {
		if v != "" {
			count++
		}
	}
	return count
}
