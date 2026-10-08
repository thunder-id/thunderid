// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agenttype

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	agentTypeBasePath = "/agent-types"
	agentBasePath     = "/agents"

	// defaultAgentTypeName is the only handle an agent type is permitted to carry.
	defaultAgentTypeName = "default"
)

// AgentTypeAPITestSuite covers the /agent-types API, whose contract differs from /user-types in
// three ways that had no integration coverage: exactly one type may exist, its handle must be
// `default`, and it cannot be deleted.
//
// The `default` agent type is a singleton shared with every other package. This suite therefore
// snapshots it in setup and restores it in teardown, and every test that mutates the schema
// restores it in a defer rather than relying on teardown — testify orders suite methods
// alphabetically, so a lingering edit would silently become another test's precondition.
type AgentTypeAPITestSuite struct {
	suite.Suite
	snapshot *testutils.AgentTypeSnapshot
	ouID     string
}

func TestAgentTypeAPITestSuite(t *testing.T) {
	suite.Run(t, new(AgentTypeAPITestSuite))
}

func (s *AgentTypeAPITestSuite) SetupSuite() {
	// Require the singleton to exist rather than creating it. Agent types cannot be deleted, so
	// creating one here would be unreversible, and an absent `default` is a broken environment
	// rather than something a test suite should paper over.
	snapshot, err := testutils.SnapshotAgentType()
	s.Require().NoError(err, "the default agent type must exist before this suite runs")
	s.Require().NotEmpty(snapshot.ID)
	s.Require().NotEmpty(snapshot.Schema, "the default agent type must carry a schema")
	s.snapshot = snapshot

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "agent-type-api-ou",
		Name:        "Agent Type API OU",
		Description: "OU supplying a valid ouId to agent type rejection tests",
	})
	s.Require().NoError(err)
	s.ouID = ouID
}

func (s *AgentTypeAPITestSuite) TearDownSuite() {
	if s.snapshot != nil {
		s.NoError(testutils.RestoreAgentType(s.snapshot),
			"the default agent type must be restored for later packages")
	}
	if s.ouID != "" {
		s.NoError(testutils.DeleteOrganizationUnit(s.ouID), "teardown: delete OU")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (s *AgentTypeAPITestSuite) do(method, path string, body interface{}) *http.Response {
	s.T().Helper()
	return s.doWith(testutils.GetHTTPClient(), method, path, body)
}

func (s *AgentTypeAPITestSuite) doWith(
	client *http.Client, method, path string, body interface{},
) *http.Response {
	s.T().Helper()

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		s.Require().NoError(err)
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, testutils.TestServerURL+path, reader)
	s.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	s.Require().NoError(err)
	return resp
}

// decodeError reads an error response and asserts the exact product error code.
func (s *AgentTypeAPITestSuite) decodeError(resp *http.Response, expectedCode string) ErrorResponse {
	s.T().Helper()

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var errResp ErrorResponse
	s.Require().NoError(json.Unmarshal(body, &errResp), "error body: %s", string(body))
	s.Equal(expectedCode, errResp.Code, "error body: %s", string(body))
	return errResp
}

// listAgentTypes returns the current agent type list.
func (s *AgentTypeAPITestSuite) listAgentTypes() AgentTypeListResponse {
	s.T().Helper()

	resp := s.do(http.MethodGet, agentTypeBasePath, nil)
	defer closeBody(resp)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var list AgentTypeListResponse
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&list))
	return list
}

// getAgentType fetches the singleton from the detail endpoint, which is the only endpoint that
// carries the schema.
func (s *AgentTypeAPITestSuite) getAgentType(id string) AgentType {
	s.T().Helper()

	resp := s.do(http.MethodGet, agentTypeBasePath+"/"+id, nil)
	defer closeBody(resp)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var agentType AgentType
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&agentType))
	return agentType
}

// putSchema replaces the singleton's schema, keeping its handle, display name and OU.
func (s *AgentTypeAPITestSuite) putSchema(schema interface{}) {
	s.T().Helper()

	encoded, err := json.Marshal(schema)
	s.Require().NoError(err)

	resp := s.do(http.MethodPut, agentTypeBasePath+"/"+s.snapshot.ID, AgentTypeRequest{
		Handle:      defaultAgentTypeName,
		DisplayName: s.snapshot.DisplayName,
		OUID:        s.snapshot.OUID,
		Schema:      encoded,
	})
	defer closeBody(resp)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "schema update failed: %s", string(body))
}

// restoreSchema puts the snapshot's schema back. Deferred by every test that edits the schema.
func (s *AgentTypeAPITestSuite) restoreSchema() {
	s.T().Helper()
	s.NoError(testutils.RestoreAgentType(s.snapshot))
}

func closeBody(resp *http.Response) { _ = resp.Body.Close() }

// ---------------------------------------------------------------------------
// Scenario 25 — only `default` may be created
// ---------------------------------------------------------------------------

// TestCreateNonDefaultAgentTypeRejected verifies that agent types are restricted to the single
// `default` schema: a create with any other handle is refused and nothing is persisted.
func (s *AgentTypeAPITestSuite) TestCreateNonDefaultAgentTypeRejected() {
	before := s.listAgentTypes()

	resp := s.do(http.MethodPost, agentTypeBasePath, AgentTypeRequest{
		Handle:      "custom-agent-type",
		DisplayName: "Custom Agent Type",
		OUID:        s.ouID,
		Schema:      json.RawMessage(`{"description": {"type": "string"}}`),
	})
	defer closeBody(resp)

	s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
	s.decodeError(resp, "USRS-1014")

	after := s.listAgentTypes()
	s.Equal(before.TotalResults, after.TotalResults, "a rejected create must not persist a type")
	for _, t := range after.Types {
		s.NotEqual("custom-agent-type", t.Handle)
	}
}

// ---------------------------------------------------------------------------
// Scenario 26 — `default` is a singleton
// ---------------------------------------------------------------------------

// TestCreateDuplicateDefaultAgentTypeRejected verifies that a second `default` agent type is
// refused as a handle conflict, so the singleton cannot be duplicated.
func (s *AgentTypeAPITestSuite) TestCreateDuplicateDefaultAgentTypeRejected() {
	before := s.listAgentTypes()

	resp := s.do(http.MethodPost, agentTypeBasePath, AgentTypeRequest{
		Handle:      defaultAgentTypeName,
		DisplayName: "Another Default",
		OUID:        s.ouID,
		Schema:      json.RawMessage(`{"description": {"type": "string"}}`),
	})
	defer closeBody(resp)

	s.Require().Equal(http.StatusConflict, resp.StatusCode)
	s.decodeError(resp, "USRS-1003")

	after := s.listAgentTypes()
	s.Equal(before.TotalResults, after.TotalResults, "a rejected create must not persist a type")
}

// ---------------------------------------------------------------------------
// Scenario 27: the `default` handle cannot be changed
// ---------------------------------------------------------------------------

// TestChangeDefaultAgentTypeHandleRejected verifies that the singleton's handle cannot be changed
// away from the `default` that agent creation depends on, and that the stored handle is unchanged.
func (s *AgentTypeAPITestSuite) TestChangeDefaultAgentTypeHandleRejected() {
	schema, err := json.Marshal(s.snapshot.Schema)
	s.Require().NoError(err)

	resp := s.do(http.MethodPut, agentTypeBasePath+"/"+s.snapshot.ID, AgentTypeRequest{
		Handle:      "renamed-agent-type",
		DisplayName: "Renamed Agent Type",
		OUID:        s.snapshot.OUID,
		Schema:      schema,
	})
	defer closeBody(resp)

	s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
	s.decodeError(resp, "USRS-1014")

	s.Equal(defaultAgentTypeName, s.getAgentType(s.snapshot.ID).Handle,
		"a rejected handle change must not change the stored handle")
}

// ---------------------------------------------------------------------------
// Scenario 28 — `default` cannot be deleted
// ---------------------------------------------------------------------------

// TestDeleteAgentTypeRejected verifies that the agent type survives a delete attempt. Agent
// creation depends on it, so deletion is refused outright rather than cascading.
func (s *AgentTypeAPITestSuite) TestDeleteAgentTypeRejected() {
	resp := s.do(http.MethodDelete, agentTypeBasePath+"/"+s.snapshot.ID, nil)
	defer closeBody(resp)

	s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
	s.decodeError(resp, "USRS-1015")

	s.Equal(defaultAgentTypeName, s.getAgentType(s.snapshot.ID).Handle,
		"the agent type must still exist after a refused delete")
}

// ---------------------------------------------------------------------------
// Scenario 29 — unknown id carries the agent-specific description
// ---------------------------------------------------------------------------

// TestGetUnknownAgentTypeReturnsAgentSpecificError verifies that /agent-types reports a missing
// type in agent terms. The category shares USRS-1002 with /user-types, so only the description
// distinguishes them.
func (s *AgentTypeAPITestSuite) TestGetUnknownAgentTypeReturnsAgentSpecificError() {
	resp := s.do(http.MethodGet, agentTypeBasePath+"/01900000-0000-7000-8000-00000000dead", nil)
	defer closeBody(resp)

	s.Require().Equal(http.StatusNotFound, resp.StatusCode)
	errResp := s.decodeError(resp, "USRS-1002")

	s.Equal("Agent type not found", errResp.Message.DefaultValue)
	s.Equal("The agent type with the specified id does not exist", errResp.Description.DefaultValue)
}

// ---------------------------------------------------------------------------
// Scenario 31 — the list holds exactly the singleton
// ---------------------------------------------------------------------------

// TestListAgentTypesReturnsOnlyTheSingleton verifies the list endpoint reports exactly one agent
// type, under the `types` key, and that it is the `default` one.
func (s *AgentTypeAPITestSuite) TestListAgentTypesReturnsOnlyTheSingleton() {
	list := s.listAgentTypes()

	s.Equal(1, list.TotalResults)
	s.Equal(1, list.Count)
	s.Require().Len(list.Types, 1)
	s.Equal(defaultAgentTypeName, list.Types[0].Handle)
	s.Equal(s.snapshot.DisplayName, list.Types[0].DisplayName)
	s.Equal(s.snapshot.ID, list.Types[0].ID)
}

// ---------------------------------------------------------------------------
// Scenario 32 — schema edits persist
// ---------------------------------------------------------------------------

// TestUpdateAgentTypeSchemaPersists verifies that editing the singleton's schema is the supported
// way to change it, and that the edit is readable back from the detail endpoint.
func (s *AgentTypeAPITestSuite) TestUpdateAgentTypeSchemaPersists() {
	defer s.restoreSchema()

	s.putSchema(map[string]interface{}{
		"description": map[string]interface{}{"type": "string"},
		"costCentre":  map[string]interface{}{"type": "string"},
	})

	var stored map[string]interface{}
	s.Require().NoError(json.Unmarshal(s.getAgentType(s.snapshot.ID).Schema, &stored))

	s.Contains(stored, "costCentre", "the edited attribute must be persisted")
	s.Contains(stored, "description")
	s.Len(stored, 2, "the update replaces the schema rather than merging into it")
}

// ---------------------------------------------------------------------------
// Handle and display name
// ---------------------------------------------------------------------------

// TestUpdateAgentTypeDisplayNamePersists verifies that a PUT without the immutable handle and with a
// new display name succeeds, changes only the display name, and keeps the schema.
func (s *AgentTypeAPITestSuite) TestUpdateAgentTypeDisplayNamePersists() {
	defer s.restoreSchema()

	schema, err := json.Marshal(s.snapshot.Schema)
	s.Require().NoError(err)

	resp := s.do(http.MethodPut, agentTypeBasePath+"/"+s.snapshot.ID, AgentTypeRequest{
		DisplayName: "Agent Type Renamed Label",
		OUID:        s.snapshot.OUID,
		Schema:      schema,
	})
	defer closeBody(resp)
	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "display name update failed: %s", string(body))

	stored := s.getAgentType(s.snapshot.ID)
	s.Equal(defaultAgentTypeName, stored.Handle, "the handle must not change")
	s.Equal("Agent Type Renamed Label", stored.DisplayName)

	var storedSchema map[string]interface{}
	s.Require().NoError(json.Unmarshal(stored.Schema, &storedSchema))
	s.Equal(s.snapshot.Schema, storedSchema, "a display name update must keep the schema")
}

// ---------------------------------------------------------------------------
// Scenario 46 — a unique attribute in the schema constrains agent creation
// ---------------------------------------------------------------------------

// TestUniqueAgentTypeAttributeRejectsDuplicateAgent verifies that a `unique` constraint declared
// in the agent type schema is enforced when agents are created: a second agent reusing the value
// is refused and not persisted. The agents carry distinct names so the conflict can only come
// from the attribute.
func (s *AgentTypeAPITestSuite) TestUniqueAgentTypeAttributeRejectsDuplicateAgent() {
	defer s.restoreSchema()

	s.putSchema(map[string]interface{}{
		"serialNumber": map[string]interface{}{"type": "string", "unique": true},
	})

	firstID := s.createAgent(Agent{
		Type:       defaultAgentTypeName,
		Name:       "agent-type-unique-first",
		OUID:       s.snapshot.OUID,
		Attributes: json.RawMessage(`{"serialNumber": "SN-AGENTTYPE-0001"}`),
	})
	defer func() { s.deleteAgent(firstID) }()

	resp := s.do(http.MethodPost, agentBasePath, Agent{
		Type:       defaultAgentTypeName,
		Name:       "agent-type-unique-second",
		OUID:       s.snapshot.OUID,
		Attributes: json.RawMessage(`{"serialNumber": "SN-AGENTTYPE-0001"}`),
	})
	defer closeBody(resp)

	s.Require().Equal(http.StatusConflict, resp.StatusCode)
	s.decodeError(resp, "AGT-1014")

	holders := s.agentsWithSerial("SN-AGENTTYPE-0001")
	s.Require().Len(holders, 1, "the rejected agent must not be persisted")
	s.Equal("agent-type-unique-first", holders[0].Name,
		"the surviving agent must be the one created first")
}

// createAgent creates an agent and returns its ID.
func (s *AgentTypeAPITestSuite) createAgent(agent Agent) string {
	s.T().Helper()

	resp := s.do(http.MethodPost, agentBasePath, agent)
	defer closeBody(resp)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusCreated, resp.StatusCode, "create agent failed: %s", string(body))

	var created Agent
	s.Require().NoError(json.Unmarshal(body, &created))
	s.Require().NotEmpty(created.ID)
	return created.ID
}

// deleteAgent removes an agent and requires the delete to succeed, so a leaked agent cannot pass
// unnoticed and become a later suite's precondition.
func (s *AgentTypeAPITestSuite) deleteAgent(id string) {
	s.T().Helper()

	resp := s.do(http.MethodDelete, agentBasePath+"/"+id, nil)
	defer closeBody(resp)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	s.Equal(http.StatusNoContent, resp.StatusCode, "cleanup: delete agent %s: %s", id, string(body))
}

// agentsWithSerial returns the agents carrying the given serialNumber, so a uniqueness claim can
// be checked against stored state rather than against a list scan.
func (s *AgentTypeAPITestSuite) agentsWithSerial(serial string) []Agent {
	s.T().Helper()

	resp := s.do(http.MethodGet,
		agentBasePath+"?filter="+url.QueryEscape(`serialNumber eq "`+serial+`"`), nil)
	defer closeBody(resp)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var list AgentListResponse
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&list))
	return list.Agents
}
