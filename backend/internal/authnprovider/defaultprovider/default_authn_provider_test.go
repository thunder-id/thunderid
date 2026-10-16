// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package defaultprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/identitygovernance"
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
	provider      providers.AuthnProviderInterface
}

func (suite *DefaultAuthnProviderTestSuite) SetupTest() {
	suite.mockService = entitymock.NewEntityServiceInterfaceMock(suite.T())
	suite.mockPasskey = passkeymock.NewPasskeyServiceInterfaceMock(suite.T())
	suite.mockFederated = commonmock.NewFederatedAuthenticatorMock(suite.T())
	suite.provider = Initialize(suite.mockService, nil, nil, nil, nil, nil, nil)
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
		Return(authResult, nil, nil).Once()
	suite.mockService.On("GetEntityProfile", mock.Anything, "user123").
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
		Return(nil, nil, entity.ErrEntityNotFound).Once()

	result, err := suite.provider.Authenticate(context.Background(), identifiers, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeUserNotFound, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_AuthenticationFailed() {
	identifiers := map[string]interface{}{"username": "testuser"}
	credentials := map[string]interface{}{"password": "wrongpassword"}

	suite.mockService.On("AuthenticateEntity", mock.Anything, identifiers, credentials).
		Return(nil, nil, entity.ErrAuthenticationFailed).Once()

	result, err := suite.provider.Authenticate(context.Background(), identifiers, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeAuthenticationFailed, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_AmbiguousEntity() {
	identifiers := map[string]interface{}{"email": "shared@example.com"}
	credentials := map[string]interface{}{"password": "password"}

	suite.mockService.On("AuthenticateEntity", mock.Anything, identifiers, credentials).
		Return(nil, nil, entity.ErrAmbiguousEntity).Once()

	result, err := suite.provider.Authenticate(context.Background(), identifiers, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.ClientErrorType, err.Type)
	suite.Equal(authnprovidercm.ErrorCodeAmbiguousUser, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_GenericAuthError() {
	identifiers := map[string]interface{}{"username": "testuser"}
	credentials := map[string]interface{}{"password": "password123"}

	suite.mockService.On("AuthenticateEntity", mock.Anything, identifiers, credentials).
		Return(nil, nil, errors.New("unexpected error")).Once()

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
		Return(authResult, nil, nil).Once()
	suite.mockService.On("GetEntityProfile", mock.Anything, "user123").
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
		Return(authResult, nil, nil).Once()
	suite.mockService.On("GetEntityProfile", mock.Anything, "user123").
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
		Return(authResult, nil, nil).Once()
	suite.mockService.On("GetEntityProfile", mock.Anything, "user123").
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
		Return(authResult, nil, nil).Once()
	suite.mockService.On("GetEntityProfile", mock.Anything, "resolved-user-123").
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
		Return(nil, nil, entity.ErrEntityNotFound).Once()

	result, err := suite.provider.Authenticate(context.Background(), identifiers, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeUserNotFound, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_ByPreResolvedUserID_AuthFailed() {
	identifiers := map[string]interface{}{"userID": "user-wrong-pw"}
	credentials := map[string]interface{}{"password": "wrongpassword"}

	suite.mockService.On("AuthenticateEntityByID", mock.Anything, "user-wrong-pw", credentials).
		Return(nil, nil, entity.ErrAuthenticationFailed).Once()

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
		Return(authResult, nil, nil).Once()
	suite.mockService.On("GetEntityProfile", mock.Anything, "user123").
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

	suite.mockService.On("GetEntityProfile", mock.Anything, "provisioned-user-123").
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

	suite.mockService.On("GetEntityProfile", mock.Anything, "provisioned-user-123").
		Return(nil, errors.New("db error")).Once()

	result, err := suite.provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_IdentifyEntity_ServerError() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := Initialize(suite.mockService, nil, mockOTP, nil, nil, nil, nil)

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
		}, nil, nil).Once()
	suite.mockService.On("IdentifyEntity", mock.Anything, otpToken).
		Return(nil, errors.New("db error")).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_IdentifyEntity_Success_ThenGetEntity() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := Initialize(suite.mockService, nil, mockOTP, nil, nil, nil, nil)

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
		}, nil, nil).Once()
	suite.mockService.On("IdentifyEntity", mock.Anything, otpToken).
		Return(new("resolved-id"), nil).Once()
	suite.mockService.On("GetEntityProfile", mock.Anything, "resolved-id").
		Return(entityObj, nil).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(err)
	suite.NotNil(result.EntityReference)
	suite.Equal("resolved-id", result.EntityReference.EntityID)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_IdentifyEntity_GetEntityFails() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := Initialize(suite.mockService, nil, mockOTP, nil, nil, nil, nil)

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
		}, nil, nil).Once()
	suite.mockService.On("IdentifyEntity", mock.Anything, otpToken).
		Return(new("resolved-id"), nil).Once()
	suite.mockService.On("GetEntityProfile", mock.Anything, "resolved-id").
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
		Return(authResult, nil, nil).Once()
	suite.mockService.On("GetEntityProfile", mock.Anything, "user123").
		Return(entityObj, nil).Once()

	result, err := suite.provider.Authenticate(context.Background(), identifiers, credentials, nil)

	suite.Nil(err)
	suite.NotNil(result)
	suite.NotNil(result.AuthenticatedClaims)
	suite.Equal("user123", result.AuthenticatedClaims[authnprovidercm.UserAttributeUserID])
}

// --- SearchEntityReferences tests ---

func (suite *DefaultAuthnProviderTestSuite) TestSearchEntityReferences_ListsEveryMatch() {
	filters := map[string]interface{}{"costCenter": "CC-1"}
	suite.mockService.On("SearchEntities", mock.Anything, filters).Return([]providers.Entity{
		{ID: "user-1", Category: providers.EntityCategoryUser, Type: "customer", OUID: "ou1"},
		{ID: "user-2", Category: providers.EntityCategoryUser, Type: "customer", OUID: "ou2"},
	}, nil).Once()

	refs, err := suite.provider.SearchEntityReferences(context.Background(), filters)

	suite.Nil(err)
	suite.Equal([]providers.EntityReference{
		{EntityID: "user-1", EntityCategory: "user", EntityType: "customer", OUID: "ou1"},
		{EntityID: "user-2", EntityCategory: "user", EntityType: "customer", OUID: "ou2"},
	}, refs)
}

func (suite *DefaultAuthnProviderTestSuite) TestSearchEntityReferences_NoMatchIsEmpty() {
	filters := map[string]interface{}{"costCenter": "CC-1"}
	suite.mockService.On("SearchEntities", mock.Anything, filters).Return(nil, entity.ErrEntityNotFound).Once()

	refs, err := suite.provider.SearchEntityReferences(context.Background(), filters)

	suite.Nil(err)
	suite.Empty(refs)
}

func (suite *DefaultAuthnProviderTestSuite) TestSearchEntityReferences_StoreFailureIsServerError() {
	filters := map[string]interface{}{"costCenter": "CC-1"}
	suite.mockService.On("SearchEntities", mock.Anything, filters).Return(nil, errors.New("db down")).Once()

	refs, err := suite.provider.SearchEntityReferences(context.Background(), filters)

	suite.Nil(refs)
	if suite.NotNil(err) {
		suite.Equal(tidcommon.InternalServerError.Code, err.Code)
	}
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

	suite.mockService.On("GetEntityProfile", mock.Anything, "user123").
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

	suite.mockService.On("GetEntityProfile", mock.Anything, "user1").
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

	suite.mockService.On("GetEntityProfile", mock.Anything, "user123").
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

	suite.mockService.On("GetEntityProfile", mock.Anything, "user123").
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

	suite.mockService.On("GetEntityProfile", mock.Anything, "user123").
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

	suite.mockService.On("GetEntityProfile", mock.Anything, "user123").
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

	suite.mockService.On("GetEntityProfile", mock.Anything, "user123").
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
	provider := Initialize(suite.mockService, nil, mockOTP, nil, nil, nil, nil)

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": "tok",
			"otp":          "wrong",
		},
	}

	mockOTP.On("Authenticate", mock.Anything, "tok", "wrong").
		Return(nil, nil, &otp.ErrorIncorrectOTP).Once()

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeAuthenticationFailed, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_OTP_InvalidPayload() {
	mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	provider := Initialize(suite.mockService, nil, mockOTP, nil, nil, nil, nil)

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
	provider := Initialize(suite.mockService, nil, mockOTP, nil, nil, nil, nil)

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
	provider := Initialize(suite.mockService, nil, mockOTP, nil, nil, nil, nil)

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
	provider := Initialize(suite.mockService, nil, mockOTP, nil, nil, nil, nil)

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": "tok",
			"otp":          "123456",
		},
	}

	mockOTP.On("Authenticate", mock.Anything, "tok", "123456").
		Return(nil, nil, &tidcommon.ServiceError{
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
	provider := Initialize(suite.mockService, nil, mockOTP, nil, nil, nil, nil)

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": "tok",
			"otp":          "123456",
		},
	}

	mockOTP.On("Authenticate", mock.Anything, "tok", "123456").
		Return(nil, nil, &tidcommon.ServiceError{
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
	provider := Initialize(suite.mockService, nil, nil, mockML, nil, nil, nil)

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
	provider := Initialize(suite.mockService, nil, nil, mockML, nil, nil, nil)

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
	provider := Initialize(suite.mockService, nil, nil, mockML, nil, nil, nil)

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
	provider := Initialize(suite.mockService, nil, nil, mockML, nil, nil, nil)

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
	setupOTP := func() (providers.AuthnProviderInterface, map[string]interface{}, map[string]interface{}) {
		mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
		token := map[string]interface{}{"mobile_number": "+1234567890"}
		mockOTP.On("Authenticate", mock.Anything, "tok", "123456").
			Return(&authncommon.AuthnResult{
				Token:               token,
				AuthenticatedClaims: map[string]interface{}{"mobile_number": "+1234567890"},
			}, nil, nil).Once()
		creds := map[string]interface{}{
			"otp": map[string]interface{}{
				"sessionToken": "tok",
				"otp":          "123456",
			},
		}
		return Initialize(suite.mockService, nil, mockOTP, nil, nil, nil, nil), creds, token
	}

	setupMagicLink := func() (providers.AuthnProviderInterface, map[string]interface{}, map[string]interface{}) {
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
		return Initialize(suite.mockService, nil, nil, mockML, nil, nil, nil), creds, token
	}

	tests := []struct {
		name  string
		setup func() (providers.AuthnProviderInterface, map[string]interface{}, map[string]interface{})
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
			suite.mockService.On("GetEntityProfile", mock.Anything, "u1").Return(entityObj, nil).Once()

			result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

			suite.Nil(err)
			suite.NotNil(result)
			suite.NotNil(result.EntityReference)
			suite.Equal("u1", result.EntityReference.EntityID)
		})
	}
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_TokenizedAuth_IdentifyEntityErrorReturnsTokens() {
	setupOTP := func() (providers.AuthnProviderInterface, map[string]interface{}, map[string]interface{}) {
		mockOTP := otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
		token := map[string]interface{}{"mobile_number": "+1234567890"}
		mockOTP.On("Authenticate", mock.Anything, "tok", "123456").
			Return(&authncommon.AuthnResult{
				Token:               token,
				AuthenticatedClaims: map[string]interface{}{"mobile_number": "+1234567890"},
			}, nil, nil).Once()
		creds := map[string]interface{}{
			"otp": map[string]interface{}{
				"sessionToken": "tok",
				"otp":          "123456",
			},
		}
		return Initialize(suite.mockService, nil, mockOTP, nil, nil, nil, nil), creds, token
	}

	setupMagicLink := func() (providers.AuthnProviderInterface, map[string]interface{}, map[string]interface{}) {
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
		return Initialize(suite.mockService, nil, nil, mockML, nil, nil, nil), creds, token
	}

	tests := []struct {
		name        string
		setup       func() (providers.AuthnProviderInterface, map[string]interface{}, map[string]interface{})
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
	provider := Initialize(suite.mockService, nil, nil, nil, nil, nil, nil)

	credentials := map[string]interface{}{
		"passkey": "not-a-passkey-struct",
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Passkey_NilPayload() {
	provider := Initialize(suite.mockService, nil, nil, nil, nil, nil, nil)

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
	provider := Initialize(suite.mockService, nil, nil, nil, nil, nil, nil)

	credentials := map[string]interface{}{
		"federated": "not-a-struct",
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Federated_NilPayload() {
	provider := Initialize(suite.mockService, nil, nil, nil, nil, nil, nil)

	credentials := map[string]interface{}{
		"federated": (*authncommon.FederatedAuthCredential)(nil),
	}

	result, err := provider.Authenticate(context.Background(), nil, credentials, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestAuthenticate_Federated_MissingIDPID() {
	provider := Initialize(suite.mockService, nil, nil, nil, nil, nil, nil)

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
	provider := Initialize(suite.mockService, nil, nil, nil, nil, nil, nil)

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
		map[providers.IDPType]authncommon.FederatedAuthenticator{}, nil)

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
	provider := Initialize(suite.mockService, suite.mockPasskey, nil, nil, nil, nil, nil)

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

	suite.mockService.On("GetEntityProfile", mock.Anything, "pk-user-1").
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
	provider := Initialize(suite.mockService, suite.mockPasskey, nil, nil, nil, nil, nil)

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
	provider := Initialize(suite.mockService, nil, nil, nil, nil, federatedAuths, nil)

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
	suite.mockService.On("GetEntityProfile", mock.Anything, "fed-user-1").
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
	provider := Initialize(suite.mockService, nil, nil, nil, nil, federatedAuths, nil)

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
	provider := Initialize(suite.mockService, nil, nil, nil, nil, federatedAuths, nil)

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
	provider := Initialize(suite.mockService, suite.mockPasskey, nil, nil, nil, nil, nil)
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
	provider := Initialize(suite.mockService, suite.mockPasskey, nil, nil, nil, nil, nil)

	result, err := provider.InitiateAuthentication(context.Background(), passkey.CredentialType, "bad", nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestInitiateEnrollment_Passkey() {
	provider := Initialize(suite.mockService, suite.mockPasskey, nil, nil, nil, nil, nil)
	req := &passkey.PasskeyRegistrationStartRequest{UserID: "user123", RelyingPartyID: "example.com"}
	startData := &passkey.PasskeyRegistrationStartData{SessionToken: "sess-1"}
	suite.mockPasskey.On("StartRegistration", mock.Anything, req).Return(startData, nil).Once()

	result, err := provider.InitiateEnrollment(context.Background(), passkey.CredentialType, req, nil)

	suite.Nil(err)
	suite.Equal(startData, result)
}

func (suite *DefaultAuthnProviderTestSuite) TestInitiateEnrollment_InvalidPayload() {
	provider := Initialize(suite.mockService, suite.mockPasskey, nil, nil, nil, nil, nil)

	result, err := provider.InitiateEnrollment(context.Background(), passkey.CredentialType, 42, nil)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, err.Code)
}

// A stated suspension keeps its own code through both enrolment steps; any other client error is
// still an enrolment failure.
func (suite *DefaultAuthnProviderTestSuite) TestPasskeyEnrollmentKeepsAStatedSuspension() {
	provider := Initialize(suite.mockService, suite.mockPasskey, nil, nil, nil, nil, nil)
	startReq := &passkey.PasskeyRegistrationStartRequest{UserID: "user123", RelyingPartyID: "example.com"}
	finishReq := &passkey.PasskeyRegistrationFinishRequest{CredentialID: "cred-1"}

	for passkeyErr, wantCode := range map[*tidcommon.ServiceError]string{
		&passkey.ErrorAccountSuspended:    authnprovidercm.ErrorCodeAccountSuspended,
		&passkey.ErrorInvalidSessionToken: authnprovidercm.ErrorCodeEnrollmentFailed,
	} {
		suite.mockPasskey.On("StartRegistration", mock.Anything, startReq).Return(nil, passkeyErr).Once()
		suite.mockPasskey.On("FinishRegistration", mock.Anything, finishReq).Return(nil, passkeyErr).Once()

		_, err := provider.InitiateEnrollment(context.Background(), passkey.CredentialType, startReq, nil)
		suite.Require().NotNil(err)
		suite.Equal(wantCode, err.Code)

		_, err = provider.Enroll(context.Background(), nil,
			map[string]interface{}{"passkey": finishReq}, nil)
		suite.Require().NotNil(err)
		suite.Equal(wantCode, err.Code)
	}
}

func (suite *DefaultAuthnProviderTestSuite) TestEnroll_Passkey_Success() {
	provider := Initialize(suite.mockService, suite.mockPasskey, nil, nil, nil, nil, nil)
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
	suite.mockService.On("GetEntityProfile", mock.Anything, "user123").Return(entityObj, nil).Once()

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
	provider := Initialize(suite.mockService, suite.mockPasskey, nil, nil, nil, nil, nil)
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

// TestDispatchKeysAreInternalCredentialTypes keeps the credentials authentication API's reject list
// in step with what this provider dispatches on. A key dispatched here but missing from
// InternalCredentialTypes would be selectable by any API client. Compare dispatch cases by constant name.
func (suite *DefaultAuthnProviderTestSuite) TestDispatchKeysAreInternalCredentialTypes() {
	dispatched := suite.dispatchedCredentialConstants("default_authn_provider.go",
		"authenticateWithCredential")
	reserved := suite.listedCredentialConstants("../common/constants.go", "InternalCredentialTypes")

	for _, name := range dispatched {
		suite.Contains(reserved, name,
			"authnprovidercm.%s is dispatched by the default provider but is not listed in "+
				"InternalCredentialTypes, so the credentials authentication API would accept it", name)
	}
}

// dispatchedCredentialConstants returns credential constants handled by the dispatch switch.
func (suite *DefaultAuthnProviderTestSuite) dispatchedCredentialConstants(
	filename, funcName string) []string {
	var fn *ast.FuncDecl
	for _, decl := range suite.parseFile(filename).Decls {
		if decl, ok := decl.(*ast.FuncDecl); ok && decl.Name.Name == funcName {
			fn = decl
			break
		}
	}
	suite.Require().NotNil(fn, "function %s not found in %s", funcName, filename)

	var names []string
	ast.Inspect(fn, func(node ast.Node) bool {
		branch, ok := node.(*ast.CaseClause)
		if !ok {
			return true
		}
		for _, expression := range branch.List {
			if key, ok := expression.(*ast.SelectorExpr); ok {
				if pkg, ok := key.X.(*ast.Ident); ok && pkg.Name == "authnprovidercm" {
					names = append(names, key.Sel.Name)
				}
			}
		}
		return true
	})
	suite.Require().NotEmpty(names, "no credential dispatch keys found in %s", funcName)
	return names
}

// listedCredentialConstants returns the constant names listed in the named []string variable.
func (suite *DefaultAuthnProviderTestSuite) listedCredentialConstants(
	filename, varName string) []string {
	var names []string
	for _, decl := range suite.parseFile(filename).Decls {
		decl, ok := decl.(*ast.GenDecl)
		if !ok || decl.Tok != token.VAR {
			continue
		}
		for _, spec := range decl.Specs {
			spec, ok := spec.(*ast.ValueSpec)
			if !ok || len(spec.Names) != 1 || spec.Names[0].Name != varName ||
				len(spec.Values) != 1 {
				continue
			}
			literal, ok := spec.Values[0].(*ast.CompositeLit)
			suite.Require().True(ok, "%s is not a composite literal", varName)
			for _, element := range literal.Elts {
				if element, ok := element.(*ast.Ident); ok {
					names = append(names, element.Name)
				}
			}
		}
	}
	suite.Require().NotEmpty(names, "variable %s not found in %s", varName, filename)
	return names
}

// parseFile parses a source file relative to this package's directory.
func (suite *DefaultAuthnProviderTestSuite) parseFile(filename string) *ast.File {
	file, err := parser.ParseFile(token.NewFileSet(), filename, nil, 0)
	suite.Require().NoError(err, "failed to parse %s", filename)
	return file
}

// --- StoreAccountLink ---

func (suite *DefaultAuthnProviderTestSuite) TestStoreAccountLink_ResolvesTokenAndDelegates() {
	suite.mockService.On("IdentifyEntity", mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	entityResult := &providers.Entity{ID: "user123", Category: providers.EntityCategoryUser, Type: "customer"}
	suite.mockService.On("GetEntityProfile", mock.Anything, "user123").Return(entityResult, nil)
	suite.mockService.On("LinkAccount", mock.Anything, "user123", "idp-a", "sub-1").Return(nil)

	token := map[string]interface{}{"userID": "user123"}
	suite.Nil(suite.provider.StoreAccountLink(context.Background(), token, "idp-a", "sub-1"))
}

func (suite *DefaultAuthnProviderTestSuite) TestStoreAccountLink_UnresolvableTokenIsClientError() {
	suite.mockService.On("IdentifyEntity", mock.Anything, mock.Anything).Return(nil, entity.ErrEntityNotFound)

	token := map[string]interface{}{"email": "nobody@example.com"}
	svcErr := suite.provider.StoreAccountLink(context.Background(), token, "idp-a", "sub-1")
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.ClientErrorType, svcErr.Type)
	suite.Equal(authnprovidercm.ErrorCodeUserNotFound, svcErr.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestStoreAccountLink_RejectsNonMapToken() {
	svcErr := suite.provider.StoreAccountLink(context.Background(), "not-a-map", "idp-a", "sub-1")
	suite.Require().NotNil(svcErr)
	suite.Equal(authnprovidercm.ErrorCodeInvalidToken, svcErr.Code)
}

func (suite *DefaultAuthnProviderTestSuite) TestStoreAccountLink_RejectsEmptyIdentity() {
	token := map[string]interface{}{"userID": "user123"}
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest,
		suite.provider.StoreAccountLink(context.Background(), token, "", "sub-1").Code)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest,
		suite.provider.StoreAccountLink(context.Background(), token, "idp-a", "").Code)
}

// Reaching the link limit is about the user's own data, so it is a client error.
func (suite *DefaultAuthnProviderTestSuite) TestStoreAccountLink_LinkLimitIsClientError() {
	entityResult := &providers.Entity{ID: "user123", Category: providers.EntityCategoryUser, Type: "customer"}
	suite.mockService.On("GetEntityProfile", mock.Anything, "user123").Return(entityResult, nil)
	suite.mockService.On("LinkAccount", mock.Anything, "user123", "idp-a", "sub-1").
		Return(fmt.Errorf("%w: too many", entity.ErrIndexedValueLimitExceeded))

	token := map[string]interface{}{"userID": "user123"}
	svcErr := suite.provider.StoreAccountLink(context.Background(), token, "idp-a", "sub-1")
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.ClientErrorType, svcErr.Type)
	suite.Equal(authnprovidercm.ErrorCodeInvalidRequest, svcErr.Code)
}

// A storage failure is the provider's own problem, not the caller's.
func (suite *DefaultAuthnProviderTestSuite) TestStoreAccountLink_StoreFailureIsServerError() {
	entityResult := &providers.Entity{ID: "user123", Category: providers.EntityCategoryUser, Type: "customer"}
	suite.mockService.On("GetEntityProfile", mock.Anything, "user123").Return(entityResult, nil)
	suite.mockService.On("LinkAccount", mock.Anything, "user123", "idp-a", "sub-1").
		Return(errors.New("boom"))

	token := map[string]interface{}{"userID": "user123"}
	svcErr := suite.provider.StoreAccountLink(context.Background(), token, "idp-a", "sub-1")
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.ServerErrorType, svcErr.Type)
}

// --- federated token resolution ---

func federatedToken(extra map[string]interface{}) map[string]interface{} {
	token := map[string]interface{}{
		authnprovidercm.UserAttributeFederatedIdpID: "idp-a",
		authnprovidercm.UserAttributeSub:            "sub-1",
	}
	for k, v := range extra {
		token[k] = v
	}
	return token
}

func (suite *DefaultAuthnProviderTestSuite) federatedAuthResult(
	token map[string]interface{},
) (*providers.AuthnResult, *tidcommon.ServiceError) {
	suite.mockFederated.On("Authenticate", mock.Anything, "idp-a", mock.Anything).
		Return(&authncommon.AuthnResult{Token: token}, nil)
	provider := Initialize(suite.mockService, nil, nil, nil, nil,
		map[providers.IDPType]authncommon.FederatedAuthenticator{providers.IDPTypeOAuth: suite.mockFederated}, nil)

	credentials := map[string]interface{}{
		authnprovidercm.CredentialTypeFederated: &authncommon.FederatedAuthCredential{
			IDPID:             "idp-a",
			IDPType:           providers.IDPTypeOAuth,
			AuthorizationData: authncommon.AuthorizationData{Code: "code"},
		},
	}
	return provider.Authenticate(context.Background(), nil, credentials, nil)
}

func (suite *DefaultAuthnProviderTestSuite) TestFederatedToken_ResolvesThroughRecordedLink() {
	suite.mockService.On("ResolveLinkedAccounts", mock.Anything, "idp-a", "sub-1").Return([]string{"user123"}, nil)
	suite.mockService.On("GetEntityProfile", mock.Anything, "user123").
		Return(&providers.Entity{ID: "user123", Category: providers.EntityCategoryUser, Type: "customer"}, nil)

	result, svcErr := suite.federatedAuthResult(federatedToken(nil))
	suite.Require().Nil(svcErr)
	suite.Require().NotNil(result.EntityReference)
	suite.Equal("user123", result.EntityReference.EntityID)
	suite.mockService.AssertNotCalled(suite.T(), "IdentifyEntity", mock.Anything, mock.Anything)
}

// A recorded link is the only thing that authenticates a federated sign-in here. With none, the
// token comes back untouched for the flow to match on attributes, verify or provision against, even
// when the token carries account-linking attributes that would match an existing user.
func (suite *DefaultAuthnProviderTestSuite) TestFederatedToken_NoLinkPassesTokenThrough() {
	suite.mockService.On("ResolveLinkedAccounts", mock.Anything, "idp-a", "sub-1").Return([]string{}, nil)

	token := federatedToken(map[string]interface{}{"email": "user@example.com"})
	result, svcErr := suite.federatedAuthResult(token)
	suite.Require().Nil(svcErr)
	suite.Nil(result.EntityReference)
	suite.Equal(token, result.EntityReferenceToken)
	suite.Equal(token, result.AttributeToken)
	suite.mockService.AssertNotCalled(suite.T(), "IdentifyEntity", mock.Anything, mock.Anything)
	suite.mockService.AssertNotCalled(suite.T(), "LinkAccount",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// Several users holding the link is for the flow to settle by letting the user pick, so the token is
// handed back as it is for an identity with no link.
func (suite *DefaultAuthnProviderTestSuite) TestFederatedToken_SeveralLinksPassTokenThrough() {
	suite.mockService.On("ResolveLinkedAccounts", mock.Anything, "idp-a", "sub-1").
		Return([]string{"user-1", "user-2"}, nil)

	token := federatedToken(nil)
	result, svcErr := suite.federatedAuthResult(token)
	suite.Require().Nil(svcErr)
	suite.Nil(result.EntityReference)
	suite.Equal(token, result.EntityReferenceToken)
	suite.mockService.AssertNotCalled(suite.T(), "GetEntity", mock.Anything, mock.Anything)
}

func (suite *DefaultAuthnProviderTestSuite) TestFederatedToken_LinkLookupFailureIsServerError() {
	suite.mockService.On("ResolveLinkedAccounts", mock.Anything, "idp-a", "sub-1").
		Return(nil, errors.New("db down"))

	_, svcErr := suite.federatedAuthResult(federatedToken(nil))
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.ServerErrorType, svcErr.Type)
}

// GetEntityReference and GetAttributes both resolve whatever token the AuthUser carries. A federated
// token resolves only through its recorded links, never through IdentifyEntity, which would scan
// for keys no index can answer.
func (suite *DefaultAuthnProviderTestSuite) TestGetEntityReference_FederatedTokenResolvesThroughLinks() {
	for name, tc := range map[string]struct {
		linkedIDs []string
		wantCode  string
		wantID    string
	}{
		"no link":       {linkedIDs: []string{}, wantCode: authnprovidercm.ErrorCodeUserNotFound},
		"one link":      {linkedIDs: []string{"user-1"}, wantID: "user-1"},
		"several links": {linkedIDs: []string{"user-1", "user-2"}, wantCode: authnprovidercm.ErrorCodeAmbiguousUser},
	} {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.mockService.On("ResolveLinkedAccounts", mock.Anything, "idp-a", "sub-1").
				Return(tc.linkedIDs, nil).Once()
			if tc.wantID != "" {
				suite.mockService.On("GetEntityProfile", mock.Anything, tc.wantID).
					Return(&providers.Entity{ID: tc.wantID, Category: providers.EntityCategoryUser}, nil).Once()
			}

			ref, svcErr := suite.provider.GetEntityReference(context.Background(), federatedToken(nil))

			if tc.wantCode != "" {
				suite.Require().NotNil(svcErr)
				suite.Equal(tc.wantCode, svcErr.Code)
			} else {
				suite.Require().Nil(svcErr)
				suite.Equal(tc.wantID, ref.EntityID)
			}
			suite.mockService.AssertNotCalled(suite.T(), "IdentifyEntity", mock.Anything, mock.Anything)
		})
	}
}

func (suite *DefaultAuthnProviderTestSuite) TestGetAttributes_FederatedTokenReportsNotFound() {
	suite.mockService.On("ResolveLinkedAccounts", mock.Anything, "idp-a", "sub-1").Return([]string{}, nil)

	_, svcErr := suite.provider.GetAttributes(context.Background(), federatedToken(nil), nil, nil)
	suite.Require().NotNil(svcErr)
	suite.Equal(authnprovidercm.ErrorCodeUserNotFound, svcErr.Code)
	suite.mockService.AssertNotCalled(suite.T(), "IdentifyEntity", mock.Anything, mock.Anything)
}

// An unlinked federated identity resolves to nobody here: the token is handed back on both sides
// and the authenticated claims ride along, so the flow can match, verify or provision on it. The
// provider never matches on attributes, so IdentifyEntity is not consulted for a federated token.
func (suite *DefaultAuthnProviderTestSuite) TestBuildAuthnResult_UnlinkedFederatedTokenPassesThrough() {
	token := map[string]interface{}{
		"federatedIdpId": "idp-github",
		"sub":            "sub-github",
		"email":          "u@example.com",
	}
	claims := map[string]interface{}{"email": "u@example.com"}
	suite.mockService.On("ResolveLinkedAccounts", mock.Anything, "idp-github", "sub-github").
		Return([]string{}, nil)

	result, svcErr := suite.provider.(*defaultAuthnProvider).buildAuthnResult(context.Background(),
		&authncommon.AuthnResult{Token: token, AuthenticatedClaims: claims})

	assert.Nil(suite.T(), svcErr)
	assert.Nil(suite.T(), result.EntityReference)
	assert.Nil(suite.T(), result.Attributes)
	assert.Equal(suite.T(), token, result.EntityReferenceToken)
	assert.Equal(suite.T(), token, result.AttributeToken)
	assert.Equal(suite.T(), providers.AuthenticatedClaims(claims), result.AuthenticatedClaims)
	suite.mockService.AssertNotCalled(suite.T(), "IdentifyEntity", mock.Anything, mock.Anything)
}

// A recorded link is the previous verification, so it authenticates regardless.
func (suite *DefaultAuthnProviderTestSuite) TestBuildAuthnResult_LinkHitStillAuthenticates() {
	token := map[string]interface{}{"federatedIdpId": "idp-github", "sub": "sub-github"}
	suite.mockService.On("ResolveLinkedAccounts", mock.Anything, "idp-github", "sub-github").
		Return([]string{"user-1"}, nil)
	suite.mockService.On("GetEntityProfile", mock.Anything, "user-1").
		Return(&providers.Entity{ID: "user-1", Type: "person", OUID: "ou-1"}, nil)

	result, svcErr := suite.provider.(*defaultAuthnProvider).buildAuthnResult(context.Background(),
		&authncommon.AuthnResult{Token: token})

	assert.Nil(suite.T(), svcErr)
	if assert.NotNil(suite.T(), result.EntityReference) {
		assert.Equal(suite.T(), "user-1", result.EntityReference.EntityID)
	}
}

// Fails when a dispatched credential type declares no scope; it would fall through to the default
// arm and count against the password's counter.
func (suite *DefaultAuthnProviderTestSuite) TestEveryDispatchedCredentialTypeDeclaresAScope() {
	declared := make(map[string]bool, len(accessScopeDispatch))
	for _, entry := range accessScopeDispatch {
		declared[entry.credentialType] = true
	}

	for _, credentialType := range authnprovidercm.InternalCredentialTypes {
		suite.True(declared[credentialType],
			"credential type %q is dispatched internally but declares no access scope: add a row to "+
				"accessScopeDispatch, using scopeNotAnAuthenticationEvent if it checks no secret",
			credentialType)
	}
}

// Machine credentials take the default arm rather than a dispatch row, so the property to pin is the
// derived scope rather than the presence of a row.
func (suite *DefaultAuthnProviderTestSuite) TestEverySystemCredentialTypeDerivesTheMachineScope() {
	for _, credentialType := range authnprovidercm.SystemCredentialTypes {
		scope, isEvent := selectedScope(map[string]interface{}{credentialType: "secret"})
		suite.Require().True(isEvent, "credential type %q must name an authentication event", credentialType)
		suite.Equal(providers.AccessScopeSystemCredential, scope,
			"credential type %q must be throttled rather than counted towards a lock", credentialType)
	}
}

// Every scope the derivation can produce has to be one the store will accept, because the scope
// becomes a path segment in the statements that write lock state.
func (suite *DefaultAuthnProviderTestSuite) TestEveryDerivableScopeIsRegistered() {
	for _, entry := range accessScopeDispatch {
		if entry.scope == scopeNotAnAuthenticationEvent {
			continue
		}
		suite.True(identitygovernance.IsAccessScope(entry.scope),
			"scope %q is derivable but not a registered access scope", entry.scope)
	}
	for _, scope := range []providers.AccessScope{
		providers.AccessScopeCredential, providers.AccessScopeSystemCredential,
	} {
		suite.True(identitygovernance.IsAccessScope(scope), "default-arm scope %q must be registered", scope)
	}
}

// The reserved key must not be derivable, or a credential type named "entity" would merge its
// counter into the identity's.
func (suite *DefaultAuthnProviderTestSuite) TestTheReservedKeyIsNotDerivable() {
	for _, entry := range accessScopeDispatch {
		suite.NotEqual(identitygovernance.AccessScopeEntity, entry.scope,
			"the reserved entity key must never be derived from a credential type")
		suite.NotEqual(identitygovernance.AccessScopeEntity, entry.credentialType,
			"no credential type may be named after the reserved entity key")
	}

	scope, isEvent := selectedScope(map[string]interface{}{string(identitygovernance.AccessScopeEntity): "x"})
	suite.Require().True(isEvent)
	suite.Equal(providers.AccessScopeCredential, scope,
		"a credential key that happens to be named entity takes the default arm, not the reserved key")
}

// Test 12: a map with both an OTP and a password derives the scope the dispatch used, otp first.
// The two carry different thresholds.
func (suite *DefaultAuthnProviderTestSuite) TestDerivationFollowsTheDispatchOrder() {
	tests := []struct {
		name        string
		credentials map[string]interface{}
		scope       providers.AccessScope
		isEvent     bool
	}{
		{
			name:        "otp beats the password because the dispatch reaches it first",
			credentials: map[string]interface{}{"password": "p", authnprovidercm.CredentialTypeOTP: "123456"},
			scope:       providers.AccessScopeOTP,
			isEvent:     true,
		},
		{
			name: "passkey beats the otp for the same reason",
			credentials: map[string]interface{}{
				authnprovidercm.CredentialTypePasskey: "assertion", authnprovidercm.CredentialTypeOTP: "123456",
			},
			scope:   providers.AccessScopePasskey,
			isEvent: true,
		},
		{
			name: "a provisioned entity is not an authentication event, whatever rides along",
			credentials: map[string]interface{}{
				authnprovidercm.CredentialTypeProvisionedEntityID: "u1", "password": "p",
			},
			isEvent: false,
		},
		{
			name:        "sub is no longer reserved, so it takes the default arm",
			credentials: map[string]interface{}{authnprovidercm.UserAttributeSub: "s1"},
			scope:       providers.AccessScopeCredential,
			isEvent:     true,
		},
		{
			name:        "a password alone is the default arm",
			credentials: map[string]interface{}{"password": "p"},
			scope:       providers.AccessScopeCredential,
			isEvent:     true,
		},
		{
			name:        "so is any other secret the schema declares",
			credentials: map[string]interface{}{"pin": "1234", "secretAnswer": "blue"},
			scope:       providers.AccessScopeCredential,
			isEvent:     true,
		},
		{
			name:        "a client secret alone is the machine scope",
			credentials: map[string]interface{}{authnprovidercm.CredentialTypeClientSecret: "s"},
			scope:       providers.AccessScopeSystemCredential,
			isEvent:     true,
		},
		{
			name: "a machine credential beside a password does not downgrade the scope",
			credentials: map[string]interface{}{
				authnprovidercm.CredentialTypeClientSecret: "s", "password": "p",
			},
			scope:   providers.AccessScopeCredential,
			isEvent: true,
		},
		{
			name:        "an empty map names nothing",
			credentials: map[string]interface{}{},
			isEvent:     false,
		},
		{
			name:        "a nil map names nothing",
			credentials: nil,
			isEvent:     false,
		},
		{
			name:        "an unrecognized key still takes the default arm",
			credentials: map[string]interface{}{"somethingNew": "x"},
			scope:       providers.AccessScopeCredential,
			isEvent:     true,
		},
		{
			name:        "federated",
			credentials: map[string]interface{}{authnprovidercm.CredentialTypeFederated: "code"},
			scope:       providers.AccessScopeFederated,
			isEvent:     true,
		},
		{
			name:        "magic link",
			credentials: map[string]interface{}{authnprovidercm.CredentialTypeMagicLink: "token"},
			scope:       providers.AccessScopeMagicLink,
			isEvent:     true,
		},
		{
			name:        "verifiable presentation",
			credentials: map[string]interface{}{authnprovidercm.CredentialTypeOpenID4VP: "vp"},
			scope:       providers.AccessScopeOpenID4VP,
			isEvent:     true,
		},
		{
			name:        "the flow secret is a machine credential",
			credentials: map[string]interface{}{authnprovidercm.CredentialTypeFlowSecret: "s"},
			scope:       providers.AccessScopeSystemCredential,
			isEvent:     true,
		},
	}

	for _, test := range tests {
		suite.Run(test.name, func() {
			scope, isEvent := selectedScope(test.credentials)
			suite.Equal(test.isEvent, isEvent)
			suite.Equal(test.scope, scope)
		})
	}
}

func selectedScope(credentials map[string]interface{}) (providers.AccessScope, bool) {
	method := selectAuthenticationMethod(credentials)
	return method.scope, method.scope != ""
}
