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

// Package thunderidengine is an embeddable ThunderID identity engine. It mounts the
// flow-metadata (GET /flow/meta), flow-execution (POST /flow/execute), and OAuth2/OIDC
// (/oauth2/*) endpoint groups on a caller-supplied http.ServeMux via New and
// RegisterRoutes. It is runtime-Redis-only: short-lived runtime state is persisted in
// the Redis connection supplied through WithRedis and it never opens a SQL database.
// Note: the dependency graph still links the SQL drivers (lib/pq, modernc.org/sqlite)
// although they are unused at runtime, so an embedding application must not also
// blank-import those drivers (database/sql would panic on duplicate registration).
// Dynamic Client Registration is not part of the engine.
package thunderidengine

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	authnassert "github.com/thunder-id/thunderid/internal/authn/assert"
	authnconfig "github.com/thunder-id/thunderid/internal/authn/config"
	authnconsent "github.com/thunder-id/thunderid/internal/authn/consent"
	"github.com/thunder-id/thunderid/internal/authz"
	"github.com/thunder-id/thunderid/internal/consent"
	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	flowcore "github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	"github.com/thunder-id/thunderid/internal/flow/flowmeta"
	"github.com/thunder-id/thunderid/internal/oauth"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	oauth2authz "github.com/thunder-id/thunderid/internal/oauth/oauth2/authz"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/ciba"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jti"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/par"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/redisstore"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/jose"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm/pki"
)

// Engine is an embeddable ThunderID identity engine. It exposes the flow metadata,
// flow execution, and OAuth2/OIDC endpoint groups, persisting runtime state in a
// caller-supplied Redis connection. It is constructed with New and mounted with
// RegisterRoutes.
type Engine struct {
	cfg           engineConfig
	redisProvider redisstore.RedisProviderInterface
	flowCfg       flowconfig.Config
	oauthCfg      oauthconfig.Config
}

// New constructs an Engine from the supplied options. WithRedis and all required
// providers must be set (see Option docs); otherwise New returns an error.
func New(opts ...Option) (*Engine, error) {
	var c engineConfig
	for _, opt := range opts {
		opt(&c)
	}

	// Seed the process-wide server runtime configuration before anything reads it.
	if c.serverConfig != nil {
		if err := config.InitializeServerRuntime(c.serverHome, c.serverConfig); err != nil {
			return nil, fmt.Errorf("thunderidengine: failed to initialize server runtime configuration: %w", err)
		}
	}

	if c.redisClient == nil {
		return nil, fmt.Errorf("thunderidengine: WithRedis is required")
	}
	if err := c.applyCryptoDefaults(); err != nil {
		return nil, err
	}

	// Wrap host-SDK providers as internal providers when the raw internal providers were not
	// supplied directly. This is how external embedders inject identity (see WithHostActorProvider
	// / WithHostAuthnProvider) without naming internal types.
	if c.actorProvider == nil && c.hostActorProvider != nil {
		c.actorProvider = newActorAdapter(c.hostActorProvider)
	}
	if c.authnProvider == nil && c.hostAuthnProvider != nil {
		c.authnProvider = newAuthnAdapter(c.hostAuthnProvider)
	}
	if c.roleService == nil && c.hostRoleProvider != nil {
		c.roleService = newRoleAdapter(c.hostRoleProvider)
	}
	if c.authZService == nil && c.roleService != nil {
		c.authZService = authz.Initialize(c.roleService)
	}

	flowCfg := flowconfig.FromServerRuntime()
	declarative := declarativeresource.IsDeclarativeModeEnabled()

	// Build the flow core (cache manager, flow factory, graph cache) once, when it is needed: to
	// construct the executor registry, and/or to build the declarative service graph / default
	// flow provider.
	var (
		cacheManager cache.CacheManagerInterface
		flowFactory  flowcore.FlowFactoryInterface
		graphCache   flowcore.GraphCacheInterface
	)
	needDeclarativeBase := declarative && c.needsDeclarativeBase()
	needFlowCore := c.executorRegistry == nil ||
		needDeclarativeBase ||
		(c.flowProvider == nil && declarative)
	if needFlowCore {
		runtime := config.GetServerRuntime()
		if runtime == nil {
			return nil, fmt.Errorf("thunderidengine: server runtime configuration is not initialized")
		}
		cacheManager = cache.Initialize(runtime.Config.Cache, flowCfg.DeploymentID)
		flowFactory, graphCache = flowcore.Initialize(cacheManager)
	}

	// Build the declarative default i18n service first when needed.
	if c.i18nService == nil && declarative {
		svc, err := buildDefaultI18nService()
		if err != nil {
			return nil, err
		}
		c.i18nService = svc
	}

	// Build SDK-minimal declarative services the embedder did not inject.
	var sdkDesign *sdkDeclarativeDesign
	if needDeclarativeBase {
		d, err := c.buildSDKDeclarativeServices(cacheManager)
		if err != nil {
			return nil, err
		}
		sdkDesign = d
	}

	// Build the executor registry, finish the declarative graph, and apply custom executors.
	if err := c.finalizeExecutorsAndFlow(sdkDesign, cacheManager, flowFactory, graphCache, declarative); err != nil {
		return nil, err
	}

	if err := c.validateRequiredProviders(); err != nil {
		return nil, err
	}

	return &Engine{
		cfg:           c,
		redisProvider: redisstore.New(c.redisClient, c.redisKeyPrefix),
		flowCfg:       flowCfg,
		oauthCfg:      oauthconfig.FromServerRuntime(),
	}, nil
}

// buildExecutorRegistry constructs the flow executor registry from the embedder-supplied
// ExecutorDependencies. It takes the engine-built flow factory, fills the dependencies the
// engine already holds when their fields are left nil, derives a default auth-assertion
// generator, and registers the executors selected by WithEnabledExecutors (or the server
// configuration's executor list when none were given).
func (c *engineConfig) buildExecutorRegistry(flowFactory flowcore.FlowFactoryInterface) (ExecutorRegistry, error) {
	runtime := config.GetServerRuntime()
	if runtime == nil {
		return nil, fmt.Errorf("thunderidengine: server runtime configuration is not initialized")
	}

	var deps ExecutorDependencies
	if c.executorDeps != nil {
		deps = *c.executorDeps
	}
	deps.FlowFactory = flowFactory
	if deps.EntityProvider == nil {
		deps.EntityProvider = c.actorProvider
	}
	if deps.AuthnProvider == nil {
		deps.AuthnProvider = c.authnProvider
	}
	if deps.OUService == nil {
		deps.OUService = c.ouService
	}
	if deps.JWTService == nil {
		deps.JWTService = c.jwtService
	}
	if deps.AttributeCacheSvc == nil {
		deps.AttributeCacheSvc = c.attributeCacheSvc
	}
	if deps.AuthZService == nil {
		deps.AuthZService = c.authZService
	}
	if deps.IDPService == nil {
		deps.IDPService = c.idpService
	}
	if deps.RoleService == nil {
		deps.RoleService = c.roleService
	}
	if deps.AuthAssertGen == nil {
		deps.AuthAssertGen = authnassert.Initialize()
	}

	flowSysCfg := runtime.Config.Flow
	if len(c.enabledExecutors) > 0 {
		flowSysCfg.Executors = c.enabledExecutors
	}

	if willRegisterConsentExecutor(flowSysCfg.Executors) && deps.ConsentEnforcer == nil {
		consentSvc := consent.Initialize()
		deps.ConsentEnforcer = authnconsent.Initialize(consentSvc, c.jwtService, authnconfig.FromServerRuntime())
	}

	reg, err := executor.Initialize(deps, flowSysCfg)
	if err != nil {
		return nil, fmt.Errorf("thunderidengine: failed to build executor registry: %w", err)
	}
	return reg, nil
}

// finalizeExecutorsAndFlow builds the executor registry when one was not supplied, finishes the
// declarative flow/design graph now the registry exists, and registers any custom executors on
// top. It is split out of New to keep that constructor's branching manageable.
func (c *engineConfig) finalizeExecutorsAndFlow(
	sdkDesign *sdkDeclarativeDesign,
	cacheManager cache.CacheManagerInterface,
	flowFactory flowcore.FlowFactoryInterface,
	graphCache flowcore.GraphCacheInterface,
	declarative bool,
) error {
	// Executor registry: use a pre-built one, otherwise build it now from the supplied
	// dependencies and/or the declarative services already recorded on the config.
	if c.executorRegistry == nil {
		reg, err := c.buildExecutorRegistry(flowFactory)
		if err != nil {
			return err
		}
		c.executorRegistry = reg
	}
	if c.interceptorRegistry == nil {
		reg, err := buildInterceptorRegistry(flowFactory)
		if err != nil {
			return err
		}
		c.interceptorRegistry = reg
	}

	// Finish the SDK declarative graph (flow provider + design resolve) now the registry exists.
	if sdkDesign != nil {
		if err := c.buildSDKDeclarativeFlowAndDesign(
			sdkDesign, cacheManager, flowFactory, graphCache, c.executorRegistry); err != nil {
			return err
		}
	} else if c.flowProvider == nil && declarative && c.executorRegistry != nil {
		fp, err := buildDefaultFlowProvider(
			cacheManager, flowFactory, graphCache, c.executorRegistry, c.interceptorRegistry)
		if err != nil {
			return err
		}
		c.flowProvider = fp
	}

	// Register embedder-supplied custom executors on top of the registry so they coexist with the
	// enabled built-ins (WithEnabledExecutors) or layer onto a supplied WithExecutorRegistry.
	return c.applyCustomExecutors()
}

// applyCustomExecutors registers the executors supplied through WithCustomExecutors on the
// engine's executor registry so they run alongside the enabled built-ins. An executor whose name
// matches a built-in replaces it. It is a no-op when no custom executors were supplied and errors
// when custom executors were supplied without a registry to register them on.
func (c *engineConfig) applyCustomExecutors() error {
	if len(c.customExecutors) == 0 {
		return nil
	}
	if c.executorRegistry == nil {
		return fmt.Errorf(
			"thunderidengine: WithCustomExecutors requires WithExecutorRegistry or WithExecutorDependencies")
	}
	for name, ex := range c.customExecutors {
		if ex == nil {
			return fmt.Errorf("thunderidengine: WithCustomExecutors: executor %q is nil", name)
		}
		c.executorRegistry.RegisterExecutor(name, ex)
	}
	return nil
}

// RegisterRoutes mounts the engine's endpoint groups (GET /flow/meta, POST /flow/execute,
// and /oauth2/*) on the provided mux.
func (e *Engine) RegisterRoutes(mux *http.ServeMux) error {
	// Flow execution: build the Redis-backed flow store and transactioner, then initialize.
	flowStore := flowexec.NewRedisStore(e.redisProvider, e.flowCfg.DeploymentID)
	flowTransactioner := e.redisProvider.GetTransactioner()
	flowExecService, err := flowexec.Initialize(
		mux, e.cfg.flowProvider, e.cfg.actorProvider, e.cfg.executorRegistry, e.cfg.interceptorRegistry,
		e.cfg.observability, e.cfg.runtimeCrypto, flowStore, flowTransactioner, e.flowCfg,
	)
	if err != nil {
		return fmt.Errorf("thunderidengine: failed to initialize flow execution: %w", err)
	}

	// Flow metadata.
	flowmeta.Initialize(mux, e.cfg.actorProvider, e.cfg.ouService, e.cfg.designResolveService, e.cfg.i18nService)

	// OAuth2/OIDC: build the Redis-backed runtime stores, then initialize.
	dep := e.oauthCfg.DeploymentID
	stores := oauth.RuntimeStores{
		JTI:                jti.NewRedisStore(e.redisProvider, dep),
		CIBA:               ciba.NewRedisStore(e.redisProvider, dep),
		PAR:                par.NewRedisStore(e.redisProvider, dep),
		AuthzCode:          oauth2authz.NewRedisAuthorizationCodeStore(e.redisProvider, dep),
		AuthzRequest:       oauth2authz.NewRedisAuthorizationRequestStore(e.redisProvider, dep),
		AuthzTransactioner: e.redisProvider.GetTransactioner(),
	}
	if err := oauth.Initialize(
		mux, e.cfg.actorProvider, e.cfg.authnProvider, e.cfg.jwtService, e.cfg.jweService,
		flowExecService, e.cfg.observability, e.cfg.runtimeCrypto, e.cfg.ouService,
		e.cfg.attributeCacheSvc, e.cfg.authZService, e.cfg.resourceService, e.cfg.i18nService,
		e.cfg.idpService, stores, e.oauthCfg,
	); err != nil {
		return fmt.Errorf("thunderidengine: failed to initialize OAuth services: %w", err)
	}

	return nil
}

// Handler builds a new http.ServeMux, registers the engine's routes on it, and returns it as
// an http.Handler — a convenience for embedders that want a ready-to-serve handler instead of
// mounting onto an existing mux via RegisterRoutes.
func (e *Engine) Handler() (http.Handler, error) {
	mux := http.NewServeMux()
	if err := e.RegisterRoutes(mux); err != nil {
		return nil, err
	}
	return mux, nil
}

// Shutdown releases the resources the engine owns. The Redis connection is owned by the caller
// (supplied via WithRedis) and is deliberately not closed here. Shutdown flushes the
// observability service when one was supplied.
func (e *Engine) Shutdown(_ context.Context) error {
	if e.cfg.observability != nil {
		e.cfg.observability.Shutdown()
	}
	return nil
}

// needsDeclarativeBase reports whether any of the engine-required declarative-backed services were
// not injected by the embedder and must therefore be built from declarative resources. Partial
// injection of this set is not supported: if any is missing, the full declarative graph is built.
func (c *engineConfig) needsDeclarativeBase() bool {
	return c.ouService == nil ||
		c.resourceService == nil ||
		c.idpService == nil ||
		c.authZService == nil ||
		c.attributeCacheSvc == nil ||
		c.designResolveService == nil ||
		c.flowProvider == nil
}

// validateRequiredProviders returns an error naming the first missing required dependency.
func (c *engineConfig) validateRequiredProviders() error {
	required := []struct {
		name    string
		present bool
	}{
		{"WithActorProvider", c.actorProvider != nil},
		{"WithAuthnProvider", c.authnProvider != nil},
		{"WithOUService", c.ouService != nil},
		{"WithAttributeCacheService", c.attributeCacheSvc != nil},
		{"WithAuthZService", c.authZService != nil},
		{"WithResourceService", c.resourceService != nil},
		{"WithI18nService", c.i18nService != nil},
		{"WithIDPService", c.idpService != nil},
		{"WithFlowProvider", c.flowProvider != nil},
		{"WithExecutorRegistry or WithExecutorDependencies", c.executorRegistry != nil},
		{"WithDesignResolveService", c.designResolveService != nil},
	}
	for _, r := range required {
		if !r.present {
			return fmt.Errorf("thunderidengine: %s is required", r.name)
		}
	}
	return nil
}

// willRegisterConsentExecutor reports whether ConsentExecutor will be registered for the given
// executor name list. An empty list registers all built-in executors.
func willRegisterConsentExecutor(executorNames []string) bool {
	if len(executorNames) == 0 {
		return true
	}
	return slices.Contains(executorNames, executor.ExecutorNameConsent)
}

// applyCryptoDefaults derives the runtime crypto, JWT, and JWE services from any PKI keys
// registered via WithPKIKey when they were not supplied explicitly, and verifies the crypto
// trio is complete. Registered keys are seeded into the server runtime configuration so the
// existing PKI loader picks them up.
func (c *engineConfig) applyCryptoDefaults() error {
	if c.runtimeCrypto == nil && len(c.pkiKeys) > 0 {
		runtime := config.GetServerRuntime()
		if runtime == nil {
			return fmt.Errorf("thunderidengine: server runtime configuration is not initialized")
		}
		for _, k := range c.pkiKeys {
			runtime.Config.Crypto.Keys = append(runtime.Config.Crypto.Keys,
				config.KeyConfig{ID: k.id, CertFile: k.certFile, KeyFile: k.keyFile})
		}
		pkiService, err := pki.Initialize()
		if err != nil {
			return fmt.Errorf("thunderidengine: failed to initialize PKI: %w", err)
		}
		runtimeCrypto, _, err := kmprovider.Initialize(pkiService)
		if err != nil {
			return fmt.Errorf("thunderidengine: failed to initialize runtime crypto: %w", err)
		}
		c.runtimeCrypto = runtimeCrypto
	}

	if c.runtimeCrypto == nil {
		return fmt.Errorf("thunderidengine: WithRuntimeCrypto or WithPKIKey is required")
	}

	if c.jwtService == nil || c.jweService == nil {
		jwtService, jweService, err := jose.Initialize(c.runtimeCrypto)
		if err != nil {
			return fmt.Errorf("thunderidengine: failed to initialize JOSE services: %w", err)
		}
		if c.jwtService == nil {
			c.jwtService = jwtService
		}
		if c.jweService == nil {
			c.jweService = jweService
		}
	}

	return nil
}
