// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	testServerURL = "https://localhost:8095"
	agentBasePath = "/agents"

	// Handles of the two isolated auth flows used by the flow-handle resolution tests.
	handleAuthFlowHandle1 = "agent-api-handle-flow-1"
	handleAuthFlowHandle2 = "agent-api-handle-flow-2"
)

var (
	testOU = testutils.OrganizationUnit{
		Handle:      "test_agent_ou",
		Name:        "Test Organization Unit for Agents",
		Description: "Organization unit created for agent API testing",
		Parent:      nil,
	}

	// agentSchema is reused from the entity type subsystem. Agents need a type that maps
	// to a user type so attribute validation and credential extraction work correctly.
	agentSchema = testutils.UserType{
		Handle:           "default",
		DisplayName:      "Default",
		SystemAttributes: &testutils.UserTypeSystemAttributes{Display: "name"},
		Schema: map[string]interface{}{
			"name":        map[string]interface{}{"type": "string", "required": true, "unique": true},
			"description": map[string]interface{}{"type": "string"},
		},
	}

	// entityOnlyAgent has no inbound auth fields — only the entity row is created.
	entityOnlyAgent = Agent{
		Type:        "default",
		Attributes:  agentAttrs("entity-only-agent", nil),
		Description: "Agent with entity row only",
	}

	// inboundAgent has an auth flow ID — entity + inbound client rows are created.
	inboundAgent = Agent{
		Type:        "default",
		Attributes:  agentAttrs("inbound-agent", nil),
		Description: "Agent with inbound auth profile",
	}

	// oauthAgent has an inbound auth config with CC grant — entity + inbound + OAuth profile rows.
	oauthAgent = Agent{
		Type:        "default",
		Attributes:  agentAttrs("oauth-agent", nil),
		Description: "Agent with OAuth client credentials profile",
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				Config: &OAuthAgentConfig{
					ClientID:                "agent_oauth_client",
					ClientSecret:            "agent_oauth_secret",
					GrantTypes:              []string{"client_credentials"},
					TokenEndpointAuthMethod: "client_secret_basic",
					PublicClient:            false,
				},
			},
		},
	}
)

var (
	testOUID          string
	defaultAuthFlowID string

	// IDs of the isolated auth flows referenced by handle in the resolution tests.
	handleAuthFlowID1 string
	handleAuthFlowID2 string

	// IDs set during SetupSuite for the primary agent used across multiple tests.
	createdAgentID   string
	createdAgentName string
)

// AgentAPITestSuite covers the full agent CRUD lifecycle, three creation modes,
// OAuth CC token issuance, tree-path endpoints, group membership, and error paths.
type AgentAPITestSuite struct {
	suite.Suite
	agentTypeSnapshot *testutils.AgentTypeSnapshot
}

func TestAgentAPITestSuite(t *testing.T) {
	suite.Run(t, new(AgentAPITestSuite))
}

// SetupSuite creates the shared OU, user type, auth flow, and a primary entity-only agent
// that most CRUD tests operate against.
func (ts *AgentAPITestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	testOUID = ouID

	// The `default` agent type is a singleton shared with every other suite. Snapshot it before
	// pointing it at this suite's OU, so teardown can put it back before that OU is deleted.
	snapshot, err := testutils.SnapshotAgentType()
	ts.Require().NoError(err, "Failed to snapshot the default agent type")
	ts.agentTypeSnapshot = snapshot

	agentSchema.OUID = testOUID
	_, err = testutils.CreateAgentType(agentSchema)
	ts.Require().NoError(err, "Failed to create agent schema (user type)")

	defaultAuthFlowID, err = testutils.GetFlowIDByHandle("default-flow", "AUTHENTICATION")
	ts.Require().NoError(err, "Failed to get default auth flow ID")

	handleAuthFlowID1, err = testutils.CreateIsolatedAuthFlow(handleAuthFlowHandle1)
	ts.Require().NoError(err, "Failed to create the first isolated auth flow")
	handleAuthFlowID2, err = testutils.CreateIsolatedAuthFlow(handleAuthFlowHandle2)
	ts.Require().NoError(err, "Failed to create the second isolated auth flow")

	// Create the primary agent used by list/get/update/groups tests.
	primaryAgent := entityOnlyAgent
	primaryAgent.OUID = testOUID
	id, err := createAgent(primaryAgent)
	ts.Require().NoError(err, "Failed to create primary entity-only agent")
	createdAgentID = id
	createdAgentName = entityOnlyAgent.AttrName()
}

// TearDownSuite removes all resources created during the suite.
func (ts *AgentAPITestSuite) TearDownSuite() {
	if createdAgentID != "" {
		if err := deleteAgent(createdAgentID); err != nil {
			ts.T().Logf("Failed to delete primary agent during teardown: %v", err)
		}
	}
	for _, flowID := range []string{handleAuthFlowID1, handleAuthFlowID2} {
		if flowID == "" {
			continue
		}
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete isolated auth flow %s during teardown: %v", flowID, err)
		}
	}
	// Restore the shared agent type before deleting the OU it points at, or the singleton is left
	// referencing a deleted OU and a later suite's restore fails. A failed restore keeps the OU:
	// leaking one is cheaper than every later suite inheriting a dangling reference.
	agentTypeRestored := true
	if ts.agentTypeSnapshot != nil {
		if err := testutils.RestoreAgentType(ts.agentTypeSnapshot); err != nil {
			ts.T().Errorf("teardown: failed to restore the default agent type: %v", err)
			agentTypeRestored = false
		}
	}
	if testOUID != "" && agentTypeRestored {
		if err := testutils.DeleteOrganizationUnit(testOUID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

// --- CRUD lifecycle ---

func (ts *AgentAPITestSuite) TestAgentListing() {
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("GET", testServerURL+agentBasePath, nil)
	ts.Require().NoError(err)

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	var listResp AgentListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))
	ts.Assert().GreaterOrEqual(listResp.TotalResults, 1)
	ts.Assert().Equal(1, listResp.StartIndex)
	ts.Assert().Equal(len(listResp.Agents), listResp.Count)

	found := false
	for _, a := range listResp.Agents {
		if a.ID == createdAgentID {
			found = true
			ts.Assert().Equal(createdAgentName, a.AttrName())
			break
		}
	}
	ts.Assert().True(found, "Primary agent not found in list response")
}

func (ts *AgentAPITestSuite) TestAgentPagination() {
	client := testutils.GetHTTPClient()

	// limit=1 must return exactly one agent
	req, err := http.NewRequest("GET", testServerURL+agentBasePath+"?limit=1&offset=0", nil)
	ts.Require().NoError(err)
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var page1 AgentListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&page1))
	ts.Assert().Equal(1, page1.Count)
	ts.Assert().Equal(1, page1.StartIndex)

	// limit=invalid must return 400
	req2, err := http.NewRequest("GET", testServerURL+agentBasePath+"?limit=invalid", nil)
	ts.Require().NoError(err)
	resp2, err := client.Do(req2)
	ts.Require().NoError(err)
	defer resp2.Body.Close()
	ts.Assert().Equal(http.StatusBadRequest, resp2.StatusCode)
}

func (ts *AgentAPITestSuite) TestAgentGetByID() {
	ts.Require().NotEmpty(createdAgentID, "agent ID not available")

	resp, err := doGet(testServerURL + agentBasePath + "/" + createdAgentID)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	var agent Agent
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&agent))
	ts.Assert().Equal(createdAgentID, agent.ID)
	ts.Assert().Equal(createdAgentName, agent.AttrName())
	ts.Assert().Equal(testOUID, agent.OUID)
	ts.Assert().Equal("default", agent.Type)
	// GET must never return clientSecret
	if len(agent.InboundAuthConfig) > 0 && agent.InboundAuthConfig[0].Config != nil {
		ts.Assert().Empty(agent.InboundAuthConfig[0].Config.ClientSecret,
			"clientSecret must be scrubbed on GET")
	}
}

func (ts *AgentAPITestSuite) TestAgentGetByID_NotFound() {
	resp, err := doGet(testServerURL + agentBasePath + "/non-existent-id")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusNotFound, resp.StatusCode)
}

// TestAgentDisplay verifies that include=display resolves the agent's display value from the
// schema's display attribute (name on the default agent type), as it does for users.
func (ts *AgentAPITestSuite) TestAgentDisplay() {
	resp, err := doGet(testServerURL + agentBasePath + "/" + createdAgentID + "?include=display")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode)
	var agent Agent
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&agent))
	ts.Assert().Equal(createdAgentName, agent.Display)

	plain, err := doGet(testServerURL + agentBasePath + "/" + createdAgentID)
	ts.Require().NoError(err)
	defer plain.Body.Close()
	var withoutDisplay Agent
	ts.Require().NoError(json.NewDecoder(plain.Body).Decode(&withoutDisplay))
	ts.Assert().Empty(withoutDisplay.Display, "display is only returned when requested")

	listResp, err := doGet(testServerURL + agentBasePath + "?include=display&limit=100")
	ts.Require().NoError(err)
	defer listResp.Body.Close()
	var list struct {
		Agents []Agent `json:"agents"`
	}
	ts.Require().NoError(json.NewDecoder(listResp.Body).Decode(&list))
	found := false
	for _, a := range list.Agents {
		if a.ID == createdAgentID {
			found = true
			ts.Assert().Equal(createdAgentName, a.Display)
		}
	}
	ts.Assert().True(found, "the created agent must be listed")
}

// TestAgentDisplay_FallsBackToIDWithoutDisplayAttribute verifies the user-like fallback: when the
// agent type no longer declares a display attribute, the agent's display value is its ID.
func (ts *AgentAPITestSuite) TestAgentDisplay_FallsBackToIDWithoutDisplayAttribute() {
	noDisplay := agentSchema
	noDisplay.SystemAttributes = nil
	_, err := testutils.CreateAgentType(noDisplay)
	ts.Require().NoError(err, "Failed to remove the display attribute from the agent type")
	defer func() {
		_, restoreErr := testutils.CreateAgentType(agentSchema)
		ts.Require().NoError(restoreErr, "Failed to restore the display attribute")
	}()

	resp, err := doGet(testServerURL + agentBasePath + "/" + createdAgentID + "?include=display")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode)
	var agent Agent
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&agent))
	ts.Assert().Equal(createdAgentID, agent.Display)
}

func (ts *AgentAPITestSuite) TestAgentUpdate() {
	ts.Require().NotEmpty(createdAgentID)

	updatePayload := Agent{
		OUID:        testOUID,
		Type:        "default",
		Attributes:  agentAttrs("entity-only-agent-updated", nil),
		Description: "Updated description",
	}
	body, _ := json.Marshal(updatePayload)
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("PUT", testServerURL+agentBasePath+"/"+createdAgentID, bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	var updated Agent
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&updated))
	ts.Assert().Equal("entity-only-agent-updated", updated.AttrName())
	ts.Assert().Equal("Updated description", updated.Description)

	// Restore the name so subsequent tests see the expected state.
	restore := Agent{
		OUID:        testOUID,
		Type:        "default",
		Attributes:  agentAttrs(createdAgentName, nil),
		Description: "Agent with entity row only",
	}
	restoreBody, _ := json.Marshal(restore)
	req2, err := http.NewRequest("PUT", testServerURL+agentBasePath+"/"+createdAgentID, bytes.NewReader(restoreBody))
	ts.Require().NoError(err)
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := client.Do(req2)
	ts.Require().NoError(err)
	resp2.Body.Close()
}

func (ts *AgentAPITestSuite) TestAgentUpdateReportsTheDerivedStatus() {
	ts.Require().NotEmpty(createdAgentID)
	encoded, err := json.Marshal(Agent{
		OUID:        testOUID,
		Type:        "default",
		Attributes:  agentAttrs(createdAgentName, nil),
		Description: "Agent with entity row only",
	})
	ts.Require().NoError(err)
	var body map[string]interface{}
	ts.Require().NoError(json.Unmarshal(encoded, &body))
	body["status"] = map[string]interface{}{"value": "SUSPENDED"}
	payload, err := json.Marshal(body)
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPut, testServerURL+agentBasePath+"/"+createdAgentID,
		bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(raw))

	var updated struct {
		Status json.RawMessage `json:"status"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &updated))
	ts.JSONEq(`{"value":"ACTIVE"}`, string(updated.Status), "the update must report the derived status")
}

func (ts *AgentAPITestSuite) TestAgentUpdate_NameConflict() {
	// Create a second agent, then try to rename it to the primary agent's name.
	other := Agent{OUID: testOUID, Type: "default", Attributes: agentAttrs("agent-conflict-temp", nil)}
	otherID, err := createAgent(other)
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(otherID) }()

	updatePayload := Agent{Type: "default", Attributes: agentAttrs(createdAgentName, nil)}
	body, _ := json.Marshal(updatePayload)
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("PUT", testServerURL+agentBasePath+"/"+otherID, bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusConflict, resp.StatusCode)
	ts.Assert().Equal("AGT-1014", errorCode(resp))
}

func (ts *AgentAPITestSuite) TestAgentDelete() {
	// Create a transient agent to delete.
	transient := Agent{OUID: testOUID, Type: "default", Attributes: agentAttrs("agent-to-delete", nil)}
	id, err := createAgent(transient)
	ts.Require().NoError(err)

	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("DELETE", testServerURL+agentBasePath+"/"+id, nil)
	ts.Require().NoError(err)

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusNoContent, resp.StatusCode)

	// Confirm it's gone.
	getResp, err := doGet(testServerURL + agentBasePath + "/" + id)
	ts.Require().NoError(err)
	getResp.Body.Close()
	ts.Assert().Equal(http.StatusNotFound, getResp.StatusCode)
}

// --- creation mode: entity only ---

func (ts *AgentAPITestSuite) TestCreateAgentEntityOnly() {
	agent := Agent{OUID: testOUID, Type: "default", Attributes: agentAttrs("create-entity-only", nil)}
	id, err := createAgent(agent)
	ts.Require().NoError(err, "entity-only agent creation must succeed")
	defer func() { _ = deleteAgent(id) }()

	resp, err := doGet(testServerURL + agentBasePath + "/" + id)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var fetched Agent
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&fetched))
	ts.Assert().Empty(fetched.InboundAuthConfig, "entity-only agent must have no inbound auth config")
	ts.Assert().Empty(fetched.AuthFlowID, "entity-only agent must have no auth flow ID")
}

// --- creation mode: entity + inbound ---

func (ts *AgentAPITestSuite) TestCreateAgentWithInboundProfile() {
	agent := inboundAgent
	agent.OUID = testOUID
	agent.AuthFlowID = defaultAuthFlowID

	resp, err := doPost(testServerURL+agentBasePath, agent)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, readBodyBytes(resp))

	var created Agent
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&created))
	ts.Assert().NotEmpty(created.ID)
	ts.Assert().Equal(defaultAuthFlowID, created.AuthFlowID)
	defer func() { _ = deleteAgent(created.ID) }()

	// GET must return the inbound fields too.
	getResp, err := doGet(testServerURL + agentBasePath + "/" + created.ID)
	ts.Require().NoError(err)
	defer getResp.Body.Close()
	var fetched Agent
	ts.Require().NoError(json.NewDecoder(getResp.Body).Decode(&fetched))
	ts.Assert().Equal(defaultAuthFlowID, fetched.AuthFlowID)
}

// --- creation mode: entity + inbound + OAuth ---

func (ts *AgentAPITestSuite) TestCreateAgentWithOAuth() {
	agent := oauthAgent
	agent.OUID = testOUID
	agent.AuthFlowID = defaultAuthFlowID

	resp, err := doPost(testServerURL+agentBasePath, agent)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, string(bodyBytes))

	var created Agent
	ts.Require().NoError(json.Unmarshal(bodyBytes, &created))
	ts.Require().NotEmpty(created.ID)
	defer func() { _ = deleteAgent(created.ID) }()

	// Create response must include clientId and clientSecret.
	ts.Require().Len(created.InboundAuthConfig, 1)
	cfg := created.InboundAuthConfig[0].Config
	ts.Require().NotNil(cfg)
	ts.Assert().Equal("agent_oauth_client", cfg.ClientID)
	ts.Assert().NotEmpty(cfg.ClientSecret, "clientSecret must be returned on create")

	// GET must scrub clientSecret.
	getResp, err := doGet(testServerURL + agentBasePath + "/" + created.ID)
	ts.Require().NoError(err)
	defer getResp.Body.Close()
	var fetched Agent
	ts.Require().NoError(json.NewDecoder(getResp.Body).Decode(&fetched))
	if len(fetched.InboundAuthConfig) > 0 && fetched.InboundAuthConfig[0].Config != nil {
		ts.Assert().Empty(fetched.InboundAuthConfig[0].Config.ClientSecret,
			"clientSecret must be scrubbed on GET")
	}
}

// delegatedAgent builds a delegated agent request with the given grant types.
func delegatedAgent(name string, grantTypes ...string) Agent {
	return Agent{
		OUID:       testOUID,
		Type:       "default",
		Attributes: agentAttrs(name, nil),
		Delegated:  true,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				Config: &OAuthAgentConfig{
					RedirectURIs: []string{"https://agent.example.com/callback"},
					GrantTypes:   grantTypes,
				},
			},
		},
	}
}

func (ts *AgentAPITestSuite) createAgentWithStatus(agent Agent, status int) (Agent, string) {
	ts.T().Helper()
	resp, err := doPost(testServerURL+agentBasePath, agent)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(status, resp.StatusCode, string(body))

	var created Agent
	_ = json.Unmarshal(body, &created)
	if created.ID != "" {
		ts.T().Cleanup(func() { _ = deleteAgent(created.ID) })
	}
	var apiErr struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(body, &apiErr)
	return created, apiErr.Code
}

func (ts *AgentAPITestSuite) TestCreateAgent_DelegatedGetsTheDefaultGrants() {
	created, _ := ts.createAgentWithStatus(delegatedAgent("delegated-defaults"), http.StatusCreated)

	ts.Require().Len(created.InboundAuthConfig, 1)
	cfg := created.InboundAuthConfig[0].Config
	ts.Require().NotNil(cfg)
	ts.Assert().Equal([]string{"client_credentials", "authorization_code", "refresh_token"}, cfg.GrantTypes)
	ts.Assert().Equal([]string{"code"}, cfg.ResponseTypes)
	ts.Assert().True(cfg.PKCERequired)
	ts.Assert().NotEmpty(cfg.ClientSecret)
}

func (ts *AgentAPITestSuite) TestCreateAgent_DelegatedWithoutAnOAuthConfigNeedsARedirectURI() {
	agent := delegatedAgent("delegated-no-config")
	agent.InboundAuthConfig = nil

	_, code := ts.createAgentWithStatus(agent, http.StatusBadRequest)

	// An OAuth config is created for it, and the default authorization code grant it brings needs a
	// redirect URI that the request had no way to give.
	ts.Assert().Equal("AGT-1021", code)
}

func (ts *AgentAPITestSuite) TestCreateAgent_NotDelegatedAlwaysHoldsClientCredentials() {
	agent := delegatedAgent("not-delegated-grants", "authorization_code")
	agent.Delegated = false
	agent.InboundAuthConfig[0].Config.ResponseTypes = []string{"code"}

	created, _ := ts.createAgentWithStatus(agent, http.StatusCreated)

	ts.Require().Len(created.InboundAuthConfig, 1)
	ts.Assert().Equal([]string{"client_credentials", "authorization_code"},
		created.InboundAuthConfig[0].Config.GrantTypes)
}

func (ts *AgentAPITestSuite) TestCreateAgent_DelegatedKeepsRequestedDelegationGrants() {
	agent := delegatedAgent("delegated-explicit", "authorization_code", "urn:openid:params:grant-type:ciba")
	agent.InboundAuthConfig[0].Config.ResponseTypes = []string{"code"}

	created, _ := ts.createAgentWithStatus(agent, http.StatusCreated)

	ts.Require().Len(created.InboundAuthConfig, 1)
	ts.Assert().Equal([]string{"client_credentials", "authorization_code", "urn:openid:params:grant-type:ciba"},
		created.InboundAuthConfig[0].Config.GrantTypes)
}

// A public client authenticates with no secret, which client credentials cannot use, so it is not
// given that grant.
func (ts *AgentAPITestSuite) TestCreateAgent_PublicClientIsNotGivenClientCredentials() {
	agent := delegatedAgent("public-client-agent", "authorization_code")
	agent.Delegated = false
	config := agent.InboundAuthConfig[0].Config
	config.ResponseTypes = []string{"code"}
	config.PKCERequired = true
	config.PublicClient = true
	config.TokenEndpointAuthMethod = "none"

	created, _ := ts.createAgentWithStatus(agent, http.StatusCreated)

	ts.Require().Len(created.InboundAuthConfig, 1)
	ts.Assert().Equal([]string{"authorization_code"}, created.InboundAuthConfig[0].Config.GrantTypes)
}

// Any one of authorization code, CIBA and token exchange makes an agent delegated. The others are not
// needed: CIBA signs a user in through the backchannel and issues refresh tokens itself.
func (ts *AgentAPITestSuite) TestCreateAgent_DelegatedWithoutTheAuthorizationCodeGrant() {
	const (
		cibaGrant     = "urn:openid:params:grant-type:ciba"
		exchangeGrant = "urn:ietf:params:oauth:grant-type:token-exchange"
	)
	cases := []struct {
		name     string
		requests []string
		expected []string
	}{
		{"CIBAAlone", []string{cibaGrant}, []string{"client_credentials", cibaGrant}},
		{"CIBAWithRefreshToken", []string{cibaGrant, "refresh_token"},
			[]string{"client_credentials", cibaGrant, "refresh_token"}},
		{"TokenExchangeAlone", []string{exchangeGrant}, []string{"client_credentials", exchangeGrant}},
	}

	for _, tc := range cases {
		ts.Run(tc.name, func() {
			agent := delegatedAgent("delegated-no-code-"+tc.name, tc.requests...)
			// Without the authorization code grant there is no callback to register.
			agent.InboundAuthConfig[0].Config.RedirectURIs = nil

			created, _ := ts.createAgentWithStatus(agent, http.StatusCreated)

			ts.Require().Len(created.InboundAuthConfig, 1)
			ts.Assert().Equal(tc.expected, created.InboundAuthConfig[0].Config.GrantTypes)
		})
	}
}

func (ts *AgentAPITestSuite) TestCreateAgent_DelegatedWithoutADelegationGrantIsRejected() {
	_, code := ts.createAgentWithStatus(delegatedAgent("delegated-refresh-only", "refresh_token"),
		http.StatusBadRequest)

	ts.Assert().Equal("AGT-1044", code)
}

// --- creation validation ---

func (ts *AgentAPITestSuite) TestCreateAgent_MissingName() {
	agent := Agent{OUID: testOUID, Type: "default", Attributes: json.RawMessage(`{}`)}
	resp, err := doPost(testServerURL+agentBasePath, agent)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode)
	ts.Assert().Equal("AGT-1015", errorCode(resp))
}

// TestCreateAgent_NoneAuthWithIDJAGRejected verifies that a public client using the 'none' auth method
// is rejected with 400 when it uses the jwt-bearer grant or enables ID-JAG, both of which require a
// confidential client.
func (ts *AgentAPITestSuite) TestCreateAgent_NoneAuthWithIDJAGRejected() {
	cases := []struct {
		name   string
		config map[string]interface{}
	}{
		{
			name: "jwt-bearer grant",
			config: map[string]interface{}{
				"clientId":                "agent_none_jwt_bearer_client",
				"grantTypes":              []string{"urn:ietf:params:oauth:grant-type:jwt-bearer"},
				"tokenEndpointAuthMethod": "none",
				"publicClient":            true,
			},
		},
		{
			name: "ID-JAG enabled",
			config: map[string]interface{}{
				"clientId":                "agent_none_idjag_client",
				"grantTypes":              []string{"urn:ietf:params:oauth:grant-type:token-exchange"},
				"tokenEndpointAuthMethod": "none",
				"publicClient":            true,
				"token": map[string]interface{}{
					"idJag": map[string]interface{}{
						"enabled":          true,
						"allowedAudiences": []string{"https://target-rs.example.com"},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		ts.Run(tc.name, func() {
			resp, err := doPost(testServerURL+agentBasePath, map[string]interface{}{
				"ouId":              testOUID,
				"type":              "default",
				"attributes":        map[string]interface{}{"name": "none-auth-" + tc.config["clientId"].(string)},
				"inboundAuthConfig": []map[string]interface{}{{"type": "oauth2", "config": tc.config}},
			})
			ts.Require().NoError(err)
			defer resp.Body.Close()
			ts.Require().Equal(http.StatusBadRequest, resp.StatusCode, readBody(resp))
			ts.Assert().Equal("AGT-1021", errorCode(resp))
		})
	}
}

// TestCreateAgent_InvalidIDJAGConfigRejected verifies that a confidential agent client enabling ID-JAG
// is rejected with 400 when it lacks the token-exchange grant, through which ID-JAGs are issued, or a
// non-blank allowed audience.
func (ts *AgentAPITestSuite) TestCreateAgent_InvalidIDJAGConfigRejected() {
	tokenExchange := "urn:ietf:params:oauth:grant-type:token-exchange"
	cases := []struct {
		name       string
		clientID   string
		grantTypes []string
		audiences  []string
		wantKey    string
	}{
		{
			name:       "without token exchange",
			clientID:   "agent_idjag_no_exchange_client",
			grantTypes: []string{"client_credentials"},
			audiences:  []string{"https://target-rs.example.com"},
			wantKey:    "error.agentservice.idjag_requires_token_exchange_description",
		},
		{
			name:       "without allowed audience",
			clientID:   "agent_idjag_no_audience_client",
			grantTypes: []string{"client_credentials", tokenExchange},
			audiences:  []string{},
			wantKey:    "error.agentservice.idjag_requires_allowed_audience_description",
		},
		{
			name:       "with blank allowed audience",
			clientID:   "agent_idjag_blank_audience_client",
			grantTypes: []string{"client_credentials", tokenExchange},
			audiences:  []string{"  "},
			wantKey:    "error.agentservice.idjag_requires_allowed_audience_description",
		},
	}

	for _, tc := range cases {
		ts.Run(tc.name, func() {
			resp, err := doPost(testServerURL+agentBasePath, map[string]interface{}{
				"ouId":       testOUID,
				"type":       "default",
				"attributes": map[string]interface{}{"name": "invalid-idjag-" + tc.clientID},
				"inboundAuthConfig": []map[string]interface{}{{
					"type": "oauth2",
					"config": map[string]interface{}{
						"clientId":                tc.clientID,
						"grantTypes":              tc.grantTypes,
						"tokenEndpointAuthMethod": "client_secret_basic",
						"token": map[string]interface{}{
							"idJag": map[string]interface{}{
								"enabled":          true,
								"allowedAudiences": tc.audiences,
							},
						},
					},
				}},
			})
			ts.Require().NoError(err)
			defer resp.Body.Close()
			body := readBody(resp)
			ts.Require().Equal(http.StatusBadRequest, resp.StatusCode, body)

			var errResp struct {
				Code        string `json:"code"`
				Description struct {
					Key string `json:"key"`
				} `json:"description"`
			}
			ts.Require().NoError(json.Unmarshal([]byte(body), &errResp))
			ts.Assert().Equal("AGT-1021", errResp.Code)
			ts.Assert().Equal(tc.wantKey, errResp.Description.Key)
		})
	}
}

func (ts *AgentAPITestSuite) TestCreateAgent_MissingType() {
	agent := Agent{OUID: testOUID, Attributes: agentAttrs("no-type-agent", nil)}
	resp, err := doPost(testServerURL+agentBasePath, agent)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode)
}

func (ts *AgentAPITestSuite) TestCreateAgent_DuplicateName() {
	// Using the primary agent name which already exists.
	dup := Agent{OUID: testOUID, Type: "default", Attributes: agentAttrs(createdAgentName, nil)}
	resp, err := doPost(testServerURL+agentBasePath, dup)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusConflict, resp.StatusCode)
	ts.Assert().Equal("AGT-1014", errorCode(resp))
}

// --- group membership ---

func (ts *AgentAPITestSuite) TestAgentGroups_EmptyMembership() {
	ts.Require().NotEmpty(createdAgentID)

	resp, err := doGet(testServerURL + agentBasePath + "/" + createdAgentID + "/groups")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	var groupResp AgentGroupListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&groupResp))
	ts.Assert().Equal(0, groupResp.TotalResults)
	ts.Assert().Equal(1, groupResp.StartIndex)
}

func (ts *AgentAPITestSuite) TestAgentGroupMembership() {
	// Create agent to add to a group.
	agentID, err := createAgent(Agent{OUID: testOUID, Type: "default", Attributes: agentAttrs("group-member-agent", nil)})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	// Create a group with the agent as a member.
	groupID, err := testutils.CreateGroup(testutils.Group{
		Name:    "agent-group-membership-test",
		OUID:    testOUID,
		Members: []testutils.Member{{Id: agentID, Type: "agent"}},
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteGroup(groupID) }()

	// Verify the agent appears in the group's member list.
	members, err := testutils.GetGroupMembers(groupID)
	ts.Require().NoError(err)
	found := false
	for _, m := range members {
		if m.ID == agentID && m.Type == "agent" {
			found = true
			break
		}
	}
	ts.Assert().True(found, "Agent should appear as a member of the group")

	// Verify the group appears in GET /agents/{id}/groups.
	resp, err := doGet(testServerURL + agentBasePath + "/" + agentID + "/groups")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	var groupResp AgentGroupListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&groupResp))
	ts.Assert().GreaterOrEqual(groupResp.TotalResults, 1)
	groupFound := false
	for _, g := range groupResp.Groups {
		if g.ID == groupID {
			groupFound = true
			break
		}
	}
	ts.Assert().True(groupFound, "Group should appear in agent's group list")
}

func (ts *AgentAPITestSuite) TestAgentGroupMembership_AddViaAPI() {
	// Create agent and an empty group.
	agentID, err := createAgent(Agent{OUID: testOUID, Type: "default", Attributes: agentAttrs("add-via-api-agent", nil)})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	groupID, err := testutils.CreateGroup(testutils.Group{Name: "agent-add-member-group", OUID: testOUID})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteGroup(groupID) }()

	// Call POST /groups/{id}/members/add with type "agent".
	addReq := map[string]interface{}{
		"members": []map[string]string{{"id": agentID, "type": "agent"}},
	}
	body, _ := json.Marshal(addReq)
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("POST", testServerURL+"/groups/"+groupID+"/members/add", bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	// Verify the agent is now in the group.
	members, err := testutils.GetGroupMembers(groupID)
	ts.Require().NoError(err)
	found := false
	for _, m := range members {
		if m.ID == agentID && m.Type == "agent" {
			found = true
			break
		}
	}
	ts.Assert().True(found, "Agent should appear as a member after add-via-API")
}

func (ts *AgentAPITestSuite) TestAgentRoleAssignment() {
	// Create agent.
	agentID, err := createAgent(Agent{OUID: testOUID, Type: "default", Attributes: agentAttrs("role-assigned-agent", nil)})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	// Create a role and assign the agent directly.
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "agent-direct-role",
		OUID: testOUID,
		Assignments: []testutils.Assignment{
			{ID: agentID, Type: "agent"},
		},
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteRole(roleID) }()

	// Verify assignment appears in GET /roles/{id}/assignments.
	assignments, err := testutils.GetRoleAssignments(roleID)
	ts.Require().NoError(err)
	ts.Require().Len(assignments, 1)
	ts.Assert().Equal(agentID, assignments[0].ID)
	ts.Assert().Equal("agent", assignments[0].Type)
}

func (ts *AgentAPITestSuite) TestAgentRoleAssignment_ViaGroup() {
	// Create agent and add it to a group.
	agentID, err := createAgent(Agent{OUID: testOUID, Type: "default", Attributes: agentAttrs("group-role-agent", nil)})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	groupID, err := testutils.CreateGroup(testutils.Group{
		Name:    "agent-role-group",
		OUID:    testOUID,
		Members: []testutils.Member{{Id: agentID, Type: "agent"}},
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteGroup(groupID) }()

	// Assign the group to a role.
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "agent-group-role",
		OUID: testOUID,
		Assignments: []testutils.Assignment{
			{ID: groupID, Type: "group"},
		},
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteRole(roleID) }()

	// Verify the group assignment appears in the role.
	assignments, err := testutils.GetRoleAssignments(roleID)
	ts.Require().NoError(err)
	ts.Require().Len(assignments, 1)
	ts.Assert().Equal(groupID, assignments[0].ID)
	ts.Assert().Equal("group", assignments[0].Type)

	// Verify the agent is still a member of the group.
	members, err := testutils.GetGroupMembers(groupID)
	ts.Require().NoError(err)
	found := false
	for _, m := range members {
		if m.ID == agentID && m.Type == "agent" {
			found = true
			break
		}
	}
	ts.Assert().True(found, "Agent should still be a member of the group")
}

// --- transition: entity-only → inbound profile on update ---

func (ts *AgentAPITestSuite) TestUpdateAgent_AddInboundProfile() {
	// Start with entity-only.
	agentID, err := createAgent(Agent{OUID: testOUID, Type: "default", Attributes: agentAttrs("transition-agent", nil)})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	// Update to add inbound fields.
	withInbound := Agent{
		OUID:       testOUID,
		Type:       "default",
		Attributes: agentAttrs("transition-agent", nil),
		AuthFlowID: defaultAuthFlowID,
	}
	body, _ := json.Marshal(withInbound)
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("PUT", testServerURL+agentBasePath+"/"+agentID, bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	var updated Agent
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&updated))
	ts.Assert().Equal(defaultAuthFlowID, updated.AuthFlowID)
}

// updateAgentWithStatus replaces an agent and checks the response status. It returns the agent and the
// error code from the response body.
func (ts *AgentAPITestSuite) updateAgentWithStatus(agentID string, agent Agent, status int) (Agent, string) {
	ts.T().Helper()
	body, err := json.Marshal(agent)
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPut, testServerURL+agentBasePath+"/"+agentID, bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(status, resp.StatusCode, string(raw))

	var updated Agent
	_ = json.Unmarshal(raw, &updated)
	var apiErr struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(raw, &apiErr)
	return updated, apiErr.Code
}

func (ts *AgentAPITestSuite) TestUpdateAgent_DelegatedGetsTheDefaultGrants() {
	agentID, err := createAgent(Agent{OUID: testOUID, Type: "default", Attributes: agentAttrs("update-delegated", nil)})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	updated, _ := ts.updateAgentWithStatus(agentID, delegatedAgent("update-delegated"), http.StatusOK)

	ts.Require().Len(updated.InboundAuthConfig, 1)
	cfg := updated.InboundAuthConfig[0].Config
	ts.Require().NotNil(cfg)
	ts.Assert().Equal([]string{"client_credentials", "authorization_code", "refresh_token"}, cfg.GrantTypes)
	ts.Assert().Equal([]string{"code"}, cfg.ResponseTypes)
	ts.Assert().True(cfg.PKCERequired)
}

func (ts *AgentAPITestSuite) TestUpdateAgent_NotDelegatedAlwaysHoldsClientCredentials() {
	agentID, err := createAgent(Agent{OUID: testOUID, Type: "default", Attributes: agentAttrs("update-cc", nil)})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	agent := delegatedAgent("update-cc", "authorization_code")
	agent.Delegated = false
	agent.InboundAuthConfig[0].Config.ResponseTypes = []string{"code"}

	updated, _ := ts.updateAgentWithStatus(agentID, agent, http.StatusOK)

	ts.Require().Len(updated.InboundAuthConfig, 1)
	ts.Assert().Equal([]string{"client_credentials", "authorization_code"},
		updated.InboundAuthConfig[0].Config.GrantTypes)
}

func (ts *AgentAPITestSuite) TestUpdateAgent_DelegatedWithoutADelegationGrantIsRejected() {
	agentID, err := createAgent(Agent{OUID: testOUID, Type: "default", Attributes: agentAttrs("update-refresh", nil)})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	_, code := ts.updateAgentWithStatus(agentID, delegatedAgent("update-refresh", "refresh_token"),
		http.StatusBadRequest)

	ts.Assert().Equal("AGT-1044", code)

	// The rejected update leaves the agent as it was, with no inbound client.
	resp, err := doGet(testServerURL + agentBasePath + "/" + agentID)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	var fetched Agent
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&fetched))
	ts.Assert().Empty(fetched.InboundAuthConfig)
}

// --- transition: inbound profile → entity-only on update ---

func (ts *AgentAPITestSuite) TestUpdateAgent_RemoveInboundProfile() {
	// Start with inbound.
	agentID, err := createAgent(Agent{
		OUID:       testOUID,
		Type:       "default",
		Attributes: agentAttrs("strip-inbound-agent", nil),
		AuthFlowID: defaultAuthFlowID,
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	// Update dropping all inbound fields.
	stripped := Agent{OUID: testOUID, Type: "default", Attributes: agentAttrs("strip-inbound-agent", nil)}
	body, _ := json.Marshal(stripped)
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("PUT", testServerURL+agentBasePath+"/"+agentID, bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	var updated Agent
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&updated))
	ts.Assert().Empty(updated.AuthFlowID,
		"AuthFlowID must be cleared after removing inbound profile")
}

// --- flow handle resolution on update ---
//
// Handles are input-only: the API resolves them to IDs and returns only the resolved ID, so these
// tests send raw request bodies and assert on the ID fields of the response.

// TestUpdateAgent_AuthFlowHandleOnly_ResolvesToFlowID covers an update whose only inbound field is
// authFlowHandle. The handle must resolve and the inbound client must survive, rather than the
// request being read as "no inbound fields supplied" and the client being dropped.
func (ts *AgentAPITestSuite) TestUpdateAgent_AuthFlowHandleOnly_ResolvesToFlowID() {
	agentID, err := createAgent(Agent{
		OUID:       testOUID,
		Type:       "default",
		Attributes: agentAttrs("handle-only-agent", nil),
		AuthFlowID: handleAuthFlowID1,
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	resp, err := putAgentRaw(agentID, map[string]interface{}{
		"ouId":           testOUID,
		"type":           "default",
		"attributes":     map[string]interface{}{"name": "handle-only-agent"},
		"authFlowHandle": handleAuthFlowHandle1,
	})
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	var updated Agent
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&updated))
	ts.Assert().Equal(handleAuthFlowID1, updated.AuthFlowID,
		"authFlowHandle must resolve to its flow ID on update")

	// Confirm the resolved binding was persisted, not just echoed in the response.
	getResp, err := doGet(testServerURL + agentBasePath + "/" + agentID)
	ts.Require().NoError(err)
	defer getResp.Body.Close()
	ts.Require().Equal(http.StatusOK, getResp.StatusCode, readBody(getResp))

	var fetched Agent
	ts.Require().NoError(json.NewDecoder(getResp.Body).Decode(&fetched))
	ts.Assert().Equal(handleAuthFlowID1, fetched.AuthFlowID,
		"the resolved auth flow ID must be persisted")
}

// TestUpdateAgent_AuthFlowHandleRepointed_SwitchesFlow covers a handle that resolves to a flow
// other than the one currently bound: the agent must move to the new flow.
func (ts *AgentAPITestSuite) TestUpdateAgent_AuthFlowHandleRepointed_SwitchesFlow() {
	agentID, err := createAgent(Agent{
		OUID:       testOUID,
		Type:       "default",
		Attributes: agentAttrs("handle-repoint-agent", nil),
		AuthFlowID: handleAuthFlowID1,
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	resp, err := putAgentRaw(agentID, map[string]interface{}{
		"ouId":           testOUID,
		"type":           "default",
		"attributes":     map[string]interface{}{"name": "handle-repoint-agent"},
		"authFlowHandle": handleAuthFlowHandle2,
	})
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	var updated Agent
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&updated))
	ts.Assert().Equal(handleAuthFlowID2, updated.AuthFlowID,
		"a handle pointing at a different flow must switch the binding")
}

// TestUpdateAgent_UnresolvableAuthFlowHandle_PreservesExistingFlow covers a handle that resolves to
// nothing: the update must be rejected and the previously bound flow left untouched.
func (ts *AgentAPITestSuite) TestUpdateAgent_UnresolvableAuthFlowHandle_PreservesExistingFlow() {
	agentID, err := createAgent(Agent{
		OUID:       testOUID,
		Type:       "default",
		Attributes: agentAttrs("handle-unresolvable-agent", nil),
		AuthFlowID: handleAuthFlowID1,
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	resp, err := putAgentRaw(agentID, map[string]interface{}{
		"ouId":           testOUID,
		"type":           "default",
		"attributes":     map[string]interface{}{"name": "handle-unresolvable-agent"},
		"authFlowHandle": "agent-api-nonexistent-flow-handle",
	})
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusBadRequest, resp.StatusCode, readBody(resp))

	getResp, err := doGet(testServerURL + agentBasePath + "/" + agentID)
	ts.Require().NoError(err)
	defer getResp.Body.Close()
	ts.Require().Equal(http.StatusOK, getResp.StatusCode, readBody(getResp))

	var fetched Agent
	ts.Require().NoError(json.NewDecoder(getResp.Body).Decode(&fetched))
	ts.Assert().Equal(handleAuthFlowID1, fetched.AuthFlowID,
		"a rejected update must leave the existing auth flow binding intact")
}

// TestUpdateAgent_AuthFlowIDTakesPrecedenceOverHandle covers a request carrying both fields: the
// explicit ID wins and the handle is ignored.
func (ts *AgentAPITestSuite) TestUpdateAgent_AuthFlowIDTakesPrecedenceOverHandle() {
	agentID, err := createAgent(Agent{
		OUID:       testOUID,
		Type:       "default",
		Attributes: agentAttrs("handle-precedence-agent", nil),
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	resp, err := putAgentRaw(agentID, map[string]interface{}{
		"ouId":           testOUID,
		"type":           "default",
		"attributes":     map[string]interface{}{"name": "handle-precedence-agent"},
		"authFlowId":     handleAuthFlowID1,
		"authFlowHandle": handleAuthFlowHandle2,
	})
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	var updated Agent
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&updated))
	ts.Assert().Equal(handleAuthFlowID1, updated.AuthFlowID,
		"authFlowId must win when both the ID and the handle are supplied")
}

// --- helpers ---

func createAgent(agent Agent) (string, error) {
	resp, err := doPost(testServerURL+agentBasePath, agent)
	if err != nil {
		return "", fmt.Errorf("failed to send create request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("expected status 201, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var created Agent
	if err := json.Unmarshal(bodyBytes, &created); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	if created.ID == "" {
		return "", fmt.Errorf("response does not contain id. Response: %s", string(bodyBytes))
	}
	return created.ID, nil
}

func deleteAgent(agentID string) error {
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("DELETE", testServerURL+agentBasePath+"/"+agentID, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 204, got %d. Response: %s", resp.StatusCode, string(body))
	}
	return nil
}

// putAgentRaw sends an update with an arbitrary body, so tests can supply the handle fields that
// the ID-only Agent struct deliberately omits.
func putAgentRaw(agentID string, body map[string]interface{}) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("PUT", testServerURL+agentBasePath+"/"+agentID, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return client.Do(req)
}

func doGet(url string) (*http.Response, error) {
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func doPost(url string, body interface{}) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return client.Do(req)
}

func readBody(resp *http.Response) string {
	b, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewReader(b))
	return string(b)
}

// errorCode reads the error code from an error response body and leaves the body readable.
func errorCode(resp *http.Response) string {
	var e struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal([]byte(readBody(resp)), &e)
	return e.Code
}

func readBodyBytes(resp *http.Response) string {
	b, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewReader(b))
	return string(b)
}

// ============================================================================
// AgentAttributesTestSuite — custom attribute CRUD and filter operations
// ============================================================================

var (
	attrTestOU = testutils.OrganizationUnit{
		Handle:      "agent-attr-test-ou",
		Name:        "Agent Attribute Test OU",
		Description: "Organization unit for agent attribute CRUD testing",
		Parent:      nil,
	}

	attrAgentSchema = testutils.UserType{
		Handle:      "default",
		DisplayName: "Default",
		Schema: map[string]interface{}{
			"name":   map[string]interface{}{"type": "string", "required": true, "unique": true},
			"region": map[string]interface{}{"type": "string"},
			"tier":   map[string]interface{}{"type": "string"},
		},
	}
)

// AgentAttributesTestSuite covers custom attribute CRUD and filter operations on agents.
type AgentAttributesTestSuite struct {
	suite.Suite
	ouID              string
	agentTypeSnapshot *testutils.AgentTypeSnapshot
}

func TestAgentAttributesTestSuite(t *testing.T) {
	suite.Run(t, new(AgentAttributesTestSuite))
}

func (ts *AgentAttributesTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(attrTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	// The `default` agent type is a singleton shared with every other suite. Snapshot it before
	// pointing it at this suite's OU, so teardown can put it back before that OU is deleted.
	snapshot, err := testutils.SnapshotAgentType()
	ts.Require().NoError(err, "Failed to snapshot the default agent type")
	ts.agentTypeSnapshot = snapshot

	attrAgentSchema.OUID = ts.ouID
	_, err = testutils.CreateAgentType(attrAgentSchema)
	ts.Require().NoError(err, "Failed to create agent schema")
}

func (ts *AgentAttributesTestSuite) TearDownSuite() {
	// Restore the shared agent type before deleting the OU it points at, or the singleton is left
	// referencing a deleted OU and a later suite's restore fails. A failed restore keeps the OU:
	// leaking one is cheaper than every later suite inheriting a dangling reference.
	agentTypeRestored := true
	if ts.agentTypeSnapshot != nil {
		if err := testutils.RestoreAgentType(ts.agentTypeSnapshot); err != nil {
			ts.T().Errorf("teardown: failed to restore the default agent type: %v", err)
			agentTypeRestored = false
		}
	}
	if ts.ouID != "" && agentTypeRestored {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// TestAgentAttributes_CreateWithAttributes verifies that custom attributes submitted on
// create are stored and returned verbatim by GET.
func (ts *AgentAttributesTestSuite) TestAgentAttributes_CreateWithAttributes() {
	attrs := json.RawMessage(`{"region":"us-east","tier":"premium"}`)
	agent := Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Attributes: withName("attr-create-agent", attrs),
	}

	resp, err := doPost(testServerURL+agentBasePath, agent)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, readBodyBytes(resp))

	var created Agent
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&created))
	ts.Require().NotEmpty(created.ID)
	defer func() { _ = deleteAgent(created.ID) }()

	getResp, err := doGet(testServerURL + agentBasePath + "/" + created.ID)
	ts.Require().NoError(err)
	defer getResp.Body.Close()
	ts.Require().Equal(http.StatusOK, getResp.StatusCode)

	var fetched Agent
	ts.Require().NoError(json.NewDecoder(getResp.Body).Decode(&fetched))

	var gotAttrs map[string]interface{}
	ts.Require().NoError(json.Unmarshal(fetched.Attributes, &gotAttrs))
	ts.Assert().Equal("us-east", gotAttrs["region"])
	ts.Assert().Equal("premium", gotAttrs["tier"])
}

// TestAgentAttributes_UpdateAttributes verifies that attributes can be replaced on update.
func (ts *AgentAttributesTestSuite) TestAgentAttributes_UpdateAttributes() {
	agentID, err := createAgent(Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Attributes: withName("attr-update-agent", json.RawMessage(`{"region":"eu-west","tier":"standard"}`)),
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	updated := Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Attributes: withName("attr-update-agent", json.RawMessage(`{"region":"ap-south","tier":"enterprise"}`)),
	}
	body, _ := json.Marshal(updated)
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("PUT", testServerURL+agentBasePath+"/"+agentID, bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	getResp, err := doGet(testServerURL + agentBasePath + "/" + agentID)
	ts.Require().NoError(err)
	defer getResp.Body.Close()
	ts.Require().Equal(http.StatusOK, getResp.StatusCode, readBody(getResp))

	var fetched Agent
	ts.Require().NoError(json.NewDecoder(getResp.Body).Decode(&fetched))

	var gotAttrs map[string]interface{}
	ts.Require().NoError(json.Unmarshal(fetched.Attributes, &gotAttrs))
	ts.Assert().Equal("ap-south", gotAttrs["region"])
	ts.Assert().Equal("enterprise", gotAttrs["tier"])
}

// TestAgentAttributes_FilterByAttribute verifies that GET /agents?filter=attr eq "value"
// returns exactly the matching agents. The result set is pinned by count and by ID, so a filter
// that silently widened to every agent would fail rather than still finding the expected one.
func (ts *AgentAttributesTestSuite) TestAgentAttributes_FilterByAttribute() {
	idA, err := createAgent(Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Attributes: withName("filter-agent-alpha", json.RawMessage(`{"region":"filter-target","tier":"gold"}`)),
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(idA) }()

	idB, err := createAgent(Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Attributes: withName("filter-agent-beta", json.RawMessage(`{"region":"other-region","tier":"silver"}`)),
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(idB) }()

	q := url.Values{}
	q.Set("filter", `region eq "filter-target"`)
	resp, err := doGet(testServerURL + agentBasePath + "?" + q.Encode())
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	var listResp AgentListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	ts.Assert().Equal(1, listResp.TotalResults, "filter must match exactly the alpha agent")
	ts.Assert().Equal(1, listResp.Count)
	ts.Require().Len(listResp.Agents, 1)
	ts.Assert().Equal(idA, listResp.Agents[0].ID,
		"the single match must be the alpha agent, which also proves beta was excluded")
}

// TestAgentAttributes_NullifyAttributes verifies that omitting attributes on update
// clears previously stored attributes.
func (ts *AgentAttributesTestSuite) TestAgentAttributes_NullifyAttributes() {
	agentID, err := createAgent(Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Attributes: withName("attr-nullify-agent", json.RawMessage(`{"region":"to-clear","tier":"basic"}`)),
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	stripped := Agent{OUID: ts.ouID, Type: "default", Attributes: agentAttrs("attr-nullify-agent", nil)}
	body, _ := json.Marshal(stripped)
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("PUT", testServerURL+agentBasePath+"/"+agentID, bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	getResp, err := doGet(testServerURL + agentBasePath + "/" + agentID)
	ts.Require().NoError(err)
	defer getResp.Body.Close()
	ts.Require().Equal(http.StatusOK, getResp.StatusCode, readBody(getResp))

	var fetched Agent
	ts.Require().NoError(json.NewDecoder(getResp.Body).Decode(&fetched))
	ts.Assert().JSONEq(`{"name":"attr-nullify-agent"}`, string(fetched.Attributes),
		"only the name should remain after the nullify update")
}
