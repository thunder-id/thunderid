// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// agentInvalidAllowedAgentTypeCode is what /agents returns for an allowedAgentTypes list it refuses.
	agentInvalidAllowedAgentTypeCode = "AGT-1043"
	// allowedAgentTypesSuiteType is the agent type of the suite's organization unit, which its agents are of.
	allowedAgentTypesSuiteType = "allowed-agent-types-agents"
)

// AgentAllowedAgentTypesTestSuite covers the `*` entry of an agent's allowedAgentTypes: `["*"]` admits
// an agent of any agent type and is stored as given, and `*` combined with any other entry is refused
// on create and on update.
type AgentAllowedAgentTypesTestSuite struct {
	suite.Suite
	ouID        string
	agentTypeID string
	agentIDs    []string
}

func TestAgentAllowedAgentTypesTestSuite(t *testing.T) {
	suite.Run(t, new(AgentAllowedAgentTypesTestSuite))
}

func (ts *AgentAllowedAgentTypesTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "agent-allowed-agent-types-ou",
		Name:        "Agent Allowed Agent Types OU",
		Description: "Organization unit for agent allowedAgentTypes testing",
	})
	ts.Require().NoError(err)
	ts.ouID = ouID

	agentTypeID, err := testutils.CreateNewAgentType(testutils.UserType{
		Handle: allowedAgentTypesSuiteType, DisplayName: "Allowed Agent Types Agents", OUID: ouID,
		Schema: map[string]interface{}{"name": map[string]interface{}{"type": "string", "required": true}},
	})
	ts.Require().NoError(err)
	ts.agentTypeID = agentTypeID
}

func (ts *AgentAllowedAgentTypesTestSuite) TearDownSuite() {
	for _, id := range ts.agentIDs {
		if err := testutils.DeleteAgent(id); err != nil {
			ts.T().Logf("teardown: failed to delete agent %s: %v", id, err)
		}
	}
	if ts.agentTypeID != "" {
		ts.NoError(testutils.DeleteAgentType(ts.agentTypeID))
	}
	if ts.ouID != "" {
		ts.NoError(testutils.DeleteOrganizationUnit(ts.ouID))
	}
}

// agentBody builds an agent request of the suite's agent type with the given allowed agent types.
func (ts *AgentAllowedAgentTypesTestSuite) agentBody(name string, allowedAgentTypes []string) map[string]interface{} {
	return map[string]interface{}{
		"type":              allowedAgentTypesSuiteType,
		"ouId":              ts.ouID,
		"attributes":        map[string]interface{}{"name": name},
		"allowedAgentTypes": allowedAgentTypes,
	}
}

// send issues a JSON request and returns the status and body.
func (ts *AgentAllowedAgentTypesTestSuite) send(method, path string, body interface{}) (int, []byte) {
	ts.T().Helper()

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		ts.Require().NoError(err)
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, testServerURL+path, reader)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, respBody
}

// createAgent creates an agent and registers it for teardown.
func (ts *AgentAllowedAgentTypesTestSuite) createAgent(name string, allowedAgentTypes []string) string {
	ts.T().Helper()

	status, body := ts.send(http.MethodPost, "/agents", ts.agentBody(name, allowedAgentTypes))
	ts.Require().Equal(http.StatusCreated, status, "create failed: %s", string(body))

	var created struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal(body, &created))
	ts.agentIDs = append(ts.agentIDs, created.ID)
	return created.ID
}

// storedAllowedAgentTypes reads an agent's allowedAgentTypes back.
func (ts *AgentAllowedAgentTypesTestSuite) storedAllowedAgentTypes(agentID string) []string {
	ts.T().Helper()

	status, body := ts.send(http.MethodGet, "/agents/"+agentID, nil)
	ts.Require().Equal(http.StatusOK, status, "get failed: %s", string(body))

	var agent struct {
		AllowedAgentTypes []string `json:"allowedAgentTypes"`
	}
	ts.Require().NoError(json.Unmarshal(body, &agent))
	return agent.AllowedAgentTypes
}

// requireInvalidAllowedAgentType asserts a refusal of the allowedAgentTypes list.
func (ts *AgentAllowedAgentTypesTestSuite) requireInvalidAllowedAgentType(status int, body []byte) {
	ts.T().Helper()

	ts.Require().Equal(http.StatusBadRequest, status, "body: %s", string(body))
	var errResp struct {
		Code string `json:"code"`
	}
	ts.Require().NoError(json.Unmarshal(body, &errResp))
	ts.Equal(agentInvalidAllowedAgentTypeCode, errResp.Code, "body: %s", string(body))
}

func (ts *AgentAllowedAgentTypesTestSuite) TestAnyAgentTypeAccepted() {
	agentID := ts.createAgent("any-agent-type-agent", []string{"*"})

	ts.Equal([]string{"*"}, ts.storedAllowedAgentTypes(agentID))
}

func (ts *AgentAllowedAgentTypesTestSuite) TestAnyAgentTypeWithOtherEntriesRejectedOnCreate() {
	for i, allowed := range [][]string{{"*", "default"}, {"default", "*"}, {"*", "*"}} {
		ts.Run(fmt.Sprintf("%v", allowed), func() {
			status, body := ts.send(http.MethodPost, "/agents",
				ts.agentBody(fmt.Sprintf("mixed-agent-types-agent-%d", i), allowed))
			ts.requireInvalidAllowedAgentType(status, body)
		})
	}
}

func (ts *AgentAllowedAgentTypesTestSuite) TestAnyAgentTypeWithOtherEntriesRejectedOnUpdate() {
	agentID := ts.createAgent("any-agent-type-update-agent", []string{"*"})

	status, body := ts.send(http.MethodPut, "/agents/"+agentID,
		ts.agentBody("any-agent-type-update-agent", []string{"default", "*"}))

	ts.requireInvalidAllowedAgentType(status, body)
	ts.Equal([]string{"*"}, ts.storedAllowedAgentTypes(agentID), "a refused update must keep the stored list")
}
