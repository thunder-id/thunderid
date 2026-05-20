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

package oauth

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/httpmock"
)

type OAuthUtilsTestSuite struct {
	suite.Suite
}

func TestOAuthUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(OAuthUtilsTestSuite))
}

func (suite *OAuthUtilsTestSuite) TestParseIDPConfig() {
	clientIDProp, _ := cmodels.NewProperty("client_id", "test_client", false)
	clientSecretProp, _ := cmodels.NewProperty("client_secret", "test_secret", false)
	redirectURIProp, _ := cmodels.NewProperty("redirect_uri", "http://localhost:3000/callback", false)
	scopesProp, _ := cmodels.NewProperty("scopes", "openid,profile,email", false)
	authEndpointProp, _ := cmodels.NewProperty("authorization_endpoint", "https://localhost:8090/auth", false)
	tokenEndpointProp, _ := cmodels.NewProperty("token_endpoint", "https://localhost:8090/token", false)
	userInfoProp, _ := cmodels.NewProperty("userinfo_endpoint", "https://localhost:8090/userinfo", false)

	idpDTO := &idp.IDPDTO{
		Properties: []cmodels.Property{
			*clientIDProp,
			*clientSecretProp,
			*redirectURIProp,
			*scopesProp,
			*authEndpointProp,
			*tokenEndpointProp,
			*userInfoProp,
		},
	}

	config, err := parseIDPConfig(idpDTO)
	suite.Nil(err)
	suite.NotNil(config)

	suite.Equal("test_client", config.ClientID)
	suite.Equal("http://localhost:3000/callback", config.RedirectURI)
	suite.Contains(config.Scopes, "openid")
	suite.Contains(config.Scopes, "profile")
	suite.Contains(config.Scopes, "email")
	suite.Equal("https://localhost:8090/auth", config.OAuthEndpoints.AuthorizationEndpoint)
	suite.Equal("https://localhost:8090/token", config.OAuthEndpoints.TokenEndpoint)
	suite.Equal("https://localhost:8090/userinfo", config.OAuthEndpoints.UserInfoEndpoint)
}

func (suite *OAuthUtilsTestSuite) TestParseIDPConfigWithSpaceSeparatedScopes() {
	clientIDProp, _ := cmodels.NewProperty("client_id", "test_client", false)
	scopesProp, _ := cmodels.NewProperty("scopes", "openid profile email", false)

	idpDTO := &idp.IDPDTO{
		Properties: []cmodels.Property{
			*clientIDProp,
			*scopesProp,
		},
	}

	config, err := parseIDPConfig(idpDTO)
	suite.Nil(err)
	suite.NotNil(config)

	suite.Len(config.Scopes, 3)
	suite.Contains(config.Scopes, "openid")
	suite.Contains(config.Scopes, "profile")
	suite.Contains(config.Scopes, "email")
}

func (suite *OAuthUtilsTestSuite) TestParseIDPConfigWithAdditionalParams() {
	clientIDProp, _ := cmodels.NewProperty("client_id", "test_client", false)
	customProp, _ := cmodels.NewProperty("custom_param", "custom_value", false)

	idpDTO := &idp.IDPDTO{
		Properties: []cmodels.Property{
			*clientIDProp,
			*customProp,
		},
	}

	config, err := parseIDPConfig(idpDTO)
	suite.Nil(err)
	suite.NotNil(config)
	suite.Equal("custom_value", config.AdditionalParams["custom_param"])
}

func (suite *OAuthUtilsTestSuite) TestParseIDPConfigWithEmptyValues() {
	clientIDProp, _ := cmodels.NewProperty("client_id", "test_client", false)
	emptyProp, _ := cmodels.NewProperty("custom_param", "", false)

	idpDTO := &idp.IDPDTO{
		Properties: []cmodels.Property{
			*clientIDProp,
			*emptyProp,
		},
	}

	config, err := parseIDPConfig(idpDTO)
	suite.Nil(err)
	suite.NotNil(config)
	suite.NotContains(config.AdditionalParams, "custom_param")
}

func (suite *OAuthUtilsTestSuite) TestBuildTokenRequestSuccess() {
	config := &OAuthClientConfig{
		ClientID:     "test_client",
		ClientSecret: "test_secret",
		RedirectURI:  "http://localhost:3000/callback",
		OAuthEndpoints: OAuthEndpoints{
			TokenEndpoint: "https://localhost:8090/token",
		},
	}
	code := "auth_code_123"
	logger := log.GetLogger()

	req, err := buildTokenRequest(config, code, logger)

	suite.Nil(err)
	suite.NotNil(req)
	suite.Equal("POST", req.Method)
	suite.Equal("https://localhost:8090/token", req.URL.String())
	suite.Equal("application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
	suite.Equal("application/json", req.Header.Get("Accept"))

	// Verify the form body
	suite.NoError(req.ParseForm())
	suite.Equal("test_client", req.FormValue("client_id"))
	suite.Equal("test_secret", req.FormValue("client_secret"))
	suite.Equal("http://localhost:3000/callback", req.FormValue("redirect_uri"))
	suite.Equal("authorization_code", req.FormValue("grant_type"))
	suite.Equal("auth_code_123", req.FormValue("code"))
}

func (suite *OAuthUtilsTestSuite) TestBuildTokenRequestWithInvalidURL() {
	config := &OAuthClientConfig{
		ClientID:     "test_client",
		ClientSecret: "test_secret",
		OAuthEndpoints: OAuthEndpoints{
			TokenEndpoint: "://invalid-url",
		},
	}
	logger := log.GetLogger()

	req, err := buildTokenRequest(config, "code123", logger)

	suite.Nil(req)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *OAuthUtilsTestSuite) TestSendTokenRequestSuccess() {
	mockHTTPClient := httpmock.NewHTTPClientInterfaceMock(suite.T())
	logger := log.GetLogger()

	tokenResp := TokenResponse{
		AccessToken:  "access_token_123",
		IDToken:      "id_token_123",
		TokenType:    "Bearer",
		RefreshToken: "refresh_token_123",
		ExpiresIn:    3600,
		Scope:        "openid profile email",
	}
	respBody, _ := json.Marshal(tokenResp)
	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(respBody)),
	}

	req, _ := http.NewRequest("POST", "https://localhost:8090/token", nil)
	mockHTTPClient.On("Do", req).Return(httpResp, nil)

	resp, err := sendTokenRequest(req, mockHTTPClient, logger)

	suite.Nil(err)
	suite.NotNil(resp)
	suite.Equal("access_token_123", resp.AccessToken)
	suite.Equal("id_token_123", resp.IDToken)
	suite.Equal("Bearer", resp.TokenType)
	suite.Equal("refresh_token_123", resp.RefreshToken)
	suite.Equal(3600, resp.ExpiresIn)
	suite.Equal("openid profile email", resp.Scope)
}

func (suite *OAuthUtilsTestSuite) TestSendTokenRequestHTTPError() {
	mockHTTPClient := httpmock.NewHTTPClientInterfaceMock(suite.T())
	logger := log.GetLogger()

	req, _ := http.NewRequest("POST", "https://localhost:8090/token", nil)
	mockHTTPClient.On("Do", req).Return(nil, http.ErrHandlerTimeout)

	resp, err := sendTokenRequest(req, mockHTTPClient, logger)

	suite.Nil(resp)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *OAuthUtilsTestSuite) TestSendTokenRequestNonOKStatus() {
	mockHTTPClient := httpmock.NewHTTPClientInterfaceMock(suite.T())
	logger := log.GetLogger()

	errorBody := `{"error":"invalid_grant","error_description":"Invalid authorization code"}`
	httpResp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewReader([]byte(errorBody))),
	}

	req, _ := http.NewRequest("POST", "https://localhost:8090/token", nil)
	mockHTTPClient.On("Do", req).Return(httpResp, nil)

	resp, err := sendTokenRequest(req, mockHTTPClient, logger)

	suite.Nil(resp)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *OAuthUtilsTestSuite) TestSendTokenRequestInvalidJSON() {
	mockHTTPClient := httpmock.NewHTTPClientInterfaceMock(suite.T())
	logger := log.GetLogger()

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte("invalid json"))),
	}

	req, _ := http.NewRequest("POST", "https://localhost:8090/token", nil)
	mockHTTPClient.On("Do", req).Return(httpResp, nil)

	resp, err := sendTokenRequest(req, mockHTTPClient, logger)

	suite.Nil(resp)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *OAuthUtilsTestSuite) TestBuildUserInfoRequestSuccess() {
	userInfoEndpoint := "https://localhost:8090/userinfo"
	accessToken := "access_token_123"
	logger := log.GetLogger()

	req, err := buildUserInfoRequest(userInfoEndpoint, accessToken, logger)

	suite.Nil(err)
	suite.NotNil(req)
	suite.Equal("GET", req.Method)
	suite.Equal("https://localhost:8090/userinfo", req.URL.String())
	suite.Equal("Bearer access_token_123", req.Header.Get("Authorization"))
	suite.Equal("application/json", req.Header.Get("Accept"))
}

func (suite *OAuthUtilsTestSuite) TestBuildUserInfoRequestWithInvalidURL() {
	logger := log.GetLogger()

	req, err := buildUserInfoRequest("://invalid-url", "token", logger)

	suite.Nil(req)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *OAuthUtilsTestSuite) TestSendUserInfoRequestSuccess() {
	mockHTTPClient := httpmock.NewHTTPClientInterfaceMock(suite.T())
	logger := log.GetLogger()

	userInfo := map[string]interface{}{
		"sub":   "user123",
		"name":  "Test User",
		"email": "test@example.com",
	}
	respBody, _ := json.Marshal(userInfo)
	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(respBody)),
	}

	req, _ := http.NewRequest("GET", "https://localhost:8090/userinfo", nil)
	mockHTTPClient.On("Do", req).Return(httpResp, nil)

	resp, err := sendUserInfoRequest(req, mockHTTPClient, logger)

	suite.Nil(err)
	suite.NotNil(resp)
	suite.Equal("user123", resp["sub"])
	suite.Equal("Test User", resp["name"])
	suite.Equal("test@example.com", resp["email"])
}

func (suite *OAuthUtilsTestSuite) TestSendUserInfoRequestHTTPError() {
	mockHTTPClient := httpmock.NewHTTPClientInterfaceMock(suite.T())
	logger := log.GetLogger()

	req, _ := http.NewRequest("GET", "https://localhost:8090/userinfo", nil)
	mockHTTPClient.On("Do", req).Return(nil, http.ErrHandlerTimeout)

	resp, err := sendUserInfoRequest(req, mockHTTPClient, logger)

	suite.Nil(resp)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *OAuthUtilsTestSuite) TestSendUserInfoRequestNonOKStatus() {
	mockHTTPClient := httpmock.NewHTTPClientInterfaceMock(suite.T())
	logger := log.GetLogger()

	errorBody := `{"error":"invalid_token"}`
	httpResp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(bytes.NewReader([]byte(errorBody))),
	}

	req, _ := http.NewRequest("GET", "https://localhost:8090/userinfo", nil)
	mockHTTPClient.On("Do", req).Return(httpResp, nil)

	resp, err := sendUserInfoRequest(req, mockHTTPClient, logger)

	suite.Nil(resp)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *OAuthUtilsTestSuite) TestSendUserInfoRequestInvalidJSON() {
	mockHTTPClient := httpmock.NewHTTPClientInterfaceMock(suite.T())
	logger := log.GetLogger()

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte("invalid json"))),
	}

	req, _ := http.NewRequest("GET", "https://localhost:8090/userinfo", nil)
	mockHTTPClient.On("Do", req).Return(httpResp, nil)

	resp, err := sendUserInfoRequest(req, mockHTTPClient, logger)

	suite.Nil(resp)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *OAuthUtilsTestSuite) TestProcessSubClaimSuccess() {
	tests := []struct {
		name         string
		userInfo     map[string]interface{}
		expectedSub  string
		expectName   string
		expectIDGone bool
	}{
		{
			name: "WithSub",
			userInfo: map[string]interface{}{
				"sub":  "user123",
				"name": "Test User",
			},
			expectedSub:  "user123",
			expectName:   "Test User",
			expectIDGone: false,
		},
		{
			name: "WithID",
			userInfo: map[string]interface{}{
				"id":   "user456",
				"name": "Test User",
			},
			expectedSub:  "user456",
			expectName:   "Test User",
			expectIDGone: true,
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			// copy the map to avoid mutation between subtests
			ui := make(map[string]interface{}, len(tc.userInfo))
			for k, v := range tc.userInfo {
				ui[k] = v
			}

			ProcessSubClaim(ui)
			suite.Equal(tc.expectedSub, ui["sub"])
			suite.Equal(tc.expectName, ui["name"])

			if tc.expectIDGone {
				suite.NotContains(ui, "id")
			}
		})
	}
}

func (suite *OAuthUtilsTestSuite) TestProcessSubClaimInvalid() {
	tests := []struct {
		name     string
		userInfo map[string]interface{}
		isNil    bool
	}{
		{
			name:     "Empty",
			userInfo: map[string]interface{}{},
			isNil:    false,
		},
		{
			name:     "Nil",
			userInfo: nil,
			isNil:    true,
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			if tc.isNil {
				// Should not panic for nil input
				suite.NotPanics(func() {
					ProcessSubClaim(nil)
				})
				return
			}

			// For non-nil (empty) map, ensure it remains empty after processing
			ui := make(map[string]interface{})
			ProcessSubClaim(ui)
			suite.Empty(ui)
		})
	}
}

func (suite *OAuthUtilsTestSuite) TestGetStringUserClaimValue() {
	tests := []struct {
		name     string
		userInfo map[string]interface{}
		claim    string
		expected string
	}{
		{
			name:     "String",
			userInfo: map[string]interface{}{"name": "Test User"},
			claim:    "name",
			expected: "Test User",
		},
		{
			name:     "Int",
			userInfo: map[string]interface{}{"age": 25},
			claim:    "age",
			expected: "25",
		},
		{
			name:     "Int64",
			userInfo: map[string]interface{}{"timestamp": int64(1234567890)},
			claim:    "timestamp",
			expected: "1234567890",
		},
		{
			name:     "Float64Whole",
			userInfo: map[string]interface{}{"id": float64(12345)},
			claim:    "id",
			expected: "12345",
		},
		{
			name:     "Float64Decimal",
			userInfo: map[string]interface{}{"rating": 4.5},
			claim:    "rating",
			expected: "4.500000",
		},
		{
			name:     "Bool",
			userInfo: map[string]interface{}{"verified": true},
			claim:    "verified",
			expected: "true",
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			result := GetStringUserClaimValue(tc.userInfo, tc.claim)
			suite.Equal(tc.expected, result)
		})
	}
}

func (suite *OAuthUtilsTestSuite) TestGetStringUserClaimValueWithMissingOrInvalidClaim() {
	tests := []struct {
		name     string
		userInfo map[string]interface{}
		claim    string
	}{
		{
			name:     "Missing",
			userInfo: map[string]interface{}{"name": "Test User"},
			claim:    "email",
		},
		{
			name:     "Empty",
			userInfo: map[string]interface{}{},
			claim:    "name",
		},
		{
			name:     "Nil",
			userInfo: nil,
			claim:    "name",
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			result := GetStringUserClaimValue(tc.userInfo, tc.claim)
			suite.Empty(result)
		})
	}
}

func (suite *OAuthUtilsTestSuite) TestParseIDPConfigWithLogoutAndJwks() {
	clientIDProp, _ := cmodels.NewProperty("client_id", "test_client", false)
	scopesProp, _ := cmodels.NewProperty("scopes", "openid,profile", false)
	logoutProp, _ := cmodels.NewProperty("logout_endpoint", "https://localhost:8090/logout", false)
	jwksProp, _ := cmodels.NewProperty("jwks_endpoint", "https://localhost:8090/jwks", false)

	idpDTO := &idp.IDPDTO{
		Properties: []cmodels.Property{
			*clientIDProp,
			*scopesProp,
			*logoutProp,
			*jwksProp,
		},
	}

	cfg, err := parseIDPConfig(idpDTO)
	suite.Nil(err)
	suite.NotNil(cfg)
	suite.Equal("https://localhost:8090/logout", cfg.OAuthEndpoints.LogoutEndpoint)
	suite.Equal("https://localhost:8090/jwks", cfg.OAuthEndpoints.JwksEndpoint)
}

// badCloseReadCloser returns data but Close returns an error to exercise defer close error handling.
type badCloseReadCloser struct {
	io.Reader
}

func (b badCloseReadCloser) Close() error { return errors.New("close failed") }

// badReadCloser returns an error on Read to simulate a failing response body read.
type badReadCloser struct{}

func (b badReadCloser) Read(p []byte) (int, error) { return 0, errors.New("read failed") }
func (b badReadCloser) Close() error               { return nil }

func (suite *OAuthUtilsTestSuite) TestSendUserInfoRequestReadErrorAndDoReturnsRespAndError() {
	logger := log.GetLogger()

	// Case: Do returns resp and an error
	userInfo := map[string]interface{}{"sub": "u1"}
	body, _ := json.Marshal(userInfo)
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}

	mockHTTPClient := httpmock.NewHTTPClientInterfaceMock(suite.T())
	req, _ := http.NewRequest("GET", "https://localhost:8090/userinfo", nil)
	mockHTTPClient.On("Do", req).Return(resp, errors.New("network issue"))

	got, err := sendUserInfoRequest(req, mockHTTPClient, logger)
	suite.Nil(got)
	suite.NotNil(err)

	// Case: response body Read returns error
	resp2 := &http.Response{StatusCode: http.StatusOK, Body: badReadCloser{}}
	mockHTTPClient2 := httpmock.NewHTTPClientInterfaceMock(suite.T())
	mockHTTPClient2.On("Do", req).Return(resp2, nil)

	got2, err2 := sendUserInfoRequest(req, mockHTTPClient2, logger)
	suite.Nil(got2)
	suite.NotNil(err2)
}

func (suite *OAuthUtilsTestSuite) TestSendTokenRequestCloseErrorAndDoReturnsRespAndError() {
	logger := log.GetLogger()

	// Successful token response but Close returns error (should not cause function to return error)
	tokenResp := TokenResponse{AccessToken: "atoken", TokenType: "Bearer"}
	b, _ := json.Marshal(tokenResp)
	resp := &http.Response{StatusCode: http.StatusOK, Body: badCloseReadCloser{bytes.NewReader(b)}}

	req, _ := http.NewRequest("POST", "https://localhost:8090/token", nil)
	mockHTTPClient := httpmock.NewHTTPClientInterfaceMock(suite.T())
	mockHTTPClient.On("Do", req).Return(resp, nil)

	got, err := sendTokenRequest(req, mockHTTPClient, logger)
	suite.NotNil(got)
	suite.Nil(err)

	// Do returns resp and error -> should return error
	resp2 := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(b))}
	mockHTTPClient2 := httpmock.NewHTTPClientInterfaceMock(suite.T())
	mockHTTPClient2.On("Do", req).Return(resp2, errors.New("network"))

	got2, err2 := sendTokenRequest(req, mockHTTPClient2, logger)
	suite.Nil(got2)
	suite.NotNil(err2)
}
