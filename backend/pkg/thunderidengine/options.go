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

package thunderidengine

import (
	"github.com/redis/go-redis/v9"

	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/host"
)

// engineConfig collects the embedder-supplied dependencies. The engine is
// runtime-Redis-only: it persists short-lived runtime state in the provided Redis
// connection and never opens a SQL database.
type engineConfig struct {
	redisClient    *redis.Client
	redisKeyPrefix string

	serverHome   string
	serverConfig *Config

	pkiKeys []pkiKey

	actorProvider        ActorProvider
	authnProvider        AuthnProvider
	hostActorProvider    host.ActorProvider
	hostAuthnProvider    host.AuthnProvider
	roleService          RoleService
	hostRoleProvider     host.RoleProvider
	ouService            OUService
	attributeCacheSvc    AttributeCacheService
	authZService         AuthZService
	resourceService      ResourceService
	i18nService          I18nService
	idpService           IDPService
	flowProvider         FlowProvider
	executorRegistry     ExecutorRegistry
	interceptorRegistry  interceptor.InterceptorRegistryInterface
	executorDeps         *ExecutorDependencies
	enabledExecutors     []string
	customExecutors      map[string]ExecutorInterface
	designResolveService DesignResolveService
	jwtService           JWTService
	jweService           JWEService
	observability        ObservabilityService
	runtimeCrypto        RuntimeCryptoProvider
}

// Option configures the engine.
type Option func(*engineConfig)

// WithRedis supplies the Redis connection (and key prefix) used for all runtime
// state. Required. The caller owns the client's lifecycle.
func WithRedis(client *redis.Client, keyPrefix string) Option {
	return func(c *engineConfig) { c.redisClient = client; c.redisKeyPrefix = keyPrefix }
}

// WithConfig seeds the process-wide server runtime configuration (deployment ID, cache, flow,
// crypto, translation, and related settings) from which the engine reads non-injected values.
// serverHome is the base directory used to resolve relative resource paths (PKI key files,
// declarative resource directories). Because the runtime configuration is a process singleton,
// the first initialization wins: WithConfig is a no-op if the runtime was already initialized
// (for example by an embedder that loaded configuration itself), and only one engine instance
// per process is supported. Supply this before relying on declarative defaults or WithPKIKey.
func WithConfig(serverHome string, cfg *Config) Option {
	return func(c *engineConfig) { c.serverHome = serverHome; c.serverConfig = cfg }
}

// WithActorProvider sets the actor provider directly (in-tree callers that already hold the
// internal interface). External embedders should use WithHostActorProvider instead. Either this
// or WithHostActorProvider satisfies the required actor provider.
func WithActorProvider(p ActorProvider) Option { return func(c *engineConfig) { c.actorProvider = p } }

// WithAuthnProvider sets the authentication provider directly (in-tree callers). External
// embedders should use WithHostAuthnProvider instead. Either this or WithHostAuthnProvider
// satisfies the required authn provider.
func WithAuthnProvider(p AuthnProvider) Option { return func(c *engineConfig) { c.authnProvider = p } }

// WithHostActorProvider sets the embedder's identity source via the public, dependency-free
// host.ActorProvider SDK contract. The engine wraps it in an adapter implementing the internal
// actor provider interface. Either this or WithActorProvider is required.
func WithHostActorProvider(p host.ActorProvider) Option {
	return func(c *engineConfig) { c.hostActorProvider = p }
}

// WithHostAuthnProvider sets the embedder's authentication source via the public, dependency-free
// host.AuthnProvider SDK contract. The engine wraps it in an adapter implementing the internal
// authn provider manager interface. Either this or WithAuthnProvider is required.
func WithHostAuthnProvider(p host.AuthnProvider) Option {
	return func(c *engineConfig) { c.hostAuthnProvider = p }
}

// WithOUService sets the organization-unit service. Required.
func WithOUService(s OUService) Option { return func(c *engineConfig) { c.ouService = s } }

// WithAttributeCacheService sets the attribute cache service. Required.
func WithAttributeCacheService(s AttributeCacheService) Option {
	return func(c *engineConfig) { c.attributeCacheSvc = s }
}

// WithAuthZService sets the authorization service. Required unless a RoleService or
// host.RoleProvider is supplied from which the engine can derive one.
func WithAuthZService(s AuthZService) Option { return func(c *engineConfig) { c.authZService = s } }

// WithRoleService sets the role service directly (in-tree callers). External embedders should
// use WithHostRoleProvider instead.
func WithRoleService(s RoleService) Option { return func(c *engineConfig) { c.roleService = s } }

// WithHostRoleProvider sets the embedder's role source via the public, dependency-free
// host.RoleProvider SDK contract. The engine wraps it and, when WithAuthZService is not set,
// derives the authorization service from it.
func WithHostRoleProvider(p host.RoleProvider) Option {
	return func(c *engineConfig) { c.hostRoleProvider = p }
}

// WithResourceService sets the resource service. Required.
func WithResourceService(s ResourceService) Option {
	return func(c *engineConfig) { c.resourceService = s }
}

// WithI18nService sets the translation service. Required.
func WithI18nService(s I18nService) Option { return func(c *engineConfig) { c.i18nService = s } }

// WithIDPService sets the identity-provider service. Required.
func WithIDPService(s IDPService) Option { return func(c *engineConfig) { c.idpService = s } }

// WithFlowProvider sets the flow definition provider. Required.
func WithFlowProvider(p FlowProvider) Option { return func(c *engineConfig) { c.flowProvider = p } }

// WithExecutorRegistry sets a pre-built flow executor registry. Either this or
// WithExecutorDependencies is required. When both are supplied, the pre-built registry wins.
func WithExecutorRegistry(r ExecutorRegistry) Option {
	return func(c *engineConfig) { c.executorRegistry = r }
}

// WithExecutorDependencies supplies the dependencies from which the engine builds the flow
// executor registry itself (an alternative to WithExecutorRegistry). The engine sets
// FlowFactory and fills any nil actor/authn/OU/JWT/attribute-cache/authz/IDP dependency from
// the providers already registered, and builds a default auth-assertion generator. Supply only
// the executor-specific extras (for example a ConsentEnforcer) plus the fields required by the
// executors named in WithEnabledExecutors.
func WithExecutorDependencies(deps ExecutorDependencies) Option {
	return func(c *engineConfig) { c.executorDeps = &deps }
}

// WithEnabledExecutors restricts the built-in executors registered when the engine builds the
// registry from WithExecutorDependencies. When empty, the server configuration's executor list
// applies (and an empty list there registers all built-in executors). Ignored when a registry
// is supplied via WithExecutorRegistry.
func WithEnabledExecutors(names ...string) Option {
	return func(c *engineConfig) { c.enabledExecutors = names }
}

// WithCustomExecutors registers embedder-supplied executors on the registry the engine ends up
// with, so custom executors run alongside the built-ins selected by WithEnabledExecutors (or the
// fully custom registry from WithExecutorRegistry). Each map key is the executor name referenced
// by flow TASK nodes; a custom executor whose name matches a built-in overrides that built-in.
// Implement ExecutorInterface (optionally embedding NewBaseExecutor) to author one. May be called
// multiple times; later entries override earlier ones on key collision.
func WithCustomExecutors(executors map[string]ExecutorInterface) Option {
	return func(c *engineConfig) {
		if c.customExecutors == nil {
			c.customExecutors = make(map[string]ExecutorInterface, len(executors))
		}
		for name, ex := range executors {
			c.customExecutors[name] = ex
		}
	}
}

// WithDesignResolveService sets the design (theme/layout) resolve service. Required.
func WithDesignResolveService(s DesignResolveService) Option {
	return func(c *engineConfig) { c.designResolveService = s }
}

// WithJWTService sets the JWT service. Required.
func WithJWTService(s JWTService) Option { return func(c *engineConfig) { c.jwtService = s } }

// WithJWEService sets the JWE service. Required.
func WithJWEService(s JWEService) Option { return func(c *engineConfig) { c.jweService = s } }

// WithObservability sets the observability service. Optional (may be nil).
func WithObservability(s ObservabilityService) Option {
	return func(c *engineConfig) { c.observability = s }
}

// WithRuntimeCrypto sets the runtime crypto provider. Required unless derived via WithPKIKey.
func WithRuntimeCrypto(p RuntimeCryptoProvider) Option {
	return func(c *engineConfig) { c.runtimeCrypto = p }
}

// pkiKey holds a PEM certificate/key file pair used to derive the default crypto provider.
type pkiKey struct {
	id       string
	certFile string
	keyFile  string
}

// WithPKIKey registers a PEM certificate/key file pair (identified by id) from which the
// engine derives the default runtime crypto, JWT, and JWE services when those are not
// supplied explicitly. The cert and key file paths are resolved relative to the configured
// server home (or used as-is when the server home is unset). May be called multiple times to
// register additional keys.
func WithPKIKey(id, certFile, keyFile string) Option {
	return func(c *engineConfig) {
		c.pkiKeys = append(c.pkiKeys, pkiKey{id: id, certFile: certFile, keyFile: keyFile})
	}
}
