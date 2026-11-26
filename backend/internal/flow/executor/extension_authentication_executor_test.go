// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	httpservice "github.com/thunder-id/thunderid/internal/system/http"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/connection/httpconnectionmock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/idpprovidermock"
)

const testAuthEndpointConnectionID = "conn-auth-endpoint"

type ExtensionAuthenticationExecutorTestSuite struct {
	suite.Suite
	executor    *extensionAuthenticationExecutor
	connections *httpconnectionmock.HTTPConnectionServiceInterfaceMock
	idpService  *idpprovidermock.IDPProviderMock
	authn       *managermock.AuthnProviderManagerMock
	server      *httptest.Server
}

func TestExtensionAuthenticationExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(ExtensionAuthenticationExecutorTestSuite))
}

func (s *ExtensionAuthenticationExecutorTestSuite) SetupSuite() {
	s.Require().NoError(config.InitializeServerRuntime("test", &config.Config{}))
}

func (s *ExtensionAuthenticationExecutorTestSuite) TearDownSuite() {
	config.ResetServerRuntime()
}

func (s *ExtensionAuthenticationExecutorTestSuite) SetupTest() {
	s.connections = httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	s.idpService = idpprovidermock.NewIDPProviderMock(s.T())
	s.authn = managermock.NewAuthnProviderManagerMock(s.T())

	factory := coremock.NewFlowFactoryInterfaceMock(s.T())
	factory.On("CreateExecutor", ExecutorNameExtensionAuthentication, providers.ExecutorTypeAuthentication,
		[]providers.Input{}, []providers.Input{}, mock.Anything).
		Return(newMockExecutor(ExecutorNameExtensionAuthentication, providers.ExecutorTypeAuthentication,
			[]providers.Input{}, []providers.Input{}))
	s.executor = newExtensionAuthenticationExecutor(factory, s.connections, s.idpService, s.authn,
		httpservice.Initialize(true).NewHTTPClientWithoutRedirects(0), nil)
}

func (s *ExtensionAuthenticationExecutorTestSuite) TearDownTest() {
	if s.server != nil {
		s.server.Close()
		s.server = nil
	}
}

// serve starts the organization's service answering every call with body, and resolves the connection to it.
func (s *ExtensionAuthenticationExecutorTestSuite) serve(body string) {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	s.connections.EXPECT().GetResolvedConnection(mock.Anything, testAuthEndpointConnectionID).
		Return(&httpconnection.ResolvedHTTPConnection{URL: s.server.URL, Timeout: 2 * time.Second,
			Headers: http.Header{}}, nil)
}

func (s *ExtensionAuthenticationExecutorTestSuite) nodeContext() *providers.NodeContext {
	return &providers.NodeContext{
		Context:       context.Background(),
		ExecutionID:   "flow-123",
		FlowType:      providers.FlowTypeAuthentication,
		CurrentNodeID: "org_login",
		UserInputs:    map[string]string{"username": "alice"},
		RuntimeData:   map[string]string{},
		NodeProperties: map[string]interface{}{
			common.NodePropertyConnectionID: testAuthEndpointConnectionID,
		},
	}
}

func (s *ExtensionAuthenticationExecutorTestSuite) authenticatedUser() providers.AuthUser {
	var authUser providers.AuthUser
	s.Require().NoError(authUser.UnmarshalJSON(
		[]byte(`{"default":{"entityReferenceToken":"tok","attributeToken":"tok"}}`)))
	return authUser
}

func (s *ExtensionAuthenticationExecutorTestSuite) expectConnection() {
	s.idpService.EXPECT().GetIdentityProvider(mock.Anything, testAuthEndpointConnectionID).
		Return(&providers.IDPDTO{ID: testAuthEndpointConnectionID, Type: providers.IDPTypeExtensionAuthentication},
			nil).Maybe()
}

func (s *ExtensionAuthenticationExecutorTestSuite) TestSuccessAuthenticatesTheVerifiedUser() {
	s.serve(`{"actionStatus":"SUCCESS","authenticatedUser":{"subject":"ext-42",` +
		`"attributes":{"email":"alice@example.com","age":31}},"other":"kept"}`)
	s.expectConnection()
	authUser := s.authenticatedUser()
	s.authn.On("AuthenticateUser", mock.Anything, mock.Anything,
		mock.MatchedBy(func(credentials map[string]interface{}) bool {
			raw := credentials[authnprovidercm.CredentialTypeExtensionVerifiedEntityID]
			cred, ok := raw.(*authncm.ExtensionVerifiedCredential)
			return ok && len(credentials) == 1 && cred.ConnectionID == testAuthEndpointConnectionID &&
				cred.Subject == "ext-42" && cred.Claims["email"] == "alice@example.com" && cred.Claims["age"] == "31"
		}), mock.Anything, mock.Anything, mock.Anything).
		Return(authUser, providers.AuthenticatedClaims{"sub": "ext-42", "email": "alice@example.com"},
			(*tidcommon.ServiceError)(nil))
	expectEntityReferenceResolved(s.authn, authUser)
	ctx := s.nodeContext()
	ctx.NodeProperties[common.NodePropertyAllowAuthenticationWithoutLocalUser] = true

	execResp, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, execResp.Status)
	s.True(execResp.AuthUser.IsAuthenticated())
	s.Equal("alice@example.com", externalClaim(execResp.RuntimeData, "email"))
	s.Equal(dataValueTrue, execResp.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning])
	s.Equal("kept", execResp.RuntimeData[common.ExtensionNamespace("org_login")+"other"])
	s.NotContains(execResp.RuntimeData, common.ExtensionNamespace("org_login")+extensionFieldAuthenticatedUser)
}

func (s *ExtensionAuthenticationExecutorTestSuite) TestSuccessWithoutAVerifiedUserFails() {
	for _, body := range []string{
		`{"actionStatus":"SUCCESS"}`,
		`{"actionStatus":"SUCCESS","authenticatedUser":"alice"}`,
		`{"actionStatus":"SUCCESS","authenticatedUser":{"attributes":{"email":"a@example.com"}}}`,
		`{"actionStatus":"SUCCESS","authenticatedUser":{"subject":"  "}}`,
	} {
		s.Run(body, func() {
			s.SetupTest()
			s.serve(body)

			execResp, err := s.executor.Execute(s.nodeContext())

			s.Require().NoError(err)
			s.Equal(providers.ExecFailure, execResp.Status)
			s.Equal(&ErrUserNotAuthenticated, execResp.Error)
			s.False(execResp.AuthUser.IsAuthenticated())
			s.server.Close()
		})
	}
}

func (s *ExtensionAuthenticationExecutorTestSuite) TestRedirectPassesThroughWithoutAuthenticating() {
	s.serve(`{"actionStatus":"INCOMPLETE","redirectUrl":"https://login.example.com/start"}`)

	execResp, err := s.executor.Execute(s.nodeContext())

	s.Require().NoError(err)
	s.Equal(providers.ExecExternalRedirection, execResp.Status)
	s.Equal("https://login.example.com/start", execResp.RedirectURL)
	s.authn.AssertNotCalled(s.T(), "AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything)
}

func (s *ExtensionAuthenticationExecutorTestSuite) TestFailedAnswerPassesThroughWithoutAuthenticating() {
	s.serve(`{"actionStatus":"FAILED","failureReason":"invalid_credentials"}`)

	execResp, err := s.executor.Execute(s.nodeContext())

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, execResp.Status)
	s.False(execResp.AuthUser.IsAuthenticated())
	s.authn.AssertNotCalled(s.T(), "AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything)
}

func (s *ExtensionAuthenticationExecutorTestSuite) TestRefusedAuthenticationFailsTheStep() {
	s.serve(`{"actionStatus":"SUCCESS","authenticatedUser":{"subject":"ext-42"}}`)
	s.authn.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims(nil),
			&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "AUTH-1"})

	execResp, err := s.executor.Execute(s.nodeContext())

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, execResp.Status)
	s.Require().NotNil(execResp.Error)
	s.False(execResp.AuthUser.IsAuthenticated())
}

func (s *ExtensionAuthenticationExecutorTestSuite) TestProviderServerErrorIsReturned() {
	s.serve(`{"actionStatus":"SUCCESS","authenticatedUser":{"subject":"ext-42"}}`)
	s.authn.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims(nil), &tidcommon.InternalServerError)

	execResp, err := s.executor.Execute(s.nodeContext())

	s.Error(err)
	s.Nil(execResp)
}

func (s *ExtensionAuthenticationExecutorTestSuite) TestSecondFactorChecksTheSignedInUser() {
	s.serve(`{"actionStatus":"SUCCESS","authenticatedUser":{"subject":"ext-42",` +
		`"attributes":{"email":"bob@example.com"}}}`)
	s.expectConnection()
	authUser := s.authenticatedUser()
	s.authn.On("GetUserAttributes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(authUser, &providers.AttributesResponse{Attributes: map[string]*providers.AttributeResponse{
			"email": {Value: "alice@example.com"},
		}}, (*tidcommon.ServiceError)(nil))
	s.authn.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).
		Return(authUser, providers.AuthenticatedClaims{"sub": "ext-42", "email": "bob@example.com"},
			(*tidcommon.ServiceError)(nil))
	ctx := s.nodeContext()
	ctx.AuthUser = authUser

	execResp, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, execResp.Status)
	s.Equal(&ErrInvalidFederatedUser, execResp.Error)
}

func (s *ExtensionAuthenticationExecutorTestSuite) TestVerifiedIdentityReadsSubjectAndAttributes() {
	subject, claims, ok := verifiedIdentity(map[string]interface{}{
		extensionFieldAuthenticatedUser: map[string]interface{}{
			extensionFieldSubject:    " ext-1 ",
			extensionFieldAttributes: map[string]interface{}{"a": "b", "gone": nil},
		},
	})

	s.True(ok)
	s.Equal("ext-1", subject)
	s.Equal(map[string]interface{}{"a": "b"}, claims)
}
