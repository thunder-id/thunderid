// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"time"
)

// sessionStore is the package-private persistence contract covering SSO sessions, their
// per-checkpoint session contexts, and their participants. A single runtime-persistent-DB-backed
// implementation (store) satisfies it, and the service depends on this one interface. It is not
// used outside the package.
type sessionStore interface {
	// Create persists a new session.
	Create(ctx context.Context, s Session) error
	// GetByHandle fetches a session by its opaque handle ID. It returns (nil, nil) when no
	// session matches; liveness checks are the resolver's responsibility.
	GetByHandle(ctx context.Context, handleID string) (*Session, error)
	// GetByExecutionID fetches the session established by the given flow execution, or (nil, nil)
	// when that execution has not established one.
	GetByExecutionID(ctx context.Context, flowExecutionID string) (*Session, error)
	// ListBySubject returns every SSO session belonging to the subject.
	ListBySubject(ctx context.Context, subjectID string) ([]Session, error)
	// Update writes the mutable fields of an existing session under an optimistic-lock guard. It
	// returns errVersionConflict when the stored version no longer matches, and bumps the in-memory
	// Version on success.
	Update(ctx context.Context, s *Session) error
	// TouchAuthenticatedAt records a fresh authentication inside an existing session and slides the
	// idle deadline with it. It carries no version guard, so it never loses to a concurrent slide.
	TouchAuthenticatedAt(ctx context.Context, sessionID string, authenticatedAt,
		idleExpiresAt time.Time) error

	// CreateContext persists (or overwrites) one checkpoint's session context for a session.
	CreateContext(ctx context.Context, c SessionContext) error
	// GetByCheckpoint fetches one checkpoint's session context. It returns (nil, nil) when none exists.
	GetByCheckpoint(ctx context.Context, sessionID, checkpointID string) (*SessionContext, error)
	// Delete removes all of a session's checkpoint contexts.
	Delete(ctx context.Context, sessionID string) error
	// DeleteSession removes the session row itself.
	DeleteSession(ctx context.Context, sessionID string) error

	// Record inserts the participant, or refreshes its LAST_ACTIVE_AT (preserving FIRST_JOINED_AT)
	// when the application has already joined the session.
	Record(ctx context.Context, p Participant) error
	// ListBySessionID returns the applications that have joined the session, oldest first.
	ListBySessionID(ctx context.Context, sessionID string) ([]Participant, error)
	// DeleteBySessionID removes all participants of a session.
	DeleteBySessionID(ctx context.Context, sessionID string) error
}
