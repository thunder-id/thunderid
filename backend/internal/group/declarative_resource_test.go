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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
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

	name, exportErr := suite.exporter.ValidateResource(context.Background(), resource, "group1", logger)

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

	name, exportErr := suite.exporter.ValidateResource(context.Background(), resource, "group1", logger)

	suite.Nil(exportErr)
	assert.Equal(suite.T(), "Admins", name)
}

// Test ValidateResource - wrong type
func (suite *GroupExporterTestSuite) TestValidateResource_WrongType() {
	logger := log.GetLogger()

	name, exportErr := suite.exporter.ValidateResource(context.Background(), "not a group", "group1", logger)

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

	name, exportErr := suite.exporter.ValidateResource(context.Background(), resource, "group1", logger)

	suite.NotNil(exportErr)
	assert.Empty(suite.T(), name)
}

// Test parseToGroup - valid YAML with all fields
func (suite *GroupExporterTestSuite) TestParseToGroup_ValidYAML() {
	yamlData := []byte(`
id: group1
name: Admins
description: Admin group
ou_id: ou1
members:
  - id: user1
    type: user
  - id: group2
    type: group
`)

	grp, err := parseToGroup(yamlData)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), grp)
	assert.Equal(suite.T(), "group1", grp.ID)
	assert.Equal(suite.T(), "Admins", grp.Name)
	assert.Equal(suite.T(), "Admin group", grp.Description)
	assert.Equal(suite.T(), "ou1", grp.OUID)
	assert.Len(suite.T(), grp.Members, 2)
	assert.Equal(suite.T(), "user1", grp.Members[0].ID)
	assert.Equal(suite.T(), memberTypeEntity, grp.Members[0].Type)
	assert.Equal(suite.T(), "group2", grp.Members[1].ID)
	assert.Equal(suite.T(), MemberTypeGroup, grp.Members[1].Type)
}

// Test parseToGroup - invalid YAML
func (suite *GroupExporterTestSuite) TestParseToGroup_InvalidYAML() {
	yamlData := []byte(`
invalid: yaml: content:
`)

	grp, err := parseToGroup(yamlData)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), grp)
}

// Test parseToGroup - optional fields omitted
func (suite *GroupExporterTestSuite) TestParseToGroup_OptionalFieldsOmitted() {
	yamlData := []byte(`
id: group1
name: Admins
ou_id: ou1
`)

	grp, err := parseToGroup(yamlData)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), grp)
	assert.Empty(suite.T(), grp.Description)
	assert.Empty(suite.T(), grp.Members)
	assert.Empty(suite.T(), grp.OUHandle)
}

// Test parseToGroup - ou_handle is preserved
func (suite *GroupExporterTestSuite) TestParseToGroup_WithOUHandle() {
	yamlData := []byte(`
id: group1
name: Admins
ou_handle: /root/engineering
`)

	grp, err := parseToGroup(yamlData)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), grp)
	assert.Equal(suite.T(), "/root/engineering", grp.OUHandle)
	assert.Empty(suite.T(), grp.OUID)
}

// Test parseToGroup - entity member types (user, app, agent) are translated to internal type
func (suite *GroupExporterTestSuite) TestParseToGroup_EntityTypesTranslated() {
	yamlData := []byte(`
id: group1
name: Mixed
ou_id: ou1
members:
  - id: u1
    type: user
  - id: a1
    type: app
  - id: ag1
    type: agent
  - id: g1
    type: group
`)

	grp, err := parseToGroup(yamlData)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), grp.Members, 4)
	assert.Equal(suite.T(), memberTypeEntity, grp.Members[0].Type)
	assert.Equal(suite.T(), memberTypeEntity, grp.Members[1].Type)
	assert.Equal(suite.T(), memberTypeEntity, grp.Members[2].Type)
	assert.Equal(suite.T(), MemberTypeGroup, grp.Members[3].Type)
}

// Test parseToGroupWrapper - returns correct type
func (suite *GroupExporterTestSuite) TestParseToGroupWrapper() {
	yamlData := []byte(`
id: group1
name: Admins
ou_id: ou1
`)

	result, err := parseToGroupWrapper(yamlData)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	grp, ok := result.(*groupDeclarativeResource)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), "group1", grp.ID)
}

// Test validateGroupWrapper - valid group
func (suite *GroupExporterTestSuite) TestValidateGroupWrapper_ValidGroup() {
	grp := &groupDeclarativeResource{
		ID:   "group1",
		Name: "Admins",
		OUID: "ou1",
	}

	err := validateGroupWrapper(grp, nil, nil, nil)

	assert.NoError(suite.T(), err)
}

// Test validateGroupWrapper - valid group with members
func (suite *GroupExporterTestSuite) TestValidateGroupWrapper_WithMembers() {
	grp := &groupDeclarativeResource{
		ID:   "group1",
		Name: "Admins",
		OUID: "ou1",
		Members: []Member{
			{ID: "user1", Type: memberTypeEntity},
			{ID: "group2", Type: MemberTypeGroup},
		},
	}

	err := validateGroupWrapper(grp, nil, nil, nil)

	assert.NoError(suite.T(), err)
}

// Test validateGroupWrapper - wrong type
func (suite *GroupExporterTestSuite) TestValidateGroupWrapper_WrongType() {
	err := validateGroupWrapper("not a group", nil, nil, nil)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "invalid type")
}

// Test validateGroupWrapper - missing ID
func (suite *GroupExporterTestSuite) TestValidateGroupWrapper_MissingID() {
	grp := &groupDeclarativeResource{
		Name: "Admins",
		OUID: "ou1",
	}

	err := validateGroupWrapper(grp, nil, nil, nil)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "group ID is required")
}

// Test validateGroupWrapper - missing name
func (suite *GroupExporterTestSuite) TestValidateGroupWrapper_MissingName() {
	grp := &groupDeclarativeResource{
		ID:   "group1",
		OUID: "ou1",
	}

	err := validateGroupWrapper(grp, nil, nil, nil)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "group name is required")
}

// Test validateGroupWrapper - missing OU ID when no handle
func (suite *GroupExporterTestSuite) TestValidateGroupWrapper_MissingOUID() {
	grp := &groupDeclarativeResource{
		ID:   "group1",
		Name: "Admins",
	}

	err := validateGroupWrapper(grp, nil, nil, nil)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "ou_id or ou_handle is required")
}

// Test validateGroupWrapper - duplicate ID in DB store
func (suite *GroupExporterTestSuite) TestValidateGroupWrapper_DuplicateInDBStore() {
	grp := &groupDeclarativeResource{
		ID:   "group1",
		Name: "Admins",
		OUID: "ou1",
	}
	mockStore := newGroupStoreInterfaceMock(suite.T())
	mockStore.On("GetGroup", mock.Anything, "group1").Return(GroupDAO{ID: "group1", Name: "Admins"}, nil)

	err := validateGroupWrapper(grp, nil, mockStore, nil)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "duplicate group ID")
	assert.Contains(suite.T(), err.Error(), "database store")
}

// Test validateGroupWrapper - no duplicate when DB returns error (not found)
func (suite *GroupExporterTestSuite) TestValidateGroupWrapper_NoDuplicateWhenDBNotFound() {
	grp := &groupDeclarativeResource{
		ID:   "group1",
		Name: "Admins",
		OUID: "ou1",
	}
	mockStore := newGroupStoreInterfaceMock(suite.T())
	mockStore.On("GetGroup", mock.Anything, "group1").Return(GroupDAO{}, ErrGroupNotFound)

	err := validateGroupWrapper(grp, nil, mockStore, nil)

	assert.NoError(suite.T(), err)
}

// GroupDeclarativeResourceLoaderTestSuite contains integration tests for loadDeclarativeResources.
type GroupDeclarativeResourceLoaderTestSuite struct {
	suite.Suite
}

func TestGroupDeclarativeResourceLoaderTestSuite(t *testing.T) {
	suite.Run(t, new(GroupDeclarativeResourceLoaderTestSuite))
}

func (suite *GroupDeclarativeResourceLoaderTestSuite) newFileStore() *fileBasedGroupStore {
	return &fileBasedGroupStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeGroup),
	}
}

func (suite *GroupDeclarativeResourceLoaderTestSuite) initRuntime() {
	tempDir := suite.T().TempDir()
	config.ResetServerRuntime()
	suite.Require().NoError(config.InitializeServerRuntime(tempDir, &config.Config{}))
}

func (suite *GroupDeclarativeResourceLoaderTestSuite) createGroupsDir() string {
	runtime := config.GetServerRuntime()
	dir := filepath.Join(runtime.ServerHome, "repository", "resources", "groups")
	suite.Require().NoError(os.MkdirAll(dir, 0o750))
	return dir
}

// TestLoadDeclarativeResources_NoResourceDirectory - missing groups dir, no files, returns nil.
func (suite *GroupDeclarativeResourceLoaderTestSuite) TestLoadDeclarativeResources_NoResourceDirectory() {
	suite.initRuntime()
	fileStore := suite.newFileStore()

	err := loadDeclarativeResources(fileStore, nil, nil)

	suite.NoError(err)
}

// TestLoadDeclarativeResources_ValidGroup - single valid YAML file is parsed, validated, and stored.
func (suite *GroupDeclarativeResourceLoaderTestSuite) TestLoadDeclarativeResources_ValidGroup() {
	suite.initRuntime()
	resourceDir := suite.createGroupsDir()

	yamlData := []byte(`
id: group1
name: Admins
description: Admin group
ou_id: ou1
`)
	suite.Require().NoError(os.WriteFile(filepath.Join(resourceDir, "group1.yaml"), yamlData, 0o600))

	fileStore := suite.newFileStore()
	err := loadDeclarativeResources(fileStore, nil, nil)

	suite.NoError(err)
	grp, getErr := fileStore.GetGroup(context.Background(), "group1")
	suite.NoError(getErr)
	suite.Equal("group1", grp.ID)
	suite.Equal("Admins", grp.Name)
	suite.Equal("Admin group", grp.Description)
	suite.Equal("ou1", grp.OUID)
}

// TestLoadDeclarativeResources_ValidGroupWithMembers - members are loaded alongside the group.
func (suite *GroupDeclarativeResourceLoaderTestSuite) TestLoadDeclarativeResources_ValidGroupWithMembers() {
	suite.initRuntime()
	resourceDir := suite.createGroupsDir()

	yamlData := []byte(`
id: group1
name: Admins
ou_id: ou1
members:
  - id: user1
    type: user
  - id: group2
    type: group
`)
	suite.Require().NoError(os.WriteFile(filepath.Join(resourceDir, "group1.yaml"), yamlData, 0o600))

	fileStore := suite.newFileStore()
	err := loadDeclarativeResources(fileStore, nil, nil)

	suite.NoError(err)
	grp, getErr := fileStore.GetGroup(context.Background(), "group1")
	suite.NoError(getErr)
	suite.Len(grp.Members, 2)
	suite.Equal("user1", grp.Members[0].ID)
	suite.Equal("group2", grp.Members[1].ID)
}

// TestLoadDeclarativeResources_MultipleGroups - each file is independently loaded.
func (suite *GroupDeclarativeResourceLoaderTestSuite) TestLoadDeclarativeResources_MultipleGroups() {
	suite.initRuntime()
	resourceDir := suite.createGroupsDir()

	for _, g := range []struct{ id, name string }{{"group1", "Admins"}, {"group2", "Engineers"}} {
		yaml := []byte("id: " + g.id + "\nname: " + g.name + "\nou_id: ou1\n")
		suite.Require().NoError(os.WriteFile(filepath.Join(resourceDir, g.id+".yaml"), yaml, 0o600))
	}

	fileStore := suite.newFileStore()
	err := loadDeclarativeResources(fileStore, nil, nil)

	suite.NoError(err)
	for _, id := range []string{"group1", "group2"} {
		grp, getErr := fileStore.GetGroup(context.Background(), id)
		suite.NoError(getErr)
		suite.Equal(id, grp.ID)
	}
}

// TestLoadDeclarativeResources_InvalidYAML - malformed YAML causes error wrapping "failed to load group resources".
func (suite *GroupDeclarativeResourceLoaderTestSuite) TestLoadDeclarativeResources_InvalidYAML() {
	suite.initRuntime()
	resourceDir := suite.createGroupsDir()

	suite.Require().NoError(
		os.WriteFile(filepath.Join(resourceDir, "bad.yaml"), []byte("invalid: yaml: content:\n"), 0o600),
	)

	fileStore := suite.newFileStore()
	err := loadDeclarativeResources(fileStore, nil, nil)

	suite.Error(err)
	suite.Contains(err.Error(), "failed to load group resources")
}

// TestLoadDeclarativeResources_MissingID - YAML without id fails validation.
func (suite *GroupDeclarativeResourceLoaderTestSuite) TestLoadDeclarativeResources_MissingID() {
	suite.initRuntime()
	resourceDir := suite.createGroupsDir()

	suite.Require().NoError(
		os.WriteFile(filepath.Join(resourceDir, "noID.yaml"), []byte("name: Admins\nou_id: ou1\n"), 0o600),
	)

	fileStore := suite.newFileStore()
	err := loadDeclarativeResources(fileStore, nil, nil)

	suite.Error(err)
	suite.Contains(err.Error(), "failed to load group resources")
}

// TestLoadDeclarativeResources_MissingOUID - YAML without ou_id or ou_handle fails validation.
func (suite *GroupDeclarativeResourceLoaderTestSuite) TestLoadDeclarativeResources_MissingOUID() {
	suite.initRuntime()
	resourceDir := suite.createGroupsDir()

	suite.Require().NoError(
		os.WriteFile(filepath.Join(resourceDir, "noOU.yaml"), []byte("id: group1\nname: Admins\n"), 0o600),
	)

	fileStore := suite.newFileStore()
	err := loadDeclarativeResources(fileStore, nil, nil)

	suite.Error(err)
	suite.Contains(err.Error(), "failed to load group resources")
}

// TestLoadDeclarativeResources_DuplicateInFileStore - second file with same ID is rejected.
func (suite *GroupDeclarativeResourceLoaderTestSuite) TestLoadDeclarativeResources_DuplicateInFileStore() {
	suite.initRuntime()
	resourceDir := suite.createGroupsDir()

	yaml1 := []byte("id: group1\nname: Admins\nou_id: ou1\n")
	yaml2 := []byte("id: group1\nname: AdminsDuplicate\nou_id: ou2\n")
	suite.Require().NoError(os.WriteFile(filepath.Join(resourceDir, "group1a.yaml"), yaml1, 0o600))
	suite.Require().NoError(os.WriteFile(filepath.Join(resourceDir, "group1b.yaml"), yaml2, 0o600))

	fileStore := suite.newFileStore()
	err := loadDeclarativeResources(fileStore, nil, nil)

	suite.Error(err)
	suite.Contains(err.Error(), "failed to load group resources")
}

// TestLoadDeclarativeResources_DuplicateInDBStore - dbStore already contains the group ID, loading is rejected.
func (suite *GroupDeclarativeResourceLoaderTestSuite) TestLoadDeclarativeResources_DuplicateInDBStore() {
	suite.initRuntime()
	resourceDir := suite.createGroupsDir()

	suite.Require().NoError(
		os.WriteFile(
			filepath.Join(resourceDir, "group1.yaml"),
			[]byte("id: group1\nname: Admins\nou_id: ou1\n"),
			0o600,
		),
	)

	mockStore := newGroupStoreInterfaceMock(suite.T())
	mockStore.On("GetGroup", mock.Anything, "group1").Return(GroupDAO{ID: "group1"}, nil)

	fileStore := suite.newFileStore()
	err := loadDeclarativeResources(fileStore, mockStore, nil)

	suite.Error(err)
	suite.Contains(err.Error(), "failed to load group resources")
	mockStore.AssertExpectations(suite.T())
}
