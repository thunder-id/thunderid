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

	governanceconfig "github.com/thunder-id/thunderid/internal/identitygovernance/config"
	"github.com/thunder-id/thunderid/internal/identitygovernance/model"
	sysconfig "github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/observabilityprovidermock"
)

type ServiceTestSuite struct {
	suite.Suite
	port     *EntityStateProviderMock
	policies *PolicyResolverMock
	svc      *service
	ctx      context.Context
	now      time.Time
}

// newTestPolicyResolver reads the fixture through the server-config source.
func newTestPolicyResolver(cfg governanceconfig.AccountAccessValue) PolicyResolver {
	source := governanceconfig.NewAccountAccessSource(sysconfig.AccountAccessConfig{}, log.GetLogger())
	governanceconfig.InstallConfigReader(testConfigReader{cfg})
	return governanceconfig.NewPolicyResolver(source)
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (s *ServiceTestSuite) SetupTest() {
	s.port = NewEntityStateProviderMock(s.T())
	s.policies = NewPolicyResolverMock(s.T())
	s.now = time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	s.svc = newService(s.port, s.policies, nil, nil)
	s.svc.now = func() time.Time { return s.now }
	s.ctx = context.Background()
}

const testScope = model.AccessScopeCredential

func (s *ServiceTestSuite) at(offset time.Duration) string {
	return s.now.Add(offset).UTC().Format(time.RFC3339)
}

// activeEntity is an ordinary user with no lock state.
func (s *ServiceTestSuite) activeEntity() model.GovernedEntity {
	return model.GovernedEntity{
		ID:          "e1",
		Category:    providers.EntityCategoryUser,
		Type:        "person",
		State:       providers.EntityStateActive,
		AccessState: model.AccessState{},
	}
}

// authenticationMethodPolicy is the default shape: per-authentication-method granularity, both plane flags off.
func authenticationMethodPolicy() governanceconfig.LockoutPolicy {
	return governanceconfig.LockoutPolicy{
		Enabled:       true,
		Threshold:     5,
		FailureWindow: 15 * time.Minute,
		LockDurations: []time.Duration{5 * time.Minute, 15 * time.Minute},
		LockDecay:     24 * time.Hour,
		Granularity:   governanceconfig.GranularityAuthenticationMethod,
	}
}

func (s *ServiceTestSuite) expectPolicy(policy governanceconfig.LockoutPolicy) {
	s.policies.On("Resolve", mock.Anything, mock.Anything, mock.Anything).
		Return(policy).Maybe()
}

func (s *ServiceTestSuite) expectRead(entity model.GovernedEntity) {
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Maybe()
}

// An empty subject, such as a client-credentials grant with no user, is admitted.
func (s *ServiceTestSuite) TestUngovernedSubjectIsAdmitted() {
	decision, err := s.svc.EvaluateAccess(s.ctx, "", model.PlaneIssuance, "")
	s.Require().NoError(err)
	s.True(decision.Admitted)
}

func (s *ServiceTestSuite) TestUnreadableAuthorityRefuses() {
	readErr := errors.New("database unavailable")
	s.port.On("GetGovernedEntity", mock.Anything, "e1").Return(model.GovernedEntity{}, readErr).Once()

	decision, err := s.svc.EvaluateAccess(s.ctx, "e1", model.PlaneAuthentication, testScope)
	s.Require().ErrorIs(err, readErr)
	s.False(decision.Admitted, "a read failure must not admit")
	s.Equal(model.HoldIdentity, decision.HoldLevel)
}

func (s *ServiceTestSuite) TestIdentityHoldAppliesAtEveryPlane() {
	entity := s.activeEntity()
	entity.State = providers.EntityStateSuspended
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	for _, plane := range []model.Plane{
		model.PlaneAuthentication, model.PlaneDispatch, model.PlaneRecovery,
		model.PlaneSession, model.PlaneIssuance, model.PlaneApplication,
	} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testScope)
		s.Require().NoError(err)
		s.False(decision.Admitted, "plane %s must refuse a suspended identity", plane)
		s.Equal(model.HoldIdentity, decision.HoldLevel)
		s.Empty(decision.Scope, "an identity hold is not scoped to a authentication_method")
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

		for _, plane := range []model.Plane{
			model.PlaneAuthentication, model.PlaneDispatch, model.PlaneRecovery,
			model.PlaneSession, model.PlaneIssuance, model.PlaneApplication,
		} {
			decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testScope)
			s.Require().NoError(err)
			s.True(decision.Admitted, "state %q must not hold at plane %s", state, plane)
			s.Equal(model.HoldNone, decision.HoldLevel)
		}
	}
}

// A suspension member holds whatever the state column says.
func (s *ServiceTestSuite) TestASuspensionMemberHoldsUnderAnyState() {
	entity := s.activeEntity()
	entity.State = providers.EntityState("SOME_FUTURE_STATE")
	entity.AccessState.Suspend = &model.SuspendState{SuspendedAt: s.at(-time.Hour)}
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, model.PlaneAuthentication, testScope)
	s.Require().NoError(err)
	s.False(decision.Admitted)
	s.Equal(model.HoldIdentity, decision.HoldLevel)
}

// A method lock refuses at the authentication plane and names the scope.
func (s *ServiceTestSuite) TestCredentialHoldRefusesAtTheCredentialPlane() {
	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, testScope, model.ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, model.PlaneAuthentication, testScope)
	s.Require().NoError(err)
	s.False(decision.Admitted)
	s.Equal(model.HoldAuthentication, decision.HoldLevel)
	s.Equal(string(testScope), decision.Scope)
}

// A locked password does not hold a passkey.
func (s *ServiceTestSuite) TestALockedScopeDoesNotHoldAnother() {
	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, testScope, model.ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID,
		model.PlaneAuthentication, model.AccessScopePasskey)
	s.Require().NoError(err)
	s.True(decision.Admitted, "a locked password must not hold the passkey")
}

func (s *ServiceTestSuite) TestALockExpiresWithoutAWrite() {
	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, testScope, model.ScopeLock{LockCount: 1, UnlockAt: s.at(-time.Second)})
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, model.PlaneAuthentication, testScope)
	s.Require().NoError(err)
	s.True(decision.Admitted)
}

// A method lock does not reach the session, issuance or recovery planes.
func (s *ServiceTestSuite) TestACredentialHoldDoesNotReachOtherPlanesByDefault() {
	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, testScope, model.ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	for _, plane := range []model.Plane{
		model.PlaneSession, model.PlaneIssuance, model.PlaneRecovery, model.PlaneApplication,
	} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testScope)
		s.Require().NoError(err)
		s.True(decision.Admitted, "plane %s must admit a locked but unsuspended identity", plane)
	}
}

// Under entity granularity one entry under the reserved key holds every method.
func (s *ServiceTestSuite) TestEntityGranularityHoldsEveryAuthenticationMethod() {
	policy := authenticationMethodPolicy()
	policy.Granularity = governanceconfig.GranularityEntity

	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, model.AccessScopeEntity, model.ScopeLock{
		LockCount: 1, UnlockAt: s.at(5 * time.Minute),
	})
	s.expectRead(entity)
	s.expectPolicy(policy)

	for _, scope := range []model.AccessScope{
		testScope,
		model.AccessScopePasskey,
		model.AccessScopeOTP,
		model.AccessScopeMagicLink,
	} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, model.PlaneAuthentication, scope)
		s.Require().NoError(err)
		s.False(decision.Admitted, "scope %s must be held by the entity-wide lock", scope)
		s.Equal(string(model.AccessScopeEntity), decision.Scope)
	}
}

func (s *ServiceTestSuite) TestEntityGranularityAloneDoesNotReachSessionsOrIssuance() {
	policy := authenticationMethodPolicy()
	policy.Granularity = governanceconfig.GranularityEntity

	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, model.AccessScopeEntity, model.ScopeLock{
		LockCount: 1, UnlockAt: s.at(5 * time.Minute),
	})
	s.expectRead(entity)
	s.expectPolicy(policy)

	for _, plane := range []model.Plane{model.PlaneSession, model.PlaneIssuance} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testScope)
		s.Require().NoError(err)
		s.True(decision.Admitted, "plane %s must stay admitted with the flags unset", plane)
	}
}

// An entity-wide lock holds only the authentication and dispatch planes, under either granularity.
func (s *ServiceTestSuite) TestAnEntityWideLockHoldsOnlyTheCredentialAndDispatchPlanes() {
	policy := authenticationMethodPolicy()
	policy.Granularity = governanceconfig.GranularityEntity

	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, model.AccessScopeEntity, model.ScopeLock{
		LockCount: 1, UnlockAt: s.at(5 * time.Minute),
	})
	s.expectRead(entity)
	s.expectPolicy(policy)

	for _, plane := range []model.Plane{model.PlaneAuthentication, model.PlaneDispatch} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testScope)
		s.Require().NoError(err)
		s.False(decision.Admitted, "plane %s must refuse under a live entity-wide lock", plane)
	}

	for _, plane := range []model.Plane{
		model.PlaneSession, model.PlaneIssuance, model.PlaneRecovery, model.PlaneApplication,
	} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testScope)
		s.Require().NoError(err)
		s.True(decision.Admitted, "plane %s must never be held by a lock", plane)
	}

	// It lapses by the clock with no write.
	s.svc.now = func() time.Time { return s.now.Add(10 * time.Minute) }
	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, model.PlaneAuthentication, testScope)
	s.Require().NoError(err)
	s.True(decision.Admitted, "a lock refuses; it does not revoke")
}

// A failure on a scope whose class is not counted is not recorded.
func (s *ServiceTestSuite) TestFailureAtANotLockableScopeIsNotRecorded() {
	err := s.svc.RecordFailure(s.ctx, s.activeEntity().ID, model.AccessScopePasskey)
	s.Require().NoError(err)
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	// The entity is not read.
}

// A failure with no subject is not an error and reads nothing.
func (s *ServiceTestSuite) TestFailureWithNoSubjectRecordsNothing() {
	s.Require().NoError(s.svc.RecordFailure(s.ctx, "", testScope))
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// The policy is resolved from the entity the service reads, not from the caller.
func (s *ServiceTestSuite) TestRecordingResolvesThePolicyFromItsOwnRead() {
	stored := s.activeEntity()
	stored.Category = providers.EntityCategoryAgent
	stored.Type = "service"
	s.expectRead(stored)

	var resolved model.GovernedEntity
	s.policies.On("Resolve", mock.Anything, mock.Anything, testScope).
		Run(func(args mock.Arguments) { resolved, _ = args.Get(1).(model.GovernedEntity) }).
		Return(authenticationMethodPolicy()).Once()
	s.port.On("IncrementFailure", mock.Anything, stored.ID, testScope, mock.Anything, mock.Anything).
		Return(model.ScopeLock{FailureCount: 1}, true, nil).Once()

	s.Require().NoError(s.svc.RecordFailure(s.ctx, stored.ID, testScope))
	s.Equal(stored, resolved)
}

// A read failure is returned, not swallowed.
func (s *ServiceTestSuite) TestRecordingReportsAnUnreadableAuthority() {
	s.port.On("GetGovernedEntity", mock.Anything, "e1").
		Return(model.GovernedEntity{}, errors.New("entity store unavailable")).Once()

	s.Require().Error(s.svc.RecordFailure(s.ctx, "e1", testScope))
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A machine credential failure is not read, counted or locked.
func (s *ServiceTestSuite) TestMachineCredentialScopeNeitherCountsNorLocks() {
	entity := s.activeEntity()
	scope := model.AccessScopeSystemCredential

	s.Require().NoError(s.svc.RecordFailure(s.ctx, entity.ID, scope))

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
		Return(model.ScopeLock{FailureCount: 4}, true, nil).Once()

	s.Require().NoError(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
	s.port.AssertNotCalled(s.T(), "FormLock",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// At the threshold a lock forms with the escalation's duration, guarded on the observed counters.
func (s *ServiceTestSuite) TestFailureAtTheThresholdFormsALock() {
	entity := s.activeEntity()
	observed := model.ScopeLock{FailureCount: 5}
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testScope, mock.Anything, mock.Anything).
		Return(observed, true, nil).Once()

	var written model.LockEpisode
	var guard model.ScopeLock
	s.port.On("FormLock", mock.Anything, entity.ID, testScope,
		mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			guard, _ = args.Get(3).(model.ScopeLock)
			written, _ = args.Get(4).(model.LockEpisode)
		}).Return(true, nil).Once()

	s.Require().NoError(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
	s.Equal(1, written.LockCount)
	s.Equal(s.now.Add(5*time.Minute).UTC().Format(time.RFC3339), written.UnlockAt)
	s.Equal(observed, guard, "the formation must be guarded on what the increment observed")
}

func (s *ServiceTestSuite) TestFailureDuringALiveLockFormsNothing() {
	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testScope, mock.Anything, mock.Anything).
		Return(model.ScopeLock{FailureCount: 9, LockCount: 1, UnlockAt: s.at(5 * time.Minute)}, true, nil).
		Once()

	s.Require().NoError(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
	s.port.AssertNotCalled(s.T(), "FormLock",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestFormationStopsWhenAReReadShowsANewerLock() {
	entity := s.activeEntity()
	s.expectPolicy(authenticationMethodPolicy())
	// The first read, then the re-read after the guard loses.
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Once()
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testScope, mock.Anything, mock.Anything).
		Return(model.ScopeLock{FailureCount: 5}, true, nil).Once()
	s.port.On("FormLock", mock.Anything, entity.ID, testScope,
		mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Once()

	refreshed := s.activeEntity()
	putScopeLock(&refreshed.AccessState, testScope, model.ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(refreshed, nil).Once()

	s.Require().NoError(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
}

func (s *ServiceTestSuite) TestFormationRetryIsGuardedOnTheReReadRevision() {
	entity := s.activeEntity()
	s.expectPolicy(authenticationMethodPolicy())
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Once()
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testScope, mock.Anything, mock.Anything).
		Return(model.ScopeLock{FailureCount: 5, Revision: 3}, true, nil).Once()
	s.port.On("FormLock", mock.Anything, entity.ID, testScope,
		model.ScopeLock{FailureCount: 5, Revision: 3}, mock.Anything, mock.Anything).Return(false, nil).Once()

	refreshed := s.activeEntity()
	refreshed.Revision = 9
	putScopeLock(&refreshed.AccessState, testScope, model.ScopeLock{FailureCount: 6})
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(refreshed, nil).Once()
	s.port.On("FormLock", mock.Anything, entity.ID, testScope,
		model.ScopeLock{FailureCount: 6, Revision: 9}, mock.Anything, mock.Anything).Return(true, nil).Once()

	s.Require().NoError(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
}

func (s *ServiceTestSuite) TestFormationStopsWhenAReReadShowsAReset() {
	entity := s.activeEntity()
	s.expectPolicy(authenticationMethodPolicy())
	// Both reads see a row with no entries.
	s.expectRead(entity)
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testScope, mock.Anything, mock.Anything).
		Return(model.ScopeLock{FailureCount: 5}, true, nil).Once()
	s.port.On("FormLock", mock.Anything, entity.ID, testScope,
		mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Once()

	s.Require().NoError(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
}

func (s *ServiceTestSuite) TestFormationRetriesAreBounded() {
	entity := s.activeEntity()
	s.expectPolicy(authenticationMethodPolicy())
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testScope, mock.Anything, mock.Anything).
		Return(model.ScopeLock{FailureCount: 5}, true, nil).Once()

	// The guard never holds and the row keeps looking eligible.
	s.port.On("FormLock", mock.Anything, entity.ID, testScope,
		mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Times(formationRetries)
	eligible := s.activeEntity()
	putScopeLock(&eligible.AccessState, testScope, model.ScopeLock{FailureCount: 5})
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(eligible, nil)

	s.Require().NoError(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
}

func (s *ServiceTestSuite) TestFailureOnAMissingRowIsNotAnError() {
	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testScope, mock.Anything, mock.Anything).
		Return(model.ScopeLock{}, false, nil).Once()

	s.Require().NoError(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
	s.port.AssertNotCalled(s.T(), "FormLock",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestFailureUnderEntityGranularityCountsUnderTheReservedKey() {
	policy := authenticationMethodPolicy()
	policy.Granularity = governanceconfig.GranularityEntity

	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectPolicy(policy)
	s.port.On("IncrementFailure", mock.Anything, entity.ID,
		model.AccessScopeEntity, mock.Anything, mock.Anything).
		Return(model.ScopeLock{FailureCount: 1}, true, nil).Once()

	s.Require().NoError(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
}

func (s *ServiceTestSuite) TestFailureWindowIsPassedAsAnInstant() {
	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	var windowStart time.Time
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testScope, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { windowStart, _ = args.Get(4).(time.Time) }).
		Return(model.ScopeLock{FailureCount: 1}, true, nil).Once()

	s.Require().NoError(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
	s.Equal(s.now.Add(-15*time.Minute), windowStart)
}

// An admitted step clears only its own method's entry, not the entity entry.
func (s *ServiceTestSuite) TestAdmittedStepClearsOnlyObservedMethodEntry() {
	entity := s.activeEntity()
	entity.Revision = 7
	putScopeLock(&entity.AccessState, testScope, model.ScopeLock{FailureCount: 2})
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Once()
	s.expectPolicy(authenticationMethodPolicy())
	s.port.On("ClearAccessStateIfUnchanged", mock.Anything, entity.ID, testScope, int64(7)).Return(true, nil).Once()
	decision, err := s.svc.AdmitAuthenticationStep(s.ctx, entity.ID, testScope)
	s.Require().NoError(err)
	s.True(decision.Admitted)
}

// An admitted step leaves the reserved entry for AdmitSignIn.
func (s *ServiceTestSuite) TestAdmittedStepNeverClearsTheReservedKey() {
	s.expectRead(s.activeEntity())
	decision, err := s.svc.AdmitAuthenticationStep(s.ctx, "e1", model.AccessScopeEntity)
	s.Require().NoError(err)
	s.True(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "ClearAccessStateIfUnchanged",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestDisabledPolicyAdmitsAndWritesNothing() {
	policy := authenticationMethodPolicy()
	policy.Enabled = false

	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectPolicy(policy)

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, model.PlaneAuthentication, testScope)
	s.Require().NoError(err, "a switched-off policy is not an error")
	s.True(decision.Admitted)

	s.Require().NoError(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestDisabledPolicyReleasesAnExistingLock() {
	policy := authenticationMethodPolicy()
	policy.Enabled = false

	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, testScope, model.ScopeLock{LockCount: 3, UnlockAt: s.at(24 * time.Hour)})
	s.expectRead(entity)
	s.expectPolicy(policy)

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, model.PlaneAuthentication, testScope)
	s.Require().NoError(err)
	s.True(decision.Admitted, "a lock must not outlive the policy that formed it being switched off")
}

func (s *ServiceTestSuite) TestDisabledPolicyStillEnforcesAnIdentityHold() {
	policy := authenticationMethodPolicy()
	policy.Enabled = false

	entity := s.activeEntity()
	entity.State = providers.EntityStateSuspended
	s.expectRead(entity)
	s.expectPolicy(policy)

	for _, plane := range []model.Plane{model.PlaneAuthentication, model.PlaneSession, model.PlaneIssuance} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testScope)
		s.Require().NoError(err)
		s.False(decision.Admitted, "plane %s must still refuse a suspended identity", plane)
		s.Equal(model.HoldIdentity, decision.HoldLevel)
	}
}

func (s *ServiceTestSuite) TestResetOnACleanAccountWritesNothing() {
	s.expectRead(s.activeEntity())

	s.expectPolicy(authenticationMethodPolicy())
	decision, err := s.svc.AdmitAuthenticationStep(s.ctx, "e1", testScope)
	s.Require().NoError(err)
	s.True(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "ClearAccessStateIfUnchanged",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestResetIgnoresStateUnderAnUnrelatedScope() {
	other := s.activeEntity()
	putScopeLock(&other.AccessState, model.AccessScopeOTP, model.ScopeLock{FailureCount: 3})
	s.expectRead(other)

	s.expectPolicy(authenticationMethodPolicy())
	decision, err := s.svc.AdmitAuthenticationStep(s.ctx, "e1", testScope)
	s.Require().NoError(err)
	s.True(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "ClearAccessStateIfUnchanged",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestAFailedSuspendReportsTheFailure() {
	s.expectRead(s.activeEntity())
	s.port.On("SetSuspension", mock.Anything, "e1", s.now, "").
		Return(errors.New("database unavailable")).Once()

	s.Error(s.svc.Suspend(s.ctx, "e1", ""))
}

// Suspend writes the state column and the document in one statement.
func (s *ServiceTestSuite) TestSuspendWritesOneAtomicSuspension() {
	s.expectRead(s.activeEntity())
	var order []string
	s.port.On("SetSuspension", mock.Anything, "e1", s.now, "Credential exposure").
		Run(func(mock.Arguments) { order = append(order, "document") }).Return(nil).Once()

	s.Require().NoError(s.svc.Suspend(s.ctx, "e1", "Credential exposure"))
	s.Equal([]string{"document"}, order)
	s.port.AssertNotCalled(s.T(), "ClearAccessState", mock.Anything, mock.Anything, mock.Anything)
}

// The first placement time and note are kept.
func (s *ServiceTestSuite) TestRepeatingSuspendSucceedsWithoutWriting() {
	for name, mutate := range map[string]func(*model.GovernedEntity){
		"both stores":   func(*model.GovernedEntity) {},
		"document only": func(e *model.GovernedEntity) { e.State = providers.EntityStateActive },
		"column only":   func(e *model.GovernedEntity) { e.AccessState.Suspend = nil },
	} {
		s.SetupTest()
		entity := s.suspendedEntity()
		mutate(&entity)
		s.expectRead(entity)

		s.Require().NoErrorf(s.svc.Suspend(s.ctx, "e1", "A different reason"), "%s", name)
		s.port.AssertNotCalled(s.T(), "SetSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	}
}

// suspendedEntity is an account carrying the suspension in both of its stores.
func (s *ServiceTestSuite) suspendedEntity() model.GovernedEntity {
	entity := s.activeEntity()
	entity.State = providers.EntityStateSuspended
	entity.AccessState.Suspend = &model.SuspendState{SuspendedAt: s.at(-time.Hour)}
	return entity
}

func (s *ServiceTestSuite) TestEitherSuspensionStoreAloneHolds() {
	for name, mutate := range map[model.AccessScope]func(*model.GovernedEntity){
		"document only": func(e *model.GovernedEntity) { e.State = providers.EntityStateActive },
		"column only":   func(e *model.GovernedEntity) { e.AccessState.Suspend = nil },
	} {
		s.SetupTest()

		entity := s.suspendedEntity()
		mutate(&entity)
		s.expectRead(entity)
		s.expectPolicy(authenticationMethodPolicy())

		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, model.PlaneAuthentication, testScope)
		s.Require().NoError(err)
		s.Falsef(decision.Admitted, "%s must still hold the identity", name)
		s.Equal(model.HoldIdentity, decision.HoldLevel)
	}
}

// A suspension refuses at every plane, including session, issuance and recovery.
func (s *ServiceTestSuite) TestASuspensionRefusesAtEveryPlane() {
	entity := s.suspendedEntity()
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	for _, plane := range []model.Plane{
		model.PlaneAuthentication, model.PlaneDispatch, model.PlaneRecovery,
		model.PlaneSession, model.PlaneIssuance, model.PlaneApplication,
	} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testScope)
		s.Require().NoError(err)
		s.False(decision.Admitted, "plane %s must refuse a suspended identity", plane)
		s.Equal(model.HoldIdentity, decision.HoldLevel)
	}
}

func (s *ServiceTestSuite) TestASuspensionAppliesWithLockingSwitchedOff() {
	policy := authenticationMethodPolicy()
	policy.Enabled = false

	entity := s.suspendedEntity()
	s.expectRead(entity)
	s.expectPolicy(policy)

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, model.PlaneAuthentication, testScope)
	s.Require().NoError(err)
	s.False(decision.Admitted)
	s.Equal(model.HoldIdentity, decision.HoldLevel)
}

// Suspend reads the entity, writes the suspension and does not clear lock state.
func (s *ServiceTestSuite) TestSuspendResolvesTheSourceBeforeWriting() {
	s.expectRead(s.activeEntity())
	s.port.On("SetSuspension", mock.Anything, "e1", s.now, "").Return(nil).Once()

	s.Require().NoError(s.svc.Suspend(s.ctx, "e1", ""))
	s.port.AssertNotCalled(s.T(), "ClearAccessState", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestUnsuspendOnAUserLandsOnTheEntityHold() {
	s.expectRead(s.suspendedEntity())

	var written *model.EntityHold
	var order []string
	s.port.On("ClearSuspension", mock.Anything, "e1", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			written, _ = args.Get(2).(*model.EntityHold)
			order = append(order, "document")
		}).
		Return(nil).Once()

	s.Require().NoError(s.svc.Unsuspend(s.ctx, "e1"))
	s.Require().NotNil(written, "a user must land held")
	s.Equal(model.PermanentUnlockAt, written.UnlockAt)
	s.Equal(model.ReasonPostSuspension, written.Reason)
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

		var written *model.EntityHold
		s.port.On("ClearSuspension", mock.Anything, "e1", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) { written, _ = args.Get(2).(*model.EntityHold) }).
			Return(nil).Once()

		s.Require().NoError(s.svc.Unsuspend(s.ctx, "e1"))
		s.Nil(written, "category %s must land on ACTIVE with no hold written", category)
	}
}

// The post-suspension hold holds every method, including ones that never lock automatically.
func (s *ServiceTestSuite) TestThePostSuspensionHoldHoldsEveryAuthenticationMethod() {
	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, model.AccessScopeEntity,
		model.ScopeLock{UnlockAt: model.PermanentUnlockAt, Reason: model.ReasonPostSuspension})
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	for _, scope := range []model.AccessScope{
		testScope,
		model.AccessScopePasskey,
		model.AccessScopeOTP,
		model.AccessScopeMagicLink,
	} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, model.PlaneAuthentication, scope)
		s.Require().NoError(err)
		s.False(decision.Admitted, "scope %s must be held by the post-suspension hold", scope)
		s.Equal(model.HoldAuthentication, decision.HoldLevel)
		s.Equal(string(model.AccessScopeEntity), decision.Scope,
			"the refusal names the identity, not the authentication_method that happened to be presented")
	}
}

func (s *ServiceTestSuite) TestThePostSuspensionHoldSurvivesLockingBeingSwitchedOff() {
	policy := authenticationMethodPolicy()
	policy.Enabled = false

	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, model.AccessScopeEntity,
		model.ScopeLock{UnlockAt: model.PermanentUnlockAt, Reason: model.ReasonPostSuspension})
	s.expectRead(entity)
	s.expectPolicy(policy)

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, model.PlaneAuthentication, testScope)
	s.Require().NoError(err)
	s.False(decision.Admitted)
	s.Equal(model.HoldAuthentication, decision.HoldLevel)
}

func (s *ServiceTestSuite) TestThePostSuspensionHoldNeverHoldsRecovery() {
	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, model.AccessScopeEntity,
		model.ScopeLock{UnlockAt: model.PermanentUnlockAt, Reason: model.ReasonPostSuspension})
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, model.PlaneRecovery, testScope)
	s.Require().NoError(err)
	s.True(decision.Admitted)
}

func (s *ServiceTestSuite) TestNoFailureIsCountedUnderALiveEntityHold() {
	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, model.AccessScopeEntity,
		model.ScopeLock{UnlockAt: model.PermanentUnlockAt, Reason: model.ReasonPostSuspension})
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	s.Require().NoError(s.svc.RecordFailure(s.ctx, entity.ID, testScope))
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// Unlock works on a suspended identity and does not release the suspension.
func (s *ServiceTestSuite) TestUnlockIsNotRefusedOnASuspendedIdentity() {
	s.port.On("GetGovernedEntity", mock.Anything, "e1").Return(s.suspendedEntity(), nil).Maybe()
	s.port.On("ClearAccessState", mock.Anything, "e1", mock.Anything).Return(nil).Once()

	s.Require().NoError(s.svc.Unlock(s.ctx, "e1"))
	s.port.AssertNotCalled(s.T(), "ClearSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestUnsuspendRefusesWhenTheReadFails() {
	s.port.On("GetGovernedEntity", mock.Anything, "e1").
		Return(model.GovernedEntity{}, errors.New("database unavailable")).Maybe()

	s.Error(s.svc.Unsuspend(s.ctx, "e1"))
	s.port.AssertNotCalled(s.T(), "ClearSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A lifecycle operation with no subject fails before any write.
func (s *ServiceTestSuite) TestLifecycleOperationsRequireASubject() {
	s.ErrorIs(s.svc.Suspend(s.ctx, "", ""), ErrNoSubject)
	s.ErrorIs(s.svc.Unsuspend(s.ctx, ""), ErrNoSubject)
	s.ErrorIs(s.svc.Unlock(s.ctx, ""), ErrNoSubject)
	s.port.AssertNotCalled(s.T(), "SetSuspension",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	s.port.AssertNotCalled(s.T(), "ClearSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// The entity entry wins under either granularity; the read does not depend on the configuration.
func (s *ServiceTestSuite) TestTheEntityEntryWinsUnderBothGranularities() {
	for _, granularity := range []governanceconfig.Granularity{
		governanceconfig.GranularityAuthenticationMethod, governanceconfig.GranularityEntity,
	} {
		s.SetupTest()

		policy := authenticationMethodPolicy()
		policy.Granularity = granularity

		entity := s.activeEntity()
		// A lapsed method entry under a live entity entry, as a granularity change can leave.
		putScopeLock(&entity.AccessState, testScope,
			model.ScopeLock{LockCount: 2, UnlockAt: s.at(-time.Hour)})
		putScopeLock(&entity.AccessState, model.AccessScopeEntity,
			model.ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
		s.expectRead(entity)
		s.expectPolicy(policy)

		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, model.PlaneAuthentication, testScope)
		s.Require().NoError(err)
		s.Falsef(decision.Admitted, "granularity %s must be held by the entity entry", granularity)
		s.Equal(string(model.AccessScopeEntity), decision.Scope)
		s.Equal(providers.AccessStatusLocked,
			ReportFor(entity.State, entity.AccessState, s.now, governanceconfig.LockEnforcement{}).Value)
	}
}

func (s *ServiceTestSuite) TestALapsedEntityEntryFallsThroughToTheAuthenticationMethod() {
	entity := s.activeEntity()
	putScopeLock(&entity.AccessState, model.AccessScopeEntity,
		model.ScopeLock{LockCount: 1, UnlockAt: s.at(-time.Hour)})
	putScopeLock(&entity.AccessState, testScope,
		model.ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, model.PlaneAuthentication, testScope)
	s.Require().NoError(err)
	s.False(decision.Admitted)
	s.Equal(string(testScope), decision.Scope)
}

// putScopeLock records one lock entry under the member the scope maps to.
func putScopeLock(state *model.AccessState, scope model.AccessScope, entry model.ScopeLock) {
	if scope == model.AccessScopeEntity {
		held := entry
		state.Lock.Entity = &held
		return
	}
	if state.Lock.AuthenticationMethods == nil {
		state.Lock.AuthenticationMethods = make(map[model.AccessScope]model.ScopeLock)
	}
	state.Lock.AuthenticationMethods[scope] = entry
}

// lockedState builds a document holding one entry.
func lockedState(scope model.AccessScope, entry model.ScopeLock) model.AccessState {
	var state model.AccessState
	putScopeLock(&state, scope, entry)
	return state
}

// RealPolicyTestSuite runs the service over the real policy resolver instead of a mock.
type RealPolicyTestSuite struct {
	suite.Suite
	port *EntityStateProviderMock
	ctx  context.Context
	now  time.Time
}

func TestRealPolicyTestSuite(t *testing.T) {
	suite.Run(t, new(RealPolicyTestSuite))
}

func (s *RealPolicyTestSuite) SetupTest() {
	s.port = NewEntityStateProviderMock(s.T())
	s.now = time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	s.ctx = context.Background()
}

// serviceWith builds the service over the real resolver for the given configuration.
func (s *RealPolicyTestSuite) serviceWith(cfg governanceconfig.AccountAccessValue) *service {
	svc := newService(s.port, newTestPolicyResolver(cfg), nil, nil)
	svc.now = func() time.Time { return s.now }
	return svc
}

func (s *RealPolicyTestSuite) at(offset time.Duration) string {
	return s.now.Add(offset).UTC().Format(time.RFC3339)
}

// entityGranularityConfig is a user category in entity mode, with method blocks that must be ignored.
func entityGranularityConfig() governanceconfig.AccountAccessValue {
	return governanceconfig.AccountAccessValue{
		User: &governanceconfig.AccountAccessCategory{
			LockGranularity: stringPtr("entity"),
			Scopes: map[string]governanceconfig.AccountAccessPolicy{
				string(model.AccessScopeEntity): {
					Enabled:              boolPtr(true),
					Threshold:            intPtr(5),
					FailureWindowSeconds: intPtr(900),
					LockDurationsSeconds: []int{300, 900},
					LockDecaySeconds:     intPtr(86400),
				},
				// Configured on purpose: entity granularity must ignore these.
				string(model.AccessScopeCredential): {
					Threshold: intPtr(99), LockDurationsSeconds: []int{60},
				},
				string(model.AccessScopeOTP): {
					Threshold: intPtr(2), LockDurationsSeconds: []int{60},
				},
			},
		},
	}
}

func (s *RealPolicyTestSuite) lockedEntityWide() model.GovernedEntity {
	return model.GovernedEntity{
		ID:       "e1",
		Category: providers.EntityCategoryUser,
		Type:     "person",
		State:    providers.EntityStateActive,
		AccessState: lockedState(model.AccessScopeEntity,
			model.ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)}),
	}
}

func (s *RealPolicyTestSuite) TestEntityWideLockHoldsAuthenticationMethodsWithNoBlockOfTheirOwn() {
	svc := s.serviceWith(entityGranularityConfig())
	entity := s.lockedEntityWide()
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Maybe()

	for _, scope := range []model.AccessScope{
		model.AccessScopeCredential,
		model.AccessScopeOTP,
		model.AccessScopePasskey,
		model.AccessScopeMagicLink,
		model.AccessScopeFederated,
		model.AccessScopeOpenID4VP,
	} {
		decision, err := svc.EvaluateAccess(s.ctx, entity.ID, model.PlaneAuthentication, scope)
		s.Require().NoError(err)
		s.False(decision.Admitted, "scope %s must be held by the entity-wide lock", scope)
		s.Equal(string(model.AccessScopeEntity), decision.Scope)
	}
}

// Under method granularity, a federated sign-in obeys an entity-wide lock and ignores a password lock.
func (s *RealPolicyTestSuite) TestAFederatedSignInObeysOnlyAnEntityWideLock() {
	svc := s.serviceWith(testAccountAccessConfig())

	entityWide := s.lockedEntityWide()
	s.port.On("GetGovernedEntity", mock.Anything, entityWide.ID).Return(entityWide, nil).Once()
	decision, err := svc.EvaluateAccess(s.ctx, entityWide.ID, model.PlaneAuthentication, model.AccessScopeFederated)
	s.Require().NoError(err)
	s.False(decision.Admitted, "an entity-wide lock holds a federated sign-in")

	passwordOnly := s.lockedEntityWide()
	passwordOnly.ID = "e2"
	passwordOnly.AccessState = lockedState(model.AccessScopeCredential,
		model.ScopeLock{LockCount: 1, UnlockAt: s.at(time.Minute)})
	s.port.On("GetGovernedEntity", mock.Anything, passwordOnly.ID).Return(passwordOnly, nil).Once()
	decision, err = svc.EvaluateAccess(s.ctx, passwordOnly.ID, model.PlaneAuthentication, model.AccessScopeFederated)
	s.Require().NoError(err)
	s.True(decision.Admitted, "a password lock does not hold a federated sign-in")
}

// The session and issuance planes present no method: the policy still resolves and the lock does not apply.
func (s *RealPolicyTestSuite) TestALockIsNotReachedWithNoAuthenticationMethodPresent() {
	svc := s.serviceWith(entityGranularityConfig())
	entity := s.lockedEntityWide()
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Maybe()

	for _, plane := range []model.Plane{model.PlaneSession, model.PlaneIssuance, model.PlaneRecovery} {
		decision, err := svc.EvaluateAccess(s.ctx, entity.ID, plane, "")
		s.Require().NoError(err)
		s.True(decision.Admitted, "plane %s must never be held by a lock", plane)
	}

	// The same lock, at the plane it does hold.
	decision, err := svc.EvaluateAccess(s.ctx, entity.ID, model.PlaneAuthentication, "")
	s.Require().NoError(err)
	s.False(decision.Admitted)
}

// Under entity granularity a password failure uses the entity block's threshold, not the credential block's.
func (s *RealPolicyTestSuite) TestEntityGranularityUsesOneThresholdForEveryAuthenticationMethod() {
	svc := s.serviceWith(entityGranularityConfig())
	entity := model.GovernedEntity{
		ID: "e1", Category: providers.EntityCategoryUser, State: providers.EntityStateActive,
		AccessState: model.AccessState{},
	}

	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Maybe()

	// Both methods count under the reserved key, with the entity block's 15-minute window.
	for _, scope := range []model.AccessScope{
		model.AccessScopeCredential,
		model.AccessScopeOTP,
	} {
		s.port.On("IncrementFailure", mock.Anything, entity.ID,
			model.AccessScopeEntity, s.now, s.now.Add(-15*time.Minute)).
			Return(model.ScopeLock{FailureCount: 4}, true, nil).Once()

		s.Require().NoError(svc.RecordFailure(s.ctx, entity.ID, scope))
	}

	// Four failures are below the entity threshold of 5; the OTP threshold of 2 would have formed a lock.
	s.port.AssertNotCalled(s.T(), "FormLock",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *RealPolicyTestSuite) TestEntityGranularityFormsTheLockFromTheEntityEscalation() {
	svc := s.serviceWith(entityGranularityConfig())
	entity := model.GovernedEntity{
		ID: "e1", Category: providers.EntityCategoryUser, State: providers.EntityStateActive,
		AccessState: model.AccessState{},
	}

	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Maybe()
	s.port.On("IncrementFailure", mock.Anything, entity.ID,
		model.AccessScopeEntity, mock.Anything, mock.Anything).
		Return(model.ScopeLock{FailureCount: 5}, true, nil).Once()

	var episode model.LockEpisode
	s.port.On("FormLock", mock.Anything, entity.ID, model.AccessScopeEntity,
		mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { episode, _ = args.Get(4).(model.LockEpisode) }).
		Return(true, nil).Once()

	s.Require().NoError(svc.RecordFailure(s.ctx, entity.ID, model.AccessScopeCredential))
	s.Equal(s.now.Add(5*time.Minute).UTC().Format(time.RFC3339), episode.UnlockAt,
		"the entity block's first duration is 5 minutes; the credential block's is 1")
}

func (s *RealPolicyTestSuite) TestEntityGranularityStillDoesNotCountUncountableAuthenticationMethods() {
	svc := s.serviceWith(entityGranularityConfig())

	// No entity is set up: the class is checked before the read.
	s.Require().NoError(svc.RecordFailure(s.ctx, "e1", model.AccessScopePasskey))
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *RealPolicyTestSuite) TestAuthenticationMethodGranularityResolvesEachAuthenticationMethodsOwnPolicy() {
	svc := s.serviceWith(testAccountAccessConfig())
	entity := model.GovernedEntity{
		ID: "e1", Category: providers.EntityCategoryUser, State: providers.EntityStateActive,
		AccessState: model.AccessState{},
	}

	// The otp block's window is 10 minutes, against credential's 15.
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Maybe()
	s.port.On("IncrementFailure", mock.Anything, entity.ID,
		model.AccessScopeOTP, s.now, s.now.Add(-10*time.Minute)).
		Return(model.ScopeLock{FailureCount: 3}, true, nil).Once()
	s.port.On("FormLock", mock.Anything, entity.ID, model.AccessScopeOTP,
		mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Once()

	s.Require().NoError(svc.RecordFailure(s.ctx, entity.ID, model.AccessScopeOTP))
}

func (s *RealPolicyTestSuite) TestAuthenticationMethodGranularityKeepsScopesIsolated() {
	svc := s.serviceWith(testAccountAccessConfig())
	entity := model.GovernedEntity{
		ID: "e1", Category: providers.EntityCategoryUser, State: providers.EntityStateActive,
		AccessState: lockedState(model.AccessScopeCredential,
			model.ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)}),
	}
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Maybe()

	held, err := svc.EvaluateAccess(s.ctx, entity.ID,
		model.PlaneAuthentication, model.AccessScopeCredential)
	s.Require().NoError(err)
	s.False(held.Admitted)

	for _, scope := range []model.AccessScope{
		model.AccessScopeOTP,
		model.AccessScopePasskey,
	} {
		decision, err := svc.EvaluateAccess(s.ctx, entity.ID, model.PlaneAuthentication, scope)
		s.Require().NoError(err)
		s.True(decision.Admitted, "scope %s must survive a credential lock", scope)
	}

	// The session and issuance planes stay open in either mode.
	for _, plane := range []model.Plane{model.PlaneSession, model.PlaneIssuance} {
		decision, err := svc.EvaluateAccess(s.ctx, entity.ID,
			plane, model.AccessScopeCredential)
		s.Require().NoError(err)
		s.True(decision.Admitted, "plane %s must be unaffected under authentication_method granularity", plane)
	}
}

// An explicitly disabled agent policy resolves through the real config: no error, no write.
func (s *RealPolicyTestSuite) TestAgentCategoryCountsNothing() {
	svc := s.serviceWith(testAccountAccessConfig())
	entity := model.GovernedEntity{
		ID: "a1", Category: providers.EntityCategoryAgent, State: providers.EntityStateActive,
	}
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Maybe()

	decision, err := svc.EvaluateAccess(s.ctx, entity.ID, model.PlaneAuthentication,
		model.AccessScopeCredential)
	s.Require().NoError(err)
	s.True(decision.Admitted, "an agent with locking off is admitted, not errored")

	s.Require().NoError(svc.RecordFailure(s.ctx, entity.ID, model.AccessScopeCredential))
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A lock formed under method granularity is not read after switching to entity granularity.
func (s *RealPolicyTestSuite) TestAAuthenticationMethodLockGoesInertWhenGranularityFlips() {
	entity := model.GovernedEntity{
		ID: "e1", Category: providers.EntityCategoryUser, State: providers.EntityStateActive,
		AccessState: lockedState(model.AccessScopeCredential,
			model.ScopeLock{LockCount: 1, UnlockAt: s.at(5 * time.Minute)}),
	}
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Maybe()

	// Under the mode it was formed in, it holds.
	held, err := s.serviceWith(testAccountAccessConfig()).EvaluateAccess(s.ctx, entity.ID,
		model.PlaneAuthentication, model.AccessScopeCredential)
	s.Require().NoError(err)
	s.False(held.Admitted)

	// Flipped to entity granularity, the same stored entry is not read at all.
	admitted, err := s.serviceWith(entityGranularityConfig()).EvaluateAccess(s.ctx, entity.ID,
		model.PlaneAuthentication, model.AccessScopeCredential)
	s.Require().NoError(err)
	s.True(admitted.Admitted, "an entry under the inactive key must not hold")
}

// In either granularity a step clears its method entry and a completed sign-in clears a lapsed
// entity entry.
func (s *RealPolicyTestSuite) TestStepAndGrantClearTheirOwnKeysInEitherMode() {
	for name, cfg := range map[string]governanceconfig.AccountAccessValue{
		"authentication_method": testAccountAccessConfig(),
		"entity":                entityGranularityConfig(),
	} {
		s.Run(name, func() {
			port := NewEntityStateProviderMock(s.T())
			svc := newService(port, newTestPolicyResolver(cfg), nil, nil)
			svc.now = func() time.Time { return s.now }

			// Both keys hold lapsed state.
			port.On("GetGovernedEntity", mock.Anything, "e1").Return(model.GovernedEntity{
				ID: "e1", Category: providers.EntityCategoryUser, State: providers.EntityStateActive,
				AccessState: model.AccessState{
					Lock: model.LockState{
						Entity: &model.ScopeLock{LockCount: 1},
						AuthenticationMethods: map[model.AccessScope]model.ScopeLock{
							model.AccessScopeCredential: {LockCount: 1},
						},
					},
				},
			}, nil).Twice()

			port.On("ClearAccessStateIfUnchanged", mock.Anything, "e1", model.AccessScopeCredential, int64(0)).
				Return(true, nil).Once()
			step, err := svc.AdmitAuthenticationStep(s.ctx, "e1", model.AccessScopeCredential)
			s.Require().NoError(err)
			s.True(step.Admitted)

			port.On("ClearAccessStateIfUnchanged", mock.Anything, "e1", model.AccessScopeEntity, int64(0)).
				Return(true, nil).Once()
			decision, err := svc.AdmitSignIn(s.ctx, "e1")
			s.Require().NoError(err)
			s.True(decision.Admitted, "a lapsed record must not hold a completed sign-in")
		})
	}
}

func (s *ServiceTestSuite) TestUnsuspendRefusesAnIdentityThatIsNotSuspended() {
	s.expectRead(s.activeEntity())
	s.ErrorIs(s.svc.Unsuspend(s.ctx, "e1"), ErrNotApplicable)
	s.port.AssertNotCalled(s.T(), "ClearSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// expectActivity registers the activity policy the category resolves to.
func (s *ServiceTestSuite) expectActivity(policy governanceconfig.ActivityPolicy) {
	s.policies.On("ActivityPolicies", mock.Anything).
		Return(governanceconfig.ActivityPolicies{User: policy}).Once()
	s.port.On("GetEntityProfile", mock.Anything, mock.Anything).
		Return(&providers.Entity{Category: providers.EntityCategoryUser}, nil).Maybe()
}

// A recording category writes the stamp and passes the resolution window to the write.
func (s *ServiceTestSuite) TestRecordLoginStampsWhenTheCategoryRecords() {
	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectActivity(governanceconfig.ActivityPolicy{Record: true, Resolution: time.Hour})
	s.port.On("RecordLogin", mock.Anything, entity.ID, s.now, s.now.Add(-time.Hour)).
		Return(nil).Once()

	s.Require().NoError(s.svc.RecordLogin(s.ctx, entity.ID))
}

func (s *ServiceTestSuite) TestRecordLoginSkipsEntityReadsWhenNoCategoryRecords() {
	s.policies.On("ActivityPolicies", mock.Anything).Return(governanceconfig.ActivityPolicies{}).Once()

	s.Require().NoError(s.svc.RecordLogin(s.ctx, "e1"))

	s.port.AssertNotCalled(s.T(), "GetEntityProfile", mock.Anything, mock.Anything)
	s.port.AssertNotCalled(s.T(), "GetGovernedEntity", mock.Anything, mock.Anything)
	s.port.AssertNotCalled(s.T(), "RecordLogin",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// Once any category records, the entity's own category decides.
func (s *ServiceTestSuite) TestRecordLoginWritesNothingWhenThisCategoryDoesNot() {
	s.policies.On("ActivityPolicies", mock.Anything).
		Return(governanceconfig.ActivityPolicies{User: governanceconfig.ActivityPolicy{Record: true}}).Once()
	s.port.On("GetEntityProfile", mock.Anything, "e1").
		Return(&providers.Entity{Category: providers.EntityCategoryAgent}, nil).Once()

	s.Require().NoError(s.svc.RecordLogin(s.ctx, "e1"))

	s.port.AssertNotCalled(s.T(), "RecordLogin",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// The window is passed to the write, so the store compares without a read-modify-write.
func (s *ServiceTestSuite) TestRecordLoginPassesTheResolutionWindowToTheWrite() {
	entity := s.activeEntity()
	s.expectActivity(governanceconfig.ActivityPolicy{Record: true, Resolution: 15 * time.Minute})
	s.port.On("RecordLogin", mock.Anything, entity.ID, s.now, s.now.Add(-15*time.Minute)).
		Return(nil).Once()

	s.Require().NoError(s.svc.RecordLogin(s.ctx, entity.ID))
}

// A zero resolution collapses the window to a single instant.
func (s *ServiceTestSuite) TestRecordLoginWithNoResolutionCollapsesTheWindow() {
	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectActivity(governanceconfig.ActivityPolicy{Record: true, Resolution: 0})
	s.port.On("RecordLogin", mock.Anything, entity.ID, s.now, s.now).Return(nil).Once()

	s.Require().NoError(s.svc.RecordLogin(s.ctx, entity.ID))
}

func (s *ServiceTestSuite) TestRecordLoginWithNoSubjectDoesNothing() {
	s.Require().NoError(s.svc.RecordLogin(s.ctx, ""))

	s.port.AssertNotCalled(s.T(), "GetEntityProfile", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestRecordLoginIsNotAuthorized() {
	authorizer := &recordingAuthorizer{}
	s.svc = newService(s.port, s.policies, nil, authorizer)
	s.svc.now = func() time.Time { return s.now }
	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectActivity(governanceconfig.ActivityPolicy{Record: true, Resolution: time.Hour})
	s.port.On("RecordLogin", mock.Anything, entity.ID, s.now, s.now.Add(-time.Hour)).
		Return(nil).Once()

	s.Require().NoError(s.svc.RecordLogin(s.ctx, entity.ID))

	s.Zero(authorizer.calls)
}

func (s *ServiceTestSuite) TestRecordLoginNeverReadsTheRuntimeRow() {
	s.expectActivity(governanceconfig.ActivityPolicy{Record: true, Resolution: time.Hour})
	s.port.On("RecordLogin", mock.Anything, "e1", s.now, s.now.Add(-time.Hour)).Return(nil).Once()

	s.Require().NoError(s.svc.RecordLogin(s.ctx, "e1"))

	s.port.AssertNotCalled(s.T(), "GetGovernedEntity", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestAdmitAuthenticationStepRefusesWithoutReset() {
	entity := s.activeEntity()
	entity.State = providers.EntityStateSuspended
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Once()
	s.expectPolicy(authenticationMethodPolicy())
	decision, err := s.svc.AdmitAuthenticationStep(s.ctx, entity.ID, testScope)
	s.Require().NoError(err)
	s.False(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "ClearAccessStateIfUnchanged",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestAdmitAuthenticationStepReadFailureDoesNotReset() {
	readErr := errors.New("database unavailable")
	s.port.On("GetGovernedEntity", mock.Anything, "e1").Return(model.GovernedEntity{}, readErr).Once()
	decision, err := s.svc.AdmitAuthenticationStep(s.ctx, "e1", testScope)
	s.Require().ErrorIs(err, readErr)
	s.False(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "ClearAccessStateIfUnchanged",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestAdmitAuthenticationStepResetsOnlyAfterAdmission() {
	for _, resetErr := range []error{nil, errors.New("write failed")} {
		s.Run(fmt.Sprintf("reset error %v", resetErr), func() {
			s.SetupTest()
			entity := s.activeEntity()
			putScopeLock(&entity.AccessState, testScope, model.ScopeLock{FailureCount: 2})
			read := s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Once()
			policy := s.policies.On("Resolve", mock.Anything, mock.Anything, testScope).
				Return(authenticationMethodPolicy()).Once()
			s.port.On("ClearAccessStateIfUnchanged", mock.Anything, entity.ID, testScope, int64(0)).
				NotBefore(read, policy).Return(false, resetErr).Once()
			decision, err := s.svc.AdmitAuthenticationStep(s.ctx, entity.ID, testScope)
			s.Require().NoError(err)
			s.True(decision.Admitted)
		})
	}
}

func (s *ServiceTestSuite) TestAdmitAuthenticationStepUngovernedSubjectDoesNoIO() {
	decision, err := s.svc.AdmitAuthenticationStep(s.ctx, "", testScope)
	s.Require().NoError(err)
	s.True(decision.Admitted)
}

func testAccountAccessConfig() governanceconfig.AccountAccessValue {
	return governanceconfig.AccountAccessValue{
		User: &governanceconfig.AccountAccessCategory{
			LockGranularity: stringPtr("authentication_method"),
			Default: &governanceconfig.AccountAccessPolicy{
				Enabled:              boolPtr(true),
				Threshold:            intPtr(5),
				FailureWindowSeconds: intPtr(900),
				LockDurationsSeconds: []int{300, 900, 3600, 86400},
				LockDecaySeconds:     intPtr(86400),
			},
			Scopes: map[string]governanceconfig.AccountAccessPolicy{
				// States only what differs from the default.
				string(model.AccessScopeOTP): {
					Threshold:            intPtr(3),
					FailureWindowSeconds: intPtr(600),
					LockDurationsSeconds: []int{300, 900},
					LockDecaySeconds:     intPtr(3600),
				},
			},
		},
		Agent: &governanceconfig.AccountAccessCategory{
			LockGranularity: stringPtr("authentication_method"),
			Default:         &governanceconfig.AccountAccessPolicy{Enabled: boolPtr(false)},
		},
	}
}

func boolPtr(v bool) *bool { return &v }
func intPtr(v int) *int    { return &v }

// A completed sign-in clears a lapsed entity-wide record: the counter and the escalation.
func (s *ServiceTestSuite) TestAdmitSignInClearsALapsedEntityRecord() {
	entity := s.activeEntity()
	entity.AccessState.Lock.Entity = &model.ScopeLock{FailureCount: 2, LockCount: 1, UnlockAt: s.at(-time.Second)}
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())
	s.port.On("ClearAccessStateIfUnchanged", mock.Anything, entity.ID, model.AccessScopeEntity, int64(0)).
		Return(true, nil).Once()

	decision, err := s.svc.AdmitSignIn(s.ctx, entity.ID)
	s.Require().NoError(err)
	s.True(decision.Admitted)
}

// A live entity-wide lock, including the permanent one Unsuspend writes, is never cleared.
func (s *ServiceTestSuite) TestAdmitSignInNeverClearsALiveLock() {
	for name, unlockAt := range map[string]string{
		"timed":     s.at(5 * time.Minute),
		"permanent": model.PermanentUnlockAt,
	} {
		s.Run(name, func() {
			s.SetupTest()
			entity := s.activeEntity()
			entity.AccessState.Lock.Entity = &model.ScopeLock{LockCount: 1, UnlockAt: unlockAt}
			s.expectRead(entity)
			s.expectPolicy(authenticationMethodPolicy())

			decision, err := s.svc.AdmitSignIn(s.ctx, entity.ID)
			s.Require().NoError(err)
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
	entity.AccessState.Lock.Entity = &model.ScopeLock{FailureCount: 2}
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())

	decision, err := s.svc.AdmitSignIn(s.ctx, entity.ID)
	s.Require().NoError(err)
	s.False(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "ClearAccessStateIfUnchanged",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A clean account and a failed clear both admit, and a clean one writes nothing.
func (s *ServiceTestSuite) TestAdmitSignInOnACleanAccountWritesNothing() {
	s.expectRead(s.activeEntity())
	s.expectPolicy(authenticationMethodPolicy())

	decision, err := s.svc.AdmitSignIn(s.ctx, "e1")
	s.Require().NoError(err)
	s.True(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "ClearAccessStateIfUnchanged",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestAdmitSignInClearFailureStillAdmits() {
	entity := s.activeEntity()
	entity.AccessState.Lock.Entity = &model.ScopeLock{FailureCount: 2}
	s.expectRead(entity)
	s.expectPolicy(authenticationMethodPolicy())
	s.port.On("ClearAccessStateIfUnchanged", mock.Anything, entity.ID, mock.Anything, mock.Anything).
		Return(false, errors.New("write failed")).Once()

	decision, err := s.svc.AdmitSignIn(s.ctx, entity.ID)
	s.Require().NoError(err)
	s.True(decision.Admitted)
}

// An unreadable authority refuses, and an ungoverned subject is admitted without IO.
func (s *ServiceTestSuite) TestAdmitSignInFailsClosedAndSkipsTheUngoverned() {
	readErr := errors.New("database unavailable")
	s.port.On("GetGovernedEntity", mock.Anything, "e1").Return(model.GovernedEntity{}, readErr).Once()
	decision, err := s.svc.AdmitSignIn(s.ctx, "e1")
	s.Require().ErrorIs(err, readErr)
	s.False(decision.Admitted)

	decision, err = s.svc.AdmitSignIn(s.ctx, "")
	s.Require().NoError(err)
	s.True(decision.Admitted)
}

type recordingAuthorizer struct {
	sysauthz.SystemAuthorizationServiceInterface
	allowed bool
	failure *tidcommon.ServiceError
	calls   int
	action  security.Action
	target  sysauthz.ActionContext
}

func (a *recordingAuthorizer) IsActionAllowed(_ context.Context, action security.Action,
	target *sysauthz.ActionContext) (bool, *tidcommon.ServiceError) {
	a.calls++
	a.action, a.target = action, *target
	return a.allowed, a.failure
}

func governedSubject(category providers.EntityCategory) model.GovernedEntity {
	return model.GovernedEntity{ID: "e1", Category: category, OUID: "ou-a", State: providers.EntityStateActive}
}

func callerContext(subject string) context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(subject, "ou-a", "", nil, nil))
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
			authz := &recordingAuthorizer{allowed: false}
			svc := newService(port, nil, nil, authz)
			for _, operation := range []struct {
				name string
				run  func() error
			}{
				{"suspend", func() error { return svc.Suspend(callerContext("operator"), "e1", "") }},
				{"validate suspend", func() error { return svc.ValidateSuspend(callerContext("operator"), "e1") }},
				{"unsuspend", func() error { return svc.Unsuspend(callerContext("operator"), "e1") }},
				{"unlock", func() error { return svc.Unlock(callerContext("operator"), "e1") }},
			} {
				s.Run(operation.name, func() {
					s.Require().ErrorIs(operation.run(), ErrNotAuthorized)
					s.Require().Equal(tc.action, authz.action)
					s.Require().Equal(tc.resource, authz.target.ResourceType)
					s.Require().Equal("ou-a", authz.target.OUID)
					s.Require().Equal("e1", authz.target.ResourceID)
				})
			}
			s.Require().Equal(4, authz.calls)
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
	authz := &recordingAuthorizer{allowed: false}

	s.Require().ErrorIs(newService(port, nil, nil, authz).Suspend(callerContext("operator"), "e1", ""),
		ErrNotAuthorized)
	s.Require().Equal(1, authz.calls)
	port.AssertNotCalled(s.T(), "SetSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// The suspend pre-check authorizes and writes nothing, even on a suspended identity.
func (s *ServiceTestSuite) TestValidateSuspendWritesNothing() {
	suspended := governedSubject(providers.EntityCategoryUser)
	suspended.State = providers.EntityStateSuspended
	port := NewEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1").Return(suspended, nil)
	authz := &recordingAuthorizer{allowed: true}
	svc := newService(port, nil, nil, authz)

	s.Require().NoError(svc.ValidateSuspend(callerContext("operator"), "e1"))
	s.Require().Equal(1, authz.calls)
	s.Require().ErrorIs(svc.ValidateSuspend(callerContext("operator"), ""), ErrNoSubject)
	port.AssertNotCalled(s.T(), "SetSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestLifecycleAuthorizationFailureRefuses() {
	port := NewEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1").Return(governedSubject(providers.EntityCategoryAgent), nil)
	authz := &recordingAuthorizer{failure: &tidcommon.InternalServerError}
	s.Require().ErrorIs(newService(port, nil, nil, authz).Unlock(callerContext("operator"), "e1"), ErrNotAuthorized)
	port.AssertNotCalled(s.T(), "ClearAccessState", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestLifecycleSelfTargetRefusedBeforeOwnerShortcut() {
	for _, category := range []providers.EntityCategory{providers.EntityCategoryUser, providers.EntityCategoryAgent} {
		authz := &recordingAuthorizer{allowed: true}
		svc := newService(nil, nil, nil, authz)
		s.Require().ErrorIs(svc.authorize(callerContext("e1"), governedSubject(category)), ErrNotAuthorized)
		s.Require().Zero(authz.calls)
		s.Require().NoError(svc.authorize(security.WithRuntimeContext(callerContext("e1")), governedSubject(category)))
		s.Require().Equal(1, authz.calls)
	}
}

func (s *ServiceTestSuite) TestApplicationHasNoSystemAuthorizationVocabulary() {
	authz := &recordingAuthorizer{allowed: false}
	s.Require().NoError(newService(nil, nil, nil, authz).authorize(callerContext("operator"),
		governedSubject(providers.EntityCategoryApp)))
	s.Require().Zero(authz.calls)
}

func (s *ServiceTestSuite) TestEmbeddableGovernanceWithoutAuthorization() {
	s.Require().NoError(newService(nil, nil, nil, nil).authorize(context.Background(),
		governedSubject(providers.EntityCategoryUser)))
}

// LockFormationNotificationTestSuite covers which failures schedule a lock notice: only the writer
// that commits a new lock.
type LockFormationNotificationTestSuite struct {
	suite.Suite

	port     *EntityStateProviderMock
	notifier *LockNotifierMock
	svc      *service
	now      time.Time
	ctx      context.Context
}

func TestLockFormationNotificationTestSuite(t *testing.T) {
	suite.Run(t, new(LockFormationNotificationTestSuite))
}

func (s *LockFormationNotificationTestSuite) SetupTest() {
	s.port = NewEntityStateProviderMock(s.T())
	s.notifier = NewLockNotifierMock(s.T())
	s.now = time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	s.ctx = context.Background()

	cfg := governanceconfig.AccountAccessValue{
		User: &governanceconfig.AccountAccessCategory{
			LockGranularity: stringPtr("authentication_method"),
			Default: &governanceconfig.AccountAccessPolicy{
				Enabled:              boolPtr(true),
				Threshold:            intPtr(3),
				FailureWindowSeconds: intPtr(900),
				LockDurationsSeconds: []int{300},
				LockDecaySeconds:     intPtr(86400),
			},
		},
	}
	s.svc = newService(s.port, newTestPolicyResolver(cfg), s.notifier, nil)
	s.svc.now = func() time.Time { return s.now }
}

func (s *LockFormationNotificationTestSuite) governedUser() model.GovernedEntity {
	return model.GovernedEntity{
		ID:       "user-1",
		Category: providers.EntityCategoryUser,
		Type:     "member",
		State:    providers.EntityStateActive,
	}
}

// expectFailureReaching sets up a failure whose increment lands on the threshold.
func (s *LockFormationNotificationTestSuite) expectFailureReaching(count int) {
	entity := s.governedUser()
	s.port.EXPECT().GetGovernedEntity(mock.Anything, entity.ID).Return(entity, nil)
	s.port.EXPECT().IncrementFailure(mock.Anything, entity.ID,
		model.AccessScopeCredential, mock.Anything, mock.Anything).
		Return(model.ScopeLock{FailureCount: count}, true, nil)
}

// A committed lock notifies with the scope that was held.
func (s *LockFormationNotificationTestSuite) TestCommittedLockNotifies() {
	s.expectFailureReaching(3)
	s.port.EXPECT().FormLock(mock.Anything, "user-1", model.AccessScopeCredential,
		mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	s.notifier.EXPECT().NotifyLockFormed(mock.Anything, mock.MatchedBy(func(e model.GovernedEntity) bool {
		return e.ID == "user-1"
	}), model.AccessScopeCredential, mock.MatchedBy(func(episode model.LockEpisode) bool {
		return episode.Duration == 5*time.Minute && episode.UnlockAt == s.now.Add(5*time.Minute).Format(time.RFC3339)
	})).Return()

	s.Require().NoError(s.svc.RecordFailure(s.ctx, "user-1", model.AccessScopeCredential))
}

func (s *LockFormationNotificationTestSuite) TestOrdinaryFailureDoesNotNotify() {
	s.expectFailureReaching(1)

	s.Require().NoError(s.svc.RecordFailure(s.ctx, "user-1", model.AccessScopeCredential))
	s.notifier.AssertNotCalled(s.T(), "NotifyLockFormed", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// Of two concurrent failures, only the guard's winner notifies.
func (s *LockFormationNotificationTestSuite) TestLosingWriterDoesNotNotify() {
	s.expectFailureReaching(3)
	s.port.EXPECT().FormLock(mock.Anything, "user-1", model.AccessScopeCredential,
		mock.Anything, mock.Anything, mock.Anything).Return(false, nil)

	// The re-read shows the lock another writer formed, so this attempt stops.
	locked := s.governedUser()
	locked.AccessState = model.AccessState{Lock: model.LockState{
		AuthenticationMethods: map[model.AccessScope]model.ScopeLock{
			model.AccessScopeCredential: {
				FailureCount: 3,
				LockCount:    1,
				UnlockAt:     s.now.Add(5 * time.Minute).UTC().Format(time.RFC3339),
			},
		}}}
	s.port.EXPECT().GetGovernedEntity(mock.Anything, "user-1").Return(locked, nil)

	s.Require().NoError(s.svc.RecordFailure(s.ctx, "user-1", model.AccessScopeCredential))
	s.notifier.AssertNotCalled(s.T(), "NotifyLockFormed", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A failure under a live hold is not counted, so it does not notify again.
func (s *LockFormationNotificationTestSuite) TestFailureUnderALiveHoldDoesNotNotify() {
	entity := s.governedUser()
	entity.AccessState = model.AccessState{Lock: model.LockState{Entity: &model.ScopeLock{
		UnlockAt: s.now.Add(5 * time.Minute).UTC().Format(time.RFC3339),
	}}}
	s.port.EXPECT().GetGovernedEntity(mock.Anything, entity.ID).Return(entity, nil)

	s.Require().NoError(s.svc.RecordFailure(s.ctx, "user-1", model.AccessScopeCredential))
	s.notifier.AssertNotCalled(s.T(), "NotifyLockFormed", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *LockFormationNotificationTestSuite) TestNoNotifierStillLocks() {
	svc := newService(s.port, s.svc.policies, nil, nil)
	svc.now = func() time.Time { return s.now }

	s.expectFailureReaching(3)
	formed := s.port.EXPECT().FormLock(mock.Anything, "user-1", model.AccessScopeCredential,
		mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	s.Require().NoError(svc.RecordFailure(s.ctx, "user-1", model.AccessScopeCredential))
	s.NotNil(formed)
}

// A check on an account with no entries resolves no policy. The mock is strict, so a resolve fails
// the test.
func (s *ServiceTestSuite) TestACleanAccountIsAnsweredWithoutAPolicy() {
	entity := s.activeEntity()
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Once()

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, model.PlaneAuthentication, testScope)

	s.Require().NoError(err)
	s.True(decision.Admitted)
	s.policies.AssertNotCalled(s.T(), "Resolve", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestAnApplicationIsAnsweredWithoutAPolicy() {
	entity := s.activeEntity()
	entity.Category = providers.EntityCategoryApp
	entity.AccessState = model.AccessState{
		Lock: model.LockState{
			AuthenticationMethods: map[model.AccessScope]model.ScopeLock{
				model.AccessScopeSystemCredential: {UnlockAt: s.at(time.Hour)},
			},
		},
	}
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Once()

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, model.PlaneApplication, "")

	s.Require().NoError(err)
	s.True(decision.Admitted)
	s.policies.AssertNotCalled(s.T(), "Resolve", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestASuspendedApplicationStillRefuses() {
	entity := s.activeEntity()
	entity.Category = providers.EntityCategoryApp
	entity.State = providers.EntityStateSuspended
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID).Return(entity, nil).Once()
	s.policies.On("Resolve", mock.Anything, mock.Anything, mock.Anything).
		Return(governanceconfig.LockoutPolicy{}).Once()

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, model.PlaneApplication, "")

	s.Require().NoError(err)
	s.False(decision.Admitted)
	s.Equal(model.HoldIdentity, decision.HoldLevel)
}

// auditedService returns a service whose administration events land in the returned slice.
func auditedService(t *testing.T, port EntityStateProvider,
	authz sysauthz.SystemAuthorizationServiceInterface) (*service, *[]*providers.Event) {
	published := []*providers.Event{}
	obs := observabilityprovidermock.NewObservabilityProviderMock(t)
	obs.EXPECT().IsEnabled().Return(true).Maybe()
	obs.EXPECT().PublishEvent(mock.Anything, mock.Anything).
		Run(func(_ context.Context, evt *providers.Event) { published = append(published, evt) }).Maybe()
	svc := newService(port, nil, nil, authz)
	svc.observability = obs
	return svc, &published
}

// A placed suspension is published with its actor and target. Only whether a note was given is
// recorded, never the text.
func (s *ServiceTestSuite) TestASuspensionIsPublishedForAudit() {
	port := NewEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1").Return(governedSubject(providers.EntityCategoryUser), nil)
	port.On("SetSuspension", mock.Anything, "e1", mock.Anything, "Credential exposure").Return(nil)
	svc, published := auditedService(s.T(), port, &recordingAuthorizer{allowed: true})

	s.Require().NoError(svc.Suspend(callerContext("operator"), "e1", "Credential exposure"))

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
		entity  model.GovernedEntity
		allowed bool
		run     func(*service) error
		want    providers.EventType
		failure string
	}{
		{"suspend outside the boundary", agent, false,
			func(s *service) error { return s.Suspend(callerContext("operator"), "e1", "") },
			event.EventTypeIdentitySuspensionFailed, "not_authorized"},
		{"suspend pre-check outside the boundary", agent, false,
			func(s *service) error { return s.ValidateSuspend(callerContext("operator"), "e1") },
			event.EventTypeIdentitySuspensionFailed, "not_authorized"},
		{"unsuspend on an active identity", agent, true,
			func(s *service) error { return s.Unsuspend(callerContext("operator"), "e1") },
			event.EventTypeIdentityUnsuspensionFailed, "not_applicable"},
		{"unlock outside the boundary", agent, false,
			func(s *service) error { return s.Unlock(callerContext("operator"), "e1") },
			event.EventTypeIdentityUnlockFailed, "not_authorized"},
	} {
		s.Run(tc.name, func() {
			port := NewEntityStateProviderMock(s.T())
			port.On("GetGovernedEntity", mock.Anything, "e1").Return(tc.entity, nil)
			svc, published := auditedService(s.T(), port, &recordingAuthorizer{allowed: tc.allowed})

			s.Require().Error(tc.run(svc))

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

func (s *ServiceTestSuite) TestASelfServiceUnlockIsPublishedWithoutAnOperator() {
	port := NewEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1").Return(governedSubject(providers.EntityCategoryUser), nil)
	port.On("ClearAccessState", mock.Anything, "e1", model.AllAccessScopes).Return(nil)
	svc, published := auditedService(s.T(), port, &recordingAuthorizer{allowed: true})

	s.Require().NoError(svc.Unlock(security.WithRuntimeContext(context.Background()), "e1"))

	s.Require().Len(*published, 1)
	evt := (*published)[0]
	s.Require().Equal(string(event.EventTypeIdentityUnlocked), evt.Type)
	s.Require().NotContains(evt.Data, event.DataKey.ActorSub)
}

// A passing pre-check publishes nothing; Suspend records the outcome.
func (s *ServiceTestSuite) TestAPassingSuspendPreCheckIsNotPublished() {
	port := NewEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1").Return(governedSubject(providers.EntityCategoryUser), nil)
	svc, published := auditedService(s.T(), port, &recordingAuthorizer{allowed: true})

	s.Require().NoError(svc.ValidateSuspend(callerContext("operator"), "e1"))
	s.Require().Empty(*published)
}

func (s *ServiceTestSuite) TestARepeatedSuspensionIsPublished() {
	suspended := governedSubject(providers.EntityCategoryUser)
	suspended.State = providers.EntityStateSuspended
	port := NewEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1").Return(suspended, nil)
	svc, published := auditedService(s.T(), port, &recordingAuthorizer{allowed: true})

	s.Require().NoError(svc.Suspend(callerContext("operator"), "e1", ""))

	s.Require().Len(*published, 1)
	s.Require().Equal(string(event.EventTypeIdentitySuspended), (*published)[0].Type)
	s.Require().Equal("Suspension already in place; containment re-run",
		(*published)[0].Data[event.DataKey.Message])
	port.AssertNotCalled(s.T(), "SetSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

type testConfigReader struct {
	value governanceconfig.AccountAccessValue
}

func (r testConfigReader) GetMergedConfig(context.Context, string) (any, *tidcommon.ServiceError) {
	return r.value, nil
}

func stringPtr(v string) *string { return &v }
