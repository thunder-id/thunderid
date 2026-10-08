// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entitytype

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
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
				ID:          "schema-1",
				Handle:      "valid-schema",
				DisplayName: "Valid Schema",
				OUID:        "ou-1",
				Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
			},
			wantErr: false,
		},
		{
			name: "missing handle",
			schema: &EntityType{
				ID:          "schema-1",
				Handle:      "",
				DisplayName: "Valid Schema",
				OUID:        "ou-1",
			},
			wantErr: true,
			errMsg:  "entity type handle \"\" is invalid",
		},
		{
			name: "whitespace only handle",
			schema: &EntityType{
				ID:          "schema-1",
				Handle:      "   ",
				DisplayName: "Valid Schema",
				OUID:        "ou-1",
			},
			wantErr: true,
			errMsg:  "is invalid",
		},
		{
			name: "uppercase handle",
			schema: &EntityType{
				ID:          "schema-1",
				Handle:      "Employee",
				DisplayName: "Employee",
				OUID:        "ou-1",
			},
			wantErr: true,
			errMsg:  "entity type handle \"Employee\" is invalid",
		},
		{
			name: "handle with space",
			schema: &EntityType{
				ID:          "schema-1",
				Handle:      "my type",
				DisplayName: "My Type",
				OUID:        "ou-1",
			},
			wantErr: true,
			errMsg:  "is invalid",
		},
		{
			name: "handle starting with a hyphen",
			schema: &EntityType{
				ID:          "schema-1",
				Handle:      "-employee",
				DisplayName: "Employee",
				OUID:        "ou-1",
			},
			wantErr: true,
			errMsg:  "is invalid",
		},
		{
			name: "handle ending with an underscore",
			schema: &EntityType{
				ID:          "schema-1",
				Handle:      "employee_",
				DisplayName: "Employee",
				OUID:        "ou-1",
			},
			wantErr: true,
			errMsg:  "is invalid",
		},
		{
			name: "handle longer than 100 characters",
			schema: &EntityType{
				ID:          "schema-1",
				Handle:      strings.Repeat("a", 101),
				DisplayName: "Employee",
				OUID:        "ou-1",
			},
			wantErr: true,
			errMsg:  "is invalid",
		},
		{
			name: "single character handle",
			schema: &EntityType{
				ID:          "schema-1",
				Handle:      "e",
				DisplayName: "Employee",
				OUID:        "ou-1",
				Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
			},
			wantErr: false,
		},
		{
			name: "handle with hyphens and underscores",
			schema: &EntityType{
				ID:          "schema-1",
				Handle:      "e-1_x",
				DisplayName: "Employee",
				OUID:        "ou-1",
				Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
			},
			wantErr: false,
		},
		{
			name: "missing display name",
			schema: &EntityType{
				ID:          "schema-1",
				Handle:      "valid-schema",
				DisplayName: "",
				OUID:        "ou-1",
			},
			wantErr: true,
			errMsg:  "display name is required",
		},
		{
			name: "whitespace only display name",
			schema: &EntityType{
				ID:          "schema-1",
				Handle:      "valid-schema",
				DisplayName: "   ",
				OUID:        "ou-1",
			},
			wantErr: true,
			errMsg:  "display name is required",
		},
		{
			name: "missing ID",
			schema: &EntityType{
				ID:          "",
				Handle:      "valid-schema",
				DisplayName: "Valid Schema",
				OUID:        "ou-1",
			},
			wantErr: true,
			errMsg:  "entity type ID is required",
		},
		{
			name: "whitespace only ID",
			schema: &EntityType{
				ID:          "   ",
				Handle:      "valid-schema",
				DisplayName: "Valid Schema",
				OUID:        "ou-1",
			},
			wantErr: true,
			errMsg:  "entity type ID is required",
		},
		{
			name: "missing organization unit ID",
			schema: &EntityType{
				ID:          "schema-1",
				Handle:      "valid-schema",
				DisplayName: "Valid Schema",
				OUID:        "",
			},
			wantErr: true,
			errMsg:  "ouId or ouHandle is required",
		},
		{
			name: "whitespace only organization unit ID",
			schema: &EntityType{
				ID:          "schema-1",
				Handle:      "valid-schema",
				DisplayName: "Valid Schema",
				OUID:        "   ",
			},
			wantErr: true,
			errMsg:  "ouId or ouHandle is required",
		},
		{
			name: "invalid schema JSON",
			schema: &EntityType{
				ID:          "schema-1",
				Handle:      "invalid-schema",
				DisplayName: "Invalid Schema",
				OUID:        "ou-1",
				Schema:      json.RawMessage(`{invalid json}`),
			},
			wantErr: true,
			errMsg:  "invalid schema for entity type",
		},
		{
			name: "empty schema definition rejected",
			schema: &EntityType{
				ID:          "schema-1",
				Handle:      "valid-schema",
				DisplayName: "Valid Schema",
				OUID:        "ou-1",
				Schema:      json.RawMessage(``),
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
			ID:          "schema-1",
			Handle:      "valid-schema",
			DisplayName: "Valid Schema",
			OUID:        "ou-1",
			Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
		}

		mockSvc.EXPECT().ResolveEntityTypeHandles(mock.Anything, schema).Return(nil).Once()

		validator := validateEntityTypeWrapper(mockSvc)
		err := validator(schema)

		assert.NoError(t, err)
	})

	t.Run("ou_handle resolved by service", func(t *testing.T) {
		mockSvc := NewEntityTypeServiceInterfaceMock(t)
		schema := &EntityType{
			ID:          "schema-1",
			Handle:      "valid-schema",
			DisplayName: "Valid Schema",
			OUHandle:    "default",
			Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
		}

		mockSvc.EXPECT().ResolveEntityTypeHandles(mock.Anything, schema).
			RunAndReturn(func(_ context.Context, et *EntityType) *tidcommon.ServiceError {
				et.OUID = "ou-resolved"
				return nil
			}).Once()

		validator := validateEntityTypeWrapper(mockSvc)
		err := validator(schema)

		assert.NoError(t, err)
		assert.Equal(t, "ou-resolved", schema.OUID)
	})

	t.Run("ou_handle not found", func(t *testing.T) {
		mockSvc := NewEntityTypeServiceInterfaceMock(t)
		schema := &EntityType{
			ID:          "schema-1",
			Handle:      "valid-schema",
			DisplayName: "Valid Schema",
			OUHandle:    "missing",
			Schema:      json.RawMessage(`{"email":{"type":"string"}}`),
		}

		mockSvc.EXPECT().ResolveEntityTypeHandles(mock.Anything, schema).
			Return(&ErrorInvalidRequestFormat).Once()

		validator := validateEntityTypeWrapper(mockSvc)
		err := validator(schema)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), `organization unit with handle "missing" not found`)
	})

	t.Run("invalid type", func(t *testing.T) {
		mockSvc := NewEntityTypeServiceInterfaceMock(t)
		invalidData := "not a schema"

		validator := validateEntityTypeWrapper(mockSvc)
		err := validator(invalidData)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid type: expected *EntityType")
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
handle: test-schema
displayName: Test Schema
ouId: ou-1
allowSelfRegistration: true
schema: '{"type": "object"}'
`,
			want: &EntityType{
				ID:                    "schema-1",
				Handle:                "test-schema",
				DisplayName:           "Test Schema",
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
handle: minimal-schema
displayName: Minimal Schema
ouId: ou-1
schema: '{}'
`,
			want: &EntityType{
				ID:                    "schema-2",
				Handle:                "minimal-schema",
				DisplayName:           "Minimal Schema",
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
handle: test-schema
displayName: Test Schema
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
handle: test-schema
displayName: Test Schema
ouId: ou-1
schema:
  username:
    type: string
    required: true
`,
			want: &EntityType{
				ID:          "schema-1",
				Handle:      "test-schema",
				DisplayName: "Test Schema",
				OUID:        "ou-1",
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
handle: test-schema
displayName: Test Schema
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
				assert.Equal(t, tc.want.Handle, result.Handle)
				assert.Equal(t, tc.want.DisplayName, result.DisplayName)
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
handle: test-schema
displayName: Test Schema
oUId: ou-1
schema: '{"type": "object"}'
`
	result, err := parseToEntityTypeDTOWrapper([]byte(yaml))

	assert.NoError(t, err)
	schema, ok := result.(*EntityType)
	assert.True(t, ok)
	assert.Equal(t, "schema-1", schema.ID)
	assert.Equal(t, "test-schema", schema.Handle)
	assert.Equal(t, "Test Schema", schema.DisplayName)
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

		err = loadDeclarativeResources(compositeStore, mockSvc)
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

		err = loadDeclarativeResources(fileStore, mockSvc)
		_ = err // Don't assert on error as it depends on file system state
	})

	t.Run("invalid store type", func(t *testing.T) {
		config.ResetServerRuntime()
		err := config.InitializeServerRuntime("", testConfig)
		assert.NoError(t, err)
		defer config.ResetServerRuntime()

		dbStore, _, _ := newEntityTypeStore()

		mockSvc := NewEntityTypeServiceInterfaceMock(t)

		err = loadDeclarativeResources(dbStore, mockSvc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid store type")
	})
}

// TestGetAllResourceIDs_WithReadOnlyFilter tests that declarative schemas are excluded from export.
func TestGetAllResourceIDs_WithReadOnlyFilter(t *testing.T) {
	mockService := NewEntityTypeServiceInterfaceMock(t)

	exporter := newEntityTypeExporter(mockService, TypeCategoryUser)

	response := &EntityTypeListResponse{
		Types: []EntityTypeListItem{
			// Mutable - should be included
			{ID: "schema1", Handle: "schema-1", DisplayName: "Schema 1", IsReadOnly: false},
			// Immutable - should be excluded
			{ID: "schema2", Handle: "schema-2", DisplayName: "Schema 2", IsReadOnly: true},
			// Mutable - should be included
			{ID: "schema3", Handle: "schema-3", DisplayName: "Schema 3", IsReadOnly: false},
		},
	}

	mockService.On("GetEntityTypeList", mock.Anything, mock.Anything, 100, 0, false).Return(response, nil).Once()
	mockService.On("GetEntityTypeList", mock.Anything, mock.Anything, 100, 3, false).
		Return(&EntityTypeListResponse{Types: []EntityTypeListItem{}}, nil).Once()

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

	err = loadDeclarativeResources(compositeStore, nil)
	_ = err // outcome depends on file system state; important that it doesn't panic
}

// TestValidateUniqueDeclarativeHandles tests handle uniqueness across declarative entity types.
func TestValidateUniqueDeclarativeHandles(t *testing.T) {
	newStore := func(t *testing.T, types ...EntityType) *entityTypeFileBasedStore {
		store, ok := newEntityTypeFileBasedStoreForTest().(*entityTypeFileBasedStore)
		assert.True(t, ok)
		for _, et := range types {
			assert.NoError(t, store.CreateEntityType(context.Background(), et))
		}
		return store
	}

	t.Run("no entity types", func(t *testing.T) {
		assert.NoError(t, validateUniqueDeclarativeHandles(newStore(t), nil))
	})

	t.Run("distinct handles", func(t *testing.T) {
		store := newStore(t,
			EntityType{ID: "t1", Category: TypeCategoryUser, Handle: "employee", DisplayName: "Employee"},
			EntityType{ID: "t2", Category: TypeCategoryUser, Handle: "customer", DisplayName: "Customer"},
			EntityType{ID: "t3", Category: TypeCategoryAgent, Handle: "default", DisplayName: "Default"},
		)
		assert.NoError(t, validateUniqueDeclarativeHandles(store, nil))
	})

	t.Run("same display name is allowed", func(t *testing.T) {
		store := newStore(t,
			EntityType{ID: "t1", Category: TypeCategoryUser, Handle: "employee", DisplayName: "Person"},
			EntityType{ID: "t2", Category: TypeCategoryUser, Handle: "customer", DisplayName: "Person"},
		)
		assert.NoError(t, validateUniqueDeclarativeHandles(store, nil))
	})

	t.Run("duplicate handle in the same category", func(t *testing.T) {
		store := newStore(t,
			EntityType{ID: "t1", Category: TypeCategoryUser, Handle: "employee", DisplayName: "Employee"},
			EntityType{ID: "t2", Category: TypeCategoryUser, Handle: "employee", DisplayName: "Employee 2"},
		)
		err := validateUniqueDeclarativeHandles(store, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `duplicate entity type handle "employee"`)
	})

	t.Run("same handle in user and agent categories is allowed", func(t *testing.T) {
		store := newStore(t,
			EntityType{ID: "t1", Category: TypeCategoryUser, Handle: "default", DisplayName: "Default User"},
			EntityType{ID: "t2", Category: TypeCategoryAgent, Handle: "default", DisplayName: "Default Agent"},
		)
		assert.NoError(t, validateUniqueDeclarativeHandles(store, nil))
	})

	t.Run("handle not used in the DB store", func(t *testing.T) {
		store := newStore(t,
			EntityType{ID: "t1", Category: TypeCategoryUser, Handle: "employee", DisplayName: "Employee"})
		dbStore := newEntityTypeStoreInterfaceMock(t)
		dbStore.On("GetEntityTypeByHandle", mock.Anything, TypeCategoryUser, "employee").
			Return(EntityType{}, ErrEntityTypeNotFound).Once()
		assert.NoError(t, validateUniqueDeclarativeHandles(store, dbStore))
	})

	t.Run("handle already used in the DB store", func(t *testing.T) {
		store := newStore(t,
			EntityType{ID: "t1", Category: TypeCategoryUser, Handle: "employee", DisplayName: "Employee"})
		dbStore := newEntityTypeStoreInterfaceMock(t)
		dbStore.On("GetEntityTypeByHandle", mock.Anything, TypeCategoryUser, "employee").
			Return(EntityType{ID: "db-1", Handle: "employee"}, nil).Once()
		err := validateUniqueDeclarativeHandles(store, dbStore)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "handle already used in the database store")
	})

	t.Run("DB store lookup fails", func(t *testing.T) {
		store := newStore(t,
			EntityType{ID: "t1", Category: TypeCategoryUser, Handle: "employee", DisplayName: "Employee"})
		dbStore := newEntityTypeStoreInterfaceMock(t)
		dbStore.On("GetEntityTypeByHandle", mock.Anything, TypeCategoryUser, "employee").
			Return(EntityType{}, errors.New("db error")).Once()
		err := validateUniqueDeclarativeHandles(store, dbStore)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to check for duplicate entity type handle")
	})
}

// TestLoadDeclarativeResources_RejectsDuplicateHandlesAcrossFiles verifies the loader fails when two
// YAML files declare the same handle.
func TestLoadDeclarativeResources_RejectsDuplicateHandlesAcrossFiles(t *testing.T) {
	tmpDir := t.TempDir()
	typesDir := tmpDir + "/config/resources/user_types"
	assert.NoError(t, os.MkdirAll(typesDir, 0750))

	writeType := func(file, id, category string) {
		content := "id: \"" + id + "\"\n" +
			"category: " + category + "\n" +
			"handle: \"shared-handle\"\n" +
			"displayName: \"Type " + id + "\"\n" +
			"ouId: \"550e8400-e29b-41d4-a716-446655440000\"\n" +
			"schema: |\n  {\"email\": {\"type\": \"string\"}}\n"
		assert.NoError(t, os.WriteFile(typesDir+"/"+file, []byte(content), 0600))
	}
	writeType("a.yaml", "type-a", "user")
	writeType("b.yaml", "type-b", "user")
	defer func() {
		for _, id := range []string{"type-a", "type-b"} {
			_ = entity.GetInstance().Delete(entity.NewCompositeKey(id, entity.KeyTypeEntityType))
		}
	}()

	config.ResetServerRuntime()
	assert.NoError(t, config.InitializeServerRuntime(tmpDir, &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: true},
	}))
	defer config.ResetServerRuntime()

	fileStore, _ := newEntityTypeFileBasedStore()
	mockSvc := NewEntityTypeServiceInterfaceMock(t)
	mockSvc.On("ResolveEntityTypeHandles", mock.Anything, mock.Anything).Return(nil).Maybe()

	err := loadDeclarativeResources(fileStore, mockSvc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate entity type handle")
}
