// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package managementapikey

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// apiKeyHeader is the header a management API key is presented in. Spelled out rather than
	// imported: the header name is part of the contract this suite is checking.
	apiKeyHeader = "API-Key"
	// managementAPIKey is the key whose SHA-256 digest is configured as
	// server.security.management_api_key_hash in the integration deployment. The deployment holds
	// only the digest, so this value appears nowhere in its configuration.
	managementAPIKey = "integration-management-api-key"
	// managedImportPath is the import a control plane deploys through. The key opens the plain import too, but
	// only this one marks what it writes as read only.
	managedImportPath = "/import/managed"
)

var testServerURL = testutils.TestServerURL

type ManagementAPIKeyTestSuite struct {
	suite.Suite
}

func TestManagementAPIKeyTestSuite(t *testing.T) {
	suite.Run(t, new(ManagementAPIKeyTestSuite))
}

// The key authenticates a call to the managed import, which is what it exists for: a control plane
// deploying configuration without first obtaining OAuth client credentials. It also proves the hashing
// round trip end to end: the server was configured with a digest and still recognized the key presented
// here.
func (ts *ManagementAPIKeyTestSuite) TestTheKeyAuthenticatesTheManagedImport() {
	status := ts.post(managedImportPath, managementAPIKey, `{"content":"","dryRun":true}`)

	// Whatever the import makes of an empty document, it was let through the gate: an unauthenticated
	// call is refused with 401 before the body is read at all.
	ts.NotEqual(http.StatusUnauthorized, status)
	ts.NotEqual(http.StatusForbidden, status)
}

// Without it, the same call is refused.
func (ts *ManagementAPIKeyTestSuite) TestNoCredentialIsRefused() {
	ts.Equal(http.StatusUnauthorized, ts.post(managedImportPath, "", `{"content":""}`))
}

// Presenting a key is a claim to be authenticated by it, so an API-Key value that is not the key is
// refused rather than passed to the OAuth check. A request that presents none is unaffected, which
// is what keeps OAuth working on these same paths.
func (ts *ManagementAPIKeyTestSuite) TestAWrongKeyIsRefused() {
	for _, wrong := range []string{
		"not-the-management-api-key",
		managementAPIKey + "x",
		managementAPIKey[:len(managementAPIKey)-1],
	} {
		ts.Equal(http.StatusUnauthorized, ts.post(managedImportPath, wrong, `{"content":""}`),
			"an API-Key value of %q must not authenticate", wrong)
	}
}

// Presenting the digest rather than the key must not authenticate. Whoever reads the deployment
// configuration holds the digest, and it has to be worth nothing to them.
func (ts *ManagementAPIKeyTestSuite) TestTheConfiguredDigestIsNotItselfACredential() {
	digest := "4955d93012fc5e2ee07b527f13ffe55de32900c410cc198f4d7a9971bb3a9a2d"

	ts.Equal(http.StatusUnauthorized, ts.post(managedImportPath, digest, `{"content":""}`),
		"the configured digest must not authenticate as though it were the key")
}

// The key belongs in its own header. Presented as a bearer token it is just an unknown token.
func (ts *ManagementAPIKeyTestSuite) TestTheKeyIsNotAcceptedAsABearerToken() {
	req, err := http.NewRequest(http.MethodPost, testServerURL+managedImportPath,
		bytes.NewReader([]byte(`{"content":""}`)))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+managementAPIKey)

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	ts.Equal(http.StatusUnauthorized, resp.StatusCode,
		"the management API key must be presented in the API-Key header, not in Authorization")
}

// The key is trusted on the import and variable store paths and nowhere else. If this starts
// passing, the key has been widened into one for the whole management surface.
func (ts *ManagementAPIKeyTestSuite) TestTheKeyOpensNothingElse() {
	for _, path := range []string{
		"/users",
		"/applications",
		"/organization-units",
		"/user-types",
		"/groups",
	} {
		status := ts.get(path, managementAPIKey)
		ts.Equal(http.StatusUnauthorized, status,
			"GET %s accepted the management API key; it is trusted only for the import and the "+
				"variable store",
			path)
	}
}

// The key also opens the plain import, for a pipeline that deploys editable resources, and a data plane
// that a control plane without the managed import still deploys to.
func (ts *ManagementAPIKeyTestSuite) TestTheKeyOpensThePlainImport() {
	ts.NotEqual(http.StatusUnauthorized, ts.post("/import", managementAPIKey, `{"content":"","dryRun":true}`))
	ts.NotEqual(http.StatusUnauthorized, ts.post("/import/delete", managementAPIKey, `{}`))
}

// The managed import is the control plane's: an administrator's token is refused there, root permission
// or not.
func (ts *ManagementAPIKeyTestSuite) TestAnAdministratorCannotUseTheManagedImport() {
	req, err := http.NewRequest(http.MethodPost, testServerURL+managedImportPath,
		bytes.NewReader([]byte(`{"content":"","dryRun":true}`)))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	ts.Equal(http.StatusForbidden, resp.StatusCode)
	ts.Equal("IMP-1008", errorCode(ts.T(), resp))
}

// An OAuth token still works on the plain import.
func (ts *ManagementAPIKeyTestSuite) TestOAuthStillWorksOnThePlainImport() {
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/import",
		bytes.NewReader([]byte(`{"content":"","dryRun":true}`)))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	ts.NotEqual(http.StatusUnauthorized, resp.StatusCode,
		"OAuth must still open the plain import")
}

func (ts *ManagementAPIKeyTestSuite) post(path, key, body string) int {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodPost, testServerURL+path, bytes.NewReader([]byte(body)))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set(apiKeyHeader, key)
	}

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

func (ts *ManagementAPIKeyTestSuite) get(path, key string) int {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodGet, testServerURL+path, nil)
	ts.Require().NoError(err)
	if key != "" {
		req.Header.Set(apiKeyHeader, key)
	}

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}
