// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"net/http"
	"net/url"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// unsignedRequestObject is an unsigned JWT ("alg":"none") carrying authorization request
// parameters, the form used by the OpenID Foundation conformance suite for its unsigned request
// object modules.
const unsignedRequestObject = "eyJhbGciOiAibm9uZSJ9." +
	"eyJyZXNwb25zZV90eXBlIjogImNvZGUiLCAic3RhdGUiOiAiaW5zaWRlLW9iamVjdCJ9."

// TestAuthorize_RequestObject_RequestNotSupported verifies that an authorization request carrying a
// request parameter is rejected rather than processed on the query string alone. JAR (RFC 9101) is
// not implemented, and silently ignoring the object would drop the parameters the client placed
// inside it, notably state and nonce. OIDC Core 6.1 defines request_not_supported for this.
func (ts *AuthzTestSuite) TestAuthorize_RequestObject_RequestNotSupported() {
	resp, err := testutils.InitiateAuthorizationFlowWithExtraParams(
		clientID, redirectURI, "code", "openid", "request-object-state",
		map[string]string{"request": unsignedRequestObject})
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode,
		"a request object should redirect the error back to the client")

	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location, "Location header should be present")

	redirected, err := url.Parse(location)
	ts.Require().NoError(err)
	ts.Require().Equal(redirectURI, redirected.Scheme+"://"+redirected.Host,
		"the error must go to the client's redirect_uri")

	query := redirected.Query()
	ts.Assert().Equal("request_not_supported", query.Get("error"))
	ts.Assert().NotEmpty(query.Get("error_description"))
	ts.Assert().Equal("request-object-state", query.Get("state"),
		"state must be echoed back to the client")
}

// TestAuthorize_RequestURI_RequestURINotSupported verifies that a request_uri which is not a PAR
// handle is rejected with request_uri_not_supported. Passing a request object by reference is not
// supported, and the reference must not be resolved or ignored.
func (ts *AuthzTestSuite) TestAuthorize_RequestURI_RequestURINotSupported() {
	resp, err := testutils.InitiateAuthorizationFlowWithExtraParams(
		clientID, redirectURI, "code", "openid", "request-uri-state",
		map[string]string{"request_uri": "https://client.example.org/request.jwt"})
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode)

	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location)

	redirected, err := url.Parse(location)
	ts.Require().NoError(err)
	ts.Require().Equal(redirectURI, redirected.Scheme+"://"+redirected.Host,
		"the error must go to the client's redirect_uri")

	query := redirected.Query()
	ts.Assert().Equal("request_uri_not_supported", query.Get("error"))
	ts.Assert().NotEmpty(query.Get("error_description"))
	ts.Assert().Equal("request-uri-state", query.Get("state"),
		"state must be echoed back to the client")
}
