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

package resolve

import (
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/internal/design/common"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const handlerLogger = "DesignResolveHandler"

// designResolveHandler is the handler for design resolve operations.
type designResolveHandler struct {
	resolveService DesignResolveServiceInterface
	logger         *log.Logger
}

// newDesignResolveHandler creates a new instance of designResolveHandler.
func newDesignResolveHandler(resolveService DesignResolveServiceInterface) *designResolveHandler {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLogger))
	return &designResolveHandler{
		resolveService: resolveService,
		logger:         logger,
	}
}

// HandleResolveRequest handles the resolve design configuration request.
func (rh *designResolveHandler) HandleResolveRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	resolveType := common.DesignResolveType(strings.ToUpper(r.URL.Query().Get("type")))
	id := r.URL.Query().Get("id")

	designResponse, svcErr := rh.resolveService.ResolveDesign(ctx, resolveType, id)
	if svcErr != nil {
		rh.handleError(w, svcErr)
		return
	}

	utils.WriteSuccessResponse(w, http.StatusOK, designResponse)

	rh.logger.Debug("Successfully resolved design configuration",
		log.String("type", string(resolveType)),
		log.String("id", id))
}

// handleError handles service errors and returns appropriate HTTP responses.
func (rh *designResolveHandler) handleError(w http.ResponseWriter, svcErr *serviceerror.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == serviceerror.ClientErrorType {
		switch svcErr.Code {
		case common.ErrorInvalidResolveType.Code,
			common.ErrorMissingResolveID.Code,
			common.ErrorUnsupportedResolveType.Code:
			statusCode = http.StatusBadRequest
		case common.ErrorApplicationHasNoDesign.Code,
			common.ErrorApplicationNotFound.Code:
			statusCode = http.StatusNotFound
		default:
			statusCode = http.StatusBadRequest
		}
	}

	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	}

	utils.WriteErrorResponse(w, statusCode, errResp)
}
