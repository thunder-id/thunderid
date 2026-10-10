// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entitytype_test

import (
	"context"
	"encoding/json"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entitytype"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
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
	s.exporter = entitytype.NewEntityTypeExporterForTest(s.mockService, entitytype.TypeCategoryUser)
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
			{ID: "schema1", Handle: "schema-1", DisplayName: "Schema 1"},
			{ID: "schema2", Handle: "schema-2", DisplayName: "Schema 2"},
		},
	}

	s.mockService.EXPECT().
		GetEntityTypeList(mock.Anything, entitytype.TypeCategoryUser, 100, 0, false).
		Return(expectedResponse, nil).Once()
	s.mockService.EXPECT().
		GetEntityTypeList(mock.Anything, entitytype.TypeCategoryUser, 100, 2, false).
		Return(&entitytype.EntityTypeListResponse{Types: []entitytype.EntityTypeListItem{}}, nil).Once()

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 2)
	assert.Equal(s.T(), "schema1", ids[0])
	assert.Equal(s.T(), "schema2", ids[1])
}

func (s *EntityTypeExporterTestSuite) TestGetAllResourceIDs_Error() {
	expectedError := &tidcommon.ServiceError{
		Code: "ERR_CODE",
		Error: tidcommon.I18nMessage{
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
		ID:          "schema1",
		Handle:      "test-schema",
		DisplayName: "Test Schema",
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
	expectedError := &tidcommon.ServiceError{
		Code: "ERR_CODE",
		Error: tidcommon.I18nMessage{
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
		ID:          "schema1",
		Handle:      "valid-schema",
		DisplayName: "Valid Schema",
		Schema:      json.RawMessage(`{"field": "value"}`),
	}

	name, err := s.exporter.ValidateResource(context.Background(), schema, "schema1", s.logger)

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "Valid Schema", name)
}

func (s *EntityTypeExporterTestSuite) TestValidateResource_InvalidType() {
	invalidResource := "not a schema"

	name, err := s.exporter.ValidateResource(context.Background(), invalidResource, "schema1", s.logger)

	assert.Empty(s.T(), name)
	assert.NotNil(s.T(), err)
	assert.Equal(s.T(), "user_type", err.ResourceType)
	assert.Equal(s.T(), "schema1", err.ResourceID)
	assert.Equal(s.T(), "INVALID_TYPE", err.Code)
}

func (s *EntityTypeExporterTestSuite) TestValidateResource_EmptyName() {
	schema := &entitytype.EntityType{
		ID:          "schema1",
		Handle:      "",
		DisplayName: "",
	}

	name, err := s.exporter.ValidateResource(context.Background(), schema, "schema1", s.logger)

	assert.Empty(s.T(), name)
	assert.NotNil(s.T(), err)
	assert.Equal(s.T(), "user_type", err.ResourceType)
	assert.Equal(s.T(), "schema1", err.ResourceID)
	assert.Equal(s.T(), "SCHEMA_VALIDATION_ERROR", err.Code)
	assert.Contains(s.T(), err.Error, "name is empty")
}

func (s *EntityTypeExporterTestSuite) TestValidateResource_NoSchema() {
	schema := &entitytype.EntityType{
		ID:          "schema1",
		Handle:      "test-schema",
		DisplayName: "Test Schema",
		Schema:      json.RawMessage(`{}`),
	}

	name, err := s.exporter.ValidateResource(context.Background(), schema, "schema1", s.logger)

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

// AgentTypeExporterTestSuite tests the entityTypeExporter configured for TypeCategoryAgent.
type AgentTypeExporterTestSuite struct {
	suite.Suite
	mockService *entitytypemock.EntityTypeServiceInterfaceMock
	exporter    declarativeresource.ResourceExporter
	logger      *log.Logger
}

func TestAgentTypeExporterTestSuite(t *testing.T) {
	suite.Run(t, new(AgentTypeExporterTestSuite))
}

func (s *AgentTypeExporterTestSuite) SetupTest() {
	s.mockService = entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	s.exporter = entitytype.NewEntityTypeExporterForTest(s.mockService, entitytype.TypeCategoryAgent)
	s.logger = log.GetLogger()
}

func (s *AgentTypeExporterTestSuite) TestGetResourceType() {
	assert.Equal(s.T(), "agent_type", s.exporter.GetResourceType())
}

func (s *AgentTypeExporterTestSuite) TestGetParameterizerType() {
	assert.Equal(s.T(), "AgentType", s.exporter.GetParameterizerType())
}

func (s *AgentTypeExporterTestSuite) TestGetAllResourceIDs_Success() {
	expectedResponse := &entitytype.EntityTypeListResponse{
		Types: []entitytype.EntityTypeListItem{
			{ID: "agenttype1", Handle: "agent-type-1", DisplayName: "Agent Type 1"},
			{ID: "agenttype2", Handle: "agent-type-2", DisplayName: "Agent Type 2"},
		},
	}

	s.mockService.EXPECT().
		GetEntityTypeList(mock.Anything, entitytype.TypeCategoryAgent, 100, 0, false).
		Return(expectedResponse, nil).Once()
	s.mockService.EXPECT().
		GetEntityTypeList(mock.Anything, entitytype.TypeCategoryAgent, 100, 2, false).
		Return(&entitytype.EntityTypeListResponse{Types: []entitytype.EntityTypeListItem{}}, nil).Once()

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 2)
	assert.ElementsMatch(s.T(), []string{"agenttype1", "agenttype2"}, ids)
}

func (s *AgentTypeExporterTestSuite) TestGetAllResourceIDs_Error() {
	expectedError := &tidcommon.ServiceError{
		Code: "ERR_CODE",
		Error: tidcommon.I18nMessage{
			Key:          "error.entitytypeexporter.test_error",
			DefaultValue: "test error",
		},
	}

	s.mockService.EXPECT().
		GetEntityTypeList(mock.Anything, entitytype.TypeCategoryAgent, 100, 0, false).
		Return(nil, expectedError)

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), ids)
	assert.Equal(s.T(), expectedError, err)
}

func (s *AgentTypeExporterTestSuite) TestGetAllResourceIDs_EmptyList() {
	expectedResponse := &entitytype.EntityTypeListResponse{
		Types: []entitytype.EntityTypeListItem{},
	}

	s.mockService.EXPECT().
		GetEntityTypeList(mock.Anything, entitytype.TypeCategoryAgent, 100, 0, false).
		Return(expectedResponse, nil)

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 0)
}

func (s *AgentTypeExporterTestSuite) TestGetAllResourceIDs_Pagination() {
	firstPage := &entitytype.EntityTypeListResponse{
		Types: []entitytype.EntityTypeListItem{
			{ID: "agenttype1", Handle: "agent-type-1", DisplayName: "Agent Type 1"},
			{ID: "agenttype2", Handle: "agent-type-2", DisplayName: "Agent Type 2"},
		},
	}
	secondPage := &entitytype.EntityTypeListResponse{Types: []entitytype.EntityTypeListItem{}}

	s.mockService.EXPECT().
		GetEntityTypeList(mock.Anything, entitytype.TypeCategoryAgent, 100, 0, false).
		Return(firstPage, nil).Once()
	s.mockService.EXPECT().
		GetEntityTypeList(mock.Anything, entitytype.TypeCategoryAgent, 100, 2, false).
		Return(secondPage, nil).Once()

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 2)
	assert.ElementsMatch(s.T(), []string{"agenttype1", "agenttype2"}, ids)
}

func (s *AgentTypeExporterTestSuite) TestGetAllResourceIDs_FiltersReadOnly() {
	response := &entitytype.EntityTypeListResponse{
		Types: []entitytype.EntityTypeListItem{
			{ID: "agenttype-db", IsReadOnly: false},
			{ID: "agenttype-decl", IsReadOnly: true},
		},
	}

	s.mockService.EXPECT().
		GetEntityTypeList(mock.Anything, entitytype.TypeCategoryAgent, 100, 0, false).
		Return(response, nil).Once()
	s.mockService.EXPECT().
		GetEntityTypeList(mock.Anything, entitytype.TypeCategoryAgent, 100, 2, false).
		Return(&entitytype.EntityTypeListResponse{Types: []entitytype.EntityTypeListItem{}}, nil).Once()

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 1)
	assert.Equal(s.T(), "agenttype-db", ids[0])
}

func (s *AgentTypeExporterTestSuite) TestGetResourceByID_Success() {
	expectedSchema := &entitytype.EntityType{
		ID:          "agenttype1",
		Handle:      "test-agent-type",
		DisplayName: "Test Agent Type",
	}

	s.mockService.EXPECT().
		GetEntityType(mock.Anything, entitytype.TypeCategoryAgent, "agenttype1", mock.Anything).
		Return(expectedSchema, nil)

	resource, name, err := s.exporter.GetResourceByID(context.Background(), "agenttype1")

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "Test Agent Type", name)
	assert.Equal(s.T(), expectedSchema, resource)
}

func (s *AgentTypeExporterTestSuite) TestGetResourceByID_Error() {
	expectedError := &tidcommon.ServiceError{
		Code: "ERR_CODE",
		Error: tidcommon.I18nMessage{
			Key:          "error.entitytypeexporter.test_error",
			DefaultValue: "test error",
		},
	}

	s.mockService.EXPECT().
		GetEntityType(mock.Anything, entitytype.TypeCategoryAgent, "agenttype1", mock.Anything).
		Return(nil, expectedError)

	resource, name, err := s.exporter.GetResourceByID(context.Background(), "agenttype1")

	assert.Nil(s.T(), resource)
	assert.Empty(s.T(), name)
	assert.Equal(s.T(), expectedError, err)
}

func (s *AgentTypeExporterTestSuite) TestValidateResource_InvalidType() {
	invalidResource := "not a schema"

	name, err := s.exporter.ValidateResource(context.Background(), invalidResource, "agenttype1", s.logger)

	assert.Empty(s.T(), name)
	assert.NotNil(s.T(), err)
	assert.Equal(s.T(), "agent_type", err.ResourceType)
	assert.Equal(s.T(), "agenttype1", err.ResourceID)
	assert.Equal(s.T(), "INVALID_TYPE", err.Code)
}

func (s *AgentTypeExporterTestSuite) TestValidateResource_EmptyName() {
	schema := &entitytype.EntityType{
		ID:          "agenttype1",
		Handle:      "",
		DisplayName: "",
	}

	name, err := s.exporter.ValidateResource(context.Background(), schema, "agenttype1", s.logger)

	assert.Empty(s.T(), name)
	assert.NotNil(s.T(), err)
	assert.Equal(s.T(), "agent_type", err.ResourceType)
	assert.Equal(s.T(), "agenttype1", err.ResourceID)
	assert.Equal(s.T(), "SCHEMA_VALIDATION_ERROR", err.Code)
	assert.Contains(s.T(), err.Error, "name is empty")
}
