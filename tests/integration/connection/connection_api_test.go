// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	Name                   string                            `json:"name"`
	ClientID               string                            `json:"clientId"`
	ClientSecret           string                            `json:"clientSecret,omitempty"`
	RedirectURI            string                            `json:"redirectUri"`
	Scopes                 []string                          `json:"scopes,omitempty"`
	AttributeConfiguration *testutils.AttributeConfiguration `json:"attributeConfiguration,omitempty"`
}

type githubConnectionRequest struct {
	Name                   string                            `json:"name"`
	ClientID               string                            `json:"clientId"`
	ClientSecret           string                            `json:"clientSecret,omitempty"`
	RedirectURI            string                            `json:"redirectUri"`
	AttributeConfiguration *testutils.AttributeConfiguration `json:"attributeConfiguration,omitempty"`
}

type oidcConnectionRequest struct {
	Name                   string                            `json:"name"`
	ClientID               string                            `json:"clientId"`
	ClientSecret           string                            `json:"clientSecret,omitempty"`
	RedirectURI            string                            `json:"redirectUri"`
	AuthorizationEndpoint  string                            `json:"authorizationEndpoint"`
	TokenEndpoint          string                            `json:"tokenEndpoint"`
	Scopes                 []string                          `json:"scopes,omitempty"`
	AttributeConfiguration *testutils.AttributeConfiguration `json:"attributeConfiguration,omitempty"`
}

type oauthConnectionRequest struct {
	Name                   string                            `json:"name"`
	ClientID               string                            `json:"clientId"`
	ClientSecret           string                            `json:"clientSecret,omitempty"`
	RedirectURI            string                            `json:"redirectUri"`
	AuthorizationEndpoint  string                            `json:"authorizationEndpoint"`
	TokenEndpoint          string                            `json:"tokenEndpoint"`
	UserInfoEndpoint       string                            `json:"userInfoEndpoint"`
	Scopes                 []string                          `json:"scopes,omitempty"`
	AttributeConfiguration *testutils.AttributeConfiguration `json:"attributeConfiguration,omitempty"`
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

	Scopes                 []string                          `json:"scopes,omitempty"`
	AttributeConfiguration *testutils.AttributeConfiguration `json:"attributeConfiguration,omitempty"`
}

const maskedSecretValue = "******"

type ConnectionAPITestSuite struct {
	suite.Suite
	ouID                  string
	userTypeID            string
	userTypeName          string
	authzResourceServerID string
	authzRoleID           string
	authzGroupID          string
}

func TestConnectionAPISuite(t *testing.T) {
	suite.Run(t, new(ConnectionAPITestSuite))
}

// attributeConfigOU and attributeConfigUserType back the attributeConfiguration cases, which need a
// real user type to validate mapping targets against. The type declares email required and unique so
// that its presence does not suppress the deployment-wide email account-linking default: seeding reads
// every user type and skips linking if any one of them allows duplicate emails.
var attributeConfigOU = testutils.OrganizationUnit{
	Handle:      "connection-attr-config-ou",
	Name:        "Connection Attribute Config OU",
	Description: "Organization unit for connection attributeConfiguration tests",
	Parent:      nil,
}

var attributeConfigUserType = testutils.UserType{
	Name: "connection_attr_person",
	Schema: map[string]interface{}{
		"username":  map[string]interface{}{"type": "string", "required": true, "unique": true},
		"email":     map[string]interface{}{"type": "string", "required": true, "unique": true},
		"firstName": map[string]interface{}{"type": "string"},
		"lastName":  map[string]interface{}{"type": "string"},
		"password":  map[string]interface{}{"type": "string", "credential": true},
	},
}

func (s *ConnectionAPITestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(attributeConfigOU)
	s.Require().NoError(err, "failed to create organization unit")
	s.ouID = ouID

	userType := attributeConfigUserType
	userType.OUID = ouID
	userTypeID, err := testutils.CreateUserType(userType)
	s.Require().NoError(err, "failed to create user type")
	s.userTypeID = userTypeID
	s.userTypeName = userType.Name

	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:       "Connection Attribute Config API",
		Identifier: "connection-attr-config-api",
		OUID:       ouID,
	}, []testutils.Action{{Name: "Read", Handle: "read", Description: "Read access"}})
	s.Require().NoError(err, "failed to create resource server")
	s.authzResourceServerID = rsID

	roleID, err := testutils.CreateRole(testutils.Role{Name: "Connection Attr Config Role", OUID: ouID})
	s.Require().NoError(err, "failed to create role")
	s.authzRoleID = roleID

	groupID, err := testutils.CreateGroup(testutils.Group{Name: "Connection Attr Config Group", OUID: ouID})
	s.Require().NoError(err, "failed to create group")
	s.authzGroupID = groupID
}

func (s *ConnectionAPITestSuite) TearDownSuite() {
	if s.authzGroupID != "" {
		if err := testutils.DeleteGroup(s.authzGroupID); err != nil {
			s.T().Logf("failed to delete group: %v", err)
		}
	}
	if s.authzRoleID != "" {
		if err := testutils.DeleteRole(s.authzRoleID); err != nil {
			s.T().Logf("failed to delete role: %v", err)
		}
	}
	if s.authzResourceServerID != "" {
		if err := testutils.DeleteResourceServer(s.authzResourceServerID); err != nil {
			s.T().Logf("failed to delete resource server: %v", err)
		}
	}
	if s.userTypeID != "" {
		if err := testutils.DeleteUserType(s.userTypeID); err != nil {
			s.T().Logf("failed to delete user type: %v", err)
		}
	}
	if s.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(s.ouID); err != nil {
			s.T().Logf("failed to delete organization unit: %v", err)
		}
	}
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

func (s *ConnectionAPITestSuite) TestUsagesOnSMSInstance() {
	created := s.createConnection("sms-gateway", smsGatewayConnectionRequest{
		Name: "Usages SMS Gateway", URL: "https://sms.example.com/usages", HTTPMethod: "POST",
	})
	defer s.deleteConnection("sms-gateway", created.ID)

	res, err := doRequest(http.MethodGet, "/connections/sms-gateway/"+created.ID+"/usages", nil)
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

// --- attributeConfiguration: round-trip, validation, seeding (A1-A19) ---

// doRawRequest sends an unmarshalled body so malformed-JSON cases can be exercised. doRequest marshals
// its argument, which cannot produce an invalid payload.
func doRawRequest(method, path, raw string) (httpResult, error) {
	req, err := http.NewRequest(method, testServerURL+path, bytes.NewReader([]byte(raw)))
	if err != nil {
		return httpResult{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return httpResult{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return httpResult{}, fmt.Errorf("failed to read response body: %w", err)
	}
	return httpResult{status: resp.StatusCode, body: body}, nil
}

// validAttributeConfig returns a configuration whose every target exists on the suite's user type.
func (s *ConnectionAPITestSuite) validAttributeConfig() *testutils.AttributeConfiguration {
	return &testutils.AttributeConfiguration{
		UserTypeResolution: &testutils.UserTypeResolution{Default: s.userTypeName},
		UserTypeAttributeMappings: []testutils.UserTypeAttributeMapping{{
			UserType: s.userTypeName,
			Attributes: []testutils.AttributeMapping{
				{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
				{ExternalAttribute: "family_name", LocalAttribute: "lastName"},
			},
		}},
		AccountLinking: &testutils.AccountLinking{Attributes: []string{"email"}},
	}
}

// oauthRequestWithConfig builds a complete generic-OAuth create body carrying the given configuration.
func (s *ConnectionAPITestSuite) oauthRequestWithConfig(
	name string, config *testutils.AttributeConfiguration) oauthConnectionRequest {
	return oauthConnectionRequest{
		Name:                   name,
		ClientID:               "attr-client",
		ClientSecret:           "attr-secret",
		RedirectURI:            "https://localhost:8095/callback",
		AuthorizationEndpoint:  "https://idp.example.com/authorize",
		TokenEndpoint:          "https://idp.example.com/token",
		UserInfoEndpoint:       "https://idp.example.com/userinfo",
		AttributeConfiguration: config,
	}
}

// invalidAttributeConfig describes one rejected configuration, named for the validation class it trips.
type invalidAttributeConfig struct {
	name   string
	config *testutils.AttributeConfiguration
}

// invalidAttributeConfigs enumerates one configuration per validation class the IdP service enforces.
// Shared by the create cases (A3-A9, A20-A22) and the update table (A16): create and update reach validation by
// separate paths, so a class proven on one says nothing about the other.
func (s *ConnectionAPITestSuite) invalidAttributeConfigs() []invalidAttributeConfig {
	mapping := func(attrs ...testutils.AttributeMapping) []testutils.UserTypeAttributeMapping {
		return []testutils.UserTypeAttributeMapping{{UserType: s.userTypeName, Attributes: attrs}}
	}

	return []invalidAttributeConfig{
		{
			name: "A3_duplicate_local_target",
			config: &testutils.AttributeConfiguration{
				UserTypeResolution: &testutils.UserTypeResolution{Default: s.userTypeName},
				UserTypeAttributeMappings: mapping(
					testutils.AttributeMapping{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
					testutils.AttributeMapping{ExternalAttribute: "nickname", LocalAttribute: "firstName"},
				),
			},
		},
		{
			name: "A4_unknown_user_type",
			config: &testutils.AttributeConfiguration{
				UserTypeResolution: &testutils.UserTypeResolution{Default: "no_such_user_type"},
				UserTypeAttributeMappings: []testutils.UserTypeAttributeMapping{{
					UserType: "no_such_user_type",
					Attributes: []testutils.AttributeMapping{
						{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
					},
				}},
			},
		},
		{
			name: "A5_target_not_in_schema",
			config: &testutils.AttributeConfiguration{
				UserTypeResolution: &testutils.UserTypeResolution{Default: s.userTypeName},
				UserTypeAttributeMappings: mapping(
					testutils.AttributeMapping{ExternalAttribute: "nickname", LocalAttribute: "notAnAttribute"},
				),
			},
		},
		{
			name: "A6_mappings_without_resolution_default",
			config: &testutils.AttributeConfiguration{
				UserTypeAttributeMappings: mapping(
					testutils.AttributeMapping{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
				),
			},
		},
		{
			name: "A7_empty_external_attribute",
			config: &testutils.AttributeConfiguration{
				UserTypeResolution: &testutils.UserTypeResolution{Default: s.userTypeName},
				UserTypeAttributeMappings: mapping(
					testutils.AttributeMapping{ExternalAttribute: "", LocalAttribute: "firstName"},
				),
			},
		},
		{
			name: "A8_duplicate_user_type_entry",
			config: &testutils.AttributeConfiguration{
				UserTypeResolution: &testutils.UserTypeResolution{Default: s.userTypeName},
				UserTypeAttributeMappings: []testutils.UserTypeAttributeMapping{
					{UserType: s.userTypeName, Attributes: []testutils.AttributeMapping{
						{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
					}},
					{UserType: s.userTypeName, Attributes: []testutils.AttributeMapping{
						{ExternalAttribute: "family_name", LocalAttribute: "lastName"},
					}},
				},
			},
		},
		{
			name: "A9_dynamic_resolution_invalid_target",
			config: &testutils.AttributeConfiguration{
				UserTypeResolution: &testutils.UserTypeResolution{
					Default:           s.userTypeName,
					ExternalAttribute: "user_type",
					ValueMapping:      map[string]string{"staff": "no_such_user_type"},
				},
			},
		},
		{
			name: "A20_authorization_mapping_role_not_found",
			config: &testutils.AttributeConfiguration{
				UserTypeResolution: &testutils.UserTypeResolution{Default: s.userTypeName},
				AuthorizationMappings: []testutils.AuthorizationMapping{{
					Claim: "groups",
					Values: []testutils.AuthorizationRule{{
						Operator: testutils.AuthorizationOperatorEquals,
						Value:    "admins",
						Targets: []testutils.AuthorizationTarget{
							{Type: testutils.AuthorizationTargetRole, ID: "00000000-0000-0000-0000-000000000001"},
						},
					}},
				}},
			},
		},
		{
			name: "A21_authorization_mapping_group_not_found",
			config: &testutils.AttributeConfiguration{
				UserTypeResolution: &testutils.UserTypeResolution{Default: s.userTypeName},
				AuthorizationMappings: []testutils.AuthorizationMapping{{
					Claim: "groups",
					Values: []testutils.AuthorizationRule{{
						Operator: testutils.AuthorizationOperatorEquals,
						Value:    "editors",
						Targets: []testutils.AuthorizationTarget{
							{Type: testutils.AuthorizationTargetGroup, ID: "00000000-0000-0000-0000-000000000002"},
						},
					}},
				}},
			},
		},
		{
			name: "A22_authorization_mapping_permission_not_found",
			config: &testutils.AttributeConfiguration{
				UserTypeResolution: &testutils.UserTypeResolution{Default: s.userTypeName},
				AuthorizationMappings: []testutils.AuthorizationMapping{{
					Claim: "groups",
					Values: []testutils.AuthorizationRule{{
						Operator: testutils.AuthorizationOperatorEquals,
						Value:    "writers",
						Targets: []testutils.AuthorizationTarget{{
							Type:             testutils.AuthorizationTargetPermission,
							ResourceServerID: "00000000-0000-0000-0000-000000000003",
							Permission:       "write",
						}},
					}},
				}},
			},
		},
		{
			name: "A24_authorization_mapping_includes_requires_multi_valued",
			config: &testutils.AttributeConfiguration{
				UserTypeResolution: &testutils.UserTypeResolution{Default: s.userTypeName},
				AuthorizationMappings: []testutils.AuthorizationMapping{{
					Claim: "department",
					Values: []testutils.AuthorizationRule{{
						Operator: testutils.AuthorizationOperatorIncludes,
						Value:    "platform",
						Targets:  []testutils.AuthorizationTarget{{Type: testutils.AuthorizationTargetRole, ID: s.authzRoleID}},
					}},
				}},
			},
		},
	}
}

// assertEmailLinkingSeeded asserts the seeded email account-linking default.
//
// Seeding only applies it when email is unique on *every* user type in the deployment, so the assertion
// first states that precondition. A type left behind by another suite, or a fixture that allows duplicate
// emails, silently disables the default for every connection — see G15 — and naming the offending type
// is far more useful than a bare "expected non-nil" failure. These cases remain order-sensitive; this
// only makes the failure legible.
func (s *ConnectionAPITestSuite) assertEmailLinkingSeeded(config *testutils.AttributeConfiguration) {
	s.T().Helper()
	s.Require().Empty(s.userTypeAllowingDuplicateEmails(),
		"seeding precondition: a user type that allows duplicate emails disables the email account-linking "+
			"default for every connection in the deployment")
	s.Require().NotNil(config.AccountLinking, "expected seeded email account linking")
	s.Equal([]string{"email"}, config.AccountLinking.Attributes)
}

// userTypeAllowingDuplicateEmails returns the name of the first user type that does not declare email
// unique, or "" when every one does.
func (s *ConnectionAPITestSuite) userTypeAllowingDuplicateEmails() string {
	s.T().Helper()
	userTypes, err := testutils.ListUserTypes()
	s.Require().NoError(err, "failed to list user types")
	s.Require().NotEmpty(userTypes, "expected at least the bootstrapped user type")
	for _, userType := range userTypes {
		if !userType.IsAttributeUnique("email") {
			return userType.Name
		}
	}
	return ""
}

// findMapping returns the mapping entry for the named user type, or nil.
func findMapping(
	config *testutils.AttributeConfiguration, userType string) *testutils.UserTypeAttributeMapping {
	if config == nil {
		return nil
	}
	for i := range config.UserTypeAttributeMappings {
		if config.UserTypeAttributeMappings[i].UserType == userType {
			return &config.UserTypeAttributeMappings[i]
		}
	}
	return nil
}

// A1: a full configuration survives create, GET, a mapping change via PUT, and a second GET.
func (s *ConnectionAPITestSuite) TestOAuthAttributeConfigurationRoundTrip() {
	created := s.createConnection("oauth",
		s.oauthRequestWithConfig("OAuth Attr Round Trip", s.validAttributeConfig()))
	defer s.deleteConnection("oauth", created.ID)

	s.Require().NotNil(created.AttributeConfiguration, "create response should echo the configuration")
	s.Equal([]string{"email"}, created.AttributeConfiguration.AccountLinking.Attributes)
	s.Equal(s.userTypeName, created.AttributeConfiguration.UserTypeResolution.Default)
	s.Require().Len(created.AttributeConfiguration.UserTypeAttributeMappings, 1)
	s.Len(created.AttributeConfiguration.UserTypeAttributeMappings[0].Attributes, 2)

	res, err := doRequest(http.MethodGet, "/connections/oauth/"+created.ID, nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, res.status, string(res.body))
	var fetched connectionResponse
	s.Require().NoError(res.decode(&fetched))
	s.Require().NotNil(fetched.AttributeConfiguration, "GET should return the stored configuration")
	s.Equal(created.AttributeConfiguration, fetched.AttributeConfiguration)

	updated := s.validAttributeConfig()
	updated.UserTypeAttributeMappings[0].Attributes = []testutils.AttributeMapping{
		{ExternalAttribute: "nickname", LocalAttribute: "firstName"},
	}
	updateRes, err := doRequest(http.MethodPut, "/connections/oauth/"+created.ID,
		s.oauthRequestWithConfig("OAuth Attr Round Trip", updated))
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, updateRes.status, string(updateRes.body))

	res, err = doRequest(http.MethodGet, "/connections/oauth/"+created.ID, nil)
	s.Require().NoError(err)
	var afterUpdate connectionResponse
	s.Require().NoError(res.decode(&afterUpdate))
	s.Require().NotNil(afterUpdate.AttributeConfiguration)
	// The targeted assertion names the change; the whole-configuration comparison is what proves nothing
	// else moved — resolution and linking survived untouched and no extra mapping entry appeared.
	s.Require().Len(afterUpdate.AttributeConfiguration.UserTypeAttributeMappings, 1)
	s.Equal([]testutils.AttributeMapping{{ExternalAttribute: "nickname", LocalAttribute: "firstName"}},
		afterUpdate.AttributeConfiguration.UserTypeAttributeMappings[0].Attributes)
	s.Equal(updated, afterUpdate.AttributeConfiguration)
}

// A2: the same round-trip on OIDC, proving the behavior is not specific to the generic OAuth vendor.
func (s *ConnectionAPITestSuite) TestOIDCAttributeConfigurationRoundTrip() {
	created := s.createConnection("oidc", oidcConnectionRequest{
		Name:                   "OIDC Attr Round Trip",
		ClientID:               "attr-client",
		ClientSecret:           "attr-secret",
		RedirectURI:            "https://localhost:8095/callback",
		AuthorizationEndpoint:  "https://idp.example.com/authorize",
		TokenEndpoint:          "https://idp.example.com/token",
		AttributeConfiguration: s.validAttributeConfig(),
	})
	defer s.deleteConnection("oidc", created.ID)

	s.Require().NotNil(created.AttributeConfiguration)
	s.Equal(s.userTypeName, created.AttributeConfiguration.UserTypeResolution.Default)

	res, err := doRequest(http.MethodGet, "/connections/oidc/"+created.ID, nil)
	s.Require().NoError(err)
	var fetched connectionResponse
	s.Require().NoError(res.decode(&fetched))
	s.Require().NotNil(fetched.AttributeConfiguration)
	s.Equal(created.AttributeConfiguration, fetched.AttributeConfiguration)

	updated := s.validAttributeConfig()
	updated.UserTypeAttributeMappings[0].Attributes = []testutils.AttributeMapping{
		{ExternalAttribute: "nickname", LocalAttribute: "firstName"},
	}
	updateRes, err := doRequest(http.MethodPut, "/connections/oidc/"+created.ID, oidcConnectionRequest{
		Name:                   "OIDC Attr Round Trip",
		ClientID:               "attr-client",
		RedirectURI:            "https://localhost:8095/callback",
		AuthorizationEndpoint:  "https://idp.example.com/authorize",
		TokenEndpoint:          "https://idp.example.com/token",
		AttributeConfiguration: updated,
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, updateRes.status, string(updateRes.body))

	res, err = doRequest(http.MethodGet, "/connections/oidc/"+created.ID, nil)
	s.Require().NoError(err)
	var afterUpdate connectionResponse
	s.Require().NoError(res.decode(&afterUpdate))
	s.Equal(updated, afterUpdate.AttributeConfiguration)
}

// A3-A9, A20-A22: every validation class is rejected on create. The unit tests for these call the service's
// private validator directly, so nothing previously proved the create path rejects them.
func (s *ConnectionAPITestSuite) TestAttributeConfigurationValidationOnCreate() {
	for _, invalid := range s.invalidAttributeConfigs() {
		s.Run(invalid.name, func() {
			res, err := doRequest(http.MethodPost, "/connections/oauth",
				s.oauthRequestWithConfig("OAuth Invalid "+invalid.name, invalid.config))
			s.Require().NoError(err)
			s.Equal(http.StatusBadRequest, res.status,
				"expected 400 for %s, got %d: %s", invalid.name, res.status, string(res.body))
			s.NotEmpty(res.errorCode(), "error response should carry a code")
		})
	}
}

// A16: the same classes rejected on update. Create and update validate through separate call sites, so
// A3-A9, A20-A22 passing says nothing about this path.
func (s *ConnectionAPITestSuite) TestAttributeConfigurationValidationOnUpdate() {
	created := s.createConnection("oauth",
		s.oauthRequestWithConfig("OAuth Invalid Update Base", s.validAttributeConfig()))
	defer s.deleteConnection("oauth", created.ID)

	for _, invalid := range s.invalidAttributeConfigs() {
		s.Run(invalid.name, func() {
			res, err := doRequest(http.MethodPut, "/connections/oauth/"+created.ID,
				s.oauthRequestWithConfig("OAuth Invalid Update Base", invalid.config))
			s.Require().NoError(err)
			s.Equal(http.StatusBadRequest, res.status,
				"expected 400 for %s, got %d: %s", invalid.name, res.status, string(res.body))
		})
	}
}

// A17: a rejected update must not partially apply. The stored configuration is unchanged afterwards.
func (s *ConnectionAPITestSuite) TestRejectedUpdateLeavesStoredConfigurationUnchanged() {
	created := s.createConnection("oauth",
		s.oauthRequestWithConfig("OAuth Atomic Update", s.validAttributeConfig()))
	defer s.deleteConnection("oauth", created.ID)

	invalid := s.invalidAttributeConfigs()[0]
	res, err := doRequest(http.MethodPut, "/connections/oauth/"+created.ID,
		s.oauthRequestWithConfig("OAuth Atomic Update", invalid.config))
	s.Require().NoError(err)
	s.Require().Equal(http.StatusBadRequest, res.status, string(res.body))

	res, err = doRequest(http.MethodGet, "/connections/oauth/"+created.ID, nil)
	s.Require().NoError(err)
	var fetched connectionResponse
	s.Require().NoError(res.decode(&fetched))
	s.Require().NotNil(fetched.AttributeConfiguration)
	s.Equal(created.AttributeConfiguration, fetched.AttributeConfiguration,
		"rejected update must leave the previously stored configuration intact")
}

// A15: a malformed body on update is a client error. Only the create path was covered before.
func (s *ConnectionAPITestSuite) TestMalformedBodyOnUpdateReturnsBadRequest() {
	created := s.createConnection("oauth",
		s.oauthRequestWithConfig("OAuth Malformed Update", s.validAttributeConfig()))
	defer s.deleteConnection("oauth", created.ID)

	res, err := doRawRequest(http.MethodPut, "/connections/oauth/"+created.ID, "{\"name\": ")
	s.Require().NoError(err)
	s.Equal(http.StatusBadRequest, res.status, string(res.body))
}

// A10: Google with an email-granting scope and no explicit configuration is seeded with email account
// linking and a username mapping.
func (s *ConnectionAPITestSuite) TestGoogleSeedsAttributeConfigurationDefaults() {
	created := s.createConnection("google", googleConnectionRequest{
		Name:         "Google Seeded Defaults",
		ClientID:     "google-client",
		ClientSecret: "google-secret",
		RedirectURI:  "https://localhost:8095/callback",
		Scopes:       []string{"openid", "email", "profile"},
	})
	defer s.deleteConnection("google", created.ID)

	config := created.AttributeConfiguration
	s.Require().NotNil(config, "expected seeded configuration")
	s.Require().NotNil(config.UserTypeResolution)
	s.NotEmpty(config.UserTypeResolution.Default, "seeding records the resolution default")
	s.assertEmailLinkingSeeded(config)

	// The default names whichever username-requiring type sorts first across the deployment, so the
	// assertion is that our type received the mapping, not that it was chosen as the default.
	mapping := findMapping(config, s.userTypeName)
	s.Require().NotNil(mapping, "expected a mapping entry for %s", s.userTypeName)
	s.Equal([]testutils.AttributeMapping{{ExternalAttribute: "email", LocalAttribute: "username"}},
		mapping.Attributes)
}

// A11: generic OAuth is seeded with nothing, by design. scopesGrantEmail cannot infer an arbitrary
// provider's scope semantics, so neither linking nor a username mapping is inferred.
func (s *ConnectionAPITestSuite) TestGenericOAuthSeedsNothing() {
	created := s.createConnection("oauth", oauthConnectionRequest{
		Name:                  "OAuth No Seeding",
		ClientID:              "attr-client",
		ClientSecret:          "attr-secret",
		RedirectURI:           "https://localhost:8095/callback",
		AuthorizationEndpoint: "https://idp.example.com/authorize",
		TokenEndpoint:         "https://idp.example.com/token",
		UserInfoEndpoint:      "https://idp.example.com/userinfo",
		Scopes:                []string{"openid", "email", "profile"},
	})
	defer s.deleteConnection("oauth", created.ID)

	s.Nil(created.AttributeConfiguration,
		"generic OAuth must not infer defaults: got %+v", created.AttributeConfiguration)
}

// A12: GitHub derives the username from its login claim rather than from email.
func (s *ConnectionAPITestSuite) TestGitHubSeedsUsernameFromLoginClaim() {
	created := s.createConnection("github", githubConnectionRequest{
		Name:         "GitHub Seeded Defaults",
		ClientID:     "github-client",
		ClientSecret: "github-secret",
		RedirectURI:  "https://localhost:8095/callback",
	})
	defer s.deleteConnection("github", created.ID)

	config := created.AttributeConfiguration
	s.Require().NotNil(config, "expected seeded configuration")
	mapping := findMapping(config, s.userTypeName)
	s.Require().NotNil(mapping, "expected a mapping entry for %s", s.userTypeName)
	s.Equal([]testutils.AttributeMapping{{ExternalAttribute: "login", LocalAttribute: "username"}},
		mapping.Attributes,
		"GitHub emits no email claim by default, so the username source is its login claim")
}

// A13: an explicit configuration on create is preserved verbatim; seeding does not overwrite it.
func (s *ConnectionAPITestSuite) TestExplicitConfigurationWinsOverSeeding() {
	explicit := &testutils.AttributeConfiguration{
		UserTypeResolution: &testutils.UserTypeResolution{Default: s.userTypeName},
		UserTypeAttributeMappings: []testutils.UserTypeAttributeMapping{{
			UserType: s.userTypeName,
			Attributes: []testutils.AttributeMapping{
				{ExternalAttribute: "given_name", LocalAttribute: "username"},
			},
		}},
		AccountLinking: &testutils.AccountLinking{Attributes: []string{"firstName"}},
	}

	created := s.createConnection("google", googleConnectionRequest{
		Name:                   "Google Explicit Config",
		ClientID:               "google-client",
		ClientSecret:           "google-secret",
		RedirectURI:            "https://localhost:8095/callback",
		Scopes:                 []string{"openid", "email", "profile"},
		AttributeConfiguration: explicit,
	})
	defer s.deleteConnection("google", created.ID)

	config := created.AttributeConfiguration
	s.Require().NotNil(config)
	s.Equal([]string{"firstName"}, config.AccountLinking.Attributes,
		"explicit linking must not be replaced by the email default")
	mapping := findMapping(config, s.userTypeName)
	s.Require().NotNil(mapping)
	s.Equal([]testutils.AttributeMapping{{ExternalAttribute: "given_name", LocalAttribute: "username"}},
		mapping.Attributes, "explicit mapping must not be replaced by the seeded one")
	// "Verbatim" is only proven by comparing the whole configuration, including the resolution section.
	s.Equal(explicit, config)
}

// A14: seeding runs on create only. An update omitting the section removes it rather than restoring the
// seeded value, so a section the administrator deleted stays deleted.
func (s *ConnectionAPITestSuite) TestUpdateWithoutConfigurationRemovesSeededDefaults() {
	created := s.createConnection("google", googleConnectionRequest{
		Name:         "Google Remove Seeded",
		ClientID:     "google-client",
		ClientSecret: "google-secret",
		RedirectURI:  "https://localhost:8095/callback",
		Scopes:       []string{"openid", "email", "profile"},
	})
	defer s.deleteConnection("google", created.ID)
	s.Require().NotNil(created.AttributeConfiguration, "precondition: create seeded a configuration")

	updateRes, err := doRequest(http.MethodPut, "/connections/google/"+created.ID, googleConnectionRequest{
		Name:        "Google Remove Seeded",
		ClientID:    "google-client",
		RedirectURI: "https://localhost:8095/callback",
		Scopes:      []string{"openid", "email", "profile"},
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, updateRes.status, string(updateRes.body))

	res, err := doRequest(http.MethodGet, "/connections/google/"+created.ID, nil)
	s.Require().NoError(err)
	var fetched connectionResponse
	s.Require().NoError(res.decode(&fetched))
	s.Nil(fetched.AttributeConfiguration,
		"update must not re-seed: got %+v", fetched.AttributeConfiguration)
}

// A18: the two sections seed independently. Supplying one leaves it untouched and seeds only the other.
func (s *ConnectionAPITestSuite) TestPartialConfigurationSeedsOnlyTheMissingSection() {
	s.Run("explicit_linking_seeds_mappings", func() {
		created := s.createConnection("google", googleConnectionRequest{
			Name:         "Google Partial Linking",
			ClientID:     "google-client",
			ClientSecret: "google-secret",
			RedirectURI:  "https://localhost:8095/callback",
			Scopes:       []string{"openid", "email", "profile"},
			AttributeConfiguration: &testutils.AttributeConfiguration{
				AccountLinking: &testutils.AccountLinking{Attributes: []string{"firstName"}},
			},
		})
		defer s.deleteConnection("google", created.ID)

		config := created.AttributeConfiguration
		s.Require().NotNil(config)
		s.Equal([]string{"firstName"}, config.AccountLinking.Attributes, "explicit linking untouched")
		s.Require().NotNil(findMapping(config, s.userTypeName), "mappings seeded")
	})

	s.Run("explicit_mappings_seed_linking", func() {
		created := s.createConnection("google", googleConnectionRequest{
			Name:         "Google Partial Mappings",
			ClientID:     "google-client",
			ClientSecret: "google-secret",
			RedirectURI:  "https://localhost:8095/callback",
			Scopes:       []string{"openid", "email", "profile"},
			AttributeConfiguration: &testutils.AttributeConfiguration{
				UserTypeResolution: &testutils.UserTypeResolution{Default: s.userTypeName},
				UserTypeAttributeMappings: []testutils.UserTypeAttributeMapping{{
					UserType: s.userTypeName,
					Attributes: []testutils.AttributeMapping{
						{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
					},
				}},
			},
		})
		defer s.deleteConnection("google", created.ID)

		config := created.AttributeConfiguration
		s.Require().NotNil(config)
		s.assertEmailLinkingSeeded(config)
		mapping := findMapping(config, s.userTypeName)
		s.Require().NotNil(mapping)
		s.Equal([]testutils.AttributeMapping{{ExternalAttribute: "given_name", LocalAttribute: "firstName"}},
			mapping.Attributes, "explicit mappings untouched")
	})
}

// googleRawRequest builds a Google create body as a generic map. The typed request struct carries
// omitempty on both configuration slices, so an empty Go slice and an absent field serialize
// identically; a map is the only way to put an explicit [] or {} on the wire.
func (s *ConnectionAPITestSuite) googleRawRequest(
	name string, config map[string]interface{}) map[string]interface{} {
	body := map[string]interface{}{
		"name":         name,
		"clientId":     "google-client",
		"clientSecret": "google-secret",
		"redirectUri":  "https://localhost:8095/callback",
		"scopes":       []string{"openid", "email", "profile"},
	}
	if config != nil {
		body["attributeConfiguration"] = config
	}
	return body
}

// A19: which nil-versus-empty distinctions actually survive the wire format. The section pointers carry a
// real distinction — a present-but-empty section suppresses seeding where an absent one triggers it — while
// the slices collapse, because omitempty makes an explicit [] indistinguishable from an absent field once
// serialized. Every case here is sent as a raw map: building them from the typed struct would make the
// "empty" and "omitted" requests byte-identical and the comparison vacuous.
func (s *ConnectionAPITestSuite) TestNilVersusEmptyConfigurationSections() {
	s.Run("present_but_empty_linking_section_suppresses_seeding", func() {
		cases := map[string]map[string]interface{}{
			"empty_object":     {"accountLinking": map[string]interface{}{}},
			"empty_attributes": {"accountLinking": map[string]interface{}{"attributes": []interface{}{}}},
		}

		for label, config := range cases {
			created := s.createConnection("google", s.googleRawRequest("Google Linking "+label, config))
			s.Require().NotNil(created.AttributeConfiguration, label)
			s.Require().NotNil(created.AttributeConfiguration.AccountLinking,
				"%s: a present section must survive as non-nil rather than being treated as absent", label)
			s.Empty(created.AttributeConfiguration.AccountLinking.Attributes,
				"%s: a present-but-empty section suppresses the email default", label)
			s.deleteConnection("google", created.ID)
		}

		// The contrast that makes the pointer distinction load-bearing: omitting the section entirely seeds it.
		created := s.createConnection("google", s.googleRawRequest("Google Linking omitted", nil))
		defer s.deleteConnection("google", created.ID)
		s.Require().NotNil(created.AttributeConfiguration)
		s.assertEmailLinkingSeeded(created.AttributeConfiguration)
	})

	s.Run("explicit_empty_and_omitted_mappings_collapse", func() {
		linking := map[string]interface{}{"attributes": []string{"email"}}
		cases := map[string]map[string]interface{}{
			"explicit_empty_array": {
				"accountLinking":            linking,
				"userTypeAttributeMappings": []interface{}{},
			},
			"omitted": {"accountLinking": linking},
		}

		for label, config := range cases {
			created := s.createConnection("google", s.googleRawRequest("Google Mappings "+label, config))
			s.Require().NotNil(created.AttributeConfiguration, label)
			s.NotNil(findMapping(created.AttributeConfiguration, s.userTypeName),
				"%s: an explicit empty array and an absent field both take the len()==0 seeding branch", label)
			s.deleteConnection("google", created.ID)
		}
	})
}

// A23: authorization mapping targets that name a real role, group, and resource-server permission are
// accepted on both create and update. A20-A22 prove the rejection side of the existence check; this is
// the accepting side, so the check is proven to gate on existence rather than reject everything.
func (s *ConnectionAPITestSuite) TestAttributeConfigurationWithExistingAuthorizationMappingTargetsAccepted() {
	config := &testutils.AttributeConfiguration{
		UserTypeResolution: &testutils.UserTypeResolution{Default: s.userTypeName},
		AuthorizationMappings: []testutils.AuthorizationMapping{{
			Claim: "groups",
			Values: []testutils.AuthorizationRule{{
				Operator: testutils.AuthorizationOperatorEquals,
				Value:    "admins",
				Targets: []testutils.AuthorizationTarget{
					{Type: testutils.AuthorizationTargetRole, ID: s.authzRoleID},
					{Type: testutils.AuthorizationTargetGroup, ID: s.authzGroupID},
					{
						Type:             testutils.AuthorizationTargetPermission,
						ResourceServerID: s.authzResourceServerID,
						Permission:       "read",
					},
				},
			}},
		}},
	}

	created := s.createConnection("oauth", s.oauthRequestWithConfig("OAuth Authz Mapping Accepted", config))
	defer s.deleteConnection("oauth", created.ID)
	s.Require().NotNil(created.AttributeConfiguration, "create response should echo the configuration")
	s.Equal(config.AuthorizationMappings, created.AttributeConfiguration.AuthorizationMappings)

	updateRes, err := doRequest(http.MethodPut, "/connections/oauth/"+created.ID,
		s.oauthRequestWithConfig("OAuth Authz Mapping Accepted", config))
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, updateRes.status, string(updateRes.body))
}
