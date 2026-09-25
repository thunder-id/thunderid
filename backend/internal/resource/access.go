// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// permissionFilter decides which of a resource server's permission strings a scope may see.
//
// An organization unit that owns the server, or is not bounded at all, sees everything. A sharee
// sees what the resolved resources rule leaves it: withholding a branch has to hide that branch
// from the listings too, or an exclusion only removes the permission and not the knowledge of it.
type permissionFilter struct {
	// unrestricted short-circuits every check.
	unrestricted bool
	// allowed is the set of permitted paths; a path is permitted when it is covered by one of them.
	allowed []string
	// excluded is subtracted from allowed, last.
	excluded []string
	// delimiter joins path segments for this server.
	delimiter string
}

// permits reports whether one permission string survives the filter.
//
// Both sides are prefix containment in the server's own delimiter, so naming a branch admits
// everything beneath it, and excluding one removes the branch and its descendants together.
func (f permissionFilter) permits(permission string) bool {
	if f.unrestricted {
		return true
	}
	for _, e := range f.excluded {
		if covers(e, permission, f.delimiter) {
			return false
		}
	}
	if f.allowed == nil {
		return true
	}
	for _, a := range f.allowed {
		if covers(a, permission, f.delimiter) {
			return true
		}
	}
	return false
}

// covers reports whether outer denotes inner: itself, or any path beneath it.
func covers(outer, inner, delimiter string) bool {
	if outer == inner {
		return true
	}
	if delimiter == "" {
		return false
	}
	return strings.HasPrefix(inner, outer+delimiter)
}

// A nil authorization service means none was wired, which happens only outside the server: the
// production constructor always supplies one. Such a caller is treated as unbounded, because
// there is no caller identity to bound it against.
//
// resourceServerView resolves whether the caller may see a resource server and, when it sees the
// server only because a policy shared it, which part of its tree that policy left.
//
// Withholding a branch has to hide it from the listings too, or an exclusion removes the permission
// without removing the knowledge of it.
func (rs *resourceService) resourceServerView(
	ctx context.Context, server *providers.ResourceServer,
) (permissionFilter, *tidcommon.ServiceError) {
	if rs.authzService == nil {
		return permissionFilter{unrestricted: true}, nil
	}

	accessible, svcErr := rs.authzService.GetAccessibleResources(
		ctx, security.ActionReadResourceServer, security.ResourceTypeOU)
	if svcErr != nil {
		rs.logger.Error(ctx, "Failed to resolve accessible organization units for reading a "+
			"resource server", log.Any("error", svcErr))
		return permissionFilter{}, &tidcommon.InternalServerError
	}

	// A deployment-wide caller, and the owning organization unit itself, see the server as its
	// owner defined it. Everyone else sees it through whatever policy shared it.
	if accessible.AllAllowed || slices.Contains(accessible.IDs, server.OUID) {
		return permissionFilter{unrestricted: true}, nil
	}
	if rs.sharingService == nil {
		return permissionFilter{}, &ErrorResourceServerNotFound
	}

	for _, ouID := range accessible.IDs {
		visible, svcErr := rs.sharingService.IsVisible(ctx, ResourceServerSharingType, server.ID, ouID)
		if svcErr != nil {
			return permissionFilter{}, mapSharingError(svcErr)
		}
		if !visible {
			continue
		}
		return rs.sharedTreeFilter(ctx, server, ouID)
	}
	return permissionFilter{}, &ErrorResourceServerNotFound
}

// resolveViewingOU resolves the organization unit a read is answered as.
//
// A token that carries one is answered as that one, so a tenant never has to name itself. A
// deployment-wide token carries none, and there is no single organization unit its answer could be
// about, so it has to say which: the parameter is optional for the first caller and required for
// the second, rather than either always or never.
//
// An explicit request still narrows and never widens. Without that the default would be cosmetic,
// since a tenant could name any organization unit it liked and be answered.
func (rs *resourceService) resolveViewingOU(
	ctx context.Context, requested string,
) (string, *tidcommon.ServiceError) {
	if rs.authzService == nil {
		// No caller identity to bound against, so only an explicit organization unit can answer.
		if requested == "" {
			return "", &ErrorViewingOUIDRequired
		}
		return requested, nil
	}

	accessible, svcErr := rs.authzService.GetAccessibleResources(
		ctx, security.ActionReadResourceServer, security.ResourceTypeOU)
	if svcErr != nil {
		rs.logger.Error(ctx, "Failed to resolve accessible organization units for reading a "+
			"resource server", log.Any("error", svcErr))
		return "", &tidcommon.InternalServerError
	}

	if requested != "" {
		if !accessible.AllAllowed && !slices.Contains(accessible.IDs, requested) {
			return "", &ErrorResourceServerNotFound
		}
		return requested, nil
	}

	switch {
	case accessible.AllAllowed || len(accessible.IDs) > 1:
		return "", &ErrorViewingOUIDRequired
	case len(accessible.IDs) == 0:
		// No standing over any organization unit, so there is nothing to be answered about.
		return "", &ErrorResourceServerNotFound
	default:
		return accessible.IDs[0], nil
	}
}

// requireVisibleToOU reports whether one organization unit holds a resource server at all.
//
// The caller's standing over that unit is settled separately, by resolveViewingOU. What is left is
// the unit's own question: it holds the server by owning it or by a policy reaching it, and holding
// it neither way is not-found rather than an empty answer.
func (rs *resourceService) requireVisibleToOU(
	ctx context.Context, server *providers.ResourceServer, ouID string,
) *tidcommon.ServiceError {
	if server.OUID == ouID {
		return nil
	}
	if rs.sharingService == nil {
		return &ErrorResourceServerNotFound
	}

	visible, svcErr := rs.sharingService.IsVisible(ctx, ResourceServerSharingType, server.ID, ouID)
	if svcErr != nil {
		return mapSharingError(svcErr)
	}
	if !visible {
		return &ErrorResourceServerNotFound
	}
	return nil
}

// sharedTreeFilter reads the resources rule an organization unit holds for a server.
func (rs *resourceService) sharedTreeFilter(
	ctx context.Context, server *providers.ResourceServer, ouID string,
) (permissionFilter, *tidcommon.ServiceError) {
	resolved, svcErr := rs.sharingService.ResolveOverlayRules(
		ctx, ResourceServerSharingType, server.ID, ouID)
	if svcErr != nil {
		return permissionFilter{}, mapSharingError(svcErr)
	}

	rule, ok := resolved.Rules[ResourcesFieldKey]
	if !ok {
		return permissionFilter{unrestricted: true}, nil
	}

	filter := permissionFilter{delimiter: server.Delimiter}
	if rule.Value != nil {
		filter.allowed = *rule.Value
	}
	if rule.ExcludedValues != nil {
		filter.excluded = *rule.ExcludedValues
	}
	return filter, nil
}

// requireResourceServerOwnership authorizes a change to a resource server or anything beneath it.
//
// Being shared a resource server conveys the right to use its permissions, not to change them: the
// owner's tree is one definition shared by every organization unit that can see it, so a sharee
// editing it would change what a permission means for everyone else.
func (rs *resourceService) requireResourceServerOwnership(
	ctx context.Context, action security.Action, server *providers.ResourceServer,
) *tidcommon.ServiceError {
	if rs.authzService == nil {
		return nil
	}

	accessible, svcErr := rs.authzService.GetAccessibleResources(ctx, action, security.ResourceTypeOU)
	if svcErr != nil {
		rs.logger.Error(ctx, "Failed to resolve accessible organization units for changing a "+
			"resource server", log.Any("error", svcErr))
		return &tidcommon.InternalServerError
	}
	if accessible.AllAllowed || slices.Contains(accessible.IDs, server.OUID) {
		return nil
	}
	return &ErrorResourceServerModificationRestrictedToOwner
}

// serverForAccess loads a resource server without an access check, for the callers that are about
// to make that decision themselves.
func (rs *resourceService) serverForAccess(
	ctx context.Context, id string,
) (*providers.ResourceServer, *tidcommon.ServiceError) {
	server, err := rs.resourceStore.GetResourceServer(ctx, id)
	if err != nil {
		if errors.Is(err, errResourceServerNotFound) {
			return nil, &ErrorResourceServerNotFound
		}
		rs.logger.Error(ctx, "Failed to get resource server", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return &server, nil
}

// ownedResourceServer loads a resource server for a write, refusing a caller that was only shared
// it. Shaped like validateAndGetResourceServer so a write reads the same as a read.
func (rs *resourceService) ownedResourceServer(
	ctx context.Context, action security.Action, resourceServerID string,
) (providers.ResourceServer, *tidcommon.ServiceError) {
	server, svcErr := rs.validateAndOwnResourceServer(ctx, action, resourceServerID)
	if svcErr != nil {
		return providers.ResourceServer{}, svcErr
	}
	return *server, nil
}

// filterResourceList drops the resources the caller may not see and restates the counts, so a page
// never reports more rows than it carries.
func filterResourceList(list *ResourceList, filter permissionFilter) {
	if filter.unrestricted || list == nil {
		return
	}
	kept := make([]providers.Resource, 0, len(list.Resources))
	for _, res := range list.Resources {
		if filter.permits(res.Permission) {
			kept = append(kept, res)
		}
	}
	list.TotalResults -= len(list.Resources) - len(kept)
	list.Resources = kept
	list.Count = len(kept)
}

// filterActionList drops the actions the caller may not see and restates the counts.
func filterActionList(list *ActionList, filter permissionFilter) {
	if filter.unrestricted || list == nil {
		return
	}
	kept := make([]providers.Action, 0, len(list.Actions))
	for _, action := range list.Actions {
		if filter.permits(action.Permission) {
			kept = append(kept, action)
		}
	}
	list.TotalResults -= len(list.Actions) - len(kept)
	list.Actions = kept
	list.Count = len(kept)
}

// ownResourceServerForDelete authorizes a delete beneath a resource server.
//
// A delete is idempotent, so a caller distinguishes "no such server" from "not yours" by the error
// code rather than by whether one is returned at all.
func (rs *resourceService) ownResourceServerForDelete(
	ctx context.Context, resourceServerID string,
) *tidcommon.ServiceError {
	_, svcErr := rs.ownedResourceServer(ctx, security.ActionDeleteResourceServer, resourceServerID)
	return svcErr
}

// viewingOU reads the organization unit a listing asks to be answered as.
//
// It narrows, never widens: the service refuses a unit the caller has no standing over, so a
// deployment-wide caller can ask what a tenant sees while a tenant cannot ask about another. Only
// the listing takes it; every other read is answered from the caller's own standing.
func viewingOU(r *http.Request) string {
	return strings.TrimSpace(r.URL.Query().Get("ouId"))
}

// mapSharingError translates a sharing framework error into the resource server's own vocabulary.
//
// The framework's codes describe a generic policy engine and mean nothing to a caller of the
// resource server API, so none of them are allowed past this boundary. The framework's description
// is carried through as detail, because that is what names the offending field, organization unit
// or policy.
func mapSharingError(svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	if svcErr == nil {
		return nil
	}
	// A server-side failure carries no caller-actionable detail worth forwarding.
	if svcErr.Type != tidcommon.ClientErrorType {
		return &tidcommon.InternalServerError
	}

	switch svcErr.Code {
	case sharing.ErrorPolicyNotFound.Code:
		return withSharingDetail(ErrorSharingPolicyNotFound, svcErr)
	case sharing.ErrorResourceNotFound.Code:
		return &ErrorResourceServerNotFound
	case sharing.ErrorPolicyExists.Code:
		return withSharingDetail(ErrorSharingPolicyExists, svcErr)
	case sharing.ErrorVersionMismatch.Code:
		return withSharingDetail(ErrorSharingPolicyVersionMismatch, svcErr)
	case sharing.ErrorPolicyDeclared.Code:
		return withSharingDetail(ErrorSharingPolicyImmutable, svcErr)
	case sharing.ErrorNotShared.Code, sharing.ErrorCrossTreeShareRestricted.Code,
		sharing.ErrorCoreConfigOwnerOnly.Code:
		return withSharingDetail(ErrorSharingNotPermitted, svcErr)
	case sharing.ErrorResourceTypeNotRegistered.Code:
		// The resource server type is registered at startup, so reaching this means the server is
		// misconfigured rather than the caller being wrong.
		return &tidcommon.InternalServerError
	default:
		// Everything else is an authoring mistake in the policy itself: an unknown field, a rule
		// that widens, a value outside its own bounds, a target that breaks the one-hop rule.
		return withSharingDetail(ErrorInvalidSharingPolicy, svcErr)
	}
}

// withSharingDetail appends the framework's description to the resource server's own error, so the
// caller still learns which field, organization unit or policy was at fault.
func withSharingDetail(
	err tidcommon.ServiceError, svcErr *tidcommon.ServiceError,
) *tidcommon.ServiceError {
	out := err
	if detail := svcErr.ErrorDescription.DefaultValue; detail != "" {
		out.ErrorDescription.DefaultValue = err.ErrorDescription.DefaultValue + ": " + detail
	}
	return &out
}
