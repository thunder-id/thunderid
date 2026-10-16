// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"context"
	"time"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// policyResolverInterface resolves the lockout and activity policies. When the configuration cannot
// be read, the deployment policy applies.
type policyResolverInterface interface {
	// Resolve returns the lockout policy for an entity and authentication method.
	Resolve(ctx context.Context, entity GovernedEntity, scope providers.AccessScope) lockoutPolicy
	// ActivityPolicies returns the login-recording policy for every category.
	ActivityPolicies(ctx context.Context) activityPolicies
}

// policyResolver resolves policies from the accountAccess section.
type policyResolver struct {
	source accountAccessSourceInterface
}

// newPolicyResolver returns a resolver over the given section source.
func newPolicyResolver(source accountAccessSourceInterface) policyResolverInterface {
	return &policyResolver{source: source}
}

// Resolve returns the policy for the entity's category and the given scope. Under entity granularity
// the entity scope's policy applies, not the presented method's.
func (r *policyResolver) Resolve(ctx context.Context, entity GovernedEntity,
	scope providers.AccessScope) lockoutPolicy {
	category := categoryConfig(r.source.Current(ctx), entity.Category)
	return policyFor(category, parseGranularity(derefOr(category.LockGranularity, "")), scope)
}

// ActivityPolicies resolves every category from one configuration read.
func (r *policyResolver) ActivityPolicies(ctx context.Context) activityPolicies {
	accountAccess := r.source.Current(ctx)
	return activityPolicies{
		user:  activityPolicyFor(categoryConfig(accountAccess, providers.EntityCategoryUser)),
		agent: activityPolicyFor(categoryConfig(accountAccess, providers.EntityCategoryAgent)),
	}
}

// effectiveScopeFor returns the access-state key a scope maps to under the given granularity.
func effectiveScopeFor(g granularity, scope providers.AccessScope) providers.AccessScope {
	if g == granularityEntity {
		return AccessScopeEntity
	}
	return scope
}

// toDuration converts seconds to a duration, clamped to [0, maxDurationSeconds].
func toDuration(seconds int) time.Duration {
	return time.Duration(min(max(seconds, 0), maxDurationSeconds)) * time.Second
}

// activityPolicyFor reads one category's activity settings. Recording is off unless enabled.
func activityPolicyFor(category accountAccessCategory) activityPolicy {
	return activityPolicy{
		record: derefOr(category.RecordLastLogin, false),
		// A negative resolution clamps to zero (write every time).
		resolution: toDuration(derefOr(category.LastLoginResolutionSeconds,
			defaultLastLoginResolutionSeconds)),
	}
}

// policyFor builds the policy a category applies at a scope.
func policyFor(category accountAccessCategory, g granularity,
	scope providers.AccessScope) lockoutPolicy {
	effective := effectiveScopeFor(g, scope)

	// A scope block overlays the category default; an unnamed method gets the default.
	var merged accountAccessPolicy
	if scopePolicy, ok := category.Scopes[string(effective)]; ok {
		merged = *overlayAccountAccessPolicy(category.Default, &scopePolicy)
	} else if category.Default != nil {
		merged = *category.Default
	}

	policy := lockoutPolicy{
		enabled:             derefOr(merged.Enabled, true),
		threshold:           derefOr(merged.Threshold, 0),
		failureWindow:       toDuration(derefOr(merged.FailureWindowSeconds, 0)),
		lockDurations:       toDurations(merged.LockDurationsSeconds),
		lockDecay:           toDuration(derefOr(merged.LockDecaySeconds, 0)),
		granularity:         g,
		discloseAccountHold: defaultDisclosureMode,
	}

	// A policy that cannot form a lock (no threshold, durations or window) is disabled.
	if policy.threshold <= 0 || len(policy.lockDurations) == 0 || merged.FailureWindowSeconds == nil {
		policy.enabled = false
	}

	return policy
}

// derefOr returns *value, or fallback if nil.
func derefOr[T any](value *T, fallback T) T {
	if value == nil {
		return fallback
	}
	return *value
}

// categoryConfig returns the configuration block for an entity category, or an empty block (a
// disabled policy) if none is set.
func categoryConfig(cfg accountAccessValue,
	category providers.EntityCategory) accountAccessCategory {
	var block *accountAccessCategory
	switch category {
	case providers.EntityCategoryUser:
		block = cfg.User
	case providers.EntityCategoryAgent:
		block = cfg.Agent
	}
	if block == nil {
		return accountAccessCategory{}
	}
	return *block
}

// parseGranularity parses the granularity. Unknown values default to per-method.
func parseGranularity(value string) granularity {
	if granularity(value) == granularityEntity {
		return granularityEntity
	}
	return granularityAuthenticationMethod
}

// toDurations converts seconds to durations, keeping zero (permanent lock) and dropping negatives.
func toDurations(seconds []int) []time.Duration {
	if len(seconds) == 0 {
		return nil
	}
	durations := make([]time.Duration, 0, len(seconds))
	for _, s := range seconds {
		if s < 0 {
			continue
		}
		durations = append(durations, toDuration(s))
	}
	return durations
}

// lockEnforcementFor resolves which of a category's recorded locks are enforced.
func lockEnforcementFor(cfg accountAccessValue,
	category providers.EntityCategory) lockEnforcement {
	block := categoryConfig(cfg, category)
	g := parseGranularity(derefOr(block.LockGranularity, ""))

	unenforced := make(map[providers.AccessScope]struct{}, len(AllAccessScopes))
	for _, scope := range AllAccessScopes {
		// Under entity granularity only the entity entry is enforced. The post-suspension hold is
		// exempt from this in reportFor.
		if (g == granularityEntity && scope != AccessScopeEntity) || !policyFor(block, g, scope).enabled {
			unenforced[scope] = struct{}{}
		}
	}
	return lockEnforcement{unenforced: unenforced}
}
