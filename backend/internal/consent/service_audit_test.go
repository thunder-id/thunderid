// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package consent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/transactionmock"
)

const (
	testAuditConsentID = "0196a1b2-0000-7000-8000-0000000000c1"
	testAuditUserID    = "0196a1b2-0000-7000-8000-0000000000d1"
	testAuditAppID     = "0196a1b2-0000-7000-8000-0000000000e1"
	testAuditElement   = "mobileNumber"
	testAuditAdminID   = "admin-1"
)

// publishedEvent is one audit event the consent service published.
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
	require.NoError(t, audit.Subscribe(registry, "consent-service-test", sink))
	return bus, func() []publishedEvent {
		stopper.Stop()
		sink.mu.Lock()
		defer sink.mu.Unlock()
		return sink.events
	}
}

// newDiscardingBus returns an event bus that accepts and drops every event, for a test the audit event
// does not matter to. A test of what the service publishes uses newRecordingBus.
func newDiscardingBus(t *testing.T) eventbus.EventBusInterface {
	bus := eventbusmock.NewEventBusInterfaceMock(t)
	bus.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Maybe()
	return bus
}

// signInConsentRequest is the request the consent enforcer sends when a user approves what an
// application asked for during sign-in.
func signInConsentRequest() *ConsentRequest {
	return &ConsentRequest{
		GroupID: testAuditAppID,
		Purposes: []ConsentPurposeItem{{
			Name: AttributePurposeName(testAuditAppID),
			Elements: []ConsentElementApproval{
				{Name: testAuditElement, Namespace: NamespaceAttribute, IsUserApproved: true},
			},
		}},
		Authorizations: []ConsentAuthorizationRequest{{
			UserID: testAuditUserID, Type: AuthorizationTypeAuthorization, Status: AuthorizationStatusApproved,
		}},
	}
}

type ConsentServiceAuditTestSuite struct {
	suite.Suite
	mockStore *consentStoreInterfaceMock
	service   *consentService
}

func TestConsentServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(ConsentServiceAuditTestSuite))
}

func (s *ConsentServiceAuditTestSuite) SetupTest() {
	s.reset()
}

func (s *ConsentServiceAuditTestSuite) SetupSubTest() {
	s.reset()
}

func (s *ConsentServiceAuditTestSuite) reset() {
	s.mockStore = newConsentStoreInterfaceMock(s.T())
	s.service = &consentService{
		consentStore:          s.mockStore,
		transactioner:         transaction.NewNoOpTransactioner(),
		inboundClientProvider: NewInboundClientProviderMock(s.T()),
		bus:                   newDiscardingBus(s.T()),
		logger:                log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ConsentService")),
	}
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt on the
// consent with consentID during sign-in, where the caller presented no verified identity yet. An
// attempt that succeeded has subject as its subject; a failed one has none.
func (s *ConsentServiceAuditTestSuite) requireOnlyEvent(events []publishedEvent, name string, action audit.Action,
	consentID string, subject audit.Ref, outcome audit.Outcome) {
	s.T().Helper()
	s.Require().Len(events, 1)
	event := events[0]
	s.Equal(name, event.name)
	s.Equal(audit.Ref{Type: audit.TargetTypeConsent, ID: consentID}, event.payload.Target())
	s.Equal(action, event.payload.Action())
	s.Equal(outcome, event.payload.Outcome())
	s.Equal(audit.Actor{Type: audit.ActorTypeAnonymous}, event.payload.Actor())
	s.Empty(event.payload.Details())
	if outcome.Result == audit.ResultFailed {
		s.Equal(audit.Ref{}, event.payload.Subject(), "a failed attempt may name no consent")
	} else {
		s.Equal(subject, event.payload.Subject())
	}
	recorded := fmt.Sprintf("%+v", event.payload)
	s.NotContains(recorded, testAuditElement, "no element of the consent is recorded")
	s.NotContains(recorded, AttributePurposeNamePrefix, "no purpose of the consent is recorded")
	s.NotContains(recorded, testAuditAppID, "the application the consent was given to is not recorded")
}

func (s *ConsentServiceAuditTestSuite) TestCreateConsent_PublishesTheConsentOfTheUser() {
	bus, published := newRecordingBus(s.T())
	s.service.bus = bus
	s.mockStore.EXPECT().CreateConsent(mock.Anything, mock.Anything).Return(nil).Once()

	created, svcErr := s.service.CreateConsent(context.Background(), signInConsentRequest())

	s.Require().Nil(svcErr)
	s.requireOnlyEvent(published(), audit.EventConsentCreate, audit.ActionCreate, created.ID,
		audit.Ref{Type: audit.TargetTypeUser, ID: testAuditUserID}, audit.OutcomeSucceeded)
}

func (s *ConsentServiceAuditTestSuite) TestCreateConsent_PublishesAFailure() {
	cases := []struct {
		name    string
		request func() *ConsentRequest
		arrange func()
		outcome audit.Outcome
	}{
		{
			name:    "the request is missing",
			request: func() *ConsentRequest { return nil },
			arrange: func() {},
			outcome: audit.OutcomeFailed("cns_1001"),
		},
		{
			name: "the request names no application",
			request: func() *ConsentRequest {
				request := signInConsentRequest()
				request.GroupID = ""
				return request
			},
			arrange: func() {},
			outcome: audit.OutcomeFailed("cns_1001"),
		},
		{
			name: "an element has an unknown namespace",
			request: func() *ConsentRequest {
				request := signInConsentRequest()
				request.Purposes[0].Elements[0].Namespace = Namespace("unknown")
				return request
			},
			arrange: func() {},
			outcome: audit.OutcomeFailed("cns_1007"),
		},
		{
			name: "the authorization has an unknown type",
			request: func() *ConsentRequest {
				request := signInConsentRequest()
				request.Authorizations[0].Type = ConsentAuthorizationType("unknown")
				return request
			},
			arrange: func() {},
			outcome: audit.OutcomeFailed("cns_1005"),
		},
		{
			name: "the authorization has an unknown status",
			request: func() *ConsentRequest {
				request := signInConsentRequest()
				request.Authorizations[0].Status = ConsentAuthorizationStatus("unknown")
				return request
			},
			arrange: func() {},
			outcome: audit.OutcomeFailed("cns_1006"),
		},
		{
			name: "the authorization names no user",
			request: func() *ConsentRequest {
				request := signInConsentRequest()
				request.Authorizations[0].UserID = ""
				return request
			},
			arrange: func() {},
			outcome: audit.OutcomeFailed("cns_1001"),
		},
		{
			name:    "the store fails",
			request: signInConsentRequest,
			arrange: func() {
				s.mockStore.EXPECT().CreateConsent(mock.Anything, mock.Anything).
					Return(errors.New("deadlock detected")).Once()
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			s.service.bus = bus
			tc.arrange()

			created, svcErr := s.service.CreateConsent(context.Background(), tc.request())

			s.Nil(created)
			s.Require().NotNil(svcErr)
			s.requireOnlyEvent(published(), audit.EventConsentCreate, audit.ActionCreate, "", audit.Ref{},
				tc.outcome)
		})
	}
}

func (s *ConsentServiceAuditTestSuite) TestUpdateConsent_PublishesOneEventPerAttempt() {
	user := audit.Ref{Type: audit.TargetTypeUser, ID: testAuditUserID}
	cases := []struct {
		name      string
		consentID string
		request   func() *ConsentRequest
		arrange   func()
		outcome   audit.Outcome
	}{
		{
			name:      "the consent is updated",
			consentID: testAuditConsentID,
			arrange: func() {
				s.mockStore.EXPECT().GetConsent(mock.Anything, testAuditConsentID).
					Return(&Consent{ID: testAuditConsentID, GroupID: testAuditAppID, Status: ConsentStatusActive}, nil).
					Once()
				s.mockStore.EXPECT().UpdateConsent(mock.Anything, mock.Anything).Return(nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name:      "the consent does not exist",
			consentID: testAuditConsentID,
			arrange: func() {
				s.mockStore.EXPECT().GetConsent(mock.Anything, testAuditConsentID).
					Return(nil, errConsentNotFound).Once()
			},
			outcome: audit.OutcomeFailed("cns_1003"),
		},
		{
			name:      "the request is missing",
			consentID: testAuditConsentID,
			request:   func() *ConsentRequest { return nil },
			arrange:   func() {},
			outcome:   audit.OutcomeFailed("cns_1001"),
		},
		{
			name:      "an element has an unknown namespace",
			consentID: testAuditConsentID,
			request: func() *ConsentRequest {
				request := signInConsentRequest()
				request.Purposes[0].Elements[0].Namespace = Namespace("unknown")
				return request
			},
			arrange: func() {},
			outcome: audit.OutcomeFailed("cns_1007"),
		},
		{
			name:      "the authorization has an unknown type",
			consentID: testAuditConsentID,
			request: func() *ConsentRequest {
				request := signInConsentRequest()
				request.Authorizations[0].Type = ConsentAuthorizationType("unknown")
				return request
			},
			arrange: func() {},
			outcome: audit.OutcomeFailed("cns_1005"),
		},
		{
			name:      "the authorization has an unknown status",
			consentID: testAuditConsentID,
			request: func() *ConsentRequest {
				request := signInConsentRequest()
				request.Authorizations[0].Status = ConsentAuthorizationStatus("unknown")
				return request
			},
			arrange: func() {},
			outcome: audit.OutcomeFailed("cns_1006"),
		},
		{
			name:      "the consent ID is missing",
			consentID: "",
			arrange:   func() {},
			outcome:   audit.OutcomeFailed("cns_1002"),
		},
		{
			name:      "the store fails",
			consentID: testAuditConsentID,
			arrange: func() {
				s.mockStore.EXPECT().GetConsent(mock.Anything, testAuditConsentID).
					Return(&Consent{ID: testAuditConsentID, GroupID: testAuditAppID, Status: ConsentStatusActive}, nil).
					Once()
				s.mockStore.EXPECT().UpdateConsent(mock.Anything, mock.Anything).
					Return(errors.New("deadlock detected")).Once()
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			s.service.bus = bus
			tc.arrange()

			request := signInConsentRequest()
			if tc.request != nil {
				request = tc.request()
			}

			_, svcErr := s.service.UpdateConsent(context.Background(), tc.consentID, request)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventConsentUpdate, audit.ActionUpdate, tc.consentID, user,
				tc.outcome)
		})
	}
}

// A consent is recorded from a runtime flow on the public path, which runs with the subject of the
// request it came in on, so a bearer token sent with the sign-in must not become the actor.
func (s *ConsentServiceAuditTestSuite) TestPublishesTheUserAsActorInARuntimeContext() {
	cases := []struct {
		name      string
		arrange   func()
		wantActor audit.Actor
		outcome   audit.Outcome
	}{
		{
			name: "the consent is created",
			arrange: func() {
				s.mockStore.EXPECT().CreateConsent(mock.Anything, mock.Anything).Return(nil).Once()
			},
			wantActor: audit.Actor{Type: audit.ActorTypeUser, ID: testAuditUserID},
			outcome:   audit.OutcomeSucceeded,
		},
		{
			name: "the store fails",
			arrange: func() {
				s.mockStore.EXPECT().CreateConsent(mock.Anything, mock.Anything).
					Return(errors.New("deadlock detected")).Once()
			},
			wantActor: audit.Actor{Type: audit.ActorTypeAnonymous},
			outcome:   audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			s.service.bus = bus
			tc.arrange()
			ctx := security.WithRuntimeContext(security.WithSecurityContextTest(context.Background(),
				security.NewSecurityContextForTest(testAuditAdminID, "ou-1", "", nil, nil)))

			_, _ = s.service.CreateConsent(ctx, signInConsentRequest())

			events := published()
			s.Require().Len(events, 1)
			s.Equal(tc.wantActor, events[0].payload.Actor())
			s.Equal(tc.outcome, events[0].payload.Outcome())
			s.NotEqual(testAuditAdminID, events[0].payload.Actor().ID)
		})
	}
}

// A consent update from a runtime flow is recorded with the user as actor in the same way.
func (s *ConsentServiceAuditTestSuite) TestUpdateConsent_PublishesTheUserAsActorInARuntimeContext() {
	bus, published := newRecordingBus(s.T())
	s.service.bus = bus
	s.mockStore.EXPECT().GetConsent(mock.Anything, testAuditConsentID).
		Return(&Consent{ID: testAuditConsentID, GroupID: testAuditAppID, Status: ConsentStatusActive}, nil).Once()
	s.mockStore.EXPECT().UpdateConsent(mock.Anything, mock.Anything).Return(nil).Once()
	ctx := security.WithRuntimeContext(security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditAdminID, "ou-1", "", nil, nil)))

	_, svcErr := s.service.UpdateConsent(ctx, testAuditConsentID, signInConsentRequest())

	s.Require().Nil(svcErr)
	events := published()
	s.Require().Len(events, 1)
	s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditUserID}, events[0].payload.Actor())
}

// Every change is published after its transaction returns, so a change that rolls back is never
// recorded as made.
func (s *ConsentServiceAuditTestSuite) TestEveryChangeIsPublishedAfterItsTransaction() {
	committed := false
	published := 0
	tx := transactionmock.NewTransactionerMock(s.T())
	tx.EXPECT().Transact(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, txFunc func(context.Context) error) error {
			committed = false
			err := txFunc(ctx)
			committed = true
			return err
		})
	bus := eventbusmock.NewEventBusInterfaceMock(s.T())
	bus.EXPECT().Publish(mock.Anything, mock.Anything).RunAndReturn(
		func(context.Context, *eventbus.Event) error {
			s.True(committed, "an event was published inside the transaction")
			published++
			return nil
		})
	s.service.transactioner = tx
	s.service.bus = bus
	s.mockStore.EXPECT().CreateConsent(mock.Anything, mock.Anything).Return(nil).Once()
	s.mockStore.EXPECT().GetConsent(mock.Anything, testAuditConsentID).
		Return(&Consent{ID: testAuditConsentID, GroupID: testAuditAppID, Status: ConsentStatusActive}, nil).Once()
	s.mockStore.EXPECT().UpdateConsent(mock.Anything, mock.Anything).Return(nil).Once()

	_, svcErr := s.service.CreateConsent(context.Background(), signInConsentRequest())
	s.Require().Nil(svcErr)
	_, svcErr = s.service.UpdateConsent(context.Background(), testAuditConsentID, signInConsentRequest())
	s.Require().Nil(svcErr)

	s.Equal(2, published)
}

// A bus that cannot take the event never fails the change it describes.
func (s *ConsentServiceAuditTestSuite) TestCreateConsent_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	s.service.bus = failing
	s.mockStore.EXPECT().CreateConsent(mock.Anything, mock.Anything).Return(nil).Once()

	created, svcErr := s.service.CreateConsent(context.Background(), signInConsentRequest())

	s.Nil(svcErr)
	s.NotEmpty(created.ID)
}
