// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

import (
	"context"
	"encoding/json"
	"fmt"

	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// FlowHandleValidatorFunc is a function that reports whether a flow handle names a valid flow of
// the given type. Injected after the flow management service is initialized.
type FlowHandleValidatorFunc func(
	ctx context.Context, handle string, flowType providers.FlowType) bool

// FlowConfigHandler decodes, validates, and merges the server-config "flow" section. It implements
// the serverconfig.ServerConfigHandlerInterface structurally so this package does not need to import
// serverconfig. A FlowHandleValidatorFunc may be injected after construction to enable handle
// existence checks on API PUT; declarative-load-time Validate skips these checks (validator is nil).
type FlowConfigHandler struct {
	validator FlowHandleValidatorFunc
}

// NewFlowConfigHandler creates a FlowConfigHandler with no handle validator. Call SetHandleValidator
// after the flow management service is initialized to enable handle-existence validation on API PUT.
// TODO: This doesn't align with the dependency injection pattern, however this is the pattern followed
// in the entire code base. Revisit this later.
func NewFlowConfigHandler() *FlowConfigHandler {
	return &FlowConfigHandler{}
}

// SetHandleValidator injects the function used to verify that a flow handle exists. It should be
// called exactly once, after flowmgt.Initialize returns.
func (h *FlowConfigHandler) SetHandleValidator(fn FlowHandleValidatorFunc) {
	h.validator = fn
}

// Decode parses a raw JSON flow-section value into FlowSectionConfig. Empty input yields a zero
// FlowSectionConfig, which resolves to built-in defaults at read time.
func (h *FlowConfigHandler) Decode(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return flowconfig.FlowSectionConfig{}, nil
	}

	var cfg flowconfig.FlowSectionConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks structural constraints on the incoming FlowSectionConfig. When a validator is present,
// it additionally verifies that every non-empty handle references an existing flow of the correct type.
func (h *FlowConfigHandler) Validate(incoming, _, _ any) error {
	cfg, ok := incoming.(flowconfig.FlowSectionConfig)
	if !ok {
		return fmt.Errorf("flow: unexpected config type %T", incoming)
	}

	type entry struct {
		tc       flowconfig.FlowTypeConfig
		flowType providers.FlowType
		field    string
	}
	entries := []entry{
		{cfg.AuthFlow, providers.FlowTypeAuthentication, "authFlow"},
		{cfg.RegistrationFlow, providers.FlowTypeRegistration, "registrationFlow"},
		{cfg.UserOnboardingFlow, providers.FlowTypeUserOnboarding, "userOnboardingFlow"},
		{cfg.RecoveryFlow, providers.FlowTypeRecovery, "recoveryFlow"},
		{cfg.SignOutFlow, providers.FlowTypeSignOut, "signOutFlow"},
		{cfg.UserDeletionFlow, providers.FlowTypeAdministration, "userDeletionFlow"},
		{cfg.ApplicationDeletionFlow, providers.FlowTypeAdministration, "applicationDeletionFlow"},
		{cfg.ClientSecretRegenerationFlow, providers.FlowTypeAdministration, "clientSecretRegenerationFlow"},
	}
	ctx := context.Background()

	for _, e := range entries {
		if e.tc.ExpirySeconds < 0 {
			return fmt.Errorf("flow: %s.expirySeconds must be >= 0", e.field)
		}
		if e.tc.DefaultHandle != "" && h.validator != nil {
			if !h.validator(ctx, e.tc.DefaultHandle, e.flowType) {
				return fmt.Errorf("flow: %s.defaultHandle %q does not reference an existing flow of the correct type",
					e.field, e.tc.DefaultHandle)
			}
		}
	}

	return nil
}

// Merge overlays the writable (db) layer onto the read-only (declarative) layer. A non-empty
// writable handle or a positive writable expiry wins for its sub-field; otherwise the read-only
// value stands.
func (h *FlowConfigHandler) Merge(readOnly, writable any) any {
	ro, _ := readOnly.(flowconfig.FlowSectionConfig)
	wr, _ := writable.(flowconfig.FlowSectionConfig)
	return flowconfig.FlowSectionConfig{
		AuthFlow:           mergeFlowTypeConfig(ro.AuthFlow, wr.AuthFlow),
		RegistrationFlow:   mergeFlowTypeConfig(ro.RegistrationFlow, wr.RegistrationFlow),
		UserOnboardingFlow: mergeFlowTypeConfig(ro.UserOnboardingFlow, wr.UserOnboardingFlow),
		RecoveryFlow:       mergeFlowTypeConfig(ro.RecoveryFlow, wr.RecoveryFlow),
		SignOutFlow:        mergeFlowTypeConfig(ro.SignOutFlow, wr.SignOutFlow),
		UserDeletionFlow:   mergeFlowTypeConfig(ro.UserDeletionFlow, wr.UserDeletionFlow),
		ApplicationDeletionFlow: mergeFlowTypeConfig(
			ro.ApplicationDeletionFlow, wr.ApplicationDeletionFlow),
		ClientSecretRegenerationFlow: mergeFlowTypeConfig(
			ro.ClientSecretRegenerationFlow, wr.ClientSecretRegenerationFlow),
	}
}

// mergeFlowTypeConfig overlays the writable (db) layer onto the read-only (declarative) layer
// for a single flow type.
func mergeFlowTypeConfig(ro, wr flowconfig.FlowTypeConfig) flowconfig.FlowTypeConfig {
	merged := ro
	if wr.DefaultHandle != "" {
		merged.DefaultHandle = wr.DefaultHandle
	}
	if wr.ExpirySeconds > 0 {
		merged.ExpirySeconds = wr.ExpirySeconds
	}
	return merged
}
