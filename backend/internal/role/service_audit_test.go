// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

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

	"github.com/thunder-id/thunderid/internal/group"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/groupmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	testAuditAdminID     = "admin-1"
	testAuditRoleName    = "Audited Librarian Role"
	testAuditDescription = "Grants the catalog permissions"
	testAuditPermission  = "catalog:write"
	testAuditGroupID     = "group-1"
	testAuditAgentID     = "agent-1"
)

// publishedEvent is one audit event the role services published.
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
	require.NoError(t, audit.Subscribe(registry, "role-service-test", sink))
	return bus, func() []publishedEvent {
		stopper.Stop()
		sink.mu.Lock()
		defer sink.mu.Unlock()
		return sink.events
	}
}

// discardingBus is an event bus that drops every event, for a test the audit event does not matter to.
// A test of what the services publish uses newRecordingBus.
type discardingBus struct{}

func (discardingBus) Publish(context.Context, *eventbus.Event) error {
	return nil
}

// adminContext is the context of a management call made by the admin.
func adminContext() context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditAdminID, "ou1", "", nil, nil))
}

// auditedRoleRequest is a role created with a user, a group and an agent assigned, the user twice.
func auditedRoleRequest() RoleCreationDetail {
	return RoleCreationDetail{
		Name:        testAuditRoleName,
		Description: testAuditDescription,
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{testAuditPermission}}},
		Assignments: []RoleAssignment{
			{ID: testUserID1, Type: AssigneeTypeUser},
			{ID: testAuditGroupID, Type: AssigneeTypeGroup},
			{ID: testAuditAgentID, Type: AssigneeTypeAgent},
			{ID: testUserID1, Type: AssigneeTypeUser},
		},
	}
}

// RoleServiceAuditTestSuite tests the audit events the role and role assignment services publish.
type RoleServiceAuditTestSuite struct {
	suite.Suite
	mockStore             *roleStoreInterfaceMock
	mockEntityService     *entitymock.EntityServiceInterfaceMock
	mockGroupService      *groupmock.GroupServiceInterfaceMock
	mockOUService         *oumock.OrganizationUnitServiceInterfaceMock
	mockResourceService   *resourcemock.ResourceServiceInterfaceMock
	mockEntityTypeService *entitytypemock.EntityTypeServiceInterfaceMock
	mockSharingService    *sharingmock.SharingServiceInterfaceMock
	authz                 *sysauthzmock.SystemAuthorizationServiceInterfaceMock
	transactioner         *fakeTransactioner
	bus                   eventbus.EventBusInterface
	published             func() []publishedEvent
	service               RoleServiceInterface
	assignmentService     RoleAssignmentServiceInterface
}

func TestRoleServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(RoleServiceAuditTestSuite))
}

func (s *RoleServiceAuditTestSuite) SetupTest() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("/tmp/test", &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: false},
	}))

	s.mockStore = newRoleStoreInterfaceMock(s.T())
	s.mockEntityService = entitymock.NewEntityServiceInterfaceMock(s.T())
	s.mockGroupService = groupmock.NewGroupServiceInterfaceMock(s.T())
	s.mockOUService = oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	s.mockResourceService = resourcemock.NewResourceServiceInterfaceMock(s.T())
	s.mockEntityTypeService = entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	s.mockSharingService = sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.authz = sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	s.transactioner = &fakeTransactioner{}
	s.bus, s.published = newRecordingBus(s.T())
	s.service = s.newRoleService(s.bus)
	s.assignmentService = newRoleAssignmentService(s.mockStore, s.mockEntityService, s.mockGroupService,
		s.mockEntityTypeService, nil, s.transactioner, s.authz, s.mockSharingService, s.bus)
}

// SetupSubTest gives each table case its own mocks and recording bus.
func (s *RoleServiceAuditTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *RoleServiceAuditTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *RoleServiceAuditTestSuite) newRoleService(bus eventbus.EventBusInterface) RoleServiceInterface {
	return newRoleService(s.mockStore, s.mockEntityService, s.mockGroupService, s.mockOUService,
		s.mockResourceService, s.transactioner, s.authz, s.mockSharingService, bus, nil)
}

// expectPermissionGrant expects the check that the caller may confer the role's permissions to be asked
// once, answered with svcErr.
func (s *RoleServiceAuditTestSuite) expectPermissionGrant(svcErr *tidcommon.ServiceError) {
	s.authz.On("CanGrantPermissions", mock.Anything, mock.Anything).Return(svcErr).Once()
}

// expectMembershipGrants expects the check that the caller may grant the role with roleID to be asked
// times times, answered with svcErr.
func (s *RoleServiceAuditTestSuite) expectMembershipGrants(roleID string, svcErr *tidcommon.ServiceError,
	times int) {
	s.authz.On("CanGrantMembership", mock.Anything, sysauthz.PrincipalTypeRole, roleID).
		Return(svcErr).Times(times)
}

// requireEvent checks that event is named name, for an attempt by the admin on the role with roleID,
// with subject as its subject, and carries nothing that describes the role.
func (s *RoleServiceAuditTestSuite) requireEvent(event publishedEvent, name string, action audit.Action,
	roleID string, outcome audit.Outcome, subject audit.Ref) {
	s.T().Helper()
	s.Require().Equal(name, event.name)
	s.Require().Equal(audit.Ref{Type: audit.TargetTypeRole, ID: roleID}, event.payload.Target())
	s.Require().Equal(action, event.payload.Action())
	s.Require().Equal(outcome, event.payload.Outcome())
	s.Require().Equal(subject, event.payload.Subject())
	s.Require().Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	s.Require().Empty(event.payload.Details(), "no attribute of the role is recorded")
	recorded := fmt.Sprintf("%+v", event.payload)
	s.Require().NotContains(recorded, testAuditRoleName, "the role name is not recorded")
	s.Require().NotContains(recorded, testAuditDescription, "the role description is not recorded")
	s.Require().NotContains(recorded, testAuditPermission, "no permission is recorded")
}

// arrangeAuditedRoleCreate stubs what creating auditedRoleRequest reads and writes.
func (s *RoleServiceAuditTestSuite) arrangeAuditedRoleCreate() {
	s.mockResourceService.On("ValidatePermissions", mock.Anything, "rs1", []string{testAuditPermission}).
		Return([]string{}, nil).Once()
	s.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").
		Return(oupkg.OrganizationUnit{ID: "ou1", Handle: "default"}, nil).Once()
	s.mockEntityService.On("GetEntitiesByIDs", mock.Anything, mock.Anything).Return([]providers.Entity{
		{ID: testUserID1, Category: providers.EntityCategoryUser, OUID: "ou1"},
		{ID: testAuditAgentID, Category: providers.EntityCategoryAgent, OUID: "ou1"},
	}, nil).Once()
	s.mockGroupService.On("ValidateGroupIDs", mock.Anything, []string{testAuditGroupID}).Return(nil).Once()
	s.mockGroupService.On("GetGroupsByIDs", mock.Anything, []string{testAuditGroupID}).
		Return(map[string]*group.Group{testAuditGroupID: {ID: testAuditGroupID, OUID: "ou1"}}, nil).Once()
	s.mockStore.On("CheckRoleNameExists", mock.Anything, "ou1", testAuditRoleName).Return(false, nil).Once()
	s.mockSharingService.EXPECT().ListVisibleResourceIDs(mock.Anything, RoleSharingType, "ou1").
		Return(nil, nil).Once()
	s.expectPermissionGrant(nil)
}

// A role created with assignees is recorded as created, and each assignee as added to it once, after
// the create, so every grant the create made is in the log.
func (s *RoleServiceAuditTestSuite) TestCreateRole_PublishesTheRoleAndEachAssignee() {
	s.arrangeAuditedRoleCreate()
	s.mockStore.On("CreateRole", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	created, svcErr := s.service.CreateRole(adminContext(), auditedRoleRequest())

	s.Require().Nil(svcErr)
	events := s.published()
	s.Require().Len(events, 4)
	s.requireEvent(events[0], audit.EventRoleCreate, audit.ActionCreate, created.ID,
		audit.OutcomeSucceeded, audit.Ref{Type: audit.TargetTypeRole, ID: created.ID})
	for i, subject := range []audit.Ref{
		{Type: audit.TargetTypeUser, ID: testUserID1},
		{Type: audit.TargetTypeGroup, ID: testAuditGroupID},
		{Type: audit.TargetTypeAgent, ID: testAuditAgentID},
	} {
		s.requireEvent(events[i+1], audit.EventRoleAssignmentAdd, audit.ActionAdd, created.ID,
			audit.OutcomeSucceeded, subject)
	}
}

// A create that fails is one event, naming no role, and no assignee is recorded as added.
func (s *RoleServiceAuditTestSuite) TestCreateRole_PublishesOneEventWhenItFails() {
	cases := []struct {
		name    string
		request func() RoleCreationDetail
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the request has no name",
			request: func() RoleCreationDetail {
				request := auditedRoleRequest()
				request.Name = ""
				return request
			},
			arrange: func() {},
			outcome: audit.OutcomeFailed("rol_1001"),
		},
		{
			name:    "the store fails",
			request: auditedRoleRequest,
			arrange: func() {
				s.arrangeAuditedRoleCreate()
				s.mockStore.On("CreateRole", mock.Anything, mock.Anything, mock.Anything).
					Return(errors.New("store failure")).Once()
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			tc.arrange()

			created, svcErr := s.service.CreateRole(adminContext(), tc.request())

			s.Nil(created)
			s.Require().NotNil(svcErr)
			events := s.published()
			s.Require().Len(events, 1)
			s.requireEvent(events[0], audit.EventRoleCreate, audit.ActionCreate, "", tc.outcome, audit.Ref{})
		})
	}
}

// A caller that does not hold what the role confers is refused inside the service, which is recorded as
// denied. A refused create made no role, so the event names none.
func (s *RoleServiceAuditTestSuite) TestCreateRole_PublishesARefusedGrantAsDenied() {
	service := s.service
	s.expectPermissionGrant(&sysauthz.ErrorGrantNotPermitted)
	s.mockResourceService.On("ValidatePermissions", mock.Anything, "rs1", []string{testAuditPermission}).
		Return([]string{}, nil).Once()
	s.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").
		Return(oupkg.OrganizationUnit{ID: "ou1"}, nil).Once()

	_, svcErr := service.CreateRole(adminContext(), auditedRoleRequest())

	s.Equal(&sysauthz.ErrorGrantNotPermitted, svcErr)
	events := s.published()
	s.Require().Len(events, 1)
	s.requireEvent(events[0], audit.EventRoleCreate, audit.ActionCreate, "",
		audit.OutcomeDenied("forbidden"), audit.Ref{})
}

func (s *RoleServiceAuditTestSuite) TestUpdateRoleWithPermissions_PublishesOneEventPerAttempt() {
	request := RoleUpdateDetail{
		Name:        testAuditRoleName,
		Description: testAuditDescription,
		OUID:        "ou1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs1", Permissions: []string{testAuditPermission}}},
	}
	role1 := audit.Ref{Type: audit.TargetTypeRole, ID: "role1"}
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
		subject audit.Ref
	}{
		{
			name: "the role is updated",
			arrange: func() {
				s.mockResourceService.On("ValidatePermissions", mock.Anything, "rs1",
					[]string{testAuditPermission}).Return([]string{}, nil).Once()
				s.mockStore.On("GetRole", mock.Anything, "role1").
					Return(RoleWithPermissions{ID: "role1", OUID: "ou1"}, nil).Once()
				s.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou1").
					Return(oupkg.OrganizationUnit{ID: "ou1"}, nil).Once()
				s.mockStore.On("CheckRoleNameExistsExcludingID", mock.Anything, "ou1", testAuditRoleName,
					"role1").Return(false, nil).Once()
				s.mockSharingService.EXPECT().ListVisibleResourceIDs(mock.Anything, RoleSharingType, "ou1").
					Return(nil, nil).Once()
				s.mockStore.On("UpdateRole", mock.Anything, "role1", mock.Anything).Return(nil).Once()
				s.expectPermissionGrant(nil)
			},
			outcome: audit.OutcomeSucceeded,
			subject: role1,
		},
		{
			name: "the role does not exist",
			arrange: func() {
				s.mockStore.On("GetRole", mock.Anything, "role1").
					Return(RoleWithPermissions{}, ErrRoleNotFound).Once()
			},
			outcome: audit.OutcomeFailed("rol_1003"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			tc.arrange()

			_, svcErr := s.service.UpdateRoleWithPermissions(adminContext(), "role1", request)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			events := s.published()
			s.Require().Len(events, 1)
			s.requireEvent(events[0], audit.EventRoleUpdate, audit.ActionUpdate, "role1", tc.outcome, tc.subject)
		})
	}
}

// Deleting a role takes its assignments with it, recorded as the delete alone.
func (s *RoleServiceAuditTestSuite) TestDeleteRole_PublishesOneEventPerAttempt() {
	role1 := audit.Ref{Type: audit.TargetTypeRole, ID: "role1"}
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
		subject audit.Ref
	}{
		{
			name: "the role is deleted with its assignments",
			arrange: func() {
				s.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(true, nil).Once()
				s.mockStore.On("ListMandatedAssignmentsForRole", mock.Anything, "role1", "").
					Return([]RoleAssignment{}, nil).Once()
				s.mockSharingService.EXPECT().DeletePoliciesForResource(mock.Anything, RoleSharingType,
					"role1").Return(nil).Once()
				s.mockStore.On("DeleteAssignmentsByRoleID", mock.Anything, "role1").Return(nil).Once()
				s.mockStore.On("DeleteRole", mock.Anything, "role1").Return(nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
			subject: role1,
		},
		{
			name: "the role cannot be read",
			arrange: func() {
				s.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(false, errors.New("store failure")).Once()
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			tc.arrange()

			svcErr := s.service.DeleteRole(adminContext(), "role1")

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			events := s.published()
			s.Require().Len(events, 1)
			s.requireEvent(events[0], audit.EventRoleDelete, audit.ActionDelete, "role1", tc.outcome, tc.subject)
		})
	}
}

// A delete of a role that does not exist succeeds without an event, so no caller can record the deletion
// of an ID it made up.
func (s *RoleServiceAuditTestSuite) TestDeleteRole_PublishesNothingWhenTheRoleIsMissing() {
	s.mockStore.On("IsRoleExist", mock.Anything, "role1").Return(false, nil).Once()

	svcErr := s.service.DeleteRole(adminContext(), "role1")

	s.Nil(svcErr)
	s.Empty(s.published())
}

// A bus that cannot take the events never fails the change they describe.
func (s *RoleServiceAuditTestSuite) TestCreateRole_SucceedsWhenTheBusRefusesTheEvents() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Times(4)
	service := s.newRoleService(failing)
	s.arrangeAuditedRoleCreate()
	s.mockStore.On("CreateRole", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	created, svcErr := service.CreateRole(adminContext(), auditedRoleRequest())

	s.Nil(svcErr)
	s.NotEmpty(created.ID)
}

// The addition of several assignees is one event per assignee, each naming who gained the role.
func (s *RoleServiceAuditTestSuite) TestAddAssignments_PublishesEachAddedAssignee() {
	s.mockStore.On("GetRole", mock.Anything, "role1").
		Return(RoleWithPermissions{ID: "role1", OUID: testOwnerOU}, nil).Once()
	s.mockEntityService.On("GetEntitiesByIDs", mock.Anything, mock.Anything).Return([]providers.Entity{
		ownedUser(), {ID: testAuditAgentID, Category: providers.EntityCategoryAgent, OUID: testOwnerOU},
	}, nil).Once()
	s.mockStore.On("AddAssignments", mock.Anything, "role1", mock.Anything).Return(nil).Once()
	s.expectMembershipGrants("role1", nil, 2)

	svcErr := s.assignmentService.AddAssignments(adminContext(), "role1", "", []RoleAssignment{
		{ID: testUserID1, Type: AssigneeTypeUser},
		{ID: testAuditAgentID, Type: AssigneeTypeAgent},
	})

	s.Require().Nil(svcErr)
	events := s.published()
	s.Require().Len(events, 2)
	s.requireEvent(events[0], audit.EventRoleAssignmentAdd, audit.ActionAdd, "role1",
		audit.OutcomeSucceeded, audit.Ref{Type: audit.TargetTypeUser, ID: testUserID1})
	s.requireEvent(events[1], audit.EventRoleAssignmentAdd, audit.ActionAdd, "role1",
		audit.OutcomeSucceeded, audit.Ref{Type: audit.TargetTypeAgent, ID: testAuditAgentID})
}

// An addition that fails or is refused is one event for the call, naming the role only: no assignee
// gained it.
func (s *RoleServiceAuditTestSuite) TestAddAssignments_PublishesOneEventWhenItFails() {
	two := []RoleAssignment{{ID: testUserID1, Type: AssigneeTypeUser}, {ID: "user2", Type: AssigneeTypeUser}}
	cases := []struct {
		name    string
		arrange func() RoleAssignmentServiceInterface
		outcome audit.Outcome
	}{
		{
			name: "the role does not exist",
			arrange: func() RoleAssignmentServiceInterface {
				s.mockStore.On("GetRole", mock.Anything, "role1").Return(RoleWithPermissions{}, ErrRoleNotFound).Once()
				s.expectMembershipGrants("role1", nil, 1)
				return s.assignmentService
			},
			outcome: audit.OutcomeFailed("rol_1003"),
		},
		{
			name: "the store fails",
			arrange: func() RoleAssignmentServiceInterface {
				s.mockStore.On("GetRole", mock.Anything, "role1").
					Return(RoleWithPermissions{ID: "role1", OUID: testOwnerOU}, nil).Once()
				s.mockEntityService.On("GetEntitiesByIDs", mock.Anything, mock.Anything).Return(
					[]providers.Entity{ownedUser(), {ID: "user2", Category: providers.EntityCategoryUser,
						OUID: testOwnerOU}}, nil).Once()
				s.mockStore.On("AddAssignments", mock.Anything, "role1", mock.Anything).
					Return(errors.New("store failure")).Once()
				s.expectMembershipGrants("role1", nil, 2)
				return s.assignmentService
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
		{
			name: "the caller may not grant the role",
			arrange: func() RoleAssignmentServiceInterface {
				s.expectMembershipGrants("role1", &sysauthz.ErrorGrantNotPermitted, 1)
				return s.assignmentService
			},
			outcome: audit.OutcomeDenied("forbidden"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			service := tc.arrange()

			svcErr := service.AddAssignments(adminContext(), "role1", "", two)

			s.Require().NotNil(svcErr)
			events := s.published()
			s.Require().Len(events, 1)
			s.requireEvent(events[0], audit.EventRoleAssignmentAdd, audit.ActionAdd, "role1", tc.outcome,
				audit.Ref{})
		})
	}
}

func (s *RoleServiceAuditTestSuite) TestRemoveAssignments_PublishesEachRemovedAssignee() {
	s.mockStore.On("GetRole", mock.Anything, "role1").
		Return(RoleWithPermissions{ID: "role1", OUID: testOwnerOU}, nil).Once()
	s.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{testUserID1}).
		Return([]providers.Entity{ownedUser()}, nil).Once()
	s.mockStore.On("GetDirectAssignments", mock.Anything, "role1", testOwnerOU, []string{testUserID1}).
		Return([]RoleAssignment{{ID: testUserID1, Type: assigneeTypeEntity, OUID: testOwnerOU}}, nil).Once()
	s.mockStore.On("ListMandatedAssignmentsForRole", mock.Anything, "role1", testOwnerOU).
		Return([]RoleAssignment{}, nil).Once()
	s.mockStore.On("RemoveAssignments", mock.Anything, "role1", mock.Anything).Return(nil).Once()
	s.expectMembershipGrants("role1", nil, 2)

	svcErr := s.assignmentService.RemoveAssignments(adminContext(), "role1", "",
		[]RoleAssignment{{ID: testUserID1, Type: AssigneeTypeUser}})

	s.Require().Nil(svcErr)
	events := s.published()
	s.Require().Len(events, 1)
	s.requireEvent(events[0], audit.EventRoleAssignmentRemove, audit.ActionRemove, "role1",
		audit.OutcomeSucceeded, audit.Ref{Type: audit.TargetTypeUser, ID: testUserID1})
}

func (s *RoleServiceAuditTestSuite) TestRemoveAssignments_PublishesOneEventWhenItFails() {
	s.expectMembershipGrants("role1", nil, 1)

	svcErr := s.assignmentService.RemoveAssignments(adminContext(), "role1", "", nil)

	s.Equal(&ErrorEmptyAssignments, svcErr)
	events := s.published()
	s.Require().Len(events, 1)
	s.requireEvent(events[0], audit.EventRoleAssignmentRemove, audit.ActionRemove, "role1",
		audit.OutcomeFailed("rol_1010"), audit.Ref{})
}

// Assigning to several roles at once is one event per role and assignee, after all of them committed:
// none is published from inside the transaction.
func (s *RoleServiceAuditTestSuite) TestAddAssigneesToRoles_PublishesEachRoleAndAssignee() {
	// A role named twice is assigned twice. The assignee is looked up once to place it in its
	// organization unit and again for each assignment, which is checked before and inside the transaction.
	s.mockStore.On("GetRole", mock.Anything, "role1").
		Return(RoleWithPermissions{ID: "role1", OUID: testOwnerOU}, nil).Times(2)
	s.mockStore.On("GetRole", mock.Anything, "role2").
		Return(RoleWithPermissions{ID: "role2", OUID: testOwnerOU}, nil).Once()
	s.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{testUserID1}).
		Return([]providers.Entity{ownedUser()}, nil).Times(4)
	s.mockStore.On("AddAssignments", mock.Anything, "role1", mock.Anything).Return(nil).Times(2)
	s.mockStore.On("AddAssignments", mock.Anything, "role2", mock.Anything).Return(nil).Once()
	s.expectMembershipGrants("role1", nil, 4)
	s.expectMembershipGrants("role2", nil, 2)

	svcErr := s.assignmentService.AddAssigneesToRoles(adminContext(),
		[]RoleAssignment{{ID: testUserID1, Type: AssigneeTypeUser}}, []string{"role1", "role2", "role1"})

	s.Require().Nil(svcErr)
	events := s.published()
	s.Require().Len(events, 2)
	user := audit.Ref{Type: audit.TargetTypeUser, ID: testUserID1}
	s.requireEvent(events[0], audit.EventRoleAssignmentAdd, audit.ActionAdd, "role1",
		audit.OutcomeSucceeded, user)
	s.requireEvent(events[1], audit.EventRoleAssignmentAdd, audit.ActionAdd, "role2",
		audit.OutcomeSucceeded, user)
}

// A refusal for one role rolls back every role, so each is recorded as failed and no assignee as added.
func (s *RoleServiceAuditTestSuite) TestAddAssigneesToRoles_PublishesEachRoleWhenItFails() {
	s.mockStore.On("GetRole", mock.Anything, "role1").
		Return(RoleWithPermissions{ID: "role1", OUID: testOwnerOU}, nil).Once()
	s.mockStore.On("GetRole", mock.Anything, "role2").Return(RoleWithPermissions{}, ErrRoleNotFound).Once()
	s.mockEntityService.On("GetEntitiesByIDs", mock.Anything, []string{testUserID1}).
		Return([]providers.Entity{ownedUser()}, nil).Times(2)
	s.mockStore.On("AddAssignments", mock.Anything, "role1", mock.Anything).Return(nil).Once()
	s.expectMembershipGrants("role1", nil, 2)
	s.expectMembershipGrants("role2", nil, 1)

	svcErr := s.assignmentService.AddAssigneesToRoles(adminContext(),
		[]RoleAssignment{{ID: testUserID1, Type: AssigneeTypeUser}}, []string{"role1", "role2"})

	s.Equal(&ErrorRoleNotFound, svcErr)
	events := s.published()
	s.Require().Len(events, 2)
	failed := audit.OutcomeFailed("rol_1003")
	s.requireEvent(events[0], audit.EventRoleAssignmentAdd, audit.ActionAdd, "role1", failed, audit.Ref{})
	s.requireEvent(events[1], audit.EventRoleAssignmentAdd, audit.ActionAdd, "role2", failed, audit.Ref{})
}

func (s *RoleServiceAuditTestSuite) TestInitialize_RefusesANilEventBus() {
	roleService, assignmentService, resolver, exporter, err := Initialize(http.NewServeMux(), nil, nil, nil, nil,
		nil, nil, nil, nil, nil)

	s.Require().ErrorIs(err, errNilEventBus)
	s.Require().Nil(roleService)
	s.Require().Nil(assignmentService)
	s.Require().Nil(resolver)
	s.Require().Nil(exporter)
}
