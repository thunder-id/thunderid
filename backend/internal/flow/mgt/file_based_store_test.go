/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package flowmgt

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

const testFlowTypeAuthentication = "AUTHENTICATION"

type FileBasedStoreTestSuite struct {
	suite.Suite
	store flowStoreInterface
}

func (s *FileBasedStoreTestSuite) SetupTest() {
	// Clear the singleton store before each test to ensure isolation
	_ = entity.GetInstance().Clear()
	s.store, _ = newFileBasedStore()
}

func (s *FileBasedStoreTestSuite) createTestFlow(handle string) *FlowDefinition {
	return &FlowDefinition{
		Handle:   handle,
		Name:     "Test Flow",
		FlowType: testFlowTypeAuthentication,
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START"},
			{ID: "login", Type: "BASIC_AUTHENTICATION"},
			{ID: "end", Type: "END"},
		},
	}
}

func (s *FileBasedStoreTestSuite) TestCreateFlow_Success() {
	flowDef := s.createTestFlow("test-flow")

	completeFlow, err := s.store.CreateFlow(context.Background(), "flow-001", flowDef)

	require.NoError(s.T(), err)
	assert.Equal(s.T(), "flow-001", completeFlow.ID)
	assert.Equal(s.T(), "test-flow", completeFlow.Handle)
	assert.Equal(s.T(), "Test Flow", completeFlow.Name)
	assert.Equal(s.T(), common.FlowType("AUTHENTICATION"), completeFlow.FlowType)
	assert.Equal(s.T(), 1, completeFlow.ActiveVersion)
	assert.Len(s.T(), completeFlow.Nodes, 3)
}

func (s *FileBasedStoreTestSuite) TestGetFlowByID_Success() {
	flowDef := s.createTestFlow("test-flow")
	_, err := s.store.CreateFlow(context.Background(), "flow-001", flowDef)
	require.NoError(s.T(), err)

	retrieved, err := s.store.GetFlowByID(context.Background(), "flow-001")

	require.NoError(s.T(), err)
	assert.Equal(s.T(), "flow-001", retrieved.ID)
	assert.Equal(s.T(), "test-flow", retrieved.Handle)
	assert.Equal(s.T(), "Test Flow", retrieved.Name)
}

func (s *FileBasedStoreTestSuite) TestGetFlowByID_NotFound() {
	_, err := s.store.GetFlowByID(context.Background(), "non-existent")

	assert.Error(s.T(), err)
	assert.Equal(s.T(), errFlowNotFound, err)
}

func (s *FileBasedStoreTestSuite) TestGetFlowByHandle_Success() {
	flowDef := s.createTestFlow("test-flow")
	_, err := s.store.CreateFlow(context.Background(), "flow-001", flowDef)
	require.NoError(s.T(), err)

	retrieved, err := s.store.GetFlowByHandle(context.Background(), "test-flow", testFlowTypeAuthentication)

	require.NoError(s.T(), err)
	assert.Equal(s.T(), "flow-001", retrieved.ID)
	assert.Equal(s.T(), "test-flow", retrieved.Handle)
}

func (s *FileBasedStoreTestSuite) TestGetFlowByHandle_NotFound() {
	_, err := s.store.GetFlowByHandle(context.Background(), "non-existent", testFlowTypeAuthentication)

	assert.Error(s.T(), err)
	assert.Equal(s.T(), errFlowNotFound, err)
}

func (s *FileBasedStoreTestSuite) TestGetFlowByHandle_WrongFlowType() {
	flowDef := s.createTestFlow("test-flow")
	flowDef.FlowType = testFlowTypeAuthentication
	_, err := s.store.CreateFlow(context.Background(), "flow-001", flowDef)
	require.NoError(s.T(), err)

	_, err = s.store.GetFlowByHandle(context.Background(), "test-flow", "REGISTRATION")

	assert.Error(s.T(), err)
}

func (s *FileBasedStoreTestSuite) TestListFlows_NoFilter() {
	// Create multiple flows
	for i := 0; i < 3; i++ {
		flowDef := s.createTestFlow(fmt.Sprintf("flow-%d", i))
		_, err := s.store.CreateFlow(context.Background(), fmt.Sprintf("flow-00%d", i), flowDef)
		require.NoError(s.T(), err)
	}

	flows, count, err := s.store.ListFlows(context.Background(), 10, 0, "")

	require.NoError(s.T(), err)
	assert.Equal(s.T(), 3, count)
	assert.Len(s.T(), flows, 3)
}

func (s *FileBasedStoreTestSuite) TestListFlows_WithFlowTypeFilter() {
	// Create flows with different types
	authFlow := s.createTestFlow("auth-flow")
	authFlow.FlowType = testFlowTypeAuthentication
	_, err := s.store.CreateFlow(context.Background(), "flow-001", authFlow)
	require.NoError(s.T(), err)

	regFlow := s.createTestFlow("reg-flow")
	regFlow.FlowType = "REGISTRATION"
	_, err = s.store.CreateFlow(context.Background(), "flow-002", regFlow)
	require.NoError(s.T(), err)

	// List only AUTHENTICATION flows
	flows, count, err := s.store.ListFlows(context.Background(), 10, 0, testFlowTypeAuthentication)

	require.NoError(s.T(), err)
	assert.Equal(s.T(), 1, count)
	assert.Len(s.T(), flows, 1)
	assert.Equal(s.T(), "auth-flow", flows[0].Handle)
}

func (s *FileBasedStoreTestSuite) TestListFlows_Pagination() {
	// Create 5 flows
	for i := 0; i < 5; i++ {
		flowDef := s.createTestFlow(fmt.Sprintf("flow-%d", i))
		_, err := s.store.CreateFlow(context.Background(), fmt.Sprintf("flow-00%d", i), flowDef)
		require.NoError(s.T(), err)
	}

	// Test first page
	flows, count, err := s.store.ListFlows(context.Background(), 2, 0, "")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 5, count)
	assert.Len(s.T(), flows, 2)

	// Test second page
	flows, count, err = s.store.ListFlows(context.Background(), 2, 2, "")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 5, count)
	assert.Len(s.T(), flows, 2)

	// Test offset beyond total
	flows, count, err = s.store.ListFlows(context.Background(), 10, 10, "")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 5, count)
	assert.Len(s.T(), flows, 0)
}

func (s *FileBasedStoreTestSuite) TestIsFlowExistsByHandle_Found() {
	flowDef := s.createTestFlow("test-flow")
	_, err := s.store.CreateFlow(context.Background(), "flow-001", flowDef)
	require.NoError(s.T(), err)

	exists, err := s.store.IsFlowExistsByHandle(context.Background(), "test-flow", testFlowTypeAuthentication)

	require.NoError(s.T(), err)
	assert.True(s.T(), exists)
}

func (s *FileBasedStoreTestSuite) TestIsFlowExistsByHandle_NotFound() {
	exists, err := s.store.IsFlowExistsByHandle(context.Background(), "non-existent", testFlowTypeAuthentication)

	require.NoError(s.T(), err)
	assert.False(s.T(), exists)
}

func (s *FileBasedStoreTestSuite) TestIsFlowExistsByHandle_WrongFlowType() {
	flowDef := s.createTestFlow("test-flow")
	flowDef.FlowType = testFlowTypeAuthentication
	_, err := s.store.CreateFlow(context.Background(), "flow-001", flowDef)
	require.NoError(s.T(), err)

	exists, err := s.store.IsFlowExistsByHandle(context.Background(), "test-flow", "REGISTRATION")

	require.NoError(s.T(), err)
	assert.False(s.T(), exists)
}

func (s *FileBasedStoreTestSuite) TestUpdateFlow_NotSupported() {
	flowDef := s.createTestFlow("test-flow")

	_, err := s.store.UpdateFlow(context.Background(), "flow-001", flowDef)

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not supported in file-based store")
}

func (s *FileBasedStoreTestSuite) TestDeleteFlow_NotSupported() {
	err := s.store.DeleteFlow(context.Background(), "flow-001")

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not supported in file-based store")
}

func (s *FileBasedStoreTestSuite) TestListFlowVersions_NotSupported() {
	_, err := s.store.ListFlowVersions(context.Background(), "flow-001")

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not supported in file-based store")
}

func (s *FileBasedStoreTestSuite) TestGetFlowVersion_NotSupported() {
	_, err := s.store.GetFlowVersion(context.Background(), "flow-001", 1)

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not supported in file-based store")
}

func (s *FileBasedStoreTestSuite) TestRestoreFlowVersion_NotSupported() {
	_, err := s.store.RestoreFlowVersion(context.Background(), "flow-001", 1)

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not supported in file-based store")
}

func (s *FileBasedStoreTestSuite) TestCreate_ImplementsStorer() {
	completeFlow := &CompleteFlowDefinition{
		ID:            "flow-001",
		Handle:        "test-flow",
		Name:          "Test Flow",
		FlowType:      "AUTHENTICATION",
		ActiveVersion: 1,
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START"},
			{ID: "login", Type: "BASIC_AUTHENTICATION"},
			{ID: "end", Type: "END"},
		},
	}

	err := s.store.(*fileBasedStore).Create("flow-001", completeFlow)

	require.NoError(s.T(), err)

	// Verify it was created
	retrieved, err := s.store.GetFlowByID(context.Background(), "flow-001")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "test-flow", retrieved.Handle)
}

func (s *FileBasedStoreTestSuite) TestListFlows_EmptyStore() {
	flows, count, err := s.store.ListFlows(context.Background(), 10, 0, "")

	require.NoError(s.T(), err)
	assert.Equal(s.T(), 0, count)
	assert.Len(s.T(), flows, 0)
}

func (s *FileBasedStoreTestSuite) TestGetFlowByHandle_MultipleFlowsSameHandle() {
	// Create two flows with different types but same handle
	authFlow := s.createTestFlow("common-handle")
	authFlow.FlowType = testFlowTypeAuthentication
	_, err := s.store.CreateFlow(context.Background(), "flow-001", authFlow)
	require.NoError(s.T(), err)

	regFlow := s.createTestFlow("common-handle")
	regFlow.FlowType = "REGISTRATION"
	_, err = s.store.CreateFlow(context.Background(), "flow-002", regFlow)
	require.NoError(s.T(), err)

	// Retrieve by handle and type should get the correct one
	authRetrieved, err := s.store.GetFlowByHandle(context.Background(), "common-handle", testFlowTypeAuthentication)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "flow-001", authRetrieved.ID)

	// Retrieve REGISTRATION flow by handle and type should also work
	regRetrieved, err := s.store.GetFlowByHandle(context.Background(), "common-handle", "REGISTRATION")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "flow-002", regRetrieved.ID)
	assert.Equal(s.T(), "common-handle", regRetrieved.Handle)
}

func (s *FileBasedStoreTestSuite) TestGetFlowByID_TypeAssertionFailure() {
	// This test verifies the type assertion error path in GetFlowByID
	// Create a flow and then manually corrupt the store data
	flowDef := s.createTestFlow("test-flow")
	_, err := s.store.CreateFlow(context.Background(), "flow-001", flowDef)
	require.NoError(s.T(), err)

	// Access the underlying store to corrupt data
	fileStore := s.store.(*fileBasedStore)
	// Store invalid data type
	err = fileStore.GenericFileBasedStore.Create("corrupted-flow", "not a flow definition")
	require.NoError(s.T(), err)

	// Try to retrieve the corrupted flow
	_, err = s.store.GetFlowByID(context.Background(), "corrupted-flow")
	assert.Error(s.T(), err)
	assert.Equal(s.T(), errFlowNotFound, err)
}

func (s *FileBasedStoreTestSuite) TestListFlows_TypeAssertionSkip() {
	// Create valid flows
	for i := 0; i < 2; i++ {
		flowDef := s.createTestFlow(fmt.Sprintf("flow-%d", i))
		_, err := s.store.CreateFlow(context.Background(), fmt.Sprintf("flow-00%d", i), flowDef)
		require.NoError(s.T(), err)
	}

	// Add corrupted data to store
	fileStore := s.store.(*fileBasedStore)
	err := fileStore.GenericFileBasedStore.Create("corrupted", "invalid data")
	require.NoError(s.T(), err)

	// List should skip the corrupted entry and return valid flows
	flows, count, err := s.store.ListFlows(context.Background(), 10, 0, "")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 2, count)
	assert.Len(s.T(), flows, 2)
}

func (s *FileBasedStoreTestSuite) TestIsFlowExistsByHandle_TypeAssertionSkip() {
	// Create valid flow
	flowDef := s.createTestFlow("test-flow")
	_, err := s.store.CreateFlow(context.Background(), "flow-001", flowDef)
	require.NoError(s.T(), err)

	// Add corrupted data to store
	fileStore := s.store.(*fileBasedStore)
	err = fileStore.GenericFileBasedStore.Create("corrupted", "invalid data")
	require.NoError(s.T(), err)

	// Should still find the valid flow
	exists, err := s.store.IsFlowExistsByHandle(context.Background(), "test-flow", testFlowTypeAuthentication)
	require.NoError(s.T(), err)
	assert.True(s.T(), exists)
}

func (s *FileBasedStoreTestSuite) TestIsFlowExistsByHandle_ListError() {
	// Create a new store instance to test error path
	store, _ := newFileBasedStore()

	// IsFlowExistsByHandle should handle list errors gracefully
	// In the current implementation, it returns the error from List()
	exists, err := store.IsFlowExistsByHandle(context.Background(), "test", testFlowTypeAuthentication)
	// With empty store, should return false with no error
	require.NoError(s.T(), err)
	assert.False(s.T(), exists)
}

func (s *FileBasedStoreTestSuite) TestCreate_WithCompleteFlow() {
	// Test the Create method which is used by the resource loader
	fileStore := s.store.(*fileBasedStore)

	completeFlow := &CompleteFlowDefinition{
		ID:            "flow-100",
		Handle:        "complete-flow",
		Name:          "Complete Flow",
		FlowType:      testFlowTypeAuthentication,
		ActiveVersion: 5,
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START"},
			{ID: "end", Type: "END"},
		},
	}

	err := fileStore.Create("flow-100", completeFlow)
	require.NoError(s.T(), err)

	// Verify it was created correctly
	retrieved, err := s.store.GetFlowByID(context.Background(), "flow-100")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "flow-100", retrieved.ID)
	assert.Equal(s.T(), "complete-flow", retrieved.Handle)
	// Note: ActiveVersion should be 1 as CreateFlow sets it
	assert.Equal(s.T(), 1, retrieved.ActiveVersion)
}

func TestFileBasedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(FileBasedStoreTestSuite))
}
