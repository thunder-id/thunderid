// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"
	"fmt"
)

// PersonalAgentConfig is the value of the server-config "personalAgent" section: the organization's
// settings for personal assistant agents. Unset values fall back to the defaults, so personal agents
// stay disabled until the organization turns them on.
type PersonalAgentConfig struct {
	// Enabled is a pointer so a writable false can override a read-only true. Unset means disabled.
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// MaxAccessTokenValidity caps, in seconds, every access token issued to a personal agent. Unset or
	// 0 means the default cap.
	MaxAccessTokenValidity int64 `json:"maxAccessTokenValidity,omitempty" yaml:"maxAccessTokenValidity,omitempty"`
	// AuthFlow names the login flow for a personal agent whose registration flow fixes none.
	AuthFlow PersonalAgentAuthFlowConfig `json:"authFlow" yaml:"authFlow"`
}

// PersonalAgentAuthFlowConfig holds the default login flow handle for personal agents.
type PersonalAgentAuthFlowConfig struct {
	DefaultHandle string `json:"defaultHandle,omitempty" yaml:"defaultHandle,omitempty"`
}

// IsEnabled reports whether personal agents are enabled for the organization. Unset means disabled.
func (c PersonalAgentConfig) IsEnabled() bool {
	return c.Enabled != nil && *c.Enabled
}

// EffectiveMaxAccessTokenValidity returns the access token lifetime cap in seconds, or the default cap
// when none is set.
func (c PersonalAgentConfig) EffectiveMaxAccessTokenValidity() int64 {
	if c.MaxAccessTokenValidity > 0 {
		return c.MaxAccessTokenValidity
	}
	return defaultPersonalAgentMaxAccessTokenValidity
}

// EffectiveAuthFlowHandle returns the default login flow handle for personal agents, or the shipped
// default flow when none is set.
func (c PersonalAgentConfig) EffectiveAuthFlowHandle() string {
	if c.AuthFlow.DefaultHandle != "" {
		return c.AuthFlow.DefaultHandle
	}
	return defaultPersonalAgentAuthFlowHandle
}

// AuthFlowHandleValidatorFunc reports whether a handle names an existing AUTHENTICATION flow.
type AuthFlowHandleValidatorFunc func(ctx context.Context, handle string) bool

// PersonalAgentConfigHandler decodes, validates, and merges the server-config "personalAgent" section.
// The login flow handle is checked only once a validator is injected with SetAuthFlowHandleValidator;
// declarative files load before that, as for the flow section.
type PersonalAgentConfigHandler struct {
	validator AuthFlowHandleValidatorFunc
}

// NewPersonalAgentConfigHandler creates a PersonalAgentConfigHandler with no login flow handle validator.
func NewPersonalAgentConfigHandler() *PersonalAgentConfigHandler {
	return &PersonalAgentConfigHandler{}
}

// SetAuthFlowHandleValidator injects the function used to verify that the login flow handle names an
// existing AUTHENTICATION flow. It is called once, after the flow management service is initialized.
func (h *PersonalAgentConfigHandler) SetAuthFlowHandleValidator(fn AuthFlowHandleValidatorFunc) {
	h.validator = fn
}

// Decode parses a raw JSON personalAgent value into PersonalAgentConfig. Empty input yields the zero
// value, which resolves to the defaults.
func (h *PersonalAgentConfigHandler) Decode(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return PersonalAgentConfig{}, nil
	}
	var cfg PersonalAgentConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate checks that the token lifetime cap is not negative and, when a validator is present, that
// the login flow handle names an existing AUTHENTICATION flow.
func (h *PersonalAgentConfigHandler) Validate(incoming, _, _ any) error {
	cfg, ok := incoming.(PersonalAgentConfig)
	if !ok {
		return fmt.Errorf("personalAgent: unexpected config type %T", incoming)
	}
	if cfg.MaxAccessTokenValidity < 0 {
		return fmt.Errorf("personalAgent: maxAccessTokenValidity must be >= 0")
	}
	if cfg.AuthFlow.DefaultHandle != "" && h.validator != nil &&
		!h.validator(context.Background(), cfg.AuthFlow.DefaultHandle) {
		return fmt.Errorf("personalAgent: authFlow.defaultHandle %q does not reference an existing "+
			"AUTHENTICATION flow", cfg.AuthFlow.DefaultHandle)
	}
	return nil
}

// Merge starts from the defaults, then overlays the read-only (declarative) layer and the writable (db)
// layer: a set value wins for its field. The merged value therefore always holds the effective settings,
// even when nothing is configured.
func (h *PersonalAgentConfigHandler) Merge(readOnly, writable any) any {
	disabled := false
	merged := PersonalAgentConfig{
		Enabled:                &disabled,
		MaxAccessTokenValidity: defaultPersonalAgentMaxAccessTokenValidity,
		AuthFlow:               PersonalAgentAuthFlowConfig{DefaultHandle: defaultPersonalAgentAuthFlowHandle},
	}
	ro, _ := readOnly.(PersonalAgentConfig)
	wr, _ := writable.(PersonalAgentConfig)
	for _, layer := range []PersonalAgentConfig{ro, wr} {
		if layer.Enabled != nil {
			merged.Enabled = layer.Enabled
		}
		if layer.MaxAccessTokenValidity > 0 {
			merged.MaxAccessTokenValidity = layer.MaxAccessTokenValidity
		}
		if layer.AuthFlow.DefaultHandle != "" {
			merged.AuthFlow.DefaultHandle = layer.AuthFlow.DefaultHandle
		}
	}
	return merged
}
