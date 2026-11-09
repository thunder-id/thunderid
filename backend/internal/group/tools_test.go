// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

type GroupToolsTestSuite struct {
	suite.Suite
	groups   *GroupServiceInterfaceMock
	entities *entitymock.EntityServiceInterfaceMock
	ous      *oumock.OrganizationUnitServiceInterfaceMock
	session  *mcp.ClientSession
}

func TestGroupToolsTestSuite(t *testing.T) {
	suite.Run(t, new(GroupToolsTestSuite))
}

func (s *GroupToolsTestSuite) SetupTest() {
	s.connect(true)
}

// connect registers the tools over mocked services and connects an in-memory MCP client. revokes is true on a
// server that issues tokens, as a data plane does.
func (s *GroupToolsTestSuite) connect(revokes bool) {
	s.groups = NewGroupServiceInterfaceMock(s.T())
	s.entities = entitymock.NewEntityServiceInterfaceMock(s.T())
	s.ous = oumock.NewOrganizationUnitServiceInterfaceMock(s.T())

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	RegisterMCPTools(server, s.groups, s.entities, s.ous, revokes)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	s.session, err = client.Connect(context.Background(), clientTransport, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = s.session.Close() })
}

func (s *GroupToolsTestSuite) call(name string, args map[string]any) *mcp.CallToolResult {
	result, err := s.session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	s.Require().NoError(err)
	return result
}

func (s *GroupToolsTestSuite) structured(result *mcp.CallToolResult, out any) {
	s.Require().False(result.IsError, "tool returned an error: %v", result.Content)
	raw, err := json.Marshal(result.StructuredContent)
	s.Require().NoError(err)
	s.Require().NoError(json.Unmarshal(raw, out))
}

func (s *GroupToolsTestSuite) errorText(result *mcp.CallToolResult) string {
	s.Require().True(result.IsError)
	s.Require().NotEmpty(result.Content)
	text, ok := result.Content[0].(*mcp.TextContent)
	s.Require().True(ok)
	return text.Text
}

// readable makes the group readable by the caller.
func (s *GroupToolsTestSuite) readable(id, name, ouID string) {
	s.groups.On("GetGroup", mock.Anything, id, false).
		Return(&Group{ID: id, Name: name, OUID: ouID}, (*tidcommon.ServiceError)(nil)).Once()
}

func (s *GroupToolsTestSuite) TestRegisterMCPTools() {
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
	s.True(tools["list_groups"].Annotations.ReadOnlyHint)
	s.True(tools["get_group"].Annotations.ReadOnlyHint)
	s.False(*tools["create_group"].Annotations.DestructiveHint)
	update := tools["update_group_memberships"].Annotations
	s.True(*update.DestructiveHint)
	s.True(update.IdempotentHint)

	s.Contains(s.errorText(s.call("update_group_memberships", map[string]any{
		"members": []any{map[string]any{"id": "u1", "type": "team"}}, "add": []string{"g1"}})), "type")
}

func (s *GroupToolsTestSuite) TestListGroups() {
	s.Run("by organization unit", func() {
		s.groups.On("GetGroupList", mock.Anything, 30, 0, map[string]interface{}{"ouId": "ou-1"}, false).
			Return(&GroupListResponse{TotalResults: 5, Groups: []GroupBasic{
				{ID: "g1", Name: "Support", OUID: "ou-1", IsReadOnly: true}}}, (*tidcommon.ServiceError)(nil)).Once()
		var out groupListMCPResponse
		s.structured(s.call("list_groups", map[string]any{"ouId": "ou-1"}), &out)
		s.Equal(5, out.TotalResults)
		s.True(out.Groups[0].ReadOnly)
		s.Nil(out.Groups[0].Direct)
	})

	s.Run("by project", func() {
		s.groups.On("GetGroupListByProject", mock.Anything, "project-1", 30, 0, map[string]interface{}(nil), false).
			Return(&GroupListResponse{TotalResults: 1, Groups: []GroupBasic{
				{ID: "g1", Name: "Shoppers", OUID: "ou-1", ProjectID: "project-1"}}},
				(*tidcommon.ServiceError)(nil)).Once()
		var out groupListMCPResponse
		s.structured(s.call("list_groups", map[string]any{"projectId": "project-1"}), &out)
		s.Equal(1, out.TotalResults)
		s.Equal("project-1", out.Groups[0].ProjectID)
	})

	s.Run("by project, refused alongside a name", func() {
		s.Contains(s.errorText(s.call("list_groups", map[string]any{"projectId": "project-1", "name": "Support"})),
			"projectId can't be combined")
	})

	s.Run("by member, direct and nested, leaving out what the caller can't read", func() {
		s.entities.On("GetTransitiveEntityGroups", mock.Anything, "u1").Return([]providers.EntityGroup{
			{ID: "g1"}, {ID: "g2"}, {ID: "g3"}}, nil).Once()
		s.readable("g1", "Support", "ou-1")
		s.readable("g2", "Customer Care", "ou-1")
		s.groups.On("GetGroup", mock.Anything, "g3", false).Return((*Group)(nil),
			&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "GRP-403"}).Once()
		s.groups.On("GetDirectGroupMember", mock.Anything, "g1", "u1").
			Return(&Member{ID: "u1", Type: MemberTypeUser}, (*tidcommon.ServiceError)(nil)).Once()
		s.groups.On("GetDirectGroupMember", mock.Anything, "g2", "u1").
			Return((*Member)(nil), (*tidcommon.ServiceError)(nil)).Once()

		var out groupListMCPResponse
		s.structured(s.call("list_groups", map[string]any{"member": map[string]any{"id": "u1", "type": "user"}}),
			&out)
		s.Require().Equal(2, out.TotalResults)
		s.Equal("Customer Care", out.Groups[0].Name)
		s.False(*out.Groups[0].Direct, "u1 is in Customer Care through a nested group")
		s.Equal("Support", out.Groups[1].Name)
		s.True(*out.Groups[1].Direct)
	})

	s.Run("a group member, narrowed by name and unit", func() {
		s.groups.On("GetGroupsByNames", mock.Anything, []string{"Support"}).Return(map[string][]*Group{
			"Support": {{ID: "g1"}, {ID: "g4"}, {ID: "g5"}}}, (*tidcommon.ServiceError)(nil)).Once()
		s.groups.On("GetTransitiveAncestorGroups", mock.Anything, "team-a").
			Return([]string{"g1", "g4"}, (*tidcommon.ServiceError)(nil)).Once()
		s.readable("g1", "Support", "ou-1")
		s.readable("g4", "Support", "ou-2")
		s.groups.On("GetDirectGroupMember", mock.Anything, "g1", "team-a").
			Return(&Member{ID: "team-a", Type: MemberTypeGroup}, (*tidcommon.ServiceError)(nil)).Once()

		var out groupListMCPResponse
		s.structured(s.call("list_groups", map[string]any{"name": "Support", "ouId": "ou-1",
			"member": map[string]any{"id": "team-a", "type": "group"}}), &out)
		s.Require().Equal(1, out.TotalResults)
		s.Equal("g1", out.Groups[0].ID)
	})
}

func (s *GroupToolsTestSuite) TestGetGroup() {
	s.groups.On("GetGroup", mock.Anything, "g1", false).
		Return(&Group{ID: "g1", Name: "Support", OUID: "ou-1"}, (*tidcommon.ServiceError)(nil)).Twice()

	var out groupMCPDetails
	s.structured(s.call("get_group", map[string]any{"groupId": "g1"}), &out)
	s.Equal("Support", out.Name)
	s.Equal([]string{"members"}, out.NotIncluded)
	s.Nil(out.Members)

	// Members come a page at a time: this page starts at the second member.
	s.groups.On("GetGroupMembers", mock.Anything, "g1", 2, 1, true).Return(&MemberListResponse{
		TotalResults: 3, Members: []Member{{ID: "u1", Type: MemberTypeUser, Display: "Bob"},
			{ID: "team-a", Type: MemberTypeGroup, Display: "Team A"}}},
		(*tidcommon.ServiceError)(nil)).Once()
	var withMembers groupMCPDetails
	s.structured(s.call("get_group", map[string]any{"groupId": "g1", "include": []string{"members"},
		"limit": 2, "offset": 1}), &withMembers)
	s.Require().NotNil(withMembers.Members)
	s.Equal([]groupMemberMCPItem{{ID: "u1", Type: "user", Name: "Bob"},
		{ID: "team-a", Type: "group", Name: "Team A"}}, *withMembers.Members)
	s.Equal(3, withMembers.MembersTotal, "the total counts every member, not only this page")
}

func (s *GroupToolsTestSuite) TestCreateGroup() {
	s.Run("in the Default organization unit, with first members", func() {
		s.ous.On("GetOrganizationUnitByPath", mock.Anything, defaultOUHandle).
			Return(oupkg.OrganizationUnit{ID: "ou-default"}, (*tidcommon.ServiceError)(nil)).Once()
		var request CreateGroupRequest
		s.groups.On("CreateGroup", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) { request = args.Get(1).(CreateGroupRequest) }).
			Return(&Group{ID: "g1", Name: "Booking", OUID: "ou-default",
				Members: []Member{{ID: "u1", Type: MemberTypeUser}}}, (*tidcommon.ServiceError)(nil)).Once()

		var out groupMCPDetails
		s.structured(s.call("create_group", map[string]any{"name": "Booking",
			"members": []any{map[string]any{"id": "u1", "type": "user"}}}), &out)
		s.Equal("ou-default", request.OUID)
		s.Equal([]Member{{ID: "u1", Type: MemberTypeUser}}, request.Members)
		s.Equal("g1", out.ID)
		s.Equal(1, out.MembersTotal)
	})

	s.Run("in a project, leaving the organization unit to the service", func() {
		var request CreateGroupRequest
		s.groups.On("CreateGroup", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) { request = args.Get(1).(CreateGroupRequest) }).
			Return(&Group{ID: "g2", Name: "Shoppers", OUID: "ou-system", ProjectID: "project-1"},
				(*tidcommon.ServiceError)(nil)).Once()

		var out groupMCPDetails
		s.structured(s.call("create_group", map[string]any{"name": "Shoppers", "projectId": "project-1"}), &out)
		s.Empty(request.OUID)
		s.Equal("project-1", request.ProjectID)
		s.Equal("project-1", out.ProjectID)
	})

	s.Run("the name is taken in a project's system organization unit", func() {
		s.groups.On("CreateGroup", mock.Anything, mock.Anything).Return((*Group)(nil), &ErrorGroupNameConflict).Once()
		s.ous.On("GetSystemOUID", mock.Anything).Return("ou-system", (*tidcommon.ServiceError)(nil)).Once()
		s.groups.On("GetGroupsByNames", mock.Anything, []string{"Shoppers"}).Return(map[string][]*Group{
			"Shoppers": {{ID: "g8", OUID: "ou-system"}}}, (*tidcommon.ServiceError)(nil)).Once()
		text := s.errorText(s.call("create_group", map[string]any{"name": "Shoppers", "projectId": "project-1"}))
		s.Contains(text, `a group named "Shoppers" already exists in this organization unit (g8)`)
	})

	s.Run("the name is taken in the unit", func() {
		s.groups.On("CreateGroup", mock.Anything, mock.Anything).Return((*Group)(nil), &ErrorGroupNameConflict).Once()
		s.groups.On("GetGroupsByNames", mock.Anything, []string{"Support"}).Return(map[string][]*Group{
			"Support": {{ID: "other", OUID: "ou-9"}, {ID: "g7", OUID: "ou-1"}}}, (*tidcommon.ServiceError)(nil)).Once()
		text := s.errorText(s.call("create_group", map[string]any{"name": "Support", "ouId": "ou-1"}))
		s.Contains(text, `a group named "Support" already exists in this organization unit (g7)`)
		s.Contains(text, "update_group_memberships")
	})

	s.Run("Default organization unit missing", func() {
		s.ous.On("GetOrganizationUnitByPath", mock.Anything, defaultOUHandle).
			Return(oupkg.OrganizationUnit{}, &oupkg.ErrorOrganizationUnitNotFound).Once()
		s.Contains(s.errorText(s.call("create_group", map[string]any{"name": "Booking"})), "give ouId")
	})
}

func (s *GroupToolsTestSuite) TestUpdateGroupMemberships() {
	members := []any{map[string]any{"id": "u1", "type": "user"}}

	s.Run("removes, then adds", func() {
		s.groups.On("GetDirectGroupMember", mock.Anything, "booking", "u1").
			Return(&Member{ID: "u1", Type: MemberTypeUser}, (*tidcommon.ServiceError)(nil)).Once()
		s.groups.On("RemoveGroupMembers", mock.Anything, "booking", []Member{{ID: "u1", Type: MemberTypeUser}}).
			Return(&Group{ID: "booking"}, (*tidcommon.ServiceError)(nil)).Once()
		s.groups.On("AddGroupMembers", mock.Anything, "pricing", []Member{{ID: "u1", Type: MemberTypeUser}}).
			Return(&Group{ID: "pricing"}, (*tidcommon.ServiceError)(nil)).Once()

		var out updateGroupMembershipsMCPResponse
		s.structured(s.call("update_group_memberships", map[string]any{"members": members,
			"add": []string{"pricing"}, "remove": []string{"booking"}}), &out)
		s.Equal([]groupMembershipChangeMCPItem{{GroupID: "booking", MemberID: "u1"}}, out.Removed)
		s.Equal([]groupMembershipChangeMCPItem{{GroupID: "pricing", MemberID: "u1"}}, out.Added)
		s.Equal(revokingRemovalNotice, out.Notice)
	})

	s.Run("a refused removal adds nothing", func() {
		s.connect(true)
		s.groups.On("GetDirectGroupMember", mock.Anything, "booking", "u1").
			Return(&Member{ID: "u1", Type: MemberTypeUser}, (*tidcommon.ServiceError)(nil)).Once()
		s.groups.On("RemoveGroupMembers", mock.Anything, "booking", mock.Anything).
			Return((*Group)(nil), &tidcommon.ServiceError{ErrorDescription: tidcommon.I18nMessage{
				DefaultValue: "refused"}}).Once()
		text := s.errorText(s.call("update_group_memberships", map[string]any{"members": members,
			"add": []string{"pricing"}, "remove": []string{"booking"}}))
		s.Contains(text, "refused")
		s.Contains(text, "Nothing was added")
		s.groups.AssertNotCalled(s.T(), "AddGroupMembers", mock.Anything, mock.Anything, mock.Anything)
	})

	s.Run("a member who isn't directly in the group is refused before anything is removed", func() {
		s.connect(true)
		s.groups.On("GetDirectGroupMember", mock.Anything, "booking", "u1").
			Return((*Member)(nil), (*tidcommon.ServiceError)(nil)).Once()
		text := s.errorText(s.call("update_group_memberships", map[string]any{"members": members,
			"add": []string{"pricing"}, "remove": []string{"booking"}}))
		s.Contains(text, "no user with this ID is directly in the group")
		s.Contains(text, "Nothing was added")
		s.groups.AssertNotCalled(s.T(), "RemoveGroupMembers", mock.Anything, mock.Anything, mock.Anything)
		s.groups.AssertNotCalled(s.T(), "AddGroupMembers", mock.Anything, mock.Anything, mock.Anything)
	})

	s.Run("a membership that can't be read is refused before anything is removed", func() {
		s.connect(true)
		s.groups.On("GetDirectGroupMember", mock.Anything, "booking", "u1").
			Return((*Member)(nil), &tidcommon.ServiceError{
				ErrorDescription: tidcommon.I18nMessage{DefaultValue: "group not found"}}).Once()
		s.Contains(s.errorText(s.call("update_group_memberships", map[string]any{"members": members,
			"remove": []string{"booking"}})), "group not found")
		s.groups.AssertNotCalled(s.T(), "RemoveGroupMembers", mock.Anything, mock.Anything, mock.Anything)
	})

	s.Run("a server that issues no tokens says removals are written directly", func() {
		s.connect(false)
		s.groups.On("GetDirectGroupMember", mock.Anything, "booking", "u1").
			Return(&Member{ID: "u1", Type: MemberTypeUser}, (*tidcommon.ServiceError)(nil)).Once()
		s.groups.On("RemoveGroupMembers", mock.Anything, "booking", []Member{{ID: "u1", Type: MemberTypeUser}}).
			Return(&Group{ID: "booking"}, (*tidcommon.ServiceError)(nil)).Once()
		var out updateGroupMembershipsMCPResponse
		s.structured(s.call("update_group_memberships", map[string]any{"members": members,
			"remove": []string{"booking"}}), &out)
		s.Len(out.Removed, 1)
		s.Empty(out.Added)
		s.Equal(directRemovalNotice, out.Notice)
	})

	s.Run("a member of another type with the same ID is refused", func() {
		s.connect(true)
		s.groups.On("GetDirectGroupMember", mock.Anything, "booking", "u1").
			Return(&Member{ID: "u1", Type: MemberTypeGroup}, (*tidcommon.ServiceError)(nil)).Once()
		s.Contains(s.errorText(s.call("update_group_memberships", map[string]any{"members": members,
			"remove": []string{"booking"}})), "no user with this ID is directly in the group")
		s.groups.AssertNotCalled(s.T(), "RemoveGroupMembers", mock.Anything, mock.Anything, mock.Anything)
	})

	s.Run("needs a change", func() {
		s.connect(true)
		s.Contains(s.errorText(s.call("update_group_memberships", map[string]any{"members": members})),
			"add, remove, or both")
	})
}

// update_group_memberships says what a removal does on this server: a data plane revokes tokens, and a
// control plane writes removals directly.
func (s *GroupToolsTestSuite) TestRemovalNoticeFollowsThePlane() {
	for _, revokes := range []bool{true, false} {
		s.connect(revokes)
		result, err := s.session.ListTools(context.Background(), nil)
		s.Require().NoError(err)
		expected := revokingRemovalNotice
		if !revokes {
			expected = directRemovalNotice
		}
		for _, t := range result.Tools {
			if t.Name == "update_group_memberships" {
				s.Contains(t.Description, expected)
			}
		}
	}
}

// A name or member search is paged like a plain list, and counts every match.
func (s *GroupToolsTestSuite) TestListGroups_SearchIsPaged() {
	s.groups.On("GetGroupsByNames", mock.Anything, []string{"Support"}).Return(map[string][]*Group{
		"Support": {{ID: "g1"}, {ID: "g2"}, {ID: "g3"}},
	}, (*tidcommon.ServiceError)(nil)).Once()
	for _, id := range []string{"g1", "g2", "g3"} {
		s.groups.On("GetGroup", mock.Anything, id, false).
			Return(&Group{ID: id, Name: "Support", OUID: "ou-" + id}, (*tidcommon.ServiceError)(nil)).Once()
	}

	var out groupListMCPResponse
	s.structured(s.call("list_groups", map[string]any{"name": "Support", "limit": 1, "offset": 1}), &out)

	s.Equal(3, out.TotalResults)
	s.Require().Len(out.Groups, 1)
	s.Equal("g2", out.Groups[0].ID)
	s.Contains(s.errorText(s.call("list_groups", map[string]any{"name": "Support", "limit": 500})),
		"limit must be 1 to 100")
}

// Every group tool needs the group permission its API needs, so none falls back to root by accident.
func (s *GroupToolsTestSuite) TestToolsRequireTheGroupAPIPermissions() {
	tools, err := s.session.ListTools(context.Background(), nil)
	s.Require().NoError(err)
	s.Require().NotEmpty(tools.Tools)
	security.ForEachPermissionPlaneForTest(func(plane string) {
		s.Run(plane, func() {
			p := security.GetSystemPermissions()
			for _, registered := range tools.Tools {
				s.Contains([]string{p.GroupView, p.GroupCreate, p.GroupUpdate},
					security.ResolveMCPToolPermission(registered.Name),
					"tool %q must require a group permission", registered.Name)
			}
		})
	})
}
