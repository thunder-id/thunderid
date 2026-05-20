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

package entityprovider

import (
	"encoding/json"
	"errors"

	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/entity"
)

// ----- DefaultEntityProvider — previously uncovered methods -----

func (suite *DefaultEntityProviderTestSuite) TestSearchEntities() {
	filters := map[string]interface{}{"email": "test@example.com"}
	found := []entity.Entity{
		{ID: testEntityID, Category: entity.EntityCategoryUser, Type: "customer"},
	}

	// Test Success
	suite.mockService.On("SearchEntities", mock.Anything, filters).Return(found, nil).Once()

	result, err := suite.provider.SearchEntities(filters)
	suite.Nil(err)
	suite.Len(result, 1)
	suite.Equal(testEntityID, result[0].ID)

	// Test Not Found
	suite.mockService.On("SearchEntities", mock.Anything, filters).
		Return(nil, entity.ErrEntityNotFound).Once()

	result, err = suite.provider.SearchEntities(filters)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorCodeEntityNotFound, err.Code)

	// Test System Error
	suite.mockService.On("SearchEntities", mock.Anything, filters).
		Return(nil, errors.New("db error")).Once()

	result, err = suite.provider.SearchEntities(filters)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorCodeSystemError, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestUpdateAttributes() {
	attrs := json.RawMessage(`{"displayName":"Alice"}`)

	// Test Success
	suite.mockService.On("UpdateAttributes", mock.Anything, testEntityID, attrs).
		Return(nil).Once()

	err := suite.provider.UpdateAttributes(testEntityID, attrs)
	suite.Nil(err)

	// Test Not Found
	suite.mockService.On("UpdateAttributes", mock.Anything, testEntityID, attrs).
		Return(entity.ErrEntityNotFound).Once()

	err = suite.provider.UpdateAttributes(testEntityID, attrs)
	suite.NotNil(err)
	suite.Equal(ErrorCodeEntityNotFound, err.Code)

	// Test Bad Attributes
	suite.mockService.On("UpdateAttributes", mock.Anything, testEntityID, attrs).
		Return(entity.ErrBadAttributesInRequest).Once()

	err = suite.provider.UpdateAttributes(testEntityID, attrs)
	suite.NotNil(err)
	suite.Equal(ErrorCodeInvalidRequestFormat, err.Code)

	// Test System Error
	suite.mockService.On("UpdateAttributes", mock.Anything, testEntityID, attrs).
		Return(errors.New("db error")).Once()

	err = suite.provider.UpdateAttributes(testEntityID, attrs)
	suite.NotNil(err)
	suite.Equal(ErrorCodeSystemError, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestGetEntityListCount() {
	filters := map[string]interface{}{}

	// Test Success
	suite.mockService.On("GetEntityListCount", mock.Anything, entity.EntityCategory("user"), filters).
		Return(42, nil).Once()

	count, err := suite.provider.GetEntityListCount(EntityCategoryUser, filters)
	suite.Nil(err)
	suite.Equal(42, count)

	// Test System Error
	suite.mockService.On("GetEntityListCount", mock.Anything, entity.EntityCategory("user"), filters).
		Return(0, errors.New("db error")).Once()

	count, err = suite.provider.GetEntityListCount(EntityCategoryUser, filters)
	suite.Equal(0, count)
	suite.NotNil(err)
	suite.Equal(ErrorCodeSystemError, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestGetEntityList() {
	filters := map[string]interface{}{}
	entities := []entity.Entity{
		{ID: "id1", Category: entity.EntityCategoryUser, Type: "customer"},
		{ID: "id2", Category: entity.EntityCategoryUser, Type: "customer"},
	}

	// Test Success
	suite.mockService.On("GetEntityList", mock.Anything, entity.EntityCategory("user"), 10, 0, filters).
		Return(entities, nil).Once()

	result, err := suite.provider.GetEntityList(EntityCategoryUser, 10, 0, filters)
	suite.Nil(err)
	suite.Len(result, 2)
	suite.Equal("id1", result[0].ID)

	// Test Not Found
	suite.mockService.On("GetEntityList", mock.Anything, entity.EntityCategory("user"), 10, 0, filters).
		Return(nil, entity.ErrEntityNotFound).Once()

	result, err = suite.provider.GetEntityList(EntityCategoryUser, 10, 0, filters)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorCodeEntityNotFound, err.Code)

	// Test System Error
	suite.mockService.On("GetEntityList", mock.Anything, entity.EntityCategory("user"), 10, 0, filters).
		Return(nil, errors.New("db error")).Once()

	result, err = suite.provider.GetEntityList(EntityCategoryUser, 10, 0, filters)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorCodeSystemError, err.Code)
}

// ----- DisabledEntityProvider — previously uncovered methods -----

func (suite *DisabledEntityProviderTestSuite) TestSearchEntities() {
	result, err := suite.provider.SearchEntities(map[string]interface{}{})
	suite.Nil(result)
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestUpdateAttributes() {
	err := suite.provider.UpdateAttributes("entity-id", json.RawMessage{})
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestGetEntityListCount() {
	count, err := suite.provider.GetEntityListCount(EntityCategoryUser, nil)
	suite.Equal(0, count)
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestGetEntityList() {
	result, err := suite.provider.GetEntityList(EntityCategoryUser, 10, 0, nil)
	suite.Nil(result)
	suite.Equal(errNotImplemented, err)
}
