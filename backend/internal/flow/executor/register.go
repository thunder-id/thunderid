// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"

	appmodel "github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/attributecache"
	"github.com/thunder-id/thunderid/internal/authn/assert"
	"github.com/thunder-id/thunderid/internal/authn/github"
	"github.com/thunder-id/thunderid/internal/authn/google"
	"github.com/thunder-id/thunderid/internal/authn/magiclink"
	"github.com/thunder-id/thunderid/internal/authn/oauth"
	"github.com/thunder-id/thunderid/internal/authn/oidc"
	"github.com/thunder-id/thunderid/internal/authn/openid4vp"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/email"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/template"
	"github.com/thunder-id/thunderid/internal/user"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
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

// appProviderConsumerInterface is implemented by the executors that act on applications. They are
// built before the application service exists, so it reaches them in a second phase.
type appProviderConsumerInterface interface {
	setApplicationProvider(provider applicationAdminProvider)
}

// SetApplicationProvider injects the application service into every registered executor that acts on
// applications. The service is constructed after the executors and sits behind them in the import graph
// (executor -> application -> inboundclient -> flowmgt -> executor), so it cannot be passed to their
// constructors. Called once during startup, before the server serves.
//
// It is a package function rather than a method on ExecutorRegistryInterface so the provider contract
// stays internal to this package. A registry that cannot accept the injection is reported rather than
// skipped, so a wiring mistake cannot leave the executors without a provider.
func SetApplicationProvider(reg ExecutorRegistryInterface, provider applicationAdminProvider) error {
	registry, ok := reg.(*executorRegistry)
	if !ok {
		return fmt.Errorf("executor registry does not support application provider injection")
	}
	registry.setApplicationProvider(provider)
	return nil
}

func (r *executorRegistry) setApplicationProvider(provider applicationAdminProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ex := range r.executors {
		if consumer, ok := ex.(appProviderConsumerInterface); ok {
			consumer.setApplicationProvider(provider)
		}
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
	MagicLinkService      magiclink.MagicLinkAuthnServiceInterface
	AuthZService          providers.AuthorizationProvider
	EntityTypeService     entitytype.EntityTypeServiceInterface
	GroupService          group.GroupServiceInterface
	RoleService           role.RoleServiceInterface
	RoleAssignmentService role.RoleAssignmentServiceInterface
	EntityProvider        entityprovider.EntityProviderInterface
	UserMgtProvider       providers.UserMgtProvider
	AttributeCacheSvc     attributecache.AttributeCacheServiceInterface
	EmailClient           email.EmailClientInterface
	TemplateService       template.TemplateServiceInterface
	OAuthSvc              oauth.OAuthAuthnServiceInterface
	OIDCSvc               oidc.OIDCAuthnServiceInterface
	GithubSvc             github.GithubOAuthAuthnServiceInterface
	GoogleSvc             google.GoogleOIDCAuthnServiceInterface
	OpenID4VPVerifierSvc  openid4vp.OpenID4VPServiceInterface
	SessionService        session.Service
	ResourceService       providers.ResourceServerProvider
	UserService           user.UserServiceInterface
	CriteriaRevoker       revocation.CriteriaRevoker
	// RoleGroupAdminProvider and ResourceAdminProvider are the administration seams the role, group and
	// scope flows act through.
	RoleGroupAdminProvider role.AdminProviderInterface
	ResourceAdminProvider  resource.AdminProviderInterface
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
				deps.FlowFactory, deps.AuthnProvider))
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
				deps.EntityProvider, deps.UserMgtProvider, deps.EntityTypeService, deps.AuthnProvider))
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
				deps.FlowFactory, deps.AuthZService, deps.EntityProvider, deps.AuthnProvider,
				deps.ResourceService))
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
		ExecutorNameSessionSignOut: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameSessionSignOut, newSessionSignOutExecutor(
				deps.FlowFactory, deps.SessionService))
		},
		ExecutorNameOTPExecutor: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameOTPExecutor, newOTPExecutor(
				deps.FlowFactory, deps.OTPService, deps.AuthnProvider, deps.EntityProvider))
		},
		ExecutorNamePreDelete: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNamePreDelete,
				newPreDeleteExecutor(deps.FlowFactory, deps.UserService))
		},
		ExecutorNameCriteriaRevocation: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameCriteriaRevocation,
				newCriteriaRevocationExecutor(deps.FlowFactory, deps.CriteriaRevoker))
		},
		ExecutorNameCriteriaRevocationRestamp: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameCriteriaRevocationRestamp,
				newCriteriaRevocationRestampExecutor(deps.FlowFactory, deps.CriteriaRevoker))
		},
		ExecutorNamePreRoleAssignmentRemoval: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNamePreRoleAssignmentRemoval,
				newPreRoleAssignmentRemovalExecutor(deps.FlowFactory, configuredMaxRevocationCriteria(),
					deps.RoleGroupAdminProvider))
		},
		ExecutorNamePreRoleDeletion: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNamePreRoleDeletion,
				newPreRoleDeletionExecutor(deps.FlowFactory, configuredMaxRevocationCriteria(),
					deps.RoleGroupAdminProvider))
		},
		ExecutorNameRoleDeletion: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameRoleDeletion,
				newRoleDeletionExecutor(deps.FlowFactory, deps.RoleGroupAdminProvider))
		},
		ExecutorNamePreRolePermissionRemoval: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNamePreRolePermissionRemoval,
				newPreRolePermissionRemovalExecutor(deps.FlowFactory, configuredMaxRevocationCriteria(),
					deps.RoleGroupAdminProvider))
		},
		ExecutorNameRolePermissionRemoval: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameRolePermissionRemoval,
				newRolePermissionRemovalExecutor(deps.FlowFactory, deps.RoleGroupAdminProvider))
		},
		ExecutorNamePreGroupDeletion: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNamePreGroupDeletion,
				newPreGroupDeletionExecutor(deps.FlowFactory, configuredMaxRevocationCriteria(),
					deps.RoleGroupAdminProvider))
		},
		ExecutorNameGroupDeletion: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameGroupDeletion,
				newGroupDeletionExecutor(deps.FlowFactory, deps.RoleGroupAdminProvider))
		},
		ExecutorNamePreGroupMembershipRemoval: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNamePreGroupMembershipRemoval,
				newPreGroupMembershipRemovalExecutor(deps.FlowFactory, configuredMaxRevocationCriteria(),
					deps.RoleGroupAdminProvider))
		},
		ExecutorNameGroupMembershipRemoval: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameGroupMembershipRemoval,
				newGroupMembershipRemovalExecutor(deps.FlowFactory, deps.RoleGroupAdminProvider))
		},
		ExecutorNamePreScopeDeletion: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNamePreScopeDeletion,
				newPreScopeDeletionExecutor(deps.FlowFactory, configuredMaxRevocationCriteria(),
					deps.ResourceAdminProvider))
		},
		ExecutorNameScopeDeletion: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameScopeDeletion,
				newScopeDeletionExecutor(deps.FlowFactory, deps.ResourceAdminProvider))
		},
		ExecutorNameRoleAssignmentRemoval: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameRoleAssignmentRemoval,
				newRoleAssignmentRemovalExecutor(deps.FlowFactory, deps.RoleGroupAdminProvider))
		},
		ExecutorNameSessionRevocation: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameSessionRevocation,
				newSessionRevocationExecutor(deps.FlowFactory, deps.SessionService))
		},
		ExecutorNameUserDelete: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameUserDelete,
				newUserDeleteExecutor(deps.FlowFactory, deps.UserService))
		},
		ExecutorNameApplicationActionValidator: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameApplicationActionValidator,
				newApplicationActionValidator(deps.FlowFactory))
		},
		ExecutorNameApplicationDelete: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameApplicationDelete,
				newApplicationDeleteExecutor(deps.FlowFactory))
		},
		ExecutorNameClientSecret: func(reg ExecutorRegistryInterface, deps ExecutorDependencies) {
			reg.RegisterExecutor(ExecutorNameClientSecret,
				newClientSecretExecutor(deps.FlowFactory))
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

// applicationAdminProvider is the application seam the administration executors consume. It is declared
// here rather than in pkg so it stays internal: the application service satisfies it structurally.
type applicationAdminProvider interface {
	ValidateDeleteApplication(ctx context.Context, appID string) (
		*appmodel.ApplicationArtifactProfile, *tidcommon.ServiceError)
	DeleteApplication(ctx context.Context, appID string) *tidcommon.ServiceError
	ValidateCredentialAction(ctx context.Context, appID string, action appmodel.CredentialAction) (
		*appmodel.ApplicationArtifactProfile, *tidcommon.ServiceError)
	ApplyCredentialAction(ctx context.Context, appID string, action appmodel.CredentialAction) (
		string, *tidcommon.ServiceError)
}

// roleAdminProvider is the role seam the role administration executors consume. Like the application
// seam it is declared here rather than in pkg so it stays internal: the role service's administration
// provider satisfies it structurally.
type roleAdminProvider interface {
	ValidateRemoveRoleAssignment(ctx context.Context, roleID, assigneeID string) (
		*revocation.ScopeRevocationTarget, *tidcommon.ServiceError)
	RemoveRoleAssignment(ctx context.Context, roleID, assigneeID string) *tidcommon.ServiceError
	ValidateRoleScopeChange(ctx context.Context, roleID string) (
		*revocation.ScopeRevocationTarget, *tidcommon.ServiceError)
	DeleteRole(ctx context.Context, roleID string) *tidcommon.ServiceError
	UpdateRolePermissions(ctx context.Context, roleID string,
		permissions []revocation.RolePermissions) *tidcommon.ServiceError
}

// groupAdminProvider is the group seam the group administration executors consume. One object satisfies
// it and roleAdminProvider both, because a group's scopes are the scopes of the roles it holds; the two
// interfaces keep each executor depending only on the half it uses.
type groupAdminProvider interface {
	ValidateGroupMembershipChange(ctx context.Context, groupID, memberID string) (
		*revocation.ScopeRevocationTarget, *tidcommon.ServiceError)
	DeleteGroup(ctx context.Context, groupID string) *tidcommon.ServiceError
	RemoveGroupMember(ctx context.Context, groupID, memberID string) *tidcommon.ServiceError
}

// resourceAdminProvider is the resource-catalog seam the scope deletion executors consume. It is its
// own seam rather than part of groupAdminProvider because deleting a scope is a change to the catalog,
// not to who holds what: it needs no role or group lookup, and it revokes deployment-wide.
type resourceAdminProvider interface {
	ValidateDeleteAction(ctx context.Context, resourceServerID, resourceID, actionID string) (
		*revocation.ScopeRevocationTarget, *tidcommon.ServiceError)
	DeleteAction(ctx context.Context, resourceServerID, resourceID, actionID string) *tidcommon.ServiceError
}

// configuredMaxRevocationCriteria returns the configured fan-out ceiling for one administrative
// revocation. A non-positive value, including when runtime config is not loaded, leaves the
// preparatory executors on their built-in ceiling.
func configuredMaxRevocationCriteria() int {
	if !config.IsServerRuntimeInitialized() {
		return 0
	}
	return config.GetServerRuntime().Config.OAuth.Revocation.Criteria.MaxCriteria
}
