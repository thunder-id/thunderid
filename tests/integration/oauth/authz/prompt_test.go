// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"net/http"
	"net/url"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// TestAuthorize_PromptNone_LoginRequired verifies that prompt=none is redirected back to the client
// with error=login_required when no SSO session exists, which is the specification's answer for
// "authentication is required but cannot be asked for" (OIDC Core §3.1.2.1). This request carries
// no session cookie; the silent-success path is covered in tests/integration/oauth/sso.
func (ts *AuthzTestSuite) TestAuthorize_PromptNone_LoginRequired() {
	resp, err := testutils.InitiateAuthorizationFlowWithPrompt(
		clientID, redirectURI, "code", "openid", "prompt-none-state", "none")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode, "prompt=none should redirect back to the client")

	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location, "Location header should be present")

	redirected, err := url.Parse(location)
	ts.Require().NoError(err)
	query := redirected.Query()

	ts.Assert().Equal("login_required", query.Get("error"))
	ts.Assert().NotEmpty(query.Get("error_description"))
	ts.Assert().Equal("prompt-none-state", query.Get("state"), "state must be echoed back to the client")
}

// TestAuthorize_PromptNoneCombinedWithOtherValues_InvalidRequest verifies that combining prompt=none
// with another prompt value is rejected as invalid_request, per OIDC Core §3.1.2.1.
func (ts *AuthzTestSuite) TestAuthorize_PromptNoneCombinedWithOtherValues_InvalidRequest() {
	resp, err := testutils.InitiateAuthorizationFlowWithPrompt(
		clientID, redirectURI, "code", "openid", "prompt-combined-state", "none login")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode)

	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location)

	redirected, err := url.Parse(location)
	ts.Require().NoError(err)
	query := redirected.Query()

	ts.Assert().Equal("invalid_request", query.Get("error"))
	ts.Assert().NotEmpty(query.Get("error_description"))
}

// TestAuthorize_PromptUnsupportedValue_InvalidRequest verifies that an unrecognized prompt value is
// rejected as invalid_request.
func (ts *AuthzTestSuite) TestAuthorize_PromptUnsupportedValue_InvalidRequest() {
	resp, err := testutils.InitiateAuthorizationFlowWithPrompt(
		clientID, redirectURI, "code", "openid", "prompt-unsupported-state", "not_a_real_prompt_value")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode)

	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location)

	redirected, err := url.Parse(location)
	ts.Require().NoError(err)
	query := redirected.Query()

	ts.Assert().Equal("invalid_request", query.Get("error"))
}

// TestAuthorize_PromptSelectAccount_AccountSelectionRequired verifies that prompt=select_account is
// redirected back to the client with error=account_selection_required, since the server does not
// support account selection prompts.
func (ts *AuthzTestSuite) TestAuthorize_PromptSelectAccount_AccountSelectionRequired() {
	resp, err := testutils.InitiateAuthorizationFlowWithPrompt(
		clientID, redirectURI, "code", "openid", "prompt-select-account-state", "select_account")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode, "prompt=select_account should redirect back to the client")

	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location, "Location header should be present")

	redirected, err := url.Parse(location)
	ts.Require().NoError(err)
	query := redirected.Query()

	ts.Assert().Equal("account_selection_required", query.Get("error"))
	ts.Assert().NotEmpty(query.Get("error_description"))
	ts.Assert().Equal("prompt-select-account-state", query.Get("state"), "state must be echoed back to the client")
}

// TestAuthorize_PromptEmpty_InvalidRequest verifies that a prompt parameter present but blank
// (prompt=, as opposed to the parameter being entirely absent) is rejected as invalid_request. The
// shared authorization-flow helper treats an empty string as "omit this parameter", so this test
// builds the raw request directly to represent the parameter being present with an empty value.
func (ts *AuthzTestSuite) TestAuthorize_PromptEmpty_InvalidRequest() {
	authorizeURL, err := url.Parse(testutils.TestServerURL + "/oauth2/authorize")
	ts.Require().NoError(err)

	query := url.Values{}
	query.Set("client_id", clientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("scope", "openid")
	query.Set("state", "prompt-empty-state")
	query.Set("prompt", "")
	authorizeURL.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodGet, authorizeURL.String(), nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetNoRedirectHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode, "blank prompt should redirect back to the client")

	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location, "Location header should be present")

	redirected, err := url.Parse(location)
	ts.Require().NoError(err)
	redirectQuery := redirected.Query()

	ts.Assert().Equal("invalid_request", redirectQuery.Get("error"))
	ts.Assert().NotEmpty(redirectQuery.Get("error_description"))
}
