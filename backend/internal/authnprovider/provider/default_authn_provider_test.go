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
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/tests/mocks/authn/magiclinkmock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/otpmock"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
)

type DefaultAuthnProviderTestSuite struct {
	suite.Suite
	mockService *entitymock.EntityServiceInterfaceMock
	provider    AuthnProviderInterface
}

func (suite *DefaultAuthnProviderTestSuite) SetupTest() {
	suite.mockService = entitymock.NewEntityServiceInterfaceMock(suite.T())
	suite.provider = newDefaultAuthnProvider(suite.mockService, nil, nil, nil, nil, nil)
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
	suite.Equal(serviceerror.ClientErrorType, err.Type)
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
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_GetEntityFails() {
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
		Return(nil, errors.New("db error")).Once()

	result, err := suite.provider.Authenticate(context.Background(), identifiers, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_GetEntityEmptyAttributes() {
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
		Attributes: json.RawMessage(`{invalid-json`),
	}

	suite.mockService.On("AuthenticateEntity", mock.Anything, identifiers, credentials).
		Return(authResult, nil).Once()
	suite.mockService.On("GetEntity", mock.Anything, "user123").
		Return(entityObj, nil).Once()

	result, err := suite.provider.Authenticate(context.Background(), identifiers, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
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

	result, err := suite.provider.Authenticate(context.Background(), identifiers, credentials, nil)

	suite.Nil(err)
	suite.NotNil(result.EntityReference)
	suite.Equal("user123", result.EntityReference.EntityID)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Provisioning_Success() {
	credentials := map[string]interface{}{
		"provisionedEntityID": "provisioned-user-123",
	}

	entityObj := &entity.Entity{
		ID:         "provisioned-user-123",
		Category:   entity.EntityCategoryUser,
		Type:       "customer",
		State:      entity.EntityStateActive,
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
	suite.Equal("provisioned-user-123", result.AuthenticatedClaims[UserAttributeUserID])
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
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

//nolint:dupl
func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_IdentifyEntity_EntityNotFound_ReturnsTokens() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := newDefaultAuthnProvider(suite.mockService, nil, mockOTP, nil, nil, nil)

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": "tok",
			"otp":          "123456",
		},
	}

	otpToken := map[string]interface{}{"mobileNumber": "+1234567890"}

	mockOTP.On("Authenticate", mock.Anything, "tok", "123456").
		Return(&authncommon.AuthnResult{
			Token:               otpToken,
			AuthenticatedClaims: map[string]interface{}{"mobileNumber": "+1234567890"},
		}, nil).Once()
	suite.mockService.On("IdentifyEntity", mock.Anything, otpToken).
		Return(nil, entity.ErrEntityNotFound).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Nil(result.EntityReference)
	suite.NotNil(result.EntityReferenceToken)
	suite.NotNil(result.AttributeToken)
}

//nolint:dupl
func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_IdentifyEntity_AmbiguousEntity_ReturnsTokens() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := newDefaultAuthnProvider(suite.mockService, nil, mockOTP, nil, nil, nil)

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": "tok",
			"otp":          "123456",
		},
	}

	otpToken := map[string]interface{}{"mobileNumber": "+1234567890"}

	mockOTP.On("Authenticate", mock.Anything, "tok", "123456").
		Return(&authncommon.AuthnResult{
			Token:               otpToken,
			AuthenticatedClaims: map[string]interface{}{"mobileNumber": "+1234567890"},
		}, nil).Once()
	suite.mockService.On("IdentifyEntity", mock.Anything, otpToken).
		Return(nil, entity.ErrAmbiguousEntity).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Nil(result.EntityReference)
	suite.NotNil(result.EntityReferenceToken)
	suite.NotNil(result.AttributeToken)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_IdentifyEntity_ServerError() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := newDefaultAuthnProvider(suite.mockService, nil, mockOTP, nil, nil, nil)

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": "tok",
			"otp":          "123456",
		},
	}

	otpToken := map[string]interface{}{"mobileNumber": "+1234567890"}

	mockOTP.On("Authenticate", mock.Anything, "tok", "123456").
		Return(&authncommon.AuthnResult{
			Token:               otpToken,
			AuthenticatedClaims: map[string]interface{}{"mobileNumber": "+1234567890"},
		}, nil).Once()
	suite.mockService.On("IdentifyEntity", mock.Anything, otpToken).
		Return(nil, errors.New("db error")).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_IdentifyEntity_Success_ThenGetEntity() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := newDefaultAuthnProvider(suite.mockService, nil, mockOTP, nil, nil, nil)

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": "tok",
			"otp":          "123456",
		},
	}

	otpToken := map[string]interface{}{"mobileNumber": "+1234567890"}

	entityObj := &entity.Entity{
		ID:         "resolved-id",
		Category:   entity.EntityCategoryUser,
		Type:       "customer",
		State:      entity.EntityStateActive,
		OUID:       "ou1",
		Attributes: json.RawMessage(`{"name":"test"}`),
	}

	mockOTP.On("Authenticate", mock.Anything, "tok", "123456").
		Return(&authncommon.AuthnResult{
			Token:               otpToken,
			AuthenticatedClaims: map[string]interface{}{"mobileNumber": "+1234567890"},
		}, nil).Once()
	suite.mockService.On("IdentifyEntity", mock.Anything, otpToken).
		Return(strPtr("resolved-id"), nil).Once()
	suite.mockService.On("GetEntity", mock.Anything, "resolved-id").
		Return(entityObj, nil).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(err)
	suite.NotNil(result.EntityReference)
	suite.Equal("resolved-id", result.EntityReference.EntityID)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_IdentifyEntity_GetEntityFails() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := newDefaultAuthnProvider(suite.mockService, nil, mockOTP, nil, nil, nil)

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": "tok",
			"otp":          "123456",
		},
	}

	otpToken := map[string]interface{}{"mobileNumber": "+1234567890"}

	mockOTP.On("Authenticate", mock.Anything, "tok", "123456").
		Return(&authncommon.AuthnResult{
			Token:               otpToken,
			AuthenticatedClaims: map[string]interface{}{"mobileNumber": "+1234567890"},
		}, nil).Once()
	suite.mockService.On("IdentifyEntity", mock.Anything, otpToken).
		Return(strPtr("resolved-id"), nil).Once()
	suite.mockService.On("GetEntity", mock.Anything, "resolved-id").
		Return(nil, errors.New("db error")).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_TokenWithUserID() {
	identifiers := map[string]interface{}{"username": "testuser"}
	credentials := map[string]interface{}{"password": "password123"}

	authResult := &entity.AuthenticateResult{
		EntityID: "user123",
	}

	entityObj := &entity.Entity{
		ID:         "user123",
		Category:   entity.EntityCategoryUser,
		Type:       "customer",
		State:      entity.EntityStateActive,
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
	suite.Equal("user123", result.AuthenticatedClaims[UserAttributeUserID])
}

// --- GetEntityReference tests ---

func (suite *DefaultAuthnProviderTestSuite) TestGetEntityReference_Success() {
	token := map[string]interface{}{"userID": "user123"}

	entityObj := &entity.Entity{
		ID:       "user123",
		Category: entity.EntityCategoryUser,
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
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
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
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestGetEntityReference_GetEntityFails() {
	token := map[string]interface{}{"userID": "user1"}

	suite.mockService.On("GetEntity", mock.Anything, "user1").
		Return(nil, errors.New("db error")).Once()

	ref, err := suite.provider.GetEntityReference(context.Background(), token)

	suite.Nil(ref)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

// --- GetAttributes tests ---

func (suite *DefaultAuthnProviderTestSuite) TestGetAttributes_Success_All() {
	token := map[string]interface{}{"userID": "user123"}
	entityObj := &entity.Entity{
		ID:         "user123",
		Category:   entity.EntityCategoryUser,
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
	entityObj := &entity.Entity{
		ID:         "user123",
		Category:   entity.EntityCategoryUser,
		Type:       "customer",
		OUID:       "ou1",
		Attributes: json.RawMessage(`{"email":"test@example.com", "age": 30}`),
	}

	suite.mockService.On("GetEntity", mock.Anything, "user123").
		Return(entityObj, nil).Once()

	reqAttrs := &authnprovidercm.RequestedAttributes{
		Attributes: map[string]*authnprovidercm.AttributeMetadataRequest{
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
	entityObj := &entity.Entity{
		ID:         "user123",
		Category:   entity.EntityCategoryUser,
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
	entityObj := &entity.Entity{
		ID:         "user123",
		Category:   entity.EntityCategoryUser,
		Type:       "customer",
		OUID:       "ou1",
		Attributes: json.RawMessage(`{invalid`),
	}

	suite.mockService.On("GetEntity", mock.Anything, "user123").
		Return(entityObj, nil).Once()

	result, err := suite.provider.GetAttributes(context.Background(), token, nil, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestGetAttributes_IdentifyEntityFails() {
	token := map[string]interface{}{"email": "test@example.com"}

	suite.mockService.On("IdentifyEntity", mock.Anything, token).
		Return(nil, errors.New("db error")).Once()

	result, err := suite.provider.GetAttributes(context.Background(), token, nil, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestGetAttributes_GetEntityFails() {
	token := map[string]interface{}{"userID": "user123"}

	suite.mockService.On("GetEntity", mock.Anything, "user123").
		Return(nil, errors.New("db error")).Once()

	result, err := suite.provider.GetAttributes(context.Background(), token, nil, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
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
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

// --- OTP authentication tests ---

//nolint:dupl // intentionally mirrors MagicLink tests to cover the OTP credential path
func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_OTP_EntityFound() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := newDefaultAuthnProvider(suite.mockService, nil, mockOTP, nil, nil, nil)

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": "tok",
			"otp":          "123456",
		},
	}

	otpToken := map[string]interface{}{"mobileNumber": "+1234567890"}

	entityObj := &entity.Entity{
		ID:         "u1",
		Category:   entity.EntityCategoryUser,
		Type:       "customer",
		OUID:       "ou1",
		Attributes: json.RawMessage(`{}`),
	}

	mockOTP.On("Authenticate", mock.Anything, "tok", "123456").
		Return(&authncommon.AuthnResult{
			Token:               otpToken,
			AuthenticatedClaims: map[string]interface{}{"mobileNumber": "+1234567890"},
		}, nil).Once()
	suite.mockService.On("IdentifyEntity", mock.Anything, otpToken).
		Return(strPtr("u1"), nil).Once()
	suite.mockService.On("GetEntity", mock.Anything, "u1").Return(entityObj, nil).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(err)
	suite.NotNil(result)
	suite.NotNil(result.EntityReference)
	suite.Equal("u1", result.EntityReference.EntityID)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_OTP_EntityNotFound() { //nolint:dupl
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := newDefaultAuthnProvider(suite.mockService, nil, mockOTP, nil, nil, nil)

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": "tok",
			"otp":          "123456",
		},
	}

	otpToken := map[string]interface{}{"mobileNumber": "+1234567890"}

	mockOTP.On("Authenticate", mock.Anything, "tok", "123456").
		Return(&authncommon.AuthnResult{
			Token:               otpToken,
			AuthenticatedClaims: map[string]interface{}{"mobileNumber": "+1234567890"},
		}, nil).Once()
	suite.mockService.On("IdentifyEntity", mock.Anything, otpToken).
		Return(nil, entity.ErrEntityNotFound).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Nil(result.EntityReference)
	suite.NotNil(result.EntityReferenceToken)
	suite.NotNil(result.AttributeToken)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_OTP_IncorrectOTP() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := newDefaultAuthnProvider(suite.mockService, nil, mockOTP, nil, nil, nil)

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
	provider := newDefaultAuthnProvider(suite.mockService, nil, mockOTP, nil, nil, nil)

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
	provider := newDefaultAuthnProvider(suite.mockService, nil, mockOTP, nil, nil, nil)

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
	provider := newDefaultAuthnProvider(suite.mockService, nil, mockOTP, nil, nil, nil)

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
	provider := newDefaultAuthnProvider(suite.mockService, nil, mockOTP, nil, nil, nil)

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": "tok",
			"otp":          "123456",
		},
	}

	mockOTP.On("Authenticate", mock.Anything, "tok", "123456").
		Return(nil, &serviceerror.ServiceError{
			Type:             serviceerror.ClientErrorType,
			Code:             "OTHER-CLIENT-ERROR",
			Error:            i18ncore.I18nMessage{DefaultValue: "Some client error"},
			ErrorDescription: i18ncore.I18nMessage{DefaultValue: "Some client error description"},
		}).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_OTP_ServerError() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := newDefaultAuthnProvider(suite.mockService, nil, mockOTP, nil, nil, nil)

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": "tok",
			"otp":          "123456",
		},
	}

	mockOTP.On("Authenticate", mock.Anything, "tok", "123456").
		Return(nil, &serviceerror.ServiceError{
			Type:             serviceerror.ServerErrorType,
			Code:             "INTERNAL",
			Error:            i18ncore.I18nMessage{DefaultValue: "Internal error"},
			ErrorDescription: i18ncore.I18nMessage{DefaultValue: "Something went wrong"},
		}).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

// --- Magic Link authentication tests ---

//nolint:dupl // intentionally mirrors OTP tests to cover the magic link credential path
func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_MagicLink_EntityFound() {
	mockML := magiclinkmock.NewMagicLinkAuthnServiceInterfaceMock(suite.T())
	provider := newDefaultAuthnProvider(suite.mockService, nil, nil, mockML, nil, nil)

	credentials := map[string]interface{}{
		"magiclink": map[string]interface{}{
			"token":            "valid-jwt-token",
			"subjectAttribute": "",
		},
	}

	mlToken := map[string]interface{}{"email": "test@example.com"}

	entityObj := &entity.Entity{
		ID:         "u1",
		Category:   entity.EntityCategoryUser,
		Type:       "customer",
		OUID:       "ou1",
		Attributes: json.RawMessage(`{}`),
	}

	mockML.On("Authenticate", mock.Anything, "valid-jwt-token", "").
		Return(&authncommon.AuthnResult{
			Token:               mlToken,
			AuthenticatedClaims: map[string]interface{}{"email": "test@example.com"},
		}, nil).Once()
	suite.mockService.On("IdentifyEntity", mock.Anything, mlToken).
		Return(strPtr("u1"), nil).Once()
	suite.mockService.On("GetEntity", mock.Anything, "u1").Return(entityObj, nil).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(err)
	suite.NotNil(result)
	suite.NotNil(result.EntityReference)
	suite.Equal("u1", result.EntityReference.EntityID)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_MagicLink_EntityNotFound() { //nolint:dupl
	mockML := magiclinkmock.NewMagicLinkAuthnServiceInterfaceMock(suite.T())
	provider := newDefaultAuthnProvider(suite.mockService, nil, nil, mockML, nil, nil)

	credentials := map[string]interface{}{
		"magiclink": map[string]interface{}{
			"token":            "valid-jwt-token",
			"subjectAttribute": "email",
		},
	}

	mlToken := map[string]interface{}{"email": "test@example.com"}

	mockML.On("Authenticate", mock.Anything, "valid-jwt-token", "email").
		Return(&authncommon.AuthnResult{
			Token:               mlToken,
			AuthenticatedClaims: map[string]interface{}{"email": "test@example.com"},
		}, nil).Once()
	suite.mockService.On("IdentifyEntity", mock.Anything, mlToken).
		Return(nil, entity.ErrEntityNotFound).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Nil(result.EntityReference)
	suite.NotNil(result.EntityReferenceToken)
	suite.NotNil(result.AttributeToken)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_MagicLink_AuthenticationFailed() {
	mockML := magiclinkmock.NewMagicLinkAuthnServiceInterfaceMock(suite.T())
	provider := newDefaultAuthnProvider(suite.mockService, nil, nil, mockML, nil, nil)

	credentials := map[string]interface{}{
		"magiclink": map[string]interface{}{
			"token": "expired-token",
		},
	}

	mockML.On("Authenticate", mock.Anything, "expired-token", "").
		Return(nil, &serviceerror.ServiceError{
			Type:             serviceerror.ClientErrorType,
			Code:             "AUTHN-ML-1002",
			Error:            i18ncore.I18nMessage{DefaultValue: "Expired token"},
			ErrorDescription: i18ncore.I18nMessage{DefaultValue: "The magic link token has expired"},
		}).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeAuthenticationFailed, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_MagicLink_ServerError() {
	mockML := magiclinkmock.NewMagicLinkAuthnServiceInterfaceMock(suite.T())
	provider := newDefaultAuthnProvider(suite.mockService, nil, nil, mockML, nil, nil)

	credentials := map[string]interface{}{
		"magiclink": map[string]interface{}{
			"token": "valid-token",
		},
	}

	mockML.On("Authenticate", mock.Anything, "valid-token", "").
		Return(nil, &serviceerror.ServiceError{
			Type:             serviceerror.ServerErrorType,
			Code:             "INTERNAL",
			Error:            i18ncore.I18nMessage{DefaultValue: "Internal error"},
			ErrorDescription: i18ncore.I18nMessage{DefaultValue: "Something went wrong"},
		}).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_MagicLink_InvalidPayload() {
	mockML := magiclinkmock.NewMagicLinkAuthnServiceInterfaceMock(suite.T())
	provider := newDefaultAuthnProvider(suite.mockService, nil, nil, mockML, nil, nil)

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
	provider := newDefaultAuthnProvider(suite.mockService, nil, nil, mockML, nil, nil)

	credentials := map[string]interface{}{
		"magiclink": map[string]interface{}{},
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

// --- Passkey authentication tests ---

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Passkey_InvalidPayload() {
	provider := newDefaultAuthnProvider(suite.mockService, nil, nil, nil, nil, nil)

	credentials := map[string]interface{}{
		"passkey": "not-a-passkey-struct",
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Passkey_NilPayload() {
	provider := newDefaultAuthnProvider(suite.mockService, nil, nil, nil, nil, nil)

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
	provider := newDefaultAuthnProvider(suite.mockService, nil, nil, nil, nil, nil)

	credentials := map[string]interface{}{
		"federated": "not-a-struct",
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Federated_NilPayload() {
	provider := newDefaultAuthnProvider(suite.mockService, nil, nil, nil, nil, nil)

	credentials := map[string]interface{}{
		"federated": (*authncommon.FederatedAuthCredential)(nil),
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Federated_MissingIDPID() {
	provider := newDefaultAuthnProvider(suite.mockService, nil, nil, nil, nil, nil)

	credentials := map[string]interface{}{
		"federated": &authncommon.FederatedAuthCredential{
			Code:    "auth-code",
			IDPType: idp.IDPType("google"),
		},
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Federated_MissingCode() {
	provider := newDefaultAuthnProvider(suite.mockService, nil, nil, nil, nil, nil)

	credentials := map[string]interface{}{
		"federated": &authncommon.FederatedAuthCredential{
			IDPID:   "idp-1",
			IDPType: idp.IDPType("google"),
		},
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Federated_UnsupportedIDPType() {
	provider := newDefaultAuthnProvider(suite.mockService, nil, nil, nil, nil,
		map[idp.IDPType]authncommon.FederatedAuthenticator{})

	credentials := map[string]interface{}{
		"federated": &authncommon.FederatedAuthCredential{
			IDPID:   "idp-1",
			IDPType: idp.IDPType("unsupported"),
			Code:    "auth-code",
		},
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

// --- Passkey success/error tests using inline mock ---

type mockPasskeyService struct {
	result *authncommon.AuthnResult
	err    *serviceerror.ServiceError
}

func (m *mockPasskeyService) StartAuthentication(_ context.Context,
	_ *passkey.PasskeyAuthenticationStartRequest,
) (*passkey.PasskeyAuthenticationStartData, *serviceerror.ServiceError) {
	return nil, nil
}

func (m *mockPasskeyService) FinishAuthentication(_ context.Context,
	_ *passkey.PasskeyAuthenticationFinishRequest) (*authncommon.AuthnResult, *serviceerror.ServiceError) {
	return m.result, m.err
}

func (m *mockPasskeyService) StartRegistration(_ context.Context,
	_ *passkey.PasskeyRegistrationStartRequest) (*passkey.PasskeyRegistrationStartData, *serviceerror.ServiceError) {
	return nil, nil
}

func (m *mockPasskeyService) FinishRegistration(_ context.Context,
	_ *passkey.PasskeyRegistrationFinishRequest) (*passkey.PasskeyRegistrationFinishData, *serviceerror.ServiceError) {
	return nil, nil
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Passkey_Success() {
	passkeyToken := map[string]interface{}{"userID": "pk-user-1"}
	mockPK := &mockPasskeyService{
		result: &authncommon.AuthnResult{
			Token:               passkeyToken,
			AuthenticatedClaims: map[string]interface{}{"userID": "pk-user-1"},
		},
	}
	provider := newDefaultAuthnProvider(suite.mockService, mockPK, nil, nil, nil, nil)

	credentials := map[string]interface{}{
		"passkey": &passkey.PasskeyAuthenticationFinishRequest{
			CredentialID: "cred-1",
		},
	}

	entityObj := &entity.Entity{
		ID:         "pk-user-1",
		Category:   entity.EntityCategoryUser,
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
	mockPK := &mockPasskeyService{
		err: &serviceerror.ServiceError{
			Type:             serviceerror.ClientErrorType,
			Code:             "PASSKEY-001",
			Error:            i18ncore.I18nMessage{DefaultValue: "Passkey auth failed"},
			ErrorDescription: i18ncore.I18nMessage{DefaultValue: "Invalid passkey credential"},
		},
	}
	provider := newDefaultAuthnProvider(suite.mockService, mockPK, nil, nil, nil, nil)

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

// --- Federated success/error tests using inline mock ---

type mockFederatedAuth struct {
	result *authncommon.AuthnResult
	err    *serviceerror.ServiceError
}

func (m *mockFederatedAuth) Authenticate(_ context.Context, _, _ string) (
	*authncommon.AuthnResult, *serviceerror.ServiceError,
) {
	return m.result, m.err
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Federated_Success() {
	fedToken := map[string]interface{}{"sub": "fed-sub-1"}
	mockFed := &mockFederatedAuth{
		result: &authncommon.AuthnResult{
			Token:               fedToken,
			AuthenticatedClaims: map[string]interface{}{"sub": "fed-sub-1"},
		},
	}
	federatedAuths := map[idp.IDPType]authncommon.FederatedAuthenticator{
		idp.IDPType("google"): mockFed,
	}
	provider := newDefaultAuthnProvider(suite.mockService, nil, nil, nil, nil, federatedAuths)

	credentials := map[string]interface{}{
		"federated": &authncommon.FederatedAuthCredential{
			IDPID:   "idp-1",
			IDPType: idp.IDPType("google"),
			Code:    "auth-code",
		},
	}

	entityObj := &entity.Entity{
		ID:         "fed-user-1",
		Category:   entity.EntityCategoryUser,
		Type:       "customer",
		OUID:       "ou1",
		Attributes: json.RawMessage(`{}`),
	}

	suite.mockService.On("IdentifyEntity", mock.Anything, fedToken).
		Return(strPtr("fed-user-1"), nil).Once()
	suite.mockService.On("GetEntity", mock.Anything, "fed-user-1").
		Return(entityObj, nil).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(err)
	suite.NotNil(result)
	suite.NotNil(result.EntityReference)
	suite.Equal("fed-user-1", result.EntityReference.EntityID)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Federated_ClientError() {
	mockFed := &mockFederatedAuth{
		err: &serviceerror.ServiceError{
			Type:             serviceerror.ClientErrorType,
			Code:             "FED-001",
			Error:            i18ncore.I18nMessage{DefaultValue: "Fed auth failed"},
			ErrorDescription: i18ncore.I18nMessage{DefaultValue: "Invalid auth code"},
		},
	}
	federatedAuths := map[idp.IDPType]authncommon.FederatedAuthenticator{
		idp.IDPType("google"): mockFed,
	}
	provider := newDefaultAuthnProvider(suite.mockService, nil, nil, nil, nil, federatedAuths)

	credentials := map[string]interface{}{
		"federated": &authncommon.FederatedAuthCredential{
			IDPID:   "idp-1",
			IDPType: idp.IDPType("google"),
			Code:    "auth-code",
		},
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeAuthenticationFailed, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Federated_ServerError() {
	mockFed := &mockFederatedAuth{
		err: &serviceerror.ServiceError{
			Type:             serviceerror.ServerErrorType,
			Code:             "INTERNAL",
			Error:            i18ncore.I18nMessage{DefaultValue: "Internal error"},
			ErrorDescription: i18ncore.I18nMessage{DefaultValue: "Something went wrong"},
		},
	}
	federatedAuths := map[idp.IDPType]authncommon.FederatedAuthenticator{
		idp.IDPType("google"): mockFed,
	}
	provider := newDefaultAuthnProvider(suite.mockService, nil, nil, nil, nil, federatedAuths)

	credentials := map[string]interface{}{
		"federated": &authncommon.FederatedAuthCredential{
			IDPID:   "idp-1",
			IDPType: idp.IDPType("google"),
			Code:    "auth-code",
		},
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

// --- Helper ---

func strPtr(s string) *string {
	return &s
}
