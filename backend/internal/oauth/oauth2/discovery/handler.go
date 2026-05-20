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

package discovery

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// DiscoveryHandlerInterface defines the interface for discovery handlers
type discoveryHandlerInterface interface {
	HandleOAuth2AuthorizationServerMetadata(w http.ResponseWriter, r *http.Request)
	HandleOIDCDiscovery(w http.ResponseWriter, r *http.Request)
}

// discoveryHandler implements DiscoveryHandlerInterface
type discoveryHandler struct {
	discoveryService DiscoveryServiceInterface
}

// NewDiscoveryHandler creates a new discovery handler
func newDiscoveryHandler(discoveryService DiscoveryServiceInterface) discoveryHandlerInterface {
	return &discoveryHandler{
		discoveryService: discoveryService,
	}
}

// HandleOAuth2AuthorizationServerMetadata handles OAuth 2.0 Authorization Server Metadata requests
func (dh *discoveryHandler) HandleOAuth2AuthorizationServerMetadata(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DiscoveryHandler"))

	metadata := dh.discoveryService.GetOAuth2AuthorizationServerMetadata(ctx)

	sysutils.WriteSuccessResponse(w, http.StatusOK, metadata)
	logger.Debug("OAuth 2.0 Authorization Server Metadata response sent successfully")
}

// HandleOIDCDiscovery handles OpenID Connect Discovery requests
func (dh *discoveryHandler) HandleOIDCDiscovery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DiscoveryHandler"))

	metadata, err := dh.discoveryService.GetOIDCMetadata(ctx)
	if err != nil {
		sysutils.WriteErrorResponse(w, http.StatusInternalServerError, apierror.ErrorResponse{})
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, metadata)
	logger.Debug("OIDC discovery response sent successfully")
}
