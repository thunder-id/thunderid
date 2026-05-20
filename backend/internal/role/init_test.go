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

package role

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

// InitTestSuite contains tests for store initialization.
type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

// SetupSuite initializes the test suite once.
func (suite *InitTestSuite) SetupSuite() {
	testConfig := &config.Config{}
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	if err != nil {
		suite.Fail("Failed to initialize runtime", err)
	}
}

// SetupTest resets configuration before each test.
func (suite *InitTestSuite) SetupTest() {
	runtime := config.GetServerRuntime()
	runtime.Config.Role.Store = ""
	runtime.Config.DeclarativeResources.Enabled = false
}

// TestInitializeStoreMutableMode tests store initialization in mutable mode.
func (suite *InitTestSuite) TestInitializeStoreMutableMode() {
	runtime := config.GetServerRuntime()
	runtime.Config.Role.Store = ""
	runtime.Config.DeclarativeResources.Enabled = false

	mockClient := &providermock.DBClientInterfaceMock{}
	mockClient.On("GetTransactioner").Return(&mockTransactioner{}, nil)
	mockProvider := &providermock.DBProviderInterfaceMock{}
	mockProvider.On("GetConfigDBClient").Return(mockClient, nil)
	originalGetDBProvider := getDBProvider
	getDBProvider = func() provider.DBProviderInterface { return mockProvider }
	defer func() { getDBProvider = originalGetDBProvider }()

	store, _, err := initializeStore()

	suite.NoError(err)
	suite.NotNil(store)
	// Verify it's a database store (not file-based or composite)
	_, isFileStore := store.(*fileBasedStore)
	_, isComposite := store.(*compositeRoleStore)
	suite.False(isFileStore, "should not be file-based store in mutable mode")
	suite.False(isComposite, "should not be composite store in mutable mode")
}

// TestInitializeStoreDeclarativeMode tests store initialization in declarative mode.
func (suite *InitTestSuite) TestInitializeStoreDeclarativeMode() {
	runtime := config.GetServerRuntime()
	runtime.Config.Role.Store = string(serverconst.StoreModeDeclarative)
	runtime.Config.DeclarativeResources.Enabled = true

	store, _, err := initializeStore()

	suite.NoError(err)
	suite.NotNil(store)
	// Verify it's a file-based store
	fileStore, isFileStore := store.(*fileBasedStore)
	suite.True(isFileStore, "should be file-based store in declarative mode")
	suite.NotNil(fileStore)
}

// TestInitializeStoreCompositeMode tests store initialization in composite mode.
func (suite *InitTestSuite) TestInitializeStoreCompositeMode() {
	runtime := config.GetServerRuntime()
	runtime.Config.Role.Store = string(serverconst.StoreModeComposite)

	mockClient := &providermock.DBClientInterfaceMock{}
	mockClient.On("GetTransactioner").Return(&mockTransactioner{}, nil)
	mockProvider := &providermock.DBProviderInterfaceMock{}
	mockProvider.On("GetConfigDBClient").Return(mockClient, nil)
	originalGetDBProvider := getDBProvider
	getDBProvider = func() provider.DBProviderInterface { return mockProvider }
	defer func() { getDBProvider = originalGetDBProvider }()

	store, _, err := initializeStore()

	suite.NoError(err)
	suite.NotNil(store)
	// Verify it's a composite store
	compositeStore, isComposite := store.(*compositeRoleStore)
	suite.True(isComposite, "should be composite store in composite mode")
	suite.NotNil(compositeStore)
	// Verify compositing has both stores
	suite.NotNil(compositeStore.fileStore)
	suite.NotNil(compositeStore.dbStore)
}

// TestInitializeStoreDeclarativeFallback tests fall back to declarative mode when enabled globally.
func (suite *InitTestSuite) TestInitializeStoreDeclarativeFallback() {
	runtime := config.GetServerRuntime()
	runtime.Config.Role.Store = ""
	runtime.Config.DeclarativeResources.Enabled = true

	store, _, err := initializeStore()

	suite.NoError(err)
	suite.NotNil(store)
	// Verify it falls back to declarative when global setting is enabled
	fileStore, isFileStore := store.(*fileBasedStore)
	suite.True(isFileStore, "should be file-based store when declarative resources are enabled globally")
	suite.NotNil(fileStore)
}

// TestInitializeStoreMutableModeFallback tests fall back to mutable mode when declarative is disabled.
func (suite *InitTestSuite) TestInitializeStoreMutableModeFallback() {
	runtime := config.GetServerRuntime()
	runtime.Config.Role.Store = ""
	runtime.Config.DeclarativeResources.Enabled = false

	mockClient := &providermock.DBClientInterfaceMock{}
	mockClient.On("GetTransactioner").Return(&mockTransactioner{}, nil)
	mockProvider := &providermock.DBProviderInterfaceMock{}
	mockProvider.On("GetConfigDBClient").Return(mockClient, nil)
	originalGetDBProvider := getDBProvider
	getDBProvider = func() provider.DBProviderInterface { return mockProvider }
	defer func() { getDBProvider = originalGetDBProvider }()

	store, _, err := initializeStore()

	suite.NoError(err)
	suite.NotNil(store)
	// Verify it falls back to mutable when global setting is disabled
	_, isFileStore := store.(*fileBasedStore)
	_, isComposite := store.(*compositeRoleStore)
	suite.False(isFileStore, "should not be file-based store when declarative resources are disabled")
	suite.False(isComposite, "should not be composite store when declarative resources are disabled")
}

// TestInitializeStoreNormalizeCaseSensitivity tests store mode is case-insensitive.
func (suite *InitTestSuite) TestInitializeStoreNormalizeCaseSensitivity() {
	runtime := config.GetServerRuntime()
	runtime.Config.Role.Store = "DECLARATIVE"

	store, _, err := initializeStore()

	suite.NoError(err)
	suite.NotNil(store)
	// Verify it handles case-insensitive mode
	fileStore, isFileStore := store.(*fileBasedStore)
	suite.True(isFileStore, "should be file-based store with uppercase declarative mode")
	suite.NotNil(fileStore)
}

// TestInitializeStoreTrimsWhitespace tests store mode trims whitespace.
func (suite *InitTestSuite) TestInitializeStoreTrimsWhitespace() {
	runtime := config.GetServerRuntime()
	runtime.Config.Role.Store = "  composite  "

	mockClient := &providermock.DBClientInterfaceMock{}
	mockClient.On("GetTransactioner").Return(&mockTransactioner{}, nil)
	mockProvider := &providermock.DBProviderInterfaceMock{}
	mockProvider.On("GetConfigDBClient").Return(mockClient, nil)
	originalGetDBProvider := getDBProvider
	getDBProvider = func() provider.DBProviderInterface { return mockProvider }
	defer func() { getDBProvider = originalGetDBProvider }()

	store, _, err := initializeStore()

	suite.NoError(err)
	suite.NotNil(store)
	// Verify it handles whitespace trimming
	compositeStore, isComposite := store.(*compositeRoleStore)
	suite.True(isComposite, "should be composite store with whitespace in mode")
	suite.NotNil(compositeStore)
}

// LoadDeclarativeResourcesTestSuite contains tests for loadDeclarativeResources.
type LoadDeclarativeResourcesTestSuite struct {
	suite.Suite
}

func TestLoadDeclarativeResourcesTestSuite(t *testing.T) {
	suite.Run(t, new(LoadDeclarativeResourcesTestSuite))
}

// SetupSuite initializes the test suite once.
func (suite *LoadDeclarativeResourcesTestSuite) SetupSuite() {
	testConfig := &config.Config{}
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	if err != nil {
		suite.Fail("Failed to initialize runtime", err)
	}
}

// SetupTest resets configuration before each test.
func (suite *LoadDeclarativeResourcesTestSuite) SetupTest() {
	runtime := config.GetServerRuntime()
	runtime.Config.Role.Store = ""
	runtime.Config.DeclarativeResources.Enabled = false
}

// TestLoadDeclarativeResourcesWithFileStore tests loading declarative resources into file store.
func (suite *LoadDeclarativeResourcesTestSuite) TestLoadDeclarativeResourcesWithFileStore() {
	// Create a generic file-based store for testing
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeRole)
	fileStore := &fileBasedStore{GenericFileBasedStore: genericStore}

	err := loadDeclarativeResources(fileStore, nil)

	// Should not error even if no resources are found (empty directory)
	suite.NoError(err)
}

// TestLoadDeclarativeResourcesWithNilDbStore tests loading with nil database store (declarative mode).
func (suite *LoadDeclarativeResourcesTestSuite) TestLoadDeclarativeResourcesWithNilDbStore() {
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeRole)
	fileStore := &fileBasedStore{GenericFileBasedStore: genericStore}

	// Should handle nil dbStore gracefully (no duplicate checking against DB)
	err := loadDeclarativeResources(fileStore, nil)

	suite.NoError(err)
}

// TestLoadDeclarativeResourcesWithDbStore tests loading with database store (composite mode).
func (suite *LoadDeclarativeResourcesTestSuite) TestLoadDeclarativeResourcesWithDbStore() {
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeRole)
	fileStore := &fileBasedStore{GenericFileBasedStore: genericStore}

	// Create a mock database store
	mockDbStore := newRoleStoreInterfaceMock(suite.T())
	// Don't setup any expectations since no resources are loaded by default

	err := loadDeclarativeResources(fileStore, mockDbStore)

	suite.NoError(err)
}

// TestLoadDeclarativeResourcesValidatorFunction tests that validator is properly configured.
func (suite *LoadDeclarativeResourcesTestSuite) TestLoadDeclarativeResourcesValidatorFunction() {
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeRole)
	fileStore := &fileBasedStore{GenericFileBasedStore: genericStore}

	// Test the validator function directly without mocks
	testRole := &RoleWithPermissionsAndAssignments{
		ID:          "test-role",
		Name:        "Test Role",
		Description: "A test role",
		OUID:        "ou-default",
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: "rs-default",
				Permissions:      []string{"read", "write"},
			},
		},
	}

	// Validate the role with no dbStore (validation should pass for valid role)
	err := validateRoleWrapper(testRole, fileStore, nil)

	suite.NoError(err)
}

// TestLoadDeclarativeResourcesIDExtractorFunction tests ID extraction from role data.
func (suite *LoadDeclarativeResourcesTestSuite) TestLoadDeclarativeResourcesIDExtractorFunction() {
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeRole)
	fileStore := &fileBasedStore{GenericFileBasedStore: genericStore}

	// Add a role with explicit ID
	testRole := &RoleWithPermissionsAndAssignments{
		ID:   "extracted-id",
		Name: "ID Test",
		OUID: "ou-default",
	}
	err := fileStore.GenericFileBasedStore.Create(testRole.ID, testRole)
	suite.NoError(err)

	// Load should correctly extract the ID
	err = loadDeclarativeResources(fileStore, nil)

	suite.NoError(err)
	// Verify the role is still in the store with correct ID
	ctx := context.Background()
	role, err := fileStore.GetRole(ctx, "extracted-id")
	suite.NoError(err)
	suite.Equal("extracted-id", role.ID)
}

// TestLoadDeclarativeResourcesParserFunction tests YAML parsing in loadDeclarativeResources.
func (suite *LoadDeclarativeResourcesTestSuite) TestLoadDeclarativeResourcesParserFunction() {
	// Test that parseToRoleWrapper correctly converts raw data
	yamlData := []byte(`
id: parser-test-role
name: Parser Test Role
description: Testing YAML parser
ou_id: ou-default
permissions:
  - resource_server_id: api-server
    permissions:
      - read
      - write
assignments:
  - id: user1
    type: user
`)

	// Parse using the wrapper function
	parsed, err := parseToRoleWrapper(yamlData)
	suite.NoError(err)
	suite.NotNil(parsed)

	role, ok := parsed.(*RoleWithPermissionsAndAssignments)
	suite.True(ok)
	suite.Equal("parser-test-role", role.ID)
	suite.Equal("Parser Test Role", role.Name)
	suite.Equal("ou-default", role.OUID)
	suite.Len(role.Permissions, 1)
	suite.Equal("api-server", role.Permissions[0].ResourceServerID)
	suite.Len(role.Assignments, 1)
	suite.Equal("user1", role.Assignments[0].ID)
	suite.Equal(assigneeTypeEntity, role.Assignments[0].Type)
}

// TestLoadDeclarativeResourcesValidateRoleWrapperRequiredFields tests validation of required fields.
func (suite *LoadDeclarativeResourcesTestSuite) TestLoadDeclarativeResourcesValidateRoleWrapperRequiredFields() {
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeRole)
	fileStore := &fileBasedStore{GenericFileBasedStore: genericStore}

	// Test missing ID
	roleNoID := &RoleWithPermissionsAndAssignments{
		Name: "No ID",
		OUID: "ou1",
	}
	err := validateRoleWrapper(roleNoID, fileStore, nil)
	suite.Error(err)
	suite.Contains(err.Error(), "role ID is required")

	// Test missing Name
	roleNoName := &RoleWithPermissionsAndAssignments{
		ID:   "role1",
		OUID: "ou1",
	}
	err = validateRoleWrapper(roleNoName, fileStore, nil)
	suite.Error(err)
	suite.Contains(err.Error(), "role name is required")

	// Test missing OUID
	roleNoOU := &RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Test Role",
	}
	err = validateRoleWrapper(roleNoOU, fileStore, nil)
	suite.Error(err)
	suite.Contains(err.Error(), "organization unit ID is required")
}

// TestLoadDeclarativeResourcesValidateAssignmentTypes tests validation of assignment types.
func (suite *LoadDeclarativeResourcesTestSuite) TestLoadDeclarativeResourcesValidateAssignmentTypes() {
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeRole)
	fileStore := &fileBasedStore{GenericFileBasedStore: genericStore}

	// Test role with valid user assignment
	roleValidUser := &RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Test Role",
		OUID: "ou1",
		Assignments: []RoleAssignment{
			{ID: "user1", Type: assigneeTypeEntity},
		},
	}
	err := validateRoleWrapper(roleValidUser, fileStore, nil)
	suite.NoError(err)

	// Test role with valid group assignment
	roleValidGroup := &RoleWithPermissionsAndAssignments{
		ID:   "role2",
		Name: "Test Role 2",
		OUID: "ou1",
		Assignments: []RoleAssignment{
			{ID: "group1", Type: AssigneeTypeGroup},
		},
	}
	err = validateRoleWrapper(roleValidGroup, fileStore, nil)
	suite.NoError(err)

	// Test role with invalid assignment type
	roleInvalidType := &RoleWithPermissionsAndAssignments{
		ID:   "role3",
		Name: "Test Role 3",
		OUID: "ou1",
		Assignments: []RoleAssignment{
			{ID: "invalid1", Type: "invalid_type"},
		},
	}
	err = validateRoleWrapper(roleInvalidType, fileStore, nil)
	suite.Error(err)
	suite.Contains(err.Error(), "invalid assignment type")

	// Test role with missing assignment ID
	roleNoAssignmentID := &RoleWithPermissionsAndAssignments{
		ID:   "role4",
		Name: "Test Role 4",
		OUID: "ou1",
		Assignments: []RoleAssignment{
			{Type: assigneeTypeEntity},
		},
	}
	err = validateRoleWrapper(roleNoAssignmentID, fileStore, nil)
	suite.Error(err)
	suite.Contains(err.Error(), "assignment ID is required")
}

// TestLoadDeclarativeResourcesValidatePermissions tests validation of resource permissions.
func (suite *LoadDeclarativeResourcesTestSuite) TestLoadDeclarativeResourcesValidatePermissions() {
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeRole)
	fileStore := &fileBasedStore{GenericFileBasedStore: genericStore}

	// Test role with missing resource_server_id
	roleNoResourceServer := &RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Test Role",
		OUID: "ou1",
		Permissions: []ResourcePermissions{
			{Permissions: []string{"read"}},
		},
	}
	err := validateRoleWrapper(roleNoResourceServer, fileStore, nil)
	suite.Error(err)
	suite.Contains(err.Error(), "resource server ID is required")

	// Test role with valid permission
	roleValidPermission := &RoleWithPermissionsAndAssignments{
		ID:   "role2",
		Name: "Test Role 2",
		OUID: "ou1",
		Permissions: []ResourcePermissions{
			{
				ResourceServerID: "api-server",
				Permissions:      []string{"read", "write"},
			},
		},
	}
	err = validateRoleWrapper(roleValidPermission, fileStore, nil)
	suite.NoError(err)
}

// TestLoadDeclarativeResourcesValidateDuplicateFileStore tests duplicate detection in file store.
func (suite *LoadDeclarativeResourcesTestSuite) TestLoadDeclarativeResourcesValidateDuplicateFileStore() {
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeRole)
	fileStore := &fileBasedStore{GenericFileBasedStore: genericStore}

	// Add a role to file store
	existingRole := &RoleWithPermissionsAndAssignments{
		ID:   "duplicate-role",
		Name: "Existing",
		OUID: "ou1",
	}
	err := fileStore.GenericFileBasedStore.Create(existingRole.ID, existingRole)
	suite.NoError(err)

	// Try to validate same role again
	err = validateRoleWrapper(existingRole, fileStore, nil)
	suite.Error(err)
	suite.Contains(err.Error(), "duplicate role ID")
	suite.Contains(err.Error(), "declarative resources")
}

// TestLoadDeclarativeResourcesValidateDuplicateDbStore tests duplicate detection in database store.
func (suite *LoadDeclarativeResourcesTestSuite) TestLoadDeclarativeResourcesValidateDuplicateDbStore() {
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeRole)
	fileStore := &fileBasedStore{GenericFileBasedStore: genericStore}
	mockDbStore := newRoleStoreInterfaceMock(suite.T())

	role := &RoleWithPermissionsAndAssignments{
		ID:   "db-duplicate",
		Name: "Test",
		OUID: "ou1",
	}

	// Mock database store to indicate role exists
	mockDbStore.On("IsRoleExist", mock.Anything, "db-duplicate").Return(true, nil).Once()

	err := validateRoleWrapper(role, fileStore, mockDbStore)
	suite.Error(err)
	suite.Contains(err.Error(), "duplicate role ID")
	suite.Contains(err.Error(), "database store")
	mockDbStore.AssertExpectations(suite.T())
}

// TestLoadDeclarativeResourcesValidateDbStoreError tests handling of database errors.
func (suite *LoadDeclarativeResourcesTestSuite) TestLoadDeclarativeResourcesValidateDbStoreError() {
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeRole)
	fileStore := &fileBasedStore{GenericFileBasedStore: genericStore}
	mockDbStore := newRoleStoreInterfaceMock(suite.T())

	role := &RoleWithPermissionsAndAssignments{
		ID:   "db-error-role",
		Name: "Test",
		OUID: "ou1",
	}

	// Mock database store to return error
	mockDbStore.On("IsRoleExist", mock.Anything, "db-error-role").Return(
		false, fmt.Errorf("database error"),
	).Once()

	err := validateRoleWrapper(role, fileStore, mockDbStore)
	suite.Error(err)
	suite.Contains(err.Error(), "database error")
	mockDbStore.AssertExpectations(suite.T())
}

// TestRoleFromDeclarativeData tests roleFromDeclarativeData function with various type assertions.
func (suite *LoadDeclarativeResourcesTestSuite) TestRoleFromDeclarativeDataPointerType() {
	role := &RoleWithPermissionsAndAssignments{
		ID:   "test1",
		Name: "Test",
		OUID: "ou1",
	}

	result, err := roleFromDeclarativeData("test1", role)

	suite.NoError(err)
	suite.Equal("test1", result.ID)
	suite.Equal("Test", result.Name)
}

// TestRoleFromDeclarativeDataValueType tests rejection of value types.
func (suite *LoadDeclarativeResourcesTestSuite) TestRoleFromDeclarativeDataValueType() {
	role := RoleWithPermissionsAndAssignments{
		ID:   "test2",
		Name: "Test Value",
		OUID: "ou1",
	}

	result, err := roleFromDeclarativeData("test2", role)

	suite.Error(err)
	suite.Contains(err.Error(), "role data corrupted")
	suite.Empty(result.ID)
}

// TestRoleFromDeclarativeDataInvalidType tests error handling for invalid types.
func (suite *LoadDeclarativeResourcesTestSuite) TestRoleFromDeclarativeDataInvalidType() {
	result, err := roleFromDeclarativeData("invalid", "invalid string type")

	suite.Error(err)
	suite.Contains(err.Error(), "role data corrupted")
	suite.Empty(result.ID)
}

// TestIsEntityNotFoundError tests the error detection function.
func (suite *LoadDeclarativeResourcesTestSuite) TestIsEntityNotFoundError() {
	suite.True(isEntityNotFoundError(fmt.Errorf("entity not found")))
	suite.True(isEntityNotFoundError(fmt.Errorf("something not found")))
	suite.False(isEntityNotFoundError(fmt.Errorf("some other error")))
	suite.False(isEntityNotFoundError(nil))
}

// TestMatchesAssignee tests the role assignment matching logic.
func (suite *LoadDeclarativeResourcesTestSuite) TestMatchesAssigneeUserMatch() {
	assignments := []RoleAssignment{
		{ID: "user1", Type: assigneeTypeEntity},
		{ID: "group1", Type: AssigneeTypeGroup},
	}
	groupSet := map[string]bool{
		"group1": true,
		"group2": true,
	}

	// Test user match
	suite.True(matchesAssignee(assignments, "user1", groupSet))
}

// TestMatchesAssigneeGroupMatch tests group assignment matching.
func (suite *LoadDeclarativeResourcesTestSuite) TestMatchesAssigneeGroupMatch() {
	assignments := []RoleAssignment{
		{ID: "user1", Type: assigneeTypeEntity},
		{ID: "group1", Type: AssigneeTypeGroup},
	}
	groupSet := map[string]bool{
		"group1": true,
	}

	// Test group match
	suite.True(matchesAssignee(assignments, "user2", groupSet))
}

// TestMatchesAssigneeNoMatch tests when no assignments match.
func (suite *LoadDeclarativeResourcesTestSuite) TestMatchesAssigneeNoMatch() {
	assignments := []RoleAssignment{
		{ID: "user1", Type: assigneeTypeEntity},
		{ID: "group1", Type: AssigneeTypeGroup},
	}
	groupSet := map[string]bool{
		"group2": true,
	}

	// Test no match
	suite.False(matchesAssignee(assignments, "user2", groupSet))
}

// TestMatchesAssigneeEmptyAssignments tests with empty assignments.
func (suite *LoadDeclarativeResourcesTestSuite) TestMatchesAssigneeEmptyAssignments() {
	assignments := []RoleAssignment{}
	groupSet := map[string]bool{}

	suite.False(matchesAssignee(assignments, "", groupSet))
}

// TestInitialize_DBClientError tests Initialize when DB client retrieval fails
func (suite *InitTestSuite) TestInitialize_DBClientError() {
	mockProvider := &providermock.DBProviderInterfaceMock{}
	mockProvider.On("GetConfigDBClient").Return(nil, errors.New("mock db client error"))

	originalGetDBProvider := getDBProvider
	getDBProvider = func() provider.DBProviderInterface {
		return mockProvider
	}
	defer func() {
		getDBProvider = originalGetDBProvider
	}()

	mux := http.NewServeMux()
	_, _, _, err := Initialize(mux, nil, nil, nil, nil, nil)

	suite.Error(err)
	suite.Equal("mock db client error", err.Error())
	mockProvider.AssertExpectations(suite.T())
}

// TestInitialize_TransactionerError tests Initialize when transactioner retrieval fails
func (suite *InitTestSuite) TestInitialize_TransactionerError() {
	mockClient := &providermock.DBClientInterfaceMock{}
	mockClient.On("GetTransactioner").Return(nil, errors.New("mock transactioner error"))

	mockProvider := &providermock.DBProviderInterfaceMock{}
	mockProvider.On("GetConfigDBClient").Return(mockClient, nil)

	originalGetDBProvider := getDBProvider
	getDBProvider = func() provider.DBProviderInterface {
		return mockProvider
	}
	defer func() {
		getDBProvider = originalGetDBProvider
	}()

	mux := http.NewServeMux()
	_, _, _, err := Initialize(mux, nil, nil, nil, nil, nil)

	suite.Error(err)
	suite.Equal("mock transactioner error", err.Error())
	mockProvider.AssertExpectations(suite.T())
	mockClient.AssertExpectations(suite.T())
}

type mockTransactioner struct{}

func (m *mockTransactioner) Transact(ctx context.Context, operation func(txCtx context.Context) error) error {
	return operation(ctx)
}

// TestInitialize_Success tests successful Initialize
func (suite *InitTestSuite) TestInitialize_Success() {
	mockClient := &providermock.DBClientInterfaceMock{}
	mockClient.On("GetTransactioner").Return(&mockTransactioner{}, nil)

	mockProvider := &providermock.DBProviderInterfaceMock{}
	mockProvider.On("GetConfigDBClient").Return(mockClient, nil)

	originalGetDBProvider := getDBProvider
	getDBProvider = func() provider.DBProviderInterface {
		return mockProvider
	}
	defer func() {
		getDBProvider = originalGetDBProvider
	}()

	mux := http.NewServeMux()
	svc, _, exporter, err := Initialize(mux, nil, nil, nil, nil, nil)

	suite.NoError(err)
	suite.NotNil(svc)
	suite.NotNil(exporter)
	mockProvider.AssertExpectations(suite.T())
	mockClient.AssertExpectations(suite.T())
}

// TestInitialize_StoreInitError tests Initialize when store initialization fails
func (suite *InitTestSuite) TestInitialize_StoreInitError() {
	config.ResetServerRuntime()
	testDir := suite.T().TempDir()

	// Create invalid roles yaml
	rolesDir := filepath.Join(testDir, "repository", "resources", "roles")
	err := os.MkdirAll(rolesDir, 0750)
	suite.NoError(err)
	err = os.WriteFile(filepath.Join(rolesDir, "invalid.yaml"), []byte("invalid yaml content: ["), 0600)
	suite.NoError(err)

	testConfig := &config.Config{
		Role: config.RoleConfig{
			Store: string(serverconst.StoreModeDeclarative),
		},
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
	}
	_ = config.InitializeServerRuntime(testDir, testConfig)
	defer func() {
		config.ResetServerRuntime()
		suite.SetupSuite()
	}()

	mux := http.NewServeMux()
	svc, _, exporter, err := Initialize(mux, nil, nil, nil, nil, nil)

	suite.Error(err)
	if err != nil {
		suite.Contains(err.Error(), "failed to load role resources")
	}
	suite.Nil(svc)
	suite.Nil(exporter)
}
