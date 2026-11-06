// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	testAuditAdminID     = "admin-1"
	testAuditGroupID     = "grp-001"
	testAuditGroupName   = "Audited Librarians"
	testAuditDescription = "Keepers of the catalog"
	testAuditUserID      = "usr-001"
	testAuditAgentID     = "agt-001"
	testAuditNestedID    = "grp-002"
)

// publishedEvent is one audit event the group service published.
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
	require.NoError(t, audit.Subscribe(registry, "group-service-test", sink))
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

// adminContext is the context of a management call made by the admin.
func adminContext() context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditAdminID, testOUID1, "", nil, nil))
}

// GroupServiceAuditTestSuite tests the audit events the group service publishes, on a recording bus.
type GroupServiceAuditTestSuite struct {
	suite.Suite
	store     *groupStoreInterfaceMock
	ous       *oumock.OrganizationUnitServiceInterfaceMock
	entities  *entitymock.EntityServiceInterfaceMock
	bus       eventbus.EventBusInterface
	published func() []publishedEvent
}

func TestGroupServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(GroupServiceAuditTestSuite))
}

func (s *GroupServiceAuditTestSuite) SetupTest() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("/tmp/test", &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}))

	s.store = newGroupStoreInterfaceMock(s.T())
	s.ous = oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	s.entities = entitymock.NewEntityServiceInterfaceMock(s.T())
	s.bus, s.published = newRecordingBus(s.T())
}

// SetupSubTest gives each table case its own mocks and recording bus.
func (s *GroupServiceAuditTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *GroupServiceAuditTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// newService returns a group service on the suite's mocks and recording bus that authorizes with authz.
func (s *GroupServiceAuditTestSuite) newService(authz sysauthz.SystemAuthorizationServiceInterface) *groupService {
	service := newGroupServiceWithStore(s.store, s.ous, s.entities, nil, authz, &stubTransactioner{}, nil, s.bus)
	service.dependencyRegistry = noopDepRegistry{}
	return service
}

// denyingAuthz refuses every group action the caller asks for, as it does for a unit it does not manage.
func denyingAuthz(t *testing.T) sysauthz.SystemAuthorizationServiceInterface {
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(false, (*tidcommon.ServiceError)(nil)).Maybe()
	return authzMock
}

// grantRefusingAuthz allows every group action and refuses every membership grant, as sysauthz does for
// a caller lacking what the group confers.
func grantRefusingAuthz(t *testing.T) sysauthz.SystemAuthorizationServiceInterface {
	authzMock := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	authzMock.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(true, (*tidcommon.ServiceError)(nil)).Maybe()
	authzMock.On("CanGrantMembership", mock.Anything, mock.Anything, mock.Anything).
		Return(&sysauthz.ErrorGrantNotPermitted).Maybe()
	return authzMock
}

// requireGroupEvent checks that event is named name, for an attempt by the admin on the group with
// groupID, with subject as its subject, and carries nothing that describes the group.
func (s *GroupServiceAuditTestSuite) requireGroupEvent(event publishedEvent, name string, action audit.Action,
	groupID string, outcome audit.Outcome, subject audit.Ref) {
	s.T().Helper()
	s.Require().Equal(name, event.name)
	s.Require().Equal(audit.Ref{Type: audit.TargetTypeGroup, ID: groupID}, event.payload.Target())
	s.Require().Equal(action, event.payload.Action())
	s.Require().Equal(outcome, event.payload.Outcome())
	s.Require().Equal(subject, event.payload.Subject())
	s.Require().Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	s.Require().Empty(event.payload.Details(), "no attribute of the group is recorded")
	recorded := fmt.Sprintf("%+v", event.payload)
	s.Require().NotContains(recorded, testAuditGroupName, "the group name is not recorded")
	s.Require().NotContains(recorded, testAuditDescription, "the group description is not recorded")
}

var (
	auditGroup  = audit.Ref{Type: audit.TargetTypeGroup, ID: testAuditGroupID}
	auditUser   = audit.Ref{Type: audit.TargetTypeUser, ID: testAuditUserID}
	auditAgent  = audit.Ref{Type: audit.TargetTypeAgent, ID: testAuditAgentID}
	auditNested = audit.Ref{Type: audit.TargetTypeGroup, ID: testAuditNestedID}
)

// auditedMembers are a user, an agent and a nested group, the user named twice.
func auditedMembers() []Member {
	return []Member{
		{ID: testAuditUserID, Type: MemberTypeUser},
		{ID: testAuditAgentID, Type: MemberTypeAgent},
		{ID: testAuditNestedID, Type: MemberTypeGroup},
		{ID: testAuditUserID, Type: MemberTypeUser},
	}
}

// memberEntities stubs the user and the agent as living in testOUID1.
func (s *GroupServiceAuditTestSuite) memberEntities() {
	s.entities.On("GetEntitiesByIDs", mock.Anything, mock.Anything).Return([]providers.Entity{
		{ID: testAuditUserID, Category: providers.EntityCategoryUser, OUID: testOUID1},
		{ID: testAuditAgentID, Category: providers.EntityCategoryAgent, OUID: testOUID1},
	}, nil)
	// A removal checks only that a nested group exists, not where it lives.
	s.store.On("GetGroupsByIDs", mock.Anything, []string{testAuditNestedID}).
		Return([]GroupBasicDAO{{ID: testAuditNestedID, OUID: testOUID1}}, nil).Maybe()
}

// memberUser stubs the user as living in testOUID1.
func (s *GroupServiceAuditTestSuite) memberUser() {
	s.entities.On("GetEntitiesByIDs", mock.Anything, []string{testAuditUserID}).Return([]providers.Entity{
		{ID: testAuditUserID, Category: providers.EntityCategoryUser, OUID: testOUID1},
	}, nil)
}

// arrangeCreate stubs a create of the group in testOUID1 that succeeds.
func (s *GroupServiceAuditTestSuite) arrangeCreate() {
	s.ous.On("IsOrganizationUnitExists", mock.Anything, testOUID1).Return(true, nil)
	s.memberEntities()
	s.store.On("CheckGroupNameConflictForCreate", mock.Anything, testAuditGroupName, testOUID1).Return(nil)
	s.store.On("CreateGroup", mock.Anything, mock.Anything).Return(nil)
}

// requireCreatedWithMembers checks that the create of the group with groupID was published, then each
// distinct member it was created with.
func (s *GroupServiceAuditTestSuite) requireCreatedWithMembers(events []publishedEvent, groupID string) {
	s.T().Helper()
	s.Require().Len(events, 4)
	group := audit.Ref{Type: audit.TargetTypeGroup, ID: groupID}
	s.requireGroupEvent(events[0], audit.EventGroupCreate, audit.ActionCreate, groupID, audit.OutcomeSucceeded,
		group)
	for i, member := range []audit.Ref{auditUser, auditAgent, auditNested} {
		s.requireGroupEvent(events[i+1], audit.EventGroupMemberAdd, audit.ActionAdd, groupID,
			audit.OutcomeSucceeded, member)
	}
}

// A group created with members is recorded as created, and each member as added to it once, so every
// membership the create granted is in the log.
func (s *GroupServiceAuditTestSuite) TestCreateGroup_PublishesTheGroupAndEachMember() {
	service := s.newService(newAllowAllAuthz(s.T()))
	s.arrangeCreate()

	created, svcErr := service.CreateGroup(adminContext(), CreateGroupRequest{
		Name: testAuditGroupName, Description: testAuditDescription, OUID: testOUID1, Members: auditedMembers(),
	})

	s.Require().Nil(svcErr)
	s.requireCreatedWithMembers(s.published(), created.ID)
}

// A create under an organization unit path is the same attempt, recorded once.
func (s *GroupServiceAuditTestSuite) TestCreateGroupByPath_PublishesTheGroupAndEachMember() {
	service := s.newService(newAllowAllAuthz(s.T()))
	s.ous.On("GetOrganizationUnitByPath", mock.Anything, "library").
		Return(oupkg.OrganizationUnit{ID: testOUID1}, nil)
	s.arrangeCreate()

	created, svcErr := service.CreateGroupByPath(adminContext(), "library", CreateGroupByPathRequest{
		Name: testAuditGroupName, Description: testAuditDescription, Members: auditedMembers(),
	})

	s.Require().Nil(svcErr)
	s.requireCreatedWithMembers(s.published(), created.ID)
}

// A create that fails or is refused is one event naming no group, and no member is recorded as added.
func (s *GroupServiceAuditTestSuite) TestCreateGroup_PublishesOneEventWhenItFails() {
	cases := []struct {
		name    string
		authz   func(t *testing.T) sysauthz.SystemAuthorizationServiceInterface
		create  func(service *groupService) *tidcommon.ServiceError
		outcome audit.Outcome
	}{
		{
			name:  "the request has no name",
			authz: newAllowAllAuthz,
			create: func(service *groupService) *tidcommon.ServiceError {
				_, svcErr := service.CreateGroup(adminContext(), CreateGroupRequest{OUID: testOUID1})
				return svcErr
			},
			outcome: audit.OutcomeFailed("grp_1001"),
		},
		{
			name:  "the organization unit path does not exist",
			authz: newAllowAllAuthz,
			create: func(service *groupService) *tidcommon.ServiceError {
				s.ous.On("GetOrganizationUnitByPath", mock.Anything, "nowhere").
					Return(oupkg.OrganizationUnit{}, &oupkg.ErrorOrganizationUnitNotFound)
				_, svcErr := service.CreateGroupByPath(adminContext(), "nowhere",
					CreateGroupByPathRequest{Name: testAuditGroupName})
				return svcErr
			},
			outcome: audit.OutcomeFailed("grp_1003"),
		},
		{
			name:  "the caller does not manage the organization unit",
			authz: denyingAuthz,
			create: func(service *groupService) *tidcommon.ServiceError {
				s.ous.On("IsOrganizationUnitExists", mock.Anything, testOUID1).Return(true, nil)
				_, svcErr := service.CreateGroup(adminContext(), CreateGroupRequest{
					Name: testAuditGroupName, OUID: testOUID1, Members: auditedMembers(),
				})
				return svcErr
			},
			outcome: audit.OutcomeDenied("forbidden"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			service := s.newService(tc.authz(s.T()))

			s.Require().NotNil(tc.create(service))

			events := s.published()
			s.Require().Len(events, 1)
			s.requireGroupEvent(events[0], audit.EventGroupCreate, audit.ActionCreate, "", tc.outcome,
				audit.Ref{})
		})
	}
}

// An update or a delete is one event naming the group the caller sent. One that was refused names a
// group the service found, so it is the subject; one that failed may name none.
func (s *GroupServiceAuditTestSuite) TestUpdateAndDeleteGroup_PublishOneEventPerAttempt() {
	update := func(service *groupService) *tidcommon.ServiceError {
		_, svcErr := service.UpdateGroup(adminContext(), testAuditGroupID, UpdateGroupRequest{
			Name: testAuditGroupName, Description: testAuditDescription, OUID: testOUID1,
		})
		return svcErr
	}
	deleteGroup := func(service *groupService) *tidcommon.ServiceError {
		return service.DeleteGroup(adminContext(), testAuditGroupID)
	}
	existing := func() {
		s.store.On("IsGroupDeclarative", mock.Anything, testAuditGroupID).Return(false, nil)
		s.store.On("GetGroup", mock.Anything, testAuditGroupID).
			Return(GroupDAO{ID: testAuditGroupID, Name: "Old", OUID: testOUID1}, nil)
	}
	cases := []struct {
		name    string
		authz   func(t *testing.T) sysauthz.SystemAuthorizationServiceInterface
		arrange func()
		call    func(service *groupService) *tidcommon.ServiceError
		event   string
		action  audit.Action
		outcome audit.Outcome
		subject audit.Ref
	}{
		{
			name: "the group is updated", authz: newAllowAllAuthz, call: update,
			arrange: func() {
				existing()
				s.store.On("CheckGroupNameConflictForUpdate", mock.Anything, testAuditGroupName, testOUID1,
					testAuditGroupID).Return(nil)
				s.store.On("UpdateGroup", mock.Anything, mock.Anything).Return(nil)
			},
			event: audit.EventGroupUpdate, action: audit.ActionUpdate, outcome: audit.OutcomeSucceeded,
			subject: auditGroup,
		},
		{
			name: "the group to update does not exist", authz: newAllowAllAuthz, call: update,
			arrange: func() {
				s.store.On("IsGroupDeclarative", mock.Anything, testAuditGroupID).Return(false, nil)
				s.store.On("GetGroup", mock.Anything, testAuditGroupID).Return(GroupDAO{}, ErrGroupNotFound)
			},
			event: audit.EventGroupUpdate, action: audit.ActionUpdate, outcome: audit.OutcomeFailed("grp_1003"),
		},
		{
			name: "the caller does not manage the group to update", authz: denyingAuthz, call: update,
			arrange: existing,
			event:   audit.EventGroupUpdate, action: audit.ActionUpdate, outcome: audit.OutcomeDenied("forbidden"),
			subject: auditGroup,
		},
		{
			name: "the group is deleted", authz: newAllowAllAuthz, call: deleteGroup,
			arrange: func() {
				existing()
				s.store.On("DeleteGroup", mock.Anything, testAuditGroupID).Return(nil)
			},
			event: audit.EventGroupDelete, action: audit.ActionDelete, outcome: audit.OutcomeSucceeded,
			subject: auditGroup,
		},
		{
			name: "the group to delete does not exist", authz: newAllowAllAuthz, call: deleteGroup,
			arrange: func() {
				s.store.On("IsGroupDeclarative", mock.Anything, testAuditGroupID).Return(false, nil)
				s.store.On("GetGroup", mock.Anything, testAuditGroupID).Return(GroupDAO{}, ErrGroupNotFound)
			},
			event: audit.EventGroupDelete, action: audit.ActionDelete, outcome: audit.OutcomeFailed("grp_1003"),
		},
		{
			name: "the caller does not manage the group to delete", authz: denyingAuthz, call: deleteGroup,
			arrange: existing,
			event:   audit.EventGroupDelete, action: audit.ActionDelete, outcome: audit.OutcomeDenied("forbidden"),
			subject: auditGroup,
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			service := s.newService(tc.authz(s.T()))
			tc.arrange()

			svcErr := tc.call(service)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			events := s.published()
			s.Require().Len(events, 1)
			s.requireGroupEvent(events[0], tc.event, tc.action, testAuditGroupID, tc.outcome, tc.subject)
		})
	}
}

// arrangeMembershipChange stubs the group and its members for a membership change that succeeds.
func (s *GroupServiceAuditTestSuite) arrangeMembershipChange() {
	s.store.On("GetGroup", mock.Anything, testAuditGroupID).
		Return(GroupDAO{ID: testAuditGroupID, OUID: testOUID1}, nil)
	s.memberEntities()
}

// Adding or removing several members is one event per distinct member, each naming who gained or lost
// the membership.
func (s *GroupServiceAuditTestSuite) TestGroupMembers_PublishEachMember() {
	cases := []struct {
		name   string
		store  string
		call   func(service *groupService) (*Group, *tidcommon.ServiceError)
		event  string
		action audit.Action
	}{
		{
			name: "add", store: "AddGroupMembers",
			call: func(service *groupService) (*Group, *tidcommon.ServiceError) {
				return service.AddGroupMembers(adminContext(), testAuditGroupID, auditedMembers())
			},
			event: audit.EventGroupMemberAdd, action: audit.ActionAdd,
		},
		{
			name: "remove", store: "RemoveGroupMembers",
			call: func(service *groupService) (*Group, *tidcommon.ServiceError) {
				return service.RemoveGroupMembers(adminContext(), testAuditGroupID, auditedMembers())
			},
			event: audit.EventGroupMemberRemove, action: audit.ActionRemove,
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			service := s.newService(newAllowAllAuthz(s.T()))
			s.arrangeMembershipChange()
			s.store.On("ValidateGroupIDs", mock.Anything, mock.Anything).Return(nil, nil).Maybe()
			s.store.On(tc.store, mock.Anything, testAuditGroupID, mock.Anything).Return(nil)

			_, svcErr := tc.call(service)

			s.Require().Nil(svcErr)
			events := s.published()
			s.Require().Len(events, 3)
			for i, member := range []audit.Ref{auditUser, auditAgent, auditNested} {
				s.requireGroupEvent(events[i], tc.event, tc.action, testAuditGroupID,
					audit.OutcomeSucceeded, member)
			}
		})
	}
}

// A membership change that fails or is refused is one event for the call, naming the group only: no
// member gained or lost it.
func (s *GroupServiceAuditTestSuite) TestGroupMembers_PublishOneEventWhenItFails() {
	cases := []struct {
		name    string
		authz   func(t *testing.T) sysauthz.SystemAuthorizationServiceInterface
		arrange func()
		call    func(service *groupService) *tidcommon.ServiceError
		event   string
		action  audit.Action
		outcome audit.Outcome
	}{
		{
			name: "the group to add to does not exist", authz: newAllowAllAuthz,
			arrange: func() {
				s.store.On("GetGroup", mock.Anything, testAuditGroupID).Return(GroupDAO{}, ErrGroupNotFound)
			},
			call: func(service *groupService) *tidcommon.ServiceError {
				_, svcErr := service.AddGroupMembers(adminContext(), testAuditGroupID, auditedMembers())
				return svcErr
			},
			event: audit.EventGroupMemberAdd, action: audit.ActionAdd, outcome: audit.OutcomeFailed("grp_1003"),
		},
		{
			name: "the caller may not grant what the group confers", authz: grantRefusingAuthz,
			arrange: func() {
				s.store.On("GetGroup", mock.Anything, testAuditGroupID).
					Return(GroupDAO{ID: testAuditGroupID, OUID: testOUID1}, nil)
			},
			call: func(service *groupService) *tidcommon.ServiceError {
				_, svcErr := service.AddGroupMembers(adminContext(), testAuditGroupID, auditedMembers())
				return svcErr
			},
			event: audit.EventGroupMemberAdd, action: audit.ActionAdd, outcome: audit.OutcomeDenied("forbidden"),
		},
		{
			name: "the caller does not manage the group to remove from", authz: denyingAuthz,
			arrange: func() {
				s.store.On("GetGroup", mock.Anything, testAuditGroupID).
					Return(GroupDAO{ID: testAuditGroupID, OUID: testOUID1}, nil)
			},
			call: func(service *groupService) *tidcommon.ServiceError {
				_, svcErr := service.RemoveGroupMembers(adminContext(), testAuditGroupID, auditedMembers())
				return svcErr
			},
			event: audit.EventGroupMemberRemove, action: audit.ActionRemove,
			outcome: audit.OutcomeDenied("forbidden"),
		},
		{
			name: "no member is named", authz: newAllowAllAuthz, arrange: func() {},
			call: func(service *groupService) *tidcommon.ServiceError {
				_, svcErr := service.RemoveGroupMembers(adminContext(), testAuditGroupID, nil)
				return svcErr
			},
			event: audit.EventGroupMemberRemove, action: audit.ActionRemove,
			outcome: audit.OutcomeFailed("grp_1013"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			service := s.newService(tc.authz(s.T()))
			tc.arrange()

			s.Require().NotNil(tc.call(service))

			events := s.published()
			s.Require().Len(events, 1)
			s.requireGroupEvent(events[0], tc.event, tc.action, testAuditGroupID, tc.outcome, audit.Ref{})
		})
	}
}

// Adding members to several groups at once is one event per group and member, after all of them
// committed: none is published from inside the transaction.
func (s *GroupServiceAuditTestSuite) TestAddMembersToGroups_PublishesEachGroupAndMember() {
	service := s.newService(newAllowAllAuthz(s.T()))
	s.store.On("ValidateGroupIDs", mock.Anything, mock.Anything).Return(nil, nil)
	for _, id := range []string{testAuditGroupID, "grp-003"} {
		s.store.On("GetGroup", mock.Anything, id).Return(GroupDAO{ID: id, OUID: testOUID1}, nil)
		s.store.On("AddGroupMembers", mock.Anything, id, mock.Anything).Return(nil)
	}
	s.memberUser()

	svcErr := service.AddMembersToGroups(adminContext(),
		[]Member{{ID: testAuditUserID, Type: MemberTypeUser}}, []string{testAuditGroupID, "grp-003"})

	s.Require().Nil(svcErr)
	events := s.published()
	s.Require().Len(events, 2)
	s.requireGroupEvent(events[0], audit.EventGroupMemberAdd, audit.ActionAdd, testAuditGroupID,
		audit.OutcomeSucceeded, auditUser)
	s.requireGroupEvent(events[1], audit.EventGroupMemberAdd, audit.ActionAdd, "grp-003",
		audit.OutcomeSucceeded, auditUser)
}

// A refusal for one group rolls back every group, so each is recorded as failed and no member as added.
func (s *GroupServiceAuditTestSuite) TestAddMembersToGroups_PublishesEachGroupWhenItFails() {
	service := s.newService(newAllowAllAuthz(s.T()))
	s.store.On("ValidateGroupIDs", mock.Anything, mock.Anything).Return(nil, nil)
	s.store.On("GetGroup", mock.Anything, testAuditGroupID).Return(GroupDAO{ID: testAuditGroupID, OUID: testOUID1}, nil)
	s.store.On("AddGroupMembers", mock.Anything, testAuditGroupID, mock.Anything).Return(nil)
	s.store.On("GetGroup", mock.Anything, "grp-003").Return(GroupDAO{}, ErrGroupNotFound)
	s.memberUser()

	svcErr := service.AddMembersToGroups(adminContext(),
		[]Member{{ID: testAuditUserID, Type: MemberTypeUser}}, []string{testAuditGroupID, "grp-003"})

	s.Equal(&ErrorGroupNotFound, svcErr)
	events := s.published()
	s.Require().Len(events, 2)
	failed := audit.OutcomeFailed("grp_1003")
	s.requireGroupEvent(events[0], audit.EventGroupMemberAdd, audit.ActionAdd, testAuditGroupID, failed,
		audit.Ref{})
	s.requireGroupEvent(events[1], audit.EventGroupMemberAdd, audit.ActionAdd, "grp-003", failed,
		audit.Ref{})
}

// A bus that cannot take the events never fails the change they describe.
func (s *GroupServiceAuditTestSuite) TestCreateGroup_SucceedsWhenTheBusRefusesTheEvents() {
	service := s.newService(newAllowAllAuthz(s.T()))
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Times(4)
	service.bus = failing
	s.arrangeCreate()

	created, svcErr := service.CreateGroup(adminContext(), CreateGroupRequest{
		Name: testAuditGroupName, OUID: testOUID1, Members: auditedMembers(),
	})

	s.Nil(svcErr)
	s.NotEmpty(created.ID)
}

func (s *GroupServiceAuditTestSuite) TestInitialize_RefusesANilEventBus() {
	groupService, resolver, exporter, err := Initialize(http.NewServeMux(), nil, nil, nil, nil, nil, nil, nil)

	s.Require().ErrorIs(err, errNilEventBus)
	s.Require().Nil(groupService)
	s.Require().Nil(resolver)
	s.Require().Nil(exporter)
}
