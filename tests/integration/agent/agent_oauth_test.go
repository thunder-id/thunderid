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
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
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

	agentTENextClientID     = "agent_token_exchange_next_client"
	agentTENextClientSecret = "agent_token_exchange_next_secret"

	agentOAuthDefaultRSIdentifier = "https://agent-oauth-default.example.com"
	ccAgentAuthzRSIdentifier      = "https://cc-agent-authz.example.com"
	agentTEDefaultRSIdentifier    = "https://agent-te-default.example.com"
)

// ============================================================================
// AgentOAuthFlowsTestSuite — grant-type flows (CC, auth code, PKCE, refresh)
// ============================================================================

// AgentOAuthFlowsTestSuite covers OAuth grant type flows for agents:
// client_secret_post CC, invalid credentials, auth code, PKCE, refresh token,
// duplicate clientID, and transitioning an entity-only agent to OAuth on update.
type AgentOAuthFlowsTestSuite struct {
	suite.Suite
	ouID              string
	agentTypeSnapshot *testutils.AgentTypeSnapshot
	entityTypeID      string
	userID            string
	authFlowID        string
	resourceServerID  string
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

	resourceServerID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Agent OAuth Default Resource Server",
		Description: "Resource server for agent OAuth flow tests",
		Identifier:  agentOAuthDefaultRSIdentifier,
		OUID:        ts.ouID,
	}, []testutils.Action{})
	ts.Require().NoError(err, "Failed to create resource server")
	ts.resourceServerID = resourceServerID

	// The `default` agent type is a singleton shared with every other suite. Snapshot it before
	// pointing it at this suite's OU, so teardown can put it back before that OU is deleted.
	snapshot, err := testutils.SnapshotAgentType()
	ts.Require().NoError(err, "Failed to snapshot the default agent type")
	ts.agentTypeSnapshot = snapshot

	_, err = testutils.CreateAgentType(testutils.UserType{
		Handle:      "default",
		DisplayName: "Default",
		OUID:        ts.ouID,
		Schema: map[string]interface{}{
			"name":        map[string]interface{}{"type": "string", "required": true, "unique": true},
			"description": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "Failed to create agent schema")

	entityTypeID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      "agent-oauth-flow-person",
		DisplayName: "Agent Oauth Flow Person",
		OUID:        ts.ouID,
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

	flowID, err := testutils.GetFlowIDByHandle("default-flow", "AUTHENTICATION")
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
	if ts.resourceServerID != "" {
		ts.NoError(testutils.DeleteResourceServerWithChildren(ts.resourceServerID),
			"teardown: delete resource server and its actions")
	}
	if ts.ouID != "" && agentTypeRestored {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// --- CC token issuance ---

// TestAgentCC_TokenIssuance verifies that a newly created agent with CC grant can obtain a token.
func (ts *AgentOAuthFlowsTestSuite) TestAgentCC_TokenIssuance() {
	agentID, err := createAgent(Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Attributes: agentAttrs("cc-token-agent", nil),
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

	tokenResult, err := testutils.RequestTokenWithResource(
		"cc_token_agent_client", "cc_token_agent_secret",
		"", "", "client_credentials",
		agentOAuthDefaultRSIdentifier,
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
		Attributes: agentAttrs("cc-post-agent", nil),
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

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", "cc_post_agent_client")
	form.Set("client_secret", "cc_post_agent_secret")
	form.Set("resource", agentOAuthDefaultRSIdentifier)
	body := form.Encode()
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
		Attributes: agentAttrs("cc-invalid-cred-agent", nil),
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

	tokenResult, err := testutils.RequestTokenWithResource(
		"cc_invalid_agent_client", "wrong_secret",
		"", "", "client_credentials",
		agentOAuthDefaultRSIdentifier,
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
		Attributes:       agentAttrs("authcode-agent", nil),
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

	token, err := obtainAgentAccessTokenWithResource(
		"authcode_agent_client", oauthFlowsRedirectURI, "openid",
		oauthFlowsTestUsername, oauthFlowsTestPassword,
		false, "authcode_agent_secret", agentOAuthDefaultRSIdentifier,
	)
	ts.Require().NoError(err, "Auth code full round trip must succeed")
	ts.Assert().NotEmpty(token.AccessToken)
}

// TestAgentAuthCode_WithPKCE verifies the authorization code + PKCE flow for an agent.
func (ts *AgentOAuthFlowsTestSuite) TestAgentAuthCode_WithPKCE() {
	agentID, err := createAgent(Agent{
		OUID:             ts.ouID,
		Type:             "default",
		Attributes:       agentAttrs("authcode-pkce-agent", nil),
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

	token, err := obtainAgentAccessTokenWithResource(
		"authcode_pkce_agent_client", oauthFlowsRedirectURI, "openid",
		oauthFlowsTestUsername, oauthFlowsTestPassword,
		true, "authcode_pkce_agent_secret", agentOAuthDefaultRSIdentifier,
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
		Attributes:       agentAttrs("authcode-refresh-agent", nil),
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

	token, err := obtainAgentAccessTokenWithResource(
		"authcode_refresh_agent_client", oauthFlowsRedirectURI, "openid",
		oauthFlowsTestUsername, oauthFlowsTestPassword,
		false, "authcode_refresh_agent_secret", agentOAuthDefaultRSIdentifier,
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
		Attributes: agentAttrs("dup-client-first-agent", nil),
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
		Attributes: agentAttrs("dup-client-second-agent", nil),
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
		Attributes: agentAttrs("dup-upd-first-agent", nil),
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
		Attributes: agentAttrs("dup-upd-second-agent", nil),
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
		Attributes: agentAttrs("dup-upd-second-agent", nil),
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
		OUID:       ts.ouID,
		Type:       "default",
		Attributes: agentAttrs("promote-to-oauth-agent", nil),
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
		Attributes: agentAttrs("promote-to-oauth-agent", nil),
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

	tokenResult, err := testutils.RequestTokenWithResource(
		"promoted_agent_client", "promoted_agent_secret",
		"", "", "client_credentials",
		agentOAuthDefaultRSIdentifier,
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
	client            *http.Client
	ouID              string
	agentTypeSnapshot *testutils.AgentTypeSnapshot
	resourceServerID  string
	agentID           string
	roleID            string
	groupID           string
	groupRoleID       string
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

	// The `default` agent type is a singleton shared with every other suite. Snapshot it before
	// pointing it at this suite's OU, so teardown can put it back before that OU is deleted.
	snapshot, err := testutils.SnapshotAgentType()
	s.Require().NoError(err)
	s.agentTypeSnapshot = snapshot

	_, err = testutils.CreateAgentType(testutils.UserType{
		Handle:      "default",
		DisplayName: "Default",
		OUID:        s.ouID,
		Schema: map[string]interface{}{
			"name":        map[string]interface{}{"type": "string", "required": true, "unique": true},
			"description": map[string]interface{}{"type": "string"},
		},
	})
	s.Require().NoError(err)

	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "CC Agent Authz API",
		Description: "Resource server for CC agent authz testing",
		Identifier:  ccAgentAuthzRSIdentifier,
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
		s.NoError(testutils.DeleteResourceServerWithChildren(s.resourceServerID),
			"teardown: delete resource server and its actions")
	}
	// Restore the shared agent type before deleting the OU it points at, or the singleton is left
	// referencing a deleted OU and a later suite's restore fails. A failed restore keeps the OU:
	// leaking one is cheaper than every later suite inheriting a dangling reference.
	agentTypeRestored := true
	if s.agentTypeSnapshot != nil {
		if err := testutils.RestoreAgentType(s.agentTypeSnapshot); err != nil {
			s.T().Errorf("teardown: failed to restore the default agent type: %v", err)
			agentTypeRestored = false
		}
	}
	if s.ouID != "" && agentTypeRestored {
		_ = testutils.DeleteOrganizationUnit(s.ouID)
	}
}

func (s *CCAgentAuthzTestSuite) createOAuthAgent() (string, error) {
	agent := map[string]interface{}{
		"attributes": map[string]interface{}{"name": "CC Authz Test Agent"},
		"type":       "default",
		"ouId":       s.ouID,
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
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("resource", ccAgentAuthzRSIdentifier)
	if scopes != "" {
		form.Set("scope", scopes)
	}

	req, err := http.NewRequest("POST", testServerURL+"/oauth2/token", strings.NewReader(form.Encode()))
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
	client            *http.Client
	ouID              string
	entityTypeID      string
	agentTypeSnapshot *testutils.AgentTypeSnapshot
	agentID           string
	nextAgentID       string
	userID            string
	resourceServerID  string
	assertionToken    string
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

	resourceServerID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Agent Token Exchange Default Resource Server",
		Description: "Resource server for agent token exchange tests",
		Identifier:  agentTEDefaultRSIdentifier,
		OUID:        s.ouID,
	}, []testutils.Action{})
	s.Require().NoError(err)
	s.resourceServerID = resourceServerID

	// The `default` agent type is a singleton shared with every other suite. Snapshot it before
	// pointing it at this suite's OU, so teardown can put it back before that OU is deleted.
	snapshot, err := testutils.SnapshotAgentType()
	s.Require().NoError(err)
	s.agentTypeSnapshot = snapshot

	_, err = testutils.CreateAgentType(testutils.UserType{
		Handle:      "default",
		DisplayName: "Default",
		OUID:        s.ouID,
		Schema: map[string]interface{}{
			"name":        map[string]interface{}{"type": "string", "required": true, "unique": true},
			"description": map[string]interface{}{"type": "string"},
		},
	})
	s.Require().NoError(err)

	entityTypeID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      "agent-te-person",
		DisplayName: "Agent Te Person",
		OUID:        s.ouID,
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

	s.agentID, err = s.createTokenExchangeAgent(
		"Agent Token Exchange Test", agentTEClientID, agentTEClientSecret)
	s.Require().NoError(err)

	s.nextAgentID, err = s.createTokenExchangeAgent(
		"Agent Token Exchange Next Test", agentTENextClientID, agentTENextClientSecret)
	s.Require().NoError(err)
}

// SetupTest mints a fresh assertion before every test. An assertion is redeemable exactly once, so a
// suite-scoped one would be spent by the first test that exchanges it and every later exchange would
// be refused as a replay.
func (s *AgentTokenExchangeTestSuite) SetupTest() {
	s.assertionToken = s.getUserAssertion()
}

func (s *AgentTokenExchangeTestSuite) TearDownSuite() {
	if s.agentID != "" {
		_ = deleteAgent(s.agentID)
	}
	if s.nextAgentID != "" {
		_ = deleteAgent(s.nextAgentID)
	}
	if s.userID != "" {
		_ = testutils.DeleteUser(s.userID)
	}
	if s.entityTypeID != "" {
		_ = testutils.DeleteUserType(s.entityTypeID)
	}
	// Restore the shared agent type before deleting the OU it points at, or the singleton is left
	// referencing a deleted OU and a later suite's restore fails. A failed restore keeps the OU:
	// leaking one is cheaper than every later suite inheriting a dangling reference.
	agentTypeRestored := true
	if s.agentTypeSnapshot != nil {
		if err := testutils.RestoreAgentType(s.agentTypeSnapshot); err != nil {
			s.T().Errorf("teardown: failed to restore the default agent type: %v", err)
			agentTypeRestored = false
		}
	}
	if s.resourceServerID != "" {
		s.NoError(testutils.DeleteResourceServerWithChildren(s.resourceServerID),
			"teardown: delete resource server and its actions")
	}
	if s.ouID != "" && agentTypeRestored {
		_ = testutils.DeleteOrganizationUnit(s.ouID)
	}
}

func (s *AgentTokenExchangeTestSuite) createTokenExchangeAgent(name, clientID, clientSecret string) (string, error) {
	agent := map[string]interface{}{
		"attributes": map[string]interface{}{"name": name},
		"type":       "default",
		"ouId":       s.ouID,
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                clientID,
					"clientSecret":            clientSecret,
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
	return s.doTokenExchangeAs(agentTEClientID, agentTEClientSecret, formData)
}

// doTokenExchangeAs submits a token-exchange request authenticated as the given agent client.
func (s *AgentTokenExchangeTestSuite) doTokenExchangeAs(
	clientID, clientSecret string, formData url.Values) (*TokenExchangeResponse, int, error) {
	req, err := http.NewRequest("POST", testServerURL+"/oauth2/token", strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

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
	formData.Set("resource", agentTEDefaultRSIdentifier)

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
	s.Equal(agentTEDefaultRSIdentifier, claims.Aud, "Audience must match the requested resource server")
}

// TestAgentTE_WithAudience verifies that the audience parameter does not drive the issued access-token audience.
func (s *AgentTokenExchangeTestSuite) TestAgentTE_WithAudience() {
	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", s.assertionToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")
	formData.Set("audience", "https://agent-api.example.com")
	formData.Set("resource", agentTEDefaultRSIdentifier)

	resp, statusCode, err := s.doTokenExchange(formData)
	s.Require().NoError(err)
	s.Equal(http.StatusOK, statusCode)
	s.NotEmpty(resp.AccessToken)

	claims, err := testutils.DecodeJWT(resp.AccessToken)
	s.Require().NoError(err)
	s.Equal(agentTEDefaultRSIdentifier, claims.Aud, "Audience must match the requested resource server")
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

// TestAgentTE_ExtendsSubjectDelegationChain verifies that an agent exchanging a token that already
// records another agent as its actor becomes the current actor, with the earlier agent nested
// beneath it (RFC 8693 Section 4.1).
func (s *AgentTokenExchangeTestSuite) TestAgentTE_ExtendsSubjectDelegationChain() {
	firstToken := s.exchangeAs(agentTEClientID, agentTEClientSecret,
		s.assertionToken, "urn:ietf:params:oauth:token-type:jwt")
	firstAct := s.actClaim(firstToken)
	s.Equal(s.agentID, firstAct["sub"])
	s.NotContains(firstAct, "act")

	nextToken := s.exchangeAs(agentTENextClientID, agentTENextClientSecret,
		firstToken, "urn:ietf:params:oauth:token-type:access_token")
	claims, err := testutils.DecodeJWT(nextToken)
	s.Require().NoError(err)
	s.Equal(s.userID, claims.Sub, "Subject must stay the original user")

	act := s.actClaim(nextToken)
	s.Equal(s.nextAgentID, act["sub"], "act.sub must identify the agent that made this exchange")
	priorAct, ok := act["act"].(map[string]interface{})
	s.Require().True(ok, "act must keep the subject token's actor as a nested act")
	s.Equal(s.agentID, priorAct["sub"], "act.act.sub must identify the earlier agent")
	s.NotContains(priorAct, "act")
}

// TestAgentTE_SameAgentIsNotRepeated verifies that an agent re-exchanging a token it already acts
// through is recorded once, not nested under itself.
func (s *AgentTokenExchangeTestSuite) TestAgentTE_SameAgentIsNotRepeated() {
	firstToken := s.exchangeAs(agentTEClientID, agentTEClientSecret,
		s.assertionToken, "urn:ietf:params:oauth:token-type:jwt")

	secondToken := s.exchangeAs(agentTEClientID, agentTEClientSecret,
		firstToken, "urn:ietf:params:oauth:token-type:access_token")

	act := s.actClaim(secondToken)
	s.Equal(s.agentID, act["sub"])
	s.NotContains(act, "act", "The same agent must not be nested under itself")
}

// exchangeAs exchanges the subject token as the given agent client for a token bound to the
// suite's resource server, and returns the issued access token.
func (s *AgentTokenExchangeTestSuite) exchangeAs(clientID, clientSecret, subjectToken, subjectTokenType string) string {
	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", subjectToken)
	formData.Set("subject_token_type", subjectTokenType)
	formData.Set("resource", agentTEDefaultRSIdentifier)

	resp, statusCode, err := s.doTokenExchangeAs(clientID, clientSecret, formData)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, statusCode, "%s: %s", resp.Error, resp.ErrorDescription)
	s.Require().NotEmpty(resp.AccessToken)
	return resp.AccessToken
}

// actClaim decodes the token and returns its act claim, failing the test when it has none.
func (s *AgentTokenExchangeTestSuite) actClaim(token string) map[string]interface{} {
	claims, err := testutils.DecodeJWT(token)
	s.Require().NoError(err)
	act, ok := claims.Additional["act"].(map[string]interface{})
	s.Require().True(ok, "Token must carry an act claim")
	return act
}

// ============================================================================
// Shared helpers
// ============================================================================

// obtainAgentAccessTokenWithResource performs the full authorization code flow (optionally with PKCE)
// for an agent OAuth client, binding the token to the given RFC 8707 resource server.
func obtainAgentAccessTokenWithResource(clientID, redirectURI, scope, username, password string,
	usePKCE bool, clientSecret, resource string) (*testutils.TokenResponse, error) {
	var codeVerifier, codeChallenge string
	if usePKCE {
		var err error
		codeVerifier, err = testutils.GenerateCodeVerifier()
		if err != nil {
			return nil, fmt.Errorf("failed to generate code verifier: %w", err)
		}
		codeChallenge = testutils.GenerateCodeChallenge(codeVerifier)
	}

	resp, err := testutils.InitiateAuthorizationFlowWithPKCE(clientID, redirectURI, "code", scope,
		"test-state", resource, codeChallenge, "S256")
	if err != nil {
		return nil, fmt.Errorf("failed to initiate authorization: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther &&
		resp.StatusCode != http.StatusTemporaryRedirect {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("expected redirect response, got status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	location := resp.Header.Get("Location")
	if location == "" {
		return nil, fmt.Errorf("no Location header in authorization response")
	}

	authID, executionId, err := testutils.ExtractAuthData(location)
	if err != nil {
		return nil, fmt.Errorf("failed to extract auth ID: %w", err)
	}

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionId, nil, "")
	if err != nil {
		return nil, fmt.Errorf("failed to execute initial authentication flow: %w", err)
	}

	flowStep, err := testutils.ExecuteAuthenticationFlow(executionId, map[string]string{
		"username": username,
		"password": password,
	}, "action_001", initialStep.ChallengeToken)
	if err != nil {
		return nil, fmt.Errorf("failed to execute authentication flow: %w", err)
	}
	if flowStep.FlowStatus != "COMPLETE" {
		return nil, fmt.Errorf("authentication flow not complete: status=%s", flowStep.FlowStatus)
	}
	if flowStep.Assertion == "" {
		return nil, fmt.Errorf("no assertion returned from authentication flow")
	}

	authzResp, err := testutils.CompleteAuthorization(authID, flowStep.Assertion)
	if err != nil {
		return nil, fmt.Errorf("failed to complete authorization: %w", err)
	}

	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	if err != nil {
		return nil, fmt.Errorf("failed to extract authorization code: %w", err)
	}

	var tokenResult *testutils.TokenHTTPResult
	if usePKCE {
		tokenResult, err = testutils.RequestTokenWithPKCEAndResourceAndClientCredentialsInBody(
			clientID, clientSecret, code, redirectURI, "authorization_code", codeVerifier, resource)
	} else {
		tokenResult, err = testutils.RequestTokenWithResourceAndClientCredentialsInBody(
			clientID, clientSecret, code, redirectURI, "authorization_code", resource)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to request token: %w", err)
	}
	if tokenResult.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed with status %d: %s", tokenResult.StatusCode,
			string(tokenResult.Body))
	}
	if tokenResult.Token == nil {
		return nil, fmt.Errorf("no token in response")
	}
	return tokenResult.Token, nil
}

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

// createLifespanAgent creates a client credentials agent whose lifespan ends at validUntil, or does
// not end when validUntil is nil.
func (ts *AgentOAuthFlowsTestSuite) createLifespanAgent(name, clientID string, validUntil *string) string {
	agentID, err := createAgent(Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Attributes: agentAttrs(name, nil),
		AuthFlowID: ts.authFlowID,
		ValidUntil: validUntil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				Config: &OAuthAgentConfig{
					ClientID:                clientID,
					ClientSecret:            clientID + "_secret",
					GrantTypes:              []string{"client_credentials"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	})
	ts.Require().NoError(err)
	return agentID
}

func (ts *AgentOAuthFlowsTestSuite) getAgent(agentID string) Agent {
	resp, err := doGet(testServerURL + agentBasePath + "/" + agentID)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))
	var agent Agent
	ts.Require().NoError(json.Unmarshal([]byte(readBody(resp)), &agent))
	return agent
}

func lifespanEnd(in time.Duration) *string {
	end := time.Now().UTC().Add(in).Truncate(time.Second).Format(time.RFC3339)
	return &end
}

// TestAgentLifespan_SetChangeAndRemove covers registering an agent with a lifespan, an update that
// leaves it out, removing the expiry with null, and the values the API refuses.
func (ts *AgentOAuthFlowsTestSuite) TestAgentLifespan_SetChangeAndRemove() {
	validUntil := lifespanEnd(48 * time.Hour)
	agentID := ts.createLifespanAgent("lifespan-crud-agent", "lifespan_crud_client", validUntil)
	defer func() { _ = deleteAgent(agentID) }()

	agent := ts.getAgent(agentID)
	ts.Require().NotNil(agent.ValidUntil)
	ts.Equal(*validUntil, *agent.ValidUntil)
	ts.Require().NotNil(agent.Lifespan)
	ts.Equal("ACTIVE", agent.Lifespan.Phase)

	// An update that does not carry validUntil leaves the lifespan unchanged.
	resp, err := putAgentRaw(agentID, map[string]interface{}{
		"type": "default", "ouId": ts.ouID, "attributes": map[string]interface{}{"name": "lifespan-crud-agent"},
	})
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))
	_ = resp.Body.Close()
	ts.Equal(*validUntil, *ts.getAgent(agentID).ValidUntil)

	// A past value is refused and changes nothing.
	resp, err = putAgentRaw(agentID, map[string]interface{}{
		"type": "default", "ouId": ts.ouID, "attributes": map[string]interface{}{"name": "renamed-agent"},
		"validUntil": "2020-01-01T00:00:00Z",
	})
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, resp.StatusCode)
	ts.Equal("AGT-1046", errorCode(resp))
	_ = resp.Body.Close()
	ts.Equal("lifespan-crud-agent", ts.getAgent(agentID).AttrName(), "a refused lifespan applies no field")

	// null removes the expiry.
	resp, err = putAgentRaw(agentID, map[string]interface{}{
		"type": "default", "ouId": ts.ouID, "attributes": map[string]interface{}{"name": "lifespan-crud-agent"},
		"validUntil": nil,
	})
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))
	_ = resp.Body.Close()
	agent = ts.getAgent(agentID)
	ts.Nil(agent.ValidUntil, "an unbounded lifespan omits validUntil")
	ts.Equal("UNBOUNDED", agent.Lifespan.Phase)
}

func (ts *AgentOAuthFlowsTestSuite) TestAgentLifespan_RegistrationRefusesInvalidValues() {
	for value, code := range map[string]string{
		"2027-03-31":           "AGT-1045",
		"2020-01-01T00:00:00Z": "AGT-1046",
	} {
		resp, err := doPost(testServerURL+agentBasePath, map[string]interface{}{
			"ouId": ts.ouID, "type": "default", "validUntil": value,
			"attributes": map[string]interface{}{"name": "lifespan-invalid-agent"},
		})
		ts.Require().NoError(err)
		ts.Equal(http.StatusBadRequest, resp.StatusCode, value)
		ts.Equal(code, errorCode(resp), value)
		_ = resp.Body.Close()
	}
}

// TestAgentLifespan_TokenCappedAtValidUntil verifies an access token never outlives the agent.
func (ts *AgentOAuthFlowsTestSuite) TestAgentLifespan_TokenCappedAtValidUntil() {
	agentID := ts.createLifespanAgent("lifespan-cap-agent", "lifespan_cap_client", lifespanEnd(2*time.Minute))
	defer func() { _ = deleteAgent(agentID) }()

	tokenResult, err := testutils.RequestTokenWithResource(
		"lifespan_cap_client", "lifespan_cap_client_secret", "", "", "client_credentials",
		agentOAuthDefaultRSIdentifier)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode, string(tokenResult.Body))
	ts.Greater(tokenResult.Token.ExpiresIn, float64(0))
	ts.LessOrEqual(tokenResult.Token.ExpiresIn, float64(120))
}

// TestAgentLifespan_ExpiredAgentObtainsNoToken verifies that the agent obtains tokens before validUntil,
// is refused once it passes, and that the agent is kept, reported as expired. Governance refuses an
// expired agent at client resolution, which answers invalid_client.
func (ts *AgentOAuthFlowsTestSuite) TestAgentLifespan_ExpiredAgentObtainsNoToken() {
	// Long enough that creating the agent and its first token request finish well before validUntil,
	// even on a slow runner.
	validUntil := lifespanEnd(20 * time.Second)
	agentID := ts.createLifespanAgent("lifespan-expiry-agent", "lifespan_expiry_client", validUntil)
	defer func() { _ = deleteAgent(agentID) }()

	requestToken := func() *testutils.TokenHTTPResult {
		result, err := testutils.RequestTokenWithResource(
			"lifespan_expiry_client", "lifespan_expiry_client_secret", "", "", "client_credentials",
			agentOAuthDefaultRSIdentifier)
		ts.Require().NoError(err)
		return result
	}

	tokenResult := requestToken()
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode, string(tokenResult.Body))

	// Poll until the expired agent's client is refused, allowing generous slack past validUntil.
	end, err := time.Parse(time.RFC3339, *validUntil)
	ts.Require().NoError(err)
	deadline := end.Add(15 * time.Second)
	for tokenResult.StatusCode != http.StatusUnauthorized && time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		tokenResult = requestToken()
	}
	ts.Require().Equal(http.StatusUnauthorized, tokenResult.StatusCode, string(tokenResult.Body))
	ts.Contains(string(tokenResult.Body), "invalid_client")

	agent := ts.getAgent(agentID)
	ts.Equal("EXPIRED", agent.Lifespan.Phase)
	ts.Require().NotNil(agent.Status)
	ts.Equal("EXPIRED", agent.Status.Value)

	// Moving validUntil into the future reactivates the agent, and the update's own response reports
	// the new status alongside the new phase.
	resp, err := putAgentRaw(agentID, map[string]interface{}{
		"type": "default", "ouId": ts.ouID, "attributes": map[string]interface{}{"name": "lifespan-expiry-agent"},
		"validUntil": *lifespanEnd(time.Hour),
	})
	ts.Require().NoError(err)
	body := readBody(resp)
	_ = resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, body)
	var reactivated Agent
	ts.Require().NoError(json.Unmarshal([]byte(body), &reactivated))
	ts.Require().NotNil(reactivated.Status)
	ts.Equal("ACTIVE", reactivated.Status.Value)
	ts.Require().NotNil(reactivated.Lifespan)
	ts.Equal("ACTIVE", reactivated.Lifespan.Phase)
}
