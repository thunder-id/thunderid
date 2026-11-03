// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/rolemock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
)

// UserSharingTestSuite covers how a user participates in sharing: what it declares to the framework,
// and how the policies its file declares are parsed.
type UserSharingTestSuite struct {
	suite.Suite
}

func TestUserSharingTestSuite(t *testing.T) {
	suite.Run(t, new(UserSharingTestSuite))
}

// A user declares the roles a share mandates, a reference set no policy names by default, and the framework finds the
// capabilities that check and apply them.
func (s *UserSharingTestSuite) TestRolesFieldIsDeclared() {
	decl := newUserSharingDeclaration(NewUserServiceInterfaceMock(s.T()), nil)

	s.Equal(UserSharingType, decl.ResourceType())
	s.Equal([]sharing.FieldDeclaration{{Key: RolesFieldKey, Kind: sharing.FieldReferenceSet}}, decl.Fields())
	s.Implements((*sharing.MemberValidator)(nil), decl)
	s.Implements((*sharing.PolicyWriteHooks)(nil), decl)
}

// Ownership is read through the runtime context. The framework asks in order to decide access, so an
// access-checked read here would recurse back through the framework.
func (s *UserSharingTestSuite) TestOwnerIsResolvedWithoutAnAccessCheck() {
	userService := NewUserServiceInterfaceMock(s.T())
	userService.On("GetUser", mock.MatchedBy(security.IsRuntimeContext), svcTestUserID1, false).
		Return(&providers.User{ID: svcTestUserID1, OUID: shareOwnerOU}, (*tidcommon.ServiceError)(nil)).Once()

	owner, svcErr := newUserSharingDeclaration(userService, nil).OwningOUID(context.Background(), svcTestUserID1)

	s.Require().Nil(svcErr)
	s.Equal(shareOwnerOU, owner)
}

// A failure to read the user is reported rather than answered as "no owner", which the framework
// would read as a resource it does not recognize.
func (s *UserSharingTestSuite) TestOwnerResolutionCarriesTheServiceError() {
	userService := NewUserServiceInterfaceMock(s.T())
	userService.On("GetUser", mock.Anything, svcTestUserID1, false).
		Return((*providers.User)(nil), &tidcommon.InternalServerError).Once()

	owner, svcErr := newUserSharingDeclaration(userService, nil).OwningOUID(context.Background(), svcTestUserID1)

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.Empty(owner)
}

// The policies have to survive the parse, because a declared user's own sharing has no other way in.
func (s *UserSharingTestSuite) TestDeclaredPoliciesSurviveYAMLParsing() {
	doc := []byte(`
id: ` + svcTestUserID1 + `
type: person
ouId: ` + shareOwnerOU + `
attributes:
  username: shared-user
sharingPolicies:
  - id: shared-user-policy
    otherTreeRoots:
      mode: selected
      units:
        - ouId: partner-root
`)

	var declared userDeclarativeResource
	s.Require().NoError(yaml.Unmarshal(doc, &declared))

	s.Require().Len(declared.SharingPolicies, 1)
	policy := declared.SharingPolicies[0]
	s.Equal("shared-user-policy", policy.ID)
	s.Require().NotNil(policy.OtherTreeRoots)
	s.Require().Len(policy.OtherTreeRoots.Units, 1)
	s.Equal("partner-root", policy.OtherTreeRoots.Units[0].OUID)
	s.Nil(policy.AllUnits)
	s.Nil(policy.ChildUnits)
}

// A policy parsed from a resource file keeps the roles each named unit is mandated, for the framework to check.
func (s *UserSharingTestSuite) TestDeclaredRolesSurviveYAMLParsing() {
	doc := []byte(`
id: ` + svcTestUserID1 + `
type: person
ouId: ` + shareOwnerOU + `
sharingPolicies:
  - id: shared-user-policy
    childUnits:
      mode: selected
      units:
        - ouId: ` + shareChildOU + `
          reach: unit
          overlayRules:
            roles:
              editable: false
              value: [role-a, role-b]
`)

	var declared userDeclarativeResource
	s.Require().NoError(yaml.Unmarshal(doc, &declared))

	s.Require().Len(declared.SharingPolicies, 1)
	units := declared.SharingPolicies[0].ChildUnits.Units
	s.Require().Len(units, 1)
	rule := units[0].OverlayRules[RolesFieldKey]
	s.False(rule.Editable)
	s.Require().NotNil(rule.Value)
	s.Equal([]string{"role-a", "role-b"}, *rule.Value)
}

// mandatesFixture is a declaration wired to role and sharing mocks.
type mandatesFixture struct {
	decl        *userSharingDeclaration
	assignments *rolemock.RoleAssignmentServiceInterfaceMock
	sharing     *sharingmock.SharingServiceInterfaceMock
}

// newMandatesFixture returns a declaration whose role assignment service is injected.
func (s *UserSharingTestSuite) newMandatesFixture() *mandatesFixture {
	f := s.newUnwiredMandatesFixture()
	f.decl.roleAssignments = f.assignments
	return f
}

// newUnwiredMandatesFixture returns a declaration loaded before roles, with no role assignment service yet.
func (s *UserSharingTestSuite) newUnwiredMandatesFixture() *mandatesFixture {
	f := &mandatesFixture{
		assignments: rolemock.NewRoleAssignmentServiceInterfaceMock(s.T()),
		sharing:     sharingmock.NewSharingServiceInterfaceMock(s.T()),
	}
	f.decl = newUserSharingDeclaration(NewUserServiceInterfaceMock(s.T()), f.sharing)
	return f
}

// fixedRoles is a roles rule pinning roleIDs, as requested and as resolved.
func fixedRoles(ouID string, roleIDs ...string) sharing.StoredRule {
	rule := sharing.OverlayRule{Value: &roleIDs}
	return sharing.StoredRule{
		FieldKey: RolesFieldKey, Grant: sharing.GrantRef{Section: sharing.SectionChildUnits, OUID: ouID},
		Requested: rule, Resolved: rule,
	}
}

// policyNaming is a policy over the test user naming each unit, carrying the given rules.
func policyNaming(units []string, rules ...sharing.StoredRule) sharing.Policy {
	p := userPolicy(shareOwnerOU)
	p.ChildUnits = &sharing.ChildUnits{Mode: sharing.ChildUnitsSelected}
	for _, ouID := range units {
		p.ChildUnits.Units = append(p.ChildUnits.Units, sharing.ChildUnit{OUID: ouID, Reach: sharing.ReachUnit})
	}
	p.Rules = rules
	return p
}

var mandatedAssignee = role.RoleAssignment{ID: svcTestUserID1, Type: role.AssigneeTypeUser}

// A created or updated policy mandates, per named unit, the roles its rule pins; a unit with no roles rule gets none.
func (s *UserSharingTestSuite) TestWrittenPolicyAppliesItsMandates() {
	f := s.newMandatesFixture()
	p := policyNaming([]string{shareChildOU, shareOtherOU, "ou-plain"},
		fixedRoles(shareChildOU, "role-a", "role-b"), fixedRoles(shareOtherOU, "role-c"))
	f.assignments.EXPECT().ApplyMandatedAssignments(mock.Anything, sharePolicyID, mandatedAssignee,
		map[string][]string{shareChildOU: {"role-a", "role-b"}, shareOtherOU: {"role-c"}}).Return(nil).Once()

	s.Nil(f.decl.OnPolicyWritten(context.Background(), p))
}

// A policy naming no roles still syncs, so an update that drops every role removes its mandates.
func (s *UserSharingTestSuite) TestPolicyWithoutRolesClearsItsMandates() {
	f := s.newMandatesFixture()
	f.assignments.EXPECT().ApplyMandatedAssignments(mock.Anything, sharePolicyID, mandatedAssignee,
		map[string][]string{}).Return(nil).Once()

	s.Nil(f.decl.OnPolicyWritten(context.Background(), policyNaming([]string{shareChildOU})))
}

// The same policy written twice applies the same mandates, which the role side makes idempotent.
func (s *UserSharingTestSuite) TestRewritingAPolicyAppliesTheSameMandates() {
	f := s.newMandatesFixture()
	p := policyNaming([]string{shareChildOU}, fixedRoles(shareChildOU, "role-a"))
	f.assignments.EXPECT().ApplyMandatedAssignments(mock.Anything, sharePolicyID, mandatedAssignee,
		map[string][]string{shareChildOU: {"role-a"}}).Return(nil).Twice()

	s.Nil(f.decl.OnPolicyWritten(context.Background(), p))
	s.Nil(f.decl.OnPolicyWritten(context.Background(), p))
}

// A deleted policy takes its mandates with it.
func (s *UserSharingTestSuite) TestRemovedPolicyRemovesItsMandates() {
	f := s.newMandatesFixture()
	f.assignments.EXPECT().RemoveMandatedAssignments(mock.Anything, sharePolicyID).Return(nil).Once()

	s.Nil(f.decl.OnPolicyRemoved(context.Background(), policyNaming([]string{shareChildOU})))
}

// The role side's refusal, a role not in the unit or one the caller may not grant, reaches the framework as given.
func (s *UserSharingTestSuite) TestRoleSideRefusalsPassThrough() {
	for _, refusal := range []tidcommon.ServiceError{
		role.ErrorMandatedRoleNotInOU, role.ErrorMandatedRolesNotSupported, sysauthz.ErrorGrantNotPermitted,
	} {
		s.Run(refusal.Code, func() {
			f := s.newMandatesFixture()
			f.assignments.EXPECT().ApplyMandatedAssignments(mock.Anything, sharePolicyID, mandatedAssignee,
				mock.Anything).Return(&refusal).Once()

			svcErr := f.decl.OnPolicyWritten(context.Background(),
				policyNaming([]string{shareChildOU}, fixedRoles(shareChildOU, "role-a")))

			s.Require().NotNil(svcErr)
			s.Equal(refusal.Code, svcErr.Code)
		})
	}
}

// Roles are mandated only on a child unit the policy names, never on a section reaching a whole family.
func (s *UserSharingTestSuite) TestRolesOnlyOnNamedChildUnits() {
	cases := []struct {
		name  string
		grant sharing.GrantRef
	}{
		{"AllUnits", sharing.GrantRef{Section: sharing.SectionAllUnits}},
		{"EveryChildUnit", sharing.GrantRef{Section: sharing.SectionChildUnits}},
		{"AllOtherTreeRoots", sharing.GrantRef{Section: sharing.SectionOtherTreeRoots}},
		{"NamedTreeRoot", sharing.GrantRef{Section: sharing.SectionOtherTreeRoots, OUID: "partner-root"}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			f := s.newMandatesFixture()
			rule := fixedRoles("", "role-a")
			rule.Grant = tc.grant

			svcErr := f.decl.OnPolicyWritten(context.Background(), policyNaming(nil, rule))

			s.Require().NotNil(svcErr)
			s.Equal(ErrorRolesNotOnNamedUnit.Code, svcErr.Code)
		})
	}
}

// A named unit carries roles only when the policy reaches no further than it: under reach subtree the rule would
// also cover every unit beneath, where nothing is mandated.
func (s *UserSharingTestSuite) TestRolesNeedReachUnit() {
	for _, reach := range []sharing.UnitReach{sharing.ReachUnit, sharing.ReachSubtree} {
		s.Run(string(reach), func() {
			f := s.newMandatesFixture()
			p := policyNaming([]string{shareChildOU}, fixedRoles(shareChildOU, "role-a"))
			p.ChildUnits.Units[0].Reach = reach
			if reach == sharing.ReachUnit {
				f.assignments.EXPECT().ApplyMandatedAssignments(mock.Anything, sharePolicyID, mandatedAssignee,
					map[string][]string{shareChildOU: {"role-a"}}).Return(nil).Once()
			}

			svcErr := f.decl.OnPolicyWritten(context.Background(), p)

			if reach == sharing.ReachUnit {
				s.Nil(svcErr)
				return
			}
			s.Require().NotNil(svcErr)
			s.Equal(ErrorRolesNotOnNamedUnit.Code, svcErr.Code)
		})
	}
}

// A declared policy is held to reach unit at load, before roles are injected.
func (s *UserSharingTestSuite) TestDeclaredRolesNeedReachUnit() {
	f := s.newUnwiredMandatesFixture()
	declared := policyNaming([]string{shareChildOU}, fixedRoles(shareChildOU, "role-a"))
	declared.ChildUnits.Units[0].Reach = sharing.ReachSubtree
	declared.Declared = true

	svcErr := f.decl.OnPolicyWritten(context.Background(), declared)

	s.Require().NotNil(svcErr)
	s.Equal(ErrorRolesNotOnNamedUnit.Code, svcErr.Code)
	s.Empty(f.decl.pendingDeclared)
}

// The roles rule lists at least one role and carries no excludedValues. The framework refuses an editable rule and
// allowedValues before the hook runs.
func (s *UserSharingTestSuite) TestRolesRuleMustBeFixed() {
	roles := []string{"role-a"}
	none := []string{}
	cases := []struct {
		name string
		rule sharing.OverlayRule
	}{
		{"NoValue", sharing.OverlayRule{}},
		{"EmptyValue", sharing.OverlayRule{Value: &none}},
		{"ExcludedValues", sharing.OverlayRule{Value: &roles, ExcludedValues: &roles}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			f := s.newMandatesFixture()
			rule := fixedRoles(shareChildOU)
			rule.Requested = tc.rule

			svcErr := f.decl.OnPolicyWritten(context.Background(), policyNaming([]string{shareChildOU}, rule))

			s.Require().NotNil(svcErr)
			s.Equal(ErrorRolesRuleNotFixed.Code, svcErr.Code)
		})
	}
}

// The issuing unit may name only a role it owns or is shared; any other field is not a role and is not checked.
func (s *UserSharingTestSuite) TestMembersAreRolesTheIssuerSees() {
	cases := []struct {
		name     string
		visible  bool
		svcErr   *tidcommon.ServiceError
		expected string
	}{
		{name: "Visible", visible: true},
		{name: "NotVisible", expected: role.ErrorMandatedRoleNotInOU.Code},
		{name: "UnknownRole", svcErr: &role.ErrorRoleNotFound, expected: role.ErrorRoleNotFound.Code},
		{name: "CheckFails", svcErr: &tidcommon.InternalServerError, expected: tidcommon.InternalServerError.Code},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			f := s.newMandatesFixture()
			f.sharing.EXPECT().IsVisible(mock.Anything, role.RoleSharingType, "role-a", shareOwnerOU).
				Return(tc.visible, tc.svcErr).Once()

			svcErr := f.decl.ValidateMembers(context.Background(), svcTestUserID1, RolesFieldKey, shareOwnerOU,
				[]string{"role-a"})

			if tc.expected == "" {
				s.Nil(svcErr)
				return
			}
			s.Require().NotNil(svcErr)
			s.Equal(tc.expected, svcErr.Code)
		})
	}

	f := s.newMandatesFixture()
	s.Nil(f.decl.ValidateMembers(context.Background(), svcTestUserID1, "other", shareOwnerOU, []string{"x"}))
}

// Without a role assignment service a policy that mandates roles fails as an internal error, since the service is
// wired before the server serves, while one that mandates none, and every removal, goes through.
func (s *UserSharingTestSuite) TestWithoutRolesAPolicyWithRolesIsRefused() {
	f := s.newUnwiredMandatesFixture()
	ctx := context.Background()

	s.Nil(f.decl.ValidateMembers(ctx, svcTestUserID1, RolesFieldKey, shareOwnerOU, []string{"role-a"}))
	svcErr := f.decl.OnPolicyWritten(ctx, policyNaming([]string{shareChildOU}, fixedRoles(shareChildOU, "role-a")))
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)

	s.Nil(f.decl.OnPolicyWritten(ctx, policyNaming([]string{shareChildOU})))
	s.Nil(f.decl.OnPolicyRemoved(ctx, policyNaming([]string{shareChildOU})))
}

// A declared policy loads before roles. Its mandates wait for them, and are checked and applied once they are
// injected, under the runtime context; a policy declared after that is applied as it is written.
func (s *UserSharingTestSuite) TestDeclaredMandatesWaitForRoles() {
	f := s.newUnwiredMandatesFixture()
	declared := policyNaming([]string{shareChildOU, shareOtherOU}, fixedRoles(shareChildOU, "role-a", "role-b"),
		fixedRoles(shareOtherOU, "role-a"))
	declared.Declared = true

	s.Nil(f.decl.OnPolicyWritten(context.Background(), declared))

	runtimeCtx := mock.MatchedBy(security.IsRuntimeContext)
	f.sharing.EXPECT().IsVisible(runtimeCtx, role.RoleSharingType, "role-a", shareOwnerOU).Return(true, nil).Once()
	f.sharing.EXPECT().IsVisible(runtimeCtx, role.RoleSharingType, "role-b", shareOwnerOU).Return(true, nil).Once()
	f.assignments.EXPECT().ApplyMandatedAssignments(runtimeCtx, sharePolicyID, mandatedAssignee,
		map[string][]string{shareChildOU: {"role-a", "role-b"}, shareOtherOU: {"role-a"}}).Return(nil).Once()

	s.Require().NoError(f.decl.setRoleAssignments(context.Background(), f.assignments))
	s.Empty(f.decl.pendingDeclared)

	f.assignments.EXPECT().ApplyMandatedAssignments(mock.Anything, sharePolicyID, mandatedAssignee,
		map[string][]string{shareChildOU: {"role-a", "role-b"}, shareOtherOU: {"role-a"}}).Return(nil).Once()
	s.Nil(f.decl.OnPolicyWritten(context.Background(), declared))
}

// A malformed roles rule in a declared policy is refused at load, without waiting for roles.
func (s *UserSharingTestSuite) TestDeclaredRolesRuleIsCheckedAtLoad() {
	f := s.newUnwiredMandatesFixture()
	rule := fixedRoles("", "role-a")
	rule.Grant = sharing.GrantRef{Section: sharing.SectionAllUnits}
	declared := policyNaming(nil, rule)
	declared.Declared = true

	svcErr := f.decl.OnPolicyWritten(context.Background(), declared)

	s.Require().NotNil(svcErr)
	s.Equal(ErrorRolesNotOnNamedUnit.Code, svcErr.Code)
	s.Empty(f.decl.pendingDeclared)
}

// A declared policy naming a role its issuer cannot see, or one the role side refuses, stops startup naming the policy.
func (s *UserSharingTestSuite) TestDeclaredMandatesThatFailStopStartup() {
	cases := []struct {
		name string
		stub func(f *mandatesFixture)
		code string
	}{
		{"IssuerCannotSeeTheRole", func(f *mandatesFixture) {
			f.sharing.EXPECT().IsVisible(mock.Anything, role.RoleSharingType, "role-a", shareOwnerOU).
				Return(false, nil).Once()
		}, role.ErrorMandatedRoleNotInOU.Code},
		{"RoleNotInTheUnit", func(f *mandatesFixture) {
			f.sharing.EXPECT().IsVisible(mock.Anything, role.RoleSharingType, "role-a", shareOwnerOU).
				Return(true, nil).Once()
			f.assignments.EXPECT().ApplyMandatedAssignments(mock.Anything, sharePolicyID, mandatedAssignee,
				mock.Anything).Return(&role.ErrorMandatedRoleNotInOU).Once()
		}, role.ErrorMandatedRoleNotInOU.Code},
		{"UnknownUnit", func(f *mandatesFixture) {
			f.sharing.EXPECT().IsVisible(mock.Anything, role.RoleSharingType, "role-a", shareOwnerOU).
				Return(false, &oupkg.ErrorOrganizationUnitNotFound).Once()
		}, oupkg.ErrorOrganizationUnitNotFound.Code},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			f := s.newUnwiredMandatesFixture()
			declared := policyNaming([]string{shareChildOU}, fixedRoles(shareChildOU, "role-a"))
			declared.Declared = true
			s.Require().Nil(f.decl.OnPolicyWritten(context.Background(), declared))
			tc.stub(f)

			err := f.decl.setRoleAssignments(context.Background(), f.assignments)

			s.Require().Error(err)
			s.Contains(err.Error(), sharePolicyID)
			s.Contains(err.Error(), tc.code)
		})
	}
}
