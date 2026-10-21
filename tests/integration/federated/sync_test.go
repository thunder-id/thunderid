// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federated

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	syncTokenClientID     = "federated-sync-token-client"
	syncTokenClientSecret = "federated-sync-token-client-secret"
	syncTokenRedirectURI  = "http://localhost:3000/callback"
)

// createSyncUser creates the local account a returning federated identity resolves to by its subject.
func (s *FederatedMappingSuite) createSyncUser(user *testutils.OIDCUserInfo) string {
	s.T().Helper()
	userID, err := testutils.CreateUser(testutils.User{
		Type: fedPersonType.Name,
		OUID: s.ouID,
		Attributes: mustJSON(map[string]interface{}{
			"username": user.Email, "email": user.Email, "sub": user.Sub, "firstName": "Old", "city": "Colombo",
		}),
	})
	s.Require().NoError(err, "failed to create the returning user")
	s.T().Cleanup(func() {
		if err := testutils.DeleteUser(userID); err != nil {
			s.T().Logf("failed to delete the returning user: %v", err)
		}
	})
	return userID
}

// signInAndReadAttributes signs the identity in through the flow without provisioning and returns the
// account's stored attributes.
func (s *FederatedMappingSuite) signInAndReadAttributes(
	config *testutils.AttributeConfiguration, user *testutils.OIDCUserInfo,
) map[string]interface{} {
	s.T().Helper()
	step, err := s.authenticate(s.strictAuthAppID, config, user)
	s.Require().NoError(err, "the returning identity should sign in")
	s.Require().Equal("COMPLETE", step.FlowStatus, "the returning identity should sign in, got %+v", step)

	stored, err := testutils.FindUserByAttribute("sub", user.Sub)
	s.Require().NoError(err, "failed to look up the returning user")
	s.Require().NotNil(stored, "the returning user should exist")
	var attributes map[string]interface{}
	s.Require().NoError(json.Unmarshal(stored.Attributes, &attributes))
	return attributes
}

func syncAttributeConfig(mode string) *testutils.AttributeConfiguration {
	config := mapping(fedPersonType.Name,
		pair("email", "email"), pair("email", "username"), pair("given_name", "firstName"))
	config.Sync = &testutils.SyncConfiguration{Attributes: mode}
	return config
}

// With override received, a claim the attribute mappings don't name is not written, even when it
// shares a local attribute's name.
func (s *FederatedMappingSuite) TestOverrideReceivedSkipsUnmappedAttributes() {
	user := s.baseUser(s.nextSubject())
	user.Custom["city"] = "Kandy"
	s.createSyncUser(user)

	attributes := s.signInAndReadAttributes(syncAttributeConfig("override_received"), user)

	s.Equal("Colombo", attributes["city"])
	s.Equal(user.GivenName, attributes["firstName"])
}

// With override received, the attributes the provider sends replace the stored ones, and the rest are kept.
func (s *FederatedMappingSuite) TestOverrideReceivedAttributesKeepsUnsentAttributes() {
	user := s.baseUser(s.nextSubject())
	s.createSyncUser(user)

	attributes := s.signInAndReadAttributes(syncAttributeConfig("override_received"), user)

	s.Equal(map[string]interface{}{
		"username": user.Email, "email": user.Email, "sub": user.Sub, "firstName": user.GivenName,
		"city": "Colombo",
	}, attributes)
}

// With no sync, a returning sign-in leaves the stored attributes alone.
func (s *FederatedMappingSuite) TestNoSyncAttributesLeavesStoredAttributes() {
	user := s.baseUser(s.nextSubject())
	s.createSyncUser(user)

	attributes := s.signInAndReadAttributes(syncAttributeConfig("no_sync"), user)

	s.Equal("Old", attributes["firstName"])
	s.Equal("Colombo", attributes["city"])
}

// With override received, the account is synced with its own user type's mapping, even when the claims
// resolve to another type whose mapping writes the same local attribute.
func (s *FederatedMappingSuite) TestOverrideReceivedUsesTheAccountsUserTypeMapping() {
	user := s.baseUser(s.nextSubject())
	s.createSyncUser(user)

	config := syncAttributeConfig("override_received")
	config.UserTypeResolution = &testutils.UserTypeResolution{Default: fedContractorType.Name}
	config.UserTypeAttributeMappings = append(config.UserTypeAttributeMappings, testutils.UserTypeAttributeMapping{
		UserType: fedContractorType.Name,
		Attributes: []testutils.AttributeMapping{
			pair("email", "email"), pair("email", "username"), pair("family_name", "firstName"),
			pair("given_name", "employeeNumber"),
		},
	})

	attributes := s.signInAndReadAttributes(config, user)

	s.Equal(user.GivenName, attributes["firstName"])
	s.NotContains(attributes, "employeeNumber")
	s.Equal("Colombo", attributes["city"])
}

// An account matched by an account-linking attribute is synced like any other returning user.
func (s *FederatedMappingSuite) TestOverrideReceivedSyncsUserMatchedByAccountLinking() {
	user := s.baseUser(s.nextSubject())
	userID, err := testutils.CreateUser(testutils.User{
		Type: fedPersonType.Name,
		OUID: s.ouID,
		Attributes: mustJSON(map[string]interface{}{
			"username": user.Email, "email": user.Email, "firstName": "Old", "city": "Colombo",
		}),
	})
	s.Require().NoError(err, "failed to create the user the identity should link to")
	s.T().Cleanup(func() {
		if err := testutils.DeleteUser(userID); err != nil {
			s.T().Logf("failed to delete the linked user: %v", err)
		}
	})
	config := syncAttributeConfig("override_received")
	config.AccountLinking = &testutils.AccountLinking{Attributes: []string{"email"}}

	step, err := s.authenticate(s.strictAuthAppID, config, user)
	s.Require().NoError(err, "the linked identity should sign in")
	s.Require().Equal("COMPLETE", step.FlowStatus, "the linked identity should sign in, got %+v", step)

	stored, err := testutils.FindUserByAttribute("email", user.Email)
	s.Require().NoError(err, "failed to read the linked user")
	s.Require().NotNil(stored, "the linked user should exist")
	s.Require().Equal(userID, stored.ID, "the identity should resolve to the linked user")
	var attributes map[string]interface{}
	s.Require().NoError(json.Unmarshal(stored.Attributes, &attributes))
	s.Equal(map[string]interface{}{
		"username": user.Email, "email": user.Email, "firstName": user.GivenName, "city": "Colombo",
	}, attributes)
}

// With override all groups and roles, the account's direct groups and roles match what the mapping
// resolves: mapped ones are added, and ones assigned in ThunderID that the provider doesn't send are
// removed.
func (s *FederatedMappingSuite) TestOverrideAllGroupsAndRolesMatchesMapping() {
	user := s.baseUser(s.nextSubject())
	userID := s.createSyncUser(user)

	mappedGroupID := s.createSyncGroup("Federated Sync Mapped Group", nil)
	manualGroupID := s.createSyncGroup("Federated Sync Manual Group", []testutils.Member{{Id: userID, Type: "user"}})
	mappedRoleID := s.createSyncRole("Federated Sync Mapped Role", nil)
	manualRoleID := s.createSyncRole("Federated Sync Manual Role", []testutils.Assignment{{ID: userID, Type: "user"}})

	user.Custom["groups"] = []interface{}{"engineering"}
	config := authzMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"engineering": {groupTarget(mappedGroupID), roleTarget(mappedRoleID)},
	})
	config.Sync = &testutils.SyncConfiguration{Attributes: "no_sync", GroupsAndRoles: "override_all"}

	s.signInAndReadAttributes(config, user)

	s.True(s.groupHasMember(mappedGroupID, userID), "the mapped group should be added")
	s.False(s.groupHasMember(manualGroupID, userID), "a group the provider doesn't send should be removed")
	s.True(s.roleHasAssignee(mappedRoleID, userID), "the mapped role should be assigned")
	s.False(s.roleHasAssignee(manualRoleID, userID), "a role the provider doesn't send should be unassigned")
}

// Under override all, a direct mapping name that matches groups in two organization units grants
// neither, and keeps the membership the user already has in one of them.
func (s *FederatedMappingSuite) TestOverrideAllKeepsMembershipOfAmbiguousGroupName() {
	user := s.baseUser(s.nextSubject())
	userID := s.createSyncUser(user)

	otherOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "federated-sync-ambiguous-" + s.nextSubject(),
		Name:   "Federated Sync Ambiguous OU",
	})
	s.Require().NoError(err, "failed to create the second organization unit")
	s.T().Cleanup(func() {
		if err := testutils.DeleteOrganizationUnit(otherOUID); err != nil {
			s.T().Logf("failed to delete the second organization unit: %v", err)
		}
	})

	name := "Federated Sync Ambiguous " + s.nextSubject()
	memberGroupID := s.createSyncGroup(name, []testutils.Member{{Id: userID, Type: "user"}})
	otherGroupID, err := testutils.CreateGroup(testutils.Group{Name: name, OUID: otherOUID})
	s.Require().NoError(err, "failed to create the duplicate-named group")
	s.T().Cleanup(func() {
		if err := testutils.DeleteGroup(otherGroupID); err != nil {
			s.T().Logf("failed to delete the duplicate-named group: %v", err)
		}
	})
	manualGroupID := s.createSyncGroup("Federated Sync Ambiguous Manual Group",
		[]testutils.Member{{Id: userID, Type: "user"}})

	user.Custom["group_name"] = name
	config := authzDirectMapping("group_name", "", testutils.AuthorizationTargetGroup, "")
	config.Sync = &testutils.SyncConfiguration{Attributes: "no_sync", GroupsAndRoles: "override_all"}

	s.signInAndReadAttributes(config, user)

	s.True(s.groupHasMember(memberGroupID, userID), "the membership in the ambiguous group should be kept")
	s.False(s.groupHasMember(otherGroupID, userID), "the other ambiguous group should not be added")
	s.False(s.groupHasMember(manualGroupID, userID), "a group the provider doesn't send should be removed")
}

// A mapping that targets a group and role deleted after it was saved doesn't fail a new user's
// registration: the deleted targets are skipped and the live ones are assigned.
func (s *FederatedMappingSuite) TestProvisioningSkipsDeletedMappedGroupAndRole() {
	liveGroupID := s.createSyncGroup("Federated Sync Live Group", nil)
	liveRoleID := s.createSyncRole("Federated Sync Live Role", nil)
	deletedGroupID, err := testutils.CreateGroup(testutils.Group{Name: "Federated Sync Deleted Group", OUID: s.ouID})
	s.Require().NoError(err, "failed to create the group to delete")
	deletedRoleID, err := testutils.CreateRole(testutils.Role{Name: "Federated Sync Deleted Role", OUID: s.ouID})
	s.Require().NoError(err, "failed to create the role to delete")

	user := s.baseUser(s.nextSubject())
	user.Custom["groups"] = []interface{}{"engineering"}
	s.applyConfig(authzMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"engineering": {
			groupTarget(liveGroupID), groupTarget(deletedGroupID),
			roleTarget(liveRoleID), roleTarget(deletedRoleID),
		},
	}))
	s.Require().NoError(testutils.DeleteGroup(deletedGroupID), "failed to delete the mapped group")
	s.Require().NoError(testutils.DeleteRole(deletedRoleID), "failed to delete the mapped role")

	s.registerWithAppliedConfig(user)

	provisioned, err := testutils.FindUserByAttribute("sub", user.Sub)
	s.Require().NoError(err, "failed to look up the provisioned user")
	s.True(s.groupHasMember(liveGroupID, provisioned.ID), "the live mapped group should be assigned")
	s.True(s.roleHasAssignee(liveRoleID, provisioned.ID), "the live mapped role should be assigned")
}

func (s *FederatedMappingSuite) createSyncGroup(name string, members []testutils.Member) string {
	s.T().Helper()
	groupID, err := testutils.CreateGroup(testutils.Group{Name: name, OUID: s.ouID, Members: members})
	s.Require().NoError(err, "failed to create group %s", name)
	s.T().Cleanup(func() {
		if err := testutils.DeleteGroup(groupID); err != nil {
			s.T().Logf("failed to delete group %s: %v", name, err)
		}
	})
	return groupID
}

func (s *FederatedMappingSuite) createSyncRole(name string, assignments []testutils.Assignment) string {
	s.T().Helper()
	roleID, err := testutils.CreateRole(testutils.Role{Name: name, OUID: s.ouID, Assignments: assignments})
	s.Require().NoError(err, "failed to create role %s", name)
	s.T().Cleanup(func() {
		if err := testutils.DeleteRole(roleID); err != nil {
			s.T().Logf("failed to delete role %s: %v", name, err)
		}
	})
	return roleID
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

// With no sync configuration, attributes default to override received and groups and roles to no sync:
// the mapped attribute is updated, and the groups and roles assigned in ThunderID are kept.
func (s *FederatedMappingSuite) TestDefaultSyncModesOverrideAttributesAndKeepGroupsAndRoles() {
	user := s.baseUser(s.nextSubject())
	userID := s.createSyncUser(user)
	manualGroupID := s.createSyncGroup("Federated Default Sync Manual Group",
		[]testutils.Member{{Id: userID, Type: "user"}})
	manualRoleID := s.createSyncRole("Federated Default Sync Manual Role",
		[]testutils.Assignment{{ID: userID, Type: "user"}})
	mappedGroupID := s.createSyncGroup("Federated Default Sync Mapped Group", nil)

	user.Custom["groups"] = []interface{}{"engineering"}
	config := authzMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"engineering": {groupTarget(mappedGroupID)},
	})
	config.UserTypeAttributeMappings = syncAttributeConfig("").UserTypeAttributeMappings

	attributes := s.signInAndReadAttributes(config, user)

	s.Equal(user.GivenName, attributes["firstName"], "attributes should default to override received")
	s.True(s.groupHasMember(manualGroupID, userID), "groups and roles should default to no sync")
	s.True(s.roleHasAssignee(manualRoleID, userID), "groups and roles should default to no sync")
	s.False(s.groupHasMember(mappedGroupID, userID), "no sync should not add a returning user to a mapped group")
}

// A registration flow that allows an existing user signs a returning identity in and syncs it like an
// authentication flow does.
func (s *FederatedMappingSuite) TestRegistrationWithExistingUserSyncsAttributes() {
	user := s.baseUser(s.nextSubject())
	s.createSyncUser(user)

	regFlowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "Federated Sync Registration With Existing User Flow",
		FlowType: "REGISTRATION",
		Handle:   "registration_flow_federated_sync_existing_user",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "user_type_resolver"},
			{
				"id":        "user_type_resolver",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "UserTypeResolver"},
				"onSuccess": "oidc_auth",
			},
			{
				"id":   "oidc_auth",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"idpId":                             s.idpID,
					"allowRegistrationWithExistingUser": true,
				},
				"executor":  map[string]interface{}{"name": "OIDCAuthExecutor"},
				"onSuccess": "provisioning",
			},
			{
				"id":        "provisioning",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "ProvisioningExecutor"},
				"onSuccess": "auth_assert",
			},
			{
				"id":        "auth_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	})
	s.Require().NoError(err, "failed to create the registration flow")
	s.perTestFlowIDs = append(s.perTestFlowIDs, regFlowID)

	authFlowID, err := testutils.CreateIsolatedAuthFlow("federated-sync-existing-user-auth")
	s.Require().NoError(err, "failed to create the isolated authentication flow")
	s.perTestFlowIDs = append(s.perTestFlowIDs, authFlowID)

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:                      "Federated Sync Registration With Existing User App",
		ClientID:                  "federated-sync-existing-user-client",
		ClientSecret:              "federated-sync-existing-user-secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{fedPersonType.Name},
		OUID:                      s.ouID,
		IsRegistrationFlowEnabled: true,
		AuthFlowID:                authFlowID,
		RegistrationFlowID:        regFlowID,
		AssertionConfig:           map[string]interface{}{"userAttributes": []string{"userType", "ouId"}},
	})
	s.Require().NoError(err, "failed to create the application")
	s.perTestAppIDs = append(s.perTestAppIDs, appID)

	s.applyConfig(syncAttributeConfig("override_received"))
	s.mockOIDC.AddUser(user)
	s.activeSub = user.Sub

	flowStep, err := common.InitiateRegistrationFlow(appID, false, nil, "")
	s.Require().NoError(err, "failed to initiate the registration flow")
	s.Require().Equal("REDIRECTION", flowStep.Type, "expected a redirection, got %+v", flowStep)
	code, state, err := testutils.SimulateFederatedOAuthFlow(flowStep.Data.RedirectURL)
	s.Require().NoError(err, "failed to simulate authorization at the identity provider")
	completed, err := common.CompleteFlow(
		flowStep.ExecutionID, map[string]string{"code": code, "state": state}, "", flowStep.ChallengeToken)
	s.Require().NoError(err, "failed to complete the registration flow")
	s.Require().Equal("COMPLETE", completed.FlowStatus, "the existing user should sign in, got %+v", completed)

	stored, err := testutils.FindUserByAttribute("sub", user.Sub)
	s.Require().NoError(err, "failed to read the existing user")
	s.Require().NotNil(stored, "the existing user should still exist")
	var attributes map[string]interface{}
	s.Require().NoError(json.Unmarshal(stored.Attributes, &attributes))
	s.Equal(user.GivenName, attributes["firstName"], "the mapped attribute should be synced")
	s.Equal("Colombo", attributes["city"], "an attribute the provider doesn't send should be kept")
}

// Override all fails the sign-in when the account can't be written, here because the synced username
// is already another user's. Nothing is written: the groups and roles stay as they were. A failed
// direct lookup can't be forced here, since role, group and permission lookups only fail on a database
// error, so the schema rule is the failure this covers.
func (s *FederatedMappingSuite) TestOverrideAllFailsSignInOnSchemaRule() {
	user := s.baseUser(s.nextSubject())
	userID := s.createSyncUser(user)

	takenUsername := s.nextSubject() + "@example.com"
	otherID, err := testutils.CreateUser(testutils.User{
		Type:       fedPersonType.Name,
		OUID:       s.ouID,
		Attributes: mustJSON(map[string]interface{}{"username": takenUsername, "email": takenUsername}),
	})
	s.Require().NoError(err, "failed to create the user holding the username")
	s.T().Cleanup(func() {
		if err := testutils.DeleteUser(otherID); err != nil {
			s.T().Logf("failed to delete the user holding the username: %v", err)
		}
	})

	manualGroupID := s.createSyncGroup("Federated Sync Rejected Manual Group",
		[]testutils.Member{{Id: userID, Type: "user"}})
	mappedGroupID := s.createSyncGroup("Federated Sync Rejected Mapped Group", nil)

	user.GivenName = takenUsername
	user.Custom["groups"] = []interface{}{"engineering"}
	config := authzMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"engineering": {groupTarget(mappedGroupID)},
	})
	config.UserTypeAttributeMappings = mapping(fedPersonType.Name,
		pair("email", "email"), pair("given_name", "username")).UserTypeAttributeMappings
	config.Sync = &testutils.SyncConfiguration{Attributes: "override_received", GroupsAndRoles: "override_all"}

	step, err := s.authenticate(s.strictAuthAppID, config, user)
	s.Require().NoError(err, "a rejected sync should fail the flow, not the request")
	s.Require().Equal("ERROR", step.FlowStatus, "a rejected sync should fail the sign-in, got %+v", step)
	s.Require().NotNil(step.Error, "the failed sign-in should carry an error")
	s.Equal("FET-1120", step.Error.Code, "expected the federated user sync error")

	s.True(s.groupHasMember(manualGroupID, userID), "a failed sync should not remove groups")
	s.False(s.groupHasMember(mappedGroupID, userID), "a failed sync should not add groups")
	stored, err := testutils.FindUserByAttribute("sub", user.Sub)
	s.Require().NoError(err, "failed to read the returning user")
	s.Require().NotNil(stored, "the returning user should exist")
	var attributes map[string]interface{}
	s.Require().NoError(json.Unmarshal(stored.Attributes, &attributes))
	s.Equal(user.Email, attributes["username"], "a failed sync should not change attributes")
}

// The connections API stores valid sync modes on create and update, and returns them.
func (s *FederatedMappingSuite) TestConnectionSyncConfigurationRoundTrips() {
	fixture := s.idpFixture(&testutils.AttributeConfiguration{
		Sync: &testutils.SyncConfiguration{Attributes: "session_only", GroupsAndRoles: "override_all"},
	})
	fixture.Name = "Federated Sync Round Trip Connection"
	idpID, err := testutils.CreateIDP(fixture)
	s.Require().NoError(err, "valid sync modes should be accepted on create")
	s.T().Cleanup(func() {
		if err := testutils.DeleteIDP(idpID); err != nil {
			s.T().Logf("failed to delete the connection: %v", err)
		}
	})
	s.Equal(&testutils.SyncConfiguration{Attributes: "session_only", GroupsAndRoles: "override_all"},
		s.readSyncConfiguration(idpID))

	fixture.AttributeConfiguration.Sync = &testutils.SyncConfiguration{
		Attributes: "no_sync", GroupsAndRoles: "session_only"}
	s.Require().NoError(testutils.UpdateIDP(idpID, fixture), "valid sync modes should be accepted on update")
	s.Equal(&testutils.SyncConfiguration{Attributes: "no_sync", GroupsAndRoles: "session_only"},
		s.readSyncConfiguration(idpID))

	fixture.AttributeConfiguration.Sync = &testutils.SyncConfiguration{GroupsAndRoles: "override_all"}
	s.Require().NoError(testutils.UpdateIDP(idpID, fixture), "a sync configuration may set one mode only")
	s.Equal(&testutils.SyncConfiguration{GroupsAndRoles: "override_all"}, s.readSyncConfiguration(idpID))
}

// The connections API rejects a sync mode that isn't valid for its setting, on create and update.
func (s *FederatedMappingSuite) TestConnectionSyncConfigurationRejectsInvalidModes() {
	for name, sync := range map[string]*testutils.SyncConfiguration{
		"override all for attributes":            {Attributes: "override_all"},
		"override received for groups and roles": {GroupsAndRoles: "override_received"},
		"unknown attributes mode":                {Attributes: "always"},
		"unknown groups and roles mode":          {GroupsAndRoles: "always"},
	} {
		s.Run(name, func() {
			fixture := s.idpFixture(&testutils.AttributeConfiguration{Sync: sync})
			fixture.Name = "Federated Sync Invalid Connection"
			idpID, err := testutils.CreateIDP(fixture)
			if err == nil {
				if delErr := testutils.DeleteIDP(idpID); delErr != nil {
					s.T().Logf("failed to delete the connection: %v", delErr)
				}
			}
			s.Require().Error(err, "an invalid sync mode should be rejected on create")
			s.Contains(err.Error(), "got 400", "an invalid sync mode should be a client error")
			s.Contains(err.Error(), "IDP-1012", "an invalid sync mode should fail attribute configuration validation")

			err = testutils.UpdateIDP(s.idpID, s.idpFixture(&testutils.AttributeConfiguration{Sync: sync}))
			s.Require().Error(err, "an invalid sync mode should be rejected on update")
			s.Contains(err.Error(), "status 400", "an invalid sync mode should be a client error")
			s.Contains(err.Error(), "IDP-1012", "an invalid sync mode should fail attribute configuration validation")
		})
	}
}

func (s *FederatedMappingSuite) readSyncConfiguration(idpID string) *testutils.SyncConfiguration {
	s.T().Helper()
	stored, err := testutils.GetIDP("oidc", idpID)
	s.Require().NoError(err, "failed to read the connection")
	s.Require().NotNil(stored.AttributeConfiguration, "the connection should carry its attribute configuration")
	return stored.AttributeConfiguration.Sync
}

// With session only attributes, the provider's attributes reach the ID token but only the identifier
// and required attributes are stored.
func (s *FederatedMappingSuite) TestSessionOnlyAttributesReachIDTokenWithoutBeingStored() {
	user := s.baseUser(s.nextSubject())
	config := syncAttributeConfig("session_only")

	tokens := s.obtainFederatedTokens(config, user, "openid profile")

	idClaims, err := testutils.DecodeJWT(tokens.IDToken)
	s.Require().NoError(err, "failed to decode the ID token")
	s.Equal(user.GivenName, idClaims.Additional["firstName"], "the session only attribute should be in the ID token")

	stored, err := testutils.FindUserByAttribute("sub", user.Sub)
	s.Require().NoError(err, "failed to read the provisioned user")
	s.Require().NotNil(stored, "the user should have been provisioned")
	var attributes map[string]interface{}
	s.Require().NoError(json.Unmarshal(stored.Attributes, &attributes))
	s.NotContains(attributes, "firstName", "the session only attribute should not be stored")
}

// With session only groups and roles, a refresh token grant still grants the scopes the mapped role
// and group gave at sign-in, though neither is assigned to the user.
func (s *FederatedMappingSuite) TestSessionOnlyGroupsAndRolesSurviveRefresh() {
	user := s.baseUser(s.nextSubject())
	user.Custom["groups"] = []interface{}{"platform-admins", "platform-editors"}
	config := authzMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"platform-admins":  {roleTarget(s.authzMappedRoleID)},
		"platform-editors": {groupTarget(s.authzMappedGroupID)},
	})
	config.Sync = &testutils.SyncConfiguration{GroupsAndRoles: "session_only"}

	tokens := s.obtainFederatedTokens(config, user, "openid read write")
	signInScopes := s.accessTokenScopes(tokens.AccessToken)
	s.Require().Subset(signInScopes, []string{"read", "write"},
		"the mapped role and group should grant their scopes at sign-in")

	stored, err := testutils.FindUserByAttribute("sub", user.Sub)
	s.Require().NoError(err, "failed to read the provisioned user")
	s.Require().NotNil(stored, "the user should have been provisioned")
	s.False(s.roleHasAssignee(s.authzMappedRoleID, stored.ID), "the session only role should not be assigned")
	s.False(s.groupHasMember(s.authzMappedGroupID, stored.ID), "the session only group should not be assigned")

	refreshed, err := testutils.RefreshAccessToken(syncTokenClientID, syncTokenClientSecret, tokens.RefreshToken)
	s.Require().NoError(err, "the refresh token grant should succeed")
	s.ElementsMatch(signInScopes, s.accessTokenScopes(refreshed.AccessToken),
		"the session only role and group should grant the same scopes after a refresh")
}

// Under no sync, a returning user's mapped role and group aren't stored or kept for the session, so
// they grant no scopes at sign-in, and a refresh doesn't grant them either.
func (s *FederatedMappingSuite) TestNoSyncMappedGroupsAndRolesGrantNothingOnRefresh() {
	user := s.baseUser(s.nextSubject())
	userID := s.createSyncUser(user)
	user.Custom["groups"] = []interface{}{"platform-admins", "platform-editors"}
	config := authzMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"platform-admins":  {roleTarget(s.authzMappedRoleID)},
		"platform-editors": {groupTarget(s.authzMappedGroupID)},
	})
	config.Sync = &testutils.SyncConfiguration{Attributes: "no_sync", GroupsAndRoles: "no_sync"}

	tokens := s.obtainFederatedTokens(config, user, "openid read write")
	signInScopes := s.accessTokenScopes(tokens.AccessToken)
	s.Require().NotContains(signInScopes, "read", "no sync should not grant the mapped role's scope")
	s.Require().NotContains(signInScopes, "write", "no sync should not grant the mapped group's scope")
	s.False(s.roleHasAssignee(s.authzMappedRoleID, userID), "no sync should not assign the mapped role")
	s.False(s.groupHasMember(s.authzMappedGroupID, userID), "no sync should not add the mapped group")

	refreshed, err := testutils.RefreshAccessToken(syncTokenClientID, syncTokenClientSecret, tokens.RefreshToken)
	s.Require().NoError(err, "the refresh token grant should succeed")
	s.ElementsMatch(signInScopes, s.accessTokenScopes(refreshed.AccessToken),
		"a refresh should not grant scopes the sign-in didn't")
}

// obtainFederatedTokens signs the identity in through the authorization code grant, with a flow that
// provisions and checks authorization after the federated login, and returns the tokens.
func (s *FederatedMappingSuite) obtainFederatedTokens(
	config *testutils.AttributeConfiguration, user *testutils.OIDCUserInfo, scope string,
) *testutils.TokenResponse {
	s.T().Helper()
	flow := fedAuthzFlow
	flow.Name = "Federated Sync Token Flow"
	flow.Handle = "auth_flow_federated_sync_tokens"
	s.createScenarioApp(flow, syncTokenClientID, map[string]interface{}{
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
	})
	s.applyConfig(config)
	s.mockOIDC.AddUser(user)
	s.activeSub = user.Sub

	resp, err := testutils.InitiateAuthorizationFlowWithResource(
		syncTokenClientID, syncTokenRedirectURI, "code", scope, "test-state", authzMappingResource)
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

	if provisioned, lookupErr := testutils.FindUserByAttribute("sub", user.Sub); lookupErr == nil && provisioned != nil {
		s.config.CreatedUserIDs = append(s.config.CreatedUserIDs, provisioned.ID)
	}

	authz, err := testutils.CompleteAuthorization(authID, step.Assertion)
	s.Require().NoError(err, "failed to complete the authorization")
	authCode, err := testutils.ExtractAuthorizationCode(authz.RedirectURI)
	s.Require().NoError(err, "failed to read the authorization code")
	result, err := testutils.RequestTokenWithResource(syncTokenClientID, syncTokenClientSecret, authCode,
		syncTokenRedirectURI, "authorization_code", authzMappingResource)
	s.Require().NoError(err, "failed to request tokens")
	s.Require().Equal(http.StatusOK, result.StatusCode, "the token request should succeed: %s", string(result.Body))
	return result.Token
}

func (s *FederatedMappingSuite) accessTokenScopes(accessToken string) []string {
	s.T().Helper()
	claims, err := testutils.DecodeJWT(accessToken)
	s.Require().NoError(err, "failed to decode the access token")
	scope, _ := claims.Additional["scope"].(string)
	return strings.Fields(scope)
}
