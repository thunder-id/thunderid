// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package identitygovernance decides account access: admission checks, lock formation, lifecycle
// operations on identities, the accountAccess server-config section and the lock email.
package identitygovernance

import (
	"context"
	"time"

	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// IdentityGovernanceServiceInterface is the governance authority. Methods take an entity ID and read
// the entity themselves.
type IdentityGovernanceServiceInterface interface {
	// AdmitAuthenticationStep admits one verified step and, when admitted, clears that method's entry.
	AdmitAuthenticationStep(ctx context.Context, entityID string,
		method providers.AuthenticationMethod) (providers.AccessDecision, *tidcommon.ServiceError)

	// RecordFailure records one failed authentication step against a method, made at a plane. Where a
	// lock applies to the method, it decides the attempt from the same read it counts from: a refused
	// decision is returned without counting. Anywhere else nothing is read and the attempt is admitted.
	// Only a failed read is an error: a failed write leaves the attempt uncounted.
	RecordFailure(ctx context.Context, entityID string, plane providers.AccessPlane,
		method providers.AuthenticationMethod) (providers.AccessDecision, *tidcommon.ServiceError)

	// EvaluateAccess decides whether an identity may be used at a plane for an authentication method.
	// An empty entityID admits; an unreadable entity refuses.
	EvaluateAccess(ctx context.Context, entityID string, plane providers.AccessPlane,
		method providers.AuthenticationMethod) (providers.AccessDecision, *tidcommon.ServiceError)

	// IsSuspended reports whether the identity is suspended, and writes nothing. A missing runtime row
	// reads as the starting state. An empty entityID, or one with no local identity, is not suspended;
	// an unreadable entity is an error.
	IsSuspended(ctx context.Context, entityID string) (bool, *tidcommon.ServiceError)

	// AdmitSignIn admits a completed sign-in at the session plane and, when admitted, clears an expired
	// entity-wide entry.
	AdmitSignIn(ctx context.Context, entityID string) (providers.AccessDecision, *tidcommon.ServiceError)

	// Unlock releases every automatic lock in every method and resets the escalation.
	Unlock(ctx context.Context, entityID string) *tidcommon.ServiceError

	// Suspend places an administrative hold. It writes state only and does not revoke sessions or
	// tokens. The operator note is optional. On an already suspended identity it writes nothing and
	// succeeds, keeping the first placement time and note.
	Suspend(ctx context.Context, entityID, operatorNote string) *tidcommon.ServiceError

	// ValidateSuspend runs Suspend's checks without writing.
	ValidateSuspend(ctx context.Context, entityID, operatorNote string) *tidcommon.ServiceError

	// Unsuspend releases the hold. A user is left under a permanent entity-wide lock; an agent keeps
	// any automatic lock still in force, and an application becomes active.
	Unsuspend(ctx context.Context, entityID string) *tidcommon.ServiceError

	// RecordLogin stamps when access was last granted, if enabled for the entity's category.
	RecordLogin(ctx context.Context, entityID string) *tidcommon.ServiceError

	// ReadEntityAccess derives the effective status and last login time for one loaded entity.
	ReadEntityAccess(ctx context.Context, entity providers.Entity) AccessRead

	// ReadAccess derives what management reads report for each entity, in order: the effective status
	// and the last login time. Each category's lockout policy is resolved once per call.
	ReadAccess(ctx context.Context, entities []providers.Entity) []AccessRead

	// SetDependencyRegistry injects the registry that names the identities suspended together with another
	SetDependencyRegistry(registry resourcedependency.Registry)
}

// entityStateProvider reads and writes the governed state. The entity service implements it.
type entityStateProvider interface {
	// GetGovernedEntity reads the entity with its runtime state. A missing runtime row reads as the
	// starting state; seedMissing also creates it, for a caller that may write next. It returns nil
	// when no identity has the ID, which access checks admit as having nothing to govern. Any error
	// must refuse, never admit.
	GetGovernedEntity(ctx context.Context, entityID string, seedMissing bool) (*GovernedEntity, error)

	// GetEntityProfile reads the profile without the runtime state. An unknown identity is an error.
	GetEntityProfile(ctx context.Context, entityID string) (*providers.Entity, error)

	// IncrementFailure records one failure against a method and returns the resulting counters. It
	// returns false when nothing was written.
	IncrementFailure(ctx context.Context, entityID string, method providers.AuthenticationMethod,
		now, windowStart time.Time) (LockEntry, bool, error)

	// FormLock opens a lock episode if the counters still match observed. It returns false, not an
	// error, when they changed.
	FormLock(ctx context.Context, entityID string, method providers.AuthenticationMethod, observed LockEntry,
		episode LockEpisode, now time.Time) (bool, error)

	// ClearAccessState removes the lock entries for the given methods. Suspension and activity are kept.
	ClearAccessState(ctx context.Context, entityID string, methods []providers.AuthenticationMethod) error

	// ClearAccessStateIfUnchanged clears a method's lock entry only if the revision is unchanged.
	ClearAccessStateIfUnchanged(ctx context.Context, entityID string, method providers.AuthenticationMethod,
		revision int64) (bool, error)

	// RecordLogin stamps lastLoginAt unless the stored stamp is within [notBefore, at]. A missing row
	// and a skipped write both succeed.
	RecordLogin(ctx context.Context, entityID string, at, notBefore time.Time) error

	// SetSuspension records the suspension time and the operator note, which may be empty. It
	// returns false, not an error, when the entity is already suspended.
	SetSuspension(ctx context.Context, entityID string, at time.Time, operatorNote string) (bool, error)

	// ClearSuspension removes the suspension placed at suspendedAt and writes the given hold, if any,
	// in the same statement. It returns false, not an error, when that suspension is no longer there.
	ClearSuspension(ctx context.Context, entityID string, hold *EntityHold, suspendedAt string) (bool, error)

	// SetOwnerSuspension records the owner's suspension and when it was placed. It returns false, not an
	// error, when the owner's suspension is already in place.
	SetOwnerSuspension(ctx context.Context, entityID string, at time.Time) (bool, error)

	// ClearOwnerSuspension removes the owner's suspension. It returns false, not an error, when there is
	// none.
	ClearOwnerSuspension(ctx context.Context, entityID string) (bool, error)

	// SetCascadeSuspension records the suspension carried from the identity this one depends on. It returns
	// false, not an error, when it is already in place.
	SetCascadeSuspension(ctx context.Context, entityID string, at time.Time) (bool, error)

	// ClearCascadeSuspension removes the cascaded suspension. It returns false, not an error, when there is none.
	ClearCascadeSuspension(ctx context.Context, entityID string) (bool, error)
}

// identityGovernanceService is the governance authority.
type identityGovernanceService struct {
	entityState entityStateProvider
	source      accountAccessSourceInterface
	// notifier is optional; locks form the same without it.
	notifier LockNotifierInterface
	// authorizer is optional; nil permits.
	authorizer sysauthz.SystemAuthorizationServiceInterface
	// observability receives lifecycle audit events. Optional.
	observability providers.ObservabilityProvider
	// dependencyRegistry names the identities suspended together with another. Optional.
	dependencyRegistry resourcedependency.Registry
	logger             *log.Logger
	now                func() time.Time
}

// newIdentityGovernanceService builds the governance service.
func newIdentityGovernanceService(entityState entityStateProvider, source accountAccessSourceInterface,
	notifier LockNotifierInterface, authorizer sysauthz.SystemAuthorizationServiceInterface,
	observability providers.ObservabilityProvider) *identityGovernanceService {
	return &identityGovernanceService{
		entityState:   entityState,
		source:        source,
		notifier:      notifier,
		authorizer:    authorizer,
		observability: observability,
		logger:        log.GetLogger().With(log.String(log.LoggerKeyComponentName, "IdentityGovernanceService")),
		now:           func() time.Time { return time.Now().UTC() },
	}
}

// AdmitAuthenticationStep clears the step's own method entry only after admission. A clear failure is
// logged and does not change the decision.
func (s *identityGovernanceService) AdmitAuthenticationStep(ctx context.Context, entityID string,
	method providers.AuthenticationMethod) (providers.AccessDecision, *tidcommon.ServiceError) {
	entity, decided, ok, svcErr := s.readForCheck(ctx, entityID)
	if !ok {
		return decided, svcErr
	}
	decision := s.decide(ctx, entity, providers.AccessPlaneAuthentication, method)
	if !decision.Admitted {
		return decision, nil
	}
	_, recorded := entity.AccessState.Lock.AuthenticationMethods[method]
	if recorded && method != AuthenticationMethodEntity {
		cleared, err := s.entityState.ClearAccessStateIfUnchanged(ctx, entityID, method, entity.Revision)
		if err != nil {
			s.logger.Warn(ctx, "Failed to clear account access state after an admitted authentication",
				log.MaskedString(log.LoggerKeyEntityID, entityID), log.String("authenticationMethod",
					string(method)), log.Error(err))
		} else if !cleared {
			s.logger.Debug(ctx, "Skipped clearing account access state: it changed after the admission read",
				log.MaskedString(log.LoggerKeyEntityID, entityID), log.String("authenticationMethod", string(method)))
		}
	}
	return decision, nil
}

// RecordFailure decides a failed attempt and, when it is admitted, counts it and forms a lock when the
// threshold is reached. The policy is resolved from the stored entity, not from the caller.
func (s *identityGovernanceService) RecordFailure(ctx context.Context, entityID string,
	plane providers.AccessPlane,
	method providers.AuthenticationMethod) (providers.AccessDecision, *tidcommon.ServiceError) {
	// A failure is counted only where a lock limits guessing the method.
	if entityID == "" || !isLockable(method) || !lockAppliesAt(plane, method) {
		return admit(), nil
	}

	entity, decided, ok, svcErr := s.readForCheck(ctx, entityID)
	if !ok {
		return decided, svcErr
	}

	// A held identity is refused without counting, as a verified one would be.
	policy := s.resolvePolicy(ctx, entity, method)
	if decision := s.decideUnder(ctx, entity, plane, method, policy); !decision.Admitted || !policy.enabled {
		return decision, nil
	}

	// A failed write is logged and leaves the attempt uncounted; it never changes the answer.
	s.countFailure(ctx, entity, method, policy)
	return admit(), nil
}

// EvaluateAccess returns the access decision at a plane.
func (s *identityGovernanceService) EvaluateAccess(ctx context.Context, entityID string,
	plane providers.AccessPlane,
	method providers.AuthenticationMethod) (providers.AccessDecision, *tidcommon.ServiceError) {
	entity, decided, ok, svcErr := s.readForCheck(ctx, entityID)
	if !ok {
		return decided, svcErr
	}
	return s.decide(ctx, entity, plane, method), nil
}

// IsSuspended reports whether the identity is suspended, from a read that writes nothing.
func (s *identityGovernanceService) IsSuspended(ctx context.Context,
	entityID string) (bool, *tidcommon.ServiceError) {
	if entityID == "" {
		return false, nil
	}
	entity, err := s.entityState.GetGovernedEntity(ctx, entityID, false)
	if err != nil {
		return false, s.serverError(ctx, "Failed to read the identity for a suspension check", entityID, err)
	}
	if entity == nil {
		return false, nil
	}

	return isSuspended(entity.State, entity.AccessState), nil
}

// AdmitSignIn checks a completed sign-in and, when admitted, clears an expired entity-wide entry. A
// live entity-wide lock is never cleared.
func (s *identityGovernanceService) AdmitSignIn(ctx context.Context,
	entityID string) (providers.AccessDecision, *tidcommon.ServiceError) {
	entity, decided, ok, svcErr := s.readForCheck(ctx, entityID)
	if !ok {
		return decided, svcErr
	}

	decision := s.decide(ctx, entity, providers.AccessPlaneSession, "")
	entry := entity.AccessState.Lock.Entity
	if decision.Admitted && entry != nil && !isLockLive(entry.UnlockAt, s.now()) {
		cleared, err := s.entityState.ClearAccessStateIfUnchanged(ctx, entityID, AuthenticationMethodEntity,
			entity.Revision)
		if err != nil {
			s.logger.Warn(ctx, "Failed to clear the entity access state after a completed sign-in",
				log.MaskedString(log.LoggerKeyEntityID, entityID), log.Error(err))
		} else if !cleared {
			s.logger.Debug(ctx, "Skipped clearing the entity access state: it changed after the admission read",
				log.MaskedString(log.LoggerKeyEntityID, entityID))
		}
	}
	return decision, nil
}

// Unlock clears lock entries in every method, including the entity-wide key. It does not release a
// suspension. A runtime-context caller is a recovery flow, so its unlock is recorded as self-service.
func (s *identityGovernanceService) Unlock(ctx context.Context, entityID string) (svcErr *tidcommon.ServiceError) {
	record := lifecycleAudit{succeeded: event.EventTypeIdentityUnlocked,
		failed: event.EventTypeIdentityUnlockFailed, entityID: entityID}
	if security.IsRuntimeContext(ctx) {
		record.succeeded, record.failed = event.EventTypeIdentitySelfUnlocked, event.EventTypeIdentitySelfUnlockFailed
	}
	defer func() { s.audit(ctx, &record, svcErr) }()

	if _, svcErr = s.governedForLifecycle(ctx, entityID, &record); svcErr != nil {
		return svcErr
	}
	if err := s.entityState.ClearAccessState(ctx, entityID, AllAuthenticationMethods); err != nil {
		return s.serverError(ctx, "Failed to release account locks", entityID, err)
	}
	record.message = messageLocksReleased
	return nil
}

// Suspend places a hold, or writes nothing when one is already in place. A call marked with
// WithOwnerApplier places the owner's hold; any other places the organization's. The two are held apart.
func (s *identityGovernanceService) Suspend(ctx context.Context,
	entityID, operatorNote string) (svcErr *tidcommon.ServiceError) {
	noteRecorded := operatorNote != ""
	appliedBy := suspensionApplier(ctx)
	record := lifecycleAudit{succeeded: event.EventTypeIdentitySuspended,
		failed: event.EventTypeIdentitySuspensionFailed, entityID: entityID, operatorNoteRecorded: &noteRecorded,
		appliedBy: appliedBy}
	defer func() { s.audit(ctx, &record, svcErr) }()

	entity, svcErr := s.governedForLifecycle(ctx, entityID, &record)
	if svcErr != nil {
		return svcErr
	}
	if isOperatorNoteTooLong(operatorNote) {
		return &ErrorOperatorNoteTooLong
	}
	if appliedBy == providers.SuspensionAppliedByOwner {
		return s.suspendByOwner(ctx, entity, &record)
	}
	// The identities suspended together with this one are suspended first
	if svcErr := s.cascadeSuspension(ctx, entity, true); svcErr != nil {
		return svcErr
	}
	// Already suspended: write nothing and keep the first placement time and note.
	if isSuspendedByOrganization(entity.State, entity.AccessState) {
		record.message = messageSuspensionInPlace
		return nil
	}
	placed, err := s.entityState.SetSuspension(ctx, entityID, s.now(), operatorNote)
	if err != nil {
		return s.serverError(ctx, "Failed to place a suspension", entityID, err)
	}
	// A concurrent suspend placed it first; that placement and its note are kept.
	record.message = messageSuspensionPlaced
	if !placed {
		record.message = messageSuspensionInPlace
	}
	return nil
}

// suspendByOwner places the owner's hold, or writes nothing when it is already in place.
func (s *identityGovernanceService) suspendByOwner(ctx context.Context, entity GovernedEntity,
	record *lifecycleAudit) *tidcommon.ServiceError {
	if entity.AccessState.OwnerSuspend != nil {
		record.message = messageOwnerSuspensionInPlace
		return nil
	}
	placed, err := s.entityState.SetOwnerSuspension(ctx, entity.ID, s.now())
	if err != nil {
		return s.serverError(ctx, "Failed to place the owner's suspension", entity.ID, err)
	}
	// A concurrent suspend by the owner placed it first.
	record.message = messageOwnerSuspensionPlaced
	if !placed {
		record.message = messageOwnerSuspensionInPlace
	}
	return nil
}

// ValidateSuspend runs Suspend's checks without writing. Only a refusal is audited.
func (s *identityGovernanceService) ValidateSuspend(ctx context.Context,
	entityID, operatorNote string) (svcErr *tidcommon.ServiceError) {
	record := lifecycleAudit{failed: event.EventTypeIdentitySuspensionFailed, entityID: entityID}
	defer func() {
		if svcErr != nil {
			s.audit(ctx, &record, svcErr)
		}
	}()

	if _, svcErr = s.governedForLifecycle(ctx, entityID, &record); svcErr != nil {
		return svcErr
	}
	if isOperatorNoteTooLong(operatorNote) {
		return &ErrorOperatorNoteTooLong
	}
	return nil
}

// Unsuspend releases the hold and, for a user, writes a permanent entity-wide lock in the same
// statement.
func (s *identityGovernanceService) Unsuspend(ctx context.Context,
	entityID string) (svcErr *tidcommon.ServiceError) {
	appliedBy := suspensionApplier(ctx)
	record := lifecycleAudit{succeeded: event.EventTypeIdentityUnsuspended,
		failed: event.EventTypeIdentityUnsuspensionFailed, entityID: entityID, appliedBy: appliedBy}
	defer func() { s.audit(ctx, &record, svcErr) }()

	entity, svcErr := s.governedForLifecycle(ctx, entityID, &record)
	if svcErr != nil {
		return svcErr
	}
	if appliedBy == providers.SuspensionAppliedByOwner {
		return s.unsuspendByOwner(ctx, entity, &record)
	}
	// The organization lifts only its own hold; the owner's, if any, stays.
	if !isSuspendedByOrganization(entity.State, entity.AccessState) {
		return &ErrorNotApplicable
	}
	// The identities suspended together with this one are released first.
	if svcErr := s.cascadeSuspension(ctx, entity, false); svcErr != nil {
		return svcErr
	}

	var hold *EntityHold
	if entity.Category == providers.EntityCategoryUser {
		// The permanent sentinel: this hold does not expire.
		hold = &EntityHold{UnlockAt: permanentUnlockAt, Reason: lockReasonPostSuspension}
	}

	observedSuspension := ""
	if entity.AccessState.Suspend != nil {
		observedSuspension = entity.AccessState.Suspend.SuspendedAt
	}
	cleared, err := s.entityState.ClearSuspension(ctx, entityID, hold, observedSuspension)
	if err != nil {
		return s.serverError(ctx, "Failed to release a suspension", entityID, err)
	}
	if !cleared {
		// A concurrent unsuspend released it, or a new suspension replaced it, after the read.
		return &ErrorNotApplicable
	}

	record.message = messageSuspensionReleased
	if hold != nil {
		record.message = messageSuspensionReleasedOntoHold
	}
	return nil
}

// SetDependencyRegistry injects the dependency registry. Called by servicemanager after the
// provider services are initialized to avoid a cyclic import.
func (s *identityGovernanceService) SetDependencyRegistry(r resourcedependency.Registry) {
	s.dependencyRegistry = r
}

// cascadeSuspension suspends, or with suspend false unsuspends, every identity suspended together with entity, as
// the dependency registry names them. Each change, or a failure to make it, is audited as that identity's own
// suspension or unsuspension, applied by the cascade. A dependent that no longer exists, or already holds the hold as
// asked, is left as it is.
func (s *identityGovernanceService) cascadeSuspension(ctx context.Context, entity GovernedEntity,
	suspend bool) *tidcommon.ServiceError {
	// Only a user has identities suspended together with it
	if s.dependencyRegistry == nil || entity.Category != providers.EntityCategoryUser {
		return nil
	}
	dependents, err := s.dependencyRegistry.SuspensionDependents(ctx, resourcedependency.ResourceTypeUser, entity.ID)
	if err != nil {
		return s.serverError(ctx, "Failed to list the identities suspended together", entity.ID, err)
	}
	for _, dependentID := range dependents {
		// The governance read seeds a missing runtime row, so the write has a row to update.
		dependent, err := s.entityState.GetGovernedEntity(ctx, dependentID, true)
		if err != nil {
			return s.serverError(ctx, "Failed to read a dependent identity", dependentID, err)
		}
		if dependent == nil || (dependent.AccessState.CascadeSuspend != nil) == suspend {
			continue
		}

		record := lifecycleAudit{succeeded: event.EventTypeIdentitySuspended,
			failed: event.EventTypeIdentitySuspensionFailed, entityID: dependentID, category: dependent.Category,
			message: messageCascadedSuspensionPlaced, appliedBy: providers.SuspensionAppliedByCascade}
		var changed bool
		if suspend {
			changed, err = s.entityState.SetCascadeSuspension(ctx, dependentID, s.now())
		} else {
			record.succeeded, record.failed = event.EventTypeIdentityUnsuspended,
				event.EventTypeIdentityUnsuspensionFailed
			record.message = messageCascadedSuspensionReleased
			changed, err = s.entityState.ClearCascadeSuspension(ctx, dependentID)
		}
		if err != nil {
			svcErr := s.serverError(ctx, "Failed to carry a suspension to a dependent identity", dependentID, err)
			s.audit(ctx, &record, svcErr)
			return svcErr
		}
		if changed {
			s.audit(ctx, &record, nil)
		}
	}
	return nil
}

// unsuspendByOwner lifts the owner's hold. The organization's, if any, stays.
func (s *identityGovernanceService) unsuspendByOwner(ctx context.Context, entity GovernedEntity,
	record *lifecycleAudit) *tidcommon.ServiceError {
	if entity.AccessState.OwnerSuspend == nil {
		return &ErrorNotApplicable
	}
	cleared, err := s.entityState.ClearOwnerSuspension(ctx, entity.ID)
	if err != nil {
		return s.serverError(ctx, "Failed to release the owner's suspension", entity.ID, err)
	}
	// A concurrent unsuspend by the owner released it after the read.
	if !cleared {
		return &ErrorNotApplicable
	}
	record.message = messageOwnerSuspensionReleased
	return nil
}

// RecordLogin stamps the last login when the entity's category has recording enabled. The store
// skips the write when the stored stamp is within the resolution window.
func (s *identityGovernanceService) RecordLogin(ctx context.Context, entityID string) *tidcommon.ServiceError {
	if entityID == "" {
		return nil
	}
	policies := s.resolveActivityPolicies(ctx)
	if !policies.enabled() {
		return nil
	}

	profile, err := s.entityState.GetEntityProfile(ctx, entityID)
	if err != nil {
		// The profile read does not name a missing identity, so the governed read tells one with no
		// local record, which has nothing to stamp, apart from a failed read.
		if governed, govErr := s.entityState.GetGovernedEntity(ctx, entityID, true); govErr == nil && governed == nil {
			return nil
		}
		return s.serverError(ctx, "Failed to read the identity to record a login", entityID, err)
	}

	policy := policies.forCategory(profile.Category)
	if !policy.record {
		return nil
	}

	now := s.now()
	if err := s.entityState.RecordLogin(ctx, entityID, now, now.Add(-policy.resolution)); err != nil {
		return s.serverError(ctx, "Failed to record a login", entityID, err)
	}
	return nil
}

// ReadEntityAccess derives the access facts for one entity. See IdentityGovernanceServiceInterface.
func (s *identityGovernanceService) ReadEntityAccess(ctx context.Context, entity providers.Entity) AccessRead {
	return accessReadFor(&entity, s.enforcementFor(ctx, entity.Category), s.now())
}

// ReadAccess derives the access facts management reads report. See IdentityGovernanceServiceInterface.
func (s *identityGovernanceService) ReadAccess(ctx context.Context, entities []providers.Entity) []AccessRead {
	now := s.now()
	enforcements := make(map[providers.EntityCategory]lockEnforcement)
	reads := make([]AccessRead, len(entities))
	for i := range entities {
		entity := &entities[i]
		enforcement, resolved := enforcements[entity.Category]
		if !resolved {
			enforcement = s.enforcementFor(ctx, entity.Category)
			enforcements[entity.Category] = enforcement
		}
		reads[i] = accessReadFor(entity, enforcement, now)
	}
	return reads
}

// enforcementFor resolves which recorded locks a category's policy enforces. An application has no
// lockout policy, so it gets the zero enforcement, which reports every recorded lock, without a read.
func (s *identityGovernanceService) enforcementFor(ctx context.Context,
	category providers.EntityCategory) lockEnforcement {
	if category == providers.EntityCategoryApp {
		return lockEnforcement{}
	}
	return lockEnforcementFor(s.source.Current(ctx), category)
}

// readForCheck reads the subject of a check. ok is false when the check is already decided: an empty
// subject, or one with no local identity, admits, since there is no hold to apply; any other read
// failure refuses.
func (s *identityGovernanceService) readForCheck(ctx context.Context,
	entityID string) (GovernedEntity, providers.AccessDecision, bool, *tidcommon.ServiceError) {
	if entityID == "" {
		return GovernedEntity{}, admit(), false, nil
	}
	entity, err := s.entityState.GetGovernedEntity(ctx, entityID, true)
	if err != nil {
		return GovernedEntity{}, providers.AccessDecision{}, false,
			s.serverError(ctx, "Failed to read the identity for an access check", entityID, err)
	}
	if entity == nil {
		// For example an identity an external authentication provider asserts with no local record.
		s.logger.Debug(ctx, "No local identity to govern; admitted", log.MaskedString(log.LoggerKeyEntityID, entityID))
		return GovernedEntity{}, admit(), false, nil
	}

	return *entity, providers.AccessDecision{}, true, nil
}

// decide returns the decision for an entity at a plane and method. An identity with nothing that could
// hold it, and an application that is not suspended, is admitted without reading the policy.
func (s *identityGovernanceService) decide(ctx context.Context, entity GovernedEntity, plane providers.AccessPlane,
	method providers.AuthenticationMethod) providers.AccessDecision {
	if !isSuspended(entity.State, entity.AccessState) &&
		(entity.Category == providers.EntityCategoryApp || !hasLockEntries(entity)) {
		return admit()
	}
	return s.decideUnder(ctx, entity, plane, method, s.resolvePolicy(ctx, entity, method))
}

// decideUnder returns the decision for an entity at a plane and method under a resolved policy.
func (s *identityGovernanceService) decideUnder(ctx context.Context, entity GovernedEntity,
	plane providers.AccessPlane, method providers.AuthenticationMethod, policy lockoutPolicy) providers.AccessDecision {
	// A suspension refuses at every plane.
	if isSuspended(entity.State, entity.AccessState) {
		return refuse(policy.discloseAccountHold, providers.AccessRefusalReasonSuspended)
	}

	// Only a suspension refuses an application.
	if entity.Category == providers.EntityCategoryApp {
		return admit()
	}

	if !hasLockEntries(entity) {
		return admit()
	}

	effective := effectiveMethodFor(policy.granularity, method)
	entityWide, held := lockFor(entity.AccessState, effective, s.now())
	if !held {
		return admit()
	}

	// An entity-wide entry holds every method. A method entry holds only a lockable method.
	if !entityWide && !isLockable(effective) {
		return admit()
	}

	// An entity-wide entry is decided under the entity-wide policy, as the status read reports it.
	if entityWide && effective != AuthenticationMethodEntity {
		policy = s.resolvePolicy(ctx, entity, AuthenticationMethodEntity)
	}

	postSuspension := entityWide && entity.AccessState.Lock.Entity.Reason == lockReasonPostSuspension
	if !holdsAuthenticationPlane(policy, plane, method, postSuspension) {
		return admit()
	}

	return refuse(policy.discloseAccountHold, providers.AccessRefusalReasonLocked)
}

// countFailure records one failure against the effective method and forms a lock when the threshold is
// reached. A store failure is logged and leaves the attempt uncounted.
func (s *identityGovernanceService) countFailure(ctx context.Context, entity GovernedEntity,
	method providers.AuthenticationMethod, policy lockoutPolicy) {
	effective := effectiveMethodFor(policy.granularity, method)
	now := s.now()

	// A zero window never resets the counter on time, so the window starts at the zero time.
	var windowStart time.Time
	if policy.failureWindow > 0 {
		windowStart = now.Add(-policy.failureWindow)
	}
	observed, recorded, err := s.entityState.IncrementFailure(ctx, entity.ID, effective, now, windowStart)
	if err != nil {
		s.logStoreFailure(ctx, "Failed to record a failed attempt", entity.ID, err)
		return
	}
	if !recorded {
		// No row, or the store's hold guard skipped the write.
		return
	}

	s.formLockIfEligible(ctx, entity, effective, policy, observed, now)
}

// formLockIfEligible opens a lock episode when the counters reach the threshold, re-reading up to
// formationRetries times when a concurrent writer wins the guard. A store failure is logged and stops
// the attempt; the recorded failures stay.
func (s *identityGovernanceService) formLockIfEligible(ctx context.Context, entity GovernedEntity,
	effective providers.AuthenticationMethod, policy lockoutPolicy, observed LockEntry, now time.Time) {
	for range formationRetries {
		if !shouldFormLock(policy, observed, now) {
			return
		}

		episode := nextLockEpisode(policy, observed, now)

		formed, err := s.entityState.FormLock(ctx, entity.ID, effective, observed, episode, now)
		if err != nil {
			s.logStoreFailure(ctx, "Failed to form an account lock", entity.ID, err)
			return
		}
		if formed {
			// Only the guard's winner reaches here, so one lock is recorded once and sends one notice.
			// Scheduling never blocks.
			s.auditLockFormed(ctx, entity, effective, episode)
			if s.notifier != nil {
				s.notifier.NotifyLockFormed(ctx, entity, effective, episode)
			}
			return
		}

		// The row changed. Re-read and decide again.
		refreshed, err := s.entityState.GetGovernedEntity(ctx, entity.ID, true)
		if err != nil {
			s.logStoreFailure(ctx, "Failed to re-read the identity to form an account lock", entity.ID, err)
			return
		}
		if refreshed == nil {
			// Deleted after the failure was counted; there is nothing left to lock.
			return
		}
		observed = refreshed.AccessState.observedFor(effective)
		observed.Revision = refreshed.Revision
	}
}

// resolvePolicy returns the lockout policy for the entity's category at a method. Under entity
// granularity the entity-wide policy applies, not the presented method's.
func (s *identityGovernanceService) resolvePolicy(ctx context.Context, entity GovernedEntity,
	method providers.AuthenticationMethod) lockoutPolicy {
	category := categoryConfig(s.source.Current(ctx), entity.Category)
	return policyFor(category, parseGranularity(sysutils.DerefOr(category.LockGranularity, "")), method)
}

// resolveActivityPolicies resolves every category's login-recording settings from one section read.
func (s *identityGovernanceService) resolveActivityPolicies(ctx context.Context) activityPolicies {
	accountAccess := s.source.Current(ctx)
	return activityPolicies{
		user:  activityPolicyFor(categoryConfig(accountAccess, providers.EntityCategoryUser)),
		agent: activityPolicyFor(categoryConfig(accountAccess, providers.EntityCategoryAgent)),
	}
}

// governedForLifecycle reads the subject of a lifecycle operation, records its category on the audit
// and authorizes the caller.
func (s *identityGovernanceService) governedForLifecycle(ctx context.Context, entityID string,
	record *lifecycleAudit) (GovernedEntity, *tidcommon.ServiceError) {
	if entityID == "" {
		return GovernedEntity{}, &ErrorMissingSubject
	}
	entity, err := s.entityState.GetGovernedEntity(ctx, entityID, true)
	if err != nil {
		return GovernedEntity{}, s.serverError(ctx, "Failed to read the identity for a lifecycle operation",
			entityID, err)
	}
	if entity == nil {
		return GovernedEntity{}, &ErrorEntityNotFound
	}
	record.category = entity.Category

	return *entity, s.authorize(ctx, *entity)
}

// serverError logs a store or read failure and returns the internal server error.
func (s *identityGovernanceService) serverError(ctx context.Context, message, entityID string,
	err error) *tidcommon.ServiceError {
	s.logStoreFailure(ctx, message, entityID, err)
	return &tidcommon.InternalServerError
}

// logStoreFailure logs a store or read failure once, with the masked subject.
func (s *identityGovernanceService) logStoreFailure(ctx context.Context, message, entityID string, err error) {
	s.logger.Error(ctx, message, log.MaskedString(log.LoggerKeyEntityID, entityID), log.Error(err))
}

// authorize checks that the caller in ctx may update the subject. It applies to lifecycle operations
// only. A nil authorizer permits.
func (s *identityGovernanceService) authorize(ctx context.Context, entity GovernedEntity) *tidcommon.ServiceError {
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
			log.MaskedString(log.LoggerKeyEntityID, entity.ID), log.String("errorCode", svcErr.Code))
		return &tidcommon.InternalServerError
	}
	if !allowed {
		return &tidcommon.ErrorUnauthorized
	}
	return nil
}

// audit publishes one lifecycle operation as an event. The actor is the caller's subject, left empty
// under a runtime context.
func (s *identityGovernanceService) audit(ctx context.Context, record *lifecycleAudit, svcErr *tidcommon.ServiceError) {
	eventType, status := record.succeeded, providers.StatusSuccess
	if svcErr != nil {
		eventType, status = record.failed, providers.StatusFailure
	}
	actor := ""
	if !security.IsRuntimeContext(ctx) {
		actor = security.GetSubject(ctx)
	}

	if s.logger.IsDebugEnabled() {
		fields := []log.Field{log.String("event", string(eventType)), log.String("outcome", status),
			log.MaskedString(log.LoggerKeyEntityID, record.entityID), log.String("category", string(record.category)),
			log.MaskedString("actor", actor), log.Bool("selfService", security.IsRuntimeContext(ctx))}
		if svcErr != nil {
			fields = append(fields, log.String("failure", string(failureClass(svcErr))))
		}
		s.logger.Debug(ctx, "Identity lifecycle operation", fields...)
	}

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
	if record.appliedBy != "" {
		evt.WithData(event.DataKey.AppliedBy, string(record.appliedBy))
	}
	if svcErr != nil {
		evt.WithData(event.DataKey.Error, string(failureClass(svcErr)))
	} else if record.message != "" {
		evt.WithData(event.DataKey.Message, record.message)
	}
	s.observability.PublishEvent(ctx, evt)
}

// auditLockFormed records one committed lock formation as a log line and, when observability is on, an
// event. Failed authentication is the trigger, so there is no actor.
func (s *identityGovernanceService) auditLockFormed(ctx context.Context, entity GovernedEntity,
	method providers.AuthenticationMethod, episode LockEpisode) {
	permanent := episode.UnlockAt == permanentUnlockAt
	if s.logger.IsDebugEnabled() {
		s.logger.Debug(ctx, "Account access lock formed",
			log.String("event", string(event.EventTypeIdentityLocked)),
			log.MaskedString(log.LoggerKeyEntityID, entity.ID),
			log.String("category", string(entity.Category)), log.String("authenticationMethod", string(method)),
			log.Int("lockCount", episode.LockCount), log.String("unlockAt", episode.UnlockAt))
	}

	if s.observability == nil || !s.observability.IsEnabled() {
		return
	}
	evt := event.NewEvent(syscontext.GetTraceID(ctx), string(event.EventTypeIdentityLocked),
		event.ComponentIdentityGovernance).
		WithStatus(providers.StatusSuccess).
		WithData(event.DataKey.Subject, entity.ID).
		WithData(event.DataKey.LockMethod, string(method)).
		WithData(event.DataKey.LockCount, episode.LockCount).
		WithData(event.DataKey.LockPermanent, permanent)
	if subjectType := event.PrincipalType(string(entity.Category)); subjectType != "" {
		evt.WithData(event.DataKey.SubjectType, subjectType)
	}
	if !permanent {
		evt.WithData(event.DataKey.UnlockAt, episode.UnlockAt)
	}
	s.observability.PublishEvent(ctx, evt)
}
