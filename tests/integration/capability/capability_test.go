// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package capability

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

type CapabilityAPITestSuite struct {
	suite.Suite
}

func TestCapabilityAPITestSuite(t *testing.T) {
	suite.Run(t, new(CapabilityAPITestSuite))
}

// get calls GET /capabilities with client.
func (ts *CapabilityAPITestSuite) get(client *http.Client) *http.Response {
	req, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+"/capabilities", nil)
	ts.Require().NoError(err)
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	return resp
}

func (ts *CapabilityAPITestSuite) TestRootAdminGetsEveryAction() {
	resp := ts.get(testutils.GetHTTPClient())
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var body testutils.CapabilitiesResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&body))

	ts.Equal("ou", body.Scope.Type)
	ts.NotEmpty(body.Scope.OUID)
	ts.True(body.Root.Manage)
	ts.NotEmpty(body.Resources)
	for resource, actions := range body.Resources {
		ts.NotEmpty(actions, resource)
		for action, allowed := range actions {
			ts.True(allowed, "%s:%s", resource, action)
		}
	}
	ts.Equal(map[string]bool{"view": true, "update": true}, body.Resources["serverconfig"])
}

func (ts *CapabilityAPITestSuite) TestUnauthenticatedCallerIsRejected() {
	resp := ts.get(testutils.GetNoRedirectHTTPClient())
	defer resp.Body.Close()
	ts.Equal(http.StatusUnauthorized, resp.StatusCode)
}

func (ts *CapabilityAPITestSuite) TestManagementAPIKeyIsRejected() {
	resp := ts.get(testutils.GetManagementAPIKeyHTTPClient())
	defer resp.Body.Close()
	ts.Equal(http.StatusUnauthorized, resp.StatusCode)
}
