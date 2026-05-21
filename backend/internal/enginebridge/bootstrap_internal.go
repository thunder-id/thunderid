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

package enginebridge

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/attributecache"
	authnAssert "github.com/thunder-id/thunderid/internal/authn/assert"
	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnConsent "github.com/thunder-id/thunderid/internal/authn/consent"
	"github.com/thunder-id/thunderid/internal/authn/github"
	"github.com/thunder-id/thunderid/internal/authn/google"
	"github.com/thunder-id/thunderid/internal/authn/magiclink"
	authnOAuth "github.com/thunder-id/thunderid/internal/authn/oauth"
	authnOIDC "github.com/thunder-id/thunderid/internal/authn/oidc"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/authz"
	"github.com/thunder-id/thunderid/internal/cert"
	"github.com/thunder-id/thunderid/internal/consent"
	layoutmgt "github.com/thunder-id/thunderid/internal/design/layout/mgt"
	"github.com/thunder-id/thunderid/internal/design/resolve"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	flowcore "github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	"github.com/thunder-id/thunderid/internal/notification"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cors"
	"github.com/thunder-id/thunderid/internal/system/cryptolab/hash"
	dbprovider "github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/email"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/jose"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm/pkiservice"
	"github.com/thunder-id/thunderid/internal/system/observability"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/template"
)

type internalServices struct {
	JWTService      jwt.JWTServiceInterface
	JWEService      jwe.JWEServiceInterface
	RuntimeCrypto   kmprovider.RuntimeCryptoProvider
	Observability   observability.ObservabilityServiceInterface
	AttributeCache  attributecache.AttributeCacheServiceInterface
	InboundClient   inboundclient.InboundClientServiceInterface
	AuthnProvider   authnprovidermgr.AuthnProviderManagerInterface
	AuthzService    authz.AuthorizationServiceInterface
	ResourceService resource.ResourceServiceInterface
	OUService       ou.OrganizationUnitServiceInterface
	IDPService      idp.IDPServiceInterface
	EntityProvider  entityprovider.EntityProviderInterface
	FlowMgtService  flowmgt.FlowMgtServiceInterface
	ExecRegistry    executor.ExecutorRegistryInterface
	DesignResolve   resolve.DesignResolveServiceInterface
	I18nService     i18nmgt.I18nServiceInterface
}

func initPlatform(cfg *config.Config) (cache.CacheManagerInterface, error) {
	cacheManager := cache.Initialize()
	if err := cors.InitializeMatcher(cfg.CORS.AllowedOrigins); err != nil {
		return nil, err
	}
	security.InitSystemPermissions(cfg.Resource.SystemResourceServer.Handle)
	return cacheManager, nil
}

func initCrypto() (jwt.JWTServiceInterface, jwe.JWEServiceInterface, kmprovider.RuntimeCryptoProvider, error) {
	pkiService, err := pkiservice.Initialize()
	if err != nil {
		return nil, nil, nil, err
	}
	configCryptoSvc, err := defaultkm.InitConfigProvider()
	if err != nil {
		return nil, nil, nil, err
	}
	runtimeCrypto := defaultkm.NewRuntimeCryptoService(pkiService, configCryptoSvc)
	jwtService, jweService, err := jose.Initialize(pkiService)
	if err != nil {
		return nil, nil, nil, err
	}
	return jwtService, jweService, runtimeCrypto, nil
}

func bootstrapInternalServices(
	adminMux *http.ServeMux, cacheManager cache.CacheManagerInterface,
) (*internalServices, error) {
	jwtService, jweService, runtimeCrypto, err := initCrypto()
	if err != nil {
		return nil, err
	}
	observabilitySvc := observability.Initialize()
	attributeCacheSvc := attributecache.Initialize()

	i18nService, _, err := i18nmgt.Initialize(adminMux)
	if err != nil {
		return nil, err
	}
	ouAuthzService, err := sysauthz.Initialize()
	if err != nil {
		return nil, err
	}
	ouService, ouHierarchyResolver, _, err := ou.Initialize(adminMux, nil, cacheManager, ouAuthzService)
	if err != nil {
		return nil, err
	}
	ouAuthzService.SetOUHierarchyResolver(ouHierarchyResolver)

	hashCfg, err := buildHashConfig()
	if err != nil {
		return nil, err
	}
	hashService, err := hash.Initialize(hashCfg)
	if err != nil {
		return nil, err
	}
	consentService := consent.Initialize()
	entityTypeService, _, err := entitytype.Initialize(
		adminMux, nil, cacheManager, ouService, ouAuthzService, consentService,
	)
	if err != nil {
		return nil, err
	}
	entityService, err := entity.Initialize(cacheManager, hashService, entityTypeService, ouService)
	if err != nil {
		return nil, err
	}
	entityProvider := entityprovider.InitializeEntityProvider(entityService)
	groupService, ouGroupResolver, _, err := group.Initialize(
		adminMux, dbprovider.GetDBProvider(), ouService, entityService, entityTypeService, ouAuthzService,
	)
	if err != nil {
		return nil, err
	}
	ouService.SetOUGroupResolver(ouGroupResolver)

	resourceService, _, err := resource.Initialize(adminMux, ouService, consentService)
	if err != nil {
		return nil, err
	}
	roleService, roleAssignmentService, _, err := role.Initialize(
		adminMux, entityService, groupService, ouService, resourceService, entityTypeService,
	)
	if err != nil {
		return nil, err
	}
	authZService := authz.Initialize(roleService)
	idpService, _, err := idp.Initialize(cacheManager, adminMux)
	if err != nil {
		return nil, err
	}
	templateService, err := template.Initialize()
	if err != nil {
		return nil, err
	}
	_, otpService, notifSenderSvc, _, err := notification.Initialize(adminMux, jwtService, templateService)
	if err != nil {
		return nil, err
	}
	passkeyService := passkey.Initialize(entityService)
	magicLinkService := magiclink.Initialize(jwtService, entityProvider)
	otpCoreService := otp.Initialize(otpService, entityProvider)
	oauthAuthnService := authnOAuth.Initialize(idpService, entityProvider)
	oidcAuthnService := authnOIDC.Initialize(oauthAuthnService, jwtService)
	googleAuthnService := google.Initialize(oidcAuthnService, jwtService)
	githubAuthnService := github.Initialize(oauthAuthnService)
	federatedAuths := map[idp.IDPType]authncm.FederatedAuthenticator{
		idp.IDPTypeOAuth:  oauthAuthnService,
		idp.IDPTypeOIDC:   oidcAuthnService,
		idp.IDPTypeGoogle: googleAuthnService,
		idp.IDPTypeGitHub: githubAuthnService,
	}
	authnProvider := authnprovidermgr.InitializeAuthnProviderManager(
		entityService, passkeyService, otpCoreService, federatedAuths,
	)
	authAssertGen := authnAssert.Initialize()
	consentEnforcer := authnConsent.Initialize(consentService, jwtService)

	flowFactory, graphCache := flowcore.Initialize(cacheManager)
	var emailClient email.EmailClientInterface
	emailClient, err = email.Initialize()
	if err != nil {
		emailClient = nil
	}
	execRegistry := executor.Initialize(executor.ExecutorDeps{
		FlowFactory:           flowFactory,
		OUService:             ouService,
		IDPService:            idpService,
		NotifSenderSvc:        notifSenderSvc,
		JWTService:            jwtService,
		AuthAssertGen:         authAssertGen,
		ConsentEnforcer:       consentEnforcer,
		AuthnProvider:         authnProvider,
		OTPService:            otpCoreService,
		PasskeyService:        passkeyService,
		MagicLinkService:      magicLinkService,
		AuthZService:          authZService,
		EntityTypeService:     entityTypeService,
		GroupService:          groupService,
		RoleService:           roleService,
		RoleAssignmentService: roleAssignmentService,
		EntityProvider:        entityProvider,
		AttributeCacheSvc:     attributeCacheSvc,
		EmailClient:           emailClient,
		TemplateService:       templateService,
		OAuthSvc:              oauthAuthnService,
		OIDCSvc:               oidcAuthnService,
		GithubSvc:             githubAuthnService,
		GoogleSvc:             googleAuthnService,
	})
	flowMgtService, err := flowmgt.InitializeRuntimeService(cacheManager, flowFactory, execRegistry, graphCache)
	if err != nil {
		return nil, err
	}
	certService, err := cert.Initialize(cacheManager, dbprovider.GetDBProvider())
	if err != nil {
		return nil, err
	}
	themeMgtService, _, err := thememgt.Initialize(adminMux, nil)
	if err != nil {
		return nil, err
	}
	layoutMgtService, _, err := layoutmgt.Initialize(adminMux)
	if err != nil {
		return nil, err
	}
	inboundClientService, err := inboundclient.Initialize(
		cacheManager, certService, entityProvider, themeMgtService, layoutMgtService,
		flowMgtService, entityTypeService, consentService,
	)
	if err != nil {
		return nil, err
	}
	applicationService, _, err := application.Initialize(
		adminMux, nil, entityProvider, entityService, inboundClientService, ouService, i18nService,
	)
	if err != nil {
		return nil, err
	}
	designResolveService := resolve.Initialize(adminMux, themeMgtService, layoutMgtService, applicationService)

	return &internalServices{
		JWTService:      jwtService,
		JWEService:      jweService,
		RuntimeCrypto:   runtimeCrypto,
		Observability:   observabilitySvc,
		AttributeCache:  attributeCacheSvc,
		InboundClient:   inboundClientService,
		AuthnProvider:   authnProvider,
		AuthzService:    authZService,
		ResourceService: resourceService,
		OUService:       ouService,
		IDPService:      idpService,
		EntityProvider:  entityProvider,
		FlowMgtService:  flowMgtService,
		ExecRegistry:    execRegistry,
		DesignResolve:   designResolveService,
		I18nService:     i18nService,
	}, nil
}

func buildHashConfig() (hash.HashConfig, error) {
	cfg := config.GetServerRuntime().Config.Crypto.PasswordHashing
	alg := hash.CredAlgorithm(strings.ToUpper(cfg.Algorithm))
	switch alg {
	case "", hash.SHA256:
		return hash.HashConfig{Algorithm: hash.SHA256, SaltSize: cfg.SHA256.SaltSize}, nil
	case hash.PBKDF2:
		return hash.HashConfig{Algorithm: alg, SaltSize: cfg.PBKDF2.SaltSize,
			Iterations: cfg.PBKDF2.Iterations, KeySize: cfg.PBKDF2.KeySize}, nil
	case hash.ARGON2ID:
		return hash.HashConfig{Algorithm: alg, SaltSize: cfg.Argon2ID.SaltSize,
			Iterations: cfg.Argon2ID.Iterations, Memory: cfg.Argon2ID.Memory,
			Parallelism: cfg.Argon2ID.Parallelism, KeySize: cfg.Argon2ID.KeySize}, nil
	default:
		return hash.HashConfig{}, fmt.Errorf("unrecognized password hashing algorithm %q", cfg.Algorithm)
	}
}
