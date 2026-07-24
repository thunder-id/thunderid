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
	s.mockStore.On("IsTokenRevoked", mock.Anything, "jti-ok").Return(false, nil)
	s.mockStore.On("isCriterionRevoked", mock.Anything, criterionTypeTokenFamily, "tfid-x").
		Return(true, nil)
	err := s.enforcementService.EnsureNotRevoked(context.Background(), "jti-ok", "tfid-x")
	s.Assert().ErrorIs(err, ErrTokenRevoked)
}

// A token whose family is not revoked (and whose jti is clean) may proceed.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_TokenFamilyNotRevoked() {
	s.mockStore.On("IsTokenRevoked", mock.Anything, "jti-ok").Return(false, nil)
	s.mockStore.On("isCriterionRevoked", mock.Anything, criterionTypeTokenFamily, "tfid-x").
		Return(false, nil)
	err := s.enforcementService.EnsureNotRevoked(context.Background(), "jti-ok", "tfid-x")
	s.Assert().NoError(err)
}

// A criteria-store error fails closed.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_TokenFamilyLookupErrorFailsClosed() {
	s.mockStore.On("IsTokenRevoked", mock.Anything, "jti-ok").Return(false, nil)
	s.mockStore.On("isCriterionRevoked", mock.Anything, criterionTypeTokenFamily, "tfid-x").
		Return(false, errors.New("db down"))
	err := s.enforcementService.EnsureNotRevoked(context.Background(), "jti-ok", "tfid-x")
	s.Assert().ErrorIs(err, ErrEnforcementUnavailable)
}

// A family-only check (no jti) consults just the criteria store.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_TokenFamilyOnly() {
	s.mockStore.On("isCriterionRevoked", mock.Anything, criterionTypeTokenFamily, "tfid-x").
		Return(true, nil)
	err := s.enforcementService.EnsureNotRevoked(context.Background(), "", "tfid-x")
	s.Assert().ErrorIs(err, ErrTokenRevoked)
	s.mockStore.AssertNotCalled(s.T(), "IsTokenRevoked", mock.Anything, mock.Anything)
}

// An empty jti is a no-op — there is nothing to match against the deny list.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_EmptyJTI() {
	err := s.enforcementService.EnsureNotRevoked(context.Background(), "", "")
	s.Assert().NoError(err)
	s.mockStore.AssertNotCalled(s.T(), "IsTokenRevoked", mock.Anything, mock.Anything)
}

// A token absent from the deny list may proceed.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_NotRevoked() {
	s.mockStore.On("IsTokenRevoked", mock.Anything, "jti-1").Return(false, nil)
	err := s.enforcementService.EnsureNotRevoked(context.Background(), "jti-1", "")
	s.Assert().NoError(err)
}

// A token on the deny list is rejected with ErrTokenRevoked.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_Revoked() {
	s.mockStore.On("IsTokenRevoked", mock.Anything, "jti-2").Return(true, nil)
	err := s.enforcementService.EnsureNotRevoked(context.Background(), "jti-2", "")
	s.Assert().ErrorIs(err, ErrTokenRevoked)
}

// A deny-list read error fails closed with ErrEnforcementUnavailable.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_DBErrorFailsClosed() {
	s.mockStore.On("IsTokenRevoked", mock.Anything, "jti-3").Return(false, errors.New("db down"))
	err := s.enforcementService.EnsureNotRevoked(context.Background(), "jti-3", "")
	s.Assert().ErrorIs(err, ErrEnforcementUnavailable)
}

// Once the circuit trips, subsequent calls short-circuit without touching the store.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_OpenCircuitShortCircuits() {
	s.mockStore.On("IsTokenRevoked", mock.Anything, mock.Anything).Return(false, errors.New("db down"))

	// Drive consecutive failures up to the threshold to trip the circuit.
	for i := 0; i < enforcementFailureThreshold; i++ {
		err := s.enforcementService.EnsureNotRevoked(context.Background(), "jti-loop", "")
		s.Assert().ErrorIs(err, ErrEnforcementUnavailable)
	}
	callsAtTrip := len(s.mockStore.Calls)

	// Further calls while open must not hit the store.
	err := s.enforcementService.EnsureNotRevoked(context.Background(), "jti-loop", "")
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
	s.mockStore.On("IsTokenRevoked", mock.Anything, mock.Anything).Return(false, errors.New("db down"))

	// Drive failures up to the threshold (the trip) plus extra calls while open.
	for i := 0; i < enforcementFailureThreshold+3; i++ {
		err := c.EnsureNotRevoked(context.Background(), "jti-alert", "")
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
	s.mockStore.On("IsTokenRevoked", mock.Anything, mock.Anything).Return(false, errors.New("db down"))

	for i := 0; i < enforcementFailureThreshold; i++ {
		s.Assert().ErrorIs(c.EnsureNotRevoked(context.Background(), "jti-alert", ""), ErrEnforcementUnavailable)
	}

	obsMock.AssertNotCalled(s.T(), "PublishEvent", mock.Anything)
}

// After the cooldown a recovered store closes the circuit and tokens flow again.
func (s *EnforcementServiceTestSuite) TestEnsureNotRevoked_RecoversAfterCooldown() {
	s.mockStore.On("IsTokenRevoked", mock.Anything, "jti-recover").
		Return(false, errors.New("db down")).Times(enforcementFailureThreshold)
	for i := 0; i < enforcementFailureThreshold; i++ {
		_ = s.enforcementService.EnsureNotRevoked(context.Background(), "jti-recover", "")
	}

	// Simulate the cooldown elapsing, then let the store recover.
	s.enforcementService.breaker.openedAt = time.Now().Add(-2 * enforcementOpenDuration)
	s.mockStore.On("IsTokenRevoked", mock.Anything, "jti-recover").Return(false, nil)

	err := s.enforcementService.EnsureNotRevoked(context.Background(), "jti-recover", "")
	s.Assert().NoError(err)
	s.Assert().True(s.enforcementService.breaker.allow(), "circuit should be closed after a successful trial call")
}
