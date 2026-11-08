// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sysauthz

import (
	"context"
	"slices"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/security"
)

// OUHierarchyResolver provides read-only traversal of the organization unit tree.
// It is defined here in the sysauthz package (rather than in the ou package) to break the
// potential import cycle: ou already imports sysauthz for its authz checks, so if sysauthz
// were to import ou for hierarchy traversal it would form a cycle.
// The ou package implements this interface and injects a concrete instance via
// SystemAuthorizationServiceInterface.SetOUHierarchyResolver at application startup.
type OUHierarchyResolver interface {
	// IsAncestor returns true when ancestorOUID appears anywhere in the parent
	// chain above descendantOUID.
	// A non-nil ServiceError indicates a traversal failure; the caller should treat the
	// result as false (deny-safe).
	IsAncestor(ctx context.Context, ancestorOUID, descendantOUID string) (bool, *tidcommon.ServiceError)

	// GetAncestorOUIDs returns every ancestor OU ID walking up
	// to the root of the tree. A non-nil ServiceError indicates a traversal failure.
	GetAncestorOUIDs(ctx context.Context, ouID string) ([]string, *tidcommon.ServiceError)
}

// ActionContext provides contextual information used to make an authorization decision.
// Not all fields are required for every action; populate only those relevant to the operation.
type ActionContext struct {
	// OUID is the organization unit ID scoping the action.
	// Leave empty if the action is not scoped to a specific OU.
	OUID string
	// ResourceType is the type of resource being acted upon.
	ResourceType security.ResourceType
	// ResourceID is the identifier of the specific resource being acted upon.
	// Leave empty for collection-level actions (e.g., list, create).
	ResourceID string
}

// AccessibleResources represents the set of resources a caller is permitted to access
// for a given action. It is used to pre-filter store queries before pagination is applied.
type AccessibleResources struct {
	// AllAllowed signals that the caller may access all resources of the requested type.
	// When true, the caller should apply no ID filter to the store query.
	// When false, only the IDs listed in IDs are accessible.
	AllAllowed bool
	// IDs is the explicit set of accessible resource IDs.
	// Only populated when AllAllowed is false.
	IDs []string
}

// BoundaryType names where a boundary lets a caller act.
type BoundaryType string

const (
	// BoundaryTypeDeployment lets a caller act on every organization unit in the deployment.
	BoundaryTypeDeployment BoundaryType = "deployment"
	// BoundaryTypeOU confines a caller to the organization unit its token names.
	BoundaryTypeOU BoundaryType = "ou"
)

// Boundary says where a caller may act.
type Boundary struct {
	// Type is the kind of boundary.
	Type BoundaryType
	// OUID is the organization unit the caller is confined to. Empty for a deployment boundary.
	OUID string
}

// listOU decides which organization unit a list of organization unit owned resources serves, for a
// store that lists one organization unit at a time. The accessible resources must be organization
// units. requestedOUID is the organization unit the request filters by, or empty when it names none.
//
// It returns the organization unit to list, where empty means every organization unit, and false
// when the list is empty because the caller may not list the requested organization unit, or any.
// It returns errMultipleListOUs when the caller may list several organization units and the request
// names none, which such a store cannot serve in one list.
func (r *AccessibleResources) listOU(requestedOUID string) (string, bool, error) {
	if r.AllAllowed {
		return requestedOUID, true, nil
	}
	if requestedOUID != "" {
		return requestedOUID, slices.Contains(r.IDs, requestedOUID), nil
	}
	switch len(r.IDs) {
	case 0:
		return "", false, nil
	case 1:
		return r.IDs[0], true, nil
	default:
		return "", false, errMultipleListOUs
	}
}

// FilterByListOU keeps the items of the organization unit a list serves, as ResolveListOU
// decides it. An empty organization unit keeps every item, and a list the caller may not see keeps none.
func FilterByListOU[T any](items []T, listOUID string, visible bool, ouOf func(T) string) []T {
	if !visible {
		return []T{}
	}
	if listOUID == "" {
		return items
	}
	kept := make([]T, 0, len(items))
	for _, item := range items {
		if ouOf(item) == listOUID {
			kept = append(kept, item)
		}
	}
	return kept
}
