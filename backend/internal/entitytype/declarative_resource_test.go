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

package entitytype_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entitytype"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
)

// EntityTypeExporterTestSuite tests the entityTypeExporter.
type EntityTypeExporterTestSuite struct {
	suite.Suite
	mockService *entitytypemock.EntityTypeServiceInterfaceMock
	exporter    declarativeresource.ResourceExporter
	logger      *log.Logger
}

func TestEntityTypeExporterTestSuite(t *testing.T) {
	suite.Run(t, new(EntityTypeExporterTestSuite))
}

func (s *EntityTypeExporterTestSuite) SetupTest() {
	s.mockService = entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	s.exporter = entitytype.NewEntityTypeExporterForTest(s.mockService)
	s.logger = log.GetLogger()
}

func (s *EntityTypeExporterTestSuite) TestNewEntityTypeExporter() {
	assert.NotNil(s.T(), s.exporter)
}

func (s *EntityTypeExporterTestSuite) TestGetResourceType() {
	assert.Equal(s.T(), "user_type", s.exporter.GetResourceType())
}

func (s *EntityTypeExporterTestSuite) TestGetParameterizerType() {
	assert.Equal(s.T(), "EntityType", s.exporter.GetParameterizerType())
}

func (s *EntityTypeExporterTestSuite) TestGetAllResourceIDs_Success() {
	expectedResponse := &entitytype.EntityTypeListResponse{
		Types: []entitytype.EntityTypeListItem{
			{ID: "schema1", Name: "Schema 1"},
			{ID: "schema2", Name: "Schema 2"},
		},
	}

	s.mockService.EXPECT().
		GetEntityTypeList(mock.Anything, entitytype.TypeCategoryUser, 100, 0, false).
		Return(expectedResponse, nil)

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 2)
	assert.Equal(s.T(), "schema1", ids[0])
	assert.Equal(s.T(), "schema2", ids[1])
}

func (s *EntityTypeExporterTestSuite) TestGetAllResourceIDs_Error() {
	expectedError := &serviceerror.ServiceError{
		Code: "ERR_CODE",
		Error: core.I18nMessage{
			Key:          "error.entitytypeexporter.test_error",
			DefaultValue: "test error",
		},
	}

	s.mockService.EXPECT().
		GetEntityTypeList(mock.Anything, entitytype.TypeCategoryUser, 100, 0, false).
		Return(nil, expectedError)

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), ids)
	assert.Equal(s.T(), expectedError, err)
}

func (s *EntityTypeExporterTestSuite) TestGetAllResourceIDs_EmptyList() {
	expectedResponse := &entitytype.EntityTypeListResponse{
		Types: []entitytype.EntityTypeListItem{},
	}

	s.mockService.EXPECT().
		GetEntityTypeList(mock.Anything, entitytype.TypeCategoryUser, 100, 0, false).
		Return(expectedResponse, nil)

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 0)
}

func (s *EntityTypeExporterTestSuite) TestGetResourceByID_Success() {
	expectedSchema := &entitytype.EntityType{
		ID:   "schema1",
		Name: "Test Schema",
	}

	s.mockService.EXPECT().
		GetEntityType(mock.Anything, entitytype.TypeCategoryUser, "schema1", mock.Anything).
		Return(expectedSchema, nil)

	resource, name, err := s.exporter.GetResourceByID(context.Background(), "schema1")

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "Test Schema", name)
	assert.Equal(s.T(), expectedSchema, resource)
}

func (s *EntityTypeExporterTestSuite) TestGetResourceByID_Error() {
	expectedError := &serviceerror.ServiceError{
		Code: "ERR_CODE",
		Error: core.I18nMessage{
			Key:          "error.entitytypeexporter.test_error",
			DefaultValue: "test error",
		},
	}

	s.mockService.EXPECT().
		GetEntityType(mock.Anything, entitytype.TypeCategoryUser, "schema1", mock.Anything).
		Return(nil, expectedError)

	resource, name, err := s.exporter.GetResourceByID(context.Background(), "schema1")

	assert.Nil(s.T(), resource)
	assert.Empty(s.T(), name)
	assert.Equal(s.T(), expectedError, err)
}

func (s *EntityTypeExporterTestSuite) TestValidateResource_Success() {
	schema := &entitytype.EntityType{
		ID:     "schema1",
		Name:   "Valid Schema",
		Schema: json.RawMessage(`{"field": "value"}`),
	}

	name, err := s.exporter.ValidateResource(schema, "schema1", s.logger)

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "Valid Schema", name)
}

func (s *EntityTypeExporterTestSuite) TestValidateResource_InvalidType() {
	invalidResource := "not a schema"

	name, err := s.exporter.ValidateResource(invalidResource, "schema1", s.logger)

	assert.Empty(s.T(), name)
	assert.NotNil(s.T(), err)
	assert.Equal(s.T(), "user_type", err.ResourceType)
	assert.Equal(s.T(), "schema1", err.ResourceID)
	assert.Equal(s.T(), "INVALID_TYPE", err.Code)
}

func (s *EntityTypeExporterTestSuite) TestValidateResource_EmptyName() {
	schema := &entitytype.EntityType{
		ID:   "schema1",
		Name: "",
	}

	name, err := s.exporter.ValidateResource(schema, "schema1", s.logger)

	assert.Empty(s.T(), name)
	assert.NotNil(s.T(), err)
	assert.Equal(s.T(), "user_type", err.ResourceType)
	assert.Equal(s.T(), "schema1", err.ResourceID)
	assert.Equal(s.T(), "SCHEMA_VALIDATION_ERROR", err.Code)
	assert.Contains(s.T(), err.Error, "name is empty")
}

func (s *EntityTypeExporterTestSuite) TestValidateResource_NoSchema() {
	schema := &entitytype.EntityType{
		ID:     "schema1",
		Name:   "Test Schema",
		Schema: json.RawMessage(`{}`),
	}

	name, err := s.exporter.ValidateResource(schema, "schema1", s.logger)

	// Should still succeed but log a warning
	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "Test Schema", name)
}

func (s *EntityTypeExporterTestSuite) TestGetResourceRules() {
	rules := s.exporter.GetResourceRules()

	assert.NotNil(s.T(), rules)
	// ResourceRules is currently an empty struct, but the function should return a valid pointer
	assert.IsType(s.T(), &declarativeresource.ResourceRules{}, rules)
}
