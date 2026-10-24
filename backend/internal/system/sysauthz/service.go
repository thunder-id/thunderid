// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package sysauthz provides system-level authorization services for protecting
// system operations management.
package sysauthz

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// SystemAuthorizationServiceInterface defines the contract for system-level authorization.
type SystemAuthorizationServiceInterface interface {
	// IsActionAllowed checks whether the authenticated caller is permitted to perform
	// the given action. Returns true if allowed, false if denied. A non-nil ServiceError
	// indicates a processing failure, not an authorization denial.
	IsActionAllowed(ctx context.Context, action security.Action,
		actionCtx *ActionContext) (bool, *tidcommon.ServiceError)

	// GetAccessibleResources returns the set of resources the caller may access for the
	// given action and resource type. The result must be applied as a store-level filter
	// before pagination so that page sizes and total counts remain correct.
	//
	// When AllAllowed is true, no ID filter should be applied.
	// When AllAllowed is false, the store should restrict results to the returned IDs.
	GetAccessibleResources(ctx context.Context, action security.Action,
		resourceType security.ResourceType) (*AccessibleResources, *tidcommon.ServiceError)

	// SetOUHierarchyResolver injects the OU hierarchy resolver used by inheritance-based
	// policies. This must be called once at application startup after the ou package has
	// been initialized, completing the two-phase initialization that avoids an import cycle
	// between sysauthz (which ou already imports) and the ou package itself.
	SetOUHierarchyResolver(resolver OUHierarchyResolver)

	// CanGrantPermissions refuses an operation that would confer permissions the caller does not
	// itself hold. Use it where the granted permissions are already known, such as when defining a
	// role's permission list.
	//
	// A nil return permits the operation. Any non-nil ServiceError is a refusal, including
	// InternalServerError: an incomplete check is not a pass.
	CanGrantPermissions(ctx context.Context, granted security.PermissionSet) *tidcommon.ServiceError

	// CanGrantMembership refuses adding or removing a member of a group or role whose permissions
	// the caller does not itself hold. It resolves what the container confers, including through
	// group nesting.
	CanGrantMembership(ctx context.Context, principalType PrincipalType,
		containerID string) *tidcommon.ServiceError

	// SetPermissionResolver injects the effective-permission resolver used by the two checks above.
	// It must be called once at startup after the role and group packages are initialized,
	// completing the same two-phase initialization used by SetOUHierarchyResolver. Until it is
	// called, both checks refuse.
	SetPermissionResolver(resolver PermissionResolver)
}

// systemAuthorizationService holds what every boundary shares: the caller checks that run before a
// boundary is applied, and the grant checks. Each boundary is a SystemAuthorizationServiceInterface
// implementation that embeds it.
type systemAuthorizationService struct {
	logger *log.Logger
	// permissionResolver enumerates effective permissions for the grant checks. nil until
	// SetPermissionResolver is called at startup, in which case those checks refuse.
	permissionResolver PermissionResolver
}

// newSystemAuthorizationService returns the shared part of a boundary implementation.
func newSystemAuthorizationService() *systemAuthorizationService {
	return &systemAuthorizationService{
		logger: log.GetLogger().With(log.String("component", "SystemAuthorizationService")),
	}
}

// authorizeCaller runs the checks every boundary applies before its own. It returns decided true
// with the outcome when one of them settles the request, and decided false when the caller holds the
// action's permission and the boundary decides. The root "system" permission covers every system
// permission, but only within the boundary.
func (s *systemAuthorizationService) authorizeCaller(ctx context.Context, action security.Action,
	actionCtx *ActionContext) (allowed, decided bool) {
	logger := s.logger.WithContext(ctx)

	// Step 1: Check if this is an internal runtime caller.
	if security.IsRuntimeContext(ctx) {
		logger.Debug(ctx, "Authorization granted: runtime context for the action",
			log.String("action", string(action)))
		return true, true
	}

	// Step 2: Verify the caller is authenticated.
	subject := security.GetSubject(ctx)
	if subject == "" {
		logger.Debug(ctx, "Authorization denied: unauthenticated caller",
			log.String("action", string(action)))
		return false, true
	}

	// Step 3: Allow resource owners to access their own resources (self-service).
	if isResourceOwner(ctx, actionCtx) {
		if logger.IsDebugEnabled() {
			logger.Debug(ctx, "Authorization granted: resource owner",
				log.String("action", string(action)),
				log.MaskedString("subject", subject))
		}
		return true, true
	}

	// Step 4: Resolve required permission for the action and evaluate using hierarchical matching.
	requiredPermission := security.ResolveActionPermission(action)
	if !security.HasSufficientPermission(security.GetPermissions(ctx), requiredPermission) {
		if logger.IsDebugEnabled() {
			logger.Debug(ctx, "Authorization denied: insufficient permissions",
				log.String("action", string(action)),
				log.MaskedString("subject", subject))
		}
		return false, true
	}
	return false, false
}

// authorizeListCaller runs the checks every boundary applies to a list before its own. It returns
// decided true with the result when one of them settles the request, and decided false when the
// caller holds the action's permission and the boundary decides.
func (s *systemAuthorizationService) authorizeListCaller(ctx context.Context, action security.Action,
	resourceType security.ResourceType) (result *AccessibleResources, decided bool) {
	logger := s.logger.WithContext(ctx)

	// Step 1: Check if this is an internal runtime caller — return all resources.
	if security.IsRuntimeContext(ctx) {
		logger.Debug(ctx, "GetAccessibleResources: runtime context, returning all resources",
			log.String("action", string(action)),
			log.String("resourceType", string(resourceType)))
		return &AccessibleResources{AllAllowed: true}, true
	}

	// Step 2: Verify the caller is authenticated.
	subject := security.GetSubject(ctx)
	if subject == "" {
		logger.Debug(ctx, "GetAccessibleResources denied: unauthenticated caller",
			log.String("action", string(action)),
			log.String("resourceType", string(resourceType)))
		return &AccessibleResources{AllAllowed: false, IDs: []string{}}, true
	}

	// Step 3: Verify the caller holds an adequate permission for the action using hierarchical matching.
	requiredPermission := security.ResolveActionPermission(action)
	if !security.HasSufficientPermission(security.GetPermissions(ctx), requiredPermission) {
		if logger.IsDebugEnabled() {
			logger.Debug(ctx, "GetAccessibleResources denied: insufficient permissions",
				log.String("action", string(action)),
				log.String("resourceType", string(resourceType)),
				log.MaskedString("subject", subject))
		}
		return &AccessibleResources{AllAllowed: false, IDs: []string{}}, true
	}
	return nil, false
}

// isResourceOwner checks whether the authenticated caller is the owner of the resource
// being acted upon. This enables self-service operations (e.g., a user accessing their own
// profile) without requiring system-level permissions.
//
// Returns true only when:
//   - The ActionContext carries a non-empty ResourceID.
//   - The resource type supports owner-based access (currently only ResourceTypeUser).
//   - The caller's subject matches the ResourceID.
func isResourceOwner(ctx context.Context, actionCtx *ActionContext) bool {
	if actionCtx == nil || actionCtx.ResourceID == "" {
		return false
	}

	// Currently only user resources support owner-based access.
	if actionCtx.ResourceType != security.ResourceTypeUser {
		return false
	}

	return security.GetSubject(ctx) == actionCtx.ResourceID
}
