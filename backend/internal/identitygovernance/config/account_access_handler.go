// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	sysconfig "github.com/thunder-id/thunderid/internal/system/config"
)

// errDeclarativeAccountAccessLocked is returned when a write targets a declaratively set section.
var errDeclarativeAccountAccessLocked = errors.New(
	"accountAccess is set declaratively and cannot be overridden")

// AccountAccessHandler decodes, validates and merges the accountAccess section. The deployment
// configuration (default.json and deployment.yaml) is the base. A declarative document, when one
// exists, is laid over the base and locks the section; otherwise the writable layer is.
type AccountAccessHandler struct {
	base AccountAccessValue
}

// NewAccountAccessHandler builds the handler over the deployment configuration.
func NewAccountAccessHandler(base sysconfig.AccountAccessConfig) AccountAccessHandler {
	return AccountAccessHandler{base: AccountAccessValueFrom(base)}
}

// Decode parses a raw section value into AccountAccessValue. Empty input yields the zero value (an
// absent layer). Unknown fields are rejected so a misspelled key does not silently drop protection.
func (AccountAccessHandler) Decode(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return AccountAccessValue{}, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var cfg AccountAccessValue
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate refuses a write while a declarative document exists, then checks the incoming value on
// its own and laid over the base. The merged check refuses an enabled policy that cannot form a lock.
// A nil readOnly means this is the declarative load.
func (h AccountAccessHandler) Validate(incoming, readOnly, _ any) error {
	if ro, ok := readOnly.(AccountAccessValue); ok && !ro.isEmpty() {
		return errDeclarativeAccountAccessLocked
	}
	cfg, ok := incoming.(AccountAccessValue)
	if !ok {
		return fmt.Errorf("%s: unexpected config type %T", ConfigNameAccountAccess, incoming)
	}
	if err := validateStructure(cfg); err != nil {
		return err
	}
	return validateCoherence(h.effective(cfg))
}

// Merge lays the declarative document over the base when one exists, otherwise the writable layer.
func (h AccountAccessHandler) Merge(readOnly, writable any) any {
	if ro, ok := readOnly.(AccountAccessValue); ok && !ro.isEmpty() {
		return h.effective(ro)
	}
	wr, _ := writable.(AccountAccessValue)
	return h.effective(wr)
}

// Base returns the deployment configuration alone, as the effective value.
func (h AccountAccessHandler) Base() AccountAccessValue {
	return h.effective(AccountAccessValue{})
}

// effective lays a layer over the base and defaults the recipient attribute.
func (h AccountAccessHandler) effective(layer AccountAccessValue) AccountAccessValue {
	merged := mergeAccountAccessValue(h.base, layer)
	if merged.User == nil {
		return merged
	}
	email := LockEmailOf(merged.User.Notifications)
	if email == nil {
		return merged
	}
	if email.RecipientAttribute != nil && strings.TrimSpace(*email.RecipientAttribute) != "" {
		return merged
	}
	// Copy before defaulting so stored layers remain unchanged.
	user := *merged.User
	effectiveEmail := *email
	attribute := DefaultLockEmailRecipientAttribute
	effectiveEmail.RecipientAttribute = &attribute
	user.Notifications = &AccountAccessNotifications{OnLock: &AccountAccessLockNotification{Email: &effectiveEmail}}
	merged.User = &user
	return merged
}

// validateStructure checks a value's fields without regard to other layers.
func validateStructure(cfg AccountAccessValue) error {
	categories := []struct {
		name  string
		value *AccountAccessCategory
	}{
		{"user", cfg.User},
		{"agent", cfg.Agent},
	}

	for _, category := range categories {
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
		if unknown := unknownScopeNames(category.value); len(unknown) > 0 {
			sort.Strings(unknown)
			return fmt.Errorf("%s: %s.scopes names unknown sign-in method(s) %s",
				ConfigNameAccountAccess, category.name, strings.Join(unknown, ", "))
		}
		if err := validatePolicyStructure(category.name+".default", category.value.Default); err != nil {
			return err
		}
		for _, scope := range sortedScopeNames(category.value.Scopes) {
			policy := category.value.Scopes[scope]
			if err := validatePolicyStructure(category.name+".scopes."+scope, &policy); err != nil {
				return err
			}
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
		return fmt.Errorf("%s: %s must be >= 0", ConfigNameAccountAccess, path)
	}
	if value > maxDurationSeconds {
		return fmt.Errorf("%s: %s must be <= %d", ConfigNameAccountAccess, path, maxDurationSeconds)
	}
	return nil
}

// validateGranularity rejects an unknown granularity, which the resolver would read as the default.
func validateGranularity(category string, value *string) error {
	if value == nil {
		return nil
	}
	switch Granularity(*value) {
	case GranularityAuthenticationMethod, GranularityEntity:
		return nil
	default:
		return fmt.Errorf("%s: %s.lockGranularity must be %q or %q",
			ConfigNameAccountAccess, category, GranularityAuthenticationMethod, GranularityEntity)
	}
}

// validatePolicyStructure checks one policy's fields in isolation.
func validatePolicyStructure(path string, policy *AccountAccessPolicy) error {
	if policy == nil {
		return nil
	}
	if policy.Threshold != nil && *policy.Threshold < 0 {
		return fmt.Errorf("%s: %s.threshold must be >= 0", ConfigNameAccountAccess, path)
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
		// Negative durations are rejected. Zero means locked until an operator unlocks.
		if err := validateSeconds(fmt.Sprintf("%s.lockDurationsSeconds[%d]", path, i), duration); err != nil {
			return err
		}
	}
	if policy.DiscloseAccountHold != nil {
		switch DisclosureMode(*policy.DiscloseAccountHold) {
		case DiscloseNever, DiscloseAlways:
		default:
			return fmt.Errorf("%s: %s.discloseAccountHold must be %q or %q",
				ConfigNameAccountAccess, path, DiscloseNever, DiscloseAlways)
		}
	}
	return nil
}

// validateCoherence rejects a merged policy that claims to be on and cannot form a lock.
func validateCoherence(merged AccountAccessValue) error {
	categories := []struct {
		name  string
		value *AccountAccessCategory
	}{
		{"user", merged.User},
		{"agent", merged.Agent},
	}

	for _, category := range categories {
		if category.value == nil {
			continue
		}
		base := category.value.Default
		if err := validateMergedPolicy(category.name+".default", base); err != nil {
			return err
		}
		// Each scope override is checked after overlaying it on the category default.
		for _, scope := range sortedScopeNames(category.value.Scopes) {
			policy := category.value.Scopes[scope]
			overlaid := overlayAccountAccessPolicy(base, &policy)
			if err := validateMergedPolicy(category.name+".scopes."+scope, overlaid); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateMergedPolicy applies the resolver's own enabled test to a fully overlaid policy.
func validateMergedPolicy(path string, policy *AccountAccessPolicy) error {
	// A nil, disabled or empty policy is not checked; the resolver treats it as off.
	if policy == nil || !derefBool(policy.Enabled, true) {
		return nil
	}
	if policy.Threshold == nil && policy.FailureWindowSeconds == nil &&
		len(policy.LockDurationsSeconds) == 0 && policy.LockDecaySeconds == nil {
		return nil
	}

	var missing []string
	if policy.Threshold == nil || *policy.Threshold <= 0 {
		missing = append(missing, "threshold")
	}
	if policy.FailureWindowSeconds == nil || *policy.FailureWindowSeconds <= 0 {
		missing = append(missing, "failureWindowSeconds")
	}
	if len(policy.LockDurationsSeconds) == 0 {
		missing = append(missing, "lockDurationsSeconds")
	}
	if len(missing) > 0 {
		return fmt.Errorf(
			"%s: %s is enabled but cannot form a lock; %s must be set, or set enabled to false",
			ConfigNameAccountAccess, path, strings.Join(missing, ", "))
	}
	return nil
}

// sortedScopeNames returns the scope keys sorted, so validation errors are deterministic.
func sortedScopeNames(scopes map[string]AccountAccessPolicy) []string {
	names := make([]string, 0, len(scopes))
	for name := range scopes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
