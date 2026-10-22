// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federated

import (
	"encoding/json"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
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
