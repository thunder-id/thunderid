// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"context"
	"slices"
	"time"

	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// IsAccessScope reports whether scope is registered, including AccessScopeEntity.
func IsAccessScope(scope providers.AccessScope) bool {
	return slices.Contains(AllAccessScopes, scope)
}

// isLockable reports whether failures at a scope are counted and may form a lock. An unregistered
// scope is not lockable.
func isLockable(scope providers.AccessScope) bool {
	return lockableScopes[scope]
}

// isSuspended reports a suspension recorded in either the state column or the access-state document.
func isSuspended(state providers.EntityState, accessState AccessState) bool {
	return state == providers.EntityStateSuspended || accessState.Suspend != nil
}

// isLockLive reports whether a stored unlockAt is permanent or still in the future.
func isLockLive(unlockAt string, now time.Time) bool {
	if unlockAt == "" {
		return false
	}
	if unlockAt == PermanentUnlockAt {
		return true
	}
	return now.UTC().Format(time.RFC3339) < unlockAt
}

// lockFor reports whether anything holds scope, ignoring configured granularity: a live entity-wide
// lock holds every method, else a live lock on scope holds it. The first return is true when the
// entity-wide entry holds.
func lockFor(a AccessState, scope providers.AccessScope, now time.Time) (bool, bool) {
	if a.Lock.Entity != nil && isLockLive(a.Lock.Entity.UnlockAt, now) {
		return true, true
	}
	entry, recorded := a.Lock.AuthenticationMethods[scope]
	if recorded && isLockLive(entry.UnlockAt, now) {
		return false, true
	}
	return false, false
}

// shouldFormLock reports whether the observed counters reach the threshold. No lock forms while one is
// live, so failures during a lock cannot escalate it.
func shouldFormLock(policy lockoutPolicy, observed ScopeLock,
	now time.Time) bool {
	if !policy.enabled || policy.threshold <= 0 || len(policy.lockDurations) == 0 {
		return false
	}
	if isLockLive(observed.UnlockAt, now) {
		return false
	}
	return observed.FailureCount >= policy.threshold
}

// nextLockEpisode returns the lock count and unlockAt for a lock about to form. A zero duration means
// a permanent lock. Call it only after shouldFormLock returns true, which guarantees a duration.
func nextLockEpisode(policy lockoutPolicy, observed ScopeLock,
	now time.Time) LockEpisode {
	lockCount := nextLockCount(policy, observed, now)

	// lockCount is one-based; the last duration repeats.
	index := lockCount - 1
	if index >= len(policy.lockDurations) {
		index = len(policy.lockDurations) - 1
	}
	duration := policy.lockDurations[index]

	unlockAt := PermanentUnlockAt
	if duration > 0 {
		unlockAt = now.UTC().Add(duration).Format(time.RFC3339)
	}

	return LockEpisode{LockCount: lockCount, UnlockAt: unlockAt, Duration: duration}
}

// nextLockCount returns the consecutive lockout number. It resets to 1 once lockDecay has passed
// since the previous lock ended.
func nextLockCount(policy lockoutPolicy, observed ScopeLock, now time.Time) int {
	if observed.UnlockAt == "" {
		return 1
	}
	// A permanent lock never decays.
	if observed.UnlockAt == PermanentUnlockAt {
		return observed.LockCount + 1
	}
	if policy.lockDecay > 0 {
		decayedBy := now.UTC().Add(-policy.lockDecay).Format(time.RFC3339)
		if observed.UnlockAt < decayedBy {
			return 1
		}
	}
	if observed.LockCount <= 0 {
		return 1
	}
	return observed.LockCount + 1
}

// holdsAuthenticationPlane reports whether an automatic lock is enforced at this plane.
func holdsAuthenticationPlane(policy lockoutPolicy, plane providers.AccessPlane,
	postSuspension bool) bool {
	if !policy.enabled && !postSuspension {
		// Disabled lockout ignores failure locks. The post-suspension hold still applies, since
		// Unsuspend writes one regardless of the policy.
		return false
	}

	switch plane {
	case providers.AccessPlaneAuthentication, providers.AccessPlaneDispatch:
		return true
	default:
		// Locks never apply at the session, issuance, recovery or application planes, so failed
		// logins cannot end sessions or block recovery. Unknown planes do not enforce locks.
		return false
	}
}

// admit returns the decision that allows access.
func admit() providers.AccessDecision {
	return providers.AccessDecision{Admitted: true}
}

// refuse returns the decision that withholds access, naming the hold only when the mode discloses it.
func refuse(mode disclosureMode, reason providers.AccessRefusalReason) providers.AccessDecision {
	if mode != discloseAlways {
		return providers.AccessDecision{}
	}
	return providers.AccessDecision{Reason: reason}
}

// authorize checks that the caller in ctx may update the subject. It applies to lifecycle operations
// only. A nil authorizer permits.
func (s *service) authorize(ctx context.Context, entity GovernedEntity) *tidcommon.ServiceError {
	if s.authorizer == nil {
		return nil
	}

	var action security.Action
	var resourceType security.ResourceType
	switch entity.Category {
	case providers.EntityCategoryUser:
		action, resourceType = security.ActionUpdateUser, security.ResourceTypeUser
	case providers.EntityCategoryAgent:
		action, resourceType = security.ActionUpdateAgent, security.ResourceTypeAgent
	default:
		// Applications have no system authorization resource.
		return nil
	}
	// A non-runtime caller may not govern itself.
	if !security.IsRuntimeContext(ctx) && security.GetSubject(ctx) == entity.ID {
		return &tidcommon.ErrorUnauthorized
	}
	allowed, svcErr := s.authorizer.IsActionAllowed(ctx, action, &sysauthz.ActionContext{
		ResourceType: resourceType, OUID: entity.OUID, ResourceID: entity.ID,
	})
	if svcErr != nil {
		s.logger.Error(ctx, "Failed to authorize a lifecycle operation",
			log.MaskedString("entityId", entity.ID), log.String("code", svcErr.Code))
		return &tidcommon.InternalServerError
	}
	if !allowed {
		return &tidcommon.ErrorUnauthorized
	}
	return nil
}

// audit publishes one lifecycle operation as an event. The actor is the caller's subject, left empty
// under a runtime context.
func (s *service) audit(ctx context.Context, record *lifecycleAudit, svcErr *tidcommon.ServiceError) {
	eventType, status := record.succeeded, providers.StatusSuccess
	if svcErr != nil {
		eventType, status = record.failed, providers.StatusFailure
	}
	actor := ""
	if !security.IsRuntimeContext(ctx) {
		actor = security.GetSubject(ctx)
	}

	fields := []log.Field{log.String("event", string(eventType)), log.String("outcome", status),
		log.MaskedString("entityId", record.entityID), log.String("category", string(record.category)),
		log.MaskedString("actor", actor), log.Bool("selfService", security.IsRuntimeContext(ctx))}
	if svcErr != nil {
		fields = append(fields, log.String("failure", failureClass(svcErr)))
	}
	s.logger.Debug(ctx, "Identity lifecycle operation", fields...)

	if s.observability == nil || !s.observability.IsEnabled() {
		return
	}
	evt := event.NewEvent(syscontext.GetTraceID(ctx), string(eventType), event.ComponentIdentityGovernance).
		WithStatus(status).
		WithData(event.DataKey.Subject, record.entityID)
	if subjectType := event.PrincipalType(string(record.category)); subjectType != "" {
		evt.WithData(event.DataKey.SubjectType, subjectType)
	}
	if actor != "" {
		evt.WithData(event.DataKey.ActorSub, actor)
	}
	if record.operatorNoteRecorded != nil {
		evt.WithData(event.DataKey.OperatorNoteRecorded, *record.operatorNoteRecorded)
	}
	if svcErr != nil {
		evt.WithData(event.DataKey.Error, failureClass(svcErr))
	} else if record.message != "" {
		evt.WithData(event.DataKey.Message, record.message)
	}
	s.observability.PublishEvent(ctx, evt)
}

// failureClass maps a lifecycle error to a short class name, without store detail.
func failureClass(svcErr *tidcommon.ServiceError) string {
	switch svcErr.Code {
	case tidcommon.ErrorUnauthorized.Code:
		return "not_authorized"
	case ErrorNotApplicable.Code:
		return "not_applicable"
	case ErrorMissingSubject.Code:
		return "no_subject"
	default:
		return "operation_failed"
	}
}
