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

package token

import (
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	sysconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// TokenHandlerInterface defines the interface for handling OAuth 2.0 token requests.
type TokenHandlerInterface interface {
	HandleTokenRequest(w http.ResponseWriter, r *http.Request)
}

// tokenHandler implements the TokenHandlerInterface.
type tokenHandler struct {
	tokenService     TokenServiceInterface
	observabilitySvc observability.ObservabilityServiceInterface
}

// newTokenHandler creates a new instance of tokenHandler.
func newTokenHandler(
	tokenService TokenServiceInterface,
	observabilitySvc observability.ObservabilityServiceInterface,
) TokenHandlerInterface {
	return &tokenHandler{
		tokenService:     tokenService,
		observabilitySvc: observabilitySvc,
	}
}

// HandleTokenRequest handles the token request for OAuth 2.0.
// It parses the HTTP request, delegates processing to the token service, and writes the HTTP response.
func (th *tokenHandler) HandleTokenRequest(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "TokenHandler"))

	startTime := time.Now().UnixMilli()

	// Parse the form data from the request body.
	if err := r.ParseForm(); err != nil {
		publishTokenIssuanceFailedEvent(th.observabilitySvc, r.Context(), "", "", "",
			http.StatusBadRequest, err.Error(), startTime)
		utils.WriteJSONError(w, constants.ErrorInvalidRequest,
			"Failed to parse request body", http.StatusBadRequest, nil)
		return
	}

	// Get authenticated client from context (set by ClientAuthMiddleware).
	clientInfo := clientauth.GetOAuthClient(r.Context())
	if clientInfo == nil {
		logger.Error("OAuth client not found in context - ClientAuthMiddleware must be applied")
		utils.WriteJSONError(w, constants.ErrorServerError,
			"Something went wrong", http.StatusInternalServerError, nil)
		return
	}

	// Build the token request domain model from the HTTP form values.
	tokenRequest := &model.TokenRequest{
		GrantType:          r.FormValue(constants.RequestParamGrantType),
		ClientID:           clientInfo.ClientID,
		ClientSecret:       clientInfo.ClientSecret,
		Scope:              r.FormValue("scope"),
		Username:           r.FormValue("username"),
		Password:           r.FormValue("password"),
		RefreshToken:       r.FormValue("refresh_token"),
		CodeVerifier:       r.FormValue("code_verifier"),
		Code:               r.FormValue("code"),
		RedirectURI:        r.FormValue("redirect_uri"),
		Resources:          r.Form[constants.RequestParamResource],
		SubjectToken:       r.FormValue(constants.RequestParamSubjectToken),
		SubjectTokenType:   r.FormValue(constants.RequestParamSubjectTokenType),
		ActorToken:         r.FormValue(constants.RequestParamActorToken),
		ActorTokenType:     r.FormValue(constants.RequestParamActorTokenType),
		RequestedTokenType: r.FormValue(constants.RequestParamRequestedTokenType),
		Audiences:          r.Form[constants.RequestParamAudience],
	}

	// Delegate all business logic to the token service.
	tokenResponse, tokenError := th.tokenService.ProcessTokenRequest(r.Context(), tokenRequest, clientInfo.OAuthApp)
	if tokenError != nil {
		if tokenError.Error != "" {
			var statusCode int
			switch tokenError.Error {
			case constants.ErrorServerError:
				statusCode = http.StatusInternalServerError
			default:
				statusCode = http.StatusBadRequest
			}
			utils.WriteJSONError(w, tokenError.Error, tokenError.ErrorDescription, statusCode, nil)
		} else {
			utils.WriteJSONError(w, constants.ErrorServerError, "Something went wrong",
				http.StatusInternalServerError, nil)
		}
		return
	}

	logger.Debug("Token response sending", log.String("client_id", clientInfo.ClientID))

	// Must include the following headers when sensitive data is returned.
	w.Header().Set(sysconst.CacheControlHeaderName, sysconst.CacheControlNoStore)
	w.Header().Set(sysconst.PragmaHeaderName, sysconst.PragmaNoCache)

	utils.WriteSuccessResponse(w, http.StatusOK, tokenResponse)
}
