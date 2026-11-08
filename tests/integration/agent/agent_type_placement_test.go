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
	// agentOUMismatchCode is what /agents returns for an agent outside its agent type's tree.
	agentOUMismatchCode = "AGT-1053"
	// agentTypeNotFoundCode is what /agents returns for an agent type that does not exist.
	agentTypeNotFoundCode  = "AGT-1054"
	unknownAgentTypeHandle = "placement-no-such-agents"

	placementTypeHandle      = "placement-agents"
	placementOtherTypeHandle = "placement-other-agents"
)

// AgentTypePlacementTestSuite covers the rule that an agent's organization unit is its agent type's
// organization unit or one of its descendants: on create, and on an update that moves the agent or changes its
// type. An update that changes neither leaves an agent placed before the rule existed editable.
//
// The tree: the agent type's unit, a child of it, and a sibling root holding a second agent type.
type AgentTypePlacementTestSuite struct {
	suite.Suite
	typeOUID      string
	childOUID     string
	siblingOUID   string
	spareOUID     string
	typeID        string
	otherTypeID   string
	createdAgents []string
}

func TestAgentTypePlacementTestSuite(t *testing.T) {
	suite.Run(t, new(AgentTypePlacementTestSuite))
}

func (ts *AgentTypePlacementTestSuite) SetupSuite() {
	ts.typeOUID = ts.createOU("agent-placement-type-ou", nil)
	ts.childOUID = ts.createOU("agent-placement-child-ou", &ts.typeOUID)
	ts.siblingOUID = ts.createOU("agent-placement-sibling-ou", nil)
	ts.spareOUID = ts.createOU("agent-placement-spare-ou", nil)
	ts.typeID = ts.createAgentType(placementTypeHandle, ts.typeOUID)
	ts.otherTypeID = ts.createAgentType(placementOtherTypeHandle, ts.siblingOUID)
}

func (ts *AgentTypePlacementTestSuite) TearDownSuite() {
	for _, id := range ts.createdAgents {
		ts.NoError(testutils.DeleteAgent(id), "teardown: delete agent %s", id)
	}
	for _, id := range []string{ts.typeID, ts.otherTypeID} {
		if id != "" {
			ts.NoError(testutils.DeleteAgentType(id), "teardown: delete agent type %s", id)
		}
	}
	for _, id := range []string{ts.childOUID, ts.typeOUID, ts.siblingOUID, ts.spareOUID} {
		if id != "" {
			ts.NoError(testutils.DeleteOrganizationUnit(id), "teardown: delete OU %s", id)
		}
	}
}

// createOU creates an organization unit for the suite.
func (ts *AgentTypePlacementTestSuite) createOU(handle string, parent *string) string {
	ts.T().Helper()
	id, err := testutils.CreateOrganizationUnit(
		testutils.OrganizationUnit{Handle: handle, Name: handle, Parent: parent})
	ts.Require().NoError(err)
	return id
}

// createAgentType creates an agent type for the suite.
func (ts *AgentTypePlacementTestSuite) createAgentType(handle, ouID string) string {
	ts.T().Helper()
	id, err := testutils.CreateNewAgentType(testutils.UserType{
		Handle: handle, DisplayName: handle, OUID: ouID,
		Schema: map[string]interface{}{"name": map[string]interface{}{"type": "string", "required": true}},
	})
	ts.Require().NoError(err)
	return id
}

// send issues a JSON request and returns the status and body.
func (ts *AgentTypePlacementTestSuite) send(method, path string, body interface{}) (int, []byte) {
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

// agentBody builds an agent request.
func agentBody(agentType, ouID, name string) map[string]interface{} {
	return map[string]interface{}{"type": agentType, "ouId": ouID, "attributes": map[string]interface{}{"name": name}}
}

// createAgent creates an agent that must be accepted and registers it for teardown.
func (ts *AgentTypePlacementTestSuite) createAgent(agentType, ouID, name string) string {
	ts.T().Helper()

	status, body := ts.send(http.MethodPost, "/agents", agentBody(agentType, ouID, name))
	ts.Require().Equal(http.StatusCreated, status, "create failed: %s", string(body))
	var created struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal(body, &created))
	ts.createdAgents = append(ts.createdAgents, created.ID)
	return created.ID
}

// requireMismatch asserts the placement refusal.
func (ts *AgentTypePlacementTestSuite) requireMismatch(status int, body []byte) {
	ts.T().Helper()
	ts.requireRefused(status, body, agentOUMismatchCode)
}

// requireRefused asserts a 400 with the given error code.
func (ts *AgentTypePlacementTestSuite) requireRefused(status int, body []byte, code string) {
	ts.T().Helper()

	ts.Require().Equal(http.StatusBadRequest, status, "body: %s", string(body))
	var errResp struct {
		Code string `json:"code"`
	}
	ts.Require().NoError(json.Unmarshal(body, &errResp))
	ts.Equal(code, errResp.Code, "body: %s", string(body))
}

func (ts *AgentTypePlacementTestSuite) TestCreateInTheAgentTypeOU() {
	ts.createAgent(placementTypeHandle, ts.typeOUID, "placement-in-type-ou")
}

func (ts *AgentTypePlacementTestSuite) TestCreateInAChildOfTheAgentTypeOU() {
	ts.createAgent(placementTypeHandle, ts.childOUID, "placement-in-child-ou")
}

func (ts *AgentTypePlacementTestSuite) TestCreateInASiblingOURejected() {
	status, body := ts.send(http.MethodPost, "/agents",
		agentBody(placementTypeHandle, ts.siblingOUID, "placement-in-sibling-ou"))
	ts.requireMismatch(status, body)
}

func (ts *AgentTypePlacementTestSuite) TestCreateWithAnUnknownAgentTypeRejected() {
	status, body := ts.send(http.MethodPost, "/agents",
		agentBody(unknownAgentTypeHandle, ts.typeOUID, "placement-unknown-type"))
	ts.requireRefused(status, body, agentTypeNotFoundCode)
}

func (ts *AgentTypePlacementTestSuite) TestUpdateSwitchingToAnUnknownAgentTypeRejected() {
	agentID := ts.createAgent(placementTypeHandle, ts.typeOUID, "placement-switch-to-unknown")

	status, body := ts.send(http.MethodPut, "/agents/"+agentID,
		agentBody(unknownAgentTypeHandle, ts.typeOUID, "placement-switch-to-unknown"))
	ts.requireRefused(status, body, agentTypeNotFoundCode)
}

func (ts *AgentTypePlacementTestSuite) TestUpdateMovingOutOfTheAgentTypeTreeRejected() {
	agentID := ts.createAgent(placementTypeHandle, ts.childOUID, "placement-move-out")

	status, body := ts.send(http.MethodPut, "/agents/"+agentID,
		agentBody(placementTypeHandle, ts.siblingOUID, "placement-move-out"))
	ts.requireMismatch(status, body)
}

func (ts *AgentTypePlacementTestSuite) TestUpdateSwitchingToAnAgentTypeOfAnotherTreeRejected() {
	agentID := ts.createAgent(placementTypeHandle, ts.typeOUID, "placement-switch-type")

	status, body := ts.send(http.MethodPut, "/agents/"+agentID,
		agentBody(placementOtherTypeHandle, ts.typeOUID, "placement-switch-type"))
	ts.requireMismatch(status, body)
}

// An agent placed correctly is left outside its type's tree when the type moves away. The agent stays
// editable as long as the update changes neither its organization unit nor its type.
func (ts *AgentTypePlacementTestSuite) TestUpdateOfAMisplacedAgentKeepingItsPlacementSucceeds() {
	agentID := ts.createAgent(placementTypeHandle, ts.typeOUID, "placement-misplaced")

	status, body := ts.send(http.MethodGet, "/agent-types/"+ts.typeID, nil)
	ts.Require().Equal(http.StatusOK, status)
	var agentType map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &agentType))
	moved := map[string]interface{}{
		"displayName": agentType["displayName"], "ouId": ts.spareOUID, "schema": agentType["schema"],
	}
	status, body = ts.send(http.MethodPut, "/agent-types/"+ts.typeID, moved)
	ts.Require().Equal(http.StatusOK, status, "moving the agent type failed: %s", string(body))
	defer func() {
		moved["ouId"] = ts.typeOUID
		status, body := ts.send(http.MethodPut, "/agent-types/"+ts.typeID, moved)
		ts.Equal(http.StatusOK, status, "restoring the agent type failed: %s", string(body))
	}()

	update := agentBody(placementTypeHandle, ts.typeOUID, "placement-misplaced-renamed")
	update["description"] = "still editable"
	status, body = ts.send(http.MethodPut, "/agents/"+agentID, update)
	ts.Equal(http.StatusOK, status, "an update keeping the placement must succeed: %s", string(body))
}

// An imported agent goes through the same placement check as /agents, so a document whose agent type is in
// another tree, or does not exist, is refused with the same reason and creates nothing.
func (ts *AgentTypePlacementTestSuite) TestImportRejectedLikeAgents() {
	cases := []struct {
		name, ouID, agentType, code string
	}{
		{name: "into a sibling organization unit", ouID: ts.siblingOUID, agentType: placementTypeHandle,
			code: agentOUMismatchCode},
		{name: "of an unknown agent type", ouID: ts.typeOUID, agentType: unknownAgentTypeHandle,
			code: agentTypeNotFoundCode},
	}
	for _, tc := range cases {
		ts.Run(tc.name, func() {
			content := fmt.Sprintf(
				"resource_type: agent\nouId: %s\ntype: %s\nattributes:\n  name: placement-imported\n",
				tc.ouID, tc.agentType)

			status, body := ts.send(http.MethodPost, "/import", agentImportRequest{
				Content: content,
				Options: agentImportOptions{Target: "runtime"},
			})
			ts.Require().Equal(http.StatusOK, status, "body: %s", string(body))
			var resp agentImportResponse
			ts.Require().NoError(json.Unmarshal(body, &resp))
			ts.Require().Len(resp.Results, 1, "body: %s", string(body))
			ts.Equal("failed", resp.Results[0].Status, "body: %s", string(body))
			ts.Equal(tc.code, resp.Results[0].Code, "body: %s", string(body))
			ts.Equal(0, resp.Summary.Imported)
		})
	}
}
