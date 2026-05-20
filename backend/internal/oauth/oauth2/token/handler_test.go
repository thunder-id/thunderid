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

package token

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
)

type TokenHandlerTestSuite struct {
	suite.Suite
	mockTokenService *TokenServiceInterfaceMock
}

func TestTokenHandlerSuite(t *testing.T) {
	suite.Run(t, new(TokenHandlerTestSuite))
}

func (suite *TokenHandlerTestSuite) SetupTest() {
	suite.mockTokenService = NewTokenServiceInterfaceMock(suite.T())
}

// newHandler creates a tokenHandler backed by the suite's service mock.
func (suite *TokenHandlerTestSuite) newHandler() *tokenHandler {
	return newTokenHandler(suite.mockTokenService, nil).(*tokenHandler)
}

// buildRequest constructs a POST /token request with URL-encoded form data.
func (suite *TokenHandlerTestSuite) buildRequest(formData url.Values) *http.Request {
	req, _ := http.NewRequest("POST", "/token", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

// withClientContext injects a fake OAuth client info into the request context.
func (suite *TokenHandlerTestSuite) withClientContext(
	req *http.Request, oauthApp *inboundmodel.OAuthClient,
) *http.Request {
	clientInfo := &clientauth.OAuthClientInfo{
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		OAuthApp:     oauthApp,
	}
	ctx := context.WithValue(req.Context(), clientauth.OAuthClientKey, clientInfo)
	return req.WithContext(ctx)
}

func (suite *TokenHandlerTestSuite) TestnewTokenHandler() {
	handler := newTokenHandler(suite.mockTokenService, nil)
	assert.NotNil(suite.T(), handler)
	assert.Implements(suite.T(), (*TokenHandlerInterface)(nil), handler)
}

func (suite *TokenHandlerTestSuite) TestHandleTokenRequest_InvalidFormData() {
	handler := suite.newHandler()
	// Malformed percent-encoding causes ParseForm to fail.
	req, _ := http.NewRequest("POST", "/token", strings.NewReader("invalid-form-data%"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.HandleTokenRequest(rr, req)

	assert.Equal(suite.T(), http.StatusBadRequest, rr.Code)
	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "invalid_request", response["error"])
	assert.Equal(suite.T(), "Failed to parse request body", response["error_description"])
}

func (suite *TokenHandlerTestSuite) TestHandleTokenRequest_MissingClientID() {
	// No OAuth client info in context → middleware was not applied → 500.
	handler := suite.newHandler()
	formData := url.Values{}
	formData.Set("grant_type", "authorization_code")
	req := suite.buildRequest(formData)
	// Deliberately skip withClientContext.
	rr := httptest.NewRecorder()

	handler.HandleTokenRequest(rr, req)

	assert.Equal(suite.T(), http.StatusInternalServerError, rr.Code)
	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "server_error", response["error"])
}

func (suite *TokenHandlerTestSuite) TestHandleTokenRequest_ServiceErrors() {
	tests := []struct {
		name          string
		grantType     string
		errCode       string
		errDesc       string
		expectedCode  int
		expectedError string
	}{
		{
			name:          "BadRequest",
			grantType:     "authorization_code",
			errCode:       constants.ErrorInvalidRequest,
			errDesc:       "Missing grant_type parameter",
			expectedCode:  http.StatusBadRequest,
			expectedError: "invalid_request",
		},
		{
			name:          "UnauthorizedClient",
			grantType:     "client_credentials",
			errCode:       constants.ErrorUnauthorizedClient,
			errDesc:       "The client is not authorized to use this grant type",
			expectedCode:  http.StatusBadRequest,
			expectedError: "unauthorized_client",
		},
	}
	for _, tc := range tests {
		suite.Run(tc.name, func() {
			mockSvc := NewTokenServiceInterfaceMock(suite.T())
			handler := newTokenHandler(mockSvc, nil).(*tokenHandler)
			mockApp := &inboundmodel.OAuthClient{ClientID: "test-client-id"}
			formData := url.Values{}
			formData.Set("grant_type", tc.grantType)
			req := suite.withClientContext(suite.buildRequest(formData), mockApp)

			mockSvc.EXPECT().
				ProcessTokenRequest(mock.Anything, mock.Anything, mock.Anything).
				Return(nil, &model.ErrorResponse{
					Error:            tc.errCode,
					ErrorDescription: tc.errDesc,
				})

			rr := httptest.NewRecorder()
			handler.HandleTokenRequest(rr, req)

			assert.Equal(suite.T(), tc.expectedCode, rr.Code)
			var response map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &response)
			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), tc.expectedError, response["error"])
		})
	}
}

func (suite *TokenHandlerTestSuite) TestHandleTokenRequest_ServiceErrorServerError() {
	handler := suite.newHandler()
	mockApp := &inboundmodel.OAuthClient{ClientID: "test-client-id"}
	formData := url.Values{}
	formData.Set("grant_type", "authorization_code")
	formData.Set("code", "test-code")
	req := suite.withClientContext(suite.buildRequest(formData), mockApp)

	suite.mockTokenService.EXPECT().
		ProcessTokenRequest(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Internal server error",
		})

	rr := httptest.NewRecorder()
	handler.HandleTokenRequest(rr, req)

	assert.Equal(suite.T(), http.StatusInternalServerError, rr.Code)
	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "server_error", response["error"])
}

func (suite *TokenHandlerTestSuite) TestHandleTokenRequest_Success() {
	handler := suite.newHandler()
	mockApp := &inboundmodel.OAuthClient{ClientID: "test-client-id"}
	formData := url.Values{}
	formData.Set("grant_type", "authorization_code")
	formData.Set("code", "test-code")
	formData.Set("scope", "openid profile")
	req := suite.withClientContext(suite.buildRequest(formData), mockApp)

	tokenResponse := &model.TokenResponse{
		AccessToken: "access-token-123",
		TokenType:   "Bearer",
		ExpiresIn:   3600,
		Scope:       "openid profile",
	}
	suite.mockTokenService.EXPECT().
		ProcessTokenRequest(mock.Anything, mock.Anything, mock.Anything).
		Return(tokenResponse, nil)

	rr := httptest.NewRecorder()
	handler.HandleTokenRequest(rr, req)

	assert.Equal(suite.T(), http.StatusOK, rr.Code)
	assert.Equal(suite.T(), "application/json", rr.Header().Get("Content-Type"))
	assert.Equal(suite.T(), "no-store", rr.Header().Get("Cache-Control"))
	assert.Equal(suite.T(), "no-cache", rr.Header().Get("Pragma"))

	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "access-token-123", response["access_token"])
	assert.Equal(suite.T(), "Bearer", response["token_type"])
	assert.Equal(suite.T(), float64(3600), response["expires_in"])
	assert.Equal(suite.T(), "openid profile", response["scope"])
}

func (suite *TokenHandlerTestSuite) TestHandleTokenRequest_SuccessWithIssuedTokenType() {
	handler := suite.newHandler()
	mockApp := &inboundmodel.OAuthClient{ClientID: "test-client-id"}
	formData := url.Values{}
	formData.Set("grant_type", string(constants.GrantTypeTokenExchange))
	formData.Set("requested_token_type", string(constants.TokenTypeIdentifierAccessToken))
	req := suite.withClientContext(suite.buildRequest(formData), mockApp)

	tokenResponse := &model.TokenResponse{
		AccessToken:     "exchanged-token",
		TokenType:       "Bearer",
		ExpiresIn:       3600,
		IssuedTokenType: string(constants.TokenTypeIdentifierAccessToken),
	}
	suite.mockTokenService.EXPECT().
		ProcessTokenRequest(mock.Anything, mock.Anything, mock.Anything).
		Return(tokenResponse, nil)

	rr := httptest.NewRecorder()
	handler.HandleTokenRequest(rr, req)

	assert.Equal(suite.T(), http.StatusOK, rr.Code)
	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "exchanged-token", response["access_token"])
	assert.Equal(suite.T(), string(constants.TokenTypeIdentifierAccessToken), response["issued_token_type"])
}
