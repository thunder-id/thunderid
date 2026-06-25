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
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/authnprovider/provider"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
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
	mgr, err := newAuthnProviderManager(
		map[string]provider.AuthnProviderInterface{string(defaultProviderName): s.mockProvider},
		nil,
	)
	s.Require().NoError(err)
	s.mgr = mgr
}

// --- helpers to build authenticated AuthUser instances ---

func authenticatedAuthUserWithTokens(entityRefToken any, attrToken any) AuthUser {
	return AuthUser{
		state: map[providerName]authState{
			defaultProviderName: {
				entityReferenceToken: entityRefToken,
				attributeToken:       attrToken,
			},
		},
	}
}

func authenticatedAuthUserWithResolved(entityRef *authnprovidercm.EntityReference,
	attrs *authnprovidercm.AttributesResponse) AuthUser {
	return AuthUser{
		state: map[providerName]authState{
			defaultProviderName: {
				entityReference: entityRef,
				attributes:      attrs,
			},
		},
	}
}

// --- AuthenticateUser tests ---

func (s *ManagerTestSuite) TestAuthenticateUser_Success_WithTokens() {
	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "secret"}
	meta := &authnprovidercm.AuthnMetadata{}

	entityRefToken := map[string]interface{}{"userID": "user-1"}
	attrToken := map[string]interface{}{"token": "attr-tok"}
	runtimeAttrs := authnprovidercm.AuthenticatedClaims{"sessionId": "sess-1"}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return(&authnprovidercm.AuthnResult{
			EntityReferenceToken: entityRefToken,
			AttributeToken:       attrToken,
			AuthenticatedClaims:  runtimeAttrs,
		}, (*serviceerror.ServiceError)(nil))

	returnedAuthUser, rtAttrs, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, AuthUser{})

	s.Nil(svcErr)
	s.Equal(runtimeAttrs, rtAttrs)
	s.True(returnedAuthUser.IsAuthenticated())
	st := returnedAuthUser.state[defaultProviderName]
	s.Equal(entityRefToken, st.entityReferenceToken)
	s.Nil(st.entityReference)
	s.Equal(attrToken, st.attributeToken)
	s.Nil(st.attributes)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Success_WithResolvedValues() {
	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "secret"}
	meta := &authnprovidercm.AuthnMetadata{}

	entityRef := &authnprovidercm.EntityReference{
		EntityID: "user-1", EntityCategory: "person", EntityType: "default", OUID: "ou-1",
	}
	attrs := &authnprovidercm.AttributesResponse{
		Attributes: map[string]*authnprovidercm.AttributeResponse{
			"email": {Value: "alice@example.com"},
		},
	}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return(&authnprovidercm.AuthnResult{
			EntityReference: entityRef,
			Attributes:      attrs,
		}, (*serviceerror.ServiceError)(nil))

	returnedAuthUser, rtAttrs, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, AuthUser{})

	s.Nil(svcErr)
	s.Nil(rtAttrs)
	s.True(returnedAuthUser.IsAuthenticated())
	st := returnedAuthUser.state[defaultProviderName]
	s.Nil(st.entityReferenceToken)
	s.Equal(entityRef, st.entityReference)
	s.Nil(st.attributeToken)
	s.Equal(attrs, st.attributes)
}

func (s *ManagerTestSuite) TestAuthenticateUser_EmptyCredentials() {
	identifiers := map[string]interface{}{"username": "alice"}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, map[string]interface{}{},
		nil, nil, AuthUser{})

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
	s.mockProvider.AssertNotCalled(s.T(), "Authenticate")
}

func (s *ManagerTestSuite) TestAuthenticateUser_NilCredentials() {
	identifiers := map[string]interface{}{"username": "alice"}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, nil, nil, nil, AuthUser{})

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
	s.mockProvider.AssertNotCalled(s.T(), "Authenticate")
}

func (s *ManagerTestSuite) TestAuthenticateUser_UnknownCredentialKey() {
	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"unknownCred": "value"}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials, nil, nil, AuthUser{})

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
	s.mockProvider.AssertNotCalled(s.T(), "Authenticate")
}

func (s *ManagerTestSuite) TestAuthenticateUser_MissingEntityAndAttributes() {
	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "secret"}
	meta := &authnprovidercm.AuthnMetadata{}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return(&authnprovidercm.AuthnResult{}, (*serviceerror.ServiceError)(nil))

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, AuthUser{})

	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_MissingEntityRef() {
	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "secret"}
	meta := &authnprovidercm.AuthnMetadata{}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return(&authnprovidercm.AuthnResult{
			AttributeToken: "tok",
		}, (*serviceerror.ServiceError)(nil))

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, AuthUser{})

	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_MissingAttributes() {
	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "secret"}
	meta := &authnprovidercm.AuthnMetadata{}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return(&authnprovidercm.AuthnResult{
			EntityReferenceToken: "ref-tok",
		}, (*serviceerror.ServiceError)(nil))

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, AuthUser{})

	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_ClientError() {
	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "wrong"}
	meta := &authnprovidercm.AuthnMetadata{}
	provErr := &serviceerror.ServiceError{
		Code:             "PROV-ERR",
		Type:             serviceerror.ClientErrorType,
		Error:            core.I18nMessage{Key: "error.test.invalid_credentials", DefaultValue: "invalid credentials"},
		ErrorDescription: core.I18nMessage{DefaultValue: "bad creds"},
	}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return((*authnprovidercm.AuthnResult)(nil), provErr)

	returnedAuthUser, rtAttrs, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, AuthUser{})

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
	s.Equal(serviceerror.ClientErrorType, svcErr.Type)
	s.Nil(rtAttrs)
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

	returnedAuthUser, rtAttrs, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, AuthUser{})

	s.NotNil(svcErr)
	s.Equal(expectedServiceErrorCode, svcErr.Code)
	s.Equal(serviceerror.ClientErrorType, svcErr.Type)
	s.Nil(rtAttrs)
	s.False(returnedAuthUser.IsAuthenticated())
}

func (s *ManagerTestSuite) TestAuthenticateUser_ServerError() {
	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "secret"}
	meta := &authnprovidercm.AuthnMetadata{}
	provErr := &serviceerror.ServiceError{
		Code: "PROV-ERR",
		Type: serviceerror.ServerErrorType,
		Error: core.I18nMessage{
			Key: "error.test.database_unavailable", DefaultValue: "database unavailable",
		},
		ErrorDescription: core.I18nMessage{DefaultValue: "db down"},
	}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return((*authnprovidercm.AuthnResult)(nil), provErr)

	returnedAuthUser, rtAttrs, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, AuthUser{})

	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
	s.Equal(serviceerror.ServerErrorType, svcErr.Type)
	s.Nil(rtAttrs)
	s.False(returnedAuthUser.IsAuthenticated())
}

func (s *ManagerTestSuite) TestAuthenticateUser_ReAuth() {
	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "secret"}
	meta := &authnprovidercm.AuthnMetadata{}

	firstResult := &authnprovidercm.AuthnResult{
		EntityReferenceToken: map[string]interface{}{"userID": "user-1"},
		AttributeToken:       "tok-first",
	}
	secondResult := &authnprovidercm.AuthnResult{
		EntityReferenceToken: map[string]interface{}{"userID": "user-1"},
		AttributeToken:       "tok-second",
	}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return(firstResult, (*serviceerror.ServiceError)(nil)).Once()
	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return(secondResult, (*serviceerror.ServiceError)(nil)).Once()

	au1, _, _ := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials, nil, meta, AuthUser{})
	au2, _, _ := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials, nil, meta, au1)

	s.True(au2.IsAuthenticated())
	s.Equal("tok-second", au2.state[defaultProviderName].attributeToken,
		"second call must overwrite attribute token")
}

// --- Disambiguation (sub in credentials) tests ---

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_Success() {
	identifiers := map[string]interface{}{"userID": "user-123"}
	credentials := map[string]interface{}{"sub": "ext-sub-1"}

	authUser := AuthUser{
		state: map[providerName]authState{
			defaultProviderName: {
				entityReferenceToken: map[string]interface{}{"sub": "ext-sub-1"},
				attributeToken:       "some-token",
			},
		},
	}

	returnedAuthUser, rtAttrs, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, nil, authUser)

	s.Nil(svcErr)
	s.Nil(rtAttrs)
	st := returnedAuthUser.state[defaultProviderName]
	s.Equal(map[string]interface{}{"userID": "user-123"}, st.entityReferenceToken)
	s.Equal(map[string]interface{}{"userID": "user-123"}, st.attributeToken)
	s.Nil(st.entityReference)
	s.Nil(st.attributes)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_EmptySub() {
	credentials := map[string]interface{}{"sub": ""}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, AuthUser{})

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_NonStringSub() {
	credentials := map[string]interface{}{"sub": 123}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, AuthUser{})

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_NotAuthenticated() {
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := AuthUser{}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_NilEntityRefToken() {
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := AuthUser{
		state: map[providerName]authState{
			defaultProviderName: {
				entityReference: &authnprovidercm.EntityReference{EntityID: "user-1"},
				attributes:      &authnprovidercm.AttributesResponse{},
			},
		},
	}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_NonMapEntityRefToken() {
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := AuthUser{
		state: map[providerName]authState{
			defaultProviderName: {
				entityReferenceToken: "not-a-map",
				attributeToken:       "tok",
			},
		},
	}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_MissingSubInEntityRefToken() {
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := AuthUser{
		state: map[providerName]authState{
			defaultProviderName: {
				entityReferenceToken: map[string]interface{}{"other": "value"},
				attributeToken:       "tok",
			},
		},
	}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_SubMismatch() {
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := AuthUser{
		state: map[providerName]authState{
			defaultProviderName: {
				entityReferenceToken: map[string]interface{}{"sub": "ext-sub-different"},
				attributeToken:       "tok",
			},
		},
	}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_MissingUserID() {
	identifiers := map[string]interface{}{}
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := AuthUser{
		state: map[providerName]authState{
			defaultProviderName: {
				entityReferenceToken: map[string]interface{}{"sub": "ext-sub-1"},
				attributeToken:       "tok",
			},
		},
	}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_NonStringUserID() {
	identifiers := map[string]interface{}{"userID": 12345}
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := AuthUser{
		state: map[providerName]authState{
			defaultProviderName: {
				entityReferenceToken: map[string]interface{}{"sub": "ext-sub-1"},
				attributeToken:       "tok",
			},
		},
	}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_EmptyUserID() {
	identifiers := map[string]interface{}{"userID": ""}
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := AuthUser{
		state: map[providerName]authState{
			defaultProviderName: {
				entityReferenceToken: map[string]interface{}{"sub": "ext-sub-1"},
				attributeToken:       "tok",
			},
		},
	}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

// --- GetEntityReference tests ---

func (s *ManagerTestSuite) TestGetEntityReference_EmptyAuthUser() {
	_, _, svcErr := s.mgr.GetEntityReference(context.Background(), AuthUser{})
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestGetEntityReference_AlreadyResolved() {
	entityRef := &authnprovidercm.EntityReference{
		EntityID: "user-1", EntityCategory: "person", EntityType: "default", OUID: "ou-1",
	}
	authUser := authenticatedAuthUserWithResolved(entityRef,
		&authnprovidercm.AttributesResponse{})

	retAuthUser, retRef, svcErr := s.mgr.GetEntityReference(context.Background(), authUser)
	s.Nil(svcErr)
	s.Equal(entityRef, retRef)
	s.Equal(authUser, retAuthUser)
	s.mockProvider.AssertNotCalled(s.T(), "GetEntityReference")
}

func (s *ManagerTestSuite) TestGetEntityReference_FetchFromProvider() {
	entityRefToken := map[string]interface{}{"userID": "user-1"}
	authUser := authenticatedAuthUserWithTokens(entityRefToken, "attr-tok")

	entityRef := &authnprovidercm.EntityReference{
		EntityID: "user-1", EntityCategory: "person", EntityType: "default", OUID: "ou-1",
	}
	s.mockProvider.On("GetEntityReference", context.Background(), entityRefToken).
		Return(entityRef, (*serviceerror.ServiceError)(nil))

	retAuthUser, retRef, svcErr := s.mgr.GetEntityReference(context.Background(), authUser)
	s.Nil(svcErr)
	s.Equal(entityRef, retRef)
	st := retAuthUser.state[defaultProviderName]
	s.Equal(entityRef, st.entityReference)
	s.Nil(st.entityReferenceToken)
}

func (s *ManagerTestSuite) TestGetEntityReference_ServerError() {
	entityRefToken := map[string]interface{}{"userID": "user-1"}
	authUser := authenticatedAuthUserWithTokens(entityRefToken, "attr-tok")

	provErr := &serviceerror.ServiceError{
		Code:             "PROV-ERR",
		Type:             serviceerror.ServerErrorType,
		ErrorDescription: core.I18nMessage{DefaultValue: "provider failure"},
	}
	s.mockProvider.On("GetEntityReference", context.Background(), entityRefToken).
		Return((*authnprovidercm.EntityReference)(nil), provErr)

	_, _, svcErr := s.mgr.GetEntityReference(context.Background(), authUser)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestGetEntityReference_UserNotFound() {
	entityRefToken := map[string]interface{}{"userID": "user-1"}
	authUser := authenticatedAuthUserWithTokens(entityRefToken, "attr-tok")

	provErr := &serviceerror.ServiceError{
		Code:             authnprovidercm.ErrorCodeUserNotFound,
		Type:             serviceerror.ClientErrorType,
		ErrorDescription: core.I18nMessage{DefaultValue: "user not found"},
	}
	s.mockProvider.On("GetEntityReference", context.Background(), entityRefToken).
		Return((*authnprovidercm.EntityReference)(nil), provErr)

	_, _, svcErr := s.mgr.GetEntityReference(context.Background(), authUser)
	s.NotNil(svcErr)
	s.Equal(ErrorUserNotFound.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestGetEntityReference_AmbiguousUser() {
	entityRefToken := map[string]interface{}{"userID": "user-1"}
	authUser := authenticatedAuthUserWithTokens(entityRefToken, "attr-tok")

	provErr := &serviceerror.ServiceError{
		Code:             authnprovidercm.ErrorCodeAmbiguousUser,
		Type:             serviceerror.ClientErrorType,
		ErrorDescription: core.I18nMessage{DefaultValue: "ambiguous user"},
	}
	s.mockProvider.On("GetEntityReference", context.Background(), entityRefToken).
		Return((*authnprovidercm.EntityReference)(nil), provErr)

	_, _, svcErr := s.mgr.GetEntityReference(context.Background(), authUser)
	s.NotNil(svcErr)
	s.Equal(ErrorAmbiguousUser.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestGetEntityReference_OtherClientError() {
	entityRefToken := map[string]interface{}{"userID": "user-1"}
	authUser := authenticatedAuthUserWithTokens(entityRefToken, "attr-tok")

	provErr := &serviceerror.ServiceError{
		Code:             "PROV-OTHER",
		Type:             serviceerror.ClientErrorType,
		ErrorDescription: core.I18nMessage{DefaultValue: "some client error"},
	}
	s.mockProvider.On("GetEntityReference", context.Background(), entityRefToken).
		Return((*authnprovidercm.EntityReference)(nil), provErr)

	_, _, svcErr := s.mgr.GetEntityReference(context.Background(), authUser)
	s.NotNil(svcErr)
	s.Equal(ErrorGetEntityReferenceClientError.Code, svcErr.Code)
	s.Equal("some client error", svcErr.ErrorDescription.DefaultValue)
}

// --- GetUserAvailableAttributes tests ---

func (s *ManagerTestSuite) TestGetUserAvailableAttributes_EmptyAuthUser() {
	attrs, svcErr := s.mgr.GetUserAvailableAttributes(context.Background(), AuthUser{})
	s.Nil(attrs)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestGetUserAvailableAttributes_NilAttributes() {
	authUser := authenticatedAuthUserWithTokens("ref-tok", "attr-tok")

	attrs, svcErr := s.mgr.GetUserAvailableAttributes(context.Background(), authUser)
	s.Nil(svcErr)
	s.NotNil(attrs)
	s.Empty(attrs.Attributes)
	s.Empty(attrs.Verifications)
	s.mockProvider.AssertNotCalled(s.T(), "GetAttributes")
}

// --- GetUserAttributes tests ---

func (s *ManagerTestSuite) TestGetUserAttributes_EmptyAuthUser() {
	_, attrs, svcErr := s.mgr.GetUserAttributes(context.Background(), nil, nil, AuthUser{})
	s.Nil(attrs)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestGetUserAttributes_CacheHit() {
	expectedAttrs := &authnprovidercm.AttributesResponse{
		Attributes: map[string]*authnprovidercm.AttributeResponse{
			"email": {Value: "a@b.com"},
		},
	}
	authUser := authenticatedAuthUserWithResolved(
		&authnprovidercm.EntityReference{EntityID: "user-1"},
		expectedAttrs,
	)

	_, attrs, svcErr := s.mgr.GetUserAttributes(context.Background(), nil, nil, authUser)
	s.Nil(svcErr)
	s.NotNil(attrs)
	s.Equal(expectedAttrs.Attributes, attrs.Attributes)
	s.mockProvider.AssertNotCalled(s.T(), "GetAttributes")
}

func (s *ManagerTestSuite) TestGetUserAttributes_CacheMissServerError() {
	attrToken := "tok"
	authUser := authenticatedAuthUserWithTokens(map[string]interface{}{"userID": "user-1"}, attrToken)

	requestedAttrs := &authnprovidercm.RequestedAttributes{}
	provErr := &serviceerror.ServiceError{
		Code:             "PROVIDER-ERR",
		Type:             serviceerror.ServerErrorType,
		Error:            core.I18nMessage{Key: "error.test.provider_failure", DefaultValue: "provider failure"},
		ErrorDescription: core.I18nMessage{DefaultValue: "server down"},
	}

	s.mockProvider.On("GetAttributes", context.Background(), attrToken, requestedAttrs,
		(*authnprovidercm.GetAttributesMetadata)(nil)).
		Return((*authnprovidercm.AttributesResponse)(nil), provErr)

	_, attrs, svcErr := s.mgr.GetUserAttributes(context.Background(), requestedAttrs, nil, authUser)
	s.Nil(attrs)
	s.NotNil(svcErr)
	s.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
	s.Equal(serviceerror.ServerErrorType, svcErr.Type)
}

func (s *ManagerTestSuite) TestGetUserAttributes_CacheMissClientError() {
	attrToken := "expired-tok"
	authUser := authenticatedAuthUserWithTokens(map[string]interface{}{"userID": "user-1"}, attrToken)

	requestedAttrs := &authnprovidercm.RequestedAttributes{}
	provErr := &serviceerror.ServiceError{
		Code:             "PROVIDER-ERR",
		Type:             serviceerror.ClientErrorType,
		Error:            core.I18nMessage{Key: "error.test.token_expired", DefaultValue: "token expired"},
		ErrorDescription: core.I18nMessage{DefaultValue: "token has expired"},
	}

	s.mockProvider.On("GetAttributes", context.Background(), attrToken, requestedAttrs,
		(*authnprovidercm.GetAttributesMetadata)(nil)).
		Return((*authnprovidercm.AttributesResponse)(nil), provErr)

	_, attrs, svcErr := s.mgr.GetUserAttributes(context.Background(), requestedAttrs, nil, authUser)
	s.Nil(attrs)
	s.NotNil(svcErr)
	s.Equal(ErrorGetAttributesClientError.Code, svcErr.Code)
	s.Equal(serviceerror.ClientErrorType, svcErr.Type)
}

func (s *ManagerTestSuite) TestGetUserAttributes_CacheMiss() {
	attrToken := "tok"
	authUser := authenticatedAuthUserWithTokens(map[string]interface{}{"userID": "user-1"}, attrToken)

	requestedAttrs := &authnprovidercm.RequestedAttributes{}
	fetchedAttrs := &authnprovidercm.AttributesResponse{
		Attributes: map[string]*authnprovidercm.AttributeResponse{
			"email": {Value: "fetched@b.com"},
		},
	}

	s.mockProvider.On("GetAttributes", context.Background(), attrToken, requestedAttrs,
		(*authnprovidercm.GetAttributesMetadata)(nil)).
		Return(fetchedAttrs, (*serviceerror.ServiceError)(nil))

	retAuthUser, attrs, svcErr := s.mgr.GetUserAttributes(context.Background(), requestedAttrs, nil, authUser)
	s.Nil(svcErr)
	s.NotNil(attrs)
	s.Equal(fetchedAttrs.Attributes, attrs.Attributes)
	st := retAuthUser.state[defaultProviderName]
	s.Equal(fetchedAttrs, st.attributes)
	s.Nil(st.attributeToken)
}

func (s *ManagerTestSuite) TestGetUserAvailableAttributes_WithData() {
	expectedAttrs := &authnprovidercm.AttributesResponse{
		Attributes: map[string]*authnprovidercm.AttributeResponse{
			"email": {Value: "a@b.com"},
		},
	}
	authUser := authenticatedAuthUserWithResolved(
		&authnprovidercm.EntityReference{EntityID: "user-1"},
		expectedAttrs,
	)

	attrs, svcErr := s.mgr.GetUserAvailableAttributes(context.Background(), authUser)
	s.Nil(svcErr)
	s.NotNil(attrs)
	s.Equal(expectedAttrs.Attributes, attrs.Attributes)
	s.mockProvider.AssertNotCalled(s.T(), "GetAttributes")
}

// --- Constructor tests ---

func TestNewAuthnProviderManager_NoProviders(t *testing.T) {
	_, err := newAuthnProviderManager(map[string]provider.AuthnProviderInterface{}, nil)
	if err == nil {
		t.Fatalf("expected error when no providers are registered")
	}
}

func TestNewAuthnProviderManager_NilProvider(t *testing.T) {
	_, err := newAuthnProviderManager(
		map[string]provider.AuthnProviderInterface{"default": nil},
		nil,
	)
	if err == nil {
		t.Fatalf("expected error when a provider is nil")
	}
}

func TestNewAuthnProviderManager_UnknownTargetInDefaultMapping(t *testing.T) {
	// Default mapping points the 8 known credential keys at "default"; if "default"
	// is not registered, construction must fail fast.
	mock := providermock.NewAuthnProviderInterfaceMock(t)
	_, err := newAuthnProviderManager(
		map[string]provider.AuthnProviderInterface{"acme": mock},
		nil,
	)
	if err == nil {
		t.Fatalf("expected error when default mapping references an unregistered provider")
	}
}

func TestNewAuthnProviderManager_OverlayUnregisteredTarget(t *testing.T) {
	mock := providermock.NewAuthnProviderInterfaceMock(t)
	_, err := newAuthnProviderManager(
		map[string]provider.AuthnProviderInterface{"default": mock},
		map[string]string{"password": "ghost"},
	)
	if err == nil {
		t.Fatalf("expected error when overlay references an unregistered provider")
	}
}

func TestNewAuthnProviderManager_OverlayRoutesPerProvider(t *testing.T) {
	defaultMock := providermock.NewAuthnProviderInterfaceMock(t)
	acmeMock := providermock.NewAuthnProviderInterfaceMock(t)

	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "secret"}

	// With the overlay, "password" routes to acme — not default.
	acmeMock.On("Authenticate", context.Background(), identifiers, credentials,
		(*authnprovidercm.AuthnMetadata)(nil)).
		Return(&authnprovidercm.AuthnResult{
			EntityReferenceToken: map[string]interface{}{"userID": "user-1"},
			AttributeToken:       map[string]interface{}{"token": "tok"},
		}, (*serviceerror.ServiceError)(nil))

	mgr, err := newAuthnProviderManager(
		map[string]provider.AuthnProviderInterface{
			"default": defaultMock,
			"acme":    acmeMock,
		},
		map[string]string{"password": "acme"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	authUser, _, svcErr := mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, nil, AuthUser{})
	if svcErr != nil {
		t.Fatalf("unexpected service error: %v", svcErr)
	}
	if _, ok := authUser.state[providerName("acme")]; !ok {
		t.Fatalf("expected acme provider to record state in AuthUser")
	}
	defaultMock.AssertNotCalled(t, "Authenticate")
}

func (s *ManagerTestSuite) TestAuthenticateUser_MultipleCredentialKeys() {
	identifiers := map[string]interface{}{"username": "alice"}
	// Two known credential keys; the sorted-first key ("otp") wins.
	credentials := map[string]interface{}{"password": "secret", "otp": "123456"}
	meta := &authnprovidercm.AuthnMetadata{}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return(&authnprovidercm.AuthnResult{
			EntityReferenceToken: map[string]interface{}{"userID": "user-1"},
			AttributeToken:       "tok",
		}, (*serviceerror.ServiceError)(nil))

	authUser, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, AuthUser{})

	s.Nil(svcErr)
	s.True(authUser.IsAuthenticated())
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_NoDefaultProviderState() {
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	// Authenticated, but only under a non-default provider.
	authUser := AuthUser{
		state: map[providerName]authState{
			providerName("acme"): {
				entityReferenceToken: map[string]interface{}{"sub": "ext-sub-1"},
				attributeToken:       "tok",
			},
		},
	}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func TestGetEntityReference_StateForUnregisteredProvider(t *testing.T) {
	mockProvider := providermock.NewAuthnProviderInterfaceMock(t)
	mgr, err := newAuthnProviderManager(
		map[string]provider.AuthnProviderInterface{string(defaultProviderName): mockProvider},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// authUser has state under "ghost" which is not registered in the manager.
	authUser := AuthUser{
		state: map[providerName]authState{
			providerName("ghost"): {
				entityReferenceToken: map[string]interface{}{"userID": "user-1"},
				attributeToken:       "tok",
			},
		},
	}

	_, _, svcErr := mgr.GetEntityReference(context.Background(), authUser)
	if svcErr == nil {
		t.Fatalf("expected service error for state referencing unregistered provider")
	}
	if svcErr.Code != serviceerror.InternalServerError.Code {
		t.Fatalf("expected InternalServerError, got %v", svcErr.Code)
	}
}

func TestGetEntityReference_MultipleProvidersMismatch(t *testing.T) {
	defaultMock := providermock.NewAuthnProviderInterfaceMock(t)
	acmeMock := providermock.NewAuthnProviderInterfaceMock(t)

	defaultMock.On("GetEntityReference", context.Background(),
		map[string]interface{}{"id": "default-tok"}).
		Return(&authnprovidercm.EntityReference{EntityID: "user-1", EntityType: "default", OUID: "ou-1"},
			(*serviceerror.ServiceError)(nil))
	acmeMock.On("GetEntityReference", context.Background(),
		map[string]interface{}{"id": "acme-tok"}).
		Return(&authnprovidercm.EntityReference{EntityID: "user-2", EntityType: "default", OUID: "ou-1"},
			(*serviceerror.ServiceError)(nil))

	mgr, err := newAuthnProviderManager(
		map[string]provider.AuthnProviderInterface{
			"default": defaultMock,
			"acme":    acmeMock,
		},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	authUser := AuthUser{
		state: map[providerName]authState{
			providerName("default"): {
				entityReferenceToken: map[string]interface{}{"id": "default-tok"},
				attributeToken:       "a",
			},
			providerName("acme"): {
				entityReferenceToken: map[string]interface{}{"id": "acme-tok"},
				attributeToken:       "a",
			},
		},
	}

	_, _, svcErr := mgr.GetEntityReference(context.Background(), authUser)
	if svcErr == nil {
		t.Fatalf("expected service error when providers return different entity references")
	}
	if svcErr.Code != serviceerror.InternalServerError.Code {
		t.Fatalf("expected InternalServerError, got %v", svcErr.Code)
	}
}

func TestGetUserAttributes_StateForUnregisteredProvider(t *testing.T) {
	mockProvider := providermock.NewAuthnProviderInterfaceMock(t)
	mgr, err := newAuthnProviderManager(
		map[string]provider.AuthnProviderInterface{string(defaultProviderName): mockProvider},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	authUser := AuthUser{
		state: map[providerName]authState{
			providerName("ghost"): {
				entityReferenceToken: map[string]interface{}{"userID": "user-1"},
				attributeToken:       "tok",
			},
		},
	}

	_, _, svcErr := mgr.GetUserAttributes(context.Background(), nil, nil, authUser)
	if svcErr == nil {
		t.Fatalf("expected service error for state referencing unregistered provider")
	}
	if svcErr.Code != serviceerror.InternalServerError.Code {
		t.Fatalf("expected InternalServerError, got %v", svcErr.Code)
	}
}

func TestIsEntityRefsEqual(t *testing.T) {
	ref := func(id, etype, ou, cat string) *authnprovidercm.EntityReference {
		return &authnprovidercm.EntityReference{
			EntityID: id, EntityType: etype, OUID: ou, EntityCategory: cat,
		}
	}

	cases := []struct {
		name string
		a, b *authnprovidercm.EntityReference
		want bool
	}{
		{"both nil", nil, nil, true},
		{"left nil", nil, ref("1", "t", "o", "c"), false},
		{"right nil", ref("1", "t", "o", "c"), nil, false},
		{"equal", ref("1", "t", "o", "c"), ref("1", "t", "o", "c"), true},
		{"category ignored", ref("1", "t", "o", "c1"), ref("1", "t", "o", "c2"), true},
		{"entityID differs", ref("1", "t", "o", "c"), ref("2", "t", "o", "c"), false},
		{"entityType differs", ref("1", "t1", "o", "c"), ref("1", "t2", "o", "c"), false},
		{"OUID differs", ref("1", "t", "o1", "c"), ref("1", "t", "o2", "c"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isEntityRefsEqual(tc.a, tc.b); got != tc.want {
				t.Errorf("isEntityRefsEqual(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestMergeAttributes_NilSrc(t *testing.T) {
	dst := newAttributesResponse()
	dst.Attributes["existing"] = &authnprovidercm.AttributeResponse{Value: "v"}
	mergeAttributes(dst, nil)
	if len(dst.Attributes) != 1 || dst.Attributes["existing"].Value != "v" {
		t.Fatalf("expected dst to be unchanged when src is nil")
	}
}

func TestMergeAttributes_WithVerifications(t *testing.T) {
	dst := newAttributesResponse()
	src := &authnprovidercm.AttributesResponse{
		Attributes: map[string]*authnprovidercm.AttributeResponse{
			"email": {Value: "a@b.com"},
		},
		Verifications: map[string]*authnprovidercm.VerificationResponse{
			"email": {},
		},
	}
	mergeAttributes(dst, src)
	if _, ok := dst.Attributes["email"]; !ok {
		t.Fatalf("expected merged attribute")
	}
	if _, ok := dst.Verifications["email"]; !ok {
		t.Fatalf("expected merged verification")
	}
}
