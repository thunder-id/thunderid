// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"time"
)

// Resolver loads a session from an opaque handle and returns it only when it is live. It does
// not check flow identity or version — that is the SSO-Check node's responsibility.
type Resolver interface {
	// Resolve returns the session referenced by handleID when it is ACTIVE and within its
	// deadlines at now. It returns (nil, nil) for every "no live session" case (absent,
	// ended/revoked, expired), and a non-nil error only on a store failure.
	Resolve(ctx context.Context, handleID string, now time.Time) (*Session, error)
}

type resolver struct {
	store    sessionStore
	timeouts timeoutsSource
}

// newResolver creates a Resolver backed by the given session store, which also holds a session to the
// timeouts in effect.
func newResolver(store sessionStore, timeouts timeoutsSource) Resolver {
	return &resolver{store: store, timeouts: timeouts}
}

// Resolve implements Resolver. A session is live while it is within the deadlines stored with it and
// within the timeouts in effect now, so a shorter timeout also ends an existing session that has gone
// past it, while a longer one never stretches a stored deadline.
func (r *resolver) Resolve(ctx context.Context, handleID string, now time.Time) (*Session, error) {
	if handleID == "" {
		return nil, nil
	}

	s, err := r.store.GetByHandle(ctx, handleID)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, nil
	}

	if s.State != StateActive {
		return nil, nil
	}
	if expired(s.IdleExpiresAt, now) || expired(s.AbsoluteExpiresAt, now) {
		return nil, nil
	}
	timeouts := r.timeouts.current(ctx)
	if expiredAfter(s.LastActiveAt, timeouts.Idle, now) || expiredAfter(s.CreatedAt, timeouts.Absolute, now) {
		return nil, nil
	}

	return s, nil
}

// expired reports whether a deadline is set and has been reached at now. A zero deadline
// means "no deadline" and never expires.
func expired(deadline, now time.Time) bool {
	return !deadline.IsZero() && !now.Before(deadline)
}

// expiredAfter reports whether lifetime has passed since start at now. A zero start is unknown and never
// expires.
func expiredAfter(start time.Time, lifetime time.Duration, now time.Time) bool {
	return !start.IsZero() && expired(start.Add(lifetime), now)
}
