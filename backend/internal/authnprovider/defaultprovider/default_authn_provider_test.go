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

package defaultprovider

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	authnprovider "github.com/thunder-id/thunderid/internal/authnprovider/provider"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/tests/mocks/authn/commonmock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/magiclinkmock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/otpmock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/passkeymock"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
)

type DefaultAuthnProviderTestSuite struct {
	suite.Suite
	mockService   *entitymock.EntityServiceInterfaceMock
	mockPasskey   *passkeymock.PasskeyServiceInterfaceMock
	mockFederated *commonmock.FederatedAuthenticatorMock
	provider      authnprovider.AuthnProviderInterface
}

func (suite *DefaultAuthnProviderTestSuite) SetupTest() {
	suite.mockService = entitymock.NewEntityServiceInterfaceMock(suite.T())
	suite.mockPasskey = passkeymock.NewPasskeyServiceInterfaceMock(suite.T())
	suite.mockFederated = commonmock.NewFederatedAuthenticatorMock(suite.T())
	suite.provider = Initialize(suite.mockService, nil, nil, nil, nil, nil)
}

func TestDefaultAuthnProviderTestSuite(t *testing.T) {
	suite.Run(t, new(DefaultAuthnProviderTestSuite))
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Success() {
	identifiers := map[string]interface{}{"username": "testuser"}
	credentials := map[string]interface{}{"password": "password123"}

	authResult := &entity.AuthenticateResult{
		EntityID:       "user123",
		EntityCategory: providers.EntityCategoryUser,
		EntityType:     "customer",
		OUID:           "ou1",
	}

	entityObj := &providers.Entity{
		ID:         "user123",
		Category:   providers.EntityCategoryUser,
		Type:       "customer",
		State:      providers.EntityStateActive,
		OUID:       "ou1",
		Attributes: json.RawMessage(`{"email":"test@example.com"}`),
	}

	suite.mockService.On("AuthenticateEntity", mock.Anything, identifiers, credentials).
		Return(authResult, nil).Once()
	suite.mockService.On("GetEntity", mock.Anything, "user123").
		Return(entityObj, nil).Once()

	result, err := suite.provider.Authenticate(context.Background(), identifiers, credentials, nil)

	suite.Nil(err)
	suite.NotNil(result.EntityReference)
	suite.Equal("user123", result.EntityReference.EntityID)
	suite.Equal("user", result.EntityReference.EntityCategory)
	suite.Equal("customer", result.EntityReference.EntityType)
	suite.Equal("ou1", result.EntityReference.OUID)
	suite.NotNil(result.Attributes)
	suite.Len(result.Attributes.Attributes, 1)
	suite.Contains(result.Attributes.Attributes, "email")
	suite.Equal("test@example.com", result.Attributes.Attributes["email"].Value)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_NilCredentials() {
	identifiers := map[string]interface{}{"username": "testuser"}

	result, err := suite.provider.Authenticate(context.Background(), identifiers, nil, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.ClientErrorType, err.Type)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_EntityNotFound() {
	identifiers := map[string]interface{}{"username": "unknown"}
	credentials := map[string]interface{}{"password": "password"}

	suite.mockService.On("AuthenticateEntity", mock.Anything, identifiers, credentials).
		Return(nil, entity.ErrEntityNotFound).Once()

	result, err := suite.provider.Authenticate(context.Background(), identifiers, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeUserNotFound, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_AuthenticationFailed() {
	identifiers := map[string]interface{}{"username": "testuser"}
	credentials := map[string]interface{}{"password": "wrongpassword"}

	suite.mockService.On("AuthenticateEntity", mock.Anything, identifiers, credentials).
		Return(nil, entity.ErrAuthenticationFailed).Once()

	result, err := suite.provider.Authenticate(context.Background(), identifiers, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeAuthenticationFailed, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_GenericAuthError() {
	identifiers := map[string]interface{}{"username": "testuser"}
	credentials := map[string]interface{}{"password": "password123"}

	suite.mockService.On("AuthenticateEntity", mock.Anything, identifiers, credentials).
		Return(nil, errors.New("unexpected error")).Once()

	result, err := suite.provider.Authenticate(context.Background(), identifiers, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_GetEntityFails() {
	identifiers := map[string]interface{}{"username": "testuser"}
	credentials := map[string]interface{}{"password": "password123"}

	authResult := &entity.AuthenticateResult{
		EntityID:       "user123",
		EntityCategory: providers.EntityCategoryUser,
		EntityType:     "customer",
		OUID:           "ou1",
	}

	suite.mockService.On("AuthenticateEntity", mock.Anything, identifiers, credentials).
		Return(authResult, nil).Once()
	suite.mockService.On("GetEntity", mock.Anything, "user123").
		Return(nil, errors.New("db error")).Once()

	result, err := suite.provider.Authenticate(context.Background(), identifiers, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_GetEntityEmptyAttributes() {
	identifiers := map[string]interface{}{"username": "testuser"}
	credentials := map[string]interface{}{"password": "password123"}

	authResult := &entity.AuthenticateResult{
		EntityID:       "user123",
		EntityCategory: providers.EntityCategoryUser,
		EntityType:     "customer",
		OUID:           "ou1",
	}

	entityObj := &providers.Entity{
		ID:         "user123",
		Category:   providers.EntityCategoryUser,
		Type:       "customer",
		State:      providers.EntityStateActive,
		OUID:       "ou1",
		Attributes: nil,
	}

	suite.mockService.On("AuthenticateEntity", mock.Anything, identifiers, credentials).
		Return(authResult, nil).Once()
	suite.mockService.On("GetEntity", mock.Anything, "user123").
		Return(entityObj, nil).Once()

	result, err := suite.provider.Authenticate(context.Background(), identifiers, credentials, nil)

	suite.Nil(err)
	suite.NotNil(result.EntityReference)
	suite.Equal("user123", result.EntityReference.EntityID)
	suite.NotNil(result.Attributes)
	suite.Len(result.Attributes.Attributes, 0)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_InvalidAttributeJSON() {
	identifiers := map[string]interface{}{"username": "testuser"}
	credentials := map[string]interface{}{"password": "password123"}

	authResult := &entity.AuthenticateResult{
		EntityID:       "user123",
		EntityCategory: providers.EntityCategoryUser,
		EntityType:     "customer",
		OUID:           "ou1",
	}

	entityObj := &providers.Entity{
		ID:         "user123",
		Category:   providers.EntityCategoryUser,
		Type:       "customer",
		State:      providers.EntityStateActive,
		OUID:       "ou1",
		Attributes: json.RawMessage(`{invalid-json`),
	}

	suite.mockService.On("AuthenticateEntity", mock.Anything, identifiers, credentials).
		Return(authResult, nil).Once()
	suite.mockService.On("GetEntity", mock.Anything, "user123").
		Return(entityObj, nil).Once()

	result, err := suite.provider.Authenticate(context.Background(), identifiers, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_ByPreResolvedUserID_Success() {
	identifiers := map[string]interface{}{"userID": "resolved-user-123"}
	credentials := map[string]interface{}{"password": "password123"}

	authResult := &entity.AuthenticateResult{
		EntityID:       "resolved-user-123",
		EntityCategory: providers.EntityCategoryUser,
		EntityType:     "customer",
		OUID:           "ou1",
	}

	entityObj := &providers.Entity{
		ID:         "resolved-user-123",
		Category:   providers.EntityCategoryUser,
		Type:       "customer",
		State:      providers.EntityStateActive,
		OUID:       "ou1",
		Attributes: json.RawMessage(`{"email":"test@example.com"}`),
	}

	suite.mockService.On("AuthenticateEntityByID", mock.Anything, "resolved-user-123", credentials).
		Return(authResult, nil).Once()
	suite.mockService.On("GetEntity", mock.Anything, "resolved-user-123").
		Return(entityObj, nil).Once()

	result, err := suite.provider.Authenticate(context.Background(), identifiers, credentials, nil)

	suite.Nil(err)
	suite.NotNil(result.EntityReference)
	suite.Equal("resolved-user-123", result.EntityReference.EntityID)
	suite.Equal("customer", result.EntityReference.EntityType)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_ByPreResolvedUserID_InvalidUserID() {
	identifiers := map[string]interface{}{"userID": 123}
	credentials := map[string]interface{}{"password": "password123"}

	result, err := suite.provider.Authenticate(context.Background(), identifiers, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_ByPreResolvedUserID_EntityNotFound() {
	identifiers := map[string]interface{}{"userID": "missing-user"}
	credentials := map[string]interface{}{"password": "password123"}

	suite.mockService.On("AuthenticateEntityByID", mock.Anything, "missing-user", credentials).
		Return(nil, entity.ErrEntityNotFound).Once()

	result, err := suite.provider.Authenticate(context.Background(), identifiers, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeUserNotFound, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_ByPreResolvedUserID_AuthFailed() {
	identifiers := map[string]interface{}{"userID": "user-wrong-pw"}
	credentials := map[string]interface{}{"password": "wrongpassword"}

	suite.mockService.On("AuthenticateEntityByID", mock.Anything, "user-wrong-pw", credentials).
		Return(nil, entity.ErrAuthenticationFailed).Once()

	result, err := suite.provider.Authenticate(context.Background(), identifiers, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeAuthenticationFailed, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_EmptyUserID_FallsBackToIdentify() {
	identifiers := map[string]interface{}{"userID": "", "username": "testuser"}
	credentials := map[string]interface{}{"password": "password123"}

	authResult := &entity.AuthenticateResult{
		EntityID:       "user123",
		EntityCategory: providers.EntityCategoryUser,
		EntityType:     "customer",
		OUID:           "ou1",
	}

	entityObj := &providers.Entity{
		ID:         "user123",
		Category:   providers.EntityCategoryUser,
		Type:       "customer",
		State:      providers.EntityStateActive,
		OUID:       "ou1",
		Attributes: json.RawMessage(`{"email":"test@example.com"}`),
	}

	suite.mockService.On("AuthenticateEntity", mock.Anything, identifiers, credentials).
		Return(authResult, nil).Once()
	suite.mockService.On("GetEntity", mock.Anything, "user123").
		Return(entityObj, nil).Once()

	result, err := suite.provider.Authenticate(context.Background(), identifiers, credentials, nil)

	suite.Nil(err)
	suite.NotNil(result.EntityReference)
	suite.Equal("user123", result.EntityReference.EntityID)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Provisioning_Success() {
	credentials := map[string]interface{}{
		"provisionedEntityID": "provisioned-user-123",
	}

	entityObj := &providers.Entity{
		ID:         "provisioned-user-123",
		Category:   providers.EntityCategoryUser,
		Type:       "customer",
		State:      providers.EntityStateActive,
		OUID:       "ou1",
		Attributes: json.RawMessage(`{"email":"test@example.com"}`),
	}

	suite.mockService.On("GetEntity", mock.Anything, "provisioned-user-123").
		Return(entityObj, nil).Once()

	result, err := suite.provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(err)
	suite.NotNil(result.EntityReference)
	suite.Equal("provisioned-user-123", result.EntityReference.EntityID)
	suite.NotNil(result.AuthenticatedClaims)
	suite.Equal("provisioned-user-123", result.AuthenticatedClaims[authnprovidercm.UserAttributeUserID])
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Provisioning_InvalidPayload() {
	credentials := map[string]interface{}{
		"provisionedEntityID": 123,
	}

	result, err := suite.provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Provisioning_EmptyString() {
	credentials := map[string]interface{}{
		"provisionedEntityID": "",
	}

	result, err := suite.provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Provisioning_GetEntityFails() {
	credentials := map[string]interface{}{
		"provisionedEntityID": "provisioned-user-123",
	}

	suite.mockService.On("GetEntity", mock.Anything, "provisioned-user-123").
		Return(nil, errors.New("db error")).Once()

	result, err := suite.provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_IdentifyEntity_ServerError() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := Initialize(suite.mockService, nil, mockOTP, nil, nil, nil)

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": "tok",
			"otp":          "123456",
		},
	}

	otpToken := map[string]interface{}{"mobile_number": "+1234567890"}

	mockOTP.On("Authenticate", mock.Anything, "tok", "123456").
		Return(&authncommon.AuthnResult{
			Token:               otpToken,
			AuthenticatedClaims: map[string]interface{}{"mobile_number": "+1234567890"},
		}, nil).Once()
	suite.mockService.On("IdentifyEntity", mock.Anything, otpToken).
		Return(nil, errors.New("db error")).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_IdentifyEntity_Success_ThenGetEntity() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := Initialize(suite.mockService, nil, mockOTP, nil, nil, nil)

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": "tok",
			"otp":          "123456",
		},
	}

	otpToken := map[string]interface{}{"mobile_number": "+1234567890"}

	entityObj := &providers.Entity{
		ID:         "resolved-id",
		Category:   providers.EntityCategoryUser,
		Type:       "customer",
		State:      providers.EntityStateActive,
		OUID:       "ou1",
		Attributes: json.RawMessage(`{"name":"test"}`),
	}

	mockOTP.On("Authenticate", mock.Anything, "tok", "123456").
		Return(&authncommon.AuthnResult{
			Token:               otpToken,
			AuthenticatedClaims: map[string]interface{}{"mobile_number": "+1234567890"},
		}, nil).Once()
	suite.mockService.On("IdentifyEntity", mock.Anything, otpToken).
		Return(new("resolved-id"), nil).Once()
	suite.mockService.On("GetEntity", mock.Anything, "resolved-id").
		Return(entityObj, nil).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(err)
	suite.NotNil(result.EntityReference)
	suite.Equal("resolved-id", result.EntityReference.EntityID)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_IdentifyEntity_GetEntityFails() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := Initialize(suite.mockService, nil, mockOTP, nil, nil, nil)

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": "tok",
			"otp":          "123456",
		},
	}

	otpToken := map[string]interface{}{"mobile_number": "+1234567890"}

	mockOTP.On("Authenticate", mock.Anything, "tok", "123456").
		Return(&authncommon.AuthnResult{
			Token:               otpToken,
			AuthenticatedClaims: map[string]interface{}{"mobile_number": "+1234567890"},
		}, nil).Once()
	suite.mockService.On("IdentifyEntity", mock.Anything, otpToken).
		Return(new("resolved-id"), nil).Once()
	suite.mockService.On("GetEntity", mock.Anything, "resolved-id").
		Return(nil, errors.New("db error")).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_TokenWithUserID() {
	identifiers := map[string]interface{}{"username": "testuser"}
	credentials := map[string]interface{}{"password": "password123"}

	authResult := &entity.AuthenticateResult{
		EntityID: "user123",
	}

	entityObj := &providers.Entity{
		ID:         "user123",
		Category:   providers.EntityCategoryUser,
		Type:       "customer",
		State:      providers.EntityStateActive,
		OUID:       "ou1",
		Attributes: json.RawMessage(`{}`),
	}

	suite.mockService.On("AuthenticateEntity", mock.Anything, identifiers, credentials).
		Return(authResult, nil).Once()
	suite.mockService.On("GetEntity", mock.Anything, "user123").
		Return(entityObj, nil).Once()

	result, err := suite.provider.Authenticate(context.Background(), identifiers, credentials, nil)

	suite.Nil(err)
	suite.NotNil(result)
	suite.NotNil(result.AuthenticatedClaims)
	suite.Equal("user123", result.AuthenticatedClaims[authnprovidercm.UserAttributeUserID])
}

// --- GetEntityReference tests ---

func (suite *DefaultAuthnProviderTestSuite) TestGetEntityReference_Success() {
	token := map[string]interface{}{"userID": "user123"}

	entityObj := &providers.Entity{
		ID:       "user123",
		Category: providers.EntityCategoryUser,
		Type:     "customer",
		OUID:     "ou1",
	}

	suite.mockService.On("GetEntity", mock.Anything, "user123").
		Return(entityObj, nil).Once()

	ref, err := suite.provider.GetEntityReference(context.Background(), token)

	suite.Nil(err)
	suite.NotNil(ref)
	suite.Equal("user123", ref.EntityID)
	suite.Equal("user", ref.EntityCategory)
	suite.Equal("customer", ref.EntityType)
	suite.Equal("ou1", ref.OUID)
}

func (suite *DefaultAuthnProviderTestSuite) TestGetEntityReference_InvalidTokenFormat() {
	ref, err := suite.provider.GetEntityReference(context.Background(), "invalid-string-token")

	suite.Nil(ref)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestGetEntityReference_EntityNotFound() {
	token := map[string]interface{}{"email": "missing@example.com"}

	suite.mockService.On("IdentifyEntity", mock.Anything, token).
		Return(nil, entity.ErrEntityNotFound).Once()

	ref, err := suite.provider.GetEntityReference(context.Background(), token)

	suite.Nil(ref)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeUserNotFound, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestGetEntityReference_AmbiguousEntity() {
	token := map[string]interface{}{"email": "ambiguous@example.com"}

	suite.mockService.On("IdentifyEntity", mock.Anything, token).
		Return(nil, entity.ErrAmbiguousEntity).Once()

	ref, err := suite.provider.GetEntityReference(context.Background(), token)

	suite.Nil(ref)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeAmbiguousUser, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestGetEntityReference_IdentifyServerError() {
	token := map[string]interface{}{"email": "user1@example.com"}

	suite.mockService.On("IdentifyEntity", mock.Anything, token).
		Return(nil, errors.New("db error")).Once()

	ref, err := suite.provider.GetEntityReference(context.Background(), token)

	suite.Nil(ref)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestGetEntityReference_GetEntityFails() {
	token := map[string]interface{}{"userID": "user1"}

	suite.mockService.On("GetEntity", mock.Anything, "user1").
		Return(nil, errors.New("db error")).Once()

	ref, err := suite.provider.GetEntityReference(context.Background(), token)

	suite.Nil(ref)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// --- GetAttributes tests ---

func (suite *DefaultAuthnProviderTestSuite) TestGetAttributes_Success_All() {
	token := map[string]interface{}{"userID": "user123"}
	entityObj := &providers.Entity{
		ID:         "user123",
		Category:   providers.EntityCategoryUser,
		Type:       "customer",
		OUID:       "ou1",
		Attributes: json.RawMessage(`{"email":"test@example.com", "age": 30}`),
	}

	suite.mockService.On("GetEntity", mock.Anything, "user123").
		Return(entityObj, nil).Once()

	result, err := suite.provider.GetAttributes(context.Background(), token, nil, nil)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("test@example.com", result.Attributes["email"].Value)
	suite.Equal(float64(30), result.Attributes["age"].Value)
}

func (suite *DefaultAuthnProviderTestSuite) TestGetAttributes_Success_Filtered() {
	token := map[string]interface{}{"userID": "user123"}
	entityObj := &providers.Entity{
		ID:         "user123",
		Category:   providers.EntityCategoryUser,
		Type:       "customer",
		OUID:       "ou1",
		Attributes: json.RawMessage(`{"email":"test@example.com", "age": 30}`),
	}

	suite.mockService.On("GetEntity", mock.Anything, "user123").
		Return(entityObj, nil).Once()

	reqAttrs := &providers.RequestedAttributes{
		Attributes: map[string]*providers.AttributeMetadataRequest{
			"email": nil,
		},
	}
	result, err := suite.provider.GetAttributes(context.Background(), token, reqAttrs, nil)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("test@example.com", result.Attributes["email"].Value)
	suite.NotContains(result.Attributes, "age")
}

func (suite *DefaultAuthnProviderTestSuite) TestGetAttributes_EmptyAttributes() {
	token := map[string]interface{}{"userID": "user123"}
	entityObj := &providers.Entity{
		ID:         "user123",
		Category:   providers.EntityCategoryUser,
		Type:       "customer",
		OUID:       "ou1",
		Attributes: nil,
	}

	suite.mockService.On("GetEntity", mock.Anything, "user123").
		Return(entityObj, nil).Once()

	result, err := suite.provider.GetAttributes(context.Background(), token, nil, nil)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Len(result.Attributes, 0)
}

func (suite *DefaultAuthnProviderTestSuite) TestGetAttributes_InvalidAttributeJSON() {
	token := map[string]interface{}{"userID": "user123"}
	entityObj := &providers.Entity{
		ID:         "user123",
		Category:   providers.EntityCategoryUser,
		Type:       "customer",
		OUID:       "ou1",
		Attributes: json.RawMessage(`{invalid`),
	}

	suite.mockService.On("GetEntity", mock.Anything, "user123").
		Return(entityObj, nil).Once()

	result, err := suite.provider.GetAttributes(context.Background(), token, nil, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestGetAttributes_IdentifyEntityFails() {
	token := map[string]interface{}{"email": "test@example.com"}

	suite.mockService.On("IdentifyEntity", mock.Anything, token).
		Return(nil, errors.New("db error")).Once()

	result, err := suite.provider.GetAttributes(context.Background(), token, nil, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestGetAttributes_GetEntityFails() {
	token := map[string]interface{}{"userID": "user123"}

	suite.mockService.On("GetEntity", mock.Anything, "user123").
		Return(nil, errors.New("db error")).Once()

	result, err := suite.provider.GetAttributes(context.Background(), token, nil, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestGetAttributes_EntityNotFound() {
	token := map[string]interface{}{"email": "missing@example.com"}

	suite.mockService.On("IdentifyEntity", mock.Anything, token).
		Return(nil, entity.ErrEntityNotFound).Once()

	result, err := suite.provider.GetAttributes(context.Background(), token, nil, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeUserNotFound, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestGetAttributes_AmbiguousEntity() {
	token := map[string]interface{}{"email": "ambiguous@example.com"}

	suite.mockService.On("IdentifyEntity", mock.Anything, token).
		Return(nil, entity.ErrAmbiguousEntity).Once()

	result, err := suite.provider.GetAttributes(context.Background(), token, nil, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeAmbiguousUser, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestGetAttributes_InvalidTokenFormat() {
	result, err := suite.provider.GetAttributes(context.Background(), "invalid-string", nil, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// --- OTP authentication tests ---

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_OTP_IncorrectOTP() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := Initialize(suite.mockService, nil, mockOTP, nil, nil, nil)

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": "tok",
			"otp":          "wrong",
		},
	}

	mockOTP.On("Authenticate", mock.Anything, "tok", "wrong").
		Return(nil, &otp.ErrorIncorrectOTP).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeAuthenticationFailed, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_OTP_InvalidPayload() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := Initialize(suite.mockService, nil, mockOTP, nil, nil, nil)

	credentials := map[string]interface{}{
		"otp": "not-a-map",
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_OTP_MissingSessionToken() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := Initialize(suite.mockService, nil, mockOTP, nil, nil, nil)

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"otp": "123456",
		},
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_OTP_MissingOTPValue() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := Initialize(suite.mockService, nil, mockOTP, nil, nil, nil)

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": "tok",
		},
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_OTP_ClientError_NonIncorrectOTP() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := Initialize(suite.mockService, nil, mockOTP, nil, nil, nil)

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": "tok",
			"otp":          "123456",
		},
	}

	mockOTP.On("Authenticate", mock.Anything, "tok", "123456").
		Return(nil, &tidcommon.ServiceError{
			Type:             tidcommon.ClientErrorType,
			Code:             "OTHER-CLIENT-ERROR",
			Error:            tidcommon.I18nMessage{DefaultValue: "Some client error"},
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Some client error description"},
		}).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_OTP_ServerError() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := Initialize(suite.mockService, nil, mockOTP, nil, nil, nil)

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": "tok",
			"otp":          "123456",
		},
	}

	mockOTP.On("Authenticate", mock.Anything, "tok", "123456").
		Return(nil, &tidcommon.ServiceError{
			Type:             tidcommon.ServerErrorType,
			Code:             "INTERNAL",
			Error:            tidcommon.I18nMessage{DefaultValue: "Internal error"},
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Something went wrong"},
		}).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// --- Magic Link authentication tests ---

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_MagicLink_AuthenticationFailed() {
	mockML := magiclinkmock.NewMagicLinkAuthnServiceInterfaceMock(suite.T())
	provider := Initialize(suite.mockService, nil, nil, mockML, nil, nil)

	credentials := map[string]interface{}{
		"magiclink": map[string]interface{}{
			"token": "expired-token",
		},
	}

	mockML.On("Authenticate", mock.Anything, "expired-token", "").
		Return(nil, &tidcommon.ServiceError{
			Type:             tidcommon.ClientErrorType,
			Code:             "AUTHN-ML-1002",
			Error:            tidcommon.I18nMessage{DefaultValue: "Expired token"},
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "The magic link token has expired"},
		}).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeAuthenticationFailed, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_MagicLink_ServerError() {
	mockML := magiclinkmock.NewMagicLinkAuthnServiceInterfaceMock(suite.T())
	provider := Initialize(suite.mockService, nil, nil, mockML, nil, nil)

	credentials := map[string]interface{}{
		"magiclink": map[string]interface{}{
			"token": "valid-token",
		},
	}

	mockML.On("Authenticate", mock.Anything, "valid-token", "").
		Return(nil, &tidcommon.ServiceError{
			Type:             tidcommon.ServerErrorType,
			Code:             "INTERNAL",
			Error:            tidcommon.I18nMessage{DefaultValue: "Internal error"},
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Something went wrong"},
		}).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_MagicLink_InvalidPayload() {
	mockML := magiclinkmock.NewMagicLinkAuthnServiceInterfaceMock(suite.T())
	provider := Initialize(suite.mockService, nil, nil, mockML, nil, nil)

	credentials := map[string]interface{}{
		"magiclink": "not-a-map",
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_MagicLink_MissingToken() {
	mockML := magiclinkmock.NewMagicLinkAuthnServiceInterfaceMock(suite.T())
	provider := Initialize(suite.mockService, nil, nil, mockML, nil, nil)

	credentials := map[string]interface{}{
		"magiclink": map[string]interface{}{},
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

// --- Tokenized credential authentication tests (OTP + MagicLink) ---

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_TokenizedAuth_EntityFound() {
	setupOTP := func() (authnprovider.AuthnProviderInterface, map[string]interface{}, map[string]interface{}) {
		mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
		token := map[string]interface{}{"mobile_number": "+1234567890"}
		mockOTP.On("Authenticate", mock.Anything, "tok", "123456").
			Return(&authncommon.AuthnResult{
				Token:               token,
				AuthenticatedClaims: map[string]interface{}{"mobile_number": "+1234567890"},
			}, nil).Once()
		creds := map[string]interface{}{
			"otp": map[string]interface{}{
				"sessionToken": "tok",
				"otp":          "123456",
			},
		}
		return Initialize(suite.mockService, nil, mockOTP, nil, nil, nil), creds, token
	}

	setupMagicLink := func() (authnprovider.AuthnProviderInterface, map[string]interface{}, map[string]interface{}) {
		mockML := magiclinkmock.NewMagicLinkAuthnServiceInterfaceMock(suite.T())
		token := map[string]interface{}{"email": "test@example.com"}
		mockML.On("Authenticate", mock.Anything, "valid-jwt-token", "").
			Return(&authncommon.AuthnResult{
				Token:               token,
				AuthenticatedClaims: map[string]interface{}{"email": "test@example.com"},
			}, nil).Once()
		creds := map[string]interface{}{
			"magiclink": map[string]interface{}{
				"token":            "valid-jwt-token",
				"subjectAttribute": "",
			},
		}
		return Initialize(suite.mockService, nil, nil, mockML, nil, nil), creds, token
	}

	tests := []struct {
		name  string
		setup func() (authnprovider.AuthnProviderInterface, map[string]interface{}, map[string]interface{})
	}{
		{name: "OTP", setup: setupOTP},
		{name: "MagicLink", setup: setupMagicLink},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			provider, credentials, token := tc.setup()

			entityObj := &providers.Entity{
				ID:         "u1",
				Category:   providers.EntityCategoryUser,
				Type:       "customer",
				OUID:       "ou1",
				Attributes: json.RawMessage(`{}`),
			}

			suite.mockService.On("IdentifyEntity", mock.Anything, token).
				Return(new("u1"), nil).Once()
			suite.mockService.On("GetEntity", mock.Anything, "u1").Return(entityObj, nil).Once()

			result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

			suite.Nil(err)
			suite.NotNil(result)
			suite.NotNil(result.EntityReference)
			suite.Equal("u1", result.EntityReference.EntityID)
		})
	}
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_TokenizedAuth_IdentifyEntityErrorReturnsTokens() {
	setupOTP := func() (authnprovider.AuthnProviderInterface, map[string]interface{}, map[string]interface{}) {
		mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
		token := map[string]interface{}{"mobile_number": "+1234567890"}
		mockOTP.On("Authenticate", mock.Anything, "tok", "123456").
			Return(&authncommon.AuthnResult{
				Token:               token,
				AuthenticatedClaims: map[string]interface{}{"mobile_number": "+1234567890"},
			}, nil).Once()
		creds := map[string]interface{}{
			"otp": map[string]interface{}{
				"sessionToken": "tok",
				"otp":          "123456",
			},
		}
		return Initialize(suite.mockService, nil, mockOTP, nil, nil, nil), creds, token
	}

	setupMagicLink := func() (authnprovider.AuthnProviderInterface, map[string]interface{}, map[string]interface{}) {
		mockML := magiclinkmock.NewMagicLinkAuthnServiceInterfaceMock(suite.T())
		token := map[string]interface{}{"email": "test@example.com"}
		mockML.On("Authenticate", mock.Anything, "valid-jwt-token", "email").
			Return(&authncommon.AuthnResult{
				Token:               token,
				AuthenticatedClaims: map[string]interface{}{"email": "test@example.com"},
			}, nil).Once()
		creds := map[string]interface{}{
			"magiclink": map[string]interface{}{
				"token":            "valid-jwt-token",
				"subjectAttribute": "email",
			},
		}
		return Initialize(suite.mockService, nil, nil, mockML, nil, nil), creds, token
	}

	tests := []struct {
		name        string
		setup       func() (authnprovider.AuthnProviderInterface, map[string]interface{}, map[string]interface{})
		identifyErr error
	}{
		{name: "OTP_EntityNotFound", setup: setupOTP, identifyErr: entity.ErrEntityNotFound},
		{name: "OTP_AmbiguousEntity", setup: setupOTP, identifyErr: entity.ErrAmbiguousEntity},
		{name: "MagicLink_EntityNotFound", setup: setupMagicLink, identifyErr: entity.ErrEntityNotFound},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			provider, credentials, token := tc.setup()

			suite.mockService.On("IdentifyEntity", mock.Anything, token).
				Return(nil, tc.identifyErr).Once()

			result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

			suite.Nil(err)
			suite.NotNil(result)
			suite.Nil(result.EntityReference)
			suite.NotNil(result.EntityReferenceToken)
			suite.NotNil(result.AttributeToken)
		})
	}
}

// --- Passkey authentication tests ---

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Passkey_InvalidPayload() {
	provider := Initialize(suite.mockService, nil, nil, nil, nil, nil)

	credentials := map[string]interface{}{
		"passkey": "not-a-passkey-struct",
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Passkey_NilPayload() {
	provider := Initialize(suite.mockService, nil, nil, nil, nil, nil)

	credentials := map[string]interface{}{
		"passkey": (*passkey.PasskeyAuthenticationFinishRequest)(nil),
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

// --- Federated authentication tests ---

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Federated_InvalidPayload() {
	provider := Initialize(suite.mockService, nil, nil, nil, nil, nil)

	credentials := map[string]interface{}{
		"federated": "not-a-struct",
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Federated_NilPayload() {
	provider := Initialize(suite.mockService, nil, nil, nil, nil, nil)

	credentials := map[string]interface{}{
		"federated": (*authncommon.FederatedAuthCredential)(nil),
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Federated_MissingIDPID() {
	provider := Initialize(suite.mockService, nil, nil, nil, nil, nil)

	credentials := map[string]interface{}{
		"federated": &authncommon.FederatedAuthCredential{
			IDPType:           providers.IDPType("google"),
			AuthorizationData: authncommon.AuthorizationData{Code: "auth-code"},
		},
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Federated_MissingCode() {
	provider := Initialize(suite.mockService, nil, nil, nil, nil, nil)

	credentials := map[string]interface{}{
		"federated": &authncommon.FederatedAuthCredential{
			IDPID:   "idp-1",
			IDPType: providers.IDPType("google"),
		},
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Federated_UnsupportedIDPType() {
	provider := Initialize(suite.mockService, nil, nil, nil, nil,
		map[providers.IDPType]authncommon.FederatedAuthenticator{})

	credentials := map[string]interface{}{
		"federated": &authncommon.FederatedAuthCredential{
			IDPID:             "idp-1",
			IDPType:           providers.IDPType("unsupported"),
			AuthorizationData: authncommon.AuthorizationData{Code: "auth-code"},
		},
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

// --- Passkey success/error tests ---

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Passkey_Success() {
	passkeyToken := map[string]interface{}{"userID": "pk-user-1"}
	suite.mockPasskey.On("FinishAuthentication", mock.Anything, mock.Anything).
		Return(&authncommon.AuthnResult{
			Token:               passkeyToken,
			AuthenticatedClaims: map[string]interface{}{"userID": "pk-user-1"},
		}, nil).Once()
	provider := Initialize(suite.mockService, suite.mockPasskey, nil, nil, nil, nil)

	credentials := map[string]interface{}{
		"passkey": &passkey.PasskeyAuthenticationFinishRequest{
			CredentialID: "cred-1",
		},
	}

	entityObj := &providers.Entity{
		ID:         "pk-user-1",
		Category:   providers.EntityCategoryUser,
		Type:       "customer",
		OUID:       "ou1",
		Attributes: json.RawMessage(`{}`),
	}

	suite.mockService.On("GetEntity", mock.Anything, "pk-user-1").
		Return(entityObj, nil).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(err)
	suite.NotNil(result)
	suite.NotNil(result.EntityReference)
	suite.Equal("pk-user-1", result.EntityReference.EntityID)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Passkey_AuthFailed() {
	suite.mockPasskey.On("FinishAuthentication", mock.Anything, mock.Anything).
		Return(nil, &tidcommon.ServiceError{
			Type:             tidcommon.ClientErrorType,
			Code:             "PASSKEY-001",
			Error:            tidcommon.I18nMessage{DefaultValue: "Passkey auth failed"},
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Invalid passkey credential"},
		}).Once()
	provider := Initialize(suite.mockService, suite.mockPasskey, nil, nil, nil, nil)

	credentials := map[string]interface{}{
		"passkey": &passkey.PasskeyAuthenticationFinishRequest{
			CredentialID: "cred-1",
		},
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeAuthenticationFailed, err.Code)
}

// --- Federated success/error tests ---

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Federated_Success() {
	fedToken := map[string]interface{}{"sub": "fed-sub-1"}
	suite.mockFederated.On("Authenticate", mock.Anything, "idp-1",
		authncommon.AuthorizationData{Code: "auth-code"}).
		Return(&authncommon.AuthnResult{
			Token:               fedToken,
			AuthenticatedClaims: map[string]interface{}{"sub": "fed-sub-1"},
		}, nil).Once()
	federatedAuths := map[providers.IDPType]authncommon.FederatedAuthenticator{
		providers.IDPType("google"): suite.mockFederated,
	}
	provider := Initialize(suite.mockService, nil, nil, nil, nil, federatedAuths)

	credentials := map[string]interface{}{
		"federated": &authncommon.FederatedAuthCredential{
			IDPID:             "idp-1",
			IDPType:           providers.IDPType("google"),
			AuthorizationData: authncommon.AuthorizationData{Code: "auth-code"},
		},
	}

	entityObj := &providers.Entity{
		ID:         "fed-user-1",
		Category:   providers.EntityCategoryUser,
		Type:       "customer",
		OUID:       "ou1",
		Attributes: json.RawMessage(`{}`),
	}

	suite.mockService.On("IdentifyEntity", mock.Anything, fedToken).
		Return(new("fed-user-1"), nil).Once()
	suite.mockService.On("GetEntity", mock.Anything, "fed-user-1").
		Return(entityObj, nil).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(err)
	suite.NotNil(result)
	suite.NotNil(result.EntityReference)
	suite.Equal("fed-user-1", result.EntityReference.EntityID)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Federated_ClientError() {
	suite.mockFederated.On("Authenticate", mock.Anything, "idp-1",
		authncommon.AuthorizationData{Code: "auth-code"}).
		Return(nil, &tidcommon.ServiceError{
			Type:             tidcommon.ClientErrorType,
			Code:             "FED-001",
			Error:            tidcommon.I18nMessage{DefaultValue: "Fed auth failed"},
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Invalid auth code"},
		}).Once()
	federatedAuths := map[providers.IDPType]authncommon.FederatedAuthenticator{
		providers.IDPType("google"): suite.mockFederated,
	}
	provider := Initialize(suite.mockService, nil, nil, nil, nil, federatedAuths)

	credentials := map[string]interface{}{
		"federated": &authncommon.FederatedAuthCredential{
			IDPID:             "idp-1",
			IDPType:           providers.IDPType("google"),
			AuthorizationData: authncommon.AuthorizationData{Code: "auth-code"},
		},
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeAuthenticationFailed, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Federated_ServerError() {
	suite.mockFederated.On("Authenticate", mock.Anything, "idp-1",
		authncommon.AuthorizationData{Code: "auth-code"}).
		Return(nil, &tidcommon.ServiceError{
			Type:             tidcommon.ServerErrorType,
			Code:             "INTERNAL",
			Error:            tidcommon.I18nMessage{DefaultValue: "Internal error"},
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Something went wrong"},
		}).Once()
	federatedAuths := map[providers.IDPType]authncommon.FederatedAuthenticator{
		providers.IDPType("google"): suite.mockFederated,
	}
	provider := Initialize(suite.mockService, nil, nil, nil, nil, federatedAuths)

	credentials := map[string]interface{}{
		"federated": &authncommon.FederatedAuthCredential{
			IDPID:             "idp-1",
			IDPType:           providers.IDPType("google"),
			AuthorizationData: authncommon.AuthorizationData{Code: "auth-code"},
		},
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestInitiateAuthentication_Passkey() {
	provider := Initialize(suite.mockService, suite.mockPasskey, nil, nil, nil, nil)
	req := &passkey.PasskeyAuthenticationStartRequest{UserID: "user123", RelyingPartyID: "example.com"}
	startData := &passkey.PasskeyAuthenticationStartData{SessionToken: "sess-1"}
	suite.mockPasskey.On("StartAuthentication", mock.Anything, req).Return(startData, nil).Once()

	result, err := provider.InitiateAuthentication(context.Background(), passkey.CredentialType, req, nil)

	suite.Nil(err)
	suite.Equal(startData, result)
}

func (suite *DefaultAuthnProviderTestSuite) TestInitiateAuthentication_UnsupportedType() {
	result, err := suite.provider.InitiateAuthentication(context.Background(), "otp", nil, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.ServerErrorType, err.Type)
}

func (suite *DefaultAuthnProviderTestSuite) TestInitiateAuthentication_InvalidPayload() {
	provider := Initialize(suite.mockService, suite.mockPasskey, nil, nil, nil, nil)

	result, err := provider.InitiateAuthentication(context.Background(), passkey.CredentialType, "bad", nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestInitiateEnrollment_Passkey() {
	provider := Initialize(suite.mockService, suite.mockPasskey, nil, nil, nil, nil)
	req := &passkey.PasskeyRegistrationStartRequest{UserID: "user123", RelyingPartyID: "example.com"}
	startData := &passkey.PasskeyRegistrationStartData{SessionToken: "sess-1"}
	suite.mockPasskey.On("StartRegistration", mock.Anything, req).Return(startData, nil).Once()

	result, err := provider.InitiateEnrollment(context.Background(), passkey.CredentialType, req, nil)

	suite.Nil(err)
	suite.Equal(startData, result)
}

func (suite *DefaultAuthnProviderTestSuite) TestInitiateEnrollment_InvalidPayload() {
	provider := Initialize(suite.mockService, suite.mockPasskey, nil, nil, nil, nil)

	result, err := provider.InitiateEnrollment(context.Background(), passkey.CredentialType, 42, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestEnroll_Passkey_Success() {
	provider := Initialize(suite.mockService, suite.mockPasskey, nil, nil, nil, nil)
	req := &passkey.PasskeyRegistrationFinishRequest{CredentialID: "cred-1"}
	credentials := map[string]interface{}{"passkey": req}
	suite.mockPasskey.On("FinishRegistration", mock.Anything, req).
		Return(&authncommon.AuthnResult{
			Token:               map[string]interface{}{"userID": "user123"},
			AuthenticatedClaims: map[string]interface{}{"userID": "user123"},
		}, nil).Once()

	entityObj := &providers.Entity{
		ID:         "user123",
		Category:   providers.EntityCategoryUser,
		Type:       "customer",
		OUID:       "ou1",
		Attributes: json.RawMessage(`{}`),
	}
	suite.mockService.On("GetEntity", mock.Anything, "user123").Return(entityObj, nil).Once()

	result, err := provider.Enroll(context.Background(), nil, credentials, nil)

	suite.Nil(err)
	suite.NotNil(result.EntityReference)
	suite.Equal("user123", result.EntityReference.EntityID)
}

func (suite *DefaultAuthnProviderTestSuite) TestEnroll_NilCredentials() {
	result, err := suite.provider.Enroll(context.Background(), nil, nil, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeEnrollmentFailed, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestEnroll_Passkey_InvalidPayload() {
	provider := Initialize(suite.mockService, suite.mockPasskey, nil, nil, nil, nil)
	credentials := map[string]interface{}{"passkey": "not-a-request-struct"}

	result, err := provider.Enroll(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestEnroll_UnsupportedCredential() {
	credentials := map[string]interface{}{"unsupported": "x"}

	result, err := suite.provider.Enroll(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.ServerErrorType, err.Type)
}
