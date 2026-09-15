// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/revocation"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/groupmock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
)

const (
	providerRoleID  = "role-1"
	providerUserID  = "user-1"
	providerGroupID = "group-1"
	californiaRSID  = "rs-ca"
	ohioRSID        = "rs-oh"
	californiaAud   = "https://api.dmv.ca.gov"
	ohioAud         = "https://api.dmv.oh.gov"
)

type AdminProviderTestSuite struct {
	suite.Suite
	roles       *RoleServiceInterfaceMock
	assignments *RoleAssignmentServiceInterfaceMock
	groups      *groupmock.GroupServiceInterfaceMock
	entities    *entitymock.EntityServiceInterfaceMock
	resources   *resourcemock.ResourceServiceInterfaceMock
	provider    AdminProviderInterface
}

func TestAdminProviderTestSuite(t *testing.T) {
	suite.Run(t, new(AdminProviderTestSuite))
}

func (s *AdminProviderTestSuite) SetupTest() {
	s.roles = NewRoleServiceInterfaceMock(s.T())
	s.assignments = NewRoleAssignmentServiceInterfaceMock(s.T())
	s.groups = groupmock.NewGroupServiceInterfaceMock(s.T())
	s.entities = entitymock.NewEntityServiceInterfaceMock(s.T())
	s.resources = resourcemock.NewResourceServiceInterfaceMock(s.T())
	s.provider = newAdminProvider(s.roles, s.assignments, s.groups, s.entities, s.resources)
}

// assigned makes the role report that the assignee holds it directly. Validation refuses an assignee
// that does not, because the removal itself is a silent no-op in that case.
func (s *AdminProviderTestSuite) assigned(assigneeID string, assigneeType AssigneeType) {
	s.assignments.On("GetRoleAssignments", mock.Anything, providerRoleID,
		assignmentPageSize, 0, false).
		Return(&AssignmentList{TotalResults: 1, Assignments: []RoleAssignmentWithDisplay{
			{ID: assigneeID, Type: assigneeType},
		}}, nil)
}

// notAGroup makes the group lookup report that the id belongs to no group, which is how an entity
// assignee is distinguished from a group one.
func (s *AdminProviderTestSuite) notAGroup(id string) {
	s.groups.On("GetGroup", mock.Anything, id, false).
		Return(nil, &group.ErrorGroupNotFound)
}

func (s *AdminProviderTestSuite) roleGranting(permissions []ResourcePermissions) {
	s.roles.On("GetRoleWithPermissions", mock.Anything, providerRoleID).
		Return(&RoleWithPermissions{ID: providerRoleID, Permissions: permissions}, nil)
}

func (s *AdminProviderTestSuite) resourceServer(id, identifier string) {
	s.resources.On("GetResourceServer", mock.Anything, id).
		Return(&providers.ResourceServer{ID: id, Identifier: identifier}, nil)
}

// The same permission string on two resource servers is two different scopes, so each must be paired
// with the audience that gives it meaning. This is the case the whole dimension exists for.
func (s *AdminProviderTestSuite) TestValidate_PairsEachPermissionWithItsAudience() {
	s.assigned(providerUserID, AssigneeTypeUser)
	s.roleGranting([]ResourcePermissions{
		{ResourceServerID: californiaRSID, Permissions: []string{"license", "vehicle-registration"}},
		{ResourceServerID: ohioRSID, Permissions: []string{"license"}},
	})
	s.resourceServer(californiaRSID, californiaAud)
	s.resourceServer(ohioRSID, ohioAud)
	s.notAGroup(providerUserID)

	target, svcErr := s.provider.ValidateRemoveRoleAssignment(
		context.Background(), providerRoleID, providerUserID)

	s.Require().Nil(svcErr)
	s.Equal([]string{providerUserID}, target.EntityIDs)
	s.ElementsMatch([]revocation.AudienceScope{
		{Audience: californiaAud, Scope: "license"},
		{Audience: californiaAud, Scope: "vehicle-registration"},
		{Audience: ohioAud, Scope: "license"},
	}, target.Scopes)
}

// A group assignee holds no tokens; its members do. The revocation has to reach them.
func (s *AdminProviderTestSuite) TestValidate_ExpandsGroupToItsMembers() {
	s.assigned(providerGroupID, AssigneeTypeGroup)
	s.roleGranting([]ResourcePermissions{
		{ResourceServerID: californiaRSID, Permissions: []string{"license"}}})
	s.resourceServer(californiaRSID, californiaAud)
	s.groups.On("GetGroup", mock.Anything, providerGroupID, false).
		Return(&group.Group{ID: providerGroupID}, nil)
	s.groups.On("GetGroupMembers", mock.Anything, providerGroupID, memberPageSize, 0, false).
		Return(&group.MemberListResponse{TotalResults: 2, Members: []group.Member{
			{ID: "member-1", Type: group.MemberTypeUser},
			{ID: "member-2", Type: group.MemberTypeAgent},
		}}, nil)

	target, svcErr := s.provider.ValidateRemoveRoleAssignment(
		context.Background(), providerRoleID, providerGroupID)

	s.Require().Nil(svcErr)
	s.ElementsMatch([]string{"member-1", "member-2"}, target.EntityIDs)
}

// A nested group is expanded too, since its members inherit the outer group's roles.
func (s *AdminProviderTestSuite) TestValidate_ExpandsNestedGroups() {
	s.assigned(providerGroupID, AssigneeTypeGroup)
	s.roleGranting([]ResourcePermissions{
		{ResourceServerID: californiaRSID, Permissions: []string{"license"}}})
	s.resourceServer(californiaRSID, californiaAud)
	s.groups.On("GetGroup", mock.Anything, providerGroupID, false).
		Return(&group.Group{ID: providerGroupID}, nil)
	s.groups.On("GetGroupMembers", mock.Anything, providerGroupID, memberPageSize, 0, false).
		Return(&group.MemberListResponse{TotalResults: 2, Members: []group.Member{
			{ID: "member-1", Type: group.MemberTypeUser},
			{ID: "nested-group", Type: group.MemberTypeGroup},
		}}, nil)
	s.nestedGroupHolding("nested-group", group.Member{ID: "member-3", Type: group.MemberTypeUser})

	target, svcErr := s.provider.ValidateRemoveRoleAssignment(
		context.Background(), providerRoleID, providerGroupID)

	s.Require().Nil(svcErr)
	s.ElementsMatch([]string{"member-1", "member-3"}, target.EntityIDs)
}

// A role granting nothing yields no scopes. The caller reports that as nothing to revoke and still
// performs the unassignment, so it must not be an error here.
func (s *AdminProviderTestSuite) TestValidate_RoleWithoutPermissionsYieldsNoScopes() {
	s.assigned(providerUserID, AssigneeTypeUser)
	s.roleGranting(nil)
	s.notAGroup(providerUserID)

	target, svcErr := s.provider.ValidateRemoveRoleAssignment(
		context.Background(), providerRoleID, providerUserID)

	s.Require().Nil(svcErr)
	s.Empty(target.Scopes)
	s.Equal([]string{providerUserID}, target.EntityIDs)
}

// A resource server that cannot be resolved contributes no audience, so its permissions would be
// unmatchable. Skipping them is better than recording a criterion keyed on an empty audience, which
// would silently match nothing.
func (s *AdminProviderTestSuite) TestValidate_SkipsResourceServerWithoutIdentifier() {
	s.assigned(providerUserID, AssigneeTypeUser)
	s.roleGranting([]ResourcePermissions{
		{ResourceServerID: californiaRSID, Permissions: []string{"license"}}})
	s.resources.On("GetResourceServer", mock.Anything, californiaRSID).
		Return(&providers.ResourceServer{ID: californiaRSID}, nil)
	s.notAGroup(providerUserID)

	target, svcErr := s.provider.ValidateRemoveRoleAssignment(
		context.Background(), providerRoleID, providerUserID)

	s.Require().Nil(svcErr)
	s.Empty(target.Scopes)
}

func (s *AdminProviderTestSuite) TestValidate_RequiresBothIdentifiers() {
	_, svcErr := s.provider.ValidateRemoveRoleAssignment(context.Background(), "", providerUserID)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorMissingRoleID.Code, svcErr.Code)

	_, svcErr = s.provider.ValidateRemoveRoleAssignment(context.Background(), providerRoleID, "")
	s.Require().NotNil(svcErr)
	s.Equal(ErrorMissingAssigneeID.Code, svcErr.Code)
}

// Assignment validation rejects an assignment with no type, so the provider must resolve it. Sending
// none is the defect this pins: it made every removal fail while the revocation had already been
// written.
func (s *AdminProviderTestSuite) TestRemove_SendsTheResolvedAssigneeType() {
	s.notAGroup(providerUserID)
	s.entities.On("GetEntity", mock.Anything, providerUserID).
		Return(&providers.Entity{ID: providerUserID, Category: providers.EntityCategoryUser}, nil)
	s.assignments.On("RemoveAssignments", mock.Anything, providerRoleID,
		[]RoleAssignment{{ID: providerUserID, Type: AssigneeTypeUser}}).Return(nil)

	svcErr := s.provider.RemoveRoleAssignment(context.Background(), providerRoleID, providerUserID)

	s.Nil(svcErr)
}

// Every entity category maps to the public assignee type of the same name; a group maps to group.
// The internal storage type must never be produced, because assignment validation rejects it.
func (s *AdminProviderTestSuite) TestRemove_ResolvesEveryAssigneeType() {
	cases := []struct {
		category providers.EntityCategory
		expected AssigneeType
	}{
		{providers.EntityCategoryUser, AssigneeTypeUser},
		{providers.EntityCategoryApp, AssigneeTypeApp},
		{providers.EntityCategoryAgent, AssigneeTypeAgent},
	}
	for _, tc := range cases {
		s.Run(string(tc.category), func() {
			s.SetupTest()
			s.notAGroup(providerUserID)
			s.entities.On("GetEntity", mock.Anything, providerUserID).
				Return(&providers.Entity{ID: providerUserID, Category: tc.category}, nil)
			s.assignments.On("RemoveAssignments", mock.Anything, providerRoleID,
				[]RoleAssignment{{ID: providerUserID, Type: tc.expected}}).Return(nil)

			s.Nil(s.provider.RemoveRoleAssignment(
				context.Background(), providerRoleID, providerUserID))
		})
	}
}

func (s *AdminProviderTestSuite) TestRemove_ResolvesGroupAssignee() {
	s.groups.On("GetGroup", mock.Anything, providerGroupID, false).
		Return(&group.Group{ID: providerGroupID}, nil)
	s.assignments.On("RemoveAssignments", mock.Anything, providerRoleID,
		[]RoleAssignment{{ID: providerGroupID, Type: AssigneeTypeGroup}}).Return(nil)

	s.Nil(s.provider.RemoveRoleAssignment(context.Background(), providerRoleID, providerGroupID))
}

// An assignee that is neither a group nor a known entity cannot be typed, and the removal must refuse
// rather than guess.
func (s *AdminProviderTestSuite) TestRemove_RefusesUnknownAssignee() {
	s.notAGroup("ghost")
	s.entities.On("GetEntity", mock.Anything, "ghost").Return(nil, errors.New("not found"))

	svcErr := s.provider.RemoveRoleAssignment(context.Background(), providerRoleID, "ghost")

	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidAssigneeType.Code, svcErr.Code)
}

// The service's own refusal must reach the caller, since the flow surfaces its code to the operator.
func (s *AdminProviderTestSuite) TestRemove_CarriesTheServiceRefusal() {
	refusal := tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "ROL-1003"}
	s.notAGroup(providerUserID)
	s.entities.On("GetEntity", mock.Anything, providerUserID).
		Return(&providers.Entity{ID: providerUserID, Category: providers.EntityCategoryUser}, nil)
	s.assignments.On("RemoveAssignments", mock.Anything, providerRoleID, mock.Anything).
		Return(&refusal)

	svcErr := s.provider.RemoveRoleAssignment(context.Background(), providerRoleID, providerUserID)

	s.Require().NotNil(svcErr)
	s.Equal("ROL-1003", svcErr.Code)
}

// A group lookup that fails for a reason other than absence is a real failure, not a signal that the
// assignee is an entity. Treating it as the latter would type the assignee wrongly.
func (s *AdminProviderTestSuite) TestValidate_PropagatesGroupLookupFailure() {
	s.assigned(providerUserID, AssigneeTypeUser)
	s.roleGranting([]ResourcePermissions{
		{ResourceServerID: californiaRSID, Permissions: []string{"license"}}})
	s.resourceServer(californiaRSID, californiaAud)
	s.groups.On("GetGroup", mock.Anything, providerUserID, false).
		Return(nil, &tidcommon.InternalServerError)

	_, svcErr := s.provider.ValidateRemoveRoleAssignment(
		context.Background(), providerRoleID, providerUserID)

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// assignedTo makes the role's assignment listing report exactly these assignees.
func (s *AdminProviderTestSuite) assignedTo(assignments ...RoleAssignmentWithDisplay) {
	s.assignments.On("GetRoleAssignments", mock.Anything, providerRoleID, assignmentPageSize, 0, false).
		Return(&AssignmentList{TotalResults: len(assignments), Assignments: assignments}, nil)
}

// groupHolding makes the group exist and report exactly these members. Use it for a group named
// directly by a caller, which the provider looks up to decide whether the id is a group at all.
func (s *AdminProviderTestSuite) groupHolding(groupID string, members ...group.Member) {
	s.groups.On("GetGroup", mock.Anything, groupID, false).Return(&group.Group{ID: groupID}, nil)
	s.nestedGroupHolding(groupID, members...)
}

// nestedGroupHolding stubs only the member listing, which is all a nested group needs: the parent's
// listing already reported its type, so the descent does not look it up again.
func (s *AdminProviderTestSuite) nestedGroupHolding(groupID string, members ...group.Member) {
	s.groups.On("GetGroupMembers", mock.Anything, groupID, memberPageSize, 0, false).
		Return(&group.MemberListResponse{TotalResults: len(members), Members: members}, nil)
}

func (s *AdminProviderTestSuite) mutableRole() {
	s.roles.On("IsRoleDeclarative", mock.Anything, providerRoleID).Return(false, nil)
}

// Deleting a role takes its scopes from everyone holding it, not from one named assignee, so the
// target must cover every assignment the role carries.
func (s *AdminProviderTestSuite) TestValidateRoleScopeChange_CoversEveryAssignee() {
	s.roleGranting([]ResourcePermissions{
		{ResourceServerID: californiaRSID, Permissions: []string{"license"}}})
	s.resourceServer(californiaRSID, californiaAud)
	s.mutableRole()
	s.assignedTo(
		RoleAssignmentWithDisplay{ID: providerUserID, Type: AssigneeTypeUser},
		RoleAssignmentWithDisplay{ID: providerGroupID, Type: AssigneeTypeGroup},
	)
	s.groupHolding(providerGroupID, group.Member{ID: "member-1", Type: group.MemberTypeUser})

	target, svcErr := s.provider.ValidateRoleScopeChange(context.Background(), providerRoleID)

	s.Require().Nil(svcErr)
	s.ElementsMatch([]string{providerUserID, "member-1"}, target.EntityIDs)
	s.Equal([]revocation.AudienceScope{{Audience: californiaAud, Scope: "license"}}, target.Scopes)
}

// A principal holding the role directly and through a group appears once. Two identical criteria would
// be idempotent writes, but they inflate the fan-out the cap is measured against.
func (s *AdminProviderTestSuite) TestValidateRoleScopeChange_DeduplicatesAssignees() {
	s.roleGranting([]ResourcePermissions{
		{ResourceServerID: californiaRSID, Permissions: []string{"license"}}})
	s.resourceServer(californiaRSID, californiaAud)
	s.mutableRole()
	s.assignedTo(
		RoleAssignmentWithDisplay{ID: providerUserID, Type: AssigneeTypeUser},
		RoleAssignmentWithDisplay{ID: providerGroupID, Type: AssigneeTypeGroup},
	)
	s.groupHolding(providerGroupID, group.Member{ID: providerUserID, Type: group.MemberTypeUser})

	target, svcErr := s.provider.ValidateRoleScopeChange(context.Background(), providerRoleID)

	s.Require().Nil(svcErr)
	s.Equal([]string{providerUserID}, target.EntityIDs)
}

// A declarative role can be neither deleted nor edited. Refusing during validation is what keeps the
// flow from denying scopes for a change that was always going to fail.
func (s *AdminProviderTestSuite) TestValidateRoleScopeChange_RefusesDeclarativeRole() {
	s.roleGranting([]ResourcePermissions{
		{ResourceServerID: californiaRSID, Permissions: []string{"license"}}})
	s.roles.On("IsRoleDeclarative", mock.Anything, providerRoleID).Return(true, nil)

	_, svcErr := s.provider.ValidateRoleScopeChange(context.Background(), providerRoleID)

	s.Require().NotNil(svcErr)
	s.Equal(ErrorImmutableRole.Code, svcErr.Code)
}

func (s *AdminProviderTestSuite) TestValidateRoleScopeChange_RequiresARole() {
	_, svcErr := s.provider.ValidateRoleScopeChange(context.Background(), "")
	s.Require().NotNil(svcErr)
	s.Equal(ErrorMissingRoleID.Code, svcErr.Code)
}

// The update takes a whole role, so the attributes the caller did not set must be carried over rather
// than sent empty. Sending them empty would rename the role and move it out of its organization unit.
func (s *AdminProviderTestSuite) TestUpdateRolePermissions_PreservesTheOtherAttributes() {
	s.roles.On("GetRoleWithPermissions", mock.Anything, providerRoleID).
		Return(&RoleWithPermissions{
			ID: providerRoleID, Name: "DMV Clerk", Description: "Counter staff", OUID: "ou-1",
			Permissions: []ResourcePermissions{
				{ResourceServerID: californiaRSID, Permissions: []string{"license", "permits"}}},
		}, nil)
	s.roles.On("UpdateRoleWithPermissions", mock.Anything, providerRoleID, RoleUpdateDetail{
		Name:        "DMV Clerk",
		Description: "Counter staff",
		OUID:        "ou-1",
		Permissions: []ResourcePermissions{
			{ResourceServerID: californiaRSID, Permissions: []string{"license"}}},
	}).Return(&RoleWithPermissions{ID: providerRoleID}, nil)

	svcErr := s.provider.UpdateRolePermissions(context.Background(), providerRoleID,
		[]revocation.RolePermissions{
			{ResourceServerID: californiaRSID, Permissions: []string{"license"}}})

	s.Nil(svcErr)
}

// An edit that leaves the role granting nothing is the largest permission removal there is, and must
// be applied rather than treated as an omission.
func (s *AdminProviderTestSuite) TestUpdateRolePermissions_AcceptsAnEmptySet() {
	s.roles.On("GetRoleWithPermissions", mock.Anything, providerRoleID).
		Return(&RoleWithPermissions{ID: providerRoleID, Name: "DMV Clerk", OUID: "ou-1"}, nil)
	s.roles.On("UpdateRoleWithPermissions", mock.Anything, providerRoleID, RoleUpdateDetail{
		Name: "DMV Clerk", OUID: "ou-1", Permissions: []ResourcePermissions{},
	}).Return(&RoleWithPermissions{ID: providerRoleID}, nil)

	s.Nil(s.provider.UpdateRolePermissions(context.Background(), providerRoleID, nil))
}

func (s *AdminProviderTestSuite) TestDeleteRole_CarriesTheServiceRefusal() {
	refusal := tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "ROL-1013"}
	s.roles.On("DeleteRole", mock.Anything, providerRoleID).Return(&refusal)

	svcErr := s.provider.DeleteRole(context.Background(), providerRoleID)

	s.Require().NotNil(svcErr)
	s.Equal("ROL-1013", svcErr.Code)
}

// Membership conveys a parent group's roles as well as the group's own, so the ancestors must be part
// of the question. Asking about the group alone under-revokes, which is the unsafe direction.
func (s *AdminProviderTestSuite) TestValidateGroupMembershipChange_IncludesAncestorScopes() {
	s.groups.On("GetTransitiveAncestorGroups", mock.Anything, providerGroupID).
		Return([]string{"parent-group"}, nil)
	s.roles.On("GetAllPermissions", mock.Anything, "", []string{providerGroupID, "parent-group"}).
		Return(security.PermissionSet{
			californiaRSID: {"license"},
			ohioRSID:       {"license"},
		}, nil)
	s.resourceServer(californiaRSID, californiaAud)
	s.resourceServer(ohioRSID, ohioAud)
	s.groupHolding(providerGroupID, group.Member{ID: "member-1", Type: group.MemberTypeUser})

	target, svcErr := s.provider.ValidateGroupMembershipChange(
		context.Background(), providerGroupID, "")

	s.Require().Nil(svcErr)
	s.Equal([]string{"member-1"}, target.EntityIDs,
		"deleting the group costs every transitive member the path through it")
	s.ElementsMatch([]revocation.AudienceScope{
		{Audience: californiaAud, Scope: "license"},
		{Audience: ohioAud, Scope: "license"},
	}, target.Scopes)
}

// Removing one member costs that member the path, and nobody else. Expanding to the whole group here
// would revoke for members who are still in it.
func (s *AdminProviderTestSuite) TestValidateGroupMembershipChange_TargetsOnlyTheDepartingMember() {
	s.groups.On("GetGroup", mock.Anything, providerGroupID, false).
		Return(&group.Group{ID: providerGroupID}, nil)
	s.groups.On("GetTransitiveAncestorGroups", mock.Anything, providerGroupID).
		Return([]string{}, nil)
	s.roles.On("GetAllPermissions", mock.Anything, "", []string{providerGroupID}).
		Return(security.PermissionSet{californiaRSID: {"license"}}, nil)
	s.resourceServer(californiaRSID, californiaAud)
	// The member must actually be in the group: a removal that changes nothing must revoke nothing.
	s.nestedGroupHolding(providerGroupID,
		group.Member{ID: providerUserID, Type: group.MemberTypeUser},
		group.Member{ID: "stays-behind", Type: group.MemberTypeUser})
	s.notAGroup(providerUserID)

	target, svcErr := s.provider.ValidateGroupMembershipChange(
		context.Background(), providerGroupID, providerUserID)

	s.Require().Nil(svcErr)
	s.Equal([]string{providerUserID}, target.EntityIDs,
		"only the departing member loses the path, not everyone in the group")
}

// A departing member that is itself a group holds no tokens; its members do.
func (s *AdminProviderTestSuite) TestValidateGroupMembershipChange_ExpandsADepartingGroup() {
	s.groups.On("GetGroup", mock.Anything, providerGroupID, false).
		Return(&group.Group{ID: providerGroupID}, nil)
	s.groups.On("GetTransitiveAncestorGroups", mock.Anything, providerGroupID).
		Return([]string{}, nil)
	s.roles.On("GetAllPermissions", mock.Anything, "", []string{providerGroupID}).
		Return(security.PermissionSet{californiaRSID: {"license"}}, nil)
	s.resourceServer(californiaRSID, californiaAud)
	s.nestedGroupHolding(providerGroupID,
		group.Member{ID: "child-group", Type: group.MemberTypeGroup})
	s.groupHolding("child-group", group.Member{ID: "member-1", Type: group.MemberTypeUser})

	target, svcErr := s.provider.ValidateGroupMembershipChange(
		context.Background(), providerGroupID, "child-group")

	s.Require().Nil(svcErr)
	s.Equal([]string{"member-1"}, target.EntityIDs)
}

// A declarative group cannot be changed, so validation refuses before anything is revoked.
func (s *AdminProviderTestSuite) TestValidateGroupMembershipChange_RefusesDeclarativeGroup() {
	s.groups.On("GetGroup", mock.Anything, providerGroupID, false).
		Return(&group.Group{ID: providerGroupID, IsReadOnly: true}, nil)

	_, svcErr := s.provider.ValidateGroupMembershipChange(
		context.Background(), providerGroupID, providerUserID)

	s.Require().NotNil(svcErr)
	s.Equal(group.ErrorImmutableGroup.Code, svcErr.Code)
}

func (s *AdminProviderTestSuite) TestValidateGroupMembershipChange_RequiresAGroup() {
	_, svcErr := s.provider.ValidateGroupMembershipChange(context.Background(), "", providerUserID)
	s.Require().NotNil(svcErr)
	s.Equal(group.ErrorMissingGroupID.Code, svcErr.Code)
}

// Membership validation rejects a member with no type, so the provider must resolve it, exactly as the
// role assignment path does.
func (s *AdminProviderTestSuite) TestRemoveGroupMember_SendsTheResolvedMemberType() {
	s.notAGroup(providerUserID)
	s.entities.On("GetEntity", mock.Anything, providerUserID).
		Return(&providers.Entity{ID: providerUserID, Category: providers.EntityCategoryUser}, nil)
	s.groups.On("RemoveGroupMembers", mock.Anything, providerGroupID,
		[]group.Member{{ID: providerUserID, Type: group.MemberTypeUser}}).
		Return(&group.Group{ID: providerGroupID}, nil)

	s.Nil(s.provider.RemoveGroupMember(context.Background(), providerGroupID, providerUserID))
}

// A nested group leaving its parent is removed as a group member, not as an entity.
func (s *AdminProviderTestSuite) TestRemoveGroupMember_ResolvesAGroupMember() {
	s.groups.On("GetGroup", mock.Anything, "child-group", false).
		Return(&group.Group{ID: "child-group"}, nil)
	s.groups.On("RemoveGroupMembers", mock.Anything, providerGroupID,
		[]group.Member{{ID: "child-group", Type: group.MemberTypeGroup}}).
		Return(&group.Group{ID: providerGroupID}, nil)

	s.Nil(s.provider.RemoveGroupMember(context.Background(), providerGroupID, "child-group"))
}

func (s *AdminProviderTestSuite) TestRemoveGroupMember_RequiresAMember() {
	svcErr := s.provider.RemoveGroupMember(context.Background(), providerGroupID, "")
	s.Require().NotNil(svcErr)
	s.Equal(group.ErrorInvalidMemberID.Code, svcErr.Code)
}

func (s *AdminProviderTestSuite) TestDeleteGroup_CarriesTheServiceRefusal() {
	refusal := tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "GRP-1015"}
	s.groups.On("DeleteGroup", mock.Anything, providerGroupID).Return(&refusal)

	svcErr := s.provider.DeleteGroup(context.Background(), providerGroupID)

	s.Require().NotNil(svcErr)
	s.Equal("GRP-1015", svcErr.Code)
}

// The services reject an over-large page outright rather than clamping it, so a page size chosen
// independently of the server's maximum makes every expansion fail with an invalid-limit refusal. That
// is invisible to a mocked test and only shows up against a running server.
func (s *AdminProviderTestSuite) TestPageSizesDoNotExceedTheServerMaximum() {
	s.LessOrEqual(memberPageSize, serverconst.MaxPageSize)
	s.LessOrEqual(assignmentPageSize, serverconst.MaxPageSize)
	s.Positive(memberPageSize)
	s.Positive(assignmentPageSize)
}

// Membership is a graph, not a tree: nothing stops two groups sharing a nested member. Without a
// visited set a cycle recurses until the stack dies, so the descent must return a refusal instead.
func (s *AdminProviderTestSuite) TestValidateGroupMembershipChange_SurvivesAMembershipCycle() {
	s.groups.On("GetGroup", mock.Anything, providerGroupID, false).
		Return(&group.Group{ID: providerGroupID}, nil)
	s.groups.On("GetTransitiveAncestorGroups", mock.Anything, providerGroupID).Return([]string{}, nil)
	s.roles.On("GetAllPermissions", mock.Anything, "", []string{providerGroupID}).
		Return(security.PermissionSet{californiaRSID: {"license"}}, nil)
	s.resourceServer(californiaRSID, californiaAud)
	// The group contains a child, and the child contains the group again.
	s.groupHolding(providerGroupID, group.Member{ID: "child-group", Type: group.MemberTypeGroup})
	s.nestedGroupHolding("child-group", group.Member{ID: providerGroupID, Type: group.MemberTypeGroup},
		group.Member{ID: providerUserID, Type: group.MemberTypeUser})

	target, svcErr := s.provider.ValidateGroupMembershipChange(
		context.Background(), providerGroupID, "")

	s.Require().Nil(svcErr, "a cycle must be tolerated, not fatal")
	s.Equal([]string{providerUserID}, target.EntityIDs,
		"the cycle must be walked once, yielding each entity exactly once")
}

// An entity reachable through two nested groups is one principal. Returning it twice would double the
// criteria written and the row count charged against the fan-out cap.
func (s *AdminProviderTestSuite) TestValidateGroupMembershipChange_DeduplicatesADiamond() {
	s.groups.On("GetGroup", mock.Anything, providerGroupID, false).
		Return(&group.Group{ID: providerGroupID}, nil)
	s.groups.On("GetTransitiveAncestorGroups", mock.Anything, providerGroupID).Return([]string{}, nil)
	s.roles.On("GetAllPermissions", mock.Anything, "", []string{providerGroupID}).
		Return(security.PermissionSet{californiaRSID: {"license"}}, nil)
	s.resourceServer(californiaRSID, californiaAud)
	s.groupHolding(providerGroupID,
		group.Member{ID: "left", Type: group.MemberTypeGroup},
		group.Member{ID: "right", Type: group.MemberTypeGroup})
	s.nestedGroupHolding("left", group.Member{ID: providerUserID, Type: group.MemberTypeUser})
	s.nestedGroupHolding("right", group.Member{ID: providerUserID, Type: group.MemberTypeUser})

	target, svcErr := s.provider.ValidateGroupMembershipChange(
		context.Background(), providerGroupID, "")

	s.Require().Nil(svcErr)
	s.Equal([]string{providerUserID}, target.EntityIDs,
		"an entity reachable by two paths is still one principal")
}

// Removing a non-member changes nothing, so revoking their tokens would deny scopes they hold by other
// paths for no reason at all, with nothing to restore them.
func (s *AdminProviderTestSuite) TestValidateGroupMembershipChange_RefusesANonMember() {
	s.groups.On("GetGroup", mock.Anything, providerGroupID, false).
		Return(&group.Group{ID: providerGroupID}, nil)
	s.groups.On("GetTransitiveAncestorGroups", mock.Anything, providerGroupID).Return([]string{}, nil)
	s.roles.On("GetAllPermissions", mock.Anything, "", []string{providerGroupID}).
		Return(security.PermissionSet{californiaRSID: {"license"}}, nil)
	s.resourceServer(californiaRSID, californiaAud)
	s.groups.On("GetGroupMembers", mock.Anything, providerGroupID, memberPageSize, 0, false).
		Return(&group.MemberListResponse{TotalResults: 1, Members: []group.Member{
			{ID: "someone-else", Type: group.MemberTypeUser},
		}}, nil)

	_, svcErr := s.provider.ValidateGroupMembershipChange(
		context.Background(), providerGroupID, providerUserID)

	s.Require().NotNil(svcErr)
	s.Equal(group.ErrorInvalidMemberID.Code, svcErr.Code)
}

// Removing an assignment that was never there is a silent no-op in the store, so validation must
// refuse it. Without this the flow would deny the principal every scope the role grants, change
// nothing, and report success, with nothing left to restore those tokens. This is the role-side
// counterpart of TestValidateGroupMembershipChange_RefusesANonMember.
func (s *AdminProviderTestSuite) TestValidate_RefusesAnAssigneeThatDoesNotHoldTheRole() {
	s.roleGranting([]ResourcePermissions{
		{ResourceServerID: californiaRSID, Permissions: []string{"license"}}})
	s.assignments.On("GetRoleAssignments", mock.Anything, providerRoleID, assignmentPageSize, 0, false).
		Return(&AssignmentList{TotalResults: 1, Assignments: []RoleAssignmentWithDisplay{
			{ID: "someone-else", Type: AssigneeTypeUser},
		}}, nil)

	_, svcErr := s.provider.ValidateRemoveRoleAssignment(
		context.Background(), providerRoleID, providerUserID)

	s.Require().NotNil(svcErr)
	s.Equal(ErrorRoleAssignmentNotFound.Code, svcErr.Code)
}

// The assignment may be on any page, so the check has to page rather than read the first.
func (s *AdminProviderTestSuite) TestValidate_FindsAnAssignmentOnALaterPage() {
	s.roleGranting(nil)
	s.assignments.On("GetRoleAssignments", mock.Anything, providerRoleID, assignmentPageSize, 0, false).
		Return(&AssignmentList{
			TotalResults: assignmentPageSize + 1,
			Assignments:  []RoleAssignmentWithDisplay{{ID: "someone-else", Type: AssigneeTypeUser}},
		}, nil)
	s.assignments.On("GetRoleAssignments", mock.Anything, providerRoleID,
		assignmentPageSize, assignmentPageSize, false).
		Return(&AssignmentList{
			TotalResults: assignmentPageSize + 1,
			Assignments:  []RoleAssignmentWithDisplay{{ID: providerUserID, Type: AssigneeTypeUser}},
		}, nil)
	s.notAGroup(providerUserID)

	target, svcErr := s.provider.ValidateRemoveRoleAssignment(
		context.Background(), providerRoleID, providerUserID)

	s.Require().Nil(svcErr)
	s.Equal([]string{providerUserID}, target.EntityIDs)
}
