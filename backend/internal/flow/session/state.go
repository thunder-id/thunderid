// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"math"
	"time"

	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
)

// maxTimeoutSeconds is the largest configured second value that converts to a time.Duration without
// overflowing its int64 nanosecond representation. A larger value would wrap to a negative duration,
// so the session section's Validate rejects it before it reaches newTimeouts.
const maxTimeoutSeconds = math.MaxInt64 / int64(time.Second)

// defaultIdleTimeout is the maximum inactivity period before a session expires. The idle deadline
// slides forward on each activity refresh.
const defaultIdleTimeout = 2 * time.Hour

// defaultAbsoluteTimeout is the maximum lifetime of a session regardless of activity. It is fixed
// at creation and never extended. It also bounds the transport cookie's max-age.
const defaultAbsoluteTimeout = 24 * time.Hour

// defaultActivityRefreshInterval is the built-in minimum time between persisted activity refreshes,
// used when the deployment does not configure session.activity_refresh_interval_seconds. Within this
// window after the last persisted activity refresh, the activity refresh (the idle-slide UPDATE) is
// skipped to cut write amplification on the hot session-reuse path.
const defaultActivityRefreshInterval = 60 * time.Second

// defaultMaxIdleTimeout is the longest idle timeout the "session" section may set, used when the
// deployment does not configure session.max_idle_timeout_seconds.
const defaultMaxIdleTimeout = 30 * 24 * time.Hour

// defaultMaxAbsoluteTimeout is the longest absolute timeout the "session" section may set, used when the
// deployment does not configure session.max_absolute_timeout_seconds.
const defaultMaxAbsoluteTimeout = 365 * 24 * time.Hour

// resolvedTimeouts holds the resolved session lifetime durations used when creating and refreshing sessions.
type resolvedTimeouts struct {
	// Idle is the maximum inactivity period; the idle deadline slides on each activity refresh.
	Idle time.Duration
	// Absolute is the maximum lifetime of a session regardless of activity.
	Absolute time.Duration
	// ActivityRefresh is the minimum spacing between persisted activity refreshes; refreshes within
	// this window of the last persisted one are skipped. newTimeouts keeps it below the idle window so
	// an active session cannot be skipped past its idle deadline.
	ActivityRefresh time.Duration
}

// deploymentLimits holds what the operator sets for SSO sessions in deployment.yaml: the activity refresh
// interval, and the longest idle and absolute timeouts the "session" section may set. A zero value takes
// the built-in default.
type deploymentLimits struct {
	activityRefresh time.Duration
	maxIdle         time.Duration
	maxAbsolute     time.Duration
}

// newDeploymentLimits reads the session limits from the flow configuration.
func newDeploymentLimits(cfg flowconfig.Config) deploymentLimits {
	return deploymentLimits{
		activityRefresh: cfg.SessionActivityRefresh,
		maxIdle:         cfg.SessionMaxIdleTimeout,
		maxAbsolute:     cfg.SessionMaxAbsoluteTimeout,
	}.resolved()
}

// resolved returns the limits with the built-in default for each one left unset.
func (l deploymentLimits) resolved() deploymentLimits {
	return deploymentLimits{
		activityRefresh: durationOrDefault(l.activityRefresh, defaultActivityRefreshInterval),
		maxIdle:         durationOrDefault(l.maxIdle, defaultMaxIdleTimeout),
		maxAbsolute:     durationOrDefault(l.maxAbsolute, defaultMaxAbsoluteTimeout),
	}
}

// durationOrDefault returns configured when it is positive, otherwise def.
func durationOrDefault(configured, def time.Duration) time.Duration {
	if configured > 0 {
		return configured
	}
	return def
}

// timeoutOrDefault converts a configured second value to a duration, or returns def when it is unset.
func timeoutOrDefault(seconds int64, def time.Duration) time.Duration {
	if seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return def
}

// newTimeouts resolves the session timeouts from a "session" section value and the deployment's limits,
// taking the built-in default for any unset timeout. Each timeout is cut to the deployment's maximum, so
// one stored before the operator lowered it applies within it. The idle window is clamped to the absolute
// lifetime so the pair is always valid, and the activity refresh interval is kept below the idle window
// so an active session is never skipped past its idle deadline, even when the deployment raised the
// interval after the section was set.
func newTimeouts(cfg Config, limits deploymentLimits) resolvedTimeouts {
	limits = limits.resolved()
	t := resolvedTimeouts{
		Idle:            min(timeoutOrDefault(cfg.IdleTimeoutSeconds, defaultIdleTimeout), limits.maxIdle),
		Absolute:        min(timeoutOrDefault(cfg.AbsoluteTimeoutSeconds, defaultAbsoluteTimeout), limits.maxAbsolute),
		ActivityRefresh: limits.activityRefresh,
	}
	if t.Idle > t.Absolute {
		t.Idle = t.Absolute
	}
	if t.ActivityRefresh >= t.Idle {
		t.ActivityRefresh = t.Idle / 2
	}
	return t
}
