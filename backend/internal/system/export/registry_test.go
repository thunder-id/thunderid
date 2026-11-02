// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package export

import (
	"context"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
)

type RegistryTestSuite struct {
	suite.Suite
	registry *resourceExporterRegistry
}

func TestRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryTestSuite))
}

func (s *RegistryTestSuite) SetupTest() {
	s.registry = newResourceExporterRegistry()
}

func (s *RegistryTestSuite) TestNewRegistry_Empty() {
	assert.NotNil(s.T(), s.registry)
	assert.Empty(s.T(), s.registry.exporters)
}

func (s *RegistryTestSuite) TestRegisterAndGet_Success() {
	// Create a mock exporter
	mockExporter := &mockResourceExporter{resourceType: "test"}

	// Register
	s.registry.Register(mockExporter)

	// Get
	exporter, exists := s.registry.Get("test")
	assert.True(s.T(), exists)
	assert.Equal(s.T(), mockExporter, exporter)
}

func (s *RegistryTestSuite) TestGet_NotFound() {
	exporter, exists := s.registry.Get("nonexistent")
	assert.False(s.T(), exists)
	assert.Nil(s.T(), exporter)
}

func (s *RegistryTestSuite) TestRegisterMultiple_GetAll() {
	// Register multiple exporters
	mock1 := &mockResourceExporter{resourceType: "type1"}
	mock2 := &mockResourceExporter{resourceType: "type2"}

	s.registry.Register(mock1)
	s.registry.Register(mock2)

	// Get all
	all := s.registry.GetAll()
	assert.Len(s.T(), all, 2)
	assert.Equal(s.T(), mock1, all["type1"])
	assert.Equal(s.T(), mock2, all["type2"])
}

func (s *RegistryTestSuite) TestRegister_Overwrite() {
	// Register first exporter
	mock1 := &mockResourceExporter{resourceType: "test", name: "first"}
	s.registry.Register(mock1)

	// Register second exporter with same type
	mock2 := &mockResourceExporter{resourceType: "test", name: "second"}
	s.registry.Register(mock2)

	// Should get the second one
	exporter, exists := s.registry.Get("test")
	assert.True(s.T(), exists)
	assert.Equal(s.T(), "second", exporter.(*mockResourceExporter).name)
}

// mockResourceExporter is a simple mock for testing registry functionality
type mockResourceExporter struct {
	resourceType      string
	paramType         string
	name              string
	getAllIDs         []string
	getAllErr         *tidcommon.ServiceError
	resourceByID      interface{}
	resourceNameByID  string
	getByIDErr        *tidcommon.ServiceError
	validateName      string
	validateExportErr *ExportError
	rules             *declarativeresource.ResourceRules
}

func (m *mockResourceExporter) GetResourceType() string {
	return m.resourceType
}

func (m *mockResourceExporter) GetParameterizerType() string {
	return m.paramType
}

func (m *mockResourceExporter) GetAllResourceIDs(ctx context.Context) ([]string, *tidcommon.ServiceError) {
	return m.getAllIDs, m.getAllErr
}

func (m *mockResourceExporter) GetResourceByID(
	ctx context.Context, id string,
) (interface{}, string, *tidcommon.ServiceError) {
	return m.resourceByID, m.resourceNameByID, m.getByIDErr
}

func (m *mockResourceExporter) ValidateResource(
	_ context.Context, resource interface{}, id string, logger *log.Logger,
) (string, *ExportError) {
	return m.validateName, m.validateExportErr
}

func (m *mockResourceExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return m.rules
}
