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

package testutils

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// InitiateAuthorizationFlow starts the OAuth2 authorization flow
func InitiateAuthorizationFlow(clientID, redirectURI, responseType, scope, state string) (*http.Response, error) {
	return initiateAuthorizationFlow(clientID, redirectURI, responseType, scope, state, "", "", "", "", "", "")
}

// InitiateAuthorizationFlowWithResource starts the OAuth2 authorization flow with resource parameter
func InitiateAuthorizationFlowWithResource(clientID, redirectURI, responseType, scope, state,
	resource string) (*http.Response, error) {
	return initiateAuthorizationFlow(clientID, redirectURI, responseType, scope, state, resource, "", "", "", "", "")
}

// InitiateAuthorizationFlowWithPKCE starts the OAuth2 authorization flow with PKCE parameters
func InitiateAuthorizationFlowWithPKCE(clientID, redirectURI, responseType, scope, state, resource,
	codeChallenge, codeChallengeMethod string) (*http.Response, error) {
	return initiateAuthorizationFlow(clientID, redirectURI, responseType, scope, state, resource,
		codeChallenge, codeChallengeMethod, "", "", "")
}

// InitiateAuthorizationFlowWithClaims starts the OAuth2 authorization flow with claims parameter
func InitiateAuthorizationFlowWithClaims(
	clientID, redirectURI, responseType, scope, state, claimsParam string,
) (*http.Response, error) {
	return initiateAuthorizationFlow(
		clientID, redirectURI, responseType, scope, state, "", "", "", claimsParam, "", "")
}

// InitiateAuthorizationFlowWithClaimsLocales starts the OAuth2 authorization flow with claims_locales parameter
func InitiateAuthorizationFlowWithClaimsLocales(
	clientID, redirectURI, responseType, scope, state, claimsLocales string,
) (*http.Response, error) {
	return initiateAuthorizationFlow(
		clientID, redirectURI, responseType, scope, state, "", "", "", "", claimsLocales, "",
	)
}

// InitiateAuthorizationFlowWithNonce starts the OAuth2 authorization flow with nonce parameter
func InitiateAuthorizationFlowWithNonce(
	clientID, redirectURI, responseType, scope, state, nonce string,
) (*http.Response, error) {
	return initiateAuthorizationFlow(
		clientID, redirectURI, responseType, scope, state,
		"", "", "", "", "", nonce,
	)
}

// initiateAuthorizationFlow starts the OAuth2 authorization flow with all optional parameters.
// clientID, redirectURI, responseType, scope, and state are required parameters.
// resource, codeChallenge, codeChallengeMethod, claimsParam, and claimsLocales, and nonce are optional parameters.
func initiateAuthorizationFlow(clientID, redirectURI, responseType, scope, state, resource,
	codeChallenge, codeChallengeMethod, claimsParam, claimsLocales, nonce string) (*http.Response, error) {
	authURL := TestServerURL + "/oauth2/authorize"
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", responseType)
	params.Set("scope", scope)
	params.Set("state", state)
	if resource != "" {
		params.Set("resource", resource)
	}
	if codeChallenge != "" {
		params.Set("code_challenge", codeChallenge)
		params.Set("code_challenge_method", codeChallengeMethod)
	}
	if claimsParam != "" {
		params.Set("claims", claimsParam)
	}
	if claimsLocales != "" {
		params.Set("claims_locales", claimsLocales)
	}
	if nonce != "" {
		params.Set("nonce", nonce)
	}

	req, err := http.NewRequest("GET", authURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create authorization request: %w", err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send authorization request: %w", err)
	}

	return resp, nil
}

// ExecuteAuthenticationFlow executes an authentication flow and returns the flow step.
func ExecuteAuthenticationFlow(executionId string, inputs map[string]string, action string,
	challengeToken ...string) (*FlowStep, error) {
	flowData := map[string]interface{}{
		"executionId": executionId,
	}

	if len(inputs) > 0 {
		flowData["inputs"] = inputs
	}
	if action != "" {
		flowData["action"] = action
	}
	if len(challengeToken) > 0 && challengeToken[0] != "" {
		flowData["challengeToken"] = challengeToken[0]
	}

	flowJSON, err := json.Marshal(flowData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal flow data: %w", err)
	}

	req, err := http.NewRequest("POST", TestServerURL+"/flow/execute", bytes.NewBuffer(flowJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create flow request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute flow: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("flow execution failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var flowStep FlowStep
	err = json.Unmarshal(bodyBytes, &flowStep)
	if err != nil {
		return nil, fmt.Errorf("failed to decode flow response: %w", err)
	}

	return &flowStep, nil
}

// CompleteAuthorization completes the authorization using the assertion
func CompleteAuthorization(authID, assertion string) (*AuthorizationResponse, error) {
	authzData := map[string]interface{}{
		"authId":    authID,
		"assertion": assertion,
	}

	authzJSON, err := json.Marshal(authzData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal authorization data: %w", err)
	}

	req, err := http.NewRequest("POST", TestServerURL+"/oauth2/auth/callback", bytes.NewBuffer(authzJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create authorization completion request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to complete authorization: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("authorization completion failed with status %d: %s",
			resp.StatusCode, string(bodyBytes))
	}

	var authzResponse AuthorizationResponse
	err = json.NewDecoder(resp.Body).Decode(&authzResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to decode authorization response: %w", err)
	}

	return &authzResponse, nil
}

// RequestToken performs a token request and returns raw HTTP result for both success and failure scenarios.
// grantType, code, and redirectURI are sent in the form body, while client credentials are sent via HTTP
// Basic Auth header.
func RequestToken(clientID, clientSecret, code, redirectURI, grantType string) (*TokenHTTPResult, error) {
	return requestToken(clientID, clientSecret, code, redirectURI, grantType, false, "")
}

// RequestTokenWithPKCE performs a token request with PKCE and returns raw HTTP result for both success and
// failure scenarios.
// grantType, code, redirectURI, and codeVerifier are sent in the form body, while client credentials are
// sent via HTTP Basic Auth header.
func RequestTokenWithPKCE(clientID, clientSecret, code, redirectURI, grantType, codeVerifier string) (
	*TokenHTTPResult, error) {
	return requestToken(clientID, clientSecret, code, redirectURI, grantType, true, codeVerifier)
}

// requestToken performs a token request and returns raw HTTP result for both success and failure scenarios.
// grantType, code, and redirectURI are required parameters.
// If tokenAuthInBody is true, client credentials are sent in the request body; otherwise, HTTP Basic Auth
// is used. codeVerifier is required for PKCE token requests.
func requestToken(clientID, clientSecret, code, redirectURI, grantType string, tokenAuthInBody bool,
	codeVerifier string) (*TokenHTTPResult, error) {
	tokenURL := TestServerURL + "/oauth2/token"
	tokenData := url.Values{}

	tokenData.Set("grant_type", grantType)
	tokenData.Set("code", code)
	tokenData.Set("redirect_uri", redirectURI)
	if codeVerifier != "" {
		tokenData.Set("code_verifier", codeVerifier)
	}
	if tokenAuthInBody {
		tokenData.Set("client_id", clientID)
		if clientSecret != "" {
			tokenData.Set("client_secret", clientSecret)
		}
	}

	req, err := http.NewRequest("POST", tokenURL, bytes.NewBufferString(tokenData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if !tokenAuthInBody {
		req.SetBasicAuth(clientID, clientSecret)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	result := &TokenHTTPResult{
		StatusCode: resp.StatusCode,
		Body:       body,
	}

	// Only try to decode token response if status is 200
	if resp.StatusCode == http.StatusOK {
		var tokenResponse TokenResponse
		if err := json.Unmarshal(body, &tokenResponse); err != nil {
			return nil, fmt.Errorf("failed to unmarshal token response: %w", err)
		}
		result.Token = &tokenResponse
	}

	return result, nil
}

// ExtractAuthorizationCode extracts the authorization code from the redirect URI
func ExtractAuthorizationCode(redirectURI string) (string, error) {
	parsedURL, err := url.Parse(redirectURI)
	if err != nil {
		return "", fmt.Errorf("failed to parse redirect URI: %w", err)
	}

	code := parsedURL.Query().Get("code")
	if code == "" {
		return "", fmt.Errorf("authorization code not found in redirect URI")
	}

	return code, nil
}

// ExtractAuthData extracts auth ID and flow ID from the authorization redirect
func ExtractAuthData(location string) (string, string, error) {
	redirectURL, err := url.Parse(location)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse redirect URL: %w", err)
	}

	authID := redirectURL.Query().Get("authId")
	if authID == "" {
		return "", "", fmt.Errorf("authId not found in redirect")
	}

	executionId := redirectURL.Query().Get("executionId")
	if executionId == "" {
		return "", "", fmt.Errorf("executionId not found in redirect")
	}

	return authID, executionId, nil
}

// ValidateOAuth2ErrorRedirect validates OAuth2 error redirect responses
func ValidateOAuth2ErrorRedirect(location string, expectedError string,
	expectedErrorDescription string) error {
	parsedURL, err := url.Parse(location)
	if err != nil {
		return fmt.Errorf("failed to parse redirect URL: %w", err)
	}

	queryParams := parsedURL.Query()

	// First check for OAuth2 error parameters (error, error_description)
	actualError := queryParams.Get("error")
	if actualError != "" {
		if actualError != expectedError {
			return fmt.Errorf("expected OAuth2 error '%s', got '%s'", expectedError, actualError)
		}

		if expectedErrorDescription != "" {
			actualErrorDescription := queryParams.Get("error_description")
			if actualErrorDescription != expectedErrorDescription {
				return fmt.Errorf("expected error_description '%s', got '%s'", expectedErrorDescription, actualErrorDescription)
			}
		}

		return nil
	}

	// Check for server error page parameters (errorCode, errorMessage)
	actualErrorCode := queryParams.Get("errorCode")
	if actualErrorCode != "" {
		if actualErrorCode != expectedError {
			return fmt.Errorf("expected error code '%s', got '%s'", expectedError, actualErrorCode)
		}

		if expectedErrorDescription != "" {
			actualErrorMessage := queryParams.Get("errorMessage")
			if actualErrorMessage != expectedErrorDescription {
				return fmt.Errorf("expected error message '%s', got '%s'", expectedErrorDescription, actualErrorMessage)
			}
		}

		return nil
	}

	return fmt.Errorf(
		"no error parameters found in redirect URL (neither 'error'/'error_description' nor " +
			"'errorCode'/'errorMessage')")
}

// ObtainAccessTokenWithPassword performs the complete OAuth authorization code flow with password
// authentication and returns a TokenResponse with the access token and expiry information.
// clientSecret is optional and can be provided for confidential clients
// and use client_secret_post authentication in the token request.
func ObtainAccessTokenWithPassword(clientID, redirectURI, scope, username, password string,
	usePKCE bool, optionalParams ...string) (*TokenResponse, error) {
	clientSecret := ""
	if len(optionalParams) > 0 {
		clientSecret = optionalParams[0]
	}

	var codeVerifier string
	var codeChallenge string

	// Generate PKCE parameters if enabled
	if usePKCE {
		var err error
		codeVerifier, err = generateCodeVerifier()
		if err != nil {
			return nil, fmt.Errorf("failed to generate code verifier: %w", err)
		}
		codeChallenge = generateCodeChallenge(codeVerifier)
		log.Printf("Generated PKCE - Verifier length: %d, Challenge: %s", len(codeVerifier), codeChallenge)
	}

	// Step 1: Initiate authorization flow with PKCE
	resp, err := InitiateAuthorizationFlowWithPKCE(clientID, redirectURI, "code", scope, "test-state", "",
		codeChallenge, "S256")
	if err != nil {
		return nil, fmt.Errorf("failed to initiate authorization: %w", err)
	}
	defer resp.Body.Close()

	// Check for redirect
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther &&
		resp.StatusCode != http.StatusTemporaryRedirect {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("expected redirect response, got status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	location := resp.Header.Get("Location")
	if location == "" {
		return nil, fmt.Errorf("no Location header in authorization response")
	}

	log.Printf("Authorization redirect location: %s", location)
	// Step 2: Extract auth ID and flow ID
	authID, executionId, err := ExtractAuthData(location)
	if err != nil {
		return nil, fmt.Errorf("failed to extract auth ID: %w", err)
	}

	// Step 3: Execute initial authentication flow step (to get to the login prompt)
	initialStep, err := ExecuteAuthenticationFlow(executionId, nil, "")
	if err != nil {
		return nil, fmt.Errorf("failed to execute initial authentication flow: %w", err)
	}

	// Step 4: Execute authentication flow with credentials, forwarding the challenge token
	flowStep, err := ExecuteAuthenticationFlow(executionId, map[string]string{
		"username": username,
		"password": password,
	}, "action_001", initialStep.ChallengeToken)
	if err != nil {
		return nil, fmt.Errorf("failed to execute authentication flow: %w", err)
	}

	if flowStep.FlowStatus != "COMPLETE" {
		stepJSON, _ := json.Marshal(flowStep)
		return nil, fmt.Errorf("authentication flow not complete: status=%s, failureReason=%s, step=%s",
			flowStep.FlowStatus, flowStep.FailureReason, string(stepJSON))
	}

	if flowStep.Assertion == "" {
		return nil, fmt.Errorf("no assertion returned from authentication flow")
	}

	// Step 5: Complete authorization with assertion
	authzResp, err := CompleteAuthorization(authID, flowStep.Assertion)
	if err != nil {
		return nil, fmt.Errorf("failed to complete authorization: %w", err)
	}

	// Step 6: Extract authorization code
	code, err := ExtractAuthorizationCode(authzResp.RedirectURI)
	if err != nil {
		return nil, fmt.Errorf("failed to extract authorization code: %w", err)
	}

	// Step 7: Exchange code for token with PKCE verifier
	tokenResult, err := RequestTokenWithPKCE(clientID, clientSecret, code, redirectURI, "authorization_code",
		codeVerifier)
	if err != nil {
		return nil, fmt.Errorf("failed to request token: %w", err)
	}

	if tokenResult.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed with status %d: %s", tokenResult.StatusCode,
			string(tokenResult.Body))
	}

	if tokenResult.Token == nil {
		return nil, fmt.Errorf("no token in response")
	}

	// Create and return token state
	return tokenResult.Token, nil
}

// ObtainAccessTokenWithPAR performs the complete OAuth authorization code flow using a Pushed
// Authorization Request (RFC 9126) and returns a TokenResponse with the access token and expiry
// information. clientSecret is optional and can be provided for confidential clients; when set, it
// is used to authenticate both the PAR submission and the token request (via client_secret_post).
func ObtainAccessTokenWithPAR(clientID, redirectURI, scope, username, password string,
	usePKCE bool, optionalParams ...string) (*TokenResponse, error) {
	clientSecret := ""
	if len(optionalParams) > 0 {
		clientSecret = optionalParams[0]
	}

	var codeVerifier string
	parParams := map[string]string{
		"response_type": "code",
		"redirect_uri":  redirectURI,
		"scope":         scope,
		"state":         "par-test-state",
	}

	if usePKCE {
		var err error
		codeVerifier, err = generateCodeVerifier()
		if err != nil {
			return nil, fmt.Errorf("failed to generate code verifier: %w", err)
		}
		parParams["code_challenge"] = generateCodeChallenge(codeVerifier)
		parParams["code_challenge_method"] = "S256"
	}

	// Step 1: Submit PAR request to obtain request_uri.
	parResult, err := SubmitPARRequest(clientID, clientSecret, parParams)
	if err != nil {
		return nil, fmt.Errorf("failed to submit PAR request: %w", err)
	}
	if parResult.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("PAR request failed with status %d: %s",
			parResult.StatusCode, string(parResult.Body))
	}
	if parResult.PAR == nil {
		return nil, fmt.Errorf("no request_uri in PAR response")
	}

	// Step 2: Initiate authorization flow with request_uri.
	resp, err := InitiateAuthorizationFlowWithRequestURI(clientID, parResult.PAR.RequestURI)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate authorization: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther &&
		resp.StatusCode != http.StatusTemporaryRedirect {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("expected redirect response, got status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	location := resp.Header.Get("Location")
	if location == "" {
		return nil, fmt.Errorf("no Location header in authorization response")
	}

	// Step 3: Extract auth ID and flow ID.
	authID, executionId, err := ExtractAuthData(location)
	if err != nil {
		return nil, fmt.Errorf("failed to extract auth ID: %w", err)
	}

	// Step 4: Execute initial authentication flow step (to get to the login prompt).
	initialStep, err := ExecuteAuthenticationFlow(executionId, nil, "")
	if err != nil {
		return nil, fmt.Errorf("failed to execute initial authentication flow: %w", err)
	}

	// Step 5: Execute authentication flow with credentials, forwarding the challenge token.
	flowStep, err := ExecuteAuthenticationFlow(executionId, map[string]string{
		"username": username,
		"password": password,
	}, "action_001", initialStep.ChallengeToken)
	if err != nil {
		return nil, fmt.Errorf("failed to execute authentication flow: %w", err)
	}

	if flowStep.FlowStatus != "COMPLETE" {
		stepJSON, _ := json.Marshal(flowStep)
		return nil, fmt.Errorf("authentication flow not complete: status=%s, failureReason=%s, step=%s",
			flowStep.FlowStatus, flowStep.FailureReason, string(stepJSON))
	}

	if flowStep.Assertion == "" {
		return nil, fmt.Errorf("no assertion returned from authentication flow")
	}

	// Step 6: Complete authorization with assertion.
	authzResp, err := CompleteAuthorization(authID, flowStep.Assertion)
	if err != nil {
		return nil, fmt.Errorf("failed to complete authorization: %w", err)
	}

	// Step 7: Extract authorization code.
	code, err := ExtractAuthorizationCode(authzResp.RedirectURI)
	if err != nil {
		return nil, fmt.Errorf("failed to extract authorization code: %w", err)
	}

	// Step 8: Exchange code for token.
	var tokenResult *TokenHTTPResult
	if usePKCE {
		tokenResult, err = RequestTokenWithPKCE(clientID, clientSecret, code, redirectURI,
			"authorization_code", codeVerifier)
	} else {
		tokenResult, err = RequestToken(clientID, clientSecret, code, redirectURI, "authorization_code")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to request token: %w", err)
	}

	if tokenResult.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed with status %d: %s", tokenResult.StatusCode,
			string(tokenResult.Body))
	}

	if tokenResult.Token == nil {
		return nil, fmt.Errorf("no token in response")
	}

	return tokenResult.Token, nil
}

// RefreshAccessToken uses the refresh token to obtain a new access token
// client credentials are sent via HTTP Basic Auth header.
func RefreshAccessToken(clientID, clientSecret, refreshToken string) (*TokenResponse, error) {
	return refreshAccessToken(clientID, clientSecret, refreshToken, false)
}

// RefreshAccessTokenWithClientCredentialsInBody uses the refresh token to obtain a new access token where
// client credentials are sent in the request body.
func RefreshAccessTokenWithClientCredentialsInBody(clientID, clientSecret, refreshToken string) (
	*TokenResponse, error) {
	return refreshAccessToken(clientID, clientSecret, refreshToken, true)
}

// refreshAccessToken uses the refresh token to obtain a new access token
func refreshAccessToken(clientID, clientSecret, refreshToken string, tokenAuthInBody bool) (
	*TokenResponse, error) {
	tokenURL := TestServerURL + "/oauth2/token"
	tokenData := url.Values{}

	tokenData.Set("grant_type", "refresh_token")
	tokenData.Set("refresh_token", refreshToken)

	if tokenAuthInBody {
		tokenData.Set("client_id", clientID)
		if clientSecret != "" {
			tokenData.Set("client_secret", clientSecret)
		}
	}

	req, err := http.NewRequest("POST", tokenURL, bytes.NewBufferString(tokenData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if !tokenAuthInBody {
		if clientID != "" {
			req.SetBasicAuth(clientID, clientSecret)
		}
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send refresh token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResponse TokenResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token response: %w", err)
	}

	// Preserve the refresh token if not returned in response (common for some OAuth servers)
	if tokenResponse.RefreshToken == "" {
		tokenResponse.RefreshToken = refreshToken
		log.Println("Refresh token not returned in response, preserving existing token")
	}

	return &tokenResponse, nil
}

// PARResponse represents the response from the PAR endpoint.
type PARResponse struct {
	RequestURI string `json:"request_uri"`
	ExpiresIn  int64  `json:"expires_in"`
}

// PARErrorResponse represents an error response from the PAR endpoint.
type PARErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// PARHTTPResult captures raw HTTP response details from the PAR endpoint.
type PARHTTPResult struct {
	StatusCode int
	Body       []byte
	PAR        *PARResponse
	Error      *PARErrorResponse
}

// SubmitPARRequest sends a pushed authorization request to the PAR endpoint.
func SubmitPARRequest(clientID, clientSecret string, params map[string]string) (*PARHTTPResult, error) {
	parURL := TestServerURL + "/oauth2/par"
	formData := url.Values{}
	for k, v := range params {
		formData.Set(k, v)
	}

	formData.Set("client_id", clientID)
	if clientSecret != "" {
		formData.Set("client_secret", clientSecret)
	}

	req, err := http.NewRequest("POST", parURL, bytes.NewBufferString(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create PAR request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send PAR request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read PAR response body: %w", err)
	}

	result := &PARHTTPResult{
		StatusCode: resp.StatusCode,
		Body:       body,
	}

	if resp.StatusCode == http.StatusCreated {
		var parResp PARResponse
		if err := json.Unmarshal(body, &parResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal PAR response: %w", err)
		}
		result.PAR = &parResp
	} else {
		var errResp PARErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			result.Error = &errResp
		}
	}

	return result, nil
}

// SubmitPARRequestWithoutAuth sends a PAR request without client authentication.
func SubmitPARRequestWithoutAuth(params map[string]string) (*PARHTTPResult, error) {
	parURL := TestServerURL + "/oauth2/par"
	formData := url.Values{}
	for k, v := range params {
		formData.Set(k, v)
	}

	req, err := http.NewRequest("POST", parURL, bytes.NewBufferString(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create PAR request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send PAR request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read PAR response body: %w", err)
	}

	result := &PARHTTPResult{
		StatusCode: resp.StatusCode,
		Body:       body,
	}

	var errResp PARErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil {
		result.Error = &errResp
	}

	return result, nil
}

// InitiateAuthorizationFlowWithRequestURI starts the OAuth2 authorization flow using a PAR request_uri.
func InitiateAuthorizationFlowWithRequestURI(clientID, requestURI string) (*http.Response, error) {
	authURL := TestServerURL + "/oauth2/authorize"
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("request_uri", requestURI)

	req, err := http.NewRequest("GET", authURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create authorization request: %w", err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send authorization request: %w", err)
	}

	return resp, nil
}

// GenerateCodeVerifier generates a PKCE code verifier (exported for PAR tests).
func GenerateCodeVerifier() (string, error) {
	return generateCodeVerifier()
}

// GenerateCodeChallenge generates a PKCE code challenge from a verifier (exported for PAR tests).
func GenerateCodeChallenge(verifier string) string {
	return generateCodeChallenge(verifier)
}

// generateCodeVerifier generates a cryptographically secure random code verifier for PKCE (RFC 7636).
func generateCodeVerifier() (string, error) {
	// Generate 32 random bytes (will result in 43 characters when base64url encoded)
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Base64 URL encode without padding
	verifier := base64.RawURLEncoding.EncodeToString(bytes)
	return verifier, nil
}

// generateCodeChallenge generates a code challenge from a code verifier using SHA-256
func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])
	return challenge
}

// DPoPKey is an asymmetric key pair plus the public-key JWK and its SHA-256
// thumbprint, used to mint DPoP proofs in integration tests.
type DPoPKey struct {
	Alg     string
	Private crypto.Signer
	JWK     map[string]any
	JKT     string
}

// GenerateDPoPKey generates a fresh key pair for the supplied DPoP signing
// algorithm. Supported values: ES256, ES384, ES512, PS256, RS256, EdDSA.
func GenerateDPoPKey(alg string) (*DPoPKey, error) {
	switch alg {
	case "ES256":
		return generateECKey(elliptic.P256(), "P-256", "ES256")
	case "ES384":
		return generateECKey(elliptic.P384(), "P-384", "ES384")
	case "ES512":
		return generateECKey(elliptic.P521(), "P-521", "ES512")
	case "PS256":
		return generateRSAKey("PS256")
	case "RS256":
		return generateRSAKey("RS256")
	case "EdDSA":
		return generateEdKey()
	default:
		return nil, fmt.Errorf("unsupported DPoP alg: %s", alg)
	}
}

func generateECKey(curve elliptic.Curve, crv, alg string) (*DPoPKey, error) {
	priv, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, err
	}

	byteSize := (curve.Params().BitSize + 7) / 8
	x := leftPadBytes(priv.PublicKey.X.Bytes(), byteSize)
	y := leftPadBytes(priv.PublicKey.Y.Bytes(), byteSize)

	jwk := map[string]any{
		"kty": "EC",
		"crv": crv,
		"x":   base64.RawURLEncoding.EncodeToString(x),
		"y":   base64.RawURLEncoding.EncodeToString(y),
	}
	jkt, err := computeJKT(jwk)
	if err != nil {
		return nil, err
	}
	return &DPoPKey{Alg: alg, Private: priv, JWK: jwk, JKT: jkt}, nil
}

func generateRSAKey(alg string) (*DPoPKey, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	jwk := map[string]any{
		"kty": "RSA",
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(priv.E)).Bytes()),
		"n":   base64.RawURLEncoding.EncodeToString(priv.N.Bytes()),
	}
	jkt, err := computeJKT(jwk)
	if err != nil {
		return nil, err
	}
	return &DPoPKey{Alg: alg, Private: priv, JWK: jwk, JKT: jkt}, nil
}

func generateEdKey() (*DPoPKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	jwk := map[string]any{
		"kty": "OKP",
		"crv": "Ed25519",
		"x":   base64.RawURLEncoding.EncodeToString(pub),
	}
	jkt, err := computeJKT(jwk)
	if err != nil {
		return nil, err
	}
	return &DPoPKey{Alg: "EdDSA", Private: priv, JWK: jwk, JKT: jkt}, nil
}

// computeJKT computes the SHA-256 thumbprint of a JWK using only the
// required public-key members in lexicographic order.
func computeJKT(jwk map[string]any) (string, error) {
	kty, _ := jwk["kty"].(string)
	var canonical string
	switch kty {
	case "EC":
		crv, _ := jwk["crv"].(string)
		x, _ := jwk["x"].(string)
		y, _ := jwk["y"].(string)
		canonical = fmt.Sprintf(`{"crv":%q,"kty":"EC","x":%q,"y":%q}`, crv, x, y)
	case "RSA":
		e, _ := jwk["e"].(string)
		n, _ := jwk["n"].(string)
		canonical = fmt.Sprintf(`{"e":%q,"kty":"RSA","n":%q}`, e, n)
	case "OKP":
		crv, _ := jwk["crv"].(string)
		x, _ := jwk["x"].(string)
		canonical = fmt.Sprintf(`{"crv":%q,"kty":"OKP","x":%q}`, crv, x)
	default:
		return "", fmt.Errorf("unsupported kty for JKT: %s", kty)
	}
	sum := sha256.Sum256([]byte(canonical))
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// DPoPProofOptions tweaks the proof JWT created by CreateProof. Default-zero
// values mean "use sensible defaults" — fresh jti, current iat, no ath, the
// key's natural alg and JWK, etc.
type DPoPProofOptions struct {
	// HTM/HTU override the supplied request method/URL inside the proof body.
	// Empty values keep the values passed to CreateProof.
	HTM string
	HTU string

	// Iat overrides the "iat" claim. Zero means time.Now().
	Iat int64
	// IatOffset, when Iat is zero, shifts the default iat by the given seconds.
	// Negative values produce a stale proof; positive values produce a proof
	// from the future.
	IatOffset int

	// Jti overrides the random "jti". Empty means a random 22-char value.
	Jti string
	// OmitJTI removes the "jti" claim entirely.
	OmitJTI bool

	// AccessToken, when non-empty, populates the "ath" claim with the
	// base64url(SHA-256(token)) hash. Ignored when AthOverride is set.
	AccessToken string
	// AthOverride, when non-empty, sets the literal "ath" value.
	AthOverride string
	// OmitAth removes the "ath" claim even when AccessToken is set.
	OmitAth bool

	// Typ overrides the "typ" header. Empty means "dpop+jwt".
	Typ string
	// OmitTyp removes the "typ" header.
	OmitTyp bool

	// Alg overrides the JWS "alg" header. Empty means the key's natural alg.
	Alg string

	// JWKOverride replaces the embedded JWK header completely.
	JWKOverride map[string]any
	// IncludePrivateInJWK adds a "d" member to the embedded JWK to simulate
	// a misconfigured client leaking private-key material.
	IncludePrivateInJWK bool
	// OmitJWK removes the JWK header entirely.
	OmitJWK bool

	// TamperSignature flips a bit in the signature so the JWS verification
	// fails while leaving the structure parseable.
	TamperSignature bool

	// Extra header members merged into the JWS protected header.
	ExtraHeader map[string]any
	// Extra payload members merged into the JWT body.
	ExtraPayload map[string]any
}

// CreateProof signs a DPoP proof JWT bound to the given HTTP method and URL.
// All proof inputs (typ, alg, jwk, htm, htu, iat, jti, ath) can be individually
// tampered with via opts to drive negative-path tests.
func (k *DPoPKey) CreateProof(htm, htu string, opts DPoPProofOptions) (string, error) {
	if opts.HTM != "" {
		htm = opts.HTM
	}
	if opts.HTU != "" {
		htu = opts.HTU
	}

	header := map[string]any{
		"typ": "dpop+jwt",
		"alg": k.Alg,
		"jwk": k.JWK,
	}
	if opts.Typ != "" {
		header["typ"] = opts.Typ
	}
	if opts.OmitTyp {
		delete(header, "typ")
	}
	if opts.Alg != "" {
		header["alg"] = opts.Alg
	}
	if opts.JWKOverride != nil {
		header["jwk"] = opts.JWKOverride
	}
	if opts.IncludePrivateInJWK {
		jwk := cloneJWK(k.JWK)
		jwk["d"] = "AAAA"
		header["jwk"] = jwk
	}
	if opts.OmitJWK {
		delete(header, "jwk")
	}
	for k2, v := range opts.ExtraHeader {
		header[k2] = v
	}

	iat := opts.Iat
	if iat == 0 {
		iat = time.Now().Unix() + int64(opts.IatOffset)
	}

	payload := map[string]any{
		"htm": htm,
		"htu": htu,
		"iat": iat,
	}
	if !opts.OmitJTI {
		jti := opts.Jti
		if jti == "" {
			jti = randomJTI()
		}
		payload["jti"] = jti
	}
	if !opts.OmitAth {
		switch {
		case opts.AthOverride != "":
			payload["ath"] = opts.AthOverride
		case opts.AccessToken != "":
			sum := sha256.Sum256([]byte(opts.AccessToken))
			payload["ath"] = base64.RawURLEncoding.EncodeToString(sum[:])
		}
	}
	for k2, v := range opts.ExtraPayload {
		payload[k2] = v
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) +
		"." + base64.RawURLEncoding.EncodeToString(payloadJSON)

	sig, err := signProof(k.Private, k.Alg, signingInput)
	if err != nil {
		return "", err
	}
	if opts.TamperSignature && len(sig) > 0 {
		sig[len(sig)-1] ^= 0x01
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// MakeUnsignedDPoPProof builds a header.payload."" JWS where the signature
// segment is empty. Useful for asserting that the verifier rejects alg=none.
func (k *DPoPKey) MakeUnsignedDPoPProof(htm, htu string) (string, error) {
	header := map[string]any{
		"typ": "dpop+jwt",
		"alg": "none",
		"jwk": k.JWK,
	}
	payload := map[string]any{
		"htm": htm,
		"htu": htu,
		"iat": time.Now().Unix(),
		"jti": randomJTI(),
	}
	hb, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	pb, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(hb) + "." +
		base64.RawURLEncoding.EncodeToString(pb) + ".", nil
}

func signProof(priv crypto.Signer, alg, signingInput string) ([]byte, error) {
	switch alg {
	case "ES256", "ES384", "ES512":
		k, ok := priv.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("alg %s requires *ecdsa.PrivateKey", alg)
		}
		h, err := ecHash(alg)
		if err != nil {
			return nil, err
		}
		h.Write([]byte(signingInput))
		r, s, err := ecdsa.Sign(rand.Reader, k, h.Sum(nil))
		if err != nil {
			return nil, err
		}
		byteSize := (k.Curve.Params().BitSize + 7) / 8
		sig := make([]byte, 2*byteSize)
		copy(sig[byteSize-len(r.Bytes()):byteSize], r.Bytes())
		copy(sig[2*byteSize-len(s.Bytes()):], s.Bytes())
		return sig, nil
	case "PS256":
		k, ok := priv.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("alg PS256 requires *rsa.PrivateKey")
		}
		h := sha256.Sum256([]byte(signingInput))
		return rsa.SignPSS(rand.Reader, k, crypto.SHA256, h[:], &rsa.PSSOptions{
			SaltLength: rsa.PSSSaltLengthEqualsHash,
		})
	case "RS256":
		k, ok := priv.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("alg RS256 requires *rsa.PrivateKey")
		}
		h := sha256.Sum256([]byte(signingInput))
		return rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, h[:])
	case "EdDSA":
		k, ok := priv.(ed25519.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("alg EdDSA requires ed25519.PrivateKey")
		}
		return ed25519.Sign(k, []byte(signingInput)), nil
	default:
		return nil, fmt.Errorf("unsupported alg for signing: %s", alg)
	}
}

func ecHash(alg string) (hash.Hash, error) {
	switch alg {
	case "ES256":
		return sha256.New(), nil
	case "ES384":
		return sha512.New384(), nil
	case "ES512":
		return sha512.New(), nil
	default:
		return nil, fmt.Errorf("unsupported EC alg: %s", alg)
	}
}

func leftPadBytes(b []byte, size int) []byte {
	if len(b) >= size {
		return b
	}
	out := make([]byte, size)
	copy(out[size-len(b):], b)
	return out
}

func cloneJWK(jwk map[string]any) map[string]any {
	out := make(map[string]any, len(jwk))
	for k, v := range jwk {
		out[k] = v
	}
	return out
}

// randomJTI returns a fresh url-safe identifier suitable for the "jti" claim.
func randomJTI() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// AthFor computes the DPoP "ath" hash for a given access token.
func AthFor(accessToken string) string {
	sum := sha256.Sum256([]byte(accessToken))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// DecodeJWTPayloadMap decodes a JWS payload as a generic claims map without
// verifying the signature. Tests use this to inspect cnf.jkt and token_type.
func DecodeJWTPayloadMap(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}
