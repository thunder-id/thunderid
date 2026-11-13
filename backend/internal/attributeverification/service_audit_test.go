// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributeverification

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/observabilityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const testAuditActorID = "admin-1"

// publishedEvent is one audit event the attribute verification service published.
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
	require.NoError(t, audit.Subscribe(registry, "attribute-verification-test", sink))
	return bus, func() []publishedEvent {
		stopper.Stop()
		sink.mu.Lock()
		defer sink.mu.Unlock()
		return sink.events
	}
}

// discardingBus is an event bus that drops every event, for a test the audit event does not matter to.
// A test of what the service publishes uses newRecordingBus.
type discardingBus struct{}

func (discardingBus) Publish(context.Context, *eventbus.Event) error {
	return nil
}

// ServiceAuditTestSuite checks the audit event each attempt to request or record a verification publishes.
type ServiceAuditTestSuite struct {
	suite.Suite
	entities *entitymock.EntityServiceInterfaceMock
	events   func() []publishedEvent
	obs      *observabilityprovidermock.ObservabilityProviderMock
	svc      *attributeVerificationService
	ctx      context.Context
}

func TestServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceAuditTestSuite))
}

func (s *ServiceAuditTestSuite) SetupTest() {
	s.reset(true)
}

func (s *ServiceAuditTestSuite) SetupSubTest() {
	s.reset(true)
}

// reset builds a service over a recording bus whose authorizer answers allowed.
func (s *ServiceAuditTestSuite) reset(allowed bool) {
	s.entities = entitymock.NewEntityServiceInterfaceMock(s.T())
	authorizer := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	authorizer.EXPECT().IsActionAllowed(mock.Anything, mock.Anything, mock.Anything).Return(allowed, nil).Maybe()
	s.obs = observabilityprovidermock.NewObservabilityProviderMock(s.T())
	s.obs.On("IsEnabled").Return(true).Maybe()
	s.obs.On("PublishEvent", mock.Anything, mock.Anything).Maybe()
	var bus eventbus.EventBusInterface
	bus, s.events = newRecordingBus(s.T())
	s.svc = newAttributeVerificationService(s.entities, contactSchema(s.T()), s.obs, authorizer, bus)
	s.ctx = security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditActorID, "ou-1", "", nil, nil))
}

// requireOnlyEvent checks that exactly one event was published for an attempt by the admin on user-1,
// with the given details, and that it never names a contact. A succeeded or denied attempt has the
// user as subject; a failed one has none.
func (s *ServiceAuditTestSuite) requireOnlyEvent(name string, action audit.Action, outcome audit.Outcome,
	details map[string]string) {
	s.T().Helper()
	events := s.events()
	s.Require().Len(events, 1)
	event := events[0]
	user := audit.Ref{Type: audit.TargetTypeUser, ID: "user-1"}
	s.Equal(name, event.name)
	s.Equal(user, event.payload.Target())
	s.Equal(action, event.payload.Action())
	s.Equal(outcome, event.payload.Outcome())
	s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditActorID}, event.payload.Actor())
	if len(details) == 0 {
		s.Empty(event.payload.Details())
	} else {
		s.Equal(details, event.payload.Details())
	}
	if outcome.Result == audit.ResultFailed {
		s.Equal(audit.Ref{}, event.payload.Subject())
	} else {
		s.Equal(user, event.payload.Subject())
	}
	s.NotContains(fmt.Sprintf("%+v", event.payload), testContactValue, "a contact is never recorded")
}

func (s *ServiceAuditTestSuite) TestPrepare_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{"the contact is pinned", func() {
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
		}, audit.OutcomeSucceeded},
		{"the caller may not update the user", func() {
			s.reset(false)
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
		}, audit.OutcomeDenied("forbidden")},
		{"the attribute is not a contact of the user", func() {
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(storedUser(`{}`), nil).Once()
		}, audit.OutcomeFailed("avs_1001")},
		{"the user is unknown", func() {
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(nil, entity.ErrEntityNotFound).Once()
		}, audit.OutcomeFailed("avs_1001")},
		{"the profile cannot be read", func() {
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(nil, errors.New("store error")).Once()
		}, audit.OutcomeFailed("sse_5000")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			tc.arrange()

			_, svcErr := s.svc.PrepareAttributeVerification(s.ctx, storedRequest("workEmail"))

			s.Equal(tc.outcome.Result == audit.ResultSucceeded, svcErr == nil)
			var details map[string]string
			if svcErr == nil {
				details = map[string]string{audit.DetailAttribute: "workEmail"}
			}
			s.requireOnlyEvent(audit.EventUserVerificationRequest, audit.ActionRequest, tc.outcome, details)
		})
	}
}

func (s *ServiceAuditTestSuite) TestPrepare_PendingCreationPublishesNoEvent() {
	pin, svcErr := s.svc.PrepareAttributeVerification(s.ctx, pendingRequest("person", "workEmail", testContactValue))
	_, refusedErr := s.svc.PrepareAttributeVerification(s.ctx, pendingRequest("person", "nickname", "x"))

	s.Require().Nil(svcErr)
	s.NotNil(pin)
	s.NotNil(refusedErr)
	s.Empty(s.events())
}

func (s *ServiceAuditTestSuite) TestRecord_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		pin     providers.AttributeVerificationPin
		outcome audit.Outcome
	}{
		{"the contact is recorded", func() {
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
			s.entities.On("RecordAttributeVerification", mock.Anything, contactRecord()).Return(true, nil).Once()
		}, contactPin(), audit.OutcomeSucceeded},
		{"the caller may not update the user", func() {
			s.reset(false)
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
		}, contactPin(), audit.OutcomeDenied("forbidden")},
		{"the contact changed since the pin", func() {
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").
				Return(storedUser(`{"workEmail":"other@example.com"}`), nil).Once()
		}, contactPin(), audit.OutcomeFailed("avs_1001")},
		{"the contact changed under the lock", func() {
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
			s.entities.On("RecordAttributeVerification", mock.Anything, contactRecord()).Return(false, nil).Once()
		}, contactPin(), audit.OutcomeFailed("avs_1001")},
		{"the write fails", func() {
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
			s.entities.On("RecordAttributeVerification", mock.Anything, contactRecord()).
				Return(false, errors.New("write failed")).Once()
		}, contactPin(), audit.OutcomeFailed("sse_5000")},
		{"the profile cannot be read", func() {
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(nil, errors.New("store error")).Once()
		}, contactPin(), audit.OutcomeFailed("sse_5000")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			tc.arrange()

			svcErr := s.svc.RecordAttributeVerification(s.ctx, tc.pin)

			s.Equal(tc.outcome.Result == audit.ResultSucceeded, svcErr == nil)
			s.requireOnlyEvent(audit.EventUserVerificationVerify, audit.ActionVerify, tc.outcome,
				map[string]string{audit.DetailAttribute: "workEmail"})
		})
	}
}

func (s *ServiceAuditTestSuite) TestRecord_KeepsThePublishedObservabilityEvents() {
	s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Twice()
	s.entities.On("RecordAttributeVerification", mock.Anything, contactRecord()).Return(true, nil).Once()

	_, prepareErr := s.svc.PrepareAttributeVerification(s.ctx, storedRequest("workEmail"))
	recordErr := s.svc.RecordAttributeVerification(s.ctx, contactPin())

	s.Nil(prepareErr)
	s.Nil(recordErr)
	s.obs.AssertNumberOfCalls(s.T(), "PublishEvent", 2)
	s.Len(s.events(), 2)
}

func (s *ServiceAuditTestSuite) TestAnAttemptInARuntimeContextNamesTheActorExplicitly() {
	signedIn := security.WithRuntimeContext(s.ctx)
	user := audit.Actor{Type: audit.ActorTypeUser, ID: "user-1"}
	anonymous := audit.Actor{Type: audit.ActorTypeAnonymous}
	cases := []struct {
		name    string
		ctx     context.Context
		arrange func()
		run     func(ctx context.Context) *tidcommon.ServiceError
		actor   audit.Actor
	}{
		{"a request is anonymous", signedIn, func() {
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
		}, func(ctx context.Context) *tidcommon.ServiceError {
			_, svcErr := s.svc.PrepareAttributeVerification(ctx, storedRequest("workEmail"))
			return svcErr
		}, anonymous},
		{"a verify that succeeded names the user", signedIn, func() {
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
			s.entities.On("RecordAttributeVerification", mock.Anything, contactRecord()).Return(true, nil).Once()
		}, func(ctx context.Context) *tidcommon.ServiceError {
			return s.svc.RecordAttributeVerification(ctx, contactPin())
		}, user},
		{"a verify that failed is anonymous", signedIn, func() {
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").
				Return(storedUser(`{"workEmail":"other@example.com"}`), nil).Once()
		}, func(ctx context.Context) *tidcommon.ServiceError {
			return s.svc.RecordAttributeVerification(ctx, contactPin())
		}, anonymous},
		{"a verify outside a runtime context names the caller", s.ctx, func() {
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
			s.entities.On("RecordAttributeVerification", mock.Anything, contactRecord()).Return(true, nil).Once()
		}, func(ctx context.Context) *tidcommon.ServiceError {
			return s.svc.RecordAttributeVerification(ctx, contactPin())
		}, audit.Actor{Type: audit.ActorTypeUser, ID: testAuditActorID}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			tc.arrange()

			tc.run(tc.ctx)

			events := s.events()
			s.Require().Len(events, 1)
			s.Equal(tc.actor, events[0].payload.Actor())
		})
	}
}

func (s *ServiceAuditTestSuite) TestPrepare_NeverRecordsATypedAttribute() {
	const typed = "alice@example.com"
	cases := []struct {
		name    string
		arrange func()
	}{
		{"the caller may not update the user", func() {
			s.reset(false)
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
		}},
		{"the attribute is not a contact of the user", func() {
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
		}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			tc.arrange()

			_, svcErr := s.svc.PrepareAttributeVerification(s.ctx, storedRequest(typed))

			s.NotNil(svcErr)
			events := s.events()
			s.Require().Len(events, 1)
			s.Empty(events[0].payload.Details())
			s.NotContains(fmt.Sprintf("%+v", events[0].payload), typed)
		})
	}
}

func (s *ServiceAuditTestSuite) TestPrepare_RecordsTheNameOfAResolvedAttribute() {
	s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()

	_, svcErr := s.svc.PrepareAttributeVerification(s.ctx, storedRequest("workEmail"))

	s.Nil(svcErr)
	events := s.events()
	s.Require().Len(events, 1)
	s.Equal(map[string]string{audit.DetailAttribute: "workEmail"}, events[0].payload.Details())
}

func (s *ServiceAuditTestSuite) TestRecord_RecordsThePinnedAttributeOfARefusedVerify() {
	s.reset(false)
	s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()

	svcErr := s.svc.RecordAttributeVerification(s.ctx, contactPin())

	s.NotNil(svcErr)
	events := s.events()
	s.Require().Len(events, 1)
	s.Equal(map[string]string{audit.DetailAttribute: "workEmail"}, events[0].payload.Details())
}

func (s *ServiceAuditTestSuite) TestAnAttemptWithoutACallerIsAnonymous() {
	s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()

	_, svcErr := s.svc.PrepareAttributeVerification(context.Background(), storedRequest("workEmail"))

	s.Nil(svcErr)
	events := s.events()
	s.Require().Len(events, 1)
	s.Equal(audit.Actor{Type: audit.ActorTypeAnonymous}, events[0].payload.Actor())
}

func (s *ServiceAuditTestSuite) TestAFailedPublishDoesNotFailTheAttempt() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Twice()
	s.svc.bus = failing
	s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Twice()
	s.entities.On("RecordAttributeVerification", mock.Anything, contactRecord()).Return(true, nil).Once()

	_, prepareErr := s.svc.PrepareAttributeVerification(s.ctx, storedRequest("workEmail"))
	recordErr := s.svc.RecordAttributeVerification(s.ctx, contactPin())

	s.Nil(prepareErr)
	s.Nil(recordErr)
}

func (s *ServiceAuditTestSuite) TestInitialize_RefusesANilEventBus() {
	service, err := Initialize(entitymock.NewEntityServiceInterfaceMock(s.T()), contactSchema(s.T()), nil,
		sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T()), nil)

	s.ErrorIs(err, errNilEventBus)
	s.Nil(service)
}
