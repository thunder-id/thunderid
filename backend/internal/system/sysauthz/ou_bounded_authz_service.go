// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sysauthz

import (
	"context"
	"slices"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// ouBoundedAuthzService confines a caller to the organization unit its token names, including a caller
// holding the root system permission. Root organization units, and connections that belong to no
// organization unit, are created only through import, by the management API key, which reaches every
// organization unit in the deployment.
type ouBoundedAuthzService struct {
	*systemAuthorizationService
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

// newOUBoundedAuthzService returns an ouBoundedAuthzService.
func newOUBoundedAuthzService() *ouBoundedAuthzService {
	return &ouBoundedAuthzService{
		systemAuthorizationService: newSystemAuthorizationService(),
		policies: &policies{
			membershipPolicy: &ouMembershipPolicy{},
		},
	}
}

// IsActionAllowed allows a caller holding the action's permission on the organization unit its
// token names.
func (s *ouBoundedAuthzService) IsActionAllowed(ctx context.Context, action security.Action,
	actionCtx *ActionContext) (bool, *tidcommon.ServiceError) {
	if allowed, decided := s.authorizeCaller(ctx, action, actionCtx); decided {
		return allowed, nil
	}

	logger := s.logger.WithContext(ctx)
	subject := security.GetSubject(ctx)

	// The management API key imports whole organization unit trees, so it reaches every unit.
	if security.IsManagementAPIKeyCaller(ctx) {
		return true, nil
	}

	// Some actions reach a data plane only through import, never from a token.
	if isImportOnlyAction(action, actionCtx) {
		if logger.IsDebugEnabled() {
			logger.Debug(ctx, "Authorization denied: action allowed only through import",
				log.String("action", string(action)),
				log.MaskedString("subject", subject))
		}
		return false, nil
	}

	allowed, svcErr := isActionAllowedByPolicies(ctx, s.policies, action, actionCtx)
	if svcErr != nil {
		return false, svcErr
	}
	if logger.IsDebugEnabled() {
		if allowed {
			logger.Debug(ctx, "Authorization granted",
				log.String("action", string(action)),
				log.MaskedString("subject", subject))
		} else {
			logger.Debug(ctx, "Authorization denied: policy evaluation failed",
				log.String("action", string(action)),
				log.MaskedString("subject", subject))
		}
	}
	return allowed, nil
}

// GetAccessibleResources returns the resources a caller may list within the organization unit its
// token names.
func (s *ouBoundedAuthzService) GetAccessibleResources(ctx context.Context, action security.Action,
	resourceType security.ResourceType) (*AccessibleResources, *tidcommon.ServiceError) {
	if result, decided := s.authorizeListCaller(ctx, action, resourceType); decided {
		return result, nil
	}
	if security.IsManagementAPIKeyCaller(ctx) {
		return &AccessibleResources{AllAllowed: true}, nil
	}

	result, svcErr := getAccessibleResourcesByPolicies(ctx, s.policies, action, resourceType)
	if svcErr != nil {
		return nil, svcErr
	}
	logger := s.logger.WithContext(ctx)
	if logger.IsDebugEnabled() && !result.AllAllowed {
		logger.Debug(ctx, "GetAccessibleResources: restricted by policy",
			log.String("action", string(action)),
			log.String("resourceType", string(resourceType)),
			log.MaskedString("subject", security.GetSubject(ctx)),
			log.Int("accessibleCount", len(result.IDs)))
	}
	return result, nil
}

// SetOUHierarchyResolver injects the OU hierarchy resolver into the service.
// It is called once at application startup after the ou package is initialized.
// The ouInheritancePolicy is built once here and reused for every subsequent authz call.
func (s *ouBoundedAuthzService) SetOUHierarchyResolver(resolver OUHierarchyResolver) {
	if resolver == nil {
		return
	}
	s.policies.inheritancePolicy = &ouInheritancePolicy{resolver: resolver}
}

// importOnlyRule reports whether an action, in its context, reaches a data plane only through import.
type importOnlyRule func(action security.Action, actionCtx *ActionContext) bool

// importOnlyActions are the rules for actions a data plane accepts only through import, never from a
// token.
var importOnlyActions = []importOnlyRule{
	isRootOUCreation,
	isConnectionCreationWithoutOU,
}

// isImportOnlyAction reports whether the action may reach a data plane only through import.
func isImportOnlyAction(action security.Action, actionCtx *ActionContext) bool {
	return slices.ContainsFunc(importOnlyActions, func(isImportOnly importOnlyRule) bool {
		return isImportOnly(action, actionCtx)
	})
}

// isRootOUCreation reports whether the action creates an organization unit with no parent.
func isRootOUCreation(action security.Action, actionCtx *ActionContext) bool {
	return action == security.ActionCreateOU && actionCtx != nil && actionCtx.OUID == ""
}

// isConnectionCreationWithoutOU reports whether the action creates a connection that belongs to no
// organization unit. Such a connection serves the whole deployment, which no token names.
func isConnectionCreationWithoutOU(action security.Action, actionCtx *ActionContext) bool {
	return action == security.ActionCreateConnection && actionCtx != nil && actionCtx.OUID == ""
}
