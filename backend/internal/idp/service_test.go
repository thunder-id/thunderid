// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package idp

import (
	"context"
	"errors"
	"testing"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/groupmock"
	"github.com/thunder-id/thunderid/tests/mocks/resourceserverprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/rolemock"
)

type mockTransactioner struct{}

func (m *mockTransactioner) Transact(ctx context.Context, operation func(txCtx context.Context) error) error {
	return operation(ctx)
}

// stubDependencyRegistry is a minimal resourcedependency.Registry for tests.
type stubDependencyRegistry struct {
	resp *resourcedependency.DependenciesResponse
	err  error
}

func (r *stubDependencyRegistry) RegisterProvider(resourcedependency.Provider) {}

func (r *stubDependencyRegistry) GetDependencies(
	context.Context, string, string) (*resourcedependency.DependenciesResponse, error) {
	return r.resp, r.err
}

func (r *stubDependencyRegistry) CascadeDelete(context.Context, string, string) (int, error) {
	return 0, nil
}

func (r *stubDependencyRegistry) ValidateReferenceUpdate(
	context.Context, string, string) *tidcommon.ServiceError {
	return nil
}

// newNoBlockingDepsRegistry returns a registry reporting confirmed-empty dependencies, so that
// deletion is permitted by the blocking guard.
func newNoBlockingDepsRegistry() *stubDependencyRegistry {
	total := 0
	return &stubDependencyRegistry{resp: &resourcedependency.DependenciesResponse{
		TotalResults: &total,
		Usages:       []resourcedependency.ResourceDependency{},
	}}
}

type IDPServiceTestSuite struct {
	suite.Suite
	mockStore    *idpStoreInterfaceMock
	mockET       *entitytypemock.EntityTypeServiceInterfaceMock
	mockRole     *rolemock.RoleServiceInterfaceMock
	mockGroup    *groupmock.GroupServiceInterfaceMock
	mockResource *resourceserverprovidermock.ResourceServerProviderMock
	idpService   *idpService
}

const (
	declarativeIDPTestID = "declarative-idp"
	mutableIDPTestID     = "mutable-idp"
)

func TestIDPServiceTestSuite(t *testing.T) {
	suite.Run(t, new(IDPServiceTestSuite))
}

func (s *IDPServiceTestSuite) SetupTest() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	s.mockStore = newIdpStoreInterfaceMock(s.T())
	s.mockET = entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	// Create and update now seed schema-aware defaults, which lists user types first. Default to a
	// deployment with none, so the target is unresolvable and seeding is a no-op for tests that are
	// not about it. Tests that exercise seeding build their own service with a dedicated mock.
	s.mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser,
		mock.Anything, mock.Anything, mock.Anything).
		Return(&entitytype.EntityTypeListResponse{}, nil).Maybe()
	// Unused unless a test configures AuthorizationMappings: no expectations are set here, so a test
	// that never reaches the existence check never touches them.
	s.mockRole = rolemock.NewRoleServiceInterfaceMock(s.T())
	s.mockGroup = groupmock.NewGroupServiceInterfaceMock(s.T())
	s.mockResource = resourceserverprovidermock.NewResourceServerProviderMock(s.T())
	s.idpService = &idpService{
		idpStore:           s.mockStore,
		transactioner:      &mockTransactioner{},
		dependencyRegistry: newNoBlockingDepsRegistry(),
		logger:             log.GetLogger().With(log.String(log.LoggerKeyComponentName, "IdPService")),
		uuidGenerator:      utils.GenerateUUIDv7,
		entityTypeService:  s.mockET,
		roleService:        s.mockRole,
		groupService:       s.mockGroup,
		resourceService:    s.mockResource,
	}
}

func (s *IDPServiceTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func createOIDCProperties() []cmodels.Property {
	prop1, _ := cmodels.NewProperty("client_id", "test-client", false)
	prop2, _ := cmodels.NewProperty("client_secret", "test-secret", false)
	prop3, _ := cmodels.NewProperty("redirect_uri", "http://localhost/callback", false)
	prop4, _ := cmodels.NewProperty("authorization_endpoint", "http://idp/auth", false)
	prop5, _ := cmodels.NewProperty("token_endpoint", "http://idp/token", false)
	return []cmodels.Property{*prop1, *prop2, *prop3, *prop4, *prop5}
}

// TestCreateIdentityProvider_Success tests successful IDP creation
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_Success() {
	idp := &providers.IDPDTO{
		Name:        "Test IDP",
		Description: "Test Description",
		Type:        providers.IDPTypeOIDC,
		Properties:  createOIDCProperties(),
	}

	s.mockStore.On("GetIdentityProviderByName", mock.Anything, "Test IDP").
		Return((*providers.IDPDTO)(nil), ErrIDPNotFound)
	s.mockStore.On("CreateIdentityProvider", mock.Anything, mock.MatchedBy(func(dto providers.IDPDTO) bool {
		return dto.Name == "Test IDP" && dto.Type == providers.IDPTypeOIDC && dto.ID != ""
	})).Return(nil)

	result, err := s.idpService.CreateIdentityProvider(context.Background(), idp)

	s.Nil(err)
	s.NotNil(result)
	s.NotEmpty(result.ID)
	s.Equal("Test IDP", result.Name)
	s.mockStore.AssertExpectations(s.T())
}

// TestCreateIdentityProvider_NilIDP tests nil IDP validation
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_NilIDP() {
	result, err := s.idpService.CreateIdentityProvider(context.Background(), nil)

	s.Nil(result)
	s.NotNil(err)
	s.Equal(ErrorIDPNil.Code, err.Code)
}

// TestCreateIdentityProvider_InvalidName tests invalid name validation
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_InvalidName() {
	testCases := []struct {
		name     string
		idpName  string
		expected tidcommon.ServiceError
	}{
		{"Empty name", "", ErrorInvalidIDPName},
		{"Whitespace name", "   ", ErrorInvalidIDPName},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			idp := &providers.IDPDTO{
				Name: tc.idpName,
				Type: providers.IDPTypeOIDC,
			}

			result, err := s.idpService.CreateIdentityProvider(context.Background(), idp)

			s.Nil(result)
			s.NotNil(err)
			s.Equal(tc.expected.Code, err.Code)
		})
	}
}

// TestCreateIdentityProvider_InvalidType tests invalid type validation
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_InvalidType() {
	testCases := []struct {
		name    string
		idpType providers.IDPType
	}{
		{"Empty type", providers.IDPType("")},
		{"Invalid type", providers.IDPType("INVALID")},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			idp := &providers.IDPDTO{
				Name: "Test IDP",
				Type: tc.idpType,
			}

			result, err := s.idpService.CreateIdentityProvider(context.Background(), idp)

			s.Nil(result)
			s.NotNil(err)
			s.Equal(ErrorInvalidIDPType.Code, err.Code)
		})
	}
}

// TestCreateIdentityProvider_AlreadyExists tests duplicate IDP name
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_AlreadyExists() {
	idp := &providers.IDPDTO{
		Name:       "Existing IDP",
		Type:       providers.IDPTypeOIDC,
		Properties: createOIDCProperties(),
	}

	existingIDP := &providers.IDPDTO{ID: "existing-id", Name: "Existing IDP"}
	s.mockStore.On("GetIdentityProviderByName", mock.Anything, "Existing IDP").Return(existingIDP, nil)

	result, err := s.idpService.CreateIdentityProvider(context.Background(), idp)

	s.Nil(result)
	s.NotNil(err)
	s.Equal(ErrorIDPAlreadyExists.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestCreateIdentityProvider_CheckExistingStoreError tests store error when checking existing IDP
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_CheckExistingStoreError() {
	idp := &providers.IDPDTO{
		Name:       "Test IDP",
		Type:       providers.IDPTypeOIDC,
		Properties: createOIDCProperties(),
	}

	s.mockStore.On("GetIdentityProviderByName", mock.Anything, "Test IDP").
		Return((*providers.IDPDTO)(nil), errors.New("database error"))

	result, err := s.idpService.CreateIdentityProvider(context.Background(), idp)

	s.Nil(result)
	s.NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestCreateIdentityProvider_StoreError tests store error handling
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_StoreError() {
	idp := &providers.IDPDTO{
		Name:       "Test IDP",
		Type:       providers.IDPTypeOIDC,
		Properties: createOIDCProperties(),
	}

	s.mockStore.On("GetIdentityProviderByName", mock.Anything, "Test IDP").
		Return((*providers.IDPDTO)(nil), ErrIDPNotFound)
	s.mockStore.On("CreateIdentityProvider", mock.Anything, mock.Anything).Return(errors.New("database error"))

	result, err := s.idpService.CreateIdentityProvider(context.Background(), idp)

	s.Nil(result)
	s.NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestCreateIdentityProvider_WithPresetID tests that a preset ID is preserved and not overwritten.
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_WithPresetID() {
	presetID := "preset-idp-id-1234"
	idp := &providers.IDPDTO{
		ID:          presetID,
		Name:        "Test IDP",
		Description: "Test Description",
		Type:        providers.IDPTypeOIDC,
		Properties:  createOIDCProperties(),
	}

	s.mockStore.On("GetIdentityProviderByName", mock.Anything, "Test IDP").
		Return((*providers.IDPDTO)(nil), ErrIDPNotFound)
	s.mockStore.On("CreateIdentityProvider", mock.Anything, mock.MatchedBy(func(dto providers.IDPDTO) bool {
		return dto.ID == presetID
	})).Return(nil)

	result, err := s.idpService.CreateIdentityProvider(context.Background(), idp)

	s.Nil(err)
	s.NotNil(result)
	s.Equal(presetID, result.ID)
	s.mockStore.AssertExpectations(s.T())
}

// TestCreateIdentityProvider_UUIDGenerationError tests that a UUID generation failure returns InternalServerError.
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_UUIDGenerationError() {
	idp := &providers.IDPDTO{
		Name:       "Test IDP",
		Type:       providers.IDPTypeOIDC,
		Properties: createOIDCProperties(),
	}

	s.idpService.uuidGenerator = func() (string, error) {
		return "", errors.New("entropy source failed")
	}

	result, err := s.idpService.CreateIdentityProvider(context.Background(), idp)

	s.Nil(result)
	s.NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// TestGetIdentityProviderList_Success tests successful list retrieval
func (s *IDPServiceTestSuite) TestGetIdentityProviderList_Success() {
	idpList := []BasicIDPDTO{
		{ID: "idp-1", Name: "IDP 1", Type: providers.IDPTypeOIDC},
		{ID: "idp-2", Name: "IDP 2", Type: providers.IDPTypeGoogle},
	}

	s.mockStore.On("GetIdentityProviderList", mock.Anything).Return(idpList, nil)

	result, err := s.idpService.GetIdentityProviderList(context.Background())

	s.Nil(err)
	s.NotNil(result)
	s.Len(result, 2)
	s.Equal("idp-1", result[0].ID)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIdentityProviderList_EmptyList tests empty list
func (s *IDPServiceTestSuite) TestGetIdentityProviderList_EmptyList() {
	s.mockStore.On("GetIdentityProviderList", mock.Anything).Return([]BasicIDPDTO{}, nil)

	result, err := s.idpService.GetIdentityProviderList(context.Background())

	s.Nil(err)
	s.NotNil(result)
	s.Len(result, 0)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIdentityProviderList_StoreError tests store error handling
func (s *IDPServiceTestSuite) TestGetIdentityProviderList_StoreError() {
	s.mockStore.On("GetIdentityProviderList", mock.Anything).Return([]BasicIDPDTO(nil), errors.New("database error"))

	result, err := s.idpService.GetIdentityProviderList(context.Background())

	s.Nil(result)
	s.NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIdentityProvider_Success tests successful IDP retrieval
func (s *IDPServiceTestSuite) TestGetIdentityProvider_Success() {
	idp := &providers.IDPDTO{
		ID:          "idp-123",
		Name:        "Test IDP",
		Description: "Test Description",
		Type:        providers.IDPTypeOIDC,
	}

	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").Return(idp, nil)

	result, err := s.idpService.GetIdentityProvider(context.Background(), "idp-123")

	s.Nil(err)
	s.NotNil(result)
	s.Equal("idp-123", result.ID)
	s.Equal("Test IDP", result.Name)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIdentityProvider_EmptyID tests empty ID validation
func (s *IDPServiceTestSuite) TestGetIdentityProvider_EmptyID() {
	result, err := s.idpService.GetIdentityProvider(context.Background(), "")

	s.Nil(result)
	s.NotNil(err)
	s.Equal(ErrorInvalidIDPID.Code, err.Code)
}

// TestGetIdentityProvider_NotFound tests IDP not found
func (s *IDPServiceTestSuite) TestGetIdentityProvider_NotFound() {
	s.mockStore.On("GetIdentityProvider", mock.Anything, "non-existent").
		Return((*providers.IDPDTO)(nil), ErrIDPNotFound)

	result, err := s.idpService.GetIdentityProvider(context.Background(), "non-existent")

	s.Nil(result)
	s.NotNil(err)
	s.Equal(ErrorIDPNotFound.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIdentityProvider_StoreError tests store error handling
func (s *IDPServiceTestSuite) TestGetIdentityProvider_StoreError() {
	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").
		Return((*providers.IDPDTO)(nil), errors.New("database error"))

	result, err := s.idpService.GetIdentityProvider(context.Background(), "idp-123")

	s.Nil(result)
	s.NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIdentityProviderByName_Success tests successful IDP retrieval by name
func (s *IDPServiceTestSuite) TestGetIdentityProviderByName_Success() {
	idp := &providers.IDPDTO{
		ID:   "idp-123",
		Name: "Test IDP",
		Type: providers.IDPTypeOIDC,
	}

	s.mockStore.On("GetIdentityProviderByName", mock.Anything, "Test IDP").Return(idp, nil)

	result, err := s.idpService.GetIdentityProviderByName(context.Background(), "Test IDP")

	s.Nil(err)
	s.NotNil(result)
	s.Equal("Test IDP", result.Name)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIdentityProviderByName_EmptyName tests empty name validation
func (s *IDPServiceTestSuite) TestGetIdentityProviderByName_EmptyName() {
	result, err := s.idpService.GetIdentityProviderByName(context.Background(), "")

	s.Nil(result)
	s.NotNil(err)
	s.Equal(ErrorInvalidIDPName.Code, err.Code)
}

// TestGetIdentityProviderByName_NotFound tests IDP not found
func (s *IDPServiceTestSuite) TestGetIdentityProviderByName_NotFound() {
	s.mockStore.On("GetIdentityProviderByName", mock.Anything, "Non-existent").
		Return((*providers.IDPDTO)(nil), ErrIDPNotFound)

	result, err := s.idpService.GetIdentityProviderByName(context.Background(), "Non-existent")

	s.Nil(result)
	s.NotNil(err)
	s.Equal(ErrorIDPNotFound.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIdentityProviderByName_StoreError tests store error handling
func (s *IDPServiceTestSuite) TestGetIdentityProviderByName_StoreError() {
	s.mockStore.On("GetIdentityProviderByName", mock.Anything, "Test").
		Return((*providers.IDPDTO)(nil), errors.New("database error"))

	result, err := s.idpService.GetIdentityProviderByName(context.Background(), "Test")

	s.Nil(result)
	s.NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIdentityProvidersByProperty_Success tests successful IDP retrieval by property
func (s *IDPServiceTestSuite) TestGetIdentityProvidersByProperty_Success() {
	prop, _ := cmodels.NewProperty(PropIssuer, "https://idp.example.com", false)
	idps := []providers.IDPDTO{
		{
			ID:         "idp-123",
			Name:       "Test IDP",
			Type:       providers.IDPTypeOIDC,
			Properties: []cmodels.Property{*prop},
		},
	}

	s.mockStore.On("GetIdentityProvidersByProperty", mock.Anything, "issuer", "https://idp.example.com").
		Return(idps, nil)

	result, err := s.idpService.GetIdentityProvidersByProperty(
		context.Background(), "issuer", "https://idp.example.com")

	s.Nil(err)
	s.NotNil(result)
	s.Len(result, 1)
	s.Equal("idp-123", result[0].ID)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIdentityProvidersByProperty_EmptyKey tests empty property key validation
func (s *IDPServiceTestSuite) TestGetIdentityProvidersByProperty_EmptyKey() {
	result, err := s.idpService.GetIdentityProvidersByProperty(context.Background(), "", "some-value")

	s.Nil(result)
	s.NotNil(err)
	s.Equal(ErrorInvalidIDPID.Code, err.Code)
}

// TestGetIdentityProvidersByProperty_EmptyValue tests empty property value validation
func (s *IDPServiceTestSuite) TestGetIdentityProvidersByProperty_EmptyValue() {
	result, err := s.idpService.GetIdentityProvidersByProperty(context.Background(), "issuer", "")

	s.Nil(result)
	s.NotNil(err)
	s.Equal(ErrorInvalidIDPID.Code, err.Code)
}

// TestGetIdentityProvidersByProperty_NotFound tests IDP not found by property
func (s *IDPServiceTestSuite) TestGetIdentityProvidersByProperty_NotFound() {
	s.mockStore.On("GetIdentityProvidersByProperty", mock.Anything, "issuer", "https://unknown.example.com").
		Return([]providers.IDPDTO(nil), ErrIDPNotFound)

	result, err := s.idpService.GetIdentityProvidersByProperty(
		context.Background(), "issuer", "https://unknown.example.com")

	s.Nil(result)
	s.NotNil(err)
	s.Equal(ErrorIDPNotFound.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIdentityProvidersByProperty_StoreError tests store error handling
func (s *IDPServiceTestSuite) TestGetIdentityProvidersByProperty_StoreError() {
	s.mockStore.On("GetIdentityProvidersByProperty", mock.Anything, "issuer", "https://idp.example.com").
		Return([]providers.IDPDTO(nil), errors.New("database error"))

	result, err := s.idpService.GetIdentityProvidersByProperty(
		context.Background(), "issuer", "https://idp.example.com")

	s.Nil(result)
	s.NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestUpdateIdentityProvider_Success tests successful IDP update
func (s *IDPServiceTestSuite) TestUpdateIdentityProvider_Success() {
	idp := &providers.IDPDTO{
		Name:       "Updated IDP",
		Type:       providers.IDPTypeOIDC,
		Properties: createOIDCProperties(),
	}

	existingIDP := &providers.IDPDTO{
		ID:         "idp-123",
		Name:       "Old Name",
		Type:       providers.IDPTypeOIDC,
		Properties: createOIDCProperties(),
	}

	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").Return(existingIDP, nil)
	s.mockStore.On("GetIdentityProviderByName", mock.Anything, "Updated IDP").
		Return((*providers.IDPDTO)(nil), ErrIDPNotFound)
	s.mockStore.On("UpdateIdentityProvider", mock.Anything, mock.MatchedBy(func(dto *providers.IDPDTO) bool {
		return dto.ID == "idp-123" && dto.Name == "Updated IDP"
	})).Return(nil)

	result, err := s.idpService.UpdateIdentityProvider(context.Background(), "idp-123", idp)

	s.Nil(err)
	s.NotNil(result)
	s.Equal("idp-123", result.ID)
	s.Equal("Updated IDP", result.Name)
	s.mockStore.AssertExpectations(s.T())
}

// TestUpdateIdentityProvider_EmptyID tests empty ID validation
func (s *IDPServiceTestSuite) TestUpdateIdentityProvider_EmptyID() {
	idp := &providers.IDPDTO{Name: "Test", Type: providers.IDPTypeOIDC, Properties: createOIDCProperties()}

	result, err := s.idpService.UpdateIdentityProvider(context.Background(), "", idp)

	s.Nil(result)
	s.NotNil(err)
	s.Equal(ErrorInvalidIDPID.Code, err.Code)
}

// TestUpdateIdentityProvider_NotFound tests IDP not found
func (s *IDPServiceTestSuite) TestUpdateIdentityProvider_NotFound() {
	idp := &providers.IDPDTO{Name: "Test", Type: providers.IDPTypeOIDC, Properties: createOIDCProperties()}

	s.mockStore.On("GetIdentityProvider", mock.Anything, "non-existent").
		Return((*providers.IDPDTO)(nil), ErrIDPNotFound)

	result, err := s.idpService.UpdateIdentityProvider(context.Background(), "non-existent", idp)

	s.Nil(result)
	s.NotNil(err)
	s.Equal(ErrorIDPNotFound.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestUpdateIdentityProvider_NameConflict tests name conflict during update
func (s *IDPServiceTestSuite) TestUpdateIdentityProvider_NameConflict() {
	idp := &providers.IDPDTO{Name: "Existing Name", Type: providers.IDPTypeOIDC, Properties: createOIDCProperties()}

	existingIDP := &providers.IDPDTO{ID: "idp-123", Name: "Old Name", Type: providers.IDPTypeOIDC,
		Properties: createOIDCProperties()}
	conflictIDP := &providers.IDPDTO{ID: "idp-456", Name: "Existing Name", Type: providers.IDPTypeOIDC,
		Properties: createOIDCProperties()}

	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").Return(existingIDP, nil)
	s.mockStore.On("GetIdentityProviderByName", mock.Anything, "Existing Name").Return(conflictIDP, nil)

	result, err := s.idpService.UpdateIdentityProvider(context.Background(), "idp-123", idp)

	s.Nil(result)
	s.NotNil(err)
	s.Equal(ErrorIDPAlreadyExists.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestUpdateIdentityProvider_SameNameUpdate tests updating without changing name
func (s *IDPServiceTestSuite) TestUpdateIdentityProvider_SameNameUpdate() {
	idp := &providers.IDPDTO{Name: "Same Name", Type: providers.IDPTypeOIDC, Description: "New Description",
		Properties: createOIDCProperties()}

	existingIDP := &providers.IDPDTO{ID: "idp-123", Name: "Same Name", Type: providers.IDPTypeOIDC,
		Properties: createOIDCProperties()}

	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").Return(existingIDP, nil)
	s.mockStore.On("UpdateIdentityProvider", mock.Anything, mock.Anything).Return(nil)

	result, err := s.idpService.UpdateIdentityProvider(context.Background(), "idp-123", idp)

	s.Nil(err)
	s.NotNil(result)
	s.mockStore.AssertExpectations(s.T())
}

// TestUpdateIdentityProvider_InvalidData tests update with invalid data
func (s *IDPServiceTestSuite) TestUpdateIdentityProvider_InvalidData() {
	testCases := []struct {
		name        string
		idp         *providers.IDPDTO
		expectedErr tidcommon.ServiceError
	}{
		{
			name:        "Nil IDP",
			idp:         nil,
			expectedErr: ErrorIDPNil,
		},
		{
			name:        "Empty name",
			idp:         &providers.IDPDTO{Name: "", Type: providers.IDPTypeOIDC},
			expectedErr: ErrorInvalidIDPName,
		},
		{
			name:        "Invalid type",
			idp:         &providers.IDPDTO{Name: "Test", Type: providers.IDPType("INVALID")},
			expectedErr: ErrorInvalidIDPType,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			result, err := s.idpService.UpdateIdentityProvider(context.Background(), "idp-123", tc.idp)

			s.Nil(result)
			s.NotNil(err)
			s.Equal(tc.expectedErr.Code, err.Code)
		})
	}
}

// TestUpdateIdentityProvider_GetStoreError tests store error when checking existing IDP
func (s *IDPServiceTestSuite) TestUpdateIdentityProvider_GetStoreError() {
	idp := &providers.IDPDTO{Name: "Test", Type: providers.IDPTypeOIDC, Properties: createOIDCProperties()}

	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").
		Return((*providers.IDPDTO)(nil), errors.New("database error"))

	result, err := s.idpService.UpdateIdentityProvider(context.Background(), "idp-123", idp)

	s.Nil(result)
	s.NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestUpdateIdentityProvider_CheckNameStoreError tests store error when checking name conflict
func (s *IDPServiceTestSuite) TestUpdateIdentityProvider_CheckNameStoreError() {
	idp := &providers.IDPDTO{Name: "New Name", Type: providers.IDPTypeOIDC, Properties: createOIDCProperties()}

	existingIDP := &providers.IDPDTO{
		ID:         "idp-123",
		Name:       "Old Name",
		Type:       providers.IDPTypeOIDC,
		Properties: createOIDCProperties(),
	}

	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").Return(existingIDP, nil)
	s.mockStore.On("GetIdentityProviderByName", mock.Anything, "New Name").
		Return((*providers.IDPDTO)(nil), errors.New("database error"))

	result, err := s.idpService.UpdateIdentityProvider(context.Background(), "idp-123", idp)

	s.Nil(result)
	s.NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestUpdateIdentityProvider_StoreError tests store error during update
func (s *IDPServiceTestSuite) TestUpdateIdentityProvider_StoreError() {
	idp := &providers.IDPDTO{Name: "Test", Type: providers.IDPTypeOIDC, Properties: createOIDCProperties()}

	existingIDP := &providers.IDPDTO{
		ID:         "idp-123",
		Name:       "Test",
		Type:       providers.IDPTypeOIDC,
		Properties: createOIDCProperties(),
	}

	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").Return(existingIDP, nil)
	s.mockStore.On("UpdateIdentityProvider", mock.Anything, mock.Anything).Return(errors.New("database error"))

	result, err := s.idpService.UpdateIdentityProvider(context.Background(), "idp-123", idp)

	s.Nil(result)
	s.NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestDeleteIdentityProvider_Success tests successful IDP deletion
func (s *IDPServiceTestSuite) TestDeleteIdentityProvider_Success() {
	existingIDP := &providers.IDPDTO{ID: "idp-123", Name: "Test IDP"}

	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").Return(existingIDP, nil)
	s.mockStore.On("DeleteIdentityProvider", mock.Anything, "idp-123").Return(nil)

	err := s.idpService.DeleteIdentityProvider(context.Background(), "idp-123")

	s.Nil(err)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIDPUsages_ReturnsDependencies verifies usages are returned for an existing IDP.
func (s *IDPServiceTestSuite) TestGetIDPUsages_ReturnsDependencies() {
	total := 1
	usages := &resourcedependency.DependenciesResponse{
		TotalResults: &total,
		Count:        1,
		Usages: []resourcedependency.ResourceDependency{
			{ResourceType: resourcedependency.ResourceTypeFlow, ID: "flow-1",
				DisplayName: "Google Login", BehaviorOnDelete: resourcedependency.BehaviorRestrict},
		},
	}
	s.idpService.dependencyRegistry = &stubDependencyRegistry{resp: usages}
	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").
		Return(&providers.IDPDTO{ID: "idp-123"}, nil)

	result, err := s.idpService.GetIDPUsages(context.Background(), "idp-123")

	s.Nil(err)
	s.Equal(usages, result)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIDPUsages_EmptyID validates the empty-ID guard.
func (s *IDPServiceTestSuite) TestGetIDPUsages_EmptyID() {
	result, err := s.idpService.GetIDPUsages(context.Background(), "")

	s.Nil(result)
	s.NotNil(err)
	s.Equal(ErrorInvalidIDPID.Code, err.Code)
}

// TestGetIDPUsages_NotFound verifies a not-found error when the IDP does not exist.
func (s *IDPServiceTestSuite) TestGetIDPUsages_NotFound() {
	s.mockStore.On("GetIdentityProvider", mock.Anything, "missing").
		Return((*providers.IDPDTO)(nil), ErrIDPNotFound)

	result, err := s.idpService.GetIDPUsages(context.Background(), "missing")

	s.Nil(result)
	s.NotNil(err)
	s.Equal(ErrorIDPNotFound.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIDPUsages_GetStoreError verifies a store error while retrieving the IDP maps to an
// internal server error.
func (s *IDPServiceTestSuite) TestGetIDPUsages_GetStoreError() {
	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").
		Return((*providers.IDPDTO)(nil), errors.New("database error"))

	result, err := s.idpService.GetIDPUsages(context.Background(), "idp-123")

	s.Nil(result)
	s.NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIDPUsages_RegistryUnset returns unknown dependencies rather than failing when the
// registry was never wired in (informational endpoint, unlike deletion which fails closed).
func (s *IDPServiceTestSuite) TestGetIDPUsages_RegistryUnset() {
	s.idpService.dependencyRegistry = nil
	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").
		Return(&providers.IDPDTO{ID: "idp-123"}, nil)

	result, err := s.idpService.GetIDPUsages(context.Background(), "idp-123")

	s.Nil(err)
	s.Require().NotNil(result)
	s.Nil(result.TotalResults)
	s.Empty(result.Usages)
	s.mockStore.AssertExpectations(s.T())
}

// TestDeleteIdentityProvider_EmptyID tests empty ID validation
func (s *IDPServiceTestSuite) TestDeleteIdentityProvider_EmptyID() {
	err := s.idpService.DeleteIdentityProvider(context.Background(), "")

	s.NotNil(err)
	s.Equal(ErrorInvalidIDPID.Code, err.Code)
}

// TestDeleteIdentityProvider_NotFound tests deleting non-existent IDP
func (s *IDPServiceTestSuite) TestDeleteIdentityProvider_NotFound() {
	s.mockStore.On("GetIdentityProvider", mock.Anything, "non-existent").
		Return((*providers.IDPDTO)(nil), ErrIDPNotFound)

	err := s.idpService.DeleteIdentityProvider(context.Background(), "non-existent")

	s.Nil(err) // Delete is idempotent, returns nil for non-existent
	s.mockStore.AssertExpectations(s.T())
}

// TestDeleteIdentityProvider_GetStoreError tests store error when checking existing IDP
func (s *IDPServiceTestSuite) TestDeleteIdentityProvider_GetStoreError() {
	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").
		Return((*providers.IDPDTO)(nil), errors.New("database error"))

	err := s.idpService.DeleteIdentityProvider(context.Background(), "idp-123")

	s.NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestDeleteIdentityProvider_StoreError tests store error handling
func (s *IDPServiceTestSuite) TestDeleteIdentityProvider_StoreError() {
	existingIDP := &providers.IDPDTO{ID: "idp-123", Name: "Test IDP"}

	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").Return(existingIDP, nil)
	s.mockStore.On("DeleteIdentityProvider", mock.Anything, "idp-123").Return(errors.New("database error"))

	err := s.idpService.DeleteIdentityProvider(context.Background(), "idp-123")

	s.NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestDeleteIdentityProvider_BlockedByFlow verifies deletion is refused when a flow references the IDP.
func (s *IDPServiceTestSuite) TestDeleteIdentityProvider_BlockedByFlow() {
	total := 1
	s.idpService.dependencyRegistry = &stubDependencyRegistry{resp: &resourcedependency.DependenciesResponse{
		TotalResults: &total,
		Count:        1,
		Usages: []resourcedependency.ResourceDependency{
			{ResourceType: resourcedependency.ResourceTypeFlow, ID: "flow-1",
				DisplayName: "Google Login", BehaviorOnDelete: resourcedependency.BehaviorRestrict},
		},
	}}

	err := s.idpService.DeleteIdentityProvider(context.Background(), "idp-123")

	s.NotNil(err)
	s.Equal(ErrorIDPHasBlockingDependencies.Code, err.Code)
	s.mockStore.AssertNotCalled(s.T(), "DeleteIdentityProvider", mock.Anything, mock.Anything)
}

// TestDeleteIdentityProvider_RefusedWhenDependenciesUnknown verifies deletion fails closed when a
// provider fails to report dependency data.
func (s *IDPServiceTestSuite) TestDeleteIdentityProvider_RefusedWhenDependenciesUnknown() {
	s.idpService.dependencyRegistry = &stubDependencyRegistry{resp: &resourcedependency.DependenciesResponse{
		TotalResults: nil,
		Usages:       []resourcedependency.ResourceDependency{},
	}}

	err := s.idpService.DeleteIdentityProvider(context.Background(), "idp-123")

	s.NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
	s.mockStore.AssertNotCalled(s.T(), "DeleteIdentityProvider", mock.Anything, mock.Anything)
}

// TestDeleteIdentityProvider_RefusedWhenRegistryUnset verifies deletion fails closed when the
// dependency registry was never wired in.
func (s *IDPServiceTestSuite) TestDeleteIdentityProvider_RefusedWhenRegistryUnset() {
	s.idpService.dependencyRegistry = nil

	err := s.idpService.DeleteIdentityProvider(context.Background(), "idp-123")

	s.NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
	s.mockStore.AssertNotCalled(s.T(), "DeleteIdentityProvider", mock.Anything, mock.Anything)
}

// TestCreateIdentityProvider_DeclarativeModeEnabled tests creation is blocked when declarative mode is enabled
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_DeclarativeModeEnabled() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
	}
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)
	defer config.ResetServerRuntime()

	idp := &providers.IDPDTO{
		Name: "Test IDP",
		Type: providers.IDPTypeOIDC,
	}

	result, err := s.idpService.CreateIdentityProvider(context.Background(), idp)

	s.Nil(result)
	s.NotNil(err)
	s.Equal(declarativeresource.ErrorDeclarativeResourceCreateOperation.Code, err.Code)
}

// TestUpdateIdentityProvider_DeclarativeModeEnabled tests update is blocked when declarative mode is enabled
func (s *IDPServiceTestSuite) TestUpdateIdentityProvider_DeclarativeModeEnabled() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
	}
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)
	defer config.ResetServerRuntime()

	idp := &providers.IDPDTO{
		Name: "Updated IDP",
		Type: providers.IDPTypeOIDC,
	}

	result, err := s.idpService.UpdateIdentityProvider(context.Background(), "idp-123", idp)

	s.Nil(result)
	s.NotNil(err)
	s.Equal(declarativeresource.ErrorDeclarativeResourceUpdateOperation.Code, err.Code)
}

// TestDeleteIdentityProvider_DeclarativeModeEnabled tests deletion is blocked when declarative mode is enabled
func (s *IDPServiceTestSuite) TestDeleteIdentityProvider_DeclarativeModeEnabled() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
	}
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)
	defer config.ResetServerRuntime()

	err := s.idpService.DeleteIdentityProvider(context.Background(), "idp-123")

	s.NotNil(err)
	s.Equal(declarativeresource.ErrorDeclarativeResourceDeleteOperation.Code, err.Code)
}

// TestValidateIDP tests IDP validation
func (s *IDPServiceTestSuite) TestValidateIDP() {
	testCases := []struct {
		name        string
		idp         *providers.IDPDTO
		expectError bool
		errorCode   string
	}{
		{
			name: "Valid IDP",
			idp: &providers.IDPDTO{
				Name:       "Test",
				Type:       providers.IDPTypeOIDC,
				Properties: createOIDCProperties(),
			},
			expectError: false,
		},
		{
			name:        "Nil IDP",
			idp:         nil,
			expectError: true,
			errorCode:   ErrorIDPNil.Code,
		},
		{
			name: "Empty name",
			idp: &providers.IDPDTO{
				Name: "",
				Type: providers.IDPTypeOIDC,
			},
			expectError: true,
			errorCode:   ErrorInvalidIDPName.Code,
		},
		{
			name: "Empty type",
			idp: &providers.IDPDTO{
				Name: "Test",
				Type: providers.IDPType(""),
			},
			expectError: true,
			errorCode:   ErrorInvalidIDPType.Code,
		},
		{
			name: "Invalid type",
			idp: &providers.IDPDTO{
				Name: "Test",
				Type: providers.IDPType("INVALID"),
			},
			expectError: true,
			errorCode:   ErrorInvalidIDPType.Code,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			logger := log.GetLogger()
			err := validateIDP(context.Background(), tc.idp, logger)
			if tc.expectError {
				s.NotNil(err)
				s.Equal(tc.errorCode, err.Code)
			} else {
				s.Nil(err)
			}
		})
	}
}

// TestUpdateIdentityProvider_FailsForDeclarativeIDP verifies immutability in composite mode
func (s *IDPServiceTestSuite) TestUpdateIdentityProvider_FailsForDeclarativeIDP() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		IdentityProvider: config.IdentityProviderConfig{
			Store: "composite",
		},
	}
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	idpID := declarativeIDPTestID
	existingIDP := &providers.IDPDTO{
		ID:          idpID,
		Name:        "Declarative IDP",
		Description: "From file store",
		Type:        providers.IDPTypeOIDC,
		Properties:  createOIDCProperties(),
	}

	fileStore := newIdpStoreInterfaceMock(s.T())
	dbStore := newIdpStoreInterfaceMock(s.T())
	compositeStore := newCompositeIDPStore(fileStore, dbStore)

	dbStore.On("GetIdentityProvider", context.Background(), idpID).Return((*providers.IDPDTO)(nil), ErrIDPNotFound)
	fileStore.On("GetIdentityProvider", context.Background(), idpID).Return(existingIDP, nil)

	dbStore.On("GetIdentityProviderByName", context.Background(), "Updated Name").
		Return((*providers.IDPDTO)(nil), ErrIDPNotFound)
	fileStore.On("GetIdentityProviderByName", context.Background(), "Updated Name").
		Return((*providers.IDPDTO)(nil), ErrIDPNotFound)

	service := newIDPService(compositeStore, nil, nil, nil, nil, &mockTransactioner{})

	updatedIDP := &providers.IDPDTO{
		Name:        "Updated Name",
		Description: "Updated Description",
		Type:        providers.IDPTypeOIDC,
		Properties:  createOIDCProperties(),
	}

	result, err := service.UpdateIdentityProvider(context.Background(), idpID, updatedIDP)

	s.Nil(result)
	s.NotNil(err)
	s.Equal("IDP-1010", err.Code)
	s.Equal("Identity provider is immutable", err.Error.DefaultValue)

	config.ResetServerRuntime()
}

// TestUpdateIdentityProvider_SucceedsForMutableIDP verifies update works for DB IDPs
func (s *IDPServiceTestSuite) TestUpdateIdentityProvider_SucceedsForMutableIDP() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		IdentityProvider: config.IdentityProviderConfig{
			Store: "composite",
		},
	}
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	idpID := mutableIDPTestID
	existingIDP := &providers.IDPDTO{
		ID:          idpID,
		Name:        "Mutable IDP",
		Description: "From database",
		Type:        providers.IDPTypeOIDC,
		Properties:  createOIDCProperties(),
	}

	fileStore := newIdpStoreInterfaceMock(s.T())
	dbStore := newIdpStoreInterfaceMock(s.T())
	compositeStore := newCompositeIDPStore(fileStore, dbStore)

	fileStore.On("GetIdentityProvider", context.Background(), idpID).Return((*providers.IDPDTO)(nil), ErrIDPNotFound)
	fileStore.On("GetIdentityProviderByName", context.Background(), "Updated Name").
		Return((*providers.IDPDTO)(nil), ErrIDPNotFound)
	dbStore.On("GetIdentityProvider", context.Background(), idpID).Return(existingIDP, nil)
	dbStore.On("GetIdentityProviderByName", context.Background(), "Updated Name").
		Return((*providers.IDPDTO)(nil), ErrIDPNotFound)
	dbStore.On("UpdateIdentityProvider", context.Background(), mock.MatchedBy(func(dto *providers.IDPDTO) bool {
		return dto.ID == idpID && dto.Name == "Updated Name"
	})).Return(nil)

	service := newIDPService(compositeStore, nil, nil, nil, nil, &mockTransactioner{})

	updatedIDP := &providers.IDPDTO{
		Name:        "Updated Name",
		Description: "Updated Description",
		Type:        providers.IDPTypeOIDC,
		Properties:  createOIDCProperties(),
	}

	result, err := service.UpdateIdentityProvider(context.Background(), idpID, updatedIDP)

	s.Nil(err)
	s.NotNil(result)
	s.Equal("Updated Name", result.Name)
	config.ResetServerRuntime()
}

// TestDeleteIdentityProvider_FailsForDeclarativeIDP verifies immutability for deletes
func (s *IDPServiceTestSuite) TestDeleteIdentityProvider_FailsForDeclarativeIDP() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		IdentityProvider: config.IdentityProviderConfig{
			Store: "composite",
		},
	}
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	idpID := "declarative-idp"
	existingIDP := &providers.IDPDTO{
		ID:          idpID,
		Name:        "Declarative IDP",
		Description: "From file store",
		Type:        providers.IDPTypeOIDC,
	}

	fileStore := newIdpStoreInterfaceMock(s.T())
	dbStore := newIdpStoreInterfaceMock(s.T())
	compositeStore := newCompositeIDPStore(fileStore, dbStore)

	dbStore.On("GetIdentityProvider", context.Background(), idpID).Return((*providers.IDPDTO)(nil), ErrIDPNotFound)
	fileStore.On("GetIdentityProvider", context.Background(), idpID).Return(existingIDP, nil)

	service := newIDPService(compositeStore, nil, nil, nil, nil, &mockTransactioner{})
	service.SetDependencyRegistry(newNoBlockingDepsRegistry())

	err := service.DeleteIdentityProvider(context.Background(), idpID)

	s.NotNil(err)
	s.Equal("IDP-1010", err.Code)
	s.Equal("Identity provider is immutable", err.Error.DefaultValue)

	config.ResetServerRuntime()
}

// TestDeleteIdentityProvider_SucceedsForMutableIDP verifies delete works for DB IDPs
func (s *IDPServiceTestSuite) TestDeleteIdentityProvider_SucceedsForMutableIDP() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		IdentityProvider: config.IdentityProviderConfig{
			Store: "composite",
		},
	}
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	idpID := "mutable-idp"
	existingIDP := &providers.IDPDTO{
		ID:          idpID,
		Name:        "Mutable IDP",
		Description: "From database",
		Type:        providers.IDPTypeOIDC,
	}

	fileStore := newIdpStoreInterfaceMock(s.T())
	dbStore := newIdpStoreInterfaceMock(s.T())
	compositeStore := newCompositeIDPStore(fileStore, dbStore)

	fileStore.On("GetIdentityProvider", context.Background(), idpID).Return((*providers.IDPDTO)(nil), ErrIDPNotFound)
	dbStore.On("GetIdentityProvider", context.Background(), idpID).Return(existingIDP, nil)
	dbStore.On("DeleteIdentityProvider", context.Background(), idpID).Return(nil)

	service := newIDPService(compositeStore, nil, nil, nil, nil, &mockTransactioner{})
	service.SetDependencyRegistry(newNoBlockingDepsRegistry())

	err := service.DeleteIdentityProvider(context.Background(), idpID)

	s.Nil(err)
	dbStore.AssertCalled(s.T(), "DeleteIdentityProvider", context.Background(), idpID)

	config.ResetServerRuntime()
}

// singleProfileMapping builds an attribute configuration that resolves to userType with a single
// user-type-attributes entry carrying the given claim mappings.
func singleProfileMapping(userType string, mappings []providers.AttributeMapping) *providers.AttributeConfiguration {
	return &providers.AttributeConfiguration{
		UserTypeResolution:        &providers.UserTypeResolution{Default: userType},
		UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{{UserType: userType, Attributes: mappings}},
	}
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_NilMapping_OK() {
	svcErr := s.idpService.validateAttributeConfiguration(context.Background(), &providers.IDPDTO{})
	s.Nil(svcErr)
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_AccountLinkingOnly_NoUserTypeResolutionRequired() {
	idp := &providers.IDPDTO{AttributeConfiguration: &providers.AttributeConfiguration{
		AccountLinking: &providers.AccountLinking{Attributes: []string{"email"}},
	}}
	s.Nil(s.idpService.validateAttributeConfiguration(context.Background(), idp))
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_Valid() {
	s.mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "person",
		entitytype.AttributeFilter{AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "firstName"}, {Attribute: "email"}},
			(*tidcommon.ServiceError)(nil))

	idp := &providers.IDPDTO{AttributeConfiguration: singleProfileMapping("person", []providers.AttributeMapping{
		{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
		{ExternalAttribute: "address.email", LocalAttribute: "email"},
	})}

	s.Nil(s.idpService.validateAttributeConfiguration(context.Background(), idp))
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_EmptyEntityType() {
	idp := &providers.IDPDTO{AttributeConfiguration: &providers.AttributeConfiguration{
		UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{{
			Attributes: []providers.AttributeMapping{{ExternalAttribute: "given_name", LocalAttribute: "firstName"}},
		}},
	}}
	svcErr := s.idpService.validateAttributeConfiguration(context.Background(), idp)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidAttributeConfiguration.Code, svcErr.Code)
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_EmptyMappings() {
	s.mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "person",
		entitytype.AttributeFilter{AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "firstName"}}, (*tidcommon.ServiceError)(nil))
	idp := &providers.IDPDTO{AttributeConfiguration: singleProfileMapping("person", nil)}
	svcErr := s.idpService.validateAttributeConfiguration(context.Background(), idp)
	s.Nil(svcErr)
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_OneSourceToMultipleTargets() {
	s.mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "person",
		entitytype.AttributeFilter{AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "email"}, {Attribute: "contactEmail"}},
			(*tidcommon.ServiceError)(nil))

	idp := &providers.IDPDTO{AttributeConfiguration: singleProfileMapping("person", []providers.AttributeMapping{
		{ExternalAttribute: "email", LocalAttribute: "email"},
		{ExternalAttribute: "email", LocalAttribute: "contactEmail"},
	})}

	s.Nil(s.idpService.validateAttributeConfiguration(context.Background(), idp))
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_DuplicateTarget() {
	idp := &providers.IDPDTO{AttributeConfiguration: singleProfileMapping("person", []providers.AttributeMapping{
		{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
		{ExternalAttribute: "first_name", LocalAttribute: "firstName"},
	})}
	svcErr := s.idpService.validateAttributeConfiguration(context.Background(), idp)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidAttributeConfiguration.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.DefaultValue, "more than once")
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_DuplicateTargetWhitespaceVariant() {
	idp := &providers.IDPDTO{AttributeConfiguration: singleProfileMapping("person", []providers.AttributeMapping{
		{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
		{ExternalAttribute: "first_name", LocalAttribute: "  firstName  "},
	})}
	svcErr := s.idpService.validateAttributeConfiguration(context.Background(), idp)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidAttributeConfiguration.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.DefaultValue, "more than once")
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_DuplicateEntityType() {
	s.mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "person",
		entitytype.AttributeFilter{AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "firstName"}}, (*tidcommon.ServiceError)(nil))
	idp := &providers.IDPDTO{AttributeConfiguration: &providers.AttributeConfiguration{
		UserTypeResolution: &providers.UserTypeResolution{Default: "person"},
		UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{
			{
				UserType: "person",
				Attributes: []providers.AttributeMapping{
					{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
				},
			},
			{
				UserType: "person",
				Attributes: []providers.AttributeMapping{
					{ExternalAttribute: "family_name", LocalAttribute: "lastName"},
				},
			},
		},
	}}
	svcErr := s.idpService.validateAttributeConfiguration(context.Background(), idp)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidAttributeConfiguration.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.DefaultValue, "configured more than once")
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_TargetNotInSchema() {
	s.mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "person",
		entitytype.AttributeFilter{AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "email"}}, (*tidcommon.ServiceError)(nil))

	idp := &providers.IDPDTO{AttributeConfiguration: singleProfileMapping("person", []providers.AttributeMapping{
		{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
	})}
	svcErr := s.idpService.validateAttributeConfiguration(context.Background(), idp)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidAttributeConfiguration.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.DefaultValue, "not an attribute")
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_UnknownEntityType() {
	s.mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "ghost",
		entitytype.AttributeFilter{AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo(nil), &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType, Code: "ETS-1004",
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "user type not found"},
		})

	idp := &providers.IDPDTO{AttributeConfiguration: singleProfileMapping("ghost", []providers.AttributeMapping{
		{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
	})}
	svcErr := s.idpService.validateAttributeConfiguration(context.Background(), idp)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidAttributeConfiguration.Code, svcErr.Code)
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_DynamicResolutionValid() {
	s.mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "employee",
		entitytype.AttributeFilter{AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo{{Attribute: "firstName"}}, (*tidcommon.ServiceError)(nil))

	idp := &providers.IDPDTO{AttributeConfiguration: &providers.AttributeConfiguration{
		UserTypeResolution: &providers.UserTypeResolution{
			Default:           "person",
			ExternalAttribute: "user_type",
			ValueMapping:      map[string]string{"staff": "employee"},
		},
	}}
	s.Nil(s.idpService.validateAttributeConfiguration(context.Background(), idp))
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_ExternalAttributeWithoutMapping_OK() {
	// An external attribute may be configured on its own; every identity resolves to Default until
	// value mappings are added later.
	idp := &providers.IDPDTO{AttributeConfiguration: &providers.AttributeConfiguration{
		UserTypeResolution: &providers.UserTypeResolution{
			Default:           "person",
			ExternalAttribute: "user_type",
		},
	}}
	s.Nil(s.idpService.validateAttributeConfiguration(context.Background(), idp))
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_MappingWithoutExternalAttribute() {
	idp := &providers.IDPDTO{AttributeConfiguration: &providers.AttributeConfiguration{
		UserTypeResolution: &providers.UserTypeResolution{
			Default:      "person",
			ValueMapping: map[string]string{"staff": "employee"},
		},
	}}
	svcErr := s.idpService.validateAttributeConfiguration(context.Background(), idp)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidAttributeConfiguration.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.DefaultValue, "requires an external attribute")
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_DynamicResolutionDefaultRequired() {
	idp := &providers.IDPDTO{AttributeConfiguration: &providers.AttributeConfiguration{
		UserTypeResolution: &providers.UserTypeResolution{
			ExternalAttribute: "user_type",
			ValueMapping:      map[string]string{"staff": "employee"},
		},
	}}
	svcErr := s.idpService.validateAttributeConfiguration(context.Background(), idp)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidAttributeConfiguration.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.DefaultValue, "default user type")
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_DynamicResolutionEmptyMapping() {
	idp := &providers.IDPDTO{AttributeConfiguration: &providers.AttributeConfiguration{
		UserTypeResolution: &providers.UserTypeResolution{
			Default:           "person",
			ExternalAttribute: "user_type",
			ValueMapping:      map[string]string{"staff": ""},
		},
	}}
	svcErr := s.idpService.validateAttributeConfiguration(context.Background(), idp)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidAttributeConfiguration.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.DefaultValue, "must not contain empty")
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_DynamicResolutionInvalidTarget() {
	s.mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "ghost",
		entitytype.AttributeFilter{AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo(nil), &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType, Code: "ETS-1004",
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "user type not found"},
		})

	idp := &providers.IDPDTO{AttributeConfiguration: &providers.AttributeConfiguration{
		UserTypeResolution: &providers.UserTypeResolution{
			Default:           "person",
			ExternalAttribute: "user_type",
			ValueMapping:      map[string]string{"staff": "ghost"},
		},
	}}
	svcErr := s.idpService.validateAttributeConfiguration(context.Background(), idp)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidAttributeConfiguration.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.DefaultValue, "invalid user type")
}

// --- ApplySchemaAwareDefaults ---

const seedUserType = "Person"

// newSeedingService builds a service with a dedicated entity-type mock, bypassing the suite-level
// catch-all so each case controls exactly what the schema looks like.
func (s *IDPServiceTestSuite) newSeedingService() (
	*idpService, *entitytypemock.EntityTypeServiceInterfaceMock) {
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())

	return &idpService{
		entityTypeService: mockET,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "IdPService")),
	}, mockET
}

// seedTestIDP builds a connection carrying only the scopes, the one property seeding reads.
func seedTestIDP(idpType providers.IDPType, scopes string) *providers.IDPDTO {
	scopesProp, _ := cmodels.NewProperty(PropScopes, scopes, false)

	return &providers.IDPDTO{
		Name:       "Test " + string(idpType),
		Type:       idpType,
		Properties: []cmodels.Property{*scopesProp},
	}
}

// expectUserTypes stubs the user-type listing seeding uses to find candidates.
func expectUserTypes(mockET *entitytypemock.EntityTypeServiceInterfaceMock,
	types ...entitytype.EntityTypeListItem) {
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser,
		mock.Anything, mock.Anything, mock.Anything).
		Return(&entitytype.EntityTypeListResponse{Types: types}, nil)
}

// expectSchemaFor stubs one user type's schema. Optional, because seeding stops before reading the
// schema whenever the connection or the candidate set already rules the defaults out.
func expectSchemaFor(mockET *entitytypemock.EntityTypeServiceInterfaceMock,
	userType string, unique []string, required []string) {
	uniqueSet := make(map[string]bool, len(unique))
	for _, name := range unique {
		uniqueSet[name] = true
	}
	requiredSet := make(map[string]bool, len(required))
	for _, name := range required {
		requiredSet[name] = true
	}

	attrs := make([]entitytype.AttributeInfo, 0, len(unique)+len(required))
	seen := make(map[string]bool, len(unique)+len(required))
	add := func(names []string) {
		for _, name := range names {
			if seen[name] {
				continue
			}
			seen[name] = true
			attrs = append(attrs, entitytype.AttributeInfo{
				Attribute: name,
				Unique:    uniqueSet[name],
				Required:  requiredSet[name],
			})
		}
	}
	add(unique)
	add(required)

	mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, userType,
		entitytype.AttributeFilter{AllowNonCredential: true}).
		Return(attrs, nil).Maybe()
}

// The whole point of the feature: a fresh Google, OIDC or GitHub connection links on email and maps
// its provider-specific claim onto the required username without the administrator configuring either.
func (s *IDPServiceTestSuite) TestApplySchemaAwareDefaults_SeedsLinkingAndMapping() {
	testCases := []struct {
		idpType        providers.IDPType
		scopes         string
		expectedSource string
	}{
		{idpType: providers.IDPTypeGoogle, scopes: "openid,email,profile", expectedSource: "email"},
		{idpType: providers.IDPTypeOIDC, scopes: "openid,email,profile", expectedSource: "email"},
		{idpType: providers.IDPTypeGitHub, scopes: "user:email", expectedSource: "login"},
	}

	for _, tc := range testCases {
		s.Run(string(tc.idpType), func() {
			service, mockET := s.newSeedingService()
			expectUserTypes(mockET, entitytype.EntityTypeListItem{Name: seedUserType})
			expectSchemaFor(mockET, seedUserType, []string{"username", "email"}, []string{"username", "email"})

			idp := seedTestIDP(tc.idpType, tc.scopes)
			service.ApplySchemaAwareDefaults(context.Background(), idp)

			s.Require().NotNil(idp.AttributeConfiguration)
			s.Require().NotNil(idp.AttributeConfiguration.AccountLinking)
			s.Equal([]string{"email"}, idp.AttributeConfiguration.AccountLinking.Attributes)

			s.Require().Len(idp.AttributeConfiguration.UserTypeAttributeMappings, 1)
			entry := idp.AttributeConfiguration.UserTypeAttributeMappings[0]
			s.Equal(seedUserType, entry.UserType)
			s.Require().Len(entry.Attributes, 1)
			s.Equal(tc.expectedSource, entry.Attributes[0].ExternalAttribute)
			s.Equal("username", entry.Attributes[0].LocalAttribute)
			// Mappings are rejected without a resolution default, so one is seeded alongside them.
			s.Require().NotNil(idp.AttributeConfiguration.UserTypeResolution)
			s.Equal(seedUserType, idp.AttributeConfiguration.UserTypeResolution.Default)
		})
	}
}

// The reported scenario: two self-registerable types, email unique on both, and only one requiring
// a username. Linking is seeded from both, and the type requiring a username becomes the mapping
// target, even though it is neither first nor the one with the fewest required attributes.
func (s *IDPServiceTestSuite) TestApplySchemaAwareDefaults_ResolvesWhenOneTypeRequiresUsername() {
	service, mockET := s.newSeedingService()
	expectUserTypes(mockET,
		entitytype.EntityTypeListItem{Name: "Guest", AllowSelfRegistration: true},
		entitytype.EntityTypeListItem{Name: seedUserType, AllowSelfRegistration: true})
	expectSchemaFor(mockET, "Guest", []string{"email"}, []string{"email"})
	expectSchemaFor(mockET, seedUserType, []string{"email"}, []string{"username", "email"})

	idp := seedTestIDP(providers.IDPTypeGoogle, "openid,email,profile")
	service.ApplySchemaAwareDefaults(context.Background(), idp)

	s.Require().NotNil(idp.AttributeConfiguration)
	s.Require().NotNil(idp.AttributeConfiguration.AccountLinking)
	s.Equal([]string{"email"}, idp.AttributeConfiguration.AccountLinking.Attributes)

	s.Require().Len(idp.AttributeConfiguration.UserTypeAttributeMappings, 1)
	s.Equal(seedUserType, idp.AttributeConfiguration.UserTypeAttributeMappings[0].UserType)
	// The default must name a type that has a mapping, or GetAttributeMappings finds nothing.
	s.Require().NotNil(idp.AttributeConfiguration.UserTypeResolution)
	s.Equal(seedUserType, idp.AttributeConfiguration.UserTypeResolution.Default)
}

// Several types requiring a username all get a mapping, and the first is taken as the resolution
// default. The listing is ordered by name, so the choice is stable rather than arbitrary.
func (s *IDPServiceTestSuite) TestApplySchemaAwareDefaults_MapsEveryTypeRequiringUsername() {
	service, mockET := s.newSeedingService()
	expectUserTypes(mockET,
		entitytype.EntityTypeListItem{Name: "Employee", AllowSelfRegistration: true},
		entitytype.EntityTypeListItem{Name: "Guest", AllowSelfRegistration: true},
		entitytype.EntityTypeListItem{Name: seedUserType, AllowSelfRegistration: true})
	expectSchemaFor(mockET, "Employee", []string{"email"}, []string{"username", "email"})
	expectSchemaFor(mockET, "Guest", []string{"email"}, []string{"email"})
	expectSchemaFor(mockET, seedUserType, []string{"email"}, []string{"username", "email"})

	idp := seedTestIDP(providers.IDPTypeGoogle, "openid,email,profile")
	service.ApplySchemaAwareDefaults(context.Background(), idp)

	s.Require().NotNil(idp.AttributeConfiguration)
	s.NotNil(idp.AttributeConfiguration.AccountLinking)

	mapped := make([]string, 0, 2)
	for _, entry := range idp.AttributeConfiguration.UserTypeAttributeMappings {
		mapped = append(mapped, entry.UserType)
		s.Equal("email", entry.Attributes[0].ExternalAttribute)
		s.Equal("username", entry.Attributes[0].LocalAttribute)
	}
	// Guest requires no username, so it is left out.
	s.Equal([]string{"Employee", seedUserType}, mapped)

	// The first requiring type becomes the default, and it must be one that has a mapping.
	s.Require().NotNil(idp.AttributeConfiguration.UserTypeResolution)
	s.Equal("Employee", idp.AttributeConfiguration.UserTypeResolution.Default)
}

// Email must be resolvable to a single user whichever type an identity provisions into.
func (s *IDPServiceTestSuite) TestApplySchemaAwareDefaults_SkipsLinkingWhenACandidateLacksUniqueEmail() {
	service, mockET := s.newSeedingService()
	expectUserTypes(mockET,
		entitytype.EntityTypeListItem{Name: seedUserType, AllowSelfRegistration: true},
		entitytype.EntityTypeListItem{Name: "Guest", AllowSelfRegistration: true})
	expectSchemaFor(mockET, seedUserType, []string{"email"}, []string{"username", "email"})
	expectSchemaFor(mockET, "Guest", []string{"username"}, []string{"email"})

	idp := seedTestIDP(providers.IDPTypeGoogle, "openid,email,profile")
	service.ApplySchemaAwareDefaults(context.Background(), idp)

	if idp.AttributeConfiguration != nil {
		s.Nil(idp.AttributeConfiguration.AccountLinking)
	}
}

// With nothing a federated user could be provisioned into, there is no schema to derive defaults from.
func (s *IDPServiceTestSuite) TestApplySchemaAwareDefaults_SkipsWhenNothingQualifies() {
	testCases := []struct {
		name    string
		types   []entitytype.EntityTypeListItem
		schemas func(*entitytypemock.EntityTypeServiceInterfaceMock)
	}{
		{name: "no user types", types: nil},
		{
			name:  "no type offers a unique email or requires a username",
			types: []entitytype.EntityTypeListItem{{Name: "Person"}, {Name: "Partner"}},
			schemas: func(mockET *entitytypemock.EntityTypeServiceInterfaceMock) {
				expectSchemaFor(mockET, "Person", []string{"username"}, nil)
				expectSchemaFor(mockET, "Partner", []string{"username"}, nil)
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			service, mockET := s.newSeedingService()
			expectUserTypes(mockET, tc.types...)
			if tc.schemas != nil {
				tc.schemas(mockET)
			}

			idp := seedTestIDP(providers.IDPTypeGoogle, "openid,email,profile")
			service.ApplySchemaAwareDefaults(context.Background(), idp)

			s.Nil(idp.AttributeConfiguration)
		})
	}
}

// Mappings are read on login as well as during provisioning, so a type that does not allow self
// registration still gets one. Its users may be created manually and still sign in federated.
func (s *IDPServiceTestSuite) TestApplySchemaAwareDefaults_SeedsTypeWithoutSelfRegistration() {
	service, mockET := s.newSeedingService()
	expectUserTypes(mockET, entitytype.EntityTypeListItem{Name: seedUserType, AllowSelfRegistration: false})
	expectSchemaFor(mockET, seedUserType, []string{"username", "email"}, []string{"username", "email"})

	idp := seedTestIDP(providers.IDPTypeGoogle, "openid,email,profile")
	service.ApplySchemaAwareDefaults(context.Background(), idp)

	s.Require().NotNil(idp.AttributeConfiguration)
	s.Require().NotNil(idp.AttributeConfiguration.AccountLinking)
	s.Equal([]string{"email"}, idp.AttributeConfiguration.AccountLinking.Attributes)
	s.Require().Len(idp.AttributeConfiguration.UserTypeAttributeMappings, 1)
	s.Equal(seedUserType, idp.AttributeConfiguration.UserTypeAttributeMappings[0].UserType)
}

// Linking on a non-unique attribute cannot resolve a single user, so it is not seeded.
func (s *IDPServiceTestSuite) TestApplySchemaAwareDefaults_SkipsLinkingWhenEmailIsNotUnique() {
	service, mockET := s.newSeedingService()
	expectUserTypes(mockET, entitytype.EntityTypeListItem{Name: seedUserType})
	expectSchemaFor(mockET, seedUserType, []string{"username"}, []string{"email"})

	idp := seedTestIDP(providers.IDPTypeGoogle, "openid,email,profile")
	service.ApplySchemaAwareDefaults(context.Background(), idp)

	if idp.AttributeConfiguration != nil {
		s.Nil(idp.AttributeConfiguration.AccountLinking)
	}
}

// Linking on an attribute the connection never returns would match nothing, so it is worse than no default.
func (s *IDPServiceTestSuite) TestApplySchemaAwareDefaults_SkipsLinkingWhenScopesCannotYieldEmail() {
	service, mockET := s.newSeedingService()
	expectUserTypes(mockET, entitytype.EntityTypeListItem{Name: seedUserType})
	expectSchemaFor(mockET, seedUserType, []string{"email"}, []string{"email"})

	idp := seedTestIDP(providers.IDPTypeGoogle, "openid,profile")
	service.ApplySchemaAwareDefaults(context.Background(), idp)

	if idp.AttributeConfiguration != nil {
		s.Nil(idp.AttributeConfiguration.AccountLinking)
	}
}

// No username requirement means no prompt to avoid, so nothing is mapped.
// Nothing needs a derived username, so no mapping is seeded. The identity still has to resolve to
// a user type, and that default is taken from the types email can match so it agrees with linking.
func (s *IDPServiceTestSuite) TestApplySchemaAwareDefaults_DefaultsToEmailTypeWhenUsernameIsOptional() {
	service, mockET := s.newSeedingService()
	expectUserTypes(mockET, entitytype.EntityTypeListItem{Name: seedUserType})
	expectSchemaFor(mockET, seedUserType, []string{"email"}, []string{"email"})

	idp := seedTestIDP(providers.IDPTypeGoogle, "openid,email,profile")
	service.ApplySchemaAwareDefaults(context.Background(), idp)

	s.Require().NotNil(idp.AttributeConfiguration)
	s.Empty(idp.AttributeConfiguration.UserTypeAttributeMappings)
	s.Require().NotNil(idp.AttributeConfiguration.AccountLinking)
	s.Equal([]string{"email"}, idp.AttributeConfiguration.AccountLinking.Attributes)
	s.Require().NotNil(idp.AttributeConfiguration.UserTypeResolution)
	s.Equal(seedUserType, idp.AttributeConfiguration.UserTypeResolution.Default)
}

// The default has to name a type email can actually identify a single user on, so a candidate without
// a unique email is passed over rather than taken just for being first.
func (s *IDPServiceTestSuite) TestApplySchemaAwareDefaults_DefaultSkipsTypeWithoutUniqueEmail() {
	service, mockET := s.newSeedingService()
	expectUserTypes(mockET,
		entitytype.EntityTypeListItem{Name: "Guest", AllowSelfRegistration: true},
		entitytype.EntityTypeListItem{Name: seedUserType, AllowSelfRegistration: true})
	expectSchemaFor(mockET, "Guest", []string{"phone"}, []string{"phone"})
	expectSchemaFor(mockET, seedUserType, []string{"email"}, []string{"email"})

	idp := seedTestIDP(providers.IDPTypeGoogle, "openid,email,profile")
	service.ApplySchemaAwareDefaults(context.Background(), idp)

	s.Require().NotNil(idp.AttributeConfiguration)
	s.Empty(idp.AttributeConfiguration.UserTypeAttributeMappings)
	// Guest has no unique email, so linking stays unseeded, but the default can still name Person.
	s.Nil(idp.AttributeConfiguration.AccountLinking)
	s.Require().NotNil(idp.AttributeConfiguration.UserTypeResolution)
	s.Equal(seedUserType, idp.AttributeConfiguration.UserTypeResolution.Default)
}

// No candidate can be matched on email and none needs a username, so there is nothing to derive.
func (s *IDPServiceTestSuite) TestApplySchemaAwareDefaults_SeedsNothingWhenNoTypeHasEmail() {
	service, mockET := s.newSeedingService()
	expectUserTypes(mockET, entitytype.EntityTypeListItem{Name: seedUserType})
	expectSchemaFor(mockET, seedUserType, []string{"phone"}, []string{"phone"})

	idp := seedTestIDP(providers.IDPTypeGoogle, "openid,email,profile")
	service.ApplySchemaAwareDefaults(context.Background(), idp)

	s.Nil(idp.AttributeConfiguration)
}

// The mapping's source is the email claim, which arrives only when the scopes ask for it. Seeding it
// anyway would leave an entry resolving to nothing while the connection looked configured.
func (s *IDPServiceTestSuite) TestApplySchemaAwareDefaults_SkipsMappingWhenScopesCannotYieldEmail() {
	service, mockET := s.newSeedingService()
	expectUserTypes(mockET, entitytype.EntityTypeListItem{Name: seedUserType})
	expectSchemaFor(mockET, seedUserType, []string{"email"}, []string{"username", "email"})

	idp := seedTestIDP(providers.IDPTypeGoogle, "openid,profile")
	service.ApplySchemaAwareDefaults(context.Background(), idp)

	s.Nil(idp.AttributeConfiguration)
}

// GitHub takes its username from the login claim in the profile, so no email scope is involved.
func (s *IDPServiceTestSuite) TestApplySchemaAwareDefaults_GitHubMapsLoginWithoutEmailScope() {
	service, mockET := s.newSeedingService()
	expectUserTypes(mockET, entitytype.EntityTypeListItem{Name: seedUserType})
	expectSchemaFor(mockET, seedUserType, []string{"email"}, []string{"username", "email"})

	idp := seedTestIDP(providers.IDPTypeGitHub, "read:user")
	service.ApplySchemaAwareDefaults(context.Background(), idp)

	s.Require().NotNil(idp.AttributeConfiguration)
	s.Require().Len(idp.AttributeConfiguration.UserTypeAttributeMappings, 1)
	s.Equal("login", idp.AttributeConfiguration.UserTypeAttributeMappings[0].Attributes[0].ExternalAttribute)
	// read:user grants no email, so linking is withheld while the login mapping still applies.
	s.Nil(idp.AttributeConfiguration.AccountLinking)
}

// A claim-driven resolution the administrator configured must survive the default being filled in.
func (s *IDPServiceTestSuite) TestApplySchemaAwareDefaults_PreservesClaimDrivenResolution() {
	service, mockET := s.newSeedingService()
	expectUserTypes(mockET, entitytype.EntityTypeListItem{Name: seedUserType})
	expectSchemaFor(mockET, seedUserType, []string{"email"}, []string{"username", "email"})

	idp := seedTestIDP(providers.IDPTypeGoogle, "openid,email,profile")
	idp.AttributeConfiguration = &providers.AttributeConfiguration{
		UserTypeResolution: &providers.UserTypeResolution{
			ExternalAttribute: "org",
			ValueMapping:      map[string]string{"acme": seedUserType},
		},
	}

	service.ApplySchemaAwareDefaults(context.Background(), idp)

	resolution := idp.AttributeConfiguration.UserTypeResolution
	s.Require().NotNil(resolution)
	s.Equal("org", resolution.ExternalAttribute)
	s.Equal(map[string]string{"acme": seedUserType}, resolution.ValueMapping)
	s.Equal(seedUserType, resolution.Default)
}

// Generic OAuth carries no scope or claim semantics ThunderID can infer.
func (s *IDPServiceTestSuite) TestApplySchemaAwareDefaults_LeavesGenericOAuthAlone() {
	service, mockET := s.newSeedingService()
	expectUserTypes(mockET, entitytype.EntityTypeListItem{Name: seedUserType})
	expectSchemaFor(mockET, seedUserType, []string{"username", "email"}, []string{"username", "email"})

	idp := seedTestIDP(providers.IDPTypeOAuth, "email")
	service.ApplySchemaAwareDefaults(context.Background(), idp)

	if idp.AttributeConfiguration != nil {
		s.Nil(idp.AttributeConfiguration.AccountLinking)
		s.Empty(idp.AttributeConfiguration.UserTypeAttributeMappings)
	}
}

// Fully explicit configuration is left alone, and short-circuits before any schema is read: the
// mock carries no expectations, so any call to it fails the test.
func (s *IDPServiceTestSuite) TestApplySchemaAwareDefaults_PreservesExplicitConfiguration() {
	service, _ := s.newSeedingService()

	idp := seedTestIDP(providers.IDPTypeGoogle, "openid,email,profile")
	idp.AttributeConfiguration = &providers.AttributeConfiguration{
		AccountLinking:     &providers.AccountLinking{Attributes: []string{"phone_number"}},
		UserTypeResolution: &providers.UserTypeResolution{Default: "Employee"},
		UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{{
			UserType:   "Employee",
			Attributes: []providers.AttributeMapping{{ExternalAttribute: "sub", LocalAttribute: "username"}},
		}},
	}

	service.ApplySchemaAwareDefaults(context.Background(), idp)

	s.Equal([]string{"phone_number"}, idp.AttributeConfiguration.AccountLinking.Attributes)
	s.Len(idp.AttributeConfiguration.UserTypeAttributeMappings, 1)
	s.Equal("Employee", idp.AttributeConfiguration.UserTypeResolution.Default)
}

// A transient read failure must not block creating a connection.
func (s *IDPServiceTestSuite) TestApplySchemaAwareDefaults_SkipsWhenUserTypesCannotBeRead() {
	service, mockET := s.newSeedingService()
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser,
		mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &tidcommon.InternalServerError)

	idp := seedTestIDP(providers.IDPTypeGoogle, "openid,email,profile")
	service.ApplySchemaAwareDefaults(context.Background(), idp)

	s.Nil(idp.AttributeConfiguration)
}

// Seeding is best-effort and runs on paths that cannot handle a panic: a nil connection or a service
// built without an entity-type dependency must leave the connection untouched rather than crash.
func (s *IDPServiceTestSuite) TestApplySchemaAwareDefaults_ToleratesNilInputs() {
	service, _ := s.newSeedingService()
	service.ApplySchemaAwareDefaults(context.Background(), nil)

	bare := &idpService{logger: log.GetLogger()}
	idp := seedTestIDP(providers.IDPTypeGoogle, "openid,email,profile")
	bare.ApplySchemaAwareDefaults(context.Background(), idp)
	s.Nil(idp.AttributeConfiguration)
}

// An update replaces the whole connection, so a section the administrator removed is
// indistinguishable from one that was never configured. Seeding on update would silently restore it,
// making the removal appear to succeed and then revert.
func (s *IDPServiceTestSuite) TestUpdateIdentityProvider_DoesNotReSeedRemovedDefaults() {
	idpID := mutableIDPTestID
	existing := &providers.IDPDTO{
		ID:         idpID,
		Name:       "Google",
		Type:       providers.IDPTypeGoogle,
		Properties: createOIDCProperties(),
		AttributeConfiguration: &providers.AttributeConfiguration{
			AccountLinking: &providers.AccountLinking{Attributes: []string{defaultAccountLinkingAttribute}},
		},
	}

	// The name is unchanged, so the uniqueness lookup is not reached.
	s.mockStore.On("GetIdentityProvider", mock.Anything, idpID).Return(existing, nil)
	s.mockStore.On("UpdateIdentityProvider", mock.Anything, mock.MatchedBy(func(dto *providers.IDPDTO) bool {
		return dto.AttributeConfiguration == nil
	})).Return(nil)

	// A schema that would seed both defaults if the update path applied them, so this test fails if
	// seeding is ever reintroduced here rather than passing because there was nothing to seed.
	// Permitted but not required: if the update path reads them, seeding would populate the section
	// and the assertion below fails.
	mockET := entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	mockET.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser,
		mock.Anything, mock.Anything, mock.Anything).
		Return(&entitytype.EntityTypeListResponse{Types: []entitytype.EntityTypeListItem{
			{Name: seedUserType, AllowSelfRegistration: true},
		}}, nil).Maybe()
	expectSchemaFor(mockET, seedUserType, []string{"email"}, []string{"username", "email"})
	service := &idpService{
		idpStore:           s.mockStore,
		transactioner:      &mockTransactioner{},
		dependencyRegistry: newNoBlockingDepsRegistry(),
		entityTypeService:  mockET,
		logger:             log.GetLogger().With(log.String(log.LoggerKeyComponentName, "IdPService")),
		uuidGenerator:      utils.GenerateUUIDv7,
	}

	// The administrator saves the connection with the account-linking section removed.
	cleared := &providers.IDPDTO{
		Name:       "Google",
		Type:       providers.IDPTypeGoogle,
		Properties: createOIDCProperties(),
	}

	result, err := service.UpdateIdentityProvider(context.Background(), idpID, cleared)

	s.Nil(err)
	s.Require().NotNil(result)
	s.Nil(result.AttributeConfiguration, "a removed section must stay removed after saving")
}

// authorizationMappingIDP builds a minimal IDP carrying one authorization mapping value with a
// single target, for the existence-check tests below.
func authorizationMappingIDP(target providers.AuthorizationTarget) *providers.IDPDTO {
	return &providers.IDPDTO{
		Name:       "Authz Mapping Test IDP",
		Type:       providers.IDPTypeOIDC,
		Properties: createOIDCProperties(),
		AttributeConfiguration: &providers.AttributeConfiguration{
			AuthorizationMappings: []providers.AuthorizationMapping{
				{
					Claim: "groups",
					Values: []providers.AuthorizationRule{
						{
							Operator: providers.AuthorizationOperatorEquals,
							Value:    "platform-admins",
							Targets:  []providers.AuthorizationTarget{target},
						},
					},
				},
			},
		},
	}
}

// TestCreateIdentityProvider_AuthorizationMapping_RoleNotFound rejects a mapping that names a role
// that does not exist.
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_AuthorizationMapping_RoleNotFound() {
	s.mockRole.On("GetRoleWithPermissions", mock.Anything, "missing-role-id").
		Return((*role.RoleWithPermissions)(nil), &role.ErrorRoleNotFound)

	idp := authorizationMappingIDP(providers.AuthorizationTarget{
		Type: providers.AuthorizationTargetRole, ID: "missing-role-id",
	})

	result, err := s.idpService.CreateIdentityProvider(context.Background(), idp)

	s.Nil(result)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidAttributeConfiguration.Code, err.Code)
}

// TestCreateIdentityProvider_AuthorizationMapping_RoleServiceServerErrorPropagates ensures a genuine
// server error from the role service is surfaced as-is, not folded into a client "not found" error.
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_AuthorizationMapping_RoleServiceServerErrorPropagates() {
	s.mockRole.On("GetRoleWithPermissions", mock.Anything, "role-id").
		Return((*role.RoleWithPermissions)(nil), &tidcommon.InternalServerError)

	idp := authorizationMappingIDP(providers.AuthorizationTarget{
		Type: providers.AuthorizationTargetRole, ID: "role-id",
	})

	result, err := s.idpService.CreateIdentityProvider(context.Background(), idp)

	s.Nil(result)
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// TestCreateIdentityProvider_AuthorizationMapping_GroupNotFound rejects a mapping that names a group
// that does not exist.
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_AuthorizationMapping_GroupNotFound() {
	s.mockGroup.On("GetGroupsByIDs", mock.Anything, []string{"missing-group-id"}).
		Return(map[string]*group.Group{}, nil)

	idp := authorizationMappingIDP(providers.AuthorizationTarget{
		Type: providers.AuthorizationTargetGroup, ID: "missing-group-id",
	})

	result, err := s.idpService.CreateIdentityProvider(context.Background(), idp)

	s.Nil(result)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidAttributeConfiguration.Code, err.Code)
}

// TestCreateIdentityProvider_AuthorizationMapping_PermissionNotFound rejects a mapping that names a
// permission that does not exist on the given resource server (which also covers a nonexistent
// resource server, since ValidatePermissions reports every requested permission as invalid then).
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_AuthorizationMapping_PermissionNotFound() {
	s.mockResource.On("ValidatePermissions", mock.Anything, "rs-1", []string{"read"}).
		Return([]string{"read"}, nil)

	idp := authorizationMappingIDP(providers.AuthorizationTarget{
		Type: providers.AuthorizationTargetPermission, ResourceServerID: "rs-1", Permission: "read",
	})

	result, err := s.idpService.CreateIdentityProvider(context.Background(), idp)

	s.Nil(result)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidAttributeConfiguration.Code, err.Code)
}

// TestCreateIdentityProvider_AuthorizationMapping_ExistingTargetsAccepted accepts a mapping whose
// role, group, and permission targets all exist.
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_AuthorizationMapping_ExistingTargetsAccepted() {
	s.mockStore.On("GetIdentityProviderByName", mock.Anything, mock.Anything).
		Return((*providers.IDPDTO)(nil), ErrIDPNotFound)
	s.mockStore.On("CreateIdentityProvider", mock.Anything, mock.Anything).Return(nil)

	s.mockRole.On("GetRoleWithPermissions", mock.Anything, "role-id").
		Return(&role.RoleWithPermissions{ID: "role-id"}, nil)
	s.mockGroup.On("GetGroupsByIDs", mock.Anything, []string{"group-id"}).
		Return(map[string]*group.Group{"group-id": {ID: "group-id"}}, nil)
	s.mockResource.On("ValidatePermissions", mock.Anything, "rs-1", []string{"read"}).
		Return([]string{}, nil)

	idp := &providers.IDPDTO{
		Name:       "Authz Mapping Test IDP",
		Type:       providers.IDPTypeOIDC,
		Properties: createOIDCProperties(),
		AttributeConfiguration: &providers.AttributeConfiguration{
			AuthorizationMappings: []providers.AuthorizationMapping{
				{
					Claim: "groups",
					Values: []providers.AuthorizationRule{
						{
							Operator: providers.AuthorizationOperatorEquals,
							Value:    "platform-admins",
							Targets: []providers.AuthorizationTarget{
								{Type: providers.AuthorizationTargetRole, ID: "role-id"},
								{Type: providers.AuthorizationTargetGroup, ID: "group-id"},
							},
						},
						{
							Operator: providers.AuthorizationOperatorEquals,
							Value:    "platform-deleters",
							Targets: []providers.AuthorizationTarget{
								{
									Type: providers.AuthorizationTargetPermission, ResourceServerID: "rs-1",
									Permission: "read",
								},
							},
						},
					},
				},
			},
		},
	}

	result, err := s.idpService.CreateIdentityProvider(context.Background(), idp)

	s.Nil(err)
	s.Require().NotNil(result)
}

// ----- AuthorizationMapping validation -----

type AuthorizationMappingTestSuite struct {
	suite.Suite
}

func TestAuthorizationMappingTestSuite(t *testing.T) {
	suite.Run(t, new(AuthorizationMappingTestSuite))
}

func roleTarget(id string) providers.AuthorizationTarget {
	return providers.AuthorizationTarget{Type: providers.AuthorizationTargetRole, ID: id}
}

func groupTarget(id string) providers.AuthorizationTarget {
	return providers.AuthorizationTarget{Type: providers.AuthorizationTargetGroup, ID: id}
}

// equalsRule builds an equals-operator rule.
func equalsRule(value string, targets ...providers.AuthorizationTarget) providers.AuthorizationRule {
	return providers.AuthorizationRule{Operator: providers.AuthorizationOperatorEquals, Value: value, Targets: targets}
}

func (suite *AuthorizationMappingTestSuite) TestValidateAuthorizationMappingsAcceptsValidRoleAndGroupTargets() {
	mappings := []providers.AuthorizationMapping{
		{
			Claim: "groups",
			Values: []providers.AuthorizationRule{
				equalsRule("engineering", roleTarget("role-eng"), groupTarget("group-eng")),
			},
		},
	}
	suite.Nil(validateAuthorizationMappings(mappings))
}

func (suite *AuthorizationMappingTestSuite) TestValidateAuthorizationMappingsAcceptsValidPermissionTarget() {
	mappings := []providers.AuthorizationMapping{
		{
			Claim: "scope",
			Values: []providers.AuthorizationRule{equalsRule("orders.write", providers.AuthorizationTarget{
				Type:             providers.AuthorizationTargetPermission,
				ResourceServerID: "rs-orders",
				Permission:       "write",
			})},
		},
	}
	suite.Nil(validateAuthorizationMappings(mappings))
}

func (suite *AuthorizationMappingTestSuite) TestValidateAuthorizationMappingsAcceptsGreaterThanOnNumberType() {
	mappings := []providers.AuthorizationMapping{
		{
			Claim:     "level",
			ValueType: providers.AuthorizationValueTypeNumber,
			Values: []providers.AuthorizationRule{
				{
					Operator: providers.AuthorizationOperatorGreaterThan,
					Value:    "5",
					Targets:  []providers.AuthorizationTarget{roleTarget("role-1")},
				},
			},
		},
	}
	suite.Nil(validateAuthorizationMappings(mappings))
}

func (suite *AuthorizationMappingTestSuite) TestValidateAuthorizationMappingsRejectsEmptyClaim() {
	mappings := []providers.AuthorizationMapping{
		{Claim: "", Values: []providers.AuthorizationRule{equalsRule("x", roleTarget("role-1"))}},
	}
	suite.NotNil(validateAuthorizationMappings(mappings))
}

func (suite *AuthorizationMappingTestSuite) TestValidateAuthorizationMappingsRejectsNoValues() {
	mappings := []providers.AuthorizationMapping{
		{Claim: "groups", Values: []providers.AuthorizationRule{}},
	}
	suite.NotNil(validateAuthorizationMappings(mappings))
}

func (suite *AuthorizationMappingTestSuite) TestValidateAuthorizationMappingsRejectsRoleTargetWithNoID() {
	mappings := []providers.AuthorizationMapping{
		{
			Claim: "groups",
			Values: []providers.AuthorizationRule{
				equalsRule("engineering", providers.AuthorizationTarget{Type: providers.AuthorizationTargetRole}),
			},
		},
	}
	suite.NotNil(validateAuthorizationMappings(mappings))
}

func (suite *AuthorizationMappingTestSuite) TestValidateAuthorizationMappingsRejectsIncompletePermissionTarget() {
	mappings := []providers.AuthorizationMapping{
		{
			Claim: "scope",
			Values: []providers.AuthorizationRule{equalsRule("orders.write", providers.AuthorizationTarget{
				Type: providers.AuthorizationTargetPermission, ResourceServerID: "rs-orders",
			})},
		},
	}
	suite.NotNil(validateAuthorizationMappings(mappings))
}

func (suite *AuthorizationMappingTestSuite) TestValidateAuthorizationMappingsRejectsUnknownTargetType() {
	mappings := []providers.AuthorizationMapping{
		{
			Claim: "groups",
			Values: []providers.AuthorizationRule{
				equalsRule("engineering", providers.AuthorizationTarget{Type: "not-a-real-type", ID: "x"}),
			},
		},
	}
	suite.NotNil(validateAuthorizationMappings(mappings))
}

func (suite *AuthorizationMappingTestSuite) TestValidateAuthorizationMappingsRejectsInvalidValueType() {
	mappings := []providers.AuthorizationMapping{
		{
			Claim:     "groups",
			ValueType: "not-a-real-type",
			Values:    []providers.AuthorizationRule{equalsRule("x", roleTarget("role-1"))},
		},
	}
	suite.NotNil(validateAuthorizationMappings(mappings))
}

func (suite *AuthorizationMappingTestSuite) TestValidateAuthorizationMappingsRejectsInvalidOperator() {
	mappings := []providers.AuthorizationMapping{
		{
			Claim: "groups",
			Values: []providers.AuthorizationRule{
				{
					Operator: "not-a-real-operator",
					Value:    "x",
					Targets:  []providers.AuthorizationTarget{roleTarget("role-1")},
				},
			},
		},
	}
	suite.NotNil(validateAuthorizationMappings(mappings))
}

func (suite *AuthorizationMappingTestSuite) TestValidateAuthorizationMappingsRejectsOrderingOperatorOnStringType() {
	mappings := []providers.AuthorizationMapping{
		{
			Claim: "groups",
			Values: []providers.AuthorizationRule{
				{
					Operator: providers.AuthorizationOperatorGreaterThan,
					Value:    "5",
					Targets:  []providers.AuthorizationTarget{roleTarget("role-1")},
				},
			},
		},
	}
	suite.NotNil(validateAuthorizationMappings(mappings))
}

func (suite *AuthorizationMappingTestSuite) TestValidateAuthorizationMappingsRejectsNonNumericValueForNumberType() {
	mappings := []providers.AuthorizationMapping{
		{
			Claim:     "level",
			ValueType: providers.AuthorizationValueTypeNumber,
			Values:    []providers.AuthorizationRule{equalsRule("not-a-number", roleTarget("role-1"))},
		},
	}
	suite.NotNil(validateAuthorizationMappings(mappings))
}

func (suite *AuthorizationMappingTestSuite) TestValidateAuthorizationMappingsRejectsNonBooleanValueForBooleanType() {
	mappings := []providers.AuthorizationMapping{
		{
			Claim:     "is_admin",
			ValueType: providers.AuthorizationValueTypeBoolean,
			Values:    []providers.AuthorizationRule{equalsRule("not-a-boolean", roleTarget("role-1"))},
		},
	}
	suite.NotNil(validateAuthorizationMappings(mappings))
}

// Validation must accept exactly what evaluation will use: whitespace around a number or boolean
// value is never significant, so it is trimmed before the parseability check the same way it is
// trimmed before comparison, rather than rejecting a value evaluation would have matched fine.
func (suite *AuthorizationMappingTestSuite) TestValidateAuthorizationMappingsAcceptsWhitespacePaddedNumberAndBoolean() {
	mappings := []providers.AuthorizationMapping{
		{
			Claim:     "level",
			ValueType: providers.AuthorizationValueTypeNumber,
			Values:    []providers.AuthorizationRule{equalsRule(" 5 ", roleTarget("role-1"))},
		},
		{
			Claim:     "is_admin",
			ValueType: providers.AuthorizationValueTypeBoolean,
			Values:    []providers.AuthorizationRule{equalsRule(" true ", roleTarget("role-2"))},
		},
	}
	suite.Nil(validateAuthorizationMappings(mappings))
}

func (suite *AuthorizationMappingTestSuite) TestValidateAuthorizationMappingsAcceptsIncludesOnArrayType() {
	mappings := []providers.AuthorizationMapping{
		{
			Claim:     "groups",
			ValueType: providers.AuthorizationValueTypeArray,
			Values: []providers.AuthorizationRule{
				{
					Operator: providers.AuthorizationOperatorIncludes,
					Value:    "engineering",
					Targets:  []providers.AuthorizationTarget{roleTarget("role-1")},
				},
			},
		},
	}
	suite.Nil(validateAuthorizationMappings(mappings))
}

func (suite *AuthorizationMappingTestSuite) TestValidateAuthorizationMappingsAcceptsNotIncludesOnDelimitedString() {
	mappings := []providers.AuthorizationMapping{
		{
			Claim:     "scope",
			ValueType: providers.AuthorizationValueTypeString,
			Delimiter: " ",
			Values: []providers.AuthorizationRule{
				{
					Operator: providers.AuthorizationOperatorNotIncludes,
					Value:    "guest",
					Targets:  []providers.AuthorizationTarget{roleTarget("role-1")},
				},
			},
		},
	}
	suite.Nil(validateAuthorizationMappings(mappings))
}

func (suite *AuthorizationMappingTestSuite) TestValidateAuthorizationMappingsRejectsIncludesOnSingleValuedString() {
	mappings := []providers.AuthorizationMapping{
		{
			Claim: "department",
			Values: []providers.AuthorizationRule{
				{
					Operator: providers.AuthorizationOperatorIncludes,
					Value:    "platform",
					Targets:  []providers.AuthorizationTarget{roleTarget("role-1")},
				},
			},
		},
	}
	suite.NotNil(validateAuthorizationMappings(mappings))
}

func (suite *AuthorizationMappingTestSuite) TestValidateAuthorizationMappingsRejectsEqualsOnArrayType() {
	mappings := []providers.AuthorizationMapping{
		{
			Claim:     "groups",
			ValueType: providers.AuthorizationValueTypeArray,
			Values:    []providers.AuthorizationRule{equalsRule("engineering", roleTarget("role-1"))},
		},
	}
	suite.NotNil(validateAuthorizationMappings(mappings))
}

func (suite *AuthorizationMappingTestSuite) TestValidateAuthorizationMappingsRejectsEqualsOnDelimitedString() {
	mappings := []providers.AuthorizationMapping{
		{
			Claim:     "scope",
			ValueType: providers.AuthorizationValueTypeString,
			Delimiter: " ",
			Values:    []providers.AuthorizationRule{equalsRule("orders.write", roleTarget("role-1"))},
		},
	}
	suite.NotNil(validateAuthorizationMappings(mappings))
}

func (suite *AuthorizationMappingTestSuite) TestValidateAuthorizationMappingsRejectsDelimiterOnNonStringType() {
	numberWithDelimiter := []providers.AuthorizationMapping{
		{
			Claim:     "levels",
			ValueType: providers.AuthorizationValueTypeNumber,
			Delimiter: ",",
			Values:    []providers.AuthorizationRule{equalsRule("5", roleTarget("role-1"))},
		},
	}
	suite.NotNil(validateAuthorizationMappings(numberWithDelimiter), "delimiter is only meaningful for string")

	arrayWithDelimiter := []providers.AuthorizationMapping{
		{
			Claim:     "groups",
			ValueType: providers.AuthorizationValueTypeArray,
			Delimiter: ",",
			Values: []providers.AuthorizationRule{
				{
					Operator: providers.AuthorizationOperatorIncludes,
					Value:    "engineering",
					Targets:  []providers.AuthorizationTarget{roleTarget("role-1")},
				},
			},
		},
	}
	suite.NotNil(validateAuthorizationMappings(arrayWithDelimiter),
		"an array is already discrete, a delimiter is meaningless")
}

func (suite *AuthorizationMappingTestSuite) TestValidateAuthorizationMappingsAcceptsDelimiterOnStringType() {
	mappings := []providers.AuthorizationMapping{
		{
			Claim:     "scope",
			ValueType: providers.AuthorizationValueTypeString,
			Delimiter: " ",
			Values: []providers.AuthorizationRule{
				{
					Operator: providers.AuthorizationOperatorIncludes,
					Value:    "orders.write",
					Targets:  []providers.AuthorizationTarget{roleTarget("role-1")},
				},
			},
		},
	}
	suite.Nil(validateAuthorizationMappings(mappings))
}

func (suite *AuthorizationMappingTestSuite) TestNoAuthorizationMappingsConfiguredIsValid() {
	suite.Nil(validateAuthorizationMappings(nil))
}
