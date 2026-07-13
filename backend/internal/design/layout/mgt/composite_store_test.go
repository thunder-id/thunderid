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

package layoutmgt

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"
)

// CompositeLayoutStoreTestSuite contains tests for the composite layout store.
type CompositeLayoutStoreTestSuite struct {
	suite.Suite
	mockDBStore   *layoutMgtStoreInterfaceMock
	mockFileStore *layoutMgtStoreInterfaceMock
	store         layoutMgtStoreInterface
}

func TestCompositeLayoutStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CompositeLayoutStoreTestSuite))
}

func (suite *CompositeLayoutStoreTestSuite) SetupTest() {
	suite.mockDBStore = newLayoutMgtStoreInterfaceMock(suite.T())
	suite.mockFileStore = newLayoutMgtStoreInterfaceMock(suite.T())
	suite.store = newCompositeLayoutStore(suite.mockFileStore, suite.mockDBStore)
}

// Test GetLayoutListCount - Adds counts from both stores
func (suite *CompositeLayoutStoreTestSuite) TestGetLayoutListCount_AddsCounts() {
	suite.mockDBStore.On("GetLayoutListCount").Return(2, nil)
	suite.mockFileStore.On("GetLayoutListCount").Return(3, nil)

	count, err := suite.store.GetLayoutListCount()

	suite.NoError(err)
	suite.Equal(5, count) // 2 + 3 = 5 (no deduplication in count)
}

// Test GetLayoutListCount - DB store error
func (suite *CompositeLayoutStoreTestSuite) TestGetLayoutListCount_DBStoreError() {
	testErr := errors.New("db error")
	suite.mockDBStore.On("GetLayoutListCount").Return(0, testErr)

	_, err := suite.store.GetLayoutListCount()

	suite.Error(err)
	suite.Equal(testErr, err)
}

// Test GetLayoutListCount - File store count error
func (suite *CompositeLayoutStoreTestSuite) TestGetLayoutListCount_FileStoreCountError() {
	testErr := errors.New("file store error")
	suite.mockDBStore.On("GetLayoutListCount").Return(2, nil)
	suite.mockFileStore.On("GetLayoutListCount").Return(0, testErr)

	_, err := suite.store.GetLayoutListCount()

	suite.Error(err)
	suite.Equal(testErr, err)
}

// Test GetLayoutListCount - Empty stores
func (suite *CompositeLayoutStoreTestSuite) TestGetLayoutListCount_EmptyStores() {
	suite.mockDBStore.On("GetLayoutListCount").Return(0, nil)
	suite.mockFileStore.On("GetLayoutListCount").Return(0, nil)

	count, err := suite.store.GetLayoutListCount()

	suite.NoError(err)
	suite.Equal(0, count)
}

// Test GetLayoutList - Pagination with merged results
func (suite *CompositeLayoutStoreTestSuite) TestGetLayoutList_Pagination() {
	dbLayouts := []Layout{{ID: "layout1"}, {ID: "layout2"}}
	fileLayouts := []Layout{{ID: "layout2"}, {ID: "layout3"}}

	suite.mockDBStore.On("GetLayoutListCount").Return(2, nil)
	suite.mockFileStore.On("GetLayoutListCount").Return(2, nil)
	suite.mockDBStore.On("GetLayoutList", 2, 0).Return(dbLayouts, nil)
	suite.mockFileStore.On("GetLayoutList", 2, 0).Return(fileLayouts, nil)

	layouts, err := suite.store.GetLayoutList(2, 1)

	suite.NoError(err)
	suite.Len(layouts, 2) // Should get 2 layouts from offset 1
}

// Test GetLayoutList - Returns limit exceeded error
func (suite *CompositeLayoutStoreTestSuite) TestGetLayoutList_LimitExceeded() {
	// Create more layouts than the max composite store limit (1000)
	suite.mockDBStore.On("GetLayoutListCount").Return(1001, nil)
	suite.mockFileStore.On("GetLayoutListCount").Return(0, nil)

	_, err := suite.store.GetLayoutList(100, 0)

	suite.Error(err)
	suite.Equal(errResultLimitExceededInCompositeMode, err)
}

// Test GetLayoutList - DB store error
func (suite *CompositeLayoutStoreTestSuite) TestGetLayoutList_DBStoreError() {
	testErr := errors.New("db error")
	suite.mockDBStore.On("GetLayoutListCount").Return(0, testErr)

	_, err := suite.store.GetLayoutList(10, 0)

	suite.Error(err)
	suite.Equal(testErr, err)
}

// Test GetLayoutList - File store count error
func (suite *CompositeLayoutStoreTestSuite) TestGetLayoutList_FileStoreCountError() {
	testErr := errors.New("file store error")
	suite.mockDBStore.On("GetLayoutListCount").Return(2, nil)
	suite.mockFileStore.On("GetLayoutListCount").Return(0, testErr)

	_, err := suite.store.GetLayoutList(10, 0)

	suite.Error(err)
	suite.Equal(testErr, err)
}

// Test GetLayoutList - DB layouts list error
func (suite *CompositeLayoutStoreTestSuite) TestGetLayoutList_DBLayoutsListError() {
	testErr := errors.New("db list error")
	suite.mockDBStore.On("GetLayoutListCount").Return(2, nil)
	suite.mockFileStore.On("GetLayoutListCount").Return(2, nil)
	suite.mockDBStore.On("GetLayoutList", 2, 0).Return(nil, testErr)

	_, err := suite.store.GetLayoutList(10, 0)

	suite.Error(err)
	suite.Equal(testErr, err)
}

// Test GetLayoutList - File layouts list error
func (suite *CompositeLayoutStoreTestSuite) TestGetLayoutList_FileLayoutsListError() {
	testErr := errors.New("file list error")
	dbLayouts := []Layout{{ID: "layout1"}}
	suite.mockDBStore.On("GetLayoutListCount").Return(1, nil)
	suite.mockFileStore.On("GetLayoutListCount").Return(2, nil)
	suite.mockDBStore.On("GetLayoutList", 1, 0).Return(dbLayouts, nil)
	suite.mockFileStore.On("GetLayoutList", 2, 0).Return(nil, testErr)

	_, err := suite.store.GetLayoutList(10, 0)

	suite.Error(err)
	suite.Equal(testErr, err)
}

// Test CreateLayout - Delegates to DB store
func (suite *CompositeLayoutStoreTestSuite) TestCreateLayout_Success() {
	createReq := CreateLayoutRequest{
		DisplayName: "Test Layout",
		Description: "Test Description",
		Layout:      json.RawMessage(`{"components": []}`),
	}
	suite.mockDBStore.On("CreateLayout", "layout1", createReq).Return(nil)

	err := suite.store.CreateLayout("layout1", createReq)

	suite.NoError(err)
	suite.mockDBStore.AssertExpectations(suite.T())
}

// Test CreateLayout - DB store error
func (suite *CompositeLayoutStoreTestSuite) TestCreateLayout_DBStoreError() {
	testErr := errors.New("db error")
	createReq := CreateLayoutRequest{
		DisplayName: "Test Layout",
	}
	suite.mockDBStore.On("CreateLayout", "layout1", createReq).Return(testErr)

	err := suite.store.CreateLayout("layout1", createReq)

	suite.Error(err)
	suite.Equal(testErr, err)
}

// Test GetLayout - From DB store (DB takes precedence)
func (suite *CompositeLayoutStoreTestSuite) TestGetLayout_FromDBStore() {
	expectedLayout := Layout{ID: "layout1", DisplayName: "DB Layout"}
	suite.mockDBStore.On("GetLayout", "layout1").Return(expectedLayout, nil)

	layout, err := suite.store.GetLayout("layout1")

	suite.NoError(err)
	suite.Equal(expectedLayout.ID, layout.ID)
	suite.Equal(expectedLayout.DisplayName, layout.DisplayName)
	suite.False(layout.IsReadOnly)
}

// Test GetLayout - From file store (fallback when not in DB store)
func (suite *CompositeLayoutStoreTestSuite) TestGetLayout_FromFileStore() {
	expectedLayout := Layout{ID: "layout1", DisplayName: "File Layout"}
	suite.mockDBStore.On("GetLayout", "layout1").Return(Layout{}, errLayoutNotFound)
	suite.mockFileStore.On("GetLayout", "layout1").Return(expectedLayout, nil)

	layout, err := suite.store.GetLayout("layout1")

	suite.NoError(err)
	suite.Equal(expectedLayout.ID, layout.ID)
	suite.Equal(expectedLayout.DisplayName, layout.DisplayName)
	suite.True(layout.IsReadOnly)
}

// Test GetLayout - Not found in either store
func (suite *CompositeLayoutStoreTestSuite) TestGetLayout_NotFound() {
	suite.mockFileStore.On("GetLayout", "layout1").Return(Layout{}, errLayoutNotFound)
	suite.mockDBStore.On("GetLayout", "layout1").Return(Layout{}, errLayoutNotFound)

	_, err := suite.store.GetLayout("layout1")

	suite.Error(err)
	suite.Equal(errLayoutNotFound, err)
}

// Test IsLayoutExist - Exists in DB store
func (suite *CompositeLayoutStoreTestSuite) TestIsLayoutExist_InDBStore() {
	suite.mockDBStore.On("IsLayoutExist", "layout1").Return(true, nil)

	exists, err := suite.store.IsLayoutExist("layout1")

	suite.NoError(err)
	suite.True(exists)
}

// Test IsLayoutExist - Exists in file store
func (suite *CompositeLayoutStoreTestSuite) TestIsLayoutExist_InFileStore() {
	suite.mockDBStore.On("IsLayoutExist", "layout1").Return(false, nil)
	suite.mockFileStore.On("IsLayoutExist", "layout1").Return(true, nil)

	exists, err := suite.store.IsLayoutExist("layout1")

	suite.NoError(err)
	suite.True(exists)
}

// Test IsLayoutExist - Not found
func (suite *CompositeLayoutStoreTestSuite) TestIsLayoutExist_NotFound() {
	suite.mockDBStore.On("IsLayoutExist", "layout1").Return(false, nil)
	suite.mockFileStore.On("IsLayoutExist", "layout1").Return(false, nil)

	exists, err := suite.store.IsLayoutExist("layout1")

	suite.NoError(err)
	suite.False(exists)
}

// Test IsLayoutExist - DB store error
func (suite *CompositeLayoutStoreTestSuite) TestIsLayoutExist_DBStoreError() {
	testErr := errors.New("db error")
	suite.mockDBStore.On("IsLayoutExist", "layout1").Return(false, testErr)

	_, err := suite.store.IsLayoutExist("layout1")

	suite.Error(err)
	suite.Equal(testErr, err)
}

// Test UpdateLayout - Success (DB layout)
func (suite *CompositeLayoutStoreTestSuite) TestUpdateLayout_Success() {
	updateReq := UpdateLayoutRequest{
		DisplayName: "Updated Layout",
	}
	suite.mockFileStore.On("IsLayoutExist", "layout1").Return(false, nil)
	suite.mockDBStore.On("UpdateLayout", "layout1", updateReq).Return(nil)

	err := suite.store.UpdateLayout("layout1", updateReq)

	suite.NoError(err)
}

// Test UpdateLayout - Rejects declarative layout
func (suite *CompositeLayoutStoreTestSuite) TestUpdateLayout_RejectsDeclarative() {
	updateReq := UpdateLayoutRequest{
		DisplayName: "Updated Layout",
	}
	suite.mockFileStore.On("IsLayoutExist", "layout1").Return(true, nil)

	err := suite.store.UpdateLayout("layout1", updateReq)

	suite.Error(err)
	suite.Equal(errCannotUpdateDeclarativeLayout, err)
}

// Test UpdateLayout - File store check error
func (suite *CompositeLayoutStoreTestSuite) TestUpdateLayout_FileStoreCheckError() {
	testErr := errors.New("file store error")
	updateReq := UpdateLayoutRequest{
		DisplayName: "Updated Layout",
	}
	suite.mockFileStore.On("IsLayoutExist", "layout1").Return(false, testErr)

	err := suite.store.UpdateLayout("layout1", updateReq)

	suite.Error(err)
	suite.Equal(testErr, err)
}

// Test DeleteLayout - Success (DB layout)
func (suite *CompositeLayoutStoreTestSuite) TestDeleteLayout_Success() {
	suite.mockFileStore.On("IsLayoutExist", "layout1").Return(false, nil)
	suite.mockDBStore.On("DeleteLayout", "layout1").Return(nil)

	err := suite.store.DeleteLayout("layout1")

	suite.NoError(err)
}

// Test DeleteLayout - Rejects declarative layout
func (suite *CompositeLayoutStoreTestSuite) TestDeleteLayout_RejectsDeclarative() {
	suite.mockFileStore.On("IsLayoutExist", "layout1").Return(true, nil)

	err := suite.store.DeleteLayout("layout1")

	suite.Error(err)
	suite.Equal(errCannotDeleteDeclarativeLayout, err)
}

// Test DeleteLayout - File store check error
func (suite *CompositeLayoutStoreTestSuite) TestDeleteLayout_FileStoreCheckError() {
	testErr := errors.New("file store error")
	suite.mockFileStore.On("IsLayoutExist", "layout1").Return(false, testErr)

	err := suite.store.DeleteLayout("layout1")

	suite.Error(err)
	suite.Equal(testErr, err)
}

// Test IsLayoutDeclarative - True for file-based layout
func (suite *CompositeLayoutStoreTestSuite) TestIsLayoutDeclarative_True() {
	suite.mockFileStore.On("IsLayoutExist", "layout1").Return(true, nil)

	isDeclarative := suite.store.IsLayoutDeclarative("layout1")

	suite.True(isDeclarative)
}

// Test IsLayoutDeclarative - False for DB layout
func (suite *CompositeLayoutStoreTestSuite) TestIsLayoutDeclarative_False() {
	suite.mockFileStore.On("IsLayoutExist", "layout1").Return(false, nil)

	isDeclarative := suite.store.IsLayoutDeclarative("layout1")

	suite.False(isDeclarative)
}

// Test IsLayoutDeclarative - False on error
func (suite *CompositeLayoutStoreTestSuite) TestIsLayoutDeclarative_Error() {
	suite.mockFileStore.On("IsLayoutExist", "layout1").Return(false, errors.New("error"))

	isDeclarative := suite.store.IsLayoutDeclarative("layout1")

	suite.False(isDeclarative)
}

// Test mergeAndDeduplicateLayouts - File layouts take precedence
func (suite *CompositeLayoutStoreTestSuite) TestMergeAndDeduplicateLayouts() {
	dbLayouts := []Layout{
		{ID: "layout1", DisplayName: "DB Layout 1"},
		{ID: "layout2", DisplayName: "DB Layout 2"},
	}
	fileLayouts := []Layout{
		{ID: "layout2", DisplayName: "File Layout 2"}, // Duplicate ID
		{ID: "layout3", DisplayName: "File Layout 3"},
	}

	merged := mergeAndDeduplicateLayouts(dbLayouts, fileLayouts)

	suite.Len(merged, 3)
	// File layout2 should take precedence
	suite.Equal("File Layout 2", merged[0].DisplayName)
	suite.Equal("File Layout 3", merged[1].DisplayName)
	suite.Equal("DB Layout 1", merged[2].DisplayName)
}

// Test mergeAndDeduplicateLayouts - Empty stores
func (suite *CompositeLayoutStoreTestSuite) TestMergeAndDeduplicateLayouts_EmptyStores() {
	merged := mergeAndDeduplicateLayouts([]Layout{}, []Layout{})

	suite.Len(merged, 0)
}

// Test IsLayoutHandleConflict - Conflict in file store (returns early)
func (suite *CompositeLayoutStoreTestSuite) TestIsLayoutHandleConflict_ConflictInFileStore() {
	suite.mockFileStore.On("IsLayoutHandleConflict", "classic", "").Return(true, nil)

	conflict, err := suite.store.IsLayoutHandleConflict("classic", "")

	suite.NoError(err)
	suite.True(conflict)
}

// Test IsLayoutHandleConflict - No conflict in file, conflict in DB store
func (suite *CompositeLayoutStoreTestSuite) TestIsLayoutHandleConflict_ConflictInDBStore() {
	suite.mockFileStore.On("IsLayoutHandleConflict", "classic", "layout1").Return(false, nil)
	suite.mockDBStore.On("IsLayoutHandleConflict", "classic", "layout1").Return(true, nil)

	conflict, err := suite.store.IsLayoutHandleConflict("classic", "layout1")

	suite.NoError(err)
	suite.True(conflict)
}

// Test IsLayoutHandleConflict - No conflict in either store
func (suite *CompositeLayoutStoreTestSuite) TestIsLayoutHandleConflict_NoConflict() {
	suite.mockFileStore.On("IsLayoutHandleConflict", "unique-handle", "layout1").Return(false, nil)
	suite.mockDBStore.On("IsLayoutHandleConflict", "unique-handle", "layout1").Return(false, nil)

	conflict, err := suite.store.IsLayoutHandleConflict("unique-handle", "layout1")

	suite.NoError(err)
	suite.False(conflict)
}

// Test IsLayoutHandleConflict - File store error
func (suite *CompositeLayoutStoreTestSuite) TestIsLayoutHandleConflict_FileStoreError() {
	testErr := errors.New("file store error")
	suite.mockFileStore.On("IsLayoutHandleConflict", "classic", "").Return(false, testErr)

	conflict, err := suite.store.IsLayoutHandleConflict("classic", "")

	suite.Error(err)
	suite.Equal(testErr, err)
	suite.False(conflict)
}

// Test IsLayoutHandleConflict - DB store error
func (suite *CompositeLayoutStoreTestSuite) TestIsLayoutHandleConflict_DBStoreError() {
	testErr := errors.New("db store error")
	suite.mockFileStore.On("IsLayoutHandleConflict", "classic", "layout1").Return(false, nil)
	suite.mockDBStore.On("IsLayoutHandleConflict", "classic", "layout1").Return(false, testErr)

	conflict, err := suite.store.IsLayoutHandleConflict("classic", "layout1")

	suite.Error(err)
	suite.Equal(testErr, err)
	suite.False(conflict)
}
