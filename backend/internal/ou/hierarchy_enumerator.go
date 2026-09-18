// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/log"
)

const loggerComponentNameHierarchyEnumerator = "OUHierarchyEnumerator"

// hierarchyPageSize bounds each page read by the walks below, which loop until a short page ends
// them. The store exposes only paginated listings, so a walk cannot ask for a whole level at once.
const hierarchyPageSize = 100

// HierarchyEnumerator walks the organization unit tree downwards.
//
// This is a separate capability from sysauthz.OUHierarchyResolver, which answers questions about a
// single unit's ancestry to decide access. Enumerating a subtree is not an access decision: it
// answers which units a change reaches, and it is deliberately not exposed on the same type.
type HierarchyEnumerator interface {
	// DescendantOUIDs returns every organization unit beneath ouID, at any depth.
	DescendantOUIDs(ctx context.Context, ouID string) ([]string, *tidcommon.ServiceError)
	// AllOUIDs returns every organization unit in the deployment, every tree included.
	AllOUIDs(ctx context.Context) ([]string, *tidcommon.ServiceError)
}

// ouHierarchyEnumerator implements HierarchyEnumerator using direct store access, bypassing the
// service layer for the same reason the resolver does: a traversal must not re-enter authorization.
type ouHierarchyEnumerator struct {
	store organizationUnitStoreInterface
}

// newOUHierarchyEnumerator returns a HierarchyEnumerator backed by the given store.
func newOUHierarchyEnumerator(store organizationUnitStoreInterface) HierarchyEnumerator {
	return &ouHierarchyEnumerator{store: store}
}

// DescendantOUIDs returns every organization unit beneath ouID, at any depth.
//
// This is the downward counterpart to GetAncestorOUIDs, needed wherever a decision applies to a
// whole subtree rather than to one named unit.
func (r *ouHierarchyEnumerator) DescendantOUIDs(
	ctx context.Context, ouID string,
) ([]string, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameHierarchyEnumerator))

	if ouID == "" {
		return []string{}, nil
	}

	var out []string
	queue := []string{ouID}
	visited := map[string]struct{}{ouID: {}}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		children, svcErr := r.childOUIDs(ctx, current)
		if svcErr != nil {
			return nil, svcErr
		}
		for _, child := range children {
			if _, seen := visited[child]; seen {
				logger.Error(ctx, "Cyclic organization unit parent chain detected while collecting descendants",
					log.String("ouID", child))
				return nil, &tidcommon.InternalServerError
			}
			visited[child] = struct{}{}
			out = append(out, child)
			queue = append(queue, child)
		}
	}

	if out == nil {
		out = []string{}
	}
	return out, nil
}

// AllOUIDs returns every organization unit in the deployment, every tree included.
func (r *ouHierarchyEnumerator) AllOUIDs(ctx context.Context) ([]string, *tidcommon.ServiceError) {
	roots, svcErr := r.rootOUIDs(ctx)
	if svcErr != nil {
		return nil, svcErr
	}

	out := make([]string, 0, len(roots))
	for _, root := range roots {
		out = append(out, root)
		descendants, svcErr := r.DescendantOUIDs(ctx, root)
		if svcErr != nil {
			return nil, svcErr
		}
		out = append(out, descendants...)
	}
	return out, nil
}

// childOUIDs pages through the direct children of one organization unit.
func (r *ouHierarchyEnumerator) childOUIDs(
	ctx context.Context, ouID string,
) ([]string, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameHierarchyEnumerator))

	var out []string
	for offset := 0; ; offset += hierarchyPageSize {
		page, err := r.store.GetOrganizationUnitChildrenList(ctx, ouID, hierarchyPageSize, offset, nil)
		if err != nil {
			logger.Error(ctx, "Failed to list child organization units", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		for _, child := range page {
			out = append(out, child.ID)
		}
		if len(page) < hierarchyPageSize {
			return out, nil
		}
	}
}

// rootOUIDs pages through the organization units that have no parent.
func (r *ouHierarchyEnumerator) rootOUIDs(ctx context.Context) ([]string, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameHierarchyEnumerator))

	var out []string
	for offset := 0; ; offset += hierarchyPageSize {
		page, err := r.store.GetOrganizationUnitList(ctx, hierarchyPageSize, offset, nil)
		if err != nil {
			logger.Error(ctx, "Failed to list root organization units", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		for _, root := range page {
			out = append(out, root.ID)
		}
		if len(page) < hierarchyPageSize {
			return out, nil
		}
	}
}
