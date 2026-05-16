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
	"strings"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const handlerLoggerComponentName = "UserInfoHandler"

// userInfoHandler handles OIDC UserInfo requests.
type userInfoHandler struct {
	service          userInfoServiceInterface
	userInfoEndpoint string
	dpopAllowedAlgs  []string
	logger           *log.Logger
}

// newUserInfoHandler creates a new userInfo handler.
func newUserInfoHandler(
	userInfoService userInfoServiceInterface,
	userInfoEndpoint string,
	dpopAllowedAlgs []string,
) *userInfoHandler {
	return &userInfoHandler{
		service:          userInfoService,
		userInfoEndpoint: userInfoEndpoint,
		dpopAllowedAlgs:  dpopAllowedAlgs,
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName)),
	}
}

// HandleUserInfo handles UserInfo requests.
func (h *userInfoHandler) HandleUserInfo(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get(serverconst.AuthorizationHeaderName)

	if dpop.IsDPoPAuth(authHeader) {
		h.handleDPoPRequest(w, r, authHeader)
		return
	}

	h.handleBearerRequest(w, r, authHeader)
}

// handleBearerRequest serves the request under the Bearer scheme. A DPoP-bound token
// presented here is rejected as a downgrade with WWW-Authenticate: DPoP.
func (h *userInfoHandler) handleBearerRequest(
	w http.ResponseWriter, r *http.Request, authHeader string,
) {
	accessToken, err := utils.ExtractBearerToken(authHeader)
	if err != nil {
		if authHeader == "" || !utils.IsBearerAuth(authHeader) {
			w.Header().Set(serverconst.WWWAuthenticateHeaderName, serverconst.TokenTypeBearer)
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			h.writeBearerError(w, constants.ErrorInvalidRequest,
				"Invalid or malformed Bearer token", http.StatusBadRequest)
		}
		return
	}

	result, svcErr := h.service.GetUserInfo(r.Context(), accessToken)
	if svcErr != nil {
		h.writeServiceErrorResponse(w, svcErr, svcErr == &errorBearerDowngrade)
		return
	}

	h.writeUserInfoResponse(w, result)
}

// handleDPoPRequest serves the request under the DPoP scheme.
func (h *userInfoHandler) handleDPoPRequest(
	w http.ResponseWriter, r *http.Request, authHeader string,
) {
	accessToken, err := dpop.ExtractDPoPToken(authHeader)
	if err != nil {
		h.writeDPoPError(w, "invalid_token", "Invalid or malformed DPoP token", http.StatusUnauthorized)
		return
	}

	dpopHeaders := r.Header.Values(constants.HeaderDPoP)
	if len(dpopHeaders) != 1 {
		h.writeDPoPError(w, "invalid_token",
			"Exactly one DPoP header is required", http.StatusUnauthorized)
		return
	}

	result, svcErr := h.service.GetUserInfoForDPoP(
		r.Context(), accessToken, dpopHeaders[0], r.Method, h.userInfoEndpoint)
	if svcErr != nil {
		h.writeServiceErrorResponse(w, svcErr, true)
		return
	}

	h.writeUserInfoResponse(w, result)
}

func (h *userInfoHandler) writeUserInfoResponse(w http.ResponseWriter, result *UserInfoResponse) {
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

// writeServiceErrorResponse writes a service error response. The dpop flag selects
// between WWW-Authenticate: Bearer and WWW-Authenticate: DPoP.
func (h *userInfoHandler) writeServiceErrorResponse(
	w http.ResponseWriter, svcErr *serviceerror.ServiceError, dpop bool,
) {
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
		h.logger.Error("Internal server error processing userinfo request",
			log.String("errorCode", svcErr.Code),
			log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
		utils.WriteJSONError(w, constants.ErrorServerError,
			serviceerror.InternalServerError.Error.DefaultValue, statusCode, nil)
		return
	}

	if dpop {
		h.writeDPoPError(w, svcErr.Code, svcErr.ErrorDescription.DefaultValue, statusCode)
	} else {
		h.writeBearerError(w, svcErr.Code, svcErr.ErrorDescription.DefaultValue, statusCode)
	}
}

// writeBearerError writes a JSON error response with a WWW-Authenticate: Bearer header.
func (h *userInfoHandler) writeBearerError(
	w http.ResponseWriter, errorCode, errorDescription string, statusCode int,
) {
	wwwAuth := fmt.Sprintf("Bearer error=%q, error_description=%q", errorCode, errorDescription)
	utils.WriteJSONError(w, errorCode, errorDescription, statusCode,
		[]map[string]string{{serverconst.WWWAuthenticateHeaderName: wwwAuth}})
}

// writeDPoPError writes a JSON error response with a WWW-Authenticate: DPoP header
// advertising the supported DPoP signing algorithms.
func (h *userInfoHandler) writeDPoPError(
	w http.ResponseWriter, errorCode, errorDescription string, statusCode int,
) {
	wwwAuth := fmt.Sprintf("DPoP algs=%q, error=%q, error_description=%q",
		strings.Join(h.dpopAllowedAlgs, " "), errorCode, errorDescription)
	utils.WriteJSONError(w, errorCode, errorDescription, statusCode,
		[]map[string]string{{serverconst.WWWAuthenticateHeaderName: wwwAuth}})
}
