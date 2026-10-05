// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/observability/observabilitymock"
)

type EnforcementServiceTestSuite struct {
	suite.Suite
	mockStore          *revocationStoreInterfaceMock
	enforcementService *enforcementService
}

func TestEnforcementServiceTestSuite(t *testing.T) {
	suite.Run(t, new(EnforcementServiceTestSuite))
}

func (s *EnforcementServiceTestSuite) SetupTest() {
	s.mockStore = newRevocationStoreInterfaceMock(s.T())
	s.enforcementService = &enforcementService{
		store:            s.mockStore,
		breaker:          newCircuitBreaker(enforcementFailureThreshold, enforcementOpenDuration),
		observabilitySvc: nil, // nil observability is tolerated; publish is a no-op.
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, "EnforcementService")),
	}
}

// A token whose family is revoked is rejected, even when its own jti is not on the deny list.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_TokenFamilyRevoked() {
	// The criteria deny list is consulted first, so a family revocation short-circuits before the
	// jti lookup. This ordering is what stops a revoked family being softened into a graced denial.
	s.mockStore.On("areCriteriaRevoked", mock.Anything,
		[]Criterion{{Type: CriterionTypeTokenFamily, Value: "tfid-x"}}, mock.Anything).
		Return(true, nil)
	err := s.enforcementService.EnsureNotRevoked(context.Background(),
		RevocationIdentity{JTI: "jti-ok", Criteria: []Criterion{{Type: CriterionTypeTokenFamily, Value: "tfid-x"}}})
	s.Assert().ErrorIs(err, ErrTokenRevoked)
}

// A token whose family is not revoked (and whose jti is clean) may proceed.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_TokenFamilyNotRevoked() {
	s.mockStore.On("IsTokenRevoked", mock.Anything, "jti-ok").Return(nil, nil)
	s.mockStore.On("areCriteriaRevoked", mock.Anything,
		[]Criterion{{Type: CriterionTypeTokenFamily, Value: "tfid-x"}}, mock.Anything).
		Return(false, nil)
	err := s.enforcementService.EnsureNotRevoked(context.Background(),
		RevocationIdentity{JTI: "jti-ok", Criteria: []Criterion{{Type: CriterionTypeTokenFamily, Value: "tfid-x"}}})
	s.Assert().NoError(err)
}

// A criteria-store error fails closed.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_TokenFamilyLookupErrorFailsClosed() {
	// Criteria are consulted first, so their failure fails closed before the jti lookup runs.
	s.mockStore.On("areCriteriaRevoked", mock.Anything,
		[]Criterion{{Type: CriterionTypeTokenFamily, Value: "tfid-x"}}, mock.Anything).
		Return(false, errors.New("db down"))
	err := s.enforcementService.EnsureNotRevoked(context.Background(),
		RevocationIdentity{JTI: "jti-ok", Criteria: []Criterion{{Type: CriterionTypeTokenFamily, Value: "tfid-x"}}})
	s.Assert().ErrorIs(err, ErrEnforcementUnavailable)
}

// A family-only check (no jti) consults just the criteria store.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_TokenFamilyOnly() {
	s.mockStore.On("areCriteriaRevoked", mock.Anything,
		[]Criterion{{Type: CriterionTypeTokenFamily, Value: "tfid-x"}}, mock.Anything).
		Return(true, nil)
	err := s.enforcementService.EnsureNotRevoked(context.Background(),
		RevocationIdentity{JTI: "", Criteria: []Criterion{{Type: CriterionTypeTokenFamily, Value: "tfid-x"}}})
	s.Assert().ErrorIs(err, ErrTokenRevoked)
	s.mockStore.AssertNotCalled(s.T(), "IsTokenRevoked", mock.Anything, mock.Anything)
}

// An empty jti is a no-op — there is nothing to match against the deny list.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_EmptyJTI() {
	err := s.enforcementService.EnsureNotRevoked(context.Background(),
		RevocationIdentity{JTI: "", Criteria: []Criterion{{Type: CriterionTypeTokenFamily, Value: ""}}})
	s.Assert().NoError(err)
	s.mockStore.AssertNotCalled(s.T(), "IsTokenRevoked", mock.Anything, mock.Anything)
	s.mockStore.AssertNotCalled(s.T(), "areCriteriaRevoked", mock.Anything, mock.Anything, mock.Anything)
}

// Every criterion is consulted in a single store call, so the number of dimensions an artifact
// carries does not multiply the cost of validating it.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_AllCriteriaInOneStoreCall() {
	criteria := []Criterion{
		{Type: CriterionTypeTokenFamily, Value: "tfid-x"},
		{Type: CriterionTypeSubject, Value: "user-x"},
	}
	s.mockStore.On("IsTokenRevoked", mock.Anything, "jti-ok").Return(nil, nil)
	s.mockStore.On("areCriteriaRevoked", mock.Anything, criteria, mock.Anything).Return(false, nil).Once()

	err := s.enforcementService.EnsureNotRevoked(context.Background(),
		RevocationIdentity{JTI: "jti-ok", Criteria: criteria})

	s.Assert().NoError(err)
	s.mockStore.AssertNumberOfCalls(s.T(), "areCriteriaRevoked", 1)
}

// Dimensions the artifact does not carry are dropped rather than queried as empty values.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_DropsEmptyCriteria() {
	s.mockStore.On("IsTokenRevoked", mock.Anything, "jti-ok").Return(nil, nil)
	s.mockStore.On("areCriteriaRevoked", mock.Anything,
		[]Criterion{{Type: CriterionTypeSubject, Value: "user-x"}}, mock.Anything).Return(false, nil)

	err := s.enforcementService.EnsureNotRevoked(context.Background(), RevocationIdentity{
		JTI: "jti-ok",
		Criteria: []Criterion{
			{Type: CriterionTypeTokenFamily, Value: ""},
			{Type: CriterionTypeSubject, Value: "user-x"},
		},
	})

	s.Assert().NoError(err)
}

// A token absent from the deny list may proceed.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_NotRevoked() {
	s.mockStore.On("IsTokenRevoked", mock.Anything, "jti-1").Return(nil, nil)
	err := s.enforcementService.EnsureNotRevoked(context.Background(),
		RevocationIdentity{JTI: "jti-1", Criteria: []Criterion{{Type: CriterionTypeTokenFamily, Value: ""}}})
	s.Assert().NoError(err)
}

// A token on the deny list is rejected with ErrTokenRevoked.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_Revoked() {
	s.mockStore.On("IsTokenRevoked", mock.Anything, "jti-2").Return(
		&revokedTokenEntry{RevokedAt: time.Now().UTC(), Reason: RevocationReasonExplicit}, nil)
	err := s.enforcementService.EnsureNotRevoked(context.Background(),
		RevocationIdentity{JTI: "jti-2", Criteria: []Criterion{{Type: CriterionTypeTokenFamily, Value: ""}}})
	s.Assert().ErrorIs(err, ErrTokenRevoked)
}

// A deny-list read error fails closed with ErrEnforcementUnavailable.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_DBErrorFailsClosed() {
	s.mockStore.On("IsTokenRevoked", mock.Anything, "jti-3").Return(nil, errors.New("db down"))
	err := s.enforcementService.EnsureNotRevoked(context.Background(),
		RevocationIdentity{JTI: "jti-3", Criteria: []Criterion{{Type: CriterionTypeTokenFamily, Value: ""}}})
	s.Assert().ErrorIs(err, ErrEnforcementUnavailable)
}

// Once the circuit trips, subsequent calls short-circuit without touching the store.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_OpenCircuitShortCircuits() {
	s.mockStore.On("IsTokenRevoked", mock.Anything, mock.Anything).Return(nil, errors.New("db down"))

	// Drive consecutive failures up to the threshold to trip the circuit.
	for i := 0; i < enforcementFailureThreshold; i++ {
		err := s.enforcementService.EnsureNotRevoked(context.Background(),
			RevocationIdentity{JTI: "jti-loop", Criteria: []Criterion{{Type: CriterionTypeTokenFamily, Value: ""}}})
		s.Assert().ErrorIs(err, ErrEnforcementUnavailable)
	}
	callsAtTrip := len(s.mockStore.Calls)

	// Further calls while open must not hit the store.
	err := s.enforcementService.EnsureNotRevoked(context.Background(),
		RevocationIdentity{JTI: "jti-loop", Criteria: []Criterion{{Type: CriterionTypeTokenFamily, Value: ""}}})
	s.Assert().ErrorIs(err, ErrEnforcementUnavailable)
	s.Assert().Equal(callsAtTrip, len(s.mockStore.Calls), "open circuit should not call the store")
}

// When the circuit trips, an RUNTIME_PERSISTENT_DB_UNAVAILABLE alert is published exactly once per trip —
// not once per failed request — so a sustained outage does not flood the observability pipeline.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_AlertsOncePerTrip() {
	obsMock := observabilitymock.NewObservabilityServiceInterfaceMock(s.T())
	obsMock.On("IsEnabled").Return(true)
	obsMock.On("PublishEvent", mock.Anything, mock.MatchedBy(func(evt *providers.Event) bool {
		return evt.Type == string(event.EventTypeRuntimePersistentDBUnavailable)
	})).Return()

	c := &enforcementService{
		store:            s.mockStore,
		breaker:          newCircuitBreaker(enforcementFailureThreshold, enforcementOpenDuration),
		observabilitySvc: obsMock,
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, "EnforcementService")),
	}
	s.mockStore.On("IsTokenRevoked", mock.Anything, mock.Anything).Return(nil, errors.New("db down"))

	// Drive failures up to the threshold (the trip) plus extra calls while open.
	for i := 0; i < enforcementFailureThreshold+3; i++ {
		err := c.EnsureNotRevoked(context.Background(),
			RevocationIdentity{JTI: "jti-alert", Criteria: []Criterion{{Type: CriterionTypeTokenFamily, Value: ""}}})
		s.Assert().ErrorIs(err, ErrEnforcementUnavailable)
	}

	obsMock.AssertNumberOfCalls(s.T(), "PublishEvent", 1)
}

// When observability is disabled the alert path is a no-op — the breaker still trips but no event
// is published.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_DisabledObservabilityDoesNotPublish() {
	obsMock := observabilitymock.NewObservabilityServiceInterfaceMock(s.T())
	obsMock.On("IsEnabled").Return(false)

	c := &enforcementService{
		store:            s.mockStore,
		breaker:          newCircuitBreaker(enforcementFailureThreshold, enforcementOpenDuration),
		observabilitySvc: obsMock,
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, "EnforcementService")),
	}
	s.mockStore.On("IsTokenRevoked", mock.Anything, mock.Anything).Return(nil, errors.New("db down"))

	for i := 0; i < enforcementFailureThreshold; i++ {
		err := c.EnsureNotRevoked(context.Background(), RevocationIdentity{
			JTI:      "jti-alert",
			Criteria: []Criterion{{Type: CriterionTypeTokenFamily, Value: ""}},
		})
		s.Assert().ErrorIs(err, ErrEnforcementUnavailable)
	}

	obsMock.AssertNotCalled(s.T(), "PublishEvent", mock.Anything)
}

// After the cooldown a recovered store closes the circuit and tokens flow again.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_RecoversAfterCooldown() {
	s.mockStore.On("IsTokenRevoked", mock.Anything, "jti-recover").
		Return(nil, errors.New("db down")).Times(enforcementFailureThreshold)
	for i := 0; i < enforcementFailureThreshold; i++ {
		_ = s.enforcementService.EnsureNotRevoked(context.Background(),
			RevocationIdentity{JTI: "jti-recover", Criteria: []Criterion{{Type: CriterionTypeTokenFamily, Value: ""}}})
	}

	// Simulate the cooldown elapsing, then let the store recover.
	s.enforcementService.breaker.openedAt = time.Now().Add(-2 * enforcementOpenDuration)
	s.mockStore.On("IsTokenRevoked", mock.Anything, "jti-recover").Return(nil, nil)

	err := s.enforcementService.EnsureNotRevoked(context.Background(),
		RevocationIdentity{JTI: "jti-recover", Criteria: []Criterion{{Type: CriterionTypeTokenFamily, Value: ""}}})
	s.Assert().NoError(err)
	s.Assert().True(s.enforcementService.breaker.allow(), "circuit should be closed after a successful trial call")
}

// ---------------------------------------------------------------------------
// Rotation reporting
// ---------------------------------------------------------------------------

// rotationService builds an enforcement service for the rotation-reporting tests.
func (s *EnforcementServiceTestSuite) rotationService() *enforcementService {
	return &enforcementService{
		store:   s.mockStore,
		breaker: newCircuitBreaker(enforcementFailureThreshold, enforcementOpenDuration),
		logger:  log.GetLogger().With(log.String(log.LoggerKeyComponentName, "EnforcementService")),
	}
}

// AC1.1: a token revoked by rotation is reported with the instant of its rotation, so the refresh
// grant can decide whether the presentation falls inside its application's window.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_RotatedTokenReportsRotationTime() {
	rotatedAt := time.Now().UTC().Add(-5 * time.Second)
	s.mockStore.On("IsTokenRevoked", mock.Anything, "jti-rotated").Return(
		&revokedTokenEntry{RevokedAt: rotatedAt, Reason: RevocationReasonRefreshRotation}, nil)

	err := s.rotationService().EnsureNotRevoked(context.Background(),
		RevocationIdentity{JTI: "jti-rotated"})

	var rotated *RotatedTokenError
	s.Require().ErrorAs(err, &rotated)
	s.Assert().Equal(rotatedAt, rotated.RotatedAt)
	// The typed error wraps ErrTokenRevoked so callers that do not distinguish it still reject.
	s.Assert().ErrorIs(err, ErrTokenRevoked)
}

// Rotation reporting is confined to rotation. A client cannot obtain a window by explicitly
// revoking its own refresh token through the RFC 7009 endpoint and then presenting it again.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_ExplicitRevocationIsNotRotation() {
	s.mockStore.On("IsTokenRevoked", mock.Anything, "jti-explicit").Return(
		&revokedTokenEntry{RevokedAt: time.Now().UTC(), Reason: RevocationReasonExplicit}, nil)

	err := s.rotationService().EnsureNotRevoked(context.Background(),
		RevocationIdentity{JTI: "jti-explicit"})

	s.Assert().ErrorIs(err, ErrTokenRevoked)
	var rotated *RotatedTokenError
	s.Assert().False(errors.As(err, &rotated), "explicit revocation must not report a rotation")
}

// A data fault denies outright rather than reporting a rotation an unreadable REVOKED_AT cannot
// anchor: the zero time would otherwise be handed to the grant handler as a window start.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_ZeroRevokedAtIsNotRotation() {
	s.mockStore.On("IsTokenRevoked", mock.Anything, "jti-corrupt").Return(
		&revokedTokenEntry{Reason: RevocationReasonRefreshRotation}, nil)

	err := s.rotationService().EnsureNotRevoked(context.Background(),
		RevocationIdentity{JTI: "jti-corrupt"})

	s.Assert().ErrorIs(err, ErrTokenRevoked)
	var rotated *RotatedTokenError
	s.Assert().False(errors.As(err, &rotated), "an unreadable rotation time must deny outright")
}

// AC4.2: a revoked token family outranks a rotation report. The criteria list is consulted first,
// so a compromise response can never be softened into a concurrency accommodation.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_FamilyRevocationOutranksRotation() {
	s.mockStore.On("areCriteriaRevoked", mock.Anything,
		[]Criterion{{Type: CriterionTypeTokenFamily, Value: "tfid-dead"}}, mock.Anything).
		Return(true, nil)

	err := s.rotationService().EnsureNotRevoked(context.Background(),
		RevocationIdentity{
			JTI:      "jti-rotated",
			Criteria: []Criterion{{Type: CriterionTypeTokenFamily, Value: "tfid-dead"}},
		})

	s.Assert().ErrorIs(err, ErrTokenRevoked)
	var rotated *RotatedTokenError
	s.Assert().False(errors.As(err, &rotated), "a revoked family must not report a rotation")
	// The jti lookup never runs; the mock asserts no unexpected call was made.
	s.mockStore.AssertNotCalled(s.T(), "IsTokenRevoked", mock.Anything, mock.Anything)
}

// AC4.5: rotation reporting never softens an unavailable deny list. Fail-closed still wins.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_RotationDoesNotSoftenFailClosed() {
	s.mockStore.On("IsTokenRevoked", mock.Anything, "jti-rotated").
		Return(nil, errors.New("db down"))

	err := s.rotationService().EnsureNotRevoked(context.Background(),
		RevocationIdentity{JTI: "jti-rotated"})

	s.Assert().ErrorIs(err, ErrEnforcementUnavailable)
	var rotated *RotatedTokenError
	s.Assert().False(errors.As(err, &rotated))
}

// AC2.1: the reported instant is REVOKED_AT, which the deny-list insert writes once. Repeated
// redemption reads it and never moves it, so every report carries the same anchor.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_RotationTimeIsStableAcrossReads() {
	rotatedAt := time.Now().UTC().Add(-29 * time.Second)
	s.mockStore.On("IsTokenRevoked", mock.Anything, "jti-rotated").Return(
		&revokedTokenEntry{RevokedAt: rotatedAt, Reason: RevocationReasonRefreshRotation}, nil)

	svc := s.rotationService()
	for i := 0; i < 3; i++ {
		err := svc.EnsureNotRevoked(context.Background(), RevocationIdentity{JTI: "jti-rotated"})
		var rotated *RotatedTokenError
		s.Require().ErrorAs(err, &rotated)
		s.Assert().Equal(rotatedAt, rotated.RotatedAt, "reading the anchor must never move it")
	}
}

// The typed error reports the rotation reason in its message and unwraps to both sentinels, so a
// caller logging it sees why the token was denied.
func (s *EnforcementServiceTestSuite) TestRotatedTokenError_MessageAndUnwrap() {
	err := &RotatedTokenError{RotatedAt: time.Now().UTC()}

	s.Assert().Equal(ErrTokenRotated.Error(), err.Error())
	s.Assert().ErrorIs(err, ErrTokenRotated)
	s.Assert().ErrorIs(err, ErrTokenRevoked)
}
