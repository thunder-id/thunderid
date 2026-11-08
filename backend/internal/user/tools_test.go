// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/group"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/groupmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
	"github.com/thunder-id/thunderid/tests/mocks/rolemock"
)

const toolUserID = "user-1"

type UserToolsTestSuite struct {
	suite.Suite
	users       *UserServiceInterfaceMock
	entities    *entitymock.EntityServiceInterfaceMock
	entityTypes *entitytypemock.EntityTypeServiceInterfaceMock
	roles       *rolemock.RoleServiceInterfaceMock
	assignments *rolemock.RoleAssignmentServiceInterfaceMock
	groups      *groupmock.GroupServiceInterfaceMock
	resources   *resourcemock.ResourceServiceInterfaceMock
	ous         *oumock.OrganizationUnitServiceInterfaceMock
	session     *mcp.ClientSession
}

func TestUserToolsTestSuite(t *testing.T) {
	suite.Run(t, new(UserToolsTestSuite))
}

func (s *UserToolsTestSuite) SetupTest() {
	s.connect(true)
}

// connect registers the tools over mocked services and connects an in-memory MCP client. withCreate
// registers create_user too, as a control plane does.
func (s *UserToolsTestSuite) connect(withCreate bool) {
	s.users = NewUserServiceInterfaceMock(s.T())
	s.entities = entitymock.NewEntityServiceInterfaceMock(s.T())
	s.entityTypes = entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	s.roles = rolemock.NewRoleServiceInterfaceMock(s.T())
	s.assignments = rolemock.NewRoleAssignmentServiceInterfaceMock(s.T())
	s.groups = groupmock.NewGroupServiceInterfaceMock(s.T())
	s.resources = resourcemock.NewResourceServiceInterfaceMock(s.T())
	s.ous = oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	// A person needs a username and an email; the password is a credential.
	s.entityTypes.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "person", mock.Anything).Return(
		[]entitytype.AttributeInfo{
			{Attribute: "username", Required: true},
			{Attribute: "email", Required: true},
			{Attribute: "department"},
			{Attribute: "password", Credential: true},
		}, (*tidcommon.ServiceError)(nil)).Maybe()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	RegisterMCPTools(server, s.users, s.entities, s.entityTypes, s.roles, s.resources)
	if withCreate {
		RegisterCreateUserTool(server, s.users, s.entities, s.entityTypes, s.ous, s.assignments, s.groups)
	}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	s.session, err = client.Connect(context.Background(), clientTransport, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = s.session.Close() })
}

func (s *UserToolsTestSuite) call(name string, args map[string]any) *mcp.CallToolResult {
	result, err := s.session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	s.Require().NoError(err)
	return result
}

func (s *UserToolsTestSuite) structured(result *mcp.CallToolResult, out any) {
	s.Require().False(result.IsError, "tool returned an error: %v", result.Content)
	raw, err := json.Marshal(result.StructuredContent)
	s.Require().NoError(err)
	s.Require().NoError(json.Unmarshal(raw, out))
}

func (s *UserToolsTestSuite) errorText(result *mcp.CallToolResult) string {
	s.Require().True(result.IsError)
	s.Require().NotEmpty(result.Content)
	text, ok := result.Content[0].(*mcp.TextContent)
	s.Require().True(ok)
	return text.Text
}

// alice is a stored person, with a password the tools must never return.
func alice() *providers.User {
	return &providers.User{ID: toolUserID, Type: "person", OUID: "ou-1", Display: "Alice",
		Attributes: json.RawMessage(`{"username":"alice","email":"alice@example.com","department":"sales",` +
			`"password":"stored-hash"}`),
		Status: &providers.AccessStatusReport{Value: providers.AccessStatusActive}}
}

func (s *UserToolsTestSuite) TestRegisterMCPTools() {
	result, err := s.session.ListTools(context.Background(), nil)
	s.Require().NoError(err)
	tools := map[string]*mcp.Tool{}
	for _, t := range result.Tools {
		tools[t.Name] = t
		s.NotContains(t.Description, "—", t.Name)
		s.NotContains(t.Description, "--", t.Name)
		s.False(*t.Annotations.OpenWorldHint, t.Name)
	}
	s.Len(tools, 4)
	s.True(tools["list_users"].Annotations.ReadOnlyHint)
	s.True(tools["get_user"].Annotations.ReadOnlyHint)
	s.True(*tools["update_user"].Annotations.DestructiveHint)
	s.False(*tools["create_user"].Annotations.DestructiveHint)
}

// Without create_user, as on a data plane, the other user tools stand on their own.
func (s *UserToolsTestSuite) TestRegisterMCPTools_WithoutCreate() {
	s.connect(false)
	result, err := s.session.ListTools(context.Background(), nil)
	s.Require().NoError(err)
	names := make([]string, 0, len(result.Tools))
	for _, t := range result.Tools {
		names = append(names, t.Name)
	}
	s.ElementsMatch([]string{"list_users", "get_user", "update_user"}, names)
}

func (s *UserToolsTestSuite) TestListUsers() {
	s.Run("by an attribute", func() {
		s.users.On("GetUserList", mock.Anything, 30, 0, map[string]interface{}{"email": "alice@example.com"}, true).
			Return(&UserListResponse{TotalResults: 1, Users: []providers.User{*alice()}},
				(*tidcommon.ServiceError)(nil)).Once()
		var out userListMCPResponse
		s.structured(s.call("list_users", map[string]any{"attribute": "email", "value": "alice@example.com"}), &out)
		s.Equal(1, out.TotalResults)
		s.Equal(userMCPItem{ID: toolUserID, Name: "Alice", Type: "person", OUID: "ou-1", Status: "ACTIVE"},
			out.Users[0])
	})

	s.Run("by organization unit", func() {
		s.users.On("GetUserList", mock.Anything, 5, 10, map[string]interface{}{"ouId": "ou-1"}, true).
			Return(&UserListResponse{TotalResults: 12}, (*tidcommon.ServiceError)(nil)).Once()
		var out userListMCPResponse
		s.structured(s.call("list_users", map[string]any{"ouId": "ou-1", "limit": 5, "offset": 10}), &out)
		s.Equal(12, out.TotalResults)
	})

	s.Run("refusals", func() {
		s.Contains(s.errorText(s.call("list_users", map[string]any{"attribute": "email"})),
			"attribute and value together")
		s.Contains(s.errorText(s.call("list_users", map[string]any{"attribute": "email", "value": "a",
			"ouId": "ou-1"})), "can't be combined")
	})
}

func (s *UserToolsTestSuite) TestGetUser() {
	s.users.On("GetUser", mock.Anything, toolUserID, true).Return(alice(), (*tidcommon.ServiceError)(nil)).Twice()

	var out userMCPDetails
	s.structured(s.call("get_user", map[string]any{"userId": toolUserID}), &out)
	s.Equal("Alice", out.Name)
	s.Equal(map[string]any{"username": "alice", "email": "alice@example.com", "department": "sales"},
		out.Attributes, "a credential is never returned")
	s.Equal([]string{"groups", "roles", "permissions"}, out.NotIncluded)

	s.entities.On("GetTransitiveEntityGroups", mock.Anything, toolUserID).Return([]providers.EntityGroup{
		{ID: "g1", Name: "Support"}, {ID: "g2", Name: "Staff"}}, nil).Once()
	s.entities.On("GetEntityGroups", mock.Anything, toolUserID, 100, 0).Return([]providers.EntityGroup{
		{ID: "g1", Name: "Support"}}, nil).Once()
	s.roles.On("GetUserRoles", mock.Anything, toolUserID, []string(nil), "").
		Return([]string{"Order Support"}, (*tidcommon.ServiceError)(nil)).Once()
	s.roles.On("GetUserRoles", mock.Anything, toolUserID, []string{"g1", "g2"}, "").
		Return([]string{"Order Support", "Staff"}, (*tidcommon.ServiceError)(nil)).Once()
	var detailed userMCPDetails
	s.structured(s.call("get_user", map[string]any{"userId": toolUserID, "include": []string{"groups", "roles"}}),
		&detailed)
	s.Equal([]userGroupMCPItem{{ID: "g1", Name: "Support", Membership: "direct"},
		{ID: "g2", Name: "Staff", Membership: "inherited"}}, *detailed.Groups)
	s.Equal([]userRoleMCPItem{{Name: "Order Support", Granted: "directly"},
		{Name: "Staff", Granted: "through_group"}}, *detailed.Roles)
	s.Equal([]string{"permissions"}, detailed.NotIncluded)
}

func (s *UserToolsTestSuite) TestUpdateUser() {
	s.Run("merges, removes and keeps credentials out", func() {
		s.users.On("GetUser", mock.Anything, toolUserID, false).Return(alice(), (*tidcommon.ServiceError)(nil)).Once()
		var sent map[string]any
		s.users.On("UpdateUserAttributes", mock.Anything, toolUserID, mock.Anything).
			Run(func(args mock.Arguments) { _ = json.Unmarshal(args.Get(2).(json.RawMessage), &sent) }).
			Return(alice(), (*tidcommon.ServiceError)(nil)).Once()
		var out userMCPDetails
		s.structured(s.call("update_user", map[string]any{"userId": toolUserID,
			"attributes": map[string]any{"department": nil, "title": "Lead"}}), &out)
		s.Equal(map[string]any{"username": "alice", "email": "alice@example.com", "title": "Lead"}, sent,
			"the change is merged, null removes, and the stored password is not sent back")
	})

	s.Run("refusals in one error", func() {
		s.connect(true)
		s.users.On("GetUser", mock.Anything, toolUserID, false).Return(alice(), (*tidcommon.ServiceError)(nil)).Once()
		text := s.errorText(s.call("update_user", map[string]any{"userId": toolUserID,
			"attributes": map[string]any{"email": nil, "password": "new"}}))
		s.Contains(text, "nothing was changed")
		s.Contains(text, "email is required")
		s.Contains(text, "password is a credential")
		s.users.AssertNotCalled(s.T(), "UpdateUserAttributes", mock.Anything, mock.Anything, mock.Anything)
	})
}

func (s *UserToolsTestSuite) TestCreateUser() {
	s.Run("in the Default organization unit, with roles and groups", func() {
		s.ous.On("GetOrganizationUnitByPath", mock.Anything, defaultOUHandle).
			Return(oupkg.OrganizationUnit{ID: "ou-default"}, (*tidcommon.ServiceError)(nil)).Once()
		var created *providers.User
		s.users.On("CreateUser", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) { created = args.Get(1).(*providers.User) }).
			Return(&providers.User{ID: "new-1", Type: "person", OUID: "ou-default"},
				(*tidcommon.ServiceError)(nil)).Once()
		s.assignments.On("AddAssignments", mock.Anything, "r1", "",
			[]role.RoleAssignment{{ID: "new-1", Type: role.AssigneeTypeUser}}).
			Return((*tidcommon.ServiceError)(nil)).Once()
		s.groups.On("AddGroupMembers", mock.Anything, "g1", []group.Member{{ID: "new-1", Type: group.MemberTypeUser}}).
			Return(&group.Group{ID: "g1"}, (*tidcommon.ServiceError)(nil)).Once()
		s.users.On("GetUser", mock.Anything, "new-1", true).Return(&providers.User{ID: "new-1", Type: "person",
			OUID: "ou-default", Display: "Bob"}, (*tidcommon.ServiceError)(nil)).Once()

		var out createUserMCPResponse
		s.structured(s.call("create_user", map[string]any{"userType": "person",
			"attributes": map[string]any{"username": "bob", "email": "bob@example.com"},
			"roles":      []string{"r1"}, "groups": []string{"g1"}}), &out)
		s.Require().NotNil(created)
		s.Equal("ou-default", created.OUID)
		s.JSONEq(`{"username":"bob","email":"bob@example.com"}`, string(created.Attributes))
		s.Equal("new-1", out.User.ID)
		s.Equal([]string{"r1"}, out.RolesGranted)
		s.Equal([]string{"g1"}, out.GroupsJoined)
		s.Contains(out.Notice, "No password")
	})

	s.Run("every problem at once, nothing created", func() {
		s.connect(true)
		text := s.errorText(s.call("create_user", map[string]any{"userType": "person",
			"attributes": map[string]any{"username": "bob", "password": "secret"}}))
		s.Contains(text, "nothing was created")
		s.Contains(text, "email is required")
		s.Contains(text, "password is a credential")
		s.users.AssertNotCalled(s.T(), "CreateUser", mock.Anything, mock.Anything)
	})

	s.Run("an unknown user type", func() {
		s.connect(true)
		s.entityTypes.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "robot", mock.Anything).
			Return([]entitytype.AttributeInfo(nil), &entitytype.ErrorEntityTypeNotFound).Once()
		s.Contains(s.errorText(s.call("create_user", map[string]any{"userType": "robot",
			"attributes": map[string]any{"username": "r"}})), "list_user_types")
	})

	s.Run("a role that can't be granted is reported with what was done", func() {
		s.connect(true)
		s.users.On("CreateUser", mock.Anything, mock.Anything).Return(&providers.User{ID: "new-2", Type: "person"},
			(*tidcommon.ServiceError)(nil)).Once()
		s.assignments.On("AddAssignments", mock.Anything, "r1", "", mock.Anything).
			Return(&tidcommon.ServiceError{
				ErrorDescription: tidcommon.I18nMessage{DefaultValue: "no such role"}}).Once()
		text := s.errorText(s.call("create_user", map[string]any{"userType": "person", "ouId": "ou-1",
			"attributes": map[string]any{"username": "c", "email": "c@example.com"}, "roles": []string{"r1"}}))
		s.Contains(text, "the user was created with ID new-2, but granting role r1 failed: no such role")
	})
}

// A user type that requires a credential can't be created here, since no credential goes through these
// tools, so create_user says so before creating anything.
func (s *UserToolsTestSuite) TestCreateUser_RefusesATypeThatRequiresACredential() {
	s.entityTypes.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "staff", mock.Anything).Return(
		[]entitytype.AttributeInfo{
			{Attribute: "username", Required: true},
			{Attribute: "password", Credential: true, Required: true},
		}, (*tidcommon.ServiceError)(nil)).Once()

	text := s.errorText(s.call("create_user", map[string]any{"userType": "staff",
		"attributes": map[string]any{"username": "carol"}}))

	s.Contains(text, "requires the credential password")
	s.Contains(text, "nothing was created")
	s.users.AssertNotCalled(s.T(), "CreateUser", mock.Anything, mock.Anything)
}

// Every user tool needs the user permission its API needs, so none falls back to root by accident.
func (s *UserToolsTestSuite) TestToolsRequireTheUserAPIPermissions() {
	security.InitSystemPermissions("")
	p := security.GetSystemPermissions()
	tools, err := s.session.ListTools(context.Background(), nil)
	s.Require().NoError(err)
	s.Require().NotEmpty(tools.Tools)
	for _, registered := range tools.Tools {
		s.Contains([]string{p.UserView, p.UserCreate, p.UserUpdate},
			security.ResolveMCPToolPermission(registered.Name),
			"tool %q must require a user permission", registered.Name)
	}
}
