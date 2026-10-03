// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	cimdPublicClientID = "https://cimd-public.example.com/oauth/client-metadata.json"
	// cimdConfidentialClientID is longer than 36 characters, so storing its certificate needs the
	// widened CERTIFICATE.REF_ID column.
	cimdConfidentialClientID = "https://cimd-confidential.example.com/oauth/client-metadata.json"
)

var cimdOU = testutils.OrganizationUnit{
	Handle:      "cimd-test-ou",
	Name:        "CIMD Test OU",
	Description: "Organization unit for the Client ID Metadata Document tests",
	Parent:      nil,
}

// CIMDTestSuite covers applications registered from a Client ID Metadata Document: create and update
// apply the document rules to the submitted values, and preview rejects URLs it must not retrieve.
type CIMDTestSuite struct {
	suite.Suite
	ouID string
}

func TestCIMDTestSuite(t *testing.T) {
	suite.Run(t, new(CIMDTestSuite))
}

func (ts *CIMDTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(cimdOU)
	ts.Require().NoError(err, "Failed to create the test organization unit")
	ts.ouID = ouID
}

func (ts *CIMDTestSuite) TearDownSuite() {
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete the test organization unit during teardown: %v", err)
		}
	}
}

// publicCIMDApp builds an MCP application with the values a preview returns for a public client.
func (ts *CIMDTestSuite) publicCIMDApp(name string) Application {
	return Application{
		OUID: ts.ouID,
		Name: name,
		Type: "mcp",
		InboundAuthConfig: []InboundAuthConfig{{
			Type: "oauth2",
			OAuthAppConfig: &OAuthAppConfig{
				ClientID:                 cimdPublicClientID,
				ClientIDMetadataDocument: true,
				RedirectURIs: []string{
					"https://cimd-public.example.com/callback", "http://127.0.0.1/callback"},
				GrantTypes:              []string{"authorization_code", "refresh_token"},
				ResponseTypes:           []string{"code"},
				TokenEndpointAuthMethod: "none",
				PKCERequired:            true,
				PublicClient:            true,
			},
		}},
	}
}

// sendJSON sends a JSON request and returns the status code, and the error code and description key from
// the response body.
func (ts *CIMDTestSuite) sendJSON(method, path string, body any) (int, string, string) {
	payload, err := json.Marshal(body)
	ts.Require().NoError(err)
	req, err := http.NewRequest(method, testServerURL+path, bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	var errResp struct {
		Code        string                `json:"code"`
		Description testutils.I18nMessage `json:"description"`
	}
	_ = json.Unmarshal(respBody, &errResp)
	return resp.StatusCode, errResp.Code, errResp.Description.Key
}

// requireRuleViolation asserts that an application request is rejected for breaking the named CIMD rule.
func (ts *CIMDTestSuite) requireRuleViolation(method, path string, body any, rule string) {
	status, code, descriptionKey := ts.sendJSON(method, path, body)
	ts.Equal(http.StatusBadRequest, status)
	ts.Equal("APP-1050", code)
	ts.Equal("error.cimdservice."+rule+"_description", descriptionKey)
}

func (ts *CIMDTestSuite) TestPublicCIMDApplicationLifecycle() {
	appID, err := createApplication(ts.publicCIMDApp("CIMD Public Client"))
	ts.Require().NoError(err)
	defer func() {
		if err := deleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete the CIMD application: %v", err)
		}
	}()

	app, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Require().Len(app.InboundAuthConfig, 1)
	oauthConfig := app.InboundAuthConfig[0].OAuthAppConfig
	ts.Equal(cimdPublicClientID, oauthConfig.ClientID)
	ts.True(oauthConfig.ClientIDMetadataDocument)
	ts.True(oauthConfig.PublicClient)

	updated := ts.publicCIMDApp("CIMD Public Client Renamed")
	ts.Require().NoError(updateApplication(appID, updated))

	changedClientID := ts.publicCIMDApp("CIMD Public Client Renamed")
	changedClientID.InboundAuthConfig[0].OAuthAppConfig.ClientID =
		"https://cimd-public.example.com/oauth/other-metadata.json"
	ts.requireRuleViolation(http.MethodPut, "/applications/"+appID, changedClientID, "immutable")

	withoutMarker := ts.publicCIMDApp("CIMD Public Client Renamed")
	withoutMarker.InboundAuthConfig[0].OAuthAppConfig.ClientIDMetadataDocument = false
	ts.requireRuleViolation(http.MethodPut, "/applications/"+appID, withoutMarker, "immutable")

	foreignRedirect := ts.publicCIMDApp("CIMD Public Client Renamed")
	foreignRedirect.InboundAuthConfig[0].OAuthAppConfig.RedirectURIs = []string{"https://evil.example.com/cb"}
	ts.requireRuleViolation(http.MethodPut, "/applications/"+appID, foreignRedirect, "invalid_redirect_uri")
}

func (ts *CIMDTestSuite) TestConfidentialCIMDApplication() {
	app := Application{
		OUID: ts.ouID,
		Name: "CIMD Confidential Client",
		Type: "custom",
		InboundAuthConfig: []InboundAuthConfig{{
			Type: "oauth2",
			OAuthAppConfig: &OAuthAppConfig{
				ClientID:                 cimdConfidentialClientID,
				ClientIDMetadataDocument: true,
				RedirectURIs:             []string{"https://cimd-confidential.example.com/callback"},
				GrantTypes:               []string{"authorization_code"},
				ResponseTypes:            []string{"code"},
				TokenEndpointAuthMethod:  "private_key_jwt",
				PKCERequired:             true,
				Certificate: &ApplicationCert{
					Type:  "JWKS_URI",
					Value: "https://cimd-confidential.example.com/oauth/jwks.json",
				},
			},
		}},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer func() {
		if err := deleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete the CIMD application: %v", err)
		}
	}()

	got, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	oauthConfig := got.InboundAuthConfig[0].OAuthAppConfig
	ts.True(oauthConfig.ClientIDMetadataDocument)
	ts.False(oauthConfig.PublicClient)
	ts.Require().NotNil(oauthConfig.Certificate)
	ts.Equal("JWKS_URI", oauthConfig.Certificate.Type)
	ts.Equal("https://cimd-confidential.example.com/oauth/jwks.json", oauthConfig.Certificate.Value)
}

func (ts *CIMDTestSuite) TestCreateRejectsRuleViolations() {
	cases := []struct {
		name   string
		modify func(c *OAuthAppConfig)
		rule   string
	}{
		{"client ID is not a URL", func(c *OAuthAppConfig) { c.ClientID = "cimd-not-a-url" }, "invalid_client_id"},
		{"foreign redirect URI", func(c *OAuthAppConfig) {
			c.RedirectURIs = []string{"https://evil.example.com/callback"}
		}, "invalid_redirect_uri"},
		{"secret-based authentication", func(c *OAuthAppConfig) {
			c.TokenEndpointAuthMethod = "client_secret_basic"
			c.PublicClient = false
		}, "invalid_auth_method"},
		{"client secret", func(c *OAuthAppConfig) {
			c.TokenEndpointAuthMethod = "private_key_jwt"
			c.PublicClient = false
			c.ClientSecret = "cimd-client-secret"
			c.Certificate = &ApplicationCert{Type: "JWKS_URI", Value: "https://cimd-public.example.com/jwks"}
		}, "client_secret_not_allowed"},
		{"unsafe JWKS URI", func(c *OAuthAppConfig) {
			c.TokenEndpointAuthMethod = "private_key_jwt"
			c.PublicClient = false
			c.Certificate = &ApplicationCert{Type: "JWKS_URI", Value: "http://cimd-public.example.com/jwks"}
		}, "invalid_jwks_uri"},
		{"confidential client without PKCE", func(c *OAuthAppConfig) {
			c.TokenEndpointAuthMethod = "private_key_jwt"
			c.PublicClient = false
			c.PKCERequired = false
			c.Certificate = &ApplicationCert{Type: "JWKS_URI", Value: "https://cimd-public.example.com/jwks"}
		}, "pkce_required"},
		{"redirect URI with a fragment", func(c *OAuthAppConfig) {
			c.RedirectURIs = []string{"https://cimd-public.example.com/callback#done"}
		}, "invalid_redirect_uri"},
		{"wildcard redirect URI", func(c *OAuthAppConfig) {
			// The integration deployment enables wildcard redirect URIs; a CIMD client still cannot use one.
			c.RedirectURIs = []string{"https://cimd-public.example.com/*"}
		}, "invalid_redirect_uri"},
		{"widened grants", func(c *OAuthAppConfig) {
			c.GrantTypes = []string{"authorization_code", "client_credentials"}
		}, "invalid_grant_types"},
	}
	for _, tc := range cases {
		ts.Run(tc.name, func() {
			app := ts.publicCIMDApp("CIMD Rejected Client")
			tc.modify(app.InboundAuthConfig[0].OAuthAppConfig)

			ts.requireRuleViolation(http.MethodPost, "/applications", app, tc.rule)
		})
	}
}

func (ts *CIMDTestSuite) TestPreviewRejectsURLs() {
	cases := []struct {
		name     string
		clientID string
		code     string
	}{
		{"not https", "http://cimd.example.com/client.json", "CIMD-1002"},
		{"IP literal", "https://127.0.0.1/client.json", "CIMD-1002"},
		{"no path", "https://cimd.example.com", "CIMD-1002"},
		{"loopback host", "https://localhost/client.json", "CIMD-1003"},
		{"unresolvable host", "https://cimd.invalid/client.json", "CIMD-1003"},
	}
	for _, tc := range cases {
		ts.Run(tc.name, func() {
			status, code, _ := ts.sendJSON(http.MethodPost, "/cimd/preview", map[string]string{"clientId": tc.clientID})

			ts.Equal(http.StatusBadRequest, status)
			ts.Equal(tc.code, code)
		})
	}
}
