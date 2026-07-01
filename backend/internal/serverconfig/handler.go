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

package serverconfig

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// maxServerConfigBodyBytes caps the PUT request body to bound memory use; config sections are small.
const maxServerConfigBodyBytes = 1 << 20 // 1 MiB

// serverConfigHandler is the handler for server config operations.
type serverConfigHandler struct {
	serverConfigService ServerConfigService
}

// newServerConfigHandler creates a new instance of serverConfigHandler.
func newServerConfigHandler(serverConfigService ServerConfigService) *serverConfigHandler {
	return &serverConfigHandler{serverConfigService: serverConfigService}
}

// HandleListServerConfigs handles GET /server-config, returning the supported section names.
func (h *serverConfigHandler) HandleListServerConfigs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	names, svcErr := h.serverConfigService.ListConfigNames(ctx)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, names)
}

// HandleGetServerConfig handles GET /server-config/{name}, returning the section's three layers.
func (h *serverConfigHandler) HandleGetServerConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	layers, svcErr := h.serverConfigService.GetConfig(ctx, ConfigName(r.PathValue("name")))
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, layers)
}

// HandleUpdateServerConfig handles PUT /server-config/{name}, replacing the writable layer and
// returning the recomputed layers.
func (h *serverConfigHandler) HandleUpdateServerConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := ConfigName(r.PathValue("name"))

	r.Body = http.MaxBytesReader(w, r.Body, maxServerConfigBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil || !json.Valid(body) {
		handleError(ctx, w, &ErrorInvalidRequestFormat)
		return
	}

	if svcErr := h.serverConfigService.SetConfig(ctx, name, json.RawMessage(body)); svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	layers, svcErr := h.serverConfigService.GetConfig(ctx, name)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, layers)
}

// handleError maps a service error to an HTTP error response.
func handleError(ctx context.Context, w http.ResponseWriter, svcErr *common.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == common.ClientErrorType {
		statusCode = http.StatusBadRequest
		if svcErr.Code == ErrorConfigNotFound.Code {
			statusCode = http.StatusNotFound
		}
	}

	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	}

	sysutils.WriteErrorResponse(ctx, w, statusCode, errResp)
}
