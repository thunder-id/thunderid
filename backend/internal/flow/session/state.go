// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"math"
	"time"
)

// maxTimeoutSeconds is the largest configured second value that converts to a time.Duration without
// overflowing its int64 nanosecond representation. A larger value would wrap to a negative duration,
// so the session section's Validate rejects it before it reaches newTimeouts.
const maxTimeoutSeconds = math.MaxInt64 / int64(time.Second)

// DefaultIdleTimeout is the maximum inactivity period before a session expires. The idle deadline
// slides forward on each activity refresh.
const DefaultIdleTimeout = 2 * time.Hour

// DefaultAbsoluteTimeout is the maximum lifetime of a session regardless of activity. It is fixed
// at creation and never extended. It also bounds the transport cookie's max-age.
const DefaultAbsoluteTimeout = 8 * time.Hour

// defaultActivityRefreshInterval is the built-in minimum time between persisted activity refreshes,
// used when the deployment does not configure session.activity_refresh_interval_seconds. Within this
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
	// this window of the last persisted one are skipped. newTimeouts keeps it below the idle window so
	// an active session cannot be skipped past its idle deadline.
	ActivityRefresh time.Duration
}

// activityRefreshInterval resolves the activity-refresh spacing: the configured interval when
// positive, otherwise the built-in default.
func activityRefreshInterval(configured time.Duration) time.Duration {
	if configured > 0 {
		return configured
	}
	return defaultActivityRefreshInterval
}

// timeoutOrDefault converts a configured second value to a duration, or returns def when it is unset.
func timeoutOrDefault(seconds int64, def time.Duration) time.Duration {
	if seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return def
}

// DefaultTimeouts returns the built-in default session timeouts.
func DefaultTimeouts() Timeouts {
	return Timeouts{
		Idle:            DefaultIdleTimeout,
		Absolute:        DefaultAbsoluteTimeout,
		ActivityRefresh: activityRefreshInterval(0),
	}
}

// newTimeouts resolves the session timeouts from a "session" section value and the deployment's
// activity refresh interval, taking the built-in default for any unset timeout. The idle window is
// clamped to the absolute lifetime so the pair is always valid, and the activity refresh interval is
// kept below the idle window so an active session is never skipped past its idle deadline, even when
// the deployment raised the interval after the section was set.
func newTimeouts(cfg Config, activityRefresh time.Duration) Timeouts {
	t := Timeouts{
		Idle:            timeoutOrDefault(cfg.IdleTimeoutSeconds, DefaultIdleTimeout),
		Absolute:        timeoutOrDefault(cfg.AbsoluteTimeoutSeconds, DefaultAbsoluteTimeout),
		ActivityRefresh: activityRefreshInterval(activityRefresh),
	}
	if t.Idle > t.Absolute {
		t.Idle = t.Absolute
	}
	if t.ActivityRefresh >= t.Idle {
		t.ActivityRefresh = t.Idle / 2
	}
	return t
}
