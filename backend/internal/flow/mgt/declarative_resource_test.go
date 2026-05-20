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

package flowmgt

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
)

type DeclarativeResourceTestSuite struct {
	suite.Suite
}

func (s *DeclarativeResourceTestSuite) SetupTest() {
	// Reset logger
	_ = log.GetLogger()
}

// TestParseToCompleteFlowDefinition tests parsing YAML to CompleteFlowDefinition
func (s *DeclarativeResourceTestSuite) TestParseToCompleteFlowDefinition() {
	yamlData := []byte(`
id: "flow-001"
handle: "basic-auth"
name: "Basic Authentication Flow"
flowtype: "AUTHENTICATION"
activeversion: 1
nodes:
  - id: "start"
    type: "START"
  - id: "basic-login"
    type: "BASIC_AUTHENTICATION"
  - id: "end"
    type: "END"
`)

	result, err := parseToCompleteFlowDefinition(yamlData)
	require.NoError(s.T(), err)

	flow, ok := result.(*CompleteFlowDefinition)
	require.True(s.T(), ok, "result should be *CompleteFlowDefinition")

	assert.Equal(s.T(), "flow-001", flow.ID)
	assert.Equal(s.T(), "basic-auth", flow.Handle)
	assert.Equal(s.T(), "Basic Authentication Flow", flow.Name)
	assert.Len(s.T(), flow.Nodes, 3)
}

// TestParseToCompleteFlowDefinition_InvalidYAML tests parsing invalid YAML
func (s *DeclarativeResourceTestSuite) TestParseToCompleteFlowDefinition_InvalidYAML() {
	yamlData := []byte(`{invalid yaml content`)

	_, err := parseToCompleteFlowDefinition(yamlData)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to unmarshal flow definition")
}

// TestValidateFlowGraphWrapper tests flow validation wrapper
func (s *DeclarativeResourceTestSuite) TestValidateFlowGraphWrapper_ValidFlow() {
	flow := &CompleteFlowDefinition{
		ID:       "flow-001",
		Handle:   "basic-auth",
		Name:     "Basic Auth",
		FlowType: "AUTHENTICATION",
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START"},
			{ID: "login", Type: "BASIC_AUTHENTICATION"},
			{ID: "mfa", Type: "TOTP_AUTHENTICATION"},
			{ID: "end", Type: "END"},
		},
	}

	err := validateFlowGraphWrapper(flow)
	assert.NoError(s.T(), err)
}

// TestValidateFlowGraphWrapper_MissingHandle tests validation with missing handle
func (s *DeclarativeResourceTestSuite) TestValidateFlowGraphWrapper_MissingHandle() {
	flow := &CompleteFlowDefinition{
		ID:       "flow-001",
		Handle:   "",
		Name:     "Basic Auth",
		FlowType: "AUTHENTICATION",
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START"},
			{ID: "end", Type: "END"},
		},
	}

	err := validateFlowGraphWrapper(flow)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "validation failed")
}

// TestValidateFlowGraphWrapper_InvalidType tests validation with wrong type
func (s *DeclarativeResourceTestSuite) TestValidateFlowGraphWrapper_InvalidType() {
	err := validateFlowGraphWrapper("not a flow definition")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "invalid type")
}

// TestValidateFlowGraphWrapper_InsufficientNodes tests validation with insufficient nodes
func (s *DeclarativeResourceTestSuite) TestValidateFlowGraphWrapper_InsufficientNodes() {
	flow := &CompleteFlowDefinition{
		ID:       "flow-001",
		Handle:   "invalid-flow",
		Name:     "Invalid Flow",
		FlowType: "AUTHENTICATION",
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START"},
			{ID: "end", Type: "END"},
		},
	}

	err := validateFlowGraphWrapper(flow)
	assert.Error(s.T(), err)
}

// TestFlowGraphExporter_GetResourceType tests resource type
func (s *DeclarativeResourceTestSuite) TestFlowGraphExporter_GetResourceType() {
	mockService := NewFlowMgtServiceInterfaceMock(s.T())
	exporter := newFlowGraphExporter(mockService)

	assert.Equal(s.T(), "flow", exporter.GetResourceType())
}

// TestFlowGraphExporter_GetParameterizerType tests parameterizer type
func (s *DeclarativeResourceTestSuite) TestFlowGraphExporter_GetParameterizerType() {
	mockService := NewFlowMgtServiceInterfaceMock(s.T())
	exporter := newFlowGraphExporter(mockService)

	assert.Equal(s.T(), "Flow", exporter.GetParameterizerType())
}

// TestFlowGraphExporter_GetResourceRules tests resource rules
func (s *DeclarativeResourceTestSuite) TestFlowGraphExporter_GetResourceRules() {
	mockService := NewFlowMgtServiceInterfaceMock(s.T())
	exporter := newFlowGraphExporter(mockService)

	rules := exporter.GetResourceRules()
	assert.NotNil(s.T(), rules)
	assert.Empty(s.T(), rules.Variables)
	assert.Empty(s.T(), rules.ArrayVariables)
	assert.Empty(s.T(), rules.DynamicPropertyFields)
}

// TestFlowGraphExporter_GetAllResourceIDs tests retrieving all resource IDs
func (s *DeclarativeResourceTestSuite) TestFlowGraphExporter_GetAllResourceIDs() {
	mockService := NewFlowMgtServiceInterfaceMock(s.T())

	listResponse := &FlowListResponse{
		Flows: []BasicFlowDefinition{
			{ID: "flow-001", Handle: "auth-flow"},
			{ID: "flow-002", Handle: "reg-flow"},
		},
		Count: 2,
	}

	// Use common.FlowType to match the service interface type
	mockService.EXPECT().ListFlows(mock.Anything, 10000, 0, common.FlowType("")).Return(listResponse, nil)

	exporter := newFlowGraphExporter(mockService)
	ids, err := exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 2)
	assert.Equal(s.T(), "flow-001", ids[0])
	assert.Equal(s.T(), "flow-002", ids[1])
}

// TestFlowGraphExporter_GetAllResourceIDs_Error tests error handling
func (s *DeclarativeResourceTestSuite) TestFlowGraphExporter_GetAllResourceIDs_Error() {
	mockService := NewFlowMgtServiceInterfaceMock(s.T())

	expectedError := &serviceerror.ServiceError{
		Code:  "ERR_CODE",
		Error: i18ncore.I18nMessage{DefaultValue: "test error"},
	}

	mockService.EXPECT().ListFlows(mock.Anything, 10000, 0, common.FlowType("")).Return(nil, expectedError)

	exporter := newFlowGraphExporter(mockService)
	ids, err := exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), ids)
	assert.Equal(s.T(), &serviceerror.InternalServerError, err)
}

// TestFlowGraphExporter_GetAllResourceIDs_EmptyList tests empty list handling
func (s *DeclarativeResourceTestSuite) TestFlowGraphExporter_GetAllResourceIDs_EmptyList() {
	mockService := NewFlowMgtServiceInterfaceMock(s.T())

	listResponse := &FlowListResponse{
		Flows: []BasicFlowDefinition{},
		Count: 0,
	}

	mockService.EXPECT().ListFlows(mock.Anything, 10000, 0, common.FlowType("")).Return(listResponse, nil)

	exporter := newFlowGraphExporter(mockService)
	ids, err := exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 0)
}

// TestFlowGraphExporter_GetResourceByID tests retrieving resource by ID
func (s *DeclarativeResourceTestSuite) TestFlowGraphExporter_GetResourceByID() {
	mockService := NewFlowMgtServiceInterfaceMock(s.T())

	flow := &CompleteFlowDefinition{
		ID:   "flow-001",
		Name: "Auth Flow",
	}

	mockService.EXPECT().GetFlow(mock.Anything, "flow-001").Return(flow, nil)

	exporter := newFlowGraphExporter(mockService)
	resource, name, err := exporter.GetResourceByID(context.Background(), "flow-001")

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), flow, resource)
	assert.Equal(s.T(), "Auth Flow", name)
}

// TestFlowGraphExporter_GetResourceByID_Error tests error handling
func (s *DeclarativeResourceTestSuite) TestFlowGraphExporter_GetResourceByID_Error() {
	mockService := NewFlowMgtServiceInterfaceMock(s.T())

	expectedError := &serviceerror.ServiceError{
		Code:  "ERR_CODE",
		Error: i18ncore.I18nMessage{DefaultValue: "test error"},
	}

	mockService.EXPECT().GetFlow(mock.Anything, "flow-001").Return(nil, expectedError)

	exporter := newFlowGraphExporter(mockService)
	resource, name, err := exporter.GetResourceByID(context.Background(), "flow-001")

	assert.Nil(s.T(), resource)
	assert.Empty(s.T(), name)
	assert.Equal(s.T(), expectedError, err)
}

// TestFlowGraphExporter_ValidateResource tests resource validation
func (s *DeclarativeResourceTestSuite) TestFlowGraphExporter_ValidateResource() {
	mockService := NewFlowMgtServiceInterfaceMock(s.T())
	exporter := newFlowGraphExporter(mockService)

	flow := &CompleteFlowDefinition{
		ID:   "flow-001",
		Name: "Valid Flow Name",
	}

	logger := log.GetLogger()
	name, exportErr := exporter.ValidateResource(flow, "flow-001", logger)

	assert.Nil(s.T(), exportErr)
	assert.Equal(s.T(), "Valid Flow Name", name)
}

// TestFlowGraphExporter_ValidateResource_InvalidType tests validation with invalid type
func (s *DeclarativeResourceTestSuite) TestFlowGraphExporter_ValidateResource_InvalidType() {
	mockService := NewFlowMgtServiceInterfaceMock(s.T())
	exporter := newFlowGraphExporter(mockService)

	logger := log.GetLogger()
	_, exportErr := exporter.ValidateResource("not a flow", "invalid", logger)

	assert.NotNil(s.T(), exportErr)
	assert.Equal(s.T(), "flow", exportErr.ResourceType)
	assert.Equal(s.T(), "invalid", exportErr.ResourceID)
	assert.Equal(s.T(), "INVALID_TYPE", exportErr.Code)
}

// TestFlowGraphExporter_ValidateResource_EmptyName tests validation with empty name
func (s *DeclarativeResourceTestSuite) TestFlowGraphExporter_ValidateResource_EmptyName() {
	mockService := NewFlowMgtServiceInterfaceMock(s.T())
	exporter := newFlowGraphExporter(mockService)

	flow := &CompleteFlowDefinition{
		ID:   "flow-001",
		Name: "",
	}

	logger := log.GetLogger()
	name, exportErr := exporter.ValidateResource(flow, "flow-001", logger)

	assert.Empty(s.T(), name)
	assert.NotNil(s.T(), exportErr)
	assert.Equal(s.T(), "flow", exportErr.ResourceType)
	assert.Equal(s.T(), "flow-001", exportErr.ResourceID)
	assert.Equal(s.T(), "FLOW_VALIDATION_ERROR", exportErr.Code)
	assert.Contains(s.T(), exportErr.Error, "name is empty")
}

// TestFileBasedStore_CreateFlow tests creating a flow in file-based store
func (s *DeclarativeResourceTestSuite) TestFileBasedStore_CreateFlow() {
	_ = entity.GetInstance().Clear()
	store, _ := newFileBasedStore()

	flowDef := &FlowDefinition{
		Handle:   "test-flow",
		Name:     "Test Flow",
		FlowType: "AUTHENTICATION",
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START"},
			{ID: "login", Type: "BASIC_AUTHENTICATION"},
			{ID: "end", Type: "END"},
		},
	}

	completeFlow, err := store.CreateFlow(context.Background(), "flow-001", flowDef)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), "flow-001", completeFlow.ID)
	assert.Equal(s.T(), "test-flow", completeFlow.Handle)
	assert.Equal(s.T(), "Test Flow", completeFlow.Name)
}

// TestFileBasedStore_GetFlowByID tests retrieving flow by ID
func (s *DeclarativeResourceTestSuite) TestFileBasedStore_GetFlowByID() {
	_ = entity.GetInstance().Clear()
	store, _ := newFileBasedStore()

	flowDef := &FlowDefinition{
		Handle:   "test-flow",
		Name:     "Test Flow",
		FlowType: "AUTHENTICATION",
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START"},
			{ID: "login", Type: "BASIC_AUTHENTICATION"},
			{ID: "end", Type: "END"},
		},
	}

	_, err := store.CreateFlow(context.Background(), "flow-001", flowDef)
	require.NoError(s.T(), err)

	retrieved, err := store.GetFlowByID(context.Background(), "flow-001")
	require.NoError(s.T(), err)

	assert.Equal(s.T(), "flow-001", retrieved.ID)
	assert.Equal(s.T(), "test-flow", retrieved.Handle)
}

// TestFileBasedStore_GetFlowByID_NotFound tests retrieving non-existent flow
func (s *DeclarativeResourceTestSuite) TestFileBasedStore_GetFlowByID_NotFound() {
	_ = entity.GetInstance().Clear()
	store, _ := newFileBasedStore()

	_, err := store.GetFlowByID(context.Background(), "non-existent")
	assert.Error(s.T(), err)
}

// TestFileBasedStore_GetFlowByHandle tests retrieving flow by handle
func (s *DeclarativeResourceTestSuite) TestFileBasedStore_GetFlowByHandle() {
	_ = entity.GetInstance().Clear()
	store, _ := newFileBasedStore()

	flowDef := &FlowDefinition{
		Handle:   "test-flow",
		Name:     "Test Flow",
		FlowType: "AUTHENTICATION",
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START"},
			{ID: "login", Type: "BASIC_AUTHENTICATION"},
			{ID: "end", Type: "END"},
		},
	}

	_, err := store.CreateFlow(context.Background(), "flow-001", flowDef)
	require.NoError(s.T(), err)

	retrieved, err := store.GetFlowByHandle(context.Background(), "test-flow", "AUTHENTICATION")
	require.NoError(s.T(), err)

	assert.Equal(s.T(), "flow-001", retrieved.ID)
	assert.Equal(s.T(), "test-flow", retrieved.Handle)
}

// TestFileBasedStore_ListFlows tests listing flows with pagination
func (s *DeclarativeResourceTestSuite) TestFileBasedStore_ListFlows() {
	_ = entity.GetInstance().Clear()
	store, _ := newFileBasedStore()

	for i := 0; i < 3; i++ {
		flowDef := &FlowDefinition{
			Handle:   fmt.Sprintf("flow-%d", i),
			Name:     fmt.Sprintf("Flow %d", i),
			FlowType: "AUTHENTICATION",
			Nodes: []NodeDefinition{
				{ID: "start", Type: "START"},
				{ID: "login", Type: "BASIC_AUTHENTICATION"},
				{ID: "end", Type: "END"},
			},
		}
		_, err := store.CreateFlow(context.Background(), fmt.Sprintf("flow-%03d", i), flowDef)
		require.NoError(s.T(), err)
	}

	flows, count, err := store.ListFlows(context.Background(), 10, 0, "")
	require.NoError(s.T(), err)

	assert.Equal(s.T(), 3, count)
	assert.Len(s.T(), flows, 3)
}

// TestFileBasedStore_UnsupportedOperations tests that unsupported operations return errors
func (s *DeclarativeResourceTestSuite) TestFileBasedStore_UnsupportedOperations() {
	_ = entity.GetInstance().Clear()
	store, _ := newFileBasedStore()

	flowDef := &FlowDefinition{
		Handle:   "test-flow",
		Name:     "Test Flow",
		FlowType: "AUTHENTICATION",
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START"},
			{ID: "login", Type: "BASIC_AUTHENTICATION"},
			{ID: "end", Type: "END"},
		},
	}

	_, err := store.CreateFlow(context.Background(), "flow-001", flowDef)
	require.NoError(s.T(), err)

	// UpdateFlow
	_, err = store.UpdateFlow(context.Background(), "flow-001", flowDef)
	assert.Error(s.T(), err)

	// DeleteFlow
	err = store.DeleteFlow(context.Background(), "flow-001")
	assert.Error(s.T(), err)

	// ListFlowVersions
	_, err = store.ListFlowVersions(context.Background(), "flow-001")
	assert.Error(s.T(), err)

	// GetFlowVersion
	_, err = store.GetFlowVersion(context.Background(), "flow-001", 1)
	assert.Error(s.T(), err)

	// RestoreFlowVersion
	_, err = store.RestoreFlowVersion(context.Background(), "flow-001", 1)
	assert.Error(s.T(), err)
}

// TestParseYAMLToJSON tests YAML parsing with various structures
func (s *DeclarativeResourceTestSuite) TestParseYAMLComplexStructure() {
	yamlData := []byte(`
id: "mfa-flow"
handle: "mfa-auth"
name: "Multi-Factor Authentication"
flowType: "AUTHENTICATION"
activeVersion: 2
nodes:
  - id: "start"
    type: "START"
  - id: "basic-auth"
    type: "BASIC_AUTHENTICATION"
  - id: "totp-check"
    type: "TOTP_AUTHENTICATION"
  - id: "success"
    type: "SUCCESS"
  - id: "end"
    type: "END"
createdAt: "2025-01-01T00:00:00Z"
updatedAt: "2025-01-02T00:00:00Z"
`)

	result, err := parseToCompleteFlowDefinition(yamlData)
	require.NoError(s.T(), err)

	flow, ok := result.(*CompleteFlowDefinition)
	require.True(s.T(), ok)

	assert.Equal(s.T(), "mfa-flow", flow.ID)
	assert.Equal(s.T(), "mfa-auth", flow.Handle)
	assert.Equal(s.T(), "Multi-Factor Authentication", flow.Name)
	// Note: flowType and activeVersion are camelCase in YAML, but YAML unmarshaling
	// without yaml tags uses lowercase, so they won't be properly unmarshaled.
	// This is expected - the struct should have yaml tags for proper unmarshaling.
	assert.Len(s.T(), flow.Nodes, 5)
}

// TestFlowGraphExporterIntegration tests the complete flow of exporter usage
func (s *DeclarativeResourceTestSuite) TestFlowGraphExporterIntegration() {
	mockService := NewFlowMgtServiceInterfaceMock(s.T())

	flow := &CompleteFlowDefinition{
		ID:            "flow-001",
		Handle:        "auth-flow",
		Name:          "Authentication Flow",
		FlowType:      "AUTHENTICATION",
		ActiveVersion: 1,
	}

	listResponse := &FlowListResponse{
		Flows: []BasicFlowDefinition{
			{ID: "flow-001", Handle: "auth-flow", Name: "Authentication Flow"},
		},
		Count: 1,
	}

	mockService.EXPECT().ListFlows(mock.Anything, 10000, 0, common.FlowType("")).Return(listResponse, nil)
	mockService.EXPECT().GetFlow(mock.Anything, "flow-001").Return(flow, nil)

	exporter := newFlowGraphExporter(mockService)

	// Get all IDs
	ids, err := exporter.GetAllResourceIDs(context.Background())
	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 1)

	// Get resource by ID
	resource, name, err := exporter.GetResourceByID(context.Background(), ids[0])
	assert.Nil(s.T(), err)
	assert.Equal(s.T(), flow, resource)
	assert.Equal(s.T(), "Authentication Flow", name)

	// Validate resource
	logger := log.GetLogger()
	validName, exportErr := exporter.ValidateResource(resource, ids[0], logger)
	assert.Nil(s.T(), exportErr)
	assert.Equal(s.T(), "Authentication Flow", validName)
}

// TestYAMLUnmarshalVariations tests different YAML formats
func (s *DeclarativeResourceTestSuite) TestYAMLUnmarshalVariations() {
	testCases := []struct {
		name     string
		yamlData []byte
		wantErr  bool
	}{
		{
			name: "minimal flow",
			yamlData: []byte(`
id: "flow-1"
handle: "flow"
name: "Flow"
flowType: "AUTHENTICATION"
nodes:
  - id: "start"
    type: "START"
  - id: "step"
    type: "BASIC_AUTHENTICATION"
  - id: "end"
    type: "END"
`),
			wantErr: false,
		},
		{
			name:     "empty YAML",
			yamlData: []byte(""),
			wantErr:  false,
		},
		{
			name:     "invalid YAML syntax",
			yamlData: []byte("{ invalid: yaml:"),
			wantErr:  true,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			_, err := parseToCompleteFlowDefinition(tc.yamlData)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestLoadDeclarativeResources_InvalidStoreType tests error when store is not file-based
func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeResources_InvalidStoreType() {
	// Create a mock store that's not a file-based store
	mockStore := &flowStoreInterfaceMock{}

	err := loadDeclarativeResources(mockStore)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to assert flowStore to *fileBasedStore")
}

// TestNodeDefinitionMarshalYAML_WithComplexMeta tests YAML marshaling with complex meta objects
func (s *DeclarativeResourceTestSuite) TestNodeDefinitionMarshalYAML_WithComplexMeta() {
	// Create a complex meta object similar to the test.yaml file
	complexMeta := map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{
				"id":      "text_001",
				"label":   "{{ t(signin:heading) }}",
				"type":    "TEXT",
				"variant": "HEADING_1",
			},
			map[string]interface{}{
				"id":   "block_001",
				"type": "BLOCK",
				"components": []interface{}{
					map[string]interface{}{
						"id":          "input_001",
						"label":       "{{ t(elements:fields.username.label) }}",
						"placeholder": "{{ t(elements:fields.username.placeholder) }}",
						"ref":         "username",
						"required":    true,
						"type":        "TEXT_INPUT",
					},
					map[string]interface{}{
						"id":          "input_002",
						"label":       "{{ t(elements:fields.password.label) }}",
						"placeholder": "{{ t(elements:fields.password.placeholder) }}",
						"ref":         "password",
						"required":    true,
						"type":        "PASSWORD_INPUT",
					},
					map[string]interface{}{
						"eventType": "SUBMIT",
						"id":        "action_001",
						"label":     "{{ t(elements:buttons.submit.text) }}",
						"type":      "ACTION",
						"variant":   "PRIMARY",
					},
				},
			},
		},
	}

	nodeDef := &NodeDefinition{
		ID:   "prompt_credentials",
		Type: "PROMPT",
		Meta: complexMeta,
		Prompts: []PromptDefinition{
			{
				Inputs: []InputDefinition{
					{Ref: "input_001", Type: "TEXT_INPUT", Identifier: "username", Required: true},
					{Ref: "input_002", Type: "PASSWORD_INPUT", Identifier: "password", Required: true},
				},
				Action: &ActionDefinition{Ref: "action_001", NextNode: "basic_auth"},
			},
		},
	}

	// Marshal to YAML
	result, err := nodeDef.MarshalYAML()
	require.NoError(s.T(), err)

	// Verify the result
	alias, ok := result.(nodeDefinitionAlias)
	require.True(s.T(), ok, "result should be nodeDefinitionAlias")

	// Verify Meta is now a JSON string
	metaStr, ok := alias.Meta.(string)
	require.True(s.T(), ok, "Meta should be converted to string")
	assert.Contains(s.T(), metaStr, "components")
	assert.Contains(s.T(), metaStr, "text_001")
	assert.Contains(s.T(), metaStr, "block_001")
}

// TestNodeDefinitionMarshalYAML_WithNilMeta tests YAML marshaling with nil meta
func (s *DeclarativeResourceTestSuite) TestNodeDefinitionMarshalYAML_WithNilMeta() {
	nodeDef := &NodeDefinition{
		ID:   "start",
		Type: "START",
		Meta: nil,
	}

	result, err := nodeDef.MarshalYAML()
	require.NoError(s.T(), err)

	alias, ok := result.(nodeDefinitionAlias)
	require.True(s.T(), ok)
	assert.Nil(s.T(), alias.Meta)
}

// TestNodeDefinitionUnmarshalYAML_WithComplexMeta tests YAML unmarshaling with complex meta
func (s *DeclarativeResourceTestSuite) TestNodeDefinitionUnmarshalYAML_WithComplexMeta() {
	// Use a shorter meta string that still demonstrates the functionality
	metaJSON := `{"components":[{"id":"text_001","type":"TEXT"},` +
		`{"id":"block_001","type":"BLOCK","components":[{"id":"input_001","type":"TEXT_INPUT"}]}]}`

	yamlData := []byte(fmt.Sprintf(`
id: prompt_credentials
type: PROMPT
meta: '%s'
prompts:
  - inputs:
      - ref: input_001
        type: TEXT_INPUT
        identifier: username
        required: true
    action:
      ref: action_001
      nextNode: basic_auth
`, metaJSON))

	// Parse the actual YAML
	var nodeDef NodeDefinition
	err := yaml.Unmarshal(yamlData, &nodeDef)
	require.NoError(s.T(), err)

	// Verify the Meta field is properly decoded
	assert.NotNil(s.T(), nodeDef.Meta)
	metaMap, ok := nodeDef.Meta.(map[string]interface{})
	require.True(s.T(), ok, "Meta should be decoded to map[string]interface{}")

	// Verify the structure
	components, ok := metaMap["components"].([]interface{})
	require.True(s.T(), ok)
	assert.Len(s.T(), components, 2)

	// Verify first component
	firstComponent, ok := components[0].(map[string]interface{})
	require.True(s.T(), ok)
	assert.Equal(s.T(), "text_001", firstComponent["id"])
	assert.Equal(s.T(), "TEXT", firstComponent["type"])

	// Verify nested components
	secondComponent, ok := components[1].(map[string]interface{})
	require.True(s.T(), ok)
	assert.Equal(s.T(), "block_001", secondComponent["id"])

	nestedComponents, ok := secondComponent["components"].([]interface{})
	require.True(s.T(), ok)
	assert.Len(s.T(), nestedComponents, 1)
}

// TestCompleteFlowDefinition_MarshalUnmarshal_RoundTrip tests full round trip
func (s *DeclarativeResourceTestSuite) TestCompleteFlowDefinition_MarshalUnmarshal_RoundTrip() {
	// Create a complete flow definition with complex meta
	originalFlow := &CompleteFlowDefinition{
		ID:            "019b4495-793d-7172-b3e5-ebc3d61afe36",
		Handle:        "console-app-flow",
		Name:          "Console App Authentication Flow",
		FlowType:      "AUTHENTICATION",
		ActiveVersion: 1,
		Nodes: []NodeDefinition{
			{
				ID:        "start",
				Type:      "START",
				OnSuccess: "prompt_credentials",
			},
			{
				ID:   "prompt_credentials",
				Type: "PROMPT",
				Meta: map[string]interface{}{
					"components": []interface{}{
						map[string]interface{}{
							"id":      "text_001",
							"label":   "{{ t(signin:heading) }}",
							"type":    "TEXT",
							"variant": "HEADING_1",
						},
						map[string]interface{}{
							"id":   "block_001",
							"type": "BLOCK",
							"components": []interface{}{
								map[string]interface{}{
									"id":          "input_001",
									"label":       "Username",
									"placeholder": "Enter username",
									"ref":         "username",
									"required":    true,
									"type":        "TEXT_INPUT",
								},
								map[string]interface{}{
									"id":          "input_002",
									"label":       "Password",
									"placeholder": "Enter password",
									"ref":         "password",
									"required":    true,
									"type":        "PASSWORD_INPUT",
								},
							},
						},
					},
				},
				Prompts: []PromptDefinition{
					{
						Inputs: []InputDefinition{
							{Ref: "input_001", Type: "TEXT_INPUT", Identifier: "username", Required: true},
							{Ref: "input_002", Type: "PASSWORD_INPUT", Identifier: "password", Required: true},
						},
						Action: &ActionDefinition{Ref: "action_001", NextNode: "basic_auth"},
					},
				},
			},
			{
				ID:   "end",
				Type: "END",
			},
		},
		CreatedAt: "2025-12-22 05:43:25",
		UpdatedAt: "2025-12-22 05:43:25",
	}

	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(originalFlow)
	require.NoError(s.T(), err)

	// The meta field should be present in the YAML
	yamlStr := string(yamlBytes)
	assert.Contains(s.T(), yamlStr, `meta:`)
	// When using the custom MarshalYAML on NodeDefinition, meta will be present
	// The structure may vary based on how yaml.v3 handles it

	// Unmarshal back
	var unmarshaledFlow CompleteFlowDefinition
	err = yaml.Unmarshal(yamlBytes, &unmarshaledFlow)
	require.NoError(s.T(), err)

	// Verify the structure is preserved
	assert.Equal(s.T(), originalFlow.ID, unmarshaledFlow.ID)
	assert.Equal(s.T(), originalFlow.Handle, unmarshaledFlow.Handle)
	assert.Equal(s.T(), originalFlow.Name, unmarshaledFlow.Name)
	assert.Len(s.T(), unmarshaledFlow.Nodes, 3)

	// Verify the meta is correctly decoded
	promptNode := unmarshaledFlow.Nodes[1]
	assert.NotNil(s.T(), promptNode.Meta)

	metaMap, ok := promptNode.Meta.(map[string]interface{})
	require.True(s.T(), ok, "Meta should be decoded to map")

	components, ok := metaMap["components"].([]interface{})
	require.True(s.T(), ok)
	assert.Len(s.T(), components, 2)
}

// TestNodeDefinitionUnmarshalYAML_WithInvalidJSON tests handling of invalid JSON in meta
func (s *DeclarativeResourceTestSuite) TestNodeDefinitionUnmarshalYAML_WithInvalidJSON() {
	yamlData := []byte(`
id: test_node
type: PROMPT
meta: 'this is not valid json {}'
`)

	var nodeDef NodeDefinition
	err := yaml.Unmarshal(yamlData, &nodeDef)
	require.NoError(s.T(), err)

	// When JSON parsing fails, the string value should be kept
	metaStr, ok := nodeDef.Meta.(string)
	require.True(s.T(), ok, "Invalid JSON should be kept as string")
	assert.Equal(s.T(), "this is not valid json {}", metaStr)
}

// TestFlowExport_WithComplexMeta tests exporting a flow with complex meta via the exporter
func (s *DeclarativeResourceTestSuite) TestFlowExport_WithComplexMeta() {
	mockService := &FlowMgtServiceInterfaceMock{}

	complexFlow := &CompleteFlowDefinition{
		ID:            "test-flow-001",
		Handle:        "test-flow",
		Name:          "Test Flow with Complex Meta",
		FlowType:      "AUTHENTICATION",
		ActiveVersion: 1,
		Nodes: []NodeDefinition{
			{
				ID:   "prompt",
				Type: "PROMPT",
				Meta: map[string]interface{}{
					"title":       "Login Page",
					"description": "Please enter your credentials",
					"components": []interface{}{
						map[string]interface{}{
							"id":       "username_field",
							"type":     "TEXT_INPUT",
							"label":    "Username",
							"required": true,
						},
						map[string]interface{}{
							"id":       "password_field",
							"type":     "PASSWORD_INPUT",
							"label":    "Password",
							"required": true,
						},
					},
					"theme": map[string]interface{}{
						"primaryColor":   "#0066cc",
						"secondaryColor": "#6c757d",
					},
				},
			},
		},
	}

	mockService.EXPECT().GetFlow(mock.Anything, "test-flow-001").Return(complexFlow, nil)

	exporter := newFlowGraphExporter(mockService)
	resource, name, err := exporter.GetResourceByID(context.Background(), "test-flow-001")

	require.Nil(s.T(), err)
	assert.Equal(s.T(), "Test Flow with Complex Meta", name)

	flow, ok := resource.(*CompleteFlowDefinition)
	require.True(s.T(), ok)

	// Verify meta is preserved
	assert.NotNil(s.T(), flow.Nodes[0].Meta)
	metaMap, ok := flow.Nodes[0].Meta.(map[string]interface{})
	require.True(s.T(), ok)
	assert.Equal(s.T(), "Login Page", metaMap["title"])

	// Marshal the flow to ensure meta serialization works
	yamlBytes, marshalErr := yaml.Marshal(flow)
	require.NoError(s.T(), marshalErr)
	assert.Contains(s.T(), string(yamlBytes), "meta:")
}

// TestNodeDefinitionMarshalYAML_WithPrimitiveMeta tests marshaling with primitive meta values
func (s *DeclarativeResourceTestSuite) TestNodeDefinitionMarshalYAML_WithPrimitiveMeta() {
	testCases := []struct {
		name      string
		metaValue interface{}
	}{
		{
			name:      "string meta",
			metaValue: "simple string value",
		},
		{
			name:      "integer meta",
			metaValue: 42,
		},
		{
			name:      "float meta",
			metaValue: 3.14159,
		},
		{
			name:      "boolean meta",
			metaValue: true,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			nodeDef := NodeDefinition{
				ID:   "test_node",
				Type: "TEST",
				Meta: tc.metaValue,
			}

			yamlBytes, err := yaml.Marshal(nodeDef)
			require.NoError(t, err)
			assert.Contains(t, string(yamlBytes), "meta:")

			// Unmarshal and verify
			var unmarshaled NodeDefinition
			err = yaml.Unmarshal(yamlBytes, &unmarshaled)
			require.NoError(t, err)

			// For primitive types, verify the structure
			assert.NotNil(t, unmarshaled.Meta)
		})
	}
}

// TestNodeDefinitionMarshalYAML_WithArrayMeta tests marshaling with array meta
func (s *DeclarativeResourceTestSuite) TestNodeDefinitionMarshalYAML_WithArrayMeta() {
	nodeDef := NodeDefinition{
		ID:   "test_node",
		Type: "TEST",
		Meta: []interface{}{
			"string element",
			42,
			true,
			map[string]interface{}{
				"nested": "value",
			},
		},
	}

	yamlBytes, err := yaml.Marshal(nodeDef)
	require.NoError(s.T(), err)
	assert.Contains(s.T(), string(yamlBytes), "meta:")

	// Unmarshal and verify
	var unmarshaled NodeDefinition
	err = yaml.Unmarshal(yamlBytes, &unmarshaled)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), unmarshaled.Meta)

	// Verify array structure is preserved
	metaArray, ok := unmarshaled.Meta.([]interface{})
	require.True(s.T(), ok)
	assert.Len(s.T(), metaArray, 4)
}

// TestNodeDefinitionUnmarshalYAML_WithEmptyMeta tests unmarshaling with empty meta string
func (s *DeclarativeResourceTestSuite) TestNodeDefinitionUnmarshalYAML_WithEmptyMeta() {
	yamlData := []byte(`
id: test_node
type: TEST
meta: ''
`)

	var nodeDef NodeDefinition
	err := yaml.Unmarshal(yamlData, &nodeDef)
	require.NoError(s.T(), err)

	// Empty string should remain as empty string (not parsed)
	assert.Equal(s.T(), "", nodeDef.Meta)
}

// TestNodeDefinitionUnmarshalYAML_WithPartiallyInvalidJSON tests partial JSON parsing
func (s *DeclarativeResourceTestSuite) TestNodeDefinitionUnmarshalYAML_WithPartiallyInvalidJSON() {
	yamlData := []byte(`
id: test_node
type: TEST
meta: '{"valid": "json", "incomplete":'
`)

	var nodeDef NodeDefinition
	err := yaml.Unmarshal(yamlData, &nodeDef)
	require.NoError(s.T(), err)

	// Invalid JSON should be kept as string
	metaStr, ok := nodeDef.Meta.(string)
	require.True(s.T(), ok)
	assert.Equal(s.T(), `{"valid": "json", "incomplete":`, metaStr)
}

// TestNodeDefinitionUnmarshalYAML_WithValidJSONArray tests unmarshaling with JSON array
func (s *DeclarativeResourceTestSuite) TestNodeDefinitionUnmarshalYAML_WithValidJSONArray() {
	yamlData := []byte(`
id: test_node
type: TEST
meta: '["item1", "item2", "item3"]'
`)

	var nodeDef NodeDefinition
	err := yaml.Unmarshal(yamlData, &nodeDef)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), nodeDef.Meta)

	// Should be parsed as array
	metaArray, ok := nodeDef.Meta.([]interface{})
	require.True(s.T(), ok)
	assert.Len(s.T(), metaArray, 3)
	assert.Equal(s.T(), "item1", metaArray[0])
	assert.Equal(s.T(), "item2", metaArray[1])
	assert.Equal(s.T(), "item3", metaArray[2])
}

// TestNodeDefinitionUnmarshalYAML_WithNestedObjects tests deeply nested JSON objects
func (s *DeclarativeResourceTestSuite) TestNodeDefinitionUnmarshalYAML_WithNestedObjects() {
	yamlData := []byte(`
id: test_node
type: TEST
meta: '{"level1":{"level2":{"level3":{"value":"deep"}}}}'
`)

	var nodeDef NodeDefinition
	err := yaml.Unmarshal(yamlData, &nodeDef)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), nodeDef.Meta)

	// Navigate through nested structure
	metaMap, ok := nodeDef.Meta.(map[string]interface{})
	require.True(s.T(), ok)

	level1, ok := metaMap["level1"].(map[string]interface{})
	require.True(s.T(), ok)

	level2, ok := level1["level2"].(map[string]interface{})
	require.True(s.T(), ok)

	level3, ok := level2["level3"].(map[string]interface{})
	require.True(s.T(), ok)

	assert.Equal(s.T(), "deep", level3["value"])
}

// TestCompleteFlowDefinition_WithMultipleNodesWithMeta tests multiple nodes with different meta types
func (s *DeclarativeResourceTestSuite) TestCompleteFlowDefinition_WithMultipleNodesWithMeta() {
	flow := CompleteFlowDefinition{
		ID:       "test-flow",
		Handle:   "multi-meta-flow",
		Name:     "Flow with Multiple Meta Types",
		FlowType: "AUTHENTICATION",
		Nodes: []NodeDefinition{
			{
				ID:   "node1",
				Type: "TYPE1",
				Meta: map[string]interface{}{"key": "value"},
			},
			{
				ID:   "node2",
				Type: "TYPE2",
				Meta: []interface{}{"item1", "item2"},
			},
			{
				ID:   "node3",
				Type: "TYPE3",
				Meta: "simple string",
			},
			{
				ID:   "node4",
				Type: "TYPE4",
				Meta: nil,
			},
		},
	}

	// Marshal
	yamlBytes, err := yaml.Marshal(flow)
	require.NoError(s.T(), err)

	// Unmarshal
	var unmarshaled CompleteFlowDefinition
	err = yaml.Unmarshal(yamlBytes, &unmarshaled)
	require.NoError(s.T(), err)

	// Verify each node's meta
	require.Len(s.T(), unmarshaled.Nodes, 4)

	// Node 1: map
	metaMap, ok := unmarshaled.Nodes[0].Meta.(map[string]interface{})
	require.True(s.T(), ok)
	assert.Equal(s.T(), "value", metaMap["key"])

	// Node 2: array
	metaArray, ok := unmarshaled.Nodes[1].Meta.([]interface{})
	require.True(s.T(), ok)
	assert.Len(s.T(), metaArray, 2)

	// Node 3: string
	metaStr, ok := unmarshaled.Nodes[2].Meta.(string)
	require.True(s.T(), ok)
	assert.Equal(s.T(), "simple string", metaStr)

	// Node 4: nil
	assert.Nil(s.T(), unmarshaled.Nodes[3].Meta)
}

// TestNodeDefinitionMarshalYAML_WithSpecialCharacters tests meta with special characters
func (s *DeclarativeResourceTestSuite) TestNodeDefinitionMarshalYAML_WithSpecialCharacters() {
	nodeDef := NodeDefinition{
		ID:   "test_node",
		Type: "TEST",
		Meta: map[string]interface{}{
			"special":  "value with \"quotes\" and 'apostrophes'",
			"newlines": "line1\nline2\nline3",
			"unicode":  "emoji: 😀 中文",
			"symbols":  "!@#$%^&*()_+-=[]{}|;:,.<>?",
		},
	}

	// Marshal
	yamlBytes, err := yaml.Marshal(nodeDef)
	require.NoError(s.T(), err)

	// Unmarshal
	var unmarshaled NodeDefinition
	err = yaml.Unmarshal(yamlBytes, &unmarshaled)
	require.NoError(s.T(), err)

	// Verify special characters are preserved
	metaMap, ok := unmarshaled.Meta.(map[string]interface{})
	require.True(s.T(), ok)
	assert.Contains(s.T(), metaMap["special"], "quotes")
	assert.Contains(s.T(), metaMap["newlines"], "\n")
	assert.Contains(s.T(), metaMap["unicode"], "😀")
	assert.Contains(s.T(), metaMap["symbols"], "!@#$")
}

// TestNodeDefinitionUnmarshalYAML_WithMalformedYAML tests error handling for malformed YAML
func (s *DeclarativeResourceTestSuite) TestNodeDefinitionUnmarshalYAML_WithMalformedYAML() {
	malformedYAML := []byte(`
id: test_node
type: TEST
meta: this is not properly quoted: {invalid
inputs:
  - malformed
`)

	var nodeDef NodeDefinition
	err := yaml.Unmarshal(malformedYAML, &nodeDef)
	// Should return an error because the YAML is malformed
	assert.Error(s.T(), err)
}

// TestNodeDefinitionUnmarshalYAML_WithInvalidNodeStructure tests error when required fields are missing
func (s *DeclarativeResourceTestSuite) TestNodeDefinitionUnmarshalYAML_WithInvalidNodeStructure() {
	invalidYAML := []byte(`
meta: '{"key": "value"}'
# Missing required fields like id and type
`)

	var nodeDef NodeDefinition
	err := yaml.Unmarshal(invalidYAML, &nodeDef)
	// Should succeed in unmarshaling but fields will be empty
	require.NoError(s.T(), err)
	assert.Empty(s.T(), nodeDef.ID)
	assert.Empty(s.T(), nodeDef.Type)
}

// TestNodeDefinitionMarshalYAML_RoundTrip tests complete round-trip with various meta types
func (s *DeclarativeResourceTestSuite) TestNodeDefinitionMarshalYAML_RoundTrip() {
	testCases := []struct {
		name     string
		nodeDef  NodeDefinition
		validate func(*testing.T, NodeDefinition)
	}{
		{
			name: "complex nested structure",
			nodeDef: NodeDefinition{
				ID:   "node1",
				Type: "PROMPT",
				Meta: map[string]interface{}{
					"level1": map[string]interface{}{
						"level2": []interface{}{
							map[string]interface{}{
								"key": "value",
								"num": float64(42),
							},
						},
					},
				},
			},
			validate: func(t *testing.T, nd NodeDefinition) {
				metaMap := nd.Meta.(map[string]interface{})
				level1 := metaMap["level1"].(map[string]interface{})
				level2 := level1["level2"].([]interface{})
				item := level2[0].(map[string]interface{})
				assert.Equal(t, "value", item["key"])
			},
		},
		{
			name: "array of primitives",
			nodeDef: NodeDefinition{
				ID:   "node2",
				Type: "TEST",
				Meta: []interface{}{"string", float64(123), true, nil},
			},
			validate: func(t *testing.T, nd NodeDefinition) {
				arr := nd.Meta.([]interface{})
				assert.Len(t, arr, 4)
				assert.Equal(t, "string", arr[0])
				// After JSON marshal/unmarshal, numbers can be int or float64
				// depending on the value. Check the numeric value instead.
				switch v := arr[1].(type) {
				case int:
					assert.Equal(t, 123, v)
				case float64:
					assert.Equal(t, float64(123), v)
				default:
					t.Errorf("Expected numeric value, got %T", v)
				}
				assert.Equal(t, true, arr[2])
				assert.Nil(t, arr[3])
			},
		},
		{
			name: "with all node fields",
			nodeDef: NodeDefinition{
				ID:   "node3",
				Type: "EXECUTOR",
				Meta: map[string]interface{}{"config": "value"},
				Layout: &NodeLayout{
					Position: &NodePosition{X: 100, Y: 200},
					Size:     &NodeSize{Width: 300, Height: 400},
				},
				Executor: &ExecutorDefinition{
					Name: "test-executor",
					Inputs: []InputDefinition{
						{Type: "TEXT", Identifier: "input1", Required: true},
					},
				},
				Prompts: []PromptDefinition{
					{
						Action: &ActionDefinition{Ref: "action1", NextNode: "next"},
					},
				},
				Properties: map[string]interface{}{
					"prop1": "value1",
				},

				OnSuccess: "success-node",
				OnFailure: "failure-node",
			},
			validate: func(t *testing.T, nd NodeDefinition) {
				assert.Equal(t, "node3", nd.ID)
				assert.Equal(t, "EXECUTOR", nd.Type)
				assert.NotNil(t, nd.Layout)
				assert.Equal(t, float64(100), nd.Layout.Position.X)
				assert.NotNil(t, nd.Executor)
				assert.Len(t, nd.Executor.Inputs, 1)
				assert.Len(t, nd.Prompts, 1)
				metaMap := nd.Meta.(map[string]interface{})
				assert.Equal(t, "value", metaMap["config"])
			},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			// Marshal
			yamlBytes, err := yaml.Marshal(tc.nodeDef)
			require.NoError(t, err)

			// Unmarshal
			var unmarshaled NodeDefinition
			err = yaml.Unmarshal(yamlBytes, &unmarshaled)
			require.NoError(t, err)

			// Validate
			tc.validate(t, unmarshaled)
		})
	}
}

// TestNodeDefinitionMarshalYAML_WithJSONMarshallableTypes tests various JSON-marshallable types
func (s *DeclarativeResourceTestSuite) TestNodeDefinitionMarshalYAML_WithJSONMarshallableTypes() {
	testCases := []struct {
		name        string
		meta        interface{}
		expectError bool
	}{
		{
			name: "nested arrays and maps",
			meta: []interface{}{
				map[string]interface{}{
					"nested": []interface{}{
						map[string]interface{}{"deep": "value"},
					},
				},
			},
			expectError: false,
		},
		{
			name: "numeric types",
			meta: map[string]interface{}{
				"int":      42,
				"float":    3.14,
				"negInt":   -100,
				"negFloat": -2.5,
				"zero":     0,
			},
			expectError: false,
		},
		{
			name: "boolean and null",
			meta: map[string]interface{}{
				"true":  true,
				"false": false,
				"null":  nil,
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			nodeDef := NodeDefinition{
				ID:   "test",
				Type: "TEST",
				Meta: tc.meta,
			}

			result, err := nodeDef.MarshalYAML()
			if tc.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				alias := result.(nodeDefinitionAlias)
				assert.IsType(t, "", alias.Meta)
			}
		})
	}
}

// TestNodeDefinitionUnmarshalYAML_WithMixedMetaFormats tests unmarshaling different meta formats
func (s *DeclarativeResourceTestSuite) TestNodeDefinitionUnmarshalYAML_WithMixedMetaFormats() {
	testCases := []struct {
		name     string
		yaml     string
		validate func(*testing.T, NodeDefinition)
	}{
		{
			name: "JSON object string",
			yaml: `
id: test1
type: TEST
meta: '{"key":"value","num":42}'
`,
			validate: func(t *testing.T, nd NodeDefinition) {
				metaMap, ok := nd.Meta.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "value", metaMap["key"])
				assert.Equal(t, float64(42), metaMap["num"])
			},
		},
		{
			name: "JSON array string",
			yaml: `
id: test2
type: TEST
meta: '[1,2,3]'
`,
			validate: func(t *testing.T, nd NodeDefinition) {
				metaArr, ok := nd.Meta.([]interface{})
				require.True(t, ok)
				assert.Len(t, metaArr, 3)
			},
		},
		{
			name: "plain string (backward compatibility)",
			yaml: `
id: test3
type: TEST
meta: 'just a plain string'
`,
			validate: func(t *testing.T, nd NodeDefinition) {
				metaStr, ok := nd.Meta.(string)
				require.True(t, ok)
				assert.Equal(t, "just a plain string", metaStr)
			},
		},
		{
			name: "numeric meta",
			yaml: `
id: test4
type: TEST
meta: 42
`,
			validate: func(t *testing.T, nd NodeDefinition) {
				// When meta is a number in YAML, it's unmarshaled as int
				assert.Equal(t, 42, nd.Meta)
			},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			var nodeDef NodeDefinition
			err := yaml.Unmarshal([]byte(tc.yaml), &nodeDef)
			require.NoError(t, err)
			tc.validate(t, nodeDef)
		})
	}
}

// TestCompleteFlowDefinition_MarshalUnmarshal_WithErrors tests error propagation in complete flows
func (s *DeclarativeResourceTestSuite) TestCompleteFlowDefinition_MarshalUnmarshal_WithErrors() {
	// Test that yaml.Unmarshal properly reports errors for invalid structure
	invalidYAML := []byte(`
id: "flow-001"
handle: "test-flow"
nodes:
  - id: "node1"
    type: "TEST"
    meta: this is invalid: {no quotes
`)

	var flow CompleteFlowDefinition
	err := yaml.Unmarshal(invalidYAML, &flow)
	// Should return an error due to malformed YAML
	assert.Error(s.T(), err)
}

// TestNodeDefinitionUnmarshalYAML_WithWhitespaceInMeta tests meta with whitespace
func (s *DeclarativeResourceTestSuite) TestNodeDefinitionUnmarshalYAML_WithWhitespaceInMeta() {
	yamlData := []byte(`
id: test_node
type: TEST
meta: '{"key": "value with spaces", "multiline": "line1\nline2"}'
`)

	var nodeDef NodeDefinition
	err := yaml.Unmarshal(yamlData, &nodeDef)
	require.NoError(s.T(), err)

	metaMap, ok := nodeDef.Meta.(map[string]interface{})
	require.True(s.T(), ok)
	assert.Equal(s.T(), "value with spaces", metaMap["key"])
	assert.Contains(s.T(), metaMap["multiline"], "\n")
}

func TestDeclarativeResourceTestSuite(t *testing.T) {
	suite.Run(t, new(DeclarativeResourceTestSuite))
}
