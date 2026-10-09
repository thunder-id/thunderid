// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/revocation"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/groupmock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
)

const (
	targetRoleID   = "role-1"
	targetUserID   = "user-1"
	targetGroupID  = "group-1"
	californiaRSID = "rs-ca"
	ohioRSID       = "rs-oh"
	californiaAud  = "https://api.dmv.ca.gov"
	ohioAud        = "https://api.dmv.oh.gov"
)

func stubGroupMembers(groups *groupmock.GroupServiceInterfaceMock, groupID string, members ...group.Member) {
	groups.On("GetGroupMembers", mock.Anything, groupID, memberPageSize, 0, false).
		Return(&group.MemberListResponse{TotalResults: len(members), Members: members}, nil)
}

func stubResourceServer(resources *resourcemock.ResourceServiceInterfaceMock, id, identifier string) {
	resources.On("GetResourceServer", mock.Anything, id).
		Return(&providers.ResourceServer{ID: id, Identifier: identifier}, nil)
}

type RoleUtilsTestSuite struct {
	suite.Suite
	groups    *groupmock.GroupServiceInterfaceMock
	resources *resourcemock.ResourceServiceInterfaceMock
}

func TestRoleUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(RoleUtilsTestSuite))
}

func (s *RoleUtilsTestSuite) SetupTest() {
	s.groups = groupmock.NewGroupServiceInterfaceMock(s.T())
	s.resources = resourcemock.NewResourceServiceInterfaceMock(s.T())
}

// The services refuse an over-large page rather than clamping it, which a mocked test cannot show.
func (s *RoleUtilsTestSuite) TestPageSizesDoNotExceedTheServerMaximum() {
	s.LessOrEqual(memberPageSize, serverconst.MaxPageSize)
	s.LessOrEqual(assignmentPageSize, serverconst.MaxPageSize)
	s.Positive(memberPageSize)
	s.Positive(assignmentPageSize)
}

func (s *RoleUtilsTestSuite) TestResolveScopes_PairsEachPermissionWithItsAudience() {
	stubResourceServer(s.resources, californiaRSID, californiaAud)
	stubResourceServer(s.resources, ohioRSID, ohioAud)

	scopes, svcErr := resolveScopes(context.Background(), s.resources, []ResourcePermissions{
		{ResourceServerID: californiaRSID, Permissions: []string{"license", "vehicle-registration"}},
		{ResourceServerID: ohioRSID, Permissions: []string{"license"}},
		{ResourceServerID: "rs-empty"},
	})

	s.Require().Nil(svcErr)
	s.ElementsMatch([]revocation.AudienceScope{
		{Audience: californiaAud, Scope: "license"},
		{Audience: californiaAud, Scope: "vehicle-registration"},
		{Audience: ohioAud, Scope: "license"},
	}, scopes)
}

// A resource server without an identifier issues no audience, so no criterion could match its scopes.
func (s *RoleUtilsTestSuite) TestResolveScopes_SkipsResourceServerWithoutIdentifier() {
	s.resources.On("GetResourceServer", mock.Anything, californiaRSID).
		Return(&providers.ResourceServer{ID: californiaRSID}, nil)

	scopes, svcErr := resolveScopes(context.Background(), s.resources, []ResourcePermissions{
		{ResourceServerID: californiaRSID, Permissions: []string{"license"}}})

	s.Require().Nil(svcErr)
	s.Empty(scopes)
}

func (s *RoleUtilsTestSuite) TestResolveScopes_CarriesTheLookupError() {
	s.resources.On("GetResourceServer", mock.Anything, californiaRSID).
		Return(nil, &tidcommon.InternalServerError)

	_, svcErr := resolveScopes(context.Background(), s.resources, []ResourcePermissions{
		{ResourceServerID: californiaRSID, Permissions: []string{"license"}}})

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *RoleUtilsTestSuite) TestExpandPrincipal_AnEntityIsItself() {
	entityIDs, svcErr := expandPrincipal(context.Background(), s.groups, targetUserID, false)

	s.Require().Nil(svcErr)
	s.Equal([]string{targetUserID}, entityIDs)
}

// A nested group is expanded too, since its members inherit the outer group's roles.
func (s *RoleUtilsTestSuite) TestExpandPrincipal_ExpandsNestedGroups() {
	stubGroupMembers(s.groups, targetGroupID,
		group.Member{ID: "member-1", Type: group.MemberTypeUser},
		group.Member{ID: "nested-group", Type: group.MemberTypeGroup})
	stubGroupMembers(s.groups, "nested-group", group.Member{ID: "member-2", Type: group.MemberTypeAgent})

	entityIDs, svcErr := expandPrincipal(context.Background(), s.groups, targetGroupID, true)

	s.Require().Nil(svcErr)
	s.ElementsMatch([]string{"member-1", "member-2"}, entityIDs)
}

func (s *RoleUtilsTestSuite) TestGroupMemberIDs_SurvivesAMembershipCycle() {
	stubGroupMembers(s.groups, targetGroupID, group.Member{ID: "child-group", Type: group.MemberTypeGroup})
	stubGroupMembers(s.groups, "child-group",
		group.Member{ID: targetGroupID, Type: group.MemberTypeGroup},
		group.Member{ID: targetUserID, Type: group.MemberTypeUser})

	entityIDs, svcErr := groupMemberIDs(context.Background(), s.groups, targetGroupID)

	s.Require().Nil(svcErr)
	s.Equal([]string{targetUserID}, entityIDs)
}

func (s *RoleUtilsTestSuite) TestGroupMemberIDs_DeduplicatesADiamond() {
	stubGroupMembers(s.groups, targetGroupID,
		group.Member{ID: "left", Type: group.MemberTypeGroup},
		group.Member{ID: "right", Type: group.MemberTypeGroup})
	stubGroupMembers(s.groups, "left", group.Member{ID: targetUserID, Type: group.MemberTypeUser})
	stubGroupMembers(s.groups, "right", group.Member{ID: targetUserID, Type: group.MemberTypeUser})

	entityIDs, svcErr := groupMemberIDs(context.Background(), s.groups, targetGroupID)

	s.Require().Nil(svcErr)
	s.Equal([]string{targetUserID}, entityIDs)
}

// A partial expansion would silently miss members, so nesting past the bound is refused.
func (s *RoleUtilsTestSuite) TestGroupMemberIDs_RefusesNestingBeyondTheBound() {
	for depth := 0; depth < maxGroupDescentDepth; depth++ {
		stubGroupMembers(s.groups, fmt.Sprintf("g-%d", depth),
			group.Member{ID: fmt.Sprintf("g-%d", depth+1), Type: group.MemberTypeGroup})
	}

	_, svcErr := groupMemberIDs(context.Background(), s.groups, "g-0")

	s.Require().NotNil(svcErr)
	s.Equal(ErrorGroupNestingTooDeep.Code, svcErr.Code)
}

func (s *RoleUtilsTestSuite) TestGroupMembers_FollowsPagination() {
	s.groups.On("GetGroupMembers", mock.Anything, targetGroupID, memberPageSize, 0, false).
		Return(&group.MemberListResponse{TotalResults: memberPageSize + 1,
			Members: []group.Member{{ID: "member-1", Type: group.MemberTypeUser}}}, nil)
	s.groups.On("GetGroupMembers", mock.Anything, targetGroupID, memberPageSize, memberPageSize, false).
		Return(&group.MemberListResponse{TotalResults: memberPageSize + 1,
			Members: []group.Member{{ID: "member-2", Type: group.MemberTypeUser}}}, nil)

	members, svcErr := groupMembers(context.Background(), s.groups, targetGroupID)

	s.Require().Nil(svcErr)
	s.Len(members, 2)
}

func (s *RoleUtilsTestSuite) TestGroupMembers_CarriesTheListingError() {
	s.groups.On("GetGroupMembers", mock.Anything, targetGroupID, memberPageSize, 0, false).
		Return(nil, &group.ErrorGroupNotFound)

	_, svcErr := groupMembers(context.Background(), s.groups, targetGroupID)

	s.Require().NotNil(svcErr)
	s.Equal(group.ErrorGroupNotFound.Code, svcErr.Code)
}
