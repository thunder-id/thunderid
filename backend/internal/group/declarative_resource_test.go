/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package group

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// GroupExporterTestSuite contains tests for the groupExporter.
type GroupExporterTestSuite struct {
	suite.Suite
	mockService *GroupServiceInterfaceMock
	exporter    declarativeresource.ResourceExporter
	ctx         context.Context
}

func TestGroupExporterTestSuite(t *testing.T) {
	suite.Run(t, new(GroupExporterTestSuite))
}

func (suite *GroupExporterTestSuite) SetupTest() {
	suite.mockService = NewGroupServiceInterfaceMock(suite.T())
	suite.exporter = newGroupExporter(suite.mockService)
	suite.ctx = context.Background()
}

// Test GetResourceType
func (suite *GroupExporterTestSuite) TestGetResourceType() {
	assert.Equal(suite.T(), resourceTypeGroup, suite.exporter.GetResourceType())
}

// Test GetParameterizerType
func (suite *GroupExporterTestSuite) TestGetParameterizerType() {
	assert.Equal(suite.T(), paramTypeGroup, suite.exporter.GetParameterizerType())
}

// Test GetResourceRules
func (suite *GroupExporterTestSuite) TestGetResourceRules() {
	rules := suite.exporter.GetResourceRules()
	assert.NotNil(suite.T(), rules)
	assert.Empty(suite.T(), rules.Variables)
	assert.Empty(suite.T(), rules.ArrayVariables)
}

// Test GetAllResourceIDs - single page of results
func (suite *GroupExporterTestSuite) TestGetAllResourceIDs_SinglePage() {
	groupList := &GroupListResponse{
		Groups: []GroupBasic{
			{ID: "group1", Name: "Admins", OUID: "ou1"},
			{ID: "group2", Name: "Engineers", OUID: "ou1"},
		},
		TotalResults: 2,
	}
	emptyPage := &GroupListResponse{
		Groups:       []GroupBasic{},
		TotalResults: 2,
	}

	suite.mockService.On("GetGroupList", suite.ctx, serverconst.MaxPageSize, 0, false).Return(groupList, nil)
	suite.mockService.On("GetGroupList", suite.ctx, serverconst.MaxPageSize, 2, false).Return(emptyPage, nil)

	ids, err := suite.exporter.GetAllResourceIDs(suite.ctx)

	suite.Nil(err)
	assert.Len(suite.T(), ids, 2)
	assert.Contains(suite.T(), ids, "group1")
	assert.Contains(suite.T(), ids, "group2")
	suite.mockService.AssertExpectations(suite.T())
}

// Test GetAllResourceIDs - multiple pages of results
func (suite *GroupExporterTestSuite) TestGetAllResourceIDs_MultiplePages() {
	page1 := &GroupListResponse{
		Groups:       []GroupBasic{{ID: "group1", Name: "Admins", OUID: "ou1"}},
		TotalResults: 2,
	}
	page2 := &GroupListResponse{
		Groups:       []GroupBasic{{ID: "group2", Name: "Engineers", OUID: "ou1"}},
		TotalResults: 2,
	}
	emptyPage := &GroupListResponse{
		Groups:       []GroupBasic{},
		TotalResults: 2,
	}

	suite.mockService.On("GetGroupList", suite.ctx, serverconst.MaxPageSize, 0, false).Return(page1, nil)
	suite.mockService.On("GetGroupList", suite.ctx, serverconst.MaxPageSize, 1, false).Return(page2, nil)
	suite.mockService.On("GetGroupList", suite.ctx, serverconst.MaxPageSize, 2, false).Return(emptyPage, nil)

	ids, err := suite.exporter.GetAllResourceIDs(suite.ctx)

	suite.Nil(err)
	assert.Len(suite.T(), ids, 2)
	assert.Contains(suite.T(), ids, "group1")
	assert.Contains(suite.T(), ids, "group2")
	suite.mockService.AssertExpectations(suite.T())
}

// Test GetAllResourceIDs - empty store
func (suite *GroupExporterTestSuite) TestGetAllResourceIDs_Empty() {
	emptyPage := &GroupListResponse{Groups: []GroupBasic{}, TotalResults: 0}
	suite.mockService.On("GetGroupList", suite.ctx, serverconst.MaxPageSize, 0, false).Return(emptyPage, nil)

	ids, err := suite.exporter.GetAllResourceIDs(suite.ctx)

	suite.Nil(err)
	assert.Empty(suite.T(), ids)
	suite.mockService.AssertExpectations(suite.T())
}

// Test GetAllResourceIDs - service error
func (suite *GroupExporterTestSuite) TestGetAllResourceIDs_ServiceError() {
	serviceErr := &serviceerror.ServiceError{Code: "500"}
	suite.mockService.On("GetGroupList", suite.ctx, serverconst.MaxPageSize, 0, false).Return(nil, serviceErr)

	ids, err := suite.exporter.GetAllResourceIDs(suite.ctx)

	suite.NotNil(err)
	assert.Nil(suite.T(), ids)
	assert.Equal(suite.T(), serviceErr, err)
	suite.mockService.AssertExpectations(suite.T())
}

// Test GetResourceByID - success with members
func (suite *GroupExporterTestSuite) TestGetResourceByID_WithMembers() {
	grp := &Group{
		ID:          "group1",
		Name:        "Admins",
		Description: "Admin group",
		OUID:        "ou1",
	}
	membersPage1 := &MemberListResponse{
		Members: []Member{
			{ID: "user1", Type: MemberTypeUser},
			{ID: "group2", Type: MemberTypeGroup},
		},
		TotalResults: 2,
	}
	membersEmpty := &MemberListResponse{Members: []Member{}, TotalResults: 2}

	suite.mockService.On("GetGroup", suite.ctx, "group1", false).Return(grp, nil)
	suite.mockService.On("GetGroupMembers", suite.ctx, "group1", serverconst.MaxPageSize, 0, false).
		Return(membersPage1, nil)
	suite.mockService.On("GetGroupMembers", suite.ctx, "group1", serverconst.MaxPageSize, 2, false).
		Return(membersEmpty, nil)

	resource, name, err := suite.exporter.GetResourceByID(suite.ctx, "group1")

	suite.Nil(err)
	assert.Equal(suite.T(), "Admins", name)
	assert.NotNil(suite.T(), resource)

	exported, ok := resource.(*groupDeclarativeResource)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), "group1", exported.ID)
	assert.Equal(suite.T(), "Admins", exported.Name)
	assert.Equal(suite.T(), "Admin group", exported.Description)
	assert.Equal(suite.T(), "ou1", exported.OUID)
	assert.Len(suite.T(), exported.Members, 2)
	assert.Equal(suite.T(), "user1", exported.Members[0].ID)
	assert.Equal(suite.T(), MemberTypeUser, exported.Members[0].Type)
	assert.Equal(suite.T(), "group2", exported.Members[1].ID)
	assert.Equal(suite.T(), MemberTypeGroup, exported.Members[1].Type)
	suite.mockService.AssertExpectations(suite.T())
}

// Test GetResourceByID - success with no members
func (suite *GroupExporterTestSuite) TestGetResourceByID_NoMembers() {
	grp := &Group{
		ID:   "group1",
		Name: "Admins",
		OUID: "ou1",
	}
	emptyMembers := &MemberListResponse{Members: []Member{}, TotalResults: 0}

	suite.mockService.On("GetGroup", suite.ctx, "group1", false).Return(grp, nil)
	suite.mockService.On("GetGroupMembers", suite.ctx, "group1", serverconst.MaxPageSize, 0, false).
		Return(emptyMembers, nil)

	resource, name, err := suite.exporter.GetResourceByID(suite.ctx, "group1")

	suite.Nil(err)
	assert.Equal(suite.T(), "Admins", name)
	exported, ok := resource.(*groupDeclarativeResource)
	assert.True(suite.T(), ok)
	assert.Empty(suite.T(), exported.Members)
	suite.mockService.AssertExpectations(suite.T())
}

// Test GetResourceByID - members fetched across multiple pages
func (suite *GroupExporterTestSuite) TestGetResourceByID_MembersPaginated() {
	grp := &Group{ID: "group1", Name: "Big Group", OUID: "ou1"}
	page1 := &MemberListResponse{
		Members:      []Member{{ID: "u1", Type: MemberTypeUser}},
		TotalResults: 2,
	}
	page2 := &MemberListResponse{
		Members:      []Member{{ID: "u2", Type: MemberTypeUser}},
		TotalResults: 2,
	}
	emptyPage := &MemberListResponse{Members: []Member{}, TotalResults: 2}

	suite.mockService.On("GetGroup", suite.ctx, "group1", false).Return(grp, nil)
	suite.mockService.On("GetGroupMembers", suite.ctx, "group1", serverconst.MaxPageSize, 0, false).
		Return(page1, nil)
	suite.mockService.On("GetGroupMembers", suite.ctx, "group1", serverconst.MaxPageSize, 1, false).
		Return(page2, nil)
	suite.mockService.On("GetGroupMembers", suite.ctx, "group1", serverconst.MaxPageSize, 2, false).
		Return(emptyPage, nil)

	resource, _, err := suite.exporter.GetResourceByID(suite.ctx, "group1")

	suite.Nil(err)
	exported, ok := resource.(*groupDeclarativeResource)
	assert.True(suite.T(), ok)
	assert.Len(suite.T(), exported.Members, 2)
	suite.mockService.AssertExpectations(suite.T())
}

// Test GetResourceByID - error on GetGroup
func (suite *GroupExporterTestSuite) TestGetResourceByID_ErrorOnGetGroup() {
	serviceErr := &serviceerror.ServiceError{Code: "GRP-1003"}
	suite.mockService.On("GetGroup", suite.ctx, "nonexistent", false).Return(nil, serviceErr)

	resource, name, err := suite.exporter.GetResourceByID(suite.ctx, "nonexistent")

	suite.NotNil(err)
	assert.Nil(suite.T(), resource)
	assert.Empty(suite.T(), name)
	assert.Equal(suite.T(), serviceErr, err)
	suite.mockService.AssertExpectations(suite.T())
}

// Test GetResourceByID - error on GetGroupMembers
func (suite *GroupExporterTestSuite) TestGetResourceByID_ErrorOnGetGroupMembers() {
	grp := &Group{ID: "group1", Name: "Admins", OUID: "ou1"}
	serviceErr := &serviceerror.ServiceError{Code: "500"}

	suite.mockService.On("GetGroup", suite.ctx, "group1", false).Return(grp, nil)
	suite.mockService.On("GetGroupMembers", suite.ctx, "group1", serverconst.MaxPageSize, 0, false).
		Return(nil, serviceErr)

	resource, name, err := suite.exporter.GetResourceByID(suite.ctx, "group1")

	suite.NotNil(err)
	assert.Nil(suite.T(), resource)
	assert.Empty(suite.T(), name)
	assert.Equal(suite.T(), serviceErr, err)
	suite.mockService.AssertExpectations(suite.T())
}

// Test ValidateResource - success
func (suite *GroupExporterTestSuite) TestValidateResource_Success() {
	resource := &groupDeclarativeResource{
		ID:   "group1",
		Name: "Admins",
		OUID: "ou1",
	}
	logger := log.GetLogger()

	name, exportErr := suite.exporter.ValidateResource(resource, "group1", logger)

	suite.Nil(exportErr)
	assert.Equal(suite.T(), "Admins", name)
}

// Test ValidateResource - success with members
func (suite *GroupExporterTestSuite) TestValidateResource_WithMembers() {
	resource := &groupDeclarativeResource{
		ID:   "group1",
		Name: "Admins",
		OUID: "ou1",
		Members: []Member{
			{ID: "user1", Type: MemberTypeUser},
		},
	}
	logger := log.GetLogger()

	name, exportErr := suite.exporter.ValidateResource(resource, "group1", logger)

	suite.Nil(exportErr)
	assert.Equal(suite.T(), "Admins", name)
}

// Test ValidateResource - wrong type
func (suite *GroupExporterTestSuite) TestValidateResource_WrongType() {
	logger := log.GetLogger()

	name, exportErr := suite.exporter.ValidateResource("not a group", "group1", logger)

	suite.NotNil(exportErr)
	assert.Empty(suite.T(), name)
}

// Test ValidateResource - empty name
func (suite *GroupExporterTestSuite) TestValidateResource_EmptyName() {
	resource := &groupDeclarativeResource{
		ID:   "group1",
		Name: "",
		OUID: "ou1",
	}
	logger := log.GetLogger()

	name, exportErr := suite.exporter.ValidateResource(resource, "group1", logger)

	suite.NotNil(exportErr)
	assert.Empty(suite.T(), name)
}
