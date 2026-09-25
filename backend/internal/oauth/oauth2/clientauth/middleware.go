// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package clientauth

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jti"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// ClientAuthMiddleware authenticates OAuth2 clients and attaches client info to request context.
// The issuer is the authorization server's issuer identifier, accepted as the audience of a client
// assertion JWT (private_key_jwt authentication), per FAPI 2.0 Security Profile Section 5.3.2.1.
func ClientAuthMiddleware(actorProvider providers.ActorProvider,
	authnProvider providers.AuthnProviderManager,
	jwtService jwt.JWTServiceInterface,
	jtiStore jti.JTIStoreInterface,
	issuer string,
	assertionCfg AssertionValidationConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			// Authenticate client
			clientInfo, authErr := authenticate(ctx, r, actorProvider, authnProvider, jwtService,
				jtiStore, issuer, assertionCfg)
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
					ctx,
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
