// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package cors

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// deploymentOrigin is configured via deployment.yaml's cors.allowedOrigins section for the
	// duration of TestMutableModeSeedsAndMergesDeploymentOrigins.
	deploymentOrigin = "https://deployment-config.example.com"
	// adminAddedOrigin is added at runtime through the server-config API while the server runs in
	// mutable mode, to verify it survives a restart alongside deploymentOrigin.
	adminAddedOrigin = "https://mutable-runtime.example.com"
	// invalidOrigin is a wildcard literal, which origin validation rejects.
	invalidOrigin = "*"
)

// CORSDeploymentConfigTestSuite validates the deployment.yaml cors.allowedOrigins section against the
// mutable server_config.store mode: deployment.yaml origins are always merged into the writable
// (database) layer on startup, without clobbering origins added at runtime through the server-config
// API. It also verifies that startup fails when the section cannot be applied. A successful
// declarative/composite-mode seed of the read-only layer is covered by unit tests in the serverconfig
// package instead: the shared integration server already runs in composite mode with a
// server_configs/cors.yaml declarative resource fixture for the cors section (see
// CORSIntegrationTestSuite), and the two read-only sources cannot coexist for the same section.
type CORSDeploymentConfigTestSuite struct {
	suite.Suite
	plainClient *http.Client
}

func TestCORSDeploymentConfigTestSuite(t *testing.T) {
	suite.Run(t, new(CORSDeploymentConfigTestSuite))
}

func (suite *CORSDeploymentConfigTestSuite) SetupSuite() {
	suite.plainClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

// TestMutableModeSeedsAndMergesDeploymentOrigins switches the shared server to mutable store mode, then
// walks:
//  0. A boot without a deployment.yaml cors section leaves the writable layer untouched.
//  1. A boot with a deployment.yaml cors section seeds deploymentOrigin into the empty writable layer.
//  2. A PUT that replaces the writable layer with a different origin drops deploymentOrigin, matching
//     PUT's full-replace semantics (mutable mode has no read-only layer to fall back on).
//  3. A restart re-merges deploymentOrigin into the persisted writable layer, so it is always honoured,
//     while the admin-added origin from step 2 survives alongside it.
//
// Restores composite store mode, clears the deployment.yaml cors section, and clears the writable layer
// afterwards so later packages see the original server state.
func (suite *CORSDeploymentConfigTestSuite) TestMutableModeSeedsAndMergesDeploymentOrigins() {
	suite.Require().NoError(testutils.PatchDeploymentConfig(corsPatch("mutable")),
		"Failed to patch deployment.yaml for mutable cors mode")
	suite.Require().NoError(testutils.RestartServer(), "Failed to restart server in mutable cors mode")
	defer func() {
		suite.Require().NoError(testutils.PatchDeploymentConfig(compositeCORSPatch()),
			"Failed to restore deployment.yaml to composite cors mode")
		suite.Require().NoError(testutils.RestartServer(), "Failed to restart server after config restore")
		suite.Require().NoError(testutils.ObtainAdminAccessToken(), "Failed to re-obtain admin token")
		suite.putCORS(testutils.GetHTTPClient(), `[]`)
	}()
	suite.Require().NoError(testutils.ObtainAdminAccessToken(), "Failed to obtain admin token")

	// (0) Without a deployment.yaml cors section, mutable mode seeds nothing into the writable layer.
	suite.Empty(suite.allowedOrigin(deploymentOrigin))

	suite.Require().NoError(testutils.PatchDeploymentConfig(mutableCORSPatch(deploymentOrigin)),
		"Failed to patch deployment.yaml with a cors section")
	suite.Require().NoError(testutils.RestartServer(), "Failed to restart server with a cors section")
	suite.Require().NoError(testutils.ObtainAdminAccessToken(), "Failed to re-obtain admin token")

	// (1) Fresh boot: the empty writable layer is seeded with deploymentOrigin.
	suite.Equal(deploymentOrigin, suite.allowedOrigin(deploymentOrigin))
	suite.Empty(suite.allowedOrigin(adminAddedOrigin))

	// (2) PUT fully replaces the writable layer; deploymentOrigin is dropped until the next startup.
	adminClient := testutils.GetHTTPClient()
	suite.putCORS(adminClient, fmt.Sprintf(`[%q]`, adminAddedOrigin))
	suite.Equal(adminAddedOrigin, suite.allowedOrigin(adminAddedOrigin))
	suite.Empty(suite.allowedOrigin(deploymentOrigin))

	// (3) A restart re-merges deploymentOrigin into the persisted writable layer: both origins are
	// allowed afterwards.
	suite.Require().NoError(testutils.RestartServer(), "Failed to restart server to verify origin merge")
	suite.Require().NoError(testutils.ObtainAdminAccessToken(), "Failed to re-obtain admin token after restart")

	suite.Equal(deploymentOrigin, suite.allowedOrigin(deploymentOrigin))
	suite.Equal(adminAddedOrigin, suite.allowedOrigin(adminAddedOrigin))
}

// TestStartupRejectsInvalidDeploymentOrigins verifies the server refuses to start when the
// deployment.yaml cors section cannot be applied:
//  1. An invalid origin in composite mode, rejected before it reaches the read-only layer.
//  2. An invalid origin in mutable mode, rejected before it is merged into the writable layer.
//  3. A valid origin in composite mode that conflicts with the server_configs/cors.yaml declarative
//     resource fixture, rejected by the duplicate check once the declarative files load.
//
// Restores composite store mode and clears the deployment.yaml cors section afterwards.
func (suite *CORSDeploymentConfigTestSuite) TestStartupRejectsInvalidDeploymentOrigins() {
	defer func() {
		suite.Require().NoError(testutils.PatchDeploymentConfig(compositeCORSPatch()),
			"Failed to restore deployment.yaml to composite cors mode")
		suite.Require().NoError(testutils.RestartServer(), "Failed to restart server after config restore")
		suite.Require().NoError(testutils.ObtainAdminAccessToken(), "Failed to re-obtain admin token")
	}()

	cases := []struct {
		name  string
		patch map[string]interface{}
	}{
		{"composite mode with an invalid origin", corsPatch("composite", invalidOrigin)},
		{"mutable mode with an invalid origin", corsPatch("mutable", invalidOrigin)},
		{"composite mode with an origin conflicting with cors.yaml", corsPatch("composite", deploymentOrigin)},
	}
	for _, tc := range cases {
		suite.Require().NoError(testutils.PatchDeploymentConfig(tc.patch),
			"Failed to patch deployment.yaml for %s", tc.name)
		suite.Require().NoError(testutils.RestartServerExpectingStartupFailure(30*time.Second),
			"Expected startup to fail for %s", tc.name)
	}
}

// corsPatch sets server_config.store to store and cors.allowedOrigins to origins.
func corsPatch(store string, origins ...string) map[string]interface{} {
	if origins == nil {
		origins = []string{}
	}
	return map[string]interface{}{
		"server_config": map[string]interface{}{"store": store},
		"cors":          map[string]interface{}{"allowedOrigins": origins},
	}
}

// mutableCORSPatch switches server_config.store to mutable and sets cors.allowedOrigins to origin.
func mutableCORSPatch(origin string) map[string]interface{} {
	return corsPatch("mutable", origin)
}

// compositeCORSPatch restores server_config.store to composite (the fixture default) and clears
// deployment.yaml's cors section, a no-op for the seeding logic, so it no longer conflicts with the
// server_configs/cors.yaml declarative resource fixture once composite mode's read-only layer loads.
func compositeCORSPatch() map[string]interface{} {
	return corsPatch("composite")
}

// allowedOrigin sends a CORS preflight for origin against a CORS-enabled, auth-free route and returns
// the echoed Access-Control-Allow-Origin (empty when rejected).
func (suite *CORSDeploymentConfigTestSuite) allowedOrigin(origin string) string {
	req, err := http.NewRequest(http.MethodOptions, corsConfigURL, nil)
	suite.Require().NoError(err)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)

	resp, err := suite.plainClient.Do(req)
	suite.Require().NoError(err)
	defer closeBodyQuietly(suite.T(), resp.Body)

	return resp.Header.Get("Access-Control-Allow-Origin")
}

// putCORS sets the cors writable layer to the given allowed-origins array using client and requires a
// 200 response.
func (suite *CORSDeploymentConfigTestSuite) putCORS(client *http.Client, allowedOrigins string) {
	body := `{"allowedOrigins":` + allowedOrigins + `}`
	req, err := http.NewRequest(http.MethodPut, corsConfigURL, strings.NewReader(body))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer closeBodyQuietly(suite.T(), resp.Body)

	suite.Require().Equal(http.StatusOK, resp.StatusCode)
}
