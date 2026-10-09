// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

func (s *ManagerTestSuite) TestAuthenticateUser_ResolvedAgentRejectedByConstraints() {
	credentials := map[string]interface{}{"password": "secret"}
	meta := &providers.AuthnMetadata{}
	ctx := authnprovidercm.WithSubjectTypeConstraints(context.Background(),
		authnprovidercm.SubjectTypeConstraints{AllowedUserTypes: []string{"customer"}})

	s.mockProvider.On("Authenticate", ctx, mock.Anything, credentials, meta).
		Return(&providers.AuthnResult{
			EntityReference: &providers.EntityReference{
				EntityID: "agent-1", EntityCategory: "agent", EntityType: "default", OUID: "ou-1",
			},
			Attributes: &providers.AttributesResponse{},
		}, (*tidcommon.ServiceError)(nil))

	returnedAuthUser, rtAttrs, svcErr := s.mgr.AuthenticateUser(ctx, nil, credentials,
		nil, meta, providers.AuthUser{})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorSubjectNotAllowed.Code, svcErr.Code)
	s.Nil(rtAttrs)
	s.False(returnedAuthUser.IsAuthenticated())
}

func (s *ManagerTestSuite) TestAuthenticateUser_ResolvedAgentAllowedByConstraints() {
	credentials := map[string]interface{}{"password": "secret"}
	meta := &providers.AuthnMetadata{}
	entityRef := &providers.EntityReference{
		EntityID: "agent-1", EntityCategory: "agent", EntityType: "default", OUID: "ou-1",
	}
	ctx := authnprovidercm.WithSubjectTypeConstraints(context.Background(),
		authnprovidercm.SubjectTypeConstraints{AllowedAgentTypes: []string{"default"}})

	s.mockProvider.On("Authenticate", ctx, mock.Anything, credentials, meta).
		Return(&providers.AuthnResult{
			EntityReference: entityRef,
			Attributes:      &providers.AttributesResponse{},
		}, (*tidcommon.ServiceError)(nil))

	returnedAuthUser, _, svcErr := s.mgr.AuthenticateUser(ctx, nil, credentials,
		nil, meta, providers.AuthUser{})

	s.Nil(svcErr)
	s.True(returnedAuthUser.IsAuthenticated())
	st, ok := returnedAuthUser.StateFor(defaultProviderName)
	s.True(ok)
	s.Equal(entityRef, st.EntityReference)
}

// A provider that has not resolved the subject yet returns an entity reference token instead of a
// reference, so there is no category or type to check. GetEntityReference applies the check later.
func (s *ManagerTestSuite) TestAuthenticateUser_UnresolvedSubjectDefersConstraintCheck() {
	credentials := map[string]interface{}{"password": "secret"}
	meta := &providers.AuthnMetadata{}
	entityRefToken := map[string]interface{}{"sub": "agent-1"}
	ctx := authnprovidercm.WithSubjectTypeConstraints(context.Background(),
		authnprovidercm.SubjectTypeConstraints{})

	s.mockProvider.On("Authenticate", ctx, mock.Anything, credentials, meta).
		Return(&providers.AuthnResult{
			EntityReferenceToken: entityRefToken,
			AttributeToken:       entityRefToken,
		}, (*tidcommon.ServiceError)(nil))

	returnedAuthUser, _, svcErr := s.mgr.AuthenticateUser(ctx, nil, credentials,
		nil, meta, providers.AuthUser{})

	s.Nil(svcErr)
	s.True(returnedAuthUser.IsAuthenticated())
}

// Entry points that are not scoped to an application (the credentials authentication API) carry no
// constraints, so the check is skipped rather than defaulting to deny.
func (s *ManagerTestSuite) TestAuthenticateUser_AgentAllowedWhenNoConstraintsOnContext() {
	credentials := map[string]interface{}{"password": "secret"}
	meta := &providers.AuthnMetadata{}

	s.mockProvider.On("Authenticate", context.Background(), mock.Anything, credentials, meta).
		Return(&providers.AuthnResult{
			EntityReference: &providers.EntityReference{
				EntityID: "agent-1", EntityCategory: "agent", EntityType: "default", OUID: "ou-1",
			},
			Attributes: &providers.AttributesResponse{},
		}, (*tidcommon.ServiceError)(nil))

	returnedAuthUser, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials,
		nil, meta, providers.AuthUser{})

	s.Nil(svcErr)
	s.True(returnedAuthUser.IsAuthenticated())
}

// Two entities holding one recorded link is reported as ambiguity, not as a failed exchange.
func (s *ManagerTestSuite) TestAuthenticateUser_FederatedAmbiguousLink() {
	credentials := map[string]interface{}{authnprovidercm.CredentialTypeFederated: "code"}
	meta := &providers.AuthnMetadata{}
	s.mockProvider.On("Authenticate", context.Background(), map[string]interface{}(nil), credentials, meta).
		Return((*providers.AuthnResult)(nil), &tidcommon.ServiceError{
			Code: authnprovidercm.ErrorCodeAmbiguousUser, Type: tidcommon.ClientErrorType,
		})

	_, _, svcErr := s.mgr.AuthenticateUser(context.Background(), nil, credentials,
		nil, meta, providers.AuthUser{})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorAmbiguousUser.Code, svcErr.Code)
}

// Other credentials keep reading an ambiguous result as a failed authentication.
func (s *ManagerTestSuite) TestAuthenticateUser_NonFederatedAmbiguousIsAuthenticationFailure() {
	s.assertAuthenticateUserClientErrorMapping(
		authnprovidercm.ErrorCodeAmbiguousUser,
		"ambiguous user",
		"several users match",
		ErrorAuthenticationFailed.Code,
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

func (s *ManagerTestSuite) TestGetEntityReference_ResolvedAgentRejectedByConstraints() {
	// An SSO checkpoint replays an already-resolved agent reference into an application that allows
	// no agent type, so the manager rejects it without consulting the provider.
	entityRef := &providers.EntityReference{
		EntityID: "agent-1", EntityCategory: "agent", EntityType: "default", OUID: "ou-1",
	}
	authUser := authenticatedAuthUserWithResolved(entityRef, &providers.AttributesResponse{})
	ctx := authnprovidercm.WithSubjectTypeConstraints(context.Background(),
		authnprovidercm.SubjectTypeConstraints{AllowedUserTypes: []string{"customer"}})

	_, retRef, svcErr := s.mgr.GetEntityReference(ctx, authUser)

	s.Nil(retRef)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorSubjectNotAllowed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestGetEntityReference_ResolvedAgentAllowedByConstraints() {
	entityRef := &providers.EntityReference{
		EntityID: "agent-1", EntityCategory: "agent", EntityType: "default", OUID: "ou-1",
	}
	authUser := authenticatedAuthUserWithResolved(entityRef, &providers.AttributesResponse{})
	ctx := authnprovidercm.WithSubjectTypeConstraints(context.Background(),
		authnprovidercm.SubjectTypeConstraints{AllowedAgentTypes: []string{"default"}})

	_, retRef, svcErr := s.mgr.GetEntityReference(ctx, authUser)

	s.Nil(svcErr)
	s.Equal(entityRef, retRef)
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

// A pending federated identity at one provider next to a verified user at another resolves nobody,
// so a verification that authenticates through a different provider cannot settle a link.
func (s *ManagerTestSuite) TestGetEntityReference_PendingStateAtAnotherProviderResolvesNobody() {
	custom := providermock.NewAuthnProviderInterfaceMock(s.T())
	mgr, err := Initialize(s.mockProvider, map[string]providers.CustomAuthnProvider{
		"corp": {Instance: custom, Creds: []string{"password"}},
	})
	s.Require().NoError(err)

	pending := map[string]interface{}{"federatedIdpId": "idp-a", "sub": "sub-1"}
	s.mockProvider.On("GetEntityReference", mock.Anything, pending).
		Return((*providers.EntityReference)(nil), &tidcommon.ServiceError{
			Code: authnprovidercm.ErrorCodeUserNotFound, Type: tidcommon.ClientErrorType,
		}).Maybe()
	custom.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(&providers.EntityReference{EntityID: "corp-user"}, (*tidcommon.ServiceError)(nil)).Maybe()

	au := authUserWithStates(map[string]providers.AuthState{
		defaultProviderName: {EntityReferenceToken: pending, AttributeToken: pending},
		"corp": {
			EntityReferenceToken: map[string]interface{}{"userID": "corp-user"},
			AttributeToken:       map[string]interface{}{"userID": "corp-user"},
		},
	})

	_, ref, svcErr := mgr.GetEntityReference(context.Background(), au)
	s.Nil(ref)
	s.Require().NotNil(svcErr)
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
	_, err := Initialize(defaultMock, map[string]providers.CustomAuthnProvider{
		"acme": {Instance: nil, Creds: []string{"password"}},
	})
	if err == nil {
		t.Fatalf("expected error when a named provider is nil")
	}
}

func TestNewAuthnProviderManager_ReservedDefaultName(t *testing.T) {
	defaultMock := providermock.NewAuthnProviderInterfaceMock(t)
	acmeMock := providermock.NewAuthnProviderInterfaceMock(t)
	_, err := Initialize(defaultMock, map[string]providers.CustomAuthnProvider{
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
	_, err := Initialize(defaultMock, map[string]providers.CustomAuthnProvider{
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

	mgr, err := Initialize(defaultMock, map[string]providers.CustomAuthnProvider{
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

func (s *ManagerTestSuite) TestAuthenticateUser_MultipleCredentialKeysSameProvider() {
	identifiers := map[string]interface{}{"username": "alice"}
	// Multiple credential keys that all resolve to the same provider (here the default, since no
	// custom provider claims them) are routed to that provider.
	credentials := map[string]interface{}{"password": "secret", "otp": "123456"}
	meta := &providers.AuthnMetadata{}

	s.mockProvider.On("Authenticate", context.Background(), identifiers, credentials, meta).
		Return(&providers.AuthnResult{
			EntityReferenceToken: map[string]interface{}{"userID": "user-1"},
			AttributeToken:       "tok",
		}, (*tidcommon.ServiceError)(nil))

	authUser, _, svcErr := s.mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, providers.AuthUser{})

	s.Nil(svcErr)
	s.True(authUser.IsAuthenticated())
	_, ok := authUser.StateFor(defaultProviderName)
	s.True(ok)
}

func TestAuthenticateUser_MultipleCredentialKeysDifferentProviders(t *testing.T) {
	identifiers := map[string]interface{}{"username": "alice"}
	// "password" falls through to the default provider while "otp" is claimed by acme, so the keys
	// fan out to different providers. That is ambiguous and treated as an internal fault.
	credentials := map[string]interface{}{"password": "secret", "otp": "123456"}
	meta := &providers.AuthnMetadata{}

	defaultMock := providermock.NewAuthnProviderInterfaceMock(t)
	acmeMock := providermock.NewAuthnProviderInterfaceMock(t)
	mgr, err := Initialize(defaultMock, map[string]providers.CustomAuthnProvider{
		"acme": {Instance: acmeMock, Creds: []string{"otp"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	authUser, _, svcErr := mgr.AuthenticateUser(context.Background(), identifiers, credentials,
		nil, meta, providers.AuthUser{})

	if svcErr == nil || svcErr.Code != tidcommon.InternalServerError.Code {
		t.Fatalf("expected InternalServerError for keys mapping to different providers, got %v", svcErr)
	}
	if authUser.IsAuthenticated() {
		t.Fatalf("expected authUser to remain unauthenticated")
	}
	defaultMock.AssertNotCalled(t, "Authenticate")
	acmeMock.AssertNotCalled(t, "Authenticate")
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

func (s *ManagerTestSuite) TestEnroll_ResolvedAgentRejectedByConstraints() {
	credentials := map[string]interface{}{"passkey": "cred"}
	meta := &providers.AuthnMetadata{}
	ctx := authnprovidercm.WithSubjectTypeConstraints(context.Background(),
		authnprovidercm.SubjectTypeConstraints{AllowedUserTypes: []string{"customer"}})

	s.mockProvider.On("Enroll", ctx, map[string]interface{}(nil), credentials, meta).
		Return(&providers.AuthnResult{
			EntityReference: &providers.EntityReference{
				EntityID: "agent-1", EntityCategory: "agent", EntityType: "default", OUID: "ou-1",
			},
			Attributes: &providers.AttributesResponse{},
		}, (*tidcommon.ServiceError)(nil))

	authUser, _, svcErr := s.mgr.Enroll(ctx, nil, credentials, nil, meta, providers.AuthUser{})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorSubjectNotAllowed.Code, svcErr.Code)
	s.False(authUser.IsAuthenticated())
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
	// Empty credentials must not panic (selectProvider guards len == 0) and is a server error.
	authUser, _, svcErr := s.mgr.Enroll(context.Background(), nil, map[string]interface{}{},
		nil, nil, providers.AuthUser{})

	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.False(authUser.IsAuthenticated())
	s.mockProvider.AssertNotCalled(s.T(), "Enroll")
}

func (s *ManagerTestSuite) TestEnroll_MultipleCredentialKeysSameProvider() {
	// Multiple credential keys that all resolve to the same provider (here the default) are routed
	// to that provider.
	credentials := map[string]interface{}{"passkey": "cred", "otp": "123456"}
	meta := &providers.AuthnMetadata{}
	entityRefToken := map[string]interface{}{"userID": "user-1"}

	s.mockProvider.On("Enroll", context.Background(), map[string]interface{}(nil), credentials, meta).
		Return(&providers.AuthnResult{
			EntityReferenceToken: entityRefToken,
			AttributeToken:       entityRefToken,
		}, (*tidcommon.ServiceError)(nil))

	authUser, _, svcErr := s.mgr.Enroll(context.Background(), nil, credentials, nil, meta, providers.AuthUser{})

	s.Nil(svcErr)
	s.True(authUser.IsAuthenticated())
	_, ok := authUser.StateFor(defaultProviderName)
	s.True(ok)
}

func TestEnroll_MultipleCredentialKeysDifferentProviders(t *testing.T) {
	// "passkey" falls through to the default provider while "otp" is claimed by acme, so the keys
	// fan out to different providers. That is ambiguous and treated as an internal fault.
	credentials := map[string]interface{}{"passkey": "cred", "otp": "123456"}

	defaultMock := providermock.NewAuthnProviderInterfaceMock(t)
	acmeMock := providermock.NewAuthnProviderInterfaceMock(t)
	mgr, err := Initialize(defaultMock, map[string]providers.CustomAuthnProvider{
		"acme": {Instance: acmeMock, Creds: []string{"otp"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	authUser, _, svcErr := mgr.Enroll(context.Background(), nil, credentials, nil, nil, providers.AuthUser{})

	if svcErr == nil || svcErr.Code != tidcommon.InternalServerError.Code {
		t.Fatalf("expected InternalServerError for keys mapping to different providers, got %v", svcErr)
	}
	if authUser.IsAuthenticated() {
		t.Fatalf("expected authUser to remain unauthenticated")
	}
	defaultMock.AssertNotCalled(t, "Enroll")
	acmeMock.AssertNotCalled(t, "Enroll")
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

	mgr, err := Initialize(defaultMock, map[string]providers.CustomAuthnProvider{
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

// --- LinkAccount ---

func (s *ManagerTestSuite) TestLinkAccount_RoutesToProviderHoldingTheUser() {
	token := map[string]interface{}{"userID": "user-1"}
	s.mockProvider.On("StoreAccountLink", mock.Anything, token, "idp-a", "sub-1").
		Return((*tidcommon.ServiceError)(nil))

	svcErr := s.mgr.LinkAccount(context.Background(),
		authenticatedAuthUserWithTokens(token, token), "idp-a", "sub-1")
	s.Nil(svcErr)
}

// GetEntityReference swaps a resolved token for the reference itself, so by the time a flow reaches
// the link write the only handle left is the entity id.
func (s *ManagerTestSuite) TestLinkAccount_BuildsTokenFromResolvedReference() {
	ref := &providers.EntityReference{EntityID: "user-2", EntityCategory: "user", EntityType: "employee"}
	s.mockProvider.On("StoreAccountLink", mock.Anything,
		map[string]interface{}{"userID": "user-2"}, "idp-a", "sub-1").
		Return((*tidcommon.ServiceError)(nil))

	au := authUserWithDefaultState(providers.AuthState{
		EntityReference: ref,
		Attributes:      &providers.AttributesResponse{},
	})
	s.Nil(s.mgr.LinkAccount(context.Background(), au, "idp-a", "sub-1"))
}

// A provider that does not store links has nothing to do, and failing the sign-in over it would
// break every sign-in through that provider that needs a link.
func (s *ManagerTestSuite) TestLinkAccount_UnsupportedIsSuccess() {
	token := map[string]interface{}{"userID": "user-3"}
	s.mockProvider.On("StoreAccountLink", mock.Anything, token, "idp-a", "sub-1").
		Return(&tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType,
			Code: authnprovidercm.ErrorCodeNotImplemented,
		})

	s.Nil(s.mgr.LinkAccount(context.Background(),
		authenticatedAuthUserWithTokens(token, token), "idp-a", "sub-1"))
}

// A malformed request is a real failure, so it must not be read as "links not stored" and silently
// drop the link.
func (s *ManagerTestSuite) TestLinkAccount_InvalidRequestIsSurfaced() {
	token := map[string]interface{}{"userID": "user-3b"}
	s.mockProvider.On("StoreAccountLink", mock.Anything, token, "idp-a", "sub-1").
		Return(&tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType,
			Code: authnprovidercm.ErrorCodeInvalidRequest,
		})

	svcErr := s.mgr.LinkAccount(context.Background(),
		authenticatedAuthUserWithTokens(token, token), "idp-a", "sub-1")
	s.Require().NotNil(svcErr)
	s.Equal(ErrorLinkAccountFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestLinkAccount_ClientErrorIsSurfaced() {
	token := map[string]interface{}{"userID": "user-4"}
	s.mockProvider.On("StoreAccountLink", mock.Anything, token, "idp-a", "sub-1").
		Return(&tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType,
			Code: authnprovidercm.ErrorCodeUserNotFound,
		})

	svcErr := s.mgr.LinkAccount(context.Background(),
		authenticatedAuthUserWithTokens(token, token), "idp-a", "sub-1")
	s.Require().NotNil(svcErr)
	s.Equal(ErrorLinkAccountFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestLinkAccount_ServerErrorBecomesInternal() {
	token := map[string]interface{}{"userID": "user-5"}
	s.mockProvider.On("StoreAccountLink", mock.Anything, token, "idp-a", "sub-1").
		Return(&tidcommon.ServiceError{Type: tidcommon.ServerErrorType})

	svcErr := s.mgr.LinkAccount(context.Background(),
		authenticatedAuthUserWithTokens(token, token), "idp-a", "sub-1")
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestLinkAccount_RejectsEmptyIdentity() {
	token := map[string]interface{}{"userID": "user-6"}
	au := authenticatedAuthUserWithTokens(token, token)

	s.Equal(ErrorInvalidRequest.Code,
		s.mgr.LinkAccount(context.Background(), au, "", "sub-1").Code)
	s.Equal(ErrorInvalidRequest.Code,
		s.mgr.LinkAccount(context.Background(), au, "idp-a", "").Code)
	s.mockProvider.AssertNotCalled(s.T(), "StoreAccountLink",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ManagerTestSuite) TestLinkAccount_RejectsAuthUserWithoutState() {
	svcErr := s.mgr.LinkAccount(context.Background(), providers.AuthUser{}, "idp-a", "sub-1")
	s.Require().NotNil(svcErr)
	s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
}

func (s *ManagerTestSuite) TestLinkAccount_RejectsStateWithoutEntity() {
	au := authUserWithDefaultState(providers.AuthState{Attributes: &providers.AttributesResponse{}})
	svcErr := s.mgr.LinkAccount(context.Background(), au, "idp-a", "sub-1")
	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidRequest.Code, svcErr.Code)
}

// A verified flow authenticates federated first and by password second, so the AuthUser carries two
// provider states. The link belongs to whichever provider claimed the first credential.
func (s *ManagerTestSuite) TestLinkAccount_PrefersTheFirstCredentialProvider() {
	custom := providermock.NewAuthnProviderInterfaceMock(s.T())
	mgr, err := Initialize(s.mockProvider, map[string]providers.CustomAuthnProvider{
		"corp": {Instance: custom, Creds: []string{authnprovidercm.CredentialTypeFederated}},
	})
	s.Require().NoError(err)

	corpToken := map[string]interface{}{"userID": "corp-user"}
	custom.On("StoreAccountLink", mock.Anything, corpToken, "idp-a", "sub-1").
		Return((*tidcommon.ServiceError)(nil))

	au := authUserWithStates(map[string]providers.AuthState{
		defaultProviderName: {
			EntityReferenceToken: map[string]interface{}{"userID": "local-user"},
			AttributeToken:       map[string]interface{}{"userID": "local-user"},
		},
		"corp": {EntityReferenceToken: corpToken, AttributeToken: corpToken},
	})

	s.Nil(mgr.LinkAccount(context.Background(), au, "idp-a", "sub-1"))
	s.mockProvider.AssertNotCalled(s.T(), "StoreAccountLink",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// --- ResolveLinkCandidates tests ---

// pendingFederatedAuthUser returns an AuthUser carrying an unresolved federated identity with the
// given account-linking filters.
func pendingFederatedAuthUser(filters any) providers.AuthUser {
	token := map[string]interface{}{"federatedIdpId": "idp-1", "sub": "sub-1"}
	if filters != nil {
		token[authnprovidercm.AccountLinkingFiltersKey] = filters
	}
	return authenticatedAuthUserWithTokens(token, token)
}

var errUserNotFound = &tidcommon.ServiceError{
	Type: tidcommon.ClientErrorType, Code: authnprovidercm.ErrorCodeUserNotFound}

// The provider resolves each filter the way it resolves any other attribute lookup.
func (s *ManagerTestSuite) TestResolveLinkCandidates_MatchesOnLinkingAttributes() {
	authUser := pendingFederatedAuthUser([]map[string]interface{}{{"email": "jane@example.com"}})
	s.mockProvider.On("GetEntityReference", context.Background(),
		map[string]interface{}{"email": "jane@example.com"}).
		Return(&providers.EntityReference{EntityID: "user-1"}, (*tidcommon.ServiceError)(nil))

	candidates, svcErr := s.mgr.ResolveLinkCandidates(context.Background(), authUser)

	s.Nil(svcErr)
	if s.NotNil(candidates) {
		s.Equal([]string{"user-1"}, candidates.EntityIDs)
		s.Equal(map[string]string{"email": "jane@example.com"}, candidates.MatchedAttributes)
	}
}

// Any one filter may name the user, so a miss on the first still tries the next.
func (s *ManagerTestSuite) TestResolveLinkCandidates_LaterFilterMatches() {
	authUser := pendingFederatedAuthUser([]map[string]interface{}{
		{"username": "jane@example.com"}, {"email": "jane@example.com"}})
	s.mockProvider.On("GetEntityReference", context.Background(),
		map[string]interface{}{"username": "jane@example.com"}).
		Return(nil, errUserNotFound)
	s.mockProvider.On("GetEntityReference", context.Background(),
		map[string]interface{}{"email": "jane@example.com"}).
		Return(&providers.EntityReference{EntityID: "user-1"}, (*tidcommon.ServiceError)(nil))

	candidates, svcErr := s.mgr.ResolveLinkCandidates(context.Background(), authUser)

	s.Nil(svcErr)
	if s.NotNil(candidates) {
		s.Equal([]string{"user-1"}, candidates.EntityIDs)
		s.Equal(map[string]string{"email": "jane@example.com"}, candidates.MatchedAttributes)
	}
}

// Filters that all name the same user report every value it matched on.
func (s *ManagerTestSuite) TestResolveLinkCandidates_ReportsEveryMatchingFilter() {
	authUser := pendingFederatedAuthUser([]map[string]interface{}{
		{"username": "jane@example.com"}, {"email": "jane@example.com"}})
	s.mockProvider.On("GetEntityReference", context.Background(), mock.Anything).
		Return(&providers.EntityReference{EntityID: "user-1"}, (*tidcommon.ServiceError)(nil))

	candidates, svcErr := s.mgr.ResolveLinkCandidates(context.Background(), authUser)

	s.Nil(svcErr)
	if s.NotNil(candidates) {
		s.Equal([]string{"user-1"}, candidates.EntityIDs)
		s.Equal(map[string]string{"username": "jane@example.com", "email": "jane@example.com"},
			candidates.MatchedAttributes)
	}
}

// Filters naming different users all count, sorted by id, and verification decides which one is
// the End-User's. A user named by two filters is listed once, and each entity type once, in the
// order found.
func (s *ManagerTestSuite) TestResolveLinkCandidates_FiltersNamingDifferentUsersAreAllCandidates() {
	authUser := pendingFederatedAuthUser([]map[string]interface{}{
		{"username": "jane@example.com"}, {"email": "jane@example.com"}, {"mobile": "+15550100"}})
	s.mockProvider.On("GetEntityReference", context.Background(),
		map[string]interface{}{"username": "jane@example.com"}).
		Return(&providers.EntityReference{EntityID: "user-2", EntityType: "employee"}, (*tidcommon.ServiceError)(nil))
	s.mockProvider.On("GetEntityReference", context.Background(),
		map[string]interface{}{"email": "jane@example.com"}).
		Return(&providers.EntityReference{EntityID: "user-1", EntityType: "customer"}, (*tidcommon.ServiceError)(nil))
	s.mockProvider.On("GetEntityReference", context.Background(),
		map[string]interface{}{"mobile": "+15550100"}).
		Return(&providers.EntityReference{EntityID: "user-2", EntityType: "employee"}, (*tidcommon.ServiceError)(nil))

	candidates, svcErr := s.mgr.ResolveLinkCandidates(context.Background(), authUser)

	s.Nil(svcErr)
	if s.NotNil(candidates) {
		s.Equal([]string{"user-1", "user-2"}, candidates.EntityIDs)
		s.Equal([]string{"employee", "customer"}, candidates.EntityTypes)
		s.Equal(map[string]string{
			"username": "jane@example.com", "email": "jane@example.com", "mobile": "+15550100"},
			candidates.MatchedAttributes)
	}
}

// Nothing matching is the ordinary first sign-in through a connection, not a failure. A provider
// answering with no entity id names nobody either.
func (s *ManagerTestSuite) TestResolveLinkCandidates_NoMatchReturnsNothing() {
	authUser := pendingFederatedAuthUser([]map[string]interface{}{
		{"username": "jane@example.com"}, {"email": "jane@example.com"}})
	s.mockProvider.On("GetEntityReference", context.Background(),
		map[string]interface{}{"username": "jane@example.com"}).
		Return(nil, errUserNotFound)
	s.mockProvider.On("GetEntityReference", context.Background(),
		map[string]interface{}{"email": "jane@example.com"}).
		Return(&providers.EntityReference{}, (*tidcommon.ServiceError)(nil))

	candidates, svcErr := s.mgr.ResolveLinkCandidates(context.Background(), authUser)

	s.Nil(svcErr)
	s.Nil(candidates)
}

// A provider that cannot list what an ambiguous lookup matched leaves no ids to verify against, so
// the lookup still fails closed.
func (s *ManagerTestSuite) TestResolveLinkCandidates_AmbiguousMatchFailsClosed() {
	authUser := pendingFederatedAuthUser([]map[string]interface{}{{"email": "jane@example.com"}})
	s.mockProvider.On("GetEntityReference", context.Background(), mock.Anything).
		Return(nil, &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType, Code: authnprovidercm.ErrorCodeAmbiguousUser})
	s.mockProvider.On("SearchEntityReferences", mock.Anything, mock.Anything).
		Return(nil, &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType, Code: authnprovidercm.ErrorCodeNotImplemented})

	candidates, svcErr := s.mgr.ResolveLinkCandidates(context.Background(), authUser)

	s.Nil(candidates)
	if s.NotNil(svcErr) {
		s.Equal(ErrorAmbiguousUser.Code, svcErr.Code)
	}
}

// searchingManager is the suite's manager, with the provider's ambiguous lookups listing refs, or
// failing with err.
func (s *ManagerTestSuite) searchingManager(refs []providers.EntityReference,
	err *tidcommon.ServiceError) providers.AuthnProviderManager {
	s.mockProvider.On("SearchEntityReferences", mock.Anything, mock.Anything).Return(refs, err)
	return s.mgr
}

var errAmbiguousUser = &tidcommon.ServiceError{
	Type: tidcommon.ClientErrorType, Code: authnprovidercm.ErrorCodeAmbiguousUser}

// entityRefs returns a reference for each id.
func entityRefs(ids ...string) []providers.EntityReference {
	refs := make([]providers.EntityReference, 0, len(ids))
	for _, id := range ids {
		refs = append(refs, providers.EntityReference{EntityID: id})
	}
	return refs
}

// One lookup matching several entities offers each of them, merged with what the other lookups
// name and sorted by id.
func (s *ManagerTestSuite) TestResolveLinkCandidates_AmbiguousMatchListsEveryUser() {
	authUser := pendingFederatedAuthUser([]map[string]interface{}{
		{"costCenter": "CC-1"}, {"email": "jane@example.com"}})
	s.mockProvider.On("GetEntityReference", context.Background(),
		map[string]interface{}{"costCenter": "CC-1"}).Return(nil, errAmbiguousUser)
	s.mockProvider.On("GetEntityReference", context.Background(),
		map[string]interface{}{"email": "jane@example.com"}).
		Return(&providers.EntityReference{EntityID: "user-1"}, (*tidcommon.ServiceError)(nil))

	candidates, svcErr := s.searchingManager(entityRefs("user-3", "user-1"), nil).
		ResolveLinkCandidates(context.Background(), authUser)

	s.Nil(svcErr)
	if s.NotNil(candidates) {
		s.Equal([]string{"user-1", "user-3"}, candidates.EntityIDs)
		s.Equal(map[string]string{"costCenter": "CC-1", "email": "jane@example.com"},
			candidates.MatchedAttributes)
	}
}

// The search matches differently from the lookup, so it is trusted only when it lists several: one
// or none is a disagreement between the two, and nothing may be picked from it.
func (s *ManagerTestSuite) TestResolveLinkCandidates_AmbiguousMatchListingFewerThanTwoFailsClosed() {
	for name, refs := range map[string][]providers.EntityReference{
		"none": nil, "one": entityRefs("user-1"), "no ids": entityRefs("", ""),
	} {
		s.Run(name, func() {
			s.SetupTest()
			authUser := pendingFederatedAuthUser([]map[string]interface{}{{"costCenter": "CC-1"}})
			s.mockProvider.On("GetEntityReference", context.Background(), mock.Anything).
				Return(nil, errAmbiguousUser)

			candidates, svcErr := s.searchingManager(refs, nil).
				ResolveLinkCandidates(context.Background(), authUser)

			s.Nil(candidates)
			if s.NotNil(svcErr) {
				s.Equal(ErrorAmbiguousUser.Code, svcErr.Code)
			}
		})
	}
}

// More candidates than the cap means a linking attribute that barely tells accounts apart, so the
// match fails closed rather than carrying them all through the flow.
func (s *ManagerTestSuite) TestResolveLinkCandidates_TooManyCandidatesFailClosed() {
	ids := make([]string, 0, maxLinkingCandidates+1)
	for i := 0; i <= maxLinkingCandidates; i++ {
		ids = append(ids, "user-"+string(rune('a'+i)))
	}
	authUser := pendingFederatedAuthUser([]map[string]interface{}{{"costCenter": "CC-1"}})
	s.mockProvider.On("GetEntityReference", context.Background(), mock.Anything).
		Return(nil, errAmbiguousUser)

	candidates, svcErr := s.searchingManager(entityRefs(ids...), nil).
		ResolveLinkCandidates(context.Background(), authUser)

	s.Nil(candidates)
	if s.NotNil(svcErr) {
		s.Equal(ErrorAmbiguousUser.Code, svcErr.Code)
	}
}

// Once the candidates pass the cap, the remaining filters are not looked up.
func (s *ManagerTestSuite) TestResolveLinkCandidates_StopsLookingUpPastTheCap() {
	ids := make([]string, 0, maxLinkingCandidates+1)
	for i := 0; i <= maxLinkingCandidates; i++ {
		ids = append(ids, "user-"+string(rune('a'+i)))
	}
	first := map[string]interface{}{"costCenter": "CC-1"}
	second := map[string]interface{}{"department": "D-1"}
	authUser := pendingFederatedAuthUser([]map[string]interface{}{first, second})
	s.mockProvider.On("GetEntityReference", context.Background(), first).
		Return(nil, errAmbiguousUser)

	candidates, svcErr := s.searchingManager(entityRefs(ids...), nil).
		ResolveLinkCandidates(context.Background(), authUser)

	s.Nil(candidates)
	if s.NotNil(svcErr) {
		s.Equal(ErrorAmbiguousUser.Code, svcErr.Code)
	}
	s.mockProvider.AssertNotCalled(s.T(), "GetEntityReference", context.Background(), second)
}

// A connection that skipped save validation can build more filters than the bound, and none of
// them is looked up.
func (s *ManagerTestSuite) TestResolveLinkCandidates_TooManyFiltersFailWithoutLookups() {
	filters := make([]map[string]interface{}, 0, authnprovidercm.MaxAccountLinkingFilters+1)
	for i := 0; i <= authnprovidercm.MaxAccountLinkingFilters; i++ {
		filters = append(filters, map[string]interface{}{"costCenter": "CC-" + string(rune('a'+i))})
	}
	authUser := pendingFederatedAuthUser(filters)

	candidates, svcErr := s.mgr.ResolveLinkCandidates(context.Background(), authUser)

	s.Nil(candidates)
	if s.NotNil(svcErr) {
		s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	}
	s.mockProvider.AssertNotCalled(s.T(), "GetEntityReference", mock.Anything, mock.Anything)
}

// Exactly the cap is still a choice verification can make.
func (s *ManagerTestSuite) TestResolveLinkCandidates_CandidatesAtTheCapAreOffered() {
	ids := make([]string, 0, maxLinkingCandidates)
	for i := 0; i < maxLinkingCandidates; i++ {
		ids = append(ids, "user-"+string(rune('a'+i)))
	}
	authUser := pendingFederatedAuthUser([]map[string]interface{}{{"costCenter": "CC-1"}})
	s.mockProvider.On("GetEntityReference", context.Background(), mock.Anything).
		Return(nil, errAmbiguousUser)

	candidates, svcErr := s.searchingManager(entityRefs(ids...), nil).
		ResolveLinkCandidates(context.Background(), authUser)

	s.Nil(svcErr)
	if s.NotNil(candidates) {
		s.Equal(ids, candidates.EntityIDs)
	}
}

func (s *ManagerTestSuite) TestResolveLinkCandidates_AmbiguousMatchSearchFailureFails() {
	authUser := pendingFederatedAuthUser([]map[string]interface{}{{"costCenter": "CC-1"}})
	s.mockProvider.On("GetEntityReference", context.Background(), mock.Anything).
		Return(nil, errAmbiguousUser)

	candidates, svcErr := s.searchingManager(nil,
		&tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "AUP-5000"}).
		ResolveLinkCandidates(context.Background(), authUser)

	s.Nil(candidates)
	if s.NotNil(svcErr) {
		s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	}
}

func (s *ManagerTestSuite) TestResolveLinkCandidates_ProviderServerErrorFails() {
	authUser := pendingFederatedAuthUser([]map[string]interface{}{{"email": "jane@example.com"}})
	s.mockProvider.On("GetEntityReference", context.Background(), mock.Anything).
		Return(nil, &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "AUP-5000"})

	candidates, svcErr := s.mgr.ResolveLinkCandidates(context.Background(), authUser)

	s.Nil(candidates)
	if s.NotNil(svcErr) {
		s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	}
}

// The token is persisted with the flow, so the filters come back from JSON in their decoded shape.
func (s *ManagerTestSuite) TestResolveLinkCandidates_ReadsFiltersAfterJSONRoundTrip() {
	authUser := pendingFederatedAuthUser([]interface{}{
		map[string]interface{}{"email": "jane@example.com"}, "not-a-filter", map[string]interface{}{}})
	s.mockProvider.On("GetEntityReference", context.Background(),
		map[string]interface{}{"email": "jane@example.com"}).
		Return(&providers.EntityReference{EntityID: "user-1"}, (*tidcommon.ServiceError)(nil)).Once()

	candidates, svcErr := s.mgr.ResolveLinkCandidates(context.Background(), authUser)

	s.Nil(svcErr)
	if s.NotNil(candidates) {
		s.Equal([]string{"user-1"}, candidates.EntityIDs)
	}
}

// A connection with no account-linking attributes configured has nothing to match on, so the
// provider is never asked. Attribute values outside the filter list are never matched on either.
func (s *ManagerTestSuite) TestResolveLinkCandidates_NoLinkingAttributesSkipsLookup() {
	authUser := authenticatedAuthUserWithTokens(
		map[string]interface{}{"federatedIdpId": "idp-1", "sub": "sub-1", "email": "jane@example.com"},
		map[string]interface{}{"federatedIdpId": "idp-1", "sub": "sub-1", "email": "jane@example.com"})

	candidates, svcErr := s.mgr.ResolveLinkCandidates(context.Background(), authUser)

	s.Nil(svcErr)
	s.Nil(candidates)
	s.mockProvider.AssertNotCalled(s.T(), "GetEntityReference", mock.Anything, mock.Anything)
}

// An already-resolved state carries no pending identity, so there is nothing to match.
func (s *ManagerTestSuite) TestResolveLinkCandidates_ResolvedStateHasNoPendingIdentity() {
	authUser := authenticatedAuthUserWithResolved(&providers.EntityReference{EntityID: "user-1"}, nil)

	candidates, svcErr := s.mgr.ResolveLinkCandidates(context.Background(), authUser)

	s.Nil(svcErr)
	s.Nil(candidates)
	s.mockProvider.AssertNotCalled(s.T(), "GetEntityReference", mock.Anything, mock.Anything)
}
