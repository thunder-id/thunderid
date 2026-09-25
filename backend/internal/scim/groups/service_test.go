// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package groups

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/group"
	scim "github.com/thunder-id/thunderid/internal/scim/common"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/groupmock"
)

// ServiceTestSuite groups the tests in service_test.go.
type ServiceTestSuite struct {
	suite.Suite
}

// TestServiceTestSuite runs ServiceTestSuite.
func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

// TestGetGroup_Success verifies that GetGroup returns members even though
// group.GroupServiceInterface.GetGroup does not populate Members for
// database-backed groups; the service must fetch them via GetGroupMembers.
// TestGetGroup_Success tests Get Group for Success.
func (suite *ServiceTestSuite) TestGetGroup_Success() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	groupNoMembers := &group.Group{ID: "group-1", Name: "Administrators"}
	mockGroupService.On("GetGroup", mock.Anything, "group-1", true).
		Return(groupNoMembers, (*tidcommon.ServiceError)(nil))

	members := &group.MemberListResponse{
		TotalResults: 2,
		Members: []group.Member{
			{ID: "user-1", Type: group.MemberTypeUser},
			{ID: "user-2", Type: group.MemberTypeUser},
		},
	}
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return(members, (*tidcommon.ServiceError)(nil))

	scimGroup, err := service.GetGroup(context.Background(), "group-1", testBaseURL)

	require.Nil(t, err)
	require.NotNil(t, scimGroup)
	require.Len(t, scimGroup.Members, 2)
	require.Equal(t, "user-1", scimGroup.Members[0].Value)
	require.Equal(t, "user-2", scimGroup.Members[1].Value)
}

// TestGetGroup_MembersExceedingPageSize verifies that GetGroup pages through
// GetGroupMembers when a group has more members than serverconst.MaxPageSize,
// rather than requesting an oversized limit that GetGroupMembers would reject.
// TestGetGroup_MembersExceedingPageSize tests Get Group for Members Exceeding Page Size.
func (suite *ServiceTestSuite) TestGetGroup_MembersExceedingPageSize() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	groupNoMembers := &group.Group{ID: "group-1", Name: "Administrators"}
	mockGroupService.On("GetGroup", mock.Anything, "group-1", true).
		Return(groupNoMembers, (*tidcommon.ServiceError)(nil))

	totalMembers := serverconst.MaxPageSize + 1
	firstPage := make([]group.Member, serverconst.MaxPageSize)
	for i := range firstPage {
		firstPage[i] = group.Member{ID: "user", Type: group.MemberTypeUser}
	}
	secondPage := []group.Member{{ID: "user-last", Type: group.MemberTypeUser}}

	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return(&group.MemberListResponse{TotalResults: totalMembers, Members: firstPage},
			(*tidcommon.ServiceError)(nil))
	mockGroupService.On(
		"GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, serverconst.MaxPageSize, true,
	).Return(&group.MemberListResponse{TotalResults: totalMembers, Members: secondPage},
		(*tidcommon.ServiceError)(nil))

	scimGroup, err := service.GetGroup(context.Background(), "group-1", testBaseURL)

	require.Nil(t, err)
	require.NotNil(t, scimGroup)
	require.Len(t, scimGroup.Members, totalMembers)
	require.Equal(t, "user-last", scimGroup.Members[totalMembers-1].Value)
}

// TestGetGroup_NotFound tests Get Group for Not Found.
func (suite *ServiceTestSuite) TestGetGroup_NotFound() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "missing", true).
		Return((*group.Group)(nil), &group.ErrorGroupNotFound)

	scimGroup, err := service.GetGroup(context.Background(), "missing", testBaseURL)

	require.NotNil(t, err)
	require.Nil(t, scimGroup)
}

// TestListGroups_Success verifies that ListGroups populates members for each
// returned group by fetching them separately, since GetGroupList only yields
// GroupBasic entries with no Members, without an extra GetGroup call per group.
// TestListGroups_Success tests List Groups for Success.
func (suite *ServiceTestSuite) TestListGroups_Success() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	listResp := &group.GroupListResponse{
		TotalResults: 1,
		Groups: []group.GroupBasic{
			{ID: "group-1", Name: "Administrators"},
		},
	}
	mockGroupService.On("GetGroupList", mock.Anything, 20, 0, true).
		Return(listResp, (*tidcommon.ServiceError)(nil))

	members := &group.MemberListResponse{
		TotalResults: 1,
		Members: []group.Member{
			{ID: "user-1", Type: group.MemberTypeUser},
		},
	}
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return(members, (*tidcommon.ServiceError)(nil))

	resp, err := service.ListGroups(context.Background(), 1, 20, testBaseURL)

	require.Nil(t, err)
	require.Len(t, resp.Resources, 1)
	require.Len(t, resp.Resources[0].Members, 1)
	require.Equal(t, "user-1", resp.Resources[0].Members[0].Value)
}

// TestListGroups_ExplicitZeroCountReturnsNoResources tests List Groups for Explicit Zero Count Returns No Resources.
func (suite *ServiceTestSuite) TestListGroups_ExplicitZeroCountReturnsNoResources() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	listResp := &group.GroupListResponse{
		TotalResults: 5,
		Groups: []group.GroupBasic{
			{ID: "group-1", Name: "Administrators"},
		},
	}
	mockGroupService.On("GetGroupList", mock.Anything, 1, 0, true).
		Return(listResp, (*tidcommon.ServiceError)(nil))

	resp, err := service.ListGroups(context.Background(), 1, 0, testBaseURL)

	require.Nil(t, err)
	require.Equal(t, 5, resp.TotalResults)
	require.Empty(t, resp.Resources)
	require.Equal(t, 0, resp.ItemsPerPage)
}

// TestCreateGroup_Success verifies that CreateGroup re-fetches the created group
// instead of returning the raw creation result, since group.Service.CreateGroup
// strips member Display when persisting and never resolves it before returning.
// TestCreateGroup_Success tests Create Group for Success.
func (suite *ServiceTestSuite) TestCreateGroup_Success() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	created := &group.Group{ID: "group-1", Name: "Engineering Team"}
	mockGroupService.On("CreateGroup", mock.Anything, mock.Anything).
		Return(created, (*tidcommon.ServiceError)(nil))

	groupNoMembers := &group.Group{ID: "group-1", Name: "Engineering Team"}
	mockGroupService.On("GetGroup", mock.Anything, "group-1", true).
		Return(groupNoMembers, (*tidcommon.ServiceError)(nil))

	members := &group.MemberListResponse{
		TotalResults: 1,
		Members: []group.Member{
			{ID: "user-1", Type: group.MemberTypeUser, Display: "johndoe234"},
		},
	}
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return(members, (*tidcommon.ServiceError)(nil))

	scimGroup, err := service.CreateGroup(context.Background(), "Engineering Team",
		[]SCIMGroupMember{{Value: "user-1", Type: "User"}}, testBaseURL)

	require.Nil(t, err)
	require.NotNil(t, scimGroup)
	require.Len(t, scimGroup.Members, 1)
	require.Equal(t, "johndoe234", scimGroup.Members[0].Display)
}

// TestCreateGroup_UnsupportedMemberType verifies that an unrecognized member type
// is rejected before reaching group.Service.CreateGroup, rather than silently
// defaulting to a User member.
// TestCreateGroup_UnsupportedMemberType tests Create Group for Unsupported Member Type.
func (suite *ServiceTestSuite) TestCreateGroup_UnsupportedMemberType() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	scimGroup, err := service.CreateGroup(context.Background(), "Engineering Team",
		[]SCIMGroupMember{{Value: "user-1", Type: "anything"}}, testBaseURL)

	require.Nil(t, scimGroup)
	require.Equal(t, scim.ErrorUnsupportedMemberType.Code, err.Code)
}

// TestReplaceGroup_Success tests Replace Group for Success.
func (suite *ServiceTestSuite) TestReplaceGroup_Success() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Old Name"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("UpdateGroup", mock.Anything, "group-1", mock.Anything).
		Return(&group.Group{ID: "group-1", Name: "New Name"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, false).
		Return(&group.MemberListResponse{Members: []group.Member{{ID: "old-user", Type: group.MemberTypeUser}}},
			(*tidcommon.ServiceError)(nil))
	mockGroupService.On("RemoveGroupMembers", mock.Anything, "group-1",
		[]group.Member{{ID: "old-user", Type: group.MemberTypeUser}}).
		Return(&group.Group{}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("AddGroupMembers", mock.Anything, "group-1",
		[]group.Member{{ID: "new-user", Type: group.MemberTypeUser}}).
		Return(&group.Group{}, (*tidcommon.ServiceError)(nil))

	// Post-replace re-fetch (GetGroup(includeDisplay=true) + GetGroupMembers(includeDisplay=true))
	mockGroupService.On("GetGroup", mock.Anything, "group-1", true).
		Return(&group.Group{ID: "group-1", Name: "New Name"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return(&group.MemberListResponse{Members: []group.Member{{ID: "new-user", Type: group.MemberTypeUser}}},
			(*tidcommon.ServiceError)(nil))

	scimGroup, err := service.ReplaceGroup(context.Background(), "group-1", "New Name",
		[]SCIMGroupMember{{Value: "new-user", Type: "User"}}, testBaseURL)

	require.Nil(t, err)
	require.Equal(t, "New Name", scimGroup.DisplayName)
	require.Len(t, scimGroup.Members, 1)
	require.Equal(t, "new-user", scimGroup.Members[0].Value)
}

// TestReplaceGroupMembers_OnlyChangesDiff tests that replace removes and adds only the members that differ.
func (suite *ServiceTestSuite) TestReplaceGroupMembers_OnlyChangesDiff() {
	t := suite.T()
	user := func(id string) group.Member { return group.Member{ID: id, Type: group.MemberTypeUser} }

	t.Run("RemovesAndAddsOnlyDiff", func(t *testing.T) {
		mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
		service := newSCIMGroupsService(mockGroupService).(*scimGroupsService)

		mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, false).
			Return(&group.MemberListResponse{TotalResults: 2, Members: []group.Member{user("keep"), user("drop")}},
				(*tidcommon.ServiceError)(nil))
		mockGroupService.On("RemoveGroupMembers", mock.Anything, "group-1", []group.Member{user("drop")}).
			Return(&group.Group{}, (*tidcommon.ServiceError)(nil)).Once()
		mockGroupService.On("AddGroupMembers", mock.Anything, "group-1", []group.Member{user("new")}).
			Return(&group.Group{}, (*tidcommon.ServiceError)(nil)).Once()

		// "new" is listed twice; it must be added once.
		svcErr := service.replaceGroupMembers(context.Background(), "group-1",
			[]group.Member{user("keep"), user("new"), user("new")}, "ReplaceGroup")
		require.Nil(t, svcErr)
	})

	t.Run("NoChangeMakesNoWrites", func(t *testing.T) {
		mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
		service := newSCIMGroupsService(mockGroupService).(*scimGroupsService)

		mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, false).
			Return(&group.MemberListResponse{TotalResults: 1, Members: []group.Member{user("keep")}},
				(*tidcommon.ServiceError)(nil))

		svcErr := service.replaceGroupMembers(context.Background(), "group-1",
			[]group.Member{user("keep")}, "ReplaceGroup")
		require.Nil(t, svcErr)
	})

	t.Run("RemoveFailureSkipsAdd", func(t *testing.T) {
		mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
		service := newSCIMGroupsService(mockGroupService).(*scimGroupsService)

		mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, false).
			Return(&group.MemberListResponse{TotalResults: 1, Members: []group.Member{user("drop")}},
				(*tidcommon.ServiceError)(nil))
		mockGroupService.On("RemoveGroupMembers", mock.Anything, "group-1", []group.Member{user("drop")}).
			Return((*group.Group)(nil), &group.ErrorGroupNotFound)

		svcErr := service.replaceGroupMembers(context.Background(), "group-1",
			[]group.Member{user("new")}, "ReplaceGroup")
		require.Equal(t, scim.ErrorResourceNotFound.Code, svcErr.Code)
	})
}

// TestReplaceGroup_ReadOnly tests Replace Group for Read Only.
func (suite *ServiceTestSuite) TestReplaceGroup_ReadOnly() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Admins", IsReadOnly: true}, (*tidcommon.ServiceError)(nil))

	scimGroup, err := service.ReplaceGroup(context.Background(), "group-1", "New Name", nil, testBaseURL)

	require.Nil(t, scimGroup)
	require.Equal(t, scim.ErrorMutabilityViolation.Code, err.Code)
}

// TestReplaceGroup_NotFound tests Replace Group for Not Found.
func (suite *ServiceTestSuite) TestReplaceGroup_NotFound() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "missing", false).
		Return((*group.Group)(nil), &group.ErrorGroupNotFound)

	scimGroup, err := service.ReplaceGroup(context.Background(), "missing", "New Name", nil, testBaseURL)

	require.Nil(t, scimGroup)
	require.Equal(t, scim.ErrorResourceNotFound.Code, err.Code)
}

// TestReplaceGroup_PreservesGroupOU_WhenCallerOUDiffers guards against a regression where
// UpdateGroupRequest.OUID was populated from the caller's own security-context OU
// (security.GetOUID(ctx)) instead of the target group's existing OUID. The SCIM Group wire
// schema (RFC 7643 §4.2) has no field for a client to specify an OU, so a plain displayName/
// members replace must never change which OU the group lives in. group.Service.UpdateGroup
// treats any OUID different from the group's current one as an explicit OU-move request
// (isOrganizationUnitChanged) and will attempt to move the group if the caller happens to
// pass both the source and target OU access checks — e.g. a caller with access to multiple
// OUs renaming a group that lives in an OU other than their own current context OU. This
// test uses a caller context OU ("ou-caller") that differs from the target group's actual
// OU ("ou-target") and asserts UpdateGroup is invoked with the group's own OUID, not the
// caller's.
// TestReplaceGroup_PreservesGroupOU_WhenCallerOUDiffers tests Replace Group for Preserves Group OU When
// Caller OU Differs.
func (suite *ServiceTestSuite) TestReplaceGroup_PreservesGroupOU_WhenCallerOUDiffers() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Old Name", OUID: "ou-target"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("UpdateGroup", mock.Anything, "group-1",
		group.UpdateGroupRequest{Name: "New Name", OUID: "ou-target"}).
		Return(&group.Group{ID: "group-1", Name: "New Name", OUID: "ou-target"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, false).
		Return(&group.MemberListResponse{}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroup", mock.Anything, "group-1", true).
		Return(&group.Group{ID: "group-1", Name: "New Name", OUID: "ou-target"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return(&group.MemberListResponse{}, (*tidcommon.ServiceError)(nil))

	callerCtx := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("caller-user", "ou-caller", "", nil, nil))

	scimGroup, err := service.ReplaceGroup(callerCtx, "group-1", "New Name", nil, testBaseURL)

	require.Nil(t, err)
	require.Equal(t, "New Name", scimGroup.DisplayName)
}

// TestPatchGroup_DisplayNameReplace tests Patch Group for Display Name Replace.
func (suite *ServiceTestSuite) TestPatchGroup_DisplayNameReplace() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Old"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("UpdateGroup", mock.Anything, "group-1", group.UpdateGroupRequest{Name: "Renamed"}).
		Return(&group.Group{ID: "group-1", Name: "Renamed"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroup", mock.Anything, "group-1", true).
		Return(&group.Group{ID: "group-1", Name: "Renamed"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return(&group.MemberListResponse{}, (*tidcommon.ServiceError)(nil))

	actions := []SCIMGroupPatchAction{
		{Op: scimPatchOpReplace, Target: scimGroupPatchTargetDisplayName, DisplayName: "Renamed"},
	}
	scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, testBaseURL)

	require.Nil(t, err)
	require.Equal(t, "Renamed", scimGroup.DisplayName)
}

// TestPatchGroup_DisplayNamePreservesGroupOU_WhenCallerOUDiffers is the PatchGroup
// counterpart to TestReplaceGroup_PreservesGroupOU_WhenCallerOUDiffers: a displayName-only
// PATCH must not move the group to the caller's own OU just because applyDisplayNamePatch
// builds an UpdateGroupRequest. It asserts UpdateGroup is called with the target group's
// own OUID (threaded in from PatchGroup's initial GetGroup fetch), not
// security.GetOUID(ctx).
// TestPatchGroup_DisplayNamePreservesGroupOU_WhenCallerOUDiffers tests Patch Group for Display Name Preserves
// Group OU When Caller OU Differs.
func (suite *ServiceTestSuite) TestPatchGroup_DisplayNamePreservesGroupOU_WhenCallerOUDiffers() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Old", OUID: "ou-target"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("UpdateGroup", mock.Anything, "group-1",
		group.UpdateGroupRequest{Name: "Renamed", OUID: "ou-target"}).
		Return(&group.Group{ID: "group-1", Name: "Renamed", OUID: "ou-target"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroup", mock.Anything, "group-1", true).
		Return(&group.Group{ID: "group-1", Name: "Renamed", OUID: "ou-target"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return(&group.MemberListResponse{}, (*tidcommon.ServiceError)(nil))

	callerCtx := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("caller-user", "ou-caller", "", nil, nil))

	actions := []SCIMGroupPatchAction{
		{Op: scimPatchOpReplace, Target: scimGroupPatchTargetDisplayName, DisplayName: "Renamed"},
	}
	scimGroup, err := service.PatchGroup(callerCtx, "group-1", actions, testBaseURL)

	require.Nil(t, err)
	require.Equal(t, "Renamed", scimGroup.DisplayName)
}

// TestPatchGroup_AddMembers tests Patch Group for Add Members.
func (suite *ServiceTestSuite) TestPatchGroup_AddMembers() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("AddGroupMembers", mock.Anything, "group-1",
		[]group.Member{{ID: "user-9", Type: group.MemberTypeUser}}).
		Return(&group.Group{}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroup", mock.Anything, "group-1", true).
		Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return(&group.MemberListResponse{Members: []group.Member{{ID: "user-9", Type: group.MemberTypeUser}}},
			(*tidcommon.ServiceError)(nil))

	actions := []SCIMGroupPatchAction{
		{Op: scimPatchOpAdd, Target: scimGroupPatchTargetMembers,
			Members: []SCIMGroupMember{{Value: "user-9", Type: "User"}}},
	}
	scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, testBaseURL)

	require.Nil(t, err)
	require.Len(t, scimGroup.Members, 1)
}

// TestPatchGroup_AddMembers_EmptyIsNoOp tests Patch Group for Add Members Empty Is No Op.
func (suite *ServiceTestSuite) TestPatchGroup_AddMembers_EmptyIsNoOp() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroup", mock.Anything, "group-1", true).
		Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return(&group.MemberListResponse{}, (*tidcommon.ServiceError)(nil))
	// AddGroupMembers deliberately NOT mocked — must not be called for an empty members list.

	actions := []SCIMGroupPatchAction{
		{Op: scimPatchOpAdd, Target: scimGroupPatchTargetMembers, Members: []SCIMGroupMember{}},
	}
	_, err := service.PatchGroup(context.Background(), "group-1", actions, testBaseURL)

	require.Nil(t, err)
	mockGroupService.AssertNotCalled(t, "AddGroupMembers", mock.Anything, mock.Anything, mock.Anything)
}

// TestPatchGroup_RemoveMember_FilteredMatch tests Patch Group for Remove Member Filtered Match.
func (suite *ServiceTestSuite) TestPatchGroup_RemoveMember_FilteredMatch() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return(&group.MemberListResponse{Members: []group.Member{
			{ID: "user-1", Type: group.MemberTypeUser},
			{ID: "user-2", Type: group.MemberTypeUser},
		}}, (*tidcommon.ServiceError)(nil)).Once()
	mockGroupService.On("RemoveGroupMembers", mock.Anything, "group-1",
		[]group.Member{{ID: "user-2", Type: group.MemberTypeUser}}).
		Return(&group.Group{}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroup", mock.Anything, "group-1", true).
		Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return(&group.MemberListResponse{Members: []group.Member{{ID: "user-1", Type: group.MemberTypeUser}}},
			(*tidcommon.ServiceError)(nil)).Once()

	actions := []SCIMGroupPatchAction{
		{Op: scimPatchOpRemove, Target: scimGroupPatchTargetMembers, FilterValue: "user-2"},
	}
	scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, testBaseURL)

	require.Nil(t, err)
	require.Len(t, scimGroup.Members, 1)
	require.Equal(t, "user-1", scimGroup.Members[0].Value)
}

// TestPatchGroup_RemoveMember_FilterNoMatch_NoOp tests Patch Group for Remove Member Filter No Match No Op.
func (suite *ServiceTestSuite) TestPatchGroup_RemoveMember_FilterNoMatch_NoOp() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return(&group.MemberListResponse{Members: []group.Member{{ID: "user-1", Type: group.MemberTypeUser}}},
			(*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroup", mock.Anything, "group-1", true).
		Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
	// RemoveGroupMembers deliberately NOT mocked — a filterValue matching nothing must be a no-op.

	actions := []SCIMGroupPatchAction{
		{Op: scimPatchOpRemove, Target: scimGroupPatchTargetMembers, FilterValue: "ghost-user"},
	}
	_, err := service.PatchGroup(context.Background(), "group-1", actions, testBaseURL)

	require.Nil(t, err)
	mockGroupService.AssertNotCalled(t, "RemoveGroupMembers", mock.Anything, mock.Anything, mock.Anything)
}

// TestPatchGroup_RemoveAllMembers_EmptyFilter tests Patch Group for Remove All Members Empty Filter.
func (suite *ServiceTestSuite) TestPatchGroup_RemoveAllMembers_EmptyFilter() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return(&group.MemberListResponse{Members: []group.Member{
			{ID: "user-1", Type: group.MemberTypeUser},
			{ID: "user-2", Type: group.MemberTypeUser},
		}}, (*tidcommon.ServiceError)(nil)).Once()
	mockGroupService.On("RemoveGroupMembers", mock.Anything, "group-1", []group.Member{
		{ID: "user-1", Type: group.MemberTypeUser},
		{ID: "user-2", Type: group.MemberTypeUser},
	}).Return(&group.Group{}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroup", mock.Anything, "group-1", true).
		Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return(&group.MemberListResponse{}, (*tidcommon.ServiceError)(nil)).Once()

	actions := []SCIMGroupPatchAction{
		{Op: scimPatchOpRemove, Target: scimGroupPatchTargetMembers},
	}
	scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, testBaseURL)

	require.Nil(t, err)
	require.Empty(t, scimGroup.Members)
}

// TestPatchGroup_ReadOnly tests Patch Group for Read Only.
func (suite *ServiceTestSuite) TestPatchGroup_ReadOnly() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Admins", IsReadOnly: true}, (*tidcommon.ServiceError)(nil))

	actions := []SCIMGroupPatchAction{
		{Op: scimPatchOpReplace, Target: scimGroupPatchTargetDisplayName, DisplayName: "New"},
	}
	scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, testBaseURL)

	require.Nil(t, scimGroup)
	require.Equal(t, scim.ErrorMutabilityViolation.Code, err.Code)
}

// TestDeleteGroup_Success tests Delete Group for Success.
func (suite *ServiceTestSuite) TestDeleteGroup_Success() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("DeleteGroup", mock.Anything, "group-1").
		Return((*tidcommon.ServiceError)(nil))

	err := service.DeleteGroup(context.Background(), "group-1")

	require.Nil(t, err)
}

// TestDeleteGroup_ReadOnly tests Delete Group for Read Only.
func (suite *ServiceTestSuite) TestDeleteGroup_ReadOnly() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Admins", IsReadOnly: true}, (*tidcommon.ServiceError)(nil))

	err := service.DeleteGroup(context.Background(), "group-1")

	require.Equal(t, scim.ErrorMutabilityViolation.Code, err.Code)
}

// TestDeleteGroup_NotFound tests Delete Group for Not Found.
func (suite *ServiceTestSuite) TestDeleteGroup_NotFound() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "missing", false).
		Return((*group.Group)(nil), &group.ErrorGroupNotFound)

	err := service.DeleteGroup(context.Background(), "missing")

	require.Equal(t, scim.ErrorResourceNotFound.Code, err.Code)
}

// TestScimMembersToThunderID_GroupTypeCaseInsensitive tests Scim Members To Thunder ID for Group Type Case Insensitive.
func (suite *ServiceTestSuite) TestScimMembersToThunderID_GroupTypeCaseInsensitive() {
	t := suite.T()
	members, err := scimMembersToThunderID([]SCIMGroupMember{
		{Value: "group-2", Type: "GROUP"},
		{Value: "user-1", Type: "user"},
	})

	require.Nil(t, err)
	require.Equal(t, group.MemberTypeGroup, members[0].Type)
	require.Equal(t, group.MemberTypeUser, members[1].Type)
}

// TestScimMembersToThunderID_AppAndAgentTypes tests Scim Members To Thunder ID for App and Agent Types.
func (suite *ServiceTestSuite) TestScimMembersToThunderID_AppAndAgentTypes() {
	t := suite.T()
	members, err := scimMembersToThunderID([]SCIMGroupMember{
		{Value: "app-1", Type: "App"},
		{Value: "agent-1", Type: "agent"},
	})

	require.Nil(t, err)
	require.Equal(t, group.MemberTypeApp, members[0].Type)
	require.Equal(t, group.MemberTypeAgent, members[1].Type)
}

// TestScimMembersToThunderID_OmittedTypeDefaultsToUser tests Scim Members To Thunder ID for Omitted Type
// Defaults To User.
func (suite *ServiceTestSuite) TestScimMembersToThunderID_OmittedTypeDefaultsToUser() {
	t := suite.T()
	members, err := scimMembersToThunderID([]SCIMGroupMember{
		{Value: "user-1", Type: ""},
	})

	require.Nil(t, err)
	require.Equal(t, group.MemberTypeUser, members[0].Type)
}

// TestMapGroupServiceErrorToSCIM tests Map Group Service Error To SCIM.
func (suite *ServiceTestSuite) TestMapGroupServiceErrorToSCIM() {
	t := suite.T()
	tests := []struct {
		name     string
		input    *tidcommon.ServiceError
		wantCode string
	}{
		{"nil passthrough", nil, ""},
		{"not found", &group.ErrorGroupNotFound, scim.ErrorResourceNotFound.Code},
		{"declarative readonly", &group.ErrorImmutableGroup, scim.ErrorMutabilityViolation.Code},
		{"invalid member id", &group.ErrorInvalidMemberID, scim.ErrorInvalidGroupMember.Code},
		{"invalid group member id", &group.ErrorInvalidGroupMemberID, scim.ErrorInvalidGroupMember.Code},
		{"unauthorized passthrough", &tidcommon.ErrorUnauthorized, tidcommon.ErrorUnauthorized.Code},
		{"server error maps to internal", &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "GRP-9999"},
			tidcommon.InternalServerError.Code},
		{"unmapped client error", &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "GRP-9998"},
			scim.ErrorInvalidRequestBody.Code},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapGroupServiceErrorToSCIM(tt.input)
			if tt.input == nil {
				require.Nil(t, got)
				return
			}
			require.Equal(t, tt.wantCode, got.Code)
		})
	}
}

// TestGetGroup_MembersFetchError tests Get Group for Members Fetch Error.
func (suite *ServiceTestSuite) TestGetGroup_MembersFetchError() {
	t := suite.T()
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", true).
		Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return((*group.MemberListResponse)(nil), &group.ErrorGroupNotFound)

	scimGroup, err := service.GetGroup(context.Background(), "group-1", testBaseURL)
	require.Nil(t, scimGroup)
	require.Equal(t, scim.ErrorResourceNotFound.Code, err.Code)
}

// TestReplaceGroup_Errors tests Replace Group for Errors.
func (suite *ServiceTestSuite) TestReplaceGroup_Errors() {
	t := suite.T()
	t.Run("GetGroupError", func(t *testing.T) {
		mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
		service := newSCIMGroupsService(mockGroupService)

		mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
			Return((*group.Group)(nil), &group.ErrorGroupNotFound)

		scimGroup, err := service.ReplaceGroup(context.Background(), "group-1", "New Team", nil, testBaseURL)
		require.Nil(t, scimGroup)
		require.Equal(t, scim.ErrorResourceNotFound.Code, err.Code)
	})

	t.Run("UpdateGroupError", func(t *testing.T) {
		mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
		service := newSCIMGroupsService(mockGroupService)

		mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
			Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
		mockGroupService.On("UpdateGroup", mock.Anything, "group-1", mock.Anything).
			Return((*group.Group)(nil), &group.ErrorGroupNotFound)

		scimGroup, err := service.ReplaceGroup(context.Background(), "group-1", "New Team", nil, testBaseURL)
		require.Nil(t, scimGroup)
		require.Equal(t, scim.ErrorResourceNotFound.Code, err.Code)
	})
}

// TestPatchGroup_ApplyErrors tests Patch Group for Apply Errors.
func (suite *ServiceTestSuite) TestPatchGroup_ApplyErrors() {
	t := suite.T()
	t.Run("InvalidMemberType", func(t *testing.T) {
		mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
		service := newSCIMGroupsService(mockGroupService)

		mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
			Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))

		actions := []SCIMGroupPatchAction{
			{Op: scimPatchOpAdd, Target: scimGroupPatchTargetMembers,
				Members: []SCIMGroupMember{{Value: "u1", Type: "Bogus"}}},
		}
		scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, testBaseURL)
		require.Nil(t, scimGroup)
		require.Equal(t, scim.ErrorUnsupportedMemberType.Code, err.Code)
	})

	t.Run("AddGroupMembersError", func(t *testing.T) {
		mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
		service := newSCIMGroupsService(mockGroupService)

		mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
			Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
		mockGroupService.On("AddGroupMembers", mock.Anything, "group-1", mock.Anything).
			Return((*group.Group)(nil), &group.ErrorGroupNotFound)

		actions := []SCIMGroupPatchAction{
			{Op: scimPatchOpAdd, Target: scimGroupPatchTargetMembers,
				Members: []SCIMGroupMember{{Value: "u1", Type: "User"}}},
		}
		scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, testBaseURL)
		require.Nil(t, scimGroup)
		require.Equal(t, scim.ErrorResourceNotFound.Code, err.Code)
	})
}

// TestPatchGroup_ReplaceOpErrors tests Patch Group for Replace Op Errors.
func (suite *ServiceTestSuite) TestPatchGroup_ReplaceOpErrors() {
	t := suite.T()
	t.Run("InvalidMemberType", func(t *testing.T) {
		mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
		service := newSCIMGroupsService(mockGroupService)

		mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
			Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))

		actions := []SCIMGroupPatchAction{
			{Op: scimPatchOpReplace, Target: scimGroupPatchTargetMembers,
				Members: []SCIMGroupMember{{Value: "u1", Type: "Bogus"}}},
		}
		scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, testBaseURL)
		require.Nil(t, scimGroup)
		require.Equal(t, scim.ErrorUnsupportedMemberType.Code, err.Code)
	})

	t.Run("FetchMembersError", func(t *testing.T) {
		mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
		service := newSCIMGroupsService(mockGroupService)

		mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
			Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
		mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, false).
			Return((*group.MemberListResponse)(nil), &group.ErrorGroupNotFound)

		actions := []SCIMGroupPatchAction{
			{Op: scimPatchOpReplace, Target: scimGroupPatchTargetMembers,
				Members: []SCIMGroupMember{{Value: "u1", Type: "User"}}},
		}
		scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, testBaseURL)
		require.Nil(t, scimGroup)
		require.Equal(t, scim.ErrorResourceNotFound.Code, err.Code)
	})

	t.Run("RemoveMembersError", func(t *testing.T) {
		mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
		service := newSCIMGroupsService(mockGroupService)

		mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
			Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
		mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, false).
			Return(&group.MemberListResponse{
				TotalResults: 1,
				Members:      []group.Member{{ID: "user-1", Type: group.MemberTypeUser}},
			}, (*tidcommon.ServiceError)(nil))
		mockGroupService.On("RemoveGroupMembers", mock.Anything, "group-1", mock.Anything).
			Return((*group.Group)(nil), &group.ErrorGroupNotFound)

		actions := []SCIMGroupPatchAction{
			{Op: scimPatchOpReplace, Target: scimGroupPatchTargetMembers,
				Members: []SCIMGroupMember{{Value: "u1", Type: "User"}}},
		}
		scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, testBaseURL)
		require.Nil(t, scimGroup)
		require.Equal(t, scim.ErrorResourceNotFound.Code, err.Code)
	})

	t.Run("AddMembersError", func(t *testing.T) {
		mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
		service := newSCIMGroupsService(mockGroupService)

		mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
			Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
		mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, false).
			Return(&group.MemberListResponse{}, (*tidcommon.ServiceError)(nil))
		mockGroupService.On("AddGroupMembers", mock.Anything, "group-1", mock.Anything).
			Return((*group.Group)(nil), &group.ErrorGroupNotFound)

		actions := []SCIMGroupPatchAction{
			{Op: scimPatchOpReplace, Target: scimGroupPatchTargetMembers,
				Members: []SCIMGroupMember{{Value: "u1", Type: "User"}}},
		}
		scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, testBaseURL)
		require.Nil(t, scimGroup)
		require.Equal(t, scim.ErrorResourceNotFound.Code, err.Code)
	})
}

// TestPatchGroup_RemoveOpErrors tests Patch Group for Remove Op Errors.
func (suite *ServiceTestSuite) TestPatchGroup_RemoveOpErrors() {
	t := suite.T()
	t.Run("FetchMembersError", func(t *testing.T) {
		mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
		service := newSCIMGroupsService(mockGroupService)

		mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
			Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
		mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
			Return((*group.MemberListResponse)(nil), &group.ErrorGroupNotFound)

		actions := []SCIMGroupPatchAction{
			{Op: scimPatchOpRemove, Target: scimGroupPatchTargetMembers, FilterValue: "user-1"},
		}
		scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, testBaseURL)
		require.Nil(t, scimGroup)
		require.Equal(t, scim.ErrorResourceNotFound.Code, err.Code)
	})

	t.Run("RemoveGroupMembersError", func(t *testing.T) {
		mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
		service := newSCIMGroupsService(mockGroupService)

		mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
			Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
		mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
			Return(&group.MemberListResponse{
				TotalResults: 1,
				Members:      []group.Member{{ID: "user-1", Type: group.MemberTypeUser}},
			}, (*tidcommon.ServiceError)(nil))
		mockGroupService.On("RemoveGroupMembers", mock.Anything, "group-1", mock.Anything).
			Return((*group.Group)(nil), &group.ErrorGroupNotFound)

		actions := []SCIMGroupPatchAction{
			{Op: scimPatchOpRemove, Target: scimGroupPatchTargetMembers, FilterValue: "user-1"},
		}
		scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, testBaseURL)
		require.Nil(t, scimGroup)
		require.Equal(t, scim.ErrorResourceNotFound.Code, err.Code)
	})
}
