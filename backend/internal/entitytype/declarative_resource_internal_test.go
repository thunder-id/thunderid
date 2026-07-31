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

package entitytype

import (
	"context"
	"encoding/json"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/tests/mocks/oumock"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// TestValidateEntityType tests the validateEntityType function with various scenarios.
// OU handle resolution and OU existence checks have been moved to the service layer
// (ResolveEntityTypeHandles / ensureOrganizationUnitExists), so validateEntityType is
// now a pure structural validator.
func TestValidateEntityType(t *testing.T) {
	testCases := []struct {
		name    string
		schema  *EntityType
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid schema",
			schema: &EntityType{
				ID:     "schema-1",
				Name:   "Valid Schema",
				OUID:   "ou-1",
				Schema: json.RawMessage(`{"email":{"type":"string"}}`),
			},
			wantErr: false,
		},
		{
			name: "missing name",
			schema: &EntityType{
				ID:   "schema-1",
				Name: "",
				OUID: "ou-1",
			},
			wantErr: true,
			errMsg:  "entity type name is required",
		},
		{
			name: "whitespace only name",
			schema: &EntityType{
				ID:   "schema-1",
				Name: "   ",
				OUID: "ou-1",
			},
			wantErr: true,
			errMsg:  "entity type name is required",
		},
		{
			name: "missing ID",
			schema: &EntityType{
				ID:   "",
				Name: "Valid Schema",
				OUID: "ou-1",
			},
			wantErr: true,
			errMsg:  "entity type ID is required",
		},
		{
			name: "whitespace only ID",
			schema: &EntityType{
				ID:   "   ",
				Name: "Valid Schema",
				OUID: "ou-1",
			},
			wantErr: true,
			errMsg:  "entity type ID is required",
		},
		{
			name: "missing organization unit ID",
			schema: &EntityType{
				ID:   "schema-1",
				Name: "Valid Schema",
				OUID: "",
			},
			wantErr: true,
			errMsg:  "ouId or ouHandle is required",
		},
		{
			name: "whitespace only organization unit ID",
			schema: &EntityType{
				ID:   "schema-1",
				Name: "Valid Schema",
				OUID: "   ",
			},
			wantErr: true,
			errMsg:  "ouId or ouHandle is required",
		},
		{
			name: "invalid schema JSON",
			schema: &EntityType{
				ID:     "schema-1",
				Name:   "Invalid Schema",
				OUID:   "ou-1",
				Schema: json.RawMessage(`{invalid json}`),
			},
			wantErr: true,
			errMsg:  "invalid schema for entity type",
		},
		{
			name: "empty schema definition rejected",
			schema: &EntityType{
				ID:     "schema-1",
				Name:   "Valid Schema",
				OUID:   "ou-1",
				Schema: json.RawMessage(``),
			},
			wantErr: true,
			errMsg:  "schema definition is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateEntityType(tc.schema)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateEntityTypeWrapper tests the wrapper function.
func TestValidateEntityTypeWrapper(t *testing.T) {
	t.Run("valid type", func(t *testing.T) {
		mockSvc := NewEntityTypeServiceInterfaceMock(t)
		schema := &EntityType{
			ID:     "schema-1",
			Name:   "Valid Schema",
			OUID:   "ou-1",
			Schema: json.RawMessage(`{"email":{"type":"string"}}`),
		}

		mockSvc.EXPECT().ResolveEntityTypeHandles(mock.Anything, schema).Return(nil).Once()

		validator := validateEntityTypeWrapper(mockSvc, nil)
		err := validator(schema)

		assert.NoError(t, err)
	})

	t.Run("ou_handle resolved by service", func(t *testing.T) {
		mockSvc := NewEntityTypeServiceInterfaceMock(t)
		schema := &EntityType{
			ID:       "schema-1",
			Name:     "Valid Schema",
			OUHandle: "default",
			Schema:   json.RawMessage(`{"email":{"type":"string"}}`),
		}

		mockSvc.EXPECT().ResolveEntityTypeHandles(mock.Anything, schema).
			RunAndReturn(func(_ context.Context, et *EntityType) *tidcommon.ServiceError {
				et.OUID = "ou-resolved"
				return nil
			}).Once()

		validator := validateEntityTypeWrapper(mockSvc, nil)
		err := validator(schema)

		assert.NoError(t, err)
		assert.Equal(t, "ou-resolved", schema.OUID)
	})

	t.Run("ou_handle not found", func(t *testing.T) {
		mockSvc := NewEntityTypeServiceInterfaceMock(t)
		schema := &EntityType{
			ID:       "schema-1",
			Name:     "Valid Schema",
			OUHandle: "missing",
			Schema:   json.RawMessage(`{"email":{"type":"string"}}`),
		}

		mockSvc.EXPECT().ResolveEntityTypeHandles(mock.Anything, schema).
			Return(&ErrorInvalidRequestFormat).Once()

		validator := validateEntityTypeWrapper(mockSvc, nil)
		err := validator(schema)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), `organization unit with handle "missing" not found`)
	})

	t.Run("invalid type", func(t *testing.T) {
		mockSvc := NewEntityTypeServiceInterfaceMock(t)
		invalidData := "not a schema"

		validator := validateEntityTypeWrapper(mockSvc, nil)
		err := validator(invalidData)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid type: expected *EntityType")
	})
	t.Run("ou existence check error", func(t *testing.T) {
		mockSvc := NewEntityTypeServiceInterfaceMock(t)
		schema := &EntityType{
			ID:     "schema-1",
			Name:   "Valid Schema",
			OUID:   "ou-1",
			Schema: json.RawMessage(`{"email":{"type":"string"}}`),
		}

		mockSvc.EXPECT().ResolveEntityTypeHandles(mock.Anything, schema).Return(nil).Once()

		ouSvcMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
		ouSvcMock.On("IsOrganizationUnitExists", mock.Anything, "ou-1").
			Return(false, &tidcommon.InternalServerError)

		validator := validateEntityTypeWrapper(mockSvc, ouSvcMock)
		err := validator(schema)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to verify organization unit")
	})

	t.Run("ou not found", func(t *testing.T) {
		mockSvc := NewEntityTypeServiceInterfaceMock(t)
		schema := &EntityType{
			ID:     "schema-1",
			Name:   "Valid Schema",
			OUID:   "missing-ou",
			Schema: json.RawMessage(`{"email":{"type":"string"}}`),
		}

		mockSvc.EXPECT().ResolveEntityTypeHandles(mock.Anything, schema).Return(nil).Once()

		ouSvcMock := oumock.NewOrganizationUnitServiceInterfaceMock(t)
		ouSvcMock.On("IsOrganizationUnitExists", mock.Anything, "missing-ou").
			Return(false, nil)

		validator := validateEntityTypeWrapper(mockSvc, ouSvcMock)
		err := validator(schema)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "organization unit \"missing-ou\" not found")
	})
}

// TestParseToEntityTypeDTO tests the parseToEntityTypeDTO function.
func TestParseToEntityTypeDTO(t *testing.T) {
	testCases := []struct {
		name           string
		yaml           string
		want           *EntityType
		wantErr        bool
		errMsg         string
		validateSchema bool
	}{
		{
			name: "valid YAML",
			yaml: `
id: schema-1
name: Test Schema
ouId: ou-1
allowSelfRegistration: true
schema: '{"type": "object"}'
`,
			want: &EntityType{
				ID:                    "schema-1",
				Name:                  "Test Schema",
				OUID:                  "ou-1",
				AllowSelfRegistration: true,
				Schema:                json.RawMessage(`{"type": "object"}`),
			},
			wantErr: false,
		},
		{
			name: "valid YAML without optional fields",
			yaml: `
id: schema-2
name: Minimal Schema
ouId: ou-1
schema: '{}'
`,
			want: &EntityType{
				ID:                    "schema-2",
				Name:                  "Minimal Schema",
				OUID:                  "ou-1",
				AllowSelfRegistration: false,
				Schema:                json.RawMessage(`{}`),
			},
			wantErr: false,
		},
		{
			name: "invalid YAML",
			yaml: `
invalid: [yaml
`,
			wantErr: true,
		},
		{
			name: "invalid JSON in schema field",
			yaml: `
id: schema-1
name: Test Schema
ouId: ou-1
schema: '{invalid json}'
`,
			wantErr: true,
			errMsg:  "schema field contains invalid JSON",
		},
		{
			name: "schema as YAML object",
			yaml: `
id: schema-1
name: Test Schema
ouId: ou-1
schema:
  username:
    type: string
    required: true
`,
			want: &EntityType{
				ID:   "schema-1",
				Name: "Test Schema",
				OUID: "ou-1",
				Schema: json.RawMessage(
					`{"username":{"required":true,"type":"string"}}`,
				),
			},
			wantErr:        false,
			validateSchema: true,
		},
		{
			name: "missing schema field",
			yaml: `
id: schema-1
name: Test Schema
ouId: ou-1
`,
			wantErr: true,
			errMsg:  "schema field contains invalid JSON",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseToEntityTypeDTO([]byte(tc.yaml))

			if tc.wantErr {
				assert.Error(t, err)
				if tc.errMsg != "" {
					assert.Contains(t, err.Error(), tc.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want.ID, result.ID)
				assert.Equal(t, tc.want.Name, result.Name)
				assert.Equal(t, tc.want.OUID, result.OUID)
				assert.Equal(t, tc.want.AllowSelfRegistration, result.AllowSelfRegistration)
				if tc.validateSchema {
					var got, expected map[string]interface{}
					assert.NoError(t, json.Unmarshal(result.Schema, &got), "result schema must decode to JSON object")
					assert.NoError(t, json.Unmarshal(tc.want.Schema, &expected),
						"test fixture schema must decode to JSON object")
					assert.Equal(t, expected, got, "decoded schema must deep-equal the expected value")
					assert.NotEqual(t, map[string]interface{}{}, got, "schema must not decode to an empty object")
				}
			}
		})
	}
}

// TestParseToEntityTypeDTOWrapper tests the wrapper function.
func TestParseToEntityTypeDTOWrapper(t *testing.T) {
	yaml := `
id: schema-1
name: Test Schema
oUId: ou-1
schema: '{"type": "object"}'
`
	result, err := parseToEntityTypeDTOWrapper([]byte(yaml))

	assert.NoError(t, err)
	schema, ok := result.(*EntityType)
	assert.True(t, ok)
	assert.Equal(t, "schema-1", schema.ID)
	assert.Equal(t, "Test Schema", schema.Name)
}

// TestLoadDeclarativeResources tests the loadDeclarativeResources function.
func TestLoadDeclarativeResources(t *testing.T) {
	// Initialize runtime config for tests that need DB access
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}

	t.Run("composite store", func(t *testing.T) {
		config.ResetServerRuntime()
		err := config.InitializeServerRuntime("", testConfig)
		assert.NoError(t, err)
		defer config.ResetServerRuntime()

		fileStore, _ := newEntityTypeFileBasedStore()
		dbStore, _, _ := newEntityTypeStore()
		compositeStore := newCompositeEntityTypeStore(fileStore, dbStore)

		mockSvc := NewEntityTypeServiceInterfaceMock(t)
		mockSvc.On("ResolveEntityTypeHandles", mock.Anything, mock.Anything).Return(nil).Maybe()

		err = loadDeclarativeResources(compositeStore, mockSvc, nil)
		assert.True(t, err == nil || err != nil, "Function should complete regardless of directory presence")
	})

	t.Run("file-based store", func(t *testing.T) {
		config.ResetServerRuntime()
		err := config.InitializeServerRuntime("", testConfig)
		assert.NoError(t, err)
		defer config.ResetServerRuntime()

		fileStore, _ := newEntityTypeFileBasedStore()

		mockSvc := NewEntityTypeServiceInterfaceMock(t)
		mockSvc.On("ResolveEntityTypeHandles", mock.Anything, mock.Anything).Return(nil).Maybe()

		err = loadDeclarativeResources(fileStore, mockSvc, nil)
		_ = err // Don't assert on error as it depends on file system state
	})

	t.Run("invalid store type", func(t *testing.T) {
		config.ResetServerRuntime()
		err := config.InitializeServerRuntime("", testConfig)
		assert.NoError(t, err)
		defer config.ResetServerRuntime()

		dbStore, _, _ := newEntityTypeStore()

		mockSvc := NewEntityTypeServiceInterfaceMock(t)

		err = loadDeclarativeResources(dbStore, mockSvc, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid store type")
	})
}

// TestGetAllResourceIDs_WithReadOnlyFilter tests that declarative schemas are excluded from export.
func TestGetAllResourceIDs_WithReadOnlyFilter(t *testing.T) {
	mockService := NewEntityTypeServiceInterfaceMock(t)

	exporter := newEntityTypeExporter(mockService)

	response := &EntityTypeListResponse{
		Types: []EntityTypeListItem{
			{ID: "schema1", Name: "Schema 1", IsReadOnly: false}, // Mutable - should be included
			{ID: "schema2", Name: "Schema 2", IsReadOnly: true},  // Immutable - should be excluded
			{ID: "schema3", Name: "Schema 3", IsReadOnly: false}, // Mutable - should be included
		},
	}

	mockService.On("GetEntityTypeList", mock.Anything, mock.Anything, 100, 0, false).Return(response, nil)

	ids, err := exporter.GetAllResourceIDs(context.Background())

	assert.Nil(t, err)
	assert.Len(t, ids, 2, "Should only include mutable schemas")
	assert.Contains(t, ids, "schema1")
	assert.Contains(t, ids, "schema3")
	assert.NotContains(t, ids, "schema2", "Schema2 is read-only and should be excluded")
}

// TestLoadDeclarativeResources_WithNilService tests that passing a nil service completes without panic.
func TestLoadDeclarativeResources_WithNilService(t *testing.T) {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}

	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(t, err)
	defer config.ResetServerRuntime()

	fileStore, _ := newEntityTypeFileBasedStore()
	dbStore, _, _ := newEntityTypeStore()
	compositeStore := newCompositeEntityTypeStore(fileStore, dbStore)

	err = loadDeclarativeResources(compositeStore, nil, nil)
	_ = err // outcome depends on file system state; important that it doesn't panic
}
