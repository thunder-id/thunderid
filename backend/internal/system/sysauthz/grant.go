// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sysauthz

import (
	"context"

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
	// ResolveForEntity returns the permissions an entity holds, directly and through its groups,
	// counting only roles owned by the entity's own organization unit, as its tokens do.
	ResolveForEntity(ctx context.Context, entityID string) (security.PermissionSet, error)

	// ResolveForGroup returns the permissions membership in a group confers, including those
	// carried by its ancestor groups.
	ResolveForGroup(ctx context.Context, groupID string) (security.PermissionSet, error)

	// ResolveForRole returns the permissions a role confers on its assignees.
	ResolveForRole(ctx context.Context, roleID string) (security.PermissionSet, error)
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
	logger := s.logger.WithContext(ctx)

	if decision, decided := s.shortCircuitGrant(ctx); decided {
		return decision
	}

	if containerID == "" {
		logger.Error(ctx, "Membership grant check called without a container ID",
			log.String("principalType", string(principalType)))
		return &tidcommon.InternalServerError
	}

	conferred, svcErr := s.resolveConferredPermissions(ctx, principalType, containerID)
	if svcErr != nil {
		return svcErr
	}

	return s.requireCoverage(ctx, conferred)
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
	logger := s.logger.WithContext(ctx)

	var conferred security.PermissionSet
	var err error

	switch principalType {
	case PrincipalTypeGroup:
		conferred, err = s.permissionResolver.ResolveForGroup(ctx, containerID)
	case PrincipalTypeRole:
		conferred, err = s.permissionResolver.ResolveForRole(ctx, containerID)
	default:
		logger.Error(ctx, "Membership grant check called with an unknown principal type",
			log.String("principalType", string(principalType)))
		return nil, &tidcommon.InternalServerError
	}

	if err != nil {
		// A failed lookup is not the same as an empty result, so it must not be treated as one.
		logger.Error(ctx, "Failed to resolve permissions conferred by container",
			log.String("principalType", string(principalType)),
			log.String("containerID", containerID),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return conferred, nil
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

	if !security.Covers(actorPermissions, granted) {
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
