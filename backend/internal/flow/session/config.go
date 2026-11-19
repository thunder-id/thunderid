// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
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
}

// isEmpty reports whether the value sets nothing.
func (c Config) isEmpty() bool {
	return c == Config{}
}

// configHandler decodes, validates and merges the "session" server-config section. A declarative
// document locks the section.
type configHandler struct {
	// activityRefresh is the deployment's activity refresh interval, which an idle timeout must exceed.
	activityRefresh time.Duration
}

// newConfigHandler builds the "session" server-config handler over the flow configuration.
func newConfigHandler(cfg flowconfig.Config) serverconfig.ServerConfigHandlerInterface {
	return configHandler{activityRefresh: activityRefreshInterval(cfg.SessionActivityRefresh)}
}

// Decode parses a raw JSON session value into Config. Empty input yields the zero Config, which takes
// every default.
func (configHandler) Decode(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return Config{}, nil
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate checks the incoming value: each timeout is unset or positive, an idle timeout is no longer
// than an absolute one set with it, and the idle timeout in effect, after the defaults apply and it is
// cut to the absolute one, is longer than the deployment's activity refresh interval, so an active
// session is never left to expire between two refreshes.
func (h configHandler) Validate(_ context.Context, incoming, _, _ any) error {
	cfg, ok := incoming.(Config)
	if !ok {
		return fmt.Errorf("%s: unexpected config type %T", serverconfig.ConfigNameSession, incoming)
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
	if cfg.IdleTimeoutSeconds > 0 && cfg.AbsoluteTimeoutSeconds > 0 &&
		cfg.IdleTimeoutSeconds > cfg.AbsoluteTimeoutSeconds {
		return fmt.Errorf("session.idleTimeoutSeconds must not exceed absoluteTimeoutSeconds")
	}
	if newTimeouts(cfg, h.activityRefresh).Idle <= h.activityRefresh {
		return fmt.Errorf("session.idleTimeoutSeconds must be longer than the activity refresh interval of %d "+
			"seconds", int64(h.activityRefresh/time.Second))
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

// Default returns the built-in timeouts. The default idle timeout is cut to an absolute timeout configured
// shorter than it, as it is in effect.
func (configHandler) Default(configured any) any {
	idle := defaultIdleTimeout
	if cfg, _ := configured.(Config); cfg.AbsoluteTimeoutSeconds > 0 {
		idle = min(idle, time.Duration(cfg.AbsoluteTimeoutSeconds)*time.Second)
	}
	return Config{
		IdleTimeoutSeconds:     int64(idle / time.Second),
		AbsoluteTimeoutSeconds: int64(defaultAbsoluteTimeout / time.Second),
	}
}

// IsReadOnly reports whether a declarative document sets the section.
func (configHandler) IsReadOnly(readOnly any) bool {
	ro, ok := readOnly.(Config)
	return ok && !ro.isEmpty()
}
