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

// Package connection provides integration tests for the /connections API: the unified
// vendor-scoped CRUD + flat listing surface that fronts the identity-provider and
// notification-sender services.
package connection

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const testServerURL = testutils.TestServerURL

// connectionListResponse mirrors backend/internal/connection/models.go connectionListResponse.
type connectionListResponse struct {
	TotalResults int                  `json:"totalResults"`
	StartIndex   int                  `json:"startIndex"`
	Count        int                  `json:"count"`
	Connections  []connectionInstance `json:"connections"`
}

type connectionInstance struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type"`
	Categories  []string `json:"categories"`
}

// errorResponse mirrors the standard API error envelope.
type errorResponse struct {
	Code string `json:"code"`
}

// httpResult captures a decoded response body alongside its status code.
type httpResult struct {
	status int
	body   []byte
}

func (r httpResult) errorCode() string {
	var e errorResponse
	_ = json.Unmarshal(r.body, &e)
	return e.Code
}

func (r httpResult) decode(v interface{}) error {
	return json.Unmarshal(r.body, v)
}

func doRequest(method, path string, body interface{}) (httpResult, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return httpResult{}, fmt.Errorf("failed to marshal body: %w", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, testServerURL+path, reader)
	if err != nil {
		return httpResult{}, fmt.Errorf("failed to create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return httpResult{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return httpResult{}, fmt.Errorf("failed to read response body: %w", err)
	}
	return httpResult{status: resp.StatusCode, body: respBody}, nil
}

// --- Vendor request/response shapes (mirror backend/internal/connection/*.go wire format) ---

type googleConnectionRequest struct {
	Name         string   `json:"name"`
	ClientID     string   `json:"clientId"`
	ClientSecret string   `json:"clientSecret,omitempty"`
	RedirectURI  string   `json:"redirectUri"`
	Scopes       []string `json:"scopes,omitempty"`
}

type githubConnectionRequest struct {
	Name         string `json:"name"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret,omitempty"`
	RedirectURI  string `json:"redirectUri"`
}

type oidcConnectionRequest struct {
	Name                  string `json:"name"`
	ClientID              string `json:"clientId"`
	ClientSecret          string `json:"clientSecret,omitempty"`
	RedirectURI           string `json:"redirectUri"`
	AuthorizationEndpoint string `json:"authorizationEndpoint"`
	TokenEndpoint         string `json:"tokenEndpoint"`
}

type oauthConnectionRequest struct {
	Name                  string `json:"name"`
	ClientID              string `json:"clientId"`
	ClientSecret          string `json:"clientSecret,omitempty"`
	RedirectURI           string `json:"redirectUri"`
	AuthorizationEndpoint string `json:"authorizationEndpoint"`
	TokenEndpoint         string `json:"tokenEndpoint"`
	UserInfoEndpoint      string `json:"userInfoEndpoint"`
}

type twilioConnectionRequest struct {
	Name       string `json:"name"`
	AccountSID string `json:"accountSid"`
	AuthToken  string `json:"authToken,omitempty"`
	SenderID   string `json:"senderId"`
}

type vonageConnectionRequest struct {
	Name      string `json:"name"`
	APIKey    string `json:"apiKey"`
	APISecret string `json:"apiSecret,omitempty"`
	SenderID  string `json:"senderId"`
}

type smsGatewayConnectionRequest struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	HTTPMethod string `json:"httpMethod,omitempty"`
}

// connectionResponse is a superset response shape covering all vendors' fields, used to
// decode any vendor's response without a per-vendor struct.
type connectionResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	AccountSID   string `json:"accountSid,omitempty"`
	AuthToken    string `json:"authToken,omitempty"`
	APIKey       string `json:"apiKey,omitempty"`
	APISecret    string `json:"apiSecret,omitempty"`
	SenderID     string `json:"senderId,omitempty"`
	URL          string `json:"url,omitempty"`
}

const maskedSecretValue = "******"

type ConnectionAPITestSuite struct {
	suite.Suite
}

func TestConnectionAPISuite(t *testing.T) {
	suite.Run(t, new(ConnectionAPITestSuite))
}

// createConnection posts to /connections/{vendor} and returns the decoded response.
func (s *ConnectionAPITestSuite) createConnection(vendor string, body interface{}) connectionResponse {
	s.T().Helper()
	res, err := doRequest(http.MethodPost, "/connections/"+vendor, body)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusCreated, res.status, "create %s: %s", vendor, string(res.body))
	var resp connectionResponse
	s.Require().NoError(res.decode(&resp))
	return resp
}

func (s *ConnectionAPITestSuite) deleteConnection(vendor, id string) {
	s.T().Helper()
	res, err := doRequest(http.MethodDelete, "/connections/"+vendor+"/"+id, nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusNoContent, res.status)
}

// --- CRUD happy paths, one per vendor family ---

func (s *ConnectionAPITestSuite) TestGoogleCRUDRoundTrip() {
	created := s.createConnection("google", googleConnectionRequest{
		Name: "Test Google", ClientID: "g-client", ClientSecret: "g-secret",
		RedirectURI: "https://localhost:3000/google/callback", Scopes: []string{"openid", "email"},
	})
	defer s.deleteConnection("google", created.ID)

	s.Equal("google", created.Type)
	s.Equal("g-client", created.ClientID)
	s.Equal(maskedSecretValue, created.ClientSecret, "secret must be masked on create response")

	res, err := doRequest(http.MethodGet, "/connections/google/"+created.ID, nil)
	s.Require().NoError(err)
	s.Equal(http.StatusOK, res.status)
	var fetched connectionResponse
	s.Require().NoError(res.decode(&fetched))
	s.Equal("Test Google", fetched.Name)
	s.Equal(maskedSecretValue, fetched.ClientSecret)

	// Update omitting the secret must keep the stored value (secret-preserving update).
	updateRes, err := doRequest(http.MethodPut, "/connections/google/"+created.ID, googleConnectionRequest{
		Name: "Test Google Renamed", ClientID: "g-client", RedirectURI: "https://localhost:3000/google/callback",
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, updateRes.status, string(updateRes.body))
	var updated connectionResponse
	s.Require().NoError(updateRes.decode(&updated))
	s.Equal("Test Google Renamed", updated.Name)
	s.Equal(maskedSecretValue, updated.ClientSecret)
}

func (s *ConnectionAPITestSuite) TestGitHubCreateAndGet() {
	created := s.createConnection("github", githubConnectionRequest{
		Name: "Test GitHub", ClientID: "gh-client", ClientSecret: "gh-secret",
		RedirectURI: "https://localhost:3000/github/callback",
	})
	defer s.deleteConnection("github", created.ID)

	s.Equal("github", created.Type)
	s.Equal(maskedSecretValue, created.ClientSecret)
}

func (s *ConnectionAPITestSuite) TestOIDCCreateAndGet() {
	created := s.createConnection("oidc", oidcConnectionRequest{
		Name: "Test OIDC", ClientID: "oidc-client", ClientSecret: "oidc-secret",
		RedirectURI:           "https://localhost:3000/oidc/callback",
		AuthorizationEndpoint: "https://issuer.example.com/authorize",
		TokenEndpoint:         "https://issuer.example.com/token",
	})
	defer s.deleteConnection("oidc", created.ID)

	s.Equal("oidc", created.Type)
}

func (s *ConnectionAPITestSuite) TestOAuthCreateAndGet() {
	created := s.createConnection("oauth", oauthConnectionRequest{
		Name: "Test OAuth", ClientID: "oauth-client", ClientSecret: "oauth-secret",
		RedirectURI:           "https://localhost:3000/oauth/callback",
		AuthorizationEndpoint: "https://issuer.example.com/authorize",
		TokenEndpoint:         "https://issuer.example.com/token",
		UserInfoEndpoint:      "https://issuer.example.com/userinfo",
	})
	defer s.deleteConnection("oauth", created.ID)

	s.Equal("oauth", created.Type)
}

func (s *ConnectionAPITestSuite) TestTwilioCRUDRoundTripWithSecretMasking() {
	created := s.createConnection("twilio", twilioConnectionRequest{
		Name: "Test Twilio", AccountSID: "AC00000000000000000000000000000000",
		AuthToken: "tw-token", SenderID: "+15005550006",
	})
	defer s.deleteConnection("twilio", created.ID)

	s.Equal("twilio", created.Type)
	s.Equal(maskedSecretValue, created.AuthToken)

	// Omitting authToken on update must preserve the stored value.
	updateRes, err := doRequest(http.MethodPut, "/connections/twilio/"+created.ID, twilioConnectionRequest{
		Name: "Test Twilio Renamed", AccountSID: "AC00000000000000000000000000000000", SenderID: "+15005550006",
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, updateRes.status, string(updateRes.body))
	var updated connectionResponse
	s.Require().NoError(updateRes.decode(&updated))
	s.Equal("Test Twilio Renamed", updated.Name)
	s.Equal(maskedSecretValue, updated.AuthToken)
}

func (s *ConnectionAPITestSuite) TestVonageCreateAndGet() {
	created := s.createConnection("vonage", vonageConnectionRequest{
		Name: "Test Vonage", APIKey: "vo-key", APISecret: "vo-secret", SenderID: "ThunderID",
	})
	defer s.deleteConnection("vonage", created.ID)

	s.Equal("vonage", created.Type)
	s.Equal(maskedSecretValue, created.APISecret)
}

func (s *ConnectionAPITestSuite) TestSMSGatewayCRUDRoundTrip() {
	created := s.createConnection("sms-gateway", smsGatewayConnectionRequest{
		Name: "Test SMS Gateway", URL: "https://sms.example.com/send", HTTPMethod: "POST",
	})
	defer s.deleteConnection("sms-gateway", created.ID)

	s.Equal("sms-gateway", created.Type)
	// SMS gateway fields are non-secret and round-trip in plaintext.
	s.Equal("https://sms.example.com/send", created.URL)

	res, err := doRequest(http.MethodGet, "/connections/sms-gateway/"+created.ID, nil)
	s.Require().NoError(err)
	s.Equal(http.StatusOK, res.status)
	var fetched connectionResponse
	s.Require().NoError(res.decode(&fetched))
	s.Equal("https://sms.example.com/send", fetched.URL)
}

// --- Cross-cutting behaviors ---

func (s *ConnectionAPITestSuite) TestCrossVendorIsolationReturnsNotFound() {
	created := s.createConnection("google", googleConnectionRequest{
		Name: "Isolation Test", ClientID: "iso-client", ClientSecret: "iso-secret",
		RedirectURI: "https://localhost:3000/google/callback",
	})
	defer s.deleteConnection("google", created.ID)

	// The instance exists as a google connection; fetching it via /connections/github must 404.
	res, err := doRequest(http.MethodGet, "/connections/github/"+created.ID, nil)
	s.Require().NoError(err)
	s.Equal(http.StatusNotFound, res.status)
}

func (s *ConnectionAPITestSuite) TestDuplicateNameReturnsConflict() {
	created := s.createConnection("google", googleConnectionRequest{
		Name: "Duplicate Name Test", ClientID: "dup-client", ClientSecret: "dup-secret",
		RedirectURI: "https://localhost:3000/google/callback",
	})
	defer s.deleteConnection("google", created.ID)

	res, err := doRequest(http.MethodPost, "/connections/google", googleConnectionRequest{
		Name: "Duplicate Name Test", ClientID: "dup-client-2", ClientSecret: "dup-secret-2",
		RedirectURI: "https://localhost:3000/google/callback",
	})
	s.Require().NoError(err)
	s.Equal(http.StatusConflict, res.status)
	s.Equal("IDP-1005", res.errorCode())
}

func (s *ConnectionAPITestSuite) TestDuplicateSenderNameReturnsConflict() {
	created := s.createConnection("twilio", twilioConnectionRequest{
		Name: "Duplicate Sender Test", AccountSID: "AC00000000000000000000000000000000",
		AuthToken: "tok", SenderID: "+15005550006",
	})
	defer s.deleteConnection("twilio", created.ID)

	res, err := doRequest(http.MethodPost, "/connections/twilio", twilioConnectionRequest{
		Name: "Duplicate Sender Test", AccountSID: "AC00000000000000000000000000000001",
		AuthToken: "tok2", SenderID: "+15005550007",
	})
	s.Require().NoError(err)
	s.Equal(http.StatusConflict, res.status)
	s.Equal("MNS-1005", res.errorCode())
}

func (s *ConnectionAPITestSuite) TestInvalidBodyReturnsBadRequest() {
	// Missing required clientId/redirectUri.
	res, err := doRequest(http.MethodPost, "/connections/google", map[string]string{"name": "Incomplete"})
	s.Require().NoError(err)
	s.Equal(http.StatusBadRequest, res.status)
}

func (s *ConnectionAPITestSuite) TestGetNonExistentReturnsNotFound() {
	res, err := doRequest(http.MethodGet, "/connections/google/does-not-exist", nil)
	s.Require().NoError(err)
	s.Equal(http.StatusNotFound, res.status)
}

func (s *ConnectionAPITestSuite) TestUsagesOnIdPInstance() {
	created := s.createConnection("google", googleConnectionRequest{
		Name: "Usages Test", ClientID: "usages-client", ClientSecret: "usages-secret",
		RedirectURI: "https://localhost:3000/google/callback",
	})
	defer s.deleteConnection("google", created.ID)

	res, err := doRequest(http.MethodGet, "/connections/google/"+created.ID+"/usages", nil)
	s.Require().NoError(err)
	s.Equal(http.StatusOK, res.status, string(res.body))
}

// --- Listing: pagination, category filtering, and negatives ---

func (s *ConnectionAPITestSuite) TestListConnectionsFiltersByCategory() {
	idp := s.createConnection("google", googleConnectionRequest{
		Name: "List Category IdP", ClientID: "list-idp-client", ClientSecret: "list-idp-secret",
		RedirectURI: "https://localhost:3000/google/callback",
	})
	defer s.deleteConnection("google", idp.ID)
	sender := s.createConnection("twilio", twilioConnectionRequest{
		Name: "List Category Sender", AccountSID: "AC00000000000000000000000000000002",
		AuthToken: "tok", SenderID: "+15005550008",
	})
	defer s.deleteConnection("twilio", sender.ID)

	res, err := doRequest(http.MethodGet, "/connections?category=identity-provider&limit=100", nil)
	s.Require().NoError(err)
	s.Equal(http.StatusOK, res.status)
	var list connectionListResponse
	s.Require().NoError(res.decode(&list))
	s.True(containsID(list.Connections, idp.ID))
	s.False(containsID(list.Connections, sender.ID))

	res, err = doRequest(http.MethodGet, "/connections?category=sms-provider&limit=100", nil)
	s.Require().NoError(err)
	s.Equal(http.StatusOK, res.status)
	s.Require().NoError(res.decode(&list))
	s.True(containsID(list.Connections, sender.ID))
	s.False(containsID(list.Connections, idp.ID))
}

func (s *ConnectionAPITestSuite) TestListConnectionsInvalidCategoryReturnsBadRequest() {
	res, err := doRequest(http.MethodGet, "/connections?category=bogus", nil)
	s.Require().NoError(err)
	s.Equal(http.StatusBadRequest, res.status)
	s.Equal("CON-1001", res.errorCode())
}

func (s *ConnectionAPITestSuite) TestListConnectionsInvalidLimitReturnsBadRequest() {
	for _, limit := range []string{"0", "-1", "abc", "101"} {
		res, err := doRequest(http.MethodGet, "/connections?limit="+limit, nil)
		s.Require().NoError(err)
		s.Equal(http.StatusBadRequest, res.status, "limit=%s", limit)
		s.Equal("CON-1002", res.errorCode(), "limit=%s", limit)
	}
}

func (s *ConnectionAPITestSuite) TestListConnectionsInvalidOffsetReturnsBadRequest() {
	res, err := doRequest(http.MethodGet, "/connections?offset=-1", nil)
	s.Require().NoError(err)
	s.Equal(http.StatusBadRequest, res.status)
	s.Equal("CON-1003", res.errorCode())
}

func (s *ConnectionAPITestSuite) TestListConnectionsPaginates() {
	var ids []string
	for i := 0; i < 3; i++ {
		created := s.createConnection("google", googleConnectionRequest{
			Name: fmt.Sprintf("Pagination Test %d", i), ClientID: fmt.Sprintf("page-client-%d", i),
			ClientSecret: "page-secret", RedirectURI: "https://localhost:3000/google/callback",
		})
		ids = append(ids, created.ID)
	}
	defer func() {
		for _, id := range ids {
			s.deleteConnection("google", id)
		}
	}()

	res, err := doRequest(http.MethodGet, "/connections?category=identity-provider&limit=1&offset=0", nil)
	s.Require().NoError(err)
	s.Equal(http.StatusOK, res.status)
	var list connectionListResponse
	s.Require().NoError(res.decode(&list))
	s.Equal(1, list.Count)
	s.GreaterOrEqual(list.TotalResults, 3)
}

func containsID(instances []connectionInstance, id string) bool {
	for _, i := range instances {
		if i.ID == id {
			return true
		}
	}
	return false
}
