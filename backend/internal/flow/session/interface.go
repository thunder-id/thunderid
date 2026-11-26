// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
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
	// ListLiveBySubject returns one page of the subject's live SSO sessions at now, most recently active
	// first. A non-nil ouID keeps only the sessions with an application signed in under it.
	ListLiveBySubject(ctx context.Context, subjectID string, ouID *string, now time.Time, limit, offset int) (
		[]Session, error)
	// CountLiveBySubject counts the subject's live SSO sessions at now, under ouID when it is not nil.
	CountLiveBySubject(ctx context.Context, subjectID string, ouID *string, now time.Time) (int, error)
	// ListLiveByApp returns one page of the live SSO sessions at now that the application has joined, most
	// recently active first. A non-nil ouID keeps only the sessions it joined under that unit.
	ListLiveByApp(ctx context.Context, appID string, ouID *string, now time.Time, limit, offset int) (
		[]Session, error)
	// CountLiveByApp counts the live SSO sessions at now that the application has joined, under ouID when it
	// is not nil.
	CountLiveByApp(ctx context.Context, appID string, ouID *string, now time.Time) (int, error)
	// GetSubjectParticipantByTokenFamily returns the participant of one of the subject's SSO sessions that
	// holds the token family, or nil when none does.
	GetSubjectParticipantByTokenFamily(ctx context.Context, subjectID, tokenFamilyID string) (*Participant, error)
	// Update writes the mutable fields of an existing session under an optimistic-lock guard. It
	// returns errVersionConflict when the stored version no longer matches, and bumps the in-memory
	// Version on success.
	Update(ctx context.Context, s *Session) error
	// TouchAuthenticatedAt records a fresh authentication inside an existing session, with the session's
	// properties (carrying the IP it came from), and slides the idle deadline with it. It carries no
	// version guard, so it never loses to a concurrent slide.
	TouchAuthenticatedAt(ctx context.Context, sessionID string, authenticatedAt,
		idleExpiresAt time.Time, properties SessionProperties) error
	// SlideIdleDeadline moves an active, unexpired session's last-active timestamp and idle deadline as
	// slide says, unless the session was refreshed after slide.refreshedBefore. It matches only a session
	// last active after slide.activeAfter and created after slide.createdAfter, so a timeout shortened
	// since the stored deadlines were set ends the session rather than letting a refresh extend it. A
	// session it does not match is left as it is and is not an error.
	SlideIdleDeadline(ctx context.Context, sessionID string, slide idleSlide) error

	// CreateContext persists (or overwrites) one checkpoint's session context for a session.
	CreateContext(ctx context.Context, c SessionContext) error
	// GetByCheckpoint fetches one checkpoint's session context. It returns (nil, nil) when none exists.
	GetByCheckpoint(ctx context.Context, sessionID, checkpointID string) (*SessionContext, error)
	// Delete removes all of a session's checkpoint contexts.
	Delete(ctx context.Context, sessionID string) error
	// DeleteSession removes the session row itself and reports whether a row was removed.
	DeleteSession(ctx context.Context, sessionID string) (bool, error)

	// Record inserts the participant, or refreshes its LAST_ACTIVE_AT (preserving FIRST_JOINED_AT and the
	// token family) on a rejoin under the same organization unit, and returns the token family id it holds.
	Record(ctx context.Context, p Participant) (string, error)
	// ReplaceTokenFamily moves the participant from currentTokenFamilyID to p.TokenFamilyID and returns its tfid.
	ReplaceTokenFamily(ctx context.Context, p Participant, currentTokenFamilyID string) (string, error)
	// ListBySessionID returns the applications that have joined the session, oldest first.
	ListBySessionID(ctx context.Context, sessionID string) ([]Participant, error)
	// ListBySessionIDs returns the participants of all the given sessions, each session's
	// participants oldest first.
	ListBySessionIDs(ctx context.Context, sessionIDs []string) ([]Participant, error)
	// DeleteBySessionID removes all participants of a session.
	DeleteBySessionID(ctx context.Context, sessionID string) error
	// ListByAppID returns every participation of the application, across sessions, oldest first. It is the
	// only listing that fills Participant.SubjectID.
	ListByAppID(ctx context.Context, appID string) ([]Participant, error)
	// DeleteParticipant removes one application's participation in one session under one organization unit.
	DeleteParticipant(ctx context.Context, sessionID, appID, ouID string) error
}

// applicationProvider reads the applications shown with each session. The application service satisfies it.
// It is declared here because the application package depends on this one, so this package can't import it.
type applicationProvider interface {
	GetApplication(ctx context.Context, appID string) (*providers.Application, *tidcommon.ServiceError)
}
