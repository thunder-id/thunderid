// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/connection/authzenpdpmock"
	"github.com/thunder-id/thunderid/tests/mocks/connection/httpconnectionmock"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	testAuditAdminID      = "admin-1"
	testAuditConnectionID = "http-1"
	testAuditPolicyID     = "p-1"
	testAuditOwnerOU      = "ou-owner"
	testAuditChildOU      = "ou-child"
	// testAuditConnectionName is part of the connection; no event may carry it.
	testAuditConnectionName = "Audited Endpoint"
)

// publishedEvent is one audit event the connection service published.
type publishedEvent struct {
	name    string
	payload audit.Payload
}

// recordingSink keeps every audit event it is handed, in order.
type recordingSink struct {
	mu     sync.Mutex
	events []publishedEvent
}

func (r *recordingSink) Handle(_ context.Context, env eventbus.Envelope, payload audit.Payload) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, publishedEvent{name: env.Name, payload: payload})
	return nil
}

// newRecordingBus returns an event bus with a sink subscribed the way the binaries subscribe theirs, so
// an event audit refuses never reaches it, and a function that stops the bus and returns the audit
// events written in order.
func newRecordingBus(t *testing.T) (eventbus.EventBusInterface, func() []publishedEvent) {
	bus, registry, stopper := eventbus.Initialize()
	t.Cleanup(stopper.Stop)

	sink := &recordingSink{}
	require.NoError(t, audit.Subscribe(registry, "connection-service-test", sink))
	return bus, func() []publishedEvent {
		stopper.Stop()
		sink.mu.Lock()
		defer sink.mu.Unlock()
		return sink.events
	}
}

// adminContext is the context of a management call made by the admin.
func adminContext() context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditAdminID, testAuditOwnerOU, "", nil, nil))
}

// SharingAuditTestSuite checks the audit event each change to a sharing policy of a connection publishes.
type SharingAuditTestSuite struct {
	suite.Suite
	mockHTTP    *httpconnectionmock.HTTPConnectionServiceInterfaceMock
	mockIDP     *idpmock.IDPServiceInterfaceMock
	mockPDP     *authzenpdpmock.AuthZENPDPServiceInterfaceMock
	mockNotif   *notificationmock.NotificationSenderMgtSvcInterfaceMock
	mockSharing *sharingmock.SharingServiceInterfaceMock
	published   func() []publishedEvent
	svc         *service
}

func TestSharingAuditTestSuite(t *testing.T) {
	suite.Run(t, new(SharingAuditTestSuite))
}

// SetupTest wires an HTTP connection owned by ou-owner, and a caller who may change connections of
// ou-owner only.
func (s *SharingAuditTestSuite) SetupTest() {
	initConfigWithTestCryptoKey(s.T())
	s.mockHTTP = httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	s.mockHTTP.On("GetHTTPConnection", mock.Anything, testAuditConnectionID).Return(
		&httpconnection.HTTPConnection{ID: testAuditConnectionID, Name: testAuditConnectionName,
			OUID: testAuditOwnerOU}, (*tidcommon.ServiceError)(nil)).Maybe()
	s.mockHTTP.On("GetHTTPConnection", mock.Anything, "missing").
		Return(nil, &httpconnection.ErrorNotFound).Maybe()
	s.mockSharing = sharingmock.NewSharingServiceInterfaceMock(s.T())

	var bus eventbus.EventBusInterface
	bus, s.published = newRecordingBus(s.T())
	s.mockNotif = notificationmock.NewNotificationSenderMgtSvcInterfaceMock(s.T())
	s.mockIDP = idpmock.NewIDPServiceInterfaceMock(s.T())
	s.mockPDP = authzenpdpmock.NewAuthZENPDPServiceInterfaceMock(s.T())
	s.svc = newService(s.mockIDP, s.mockNotif, &testResourceServerLister{}, s.mockPDP, s.mockHTTP, bus)
	s.svc.sharingService = s.mockSharing
	authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	ownerOnly := func(actionCtx *sysauthz.ActionContext) bool {
		return actionCtx == nil || actionCtx.OUID == "" || actionCtx.OUID == testAuditOwnerOU
	}
	authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.MatchedBy(ownerOnly)).Return(true, nil).Maybe()
	authz.On("IsActionAllowed", mock.Anything, mock.Anything,
		mock.MatchedBy(func(actionCtx *sysauthz.ActionContext) bool { return !ownerOnly(actionCtx) })).
		Return(false, nil).Maybe()
	s.svc.authzService = authz
}

func (s *SharingAuditTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *SharingAuditTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// connectionPolicy is a policy of the connection made by the initiating organization unit.
func connectionPolicy(initiating string) sharing.Policy {
	return sharing.Policy{ID: testAuditPolicyID, ResourceType: ConnectionSharingType,
		ResourceID: testAuditConnectionID, InitiatingOUID: initiating}
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt by the admin on
// the policy with policyID of the connection. A succeeded or denied attempt has the connection as subject;
// a failed one has none. Nothing of the policy or the connection is recorded.
func (s *SharingAuditTestSuite) requireOnlyEvent(name string, action audit.Action, policyID string,
	outcome audit.Outcome) {
	s.T().Helper()
	events := s.published()
	s.Require().Len(events, 1)
	event := events[0]
	s.Equal(name, event.name)
	s.Equal(audit.Ref{Type: audit.TargetTypeConnectionSharingPolicy, ID: policyID}, event.payload.Target())
	s.Equal(action, event.payload.Action())
	s.Equal(outcome, event.payload.Outcome())
	s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	s.Empty(event.payload.Details())
	if outcome.Result == audit.ResultFailed {
		s.Equal(audit.Ref{}, event.payload.Subject())
	} else {
		s.Equal(audit.Ref{Type: audit.TargetTypeConnection, ID: testAuditConnectionID}, event.payload.Subject())
	}
	recorded := fmt.Sprintf("%+v", event.payload)
	for _, value := range []string{testAuditOwnerOU, testAuditChildOU, testAuditConnectionName} {
		s.NotContains(recorded, value, "nothing of the policy or the connection is recorded")
	}
}

func (s *SharingAuditTestSuite) TestCreateSharingPolicy_PublishesOneEventPerAttempt() {
	cases := []struct {
		name         string
		connectionID string
		request      sharing.PolicyRequest
		arrange      func()
		policyID     string
		outcome      audit.Outcome
	}{
		{
			name: "the policy is created", connectionID: testAuditConnectionID,
			arrange: func() {
				s.mockSharing.On("CreatePolicy", mock.Anything, ConnectionSharingType, testAuditConnectionID,
					testAuditOwnerOU, mock.Anything).
					Return(connectionPolicy(testAuditOwnerOU), (*tidcommon.ServiceError)(nil)).Once()
			},
			policyID: testAuditPolicyID, outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the caller may not act for the initiating unit", connectionID: testAuditConnectionID,
			request: sharing.PolicyRequest{InitiatingOUID: testAuditChildOU},
			arrange: func() {}, outcome: audit.OutcomeDenied("forbidden"),
		},
		{
			name: "the framework refuses the policy", connectionID: testAuditConnectionID,
			arrange: func() {
				s.mockSharing.On("CreatePolicy", mock.Anything, ConnectionSharingType, testAuditConnectionID,
					testAuditOwnerOU, mock.Anything).Return(sharing.Policy{}, &sharing.ErrorPolicyExists).Once()
			},
			outcome: audit.OutcomeFailed("con_1033"),
		},
		{
			name: "the connection does not exist", connectionID: "missing",
			arrange: func() {}, outcome: audit.OutcomeFailed("con_1015"),
		},
		{
			// The connection is looked up before the caller is checked, so a connection that is not there is a
			// failed attempt whoever the initiating unit is, and records no subject.
			name:         "the connection does not exist and the caller may not act for the initiating unit",
			connectionID: "missing", request: sharing.PolicyRequest{InitiatingOUID: testAuditChildOU},
			arrange: func() {}, outcome: audit.OutcomeFailed("con_1015"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			tc.arrange()

			_, svcErr := s.svc.createSharingPolicy(adminContext(), s.svc.httpOwner, tc.connectionID, tc.request)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(audit.EventConnectionSharingPolicyCreate, audit.ActionCreate, tc.policyID,
				tc.outcome)
		})
	}
}

func (s *SharingAuditTestSuite) TestUpdateSharingPolicy_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		policy  sharing.Policy
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the policy is updated", policy: connectionPolicy(testAuditOwnerOU),
			arrange: func() {
				s.mockSharing.On("UpdatePolicy", mock.Anything, testAuditPolicyID, mock.Anything).
					Return(connectionPolicy(testAuditOwnerOU), (*tidcommon.ServiceError)(nil)).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{name: "another unit's policy", policy: connectionPolicy(testAuditChildOU), arrange: func() {},
			outcome: audit.OutcomeDenied("forbidden")},
		{
			name: "the policy belongs to another connection",
			policy: sharing.Policy{ID: testAuditPolicyID, ResourceType: ConnectionSharingType, ResourceID: "http-2",
				InitiatingOUID: testAuditOwnerOU},
			arrange: func() {}, outcome: audit.OutcomeFailed("con_1032"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.mockSharing.On("GetPolicy", mock.Anything, testAuditPolicyID).
				Return(tc.policy, (*tidcommon.ServiceError)(nil)).Once()
			tc.arrange()

			_, svcErr := s.svc.updateSharingPolicy(adminContext(), s.svc.httpOwner, testAuditConnectionID,
				testAuditPolicyID, sharing.PolicyRequest{Version: 1})

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(audit.EventConnectionSharingPolicyUpdate, audit.ActionUpdate, testAuditPolicyID,
				tc.outcome)
		})
	}
}

func (s *SharingAuditTestSuite) TestDeleteSharingPolicy_PublishesOneEventPerAttempt() {
	declared := connectionPolicy(testAuditOwnerOU)
	declared.Declared = true
	cases := []struct {
		name    string
		policy  sharing.Policy
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the policy is deleted", policy: connectionPolicy(testAuditOwnerOU),
			arrange: func() {
				s.mockSharing.On("DeletePolicy", mock.Anything, testAuditPolicyID).
					Return((*tidcommon.ServiceError)(nil)).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{name: "another unit's policy", policy: connectionPolicy(testAuditChildOU), arrange: func() {},
			outcome: audit.OutcomeDenied("forbidden")},
		{name: "a declared policy", policy: declared, arrange: func() {}, outcome: audit.OutcomeFailed("con_1035")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.mockSharing.On("GetPolicy", mock.Anything, testAuditPolicyID).
				Return(tc.policy, (*tidcommon.ServiceError)(nil)).Once()
			tc.arrange()

			svcErr := s.svc.deleteSharingPolicy(adminContext(), s.svc.httpOwner, testAuditConnectionID,
				testAuditPolicyID)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(audit.EventConnectionSharingPolicyDelete, audit.ActionDelete, testAuditPolicyID,
				tc.outcome)
		})
	}
}

// A delete stays idempotent, but one that finds no policy of the connection records nothing, so a caller
// cannot write a succeeded delete for any ID it chooses.
func (s *SharingAuditTestSuite) TestDeleteSharingPolicy_PublishesNothingForAMissingPolicy() {
	cases := []struct {
		name   string
		policy sharing.Policy
		err    *tidcommon.ServiceError
	}{
		{name: "no policy has the ID", err: &sharing.ErrorPolicyNotFound},
		{name: "the policy belongs to another connection", policy: sharing.Policy{ID: testAuditPolicyID,
			ResourceType: ConnectionSharingType, ResourceID: "http-2", InitiatingOUID: testAuditOwnerOU}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.mockSharing.On("GetPolicy", mock.Anything, testAuditPolicyID).Return(tc.policy, tc.err).Once()

			svcErr := s.svc.deleteSharingPolicy(adminContext(), s.svc.httpOwner, testAuditConnectionID,
				testAuditPolicyID)

			s.Nil(svcErr)
			s.Empty(s.published())
		})
	}
}

// A connection that is not there fails the attempt before any policy is read.
func (s *SharingAuditTestSuite) TestUpdateAndDeleteSharingPolicy_FailWhenTheConnectionDoesNotExist() {
	s.Run("update", func() {
		_, svcErr := s.svc.updateSharingPolicy(adminContext(), s.svc.httpOwner, "missing", testAuditPolicyID,
			sharing.PolicyRequest{Version: 1})

		s.Require().NotNil(svcErr)
		s.requireOnlyEvent(audit.EventConnectionSharingPolicyUpdate, audit.ActionUpdate, testAuditPolicyID,
			audit.OutcomeFailed("con_1015"))
	})
	s.Run("delete", func() {
		svcErr := s.svc.deleteSharingPolicy(adminContext(), s.svc.httpOwner, "missing", testAuditPolicyID)

		s.Require().NotNil(svcErr)
		s.requireOnlyEvent(audit.EventConnectionSharingPolicyDelete, audit.ActionDelete, testAuditPolicyID,
			audit.OutcomeFailed("con_1015"))
	})
}

// A notification sender is a connection to its owner, but its own audit events name it as a notification
// sender, so a change to its sharing policy does too.
func (s *SharingAuditTestSuite) TestCreateSharingPolicy_NamesANotificationSenderAsOne() {
	s.mockNotif.On("GetSender", mock.Anything, "sm-1").Return(&ncommon.NotificationSenderDTO{ID: "sm-1",
		OUID: testAuditOwnerOU, Type: ncommon.NotificationSenderTypeEmail,
		Provider: ncommon.NotificationProviderTypeSMTP}, (*tidcommon.ServiceError)(nil)).Once()
	s.mockSharing.On("CreatePolicy", mock.Anything, ConnectionSharingType, "sm-1", testAuditOwnerOU, mock.Anything).
		Return(connectionPolicy(testAuditOwnerOU), (*tidcommon.ServiceError)(nil)).Once()

	_, svcErr := s.svc.createSharingPolicy(adminContext(), s.svc.senderOwner(ncommon.NotificationSenderTypeEmail,
		ncommon.NotificationProviderTypeSMTP), "sm-1", sharing.PolicyRequest{})

	s.Require().Nil(svcErr)
	events := s.published()
	s.Require().Len(events, 1)
	s.Equal(audit.Ref{Type: audit.TargetTypeConnectionSharingPolicy, ID: testAuditPolicyID},
		events[0].payload.Target())
	s.Equal(audit.Ref{Type: audit.TargetTypeNotificationSender, ID: "sm-1"}, events[0].payload.Subject())
	s.Equal(audit.OutcomeSucceeded, events[0].payload.Outcome())
}

// An identity provider and an AuthZEN PDP are connections to their own audit events too, so a change to the
// sharing policy of either names the connection as the subject.
func (s *SharingAuditTestSuite) TestCreateSharingPolicy_NamesIDPAndPDPOwnersAsConnections() {
	s.mockIDP.On("GetIdentityProvider", mock.Anything, "idp-1").Return(&providers.IDPDTO{ID: "idp-1",
		Type: providers.IDPTypeGoogle, OUID: testAuditOwnerOU}, (*tidcommon.ServiceError)(nil)).Once()
	s.mockPDP.On("GetAuthZENPDP", mock.Anything, "pdp-1").Return(&authzenpdp.AuthZENPDPConnection{ID: "pdp-1",
		OUID: testAuditOwnerOU}, (*tidcommon.ServiceError)(nil)).Once()
	cases := []struct {
		name         string
		connectionID string
		owner        ownerLookup
	}{
		{name: "an identity provider", connectionID: "idp-1", owner: s.svc.idpOwner(providers.IDPTypeGoogle)},
		{name: "an AuthZEN PDP", connectionID: "pdp-1", owner: s.svc.authZENPDPOwner},
	}
	for _, tc := range cases {
		s.mockSharing.On("CreatePolicy", mock.Anything, ConnectionSharingType, tc.connectionID, testAuditOwnerOU,
			mock.Anything).Return(connectionPolicy(testAuditOwnerOU), (*tidcommon.ServiceError)(nil)).Once()

		_, svcErr := s.svc.createSharingPolicy(adminContext(), tc.owner, tc.connectionID, sharing.PolicyRequest{})

		s.Require().Nil(svcErr, tc.name)
	}
	events := s.published()
	s.Require().Len(events, 2)
	for i, tc := range cases {
		s.Equal(audit.Ref{Type: audit.TargetTypeConnectionSharingPolicy, ID: testAuditPolicyID},
			events[i].payload.Target(), tc.name)
		s.Equal(audit.Ref{Type: audit.TargetTypeConnection, ID: tc.connectionID}, events[i].payload.Subject(),
			tc.name)
		s.Equal(audit.OutcomeSucceeded, events[i].payload.Outcome(), tc.name)
	}
}

// A bus that cannot take the event never fails the change it describes.
func (s *SharingAuditTestSuite) TestCreateSharingPolicy_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	s.svc.bus = failing
	s.mockSharing.On("CreatePolicy", mock.Anything, ConnectionSharingType, testAuditConnectionID, testAuditOwnerOU,
		mock.Anything).Return(connectionPolicy(testAuditOwnerOU), (*tidcommon.ServiceError)(nil)).Once()

	policy, svcErr := s.svc.createSharingPolicy(adminContext(), s.svc.httpOwner, testAuditConnectionID,
		sharing.PolicyRequest{})

	s.Nil(svcErr)
	s.Equal(testAuditPolicyID, policy.ID)
}
