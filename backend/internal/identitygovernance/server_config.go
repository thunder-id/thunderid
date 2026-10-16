// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/serverconfig"
)

// accountAccessHandler decodes, validates and merges the accountAccess section. The deployment
// configuration is the base. A declarative document, when one exists, is laid over the base and
// locks the section; otherwise the writable layer is.
type accountAccessHandler struct {
	base accountAccessValue
}

// NewAccountAccessConfigHandler builds the accountAccess server-config handler. It is registered
// before the server-config service exists, so it is built apart from Initialize.
func NewAccountAccessConfigHandler(cfg Config) serverconfig.ServerConfigHandlerInterface {
	return accountAccessHandler{base: cfg.AccountAccess}
}

// Decode parses a raw section value. Empty input yields the zero value (an absent layer). Unknown
// fields are rejected so a misspelled key does not silently drop protection.
func (h accountAccessHandler) Decode(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return accountAccessValue{}, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var cfg accountAccessValue
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate refuses a write while a declarative document exists, then checks the incoming value on
// its own and laid over the base. A nil readOnly means this is the declarative load.
func (h accountAccessHandler) Validate(incoming, readOnly, _ any) error {
	if ro, ok := readOnly.(accountAccessValue); ok && !ro.IsEmpty() {
		return errDeclarativeAccountAccessLocked
	}
	cfg, ok := incoming.(accountAccessValue)
	if !ok {
		return fmt.Errorf("%s: unexpected config type %T", serverconfig.ConfigNameAccountAccess, incoming)
	}
	if err := validateStructure(cfg); err != nil {
		return err
	}
	return validateCoherence(effectiveAccountAccess(h.base, cfg))
}

// Merge lays the declarative document over the base when one exists, otherwise the writable layer.
func (h accountAccessHandler) Merge(readOnly, writable any) any {
	if ro, ok := readOnly.(accountAccessValue); ok && !ro.IsEmpty() {
		return effectiveAccountAccess(h.base, ro)
	}
	wr, _ := writable.(accountAccessValue)
	return effectiveAccountAccess(h.base, wr)
}

// effectiveAccountAccess lays a layer over the base and defaults the lock email recipient attribute.
// This is the only place the default is applied.
func effectiveAccountAccess(base, layer accountAccessValue) accountAccessValue {
	merged := mergeAccountAccessValue(base, layer)
	if merged.User == nil {
		return merged
	}
	email := merged.User.Notifications.LockEmail()
	if email == nil || strings.TrimSpace(derefOr(email.RecipientAttribute, "")) != "" {
		return merged
	}
	// Copy before defaulting so stored layers remain unchanged.
	user := *merged.User
	effectiveEmail := *email
	attribute := defaultLockEmailRecipientAttribute
	effectiveEmail.RecipientAttribute = &attribute
	user.Notifications = &accountAccessNotifications{
		OnLock: &accountAccessLockNotification{Email: &effectiveEmail},
	}
	merged.User = &user
	return merged
}

// categoriesOf lists a section's categories in a fixed order.
func categoriesOf(cfg accountAccessValue) []namedCategory {
	return []namedCategory{{"user", cfg.User}, {"agent", cfg.Agent}}
}

// validateStructure checks a value's fields without regard to other layers.
func validateStructure(cfg accountAccessValue) error {
	for _, category := range categoriesOf(cfg) {
		if category.value == nil {
			continue
		}
		if err := validateGranularity(category.name, category.value.LockGranularity); err != nil {
			return err
		}
		if err := validateLastLoginResolution(category.name,
			category.value.LastLoginResolutionSeconds); err != nil {
			return err
		}
		if err := validatePolicyStructure(category.name+".default", category.value.Default); err != nil {
			return err
		}
	}
	return nil
}

// validateLastLoginResolution checks the last-login resolution is within range.
func validateLastLoginResolution(category string, value *int) error {
	if value == nil {
		return nil
	}
	return validateSeconds(category+".lastLoginResolutionSeconds", *value)
}

// validateSeconds rejects a negative seconds value or one above maxDurationSeconds. The resolver
// clamps such values; a write is refused instead.
func validateSeconds(path string, value int) error {
	if value < 0 {
		return fmt.Errorf("%s: %s must be >= 0", serverconfig.ConfigNameAccountAccess, path)
	}
	if value > maxDurationSeconds {
		return fmt.Errorf("%s: %s must be <= %d", serverconfig.ConfigNameAccountAccess, path, maxDurationSeconds)
	}
	return nil
}

// validateGranularity rejects an unknown granularity, which the resolver would read as the default.
func validateGranularity(category string, value *string) error {
	if value == nil {
		return nil
	}
	switch granularity(*value) {
	case granularityAuthenticationMethod, granularityEntity:
		return nil
	default:
		return fmt.Errorf("%s: %s.lockGranularity must be %q or %q",
			serverconfig.ConfigNameAccountAccess, category, granularityAuthenticationMethod, granularityEntity)
	}
}

// validatePolicyStructure checks one policy's fields in isolation.
func validatePolicyStructure(path string, policy *accountAccessPolicy) error {
	if policy == nil {
		return nil
	}
	if policy.Threshold != nil && *policy.Threshold < 0 {
		return fmt.Errorf("%s: %s.threshold must be >= 0", serverconfig.ConfigNameAccountAccess, path)
	}
	if policy.FailureWindowSeconds != nil {
		if err := validateSeconds(path+".failureWindowSeconds", *policy.FailureWindowSeconds); err != nil {
			return err
		}
	}
	if policy.LockDecaySeconds != nil {
		if err := validateSeconds(path+".lockDecaySeconds", *policy.LockDecaySeconds); err != nil {
			return err
		}
	}
	for i, duration := range policy.LockDurationsSeconds {
		// Zero means locked until an operator unlocks.
		if err := validateSeconds(fmt.Sprintf("%s.lockDurationsSeconds[%d]", path, i), duration); err != nil {
			return err
		}
	}
	return nil
}

// validateCoherence rejects a merged policy that claims to be on and cannot form a lock.
func validateCoherence(merged accountAccessValue) error {
	for _, category := range categoriesOf(merged) {
		if category.value == nil {
			continue
		}
		if err := validateMergedPolicy(category.name+".default", category.value.Default); err != nil {
			return err
		}
	}
	return nil
}

// validateMergedPolicy rejects an enabled policy that cannot form a lock. A policy is enabled when it
// says so, or when it states any lock field without saying it is off. A policy that states nothing
// configures no lock and is not checked.
func validateMergedPolicy(path string, policy *accountAccessPolicy) error {
	if policy == nil || !derefOr(policy.Enabled, true) {
		return nil
	}
	statesLock := policy.Threshold != nil || policy.FailureWindowSeconds != nil ||
		len(policy.LockDurationsSeconds) > 0 || policy.LockDecaySeconds != nil
	if policy.Enabled == nil && !statesLock {
		return nil
	}

	var missing []string
	if policy.Threshold == nil || *policy.Threshold <= 0 {
		missing = append(missing, "threshold")
	}
	if policy.FailureWindowSeconds == nil {
		missing = append(missing, "failureWindowSeconds")
	}
	if len(policy.LockDurationsSeconds) == 0 {
		missing = append(missing, "lockDurationsSeconds")
	}
	if len(missing) > 0 {
		return fmt.Errorf(
			"%s: %s is enabled but cannot form a lock; %s must be set, or set enabled to false",
			serverconfig.ConfigNameAccountAccess, path, strings.Join(missing, ", "))
	}
	return nil
}

// mergeAccountAccessValue overlays the writable layer onto the read-only one, category by category.
func mergeAccountAccessValue(readOnly,
	writable accountAccessValue) accountAccessValue {
	return accountAccessValue{
		User:  mergeAccountAccessCategory(readOnly.User, writable.User),
		Agent: mergeAccountAccessCategory(readOnly.Agent, writable.Agent),
	}
}

// mergeAccountAccessCategory overlays one category. If either side is nil, the other is returned.
func mergeAccountAccessCategory(readOnly,
	writable *accountAccessCategory) *accountAccessCategory {
	if writable == nil {
		return readOnly
	}
	if readOnly == nil {
		return writable
	}

	merged := &accountAccessCategory{
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

	merged.Notifications = mergeAccountAccessNotifications(readOnly.Notifications, writable.Notifications)
	return merged
}

// mergeAccountAccessNotifications overlays one notification block onto another, field by field.
func mergeAccountAccessNotifications(readOnly,
	writable *accountAccessNotifications) *accountAccessNotifications {
	base, override := readOnly.LockEmail(), writable.LockEmail()
	if override == nil {
		return readOnly
	}
	if base == nil {
		return writable
	}

	merged := &accountAccessLockEmail{
		Enabled:            base.Enabled,
		RecipientAttribute: base.RecipientAttribute,
	}
	if override.Enabled != nil {
		merged.Enabled = override.Enabled
	}
	if override.RecipientAttribute != nil {
		merged.RecipientAttribute = override.RecipientAttribute
	}
	return &accountAccessNotifications{
		OnLock: &accountAccessLockNotification{Email: merged},
	}
}

// overlayAccountAccessPolicy overlays one policy on another, field by field. Used both for layer
// merges and for a scope over its category default.
func overlayAccountAccessPolicy(base,
	override *accountAccessPolicy) *accountAccessPolicy {
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
	return &merged
}
