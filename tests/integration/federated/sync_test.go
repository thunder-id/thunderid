// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federated

import (
	"encoding/json"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

/*
Syncing a returning federated user.

A returning identity resolves through its recorded link, so every scenario links its user the way an
End-User would, through a verified linking flow, with sync off so the link itself writes nothing. The
sign-in under test then runs with the sync modes it is about.
*/

// federatedUserSyncFailedCode is the error a sign-in fails with when the sync can't be written.
const federatedUserSyncFailedCode = "FET-1120"

// linkedSyncUser creates a fed_person holding attributes, and records a link to it for a new identity.
// It returns the identity and the user's id.
func (s *FederatedMappingSuite) linkedSyncUser(
	attributes map[string]interface{}) (*testutils.OIDCUserInfo, string) {
	s.T().Helper()
	email := s.nextSubject() + "@example.com"
	stored := map[string]interface{}{"username": email, "email": email, "password": linkPassword}
	for name, value := range attributes {
		stored[name] = value
	}
	userID := s.createLocalUser(stored)

	user := s.baseUser(s.nextSubject())
	user.Email = email
	config := linkOn([]string{"email"}, pair("email", "email"))
	config.Sync = &testutils.SyncConfiguration{Attributes: "no_sync"}
	s.applyConfig(config)
	s.mockOIDC.AddUser(user)
	s.recordLink("OIDCAuthExecutor", s.idpID, user.Sub, email)
	return user, userID
}

// signInAndReadAttributes signs the linked identity in through the strict flow, which resolves it
// through its link alone, and returns the user's stored attributes.
func (s *FederatedMappingSuite) signInAndReadAttributes(
	config *testutils.AttributeConfiguration, user *testutils.OIDCUserInfo, userID string,
) map[string]interface{} {
	s.T().Helper()
	step, err := s.authenticate(s.strictAuthAppID, config, user)
	s.Require().NoError(err, "the linked identity should sign in")
	s.Require().Equal("COMPLETE", step.FlowStatus, "the linked identity should sign in, got %+v", step)
	claims, err := testutils.DecodeJWT(step.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	s.Require().Equal(userID, claims.Sub, "the identity should resolve to the linked user")
	return s.storedAttributes(user.Email)
}

// storedAttributes returns the attributes of the user holding email.
func (s *FederatedMappingSuite) storedAttributes(email string) map[string]interface{} {
	s.T().Helper()
	stored, err := testutils.FindUserByAttribute("email", email)
	s.Require().NoError(err, "failed to look up the user holding %s", email)
	s.Require().NotNil(stored, "a user should hold %s", email)
	var attributes map[string]interface{}
	s.Require().NoError(json.Unmarshal(stored.Attributes, &attributes))
	return attributes
}

// syncAttributeConfig maps given_name onto firstName, and nickname, which the mock never sends, onto
// lastName.
func syncAttributeConfig(mode string) *testutils.AttributeConfiguration {
	config := mapping(fedPersonType.Handle,
		pair("email", "email"), pair("given_name", "firstName"), pair("nickname", "lastName"))
	config.Sync = &testutils.SyncConfiguration{Attributes: mode}
	return config
}

// With override received, the mapped attributes the provider sends replace the stored ones. A mapped
// attribute it doesn't send is kept, and a claim no mapping names is not written, even when it shares
// a local attribute's name, the subject included.
func (s *FederatedMappingSuite) TestOverrideReceivedWritesOnlyReceivedMappedAttributes() {
	user, userID := s.linkedSyncUser(map[string]interface{}{
		"firstName": "Old", "lastName": "Stored", "city": "Colombo"})
	user.Custom["city"] = "Kandy"

	attributes := s.signInAndReadAttributes(syncAttributeConfig("override_received"), user, userID)

	s.Equal(user.GivenName, attributes["firstName"], "a received mapped attribute should be written")
	s.Equal("Stored", attributes["lastName"], "a mapped attribute the provider didn't send should be kept")
	s.Equal("Colombo", attributes["city"], "an unmapped claim should not be written")
	s.NotContains(attributes, "sub", "the subject should not be written")
}

// With no sync, a returning sign-in leaves the stored attributes alone.
func (s *FederatedMappingSuite) TestNoSyncAttributesLeavesStoredAttributes() {
	user, userID := s.linkedSyncUser(map[string]interface{}{"firstName": "Old"})

	attributes := s.signInAndReadAttributes(syncAttributeConfig("no_sync"), user, userID)

	s.Equal("Old", attributes["firstName"])
}

// With override all groups and roles, the user's direct groups and roles match what the mapping
// resolves: mapped ones are added, and ones assigned in ThunderID that the provider doesn't send are
// removed.
func (s *FederatedMappingSuite) TestOverrideAllGroupsAndRolesMatchesMapping() {
	user, userID := s.linkedSyncUser(nil)
	mappedGroupID := s.createSyncGroup("Federated Sync Mapped Group", s.ouID, nil)
	manualGroupID := s.createSyncGroup("Federated Sync Manual Group", s.ouID,
		[]testutils.Member{{Id: userID, Type: "user"}})
	mappedRoleID := s.createSyncRole("Federated Sync Mapped Role", nil)
	manualRoleID := s.createSyncRole("Federated Sync Manual Role", []testutils.Assignment{{ID: userID, Type: "user"}})

	user.Custom["groups"] = []interface{}{"engineering"}
	config := authzMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"engineering": {groupTarget(mappedGroupID), roleTarget(mappedRoleID)},
	})
	config.Sync = &testutils.SyncConfiguration{Attributes: "no_sync", GroupsAndRoles: "override_all"}

	s.signInAndReadAttributes(config, user, userID)

	s.True(s.groupHasMember(mappedGroupID, userID), "the mapped group should be added")
	s.False(s.groupHasMember(manualGroupID, userID), "a group the provider doesn't send should be removed")
	s.True(s.roleHasAssignee(mappedRoleID, userID), "the mapped role should be assigned")
	s.False(s.roleHasAssignee(manualRoleID, userID), "a role the provider doesn't send should be unassigned")
}

// Under override all, a direct group name that matches groups in two organization units grants
// neither, and keeps the membership the user already has in one of them. The mapping targets no
// roles, so the user's roles are left alone.
func (s *FederatedMappingSuite) TestOverrideAllKeepsAmbiguousGroupAndUntargetedRoles() {
	user, userID := s.linkedSyncUser(nil)
	otherOU := s.createSyncOU()

	name := "Federated Sync Ambiguous " + s.nextSubject()
	memberGroupID := s.createSyncGroup(name, s.ouID, []testutils.Member{{Id: userID, Type: "user"}})
	otherGroupID := s.createSyncGroup(name, otherOU.ID, nil)
	manualGroupID := s.createSyncGroup("Federated Sync Ambiguous Manual Group", s.ouID,
		[]testutils.Member{{Id: userID, Type: "user"}})
	manualRoleID := s.createSyncRole("Federated Sync Untargeted Role",
		[]testutils.Assignment{{ID: userID, Type: "user"}})

	user.Custom["group_name"] = name
	config := authzDirectMapping("group_name", "", testutils.AuthorizationTargetGroup, "")
	config.Sync = &testutils.SyncConfiguration{Attributes: "no_sync", GroupsAndRoles: "override_all"}

	s.signInAndReadAttributes(config, user, userID)

	s.True(s.groupHasMember(memberGroupID, userID), "the membership in the ambiguous group should be kept")
	s.False(s.groupHasMember(otherGroupID, userID), "the other ambiguous group should not be added")
	s.False(s.groupHasMember(manualGroupID, userID), "a group the provider doesn't send should be removed")
	s.True(s.roleHasAssignee(manualRoleID, userID), "a role type the mapping never targets should be kept")
}

// The sign-in that verifies an account and records the link also syncs the account.
func (s *FederatedMappingSuite) TestLinkingSyncsTheVerifiedAccount() {
	email := s.nextSubject() + "@example.com"
	userID := s.createLocalUser(map[string]interface{}{
		"username": email, "email": email, "password": linkPassword, "firstName": "Old", "city": "Colombo",
	})
	user := s.baseUser(s.nextSubject())
	user.Email = email
	config := linkOn([]string{"email"}, pair("email", "email"), pair("given_name", "firstName"))
	config.Sync = &testutils.SyncConfiguration{Attributes: "override_received"}
	appID := s.passwordLinkingApp("federated-sync-link")

	final := s.linkByPassword(appID, config, user, email)
	s.Require().Equal("COMPLETE", final.FlowStatus, "the verified account should be linked, got %+v", final)
	claims, err := testutils.DecodeJWT(final.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	s.Equal(userID, claims.Sub, "the assertion should be issued for the verified account")

	attributes := s.storedAttributes(email)
	s.Equal(user.GivenName, attributes["firstName"], "linking should sync the received mapped attribute")
	s.Equal("Colombo", attributes["city"], "an attribute the provider doesn't send should be kept")
}

// A sync that can't be written fails the sign-in that links the account, here because the synced
// username is already another user's. The link is written before the sync and kept, so the next
// sign-in resolves through it with no verification, and syncs again.
func (s *FederatedMappingSuite) TestFailedSyncKeepsTheLink() {
	email := s.nextSubject() + "@example.com"
	userID := s.createLocalUser(map[string]interface{}{"username": email, "email": email, "password": linkPassword})
	takenUsername := s.nextSubject()
	s.createLocalUser(map[string]interface{}{"username": takenUsername, "email": takenUsername + "@example.com"})

	user := s.baseUser(s.nextSubject())
	user.Email = email
	user.GivenName = takenUsername
	config := linkOn([]string{"email"}, pair("email", "email"), pair("given_name", "username"))
	config.Sync = &testutils.SyncConfiguration{Attributes: "override_received"}
	appID := s.passwordLinkingApp("federated-sync-failed")

	failed := s.linkByPassword(appID, config, user, email)
	s.Require().Equal("ERROR", failed.FlowStatus, "a rejected sync should fail the sign-in, got %+v", failed)
	s.Require().NotNil(failed.Error, "the failed sign-in should carry an error")
	s.Equal(federatedUserSyncFailedCode, failed.Error.Code, "expected the federated user sync error")
	s.Empty(failed.Assertion, "a failed sync must not issue an assertion")
	s.Equal(email, s.storedAttributes(email)["username"], "a failed sync should not change attributes")

	user.GivenName = s.nextSubject()
	repeat, err := s.authenticate(appID, config, user)
	s.Require().NoError(err, "the linked identity should sign in again")
	s.Require().Equal("COMPLETE", repeat.FlowStatus,
		"the kept link should resolve the account with no verification, got %+v", repeat)
	claims, err := testutils.DecodeJWT(repeat.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	s.Equal(userID, claims.Sub, "the kept link should resolve the account it was written for")
	s.Equal(user.GivenName, s.storedAttributes(email)["username"], "the next sign-in should sync again")
}

// Two users in different organization units hold the identity's link. The End-User picks one at the
// resolve node, and only that one is synced.
func (s *FederatedMappingSuite) TestOnlyThePickedLinkHolderIsSynced() {
	otherOU := s.createSyncOU()
	costCenter := "CC-" + s.nextSubject()
	usernames := [2]string{s.nextSubject(), s.nextSubject()}
	var ids [2]string
	for i, ouID := range []string{s.ouID, otherOU.ID} {
		userID, err := testutils.CreateUser(testutils.User{
			Type: fedPersonType.Handle,
			OUID: ouID,
			Attributes: mustJSON(map[string]interface{}{
				"username": usernames[i], "email": usernames[i] + "@example.com", "costCenter": costCenter,
				"password": linkPassword, "firstName": "Old",
			}),
		})
		s.Require().NoError(err, "failed to create link holder %d", i+1)
		s.config.CreatedUserIDs = append(s.config.CreatedUserIDs, userID)
		ids[i] = userID
	}
	user := s.baseUser(s.nextSubject())
	user.Custom["cost_centre"] = costCenter
	config := linkOn([]string{"costCenter"}, pair("cost_centre", "costCenter"), pair("given_name", "firstName"))
	config.Sync = &testutils.SyncConfiguration{Attributes: "no_sync"}
	verifyFlowID := s.createVerifyFlow(common.VerifyPasswordFlow("verify_flow_federated-sync-holders"))
	appID := s.createScenarioApp(fedVerifiedLinkingFlow(s.idpID, verifyFlowID), "federated-sync-holders")

	// A sign-in pauses on the linking prompt while another links the first user, then links the second
	// beside it.
	paused, err := s.authenticate(appID, config, user)
	s.Require().NoError(err, "the shared value should reach the linking prompt")
	s.Require().Equal("INCOMPLETE", paused.FlowStatus, "expected the linking prompt, got %+v", paused)
	s.recordLink("OIDCAuthExecutor", s.idpID, user.Sub, usernames[0])
	linked := s.verifyLink(paused, usernames[1])
	s.Require().Equal("COMPLETE", linked.FlowStatus, "the second holder should be linked, got %+v", linked)

	config.Sync = &testutils.SyncConfiguration{Attributes: "override_received"}
	s.applyConfig(config)
	resolvingAppID := s.createScenarioApp(fedLinkResolvingFlow(s.idpID, verifyFlowID), "federated-sync-resolving")
	pick, err := s.authenticateFlow(resolvingAppID, user.Sub)
	s.Require().NoError(err, "the identity linked to two users should reach the picker")
	s.Require().Equal("INCOMPLETE", pick.FlowStatus, "two link holders should be picked between, got %+v", pick)
	s.Require().True(common.HasInput(pick.Data.Inputs, "ouHandle"),
		"the holders differ on organization unit, so it should be offered, got %+v", pick.Data.Inputs)

	picked, err := common.CompleteFlow(pick.ExecutionID, map[string]string{"ouHandle": otherOU.Handle},
		"action_pick", pick.ChallengeToken)
	s.Require().NoError(err, "picking a holder should complete the flow")
	s.Require().Equal("COMPLETE", picked.FlowStatus, "the picked holder should sign in, got %+v", picked)
	claims, err := testutils.DecodeJWT(picked.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	s.Equal(ids[1], claims.Sub, "the assertion should be issued for the picked holder")

	pickedEmail, otherEmail := usernames[1]+"@example.com", usernames[0]+"@example.com"
	s.Equal(user.GivenName, s.storedAttributes(pickedEmail)["firstName"], "the picked holder should be synced")
	s.Equal("Old", s.storedAttributes(otherEmail)["firstName"], "the holder not picked should not be synced")
}

// linkByPassword signs the identity in on a verified linking application, confirms the match and
// proves the account with username and linkPassword, and returns the step that follows.
func (s *FederatedMappingSuite) linkByPassword(appID string, config *testutils.AttributeConfiguration,
	user *testutils.OIDCUserInfo, username string) *common.FlowStep {
	s.T().Helper()
	step, err := s.authenticate(appID, config, user)
	s.Require().NoError(err, "the matched identity should reach the linking prompt")
	s.Require().Equal("INCOMPLETE", step.FlowStatus, "expected the linking prompt, got %+v", step)
	return s.verifyLink(step, username)
}

// verifyLink confirms the match on a linking prompt and proves the account with username and
// linkPassword.
func (s *FederatedMappingSuite) verifyLink(prompt *common.FlowStep, username string) *common.FlowStep {
	s.T().Helper()
	verification, err := common.CompleteFlow(prompt.ExecutionID, nil, common.LinkAccountAction,
		prompt.ChallengeToken)
	s.Require().NoError(err, "confirming the match should route to the verification step")
	final, err := common.CompleteFlow(verification.ExecutionID,
		map[string]string{"username": username, "password": linkPassword}, common.VerifyPasswordAction,
		verification.ChallengeToken)
	s.Require().NoError(err, "a verified link is still a 200 carrying its state, got %v", err)
	return final
}

// createSyncOU creates an organization unit under the suite's, which fed_person users may be created in.
func (s *FederatedMappingSuite) createSyncOU() testutils.OrganizationUnit {
	s.T().Helper()
	ou := testutils.OrganizationUnit{
		Handle: "federated-sync-" + s.nextSubject(),
		Name:   "Federated Sync OU",
		Parent: &s.ouID,
	}
	ouID, err := testutils.CreateOrganizationUnit(ou)
	s.Require().NoError(err, "failed to create the organization unit")
	s.T().Cleanup(func() {
		if err := testutils.DeleteOrganizationUnit(ouID); err != nil {
			s.T().Logf("failed to delete the organization unit: %v", err)
		}
	})
	ou.ID = ouID
	return ou
}

func (s *FederatedMappingSuite) createSyncGroup(name, ouID string, members []testutils.Member) string {
	s.T().Helper()
	groupID, err := testutils.CreateGroup(testutils.Group{Name: name, OUID: ouID, Members: members})
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
