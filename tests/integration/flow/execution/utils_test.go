// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// machineClient is a client_credentials principal requesting tokens for one audience.
type machineClient struct {
	id       string
	secret   string
	audience string
}

// grantedToken requires the issued token to carry the scope and to be active.
func grantedToken(s *suite.Suite, c machineClient, scope string) string {
	s.T().Helper()

	result, err := testutils.RequestClientCredentialsToken(c.id, c.secret, scope, c.audience)
	s.Require().NoError(err, "Token request failed")
	s.Require().Equal(http.StatusOK, result.StatusCode, "Token response: %s", string(result.Body))
	s.Require().NotEmpty(result.Token.AccessToken, "%s must obtain a token for %s on %s", c.id, scope, c.audience)
	s.Require().Contains(strings.Fields(result.Token.Scope), scope,
		"The token issued to %s must carry the scope %s", c.id, scope)
	s.Require().True(tokenIsActive(s, c, result.Token.AccessToken),
		"The token issued to %s for %s must be active when issued", c.id, scope)
	return result.Token.AccessToken
}

func tokenIsActive(s *suite.Suite, c machineClient, token string) bool {
	s.T().Helper()

	active, err := testutils.IntrospectToken(token, c.id, c.secret)
	s.Require().NoError(err, "Introspection request failed")
	return active
}

// createRole creates the role and records its id in created for teardown.
func createRole(s *suite.Suite, created *[]string, ouID, resourceServerID, name string,
	permissions []string, assignments ...testutils.Assignment) string {
	s.T().Helper()

	role := testutils.Role{
		Name:        name,
		Description: "Role created for flow execution testing",
		OUID:        ouID,
		Assignments: assignments,
	}
	if len(permissions) > 0 {
		role.Permissions = []testutils.ResourcePermissions{
			{ResourceServerID: resourceServerID, Permissions: permissions},
		}
	}

	roleID, err := testutils.CreateRole(role)
	s.Require().NoError(err, "Failed to create test role")
	*created = append(*created, roleID)
	return roleID
}

// permissionSet encodes permissions on one resource server as the role permission flow input.
func permissionSet(s *suite.Suite, resourceServerID string, permissions ...string) string {
	s.T().Helper()

	encoded, err := json.Marshal([]map[string]interface{}{
		{"resourceServerId": resourceServerID, "permissions": permissions},
	})
	s.Require().NoError(err)
	return string(encoded)
}

func executeAsAdmin(s *suite.Suite, flowID string, inputs map[string]string) *common.FlowStep {
	s.T().Helper()

	result, err := common.ExecuteAdministrationFlow(flowID, inputs, true)
	s.Require().NoError(err, "Failed to execute administration flow %s", flowID)
	s.Require().Equal(http.StatusOK, result.StatusCode, "Flow error response: %+v", result.Error)
	return result.Step
}

func requireCompleted(s *suite.Suite, step *common.FlowStep) {
	s.T().Helper()
	s.Require().Equal("COMPLETE", step.FlowStatus, "Flow response: %+v", step)
}
