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
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	agentClientAttrsClientID     = "agent_client_attrs_test_client"
	agentClientAttrsClientSecret = "agent_client_attrs_test_secret"
	agentClientAttrsRSIdentifier = "https://agent-client-attrs.example.com"
)

// AgentClientAttributesTestSuite verifies that client_credentials access tokens issued for
// an agent surface client-scoped claims (schema attributes, system attributes name/owner,
// ouId, groups, roles) selected by the agent's token.accessToken.clientConfig.attributes
// allow-list.
type AgentClientAttributesTestSuite struct {
	suite.Suite
	client            *http.Client
	ouID              string
	agentTypeSnapshot *testutils.AgentTypeSnapshot
	entityTypeID      string
	ownerUserID       string
	resourceServerID  string
}

// TestAgentClientAttributesTestSuite runs the AgentClientAttributesTestSuite.
func TestAgentClientAttributesTestSuite(t *testing.T) {
	suite.Run(t, new(AgentClientAttributesTestSuite))
}

// SetupSuite creates the shared organization unit, agent schema, owner user, and resource
// server for the suite.
func (s *AgentClientAttributesTestSuite) SetupSuite() {
	s.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "agent-client-attrs-ou",
		Name:        "Agent Client Attributes OU",
		Description: "Organization unit for agent client-attribute integration tests",
	})
	s.Require().NoError(err)
	s.ouID = ouID

	// The `default` agent type is a singleton shared with every other suite. Snapshot it before
	// pointing it at this suite's OU, so teardown can put it back before that OU is deleted.
	snapshot, err := testutils.SnapshotAgentType()
	s.Require().NoError(err)
	s.agentTypeSnapshot = snapshot

	_, err = testutils.CreateAgentType(testutils.UserType{
		Name: "default",
		OUID: s.ouID,
		Schema: map[string]interface{}{
			"modelProvider": map[string]interface{}{"type": "string"},
			"description":   map[string]interface{}{"type": "string"},
			// Named after a reserved claim, so a test can store a conflicting value.
			"sub_type": map[string]interface{}{"type": "string"},
		},
	})
	s.Require().NoError(err)

	entityTypeID, err := testutils.CreateUserType(testutils.UserType{
		Name: "agent-client-attrs-owner",
		OUID: s.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	s.Require().NoError(err)
	s.entityTypeID = entityTypeID

	attributesJSON, err := json.Marshal(map[string]interface{}{
		"username": "agent-client-attrs-owner-user",
		"password": "OwnerUserPass1!",
	})
	s.Require().NoError(err)
	ownerUserID, err := testutils.CreateUser(testutils.User{
		Type:       "agent-client-attrs-owner",
		OUID:       s.ouID,
		Attributes: attributesJSON,
	})
	s.Require().NoError(err)
	s.ownerUserID = ownerUserID

	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Agent Client Attributes API",
		Description: "Resource server for agent client-attribute testing",
		Identifier:  agentClientAttrsRSIdentifier,
		OUID:        s.ouID,
	}, []testutils.Action{})
	s.Require().NoError(err)
	s.resourceServerID = rsID
}

// TearDownSuite deletes the shared resources created in SetupSuite.
func (s *AgentClientAttributesTestSuite) TearDownSuite() {
	if s.resourceServerID != "" {
		s.NoError(testutils.DeleteResourceServerWithChildren(s.resourceServerID),
			"teardown: delete resource server and its actions")
	}
	if s.ownerUserID != "" {
		_ = testutils.DeleteUser(s.ownerUserID)
	}
	if s.entityTypeID != "" {
		_ = testutils.DeleteUserType(s.entityTypeID)
	}
	// Restore the shared agent type before deleting the OU it points at, or the singleton is left
	// referencing a deleted OU and a later suite's restore fails.
	if s.agentTypeSnapshot != nil {
		if err := testutils.RestoreAgentType(s.agentTypeSnapshot); err != nil {
			s.T().Errorf("teardown: failed to restore the default agent type: %v", err)
		}
	}
	if s.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(s.ouID)
	}
}

// createAgentWithAttributes creates an agent with the given schema attributes, owner, and
// clientConfig.attributes allow-list.
func (s *AgentClientAttributesTestSuite) createAgentWithAttributes(
	clientID string, clientAttributes []string, attributes json.RawMessage, owner string,
) (string, error) {
	return s.createAgentWithClientConfig(clientID, &AccessTokenSubConfig{Attributes: clientAttributes},
		attributes, owner)
}

// createAgentWithClientConfig creates an agent with the given token.accessToken.clientConfig block.
func (s *AgentClientAttributesTestSuite) createAgentWithClientConfig(
	clientID string, clientConfig *AccessTokenSubConfig, attributes json.RawMessage, owner string,
) (string, error) {
	return createAgent(Agent{
		OUID:       s.ouID,
		Type:       "default",
		Name:       "Client Attrs Agent " + clientID,
		Owner:      owner,
		Attributes: attributes,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				Config: &OAuthAgentConfig{
					ClientID:                clientID,
					ClientSecret:            agentClientAttrsClientSecret,
					GrantTypes:              []string{"client_credentials"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Token: &OAuthTokenConfig{
						AccessToken: &AccessTokenConfig{
							ClientConfig: clientConfig,
						},
					},
				},
			},
		},
	})
}

// updateClientAttributes replaces the agent's client-token attribute selection, as the Console does
// when an operator toggles a chip.
func (s *AgentClientAttributesTestSuite) updateClientAttributes(
	agentID, clientID string, attributes []string, attrs json.RawMessage,
) error {
	body := Agent{
		OUID:       s.ouID,
		Type:       "default",
		Name:       "Client Attrs Agent " + clientID,
		Owner:      s.ownerUserID,
		Attributes: attrs,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				Config: &OAuthAgentConfig{
					ClientID:                clientID,
					ClientSecret:            agentClientAttrsClientSecret,
					GrantTypes:              []string{"client_credentials"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Token: &OAuthTokenConfig{
						AccessToken: &AccessTokenConfig{
							ClientConfig: &AccessTokenSubConfig{Attributes: attributes},
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, testServerURL+"/agents/"+agentID, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 200, got %d. Response: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// requestToken performs a client_credentials token request for the given client ID.
func (s *AgentClientAttributesTestSuite) requestToken(clientID string) (int, map[string]interface{}) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("resource", agentClientAttrsRSIdentifier)

	req, err := http.NewRequest("POST", testServerURL+"/oauth2/token", strings.NewReader(form.Encode()))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, agentClientAttrsClientSecret)

	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	var respBody map[string]interface{}
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&respBody))
	return resp.StatusCode, respBody
}

// TestAgentClientAttrs_SchemaAndSystemAndGroupsAndRoles verifies that an agent's schema
// attribute (modelProvider), system attributes (name, owner), ouId, groups, and roles are
// all present in the client_credentials access token when allow-listed.
func (s *AgentClientAttributesTestSuite) TestAgentClientAttrs_SchemaAndSystemAndGroupsAndRoles() {
	clientID := agentClientAttrsClientID + "_full"
	attrs, err := json.Marshal(map[string]interface{}{"modelProvider": "anthropic", "description": "d"})
	s.Require().NoError(err)

	agentID, err := s.createAgentWithAttributes(clientID,
		[]string{"modelProvider", "name", "owner", "ouId", "groups", "roles"}, attrs, s.ownerUserID)
	s.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	directRoleID, err := testutils.CreateRole(testutils.Role{
		Name:        "Agent Client Attrs Direct Role",
		Description: "Role assigned directly to the agent",
		OUID:        s.ouID,
		Assignments: []testutils.Assignment{{ID: agentID, Type: "agent"}},
	})
	s.Require().NoError(err)
	defer func() { _ = testutils.DeleteRole(directRoleID) }()

	groupID, err := testutils.CreateGroup(testutils.Group{
		Name:    "Agent Client Attrs Group",
		OUID:    s.ouID,
		Members: []testutils.Member{{Id: agentID, Type: "agent"}},
	})
	s.Require().NoError(err)
	defer func() { _ = testutils.DeleteGroup(groupID) }()

	groupRoleID, err := testutils.CreateRole(testutils.Role{
		Name:        "Agent Client Attrs Group Role",
		Description: "Role assigned to the group containing the agent",
		OUID:        s.ouID,
		Assignments: []testutils.Assignment{{ID: groupID, Type: "group"}},
	})
	s.Require().NoError(err)
	defer func() { _ = testutils.DeleteRole(groupRoleID) }()

	status, body := s.requestToken(clientID)
	s.Require().Equal(http.StatusOK, status, "CC token request must succeed: %v", body)
	token, ok := body["access_token"].(string)
	s.Require().True(ok, "Response should contain access_token")

	claims, err := testutils.DecodeJWT(token)
	s.Require().NoError(err)

	s.Assert().Equal("anthropic", claims.Additional["modelProvider"], "agent schema attribute should be surfaced")
	s.Assert().Equal("Client Attrs Agent "+clientID, claims.Additional["name"], "agent system attribute name should be surfaced")
	s.Assert().Equal(s.ownerUserID, claims.Additional["owner"], "agent system attribute owner should be surfaced")
	s.Assert().Equal(s.ouID, claims.Additional["ouId"])
	s.Assert().NotContains(claims.Additional, "description", "non-allow-listed schema attribute must be excluded")

	groups, ok := claims.Additional["groups"].([]interface{})
	s.Require().True(ok, "groups claim should be present as an array")
	s.Assert().Contains(groups, "Agent Client Attrs Group")

	roles, ok := claims.Additional["roles"].([]interface{})
	s.Require().True(ok, "roles claim should be present as an array")
	s.Assert().ElementsMatch(
		[]interface{}{"Agent Client Attrs Direct Role", "Agent Client Attrs Group Role"}, roles)
}

// TestAgentClientAttrs_SubTypeAgent verifies that an agent's client_credentials token carries
// sub_type=agent. The agent stores a conflicting sub_type attribute and allow-lists it, so the token
// value also proves the claim is server-set and cannot be spoofed.
func (s *AgentClientAttributesTestSuite) TestAgentClientAttrs_SubTypeAgent() {
	clientID := agentClientAttrsClientID + "_subtype"
	attrs, err := json.Marshal(map[string]interface{}{
		"modelProvider": "anthropic",
		"sub_type":      "application",
	})
	s.Require().NoError(err)

	agentID, err := s.createAgentWithAttributes(
		clientID, []string{"sub_type", "modelProvider"}, attrs, s.ownerUserID)
	s.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	status, body := s.requestToken(clientID)
	s.Require().Equal(http.StatusOK, status, "CC token request must succeed: %v", body)
	token, ok := body["access_token"].(string)
	s.Require().True(ok)

	claims, err := testutils.DecodeJWT(token)
	s.Require().NoError(err)

	s.Assert().Equal("agent", claims.Additional["sub_type"],
		"a configured sub_type attribute must not override the server-asserted identity class")
	s.Assert().Equal("anthropic", claims.Additional["modelProvider"])
}

// TestAgentClientAttrs_SubTypeSeededOnCreate verifies that an agent created with no client token
// configuration still receives sub_type, because creation selects the claim for it.
func (s *AgentClientAttributesTestSuite) TestAgentClientAttrs_SubTypeSeededOnCreate() {
	clientID := agentClientAttrsClientID + "_subtype_seeded"
	agentID, err := s.createAgentWithClientConfig(clientID, nil, nil, s.ownerUserID)
	s.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	status, body := s.requestToken(clientID)
	s.Require().Equal(http.StatusOK, status, "CC token request must succeed: %v", body)
	token, ok := body["access_token"].(string)
	s.Require().True(ok)

	claims, err := testutils.DecodeJWT(token)
	s.Require().NoError(err)

	s.Assert().Equal("agent", claims.Additional["sub_type"],
		"creation must select sub_type so a new agent is identifiable without being reconfigured")
}

// TestAgentClientAttrs_SubTypeRemovedByUpdate verifies that dropping sub_type from an agent's selection
// stops the claim. Removal is an update, which must be authoritative: re-seeding would make it
// unremovable.
func (s *AgentClientAttributesTestSuite) TestAgentClientAttrs_SubTypeRemovedByUpdate() {
	clientID := agentClientAttrsClientID + "_subtype_off"
	attrs, err := json.Marshal(map[string]interface{}{"modelProvider": "anthropic"})
	s.Require().NoError(err)

	agentID, err := s.createAgentWithClientConfig(clientID, nil, attrs, s.ownerUserID)
	s.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	status, body := s.requestToken(clientID)
	s.Require().Equal(http.StatusOK, status, "CC token request must succeed: %v", body)
	token, ok := body["access_token"].(string)
	s.Require().True(ok)
	claims, err := testutils.DecodeJWT(token)
	s.Require().NoError(err)
	s.Require().Equal("agent", claims.Additional["sub_type"],
		"creation must select the claim, otherwise this test cannot show it being removed")

	s.Require().NoError(s.updateClientAttributes(agentID, clientID, []string{"modelProvider"}, attrs))

	status, body = s.requestToken(clientID)
	s.Require().Equal(http.StatusOK, status, "CC token request must succeed: %v", body)
	token, ok = body["access_token"].(string)
	s.Require().True(ok)
	claims, err = testutils.DecodeJWT(token)
	s.Require().NoError(err)

	s.Assert().NotContains(claims.Additional, "sub_type",
		"sub_type must stay out once it is removed from the selection")
	s.Assert().Equal("anthropic", claims.Additional["modelProvider"],
		"the rest of the updated selection must still be surfaced")
}

// TestAgentClientAttrs_PartialAllowList verifies that only the allow-listed attributes are
// included, even when the agent has other resolvable attributes (schema, system, OU).
func (s *AgentClientAttributesTestSuite) TestAgentClientAttrs_PartialAllowList() {
	clientID := agentClientAttrsClientID + "_partial"
	attrs, err := json.Marshal(map[string]interface{}{"modelProvider": "anthropic"})
	s.Require().NoError(err)

	agentID, err := s.createAgentWithAttributes(clientID, []string{"modelProvider"}, attrs, s.ownerUserID)
	s.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	status, body := s.requestToken(clientID)
	s.Require().Equal(http.StatusOK, status, "CC token request must succeed: %v", body)
	token, ok := body["access_token"].(string)
	s.Require().True(ok)

	claims, err := testutils.DecodeJWT(token)
	s.Require().NoError(err)

	s.Assert().Equal("anthropic", claims.Additional["modelProvider"])
	s.Assert().NotContains(claims.Additional, "name", "name must be excluded when not allow-listed")
	s.Assert().NotContains(claims.Additional, "owner", "owner must be excluded when not allow-listed")
	s.Assert().NotContains(claims.Additional, "ouId", "ouId must be excluded when not allow-listed")
}
