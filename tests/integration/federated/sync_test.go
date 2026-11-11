// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federated

import (
	"encoding/json"
	"maps"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	syncTokenClientID     = "federated-sync-token-client"
	syncTokenClientSecret = "federated-sync-token-client-secret"
	syncTokenRedirectURI  = "http://localhost:3000/callback"
	syncTokenResource     = "https://federated-sync.example.com"

	// The declarative fixtures: decl-role-1 grants declarativeSyncScope on declarativeSyncResource, and is
	// assigned to decl-group-1. Both belong to decl-ou-1.
	declarativeSyncOUID     = "decl-ou-1"
	declarativeSyncRoleID   = "decl-role-1"
	declarativeSyncGroupID  = "decl-group-1"
	declarativeSyncResource = "https://localhost:8090/decl-rs-1"
	declarativeSyncScope    = "test-resource:read"
)

// syncTokenFixtures are the resource server, a role granting read on it, and a group whose role grants
// write on it, that the token scenarios map.
type syncTokenFixtures struct {
	resourceServerID string
	roleID           string
	groupID          string
}

// With session only attributes, the provider's attributes reach the ID token but only the required
// attributes are stored.
func (s *FederatedMappingSuite) TestSessionOnlyAttributesReachIDTokenWithoutBeingStored() {
	s.createSyncTokenApp(s.idpID, s.ouID, fedPersonType.Handle)
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
	s.createSyncTokenApp(s.idpID, s.ouID, fedPersonType.Handle)
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
	s.assertGroupsAndRolesClaims(tokens.AccessToken)

	refreshed, err := testutils.RefreshAccessToken(s.syncClientID, syncTokenClientSecret, tokens.RefreshToken)
	s.Require().NoError(err, "the refresh token grant should succeed")
	s.ElementsMatch(signInScopes, s.accessTokenScopes(refreshed.AccessToken),
		"the session only role and group should grant the same scopes after a refresh")
	s.assertGroupsAndRolesClaims(refreshed.AccessToken)
}

// A permission the mapping grants directly is never stored, so a refresh token grant still grants it.
func (s *FederatedMappingSuite) TestMappedPermissionsSurviveRefresh() {
	s.createSyncTokenApp(s.idpID, s.ouID, fedPersonType.Handle)
	fixtures := s.createSyncTokenFixtures()
	user := s.baseUser(s.nextSubject())
	user.Custom["groups"] = []interface{}{"platform-readers"}
	config := authzMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"platform-readers": {permissionTarget(fixtures.resourceServerID, "read")},
	})

	tokens, _ := s.obtainFederatedTokens(config, user, "openid read write")
	signInScopes := s.accessTokenScopes(tokens.AccessToken)
	s.Require().Contains(signInScopes, "read", "the mapped permission should be granted at sign-in")
	s.NotContains(signInScopes, "write", "only the mapped permission should be granted")

	refreshed, err := testutils.RefreshAccessToken(s.syncClientID, syncTokenClientSecret, tokens.RefreshToken)
	s.Require().NoError(err, "the refresh token grant should succeed")
	s.ElementsMatch(signInScopes, s.accessTokenScopes(refreshed.AccessToken),
		"the mapped permission should be granted after a refresh")
}

// Under no sync, a returning user's mapped role and group aren't stored or kept for the session, so
// they grant no scopes at sign-in, and a refresh doesn't grant them either.
func (s *FederatedMappingSuite) TestNoSyncMappedGroupsAndRolesGrantNothingOnRefresh() {
	s.createSyncTokenApp(s.idpID, s.ouID, fedPersonType.Handle)
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

	refreshed, err := testutils.RefreshAccessToken(s.syncClientID, syncTokenClientSecret, tokens.RefreshToken)
	s.Require().NoError(err, "the refresh token grant should succeed")
	s.ElementsMatch(signInScopes, s.accessTokenScopes(refreshed.AccessToken),
		"a refresh should not grant scopes the sign-in didn't")
}

// With roles and groups kept only in declarative files, the account can't hold a mapped role or group, so
// either applies to the session whatever the sync mode: it grants its scope at sign-in and after a
// refresh, and nothing is stored.
func (s *FederatedMappingSuite) TestDeclarativeGroupsAndRolesApplyToTheSession() {
	// The application and user live in the fixtures' organization unit, where the user can be assigned.
	userType := s.createDeclarativeSyncUserType()
	config := mapping(userType, pair("email", "email"), pair("email", "username"))
	config.AuthorizationMapping = authzMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"decl-readers": {roleTarget(declarativeSyncRoleID)},
		"decl-members": {groupTarget(declarativeSyncGroupID)},
	}).AuthorizationMapping
	s.createSyncTokenApp(s.createDeclarativeSyncConnection(config), declarativeSyncOUID, userType)
	byRole := s.baseUser(s.nextSubject())
	byRole.Custom["groups"] = []interface{}{"decl-readers"}
	byGroup := s.baseUser(s.nextSubject())
	byGroup.Custom["groups"] = []interface{}{"decl-members"}

	restore := s.restartWithDeclarativeGroupsAndRoles()
	defer restore()
	var signedIn []string
	for _, user := range []*testutils.OIDCUserInfo{byRole, byGroup} {
		tokens, stored := s.signInForTokens(user, "openid "+declarativeSyncScope, declarativeSyncResource)
		signedIn = append(signedIn, stored.ID)
		s.Contains(s.accessTokenScopes(tokens.AccessToken), declarativeSyncScope,
			"the declarative mapping should grant its scope at sign-in")

		refreshed, err := testutils.RefreshAccessToken(s.syncClientID, syncTokenClientSecret, tokens.RefreshToken)
		s.Require().NoError(err, "the refresh token grant should succeed")
		s.Contains(s.accessTokenScopes(refreshed.AccessToken), declarativeSyncScope,
			"the declarative mapping should grant its scope after a refresh")
	}

	// Back on the composite stores, an assignment the sign-ins had written would be readable.
	restore()
	for _, userID := range signedIn {
		s.False(s.roleHasAssignee(declarativeSyncRoleID, userID), "the declarative role should not be assigned")
		s.False(s.groupHasMember(declarativeSyncGroupID, userID), "the declarative group should not be assigned")
	}
}

// createDeclarativeSyncUserType creates a user type like fedPersonType in the declarative fixtures'
// organization unit, removed with the suite, and returns its handle.
func (s *FederatedMappingSuite) createDeclarativeSyncUserType() string {
	s.T().Helper()
	userType := fedPersonType
	userType.Handle = "fed_decl_person"
	userType.DisplayName = "Fed Declarative Person"
	userType.OUID = declarativeSyncOUID
	typeID, err := testutils.CreateUserType(userType)
	s.Require().NoError(err, "failed to create the declarative sync user type")
	s.typeIDs = append(s.typeIDs, typeID)
	return userType.Handle
}

// createDeclarativeSyncConnection creates a connection with config in the declarative fixtures'
// organization unit, removed after the test, since a connection maps only the roles and groups its unit
// owns or has shared with it.
func (s *FederatedMappingSuite) createDeclarativeSyncConnection(config *testutils.AttributeConfiguration) string {
	s.T().Helper()
	connection := s.idpFixture(config)
	connection.Name = "Federated Sync Declarative Connection"
	connection.OUID = declarativeSyncOUID
	idpID, err := testutils.CreateIDP(connection)
	s.Require().NoError(err, "failed to create the declarative sync connection")
	s.T().Cleanup(func() {
		if err := testutils.DeleteIDP(idpID); err != nil {
			s.T().Logf("failed to delete the declarative sync connection: %v", err)
		}
	})
	return idpID
}

// restartWithDeclarativeGroupsAndRoles restarts the server with the group and role stores declarative, and
// returns a function that puts them back to composite. Later calls of the returned function do nothing.
// The admin token from before the restart is kept, since a declarative role store holds no admin role.
func (s *FederatedMappingSuite) restartWithDeclarativeGroupsAndRoles() func() {
	s.T().Helper()
	restored := false
	restore := func() {
		if restored {
			return
		}
		restored = true
		s.Assert().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{
			"group": map[string]interface{}{"store": "composite"},
			"role":  map[string]interface{}{"store": "composite"},
		}))
		s.Assert().NoError(testutils.RestartServer())
		s.Assert().NoError(testutils.ObtainAdminAccessToken())
	}
	// Registered before the patch, so a failed restart still puts the stores back.
	s.T().Cleanup(restore)
	s.Require().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{
		"group": map[string]interface{}{"store": "declarative"},
		"role":  map[string]interface{}{"store": "declarative"},
	}))
	s.Require().NoError(testutils.RestartServer())
	return restore
}

// createSyncTokenApp creates an application in the organization unit ouID that signs a userType identity
// in over the connection idpID through the authorization code grant, with a flow that provisions and
// checks authorization after the federated login. Both are torn down after the test. Each test gets a
// client id and resource of its own, since deleting the last test's application and fixtures revokes
// what was issued for them.
func (s *FederatedMappingSuite) createSyncTokenApp(idpID, ouID, userType string) {
	s.T().Helper()
	suffix := s.nextSubject()
	s.syncClientID = syncTokenClientID + "-" + suffix
	s.syncResource = syncTokenResource + "/" + suffix
	flow := fedAuthzFlow
	flow.Name = "Federated Sync Token Flow"
	flow.Handle = "auth_flow_federated_sync_tokens"
	nodes := make([]map[string]interface{}, 0, len(fedAuthzFlow.Nodes.([]map[string]interface{})))
	for _, node := range fedAuthzFlow.Nodes.([]map[string]interface{}) {
		if node["id"] == "oidc_auth" {
			node = maps.Clone(node)
			properties := maps.Clone(node["properties"].(map[string]interface{}))
			properties["idpId"] = idpID
			node["properties"] = properties
		}
		nodes = append(nodes, node)
	}
	flow.Nodes = nodes
	flowID, err := testutils.CreateFlow(flow)
	s.Require().NoError(err, "failed to create flow %s", flow.Handle)
	s.perTestFlowIDs = append(s.perTestFlowIDs, flowID)

	regFlowID, err := testutils.CreateIsolatedRegistrationFlow(s.syncClientID + "-reg")
	s.Require().NoError(err, "failed to create the isolated registration flow")
	s.perTestFlowIDs = append(s.perTestFlowIDs, regFlowID)

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:               "Federated Sync Token App",
		OUID:               ouID,
		AuthFlowID:         flowID,
		RegistrationFlowID: regFlowID,
		AllowedUserTypes:   []string{userType},
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                s.syncClientID,
				"clientSecret":            syncTokenClientSecret,
				"redirectUris":            []string{syncTokenRedirectURI},
				"grantTypes":              []string{"authorization_code", "refresh_token"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "client_secret_basic",
				"token": map[string]interface{}{
					"idToken": map[string]interface{}{"userAttributes": []string{"firstName"}},
					"accessToken": map[string]interface{}{
						"userConfig": map[string]interface{}{"attributes": []string{"groups", "roles"}},
					},
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
		Name: "Federated Sync API", Identifier: s.syncResource, OUID: s.ouID,
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
	return syncTokenFixtures{resourceServerID: rsID, roleID: roleID, groupID: groupID}
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
	return s.signInForTokens(user, scope, s.syncResource)
}

// signInForTokens signs the identity in through the application createSyncTokenApp made, under the
// connection's current configuration, and returns tokens for resource and the user the sign-in resolved.
func (s *FederatedMappingSuite) signInForTokens(
	user *testutils.OIDCUserInfo, scope, resource string,
) (*testutils.TokenResponse, *testutils.User) {
	s.T().Helper()
	s.mockOIDC.AddUser(user)
	s.activeSub = user.Sub

	resp, err := testutils.InitiateAuthorizationFlowWithResource(
		s.syncClientID, syncTokenRedirectURI, "code", scope, "test-state", resource)
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
	result, err := testutils.RequestTokenWithResource(s.syncClientID, syncTokenClientSecret, authCode,
		syncTokenRedirectURI, "authorization_code", resource)
	s.Require().NoError(err, "failed to request tokens")
	s.Require().Equal(http.StatusOK, result.StatusCode, "the token request should succeed: %s", string(result.Body))
	return result.Token, signedIn
}

// assertGroupsAndRolesClaims checks that the access token carries the session only group and role the
// mapping granted, and the role the group holds.
func (s *FederatedMappingSuite) assertGroupsAndRolesClaims(accessToken string) {
	s.T().Helper()
	claims, err := testutils.DecodeJWT(accessToken)
	s.Require().NoError(err, "failed to decode the access token")
	s.ElementsMatch([]interface{}{"Federated Sync Editors"}, claims.Additional["groups"],
		"the groups claim should carry the session only group")
	s.ElementsMatch([]interface{}{"Federated Sync Reader", "Federated Sync Writer"}, claims.Additional["roles"],
		"the roles claim should carry the session only role and the session only group's role")
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
