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

package entitytype

import (
	"context"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type EntityTypeToolsTestSuite struct {
	suite.Suite
}

func TestEntityTypeToolsTestSuite(t *testing.T) {
	suite.Run(t, new(EntityTypeToolsTestSuite))
}

func (suite *EntityTypeToolsTestSuite) TestListUserTypes_Success() {
	mockService := NewEntityTypeServiceInterfaceMock(suite.T())
	tools := &entityTypeTools{entityTypeService: mockService}

	expectedTypes := []EntityTypeListItem{
		{
			ID:                    "et1",
			Name:                  "Employee",
			OUID:                  "ou1",
			OUHandle:              "engineering",
			AllowSelfRegistration: false,
			IsReadOnly:            false,
		},
		{
			ID:                    "et2",
			Name:                  "Customer",
			OUID:                  "ou2",
			OUHandle:              "sales",
			AllowSelfRegistration: true,
			IsReadOnly:            true,
		},
	}

	mockService.On("GetEntityTypeList", mock.Anything, TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(&EntityTypeListResponse{
			TotalResults: 2,
			Types:        expectedTypes,
		}, nil)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.listUserTypes(ctx, req, nil)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), output)
	assert.Equal(suite.T(), 2, output.TotalResults)
	assert.Len(suite.T(), output.Types, 2)

	item := output.Types[0]
	assert.Equal(suite.T(), "et1", item.ID)
	assert.Equal(suite.T(), "Employee", item.Name)
	assert.Equal(suite.T(), "ou1", item.OUID)
	assert.Equal(suite.T(), "engineering", item.OUHandle)
	assert.False(suite.T(), item.AllowSelfRegistration)
	assert.False(suite.T(), item.IsReadOnly)

	item2 := output.Types[1]
	assert.Equal(suite.T(), "et2", item2.ID)
	assert.True(suite.T(), item2.AllowSelfRegistration)
	assert.True(suite.T(), item2.IsReadOnly)

	mockService.AssertExpectations(suite.T())
}

func (suite *EntityTypeToolsTestSuite) TestListUserTypes_ServiceError() {
	mockService := NewEntityTypeServiceInterfaceMock(suite.T())
	tools := &entityTypeTools{entityTypeService: mockService}

	mockService.On("GetEntityTypeList", mock.Anything, TypeCategoryUser, mock.Anything, mock.Anything, false).
		Return(nil, &tidcommon.ServiceError{
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "database error"},
		})

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	result, output, err := tools.listUserTypes(ctx, req, nil)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), output)
	assert.Contains(suite.T(), err.Error(), "failed to list user types")

	mockService.AssertExpectations(suite.T())
}
