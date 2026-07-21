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
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
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
	mockStore  *idpStoreInterfaceMock
	mockET     *entitytypemock.EntityTypeServiceInterfaceMock
	idpService *idpService
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
	s.idpService = &idpService{
		idpStore:           s.mockStore,
		transactioner:      &mockTransactioner{},
		dependencyRegistry: newNoBlockingDepsRegistry(),
		logger:             log.GetLogger().With(log.String(log.LoggerKeyComponentName, "IdPService")),
		uuidGenerator:      utils.GenerateUUIDv7,
		entityTypeService:  s.mockET,
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

	service := newIDPService(compositeStore, nil, &mockTransactioner{})

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

	service := newIDPService(compositeStore, nil, &mockTransactioner{})

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

	service := newIDPService(compositeStore, nil, &mockTransactioner{})
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

	service := newIDPService(compositeStore, nil, &mockTransactioner{})
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
	s.mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "person", false, true, false).
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
	s.mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "person", false, true, false).
		Return([]entitytype.AttributeInfo{{Attribute: "firstName"}}, (*tidcommon.ServiceError)(nil))
	idp := &providers.IDPDTO{AttributeConfiguration: singleProfileMapping("person", nil)}
	svcErr := s.idpService.validateAttributeConfiguration(context.Background(), idp)
	s.Nil(svcErr)
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_OneSourceToMultipleTargets() {
	s.mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "person", false, true, false).
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
	s.mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "person", false, true, false).
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
	s.mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "person", false, true, false).
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
	s.mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "ghost", false, true, false).
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
	s.mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "employee", false, true, false).
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
	s.mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "ghost", false, true, false).
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
