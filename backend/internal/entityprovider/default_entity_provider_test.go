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
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
)

type DefaultEntityProviderTestSuite struct {
	suite.Suite
	mockService *entitymock.EntityServiceInterfaceMock
	provider    EntityProviderInterface
}

func (suite *DefaultEntityProviderTestSuite) SetupTest() {
	suite.mockService = entitymock.NewEntityServiceInterfaceMock(suite.T())
	suite.provider = newDefaultEntityProvider(suite.mockService)
}

func TestDefaultEntityProviderTestSuite(t *testing.T) {
	suite.Run(t, new(DefaultEntityProviderTestSuite))
}

const testEntityID = "entity123"

func (suite *DefaultEntityProviderTestSuite) TestIdentifyEntity() {
	filters := map[string]interface{}{"clientId": "test-client"}
	idAddr := testEntityID

	// Test Success
	suite.mockService.On("IdentifyEntity", mock.Anything, filters).Return(&idAddr, nil).Once()

	id, err := suite.provider.IdentifyEntity(filters)
	suite.Nil(err)
	suite.Equal(testEntityID, *id)

	// Test Not Found
	suite.mockService.On("IdentifyEntity", mock.Anything, filters).
		Return(nil, entity.ErrEntityNotFound).Once()

	id, err = suite.provider.IdentifyEntity(filters)
	suite.Nil(id)
	suite.NotNil(err)
	suite.Equal(ErrorCodeEntityNotFound, err.Code)

	// Test System Error
	suite.mockService.On("IdentifyEntity", mock.Anything, filters).
		Return(nil, errors.New("db error")).Once()

	id, err = suite.provider.IdentifyEntity(filters)
	suite.Nil(id)
	suite.NotNil(err)
	suite.Equal(ErrorCodeSystemError, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestGetEntity() {
	expected := &entity.Entity{
		ID:       testEntityID,
		Category: entity.EntityCategoryUser,
		Type:     "customer",
	}

	// Test Success
	suite.mockService.On("GetEntity", mock.Anything, testEntityID).Return(expected, nil).Once()

	e, err := suite.provider.GetEntity(testEntityID)
	suite.Nil(err)
	suite.Equal(testEntityID, e.ID)
	suite.Equal(EntityCategory("user"), e.Category)

	// Test Not Found
	suite.mockService.On("GetEntity", mock.Anything, testEntityID).
		Return(nil, entity.ErrEntityNotFound).Once()

	e, err = suite.provider.GetEntity(testEntityID)
	suite.Nil(e)
	suite.NotNil(err)
	suite.Equal(ErrorCodeEntityNotFound, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestCreateEntity() {
	providerEntity := &Entity{
		ID:       testEntityID,
		Category: EntityCategoryApp,
		Type:     "application",
	}
	created := &entity.Entity{
		ID:       testEntityID,
		Category: entity.EntityCategoryApp,
		Type:     "application",
	}

	// Test Success
	suite.mockService.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return(created, nil).Once()

	e, err := suite.provider.CreateEntity(providerEntity, json.RawMessage(`{}`))
	suite.Nil(err)
	suite.Equal(testEntityID, e.ID)

	// Test Nil Entity
	e, err = suite.provider.CreateEntity(nil, nil)
	suite.Nil(e)
	suite.NotNil(err)
	suite.Equal(ErrorCodeInvalidRequestFormat, err.Code)

	// Test Attribute Conflict
	suite.mockService.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, entity.ErrAttributeConflict).Once()

	e, err = suite.provider.CreateEntity(providerEntity, nil)
	suite.Nil(e)
	suite.NotNil(err)
	suite.Equal(ErrorCodeAttributeConflict, err.Code)

	// Test Schema Validation Failed
	suite.mockService.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, entity.ErrSchemaValidationFailed).Once()

	e, err = suite.provider.CreateEntity(providerEntity, nil)
	suite.Nil(e)
	suite.NotNil(err)
	suite.Equal(ErrorCodeSchemaValidationFailed, err.Code)

	// Test Bad Attributes In Request
	suite.mockService.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, entity.ErrBadAttributesInRequest).Once()

	e, err = suite.provider.CreateEntity(providerEntity, nil)
	suite.Nil(e)
	suite.NotNil(err)
	suite.Equal(ErrorCodeInvalidRequestFormat, err.Code)

	// Test Invalid Credential
	suite.mockService.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, entity.ErrInvalidCredential).Once()

	e, err = suite.provider.CreateEntity(providerEntity, nil)
	suite.Nil(e)
	suite.NotNil(err)
	suite.Equal(ErrorCodeInvalidRequestFormat, err.Code)

	// Test System Error
	suite.mockService.On("CreateEntity", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("db error")).Once()

	e, err = suite.provider.CreateEntity(providerEntity, nil)
	suite.Nil(e)
	suite.NotNil(err)
	suite.Equal(ErrorCodeSystemError, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestUpdateEntity() {
	providerEntity := &Entity{
		ID:   testEntityID,
		Type: "customer",
	}
	updated := &entity.Entity{
		ID:   testEntityID,
		Type: "customer",
	}

	// Test Success
	suite.mockService.On("UpdateEntity", mock.Anything, testEntityID, mock.Anything).
		Return(updated, nil).Once()

	e, err := suite.provider.UpdateEntity(testEntityID, providerEntity)
	suite.Nil(err)
	suite.Equal(testEntityID, e.ID)

	// Test Nil Entity
	e, err = suite.provider.UpdateEntity(testEntityID, nil)
	suite.Nil(e)
	suite.NotNil(err)
	suite.Equal(ErrorCodeInvalidRequestFormat, err.Code)

	// Test Not Found
	suite.mockService.On("UpdateEntity", mock.Anything, testEntityID, mock.Anything).
		Return(nil, entity.ErrEntityNotFound).Once()

	e, err = suite.provider.UpdateEntity(testEntityID, providerEntity)
	suite.Nil(e)
	suite.NotNil(err)
	suite.Equal(ErrorCodeEntityNotFound, err.Code)

	// Test Attribute Conflict
	suite.mockService.On("UpdateEntity", mock.Anything, testEntityID, mock.Anything).
		Return(nil, entity.ErrAttributeConflict).Once()

	e, err = suite.provider.UpdateEntity(testEntityID, providerEntity)
	suite.Nil(e)
	suite.NotNil(err)
	suite.Equal(ErrorCodeAttributeConflict, err.Code)

	// Test Schema Validation Failed
	suite.mockService.On("UpdateEntity", mock.Anything, testEntityID, mock.Anything).
		Return(nil, entity.ErrSchemaValidationFailed).Once()

	e, err = suite.provider.UpdateEntity(testEntityID, providerEntity)
	suite.Nil(e)
	suite.NotNil(err)
	suite.Equal(ErrorCodeSchemaValidationFailed, err.Code)

	// Test Bad Attributes In Request
	suite.mockService.On("UpdateEntity", mock.Anything, testEntityID, mock.Anything).
		Return(nil, entity.ErrBadAttributesInRequest).Once()

	e, err = suite.provider.UpdateEntity(testEntityID, providerEntity)
	suite.Nil(e)
	suite.NotNil(err)
	suite.Equal(ErrorCodeInvalidRequestFormat, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestUpdateCredentials() {
	creds := json.RawMessage(`{"password":"newpassword"}`)

	// Test Success
	suite.mockService.On("UpdateCredentials", mock.Anything, testEntityID, creds).
		Return(nil).Once()

	err := suite.provider.UpdateCredentials(testEntityID, creds)
	suite.Nil(err)

	// Test Not Found
	suite.mockService.On("UpdateCredentials", mock.Anything, testEntityID, creds).
		Return(entity.ErrEntityNotFound).Once()

	err = suite.provider.UpdateCredentials(testEntityID, creds)
	suite.NotNil(err)
	suite.Equal(ErrorCodeEntityNotFound, err.Code)

	// Test Invalid Credential
	suite.mockService.On("UpdateCredentials", mock.Anything, testEntityID, creds).
		Return(entity.ErrInvalidCredential).Once()

	err = suite.provider.UpdateCredentials(testEntityID, creds)
	suite.NotNil(err)
	suite.Equal(ErrorCodeInvalidRequestFormat, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestDeleteEntity() {
	// Test Success
	suite.mockService.On("DeleteEntity", mock.Anything, testEntityID).Return(nil).Once()

	err := suite.provider.DeleteEntity(testEntityID)
	suite.Nil(err)

	// Test Not Found (returns nil — idempotent delete)
	suite.mockService.On("DeleteEntity", mock.Anything, testEntityID).
		Return(entity.ErrEntityNotFound).Once()

	err = suite.provider.DeleteEntity(testEntityID)
	suite.Nil(err)

	// Test System Error
	suite.mockService.On("DeleteEntity", mock.Anything, testEntityID).
		Return(errors.New("db error")).Once()

	err = suite.provider.DeleteEntity(testEntityID)
	suite.NotNil(err)
	suite.Equal(ErrorCodeSystemError, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestUpdateSystemAttributes() {
	attrs := json.RawMessage(`{"name":"test"}`)

	// Test Success
	suite.mockService.On("UpdateSystemAttributes", mock.Anything, testEntityID, attrs).
		Return(nil).Once()

	err := suite.provider.UpdateSystemAttributes(testEntityID, attrs)
	suite.Nil(err)

	// Test Not Found
	suite.mockService.On("UpdateSystemAttributes", mock.Anything, testEntityID, attrs).
		Return(entity.ErrEntityNotFound).Once()

	err = suite.provider.UpdateSystemAttributes(testEntityID, attrs)
	suite.NotNil(err)
	suite.Equal(ErrorCodeEntityNotFound, err.Code)

	// Test Attribute Conflict
	suite.mockService.On("UpdateSystemAttributes", mock.Anything, testEntityID, attrs).
		Return(entity.ErrAttributeConflict).Once()

	err = suite.provider.UpdateSystemAttributes(testEntityID, attrs)
	suite.NotNil(err)
	suite.Equal(ErrorCodeAttributeConflict, err.Code)

	// Test Bad Attributes In Request
	suite.mockService.On("UpdateSystemAttributes", mock.Anything, testEntityID, attrs).
		Return(entity.ErrBadAttributesInRequest).Once()

	err = suite.provider.UpdateSystemAttributes(testEntityID, attrs)
	suite.NotNil(err)
	suite.Equal(ErrorCodeInvalidRequestFormat, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestUpdateSystemCredentials() {
	creds := json.RawMessage(`{"clientSecret":"secret"}`)

	// Test Success
	suite.mockService.On("UpdateSystemCredentials", mock.Anything, testEntityID, creds).
		Return(nil).Once()

	err := suite.provider.UpdateSystemCredentials(testEntityID, creds)
	suite.Nil(err)

	// Test Not Found
	suite.mockService.On("UpdateSystemCredentials", mock.Anything, testEntityID, creds).
		Return(entity.ErrEntityNotFound).Once()

	err = suite.provider.UpdateSystemCredentials(testEntityID, creds)
	suite.NotNil(err)
	suite.Equal(ErrorCodeEntityNotFound, err.Code)

	// Test Invalid Credential
	suite.mockService.On("UpdateSystemCredentials", mock.Anything, testEntityID, creds).
		Return(entity.ErrInvalidCredential).Once()

	err = suite.provider.UpdateSystemCredentials(testEntityID, creds)
	suite.NotNil(err)
	suite.Equal(ErrorCodeInvalidRequestFormat, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestMapEntityError() {
	// Verifies the centralized error mapping helper.
	cases := []struct {
		name     string
		err      error
		expected ErrorCode
	}{
		{"EntityNotFound", entity.ErrEntityNotFound, ErrorCodeEntityNotFound},
		{"AttributeConflict", entity.ErrAttributeConflict, ErrorCodeAttributeConflict},
		{"SchemaValidationFailed", entity.ErrSchemaValidationFailed, ErrorCodeSchemaValidationFailed},
		{"InvalidCredential", entity.ErrInvalidCredential, ErrorCodeInvalidRequestFormat},
		{"BadAttributesInRequest", entity.ErrBadAttributesInRequest, ErrorCodeInvalidRequestFormat},
		{"Unknown", errors.New("unexpected"), ErrorCodeSystemError},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			result := mapEntityError(tc.err)
			suite.NotNil(result)
			suite.Equal(tc.expected, result.Code)
		})
	}
}

func (suite *DefaultEntityProviderTestSuite) TestGetTransitiveEntityGroups() {
	groups := []entity.EntityGroup{
		{ID: "g1", Name: "Group 1", OUID: "ou1"},
		{ID: "g2", Name: "Group 2", OUID: "ou1"},
	}

	// Test Success
	suite.mockService.On("GetTransitiveEntityGroups", mock.Anything, testEntityID).
		Return(groups, nil).Once()

	result, err := suite.provider.GetTransitiveEntityGroups(testEntityID)
	suite.Nil(err)
	suite.Len(result, 2)
	suite.Equal("g1", result[0].ID)

	// Test Not Found
	suite.mockService.On("GetTransitiveEntityGroups", mock.Anything, testEntityID).
		Return(nil, entity.ErrEntityNotFound).Once()

	result, err = suite.provider.GetTransitiveEntityGroups(testEntityID)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorCodeEntityNotFound, err.Code)

	// Test System Error
	suite.mockService.On("GetTransitiveEntityGroups", mock.Anything, testEntityID).
		Return(nil, errors.New("db error")).Once()

	result, err = suite.provider.GetTransitiveEntityGroups(testEntityID)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorCodeSystemError, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestValidateEntityIDs() {
	ids := []string{"id1", "id2"}

	// Test Success - all valid
	suite.mockService.On("ValidateEntityIDs", mock.Anything, ids).
		Return([]string{}, nil).Once()

	invalid, err := suite.provider.ValidateEntityIDs(ids)
	suite.Nil(err)
	suite.Empty(invalid)

	// Test Success - some invalid
	suite.mockService.On("ValidateEntityIDs", mock.Anything, ids).
		Return([]string{"id2"}, nil).Once()

	invalid, err = suite.provider.ValidateEntityIDs(ids)
	suite.Nil(err)
	suite.Equal([]string{"id2"}, invalid)

	// Test System Error
	suite.mockService.On("ValidateEntityIDs", mock.Anything, ids).
		Return(nil, errors.New("db error")).Once()

	invalid, err = suite.provider.ValidateEntityIDs(ids)
	suite.Nil(invalid)
	suite.NotNil(err)
	suite.Equal(ErrorCodeSystemError, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestGetEntitiesByIDs() {
	ids := []string{"id1"}
	entities := []entity.Entity{
		{ID: "id1", Category: entity.EntityCategoryUser, Type: "customer"},
	}

	// Test Success
	suite.mockService.On("GetEntitiesByIDs", mock.Anything, ids).Return(entities, nil).Once()

	result, err := suite.provider.GetEntitiesByIDs(ids)
	suite.Nil(err)
	suite.Len(result, 1)
	suite.Equal("id1", result[0].ID)

	// Test Not Found
	suite.mockService.On("GetEntitiesByIDs", mock.Anything, ids).
		Return(nil, entity.ErrEntityNotFound).Once()

	result, err = suite.provider.GetEntitiesByIDs(ids)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorCodeEntityNotFound, err.Code)

	// Test System Error
	suite.mockService.On("GetEntitiesByIDs", mock.Anything, ids).
		Return(nil, errors.New("db error")).Once()

	result, err = suite.provider.GetEntitiesByIDs(ids)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorCodeSystemError, err.Code)
}
