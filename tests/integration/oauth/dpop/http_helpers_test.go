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

package dpop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// tokenResponse is the success body of /oauth2/token. We re-declare it locally
// so tests inspect token_type via JSON without depending on internal types.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
}

// errorResponse is the standard OAuth2 error body.
type errorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// tokenHTTPResult wraps the raw /oauth2/token HTTP response.
type tokenHTTPResult struct {
	StatusCode int
	Body       []byte
	Token      *tokenResponse
	Error      *errorResponse
}

// requestTokenWithDPoP exchanges an authorization code at /oauth2/token,
// optionally attaching a DPoP header. It uses HTTP Basic Auth for the client
// credentials, mirroring the suite's app config (client_secret_basic).
//
// Pass dpopHeaders to send multiple DPoP headers (negative test).
// dpopProof is the convenience single-header variant; ignored when
// dpopHeaders is non-nil.
func requestTokenWithDPoP(
	clientID, clientSecret, code, redirectURI, codeVerifier, dpopProof string,
	dpopHeaders []string,
) (*tokenHTTPResult, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	if codeVerifier != "" {
		form.Set("code_verifier", codeVerifier)
	}

	req, err := http.NewRequest("POST", tokenEndpoint, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	if dpopHeaders != nil {
		for _, h := range dpopHeaders {
			req.Header.Add("DPoP", h)
		}
	} else if dpopProof != "" {
		req.Header.Set("DPoP", dpopProof)
	}

	client := testutils.GetNoRedirectHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	out := &tokenHTTPResult{StatusCode: resp.StatusCode, Body: body}
	if resp.StatusCode == http.StatusOK {
		var tr tokenResponse
		if err := json.Unmarshal(body, &tr); err == nil {
			out.Token = &tr
		}
	} else {
		var er errorResponse
		if err := json.Unmarshal(body, &er); err == nil {
			out.Error = &er
		}
	}
	return out, nil
}

// submitPARWithDPoP posts a PAR request with optional DPoP header(s).
// Pass dpopHeaders to send multiple headers; otherwise dpopProof is used as
// a single header value.
func submitPARWithDPoP(
	clientID, clientSecret, dpopProof string, params map[string]string,
) (*testutils.PARHTTPResult, error) {
	return submitPAR(clientID, clientSecret, dpopProof, nil, params)
}

func submitPAR(
	clientID, clientSecret, singleProof string, headers []string,
	params map[string]string,
) (*testutils.PARHTTPResult, error) {
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}

	req, err := http.NewRequest("POST", parEndpoint, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)
	if headers != nil {
		for _, h := range headers {
			req.Header.Add("DPoP", h)
		}
	} else if singleProof != "" {
		req.Header.Set("DPoP", singleProof)
	}

	client := testutils.GetNoRedirectHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	out := &testutils.PARHTTPResult{StatusCode: resp.StatusCode, Body: body}
	if resp.StatusCode == http.StatusCreated {
		var p testutils.PARResponse
		if err := json.Unmarshal(body, &p); err == nil {
			out.PAR = &p
		}
	} else {
		var er testutils.PARErrorResponse
		if err := json.Unmarshal(body, &er); err == nil {
			out.Error = &er
		}
	}
	return out, nil
}

// userInfoHTTPResult captures the raw response from /oauth2/userinfo.
type userInfoHTTPResult struct {
	StatusCode      int
	Body            []byte
	WWWAuthenticate string
	JSON            map[string]any
}

// callUserInfoBearer calls /oauth2/userinfo under the Bearer scheme.
func callUserInfoBearer(accessToken string) (*userInfoHTTPResult, error) {
	return callUserInfo("Bearer "+accessToken, nil)
}

// callUserInfoDPoP calls /oauth2/userinfo under the DPoP scheme.
// proofHeaders may carry one or many DPoP headers (multiple is a protocol violation).
func callUserInfoDPoP(accessToken string, proofHeaders []string) (*userInfoHTTPResult, error) {
	return callUserInfo("DPoP "+accessToken, proofHeaders)
}

func callUserInfo(authzHeader string, proofHeaders []string) (*userInfoHTTPResult, error) {
	req, err := http.NewRequest("GET", userInfoEndpoint, nil)
	if err != nil {
		return nil, err
	}
	if authzHeader != "" {
		req.Header.Set("Authorization", authzHeader)
	}
	for _, h := range proofHeaders {
		req.Header.Add("DPoP", h)
	}

	client := testutils.GetNoRedirectHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	out := &userInfoHTTPResult{
		StatusCode:      resp.StatusCode,
		Body:            body,
		WWWAuthenticate: resp.Header.Get("WWW-Authenticate"),
	}
	if len(body) > 0 {
		_ = json.Unmarshal(body, &out.JSON)
	}
	return out, nil
}

// introspectionResult is the parsed body of /oauth2/introspect.
type introspectionResult struct {
	StatusCode int
	Active     bool
	TokenType  string
	Cnf        map[string]any
	Scope      string
	ClientID   string
	Sub        string
	Raw        map[string]any
}

// introspectToken calls /oauth2/introspect using Basic auth with the supplied
// client credentials. The endpoint is behind ClientAuthMiddleware.
func introspectToken(clientID, clientSecret, token string) (*introspectionResult, error) {
	form := url.Values{}
	form.Set("token", token)

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/introspect",
		bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	client := testutils.GetNoRedirectHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("introspection returned %d: %s", resp.StatusCode, string(body))
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal introspection body: %w", err)
	}
	out := &introspectionResult{StatusCode: resp.StatusCode, Raw: raw}
	if v, ok := raw["active"].(bool); ok {
		out.Active = v
	}
	if v, ok := raw["token_type"].(string); ok {
		out.TokenType = v
	}
	if v, ok := raw["cnf"].(map[string]any); ok {
		out.Cnf = v
	}
	if v, ok := raw["scope"].(string); ok {
		out.Scope = v
	}
	if v, ok := raw["client_id"].(string); ok {
		out.ClientID = v
	}
	if v, ok := raw["sub"].(string); ok {
		out.Sub = v
	}
	return out, nil
}
