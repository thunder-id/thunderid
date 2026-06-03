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
	HandleBackchannelAuthCallback(w http.ResponseWriter, r *http.Request)
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
		utils.WriteJSONError(w, oauth2const.ErrorInvalidRequest, "Failed to parse request body",
			http.StatusBadRequest, nil)
		return
	}

	// Get authenticated client from context (set by ClientAuthMiddleware).
	clientInfo := clientauth.GetOAuthClient(r.Context())
	if clientInfo == nil {
		h.logger.Error("OAuth client not found in context - ClientAuthMiddleware must be applied")
		utils.WriteJSONError(w, oauth2const.ErrorServerError, "Something went wrong",
			http.StatusInternalServerError, nil)
		return
	}

	request := &BackchannelAuthRequest{
		LoginHint:       r.FormValue(oauth2const.RequestParamLoginHint),
		Scope:           r.FormValue(oauth2const.RequestParamScope),
		BindingMessage:  r.FormValue(oauth2const.RequestParamBindingMessage),
		RequestedExpiry: r.FormValue(oauth2const.RequestParamRequestedExpiry),
	}

	response, cibaErr := h.cibaService.InitiateBackchannelAuth(r.Context(), request, clientInfo.OAuthApp)
	if cibaErr != nil {
		writeCIBAError(w, cibaErr)
		return
	}

	w.Header().Set(sysconst.CacheControlHeaderName, sysconst.CacheControlNoStore)
	w.Header().Set(sysconst.PragmaHeaderName, sysconst.PragmaNoCache)
	utils.WriteSuccessResponse(w, http.StatusOK, response)
}

// HandleBackchannelAuthCallback handles a POST /oauth2/bc-authorize/callback request.
func (h *cibaHandler) HandleBackchannelAuthCallback(w http.ResponseWriter, r *http.Request) {
	callbackReq, err := utils.DecodeJSONBody[CallbackRequest](r)
	if err != nil {
		utils.WriteJSONError(w, oauth2const.ErrorInvalidRequest, "Invalid request body",
			http.StatusBadRequest, nil)
		return
	}

	cibaErr := h.cibaService.HandleCallback(r.Context(), callbackReq.AuthReqID, callbackReq.Assertion)
	if cibaErr != nil {
		writeCIBAError(w, cibaErr)
		return
	}

	utils.WriteSuccessResponse(w, http.StatusOK, map[string]string{"status": "OK"})
}

// writeCIBAError maps a cibaError to the appropriate HTTP status code and writes the JSON response.
func writeCIBAError(w http.ResponseWriter, cibaErr *cibaError) {
	statusCode := http.StatusBadRequest
	if cibaErr.Code == oauth2const.ErrorServerError {
		statusCode = http.StatusInternalServerError
	}
	utils.WriteJSONError(w, cibaErr.Code, cibaErr.Message, statusCode, nil)
}
