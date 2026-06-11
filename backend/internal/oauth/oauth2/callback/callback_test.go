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

package callback

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	oauth2authz "github.com/thunder-id/thunderid/internal/oauth/oauth2/authz"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/ciba"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/authzmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/cibamock"
)

type CallbackDispatcherTestSuite struct {
	suite.Suite
	mockAuthZ  *authzmock.AuthorizeServiceInterfaceMock
	mockCIBA   *cibamock.CIBAServiceInterfaceMock
	dispatcher *callbackDispatcher
}

func TestCallbackDispatcherSuite(t *testing.T) {
	suite.Run(t, new(CallbackDispatcherTestSuite))
}

func (suite *CallbackDispatcherTestSuite) SetupTest() {
	suite.mockAuthZ = authzmock.NewAuthorizeServiceInterfaceMock(suite.T())
	suite.mockCIBA = cibamock.NewCIBAServiceInterfaceMock(suite.T())
	suite.dispatcher = newCallbackDispatcher(suite.mockAuthZ, suite.mockCIBA)

	_ = config.InitializeServerRuntime("test", &config.Config{
		JWT: config.JWTConfig{
			Issuer: "https://localhost:8090/oauth2",
		},
		GateClient: config.GateClientConfig{
			Scheme:    "https",
			Hostname:  "localhost",
			Port:      3000,
			ErrorPath: "/error",
		},
	})
}

func (suite *CallbackDispatcherTestSuite) postCallback(body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/oauth2/auth/callback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.dispatcher.handleFlowCallback(w, req)
	return w
}

// --- handleFlowCallback: empty / malformed body ---

func (suite *CallbackDispatcherTestSuite) TestHandleFlowCallback_EmptyBody_ReturnsBadRequest() {
	w := suite.postCallback("")
	suite.Equal(http.StatusBadRequest, w.Code)
	var body map[string]string
	suite.NoError(json.NewDecoder(w.Body).Decode(&body))
	suite.Equal(oauth2const.ErrorInvalidRequest, body["error"])
}

func (suite *CallbackDispatcherTestSuite) TestHandleFlowCallback_InvalidJSON_ReturnsBadRequest() {
	w := suite.postCallback("not-json{{{")
	suite.Equal(http.StatusBadRequest, w.Code)
	var body map[string]string
	suite.NoError(json.NewDecoder(w.Body).Decode(&body))
	suite.Equal(oauth2const.ErrorInvalidRequest, body["error"])
}

// --- handleFlowCallback: missing required fields ---

func (suite *CallbackDispatcherTestSuite) TestHandleFlowCallback_MissingAuthID_ReturnsBadRequest() {
	w := suite.postCallback(`{"assertion":"some-jwt"}`)
	suite.Equal(http.StatusBadRequest, w.Code)
	var body map[string]string
	suite.NoError(json.NewDecoder(w.Body).Decode(&body))
	suite.Equal(oauth2const.ErrorInvalidRequest, body["error"])
}

func (suite *CallbackDispatcherTestSuite) TestHandleFlowCallback_MissingAssertion_ReturnsBadRequest() {
	w := suite.postCallback(`{"authId":"auth-1"}`)
	suite.Equal(http.StatusBadRequest, w.Code)
	var body map[string]string
	suite.NoError(json.NewDecoder(w.Body).Decode(&body))
	suite.Equal(oauth2const.ErrorInvalidRequest, body["error"])
}

// --- handleFlowCallback: authorization_code path ---

func (suite *CallbackDispatcherTestSuite) TestHandleFlowCallback_AuthCode_DefaultType_Success() {
	suite.mockAuthZ.EXPECT().
		HandleAuthorizationCallback(mock.Anything, "auth-1", "the-assertion").
		Return("https://client.example.com/cb?code=xyz", nil)

	w := suite.postCallback(`{"authId":"auth-1","assertion":"the-assertion"}`)

	suite.Equal(http.StatusOK, w.Code)
	var resp oauth2authz.AuthZPostResponse
	suite.NoError(json.NewDecoder(w.Body).Decode(&resp))
	suite.Equal("https://client.example.com/cb?code=xyz", resp.RedirectURI)
}

func (suite *CallbackDispatcherTestSuite) TestHandleFlowCallback_AuthCode_ExplicitType_Success() {
	suite.mockAuthZ.EXPECT().
		HandleAuthorizationCallback(mock.Anything, "auth-1", "the-assertion").
		Return("https://client.example.com/cb?code=xyz", nil)

	w := suite.postCallback(`{"authId":"auth-1","assertion":"the-assertion","type":"authorization_code"}`)

	suite.Equal(http.StatusOK, w.Code)
	var resp oauth2authz.AuthZPostResponse
	suite.NoError(json.NewDecoder(w.Body).Decode(&resp))
	suite.Contains(resp.RedirectURI, "code=xyz")
}

func (suite *CallbackDispatcherTestSuite) TestHandleFlowCallback_AuthCode_ErrorSentToClient_WithState() {
	authErr := &oauth2authz.AuthorizationError{
		Code:              oauth2const.ErrorAccessDenied,
		Message:           "user denied",
		SendErrorToClient: true,
		ClientRedirectURI: "https://client.example.com/cb",
		State:             "state-abc",
	}
	suite.mockAuthZ.EXPECT().
		HandleAuthorizationCallback(mock.Anything, "auth-1", "the-assertion").
		Return("", authErr)

	w := suite.postCallback(`{"authId":"auth-1","assertion":"the-assertion"}`)

	suite.Equal(http.StatusOK, w.Code)
	var resp oauth2authz.AuthZPostResponse
	suite.NoError(json.NewDecoder(w.Body).Decode(&resp))
	suite.Contains(resp.RedirectURI, "error="+oauth2const.ErrorAccessDenied)
	suite.Contains(resp.RedirectURI, "state=state-abc")
}

func (suite *CallbackDispatcherTestSuite) TestHandleFlowCallback_AuthCode_ErrorSentToClient_NoState() {
	authErr := &oauth2authz.AuthorizationError{
		Code:              oauth2const.ErrorAccessDenied,
		Message:           "user denied",
		SendErrorToClient: true,
		ClientRedirectURI: "https://client.example.com/cb",
		State:             "",
	}
	suite.mockAuthZ.EXPECT().
		HandleAuthorizationCallback(mock.Anything, "auth-1", "the-assertion").
		Return("", authErr)

	w := suite.postCallback(`{"authId":"auth-1","assertion":"the-assertion"}`)

	suite.Equal(http.StatusOK, w.Code)
	var resp oauth2authz.AuthZPostResponse
	suite.NoError(json.NewDecoder(w.Body).Decode(&resp))
	suite.Contains(resp.RedirectURI, "error="+oauth2const.ErrorAccessDenied)
	suite.NotContains(resp.RedirectURI, "state=")
}

func (suite *CallbackDispatcherTestSuite) TestHandleFlowCallback_AuthCode_ErrorPage_WithState() {
	authErr := &oauth2authz.AuthorizationError{
		Code:              oauth2const.ErrorServerError,
		Message:           "internal error",
		SendErrorToClient: false,
		State:             "mystate",
	}
	suite.mockAuthZ.EXPECT().
		HandleAuthorizationCallback(mock.Anything, "auth-1", "the-assertion").
		Return("", authErr)

	w := suite.postCallback(`{"authId":"auth-1","assertion":"the-assertion"}`)

	suite.Equal(http.StatusOK, w.Code)
	var resp oauth2authz.AuthZPostResponse
	suite.NoError(json.NewDecoder(w.Body).Decode(&resp))
	suite.Contains(resp.RedirectURI, "errorCode="+oauth2const.ErrorServerError)
	suite.Contains(resp.RedirectURI, "state=mystate")
}

func (suite *CallbackDispatcherTestSuite) TestHandleFlowCallback_AuthCode_ErrorPage_NoState() {
	authErr := &oauth2authz.AuthorizationError{
		Code:              oauth2const.ErrorServerError,
		Message:           "internal error",
		SendErrorToClient: false,
		State:             "",
	}
	suite.mockAuthZ.EXPECT().
		HandleAuthorizationCallback(mock.Anything, "auth-1", "the-assertion").
		Return("", authErr)

	w := suite.postCallback(`{"authId":"auth-1","assertion":"the-assertion"}`)

	suite.Equal(http.StatusOK, w.Code)
	var resp oauth2authz.AuthZPostResponse
	suite.NoError(json.NewDecoder(w.Body).Decode(&resp))
	suite.Contains(resp.RedirectURI, "errorCode="+oauth2const.ErrorServerError)
	suite.NotContains(resp.RedirectURI, "state=")
}

// --- handleFlowCallback: CIBA path ---

func (suite *CallbackDispatcherTestSuite) TestHandleFlowCallback_CIBA_Success() {
	suite.mockCIBA.EXPECT().
		HandleCallback(mock.Anything, "auth-req-1", "ciba-assertion").
		Return(nil)

	w := suite.postCallback(
		`{"authId":"auth-req-1","assertion":"ciba-assertion","type":"urn:openid:params:grant-type:ciba"}`)

	suite.Equal(http.StatusOK, w.Code)
	var body map[string]string
	suite.NoError(json.NewDecoder(w.Body).Decode(&body))
	suite.Equal("OK", body["status"])
}

func (suite *CallbackDispatcherTestSuite) TestHandleFlowCallback_CIBA_Error() {
	suite.mockCIBA.EXPECT().
		HandleCallback(mock.Anything, "auth-req-1", "bad-assertion").
		Return(&ciba.CIBAError{Code: oauth2const.ErrorAccessDenied, Message: "denied"})

	w := suite.postCallback(
		`{"authId":"auth-req-1","assertion":"bad-assertion","type":"urn:openid:params:grant-type:ciba"}`)

	suite.Equal(http.StatusBadRequest, w.Code)
	var body map[string]string
	suite.NoError(json.NewDecoder(w.Body).Decode(&body))
	suite.Equal(oauth2const.ErrorAccessDenied, body["error"])
}

// --- handleFlowCallback: unsupported type ---

func (suite *CallbackDispatcherTestSuite) TestHandleFlowCallback_UnsupportedType_ReturnsBadRequest() {
	w := suite.postCallback(`{"authId":"auth-1","assertion":"some-jwt","type":"unknown_grant"}`)

	suite.Equal(http.StatusBadRequest, w.Code)
	var body map[string]string
	suite.NoError(json.NewDecoder(w.Body).Decode(&body))
	suite.Equal(oauth2const.ErrorInvalidRequest, body["error"])
	suite.Contains(body["error_description"], "Unsupported callback type")
}

// --- writeRedirectWithError ---

func (suite *CallbackDispatcherTestSuite) TestWriteRedirectWithError_WithState() {
	authErr := &oauth2authz.AuthorizationError{
		Code:              oauth2const.ErrorInvalidRequest,
		Message:           "bad params",
		SendErrorToClient: true,
		ClientRedirectURI: "https://client.example.com/cb",
		State:             "abc123",
	}
	w := httptest.NewRecorder()
	suite.dispatcher.writeRedirectWithError(context.Background(), w, authErr)

	suite.Equal(http.StatusOK, w.Code)
	var resp oauth2authz.AuthZPostResponse
	suite.NoError(json.NewDecoder(w.Body).Decode(&resp))
	suite.Contains(resp.RedirectURI, "state=abc123")
	suite.Contains(resp.RedirectURI, "error="+oauth2const.ErrorInvalidRequest)
}

func (suite *CallbackDispatcherTestSuite) TestWriteRedirectWithError_WithoutState() {
	authErr := &oauth2authz.AuthorizationError{
		Code:              oauth2const.ErrorInvalidRequest,
		Message:           "bad params",
		SendErrorToClient: true,
		ClientRedirectURI: "https://client.example.com/cb",
		State:             "",
	}
	w := httptest.NewRecorder()
	suite.dispatcher.writeRedirectWithError(context.Background(), w, authErr)

	suite.Equal(http.StatusOK, w.Code)
	var resp oauth2authz.AuthZPostResponse
	suite.NoError(json.NewDecoder(w.Body).Decode(&resp))
	suite.NotContains(resp.RedirectURI, "state=")
	suite.Contains(resp.RedirectURI, "error="+oauth2const.ErrorInvalidRequest)
}

func (suite *CallbackDispatcherTestSuite) TestWriteRedirectWithError_URIConstructionError_FallsBackToErrorPage() {
	// Inject an invalid error code character so GetURIWithQueryParams returns an error,
	// exercising the fallback path that calls writeErrorPageRedirect.
	authErr := &oauth2authz.AuthorizationError{
		Code:              "invalid\x22code",
		Message:           "bad",
		SendErrorToClient: true,
		ClientRedirectURI: "https://client.example.com/cb",
		State:             "s1",
	}
	w := httptest.NewRecorder()
	suite.dispatcher.writeRedirectWithError(context.Background(), w, authErr)

	suite.Equal(http.StatusOK, w.Code)
	var resp oauth2authz.AuthZPostResponse
	suite.NoError(json.NewDecoder(w.Body).Decode(&resp))
	suite.Contains(resp.RedirectURI, "/error")
}

// --- writeErrorPageRedirect ---

func (suite *CallbackDispatcherTestSuite) TestWriteErrorPageRedirect_WithState() {
	w := httptest.NewRecorder()
	suite.dispatcher.writeErrorPageRedirect(
		context.Background(),
		w,
		oauth2const.ErrorServerError,
		"something broke",
		"stateXYZ")

	suite.Equal(http.StatusOK, w.Code)
	var resp oauth2authz.AuthZPostResponse
	suite.NoError(json.NewDecoder(w.Body).Decode(&resp))
	suite.Contains(resp.RedirectURI, "errorCode="+oauth2const.ErrorServerError)
	suite.Contains(resp.RedirectURI, "state=stateXYZ")
}

func (suite *CallbackDispatcherTestSuite) TestWriteErrorPageRedirect_WithoutState() {
	w := httptest.NewRecorder()
	suite.dispatcher.writeErrorPageRedirect(
		context.Background(),
		w,
		oauth2const.ErrorServerError,
		"something broke",
		"")

	suite.Equal(http.StatusOK, w.Code)
	var resp oauth2authz.AuthZPostResponse
	suite.NoError(json.NewDecoder(w.Body).Decode(&resp))
	suite.Contains(resp.RedirectURI, "errorCode="+oauth2const.ErrorServerError)
	suite.NotContains(resp.RedirectURI, "state=")
}
