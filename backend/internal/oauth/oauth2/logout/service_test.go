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

package logout

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	"github.com/thunder-id/thunderid/internal/system/config"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/actorprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/flowexecmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

const (
	testIssuer      = "https://issuer.test"
	testExecutionID = "exec-1"
)

type LogoutServiceTestSuite struct {
	suite.Suite
}

func TestLogoutServiceTestSuite(t *testing.T) {
	suite.Run(t, new(LogoutServiceTestSuite))
}

func (suite *LogoutServiceTestSuite) SetupTest() {
	suite.Require().NoError(config.InitializeServerRuntime(suite.T().TempDir(), &config.Config{}))
}

func (suite *LogoutServiceTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *LogoutServiceTestSuite) newService() (*logoutService, *jwtmock.JWTServiceInterfaceMock,
	*actorprovidermock.ActorProviderMock) {
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(suite.T())
	actor := actorprovidermock.NewActorProviderMock(suite.T())
	flowSvc := flowexecmock.NewFlowExecServiceInterfaceMock(suite.T())
	store := newLogoutRequestStoreInterfaceMock(suite.T())
	return newLogoutService(jwtSvc, actor, flowSvc, store, testIssuer), jwtSvc, actor
}

func (suite *LogoutServiceTestSuite) newServiceWithStore(
	store logoutRequestStoreInterface, flowSvc *flowexecmock.FlowExecServiceInterfaceMock,
) *logoutService {
	return newLogoutService(jwtmock.NewJWTServiceInterfaceMock(suite.T()),
		actorprovidermock.NewActorProviderMock(suite.T()), flowSvc, store, testIssuer)
}

func (suite *LogoutServiceTestSuite) TestInitiateSignOutFlow_StoresContextAndInitiates() {
	store := newLogoutRequestStoreInterfaceMock(suite.T())
	// The validated target is persisted server-side, keyed by the returned logout id.
	store.EXPECT().AddRequest(mock.Anything, logoutRequestContext{
		AppID: "app-1", PostLogoutRedirectURI: "https://rp.example/after", State: "xyz",
	}).Return("logout-1", nil)
	flowSvc := flowexecmock.NewFlowExecServiceInterfaceMock(suite.T())
	var captured *flowexec.FlowInitContext
	flowSvc.EXPECT().InitiateFlow(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, ic *flowexec.FlowInitContext) (string, *tidcommon.ServiceError) {
			captured = ic
			return testExecutionID, nil
		})
	svc := suite.newServiceWithStore(store, flowSvc)

	initiation, svcErr := svc.InitiateSignOutFlow(context.Background(), &LogoutResolution{
		AppID: "app-1", PostLogoutRedirectURI: "https://rp.example/after", State: "xyz",
	})

	suite.Nil(svcErr)
	suite.Require().NotNil(initiation)
	suite.Equal(testExecutionID, initiation.ExecutionID)
	suite.Equal("logout-1", initiation.LogoutID)
	// The flow carries no post-logout data — it stays protocol-agnostic.
	suite.Require().NotNil(captured)
	suite.Equal("app-1", captured.ApplicationID)
	suite.Equal("SIGNOUT", captured.FlowType)
	suite.Empty(captured.RuntimeData)
}

func (suite *LogoutServiceTestSuite) TestCompleteSignOut_ReturnsRedirectWithStateAndConsumes() {
	store := newLogoutRequestStoreInterfaceMock(suite.T())
	store.EXPECT().GetRequest(mock.Anything, "logout-1").Return(true, logoutRequestContext{
		AppID: "app-1", PostLogoutRedirectURI: "https://rp.example/after", State: "xyz",
	}, nil)
	// Single-use: the request is consumed on completion.
	store.EXPECT().ClearRequest(mock.Anything, "logout-1").Return(nil)
	svc := suite.newServiceWithStore(store, flowexecmock.NewFlowExecServiceInterfaceMock(suite.T()))

	redirectURI, err := svc.CompleteSignOut(context.Background(), "logout-1")

	suite.Require().NoError(err)
	suite.Contains(redirectURI, "https://rp.example/after")
	suite.Contains(redirectURI, "state=xyz")
}

func (suite *LogoutServiceTestSuite) TestCompleteSignOut_NoRedirectURI() {
	store := newLogoutRequestStoreInterfaceMock(suite.T())
	store.EXPECT().GetRequest(mock.Anything, "logout-1").
		Return(true, logoutRequestContext{AppID: "app-1"}, nil)
	store.EXPECT().ClearRequest(mock.Anything, "logout-1").Return(nil)
	svc := suite.newServiceWithStore(store, flowexecmock.NewFlowExecServiceInterfaceMock(suite.T()))

	redirectURI, err := svc.CompleteSignOut(context.Background(), "logout-1")

	suite.Require().NoError(err)
	suite.Empty(redirectURI)
}

func (suite *LogoutServiceTestSuite) TestCompleteSignOut_UnknownID() {
	store := newLogoutRequestStoreInterfaceMock(suite.T())
	store.EXPECT().GetRequest(mock.Anything, "unknown").Return(false, logoutRequestContext{}, nil)
	svc := suite.newServiceWithStore(store, flowexecmock.NewFlowExecServiceInterfaceMock(suite.T()))

	redirectURI, err := svc.CompleteSignOut(context.Background(), "unknown")

	suite.Require().NoError(err)
	suite.Empty(redirectURI)
}

func (suite *LogoutServiceTestSuite) TestInitiateSignOutFlow_StripsIDTokenHintFromInitiatorRequest() {
	// id_token_hint has already been consumed by the OAuth layer at Resolve() to identify the
	// target client; it must not be persisted into the flow context store as part of the initiator
	// request. Other logout params (state, client_id, post_logout_redirect_uri) are not
	// credential-bearing and must be preserved.
	store := newLogoutRequestStoreInterfaceMock(suite.T())
	store.EXPECT().AddRequest(mock.Anything, mock.Anything).Return("logout-1", nil)
	flowSvc := flowexecmock.NewFlowExecServiceInterfaceMock(suite.T())
	var captured *flowexec.FlowInitContext
	flowSvc.EXPECT().InitiateFlow(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, ic *flowexec.FlowInitContext) (string, *tidcommon.ServiceError) {
			captured = ic
			return testExecutionID, nil
		})
	svc := suite.newServiceWithStore(store, flowSvc)

	_, svcErr := svc.InitiateSignOutFlow(context.Background(), &LogoutResolution{
		AppID: "app-1",
		QueryParams: map[string][]string{
			"id_token_hint":            {"header.payload.sig"},
			"client_id":                {"client-x"},
			"post_logout_redirect_uri": {"https://rp.example/after"},
			"state":                    {"xyz"},
		},
	})

	suite.Nil(svcErr)
	suite.Require().NotNil(captured)
	suite.Require().NotNil(captured.InitiatorRequest)
	forwarded := captured.InitiatorRequest.QueryParams
	suite.NotContains(forwarded, "id_token_hint")
	suite.Equal([]string{"client-x"}, forwarded["client_id"])
	suite.Equal([]string{"https://rp.example/after"}, forwarded["post_logout_redirect_uri"])
	suite.Equal([]string{"xyz"}, forwarded["state"])
}

func (suite *LogoutServiceTestSuite) TestFilterQueryParams() {
	testCases := []struct {
		name     string
		input    map[string][]string
		exclude  []string
		expected map[string][]string
	}{
		{
			name:     "NilInputReturnedAsIs",
			input:    nil,
			exclude:  []string{"id_token_hint"},
			expected: nil,
		},
		{
			name:     "EmptyInputReturnedAsIs",
			input:    map[string][]string{},
			exclude:  []string{"id_token_hint"},
			expected: map[string][]string{},
		},
		{
			name: "NoExcludeReturnsCopy",
			input: map[string][]string{
				"client_id": {"c1"},
				"state":     {"s1"},
			},
			exclude: nil,
			expected: map[string][]string{
				"client_id": {"c1"},
				"state":     {"s1"},
			},
		},
		{
			name: "RemovesNamedKey",
			input: map[string][]string{
				"id_token_hint": {"header.payload.sig"},
				"client_id":     {"c1"},
			},
			exclude: []string{"id_token_hint"},
			expected: map[string][]string{
				"client_id": {"c1"},
			},
		},
		{
			name: "RemovesMultipleKeys",
			input: map[string][]string{
				"a": {"1"},
				"b": {"2"},
				"c": {"3"},
			},
			exclude: []string{"a", "c"},
			expected: map[string][]string{
				"b": {"2"},
			},
		},
		{
			name: "UnknownExcludeIsNoOp",
			input: map[string][]string{
				"client_id": {"c1"},
			},
			exclude: []string{"not_present"},
			expected: map[string][]string{
				"client_id": {"c1"},
			},
		},
		{
			name: "MatchIsCaseSensitive",
			input: map[string][]string{
				"ID_Token_Hint": {"header.payload.sig"},
				"id_token_hint": {"other"},
			},
			exclude: []string{"id_token_hint"},
			expected: map[string][]string{
				"ID_Token_Hint": {"header.payload.sig"},
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result := filterQueryParams(tc.input, tc.exclude...)
			suite.Equal(tc.expected, result)
		})
	}
}

func (suite *LogoutServiceTestSuite) TestFilterQueryParams_DoesNotMutateInput() {
	input := map[string][]string{
		"id_token_hint": {"header.payload.sig"},
		"client_id":     {"c1"},
	}

	_ = filterQueryParams(input, "id_token_hint")

	// Original map must remain intact — filterQueryParams returns a copy.
	suite.Contains(input, "id_token_hint")
	suite.Equal([]string{"header.payload.sig"}, input["id_token_hint"])
	suite.Equal([]string{"c1"}, input["client_id"])
}

func clientWithPostLogout(uris ...string) *providers.OAuthClient {
	return &providers.OAuthClient{ID: "app-1", ClientID: "client-x", PostLogoutRedirectURIs: uris}
}

func makeIDToken(iss, aud string) string {
	enc := func(v interface{}) string {
		b, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	return enc(map[string]string{"alg": "RS256", "typ": "JWT"}) + "." +
		enc(map[string]interface{}{"iss": iss, "aud": aud}) + ".sig"
}

func makeIDTokenMultiAud(iss string, aud []string, azp string) string {
	enc := func(v interface{}) string {
		b, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	return enc(map[string]string{"alg": "RS256", "typ": "JWT"}) + "." +
		enc(map[string]interface{}{"iss": iss, "aud": aud, "azp": azp}) + ".sig"
}

func (suite *LogoutServiceTestSuite) TestResolve_RedirectWithoutIDTokenHintRejected() {
	svc, _, _ := suite.newService()

	_, err := svc.Resolve(context.Background(), LogoutRequest{
		ClientID: "client-x", PostLogoutRedirectURI: "https://rp.example/after",
	})

	suite.Require().ErrorIs(err, errIDTokenHintRequired)
}

func (suite *LogoutServiceTestSuite) TestResolve_UnregisteredRedirectRejected() {
	svc, jwtSvc, actor := suite.newService()
	token := makeIDToken(testIssuer, "client-x")
	jwtSvc.EXPECT().VerifyJWTSignature(mock.Anything, token).Return(nil)
	actor.EXPECT().GetOAuthClientByClientID(mock.Anything, "client-x").
		Return(clientWithPostLogout("https://rp.example/after"), nil)

	_, err := svc.Resolve(context.Background(), LogoutRequest{
		IDTokenHint: token, PostLogoutRedirectURI: "https://evil.example/steal",
	})

	suite.Require().ErrorIs(err, errInvalidPostLogoutRedirectURI)
}

func (suite *LogoutServiceTestSuite) TestResolve_ClientIDWithoutRedirect() {
	svc, _, actor := suite.newService()
	actor.EXPECT().GetOAuthClientByClientID(mock.Anything, "client-x").
		Return(clientWithPostLogout(), nil)

	res, err := svc.Resolve(context.Background(), LogoutRequest{ClientID: "client-x"})

	suite.Require().NoError(err)
	suite.Equal("app-1", res.AppID)
	suite.Empty(res.PostLogoutRedirectURI)
}

func (suite *LogoutServiceTestSuite) TestResolve_NoClientReference() {
	svc, _, _ := suite.newService()

	_, err := svc.Resolve(context.Background(), LogoutRequest{})

	suite.Require().ErrorIs(err, errClientRequired)
}

func (suite *LogoutServiceTestSuite) TestResolve_UnknownClient() {
	svc, _, actor := suite.newService()
	actor.EXPECT().GetOAuthClientByClientID(mock.Anything, "client-x").
		Return(nil, (*tidcommon.ServiceError)(nil))

	_, err := svc.Resolve(context.Background(), LogoutRequest{ClientID: "client-x"})

	suite.Require().ErrorIs(err, errInvalidClient)
}

func (suite *LogoutServiceTestSuite) TestResolve_IDTokenHintIdentifiesClient() {
	svc, jwtSvc, actor := suite.newService()
	token := makeIDToken(testIssuer, "client-x")
	jwtSvc.EXPECT().VerifyJWTSignature(mock.Anything, token).Return(nil)
	actor.EXPECT().GetOAuthClientByClientID(mock.Anything, "client-x").
		Return(clientWithPostLogout("https://rp.example/after"), nil)

	res, err := svc.Resolve(context.Background(), LogoutRequest{
		IDTokenHint: token, PostLogoutRedirectURI: "https://rp.example/after", State: "xyz",
	})

	suite.Require().NoError(err)
	suite.Equal("app-1", res.AppID)
	suite.Equal("https://rp.example/after", res.PostLogoutRedirectURI)
	suite.Equal("xyz", res.State)
}

func (suite *LogoutServiceTestSuite) TestResolve_IDTokenHintPrefersAzpForMultiAudience() {
	svc, jwtSvc, actor := suite.newService()
	token := makeIDTokenMultiAud(testIssuer, []string{"other-aud", "client-x"}, "client-x")
	jwtSvc.EXPECT().VerifyJWTSignature(mock.Anything, token).Return(nil)
	actor.EXPECT().GetOAuthClientByClientID(mock.Anything, "client-x").
		Return(clientWithPostLogout(), nil)

	res, err := svc.Resolve(context.Background(), LogoutRequest{IDTokenHint: token})

	suite.Require().NoError(err)
	suite.Equal("app-1", res.AppID)
}

func (suite *LogoutServiceTestSuite) TestResolve_ClientIDMismatchWithIDTokenHint() {
	svc, jwtSvc, _ := suite.newService()
	token := makeIDToken(testIssuer, "other-client")
	jwtSvc.EXPECT().VerifyJWTSignature(mock.Anything, token).Return(nil)

	_, err := svc.Resolve(context.Background(), LogoutRequest{ClientID: "client-x", IDTokenHint: token})

	suite.Require().ErrorIs(err, errClientMismatch)
}

func (suite *LogoutServiceTestSuite) TestResolve_IDTokenHintBadSignature() {
	svc, jwtSvc, _ := suite.newService()
	token := makeIDToken(testIssuer, "client-x")
	jwtSvc.EXPECT().VerifyJWTSignature(mock.Anything, token).
		Return(&tidcommon.ServiceError{Code: "bad", Type: tidcommon.ClientErrorType})

	_, err := svc.Resolve(context.Background(), LogoutRequest{IDTokenHint: token})

	suite.Require().ErrorIs(err, errInvalidIDTokenHint)
}

func (suite *LogoutServiceTestSuite) TestResolve_IDTokenHintWrongIssuer() {
	svc, jwtSvc, _ := suite.newService()
	token := makeIDToken("https://other.issuer", "client-x")
	jwtSvc.EXPECT().VerifyJWTSignature(mock.Anything, token).Return(nil)

	_, err := svc.Resolve(context.Background(), LogoutRequest{IDTokenHint: token})

	suite.Require().ErrorIs(err, errInvalidIDTokenHint)
}

func (suite *LogoutServiceTestSuite) TestResolve_IDTokenHintUndecodablePayload() {
	svc, jwtSvc, _ := suite.newService()
	// Signature verification passes (mocked), but the payload segment is not valid base64url JSON.
	token := "header.@@@notbase64@@@.sig"
	jwtSvc.EXPECT().VerifyJWTSignature(mock.Anything, token).Return(nil)

	_, err := svc.Resolve(context.Background(), LogoutRequest{IDTokenHint: token})

	suite.Require().ErrorIs(err, errInvalidIDTokenHint)
}

func (suite *LogoutServiceTestSuite) TestResolve_IDTokenHintMultiAudienceWithoutAzp() {
	svc, jwtSvc, actor := suite.newService()
	// No azp: the client is taken from the first aud entry.
	token := makeIDTokenMultiAud(testIssuer, []string{"client-x", "other-aud"}, "")
	jwtSvc.EXPECT().VerifyJWTSignature(mock.Anything, token).Return(nil)
	actor.EXPECT().GetOAuthClientByClientID(mock.Anything, "client-x").
		Return(clientWithPostLogout(), nil)

	res, err := svc.Resolve(context.Background(), LogoutRequest{IDTokenHint: token})

	suite.Require().NoError(err)
	suite.Equal("app-1", res.AppID)
}

func (suite *LogoutServiceTestSuite) TestResolve_ClientResolutionClientError() {
	svc, _, actor := suite.newService()
	actor.EXPECT().GetOAuthClientByClientID(mock.Anything, "client-x").
		Return(nil, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "no-client"})

	_, err := svc.Resolve(context.Background(), LogoutRequest{ClientID: "client-x"})

	suite.Require().ErrorIs(err, errInvalidClient)
}

func (suite *LogoutServiceTestSuite) TestResolve_ClientResolutionServerError() {
	svc, _, actor := suite.newService()
	actor.EXPECT().GetOAuthClientByClientID(mock.Anything, "client-x").
		Return(nil, &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "boom"})

	_, err := svc.Resolve(context.Background(), LogoutRequest{ClientID: "client-x"})

	suite.Require().ErrorIs(err, errInvalidClient)
}

func (suite *LogoutServiceTestSuite) TestInitiateSignOutFlow_StorePersistError() {
	store := newLogoutRequestStoreInterfaceMock(suite.T())
	store.EXPECT().AddRequest(mock.Anything, mock.Anything).
		Return("", fmt.Errorf("store down"))
	// The flow must not be initiated when the request cannot be persisted.
	svc := suite.newServiceWithStore(store, flowexecmock.NewFlowExecServiceInterfaceMock(suite.T()))

	initiation, svcErr := svc.InitiateSignOutFlow(context.Background(), &LogoutResolution{AppID: "app-1"})

	suite.Nil(initiation)
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.ServerErrorType, svcErr.Type)
}

func (suite *LogoutServiceTestSuite) TestCompleteSignOut_GetRequestError() {
	store := newLogoutRequestStoreInterfaceMock(suite.T())
	store.EXPECT().GetRequest(mock.Anything, "logout-1").
		Return(false, logoutRequestContext{}, fmt.Errorf("store down"))
	svc := suite.newServiceWithStore(store, flowexecmock.NewFlowExecServiceInterfaceMock(suite.T()))

	_, err := svc.CompleteSignOut(context.Background(), "logout-1")

	suite.Require().Error(err)
}

func (suite *LogoutServiceTestSuite) TestCompleteSignOut_ClearErrorStillReturnsRedirect() {
	store := newLogoutRequestStoreInterfaceMock(suite.T())
	store.EXPECT().GetRequest(mock.Anything, "logout-1").Return(true, logoutRequestContext{
		AppID: "app-1", PostLogoutRedirectURI: "https://rp.example/after",
	}, nil)
	// A best-effort clear failure is logged but must not fail the completion.
	store.EXPECT().ClearRequest(mock.Anything, "logout-1").Return(fmt.Errorf("clear failed"))
	svc := suite.newServiceWithStore(store, flowexecmock.NewFlowExecServiceInterfaceMock(suite.T()))

	redirectURI, err := svc.CompleteSignOut(context.Background(), "logout-1")

	suite.Require().NoError(err)
	suite.Equal("https://rp.example/after", redirectURI)
}
