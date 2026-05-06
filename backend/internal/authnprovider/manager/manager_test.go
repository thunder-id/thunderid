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
	"github.com/asgardeo/thunder/internal/system/i18n/core"

	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	authnprovidercm "github.com/asgardeo/thunder/internal/authnprovider/common"
	"github.com/asgardeo/thunder/internal/system/error/serviceerror"
	"github.com/asgardeo/thunder/tests/mocks/authnprovider/providermock"
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
	authnType := authnprovidercm.AuthnDataTypeCredentials
	authnData := &authnprovidercm.CredentialsAuthnData{
		Identifiers: map[string]interface{}{"username": "alice"},
		Credentials: map[string]interface{}{"password": "secret"},
	}
	meta := &authnprovidercm.AuthnMetadata{}

	s.mockProvider.On("Authenticate", context.Background(), authnType, authnData, meta).
		Return(&authnprovidercm.AuthnResult{
			UserID:                    "user-1",
			UserType:                  "customer",
			OUID:                      "ou-1",
			Token:                     "tok",
			IsAttributeValuesIncluded: false,
			AttributesResponse:        &authnprovidercm.AttributesResponse{},
			IsExistingUser:            true,
		}, (*serviceerror.ServiceError)(nil))

	returnedAuthUser, svcErr := s.mgr.AuthenticateUser(context.Background(), authnType, authnData,
		nil, meta, AuthUser{})

	s.Nil(svcErr)
	s.True(returnedAuthUser.IsAuthenticated())
	s.Equal("user-1", returnedAuthUser.GetUserID())
	s.Equal("customer", returnedAuthUser.GetUserType())
	s.Equal("ou-1", returnedAuthUser.GetOUID())

	s.Require().Len(returnedAuthUser.authHistory, 1)
	s.Require().Len(returnedAuthUser.userHistory, 1)
	ur := returnedAuthUser.userHistory[0]
	s.Equal("tok", ur.token)
	s.False(ur.isValuesIncluded)
}

func (s *ManagerTestSuite) TestAuthenticateUser_FederatedNewUser() {
	authnType := authnprovidercm.AuthnDataTypeFederated
	authnData := &authnprovidercm.FederatedAuthnData{
		OAuthCredential: authnprovidercm.OAuthCredential{Code: "token"},
	}
	meta := &authnprovidercm.AuthnMetadata{}

	attrResp := &authnprovidercm.AttributesResponse{
		Attributes: map[string]*authnprovidercm.AttributeResponse{
			"email": {Value: "new@example.com"},
		},
		Verifications: make(map[string]*authnprovidercm.VerificationResponse),
	}
	s.mockProvider.On("Authenticate", context.Background(), authnType, authnData, meta).
		Return(&authnprovidercm.AuthnResult{
			AuthType:                  authnprovidercm.AuthenticatorOAuth,
			IsExistingUser:            false,
			IsAmbiguousUser:           false,
			ExternalSub:               "ext-sub-123",
			ExternalClaims:            map[string]interface{}{"email": "new@example.com"},
			IsAttributeValuesIncluded: true,
			AttributesResponse:        attrResp,
		}, (*serviceerror.ServiceError)(nil))

	returnedAuthUser, svcErr := s.mgr.AuthenticateUser(context.Background(), authnType, authnData,
		nil, meta, AuthUser{})

	s.Nil(svcErr)
	s.False(returnedAuthUser.IsAuthenticated())
	s.Empty(returnedAuthUser.userHistory)

	s.Require().Len(returnedAuthUser.authHistory, 1)
	ar := returnedAuthUser.authHistory[0]
	s.Equal("ext-sub-123", returnedAuthUser.GetLastFederatedSub())
	s.Equal("new@example.com", ar.runtimeAttributes["email"])
	s.False(returnedAuthUser.IsLocalUserAmbiguous())
}

func (s *ManagerTestSuite) TestAuthenticateUser_ClientError() {
	authnType := authnprovidercm.AuthnDataTypeCredentials
	authnData := &authnprovidercm.CredentialsAuthnData{
		Identifiers: map[string]interface{}{"username": "alice"},
		Credentials: map[string]interface{}{"password": "wrong"},
	}
	meta := &authnprovidercm.AuthnMetadata{}
	provErr := &serviceerror.ServiceError{
		Code:  "PROV-ERR",
		Type:  serviceerror.ClientErrorType,
		Error: core.I18nMessage{Key: "error.test.invalid_credentials", DefaultValue: "invalid credentials"},
	}

	s.mockProvider.On("Authenticate", context.Background(), authnType, authnData, meta).
		Return((*authnprovidercm.AuthnResult)(nil), provErr)

	returnedAuthUser, svcErr := s.mgr.AuthenticateUser(context.Background(), authnType, authnData,
		nil, meta, AuthUser{})

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
	s.Equal(serviceerror.ClientErrorType, svcErr.Type)
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
	authnType := authnprovidercm.AuthnDataTypeCredentials
	authnData := &authnprovidercm.CredentialsAuthnData{
		Identifiers: map[string]interface{}{"username": "alice"},
		Credentials: map[string]interface{}{"password": "secret"},
	}
	meta := &authnprovidercm.AuthnMetadata{}
	provErr := &serviceerror.ServiceError{
		Code: providerErrorCode, Type: serviceerror.ClientErrorType,
		Error:            core.I18nMessage{DefaultValue: providerError},
		ErrorDescription: core.I18nMessage{DefaultValue: providerErrorDescription},
	}

	s.mockProvider.On("Authenticate", context.Background(), authnType, authnData, meta).
		Return((*authnprovidercm.AuthnResult)(nil), provErr)

	returnedAuthUser, svcErr := s.mgr.AuthenticateUser(context.Background(), authnType, authnData,
		nil, meta, AuthUser{})

	s.NotNil(svcErr)
	s.Equal(expectedServiceErrorCode, svcErr.Code)
	s.Equal(serviceerror.ClientErrorType, svcErr.Type)
	s.False(returnedAuthUser.IsAuthenticated())
}

func (s *ManagerTestSuite) TestAuthenticateUser_ServerError() {
	authnType := authnprovidercm.AuthnDataTypeCredentials
	authnData := &authnprovidercm.CredentialsAuthnData{
		Identifiers: map[string]interface{}{"username": "alice"},
		Credentials: map[string]interface{}{"password": "secret"},
	}
	meta := &authnprovidercm.AuthnMetadata{}
	provErr := &serviceerror.ServiceError{
		Code:  "PROV-ERR",
		Type:  serviceerror.ServerErrorType,
		Error: core.I18nMessage{Key: "error.test.database_unavailable", DefaultValue: "database unavailable"},
	}

	s.mockProvider.On("Authenticate", context.Background(), authnType, authnData, meta).
		Return((*authnprovidercm.AuthnResult)(nil), provErr)

	returnedAuthUser, svcErr := s.mgr.AuthenticateUser(context.Background(), authnType, authnData,
		nil, meta, AuthUser{})

	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
	s.Equal(serviceerror.ServerErrorType, svcErr.Type)
	s.False(returnedAuthUser.IsAuthenticated())
}

func (s *ManagerTestSuite) TestAuthenticateUser_ReAuth() {
	authnType := authnprovidercm.AuthnDataTypeCredentials
	authnData := &authnprovidercm.CredentialsAuthnData{
		Identifiers: map[string]interface{}{"username": "alice"},
		Credentials: map[string]interface{}{"password": "secret"},
	}
	meta := &authnprovidercm.AuthnMetadata{}

	firstResult := &authnprovidercm.AuthnResult{
		UserID: "user-1", UserType: "customer", OUID: "ou-1", Token: "tok-first", IsExistingUser: true,
		AttributesResponse: &authnprovidercm.AttributesResponse{},
	}
	secondResult := &authnprovidercm.AuthnResult{
		UserID: "user-1", UserType: "customer", OUID: "ou-1", Token: "tok-second", IsExistingUser: true,
		AttributesResponse: &authnprovidercm.AttributesResponse{},
	}

	s.mockProvider.On("Authenticate", context.Background(), authnType, authnData, meta).
		Return(firstResult, (*serviceerror.ServiceError)(nil)).Once()
	s.mockProvider.On("Authenticate", context.Background(), authnType, authnData, meta).
		Return(secondResult, (*serviceerror.ServiceError)(nil)).Once()

	au1, _ := s.mgr.AuthenticateUser(context.Background(), authnType, authnData, nil, meta, AuthUser{})
	au2, _ := s.mgr.AuthenticateUser(context.Background(), authnType, authnData, nil, meta, au1)

	s.Require().Len(au2.userHistory, 2)
	s.Equal("tok-second", au2.userHistory[1].token, "second call must append a new user history entry")
}

func (s *ManagerTestSuite) TestGetUserAvailableAttributes_EmptyAuthUser() {
	attrs, svcErr := s.mgr.GetUserAvailableAttributes(context.Background(), AuthUser{})
	s.Nil(svcErr)
	s.NotNil(attrs)
	s.Empty(attrs.Attributes)
}

func (s *ManagerTestSuite) TestGetUserAvailableAttributes_WithData() {
	expectedAttrs := &authnprovidercm.AttributesResponse{
		Attributes: map[string]*authnprovidercm.AttributeResponse{
			"email": {Value: "a@b.com"},
		},
	}
	authUser := AuthUser{
		userState: ProviderUserStateExists,
		authHistory: []*authResult{
			{authenticator: "password", isVerified: true},
		},
		userHistory: []*providerUserResult{
			{
				userID:           "user-1",
				userType:         "person",
				ouID:             "ou-1",
				attributes:       map[string]interface{}{"email": "a@b.com"},
				isValuesIncluded: true,
			},
		},
	}

	attrs, svcErr := s.mgr.GetUserAvailableAttributes(context.Background(), authUser)
	s.Nil(svcErr)
	s.Equal(expectedAttrs.Attributes, attrs.Attributes)
	// No provider call should have been made
	s.mockProvider.AssertNotCalled(s.T(), "GetAttributes")
}

func (s *ManagerTestSuite) TestGetUserAttributes_EmptyAuthUser() {
	_, attrs, svcErr := s.mgr.GetUserAttributes(context.Background(), nil, nil, AuthUser{})
	s.Nil(svcErr)
	s.NotNil(attrs)
	s.Empty(attrs.Attributes)
}

func (s *ManagerTestSuite) TestGetUserAttributes_CacheHit() {
	expectedAttrs := &authnprovidercm.AttributesResponse{
		Attributes: map[string]*authnprovidercm.AttributeResponse{
			"email": {Value: "a@b.com"},
		},
	}
	authUser := AuthUser{
		userState: ProviderUserStateExists,
		authHistory: []*authResult{
			{authenticator: "password", isVerified: true},
		},
		userHistory: []*providerUserResult{
			{
				userID:           "user-1",
				userType:         "person",
				ouID:             "ou-1",
				attributes:       map[string]interface{}{"email": "a@b.com"},
				isValuesIncluded: true,
			},
		},
	}

	retAuthUser, attrs, svcErr := s.mgr.GetUserAttributes(context.Background(), nil, nil, authUser)
	s.Nil(svcErr)
	s.Equal(expectedAttrs.Attributes, attrs.Attributes)
	s.Equal(authUser, retAuthUser)
	s.mockProvider.AssertNotCalled(s.T(), "GetAttributes")
}

func (s *ManagerTestSuite) TestGetUserAvailableAttributes_NoProviderData() {
	authUser := AuthUser{
		userState: ProviderUserStateExists,
		authHistory: []*authResult{
			{authenticator: "password", isVerified: true},
		},
		userHistory: []*providerUserResult{
			{userID: "user-1", userType: "person", ouID: "ou-1", isValuesIncluded: true},
		},
	}
	attrs, svcErr := s.mgr.GetUserAvailableAttributes(context.Background(), authUser)
	s.Nil(svcErr)
	s.NotNil(attrs)
	s.Empty(attrs.Attributes)
}

func (s *ManagerTestSuite) TestGetUserAttributes_NoProviderData() {
	authUser := AuthUser{
		userState: ProviderUserStateExists,
		authHistory: []*authResult{
			{authenticator: "password", isVerified: true},
		},
		userHistory: []*providerUserResult{
			{userID: "user-1", userType: "person", ouID: "ou-1", isValuesIncluded: true},
		},
	}
	_, attrs, svcErr := s.mgr.GetUserAttributes(context.Background(), nil, nil, authUser)
	s.Nil(svcErr)
	s.NotNil(attrs)
	s.Empty(attrs.Attributes)
}

func (s *ManagerTestSuite) TestGetUserAttributes_CacheMissServerError() {
	authUser := AuthUser{
		userState: ProviderUserStateExists,
		authHistory: []*authResult{
			{authenticator: "password", isVerified: true},
		},
		userHistory: []*providerUserResult{
			{userID: "user-1", userType: "person", ouID: "ou-1", token: "tok", isValuesIncluded: false},
		},
	}

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
	authUser := AuthUser{
		userState: ProviderUserStateExists,
		authHistory: []*authResult{
			{authenticator: "password", isVerified: true},
		},
		userHistory: []*providerUserResult{
			{userID: "user-1", userType: "person", ouID: "ou-1", token: "expired-tok", isValuesIncluded: false},
		},
	}

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
	authUser := AuthUser{
		userState: ProviderUserStateExists,
		authHistory: []*authResult{
			{authenticator: "password", isVerified: true},
		},
		userHistory: []*providerUserResult{
			{userID: "user-1", userType: "person", ouID: "ou-1", token: "tok", isValuesIncluded: false},
		},
	}

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
	s.Equal(fetchedAttrs.Attributes, attrs.Attributes)
	s.Require().Len(retAuthUser.userHistory, 1)
	s.True(retAuthUser.userHistory[0].isValuesIncluded)
	s.Equal(map[string]interface{}{"email": "fetched@b.com"}, retAuthUser.userHistory[0].attributes)
}

func TestIsAttributeRequested(t *testing.T) {
	tests := []struct {
		name                string
		attrName            string
		requestedAttributes *authnprovidercm.RequestedAttributes
		want                bool
	}{
		{
			name:                "nil requestedAttributes includes all",
			attrName:            "email",
			requestedAttributes: nil,
			want:                true,
		},
		{
			name:                "nil Attributes map includes all",
			attrName:            "email",
			requestedAttributes: &authnprovidercm.RequestedAttributes{Attributes: nil},
			want:                true,
		},
		{
			name:     "attribute present in filter with nil value",
			attrName: "email",
			requestedAttributes: &authnprovidercm.RequestedAttributes{
				Attributes: map[string]*authnprovidercm.AttributeMetadataRequest{
					"email": nil,
				},
			},
			want: true,
		},
		{
			name:     "attribute present in filter with non-nil value",
			attrName: "email",
			requestedAttributes: &authnprovidercm.RequestedAttributes{
				Attributes: map[string]*authnprovidercm.AttributeMetadataRequest{
					"email": {},
				},
			},
			want: true,
		},
		{
			name:     "attribute absent from filter",
			attrName: "phone",
			requestedAttributes: &authnprovidercm.RequestedAttributes{
				Attributes: map[string]*authnprovidercm.AttributeMetadataRequest{
					"email": nil,
				},
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isAttributeRequested(tc.attrName, tc.requestedAttributes)
			if got != tc.want {
				t.Errorf("isAttributeRequested(%q, ...) = %v, want %v", tc.attrName, got, tc.want)
			}
		})
	}
}
