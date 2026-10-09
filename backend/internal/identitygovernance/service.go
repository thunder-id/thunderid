// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package identitygovernance decides account access: admission checks, lock formation and lifecycle
// operations on identities.
package identitygovernance

import (
	"context"
	"errors"
	"time"

	governanceconfig "github.com/thunder-id/thunderid/internal/identitygovernance/config"
	"github.com/thunder-id/thunderid/internal/identitygovernance/model"

	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// formationRetries bounds the re-reads when a concurrent writer wins the lock-formation guard.
const formationRetries = 3

// IdentityGovernanceServiceInterface is the governance authority. Methods take an entity ID and read
// the entity themselves.
type IdentityGovernanceServiceInterface interface {
	// AdmitAuthenticationStep admits one verified step and, when admitted, clears that method's entry.
	AdmitAuthenticationStep(ctx context.Context, entityID string, scope model.AccessScope) (model.Decision, error)

	// RecordFailure records one failed authentication step against a scope.
	RecordFailure(ctx context.Context, entityID string, scope model.AccessScope) error

	// EvaluateAccess decides whether an identity may be used at a plane for an authentication method
	// scope. An empty entityID admits; an unreadable entity refuses.
	EvaluateAccess(ctx context.Context, entityID string, plane model.Plane,
		scope model.AccessScope) (model.Decision, error)

	// AdmitSignIn admits a completed sign-in at the session plane and, when admitted, clears an expired
	// entity-wide entry.
	AdmitSignIn(ctx context.Context, entityID string) (model.Decision, error)

	// Unlock releases every automatic lock in every scope and resets the escalation.
	Unlock(ctx context.Context, entityID string) error

	// Suspend places an administrative hold. It writes state only and does not revoke sessions or
	// tokens. The operator note is optional. On an already suspended identity it writes nothing and
	// succeeds, keeping the first placement time and note.
	Suspend(ctx context.Context, entityID, operatorNote string) error

	// ValidateSuspend runs Suspend's checks without writing.
	ValidateSuspend(ctx context.Context, entityID string) error

	// Unsuspend releases the hold. A user is left under a permanent entity-wide lock; an agent or an
	// application becomes active.
	Unsuspend(ctx context.Context, entityID string) error

	// RecordLogin stamps when access was last granted, if enabled for the entity's category.
	RecordLogin(ctx context.Context, entityID string) error
}

// EntityStateProvider reads and writes the governed state. The entity service implements it.
type EntityStateProvider interface {
	// GetGovernedEntity reads the entity with its runtime state. A failure must refuse, never admit.
	GetGovernedEntity(ctx context.Context, entityID string) (model.GovernedEntity, error)

	// GetEntityProfile reads the profile without the runtime state. An unknown identity is an error.
	GetEntityProfile(ctx context.Context, entityID string) (*providers.Entity, error)

	// IncrementFailure records one failure against a scope and returns the resulting counters. It
	// returns false when nothing was written.
	IncrementFailure(ctx context.Context, entityID string, scope model.AccessScope,
		now, windowStart time.Time) (model.ScopeLock, bool, error)

	// FormLock opens a lock episode if the counters still match observed. It returns false, not an
	// error, when they changed.
	FormLock(ctx context.Context, entityID string, scope model.AccessScope, observed model.ScopeLock,
		episode model.LockEpisode, now time.Time) (bool, error)

	// ClearAccessState removes the lock entries for the given scopes. Suspension and activity are kept.
	ClearAccessState(ctx context.Context, entityID string, scopes []model.AccessScope) error
	// ClearAccessStateIfUnchanged clears a scope's lock entry only if the revision is unchanged.
	ClearAccessStateIfUnchanged(ctx context.Context, entityID string, scope model.AccessScope,
		revision int64) (bool, error)

	// RecordLogin stamps lastLoginAt unless the stored stamp is within [notBefore, at]. A missing row
	// and a skipped write both succeed.
	RecordLogin(ctx context.Context, entityID string, at, notBefore time.Time) error

	// SetSuspension records the suspension time and the operator note, which may be empty.
	SetSuspension(ctx context.Context, entityID string, at time.Time, operatorNote string) error

	// ClearSuspension removes the suspension and writes the given hold, if any, in the same statement.
	ClearSuspension(ctx context.Context, entityID string, hold *model.EntityHold, suspendedAt string) error
}

// service is the governance authority.
type service struct {
	entityState EntityStateProvider
	policies    PolicyResolver
	// notifier is optional; locks form the same without it.
	notifier LockNotifier
	// authorizer is optional; nil permits.
	authorizer sysauthz.SystemAuthorizationServiceInterface
	// observability receives lifecycle audit events. Optional.
	observability providers.ObservabilityProvider
	logger        *log.Logger
	now           func() time.Time
}

// newService builds the governance service.
func newService(entityState EntityStateProvider, policies PolicyResolver,
	notifier LockNotifier, authorizer sysauthz.SystemAuthorizationServiceInterface) *service {
	return &service{
		entityState: entityState,
		policies:    policies,
		notifier:    notifier,
		authorizer:  authorizer,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "IdentityGovernanceService")),
		now: func() time.Time { return time.Now().UTC() },
	}
}

// EvaluateAccess returns the access decision at a plane.
func (s *service) EvaluateAccess(ctx context.Context, entityID string, plane model.Plane,
	scope model.AccessScope) (model.Decision, error) {
	entity, decided, ok, err := s.readForCheck(ctx, entityID)
	if !ok {
		return decided, err
	}
	return s.decide(ctx, entity, plane, scope), nil
}

// readForCheck reads the subject of a check. ok is false when the check is already decided: an empty
// subject admits and a read failure refuses.
func (s *service) readForCheck(ctx context.Context,
	entityID string) (model.GovernedEntity, model.Decision, bool, error) {
	if entityID == "" {
		return model.GovernedEntity{}, admit(), false, nil
	}
	entity, err := s.entityState.GetGovernedEntity(ctx, entityID)
	if err != nil {
		return model.GovernedEntity{}, refuse(model.HoldIdentity, ""), false, err
	}
	return entity, model.Decision{}, true, nil
}

// AdmitAuthenticationStep clears the step's own method entry only after admission. A clear failure is
// logged and does not change the decision.
func (s *service) AdmitAuthenticationStep(ctx context.Context, entityID string,
	scope model.AccessScope) (model.Decision, error) {
	entity, decided, ok, err := s.readForCheck(ctx, entityID)
	if !ok {
		return decided, err
	}
	decision := s.decide(ctx, entity, model.PlaneAuthentication, scope)
	if !decision.Admitted {
		return decision, nil
	}
	_, recorded := entity.AccessState.Lock.AuthenticationMethods[scope]
	if recorded && scope != model.AccessScopeEntity {
		cleared, err := s.entityState.ClearAccessStateIfUnchanged(ctx, entityID, scope, entity.Revision)
		if err != nil {
			s.logger.Warn(ctx, "Failed to clear account access state after an admitted authentication",
				log.MaskedString("entityId", entityID), log.String("scope", string(scope)), log.Error(err))
		} else if !cleared {
			s.logger.Debug(ctx, "Skipped clearing account access state: it changed after the admission read",
				log.MaskedString("entityId", entityID), log.String("scope", string(scope)))
		}
	}
	return decision, nil
}

// AdmitSignIn checks a completed sign-in and, when admitted, clears an expired entity-wide entry. A
// live entity-wide lock is never cleared.
func (s *service) AdmitSignIn(ctx context.Context, entityID string) (model.Decision, error) {
	entity, decided, ok, err := s.readForCheck(ctx, entityID)
	if !ok {
		return decided, err
	}

	decision := s.decide(ctx, entity, model.PlaneSession, "")
	entry := entity.AccessState.Lock.Entity
	if decision.Admitted && entry != nil && !isLockLive(entry.UnlockAt, s.now()) {
		cleared, err := s.entityState.ClearAccessStateIfUnchanged(ctx, entityID, model.AccessScopeEntity,
			entity.Revision)
		if err != nil {
			s.logger.Warn(ctx, "Failed to clear the entity access state after a completed sign-in",
				log.MaskedString("entityId", entityID), log.Error(err))
		} else if !cleared {
			s.logger.Debug(ctx, "Skipped clearing the entity access state: it changed after the admission read",
				log.MaskedString("entityId", entityID))
		}
	}
	return decision, nil
}

// decide returns the decision for an entity at a plane and scope.
func (s *service) decide(ctx context.Context, entity model.GovernedEntity, plane model.Plane,
	scope model.AccessScope) model.Decision {
	// A suspension refuses at every plane. The policy is read only for disclosure.
	if isSuspended(entity.State, entity.AccessState) {
		policy := s.policies.Resolve(ctx, entity, scope)
		return discloseHold(refuse(model.HoldIdentity, ""), policy.DiscloseAccountHold, model.ReasonIdentityHeld)
	}

	// Only a suspension refuses an application.
	if entity.Category == providers.EntityCategoryApp {
		return admit()
	}

	// No lock entries: admit without reading the policy.
	if entity.AccessState.Lock.Entity == nil && len(entity.AccessState.Lock.AuthenticationMethods) == 0 {
		return admit()
	}

	policy := s.policies.Resolve(ctx, entity, scope)
	effective := governanceconfig.EffectiveScopeFor(policy.Granularity, scope)
	entityWide, held := lockFor(entity.AccessState, effective, s.now())
	if !held {
		return admit()
	}

	// An entity-wide entry holds every method. A method-scoped entry holds only a lockable method.
	if !entityWide && model.ClassOf(effective) != model.ClassLockable {
		return admit()
	}

	if !s.holdsAuthenticationPlane(policy, plane, entityWide) {
		return admit()
	}

	// Report the entity scope when the entity-wide entry refused.
	refused := effective
	if entityWide {
		refused = model.AccessScopeEntity
	}

	return discloseHold(refuse(model.HoldAuthentication, refused),
		policy.DiscloseAccountHold, model.ReasonCredentialLocked)
}

// holdsAuthenticationPlane reports whether an automatic lock is enforced at this plane.
func (s *service) holdsAuthenticationPlane(policy governanceconfig.LockoutPolicy, plane model.Plane,
	entityWide bool) bool {
	if !policy.Enabled && !entityWide {
		// Disabled lockout ignores method locks. An entity-wide hold still applies, since Unsuspend
		// writes one regardless of the policy.
		return false
	}

	switch plane {
	case model.PlaneAuthentication, model.PlaneDispatch:
		return true
	case model.PlaneSession, model.PlaneIssuance, model.PlaneRecovery, model.PlaneApplication:
		// Locks do not apply here, so failed logins cannot end sessions or block recovery.
		return false
	default:
		// Unknown planes do not enforce locks.
		return false
	}
}

// RecordFailure counts a failure and forms a lock when the threshold is reached. The policy is
// resolved from the stored entity, not from the caller.
func (s *service) RecordFailure(ctx context.Context, entityID string, scope model.AccessScope) error {
	if entityID == "" {
		return nil
	}
	if !model.ClassOf(scope).Counted() {
		return nil
	}

	entity, err := s.entityState.GetGovernedEntity(ctx, entityID)
	if err != nil {
		return err
	}

	policy := s.policies.Resolve(ctx, entity, scope)
	if !policy.Enabled {
		return nil
	}
	effective := governanceconfig.EffectiveScopeFor(policy.Granularity, scope)
	now := s.now()

	// Do not count while a live lock already refuses this scope. Skipping the write also keeps a wrong
	// secret from being distinguishable by timing.
	if _, held := lockFor(entity.AccessState, effective, now); held {
		return nil
	}

	observed, recorded, err := s.entityState.IncrementFailure(
		ctx, entity.ID, effective, now, now.Add(-policy.FailureWindow))
	if err != nil {
		return err
	}
	if !recorded {
		// No row, or the store's hold guard skipped the write.
		return nil
	}

	return s.formLockIfEligible(ctx, entity, effective, policy, observed, now)
}

// formLockIfEligible opens a lock episode when the counters reach the threshold, re-reading up to
// formationRetries times when a concurrent writer wins the guard.
func (s *service) formLockIfEligible(ctx context.Context, entity model.GovernedEntity, effective model.AccessScope,
	policy governanceconfig.LockoutPolicy, observed model.ScopeLock, now time.Time) error {
	for range formationRetries {
		if !shouldFormLock(policy, observed, now) {
			return nil
		}

		episode := nextLockEpisode(policy, observed, now)

		formed, err := s.entityState.FormLock(ctx, entity.ID, effective, observed, episode, now)
		if err != nil {
			return err
		}
		if formed {
			s.logger.Debug(ctx, "Account access lock formed",
				log.MaskedString("entityId", entity.ID),
				log.String("scope", string(effective)),
				log.Int("lockCount", episode.LockCount))

			// Only the guard's winner reaches here, so one lock sends one notice. Scheduling never blocks.
			if s.notifier != nil {
				s.notifier.NotifyLockFormed(ctx, entity, effective, episode)
			}
			return nil
		}

		// The row changed. Re-read and decide again.
		refreshed, err := s.entityState.GetGovernedEntity(ctx, entity.ID)
		if err != nil {
			return err
		}
		observed = refreshed.AccessState.ObservedFor(effective)
		observed.Revision = refreshed.Revision
	}

	return nil
}

// Suspend places an administrative hold. See IdentityGovernanceServiceInterface.
func (s *service) Suspend(ctx context.Context, entityID, operatorNote string) (err error) {
	noteRecorded := operatorNote != ""
	record := lifecycleAudit{succeeded: event.EventTypeIdentitySuspended,
		failed: event.EventTypeIdentitySuspensionFailed, entityID: entityID, operatorNoteRecorded: &noteRecorded}
	defer func() { s.audit(ctx, &record, err) }()

	entity, err := s.governedForLifecycle(ctx, entityID, &record)
	if err != nil {
		return err
	}
	// Already suspended: write nothing and keep the first placement time and note.
	if isSuspended(entity.State, entity.AccessState) {
		record.message = "Suspension already in place; containment re-run"
		return nil
	}
	if err := s.entityState.SetSuspension(ctx, entityID, s.now().UTC(), operatorNote); err != nil {
		return err
	}
	record.message = "Suspension placed"
	return nil
}

// ValidateSuspend runs Suspend's checks without writing. Only a refusal is audited.
func (s *service) ValidateSuspend(ctx context.Context, entityID string) (err error) {
	record := lifecycleAudit{failed: event.EventTypeIdentitySuspensionFailed, entityID: entityID}
	defer func() {
		if err != nil {
			s.audit(ctx, &record, err)
		}
	}()

	_, err = s.governedForLifecycle(ctx, entityID, &record)
	return err
}

// governedForLifecycle reads the subject of a lifecycle operation, records its category on the audit
// and authorizes the caller.
func (s *service) governedForLifecycle(ctx context.Context, entityID string,
	record *lifecycleAudit) (model.GovernedEntity, error) {
	if entityID == "" {
		return model.GovernedEntity{}, ErrNoSubject
	}
	entity, err := s.entityState.GetGovernedEntity(ctx, entityID)
	if err != nil {
		return model.GovernedEntity{}, err
	}
	record.category = entity.Category
	return entity, s.authorize(ctx, entity)
}

// Unsuspend releases the hold and, for a user, writes a permanent entity-wide lock in the same
// statement. See IdentityGovernanceServiceInterface.
func (s *service) Unsuspend(ctx context.Context, entityID string) (err error) {
	record := lifecycleAudit{succeeded: event.EventTypeIdentityUnsuspended,
		failed: event.EventTypeIdentityUnsuspensionFailed, entityID: entityID}
	defer func() { s.audit(ctx, &record, err) }()

	entity, err := s.governedForLifecycle(ctx, entityID, &record)
	if err != nil {
		return err
	}
	if !isSuspended(entity.State, entity.AccessState) {
		return ErrNotApplicable
	}

	var hold *model.EntityHold
	if entity.Category == providers.EntityCategoryUser {
		// The permanent sentinel: this hold does not expire.
		hold = &model.EntityHold{UnlockAt: model.PermanentUnlockAt, Reason: model.ReasonPostSuspension}
	}

	observedSuspension := ""
	if entity.AccessState.Suspend != nil {
		observedSuspension = entity.AccessState.Suspend.SuspendedAt
	}
	if err := s.entityState.ClearSuspension(ctx, entityID, hold, observedSuspension); err != nil {
		return err
	}

	record.message = "Suspension released"
	if hold != nil {
		record.message = "Suspension released onto a post-suspension lock"
	}
	return nil
}

// RecordLogin stamps the last login when the entity's category has recording enabled. The store
// skips the write when the stored stamp is within the resolution window.
func (s *service) RecordLogin(ctx context.Context, entityID string) error {
	if entityID == "" {
		return nil
	}
	policies := s.policies.ActivityPolicies(ctx)
	if !policies.Enabled() {
		return nil
	}

	profile, err := s.entityState.GetEntityProfile(ctx, entityID)
	if err != nil {
		return err
	}

	policy := policies.For(profile.Category)
	if !policy.Record {
		return nil
	}

	now := s.now()
	return s.entityState.RecordLogin(ctx, entityID, now, now.Add(-policy.Resolution))
}

// Unlock clears lock entries in every scope, including the entity scope. It does not release a
// suspension.
func (s *service) Unlock(ctx context.Context, entityID string) (err error) {
	record := lifecycleAudit{succeeded: event.EventTypeIdentityUnlocked,
		failed: event.EventTypeIdentityUnlockFailed, entityID: entityID}
	defer func() { s.audit(ctx, &record, err) }()

	if _, err := s.governedForLifecycle(ctx, entityID, &record); err != nil {
		return err
	}
	if err := s.entityState.ClearAccessState(ctx, entityID, model.AllAccessScopes); err != nil {
		return err
	}
	record.message = "Every automatic lock released"
	return nil
}

// lifecycleAudit holds what one lifecycle operation records: event types, target and outcome.
type lifecycleAudit struct {
	succeeded, failed providers.EventType
	entityID          string
	category          providers.EntityCategory
	message           string
	// operatorNoteRecorded says whether a suspension carried a note. The note is never recorded.
	operatorNoteRecorded *bool
}

// audit logs one lifecycle operation and publishes it as an event. The actor is the caller's subject,
// left empty under a runtime context.
func (s *service) audit(ctx context.Context, record *lifecycleAudit, err error) {
	eventType, status := record.succeeded, providers.StatusSuccess
	if err != nil {
		eventType, status = record.failed, providers.StatusFailure
	}
	actor := ""
	if !security.IsRuntimeContext(ctx) {
		actor = security.GetSubject(ctx)
	}

	fields := []log.Field{log.String("event", string(eventType)), log.String("outcome", status),
		log.MaskedString("entityId", record.entityID), log.MaskedString("actor", actor),
		log.Bool("selfService", security.IsRuntimeContext(ctx))}
	if err != nil {
		fields = append(fields, log.String("failure", failureClass(err)), log.Error(err))
	}
	s.logger.Info(ctx, "Identity lifecycle operation", fields...)

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
	if err != nil {
		evt.WithData(event.DataKey.Error, failureClass(err))
	} else if record.message != "" {
		evt.WithData(event.DataKey.Message, record.message)
	}
	s.observability.PublishEvent(ctx, evt)
}

// failureClass maps a lifecycle error to a short class name, without store detail.
func failureClass(err error) string {
	switch {
	case errors.Is(err, ErrNotAuthorized):
		return "not_authorized"
	case errors.Is(err, ErrNotApplicable):
		return "not_applicable"
	case errors.Is(err, ErrNoSubject):
		return "no_subject"
	default:
		return "operation_failed"
	}
}

// authorize checks that the caller in ctx may update the subject. It applies to lifecycle operations
// only. A nil authorizer permits.
func (s *service) authorize(ctx context.Context, entity model.GovernedEntity) error {
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
		return ErrNotAuthorized
	}
	allowed, svcErr := s.authorizer.IsActionAllowed(ctx, action, &sysauthz.ActionContext{
		ResourceType: resourceType, OUID: entity.OUID, ResourceID: entity.ID,
	})
	if svcErr != nil {
		s.logger.Error(ctx, "Could not decide whether the caller may govern this subject",
			log.MaskedString("entityId", entity.ID), log.Any("error", svcErr))
		return ErrNotAuthorized
	}
	if !allowed {
		s.logger.Debug(ctx, "Refused a lifecycle operation outside the caller's boundary",
			log.MaskedString("entityId", entity.ID),
			log.String("category", string(entity.Category)))
		return ErrNotAuthorized
	}
	return nil
}

// LockNotifier sends a notice when an automatic lock forms.
type LockNotifier interface {
	// NotifyLockFormed schedules a notice for a committed lock. It must not block or affect the
	// authentication response.
	NotifyLockFormed(ctx context.Context, entity model.GovernedEntity,
		scope model.AccessScope, episode model.LockEpisode)
}

func admit() model.Decision {
	return model.Decision{Admitted: true, HoldLevel: model.HoldNone}
}

func refuse(level model.HoldLevel, scope model.AccessScope) model.Decision {
	return model.Decision{Admitted: false, HoldLevel: level, Scope: string(scope)}
}

// discloseHold sets the refusal reason when the policy allows disclosing the hold.
func discloseHold(decision model.Decision, mode governanceconfig.DisclosureMode,
	reason string) model.Decision {
	if mode != governanceconfig.DiscloseAlways {
		return decision
	}
	decision.Reason = reason
	return decision
}

// PolicyResolver resolves the lockout policy for an entity and authentication method. When the
// configuration cannot be read, the deployment policy applies.
type PolicyResolver interface {
	Resolve(ctx context.Context, entity model.GovernedEntity, scope model.AccessScope) governanceconfig.LockoutPolicy

	// ActivityPolicies returns the login-recording policy for every category.
	ActivityPolicies(ctx context.Context) governanceconfig.ActivityPolicies
}
