// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

var rolesTestOU = testutils.OrganizationUnit{
	Handle:      "agent-roles-display-ou",
	Name:        "Agent Roles Display OU",
	Description: "Organization unit for the agent roles and display listing tests",
	Parent:      nil,
}

// rolesTestAgentTypeName is the shipped agent type this suite stores its agent as. The suite does
// not declare a type of its own: testutils.CreateAgentType always writes the singleton "default"
// type, so declaring one would rewrite the schema that the agent type API suite asserts on. Using
// the type as it ships keeps this suite out of that shared state, at the cost of not being able to
// store a custom attribute, which is why the list tests locate the agent by id rather than by
// filtering on one.
const rolesTestAgentTypeName = "default"

// AgentRolesDisplayTestSuite covers GET /agents/{id}/roles, which reports the roles an agent holds
// directly and through its groups, and the include=display variants of the agent get and list
// endpoints, which resolve the agent's OU handle.
type AgentRolesDisplayTestSuite struct {
	suite.Suite
	ouID       string
	agentID    string
	directRole string
	groupRole  string
	roleIDs    []string
	groupID    string
}

func TestAgentRolesDisplayTestSuite(t *testing.T) {
	suite.Run(t, new(AgentRolesDisplayTestSuite))
}

func (ts *AgentRolesDisplayTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(rolesTestOU)
	ts.Require().NoError(err, "Failed to create the test organization unit")
	ts.ouID = ouID

	agentID, err := createAgent(Agent{
		OUID:        ts.ouID,
		Type:        rolesTestAgentTypeName,
		Name:        "agent-roles-display-agent",
		Description: "Agent used by the roles and display listing tests",
	})
	ts.Require().NoError(err, "Failed to create the test agent")
	ts.agentID = agentID

	// A role assigned to the agent directly.
	ts.directRole = "agent-roles-display-direct"
	directRoleID, err := testutils.CreateRole(testutils.Role{
		Name: ts.directRole,
		OUID: ts.ouID,
		Assignments: []testutils.Assignment{
			{ID: ts.agentID, Type: "agent"},
		},
	})
	ts.Require().NoError(err, "Failed to create the directly assigned role")
	ts.roleIDs = append(ts.roleIDs, directRoleID)

	// A role the agent inherits through a group it belongs to.
	groupID, err := testutils.CreateGroup(testutils.Group{
		Name:    "agent-roles-display-group",
		OUID:    ts.ouID,
		Members: []testutils.Member{{Id: ts.agentID, Type: "agent"}},
	})
	ts.Require().NoError(err, "Failed to create the test group")
	ts.groupID = groupID

	ts.groupRole = "agent-roles-display-group-role"
	groupRoleID, err := testutils.CreateRole(testutils.Role{
		Name: ts.groupRole,
		OUID: ts.ouID,
		Assignments: []testutils.Assignment{
			{ID: ts.groupID, Type: "group"},
		},
	})
	ts.Require().NoError(err, "Failed to create the group assigned role")
	ts.roleIDs = append(ts.roleIDs, groupRoleID)
}

func (ts *AgentRolesDisplayTestSuite) TearDownSuite() {
	for i := len(ts.roleIDs) - 1; i >= 0; i-- {
		if err := testutils.DeleteRole(ts.roleIDs[i]); err != nil {
			ts.T().Logf("Failed to delete role %s during teardown: %v", ts.roleIDs[i], err)
		}
	}
	if ts.groupID != "" {
		if err := testutils.DeleteGroup(ts.groupID); err != nil {
			ts.T().Logf("Failed to delete group during teardown: %v", err)
		}
	}
	if ts.agentID != "" {
		if err := deleteAgent(ts.agentID); err != nil {
			ts.T().Logf("Failed to delete agent during teardown: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete organization unit during teardown: %v", err)
		}
	}
}

// --- helpers ---

func (ts *AgentRolesDisplayTestSuite) getRoles(query string) (int, *AgentRoleListResponse, []byte) {
	requestURL := fmt.Sprintf("%s%s/%s/roles", testServerURL, agentBasePath, ts.agentID)
	if query != "" {
		requestURL += "?" + query
	}
	return ts.getRolesFromURL(requestURL)
}

func (ts *AgentRolesDisplayTestSuite) getRolesFromURL(requestURL string) (int, *AgentRoleListResponse, []byte) {
	resp, err := doGet(requestURL)
	ts.Require().NoError(err, "Failed to send the agent roles request")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "Failed to read the agent roles response")
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil, body
	}
	var roleList AgentRoleListResponse
	ts.Require().NoError(json.Unmarshal(body, &roleList),
		"Failed to parse the agent roles response: %s", string(body))
	return resp.StatusCode, &roleList, body
}

func (ts *AgentRolesDisplayTestSuite) assertErrorCode(body []byte, expectedCode string) {
	var errResp struct {
		Code    string `json:"code"`
		Message struct {
			DefaultValue string `json:"defaultValue"`
		} `json:"message"`
	}
	ts.Require().NoError(json.Unmarshal(body, &errResp), "Failed to parse the error response: %s", string(body))
	ts.Equal(expectedCode, errResp.Code, "Unexpected error code for response: %s", string(body))
}

// --- tests ---

// TestAgentRolesIncludesDirectAndInheritedRoles asserts that the roles endpoint reports both the
// role assigned to the agent itself and the role it inherits from its group membership.
func (ts *AgentRolesDisplayTestSuite) TestAgentRolesIncludesDirectAndInheritedRoles() {
	status, roleList, body := ts.getRoles("")
	ts.Require().Equal(http.StatusOK, status, "Listing agent roles should return 200: %s", string(body))

	ts.Contains(roleList.Roles, ts.directRole, "A role assigned to the agent must be reported")
	ts.Contains(roleList.Roles, ts.groupRole, "A role assigned to the agent's group must be reported")
	ts.Equal(len(roleList.Roles), roleList.Count, "Count must match the number of returned roles")
	ts.Equal(1, roleList.StartIndex, "The default page starts at index 1")
	ts.GreaterOrEqual(roleList.TotalResults, 2,
		"The agent holds at least its direct role and its inherited role")
}

// TestAgentRolesPagination asserts the paging contract of the roles endpoint, including the empty
// page returned for an offset past the end of the result set.
func (ts *AgentRolesDisplayTestSuite) TestAgentRolesPagination() {
	status, firstPage, body := ts.getRoles("limit=1&offset=0")
	ts.Require().Equal(http.StatusOK, status, "Listing agent roles should return 200: %s", string(body))
	ts.Equal(1, firstPage.Count, "A limit of 1 must return a single role")
	ts.Equal(1, firstPage.StartIndex, "The first page starts at index 1")
	ts.GreaterOrEqual(firstPage.TotalResults, 2, "The total count must cover every role of the agent")
	ts.NotEmpty(firstPage.Links, "A paged response must carry pagination links")

	status, secondPage, body := ts.getRoles("limit=1&offset=1")
	ts.Require().Equal(http.StatusOK, status, "Listing agent roles should return 200: %s", string(body))
	ts.Equal(1, secondPage.Count, "The second page must return the next role")
	ts.Equal(2, secondPage.StartIndex, "The second page starts at index 2")
	ts.NotEqual(firstPage.Roles[0], secondPage.Roles[0], "Consecutive pages must not repeat a role")

	status, emptyPage, body := ts.getRoles("limit=1&offset=100")
	ts.Require().Equal(http.StatusOK, status,
		"An offset past the end of the result set is still a valid page: %s", string(body))
	ts.Empty(emptyPage.Roles, "An offset past the end must return no roles")
	ts.Equal(0, emptyPage.Count, "An empty page reports a count of zero")
	ts.GreaterOrEqual(emptyPage.TotalResults, 2, "The total count is independent of the requested page")
}

// TestAgentRolesInvalidPagination asserts that the roles endpoint rejects out of range pagination
// parameters with the documented error codes.
func (ts *AgentRolesDisplayTestSuite) TestAgentRolesInvalidPagination() {
	testCases := []struct {
		name         string
		query        string
		expectedCode string
	}{
		{name: "non numeric limit", query: "limit=abc", expectedCode: "AGT-1011"},
		{name: "zero limit", query: "limit=0", expectedCode: "AGT-1011"},
		{name: "limit above the maximum", query: "limit=101", expectedCode: "AGT-1011"},
		{name: "negative offset", query: "offset=-1", expectedCode: "AGT-1012"},
		{name: "non numeric offset", query: "offset=abc", expectedCode: "AGT-1012"},
	}

	for _, tc := range testCases {
		ts.Run(tc.name, func() {
			status, _, body := ts.getRoles(tc.query)
			ts.Equal(http.StatusBadRequest, status, "Invalid pagination should be rejected with 400")
			ts.assertErrorCode(body, tc.expectedCode)
		})
	}
}

// TestAgentRolesUnknownAgent asserts that the roles endpoint reports a missing agent as not found,
// both for an identifier that does not exist and for one that belongs to a non-agent entity.
func (ts *AgentRolesDisplayTestSuite) TestAgentRolesUnknownAgent() {
	status, _, body := ts.getRolesFromURL(fmt.Sprintf("%s%s/%s/roles", testServerURL, agentBasePath,
		"00000000-0000-0000-0000-000000000000"))
	ts.Equal(http.StatusNotFound, status, "An unknown agent id should return 404")
	ts.assertErrorCode(body, "AGT-1004")

	// A group is an entity of another category, so its id must not resolve as an agent.
	status, _, body = ts.getRolesFromURL(fmt.Sprintf("%s%s/%s/roles", testServerURL, agentBasePath, ts.groupID))
	ts.Equal(http.StatusNotFound, status, "An id of another entity category should return 404")
	ts.assertErrorCode(body, "AGT-1004")
}

// TestAgentGetWithDisplayResolvesOUHandle asserts that GET /agents/{id} only resolves the OU handle
// when display attributes are requested.
func (ts *AgentRolesDisplayTestSuite) TestAgentGetWithDisplayResolvesOUHandle() {
	resp, err := doGet(fmt.Sprintf("%s%s/%s", testServerURL, agentBasePath, ts.agentID))
	ts.Require().NoError(err, "Failed to send the agent get request")
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	ts.Require().NoError(err, "Failed to read the agent get response")
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "Getting an agent should return 200: %s", string(body))

	var plain Agent
	ts.Require().NoError(json.Unmarshal(body, &plain), "Failed to parse the agent get response")
	ts.Empty(plain.OUHandle, "The OU handle must not be resolved without include=display")

	resp, err = doGet(fmt.Sprintf("%s%s/%s?include=display", testServerURL, agentBasePath, ts.agentID))
	ts.Require().NoError(err, "Failed to send the agent get request with display")
	body, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	ts.Require().NoError(err, "Failed to read the agent get response with display")
	ts.Require().Equal(http.StatusOK, resp.StatusCode,
		"Getting an agent with display should return 200: %s", string(body))

	var withDisplay Agent
	ts.Require().NoError(json.Unmarshal(body, &withDisplay), "Failed to parse the agent get response")
	ts.Equal(rolesTestOU.Handle, withDisplay.OUHandle,
		"include=display must resolve the handle of the agent's organization unit")
}

// TestAgentListWithDisplayResolvesOUHandles asserts that the batch OU handle resolution of the agent
// list endpoint runs only when display attributes are requested.
func (ts *AgentRolesDisplayTestSuite) TestAgentListWithDisplayResolvesOUHandles() {
	findAgent := func(list *AgentListResponse) *Agent {
		for i := range list.Agents {
			if list.Agents[i].ID == ts.agentID {
				return &list.Agents[i]
			}
		}
		return nil
	}

	listAgents := func(query url.Values) *AgentListResponse {
		resp, err := doGet(fmt.Sprintf("%s%s?%s", testServerURL, agentBasePath, query.Encode()))
		ts.Require().NoError(err, "Failed to send the agent list request")
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		ts.Require().NoError(err, "Failed to read the agent list response")
		ts.Require().Equal(http.StatusOK, resp.StatusCode,
			"Listing agents should return 200: %s", string(body))
		var list AgentListResponse
		ts.Require().NoError(json.Unmarshal(body, &list),
			"Failed to parse the agent list response: %s", string(body))
		return &list
	}

	// Page through the listing until this suite's agent is found, rather than filtering on an
	// attribute. Storing an attribute would mean declaring a schema on the shared agent type.
	locate := func(include bool) *Agent {
		for offset := 0; offset < 500; offset += 100 {
			query := url.Values{}
			query.Set("limit", "100")
			query.Set("offset", strconv.Itoa(offset))
			if include {
				query.Set("include", "display")
			}
			list := listAgents(query)
			if found := findAgent(list); found != nil {
				return found
			}
			if offset+len(list.Agents) >= list.TotalResults {
				break
			}
		}
		return nil
	}

	plainAgent := locate(false)
	ts.Require().NotNil(plainAgent, "The test agent must be returned by the list")
	ts.Empty(plainAgent.OUHandle, "The OU handle must not be resolved without include=display")

	displayAgent := locate(true)
	ts.Require().NotNil(displayAgent, "The test agent must be returned by the list with display")
	ts.Equal(rolesTestOU.Handle, displayAgent.OUHandle,
		"include=display must resolve the OU handle of every listed agent")
}
