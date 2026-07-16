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

package executor

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/thunder-id/thunderid/internal/attributecache"
	"github.com/thunder-id/thunderid/internal/authn/assert"
	"github.com/thunder-id/thunderid/internal/authn/github"
	"github.com/thunder-id/thunderid/internal/authn/google"
	"github.com/thunder-id/thunderid/internal/authn/magiclink"
	"github.com/thunder-id/thunderid/internal/authn/oauth"
	"github.com/thunder-id/thunderid/internal/authn/oidc"
	"github.com/thunder-id/thunderid/internal/authn/openid4vp"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/email"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/template"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// ExecutorRegistryInterface defines registry operations for executors.
type ExecutorRegistryInterface interface {
	GetExecutor(name string) (providers.Executor, error)
	RegisterExecutor(name string, ex providers.Executor)
	IsRegistered(name string) bool
	GetExecutorMeta(name string) (*providers.ExecutorMeta, error)
}

// executorRegistry is the default implementation of ExecutorRegistryInterface.
type executorRegistry struct {
	mu        sync.RWMutex
	executors map[string]providers.Executor
}

// newExecutorRegistry creates a new instance of executorRegistry.
func newExecutorRegistry() ExecutorRegistryInterface {
	return &executorRegistry{
		executors: make(map[string]providers.Executor),
	}
}

// RegisterExecutor registers an executor instance.
func (r *executorRegistry) RegisterExecutor(name string, exec providers.Executor) {
	// Executors are registered at server startup, outside any request,
	// so there is no request context (or trace ID) to propagate.
	ctx := context.Background()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ExecutorRegistry"))

	if exec == nil {
		logger.Warn(ctx, "Skipping registration of nil executor")
		return
	}
	if name == "" {
		logger.Warn(ctx, "Skipping registration of executor with empty name")
		return
	}

	logger.Debug(ctx, "Registering executor", log.String("executorName", name))

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.executors[name]; ok {
		logger.Warn(ctx, "Executor already registered", log.String("executorName", name))
		return
	}
	r.executors[name] = exec
}

// GetExecutor retrieves executor instance from the executor registry.
func (r *executorRegistry) GetExecutor(name string) (providers.Executor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ex, ok := r.executors[name]
	if !ok {
		return nil, fmt.Errorf("executor '%s' not found", name)
	}
	return ex, nil
}

// IsRegistered checks if an executor with the given name is registered.
func (r *executorRegistry) IsRegistered(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.executors[name]
	return ok
}

// GetExecutorMeta retrieves the ExecutorMeta for the named executor.
func (r *executorRegistry) GetExecutorMeta(name string) (*providers.ExecutorMeta, error) {
	exec, err := r.GetExecutor(name)
	if err != nil {
		return nil, err
	}
	return exec.GetMeta(), nil
}

// ExecutorDependencies holds service dependencies required to construct built-in executors.
type ExecutorDependencies struct {
	FlowFactory           core.FlowFactoryInterface
	OUService             ou.OrganizationUnitServiceInterface
	IDPService            idp.IDPServiceInterface
	NotifSenderSvc        notification.NotificationSenderServiceInterface
	JWTService            jwt.JWTServiceInterface
	AuthAssertGen         assert.AuthAssertGeneratorInterface
	ConsentEnforcer       providers.ConsentProvider
	AuthnProvider         providers.AuthnProviderManager
	OTPService            otp.OTPAuthnServiceInterface
	PasskeyService        passkey.PasskeyServiceInterface
	MagicLinkService      magiclink.MagicLinkAuthnServiceInterface
	AuthZService          providers.AuthorizationProvider
	EntityTypeService     entitytype.EntityTypeServiceInterface
	GroupService          group.GroupServiceInterface
	RoleService           role.RoleServiceInterface
	RoleAssignmentService role.RoleAssignmentServiceInterface
	EntityProvider        entityprovider.EntityProviderInterface
	AttributeCacheSvc     attributecache.AttributeCacheServiceInterface
	EmailClient           email.EmailClientInterface
	TemplateService       template.TemplateServiceInterface
	OAuthSvc              oauth.OAuthAuthnServiceInterface
	OIDCSvc               oidc.OIDCAuthnServiceInterface
	GithubSvc             github.GithubOAuthAuthnServiceInterface
	GoogleSvc             google.GoogleOIDCAuthnServiceInterface
	OpenID4VPVerifierSvc  openid4vp.OpenID4VPServiceInterface
	SessionService        session.Service
}

type builtInExecutorRegistrar func(ExecutorRegistryInterface, ExecutorDependencies)

// newBuiltInExecutorRegistrars creates a new map of built-in executor registrars.
func newBuiltInExecutorRegistrars() map[string]builtInExecutorRegistrar {
	return map[string]builtInExecutorRegistrar{
		ExecutorNameCredentialsAuth: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameCredentialsAuth, newCredentialsAuthExecutor(
				deps.FlowFactory, deps.EntityProvider, deps.AuthnProvider))
		},
		ExecutorNamePasskeyAuth: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNamePasskeyAuth, newPasskeyAuthExecutor(
				deps.FlowFactory, deps.PasskeyService, deps.AuthnProvider, deps.EntityProvider))
		},
		ExecutorNameMagicLink: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameMagicLink, newMagicLinkExecutor(
				deps.FlowFactory, deps.MagicLinkService, deps.AuthnProvider, deps.EntityProvider))
		},
		ExecutorNameOAuth: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameOAuth, newOAuthExecutor(
				"", []providers.Input{}, []providers.Input{}, deps.FlowFactory, deps.IDPService,
				deps.OAuthSvc, deps.AuthnProvider, providers.IDPTypeOAuth))
		},
		ExecutorNameOIDCAuth: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameOIDCAuth, newOIDCAuthExecutor(
				"", []providers.Input{}, []providers.Input{}, deps.FlowFactory, deps.IDPService,
				deps.OIDCSvc, deps.AuthnProvider, providers.IDPTypeOIDC))
		},
		ExecutorNameGitHubAuth: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameGitHubAuth, newGithubOAuthExecutor(
				deps.FlowFactory, deps.IDPService, deps.GithubSvc, deps.AuthnProvider))
		},
		ExecutorNameGoogleAuth: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameGoogleAuth, newGoogleOIDCAuthExecutor(
				deps.FlowFactory, deps.IDPService, deps.GoogleSvc, deps.AuthnProvider))
		},
		ExecutorNameProvisioning: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameProvisioning, newProvisioningExecutor(
				deps.FlowFactory, deps.GroupService, deps.RoleService, deps.RoleAssignmentService,
				deps.EntityProvider, deps.EntityTypeService, deps.AuthnProvider))
		},
		ExecutorNameOUCreation: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameOUCreation, newOUExecutor(deps.FlowFactory, deps.OUService,
				deps.AuthnProvider, deps.EntityTypeService))
		},
		ExecutorNameAttributeCollect: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameAttributeCollect, newAttributeCollector(
				deps.FlowFactory, deps.EntityProvider, deps.AuthnProvider))
		},
		ExecutorNameAuthAssert: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameAuthAssert, newAuthAssertExecutor(deps.FlowFactory, deps.JWTService,
				deps.OUService, deps.AuthAssertGen, deps.AuthnProvider, deps.EntityProvider,
				deps.AttributeCacheSvc, deps.RoleService))
		},
		ExecutorNameAuthorization: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameAuthorization, newAuthorizationExecutor(
				deps.FlowFactory, deps.AuthZService, deps.EntityProvider, deps.AuthnProvider))
		},
		ExecutorNameHTTPRequest: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameHTTPRequest, newHTTPRequestExecutor(deps.FlowFactory, deps.OUService,
				deps.AuthnProvider))
		},
		ExecutorNameUserTypeResolver: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameUserTypeResolver, newUserTypeResolver(
				deps.FlowFactory, deps.EntityTypeService, deps.OUService))
		},
		ExecutorNameInviteExecutor: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameInviteExecutor, newInviteExecutor(deps.FlowFactory))
		},
		ExecutorNameEmailExecutor: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameEmailExecutor, newEmailExecutor(
				deps.FlowFactory, deps.EmailClient, deps.TemplateService, deps.EntityProvider))
		},
		ExecutorNameCredentialSetter: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameCredentialSetter, newCredentialSetter(
				deps.FlowFactory, deps.EntityProvider, deps.AuthnProvider))
		},
		ExecutorNamePermissionValidator: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNamePermissionValidator, newPermissionValidator(deps.FlowFactory))
		},
		ExecutorNameIdentifying: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			identifyingInputs := []providers.Input{
				{Identifier: userAttributeUsername, Type: "string", Required: true},
			}
			reg.RegisterExecutor(ExecutorNameIdentifying, newIdentifyingExecutor(
				"", identifyingInputs, []providers.Input{}, deps.FlowFactory, deps.EntityProvider))
		},
		ExecutorNameConsent: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameConsent, newConsentExecutor(
				deps.FlowFactory, deps.ConsentEnforcer, deps.AuthnProvider))
		},
		ExecutorNameOUResolver: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameOUResolver, newOUResolverExecutor(deps.FlowFactory, deps.OUService))
		},
		ExecutorNameAttributeUniquenessValidator: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameAttributeUniquenessValidator, newAttributeUniquenessValidator(
				deps.FlowFactory, deps.EntityTypeService, deps.EntityProvider, deps.AuthnProvider))
		},
		ExecutorNameSMSExecutor: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameSMSExecutor, newSMSExecutor(
				deps.FlowFactory, deps.NotifSenderSvc, deps.TemplateService, deps.EntityProvider))
		},
		ExecutorNameFederatedAuthResolver: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameFederatedAuthResolver, newFederatedAuthResolverExecutor(deps.FlowFactory,
				deps.AuthnProvider))
		},
		ExecutorNameOpenID4VPVerify: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameOpenID4VPVerify, newOpenID4VPVerifier(
				deps.FlowFactory, deps.OpenID4VPVerifierSvc, deps.AuthnProvider))
		},
		ExecutorNameSSOCheck: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameSSOCheck, newSSOCheckExecutor(deps.FlowFactory, deps.SessionService))
		},
		ExecutorNameSession: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameSession, newSessionExecutor(
				deps.FlowFactory, deps.SessionService, deps.AuthnProvider))
		},
		ExecutorNameOTPExecutor: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameOTPExecutor, newOTPExecutor(
				deps.FlowFactory, deps.OTPService, deps.AuthnProvider, deps.EntityProvider))
		},
	}
}

// defaultBuiltInExecutorNames returns the names of all built-in executors.
func defaultBuiltInExecutorNames() []string {
	return sortedMapKeys(newBuiltInExecutorRegistrars())
}

// sortedMapKeys returns the keys of a map sorted alphabetically.
func sortedMapKeys(m map[string]builtInExecutorRegistrar) []string {
	names := slices.Collect(maps.Keys(m))
	slices.Sort(names)
	return names
}

// registerBuiltInExecutors registers the requested built-in executors on reg.
// When names is empty, all built-in executors are registered.
func registerBuiltInExecutors(reg ExecutorRegistryInterface, deps ExecutorDependencies, names []string) error {
	catalog := newBuiltInExecutorRegistrars()
	resolved, err := resolveBuiltInExecutorNames(catalog, names)
	if err != nil {
		return err
	}
	for _, name := range resolved {
		if err := registerBuiltInExecutor(reg, catalog, deps, name); err != nil {
			return err
		}
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ExecutorRegistry"))
	logger.Debug(context.Background(), "Registered built-in flow executors",
		log.Int("count", len(resolved)),
		log.Any("executors", resolved))
	return nil
}

// resolveBuiltInExecutorNames resolves the names of the built-in executors.
func resolveBuiltInExecutorNames(catalog map[string]builtInExecutorRegistrar, names []string) ([]string, error) {
	if len(names) == 0 {
		return sortedMapKeys(catalog), nil
	}
	names = dedupeExecutorNames(names)
	for _, name := range names {
		if _, ok := catalog[name]; !ok {
			return nil, fmt.Errorf("unknown built-in executor: %q", name)
		}
	}
	return names, nil
}

// dedupeExecutorNames deduplicates the names of the built-in executors.
func dedupeExecutorNames(names []string) []string {
	seen := make(map[string]struct{}, len(names))
	out := make([]string, 0, len(names))
	for _, name := range names {
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

// registerBuiltInExecutor registers a built-in executor on reg.
func registerBuiltInExecutor(
	reg ExecutorRegistryInterface,
	catalog map[string]builtInExecutorRegistrar,
	deps ExecutorDependencies,
	name string,
) error {
	register, ok := catalog[name]
	if !ok {
		return fmt.Errorf("unhandled built-in executor: %q", name)
	}
	register(reg, deps)
	if !reg.IsRegistered(name) {
		return fmt.Errorf("failed to register built-in executor: %q", name)
	}
	return nil
}
