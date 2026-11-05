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

	// defaultAgentTypeName is the handle of the bootstrap agent type.
	defaultAgentTypeName = "default"

	// agentTypeInOUExistsCode is returned when an organization unit already has an agent type.
	agentTypeInOUExistsCode = "USRS-1021"
	// agentTypeInUseCode is returned when an agent type is deleted while agents are of it.
	agentTypeInUseCode = "USRS-1022"
)

// AgentTypeAPITestSuite covers the /agent-types API, whose contract differs from /user-types in two
// ways: an organization unit can have at most one agent type, and an agent type can be deleted only
// while no agent is of it.
//
// The bootstrap `default` agent type is shared with every other package. This suite therefore
// snapshots it in setup and restores it in teardown, and every test that mutates the schema
// restores it in a defer rather than relying on teardown — testify orders suite methods
// alphabetically, so a lingering edit would silently become another test's precondition. Agent
// types a test creates are its own, in organization units it creates, and are deleted by the test.
type AgentTypeAPITestSuite struct {
	suite.Suite
	snapshot *testutils.AgentTypeSnapshot
	ouID     string
}

func TestAgentTypeAPITestSuite(t *testing.T) {
	suite.Run(t, new(AgentTypeAPITestSuite))
}

func (s *AgentTypeAPITestSuite) SetupSuite() {
	// Require the bootstrap agent type to exist rather than creating it: an absent `default` is a
	// broken environment rather than something a test suite should paper over.
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

// getAgentType fetches an agent type from the detail endpoint, which is the only endpoint that
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

// putSchema replaces the `default` agent type's schema, keeping its handle, display name and OU.
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

// createOU creates an organization unit for one test. The caller deletes it.
func (s *AgentTypeAPITestSuite) createOU(handle string) string {
	s.T().Helper()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      handle,
		Name:        handle,
		Description: "OU holding an agent type created by the agent type API suite",
	})
	s.Require().NoError(err)
	return ouID
}

// deleteOU removes an organization unit a test created.
func (s *AgentTypeAPITestSuite) deleteOU(ouID string) {
	s.T().Helper()
	s.NoError(testutils.DeleteOrganizationUnit(ouID), "cleanup: delete OU %s", ouID)
}

// createAgentType creates an agent type the test owns and returns its ID. The caller deletes it.
func (s *AgentTypeAPITestSuite) createAgentType(handle, ouID string) string {
	s.T().Helper()

	id, err := testutils.CreateNewAgentType(testutils.UserType{
		Handle:      handle,
		DisplayName: handle,
		OUID:        ouID,
		Schema:      map[string]interface{}{"name": map[string]interface{}{"type": "string", "required": true}},
	})
	s.Require().NoError(err)
	return id
}

// deleteAgentType removes an agent type a test created.
func (s *AgentTypeAPITestSuite) deleteAgentType(id string) {
	s.T().Helper()
	s.NoError(testutils.DeleteAgentType(id), "cleanup: delete agent type %s", id)
}

// moveAgentType updates an agent type into the given organization unit, keeping its other fields.
func (s *AgentTypeAPITestSuite) moveAgentType(id, ouID string) *http.Response {
	s.T().Helper()

	current := s.getAgentType(id)
	return s.do(http.MethodPut, agentTypeBasePath+"/"+id, AgentTypeRequest{
		DisplayName: current.DisplayName,
		OUID:        ouID,
		Schema:      current.Schema,
	})
}

// agentTypeHandles returns the handles in the current agent type list.
func (s *AgentTypeAPITestSuite) agentTypeHandles() []string {
	s.T().Helper()

	list := s.listAgentTypes()
	handles := make([]string, 0, len(list.Types))
	for _, t := range list.Types {
		handles = append(handles, t.Handle)
	}
	return handles
}

// ---------------------------------------------------------------------------
// Any handle, one agent type per organization unit
// ---------------------------------------------------------------------------

// TestCreateAgentTypeWithAnyHandleInAnotherOU verifies that an agent type other than `default` can
// be created in an organization unit that has no agent type, and that it is persisted there.
func (s *AgentTypeAPITestSuite) TestCreateAgentTypeWithAnyHandleInAnotherOU() {
	ouID := s.createOU("agent-type-api-any-handle-ou")
	defer s.deleteOU(ouID)

	resp := s.do(http.MethodPost, agentTypeBasePath, AgentTypeRequest{
		Handle:      "support-agents",
		DisplayName: "Support Agents",
		OUID:        ouID,
		Schema:      json.RawMessage(`{"name": {"type": "string", "required": true}}`),
	})
	defer closeBody(resp)
	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusCreated, resp.StatusCode, "create failed: %s", string(body))

	var created AgentType
	s.Require().NoError(json.Unmarshal(body, &created))
	defer s.deleteAgentType(created.ID)

	stored := s.getAgentType(created.ID)
	s.Equal("support-agents", stored.Handle)
	s.Equal(ouID, stored.OUID)
	s.Contains(s.agentTypeHandles(), "support-agents")
}

// TestCreateSecondAgentTypeInSameOURejected verifies that an organization unit holds at most one agent
// type: a second create in it is refused and nothing is persisted.
func (s *AgentTypeAPITestSuite) TestCreateSecondAgentTypeInSameOURejected() {
	ouID := s.createOU("agent-type-api-same-ou")
	defer s.deleteOU(ouID)
	firstID := s.createAgentType("same-ou-first", ouID)
	defer s.deleteAgentType(firstID)

	resp := s.do(http.MethodPost, agentTypeBasePath, AgentTypeRequest{
		Handle:      "same-ou-second",
		DisplayName: "Same OU Second",
		OUID:        ouID,
		Schema:      json.RawMessage(`{"name": {"type": "string"}}`),
	})
	defer closeBody(resp)

	s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
	s.decodeError(resp, agentTypeInOUExistsCode)
	s.NotContains(s.agentTypeHandles(), "same-ou-second", "a rejected create must not persist a type")
}

// TestCreateAgentTypeInDefaultAgentTypeOURejected verifies the rule holds for the organization unit of
// the bootstrap agent type too.
func (s *AgentTypeAPITestSuite) TestCreateAgentTypeInDefaultAgentTypeOURejected() {
	resp := s.do(http.MethodPost, agentTypeBasePath, AgentTypeRequest{
		Handle:      "beside-default",
		DisplayName: "Beside Default",
		OUID:        s.snapshot.OUID,
		Schema:      json.RawMessage(`{"name": {"type": "string"}}`),
	})
	defer closeBody(resp)

	s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
	s.decodeError(resp, agentTypeInOUExistsCode)
	s.NotContains(s.agentTypeHandles(), "beside-default")
}

// TestUpdateMovingAgentTypeIntoOccupiedOURejected verifies that an update cannot move an agent type
// into an organization unit that already has another agent type, and that the type stays where it was.
func (s *AgentTypeAPITestSuite) TestUpdateMovingAgentTypeIntoOccupiedOURejected() {
	occupiedOUID := s.createOU("agent-type-api-occupied-ou")
	defer s.deleteOU(occupiedOUID)
	movingOUID := s.createOU("agent-type-api-moving-ou")
	defer s.deleteOU(movingOUID)
	occupantID := s.createAgentType("occupant-agents", occupiedOUID)
	defer s.deleteAgentType(occupantID)
	movingID := s.createAgentType("moving-agents", movingOUID)
	defer s.deleteAgentType(movingID)

	resp := s.moveAgentType(movingID, occupiedOUID)
	defer closeBody(resp)

	s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
	s.decodeError(resp, agentTypeInOUExistsCode)
	s.Equal(movingOUID, s.getAgentType(movingID).OUID, "a rejected move must not change the stored OU")
}

// TestUpdateMovingAgentTypeIntoEmptyOUSucceeds is the control for the rejected move: an organization
// unit without an agent type accepts one.
func (s *AgentTypeAPITestSuite) TestUpdateMovingAgentTypeIntoEmptyOUSucceeds() {
	fromOUID := s.createOU("agent-type-api-move-from-ou")
	defer s.deleteOU(fromOUID)
	toOUID := s.createOU("agent-type-api-move-to-ou")
	defer s.deleteOU(toOUID)
	agentTypeID := s.createAgentType("relocated-agents", fromOUID)
	defer s.deleteAgentType(agentTypeID)

	resp := s.moveAgentType(agentTypeID, toOUID)
	defer closeBody(resp)
	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "move failed: %s", string(body))

	s.Equal(toOUID, s.getAgentType(agentTypeID).OUID)
}

// ---------------------------------------------------------------------------
// The handle cannot be changed
// ---------------------------------------------------------------------------

// TestChangeDefaultAgentTypeHandleRejected verifies that an agent type's handle cannot be changed,
// and that the stored handle is unchanged.
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
	s.decodeError(resp, "USRS-1017")

	s.Equal(defaultAgentTypeName, s.getAgentType(s.snapshot.ID).Handle,
		"a rejected handle change must not change the stored handle")
}

// TestCreateDuplicateDefaultAgentTypeRejected verifies that a second `default` agent type is
// refused as a handle conflict, even in an organization unit without an agent type.
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
// Deletion
// ---------------------------------------------------------------------------

// TestDeleteUnusedAgentTypeSucceeds verifies that an agent type no agent is of can be deleted.
func (s *AgentTypeAPITestSuite) TestDeleteUnusedAgentTypeSucceeds() {
	ouID := s.createOU("agent-type-api-delete-ou")
	defer s.deleteOU(ouID)
	agentTypeID := s.createAgentType("short-lived-agents", ouID)

	resp := s.do(http.MethodDelete, agentTypeBasePath+"/"+agentTypeID, nil)
	defer closeBody(resp)
	s.Require().Equal(http.StatusNoContent, resp.StatusCode)

	getResp := s.do(http.MethodGet, agentTypeBasePath+"/"+agentTypeID, nil)
	defer closeBody(getResp)
	s.Equal(http.StatusNotFound, getResp.StatusCode, "a deleted agent type must not be readable")
}

// TestDeleteAgentTypeInUseRejected verifies that an agent type cannot be deleted while an agent is of
// it, and can be once that agent is gone.
func (s *AgentTypeAPITestSuite) TestDeleteAgentTypeInUseRejected() {
	ouID := s.createOU("agent-type-api-in-use-ou")
	defer s.deleteOU(ouID)
	agentTypeID := s.createAgentType("busy-agents", ouID)
	agentID := s.createAgent(Agent{
		Type:       "busy-agents",
		OUID:       ouID,
		Attributes: json.RawMessage(`{"name": "agent-type-in-use-agent"}`),
	})
	agentDeleted := false
	defer func() {
		if !agentDeleted {
			s.deleteAgent(agentID)
		}
		closeBody(s.do(http.MethodDelete, agentTypeBasePath+"/"+agentTypeID, nil))
	}()

	resp := s.do(http.MethodDelete, agentTypeBasePath+"/"+agentTypeID, nil)
	defer closeBody(resp)
	s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
	s.decodeError(resp, agentTypeInUseCode)
	s.Equal("busy-agents", s.getAgentType(agentTypeID).Handle, "a refused delete must keep the agent type")

	s.deleteAgent(agentID)
	agentDeleted = true

	retry := s.do(http.MethodDelete, agentTypeBasePath+"/"+agentTypeID, nil)
	defer closeBody(retry)
	s.Equal(http.StatusNoContent, retry.StatusCode, "the agent type must be deletable once no agent is of it")
}

// requireDeleteRefusedUntil asserts that the agent type cannot be deleted while a reference holds it, and
// that it can be once release removes the reference.
func (s *AgentTypeAPITestSuite) requireDeleteRefusedUntil(agentTypeID string, release func()) {
	s.T().Helper()

	resp := s.do(http.MethodDelete, agentTypeBasePath+"/"+agentTypeID, nil)
	defer closeBody(resp)
	s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
	s.decodeError(resp, agentTypeInUseCode)

	release()

	retry := s.do(http.MethodDelete, agentTypeBasePath+"/"+agentTypeID, nil)
	defer closeBody(retry)
	s.Equal(http.StatusNoContent, retry.StatusCode, "the agent type must be deletable once nothing lists it")
}

// TestDeleteAgentTypeListedByApplicationRejected verifies that an application allowing an agent type
// blocks its deletion: otherwise the handle could be recreated elsewhere and inherit the allowance.
func (s *AgentTypeAPITestSuite) TestDeleteAgentTypeListedByApplicationRejected() {
	ouID := s.createOU("agent-type-api-app-ref-ou")
	defer s.deleteOU(ouID)
	agentTypeID := s.createAgentType("app-allowed-agents", ouID)
	defer func() { closeBody(s.do(http.MethodDelete, agentTypeBasePath+"/"+agentTypeID, nil)) }()

	appID, err := testutils.CreateApplication(testutils.Application{
		Name: "Agent Type Reference App", OUID: ouID, Type: "fullstack",
		AllowedAgentTypes: []string{"app-allowed-agents"},
	})
	s.Require().NoError(err)
	appDeleted := false
	defer func() {
		if !appDeleted {
			s.NoError(testutils.DeleteApplication(appID))
		}
	}()

	s.requireDeleteRefusedUntil(agentTypeID, func() {
		s.Require().NoError(testutils.DeleteApplication(appID))
		appDeleted = true
	})
}

// TestDeleteAgentTypeListedByAgentRejected verifies that an agent allowing an agent type to sign in to it
// blocks the type's deletion, even though the agent is of another type.
func (s *AgentTypeAPITestSuite) TestDeleteAgentTypeListedByAgentRejected() {
	listedOUID := s.createOU("agent-type-api-agent-ref-ou")
	defer s.deleteOU(listedOUID)
	listedTypeID := s.createAgentType("agent-allowed-agents", listedOUID)
	defer func() { closeBody(s.do(http.MethodDelete, agentTypeBasePath+"/"+listedTypeID, nil)) }()

	holderOUID := s.createOU("agent-type-api-agent-holder-ou")
	defer s.deleteOU(holderOUID)
	holderTypeID := s.createAgentType("allowing-agents", holderOUID)
	defer s.deleteAgentType(holderTypeID)

	agentID, err := testutils.CreateAgent(testutils.Agent{
		Type: "allowing-agents", OUID: holderOUID,
		Attributes:        map[string]interface{}{"name": "agent-type-allowing-agent"},
		AllowedAgentTypes: []string{"agent-allowed-agents"},
	})
	s.Require().NoError(err)
	agentDeleted := false
	defer func() {
		if !agentDeleted {
			s.deleteAgent(agentID)
		}
	}()

	s.requireDeleteRefusedUntil(listedTypeID, func() {
		s.deleteAgent(agentID)
		agentDeleted = true
	})
}

// TestDeleteAgentTypeListedByFlowRejected verifies that a flow whose AgentTypeResolver node narrows its
// candidates to an agent type blocks the type's deletion.
func (s *AgentTypeAPITestSuite) TestDeleteAgentTypeListedByFlowRejected() {
	ouID := s.createOU("agent-type-api-flow-ref-ou")
	defer s.deleteOU(ouID)
	agentTypeID := s.createAgentType("flow-allowed-agents", ouID)
	defer func() { closeBody(s.do(http.MethodDelete, agentTypeBasePath+"/"+agentTypeID, nil)) }()

	flowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "Agent Type Reference Flow",
		Handle:   "agent-type-reference-flow",
		FlowType: "ADMINISTRATION",
		OUID:     ouID,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "resolve"},
			{
				"id":           "resolve",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "AgentTypeResolver"},
				"properties":   map[string]interface{}{"allowedAgentTypes": []string{"flow-allowed-agents"}},
				"onSuccess":    "end",
				"onIncomplete": "prompt_type",
			},
			{
				"id":   "prompt_type",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{{
					"inputs": []map[string]interface{}{
						{"ref": "agenttype_input", "identifier": "agentType", "type": "SELECT", "required": true},
					},
					"action": map[string]interface{}{"ref": "action_type", "nextNode": "resolve"},
				}},
			},
			{"id": "end", "type": "END"},
		},
	})
	s.Require().NoError(err)
	flowDeleted := false
	defer func() {
		if !flowDeleted {
			s.NoError(testutils.DeleteFlow(flowID))
		}
	}()

	s.requireDeleteRefusedUntil(agentTypeID, func() {
		s.Require().NoError(testutils.DeleteFlow(flowID))
		flowDeleted = true
	})
}

// TestDeleteAgentTypeNotBlockedByAnyAgentTypeApplication verifies that an application admitting any agent
// type names no agent type, so it does not block a delete.
func (s *AgentTypeAPITestSuite) TestDeleteAgentTypeNotBlockedByAnyAgentTypeApplication() {
	ouID := s.createOU("agent-type-api-any-ref-ou")
	defer s.deleteOU(ouID)
	agentTypeID := s.createAgentType("any-allowed-agents", ouID)

	appID, err := testutils.CreateApplication(testutils.Application{
		Name: "Any Agent Type Reference App", OUID: ouID, Type: "fullstack", AllowedAgentTypes: []string{"*"},
	})
	s.Require().NoError(err)
	defer func() { s.NoError(testutils.DeleteApplication(appID)) }()

	resp := s.do(http.MethodDelete, agentTypeBasePath+"/"+agentTypeID, nil)
	defer closeBody(resp)
	s.Equal(http.StatusNoContent, resp.StatusCode)
}

// TestDeleteOUAfterDeletingItsAgentType verifies that an agent type blocks the deletion of its
// organization unit only until the agent type itself is deleted.
func (s *AgentTypeAPITestSuite) TestDeleteOUAfterDeletingItsAgentType() {
	ouID := s.createOU("agent-type-api-ou-delete")
	ouDeleted := false
	defer func() {
		if !ouDeleted {
			s.deleteOU(ouID)
		}
	}()
	agentTypeID := s.createAgentType("ou-bound-agents", ouID)

	blocked := s.do(http.MethodDelete, "/organization-units/"+ouID, nil)
	defer closeBody(blocked)
	s.Require().Equal(http.StatusBadRequest, blocked.StatusCode, "the agent type must block the OU delete")

	s.deleteAgentType(agentTypeID)

	resp := s.do(http.MethodDelete, "/organization-units/"+ouID, nil)
	defer closeBody(resp)
	s.Require().Equal(http.StatusNoContent, resp.StatusCode, "the OU must be deletable once its agent type is gone")
	ouDeleted = true
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
// Scenario 31: the list holds the bootstrap agent type
// ---------------------------------------------------------------------------

// TestListAgentTypesIncludesTheDefault verifies the list endpoint reports agent types under the
// `types` key, including the bootstrap `default` one.
func (s *AgentTypeAPITestSuite) TestListAgentTypesIncludesTheDefault() {
	list := s.listAgentTypes()

	s.GreaterOrEqual(list.TotalResults, 1)
	s.Equal(len(list.Types), list.Count)
	var found *AgentTypeListItem
	for i := range list.Types {
		if list.Types[i].Handle == defaultAgentTypeName {
			found = &list.Types[i]
		}
	}
	s.Require().NotNil(found, "the default agent type must be listed")
	s.Equal(s.snapshot.DisplayName, found.DisplayName)
	s.Equal(s.snapshot.ID, found.ID)
}

// ---------------------------------------------------------------------------
// Scenario 32 — schema edits persist
// ---------------------------------------------------------------------------

// TestUpdateAgentTypeSchemaPersists verifies that editing an agent type's schema persists, and that
// the edit is readable back from the detail endpoint.
func (s *AgentTypeAPITestSuite) TestUpdateAgentTypeSchemaPersists() {
	defer s.restoreSchema()

	s.putSchema(map[string]interface{}{
		"name":        map[string]interface{}{"type": "string", "required": true, "unique": true},
		"description": map[string]interface{}{"type": "string"},
		"costCentre":  map[string]interface{}{"type": "string"},
	})

	var stored map[string]interface{}
	s.Require().NoError(json.Unmarshal(s.getAgentType(s.snapshot.ID).Schema, &stored))

	s.Contains(stored, "costCentre", "the edited attribute must be persisted")
	s.Contains(stored, "description")
	s.Len(stored, 3, "the update replaces the schema rather than merging into it")
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
		"name":         map[string]interface{}{"type": "string", "required": true, "unique": true},
		"serialNumber": map[string]interface{}{"type": "string", "unique": true},
	})

	firstID := s.createAgent(Agent{
		Type:       defaultAgentTypeName,
		OUID:       s.snapshot.OUID,
		Attributes: json.RawMessage(`{"name": "agent-type-unique-first", "serialNumber": "SN-AGENTTYPE-0001"}`),
	})
	defer func() { s.deleteAgent(firstID) }()

	resp := s.do(http.MethodPost, agentBasePath, Agent{
		Type:       defaultAgentTypeName,
		OUID:       s.snapshot.OUID,
		Attributes: json.RawMessage(`{"name": "agent-type-unique-second", "serialNumber": "SN-AGENTTYPE-0001"}`),
	})
	defer closeBody(resp)

	s.Require().Equal(http.StatusConflict, resp.StatusCode)
	s.decodeError(resp, "AGT-1014")

	holders := s.agentsWithSerial("SN-AGENTTYPE-0001")
	s.Require().Len(holders, 1, "the rejected agent must not be persisted")
	s.Equal("agent-type-unique-first", holders[0].AttrName(),
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
