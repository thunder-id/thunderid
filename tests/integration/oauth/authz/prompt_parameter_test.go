// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	promptClientID    = "authz_prompt_test_client"
	promptRedirectURI = "https://localhost:3000/prompt-callback"
)

var promptTestOU = testutils.OrganizationUnit{
	Handle:      "oauth2-prompt-test-ou",
	Name:        "OAuth2 Prompt Test OU",
	Description: "Organization unit for the OIDC prompt parameter tests",
	Parent:      nil,
}

// PromptParameterTestSuite covers the OIDC prompt parameter contract of the authorization endpoint
// (OIDC Core 3.1.2.1). Rejections arrive as error redirects to the client's redirect URI, so each
// case asserts the error code carried in that redirect.
type PromptParameterTestSuite struct {
	suite.Suite
	client        *http.Client
	ouID          string
	applicationID string
}

func TestPromptParameterTestSuite(t *testing.T) {
	suite.Run(t, new(PromptParameterTestSuite))
}

func (ts *PromptParameterTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(promptTestOU)
	ts.Require().NoError(err, "Failed to create the test organization unit")
	ts.ouID = ouID

	// The shipped default authentication flow is enough: no case here completes authentication.
	authFlowID, err := testutils.GetFlowIDByHandle("default-flow", "AUTHENTICATION")
	ts.Require().NoError(err, "Failed to resolve the default authentication flow")

	app := map[string]interface{}{
		"name":                      "OAuth2 Prompt Test App",
		"description":               "Application for the OIDC prompt parameter tests",
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"authFlowId":                authFlowID,
		"isRegistrationFlowEnabled": false,
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                promptClientID,
					"clientSecret":            "authz_prompt_test_secret",
					"redirectUris":            []string{promptRedirectURI},
					"grantTypes":              []string{"authorization_code", "refresh_token"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	}

	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err, "Failed to encode the application payload")

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err, "Failed to create the application request")
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to send the application request")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "Failed to read the application response")
	ts.Require().Equal(http.StatusCreated, resp.StatusCode,
		"Creating the test application should return 201: %s", string(body))

	var created struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal(body, &created), "Failed to parse the application response")
	ts.applicationID = created.ID
}

func (ts *PromptParameterTestSuite) TearDownSuite() {
	if ts.applicationID != "" {
		req, err := http.NewRequest(http.MethodDelete,
			fmt.Sprintf("%s/applications/%s", testutils.TestServerURL, ts.applicationID), nil)
		if err == nil {
			resp, doErr := ts.client.Do(req)
			if doErr != nil {
				ts.T().Logf("Failed to delete the test application during teardown: %v", doErr)
			} else {
				resp.Body.Close()
			}
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete the test organization unit during teardown: %v", err)
		}
	}
}

// authorize issues an authorization request carrying the given prompt value.
func (ts *PromptParameterTestSuite) authorize(prompt string, promptSet bool) *http.Response {
	params := url.Values{}
	params.Set("client_id", promptClientID)
	params.Set("redirect_uri", promptRedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid")
	params.Set("state", "prompt-test-state")
	if promptSet {
		params.Set("prompt", prompt)
	}

	resp, err := testutils.SubmitAuthorizationRequest(params)
	ts.Require().NoError(err, "Failed to send the authorization request")
	return resp
}

// TestRejectedPromptValues asserts that every prompt value the server cannot honour is reported as an
// error redirect to the client, with the error code OIDC prescribes for it.
func (ts *PromptParameterTestSuite) TestRejectedPromptValues() {
	testCases := []struct {
		name          string
		prompt        string
		expectedError string
	}{
		{
			name:          "empty prompt",
			prompt:        "",
			expectedError: "invalid_request",
		},
		{
			name:          "whitespace only prompt",
			prompt:        "   ",
			expectedError: "invalid_request",
		},
		{
			name:          "unsupported prompt value",
			prompt:        "reauthenticate",
			expectedError: "invalid_request",
		},
		{
			name:          "none combined with another value",
			prompt:        "none login",
			expectedError: "invalid_request",
		},
		{
			// The server does not serve an authorization request without user interaction, so a
			// request that forbids interaction is answered with login_required.
			name:          "none",
			prompt:        "none",
			expectedError: "login_required",
		},
		{
			// Account selection is not implemented, which OIDC represents with its own error code
			// rather than a generic invalid_request.
			name:          "select_account",
			prompt:        "select_account",
			expectedError: "account_selection_required",
		},
	}

	for _, tc := range testCases {
		ts.Run(tc.name, func() {
			resp := ts.authorize(tc.prompt, true)
			defer resp.Body.Close()

			ts.Require().Equal(http.StatusFound, resp.StatusCode,
				"A rejected prompt must be reported as a redirect")
			location := resp.Header.Get("Location")
			ts.Require().NotEmpty(location, "The rejection must carry a redirect location")

			parsed, err := url.Parse(location)
			ts.Require().NoError(err, "Failed to parse the redirect location")
			ts.Equal(promptRedirectURI, parsed.Scheme+"://"+parsed.Host+parsed.Path,
				"The error must be redirected to the client's registered redirect URI")
			ts.Equal("prompt-test-state", parsed.Query().Get("state"),
				"An error redirect must echo the request's state")
			ts.Require().NoError(testutils.ValidateOAuth2ErrorRedirect(location, tc.expectedError, ""),
				"Unexpected error for prompt %q in redirect %s", tc.prompt, location)
		})
	}
}

// TestAcceptedPromptValues asserts that the interactive prompt values are accepted: the request is
// not turned into an error redirect but continues to the login application. Being accepted at all is
// the assertion here, since these values only affect how authentication is presented.
func (ts *PromptParameterTestSuite) TestAcceptedPromptValues() {
	for _, prompt := range []string{"login", "consent", "login consent"} {
		ts.Run("prompt "+prompt, func() {
			resp := ts.authorize(prompt, true)
			defer resp.Body.Close()

			ts.Require().Equal(http.StatusFound, resp.StatusCode,
				"An accepted authorization request redirects to the login application")
			location := resp.Header.Get("Location")
			ts.Require().NotEmpty(location, "The redirect must carry a location")

			parsed, err := url.Parse(location)
			ts.Require().NoError(err, "Failed to parse the redirect location")
			ts.Empty(parsed.Query().Get("error"),
				"An accepted prompt value must not produce an OAuth2 error: %s", location)
			ts.Empty(parsed.Query().Get("errorCode"),
				"An accepted prompt value must not produce a server error page: %s", location)
			ts.NotEqual(promptRedirectURI, parsed.Scheme+"://"+parsed.Host+parsed.Path,
				"An accepted request is not redirected back to the client yet: %s", location)
		})
	}
}

// TestOmittedPromptIsAccepted asserts that prompt is optional: omitting it entirely leaves the
// request untouched by prompt validation.
func (ts *PromptParameterTestSuite) TestOmittedPromptIsAccepted() {
	resp := ts.authorize("", false)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusFound, resp.StatusCode,
		"An authorization request without prompt redirects to the login application")
	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location, "The redirect must carry a location")

	parsed, err := url.Parse(location)
	ts.Require().NoError(err, "Failed to parse the redirect location")
	ts.Empty(parsed.Query().Get("error"), "Omitting prompt must not produce an OAuth2 error: %s", location)
}
