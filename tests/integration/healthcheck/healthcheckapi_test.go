// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

var testServerURL = testutils.TestServerURL

type HealthCheckAPITestSuite struct {
	suite.Suite
}

func TestHealthCheckAPITestSuite(t *testing.T) {
	suite.Run(t, new(HealthCheckAPITestSuite))
}

// TestLivenessCheck tests the liveness endpoint.
func (ts *HealthCheckAPITestSuite) TestLivenessCheck() {
	req, err := http.NewRequest("GET", testServerURL+"/health/liveness", nil)
	ts.Require().NoError(err)

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode)
}

// TestReadinessCheck tests the readiness endpoint.
func (ts *HealthCheckAPITestSuite) TestReadinessCheck() {
	req, err := http.NewRequest("GET", testServerURL+"/health/readiness", nil)
	ts.Require().NoError(err)

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var healthStatus map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&healthStatus)
	ts.Require().NoError(err)
	ts.Require().Equal("UP", healthStatus["status"])
	ts.Require().NotEmpty(healthStatus["serviceStatus"])
}
