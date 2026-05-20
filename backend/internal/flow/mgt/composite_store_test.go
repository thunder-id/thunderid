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

package flowmgt

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
)

const (
	testFlowID     = "flow-001"
	testFlowHandle = "test-flow"
)

type CompositeStoreTestSuite struct {
	suite.Suite
	mockDBStore    *flowStoreInterfaceMock
	mockFileStore  *flowStoreInterfaceMock
	compositeStore *compositeFlowStore
}

func TestCompositeStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CompositeStoreTestSuite))
}

func (s *CompositeStoreTestSuite) SetupTest() {
	s.mockDBStore = newFlowStoreInterfaceMock(s.T())
	s.mockFileStore = newFlowStoreInterfaceMock(s.T())
	s.compositeStore = newCompositeFlowStore(s.mockFileStore, s.mockDBStore)
}

// Helper function to create a test flow
func (s *CompositeStoreTestSuite) createTestFlow(id, name string) *CompleteFlowDefinition {
	return &CompleteFlowDefinition{
		ID:            id,
		Handle:        testFlowHandle,
		Name:          name,
		FlowType:      common.FlowTypeAuthentication,
		ActiveVersion: 1,
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START"},
			{ID: "end", Type: "END"},
		},
	}
}

func (s *CompositeStoreTestSuite) createBasicTestFlow(id, handle, name string, readOnly bool) BasicFlowDefinition {
	return BasicFlowDefinition{
		ID:         id,
		Handle:     handle,
		Name:       name,
		FlowType:   common.FlowTypeAuthentication,
		IsReadOnly: readOnly,
	}
}

// CreateFlow tests
func (s *CompositeStoreTestSuite) TestCreateFlow_RoutedToDBOnly() {
	flowDef := &FlowDefinition{
		Handle:   testFlowHandle,
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
	}
	expected := s.createTestFlow("flow-id", "Test Flow")
	s.mockDBStore.EXPECT().CreateFlow(mock.Anything, "flow-id", flowDef).Return(expected, nil).Once()

	result, err := s.compositeStore.CreateFlow(context.Background(), "flow-id", flowDef)

	require.NoError(s.T(), err)
	assert.Equal(s.T(), expected.ID, result.ID)
	s.mockDBStore.AssertExpectations(s.T())
	s.mockFileStore.AssertNotCalled(s.T(), "CreateFlow")
}

// GetFlowByID tests
func (s *CompositeStoreTestSuite) TestGetFlowByID_FromDB() {
	flowID := testFlowID
	expected := s.createTestFlow(flowID, "Test Flow")
	s.mockDBStore.EXPECT().GetFlowByID(mock.Anything, flowID).Return(expected, nil).Once()

	result, err := s.compositeStore.GetFlowByID(context.Background(), flowID)

	require.NoError(s.T(), err)
	assert.Equal(s.T(), expected.ID, result.ID)
	// Verify that flows from DB are not marked as read-only
	assert.False(s.T(), result.IsReadOnly, "Flow from DB should not be marked as read-only")
}

func (s *CompositeStoreTestSuite) TestGetFlowByID_FallbackToFile() {
	flowID := testFlowID
	expected := s.createTestFlow(flowID, "Test Flow")
	s.mockDBStore.EXPECT().GetFlowByID(mock.Anything, flowID).Return(nil, errFlowNotFound).Once()
	s.mockFileStore.EXPECT().GetFlowByID(mock.Anything, flowID).Return(expected, nil).Once()

	result, err := s.compositeStore.GetFlowByID(context.Background(), flowID)

	require.NoError(s.T(), err)
	assert.Equal(s.T(), expected.ID, result.ID)
	// Verify that flows from file store are marked as read-only
	assert.True(s.T(), result.IsReadOnly, "Flow from file store should be marked as read-only")
}

func (s *CompositeStoreTestSuite) TestGetFlowByID_NotFound() {
	flowID := testFlowID
	s.mockDBStore.EXPECT().GetFlowByID(mock.Anything, flowID).Return(nil, errFlowNotFound).Once()
	s.mockFileStore.EXPECT().GetFlowByID(mock.Anything, flowID).Return(nil, errFlowNotFound).Once()

	result, err := s.compositeStore.GetFlowByID(context.Background(), flowID)

	assert.Error(s.T(), err)
	assert.Nil(s.T(), result)
	assert.Equal(s.T(), errFlowNotFound, err)
}

// GetFlowByHandle tests
func (s *CompositeStoreTestSuite) TestGetFlowByHandle_FromDB() {
	handle := testFlowHandle
	flowType := common.FlowTypeAuthentication
	expected := s.createTestFlow(testFlowID, "Test Flow")
	s.mockDBStore.EXPECT().GetFlowByHandle(mock.Anything, handle, flowType).Return(expected, nil).Once()

	result, err := s.compositeStore.GetFlowByHandle(context.Background(), handle, flowType)

	require.NoError(s.T(), err)
	assert.Equal(s.T(), handle, result.Handle)
	// Verify that flows from DB are not marked as read-only
	assert.False(s.T(), result.IsReadOnly, "Flow from DB should not be marked as read-only")
}

func (s *CompositeStoreTestSuite) TestGetFlowByHandle_FallbackToFile() {
	handle := testFlowHandle
	flowType := common.FlowTypeAuthentication
	expected := s.createTestFlow(testFlowID, "Test Flow")
	s.mockDBStore.EXPECT().GetFlowByHandle(mock.Anything, handle, flowType).Return(nil, errFlowNotFound).Once()
	s.mockFileStore.EXPECT().GetFlowByHandle(mock.Anything, handle, flowType).Return(expected, nil).Once()

	result, err := s.compositeStore.GetFlowByHandle(context.Background(), handle, flowType)

	require.NoError(s.T(), err)
	assert.Equal(s.T(), handle, result.Handle)
	// Verify that flows from file store are marked as read-only
	assert.True(s.T(), result.IsReadOnly, "Flow from file store should be marked as read-only")
}

// ListFlows tests - merge and deduplicate
func (s *CompositeStoreTestSuite) TestListFlows_MergeAndDeduplicate() {
	limit := 10
	offset := 0
	flowType := ""

	// DB flows
	dbFlows := []BasicFlowDefinition{
		s.createBasicTestFlow("flow-1", "auth-flow", "Auth Flow", false),
		s.createBasicTestFlow("flow-2", "signup-flow", "Signup Flow", false),
	}

	// File flows (one duplicate ID, one new)
	fileFlows := []BasicFlowDefinition{
		s.createBasicTestFlow("flow-1", "auth-flow", "Auth Flow Override", true),
		s.createBasicTestFlow("flow-3", "custom-flow", "Custom Flow", true),
	}

	// Composite store fetches all flows with unlimited sentinel (10000)
	s.mockDBStore.EXPECT().ListFlows(mock.Anything, 10000, 0, flowType).Return(dbFlows, 2, nil).Once()
	s.mockFileStore.EXPECT().ListFlows(mock.Anything, 10000, 0, flowType).Return(fileFlows, 2, nil).Once()

	result, total, err := s.compositeStore.ListFlows(context.Background(), limit, offset, flowType)

	require.NoError(s.T(), err)
	assert.Equal(s.T(), 3, len(result))
	// Total is the deduplicated count after merging (matches len(result))
	assert.Equal(s.T(), 3, total)

	// Find flow-1 - should be the DB version (first encountered)
	flow1 := findFlowByID(result, "flow-1")
	require.NotNil(s.T(), flow1)
	assert.False(s.T(), flow1.IsReadOnly)

	// Find flow-3 - should be from file store with IsReadOnly=true
	flow3 := findFlowByID(result, "flow-3")
	require.NotNil(s.T(), flow3)
	assert.True(s.T(), flow3.IsReadOnly)
}

func (s *CompositeStoreTestSuite) TestListFlows_MarkDBAsReadWrite() {
	dbFlows := []BasicFlowDefinition{
		s.createBasicTestFlow("flow-1", "auth-flow", "Auth Flow", false),
	}
	s.mockDBStore.EXPECT().ListFlows(mock.Anything, 10000, 0, "").Return(dbFlows, 1, nil).Once()
	s.mockFileStore.EXPECT().ListFlows(mock.Anything, 10000, 0, "").Return([]BasicFlowDefinition{}, 0, nil).Once()

	result, _, err := s.compositeStore.ListFlows(context.Background(), 10, 0, "")

	require.NoError(s.T(), err)
	require.Len(s.T(), result, 1)
	assert.False(s.T(), result[0].IsReadOnly)
}

func (s *CompositeStoreTestSuite) TestListFlows_MarkFileAsReadOnly() {
	fileFlows := []BasicFlowDefinition{
		s.createBasicTestFlow("flow-1", "auth-flow", "Auth Flow", false),
	}
	s.mockDBStore.EXPECT().ListFlows(mock.Anything, 10000, 0, "").Return([]BasicFlowDefinition{}, 0, nil).Once()
	s.mockFileStore.EXPECT().ListFlows(mock.Anything, 10000, 0, "").Return(fileFlows, 1, nil).Once()

	result, _, err := s.compositeStore.ListFlows(context.Background(), 10, 0, "")

	require.NoError(s.T(), err)
	require.Len(s.T(), result, 1)
	assert.True(s.T(), result[0].IsReadOnly)
}

// UpdateFlow tests - routed to DB only
func (s *CompositeStoreTestSuite) TestUpdateFlow_RoutedToDBOnly() {
	flowID := testFlowID
	flowDef := &FlowDefinition{
		Handle:   testFlowHandle,
		Name:     "Updated Flow",
		FlowType: common.FlowTypeAuthentication,
	}
	expected := s.createTestFlow(flowID, "Updated Flow")
	s.mockDBStore.EXPECT().UpdateFlow(mock.Anything, flowID, flowDef).Return(expected, nil).Once()

	result, err := s.compositeStore.UpdateFlow(context.Background(), flowID, flowDef)

	require.NoError(s.T(), err)
	assert.Equal(s.T(), expected.ID, result.ID)
	s.mockFileStore.AssertNotCalled(s.T(), "UpdateFlow")
}

// DeleteFlow tests - routed to DB only
func (s *CompositeStoreTestSuite) TestDeleteFlow_RoutedToDBOnly() {
	flowID := testFlowID
	s.mockDBStore.EXPECT().DeleteFlow(mock.Anything, flowID).Return(nil).Once()

	err := s.compositeStore.DeleteFlow(context.Background(), flowID)

	require.NoError(s.T(), err)
	s.mockFileStore.AssertNotCalled(s.T(), "DeleteFlow")
}

// Error handling tests
func (s *CompositeStoreTestSuite) TestListFlows_DBError() {
	dbError := errors.New("database error")
	s.mockDBStore.EXPECT().ListFlows(mock.Anything, 10000, 0, "").Return(nil, 0, dbError).Once()

	result, total, err := s.compositeStore.ListFlows(context.Background(), 10, 0, "")

	assert.Error(s.T(), err)
	assert.Equal(s.T(), dbError, err)
	assert.Nil(s.T(), result)
	assert.Equal(s.T(), 0, total)
}

func (s *CompositeStoreTestSuite) TestGetFlowByID_DBErrorAndFileError() {
	flowID := testFlowID
	s.mockDBStore.EXPECT().GetFlowByID(mock.Anything, flowID).Return(nil, errFlowNotFound).Once()
	fileError := errors.New("file read error")
	s.mockFileStore.EXPECT().GetFlowByID(mock.Anything, flowID).Return(nil, fileError).Once()

	result, err := s.compositeStore.GetFlowByID(context.Background(), flowID)

	assert.Error(s.T(), err)
	assert.Equal(s.T(), fileError, err)
	assert.Nil(s.T(), result)
}

// Helper function to find a flow by ID in a list
func findFlowByID(flows []BasicFlowDefinition, id string) *BasicFlowDefinition {
	for i := range flows {
		if flows[i].ID == id {
			return &flows[i]
		}
	}
	return nil
}

// Additional tests for composite store to achieve full coverage

// ListFlows error handling tests
func (s *CompositeStoreTestSuite) TestListFlows_FileStoreError() {
	dbFlows := []BasicFlowDefinition{
		s.createBasicTestFlow("flow-1", "auth-flow", "Auth Flow", false),
	}
	fileError := errors.New("file read error")

	s.mockDBStore.EXPECT().ListFlows(mock.Anything, 10000, 0, "").Return(dbFlows, 1, nil).Once()
	s.mockFileStore.EXPECT().ListFlows(mock.Anything, 10000, 0, "").Return(nil, 0, fileError).Once()

	result, total, err := s.compositeStore.ListFlows(context.Background(), 10, 0, "")

	// File store error should propagate
	s.Error(err)
	s.Equal(fileError, err)
	s.Nil(result)
	s.Equal(0, total)
}

// GetFlowByHandle error handling tests
func (s *CompositeStoreTestSuite) TestGetFlowByHandle_NotFound() {
	handle := "non-existent"
	flowType := common.FlowTypeAuthentication

	s.mockDBStore.EXPECT().GetFlowByHandle(mock.Anything, handle, flowType).Return(nil, errFlowNotFound).Once()
	s.mockFileStore.EXPECT().GetFlowByHandle(mock.Anything, handle, flowType).Return(nil, errFlowNotFound).Once()

	result, err := s.compositeStore.GetFlowByHandle(context.Background(), handle, flowType)

	s.Error(err)
	s.Equal(errFlowNotFound, err)
	s.Nil(result)
}

func (s *CompositeStoreTestSuite) TestGetFlowByHandle_DBAndFileError() {
	handle := testFlowHandle
	flowType := common.FlowTypeAuthentication
	fileError := errors.New("file system error")

	s.mockDBStore.EXPECT().GetFlowByHandle(mock.Anything, handle, flowType).Return(nil, errFlowNotFound).Once()
	s.mockFileStore.EXPECT().GetFlowByHandle(mock.Anything, handle, flowType).Return(nil, fileError).Once()

	result, err := s.compositeStore.GetFlowByHandle(context.Background(), handle, flowType)

	s.Error(err)
	s.Equal(fileError, err)
	s.Nil(result)
}

// IsFlowExistsByHandle error handling tests
func (s *CompositeStoreTestSuite) TestIsFlowExistsByHandle_FileStoreError() {
	handle := testFlowHandle
	flowType := common.FlowTypeAuthentication
	fileError := errors.New("file read error")

	s.mockFileStore.EXPECT().IsFlowExistsByHandle(mock.Anything, handle, flowType).Return(false, fileError).Once()

	exists, err := s.compositeStore.IsFlowExistsByHandle(context.Background(), handle, flowType)

	// Error from file store should propagate
	s.Error(err)
	s.Equal(fileError, err)
	s.False(exists)
}

func (s *CompositeStoreTestSuite) TestIsFlowExistsByHandle_DBStoreError() {
	handle := testFlowHandle
	flowType := common.FlowTypeAuthentication
	dbError := errors.New("database error")

	s.mockFileStore.EXPECT().IsFlowExistsByHandle(mock.Anything, handle, flowType).Return(false, nil).Once()
	s.mockDBStore.EXPECT().IsFlowExistsByHandle(mock.Anything, handle, flowType).Return(false, dbError).Once()

	exists, err := s.compositeStore.IsFlowExistsByHandle(context.Background(), handle, flowType)

	// Error from DB store should propagate
	s.Error(err)
	s.Equal(dbError, err)
	s.False(exists)
}

func (s *CompositeStoreTestSuite) TestIsFlowExistsByHandle_FileStoreOnly() {
	handle := testFlowHandle
	flowType := common.FlowTypeAuthentication

	s.mockFileStore.EXPECT().IsFlowExistsByHandle(mock.Anything, handle, flowType).Return(true, nil).Once()

	exists, err := s.compositeStore.IsFlowExistsByHandle(context.Background(), handle, flowType)

	// Should return true when found in file store
	s.NoError(err)
	s.True(exists)
}

func (s *CompositeStoreTestSuite) TestIsFlowExistsByHandle_Found() {
	handle := testFlowHandle
	flowType := common.FlowTypeAuthentication

	s.mockFileStore.EXPECT().IsFlowExistsByHandle(mock.Anything, handle, flowType).Return(false, nil).Once()
	s.mockDBStore.EXPECT().IsFlowExistsByHandle(mock.Anything, handle, flowType).Return(true, nil).Once()

	exists, err := s.compositeStore.IsFlowExistsByHandle(context.Background(), handle, flowType)

	// Should return true when found in DB store
	s.NoError(err)
	s.True(exists)
}

func (s *CompositeStoreTestSuite) TestIsFlowExistsByHandle_NotFoundBothStores() {
	handle := testFlowHandle
	flowType := common.FlowTypeAuthentication

	s.mockFileStore.EXPECT().IsFlowExistsByHandle(mock.Anything, handle, flowType).Return(false, nil).Once()
	s.mockDBStore.EXPECT().IsFlowExistsByHandle(mock.Anything, handle, flowType).Return(false, nil).Once()

	exists, err := s.compositeStore.IsFlowExistsByHandle(context.Background(), handle, flowType)

	// Should return false when not found in either store
	s.NoError(err)
	s.False(exists)
}

// Version management tests (all delegate to DB store)
func (s *CompositeStoreTestSuite) TestListFlowVersions_Success() {
	flowID := testFlowID
	versions := []BasicFlowVersion{
		{Version: 1, CreatedAt: "2025-01-01"},
		{Version: 2, CreatedAt: "2025-01-02"},
	}

	s.mockDBStore.EXPECT().ListFlowVersions(mock.Anything, flowID).Return(versions, nil).Once()

	result, err := s.compositeStore.ListFlowVersions(context.Background(), flowID)

	s.NoError(err)
	s.Equal(versions, result)
	s.mockFileStore.AssertNotCalled(s.T(), "ListFlowVersions")
}

func (s *CompositeStoreTestSuite) TestListFlowVersions_Error() {
	flowID := testFlowID
	dbError := errors.New("database error")

	s.mockDBStore.EXPECT().ListFlowVersions(mock.Anything, flowID).Return(nil, dbError).Once()

	result, err := s.compositeStore.ListFlowVersions(context.Background(), flowID)

	s.Error(err)
	s.Equal(dbError, err)
	s.Nil(result)
}

func (s *CompositeStoreTestSuite) TestGetFlowVersion_Success() {
	flowID := testFlowID
	version := 1
	flowVersion := &FlowVersion{
		Version:   version,
		CreatedAt: "2025-01-01",
	}

	s.mockDBStore.EXPECT().GetFlowVersion(mock.Anything, flowID, version).Return(flowVersion, nil).Once()

	result, err := s.compositeStore.GetFlowVersion(context.Background(), flowID, version)

	s.NoError(err)
	s.Equal(flowVersion, result)
	s.mockFileStore.AssertNotCalled(s.T(), "GetFlowVersion")
}

func (s *CompositeStoreTestSuite) TestGetFlowVersion_Error() {
	flowID := testFlowID
	version := 1
	dbError := errors.New("database error")

	s.mockDBStore.EXPECT().GetFlowVersion(mock.Anything, flowID, version).Return(nil, dbError).Once()

	result, err := s.compositeStore.GetFlowVersion(context.Background(), flowID, version)

	s.Error(err)
	s.Equal(dbError, err)
	s.Nil(result)
}

func (s *CompositeStoreTestSuite) TestRestoreFlowVersion_Success() {
	flowID := testFlowID
	version := 1
	restoredFlow := s.createTestFlow(flowID, "Test Flow")

	s.mockDBStore.EXPECT().RestoreFlowVersion(mock.Anything, flowID, version).Return(restoredFlow, nil).Once()

	result, err := s.compositeStore.RestoreFlowVersion(context.Background(), flowID, version)

	s.NoError(err)
	s.Equal(restoredFlow, result)
	s.mockFileStore.AssertNotCalled(s.T(), "RestoreFlowVersion")
}

func (s *CompositeStoreTestSuite) TestRestoreFlowVersion_Error() {
	flowID := testFlowID
	version := 1
	dbError := errors.New("database error")

	s.mockDBStore.EXPECT().RestoreFlowVersion(mock.Anything, flowID, version).Return(nil, dbError).Once()

	result, err := s.compositeStore.RestoreFlowVersion(context.Background(), flowID, version)

	s.Error(err)
	s.Equal(dbError, err)
	s.Nil(result)
}

// Write operation error tests
func (s *CompositeStoreTestSuite) TestCreateFlow_Error() {
	flowDef := &FlowDefinition{
		Handle:   testFlowHandle,
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
	}
	createError := errors.New("database error")

	s.mockDBStore.EXPECT().CreateFlow(mock.Anything, "flow-id", flowDef).Return(nil, createError).Once()

	result, err := s.compositeStore.CreateFlow(context.Background(), "flow-id", flowDef)

	s.Error(err)
	s.Equal(createError, err)
	s.Nil(result)
	s.mockFileStore.AssertNotCalled(s.T(), "CreateFlow")
}

func (s *CompositeStoreTestSuite) TestUpdateFlow_Error() {
	flowID := testFlowID
	flowDef := &FlowDefinition{
		Handle:   testFlowHandle,
		Name:     "Updated Flow",
		FlowType: common.FlowTypeAuthentication,
	}
	updateError := errors.New("database error")

	s.mockDBStore.EXPECT().UpdateFlow(mock.Anything, flowID, flowDef).Return(nil, updateError).Once()

	result, err := s.compositeStore.UpdateFlow(context.Background(), flowID, flowDef)

	s.Error(err)
	s.Equal(updateError, err)
	s.Nil(result)
	s.mockFileStore.AssertNotCalled(s.T(), "UpdateFlow")
}

func (s *CompositeStoreTestSuite) TestDeleteFlow_Error() {
	flowID := testFlowID
	deleteError := errors.New("database error")

	s.mockDBStore.EXPECT().DeleteFlow(mock.Anything, flowID).Return(deleteError).Once()

	err := s.compositeStore.DeleteFlow(context.Background(), flowID)

	s.Error(err)
	s.Equal(deleteError, err)
	s.mockFileStore.AssertNotCalled(s.T(), "DeleteFlow")
}

// Edge case: GetFlowByID with nil flow from file store
func (s *CompositeStoreTestSuite) TestGetFlowByID_FileStoreReturnsNilFlow() {
	flowID := testFlowID

	s.mockDBStore.EXPECT().GetFlowByID(mock.Anything, flowID).Return(nil, errFlowNotFound).Once()
	// File store returns nil flow with no error (edge case)
	s.mockFileStore.EXPECT().GetFlowByID(mock.Anything, flowID).Return(nil, nil).Once()

	result, err := s.compositeStore.GetFlowByID(context.Background(), flowID)

	// Should handle nil flow gracefully
	s.NoError(err)
	s.Nil(result)
}

// Edge case: GetFlowByHandle with nil flow from file store
func (s *CompositeStoreTestSuite) TestGetFlowByHandle_FileStoreReturnsNilFlow() {
	handle := testFlowHandle
	flowType := common.FlowTypeAuthentication

	s.mockDBStore.EXPECT().GetFlowByHandle(mock.Anything, handle, flowType).Return(nil, errFlowNotFound).Once()
	// File store returns nil flow with no error (edge case)
	s.mockFileStore.EXPECT().GetFlowByHandle(mock.Anything, handle, flowType).Return(nil, nil).Once()

	result, err := s.compositeStore.GetFlowByHandle(context.Background(), handle, flowType)

	// Should handle nil flow gracefully
	s.NoError(err)
	s.Nil(result)
}

// Test mergeAndDeduplicateFlows with empty slices
func (s *CompositeStoreTestSuite) TestMergeAndDeduplicateFlows_BothEmpty() {
	result := mergeAndDeduplicateFlows([]BasicFlowDefinition{}, []BasicFlowDefinition{})

	s.NotNil(result)
	s.Empty(result)
}

func (s *CompositeStoreTestSuite) TestMergeAndDeduplicateFlows_EmptyDB() {
	fileFlows := []BasicFlowDefinition{
		s.createBasicTestFlow("flow-1", "file-flow", "File Flow", false),
	}

	result := mergeAndDeduplicateFlows([]BasicFlowDefinition{}, fileFlows)

	s.Len(result, 1)
	s.True(result[0].IsReadOnly, "Flow from file should be marked as read-only")
}

func (s *CompositeStoreTestSuite) TestMergeAndDeduplicateFlows_EmptyFile() {
	dbFlows := []BasicFlowDefinition{
		s.createBasicTestFlow("flow-1", "db-flow", "DB Flow", true),
	}

	result := mergeAndDeduplicateFlows(dbFlows, []BasicFlowDefinition{})

	s.Len(result, 1)
	s.False(result[0].IsReadOnly, "Flow from DB should be marked as mutable")
}

// Test duplicate IDs in same store (defensive programming)
func (s *CompositeStoreTestSuite) TestMergeAndDeduplicateFlows_DuplicateInDB() {
	dbFlows := []BasicFlowDefinition{
		s.createBasicTestFlow("flow-1", "flow-a", "Flow A", false),
		s.createBasicTestFlow("flow-1", "flow-b", "Flow B", false), // Duplicate ID
	}

	result := mergeAndDeduplicateFlows(dbFlows, []BasicFlowDefinition{})

	// Should deduplicate - only first occurrence kept
	s.Len(result, 1)
	s.Equal("flow-a", result[0].Handle, "First occurrence should be kept")
}

func (s *CompositeStoreTestSuite) TestMergeAndDeduplicateFlows_DuplicateInFile() {
	fileFlows := []BasicFlowDefinition{
		s.createBasicTestFlow("flow-1", "flow-a", "Flow A", true),
		s.createBasicTestFlow("flow-1", "flow-b", "Flow B", true), // Duplicate ID
	}

	result := mergeAndDeduplicateFlows([]BasicFlowDefinition{}, fileFlows)

	// Should deduplicate - only first occurrence kept
	s.Len(result, 1)
	s.Equal("flow-a", result[0].Handle, "First occurrence should be kept")
	s.True(result[0].IsReadOnly)
}
