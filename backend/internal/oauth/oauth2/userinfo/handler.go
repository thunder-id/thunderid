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

package userinfo

import (
	"fmt"
	"net/http"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const handlerLoggerComponentName = "UserInfoHandler"

// userInfoHandler handles OIDC UserInfo requests.
type userInfoHandler struct {
	service userInfoServiceInterface
	logger  *log.Logger
}

// newUserInfoHandler creates a new userInfo handler.
func newUserInfoHandler(userInfoService userInfoServiceInterface) *userInfoHandler {
	return &userInfoHandler{
		service: userInfoService,
		logger:  log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName)),
	}
}

// HandleUserInfo handles UserInfo requests.
func (h *userInfoHandler) HandleUserInfo(w http.ResponseWriter, r *http.Request) {
	// Extract access token from Authorization header
	authHeader := r.Header.Get(serverconst.AuthorizationHeaderName)
	accessToken, err := utils.ExtractBearerToken(authHeader)
	if err != nil {
		if authHeader == "" || !utils.IsBearerAuth(authHeader) {
			w.Header().Set(serverconst.WWWAuthenticateHeaderName, serverconst.TokenTypeBearer)
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			writeBearerError(w, constants.ErrorInvalidRequest,
				"Invalid or malformed Bearer token", http.StatusBadRequest)
		}
		return
	}

	result, svcErr := h.service.GetUserInfo(r.Context(), accessToken)
	if svcErr != nil {
		h.writeServiceErrorResponse(w, svcErr)
		return
	}

	w.Header().Set(serverconst.CacheControlHeaderName, serverconst.CacheControlNoStore)
	w.Header().Set(serverconst.PragmaHeaderName, serverconst.PragmaNoCache)

	switch result.Type {
	case inboundmodel.UserInfoResponseTypeJWS:
		w.Header().Set(serverconst.ContentTypeHeaderName, serverconst.ContentTypeJWT)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(result.JWTBody))
	case inboundmodel.UserInfoResponseTypeJWE, inboundmodel.UserInfoResponseTypeNESTEDJWT:
		w.Header().Set(serverconst.ContentTypeHeaderName, serverconst.ContentTypeJWT)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(result.JWTBody))
	default:
		utils.WriteSuccessResponse(w, http.StatusOK, result.JSONBody)
	}

	h.logger.Debug("UserInfo response sent successfully")
}

// writeServiceErrorResponse writes a service error response.
func (h *userInfoHandler) writeServiceErrorResponse(w http.ResponseWriter, svcErr *serviceerror.ServiceError) {
	var statusCode int

	switch svcErr.Type {
	case serviceerror.ClientErrorType:
		if svcErr.Code == errorInsufficientScope.Code {
			statusCode = http.StatusForbidden
		} else {
			statusCode = http.StatusUnauthorized
		}
	case serviceerror.ServerErrorType:
		statusCode = http.StatusInternalServerError
	default:
		statusCode = http.StatusUnauthorized
	}

	if statusCode == http.StatusInternalServerError {
		utils.WriteJSONError(w, constants.ErrorServerError,
			serviceerror.InternalServerError.Error.DefaultValue, statusCode, nil)
	} else {
		writeBearerError(w, svcErr.Code, svcErr.ErrorDescription.DefaultValue, statusCode)
	}
}

// writeBearerError writes a JSON error response with a WWW-Authenticate: Bearer header.
func writeBearerError(w http.ResponseWriter, errorCode, errorDescription string, statusCode int) {
	wwwAuth := fmt.Sprintf("Bearer error=%q, error_description=%q", errorCode, errorDescription)
	utils.WriteJSONError(w, errorCode, errorDescription, statusCode,
		[]map[string]string{{serverconst.WWWAuthenticateHeaderName: wwwAuth}})
}
