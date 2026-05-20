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

package par

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// parHandlerInterface defines the interface for handling PAR requests.
type parHandlerInterface interface {
	HandlePARRequest(w http.ResponseWriter, r *http.Request)
}

// parHandler implements parHandlerInterface.
type parHandler struct {
	parService PARServiceInterface
	logger     *log.Logger
}

// newPARHandler creates a new PAR handler instance.
func newPARHandler(parService PARServiceInterface) parHandlerInterface {
	return &parHandler{
		parService: parService,
		logger:     log.GetLogger().With(log.String(log.LoggerKeyComponentName, "PARHandler")),
	}
}

// HandlePARRequest handles the POST /oauth2/par request.
func (h *parHandler) HandlePARRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Client authentication is handled by the ClientAuthMiddleware.
	clientInfo := clientauth.GetOAuthClient(ctx)
	if clientInfo == nil {
		h.logger.Error("OAuth client not found in context - ClientAuthMiddleware must be applied")
		utils.WriteJSONError(w, oauth2const.ErrorServerError,
			"Something went wrong", http.StatusInternalServerError, nil)
		return
	}

	// Parse form-encoded body.
	if err := r.ParseForm(); err != nil {
		utils.WriteJSONError(w, oauth2const.ErrorInvalidRequest, "Failed to parse request body",
			http.StatusBadRequest, nil)
		return
	}

	params := make(map[string]string)
	for key, values := range r.PostForm {
		if len(values) > 0 {
			params[key] = values[0]
		}
	}
	resources := r.PostForm[oauth2const.RequestParamResource]

	resp, errCode, errDesc := h.parService.HandlePushedAuthorizationRequest(ctx, params, resources, clientInfo.OAuthApp)
	if errCode != "" {
		statusCode := http.StatusBadRequest
		if errCode == oauth2const.ErrorServerError {
			h.logger.Error("Internal server error processing pushed authorization request",
				log.MaskedString("clientID", clientInfo.ClientID),
				log.String("errorCode", errCode),
				log.String("errorDescription", errDesc),
			)
			statusCode = http.StatusInternalServerError
		}
		utils.WriteJSONError(w, errCode, errDesc, statusCode, nil)
		return
	}

	utils.WriteSuccessResponse(w, http.StatusCreated, resp)
}
