// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entityprovider

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// ----- DefaultEntityProvider — previously uncovered methods -----

func (suite *DefaultEntityProviderTestSuite) TestSearchEntities() {
	filters := map[string]interface{}{"email": "test@example.com"}
	found := []providers.Entity{
		{ID: testEntityID, Category: providers.EntityCategoryUser, Type: "customer"},
	}

	// Test Success
	suite.mockService.On("SearchEntities", mock.Anything, filters).Return(found, nil).Once()

	result, err := suite.provider.SearchEntities(context.Background(), filters)
	suite.Nil(err)
	suite.Len(result, 1)
	suite.Equal(testEntityID, result[0].ID)

	// Test Not Found
	suite.mockService.On("SearchEntities", mock.Anything, filters).
		Return(nil, entity.ErrEntityNotFound).Once()

	result, err = suite.provider.SearchEntities(context.Background(), filters)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorCodeEntityNotFound, err.Code)

	// Test System Error
	suite.mockService.On("SearchEntities", mock.Anything, filters).
		Return(nil, errors.New("db error")).Once()

	result, err = suite.provider.SearchEntities(context.Background(), filters)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorCodeSystemError, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestUpdateAttributes() {
	attrs := json.RawMessage(`{"displayName":"Alice"}`)

	// Test Success
	suite.mockService.On("UpdateAttributes", mock.Anything, testEntityID, attrs).
		Return(nil).Once()

	err := suite.provider.UpdateAttributes(context.Background(), testEntityID, attrs)
	suite.Nil(err)

	// Test Not Found
	suite.mockService.On("UpdateAttributes", mock.Anything, testEntityID, attrs).
		Return(entity.ErrEntityNotFound).Once()

	err = suite.provider.UpdateAttributes(context.Background(), testEntityID, attrs)
	suite.NotNil(err)
	suite.Equal(ErrorCodeEntityNotFound, err.Code)

	// Test Bad Attributes
	suite.mockService.On("UpdateAttributes", mock.Anything, testEntityID, attrs).
		Return(entity.ErrBadAttributesInRequest).Once()

	err = suite.provider.UpdateAttributes(context.Background(), testEntityID, attrs)
	suite.NotNil(err)
	suite.Equal(ErrorCodeInvalidRequestFormat, err.Code)

	// Test System Error
	suite.mockService.On("UpdateAttributes", mock.Anything, testEntityID, attrs).
		Return(errors.New("db error")).Once()

	err = suite.provider.UpdateAttributes(context.Background(), testEntityID, attrs)
	suite.NotNil(err)
	suite.Equal(ErrorCodeSystemError, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestGetEntityListCount() {
	filters := map[string]interface{}{}

	// Test Success
	suite.mockService.On("GetEntityListCount", mock.Anything, providers.EntityCategory("user"), filters).
		Return(42, nil).Once()

	count, err := suite.provider.GetEntityListCount(context.Background(), providers.EntityCategoryUser, filters)
	suite.Nil(err)
	suite.Equal(42, count)

	// Test System Error
	suite.mockService.On("GetEntityListCount", mock.Anything, providers.EntityCategory("user"), filters).
		Return(0, errors.New("db error")).Once()

	count, err = suite.provider.GetEntityListCount(context.Background(), providers.EntityCategoryUser, filters)
	suite.Equal(0, count)
	suite.NotNil(err)
	suite.Equal(ErrorCodeSystemError, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestGetEntityList() {
	filters := map[string]interface{}{}
	entities := []providers.Entity{
		{ID: "id1", Category: providers.EntityCategoryUser, Type: "customer"},
		{ID: "id2", Category: providers.EntityCategoryUser, Type: "customer"},
	}

	// Test Success
	suite.mockService.On("GetEntityList", mock.Anything, providers.EntityCategory("user"), 10, 0, filters).
		Return(entities, nil).Once()

	result, err := suite.provider.GetEntityList(context.Background(), providers.EntityCategoryUser, 10, 0, filters)
	suite.Nil(err)
	suite.Len(result, 2)
	suite.Equal("id1", result[0].ID)

	// Test Not Found
	suite.mockService.On("GetEntityList", mock.Anything, providers.EntityCategory("user"), 10, 0, filters).
		Return(nil, entity.ErrEntityNotFound).Once()

	result, err = suite.provider.GetEntityList(context.Background(), providers.EntityCategoryUser, 10, 0, filters)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorCodeEntityNotFound, err.Code)

	// Test System Error
	suite.mockService.On("GetEntityList", mock.Anything, providers.EntityCategory("user"), 10, 0, filters).
		Return(nil, errors.New("db error")).Once()

	result, err = suite.provider.GetEntityList(context.Background(), providers.EntityCategoryUser, 10, 0, filters)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorCodeSystemError, err.Code)
}

// ----- DisabledEntityProvider — previously uncovered methods -----

func (suite *DisabledEntityProviderTestSuite) TestSearchEntities() {
	result, err := suite.provider.SearchEntities(context.Background(), map[string]interface{}{})
	suite.Nil(result)
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestUpdateAttributes() {
	err := suite.provider.UpdateAttributes(context.Background(), "entity-id", json.RawMessage{})
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestGetEntityListCount() {
	count, err := suite.provider.GetEntityListCount(context.Background(), providers.EntityCategoryUser, nil)
	suite.Equal(0, count)
	suite.Equal(errNotImplemented, err)
}

func (suite *DisabledEntityProviderTestSuite) TestGetEntityList() {
	result, err := suite.provider.GetEntityList(context.Background(), providers.EntityCategoryUser, 10, 0, nil)
	suite.Nil(result)
	suite.Equal(errNotImplemented, err)
}
