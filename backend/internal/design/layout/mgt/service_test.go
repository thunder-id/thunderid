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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// Test Suite
type LayoutServiceTestSuite struct {
	suite.Suite
	mockStore *layoutMgtStoreInterfaceMock
	service   LayoutMgtServiceInterface
}

func TestLayoutServiceTestSuite(t *testing.T) {
	suite.Run(t, new(LayoutServiceTestSuite))
}

func (suite *LayoutServiceTestSuite) SetupTest() {
	// Initialize config runtime with default values
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	if err != nil {
		suite.Fail("Failed to initialize runtime", err)
	}

	suite.mockStore = newLayoutMgtStoreInterfaceMock(suite.T())
	suite.service = newLayoutMgtService(suite.mockStore)
}

// Test GetLayoutList - Success
func (suite *LayoutServiceTestSuite) TestGetLayoutList_Success() {
	layouts := []Layout{
		{
			ID:          "layout-1",
			DisplayName: "Classic Layout",
			Description: "A classic layout",
			Layout:      json.RawMessage(`{"structure": "centered"}`),
		},
		{
			ID:          "layout-2",
			DisplayName: "Modern Layout",
			Description: "A modern layout",
			Layout:      json.RawMessage(`{"structure": "sidebar"}`),
		},
	}

	suite.mockStore.On("GetLayoutListCount").Return(2, nil)
	suite.mockStore.On("GetLayoutList", 10, 0).Return(layouts, nil)

	result, err := suite.service.GetLayoutList(10, 0)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), 2, result.TotalResults)
	assert.Equal(suite.T(), 2, result.Count)
	assert.Equal(suite.T(), 1, result.StartIndex)
	assert.Len(suite.T(), result.Layouts, 2)
}

// Test GetLayoutList - Store Count Error
func (suite *LayoutServiceTestSuite) TestGetLayoutList_CountError() {
	suite.mockStore.On("GetLayoutListCount").Return(0, errors.New("database error"))

	result, err := suite.service.GetLayoutList(10, 0)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
}

// Test GetLayoutList - Store Error
func (suite *LayoutServiceTestSuite) TestGetLayoutList_StoreError() {
	suite.mockStore.On("GetLayoutListCount").Return(2, nil)
	suite.mockStore.On("GetLayoutList", 10, 0).Return(nil, errors.New("database error"))

	result, err := suite.service.GetLayoutList(10, 0)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
}

// Test GetLayoutList - Invalid Pagination
func (suite *LayoutServiceTestSuite) TestGetLayoutList_InvalidLimit() {
	result, err := suite.service.GetLayoutList(-1, 0)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "LAY-1009", err.Code)
}

func (suite *LayoutServiceTestSuite) TestGetLayoutList_InvalidOffset() {
	result, err := suite.service.GetLayoutList(10, -1)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "LAY-1010", err.Code)
}

// Test CreateLayout - Success
func (suite *LayoutServiceTestSuite) TestCreateLayout_Success() {
	layoutRequest := CreateLayoutRequest{
		Handle:      "new-layout",
		DisplayName: "New Layout",
		Description: "A new layout",
		Layout:      json.RawMessage(`{"structure": "grid"}`),
	}

	suite.mockStore.On("IsLayoutHandleConflict", "new-layout", "").Return(false, nil)
	suite.mockStore.On("CreateLayout", mock.AnythingOfType("string"), layoutRequest).Return(nil)

	result, err := suite.service.CreateLayout(layoutRequest)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "new-layout", result.Handle)
	assert.Equal(suite.T(), "New Layout", result.DisplayName)
	assert.Equal(suite.T(), "A new layout", result.Description)
	assert.NotEmpty(suite.T(), result.ID)
}

// Test CreateLayout - Missing Display Name
func (suite *LayoutServiceTestSuite) TestCreateLayout_MissingDisplayName() {
	layoutRequest := CreateLayoutRequest{
		Handle:      "my-layout",
		DisplayName: "",
		Description: "A layout without name",
		Layout:      json.RawMessage(`{"structure": "grid"}`),
	}

	result, err := suite.service.CreateLayout(layoutRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "LAY-1005", err.Code)
}

// Test CreateLayout - Missing Handle
func (suite *LayoutServiceTestSuite) TestCreateLayout_MissingHandle() {
	layoutRequest := CreateLayoutRequest{
		Handle:      "",
		DisplayName: "My Layout",
		Description: "A layout without handle",
		Layout:      json.RawMessage(`{"structure": "grid"}`),
	}

	result, err := suite.service.CreateLayout(layoutRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "LAY-1017", err.Code)
}

// Test CreateLayout - Duplicate Handle
func (suite *LayoutServiceTestSuite) TestCreateLayout_DuplicateHandle() {
	layoutRequest := CreateLayoutRequest{
		Handle:      "existing-layout",
		DisplayName: "My Layout",
		Description: "A layout with duplicate handle",
		Layout:      json.RawMessage(`{"structure": "grid"}`),
	}

	suite.mockStore.On("IsLayoutHandleConflict", "existing-layout", "").Return(true, nil)

	result, err := suite.service.CreateLayout(layoutRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "LAY-1016", err.Code)
}

// Test CreateLayout - Declarative mode enabled
func (suite *LayoutServiceTestSuite) TestCreateLayout_DeclarativeModeEnabled() {
	runtime := config.GetServerRuntime()
	runtime.Config.Layout.Store = "declarative"

	layoutRequest := CreateLayoutRequest{
		Handle:      "declarative-layout",
		DisplayName: "Declarative Layout",
		Description: "Should be blocked",
		Layout:      json.RawMessage(`{"structure": "grid"}`),
	}

	result, err := suite.service.CreateLayout(layoutRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "LAY-1015", err.Code)
}

// Test CreateLayout - Invalid Layout JSON
func (suite *LayoutServiceTestSuite) TestCreateLayout_InvalidJSON() {
	layoutRequest := CreateLayoutRequest{
		Handle:      "my-layout",
		DisplayName: "Layout",
		Description: "Invalid JSON layout",
		Layout:      json.RawMessage(`{invalid json}`),
	}

	suite.mockStore.On("IsLayoutHandleConflict", "my-layout", "").Return(false, nil)

	result, err := suite.service.CreateLayout(layoutRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "LAY-1007", err.Code)
}

// Test CreateLayout - Store Error
func (suite *LayoutServiceTestSuite) TestCreateLayout_StoreError() {
	layoutRequest := CreateLayoutRequest{
		Handle:      "my-layout",
		DisplayName: "Layout",
		Description: "A layout",
		Layout:      json.RawMessage(`{"structure": "grid"}`),
	}

	suite.mockStore.On("IsLayoutHandleConflict", "my-layout", "").Return(false, nil)
	suite.mockStore.On("CreateLayout", mock.AnythingOfType("string"), layoutRequest).
		Return(errors.New("database error"))

	result, err := suite.service.CreateLayout(layoutRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
}

// Test GetLayout - Success
func (suite *LayoutServiceTestSuite) TestGetLayout_Success() {
	layout := Layout{
		ID:          "layout-123",
		DisplayName: "Test Layout",
		Description: "A test layout",
		Layout:      json.RawMessage(`{"structure": "centered"}`),
	}

	suite.mockStore.On("GetLayout", "layout-123").Return(layout, nil)

	result, err := suite.service.GetLayout("layout-123")

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "layout-123", result.ID)
	assert.Equal(suite.T(), "Test Layout", result.DisplayName)
}

// Test GetLayout - Invalid ID
func (suite *LayoutServiceTestSuite) TestGetLayout_InvalidID() {
	result, err := suite.service.GetLayout("")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "LAY-1002", err.Code)
}

// Test GetLayout - Not Found
func (suite *LayoutServiceTestSuite) TestGetLayout_NotFound() {
	suite.mockStore.On("GetLayout", "non-existent").Return(Layout{}, errLayoutNotFound)

	result, err := suite.service.GetLayout("non-existent")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "LAY-1003", err.Code)
}

// Test GetLayout - Store Error
func (suite *LayoutServiceTestSuite) TestGetLayout_StoreError() {
	suite.mockStore.On("GetLayout", "layout-123").Return(Layout{}, errors.New("database error"))

	result, err := suite.service.GetLayout("layout-123")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
}

// Test UpdateLayout - Success
func (suite *LayoutServiceTestSuite) TestUpdateLayout_Success() {
	updateRequest := UpdateLayoutRequest{
		Handle:      "my-layout",
		DisplayName: "Updated Layout",
		Description: "An updated layout",
		Layout:      json.RawMessage(`{"structure": "flex"}`),
	}
	existingLayout := Layout{
		ID:     "layout-123",
		Handle: "my-layout",
	}

	suite.mockStore.On("IsLayoutDeclarative", "layout-123").Return(false)
	suite.mockStore.On("GetLayout", "layout-123").Return(existingLayout, nil)
	suite.mockStore.On("UpdateLayout", "layout-123", updateRequest).Return(nil)

	result, err := suite.service.UpdateLayout("layout-123", updateRequest)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "layout-123", result.ID)
	assert.Equal(suite.T(), "my-layout", result.Handle)
	assert.Equal(suite.T(), "Updated Layout", result.DisplayName)
}

// Test UpdateLayout - Omitted Handle uses existing handle
func (suite *LayoutServiceTestSuite) TestUpdateLayout_OmittedHandle_UsesExisting() {
	updateRequest := UpdateLayoutRequest{
		Handle:      "",
		DisplayName: "Updated Layout",
		Description: "An updated layout",
		Layout:      json.RawMessage(`{"structure": "flex"}`),
	}
	existingLayout := Layout{
		ID:     "layout-123",
		Handle: "existing-handle",
	}

	suite.mockStore.On("IsLayoutDeclarative", "layout-123").Return(false)
	suite.mockStore.On("GetLayout", "layout-123").Return(existingLayout, nil)
	suite.mockStore.On("UpdateLayout", "layout-123", updateRequest).Return(nil)

	result, err := suite.service.UpdateLayout("layout-123", updateRequest)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "existing-handle", result.Handle)
}

// Test UpdateLayout - Invalid ID
func (suite *LayoutServiceTestSuite) TestUpdateLayout_InvalidID() {
	updateRequest := UpdateLayoutRequest{
		Handle:      "my-layout",
		DisplayName: "Layout",
		Description: "A layout",
		Layout:      json.RawMessage(`{"structure": "grid"}`),
	}

	result, err := suite.service.UpdateLayout("", updateRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "LAY-1002", err.Code)
}

// Test UpdateLayout - Missing Display Name
func (suite *LayoutServiceTestSuite) TestUpdateLayout_MissingDisplayName() {
	updateRequest := UpdateLayoutRequest{
		Handle:      "my-layout",
		DisplayName: "",
		Description: "A layout",
		Layout:      json.RawMessage(`{"structure": "grid"}`),
	}

	result, err := suite.service.UpdateLayout("layout-123", updateRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "LAY-1005", err.Code)
}

// Test UpdateLayout - Immutable Handle
func (suite *LayoutServiceTestSuite) TestUpdateLayout_ImmutableHandle() {
	updateRequest := UpdateLayoutRequest{
		Handle:      "different-handle",
		DisplayName: "Layout",
		Description: "A layout",
		Layout:      json.RawMessage(`{"structure": "grid"}`),
	}
	existingLayout := Layout{
		ID:     "layout-123",
		Handle: "my-layout",
	}

	suite.mockStore.On("IsLayoutDeclarative", "layout-123").Return(false)
	suite.mockStore.On("GetLayout", "layout-123").Return(existingLayout, nil)

	result, err := suite.service.UpdateLayout("layout-123", updateRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "LAY-1018", err.Code)
}

// Test UpdateLayout - Not Found
func (suite *LayoutServiceTestSuite) TestUpdateLayout_NotFound() {
	updateRequest := UpdateLayoutRequest{
		Handle:      "my-layout",
		DisplayName: "Layout",
		Description: "A layout",
		Layout:      json.RawMessage(`{"structure": "grid"}`),
	}

	suite.mockStore.On("IsLayoutDeclarative", "non-existent").Return(false)
	suite.mockStore.On("GetLayout", "non-existent").Return(Layout{}, errLayoutNotFound)

	result, err := suite.service.UpdateLayout("non-existent", updateRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "LAY-1003", err.Code)
}

// Test UpdateLayout - Invalid JSON
func (suite *LayoutServiceTestSuite) TestUpdateLayout_InvalidJSON() {
	updateRequest := UpdateLayoutRequest{
		Handle:      "my-layout",
		DisplayName: "Layout",
		Description: "A layout",
		Layout:      json.RawMessage(`{invalid}`),
	}
	existingLayout := Layout{
		ID:     "layout-123",
		Handle: "my-layout",
	}
	suite.mockStore.On("IsLayoutDeclarative", "layout-123").Return(false)
	suite.mockStore.On("GetLayout", "layout-123").Return(existingLayout, nil)
	result, err := suite.service.UpdateLayout("layout-123", updateRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "LAY-1007", err.Code)
}

// Test DeleteLayout - Success
func (suite *LayoutServiceTestSuite) TestDeleteLayout_Success() {
	suite.mockStore.On("IsLayoutDeclarative", "layout-123").Return(false)
	suite.mockStore.On("IsLayoutExist", "layout-123").Return(true, nil)
	suite.mockStore.On("GetApplicationsCountByLayoutID", "layout-123").Return(0, nil)
	suite.mockStore.On("DeleteLayout", "layout-123").Return(nil)

	err := suite.service.DeleteLayout("layout-123")

	assert.Nil(suite.T(), err)
}

// Test DeleteLayout - Invalid ID
func (suite *LayoutServiceTestSuite) TestDeleteLayout_InvalidID() {
	err := suite.service.DeleteLayout("")

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "LAY-1002", err.Code)
}

// Test DeleteLayout - Not Found (idempotent delete returns success)
func (suite *LayoutServiceTestSuite) TestDeleteLayout_NotFound() {
	suite.mockStore.On("IsLayoutDeclarative", "non-existent").Return(false)
	suite.mockStore.On("IsLayoutExist", "non-existent").Return(false, nil)

	err := suite.service.DeleteLayout("non-existent")

	assert.Nil(suite.T(), err)
}

// Test DeleteLayout - Layout In Use
func (suite *LayoutServiceTestSuite) TestDeleteLayout_InUse() {
	suite.mockStore.On("IsLayoutDeclarative", "layout-123").Return(false)
	suite.mockStore.On("IsLayoutExist", "layout-123").Return(true, nil)
	suite.mockStore.On("GetApplicationsCountByLayoutID", "layout-123").Return(5, nil)

	err := suite.service.DeleteLayout("layout-123")

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "LAY-1008", err.Code)
	assert.Contains(suite.T(), err.ErrorDescription.DefaultValue, "5 application(s)")
}

// Test DeleteLayout - Store Error
func (suite *LayoutServiceTestSuite) TestDeleteLayout_StoreError() {
	suite.mockStore.On("IsLayoutDeclarative", "layout-123").Return(false)
	suite.mockStore.On("IsLayoutExist", "layout-123").Return(true, nil)
	suite.mockStore.On("GetApplicationsCountByLayoutID", "layout-123").Return(0, nil)
	suite.mockStore.On("DeleteLayout", "layout-123").Return(errors.New("database error"))

	err := suite.service.DeleteLayout("layout-123")

	assert.NotNil(suite.T(), err)
}

// Test IsLayoutExist - Exists
func (suite *LayoutServiceTestSuite) TestIsLayoutExist_True() {
	suite.mockStore.On("IsLayoutExist", "layout-123").Return(true, nil)

	exists, err := suite.service.IsLayoutExist("layout-123")

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), exists)
}

// Test IsLayoutExist - Not Exists
func (suite *LayoutServiceTestSuite) TestIsLayoutExist_False() {
	suite.mockStore.On("IsLayoutExist", "non-existent").Return(false, nil)

	exists, err := suite.service.IsLayoutExist("non-existent")

	assert.Nil(suite.T(), err)
	assert.False(suite.T(), exists)
}

// Test IsLayoutExist - Store Error
func (suite *LayoutServiceTestSuite) TestIsLayoutExist_StoreError() {
	suite.mockStore.On("IsLayoutExist", "layout-123").Return(false, errors.New("database error"))

	exists, err := suite.service.IsLayoutExist("layout-123")

	assert.NotNil(suite.T(), err)
	assert.False(suite.T(), exists)
}

// Test CreateLayout - Handle conflict check error
func (suite *LayoutServiceTestSuite) TestCreateLayout_HandleConflictError() {
	layoutRequest := CreateLayoutRequest{
		Handle:      "my-layout",
		DisplayName: "Layout",
		Description: "A layout",
		Layout:      json.RawMessage(`{"structure": "grid"}`),
	}

	suite.mockStore.On("IsLayoutHandleConflict", "my-layout", "").Return(false, errors.New("database error"))

	result, err := suite.service.CreateLayout(layoutRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
}

// Test UpdateLayout - GetLayout store error
func (suite *LayoutServiceTestSuite) TestUpdateLayout_GetLayoutError() {
	updateRequest := UpdateLayoutRequest{
		Handle:      "my-layout",
		DisplayName: "Layout",
		Description: "A layout",
		Layout:      json.RawMessage(`{"structure": "grid"}`),
	}

	suite.mockStore.On("IsLayoutDeclarative", "layout-123").Return(false)
	suite.mockStore.On("GetLayout", "layout-123").Return(Layout{}, errors.New("database error"))

	result, err := suite.service.UpdateLayout("layout-123", updateRequest)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
}

// Test DeleteLayout - Applications count error
func (suite *LayoutServiceTestSuite) TestDeleteLayout_ApplicationsCountError() {
	suite.mockStore.On("IsLayoutDeclarative", "layout-123").Return(false)
	suite.mockStore.On("IsLayoutExist", "layout-123").Return(true, nil)
	suite.mockStore.On("GetApplicationsCountByLayoutID", "layout-123").Return(0, errors.New("database error"))

	err := suite.service.DeleteLayout("layout-123")

	assert.NotNil(suite.T(), err)
}
