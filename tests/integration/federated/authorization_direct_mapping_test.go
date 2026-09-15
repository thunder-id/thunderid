// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/*
AuthorizationDirectMapping integration tests.

These exercise the connection's authorizationMapping.direct section end to end: a mock IdP returns
claims, a federated authentication flow provisions (or resolves) a local entity, an AuthorizationExecutor
node evaluates the requested permissions, and the resulting AuthAssertExecutor assertion's
authorized_permissions claim is read back to observe what the mapping produced. This is the direct-mapping
counterpart to authorization_mapping_test.go's rule-based scenarios, reusing the same suite fixtures
(s.authzResourceServerID, s.authzOtherResourceServerID, s.authzMappedRoleID "Federated Mapped Reader",
s.authzMappedGroupID "Federated Mapped Editors") but resolving by name instead of by an explicit rule
table.
*/
package federated

import (
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// authzDirectMapping builds a single-entry AuthorizationDirectMapping configuration, alongside the
// attribute mappings fedPersonType needs to provision, mirroring authzMapping's shape for the
// rule-based mode.
func authzDirectMapping(claim, delimiter, targetType, resourceServerID string) *testutils.AttributeConfiguration {
	config := mapping(fedPersonType.Name, pair("email", "email"), pair("email", "username"))
	config.AuthorizationMapping = &testutils.AuthorizationMapping{
		Direct: []testutils.AuthorizationDirectMapping{
			{Claim: claim, Delimiter: delimiter, TargetType: targetType, ResourceServerID: resourceServerID},
		},
	}
	return config
}

// A claim value matching a role's name exactly grants that role's permission.
func (s *FederatedMappingSuite) TestAuthzDirectMapping_RoleNameMatchGrants() {
	user := s.baseUser(s.nextSubject())
	user.Custom["role_name"] = "Federated Mapped Reader"

	config := authzDirectMapping("role_name", "", testutils.AuthorizationTargetRole, "")

	authorized := s.authorizeFederated(config, user, "read write", "federated-authz-mapping-api")
	s.Contains(authorized, "read", "the claim value should resolve the role by exact name")
	s.NotContains(authorized, "write", "only the matched role's own permission should be authorized")
}

// A claim value naming no existing role confers nothing, rather than erroring.
func (s *FederatedMappingSuite) TestAuthzDirectMapping_RoleNameNoMatchGrantsNothing() {
	user := s.baseUser(s.nextSubject())
	user.Custom["role_name"] = "Nonexistent Role"

	config := authzDirectMapping("role_name", "", testutils.AuthorizationTargetRole, "")

	authorized := s.authorizeFederated(config, user, "read", "federated-authz-mapping-api")
	s.Empty(authorized, "an unmatched role name should confer nothing")
}

// A role name is unique only within an organization unit, so the same name in a second OU makes the
// claim value ambiguous: it must confer nothing rather than picking one of the two roles arbitrarily.
func (s *FederatedMappingSuite) TestAuthzDirectMapping_AmbiguousRoleNameAcrossOUsSkipped() {
	otherOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "direct-mapping-ambiguous-" + s.nextSubject(),
		Name:   "Direct Mapping Ambiguous OU",
	})
	s.Require().NoError(err, "failed to create the second organization unit")
	defer func() {
		if err := testutils.DeleteOrganizationUnit(otherOUID); err != nil {
			s.T().Logf("failed to delete the second organization unit: %v", err)
		}
	}()

	dupRoleID, err := testutils.CreateRole(testutils.Role{Name: "Federated Mapped Reader", OUID: otherOUID})
	s.Require().NoError(err, "failed to create the duplicate-named role")
	defer func() {
		// A failed delete would leave this ambiguous name behind and fail a later exact-name test
		// instead of this one, so cleanup failure must fail this test, not just log it.
		s.Require().NoError(testutils.DeleteRole(dupRoleID), "failed to delete the duplicate-named role")
	}()

	user := s.baseUser(s.nextSubject())
	user.Custom["role_name"] = "Federated Mapped Reader"

	config := authzDirectMapping("role_name", "", testutils.AuthorizationTargetRole, "")

	authorized := s.authorizeFederated(config, user, "read", "federated-authz-mapping-api")
	s.Empty(authorized, "a name matching more than one role should confer nothing, not pick one arbitrarily")
}

// A claim value matching a group's name exactly grants access through that group's own role
// assignment, the same way an explicit group target would.
func (s *FederatedMappingSuite) TestAuthzDirectMapping_GroupNameMatchGrants() {
	user := s.baseUser(s.nextSubject())
	user.Custom["group_name"] = "Federated Mapped Editors"

	config := authzDirectMapping("group_name", "", testutils.AuthorizationTargetGroup, "")

	authorized := s.authorizeFederated(config, user, "read write", "federated-authz-mapping-api")
	s.Contains(authorized, "write", "the claim value should resolve the group by exact name")
	s.NotContains(authorized, "read", "only the matched group's own role permission should be authorized")
}

// A group name matching more than one group across organization units is ambiguous, mirroring the
// role case above.
func (s *FederatedMappingSuite) TestAuthzDirectMapping_AmbiguousGroupNameAcrossOUsSkipped() {
	otherOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "direct-mapping-ambiguous-group-" + s.nextSubject(),
		Name:   "Direct Mapping Ambiguous Group OU",
	})
	s.Require().NoError(err, "failed to create the second organization unit")
	defer func() {
		if err := testutils.DeleteOrganizationUnit(otherOUID); err != nil {
			s.T().Logf("failed to delete the second organization unit: %v", err)
		}
	}()

	dupGroupID, err := testutils.CreateGroup(testutils.Group{Name: "Federated Mapped Editors", OUID: otherOUID})
	s.Require().NoError(err, "failed to create the duplicate-named group")
	defer func() {
		// A failed delete would leave this ambiguous name behind and fail a later exact-name test
		// instead of this one, so cleanup failure must fail this test, not just log it.
		s.Require().NoError(testutils.DeleteGroup(dupGroupID), "failed to delete the duplicate-named group")
	}()

	user := s.baseUser(s.nextSubject())
	user.Custom["group_name"] = "Federated Mapped Editors"

	config := authzDirectMapping("group_name", "", testutils.AuthorizationTargetGroup, "")

	authorized := s.authorizeFederated(config, user, "write", "federated-authz-mapping-api")
	s.Empty(authorized, "a name matching more than one group should confer nothing, not pick one arbitrarily")
}

// A list-valued claim (a genuine JSON array on the signed ID token, not a delimited string) resolves
// every element independently, with no delimiter configured at all.
func (s *FederatedMappingSuite) TestAuthzDirectMapping_ListValuedClaimResolvesEachElement() {
	user := s.baseUser(s.nextSubject())
	user.Custom["names"] = []interface{}{"Federated Mapped Reader", "Federated Mapped Editors"}

	config := mapping(fedPersonType.Name, pair("email", "email"), pair("email", "username"))
	config.AuthorizationMapping = &testutils.AuthorizationMapping{
		Direct: []testutils.AuthorizationDirectMapping{
			{Claim: "names", TargetType: testutils.AuthorizationTargetRole},
			{Claim: "names", TargetType: testutils.AuthorizationTargetGroup},
		},
	}

	authorized := s.authorizeFederated(config, user, "read write", "federated-authz-mapping-api")
	s.ElementsMatch([]string{"read", "write"}, authorized,
		"every element of the list claim should resolve independently, one against each target type")
}

// A single string claim splits into multiple tokens via Delimiter, resolving the same way a native
// list claim does.
func (s *FederatedMappingSuite) TestAuthzDirectMapping_DelimitedStringClaimSplitsIntoTokens() {
	user := s.baseUser(s.nextSubject())
	user.Custom["names"] = "Federated Mapped Reader,Federated Mapped Editors"

	config := mapping(fedPersonType.Name, pair("email", "email"), pair("email", "username"))
	config.AuthorizationMapping = &testutils.AuthorizationMapping{
		Direct: []testutils.AuthorizationDirectMapping{
			{Claim: "names", Delimiter: ",", TargetType: testutils.AuthorizationTargetRole},
			{Claim: "names", Delimiter: ",", TargetType: testutils.AuthorizationTargetGroup},
		},
	}

	authorized := s.authorizeFederated(config, user, "read write", "federated-authz-mapping-api")
	s.ElementsMatch([]string{"read", "write"}, authorized,
		"a delimited string claim should split into tokens the same way a native list claim does")
}

// A claim that isn't a JSON string at all (a boolean here) still resolves: the claim value is
// stringified into a single token, exactly as it is for rule-based mapping, with no valueType needed
// since direct mapping has no comparison operator whose semantics would depend on one.
func (s *FederatedMappingSuite) TestAuthzDirectMapping_NonStringClaimValueIsStringified() {
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "true",
		OUID: s.ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: s.authzResourceServerID, Permissions: []string{"read"}},
		},
	})
	s.Require().NoError(err, "failed to create the boolean-named role")
	defer func() {
		if err := testutils.DeleteRole(roleID); err != nil {
			s.T().Logf("failed to delete the boolean-named role: %v", err)
		}
	}()

	user := s.baseUser(s.nextSubject())
	user.Custom["is_admin"] = true

	config := authzDirectMapping("is_admin", "", testutils.AuthorizationTargetRole, "")

	authorized := s.authorizeFederated(config, user, "read", "federated-authz-mapping-api")
	s.Contains(authorized, "read", `a boolean claim value should stringify to "true" and resolve a role literally named that`)
}

// A permission target grants only the claim values ValidatePermissions confirms exist on the
// configured resource server; anything else is dropped rather than erroring the whole request.
func (s *FederatedMappingSuite) TestAuthzDirectMapping_PermissionTargetGrantsOnlyValidatedPermissions() {
	user := s.baseUser(s.nextSubject())
	user.Custom["perms"] = []interface{}{"read", "write", "not-a-real-permission"}

	config := authzDirectMapping("perms", "", testutils.AuthorizationTargetPermission, s.authzResourceServerID)

	authorized := s.authorizeFederated(config, user, "read write delete", "federated-authz-mapping-api")
	s.ElementsMatch([]string{"read", "write"}, authorized,
		"only claim values that are valid permissions on the configured resource server should be granted")
}

// A permission target stays scoped to the one resource server it configures: a claim value that is a
// valid permission on a different resource server must not leak into an evaluation against this one.
func (s *FederatedMappingSuite) TestAuthzDirectMapping_PermissionTargetScopedToItsResourceServer() {
	user := s.baseUser(s.nextSubject())
	user.Custom["perms"] = []interface{}{"read"}

	config := authzDirectMapping("perms", "", testutils.AuthorizationTargetPermission, s.authzOtherResourceServerID)

	authorized := s.authorizeFederated(config, user, "read", "federated-authz-mapping-api")
	s.Empty(authorized, "a permission target scoped to a different resource server must not grant access here")
}

// A direct mapping and a rule-based mapping configured on the same connection combine their results,
// the same additive/union behavior the connection already has for local group membership plus a
// mapping (see TestAuthzMapping_LocalGroupPlusMappedGroupCombine).
func (s *FederatedMappingSuite) TestAuthzDirectMapping_CombinesWithRuleBasedMapping() {
	user := s.baseUser(s.nextSubject())
	user.Custom["group_name"] = "Federated Mapped Editors"
	user.Custom["groups"] = []interface{}{"platform-admins"}

	config := mapping(fedPersonType.Name, pair("email", "email"), pair("email", "username"))
	config.AuthorizationMapping = &testutils.AuthorizationMapping{
		Direct: []testutils.AuthorizationDirectMapping{
			{Claim: "group_name", TargetType: testutils.AuthorizationTargetGroup},
		},
		Rules: []testutils.AuthorizationRuleMapping{
			{Claim: "groups", Values: equalsRules(map[string][]testutils.AuthorizationTarget{
				"platform-admins": {roleTarget(s.authzMappedRoleID)},
			})},
		},
	}

	authorized := s.authorizeFederated(config, user, "read write", "federated-authz-mapping-api")
	s.ElementsMatch([]string{"read", "write"}, authorized,
		"a direct mapping and a rule-based mapping configured together should combine their results")
}

// When a direct mapping and a rule-based mapping both resolve to the same role, combining their
// results (a plain append, not a set union, once the two mechanisms' own internal dedup has run) must
// not double-grant, error, or otherwise behave differently than a single mapping to that role would.
func (s *FederatedMappingSuite) TestAuthzDirectMapping_SameRoleFromBothMechanismsGrantsOnce() {
	user := s.baseUser(s.nextSubject())
	user.Custom["role_name"] = "Federated Mapped Reader"
	user.Custom["groups"] = []interface{}{"platform-admins"}

	config := mapping(fedPersonType.Name, pair("email", "email"), pair("email", "username"))
	config.AuthorizationMapping = &testutils.AuthorizationMapping{
		Direct: []testutils.AuthorizationDirectMapping{
			{Claim: "role_name", TargetType: testutils.AuthorizationTargetRole},
		},
		Rules: []testutils.AuthorizationRuleMapping{
			{Claim: "groups", Values: equalsRules(map[string][]testutils.AuthorizationTarget{
				"platform-admins": {roleTarget(s.authzMappedRoleID)},
			})},
		},
	}

	authorized := s.authorizeFederated(config, user, "read write", "federated-authz-mapping-api")
	s.ElementsMatch([]string{"read"}, authorized,
		"the same role resolved by both mechanisms should grant its permission once, and not leak write")
}

// The same collision, for a permission target instead of a role: both mechanisms resolving to the
// same permission on the same resource server must still grant it exactly once.
func (s *FederatedMappingSuite) TestAuthzDirectMapping_SamePermissionFromBothMechanismsGrantsOnce() {
	user := s.baseUser(s.nextSubject())
	user.Custom["perms"] = []interface{}{"read"}
	user.Custom["groups"] = []interface{}{"platform-admins"}

	config := mapping(fedPersonType.Name, pair("email", "email"), pair("email", "username"))
	config.AuthorizationMapping = &testutils.AuthorizationMapping{
		Direct: []testutils.AuthorizationDirectMapping{
			{Claim: "perms", TargetType: testutils.AuthorizationTargetPermission, ResourceServerID: s.authzResourceServerID},
		},
		Rules: []testutils.AuthorizationRuleMapping{
			{Claim: "groups", Values: equalsRules(map[string][]testutils.AuthorizationTarget{
				"platform-admins": {permissionTarget(s.authzResourceServerID, "read")},
			})},
		},
	}

	authorized := s.authorizeFederated(config, user, "read write", "federated-authz-mapping-api")
	s.ElementsMatch([]string{"read"}, authorized,
		"the same permission resolved by both mechanisms should grant it once, and not leak write")
}
