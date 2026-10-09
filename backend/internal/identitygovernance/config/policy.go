// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"math"
	"time"

	"github.com/thunder-id/thunderid/internal/identitygovernance/model"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Granularity is what a lock holds: the authentication method that failed, or the whole entity.
type Granularity string

const (
	// GranularityAuthenticationMethod locks only the method that failed; other methods keep working.
	// The default.
	GranularityAuthenticationMethod Granularity = "authentication_method"

	// GranularityEntity counts every lockable failure under the entity scope, and a lock holds every
	// method.
	GranularityEntity Granularity = "entity"
)

// ActivityPolicy is the resolved activity-recording configuration for one entity category.
type ActivityPolicy struct {
	// Record says whether lastLoginAt is written when access is granted.
	Record bool
	// Resolution is how old the stored value may be before it is rewritten. Zero writes every time.
	Resolution time.Duration
}

// LockoutPolicy is the resolved lockout configuration for one category and method scope.
type LockoutPolicy struct {
	// Enabled turns automatic locking on or off.
	Enabled bool
	// Threshold is the number of failures within FailureWindow that forms a lock.
	Threshold int
	// FailureWindow is how long failures are counted before the counter resets.
	FailureWindow time.Duration
	// LockDurations is the lock duration per consecutive lockout; the last one repeats. Zero means
	// locked until an operator unlocks.
	LockDurations []time.Duration
	// LockDecay is how long after a lock ends the consecutive lockout count resets.
	LockDecay time.Duration
	// Granularity is what a lock under this policy holds.
	Granularity Granularity
	// DiscloseAccountHold says whether a refusal may state that the account is locked or suspended.
	// Defaults to never, so a refusal does not confirm the username exists.
	DiscloseAccountHold DisclosureMode
}

// DisclosureMode is whether a refusal may state the account's hold.
type DisclosureMode string

const (
	// DiscloseNever answers a held account the same as a wrong secret. The default.
	DiscloseNever DisclosureMode = "never"

	// DiscloseAlways lets the refusal say the account is locked or suspended.
	DiscloseAlways DisclosureMode = "always"
)

// EffectiveScopeFor returns the accessState key a scope maps to under the given granularity.
func EffectiveScopeFor(granularity Granularity, scope model.AccessScope) model.AccessScope {
	if granularity == GranularityEntity {
		return model.AccessScopeEntity
	}
	return scope
}

// policyResolver resolves lockout and activity policies from the account access section source.
type policyResolver struct {
	section *AccountAccessSource
}

// NewPolicyResolver returns a resolver over the given section source.
func NewPolicyResolver(section *AccountAccessSource) *policyResolver {
	return &policyResolver{section: section}
}

// Resolve returns the policy for the entity's category and the given scope.
func (r *policyResolver) Resolve(ctx context.Context, entity model.GovernedEntity,
	scope model.AccessScope) LockoutPolicy {
	category := categoryConfig(r.section.Current(ctx), entity.Category)
	granularity := parseGranularity(derefString(category.LockGranularity))

	// Under entity granularity the entity scope's policy applies, not the presented method's.
	return policyFor(category, granularity, scope)
}

// ActivityPolicies resolves all categories from one configuration read.
func (r *policyResolver) ActivityPolicies(ctx context.Context) ActivityPolicies {
	accountAccess := r.section.Current(ctx)
	return ActivityPolicies{
		User:  activityPolicyFor(categoryConfig(accountAccess, providers.EntityCategoryUser)),
		Agent: activityPolicyFor(categoryConfig(accountAccess, providers.EntityCategoryAgent)),
	}
}

// ActivityPolicies holds the activity settings for each category.
type ActivityPolicies struct {
	User, Agent ActivityPolicy
}

// Enabled reports whether any category records activity.
func (p ActivityPolicies) Enabled() bool {
	return p.User.Record || p.Agent.Record
}

// For returns the settings for an entity category.
func (p ActivityPolicies) For(category providers.EntityCategory) ActivityPolicy {
	switch category {
	case providers.EntityCategoryUser:
		return p.User
	case providers.EntityCategoryAgent:
		return p.Agent
	default:
		return ActivityPolicy{}
	}
}

// maxDurationSeconds is the largest seconds value that converts to a time.Duration without
// overflowing. Writes above it are refused (validateSeconds); conversion clamps.
const maxDurationSeconds = int(math.MaxInt64 / time.Second)

// toDuration converts seconds to a duration, clamped to [0, maxDurationSeconds].
func toDuration(seconds int) time.Duration {
	return time.Duration(min(max(seconds, 0), maxDurationSeconds)) * time.Second
}

// activityPolicyFor reads one category's activity settings. Recording is off unless enabled.
func activityPolicyFor(category AccountAccessCategory) ActivityPolicy {
	return ActivityPolicy{
		Record: derefBool(category.RecordLastLogin, false),
		// A negative resolution clamps to zero (write every time).
		Resolution: toDuration(derefInt(category.LastLoginResolutionSeconds,
			defaultLastLoginResolutionSeconds)),
	}
}

// defaultLastLoginResolutionSeconds is the resolution used when recording is on and none is set.
const defaultLastLoginResolutionSeconds = 3600

// policyFor builds the policy a category applies at a scope. Also used by LockEnforcementFor.
func policyFor(category AccountAccessCategory, granularity Granularity,
	scope model.AccessScope) LockoutPolicy {
	effective := EffectiveScopeFor(granularity, scope)

	// A scope block overlays the category default; an unnamed method gets the default.
	var merged AccountAccessPolicy
	if scopePolicy, ok := category.Scopes[string(effective)]; ok {
		merged = *overlayAccountAccessPolicy(category.Default, &scopePolicy)
	} else if category.Default != nil {
		merged = *category.Default
	}

	policy := LockoutPolicy{
		Enabled:       derefBool(merged.Enabled, true),
		Threshold:     derefInt(merged.Threshold, 0),
		FailureWindow: toDuration(derefInt(merged.FailureWindowSeconds, 0)),
		LockDurations: toDurations(merged.LockDurationsSeconds),
		LockDecay:     toDuration(derefInt(merged.LockDecaySeconds, 0)),
		Granularity:   granularity,
		// From configuration, never from the request.
		DiscloseAccountHold: parseDisclosureMode(derefString(merged.DiscloseAccountHold)),
	}

	// A policy that cannot form a lock (no threshold, durations or window) is disabled.
	if policy.Threshold <= 0 || len(policy.LockDurations) == 0 || policy.FailureWindow <= 0 {
		policy.Enabled = false
	}

	return policy
}

// derefBool returns *value, or fallback if nil.
func derefBool(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

// derefString returns *value, or "" if nil.
func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// derefInt returns *value, or fallback if nil.
func derefInt(value *int, fallback int) int {
	if value == nil {
		return fallback
	}
	return *value
}

// categoryConfig returns the configuration block for an entity category, or an empty block (a
// disabled policy) if none is set.
func categoryConfig(cfg AccountAccessValue, category providers.EntityCategory) AccountAccessCategory {
	var block *AccountAccessCategory
	switch category {
	case providers.EntityCategoryUser:
		block = cfg.User
	case providers.EntityCategoryAgent:
		block = cfg.Agent
	}
	if block == nil {
		return AccountAccessCategory{}
	}
	return *block
}

// parseGranularity parses the granularity. Unknown values default to per-method.
func parseGranularity(value string) Granularity {
	if Granularity(value) == GranularityEntity {
		return GranularityEntity
	}
	return GranularityAuthenticationMethod
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

// parseDisclosureMode parses the disclosure mode. Unknown values default to never.
func parseDisclosureMode(value string) DisclosureMode {
	if DisclosureMode(value) == DiscloseAlways {
		return DiscloseAlways
	}
	return DiscloseNever
}
