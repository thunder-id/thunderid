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
	// the session's last-active timestamp and idle deadline, and records the joining participant
	// (both best-effort). It errors when the session or its checkpoint context no longer exists.
	LoadCheckpoint(ctx context.Context, handle, checkpoint, appID string) (*Session, *SessionContext, error)
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
}

// SaveCheckpointResult reports the outcome of a save. Handle is the session's handle; Created is
// true only when this call minted the session (so the caller emits the cookie); Skipped is true
// when the save was declined because of a subject mismatch.
type SaveCheckpointResult struct {
	Handle  string
	Created bool
	Skipped bool
}

// service is the store-backed implementation of Service.
type service struct {
	store         sessionStore
	resolver      Resolver
	transactioner transaction.Transactioner
	timeouts      Timeouts
	logger        *log.Logger
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
		return s.recordParticipant(txCtx, target.SessionID, in.AppID, now)
	}); err != nil {
		return SaveCheckpointResult{}, err
	}

	s.logger.Debug(ctx, "Saved SSO checkpoint", log.String("checkpoint", in.Checkpoint))
	return SaveCheckpointResult{Handle: target.HandleID, Created: created}, nil
}

// LoadCheckpoint implements Service.
func (s *service) LoadCheckpoint(ctx context.Context, handle, checkpoint, appID string) (
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

	// Record the joining application as a participant. Best-effort: the session loaded fine even if
	// this fails.
	if partErr := s.recordParticipant(ctx, sess.SessionID, appID, now); partErr != nil {
		s.logger.Warn(ctx, "Failed to record SSO session participant", log.Error(partErr))
	}

	return sess, sc, nil
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
// last-active time if it has joined before. It is a no-op when the application id is unknown.
func (s *service) recordParticipant(ctx context.Context, sessionID, appID string, now time.Time) error {
	if appID == "" {
		return nil
	}
	return s.store.Record(ctx, Participant{
		SessionID:     sessionID,
		AppID:         appID,
		FirstJoinedAt: now,
		LastActiveAt:  now,
	})
}
