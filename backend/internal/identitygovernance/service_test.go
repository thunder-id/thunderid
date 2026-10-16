// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/observabilityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

type ServiceTestSuite struct {
	suite.Suite
	port     *EntityStateProviderMock
	policies *policyResolverInterfaceMock
	svc      *service
	ctx      context.Context
	now      time.Time
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (s *ServiceTestSuite) SetupTest() {
	s.port = NewEntityStateProviderMock(s.T())
	s.policies = newPolicyResolverInterfaceMock(s.T())
	s.now = time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	s.svc = newTestService(s.port, accountAccessValue{}, nil, nil)
	s.svc.policies = s.policies
	s.svc.now = func() time.Time { return s.now }
	s.ctx = context.Background()
}

// newTestService builds the service over a fixed configuration and no server-config reader.
func newTestService(port EntityStateProvider, cfg accountAccessValue,
	notifier lockNotifierInterface, authorizer sysauthz.SystemAuthorizationServiceInterface) *service {
	return newService(port, newAccountAccessSource(cfg, nil), notifier, authorizer, nil)
}

// requireServiceError checks that svcErr is the expected service error.
func (s *ServiceTestSuite) requireServiceError(svcErr *tidcommon.ServiceError, want tidcommon.ServiceError) {
	s.T().Helper()
	s.Require().NotNil(svcErr)
	s.Require().Equal(want.Code, svcErr.Code)
}

const testScope = providers.AccessScopeCredential

func (s *ServiceTestSuite) at(offset time.Duration) string {
	return s.now.Add(offset).UTC().Format(time.RFC3339)
}

// activeEntity is an ordinary user with no lock state.
func (s *ServiceTestSuite) activeEntity() GovernedEntity {
	return GovernedEntity{
		ID:          "e1",
		Category:    providers.EntityCategoryUser,
		Type:        "person",
		State:       providers.EntityStateActive,
		AccessState: AccessState{},
	}
}

// authenticationMethodPolicy is the default shape: per-authentication-method granularity, disclosing
// the hold.
func authenticationMethodPolicy() lockoutPolicy {
	return lockoutPolicy{
		enabled:             true,
		threshold:           5,
		failureWindow:       15 * time.Minute,
		lockDurations:       []time.Duration{5 * time.Minute, 15 * time.Minute},
		lockDecay:           24 * time.Hour,
		granularity:         granularityAuthenticationMethod,
		discloseAccountHold: discloseAlways,
	}
}

func (s *ServiceTestSuite) expectPolicy(policy lockoutPolicy) {
	s.policies.On("Resolve", mock.Anything, mock.Anything, mock.Anything).
		Return(policy).Maybe()
}

func (s *ServiceTestSuite) expectRead(entity GovernedEntity) {
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Maybe()
}

// An empty subject, such as a client-credentials grant with no user, is admitted.
func (s *ServiceTestSuite) TestUngovernedSubjectIsAdmitted() {
	decision, err := s.svc.EvaluateAccess(s.ctx, "", providers.AccessPlaneIssuance, "")
	s.Require().Nil(err)
	s.True(decision.Admitted)
}

func (s *ServiceTestSuite) TestUnreadableAuthorityRefuses() {
	readErr := errors.New("database unavailable")
	s.port.On("GetGovernedEntity", mock.Anything, "e1").Return(GovernedEntity{}, readErr).Once()

	decision, err := s.svc.EvaluateAccess(s.ctx, "e1", providers.AccessPlaneAuthentication, testScope)
	s.requireServiceError(err, tidcommon.InternalServerError)
	s.False(decision.Admitted, "a read failure must not admit")
	s.Empty(decision.Reason)
}

func (s *ServiceTestSuite) TestIdentityHoldAppliesAtEveryPlane() {
	entity := s.activeEntity()
	entity.State = providers.EntityStateSuspended
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	for _, plane := range []providers.AccessPlane{
		providers.AccessPlaneAuthentication, providers.AccessPlaneDispatch, providers.AccessPlaneRecovery,
		providers.AccessPlaneSession, providers.AccessPlaneIssuance, providers.AccessPlaneApplication,
	} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testScope)
		s.Require().Nil(err)
		s.False(decision.Admitted, "plane %s must refuse a suspended identity", plane)
		s.Equal(providers.AccessRefusalReasonSuspended, decision.Reason)
	}
}

// Only SUSPENDED holds; any other or empty state is not read as a suspension.
func (s *ServiceTestSuite) TestAStateOtherThanSuspendedIsNotAHold() {
	for _, state := range []providers.EntityState{"SOME_FUTURE_STATE", ""} {
		entity := s.activeEntity()
		entity.ID = "state-" + string(state)
		entity.State = state
		s.expectRead(entity)
		s.expectPolicy(authenticationMethodPolicy())

		for _, plane := range []providers.AccessPlane{
			providers.AccessPlaneAuthentication, providers.AccessPlaneDispatch, providers.AccessPlaneRecovery,
			providers.AccessPlaneSession, providers.AccessPlaneIssuance, providers.AccessPlaneApplication,
		} {
			decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testScope)
			s.Require().Nil(err)
			s.True(decision.Admitted, "state %q must not hold at plane %s", state, plane)
		}
	}
}

// A suspension member holds whatever the state column says.
func (s *ServiceTestSuite) TestASuspensionMemberHoldsUnderAnyState() {
	entity := s.activeEntity()
	entity.State = providers.EntityState("SOME_FUTURE_STATE")
	entity.AccessState.Suspend = &SuspendState{SuspendedAt: s.at(-time.Hour)}
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testScope)
	s.Require().Nil(err)
	s.False(decision.Admitted)
	s.Equal(providers.AccessRefusalReasonSuspended, decision.Reason)
}

// A method lock refuses at the authentication plane as a lock.
func (s *ServiceTestSuite) TestCredentialHoldRefusesAtTheCredentialPlane() {
	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, testScope, ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testScope)
	s.Require().Nil(err)
	s.False(decision.Admitted)
	s.Equal(providers.AccessRefusalReasonLocked, decision.Reason)
}

// A locked password does not hold a passkey.
func (s *ServiceTestSuite) TestALockedScopeDoesNotHoldAnother() {
	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, testScope, ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID,
		providers.AccessPlaneAuthentication, providers.AccessScopePasskey)
	s.Require().Nil(err)
	s.True(decision.Admitted, "a locked password must not hold the passkey")
}

func (s *ServiceTestSuite) TestALockExpiresWithoutAWrite() {
	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, testScope, ScopeLock{LockCount: 1, UnlockAt: s.at(-time.Second)})
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testScope)
	s.Require().Nil(err)
	s.True(decision.Admitted)
}

// A method lock does not reach the session, issuance or recovery planes.
func (s *ServiceTestSuite) TestACredentialHoldDoesNotReachOtherPlanesByDefault() {
	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, testScope, ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	for _, plane := range []providers.AccessPlane{
		providers.AccessPlaneSession, providers.AccessPlaneIssuance, providers.AccessPlaneRecovery,
		providers.AccessPlaneApplication,
	} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testScope)
		s.Require().Nil(err)
		s.True(decision.Admitted, "plane %s must admit a locked but unsuspended identity", plane)
	}
}

// Under entity granularity one entry under the reserved key holds every method.
func (s *ServiceTestSuite) TestEntityGranularityHoldsEveryAuthenticationMethod() {
	policy := authenticationMethodPolicy()
	policy.granularity = granularityEntity

	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, AccessScopeEntity, ScopeLock{
		LockCount: 1, UnlockAt: s.at(5 * time.Minute),
	})
	s.expectRead(entity)
	s.expectPolicy(policy)

	for _, scope := range []providers.AccessScope{
		testScope,
		providers.AccessScopePasskey,
		providers.AccessScopeOTP,
		providers.AccessScopeMagicLink,
	} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, scope)
		s.Require().Nil(err)
		s.False(decision.Admitted, "scope %s must be held by the entity-wide lock", scope)
	}
}

func (s *ServiceTestSuite) TestEntityGranularityAloneDoesNotReachSessionsOrIssuance() {
	policy := authenticationMethodPolicy()
	policy.granularity = granularityEntity

	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, AccessScopeEntity, ScopeLock{
		LockCount: 1, UnlockAt: s.at(5 * time.Minute),
	})
	s.expectRead(entity)
	s.expectPolicy(policy)

	for _, plane := range []providers.AccessPlane{providers.AccessPlaneSession, providers.AccessPlaneIssuance} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testScope)
		s.Require().Nil(err)
		s.True(decision.Admitted, "plane %s must stay admitted with the flags unset", plane)
	}
}

// An entity-wide lock holds only the authentication and dispatch planes, under either granularity.
func (s *ServiceTestSuite) TestAnEntityWideLockHoldsOnlyTheCredentialAndDispatchPlanes() {
	policy := authenticationMethodPolicy()
	policy.granularity = granularityEntity

	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, AccessScopeEntity, ScopeLock{
		LockCount: 1, UnlockAt: s.at(5 * time.Minute),
	})
	s.expectRead(entity)
	s.expectPolicy(policy)

	for _, plane := range []providers.AccessPlane{providers.AccessPlaneAuthentication, providers.AccessPlaneDispatch} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testScope)
		s.Require().Nil(err)
		s.False(decision.Admitted, "plane %s must refuse under a live entity-wide lock", plane)
	}

	for _, plane := range []providers.AccessPlane{
		providers.AccessPlaneSession, providers.AccessPlaneIssuance, providers.AccessPlaneRecovery,
		providers.AccessPlaneApplication,
	} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testScope)
		s.Require().Nil(err)
		s.True(decision.Admitted, "plane %s must never be held by a lock", plane)
	}

	// It lapses by the clock with no write.
	s.svc.now = func() time.Time { return s.now.Add(10 * time.Minute) }
	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testScope)
	s.Require().Nil(err)
	s.True(decision.Admitted, "a lock refuses; it does not revoke")
}

// A failure on a scope whose class is not counted is not recorded.
func (s *ServiceTestSuite) TestFailureAtANotLockableScopeIsNotRecorded() {
	err := s.svc.RecordFailure(s.ctx, s.activeEntity().ID, providers.AccessScopePasskey)
	s.Require().Nil(err)
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	// The entity is not read.
}

// A failure with no subject is not an error and reads nothing.
func (s *ServiceTestSuite) TestFailureWithNoSubjectRecordsNothing() {
	s.Require().Nil(s.svc.RecordFailure(s.ctx, "", testScope))
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// The policy is resolved from the entity the service reads, not from the caller.
func (s *ServiceTestSuite) TestRecordingResolvesThePolicyFromItsOwnRead() {
	stored := s.activeEntity()
	stored.Category = providers.EntityCategoryAgent
	stored.Type = "service"
	s.expectRead(stored)

	var resolved GovernedEntity
	s.policies.On("Resolve", mock.Anything, mock.Anything, testScope).
		Run(func(args mock.Arguments) { resolved, _ = args.Get(1).(GovernedEntity) }).
		Return(authenticationMethodPolicy()).Once()
	s.port.On("IncrementFailure", mock.Anything, stored.ID, testScope, mock.Anything, mock.Anything).
		Return(ScopeLock{FailureCount: 1}, true, nil).Once()

	s.Require().Nil(s.svc.RecordFailure(s.ctx, stored.ID, testScope))
	s.Equal(stored, resolved)
}

// The counter resets once the last failure is older than the window.
func (s *ServiceTestSuite) TestTheWindowStartIsTheWindowBeforeNow() {
	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testScope, s.now, s.now.Add(-15*time.Minute)).
		Return(ScopeLock{FailureCount: 1}, true, nil).Once()

	s.Require().Nil(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
}

// A zero window passes the zero time, so no failure is ever outside the window.
func (s *ServiceTestSuite) TestAZeroWindowNeverResetsTheCounterOnTime() {
	policy := authenticationMethodPolicy()
	policy.failureWindow = 0

	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectPolicy(policy)
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testScope, s.now, time.Time{}).
		Return(ScopeLock{FailureCount: 5}, true, nil).Once()
	s.port.On("FormLock", mock.Anything, entity.ID, testScope, mock.Anything, mock.Anything, s.now).
		Return(true, nil).Once()

	s.Require().Nil(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
	s.port.AssertExpectations(s.T())
}

// A read failure is returned, not swallowed.
func (s *ServiceTestSuite) TestRecordingReportsAnUnreadableAuthority() {
	s.port.On("GetGovernedEntity", mock.Anything, "e1").
		Return(GovernedEntity{}, errors.New("entity store unavailable")).Once()

	s.Require().NotNil(s.svc.RecordFailure(s.ctx, "e1", testScope))
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A machine credential failure is not read, counted or locked.
func (s *ServiceTestSuite) TestMachineCredentialScopeNeitherCountsNorLocks() {
	entity := s.activeEntity()
	scope := providers.AccessScopeSystemCredential

	s.Require().Nil(s.svc.RecordFailure(s.ctx, entity.ID, scope))

	s.port.AssertNotCalled(s.T(), "GetGovernedEntity", mock.Anything, mock.Anything)
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	s.port.AssertNotCalled(s.T(), "FormLock",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestFailureBelowTheThresholdFormsNoLock() {
	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testScope, mock.Anything, mock.Anything).
		Return(ScopeLock{FailureCount: 4}, true, nil).Once()

	s.Require().Nil(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
	s.port.AssertNotCalled(s.T(), "FormLock",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// At the threshold a lock forms with the escalation's duration, guarded on the observed counters.
func (s *ServiceTestSuite) TestFailureAtTheThresholdFormsALock() {
	entity := s.activeEntity()
	observed := ScopeLock{FailureCount: 5}
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testScope, mock.Anything, mock.Anything).
		Return(observed, true, nil).Once()

	var written LockEpisode
	var guard ScopeLock
	s.port.On("FormLock", mock.Anything, entity.ID, testScope,
		mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			guard, _ = args.Get(3).(ScopeLock)
			written, _ = args.Get(4).(LockEpisode)
		}).Return(true, nil).Once()

	s.Require().Nil(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
	s.Equal(1, written.LockCount)
	s.Equal(s.now.Add(5*time.Minute).UTC().Format(time.RFC3339), written.UnlockAt)
	s.Equal(observed, guard, "the formation must be guarded on what the increment observed")
}

func (s *ServiceTestSuite) TestFailureDuringALiveLockFormsNothing() {
	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testScope, mock.Anything, mock.Anything).
		Return(ScopeLock{FailureCount: 9, LockCount: 1, UnlockAt: s.at(5 * time.Minute)}, true, nil).
		Once()

	s.Require().Nil(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
	s.port.AssertNotCalled(s.T(), "FormLock",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestFormationStopsWhenAReReadShowsANewerLock() {
	entity := s.activeEntity()
	s.expectPolicy(authenticationMethodPolicy())
	// The first read, then the re-read after the guard loses.
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Once()
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testScope, mock.Anything, mock.Anything).
		Return(ScopeLock{FailureCount: 5}, true, nil).Once()
	s.port.On("FormLock", mock.Anything, entity.ID, testScope,
		mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Once()

	refreshed := s.activeEntity()
	putScopeLock(&refreshed.AccessState, testScope, ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(refreshed, nil).Once()

	s.Require().Nil(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
}

func (s *ServiceTestSuite) TestFormationRetryIsGuardedOnTheReReadRevision() {
	entity := s.activeEntity()
	s.expectPolicy(authenticationMethodPolicy())
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Once()
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testScope, mock.Anything, mock.Anything).
		Return(ScopeLock{FailureCount: 5, Revision: 3}, true, nil).Once()
	s.port.On("FormLock", mock.Anything, entity.ID, testScope,
		ScopeLock{FailureCount: 5, Revision: 3}, mock.Anything, mock.Anything).Return(false, nil).Once()

	refreshed := s.activeEntity()
	refreshed.Revision = 9
	putScopeLock(&refreshed.AccessState, testScope, ScopeLock{FailureCount: 6})
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(refreshed, nil).Once()
	s.port.On("FormLock", mock.Anything, entity.ID, testScope,
		ScopeLock{FailureCount: 6, Revision: 9}, mock.Anything, mock.Anything).Return(true, nil).Once()

	s.Require().Nil(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
}

func (s *ServiceTestSuite) TestFormationStopsWhenAReReadShowsAReset() {
	entity := s.activeEntity()
	s.expectPolicy(authenticationMethodPolicy())
	// Both reads see a row with no entries.
	s.expectRead(entity)
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testScope, mock.Anything, mock.Anything).
		Return(ScopeLock{FailureCount: 5}, true, nil).Once()
	s.port.On("FormLock", mock.Anything, entity.ID, testScope,
		mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Once()

	s.Require().Nil(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
}

func (s *ServiceTestSuite) TestFormationRetriesAreBounded() {
	entity := s.activeEntity()
	s.expectPolicy(authenticationMethodPolicy())
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testScope, mock.Anything, mock.Anything).
		Return(ScopeLock{FailureCount: 5}, true, nil).Once()

	// The guard never holds and the row keeps looking eligible.
	s.port.On("FormLock", mock.Anything, entity.ID, testScope,
		mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Times(formationRetries)
	eligible := s.activeEntity()
	putScopeLock(&eligible.AccessState, testScope, ScopeLock{FailureCount: 5})
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(eligible, nil)

	s.Require().Nil(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
}

func (s *ServiceTestSuite) TestFailureOnAMissingRowIsNotAnError() {
	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testScope, mock.Anything, mock.Anything).
		Return(ScopeLock{}, false, nil).Once()

	s.Require().Nil(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
	s.port.AssertNotCalled(s.T(), "FormLock",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestFailureUnderEntityGranularityCountsUnderTheReservedKey() {
	policy := authenticationMethodPolicy()
	policy.granularity = granularityEntity

	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectPolicy(policy)
	s.port.On("IncrementFailure", mock.Anything, entity.ID,
		AccessScopeEntity, mock.Anything, mock.Anything).
		Return(ScopeLock{FailureCount: 1}, true, nil).Once()

	s.Require().Nil(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
}

func (s *ServiceTestSuite) TestFailureWindowIsPassedAsAnInstant() {
	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	var windowStart time.Time
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testScope, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { windowStart, _ = args.Get(4).(time.Time) }).
		Return(ScopeLock{FailureCount: 1}, true, nil).Once()

	s.Require().Nil(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
	s.Equal(s.now.Add(-15*time.Minute), windowStart)
}

// An admitted step clears only its own method's entry, not the entity entry.
func (s *ServiceTestSuite) TestAdmittedStepClearsOnlyObservedMethodEntry() {
	entity := s.activeEntity()
	entity.Revision = 7
	putScopeLock(&entity.AccessState, testScope, ScopeLock{FailureCount: 2})
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Once()
	s.expectPolicy(authenticationMethodPolicy())
	s.port.On("ClearAccessStateIfUnchanged", mock.Anything, entity.ID, testScope, int64(7)).Return(true, nil).Once()
	decision, err := s.svc.AdmitAuthenticationStep(s.ctx, entity.ID, testScope)
	s.Require().Nil(err)
	s.True(decision.Admitted)
}

// An admitted step leaves the reserved entry for AdmitSignIn.
func (s *ServiceTestSuite) TestAdmittedStepNeverClearsTheReservedKey() {
	s.expectRead(s.activeEntity())
	decision, err := s.svc.AdmitAuthenticationStep(s.ctx, "e1", AccessScopeEntity)
	s.Require().Nil(err)
	s.True(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "ClearAccessStateIfUnchanged",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestDisabledPolicyAdmitsAndWritesNothing() {
	policy := authenticationMethodPolicy()
	policy.enabled = false

	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectPolicy(policy)

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testScope)
	s.Require().Nil(err, "a switched-off policy is not an error")
	s.True(decision.Admitted)

	s.Require().Nil(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestDisabledPolicyReleasesAnExistingLock() {
	policy := authenticationMethodPolicy()
	policy.enabled = false

	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, testScope, ScopeLock{LockCount: 3, UnlockAt: s.at(24 * time.Hour)})
	s.expectRead(entity)
	s.expectPolicy(policy)

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testScope)
	s.Require().Nil(err)
	s.True(decision.Admitted, "a lock must not outlive the policy that formed it being switched off")
}

func (s *ServiceTestSuite) TestDisabledPolicyStillEnforcesAnIdentityHold() {
	policy := authenticationMethodPolicy()
	policy.enabled = false

	entity := s.activeEntity()
	entity.State = providers.EntityStateSuspended
	s.expectRead(entity)
	s.expectPolicy(policy)

	for _, plane := range []providers.AccessPlane{
		providers.AccessPlaneAuthentication, providers.AccessPlaneSession, providers.AccessPlaneIssuance,
	} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testScope)
		s.Require().Nil(err)
		s.False(decision.Admitted, "plane %s must still refuse a suspended identity", plane)
		s.Equal(providers.AccessRefusalReasonSuspended, decision.Reason)
	}
}

func (s *ServiceTestSuite) TestResetOnACleanAccountWritesNothing() {
	s.expectRead(s.activeEntity())

	s.expectPolicy(authenticationMethodPolicy())
	decision, err := s.svc.AdmitAuthenticationStep(s.ctx, "e1", testScope)
	s.Require().Nil(err)
	s.True(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "ClearAccessStateIfUnchanged",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestResetIgnoresStateUnderAnUnrelatedScope() {
	other := s.activeEntity()
	putScopeLock(&other.AccessState, providers.AccessScopeOTP, ScopeLock{FailureCount: 3})
	s.expectRead(other)

	s.expectPolicy(authenticationMethodPolicy())
	decision, err := s.svc.AdmitAuthenticationStep(s.ctx, "e1", testScope)
	s.Require().Nil(err)
	s.True(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "ClearAccessStateIfUnchanged",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestAFailedSuspendReportsTheFailure() {
	s.expectRead(s.activeEntity())
	s.port.On("SetSuspension", mock.Anything, "e1", s.now, "").
		Return(false, errors.New("database unavailable")).Once()

	s.requireServiceError(s.svc.Suspend(s.ctx, "e1", ""), tidcommon.InternalServerError)
}

// Suspend writes the state column and the document in one statement.
func (s *ServiceTestSuite) TestSuspendWritesOneAtomicSuspension() {
	s.expectRead(s.activeEntity())
	var order []string
	s.port.On("SetSuspension", mock.Anything, "e1", s.now, "Credential exposure").
		Run(func(mock.Arguments) { order = append(order, "document") }).Return(true, nil).Once()

	s.Require().Nil(s.svc.Suspend(s.ctx, "e1", "Credential exposure"))
	s.Equal([]string{"document"}, order)
	s.port.AssertNotCalled(s.T(), "ClearAccessState", mock.Anything, mock.Anything, mock.Anything)
}

// An unknown identity is a client error, not a failed read.
func (s *ServiceTestSuite) TestSuspendOfAnUnknownIdentityIsAClientError() {
	s.port.On("GetGovernedEntity", mock.Anything, "missing").
		Return(GovernedEntity{}, ErrEntityNotFound).Once()

	svcErr := s.svc.Suspend(s.ctx, "missing", "")
	s.Require().NotNil(svcErr)
	s.Equal(ErrorEntityNotFound.Code, svcErr.Code)
	s.port.AssertNotCalled(s.T(), "SetSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// The first placement time and note are kept.
func (s *ServiceTestSuite) TestRepeatingSuspendSucceedsWithoutWriting() {
	for name, mutate := range map[string]func(*GovernedEntity){
		"both stores":   func(*GovernedEntity) {},
		"document only": func(e *GovernedEntity) { e.State = providers.EntityStateActive },
		"column only":   func(e *GovernedEntity) { e.AccessState.Suspend = nil },
	} {
		s.SetupTest()
		entity := s.suspendedEntity()
		mutate(&entity)
		s.expectRead(entity)

		s.Require().Nilf(s.svc.Suspend(s.ctx, "e1", "A different reason"), "%s", name)
		s.port.AssertNotCalled(s.T(), "SetSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	}
}

// suspendedEntity is an account carrying the suspension in both of its stores.
func (s *ServiceTestSuite) suspendedEntity() GovernedEntity {
	entity := s.activeEntity()
	entity.State = providers.EntityStateSuspended
	entity.AccessState.Suspend = &SuspendState{SuspendedAt: s.at(-time.Hour)}
	return entity
}

func (s *ServiceTestSuite) TestEitherSuspensionStoreAloneHolds() {
	for name, mutate := range map[providers.AccessScope]func(*GovernedEntity){
		"document only": func(e *GovernedEntity) { e.State = providers.EntityStateActive },
		"column only":   func(e *GovernedEntity) { e.AccessState.Suspend = nil },
	} {
		s.SetupTest()

		entity := s.suspendedEntity()
		mutate(&entity)
		s.expectRead(entity)
		s.expectPolicy(authenticationMethodPolicy())

		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testScope)
		s.Require().Nil(err)
		s.Falsef(decision.Admitted, "%s must still hold the identity", name)
		s.Equal(providers.AccessRefusalReasonSuspended, decision.Reason)
	}
}

// A suspension refuses at every plane, including session, issuance and recovery.
func (s *ServiceTestSuite) TestASuspensionRefusesAtEveryPlane() {
	entity := s.suspendedEntity()
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	for _, plane := range []providers.AccessPlane{
		providers.AccessPlaneAuthentication, providers.AccessPlaneDispatch, providers.AccessPlaneRecovery,
		providers.AccessPlaneSession, providers.AccessPlaneIssuance, providers.AccessPlaneApplication,
	} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testScope)
		s.Require().Nil(err)
		s.False(decision.Admitted, "plane %s must refuse a suspended identity", plane)
		s.Equal(providers.AccessRefusalReasonSuspended, decision.Reason)
	}
}

func (s *ServiceTestSuite) TestASuspensionAppliesWithLockingSwitchedOff() {
	policy := authenticationMethodPolicy()
	policy.enabled = false

	entity := s.suspendedEntity()
	s.expectRead(entity)
	s.expectPolicy(policy)

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testScope)
	s.Require().Nil(err)
	s.False(decision.Admitted)
	s.Equal(providers.AccessRefusalReasonSuspended, decision.Reason)
}

// Suspend reads the entity, writes the suspension and does not clear lock state.
func (s *ServiceTestSuite) TestSuspendResolvesTheSourceBeforeWriting() {
	s.expectRead(s.activeEntity())
	s.port.On("SetSuspension", mock.Anything, "e1", s.now, "").Return(true, nil).Once()

	s.Require().Nil(s.svc.Suspend(s.ctx, "e1", ""))
	s.port.AssertNotCalled(s.T(), "ClearAccessState", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestUnsuspendOnAUserLandsOnTheEntityHold() {
	s.expectRead(s.suspendedEntity())

	var written *EntityHold
	var order []string
	s.port.On("ClearSuspension", mock.Anything, "e1", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			written, _ = args.Get(2).(*EntityHold)
			order = append(order, "document")
		}).
		Return(true, nil).Once()

	s.Require().Nil(s.svc.Unsuspend(s.ctx, "e1"))
	s.Require().NotNil(written, "a user must land held")
	s.Equal(PermanentUnlockAt, written.UnlockAt)
	s.Equal(LockReasonPostSuspension, written.Reason)
	s.Equal([]string{"document"}, order)
}

func (s *ServiceTestSuite) TestUnsuspendOnAnAgentOrApplicationLandsActive() {
	for _, category := range []providers.EntityCategory{
		providers.EntityCategoryAgent, providers.EntityCategoryApp,
	} {
		s.SetupTest()

		entity := s.suspendedEntity()
		entity.Category = category
		s.expectRead(entity)

		var written *EntityHold
		s.port.On("ClearSuspension", mock.Anything, "e1", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) { written, _ = args.Get(2).(*EntityHold) }).
			Return(true, nil).Once()

		s.Require().Nil(s.svc.Unsuspend(s.ctx, "e1"))
		s.Nil(written, "category %s must land on ACTIVE with no hold written", category)
	}
}

// The post-suspension hold holds every method, including ones that never lock automatically.
func (s *ServiceTestSuite) TestThePostSuspensionHoldHoldsEveryAuthenticationMethod() {
	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, AccessScopeEntity,
		ScopeLock{UnlockAt: PermanentUnlockAt, Reason: LockReasonPostSuspension})
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	for _, scope := range []providers.AccessScope{
		testScope,
		providers.AccessScopePasskey,
		providers.AccessScopeOTP,
		providers.AccessScopeMagicLink,
	} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, scope)
		s.Require().Nil(err)
		s.False(decision.Admitted, "scope %s must be held by the post-suspension hold", scope)
		s.Equal(providers.AccessRefusalReasonLocked, decision.Reason)
	}
}

func (s *ServiceTestSuite) TestThePostSuspensionHoldSurvivesLockingBeingSwitchedOff() {
	policy := authenticationMethodPolicy()
	policy.enabled = false

	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, AccessScopeEntity,
		ScopeLock{UnlockAt: PermanentUnlockAt, Reason: LockReasonPostSuspension})
	s.expectRead(entity)
	s.expectPolicy(policy)

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testScope)
	s.Require().Nil(err)
	s.False(decision.Admitted)
	s.Equal(providers.AccessRefusalReasonLocked, decision.Reason)
}

// Only the post-suspension hold outlives disabled locking; an entity-wide failure lock does not.
func (s *ServiceTestSuite) TestAnEntityWideFailureLockDoesNotSurviveLockingBeingSwitchedOff() {
	policy := authenticationMethodPolicy()
	policy.enabled = false

	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, AccessScopeEntity, ScopeLock{LockCount: 3, UnlockAt: PermanentUnlockAt})
	s.expectRead(entity)
	s.expectPolicy(policy)

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testScope)
	s.Require().Nil(err)
	s.True(decision.Admitted)
}

func (s *ServiceTestSuite) TestThePostSuspensionHoldNeverHoldsRecovery() {
	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, AccessScopeEntity,
		ScopeLock{UnlockAt: PermanentUnlockAt, Reason: LockReasonPostSuspension})
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneRecovery, testScope)
	s.Require().Nil(err)
	s.True(decision.Admitted)
}

func (s *ServiceTestSuite) TestNoFailureIsCountedUnderALiveEntityHold() {
	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, AccessScopeEntity,
		ScopeLock{UnlockAt: PermanentUnlockAt, Reason: LockReasonPostSuspension})
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	s.Require().Nil(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// Unlock works on a suspended identity and does not release the suspension.
func (s *ServiceTestSuite) TestUnlockIsNotRefusedOnASuspendedIdentity() {
	s.port.On("GetGovernedEntity", mock.Anything, "e1").Return(s.suspendedEntity(), nil).Maybe()
	s.port.On("ClearAccessState", mock.Anything, "e1", mock.Anything).Return(nil).Once()

	s.Require().Nil(s.svc.Unlock(s.ctx, "e1"))
	s.port.AssertNotCalled(s.T(), "ClearSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestUnsuspendRefusesWhenTheReadFails() {
	s.port.On("GetGovernedEntity", mock.Anything, "e1").
		Return(GovernedEntity{}, errors.New("database unavailable")).Maybe()

	s.NotNil(s.svc.Unsuspend(s.ctx, "e1"))
	s.port.AssertNotCalled(s.T(), "ClearSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A lifecycle operation with no subject fails before any write.
func (s *ServiceTestSuite) TestLifecycleOperationsRequireASubject() {
	s.requireServiceError(s.svc.Suspend(s.ctx, "", ""), ErrorMissingSubject)
	s.requireServiceError(s.svc.Unsuspend(s.ctx, ""), ErrorMissingSubject)
	s.requireServiceError(s.svc.Unlock(s.ctx, ""), ErrorMissingSubject)
	s.port.AssertNotCalled(s.T(), "SetSuspension",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	s.port.AssertNotCalled(s.T(), "ClearSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// The entity entry wins under either granularity; the read does not depend on the configuration.
func (s *ServiceTestSuite) TestTheEntityEntryWinsUnderBothGranularities() {
	for _, granularity := range []granularity{
		granularityAuthenticationMethod, granularityEntity,
	} {
		s.SetupTest()

		policy := authenticationMethodPolicy()
		policy.granularity = granularity

		entity := s.activeEntity()
		// A lapsed method entry under a live entity entry, as a granularity change can leave.
		putScopeLock(&entity.AccessState, testScope,
			ScopeLock{LockCount: 2, UnlockAt: s.at(-time.Hour)})
		putScopeLock(&entity.AccessState, AccessScopeEntity,
			ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
		s.expectRead(entity)
		s.expectPolicy(policy)

		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testScope)
		s.Require().Nil(err)
		s.Falsef(decision.Admitted, "granularity %s must be held by the entity entry", granularity)
		s.Equal(providers.AccessStatusLocked,
			reportFor(entity.State, entity.AccessState, s.now, lockEnforcement{}).Value)
	}
}

func (s *ServiceTestSuite) TestALapsedEntityEntryFallsThroughToTheAuthenticationMethod() {
	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, AccessScopeEntity,
		ScopeLock{LockCount: 1, UnlockAt: s.at(-time.Hour)})
	putScopeLock(&entity.AccessState, testScope,
		ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testScope)
	s.Require().Nil(err)
	s.False(decision.Admitted)
}

// putScopeLock records one lock entry under the member the scope maps to.
func putScopeLock(state *AccessState, scope providers.AccessScope, entry ScopeLock) {
	if scope == AccessScopeEntity {
		held := entry
		state.Lock.Entity = &held
		return
	}
	if state.Lock.AuthenticationMethods == nil {
		state.Lock.AuthenticationMethods = make(map[providers.AccessScope]ScopeLock)
	}
	state.Lock.AuthenticationMethods[scope] = entry
}

// lockedState builds a document holding one entry.
func lockedState(scope providers.AccessScope, entry ScopeLock) AccessState {
	var state AccessState
	putScopeLock(&state, scope, entry)
	return state
}

// serviceWith builds the service over the real resolver for the given configuration.
func (s *ServiceTestSuite) serviceWith(cfg accountAccessValue) *service {
	svc := newTestService(s.port, cfg, nil, nil)
	svc.now = func() time.Time { return s.now }
	return svc
}

// entityGranularityConfig is a user category in entity mode.
func entityGranularityConfig() accountAccessValue {
	return accountAccessValue{
		User: &accountAccessCategory{
			LockGranularity: new("entity"),
			Default: &accountAccessPolicy{
				Enabled:              new(true),
				Threshold:            new(5),
				FailureWindowSeconds: new(900),
				LockDurationsSeconds: []int{300, 900},
				LockDecaySeconds:     new(86400),
			},
		},
	}
}

func (s *ServiceTestSuite) lockedEntityWide() GovernedEntity {
	return GovernedEntity{
		ID:       "e1",
		Category: providers.EntityCategoryUser,
		Type:     "person",
		State:    providers.EntityStateActive,
		AccessState: lockedState(AccessScopeEntity,
			ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)}),
	}
}

func (s *ServiceTestSuite) TestEntityWideLockHoldsAuthenticationMethodsWithNoBlockOfTheirOwn() {
	svc := s.serviceWith(entityGranularityConfig())
	entity := s.lockedEntityWide()
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Maybe()

	for _, scope := range []providers.AccessScope{
		providers.AccessScopeCredential,
		providers.AccessScopeOTP,
		providers.AccessScopePasskey,
		providers.AccessScopeMagicLink,
		providers.AccessScopeFederated,
		providers.AccessScopeOpenID4VP,
	} {
		decision, err := svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, scope)
		s.Require().Nil(err)
		s.False(decision.Admitted, "scope %s must be held by the entity-wide lock", scope)
	}
}

// Under method granularity, a federated sign-in obeys an entity-wide lock and ignores a password lock.
func (s *ServiceTestSuite) TestAFederatedSignInObeysOnlyAnEntityWideLock() {
	svc := s.serviceWith(testAccountAccessConfig())

	entityWide := s.lockedEntityWide()
	s.port.On("GetGovernedEntity", mock.Anything, entityWide.ID).Return(entityWide, nil).Once()
	decision, err := svc.EvaluateAccess(s.ctx, entityWide.ID, providers.AccessPlaneAuthentication,
		providers.AccessScopeFederated)
	s.Require().Nil(err)
	s.False(decision.Admitted, "an entity-wide lock holds a federated sign-in")

	passwordOnly := s.lockedEntityWide()
	passwordOnly.ID = "e2"
	passwordOnly.AccessState = lockedState(providers.AccessScopeCredential,
		ScopeLock{LockCount: 1, UnlockAt: s.at(time.Minute)})
	s.port.On("GetGovernedEntity", mock.Anything, passwordOnly.ID).Return(passwordOnly, nil).Once()
	decision, err = svc.EvaluateAccess(s.ctx, passwordOnly.ID, providers.AccessPlaneAuthentication,
		providers.AccessScopeFederated)
	s.Require().Nil(err)
	s.True(decision.Admitted, "a password lock does not hold a federated sign-in")
}

// The session and issuance planes present no method: the policy still resolves and the lock does not apply.
func (s *ServiceTestSuite) TestALockIsNotReachedWithNoAuthenticationMethodPresent() {
	svc := s.serviceWith(entityGranularityConfig())
	entity := s.lockedEntityWide()
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Maybe()

	for _, plane := range []providers.AccessPlane{
		providers.AccessPlaneSession, providers.AccessPlaneIssuance, providers.AccessPlaneRecovery,
	} {
		decision, err := svc.EvaluateAccess(s.ctx, entity.ID, plane, "")
		s.Require().Nil(err)
		s.True(decision.Admitted, "plane %s must never be held by a lock", plane)
	}

	// The same lock, at the plane it does hold.
	decision, err := svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, "")
	s.Require().Nil(err)
	s.False(decision.Admitted)
}

// Under entity granularity a password failure uses the entity block's threshold, not the credential block's.
func (s *ServiceTestSuite) TestEntityGranularityUsesOneThresholdForEveryAuthenticationMethod() {
	svc := s.serviceWith(entityGranularityConfig())
	entity := GovernedEntity{
		ID: "e1", Category: providers.EntityCategoryUser, State: providers.EntityStateActive,
		AccessState: AccessState{},
	}

	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Maybe()

	// Both methods count under the reserved key, with the entity block's 15-minute window.
	for _, scope := range []providers.AccessScope{
		providers.AccessScopeCredential,
		providers.AccessScopeOTP,
	} {
		s.port.On("IncrementFailure", mock.Anything, entity.ID,
			AccessScopeEntity, s.now, s.now.Add(-15*time.Minute)).
			Return(ScopeLock{FailureCount: 4}, true, nil).Once()

		s.Require().Nil(svc.RecordFailure(s.ctx, entity.ID, scope))
	}

	// Four failures are below the entity threshold of 5; the OTP threshold of 2 would have formed a lock.
	s.port.AssertNotCalled(s.T(), "FormLock",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestEntityGranularityFormsTheLockFromTheEntityEscalation() {
	svc := s.serviceWith(entityGranularityConfig())
	entity := GovernedEntity{
		ID: "e1", Category: providers.EntityCategoryUser, State: providers.EntityStateActive,
		AccessState: AccessState{},
	}

	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Maybe()
	s.port.On("IncrementFailure", mock.Anything, entity.ID,
		AccessScopeEntity, mock.Anything, mock.Anything).
		Return(ScopeLock{FailureCount: 5}, true, nil).Once()

	var episode LockEpisode
	s.port.On("FormLock", mock.Anything, entity.ID, AccessScopeEntity,
		mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { episode, _ = args.Get(4).(LockEpisode) }).
		Return(true, nil).Once()

	s.Require().Nil(svc.RecordFailure(s.ctx, entity.ID, providers.AccessScopeCredential))
	s.Equal(s.now.Add(5*time.Minute).UTC().Format(time.RFC3339), episode.UnlockAt,
		"the entity block's first duration is 5 minutes; the credential block's is 1")
}

func (s *ServiceTestSuite) TestEntityGranularityStillDoesNotCountUncountableAuthenticationMethods() {
	svc := s.serviceWith(entityGranularityConfig())

	// No entity is set up: the class is checked before the read.
	s.Require().Nil(svc.RecordFailure(s.ctx, "e1", providers.AccessScopePasskey))
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// Each method counts under its own key, with the category default's window and threshold.
func (s *ServiceTestSuite) TestAuthenticationMethodGranularityCountsEachMethodUnderTheCategoryDefault() {
	svc := s.serviceWith(testAccountAccessConfig())
	entity := GovernedEntity{
		ID: "e1", Category: providers.EntityCategoryUser, State: providers.EntityStateActive,
		AccessState: AccessState{},
	}

	// The default's window is 15 minutes and its threshold 5, so a fifth failure forms the lock.
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Maybe()
	s.port.On("IncrementFailure", mock.Anything, entity.ID,
		providers.AccessScopeOTP, s.now, s.now.Add(-15*time.Minute)).
		Return(ScopeLock{FailureCount: 5}, true, nil).Once()
	s.port.On("FormLock", mock.Anything, entity.ID, providers.AccessScopeOTP,
		mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Once()

	s.Require().Nil(svc.RecordFailure(s.ctx, entity.ID, providers.AccessScopeOTP))
}

func (s *ServiceTestSuite) TestAuthenticationMethodGranularityKeepsScopesIsolated() {
	svc := s.serviceWith(testAccountAccessConfig())
	entity := GovernedEntity{
		ID: "e1", Category: providers.EntityCategoryUser, State: providers.EntityStateActive,
		AccessState: lockedState(providers.AccessScopeCredential,
			ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)}),
	}
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Maybe()

	held, err := svc.EvaluateAccess(s.ctx, entity.ID,
		providers.AccessPlaneAuthentication, providers.AccessScopeCredential)
	s.Require().Nil(err)
	s.False(held.Admitted)

	for _, scope := range []providers.AccessScope{
		providers.AccessScopeOTP,
		providers.AccessScopePasskey,
	} {
		decision, err := svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, scope)
		s.Require().Nil(err)
		s.True(decision.Admitted, "scope %s must survive a credential lock", scope)
	}

	// The session and issuance planes stay open in either mode.
	for _, plane := range []providers.AccessPlane{providers.AccessPlaneSession, providers.AccessPlaneIssuance} {
		decision, err := svc.EvaluateAccess(s.ctx, entity.ID,
			plane, providers.AccessScopeCredential)
		s.Require().Nil(err)
		s.True(decision.Admitted, "plane %s must be unaffected under authentication_method granularity", plane)
	}
}

// An explicitly disabled agent policy resolves through the real config: no error, no write.
func (s *ServiceTestSuite) TestAgentCategoryCountsNothing() {
	svc := s.serviceWith(testAccountAccessConfig())
	entity := GovernedEntity{
		ID: "a1", Category: providers.EntityCategoryAgent, State: providers.EntityStateActive,
	}
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Maybe()

	decision, err := svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
		providers.AccessScopeCredential)
	s.Require().Nil(err)
	s.True(decision.Admitted, "an agent with locking off is admitted, not errored")

	s.Require().Nil(svc.RecordFailure(s.ctx, entity.ID, providers.AccessScopeCredential))
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A lock formed under method granularity is not read after switching to entity granularity.
func (s *ServiceTestSuite) TestAAuthenticationMethodLockGoesInertWhenGranularityFlips() {
	entity := GovernedEntity{
		ID: "e1", Category: providers.EntityCategoryUser, State: providers.EntityStateActive,
		AccessState: lockedState(providers.AccessScopeCredential,
			ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)}),
	}
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Maybe()

	// Under the mode it was formed in, it holds.
	held, err := s.serviceWith(testAccountAccessConfig()).EvaluateAccess(s.ctx, entity.ID,
		providers.AccessPlaneAuthentication, providers.AccessScopeCredential)
	s.Require().Nil(err)
	s.False(held.Admitted)

	// Flipped to entity granularity, the same stored entry is not read at all.
	admitted, err := s.serviceWith(entityGranularityConfig()).EvaluateAccess(s.ctx, entity.ID,
		providers.AccessPlaneAuthentication, providers.AccessScopeCredential)
	s.Require().Nil(err)
	s.True(admitted.Admitted, "an entry under the inactive key must not hold")
}

// In either granularity a step clears its method entry and a completed sign-in clears a lapsed
// entity entry.
func (s *ServiceTestSuite) TestStepAndGrantClearTheirOwnKeysInEitherMode() {
	for name, cfg := range map[string]accountAccessValue{
		"authentication_method": testAccountAccessConfig(),
		"entity":                entityGranularityConfig(),
	} {
		s.Run(name, func() {
			port := NewEntityStateProviderMock(s.T())
			svc := newTestService(port, cfg, nil, nil)
			svc.now = func() time.Time { return s.now }

			// Both keys hold lapsed state.
			port.On("GetGovernedEntity", mock.Anything, "e1").Return(GovernedEntity{
				ID: "e1", Category: providers.EntityCategoryUser, State: providers.EntityStateActive,
				AccessState: AccessState{
					Lock: LockState{
						Entity: &ScopeLock{LockCount: 1},
						AuthenticationMethods: map[providers.AccessScope]ScopeLock{
							providers.AccessScopeCredential: {LockCount: 1},
						},
					},
				},
			}, nil).Twice()

			port.On("ClearAccessStateIfUnchanged", mock.Anything, "e1", providers.AccessScopeCredential, int64(0)).
				Return(true, nil).Once()
			step, err := svc.AdmitAuthenticationStep(s.ctx, "e1", providers.AccessScopeCredential)
			s.Require().Nil(err)
			s.True(step.Admitted)

			port.On("ClearAccessStateIfUnchanged", mock.Anything, "e1", AccessScopeEntity, int64(0)).
				Return(true, nil).Once()
			decision, err := svc.AdmitSignIn(s.ctx, "e1")
			s.Require().Nil(err)
			s.True(decision.Admitted, "a lapsed record must not hold a completed sign-in")
		})
	}
}

func (s *ServiceTestSuite) TestUnsuspendRefusesAnIdentityThatIsNotSuspended() {
	s.expectRead(s.activeEntity())
	s.requireServiceError(s.svc.Unsuspend(s.ctx, "e1"), ErrorNotApplicable)
	s.port.AssertNotCalled(s.T(), "ClearSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// expectActivity registers the activity policy the category resolves to.
func (s *ServiceTestSuite) expectActivity(policy activityPolicy) {
	s.policies.On("ActivityPolicies", mock.Anything).
		Return(activityPolicies{user: policy}).Once()
	s.port.On("GetEntityProfile", mock.Anything, mock.Anything).
		Return(&providers.Entity{Category: providers.EntityCategoryUser}, nil).Maybe()
}

// A recording category writes the stamp and passes the resolution window to the write.
func (s *ServiceTestSuite) TestRecordLoginStampsWhenTheCategoryRecords() {
	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectActivity(activityPolicy{record: true, resolution: time.Hour})
	s.port.On("RecordLogin", mock.Anything, entity.ID, s.now, s.now.Add(-time.Hour)).
		Return(nil).Once()

	s.Require().Nil(s.svc.RecordLogin(s.ctx, entity.ID))
}

func (s *ServiceTestSuite) TestRecordLoginSkipsEntityReadsWhenNoCategoryRecords() {
	s.policies.On("ActivityPolicies", mock.Anything).Return(activityPolicies{}).Once()

	s.Require().Nil(s.svc.RecordLogin(s.ctx, "e1"))

	s.port.AssertNotCalled(s.T(), "GetEntityProfile", mock.Anything, mock.Anything)
	s.port.AssertNotCalled(s.T(), "GetGovernedEntity", mock.Anything, mock.Anything)
	s.port.AssertNotCalled(s.T(), "RecordLogin",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// Once any category records, the entity's own category decides.
func (s *ServiceTestSuite) TestRecordLoginWritesNothingWhenThisCategoryDoesNot() {
	s.policies.On("ActivityPolicies", mock.Anything).
		Return(activityPolicies{user: activityPolicy{record: true}}).Once()
	s.port.On("GetEntityProfile", mock.Anything, "e1").
		Return(&providers.Entity{Category: providers.EntityCategoryAgent}, nil).Once()

	s.Require().Nil(s.svc.RecordLogin(s.ctx, "e1"))

	s.port.AssertNotCalled(s.T(), "RecordLogin",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// The window is passed to the write, so the store compares without a read-modify-write.
func (s *ServiceTestSuite) TestRecordLoginPassesTheResolutionWindowToTheWrite() {
	entity := s.activeEntity()
	s.expectActivity(activityPolicy{record: true, resolution: 15 * time.Minute})
	s.port.On("RecordLogin", mock.Anything, entity.ID, s.now, s.now.Add(-15*time.Minute)).
		Return(nil).Once()

	s.Require().Nil(s.svc.RecordLogin(s.ctx, entity.ID))
}

// A zero resolution collapses the window to a single instant.
func (s *ServiceTestSuite) TestRecordLoginWithNoResolutionCollapsesTheWindow() {
	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectActivity(activityPolicy{record: true, resolution: 0})
	s.port.On("RecordLogin", mock.Anything, entity.ID, s.now, s.now).Return(nil).Once()

	s.Require().Nil(s.svc.RecordLogin(s.ctx, entity.ID))
}

func (s *ServiceTestSuite) TestRecordLoginWithNoSubjectDoesNothing() {
	s.Require().Nil(s.svc.RecordLogin(s.ctx, ""))

	s.port.AssertNotCalled(s.T(), "GetEntityProfile", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestRecordLoginIsNotAuthorized() {
	authorizer := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	s.svc.authorizer = authorizer
	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectActivity(activityPolicy{record: true, resolution: time.Hour})
	s.port.On("RecordLogin", mock.Anything, entity.ID, s.now, s.now.Add(-time.Hour)).
		Return(nil).Once()

	s.Require().Nil(s.svc.RecordLogin(s.ctx, entity.ID))

	authorizer.AssertNotCalled(s.T(), "IsActionAllowed", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestRecordLoginNeverReadsTheRuntimeRow() {
	s.expectActivity(activityPolicy{record: true, resolution: time.Hour})
	s.port.On("RecordLogin", mock.Anything, "e1", s.now, s.now.Add(-time.Hour)).Return(nil).Once()

	s.Require().Nil(s.svc.RecordLogin(s.ctx, "e1"))

	s.port.AssertNotCalled(s.T(), "GetGovernedEntity", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestAdmitAuthenticationStepRefusesWithoutReset() {
	entity := s.activeEntity()
	entity.State = providers.EntityStateSuspended
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Once()
	s.expectPolicy(authenticationMethodPolicy())
	decision, err := s.svc.AdmitAuthenticationStep(s.ctx, entity.ID, testScope)
	s.Require().Nil(err)
	s.False(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "ClearAccessStateIfUnchanged",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestAdmitAuthenticationStepReadFailureDoesNotReset() {
	readErr := errors.New("database unavailable")
	s.port.On("GetGovernedEntity", mock.Anything, "e1").Return(GovernedEntity{}, readErr).Once()
	decision, err := s.svc.AdmitAuthenticationStep(s.ctx, "e1", testScope)
	s.requireServiceError(err, tidcommon.InternalServerError)
	s.False(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "ClearAccessStateIfUnchanged",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestAdmitAuthenticationStepResetsOnlyAfterAdmission() {
	for _, resetErr := range []error{nil, errors.New("write failed")} {
		s.Run(fmt.Sprintf("reset error %v", resetErr), func() {
			s.SetupTest()
			entity := s.activeEntity()
			putScopeLock(&entity.AccessState, testScope, ScopeLock{FailureCount: 2})
			read := s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Once()
			policy := s.policies.On("Resolve", mock.Anything, mock.Anything, testScope).
				Return(authenticationMethodPolicy()).Once()
			s.port.On("ClearAccessStateIfUnchanged", mock.Anything, entity.ID, testScope, int64(0)).
				NotBefore(read, policy).Return(false, resetErr).Once()
			decision, err := s.svc.AdmitAuthenticationStep(s.ctx, entity.ID, testScope)
			s.Require().Nil(err)
			s.True(decision.Admitted)
		})
	}
}

func (s *ServiceTestSuite) TestAdmitAuthenticationStepUngovernedSubjectDoesNoIO() {
	decision, err := s.svc.AdmitAuthenticationStep(s.ctx, "", testScope)
	s.Require().Nil(err)
	s.True(decision.Admitted)
}

// A completed sign-in clears a lapsed entity-wide record: the counter and the escalation.
func (s *ServiceTestSuite) TestAdmitSignInClearsALapsedEntityRecord() {
	entity := s.activeEntity()
	entity.AccessState.Lock.Entity = &ScopeLock{FailureCount: 2, LockCount: 1, UnlockAt: s.at(-time.Second)}
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())
	s.port.On("ClearAccessStateIfUnchanged", mock.Anything, entity.ID, AccessScopeEntity, int64(0)).
		Return(true, nil).Once()

	decision, err := s.svc.AdmitSignIn(s.ctx, entity.ID)
	s.Require().Nil(err)
	s.True(decision.Admitted)
}

// A live entity-wide lock, including the permanent one Unsuspend writes, is never cleared.
func (s *ServiceTestSuite) TestAdmitSignInNeverClearsALiveLock() {
	for name, unlockAt := range map[string]string{
		"timed":     s.at(5 * time.Minute),
		"permanent": PermanentUnlockAt,
	} {
		s.Run(name, func() {
			s.SetupTest()
			entity := s.activeEntity()
			entity.AccessState.Lock.Entity = &ScopeLock{LockCount: 1, UnlockAt: unlockAt}
			s.expectRead(entity)
			s.expectPolicy(authenticationMethodPolicy())

			decision, err := s.svc.AdmitSignIn(s.ctx, entity.ID)
			s.Require().Nil(err)
			s.True(decision.Admitted, "a lock does not reach the session plane")
			s.port.AssertNotCalled(s.T(), "ClearAccessStateIfUnchanged",
				mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// A refused grant clears nothing.
func (s *ServiceTestSuite) TestAdmitSignInRefusesWithoutClearing() {
	entity := s.activeEntity()
	entity.State = providers.EntityStateSuspended
	entity.AccessState.Lock.Entity = &ScopeLock{FailureCount: 2}
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	decision, err := s.svc.AdmitSignIn(s.ctx, entity.ID)
	s.Require().Nil(err)
	s.False(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "ClearAccessStateIfUnchanged",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A clean account and a failed clear both admit, and a clean one writes nothing.
func (s *ServiceTestSuite) TestAdmitSignInOnACleanAccountWritesNothing() {
	s.expectRead(s.activeEntity())
	s.expectPolicy(authenticationMethodPolicy())

	decision, err := s.svc.AdmitSignIn(s.ctx, "e1")
	s.Require().Nil(err)
	s.True(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "ClearAccessStateIfUnchanged",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestAdmitSignInClearFailureStillAdmits() {
	entity := s.activeEntity()
	entity.AccessState.Lock.Entity = &ScopeLock{FailureCount: 2}
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())
	s.port.On("ClearAccessStateIfUnchanged", mock.Anything, entity.ID, mock.Anything, mock.Anything).
		Return(false, errors.New("write failed")).Once()

	decision, err := s.svc.AdmitSignIn(s.ctx, entity.ID)
	s.Require().Nil(err)
	s.True(decision.Admitted)
}

// An unreadable authority refuses, and an ungoverned subject is admitted without IO.
func (s *ServiceTestSuite) TestAdmitSignInFailsClosedAndSkipsTheUngoverned() {
	readErr := errors.New("database unavailable")
	s.port.On("GetGovernedEntity", mock.Anything, "e1").Return(GovernedEntity{}, readErr).Once()
	decision, err := s.svc.AdmitSignIn(s.ctx, "e1")
	s.requireServiceError(err, tidcommon.InternalServerError)
	s.False(decision.Admitted)

	decision, err = s.svc.AdmitSignIn(s.ctx, "")
	s.Require().Nil(err)
	s.True(decision.Admitted)
}

func governedSubject(category providers.EntityCategory) GovernedEntity {
	return GovernedEntity{ID: "e1", Category: category, OUID: "ou-a", State: providers.EntityStateActive}
}

func callerContext(subject string) context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(subject, "ou-a", "", nil, nil))
}

// authorizer returns an authorizer mock that answers every check with allowed.
func (s *ServiceTestSuite) authorizer(allowed bool) *sysauthzmock.SystemAuthorizationServiceInterfaceMock {
	authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	authz.EXPECT().IsActionAllowed(mock.Anything, mock.Anything, mock.Anything).Return(allowed, nil).Maybe()
	return authz
}

func (s *ServiceTestSuite) TestLifecycleAuthorization() {
	for _, tc := range []struct {
		category providers.EntityCategory
		action   security.Action
		resource security.ResourceType
	}{
		{providers.EntityCategoryUser, security.ActionUpdateUser, security.ResourceTypeUser},
		{providers.EntityCategoryAgent, security.ActionUpdateAgent, security.ResourceTypeAgent},
	} {
		s.Run(string(tc.category), func() {
			port := NewEntityStateProviderMock(s.T())
			port.On("GetGovernedEntity", mock.Anything, "e1").Return(governedSubject(tc.category), nil)
			authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
			authz.EXPECT().IsActionAllowed(mock.Anything, tc.action, &sysauthz.ActionContext{
				ResourceType: tc.resource, OUID: "ou-a", ResourceID: "e1",
			}).Return(false, nil).Times(4)
			svc := newTestService(port, accountAccessValue{}, nil, authz)
			for _, operation := range []struct {
				name string
				run  func() *tidcommon.ServiceError
			}{
				{"suspend", func() *tidcommon.ServiceError { return svc.Suspend(callerContext("operator"), "e1", "") }},
				{"validate suspend", func() *tidcommon.ServiceError {
					return svc.ValidateSuspend(callerContext("operator"), "e1")
				}},
				{"unsuspend", func() *tidcommon.ServiceError { return svc.Unsuspend(callerContext("operator"), "e1") }},
				{"unlock", func() *tidcommon.ServiceError { return svc.Unlock(callerContext("operator"), "e1") }},
			} {
				s.Run(operation.name, func() {
					s.requireServiceError(operation.run(), tidcommon.ErrorUnauthorized)
				})
			}
			port.AssertNotCalled(s.T(), "SetSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			port.AssertNotCalled(s.T(), "ClearAccessState", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// A repeated suspend is authorized like a first one.
func (s *ServiceTestSuite) TestRepeatedSuspendIsAuthorized() {
	suspended := governedSubject(providers.EntityCategoryUser)
	suspended.State = providers.EntityStateSuspended
	port := NewEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1").Return(suspended, nil)
	authz := s.authorizer(false)

	s.requireServiceError(newTestService(port, accountAccessValue{}, nil, authz).
		Suspend(callerContext("operator"), "e1", ""), tidcommon.ErrorUnauthorized)
	authz.AssertNumberOfCalls(s.T(), "IsActionAllowed", 1)
	port.AssertNotCalled(s.T(), "SetSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// The suspend pre-check authorizes and writes nothing, even on a suspended identity.
func (s *ServiceTestSuite) TestValidateSuspendWritesNothing() {
	suspended := governedSubject(providers.EntityCategoryUser)
	suspended.State = providers.EntityStateSuspended
	port := NewEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1").Return(suspended, nil)
	authz := s.authorizer(true)
	svc := newTestService(port, accountAccessValue{}, nil, authz)

	s.Require().Nil(svc.ValidateSuspend(callerContext("operator"), "e1"))
	authz.AssertNumberOfCalls(s.T(), "IsActionAllowed", 1)
	s.requireServiceError(svc.ValidateSuspend(callerContext("operator"), ""), ErrorMissingSubject)
	port.AssertNotCalled(s.T(), "SetSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// An authorizer that cannot decide fails the operation as a server error, and nothing is written.
func (s *ServiceTestSuite) TestLifecycleAuthorizationFailureIsAServerError() {
	port := NewEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1").Return(governedSubject(providers.EntityCategoryAgent), nil)
	authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	authz.EXPECT().IsActionAllowed(mock.Anything, mock.Anything, mock.Anything).
		Return(false, &tidcommon.InternalServerError)

	s.requireServiceError(newTestService(port, accountAccessValue{}, nil, authz).
		Unlock(callerContext("operator"), "e1"), tidcommon.InternalServerError)
	port.AssertNotCalled(s.T(), "ClearAccessState", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestLifecycleSelfTargetRefusedBeforeTheAuthorizer() {
	for _, category := range []providers.EntityCategory{providers.EntityCategoryUser, providers.EntityCategoryAgent} {
		authz := s.authorizer(true)
		svc := newTestService(nil, accountAccessValue{}, nil, authz)
		s.requireServiceError(svc.authorize(callerContext("e1"), governedSubject(category)),
			tidcommon.ErrorUnauthorized)
		authz.AssertNotCalled(s.T(), "IsActionAllowed", mock.Anything, mock.Anything, mock.Anything)
		s.Require().Nil(svc.authorize(security.WithRuntimeContext(callerContext("e1")), governedSubject(category)))
		authz.AssertNumberOfCalls(s.T(), "IsActionAllowed", 1)
	}
}

func (s *ServiceTestSuite) TestApplicationHasNoSystemAuthorizationVocabulary() {
	authz := s.authorizer(false)
	s.Require().Nil(newTestService(nil, accountAccessValue{}, nil, authz).
		authorize(callerContext("operator"), governedSubject(providers.EntityCategoryApp)))
	authz.AssertNotCalled(s.T(), "IsActionAllowed", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestEmbeddableGovernanceWithoutAuthorization() {
	s.Require().Nil(newTestService(nil, accountAccessValue{}, nil, nil).
		authorize(context.Background(), governedSubject(providers.EntityCategoryUser)))
}

// lockFormationConfig is a user policy that locks at the third failure.
func lockFormationConfig() accountAccessValue {
	return accountAccessValue{
		User: &accountAccessCategory{
			LockGranularity: new("authentication_method"),
			Default: &accountAccessPolicy{
				Enabled:              new(true),
				Threshold:            new(3),
				FailureWindowSeconds: new(900),
				LockDurationsSeconds: []int{300},
				LockDecaySeconds:     new(86400),
			},
		},
	}
}

// notifyingService builds the service over lockFormationConfig with a notifier mock. Only the writer
// that commits a new lock notifies.
func (s *ServiceTestSuite) notifyingService() (*service, *lockNotifierInterfaceMock) {
	notifier := newLockNotifierInterfaceMock(s.T())
	svc := newTestService(s.port, lockFormationConfig(), notifier, nil)
	svc.now = func() time.Time { return s.now }
	return svc, notifier
}

func (s *ServiceTestSuite) governedUser() GovernedEntity {
	return GovernedEntity{
		ID:       "user-1",
		Category: providers.EntityCategoryUser,
		Type:     "member",
		State:    providers.EntityStateActive,
	}
}

// expectFailureReaching sets up a failure whose increment lands on the threshold.
func (s *ServiceTestSuite) expectFailureReaching(count int) {
	entity := s.governedUser()
	s.port.EXPECT().GetGovernedEntity(mock.Anything, entity.ID).Return(entity, nil)
	s.port.EXPECT().IncrementFailure(mock.Anything, entity.ID,
		providers.AccessScopeCredential, mock.Anything, mock.Anything).
		Return(ScopeLock{FailureCount: count}, true, nil)
}

// A committed lock notifies with the scope that was held.
func (s *ServiceTestSuite) TestCommittedLockNotifies() {
	svc, notifier := s.notifyingService()
	s.expectFailureReaching(3)
	s.port.EXPECT().FormLock(mock.Anything, "user-1", providers.AccessScopeCredential,
		mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	notifier.EXPECT().NotifyLockFormed(mock.Anything, mock.MatchedBy(func(e GovernedEntity) bool {
		return e.ID == "user-1"
	}), providers.AccessScopeCredential, mock.MatchedBy(func(episode LockEpisode) bool {
		return episode.Duration == 5*time.Minute && episode.UnlockAt == s.now.Add(5*time.Minute).Format(time.RFC3339)
	})).Return()

	s.Require().Nil(svc.RecordFailure(s.ctx, "user-1", providers.AccessScopeCredential))
}

func (s *ServiceTestSuite) TestOrdinaryFailureDoesNotNotify() {
	svc, notifier := s.notifyingService()
	s.expectFailureReaching(1)

	s.Require().Nil(svc.RecordFailure(s.ctx, "user-1", providers.AccessScopeCredential))
	notifier.AssertNotCalled(s.T(), "NotifyLockFormed", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// Of two concurrent failures, only the guard's winner notifies.
func (s *ServiceTestSuite) TestLosingWriterDoesNotNotify() {
	svc, notifier := s.notifyingService()
	s.expectFailureReaching(3)
	s.port.EXPECT().FormLock(mock.Anything, "user-1", providers.AccessScopeCredential,
		mock.Anything, mock.Anything, mock.Anything).Return(false, nil)

	// The re-read shows the lock another writer formed, so this attempt stops.
	locked := s.governedUser()
	locked.AccessState = AccessState{Lock: LockState{
		AuthenticationMethods: map[providers.AccessScope]ScopeLock{
			providers.AccessScopeCredential: {
				FailureCount: 3,
				LockCount:    1,
				UnlockAt:     s.now.Add(5 * time.Minute).UTC().Format(time.RFC3339),
			},
		}}}
	s.port.EXPECT().GetGovernedEntity(mock.Anything, "user-1").Return(locked, nil)

	s.Require().Nil(svc.RecordFailure(s.ctx, "user-1", providers.AccessScopeCredential))
	notifier.AssertNotCalled(s.T(), "NotifyLockFormed", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A failure under a live hold is not counted, so it does not notify again.
func (s *ServiceTestSuite) TestFailureUnderALiveHoldDoesNotNotify() {
	svc, notifier := s.notifyingService()
	entity := s.governedUser()
	entity.AccessState = AccessState{Lock: LockState{Entity: &ScopeLock{
		UnlockAt: s.now.Add(5 * time.Minute).UTC().Format(time.RFC3339),
	}}}
	s.port.EXPECT().GetGovernedEntity(mock.Anything, entity.ID).Return(entity, nil)

	s.Require().Nil(svc.RecordFailure(s.ctx, "user-1", providers.AccessScopeCredential))
	notifier.AssertNotCalled(s.T(), "NotifyLockFormed", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestNoNotifierStillLocks() {
	svc := newTestService(s.port, lockFormationConfig(), nil, nil)
	svc.now = func() time.Time { return s.now }

	s.expectFailureReaching(3)
	formed := s.port.EXPECT().FormLock(mock.Anything, "user-1", providers.AccessScopeCredential,
		mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	s.Require().Nil(svc.RecordFailure(s.ctx, "user-1", providers.AccessScopeCredential))
	s.NotNil(formed)
}

// A check on an account with no entries resolves no policy. The mock is strict, so a resolve fails
// the test.
func (s *ServiceTestSuite) TestACleanAccountIsAnsweredWithoutAPolicy() {
	entity := s.activeEntity()
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Once()

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testScope)

	s.Require().Nil(err)
	s.True(decision.Admitted)
	s.policies.AssertNotCalled(s.T(), "Resolve", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestAnApplicationIsAnsweredWithoutAPolicy() {
	entity := s.activeEntity()
	entity.Category = providers.EntityCategoryApp
	entity.AccessState = AccessState{
		Lock: LockState{
			AuthenticationMethods: map[providers.AccessScope]ScopeLock{
				providers.AccessScopeSystemCredential: {UnlockAt: s.at(time.Hour)},
			},
		},
	}
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Once()

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneApplication, "")

	s.Require().Nil(err)
	s.True(decision.Admitted)
	s.policies.AssertNotCalled(s.T(), "Resolve", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestASuspendedApplicationStillRefuses() {
	entity := s.activeEntity()
	entity.Category = providers.EntityCategoryApp
	entity.State = providers.EntityStateSuspended
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Once()
	s.policies.On("Resolve", mock.Anything, mock.Anything, providers.AccessScope("")).
		Return(lockoutPolicy{discloseAccountHold: discloseAlways}).Once()

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneApplication, "")

	s.Require().Nil(err)
	s.False(decision.Admitted)
	s.Equal(providers.AccessRefusalReasonSuspended, decision.Reason)
}

// authorizerAllowing returns an authorizer mock that answers every check with allowed.
func authorizerAllowing(t *testing.T, allowed bool) *sysauthzmock.SystemAuthorizationServiceInterfaceMock {
	authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	authz.EXPECT().IsActionAllowed(mock.Anything, mock.Anything, mock.Anything).Return(allowed, nil).Maybe()
	return authz
}

// auditedService returns a service whose administration events land in the returned slice.
func auditedService(t *testing.T, port EntityStateProvider,
	authz sysauthz.SystemAuthorizationServiceInterface) (*service, *[]*providers.Event) {
	published := []*providers.Event{}
	obs := observabilityprovidermock.NewObservabilityProviderMock(t)
	obs.EXPECT().IsEnabled().Return(true).Maybe()
	obs.EXPECT().PublishEvent(mock.Anything, mock.Anything).
		Run(func(_ context.Context, evt *providers.Event) { published = append(published, evt) }).Maybe()
	svc := newTestService(port, accountAccessValue{}, nil, authz)
	svc.observability = obs
	return svc, &published
}

// A placed suspension is published with its actor and target. Only whether a note was given is
// recorded, never the text.
func (s *ServiceTestSuite) TestASuspensionIsPublishedForAudit() {
	port := NewEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1").Return(governedSubject(providers.EntityCategoryUser), nil)
	port.On("SetSuspension", mock.Anything, "e1", mock.Anything, "Credential exposure").Return(true, nil)
	svc, published := auditedService(s.T(), port, authorizerAllowing(s.T(), true))

	s.Require().Nil(svc.Suspend(callerContext("operator"), "e1", "Credential exposure"))

	s.Require().Len(*published, 1)
	evt := (*published)[0]
	s.Require().Equal(string(event.EventTypeIdentitySuspended), evt.Type)
	s.Require().Equal(providers.StatusSuccess, evt.Status)
	s.Require().Equal("e1", evt.Data[event.DataKey.Subject])
	s.Require().Equal(event.PrincipalTypeUser, evt.Data[event.DataKey.SubjectType])
	s.Require().Equal("operator", evt.Data[event.DataKey.ActorSub])
	s.Require().Equal(true, evt.Data[event.DataKey.OperatorNoteRecorded])
	for key, value := range evt.Data {
		s.Require().NotContains(fmt.Sprint(value), "Credential exposure", "field %s carries the note", key)
	}
}

// A refused or inapplicable operation is recorded as a failure, with why, and writes nothing.
func (s *ServiceTestSuite) TestARefusedLifecycleOperationIsPublishedAsAFailure() {
	agent := governedSubject(providers.EntityCategoryAgent)
	for _, tc := range []struct {
		name    string
		entity  GovernedEntity
		allowed bool
		run     func(*service) *tidcommon.ServiceError
		want    providers.EventType
		failure string
	}{
		{"suspend outside the boundary", agent, false,
			func(s *service) *tidcommon.ServiceError { return s.Suspend(callerContext("operator"), "e1", "") },
			event.EventTypeIdentitySuspensionFailed, "not_authorized"},
		{"suspend pre-check outside the boundary", agent, false,
			func(s *service) *tidcommon.ServiceError { return s.ValidateSuspend(callerContext("operator"), "e1") },
			event.EventTypeIdentitySuspensionFailed, "not_authorized"},
		{"unsuspend on an active identity", agent, true,
			func(s *service) *tidcommon.ServiceError { return s.Unsuspend(callerContext("operator"), "e1") },
			event.EventTypeIdentityUnsuspensionFailed, "not_applicable"},
		{"unlock outside the boundary", agent, false,
			func(s *service) *tidcommon.ServiceError { return s.Unlock(callerContext("operator"), "e1") },
			event.EventTypeIdentityUnlockFailed, "not_authorized"},
	} {
		s.Run(tc.name, func() {
			port := NewEntityStateProviderMock(s.T())
			port.On("GetGovernedEntity", mock.Anything, "e1").Return(tc.entity, nil)
			svc, published := auditedService(s.T(), port, authorizerAllowing(s.T(), tc.allowed))

			s.Require().NotNil(tc.run(svc))

			s.Require().Len(*published, 1)
			evt := (*published)[0]
			s.Require().Equal(string(tc.want), evt.Type)
			s.Require().Equal(providers.StatusFailure, evt.Status)
			s.Require().Equal(tc.failure, evt.Data[event.DataKey.Error])
			s.Require().Equal(event.PrincipalTypeAgent, evt.Data[event.DataKey.SubjectType])
			s.Require().Equal("operator", evt.Data[event.DataKey.ActorSub])
		})
	}
}

// A recovery flow's unlock is its own event, in the authentication category, with no operator.
func (s *ServiceTestSuite) TestASelfServiceUnlockIsPublishedWithoutAnOperator() {
	port := NewEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1").Return(governedSubject(providers.EntityCategoryUser), nil)
	port.On("ClearAccessState", mock.Anything, "e1", AllAccessScopes).Return(nil)
	svc, published := auditedService(s.T(), port, authorizerAllowing(s.T(), true))

	s.Require().Nil(svc.Unlock(security.WithRuntimeContext(context.Background()), "e1"))

	s.Require().Len(*published, 1)
	evt := (*published)[0]
	s.Require().Equal(string(event.EventTypeIdentitySelfUnlocked), evt.Type)
	s.Require().Equal(providers.StatusSuccess, evt.Status)
	s.Require().NotContains(evt.Data, event.DataKey.ActorSub)
	category, err := event.GetCategory(providers.EventType(evt.Type))
	s.Require().NoError(err)
	s.Require().Equal(event.CategoryAuthentication, category)
}

func (s *ServiceTestSuite) TestAFailedSelfServiceUnlockIsPublishedAsASelfServiceFailure() {
	port := NewEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1").Return(governedSubject(providers.EntityCategoryUser), nil)
	port.On("ClearAccessState", mock.Anything, "e1", AllAccessScopes).Return(errors.New("db down"))
	svc, published := auditedService(s.T(), port, authorizerAllowing(s.T(), true))

	s.Require().NotNil(svc.Unlock(security.WithRuntimeContext(context.Background()), "e1"))

	s.Require().Len(*published, 1)
	evt := (*published)[0]
	s.Require().Equal(string(event.EventTypeIdentitySelfUnlockFailed), evt.Type)
	s.Require().Equal(providers.StatusFailure, evt.Status)
	s.Require().NotContains(evt.Data, event.DataKey.ActorSub)
}

// An operator's unlock stays an administration event that names the operator.
func (s *ServiceTestSuite) TestAnOperatorUnlockIsPublishedAsAdministration() {
	port := NewEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1").Return(governedSubject(providers.EntityCategoryUser), nil)
	port.On("ClearAccessState", mock.Anything, "e1", AllAccessScopes).Return(nil)
	svc, published := auditedService(s.T(), port, authorizerAllowing(s.T(), true))

	s.Require().Nil(svc.Unlock(callerContext("operator"), "e1"))

	s.Require().Len(*published, 1)
	evt := (*published)[0]
	s.Require().Equal(string(event.EventTypeIdentityUnlocked), evt.Type)
	s.Require().Equal("operator", evt.Data[event.DataKey.ActorSub])
	category, err := event.GetCategory(providers.EventType(evt.Type))
	s.Require().NoError(err)
	s.Require().Equal(event.CategoryAdministration, category)
}

// A passing pre-check publishes nothing; Suspend records the outcome.
func (s *ServiceTestSuite) TestAPassingSuspendPreCheckIsNotPublished() {
	port := NewEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1").Return(governedSubject(providers.EntityCategoryUser), nil)
	svc, published := auditedService(s.T(), port, authorizerAllowing(s.T(), true))

	s.Require().Nil(svc.ValidateSuspend(callerContext("operator"), "e1"))
	s.Require().Empty(*published)
}

func (s *ServiceTestSuite) TestARepeatedSuspensionIsPublished() {
	suspended := governedSubject(providers.EntityCategoryUser)
	suspended.State = providers.EntityStateSuspended
	port := NewEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1").Return(suspended, nil)
	svc, published := auditedService(s.T(), port, authorizerAllowing(s.T(), true))

	s.Require().Nil(svc.Suspend(callerContext("operator"), "e1", ""))

	s.Require().Len(*published, 1)
	s.Require().Equal(string(event.EventTypeIdentitySuspended), (*published)[0].Type)
	s.Require().Equal("Suspension already in place",
		(*published)[0].Data[event.DataKey.Message])
	port.AssertNotCalled(s.T(), "SetSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A suspension reads the policy at the presented scope for its disclosure mode, and a disclosing
// policy names the suspension.
func (s *ServiceTestSuite) TestADisclosingPolicyNamesASuspension() {
	s.policies.On("Resolve", mock.Anything, mock.Anything, testScope).
		Return(authenticationMethodPolicy()).Once()
	s.expectRead(s.suspendedEntity())

	decision, err := s.svc.EvaluateAccess(s.ctx, "e1", providers.AccessPlaneAuthentication, testScope)

	s.Require().Nil(err)
	s.Equal(providers.AccessDecision{Reason: providers.AccessRefusalReasonSuspended}, decision)
}

// A policy that does not disclose refuses a suspension and a lock without naming either.
func (s *ServiceTestSuite) TestANonDisclosingPolicyNamesNoHold() {
	policy := authenticationMethodPolicy()
	policy.discloseAccountHold = discloseNever
	s.expectPolicy(policy)
	locked := s.activeEntity()
	locked.AccessState = lockedState(testScope, ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})

	for _, entity := range []GovernedEntity{s.suspendedEntity(), locked} {
		s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Once()

		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testScope)

		s.Require().Nil(err)
		s.Equal(providers.AccessDecision{}, decision)
	}
}

// A lock refusal says the account is locked, and nothing more.
func (s *ServiceTestSuite) TestALockRefusalNamesTheLock() {
	s.expectPolicy(authenticationMethodPolicy())
	entity := s.activeEntity()
	entity.AccessState = lockedState(testScope, ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
	s.expectRead(entity)

	decision, err := s.svc.EvaluateAccess(s.ctx, "e1", providers.AccessPlaneAuthentication, testScope)

	s.Require().Nil(err)
	s.Equal(providers.AccessDecision{Reason: providers.AccessRefusalReasonLocked}, decision)
}

// A failed increment is a server error and forms no lock.
func (s *ServiceTestSuite) TestRecordFailureReportsAFailedIncrement() {
	s.expectPolicy(authenticationMethodPolicy())
	s.expectRead(s.activeEntity())
	s.port.On("IncrementFailure", mock.Anything, "e1", testScope, mock.Anything, mock.Anything).
		Return(ScopeLock{}, false, errors.New("database unavailable")).Once()

	s.requireServiceError(s.svc.RecordFailure(s.ctx, "e1", testScope), tidcommon.InternalServerError)
	s.port.AssertNotCalled(s.T(), "FormLock", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything)
}

// A failed lock write is a server error.
func (s *ServiceTestSuite) TestRecordFailureReportsAFailedLockWrite() {
	s.expectPolicy(authenticationMethodPolicy())
	s.expectRead(s.activeEntity())
	s.port.On("IncrementFailure", mock.Anything, "e1", testScope, mock.Anything, mock.Anything).
		Return(ScopeLock{FailureCount: 5}, true, nil).Once()
	s.port.On("FormLock", mock.Anything, "e1", testScope, mock.Anything, mock.Anything, mock.Anything).
		Return(false, errors.New("database unavailable")).Once()

	s.requireServiceError(s.svc.RecordFailure(s.ctx, "e1", testScope), tidcommon.InternalServerError)
}

// A failed re-read after a lost lock guard is a server error.
func (s *ServiceTestSuite) TestRecordFailureReportsAFailedReRead() {
	s.expectPolicy(authenticationMethodPolicy())
	s.port.On("GetGovernedEntity", mock.Anything, "e1").Return(s.activeEntity(), nil).Once()
	s.port.On("IncrementFailure", mock.Anything, "e1", testScope, mock.Anything, mock.Anything).
		Return(ScopeLock{FailureCount: 5}, true, nil).Once()
	s.port.On("FormLock", mock.Anything, "e1", testScope, mock.Anything, mock.Anything, mock.Anything).
		Return(false, nil).Once()
	s.port.On("GetGovernedEntity", mock.Anything, "e1").
		Return(GovernedEntity{}, errors.New("database unavailable")).Once()

	s.requireServiceError(s.svc.RecordFailure(s.ctx, "e1", testScope), tidcommon.InternalServerError)
}

// A failed release write is a server error.
func (s *ServiceTestSuite) TestUnsuspendReportsAFailedWrite() {
	s.expectRead(s.suspendedEntity())
	s.port.On("ClearSuspension", mock.Anything, "e1", mock.Anything, mock.Anything).
		Return(false, errors.New("database unavailable")).Once()

	s.requireServiceError(s.svc.Unsuspend(s.ctx, "e1"), tidcommon.InternalServerError)
}

// A suspension released or replaced after the read is not released again.
func (s *ServiceTestSuite) TestUnsuspendOfAChangedSuspensionDoesNotApply() {
	s.expectRead(s.suspendedEntity())
	s.port.On("ClearSuspension", mock.Anything, "e1", mock.Anything, s.at(-time.Hour)).
		Return(false, nil).Once()

	s.requireServiceError(s.svc.Unsuspend(s.ctx, "e1"), ErrorNotApplicable)
}

// A concurrent suspend that placed the hold first wins; this one succeeds without replacing it.
func (s *ServiceTestSuite) TestAConcurrentSuspendKeepsTheFirstPlacement() {
	s.expectRead(s.activeEntity())
	s.port.On("SetSuspension", mock.Anything, "e1", s.now, "Second note").Return(false, nil).Once()

	s.Require().Nil(s.svc.Suspend(s.ctx, "e1", "Second note"))
}

// A failed unlock write is a server error.
func (s *ServiceTestSuite) TestUnlockReportsAFailedWrite() {
	s.expectRead(s.activeEntity())
	s.port.On("ClearAccessState", mock.Anything, "e1", AllAccessScopes).
		Return(errors.New("database unavailable")).Once()

	s.requireServiceError(s.svc.Unlock(s.ctx, "e1"), tidcommon.InternalServerError)
}

// A failed profile read or stamp write is a server error.
func (s *ServiceTestSuite) TestRecordLoginReportsStoreFailures() {
	s.policies.On("ActivityPolicies", mock.Anything).
		Return(activityPolicies{user: activityPolicy{record: true}}).Twice()
	s.port.On("GetEntityProfile", mock.Anything, "e1").Return(nil, errors.New("database unavailable")).Once()
	s.port.On("GetGovernedEntity", mock.Anything, "e1").
		Return(GovernedEntity{}, errors.New("database unavailable")).Once()

	s.requireServiceError(s.svc.RecordLogin(s.ctx, "e1"), tidcommon.InternalServerError)

	s.port.On("GetEntityProfile", mock.Anything, "e1").
		Return(&providers.Entity{Category: providers.EntityCategoryUser}, nil).Once()
	s.port.On("RecordLogin", mock.Anything, "e1", s.now, s.now).Return(errors.New("database unavailable")).Once()

	s.requireServiceError(s.svc.RecordLogin(s.ctx, "e1"), tidcommon.InternalServerError)
}

// An identity with no local record has nothing to govern, so every check admits it. Only that
// sentinel admits; any other read failure refuses, as the test below shows.
func (s *ServiceTestSuite) TestAnIdentityWithNoLocalRecordIsAdmitted() {
	s.port.On("GetGovernedEntity", mock.Anything, "ext-1").Return(GovernedEntity{}, ErrEntityNotFound)

	decision, err := s.svc.EvaluateAccess(s.ctx, "ext-1", providers.AccessPlaneSession, "")
	s.Require().Nil(err)
	s.True(decision.Admitted)

	decision, err = s.svc.AdmitSignIn(s.ctx, "ext-1")
	s.Require().Nil(err)
	s.True(decision.Admitted)

	decision, err = s.svc.AdmitAuthenticationStep(s.ctx, "ext-1", testScope)
	s.Require().Nil(err)
	s.True(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "ClearAccessStateIfUnchanged",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// An identity with no local record records nothing and reports no error.
func (s *ServiceTestSuite) TestAnIdentityWithNoLocalRecordRecordsNothing() {
	s.port.On("GetGovernedEntity", mock.Anything, "ext-1").Return(GovernedEntity{}, ErrEntityNotFound)
	s.Nil(s.svc.RecordFailure(s.ctx, "ext-1", testScope))
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	s.policies.On("ActivityPolicies", mock.Anything).
		Return(activityPolicies{user: activityPolicy{record: true}}).Once()
	s.port.On("GetEntityProfile", mock.Anything, "ext-1").Return(nil, errors.New("entity not found")).Once()
	s.Nil(s.svc.RecordLogin(s.ctx, "ext-1"))
	s.port.AssertNotCalled(s.T(), "RecordLogin", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A failed read refuses a completed sign-in with a server error.
func (s *ServiceTestSuite) TestAdmitSignInReportsAFailedRead() {
	s.port.On("GetGovernedEntity", mock.Anything, "e1").
		Return(GovernedEntity{}, errors.New("database unavailable")).Once()

	decision, err := s.svc.AdmitSignIn(s.ctx, "e1")

	s.requireServiceError(err, tidcommon.InternalServerError)
	s.False(decision.Admitted)
}

// An operation naming no identity is published with the no_subject class.
func (s *ServiceTestSuite) TestAMissingSubjectIsPublishedAsNoSubject() {
	svc, published := auditedService(s.T(), NewEntityStateProviderMock(s.T()), nil)

	s.requireServiceError(svc.Unlock(callerContext("operator"), ""), ErrorMissingSubject)

	s.Require().Len(*published, 1)
	s.Equal(providers.StatusFailure, (*published)[0].Status)
	s.Equal("no_subject", (*published)[0].Data[event.DataKey.Error])
}

// The access reader resolves the category's enforcement from the current section.
func (s *ServiceTestSuite) TestNewAccessReaderUsesTheCurrentSection() {
	cfg := testAccountAccessConfig()
	cfg.User.Default.Enabled = new(false)
	svc := newTestService(s.port, cfg, nil, nil)
	entity := providers.Entity{
		ID: "e1", Category: providers.EntityCategoryUser, State: providers.EntityStateActive,
		RuntimeAttributes: []byte(`{"accessState":{"lock":{"authenticationMethods":{"otp":` +
			`{"unlockAt":"9999-12-31T23:59:59Z"}}}}}`),
	}

	read := svc.NewAccessReader(s.ctx, providers.EntityCategoryUser).Read(entity)

	s.Equal(providers.AccessStatusActive, read.Status.Value, "a lock whose policy is switched off is not reported")
}
