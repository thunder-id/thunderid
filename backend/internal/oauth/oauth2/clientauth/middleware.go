// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package clientauth

import (
	"context"
	"net/http"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jti"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/utils"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// ClientAuthMiddleware authenticates OAuth2 clients and attaches client info to request context. A
// client that governance holds fails authentication.
// The issuer is the authorization server's issuer identifier, accepted as the audience of a client
// assertion JWT (private_key_jwt authentication), per FAPI 2.0 Security Profile Section 5.3.2.1.
// Every refused authentication is audited on bus.
func ClientAuthMiddleware(actorProvider providers.ActorProvider,
	authnProvider providers.AuthnProviderManager,
	governance providers.IdentityGovernanceProvider,
	jwtService jwt.JWTServiceInterface,
	jtiStore jti.JTIStoreInterface,
	issuer string,
	assertionCfg engineconfig.ClientAssertionConfig,
	jwtLeeway int64,
	bus eventbus.EventBusInterface) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			// Authenticate client
			clientInfo, authErr := authenticate(ctx, r, actorProvider, authnProvider, governance, jwtService,
				jtiStore, issuer, assertionCfg, jwtLeeway)
			if authErr != nil {
				publishRefusal(ctx, bus, r, authErr)
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

// publishRefusal publishes the audit event of a refused client authentication. The actor is anonymous
// whatever token the request carries, and the target is the client the request named, unverified.
func publishRefusal(ctx context.Context, bus eventbus.EventBusInterface, r *http.Request, authErr *authError) {
	target := audit.Ref{Type: audit.TargetTypeApplication, ID: presentedClientID(r)}
	payload := audit.NewPayload(ctx, target, audit.ActionAuthenticate, oauth2utils.AuditOutcome(authErr.ErrorCode)).
		WithActor(audit.Actor{Type: audit.ActorTypeAnonymous})
	eventbus.TryPublish(ctx, bus, eventbus.NewEvent(ctx, audit.EventClientAuthenticate).With(payload))
}

// presentedClientID returns the client ID a request named, whether or not it authenticates: the
// client_id parameter, else the user name of its Basic credentials, else the subject of its client
// assertion. Never a secret or the assertion itself.
func presentedClientID(r *http.Request) string {
	if clientID := r.FormValue(constants.RequestParamClientID); clientID != "" {
		return clientID
	}
	if clientID, _, err := extractBasicAuthCredentials(r); err == nil {
		return clientID
	}
	if assertion := r.FormValue(constants.RequestParamClientAssertion); assertion != "" {
		if clientID, err := extractClientIDFromAssertion(r.Context(), assertion); err == nil {
			return clientID
		}
	}
	return ""
}
