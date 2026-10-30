// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federated

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	syncTokenClientID     = "federated-sync-token-client"
	syncTokenClientSecret = "federated-sync-token-client-secret"
	syncTokenRedirectURI  = "http://localhost:3000/callback"
	syncTokenResource     = "https://federated-sync.example.com"
)

// syncTokenFixtures are the resource server, a role granting read on it, and a group whose role grants
// write on it, that the token scenarios map.
type syncTokenFixtures struct {
	roleID  string
	groupID string
}

// With session only attributes, the provider's attributes reach the ID token but only the required
// attributes are stored.
func (s *FederatedMappingSuite) TestSessionOnlyAttributesReachIDTokenWithoutBeingStored() {
	s.createSyncTokenApp()
	s.createSyncTokenFixtures()
	user := s.baseUser(s.nextSubject())
	config := mapping(fedPersonType.Handle,
		pair("email", "email"), pair("email", "username"), pair("given_name", "firstName"))
	config.Sync = &testutils.SyncConfiguration{Attributes: "session_only"}

	tokens, stored := s.obtainFederatedTokens(config, user, "openid profile")

	idClaims, err := testutils.DecodeJWT(tokens.IDToken)
	s.Require().NoError(err, "failed to decode the ID token")
	s.Equal(user.GivenName, idClaims.Additional["firstName"], "the session only attribute should be in the ID token")

	var attributes map[string]interface{}
	s.Require().NoError(json.Unmarshal(stored.Attributes, &attributes))
	s.NotContains(attributes, "firstName", "the session only attribute should not be stored")
}

// With session only groups and roles, a refresh token grant still grants the scopes the mapped role
// and group gave at sign-in, though neither is assigned to the user.
func (s *FederatedMappingSuite) TestSessionOnlyGroupsAndRolesSurviveRefresh() {
	s.createSyncTokenApp()
	fixtures := s.createSyncTokenFixtures()
	user := s.baseUser(s.nextSubject())
	user.Custom["groups"] = []interface{}{"platform-admins", "platform-editors"}
	config := sessionOnlyGroupsAndRoles(authzMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"platform-admins":  {roleTarget(fixtures.roleID)},
		"platform-editors": {groupTarget(fixtures.groupID)},
	}))

	tokens, stored := s.obtainFederatedTokens(config, user, "openid read write")
	signInScopes := s.accessTokenScopes(tokens.AccessToken)
	s.Require().Subset(signInScopes, []string{"read", "write"},
		"the mapped role and group should grant their scopes at sign-in")
	s.False(s.roleHasAssignee(fixtures.roleID, stored.ID), "the session only role should not be assigned")
	s.False(s.groupHasMember(fixtures.groupID, stored.ID), "the session only group should not be assigned")

	refreshed, err := testutils.RefreshAccessToken(syncTokenClientID, syncTokenClientSecret, tokens.RefreshToken)
	s.Require().NoError(err, "the refresh token grant should succeed")
	s.ElementsMatch(signInScopes, s.accessTokenScopes(refreshed.AccessToken),
		"the session only role and group should grant the same scopes after a refresh")
}

// Under no sync, a returning user's mapped role and group aren't stored or kept for the session, so
// they grant no scopes at sign-in, and a refresh doesn't grant them either.
func (s *FederatedMappingSuite) TestNoSyncMappedGroupsAndRolesGrantNothingOnRefresh() {
	s.createSyncTokenApp()
	fixtures := s.createSyncTokenFixtures()
	user := s.baseUser(s.nextSubject())
	_, stored := s.obtainFederatedTokens(
		mapping(fedPersonType.Handle, pair("email", "email"), pair("email", "username")), user, "openid")

	user.Custom["groups"] = []interface{}{"platform-admins", "platform-editors"}
	config := authzMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"platform-admins":  {roleTarget(fixtures.roleID)},
		"platform-editors": {groupTarget(fixtures.groupID)},
	})
	config.Sync = &testutils.SyncConfiguration{GroupsAndRoles: "no_sync"}

	tokens, returning := s.obtainFederatedTokens(config, user, "openid read write")
	s.Require().Equal(stored.ID, returning.ID, "the second sign-in should resolve to the same user")
	signInScopes := s.accessTokenScopes(tokens.AccessToken)
	s.NotContains(signInScopes, "read", "no sync should not grant the mapped role's scope")
	s.NotContains(signInScopes, "write", "no sync should not grant the mapped group's scope")
	s.False(s.roleHasAssignee(fixtures.roleID, stored.ID), "no sync should not assign the mapped role")
	s.False(s.groupHasMember(fixtures.groupID, stored.ID), "no sync should not add the mapped group")

	refreshed, err := testutils.RefreshAccessToken(syncTokenClientID, syncTokenClientSecret, tokens.RefreshToken)
	s.Require().NoError(err, "the refresh token grant should succeed")
	s.ElementsMatch(signInScopes, s.accessTokenScopes(refreshed.AccessToken),
		"a refresh should not grant scopes the sign-in didn't")
}

// createSyncTokenApp creates an application that signs the identity in through the authorization code
// grant, with a flow that provisions and checks authorization after the federated login. Both are torn
// down after the test.
func (s *FederatedMappingSuite) createSyncTokenApp() {
	s.T().Helper()
	flow := fedAuthzFlow
	flow.Name = "Federated Sync Token Flow"
	flow.Handle = "auth_flow_federated_sync_tokens"
	flowID, err := testutils.CreateFlow(flow)
	s.Require().NoError(err, "failed to create flow %s", flow.Handle)
	s.perTestFlowIDs = append(s.perTestFlowIDs, flowID)

	regFlowID, err := testutils.CreateIsolatedRegistrationFlow(syncTokenClientID + "-reg")
	s.Require().NoError(err, "failed to create the isolated registration flow")
	s.perTestFlowIDs = append(s.perTestFlowIDs, regFlowID)

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:               "Federated Sync Token App",
		OUID:               s.ouID,
		AuthFlowID:         flowID,
		RegistrationFlowID: regFlowID,
		AllowedUserTypes:   []string{fedPersonType.Handle},
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                syncTokenClientID,
				"clientSecret":            syncTokenClientSecret,
				"redirectUris":            []string{syncTokenRedirectURI},
				"grantTypes":              []string{"authorization_code", "refresh_token"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "client_secret_basic",
				"token": map[string]interface{}{
					"idToken": map[string]interface{}{"userAttributes": []string{"firstName"}},
				},
				"scopeClaims": map[string][]string{"profile": {"firstName"}},
			},
		}},
	})
	s.Require().NoError(err, "failed to create the sync token application")
	s.perTestAppIDs = append(s.perTestAppIDs, appID)
}

// createSyncTokenFixtures creates the scenario's resource server, role and group, removed after the test.
func (s *FederatedMappingSuite) createSyncTokenFixtures() syncTokenFixtures {
	s.T().Helper()
	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name: "Federated Sync API", Identifier: syncTokenResource, OUID: s.ouID,
	}, []testutils.Action{
		{Name: "Read", Handle: "read", Description: "Read access"},
		{Name: "Write", Handle: "write", Description: "Write access"},
	})
	s.Require().NoError(err, "failed to create the sync resource server")
	s.T().Cleanup(func() {
		if err := testutils.DeleteResourceServerWithChildren(rsID); err != nil {
			s.T().Logf("failed to delete the sync resource server: %v", err)
		}
	})

	roleID := s.createSyncRole(testutils.Role{
		Name: "Federated Sync Reader", OUID: s.ouID,
		Permissions: []testutils.ResourcePermissions{{ResourceServerID: rsID, Permissions: []string{"read"}}},
	})
	groupID, err := testutils.CreateGroup(testutils.Group{Name: "Federated Sync Editors", OUID: s.ouID})
	s.Require().NoError(err, "failed to create the sync group")
	s.T().Cleanup(func() {
		if err := testutils.DeleteGroup(groupID); err != nil {
			s.T().Logf("failed to delete the sync group: %v", err)
		}
	})
	s.createSyncRole(testutils.Role{
		Name: "Federated Sync Writer", OUID: s.ouID,
		Permissions: []testutils.ResourcePermissions{{ResourceServerID: rsID, Permissions: []string{"write"}}},
		Assignments: []testutils.Assignment{{ID: groupID, Type: "group"}},
	})
	return syncTokenFixtures{roleID: roleID, groupID: groupID}
}

func (s *FederatedMappingSuite) createSyncRole(role testutils.Role) string {
	s.T().Helper()
	roleID, err := testutils.CreateRole(role)
	s.Require().NoError(err, "failed to create role %s", role.Name)
	s.T().Cleanup(func() {
		if err := testutils.DeleteRole(roleID); err != nil {
			s.T().Logf("failed to delete role %s: %v", role.Name, err)
		}
	})
	return roleID
}

// obtainFederatedTokens signs the identity in through the application createSyncTokenApp made, and
// returns the tokens and the user the sign-in resolved.
func (s *FederatedMappingSuite) obtainFederatedTokens(
	config *testutils.AttributeConfiguration, user *testutils.OIDCUserInfo, scope string,
) (*testutils.TokenResponse, *testutils.User) {
	s.T().Helper()
	s.applyConfig(config)
	s.mockOIDC.AddUser(user)
	s.activeSub = user.Sub

	resp, err := testutils.InitiateAuthorizationFlowWithResource(
		syncTokenClientID, syncTokenRedirectURI, "code", scope, "test-state", syncTokenResource)
	s.Require().NoError(err, "failed to initiate the authorization request")
	defer resp.Body.Close()
	s.Require().Equal(http.StatusFound, resp.StatusCode, "expected a redirect from the authorization endpoint")
	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	s.Require().NoError(err, "failed to read the authorization redirect %s", resp.Header.Get("Location"))

	step, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	s.Require().NoError(err, "failed to start the authentication flow")
	s.Require().Equal("REDIRECTION", step.Type, "expected a redirection to the identity provider, got %+v", step)
	code, state, err := testutils.SimulateFederatedOAuthFlow(step.Data.RedirectURL)
	s.Require().NoError(err, "failed to simulate authorization at the identity provider")
	step, err = testutils.ExecuteAuthenticationFlow(
		executionID, map[string]string{"code": code, "state": state}, "", step.ChallengeToken)
	s.Require().NoError(err, "failed to complete the authentication flow")
	s.Require().Equal("COMPLETE", step.FlowStatus, "expected the flow to complete, got %+v", step)

	signedIn, err := testutils.GetUserFromAssertion(step.Assertion)
	s.Require().NoError(err, "failed to look up the user the assertion was issued for")
	s.config.CreatedUserIDs = append(s.config.CreatedUserIDs, signedIn.ID)

	authz, err := testutils.CompleteAuthorization(authID, step.Assertion)
	s.Require().NoError(err, "failed to complete the authorization")
	authCode, err := testutils.ExtractAuthorizationCode(authz.RedirectURI)
	s.Require().NoError(err, "failed to read the authorization code")
	result, err := testutils.RequestTokenWithResource(syncTokenClientID, syncTokenClientSecret, authCode,
		syncTokenRedirectURI, "authorization_code", syncTokenResource)
	s.Require().NoError(err, "failed to request tokens")
	s.Require().Equal(http.StatusOK, result.StatusCode, "the token request should succeed: %s", string(result.Body))
	return result.Token, signedIn
}

func (s *FederatedMappingSuite) accessTokenScopes(accessToken string) []string {
	s.T().Helper()
	claims, err := testutils.DecodeJWT(accessToken)
	s.Require().NoError(err, "failed to decode the access token")
	scope, _ := claims.Additional["scope"].(string)
	return strings.Fields(scope)
}

func (s *FederatedMappingSuite) groupHasMember(groupID, userID string) bool {
	s.T().Helper()
	members, err := testutils.GetGroupMembers(groupID)
	s.Require().NoError(err, "failed to read group members")
	for _, m := range members {
		if m.ID == userID {
			return true
		}
	}
	return false
}

func (s *FederatedMappingSuite) roleHasAssignee(roleID, userID string) bool {
	s.T().Helper()
	assignments, err := testutils.GetRoleAssignments(roleID)
	s.Require().NoError(err, "failed to read role assignments")
	for _, a := range assignments {
		if a.ID == userID {
			return true
		}
	}
	return false
}
