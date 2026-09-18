// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package managementtoken

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	testServerURL = "https://localhost:8095"
	// managementToken matches server.security.management_token in the integration deployment.
	managementToken = "integration-management-token"
)

type ManagementTokenTestSuite struct {
	suite.Suite
}

func TestManagementTokenTestSuite(t *testing.T) {
	suite.Run(t, new(ManagementTokenTestSuite))
}

// The token authenticates a call to the import API, which is what it exists for: a deployment
// pipeline pushing configuration without first obtaining OAuth client credentials.
func (ts *ManagementTokenTestSuite) TestTheTokenAuthenticatesTheImportAPI() {
	status := ts.post("/import", managementToken, `{"content":"","dryRun":true}`)

	// Whatever the import makes of an empty document, it was let through the gate: an unauthenticated
	// call is refused with 401 before the body is read at all.
	ts.NotEqual(http.StatusUnauthorized, status)
	ts.NotEqual(http.StatusForbidden, status)
}

// Without it, the same call is refused.
func (ts *ManagementTokenTestSuite) TestNoCredentialIsRefused() {
	ts.Equal(http.StatusUnauthorized, ts.post("/import", "", `{"content":""}`))
}

// A bearer value that is not the token falls through to the ordinary OAuth check, which refuses it.
// That fallthrough is what keeps tokens working on these same paths.
func (ts *ManagementTokenTestSuite) TestAWrongTokenIsRefused() {
	for _, wrong := range []string{
		"not-the-management-token",
		managementToken + "x",
		managementToken[:len(managementToken)-1],
	} {
		ts.Equal(http.StatusUnauthorized, ts.post("/import", wrong, `{"content":""}`),
			"a bearer value of %q must not authenticate", wrong)
	}
}

// The token is trusted on the import and variable store paths and nowhere else. If this starts
// passing, the token has been widened into a key for the whole management surface.
func (ts *ManagementTokenTestSuite) TestTheTokenOpensNothingElse() {
	for _, path := range []string{
		"/users",
		"/applications",
		"/organization-units",
		"/user-types",
		"/groups",
	} {
		status := ts.get(path, managementToken)
		ts.Equal(http.StatusUnauthorized, status,
			"GET %s accepted the management token; it is trusted only for import and the variable store",
			path)
	}
}

// An OAuth token still works on the paths the management token also opens.
func (ts *ManagementTokenTestSuite) TestOAuthStillWorksOnTheSamePaths() {
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/import",
		bytes.NewReader([]byte(`{"content":"","dryRun":true}`)))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	ts.NotEqual(http.StatusUnauthorized, resp.StatusCode,
		"the management token must not have displaced OAuth on this path")
}

func (ts *ManagementTokenTestSuite) post(path, token, body string) int {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodPost, testServerURL+path, bytes.NewReader([]byte(body)))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

func (ts *ManagementTokenTestSuite) get(path, token string) int {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodGet, testServerURL+path, nil)
	ts.Require().NoError(err)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}
