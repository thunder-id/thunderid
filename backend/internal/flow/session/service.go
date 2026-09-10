// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package session provides the persistent SSO session model and relational store.
//
// A session is the unit that carries authenticated state across separate flow
// executions. It is grouped by flow: the flow ID is the group key, so only
// applications configured with the same flow can share a session (SSO). The
// session is referenced by an opaque handle, decoupled from the transport that
// carries it (a cookie is one such transport; see HandleTransport).
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Service is the SSO session capability. It wraps every session-store operation so callers (the
// flow executors) depend only on this interface and never touch the stores directly. Construct it
// with Initialize.
type Service interface {
	// Resolve returns the live session for the given flow, or nil when none applies: no or expired
	// session, a session from a different flow, or one established at an incompatible flow version.
	Resolve(ctx context.Context, handle, flowID string, flowVersion int, now time.Time) (*Session, error)

	// FindCheckpoint returns the resolved session's snapshot for the checkpoint, or (nil, nil) when
	// the session holds none. Callers deciding only whether a checkpoint is available compare the
	// result against nil; the SSO-Check node forwards it to the paired Session node so the load path
	// does not fetch the same row again.
	FindCheckpoint(ctx context.Context, sessionID, checkpoint string) (*SessionContext, error)

	// SaveCheckpoint attaches the checkpoint to this flow execution's session — the one already
	// resolved (via HandleHint), one an earlier join minted, or a freshly established one — writing
	// the checkpoint context and the joining participant in a single transaction. Result.Skipped is
	// true when the authenticated subject conflicts with the existing session's subject.
	SaveCheckpoint(ctx context.Context, in SaveCheckpointInput) (SaveCheckpointResult, error)

	// LoadCheckpoint returns the session referenced by in.Handle and the checkpoint's context, refreshes
	// the session's last-active timestamp and idle deadline (throttled: skipped when the last refresh is
	// within the activity-refresh window), and records the joining application as a participant. It uses
	// the rows the SSO-Check node forwarded when they match what is being loaded, and reads whatever it
	// was not given. The activity refresh is best-effort, but recording the participant is not when
	// in.TokenFamilyID is set: a token family with no persisted mapping would be unrevocable, so that
	// failure aborts the load. It errors when a row it has to read no longer exists.
	LoadCheckpoint(ctx context.Context, in LoadCheckpointInput) (*Session, *SessionContext, error)

	// Terminate ends the session referenced by handle: it revokes the token families of every
	// participating application (when a revoker is wired) and hard-deletes the session along with its
	// checkpoint contexts and participants, all in one transaction, so nothing is left that could back
	// SSO or hold live grants. When flowID is non-empty the handle must belong to that flow, guarding
	// against ending a session grouped under a different flow. It is idempotent, returning (nil, nil)
	// when no session matches the handle, and returns the deleted session on success.
	Terminate(ctx context.Context, handle, flowID string) (*Session, error)
	// TerminateBySubject ends every SSO session belonging to the subject, revoking the subject's
	// grants first so no session is deleted while its tokens remain live. It is idempotent, returning
	// nil when the subject holds no sessions.
	TerminateBySubject(ctx context.Context, subjectID string) error
}

// LoadCheckpointInput carries what a Session join needs to restore a checkpoint. Session and Context
// are the rows the SSO-Check node already read for this checkpoint, handed over so the load path does
// not repeat those two queries; either may be nil, in which case it is read from the store.
type LoadCheckpointInput struct {
	// Handle is the resolved session handle. It identifies the session to load when Session is nil,
	// and guards the handed-over Session against belonging to a different handle.
	Handle string
	// Checkpoint is the checkpoint id whose snapshot is being restored.
	Checkpoint string
	// AppID is the joining application, recorded as a session participant.
	AppID string
	// TokenFamilyID is the token family id (tfid) minted for this grant. Recording it is required
	// when non-empty: a tfid with no persisted mapping would be unrevocable.
	TokenFamilyID string
	// Session is the session the SSO-Check node resolved, or nil to read it by Handle.
	Session *Session
	// Context is the checkpoint context the SSO-Check node fetched, or nil to read it.
	Context *SessionContext
}

// SaveCheckpointInput carries the data a Session join needs to persist. The caller resolves the
// subject and builds the (already sanitized) snapshot; the service only stores it.
type SaveCheckpointInput struct {
	SubjectID      string
	FlowID         string
	FlowVersion    int
	ExecutionID    string
	HandleHint     string // shared handle for this execution, or "" to look up by execution id
	Checkpoint     string
	AuthUser       json.RawMessage
	RuntimeData    map[string]string
	CompletedSteps map[string]StepFact
	AppID          string
	// TokenFamilyID is the token family id (tfid) minted by the caller for this grant. It is stored on
	// the joining participant so logout can resolve the session to its families. Empty leaves the
	// participant's tfid unset.
	TokenFamilyID string
}

// SaveCheckpointResult reports the outcome of a save. Handle is the session's handle; Created is
// true only when this call minted the session (so the caller emits the cookie); Skipped is true
// when the save was declined because of a subject mismatch.
type SaveCheckpointResult struct {
	Handle  string
	Created bool
	Skipped bool
}

// CriteriaRevoker revokes a token family (one authorization grant) by its id. It is injected so session
// sign-out can drop the session's grants without the session package depending on the OAuth
// revocation implementation. A nil revoker disables sign-out revocation.
type CriteriaRevoker interface {
	RevokeTokenFamily(ctx context.Context, tokenFamilyID string) error
}

// service is the store-backed implementation of Service.
type service struct {
	store           sessionStore
	resolver        Resolver
	transactioner   providers.Transactioner
	criteriaRevoker CriteriaRevoker
	timeouts        Timeouts
	logger          *log.Logger
}

var _ Service = (*service)(nil)

// Resolve implements Service.
func (s *service) Resolve(ctx context.Context, handle, flowID string, flowVersion int,
	now time.Time) (*Session, error) {
	if handle == "" {
		return nil, nil
	}
	sess, err := s.resolver.Resolve(ctx, handle, now)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve SSO session: %w", err)
	}
	if sess == nil {
		return nil, nil
	}
	if sess.FlowID != flowID {
		s.logger.Debug(ctx, "Resolved session belongs to a different flow; ignoring")
		return nil, nil
	}
	if sess.FlowVersion != flowVersion {
		s.logger.Debug(ctx, "Resolved session has an incompatible flow version; forcing full authentication")
		return nil, nil
	}
	return sess, nil
}

// FindCheckpoint implements Service. Fetching the checkpoint context by its full primary key answers
// the availability question by itself, so this replaces listing every checkpoint id and matching in
// Go, and it returns the row for the caller to hand to LoadCheckpoint.
func (s *service) FindCheckpoint(ctx context.Context, sessionID, checkpoint string) (*SessionContext, error) {
	snapshot, err := s.store.GetByCheckpoint(ctx, sessionID, checkpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSO session checkpoint: %w", err)
	}
	return snapshot, nil
}

// SaveCheckpoint implements Service.
func (s *service) SaveCheckpoint(ctx context.Context, in SaveCheckpointInput) (SaveCheckpointResult, error) {
	target, created, err := s.targetSession(ctx, in)
	if err != nil {
		return SaveCheckpointResult{}, err
	}
	if target == nil {
		return SaveCheckpointResult{Skipped: true}, nil
	}

	// Reaching this point means the subject authenticated during this execution: the SSO-hit path
	// loads its checkpoint and never saves one. When that happens inside a session that already
	// existed, it was a re-authentication (prompt=login, or a max_age the previous authentication no
	// longer satisfied), so the session's authentication time has to move forward with it. Leaving it
	// stale would make the session claim an older authentication than actually took place, which
	// under-reports auth_time and makes a later max_age check reject a request it should allow.
	if !created {
		now := time.Now().UTC()
		if err := s.store.TouchAuthenticatedAt(ctx, target.SessionID, now,
			now.Add(s.timeouts.Idle)); err != nil {
			// The checkpoint itself is still worth saving, so degrade rather than fail the login.
			s.logger.Error(ctx, "Failed to refresh session authentication time", log.Error(err))
		} else {
			target.AuthenticatedAt = now
		}
	}

	snapshot := SessionContext{
		SessionID:      target.SessionID,
		CheckpointID:   in.Checkpoint,
		RuntimeData:    in.RuntimeData,
		AuthUser:       in.AuthUser,
		CompletedSteps: in.CompletedSteps,
		ContextVersion: 1,
	}

	// Write this checkpoint's context (upsert) and the joining participant in one transaction.
	now := time.Now().UTC()
	if err := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := s.store.CreateContext(txCtx, snapshot); err != nil {
			return err
		}
		return s.recordParticipant(txCtx, target.SessionID, in.AppID, in.TokenFamilyID, now)
	}); err != nil {
		return SaveCheckpointResult{}, err
	}

	s.logger.Debug(ctx, "Saved SSO checkpoint", log.String("checkpoint", in.Checkpoint))
	return SaveCheckpointResult{Handle: target.HandleID, Created: created}, nil
}

// LoadCheckpoint implements Service.
func (s *service) LoadCheckpoint(ctx context.Context, in LoadCheckpointInput) (
	*Session, *SessionContext, error) {
	if in.Handle == "" {
		return nil, nil, fmt.Errorf("no resolved session handle to load")
	}
	// The SSO-Check node resolved the session and read this checkpoint's context to decide routing, and
	// forwards both here, so this path re-reads neither. The resolver has already applied every liveness,
	// flow-identity and flow-version check. What the handover gives up is noticing that a concurrent
	// sign-out deleted the session in between: the idle slide below then matches no row and logs, and the
	// reuse still completes. That is accepted, since the check node had already committed to skipping.
	// ForwardedData reaches the immediate next node only, so anything not handed over is read here and a
	// load still works when the handover did not survive to this node.
	sess := in.Session
	if sess != nil && sess.HandleID != in.Handle {
		sess = nil // forwarded from a different handle; do not trust it
	}
	if sess == nil {
		loaded, err := s.store.GetByHandle(ctx, in.Handle)
		if err != nil {
			return nil, nil, err
		}
		if loaded == nil {
			return nil, nil, fmt.Errorf("resolved session no longer exists")
		}
		sess = loaded
	}

	// This checkpoint's durable context, read here only when the check node's row did not reach us.
	snapshot := in.Context
	if snapshot != nil && (snapshot.SessionID != sess.SessionID || snapshot.CheckpointID != in.Checkpoint) {
		snapshot = nil // forwarded for a different session or checkpoint; do not trust it
	}
	if snapshot == nil {
		loaded, err := s.store.GetByCheckpoint(ctx, sess.SessionID, in.Checkpoint)
		if err != nil {
			return nil, nil, err
		}
		if loaded == nil {
			return nil, nil, fmt.Errorf("session context for checkpoint %q no longer exists", in.Checkpoint)
		}
		snapshot = loaded
	}

	// Refresh last-active and slide the idle deadline under the optimistic-lock guard — touches
	// SESSION only. The absolute deadline is left unchanged so it keeps capping total lifetime. A
	// conflict here is non-fatal: the session loaded successfully.
	//
	// Throttle the write: within ActivityRefresh of the last persisted activity refresh, skip it. This
	// hot path fires on every session reuse, and an unthrottled UPDATE per reuse is the dominant write
	// load (and, on Postgres, the main source of dead tuples) on the session table. The persisted idle
	// deadline then lags real activity by at most ActivityRefresh; config validation keeps that below
	// the idle window so an active session is never skipped past its idle deadline.
	now := time.Now().UTC()
	if now.Sub(sess.LastActiveAt) >= s.timeouts.ActivityRefresh {
		sess.LastActiveAt = now
		sess.IdleExpiresAt = now.Add(s.timeouts.Idle)
		if updErr := s.store.Update(ctx, sess); updErr != nil {
			s.logger.Warn(ctx, "Failed to refresh session last-active timestamp", log.Error(updErr))
		}
	}

	// Record the joining application as a participant. When this reused session issues a token family,
	// its SESSION_ID -> tfid mapping is security-critical: logout resolves the families to revoke from
	// these rows, so a token stamped with a tfid that has no persisted mapping would be unrevocable.
	// Fail closed in that case so the reuse does not issue an unrevocable family (the caller aborts the
	// load before publishing the tfid, forcing full re-authentication). Without a tfid there is nothing
	// to revoke, so the write stays best-effort. Either way it is not throttled with the activity
	// refresh above: the upsert also registers an application joining the session for the first time.
	if partErr := s.recordParticipant(ctx, sess.SessionID, in.AppID, in.TokenFamilyID, now); partErr != nil {
		if in.TokenFamilyID != "" {
			return nil, nil, fmt.Errorf("failed to record SSO session participant for token family: %w", partErr)
		}
		s.logger.Warn(ctx, "Failed to record SSO session participant", log.Error(partErr))
	}

	return sess, snapshot, nil
}

// Terminate implements Service.
func (s *service) Terminate(ctx context.Context, handle, flowID string) (*Session, error) {
	if handle == "" {
		return nil, nil
	}
	sess, err := s.store.GetByHandle(ctx, handle)
	if err != nil {
		return nil, fmt.Errorf("failed to load session for termination: %w", err)
	}
	if sess == nil {
		return nil, nil
	}
	// The handle must belong to the expected flow. A per-flow handle resolving to a session grouped
	// under a different flow should never happen; surface it as an error rather than silently skipping.
	if flowID != "" && sess.FlowID != flowID {
		return nil, fmt.Errorf("session handle belongs to flow %q, expected %q", sess.FlowID, flowID)
	}
	// Hard-delete the session and its derived state (checkpoint contexts and participants) in one
	// transaction. Sign-out ends SSO reuse outright and nothing references the session afterwards, so the
	// row is removed rather than tombstoned. DeleteSession removes the session row (SSO_SESSION), Delete
	// its checkpoint contexts (SSO_SESSION_CONTEXT), and DeleteBySessionID its participants
	// (SSO_SESSION_PARTICIPANT). Repeated calls are idempotent: once the row is gone, GetByHandle
	// returns nil above. Token families are revoked first, in the same transaction, so a crash can
	// never orphan live tokens for a deleted session.
	if txErr := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if revErr := s.revokeSessionFamilies(txCtx, sess.SessionID); revErr != nil {
			return revErr
		}
		if delErr := s.store.DeleteSession(txCtx, sess.SessionID); delErr != nil {
			return delErr
		}
		if delErr := s.store.Delete(txCtx, sess.SessionID); delErr != nil {
			return delErr
		}
		return s.store.DeleteBySessionID(txCtx, sess.SessionID)
	}); txErr != nil {
		return nil, fmt.Errorf("failed to terminate session: %w", txErr)
	}

	s.logger.Debug(ctx, "Terminated SSO session", log.String("flowId", sess.FlowID))
	return sess, nil
}

// TerminateBySubject ends every SSO session belonging to the subject. Repeated calls are idempotent.
//
// Unlike Terminate, this does not revoke token families. Subject-wide termination is driven by a
// criteria revocation on the subject dimension, which already matches every token those sessions
// hold regardless of which application's family issued it, so enumerating families here would write
// one redundant deny-list row per participating application.
//
// That makes the criteria revocation a precondition, not an optimisation: the caller must have
// persisted the subject criterion before calling this, or sessions are deleted while their tokens
// stay live. In flows this is enforced at flow-creation time, where a flow containing
// SessionRevocationExecutor must also contain CriteriaRevocationExecutor.
//
// All deletions share one transaction, so a partial failure leaves every session intact rather than
// some subset terminated.
func (s *service) TerminateBySubject(ctx context.Context, subjectID string) error {
	if subjectID == "" {
		return nil
	}
	sessions, err := s.store.ListBySubject(ctx, subjectID)
	if err != nil {
		return fmt.Errorf("failed to list sessions by subject: %w", err)
	}
	if len(sessions) == 0 {
		return nil
	}

	if txErr := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		for _, sess := range sessions {
			if delErr := s.store.DeleteSession(txCtx, sess.SessionID); delErr != nil {
				return delErr
			}
			if delErr := s.store.Delete(txCtx, sess.SessionID); delErr != nil {
				return delErr
			}
			if delErr := s.store.DeleteBySessionID(txCtx, sess.SessionID); delErr != nil {
				return delErr
			}
		}
		return nil
	}); txErr != nil {
		return fmt.Errorf("failed to terminate subject sessions: %w", txErr)
	}

	s.logger.Debug(ctx, "Terminated all SSO sessions for subject", log.Int("sessionCount", len(sessions)))
	return nil
}

// revokeSessionFamilies revokes the token family of every application participating in the session,
// so signing out of a login drops all of that login's grants. It is a no-op when no family revoker is
// wired. A participant recorded before tfid was introduced (empty tfid) is skipped by the revoker.
func (s *service) revokeSessionFamilies(ctx context.Context, sessionID string) error {
	if s.criteriaRevoker == nil {
		return nil
	}
	participants, err := s.store.ListBySessionID(ctx, sessionID)
	if err != nil {
		return err
	}
	for _, p := range participants {
		if err := s.criteriaRevoker.RevokeTokenFamily(ctx, p.TokenFamilyID); err != nil {
			return err
		}
	}
	return nil
}

// targetSession returns the session this execution's checkpoints attach to, establishing one when
// none exists yet. The bool reports whether this call minted the session. It returns (nil, false,
// nil) when an existing session belongs to a different subject than the one just authenticated, so
// the caller skips the save rather than cross-attaching.
func (s *service) targetSession(ctx context.Context, in SaveCheckpointInput) (*Session, bool, error) {
	existing, err := s.existingSession(ctx, in.HandleHint, in.ExecutionID)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		if existing.SubjectID != in.SubjectID {
			s.logger.Warn(ctx,
				"Authenticated subject differs from the SSO session subject; not attaching checkpoint")
			return nil, false, nil
		}
		return existing, false, nil
	}
	return s.establishSession(ctx, in)
}

// existingSession returns the session already backing this execution: the one referenced by the
// shared handle hint, else the one recorded against this flow execution id. Returns (nil, nil) when
// none exists yet.
func (s *service) existingSession(ctx context.Context, handleHint, executionID string) (*Session, error) {
	if handleHint != "" {
		return s.store.GetByHandle(ctx, handleHint)
	}
	return s.store.GetByExecutionID(ctx, executionID)
}

// establishSession mints and inserts a new session for this flow execution. The insert is idempotent
// on the flow execution id, so under concurrency it re-reads and returns whichever session won the
// race; the returned bool is true only when this call minted the winner.
func (s *service) establishSession(ctx context.Context, in SaveCheckpointInput) (*Session, bool, error) {
	sessionID, err := sysutils.GenerateUUIDv7()
	if err != nil {
		return nil, false, fmt.Errorf("failed to generate session id: %w", err)
	}
	handle, err := cryptolib.GenerateSecureToken()
	if err != nil {
		return nil, false, fmt.Errorf("failed to generate session handle: %w", err)
	}

	now := time.Now().UTC()
	newSession := Session{
		SessionID:       sessionID,
		SubjectID:       in.SubjectID,
		FlowID:          in.FlowID,
		FlowVersion:     in.FlowVersion,
		FlowExecutionID: in.ExecutionID,
		HandleID:        handle,
		AuthenticatedAt: now,
		CreatedAt:       now,
		LastActiveAt:    now,
		// The idle deadline slides on each activity refresh; the absolute deadline is fixed here and
		// caps the session's total lifetime. The resolver rejects a session past either deadline.
		IdleExpiresAt:     now.Add(s.timeouts.Idle),
		AbsoluteExpiresAt: now.Add(s.timeouts.Absolute),
		State:             StateActive,
		Version:           1,
	}
	if err := s.store.Create(ctx, newSession); err != nil {
		return nil, false, err
	}

	// Re-read the session that actually persisted for this execution: the insert is a no-op when a
	// concurrent join already established one, so this returns the winner (this call's row or the racer's).
	established, err := s.store.GetByExecutionID(ctx, in.ExecutionID)
	if err != nil {
		return nil, false, err
	}
	if established == nil {
		return nil, false, fmt.Errorf("session establishment did not persist for execution %q", in.ExecutionID)
	}
	created := established.HandleID == handle
	if created {
		s.logger.Debug(ctx, "Established SSO session", log.String("flowId", in.FlowID))
	}
	return established, created, nil
}

// recordParticipant records the application as a participant of the session, refreshing its
// last-active time and current-grant tfid if it has joined before. It is a no-op when the
// application id is unknown.
func (s *service) recordParticipant(ctx context.Context, sessionID, appID, tokenFamilyID string,
	now time.Time) error {
	if appID == "" {
		return nil
	}
	return s.store.Record(ctx, Participant{
		SessionID:     sessionID,
		AppID:         appID,
		TokenFamilyID: tokenFamilyID,
		FirstJoinedAt: now,
		LastActiveAt:  now,
	})
}
