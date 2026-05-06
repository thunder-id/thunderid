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

package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/asgardeo/thunder/internal/authn/otp"
	authnprovidercm "github.com/asgardeo/thunder/internal/authnprovider/common"
	"github.com/asgardeo/thunder/internal/entity"
	"github.com/asgardeo/thunder/internal/entityprovider"
	"github.com/asgardeo/thunder/tests/mocks/authn/otpmock"
	"github.com/asgardeo/thunder/tests/mocks/entitymock"
)

type DefaultAuthnProviderTestSuite struct {
	suite.Suite
	mockService *entitymock.EntityServiceInterfaceMock
	provider    AuthnProviderInterface
}

func (suite *DefaultAuthnProviderTestSuite) SetupTest() {
	suite.mockService = entitymock.NewEntityServiceInterfaceMock(suite.T())
	suite.provider = newDefaultAuthnProvider(suite.mockService, nil, nil, nil)
}

func TestDefaultAuthnProviderTestSuite(t *testing.T) {
	suite.Run(t, new(DefaultAuthnProviderTestSuite))
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Success() {
	identifiers := map[string]interface{}{"username": "testuser"}
	credentials := map[string]interface{}{"password": "password123"}

	authResult := &entity.AuthenticateResult{
		EntityID:       "user123",
		EntityCategory: entity.EntityCategoryUser,
		EntityType:     "customer",
		OUID:           "ou1",
	}

	entityObj := &entity.Entity{
		ID:         "user123",
		Category:   entity.EntityCategoryUser,
		Type:       "customer",
		State:      entity.EntityStateActive,
		OUID:       "ou1",
		Attributes: json.RawMessage(`{"email":"test@example.com"}`),
	}

	suite.mockService.On("AuthenticateEntity", mock.Anything, identifiers, credentials).
		Return(authResult, nil).Once()
	suite.mockService.On("GetEntity", mock.Anything, "user123").
		Return(entityObj, nil).Once()

	result, err := suite.provider.Authenticate(context.Background(), authnprovidercm.AuthnDataTypeCredentials,
		&authnprovidercm.CredentialsAuthnData{
			Identifiers: identifiers, Credentials: credentials,
		}, nil)

	suite.Nil(err)
	suite.Equal("user123", result.EntityID)
	suite.Equal("user", result.EntityCategory)
	suite.Equal("customer", result.EntityType)
	suite.Equal("user123", result.UserID)
	suite.Equal("user123", result.Token)
	suite.Equal("customer", result.UserType)
	suite.Equal("ou1", result.OUID)
	suite.True(result.IsAttributeValuesIncluded)
	suite.NotNil(result.AttributesResponse)
	suite.Len(result.AttributesResponse.Attributes, 1)
	suite.Contains(result.AttributesResponse.Attributes, "email")
	suite.Equal("test@example.com", result.AttributesResponse.Attributes["email"].Value)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_EntityNotFound() {
	identifiers := map[string]interface{}{"username": "unknown"}
	credentials := map[string]interface{}{"password": "password"}

	suite.mockService.On("AuthenticateEntity", mock.Anything, identifiers, credentials).
		Return(nil, entity.ErrEntityNotFound).Once()

	result, err := suite.provider.Authenticate(context.Background(), authnprovidercm.AuthnDataTypeCredentials,
		&authnprovidercm.CredentialsAuthnData{
			Identifiers: identifiers, Credentials: credentials,
		}, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeUserNotFound, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_AuthenticationFailed() {
	identifiers := map[string]interface{}{"username": "testuser"}
	credentials := map[string]interface{}{"password": "wrongpassword"}

	suite.mockService.On("AuthenticateEntity", mock.Anything, identifiers, credentials).
		Return(nil, entity.ErrAuthenticationFailed).Once()

	result, err := suite.provider.Authenticate(context.Background(), authnprovidercm.AuthnDataTypeCredentials,
		&authnprovidercm.CredentialsAuthnData{
			Identifiers: identifiers, Credentials: credentials,
		}, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeAuthenticationFailed, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_GetEntityNotFound() {
	identifiers := map[string]interface{}{"username": "testuser"}
	credentials := map[string]interface{}{"password": "password123"}

	authResult := &entity.AuthenticateResult{
		EntityID:       "user123",
		EntityCategory: entity.EntityCategoryUser,
		EntityType:     "customer",
		OUID:           "ou1",
	}

	suite.mockService.On("AuthenticateEntity", mock.Anything, identifiers, credentials).
		Return(authResult, nil).Once()
	suite.mockService.On("GetEntity", mock.Anything, "user123").
		Return(nil, entity.ErrEntityNotFound).Once()

	result, err := suite.provider.Authenticate(context.Background(), authnprovidercm.AuthnDataTypeCredentials,
		&authnprovidercm.CredentialsAuthnData{
			Identifiers: identifiers, Credentials: credentials,
		}, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeUserNotFound, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_ByPreResolvedUserID_Success() {
	identifiers := map[string]interface{}{"userID": "resolved-user-123"}
	credentials := map[string]interface{}{"password": "password123"}

	authResult := &entity.AuthenticateResult{
		EntityID:       "resolved-user-123",
		EntityCategory: entity.EntityCategoryUser,
		EntityType:     "customer",
		OUID:           "ou1",
	}

	entityObj := &entity.Entity{
		ID:         "resolved-user-123",
		Category:   entity.EntityCategoryUser,
		Type:       "customer",
		State:      entity.EntityStateActive,
		OUID:       "ou1",
		Attributes: json.RawMessage(`{"email":"test@example.com"}`),
	}

	suite.mockService.On("AuthenticateEntityByID", mock.Anything, "resolved-user-123", credentials).
		Return(authResult, nil).Once()
	suite.mockService.On("GetEntity", mock.Anything, "resolved-user-123").
		Return(entityObj, nil).Once()

	result, err := suite.provider.Authenticate(context.Background(), authnprovidercm.AuthnDataTypeCredentials,
		&authnprovidercm.CredentialsAuthnData{
			Identifiers: identifiers, Credentials: credentials,
		}, nil)

	suite.Nil(err)
	suite.Equal("resolved-user-123", result.EntityID)
	suite.Equal("resolved-user-123", result.UserID)
	suite.Equal("customer", result.UserType)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_ByPreResolvedUserID_EntityNotFound() {
	identifiers := map[string]interface{}{"userID": "missing-user"}
	credentials := map[string]interface{}{"password": "password123"}

	suite.mockService.On("AuthenticateEntityByID", mock.Anything, "missing-user", credentials).
		Return(nil, entity.ErrEntityNotFound).Once()

	result, err := suite.provider.Authenticate(context.Background(), authnprovidercm.AuthnDataTypeCredentials,
		&authnprovidercm.CredentialsAuthnData{
			Identifiers: identifiers, Credentials: credentials,
		}, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeUserNotFound, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_ByPreResolvedUserID_AuthFailed() {
	identifiers := map[string]interface{}{"userID": "user-wrong-pw"}
	credentials := map[string]interface{}{"password": "wrongpassword"}

	suite.mockService.On("AuthenticateEntityByID", mock.Anything, "user-wrong-pw", credentials).
		Return(nil, entity.ErrAuthenticationFailed).Once()

	result, err := suite.provider.Authenticate(context.Background(), authnprovidercm.AuthnDataTypeCredentials,
		&authnprovidercm.CredentialsAuthnData{
			Identifiers: identifiers, Credentials: credentials,
		}, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeAuthenticationFailed, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_EmptyUserID_FallsBackToIdentify() {
	identifiers := map[string]interface{}{"userID": "", "username": "testuser"}
	credentials := map[string]interface{}{"password": "password123"}

	authResult := &entity.AuthenticateResult{
		EntityID:       "user123",
		EntityCategory: entity.EntityCategoryUser,
		EntityType:     "customer",
		OUID:           "ou1",
	}

	entityObj := &entity.Entity{
		ID:         "user123",
		Category:   entity.EntityCategoryUser,
		Type:       "customer",
		State:      entity.EntityStateActive,
		OUID:       "ou1",
		Attributes: json.RawMessage(`{"email":"test@example.com"}`),
	}

	suite.mockService.On("AuthenticateEntity", mock.Anything, identifiers, credentials).
		Return(authResult, nil).Once()
	suite.mockService.On("GetEntity", mock.Anything, "user123").
		Return(entityObj, nil).Once()

	result, err := suite.provider.Authenticate(context.Background(), authnprovidercm.AuthnDataTypeCredentials,
		&authnprovidercm.CredentialsAuthnData{
			Identifiers: identifiers, Credentials: credentials,
		}, nil)

	suite.Nil(err)
	suite.Equal("user123", result.EntityID)
}

func (suite *DefaultAuthnProviderTestSuite) TestGetAttributes_Success_All() {
	token := "user123"
	entityObj := &entity.Entity{
		ID:         "user123",
		Category:   entity.EntityCategoryUser,
		Type:       "customer",
		OUID:       "ou1",
		Attributes: json.RawMessage(`{"email":"test@example.com", "age": 30}`),
	}

	suite.mockService.On("GetEntity", mock.Anything, token).
		Return(entityObj, nil).Once()

	result, err := suite.provider.GetAttributes(context.Background(), token, nil, nil)

	suite.Nil(err)
	suite.Equal("user123", result.EntityID)
	suite.Equal("user123", result.UserID)
	suite.NotNil(result.AttributesResponse)
	suite.Equal("test@example.com", result.AttributesResponse.Attributes["email"].Value)
	suite.Equal(float64(30), result.AttributesResponse.Attributes["age"].Value)
}

func (suite *DefaultAuthnProviderTestSuite) TestGetAttributes_Success_Filtered() {
	token := "user123"
	entityObj := &entity.Entity{
		ID:         "user123",
		Category:   entity.EntityCategoryUser,
		Type:       "customer",
		OUID:       "ou1",
		Attributes: json.RawMessage(`{"email":"test@example.com", "age": 30}`),
	}

	suite.mockService.On("GetEntity", mock.Anything, token).
		Return(entityObj, nil).Once()

	reqAttrs := &authnprovidercm.RequestedAttributes{
		Attributes: map[string]*authnprovidercm.AttributeMetadataRequest{
			"email": nil,
		},
	}
	result, err := suite.provider.GetAttributes(context.Background(), token, reqAttrs, nil)

	suite.Nil(err)
	suite.Equal("user123", result.UserID)
	suite.NotNil(result.AttributesResponse)
	suite.Equal("test@example.com", result.AttributesResponse.Attributes["email"].Value)
	suite.NotContains(result.AttributesResponse.Attributes, "age")
}

func (suite *DefaultAuthnProviderTestSuite) TestGetAttributes_InvalidToken() {
	token := "invalid"

	suite.mockService.On("GetEntity", mock.Anything, token).
		Return(nil, entity.ErrEntityNotFound).Once()

	result, err := suite.provider.GetAttributes(context.Background(), token, nil, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidToken, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_OTP_UserFound() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := newDefaultAuthnProvider(suite.mockService, nil, mockOTP, nil)

	entityObj := &entity.Entity{
		ID:         "u1",
		Category:   entity.EntityCategoryUser,
		Type:       "customer",
		OUID:       "ou1",
		Attributes: json.RawMessage(`{}`),
	}

	mockOTP.On("Authenticate", mock.Anything, "tok", "123456").
		Return(&otp.OTPAuthnResult{InternalEntity: &entityprovider.Entity{ID: "u1"}}, nil).Once()
	suite.mockService.On("GetEntity", mock.Anything, "u1").Return(entityObj, nil).Once()

	result, err := provider.Authenticate(context.Background(), authnprovidercm.AuthnDataTypeOTP,
		&authnprovidercm.OTPAuthnData{
			SessionToken: "tok", OTP: "123456",
		}, nil)

	suite.Nil(err)
	suite.NotNil(result)
	suite.True(result.IsExistingUser)
	suite.Equal("u1", result.UserID)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_OTP_UserNotFound() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := newDefaultAuthnProvider(suite.mockService, nil, mockOTP, nil)

	mockOTP.On("Authenticate", mock.Anything, "tok", "123456").
		Return(&otp.OTPAuthnResult{
			InternalEntity:      nil,
			VerifiedIdentifiers: map[string]interface{}{"mobileNumber": "+1234567890"},
		}, nil).Once()

	result, err := provider.Authenticate(context.Background(), authnprovidercm.AuthnDataTypeOTP,
		&authnprovidercm.OTPAuthnData{
			SessionToken: "tok", OTP: "123456",
		}, nil)

	suite.Nil(err)
	suite.NotNil(result)
	suite.False(result.IsExistingUser)
	suite.True(result.IsAttributeValuesIncluded)
	suite.NotNil(result.AttributesResponse)
	suite.Equal("+1234567890", result.AttributesResponse.Attributes["mobileNumber"].Value)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_OTP_IncorrectOTP() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := newDefaultAuthnProvider(suite.mockService, nil, mockOTP, nil)

	mockOTP.On("Authenticate", mock.Anything, "tok", "wrong").
		Return(nil, &otp.ErrorIncorrectOTP).Once()

	result, err := provider.Authenticate(context.Background(), authnprovidercm.AuthnDataTypeOTP,
		&authnprovidercm.OTPAuthnData{
			SessionToken: "tok", OTP: "wrong",
		}, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeAuthenticationFailed, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_OTP_InvalidPayload() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := newDefaultAuthnProvider(suite.mockService, nil, mockOTP, nil)

	result, err := provider.Authenticate(context.Background(), authnprovidercm.AuthnDataTypeOTP,
		&authnprovidercm.CredentialsAuthnData{}, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}
