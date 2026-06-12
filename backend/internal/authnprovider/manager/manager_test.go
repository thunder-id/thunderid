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
	s.mgr = newAuthnProviderManager(s.mockProvider)
}

// --- helpers to build authenticated AuthUser instances ---

func authenticatedAuthUserWithTokens(entityRefToken any, attrToken any) AuthUser {
	return AuthUser{
		entityReferenceToken: entityRefToken,
		attributeToken:       attrToken,
	}
}

func authenticatedAuthUserWithResolved(entityRef *authnprovidercm.EntityReference,
	attrs *authnprovidercm.AttributesResponse) AuthUser {
	return AuthUser{
		entityReference: entityRef,
		attributes:      attrs,
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
	s.Equal(entityRefToken, returnedAuthUser.entityReferenceToken)
	s.Nil(returnedAuthUser.entityReference)
	s.Equal(attrToken, returnedAuthUser.attributeToken)
	s.Nil(returnedAuthUser.attributes)
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
	s.Nil(returnedAuthUser.entityReferenceToken)
	s.Equal(entityRef, returnedAuthUser.entityReference)
	s.Nil(returnedAuthUser.attributeToken)
	s.Equal(attrs, returnedAuthUser.attributes)
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
	s.Equal("tok-second", au2.attributeToken, "second call must overwrite attribute token")
}

// --- Disambiguation (sub in credentials) tests ---

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_Success() {
	identifiers := map[string]interface{}{"userID": "user-123"}
	credentials := map[string]interface{}{"sub": "ext-sub-1"}

	authUser := AuthUser{
		entityReferenceToken: map[string]interface{}{"sub": "ext-sub-1"},
		attributeToken:       "some-token",
	}

	returnedAuthUser, rtAttrs, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, nil, authUser)

	s.Nil(svcErr)
	s.Nil(rtAttrs)
	s.Equal(map[string]interface{}{"userID": "user-123"}, returnedAuthUser.entityReferenceToken)
	s.Equal(map[string]interface{}{"userID": "user-123"}, returnedAuthUser.attributeToken)
	s.Nil(returnedAuthUser.entityReference)
	s.Nil(returnedAuthUser.attributes)
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
		entityReference: &authnprovidercm.EntityReference{EntityID: "user-1"},
		attributes:      &authnprovidercm.AttributesResponse{},
	}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_NonMapEntityRefToken() {
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := AuthUser{
		entityReferenceToken: "not-a-map",
		attributeToken:       "tok",
	}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_MissingSubInEntityRefToken() {
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := AuthUser{
		entityReferenceToken: map[string]interface{}{"other": "value"},
		attributeToken:       "tok",
	}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_SubMismatch() {
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := AuthUser{
		entityReferenceToken: map[string]interface{}{"sub": "ext-sub-different"},
		attributeToken:       "tok",
	}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_MissingUserID() {
	identifiers := map[string]interface{}{}
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := AuthUser{
		entityReferenceToken: map[string]interface{}{"sub": "ext-sub-1"},
		attributeToken:       "tok",
	}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_NonStringUserID() {
	identifiers := map[string]interface{}{"userID": 12345}
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := AuthUser{
		entityReferenceToken: map[string]interface{}{"sub": "ext-sub-1"},
		attributeToken:       "tok",
	}

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials, nil, nil, authUser)

	s.NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestAuthenticateUser_Disambiguation_EmptyUserID() {
	identifiers := map[string]interface{}{"userID": ""}
	credentials := map[string]interface{}{"sub": "ext-sub-1"}
	authUser := AuthUser{
		entityReferenceToken: map[string]interface{}{"sub": "ext-sub-1"},
		attributeToken:       "tok",
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
	s.Equal(entityRef, retAuthUser.entityReference)
	s.Nil(retAuthUser.entityReferenceToken)
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

	retAuthUser, attrs, svcErr := s.mgr.GetUserAttributes(context.Background(), nil, nil, authUser)
	s.Nil(svcErr)
	s.Equal(expectedAttrs, attrs)
	s.Equal(authUser, retAuthUser)
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
	s.Equal(fetchedAttrs, attrs)
	s.Equal(fetchedAttrs, retAuthUser.attributes)
	s.Nil(retAuthUser.attributeToken)
}
