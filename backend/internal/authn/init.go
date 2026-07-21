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

package authn

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thunder-id/thunderid/internal/authn/assert"
	"github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/github"
	"github.com/thunder-id/thunderid/internal/authn/google"
	"github.com/thunder-id/thunderid/internal/authn/magiclink"
	"github.com/thunder-id/thunderid/internal/authn/nextjssdk"
	"github.com/thunder-id/thunderid/internal/authn/oauth"
	"github.com/thunder-id/thunderid/internal/authn/oidc"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	"github.com/thunder-id/thunderid/internal/authn/reactsdk"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/template"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the authentication service and registers its routes. It also creates the
// Direct Auth Secret guard used to gate the Direct API endpoints and returns it so callers that own
// other Direct API endpoints (e.g. authzen) can reuse the same guard.
func Initialize(
	mux *http.ServeMux,
	mcpServer *mcp.Server,
	idpSvc idp.IDPServiceInterface,
	jwtSvc jwt.JWTServiceInterface,
	authnProvider providers.AuthnProviderManager,
	authAssertGen assert.AuthAssertGeneratorInterface,
	passkeySvc passkey.PasskeyServiceInterface,
	otpSvc otp.OTPAuthnServiceInterface,
	notifSenderSvc notification.NotificationSenderServiceInterface,
	templateSvc template.TemplateServiceInterface,
	magicLinkSvc magiclink.MagicLinkAuthnServiceInterface,
	oauthSvc oauth.OAuthAuthnServiceInterface,
	oidcSvc oidc.OIDCAuthnServiceInterface,
	googleSvc google.GoogleOIDCAuthnServiceInterface,
	githubSvc github.GithubOAuthAuthnServiceInterface,
	directAuthSecret string,
) (AuthenticationServiceInterface, DirectAuthGuardInterface) {
	common.RegisterAuthenticator(common.AuthenticatorMeta{
		Name:    common.AuthenticatorCredentials,
		Factors: []common.AuthenticationFactor{common.FactorKnowledge},
	})
	common.RegisterAuthenticator(common.AuthenticatorMeta{
		Name:    common.AuthenticatorOTP,
		Factors: []common.AuthenticationFactor{common.FactorPossession},
	})
	common.RegisterAuthenticator(common.AuthenticatorMeta{
		Name:    common.AuthenticatorPasskey,
		Factors: []common.AuthenticationFactor{common.FactorPossession, common.FactorInherence},
	})
	common.RegisterAuthenticator(common.AuthenticatorMeta{
		Name:          common.AuthenticatorOAuth,
		Factors:       []common.AuthenticationFactor{common.FactorKnowledge},
		AssociatedIDP: providers.IDPTypeOAuth,
	})
	common.RegisterAuthenticator(common.AuthenticatorMeta{
		Name:          common.AuthenticatorOIDC,
		Factors:       []common.AuthenticationFactor{common.FactorKnowledge},
		AssociatedIDP: providers.IDPTypeOIDC,
	})
	common.RegisterAuthenticator(common.AuthenticatorMeta{
		Name:          common.AuthenticatorGithub,
		Factors:       []common.AuthenticationFactor{common.FactorKnowledge},
		AssociatedIDP: providers.IDPTypeGitHub,
	})
	common.RegisterAuthenticator(common.AuthenticatorMeta{
		Name:          common.AuthenticatorGoogle,
		Factors:       []common.AuthenticationFactor{common.FactorKnowledge},
		AssociatedIDP: providers.IDPTypeGoogle,
	})
	common.RegisterAuthenticator(common.AuthenticatorMeta{
		Name:    common.AuthenticatorMagicLink,
		Factors: []common.AuthenticationFactor{common.FactorPossession},
	})
	common.RegisterAuthenticator(common.AuthenticatorMeta{
		Name:    common.AuthenticatorOpenID4VP,
		Factors: []common.AuthenticationFactor{common.FactorPossession, common.FactorInherence},
	})

	authnService := newAuthenticationService(
		idpSvc,
		jwtSvc,
		authAssertGen,
		authnProvider,
		otpSvc,
		notifSenderSvc,
		templateSvc,
		magicLinkSvc,
		oauthSvc,
		oidcSvc,
		googleSvc,
		githubSvc,
		passkeySvc,
	)

	directAuthGuard := newDirectAuthGuard(directAuthSecret)

	authnHandler := newAuthenticationHandler(authnService)
	registerRoutes(mux, authnHandler, directAuthGuard)

	// Register MCP tools
	if mcpServer != nil {
		reactsdk.RegisterTools(mcpServer)
		nextjssdk.RegisterTools(mcpServer)
	}

	return authnService, directAuthGuard
}

// registerRoutes registers the routes for the authentication. Direct API handlers are gated by the
// Direct Auth Secret; the CORS preflight (OPTIONS) handlers are not.
func registerRoutes(mux *http.ServeMux, authnHandler *authenticationHandler,
	guard DirectAuthGuardInterface) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	// directRoute gates a handler with the guard, applied outside CORS so a rejected request gets a
	// bare 401 with no CORS headers.
	directRoute := func(pattern string, handler http.HandlerFunc) {
		p, corsHandler := middleware.WithCORS(pattern, handler, opts)
		mux.HandleFunc(p, guard.Wrap(corsHandler))
	}

	// Credentials authentication routes
	directRoute("POST /auth/credentials/authenticate", authnHandler.HandleCredentialsAuthRequest)
	mux.HandleFunc(middleware.WithCORS("OPTIONS /auth/credentials/authenticate",
		optionsNoContentHandler, opts))

	// SMS OTP routes
	directRoute("POST /auth/otp/sms/send", authnHandler.HandleSendSMSOTPRequest)
	directRoute("POST /auth/otp/sms/verify", authnHandler.HandleVerifySMSOTPRequest)
	mux.HandleFunc(middleware.WithCORS("OPTIONS /auth/otp/sms/send",
		optionsNoContentHandler, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /auth/otp/sms/verify",
		optionsNoContentHandler, opts))

	// Google OAuth routes
	directRoute("POST /auth/oauth/google/start", authnHandler.HandleGoogleAuthStartRequest)
	directRoute("POST /auth/oauth/google/finish", authnHandler.HandleGoogleAuthFinishRequest)
	mux.HandleFunc(middleware.WithCORS("OPTIONS /auth/oauth/google/start",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /auth/oauth/google/finish",
		optionsNoContentHandler, opts))

	// GitHub OAuth routes
	directRoute("POST /auth/oauth/github/start", authnHandler.HandleGithubAuthStartRequest)
	directRoute("POST /auth/oauth/github/finish", authnHandler.HandleGithubAuthFinishRequest)
	mux.HandleFunc(middleware.WithCORS("OPTIONS /auth/oauth/github/start",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /auth/oauth/github/finish",
		optionsNoContentHandler, opts))

	// Standard OAuth routes
	directRoute("POST /auth/oauth/standard/start", authnHandler.HandleStandardOAuthStartRequest)
	directRoute("POST /auth/oauth/standard/finish", authnHandler.HandleStandardOAuthFinishRequest)
	mux.HandleFunc(middleware.WithCORS("OPTIONS /auth/oauth/standard/start",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /auth/oauth/standard/finish",
		optionsNoContentHandler, opts))

	// Passkey routes
	directRoute("POST /register/passkey/start", authnHandler.HandlePasskeyRegisterStartRequest)
	directRoute("POST /register/passkey/finish", authnHandler.HandlePasskeyRegisterFinishRequest)
	directRoute("POST /auth/passkey/start", authnHandler.HandlePasskeyStartRequest)
	directRoute("POST /auth/passkey/finish", authnHandler.HandlePasskeyFinishRequest)
	mux.HandleFunc(middleware.WithCORS("OPTIONS /register/passkey/start",
		optionsNoContentHandler, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /register/passkey/finish",
		optionsNoContentHandler, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /auth/passkey/start",
		optionsNoContentHandler, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /auth/passkey/finish",
		optionsNoContentHandler, opts))
}

// optionsNoContentHandler handles OPTIONS requests by responding with 204 No Content.
func optionsNoContentHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
