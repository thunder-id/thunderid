// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/internal/system/log"
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

// NewConfigHandler builds the "session" server-config handler over the flow configuration.
func NewConfigHandler(cfg flowconfig.Config) serverconfig.ServerConfigHandlerInterface {
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

// Default returns the built-in timeouts.
func (configHandler) Default() any {
	return Config{
		IdleTimeoutSeconds:     int64(DefaultIdleTimeout / time.Second),
		AbsoluteTimeoutSeconds: int64(DefaultAbsoluteTimeout / time.Second),
	}
}

// IsLocked reports whether a declarative document sets the section.
func (configHandler) IsLocked(readOnly any) bool {
	ro, ok := readOnly.(Config)
	return ok && !ro.isEmpty()
}

// timeoutsSource supplies the session timeouts in effect.
type timeoutsSource interface {
	current(ctx context.Context) Timeouts
}

// sectionTimeouts reads the "session" section on every use, so a change made on any node applies
// without a restart. When the section cannot be read, the timeouts last read apply, so a failed read
// never lengthens a session; before any read succeeds, the defaults apply.
type sectionTimeouts struct {
	reader          serverconfig.ServerConfigService
	activityRefresh time.Duration
	// reads numbers each read in the order it started, and lastRead holds the timeouts of the latest
	// started read that succeeded, so a read that started earlier and finished later can't replace them.
	reads    atomic.Uint64
	lastRead atomic.Pointer[timeoutsRead]
	logger   *log.Logger
}

// timeoutsRead is the timeouts one read resolved, with the number of the read.
type timeoutsRead struct {
	seq      uint64
	timeouts Timeouts
}

// newSectionTimeouts builds the source over the server-config service the section is read from and the
// deployment's activity refresh interval.
func newSectionTimeouts(reader serverconfig.ServerConfigService, activityRefresh time.Duration) timeoutsSource {
	return &sectionTimeouts{
		reader:          reader,
		activityRefresh: activityRefresh,
		logger:          log.GetLogger().With(log.String(log.LoggerKeyComponentName, "SessionTimeouts")),
	}
}

// current reads the effective "session" section and resolves its timeouts.
func (s *sectionTimeouts) current(ctx context.Context) Timeouts {
	seq := s.reads.Add(1)
	value, svcErr := s.reader.GetMergedConfig(ctx, string(serverconfig.ConfigNameSession))
	if svcErr != nil {
		s.logger.Warn(ctx, "Failed to read the session server config; using the timeouts last read",
			log.String("errorCode", svcErr.Code))
		return s.lastReadOrDefault()
	}
	cfg, ok := value.(Config)
	if !ok {
		s.logger.Warn(ctx, "Unexpected session server config type; using the timeouts last read")
		return s.lastReadOrDefault()
	}
	timeouts := newTimeouts(cfg, s.activityRefresh)
	s.keep(&timeoutsRead{seq: seq, timeouts: timeouts})
	return timeouts
}

// keep makes read the last read, unless a read that started after it is already kept.
func (s *sectionTimeouts) keep(read *timeoutsRead) {
	for {
		last := s.lastRead.Load()
		if last != nil && last.seq > read.seq {
			return
		}
		if s.lastRead.CompareAndSwap(last, read) {
			return
		}
	}
}

// lastReadOrDefault returns the timeouts last read from the section, or the defaults before any read
// succeeded.
func (s *sectionTimeouts) lastReadOrDefault() Timeouts {
	if last := s.lastRead.Load(); last != nil {
		return last.timeouts
	}
	return newTimeouts(Config{}, s.activityRefresh)
}
