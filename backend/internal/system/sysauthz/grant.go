// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sysauthz

import (
	"context"
	"fmt"
	"slices"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// PrincipalType identifies the kind of container whose conferred permissions are examined.
type PrincipalType string

const (
	// PrincipalTypeGroup identifies a group.
	PrincipalTypeGroup PrincipalType = "group"
	// PrincipalTypeRole identifies a role.
	PrincipalTypeRole PrincipalType = "role"
)

// PermissionResolver enumerates the permissions a principal effectively holds, across every
// resource server. Declared here because role imports group and group imports sysauthz, so
// sysauthz can import neither; an implementation is injected by SetPermissionResolver at startup.
//
// Implementations must return a complete set. An incomplete result changes the outcome of the
// checks below, so partial data is not an acceptable degradation.
type PermissionResolver interface {
	// ResolveForEntity returns the permissions an entity holds, directly and through its groups.
	ResolveForEntity(ctx context.Context, entityID string) (security.PermissionSet, error)

	// ResolveForGroup returns the permissions membership in a group confers, including those
	// carried by its ancestor groups.
	ResolveForGroup(ctx context.Context, groupID string) (security.PermissionSet, error)

	// ResolveForRole returns the permissions a role confers on its assignees.
	ResolveForRole(ctx context.Context, roleID string) (security.PermissionSet, error)

	// ResolveDelimiters returns administrative coverage delimiters; missing entries use exact matching.
	// The resolver avoids an import cycle between sysauthz and the resource service.
	ResolveDelimiters(ctx context.Context, resourceServerIDs []string) map[string]string
}

// SetPermissionResolver injects the resolver used by the grant checks. Call once at startup, after
// the role and group packages are initialized. Until then the checks refuse.
func (s *systemAuthorizationService) SetPermissionResolver(resolver PermissionResolver) {
	if resolver == nil {
		return
	}
	s.permissionResolver = resolver
}

// CanGrantPermissions enforces that the caller may only confer permissions it already holds. Use
// it when the granted set is already known, such as a role's permission list; prefer
// CanGrantMembership otherwise, since that resolves nesting for the caller.
//
// Any non-nil error is a refusal, including InternalServerError.
func (s *systemAuthorizationService) CanGrantPermissions(
	ctx context.Context, granted security.PermissionSet,
) *tidcommon.ServiceError {
	if decision, decided := s.shortCircuitGrant(ctx); decided {
		return decision
	}
	return s.requireCoverage(ctx, granted)
}

// CanGrantMembership enforces that the caller may only add to, or remove from, a group or role
// whose permissions it already holds. Both directions carry the same requirement.
func (s *systemAuthorizationService) CanGrantMembership(
	ctx context.Context, principalType PrincipalType, containerID string,
) *tidcommon.ServiceError {
	if !membershipApproved(ctx, principalType, containerID) {
		s.logger.Debug(ctx, "Grant refused: membership not approved", log.String("target", string(principalType)))
		return &ErrorGrantNotPermitted
	}
	return s.requireCovers(ctx, containerID, string(principalType), s.conferredBy(principalType))
}

// membershipApproved reports whether a caller limited to the writes it approved approved changing the
// membership of the group or role. Any other caller is not limited.
func membershipApproved(ctx context.Context, principalType PrincipalType, containerID string) bool {
	if !security.IsLimitedToApprovedActions(ctx) {
		return true
	}
	action := security.ActionUpdateGroup
	if principalType == PrincipalTypeRole {
		action = security.ActionUpdateRole
	}
	return security.IsApprovedOn(ctx, action, containerID)
}

// CanControlEntity enforces that the caller may only take control of an entity, such as by changing
// its credentials or attributes or by deleting it, when it already holds every permission the entity holds.
func (s *systemAuthorizationService) CanControlEntity(
	ctx context.Context, entityID string,
) *tidcommon.ServiceError {
	// An entity always holds its own permissions, so acting on itself needs no lookup.
	if sub := security.GetSubject(ctx); sub != "" && sub == entityID {
		return nil
	}
	return s.requireCovers(ctx, entityID, "entity",
		func(ctx context.Context, id string) (security.PermissionSet, error) {
			return s.permissionResolver.ResolveForEntity(ctx, id)
		})
}

// requireCovers refuses unless the caller holds every permission resolve returns for the target.
func (s *systemAuthorizationService) requireCovers(ctx context.Context, id, what string,
	resolve func(context.Context, string) (security.PermissionSet, error),
) *tidcommon.ServiceError {
	if decision, decided := s.shortCircuitGrant(ctx); decided {
		return decision
	}

	logger := s.logger.WithContext(ctx)
	if id == "" {
		logger.Error(ctx, "Grant check called without a target ID", log.String("target", what))
		return &tidcommon.InternalServerError
	}

	conferred, err := resolve(ctx, id)
	if err != nil {
		// A failed lookup is not the same as an empty result, so it must not be treated as one.
		logger.Error(ctx, "Failed to resolve the permissions of the grant target",
			log.String("target", what), log.String("targetID", id), log.Error(err))
		return &tidcommon.InternalServerError
	}

	return s.requireCoverage(ctx, conferred)
}

// GrantableMemberships returns the containers whose membership the caller may grant, resolving what each confers
// and, once, what the caller holds.
func (s *systemAuthorizationService) GrantableMemberships(
	ctx context.Context, principalType PrincipalType, containerIDs []string,
) ([]string, *tidcommon.ServiceError) {
	containerIDs = slices.DeleteFunc(slices.Clone(containerIDs), func(id string) bool {
		return !membershipApproved(ctx, principalType, id)
	})
	if len(containerIDs) == 0 {
		return nil, nil
	}
	if decision, decided := s.shortCircuitGrant(ctx); decided {
		if decision != nil {
			return nil, decision
		}
		return slices.Clone(containerIDs), nil
	}

	subject := security.GetSubject(ctx)
	var actorPermissions security.PermissionSet
	grantable := make([]string, 0, len(containerIDs))
	for _, containerID := range containerIDs {
		conferred, svcErr := s.resolveConferredPermissions(ctx, principalType, containerID)
		if svcErr != nil {
			return nil, svcErr
		}
		if grantsNothing(conferred) {
			grantable = append(grantable, containerID)
			continue
		}
		if subject == "" {
			continue
		}
		if actorPermissions == nil {
			resolved, err := s.permissionResolver.ResolveForEntity(ctx, subject)
			if err != nil {
				s.logger.Error(ctx, "Failed to resolve caller permissions for grant check",
					log.MaskedString("subject", subject), log.Error(err))
				return nil, &tidcommon.InternalServerError
			}
			actorPermissions = resolved
			if actorPermissions == nil {
				actorPermissions = security.PermissionSet{}
			}
		}
		delimiters := s.permissionResolver.ResolveDelimiters(ctx, grantedResourceServerIDs(conferred))
		if security.Covers(actorPermissions, conferred, delimiters) {
			grantable = append(grantable, containerID)
		}
	}
	return grantable, nil
}

// shortCircuitGrant decides the cases that need no stored lookup. The boolean reports whether a
// decision was reached.
func (s *systemAuthorizationService) shortCircuitGrant(
	ctx context.Context,
) (*tidcommon.ServiceError, bool) {
	logger := s.logger.WithContext(ctx)

	// Internal runtime callers act without a subject, as bootstrap and the declarative loaders do.
	if security.IsRuntimeContext(ctx) {
		return nil, true
	}

	// Refuse rather than pass when startup wiring is incomplete.
	if s.permissionResolver == nil {
		logger.Error(ctx, "Grant check invoked before a permission resolver was injected")
		return &tidcommon.InternalServerError, true
	}

	// The root permission already covers every system operation, so the comparison is redundant.
	// Read from the token, as IsActionAllowed does.
	if security.HasSystemPermission(security.GetPermissions(ctx)) {
		return nil, true
	}

	return nil, false
}

// resolveConferredPermissions determines what membership in the given container confers.
func (s *systemAuthorizationService) resolveConferredPermissions(
	ctx context.Context, principalType PrincipalType, containerID string,
) (security.PermissionSet, *tidcommon.ServiceError) {
	conferred, err := s.conferredBy(principalType)(ctx, containerID)
	if err != nil {
		// A failed lookup is not the same as an empty result, so it must not be treated as one.
		s.logger.WithContext(ctx).Error(ctx, "Failed to resolve permissions conferred by container",
			log.String("principalType", string(principalType)),
			log.String("containerID", containerID),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return conferred, nil
}

// conferredBy returns the lookup for what membership in a container of the given type confers. The
// resolver is read when the lookup runs, after shortCircuitGrant has confirmed it is set.
func (s *systemAuthorizationService) conferredBy(
	principalType PrincipalType,
) func(context.Context, string) (security.PermissionSet, error) {
	return func(ctx context.Context, containerID string) (security.PermissionSet, error) {
		switch principalType {
		case PrincipalTypeGroup:
			return s.permissionResolver.ResolveForGroup(ctx, containerID)
		case PrincipalTypeRole:
			return s.permissionResolver.ResolveForRole(ctx, containerID)
		}
		return nil, fmt.Errorf("unknown principal type %q", principalType)
	}
}

// requireCoverage refuses unless the caller's permissions cover every permission being granted.
func (s *systemAuthorizationService) requireCoverage(
	ctx context.Context, granted security.PermissionSet,
) *tidcommon.ServiceError {
	logger := s.logger.WithContext(ctx)

	// Nothing is conferred, so the comparison is unnecessary. This is the common case.
	if grantsNothing(granted) {
		return nil
	}

	subject := security.GetSubject(ctx)
	if subject == "" {
		logger.Debug(ctx, "Grant refused: no authenticated subject")
		return &ErrorGrantNotPermitted
	}

	// From storage, not the token: the token's claim is narrowed to one resource server and carries
	// no resource server labels, so it cannot answer a cross-server coverage question.
	actorPermissions, err := s.permissionResolver.ResolveForEntity(ctx, subject)
	if err != nil {
		logger.Error(ctx, "Failed to resolve caller permissions for grant check",
			log.MaskedString("subject", subject), log.Error(err))
		return &tidcommon.InternalServerError
	}

	// Covers preserves fixed colon matching for system permissions.
	delimiters := s.permissionResolver.ResolveDelimiters(ctx, grantedResourceServerIDs(granted))

	if !security.Covers(actorPermissions, granted, delimiters) {
		logger.Debug(ctx, "Grant refused: exceeds caller permissions",
			log.MaskedString("subject", subject))
		return &ErrorGrantNotPermitted
	}

	return nil
}

// grantsNothing reports whether a permission set confers nothing.
func grantsNothing(granted security.PermissionSet) bool {
	for _, permissions := range granted {
		if len(permissions) > 0 {
			return false
		}
	}
	return true
}

// grantedResourceServerIDs lists the resource servers a grant names, which are the only ones whose
// delimiters the coverage check needs.
func grantedResourceServerIDs(granted security.PermissionSet) []string {
	ids := make([]string, 0, len(granted))
	for resourceServerID, permissions := range granted {
		if resourceServerID == "" || len(permissions) == 0 {
			continue
		}
		ids = append(ids, resourceServerID)
	}
	return ids
}
