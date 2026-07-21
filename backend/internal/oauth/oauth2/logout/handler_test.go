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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/system/config"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/actorprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/flowexecmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

type LogoutHandlerTestSuite struct {
	suite.Suite
}

func TestLogoutHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(LogoutHandlerTestSuite))
}

func (suite *LogoutHandlerTestSuite) SetupTest() {
	suite.Require().NoError(config.InitializeServerRuntime(suite.T().TempDir(), &config.Config{}))
}

func (suite *LogoutHandlerTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func gateConfig() oauthconfig.Config {
	return oauthconfig.Config{
		GateClient: engineconfig.GateClientConfig{
			Scheme:      "https",
			Hostname:    "gate.example",
			Port:        9443,
			SignOutPath: "/signout",
		},
	}
}

func (suite *LogoutHandlerTestSuite) TestHandleLogout_InitiatesFlowAndRedirectsToGate() {
	actor := actorprovidermock.NewActorProviderMock(suite.T())
	actor.EXPECT().GetOAuthClientByClientID(mock.Anything, "client-x").
		Return(clientWithPostLogout("https://rp.example/after"), nil)

	flowSvc := flowexecmock.NewFlowExecServiceInterfaceMock(suite.T())
	var capturedRuntime map[string]string
	flowSvc.EXPECT().InitiateFlow(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, ic *flowexec.FlowInitContext) (string, *tidcommon.ServiceError) {
			suite.Equal("app-1", ic.ApplicationID)
			suite.Equal("SIGNOUT", ic.FlowType)
			capturedRuntime = ic.RuntimeData
			return "exec-1", nil
		})

	// A post_logout_redirect_uri is only honored with an id_token_hint, so drive the flow with one.
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(suite.T())
	token := makeIDToken(testIssuer, "client-x")
	jwtSvc.EXPECT().VerifyJWTSignature(mock.Anything, token).Return(nil)

	store := newLogoutRequestStoreInterfaceMock(suite.T())
	store.EXPECT().AddRequest(mock.Anything, mock.Anything).Return("logout-1", nil)
	handler := newLogoutHandler(
		newLogoutService(jwtSvc, actor, flowSvc, store, testIssuer), gateConfig())

	req := httptest.NewRequest(http.MethodGet,
		"/oauth2/logout?id_token_hint="+token+"&post_logout_redirect_uri=https://rp.example/after&state=xyz", nil)
	rec := httptest.NewRecorder()

	handler.HandleLogout(rec, req)

	suite.Equal(http.StatusFound, rec.Code)
	location := rec.Header().Get("Location")
	suite.Contains(location, "https://gate.example:9443/signout")
	suite.Contains(location, "applicationId=app-1")
	suite.Contains(location, "executionId=exec-1")
	suite.Contains(location, "logoutId=", "the gate needs the logout id to complete on callback")

	// The post-logout target is kept in the OAuth layer, not threaded through the flow.
	suite.Empty(capturedRuntime, "the sign-out flow must not carry post-logout runtime data")
}

func (suite *LogoutHandlerTestSuite) TestHandleLogout_InvalidRequestRejected() {
	// No client_id and no id_token_hint: the request cannot be resolved and must be rejected before
	// any flow is initiated or redirect happens.
	actor := actorprovidermock.NewActorProviderMock(suite.T())
	flowSvc := flowexecmock.NewFlowExecServiceInterfaceMock(suite.T())
	// The request is rejected during resolution, so the store is never touched.
	svc := newLogoutService(jwtmock.NewJWTServiceInterfaceMock(suite.T()), actor, flowSvc,
		newLogoutRequestStoreInterfaceMock(suite.T()), testIssuer)
	handler := newLogoutHandler(svc, gateConfig())

	req := httptest.NewRequest(http.MethodGet, "/oauth2/logout", nil)
	rec := httptest.NewRecorder()

	handler.HandleLogout(rec, req)

	suite.Equal(http.StatusBadRequest, rec.Code)
}

// A sign-out flow initiation failure maps to the HTTP status implied by the service error type
// (client -> 400, server -> 500) and does not redirect. Also exercises both GET and POST.
func (suite *LogoutHandlerTestSuite) TestHandleLogout_FlowInitiationError_MapsStatus() {
	cases := []struct {
		name       string
		method     string
		errType    tidcommon.ServiceErrorType
		wantStatus int
	}{
		{"client error -> 400", http.MethodGet, tidcommon.ClientErrorType, http.StatusBadRequest},
		{"server error -> 500", http.MethodPost, tidcommon.ServerErrorType, http.StatusInternalServerError},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			actor := actorprovidermock.NewActorProviderMock(suite.T())
			actor.EXPECT().GetOAuthClientByClientID(mock.Anything, "client-x").
				Return(clientWithPostLogout(), nil)
			flowSvc := flowexecmock.NewFlowExecServiceInterfaceMock(suite.T())
			flowSvc.EXPECT().InitiateFlow(mock.Anything, mock.Anything).Return("",
				&tidcommon.ServiceError{Type: tc.errType, Error: tidcommon.I18nMessage{DefaultValue: "flow boom"}})
			// The request is persisted before the flow is initiated; the flow init is what fails here.
			store := newLogoutRequestStoreInterfaceMock(suite.T())
			store.EXPECT().AddRequest(mock.Anything, mock.Anything).Return("logout-1", nil)
			svc := newLogoutService(jwtmock.NewJWTServiceInterfaceMock(suite.T()), actor, flowSvc,
				store, testIssuer)
			handler := newLogoutHandler(svc, gateConfig())

			req := httptest.NewRequest(tc.method, "/oauth2/logout?client_id=client-x", nil)
			rec := httptest.NewRecorder()

			handler.HandleLogout(rec, req)

			suite.Equal(tc.wantStatus, rec.Code)
			suite.Empty(rec.Header().Get("Location"), "must not redirect when flow initiation fails")
		})
	}
}

// A POST to the end_session_endpoint initiates the flow and redirects to the gate, exactly like GET.
func (suite *LogoutHandlerTestSuite) TestHandleLogout_POSTInitiatesAndRedirects() {
	actor := actorprovidermock.NewActorProviderMock(suite.T())
	actor.EXPECT().GetOAuthClientByClientID(mock.Anything, "client-x").Return(clientWithPostLogout(), nil)
	flowSvc := flowexecmock.NewFlowExecServiceInterfaceMock(suite.T())
	flowSvc.EXPECT().InitiateFlow(mock.Anything, mock.Anything).Return("exec-2", nil)
	store := newLogoutRequestStoreInterfaceMock(suite.T())
	store.EXPECT().AddRequest(mock.Anything, mock.Anything).Return("logout-1", nil)
	svc := newLogoutService(jwtmock.NewJWTServiceInterfaceMock(suite.T()), actor, flowSvc,
		store, testIssuer)
	handler := newLogoutHandler(svc, gateConfig())

	req := httptest.NewRequest(http.MethodPost, "/oauth2/logout?client_id=client-x", nil)
	rec := httptest.NewRecorder()

	handler.HandleLogout(rec, req)

	suite.Equal(http.StatusFound, rec.Code)
	location := rec.Header().Get("Location")
	suite.Contains(location, "https://gate.example:9443/signout")
	suite.Contains(location, "executionId=exec-2")
}

// The completion callback consumes the stored logout request and returns the post-logout redirect URI.
func (suite *LogoutHandlerTestSuite) TestHandleLogoutCallback_ReturnsRedirect() {
	store := newLogoutRequestStoreInterfaceMock(suite.T())
	store.EXPECT().GetRequest(mock.Anything, "logout-1").Return(true, logoutRequestContext{
		AppID: "app-1", PostLogoutRedirectURI: "https://rp.example/after", State: "xyz",
	}, nil)
	store.EXPECT().ClearRequest(mock.Anything, "logout-1").Return(nil)
	svc := newLogoutService(jwtmock.NewJWTServiceInterfaceMock(suite.T()),
		actorprovidermock.NewActorProviderMock(suite.T()),
		flowexecmock.NewFlowExecServiceInterfaceMock(suite.T()), store, testIssuer)
	handler := newLogoutHandler(svc, gateConfig())

	req := httptest.NewRequest(http.MethodPost, "/oauth2/logout/callback",
		strings.NewReader(`{"logoutId":"logout-1"}`))
	rec := httptest.NewRecorder()

	handler.HandleLogoutCallback(rec, req)

	suite.Equal(http.StatusOK, rec.Code)
	var body struct {
		RedirectURI string `json:"redirect_uri"`
	}
	suite.Require().NoError(json.NewDecoder(rec.Body).Decode(&body))
	suite.Contains(body.RedirectURI, "https://rp.example/after")
	suite.Contains(body.RedirectURI, "state=xyz")
}

func (suite *LogoutHandlerTestSuite) TestHandleLogout_MalformedFormRejected() {
	// An unparseable query string (invalid percent-encoding) fails ParseForm before anything else.
	svc := newLogoutService(jwtmock.NewJWTServiceInterfaceMock(suite.T()),
		actorprovidermock.NewActorProviderMock(suite.T()),
		flowexecmock.NewFlowExecServiceInterfaceMock(suite.T()),
		newLogoutRequestStoreInterfaceMock(suite.T()), testIssuer)
	handler := newLogoutHandler(svc, gateConfig())

	req := httptest.NewRequest(http.MethodGet, "/oauth2/logout?%zz", nil)
	rec := httptest.NewRecorder()

	handler.HandleLogout(rec, req)

	suite.Equal(http.StatusBadRequest, rec.Code)
}

// A completion-callback store failure maps to 500 and returns no redirect.
func (suite *LogoutHandlerTestSuite) TestHandleLogoutCallback_CompletionError() {
	store := newLogoutRequestStoreInterfaceMock(suite.T())
	store.EXPECT().GetRequest(mock.Anything, "logout-1").
		Return(false, logoutRequestContext{}, fmt.Errorf("store down"))
	svc := newLogoutService(jwtmock.NewJWTServiceInterfaceMock(suite.T()),
		actorprovidermock.NewActorProviderMock(suite.T()),
		flowexecmock.NewFlowExecServiceInterfaceMock(suite.T()), store, testIssuer)
	handler := newLogoutHandler(svc, gateConfig())

	req := httptest.NewRequest(http.MethodPost, "/oauth2/logout/callback",
		strings.NewReader(`{"logoutId":"logout-1"}`))
	rec := httptest.NewRecorder()

	handler.HandleLogoutCallback(rec, req)

	suite.Equal(http.StatusInternalServerError, rec.Code)
}

func (suite *LogoutHandlerTestSuite) TestHandleLogoutCallback_InvalidRequest() {
	// The body carries no logout id, so it is rejected before the store is consulted.
	svc := newLogoutService(jwtmock.NewJWTServiceInterfaceMock(suite.T()),
		actorprovidermock.NewActorProviderMock(suite.T()),
		flowexecmock.NewFlowExecServiceInterfaceMock(suite.T()),
		newLogoutRequestStoreInterfaceMock(suite.T()), testIssuer)
	handler := newLogoutHandler(svc, gateConfig())

	req := httptest.NewRequest(http.MethodPost, "/oauth2/logout/callback", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	handler.HandleLogoutCallback(rec, req)

	suite.Equal(http.StatusBadRequest, rec.Code)
}
