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

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/suite"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
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
	s.mgr = newAuthnProviderManager(s.mockProvider)
}

// --- helpers to build authenticated AuthUser instances ---

func authenticatedAuthUserWithTokens(entityRefToken any, attrToken any) providers.AuthUser {
	authUser := providers.AuthUser{}
	authUser.SetEntityReferenceToken(entityRefToken)
	authUser.SetAttributeToken(attrToken)
	return authUser
}

func authenticatedAuthUserWithResolved(entityRef *providers.EntityReference,
	attrs *providers.AttributesResponse) providers.AuthUser {
	authUser := providers.AuthUser{}
	authUser.SetEntityReference(entityRef)
	authUser.SetAttributes(attrs)
	return authUser
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
	s.Equal(entityRefToken, returnedAuthUser.EntityReferenceToken())
	s.Nil(returnedAuthUser.EntityReference())
	s.Equal(attrToken, returnedAuthUser.AttributeToken())
	s.Nil(returnedAuthUser.Attributes())
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
	s.Nil(returnedAuthUser.EntityReferenceToken())
	s.Equal(entityRef, returnedAuthUser.EntityReference())
	s.Nil(returnedAuthUser.AttributeToken())
	s.Equal(attrs, returnedAuthUser.Attributes())
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
	s.Equal("tok-second", au2.AttributeToken(), "second call must overwrite attribute token")
}

// --- Disambiguation (sub in credentials) tests ---

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_Success() {
	identifiers := map[string]interface{}{"userID": "user-123"}
	credentials := map[string]interface{}{"sub": "ext-sub-1"}

	authUser := providers.AuthUser{}
	authUser.SetEntityReferenceToken(map[string]interface{}{"sub": "ext-sub-1"})
	authUser.SetAttributeToken("some-token")

	returnedAuthUser, rtAttrs, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, nil, authUser)

	s.Nil(svcErr)
	s.Nil(rtAttrs)
	s.Equal(map[string]interface{}{"userID": "user-123"}, returnedAuthUser.EntityReferenceToken())
	s.Equal(map[string]interface{}{"userID": "user-123"}, returnedAuthUser.AttributeToken())
	s.Nil(returnedAuthUser.EntityReference())
	s.Nil(returnedAuthUser.Attributes())
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
	authUser := providers.AuthUser{}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_NilEntityRefToken() {
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := providers.AuthUser{}
	authUser.SetEntityReference(
		&providers.EntityReference{EntityID: "user-1"})
	authUser.SetAttributes(&providers.AttributesResponse{})

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_NonMapEntityRefToken() {
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := providers.AuthUser{}
	authUser.SetEntityReferenceToken("not-a-map")
	authUser.SetAttributeToken("tok")

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_MissingSubInEntityRefToken() {
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := providers.AuthUser{}
	authUser.SetEntityReferenceToken(map[string]interface{}{"other": "value"})
	authUser.SetAttributeToken("tok")

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_SubMismatch() {
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := providers.AuthUser{}
	authUser.SetEntityReferenceToken(map[string]interface{}{"sub": "ext-sub-different"})
	authUser.SetAttributeToken("tok")

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_MissingUserID() {
	identifiers := map[string]interface{}{}
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := providers.AuthUser{}
	authUser.SetEntityReferenceToken(map[string]interface{}{"sub": "ext-sub-1"})
	authUser.SetAttributeToken("tok")

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_NonStringUserID() {
	identifiers := map[string]interface{}{"userID": 12345}
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := providers.AuthUser{}
	authUser.SetEntityReferenceToken(map[string]interface{}{"sub": "ext-sub-1"})
	authUser.SetAttributeToken("tok")

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_EmptyUserID() {
	identifiers := map[string]interface{}{"userID": ""}
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := providers.AuthUser{}
	authUser.SetEntityReferenceToken(map[string]interface{}{"sub": "ext-sub-1"})
	authUser.SetAttributeToken("tok")

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
	s.Equal(entityRef, retAuthUser.EntityReference())
	s.Nil(retAuthUser.EntityReferenceToken())
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
	s.Equal(expectedAttrs, attrs)
	s.mockProvider.AssertNotCalled(s.T(), "GetAttributes")
}

func (s *ManagerTestSuite) TestGetUserAvailableAttributes_NilAttributes() {
	authUser := authenticatedAuthUserWithTokens("ref-tok", "attr-tok")

	attrs, svcErr := s.mgr.GetUserAvailableAttributes(context.Background(), authUser)
	s.Nil(svcErr)
	s.Nil(attrs)
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

	retAuthUser, attrs, svcErr := s.mgr.GetUserAttributes(context.Background(), nil, nil, authUser)
	s.Nil(svcErr)
	s.Equal(expectedAttrs, attrs)
	s.Equal(authUser, retAuthUser)
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
	s.Equal(fetchedAttrs, attrs)
	s.Equal(fetchedAttrs, retAuthUser.Attributes())
	s.Nil(retAuthUser.AttributeToken())
}
