// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
)

const (
	// testAuditGatewayKey, testAuditGatewayName, testAuditGatewayURL and testAuditGatewayCertificate are
	// parts of a registration; no event may carry them.
	testAuditGatewayKey         = "gateway-management-key"
	testAuditGatewayName        = "audited-gateway"
	testAuditGatewayURL         = "https://audited.example.test"
	testAuditGatewayCertificate = "audited-gateway-ca-certificate"
)

// ServiceAuditTestSuite checks the audit event each change to a gateway registration publishes.
type ServiceAuditTestSuite struct {
	suite.Suite
	store *fakeStore
}

func TestServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceAuditTestSuite))
}

func (s *ServiceAuditTestSuite) SetupTest() {
	config.ResetServerRuntime()
	configureGatewayBound(shippedGatewayBound)
	cmodels.SetConfigCryptoProvider(reversingCrypto{})
	s.store = &fakeStore{}
}

func (s *ServiceAuditTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *ServiceAuditTestSuite) TearDownTest() {
	cmodels.SetConfigCryptoProvider(nil)
	config.ResetServerRuntime()
	configureGatewayBound(shippedGatewayBound)
}

// registered stores the gateway an update or a delete is aimed at.
func (s *ServiceAuditTestSuite) registered() {
	s.store.gateways = []Gateway{{ID: "gw-1", Name: testAuditGatewayName, BaseURL: testAuditGatewayURL,
		Key: "enc:" + testAuditGatewayKey}}
}

// requireOnlyEvent checks the one event the gateway service published; nothing of the registration is
// recorded.
func (s *ServiceAuditTestSuite) requireOnlyEvent(events []publishedEvent, name string, action audit.Action,
	gatewayID string, outcome audit.Outcome) {
	s.T().Helper()
	requireOnlyGatewayEvent(&s.Suite, events, name, action, gatewayID, outcome,
		testAuditGatewayKey, testAuditGatewayName, testAuditGatewayURL, testAuditGatewayCertificate)
}

func (s *ServiceAuditTestSuite) TestEachRegistrationAttemptPublishesOneEvent() {
	cases := []struct {
		name    string
		arrange func()
		request RegisterRequest
		outcome audit.Outcome
	}{
		{
			name: "the gateway is registered",
			request: RegisterRequest{Name: testAuditGatewayName, BaseURL: testAuditGatewayURL,
				Key: testAuditGatewayKey, CACertificate: testAuditGatewayCertificate},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name:    "the registration has no name",
			request: RegisterRequest{BaseURL: testAuditGatewayURL, Key: testAuditGatewayKey},
			outcome: audit.OutcomeFailed("gtw_1003"),
		},
		{
			name:    "the deployment administers as many gateways as it may",
			arrange: s.registered,
			request: RegisterRequest{Name: "second", BaseURL: "https://second.example.test"},
			outcome: audit.OutcomeFailed("gtw_1006"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			if tc.arrange != nil {
				tc.arrange()
			}
			bus, published := newRecordingBus(s.T())
			svc := newService(s.store, newPipeline(), bus)

			_, svcErr := svc.Register(adminContext(), tc.request)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			var gatewayID string
			if svcErr == nil {
				gatewayID = s.store.gateways[len(s.store.gateways)-1].ID
				s.NotEmpty(gatewayID)
			}
			s.requireOnlyEvent(published(), audit.EventGatewayCreate, audit.ActionCreate, gatewayID, tc.outcome)
		})
	}
}

func (s *ServiceAuditTestSuite) TestEachUpdateAttemptPublishesOneEvent() {
	key := testAuditGatewayKey
	name := "audited-renamed"
	certificate := testAuditGatewayCertificate
	cases := []struct {
		name      string
		gatewayID string
		request   UpdateRequest
		storeErr  error
		outcome   audit.Outcome
	}{
		{
			name: "the key is rotated", gatewayID: "gw-1",
			request: UpdateRequest{Key: &key, CACertificate: &certificate}, outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the gateway is declared in a file", gatewayID: "gw-1", request: UpdateRequest{Key: &key},
			storeErr: errDeclarativeGateway, outcome: audit.OutcomeFailed("gtw_1012"),
		},
		{
			name: "the gateway is not registered", gatewayID: "gw-missing", request: UpdateRequest{Name: &name},
			outcome: audit.OutcomeFailed("gtw_1002"),
		},
		{
			name: "the update changes nothing", gatewayID: "gw-1", request: UpdateRequest{},
			outcome: audit.OutcomeFailed("gtw_1011"),
		},
		{
			name: "the store fails", gatewayID: "gw-1", request: UpdateRequest{Key: &key},
			storeErr: errors.New("database is down"), outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.registered()
			s.store.err = tc.storeErr
			bus, published := newRecordingBus(s.T())
			svc := newService(s.store, newPipeline(), bus)

			_, svcErr := svc.Update(adminContext(), tc.gatewayID, tc.request)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventGatewayUpdate, audit.ActionUpdate, tc.gatewayID, tc.outcome)
		})
	}
}

// A delete of a gateway that is not registered answers not found, so it is recorded as failed.
func (s *ServiceAuditTestSuite) TestEachDeleteAttemptPublishesOneEvent() {
	cases := []struct {
		name      string
		gatewayID string
		storeErr  error
		outcome   audit.Outcome
	}{
		{name: "the gateway is removed", gatewayID: "gw-1", outcome: audit.OutcomeSucceeded},
		{name: "the gateway is not registered", gatewayID: "gw-missing", outcome: audit.OutcomeFailed("gtw_1002")},
		{name: "the gateway is declared in a file", gatewayID: "gw-1", storeErr: errDeclarativeGateway,
			outcome: audit.OutcomeFailed("gtw_1012")},
		{name: "the store fails", gatewayID: "gw-1", storeErr: errors.New("database is down"),
			outcome: audit.OutcomeFailed("sse_5000")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.registered()
			s.store.err = tc.storeErr
			bus, published := newRecordingBus(s.T())
			svc := newService(s.store, newPipeline(), bus)

			svcErr := svc.Delete(adminContext(), tc.gatewayID)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventGatewayDelete, audit.ActionDelete, tc.gatewayID, tc.outcome)
			if tc.outcome != audit.OutcomeSucceeded {
				s.Empty(s.store.deleted, "a refused delete reached the store")
			}
		})
	}
}

// A bus that cannot take the event never fails the change it describes.
func (s *ServiceAuditTestSuite) TestARefusingBusDoesNotFailTheRegistration() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	svc := newService(s.store, newPipeline(), failing)

	registration, svcErr := svc.Register(adminContext(), RegisterRequest{Name: testAuditGatewayName,
		BaseURL: testAuditGatewayURL})

	s.Nil(svcErr)
	s.NotEmpty(registration.ID)
}
