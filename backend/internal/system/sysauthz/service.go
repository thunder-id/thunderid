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

// Package sysauthz provides system-level authorization services for protecting
// system operations management.
package sysauthz

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// SystemAuthorizationServiceInterface defines the contract for system-level authorization.
type SystemAuthorizationServiceInterface interface {
	// IsActionAllowed checks whether the authenticated caller is permitted to perform
	// the given action. Returns true if allowed, false if denied. A non-nil ServiceError
	// indicates a processing failure, not an authorization denial.
	IsActionAllowed(ctx context.Context, action security.Action,
		actionCtx *ActionContext) (bool, *serviceerror.ServiceError)

	// GetAccessibleResources returns the set of resources the caller may access for the
	// given action and resource type. The result must be applied as a store-level filter
	// before pagination so that page sizes and total counts remain correct.
	//
	// When AllAllowed is true, no ID filter should be applied.
	// When AllAllowed is false, the store should restrict results to the returned IDs.
	GetAccessibleResources(ctx context.Context, action security.Action,
		resourceType security.ResourceType) (*AccessibleResources, *serviceerror.ServiceError)

	// SetOUHierarchyResolver injects the OU hierarchy resolver used by inheritance-based
	// policies. This must be called once at application startup after the ou package has
	// been initialized, completing the two-phase initialization that avoids an import cycle
	// between sysauthz (which ou already imports) and the ou package itself.
	SetOUHierarchyResolver(resolver OUHierarchyResolver)
}

// systemAuthorizationService is the default implementation of SystemAuthorizationServiceInterface.
type systemAuthorizationService struct {
	logger   *log.Logger
	policies *policies
}

type policies struct {
	// membershipPolicy enforces same-OU access for standard operations.
	// Declared as an interface so that tests can inject stubs without importing
	// the concrete type directly.
	membershipPolicy authorizationPolicy
	// inheritancePolicy grants child-OU callers read access to parent-OU resources.
	// nil when no OUHierarchyResolver has been injected yet.
	inheritancePolicy authorizationPolicy
}

// newSystemAuthorizationService returns a new systemAuthorizationService.
func newSystemAuthorizationService() SystemAuthorizationServiceInterface {
	return &systemAuthorizationService{
		logger: log.GetLogger().With(log.String("component", "SystemAuthorizationService")),
		policies: &policies{
			membershipPolicy: &ouMembershipPolicy{},
		},
	}
}

// SetOUHierarchyResolver injects the OU hierarchy resolver into the service.
// It is called once at application startup after the ou package is initialized.
// The ouInheritancePolicy is built once here and reused for every subsequent authz call.
func (s *systemAuthorizationService) SetOUHierarchyResolver(resolver OUHierarchyResolver) {
	if resolver == nil {
		return
	}
	s.policies.inheritancePolicy = &ouInheritancePolicy{resolver: resolver}
}

// IsActionAllowed evaluates whether the authenticated caller may perform the given action.
func (s *systemAuthorizationService) IsActionAllowed(ctx context.Context, action security.Action,
	actionCtx *ActionContext) (bool, *serviceerror.ServiceError) {
	logger := s.logger.WithContext(ctx)

	// Step 1: Check if SKIP_SECURITY flag is set.
	if security.IsSecuritySkipped(ctx) {
		logger.Debug("Authorization skipped: SKIP_SECURITY is enabled",
			log.String("action", string(action)))
		return true, nil
	}

	// Step 2: Check if this is an internal runtime caller.
	if security.IsRuntimeContext(ctx) {
		logger.Debug("Authorization granted: runtime context for the action",
			log.String("action", string(action)))
		return true, nil
	}

	// Step 3: Verify the caller is authenticated.
	subject := security.GetSubject(ctx)
	if subject == "" {
		logger.Debug("Authorization denied: unauthenticated caller",
			log.String("action", string(action)))
		return false, nil
	}

	permissions := security.GetPermissions(ctx)

	// Step 4: Short-circuit: the "system" permission grants access to all system operations.
	if security.HasSystemPermission(permissions) {
		return true, nil
	}

	// Step 5: Allow resource owners to access their own resources (self-service).
	if isResourceOwner(ctx, actionCtx) {
		if logger.IsDebugEnabled() {
			logger.Debug("Authorization granted: resource owner",
				log.String("action", string(action)),
				log.MaskedString("subject", subject))
		}
		return true, nil
	}

	// Step 6: Resolve required permission for the action and evaluate using hierarchical matching.
	requiredPermission := security.ResolveActionPermission(action)
	if !security.HasSufficientPermission(permissions, requiredPermission) {
		if logger.IsDebugEnabled() {
			logger.Debug("Authorization denied: insufficient permissions",
				log.String("action", string(action)),
				log.MaskedString("subject", subject))
		}
		return false, nil
	}

	// Step 7: Evaluate global policies (e.g., OU scope check).
	allowed, svcErr := isActionAllowedByPolicies(ctx, s.policies, action, actionCtx)
	if svcErr != nil {
		return false, svcErr
	}
	if !allowed {
		if logger.IsDebugEnabled() {
			logger.Debug("Authorization denied: policy evaluation failed",
				log.String("action", string(action)),
				log.MaskedString("subject", subject))
		}
		return false, nil
	}

	if logger.IsDebugEnabled() {
		logger.Debug("Authorization granted",
			log.String("action", string(action)),
			log.MaskedString("subject", subject))
	}

	return true, nil
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

// GetAccessibleResources returns the set of resources the caller can access for the given
// action and resource type.
func (s *systemAuthorizationService) GetAccessibleResources(ctx context.Context, action security.Action,
	resourceType security.ResourceType) (*AccessibleResources, *serviceerror.ServiceError) {
	logger := s.logger.WithContext(ctx)

	// Step 1: Check if SKIP_SECURITY flag is set.
	if security.IsSecuritySkipped(ctx) {
		logger.Debug("GetAccessibleResources skipped: SKIP_SECURITY is enabled",
			log.String("action", string(action)),
			log.String("resourceType", string(resourceType)))
		return &AccessibleResources{AllAllowed: true}, nil
	}

	// Step 2: Check if this is an internal runtime caller — return all resources.
	if security.IsRuntimeContext(ctx) {
		logger.Debug("GetAccessibleResources: runtime context, returning all resources",
			log.String("action", string(action)),
			log.String("resourceType", string(resourceType)))
		return &AccessibleResources{AllAllowed: true}, nil
	}

	// Step 3: Verify the caller is authenticated.
	subject := security.GetSubject(ctx)
	if subject == "" {
		logger.Debug("GetAccessibleResources denied: unauthenticated caller",
			log.String("action", string(action)),
			log.String("resourceType", string(resourceType)))
		return &AccessibleResources{AllAllowed: false, IDs: []string{}}, nil
	}

	permissions := security.GetPermissions(ctx)

	// Step 4: Short-circuit: the "system" permission grants access to all resources.
	if security.HasSystemPermission(permissions) {
		return &AccessibleResources{AllAllowed: true}, nil
	}

	// Step 5: Verify the caller holds an adequate permission for the action using hierarchical matching.
	requiredPermission := security.ResolveActionPermission(action)
	if !security.HasSufficientPermission(permissions, requiredPermission) {
		if logger.IsDebugEnabled() {
			logger.Debug("GetAccessibleResources denied: insufficient permissions",
				log.String("action", string(action)),
				log.String("resourceType", string(resourceType)),
				log.MaskedString("subject", subject))
		}
		return &AccessibleResources{AllAllowed: false, IDs: []string{}}, nil
	}

	// Step 6: Delegate to the policy chain to determine the accessible resource set.
	result, svcErr := getAccessibleResourcesByPolicies(ctx, s.policies, action, resourceType)
	if svcErr != nil {
		return nil, svcErr
	}
	if logger.IsDebugEnabled() && !result.AllAllowed {
		logger.Debug("GetAccessibleResources: restricted by policy",
			log.String("action", string(action)),
			log.String("resourceType", string(resourceType)),
			log.MaskedString("subject", subject),
			log.Int("accessibleCount", len(result.IDs)))
	}
	return result, nil
}
