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

package export

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

type RegistryTestSuite struct {
	suite.Suite
	registry *ResourceExporterRegistry
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
	getAllErr         *serviceerror.ServiceError
	resourceByID      interface{}
	resourceNameByID  string
	getByIDErr        *serviceerror.ServiceError
	validateName      string
	validateExportErr *ExportError
}

func (m *mockResourceExporter) GetResourceType() string {
	return m.resourceType
}

func (m *mockResourceExporter) GetParameterizerType() string {
	return m.paramType
}

func (m *mockResourceExporter) GetAllResourceIDs(ctx context.Context) ([]string, *serviceerror.ServiceError) {
	return m.getAllIDs, m.getAllErr
}

func (m *mockResourceExporter) GetResourceByID(
	ctx context.Context, id string,
) (interface{}, string, *serviceerror.ServiceError) {
	return m.resourceByID, m.resourceNameByID, m.getByIDErr
}

func (m *mockResourceExporter) ValidateResource(
	resource interface{}, id string, logger *log.Logger,
) (string, *ExportError) {
	return m.validateName, m.validateExportErr
}

func (m *mockResourceExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return &declarativeresource.ResourceRules{}
}
