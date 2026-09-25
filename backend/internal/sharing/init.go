// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
)

// Initialize wires the sharing service. It exposes no routes of its own: each onboarded resource
// type mounts the shared surface for its own collection.
//
// ouHierarchyResolver and ouEnumerator are the instances organization unit initialization already
// produces, so this package needs no direct dependency on the organization unit service for tree
// traversal. They are separate arguments because they answer separate questions: the resolver
// decides what one unit may reach, the enumerator reports which units a change reaches.
func Initialize(
	cacheManager cache.CacheManagerInterface,
	ouHierarchyResolver sysauthz.OUHierarchyResolver,
	ouEnumerator OUEnumerator,
	allowChildOUCrossTreeSharing bool,
) (ServiceInterface, error) {
	dbStore, transactioner, err := newSharingStore()
	if err != nil {
		return nil, err
	}

	// Policies declared by resource files are held in memory alongside the stored ones, so they
	// live and die with the file that declares them, exactly as the resource itself does.
	//
	// The store is always created rather than gated on a mode. Whether a resource type loads from
	// files is that type's own setting, and a resource server configured for declarative storage
	// declares policies even when the deployment-wide declarative flag is off. Guessing here made
	// the two settings disagree and turned a declared policy into a startup failure. Empty until
	// something declares into it, so there is nothing to save by withholding it.
	declarativeStore := newDeclarativePolicyStore()

	var visibilityCache cache.CacheInterface[bool]
	var overlayRuleCache cache.CacheInterface[ResolvedOverlay]
	if cacheManager != nil {
		visibilityCache = cache.GetCache[bool](cacheManager, "SharingVisibilityCache")
		overlayRuleCache = cache.GetCache[ResolvedOverlay](cacheManager, "SharingOverlayCache")
	}

	return newService(dbStore, declarativeStore, ouHierarchyResolver, ouEnumerator, transactioner,
		visibilityCache, overlayRuleCache, allowChildOUCrossTreeSharing), nil
}
