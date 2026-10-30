// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/ouprovidermock"
)

const (
	testResolverSourceKey = "extension.pin_check.verifiedUsername"
	testResolverUsername  = "jdoe"
	testResolverUserID    = "user-jdoe"
	testResolverUserType  = "person"
	testResolverAppOUID   = "ou-app"
	testResolverUserOUID  = "ou-team"
)

type ExtensionAuthResolverTestSuite struct {
	suite.Suite
	entities *entityprovidermock.EntityProviderInterfaceMock
	authn    *managermock.AuthnProviderManagerMock
	ous      *ouprovidermock.OrganizationUnitProviderMock
	resolver *extensionAuthResolver
}

func TestExtensionAuthResolverTestSuite(t *testing.T) {
	suite.Run(t, new(ExtensionAuthResolverTestSuite))
}

func (s *ExtensionAuthResolverTestSuite) SetupTest() {
	s.entities = entityprovidermock.NewEntityProviderInterfaceMock(s.T())
	s.authn = managermock.NewAuthnProviderManagerMock(s.T())
	s.ous = ouprovidermock.NewOrganizationUnitProviderMock(s.T())
	s.ous.EXPECT().IsParent(mock.Anything, testResolverAppOUID, testResolverUserOUID).Return(true, nil).Maybe()
	factory := coremock.NewFlowFactoryInterfaceMock(s.T())
	factory.On("CreateExecutor", ExecutorNameExtensionAuthResolver, providers.ExecutorTypeAuthentication,
		[]providers.Input{}, []providers.Input{}, mock.Anything).
		Return(newMockExecutor(ExecutorNameExtensionAuthResolver, providers.ExecutorTypeAuthentication,
			[]providers.Input{}, []providers.Input{}))
	s.resolver = newExtensionAuthResolver(factory, s.entities, s.authn, s.ous)
}

// nodeContext is the resolver placed after the extension step pin_check, which stored the username it
// verified.
func (s *ExtensionAuthResolverTestSuite) nodeContext() *providers.NodeContext {
	return &providers.NodeContext{
		Context:       context.Background(),
		ExecutionID:   "exec-1",
		FlowType:      providers.FlowTypeAuthentication,
		CurrentNodeID: "resolve_user",
		RuntimeData: map[string]string{
			testResolverSourceKey:              testResolverUsername,
			"extension.pin_check.email":        "jdoe@example.com",
			"extension.pin_check.serviceToken": "tok-1",
		},
		NodeProperties: map[string]interface{}{common.NodePropertySourceKey: testResolverSourceKey},
		Application: providers.Application{
			OUID:             testResolverAppOUID,
			AllowedUserTypes: []string{testResolverUserType},
		},
	}
}

func entityNotFound() *entityprovider.EntityProviderError {
	return entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", "")
}

func (s *ExtensionAuthResolverTestSuite) user(state providers.EntityState) *providers.Entity {
	return &providers.Entity{ID: testResolverUserID, Category: providers.EntityCategoryUser, State: state,
		Type: testResolverUserType, OUID: testResolverUserOUID}
}

func (s *ExtensionAuthResolverTestSuite) identifies(id string) {
	s.entities.On("IdentifyEntity", map[string]interface{}{userAttributeUsername: testResolverUsername}).
		Return(&id, nil).Once()
}

func (s *ExtensionAuthResolverTestSuite) identifyFails(code entityprovider.ErrorCode) {
	s.entities.On("IdentifyEntity", mock.Anything).
		Return(nil, entityprovider.NewEntityProviderError(code, "", "")).Once()
}

// expectAuthentication expects the resolved user to be authenticated through the internal credential.
func (s *ExtensionAuthResolverTestSuite) expectAuthentication(authUser providers.AuthUser,
	svcErr *tidcommon.ServiceError) {
	s.authn.On("AuthenticateUser", mock.Anything, mock.Anything,
		map[string]interface{}{authnprovidercm.CredentialTypeExtensionVerifiedEntityID: testResolverUserID},
		mock.Anything, mock.Anything, mock.Anything).
		Return(authUser, providers.AuthenticatedClaims{userAttributeUserID: testResolverUserID}, svcErr).Once()
}

func (s *ExtensionAuthResolverTestSuite) execute(ctx *providers.NodeContext) *providers.ExecutorResponse {
	execResp, err := s.resolver.Execute(ctx)
	s.Require().NoError(err)
	s.Require().NotNil(execResp)
	return execResp
}

// assertNotFound checks the resolver failed with the error a missing user gives, authenticating no one
// and marking no one for provisioning.
func (s *ExtensionAuthResolverTestSuite) assertNotFound(execResp *providers.ExecutorResponse) {
	s.T().Helper()
	s.Equal(providers.ExecFailure, execResp.Status)
	s.Equal(errForEntityCategory(ErrEntityNotFound, entitytype.TypeCategoryUser), execResp.Error)
	s.False(execResp.AuthUser.IsAuthenticated())
	s.NotContains(execResp.RuntimeData, common.RuntimeKeyUserEligibleForProvisioning)
	s.authn.AssertNotCalled(s.T(), "AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything)
}

func (s *ExtensionAuthResolverTestSuite) TestExecute_SignsInTheUserTheServiceVerified() {
	s.identifies(testResolverUserID)
	s.entities.On("GetEntity", testResolverUserID).Return(s.user(providers.EntityStateActive), nil).Once()
	s.expectAuthentication(newAuthenticatedAuthUser(), nil)

	execResp := s.execute(s.nodeContext())

	s.Equal(providers.ExecComplete, execResp.Status)
	s.Nil(execResp.Error)
	s.True(execResp.AuthUser.IsAuthenticated())
	s.Equal(testResolverUserID, execResp.RuntimeData[userAttributeUserID])
	s.NotContains(execResp.RuntimeData, common.RuntimeKeyUserEligibleForProvisioning)
	s.NotContains(execResp.RuntimeData, common.RuntimeKeyExternalIdentity)
}

func (s *ExtensionAuthResolverTestSuite) TestExecute_MatchesByUserIDWithoutALookup() {
	ctx := s.nodeContext()
	ctx.RuntimeData[testResolverSourceKey] = testResolverUserID
	ctx.NodeProperties[common.NodePropertyMatchAttribute] = userAttributeUserID
	s.entities.On("GetEntity", testResolverUserID).Return(s.user(providers.EntityStateActive), nil).Once()
	s.expectAuthentication(newAuthenticatedAuthUser(), nil)

	execResp := s.execute(ctx)

	s.Equal(providers.ExecComplete, execResp.Status)
	s.entities.AssertNotCalled(s.T(), "IdentifyEntity", mock.Anything)
}

func (s *ExtensionAuthResolverTestSuite) TestExecute_UnusableConfigurationFailsBeforeAnyLookup() {
	cases := map[string]map[string]interface{}{
		"no sourceKey":                    {},
		"blank sourceKey":                 {common.NodePropertySourceKey: "  "},
		"non-string sourceKey":            {common.NodePropertySourceKey: 42},
		"sourceKey outside the namespace": {common.NodePropertySourceKey: userAttributeUsername},
		"non-string matchAttribute": {common.NodePropertySourceKey: testResolverSourceKey,
			common.NodePropertyMatchAttribute: true},
		"self-service matchAttribute": {common.NodePropertySourceKey: testResolverSourceKey,
			common.NodePropertyMatchAttribute: userAttributeEmail},
		"attributeMapping not a map": {common.NodePropertySourceKey: testResolverSourceKey,
			common.NodePropertyAttributeMapping: userAttributeEmail},
		"attributeMapping value outside the namespace": {common.NodePropertySourceKey: testResolverSourceKey,
			common.NodePropertyAttributeMapping: map[string]interface{}{userAttributeEmail: userAttributeEmail}},
		"attributeMapping non-string value": {common.NodePropertySourceKey: testResolverSourceKey,
			common.NodePropertyAttributeMapping: map[string]interface{}{userAttributeEmail: 1}},
		"attributeMapping blank attribute": {common.NodePropertySourceKey: testResolverSourceKey,
			common.NodePropertyAttributeMapping: map[string]interface{}{" ": "extension.pin_check.email"}},
	}
	for name, properties := range cases {
		s.Run(name, func() {
			s.SetupTest()
			ctx := s.nodeContext()
			ctx.NodeProperties = properties

			execResp := s.execute(ctx)

			s.Equal(providers.ExecFailure, execResp.Status)
			s.Equal(&ErrExtensionAuthResolverConfigInvalid, execResp.Error)
			s.entities.AssertNotCalled(s.T(), "IdentifyEntity", mock.Anything)
			s.entities.AssertNotCalled(s.T(), "GetEntity", mock.Anything)
		})
	}
}

func (s *ExtensionAuthResolverTestSuite) TestExecute_MissingSourceValueAuthenticatesNoOne() {
	for name, value := range map[string]*string{"absent": nil, "empty": new("")} {
		s.Run(name, func() {
			s.SetupTest()
			ctx := s.nodeContext()
			delete(ctx.RuntimeData, testResolverSourceKey)
			if value != nil {
				ctx.RuntimeData[testResolverSourceKey] = *value
			}
			ctx.NodeProperties[common.NodePropertyAllowAuthenticationWithoutLocalUser] = true

			execResp := s.execute(ctx)

			s.Equal(providers.ExecFailure, execResp.Status)
			s.Equal(&ErrUserNotAuthenticated, execResp.Error)
			s.NotContains(execResp.RuntimeData, common.RuntimeKeyUserEligibleForProvisioning)
			s.entities.AssertNotCalled(s.T(), "IdentifyEntity", mock.Anything)
		})
	}
}

func (s *ExtensionAuthResolverTestSuite) TestExecute_NotFoundNonUserAndInactiveGiveTheSameError() {
	cases := map[string]func(){
		"no match": func() { s.identifyFails(entityprovider.ErrorCodeEntityNotFound) },
		"empty match": func() {
			s.identifies("")
		},
		"non-user entity": func() {
			s.identifies(testResolverUserID)
			s.entities.On("GetEntity", testResolverUserID).Return(&providers.Entity{ID: testResolverUserID,
				Category: providers.EntityCategoryAgent, State: providers.EntityStateActive}, nil).Once()
		},
		"suspended user": func() {
			s.identifies(testResolverUserID)
			s.entities.On("GetEntity", testResolverUserID).Return(s.user(providers.EntityStateSuspended), nil).Once()
		},
		"user without a state": func() {
			s.identifies(testResolverUserID)
			s.entities.On("GetEntity", testResolverUserID).Return(s.user(""), nil).Once()
		},
		"entity gone after the lookup": func() {
			s.identifies(testResolverUserID)
			s.entities.On("GetEntity", testResolverUserID).
				Return(nil, entityNotFound()).Once()
		},
	}
	for name, setup := range cases {
		s.Run(name, func() {
			s.SetupTest()
			setup()

			s.assertNotFound(s.execute(s.nodeContext()))
		})
	}
}

func (s *ExtensionAuthResolverTestSuite) TestExecute_UserTheApplicationDoesNotAllowIsNotFound() {
	cases := map[string]func(ctx *providers.NodeContext){
		"user type not allowed": func(ctx *providers.NodeContext) {
			ctx.Application.AllowedUserTypes = []string{"partner"}
		},
		"no allowed user types": func(ctx *providers.NodeContext) {
			ctx.Application.AllowedUserTypes = nil
		},
		"application without an organization unit": func(ctx *providers.NodeContext) {
			ctx.Application.OUID = ""
		},
		"organization unit the application does not serve": func(ctx *providers.NodeContext) {
			ctx.Application.OUID = "ou-other"
			s.ous.EXPECT().IsParent(mock.Anything, "ou-other", testResolverUserOUID).Return(false, nil).Once()
		},
		"organization unit that cannot be read": func(ctx *providers.NodeContext) {
			ctx.Application.OUID = "ou-gone"
			s.ous.EXPECT().IsParent(mock.Anything, "ou-gone", testResolverUserOUID).
				Return(false, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "OU-1003"}).Once()
		},
	}
	for name, adjust := range cases {
		s.Run(name, func() {
			s.SetupTest()
			ctx := s.nodeContext()
			ctx.NodeProperties[common.NodePropertyAllowAuthenticationWithoutLocalUser] = true
			adjust(ctx)
			s.identifies(testResolverUserID)
			s.entities.On("GetEntity", testResolverUserID).Return(s.user(providers.EntityStateActive), nil).Once()

			execResp := s.execute(ctx)

			s.assertNotFound(execResp)
		})
	}
}

func (s *ExtensionAuthResolverTestSuite) TestExecute_OrganizationUnitServerErrorIsAnInternalError() {
	ctx := s.nodeContext()
	ctx.Application.OUID = "ou-broken"
	s.ous.EXPECT().IsParent(mock.Anything, "ou-broken", testResolverUserOUID).
		Return(false, &tidcommon.InternalServerError).Once()
	s.identifies(testResolverUserID)
	s.entities.On("GetEntity", testResolverUserID).Return(s.user(providers.EntityStateActive), nil).Once()

	execResp, err := s.resolver.Execute(ctx)

	s.Error(err)
	s.Nil(execResp)
}

func (s *ExtensionAuthResolverTestSuite) TestExecute_ResolverIsNotRecordedAsAnAuthenticator() {
	s.Empty(getAuthnServiceName(ExecutorNameExtensionAuthResolver),
		"a sign-in through the resolver claims no authentication method the flow did not record")
}

func (s *ExtensionAuthResolverTestSuite) TestExecute_MoreThanOneMatchIsAmbiguous() {
	s.identifyFails(entityprovider.ErrorCodeAmbiguousEntity)

	execResp := s.execute(s.nodeContext())

	s.Equal(providers.ExecFailure, execResp.Status)
	s.Equal(errForEntityCategory(ErrAmbiguousEntityIdentity, entitytype.TypeCategoryUser), execResp.Error)
	s.False(execResp.AuthUser.IsAuthenticated())
}

func (s *ExtensionAuthResolverTestSuite) TestExecute_EntityProviderFailureIsAnInternalError() {
	s.Run("lookup", func() {
		s.SetupTest()
		s.identifyFails(entityprovider.ErrorCodeSystemError)

		execResp, err := s.resolver.Execute(s.nodeContext())

		s.Error(err)
		s.Nil(execResp)
	})
	s.Run("read", func() {
		s.SetupTest()
		s.identifies(testResolverUserID)
		s.entities.On("GetEntity", testResolverUserID).
			Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "", "")).Once()

		execResp, err := s.resolver.Execute(s.nodeContext())

		s.Error(err)
		s.Nil(execResp)
	})
}

func (s *ExtensionAuthResolverTestSuite) TestExecute_AuthenticationFailureAuthenticatesNoOne() {
	cases := map[string]struct {
		authUser providers.AuthUser
		svcErr   *tidcommon.ServiceError
	}{
		"refused":         {svcErr: &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "AUTHN-1"}},
		"unauthenticated": {},
	}
	for name, tc := range cases {
		s.Run(name, func() {
			s.SetupTest()
			s.identifies(testResolverUserID)
			s.entities.On("GetEntity", testResolverUserID).Return(s.user(providers.EntityStateActive), nil).Once()
			s.expectAuthentication(tc.authUser, tc.svcErr)

			execResp := s.execute(s.nodeContext())

			s.Equal(providers.ExecFailure, execResp.Status)
			s.Equal(errForEntityCategory(ErrEntityAuthFailed, entitytype.TypeCategoryUser), execResp.Error)
			s.False(execResp.AuthUser.IsAuthenticated())
			s.NotContains(execResp.RuntimeData, userAttributeUserID)
		})
	}
}

func (s *ExtensionAuthResolverTestSuite) TestExecute_AuthenticationServerErrorIsAnInternalError() {
	s.identifies(testResolverUserID)
	s.entities.On("GetEntity", testResolverUserID).Return(s.user(providers.EntityStateActive), nil).Once()
	s.expectAuthentication(providers.AuthUser{}, &tidcommon.InternalServerError)

	execResp, err := s.resolver.Execute(s.nodeContext())

	s.Error(err)
	s.Nil(execResp)
}

func (s *ExtensionAuthResolverTestSuite) TestExecute_MissingUserIsMarkedForProvisioningWhenAllowed() {
	s.identifyFails(entityprovider.ErrorCodeEntityNotFound)
	ctx := s.nodeContext()
	ctx.NodeProperties[common.NodePropertyAllowAuthenticationWithoutLocalUser] = true
	ctx.NodeProperties[common.NodePropertyAttributeMapping] = map[string]interface{}{
		userAttributeUsername: testResolverSourceKey,
		userAttributeEmail:    "extension.pin_check.email",
		"given_name":          "extension.pin_check.givenName",
	}

	execResp := s.execute(ctx)

	s.Equal(providers.ExecComplete, execResp.Status)
	s.Nil(execResp.Error)
	s.False(execResp.AuthUser.IsAuthenticated(), "no one is signed in until the user is provisioned")
	s.Equal(dataValueTrue, execResp.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning])
	identity := core.GetExternalIdentity(execResp.RuntimeData)
	s.Require().NotNil(identity)
	s.Empty(identity.IdpID)
	s.Empty(identity.Sub)
	s.Equal(map[string]interface{}{userAttributeUsername: testResolverUsername, userAttributeEmail: "jdoe@example.com"},
		identity.Claims, "only mapped attributes that hold a value are offered, never other stored fields")
	s.authn.AssertNotCalled(s.T(), "AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything)
}

func (s *ExtensionAuthResolverTestSuite) TestExecute_MissingUserWithoutAMappingOffersNoAttributes() {
	s.identifyFails(entityprovider.ErrorCodeEntityNotFound)
	ctx := s.nodeContext()
	ctx.NodeProperties[common.NodePropertyAllowAuthenticationWithoutLocalUser] = true

	execResp := s.execute(ctx)

	s.Equal(providers.ExecComplete, execResp.Status)
	s.Equal(dataValueTrue, execResp.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning])
	s.NotContains(execResp.RuntimeData, common.RuntimeKeyExternalIdentity)
}

func (s *ExtensionAuthResolverTestSuite) TestExecute_OnlyAMissingUserIsMarkedForProvisioning() {
	cases := map[string]struct {
		setup  func(ctx *providers.NodeContext)
		assert func(execResp *providers.ExecutorResponse)
	}{
		"non-user entity": {
			setup: func(*providers.NodeContext) {
				s.identifies(testResolverUserID)
				s.entities.On("GetEntity", testResolverUserID).Return(&providers.Entity{ID: testResolverUserID,
					Category: providers.EntityCategoryAgent, State: providers.EntityStateActive}, nil).Once()
			},
			assert: s.assertNotFound,
		},
		"inactive user": {
			setup: func(*providers.NodeContext) {
				s.identifies(testResolverUserID)
				s.entities.On("GetEntity", testResolverUserID).
					Return(s.user(providers.EntityStateSuspended), nil).Once()
			},
			assert: s.assertNotFound,
		},
		"more than one match": {
			setup: func(*providers.NodeContext) { s.identifyFails(entityprovider.ErrorCodeAmbiguousEntity) },
			assert: func(execResp *providers.ExecutorResponse) {
				s.Equal(errForEntityCategory(ErrAmbiguousEntityIdentity, entitytype.TypeCategoryUser), execResp.Error)
			},
		},
		"lookup by userID": {
			setup: func(ctx *providers.NodeContext) {
				ctx.NodeProperties[common.NodePropertyMatchAttribute] = userAttributeUserID
				s.entities.On("GetEntity", testResolverUsername).
					Return(nil, entityNotFound()).Once()
			},
			assert: s.assertNotFound,
		},
		"registration flow": {
			setup: func(ctx *providers.NodeContext) {
				ctx.FlowType = providers.FlowTypeRegistration
				s.identifyFails(entityprovider.ErrorCodeEntityNotFound)
			},
			assert: s.assertNotFound,
		},
	}
	for name, tc := range cases {
		s.Run(name, func() {
			s.SetupTest()
			ctx := s.nodeContext()
			ctx.NodeProperties[common.NodePropertyAllowAuthenticationWithoutLocalUser] = true
			tc.setup(ctx)

			execResp := s.execute(ctx)

			s.Equal(providers.ExecFailure, execResp.Status)
			s.NotContains(execResp.RuntimeData, common.RuntimeKeyUserEligibleForProvisioning)
			s.NotContains(execResp.RuntimeData, common.RuntimeKeyExternalIdentity)
			tc.assert(execResp)
		})
	}
}
