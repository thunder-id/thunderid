// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package config resolves and validates account access configuration.
package config

import (
	"github.com/thunder-id/thunderid/internal/identitygovernance/model"
	sysconfig "github.com/thunder-id/thunderid/internal/system/config"
)

// ConfigNameAccountAccess is the server-config section name for the account access policy.
const ConfigNameAccountAccess = "accountAccess"

// DefaultLockEmailRecipientAttribute names the default profile contact.
const DefaultLockEmailRecipientAttribute = "email"

// AccountAccessValue is the `accountAccess` server-config value in camelCase JSON. Every field is
// optional so a layer states only what it changes.
type AccountAccessValue struct {
	User  *AccountAccessCategory `json:"user,omitempty"`
	Agent *AccountAccessCategory `json:"agent,omitempty"`
}

// AccountAccessCategory is one entity category's policy. A nil category inherits the layer below it.
type AccountAccessCategory struct {
	// LockGranularity is "authentication_method" or "entity". Empty inherits.
	LockGranularity *string `json:"lockGranularity,omitempty"`
	// RecordLastLogin turns on the `lastLoginAt` write when access is granted. Independent of the
	// lockout policy.
	RecordLastLogin *bool `json:"recordLastLogin,omitempty"`
	// LastLoginResolutionSeconds skips the write when the stored value is newer than this. 0 writes
	// every time.
	LastLoginResolutionSeconds *int `json:"lastLoginResolutionSeconds,omitempty"`
	// Notifications configures the notices sent to the account holder. Independent of the lockout
	// policy.
	Notifications *AccountAccessNotifications `json:"notifications,omitempty"`
	// Default is the policy for every authentication method in this category.
	Default *AccountAccessPolicy `json:"default,omitempty"`
	// Scopes holds per-method overrides, overlaid on Default field by field.
	Scopes map[string]AccountAccessPolicy `json:"scopes,omitempty"`
}

// AccountAccessNotifications is the notices one category sends, keyed by event.
type AccountAccessNotifications struct {
	OnLock *AccountAccessLockNotification `json:"onLock,omitempty"`
}

// AccountAccessLockNotification is what is sent when an automatic lock forms, keyed by channel.
type AccountAccessLockNotification struct {
	Email *AccountAccessLockEmail `json:"email,omitempty"`
}

// AccountAccessLockEmail is the automatic-lock email for one category. It is sent through the
// default email sender of the `notification` section.
type AccountAccessLockEmail struct {
	// Enabled turns the email on or off. Read for each notice.
	Enabled *bool `json:"enabled,omitempty"`
	// RecipientAttribute is the profile attribute holding the address, "email" by default. A user
	// without it is skipped.
	RecipientAttribute *string `json:"recipientAttribute,omitempty"`
}

// AccountAccessPolicy is a lockout policy, used as a category default or a per-method override.
// Fields are pointers or slices so unset is distinguishable from zero. A duration of 0 means locked
// until an operator unlocks; a decay of 0 means never decay.
type AccountAccessPolicy struct {
	Enabled              *bool   `json:"enabled,omitempty"`
	Threshold            *int    `json:"threshold,omitempty"`
	FailureWindowSeconds *int    `json:"failureWindowSeconds,omitempty"`
	LockDurationsSeconds []int   `json:"lockDurationsSeconds,omitempty"`
	LockDecaySeconds     *int    `json:"lockDecaySeconds,omitempty"`
	DiscloseAccountHold  *string `json:"discloseAccountHold,omitempty"`
}

// accountAccessPolicyFrom converts a deployment policy into its section form.
func accountAccessPolicyFrom(c sysconfig.ScopeAccessConfig) AccountAccessPolicy {
	return AccountAccessPolicy{
		Enabled:              c.Enabled,
		Threshold:            c.Threshold,
		FailureWindowSeconds: c.FailureWindowSeconds,
		LockDurationsSeconds: c.LockDurationsSeconds,
		LockDecaySeconds:     c.LockDecaySeconds,
		DiscloseAccountHold:  c.DiscloseAccountHold,
	}
}

// accountAccessCategoryFrom converts a deployment category into its section form. A category that
// states nothing converts to nil, so absence survives the conversion.
func accountAccessCategoryFrom(c sysconfig.CategoryAccessConfig) *AccountAccessCategory {
	out := &AccountAccessCategory{
		RecordLastLogin:            c.RecordLastLogin,
		LastLoginResolutionSeconds: c.LastLoginResolutionSeconds,
	}
	if c.LockGranularity != "" {
		granularity := c.LockGranularity
		out.LockGranularity = &granularity
	}
	if policy := accountAccessPolicyFrom(c.Default); !policy.isEmpty() {
		out.Default = &policy
	}
	if len(c.Scopes) > 0 {
		out.Scopes = make(map[string]AccountAccessPolicy, len(c.Scopes))
		for name, scope := range c.Scopes {
			out.Scopes[name] = accountAccessPolicyFrom(scope)
		}
	}
	out.Notifications = accountAccessNotificationsFrom(c.Notifications)
	if out.LockGranularity == nil && out.Default == nil && len(out.Scopes) == 0 &&
		out.Notifications == nil && out.RecordLastLogin == nil && out.LastLoginResolutionSeconds == nil {
		return nil
	}
	return out
}

// accountAccessNotificationsFrom converts a deployment notification block, or returns nil when it
// states nothing.
func accountAccessNotificationsFrom(n sysconfig.AccessNotificationsConfig) *AccountAccessNotifications {
	email := n.OnLock.Email
	if email.Enabled == nil && email.RecipientAttribute == nil {
		return nil
	}
	return &AccountAccessNotifications{OnLock: &AccountAccessLockNotification{Email: &AccountAccessLockEmail{
		Enabled:            email.Enabled,
		RecipientAttribute: email.RecipientAttribute,
	}}}
}

// isEmpty reports whether a policy states nothing at all.
func (p AccountAccessPolicy) isEmpty() bool {
	return p.Enabled == nil && p.Threshold == nil && p.FailureWindowSeconds == nil &&
		len(p.LockDurationsSeconds) == 0 && p.LockDecaySeconds == nil &&
		p.DiscloseAccountHold == nil
}

// isEmpty reports whether a layer states nothing at all.
func (v AccountAccessValue) isEmpty() bool {
	return v.User == nil && v.Agent == nil
}

// AccountAccessValueFrom converts the deployment configuration into its section form.
func AccountAccessValueFrom(cfg sysconfig.AccountAccessConfig) AccountAccessValue {
	return AccountAccessValue{
		User:  accountAccessCategoryFrom(cfg.User),
		Agent: accountAccessCategoryFrom(cfg.Agent),
	}
}

// mergeAccountAccessValue overlays the writable layer onto the read-only one, category by category.
func mergeAccountAccessValue(readOnly, writable AccountAccessValue) AccountAccessValue {
	return AccountAccessValue{
		User:  mergeAccountAccessCategory(readOnly.User, writable.User),
		Agent: mergeAccountAccessCategory(readOnly.Agent, writable.Agent),
	}
}

// mergeAccountAccessCategory overlays one category. If either side is nil, the other is returned.
func mergeAccountAccessCategory(readOnly, writable *AccountAccessCategory) *AccountAccessCategory {
	if writable == nil {
		return readOnly
	}
	if readOnly == nil {
		return writable
	}

	merged := &AccountAccessCategory{
		LockGranularity:            readOnly.LockGranularity,
		RecordLastLogin:            readOnly.RecordLastLogin,
		LastLoginResolutionSeconds: readOnly.LastLoginResolutionSeconds,
		Default:                    overlayAccountAccessPolicy(readOnly.Default, writable.Default),
	}
	if writable.LockGranularity != nil {
		merged.LockGranularity = writable.LockGranularity
	}
	if writable.RecordLastLogin != nil {
		merged.RecordLastLogin = writable.RecordLastLogin
	}
	if writable.LastLoginResolutionSeconds != nil {
		merged.LastLoginResolutionSeconds = writable.LastLoginResolutionSeconds
	}

	// Scopes are merged as a union, so a writable layer naming one scope keeps the others.
	if len(readOnly.Scopes) > 0 || len(writable.Scopes) > 0 {
		merged.Scopes = make(map[string]AccountAccessPolicy, len(readOnly.Scopes)+len(writable.Scopes))
		for name, policy := range readOnly.Scopes {
			merged.Scopes[name] = policy
		}
		for name, policy := range writable.Scopes {
			if base, ok := merged.Scopes[name]; ok {
				merged.Scopes[name] = *overlayAccountAccessPolicy(&base, &policy)
				continue
			}
			merged.Scopes[name] = policy
		}
	}
	merged.Notifications = mergeAccountAccessNotifications(readOnly.Notifications, writable.Notifications)
	return merged
}

// mergeAccountAccessNotifications overlays one notification block onto another, field by field.
func mergeAccountAccessNotifications(readOnly, writable *AccountAccessNotifications) *AccountAccessNotifications {
	base, override := LockEmailOf(readOnly), LockEmailOf(writable)
	if override == nil {
		return readOnly
	}
	if base == nil {
		return writable
	}

	merged := &AccountAccessLockEmail{
		Enabled:            base.Enabled,
		RecipientAttribute: base.RecipientAttribute,
	}
	if override.Enabled != nil {
		merged.Enabled = override.Enabled
	}
	if override.RecipientAttribute != nil {
		merged.RecipientAttribute = override.RecipientAttribute
	}
	return &AccountAccessNotifications{OnLock: &AccountAccessLockNotification{Email: merged}}
}

// LockEmailOf returns the on-lock email block, or nil if any level is unset.
func LockEmailOf(n *AccountAccessNotifications) *AccountAccessLockEmail {
	if n == nil || n.OnLock == nil || n.OnLock.Email == nil {
		return nil
	}
	return n.OnLock.Email
}

// overlayAccountAccessPolicy overlays one policy on another, field by field. Used both for layer
// merges and for a scope over its category default.
func overlayAccountAccessPolicy(base, override *AccountAccessPolicy) *AccountAccessPolicy {
	if override == nil {
		return base
	}
	if base == nil {
		return override
	}
	merged := *base
	if override.Enabled != nil {
		merged.Enabled = override.Enabled
	}
	if override.Threshold != nil {
		merged.Threshold = override.Threshold
	}
	if override.FailureWindowSeconds != nil {
		merged.FailureWindowSeconds = override.FailureWindowSeconds
	}
	if len(override.LockDurationsSeconds) > 0 {
		merged.LockDurationsSeconds = override.LockDurationsSeconds
	}
	if override.LockDecaySeconds != nil {
		merged.LockDecaySeconds = override.LockDecaySeconds
	}
	if override.DiscloseAccountHold != nil {
		merged.DiscloseAccountHold = override.DiscloseAccountHold
	}
	return &merged
}

// unknownScopeNames returns the scope keys in the category that are not registered access scopes.
func unknownScopeNames(category *AccountAccessCategory) []string {
	if category == nil {
		return nil
	}
	var unknown []string
	for name := range category.Scopes {
		if !model.IsAccessScope(model.AccessScope(name)) {
			unknown = append(unknown, name)
		}
	}
	return unknown
}
