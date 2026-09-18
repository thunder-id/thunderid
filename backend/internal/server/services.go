// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/attributecache"
	"github.com/thunder-id/thunderid/internal/authn/assert"
	githubauthn "github.com/thunder-id/thunderid/internal/authn/github"
	googleauthn "github.com/thunder-id/thunderid/internal/authn/google"
	"github.com/thunder-id/thunderid/internal/authn/magiclink"
	oauthauthn "github.com/thunder-id/thunderid/internal/authn/oauth"
	"github.com/thunder-id/thunderid/internal/authn/oidc"
	"github.com/thunder-id/thunderid/internal/authn/openid4vp"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/flow/graphbuilder"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	flowsession "github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/serverconfig"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/importer"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability"
	"github.com/thunder-id/thunderid/internal/system/template"
	"github.com/thunder-id/thunderid/internal/user"
	"github.com/thunder-id/thunderid/internal/vc/credential"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Services is what the shared build produces and a plane's runtime wiring consumes.
//
// It exists so the two halves can be read apart from each other. Everything a runtime surface needs
// is named here, which turns the split into a list rather than a judgement call: a value that is not
// on this struct cannot be reached by the runtime half, so adding one is a deliberate act rather
// than something that happens because a variable stayed in scope.
type Services struct {
	Mux       *http.ServeMux
	Logger    *log.Logger
	MCPServer *mcpsdk.Server

	JWTService       jwt.JWTServiceInterface
	JWEService       jwe.JWEServiceInterface
	RuntimeCryptoSvc providers.RuntimeCryptoProvider
	ImportService    importer.ImportServiceInterface
	ObservabilitySvc observability.ObservabilityServiceInterface

	// The management services a runtime surface reads from.
	OUService          ou.ConfigurableOUService
	I18nService        i18nmgt.I18nServiceInterface
	IDPService         idp.IDPServiceInterface
	UserService        user.UserServiceInterface
	ApplicationService application.ApplicationServiceInterface
	ResourceService    resource.ResourceServiceInterface
	FlowMgtService     flowmgt.FlowMgtServiceInterface

	AuthZService   providers.AuthorizationProvider
	EntityProvider entityprovider.EntityProviderInterface
	ActorProvider  providers.ActorProvider
	AuthnProvider  providers.AuthnProviderManager
	AuthAssertGen  assert.AuthAssertGeneratorInterface

	ServerConfigService   serverconfig.ServerConfigService
	SessionService        flowsession.Service
	AttributeCacheService attributecache.AttributeCacheServiceInterface
	RuntimeStoreProvider  providers.RuntimeStoreProvider
	Transactioner         providers.Transactioner

	DPoPVerifier           dpop.VerifierInterface
	RevocationEnforcer     revocation.EnforcementServiceInterface
	RevocationSvc          revocation.RevocationServiceInterface
	ResourceServerProvider providers.ResourceServerProvider

	// Flow execution needs the registries and graph builder the shared build assembled.
	ExecRegistry        executor.ExecutorRegistryInterface
	InterceptorRegistry interceptor.InterceptorRegistryInterface
	GraphBuilder        graphbuilder.GraphBuilderInterface
	FlowConfig          flowconfig.Config

	OAuthCfg          oauthconfig.Config
	OpenID4VCICredSvc credential.CredentialConfigurationServiceInterface

	// OpenID4VPSvc is built by the shared build because the management surface reads presentation
	// definitions through it. Its endpoints are runtime, and are mounted only by a plane that
	// serves them.
	OpenID4VPSvc openid4vp.Service

	// The authenticators the Direct API surface presents.
	OTPCoreService     otp.OTPAuthnServiceInterface
	NotifSenderSvc     notification.NotificationSenderServiceInterface
	TemplateService    template.TemplateServiceInterface
	MagicLinkService   magiclink.MagicLinkAuthnServiceInterface
	OAuthAuthnService  oauthauthn.OAuthAuthnServiceInterface
	OIDCAuthnService   oidc.OIDCAuthnServiceInterface
	GoogleAuthnService googleauthn.GoogleOIDCAuthnServiceInterface
	GitHubAuthnService githubauthn.GithubOAuthAuthnServiceInterface
	DirectAuthSecret   string
}
