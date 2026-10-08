// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authn

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const directAPIConfigKey = "direct_api"

var directAPIEndpoints = []string{
	"/auth/credentials/authenticate",
	"/auth/otp/sms/send",
	"/auth/otp/sms/verify",
	"/auth/oauth/google/start",
	"/auth/oauth/google/finish",
	"/auth/oauth/github/start",
	"/auth/oauth/github/finish",
	"/auth/oauth/standard/start",
	"/auth/oauth/standard/finish",
	"/auth/passkey/start",
	"/auth/passkey/finish",
	"/register/passkey/start",
	"/register/passkey/finish",
}

// DirectAPIDisabledTestSuite verifies that direct_api.enabled: false removes the Direct API endpoints
// while AuthZEN access and flow execution stay available. It restarts the server with the patched
// config and restores the original config afterwards.
type DirectAPIDisabledTestSuite struct {
	suite.Suite
	client         *http.Client
	originalConfig interface{}
}

func TestDirectAPIDisabledTestSuite(t *testing.T) {
	suite.Run(t, new(DirectAPIDisabledTestSuite))
}

func (ts *DirectAPIDisabledTestSuite) SetupSuite() {
	ts.client = testutils.GetRawHTTPClient()

	original, err := testutils.ReadDeploymentConfigKey(directAPIConfigKey)
	ts.Require().NoError(err, "failed to read the direct_api config")
	ts.originalConfig = original

	ts.Require().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{
		directAPIConfigKey: map[string]interface{}{"enabled": false},
	}), "failed to disable the Direct API")
	ts.Require().NoError(testutils.RestartServer(), "failed to restart server with the Direct API disabled")
	ts.Require().NoError(testutils.ObtainAdminAccessToken(), "failed to re-obtain admin token after restart")
}

func (ts *DirectAPIDisabledTestSuite) TearDownSuite() {
	// Restores the value read in SetupSuite. A missing key is written back as null, which the server
	// treats the same as an absent key. Assert rather than Require, so one failure does not skip the
	// restoration steps that follow it.
	ts.Assert().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{
		directAPIConfigKey: ts.originalConfig,
	}), "teardown: failed to restore the direct_api config")
	ts.Assert().NoError(testutils.RestartServer(), "teardown: server did not restart cleanly after config restore")
	ts.Assert().NoError(testutils.ObtainAdminAccessToken(), "teardown: failed to re-obtain admin token after restore")
}

// TestDirectAPIEndpointsNotFound verifies every Direct API endpoint returns 404, even with a valid
// Direct Auth Secret.
func (ts *DirectAPIDisabledTestSuite) TestDirectAPIEndpointsNotFound() {
	for _, endpoint := range directAPIEndpoints {
		ts.Equal(http.StatusNotFound, ts.send(http.MethodPost, endpoint, testutils.DirectAuthHeaderValue),
			"POST %s", endpoint)
		ts.Equal(http.StatusNotFound, ts.send(http.MethodOptions, endpoint, ""), "OPTIONS %s", endpoint)
	}
}

// TestAuthZENAccessEndpointStillGated verifies the AuthZEN access endpoints stay registered and keep
// the Direct Auth Secret gate.
func (ts *DirectAPIDisabledTestSuite) TestAuthZENAccessEndpointStillGated() {
	const evaluationEndpoint = "/access/v1/evaluation"

	ts.Equal(http.StatusUnauthorized, ts.send(http.MethodPost, evaluationEndpoint, ""),
		"expected 401 when the Direct Auth secret header is missing")

	status := ts.send(http.MethodPost, evaluationEndpoint, testutils.DirectAuthHeaderValue)
	ts.NotEqual(http.StatusNotFound, status, "AuthZEN access endpoint must stay registered")
	ts.NotEqual(http.StatusUnauthorized, status, "a valid Direct Auth secret must pass the gate")
}

// send issues a request with an empty JSON body and the given secret header value (a value of ""
// omits the header), and returns the status code.
func (ts *DirectAPIDisabledTestSuite) send(method, endpoint, secret string) int {
	req, err := http.NewRequest(method, testutils.TestServerURL+endpoint, bytes.NewReader([]byte("{}")))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set(testutils.DirectAuthHeaderName, secret)
	}

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	return resp.StatusCode
}
