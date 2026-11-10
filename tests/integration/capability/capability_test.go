// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package capability

import (
	"encoding/json"
	"net/http"
	"slices"
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

// importOnlyWrites are the writes to resources that apply to the whole deployment, which a data plane takes
// only from an import with the management API key, so no token is allowed them.
var importOnlyWrites = map[string][]string{
	"design":       {"create", "update", "delete"},
	"i18n":         {"create", "update", "delete"},
	"authzdetail":  {"create", "update", "delete"},
	"serverconfig": {"update"},
	"ou":           {"createRoot"},
}

// The root admin is allowed every action except the writes a data plane takes only from an import.
func (ts *CapabilityAPITestSuite) TestRootAdminGetsEveryActionButImportOnlyWrites() {
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
			ts.Equal(!slices.Contains(importOnlyWrites[resource], action), allowed, "%s:%s", resource, action)
		}
	}
	for resource, actions := range importOnlyWrites {
		for _, action := range actions {
			ts.Contains(body.Resources[resource], action, "%s:%s is reported", resource, action)
		}
	}
	ts.Equal(map[string]bool{"view": true, "update": false}, body.Resources["serverconfig"])
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
