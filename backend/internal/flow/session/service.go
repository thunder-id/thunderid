/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
	"github.com/thunder-id/thunderid/internal/system/transaction"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// Service is the SSO session capability. It wraps every session-store operation so callers (the
// flow executors) depend only on this interface and never touch the stores directly. Construct it
// with Initialize.
type Service interface {
	// Resolve returns the live session for the given flow, or nil when none applies: no or expired
	// session, a session from a different flow, or one established at an incompatible flow version.
	Resolve(ctx context.Context, handle, flowID string, flowVersion int, now time.Time) (*Session, error)

	// HasCheckpoint reports whether the resolved session already holds a snapshot for the checkpoint,
	// using the decrypt-free checkpoint listing.
	HasCheckpoint(ctx context.Context, sessionID, checkpoint string) (bool, error)

	// SaveCheckpoint attaches the checkpoint to this flow execution's session — the one already
	// resolved (via HandleHint), one an earlier join minted, or a freshly established one — writing
	// the checkpoint context and the joining participant in a single transaction. Result.Skipped is
	// true when the authenticated subject conflicts with the existing session's subject.
	SaveCheckpoint(ctx context.Context, in SaveCheckpointInput) (SaveCheckpointResult, error)

	// LoadCheckpoint fetches the session referenced by handle and its checkpoint context, refreshes
	// the session's last-active timestamp and idle deadline, and records the joining participant with
	// the grant's token family id (all best-effort). It errors when the session or its checkpoint
	// context no longer exists.
	LoadCheckpoint(ctx context.Context, handle, checkpoint, appID, tokenFamilyID string) (
		*Session, *SessionContext, error)

	// Terminate ends the session referenced by handle: it marks the session ENDED (so it can no
	// longer back SSO) and removes its checkpoint contexts and participants, all in one transaction.
	// When flowID is non-empty the handle must belong to that flow, guarding against ending a
	// session grouped under a different flow. It is idempotent — a no-op returning (nil, nil) when
	// no session matches the handle, and the unchanged session when it is already ended — and
	// returns the ended session on success.
	Terminate(ctx context.Context, handle, flowID string) (*Session, error)
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
	transactioner   transaction.Transactioner
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

// HasCheckpoint implements Service.
func (s *service) HasCheckpoint(ctx context.Context, sessionID, checkpoint string) (bool, error) {
	ids, err := s.store.ListCheckpointIDs(ctx, sessionID)
	if err != nil {
		return false, fmt.Errorf("failed to list SSO session checkpoints: %w", err)
	}
	for _, id := range ids {
		if id == checkpoint {
			return true, nil
		}
	}
	return false, nil
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
func (s *service) LoadCheckpoint(ctx context.Context, handle, checkpoint, appID, tokenFamilyID string) (
	*Session, *SessionContext, error) {
	if handle == "" {
		return nil, nil, fmt.Errorf("no resolved session handle to load")
	}
	sess, err := s.store.GetByHandle(ctx, handle)
	if err != nil {
		return nil, nil, err
	}
	if sess == nil {
		return nil, nil, fmt.Errorf("resolved session no longer exists")
	}

	// Lazily load this checkpoint's durable session context (only the load path reads it).
	sc, err := s.store.GetByCheckpoint(ctx, sess.SessionID, checkpoint)
	if err != nil {
		return nil, nil, err
	}
	if sc == nil {
		return nil, nil, fmt.Errorf("session context for checkpoint %q no longer exists", checkpoint)
	}

	// Refresh last-active and slide the idle deadline under the optimistic-lock guard — touches
	// SESSION only. The absolute deadline is left unchanged so it keeps capping total lifetime. A
	// conflict here is non-fatal: the session loaded successfully.
	now := time.Now().UTC()
	sess.LastActiveAt = now
	sess.IdleExpiresAt = now.Add(s.timeouts.Idle)
	if updErr := s.store.Update(ctx, sess); updErr != nil {
		s.logger.Warn(ctx, "Failed to refresh session last-active timestamp", log.Error(updErr))
	}

	// Record the joining application as a participant. When this reused session issues a token family,
	// its SESSION_ID -> tfid mapping is security-critical: logout resolves the families to revoke from
	// these rows, so a token stamped with a tfid that has no persisted mapping would be unrevocable.
	// Fail closed in that case so the reuse does not issue an unrevocable family (the caller aborts the
	// load before publishing the tfid, forcing full re-authentication). Without a tfid there is nothing
	// to revoke, so the write stays best-effort.
	if partErr := s.recordParticipant(ctx, sess.SessionID, appID, tokenFamilyID, now); partErr != nil {
		if tokenFamilyID != "" {
			return nil, nil, fmt.Errorf("failed to record SSO session participant for token family: %w", partErr)
		}
		s.logger.Warn(ctx, "Failed to record SSO session participant", log.Error(partErr))
	}

	return sess, sc, nil
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
		// The idle deadline slides on each activity touch; the absolute deadline is fixed here and
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
