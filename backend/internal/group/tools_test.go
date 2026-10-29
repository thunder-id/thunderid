// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/internal/system/mcp/tool"
)

type GroupToolsTestSuite struct {
	suite.Suite
}

func TestGroupToolsTestSuite(t *testing.T) {
	suite.Run(t, new(GroupToolsTestSuite))
}

func (suite *GroupToolsTestSuite) TestRegisterMCPTools() {
	mockService := NewGroupServiceInterfaceMock(suite.T())
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "test-server",
		Version: "1.0.0",
	}, nil)

	registerMCPTools(server, mockService)
	assert.NotNil(suite.T(), server)
}

func (suite *GroupToolsTestSuite) TestCreateGroup_Success() {
	mockService := NewGroupServiceInterfaceMock(suite.T())
	tools := &groupTools{groupService: mockService}

	input := createGroupInput{
		Name:        "Engineering",
		Description: "Engineering Team",
		OUID:        "ou123",
		Members: []Member{
			{ID: "user1", Type: MemberTypeUser},
		},
	}

	expectedGroup := &Group{
		ID:          "grp123",
		Name:        "Engineering",
		Description: "Engineering Team",
		OUID:        "ou123",
		Members: []Member{
			{ID: "user1", Type: MemberTypeUser},
		},
	}

	mockService.On("CreateGroup", mock.Anything, CreateGroupRequest{
		Name:        input.Name,
		Description: input.Description,
		OUID:        input.OUID,
		Members:     input.Members,
	}).Return(expectedGroup, nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.createGroup(ctx, req, input)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), expectedGroup, output)

	mockService.AssertExpectations(suite.T())
}

func (suite *GroupToolsTestSuite) TestCreateGroup_Error() {
	mockService := NewGroupServiceInterfaceMock(suite.T())
	tools := &groupTools{groupService: mockService}

	input := createGroupInput{
		Name: "Invalid Group",
		OUID: "ou123",
	}

	mockService.On("CreateGroup", mock.Anything, mock.Anything).
		Return(nil, &tidcommon.ServiceError{
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "failed to create group error"},
		})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.createGroup(ctx, req, input)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), output)
	assert.Contains(suite.T(), err.Error(), "failed to create group")

	mockService.AssertExpectations(suite.T())
}

func (suite *GroupToolsTestSuite) TestGetGroup_Success() {
	mockService := NewGroupServiceInterfaceMock(suite.T())
	tools := &groupTools{groupService: mockService}

	input := tool.IDInput{ID: "grp123"}
	expectedGroup := &Group{
		ID:   "grp123",
		Name: "Engineering",
		OUID: "ou123",
	}

	mockService.On("GetGroup", mock.Anything, "grp123", true).Return(expectedGroup, nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.getGroup(ctx, req, input)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), expectedGroup, output)

	mockService.AssertExpectations(suite.T())
}

func (suite *GroupToolsTestSuite) TestGetGroup_Error() {
	mockService := NewGroupServiceInterfaceMock(suite.T())
	tools := &groupTools{groupService: mockService}

	input := tool.IDInput{ID: "grp999"}

	mockService.On("GetGroup", mock.Anything, "grp999", true).
		Return(nil, &tidcommon.ServiceError{
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "group not found"},
		})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.getGroup(ctx, req, input)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), output)
	assert.Contains(suite.T(), err.Error(), "failed to get group")

	mockService.AssertExpectations(suite.T())
}

func (suite *GroupToolsTestSuite) TestListGroups_Success() {
	mockService := NewGroupServiceInterfaceMock(suite.T())
	tools := &groupTools{groupService: mockService}

	input := listGroupsInput{
		PaginationInput: tool.PaginationInput{Limit: 10, Offset: 0},
		IncludeDisplay:  true,
	}

	expectedResp := &GroupListResponse{
		TotalResults: 1,
		Count:        1,
		Groups: []GroupBasic{
			{ID: "grp123", Name: "Engineering", OUID: "ou123"},
		},
	}

	mockService.On("GetGroupList", mock.Anything, 10, 0, true).Return(expectedResp, nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.listGroups(ctx, req, input)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), expectedResp, output)

	mockService.AssertExpectations(suite.T())
}

func (suite *GroupToolsTestSuite) TestListGroups_DefaultLimit() {
	mockService := NewGroupServiceInterfaceMock(suite.T())
	tools := &groupTools{groupService: mockService}

	input := listGroupsInput{
		PaginationInput: tool.PaginationInput{Limit: 0, Offset: 0},
		IncludeDisplay:  false,
	}

	expectedResp := &GroupListResponse{
		TotalResults: 0,
		Groups:       []GroupBasic{},
	}

	mockService.On("GetGroupList", mock.Anything, mock.AnythingOfType("int"), 0, false).Return(expectedResp, nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.listGroups(ctx, req, input)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), expectedResp, output)

	mockService.AssertExpectations(suite.T())
}

func (suite *GroupToolsTestSuite) TestListGroups_Error() {
	mockService := NewGroupServiceInterfaceMock(suite.T())
	tools := &groupTools{groupService: mockService}

	input := listGroupsInput{
		PaginationInput: tool.PaginationInput{Limit: 10, Offset: 0},
	}

	mockService.On("GetGroupList", mock.Anything, 10, 0, false).
		Return(nil, &tidcommon.ServiceError{
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "db error"},
		})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.listGroups(ctx, req, input)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), output)
	assert.Contains(suite.T(), err.Error(), "failed to list groups")

	mockService.AssertExpectations(suite.T())
}

func (suite *GroupToolsTestSuite) TestUpdateGroup_Success() {
	mockService := NewGroupServiceInterfaceMock(suite.T())
	tools := &groupTools{groupService: mockService}

	input := updateGroupInput{
		ID:          "grp123",
		Name:        "Engineering Updated",
		Description: "Updated desc",
		OUID:        "ou123",
	}

	expectedGroup := &Group{
		ID:          "grp123",
		Name:        "Engineering Updated",
		Description: "Updated desc",
		OUID:        "ou123",
	}

	mockService.On("UpdateGroup", mock.Anything, "grp123", UpdateGroupRequest{
		Name:        input.Name,
		Description: input.Description,
		OUID:        input.OUID,
	}).Return(expectedGroup, nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.updateGroup(ctx, req, input)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), expectedGroup, output)

	mockService.AssertExpectations(suite.T())
}

func (suite *GroupToolsTestSuite) TestUpdateGroup_Error() {
	mockService := NewGroupServiceInterfaceMock(suite.T())
	tools := &groupTools{groupService: mockService}

	input := updateGroupInput{
		ID:   "grp123",
		Name: "Updated Name",
		OUID: "ou123",
	}

	mockService.On("UpdateGroup", mock.Anything, "grp123", mock.Anything).
		Return(nil, &tidcommon.ServiceError{
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "update error"},
		})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.updateGroup(ctx, req, input)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), output)
	assert.Contains(suite.T(), err.Error(), "failed to update group")

	mockService.AssertExpectations(suite.T())
}

func (suite *GroupToolsTestSuite) TestDeleteGroup_Success() {
	mockService := NewGroupServiceInterfaceMock(suite.T())
	tools := &groupTools{groupService: mockService}

	input := tool.IDInput{ID: "grp123"}

	mockService.On("DeleteGroup", mock.Anything, "grp123").Return(nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.deleteGroup(ctx, req, input)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), output)
	assert.True(suite.T(), output.Success)

	mockService.AssertExpectations(suite.T())
}

func (suite *GroupToolsTestSuite) TestDeleteGroup_Error() {
	mockService := NewGroupServiceInterfaceMock(suite.T())
	tools := &groupTools{groupService: mockService}

	input := tool.IDInput{ID: "grp123"}

	mockService.On("DeleteGroup", mock.Anything, "grp123").
		Return(&tidcommon.ServiceError{
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "delete error"},
		})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.deleteGroup(ctx, req, input)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), output)
	assert.Contains(suite.T(), err.Error(), "failed to delete group")

	mockService.AssertExpectations(suite.T())
}

func (suite *GroupToolsTestSuite) TestAddGroupMember_Success() {
	mockService := NewGroupServiceInterfaceMock(suite.T())
	tools := &groupTools{groupService: mockService}

	input := groupMemberInput{
		GroupID: "grp123",
		Members: []Member{
			{ID: "user2", Type: MemberTypeUser},
		},
	}

	expectedGroup := &Group{
		ID:   "grp123",
		Name: "Engineering",
		Members: []Member{
			{ID: "user1", Type: MemberTypeUser},
			{ID: "user2", Type: MemberTypeUser},
		},
	}

	mockService.On("AddGroupMembers", mock.Anything, "grp123", input.Members).
		Return(expectedGroup, nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.addGroupMember(ctx, req, input)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), expectedGroup, output)

	mockService.AssertExpectations(suite.T())
}

func (suite *GroupToolsTestSuite) TestAddGroupMember_Error() {
	mockService := NewGroupServiceInterfaceMock(suite.T())
	tools := &groupTools{groupService: mockService}

	input := groupMemberInput{
		GroupID: "grp123",
		Members: []Member{
			{ID: "user2", Type: MemberTypeUser},
		},
	}

	mockService.On("AddGroupMembers", mock.Anything, "grp123", input.Members).
		Return(nil, &tidcommon.ServiceError{
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "add members error"},
		})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.addGroupMember(ctx, req, input)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), output)
	assert.Contains(suite.T(), err.Error(), "failed to add group members")

	mockService.AssertExpectations(suite.T())
}

func (suite *GroupToolsTestSuite) TestRemoveGroupMember_Success() {
	mockService := NewGroupServiceInterfaceMock(suite.T())
	tools := &groupTools{groupService: mockService}

	input := groupMemberInput{
		GroupID: "grp123",
		Members: []Member{
			{ID: "user2", Type: MemberTypeUser},
		},
	}

	expectedGroup := &Group{
		ID:   "grp123",
		Name: "Engineering",
		Members: []Member{
			{ID: "user1", Type: MemberTypeUser},
		},
	}

	mockService.On("RemoveGroupMembers", mock.Anything, "grp123", input.Members).
		Return(expectedGroup, nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.removeGroupMember(ctx, req, input)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), expectedGroup, output)

	mockService.AssertExpectations(suite.T())
}

func (suite *GroupToolsTestSuite) TestRemoveGroupMember_Error() {
	mockService := NewGroupServiceInterfaceMock(suite.T())
	tools := &groupTools{groupService: mockService}

	input := groupMemberInput{
		GroupID: "grp123",
		Members: []Member{
			{ID: "user2", Type: MemberTypeUser},
		},
	}

	mockService.On("RemoveGroupMembers", mock.Anything, "grp123", input.Members).
		Return(nil, &tidcommon.ServiceError{
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "remove members error"},
		})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.removeGroupMember(ctx, req, input)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), output)
	assert.Contains(suite.T(), err.Error(), "failed to remove group members")

	mockService.AssertExpectations(suite.T())
}

func (suite *GroupToolsTestSuite) TestSchemas() {
	assert.NotNil(suite.T(), getCreateGroupSchema())
	assert.NotNil(suite.T(), getGetGroupSchema())
	assert.NotNil(suite.T(), getListGroupsSchema())
	assert.NotNil(suite.T(), getUpdateGroupSchema())
	assert.NotNil(suite.T(), getDeleteGroupSchema())
	assert.NotNil(suite.T(), getAddGroupMemberSchema())
	assert.NotNil(suite.T(), getRemoveGroupMemberSchema())
}
