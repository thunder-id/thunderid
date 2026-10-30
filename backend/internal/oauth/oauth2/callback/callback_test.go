// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package callback

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	oauth2authz "github.com/thunder-id/thunderid/internal/oauth/oauth2/authz"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/ciba"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/authzmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/cibamock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/deviceauthmock"
	"github.com/thunder-id/thunderid/tests/testhelpers"
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
	suite.dispatcher = newCallbackDispatcher(testhelpers.OAuthConfig(), suite.mockAuthZ, suite.mockCIBA, nil)

	_ = config.InitializeServerRuntime("test", &config.Config{
		JWT: engineconfig.JWTConfig{
			Issuer: "https://localhost:8090/oauth2",
		},
		GateClient: engineconfig.GateClientConfig{
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

func (suite *CallbackDispatcherTestSuite) TestHandleFlowCallback_OversizedBody_ReturnsBadRequest() {
	w := suite.postCallback(`{"authId":"auth-1","assertion":"` + strings.Repeat("a", maxCallbackBodyBytes) + `"}`)

	suite.Equal(http.StatusBadRequest, w.Code)
	suite.mockAuthZ.AssertNotCalled(suite.T(), "HandleAuthorizationCallback",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

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
		HandleAuthorizationCallback(mock.Anything, "auth-1", "the-assertion", mock.Anything).
		Return("https://client.example.com/cb?code=xyz", nil)

	w := suite.postCallback(`{"authId":"auth-1","assertion":"the-assertion"}`)

	suite.Equal(http.StatusOK, w.Code)
	var resp oauth2authz.AuthZPostResponse
	suite.NoError(json.NewDecoder(w.Body).Decode(&resp))
	suite.Equal("https://client.example.com/cb?code=xyz", resp.RedirectURI)
}

func (suite *CallbackDispatcherTestSuite) TestHandleFlowCallback_AuthCode_PassesSessionData() {
	suite.mockAuthZ.EXPECT().
		HandleAuthorizationCallback(mock.Anything, "auth-1", "the-assertion", []oauth2model.SessionData{
			{FlowIdentifier: "f51bdb0f22d906bb", SessionHandle: "handle-1"},
		}).
		Return("https://client.example.com/cb?code=xyz", nil)

	w := suite.postCallback(`{"authId":"auth-1","assertion":"the-assertion",` +
		`"sessionData":[{"flowIdentifier":"f51bdb0f22d906bb","sessionHandle":"handle-1"}]}`)

	suite.Equal(http.StatusOK, w.Code)
}

func (suite *CallbackDispatcherTestSuite) TestHandleFlowCallback_AuthCode_ExplicitType_Success() {
	suite.mockAuthZ.EXPECT().
		HandleAuthorizationCallback(mock.Anything, "auth-1", "the-assertion", mock.Anything).
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
		HandleAuthorizationCallback(mock.Anything, "auth-1", "the-assertion", mock.Anything).
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
		HandleAuthorizationCallback(mock.Anything, "auth-1", "the-assertion", mock.Anything).
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
		HandleAuthorizationCallback(mock.Anything, "auth-1", "the-assertion", mock.Anything).
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
		HandleAuthorizationCallback(mock.Anything, "auth-1", "the-assertion", mock.Anything).
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

func (suite *CallbackDispatcherTestSuite) TestHandleFlowCallback_CIBA_NilCIBAService_ReturnsBadRequest() {
	// When the CIBA grant type is not in allowed_grant_types, cibaService is nil. A CIBA
	// callback must be rejected gracefully instead of panicking on the nil service.
	suite.dispatcher = newCallbackDispatcher(testhelpers.OAuthConfig(), suite.mockAuthZ, nil, nil)

	w := suite.postCallback(
		`{"authId":"auth-req-1","assertion":"ciba-assertion","type":"urn:openid:params:grant-type:ciba"}`)

	suite.Equal(http.StatusBadRequest, w.Code)
	var body map[string]string
	suite.NoError(json.NewDecoder(w.Body).Decode(&body))
	suite.Equal(oauth2const.ErrorInvalidRequest, body["error"])
	suite.Contains(body["error_description"], "Unsupported callback type")
}

func (suite *CallbackDispatcherTestSuite) TestHandleFlowCallback_DeviceGrant_Success() {
	deviceService := deviceauthmock.NewDeviceAuthServiceInterfaceMock(suite.T())
	deviceService.EXPECT().HandleCallback(mock.Anything, "device-request-1", "device-assertion").Return(nil)
	suite.dispatcher = newCallbackDispatcher(
		testhelpers.OAuthConfig(), suite.mockAuthZ, suite.mockCIBA, deviceService)

	w := suite.postCallback(
		`{"authId":"device-request-1","assertion":"device-assertion",` +
			`"type":"urn:ietf:params:oauth:grant-type:device_code"}`)

	suite.Equal(http.StatusOK, w.Code)
	var body map[string]string
	suite.NoError(json.NewDecoder(w.Body).Decode(&body))
	suite.Equal("OK", body["status"])
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

// --- handleFlowCallback: failure outcomes ---
//
// Success and failure assertions share the assertion field, and the dispatcher passes it through
// opaquely, so a failure produces exactly the same routing as any other error from the grant-type
// handler (covered above). Which branch the assertion takes is decided inside the services, and is
// tested in authz/service_test.go and ciba/service_test.go. Only the outcomes those tests cannot
// reach from the service layer live here.

// TestHandleFlowCallback_SuppressedServerErrorNeverContactsClient covers the dispatcher half of
// oauth.send_server_errors_to_client: a server_error that is not for the client goes to the error
// page and the client redirect URI, though present on the error, is not used.
func (suite *CallbackDispatcherTestSuite) TestHandleFlowCallback_SuppressedServerErrorNeverContactsClient() {
	authErr := &oauth2authz.AuthorizationError{
		Code:              oauth2const.ErrorServerError,
		Message:           "Flow engine failure",
		SendErrorToClient: false,
		ClientRedirectURI: "https://client.example.com/cb",
		State:             "state-abc",
	}
	suite.mockAuthZ.EXPECT().
		HandleAuthorizationCallback(mock.Anything, "auth-1", "the-assertion", mock.Anything).
		Return("", authErr)

	w := suite.postCallback(`{"authId":"auth-1","assertion":"the-assertion"}`)

	suite.Equal(http.StatusOK, w.Code)
	var resp oauth2authz.AuthZPostResponse
	suite.NoError(json.NewDecoder(w.Body).Decode(&resp))
	suite.Contains(resp.RedirectURI, "/error")
	suite.Contains(resp.RedirectURI, "errorCode="+oauth2const.ErrorServerError)
	suite.NotContains(resp.RedirectURI, "client.example.com")
}

// TestHandleFlowCallback_CIBA_ServerError_Returns500 pins the status mapping a CIBA server-side flow
// failure relies on: the polling client must see a 500, not the 400 every other CIBA error produces.
func (suite *CallbackDispatcherTestSuite) TestHandleFlowCallback_CIBA_ServerError_Returns500() {
	suite.mockCIBA.EXPECT().
		HandleCallback(mock.Anything, "auth-req-1", "the-assertion").
		Return(&ciba.CIBAError{Code: oauth2const.ErrorServerError, Message: "flow failed"})

	w := suite.postCallback(
		`{"authId":"auth-req-1","assertion":"the-assertion","type":"urn:openid:params:grant-type:ciba"}`)

	suite.Equal(http.StatusInternalServerError, w.Code)
	var body map[string]string
	suite.NoError(json.NewDecoder(w.Body).Decode(&body))
	suite.Equal(oauth2const.ErrorServerError, body["error"])
	// The CIBA client is never redirected; it learns the outcome by polling the token endpoint.
	suite.mockAuthZ.AssertNotCalled(suite.T(), "HandleAuthorizationCallback",
		mock.Anything, mock.Anything, mock.Anything)
}
