// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	port   *entityStateProviderMock
	source *accountAccessSourceInterfaceMock
	svc    *identityGovernanceService
	ctx    context.Context
	now    time.Time
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (s *ServiceTestSuite) SetupTest() {
	s.port = newEntityStateProviderMock(s.T())
	s.source = newAccountAccessSourceInterfaceMock(s.T())
	s.now = time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	s.svc = newIdentityGovernanceService(s.port, s.source, nil, nil, nil)
	s.svc.now = func() time.Time { return s.now }
	s.ctx = context.Background()
}

// newTestService builds the service over a source that always serves cfg.
func newTestService(t *testing.T, port entityStateProvider, cfg accountAccessValue, notifier LockNotifierInterface,
	authorizer sysauthz.SystemAuthorizationServiceInterface) *identityGovernanceService {
	source := newAccountAccessSourceInterfaceMock(t)
	source.EXPECT().Current(mock.Anything).Return(cfg).Maybe()
	return newIdentityGovernanceService(port, source, notifier, authorizer, nil)
}

// admittedFailure returns RecordFailure's error and requires an attempt that read cleanly to be
// admitted, for the tests about counting. Tests of a refusal assert the decision themselves.
func (s *ServiceTestSuite) admittedFailure(decision providers.AccessDecision,
	svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	s.T().Helper()
	if svcErr == nil {
		s.True(decision.Admitted, "an attempt nothing holds must be admitted")
	}
	return svcErr
}

// requireServiceError checks that svcErr is the expected service error.
func (s *ServiceTestSuite) requireServiceError(svcErr *tidcommon.ServiceError, want tidcommon.ServiceError) {
	s.T().Helper()
	s.Require().NotNil(svcErr)
	s.Require().Equal(want.Code, svcErr.Code)
}

const testMethod = providers.AuthenticationMethodCredential

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

// methodGranularityConfig is the default shape: a user locked per authentication method after five
// failures in fifteen minutes, for five and then fifteen minutes.
func methodGranularityConfig() accountAccessValue {
	cfg := entityGranularityConfig()
	cfg.User.LockGranularity = new(granularityAuthenticationMethod)
	return cfg
}

// disabledConfig is methodGranularityConfig with locking switched off.
func disabledConfig() accountAccessValue {
	cfg := methodGranularityConfig()
	cfg.User.Default.Enabled = new(false)
	return cfg
}

// expectConfig serves cfg as the accountAccess section for the rest of the test.
func (s *ServiceTestSuite) expectConfig(cfg accountAccessValue) {
	s.source.EXPECT().Current(mock.Anything).Return(cfg).Maybe()
}

func (s *ServiceTestSuite) expectRead(entity GovernedEntity) {
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Maybe()
}

// An empty subject, such as a client-credentials grant with no user, is admitted.
func (s *ServiceTestSuite) TestUngovernedSubjectIsAdmitted() {
	decision, err := s.svc.EvaluateAccess(s.ctx, "", providers.AccessPlaneIssuance, "")
	s.Require().Nil(err)
	s.True(decision.Admitted)
}

func (s *ServiceTestSuite) TestUnreadableAuthorityRefuses() {
	readErr := errors.New("database unavailable")
	s.port.On("GetGovernedEntity", mock.Anything, "e1", true).Return(nil, readErr).Once()

	decision, err := s.svc.EvaluateAccess(s.ctx, "e1", providers.AccessPlaneAuthentication, testMethod)
	s.requireServiceError(err, tidcommon.InternalServerError)
	s.False(decision.Admitted, "a read failure must not admit")
	s.Empty(decision.Reason)
}

func (s *ServiceTestSuite) TestIdentityHoldAppliesAtEveryPlane() {
	entity := s.activeEntity()
	entity.State = providers.EntityStateSuspended
	s.expectRead(entity)
	s.expectConfig(methodGranularityConfig())

	for _, plane := range []providers.AccessPlane{
		providers.AccessPlaneAuthentication, providers.AccessPlaneDispatch, providers.AccessPlaneRecovery,
		providers.AccessPlaneSession, providers.AccessPlaneIssuance, providers.AccessPlaneApplication,
	} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testMethod)
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
		s.expectConfig(methodGranularityConfig())

		for _, plane := range []providers.AccessPlane{
			providers.AccessPlaneAuthentication, providers.AccessPlaneDispatch, providers.AccessPlaneRecovery,
			providers.AccessPlaneSession, providers.AccessPlaneIssuance, providers.AccessPlaneApplication,
		} {
			decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testMethod)
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
	s.expectConfig(methodGranularityConfig())

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testMethod)
	s.Require().Nil(err)
	s.False(decision.Admitted)
	s.Equal(providers.AccessRefusalReasonSuspended, decision.Reason)
}

// The owner's suspension holds at every plane, as the organization's does.
func (s *ServiceTestSuite) TestAnOwnerSuspensionHoldsAtEveryPlane() {
	for _, plane := range []providers.AccessPlane{
		providers.AccessPlaneAuthentication, providers.AccessPlaneDispatch, providers.AccessPlaneRecovery,
		providers.AccessPlaneSession, providers.AccessPlaneIssuance, providers.AccessPlaneApplication,
	} {
		s.Run(string(plane), func() {
			s.SetupTest()
			s.expectRead(s.ownerSuspendedEntity())
			s.expectConfig(methodGranularityConfig())

			decision, err := s.svc.EvaluateAccess(s.ctx, "e1", plane, testMethod)
			s.Require().Nil(err)
			s.False(decision.Admitted)
			s.Equal(providers.AccessRefusalReasonSuspended, decision.Reason)
		})
	}
}

// A method lock refuses at the authentication plane as a lock.
func (s *ServiceTestSuite) TestCredentialHoldRefusesAtTheCredentialPlane() {
	entity := s.activeEntity()
	putLockEntry(&entity.AccessState, testMethod, LockEntry{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
	s.expectRead(entity)
	s.expectConfig(methodGranularityConfig())

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testMethod)
	s.Require().Nil(err)
	s.False(decision.Admitted)
	s.Equal(providers.AccessRefusalReasonLocked, decision.Reason)
}

// A locked password does not hold a passkey.
func (s *ServiceTestSuite) TestALockedMethodDoesNotHoldAnother() {
	entity := s.activeEntity()
	putLockEntry(&entity.AccessState, testMethod, LockEntry{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
	s.expectRead(entity)
	s.expectConfig(methodGranularityConfig())

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID,
		providers.AccessPlaneAuthentication, providers.AuthenticationMethodPasskey)
	s.Require().Nil(err)
	s.True(decision.Admitted, "a locked password must not hold the passkey")
}

func (s *ServiceTestSuite) TestALockExpiresWithoutAWrite() {
	entity := s.activeEntity()
	putLockEntry(&entity.AccessState, testMethod, LockEntry{LockCount: 1, UnlockAt: s.at(-time.Second)})
	s.expectRead(entity)
	s.expectConfig(methodGranularityConfig())

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testMethod)
	s.Require().Nil(err)
	s.True(decision.Admitted)
}

// A method lock does not reach the session, issuance or application planes, nor a recovery step
// that names no method.
func (s *ServiceTestSuite) TestACredentialHoldDoesNotReachOtherPlanesByDefault() {
	entity := s.activeEntity()
	putLockEntry(&entity.AccessState, testMethod, LockEntry{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
	s.expectRead(entity)
	s.expectConfig(methodGranularityConfig())

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneRecovery, "")
	s.Require().Nil(err)
	s.True(decision.Admitted, "a recovery step that names no method must be admitted under a lock")

	for _, plane := range []providers.AccessPlane{
		providers.AccessPlaneSession, providers.AccessPlaneIssuance, providers.AccessPlaneApplication,
	} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testMethod)
		s.Require().Nil(err)
		s.True(decision.Admitted, "plane %s must admit a locked but unsuspended identity", plane)
	}
}

// Under entity granularity one entry under the reserved key holds every method.
func (s *ServiceTestSuite) TestEntityGranularityHoldsEveryAuthenticationMethod() {
	cfg := methodGranularityConfig()
	cfg.User.LockGranularity = new(granularityEntity)

	entity := s.activeEntity()
	putLockEntry(&entity.AccessState, AuthenticationMethodEntity, LockEntry{
		LockCount: 1, UnlockAt: s.at(5 * time.Minute),
	})
	s.expectRead(entity)
	s.expectConfig(cfg)

	for _, method := range []providers.AuthenticationMethod{
		testMethod,
		providers.AuthenticationMethodPasskey,
		providers.AuthenticationMethodOTP,
		providers.AuthenticationMethodMagicLink,
	} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, method)
		s.Require().Nil(err)
		s.False(decision.Admitted, "method %s must be held by the entity-wide lock", method)
	}
}

func (s *ServiceTestSuite) TestEntityGranularityAloneDoesNotReachSessionsOrIssuance() {
	cfg := methodGranularityConfig()
	cfg.User.LockGranularity = new(granularityEntity)

	entity := s.activeEntity()
	putLockEntry(&entity.AccessState, AuthenticationMethodEntity, LockEntry{
		LockCount: 1, UnlockAt: s.at(5 * time.Minute),
	})
	s.expectRead(entity)
	s.expectConfig(cfg)

	for _, plane := range []providers.AccessPlane{providers.AccessPlaneSession, providers.AccessPlaneIssuance} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testMethod)
		s.Require().Nil(err)
		s.True(decision.Admitted, "plane %s must stay admitted with the flags unset", plane)
	}
}

// An entity-wide lock holds only the authentication and dispatch planes, under either granularity.
func (s *ServiceTestSuite) TestAnEntityWideLockHoldsOnlyTheCredentialAndDispatchPlanes() {
	cfg := methodGranularityConfig()
	cfg.User.LockGranularity = new(granularityEntity)

	entity := s.activeEntity()
	putLockEntry(&entity.AccessState, AuthenticationMethodEntity, LockEntry{
		LockCount: 1, UnlockAt: s.at(5 * time.Minute),
	})
	s.expectRead(entity)
	s.expectConfig(cfg)

	for _, plane := range []providers.AccessPlane{providers.AccessPlaneAuthentication, providers.AccessPlaneDispatch} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testMethod)
		s.Require().Nil(err)
		s.False(decision.Admitted, "plane %s must refuse under a live entity-wide lock", plane)
	}

	for _, plane := range []providers.AccessPlane{
		providers.AccessPlaneSession, providers.AccessPlaneIssuance, providers.AccessPlaneApplication,
	} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testMethod)
		s.Require().Nil(err)
		s.True(decision.Admitted, "plane %s must never be held by a lock", plane)
	}

	// It lapses by the clock with no write.
	s.svc.now = func() time.Time { return s.now.Add(10 * time.Minute) }
	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testMethod)
	s.Require().Nil(err)
	s.True(decision.Admitted, "a lock refuses; it does not revoke")
}

// At the recovery plane a lock gives way to a one-time code, a signature or a federated assertion,
// which is how a locked account gets out. It still holds a password, which can be guessed.
func (s *ServiceTestSuite) TestAtTheRecoveryPlaneALockHoldsOnlyAGuessableSecret() {
	entity := s.activeEntity()
	putLockEntry(&entity.AccessState, testMethod, LockEntry{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
	putLockEntry(&entity.AccessState, providers.AuthenticationMethodOTP,
		LockEntry{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
	s.expectRead(entity)
	s.expectConfig(methodGranularityConfig())

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneRecovery, testMethod)
	s.Require().Nil(err)
	s.False(decision.Admitted, "a locked password must stay locked in a recovery flow")
	s.Equal(providers.AccessRefusalReasonLocked, decision.Reason)

	decision, err = s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneRecovery,
		providers.AuthenticationMethodOTP)
	s.Require().Nil(err)
	s.True(decision.Admitted, "a locked one-time password must give way to a recovery code")
}

// The hold an unsuspend leaves holds every sign-in method, and recovery is the holder's way out, so a
// passkey or a code gets past it at the recovery plane. A password does not.
func (s *ServiceTestSuite) TestThePostSuspensionHoldGivesWayToARecoveryProof() {
	entity := s.activeEntity()
	entity.AccessState.Lock.Entity = &LockEntry{UnlockAt: permanentUnlockAt, Reason: lockReasonPostSuspension}
	s.expectRead(entity)
	s.expectConfig(methodGranularityConfig())

	for method, admitted := range map[providers.AuthenticationMethod]bool{
		providers.AuthenticationMethodPasskey:   true,
		providers.AuthenticationMethodOTP:       true,
		providers.AuthenticationMethodMagicLink: true,
		providers.AuthenticationMethodFederated: true,
		providers.AuthenticationMethodOpenID4VP: true,
		testMethod:                              false,
	} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneRecovery, method)
		s.Require().Nil(err)
		s.Equal(admitted, decision.Admitted, string(method))
	}
}

// A suspension still refuses every recovery proof.
func (s *ServiceTestSuite) TestASuspensionRefusesEveryRecoveryProof() {
	entity := s.activeEntity()
	entity.State = providers.EntityStateSuspended
	s.expectRead(entity)
	s.expectConfig(methodGranularityConfig())

	for _, method := range []providers.AuthenticationMethod{"", providers.AuthenticationMethodOTP,
		providers.AuthenticationMethodPasskey} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneRecovery, method)
		s.Require().Nil(err)
		s.False(decision.Admitted, string(method))
	}
}

// Every registered method says whether a lock holds it at the recovery plane, so a new method cannot
// be added without that decision.
func (s *ServiceTestSuite) TestEveryMethodIsClassifiedForRecovery() {
	for _, method := range AllAuthenticationMethods {
		_, classified := heldAtRecovery[method]
		s.True(classified, string(method))
	}
}

// A wrong recovery code is not counted: the lock gives way to it in recovery, so the count would only
// lock sign-in. A wrong password in recovery is counted, since the lock still holds it there.
func (s *ServiceTestSuite) TestAWrongRecoveryCodeIsNotCounted() {
	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectConfig(methodGranularityConfig())
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testMethod, mock.Anything, mock.Anything).
		Return(LockEntry{FailureCount: 1}, true, nil).Once()

	s.Require().Nil(s.admittedFailure(s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneRecovery,
		providers.AuthenticationMethodOTP)))
	s.Require().Nil(s.admittedFailure(s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneRecovery, testMethod)))
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, entity.ID, providers.AuthenticationMethodOTP, mock.Anything, mock.Anything)
}

// A failure on a method whose class is not counted is neither read nor recorded.
func (s *ServiceTestSuite) TestFailureAtANotLockableMethodIsNotRecorded() {
	decision, err := s.svc.RecordFailure(s.ctx, s.activeEntity().ID, providers.AccessPlaneAuthentication,
		providers.AuthenticationMethodPasskey)

	s.Require().Nil(err)
	s.True(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	s.port.AssertNotCalled(s.T(), "GetGovernedEntity", mock.Anything, mock.Anything, mock.Anything)
}

// A failure with no subject is not an error and reads nothing.
func (s *ServiceTestSuite) TestFailureWithNoSubjectRecordsNothing() {
	s.Require().Nil(s.admittedFailure(s.svc.RecordFailure(s.ctx, "", providers.AccessPlaneAuthentication, testMethod)))
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// The policy is resolved from the category of the entity the service reads: the agent's ten-minute
// window applies, not the user's fifteen.
func (s *ServiceTestSuite) TestRecordingResolvesThePolicyFromItsOwnRead() {
	stored := s.activeEntity()
	stored.Category = providers.EntityCategoryAgent
	stored.Type = "service"
	s.expectRead(stored)
	cfg := methodGranularityConfig()
	cfg.Agent = &accountAccessCategory{Default: &accountAccessPolicy{
		Threshold: new(5), FailureWindowSeconds: new(600), LockDurationsSeconds: []int{300},
	}}
	s.expectConfig(cfg)
	s.port.On("IncrementFailure", mock.Anything, stored.ID, testMethod, s.now, s.now.Add(-10*time.Minute)).
		Return(LockEntry{FailureCount: 1}, true, nil).Once()

	s.Require().Nil(s.admittedFailure(s.svc.RecordFailure(s.ctx, stored.ID, providers.AccessPlaneAuthentication,
		testMethod)))
}

// The counter resets once the last failure is older than the window.
func (s *ServiceTestSuite) TestTheWindowStartIsTheWindowBeforeNow() {
	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectConfig(methodGranularityConfig())
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testMethod, s.now, s.now.Add(-15*time.Minute)).
		Return(LockEntry{FailureCount: 1}, true, nil).Once()

	s.Require().Nil(s.admittedFailure(s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
		testMethod)))
}

// A zero window passes the zero time, so no failure is ever outside the window.
func (s *ServiceTestSuite) TestAZeroWindowNeverResetsTheCounterOnTime() {
	cfg := methodGranularityConfig()
	cfg.User.Default.FailureWindowSeconds = new(0)

	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectConfig(cfg)
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testMethod, s.now, time.Time{}).
		Return(LockEntry{FailureCount: 5}, true, nil).Once()
	s.port.On("FormLock", mock.Anything, entity.ID, testMethod, mock.Anything, mock.Anything, s.now).
		Return(true, nil).Once()

	s.Require().Nil(s.admittedFailure(s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
		testMethod)))
	s.port.AssertExpectations(s.T())
}

// A read failure is returned, not swallowed.
func (s *ServiceTestSuite) TestRecordingReportsAnUnreadableAuthority() {
	s.port.On("GetGovernedEntity", mock.Anything, "e1", true).
		Return(nil, errors.New("entity store unavailable")).Once()

	s.Require().NotNil(s.admittedFailure(s.svc.RecordFailure(s.ctx, "e1", providers.AccessPlaneAuthentication,
		testMethod)))
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A machine credential failure is not read, counted or locked, so a hold, a suspension included, cannot
// change its answer: no lock limits guessing it, and only a correct secret states the hold.
func (s *ServiceTestSuite) TestMachineCredentialMethodNeitherCountsNorLocks() {
	entity := s.activeEntity()
	method := providers.AuthenticationMethodSystemCredential

	s.Require().Nil(s.admittedFailure(s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
		method)))

	s.port.AssertNotCalled(s.T(), "GetGovernedEntity", mock.Anything, mock.Anything, mock.Anything)
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	s.port.AssertNotCalled(s.T(), "FormLock",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestFailureBelowTheThresholdFormsNoLock() {
	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectConfig(methodGranularityConfig())
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testMethod, mock.Anything, mock.Anything).
		Return(LockEntry{FailureCount: 4}, true, nil).Once()

	s.Require().Nil(s.admittedFailure(s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
		testMethod)))
	s.port.AssertNotCalled(s.T(), "FormLock",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// At the threshold a lock forms with the escalation's duration, guarded on the observed counters.
func (s *ServiceTestSuite) TestFailureAtTheThresholdFormsALock() {
	entity := s.activeEntity()
	observed := LockEntry{FailureCount: 5}
	s.expectRead(entity)
	s.expectConfig(methodGranularityConfig())
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testMethod, mock.Anything, mock.Anything).
		Return(observed, true, nil).Once()

	var written LockEpisode
	var guard LockEntry
	s.port.On("FormLock", mock.Anything, entity.ID, testMethod,
		mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			guard, _ = args.Get(3).(LockEntry)
			written, _ = args.Get(4).(LockEpisode)
		}).Return(true, nil).Once()

	s.Require().Nil(s.admittedFailure(s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
		testMethod)))
	s.Equal(1, written.LockCount)
	s.Equal(s.now.Add(5*time.Minute).UTC().Format(time.RFC3339), written.UnlockAt)
	s.Equal(observed, guard, "the formation must be guarded on what the increment observed")
}

// Reaching the consecutive ceiling forms a permanent lock even below the threshold.
func (s *ServiceTestSuite) TestFailureAtTheConsecutiveCeilingFormsAPermanentLock() {
	cfg := methodGranularityConfig()
	cfg.User.Default.MaxConsecutiveFailures = new(12)
	entity := s.activeEntity()
	observed := LockEntry{FailureCount: 2, LockCount: 2, UnlockAt: s.at(-time.Minute), ConsecutiveFailures: 12}
	s.expectRead(entity)
	s.expectConfig(cfg)
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testMethod, mock.Anything, mock.Anything).
		Return(observed, true, nil).Once()

	var written LockEpisode
	s.port.On("FormLock", mock.Anything, entity.ID, testMethod, observed, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { written, _ = args.Get(4).(LockEpisode) }).Return(true, nil).Once()

	s.Require().Nil(s.admittedFailure(s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
		testMethod)))
	s.Equal(permanentUnlockAt, written.UnlockAt, "the ceiling's lock must not lapse")
	s.Equal(3, written.LockCount)
}

func (s *ServiceTestSuite) TestFailureDuringALiveLockFormsNothing() {
	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectConfig(methodGranularityConfig())
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testMethod, mock.Anything, mock.Anything).
		Return(LockEntry{FailureCount: 9, LockCount: 1, UnlockAt: s.at(5 * time.Minute)}, true, nil).
		Once()

	s.Require().Nil(s.admittedFailure(s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
		testMethod)))
	s.port.AssertNotCalled(s.T(), "FormLock",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestFormationStopsWhenAReReadShowsANewerLock() {
	entity := s.activeEntity()
	s.expectConfig(methodGranularityConfig())
	// The first read, then the re-read after the guard loses.
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Once()
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testMethod, mock.Anything, mock.Anything).
		Return(LockEntry{FailureCount: 5}, true, nil).Once()
	s.port.On("FormLock", mock.Anything, entity.ID, testMethod,
		mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Once()

	refreshed := s.activeEntity()
	putLockEntry(&refreshed.AccessState, testMethod, LockEntry{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&refreshed, nil).Once()

	s.Require().Nil(s.admittedFailure(s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
		testMethod)))
}

func (s *ServiceTestSuite) TestFormationRetryIsGuardedOnTheReReadRevision() {
	entity := s.activeEntity()
	s.expectConfig(methodGranularityConfig())
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Once()
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testMethod, mock.Anything, mock.Anything).
		Return(LockEntry{FailureCount: 5, Revision: 3}, true, nil).Once()
	s.port.On("FormLock", mock.Anything, entity.ID, testMethod,
		LockEntry{FailureCount: 5, Revision: 3}, mock.Anything, mock.Anything).Return(false, nil).Once()

	refreshed := s.activeEntity()
	refreshed.Revision = 9
	putLockEntry(&refreshed.AccessState, testMethod, LockEntry{FailureCount: 6})
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&refreshed, nil).Once()
	s.port.On("FormLock", mock.Anything, entity.ID, testMethod,
		LockEntry{FailureCount: 6, Revision: 9}, mock.Anything, mock.Anything).Return(true, nil).Once()

	s.Require().Nil(s.admittedFailure(s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
		testMethod)))
}

func (s *ServiceTestSuite) TestFormationStopsWhenAReReadShowsAReset() {
	entity := s.activeEntity()
	s.expectConfig(methodGranularityConfig())
	// Both reads see a row with no entries.
	s.expectRead(entity)
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testMethod, mock.Anything, mock.Anything).
		Return(LockEntry{FailureCount: 5}, true, nil).Once()
	s.port.On("FormLock", mock.Anything, entity.ID, testMethod,
		mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Once()

	s.Require().Nil(s.admittedFailure(s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
		testMethod)))
}

func (s *ServiceTestSuite) TestFormationRetriesAreBounded() {
	entity := s.activeEntity()
	s.expectConfig(methodGranularityConfig())
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testMethod, mock.Anything, mock.Anything).
		Return(LockEntry{FailureCount: 5}, true, nil).Once()

	// The guard never holds and the row keeps looking eligible.
	s.port.On("FormLock", mock.Anything, entity.ID, testMethod,
		mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Times(formationRetries)
	eligible := s.activeEntity()
	putLockEntry(&eligible.AccessState, testMethod, LockEntry{FailureCount: 5})
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&eligible, nil)

	s.Require().Nil(s.admittedFailure(s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
		testMethod)))
}

func (s *ServiceTestSuite) TestFailureOnAMissingRowIsNotAnError() {
	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectConfig(methodGranularityConfig())
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testMethod, mock.Anything, mock.Anything).
		Return(LockEntry{}, false, nil).Once()

	s.Require().Nil(s.admittedFailure(s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
		testMethod)))
	s.port.AssertNotCalled(s.T(), "FormLock",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestFailureUnderEntityGranularityCountsUnderTheReservedKey() {
	cfg := methodGranularityConfig()
	cfg.User.LockGranularity = new(granularityEntity)

	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectConfig(cfg)
	s.port.On("IncrementFailure", mock.Anything, entity.ID,
		AuthenticationMethodEntity, mock.Anything, mock.Anything).
		Return(LockEntry{FailureCount: 1}, true, nil).Once()

	s.Require().Nil(s.admittedFailure(s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
		testMethod)))
}

func (s *ServiceTestSuite) TestFailureWindowIsPassedAsAnInstant() {
	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectConfig(methodGranularityConfig())

	var windowStart time.Time
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testMethod, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { windowStart, _ = args.Get(4).(time.Time) }).
		Return(LockEntry{FailureCount: 1}, true, nil).Once()

	s.Require().Nil(s.admittedFailure(s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
		testMethod)))
	s.Equal(s.now.Add(-15*time.Minute), windowStart)
}

// An admitted step clears only its own method's entry, not the entity entry.
func (s *ServiceTestSuite) TestAdmittedStepClearsOnlyObservedMethodEntry() {
	entity := s.activeEntity()
	entity.Revision = 7
	putLockEntry(&entity.AccessState, testMethod, LockEntry{FailureCount: 2})
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Once()
	s.expectConfig(methodGranularityConfig())
	s.port.On("ClearAccessStateIfUnchanged", mock.Anything, entity.ID, testMethod, int64(7)).Return(true, nil).Once()
	decision, err := s.svc.AdmitAuthenticationStep(s.ctx, entity.ID, testMethod)
	s.Require().Nil(err)
	s.True(decision.Admitted)
}

// An admitted step leaves the reserved entry for AdmitSignIn.
func (s *ServiceTestSuite) TestAdmittedStepNeverClearsTheReservedKey() {
	s.expectRead(s.activeEntity())
	decision, err := s.svc.AdmitAuthenticationStep(s.ctx, "e1", AuthenticationMethodEntity)
	s.Require().Nil(err)
	s.True(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "ClearAccessStateIfUnchanged",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestDisabledPolicyAdmitsAndWritesNothing() {
	cfg := methodGranularityConfig()
	cfg.User.Default.Enabled = new(false)

	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectConfig(cfg)

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testMethod)
	s.Require().Nil(err, "a switched-off policy is not an error")
	s.True(decision.Admitted)

	s.Require().Nil(s.admittedFailure(s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
		testMethod)))
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestDisabledPolicyReleasesAnExistingLock() {
	cfg := methodGranularityConfig()
	cfg.User.Default.Enabled = new(false)

	entity := s.activeEntity()
	putLockEntry(&entity.AccessState, testMethod, LockEntry{LockCount: 3, UnlockAt: s.at(24 * time.Hour)})
	s.expectRead(entity)
	s.expectConfig(cfg)

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testMethod)
	s.Require().Nil(err)
	s.True(decision.Admitted, "a lock must not outlive the policy that formed it being switched off")
}

func (s *ServiceTestSuite) TestDisabledPolicyStillEnforcesAnIdentityHold() {
	cfg := methodGranularityConfig()
	cfg.User.Default.Enabled = new(false)

	entity := s.activeEntity()
	entity.State = providers.EntityStateSuspended
	s.expectRead(entity)
	s.expectConfig(cfg)

	for _, plane := range []providers.AccessPlane{
		providers.AccessPlaneAuthentication, providers.AccessPlaneSession, providers.AccessPlaneIssuance,
	} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testMethod)
		s.Require().Nil(err)
		s.False(decision.Admitted, "plane %s must still refuse a suspended identity", plane)
		s.Equal(providers.AccessRefusalReasonSuspended, decision.Reason)
	}
}

func (s *ServiceTestSuite) TestResetOnACleanAccountWritesNothing() {
	s.expectRead(s.activeEntity())

	s.expectConfig(methodGranularityConfig())
	decision, err := s.svc.AdmitAuthenticationStep(s.ctx, "e1", testMethod)
	s.Require().Nil(err)
	s.True(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "ClearAccessStateIfUnchanged",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestResetIgnoresStateUnderAnUnrelatedMethod() {
	other := s.activeEntity()
	putLockEntry(&other.AccessState, providers.AuthenticationMethodOTP, LockEntry{FailureCount: 3})
	s.expectRead(other)

	s.expectConfig(methodGranularityConfig())
	decision, err := s.svc.AdmitAuthenticationStep(s.ctx, "e1", testMethod)
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

// The note limit counts characters, not bytes, and a longer note is refused before anything is written.
func (s *ServiceTestSuite) TestSuspendTakesANoteUpToTheLimit() {
	longest := strings.Repeat("é", maxOperatorNoteLength)
	s.expectRead(s.activeEntity())
	s.port.On("SetSuspension", mock.Anything, "e1", s.now, longest).Return(true, nil).Once()
	s.Require().Nil(s.svc.Suspend(s.ctx, "e1", longest))
	s.Require().Nil(s.svc.ValidateSuspend(s.ctx, "e1", longest))

	s.requireServiceError(s.svc.Suspend(s.ctx, "e1", longest+"é"), ErrorOperatorNoteTooLong)
	s.requireServiceError(s.svc.ValidateSuspend(s.ctx, "e1", longest+"é"), ErrorOperatorNoteTooLong)
	s.port.AssertNumberOfCalls(s.T(), "SetSuspension", 1)
}

// An unknown identity is a client error, not a failed read.
func (s *ServiceTestSuite) TestSuspendOfAnUnknownIdentityIsAClientError() {
	s.port.On("GetGovernedEntity", mock.Anything, "missing", true).
		Return(nil, nil).Once()

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

// ownerSuspendedEntity is an account its owner holds suspended, with no suspension by the organization.
func (s *ServiceTestSuite) ownerSuspendedEntity() GovernedEntity {
	entity := s.activeEntity()
	entity.AccessState.OwnerSuspend = &OwnerSuspendState{SuspendedAt: s.at(-2 * time.Hour)}
	return entity
}

// The owner's suspension is placed in its own member.
func (s *ServiceTestSuite) TestAnOwnerSuspensionIsPlacedApart() {
	s.expectRead(s.activeEntity())
	s.port.On("SetOwnerSuspension", mock.Anything, "e1", s.now).Return(true, nil).Once()

	s.Require().Nil(s.svc.Suspend(WithOwnerApplier(s.ctx), "e1", ""))
	s.port.AssertNotCalled(s.T(), "SetSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// The owner's suspension is placed even while the organization's is in force, and an existing one is left
// as it is.
func (s *ServiceTestSuite) TestAnOwnerSuspensionIgnoresTheOrganizationsHold() {
	s.expectRead(s.suspendedEntity())
	s.port.On("SetOwnerSuspension", mock.Anything, "e1", s.now).Return(true, nil).Once()
	s.Require().Nil(s.svc.Suspend(WithOwnerApplier(s.ctx), "e1", ""))

	s.SetupTest()
	s.expectRead(s.ownerSuspendedEntity())
	s.Require().Nil(s.svc.Suspend(WithOwnerApplier(s.ctx), "e1", ""))
	s.port.AssertNotCalled(s.T(), "SetOwnerSuspension", mock.Anything, mock.Anything, mock.Anything)
}

// The organization's suspension is placed even while the owner's is in force.
func (s *ServiceTestSuite) TestTheOrganizationsSuspensionIgnoresTheOwnersHold() {
	s.expectRead(s.ownerSuspendedEntity())
	s.port.On("SetSuspension", mock.Anything, "e1", s.now, "Security advisory").Return(true, nil).Once()

	s.Require().Nil(s.svc.Suspend(s.ctx, "e1", "Security advisory"))
}

// Each party lifts only its own suspension, and the other party's stays.
func (s *ServiceTestSuite) TestEachPartyLiftsOnlyItsOwnSuspension() {
	both := s.suspendedEntity()
	both.Category = providers.EntityCategoryAgent
	both.AccessState.OwnerSuspend = &OwnerSuspendState{SuspendedAt: s.at(-2 * time.Hour)}

	s.expectRead(both)
	s.port.On("ClearOwnerSuspension", mock.Anything, "e1").Return(true, nil).Once()
	s.Require().Nil(s.svc.Unsuspend(WithOwnerApplier(s.ctx), "e1"))
	s.port.AssertNotCalled(s.T(), "ClearSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	s.SetupTest()
	s.expectRead(both)
	s.port.On("ClearSuspension", mock.Anything, "e1", (*EntityHold)(nil), s.at(-time.Hour)).Return(true, nil).Once()
	s.Require().Nil(s.svc.Unsuspend(s.ctx, "e1"))
	s.port.AssertNotCalled(s.T(), "ClearOwnerSuspension", mock.Anything, mock.Anything)
}

// A party with no suspension of its own has nothing to lift, whatever the other party holds.
func (s *ServiceTestSuite) TestLiftingAnotherPartysSuspensionDoesNotApply() {
	s.expectRead(s.suspendedEntity())
	s.requireServiceError(s.svc.Unsuspend(WithOwnerApplier(s.ctx), "e1"), ErrorNotApplicable)

	s.SetupTest()
	s.expectRead(s.ownerSuspendedEntity())
	s.requireServiceError(s.svc.Unsuspend(s.ctx, "e1"), ErrorNotApplicable)

	s.port.AssertNotCalled(s.T(), "ClearOwnerSuspension", mock.Anything, mock.Anything)
	s.port.AssertNotCalled(s.T(), "ClearSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A lift that finds the owner's suspension already gone does not apply.
func (s *ServiceTestSuite) TestAnOwnerLiftOfAGoneSuspensionDoesNotApply() {
	s.expectRead(s.ownerSuspendedEntity())
	s.port.On("ClearOwnerSuspension", mock.Anything, "e1").Return(false, nil).Once()

	s.requireServiceError(s.svc.Unsuspend(WithOwnerApplier(s.ctx), "e1"), ErrorNotApplicable)
}

func (s *ServiceTestSuite) TestEitherSuspensionStoreAloneHolds() {
	for name, mutate := range map[providers.AuthenticationMethod]func(*GovernedEntity){
		"document only": func(e *GovernedEntity) { e.State = providers.EntityStateActive },
		"column only":   func(e *GovernedEntity) { e.AccessState.Suspend = nil },
	} {
		s.SetupTest()

		entity := s.suspendedEntity()
		mutate(&entity)
		s.expectRead(entity)
		s.expectConfig(methodGranularityConfig())

		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testMethod)
		s.Require().Nil(err)
		s.Falsef(decision.Admitted, "%s must still hold the identity", name)
		s.Equal(providers.AccessRefusalReasonSuspended, decision.Reason)
	}
}

// A suspension refuses at every plane, including session, issuance and recovery.
func (s *ServiceTestSuite) TestASuspensionRefusesAtEveryPlane() {
	entity := s.suspendedEntity()
	s.expectRead(entity)
	s.expectConfig(methodGranularityConfig())

	for _, plane := range []providers.AccessPlane{
		providers.AccessPlaneAuthentication, providers.AccessPlaneDispatch, providers.AccessPlaneRecovery,
		providers.AccessPlaneSession, providers.AccessPlaneIssuance, providers.AccessPlaneApplication,
	} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, plane, testMethod)
		s.Require().Nil(err)
		s.False(decision.Admitted, "plane %s must refuse a suspended identity", plane)
		s.Equal(providers.AccessRefusalReasonSuspended, decision.Reason)
	}
}

func (s *ServiceTestSuite) TestASuspensionAppliesWithLockingSwitchedOff() {
	cfg := methodGranularityConfig()
	cfg.User.Default.Enabled = new(false)

	entity := s.suspendedEntity()
	s.expectRead(entity)
	s.expectConfig(cfg)

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testMethod)
	s.Require().Nil(err)
	s.False(decision.Admitted)
	s.Equal(providers.AccessRefusalReasonSuspended, decision.Reason)
}

// IsSuspended reads without seeding, either store alone answers true, and a lock is not a suspension.
// The strict port mock expects seedMissing false, so a seeding read fails the test.
func (s *ServiceTestSuite) TestIsSuspendedReadsWithoutWriting() {
	locked := s.activeEntity()
	locked.AccessState.Lock.Entity = &LockEntry{UnlockAt: permanentUnlockAt, Reason: lockReasonPostSuspension}
	documentOnly := s.suspendedEntity()
	documentOnly.State = providers.EntityStateActive
	columnOnly := s.suspendedEntity()
	columnOnly.AccessState.Suspend = nil

	for name, tc := range map[string]struct {
		entity GovernedEntity
		want   bool
	}{
		"active":        {entity: s.activeEntity(), want: false},
		"locked":        {entity: locked, want: false},
		"document only": {entity: documentOnly, want: true},
		"column only":   {entity: columnOnly, want: true},
	} {
		s.SetupTest()
		s.port.On("GetGovernedEntity", mock.Anything, tc.entity.ID, false).Return(&tc.entity, nil).Once()

		suspended, err := s.svc.IsSuspended(s.ctx, tc.entity.ID)
		s.Require().Nil(err, name)
		s.Equal(tc.want, suspended, name)
	}
}

func (s *ServiceTestSuite) TestIsSuspendedAdmitsAnUngovernedSubject() {
	suspended, err := s.svc.IsSuspended(s.ctx, "")
	s.Require().Nil(err)
	s.False(suspended, "an empty subject must not be suspended")

	s.port.On("GetGovernedEntity", mock.Anything, "e1", false).Return(nil, nil).Once()
	suspended, err = s.svc.IsSuspended(s.ctx, "e1")
	s.Require().Nil(err)
	s.False(suspended, "an identity with no local record must not be suspended")
}

func (s *ServiceTestSuite) TestIsSuspendedReportsAnUnreadableAuthority() {
	s.port.On("GetGovernedEntity", mock.Anything, "e1", false).
		Return(nil, errors.New("database unavailable")).Once()

	suspended, err := s.svc.IsSuspended(s.ctx, "e1")
	s.requireServiceError(err, tidcommon.InternalServerError)
	s.False(suspended)
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
	s.Equal(permanentUnlockAt, written.UnlockAt)
	s.Equal(lockReasonPostSuspension, written.Reason)
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
	putLockEntry(&entity.AccessState, AuthenticationMethodEntity,
		LockEntry{UnlockAt: permanentUnlockAt, Reason: lockReasonPostSuspension})
	s.expectRead(entity)
	s.expectConfig(methodGranularityConfig())

	for _, method := range []providers.AuthenticationMethod{
		testMethod,
		providers.AuthenticationMethodPasskey,
		providers.AuthenticationMethodOTP,
		providers.AuthenticationMethodMagicLink,
		providers.AuthenticationMethodFederated,
	} {
		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, method)
		s.Require().Nil(err)
		s.False(decision.Admitted, "method %s must be held by the post-suspension hold", method)
		s.Equal(providers.AccessRefusalReasonLocked, decision.Reason)
	}
}

func (s *ServiceTestSuite) TestThePostSuspensionHoldSurvivesLockingBeingSwitchedOff() {
	cfg := methodGranularityConfig()
	cfg.User.Default.Enabled = new(false)

	entity := s.activeEntity()
	putLockEntry(&entity.AccessState, AuthenticationMethodEntity,
		LockEntry{UnlockAt: permanentUnlockAt, Reason: lockReasonPostSuspension})
	s.expectRead(entity)
	s.expectConfig(cfg)

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testMethod)
	s.Require().Nil(err)
	s.False(decision.Admitted)
	s.Equal(providers.AccessRefusalReasonLocked, decision.Reason)
}

// Only the post-suspension hold outlives disabled locking; an entity-wide failure lock does not.
func (s *ServiceTestSuite) TestAnEntityWideFailureLockDoesNotSurviveLockingBeingSwitchedOff() {
	cfg := methodGranularityConfig()
	cfg.User.Default.Enabled = new(false)

	entity := s.activeEntity()
	putLockEntry(&entity.AccessState, AuthenticationMethodEntity, LockEntry{LockCount: 3, UnlockAt: permanentUnlockAt})
	s.expectRead(entity)
	s.expectConfig(cfg)

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testMethod)
	s.Require().Nil(err)
	s.True(decision.Admitted)
}

// The post-suspension hold never holds a recovery step that names no method, such as identifying the
// account or writing its new credential.
func (s *ServiceTestSuite) TestThePostSuspensionHoldNeverHoldsRecovery() {
	entity := s.activeEntity()
	putLockEntry(&entity.AccessState, AuthenticationMethodEntity,
		LockEntry{UnlockAt: permanentUnlockAt, Reason: lockReasonPostSuspension})
	s.expectRead(entity)
	s.expectConfig(methodGranularityConfig())

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneRecovery, "")
	s.Require().Nil(err)
	s.True(decision.Admitted)
}

func (s *ServiceTestSuite) TestNoFailureIsCountedUnderALiveEntityHold() {
	entity := s.activeEntity()
	putLockEntry(&entity.AccessState, AuthenticationMethodEntity,
		LockEntry{UnlockAt: permanentUnlockAt, Reason: lockReasonPostSuspension})
	s.expectRead(entity)
	s.expectConfig(methodGranularityConfig())

	decision, err := s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testMethod)

	s.Require().Nil(err)
	s.Equal(providers.AccessDecision{Reason: providers.AccessRefusalReasonLocked}, decision)
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// Unlock works on a suspended identity and does not release the suspension.
func (s *ServiceTestSuite) TestUnlockIsNotRefusedOnASuspendedIdentity() {
	s.port.On("GetGovernedEntity", mock.Anything, "e1", true).Return(new(s.suspendedEntity()), nil).Maybe()
	s.port.On("ClearAccessState", mock.Anything, "e1", mock.Anything).Return(nil).Once()

	s.Require().Nil(s.svc.Unlock(s.ctx, "e1"))
	s.port.AssertNotCalled(s.T(), "ClearSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestUnsuspendRefusesWhenTheReadFails() {
	s.port.On("GetGovernedEntity", mock.Anything, "e1", true).
		Return(nil, errors.New("database unavailable")).Maybe()

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

		cfg := methodGranularityConfig()
		cfg.User.LockGranularity = new(granularity)

		entity := s.activeEntity()
		// A lapsed method entry under a live entity entry, as a granularity change can leave.
		putLockEntry(&entity.AccessState, testMethod,
			LockEntry{LockCount: 2, UnlockAt: s.at(-time.Hour)})
		putLockEntry(&entity.AccessState, AuthenticationMethodEntity,
			LockEntry{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
		s.expectRead(entity)
		s.expectConfig(cfg)

		decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testMethod)
		s.Require().Nil(err)
		s.Falsef(decision.Admitted, "granularity %s must be held by the entity entry", granularity)
		s.Equal(providers.AccessStatusLocked,
			reportFor(entity.State, entity.AccessState, s.now, lockEnforcement{}).Value)
	}
}

func (s *ServiceTestSuite) TestALapsedEntityEntryFallsThroughToTheAuthenticationMethod() {
	entity := s.activeEntity()
	putLockEntry(&entity.AccessState, AuthenticationMethodEntity,
		LockEntry{LockCount: 1, UnlockAt: s.at(-time.Hour)})
	putLockEntry(&entity.AccessState, testMethod,
		LockEntry{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
	s.expectRead(entity)
	s.expectConfig(methodGranularityConfig())

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testMethod)
	s.Require().Nil(err)
	s.False(decision.Admitted)
}

// putLockEntry records one lock entry under the member the method maps to.
func putLockEntry(state *AccessState, method providers.AuthenticationMethod, entry LockEntry) {
	if method == AuthenticationMethodEntity {
		held := entry
		state.Lock.Entity = &held
		return
	}
	if state.Lock.AuthenticationMethods == nil {
		state.Lock.AuthenticationMethods = make(map[providers.AuthenticationMethod]LockEntry)
	}
	state.Lock.AuthenticationMethods[method] = entry
}

// lockedState builds a document holding one entry.
func lockedState(method providers.AuthenticationMethod, entry LockEntry) AccessState {
	var state AccessState
	putLockEntry(&state, method, entry)
	return state
}

// serviceWith builds the service over a source that serves cfg.
func (s *ServiceTestSuite) serviceWith(cfg accountAccessValue) *identityGovernanceService {
	svc := newTestService(s.T(), s.port, cfg, nil, nil)
	svc.now = func() time.Time { return s.now }
	return svc
}

// entityGranularityConfig is a user category in entity mode.
func entityGranularityConfig() accountAccessValue {
	return accountAccessValue{
		User: &accountAccessCategory{
			LockGranularity: new(granularityEntity),
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
		AccessState: lockedState(AuthenticationMethodEntity,
			LockEntry{LockCount: 1, UnlockAt: s.at(5 * time.Minute)}),
	}
}

func (s *ServiceTestSuite) TestEntityWideLockHoldsAuthenticationMethodsWithNoBlockOfTheirOwn() {
	svc := s.serviceWith(entityGranularityConfig())
	entity := s.lockedEntityWide()
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Maybe()

	for _, method := range []providers.AuthenticationMethod{
		providers.AuthenticationMethodCredential,
		providers.AuthenticationMethodOTP,
		providers.AuthenticationMethodPasskey,
		providers.AuthenticationMethodMagicLink,
		providers.AuthenticationMethodFederated,
		providers.AuthenticationMethodOpenID4VP,
	} {
		decision, err := svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, method)
		s.Require().Nil(err)
		s.False(decision.Admitted, "method %s must be held by the entity-wide lock", method)
	}
}

// Under method granularity, a federated sign-in obeys an entity-wide lock and ignores a password lock.
func (s *ServiceTestSuite) TestAFederatedSignInObeysOnlyAnEntityWideLock() {
	svc := s.serviceWith(testAccountAccessConfig())

	entityWide := s.lockedEntityWide()
	s.port.On("GetGovernedEntity", mock.Anything, entityWide.ID, true).Return(&entityWide, nil).Once()
	decision, err := svc.EvaluateAccess(s.ctx, entityWide.ID, providers.AccessPlaneAuthentication,
		providers.AuthenticationMethodFederated)
	s.Require().Nil(err)
	s.False(decision.Admitted, "an entity-wide lock holds a federated sign-in")

	passwordOnly := s.lockedEntityWide()
	passwordOnly.ID = "e2"
	passwordOnly.AccessState = lockedState(providers.AuthenticationMethodCredential,
		LockEntry{LockCount: 1, UnlockAt: s.at(time.Minute)})
	s.port.On("GetGovernedEntity", mock.Anything, passwordOnly.ID, true).Return(&passwordOnly, nil).Once()
	decision, err = svc.EvaluateAccess(s.ctx, passwordOnly.ID, providers.AccessPlaneAuthentication,
		providers.AuthenticationMethodFederated)
	s.Require().Nil(err)
	s.True(decision.Admitted, "a password lock does not hold a federated sign-in")
}

// The session and issuance planes present no method: the policy still resolves and the lock does not apply.
func (s *ServiceTestSuite) TestALockIsNotReachedWithNoAuthenticationMethodPresent() {
	svc := s.serviceWith(entityGranularityConfig())
	entity := s.lockedEntityWide()
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Maybe()

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

	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Maybe()

	// Both methods count under the reserved key, with the entity block's 15-minute window.
	for _, method := range []providers.AuthenticationMethod{
		providers.AuthenticationMethodCredential,
		providers.AuthenticationMethodOTP,
	} {
		s.port.On("IncrementFailure", mock.Anything, entity.ID,
			AuthenticationMethodEntity, s.now, s.now.Add(-15*time.Minute)).
			Return(LockEntry{FailureCount: 4}, true, nil).Once()

		s.Require().Nil(s.admittedFailure(svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
			method)))
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

	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Maybe()
	s.port.On("IncrementFailure", mock.Anything, entity.ID,
		AuthenticationMethodEntity, mock.Anything, mock.Anything).
		Return(LockEntry{FailureCount: 5}, true, nil).Once()

	var episode LockEpisode
	s.port.On("FormLock", mock.Anything, entity.ID, AuthenticationMethodEntity,
		mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { episode, _ = args.Get(4).(LockEpisode) }).
		Return(true, nil).Once()

	s.Require().Nil(s.admittedFailure(svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
		providers.AuthenticationMethodCredential)))
	s.Equal(s.now.Add(5*time.Minute).UTC().Format(time.RFC3339), episode.UnlockAt,
		"the entity block's first duration is 5 minutes; the credential block's is 1")
}

func (s *ServiceTestSuite) TestEntityGranularityStillDoesNotCountUncountableAuthenticationMethods() {
	svc := s.serviceWith(entityGranularityConfig())

	// No entity is set up: the class is checked before the read.
	s.Require().Nil(s.admittedFailure(svc.RecordFailure(s.ctx, "e1", providers.AccessPlaneAuthentication,
		providers.AuthenticationMethodPasskey)))
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
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Maybe()
	s.port.On("IncrementFailure", mock.Anything, entity.ID,
		providers.AuthenticationMethodOTP, s.now, s.now.Add(-15*time.Minute)).
		Return(LockEntry{FailureCount: 5}, true, nil).Once()
	s.port.On("FormLock", mock.Anything, entity.ID, providers.AuthenticationMethodOTP,
		mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Once()

	s.Require().Nil(s.admittedFailure(svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
		providers.AuthenticationMethodOTP)))
}

func (s *ServiceTestSuite) TestAuthenticationMethodGranularityKeepsMethodsIsolated() {
	svc := s.serviceWith(testAccountAccessConfig())
	entity := GovernedEntity{
		ID: "e1", Category: providers.EntityCategoryUser, State: providers.EntityStateActive,
		AccessState: lockedState(providers.AuthenticationMethodCredential,
			LockEntry{LockCount: 1, UnlockAt: s.at(5 * time.Minute)}),
	}
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Maybe()

	held, err := svc.EvaluateAccess(s.ctx, entity.ID,
		providers.AccessPlaneAuthentication, providers.AuthenticationMethodCredential)
	s.Require().Nil(err)
	s.False(held.Admitted)

	for _, method := range []providers.AuthenticationMethod{
		providers.AuthenticationMethodOTP,
		providers.AuthenticationMethodPasskey,
	} {
		decision, err := svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, method)
		s.Require().Nil(err)
		s.True(decision.Admitted, "method %s must survive a credential lock", method)
	}

	// The session and issuance planes stay open in either mode.
	for _, plane := range []providers.AccessPlane{providers.AccessPlaneSession, providers.AccessPlaneIssuance} {
		decision, err := svc.EvaluateAccess(s.ctx, entity.ID,
			plane, providers.AuthenticationMethodCredential)
		s.Require().Nil(err)
		s.True(decision.Admitted, "plane %s must be unaffected under authentication_method granularity", plane)
	}
}

// An explicitly disabled agent policy is no error and writes nothing.
func (s *ServiceTestSuite) TestAgentCategoryCountsNothing() {
	svc := s.serviceWith(testAccountAccessConfig())
	entity := GovernedEntity{
		ID: "a1", Category: providers.EntityCategoryAgent, State: providers.EntityStateActive,
	}
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Maybe()

	decision, err := svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
		providers.AuthenticationMethodCredential)
	s.Require().Nil(err)
	s.True(decision.Admitted, "an agent with locking off is admitted, not errored")

	s.Require().Nil(s.admittedFailure(svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
		providers.AuthenticationMethodCredential)))
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A lock formed under method granularity is not read after switching to entity granularity.
func (s *ServiceTestSuite) TestAAuthenticationMethodLockGoesInertWhenGranularityFlips() {
	entity := GovernedEntity{
		ID: "e1", Category: providers.EntityCategoryUser, State: providers.EntityStateActive,
		AccessState: lockedState(providers.AuthenticationMethodCredential,
			LockEntry{LockCount: 1, UnlockAt: s.at(5 * time.Minute)}),
	}
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Maybe()

	// Under the mode it was formed in, it holds.
	held, err := s.serviceWith(testAccountAccessConfig()).EvaluateAccess(s.ctx, entity.ID,
		providers.AccessPlaneAuthentication, providers.AuthenticationMethodCredential)
	s.Require().Nil(err)
	s.False(held.Admitted)

	// Flipped to entity granularity, the same stored entry is not read at all.
	admitted, err := s.serviceWith(entityGranularityConfig()).EvaluateAccess(s.ctx, entity.ID,
		providers.AccessPlaneAuthentication, providers.AuthenticationMethodCredential)
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
			port := newEntityStateProviderMock(s.T())
			svc := newTestService(s.T(), port, cfg, nil, nil)
			svc.now = func() time.Time { return s.now }

			// Both keys hold lapsed state.
			port.On("GetGovernedEntity", mock.Anything, "e1", true).Return(&GovernedEntity{
				ID: "e1", Category: providers.EntityCategoryUser, State: providers.EntityStateActive,
				AccessState: AccessState{
					Lock: LockState{
						Entity: &LockEntry{LockCount: 1},
						AuthenticationMethods: map[providers.AuthenticationMethod]LockEntry{
							providers.AuthenticationMethodCredential: {LockCount: 1},
						},
					},
				},
			}, nil).Twice()

			port.On("ClearAccessStateIfUnchanged", mock.Anything, "e1", providers.AuthenticationMethodCredential,
				int64(0)).
				Return(true, nil).Once()
			step, err := svc.AdmitAuthenticationStep(s.ctx, "e1", providers.AuthenticationMethodCredential)
			s.Require().Nil(err)
			s.True(step.Admitted)

			port.On("ClearAccessStateIfUnchanged", mock.Anything, "e1", AuthenticationMethodEntity, int64(0)).
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

// recordingConfig is a section in which users record logins at the given resolution.
func recordingConfig(resolution time.Duration) accountAccessValue {
	return accountAccessValue{User: &accountAccessCategory{
		RecordLastLogin: new(true), LastLoginResolutionSeconds: new(int(resolution / time.Second)),
	}}
}

// expectActivity serves recordingConfig once and a user profile.
func (s *ServiceTestSuite) expectActivity(resolution time.Duration) {
	s.source.EXPECT().Current(mock.Anything).Return(recordingConfig(resolution)).Once()
	s.port.On("GetEntityProfile", mock.Anything, mock.Anything).
		Return(&providers.Entity{Category: providers.EntityCategoryUser}, nil).Maybe()
}

// A recording category writes the stamp and passes the resolution window to the write.
func (s *ServiceTestSuite) TestRecordLoginStampsWhenTheCategoryRecords() {
	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectActivity(time.Hour)
	s.port.On("RecordLogin", mock.Anything, entity.ID, s.now, s.now.Add(-time.Hour)).
		Return(nil).Once()

	s.Require().Nil(s.svc.RecordLogin(s.ctx, entity.ID))
}

func (s *ServiceTestSuite) TestRecordLoginSkipsEntityReadsWhenNoCategoryRecords() {
	s.source.EXPECT().Current(mock.Anything).Return(accountAccessValue{}).Once()

	s.Require().Nil(s.svc.RecordLogin(s.ctx, "e1"))

	s.port.AssertNotCalled(s.T(), "GetEntityProfile", mock.Anything, mock.Anything)
	s.port.AssertNotCalled(s.T(), "GetGovernedEntity", mock.Anything, mock.Anything, mock.Anything)
	s.port.AssertNotCalled(s.T(), "RecordLogin",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// Once any category records, the entity's own category decides, and an application never records.
func (s *ServiceTestSuite) TestRecordLoginWritesNothingWhenThisCategoryDoesNot() {
	everyCategoryRecords := recordingConfig(0)
	everyCategoryRecords.Agent = &accountAccessCategory{RecordLastLogin: new(true)}
	testCases := []struct {
		name     string
		config   accountAccessValue
		category providers.EntityCategory
	}{
		{name: "agent", config: recordingConfig(0), category: providers.EntityCategoryAgent},
		{name: "application", config: everyCategoryRecords, category: providers.EntityCategoryApp},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.source.EXPECT().Current(mock.Anything).Return(tc.config).Once()
			s.port.On("GetEntityProfile", mock.Anything, "e1").
				Return(&providers.Entity{Category: tc.category}, nil).Once()

			s.Require().Nil(s.svc.RecordLogin(s.ctx, "e1"))

			s.port.AssertNotCalled(s.T(), "RecordLogin",
				mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// The window is passed to the write, so the store compares without a read-modify-write.
func (s *ServiceTestSuite) TestRecordLoginPassesTheResolutionWindowToTheWrite() {
	entity := s.activeEntity()
	s.expectActivity(15 * time.Minute)
	s.port.On("RecordLogin", mock.Anything, entity.ID, s.now, s.now.Add(-15*time.Minute)).
		Return(nil).Once()

	s.Require().Nil(s.svc.RecordLogin(s.ctx, entity.ID))
}

// A zero resolution collapses the window to a single instant.
func (s *ServiceTestSuite) TestRecordLoginWithNoResolutionCollapsesTheWindow() {
	entity := s.activeEntity()
	s.expectRead(entity)
	s.expectActivity(0)
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
	s.expectActivity(time.Hour)
	s.port.On("RecordLogin", mock.Anything, entity.ID, s.now, s.now.Add(-time.Hour)).
		Return(nil).Once()

	s.Require().Nil(s.svc.RecordLogin(s.ctx, entity.ID))

	authorizer.AssertNotCalled(s.T(), "IsActionAllowed", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestRecordLoginNeverReadsTheRuntimeRow() {
	s.expectActivity(time.Hour)
	s.port.On("RecordLogin", mock.Anything, "e1", s.now, s.now.Add(-time.Hour)).Return(nil).Once()

	s.Require().Nil(s.svc.RecordLogin(s.ctx, "e1"))

	s.port.AssertNotCalled(s.T(), "GetGovernedEntity", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestAdmitAuthenticationStepRefusesWithoutReset() {
	entity := s.activeEntity()
	entity.State = providers.EntityStateSuspended
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Once()
	s.expectConfig(methodGranularityConfig())
	decision, err := s.svc.AdmitAuthenticationStep(s.ctx, entity.ID, testMethod)
	s.Require().Nil(err)
	s.False(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "ClearAccessStateIfUnchanged",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestAdmitAuthenticationStepReadFailureDoesNotReset() {
	readErr := errors.New("database unavailable")
	s.port.On("GetGovernedEntity", mock.Anything, "e1", true).Return(nil, readErr).Once()
	decision, err := s.svc.AdmitAuthenticationStep(s.ctx, "e1", testMethod)
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
			putLockEntry(&entity.AccessState, testMethod, LockEntry{FailureCount: 2})
			read := s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Once()
			current := s.source.EXPECT().Current(mock.Anything).Return(methodGranularityConfig()).Once()
			s.port.On("ClearAccessStateIfUnchanged", mock.Anything, entity.ID, testMethod, int64(0)).
				NotBefore(read, current).Return(false, resetErr).Once()
			decision, err := s.svc.AdmitAuthenticationStep(s.ctx, entity.ID, testMethod)
			s.Require().Nil(err)
			s.True(decision.Admitted)
		})
	}
}

func (s *ServiceTestSuite) TestAdmitAuthenticationStepUngovernedSubjectDoesNoIO() {
	decision, err := s.svc.AdmitAuthenticationStep(s.ctx, "", testMethod)
	s.Require().Nil(err)
	s.True(decision.Admitted)
}

// A completed sign-in clears a lapsed entity-wide record: the counter and the escalation.
func (s *ServiceTestSuite) TestAdmitSignInClearsALapsedEntityRecord() {
	entity := s.activeEntity()
	entity.AccessState.Lock.Entity = &LockEntry{FailureCount: 2, LockCount: 1, UnlockAt: s.at(-time.Second)}
	s.expectRead(entity)
	s.expectConfig(methodGranularityConfig())
	s.port.On("ClearAccessStateIfUnchanged", mock.Anything, entity.ID, AuthenticationMethodEntity, int64(0)).
		Return(true, nil).Once()

	decision, err := s.svc.AdmitSignIn(s.ctx, entity.ID)
	s.Require().Nil(err)
	s.True(decision.Admitted)
}

// A live entity-wide lock, including the permanent one Unsuspend writes, is never cleared.
func (s *ServiceTestSuite) TestAdmitSignInNeverClearsALiveLock() {
	for name, unlockAt := range map[string]string{
		"timed":     s.at(5 * time.Minute),
		"permanent": permanentUnlockAt,
	} {
		s.Run(name, func() {
			s.SetupTest()
			entity := s.activeEntity()
			entity.AccessState.Lock.Entity = &LockEntry{LockCount: 1, UnlockAt: unlockAt}
			s.expectRead(entity)
			s.expectConfig(methodGranularityConfig())

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
	entity.AccessState.Lock.Entity = &LockEntry{FailureCount: 2}
	s.expectRead(entity)
	s.expectConfig(methodGranularityConfig())

	decision, err := s.svc.AdmitSignIn(s.ctx, entity.ID)
	s.Require().Nil(err)
	s.False(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "ClearAccessStateIfUnchanged",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A clean account and a failed clear both admit, and a clean one writes nothing.
func (s *ServiceTestSuite) TestAdmitSignInOnACleanAccountWritesNothing() {
	s.expectRead(s.activeEntity())
	s.expectConfig(methodGranularityConfig())

	decision, err := s.svc.AdmitSignIn(s.ctx, "e1")
	s.Require().Nil(err)
	s.True(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "ClearAccessStateIfUnchanged",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A clear that fails, or that finds the entry changed since the read, still admits.
func (s *ServiceTestSuite) TestAdmitSignInClearFailureStillAdmits() {
	for name, clearErr := range map[string]error{"failed": errors.New("write failed"), "changed": nil} {
		s.Run(name, func() {
			s.SetupTest()
			entity := s.activeEntity()
			entity.AccessState.Lock.Entity = &LockEntry{FailureCount: 2}
			s.expectRead(entity)
			s.expectConfig(methodGranularityConfig())
			s.port.On("ClearAccessStateIfUnchanged", mock.Anything, entity.ID, AuthenticationMethodEntity, int64(0)).
				Return(false, clearErr).Once()

			decision, err := s.svc.AdmitSignIn(s.ctx, entity.ID)
			s.Require().Nil(err)
			s.True(decision.Admitted)
		})
	}
}

// An unreadable authority refuses, and an ungoverned subject is admitted without IO.
func (s *ServiceTestSuite) TestAdmitSignInFailsClosedAndSkipsTheUngoverned() {
	readErr := errors.New("database unavailable")
	s.port.On("GetGovernedEntity", mock.Anything, "e1", true).Return(nil, readErr).Once()
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
			port := newEntityStateProviderMock(s.T())
			port.On("GetGovernedEntity", mock.Anything, "e1", true).Return(new(governedSubject(tc.category)), nil)
			authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
			authz.EXPECT().IsActionAllowed(mock.Anything, tc.action, &sysauthz.ActionContext{
				ResourceType: tc.resource, OUID: "ou-a", ResourceID: "e1",
			}).Return(false, nil).Times(6)
			svc := newTestService(s.T(), port, accountAccessValue{}, nil, authz)
			for _, operation := range []struct {
				name string
				run  func() *tidcommon.ServiceError
			}{
				{"suspend", func() *tidcommon.ServiceError { return svc.Suspend(callerContext("operator"), "e1", "") }},
				{"validate suspend", func() *tidcommon.ServiceError {
					return svc.ValidateSuspend(callerContext("operator"), "e1", "")
				}},
				// Authorized before the note is checked, so the limit cannot be probed from outside.
				{"suspend with an over-long note", func() *tidcommon.ServiceError {
					return svc.Suspend(callerContext("operator"), "e1", strings.Repeat("n", maxOperatorNoteLength+1))
				}},
				{"validate suspend with an over-long note", func() *tidcommon.ServiceError {
					return svc.ValidateSuspend(callerContext("operator"), "e1",
						strings.Repeat("n", maxOperatorNoteLength+1))
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
	port := newEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1", true).Return(&suspended, nil)
	authz := s.authorizer(false)

	s.requireServiceError(newTestService(s.T(), port, accountAccessValue{}, nil, authz).
		Suspend(callerContext("operator"), "e1", ""), tidcommon.ErrorUnauthorized)
	authz.AssertNumberOfCalls(s.T(), "IsActionAllowed", 1)
	port.AssertNotCalled(s.T(), "SetSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// The suspend pre-check authorizes and writes nothing, even on a suspended identity.
func (s *ServiceTestSuite) TestValidateSuspendWritesNothing() {
	suspended := governedSubject(providers.EntityCategoryUser)
	suspended.State = providers.EntityStateSuspended
	port := newEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1", true).Return(&suspended, nil)
	authz := s.authorizer(true)
	svc := newTestService(s.T(), port, accountAccessValue{}, nil, authz)

	s.Require().Nil(svc.ValidateSuspend(callerContext("operator"), "e1", ""))
	authz.AssertNumberOfCalls(s.T(), "IsActionAllowed", 1)
	s.requireServiceError(svc.ValidateSuspend(callerContext("operator"), "", ""), ErrorMissingSubject)
	port.AssertNotCalled(s.T(), "SetSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// An authorizer that cannot decide fails the operation as a server error, and nothing is written.
func (s *ServiceTestSuite) TestLifecycleAuthorizationFailureIsAServerError() {
	port := newEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1", true).
		Return(new(governedSubject(providers.EntityCategoryAgent)), nil)
	authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	authz.EXPECT().IsActionAllowed(mock.Anything, mock.Anything, mock.Anything).
		Return(false, &tidcommon.InternalServerError)

	s.requireServiceError(newTestService(s.T(), port, accountAccessValue{}, nil, authz).
		Unlock(callerContext("operator"), "e1"), tidcommon.InternalServerError)
	port.AssertNotCalled(s.T(), "ClearAccessState", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestLifecycleSelfTargetRefusedBeforeTheAuthorizer() {
	for _, category := range []providers.EntityCategory{providers.EntityCategoryUser, providers.EntityCategoryAgent} {
		authz := s.authorizer(true)
		svc := newTestService(s.T(), nil, accountAccessValue{}, nil, authz)
		s.requireServiceError(svc.authorize(callerContext("e1"), governedSubject(category)),
			tidcommon.ErrorUnauthorized)
		authz.AssertNotCalled(s.T(), "IsActionAllowed", mock.Anything, mock.Anything, mock.Anything)
		s.Require().Nil(svc.authorize(security.WithRuntimeContext(callerContext("e1")), governedSubject(category)))
		authz.AssertNumberOfCalls(s.T(), "IsActionAllowed", 1)
	}
}

func (s *ServiceTestSuite) TestApplicationHasNoSystemAuthorizationVocabulary() {
	authz := s.authorizer(false)
	s.Require().Nil(newTestService(s.T(), nil, accountAccessValue{}, nil, authz).
		authorize(callerContext("operator"), governedSubject(providers.EntityCategoryApp)))
	authz.AssertNotCalled(s.T(), "IsActionAllowed", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestEmbeddableGovernanceWithoutAuthorization() {
	s.Require().Nil(newTestService(s.T(), nil, accountAccessValue{}, nil, nil).
		authorize(context.Background(), governedSubject(providers.EntityCategoryUser)))
}

// lockFormationConfig is a user policy that locks at the third failure.
func lockFormationConfig() accountAccessValue {
	return accountAccessValue{
		User: &accountAccessCategory{
			LockGranularity: new(granularityAuthenticationMethod),
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
func (s *ServiceTestSuite) notifyingService() (*identityGovernanceService, *LockNotifierInterfaceMock) {
	notifier := NewLockNotifierInterfaceMock(s.T())
	svc := newTestService(s.T(), s.port, lockFormationConfig(), notifier, nil)
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
	s.port.EXPECT().GetGovernedEntity(mock.Anything, entity.ID, true).Return(&entity, nil)
	s.port.EXPECT().IncrementFailure(mock.Anything, entity.ID,
		providers.AuthenticationMethodCredential, mock.Anything, mock.Anything).
		Return(LockEntry{FailureCount: count}, true, nil)
}

// A committed lock notifies with the method that was held.
func (s *ServiceTestSuite) TestCommittedLockNotifies() {
	svc, notifier := s.notifyingService()
	s.expectFailureReaching(3)
	s.port.EXPECT().FormLock(mock.Anything, "user-1", providers.AuthenticationMethodCredential,
		mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	notifier.EXPECT().NotifyLockFormed(mock.Anything, mock.MatchedBy(func(e GovernedEntity) bool {
		return e.ID == "user-1"
	}), providers.AuthenticationMethodCredential, mock.MatchedBy(func(episode LockEpisode) bool {
		return episode.Duration == 5*time.Minute && episode.UnlockAt == s.now.Add(5*time.Minute).Format(time.RFC3339)
	})).Return()

	s.Require().Nil(s.admittedFailure(svc.RecordFailure(s.ctx, "user-1", providers.AccessPlaneAuthentication,
		providers.AuthenticationMethodCredential)))
}

func (s *ServiceTestSuite) TestOrdinaryFailureDoesNotNotify() {
	svc, notifier := s.notifyingService()
	s.expectFailureReaching(1)

	s.Require().Nil(s.admittedFailure(svc.RecordFailure(s.ctx, "user-1", providers.AccessPlaneAuthentication,
		providers.AuthenticationMethodCredential)))
	notifier.AssertNotCalled(s.T(), "NotifyLockFormed", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// Of two concurrent failures, only the guard's winner notifies.
func (s *ServiceTestSuite) TestLosingWriterDoesNotNotify() {
	svc, notifier := s.notifyingService()
	s.expectFailureReaching(3)
	s.port.EXPECT().FormLock(mock.Anything, "user-1", providers.AuthenticationMethodCredential,
		mock.Anything, mock.Anything, mock.Anything).Return(false, nil)

	// The re-read shows the lock another writer formed, so this attempt stops.
	locked := s.governedUser()
	locked.AccessState = AccessState{Lock: LockState{
		AuthenticationMethods: map[providers.AuthenticationMethod]LockEntry{
			providers.AuthenticationMethodCredential: {
				FailureCount: 3,
				LockCount:    1,
				UnlockAt:     s.now.Add(5 * time.Minute).UTC().Format(time.RFC3339),
			},
		}}}
	s.port.EXPECT().GetGovernedEntity(mock.Anything, "user-1", true).Return(&locked, nil)

	s.Require().Nil(s.admittedFailure(svc.RecordFailure(s.ctx, "user-1", providers.AccessPlaneAuthentication,
		providers.AuthenticationMethodCredential)))
	notifier.AssertNotCalled(s.T(), "NotifyLockFormed", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A failure under a live hold is not counted, so it does not notify again.
func (s *ServiceTestSuite) TestFailureUnderALiveHoldDoesNotNotify() {
	svc, notifier := s.notifyingService()
	entity := s.governedUser()
	entity.AccessState = AccessState{Lock: LockState{Entity: &LockEntry{
		UnlockAt: s.now.Add(5 * time.Minute).UTC().Format(time.RFC3339),
	}}}
	s.port.EXPECT().GetGovernedEntity(mock.Anything, entity.ID, true).Return(&entity, nil)

	decision, err := svc.RecordFailure(s.ctx, "user-1", providers.AccessPlaneAuthentication,
		providers.AuthenticationMethodCredential)
	s.Require().Nil(err)
	s.False(decision.Admitted)
	notifier.AssertNotCalled(s.T(), "NotifyLockFormed", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestNoNotifierStillLocks() {
	svc := newTestService(s.T(), s.port, lockFormationConfig(), nil, nil)
	svc.now = func() time.Time { return s.now }

	s.expectFailureReaching(3)
	formed := s.port.EXPECT().FormLock(mock.Anything, "user-1", providers.AuthenticationMethodCredential,
		mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	s.Require().Nil(s.admittedFailure(svc.RecordFailure(s.ctx, "user-1", providers.AccessPlaneAuthentication,
		providers.AuthenticationMethodCredential)))
	s.NotNil(formed)
}

// lockAuditedService builds the service over cfg with its events landing in the returned slice.
func (s *ServiceTestSuite) lockAuditedService(
	cfg accountAccessValue) (*identityGovernanceService, *[]*providers.Event) {
	published := []*providers.Event{}
	obs := observabilityprovidermock.NewObservabilityProviderMock(s.T())
	obs.EXPECT().IsEnabled().Return(true).Maybe()
	obs.EXPECT().PublishEvent(mock.Anything, mock.Anything).
		Run(func(_ context.Context, evt *providers.Event) { published = append(published, evt) }).Maybe()
	svc := newTestService(s.T(), s.port, cfg, nil, nil)
	svc.observability = obs
	svc.now = func() time.Time { return s.now }
	return svc, &published
}

// A committed lock is published once, with the target, the method and when it lifts, and no actor.
func (s *ServiceTestSuite) TestAFormedLockIsPublishedOnce() {
	svc, published := s.lockAuditedService(lockFormationConfig())
	s.expectFailureReaching(3)
	s.port.EXPECT().FormLock(mock.Anything, "user-1", providers.AuthenticationMethodCredential,
		mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	s.Require().Nil(s.admittedFailure(svc.RecordFailure(security.WithRuntimeContext(s.ctx), "user-1",
		providers.AccessPlaneAuthentication, providers.AuthenticationMethodCredential)))

	s.Require().Len(*published, 1)
	evt := (*published)[0]
	s.Equal(string(event.EventTypeIdentityLocked), evt.Type)
	s.Equal(providers.StatusSuccess, evt.Status)
	s.Equal("user-1", evt.Data[event.DataKey.Subject])
	s.Equal(event.PrincipalTypeUser, evt.Data[event.DataKey.SubjectType])
	s.Equal(string(providers.AuthenticationMethodCredential), evt.Data[event.DataKey.LockMethod])
	s.Equal(1, evt.Data[event.DataKey.LockCount])
	s.Equal(false, evt.Data[event.DataKey.LockPermanent])
	s.Equal(s.now.Add(5*time.Minute).Format(time.RFC3339), evt.Data[event.DataKey.UnlockAt])
	s.NotContains(evt.Data, event.DataKey.ActorSub)
	category, err := event.GetCategory(providers.EventType(evt.Type))
	s.Require().NoError(err)
	s.Equal(event.CategoryAuthentication, category)
}

// A lock that does not expire is published as permanent, with no unlock time.
func (s *ServiceTestSuite) TestAPermanentLockIsPublishedWithoutAnUnlockTime() {
	cfg := lockFormationConfig()
	cfg.User.Default.LockDurationsSeconds = []int{0}
	svc, published := s.lockAuditedService(cfg)
	s.expectFailureReaching(3)
	s.port.EXPECT().FormLock(mock.Anything, "user-1", providers.AuthenticationMethodCredential,
		mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	s.Require().Nil(s.admittedFailure(svc.RecordFailure(s.ctx, "user-1", providers.AccessPlaneAuthentication,
		providers.AuthenticationMethodCredential)))

	s.Require().Len(*published, 1)
	s.Equal(true, (*published)[0].Data[event.DataKey.LockPermanent])
	s.NotContains((*published)[0].Data, event.DataKey.UnlockAt)
}

// A failure that forms no lock, and a formation writer that loses the guard, publish nothing.
func (s *ServiceTestSuite) TestNoLockFormedPublishesNothing() {
	s.Run("below the threshold", func() {
		s.SetupTest()
		svc, published := s.lockAuditedService(lockFormationConfig())
		s.expectFailureReaching(1)

		s.Require().Nil(s.admittedFailure(svc.RecordFailure(s.ctx, "user-1", providers.AccessPlaneAuthentication,
			providers.AuthenticationMethodCredential)))
		s.Empty(*published)
	})
	s.Run("losing writer", func() {
		s.SetupTest()
		svc, published := s.lockAuditedService(lockFormationConfig())
		s.expectFailureReaching(3)
		s.port.EXPECT().FormLock(mock.Anything, "user-1", providers.AuthenticationMethodCredential,
			mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
		locked := s.governedUser()
		putLockEntry(&locked.AccessState, providers.AuthenticationMethodCredential,
			LockEntry{FailureCount: 3, LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
		s.port.EXPECT().GetGovernedEntity(mock.Anything, "user-1", true).Return(&locked, nil)

		s.Require().Nil(s.admittedFailure(svc.RecordFailure(s.ctx, "user-1", providers.AccessPlaneAuthentication,
			providers.AuthenticationMethodCredential)))
		s.Empty(*published)
	})
}

// A check on an account with no entries resolves no policy. The source mock is strict, so a read of the
// section fails the test.
func (s *ServiceTestSuite) TestACleanAccountIsAnsweredWithoutAPolicy() {
	entity := s.activeEntity()
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Once()

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testMethod)

	s.Require().Nil(err)
	s.True(decision.Admitted)
	s.source.AssertNotCalled(s.T(), "Current", mock.Anything)
}

func (s *ServiceTestSuite) TestAnApplicationIsAnsweredWithoutAPolicy() {
	entity := s.activeEntity()
	entity.Category = providers.EntityCategoryApp
	entity.AccessState = AccessState{
		Lock: LockState{
			AuthenticationMethods: map[providers.AuthenticationMethod]LockEntry{
				providers.AuthenticationMethodSystemCredential: {UnlockAt: s.at(time.Hour)},
			},
		},
	}
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Once()

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneApplication, "")

	s.Require().Nil(err)
	s.True(decision.Admitted)
	s.source.AssertNotCalled(s.T(), "Current", mock.Anything)
}

func (s *ServiceTestSuite) TestASuspendedApplicationStillRefuses() {
	entity := s.activeEntity()
	entity.Category = providers.EntityCategoryApp
	entity.State = providers.EntityStateSuspended
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Once()
	s.source.EXPECT().Current(mock.Anything).Return(accountAccessValue{}).Once()

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
func auditedService(t *testing.T, port entityStateProvider,
	authz sysauthz.SystemAuthorizationServiceInterface) (*identityGovernanceService, *[]*providers.Event) {
	published := []*providers.Event{}
	obs := observabilityprovidermock.NewObservabilityProviderMock(t)
	obs.EXPECT().IsEnabled().Return(true).Maybe()
	obs.EXPECT().PublishEvent(mock.Anything, mock.Anything).
		Run(func(_ context.Context, evt *providers.Event) { published = append(published, evt) }).Maybe()
	svc := newTestService(t, port, accountAccessValue{}, nil, authz)
	svc.observability = obs
	return svc, &published
}

// A placed suspension is published with its actor and target. Only whether a note was given is
// recorded, never the text.
func (s *ServiceTestSuite) TestASuspensionIsPublishedForAudit() {
	port := newEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1", true).
		Return(new(governedSubject(providers.EntityCategoryUser)), nil)
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
	s.Require().Equal("organization", evt.Data[event.DataKey.AppliedBy])
	for key, value := range evt.Data {
		s.Require().NotContains(fmt.Sprint(value), "Credential exposure", "field %s carries the note", key)
	}
}

// The owner's suspension is published as the owner's, with no actor, since it runs with the runtime context.
func (s *ServiceTestSuite) TestAnOwnerSuspensionIsPublishedAsTheOwners() {
	port := newEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1", true).
		Return(new(governedSubject(providers.EntityCategoryAgent)), nil)
	port.On("SetOwnerSuspension", mock.Anything, "e1", mock.Anything).Return(true, nil)
	svc, published := auditedService(s.T(), port, authorizerAllowing(s.T(), true))

	s.Require().Nil(svc.Suspend(WithOwnerApplier(security.WithRuntimeContext(context.Background())), "e1", ""))

	s.Require().Len(*published, 1)
	evt := (*published)[0]
	s.Require().Equal(string(event.EventTypeIdentitySuspended), evt.Type)
	s.Require().Equal("owner", evt.Data[event.DataKey.AppliedBy])
	s.Require().NotContains(evt.Data, event.DataKey.ActorSub)
}

// A refused or inapplicable operation is recorded as a failure, with why, and writes nothing.
func (s *ServiceTestSuite) TestARefusedLifecycleOperationIsPublishedAsAFailure() {
	agent := governedSubject(providers.EntityCategoryAgent)
	for _, tc := range []struct {
		name    string
		entity  GovernedEntity
		allowed bool
		run     func(*identityGovernanceService) *tidcommon.ServiceError
		want    providers.EventType
		failure string
	}{
		{"suspend outside the boundary", agent, false,
			func(s *identityGovernanceService) *tidcommon.ServiceError {
				return s.Suspend(callerContext("operator"), "e1", "")
			},
			event.EventTypeIdentitySuspensionFailed, "not_authorized"},
		{"suspend pre-check outside the boundary", agent, false,
			func(s *identityGovernanceService) *tidcommon.ServiceError {
				return s.ValidateSuspend(callerContext("operator"), "e1", "")
			},
			event.EventTypeIdentitySuspensionFailed, "not_authorized"},
		{"suspend with an over-long note", agent, true,
			func(s *identityGovernanceService) *tidcommon.ServiceError {
				return s.Suspend(callerContext("operator"), "e1", strings.Repeat("n", maxOperatorNoteLength+1))
			},
			event.EventTypeIdentitySuspensionFailed, "note_too_long"},
		{"suspend pre-check with an over-long note", agent, true,
			func(s *identityGovernanceService) *tidcommon.ServiceError {
				return s.ValidateSuspend(callerContext("operator"), "e1", strings.Repeat("n", maxOperatorNoteLength+1))
			},
			event.EventTypeIdentitySuspensionFailed, "note_too_long"},
		{"unsuspend on an active identity", agent, true,
			func(s *identityGovernanceService) *tidcommon.ServiceError {
				return s.Unsuspend(callerContext("operator"), "e1")
			},
			event.EventTypeIdentityUnsuspensionFailed, "not_applicable"},
		{"unlock outside the boundary", agent, false,
			func(s *identityGovernanceService) *tidcommon.ServiceError {
				return s.Unlock(callerContext("operator"), "e1")
			},
			event.EventTypeIdentityUnlockFailed, "not_authorized"},
	} {
		s.Run(tc.name, func() {
			port := newEntityStateProviderMock(s.T())
			port.On("GetGovernedEntity", mock.Anything, "e1", true).Return(&tc.entity, nil)
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
	port := newEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1", true).
		Return(new(governedSubject(providers.EntityCategoryUser)), nil)
	port.On("ClearAccessState", mock.Anything, "e1", AllAuthenticationMethods).Return(nil)
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
	port := newEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1", true).
		Return(new(governedSubject(providers.EntityCategoryUser)), nil)
	port.On("ClearAccessState", mock.Anything, "e1", AllAuthenticationMethods).Return(errors.New("db down"))
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
	port := newEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1", true).
		Return(new(governedSubject(providers.EntityCategoryUser)), nil)
	port.On("ClearAccessState", mock.Anything, "e1", AllAuthenticationMethods).Return(nil)
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
	port := newEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1", true).
		Return(new(governedSubject(providers.EntityCategoryUser)), nil)
	svc, published := auditedService(s.T(), port, authorizerAllowing(s.T(), true))

	s.Require().Nil(svc.ValidateSuspend(callerContext("operator"), "e1", ""))
	s.Require().Empty(*published)
}

func (s *ServiceTestSuite) TestARepeatedSuspensionIsPublished() {
	suspended := governedSubject(providers.EntityCategoryUser)
	suspended.State = providers.EntityStateSuspended
	port := newEntityStateProviderMock(s.T())
	port.On("GetGovernedEntity", mock.Anything, "e1", true).Return(&suspended, nil)
	svc, published := auditedService(s.T(), port, authorizerAllowing(s.T(), true))

	s.Require().Nil(svc.Suspend(callerContext("operator"), "e1", ""))

	s.Require().Len(*published, 1)
	s.Require().Equal(string(event.EventTypeIdentitySuspended), (*published)[0].Type)
	s.Require().Equal("Suspension already in place",
		(*published)[0].Data[event.DataKey.Message])
	port.AssertNotCalled(s.T(), "SetSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A suspension reads the policy once for its disclosure mode, and every policy names the suspension.
func (s *ServiceTestSuite) TestADisclosingPolicyNamesASuspension() {
	s.source.EXPECT().Current(mock.Anything).Return(methodGranularityConfig()).Once()
	s.expectRead(s.suspendedEntity())

	decision, err := s.svc.EvaluateAccess(s.ctx, "e1", providers.AccessPlaneAuthentication, testMethod)

	s.Require().Nil(err)
	s.Equal(providers.AccessDecision{Reason: providers.AccessRefusalReasonSuspended}, decision)
}

// A policy that does not disclose refuses a suspension and a method lock without naming either.
// Disclosure is fixed in code, so the decision is driven with such a policy directly.
func (s *ServiceTestSuite) TestANonDisclosingPolicyNamesNoHold() {
	s.expectConfig(methodGranularityConfig())
	policy := s.svc.resolvePolicy(s.ctx, s.activeEntity(), testMethod)
	policy.discloseAccountHold = discloseNever
	locked := s.activeEntity()
	locked.AccessState = lockedState(testMethod, LockEntry{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})

	for _, entity := range []GovernedEntity{s.suspendedEntity(), locked} {
		decision := s.svc.decideUnder(s.ctx, entity, providers.AccessPlaneAuthentication, testMethod, policy)

		s.Equal(providers.AccessDecision{}, decision)
	}
}

// A lock refusal says the account is locked, and nothing more.
func (s *ServiceTestSuite) TestALockRefusalNamesTheLock() {
	s.expectConfig(methodGranularityConfig())
	entity := s.activeEntity()
	entity.AccessState = lockedState(testMethod, LockEntry{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
	s.expectRead(entity)

	decision, err := s.svc.EvaluateAccess(s.ctx, "e1", providers.AccessPlaneAuthentication, testMethod)

	s.Require().Nil(err)
	s.Equal(providers.AccessDecision{Reason: providers.AccessRefusalReasonLocked}, decision)
}

// A failed counter write leaves the attempt uncounted and admitted: it never changes the answer. Only a
// failed read is an error.
func (s *ServiceTestSuite) TestAFailedCounterWriteLeavesTheAttemptAdmitted() {
	for name, arrange := range map[string]func(){
		"increment": func() {
			s.expectRead(s.activeEntity())
			s.port.On("IncrementFailure", mock.Anything, "e1", testMethod, mock.Anything, mock.Anything).
				Return(LockEntry{}, false, errors.New("database unavailable")).Once()
		},
		"lock write": func() {
			s.expectRead(s.activeEntity())
			s.port.On("IncrementFailure", mock.Anything, "e1", testMethod, mock.Anything, mock.Anything).
				Return(LockEntry{FailureCount: 5}, true, nil).Once()
			s.port.On("FormLock", mock.Anything, "e1", testMethod, mock.Anything, mock.Anything, mock.Anything).
				Return(false, errors.New("database unavailable")).Once()
		},
		"re-read after a lost guard": func() {
			s.port.On("GetGovernedEntity", mock.Anything, "e1", true).Return(new(s.activeEntity()), nil).Once()
			s.port.On("IncrementFailure", mock.Anything, "e1", testMethod, mock.Anything, mock.Anything).
				Return(LockEntry{FailureCount: 5}, true, nil).Once()
			s.port.On("FormLock", mock.Anything, "e1", testMethod, mock.Anything, mock.Anything, mock.Anything).
				Return(false, nil).Once()
			s.port.On("GetGovernedEntity", mock.Anything, "e1", true).
				Return(nil, errors.New("database unavailable")).Once()
		},
	} {
		s.Run(name, func() {
			s.SetupTest()
			s.expectConfig(methodGranularityConfig())
			arrange()

			decision, err := s.svc.RecordFailure(s.ctx, "e1", providers.AccessPlaneAuthentication, testMethod)

			s.Require().Nil(err)
			s.True(decision.Admitted)
			s.port.AssertExpectations(s.T())
		})
	}
}

// A failure on an identity a live lock already holds is refused with the lock, as a verified proof
// would be, and nothing is written.
func (s *ServiceTestSuite) TestAFailureOnAHeldIdentityIsRefusedWithoutCounting() {
	entity := s.activeEntity()
	putLockEntry(&entity.AccessState, testMethod,
		LockEntry{FailureCount: 5, LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Once()
	s.expectConfig(methodGranularityConfig())

	decision, err := s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testMethod)

	s.Require().Nil(err)
	s.Equal(providers.AccessDecision{Reason: providers.AccessRefusalReasonLocked}, decision)
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A wrong recovery code is admitted without a read, so a hold, a suspension included, cannot change its
// answer: the lock gives way to the code there, and only a correct code states the hold.
func (s *ServiceTestSuite) TestAWrongRecoveryCodeIsAdmittedWithoutARead() {
	entity := s.activeEntity()

	decision, err := s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneRecovery,
		providers.AuthenticationMethodOTP)

	s.Require().Nil(err)
	s.True(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "GetGovernedEntity", mock.Anything, mock.Anything, mock.Anything)
}

// A failure is refused, not counted, wherever a live lock holds the method at that plane.
func (s *ServiceTestSuite) TestAFailureMeetsEveryLockThatHoldsIt() {
	disabled := disabledConfig()
	for name, tc := range map[string]struct {
		lock  func(*AccessState)
		cfg   accountAccessValue
		plane providers.AccessPlane
	}{
		"entity-wide lock under method granularity": {
			lock: func(a *AccessState) {
				putLockEntry(a, AuthenticationMethodEntity, LockEntry{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
			},
			cfg:   methodGranularityConfig(),
			plane: providers.AccessPlaneAuthentication,
		},
		"password lock at the recovery plane": {
			lock: func(a *AccessState) {
				putLockEntry(a, testMethod, LockEntry{LockCount: 1, UnlockAt: s.at(5 * time.Minute)})
			},
			cfg:   methodGranularityConfig(),
			plane: providers.AccessPlaneRecovery,
		},
		"post-suspension hold under a disabled policy": {
			lock: func(a *AccessState) {
				putLockEntry(a, AuthenticationMethodEntity,
					LockEntry{UnlockAt: permanentUnlockAt, Reason: lockReasonPostSuspension})
			},
			cfg:   disabled,
			plane: providers.AccessPlaneAuthentication,
		},
	} {
		s.Run(name, func() {
			s.SetupTest()
			entity := s.activeEntity()
			tc.lock(&entity.AccessState)
			s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Once()
			s.expectConfig(tc.cfg)

			decision, err := s.svc.RecordFailure(s.ctx, entity.ID, tc.plane, testMethod)

			s.Require().Nil(err)
			s.Equal(providers.AccessDecision{Reason: providers.AccessRefusalReasonLocked}, decision)
			s.port.AssertNotCalled(s.T(), "IncrementFailure",
				mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// A failure on a suspended identity is refused with the suspension and not counted, whether locking is
// on or off.
func (s *ServiceTestSuite) TestAFailureOnASuspendedIdentityIsRefusedWithoutCounting() {
	stated := providers.AccessDecision{Reason: providers.AccessRefusalReasonSuspended}
	for name, tc := range map[string]struct {
		cfg      accountAccessValue
		decision providers.AccessDecision
	}{
		"locking enabled":  {cfg: methodGranularityConfig(), decision: stated},
		"locking disabled": {cfg: disabledConfig(), decision: stated},
	} {
		s.Run(name, func() {
			s.SetupTest()
			entity := s.suspendedEntity()
			s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Once()
			s.expectConfig(tc.cfg)

			decision, err := s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testMethod)

			s.Require().Nil(err)
			s.Equal(tc.decision, decision)
			s.port.AssertNotCalled(s.T(), "IncrementFailure",
				mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// A failure on an identity nothing holds is counted and admitted, from one read.
func (s *ServiceTestSuite) TestAFailureOnAFreeIdentityIsCountedAndAdmitted() {
	entity := s.activeEntity()
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Once()
	s.expectConfig(methodGranularityConfig())
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testMethod, mock.Anything, mock.Anything).
		Return(LockEntry{FailureCount: 1}, true, nil).Once()

	decision, err := s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testMethod)

	s.Require().Nil(err)
	s.True(decision.Admitted)
	s.port.AssertExpectations(s.T())
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
	s.port.On("ClearAccessState", mock.Anything, "e1", AllAuthenticationMethods).
		Return(errors.New("database unavailable")).Once()

	s.requireServiceError(s.svc.Unlock(s.ctx, "e1"), tidcommon.InternalServerError)
}

// A failed profile read or stamp write is a server error.
func (s *ServiceTestSuite) TestRecordLoginReportsStoreFailures() {
	s.source.EXPECT().Current(mock.Anything).Return(recordingConfig(0)).Twice()
	s.port.On("GetEntityProfile", mock.Anything, "e1").Return(nil, errors.New("database unavailable")).Once()
	s.port.On("GetGovernedEntity", mock.Anything, "e1", true).
		Return(nil, errors.New("database unavailable")).Once()

	s.requireServiceError(s.svc.RecordLogin(s.ctx, "e1"), tidcommon.InternalServerError)

	s.port.On("GetEntityProfile", mock.Anything, "e1").
		Return(&providers.Entity{Category: providers.EntityCategoryUser}, nil).Once()
	s.port.On("RecordLogin", mock.Anything, "e1", s.now, s.now).Return(errors.New("database unavailable")).Once()

	s.requireServiceError(s.svc.RecordLogin(s.ctx, "e1"), tidcommon.InternalServerError)
}

// An identity with no local record has nothing to govern, so every check admits it. Only that
// sentinel admits; any other read failure refuses, as the test below shows.
func (s *ServiceTestSuite) TestAnIdentityWithNoLocalRecordIsAdmitted() {
	s.port.On("GetGovernedEntity", mock.Anything, "ext-1", true).Return(nil, nil)

	decision, err := s.svc.EvaluateAccess(s.ctx, "ext-1", providers.AccessPlaneSession, "")
	s.Require().Nil(err)
	s.True(decision.Admitted)

	decision, err = s.svc.AdmitSignIn(s.ctx, "ext-1")
	s.Require().Nil(err)
	s.True(decision.Admitted)

	decision, err = s.svc.AdmitAuthenticationStep(s.ctx, "ext-1", testMethod)
	s.Require().Nil(err)
	s.True(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "ClearAccessStateIfUnchanged",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// An identity with no local record records nothing and reports no error.
func (s *ServiceTestSuite) TestAnIdentityWithNoLocalRecordRecordsNothing() {
	s.port.On("GetGovernedEntity", mock.Anything, "ext-1", true).Return(nil, nil)
	s.Nil(s.admittedFailure(s.svc.RecordFailure(s.ctx, "ext-1", providers.AccessPlaneAuthentication, testMethod)))
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	s.source.EXPECT().Current(mock.Anything).Return(recordingConfig(0)).Once()
	s.port.On("GetEntityProfile", mock.Anything, "ext-1").Return(nil, errors.New("entity not found")).Once()
	s.Nil(s.svc.RecordLogin(s.ctx, "ext-1"))
	s.port.AssertNotCalled(s.T(), "RecordLogin", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A failed read refuses a completed sign-in with a server error.
func (s *ServiceTestSuite) TestAdmitSignInReportsAFailedRead() {
	s.port.On("GetGovernedEntity", mock.Anything, "e1", true).
		Return(nil, errors.New("database unavailable")).Once()

	decision, err := s.svc.AdmitSignIn(s.ctx, "e1")

	s.requireServiceError(err, tidcommon.InternalServerError)
	s.False(decision.Admitted)
}

// An operation naming no identity is published with the no_subject class.
func (s *ServiceTestSuite) TestAMissingSubjectIsPublishedAsNoSubject() {
	svc, published := auditedService(s.T(), newEntityStateProviderMock(s.T()), nil)

	s.requireServiceError(svc.Unlock(callerContext("operator"), ""), ErrorMissingSubject)

	s.Require().Len(*published, 1)
	s.Equal(providers.StatusFailure, (*published)[0].Status)
	s.Equal("no_subject", (*published)[0].Data[event.DataKey.Error])
}

// A management read resolves the category's enforcement from the current section.
func (s *ServiceTestSuite) TestReadAccessUsesTheCurrentSection() {
	cfg := testAccountAccessConfig()
	cfg.User.Default.Enabled = new(false)
	svc := newTestService(s.T(), s.port, cfg, nil, nil)
	entity := providers.Entity{
		ID: "e1", Category: providers.EntityCategoryUser, State: providers.EntityStateActive,
		RuntimeAttributes: []byte(`{"accessState":{"lock":{"authenticationMethods":{"otp":` +
			`{"unlockAt":"9999-12-31T23:59:59Z"}}}}}`),
	}

	reads := svc.ReadAccess(s.ctx, []providers.Entity{entity})

	s.Require().Len(reads, 1)
	s.Equal(providers.AccessStatusActive, reads[0].Status.Value,
		"a lock whose policy is switched off is not reported")
}

func (s *ServiceTestSuite) TestReadEntityAccessReportsCurrentStatus() {
	for _, tc := range []struct {
		name     string
		category providers.EntityCategory
		state    providers.EntityState
		unlockAt string
		disabled bool
		corrupt  bool
		want     providers.AccessStatus
	}{
		{name: "active", category: providers.EntityCategoryUser, want: providers.AccessStatusActive},
		{name: "live user lock", category: providers.EntityCategoryUser,
			unlockAt: s.now.Add(time.Minute).Format(time.RFC3339), want: providers.AccessStatusLocked},
		{name: "expired lock", category: providers.EntityCategoryUser,
			unlockAt: s.now.Add(-time.Minute).Format(time.RFC3339), want: providers.AccessStatusActive},
		{name: "disabled lock", category: providers.EntityCategoryUser, disabled: true,
			unlockAt: permanentUnlockAt, want: providers.AccessStatusActive},
		{name: "agent lock", category: providers.EntityCategoryAgent,
			unlockAt: permanentUnlockAt, want: providers.AccessStatusLocked},
		{name: "suspension overrides lock", category: providers.EntityCategoryUser,
			state: providers.EntityStateSuspended, unlockAt: permanentUnlockAt, want: providers.AccessStatusSuspended},
		{name: "application suspension", category: providers.EntityCategoryApp,
			state: providers.EntityStateSuspended, want: providers.AccessStatusSuspended},
		{name: "corrupt attributes", category: providers.EntityCategoryUser, corrupt: true,
			want: providers.AccessStatusSuspended},
	} {
		s.Run(tc.name, func() {
			source := newAccountAccessSourceInterfaceMock(s.T())
			cfg := testAccountAccessConfig()
			agentPolicy := *cfg.User.Default
			cfg.Agent.Default = &agentPolicy
			cfg.User.Default.Enabled = new(tc.category != providers.EntityCategoryAgent && !tc.disabled)
			cfg.Agent.LockGranularity = new(granularityAuthenticationMethod)
			cfg.Agent.Default.Enabled = new(tc.category == providers.EntityCategoryAgent && !tc.disabled)
			if tc.category != providers.EntityCategoryApp {
				source.EXPECT().Current(s.ctx).Return(cfg).Once()
			}
			svc := newIdentityGovernanceService(newEntityStateProviderMock(s.T()), source, nil, nil, nil)
			svc.now = func() time.Time { return s.now }
			stored := RuntimeAttributes{AccessState: AccessState{LastLoginAt: "2026-09-07T10:00:00Z"}}
			if tc.unlockAt != "" {
				stored.AccessState.Lock.AuthenticationMethods = map[providers.AuthenticationMethod]LockEntry{
					providers.AuthenticationMethodCredential: {UnlockAt: tc.unlockAt},
				}
			}
			attributes, err := json.Marshal(stored)
			s.Require().NoError(err)
			if tc.corrupt {
				attributes = []byte(`{"accessState":`)
			}
			entity := providers.Entity{ID: "e1", Category: tc.category, State: tc.state, RuntimeAttributes: attributes}

			read := svc.ReadEntityAccess(s.ctx, entity)

			s.Equal(tc.want, read.Status.Value)
			if tc.corrupt {
				s.Empty(read.LastLoginAt)
			} else {
				s.Equal(stored.AccessState.LastLoginAt, read.LastLoginAt)
			}
		})
	}
}

// A page of one category resolves the policy once and reports each entity in order.
func (s *ServiceTestSuite) TestReadAccessResolvesThePolicyOncePerCall() {
	s.source.EXPECT().Current(mock.Anything).Return(testAccountAccessConfig()).Once()
	entities := []providers.Entity{
		{ID: "e1", Category: providers.EntityCategoryUser, State: providers.EntityStateSuspended},
		{ID: "e2", Category: providers.EntityCategoryUser, State: providers.EntityStateActive},
	}

	reads := s.svc.ReadAccess(s.ctx, entities)

	s.Require().Len(reads, 2)
	s.Equal(providers.AccessStatusSuspended, reads[0].Status.Value)
	s.Equal(providers.AccessStatusActive, reads[1].Status.Value)
}

// An application has no lockout policy: its read resolves no section and reports a recorded lock.
func (s *ServiceTestSuite) TestReadAccessForAnApplicationReadsNoSection() {
	entity := providers.Entity{
		ID: "a1", Category: providers.EntityCategoryApp, State: providers.EntityStateActive,
		RuntimeAttributes: []byte(`{"accessState":{"lock":{"entity":{"unlockAt":"9999-12-31T23:59:59Z"}}}}`),
	}

	reads := s.svc.ReadAccess(s.ctx, []providers.Entity{entity})

	s.Require().Len(reads, 1)
	s.Equal(providers.AccessStatusLocked, reads[0].Status.Value)
	s.source.AssertNotCalled(s.T(), "Current", mock.Anything)
}

// testAccountAccessConfig is a representative deployment: a user default and agent locking switched
// off.
func testAccountAccessConfig() accountAccessValue {
	return accountAccessValue{
		User: &accountAccessCategory{
			LockGranularity: new(granularityAuthenticationMethod),
			Default: &accountAccessPolicy{
				Enabled:              new(true),
				Threshold:            new(5),
				FailureWindowSeconds: new(900),
				LockDurationsSeconds: []int{300, 900, 3600, 86400},
				LockDecaySeconds:     new(86400),
			},
		},
		Agent: &accountAccessCategory{
			LockGranularity: new(granularityAuthenticationMethod),
			Default:         &accountAccessPolicy{Enabled: new(false)},
		},
	}
}

func entityOf(category providers.EntityCategory) GovernedEntity {
	return GovernedEntity{ID: "e1", Category: category, State: providers.EntityStateActive}
}

// resolve resolves the lockout policy for a category at a method from one read of cfg.
func (s *ServiceTestSuite) resolve(cfg accountAccessValue, category providers.EntityCategory,
	method providers.AuthenticationMethod) lockoutPolicy {
	source := newAccountAccessSourceInterfaceMock(s.T())
	source.EXPECT().Current(mock.Anything).Return(cfg).Once()
	s.svc.source = source
	return s.svc.resolvePolicy(s.ctx, entityOf(category), method)
}

// activity resolves the activity settings from one read of cfg.
func (s *ServiceTestSuite) activity(cfg accountAccessValue) activityPolicies {
	source := newAccountAccessSourceInterfaceMock(s.T())
	source.EXPECT().Current(mock.Anything).Return(cfg).Once()
	s.svc.source = source
	return s.svc.resolveActivityPolicies(s.ctx)
}

// withOTPOverride is the test configuration with a tighter one-time password policy that states only
// what differs from the user default.
func withOTPOverride() accountAccessValue {
	cfg := testAccountAccessConfig()
	cfg.User.Methods = map[string]accountAccessPolicy{
		string(providers.AuthenticationMethodOTP): {
			Threshold:            new(3),
			FailureWindowSeconds: new(600),
			LockDurationsSeconds: []int{300, 900},
		},
	}
	return cfg
}

// The configured policy is translated whole, with seconds becoming durations.
func (s *ServiceTestSuite) TestResolvesTheConfiguredPolicy() {
	policy := s.resolve(testAccountAccessConfig(), providers.EntityCategoryUser,
		providers.AuthenticationMethodCredential)

	s.True(policy.enabled)
	s.Equal(5, policy.threshold)
	s.Equal(15*time.Minute, policy.failureWindow)
	s.Equal([]time.Duration{5 * time.Minute, 15 * time.Minute, time.Hour, 24 * time.Hour}, policy.lockDurations)
	s.Equal(24*time.Hour, policy.lockDecay)
	s.Equal(granularityAuthenticationMethod, policy.granularity)
}

// The consecutive ceiling is resolved from the policy; an absent one sets no ceiling.
func (s *ServiceTestSuite) TestTheConsecutiveCeilingIsResolved() {
	cfg := testAccountAccessConfig()
	s.Equal(0, s.resolve(cfg, providers.EntityCategoryUser,
		providers.AuthenticationMethodCredential).maxConsecutiveFailures,
		"an absent ceiling sets none")

	cfg.User.Default.MaxConsecutiveFailures = new(20)
	s.Equal(20, s.resolve(cfg, providers.EntityCategoryUser,
		providers.AuthenticationMethodCredential).maxConsecutiveFailures)
}

// Each category resolves its own policy.
func (s *ServiceTestSuite) TestResolvesPerCategory() {
	s.True(s.resolve(testAccountAccessConfig(), providers.EntityCategoryUser,
		providers.AuthenticationMethodCredential).enabled)
	agent := s.resolve(testAccountAccessConfig(), providers.EntityCategoryAgent,
		providers.AuthenticationMethodCredential)
	s.False(agent.enabled, "this deployment explicitly disables agent locking")
}

func (s *ServiceTestSuite) TestUnconfiguredCategoryLocksNothing() {
	policy := s.resolve(testAccountAccessConfig(), providers.EntityCategoryApp,
		providers.AuthenticationMethodCredential)

	s.False(policy.enabled)
	s.Equal(granularityAuthenticationMethod, policy.granularity, "and still defaults to per-authentication_method")
}

// A method override is overlaid on the category default field by field.
func (s *ServiceTestSuite) TestAnOverrideIsOverlaidOnTheDefault() {
	otp := s.resolve(withOTPOverride(), providers.EntityCategoryUser, providers.AuthenticationMethodOTP)

	s.True(otp.enabled)
	s.Equal(3, otp.threshold, "the overridden threshold")
	s.Equal(10*time.Minute, otp.failureWindow, "the overridden window")
	s.Equal([]time.Duration{5 * time.Minute, 15 * time.Minute}, otp.lockDurations)
	s.Equal(24*time.Hour, otp.lockDecay, "the decay the override omits comes from the default")
}

// A method no override names gets the category default.
func (s *ServiceTestSuite) TestAnUnnamedMethodGetsTheDefault() {
	credential := s.resolve(withOTPOverride(), providers.EntityCategoryUser, providers.AuthenticationMethodCredential)

	s.True(credential.enabled)
	s.Equal(5, credential.threshold)
	s.Equal(15*time.Minute, credential.failureWindow)
	s.Len(credential.lockDurations, 4)
}

// An override may switch one method off while the rest of the category stays protected.
func (s *ServiceTestSuite) TestAnOverrideMaySwitchOneMethodOff() {
	cfg := testAccountAccessConfig()
	cfg.User.Methods = map[string]accountAccessPolicy{string(providers.AuthenticationMethodOTP): {Enabled: new(false)}}

	s.False(s.resolve(cfg, providers.EntityCategoryUser, providers.AuthenticationMethodOTP).enabled)
	s.True(s.resolve(cfg, providers.EntityCategoryUser, providers.AuthenticationMethodCredential).enabled,
		"switching one method off must not switch the category off")
}

// Under entity granularity every method resolves the entity-wide policy, not its own.
func (s *ServiceTestSuite) TestEntityGranularityUsesTheEntityMethodPolicy() {
	cfg := testAccountAccessConfig()
	cfg.User.LockGranularity = new(granularityEntity)
	cfg.User.Methods = map[string]accountAccessPolicy{
		string(AuthenticationMethodEntity):        {Threshold: new(7)},
		string(providers.AuthenticationMethodOTP): {Threshold: new(2)},
	}

	otp := s.resolve(cfg, providers.EntityCategoryUser, providers.AuthenticationMethodOTP)

	s.Equal(7, otp.threshold, "the entity-wide override applies, not the method's")
	s.Equal(15*time.Minute, otp.failureWindow, "over the category default")
	s.Equal(granularityEntity, otp.granularity)
}

// Zero is a value, not an omission: a zero decay never decays and a zero duration is permanent.
func (s *ServiceTestSuite) TestZeroIsAValueRatherThanAnOmission() {
	cfg := testAccountAccessConfig()
	cfg.User.Default.LockDecaySeconds = new(0)
	cfg.User.Default.LockDurationsSeconds = []int{300, 0}

	policy := s.resolve(cfg, providers.EntityCategoryUser, providers.AuthenticationMethodCredential)

	s.Equal(time.Duration(0), policy.lockDecay, "an explicit zero decay is kept")
	s.Equal([]time.Duration{5 * time.Minute, 0}, policy.lockDurations)
}

// A policy that cannot form a lock is off: no threshold, no durations or no window.
func (s *ServiceTestSuite) TestPolicyThatCannotLockIsNotEnabled() {
	for name, edit := range map[string]func(*accountAccessPolicy){
		"zero threshold":   func(p *accountAccessPolicy) { p.Threshold = new(0) },
		"no durations":     func(p *accountAccessPolicy) { p.LockDurationsSeconds = nil },
		"no window at all": func(p *accountAccessPolicy) { p.FailureWindowSeconds = nil },
	} {
		cfg := testAccountAccessConfig()
		edit(cfg.User.Default)

		s.False(s.resolve(cfg, providers.EntityCategoryUser, providers.AuthenticationMethodCredential).enabled, name)
	}
}

// An omitted enabled flag keeps the default, which is on.
func (s *ServiceTestSuite) TestOmittedEnabledFlagDefaultsToOn() {
	cfg := testAccountAccessConfig()
	cfg.User.Default.Enabled = nil

	s.True(s.resolve(cfg, providers.EntityCategoryUser, providers.AuthenticationMethodCredential).enabled)
}

// An oversized seconds value is clamped rather than wrapping to a negative duration.
func (s *ServiceTestSuite) TestADurationTooLargeToCarryIsClampedRatherThanWrapped() {
	const overflowing = maxDurationSeconds + 1
	cfg := testAccountAccessConfig()
	cfg.User.Default.FailureWindowSeconds = new(overflowing)
	cfg.User.Default.LockDecaySeconds = new(overflowing)
	cfg.User.Default.LockDurationsSeconds = []int{overflowing}
	cfg.User.RecordLastLogin = new(true)
	cfg.User.LastLoginResolutionSeconds = new(overflowing)

	policy := s.resolve(cfg, providers.EntityCategoryUser, providers.AuthenticationMethodCredential)

	ceiling := time.Duration(maxDurationSeconds) * time.Second
	s.Equal(ceiling, policy.failureWindow)
	s.Equal(ceiling, policy.lockDecay)
	s.Equal([]time.Duration{ceiling}, policy.lockDurations)
	s.True(policy.enabled, "a window that wrapped would have switched the policy off")
	s.Equal(ceiling, s.activity(cfg).forCategory(providers.EntityCategoryUser).resolution)
}

// A window of zero keeps the policy on and never resets the counter on time.
func (s *ServiceTestSuite) TestAZeroWindowKeepsThePolicyEnabled() {
	cfg := testAccountAccessConfig()
	cfg.User.Default.FailureWindowSeconds = new(0)

	policy := s.resolve(cfg, providers.EntityCategoryUser, providers.AuthenticationMethodCredential)

	s.True(policy.enabled)
	s.Zero(policy.failureWindow)
}

// A category the configuration does not mention resolves the same as one switched off, for every method.
func (s *ServiceTestSuite) TestAbsentCategoryResolvesAsSwitchedOff() {
	stubbed := accountAccessValue{Agent: &accountAccessCategory{
		LockGranularity: new(granularityAuthenticationMethod),
		Default:         &accountAccessPolicy{Enabled: new(false)},
	}}

	for _, category := range []providers.EntityCategory{providers.EntityCategoryAgent, providers.EntityCategoryApp} {
		for _, method := range AllAuthenticationMethods {
			withStub := s.resolve(stubbed, category, method)
			withNone := s.resolve(accountAccessValue{}, category, method)

			s.Equal(withStub, withNone, "category %s method %s", category, method)
			s.False(withNone.enabled, "category %s method %s", category, method)
			s.Equal(granularityAuthenticationMethod, withNone.granularity, "category %s method %s", category, method)
		}
	}
}

// Every policy discloses the hold; the mode is fixed in code, not read from configuration.
func (s *ServiceTestSuite) TestEveryPolicyDisclosesTheHold() {
	for _, category := range []providers.EntityCategory{
		providers.EntityCategoryUser, providers.EntityCategoryAgent, providers.EntityCategoryApp,
	} {
		for _, method := range AllAuthenticationMethods {
			policy := s.resolve(testAccountAccessConfig(), category, method)
			s.Equal(discloseAlways, policy.discloseAccountHold, "category %s method %s", category, method)
		}
	}
}

// Activity recording is resolved per category from one read, separately from the lockout policy.
func (s *ServiceTestSuite) TestActivityPolicyIsResolvedPerCategory() {
	cfg := testAccountAccessConfig()
	cfg.User.RecordLastLogin = new(true)
	cfg.User.LastLoginResolutionSeconds = new(300)
	cfg.Agent.RecordLastLogin = new(false)

	policies := s.activity(cfg)

	s.True(policies.enabled())
	s.True(policies.forCategory(providers.EntityCategoryUser).record)
	s.Equal(5*time.Minute, policies.forCategory(providers.EntityCategoryUser).resolution)
	s.False(policies.forCategory(providers.EntityCategoryAgent).record,
		"a category may record nothing while its sibling records")
	s.Equal(activityPolicy{}, policies.forCategory(providers.EntityCategoryApp), "applications record nothing")
}

func (s *ServiceTestSuite) TestActivityRecordingIsOffWhenUnstated() {
	activity := s.activity(testAccountAccessConfig()).forCategory(providers.EntityCategoryUser)

	s.False(activity.record)
	s.Equal(time.Hour, activity.resolution,
		"the floor still has a default, so switching recording on alone cannot mean a write per sign-in")
}

// Resolution edges: zero writes every time, and a negative value clamps to zero.
func (s *ServiceTestSuite) TestActivityResolutionEdges() {
	for _, seconds := range []int{0, -60} {
		cfg := testAccountAccessConfig()
		cfg.User.RecordLastLogin = new(true)
		cfg.User.LastLoginResolutionSeconds = new(seconds)

		s.Zero(s.activity(cfg).forCategory(providers.EntityCategoryUser).resolution, "%d seconds", seconds)
	}
}

// One recording category is enough to reach the read that decides the rest.
func (s *ServiceTestSuite) TestActivityIsEnabledByAnyCategory() {
	cfg := testAccountAccessConfig()
	s.False(s.activity(cfg).enabled(), "recording off must not need an entity read")

	cfg.Agent.RecordLastLogin = new(true)
	s.True(s.activity(cfg).enabled())
}

// Under method granularity a live entity-wide failure lock is decided under the entity-wide policy,
// not the presented method's, so the decision agrees with the status a management read reports.
func (s *ServiceTestSuite) TestAnEntityWideLockIsDecidedUnderTheEntityMethodPolicy() {
	for name, tc := range map[string]struct {
		disabledMethod providers.AuthenticationMethod
		admitted       bool
		status         providers.AccessStatus
	}{
		"method switched off": {providers.AuthenticationMethodCredential, false,
			providers.AccessStatusLocked},
		"entity-wide policy switched off": {AuthenticationMethodEntity, true, providers.AccessStatusActive},
	} {
		s.Run(name, func() {
			s.SetupTest()
			cfg := methodGranularityConfig()
			cfg.User.Methods = map[string]accountAccessPolicy{string(tc.disabledMethod): {Enabled: new(false)}}
			s.expectConfig(cfg)
			entity := s.lockedEntityWide()
			s.expectRead(entity)
			attributes, err := json.Marshal(RuntimeAttributes{AccessState: entity.AccessState})
			s.Require().NoError(err)

			decision, svcErr := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
				providers.AuthenticationMethodCredential)
			reads := s.svc.ReadAccess(s.ctx, []providers.Entity{{
				ID: entity.ID, Category: entity.Category, State: entity.State, RuntimeAttributes: attributes,
			}})

			s.Require().Nil(svcErr)
			s.Equal(tc.admitted, decision.Admitted)
			s.Require().Len(reads, 1)
			s.Equal(tc.status, reads[0].Status.Value)
		})
	}
}

// A failure on a held account whose method policy is switched off is refused under the entity-wide
// policy, without counting.
func (s *ServiceTestSuite) TestAFailureUnderAnEntityWideLockIsRefusedWhenTheMethodIsSwitchedOff() {
	cfg := methodGranularityConfig()
	cfg.User.Methods = map[string]accountAccessPolicy{string(testMethod): {Enabled: new(false)}}
	s.expectConfig(cfg)
	s.expectRead(s.lockedEntityWide())

	decision, err := s.svc.RecordFailure(s.ctx, "e1", providers.AccessPlaneAuthentication, testMethod)

	s.Require().Nil(err)
	s.Equal(providers.AccessDecision{Reason: providers.AccessRefusalReasonLocked}, decision)
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A lifecycle operation on an ID no identity has is published with the not_found class.
func (s *ServiceTestSuite) TestAnUnknownIdentityIsPublishedAsNotFound() {
	for name, run := range map[string]func(*identityGovernanceService) *tidcommon.ServiceError{
		"suspend": func(svc *identityGovernanceService) *tidcommon.ServiceError {
			return svc.Suspend(s.ctx, "x", "")
		},
		"validate suspend": func(svc *identityGovernanceService) *tidcommon.ServiceError {
			return svc.ValidateSuspend(s.ctx, "x", "")
		},
		"unsuspend": func(svc *identityGovernanceService) *tidcommon.ServiceError {
			return svc.Unsuspend(s.ctx, "x")
		},
		"unlock": func(svc *identityGovernanceService) *tidcommon.ServiceError {
			return svc.Unlock(s.ctx, "x")
		},
	} {
		s.Run(name, func() {
			port := newEntityStateProviderMock(s.T())
			port.EXPECT().GetGovernedEntity(mock.Anything, "x", true).Return(nil, nil).Once()
			svc, published := auditedService(s.T(), port, nil)

			s.requireServiceError(run(svc), ErrorEntityNotFound)

			s.Require().Len(*published, 1)
			s.Equal(providers.StatusFailure, (*published)[0].Status)
			s.Equal("not_found", (*published)[0].Data[event.DataKey.Error])
		})
	}
}

// Under entity granularity a lost formation re-reads the entity entry and guards the retry on it; with
// no entry left there is nothing to lock.
func (s *ServiceTestSuite) TestFormationRetryUnderEntityGranularityReadsTheEntityEntry() {
	for name, tc := range map[string]struct {
		entry   *LockEntry
		retried bool
	}{
		"entry advanced": {&LockEntry{FailureCount: 6}, true},
		"entry cleared":  {nil, false},
	} {
		s.Run(name, func() {
			s.SetupTest()
			s.expectConfig(entityGranularityConfig())
			entity := s.activeEntity()
			s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Once()
			s.port.On("IncrementFailure", mock.Anything, entity.ID, AuthenticationMethodEntity, mock.Anything,
				mock.Anything).
				Return(LockEntry{FailureCount: 5, Revision: 3}, true, nil).Once()
			s.port.On("FormLock", mock.Anything, entity.ID, AuthenticationMethodEntity,
				LockEntry{FailureCount: 5, Revision: 3}, mock.Anything, mock.Anything).Return(false, nil).Once()
			refreshed := s.activeEntity()
			refreshed.Revision = 9
			refreshed.AccessState.Lock.Entity = tc.entry
			s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&refreshed, nil).Once()
			if tc.retried {
				s.port.On("FormLock", mock.Anything, entity.ID, AuthenticationMethodEntity,
					LockEntry{FailureCount: 6, Revision: 9}, mock.Anything, mock.Anything).Return(true, nil).Once()
			}

			s.Require().Nil(s.admittedFailure(s.svc.RecordFailure(s.ctx, entity.ID,
				providers.AccessPlaneAuthentication, testMethod)))
			s.port.AssertNumberOfCalls(s.T(), "FormLock", map[bool]int{true: 2, false: 1}[tc.retried])
		})
	}
}

// An identity deleted between the counted failure and the re-read has nothing left to lock.
func (s *ServiceTestSuite) TestFormationStopsWhenTheIdentityIsGone() {
	s.expectConfig(methodGranularityConfig())
	entity := s.activeEntity()
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Once()
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testMethod, mock.Anything, mock.Anything).
		Return(LockEntry{FailureCount: 5}, true, nil).Once()
	s.port.On("FormLock", mock.Anything, entity.ID, testMethod, mock.Anything, mock.Anything, mock.Anything).
		Return(false, nil).Once()
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(nil, nil).Once()

	s.Require().Nil(s.admittedFailure(s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
		testMethod)))
	s.port.AssertNumberOfCalls(s.T(), "FormLock", 1)
}

// Only a suspension holds an application, so its failure is admitted and not counted even with an
// entity-wide hold recorded.
func (s *ServiceTestSuite) TestAnApplicationFailureIsAdmittedWithoutCounting() {
	s.expectConfig(methodGranularityConfig())
	entity := s.activeEntity()
	entity.Category = providers.EntityCategoryApp
	entity.AccessState.Lock.Entity = &LockEntry{UnlockAt: permanentUnlockAt, Reason: lockReasonPostSuspension}
	s.expectRead(entity)

	decision, err := s.svc.RecordFailure(s.ctx, entity.ID, providers.AccessPlaneAuthentication, testMethod)

	s.Require().Nil(err)
	s.True(decision.Admitted)
	s.port.AssertNotCalled(s.T(), "IncrementFailure",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A live entry recorded under a method that never locks, as an older build could leave, holds nothing.
func (s *ServiceTestSuite) TestARecordedLockOnAMethodThatDoesNotLockIsIgnored() {
	s.expectConfig(methodGranularityConfig())
	entity := s.activeEntity()
	putLockEntry(&entity.AccessState, providers.AuthenticationMethodPasskey, LockEntry{LockCount: 1,
		UnlockAt: s.at(time.Hour)})
	s.expectRead(entity)

	decision, err := s.svc.EvaluateAccess(s.ctx, entity.ID, providers.AccessPlaneAuthentication,
		providers.AuthenticationMethodPasskey)

	s.Require().Nil(err)
	s.True(decision.Admitted)
}
