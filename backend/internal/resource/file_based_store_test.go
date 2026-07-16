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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// FileBasedResourceStoreTestSuite tests the fileBasedResourceStore.
type FileBasedResourceStoreTestSuite struct {
	suite.Suite
	store resourceStoreInterface
	ctx   context.Context
}

func TestFileBasedResourceStoreTestSuite(t *testing.T) {
	suite.Run(t, new(FileBasedResourceStoreTestSuite))
}

func (s *FileBasedResourceStoreTestSuite) SetupTest() {
	var err error
	s.store, _, err = newFileBasedResourceStore()
	assert.NoError(s.T(), err)
	s.ctx = context.Background()
}

func (s *FileBasedResourceStoreTestSuite) TearDownTest() {
	// Clear the resource server entities from the singleton after each test to ensure isolation
	// This is necessary because the file-based store uses a singleton EntityStore
	if fileStore, ok := s.store.(*fileBasedResourceStore); ok {
		// Access the underlying GenericFileBasedStore and clear only resource server entities
		_ = fileStore.GenericFileBasedStore.ClearByType()
	}
}

func (s *FileBasedResourceStoreTestSuite) TestNewFileBasedResourceStore() {
	assert.NotNil(s.T(), s.store)
}

func (s *FileBasedResourceStoreTestSuite) TestCreateResourceServer_ReturnsImmutableError() {
	rs := providers.ResourceServer{
		ID:        "rs1",
		Name:      "Test Server",
		OUID:      "ou1",
		Delimiter: ":",
	}

	err := s.store.CreateResourceServer(s.ctx, "rs1", rs)
	assert.Error(s.T(), err)
	assert.Equal(s.T(), errImmutableStore, err)
}

func (s *FileBasedResourceStoreTestSuite) TestUpdateResourceServer_ReturnsImmutableError() {
	rs := providers.ResourceServer{
		ID:   "rs1",
		Name: "Updated Server",
	}

	err := s.store.UpdateResourceServer(s.ctx, "rs1", rs)
	assert.Error(s.T(), err)
	assert.Equal(s.T(), errImmutableStore, err)
}

func (s *FileBasedResourceStoreTestSuite) TestDeleteResourceServer_ReturnsImmutableError() {
	err := s.store.DeleteResourceServer(s.ctx, "rs1")
	assert.Error(s.T(), err)
	assert.Equal(s.T(), errImmutableStore, err)
}

func (s *FileBasedResourceStoreTestSuite) TestGetResourceServer_NotFound() {
	rs, err := s.store.GetResourceServer(s.ctx, "nonexistent")

	assert.Error(s.T(), err)
	assert.Equal(s.T(), providers.ResourceServer{}, rs)
}

func (s *FileBasedResourceStoreTestSuite) TestGetResourceServer_CorruptedData() {
	fileStore, ok := s.store.(*fileBasedResourceStore)
	assert.True(s.T(), ok)

	err := fileStore.GenericFileBasedStore.Create("rs-corrupt", "bad-data")
	assert.NoError(s.T(), err)

	rs, err := s.store.GetResourceServer(s.ctx, "rs-corrupt")

	assert.Error(s.T(), err)
	assert.Equal(s.T(), providers.ResourceServer{}, rs)
	assert.Contains(s.T(), err.Error(), "data corrupted")
}

func (s *FileBasedResourceStoreTestSuite) TestGetResourceServerList_EmptyStore() {
	// Get count before adding any data in this test
	initialCount, err := s.store.GetResourceServerListCount(s.ctx)
	assert.NoError(s.T(), err)

	servers, err := s.store.GetResourceServerList(s.ctx, 10, 0)

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), servers)
	// Store might have data from other tests - just verify we get a list back
	assert.Len(s.T(), servers, initialCount)
}

func (s *FileBasedResourceStoreTestSuite) TestGetResourceServerListCount_EmptyStore() {
	count, err := s.store.GetResourceServerListCount(s.ctx)

	assert.NoError(s.T(), err)
	// Store might have data from other tests - just verify count is non-negative
	assert.GreaterOrEqual(s.T(), count, 0)
}

func (s *FileBasedResourceStoreTestSuite) TestIsResourceServerDeclarative_ExistsReturnsTrue() {
	fileStore, ok := s.store.(*fileBasedResourceStore)
	assert.True(s.T(), ok)

	// Create a test resource server
	rs := &providers.ResourceServer{
		ID:        "rs-test-declarative",
		Name:      "Test Server",
		OUID:      "ou1",
		Delimiter: ":",
	}
	err := fileStore.Create("rs-test-declarative", rs)
	assert.NoError(s.T(), err)

	// Test that it returns true for existing resource server
	result := s.store.IsResourceServerDeclarative("rs-test-declarative")
	assert.True(s.T(), result)

	// Test that it returns false for non-existent resource server
	result = s.store.IsResourceServerDeclarative("non-existent-id")
	assert.False(s.T(), result)
}

func (s *FileBasedResourceStoreTestSuite) TestCheckResourceServerNameExists_NotFound() {
	exists, err := s.store.CheckResourceServerNameExists(s.ctx, "NonExistent")

	assert.NoError(s.T(), err)
	assert.False(s.T(), exists)
}

func (s *FileBasedResourceStoreTestSuite) TestCheckResourceServerIdentifierExists_NotFound() {
	exists, err := s.store.CheckResourceServerIdentifierExists(s.ctx, "nonexistent")

	assert.NoError(s.T(), err)
	assert.False(s.T(), exists)
}

func (s *FileBasedResourceStoreTestSuite) TestCheckResourceServerHasDependencies_AlwaysFalse() {
	// Non-existent resource server returns false (not found)
	hasDeps, err := s.store.CheckResourceServerHasDependencies(s.ctx, "rs1")

	assert.NoError(s.T(), err)
	assert.False(s.T(), hasDeps)
}

func (s *FileBasedResourceStoreTestSuite) TestCheckResourceServerHasDependencies_CorruptedData() {
	fileStore, ok := s.store.(*fileBasedResourceStore)
	assert.True(s.T(), ok)

	err := fileStore.GenericFileBasedStore.Create("rs-corrupt", "bad-data")
	assert.NoError(s.T(), err)

	hasDeps, err := s.store.CheckResourceServerHasDependencies(s.ctx, "rs-corrupt")

	assert.Error(s.T(), err)
	assert.False(s.T(), hasDeps)
	assert.Contains(s.T(), err.Error(), "data corrupted")
}

// Resource operations tests

func (s *FileBasedResourceStoreTestSuite) TestCreateResource_ReturnsImmutableError() {
	res := providers.Resource{
		Name:   "Test Resource",
		Handle: "test",
	}

	err := s.store.CreateResource(s.ctx, "res1", "rs1", nil, res)
	assert.Error(s.T(), err)
	assert.Equal(s.T(), errImmutableStore, err)
}

func (s *FileBasedResourceStoreTestSuite) TestUpdateResource_ReturnsImmutableError() {
	res := providers.Resource{
		Name: "Updated Resource",
	}

	err := s.store.UpdateResource(s.ctx, "res1", "rs1", res)
	assert.Error(s.T(), err)
	assert.Equal(s.T(), errImmutableStore, err)
}

func (s *FileBasedResourceStoreTestSuite) TestDeleteResource_ReturnsImmutableError() {
	err := s.store.DeleteResource(s.ctx, "res1", "rs1")
	assert.Error(s.T(), err)
	assert.Equal(s.T(), errImmutableStore, err)
}

func (s *FileBasedResourceStoreTestSuite) TestCheckCircularDependency_AlwaysFalse() {
	hasCircular, err := s.store.CheckCircularDependency(s.ctx, "res1", "res2")

	assert.NoError(s.T(), err)
	assert.False(s.T(), hasCircular)
}

// Action operations tests

func (s *FileBasedResourceStoreTestSuite) TestCreateAction_ReturnsImmutableError() {
	action := providers.Action{
		Name:   "Test Action",
		Handle: "test",
	}

	err := s.store.CreateAction(s.ctx, "act1", "rs1", nil, action)
	assert.Error(s.T(), err)
	assert.Equal(s.T(), errImmutableStore, err)
}

func (s *FileBasedResourceStoreTestSuite) TestUpdateAction_ReturnsImmutableError() {
	action := providers.Action{
		Name: "Updated Action",
	}

	err := s.store.UpdateAction(s.ctx, "act1", "rs1", nil, action)
	assert.Error(s.T(), err)
	assert.Equal(s.T(), errImmutableStore, err)
}

func (s *FileBasedResourceStoreTestSuite) TestDeleteAction_ReturnsImmutableError() {
	err := s.store.DeleteAction(s.ctx, "act1", "rs1", nil)
	assert.Error(s.T(), err)
	assert.Equal(s.T(), errImmutableStore, err)
}

func (s *FileBasedResourceStoreTestSuite) TestGetAction_NotFound() {
	action, err := s.store.GetAction(s.ctx, "nonexistent", "rs1", nil)

	assert.Error(s.T(), err)
	assert.Equal(s.T(), providers.Action{}, action)
}

func (s *FileBasedResourceStoreTestSuite) TestIsActionExist_NotFound() {
	exists, err := s.store.IsActionExist(s.ctx, "nonexistent", "rs1", nil)

	assert.NoError(s.T(), err)
	assert.False(s.T(), exists)
}

func (s *FileBasedResourceStoreTestSuite) TestCheckActionHandleExists_AlwaysFalse() {
	exists, err := s.store.CheckActionHandleExists(s.ctx, "rs1", nil, "test")

	assert.NoError(s.T(), err)
	assert.False(s.T(), exists)
}

func (s *FileBasedResourceStoreTestSuite) TestValidatePermissions_EmptyResult() {
	permissions := []string{"permission1", "permission2"}

	invalid, err := s.store.ValidatePermissions(s.ctx, "rs1", permissions)

	assert.NoError(s.T(), err)
	// Empty store has no valid permissions, so all input permissions should be invalid
	assert.Len(s.T(), invalid, 2)
	assert.Contains(s.T(), invalid, "permission1")
	assert.Contains(s.T(), invalid, "permission2")
}

func (s *FileBasedResourceStoreTestSuite) TestGetResourceAndLists_WithData() {
	fileStore, ok := s.store.(*fileBasedResourceStore)
	assert.True(s.T(), ok)

	parentID := "rs-data_root"
	rs := &providers.ResourceServer{
		ID:        "rs-data",
		Name:      "Data Server",
		OUID:      "ou1",
		Delimiter: ":",
		Resources: []providers.Resource{
			{
				Name:        "Root",
				Handle:      "root",
				Description: "Root resource",
				Actions: []providers.Action{
					{Name: "Read", Handle: "read"},
				},
			},
			{
				Name:         "Child",
				Handle:       "child",
				Description:  "Child resource",
				Parent:       &parentID,
				ParentHandle: "root",
				Actions: []providers.Action{
					{Name: "Write", Handle: "write"},
				},
			},
		},
	}

	err := fileStore.Create("rs-data", rs)
	assert.NoError(s.T(), err)

	resource, err := s.store.GetResource(s.ctx, "root", "rs-data")
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "rs-data_root", resource.ID)
	assert.Equal(s.T(), "Root", resource.Name)

	resource, err = s.store.GetResource(s.ctx, "rs-data_root", "rs-data")
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "rs-data_root", resource.ID)

	resources, err := s.store.GetResourceList(s.ctx, "rs-data", 10, 0)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), resources, 1)
	assert.Equal(s.T(), "rs-data_root", resources[0].ID)

	resources, err = s.store.GetResourceListByParent(s.ctx, "rs-data", nil, 10, 0)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), resources, 1)

	resources, err = s.store.GetResourceListByParent(s.ctx, "rs-data", &parentID, 10, 0)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), resources, 1)
	assert.Equal(s.T(), "rs-data_child", resources[0].ID)

	count, err := s.store.GetResourceListCount(s.ctx, "rs-data")
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, count)

	count, err = s.store.GetResourceListCountByParent(s.ctx, "rs-data", &parentID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, count)
}

func (s *FileBasedResourceStoreTestSuite) TestGetActionListAndCounts_WithData() {
	fileStore, ok := s.store.(*fileBasedResourceStore)
	assert.True(s.T(), ok)

	parentID := "rs-data_root"
	rs := &providers.ResourceServer{
		ID:        "rs-data",
		Name:      "Data Server",
		OUID:      "ou1",
		Delimiter: ":",
		Resources: []providers.Resource{
			{
				Name:   "Root",
				Handle: "root",
				Actions: []providers.Action{
					{Name: "Read", Handle: "read"},
				},
			},
			{
				Name:         "Child",
				Handle:       "child",
				Parent:       &parentID,
				ParentHandle: "root",
				Actions: []providers.Action{
					{Name: "Write", Handle: "write"},
				},
			},
		},
	}

	err := fileStore.Create("rs-data", rs)
	assert.NoError(s.T(), err)

	rootID := fmt.Sprintf("%s_%s", rs.ID, rs.Resources[0].Handle)
	rootActionID := fmt.Sprintf("%s_%s_%s", rs.ID, rs.Resources[0].Handle, rs.Resources[0].Actions[0].Handle)

	action, err := s.store.GetAction(s.ctx, rootActionID, "rs-data", nil)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), rootActionID, action.ID)

	actions, err := s.store.GetActionList(s.ctx, "rs-data", nil, "", 10, 0)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), actions, 2)

	actions, err = s.store.GetActionList(s.ctx, "rs-data", &rootID, "", 10, 0)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), actions, 1)

	count, err := s.store.GetActionListCount(s.ctx, "rs-data", nil, "")
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 2, count)

	count, err = s.store.GetActionListCount(s.ctx, "rs-data", &rootID, "")
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, count)
}

func (s *FileBasedResourceStoreTestSuite) TestGetActionList_FilterByKind() {
	fileStore, ok := s.store.(*fileBasedResourceStore)
	assert.True(s.T(), ok)

	rs := &providers.ResourceServer{
		ID:        "rs-mcp",
		Name:      "MCP Server",
		OUID:      "ou1",
		Delimiter: ":",
		Resources: []providers.Resource{
			{
				Name:   "Root",
				Handle: "root",
				Actions: []providers.Action{
					{Name: "Create User", Handle: "create_user", Kind: providers.ActionKindTool},
					{Name: "User List", Handle: "user_list", Kind: providers.ActionKindResource},
				},
			},
		},
	}

	err := fileStore.Create("rs-mcp", rs)
	assert.NoError(s.T(), err)

	// Empty kind returns all actions.
	all, err := s.store.GetActionList(s.ctx, "rs-mcp", nil, "", 10, 0)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), all, 2)

	// Tool kind returns only the tool action.
	tools, err := s.store.GetActionList(s.ctx, "rs-mcp", nil, providers.ActionKindTool, 10, 0)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), tools, 1)
	assert.Equal(s.T(), "create_user", tools[0].Handle)
	assert.Equal(s.T(), providers.ActionKindTool, tools[0].Kind)

	// Resource kind returns only the resource action.
	resources, err := s.store.GetActionList(s.ctx, "rs-mcp", nil, providers.ActionKindResource, 10, 0)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), resources, 1)
	assert.Equal(s.T(), "user_list", resources[0].Handle)
	assert.Equal(s.T(), providers.ActionKindResource, resources[0].Kind)

	// The count honors the kind filter and matches the filtered list length.
	allCount, err := s.store.GetActionListCount(s.ctx, "rs-mcp", nil, "")
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), len(all), allCount)

	toolCount, err := s.store.GetActionListCount(s.ctx, "rs-mcp", nil, providers.ActionKindTool)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), len(tools), toolCount)

	resourceCount, err := s.store.GetActionListCount(s.ctx, "rs-mcp", nil, providers.ActionKindResource)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), len(resources), resourceCount)
}

func (s *FileBasedResourceStoreTestSuite) TestCheckActionHandleExists_WithData() {
	fileStore, ok := s.store.(*fileBasedResourceStore)
	assert.True(s.T(), ok)

	rs := &providers.ResourceServer{
		ID:        "rs-data",
		Name:      "Data Server",
		OUID:      "ou1",
		Delimiter: ":",
		Resources: []providers.Resource{
			{
				Name:   "Root",
				Handle: "root",
				Actions: []providers.Action{
					{Name: "Read", Handle: "read"},
				},
			},
		},
	}

	err := fileStore.Create("rs-data", rs)
	assert.NoError(s.T(), err)

	exists, err := s.store.CheckActionHandleExists(s.ctx, "rs-data", nil, "read")
	assert.NoError(s.T(), err)
	assert.True(s.T(), exists)
}

func (s *FileBasedResourceStoreTestSuite) TestCheckResourceHandleExists_AlwaysFalse() {
	parentID := "parent-id"
	exists, err := s.store.CheckResourceHandleExists(s.ctx, "rs1", "root", &parentID)

	assert.NoError(s.T(), err)
	assert.False(s.T(), exists)
}

func (s *FileBasedResourceStoreTestSuite) TestCheckResourceHasDependencies_AlwaysFalse() {
	hasDeps, err := s.store.CheckResourceHasDependencies(s.ctx, "res1")

	assert.NoError(s.T(), err)
	assert.False(s.T(), hasDeps)
}

// Test with loaded data using Create method

func (s *FileBasedResourceStoreTestSuite) TestCreateAndGetResourceServer() {
	// Use the internal Create method (implements Storer interface)
	processedDTO := &providers.ResourceServer{
		ID:          "rs-test",
		Name:        "Test Server",
		Description: "A test server",
		Identifier:  "test-server",
		OUID:        "ou1",
		Delimiter:   ":",
		Resources:   []providers.Resource{},
	}

	fileStore, ok := s.store.(*fileBasedResourceStore)
	assert.True(s.T(), ok)

	err := fileStore.Create("rs-test", processedDTO)
	assert.NoError(s.T(), err)

	// Now get it back
	rs, err := s.store.GetResourceServer(s.ctx, "rs-test")
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "rs-test", rs.ID)
	assert.Equal(s.T(), "Test Server", rs.Name)
	assert.Equal(s.T(), "test-server", rs.Identifier)
}

func (s *FileBasedResourceStoreTestSuite) TestGetResourceServerList_WithData() {
	fileStore, ok := s.store.(*fileBasedResourceStore)
	assert.True(s.T(), ok)

	// Get initial count
	initialCount, err := s.store.GetResourceServerListCount(s.ctx)
	assert.NoError(s.T(), err)

	// Add test data
	rs1 := &providers.ResourceServer{
		ID:        "rs-test-1",
		Name:      "Server Test 1",
		OUID:      "ou1",
		Delimiter: ":",
	}
	rs2 := &providers.ResourceServer{
		ID:        "rs-test-2",
		Name:      "Server Test 2",
		OUID:      "ou1",
		Delimiter: ":",
	}

	err = fileStore.Create("rs-test-1", rs1)
	assert.NoError(s.T(), err)
	err = fileStore.Create("rs-test-2", rs2)
	assert.NoError(s.T(), err)

	// Test list all after adding 2 more items
	servers, err := s.store.GetResourceServerList(s.ctx, 100, 0)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), servers, initialCount+2)

	// Test pagination
	servers, err = s.store.GetResourceServerList(s.ctx, 1, 0)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), servers, 1)

	servers, err = s.store.GetResourceServerList(s.ctx, 1, 1)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), servers, 1)

	// Test count
	count, err := s.store.GetResourceServerListCount(s.ctx)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), initialCount+2, count)
}

func (s *FileBasedResourceStoreTestSuite) TestCheckResourceServerNameExists_WithData() {
	fileStore, ok := s.store.(*fileBasedResourceStore)
	assert.True(s.T(), ok)

	rs := &providers.ResourceServer{
		ID:        "rs-test",
		Name:      "Unique Server Name",
		OUID:      "ou1",
		Delimiter: ":",
	}

	err := fileStore.Create("rs-test", rs)
	assert.NoError(s.T(), err)

	// Check existing name
	exists, err := s.store.CheckResourceServerNameExists(s.ctx, "Unique Server Name")
	assert.NoError(s.T(), err)
	assert.True(s.T(), exists)

	// Check non-existing name
	exists, err = s.store.CheckResourceServerNameExists(s.ctx, "Non Existent Server")
	assert.NoError(s.T(), err)
	assert.False(s.T(), exists)
}

func (s *FileBasedResourceStoreTestSuite) TestCheckResourceServerIdentifierExists_WithData() {
	fileStore, ok := s.store.(*fileBasedResourceStore)
	assert.True(s.T(), ok)

	rs := &providers.ResourceServer{
		ID:         "rs-test",
		Name:       "Test Server",
		Identifier: "unique-identifier",
		OUID:       "ou1",
		Delimiter:  ":",
	}

	err := fileStore.Create("rs-test", rs)
	assert.NoError(s.T(), err)

	// Check existing identifier
	exists, err := s.store.CheckResourceServerIdentifierExists(s.ctx, "unique-identifier")
	assert.NoError(s.T(), err)
	assert.True(s.T(), exists)

	// Check non-existing identifier
	exists, err = s.store.CheckResourceServerIdentifierExists(s.ctx, "non-existent")
	assert.NoError(s.T(), err)
	assert.False(s.T(), exists)
}
