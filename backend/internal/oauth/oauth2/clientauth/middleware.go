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

package clientauth

import (
	"net/http"

	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// ClientAuthMiddleware authenticates OAuth2 clients and attaches client info to request context.
// The endpointURL is the full URL of the endpoint being protected, used as the expected audience
// when validating client assertion JWTs (private_key_jwt authentication).
func ClientAuthMiddleware(inboundClient inboundclient.InboundClientServiceInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
	jwtService jwt.JWTServiceInterface,
	endpointURL string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			// Authenticate client
			clientInfo, authErr := authenticate(ctx, r, inboundClient, authnProvider, jwtService, endpointURL)
			if authErr != nil {
				// If the client attempted to authenticate via the Authorization
				// header, include WWW-Authenticate in 401 responses.
				var respHeaders []map[string]string
				if authErr.StatusCode == http.StatusUnauthorized &&
					r.Header.Get(serverconst.AuthorizationHeaderName) != "" {
					respHeaders = []map[string]string{
						{serverconst.WWWAuthenticateHeaderName: "Basic"},
					}
				}
				// Write error response
				utils.WriteJSONError(
					w,
					authErr.ErrorCode,
					authErr.ErrorDescription,
					authErr.StatusCode,
					respHeaders,
				)
				return
			}

			// Attach client info to context
			ctx = withOAuthClient(ctx, clientInfo)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}
