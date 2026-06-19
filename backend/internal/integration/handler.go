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

package integration

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// handler handles integration catalog HTTP requests.
type handler struct {
	service ServiceInterface
}

// newHandler creates a new integration handler.
func newHandler(service ServiceInterface) *handler {
	return &handler{service: service}
}

// HandleIntegrationsRequest handles GET /integrations requests.
func (h *handler) HandleIntegrationsRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "IntegrationHandler"))

	integrations := h.service.GetIntegrations(ctx)

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, integrations)
	logger.Debug(ctx, "Integrations catalog response sent successfully")
}
