// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package granthandlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// configSectionOAuth duplicates the serverconfig.ConfigNameOAuth literal so this package need not
// import serverconfig, which would invert the dependency: section handlers live in the consuming
// package and are registered at the composition root.
const configSectionOAuth = "oauth"

// maxGraceCeilingSeconds bounds the configurable ceiling. The window is a deliberate weakening of
// refresh token replay detection, so an unbounded value would let an operator turn rotation into
// effectively unlimited reuse of the previous generation while believing replay protection is
// intact. An hour is far beyond any legitimate concurrency spread.
const maxGraceCeilingSeconds int64 = 3600

// OAuthServerConfig is the value of the server-config "oauth" section: the runtime-mutable OAuth
// policy of the deployment. Each sub-domain has its own block, so a setting added later sits beside
// the existing ones without renaming them.
type OAuthServerConfig struct {
	// RefreshToken carries the graceful refresh token rotation policy.
	RefreshToken RefreshTokenServerConfig `json:"refreshToken" yaml:"refreshToken"`
}

// RefreshTokenServerConfig is the graceful refresh token rotation policy.
//
// The switch and the ceiling live together so the kill switch sits beside the value it kills:
// setting GraceEnabled to false closes every open window immediately, in one write, without having
// to find and correct each application's own grace period first.
type RefreshTokenServerConfig struct {
	// GraceEnabled gates the whole feature. When false, no application receives a grace window
	// whatever its own configuration says. It is a pointer so an explicit false in the writable layer
	// can be told apart from a field that was not written, and so can switch the feature off over a
	// declarative layer that enables it.
	GraceEnabled *bool `json:"graceEnabled,omitempty" yaml:"graceEnabled,omitempty"`
	// GraceCeilingSeconds is the longest grace window any application may use. An application's own
	// value can narrow the window, never widen it past this ceiling.
	GraceCeilingSeconds int64 `json:"graceCeilingSeconds,omitempty" yaml:"graceCeilingSeconds,omitempty"`
}

// validateRanges rejects out-of-range values in a single layer.
func (c RefreshTokenServerConfig) validateRanges() error {
	if c.GraceCeilingSeconds < 0 {
		return fmt.Errorf("oauth.refreshToken.graceCeilingSeconds must be greater than or equal to 0")
	}
	if c.GraceCeilingSeconds > maxGraceCeilingSeconds {
		return fmt.Errorf("oauth.refreshToken.graceCeilingSeconds must not exceed %d", maxGraceCeilingSeconds)
	}
	return nil
}

// validateCoherence rejects an effective policy that enables the feature with no ceiling.
func (c RefreshTokenServerConfig) validateCoherence() error {
	if c.graceEnabled() && c.GraceCeilingSeconds == 0 {
		return fmt.Errorf(
			"oauth.refreshToken.graceCeilingSeconds must be greater than 0 when graceEnabled is true")
	}
	return nil
}

func (c RefreshTokenServerConfig) graceEnabled() bool {
	return c.GraceEnabled != nil && *c.GraceEnabled
}

// Ceiling returns the ceiling as a duration, or zero when the feature is disabled. A disabled
// feature reports no window regardless of the configured ceiling, so callers cannot accidentally
// honor a stale value.
func (c RefreshTokenServerConfig) Ceiling() time.Duration {
	if !c.graceEnabled() || c.GraceCeilingSeconds <= 0 {
		return 0
	}
	return time.Duration(c.GraceCeilingSeconds) * time.Second
}

// OAuthServerConfigHandler decodes, validates, and merges the "oauth" server-config section. It
// implements the serverconfig section-handler contract structurally so the section can be
// registered at the composition root without this package depending on the serverconfig package.
type OAuthServerConfigHandler struct{}

// Decode parses a raw JSON oauth value into OAuthServerConfig. Empty input yields the zero value,
// which reports every feature disabled.
func (OAuthServerConfigHandler) Decode(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return OAuthServerConfig{}, nil
	}
	var cfg OAuthServerConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate checks the incoming value's own ranges, then checks coherence of the policy that would
// take effect. A write replaces the writable layer, so the effective policy is the incoming value
// overlaid on the read-only layer; an incoming value that only switches the feature on is coherent
// when the declarative layer already supplies the ceiling.
func (h OAuthServerConfigHandler) Validate(incoming, readOnly, _ any) error {
	in, _ := incoming.(OAuthServerConfig)
	if err := in.RefreshToken.validateRanges(); err != nil {
		return err
	}
	effective, _ := h.Merge(readOnly, in).(OAuthServerConfig)
	return effective.RefreshToken.validateCoherence()
}

// Merge overlays the writable (db) layer onto the read-only (declarative) layer block by block and
// field by field, so a write to one block never resets another.
func (OAuthServerConfigHandler) Merge(readOnly, writable any) any {
	ro, _ := readOnly.(OAuthServerConfig)
	wr, _ := writable.(OAuthServerConfig)
	return OAuthServerConfig{
		RefreshToken: mergeRefreshTokenServerConfig(ro.RefreshToken, wr.RefreshToken),
	}
}

// mergeRefreshTokenServerConfig overlays the writable refresh token policy onto the read-only one. A
// written GraceEnabled wins even when false, and a positive ceiling wins.
func mergeRefreshTokenServerConfig(ro, wr RefreshTokenServerConfig) RefreshTokenServerConfig {
	merged := ro
	if wr.GraceEnabled != nil {
		merged.GraceEnabled = wr.GraceEnabled
	}
	if wr.GraceCeilingSeconds > 0 {
		merged.GraceCeilingSeconds = wr.GraceCeilingSeconds
	}
	return merged
}

// OAuthServerConfigReader reads the merged (effective) value of a server-config section. The
// server-config service satisfies it; this narrow interface keeps this package from importing
// serverconfig.
type OAuthServerConfigReader interface {
	GetMergedConfig(ctx context.Context, name string) (any, *common.ServiceError)
}
