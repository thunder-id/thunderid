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

package clientauth

import (
	"github.com/thunder-id/thunderid/internal/system/i18n/core"

	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

type ClientAuthMiddlewareTestSuite struct {
	suite.Suite
	mockInboundClient *inboundclientmock.InboundClientServiceInterfaceMock
	mockAuthnProvider *managermock.AuthnProviderManagerInterfaceMock
	mockJwtService    *jwtmock.JWTServiceInterfaceMock
}

func TestClientAuthMiddlewareTestSuite(t *testing.T) {
	suite.Run(t, new(ClientAuthMiddlewareTestSuite))
}

func (suite *ClientAuthMiddlewareTestSuite) SetupTest() {
	suite.mockInboundClient = inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerInterfaceMock(suite.T())
	suite.mockJwtService = jwtmock.NewJWTServiceInterfaceMock(suite.T())

	// Default authn mock: return success for client secret authentication.
	// Individual tests can override with Once() for specific behavior.
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidermgr.AuthnBasicResult{UserID: testClientID},
			(*serviceerror.ServiceError)(nil)).Maybe()
}

func (suite *ClientAuthMiddlewareTestSuite) TestClientAuthMiddleware_Success_ClientSecretPost() {
	// Setup mock OAuth app
	clientSecret := testClientSecret
	mockApp := &inboundmodel.OAuthClient{
		ClientID:                testClientID,
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
	}

	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(mockApp, nil).Once()

	// Create middleware (authn success mock from SetupTest applies via Maybe())
	middleware := ClientAuthMiddleware(
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, "https://localhost:9443/oauth2/token")

	// Create test handler that checks context
	var clientInfo *OAuthClientInfo
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientInfo = GetOAuthClient(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	// Create request with client_secret_post
	formData := url.Values{}
	formData.Set("client_id", testClientID)
	formData.Set("client_secret", clientSecret)

	req := httptest.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	// Execute middleware
	middleware(handler).ServeHTTP(w, req)

	// Verify
	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.NotNil(suite.T(), clientInfo, "Expected client info in context")
	if clientInfo != nil {
		assert.Equal(suite.T(), testClientID, clientInfo.ClientID)
		assert.Equal(suite.T(), "test-secret", clientInfo.ClientSecret)
		assert.NotNil(suite.T(), clientInfo.OAuthApp)
	}
}

func (suite *ClientAuthMiddlewareTestSuite) TestClientAuthMiddleware_Success_ClientSecretBasic() {
	// Setup mock OAuth app
	clientSecret := testClientSecret
	mockApp := &inboundmodel.OAuthClient{
		ClientID:                testClientID,
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretBasic,
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
	}

	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(mockApp, nil).Once()

	// Create middleware
	middleware := ClientAuthMiddleware(
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, "https://localhost:9443/oauth2/token")

	// Create test handler
	var clientInfo *OAuthClientInfo
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientInfo = GetOAuthClient(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	// Create request with Basic Auth
	req := httptest.NewRequest("POST", "/test", nil)
	req.SetBasicAuth(testClientID, clientSecret)
	w := httptest.NewRecorder()

	// Execute middleware
	middleware(handler).ServeHTTP(w, req)

	// Verify
	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.NotNil(suite.T(), clientInfo, "Expected client info in context")
	if clientInfo != nil {
		assert.Equal(suite.T(), testClientID, clientInfo.ClientID)
	}
}

func (suite *ClientAuthMiddlewareTestSuite) TestClientAuthMiddleware_MissingClientID() {
	// Create middleware
	middleware := ClientAuthMiddleware(
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, "https://localhost:9443/oauth2/token")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create request without client_id
	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()

	// Execute middleware
	middleware(handler).ServeHTTP(w, req)

	// Verify error response
	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "invalid_request", response["error"])
}

func (suite *ClientAuthMiddlewareTestSuite) TestClientAuthMiddleware_InvalidClient() {
	// Mock app service to return nil (client not found)
	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "invalid-client").
		Return(nil, nil).Once()

	// Create middleware
	middleware := ClientAuthMiddleware(
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, "https://localhost:9443/oauth2/token")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create request with invalid client
	formData := url.Values{}
	formData.Set("client_id", "invalid-client")
	formData.Set("client_secret", "test-secret")

	req := httptest.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	// Execute middleware
	middleware(handler).ServeHTTP(w, req)

	// Verify error response
	assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "invalid_client", response["error"])
}

func (suite *ClientAuthMiddlewareTestSuite) TestClientAuthMiddleware_InvalidClientSecret() {
	// Setup mock OAuth app
	mockApp := &inboundmodel.OAuthClient{
		ClientID:                testClientID,
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
	}

	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(mockApp, nil).Once()

	// Override authn mock to fail for wrong secret.
	failAuthnProvider := managermock.NewAuthnProviderManagerInterfaceMock(suite.T())
	failAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, (*authnprovidermgr.AuthnBasicResult)(nil),
			&serviceerror.ServiceError{
				Type:             serviceerror.ClientErrorType,
				Code:             authnprovidermgr.ErrorAuthenticationFailed.Code,
				Error:            core.I18nMessage{Key: "error.test.auth_failed", DefaultValue: "auth failed"},
				ErrorDescription: core.I18nMessage{Key: "error.test.wrong_secret", DefaultValue: "wrong secret"},
			}).Maybe()

	// Create middleware with failing authn provider
	middleware := ClientAuthMiddleware(
		suite.mockInboundClient, failAuthnProvider, suite.mockJwtService, "https://localhost:9443/oauth2/token")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create request with wrong client_secret
	formData := url.Values{}
	formData.Set("client_id", testClientID)
	formData.Set("client_secret", "wrong-secret")

	req := httptest.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	// Execute middleware
	middleware(handler).ServeHTTP(w, req)

	// Verify error response
	assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "invalid_client", response["error"])
}

func (suite *ClientAuthMiddlewareTestSuite) TestClientAuthMiddleware_HandlerNotCalledOnAuthFailure() {
	// Mock app service to return nil (client not found)
	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, mock.Anything).
		Return(nil, nil).Once()

	// Create middleware
	middleware := ClientAuthMiddleware(
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, "https://localhost:9443/oauth2/token")

	// Track if handler was called
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Create request with invalid client
	formData := url.Values{}
	formData.Set("client_id", "invalid-client")
	formData.Set("client_secret", "test-secret")

	req := httptest.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	// Execute middleware
	middleware(handler).ServeHTTP(w, req)

	// Verify handler was not called
	assert.False(suite.T(), handlerCalled, "Handler should not be called when authentication fails")
	assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
}

func (suite *ClientAuthMiddlewareTestSuite) TestClientAuthMiddleware_ContextPropagation() {
	// Setup mock OAuth app
	clientSecret := testClientSecret
	mockApp := &inboundmodel.OAuthClient{
		ClientID:                testClientID,
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
	}

	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(mockApp, nil).Once()

	// Create middleware
	middleware := ClientAuthMiddleware(
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, "https://localhost:9443/oauth2/token")

	// Create nested handler that also checks context
	var clientInfo *OAuthClientInfo
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientInfo = GetOAuthClient(r.Context())
		// Verify context is available
		if clientInfo == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Create request
	formData := url.Values{}
	formData.Set("client_id", testClientID)
	formData.Set("client_secret", clientSecret)

	req := httptest.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	// Execute middleware
	middleware(handler).ServeHTTP(w, req)

	// Verify context was propagated
	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.NotNil(suite.T(), clientInfo)
}

// Tests for RFC 6749 §5.2: WWW-Authenticate header on 401 responses when client used Authorization header.

func (suite *ClientAuthMiddlewareTestSuite) TestClientAuthMiddleware_BasicAuth_401_IncludesWWWAuthenticate() {
	// Client not found with Basic auth should include WWW-Authenticate: Basic
	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(nil, nil).Once()

	middleware := ClientAuthMiddleware(
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, "https://localhost:9443/oauth2/token")
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/test", nil)
	req.SetBasicAuth(testClientID, testClientSecret)
	w := httptest.NewRecorder()

	middleware(handler).ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
	assert.Equal(suite.T(), "Basic", w.Header().Get("WWW-Authenticate"))
}

func (suite *ClientAuthMiddlewareTestSuite) TestClientAuthMiddleware_BasicAuth_InvalidCreds_IncludesWWWAuth() {
	mockApp := &inboundmodel.OAuthClient{
		ClientID:                testClientID,
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretBasic,
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
	}

	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(mockApp, nil).Once()

	// Override authn mock to fail for wrong secret.
	failAuthnProvider := managermock.NewAuthnProviderManagerInterfaceMock(suite.T())
	failAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, (*authnprovidermgr.AuthnBasicResult)(nil),
			&serviceerror.ServiceError{
				Type:             serviceerror.ClientErrorType,
				Code:             authnprovidermgr.ErrorAuthenticationFailed.Code,
				Error:            core.I18nMessage{Key: "error.test.auth_failed", DefaultValue: "auth failed"},
				ErrorDescription: core.I18nMessage{Key: "error.test.wrong_secret", DefaultValue: "wrong secret"},
			}).Maybe()

	middleware := ClientAuthMiddleware(
		suite.mockInboundClient, failAuthnProvider, suite.mockJwtService, "https://localhost:9443/oauth2/token")
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/test", nil)
	req.SetBasicAuth(testClientID, "wrong-secret")
	w := httptest.NewRecorder()

	middleware(handler).ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
	assert.Equal(suite.T(), "Basic", w.Header().Get("WWW-Authenticate"))
}

func (suite *ClientAuthMiddlewareTestSuite) TestClientAuthMiddleware_PostAuth_401_NoWWWAuthenticate() {
	// Client not found with POST body auth should not include WWW-Authenticate
	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "non-existent").
		Return(nil, nil).Once()

	middleware := ClientAuthMiddleware(
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, "https://localhost:9443/oauth2/token")
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	formData := url.Values{}
	formData.Set("client_id", "non-existent")
	formData.Set("client_secret", testClientSecret)

	req := httptest.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	middleware(handler).ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
	assert.Empty(suite.T(), w.Header().Get("WWW-Authenticate"))
}

func (suite *ClientAuthMiddlewareTestSuite) TestClientAuthMiddleware_InvalidBasicAuth_IncludesWWWAuthenticate() {
	// Invalid Basic auth header format should include WWW-Authenticate: Basic
	middleware := ClientAuthMiddleware(
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, "https://localhost:9443/oauth2/token")
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	w := httptest.NewRecorder()

	middleware(handler).ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
	assert.Equal(suite.T(), "Basic", w.Header().Get("WWW-Authenticate"))
}
