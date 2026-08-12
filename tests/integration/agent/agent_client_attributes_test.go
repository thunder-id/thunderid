// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"encoding/json"
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
	client           *http.Client
	ouID             string
	agentSchemaID    string
	entityTypeID     string
	ownerUserID      string
	resourceServerID string
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

	schemaID, err := testutils.CreateAgentType(testutils.UserType{
		Name: "default",
		OUID: s.ouID,
		Schema: map[string]interface{}{
			"modelProvider": map[string]interface{}{"type": "string"},
			"description":   map[string]interface{}{"type": "string"},
		},
	})
	s.Require().NoError(err)
	s.agentSchemaID = schemaID

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
		_ = testutils.DeleteResourceServer(s.resourceServerID)
	}
	if s.ownerUserID != "" {
		_ = testutils.DeleteUser(s.ownerUserID)
	}
	if s.entityTypeID != "" {
		_ = testutils.DeleteUserType(s.entityTypeID)
	}
	if s.agentSchemaID != "" {
		_ = testutils.DeleteAgentType(s.agentSchemaID)
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
							ClientConfig: &AccessTokenSubConfig{Attributes: clientAttributes},
						},
					},
				},
			},
		},
	})
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
