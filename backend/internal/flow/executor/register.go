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
	"github.com/thunder-id/thunderid/internal/authn/consent"
	"github.com/thunder-id/thunderid/internal/authn/github"
	"github.com/thunder-id/thunderid/internal/authn/google"
	"github.com/thunder-id/thunderid/internal/authn/magiclink"
	"github.com/thunder-id/thunderid/internal/authn/oauth"
	"github.com/thunder-id/thunderid/internal/authn/oidc"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/authz"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	"github.com/thunder-id/thunderid/internal/openid4vp"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/email"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/template"
)

// executorRegistry is the default implementation of core.ExecutorRegistryInterface.
type executorRegistry struct {
	mu        sync.RWMutex
	executors map[string]core.ExecutorInterface
}

// newExecutorRegistry creates a new instance of executorRegistry.
func newExecutorRegistry() core.ExecutorRegistryInterface {
	return &executorRegistry{
		executors: make(map[string]core.ExecutorInterface),
	}
}

// RegisterExecutor registers an executor instance.
func (r *executorRegistry) RegisterExecutor(name string, exec core.ExecutorInterface) {
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
func (r *executorRegistry) GetExecutor(name string) (core.ExecutorInterface, error) {
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

// ExecutorDependencies holds service dependencies required to construct built-in executors.
type ExecutorDependencies struct {
	OUService             ou.OrganizationUnitServiceInterface
	IDPService            idp.IDPServiceInterface
	NotifSenderSvc        notification.NotificationSenderServiceInterface
	JWTService            jwt.JWTServiceInterface
	AuthAssertGen         assert.AuthAssertGeneratorInterface
	ConsentEnforcer       consent.ConsentEnforcerServiceInterface
	AuthnProvider         authnprovidermgr.AuthnProviderManagerInterface
	OTPService            otp.OTPAuthnServiceInterface
	PasskeyService        passkey.PasskeyServiceInterface
	MagicLinkService      magiclink.MagicLinkAuthnServiceInterface
	AuthZService          authz.AuthorizationServiceInterface
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
}

type builtInExecutorRegistrar func(core.ExecutorRegistryInterface, ExecutorDependencies)

// newBuiltInExecutorRegistrars creates a new map of built-in executor registrars.
func newBuiltInExecutorRegistrars() map[string]builtInExecutorRegistrar {
	return map[string]builtInExecutorRegistrar{
		ExecutorNameBasicAuth: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameBasicAuth, newBasicAuthExecutor(
				deps.EntityProvider, deps.AuthnProvider))
		},
		ExecutorNameSMSAuth: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameSMSAuth, newSMSOTPAuthExecutor(
				deps.OTPService, deps.AuthnProvider, deps.EntityProvider))
		},
		ExecutorNamePasskeyAuth: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNamePasskeyAuth, newPasskeyAuthExecutor(
				deps.PasskeyService, deps.AuthnProvider, deps.EntityProvider))
		},
		ExecutorNameMagicLink: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameMagicLink, newMagicLinkExecutor(
				deps.MagicLinkService, deps.AuthnProvider, deps.EntityProvider))
		},
		ExecutorNameOAuth: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameOAuth, newOAuthExecutor(
				"", []common.Input{}, []common.Input{}, deps.IDPService, deps.EntityTypeService,
				deps.OAuthSvc, deps.AuthnProvider, idp.IDPTypeOAuth))
		},
		ExecutorNameOIDCAuth: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameOIDCAuth, newOIDCAuthExecutor(
				"", []common.Input{}, []common.Input{}, deps.IDPService, deps.EntityTypeService,
				deps.OIDCSvc, deps.AuthnProvider, idp.IDPTypeOIDC))
		},
		ExecutorNameGitHubAuth: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameGitHubAuth, newGithubOAuthExecutor(
				deps.IDPService, deps.EntityTypeService, deps.GithubSvc, deps.AuthnProvider))
		},
		ExecutorNameGoogleAuth: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameGoogleAuth, newGoogleOIDCAuthExecutor(
				deps.IDPService, deps.EntityTypeService, deps.GoogleSvc, deps.AuthnProvider))
		},
		ExecutorNameProvisioning: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameProvisioning, newProvisioningExecutor(
				deps.GroupService, deps.RoleService, deps.RoleAssignmentService,
				deps.EntityProvider, deps.EntityTypeService))
		},
		ExecutorNameOUCreation: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameOUCreation, newOUExecutor(deps.OUService))
		},
		ExecutorNameAttributeCollect: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameAttributeCollect, newAttributeCollector(
				deps.EntityProvider))
		},
		ExecutorNameAuthAssert: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameAuthAssert, newAuthAssertExecutor(deps.JWTService,
				deps.OUService, deps.AuthAssertGen, deps.AuthnProvider, deps.EntityProvider,
				deps.AttributeCacheSvc, deps.RoleService))
		},
		ExecutorNameAuthorization: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameAuthorization, newAuthorizationExecutor(
				deps.AuthZService, deps.EntityProvider))
		},
		ExecutorNameHTTPRequest: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameHTTPRequest, newHTTPRequestExecutor(deps.OUService))
		},
		ExecutorNameUserTypeResolver: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameUserTypeResolver, newUserTypeResolver(
				deps.EntityTypeService, deps.OUService))
		},
		ExecutorNameInviteExecutor: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameInviteExecutor, newInviteExecutor())
		},
		ExecutorNameEmailExecutor: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameEmailExecutor, newEmailExecutor(
				deps.EmailClient, deps.TemplateService, deps.EntityProvider))
		},
		ExecutorNameCredentialSetter: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameCredentialSetter, newCredentialSetter(
				deps.EntityProvider))
		},
		ExecutorNamePermissionValidator: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNamePermissionValidator, newPermissionValidator())
		},
		ExecutorNameIdentifying: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			identifyingInputs := []common.Input{
				{Identifier: userAttributeUsername, Type: "string", Required: true},
			}
			reg.RegisterExecutor(ExecutorNameIdentifying, newIdentifyingExecutor(
				"", identifyingInputs, []common.Input{}, deps.EntityProvider))
		},
		ExecutorNameConsent: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameConsent, newConsentExecutor(
				deps.ConsentEnforcer, deps.AuthnProvider))
		},
		ExecutorNameOUResolver: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameOUResolver, newOUResolverExecutor(deps.OUService))
		},
		ExecutorNameAttributeUniquenessValidator: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameAttributeUniquenessValidator, newAttributeUniquenessValidator(
				deps.EntityTypeService, deps.EntityProvider))
		},
		ExecutorNameSMSExecutor: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameSMSExecutor, newSMSExecutor(
				deps.NotifSenderSvc, deps.TemplateService, deps.EntityProvider))
		},
		ExecutorNameFederatedAuthResolver: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameFederatedAuthResolver, newFederatedAuthResolverExecutor())
		},
		ExecutorNameOpenID4VPVerify: func(reg core.ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameOpenID4VPVerify, newOpenID4VPVerifier(
				deps.OpenID4VPVerifierSvc, deps.EntityTypeService, deps.EntityProvider))
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
func registerBuiltInExecutors(reg core.ExecutorRegistryInterface, deps ExecutorDependencies, names []string) error {
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
	reg core.ExecutorRegistryInterface,
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
