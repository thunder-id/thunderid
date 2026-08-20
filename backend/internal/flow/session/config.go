// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"encoding/json"
	"fmt"
)

// Config is the value of the server-config "session" section: the SSO session lifetime
// configuration. Durations are in seconds; a zero or omitted value falls back to the built-in
// default (see NewTimeouts).
type Config struct {
	IdleTimeoutSeconds     int64 `json:"idleTimeoutSeconds"     yaml:"idleTimeoutSeconds"`
	AbsoluteTimeoutSeconds int64 `json:"absoluteTimeoutSeconds" yaml:"absoluteTimeoutSeconds"`
	// ActivityRefreshIntervalSeconds is the minimum spacing between persisted activity refreshes: a
	// session reuse within this window of the last persisted activity refresh skips the idle-slide
	// write, cutting write load on the hot path. It is honored as configured and must be less than
	// idleTimeoutSeconds (enforced by Validate), so an active session is never skipped past its idle
	// deadline.
	ActivityRefreshIntervalSeconds int64 `json:"activityRefreshIntervalSeconds" yaml:"activityRefreshIntervalSeconds"`
}

// Validate ensures the configured session timeouts are coherent. Unset (zero) values are allowed and
// fall back to defaults; the activity-refresh/idle invariant is checked against the resolved
// durations so a defaulted side cannot silently violate it.
func (c Config) Validate() error {
	if c.IdleTimeoutSeconds < 0 {
		return fmt.Errorf("session.idleTimeoutSeconds must be greater than or equal to 0")
	}
	if c.AbsoluteTimeoutSeconds < 0 {
		return fmt.Errorf("session.absoluteTimeoutSeconds must be greater than or equal to 0")
	}
	if c.ActivityRefreshIntervalSeconds < 0 {
		return fmt.Errorf("session.activityRefreshIntervalSeconds must be greater than or equal to 0")
	}
	// Reject values that would overflow when converted to a time.Duration (nanoseconds); such a value
	// wraps to a negative duration and would otherwise slip past the invariant check below.
	if c.IdleTimeoutSeconds > maxTimeoutSeconds || c.AbsoluteTimeoutSeconds > maxTimeoutSeconds ||
		c.ActivityRefreshIntervalSeconds > maxTimeoutSeconds {
		return fmt.Errorf("session timeout seconds must not exceed %d", maxTimeoutSeconds)
	}
	if c.IdleTimeoutSeconds > 0 && c.AbsoluteTimeoutSeconds > 0 &&
		c.IdleTimeoutSeconds > c.AbsoluteTimeoutSeconds {
		return fmt.Errorf("session.idleTimeoutSeconds must not exceed absoluteTimeoutSeconds")
	}
	// Check the resolved durations, not the raw config values: a zero means "use the default", so the
	// activity-refresh/idle invariant must be verified after defaults are substituted (and idle is
	// clamped to absolute). Comparing only raw positive values lets a defaulted side violate it, e.g.
	// a small configured idle with a defaulted refresh, or a defaulted idle with a large configured
	// refresh, either of which could skip an active session past its idle deadline.
	resolved := NewTimeouts(c.IdleTimeoutSeconds, c.AbsoluteTimeoutSeconds, c.ActivityRefreshIntervalSeconds)
	if resolved.ActivityRefresh >= resolved.Idle {
		return fmt.Errorf("session.activityRefreshIntervalSeconds must be less than idleTimeoutSeconds")
	}
	return nil
}

// ConfigHandler decodes, validates, and merges the "session" server-config section. It implements
// the serverconfig section-handler contract structurally so the section can be registered at the
// composition root without this package depending on the serverconfig package.
type ConfigHandler struct{}

// Decode parses a raw JSON session value into Config. Empty input yields the zero Config, which
// resolves to the built-in default timeouts.
func (ConfigHandler) Decode(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return Config{}, nil
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate checks that the incoming value is a coherent session config.
func (ConfigHandler) Validate(_ context.Context, incoming, _, _ any) error {
	cfg, _ := incoming.(Config)
	return cfg.Validate()
}

// Merge overlays the writable (db) layer onto the read-only (declarative) layer: a positive writable
// value wins for its field, otherwise the read-only value stands.
func (ConfigHandler) Merge(readOnly, writable any) any {
	merged, _ := readOnly.(Config)
	wr, _ := writable.(Config)
	if wr.IdleTimeoutSeconds > 0 {
		merged.IdleTimeoutSeconds = wr.IdleTimeoutSeconds
	}
	if wr.AbsoluteTimeoutSeconds > 0 {
		merged.AbsoluteTimeoutSeconds = wr.AbsoluteTimeoutSeconds
	}
	if wr.ActivityRefreshIntervalSeconds > 0 {
		merged.ActivityRefreshIntervalSeconds = wr.ActivityRefreshIntervalSeconds
	}
	return merged
}
