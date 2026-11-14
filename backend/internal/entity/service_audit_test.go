// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"encoding/json"
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
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
)

const (
	testAuditActorID = "operator"
	testAuditEmail   = "private.contact@example.com"
	testAuditMobile  = "+94771234567"
)

// bothVerified is stored verification metadata with workEmail and mobile verified.
const bothVerified = `{"attributeVerification":{"workEmail":{"verifiedAt":"2026-10-15T00:00:00Z"},` +
	`"mobile":{"verifiedAt":"2026-10-15T00:00:00Z"}}}`

// publishedEvent is one audit event the entity service published.
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
	require.NoError(t, audit.Subscribe(registry, "entity-service-test", sink))
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

// VerificationAuditTestSuite checks the audit event each verification a committed write clears publishes.
type VerificationAuditTestSuite struct {
	suite.Suite
}

func TestVerificationAuditTestSuite(t *testing.T) {
	suite.Run(t, new(VerificationAuditTestSuite))
}

// clearingService returns a service whose update of user-1's attributes to an empty set clears the
// verification of workEmail and mobile, publishing on bus.
func (s *VerificationAuditTestSuite) clearingService(bus eventbus.EventBusInterface) *entityService {
	store := newEntityStoreInterfaceMock(s.T())
	store.On("GetIndexedAttributes").Return(map[string]bool{}).Maybe()
	expectAttributeUpdate(store, verificationUser(
		`{"workEmail":"`+testAuditEmail+`","mobile":"`+testAuditMobile+`"}`, bothVerified))
	store.On("UpdateAttributes", inTransaction(), "user-1", mock.Anything).Return(nil).Once()
	store.On("UpdateSystemAttributes", inTransaction(), "user-1", withoutVerification()).Return(nil).Once()
	runtime := newEntityRuntimeStoreInterfaceMock(s.T())
	installRuntimeFixture(runtime)
	return &entityService{store: store, runtimeStore: runtime, entityTypeService: contactTypeService(s.T()),
		transactioner: markingTransactioner(s.T()), bus: bus, logger: log.GetLogger()}
}

func (s *VerificationAuditTestSuite) TestAClearedVerificationPublishesOneEventEach() {
	bus, events := newRecordingBus(s.T())
	svc := s.clearingService(bus)
	ctx := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditActorID, "ou1", "", nil, nil))

	s.Require().NoError(svc.UpdateAttributes(ctx, "user-1", json.RawMessage(`{}`)))

	published := events()
	s.Require().Len(published, 2)
	user := audit.Ref{Type: audit.TargetTypeUser, ID: "user-1"}
	attributes := make([]string, 0, len(published))
	for _, evt := range published {
		s.Equal(audit.EventUserVerificationInvalidate, evt.name)
		s.Equal(user, evt.payload.Target())
		s.Equal(user, evt.payload.Subject())
		s.Equal(audit.ActionInvalidate, evt.payload.Action())
		s.Equal(audit.OutcomeSucceeded, evt.payload.Outcome())
		s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditActorID}, evt.payload.Actor())
		s.Require().Len(evt.payload.Details(), 1)
		attributes = append(attributes, evt.payload.Details()[audit.DetailAttribute])
		recorded := fmt.Sprintf("%+v", evt.payload)
		s.NotContains(recorded, testAuditEmail, "a contact is never recorded")
		s.NotContains(recorded, testAuditMobile, "a contact is never recorded")
		s.NotContains(recorded, string(verificationValueRemoved), "the invalidation reason is not recorded")
	}
	s.ElementsMatch([]string{"workEmail", "mobile"}, attributes)
}

func (s *VerificationAuditTestSuite) TestARuntimeContextIsAnonymous() {
	signedIn := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditActorID, "ou1", "", nil, nil))
	for name, ctx := range map[string]context.Context{
		"no caller":          security.WithRuntimeContext(context.Background()),
		"a caller in a flow": security.WithRuntimeContext(signedIn),
	} {
		s.Run(name, func() {
			bus, events := newRecordingBus(s.T())
			svc := s.clearingService(bus)

			s.Require().NoError(svc.UpdateAttributes(ctx, "user-1", json.RawMessage(`{}`)))

			published := events()
			s.Require().Len(published, 2)
			for _, evt := range published {
				s.Equal(audit.Actor{Type: audit.ActorTypeAnonymous}, evt.payload.Actor())
			}
		})
	}
}

func (s *VerificationAuditTestSuite) TestAuditDoesNotNeedObservability() {
	bus, events := newRecordingBus(s.T())
	svc := s.clearingService(bus)
	svc.observability = nil

	s.Require().NoError(svc.UpdateAttributes(context.Background(), "user-1", json.RawMessage(`{}`)))

	s.Len(events(), 2)
}

func (s *VerificationAuditTestSuite) TestAWriteThatClearsNothingPublishesNoEvent() {
	bus, events := newRecordingBus(s.T())
	store := newEntityStoreInterfaceMock(s.T())
	store.On("GetIndexedAttributes").Return(map[string]bool{}).Maybe()
	expectAttributeUpdate(store, verificationUser(`{"workEmail":"`+testAuditEmail+`"}`, `{}`))
	store.On("UpdateAttributes", inTransaction(), "user-1", mock.Anything).Return(nil).Once()
	runtime := newEntityRuntimeStoreInterfaceMock(s.T())
	installRuntimeFixture(runtime)
	svc := &entityService{store: store, runtimeStore: runtime, entityTypeService: contactTypeService(s.T()),
		transactioner: markingTransactioner(s.T()), bus: bus, logger: log.GetLogger()}

	s.Require().NoError(svc.UpdateAttributes(context.Background(), "user-1", json.RawMessage(`{}`)))

	s.Empty(events())
}

func (s *VerificationAuditTestSuite) TestAFailedPublishDoesNotFailTheWrite() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(fmt.Errorf("audit queue is full")).Twice()
	svc := s.clearingService(failing)

	s.NoError(svc.UpdateAttributes(context.Background(), "user-1", json.RawMessage(`{}`)))
}

func (s *VerificationAuditTestSuite) TestInitialize_RefusesANilEventBus() {
	service, err := Initialize(nil, nil, nil, nil, nil, nil, nil)

	s.ErrorIs(err, errNilEventBus)
	s.Nil(service)
}
