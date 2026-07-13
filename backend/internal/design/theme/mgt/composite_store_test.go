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

package thememgt

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"
)

// CompositeThemeStoreTestSuite contains tests for the composite theme store.
type CompositeThemeStoreTestSuite struct {
	suite.Suite
	mockDBStore   *themeMgtStoreInterfaceMock
	mockFileStore *themeMgtStoreInterfaceMock
	store         themeMgtStoreInterface
}

func TestCompositeThemeStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CompositeThemeStoreTestSuite))
}

func (suite *CompositeThemeStoreTestSuite) SetupTest() {
	suite.mockDBStore = newThemeMgtStoreInterfaceMock(suite.T())
	suite.mockFileStore = newThemeMgtStoreInterfaceMock(suite.T())
	suite.store = newCompositeThemeStore(suite.mockFileStore, suite.mockDBStore)
}

// Test GetThemeListCount - Adds counts from both stores
func (suite *CompositeThemeStoreTestSuite) TestGetThemeListCount_AddsCounts() {
	suite.mockDBStore.On("GetThemeListCount").Return(2, nil)
	suite.mockFileStore.On("GetThemeListCount").Return(3, nil)

	count, err := suite.store.GetThemeListCount()

	suite.NoError(err)
	suite.Equal(5, count) // 2 + 3 = 5 (no deduplication in count)
}

// Test GetThemeListCount - DB store error
func (suite *CompositeThemeStoreTestSuite) TestGetThemeListCount_DBStoreError() {
	testErr := errors.New("db error")
	suite.mockDBStore.On("GetThemeListCount").Return(0, testErr)

	_, err := suite.store.GetThemeListCount()

	suite.Error(err)
	suite.Equal(testErr, err)
}

// Test GetThemeListCount - File store count error
func (suite *CompositeThemeStoreTestSuite) TestGetThemeListCount_FileStoreCountError() {
	testErr := errors.New("file store error")
	suite.mockDBStore.On("GetThemeListCount").Return(2, nil)
	suite.mockFileStore.On("GetThemeListCount").Return(0, testErr)

	_, err := suite.store.GetThemeListCount()

	suite.Error(err)
	suite.Equal(testErr, err)
}

// Test GetThemeListCount - Empty stores
func (suite *CompositeThemeStoreTestSuite) TestGetThemeListCount_EmptyStores() {
	suite.mockDBStore.On("GetThemeListCount").Return(0, nil)
	suite.mockFileStore.On("GetThemeListCount").Return(0, nil)

	count, err := suite.store.GetThemeListCount()

	suite.NoError(err)
	suite.Equal(0, count)
}

// Test GetThemeList - Pagination with merged results
func (suite *CompositeThemeStoreTestSuite) TestGetThemeList_Pagination() {
	dbThemes := []Theme{{ID: "theme1"}, {ID: "theme2"}}
	fileThemes := []Theme{{ID: "theme2"}, {ID: "theme3"}}

	suite.mockDBStore.On("GetThemeListCount").Return(2, nil)
	suite.mockFileStore.On("GetThemeListCount").Return(2, nil)
	suite.mockDBStore.On("GetThemeList", 2, 0).Return(dbThemes, nil)
	suite.mockFileStore.On("GetThemeList", 2, 0).Return(fileThemes, nil)

	themes, err := suite.store.GetThemeList(2, 1)

	suite.NoError(err)
	suite.Len(themes, 2) // Should get 2 themes from offset 1
}

// Test GetThemeList - Returns limit exceeded error
func (suite *CompositeThemeStoreTestSuite) TestGetThemeList_LimitExceeded() {
	// Create more themes than the max composite store limit (1000)
	suite.mockDBStore.On("GetThemeListCount").Return(1001, nil)
	suite.mockFileStore.On("GetThemeListCount").Return(0, nil)

	_, err := suite.store.GetThemeList(100, 0)

	suite.Error(err)
	suite.Equal(errResultLimitExceededInCompositeMode, err)
}

// Test GetThemeList - DB store error
func (suite *CompositeThemeStoreTestSuite) TestGetThemeList_DBStoreError() {
	testErr := errors.New("db error")
	suite.mockDBStore.On("GetThemeListCount").Return(0, testErr)

	_, err := suite.store.GetThemeList(10, 0)

	suite.Error(err)
	suite.Equal(testErr, err)
}

// Test GetThemeList - File store count error
func (suite *CompositeThemeStoreTestSuite) TestGetThemeList_FileStoreCountError() {
	testErr := errors.New("file store error")
	suite.mockDBStore.On("GetThemeListCount").Return(2, nil)
	suite.mockFileStore.On("GetThemeListCount").Return(0, testErr)

	_, err := suite.store.GetThemeList(10, 0)

	suite.Error(err)
	suite.Equal(testErr, err)
}

// Test GetThemeList - DB themes list error
func (suite *CompositeThemeStoreTestSuite) TestGetThemeList_DBThemesListError() {
	testErr := errors.New("db list error")
	suite.mockDBStore.On("GetThemeListCount").Return(2, nil)
	suite.mockFileStore.On("GetThemeListCount").Return(2, nil)
	suite.mockDBStore.On("GetThemeList", 2, 0).Return(nil, testErr)

	_, err := suite.store.GetThemeList(10, 0)

	suite.Error(err)
	suite.Equal(testErr, err)
}

// Test GetThemeList - File themes list error
func (suite *CompositeThemeStoreTestSuite) TestGetThemeList_FileThemesListError() {
	testErr := errors.New("file list error")
	dbThemes := []Theme{{ID: "theme1"}}
	suite.mockDBStore.On("GetThemeListCount").Return(1, nil)
	suite.mockFileStore.On("GetThemeListCount").Return(2, nil)
	suite.mockDBStore.On("GetThemeList", 1, 0).Return(dbThemes, nil)
	suite.mockFileStore.On("GetThemeList", 2, 0).Return(nil, testErr)

	_, err := suite.store.GetThemeList(10, 0)

	suite.Error(err)
	suite.Equal(testErr, err)
}

// Test CreateTheme - Delegates to DB store
func (suite *CompositeThemeStoreTestSuite) TestCreateTheme_Success() {
	createReq := CreateThemeRequest{
		DisplayName: "Test Theme",
		Description: "Test Description",
		Theme:       json.RawMessage(`{"colors": {}}`),
	}
	suite.mockDBStore.On("CreateTheme", "theme1", createReq).Return(nil)

	err := suite.store.CreateTheme("theme1", createReq)

	suite.NoError(err)
	suite.mockDBStore.AssertExpectations(suite.T())
}

// Test CreateTheme - DB store error
func (suite *CompositeThemeStoreTestSuite) TestCreateTheme_DBStoreError() {
	testErr := errors.New("db error")
	createReq := CreateThemeRequest{
		DisplayName: "Test Theme",
	}
	suite.mockDBStore.On("CreateTheme", "theme1", createReq).Return(testErr)

	err := suite.store.CreateTheme("theme1", createReq)

	suite.Error(err)
	suite.Equal(testErr, err)
}

// Test GetTheme - From DB store (DB takes precedence)
func (suite *CompositeThemeStoreTestSuite) TestGetTheme_FromDBStore() {
	expectedTheme := Theme{ID: "theme1", DisplayName: "DB Theme"}
	suite.mockDBStore.On("GetTheme", "theme1").Return(expectedTheme, nil)

	theme, err := suite.store.GetTheme("theme1")

	suite.NoError(err)
	suite.Equal(expectedTheme.ID, theme.ID)
	suite.Equal(expectedTheme.DisplayName, theme.DisplayName)
	suite.False(theme.IsReadOnly)
}

// Test GetTheme - From file store (fallback when not in DB store)
func (suite *CompositeThemeStoreTestSuite) TestGetTheme_FromFileStore() {
	expectedTheme := Theme{ID: "theme1", DisplayName: "File Theme"}
	suite.mockDBStore.On("GetTheme", "theme1").Return(Theme{}, errThemeNotFound)
	suite.mockFileStore.On("GetTheme", "theme1").Return(expectedTheme, nil)

	theme, err := suite.store.GetTheme("theme1")

	suite.NoError(err)
	suite.Equal(expectedTheme.ID, theme.ID)
	suite.Equal(expectedTheme.DisplayName, theme.DisplayName)
	suite.True(theme.IsReadOnly)
}

// Test GetTheme - Not found in either store
func (suite *CompositeThemeStoreTestSuite) TestGetTheme_NotFound() {
	suite.mockFileStore.On("GetTheme", "theme1").Return(Theme{}, errThemeNotFound)
	suite.mockDBStore.On("GetTheme", "theme1").Return(Theme{}, errThemeNotFound)

	_, err := suite.store.GetTheme("theme1")

	suite.Error(err)
	suite.Equal(errThemeNotFound, err)
}

// Test IsThemeExist - Exists in DB store
func (suite *CompositeThemeStoreTestSuite) TestIsThemeExist_InDBStore() {
	suite.mockDBStore.On("IsThemeExist", "theme1").Return(true, nil)

	exists, err := suite.store.IsThemeExist("theme1")

	suite.NoError(err)
	suite.True(exists)
}

// Test IsThemeExist - Exists in file store
func (suite *CompositeThemeStoreTestSuite) TestIsThemeExist_InFileStore() {
	suite.mockDBStore.On("IsThemeExist", "theme1").Return(false, nil)
	suite.mockFileStore.On("IsThemeExist", "theme1").Return(true, nil)

	exists, err := suite.store.IsThemeExist("theme1")

	suite.NoError(err)
	suite.True(exists)
}

// Test IsThemeExist - Not found
func (suite *CompositeThemeStoreTestSuite) TestIsThemeExist_NotFound() {
	suite.mockDBStore.On("IsThemeExist", "theme1").Return(false, nil)
	suite.mockFileStore.On("IsThemeExist", "theme1").Return(false, nil)

	exists, err := suite.store.IsThemeExist("theme1")

	suite.NoError(err)
	suite.False(exists)
}

// Test IsThemeExist - DB store error
func (suite *CompositeThemeStoreTestSuite) TestIsThemeExist_DBStoreError() {
	testErr := errors.New("db error")
	suite.mockDBStore.On("IsThemeExist", "theme1").Return(false, testErr)

	_, err := suite.store.IsThemeExist("theme1")

	suite.Error(err)
	suite.Equal(testErr, err)
}

// Test UpdateTheme - Success (DB theme)
func (suite *CompositeThemeStoreTestSuite) TestUpdateTheme_Success() {
	updateReq := UpdateThemeRequest{
		DisplayName: "Updated Theme",
	}
	suite.mockFileStore.On("IsThemeExist", "theme1").Return(false, nil)
	suite.mockDBStore.On("UpdateTheme", "theme1", updateReq).Return(nil)

	err := suite.store.UpdateTheme("theme1", updateReq)

	suite.NoError(err)
}

// Test UpdateTheme - Rejects declarative theme
func (suite *CompositeThemeStoreTestSuite) TestUpdateTheme_RejectsDeclarative() {
	updateReq := UpdateThemeRequest{
		DisplayName: "Updated Theme",
	}
	suite.mockFileStore.On("IsThemeExist", "theme1").Return(true, nil)

	err := suite.store.UpdateTheme("theme1", updateReq)

	suite.Error(err)
	suite.Equal(errCannotUpdateDeclarativeTheme, err)
}

// Test UpdateTheme - File store check error
func (suite *CompositeThemeStoreTestSuite) TestUpdateTheme_FileStoreCheckError() {
	testErr := errors.New("file store error")
	updateReq := UpdateThemeRequest{
		DisplayName: "Updated Theme",
	}
	suite.mockFileStore.On("IsThemeExist", "theme1").Return(false, testErr)

	err := suite.store.UpdateTheme("theme1", updateReq)

	suite.Error(err)
	suite.Equal(testErr, err)
}

// Test DeleteTheme - Success (DB theme)
func (suite *CompositeThemeStoreTestSuite) TestDeleteTheme_Success() {
	suite.mockFileStore.On("IsThemeExist", "theme1").Return(false, nil)
	suite.mockDBStore.On("DeleteTheme", "theme1").Return(nil)

	err := suite.store.DeleteTheme("theme1")

	suite.NoError(err)
}

// Test DeleteTheme - Rejects declarative theme
func (suite *CompositeThemeStoreTestSuite) TestDeleteTheme_RejectsDeclarative() {
	suite.mockFileStore.On("IsThemeExist", "theme1").Return(true, nil)

	err := suite.store.DeleteTheme("theme1")

	suite.Error(err)
	suite.Equal(errCannotDeleteDeclarativeTheme, err)
}

// Test DeleteTheme - File store check error
func (suite *CompositeThemeStoreTestSuite) TestDeleteTheme_FileStoreCheckError() {
	testErr := errors.New("file store error")
	suite.mockFileStore.On("IsThemeExist", "theme1").Return(false, testErr)

	err := suite.store.DeleteTheme("theme1")

	suite.Error(err)
	suite.Equal(testErr, err)
}

// Test IsThemeDeclarative - True for file-based theme
func (suite *CompositeThemeStoreTestSuite) TestIsThemeDeclarative_True() {
	suite.mockFileStore.On("IsThemeExist", "theme1").Return(true, nil)

	isDeclarative := suite.store.IsThemeDeclarative("theme1")

	suite.True(isDeclarative)
}

// Test IsThemeDeclarative - False for DB theme
func (suite *CompositeThemeStoreTestSuite) TestIsThemeDeclarative_False() {
	suite.mockFileStore.On("IsThemeExist", "theme1").Return(false, nil)

	isDeclarative := suite.store.IsThemeDeclarative("theme1")

	suite.False(isDeclarative)
}

// Test IsThemeDeclarative - False on error
func (suite *CompositeThemeStoreTestSuite) TestIsThemeDeclarative_Error() {
	suite.mockFileStore.On("IsThemeExist", "theme1").Return(false, errors.New("error"))

	isDeclarative := suite.store.IsThemeDeclarative("theme1")

	suite.False(isDeclarative)
}

// Test mergeAndDeduplicateThemes - File themes take precedence
func (suite *CompositeThemeStoreTestSuite) TestMergeAndDeduplicateThemes() {
	dbThemes := []Theme{
		{ID: "theme1", DisplayName: "DB Theme 1"},
		{ID: "theme2", DisplayName: "DB Theme 2"},
	}
	fileThemes := []Theme{
		{ID: "theme2", DisplayName: "File Theme 2"}, // Duplicate ID
		{ID: "theme3", DisplayName: "File Theme 3"},
	}

	merged := mergeAndDeduplicateThemes(dbThemes, fileThemes)

	suite.Len(merged, 3)
	// File theme2 should take precedence
	suite.Equal("File Theme 2", merged[0].DisplayName)
	suite.Equal("File Theme 3", merged[1].DisplayName)
	suite.Equal("DB Theme 1", merged[2].DisplayName)
}

// Test mergeAndDeduplicateThemes - Empty stores
func (suite *CompositeThemeStoreTestSuite) TestMergeAndDeduplicateThemes_EmptyStores() {
	merged := mergeAndDeduplicateThemes([]Theme{}, []Theme{})

	suite.Len(merged, 0)
}

// Test IsThemeHandleConflict - Conflict in file store (returns early)
func (suite *CompositeThemeStoreTestSuite) TestIsThemeHandleConflict_ConflictInFileStore() {
	suite.mockFileStore.On("IsThemeHandleConflict", "classic", "").Return(true, nil)

	conflict, err := suite.store.IsThemeHandleConflict("classic", "")

	suite.NoError(err)
	suite.True(conflict)
}

// Test IsThemeHandleConflict - No conflict in file, conflict in DB store
func (suite *CompositeThemeStoreTestSuite) TestIsThemeHandleConflict_ConflictInDBStore() {
	suite.mockFileStore.On("IsThemeHandleConflict", "classic", "theme1").Return(false, nil)
	suite.mockDBStore.On("IsThemeHandleConflict", "classic", "theme1").Return(true, nil)

	conflict, err := suite.store.IsThemeHandleConflict("classic", "theme1")

	suite.NoError(err)
	suite.True(conflict)
}

// Test IsThemeHandleConflict - No conflict in either store
func (suite *CompositeThemeStoreTestSuite) TestIsThemeHandleConflict_NoConflict() {
	suite.mockFileStore.On("IsThemeHandleConflict", "unique-handle", "theme1").Return(false, nil)
	suite.mockDBStore.On("IsThemeHandleConflict", "unique-handle", "theme1").Return(false, nil)

	conflict, err := suite.store.IsThemeHandleConflict("unique-handle", "theme1")

	suite.NoError(err)
	suite.False(conflict)
}

// Test IsThemeHandleConflict - File store error
func (suite *CompositeThemeStoreTestSuite) TestIsThemeHandleConflict_FileStoreError() {
	testErr := errors.New("file store error")
	suite.mockFileStore.On("IsThemeHandleConflict", "classic", "").Return(false, testErr)

	conflict, err := suite.store.IsThemeHandleConflict("classic", "")

	suite.Error(err)
	suite.Equal(testErr, err)
	suite.False(conflict)
}

// Test IsThemeHandleConflict - DB store error
func (suite *CompositeThemeStoreTestSuite) TestIsThemeHandleConflict_DBStoreError() {
	testErr := errors.New("db store error")
	suite.mockFileStore.On("IsThemeHandleConflict", "classic", "theme1").Return(false, nil)
	suite.mockDBStore.On("IsThemeHandleConflict", "classic", "theme1").Return(false, testErr)

	conflict, err := suite.store.IsThemeHandleConflict("classic", "theme1")

	suite.Error(err)
	suite.Equal(testErr, err)
	suite.False(conflict)
}
