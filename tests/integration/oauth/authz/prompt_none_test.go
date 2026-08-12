// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"net/http"
	"net/url"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// TestAuthorize_PromptNone_LoginRequired verifies that prompt=none is redirected back to the client
// with error=login_required, since the server does not support silent re-authentication via
// server-side sessions (OIDC Core §3.1.2.1).
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
