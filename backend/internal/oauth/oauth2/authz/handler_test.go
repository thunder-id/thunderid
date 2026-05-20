/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package authz

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/system/config"
)

const (
	testAuthID = "test-auth-id"
)

type AuthorizeHandlerTestSuite struct {
	suite.Suite
	handler          *authorizeHandler
	mockAuthzService *AuthorizeServiceInterfaceMock
}

func TestAuthorizeHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(AuthorizeHandlerTestSuite))
}

func (suite *AuthorizeHandlerTestSuite) BeforeTest(suiteName, testName string) {
	config.ResetServerRuntime()

	testConfig := &config.Config{
		GateClient: config.GateClientConfig{
			Scheme:    "https",
			Hostname:  "localhost",
			Port:      3000,
			LoginPath: "/login",
			ErrorPath: "/error",
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
		JWT: config.JWTConfig{
			Issuer: "https://localhost:8090",
		},
		OAuth: config.OAuthConfig{
			AuthorizationCode: config.AuthorizationCodeConfig{
				ValidityPeriod: 600,
			},
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)
}

func (suite *AuthorizeHandlerTestSuite) SetupTest() {
	suite.mockAuthzService = NewAuthorizeServiceInterfaceMock(suite.T())
	suite.handler = newAuthorizeHandler(suite.mockAuthzService).(*authorizeHandler)
}

func (suite *AuthorizeHandlerTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *AuthorizeHandlerTestSuite) TestnewAuthorizeHandler() {
	mockSvc := NewAuthorizeServiceInterfaceMock(suite.T())
	handler := newAuthorizeHandler(mockSvc)
	assert.NotNil(suite.T(), handler)
	assert.Implements(suite.T(), (*AuthorizeHandlerInterface)(nil), handler)
}

func (suite *AuthorizeHandlerTestSuite) TestGetOAuthMessageForGetRequest_Success() {
	req := httptest.NewRequest(http.MethodGet, "/auth?client_id=test-client&redirect_uri=https://example.com", nil)

	msg, err := suite.handler.getOAuthMessageForGetRequest(req)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), msg)
	if msg != nil {
		assert.Equal(suite.T(), oauth2const.TypeInitialAuthorizationRequest, msg.RequestType)
		assert.Equal(suite.T(), "test-client", msg.RequestQueryParams["client_id"])
		assert.Equal(suite.T(), "https://example.com", msg.RequestQueryParams["redirect_uri"])
		assert.Empty(suite.T(), msg.AuthID)
	}
}

func (suite *AuthorizeHandlerTestSuite) TestGetOAuthMessageForGetRequest_ParseFormError() {
	req := httptest.NewRequest(http.MethodGet, "/auth?client_id=%ZZ", nil)

	msg, err := suite.handler.getOAuthMessageForGetRequest(req)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), msg)
	assert.Contains(suite.T(), err.Error(), "failed to parse form data")
}

func (suite *AuthorizeHandlerTestSuite) TestGetOAuthMessageForGetRequest_WithClaimsLocales() {
	req := httptest.NewRequest(http.MethodGet,
		"/auth?client_id=test-client&redirect_uri=https://example.com&claims_locales=en-US%20fr-CA%20ja", nil)

	msg, err := suite.handler.getOAuthMessageForGetRequest(req)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), msg)
	if msg != nil {
		assert.Equal(suite.T(), "test-client", msg.RequestQueryParams["client_id"])
		assert.Equal(suite.T(), "en-US fr-CA ja", msg.RequestQueryParams["claims_locales"])
	}
}

// §8 — only the resource parameter is permitted to be repeated.

func (suite *AuthorizeHandlerTestSuite) TestGetOAuthMessageForGetRequest_DuplicateNonResourceParam() {
	// Repeated client_id must be rejected with an error.
	req := httptest.NewRequest(http.MethodGet, "/auth?client_id=a&client_id=b", nil)

	msg, err := suite.handler.getOAuthMessageForGetRequest(req)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), msg)
	assert.Contains(suite.T(), err.Error(), "client_id")
}

func (suite *AuthorizeHandlerTestSuite) TestGetOAuthMessageForGetRequest_MultipleResourceValues() {
	// Repeated resource is allowed; both values must appear in msg.Resources.
	req := httptest.NewRequest(http.MethodGet,
		"/auth?client_id=test-client&resource=https%3A%2F%2Frs1.example.com&resource=https%3A%2F%2Frs2.example.com",
		nil)

	msg, err := suite.handler.getOAuthMessageForGetRequest(req)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), msg)
	if msg != nil {
		assert.Equal(suite.T(), 2, len(msg.Resources))
		assert.Contains(suite.T(), msg.Resources, "https://rs1.example.com")
		assert.Contains(suite.T(), msg.Resources, "https://rs2.example.com")
		assert.Equal(suite.T(), "test-client", msg.RequestQueryParams["client_id"])
	}
}

func (suite *AuthorizeHandlerTestSuite) TestGetOAuthMessageForGetRequest_DuplicateScopeParam() {
	// Repeated scope must be rejected — only resource may repeat.
	req := httptest.NewRequest(http.MethodGet,
		"/auth?client_id=test-client&resource=https%3A%2F%2Frs1.example.com&scope=openid&scope=profile",
		nil)

	msg, err := suite.handler.getOAuthMessageForGetRequest(req)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), msg)
	assert.Contains(suite.T(), err.Error(), "scope")
}

func (suite *AuthorizeHandlerTestSuite) TestHandleAuthorizeGetRequest_DuplicateNonResourceParamReturns400() {
	// End-to-end: repeated non-resource query param must produce HTTP 400 invalid_request.
	req := httptest.NewRequest("GET", "/oauth2/authorize?client_id=a&client_id=b", nil)
	rr := httptest.NewRecorder()

	suite.handler.HandleAuthorizeGetRequest(rr, req)

	assert.Equal(suite.T(), http.StatusBadRequest, rr.Code)
}

func (suite *AuthorizeHandlerTestSuite) TestGetOAuthMessageForPostRequest_MissingAuthID() {
	postData := AuthZPostRequest{
		AuthID:    "",
		Assertion: "test-assertion",
	}
	jsonData, _ := json.Marshal(postData)

	req := httptest.NewRequest(http.MethodPost, "/auth", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")

	msg, err := suite.handler.getOAuthMessageForPostRequest(req)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), msg)
	assert.Contains(suite.T(), err.Error(), "authId or assertion is missing")
}

func (suite *AuthorizeHandlerTestSuite) TestGetOAuthMessageForPostRequest_MissingAssertion() {
	postData := AuthZPostRequest{
		AuthID:    testAuthID,
		Assertion: "",
	}
	jsonData, _ := json.Marshal(postData)

	req := httptest.NewRequest(http.MethodPost, "/auth", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")

	msg, err := suite.handler.getOAuthMessageForPostRequest(req)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), msg)
	assert.Contains(suite.T(), err.Error(), "authId or assertion is missing")
}

func (suite *AuthorizeHandlerTestSuite) TestGetOAuthMessage_UnsupportedMethod() {
	req := httptest.NewRequest(http.MethodPatch, "/auth", nil)
	rr := httptest.NewRecorder()

	msg := suite.handler.getOAuthMessage(req, rr)

	assert.Nil(suite.T(), msg)
	assert.Equal(suite.T(), http.StatusBadRequest, rr.Code)
}

func (suite *AuthorizeHandlerTestSuite) TestGetOAuthMessage_NilRequest() {
	rr := httptest.NewRecorder()

	msg := suite.handler.getOAuthMessage(nil, rr)

	assert.Nil(suite.T(), msg)
}

func (suite *AuthorizeHandlerTestSuite) TestGetOAuthMessage_NilResponseWriter() {
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)

	msg := suite.handler.getOAuthMessage(req, nil)

	assert.Nil(suite.T(), msg)
}

func (suite *AuthorizeHandlerTestSuite) TestHandleAuthorizeGetRequest_Success() {
	result := &AuthorizationInitResult{
		QueryParams: map[string]string{
			oauth2const.AuthID:      testAuthID,
			oauth2const.AppID:       "test-app-id",
			oauth2const.ExecutionID: "test-flow-id",
		},
	}
	suite.mockAuthzService.EXPECT().HandleInitialAuthorizationRequest(mock.Anything, mock.Anything).Return(result, nil)

	req := httptest.NewRequest("GET",
		"/oauth2/authorize?client_id=test-client&redirect_uri=https://example.com/callback&response_type=code", nil)
	rr := httptest.NewRecorder()

	suite.handler.HandleAuthorizeGetRequest(rr, req)

	assert.Equal(suite.T(), http.StatusFound, rr.Code)
	location := rr.Header().Get("Location")
	assert.Contains(suite.T(), location, "/login")
}

func (suite *AuthorizeHandlerTestSuite) TestHandleAuthorizeGetRequest_ServiceErrorRedirectToErrorPage() {
	authErr := &AuthorizationError{
		Code:              oauth2const.ErrorInvalidRequest,
		Message:           "Missing client_id parameter",
		SendErrorToClient: false,
	}
	suite.mockAuthzService.EXPECT().HandleInitialAuthorizationRequest(mock.Anything, mock.Anything).Return(nil, authErr)

	req := httptest.NewRequest("GET", "/oauth2/authorize?client_id=&redirect_uri=", nil)
	rr := httptest.NewRecorder()

	suite.handler.HandleAuthorizeGetRequest(rr, req)

	assert.Equal(suite.T(), http.StatusFound, rr.Code)
	location := rr.Header().Get("Location")
	assert.Contains(suite.T(), location, "/error")
}

func (suite *AuthorizeHandlerTestSuite) TestHandleAuthorizeGetRequest_ServiceErrorRedirectToClient() {
	authErr := &AuthorizationError{
		Code:              oauth2const.ErrorInvalidRequest,
		Message:           "Invalid response type",
		SendErrorToClient: true,
		ClientRedirectURI: "https://client.example.com/callback",
		State:             "test-state",
	}
	suite.mockAuthzService.EXPECT().HandleInitialAuthorizationRequest(mock.Anything, mock.Anything).Return(nil, authErr)

	reqURL := "/oauth2/authorize?client_id=test-client" +
		"&redirect_uri=https://client.example.com/callback&response_type=invalid"
	req := httptest.NewRequest("GET", reqURL, nil)
	rr := httptest.NewRecorder()

	suite.handler.HandleAuthorizeGetRequest(rr, req)

	assert.Equal(suite.T(), http.StatusFound, rr.Code)
	location := rr.Header().Get("Location")
	assert.Contains(suite.T(), location, "error=invalid_request")
	assert.Contains(suite.T(), location, "state=test-state")
	assert.Contains(suite.T(), location, "iss=https%3A%2F%2Flocalhost%3A8090")
}

func (suite *AuthorizeHandlerTestSuite) TestHandleAuthorizeGetRequest_IssAlwaysPresent() {
	// RFC 9207 §2: iss is unconditional. State is absent here to confirm iss appears regardless.
	authErr := &AuthorizationError{
		Code:              oauth2const.ErrorInvalidRequest,
		Message:           "Invalid response type",
		SendErrorToClient: true,
		ClientRedirectURI: "https://client.example.com/callback",
	}
	suite.mockAuthzService.EXPECT().HandleInitialAuthorizationRequest(mock.Anything, mock.Anything).Return(nil, authErr)

	reqURL := "/oauth2/authorize?client_id=test-client" +
		"&redirect_uri=https://client.example.com/callback&response_type=invalid"
	req := httptest.NewRequest("GET", reqURL, nil)
	rr := httptest.NewRecorder()

	suite.handler.HandleAuthorizeGetRequest(rr, req)

	assert.Equal(suite.T(), http.StatusFound, rr.Code)
	location := rr.Header().Get("Location")
	assert.Contains(suite.T(), location, "iss=https%3A%2F%2Flocalhost%3A8090")
	assert.NotContains(suite.T(), location, "state=")
}

func (suite *AuthorizeHandlerTestSuite) TestHandleAuthorizeGetRequest_GetOAuthMessageReturnsNil() {
	req := httptest.NewRequest("GET", "/oauth2/authorize?client_id=%ZZ", nil)
	rr := httptest.NewRecorder()

	suite.handler.HandleAuthorizeGetRequest(rr, req)

	assert.Equal(suite.T(), http.StatusBadRequest, rr.Code)
}

func (suite *AuthorizeHandlerTestSuite) TestHandleAuthCallbackPostRequest_Success() {
	redirectURI := "https://client.example.com/callback?code=test-code&state=test-state"
	suite.mockAuthzService.EXPECT().
		HandleAuthorizationCallback(mock.Anything, testAuthID, "test-assertion").
		Return(redirectURI, nil)

	postData := AuthZPostRequest{
		AuthID:    testAuthID,
		Assertion: "test-assertion",
	}
	jsonData, _ := json.Marshal(postData)

	req := httptest.NewRequest(http.MethodPost, "/oauth2/auth/callback", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	suite.handler.HandleAuthCallbackPostRequest(rr, req)

	assert.Equal(suite.T(), http.StatusOK, rr.Code)
	var resp AuthZPostResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), redirectURI, resp.RedirectURI)
}

func (suite *AuthorizeHandlerTestSuite) TestHandleAuthCallbackPostRequest_ServiceError() {
	authErr := &AuthorizationError{
		Code:    oauth2const.ErrorInvalidRequest,
		Message: "Invalid authorization request",
		State:   "test-state",
	}
	suite.mockAuthzService.EXPECT().
		HandleAuthorizationCallback(mock.Anything, testAuthID, "test-assertion").
		Return("", authErr)

	postData := AuthZPostRequest{
		AuthID:    testAuthID,
		Assertion: "test-assertion",
	}
	jsonData, _ := json.Marshal(postData)

	req := httptest.NewRequest(http.MethodPost, "/oauth2/auth/callback", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	suite.handler.HandleAuthCallbackPostRequest(rr, req)

	assert.Equal(suite.T(), http.StatusOK, rr.Code)
	var resp AuthZPostResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), resp.RedirectURI, "/error")
	assert.Contains(suite.T(), resp.RedirectURI, "state=test-state")
}

func (suite *AuthorizeHandlerTestSuite) TestHandleAuthCallbackPostRequest_ServiceErrorRedirectToClient() {
	authErr := &AuthorizationError{
		Code:              oauth2const.ErrorServerError,
		Message:           "Failed to process authorization request",
		State:             "test-state",
		SendErrorToClient: true,
		ClientRedirectURI: "https://client.example.com/callback",
	}
	suite.mockAuthzService.EXPECT().HandleAuthorizationCallback(mock.Anything, testAuthID, "test-assertion").
		Return("", authErr)

	postData := AuthZPostRequest{
		AuthID:    testAuthID,
		Assertion: "test-assertion",
	}
	jsonData, _ := json.Marshal(postData)

	req := httptest.NewRequest(http.MethodPost, "/oauth2/auth/callback", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	suite.handler.HandleAuthCallbackPostRequest(rr, req)

	assert.Equal(suite.T(), http.StatusOK, rr.Code)
	var resp AuthZPostResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), resp.RedirectURI, "https://client.example.com/callback")
	assert.Contains(suite.T(), resp.RedirectURI, "error=server_error")
	assert.Contains(suite.T(), resp.RedirectURI, "state=test-state")
	assert.Contains(suite.T(), resp.RedirectURI, "iss=https%3A%2F%2Flocalhost%3A8090")
}

func (suite *AuthorizeHandlerTestSuite) TestHandleAuthCallbackPostRequest_ClientErrorIssAlwaysPresent() {
	// RFC 9207 §2: iss is unconditional. Confirm iss is present even when state is absent.
	authErr := &AuthorizationError{
		Code:              oauth2const.ErrorServerError,
		Message:           "Failed to process authorization request",
		SendErrorToClient: true,
		ClientRedirectURI: "https://client.example.com/callback",
	}
	suite.mockAuthzService.EXPECT().HandleAuthorizationCallback(mock.Anything, testAuthID, "test-assertion").
		Return("", authErr)

	postData := AuthZPostRequest{
		AuthID:    testAuthID,
		Assertion: "test-assertion",
	}
	jsonData, _ := json.Marshal(postData)

	req := httptest.NewRequest(http.MethodPost, "/oauth2/auth/callback", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	suite.handler.HandleAuthCallbackPostRequest(rr, req)

	assert.Equal(suite.T(), http.StatusOK, rr.Code)
	var resp AuthZPostResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), resp.RedirectURI, "https://client.example.com/callback")
	assert.Contains(suite.T(), resp.RedirectURI, "error=server_error")
	assert.Contains(suite.T(), resp.RedirectURI, "iss=https%3A%2F%2Flocalhost%3A8090")
	assert.NotContains(suite.T(), resp.RedirectURI, "state=")
}

func (suite *AuthorizeHandlerTestSuite) TestHandleAuthCallbackPostRequest_InvalidRequestType() {
	// nil body causes JSON decode to fail → getOAuthMessage returns nil → 400
	req := httptest.NewRequest(http.MethodPost, "/oauth2/auth/callback", nil)
	rr := httptest.NewRecorder()

	suite.handler.HandleAuthCallbackPostRequest(rr, req)

	assert.Equal(suite.T(), http.StatusBadRequest, rr.Code)
}

func (suite *AuthorizeHandlerTestSuite) TestHandleAuthCallbackPostRequest_GetOAuthMessageReturnsNil() {
	req := httptest.NewRequest("POST", "/oauth2/auth/callback", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	suite.handler.HandleAuthCallbackPostRequest(rr, req)

	assert.Equal(suite.T(), http.StatusBadRequest, rr.Code)
}

func (suite *AuthorizeHandlerTestSuite) TestHandleAuthCallbackPostRequest_UnsupportedMethod() {
	req := httptest.NewRequest(http.MethodPut, "/oauth2/auth/callback", nil)
	rr := httptest.NewRecorder()

	suite.handler.HandleAuthCallbackPostRequest(rr, req)

	assert.Equal(suite.T(), http.StatusBadRequest, rr.Code)
	var response map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "invalid_request", response["error"])
}

func (suite *AuthorizeHandlerTestSuite) TestRedirectToLoginPage_NilResponseWriter() {
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	queryParams := map[string]string{"authId": "test-key"}

	suite.handler.redirectToLoginPage(nil, req, queryParams)
	// Should not panic
}

func (suite *AuthorizeHandlerTestSuite) TestRedirectToLoginPage_NilRequest() {
	rr := httptest.NewRecorder()
	queryParams := map[string]string{"authId": "test-key"}

	suite.handler.redirectToLoginPage(rr, nil, queryParams)
	// Should not panic
}

func (suite *AuthorizeHandlerTestSuite) TestRedirectToLoginPage_Success() {
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	rr := httptest.NewRecorder()
	validParams := map[string]string{
		"authId": "test-key",
		"appId":  "test-app",
	}

	suite.handler.redirectToLoginPage(rr, req, validParams)

	assert.Equal(suite.T(), http.StatusFound, rr.Code)
	assert.NotEmpty(suite.T(), rr.Header().Get("Location"))
}

func (suite *AuthorizeHandlerTestSuite) TestRedirectToErrorPage_NilResponseWriter() {
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)

	suite.handler.redirectToErrorPage(nil, req, "error_code", "error message")
	// Should not panic
}

func (suite *AuthorizeHandlerTestSuite) TestRedirectToErrorPage_NilRequest() {
	rr := httptest.NewRecorder()

	suite.handler.redirectToErrorPage(rr, nil, "error_code", "error message")
	// Should not panic; status remains unchanged
	assert.Equal(suite.T(), http.StatusOK, rr.Code)
}

func (suite *AuthorizeHandlerTestSuite) TestWriteAuthZResponseToErrorPage_WithState() {
	rr := httptest.NewRecorder()
	suite.handler.writeAuthZResponseToErrorPage(rr, "error_code", "error message", "test-state")

	assert.Equal(suite.T(), http.StatusOK, rr.Code)
	var resp AuthZPostResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), resp.RedirectURI, "state=test-state")
}

func (suite *AuthorizeHandlerTestSuite) TestWriteAuthZResponseToErrorPage_NoState() {
	rr := httptest.NewRecorder()
	suite.handler.writeAuthZResponseToErrorPage(rr, "error_code", "error message", "")

	assert.Equal(suite.T(), http.StatusOK, rr.Code)
	var resp AuthZPostResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), resp.RedirectURI)
	assert.Contains(suite.T(), resp.RedirectURI, "/error")
}

func (suite *AuthorizeHandlerTestSuite) TestWriteAuthZResponse() {
	rr := httptest.NewRecorder()

	suite.handler.writeAuthZResponse(rr, "https://example.com/callback?code=abc123")

	assert.Equal(suite.T(), http.StatusOK, rr.Code)
	assert.Equal(suite.T(), "application/json", rr.Header().Get("Content-Type"))
	var resp AuthZPostResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "https://example.com/callback?code=abc123", resp.RedirectURI)
}

func (suite *AuthorizeHandlerTestSuite) TestGetLoginPageRedirectURI_Success() {
	queryParams := map[string]string{
		"authId": "test-key",
		"appId":  "test-app",
	}

	redirectURI, err := getLoginPageRedirectURI(queryParams)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), redirectURI, "authId=test-key")
	assert.Contains(suite.T(), redirectURI, "appId=test-app")
}

func (suite *AuthorizeHandlerTestSuite) TestGetErrorPageRedirectURL_Success() {
	redirectURI, err := getErrorPageRedirectURL("invalid_request", "Missing parameter")
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), redirectURI, "errorCode=invalid_request")
	assert.Contains(suite.T(), redirectURI, "errorMessage=Missing+parameter")
}

func (suite *AuthorizeHandlerTestSuite) TestGetAuthorizationCode_Success() {
	authRequestCtx := &authRequestContext{
		OAuthParameters: oauth2model.OAuthParameters{
			ClientID:         "test-client",
			RedirectURI:      "https://client.example.com/callback",
			StandardScopes:   []string{"openid", "profile"},
			PermissionScopes: []string{"read", "write"},
		},
	}

	clms := &assertionClaims{userID: "test-user"}
	authTime := time.Now()

	result, err := createAuthorizationCode(authRequestCtx, clms, authTime)

	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), result.CodeID)
	assert.NotEmpty(suite.T(), result.Code)
	assert.Equal(suite.T(), "test-client", result.ClientID)
	assert.Equal(suite.T(), "https://client.example.com/callback", result.RedirectURI)
	assert.Equal(suite.T(), "test-user", result.AuthorizedUserID)
	assert.Equal(suite.T(), "openid profile read write", result.Scopes)
	assert.Equal(suite.T(), AuthCodeStateActive, result.State)
	assert.NotZero(suite.T(), result.TimeCreated)
}

func (suite *AuthorizeHandlerTestSuite) TestGetAuthorizationCode_MissingClientID() {
	authRequestCtx := &authRequestContext{
		OAuthParameters: oauth2model.OAuthParameters{
			ClientID:    "",
			RedirectURI: "https://client.example.com/callback",
		},
	}

	clms := &assertionClaims{userID: "test-user"}
	authTime := time.Now()

	result, err := createAuthorizationCode(authRequestCtx, clms, authTime)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "client_id or redirect_uri is missing")
	assert.Equal(suite.T(), AuthorizationCode{}, result)
}

func (suite *AuthorizeHandlerTestSuite) TestGetAuthorizationCode_MissingRedirectURI() {
	authRequestCtx := &authRequestContext{
		OAuthParameters: oauth2model.OAuthParameters{
			ClientID:    "test-client",
			RedirectURI: "",
		},
	}

	clms := &assertionClaims{userID: "test-user"}
	authTime := time.Now()

	result, err := createAuthorizationCode(authRequestCtx, clms, authTime)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "client_id or redirect_uri is missing")
	assert.Equal(suite.T(), AuthorizationCode{}, result)
}

func (suite *AuthorizeHandlerTestSuite) TestGetAuthorizationCode_EmptyUserID() {
	authRequestCtx := &authRequestContext{
		OAuthParameters: oauth2model.OAuthParameters{
			ClientID:    "test-client-id",
			RedirectURI: "https://client.example.com/callback",
		},
	}

	clms := &assertionClaims{userID: ""}
	authTime := time.Now()

	result, err := createAuthorizationCode(authRequestCtx, clms, authTime)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "authenticated user not found")
	assert.Equal(suite.T(), AuthorizationCode{}, result)
}

func (suite *AuthorizeHandlerTestSuite) TestGetAuthorizationCode_ZeroAuthTime() {
	authRequestCtx := &authRequestContext{
		OAuthParameters: oauth2model.OAuthParameters{
			ClientID:    "test-client-id",
			RedirectURI: "https://client.example.com/callback",
		},
	}

	clms := &assertionClaims{userID: "test-user"}
	zeroAuthTime := time.Time{}
	beforeCreation := time.Now()

	result, err := createAuthorizationCode(authRequestCtx, clms, zeroAuthTime)

	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), result.CodeID)
	assert.NotEmpty(suite.T(), result.Code)
	assert.NotZero(suite.T(), result.TimeCreated)
	afterCreation := time.Now()
	assert.True(suite.T(), result.TimeCreated.After(beforeCreation) || result.TimeCreated.Equal(beforeCreation))
	assert.True(suite.T(), result.TimeCreated.Before(afterCreation) || result.TimeCreated.Equal(afterCreation))
}

func (suite *AuthorizeHandlerTestSuite) TestCreateAuthorizationCode_WithClaimsLocales() {
	authRequestCtx := &authRequestContext{
		OAuthParameters: oauth2model.OAuthParameters{
			ClientID:         "test-client",
			RedirectURI:      "https://client.example.com/callback",
			StandardScopes:   []string{"openid", "profile"},
			PermissionScopes: []string{"read"},
			ClaimsLocales:    "en-US ja",
		},
	}

	clms := &assertionClaims{userID: "test-user"}
	authTime := time.Now()

	result, err := createAuthorizationCode(authRequestCtx, clms, authTime)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test-client", result.ClientID)
	assert.Equal(suite.T(), "en-US ja", result.ClaimsLocales)
}

func (suite *AuthorizeHandlerTestSuite) TestDecodeAttributesFromAssertion_Success() {
	// JWT with: sub, username, email, given_name, family_name, authorized_permissions, userType, ouId, ouName, ouHandle
	validJWT := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
		"eyJzdWIiOiJ0ZXN0LXVzZXIiLCJ1c2VybmFtZSI6InRlc3R1c2VyIiwiZW1haWwiOiJ0ZXN0QGV4YW1wbGUuY29tIiwiZ2l2ZW5fbmFtZSI6IlRlc3QiLCJmYW1pbHlfbmFtZSI6IlVzZXIiLCJhdXRob3JpemVkX3Blcm1pc3Npb25zIjoicmVhZCB3cml0ZSIsInVzZXJUeXBlIjoibG9jYWwiLCJvdUlkIjoib3UxMjMiLCJvdU5hbWUiOiJPcmdhbml6YXRpb24iLCJvdUhhbmRsZSI6Im9yZy1oYW5kbGUifQ." //nolint:lll

	clms, _, err := decodeAttributesFromAssertion(validJWT)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test-user", clms.userID)
	assert.Equal(suite.T(), "", clms.attributeCacheID)
	assert.Equal(suite.T(), "read write", clms.authorizedPermissions)
}

func (suite *AuthorizeHandlerTestSuite) TestDecodeAttributesFromAssertion_DecodeError() {
	invalidJWT := "invalid.jwt.token"

	_, _, err := decodeAttributesFromAssertion(invalidJWT)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to decode the JWT token")
}

func (suite *AuthorizeHandlerTestSuite) TestDecodeAttributesFromAssertion_InvalidSubClaim() {
	// JWT payload: {"sub":12345}
	invalidSubJWT := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOjEyMzQ1fQ."

	clms, _, err := decodeAttributesFromAssertion(invalidSubJWT)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "JWT 'sub' claim is not a string")
	assert.Equal(suite.T(), "", clms.userID)
}

func (suite *AuthorizeHandlerTestSuite) TestDecodeAttributesFromAssertion_NonStringAttributes() {
	// JWT payload: {"sub":"test-user","username":12345,"email":12345,"given_name":12345,
	// "family_name":12345,"authorized_permissions":12345}
	nonStringAttrsJWT := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
		"eyJzdWIiOiJ0ZXN0LXVzZXIiLCJ1c2VybmFtZSI6MTIzNDUsImVtYWlsIjoxMjM0NSwi" +
		"Z2l2ZW5fbmFtZSI6MTIzNDUsImZhbWlseV9uYW1lIjoxMjM0NSwiYXV0aG9yaXplZF9w" +
		"ZXJtaXNzaW9ucyI6MTIzNDV9."

	clms, _, err := decodeAttributesFromAssertion(nonStringAttrsJWT)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test-user", clms.userID)
	// Non-string authorized_permissions is ignored
	assert.Equal(suite.T(), "", clms.authorizedPermissions)
}

func (suite *AuthorizeHandlerTestSuite) TestDecodeAttributesFromAssertion_UserTypeInUserAttributes() {
	// JWT payload: {"sub":"test-user","userType":12345}
	userTypeJWT := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiJ0ZXN0LXVzZXIiLCJ1c2VyVHlwZSI6MTIzNDV9."

	clms, _, err := decodeAttributesFromAssertion(userTypeJWT)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test-user", clms.userID)
}

func (suite *AuthorizeHandlerTestSuite) TestDecodeAttributesFromAssertion_OUClaimsInUserAttributes() {
	// JWT payload: {"sub":"test-user","ouId":12345,"ouName":12345,"ouHandle":12345}
	jwtToken := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
		"eyJzdWIiOiJ0ZXN0LXVzZXIiLCJvdUlkIjoxMjM0NSwib3VOYW1lIjoxMjM0NSwib3VIYW5kbGUiOjEyMzQ1fQ."

	clms, _, err := decodeAttributesFromAssertion(jwtToken)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test-user", clms.userID)
}

func (suite *AuthorizeHandlerTestSuite) TestDecodeAttributesFromAssertion_WithAttributeCacheID() {
	// JWT payload: {"sub":"test-user","aci":"cache-abc-123","authorized_permissions":"read write"}
	jwtToken := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
		"eyJzdWIiOiJ0ZXN0LXVzZXIiLCJhY2kiOiJjYWNoZS1hYmMtMTIzIiwiYXV0aG9yaXplZF9wZXJtaXNzaW9ucyI6InJlYWQgd3JpdGUifQ."

	clms, _, err := decodeAttributesFromAssertion(jwtToken)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test-user", clms.userID)
	assert.Equal(suite.T(), "cache-abc-123", clms.attributeCacheID)
	assert.Equal(suite.T(), "read write", clms.authorizedPermissions)
}

func (suite *AuthorizeHandlerTestSuite) TestDecodeAttributesFromAssertion_NonStringAttributeCacheID() {
	// JWT payload: {"sub":"test-user","aci":12345}
	jwtToken := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
		"eyJzdWIiOiJ0ZXN0LXVzZXIiLCJhY2kiOjEyMzQ1fQ."

	_, _, err := decodeAttributesFromAssertion(jwtToken)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "JWT 'aci' claim is not a string")
}

func (suite *AuthorizeHandlerTestSuite) TestDecodeAttributesFromAssertion_WithCompletedAuthClass() {
	// JWT payload: {"sub":"test-user","completed_auth_class":"urn:thunder:acr:password"}
	jwtToken := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
		"eyJzdWIiOiJ0ZXN0LXVzZXIiLCJjb21wbGV0ZWRfYXV0aF9jbGFzcyI6InVybjp0aHVuZGVyOmFjcjpwYXNzd29yZCJ9."

	clms, _, err := decodeAttributesFromAssertion(jwtToken)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test-user", clms.userID)
	assert.Equal(suite.T(), "urn:thunder:acr:password", clms.completedACR)
}

func (suite *AuthorizeHandlerTestSuite) TestDecodeAttributesFromAssertion_NonStringCompletedAuthClass() {
	// JWT payload: {"sub":"test-user","completed_auth_class":12345}
	jwtToken := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
		"eyJzdWIiOiJ0ZXN0LXVzZXIiLCJjb21wbGV0ZWRfYXV0aF9jbGFzcyI6MTIzNDV9."

	_, _, err := decodeAttributesFromAssertion(jwtToken)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "JWT 'completed_auth_class' claim is not a string")
}

func (suite *AuthorizeHandlerTestSuite) TestValidateSubClaimConstraint() {
	tests := []struct {
		name          string
		claimsRequest *oauth2model.ClaimsRequest
		actualSubject string
		expectError   bool
	}{
		{
			name:          "nil claims request should pass",
			claimsRequest: nil,
			actualSubject: "user123",
			expectError:   false,
		},
		{
			name: "no sub constraint should pass",
			claimsRequest: &oauth2model.ClaimsRequest{
				IDToken: map[string]*oauth2model.IndividualClaimRequest{
					"email": nil,
				},
			},
			actualSubject: "user123",
			expectError:   false,
		},
		{
			name: "matching id_token sub value should pass",
			claimsRequest: &oauth2model.ClaimsRequest{
				IDToken: map[string]*oauth2model.IndividualClaimRequest{
					"sub": {Value: "user123"},
				},
			},
			actualSubject: "user123",
			expectError:   false,
		},
		{
			name: "non-matching id_token sub value should fail",
			claimsRequest: &oauth2model.ClaimsRequest{
				IDToken: map[string]*oauth2model.IndividualClaimRequest{
					"sub": {Value: "expected-user"},
				},
			},
			actualSubject: "actual-user",
			expectError:   true,
		},
		{
			name: "matching userinfo sub value should pass",
			claimsRequest: &oauth2model.ClaimsRequest{
				UserInfo: map[string]*oauth2model.IndividualClaimRequest{
					"sub": {Value: "user456"},
				},
			},
			actualSubject: "user456",
			expectError:   false,
		},
		{
			name: "non-matching userinfo sub value should fail",
			claimsRequest: &oauth2model.ClaimsRequest{
				UserInfo: map[string]*oauth2model.IndividualClaimRequest{
					"sub": {Value: "expected-user"},
				},
			},
			actualSubject: "actual-user",
			expectError:   true,
		},
		{
			name: "matching sub in values array should pass",
			claimsRequest: &oauth2model.ClaimsRequest{
				IDToken: map[string]*oauth2model.IndividualClaimRequest{
					"sub": {Values: []interface{}{"user1", "user2", "user3"}},
				},
			},
			actualSubject: "user2",
			expectError:   false,
		},
		{
			name: "non-matching sub in values array should fail",
			claimsRequest: &oauth2model.ClaimsRequest{
				IDToken: map[string]*oauth2model.IndividualClaimRequest{
					"sub": {Values: []interface{}{"user1", "user2", "user3"}},
				},
			},
			actualSubject: "user4",
			expectError:   true,
		},
		{
			name: "null sub request (voluntary) should pass",
			claimsRequest: &oauth2model.ClaimsRequest{
				IDToken: map[string]*oauth2model.IndividualClaimRequest{
					"sub": nil,
				},
			},
			actualSubject: "any-user",
			expectError:   false,
		},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			err := validateSubClaimConstraint(tt.claimsRequest, tt.actualSubject)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
