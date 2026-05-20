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

package resource

import (
	"context"
	"errors"
	"testing"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// ResourceServerExporterTestSuite tests the resourceServerExporter.
type ResourceServerExporterTestSuite struct {
	suite.Suite
	mockService *ResourceServiceInterfaceMock
	exporter    *resourceServerExporter
	logger      *log.Logger
}

func TestResourceServerExporterTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceServerExporterTestSuite))
}

func (s *ResourceServerExporterTestSuite) SetupTest() {
	s.mockService = NewResourceServiceInterfaceMock(s.T())
	s.exporter = newResourceServerExporterForTest(s.mockService)
	s.logger = log.GetLogger()
}

func (s *ResourceServerExporterTestSuite) TestNewResourceServerExporter() {
	assert.NotNil(s.T(), s.exporter)
}

func (s *ResourceServerExporterTestSuite) TestGetResourceType() {
	assert.Equal(s.T(), "resource_server", s.exporter.GetResourceType())
}

func (s *ResourceServerExporterTestSuite) TestGetParameterizerType() {
	assert.Equal(s.T(), "ResourceServer", s.exporter.GetParameterizerType())
}

func (s *ResourceServerExporterTestSuite) TestGetAllResourceIDs_Success() {
	ctx := context.Background()
	expectedList := &ResourceServerList{
		TotalResults: 2,
		ResourceServers: []ResourceServer{
			{ID: "rs1", Name: "Resource Server 1"},
			{ID: "rs2", Name: "Resource Server 2"},
		},
	}

	s.mockService.EXPECT().GetResourceServerList(ctx, serverconst.MaxPageSize, 0).Return(expectedList, nil)
	s.mockService.EXPECT().IsResourceServerDeclarative("rs1").Return(false)
	s.mockService.EXPECT().IsResourceServerDeclarative("rs2").Return(false)

	ids, err := s.exporter.GetAllResourceIDs(ctx)

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 2)
	assert.Equal(s.T(), "rs1", ids[0])
	assert.Equal(s.T(), "rs2", ids[1])
}

func (s *ResourceServerExporterTestSuite) TestGetAllResourceIDs_FilterDeclarative() {
	ctx := context.Background()
	expectedList := &ResourceServerList{
		TotalResults: 3,
		ResourceServers: []ResourceServer{
			{ID: "rs1", Name: "Mutable Server"},
			{ID: "rs2", Name: "Declarative Server"},
			{ID: "rs3", Name: "Another Mutable Server"},
		},
	}

	s.mockService.EXPECT().GetResourceServerList(ctx, serverconst.MaxPageSize, 0).Return(expectedList, nil)
	s.mockService.EXPECT().IsResourceServerDeclarative("rs1").Return(false)
	s.mockService.EXPECT().IsResourceServerDeclarative("rs2").Return(true)
	s.mockService.EXPECT().IsResourceServerDeclarative("rs3").Return(false)

	ids, err := s.exporter.GetAllResourceIDs(ctx)

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 2)
	assert.Equal(s.T(), "rs1", ids[0])
	assert.Equal(s.T(), "rs3", ids[1])
}

func (s *ResourceServerExporterTestSuite) TestGetAllResourceIDs_Error() {
	ctx := context.Background()
	expectedError := &serviceerror.ServiceError{
		Code:  "ERR_CODE",
		Error: i18ncore.I18nMessage{DefaultValue: "test error"},
	}

	s.mockService.EXPECT().GetResourceServerList(ctx, serverconst.MaxPageSize, 0).Return(nil, expectedError)

	ids, err := s.exporter.GetAllResourceIDs(ctx)

	assert.Nil(s.T(), ids)
	assert.Equal(s.T(), expectedError, err)
}

func (s *ResourceServerExporterTestSuite) TestGetResourceByID_Success() {
	ctx := context.Background()
	serverID := "rs1"

	server := &ResourceServer{
		ID:          serverID,
		Name:        "Test Server",
		Description: "A test server",
		Identifier:  "test-server",
		OUID:        "ou1",
		Delimiter:   ":",
	}

	resources := &ResourceList{
		TotalResults: 1,
		Resources: []Resource{
			{
				ID:           "res1",
				Name:         "Resource 1",
				Handle:       "resource1",
				Description:  "First resource",
				Parent:       nil,
				ParentHandle: "",
				Permission:   "test-server:resource1",
			},
		},
	}

	actions := &ActionList{
		TotalResults: 1,
		Actions: []Action{
			{
				ID:          "act1",
				Name:        "Action 1",
				Handle:      "read",
				Description: "Read action",
				Permission:  "test-server:resource1:read",
			},
		},
	}

	resourceID := "res1"
	s.mockService.EXPECT().GetResourceServer(ctx, serverID).Return(server, nil)
	s.mockService.EXPECT().GetResourceList(
		ctx, serverID, (*string)(nil), serverconst.MaxPageSize, 0).Return(resources, nil)
	s.mockService.EXPECT().GetActionList(ctx, serverID, &resourceID, serverconst.MaxPageSize, 0).Return(actions, nil)

	result, name, err := s.exporter.GetResourceByID(ctx, serverID)

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "Test Server", name)
	assert.NotNil(s.T(), result)

	dto, ok := result.(*ResourceServer)
	assert.True(s.T(), ok)
	assert.Equal(s.T(), serverID, dto.ID)
	assert.Equal(s.T(), "Test Server", dto.Name)
	assert.Len(s.T(), dto.Resources, 1)
	assert.Len(s.T(), dto.Resources[0].Actions, 1)
}

func (s *ResourceServerExporterTestSuite) TestGetResourceByID_ServerNotFound() {
	ctx := context.Background()
	serverID := "rs-nonexistent"

	expectedError := &ErrorResourceServerNotFound

	s.mockService.EXPECT().GetResourceServer(ctx, serverID).Return(nil, expectedError)

	result, name, err := s.exporter.GetResourceByID(ctx, serverID)

	assert.Nil(s.T(), result)
	assert.Equal(s.T(), "", name)
	assert.Equal(s.T(), expectedError, err)
}

func (s *ResourceServerExporterTestSuite) TestValidateResource_Success() {
	dto := &ResourceServer{
		ID:          "rs1",
		Name:        "Test Server",
		Description: "A test server",
		OUID:        "ou1",
	}

	name, err := s.exporter.ValidateResource(dto, "rs1", s.logger)

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "Test Server", name)
}

func (s *ResourceServerExporterTestSuite) TestValidateResource_InvalidType() {
	invalidData := "not a resource server dto"

	name, err := s.exporter.ValidateResource(invalidData, "rs1", s.logger)

	assert.Equal(s.T(), "", name)
	assert.NotNil(s.T(), err)
}

func (s *ResourceServerExporterTestSuite) TestValidateResource_EmptyName() {
	dto := &ResourceServer{
		ID:   "rs1",
		Name: "",
		OUID: "ou1",
	}

	name, err := s.exporter.ValidateResource(dto, "rs1", s.logger)

	assert.Equal(s.T(), "", name)
	assert.NotNil(s.T(), err)
}

func (s *ResourceServerExporterTestSuite) TestGetResourceRules() {
	rules := s.exporter.GetResourceRules()

	assert.Nil(s.T(), rules)
}

func TestParseToResourceServer(t *testing.T) {
	yamlData := []byte(`
id: "rs1"
name: "Test Server"
description: "Test description"
identifier: "test-server"
ou_id: "ou1"
delimiter: ":"
resources:
  - name: "Users"
    handle: "users"
    description: "User resources"
    actions:
      - name: "Read"
        handle: "read"
`)

	dto, err := parseToResourceServer(yamlData)

	assert.NoError(t, err)
	assert.NotNil(t, dto)
	assert.Equal(t, "rs1", dto.ID)
	assert.Equal(t, "Test Server", dto.Name)
	assert.Equal(t, "test-server", dto.Identifier)
	assert.Equal(t, "ou1", dto.OUID)
	assert.Equal(t, ":", dto.Delimiter)
	assert.Len(t, dto.Resources, 1)
	assert.Equal(t, "users", dto.Resources[0].Handle)
	assert.Len(t, dto.Resources[0].Actions, 1)
}

func TestParseToResourceServer_MissingID(t *testing.T) {
	yamlData := []byte(`
name: "Test Server"
ou_id: "ou1"
`)

	dto, err := parseToResourceServer(yamlData)

	assert.Error(t, err)
	assert.Nil(t, dto)
	assert.Contains(t, err.Error(), "ID cannot be empty")
}

func TestParseToResourceServer_MissingName(t *testing.T) {
	yamlData := []byte(`
id: "rs1"
ou_id: "ou1"
`)

	dto, err := parseToResourceServer(yamlData)

	assert.Error(t, err)
	assert.Nil(t, dto)
	assert.Contains(t, err.Error(), "name cannot be empty")
}

func TestBuildPermissionString(t *testing.T) {
	resourceHandleMap := map[string]*Resource{
		"users": {
			Handle:       "users",
			Parent:       nil,
			ParentHandle: "",
		},
		"profile": {
			Handle:       "profile",
			Parent:       nil,
			ParentHandle: "users",
		},
	}

	tests := []struct {
		name      string
		resource  *Resource
		handler   string
		delimiter string
		expected  string
	}{
		{
			name: "root resource with handler",
			resource: &Resource{
				Handle:       "users",
				ParentHandle: "",
			},
			handler:   "booking-api",
			delimiter: ":",
			expected:  "booking-api:users",
		},
		{
			name: "nested resource with handler",
			resource: &Resource{
				Handle:       "profile",
				ParentHandle: "users",
			},
			handler:   "booking-api",
			delimiter: ":",
			expected:  "booking-api:users:profile",
		},
		{
			name: "root resource without handler",
			resource: &Resource{
				Handle:       "users",
				ParentHandle: "",
			},
			handler:   "",
			delimiter: ":",
			expected:  "users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := buildPermissionString(tt.resource, resourceHandleMap, tt.handler, tt.delimiter)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProcessResourceServer_SetsPermissionsAndDelimiter(t *testing.T) {
	rs := &ResourceServer{
		ID:         "rs1",
		Name:       "Test Server",
		Handle:     "test-api",
		OUID:       "ou1",
		Identifier: "api",
		Resources: []Resource{
			{
				Name:   "Users",
				Handle: "users",
				Actions: []Action{
					{Name: "Read", Handle: "read"},
				},
			},
			{
				Name:         "Profile",
				Handle:       "profile",
				ParentHandle: "users",
			},
		},
	}

	err := ProcessResourceServer(rs)

	assert.NoError(t, err)
	assert.Equal(t, ":", rs.Delimiter)
	assert.Equal(t, "test-api:users", rs.Resources[0].Permission)
	assert.Equal(t, "test-api:users:read", rs.Resources[0].Actions[0].Permission)
	assert.Equal(t, "test-api:users:profile", rs.Resources[1].Permission)
}

func TestProcessResourceServer_WithHandlePrefixesPermissions(t *testing.T) {
	rs := &ResourceServer{
		ID:     "rs1",
		Name:   "Test Server",
		OUID:   "ou1",
		Handle: "booking-api",
		Resources: []Resource{
			{
				Name:   "Users",
				Handle: "users",
				Actions: []Action{
					{Name: "Read", Handle: "read"},
				},
			},
			{
				Name:         "Profile",
				Handle:       "profile",
				ParentHandle: "users",
			},
		},
	}

	err := ProcessResourceServer(rs)

	assert.NoError(t, err)
	assert.Equal(t, ":", rs.Delimiter)
	assert.Equal(t, "booking-api:users", rs.Resources[0].Permission)
	assert.Equal(t, "booking-api:users:read", rs.Resources[0].Actions[0].Permission)
	assert.Equal(t, "booking-api:users:profile", rs.Resources[1].Permission)
}

func TestProcessResourceServer_DuplicateHandle(t *testing.T) {
	rs := &ResourceServer{
		ID:     "rs1",
		Name:   "Test Server",
		Handle: "dup-test",
		OUID:   "ou1",
		Resources: []Resource{
			{Name: "Users", Handle: "users"},
			{Name: "Users Duplicate", Handle: "users"},
		},
	}

	err := ProcessResourceServer(rs)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate resource handle")
}

func TestProcessResource_SetsPermissions(t *testing.T) {
	root := &Resource{Handle: "root"}
	resource := &Resource{
		Handle:       "child",
		ParentHandle: "root",
		Actions: []Action{
			{Name: "Read", Handle: "read"},
		},
	}
	resourceHandleMap := map[string]*Resource{
		"root":  root,
		"child": resource,
	}

	err := processResource(resource, resourceHandleMap, "", ":")

	assert.NoError(t, err)
	assert.Equal(t, "root:child", resource.Permission)
	assert.Equal(t, "root:child:read", resource.Actions[0].Permission)
}

func TestProcessResource_MissingParent(t *testing.T) {
	resource := &Resource{Handle: "child", ParentHandle: "missing"}
	resourceHandleMap := map[string]*Resource{}

	err := processResource(resource, resourceHandleMap, "", ":")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parent resource handle")
}

func TestParseAndValidateResourceServerWrapper_Success(t *testing.T) {
	yamlData := []byte(`
id: "rs1"
name: "Test Server"
handle: "test-api"
identifier: "api"
ou_id: "ou1"
resources:
  - name: "Users"
    handle: "users"
    actions:
      - name: "Read"
        handle: "read"
  - name: "Profile"
    handle: "profile"
    parent: "users"
`)

	parser := parseAndValidateResourceServerWrapper(nil)
	result, err := parser(yamlData)

	assert.NoError(t, err)
	rs, ok := result.(*ResourceServer)
	assert.True(t, ok)
	assert.Equal(t, "test-api:users", rs.Resources[0].Permission)
	assert.Equal(t, "test-api:users:read", rs.Resources[0].Actions[0].Permission)
	assert.Equal(t, "test-api:users:profile", rs.Resources[1].Permission)
}

func TestParseAndValidateResourceServerWrapper_InvalidYAML(t *testing.T) {
	yamlData := []byte(`::invalid`)

	parser := parseAndValidateResourceServerWrapper(nil)
	result, err := parser(yamlData)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestValidateResourceServerWrapper_InvalidType(t *testing.T) {
	err := validateResourceServerWrapper("invalid", newResourceStoreInterfaceMock(t), nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid type")
}

func TestValidateResourceServerWrapper_EmptyName(t *testing.T) {
	fileStore := newResourceStoreInterfaceMock(t)

	err := validateResourceServerWrapper(&ResourceServer{ID: "rs1"}, fileStore, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name cannot be empty")
}

func TestValidateResourceServerWrapper_DuplicateInFileStore(t *testing.T) {
	fileStore := newResourceStoreInterfaceMock(t)
	fileStore.On("GetResourceServer", mock.Anything, "rs1").Return(ResourceServer{ID: "rs1"}, nil)

	err := validateResourceServerWrapper(&ResourceServer{ID: "rs1", Name: "Server"}, fileStore, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate resource server ID")
}

func TestValidateResourceServerWrapper_FileStoreError(t *testing.T) {
	fileStore := newResourceStoreInterfaceMock(t)
	fileStore.On("GetResourceServer", mock.Anything, "rs1").Return(ResourceServer{}, errors.New("file error"))

	err := validateResourceServerWrapper(&ResourceServer{ID: "rs1", Name: "Server"}, fileStore, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check")
}

func TestValidateResourceServerWrapper_DuplicateInDBStore(t *testing.T) {
	fileStore := newResourceStoreInterfaceMock(t)
	dbStore := newResourceStoreInterfaceMock(t)
	fileStore.On("GetResourceServer", mock.Anything, "rs1").Return(ResourceServer{}, errResourceServerNotFound)
	dbStore.On("GetResourceServer", mock.Anything, "rs1").Return(ResourceServer{ID: "rs1"}, nil)

	err := validateResourceServerWrapper(&ResourceServer{ID: "rs1", Name: "Server"}, fileStore, dbStore)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database store")
}

func TestValidateResourceServerWrapper_DBStoreError(t *testing.T) {
	fileStore := newResourceStoreInterfaceMock(t)
	dbStore := newResourceStoreInterfaceMock(t)
	fileStore.On("GetResourceServer", mock.Anything, "rs1").Return(ResourceServer{}, errResourceServerNotFound)
	dbStore.On("GetResourceServer", mock.Anything, "rs1").Return(ResourceServer{}, errors.New("db error"))

	err := validateResourceServerWrapper(&ResourceServer{ID: "rs1", Name: "Server"}, fileStore, dbStore)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database store")
}

func TestValidateResourceServerWrapper_Success(t *testing.T) {
	fileStore := newResourceStoreInterfaceMock(t)
	fileStore.On("GetResourceServer", mock.Anything, "rs1").Return(ResourceServer{}, errResourceServerNotFound)

	err := validateResourceServerWrapper(&ResourceServer{ID: "rs1", Name: "Server"}, fileStore, nil)

	assert.NoError(t, err)
}

func TestLoadDeclarativeResources_CompositeFileStoreTypeError(t *testing.T) {
	fileStore := newResourceStoreInterfaceMock(t)
	dbStore := newResourceStoreInterfaceMock(t)
	compositeStore := newCompositeResourceStore(fileStore, dbStore)

	err := loadDeclarativeResources(compositeStore, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to assert fileStore")
}

func TestLoadDeclarativeResources_InvalidStoreType(t *testing.T) {
	invalidStore := newResourceStoreInterfaceMock(t)

	err := loadDeclarativeResources(invalidStore, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid store type")
}
