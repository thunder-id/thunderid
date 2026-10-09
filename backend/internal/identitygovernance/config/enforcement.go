// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"

	"github.com/thunder-id/thunderid/internal/identitygovernance/model"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// LockEnforcement says which of a category's recorded locks are still enforced. A method lock whose
// policy is switched off is not. The zero value enforces every lock.
type LockEnforcement struct {
	// unenforced is the set of method scopes whose policy is switched off.
	unenforced map[model.AccessScope]struct{}
}

// Enforces reports whether a recorded lock on this scope is enforced.
func (e LockEnforcement) Enforces(scope model.AccessScope) bool {
	if e.unenforced == nil {
		return true
	}
	_, off := e.unenforced[scope]
	return !off
}

// NewLockEnforcement returns a LockEnforcement that skips the given scopes.
func NewLockEnforcement(unenforced map[model.AccessScope]struct{}) LockEnforcement {
	return LockEnforcement{unenforced: unenforced}
}

// LockEnforcementFor resolves which of a category's recorded locks are enforced. It reads the
// configuration, so resolve it once per list. Before initialization every lock is enforced.
func LockEnforcementFor(ctx context.Context, category providers.EntityCategory) LockEnforcement {
	source := activeSource.Load()
	if source == nil {
		return LockEnforcement{}
	}

	cfg := categoryConfig(source.Current(ctx), category)
	granularity := parseGranularity(derefString(cfg.LockGranularity))

	// Under entity granularity only the entity entry is enforced.
	if granularity == GranularityEntity {
		return NewLockEnforcement(allMethodScopes())
	}

	var unenforced map[model.AccessScope]struct{}
	for _, scope := range model.AllAccessScopes {
		if scope == model.AccessScopeEntity {
			// The entity entry is always enforced; unsuspend writes one even with locking off.
			continue
		}
		if policyFor(cfg, granularity, scope).Enabled {
			continue
		}
		if unenforced == nil {
			unenforced = make(map[model.AccessScope]struct{}, len(model.AllAccessScopes))
		}
		unenforced[scope] = struct{}{}
	}
	return NewLockEnforcement(unenforced)
}

// allMethodScopes returns every registered scope except the entity scope.
func allMethodScopes() map[model.AccessScope]struct{} {
	scopes := make(map[model.AccessScope]struct{}, len(model.AllAccessScopes))
	for _, scope := range model.AllAccessScopes {
		if scope != model.AccessScopeEntity {
			scopes[scope] = struct{}{}
		}
	}
	return scopes
}
