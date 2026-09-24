// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/*
Hardening tests for two gaps a reviewer found in the AuthorizationRuleMapping/AuthorizationDirectMapping
implementation:

  - An external claim sharing the name of a reserved runtime key (e.g. "mapped_role_ids") must never be
    trusted as if it were server-computed mapping output.
  - A group granted through authorization mapping must inherit permissions from its ancestor groups, the
    same way a real, directly-assigned member of that group already does.
*/
package federated

import "github.com/thunder-id/thunderid/tests/integration/testutils"

// A claim literally named "mapped_role_ids" must not be trusted as mapping output. The connection has no
// authorization mapping configured at all, so before the fix the raw claim would have been copied
// straight into runtime data and read back by the authorization executor as if it were a resolved role.
func (s *FederatedMappingSuite) TestAuthzMapping_ReservedKeyClaimIsNotTrusted() {
	privilegedRoleID, err := testutils.CreateRole(testutils.Role{
		Name: "FedPrivRoleA " + s.nextSubject(),
		OUID: s.ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: s.authzResourceServerID, Permissions: []string{"delete"}},
		},
	})
	s.Require().NoError(err, "failed to create the privileged role the injected claim targets")
	defer func() {
		if err := testutils.DeleteRole(privilegedRoleID); err != nil {
			s.T().Logf("failed to delete the privileged role: %v", err)
		}
	}()

	user := s.baseUser(s.nextSubject())
	user.Custom["mapped_role_ids"] = privilegedRoleID

	config := mapping(fedPersonType.Name, pair("email", "email"), pair("email", "username"))

	authorized := s.authorizeFederated(config, user, "delete", "federated-authz-mapping-api")
	s.NotContains(authorized, "delete",
		"a claim named like the reserved runtime key must not grant the role it names")
}

// The same reserved-key claim must not survive even when authorization mapping IS configured, as long as
// it maps a different claim: mapping resolving to nothing for this subject must not leave the injected
// claim's raw value sitting in the key the mapping writer owns.
func (s *FederatedMappingSuite) TestAuthzMapping_ReservedKeyClaimNotTrustedWhenMappingConfigured() {
	privilegedRoleID, err := testutils.CreateRole(testutils.Role{
		Name: "FedPrivRoleB " + s.nextSubject(),
		OUID: s.ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: s.authzResourceServerID, Permissions: []string{"delete"}},
		},
	})
	s.Require().NoError(err, "failed to create the privileged role the injected claim targets")
	defer func() {
		if err := testutils.DeleteRole(privilegedRoleID); err != nil {
			s.T().Logf("failed to delete the privileged role: %v", err)
		}
	}()

	user := s.baseUser(s.nextSubject())
	user.Custom["mapped_role_ids"] = privilegedRoleID
	user.Custom["groups"] = []interface{}{"unrelated-value"}

	config := authzMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"platform-admins": {roleTarget(s.authzMappedRoleID)},
	})

	authorized := s.authorizeFederated(config, user, "read delete", "federated-authz-mapping-api")
	s.NotContains(authorized, "read", "the unmatched claim value should not authorize the mapped role")
	s.NotContains(authorized, "delete",
		"a claim named like the reserved runtime key must not grant the role it names")
}

// A claim value that resolves, by direct name match, to a CHILD group must also carry whatever that
// group's own ancestor group grants — the same as it would for a stored member of the child group.
func (s *FederatedMappingSuite) TestAuthzMapping_MappedChildGroupGrantsParentGroupRole() {
	childGroupID, err := testutils.CreateGroup(testutils.Group{
		Name: "FedAncChild " + s.nextSubject(),
		OUID: s.ouID,
	})
	s.Require().NoError(err, "failed to create the child group")
	defer func() {
		if err := testutils.DeleteGroup(childGroupID); err != nil {
			s.T().Logf("failed to delete the child group: %v", err)
		}
	}()

	parentGroupID, err := testutils.CreateGroup(testutils.Group{
		Name:    "FedAncParent " + s.nextSubject(),
		OUID:    s.ouID,
		Members: []testutils.Member{{Id: childGroupID, Type: "group"}},
	})
	s.Require().NoError(err, "failed to create the parent group with the child nested inside it")
	defer func() {
		if err := testutils.DeleteGroup(parentGroupID); err != nil {
			s.T().Logf("failed to delete the parent group: %v", err)
		}
	}()

	parentRoleID, err := testutils.CreateRole(testutils.Role{
		Name: "FedAncPRole " + s.nextSubject(),
		OUID: s.ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: s.authzResourceServerID, Permissions: []string{"delete"}},
		},
		Assignments: []testutils.Assignment{{ID: parentGroupID, Type: "group"}},
	})
	s.Require().NoError(err, "failed to create the role assigned to the parent group")
	defer func() {
		if err := testutils.DeleteRole(parentRoleID); err != nil {
			s.T().Logf("failed to delete the parent group's role: %v", err)
		}
	}()

	user := s.baseUser(s.nextSubject())
	user.Custom["groups"] = []interface{}{"engineering"}

	config := authzMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"engineering": {groupTarget(childGroupID)},
	})

	authorized := s.authorizeFederated(config, user, "delete", "federated-authz-mapping-api")
	s.Contains(authorized, "delete",
		"a role granted to the mapped group's ancestor should be authorized, same as a real member")
}
