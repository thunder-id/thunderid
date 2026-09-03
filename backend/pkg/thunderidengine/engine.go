// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package thunderidengine provides the core engine for the Thunder ID platform.
package thunderidengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/internal/attributecache"
	"github.com/thunder-id/thunderid/internal/authn/assert"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	"github.com/thunder-id/thunderid/internal/flow/flowmeta"
	"github.com/thunder-id/thunderid/internal/flow/graphbuilder"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	"github.com/thunder-id/thunderid/internal/oauth"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jti"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/runtimestore"
	"github.com/thunder-id/thunderid/internal/system/cache"
	systemconfig "github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cors"
	"github.com/thunder-id/thunderid/internal/system/jose"
	joseconfig "github.com/thunder-id/thunderid/internal/system/jose/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm/pki"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Engine is the ThunderID runtime engine that wires core platform services.
type Engine struct {
	engineCtx *engineContext
}

// New creates and initializes a ThunderID engine with the given HTTP mux and options.
func New(mux *http.ServeMux, opts ...Option) *Engine {
	logger := log.GetLogger()
	ctx := context.Background()

	var engineCtx engineContext
	for _, opt := range opts {
		opt(&engineCtx)
	}

	err := validateEngineContext(&engineCtx)
	if err != nil {
		logger.Fatal(ctx, "Engine context is missing required fields", log.Error(err))
	}

	// Apply the logging configuration before any service is initialized, so the engine's
	// own boot logging is emitted at the configured level and format.
	if engineCtx.logConfig.Level != "" {
		if err := logger.SetLevel(engineCtx.logConfig.Level); err != nil {
			logger.Fatal(ctx, "invalid log level in LogConfig", log.Error(err))
		}
	}
	if engineCtx.logConfig.Format != "" {
		// SetFormat rather than Configure: the engine owns the format but not where the
		// records go. A host application may have configured console and file output
		// already, and Configure would replace it with a console-only sink and close the
		// host's file writer.
		if err := logger.SetFormat(engineCtx.logConfig.Format); err != nil {
			logger.Fatal(ctx, "failed to configure logger", log.Error(err))
		}
	}

	// Initialize the cache manager.
	engineCtx.cacheManager = cache.Initialize(engineCtx.cacheConfig, engineCtx.serverConfig.Identifier)

	sysConfig := systemconfig.Config{
		GateClient: engineCtx.gateClientConfig,
		Crypto: systemconfig.CryptoConfig{
			Keys:       engineCtx.keyConfigs,
			Encryption: engineCtx.encryptionConfig,
		},
		AttributeCache: engineCtx.attributeCacheConfig,
	}

	err = systemconfig.InitializeServerRuntime(engineCtx.serverHome, &sysConfig)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize Server Runtime", log.Error(err))
	}

	if engineCtx.runtimeCryptoSvc == nil {
		logger.Debug(ctx, "runtimeCryptoSvc is not set, Starting to Initialize ThunderID defaultkm")
		// Load the server's private key for signing JWTs.
		pkiService, err := pki.Initialize()
		if err != nil {
			logger.Fatal(ctx, "Failed to initialize certificate service", log.Error(err))
		}
		engineCtx.runtimeCryptoSvc, _, err = kmprovider.Initialize(pkiService)
		if err != nil {
			logger.Fatal(ctx, "Failed to initialize key manager provider", log.Error(err))
		}
	}

	// Initialize JOSE services for JWT and JWE handling.
	engineCtx.jwtService, engineCtx.jweService, err = jose.Initialize(
		engineCtx.runtimeCryptoSvc, engineCtx.joseConfig())
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize JOSE services", log.Error(err))
	}

	if engineCtx.runtimeStoreProvider == nil {
		engineCtx.runtimeStoreProvider, engineCtx.transactioner, err = runtimestore.Initialize(
			engineCtx.runtimeTransientDBType, engineCtx.serverConfig.Identifier)
		if err != nil {
			logger.Fatal(ctx, "Failed to initialize runtime store", log.Error(err))
		}
	}

	engineCtx.attributeCacheService = attributecache.Initialize(engineCtx.runtimeStoreProvider,
		engineCtx.runtimeCryptoSvc, systemconfig.GetServerRuntime().Config.AttributeCache.Encryption.Enabled)
	engineCtx.authAssertGen = assert.Initialize()

	authnProviderManager, err := authnprovidermgr.Initialize(
		engineCtx.defaultAuthnProvider, engineCtx.customAuthnProviders)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize authn provider manager", log.Error(err))
	}

	// Initialize flow metadata service
	_ = flowmeta.Initialize(mux, engineCtx.actorProvider, engineCtx.ouProvider,
		engineCtx.designResolveProvider, engineCtx.i18nProvider)

	// Initialize flow core services.
	flowConfig := flowconfig.Config{
		Flow: engineCtx.flowConfig,
	}
	flowFactory, graphCache := core.Initialize(engineCtx.cacheManager)
	engineCtx.flowFactory = flowFactory
	execDeps := executor.ExecutorDependencies{
		FlowFactory:       engineCtx.flowFactory,
		AttributeCacheSvc: engineCtx.attributeCacheService,
		AuthZService:      engineCtx.authzProvider,
		ConsentEnforcer:   engineCtx.consentProvider,
		AuthnProvider:     authnProviderManager,
		JWTService:        engineCtx.jwtService,
		AuthAssertGen:     engineCtx.authAssertGen,
		ResourceService:   engineCtx.resourceProvider,
	}
	interceptorDeps := interceptor.InterceptorDependencies{
		FlowFactory:    engineCtx.flowFactory,
		CaptchaService: engineCtx.captchaValidationProvider,
	}

	engineCtx.execRegistry, err = executor.Initialize(execDeps, flowConfig.Flow)
	if err != nil {
		logger.Fatal(ctx, "Failed to register flow executors", log.Error(err))
	}
	err = engineCtx.applyCustomExecutors()
	if err != nil {
		logger.Fatal(ctx, "Failed to apply custom executors", log.Error(err))
	}

	engineCtx.interceptorRegistry, err = interceptor.Initialize(interceptorDeps, flowConfig.Flow)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize Interceptor registry", log.Error(err))
	}

	engineCtx.graphBuilder = graphbuilder.Initialize(engineCtx.flowFactory, engineCtx.execRegistry,
		engineCtx.interceptorRegistry, graphCache)

	engineCtx.flowExecService, err = flowexec.Initialize(mux, engineCtx.flowProvider, engineCtx.actorProvider,
		engineCtx.execRegistry, engineCtx.interceptorRegistry, engineCtx.observabilitySvc,
		engineCtx.runtimeCryptoSvc, engineCtx.attestationProvider, engineCtx.graphBuilder,
		engineCtx.jwtService, engineCtx.runtimeStoreProvider, engineCtx.transactioner, nil, flowConfig)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize flow execution service", log.Error(err))
	}

	oauthConfig := oauthconfig.Config{
		DeploymentID:           engineCtx.serverConfig.Identifier,
		RuntimeTransientDBType: engineCtx.runtimeTransientDBType,
		BaseURL:                config.GetServerURL(&engineCtx.serverConfig),
		JWT:                    engineCtx.jwtConfig,
		OAuth:                  engineCtx.oauthConfig,
		GateClient:             engineCtx.gateClientConfig,
	}

	engineCtx.dpopVerifier = dpop.Initialize(oauthConfig, jti.Initialize(engineCtx.runtimeStoreProvider),
		engineCtx.runtimeCryptoSvc)
	tokenFamilyRevocationTTL := time.Duration(engineCtx.oauthConfig.RefreshToken.ValidityPeriod) * time.Second
	revocationEnforcer, revocationService := revocation.Initialize(engineCtx.jwtService,
		engineCtx.observabilitySvc, tokenFamilyRevocationTTL,
		engineCtx.oauthConfig.Revocation.TokenFamily.OnExplicitRevokeEnabled())

	// CORS origins come from the engine's static OriginConfig; unlike the full server there is no
	// server-config store to back a dynamic matcher. An empty config leaves CORS disabled (matcher
	// reset to nil, so every cross-origin request to the routes registered below is denied). The
	// matcher is global, so it must be reset explicitly rather than left as whatever a previous
	// engine installed.
	originReader, err := buildOriginReader(engineCtx.originConfig)
	if err != nil {
		logger.Fatal(ctx, "Invalid origin configuration", log.Error(err))
	}
	cors.InitializeDynamicMatcher(originReader)

	// The embedded engine has no server-config store, so no default resource server is available: the
	// resource provider is passed undecorated. Implicit no-resource requests that carry permission
	// scopes are rejected (the provider resolves no server for an empty identifier); OIDC-only or
	// scopeless requests do not need resource-server binding.
	_, err = oauth.Initialize(mux, engineCtx.actorProvider, authnProviderManager, engineCtx.jwtService,
		engineCtx.jweService, engineCtx.flowExecService, engineCtx.observabilitySvc, engineCtx.runtimeCryptoSvc,
		engineCtx.ouProvider, engineCtx.attributeCacheService, engineCtx.authzProvider, engineCtx.resourceProvider,
		engineCtx.i18nProvider, engineCtx.idpProvider, engineCtx.dpopVerifier, engineCtx.runtimeStoreProvider,
		engineCtx.transactioner, revocationEnforcer, revocationService, oauthConfig)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize OAuth services", log.Error(err))
	}

	return &Engine{
		engineCtx: &engineCtx,
	}
}

// validateEngineContext checks that the engine context has all required fields set.
func validateEngineContext(ctx *engineContext) error {
	if ctx.serverHome == "" {
		return errors.New("thunderidengine: server home directory is not set")
	}
	if ctx.serverConfig.Identifier == "" {
		return errors.New("thunderidengine: server identifier is not set")
	}
	if ctx.observabilitySvc == nil {
		return errors.New("thunderidengine: observability provider is not set")
	}
	if ctx.authzProvider == nil {
		return errors.New("thunderidengine: authorization provider is not set")
	}
	if ctx.actorProvider == nil {
		return errors.New("thunderidengine: actor provider is not set")
	}
	if ctx.resourceProvider == nil {
		return errors.New("thunderidengine: resource server provider is not set")
	}
	if ctx.ouProvider == nil {
		return errors.New("thunderidengine: organization unit provider is not set")
	}
	if ctx.designResolveProvider == nil {
		return errors.New("thunderidengine: design provider is not set")
	}
	if ctx.flowProvider == nil {
		return errors.New("thunderidengine: flow provider is not set")
	}
	if ctx.i18nProvider == nil {
		return errors.New("thunderidengine: i18n provider is not set")
	}
	if ctx.idpProvider == nil {
		return errors.New("thunderidengine: idp provider is not set")
	}
	if ctx.consentProvider == nil {
		return errors.New("thunderidengine: consent provider is not set")
	}
	if ctx.defaultAuthnProvider == nil {
		return errors.New("thunderidengine: default authentication provider is not set")
	}
	return nil
}

// joseConfig builds the JOSE configuration from the engine's injected JWT and server
// configuration, decoupling the JOSE services from the global server runtime.
func (e *engineContext) joseConfig() joseconfig.Config {
	return joseconfig.Config{
		Issuer:         e.jwtConfig.Issuer,
		ValidityPeriod: e.jwtConfig.ValidityPeriod,
		Audience:       e.jwtConfig.Audience,
		PreferredKeyID: e.jwtConfig.PreferredKeyID,
		Leeway:         e.jwtConfig.Leeway,
		JWKSCacheTTL:   time.Duration(e.serverConfig.SecurityConfig.JWKSCacheTTL) * time.Second,
	}
}

// applyCustomExecutors registers the custom executors with the executor registry.
func (e *engineContext) applyCustomExecutors() error {
	if len(e.customExecutors) == 0 {
		return nil
	}
	if e.execRegistry == nil {
		return errors.New("thunderidengine: executor registry is nil")
	}
	for name, ex := range e.customExecutors {
		e.execRegistry.RegisterExecutor(name, ex)
	}
	return nil
}

// buildOriginReader validates cfg and, if it carries any allowed origins, returns a
// cors.ServerConfigReader that serves them as a static read-only layer with no writable layer.
// An empty cfg returns a nil reader and no error: the caller then leaves the CORS dynamic matcher
// uninstalled, which denies every cross-origin request.
func buildOriginReader(cfg engineconfig.OriginConfig) (cors.ServerConfigReader, error) {
	if len(cfg.AllowedOrigins) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("thunderidengine: failed to encode origin configuration: %w", err)
	}
	decoded, err := cors.OriginHandler{}.Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("thunderidengine: invalid origin configuration: %w", err)
	}
	if err := (cors.OriginHandler{}).Validate(decoded, nil, nil); err != nil {
		return nil, fmt.Errorf("thunderidengine: invalid origin configuration: %w", err)
	}
	return staticOriginReader{config: decoded.(cors.OriginConfig)}, nil
}

// staticOriginReader implements cors.ServerConfigReader over a fixed origin list supplied via
// WithOriginConfig. It has no writable layer: origins can only change by restarting the engine
// with a new OriginConfig.
type staticOriginReader struct {
	config cors.OriginConfig
}

// GetReadOnlyConfig returns the engine's static origin configuration.
func (r staticOriginReader) GetReadOnlyConfig(_ context.Context, _ string) (any, *common.ServiceError) {
	return r.config, nil
}

// GetWritableConfig returns an empty configuration: staticOriginReader has no writable layer.
func (r staticOriginReader) GetWritableConfig(_ context.Context, _ string) (any, *common.ServiceError) {
	return cors.OriginConfig{}, nil
}

type engineContext struct {
	cacheManager          cache.CacheManagerInterface
	jwtService            jwt.JWTServiceInterface
	jweService            jwe.JWEServiceInterface
	flowFactory           core.FlowFactoryInterface
	execRegistry          executor.ExecutorRegistryInterface
	interceptorRegistry   interceptor.InterceptorRegistryInterface
	graphBuilder          graphbuilder.GraphBuilderInterface
	authAssertGen         assert.AuthAssertGeneratorInterface
	dpopVerifier          dpop.VerifierInterface
	flowExecService       flowexec.FlowExecServiceInterface
	attributeCacheService attributecache.AttributeCacheServiceInterface

	serverHome             string
	runtimeTransientDBType string
	oauthConfig            engineconfig.OAuthConfig
	jwtConfig              engineconfig.JWTConfig
	flowConfig             engineconfig.FlowConfig
	serverConfig           engineconfig.ServerConfig
	cacheConfig            engineconfig.CacheConfig
	observabilityConfig    engineconfig.ObservabilityConfig
	gateClientConfig       engineconfig.GateClientConfig
	keyConfigs             []engineconfig.KeyConfig
	encryptionConfig       engineconfig.EncryptionConfig
	logConfig              engineconfig.LogConfig
	attributeCacheConfig   engineconfig.AttributeCacheConfig
	originConfig           engineconfig.OriginConfig

	actorProvider             providers.ActorProvider
	defaultAuthnProvider      providers.AuthnProviderInterface
	customAuthnProviders      map[string]providers.CustomAuthnProvider
	resourceProvider          providers.ResourceServerProvider
	ouProvider                providers.OrganizationUnitProvider
	designResolveProvider     providers.DesignProvider
	flowProvider              providers.FlowProvider
	i18nProvider              providers.I18nProvider
	idpProvider               providers.IDPProvider
	consentProvider           providers.ConsentProvider
	customExecutors           map[string]providers.Executor
	observabilitySvc          providers.ObservabilityProvider
	authzProvider             providers.AuthorizationProvider
	attestationProvider       providers.AttestationProvider
	captchaValidationProvider providers.CaptchaValidationProvider

	transactioner        providers.Transactioner
	runtimeStoreProvider providers.RuntimeStoreProvider
	runtimeCryptoSvc     kmprovider.RuntimeCryptoProvider
}

// Option configures engine initialization.
type Option func(*engineContext)

// WithServerHome supplies the server home directory used for all runtime
// state. Required.
func WithServerHome(serverHome string) Option {
	return func(c *engineContext) { c.serverHome = serverHome }
}

// WithRuntimeTransientDBType supplies the RuntimeStore DB type.
func WithRuntimeTransientDBType(runtimeTransientDBType string) Option {
	return func(c *engineContext) { c.runtimeTransientDBType = runtimeTransientDBType }
}

// WithKeyConfigs supplies the keyconfigs.
func WithKeyConfigs(keyConfigs []engineconfig.KeyConfig) Option {
	return func(c *engineContext) { c.keyConfigs = keyConfigs }
}

// WithEncryptionConfig supplies the encryption configs.
func WithEncryptionConfig(encryptionConfig engineconfig.EncryptionConfig) Option {
	return func(c *engineContext) { c.encryptionConfig = encryptionConfig }
}

// WithAttributeCacheConfig supplies the attribute cache configuration.
func WithAttributeCacheConfig(config engineconfig.AttributeCacheConfig) Option {
	return func(c *engineContext) { c.attributeCacheConfig = config }
}

// WithOriginConfig supplies the allowed cross-origin origins for the engine's CORS-enabled
// endpoints (well-known discovery, JWKS, token, userinfo, and the rest of the OAuth surface).
// Omitting this option leaves CORS disabled: no cross-origin request is allowed to read a
// response from those endpoints.
func WithOriginConfig(config engineconfig.OriginConfig) Option {
	return func(c *engineContext) { c.originConfig = config }
}

// WithServerConfig supplies the server configuration.
func WithServerConfig(config engineconfig.ServerConfig) Option {
	return func(c *engineContext) { c.serverConfig = config }
}

// WithCacheConfig supplies the cache configuration.
func WithCacheConfig(config engineconfig.CacheConfig) Option {
	return func(c *engineContext) { c.cacheConfig = config }
}

// WithGateClientConfig supplies the gate client configuration.
func WithGateClientConfig(config engineconfig.GateClientConfig) Option {
	return func(c *engineContext) { c.gateClientConfig = config }
}

// WithOAuthConfig supplies the OAuth configuration.
func WithOAuthConfig(config engineconfig.OAuthConfig) Option {
	return func(c *engineContext) { c.oauthConfig = config }
}

// WithJWTConfig supplies the JWT configuration.
func WithJWTConfig(config engineconfig.JWTConfig) Option {
	return func(c *engineContext) { c.jwtConfig = config }
}

// WithFlowConfig supplies the flow configuration.
func WithFlowConfig(config engineconfig.FlowConfig) Option {
	return func(c *engineContext) { c.flowConfig = config }
}

// WithObservabilityConfig supplies the observability configuration.
func WithObservabilityConfig(config engineconfig.ObservabilityConfig) Option {
	return func(c *engineContext) { c.observabilityConfig = config }
}

// WithActorProvider supplies the actor provider.
func WithActorProvider(provider providers.ActorProvider) Option {
	return func(c *engineContext) { c.actorProvider = provider }
}

// WithDefaultAuthnProvider supplies the default authentication provider.
func WithDefaultAuthnProvider(provider providers.AuthnProviderInterface) Option {
	return func(c *engineContext) { c.defaultAuthnProvider = provider }
}

// WithCustomAuthnProvider supplies a custom authentication provider with its associated credential keys.
func WithCustomAuthnProvider(name string, provider providers.CustomAuthnProvider) Option {
	return func(c *engineContext) {
		if c.customAuthnProviders == nil {
			c.customAuthnProviders = make(map[string]providers.CustomAuthnProvider)
		}
		c.customAuthnProviders[name] = provider
	}
}

// WithResourceProvider supplies the resource provider.
func WithResourceProvider(provider providers.ResourceServerProvider) Option {
	return func(c *engineContext) { c.resourceProvider = provider }
}

// WithOUProvider supplies the organization unit provider.
func WithOUProvider(provider providers.OrganizationUnitProvider) Option {
	return func(c *engineContext) { c.ouProvider = provider }
}

// WithDesignResolveProvider supplies the design resolve provider.
func WithDesignResolveProvider(provider providers.DesignProvider) Option {
	return func(c *engineContext) { c.designResolveProvider = provider }
}

// WithFlowProvider supplies the flow provider.
func WithFlowProvider(provider providers.FlowProvider) Option {
	return func(c *engineContext) { c.flowProvider = provider }
}

// WithI18nProvider supplies the i18n provider.
func WithI18nProvider(provider providers.I18nProvider) Option {
	return func(c *engineContext) { c.i18nProvider = provider }
}

// WithIDPProvider supplies the IDP provider.
func WithIDPProvider(provider providers.IDPProvider) Option {
	return func(c *engineContext) { c.idpProvider = provider }
}

// WithConsentProvider supplies the consent provider.
func WithConsentProvider(provider providers.ConsentProvider) Option {
	return func(c *engineContext) { c.consentProvider = provider }
}

// WithCustomExecutors supplies the custom executors to be registered with the engine.
func WithCustomExecutors(executors map[string]providers.Executor) Option {
	return func(c *engineContext) {
		if c.customExecutors == nil {
			c.customExecutors = make(map[string]providers.Executor, len(executors))
		}
		for name, ex := range executors {
			c.customExecutors[name] = ex
		}
	}
}

// WithObservabilityProvider supplies the observability provider.
func WithObservabilityProvider(provider providers.ObservabilityProvider) Option {
	return func(c *engineContext) { c.observabilitySvc = provider }
}

// WithAuthorizationProvider supplies the authorization provider.
func WithAuthorizationProvider(provider providers.AuthorizationProvider) Option {
	return func(c *engineContext) { c.authzProvider = provider }
}

// WithRuntimeStoreProvider supplies the RuntimeStore provider.
func WithRuntimeStoreProvider(provider providers.RuntimeStoreProvider) Option {
	return func(c *engineContext) { c.runtimeStoreProvider = provider }
}

// WithLogConfig supplies the log level and format configuration.
func WithLogConfig(config engineconfig.LogConfig) Option {
	return func(ctx *engineContext) {
		ctx.logConfig = config
	}
}

// WithAttestationProvider supplies the Attestation provider.
func WithAttestationProvider(provider providers.AttestationProvider) Option {
	return func(c *engineContext) { c.attestationProvider = provider }
}

// WithTransactioner supplies the Transactioner.
func WithTransactioner(provider providers.Transactioner) Option {
	return func(c *engineContext) { c.transactioner = provider }
}

// WithCaptchaValidationProvider supplies the CaptchaValidationProvider.
func WithCaptchaValidationProvider(provider providers.CaptchaValidationProvider) Option {
	return func(c *engineContext) { c.captchaValidationProvider = provider }
}

// WithRuntimeCryptoProvider supplies the RuntimeCryptoProvider.
func WithRuntimeCryptoProvider(provider kmprovider.RuntimeCryptoProvider) Option {
	return func(c *engineContext) { c.runtimeCryptoSvc = provider }
}
