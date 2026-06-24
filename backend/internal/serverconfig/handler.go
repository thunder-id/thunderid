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
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const handlerLoggerComponentName = "ServerConfigHandler"

// serverConfigHandler is the handler for server config operations.
type serverConfigHandler struct {
	serverConfigService ServerConfigService
}

// newServerConfigHandler creates a new instance of serverConfigHandler.
func newServerConfigHandler(serverConfigService ServerConfigService) *serverConfigHandler {
	return &serverConfigHandler{
		serverConfigService: serverConfigService,
	}
}

// HandleGetServerConfig handles GET /server-config
func (h *serverConfigHandler) HandleGetServerConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	configs, svcErr := h.serverConfigService.ListConfigs(ctx)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, toServerConfigResponse(configs))
	logger.Debug(ctx, "Successfully retrieved server config")
}

// HandleGetServerConfigByName handles GET /server-config/{name}
func (h *serverConfigHandler) HandleGetServerConfigByName(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	name := ConfigName(r.PathValue("name"))
	value, svcErr := h.serverConfigService.GetConfig(ctx, name)
	if svcErr != nil {
		if svcErr.Code != ErrorConfigNotFound.Code {
			handleError(ctx, w, svcErr)
			return
		}
		value = emptyConfigValue(name)
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, map[ConfigName]json.RawMessage{name: value})
	logger.Debug(ctx, "Successfully retrieved server config section")
}

// HandleUpdateServerConfig handles PUT /server-config
func (h *serverConfigHandler) HandleUpdateServerConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	req, err := sysutils.DecodeJSONBody[ServerConfigRequest](r)
	if err != nil {
		handleError(ctx, w, &ErrorInvalidRequestFormat)
		return
	}

	configs := make(map[ConfigName]json.RawMessage)
	if req.CORS != nil {
		configs[ConfigNameCORS] = req.CORS
	}

	if svcErr := h.serverConfigService.SetConfigs(ctx, configs); svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	updated, svcErr := h.serverConfigService.ListConfigs(ctx)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, toServerConfigResponse(updated))
	logger.Debug(ctx, "Successfully updated server config")
}

// toServerConfigResponse assembles the typed response from the generic config map, defaulting an
// unset section to its empty value so the response shape is stable.
func toServerConfigResponse(configs map[ConfigName]json.RawMessage) ServerConfigResponse {
	cors := configs[ConfigNameCORS]
	if cors == nil {
		cors = emptyConfigValue(ConfigNameCORS)
	}
	return ServerConfigResponse{
		CORS: cors,
	}
}

// emptyConfigValue returns the empty representation of a config section that has not been set.
func emptyConfigValue(name ConfigName) json.RawMessage {
	switch name {
	case ConfigNameCORS:
		return json.RawMessage("[]")
	default:
		return json.RawMessage("null")
	}
}

// handleError maps a service error to an HTTP error response.
func handleError(ctx context.Context, w http.ResponseWriter, svcErr *serviceerror.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == serviceerror.ClientErrorType {
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
