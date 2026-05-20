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

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

type mockTransactioner struct{}

func (m *mockTransactioner) Transact(ctx context.Context, operation func(txCtx context.Context) error) error {
	return operation(ctx)
}

type IDPServiceTestSuite struct {
	suite.Suite
	mockStore  *idpStoreInterfaceMock
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
	s.idpService = &idpService{
		idpStore:      s.mockStore,
		transactioner: &mockTransactioner{},
		logger:        log.GetLogger().With(log.String(log.LoggerKeyComponentName, "IdPService")),
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
	idp := &IDPDTO{
		Name:        "Test IDP",
		Description: "Test Description",
		Type:        IDPTypeOIDC,
		Properties:  createOIDCProperties(),
	}

	s.mockStore.On("GetIdentityProviderByName", mock.Anything, "Test IDP").Return((*IDPDTO)(nil), ErrIDPNotFound)
	s.mockStore.On("CreateIdentityProvider", mock.Anything, mock.MatchedBy(func(dto IDPDTO) bool {
		return dto.Name == "Test IDP" && dto.Type == IDPTypeOIDC && dto.ID != ""
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
		expected serviceerror.ServiceError
	}{
		{"Empty name", "", ErrorInvalidIDPName},
		{"Whitespace name", "   ", ErrorInvalidIDPName},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			idp := &IDPDTO{
				Name: tc.idpName,
				Type: IDPTypeOIDC,
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
		idpType IDPType
	}{
		{"Empty type", IDPType("")},
		{"Invalid type", IDPType("INVALID")},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			idp := &IDPDTO{
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
	idp := &IDPDTO{
		Name:       "Existing IDP",
		Type:       IDPTypeOIDC,
		Properties: createOIDCProperties(),
	}

	existingIDP := &IDPDTO{ID: "existing-id", Name: "Existing IDP"}
	s.mockStore.On("GetIdentityProviderByName", mock.Anything, "Existing IDP").Return(existingIDP, nil)

	result, err := s.idpService.CreateIdentityProvider(context.Background(), idp)

	s.Nil(result)
	s.NotNil(err)
	s.Equal(ErrorIDPAlreadyExists.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestCreateIdentityProvider_CheckExistingStoreError tests store error when checking existing IDP
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_CheckExistingStoreError() {
	idp := &IDPDTO{
		Name:       "Test IDP",
		Type:       IDPTypeOIDC,
		Properties: createOIDCProperties(),
	}

	s.mockStore.On("GetIdentityProviderByName", mock.Anything, "Test IDP").
		Return((*IDPDTO)(nil), errors.New("database error"))

	result, err := s.idpService.CreateIdentityProvider(context.Background(), idp)

	s.Nil(result)
	s.NotNil(err)
	s.Equal(serviceerror.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestCreateIdentityProvider_StoreError tests store error handling
func (s *IDPServiceTestSuite) TestCreateIdentityProvider_StoreError() {
	idp := &IDPDTO{
		Name:       "Test IDP",
		Type:       IDPTypeOIDC,
		Properties: createOIDCProperties(),
	}

	s.mockStore.On("GetIdentityProviderByName", mock.Anything, "Test IDP").Return((*IDPDTO)(nil), ErrIDPNotFound)
	s.mockStore.On("CreateIdentityProvider", mock.Anything, mock.Anything).Return(errors.New("database error"))

	result, err := s.idpService.CreateIdentityProvider(context.Background(), idp)

	s.Nil(result)
	s.NotNil(err)
	s.Equal(serviceerror.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIdentityProviderList_Success tests successful list retrieval
func (s *IDPServiceTestSuite) TestGetIdentityProviderList_Success() {
	idpList := []BasicIDPDTO{
		{ID: "idp-1", Name: "IDP 1", Type: IDPTypeOIDC},
		{ID: "idp-2", Name: "IDP 2", Type: IDPTypeGoogle},
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
	s.Equal(serviceerror.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIdentityProvider_Success tests successful IDP retrieval
func (s *IDPServiceTestSuite) TestGetIdentityProvider_Success() {
	idp := &IDPDTO{
		ID:          "idp-123",
		Name:        "Test IDP",
		Description: "Test Description",
		Type:        IDPTypeOIDC,
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
	s.mockStore.On("GetIdentityProvider", mock.Anything, "non-existent").Return((*IDPDTO)(nil), ErrIDPNotFound)

	result, err := s.idpService.GetIdentityProvider(context.Background(), "non-existent")

	s.Nil(result)
	s.NotNil(err)
	s.Equal(ErrorIDPNotFound.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIdentityProvider_StoreError tests store error handling
func (s *IDPServiceTestSuite) TestGetIdentityProvider_StoreError() {
	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").Return((*IDPDTO)(nil), errors.New("database error"))

	result, err := s.idpService.GetIdentityProvider(context.Background(), "idp-123")

	s.Nil(result)
	s.NotNil(err)
	s.Equal(serviceerror.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIdentityProviderByName_Success tests successful IDP retrieval by name
func (s *IDPServiceTestSuite) TestGetIdentityProviderByName_Success() {
	idp := &IDPDTO{
		ID:   "idp-123",
		Name: "Test IDP",
		Type: IDPTypeOIDC,
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
	s.mockStore.On("GetIdentityProviderByName", mock.Anything, "Non-existent").Return((*IDPDTO)(nil), ErrIDPNotFound)

	result, err := s.idpService.GetIdentityProviderByName(context.Background(), "Non-existent")

	s.Nil(result)
	s.NotNil(err)
	s.Equal(ErrorIDPNotFound.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIdentityProviderByName_StoreError tests store error handling
func (s *IDPServiceTestSuite) TestGetIdentityProviderByName_StoreError() {
	s.mockStore.On("GetIdentityProviderByName", mock.Anything, "Test").
		Return((*IDPDTO)(nil), errors.New("database error"))

	result, err := s.idpService.GetIdentityProviderByName(context.Background(), "Test")

	s.Nil(result)
	s.NotNil(err)
	s.Equal(serviceerror.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIdentityProviderByIssuer_Success tests successful IDP retrieval by issuer
func (s *IDPServiceTestSuite) TestGetIdentityProviderByIssuer_Success() {
	prop, _ := cmodels.NewProperty(PropIssuer, "https://idp.example.com", false)
	idp := &IDPDTO{
		ID:         "idp-123",
		Name:       "Test IDP",
		Type:       IDPTypeOIDC,
		Properties: []cmodels.Property{*prop},
	}

	s.mockStore.On("GetIdentityProviderByIssuer", mock.Anything, "https://idp.example.com").Return(idp, nil)

	result, err := s.idpService.GetIdentityProviderByIssuer(context.Background(), "https://idp.example.com")

	s.Nil(err)
	s.NotNil(result)
	s.Equal("idp-123", result.ID)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIdentityProviderByIssuer_EmptyIssuer tests empty issuer validation
func (s *IDPServiceTestSuite) TestGetIdentityProviderByIssuer_EmptyIssuer() {
	result, err := s.idpService.GetIdentityProviderByIssuer(context.Background(), "")

	s.Nil(result)
	s.NotNil(err)
	s.Equal(ErrorInvalidIDPID.Code, err.Code)
}

// TestGetIdentityProviderByIssuer_NotFound tests IDP not found by issuer
func (s *IDPServiceTestSuite) TestGetIdentityProviderByIssuer_NotFound() {
	s.mockStore.On("GetIdentityProviderByIssuer", mock.Anything, "https://unknown.example.com").
		Return((*IDPDTO)(nil), ErrIDPNotFound)

	result, err := s.idpService.GetIdentityProviderByIssuer(context.Background(), "https://unknown.example.com")

	s.Nil(result)
	s.NotNil(err)
	s.Equal(ErrorIDPNotFound.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetIdentityProviderByIssuer_StoreError tests store error handling
func (s *IDPServiceTestSuite) TestGetIdentityProviderByIssuer_StoreError() {
	s.mockStore.On("GetIdentityProviderByIssuer", mock.Anything, "https://idp.example.com").
		Return((*IDPDTO)(nil), errors.New("database error"))

	result, err := s.idpService.GetIdentityProviderByIssuer(context.Background(), "https://idp.example.com")

	s.Nil(result)
	s.NotNil(err)
	s.Equal(serviceerror.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestUpdateIdentityProvider_Success tests successful IDP update
func (s *IDPServiceTestSuite) TestUpdateIdentityProvider_Success() {
	idp := &IDPDTO{
		Name:       "Updated IDP",
		Type:       IDPTypeOIDC,
		Properties: createOIDCProperties(),
	}

	existingIDP := &IDPDTO{
		ID:         "idp-123",
		Name:       "Old Name",
		Type:       IDPTypeOIDC,
		Properties: createOIDCProperties(),
	}

	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").Return(existingIDP, nil)
	s.mockStore.On("GetIdentityProviderByName", mock.Anything, "Updated IDP").Return((*IDPDTO)(nil), ErrIDPNotFound)
	s.mockStore.On("UpdateIdentityProvider", mock.Anything, mock.MatchedBy(func(dto *IDPDTO) bool {
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
	idp := &IDPDTO{Name: "Test", Type: IDPTypeOIDC, Properties: createOIDCProperties()}

	result, err := s.idpService.UpdateIdentityProvider(context.Background(), "", idp)

	s.Nil(result)
	s.NotNil(err)
	s.Equal(ErrorInvalidIDPID.Code, err.Code)
}

// TestUpdateIdentityProvider_NotFound tests IDP not found
func (s *IDPServiceTestSuite) TestUpdateIdentityProvider_NotFound() {
	idp := &IDPDTO{Name: "Test", Type: IDPTypeOIDC, Properties: createOIDCProperties()}

	s.mockStore.On("GetIdentityProvider", mock.Anything, "non-existent").Return((*IDPDTO)(nil), ErrIDPNotFound)

	result, err := s.idpService.UpdateIdentityProvider(context.Background(), "non-existent", idp)

	s.Nil(result)
	s.NotNil(err)
	s.Equal(ErrorIDPNotFound.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestUpdateIdentityProvider_NameConflict tests name conflict during update
func (s *IDPServiceTestSuite) TestUpdateIdentityProvider_NameConflict() {
	idp := &IDPDTO{Name: "Existing Name", Type: IDPTypeOIDC, Properties: createOIDCProperties()}

	existingIDP := &IDPDTO{ID: "idp-123", Name: "Old Name", Type: IDPTypeOIDC,
		Properties: createOIDCProperties()}
	conflictIDP := &IDPDTO{ID: "idp-456", Name: "Existing Name", Type: IDPTypeOIDC,
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
	idp := &IDPDTO{Name: "Same Name", Type: IDPTypeOIDC, Description: "New Description",
		Properties: createOIDCProperties()}

	existingIDP := &IDPDTO{ID: "idp-123", Name: "Same Name", Type: IDPTypeOIDC,
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
		idp         *IDPDTO
		expectedErr serviceerror.ServiceError
	}{
		{
			name:        "Nil IDP",
			idp:         nil,
			expectedErr: ErrorIDPNil,
		},
		{
			name:        "Empty name",
			idp:         &IDPDTO{Name: "", Type: IDPTypeOIDC},
			expectedErr: ErrorInvalidIDPName,
		},
		{
			name:        "Invalid type",
			idp:         &IDPDTO{Name: "Test", Type: IDPType("INVALID")},
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
	idp := &IDPDTO{Name: "Test", Type: IDPTypeOIDC, Properties: createOIDCProperties()}

	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").Return((*IDPDTO)(nil), errors.New("database error"))

	result, err := s.idpService.UpdateIdentityProvider(context.Background(), "idp-123", idp)

	s.Nil(result)
	s.NotNil(err)
	s.Equal(serviceerror.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestUpdateIdentityProvider_CheckNameStoreError tests store error when checking name conflict
func (s *IDPServiceTestSuite) TestUpdateIdentityProvider_CheckNameStoreError() {
	idp := &IDPDTO{Name: "New Name", Type: IDPTypeOIDC, Properties: createOIDCProperties()}

	existingIDP := &IDPDTO{ID: "idp-123", Name: "Old Name", Type: IDPTypeOIDC, Properties: createOIDCProperties()}

	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").Return(existingIDP, nil)
	s.mockStore.On("GetIdentityProviderByName", mock.Anything, "New Name").
		Return((*IDPDTO)(nil), errors.New("database error"))

	result, err := s.idpService.UpdateIdentityProvider(context.Background(), "idp-123", idp)

	s.Nil(result)
	s.NotNil(err)
	s.Equal(serviceerror.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestUpdateIdentityProvider_StoreError tests store error during update
func (s *IDPServiceTestSuite) TestUpdateIdentityProvider_StoreError() {
	idp := &IDPDTO{Name: "Test", Type: IDPTypeOIDC, Properties: createOIDCProperties()}

	existingIDP := &IDPDTO{ID: "idp-123", Name: "Test", Type: IDPTypeOIDC, Properties: createOIDCProperties()}

	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").Return(existingIDP, nil)
	s.mockStore.On("UpdateIdentityProvider", mock.Anything, mock.Anything).Return(errors.New("database error"))

	result, err := s.idpService.UpdateIdentityProvider(context.Background(), "idp-123", idp)

	s.Nil(result)
	s.NotNil(err)
	s.Equal(serviceerror.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestDeleteIdentityProvider_Success tests successful IDP deletion
func (s *IDPServiceTestSuite) TestDeleteIdentityProvider_Success() {
	existingIDP := &IDPDTO{ID: "idp-123", Name: "Test IDP"}

	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").Return(existingIDP, nil)
	s.mockStore.On("DeleteIdentityProvider", mock.Anything, "idp-123").Return(nil)

	err := s.idpService.DeleteIdentityProvider(context.Background(), "idp-123")

	s.Nil(err)
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
	s.mockStore.On("GetIdentityProvider", mock.Anything, "non-existent").Return((*IDPDTO)(nil), ErrIDPNotFound)

	err := s.idpService.DeleteIdentityProvider(context.Background(), "non-existent")

	s.Nil(err) // Delete is idempotent, returns nil for non-existent
	s.mockStore.AssertExpectations(s.T())
}

// TestDeleteIdentityProvider_GetStoreError tests store error when checking existing IDP
func (s *IDPServiceTestSuite) TestDeleteIdentityProvider_GetStoreError() {
	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").Return((*IDPDTO)(nil), errors.New("database error"))

	err := s.idpService.DeleteIdentityProvider(context.Background(), "idp-123")

	s.NotNil(err)
	s.Equal(serviceerror.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
}

// TestDeleteIdentityProvider_StoreError tests store error handling
func (s *IDPServiceTestSuite) TestDeleteIdentityProvider_StoreError() {
	existingIDP := &IDPDTO{ID: "idp-123", Name: "Test IDP"}

	s.mockStore.On("GetIdentityProvider", mock.Anything, "idp-123").Return(existingIDP, nil)
	s.mockStore.On("DeleteIdentityProvider", mock.Anything, "idp-123").Return(errors.New("database error"))

	err := s.idpService.DeleteIdentityProvider(context.Background(), "idp-123")

	s.NotNil(err)
	s.Equal(serviceerror.InternalServerError.Code, err.Code)
	s.mockStore.AssertExpectations(s.T())
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

	idp := &IDPDTO{
		Name: "Test IDP",
		Type: IDPTypeOIDC,
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

	idp := &IDPDTO{
		Name: "Updated IDP",
		Type: IDPTypeOIDC,
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
		idp         *IDPDTO
		expectError bool
		errorCode   string
	}{
		{
			name: "Valid IDP",
			idp: &IDPDTO{
				Name:       "Test",
				Type:       IDPTypeOIDC,
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
			idp: &IDPDTO{
				Name: "",
				Type: IDPTypeOIDC,
			},
			expectError: true,
			errorCode:   ErrorInvalidIDPName.Code,
		},
		{
			name: "Empty type",
			idp: &IDPDTO{
				Name: "Test",
				Type: IDPType(""),
			},
			expectError: true,
			errorCode:   ErrorInvalidIDPType.Code,
		},
		{
			name: "Invalid type",
			idp: &IDPDTO{
				Name: "Test",
				Type: IDPType("INVALID"),
			},
			expectError: true,
			errorCode:   ErrorInvalidIDPType.Code,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			logger := log.GetLogger()
			err := validateIDP(tc.idp, logger)
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
	existingIDP := &IDPDTO{
		ID:          idpID,
		Name:        "Declarative IDP",
		Description: "From file store",
		Type:        IDPTypeOIDC,
		Properties:  createOIDCProperties(),
	}

	fileStore := newIdpStoreInterfaceMock(s.T())
	dbStore := newIdpStoreInterfaceMock(s.T())
	compositeStore := newCompositeIDPStore(fileStore, dbStore)

	dbStore.On("GetIdentityProvider", context.Background(), idpID).Return((*IDPDTO)(nil), ErrIDPNotFound)
	fileStore.On("GetIdentityProvider", context.Background(), idpID).Return(existingIDP, nil)

	dbStore.On("GetIdentityProviderByName", context.Background(), "Updated Name").Return((*IDPDTO)(nil), ErrIDPNotFound)
	fileStore.On("GetIdentityProviderByName", context.Background(), "Updated Name").
		Return((*IDPDTO)(nil), ErrIDPNotFound)

	service := newIDPService(compositeStore, &mockTransactioner{})

	updatedIDP := &IDPDTO{
		Name:        "Updated Name",
		Description: "Updated Description",
		Type:        IDPTypeOIDC,
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
	existingIDP := &IDPDTO{
		ID:          idpID,
		Name:        "Mutable IDP",
		Description: "From database",
		Type:        IDPTypeOIDC,
		Properties:  createOIDCProperties(),
	}

	fileStore := newIdpStoreInterfaceMock(s.T())
	dbStore := newIdpStoreInterfaceMock(s.T())
	compositeStore := newCompositeIDPStore(fileStore, dbStore)

	fileStore.On("GetIdentityProvider", context.Background(), idpID).Return((*IDPDTO)(nil), ErrIDPNotFound)
	fileStore.On("GetIdentityProviderByName", context.Background(), "Updated Name").
		Return((*IDPDTO)(nil), ErrIDPNotFound)
	dbStore.On("GetIdentityProvider", context.Background(), idpID).Return(existingIDP, nil)
	dbStore.On("GetIdentityProviderByName", context.Background(), "Updated Name").Return((*IDPDTO)(nil), ErrIDPNotFound)
	dbStore.On("UpdateIdentityProvider", context.Background(), mock.MatchedBy(func(dto *IDPDTO) bool {
		return dto.ID == idpID && dto.Name == "Updated Name"
	})).Return(nil)

	service := newIDPService(compositeStore, &mockTransactioner{})

	updatedIDP := &IDPDTO{
		Name:        "Updated Name",
		Description: "Updated Description",
		Type:        IDPTypeOIDC,
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
	existingIDP := &IDPDTO{
		ID:          idpID,
		Name:        "Declarative IDP",
		Description: "From file store",
		Type:        IDPTypeOIDC,
	}

	fileStore := newIdpStoreInterfaceMock(s.T())
	dbStore := newIdpStoreInterfaceMock(s.T())
	compositeStore := newCompositeIDPStore(fileStore, dbStore)

	dbStore.On("GetIdentityProvider", context.Background(), idpID).Return((*IDPDTO)(nil), ErrIDPNotFound)
	fileStore.On("GetIdentityProvider", context.Background(), idpID).Return(existingIDP, nil)

	service := newIDPService(compositeStore, &mockTransactioner{})

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
	existingIDP := &IDPDTO{
		ID:          idpID,
		Name:        "Mutable IDP",
		Description: "From database",
		Type:        IDPTypeOIDC,
	}

	fileStore := newIdpStoreInterfaceMock(s.T())
	dbStore := newIdpStoreInterfaceMock(s.T())
	compositeStore := newCompositeIDPStore(fileStore, dbStore)

	fileStore.On("GetIdentityProvider", context.Background(), idpID).Return((*IDPDTO)(nil), ErrIDPNotFound)
	dbStore.On("GetIdentityProvider", context.Background(), idpID).Return(existingIDP, nil)
	dbStore.On("DeleteIdentityProvider", context.Background(), idpID).Return(nil)

	service := newIDPService(compositeStore, &mockTransactioner{})

	err := service.DeleteIdentityProvider(context.Background(), idpID)

	s.Nil(err)
	dbStore.AssertCalled(s.T(), "DeleteIdentityProvider", context.Background(), idpID)

	config.ResetServerRuntime()
}
