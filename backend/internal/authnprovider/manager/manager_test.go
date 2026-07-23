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

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/providermock"
)

type ManagerTestSuite struct {
	suite.Suite
	mockProvider *providermock.AuthnProviderInterfaceMock
	mgr          providers.AuthnProviderManager
}

func TestManagerTestSuite(t *testing.T) {
	suite.Run(t, new(ManagerTestSuite))
}

func (s *ManagerTestSuite) SetupTest() {
	s.mockProvider = providermock.NewAuthnProviderInterfaceMock(s.T())
	mgr, err := Initialize(s.mockProvider, nil)
	s.Require().NoError(err)
	s.mgr = mgr
}

// --- helpers to build authenticated AuthUser instances ---

func authUserWithStates(states map[string]providers.AuthState) providers.AuthUser {
	au := providers.AuthUser{}
	for name, st := range states {
		au.SetStateFor(name, st)
	}
	return au
}

func authUserWithDefaultState(st providers.AuthState) providers.AuthUser {
	au := providers.AuthUser{}
	au.SetStateFor(defaultProviderName, st)
	return au
}

func authenticatedAuthUserWithTokens(entityRefToken any, attrToken any) providers.AuthUser {
	return authUserWithDefaultState(providers.AuthState{
		EntityReferenceToken: entityRefToken,
		AttributeToken:       attrToken,
	})
}

func authenticatedAuthUserWithResolved(entityRef *providers.EntityReference,
	attrs *providers.AttributesResponse) providers.AuthUser {
	return authUserWithDefaultState(providers.AuthState{
		EntityReference: entityRef,
		Attributes:      attrs,
	})
}

// --- AuthenticateUser tests ---

func (s *ManagerTestSuite) TestAuthenticateUser_Success_WithTokens() {
	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "secret"}
	meta := &providers.AuthnMetadata{}

	entityRefToken := map[string]interface{}{"userID": "user-1"}
	attrToken := map[string]interface{}{"token": "attr-tok"}
	runtimeAttrs := providers.AuthenticatedClaims{"sessionId": "sess-1"}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return(&providers.AuthnResult{
			EntityReferenceToken: entityRefToken,
			AttributeToken:       attrToken,
			AuthenticatedClaims:  runtimeAttrs,
		}, (*tidcommon.ServiceError)(nil))

	returnedAuthUser, rtAttrs, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, providers.AuthUser{})

	s.Nil(svcErr)
	s.Equal(runtimeAttrs, rtAttrs)
	s.True(returnedAuthUser.IsAuthenticated())
	st, ok := returnedAuthUser.StateFor(defaultProviderName)
	s.True(ok)
	s.Equal(entityRefToken, st.EntityReferenceToken)
	s.Nil(st.EntityReference)
	s.Equal(attrToken, st.AttributeToken)
	s.Nil(st.Attributes)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Success_WithResolvedValues() {
	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "secret"}
	meta := &providers.AuthnMetadata{}

	entityRef := &providers.EntityReference{
		EntityID: "user-1", EntityCategory: "person", EntityType: "default", OUID: "ou-1",
	}
	attrs := &providers.AttributesResponse{
		Attributes: map[string]*providers.AttributeResponse{
			"email": {Value: "alice@example.com"},
		},
	}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return(&providers.AuthnResult{
			EntityReference: entityRef,
			Attributes:      attrs,
		}, (*tidcommon.ServiceError)(nil))

	returnedAuthUser, rtAttrs, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, providers.AuthUser{})

	s.Nil(svcErr)
	s.Nil(rtAttrs)
	s.True(returnedAuthUser.IsAuthenticated())
	st, ok := returnedAuthUser.StateFor(defaultProviderName)
	s.True(ok)
	s.Nil(st.EntityReferenceToken)
	s.Equal(entityRef, st.EntityReference)
	s.Nil(st.AttributeToken)
	s.Equal(attrs, st.Attributes)
}

func (s *ManagerTestSuite) TestAuthenticateUser_EmptyCredentials() {
	identifiers := map[string]interface{}{"username": "alice"}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, map[string]interface{}{},
		nil, nil, providers.AuthUser{})

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
	s.mockProvider.AssertNotCalled(s.T(), "Authenticate")
}

func (s *ManagerTestSuite) TestAuthenticateUser_NilCredentials() {
	identifiers := map[string]interface{}{"username": "alice"}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, nil, nil, nil, providers.AuthUser{})

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
	s.mockProvider.AssertNotCalled(s.T(), "Authenticate")
}

func (s *ManagerTestSuite) TestAuthenticateUser_UnmappedCredentialRoutesToDefault() {
	identifiers := map[string]interface{}{"username": "alice"}
	// A credential key not claimed by any named provider falls through to the default provider.
	credentials := map[string]interface{}{"customCred": "value"}
	meta := &providers.AuthnMetadata{}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return(&providers.AuthnResult{
			EntityReferenceToken: map[string]interface{}{"userID": "user-1"},
			AttributeToken:       "tok",
		}, (*tidcommon.ServiceError)(nil))

	returnedAuthUser, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, providers.AuthUser{})

	s.Nil(svcErr)
	s.True(returnedAuthUser.IsAuthenticated())
	_, ok := returnedAuthUser.StateFor(defaultProviderName)
	s.True(ok)
}

func (s *ManagerTestSuite) TestAuthenticateUser_MissingEntityAndAttributes() {
	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "secret"}
	meta := &providers.AuthnMetadata{}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return(&providers.AuthnResult{}, (*tidcommon.ServiceError)(nil))

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, providers.AuthUser{})

	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_MissingEntityRef() {
	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "secret"}
	meta := &providers.AuthnMetadata{}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return(&providers.AuthnResult{
			AttributeToken: "tok",
		}, (*tidcommon.ServiceError)(nil))

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, providers.AuthUser{})

	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_MissingAttributes() {
	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "secret"}
	meta := &providers.AuthnMetadata{}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return(&providers.AuthnResult{
			EntityReferenceToken: "ref-tok",
		}, (*tidcommon.ServiceError)(nil))

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, providers.AuthUser{})

	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_ClientError() {
	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "wrong"}
	meta := &providers.AuthnMetadata{}
	provErr := &tidcommon.ServiceError{
		Code: "PROV-ERR",
		Type: tidcommon.ClientErrorType,
		Error: tidcommon.I18nMessage{
			Key: "error.test.invalid_credentials", DefaultValue: "invalid credentials",
		},
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "bad creds"},
	}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return((*providers.AuthnResult)(nil), provErr)

	returnedAuthUser, rtAttrs, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, providers.AuthUser{})

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
	s.Equal(tidcommon.ClientErrorType, svcErr.Type)
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
	meta := &providers.AuthnMetadata{}
	provErr := &tidcommon.ServiceError{
		Code: providerErrorCode, Type: tidcommon.ClientErrorType,
		Error:            tidcommon.I18nMessage{DefaultValue: providerError},
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: providerErrorDescription},
	}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return((*providers.AuthnResult)(nil), provErr)

	returnedAuthUser, rtAttrs, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, providers.AuthUser{})

	s.NotNil(svcErr)
	s.Equal(expectedServiceErrorCode, svcErr.Code)
	s.Equal(tidcommon.ClientErrorType, svcErr.Type)
	s.Nil(rtAttrs)
	s.False(returnedAuthUser.IsAuthenticated())
}

func (s *ManagerTestSuite) TestAuthenticateUser_ServerError() {
	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "secret"}
	meta := &providers.AuthnMetadata{}
	provErr := &tidcommon.ServiceError{
		Code: "PROV-ERR",
		Type: tidcommon.ServerErrorType,
		Error: tidcommon.I18nMessage{
			Key: "error.test.database_unavailable", DefaultValue: "database unavailable",
		},
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "db down"},
	}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return((*providers.AuthnResult)(nil), provErr)

	returnedAuthUser, rtAttrs, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, providers.AuthUser{})

	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.Equal(tidcommon.ServerErrorType, svcErr.Type)
	s.Nil(rtAttrs)
	s.False(returnedAuthUser.IsAuthenticated())
}

func (s *ManagerTestSuite) TestAuthenticateUser_ReAuth() {
	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "secret"}
	meta := &providers.AuthnMetadata{}

	firstResult := &providers.AuthnResult{
		EntityReferenceToken: map[string]interface{}{"userID": "user-1"},
		AttributeToken:       "tok-first",
	}
	secondResult := &providers.AuthnResult{
		EntityReferenceToken: map[string]interface{}{"userID": "user-1"},
		AttributeToken:       "tok-second",
	}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return(firstResult, (*tidcommon.ServiceError)(nil)).Once()
	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return(secondResult, (*tidcommon.ServiceError)(nil)).Once()

	au1, _, _ := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials, nil, meta, providers.AuthUser{})
	au2, _, _ := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials, nil, meta, au1)

	s.True(au2.IsAuthenticated())
	st, _ := au2.StateFor(defaultProviderName)
	s.Equal("tok-second", st.AttributeToken, "second call must overwrite attribute token")
}

// --- Disambiguation (sub in credentials) tests ---

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_Success() {
	identifiers := map[string]interface{}{"userID": "user-123"}
	credentials := map[string]interface{}{"sub": "ext-sub-1"}

	authUser := authUserWithDefaultState(providers.AuthState{
		EntityReferenceToken: map[string]interface{}{"sub": "ext-sub-1"},
		AttributeToken:       "some-token",
	})

	returnedAuthUser, rtAttrs, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, nil, authUser)

	s.Nil(svcErr)
	s.Nil(rtAttrs)
	st, ok := returnedAuthUser.StateFor(defaultProviderName)
	s.True(ok)
	s.Equal(map[string]interface{}{"userID": "user-123"}, st.EntityReferenceToken)
	s.Equal(map[string]interface{}{"userID": "user-123"}, st.AttributeToken)
	s.Nil(st.EntityReference)
	s.Nil(st.Attributes)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_EmptySub() {
	credentials := map[string]interface{}{"sub": ""}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, providers.AuthUser{})

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_NonStringSub() {
	credentials := map[string]interface{}{"sub": 123}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, providers.AuthUser{})

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_NotAuthenticated() {
	credentials := map[string]interface{}{"sub": "ext-sub-1"}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, providers.AuthUser{})

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_NilEntityRefToken() {
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := authUserWithDefaultState(providers.AuthState{
		EntityReference: &providers.EntityReference{EntityID: "user-1"},
		Attributes:      &providers.AttributesResponse{},
	})

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_NonMapEntityRefToken() {
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := authUserWithDefaultState(providers.AuthState{
		EntityReferenceToken: "not-a-map",
		AttributeToken:       "tok",
	})

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_MissingSubInEntityRefToken() {
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := authUserWithDefaultState(providers.AuthState{
		EntityReferenceToken: map[string]interface{}{"other": "value"},
		AttributeToken:       "tok",
	})

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_SubMismatch() {
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := authUserWithDefaultState(providers.AuthState{
		EntityReferenceToken: map[string]interface{}{"sub": "ext-sub-different"},
		AttributeToken:       "tok",
	})

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_MissingUserID() {
	identifiers := map[string]interface{}{}
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := authUserWithDefaultState(providers.AuthState{
		EntityReferenceToken: map[string]interface{}{"sub": "ext-sub-1"},
		AttributeToken:       "tok",
	})

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_NonStringUserID() {
	identifiers := map[string]interface{}{"userID": 12345}
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := authUserWithDefaultState(providers.AuthState{
		EntityReferenceToken: map[string]interface{}{"sub": "ext-sub-1"},
		AttributeToken:       "tok",
	})

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_EmptyUserID() {
	identifiers := map[string]interface{}{"userID": ""}
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := authUserWithDefaultState(providers.AuthState{
		EntityReferenceToken: map[string]interface{}{"sub": "ext-sub-1"},
		AttributeToken:       "tok",
	})

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

// --- GetEntityReference tests ---

func (s *ManagerTestSuite) TestGetEntityReference_EmptyAuthUser() {
	_, _, svcErr := s.mgr.GetEntityReference(context.Background(), providers.AuthUser{})
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestGetEntityReference_AlreadyResolved() {
	entityRef := &providers.EntityReference{
		EntityID: "user-1", EntityCategory: "person", EntityType: "default", OUID: "ou-1",
	}
	authUser := authenticatedAuthUserWithResolved(entityRef,
		&providers.AttributesResponse{})

	retAuthUser, retRef, svcErr := s.mgr.GetEntityReference(context.Background(), authUser)
	s.Nil(svcErr)
	s.Equal(entityRef, retRef)
	s.Equal(authUser, retAuthUser)
	s.mockProvider.AssertNotCalled(s.T(), "GetEntityReference")
}

func (s *ManagerTestSuite) TestGetEntityReference_FetchFromProvider() {
	entityRefToken := map[string]interface{}{"userID": "user-1"}
	authUser := authenticatedAuthUserWithTokens(entityRefToken, "attr-tok")

	entityRef := &providers.EntityReference{
		EntityID: "user-1", EntityCategory: "person", EntityType: "default", OUID: "ou-1",
	}
	s.mockProvider.On("GetEntityReference", context.Background(), entityRefToken).
		Return(entityRef, (*tidcommon.ServiceError)(nil))

	retAuthUser, retRef, svcErr := s.mgr.GetEntityReference(context.Background(), authUser)
	s.Nil(svcErr)
	s.Equal(entityRef, retRef)
	st, _ := retAuthUser.StateFor(defaultProviderName)
	s.Equal(entityRef, st.EntityReference)
	s.Nil(st.EntityReferenceToken)
}

func (s *ManagerTestSuite) TestGetEntityReference_ServerError() {
	entityRefToken := map[string]interface{}{"userID": "user-1"}
	authUser := authenticatedAuthUserWithTokens(entityRefToken, "attr-tok")

	provErr := &tidcommon.ServiceError{
		Code:             "PROV-ERR",
		Type:             tidcommon.ServerErrorType,
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "provider failure"},
	}
	s.mockProvider.On("GetEntityReference", context.Background(), entityRefToken).
		Return((*providers.EntityReference)(nil), provErr)

	_, _, svcErr := s.mgr.GetEntityReference(context.Background(), authUser)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestGetEntityReference_UserNotFound() {
	entityRefToken := map[string]interface{}{"userID": "user-1"}
	authUser := authenticatedAuthUserWithTokens(entityRefToken, "attr-tok")

	provErr := &tidcommon.ServiceError{
		Code:             authnprovidercm.ErrorCodeUserNotFound,
		Type:             tidcommon.ClientErrorType,
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "user not found"},
	}
	s.mockProvider.On("GetEntityReference", context.Background(), entityRefToken).
		Return((*providers.EntityReference)(nil), provErr)

	_, _, svcErr := s.mgr.GetEntityReference(context.Background(), authUser)
	s.NotNil(svcErr)
	s.Equal(ErrorUserNotFound.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestGetEntityReference_AmbiguousUser() {
	entityRefToken := map[string]interface{}{"userID": "user-1"}
	authUser := authenticatedAuthUserWithTokens(entityRefToken, "attr-tok")

	provErr := &tidcommon.ServiceError{
		Code:             authnprovidercm.ErrorCodeAmbiguousUser,
		Type:             tidcommon.ClientErrorType,
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "ambiguous user"},
	}
	s.mockProvider.On("GetEntityReference", context.Background(), entityRefToken).
		Return((*providers.EntityReference)(nil), provErr)

	_, _, svcErr := s.mgr.GetEntityReference(context.Background(), authUser)
	s.NotNil(svcErr)
	s.Equal(ErrorAmbiguousUser.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestGetEntityReference_OtherClientError() {
	entityRefToken := map[string]interface{}{"userID": "user-1"}
	authUser := authenticatedAuthUserWithTokens(entityRefToken, "attr-tok")

	provErr := &tidcommon.ServiceError{
		Code:             "PROV-OTHER",
		Type:             tidcommon.ClientErrorType,
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "some client error"},
	}
	s.mockProvider.On("GetEntityReference", context.Background(), entityRefToken).
		Return((*providers.EntityReference)(nil), provErr)

	_, _, svcErr := s.mgr.GetEntityReference(context.Background(), authUser)
	s.NotNil(svcErr)
	s.Equal(ErrorGetEntityReferenceClientError.Code, svcErr.Code)
	s.Equal("some client error", svcErr.ErrorDescription.DefaultValue)
}

// --- GetUserAvailableAttributes tests ---

func (s *ManagerTestSuite) TestGetUserAvailableAttributes_EmptyAuthUser() {
	attrs, svcErr := s.mgr.GetUserAvailableAttributes(context.Background(), providers.AuthUser{})
	s.Nil(attrs)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
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
	_, attrs, svcErr := s.mgr.GetUserAttributes(context.Background(), nil, nil, providers.AuthUser{})
	s.Nil(attrs)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestGetUserAttributes_CacheHit() {
	expectedAttrs := &providers.AttributesResponse{
		Attributes: map[string]*providers.AttributeResponse{
			"email": {Value: "a@b.com"},
		},
	}
	authUser := authenticatedAuthUserWithResolved(
		&providers.EntityReference{EntityID: "user-1"},
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

	requestedAttrs := &providers.RequestedAttributes{}
	provErr := &tidcommon.ServiceError{
		Code:             "PROVIDER-ERR",
		Type:             tidcommon.ServerErrorType,
		Error:            tidcommon.I18nMessage{Key: "error.test.provider_failure", DefaultValue: "provider failure"},
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "server down"},
	}

	s.mockProvider.On("GetAttributes", context.Background(), attrToken, requestedAttrs,
		(*providers.GetAttributesMetadata)(nil)).
		Return((*providers.AttributesResponse)(nil), provErr)

	_, attrs, svcErr := s.mgr.GetUserAttributes(context.Background(), requestedAttrs, nil, authUser)
	s.Nil(attrs)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.Equal(tidcommon.ServerErrorType, svcErr.Type)
}

func (s *ManagerTestSuite) TestGetUserAttributes_CacheMissClientError() {
	attrToken := "expired-tok"
	authUser := authenticatedAuthUserWithTokens(map[string]interface{}{"userID": "user-1"}, attrToken)

	requestedAttrs := &providers.RequestedAttributes{}
	provErr := &tidcommon.ServiceError{
		Code:             "PROVIDER-ERR",
		Type:             tidcommon.ClientErrorType,
		Error:            tidcommon.I18nMessage{Key: "error.test.token_expired", DefaultValue: "token expired"},
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "token has expired"},
	}

	s.mockProvider.On("GetAttributes", context.Background(), attrToken, requestedAttrs,
		(*providers.GetAttributesMetadata)(nil)).
		Return((*providers.AttributesResponse)(nil), provErr)

	_, attrs, svcErr := s.mgr.GetUserAttributes(context.Background(), requestedAttrs, nil, authUser)
	s.Nil(attrs)
	s.NotNil(svcErr)
	s.Equal(ErrorGetAttributesClientError.Code, svcErr.Code)
	s.Equal(tidcommon.ClientErrorType, svcErr.Type)
}

func (s *ManagerTestSuite) TestGetUserAttributes_CacheMiss() {
	attrToken := "tok"
	authUser := authenticatedAuthUserWithTokens(map[string]interface{}{"userID": "user-1"}, attrToken)

	requestedAttrs := &providers.RequestedAttributes{}
	fetchedAttrs := &providers.AttributesResponse{
		Attributes: map[string]*providers.AttributeResponse{
			"email": {Value: "fetched@b.com"},
		},
	}

	s.mockProvider.On("GetAttributes", context.Background(), attrToken, requestedAttrs,
		(*providers.GetAttributesMetadata)(nil)).
		Return(fetchedAttrs, (*tidcommon.ServiceError)(nil))

	retAuthUser, attrs, svcErr := s.mgr.GetUserAttributes(context.Background(), requestedAttrs, nil, authUser)
	s.Nil(svcErr)
	s.NotNil(attrs)
	s.Equal(fetchedAttrs.Attributes, attrs.Attributes)
	st, _ := retAuthUser.StateFor(defaultProviderName)
	s.Equal(fetchedAttrs, st.Attributes)
	s.Nil(st.AttributeToken)
}

func (s *ManagerTestSuite) TestGetUserAvailableAttributes_WithData() {
	expectedAttrs := &providers.AttributesResponse{
		Attributes: map[string]*providers.AttributeResponse{
			"email": {Value: "a@b.com"},
		},
	}
	authUser := authenticatedAuthUserWithResolved(
		&providers.EntityReference{EntityID: "user-1"},
		expectedAttrs,
	)

	attrs, svcErr := s.mgr.GetUserAvailableAttributes(context.Background(), authUser)
	s.Nil(svcErr)
	s.NotNil(attrs)
	s.Equal(expectedAttrs.Attributes, attrs.Attributes)
	s.mockProvider.AssertNotCalled(s.T(), "GetAttributes")
}

// --- Constructor tests ---

func TestNewAuthnProviderManager_NilDefaultProvider(t *testing.T) {
	_, err := Initialize(nil, nil)
	if err == nil {
		t.Fatalf("expected error when the default provider is nil")
	}
}

func TestNewAuthnProviderManager_NilProvider(t *testing.T) {
	defaultMock := providermock.NewAuthnProviderInterfaceMock(t)
	_, err := Initialize(defaultMock, map[string]AuthnProvider{
		"acme": {Instance: nil, Creds: []string{"password"}},
	})
	if err == nil {
		t.Fatalf("expected error when a named provider is nil")
	}
}

func TestNewAuthnProviderManager_ReservedDefaultName(t *testing.T) {
	defaultMock := providermock.NewAuthnProviderInterfaceMock(t)
	acmeMock := providermock.NewAuthnProviderInterfaceMock(t)
	_, err := Initialize(defaultMock, map[string]AuthnProvider{
		defaultProviderName: {Instance: acmeMock, Creds: []string{"password"}},
	})
	if err == nil {
		t.Fatalf("expected error when a named provider uses the reserved default name")
	}
}

func TestNewAuthnProviderManager_DuplicateCredentialClaim(t *testing.T) {
	// Two named providers claiming the same credential key must fail fast.
	defaultMock := providermock.NewAuthnProviderInterfaceMock(t)
	acmeMock := providermock.NewAuthnProviderInterfaceMock(t)
	betaMock := providermock.NewAuthnProviderInterfaceMock(t)
	_, err := Initialize(defaultMock, map[string]AuthnProvider{
		"acme": {Instance: acmeMock, Creds: []string{"password"}},
		"beta": {Instance: betaMock, Creds: []string{"password"}},
	})
	if err == nil {
		t.Fatalf("expected error when two providers claim the same credential key")
	}
}

func TestNewAuthnProviderManager_NamedProviderHandlesClaimedCredential(t *testing.T) {
	defaultMock := providermock.NewAuthnProviderInterfaceMock(t)
	acmeMock := providermock.NewAuthnProviderInterfaceMock(t)

	identifiers := map[string]interface{}{"username": "alice"}
	credentials := map[string]interface{}{"password": "secret"}

	// acme declares it handles "password", so that key routes to acme instead of the default.
	acmeMock.On("Authenticate", context.Background(), identifiers, credentials,
		(*providers.AuthnMetadata)(nil)).
		Return(&providers.AuthnResult{
			EntityReferenceToken: map[string]interface{}{"userID": "user-1"},
			AttributeToken:       map[string]interface{}{"token": "tok"},
		}, (*tidcommon.ServiceError)(nil))

	mgr, err := Initialize(defaultMock, map[string]AuthnProvider{
		"acme": {Instance: acmeMock, Creds: []string{"password"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	authUser, _, svcErr := mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, nil, providers.AuthUser{})
	if svcErr != nil {
		t.Fatalf("unexpected service error: %v", svcErr)
	}
	if _, ok := authUser.StateFor("acme"); !ok {
		t.Fatalf("expected acme provider to record state in AuthUser")
	}
	defaultMock.AssertNotCalled(t, "Authenticate")
}

func (s *ManagerTestSuite) TestAuthenticateUser_MultipleCredentialKeys() {
	identifiers := map[string]interface{}{"username": "alice"}
	// Multiple credential keys make the request ambiguous. Callers must supply exactly one
	// key, so this is treated as an internal fault (server error) rather than a client error.
	credentials := map[string]interface{}{"password": "secret", "otp": "123456"}
	meta := &providers.AuthnMetadata{}

	authUser, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, providers.AuthUser{})

	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.False(authUser.IsAuthenticated())
	s.mockProvider.AssertNotCalled(s.T(), "Authenticate")
}

func (s *ManagerTestSuite) TestInitiateAuthentication_RoutesToDefaultProvider() {
	initData := map[string]interface{}{"relyingPartyId": "example.com"}
	meta := &providers.AuthnMetadata{}
	expected := map[string]interface{}{"challenge": "abc"}

	s.mockProvider.On("InitiateAuthentication", context.Background(), "passkey", initData, meta).
		Return(expected, (*tidcommon.ServiceError)(nil))

	result, svcErr := s.mgr.InitiateAuthentication(context.Background(), "passkey", initData, meta)

	s.Nil(svcErr)
	s.Equal(expected, result)
}

func (s *ManagerTestSuite) TestInitiateAuthentication_ProviderError() {
	provErr := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "X"}
	s.mockProvider.On("InitiateAuthentication", context.Background(), "passkey", mock.Anything, mock.Anything).
		Return(nil, provErr)

	result, svcErr := s.mgr.InitiateAuthentication(context.Background(), "passkey", nil, nil)

	s.Nil(result)
	s.Equal(provErr, svcErr)
}

func (s *ManagerTestSuite) TestInitiateEnrollment_RoutesToDefaultProvider() {
	initData := map[string]interface{}{"userId": "user-1"}
	expected := map[string]interface{}{"creationOptions": "xyz"}

	s.mockProvider.On("InitiateEnrollment", context.Background(), "passkey", initData, mock.Anything).
		Return(expected, (*tidcommon.ServiceError)(nil))

	result, svcErr := s.mgr.InitiateEnrollment(context.Background(), "passkey", initData, nil)

	s.Nil(svcErr)
	s.Equal(expected, result)
}

func (s *ManagerTestSuite) TestEnroll_Success() {
	credentials := map[string]interface{}{"passkey": "cred"}
	meta := &providers.AuthnMetadata{}
	entityRefToken := map[string]interface{}{"userID": "user-1"}
	claims := providers.AuthenticatedClaims{"userID": "user-1"}

	s.mockProvider.On("Enroll", context.Background(), map[string]interface{}(nil), credentials, meta).
		Return(&providers.AuthnResult{
			EntityReferenceToken: entityRefToken,
			AttributeToken:       entityRefToken,
			AuthenticatedClaims:  claims,
		}, (*tidcommon.ServiceError)(nil))

	authUser, rtClaims, svcErr := s.mgr.Enroll(context.Background(), nil, credentials, nil, meta, providers.AuthUser{})

	s.Nil(svcErr)
	s.Equal(claims, rtClaims)
	s.True(authUser.IsAuthenticated())
	st, ok := authUser.StateFor(defaultProviderName)
	s.True(ok)
	s.Equal(entityRefToken, st.EntityReferenceToken)
}

func (s *ManagerTestSuite) TestEnroll_ServerError() {
	credentials := map[string]interface{}{"passkey": "cred"}
	s.mockProvider.On("Enroll", context.Background(), mock.Anything, credentials, mock.Anything).
		Return(nil, &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "boom"})

	authUser, _, svcErr := s.mgr.Enroll(context.Background(), nil, credentials, nil, nil, providers.AuthUser{})

	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.False(authUser.IsAuthenticated())
}

func (s *ManagerTestSuite) TestEnroll_ClientErrorMapping() {
	cases := []struct {
		providerCode string
		expectedCode string
	}{
		{authnprovidercm.ErrorCodeUserNotFound, ErrorUserNotFound.Code},
		{authnprovidercm.ErrorCodeInvalidRequest, ErrorInvalidRequest.Code},
		{authnprovidercm.ErrorCodeEnrollmentFailed, ErrorEnrollmentFailed.Code},
		{"SOMETHING_ELSE", ErrorEnrollmentFailed.Code},
	}
	for _, tc := range cases {
		s.SetupTest()
		credentials := map[string]interface{}{"passkey": "cred"}
		s.mockProvider.On("Enroll", context.Background(), mock.Anything, credentials, mock.Anything).
			Return(nil, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: tc.providerCode})

		_, _, svcErr := s.mgr.Enroll(context.Background(), nil, credentials, nil, nil, providers.AuthUser{})

		s.NotNil(svcErr)
		s.Equal(tc.expectedCode, svcErr.Code, "provider code %s", tc.providerCode)
	}
}

func (s *ManagerTestSuite) TestEnroll_EmptyCredentials() {
	// Empty credentials must not panic (selectProvider guards len != 1) and is a server error.
	authUser, _, svcErr := s.mgr.Enroll(context.Background(), nil, map[string]interface{}{},
		nil, nil, providers.AuthUser{})

	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.False(authUser.IsAuthenticated())
	s.mockProvider.AssertNotCalled(s.T(), "Enroll")
}

func (s *ManagerTestSuite) TestEnroll_MultipleCredentialKeys() {
	credentials := map[string]interface{}{"passkey": "cred", "otp": "123456"}
	authUser, _, svcErr := s.mgr.Enroll(context.Background(), nil, credentials, nil, nil, providers.AuthUser{})

	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.False(authUser.IsAuthenticated())
	s.mockProvider.AssertNotCalled(s.T(), "Enroll")
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_NoDefaultProviderState() {
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	// Authenticated, but only under a non-default provider.
	authUser := authUserWithStates(map[string]providers.AuthState{
		"acme": {
			EntityReferenceToken: map[string]interface{}{"sub": "ext-sub-1"},
			AttributeToken:       "tok",
		},
	})

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func TestGetEntityReference_StateForUnregisteredProvider(t *testing.T) {
	mockProvider := providermock.NewAuthnProviderInterfaceMock(t)
	mgr, err := Initialize(mockProvider, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// authUser has state under "ghost" which is not registered in the manager.
	authUser := authUserWithStates(map[string]providers.AuthState{
		"ghost": {
			EntityReferenceToken: map[string]interface{}{"userID": "user-1"},
			AttributeToken:       "tok",
		},
	})

	_, _, svcErr := mgr.GetEntityReference(context.Background(), authUser)
	if svcErr == nil {
		t.Fatalf("expected service error for state referencing unregistered provider")
	}
	if svcErr.Code != tidcommon.InternalServerError.Code {
		t.Fatalf("expected InternalServerError, got %v", svcErr.Code)
	}
}

func TestGetEntityReference_MultipleProvidersMismatch(t *testing.T) {
	defaultMock := providermock.NewAuthnProviderInterfaceMock(t)
	acmeMock := providermock.NewAuthnProviderInterfaceMock(t)

	defaultMock.On("GetEntityReference", context.Background(),
		map[string]interface{}{"id": "default-tok"}).
		Return(&providers.EntityReference{EntityID: "user-1", EntityType: "default", OUID: "ou-1"},
			(*tidcommon.ServiceError)(nil))
	acmeMock.On("GetEntityReference", context.Background(),
		map[string]interface{}{"id": "acme-tok"}).
		Return(&providers.EntityReference{EntityID: "user-2", EntityType: "default", OUID: "ou-1"},
			(*tidcommon.ServiceError)(nil))

	mgr, err := Initialize(defaultMock, map[string]AuthnProvider{
		"acme": {Instance: acmeMock, Creds: nil},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	authUser := authUserWithStates(map[string]providers.AuthState{
		"default": {
			EntityReferenceToken: map[string]interface{}{"id": "default-tok"},
			AttributeToken:       "a",
		},
		"acme": {
			EntityReferenceToken: map[string]interface{}{"id": "acme-tok"},
			AttributeToken:       "a",
		},
	})

	_, _, svcErr := mgr.GetEntityReference(context.Background(), authUser)
	if svcErr == nil {
		t.Fatalf("expected service error when providers return different entity references")
	}
	if svcErr.Code != tidcommon.InternalServerError.Code {
		t.Fatalf("expected InternalServerError, got %v", svcErr.Code)
	}
}

func TestGetUserAttributes_StateForUnregisteredProvider(t *testing.T) {
	mockProvider := providermock.NewAuthnProviderInterfaceMock(t)
	mgr, err := Initialize(mockProvider, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	authUser := authUserWithStates(map[string]providers.AuthState{
		"ghost": {
			EntityReferenceToken: map[string]interface{}{"userID": "user-1"},
			AttributeToken:       "tok",
		},
	})

	_, _, svcErr := mgr.GetUserAttributes(context.Background(), nil, nil, authUser)
	if svcErr == nil {
		t.Fatalf("expected service error for state referencing unregistered provider")
	}
	if svcErr.Code != tidcommon.InternalServerError.Code {
		t.Fatalf("expected InternalServerError, got %v", svcErr.Code)
	}
}

func TestIsEntityRefsEqual(t *testing.T) {
	ref := func(id, etype, ou, cat string) *providers.EntityReference {
		return &providers.EntityReference{
			EntityID: id, EntityType: etype, OUID: ou, EntityCategory: cat,
		}
	}

	cases := []struct {
		name string
		a, b *providers.EntityReference
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
	dst.Attributes["existing"] = &providers.AttributeResponse{Value: "v"}
	mergeAttributes(dst, nil)
	if len(dst.Attributes) != 1 || dst.Attributes["existing"].Value != "v" {
		t.Fatalf("expected dst to be unchanged when src is nil")
	}
}

func TestMergeAttributes_WithVerifications(t *testing.T) {
	dst := newAttributesResponse()
	src := &providers.AttributesResponse{
		Attributes: map[string]*providers.AttributeResponse{
			"email": {Value: "a@b.com"},
		},
		Verifications: map[string]*providers.VerificationResponse{
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
