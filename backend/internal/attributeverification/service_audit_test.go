// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributeverification

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit/audittest"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/observabilityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const testAuditActorID = "admin-1"

// ServiceAuditTestSuite checks the audit event each attempt to request or record a verification publishes.
type ServiceAuditTestSuite struct {
	suite.Suite
	entities   *entitymock.EntityServiceInterfaceMock
	authorizer *sysauthzmock.SystemAuthorizationServiceInterfaceMock
	events     func() []audittest.Event
	obs        *observabilityprovidermock.ObservabilityProviderMock
	svc        *attributeVerificationService
	ctx        context.Context
}

func TestServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceAuditTestSuite))
}

func (s *ServiceAuditTestSuite) SetupTest() {
	s.reset()
}

func (s *ServiceAuditTestSuite) SetupSubTest() {
	s.reset()
}

// reset builds a service over a recording bus. A test expects the authorizer and observability calls its
// path makes with authorizes and observes, so a call that is skipped or added fails it.
func (s *ServiceAuditTestSuite) reset() {
	s.entities = entitymock.NewEntityServiceInterfaceMock(s.T())
	s.authorizer = sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	s.obs = observabilityprovidermock.NewObservabilityProviderMock(s.T())
	var bus eventbus.EventBusInterface
	bus, s.events = audittest.NewRecordingBus(s.T(), "attribute-verification-test")
	s.svc = newAttributeVerificationService(s.entities, contactSchema(s.T()), s.obs, s.authorizer, bus)
	s.ctx = security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditActorID, "ou-1", "", nil, nil))
}

// authorizes expects the authorizer to be asked times, and to answer allowed.
func (s *ServiceAuditTestSuite) authorizes(allowed bool, times int) {
	s.authorizer.EXPECT().IsActionAllowed(mock.Anything, mock.Anything, mock.Anything).
		Return(allowed, nil).Times(times)
}

// observes expects times observability events to be published.
func (s *ServiceAuditTestSuite) observes(times int) {
	s.obs.On("IsEnabled").Return(true).Times(times)
	s.obs.On("PublishEvent", mock.Anything, mock.Anything).Times(times)
}

// named is the details of an event that names the attribute.
func named(attribute string) map[string]string {
	return map[string]string{audit.DetailAttribute: attribute}
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
	s.Equal(name, event.Name)
	s.Equal(user, event.Payload.Target())
	s.Equal(action, event.Payload.Action())
	s.Equal(outcome, event.Payload.Outcome())
	s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditActorID}, event.Payload.Actor())
	if len(details) == 0 {
		s.Empty(event.Payload.Details())
	} else {
		s.Equal(details, event.Payload.Details())
	}
	if outcome.Result == audit.ResultFailed {
		s.Equal(audit.Ref{}, event.Payload.Subject())
	} else {
		s.Equal(user, event.Payload.Subject())
	}
	s.NotContains(fmt.Sprintf("%+v", event.Payload), testContactValue, "a contact is never recorded")
}

func (s *ServiceAuditTestSuite) TestPrepare_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{"the contact is pinned", func() {
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
			s.authorizes(true, 1)
			s.observes(1)
		}, audit.OutcomeSucceeded},
		{"the caller may not update the user", func() {
			s.authorizes(false, 1)
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
		}, audit.OutcomeDenied("forbidden")},
		{"the attribute is not a contact of the user", func() {
			s.authorizes(true, 1)
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
		details map[string]string
	}{
		{"the contact is recorded", func() {
			s.authorizes(true, 1)
			s.observes(1)
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
			s.entities.On("RecordAttributeVerification", mock.Anything, contactRecord()).Return(true, nil).Once()
		}, contactPin(), audit.OutcomeSucceeded, named("workEmail")},
		{"the caller may not update the user", func() {
			s.authorizes(false, 1)
			s.observes(1)
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
		}, contactPin(), audit.OutcomeDenied("forbidden"), nil},
		{"the pinned attribute is not a contact of the user", func() {
			s.authorizes(true, 1)
			s.observes(1)
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(storedUser(`{}`), nil).Once()
		}, contactPin(), audit.OutcomeFailed("avs_1001"), nil},
		{"the contact changed since the pin", func() {
			s.authorizes(true, 1)
			s.observes(1)
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").
				Return(storedUser(`{"workEmail":"other@example.com"}`), nil).Once()
		}, contactPin(), audit.OutcomeFailed("avs_1001"), named("workEmail")},
		{"the contact changed under the lock", func() {
			s.authorizes(true, 1)
			s.observes(1)
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
			s.entities.On("RecordAttributeVerification", mock.Anything, contactRecord()).Return(false, nil).Once()
		}, contactPin(), audit.OutcomeFailed("avs_1001"), named("workEmail")},
		{"the write fails", func() {
			s.authorizes(true, 1)
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
			s.entities.On("RecordAttributeVerification", mock.Anything, contactRecord()).
				Return(false, errors.New("write failed")).Once()
		}, contactPin(), audit.OutcomeFailed("sse_5000"), named("workEmail")},
		{"the profile cannot be read", func() {
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(nil, errors.New("store error")).Once()
		}, contactPin(), audit.OutcomeFailed("sse_5000"), nil},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			tc.arrange()

			svcErr := s.svc.RecordAttributeVerification(s.ctx, tc.pin)

			s.Equal(tc.outcome.Result == audit.ResultSucceeded, svcErr == nil)
			s.requireOnlyEvent(audit.EventUserVerificationVerify, audit.ActionVerify, tc.outcome, tc.details)
		})
	}
}

func (s *ServiceAuditTestSuite) TestRecord_KeepsThePublishedObservabilityEvents() {
	s.authorizes(true, 2)
	s.observes(2)
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
			s.authorizes(true, 1)
			s.observes(1)
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
		}, func(ctx context.Context) *tidcommon.ServiceError {
			_, svcErr := s.svc.PrepareAttributeVerification(ctx, storedRequest("workEmail"))
			return svcErr
		}, anonymous},
		{"a verify that succeeded names the user", signedIn, func() {
			s.authorizes(true, 1)
			s.observes(1)
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
			s.entities.On("RecordAttributeVerification", mock.Anything, contactRecord()).Return(true, nil).Once()
		}, func(ctx context.Context) *tidcommon.ServiceError {
			return s.svc.RecordAttributeVerification(ctx, contactPin())
		}, user},
		{"a verify that failed is anonymous", signedIn, func() {
			s.authorizes(true, 1)
			s.observes(1)
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").
				Return(storedUser(`{"workEmail":"other@example.com"}`), nil).Once()
		}, func(ctx context.Context) *tidcommon.ServiceError {
			return s.svc.RecordAttributeVerification(ctx, contactPin())
		}, anonymous},
		{"a verify outside a runtime context names the caller", s.ctx, func() {
			s.authorizes(true, 1)
			s.observes(1)
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
			s.Equal(tc.actor, events[0].Payload.Actor())
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
			s.authorizes(false, 1)
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
		}},
		{"the attribute is not a contact of the user", func() {
			s.authorizes(true, 1)
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
			s.Empty(events[0].Payload.Details())
			s.NotContains(fmt.Sprintf("%+v", events[0].Payload), typed)
		})
	}
}

func (s *ServiceAuditTestSuite) TestPrepare_RecordsTheNameOfAResolvedAttribute() {
	s.authorizes(true, 1)
	s.observes(1)
	s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()

	_, svcErr := s.svc.PrepareAttributeVerification(s.ctx, storedRequest("workEmail"))

	s.Nil(svcErr)
	events := s.events()
	s.Require().Len(events, 1)
	s.Equal(map[string]string{audit.DetailAttribute: "workEmail"}, events[0].Payload.Details())
}

func (s *ServiceAuditTestSuite) TestRecord_NeverRecordsAPinnedAttributeThatDidNotResolve() {
	cases := []struct {
		name    string
		pin     providers.AttributeVerificationPin
		allowed bool
	}{
		{"a pin the caller may not record", contactPin(), false},
		{"a pin of a typed attribute", providers.AttributeVerificationPin{Subject: "user-1",
			Attribute: testContactValue, Value: testContactValue}, true},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.authorizes(tc.allowed, 1)
			s.observes(1)
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()

			svcErr := s.svc.RecordAttributeVerification(s.ctx, tc.pin)

			s.NotNil(svcErr)
			events := s.events()
			s.Require().Len(events, 1)
			s.Empty(events[0].Payload.Details())
			s.NotContains(fmt.Sprintf("%+v", events[0].Payload), testContactValue)
		})
	}
}

func (s *ServiceAuditTestSuite) TestAnAttemptWithoutACallerIsAnonymous() {
	s.authorizes(true, 1)
	s.observes(1)
	s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()

	_, svcErr := s.svc.PrepareAttributeVerification(context.Background(), storedRequest("workEmail"))

	s.Nil(svcErr)
	events := s.events()
	s.Require().Len(events, 1)
	s.Equal(audit.Actor{Type: audit.ActorTypeAnonymous}, events[0].Payload.Actor())
}

func (s *ServiceAuditTestSuite) TestAFailedPublishDoesNotFailTheAttempt() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Twice()
	s.svc.bus = failing
	s.authorizes(true, 2)
	s.observes(2)
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
