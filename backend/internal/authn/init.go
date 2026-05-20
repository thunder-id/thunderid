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
	"github.com/thunder-id/thunderid/internal/authn/oauth"
	"github.com/thunder-id/thunderid/internal/authn/oidc"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	"github.com/thunder-id/thunderid/internal/authn/reactsdk"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize initializes the authentication service and registers its routes.
func Initialize(
	mux *http.ServeMux,
	mcpServer *mcp.Server,
	idpSvc idp.IDPServiceInterface,
	jwtSvc jwt.JWTServiceInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
	authAssertGen assert.AuthAssertGeneratorInterface,
	passkeySvc passkey.PasskeyServiceInterface,
	otpSvc otp.OTPAuthnServiceInterface,
	magicLinkSvc magiclink.MagicLinkAuthnServiceInterface,
	oauthSvc oauth.OAuthAuthnServiceInterface,
	oidcSvc oidc.OIDCAuthnServiceInterface,
	googleSvc google.GoogleOIDCAuthnServiceInterface,
	githubSvc github.GithubOAuthAuthnServiceInterface,
) AuthenticationServiceInterface {
	common.RegisterAuthenticator(common.AuthenticatorMeta{
		Name:    common.AuthenticatorCredentials,
		Factors: []common.AuthenticationFactor{common.FactorKnowledge},
	})
	common.RegisterAuthenticator(common.AuthenticatorMeta{
		Name:    common.AuthenticatorSMSOTP,
		Factors: []common.AuthenticationFactor{common.FactorPossession},
	})
	common.RegisterAuthenticator(common.AuthenticatorMeta{
		Name:    common.AuthenticatorPasskey,
		Factors: []common.AuthenticationFactor{common.FactorPossession, common.FactorInherence},
	})
	common.RegisterAuthenticator(common.AuthenticatorMeta{
		Name:          common.AuthenticatorOAuth,
		Factors:       []common.AuthenticationFactor{common.FactorKnowledge},
		AssociatedIDP: idp.IDPTypeOAuth,
	})
	common.RegisterAuthenticator(common.AuthenticatorMeta{
		Name:          common.AuthenticatorOIDC,
		Factors:       []common.AuthenticationFactor{common.FactorKnowledge},
		AssociatedIDP: idp.IDPTypeOIDC,
	})
	common.RegisterAuthenticator(common.AuthenticatorMeta{
		Name:          common.AuthenticatorGithub,
		Factors:       []common.AuthenticationFactor{common.FactorKnowledge},
		AssociatedIDP: idp.IDPTypeGitHub,
	})
	common.RegisterAuthenticator(common.AuthenticatorMeta{
		Name:          common.AuthenticatorGoogle,
		Factors:       []common.AuthenticationFactor{common.FactorKnowledge},
		AssociatedIDP: idp.IDPTypeGoogle,
	})
	common.RegisterAuthenticator(common.AuthenticatorMeta{
		Name:    common.AuthenticatorMagicLink,
		Factors: []common.AuthenticationFactor{common.FactorPossession},
	})

	authnService := newAuthenticationService(
		idpSvc,
		jwtSvc,
		authAssertGen,
		authnProvider,
		otpSvc,
		magicLinkSvc,
		oauthSvc,
		oidcSvc,
		googleSvc,
		githubSvc,
		passkeySvc,
	)

	authnHandler := newAuthenticationHandler(authnService)
	registerRoutes(mux, authnHandler)

	// Register MCP tools
	if mcpServer != nil {
		reactsdk.RegisterTools(mcpServer)
	}

	return authnService
}

// registerRoutes registers the routes for the authentication.
func registerRoutes(mux *http.ServeMux, authnHandler *authenticationHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	// Credentials authentication routes
	mux.HandleFunc(middleware.WithCORS("POST /auth/credentials/authenticate",
		authnHandler.HandleCredentialsAuthRequest, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /auth/credentials/authenticate",
		optionsNoContentHandler, opts))

	// SMS OTP routes
	mux.HandleFunc(middleware.WithCORS("POST /auth/otp/sms/send",
		authnHandler.HandleSendSMSOTPRequest, opts))
	mux.HandleFunc(middleware.WithCORS("POST /auth/otp/sms/verify",
		authnHandler.HandleVerifySMSOTPRequest, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /auth/otp/sms/send",
		optionsNoContentHandler, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /auth/otp/sms/verify",
		optionsNoContentHandler, opts))

	// Google OAuth routes
	mux.HandleFunc(middleware.WithCORS("POST /auth/oauth/google/start",
		authnHandler.HandleGoogleAuthStartRequest, opts))
	mux.HandleFunc(middleware.WithCORS("POST /auth/oauth/google/finish",
		authnHandler.HandleGoogleAuthFinishRequest, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /auth/oauth/google/start",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /auth/oauth/google/finish",
		optionsNoContentHandler, opts))

	// GitHub OAuth routes
	mux.HandleFunc(middleware.WithCORS("POST /auth/oauth/github/start",
		authnHandler.HandleGithubAuthStartRequest, opts))
	mux.HandleFunc(middleware.WithCORS("POST /auth/oauth/github/finish",
		authnHandler.HandleGithubAuthFinishRequest, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /auth/oauth/github/start",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /auth/oauth/github/finish",
		optionsNoContentHandler, opts))

	// Standard OAuth routes
	mux.HandleFunc(middleware.WithCORS("POST /auth/oauth/standard/start",
		authnHandler.HandleStandardOAuthStartRequest, opts))
	mux.HandleFunc(middleware.WithCORS("POST /auth/oauth/standard/finish",
		authnHandler.HandleStandardOAuthFinishRequest, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /auth/oauth/standard/start",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /auth/oauth/standard/finish",
		optionsNoContentHandler, opts))

	// Passkey routes
	mux.HandleFunc(middleware.WithCORS("POST /register/passkey/start",
		authnHandler.HandlePasskeyRegisterStartRequest, opts))
	mux.HandleFunc(middleware.WithCORS("POST /register/passkey/finish",
		authnHandler.HandlePasskeyRegisterFinishRequest, opts))
	mux.HandleFunc(middleware.WithCORS("POST /auth/passkey/start",
		authnHandler.HandlePasskeyStartRequest, opts))
	mux.HandleFunc(middleware.WithCORS("POST /auth/passkey/finish",
		authnHandler.HandlePasskeyFinishRequest, opts))
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
