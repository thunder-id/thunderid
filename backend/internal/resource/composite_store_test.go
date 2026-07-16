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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	testParentID = "parent-1"
	testRS1ID    = "rs1"
)

// CompositeResourceStoreTestSuite tests the compositeResourceStore.
type CompositeResourceStoreTestSuite struct {
	suite.Suite
	compositeStore *compositeResourceStore
	fileStoreMock  *resourceStoreInterfaceMock
	dbStoreMock    *resourceStoreInterfaceMock
	ctx            context.Context
}

func TestCompositeResourceStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CompositeResourceStoreTestSuite))
}

func (s *CompositeResourceStoreTestSuite) SetupTest() {
	s.fileStoreMock = newResourceStoreInterfaceMock(s.T())
	s.dbStoreMock = newResourceStoreInterfaceMock(s.T())
	s.compositeStore = newCompositeResourceStore(s.fileStoreMock, s.dbStoreMock)
	s.ctx = context.Background()
}

// Resource Server tests

func (s *CompositeResourceStoreTestSuite) TestCreateResourceServer_DelegatesToDB() {
	rs := providers.ResourceServer{
		ID:   "rs1",
		Name: "Test Server",
	}

	s.dbStoreMock.On("CreateResourceServer", s.ctx, "rs1", rs).Return(nil)

	err := s.compositeStore.CreateResourceServer(s.ctx, "rs1", rs)

	assert.NoError(s.T(), err)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertNotCalled(s.T(), "CreateResourceServer")
}

func (s *CompositeResourceStoreTestSuite) TestGetResourceServer_FoundInDB() {
	rs := providers.ResourceServer{
		ID:   "rs1",
		Name: "DB Server",
	}

	s.dbStoreMock.On("GetResourceServer", s.ctx, "rs1").Return(rs, nil)

	result, err := s.compositeStore.GetResourceServer(s.ctx, "rs1")

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "DB Server", result.Name)
	assert.False(s.T(), result.IsReadOnly, "DB resource server should have IsReadOnly=false")
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertNotCalled(s.T(), "GetResourceServer")
}

func (s *CompositeResourceStoreTestSuite) TestGetResourceServer_FoundInFile() {
	fileRS := providers.ResourceServer{
		ID:   "rs-file",
		Name: "File Server",
	}

	s.dbStoreMock.On("GetResourceServer", s.ctx, "rs-file").
		Return(providers.ResourceServer{}, errResourceServerNotFound)
	s.fileStoreMock.On("GetResourceServer", s.ctx, "rs-file").Return(fileRS, nil)

	result, err := s.compositeStore.GetResourceServer(s.ctx, "rs-file")

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "File Server", result.Name)
	assert.True(s.T(), result.IsReadOnly, "File resource server should have IsReadOnly=true")
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestGetResourceServer_NotFound() {
	s.dbStoreMock.On("GetResourceServer", s.ctx, "nonexistent").
		Return(providers.ResourceServer{}, errResourceServerNotFound)
	s.fileStoreMock.On("GetResourceServer", s.ctx, "nonexistent").
		Return(providers.ResourceServer{}, errResourceServerNotFound)

	result, err := s.compositeStore.GetResourceServer(s.ctx, "nonexistent")

	assert.Error(s.T(), err)
	assert.Equal(s.T(), providers.ResourceServer{}, result)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestGetResourceServer_DBError() {
	dbErr := errors.New("database error")

	s.dbStoreMock.On("GetResourceServer", s.ctx, "rs1").Return(providers.ResourceServer{}, dbErr)

	result, err := s.compositeStore.GetResourceServer(s.ctx, "rs1")

	assert.Error(s.T(), err)
	assert.Equal(s.T(), dbErr, err)
	assert.Equal(s.T(), providers.ResourceServer{}, result)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertNotCalled(s.T(), "GetResourceServer")
}

func (s *CompositeResourceStoreTestSuite) TestGetResourceServerList_MergesBothStores() {
	dbServers := []providers.ResourceServer{
		{ID: "rs-db1", Name: "DB Server 1"},
		{ID: "rs-db2", Name: "DB Server 2"},
	}
	fileServers := []providers.ResourceServer{
		{ID: "rs-file1", Name: "File Server 1"},
	}

	s.dbStoreMock.On("GetResourceServerListCount", s.ctx).Return(len(dbServers), nil)
	s.fileStoreMock.On("GetResourceServerListCount", s.ctx).Return(len(fileServers), nil)
	s.dbStoreMock.On("GetResourceServerList", s.ctx, mock.Anything, 0).Return(dbServers, nil)
	s.fileStoreMock.On("GetResourceServerList", s.ctx, mock.Anything, 0).Return(fileServers, nil)

	result, err := s.compositeStore.GetResourceServerList(s.ctx, 10, 0)

	assert.NoError(s.T(), err)
	assert.Len(s.T(), result, 3)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestGetResourceServerList_WithPagination() {
	dbServers := []providers.ResourceServer{
		{ID: "rs1", Name: "Server 1"},
		{ID: "rs2", Name: "Server 2"},
	}
	fileServers := []providers.ResourceServer{
		{ID: "rs3", Name: "Server 3"},
	}

	s.dbStoreMock.On("GetResourceServerListCount", s.ctx).Return(len(dbServers), nil)
	s.fileStoreMock.On("GetResourceServerListCount", s.ctx).Return(len(fileServers), nil)
	s.dbStoreMock.On("GetResourceServerList", s.ctx, mock.Anything, 0).Return(dbServers, nil)
	s.fileStoreMock.On("GetResourceServerList", s.ctx, mock.Anything, 0).Return(fileServers, nil)

	// Get first page
	result, err := s.compositeStore.GetResourceServerList(s.ctx, 2, 0)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), result, 2)

	// Get second page
	result, err = s.compositeStore.GetResourceServerList(s.ctx, 2, 2)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), result, 1)
}

func (s *CompositeResourceStoreTestSuite) TestGetResourceServerList_DeduplicatesDuplicates() {
	dbServers := []providers.ResourceServer{
		{ID: testRS1ID, Name: "Server 1"},
		{ID: "rs2", Name: "Server 2"},
	}
	fileServers := []providers.ResourceServer{
		{ID: testRS1ID, Name: "Server 1 File"}, // Duplicate ID
		{ID: "rs3", Name: "Server 3"},
	}

	s.dbStoreMock.On("GetResourceServerListCount", s.ctx).Return(len(dbServers), nil)
	s.fileStoreMock.On("GetResourceServerListCount", s.ctx).Return(len(fileServers), nil)
	s.dbStoreMock.On("GetResourceServerList", s.ctx, mock.Anything, 0).Return(dbServers, nil)
	s.fileStoreMock.On("GetResourceServerList", s.ctx, mock.Anything, 0).Return(fileServers, nil)

	result, err := s.compositeStore.GetResourceServerList(s.ctx, 10, 0)

	assert.NoError(s.T(), err)
	// Should have 3 unique servers (DB rs1 takes precedence over file rs1)
	assert.Len(s.T(), result, 3)

	// Verify DB version is kept (not file version)
	for _, rs := range result {
		if rs.ID == testRS1ID {
			assert.Equal(s.T(), "Server 1", rs.Name)
		}
	}
}

func (s *CompositeResourceStoreTestSuite) TestGetResourceServerList_VerifiesIsReadOnlyFlags() {
	dbServers := []providers.ResourceServer{
		{ID: "rs-db1", Name: "DB Server 1"},
		{ID: "rs-db2", Name: "DB Server 2"},
	}
	fileServers := []providers.ResourceServer{
		{ID: "rs-file1", Name: "File Server 1"},
	}

	s.dbStoreMock.On("GetResourceServerListCount", s.ctx).Return(len(dbServers), nil)
	s.fileStoreMock.On("GetResourceServerListCount", s.ctx).Return(len(fileServers), nil)
	s.dbStoreMock.On("GetResourceServerList", s.ctx, mock.Anything, 0).Return(dbServers, nil)
	s.fileStoreMock.On("GetResourceServerList", s.ctx, mock.Anything, 0).Return(fileServers, nil)

	result, err := s.compositeStore.GetResourceServerList(s.ctx, 10, 0)

	assert.NoError(s.T(), err)
	assert.Len(s.T(), result, 3)

	// Verify all resource servers have correct IsReadOnly flags
	for _, rs := range result {
		if rs.ID == "rs-db1" || rs.ID == "rs-db2" {
			assert.False(s.T(), rs.IsReadOnly, "DB resource server %s should have IsReadOnly=false", rs.ID)
		} else if rs.ID == "rs-file1" {
			assert.True(s.T(), rs.IsReadOnly, "File resource server %s should have IsReadOnly=true", rs.ID)
		}
	}

	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestGetResourceServerList_DeduplicatesAndSetsIsReadOnly() {
	dbServers := []providers.ResourceServer{
		{ID: testRS1ID, Name: "Server 1"},
		{ID: "rs2", Name: "Server 2"},
	}
	fileServers := []providers.ResourceServer{
		{ID: testRS1ID, Name: "Server 1 File"}, // Duplicate ID
		{ID: "rs3", Name: "Server 3"},
	}

	s.dbStoreMock.On("GetResourceServerListCount", s.ctx).Return(len(dbServers), nil)
	s.fileStoreMock.On("GetResourceServerListCount", s.ctx).Return(len(fileServers), nil)
	s.dbStoreMock.On("GetResourceServerList", s.ctx, mock.Anything, 0).Return(dbServers, nil)
	s.fileStoreMock.On("GetResourceServerList", s.ctx, mock.Anything, 0).Return(fileServers, nil)

	result, err := s.compositeStore.GetResourceServerList(s.ctx, 10, 0)

	assert.NoError(s.T(), err)
	// Should have 3 unique servers (DB rs1 takes precedence over file rs1)
	assert.Len(s.T(), result, 3)

	// Verify IsReadOnly flags are correct
	for _, rs := range result {
		if rs.ID == testRS1ID || rs.ID == "rs2" {
			assert.False(s.T(), rs.IsReadOnly, "DB resource server %s should have IsReadOnly=false", rs.ID)
		} else if rs.ID == "rs3" {
			assert.True(s.T(), rs.IsReadOnly, "File resource server %s should have IsReadOnly=true", rs.ID)
		}
	}

	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestGetResourceServerList_HandlesEmptyStoresAndSetsIsReadOnly() {
	// Both stores empty
	s.dbStoreMock.On("GetResourceServerListCount", s.ctx).Return(0, nil)
	s.fileStoreMock.On("GetResourceServerListCount", s.ctx).Return(0, nil)

	result, err := s.compositeStore.GetResourceServerList(s.ctx, 10, 0)

	assert.NoError(s.T(), err)
	assert.Empty(s.T(), result)
	s.dbStoreMock.AssertNotCalled(s.T(), "GetResourceServerList", s.ctx, mock.Anything, 0)
	s.fileStoreMock.AssertNotCalled(s.T(), "GetResourceServerList", s.ctx, mock.Anything, 0)

	// DB store has servers, file store is empty
	s.dbStoreMock.ExpectedCalls = nil
	s.fileStoreMock.ExpectedCalls = nil

	dbServers := []providers.ResourceServer{
		{ID: "rs-db1", Name: "DB Server 1"},
	}

	s.dbStoreMock.On("GetResourceServerListCount", s.ctx).Return(len(dbServers), nil)
	s.fileStoreMock.On("GetResourceServerListCount", s.ctx).Return(0, nil)
	s.dbStoreMock.On("GetResourceServerList", s.ctx, mock.Anything, 0).Return(dbServers, nil)
	s.fileStoreMock.On("GetResourceServerList", s.ctx, mock.Anything, 0).Return([]providers.ResourceServer{}, nil)

	result, err = s.compositeStore.GetResourceServerList(s.ctx, 10, 0)

	assert.NoError(s.T(), err)
	assert.Len(s.T(), result, 1)
	assert.False(s.T(), result[0].IsReadOnly, "DB resource server should have IsReadOnly=false")

	// File store has servers, DB store is empty
	s.dbStoreMock.ExpectedCalls = nil
	s.fileStoreMock.ExpectedCalls = nil

	fileServers := []providers.ResourceServer{
		{ID: "rs-file1", Name: "File Server 1"},
	}

	s.dbStoreMock.On("GetResourceServerListCount", s.ctx).Return(0, nil)
	s.fileStoreMock.On("GetResourceServerListCount", s.ctx).Return(len(fileServers), nil)
	s.dbStoreMock.On("GetResourceServerList", s.ctx, mock.Anything, 0).Return([]providers.ResourceServer{}, nil)
	s.fileStoreMock.On("GetResourceServerList", s.ctx, mock.Anything, 0).Return(fileServers, nil)

	result, err = s.compositeStore.GetResourceServerList(s.ctx, 10, 0)

	assert.NoError(s.T(), err)
	assert.Len(s.T(), result, 1)
	assert.True(s.T(), result[0].IsReadOnly, "File resource server should have IsReadOnly=true")
}

func (s *CompositeResourceStoreTestSuite) TestGetResourceServerListCount_SumsBothStores() {
	dbServers := []providers.ResourceServer{
		{ID: "rs1", Name: "Server 1"},
		{ID: "rs2", Name: "Server 2"},
	}
	fileServers := []providers.ResourceServer{
		{ID: "rs2", Name: "File Server 2"},
		{ID: "rs3", Name: "Server 3"},
	}

	s.dbStoreMock.On("GetResourceServerListCount", s.ctx).Return(len(dbServers), nil)
	s.fileStoreMock.On("GetResourceServerListCount", s.ctx).Return(len(fileServers), nil)
	s.dbStoreMock.On("GetResourceServerList", s.ctx, mock.Anything, 0).Return(dbServers, nil)
	s.fileStoreMock.On("GetResourceServerList", s.ctx, mock.Anything, 0).Return(fileServers, nil)

	count, err := s.compositeStore.GetResourceServerListCount(s.ctx)

	assert.NoError(s.T(), err)
	// Should return deduplicated count: rs1, rs2 (from db), rs3
	assert.Equal(s.T(), 3, count)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestUpdateResourceServer_DelegatesToDB() {
	rs := providers.ResourceServer{
		ID:   "rs1",
		Name: "Updated Server",
	}

	s.dbStoreMock.On("UpdateResourceServer", s.ctx, "rs1", rs).Return(nil)

	err := s.compositeStore.UpdateResourceServer(s.ctx, "rs1", rs)

	assert.NoError(s.T(), err)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertNotCalled(s.T(), "UpdateResourceServer")
}

func (s *CompositeResourceStoreTestSuite) TestDeleteResourceServer_DelegatesToDB() {
	s.dbStoreMock.On("DeleteResourceServer", s.ctx, "rs1").Return(nil)

	err := s.compositeStore.DeleteResourceServer(s.ctx, "rs1")

	assert.NoError(s.T(), err)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertNotCalled(s.T(), "DeleteResourceServer")
}

func (s *CompositeResourceStoreTestSuite) TestCheckResourceServerNameExists_ExistsInFile() {
	s.fileStoreMock.On("CheckResourceServerNameExists", s.ctx, "Test Server").Return(true, nil)

	exists, err := s.compositeStore.CheckResourceServerNameExists(s.ctx, "Test Server")

	assert.NoError(s.T(), err)
	assert.True(s.T(), exists)
	s.fileStoreMock.AssertExpectations(s.T())
	s.dbStoreMock.AssertNotCalled(s.T(), "CheckResourceServerNameExists")
}

func (s *CompositeResourceStoreTestSuite) TestCheckResourceServerNameExists_ExistsInDB() {
	s.fileStoreMock.On("CheckResourceServerNameExists", s.ctx, "Test Server").Return(false, nil)
	s.dbStoreMock.On("CheckResourceServerNameExists", s.ctx, "Test Server").Return(true, nil)

	exists, err := s.compositeStore.CheckResourceServerNameExists(s.ctx, "Test Server")

	assert.NoError(s.T(), err)
	assert.True(s.T(), exists)
	s.fileStoreMock.AssertExpectations(s.T())
	s.dbStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestCheckResourceServerNameExists_NotFound() {
	s.fileStoreMock.On("CheckResourceServerNameExists", s.ctx, "Nonexistent").Return(false, nil)
	s.dbStoreMock.On("CheckResourceServerNameExists", s.ctx, "Nonexistent").Return(false, nil)

	exists, err := s.compositeStore.CheckResourceServerNameExists(s.ctx, "Nonexistent")

	assert.NoError(s.T(), err)
	assert.False(s.T(), exists)
	s.fileStoreMock.AssertExpectations(s.T())
	s.dbStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestCheckResourceServerIdentifierExists_ExistsInFile() {
	s.fileStoreMock.On("CheckResourceServerIdentifierExists", s.ctx, "test-id").Return(true, nil)

	exists, err := s.compositeStore.CheckResourceServerIdentifierExists(s.ctx, "test-id")

	assert.NoError(s.T(), err)
	assert.True(s.T(), exists)
	s.fileStoreMock.AssertExpectations(s.T())
	s.dbStoreMock.AssertNotCalled(s.T(), "CheckResourceServerIdentifierExists")
}

func (s *CompositeResourceStoreTestSuite) TestCheckResourceServerIdentifierExists_ExistsInDB() {
	s.fileStoreMock.On("CheckResourceServerIdentifierExists", s.ctx, "test-id").Return(false, nil)
	s.dbStoreMock.On("CheckResourceServerIdentifierExists", s.ctx, "test-id").Return(true, nil)

	exists, err := s.compositeStore.CheckResourceServerIdentifierExists(s.ctx, "test-id")

	assert.NoError(s.T(), err)
	assert.True(s.T(), exists)
	s.fileStoreMock.AssertExpectations(s.T())
	s.dbStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestCheckResourceServerIdentifierExists_FileError() {
	fileErr := errors.New("file store error")
	s.fileStoreMock.On("CheckResourceServerIdentifierExists", s.ctx, "test-id").Return(false, fileErr)

	exists, err := s.compositeStore.CheckResourceServerIdentifierExists(s.ctx, "test-id")

	assert.Error(s.T(), err)
	assert.False(s.T(), exists)
	s.fileStoreMock.AssertExpectations(s.T())
	s.dbStoreMock.AssertNotCalled(s.T(), "CheckResourceServerIdentifierExists")
}

func (s *CompositeResourceStoreTestSuite) TestCheckResourceServerHasDependencies_OnlyChecksDB() {
	s.dbStoreMock.On("CheckResourceServerHasDependencies", s.ctx, "rs1").Return(true, nil)

	hasDeps, err := s.compositeStore.CheckResourceServerHasDependencies(s.ctx, "rs1")

	assert.NoError(s.T(), err)
	assert.True(s.T(), hasDeps)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertNotCalled(s.T(), "CheckResourceServerHasDependencies")
}

func (s *CompositeResourceStoreTestSuite) TestIsResourceServerDeclarative_FileServer() {
	s.fileStoreMock.On("GetResourceServer", mock.Anything, "rs-file").Return(providers.ResourceServer{}, nil)

	result := s.compositeStore.IsResourceServerDeclarative("rs-file")

	assert.True(s.T(), result)
	s.fileStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestIsResourceServerDeclarative_DBServer() {
	s.fileStoreMock.On("GetResourceServer", mock.Anything, "rs-db").
		Return(providers.ResourceServer{}, errResourceServerNotFound)

	result := s.compositeStore.IsResourceServerDeclarative("rs-db")

	assert.False(s.T(), result)
	s.fileStoreMock.AssertExpectations(s.T())
}

// Resource tests

func (s *CompositeResourceStoreTestSuite) TestCreateResource_DelegatesToDB() {
	res := providers.Resource{
		Name:   "Test Resource",
		Handle: "test",
	}

	s.dbStoreMock.On("CreateResource", s.ctx, "res1", "rs1", (*string)(nil), res).Return(nil)

	err := s.compositeStore.CreateResource(s.ctx, "res1", "rs1", nil, res)

	assert.NoError(s.T(), err)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertNotCalled(s.T(), "CreateResource")
}

func (s *CompositeResourceStoreTestSuite) TestGetResource_FoundInDB() {
	res := providers.Resource{
		ID:     "res-db",
		Name:   "DB Resource",
		Handle: "db",
	}

	s.dbStoreMock.On("GetResource", s.ctx, "res-db", "rs1").Return(res, nil)

	result, err := s.compositeStore.GetResource(s.ctx, "res-db", "rs1")

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "DB Resource", result.Name)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertNotCalled(s.T(), "GetResource")
}

func (s *CompositeResourceStoreTestSuite) TestGetResource_FoundInFile() {
	fileRes := providers.Resource{
		ID:     "res-file",
		Name:   "File Resource",
		Handle: "file",
	}

	s.dbStoreMock.On("GetResource", s.ctx, "res-file", "rs1").Return(providers.Resource{}, errResourceNotFound)
	s.fileStoreMock.On("GetResource", s.ctx, "res-file", "rs1").Return(fileRes, nil)

	result, err := s.compositeStore.GetResource(s.ctx, "res-file", "rs1")

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "File Resource", result.Name)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestGetResourceList_MergesBothStores() {
	dbResources := []providers.Resource{
		{ID: "res-db1", Name: "DB Resource 1"},
	}
	fileResources := []providers.Resource{
		{ID: "res-file1", Name: "File Resource 1"},
	}

	s.dbStoreMock.On("GetResourceListCount", s.ctx, "rs1").Return(len(dbResources), nil)
	s.fileStoreMock.On("GetResourceListCount", s.ctx, "rs1").Return(len(fileResources), nil)
	s.dbStoreMock.On("GetResourceList", s.ctx, "rs1", mock.Anything, 0).Return(dbResources, nil)
	s.fileStoreMock.On("GetResourceList", s.ctx, "rs1", mock.Anything, 0).Return(fileResources, nil)

	result, err := s.compositeStore.GetResourceList(s.ctx, "rs1", 10, 0)

	assert.NoError(s.T(), err)
	assert.Len(s.T(), result, 2)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestGetResourceListByParent_MergesBothStores() {
	parentID := testParentID
	dbResources := []providers.Resource{
		{ID: "res-db1", Name: "DB Resource 1"},
	}
	fileResources := []providers.Resource{
		{ID: "res-file1", Name: "File Resource 1"},
	}

	s.dbStoreMock.On("GetResourceListCountByParent", s.ctx, "rs1", &parentID).Return(len(dbResources), nil)
	s.fileStoreMock.On("GetResourceListCountByParent", s.ctx, "rs1", &parentID).Return(len(fileResources), nil)
	s.dbStoreMock.On(
		"GetResourceListByParent", s.ctx, "rs1", &parentID, mock.Anything, 0,
	).Return(dbResources, nil)
	s.fileStoreMock.On(
		"GetResourceListByParent", s.ctx, "rs1", &parentID, mock.Anything, 0,
	).Return(fileResources, nil)

	result, err := s.compositeStore.GetResourceListByParent(s.ctx, "rs1", &parentID, 10, 0)

	assert.NoError(s.T(), err)
	assert.Len(s.T(), result, 2)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestGetResourceListByParent_DBError() {
	parentID := testParentID
	dbErr := errors.New("db error")

	s.dbStoreMock.On("GetResourceListCountByParent", s.ctx, "rs1", &parentID).Return(0, dbErr)

	result, err := s.compositeStore.GetResourceListByParent(s.ctx, "rs1", &parentID, 10, 0)

	assert.Error(s.T(), err)
	assert.Nil(s.T(), result)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertNotCalled(s.T(), "GetResourceListByParent")
}

func (s *CompositeResourceStoreTestSuite) TestGetResourceListByParent_FileError() {
	parentID := testParentID
	fileErr := errors.New("file error")

	s.dbStoreMock.On("GetResourceListCountByParent", s.ctx, "rs1", &parentID).Return(0, nil)
	s.fileStoreMock.On("GetResourceListCountByParent", s.ctx, "rs1", &parentID).Return(0, fileErr)

	result, err := s.compositeStore.GetResourceListByParent(s.ctx, "rs1", &parentID, 10, 0)

	assert.Error(s.T(), err)
	assert.Nil(s.T(), result)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertExpectations(s.T())
	s.dbStoreMock.AssertNotCalled(s.T(), "GetResourceListByParent")
	s.fileStoreMock.AssertNotCalled(s.T(), "GetResourceListByParent")
}

func (s *CompositeResourceStoreTestSuite) TestGetResourceListCount_SumsBothStores() {
	dbResources := []providers.Resource{
		{ID: "res1", Name: "Resource 1"},
		{ID: "res2", Name: "Resource 2"},
	}
	fileResources := []providers.Resource{
		{ID: "res2", Name: "File Resource 2"},
		{ID: "res3", Name: "Resource 3"},
	}

	s.dbStoreMock.On("GetResourceListCount", s.ctx, "rs1").Return(len(dbResources), nil)
	s.fileStoreMock.On("GetResourceListCount", s.ctx, "rs1").Return(len(fileResources), nil)
	s.dbStoreMock.On("GetResourceList", s.ctx, "rs1", mock.Anything, 0).Return(dbResources, nil)
	s.fileStoreMock.On("GetResourceList", s.ctx, "rs1", mock.Anything, 0).Return(fileResources, nil)

	count, err := s.compositeStore.GetResourceListCount(s.ctx, "rs1")

	assert.NoError(s.T(), err)
	// Should return deduplicated count: res1, res2 (from db), res3
	assert.Equal(s.T(), 3, count)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestGetResourceListCount_DBError() {
	dbErr := errors.New("db error")
	s.dbStoreMock.On("GetResourceListCount", s.ctx, "rs1").Return(0, dbErr)

	count, err := s.compositeStore.GetResourceListCount(s.ctx, "rs1")

	assert.Error(s.T(), err)
	assert.Equal(s.T(), 0, count)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertNotCalled(s.T(), "GetResourceList")
}

func (s *CompositeResourceStoreTestSuite) TestGetResourceListCount_FileError() {
	fileErr := errors.New("file error")
	s.dbStoreMock.On("GetResourceListCount", s.ctx, "rs1").Return(0, nil)
	s.fileStoreMock.On("GetResourceListCount", s.ctx, "rs1").Return(0, fileErr)

	count, err := s.compositeStore.GetResourceListCount(s.ctx, "rs1")

	assert.Error(s.T(), err)
	assert.Equal(s.T(), 0, count)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertExpectations(s.T())
	s.dbStoreMock.AssertNotCalled(s.T(), "GetResourceList")
	s.fileStoreMock.AssertNotCalled(s.T(), "GetResourceList")
}

func (s *CompositeResourceStoreTestSuite) TestGetResourceListCountByParent_SumsBothStores() {
	parentID := "parent-2"
	dbResources := []providers.Resource{
		{ID: "res1", Name: "Resource 1"},
		{ID: "res2", Name: "Resource 2"},
	}
	fileResources := []providers.Resource{
		{ID: "res2", Name: "File Resource 2"},
		{ID: "res3", Name: "Resource 3"},
	}

	s.dbStoreMock.On("GetResourceListCountByParent", s.ctx, "rs1", &parentID).Return(len(dbResources), nil)
	s.fileStoreMock.On("GetResourceListCountByParent", s.ctx, "rs1", &parentID).Return(len(fileResources), nil)
	s.dbStoreMock.On(
		"GetResourceListByParent", s.ctx, "rs1", &parentID, mock.Anything, 0,
	).Return(dbResources, nil)
	s.fileStoreMock.On(
		"GetResourceListByParent", s.ctx, "rs1", &parentID, mock.Anything, 0,
	).Return(fileResources, nil)

	count, err := s.compositeStore.GetResourceListCountByParent(s.ctx, "rs1", &parentID)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 3, count)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestUpdateResource_DelegatesToDB() {
	res := providers.Resource{
		Name: "Updated Resource",
	}

	s.dbStoreMock.On("UpdateResource", s.ctx, "res1", "rs1", res).Return(nil)

	err := s.compositeStore.UpdateResource(s.ctx, "res1", "rs1", res)

	assert.NoError(s.T(), err)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertNotCalled(s.T(), "UpdateResource")
}

func (s *CompositeResourceStoreTestSuite) TestDeleteResource_DelegatesToDB() {
	s.dbStoreMock.On("DeleteResource", s.ctx, "res1", "rs1").Return(nil)

	err := s.compositeStore.DeleteResource(s.ctx, "res1", "rs1")

	assert.NoError(s.T(), err)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertNotCalled(s.T(), "DeleteResource")
}

func (s *CompositeResourceStoreTestSuite) TestCheckResourceHandleExists_FileTrue() {
	parentID := "parent-3"
	s.fileStoreMock.On(
		"CheckResourceHandleExists", s.ctx, "rs1", "res-handle", &parentID,
	).Return(true, nil)

	exists, err := s.compositeStore.CheckResourceHandleExists(s.ctx, "rs1", "res-handle", &parentID)

	assert.NoError(s.T(), err)
	assert.True(s.T(), exists)
	s.fileStoreMock.AssertExpectations(s.T())
	s.dbStoreMock.AssertNotCalled(s.T(), "CheckResourceHandleExists")
}

func (s *CompositeResourceStoreTestSuite) TestCheckResourceHandleExists_DBTrue() {
	parentID := "parent-4"
	s.fileStoreMock.On(
		"CheckResourceHandleExists", s.ctx, "rs1", "res-handle", &parentID,
	).Return(false, nil)
	s.dbStoreMock.On(
		"CheckResourceHandleExists", s.ctx, "rs1", "res-handle", &parentID,
	).Return(true, nil)

	exists, err := s.compositeStore.CheckResourceHandleExists(s.ctx, "rs1", "res-handle", &parentID)

	assert.NoError(s.T(), err)
	assert.True(s.T(), exists)
	s.fileStoreMock.AssertExpectations(s.T())
	s.dbStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestCheckResourceHandleExists_FileError() {
	parentID := "parent-5"
	fileErr := errors.New("file store error")
	s.fileStoreMock.On(
		"CheckResourceHandleExists", s.ctx, "rs1", "res-handle", &parentID,
	).Return(false, fileErr)

	exists, err := s.compositeStore.CheckResourceHandleExists(s.ctx, "rs1", "res-handle", &parentID)

	assert.Error(s.T(), err)
	assert.False(s.T(), exists)
	s.fileStoreMock.AssertExpectations(s.T())
	s.dbStoreMock.AssertNotCalled(s.T(), "CheckResourceHandleExists")
}

func (s *CompositeResourceStoreTestSuite) TestCheckResourceHasDependencies_DelegatesToDB() {
	s.dbStoreMock.On("CheckResourceHasDependencies", s.ctx, "res1").Return(true, nil)

	hasDeps, err := s.compositeStore.CheckResourceHasDependencies(s.ctx, "res1")

	assert.NoError(s.T(), err)
	assert.True(s.T(), hasDeps)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertNotCalled(s.T(), "CheckResourceHasDependencies")
}

func (s *CompositeResourceStoreTestSuite) TestCheckCircularDependency_DelegatesToDB() {
	s.dbStoreMock.On("CheckCircularDependency", s.ctx, "res1", "parent-1").Return(false, nil)

	hasCircular, err := s.compositeStore.CheckCircularDependency(s.ctx, "res1", "parent-1")

	assert.NoError(s.T(), err)
	assert.False(s.T(), hasCircular)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertNotCalled(s.T(), "CheckCircularDependency")
}

// Action tests

func (s *CompositeResourceStoreTestSuite) TestCreateAction_DelegatesToDB() {
	action := providers.Action{
		Name:   "Test Action",
		Handle: "test",
	}
	var resID *string = nil

	s.dbStoreMock.On("CreateAction", s.ctx, "act1", "rs1", resID, action).Return(nil)

	err := s.compositeStore.CreateAction(s.ctx, "act1", "rs1", resID, action)

	assert.NoError(s.T(), err)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertNotCalled(s.T(), "CreateAction")
}

func (s *CompositeResourceStoreTestSuite) TestGetAction_FoundInDB() {
	action := providers.Action{
		ID:     "act-db",
		Name:   "DB Action",
		Handle: "db",
	}

	s.dbStoreMock.On("GetAction", s.ctx, "act-db", "rs1", (*string)(nil)).Return(action, nil)

	result, err := s.compositeStore.GetAction(s.ctx, "act-db", "rs1", nil)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "DB Action", result.Name)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertNotCalled(s.T(), "GetAction")
}

func (s *CompositeResourceStoreTestSuite) TestGetAction_FoundInFile() {
	fileAction := providers.Action{
		ID:     "act-file",
		Name:   "File Action",
		Handle: "file",
	}

	s.dbStoreMock.On("GetAction", s.ctx, "act-file", "rs1", (*string)(nil)).
		Return(providers.Action{}, errActionNotFound)
	s.fileStoreMock.On("GetAction", s.ctx, "act-file", "rs1", (*string)(nil)).Return(fileAction, nil)

	result, err := s.compositeStore.GetAction(s.ctx, "act-file", "rs1", nil)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "File Action", result.Name)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestGetActionList_MergesBothStores() {
	dbActions := []providers.Action{
		{ID: "act-db1", Name: "DB Action 1"},
	}
	fileActions := []providers.Action{
		{ID: "act-file1", Name: "File Action 1"},
	}

	s.dbStoreMock.On("GetActionListCount", s.ctx, "rs1", (*string)(nil), providers.ActionKind("")).
		Return(len(dbActions), nil)
	s.fileStoreMock.On("GetActionListCount", s.ctx, "rs1", (*string)(nil), providers.ActionKind("")).
		Return(len(fileActions), nil)
	s.dbStoreMock.On("GetActionList", s.ctx, "rs1", (*string)(nil), providers.ActionKind(""), mock.Anything, 0).
		Return(dbActions, nil)
	s.fileStoreMock.On("GetActionList", s.ctx, "rs1", (*string)(nil), providers.ActionKind(""), mock.Anything, 0).
		Return(fileActions, nil)

	result, err := s.compositeStore.GetActionList(s.ctx, "rs1", nil, "", 10, 0)

	assert.NoError(s.T(), err)
	assert.Len(s.T(), result, 2)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestGetActionList_ThreadsKindToBothStores() {
	dbActions := []providers.Action{
		{ID: "act-db1", Name: "DB Tool", Kind: providers.ActionKindTool},
	}
	fileActions := []providers.Action{
		{ID: "act-file1", Name: "File Tool", Kind: providers.ActionKindTool},
	}

	s.dbStoreMock.On("GetActionListCount", s.ctx, "rs1", (*string)(nil), providers.ActionKindTool).
		Return(len(dbActions), nil)
	s.fileStoreMock.On("GetActionListCount", s.ctx, "rs1", (*string)(nil), providers.ActionKindTool).
		Return(len(fileActions), nil)
	s.dbStoreMock.On("GetActionList", s.ctx, "rs1", (*string)(nil), providers.ActionKindTool, mock.Anything, 0).
		Return(dbActions, nil)
	s.fileStoreMock.On("GetActionList", s.ctx, "rs1", (*string)(nil), providers.ActionKindTool, mock.Anything, 0).
		Return(fileActions, nil)

	result, err := s.compositeStore.GetActionList(s.ctx, "rs1", nil, providers.ActionKindTool, 10, 0)

	assert.NoError(s.T(), err)
	assert.Len(s.T(), result, 2)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestGetActionList_DBError() {
	dbErr := errors.New("db error")
	s.dbStoreMock.On("GetActionListCount", s.ctx, "rs1", (*string)(nil), providers.ActionKind("")).Return(1, nil)
	s.fileStoreMock.On("GetActionListCount", s.ctx, "rs1", (*string)(nil), providers.ActionKind("")).Return(0, nil)
	s.dbStoreMock.On("GetActionList", s.ctx, "rs1", (*string)(nil), providers.ActionKind(""), mock.Anything, 0).
		Return(nil, dbErr)

	result, err := s.compositeStore.GetActionList(s.ctx, "rs1", nil, "", 10, 0)

	assert.Error(s.T(), err)
	assert.Nil(s.T(), result)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertNotCalled(s.T(), "GetActionList")
}

func (s *CompositeResourceStoreTestSuite) TestGetActionList_FileError() {
	fileErr := errors.New("file error")
	s.dbStoreMock.On("GetActionListCount", s.ctx, "rs1", (*string)(nil), providers.ActionKind("")).Return(0, nil)
	s.fileStoreMock.On("GetActionListCount", s.ctx, "rs1", (*string)(nil), providers.ActionKind("")).Return(1, nil)
	s.dbStoreMock.On("GetActionList", s.ctx, "rs1", (*string)(nil), providers.ActionKind(""), mock.Anything, 0).
		Return([]providers.Action{}, nil)
	s.fileStoreMock.On("GetActionList", s.ctx, "rs1", (*string)(nil), providers.ActionKind(""), mock.Anything, 0).
		Return(nil, fileErr)

	result, err := s.compositeStore.GetActionList(s.ctx, "rs1", nil, "", 10, 0)

	assert.Error(s.T(), err)
	assert.Nil(s.T(), result)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestGetActionListCount_SumsBothStores() {
	dbActions := []providers.Action{
		{ID: "act1", Name: "Action 1"},
		{ID: "act2", Name: "Action 2"},
	}
	fileActions := []providers.Action{
		{ID: "act2", Name: "File Action 2"},
		{ID: "act3", Name: "Action 3"},
	}

	s.dbStoreMock.On("GetActionListCount", s.ctx, "rs1", (*string)(nil), providers.ActionKind("")).
		Return(len(dbActions), nil)
	s.fileStoreMock.On("GetActionListCount", s.ctx, "rs1", (*string)(nil), providers.ActionKind("")).
		Return(len(fileActions), nil)
	s.dbStoreMock.On("GetActionList", s.ctx, "rs1", (*string)(nil), providers.ActionKind(""), mock.Anything, 0).
		Return(dbActions, nil)
	s.fileStoreMock.On("GetActionList", s.ctx, "rs1", (*string)(nil), providers.ActionKind(""), mock.Anything, 0).
		Return(fileActions, nil)

	count, err := s.compositeStore.GetActionListCount(s.ctx, "rs1", nil, "")

	assert.NoError(s.T(), err)
	// Should return deduplicated count: act1, act2 (from db), act3
	assert.Equal(s.T(), 3, count)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestGetActionListCount_FilteredByKind() {
	dbActions := []providers.Action{
		{ID: "act1", Name: "DB Tool", Kind: providers.ActionKindTool},
	}
	fileActions := []providers.Action{
		{ID: "act2", Name: "File Tool", Kind: providers.ActionKindTool},
	}

	s.dbStoreMock.On("GetActionListCount", s.ctx, "rs1", (*string)(nil), providers.ActionKindTool).
		Return(len(dbActions), nil)
	s.fileStoreMock.On("GetActionListCount", s.ctx, "rs1", (*string)(nil), providers.ActionKindTool).
		Return(len(fileActions), nil)
	s.dbStoreMock.On("GetActionList", s.ctx, "rs1", (*string)(nil), providers.ActionKindTool, mock.Anything, 0).
		Return(dbActions, nil)
	s.fileStoreMock.On("GetActionList", s.ctx, "rs1", (*string)(nil), providers.ActionKindTool, mock.Anything, 0).
		Return(fileActions, nil)

	count, err := s.compositeStore.GetActionListCount(s.ctx, "rs1", nil, providers.ActionKindTool)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 2, count)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestGetActionListCount_DBError() {
	dbErr := errors.New("db error")
	s.dbStoreMock.On("GetActionListCount", s.ctx, "rs1", (*string)(nil), providers.ActionKind("")).Return(0, dbErr)

	count, err := s.compositeStore.GetActionListCount(s.ctx, "rs1", nil, "")

	assert.Error(s.T(), err)
	assert.Equal(s.T(), 0, count)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertNotCalled(s.T(), "GetActionList")
}

func (s *CompositeResourceStoreTestSuite) TestGetActionListCount_FileError() {
	fileErr := errors.New("file error")
	s.dbStoreMock.On("GetActionListCount", s.ctx, "rs1", (*string)(nil), providers.ActionKind("")).Return(0, nil)
	s.fileStoreMock.On("GetActionListCount", s.ctx, "rs1", (*string)(nil), providers.ActionKind("")).Return(0, fileErr)

	count, err := s.compositeStore.GetActionListCount(s.ctx, "rs1", nil, "")

	assert.Error(s.T(), err)
	assert.Equal(s.T(), 0, count)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertExpectations(s.T())
	s.dbStoreMock.AssertNotCalled(s.T(), "GetActionList")
	s.fileStoreMock.AssertNotCalled(s.T(), "GetActionList")
}

func (s *CompositeResourceStoreTestSuite) TestUpdateAction_DelegatesToDB() {
	action := providers.Action{
		Name: "Updated Action",
	}

	s.dbStoreMock.On("UpdateAction", s.ctx, "act1", "rs1", (*string)(nil), action).Return(nil)

	err := s.compositeStore.UpdateAction(s.ctx, "act1", "rs1", nil, action)

	assert.NoError(s.T(), err)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertNotCalled(s.T(), "UpdateAction")
}

func (s *CompositeResourceStoreTestSuite) TestDeleteAction_DelegatesToDB() {
	s.dbStoreMock.On("DeleteAction", s.ctx, "act1", "rs1", (*string)(nil)).Return(nil)

	err := s.compositeStore.DeleteAction(s.ctx, "act1", "rs1", nil)

	assert.NoError(s.T(), err)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertNotCalled(s.T(), "DeleteAction")
}

func (s *CompositeResourceStoreTestSuite) TestIsActionExist_FileTrue() {
	s.fileStoreMock.On("IsActionExist", s.ctx, "act1", "rs1", (*string)(nil)).Return(true, nil)

	exists, err := s.compositeStore.IsActionExist(s.ctx, "act1", "rs1", nil)

	assert.NoError(s.T(), err)
	assert.True(s.T(), exists)
	s.fileStoreMock.AssertExpectations(s.T())
	s.dbStoreMock.AssertNotCalled(s.T(), "IsActionExist")
}

func (s *CompositeResourceStoreTestSuite) TestIsActionExist_DBTrue() {
	s.fileStoreMock.On("IsActionExist", s.ctx, "act1", "rs1", (*string)(nil)).Return(false, nil)
	s.dbStoreMock.On("IsActionExist", s.ctx, "act1", "rs1", (*string)(nil)).Return(true, nil)

	exists, err := s.compositeStore.IsActionExist(s.ctx, "act1", "rs1", nil)

	assert.NoError(s.T(), err)
	assert.True(s.T(), exists)
	s.fileStoreMock.AssertExpectations(s.T())
	s.dbStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestIsActionExist_FileError() {
	fileErr := errors.New("file store error")
	s.fileStoreMock.On("IsActionExist", s.ctx, "act1", "rs1", (*string)(nil)).Return(false, fileErr)

	exists, err := s.compositeStore.IsActionExist(s.ctx, "act1", "rs1", nil)

	assert.Error(s.T(), err)
	assert.False(s.T(), exists)
	s.fileStoreMock.AssertExpectations(s.T())
	s.dbStoreMock.AssertNotCalled(s.T(), "IsActionExist")
}

func (s *CompositeResourceStoreTestSuite) TestCheckActionHandleExists_FileTrue() {
	s.fileStoreMock.On(
		"CheckActionHandleExists", s.ctx, "rs1", (*string)(nil), "read",
	).Return(true, nil)

	exists, err := s.compositeStore.CheckActionHandleExists(s.ctx, "rs1", nil, "read")

	assert.NoError(s.T(), err)
	assert.True(s.T(), exists)
	s.fileStoreMock.AssertExpectations(s.T())
	s.dbStoreMock.AssertNotCalled(s.T(), "CheckActionHandleExists")
}

func (s *CompositeResourceStoreTestSuite) TestCheckActionHandleExists_DBTrue() {
	s.fileStoreMock.On(
		"CheckActionHandleExists", s.ctx, "rs1", (*string)(nil), "read",
	).Return(false, nil)
	s.dbStoreMock.On(
		"CheckActionHandleExists", s.ctx, "rs1", (*string)(nil), "read",
	).Return(true, nil)

	exists, err := s.compositeStore.CheckActionHandleExists(s.ctx, "rs1", nil, "read")

	assert.NoError(s.T(), err)
	assert.True(s.T(), exists)
	s.fileStoreMock.AssertExpectations(s.T())
	s.dbStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestCheckActionHandleExists_FileError() {
	fileErr := errors.New("file store error")
	s.fileStoreMock.On(
		"CheckActionHandleExists", s.ctx, "rs1", (*string)(nil), "read",
	).Return(false, fileErr)

	exists, err := s.compositeStore.CheckActionHandleExists(s.ctx, "rs1", nil, "read")

	assert.Error(s.T(), err)
	assert.False(s.T(), exists)
	s.fileStoreMock.AssertExpectations(s.T())
	s.dbStoreMock.AssertNotCalled(s.T(), "CheckActionHandleExists")
}

func (s *CompositeResourceStoreTestSuite) TestValidatePermissions_DelegatesToDB() {
	permissions := []string{"perm1", "perm2", "perm3"}

	s.dbStoreMock.On("ValidatePermissions", mock.Anything, "rs1", permissions).Return([]string{}, nil)
	s.fileStoreMock.On("ValidatePermissions", mock.Anything, "rs1", permissions).Return([]string{}, nil)

	invalid, err := s.compositeStore.ValidatePermissions(s.ctx, "rs1", permissions)

	assert.NoError(s.T(), err)
	assert.Len(s.T(), invalid, 0)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertExpectations(s.T())
}

func (s *CompositeResourceStoreTestSuite) TestValidatePermissions_DBError() {
	permissions := []string{"perm1", "perm2"}
	dbErr := errors.New("db error")

	s.dbStoreMock.On("ValidatePermissions", mock.Anything, "rs1", permissions).Return(nil, dbErr)

	invalid, err := s.compositeStore.ValidatePermissions(s.ctx, "rs1", permissions)

	assert.Error(s.T(), err)
	assert.Nil(s.T(), invalid)
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertNotCalled(s.T(), "ValidatePermissions")
}

func (s *CompositeResourceStoreTestSuite) TestValidatePermissions_SomeInvalid() {
	permissions := []string{"perm1", "perm2", "perm3"}

	// Test intersection: perm2 is invalid in both stores, perm1 only in db, perm3 only in file
	s.dbStoreMock.On("ValidatePermissions", mock.Anything, "rs1", permissions).Return([]string{"perm1", "perm2"}, nil)
	s.fileStoreMock.On("ValidatePermissions", mock.Anything, "rs1", permissions).Return([]string{"perm2", "perm3"}, nil)

	invalid, err := s.compositeStore.ValidatePermissions(s.ctx, "rs1", permissions)

	assert.NoError(s.T(), err)
	// Only perm2 is in both invalid lists (intersection)
	assert.Len(s.T(), invalid, 1)
	assert.Equal(s.T(), "perm2", invalid[0])
	s.dbStoreMock.AssertExpectations(s.T())
	s.fileStoreMock.AssertExpectations(s.T())
}

// TestMergeAndDeduplicateResourceServers tests the merge helper function for resource servers.
func (s *CompositeResourceStoreTestSuite) TestMergeAndDeduplicateResourceServers_MarksCorrectIsReadOnly() {
	dbServers := []providers.ResourceServer{
		{ID: "rs-db1", Name: "DB 1"},
		{ID: "rs-db2", Name: "DB 2"},
	}
	fileServers := []providers.ResourceServer{
		{ID: "rs-file1", Name: "File 1"},
		{ID: "rs-file2", Name: "File 2"},
	}

	result := mergeAndDeduplicateResourceServers(dbServers, fileServers)

	assert.Len(s.T(), result, 4)

	// Verify IsReadOnly flags are correct
	for _, rs := range result {
		if rs.ID == "rs-db1" || rs.ID == "rs-db2" {
			assert.False(s.T(), rs.IsReadOnly, "DB resource server %s should have IsReadOnly=false", rs.ID)
		} else if rs.ID == "rs-file1" || rs.ID == "rs-file2" {
			assert.True(s.T(), rs.IsReadOnly, "File resource server %s should have IsReadOnly=true", rs.ID)
		}
	}
}

func (s *CompositeResourceStoreTestSuite) TestMergeAndDeduplicateResourceServers_DBTakesPrecedenceAndIsNotReadOnly() {
	dbServers := []providers.ResourceServer{
		{ID: "duplicate", Name: "DB Server"},
	}
	fileServers := []providers.ResourceServer{
		{ID: "duplicate", Name: "File Server"},
	}

	result := mergeAndDeduplicateResourceServers(dbServers, fileServers)

	assert.Len(s.T(), result, 1)
	assert.Equal(s.T(), "DB Server", result[0].Name)
	assert.False(s.T(), result[0].IsReadOnly, "DB resource server should be marked as mutable (IsReadOnly=false)")
}

func (s *CompositeResourceStoreTestSuite) TestMergeAndDeduplicateResourceServers_HandlesEmptySlices() {
	// Both empty
	result := mergeAndDeduplicateResourceServers([]providers.ResourceServer{}, []providers.ResourceServer{})
	assert.Empty(s.T(), result)

	// DB has servers, file is empty
	dbServers := []providers.ResourceServer{{ID: "rs-db1", Name: "DB 1"}}
	result = mergeAndDeduplicateResourceServers(dbServers, []providers.ResourceServer{})
	assert.Len(s.T(), result, 1)
	assert.False(s.T(), result[0].IsReadOnly, "DB resource server should have IsReadOnly=false")

	// File has servers, DB is empty
	fileServers := []providers.ResourceServer{{ID: "rs-file1", Name: "File 1"}}
	result = mergeAndDeduplicateResourceServers([]providers.ResourceServer{}, fileServers)
	assert.Len(s.T(), result, 1)
	assert.True(s.T(), result[0].IsReadOnly, "File resource server should have IsReadOnly=true")
}
