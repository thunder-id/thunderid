// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"math"
	"time"
)

// maxTimeoutSeconds is the largest configured second value that converts to a time.Duration without
// overflowing its int64 nanosecond representation. A larger value would wrap to a negative duration,
// so Config.Validate rejects it before it reaches NewTimeouts.
const maxTimeoutSeconds = math.MaxInt64 / int64(time.Second)

// DefaultIdleTimeout is the maximum inactivity period before a session expires. The idle deadline
// slides forward on each activity refresh.
const DefaultIdleTimeout = 2 * time.Hour

// DefaultAbsoluteTimeout is the maximum lifetime of a session regardless of activity. It is fixed
// at creation and never extended. It also bounds the transport cookie's max-age.
const DefaultAbsoluteTimeout = 8 * time.Hour

// defaultActivityRefreshInterval is the built-in minimum time between persisted activity refreshes,
// used when the deployment does not configure session.activityRefreshIntervalSeconds. Within this
// window after the last persisted activity refresh, the activity refresh (the idle-slide UPDATE) is
// skipped to cut write amplification on the hot session-reuse path.
const defaultActivityRefreshInterval = 60 * time.Second

// Timeouts holds the resolved session lifetime durations used when minting and refreshing sessions.
type Timeouts struct {
	// Idle is the maximum inactivity period; the idle deadline slides on each activity refresh.
	Idle time.Duration
	// Absolute is the maximum lifetime of a session regardless of activity.
	Absolute time.Duration
	// ActivityRefresh is the minimum spacing between persisted activity refreshes; refreshes within
	// this window of the last persisted one are skipped. Config validation keeps it below the idle
	// window so an active session cannot be skipped past its idle deadline.
	ActivityRefresh time.Duration
}

// activityRefreshInterval resolves the activity-refresh spacing: the configured value (in seconds)
// when positive, otherwise the built-in default. The value is honored as configured; config
// validation is responsible for keeping it below the idle window.
func activityRefreshInterval(configuredSeconds int64) time.Duration {
	if configuredSeconds > 0 {
		return time.Duration(configuredSeconds) * time.Second
	}
	return defaultActivityRefreshInterval
}

// DefaultTimeouts returns the built-in default session timeouts.
func DefaultTimeouts() Timeouts {
	return Timeouts{
		Idle:            DefaultIdleTimeout,
		Absolute:        DefaultAbsoluteTimeout,
		ActivityRefresh: activityRefreshInterval(0),
	}
}

// NewTimeouts builds session timeouts from per-field second values, falling back to the built-in
// default for any non-positive value. The idle window is clamped to the absolute lifetime so the
// pair is always valid — a defaulted idle can otherwise outrun a small configured absolute, which
// SessionConfig validation does not catch (it only compares the raw, positive config values). The
// activity-refresh interval uses activityRefreshSeconds when positive, else the built-in default.
func NewTimeouts(idleSeconds, absoluteSeconds, activityRefreshSeconds int64) Timeouts {
	t := DefaultTimeouts()
	if idleSeconds > 0 {
		t.Idle = time.Duration(idleSeconds) * time.Second
	}
	if absoluteSeconds > 0 {
		t.Absolute = time.Duration(absoluteSeconds) * time.Second
	}
	if t.Idle > t.Absolute {
		t.Idle = t.Absolute
	}
	t.ActivityRefresh = activityRefreshInterval(activityRefreshSeconds)
	return t
}
