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

package jwks

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// jwksHandler handles requests for the JSON Web Key Set (JWKS).
type jwksHandler struct {
	jwksService JWKSServiceInterface
}

// newJWKSHandler creates a new instance of jwksHandler.
func newJWKSHandler(jwksService JWKSServiceInterface) *jwksHandler {
	return &jwksHandler{
		jwksService: jwksService,
	}
}

// HandleJWKSRequest handles the HTTP request to retrieve the JSON Web Key Set (JWKS).
func (h *jwksHandler) HandleJWKSRequest(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "JWKSHandler"))

	jwksResponse, svcErr := h.jwksService.GetJWKS()
	if svcErr != nil {
		h.logAndWriteError(w, logger, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, jwksResponse)
	logger.Debug("JWKS response successfully sent")
}

// logAndWriteError logs server errors and writes an appropriate error response to the HTTP response writer.
func (h *jwksHandler) logAndWriteError(w http.ResponseWriter, logger *log.Logger,
	svcErr *serviceerror.ServiceError) {
	statusCode := http.StatusBadRequest
	if svcErr.Type == serviceerror.ServerErrorType {
		statusCode = http.StatusInternalServerError
		logger.Error("Failed to retrieve JWKS", log.String("error_code", svcErr.Code))
	}

	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	}

	sysutils.WriteErrorResponse(w, statusCode, errResp)
}
