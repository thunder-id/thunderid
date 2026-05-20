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

package manager

import (
	"github.com/thunder-id/thunderid/internal/system/i18n/core"

	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/providermock"
)

type ManagerTestSuite struct {
	suite.Suite
	mockProvider *providermock.AuthnProviderInterfaceMock
	mgr          AuthnProviderManagerInterface
}

func TestManagerTestSuite(t *testing.T) {
	suite.Run(t, new(ManagerTestSuite))
}

func (s *ManagerTestSuite) SetupTest() {
	s.mockProvider = providermock.NewAuthnProviderInterfaceMock(s.T())
	s.mgr = newAuthnProviderManager(s.mockProvider)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Success() {
	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "secret"}
	meta := &authnprovidercm.AuthnMetadata{}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return(&authnprovidercm.AuthnResult{
			UserID:                    "user-1",
			UserType:                  "customer",
			OUID:                      "ou-1",
			Token:                     "tok",
			IsAttributeValuesIncluded: false,
			AttributesResponse:        nil,
			IsExistingUser:            true,
		}, (*serviceerror.ServiceError)(nil))

	returnedAuthUser, result, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, AuthUser{})

	s.Nil(svcErr)
	s.NotNil(result)
	s.Equal("user-1", result.UserID)
	s.Equal("ou-1", result.OUID)
	s.Equal("customer", result.UserType)

	s.True(returnedAuthUser.IsAuthenticated())
	s.Equal("user-1", returnedAuthUser.userID)
	s.Equal("customer", returnedAuthUser.userType)
	s.Equal("ou-1", returnedAuthUser.ouID)

	pd, ok := returnedAuthUser.getProviderData(defaultProvider)
	s.True(ok)
	s.Equal("tok", pd.token)
	s.False(pd.isAttributeValuesIncluded)
}

func (s *ManagerTestSuite) TestAuthenticateUser_FederatedNewUser() {
	identifiers := map[string]interface{}{}
	credentials := map[string]interface{}{"federated": "token"}
	meta := &authnprovidercm.AuthnMetadata{}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return(&authnprovidercm.AuthnResult{
			IsExistingUser:  false,
			IsAmbiguousUser: false,
			ExternalSub:     "ext-sub-123",
			ExternalClaims:  map[string]interface{}{"email": "new@example.com"},
		}, (*serviceerror.ServiceError)(nil))

	returnedAuthUser, result, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, AuthUser{})

	s.Nil(svcErr)
	s.NotNil(result)
	s.False(result.IsExistingUser)
	s.False(result.IsAmbiguousUser)
	s.Equal("ext-sub-123", result.ExternalSub)
	s.Equal(map[string]interface{}{"email": "new@example.com"}, result.ExternalClaims)

	s.False(returnedAuthUser.IsAuthenticated())
	_, ok := returnedAuthUser.getProviderData(defaultProvider)
	s.False(ok)
}

func (s *ManagerTestSuite) TestAuthenticateUser_ClientError() {
	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "wrong"}
	meta := &authnprovidercm.AuthnMetadata{}
	provErr := &serviceerror.ServiceError{
		Code:  "PROV-ERR",
		Type:  serviceerror.ClientErrorType,
		Error: core.I18nMessage{Key: "error.test.invalid_credentials", DefaultValue: "invalid credentials"},
	}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return((*authnprovidercm.AuthnResult)(nil), provErr)

	returnedAuthUser, result, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, AuthUser{})

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
	s.Equal(serviceerror.ClientErrorType, svcErr.Type)
	s.Nil(result)
	s.False(returnedAuthUser.IsAuthenticated())
}

func (s *ManagerTestSuite) TestAuthenticateUser_UserNotFound() {
	s.assertAuthenticateUserClientErrorMapping(
		authnprovidercm.ErrorCodeUserNotFound,
		"user not found",
		"no user matches",
		ErrorUserNotFound.Code,
	)
}

func (s *ManagerTestSuite) TestAuthenticateUser_InvalidRequest() {
	s.assertAuthenticateUserClientErrorMapping(
		authnprovidercm.ErrorCodeInvalidRequest,
		"invalid request",
		"missing required field",
		ErrorInvalidRequest.Code,
	)
}

func (s *ManagerTestSuite) assertAuthenticateUserClientErrorMapping(
	providerErrorCode, providerError, providerErrorDescription, expectedServiceErrorCode string,
) {
	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "secret"}
	meta := &authnprovidercm.AuthnMetadata{}
	provErr := &serviceerror.ServiceError{
		Code: providerErrorCode, Type: serviceerror.ClientErrorType,
		Error:            core.I18nMessage{DefaultValue: providerError},
		ErrorDescription: core.I18nMessage{DefaultValue: providerErrorDescription},
	}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return((*authnprovidercm.AuthnResult)(nil), provErr)

	returnedAuthUser, result, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, AuthUser{})

	s.NotNil(svcErr)
	s.Equal(expectedServiceErrorCode, svcErr.Code)
	s.Equal(serviceerror.ClientErrorType, svcErr.Type)
	s.Equal(providerErrorDescription, svcErr.ErrorDescription.DefaultValue)
	s.Nil(result)
	s.False(returnedAuthUser.IsAuthenticated())
}

func (s *ManagerTestSuite) TestAuthenticateUser_ServerError() {
	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "secret"}
	meta := &authnprovidercm.AuthnMetadata{}
	provErr := &serviceerror.ServiceError{
		Code:  "PROV-ERR",
		Type:  serviceerror.ServerErrorType,
		Error: core.I18nMessage{Key: "error.test.database_unavailable", DefaultValue: "database unavailable"},
	}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return((*authnprovidercm.AuthnResult)(nil), provErr)

	returnedAuthUser, result, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, AuthUser{})

	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
	s.Equal(serviceerror.ServerErrorType, svcErr.Type)
	s.Nil(result)
	s.False(returnedAuthUser.IsAuthenticated())
}

func (s *ManagerTestSuite) TestAuthenticateUser_ReAuth() {
	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "secret"}
	meta := &authnprovidercm.AuthnMetadata{}

	firstResult := &authnprovidercm.AuthnResult{
		UserID: "user-1", UserType: "customer", OUID: "ou-1", Token: "tok-first", IsExistingUser: true,
	}
	secondResult := &authnprovidercm.AuthnResult{
		UserID: "user-1", UserType: "customer", OUID: "ou-1", Token: "tok-second", IsExistingUser: true,
	}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return(firstResult, (*serviceerror.ServiceError)(nil)).Once()
	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return(secondResult, (*serviceerror.ServiceError)(nil)).Once()

	au1, _, _ := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials, nil, meta, AuthUser{})
	au2, _, _ := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials, nil, meta, au1)

	pd, ok := au2.getProviderData(defaultProvider)
	s.True(ok)
	s.Equal("tok-second", pd.token, "second call must overwrite provider data")
}

func (s *ManagerTestSuite) TestGetUserAvailableAttributes_EmptyAuthUser() {
	attrs, svcErr := s.mgr.GetUserAvailableAttributes(context.Background(), AuthUser{})
	s.Nil(attrs)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestGetUserAvailableAttributes_WithData() {
	var authUser AuthUser
	authUser.setIdentity("user-1", "person", "ou-1")
	expectedAttrs := &authnprovidercm.AttributesResponse{
		Attributes: map[string]*authnprovidercm.AttributeResponse{
			"email": {Value: "a@b.com"},
		},
	}
	authUser.setProviderData(defaultProvider, providerData{
		token:                     "tok",
		attributes:                expectedAttrs,
		isAttributeValuesIncluded: true,
	})

	attrs, svcErr := s.mgr.GetUserAvailableAttributes(context.Background(), authUser)
	s.Nil(svcErr)
	s.Equal(expectedAttrs, attrs)
	// No provider call should have been made
	s.mockProvider.AssertNotCalled(s.T(), "GetAttributes")
}

func (s *ManagerTestSuite) TestGetUserAttributes_EmptyAuthUser() {
	_, attrs, svcErr := s.mgr.GetUserAttributes(context.Background(), nil, nil, AuthUser{})
	s.Nil(attrs)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestGetUserAttributes_CacheHit() {
	var authUser AuthUser
	authUser.setIdentity("user-1", "person", "ou-1")
	expectedAttrs := &authnprovidercm.AttributesResponse{
		Attributes: map[string]*authnprovidercm.AttributeResponse{
			"email": {Value: "a@b.com"},
		},
	}
	authUser.setProviderData(defaultProvider, providerData{
		token:                     "tok",
		attributes:                expectedAttrs,
		isAttributeValuesIncluded: true,
	})

	retAuthUser, attrs, svcErr := s.mgr.GetUserAttributes(context.Background(), nil, nil, authUser)
	s.Nil(svcErr)
	s.Equal(expectedAttrs, attrs)
	s.Equal(authUser, retAuthUser)
	s.mockProvider.AssertNotCalled(s.T(), "GetAttributes")
}

func (s *ManagerTestSuite) TestGetUserAvailableAttributes_NoProviderData() {
	var authUser AuthUser
	authUser.setIdentity("user-1", "person", "ou-1") // authenticated but no provider data set
	attrs, svcErr := s.mgr.GetUserAvailableAttributes(context.Background(), authUser)
	s.Nil(attrs)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestGetUserAttributes_NoProviderData() {
	var authUser AuthUser
	authUser.setIdentity("user-1", "person", "ou-1") // authenticated but no provider data set
	_, attrs, svcErr := s.mgr.GetUserAttributes(context.Background(), nil, nil, authUser)
	s.Nil(attrs)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestGetUserAttributes_CacheMissServerError() {
	var authUser AuthUser
	authUser.setIdentity("user-1", "person", "ou-1")
	authUser.setProviderData(defaultProvider, providerData{token: "tok", isAttributeValuesIncluded: false})

	requestedAttrs := &authnprovidercm.RequestedAttributes{}
	provErr := &serviceerror.ServiceError{
		Code:  "PROVIDER-ERR",
		Type:  serviceerror.ServerErrorType,
		Error: core.I18nMessage{Key: "error.test.provider_failure", DefaultValue: "provider failure"},
	}

	s.mockProvider.On("GetAttributes", context.Background(), "tok", requestedAttrs,
		(*authnprovidercm.GetAttributesMetadata)(nil)).
		Return((*authnprovidercm.GetAttributesResult)(nil), provErr)

	_, attrs, svcErr := s.mgr.GetUserAttributes(context.Background(), requestedAttrs, nil, authUser)
	s.Nil(attrs)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
	s.Equal(serviceerror.ServerErrorType, svcErr.Type)
}

func (s *ManagerTestSuite) TestGetUserAttributes_CacheMissClientError() {
	var authUser AuthUser
	authUser.setIdentity("user-1", "person", "ou-1")
	authUser.setProviderData(defaultProvider, providerData{token: "expired-tok", isAttributeValuesIncluded: false})

	requestedAttrs := &authnprovidercm.RequestedAttributes{}
	provErr := &serviceerror.ServiceError{
		Code:  "PROVIDER-ERR",
		Type:  serviceerror.ClientErrorType,
		Error: core.I18nMessage{Key: "error.test.token_expired", DefaultValue: "token expired"},
	}

	s.mockProvider.On("GetAttributes", context.Background(), "expired-tok", requestedAttrs,
		(*authnprovidercm.GetAttributesMetadata)(nil)).
		Return((*authnprovidercm.GetAttributesResult)(nil), provErr)

	_, attrs, svcErr := s.mgr.GetUserAttributes(context.Background(), requestedAttrs, nil, authUser)
	s.Nil(attrs)
	s.NotNil(svcErr)
	s.Equal(ErrorGetAttributesClientError.Code, svcErr.Code)
	s.Equal(serviceerror.ClientErrorType, svcErr.Type)
}

func (s *ManagerTestSuite) TestGetUserAttributes_CacheMiss() {
	var authUser AuthUser
	authUser.setIdentity("user-1", "person", "ou-1")
	authUser.setProviderData(defaultProvider, providerData{
		token:                     "tok",
		attributes:                nil,
		isAttributeValuesIncluded: false,
	})

	requestedAttrs := &authnprovidercm.RequestedAttributes{}
	fetchedAttrs := &authnprovidercm.AttributesResponse{
		Attributes: map[string]*authnprovidercm.AttributeResponse{
			"email": {Value: "fetched@b.com"},
		},
	}

	s.mockProvider.On("GetAttributes", context.Background(), "tok", requestedAttrs,
		(*authnprovidercm.GetAttributesMetadata)(nil)).
		Return(&authnprovidercm.GetAttributesResult{AttributesResponse: fetchedAttrs},
			(*serviceerror.ServiceError)(nil))

	retAuthUser, attrs, svcErr := s.mgr.GetUserAttributes(context.Background(), requestedAttrs, nil, authUser)
	s.Nil(svcErr)
	s.Equal(fetchedAttrs, attrs)
	retData, ok := retAuthUser.getProviderData(defaultProvider)
	s.True(ok)
	s.True(retData.isAttributeValuesIncluded)
	s.Equal(fetchedAttrs, retData.attributes)
}
