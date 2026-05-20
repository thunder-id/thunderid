/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

// ============================================================================
// Constants
// ============================================================================

const (
	oauthFlowsRedirectURI  = "https://localhost:3000"
	oauthFlowsTestUsername = "agent-oauth-flow-user"
	oauthFlowsTestPassword = "AgentOAuthFlows1!"

	ccAgentClientID     = "cc_agent_authz_client"
	ccAgentClientSecret = "cc_agent_authz_secret"

	agentTEClientID     = "agent_token_exchange_client"
	agentTEClientSecret = "agent_token_exchange_secret"
	agentTEUsername     = "agent_te_test_user"
	agentTEPassword     = "AgentTE_Pass1!"
)

// ============================================================================
// AgentOAuthFlowsTestSuite — grant-type flows (CC, auth code, PKCE, refresh)
// ============================================================================

// AgentOAuthFlowsTestSuite covers OAuth grant type flows for agents:
// client_secret_post CC, invalid credentials, auth code, PKCE, refresh token,
// duplicate clientID, and transitioning an entity-only agent to OAuth on update.
type AgentOAuthFlowsTestSuite struct {
	suite.Suite
	ouID         string
	schemaID     string
	entityTypeID string
	userID       string
	authFlowID   string
}

func TestAgentOAuthFlowsTestSuite(t *testing.T) {
	suite.Run(t, new(AgentOAuthFlowsTestSuite))
}

func (ts *AgentOAuthFlowsTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "agent-oauth-flows-ou",
		Name:        "Agent OAuth Flows OU",
		Description: "Organization unit for agent OAuth flow tests",
	})
	ts.Require().NoError(err, "Failed to create OU")
	ts.ouID = ouID

	schemaID, err := testutils.CreateAgentType(testutils.UserType{
		Name: "default",
		OUID: ts.ouID,
		Schema: map[string]interface{}{
			"description": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "Failed to create agent schema")
	ts.schemaID = schemaID

	entityTypeID, err := testutils.CreateUserType(testutils.UserType{
		Name: "agent-oauth-flow-person",
		OUID: ts.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	ts.Require().NoError(err, "Failed to create user type")
	ts.entityTypeID = entityTypeID

	attributesJSON, err := json.Marshal(map[string]interface{}{
		"username": oauthFlowsTestUsername,
		"password": oauthFlowsTestPassword,
	})
	ts.Require().NoError(err)
	userID, err := testutils.CreateUser(testutils.User{
		Type:       "agent-oauth-flow-person",
		OUID:       ts.ouID,
		Attributes: attributesJSON,
	})
	ts.Require().NoError(err, "Failed to create test user")
	ts.userID = userID

	flowID, err := testutils.GetFlowIDByHandle("default-basic-flow", "AUTHENTICATION")
	ts.Require().NoError(err, "Failed to get default auth flow ID")
	ts.authFlowID = flowID
}

func (ts *AgentOAuthFlowsTestSuite) TearDownSuite() {
	if ts.userID != "" {
		_ = testutils.DeleteUser(ts.userID)
	}
	if ts.entityTypeID != "" {
		_ = testutils.DeleteUserType(ts.entityTypeID)
	}
	if ts.schemaID != "" {
		_ = testutils.DeleteAgentType(ts.schemaID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// --- CC token issuance ---

// TestAgentCC_TokenIssuance verifies that a newly created agent with CC grant can obtain a token.
func (ts *AgentOAuthFlowsTestSuite) TestAgentCC_TokenIssuance() {
	agentID, err := createAgent(Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Name:       "cc-token-agent",
		AuthFlowID: ts.authFlowID,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				Config: &OAuthAgentConfig{
					ClientID:                "cc_token_agent_client",
					ClientSecret:            "cc_token_agent_secret",
					GrantTypes:              []string{"client_credentials"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	tokenResult, err := testutils.RequestToken(
		"cc_token_agent_client", "cc_token_agent_secret",
		"", "", "client_credentials",
	)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode,
		"CC token request must succeed: "+string(tokenResult.Body))
	ts.Require().NotNil(tokenResult.Token)
	ts.Assert().NotEmpty(tokenResult.Token.AccessToken)
	ts.Assert().Equal("Bearer", tokenResult.Token.TokenType)
}

// TestAgentCC_ClientSecretPost verifies a CC token can be obtained when credentials
// are passed as form body parameters (client_secret_post) rather than HTTP Basic Auth.
func (ts *AgentOAuthFlowsTestSuite) TestAgentCC_ClientSecretPost() {
	agentID, err := createAgent(Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Name:       "cc-post-agent",
		AuthFlowID: ts.authFlowID,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				Config: &OAuthAgentConfig{
					ClientID:                "cc_post_agent_client",
					ClientSecret:            "cc_post_agent_secret",
					GrantTypes:              []string{"client_credentials"},
					TokenEndpointAuthMethod: "client_secret_post",
				},
			},
		},
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	body := "grant_type=client_credentials&client_id=cc_post_agent_client&client_secret=cc_post_agent_secret"
	result, statusCode, err := agentTokenRequest(body, "", "", true)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, statusCode, "CC with client_secret_post must succeed")
	ts.Assert().NotEmpty(result["access_token"])
}

// TestAgentCC_InvalidCredentials verifies that a CC request with wrong credentials is rejected.
func (ts *AgentOAuthFlowsTestSuite) TestAgentCC_InvalidCredentials() {
	agentID, err := createAgent(Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Name:       "cc-invalid-cred-agent",
		AuthFlowID: ts.authFlowID,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				Config: &OAuthAgentConfig{
					ClientID:                "cc_invalid_agent_client",
					ClientSecret:            "cc_invalid_agent_secret",
					GrantTypes:              []string{"client_credentials"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	tokenResult, err := testutils.RequestToken(
		"cc_invalid_agent_client", "wrong_secret",
		"", "", "client_credentials",
	)
	ts.Require().NoError(err)
	ts.Assert().Equal(http.StatusUnauthorized, tokenResult.StatusCode,
		"CC with wrong credentials must be rejected")
}

// --- auth code grant ---

// TestAgentAuthCode_FullRoundTrip verifies the full authorization code flow for an agent.
func (ts *AgentOAuthFlowsTestSuite) TestAgentAuthCode_FullRoundTrip() {
	agentID, err := createAgent(Agent{
		OUID:             ts.ouID,
		Type:             "default",
		Name:             "authcode-agent",
		AuthFlowID:       ts.authFlowID,
		AllowedUserTypes: []string{"agent-oauth-flow-person"},
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				Config: &OAuthAgentConfig{
					ClientID:                "authcode_agent_client",
					ClientSecret:            "authcode_agent_secret",
					RedirectURIs:            []string{oauthFlowsRedirectURI},
					GrantTypes:              []string{"authorization_code", "refresh_token"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_post",
				},
			},
		},
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	token, err := testutils.ObtainAccessTokenWithPassword(
		"authcode_agent_client", oauthFlowsRedirectURI, "openid",
		oauthFlowsTestUsername, oauthFlowsTestPassword,
		false, "authcode_agent_secret",
	)
	ts.Require().NoError(err, "Auth code full round trip must succeed")
	ts.Assert().NotEmpty(token.AccessToken)
}

// TestAgentAuthCode_WithPKCE verifies the authorization code + PKCE flow for an agent.
func (ts *AgentOAuthFlowsTestSuite) TestAgentAuthCode_WithPKCE() {
	agentID, err := createAgent(Agent{
		OUID:             ts.ouID,
		Type:             "default",
		Name:             "authcode-pkce-agent",
		AuthFlowID:       ts.authFlowID,
		AllowedUserTypes: []string{"agent-oauth-flow-person"},
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				Config: &OAuthAgentConfig{
					ClientID:                "authcode_pkce_agent_client",
					ClientSecret:            "authcode_pkce_agent_secret",
					RedirectURIs:            []string{oauthFlowsRedirectURI},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_post",
					PKCERequired:            true,
				},
			},
		},
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	token, err := testutils.ObtainAccessTokenWithPassword(
		"authcode_pkce_agent_client", oauthFlowsRedirectURI, "openid",
		oauthFlowsTestUsername, oauthFlowsTestPassword,
		true, "authcode_pkce_agent_secret",
	)
	ts.Require().NoError(err, "Auth code with PKCE must succeed")
	ts.Assert().NotEmpty(token.AccessToken)
}

// TestAgentAuthCode_RefreshToken verifies that a refresh token obtained via auth code
// can be used to obtain a fresh access token.
func (ts *AgentOAuthFlowsTestSuite) TestAgentAuthCode_RefreshToken() {
	agentID, err := createAgent(Agent{
		OUID:             ts.ouID,
		Type:             "default",
		Name:             "authcode-refresh-agent",
		AuthFlowID:       ts.authFlowID,
		AllowedUserTypes: []string{"agent-oauth-flow-person"},
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				Config: &OAuthAgentConfig{
					ClientID:                "authcode_refresh_agent_client",
					ClientSecret:            "authcode_refresh_agent_secret",
					RedirectURIs:            []string{oauthFlowsRedirectURI},
					GrantTypes:              []string{"authorization_code", "refresh_token"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_post",
				},
			},
		},
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	token, err := testutils.ObtainAccessTokenWithPassword(
		"authcode_refresh_agent_client", oauthFlowsRedirectURI, "openid",
		oauthFlowsTestUsername, oauthFlowsTestPassword,
		false, "authcode_refresh_agent_secret",
	)
	ts.Require().NoError(err, "Initial auth code flow must succeed")
	ts.Require().NotEmpty(token.RefreshToken, "Refresh token must be returned")

	refreshed, err := testutils.RefreshAccessTokenWithClientCredentialsInBody(
		"authcode_refresh_agent_client", "authcode_refresh_agent_secret",
		token.RefreshToken,
	)
	ts.Require().NoError(err, "Refresh token request must succeed")
	ts.Assert().NotEmpty(refreshed.AccessToken)
}

// --- duplicate clientID ---

// TestAgentCreate_DuplicateClientID verifies that creating an agent with a taken clientId returns 409.
func (ts *AgentOAuthFlowsTestSuite) TestAgentCreate_DuplicateClientID() {
	firstID, err := createAgent(Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Name:       "dup-client-first-agent",
		AuthFlowID: ts.authFlowID,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				Config: &OAuthAgentConfig{
					ClientID:                "dup_clientid_agent",
					ClientSecret:            "dup_clientid_secret",
					GrantTypes:              []string{"client_credentials"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(firstID) }()

	resp, err := doPost(testServerURL+agentBasePath, Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Name:       "dup-client-second-agent",
		AuthFlowID: ts.authFlowID,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				Config: &OAuthAgentConfig{
					ClientID:                "dup_clientid_agent",
					ClientSecret:            "dup_clientid_secret2",
					GrantTypes:              []string{"client_credentials"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	})
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusConflict, resp.StatusCode,
		"Duplicate clientId must return 409: "+readBody(resp))
}

// TestAgentUpdate_DuplicateClientID verifies that updating an agent to use a taken clientId returns 409.
func (ts *AgentOAuthFlowsTestSuite) TestAgentUpdate_DuplicateClientID() {
	firstID, err := createAgent(Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Name:       "dup-upd-first-agent",
		AuthFlowID: ts.authFlowID,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				Config: &OAuthAgentConfig{
					ClientID:                "dup_upd_first_clientid",
					ClientSecret:            "dup_upd_secret",
					GrantTypes:              []string{"client_credentials"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(firstID) }()

	secondID, err := createAgent(Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Name:       "dup-upd-second-agent",
		AuthFlowID: ts.authFlowID,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				Config: &OAuthAgentConfig{
					ClientID:                "dup_upd_second_clientid",
					ClientSecret:            "dup_upd_secret2",
					GrantTypes:              []string{"client_credentials"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(secondID) }()

	updatePayload := Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Name:       "dup-upd-second-agent",
		AuthFlowID: ts.authFlowID,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				Config: &OAuthAgentConfig{
					ClientID:                "dup_upd_first_clientid",
					ClientSecret:            "dup_upd_secret2",
					GrantTypes:              []string{"client_credentials"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	}
	body, _ := json.Marshal(updatePayload)
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("PUT", testServerURL+agentBasePath+"/"+secondID, bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusConflict, resp.StatusCode,
		"Update to taken clientId must return 409: "+readBody(resp))
}

// --- transition: entity-only → OAuth on update ---

// TestAgentUpdate_AddOAuthProfile verifies that an entity-only agent can be promoted to an
// OAuth client via an update, and that a CC token can then be obtained.
func (ts *AgentOAuthFlowsTestSuite) TestAgentUpdate_AddOAuthProfile() {
	agentID, err := createAgent(Agent{
		OUID: ts.ouID,
		Type: "default",
		Name: "promote-to-oauth-agent",
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	getResp, err := doGet(testServerURL + agentBasePath + "/" + agentID)
	ts.Require().NoError(err)
	defer getResp.Body.Close()
	ts.Require().Equal(http.StatusOK, getResp.StatusCode, readBody(getResp))
	var before Agent
	ts.Require().NoError(json.NewDecoder(getResp.Body).Decode(&before))
	ts.Assert().Empty(before.InboundAuthConfig, "Entity-only agent must have no inbound config")

	withOAuth := Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Name:       "promote-to-oauth-agent",
		AuthFlowID: ts.authFlowID,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				Config: &OAuthAgentConfig{
					ClientID:                "promoted_agent_client",
					ClientSecret:            "promoted_agent_secret",
					GrantTypes:              []string{"client_credentials"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	}
	putBody, _ := json.Marshal(withOAuth)
	client := testutils.GetHTTPClient()
	putReq, err := http.NewRequest("PUT", testServerURL+agentBasePath+"/"+agentID, bytes.NewReader(putBody))
	ts.Require().NoError(err)
	putReq.Header.Set("Content-Type", "application/json")
	putResp, err := client.Do(putReq)
	ts.Require().NoError(err)
	defer putResp.Body.Close()
	ts.Require().Equal(http.StatusOK, putResp.StatusCode, readBody(putResp))

	tokenResult, err := testutils.RequestToken(
		"promoted_agent_client", "promoted_agent_secret",
		"", "", "client_credentials",
	)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode,
		"CC token for promoted agent must succeed: "+string(tokenResult.Body))
	ts.Assert().NotEmpty(tokenResult.Token.AccessToken)
}

// ============================================================================
// CCAgentAuthzTestSuite — role-based scope grants via client_credentials
// ============================================================================

// CCAgentAuthzTestSuite verifies that agents can be assigned roles directly and via groups,
// and that client_credentials tokens reflect those role-based scope grants.
type CCAgentAuthzTestSuite struct {
	suite.Suite
	client           *http.Client
	ouID             string
	agentSchemaID    string
	resourceServerID string
	agentID          string
	roleID           string
	groupID          string
	groupRoleID      string
}

func TestCCAgentAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(CCAgentAuthzTestSuite))
}

func (s *CCAgentAuthzTestSuite) SetupSuite() {
	s.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "cc-agent-authz-ou",
		Name:        "CC Agent Authz OU",
		Description: "Organization unit for CC agent authorization tests",
	})
	s.Require().NoError(err)
	s.ouID = ouID

	schemaID, err := testutils.CreateAgentType(testutils.UserType{
		Name: "default",
		OUID: s.ouID,
		Schema: map[string]interface{}{
			"description": map[string]interface{}{"type": "string"},
		},
	})
	s.Require().NoError(err)
	s.agentSchemaID = schemaID

	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "CC Agent Authz API",
		Description: "Resource server for CC agent authz testing",
		Identifier:  "cc-agent-authz-api",
		OUID:        s.ouID,
	}, []testutils.Action{
		{Name: "Read", Handle: "read", Description: "Read access"},
		{Name: "Write", Handle: "write", Description: "Write access"},
		{Name: "Delete", Handle: "delete", Description: "Delete access"},
		{Name: "Approve", Handle: "approve", Description: "Approve access"},
	})
	s.Require().NoError(err)
	s.resourceServerID = rsID

	agentID, err := s.createOAuthAgent()
	s.Require().NoError(err)
	s.agentID = agentID

	roleID, err := testutils.CreateRole(testutils.Role{
		Name:        "CC Agent Direct Role",
		Description: "Role with read+write assigned directly to the agent",
		OUID:        s.ouID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: s.resourceServerID,
				Permissions:      []string{"read", "write"},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: s.agentID, Type: "agent"},
		},
	})
	s.Require().NoError(err)
	s.roleID = roleID

	groupID, err := testutils.CreateGroup(testutils.Group{
		Name:    "CC Agent Authz Group",
		OUID:    s.ouID,
		Members: []testutils.Member{{Id: s.agentID, Type: "agent"}},
	})
	s.Require().NoError(err)
	s.groupID = groupID

	groupRoleID, err := testutils.CreateRole(testutils.Role{
		Name:        "CC Agent Group Role",
		Description: "Role with approve assigned to the group containing the agent",
		OUID:        s.ouID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: s.resourceServerID,
				Permissions:      []string{"approve"},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: s.groupID, Type: "group"},
		},
	})
	s.Require().NoError(err)
	s.groupRoleID = groupRoleID
}

func (s *CCAgentAuthzTestSuite) TearDownSuite() {
	if s.groupRoleID != "" {
		_ = testutils.DeleteRole(s.groupRoleID)
	}
	if s.roleID != "" {
		_ = testutils.DeleteRole(s.roleID)
	}
	if s.groupID != "" {
		_ = testutils.DeleteGroup(s.groupID)
	}
	if s.agentID != "" {
		_ = deleteAgent(s.agentID)
	}
	if s.resourceServerID != "" {
		_ = testutils.DeleteResourceServer(s.resourceServerID)
	}
	if s.agentSchemaID != "" {
		_ = testutils.DeleteAgentType(s.agentSchemaID)
	}
	if s.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(s.ouID)
	}
}

func (s *CCAgentAuthzTestSuite) createOAuthAgent() (string, error) {
	agent := map[string]interface{}{
		"name": "CC Authz Test Agent",
		"type": "default",
		"ouId": s.ouID,
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                ccAgentClientID,
					"clientSecret":            ccAgentClientSecret,
					"grantTypes":              []string{"client_credentials"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	}

	jsonData, err := json.Marshal(agent)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", testServerURL+agentBasePath, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("failed to create agent: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var respData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &respData); err != nil {
		return "", err
	}
	return respData["id"].(string), nil
}

func (s *CCAgentAuthzTestSuite) requestToken(scopes string) (int, map[string]interface{}) {
	body := "grant_type=client_credentials"
	if scopes != "" {
		body += "&scope=" + scopes
	}

	req, err := http.NewRequest("POST", testServerURL+"/oauth2/token", strings.NewReader(body))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(ccAgentClientID, ccAgentClientSecret)

	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	var respBody map[string]interface{}
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&respBody))
	return resp.StatusCode, respBody
}

// TestAgentCC_DirectRoleScopes verifies scopes granted via a role assigned directly to the agent.
func (s *CCAgentAuthzTestSuite) TestAgentCC_DirectRoleScopes() {
	status, body := s.requestToken("read write")
	s.Equal(http.StatusOK, status)
	s.Contains(body, "access_token")

	scopeStr, ok := body["scope"].(string)
	s.Require().True(ok, "Response should contain scope")
	s.ElementsMatch([]string{"read", "write"}, strings.Fields(scopeStr))
}

// TestAgentCC_PartialDirectScopes requests one authorized and one unauthorized scope.
func (s *CCAgentAuthzTestSuite) TestAgentCC_PartialDirectScopes() {
	status, body := s.requestToken("read delete")
	s.Equal(http.StatusOK, status)
	s.Contains(body, "access_token")

	scopeStr, ok := body["scope"].(string)
	s.Require().True(ok, "Response should contain scope")
	s.ElementsMatch([]string{"read"}, strings.Fields(scopeStr))
}

// TestAgentCC_UnauthorizedScopes verifies that unauthorized scopes are not issued.
func (s *CCAgentAuthzTestSuite) TestAgentCC_UnauthorizedScopes() {
	status, body := s.requestToken("delete")
	s.Equal(http.StatusOK, status)
	s.Contains(body, "access_token")

	_, hasScope := body["scope"]
	s.False(hasScope, "Response should not contain scope for unauthorized-only request")
}

// TestAgentCC_NoScopes verifies that a token is issued without scopes when none are requested.
func (s *CCAgentAuthzTestSuite) TestAgentCC_NoScopes() {
	status, body := s.requestToken("")
	s.Equal(http.StatusOK, status)
	s.Contains(body, "access_token")

	_, hasScope := body["scope"]
	s.False(hasScope, "Response should not contain scope when none were requested")
}

// TestAgentCC_GroupInheritedScope verifies scopes granted via a role assigned to a group the agent belongs to.
func (s *CCAgentAuthzTestSuite) TestAgentCC_GroupInheritedScope() {
	status, body := s.requestToken("approve")
	s.Equal(http.StatusOK, status)
	s.Contains(body, "access_token")

	scopeStr, ok := body["scope"].(string)
	s.Require().True(ok, "Response should contain scope")
	s.Equal("approve", scopeStr)
}

// TestAgentCC_AllScopes verifies that direct and group-inherited scopes can all be requested at once.
func (s *CCAgentAuthzTestSuite) TestAgentCC_AllScopes() {
	status, body := s.requestToken("read write approve")
	s.Equal(http.StatusOK, status)
	s.Contains(body, "access_token")

	scopeStr, ok := body["scope"].(string)
	s.Require().True(ok, "Response should contain scope")
	s.ElementsMatch([]string{"read", "write", "approve"}, strings.Fields(scopeStr))
}

// ============================================================================
// AgentTokenExchangeTestSuite — token exchange grant
// ============================================================================

// AgentTokenExchangeTestSuite verifies the token exchange grant when an agent acts as
// the OAuth client (authenticated via client_id/client_secret) and the subject_token
// is a user assertion.
type AgentTokenExchangeTestSuite struct {
	suite.Suite
	client         *http.Client
	ouID           string
	entityTypeID   string
	agentSchemaID  string
	agentID        string
	userID         string
	assertionToken string
}

func TestAgentTokenExchangeTestSuite(t *testing.T) {
	suite.Run(t, new(AgentTokenExchangeTestSuite))
}

func (s *AgentTokenExchangeTestSuite) SetupSuite() {
	s.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "agent-te-ou",
		Name:        "Agent Token Exchange OU",
		Description: "Organization unit for agent token exchange tests",
	})
	s.Require().NoError(err)
	s.ouID = ouID

	agentSchemaID, err := testutils.CreateAgentType(testutils.UserType{
		Name: "default",
		OUID: s.ouID,
		Schema: map[string]interface{}{
			"description": map[string]interface{}{"type": "string"},
		},
	})
	s.Require().NoError(err)
	s.agentSchemaID = agentSchemaID

	entityTypeID, err := testutils.CreateUserType(testutils.UserType{
		Name: "agent-te-person",
		OUID: s.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	s.Require().NoError(err)
	s.entityTypeID = entityTypeID

	attributesJSON, err := json.Marshal(map[string]interface{}{
		"username": agentTEUsername,
		"password": agentTEPassword,
	})
	s.Require().NoError(err)
	userID, err := testutils.CreateUser(testutils.User{
		Type:       "agent-te-person",
		OUID:       s.ouID,
		Attributes: attributesJSON,
	})
	s.Require().NoError(err)
	s.userID = userID

	s.agentID, err = s.createTokenExchangeAgent()
	s.Require().NoError(err)

	s.assertionToken = s.getUserAssertion()
}

func (s *AgentTokenExchangeTestSuite) TearDownSuite() {
	if s.agentID != "" {
		_ = deleteAgent(s.agentID)
	}
	if s.userID != "" {
		_ = testutils.DeleteUser(s.userID)
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

func (s *AgentTokenExchangeTestSuite) createTokenExchangeAgent() (string, error) {
	agent := map[string]interface{}{
		"name": "Agent Token Exchange Test",
		"type": "default",
		"ouId": s.ouID,
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                agentTEClientID,
					"clientSecret":            agentTEClientSecret,
					"grantTypes":              []string{"urn:ietf:params:oauth:grant-type:token-exchange"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	}

	jsonData, err := json.Marshal(agent)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", testServerURL+agentBasePath, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("failed to create agent: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var respData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &respData); err != nil {
		return "", err
	}
	return respData["id"].(string), nil
}

func (s *AgentTokenExchangeTestSuite) getUserAssertion() string {
	authRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"username": agentTEUsername,
		},
		"credentials": map[string]interface{}{
			"password": agentTEPassword,
		},
	}

	requestJSON, err := json.Marshal(authRequest)
	s.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+"/auth/credentials/authenticate", bytes.NewReader(requestJSON))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode, "User authentication must succeed")

	var authResponse testutils.AuthenticationResponse
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&authResponse))
	s.Require().NotEmpty(authResponse.Assertion, "Assertion token must be returned")
	return authResponse.Assertion
}

func (s *AgentTokenExchangeTestSuite) doTokenExchange(formData url.Values) (*TokenExchangeResponse, int, error) {
	req, err := http.NewRequest("POST", testServerURL+"/oauth2/token", strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(agentTEClientID, agentTEClientSecret)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var tokenResp TokenExchangeResponse
	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
			return nil, resp.StatusCode, err
		}
		return &tokenResp, resp.StatusCode, nil
	}

	var errResp map[string]interface{}
	_ = json.Unmarshal(bodyBytes, &errResp)
	tokenResp.Error = fmt.Sprintf("%v", errResp["error"])
	if desc, ok := errResp["error_description"]; ok {
		tokenResp.ErrorDescription = fmt.Sprintf("%v", desc)
	}
	return &tokenResp, resp.StatusCode, nil
}

// TestAgentTE_BasicSuccess verifies a successful token exchange where the agent is the
// OAuth client and the subject_token is a user assertion.
func (s *AgentTokenExchangeTestSuite) TestAgentTE_BasicSuccess() {
	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", s.assertionToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")

	resp, statusCode, err := s.doTokenExchange(formData)
	s.Require().NoError(err)
	s.Equal(http.StatusOK, statusCode)
	s.NotEmpty(resp.AccessToken, "Access token must be present")
	s.Equal("Bearer", resp.TokenType)
	s.NotZero(resp.ExpiresIn)
	s.Equal("urn:ietf:params:oauth:token-type:access_token", resp.IssuedTokenType)

	claims, err := testutils.DecodeJWT(resp.AccessToken)
	s.Require().NoError(err, "Access token must be a valid JWT")
	s.Equal(s.userID, claims.Sub, "Subject must match the authenticated user")
}

// TestAgentTE_WithAudience verifies that the audience parameter is reflected in the issued token.
func (s *AgentTokenExchangeTestSuite) TestAgentTE_WithAudience() {
	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", s.assertionToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")
	formData.Set("audience", "https://agent-api.example.com")

	resp, statusCode, err := s.doTokenExchange(formData)
	s.Require().NoError(err)
	s.Equal(http.StatusOK, statusCode)
	s.NotEmpty(resp.AccessToken)

	claims, err := testutils.DecodeJWT(resp.AccessToken)
	s.Require().NoError(err)

	rawAud, ok := claims.Additional["aud"]
	s.Require().True(ok, "JWT must contain an aud claim")
	switch aud := rawAud.(type) {
	case string:
		s.Equal("https://agent-api.example.com", aud)
	case []interface{}:
		found := false
		for _, v := range aud {
			if str, ok := v.(string); ok && str == "https://agent-api.example.com" {
				found = true
				break
			}
		}
		s.True(found, "Audience array must contain requested audience")
	default:
		s.Failf("unexpected aud type", "%T", rawAud)
	}
}

// TestAgentTE_InvalidSubjectToken verifies that a malformed subject_token is rejected.
func (s *AgentTokenExchangeTestSuite) TestAgentTE_InvalidSubjectToken() {
	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", "not.a.valid.jwt")
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")

	resp, statusCode, err := s.doTokenExchange(formData)
	s.Require().NoError(err)
	s.Equal(http.StatusBadRequest, statusCode)
	s.Equal("invalid_request", resp.Error)
	s.Contains(resp.ErrorDescription, "Invalid subject_token")
}

// TestAgentTE_InvalidAgentCredentials verifies that wrong agent credentials cause a 401.
func (s *AgentTokenExchangeTestSuite) TestAgentTE_InvalidAgentCredentials() {
	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", s.assertionToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")

	req, err := http.NewRequest("POST", testServerURL+"/oauth2/token", strings.NewReader(formData.Encode()))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(agentTEClientID, "wrong_secret")

	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()
	s.Equal(http.StatusUnauthorized, resp.StatusCode)
}

// TestAgentTE_MissingSubjectToken verifies that omitting subject_token returns a 400.
func (s *AgentTokenExchangeTestSuite) TestAgentTE_MissingSubjectToken() {
	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")

	resp, statusCode, err := s.doTokenExchange(formData)
	s.Require().NoError(err)
	s.Equal(http.StatusBadRequest, statusCode)
	s.Equal("invalid_request", resp.Error)
	s.Contains(resp.ErrorDescription, "subject_token")
}

// ============================================================================
// Shared helpers
// ============================================================================

// agentTokenRequest sends a token request to the server and returns the response body and status.
func agentTokenRequest(body string, clientID, clientSecret string, inBody bool) (map[string]interface{}, int, error) {
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("POST", testServerURL+"/oauth2/token", strings.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if !inBody {
		req.SetBasicAuth(clientID, clientSecret)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to send token request: %w", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	_ = json.Unmarshal(bodyBytes, &result)
	return result, resp.StatusCode, nil
}
