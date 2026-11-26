// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package session provides the persistent SSO session model and relational store.
//
// A session is the unit that carries authenticated state across separate flow
// executions. It is grouped by flow: the flow ID is the group key, so only
// applications configured with the same flow can share a session (SSO). The
// session is referenced by an opaque handle, decoupled from the transport that
// carries it (a cookie is one such transport; see HandleTransportInterface).
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"
	"time"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/eventlistener"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Service is the SSO session capability. It wraps every session-store operation so callers (the
// flow executors) depend only on this interface and never touch the stores directly. Construct it
// with Initialize.
type Service interface {
	// Resolve returns the live session for the given flow, or nil when none applies: no or expired
	// session, a session from a different flow, or one established at an incompatible flow version.
	Resolve(ctx context.Context, handle, flowID string, flowVersion int, now time.Time) (*Session, error)

	// AbsoluteTimeout returns the longest a session lasts from its start.
	AbsoluteTimeout() time.Duration

	// FindCheckpoint returns the resolved session's snapshot for the checkpoint, or (nil, nil) when
	// the session holds none or the sign-in on the context cannot reuse it. Callers deciding only whether a
	// checkpoint is available compare the result against nil; the SSO-Check node forwards it to the paired
	// Session node so the load path does not fetch the same row again.
	FindCheckpoint(ctx context.Context, sess *Session, checkpoint string) (*SessionContext, error)

	// SaveCheckpoint attaches the checkpoint to this flow execution's session — the one already
	// resolved (via HandleHint), one an earlier join minted, or a freshly established one — writing
	// the checkpoint context and the joining participant in a single transaction. Result.Skipped is
	// true when the authenticated subject conflicts with the existing session's subject.
	SaveCheckpoint(ctx context.Context, in SaveCheckpointInput) (SaveCheckpointResult, error)

	// CreateDirect records a direct session for a login that established no SSO session: the session and the
	// signing-in application as its participant, written in one transaction. A direct session has no handle,
	// so no later login can reuse it.
	CreateDirect(ctx context.Context, in CreateDirectInput) error

	// LoadCheckpoint returns the session referenced by in.Handle and the checkpoint's context, refreshes
	// the session's last-active timestamp and idle deadline (throttled: skipped when the last refresh is
	// within the activity-refresh window), and records the joining application as a participant. It uses
	// the rows the SSO-Check node forwarded when they match what is being loaded, and reads whatever it
	// was not given. The activity refresh is best-effort, but recording the participant is not when
	// in.TokenFamilyID is set: a token family with no persisted mapping would be unrevocable, so that
	// failure aborts the load. It errors when a row it has to read no longer exists.
	LoadCheckpoint(ctx context.Context, in LoadCheckpointInput) (*Session, *SessionContext, error)

	// RecordActivity counts a use of the session outside the flow engine as activity: it refreshes the
	// last-active timestamp and slides the idle deadline under the same activity-refresh throttle as a
	// checkpoint load. The absolute deadline is unchanged, and an expired, terminated or unknown session
	// is left as it is without an error.
	RecordActivity(ctx context.Context, sessionID string) error

	// Terminate ends the session referenced by handle: it revokes the token families of every
	// participating application (when a revoker is wired) and hard-deletes the session along with its
	// checkpoint contexts and participants, all in one transaction, so nothing is left that could back
	// SSO or hold live grants. When flowID is non-empty the handle must belong to that flow, guarding
	// against ending a session grouped under a different flow. It is idempotent, returning (nil, nil)
	// when no session matches the handle, and returns the deleted session on success.
	Terminate(ctx context.Context, handle, flowID string) (*Session, error)
	// TerminateBySubject ends every session belonging to the subject, SSO and direct, revoking the
	// subject's grants first so no session is deleted while its tokens remain live. It notifies only for
	// sessions that had not expired. It is idempotent, returning nil when the subject holds no sessions.
	TerminateBySubject(ctx context.Context, subjectID string) error

	// DetachApplication detaches an application from every session it participates in: for each
	// organization unit it signed in under, it revokes that participation's token family and drops its
	// row, deleting the session itself only when the application was its last participant. It is
	// idempotent, returning nil when the application participates in none.
	DetachApplication(ctx context.Context, appID string) error
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
	// ClientInfo is the requesting client's metadata; its IP is recorded as the session's last-active
	// IP when the login flow reuses the session.
	ClientInfo ClientInfo
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
	// ClientInfo is the requesting client's metadata, recorded on a session this save establishes and
	// used to update the last-active IP of an existing one.
	ClientInfo ClientInfo
}

// CreateDirectInput carries what recording a direct session needs.
type CreateDirectInput struct {
	// SessionID is the id the login's assertion carries as sid.
	SessionID   string
	SubjectID   string
	FlowID      string
	FlowVersion int
	AppID       string
	// TokenFamilyID is the token family id (tfid) of the login's grant. It is stored on the participant so
	// ending the session revokes the grant.
	TokenFamilyID string
	// ClientInfo is the requesting client's metadata, recorded on the session.
	ClientInfo ClientInfo
}

// SaveCheckpointResult reports the outcome of a save. Handle is the session's handle and SessionID
// its id; Skipped is true when the save was declined because of a subject mismatch.
type SaveCheckpointResult struct {
	Handle    string
	SessionID string
	Skipped   bool
	// AuthenticatedAt is when the subject authenticated for this session, as the session records
	// it. The caller publishes it so auth_time is read from the session on the fresh-login path
	// too, rather than being re-derived from the clock when the assertion is built.
	AuthenticatedAt time.Time
	// AbsoluteExpiresAt is when the session ends, as the session records it. A joined session keeps the
	// end it was created with.
	AbsoluteExpiresAt time.Time
}

// CriteriaRevoker revokes a token family (one authorization grant) by its id. It is injected so session
// sign-out can drop the session's grants without the session package depending on the OAuth
// revocation implementation. A nil revoker disables sign-out revocation.
type CriteriaRevoker interface {
	RevokeTokenFamily(ctx context.Context, tokenFamilyID string) error
}

// service is the store-backed implementation of Service.
type service struct {
	store            sessionStore
	resolver         Resolver
	transactioner    providers.Transactioner
	criteriaRevoker  CriteriaRevoker
	ouEligibility    OUEligibility
	terminationTopic *eventlistener.Topic[TerminatedSession]
	timeouts         Timeouts
	entities         entity.EntityServiceInterface
	entityTypes      entitytype.EntityTypeServiceInterface
	apps             applicationProvider
	logger           *log.Logger
}

// OUEligibility answers whether the sign-in on the context may land in an organization unit. The OU provider
// implements it.
type OUEligibility interface {
	IsEligibleOU(ctx context.Context, ouID string) (bool, *tidcommon.ServiceError)
}

var _ Service = (*service)(nil)

// AbsoluteTimeout implements Service.
func (s *service) AbsoluteTimeout() time.Duration {
	return s.timeouts.Absolute
}

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
func (s *service) FindCheckpoint(ctx context.Context, sess *Session, checkpoint string) (*SessionContext, error) {
	snapshot, err := s.store.GetByCheckpoint(ctx, sess.SessionID, checkpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSO session checkpoint: %w", err)
	}
	if snapshot == nil {
		return nil, nil
	}
	if _, reusable, err := s.reuseOU(ctx, sess); err != nil || !reusable {
		return nil, err
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
		properties := target.Properties
		properties.LastActiveIP = s.activeIP(in.ClientInfo, target)
		if err := s.store.TouchAuthenticatedAt(ctx, target.SessionID, now,
			now.Add(s.timeouts.Idle), properties); err != nil {
			// The checkpoint itself is still worth saving, so degrade rather than fail the login.
			s.logger.Error(ctx, "Failed to refresh session authentication time", log.Error(err))
		} else {
			target.AuthenticatedAt = now
			target.Properties = properties
		}
	}

	snapshot := SessionContext{
		SessionID:      target.SessionID,
		CheckpointID:   in.Checkpoint,
		RuntimeData:    snapshotRuntimeData(in.RuntimeData),
		AuthUser:       in.AuthUser,
		CompletedSteps: in.CompletedSteps,
		ContextVersion: 1,
	}

	participantOU, err := s.participantOU(ctx, in.SubjectID, participantOUID(ctx))
	if err != nil {
		return SaveCheckpointResult{}, err
	}

	// Write this checkpoint's context (upsert) and the joining participant in one transaction.
	now := time.Now().UTC()
	if err := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := s.store.CreateContext(txCtx, snapshot); err != nil {
			return err
		}
		return s.recordParticipant(txCtx, target.SessionID, in.AppID, participantOU, in.TokenFamilyID, now)
	}); err != nil {
		return SaveCheckpointResult{}, err
	}

	s.logger.Debug(ctx, "Saved SSO checkpoint", log.String("checkpoint", in.Checkpoint))
	return SaveCheckpointResult{
		Handle:            target.HandleID,
		SessionID:         target.SessionID,
		AuthenticatedAt:   target.AuthenticatedAt,
		AbsoluteExpiresAt: target.AbsoluteExpiresAt,
	}, nil
}

// CreateDirect implements Service.
func (s *service) CreateDirect(ctx context.Context, in CreateDirectInput) error {
	participantOU, err := s.participantOU(ctx, in.SubjectID, participantOUID(ctx))
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	sess := s.newSession(TypeDirect, in.SessionID, in.SubjectID, in.FlowID, in.FlowVersion, in.ClientInfo, now)
	if err := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := s.store.Create(txCtx, sess); err != nil {
			return err
		}
		return s.recordParticipant(txCtx, in.SessionID, in.AppID, participantOU, in.TokenFamilyID, now)
	}); err != nil {
		return fmt.Errorf("failed to record direct session: %w", err)
	}

	s.logger.Debug(ctx, "Recorded direct session", log.String("flowID", in.FlowID))
	return nil
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
		sess.Properties.LastActiveIP = s.activeIP(in.ClientInfo, sess)
		sess.IdleExpiresAt = now.Add(s.timeouts.Idle)
		if updErr := s.store.Update(ctx, sess); updErr != nil {
			s.logger.Warn(ctx, "Failed to refresh session last-active timestamp", log.Error(updErr))
		}
	}

	// A sign-in that decided no organization unit follows the one the reuse does, which reaches the flow through
	// the replayed runtime data.
	followedOUID, reusable, err := s.reuseOU(ctx, sess)
	if err != nil {
		return nil, nil, err
	}
	if !reusable {
		return nil, nil, fmt.Errorf("the sign-in can no longer reuse the session")
	}
	if followedOUID != "" {
		followed := *snapshot
		followed.RuntimeData = maps.Clone(snapshot.RuntimeData)
		if followed.RuntimeData == nil {
			followed.RuntimeData = map[string]string{}
		}
		followed.RuntimeData[common.RuntimeKeyOUID] = followedOUID
		snapshot = &followed
	}

	// Record the joining application as a participant. When this reused session issues a token family,
	// its SESSION_ID -> tfid mapping is security-critical: logout resolves the families to revoke from
	// these rows, so a token stamped with a tfid that has no persisted mapping would be unrevocable.
	// Fail closed in that case so the reuse does not issue an unrevocable family (the caller aborts the
	// load before publishing the tfid, forcing full re-authentication). Without a tfid there is nothing
	// to revoke, so the write stays best-effort. Either way it is not throttled with the activity
	// refresh above: the upsert also registers an application joining the session for the first time.
	participantOU := followedOUID
	if fixed := participantOUID(ctx); fixed != "" {
		if participantOU, err = s.participantOU(ctx, sess.SubjectID, fixed); err != nil {
			return nil, nil, err
		}
	}
	if partErr := s.recordParticipant(ctx, sess.SessionID, in.AppID, participantOU, in.TokenFamilyID,
		now); partErr != nil {
		if in.TokenFamilyID != "" {
			return nil, nil, fmt.Errorf("failed to record SSO session participant for token family: %w", partErr)
		}
		s.logger.Warn(ctx, "Failed to record SSO session participant", log.Error(partErr))
	}

	return sess, snapshot, nil
}

// participantOUID returns the sign-in boundary's organization unit, the one the grant is signed in under,
// or "" outside a boundary.
func participantOUID(ctx context.Context) string {
	boundary, _ := authnprovidercm.SignInBoundaryFrom(ctx)
	return boundary.OUID
}

// participantOU returns the organization unit to record a participant under: ouID, while the subject may sign in
// there. A sign-in the assertion will refuse is recorded under none, so it never becomes the session's last-used
// organization unit.
func (s *service) participantOU(ctx context.Context, subjectID, ouID string) (string, error) {
	if ouID == "" {
		return "", nil
	}
	boundary, _ := authnprovidercm.SignInBoundaryFrom(ctx)
	boundary.EntityID = subjectID
	eligible, svcErr := s.ouEligibility.IsEligibleOU(authnprovidercm.WithSignInBoundary(ctx, boundary), ouID)
	if svcErr != nil {
		return "", fmt.Errorf("failed to check the participant's organization unit: %s", svcErr.Code)
	}
	if !eligible {
		s.logger.Debug(ctx, "Subject cannot sign in to the organization unit; recording the participant under none")
		return "", nil
	}
	return ouID, nil
}

// snapshotRuntimeData copies the runtime data for the snapshot without the organization unit and its fixed mark,
// which are decided per sign-in; a replayed copy would overwrite the joining flow's own.
func snapshotRuntimeData(runtimeData map[string]string) map[string]string {
	snapshot := maps.Clone(runtimeData)
	delete(snapshot, common.RuntimeKeyOUID)
	delete(snapshot, common.RuntimeKeyOUFixed)
	return snapshot
}

// reuseOU returns the organization unit a sign-in follows when it reuses the session. A sign-in bounded with no
// organization unit decided follows the one the session's subject last signed in to with it, while the subject may
// still sign in there. A sign-in bounded to a decided organization unit follows none, and reuses the session only
// while its subject may sign in to that unit. reusable is false otherwise, and the sign-in authenticates instead. An
// unbounded sign-in follows none and may reuse the session.
func (s *service) reuseOU(ctx context.Context, sess *Session) (string, bool, error) {
	boundary, bounded := authnprovidercm.SignInBoundaryFrom(ctx)
	if !bounded {
		return "", true, nil
	}
	if boundary.OUID != "" {
		boundary.EntityID = sess.SubjectID
		eligible, svcErr := s.ouEligibility.IsEligibleOU(authnprovidercm.WithSignInBoundary(ctx, boundary),
			boundary.OUID)
		if svcErr != nil {
			return "", false, fmt.Errorf("failed to check the session subject's organization unit: %s", svcErr.Code)
		}
		if !eligible {
			s.logger.Debug(ctx, "Session subject cannot sign in to the decided organization unit; authenticating")
		}
		return "", eligible, nil
	}
	lastUsed, err := s.lastUsedOUID(ctx, sess.SessionID)
	if err != nil {
		return "", false, err
	}
	if lastUsed == "" {
		s.logger.Debug(ctx, "Session has no last-used organization unit; authenticating")
		return "", false, nil
	}

	boundary.EntityID = sess.SubjectID
	eligible, svcErr := s.ouEligibility.IsEligibleOU(authnprovidercm.WithSignInBoundary(ctx, boundary), lastUsed)
	if svcErr != nil {
		return "", false, fmt.Errorf("failed to check the session subject's last-used organization unit: %s",
			svcErr.Code)
	}
	if !eligible {
		s.logger.Debug(ctx, "Session subject cannot sign in to its last-used organization unit; authenticating")
		return "", false, nil
	}
	return lastUsed, true, nil
}

// lastUsedOUID returns the organization unit of the session's most recently active participant that signed in
// under one, or "" when none did.
func (s *service) lastUsedOUID(ctx context.Context, sessionID string) (string, error) {
	participants, err := s.store.ListBySessionID(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to list session participants: %w", err)
	}
	var latest *Participant
	for i := range participants {
		p := &participants[i]
		if p.OUID != "" && (latest == nil || p.LastActiveAt.After(latest.LastActiveAt)) {
			latest = p
		}
	}
	if latest == nil {
		return "", nil
	}
	return latest.OUID, nil
}

// RecordActivity implements Service.
func (s *service) RecordActivity(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	now := time.Now().UTC()
	if err := s.store.SlideIdleDeadline(ctx, sessionID, now, now.Add(s.timeouts.Idle),
		now.Add(-s.timeouts.ActivityRefresh)); err != nil {
		return fmt.Errorf("failed to record SSO session activity: %w", err)
	}
	return nil
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
	if err := s.terminateSessions(ctx, []Session{*sess}, nil, TerminationReasonSignOut); err != nil {
		return nil, fmt.Errorf("failed to terminate session: %w", err)
	}

	s.logger.Debug(ctx, "Terminated SSO session", log.String("flowID", sess.FlowID))
	return sess, nil
}

// TerminateBySubject ends every session belonging to the subject. Repeated calls are idempotent.
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

	// Capture every session's participants in one read inside the transaction, before the deletes
	// remove them, as Terminate does. A failed read only drops the notification; it never blocks the
	// revocation.
	var participantsBySession map[string][]Participant
	removed := make(map[string]bool, len(sessions))
	if txErr := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		participantsBySession = s.participantsForTermination(txCtx, sessions)
		for _, sess := range sessions {
			wasRemoved, delErr := s.deleteSession(txCtx, sess.SessionID)
			if delErr != nil {
				return delErr
			}
			removed[sess.SessionID] = wasRemoved
		}
		return nil
	}); txErr != nil {
		return fmt.Errorf("failed to terminate subject sessions: %w", txErr)
	}

	s.logger.Debug(ctx, "Terminated all sessions for subject", log.Int("sessionCount", len(sessions)))
	if participantsBySession != nil {
		now := time.Now()
		for _, sess := range sessions {
			if !removed[sess.SessionID] {
				// A concurrent termination already removed this session and notifies for it.
				continue
			}
			if expired(sess.IdleExpiresAt, now) || expired(sess.AbsoluteExpiresAt, now) {
				// Expiry already ended it, and expiry sends no notification.
				continue
			}
			s.terminationTopic.Notify(ctx, TerminatedSession{
				SessionID:    sess.SessionID,
				SubjectID:    sess.SubjectID,
				Participants: participantsBySession[sess.SessionID],
				Reason:       TerminationReasonSubjectRevocation,
			})
		}
	}
	return nil
}

// terminateSessions ends the sessions in one transaction. Each session's token families are revoked before it is
// deleted, in the same transaction, so a crash never leaves live tokens for a deleted session. A non-nil ouID
// ends only the sessions with an application signed in under that organization unit. Listeners are notified
// after commit, for the sessions this call removed.
func (s *service) terminateSessions(ctx context.Context, sessions []Session, ouID *string,
	reason TerminationReason) error {
	notify := s.terminationTopic.HasListeners()
	var (
		participants map[string][]Participant
		ended        []Session
	)
	if txErr := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		ended = nil
		if s.criteriaRevoker != nil || notify || ouID != nil {
			var readErr error
			participants, readErr = s.participantsBySession(txCtx, sessions)
			switch {
			case readErr == nil:
			case s.criteriaRevoker != nil || ouID != nil:
				return readErr
			default:
				s.logger.Error(txCtx, "Failed to read session participants before termination; "+
					"participants will not be notified", log.Error(readErr))
				notify = false
			}
		}

		for _, sess := range sessions {
			if ouID != nil && !slices.ContainsFunc(participants[sess.SessionID],
				func(p Participant) bool { return p.OUID == *ouID }) {
				continue
			}
			if revErr := s.revokeFamilies(txCtx, participants[sess.SessionID]); revErr != nil {
				return revErr
			}
			removed, delErr := s.deleteSession(txCtx, sess.SessionID)
			if delErr != nil {
				return delErr
			}
			// A concurrent termination already removed the row; only the call that removed it notifies.
			if removed {
				ended = append(ended, sess)
			}
		}
		return nil
	}); txErr != nil {
		return txErr
	}

	if !notify {
		return nil
	}
	for _, sess := range ended {
		s.terminationTopic.Notify(ctx, TerminatedSession{
			SessionID:    sess.SessionID,
			SubjectID:    sess.SubjectID,
			Participants: participants[sess.SessionID],
			Reason:       reason,
		})
	}
	return nil
}

// DetachApplication detaches an application from every session it participates in. Narrower than
// Terminate: only this application's participation goes, and the session is deleted once nothing is left
// to participate. Each token family is revoked before its row is dropped, in one transaction.
func (s *service) DetachApplication(ctx context.Context, appID string) error {
	if appID == "" {
		return nil
	}
	participations, err := s.store.ListByAppID(ctx, appID)
	if err != nil {
		return fmt.Errorf("failed to list session participation by application: %w", err)
	}
	if len(participations) == 0 {
		return nil
	}

	if txErr := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		for _, participation := range participations {
			if s.criteriaRevoker != nil {
				if revErr := s.criteriaRevoker.RevokeTokenFamily(txCtx, participation.TokenFamilyID); revErr != nil {
					return revErr
				}
			}
			if delErr := s.store.DeleteParticipant(txCtx, participation.SessionID, appID,
				participation.OUID); delErr != nil {
				return delErr
			}
			remaining, listErr := s.store.ListBySessionID(txCtx, participation.SessionID)
			if listErr != nil {
				return listErr
			}
			if len(remaining) > 0 {
				continue
			}
			if _, delErr := s.deleteSession(txCtx, participation.SessionID); delErr != nil {
				return delErr
			}
		}
		return nil
	}); txErr != nil {
		return fmt.Errorf("failed to remove session for the application: %w", txErr)
	}

	if s.logger.IsDebugEnabled() {
		sessionIDs := make(map[string]struct{}, len(participations))
		for _, participation := range participations {
			sessionIDs[participation.SessionID] = struct{}{}
		}
		s.logger.Debug(ctx, "Detached application from SSO sessions", log.Int("sessionCount", len(sessionIDs)))
	}
	return nil
}

// listLiveBySubject returns one page of the subject's live SSO sessions at now, most recently active first,
// each with the applications that have joined it. A non-nil ouID keeps only the sessions with an application
// signed in under that organization unit, and only those participants. The sessions carry the handle, so
// callers must never expose it.
func (s *service) listLiveBySubject(ctx context.Context, subjectID string, ouID *string, now time.Time,
	limit, offset int) (*sessionPage, error) {
	total, err := s.store.CountLiveBySubject(ctx, subjectID, ouID, now)
	if err != nil {
		return nil, fmt.Errorf("failed to count sessions by subject: %w", err)
	}
	if offset >= total {
		return &sessionPage{Sessions: []sessionDetail{}, TotalResults: total}, nil
	}
	sessions, err := s.store.ListLiveBySubject(ctx, subjectID, ouID, now, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions by subject: %w", err)
	}
	return s.page(ctx, sessions, total, ouID)
}

// listLiveByApp returns one page of the live SSO sessions at now that the application has joined, most
// recently active first, each with the applications that have joined it. A non-nil ouID keeps only the
// sessions the application joined under that organization unit, and only the participants signed in under it.
func (s *service) listLiveByApp(ctx context.Context, appID string, ouID *string, now time.Time,
	limit, offset int) (*sessionPage, error) {
	total, err := s.store.CountLiveByApp(ctx, appID, ouID, now)
	if err != nil {
		return nil, fmt.Errorf("failed to count sessions by application: %w", err)
	}
	if offset >= total {
		return &sessionPage{Sessions: []sessionDetail{}, TotalResults: total}, nil
	}
	sessions, err := s.store.ListLiveByApp(ctx, appID, ouID, now, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions by application: %w", err)
	}
	return s.page(ctx, sessions, total, ouID)
}

// getSubjectParticipantByTokenFamily returns the participant of one of the subject's SSO sessions that holds
// the token family, or nil when none does.
func (s *service) getSubjectParticipantByTokenFamily(ctx context.Context, subjectID, tokenFamilyID string) (
	*Participant, error) {
	participant, err := s.store.GetSubjectParticipantByTokenFamily(ctx, subjectID, tokenFamilyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participant by token family: %w", err)
	}
	return participant, nil
}

// ListSessions returns one page of the live sessions of a user (userID) or of an application (appID). Exactly
// one of them must be given.
func (s *service) ListSessions(ctx context.Context, userID, appID string, limit, offset int) (
	*sessionListResponse, *tidcommon.ServiceError) {
	userID = strings.TrimSpace(userID)
	appID = strings.TrimSpace(appID)
	if (userID == "") == (appID == "") {
		return nil, &ErrorInvalidListFilter
	}
	ouID, svcErr := confinedOU(ctx)
	if svcErr != nil {
		return nil, svcErr
	}

	var (
		page       *sessionPage
		err        error
		extraQuery string
	)
	now := time.Now().UTC()
	if userID != "" {
		page, err = s.listLiveBySubject(ctx, userID, ouID, now, limit, offset)
		extraQuery = "&userId=" + url.QueryEscape(userID)
	} else {
		page, err = s.listLiveByApp(ctx, appID, ouID, now, limit, offset)
		extraQuery = "&appId=" + url.QueryEscape(appID)
	}
	if err != nil {
		s.logger.Error(ctx, "Failed to list sessions", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	displays := s.subjectDisplays(ctx, page.Sessions)
	// A session permission doesn't let the caller read applications, so they are read as the runtime.
	apps := s.appDisplays(security.WithRuntimeContext(ctx), page.Sessions)
	sessions := make([]sessionResponse, 0, len(page.Sessions))
	for _, sess := range page.Sessions {
		sessions = append(sessions, toSessionResponse(sess, displays, apps))
	}

	return s.buildListResponse(sessions, page.TotalResults, "/sessions", limit, offset, extraQuery), nil
}

// ListOwnSessions returns one page of the authenticated caller's own live sessions, with the one behind the
// caller's access token marked as current.
func (s *service) ListOwnSessions(ctx context.Context, limit, offset int) (
	*sessionListResponse, *tidcommon.ServiceError) {
	subject := security.GetSubject(ctx)
	participant, tokenFamilyID, err := s.ownParticipant(ctx, subject)
	if err != nil {
		s.logger.Error(ctx, "Failed to read the caller's session participant", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if participant == nil {
		return s.buildListResponse([]sessionResponse{}, 0, "/users/me/sessions", limit, offset, ""), nil
	}

	page, err := s.listLiveBySubject(ctx, subject, &participant.OUID, time.Now().UTC(), limit, offset)
	if err != nil {
		s.logger.Error(ctx, "Failed to list own sessions", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	// The caller already knows who they are, so only the applications are resolved. The caller signed in to
	// these applications but may not be allowed to read them, so they are read as the runtime.
	apps := s.appDisplays(security.WithRuntimeContext(ctx), page.Sessions)
	sessions := make([]sessionResponse, 0, len(page.Sessions))
	for _, sess := range page.Sessions {
		resp := toSessionResponse(sess, nil, apps)
		isCurrent := isCurrentSession(sess, tokenFamilyID)
		resp.IsCurrent = &isCurrent
		sessions = append(sessions, resp)
	}

	return s.buildListResponse(sessions, page.TotalResults, "/users/me/sessions", limit, offset, ""), nil
}

// RevokeSession ends the session with the given ID. A caller without the system permission ends it only when an
// application signed in to it under the organization unit its token names. A session that is gone or out of
// reach is not an error.
func (s *service) RevokeSession(ctx context.Context, sessionID string) *tidcommon.ServiceError {
	ouID, svcErr := confinedOU(ctx)
	if svcErr != nil {
		return svcErr
	}

	if err := s.revokeSession(ctx, sessionID, "", ouID); err != nil {
		s.logger.Error(ctx, "Failed to revoke session", log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}

// RevokeUserSessions ends the user's live SSO sessions, confined as RevokeSession is.
func (s *service) RevokeUserSessions(ctx context.Context, userID string) *tidcommon.ServiceError {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return &ErrorMissingUserID
	}
	ouID, svcErr := confinedOU(ctx)
	if svcErr != nil {
		return svcErr
	}

	if err := s.revokeBySubject(ctx, userID, ouID, ""); err != nil {
		s.logger.Error(ctx, "Failed to revoke user sessions", log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}

// RevokeOwnSession ends one of the sessions the caller's own list shows. Any other session is left as it is,
// without an error.
func (s *service) RevokeOwnSession(ctx context.Context, sessionID string) *tidcommon.ServiceError {
	subject := security.GetSubject(ctx)
	participant, _, err := s.ownParticipant(ctx, subject)
	if err != nil {
		s.logger.Error(ctx, "Failed to read the caller's session participant", log.Error(err))
		return &tidcommon.InternalServerError
	}
	if participant == nil {
		return nil
	}

	if err := s.revokeSession(ctx, sessionID, subject, &participant.OUID); err != nil {
		s.logger.Error(ctx, "Failed to revoke own session", log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}

// RevokeOwnSessions ends every session the caller's own list shows, or with exceptCurrent every one but the
// session the caller's access token was issued in.
func (s *service) RevokeOwnSessions(ctx context.Context, exceptCurrent bool) *tidcommon.ServiceError {
	subject := security.GetSubject(ctx)
	participant, _, err := s.ownParticipant(ctx, subject)
	if err != nil {
		s.logger.Error(ctx, "Failed to read the caller's session participant", log.Error(err))
		return &tidcommon.InternalServerError
	}
	if participant == nil {
		if exceptCurrent {
			return &ErrorCurrentSessionUnknown
		}
		return nil
	}

	keepSessionID := ""
	if exceptCurrent {
		keepSessionID = participant.SessionID
	}
	if err := s.revokeBySubject(ctx, subject, &participant.OUID, keepSessionID); err != nil {
		s.logger.Error(ctx, "Failed to revoke own sessions", log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}

// confinedOU returns the organization unit an administrator's session request is confined to: nil when it
// holds the system permission, which reaches every unit, or else the unit its token names. The security
// middleware refuses a caller whose token names no unit, so that is reported as unauthorized.
func confinedOU(ctx context.Context) (*string, *tidcommon.ServiceError) {
	if security.HasSystemPermission(security.GetPermissions(ctx)) {
		return nil, nil
	}
	ouID := security.GetOUID(ctx)
	if ouID == "" {
		return nil, &tidcommon.ErrorUnauthorized
	}
	return &ouID, nil
}

// ownParticipant returns the participant whose token family the caller's access token carries, and that token
// family. The caller sees only the sessions with an application signed in under the organization unit this
// participant signed in under, and only those applications, so an application in one unit can't see or end
// the user's sign-ins in another. The unit is read from the participant, not from the token's claims, which
// the application's token configuration decides. A token that no participant of an SSO session holds, such as
// one from a direct session's sign-in, gets a nil participant and sees no session.
func (s *service) ownParticipant(ctx context.Context, subject string) (*Participant, string, error) {
	tokenFamilyID, _ := security.GetAttribute(ctx, oauth2const.ClaimTokenFamilyID).(string)
	if tokenFamilyID == "" {
		return nil, "", nil
	}
	participant, err := s.getSubjectParticipantByTokenFamily(ctx, subject, tokenFamilyID)
	if err != nil {
		return nil, "", err
	}
	return participant, tokenFamilyID, nil
}

// revokeSession ends the live SSO session with the given ID as Terminate does. A non-empty ownerSubjectID must
// own it, and a non-nil ouID must have an application signed in to it, though the whole session ends. Any other
// session is left as it is.
func (s *service) revokeSession(ctx context.Context, sessionID, ownerSubjectID string, ouID *string) error {
	sess, err := s.store.GetBySessionID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to load session for revocation: %w", err)
	}
	if sess == nil || !live(sess, time.Now().UTC()) || (ownerSubjectID != "" && sess.SubjectID != ownerSubjectID) {
		return nil
	}

	if err := s.terminateSessions(ctx, []Session{*sess}, ouID, TerminationReasonSessionRevocation); err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}
	s.logger.Debug(ctx, "Revoked SSO session")
	return nil
}

// revokeBySubject ends the subject's live SSO sessions as Terminate does, except keepSessionID when it is not
// empty. A non-nil ouID ends only those with an application signed in under it. Unlike TerminateBySubject it
// revokes only the sessions' grants, not every token the subject holds.
func (s *service) revokeBySubject(ctx context.Context, subjectID string, ouID *string, keepSessionID string) error {
	sessions, err := s.store.ListBySubject(ctx, subjectID)
	if err != nil {
		return fmt.Errorf("failed to list sessions by subject: %w", err)
	}

	now := time.Now().UTC()
	sessions = slices.DeleteFunc(sessions, func(sess Session) bool {
		return sess.Type != TypeSSO || sess.SessionID == keepSessionID || !live(&sess, now)
	})
	if len(sessions) == 0 {
		return nil
	}

	if err := s.terminateSessions(ctx, sessions, ouID, TerminationReasonSessionRevocation); err != nil {
		return fmt.Errorf("failed to revoke subject sessions: %w", err)
	}
	s.logger.Debug(ctx, "Revoked SSO sessions for subject", log.Int("sessionCount", len(sessions)))
	return nil
}

// subjectDisplays resolves the display name of each distinct user or agent across the sessions in one read.
// A subject that can't be found, or a failed read, leaves the name out.
func (s *service) subjectDisplays(ctx context.Context, sessions []sessionDetail) map[string]string {
	seen := make(map[string]bool, len(sessions))
	subjectIDs := make([]string, 0, len(sessions))
	for _, sess := range sessions {
		if !seen[sess.SubjectID] {
			seen[sess.SubjectID] = true
			subjectIDs = append(subjectIDs, sess.SubjectID)
		}
	}
	if len(subjectIDs) == 0 {
		return nil
	}
	subjects, err := s.entities.GetEntitiesByIDs(ctx, subjectIDs)
	if err != nil {
		s.logger.Warn(ctx, "Failed to read the session subjects for their display names", log.Error(err))
		return nil
	}
	return entitytype.ResolveEntityDisplays(ctx, subjects, s.entityTypes, s.logger)
}

// appDisplays resolves the name and logo of each distinct participating application across the sessions,
// once per application. An application that can't be found gets neither.
func (s *service) appDisplays(ctx context.Context, sessions []sessionDetail) map[string]appDisplay {
	apps := make(map[string]appDisplay)
	for _, sess := range sessions {
		for _, p := range sess.Participants {
			if _, seen := apps[p.AppID]; seen {
				continue
			}
			apps[p.AppID] = appDisplay{}
			if a, svcErr := s.apps.GetApplication(ctx, p.AppID); svcErr == nil && a != nil {
				apps[p.AppID] = appDisplay{name: a.Name, logoURL: a.LogoURL}
			}
		}
	}
	return apps
}

// isCurrentSession reports whether the session issued the caller's access token: one of its participants
// holds the token's family.
func isCurrentSession(sess sessionDetail, tokenFamilyID string) bool {
	for _, p := range sess.Participants {
		if p.TokenFamilyID == tokenFamilyID {
			return true
		}
	}
	return false
}

// buildListResponse assembles the paginated list body. basePath is the route the pagination links point
// back to, and extraQuery carries the list filter into them.
func (s *service) buildListResponse(sessions []sessionResponse, total int, basePath string, limit, offset int,
	extraQuery string) *sessionListResponse {
	return &sessionListResponse{
		TotalResults: total,
		StartIndex:   offset + 1,
		Count:        len(sessions),
		Sessions:     sessions,
		Links:        sysutils.BuildPaginationLinks(basePath, limit, offset, total, extraQuery),
	}
}

// revokeFamilies revokes each participant's token family, so signing out drops all of the login's
// grants. It is a no-op when no revoker is wired; the revoker skips an empty tfid.
func (s *service) revokeFamilies(ctx context.Context, participants []Participant) error {
	if s.criteriaRevoker == nil {
		return nil
	}
	for _, p := range participants {
		if err := s.criteriaRevoker.RevokeTokenFamily(ctx, p.TokenFamilyID); err != nil {
			return err
		}
	}
	return nil
}

// participantsForTermination returns the participants of the given sessions grouped by session id.
// It returns nil, meaning nothing to notify, when no listener is wired or the read fails.
func (s *service) participantsForTermination(ctx context.Context, sessions []Session) map[string][]Participant {
	if !s.terminationTopic.HasListeners() {
		return nil
	}
	grouped, err := s.participantsBySession(ctx, sessions)
	if err != nil {
		s.logger.Error(ctx, "Failed to read session participants before subject termination; "+
			"participants will not be notified", log.Error(err))
		return nil
	}
	return grouped
}

// participantsBySession reads the participants of the sessions in one query, grouped by session id.
func (s *service) participantsBySession(ctx context.Context, sessions []Session) (
	map[string][]Participant, error) {
	sessionIDs := make([]string, 0, len(sessions))
	for _, sess := range sessions {
		sessionIDs = append(sessionIDs, sess.SessionID)
	}
	participants, err := s.store.ListBySessionIDs(ctx, sessionIDs)
	if err != nil {
		return nil, err
	}

	grouped := make(map[string][]Participant, len(sessions))
	for _, p := range participants {
		grouped[p.SessionID] = append(grouped[p.SessionID], p)
	}
	return grouped, nil
}

// deleteSession deletes the session row with its checkpoint contexts and participants, and reports whether
// this call removed the row.
func (s *service) deleteSession(ctx context.Context, sessionID string) (bool, error) {
	removed, err := s.store.DeleteSession(ctx, sessionID)
	if err != nil {
		return false, err
	}
	if err := s.store.Delete(ctx, sessionID); err != nil {
		return false, err
	}
	if err := s.store.DeleteBySessionID(ctx, sessionID); err != nil {
		return false, err
	}
	return removed, nil
}

// page attaches the participants to a page of sessions, reading them for the whole page in one query. A
// non-nil ouID keeps only the participants signed in under that organization unit.
func (s *service) page(ctx context.Context, sessions []Session, total int, ouID *string) (*sessionPage, error) {
	sessionIDs := make([]string, 0, len(sessions))
	for _, sess := range sessions {
		sessionIDs = append(sessionIDs, sess.SessionID)
	}
	participants, err := s.store.ListBySessionIDs(ctx, sessionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to list session participants: %w", err)
	}
	bySession := make(map[string][]Participant, len(sessions))
	for _, p := range participants {
		if ouID != nil && p.OUID != *ouID {
			continue
		}
		bySession[p.SessionID] = append(bySession[p.SessionID], p)
	}

	details := make([]sessionDetail, 0, len(sessions))
	for _, sess := range sessions {
		details = append(details, sessionDetail{Session: sess, Participants: bySession[sess.SessionID]})
	}
	return &sessionPage{Sessions: details, TotalResults: total}, nil
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

	newSession := s.newSession(TypeSSO, sessionID, in.SubjectID, in.FlowID, in.FlowVersion, in.ClientInfo,
		time.Now().UTC())
	newSession.FlowExecutionID = in.ExecutionID
	newSession.HandleID = handle
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
		s.logger.Debug(ctx, "Established SSO session", log.String("flowID", in.FlowID))
	}
	return established, created, nil
}

// newSession builds an active session of the given type, created at now.
func (s *service) newSession(sessionType Type, sessionID, subjectID, flowID string, flowVersion int,
	client ClientInfo, now time.Time) Session {
	return Session{
		SessionID:       sessionID,
		Type:            sessionType,
		SubjectID:       subjectID,
		FlowID:          flowID,
		FlowVersion:     flowVersion,
		AuthenticatedAt: now,
		CreatedAt:       now,
		LastActiveAt:    now,
		// The idle deadline slides on each activity refresh; the absolute deadline is fixed here and
		// caps the session's total lifetime. The resolver rejects a session past either deadline.
		IdleExpiresAt:     now.Add(s.timeouts.Idle),
		AbsoluteExpiresAt: now.Add(s.timeouts.Absolute),
		Properties: SessionProperties{
			UserAgent:    client.UserAgent,
			CreationIP:   client.IP,
			LastActiveIP: client.IP,
		},
		State:   StateActive,
		Version: 1,
	}
}

// recordParticipant records the application as a participant of the session under the organization
// unit, refreshing its last-active time and current-grant tfid if it has joined under that unit before.
// It is a no-op when the application id is unknown.
func (s *service) recordParticipant(ctx context.Context, sessionID, appID, ouID, tokenFamilyID string,
	now time.Time) error {
	if appID == "" {
		return nil
	}
	return s.store.Record(ctx, Participant{
		SessionID:     sessionID,
		AppID:         appID,
		OUID:          ouID,
		TokenFamilyID: tokenFamilyID,
		FirstJoinedAt: now,
		LastActiveAt:  now,
	})
}

// activeIP returns the IP to record as the session's last-active IP: the requesting client's, or the
// one already recorded when the request carried none, so a request without an IP never erases a known
// address.
func (s *service) activeIP(client ClientInfo, sess *Session) string {
	if client.IP != "" {
		return client.IP
	}
	return sess.Properties.LastActiveIP
}
