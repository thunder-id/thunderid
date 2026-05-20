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
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/cert"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

const (
	testClientID     = "test-client-id"
	testClientSecret = "test-secret"
	testEndpointURL  = "https://localhost:9443/oauth2/token"
)

type ClientAuthTestSuite struct {
	suite.Suite
	mockInboundClient *inboundclientmock.InboundClientServiceInterfaceMock
	mockAuthnProvider *managermock.AuthnProviderManagerInterfaceMock
	mockJwtService    *jwtmock.JWTServiceInterfaceMock
}

func TestClientAuthTestSuite(t *testing.T) {
	suite.Run(t, new(ClientAuthTestSuite))
}

func (suite *ClientAuthTestSuite) SetupTest() {
	suite.mockInboundClient = inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerInterfaceMock(suite.T())
	suite.mockJwtService = jwtmock.NewJWTServiceInterfaceMock(suite.T())

	// Default authn mock: return success for client secret authentication.
	// Tests that need failure override this with a fresh mock.
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidermgr.AuthnBasicResult{UserID: testClientID},
			(*serviceerror.ServiceError)(nil)).Maybe()
}

func (suite *ClientAuthTestSuite) TestAuthenticate_Success_ClientSecretPost() {
	clientSecret := testClientSecret
	mockApp := &inboundmodel.OAuthClient{
		ClientID:                testClientID,
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
	}

	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(mockApp, nil).Once()

	formData := url.Values{}
	formData.Set("client_id", testClientID)
	formData.Set("client_secret", clientSecret)

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.Nil(suite.T(), authErr)
	assert.NotNil(suite.T(), clientInfo)
	if clientInfo != nil {
		assert.Equal(suite.T(), testClientID, clientInfo.ClientID)
		assert.Equal(suite.T(), clientSecret, clientInfo.ClientSecret)
		assert.NotNil(suite.T(), clientInfo.OAuthApp)
		assert.Equal(suite.T(), testClientID, clientInfo.OAuthApp.ClientID)
	}
}

func (suite *ClientAuthTestSuite) TestAuthenticate_Success_ClientSecretBasic() {
	clientSecret := testClientSecret
	mockApp := &inboundmodel.OAuthClient{
		ClientID:                testClientID,
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretBasic,
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
	}

	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(mockApp, nil).Once()

	req, _ := http.NewRequest("POST", "/test", nil)
	req.SetBasicAuth(testClientID, clientSecret)

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.Nil(suite.T(), authErr)
	assert.NotNil(suite.T(), clientInfo)
	if clientInfo != nil {
		assert.Equal(suite.T(), testClientID, clientInfo.ClientID)
		assert.Equal(suite.T(), clientSecret, clientInfo.ClientSecret)
	}
}

func (suite *ClientAuthTestSuite) TestAuthenticate_Success_ClientSecretBasic_URLEncodedCredentials() {
	rawClientID := "client:id"
	rawClientSecret := "secret with spaces"
	encodedClientID := url.QueryEscape(rawClientID)
	encodedClientSecret := url.QueryEscape(rawClientSecret)
	mockApp := &inboundmodel.OAuthClient{
		ClientID:                rawClientID,
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretBasic,
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
	}

	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, rawClientID).
		Return(mockApp, nil).Once()

	// Manually construct the Basic Auth header with URL-encoded credentials.
	basicValue := base64.StdEncoding.EncodeToString([]byte(encodedClientID + ":" + encodedClientSecret))
	req, _ := http.NewRequest("POST", "/test", nil)
	req.Header.Set("Authorization", "Basic "+basicValue)

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.Nil(suite.T(), authErr)
	assert.NotNil(suite.T(), clientInfo)
	if clientInfo != nil {
		assert.Equal(suite.T(), rawClientID, clientInfo.ClientID)
		assert.Equal(suite.T(), rawClientSecret, clientInfo.ClientSecret)
	}
}

func (suite *ClientAuthTestSuite) TestAuthenticate_InvalidBasicAuth_BadPercentEncoding() {
	// Use an invalid percent-encoded value in the client ID.
	basicValue := base64.StdEncoding.EncodeToString([]byte("client%ZZid:secret"))
	req, _ := http.NewRequest("POST", "/test", nil)
	req.Header.Set("Authorization", "Basic "+basicValue)

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errInvalidAuthorizationHeader, authErr)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_InvalidBasicAuth_BadPercentEncodingInSecret() {
	// Client ID is valid but secret contains an invalid percent-encoded value.
	basicValue := base64.StdEncoding.EncodeToString([]byte("validclient:secret%ZZvalue"))
	req, _ := http.NewRequest("POST", "/test", nil)
	req.Header.Set("Authorization", "Basic "+basicValue)

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errInvalidAuthorizationHeader, authErr)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_Success_PublicClient() {
	mockApp := &inboundmodel.OAuthClient{
		ClientID:                "public-client-id",
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodNone,
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
		PublicClient:            true,
	}

	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "public-client-id").
		Return(mockApp, nil).Once()

	formData := url.Values{}
	formData.Set("client_id", "public-client-id")

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.Nil(suite.T(), authErr)
	assert.NotNil(suite.T(), clientInfo)
	if clientInfo != nil {
		assert.Equal(suite.T(), "public-client-id", clientInfo.ClientID)
		assert.Equal(suite.T(), "", clientInfo.ClientSecret)
	}
}

func (suite *ClientAuthTestSuite) TestAuthenticate_MissingClientID() {
	req, _ := http.NewRequest("POST", "/test", nil)
	_ = req.ParseForm()

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errMissingClientID, authErr)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, authErr.ErrorCode)
	assert.Equal(suite.T(), "Missing client_id parameter", authErr.ErrorDescription)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_EmptyClientIDInBasicAuth() {
	// Sending Basic Auth with empty client_id (username) should return invalid_request, not invalid_client.
	req, _ := http.NewRequest("POST", "/test", nil)
	req.SetBasicAuth("", testClientSecret)

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errMissingClientID, authErr)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, authErr.ErrorCode)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_EmptyClientIDAndSecretInBasicAuth() {
	// Sending Basic Auth with both empty client_id and client_secret should return invalid_request.
	req, _ := http.NewRequest("POST", "/test", nil)
	req.SetBasicAuth("", "")

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errMissingClientID, authErr)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, authErr.ErrorCode)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_MissingClientSecret() {
	formData := url.Values{}
	formData.Set("client_id", testClientID)

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()

	// This should succeed for public clients, but fail for confidential clients
	// Since we don't have an app yet, it will fail at app retrieval
	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(nil, nil).Once()

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errInvalidClientCredentials, authErr)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_InvalidBasicAuth() {
	req, _ := http.NewRequest("POST", "/test", nil)
	req.Header.Set("Authorization", "Basic invalid_base64")

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errInvalidAuthorizationHeader, authErr)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_InvalidAuthorizationHeader() {
	req, _ := http.NewRequest("POST", "/test", nil)
	req.Header.Set("Authorization", "Bearer token")

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errInvalidAuthorizationHeader, authErr)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_BothHeaderAndBody() {
	formData := url.Values{}
	formData.Set("client_id", testClientID)
	formData.Set("client_secret", testClientSecret)

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(testClientID, testClientSecret)
	_ = req.ParseForm()

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errMultipleAuthMethods, authErr)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, authErr.ErrorCode)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_ClientNotFound() {
	formData := url.Values{}
	formData.Set("client_id", "non-existent-client")
	formData.Set("client_secret", testClientSecret)

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()

	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "non-existent-client").
		Return(nil, nil).Once()

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errInvalidClientCredentials, authErr)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_InvalidClientSecret() {
	wrongSecret := "wrong-secret"
	mockApp := &inboundmodel.OAuthClient{
		ClientID:                testClientID,
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
	}

	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(mockApp, nil).Once()

	// Create a fresh authn mock that fails for wrong secret.
	failAuthnProvider := managermock.NewAuthnProviderManagerInterfaceMock(suite.T())
	failAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, (*authnprovidermgr.AuthnBasicResult)(nil),
			&serviceerror.ServiceError{
				Type:             serviceerror.ClientErrorType,
				Code:             authnprovidermgr.ErrorAuthenticationFailed.Code,
				Error:            i18ncore.I18nMessage{DefaultValue: "auth failed"},
				ErrorDescription: i18ncore.I18nMessage{DefaultValue: "wrong secret"},
			}).Maybe()

	formData := url.Values{}
	formData.Set("client_id", testClientID)
	formData.Set("client_secret", wrongSecret)

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, failAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errInvalidClientCredentials, authErr)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_WrongAuthMethod() {
	clientSecret := testClientSecret
	mockApp := &inboundmodel.OAuthClient{
		ClientID:                testClientID,
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
	}

	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(mockApp, nil).Once()

	// Try to use client_secret_basic when app only allows client_secret_post
	req, _ := http.NewRequest("POST", "/test", nil)
	req.SetBasicAuth(testClientID, clientSecret)

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errUnauthorizedAuthMethod, authErr)
	assert.Equal(suite.T(), constants.ErrorUnauthorizedClient, authErr.ErrorCode)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_PublicClientWithSecret() {
	mockApp := &inboundmodel.OAuthClient{
		ClientID:                "public-client-id",
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodNone,
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
		PublicClient:            true,
	}

	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "public-client-id").
		Return(mockApp, nil).Once()

	formData := url.Values{}
	formData.Set("client_id", "public-client-id")
	formData.Set("client_secret", "some-secret")

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()

	// Try to use client_secret_post with public client
	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errUnauthorizedAuthMethod, authErr)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_PublicClientMissingSecret() {
	mockApp := &inboundmodel.OAuthClient{
		ClientID:                "public-client-id",
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodNone,
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
		PublicClient:            true,
	}

	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "public-client-id").
		Return(mockApp, nil).Once()

	formData := url.Values{}
	formData.Set("client_id", "public-client-id")

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()

	// Public client with authMethod = none should succeed
	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.Nil(suite.T(), authErr)
	assert.NotNil(suite.T(), clientInfo)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_ClientIDMismatch_HeaderVsBody() {
	// Test that client_id in body mismatches client_id extracted from auth header.
	mockApp := &inboundmodel.OAuthClient{
		ClientID:                testClientID,
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretBasic,
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
	}

	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(mockApp, nil).Maybe()

	formData := url.Values{}
	formData.Set("client_id", "different-client-id") // Mismatch with basic auth clientID.

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(testClientID, testClientSecret)
	_ = req.ParseForm()

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errClientIDMismatch, authErr)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_ServiceError() {
	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(nil, errors.New("internal error")).Once()

	formData := url.Values{}
	formData.Set("client_id", testClientID)
	formData.Set("client_secret", testClientSecret)

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errInvalidClientCredentials, authErr)
}

// buildTestJWT constructs a fake JWT string (header.payload.signature) for testing purposes.
// It accepts header claims and payload claims as maps.
func buildTestJWT(headerClaims, payloadClaims map[string]any) string {
	headerJSON, _ := json.Marshal(headerClaims)
	payloadJSON, _ := json.Marshal(payloadClaims)
	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signature := base64.RawURLEncoding.EncodeToString([]byte("fake-signature"))
	return header + "." + payload + "." + signature
}

// buildTestRSAJWKS generates an RSA key pair and returns the JWKS JSON string for the given kid.
func buildTestRSAJWKS(kid string) string {
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pubKey := &privKey.PublicKey
	nBytes := pubKey.N.Bytes()
	eBytes := big.NewInt(int64(pubKey.E)).Bytes()
	jwk := map[string]any{
		"kty": "RSA",
		"kid": kid,
		"n":   base64.RawURLEncoding.EncodeToString(nBytes),
		"e":   base64.RawURLEncoding.EncodeToString(eBytes),
	}
	jwks := map[string]any{"keys": []map[string]any{jwk}}
	jwksJSON, _ := json.Marshal(jwks)
	return string(jwksJSON)
}

// buildFakeJWTWithSub constructs a fake JWT with a given subject and kid in the header.
func buildFakeJWTWithSub(subject string) string {
	return buildTestJWT(
		map[string]any{"alg": "RS256", "kid": "test-kid", "typ": "JWT"},
		map[string]any{"sub": subject, "aud": "https://token"},
	)
}

// buildFakeJWTWithPayload constructs a fake JWT string with a custom payload for testing purposes.
func buildFakeJWTWithPayload(payloadJSON string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"test-kid","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(payloadJSON))
	signature := base64.RawURLEncoding.EncodeToString([]byte("fake-signature"))
	return header + "." + payload + "." + signature
}

func (suite *ClientAuthTestSuite) TestAuthenticate_Success_PrivateKeyJWT() {
	jwksJSON := buildTestRSAJWKS("test-kid")
	assertion := buildFakeJWTWithSub(testClientID)
	mockApp := &inboundmodel.OAuthClient{
		ClientID:                testClientID,
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodPrivateKeyJWT,
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
		Certificate:             &inboundmodel.Certificate{Value: jwksJSON},
	}

	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(mockApp, nil).Once()
	suite.mockJwtService.EXPECT().
		VerifyJWTWithPublicKey(assertion, mock.Anything, "https://localhost:9443/oauth2/token", testClientID).
		Return(nil)

	formData := url.Values{}
	formData.Set("client_assertion_type", constants.SupportedClientAssertionType)
	formData.Set("client_assertion", assertion)

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.Nil(suite.T(), authErr)
	assert.NotNil(suite.T(), clientInfo)
	if clientInfo != nil {
		assert.Equal(suite.T(), testClientID, clientInfo.ClientID)
		assert.NotNil(suite.T(), clientInfo.OAuthApp)
		assert.Equal(suite.T(), testClientID, clientInfo.OAuthApp.ClientID)
	}
}

func (suite *ClientAuthTestSuite) TestAuthenticate_Success_PrivateKeyJWT_WithClientIDInBody() {
	jwksJSON := buildTestRSAJWKS("test-kid")
	assertion := buildFakeJWTWithSub(testClientID)
	mockApp := &inboundmodel.OAuthClient{
		ClientID:                testClientID,
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodPrivateKeyJWT,
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
		Certificate:             &inboundmodel.Certificate{Value: jwksJSON},
	}

	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(mockApp, nil).Once()
	suite.mockJwtService.EXPECT().
		VerifyJWTWithPublicKey(assertion, mock.Anything, "https://localhost:9443/oauth2/token", testClientID).
		Return(nil)

	formData := url.Values{}
	formData.Set("client_id", testClientID)
	formData.Set("client_assertion_type", constants.SupportedClientAssertionType)
	formData.Set("client_assertion", assertion)

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.Nil(suite.T(), authErr)
	assert.NotNil(suite.T(), clientInfo)
	if clientInfo != nil {
		assert.Equal(suite.T(), testClientID, clientInfo.ClientID)
	}
}

func (suite *ClientAuthTestSuite) TestAuthenticate_PrivateKeyJWT_UnsupportedAssertionType() {
	formData := url.Values{}
	formData.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:saml2-bearer")
	formData.Set("client_assertion", "some-assertion")

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errInvalidClientAssertion, authErr)
	assert.Equal(suite.T(), constants.ErrorInvalidClient, authErr.ErrorCode)
	assert.Equal(suite.T(), "Invalid client assertion", authErr.ErrorDescription)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_PrivateKeyJWT_OnlyAssertionTypeProvided() {
	formData := url.Values{}
	formData.Set("client_assertion_type", constants.SupportedClientAssertionType)

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errInvalidClientAssertion, authErr)
	assert.Equal(suite.T(), constants.ErrorInvalidClient, authErr.ErrorCode)
	assert.Equal(suite.T(), "Invalid client assertion", authErr.ErrorDescription)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_PrivateKeyJWT_OnlyAssertionProvided() {
	assertion := buildFakeJWTWithSub(testClientID)

	formData := url.Values{}
	formData.Set("client_assertion", assertion)

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()

	// Only client_assertion without client_assertion_type: assertion_type is empty,
	// but the code detects it as private_key_jwt since client_assertion is present.
	// Then it checks assertion_type != SupportedClientAssertionType, which fails.
	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errInvalidClientAssertion, authErr)
	assert.Equal(suite.T(), constants.ErrorInvalidClient, authErr.ErrorCode)
	assert.Equal(suite.T(), "Invalid client assertion", authErr.ErrorDescription)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_PrivateKeyJWT_InvalidAssertionFormat() {
	tests := []struct {
		name      string
		assertion string
	}{
		{"invalid JWT format", "not-a-valid-jwt"},
		{"missing sub claim", buildFakeJWTWithPayload(`{"aud":"https://token","iss":"some-issuer"}`)},
		{"empty sub claim", buildFakeJWTWithPayload(`{"sub":"","aud":"https://token"}`)},
		{"non-string sub claim", buildFakeJWTWithPayload(`{"sub":12345,"aud":"https://token"}`)},
	}
	for _, tc := range tests {
		suite.Run(tc.name, func() {
			formData := url.Values{}
			formData.Set("client_assertion_type", constants.SupportedClientAssertionType)
			formData.Set("client_assertion", tc.assertion)

			req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			_ = req.ParseForm()

			clientInfo, authErr := authenticate(
				req.Context(), req,
				suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

			assert.NotNil(suite.T(), authErr)
			assert.Nil(suite.T(), clientInfo)
			assert.Equal(suite.T(), errInvalidClientAssertion, authErr)
			assert.Equal(suite.T(), constants.ErrorInvalidClient, authErr.ErrorCode)
			assert.Equal(suite.T(), "Invalid client assertion", authErr.ErrorDescription)
		})
	}
}

func (suite *ClientAuthTestSuite) TestAuthenticate_PrivateKeyJWT_ClientNotFound() {
	assertion := buildFakeJWTWithSub("unknown-client")

	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "unknown-client").
		Return(nil, nil).Once()

	formData := url.Values{}
	formData.Set("client_assertion_type", constants.SupportedClientAssertionType)
	formData.Set("client_assertion", assertion)

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errInvalidClientCredentials, authErr)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_PrivateKeyJWT_AuthMethodNotAllowed() {
	assertion := buildFakeJWTWithSub(testClientID)
	// App only allows client_secret_post, not private_key_jwt
	mockApp := &inboundmodel.OAuthClient{
		ClientID:                testClientID,
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodClientSecretPost,
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
	}

	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(mockApp, nil).Once()

	formData := url.Values{}
	formData.Set("client_assertion_type", constants.SupportedClientAssertionType)
	formData.Set("client_assertion", assertion)

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errUnauthorizedAuthMethod, authErr)
	assert.Equal(suite.T(), constants.ErrorUnauthorizedClient, authErr.ErrorCode)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_PrivateKeyJWT_AssertionValidationFails() {
	assertion := buildFakeJWTWithSub(testClientID)
	mockApp := &inboundmodel.OAuthClient{
		ClientID:                testClientID,
		TokenEndpointAuthMethod: constants.TokenEndpointAuthMethodPrivateKeyJWT,
		GrantTypes:              []constants.GrantType{constants.GrantTypeAuthorizationCode},
		// No certificate configured, so ValidateClientAssertion will return false
	}

	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(mockApp, nil).Once()

	formData := url.Values{}
	formData.Set("client_assertion_type", constants.SupportedClientAssertionType)
	formData.Set("client_assertion", assertion)

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errInvalidClientAssertion, authErr)
	assert.Equal(suite.T(), constants.ErrorInvalidClient, authErr.ErrorCode)
	assert.Equal(suite.T(), "Invalid client assertion", authErr.ErrorDescription)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_PrivateKeyJWT_ClientIDMismatch() {
	assertion := buildFakeJWTWithSub("different-client-id")

	formData := url.Values{}
	formData.Set("client_id", testClientID)
	formData.Set("client_assertion_type", constants.SupportedClientAssertionType)
	formData.Set("client_assertion", assertion)

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errClientIDMismatch, authErr)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, authErr.ErrorCode)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_PrivateKeyJWT_WithBasicAuth_MultipleAuthMethods() {
	assertion := buildFakeJWTWithSub(testClientID)

	formData := url.Values{}
	formData.Set("client_assertion_type", constants.SupportedClientAssertionType)
	formData.Set("client_assertion", assertion)

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(testClientID, testClientSecret)
	_ = req.ParseForm()

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errMultipleAuthMethods, authErr)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, authErr.ErrorCode)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_PrivateKeyJWT_WithClientSecret_MultipleAuthMethods() {
	assertion := buildFakeJWTWithSub(testClientID)

	formData := url.Values{}
	formData.Set("client_id", testClientID)
	formData.Set("client_secret", testClientSecret)
	formData.Set("client_assertion_type", constants.SupportedClientAssertionType)
	formData.Set("client_assertion", assertion)

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errMultipleAuthMethods, authErr)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, authErr.ErrorCode)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_PrivateKeyJWT_ServiceError() {
	assertion := buildFakeJWTWithSub(testClientID)

	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(nil, errors.New("internal error")).Once()

	formData := url.Values{}
	formData.Set("client_assertion_type", constants.SupportedClientAssertionType)
	formData.Set("client_assertion", assertion)

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errInvalidClientCredentials, authErr)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_PrivateKeyJWT_InvalidBase64Payload() {
	// Build a JWT with invalid base64 in the payload segment
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`))
	signature := base64.RawURLEncoding.EncodeToString([]byte("fake-signature"))
	invalidJWT := header + ".!!!invalid-base64!!!." + signature

	formData := url.Values{}
	formData.Set("client_assertion_type", constants.SupportedClientAssertionType)
	formData.Set("client_assertion", invalidJWT)

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errInvalidClientAssertion, authErr)
	assert.Equal(suite.T(), constants.ErrorInvalidClient, authErr.ErrorCode)
	assert.Equal(suite.T(), "Invalid client assertion", authErr.ErrorDescription)
}

func (suite *ClientAuthTestSuite) TestAuthenticate_PrivateKeyJWT_InvalidJSONPayload() {
	// Build a JWT with valid base64 but invalid JSON in the payload
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`not-json`))
	signature := base64.RawURLEncoding.EncodeToString([]byte("fake-signature"))
	invalidJWT := header + "." + payload + "." + signature

	formData := url.Values{}
	formData.Set("client_assertion_type", constants.SupportedClientAssertionType)
	formData.Set("client_assertion", invalidJWT)

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()

	clientInfo, authErr := authenticate(
		req.Context(), req,
		suite.mockInboundClient, suite.mockAuthnProvider, suite.mockJwtService, testEndpointURL)

	assert.NotNil(suite.T(), authErr)
	assert.Nil(suite.T(), clientInfo)
	assert.Equal(suite.T(), errInvalidClientAssertion, authErr)
	assert.Equal(suite.T(), constants.ErrorInvalidClient, authErr.ErrorCode)
	assert.Equal(suite.T(), "Invalid client assertion", authErr.ErrorDescription)
}

// validateClientAssertion tests

func (suite *ClientAuthTestSuite) TestValidateClientAssertion_NilCertificate() {
	oauthApp := &inboundmodel.OAuthClient{
		ClientID:    "test-client",
		Certificate: nil,
	}

	err := validateClientAssertion(
		oauthApp, suite.mockJwtService, testEndpointURL, "test-client",
		"some.jwt.token")
	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "no certificate configured")
}

func (suite *ClientAuthTestSuite) TestValidateClientAssertion_JWKSURI_Success() {
	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Certificate: &inboundmodel.Certificate{
			Type:  cert.CertificateTypeJWKSURI,
			Value: "https://example.com/.well-known/jwks.json",
		},
	}

	assertion := buildFakeJWTWithSub("test-client")

	suite.mockJwtService.EXPECT().
		VerifyJWTWithJWKS(assertion, "https://example.com/.well-known/jwks.json",
			"https://localhost:9443/oauth2/token", "test-client").
		Return(nil)

	err := validateClientAssertion(
		oauthApp, suite.mockJwtService, testEndpointURL, "test-client", assertion)
	assert.Nil(suite.T(), err)
}

func (suite *ClientAuthTestSuite) TestValidateClientAssertion_JWKSURI_VerificationFails() {
	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Certificate: &inboundmodel.Certificate{
			Type:  cert.CertificateTypeJWKSURI,
			Value: "https://example.com/.well-known/jwks.json",
		},
	}

	assertion := buildFakeJWTWithSub("test-client")

	suite.mockJwtService.EXPECT().
		VerifyJWTWithJWKS(assertion, "https://example.com/.well-known/jwks.json",
			"https://localhost:9443/oauth2/token", "test-client").
		Return(&serviceerror.ServiceError{Error: i18ncore.I18nMessage{DefaultValue: "verification failed"}})

	err := validateClientAssertion(
		oauthApp, suite.mockJwtService, testEndpointURL, "test-client", assertion)
	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "client assertion verification with JWKS URI failed")
}

func (suite *ClientAuthTestSuite) TestValidateClientAssertion_InvalidJWKSJSON() {
	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Certificate: &inboundmodel.Certificate{
			Type:  "jwks",
			Value: "not-valid-json",
		},
	}

	err := validateClientAssertion(oauthApp, suite.mockJwtService, testEndpointURL,
		"test-client", "some.jwt.token")
	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "invalid JWKS certificate format")
}

func (suite *ClientAuthTestSuite) TestValidateClientAssertion_InvalidJWTFormat() {
	jwksJSON := buildTestRSAJWKS("test-kid")
	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Certificate: &inboundmodel.Certificate{
			Type:  "jwks",
			Value: jwksJSON,
		},
	}

	err := validateClientAssertion(oauthApp, suite.mockJwtService, testEndpointURL,
		"test-client", "invalid-jwt")
	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to decode header")
}

func (suite *ClientAuthTestSuite) TestValidateClientAssertion_MissingKidInHeader() {
	jwksJSON := buildTestRSAJWKS("test-kid")
	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Certificate: &inboundmodel.Certificate{
			Type:  "jwks",
			Value: jwksJSON,
		},
	}

	fakeJWT := buildTestJWT(map[string]any{"alg": "RS256", "typ": "JWT"}, map[string]any{"sub": "test-client"})

	err := validateClientAssertion(
		oauthApp, suite.mockJwtService, testEndpointURL, "test-client", fakeJWT)
	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "JWT header missing 'kid' claim")
}

func (suite *ClientAuthTestSuite) TestValidateClientAssertion_EmptyKidInHeader() {
	jwksJSON := buildTestRSAJWKS("test-kid")
	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Certificate: &inboundmodel.Certificate{
			Type:  "jwks",
			Value: jwksJSON,
		},
	}

	fakeJWT := buildTestJWT(map[string]any{"alg": "RS256", "kid": "", "typ": "JWT"},
		map[string]any{"sub": "test-client"})

	err := validateClientAssertion(
		oauthApp, suite.mockJwtService, testEndpointURL, "test-client", fakeJWT)
	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "JWT header missing 'kid' claim")
}

func (suite *ClientAuthTestSuite) TestValidateClientAssertion_KidNotAString() {
	jwksJSON := buildTestRSAJWKS("test-kid")
	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Certificate: &inboundmodel.Certificate{
			Type:  "jwks",
			Value: jwksJSON,
		},
	}

	fakeJWT := buildTestJWT(map[string]any{"alg": "RS256", "kid": 12345, "typ": "JWT"},
		map[string]any{"sub": "test-client"})

	err := validateClientAssertion(
		oauthApp, suite.mockJwtService, testEndpointURL, "test-client", fakeJWT)
	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "JWT header missing 'kid' claim")
}

func (suite *ClientAuthTestSuite) TestValidateClientAssertion_NoMatchingKidInJWKS() {
	jwksJSON := buildTestRSAJWKS("different-kid")
	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Certificate: &inboundmodel.Certificate{
			Type:  "jwks",
			Value: jwksJSON,
		},
	}

	fakeJWT := buildTestJWT(map[string]any{"alg": "RS256", "kid": "test-kid", "typ": "JWT"},
		map[string]any{"sub": "test-client"})

	err := validateClientAssertion(
		oauthApp, suite.mockJwtService, testEndpointURL, "test-client", fakeJWT)
	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "no matching key found in JWKS")
}

func (suite *ClientAuthTestSuite) TestValidateClientAssertion_InvalidJWKCannotConvertToPublicKey() {
	invalidJWK := map[string]any{
		"kty": "RSA",
		"kid": "test-kid",
	}
	jwks := map[string]any{"keys": []map[string]any{invalidJWK}}
	jwksJSON, _ := json.Marshal(jwks)

	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Certificate: &inboundmodel.Certificate{
			Type:  "jwks",
			Value: string(jwksJSON),
		},
	}

	fakeJWT := buildTestJWT(map[string]any{"alg": "RS256", "kid": "test-kid", "typ": "JWT"},
		map[string]any{"sub": "test-client"})

	err := validateClientAssertion(
		oauthApp, suite.mockJwtService, testEndpointURL, "test-client", fakeJWT)
	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to convert JWK to public key")
}

func (suite *ClientAuthTestSuite) TestValidateClientAssertion_VerificationFails() {
	jwksJSON := buildTestRSAJWKS("test-kid")
	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Certificate: &inboundmodel.Certificate{
			Type:  "jwks",
			Value: jwksJSON,
		},
	}

	fakeJWT := buildTestJWT(map[string]any{"alg": "RS256", "kid": "test-kid", "typ": "JWT"},
		map[string]any{"sub": "test-client"})

	suite.mockJwtService.EXPECT().
		VerifyJWTWithPublicKey(fakeJWT, mock.Anything, "https://localhost:9443/oauth2/token", "test-client").
		Return(&serviceerror.ServiceError{
			Code:  "JWT-00001",
			Type:  serviceerror.ClientErrorType,
			Error: i18ncore.I18nMessage{DefaultValue: "invalid_token"},
		})

	err := validateClientAssertion(
		oauthApp, suite.mockJwtService, testEndpointURL, "test-client", fakeJWT)
	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "client assertion verification failed")
}

func (suite *ClientAuthTestSuite) TestValidateClientAssertion_Success() {
	jwksJSON := buildTestRSAJWKS("test-kid")
	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Certificate: &inboundmodel.Certificate{
			Type:  "jwks",
			Value: jwksJSON,
		},
	}

	fakeJWT := buildTestJWT(map[string]any{"alg": "RS256", "kid": "test-kid", "typ": "JWT"},
		map[string]any{"sub": "test-client"})

	suite.mockJwtService.EXPECT().
		VerifyJWTWithPublicKey(fakeJWT, mock.Anything, "https://localhost:9443/oauth2/token", "test-client").
		Return(nil)

	err := validateClientAssertion(
		oauthApp, suite.mockJwtService, testEndpointURL, "test-client", fakeJWT)
	assert.Nil(suite.T(), err)
}

func (suite *ClientAuthTestSuite) TestValidateClientAssertion_EmptyJWKSKeys() {
	jwks := map[string]any{"keys": []map[string]any{}}
	jwksJSON, _ := json.Marshal(jwks)

	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Certificate: &inboundmodel.Certificate{
			Type:  "jwks",
			Value: string(jwksJSON),
		},
	}

	fakeJWT := buildTestJWT(map[string]any{"alg": "RS256", "kid": "test-kid", "typ": "JWT"},
		map[string]any{"sub": "test-client"})

	err := validateClientAssertion(
		oauthApp, suite.mockJwtService, testEndpointURL, "test-client", fakeJWT)
	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "no matching key found in JWKS")
}

func (suite *ClientAuthTestSuite) TestValidateClientAssertion_MultipleKeysMatchesCorrectKid() {
	jwksJSON1 := buildTestRSAJWKS("kid-1")
	jwksJSON2 := buildTestRSAJWKS("kid-2")

	var jwks1, jwks2 struct {
		Keys []map[string]any `json:"keys"`
	}
	_ = json.Unmarshal([]byte(jwksJSON1), &jwks1)
	_ = json.Unmarshal([]byte(jwksJSON2), &jwks2)

	combinedJWKS := map[string]any{
		"keys": []map[string]any{jwks1.Keys[0], jwks2.Keys[0]},
	}
	combinedJSON, _ := json.Marshal(combinedJWKS)

	oauthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Certificate: &inboundmodel.Certificate{
			Type:  "jwks",
			Value: string(combinedJSON),
		},
	}

	fakeJWT := buildTestJWT(map[string]any{"alg": "RS256", "kid": "kid-2", "typ": "JWT"},
		map[string]any{"sub": "test-client"})

	suite.mockJwtService.EXPECT().
		VerifyJWTWithPublicKey(fakeJWT, mock.Anything, "https://localhost:9443/oauth2/token", "test-client").
		Return(nil)

	err := validateClientAssertion(
		oauthApp, suite.mockJwtService, testEndpointURL, "test-client", fakeJWT)
	assert.Nil(suite.T(), err)
}
