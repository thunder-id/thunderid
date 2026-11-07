// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entitytype

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	testAuditAdminID     = "admin-1"
	testAuditTypeID      = "01900000-0000-7000-8000-0000000000a1"
	testAuditDisplayName = "Audited Contractors"
	testAuditAttribute   = "audit_badge_number"
)

// publishedEvent is one audit event the entity type service published.
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
	require.NoError(t, audit.Subscribe(registry, "entity-type-service-test", sink))
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

// auditEvents names the audit events and target type of one entity type category.
type auditEvents struct {
	category   TypeCategory
	targetType string
	create     string
	update     string
	delete     string
}

var (
	userTypeEvents = auditEvents{TypeCategoryUser, audit.TargetTypeUserType,
		audit.EventUserTypeCreate, audit.EventUserTypeUpdate, audit.EventUserTypeDelete}
	agentTypeEvents = auditEvents{TypeCategoryAgent, audit.TargetTypeAgentType,
		audit.EventAgentTypeCreate, audit.EventAgentTypeUpdate, audit.EventAgentTypeDelete}
)

// EntityTypeServiceAuditTestSuite tests the audit events of the user types and agent types the
// entity type service creates, updates and deletes.
type EntityTypeServiceAuditTestSuite struct {
	suite.Suite
	store   *entityTypeStoreInterfaceMock
	ou      *oumock.OrganizationUnitServiceInterfaceMock
	authz   *sysauthzmock.SystemAuthorizationServiceInterfaceMock
	service *entityTypeService
}

func TestEntityTypeServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(EntityTypeServiceAuditTestSuite))
}

func (s *EntityTypeServiceAuditTestSuite) SetupTest() {
	s.reset()
}

func (s *EntityTypeServiceAuditTestSuite) SetupSubTest() {
	s.reset()
}

func (s *EntityTypeServiceAuditTestSuite) reset() {
	initEntityTypeTestRuntime(s.T())
	s.store = newEntityTypeStoreInterfaceMock(s.T())
	s.ou = oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	s.authz = sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	s.service = &entityTypeService{
		bus:             discardingBus{},
		entityTypeStore: s.store,
		ouService:       s.ou,
		transactioner:   &mockTransactioner{},
		authzService:    s.authz,
	}
}

// record replaces the service's bus with a recording one and returns what it published.
func (s *EntityTypeServiceAuditTestSuite) record() func() []publishedEvent {
	bus, published := newRecordingBus(s.T())
	s.service.bus = bus
	return published
}

// access makes the service's access check answer allowed once.
func (s *EntityTypeServiceAuditTestSuite) access(allowed bool) {
	s.authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(allowed, (*tidcommon.ServiceError)(nil)).Once()
}

// ouExists makes the organization unit check of a create or an update pass once.
func (s *EntityTypeServiceAuditTestSuite) ouExists() {
	s.ou.On("IsOrganizationUnitExists", mock.Anything, testOUID1).
		Return(true, (*tidcommon.ServiceError)(nil)).Once()
}

// created makes a create of a type in category find no type with its handle and store it once, or fail
// to store it with storeErr.
func (s *EntityTypeServiceAuditTestSuite) created(category TypeCategory, storeErr error) {
	s.ouExists()
	s.access(true)
	s.store.On("GetEntityTypeByHandle", mock.Anything, category, createRequest(category).Handle).
		Return(EntityType{}, ErrEntityTypeNotFound).Once()
	s.store.On("CreateEntityType", mock.Anything, mock.Anything).Return(storeErr).Once()
}

// adminContext is the context of a management call made by the admin.
func adminContext() context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditAdminID, testOUID1, "", nil, nil))
}

// auditedSchema is a schema whose attribute name, like the display name, must never be recorded.
func auditedSchema() json.RawMessage {
	return json.RawMessage(`{"` + testAuditAttribute + `":{"type":"string","unique":true}}`)
}

func createRequest(category TypeCategory) CreateEntityTypeRequestWithID {
	handle := "contractor"
	if category == TypeCategoryAgent {
		handle = DefaultAgentTypeHandle
	}
	return CreateEntityTypeRequestWithID{
		Handle: handle, DisplayName: testAuditDisplayName, OUID: testOUID1, Schema: auditedSchema(),
	}
}

func updateRequest() UpdateEntityTypeRequest {
	return UpdateEntityTypeRequest{DisplayName: testAuditDisplayName, OUID: testOUID1, Schema: auditedSchema()}
}

// storedType is the entity type the store holds under testAuditTypeID.
func storedType(category TypeCategory) EntityType {
	return EntityType{ID: testAuditTypeID, Category: category, Handle: createRequest(category).Handle,
		DisplayName: "Contractors", OUID: testOUID1, Schema: auditedSchema()}
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt by the admin
// on the entity type with id, and that it records nothing of the type. An attempt that succeeded or was
// denied names an entity type has it as subject too; a failed one has no subject.
func (s *EntityTypeServiceAuditTestSuite) requireOnlyEvent(events []publishedEvent, name, targetType string,
	action audit.Action, id string, outcome audit.Outcome) {
	s.T().Helper()
	s.Require().Len(events, 1)
	event := events[0]
	target := audit.Ref{Type: targetType, ID: id}
	s.Equal(name, event.name)
	s.Equal(target, event.payload.Target())
	s.Equal(action, event.payload.Action())
	s.Equal(outcome, event.payload.Outcome())
	s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	s.Empty(event.payload.Details(), "no part of the entity type is recorded")
	if outcome.Result == audit.ResultFailed || id == "" {
		s.Equal(audit.Ref{}, event.payload.Subject())
	} else {
		s.Equal(target, event.payload.Subject())
	}
	recorded := fmt.Sprintf("%+v", event.payload)
	s.NotContains(recorded, testAuditDisplayName, "the display name is never recorded")
	s.NotContains(recorded, testAuditAttribute, "no attribute definition is recorded")
}

func (s *EntityTypeServiceAuditTestSuite) TestCreateEntityType_PublishesTheCreatedTypeByCategory() {
	for _, events := range []auditEvents{userTypeEvents, agentTypeEvents} {
		s.Run(string(events.category), func() {
			published := s.record()
			s.created(events.category, nil)

			created, svcErr := s.service.CreateEntityType(adminContext(), events.category,
				createRequest(events.category))

			s.Require().Nil(svcErr)
			s.requireOnlyEvent(published(), events.create, events.targetType, audit.ActionCreate, created.ID,
				audit.OutcomeSucceeded)
		})
	}
}

func (s *EntityTypeServiceAuditTestSuite) TestCreateEntityType_PublishesTheSameNameWhenItDoesNotSucceed() {
	cases := []struct {
		name    string
		events  auditEvents
		request func() CreateEntityTypeRequestWithID
		arrange func()
		outcome audit.Outcome
	}{
		{
			name:   "the handle is invalid",
			events: userTypeEvents,
			request: func() CreateEntityTypeRequestWithID {
				req := createRequest(TypeCategoryUser)
				req.Handle = "Not A Handle"
				return req
			},
			outcome: audit.OutcomeFailed("usrs_1016"),
		},
		{
			name:   "an agent type other than the default one",
			events: agentTypeEvents,
			request: func() CreateEntityTypeRequestWithID {
				req := createRequest(TypeCategoryAgent)
				req.Handle = "tools"
				return req
			},
			outcome: audit.OutcomeFailed("usrs_1014"),
		},
		{
			name:    "the store fails",
			events:  userTypeEvents,
			request: func() CreateEntityTypeRequestWithID { return createRequest(TypeCategoryUser) },
			arrange: func() { s.created(TypeCategoryUser, errors.New("store failure")) },
			outcome: audit.OutcomeFailed("sse_5000"),
		},
		{
			name:    "the caller may not create in the organization unit",
			events:  userTypeEvents,
			request: func() CreateEntityTypeRequestWithID { return createRequest(TypeCategoryUser) },
			arrange: func() {
				s.ouExists()
				s.access(false)
			},
			outcome: audit.OutcomeDenied("forbidden"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			published := s.record()
			if tc.arrange != nil {
				tc.arrange()
			}

			created, svcErr := s.service.CreateEntityType(adminContext(), tc.events.category, tc.request())

			s.Nil(created)
			s.Require().NotNil(svcErr)
			s.requireOnlyEvent(published(), tc.events.create, tc.events.targetType, audit.ActionCreate, "",
				tc.outcome)
		})
	}
}

func (s *EntityTypeServiceAuditTestSuite) TestUpdateEntityType_PublishesOneEventPerAttempt() {
	existing := func(category TypeCategory, allowed bool) {
		s.store.On("IsEntityTypeDeclarative", category, testAuditTypeID).Return(false).Once()
		s.ouExists()
		s.store.On("GetEntityTypeByID", mock.Anything, category, testAuditTypeID).
			Return(storedType(category), nil).Once()
		s.access(allowed)
	}
	cases := []struct {
		name    string
		events  auditEvents
		arrange func()
		outcome audit.Outcome
	}{
		{
			name:   "a user type is updated",
			events: userTypeEvents,
			arrange: func() {
				existing(TypeCategoryUser, true)
				s.store.On("UpdateEntityTypeByID", mock.Anything, TypeCategoryUser, testAuditTypeID,
					mock.Anything).Return(nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name:   "an agent type is updated",
			events: agentTypeEvents,
			arrange: func() {
				existing(TypeCategoryAgent, true)
				s.store.On("UpdateEntityTypeByID", mock.Anything, TypeCategoryAgent, testAuditTypeID,
					mock.Anything).Return(nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name:    "the caller may not change the user type",
			events:  userTypeEvents,
			arrange: func() { existing(TypeCategoryUser, false) },
			outcome: audit.OutcomeDenied("forbidden"),
		},
		{
			name:   "the user type does not exist",
			events: userTypeEvents,
			arrange: func() {
				s.store.On("IsEntityTypeDeclarative", TypeCategoryUser, testAuditTypeID).Return(false).Once()
				s.ouExists()
				s.store.On("GetEntityTypeByID", mock.Anything, TypeCategoryUser, testAuditTypeID).
					Return(EntityType{}, ErrEntityTypeNotFound).Once()
			},
			outcome: audit.OutcomeFailed("usrs_1002"),
		},
		{
			name:   "the user type is declarative",
			events: userTypeEvents,
			arrange: func() {
				s.store.On("IsEntityTypeDeclarative", TypeCategoryUser, testAuditTypeID).Return(true).Once()
			},
			outcome: audit.OutcomeFailed("usrs_1008"),
		},
		{
			name:   "the store fails",
			events: userTypeEvents,
			arrange: func() {
				existing(TypeCategoryUser, true)
				s.store.On("UpdateEntityTypeByID", mock.Anything, TypeCategoryUser, testAuditTypeID,
					mock.Anything).Return(errors.New("store failure")).Once()
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			published := s.record()
			tc.arrange()

			_, svcErr := s.service.UpdateEntityType(adminContext(), tc.events.category, testAuditTypeID,
				updateRequest())

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), tc.events.update, tc.events.targetType, audit.ActionUpdate,
				testAuditTypeID, tc.outcome)
		})
	}
}

func (s *EntityTypeServiceAuditTestSuite) TestDeleteEntityType_PublishesOneEventPerAttempt() {
	existing := func(allowed bool) {
		s.store.On("GetEntityTypeByID", mock.Anything, TypeCategoryUser, testAuditTypeID).
			Return(storedType(TypeCategoryUser), nil).Once()
		s.access(allowed)
	}
	cases := []struct {
		name    string
		events  auditEvents
		arrange func()
		outcome audit.Outcome
	}{
		{
			name:   "the user type is deleted",
			events: userTypeEvents,
			arrange: func() {
				existing(true)
				s.store.On("IsEntityTypeDeclarative", TypeCategoryUser, testAuditTypeID).Return(false).Once()
				s.store.On("DeleteEntityTypeByID", mock.Anything, TypeCategoryUser, testAuditTypeID).
					Return(nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name:    "the caller may not delete the user type",
			events:  userTypeEvents,
			arrange: func() { existing(false) },
			outcome: audit.OutcomeDenied("forbidden"),
		},
		{
			name:   "the user type is declarative",
			events: userTypeEvents,
			arrange: func() {
				existing(true)
				s.store.On("IsEntityTypeDeclarative", TypeCategoryUser, testAuditTypeID).Return(true).Once()
			},
			outcome: audit.OutcomeFailed("usrs_1008"),
		},
		{
			name:   "the store fails",
			events: userTypeEvents,
			arrange: func() {
				existing(true)
				s.store.On("IsEntityTypeDeclarative", TypeCategoryUser, testAuditTypeID).Return(false).Once()
				s.store.On("DeleteEntityTypeByID", mock.Anything, TypeCategoryUser, testAuditTypeID).
					Return(errors.New("store failure")).Once()
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
		{
			name:    "an agent type is never deleted",
			events:  agentTypeEvents,
			arrange: func() {},
			outcome: audit.OutcomeFailed("usrs_1015"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			published := s.record()
			tc.arrange()

			svcErr := s.service.DeleteEntityType(adminContext(), tc.events.category, testAuditTypeID)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), tc.events.delete, tc.events.targetType, audit.ActionDelete,
				testAuditTypeID, tc.outcome)
		})
	}
}

// A delete stays idempotent, but one that finds no user type records nothing, so a caller cannot write a
// succeeded delete for any ID it chooses. The store answers a missing ID with ErrEntityTypeNotFound.
func (s *EntityTypeServiceAuditTestSuite) TestDeleteEntityType_PublishesNothingForAMissingType() {
	published := s.record()
	s.store.On("GetEntityTypeByID", mock.Anything, TypeCategoryUser, testAuditTypeID).
		Return(EntityType{}, ErrEntityTypeNotFound).Once()
	s.access(true)

	svcErr := s.service.DeleteEntityType(adminContext(), TypeCategoryUser, testAuditTypeID)

	s.Nil(svcErr)
	s.Empty(published())
	s.store.AssertNotCalled(s.T(), "DeleteEntityTypeByID", mock.Anything, mock.Anything, mock.Anything)
}

// The access check runs before the delete knows whether the user type exists, so a caller refused it is
// recorded as denied even for an ID that matches no user type.
func (s *EntityTypeServiceAuditTestSuite) TestDeleteEntityType_PublishesARefusalForAMissingType() {
	published := s.record()
	s.store.On("GetEntityTypeByID", mock.Anything, TypeCategoryUser, testAuditTypeID).
		Return(EntityType{}, ErrEntityTypeNotFound).Once()
	s.access(false)

	svcErr := s.service.DeleteEntityType(adminContext(), TypeCategoryUser, testAuditTypeID)

	s.Equal(&tidcommon.ErrorUnauthorized, svcErr)
	s.requireOnlyEvent(published(), audit.EventUserTypeDelete, audit.TargetTypeUserType, audit.ActionDelete,
		testAuditTypeID, audit.OutcomeDenied("forbidden"))
}

// A bus that cannot take the event never fails the change it describes.
func (s *EntityTypeServiceAuditTestSuite) TestCreateEntityType_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	s.service.bus = failing
	s.created(TypeCategoryUser, nil)

	created, svcErr := s.service.CreateEntityType(adminContext(), TypeCategoryUser,
		createRequest(TypeCategoryUser))

	s.Nil(svcErr)
	s.NotEmpty(created.ID)
}
