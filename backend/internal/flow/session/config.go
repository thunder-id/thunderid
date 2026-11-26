// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/internal/serverconfig"
)

// Config is the value of the server-config "session" section: how long an SSO session lasts.
// Durations are in seconds; an omitted value takes its default.
type Config struct {
	IdleTimeoutSeconds     int64 `json:"idleTimeoutSeconds,omitempty"     yaml:"idleTimeoutSeconds,omitempty"`
	AbsoluteTimeoutSeconds int64 `json:"absoluteTimeoutSeconds,omitempty" yaml:"absoluteTimeoutSeconds,omitempty"`
	// decodeErr is why a strict decode refused the value, such as a field the section does not have.
	// Validate refuses a write with it, while a value stored before still loads.
	decodeErr string
}

// sectionValue is a session value as it is written or stored. activityRefreshIntervalSeconds was part of
// the section before it moved to deployment.yaml, and an export from those versions always carries it,
// so it is accepted and ignored.
type sectionValue struct {
	Config
	ActivityRefreshIntervalSeconds int64 `json:"activityRefreshIntervalSeconds"`
}

// isEmpty reports whether the value sets no timeout.
func (c Config) isEmpty() bool {
	return c.IdleTimeoutSeconds == 0 && c.AbsoluteTimeoutSeconds == 0
}

// configHandler decodes, validates and merges the "session" server-config section. A declarative
// document locks the section.
type configHandler struct {
	// limits are the deployment's session limits: the activity refresh interval an idle timeout must
	// exceed, and the longest timeouts the section may set.
	limits deploymentLimits
}

// newConfigHandler builds the "session" server-config handler over the flow configuration.
func newConfigHandler(cfg flowconfig.Config) serverconfig.ServerConfigHandlerInterface {
	return configHandler{limits: newDeploymentLimits(cfg)}
}

// Decode parses a raw JSON session value into Config. Empty input yields the zero Config, which takes
// every default. A value the strict decode refuses, such as one with a field the section does not have, is
// decoded again without the check, since Decode also reads values stored before, and the refusal is kept
// for Validate.
func (configHandler) Decode(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return Config{}, nil
	}
	var value sectionValue
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	strictErr := decoder.Decode(&value)
	if strictErr == nil {
		return value.Config, nil
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	cfg.decodeErr = strictErr.Error()
	return cfg, nil
}

// Validate checks the incoming value: it names no field the section does not have, each timeout is unset
// or positive and no longer than the deployment's maximum for it, an idle timeout is no longer than the
// absolute timeout in effect, and the idle timeout in effect, after the defaults apply and it is cut to
// the absolute one, is longer than the deployment's activity refresh interval, so an active session is
// never left to expire between two refreshes.
func (h configHandler) Validate(_ context.Context, incoming, _, _ any) error {
	cfg, ok := incoming.(Config)
	if !ok {
		return fmt.Errorf("%s: unexpected config type %T", serverconfig.ConfigNameSession, incoming)
	}
	if cfg.decodeErr != "" {
		return fmt.Errorf("session: %s", cfg.decodeErr)
	}
	if cfg.IdleTimeoutSeconds < 0 {
		return fmt.Errorf("session.idleTimeoutSeconds must be greater than or equal to 0")
	}
	if cfg.AbsoluteTimeoutSeconds < 0 {
		return fmt.Errorf("session.absoluteTimeoutSeconds must be greater than or equal to 0")
	}
	// A larger value overflows when converted to a time.Duration and wraps to a negative one.
	if cfg.IdleTimeoutSeconds > maxTimeoutSeconds || cfg.AbsoluteTimeoutSeconds > maxTimeoutSeconds {
		return fmt.Errorf("session timeout seconds must not exceed %d", maxTimeoutSeconds)
	}
	if cfg.IdleTimeoutSeconds > int64(h.limits.maxIdle/time.Second) {
		return fmt.Errorf("session.idleTimeoutSeconds must not exceed the maximum of %d seconds",
			int64(h.limits.maxIdle/time.Second))
	}
	if cfg.AbsoluteTimeoutSeconds > int64(h.limits.maxAbsolute/time.Second) {
		return fmt.Errorf("session.absoluteTimeoutSeconds must not exceed the maximum of %d seconds",
			int64(h.limits.maxAbsolute/time.Second))
	}
	t := newTimeouts(cfg, h.limits)
	// The idle timeout in effect is cut to the absolute one, so a longer idle timeout would never apply.
	if cfg.IdleTimeoutSeconds > int64(t.Absolute/time.Second) {
		return fmt.Errorf("session.idleTimeoutSeconds must not exceed the absolute timeout of %d seconds",
			int64(t.Absolute/time.Second))
	}
	if t.Idle <= h.limits.activityRefresh {
		return fmt.Errorf("session.idleTimeoutSeconds must be longer than the activity refresh interval of %d "+
			"seconds", int64(h.limits.activityRefresh/time.Second))
	}
	return nil
}

// Merge returns the declarative document when one sets the section, otherwise the writable layer.
func (configHandler) Merge(readOnly, writable any) any {
	if ro, ok := readOnly.(Config); ok && !ro.isEmpty() {
		return ro
	}
	wr, _ := writable.(Config)
	return wr
}

// Default returns the built-in timeouts, each cut to the deployment's maximum for it. Sessions cut the
// idle timeout to the absolute one when they apply the section, not here.
func (h configHandler) Default() any {
	return Config{
		IdleTimeoutSeconds:     int64(min(defaultIdleTimeout, h.limits.maxIdle) / time.Second),
		AbsoluteTimeoutSeconds: int64(min(defaultAbsoluteTimeout, h.limits.maxAbsolute) / time.Second),
	}
}

// IsReadOnly reports whether a declarative document sets the section.
func (configHandler) IsReadOnly(readOnly any) bool {
	ro, ok := readOnly.(Config)
	return ok && !ro.isEmpty()
}
