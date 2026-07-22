package scim

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/group"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/groupmock"
)

// TestGetGroup_Success verifies that GetGroup returns members even though
// group.GroupServiceInterface.GetGroup does not populate Members for
// database-backed groups; the service must fetch them via GetGroupMembers.
func TestGetGroup_Success(t *testing.T) {
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
func TestGetGroup_MembersExceedingPageSize(t *testing.T) {
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

func TestGetGroup_NotFound(t *testing.T) {
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
func TestListGroups_Success(t *testing.T) {
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

// TestCreateGroup_Success verifies that CreateGroup re-fetches the created group
// instead of returning the raw creation result, since group.Service.CreateGroup
// strips member Display when persisting and never resolves it before returning.
func TestCreateGroup_Success(t *testing.T) {
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
func TestCreateGroup_UnsupportedMemberType(t *testing.T) {
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	scimGroup, err := service.CreateGroup(context.Background(), "Engineering Team",
		[]SCIMGroupMember{{Value: "user-1", Type: "anything"}}, testBaseURL)

	require.Nil(t, scimGroup)
	require.Equal(t, ErrorUnsupportedMemberType.Code, err.Code)
}

func TestReplaceGroup_Success(t *testing.T) {
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
		[]SCIMGroupMember{{Value: "new-user", Type: "User"}}, "", testBaseURL)

	require.Nil(t, err)
	require.Equal(t, "New Name", scimGroup.DisplayName)
	require.Len(t, scimGroup.Members, 1)
	require.Equal(t, "new-user", scimGroup.Members[0].Value)
}

func TestReplaceGroup_ReadOnly(t *testing.T) {
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Admins", IsReadOnly: true}, (*tidcommon.ServiceError)(nil))

	scimGroup, err := service.ReplaceGroup(context.Background(), "group-1", "New Name", nil, "", testBaseURL)

	require.Nil(t, scimGroup)
	require.Equal(t, ErrorMutabilityViolation.Code, err.Code)
}

func TestReplaceGroup_NotFound(t *testing.T) {
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "missing", false).
		Return((*group.Group)(nil), &group.ErrorGroupNotFound)

	scimGroup, err := service.ReplaceGroup(context.Background(), "missing", "New Name", nil, "", testBaseURL)

	require.Nil(t, scimGroup)
	require.Equal(t, ErrorResourceNotFound.Code, err.Code)
}

func TestPatchGroup_DisplayNameReplace(t *testing.T) {
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
	scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, "", testBaseURL)

	require.Nil(t, err)
	require.Equal(t, "Renamed", scimGroup.DisplayName)
}

func TestPatchGroup_AddMembers(t *testing.T) {
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
	scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, "", testBaseURL)

	require.Nil(t, err)
	require.Len(t, scimGroup.Members, 1)
}

func TestPatchGroup_AddMembers_EmptyIsNoOp(t *testing.T) {
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
	_, err := service.PatchGroup(context.Background(), "group-1", actions, "", testBaseURL)

	require.Nil(t, err)
	mockGroupService.AssertNotCalled(t, "AddGroupMembers", mock.Anything, mock.Anything, mock.Anything)
}

func TestPatchGroup_RemoveMember_FilteredMatch(t *testing.T) {
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
	scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, "", testBaseURL)

	require.Nil(t, err)
	require.Len(t, scimGroup.Members, 1)
	require.Equal(t, "user-1", scimGroup.Members[0].Value)
}

func TestPatchGroup_RemoveMember_FilterNoMatch_NoOp(t *testing.T) {
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
	_, err := service.PatchGroup(context.Background(), "group-1", actions, "", testBaseURL)

	require.Nil(t, err)
	mockGroupService.AssertNotCalled(t, "RemoveGroupMembers", mock.Anything, mock.Anything, mock.Anything)
}

func TestPatchGroup_RemoveAllMembers_EmptyFilter(t *testing.T) {
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
	scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, "", testBaseURL)

	require.Nil(t, err)
	require.Empty(t, scimGroup.Members)
}

func TestPatchGroup_ReadOnly(t *testing.T) {
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Admins", IsReadOnly: true}, (*tidcommon.ServiceError)(nil))

	actions := []SCIMGroupPatchAction{
		{Op: scimPatchOpReplace, Target: scimGroupPatchTargetDisplayName, DisplayName: "New"},
	}
	scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, "", testBaseURL)

	require.Nil(t, scimGroup)
	require.Equal(t, ErrorMutabilityViolation.Code, err.Code)
}

func TestDeleteGroup_Success(t *testing.T) {
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("DeleteGroup", mock.Anything, "group-1").
		Return((*tidcommon.ServiceError)(nil))

	err := service.DeleteGroup(context.Background(), "group-1", "")

	require.Nil(t, err)
}

func TestDeleteGroup_ReadOnly(t *testing.T) {
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Admins", IsReadOnly: true}, (*tidcommon.ServiceError)(nil))

	err := service.DeleteGroup(context.Background(), "group-1", "")

	require.Equal(t, ErrorMutabilityViolation.Code, err.Code)
}

func TestDeleteGroup_NotFound(t *testing.T) {
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "missing", false).
		Return((*group.Group)(nil), &group.ErrorGroupNotFound)

	err := service.DeleteGroup(context.Background(), "missing", "")

	require.Equal(t, ErrorResourceNotFound.Code, err.Code)
}

func TestScimMembersToThunder_GroupTypeCaseInsensitive(t *testing.T) {
	members, err := scimMembersToThunder([]SCIMGroupMember{
		{Value: "group-2", Type: "GROUP"},
		{Value: "user-1", Type: "user"},
	})

	require.Nil(t, err)
	require.Equal(t, group.MemberTypeGroup, members[0].Type)
	require.Equal(t, group.MemberTypeUser, members[1].Type)
}

func TestMapGroupServiceErrorToSCIM(t *testing.T) {
	tests := []struct {
		name     string
		input    *tidcommon.ServiceError
		wantCode string
	}{
		{"nil passthrough", nil, ""},
		{"not found", &group.ErrorGroupNotFound, ErrorResourceNotFound.Code},
		{"declarative readonly", &group.ErrorImmutableGroup, ErrorMutabilityViolation.Code},
		{"invalid member id", &group.ErrorInvalidMemberID, ErrorInvalidGroupMember.Code},
		{"invalid group member id", &group.ErrorInvalidGroupMemberID, ErrorInvalidGroupMember.Code},
		{"unauthorized passthrough", &tidcommon.ErrorUnauthorized, tidcommon.ErrorUnauthorized.Code},
		{"server error maps to internal", &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "GRP-9999"},
			tidcommon.InternalServerError.Code},
		{"unmapped client error", &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "GRP-9998"},
			ErrorInvalidRequestBody.Code},
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

func TestReplaceGroup_IfMatch_Match(t *testing.T) {
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	existing := &group.Group{ID: "group-1", Name: "Old Name"}
	currentVersion := generateVersion(groupVersionState(*existing))

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(existing, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return(&group.MemberListResponse{}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("UpdateGroup", mock.Anything, "group-1", mock.Anything).
		Return(&group.Group{ID: "group-1", Name: "New Name"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, false).
		Return(&group.MemberListResponse{}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("AddGroupMembers", mock.Anything, "group-1",
		[]group.Member{{ID: "new-user", Type: group.MemberTypeUser}}).
		Return(&group.Group{}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroup", mock.Anything, "group-1", true).
		Return(&group.Group{ID: "group-1", Name: "New Name"}, (*tidcommon.ServiceError)(nil))

	scimGroup, err := service.ReplaceGroup(context.Background(), "group-1", "New Name",
		[]SCIMGroupMember{{Value: "new-user", Type: "User"}}, currentVersion, testBaseURL)

	require.Nil(t, err)
	require.Equal(t, "New Name", scimGroup.DisplayName)
}

func TestReplaceGroup_IfMatch_Mismatch(t *testing.T) {
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Old Name"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return(&group.MemberListResponse{}, (*tidcommon.ServiceError)(nil))

	scimGroup, err := service.ReplaceGroup(context.Background(), "group-1", "New Name",
		nil, `W/"stale-version"`, testBaseURL)

	require.Nil(t, scimGroup)
	require.Equal(t, ErrorPreconditionFailed.Code, err.Code)
	mockGroupService.AssertNotCalled(t, "UpdateGroup", mock.Anything, mock.Anything, mock.Anything)
}

func TestReplaceGroup_IfMatch_Wildcard(t *testing.T) {
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Old Name"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return(&group.MemberListResponse{}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("UpdateGroup", mock.Anything, "group-1", mock.Anything).
		Return(&group.Group{ID: "group-1", Name: "New Name"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, false).
		Return(&group.MemberListResponse{}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroup", mock.Anything, "group-1", true).
		Return(&group.Group{ID: "group-1", Name: "New Name"}, (*tidcommon.ServiceError)(nil))

	scimGroup, err := service.ReplaceGroup(context.Background(), "group-1", "New Name", nil, "*", testBaseURL)

	require.Nil(t, err)
	require.Equal(t, "New Name", scimGroup.DisplayName)
}

func TestPatchGroup_IfMatch_Mismatch(t *testing.T) {
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return(&group.MemberListResponse{}, (*tidcommon.ServiceError)(nil))

	actions := []SCIMGroupPatchAction{
		{Op: scimPatchOpReplace, Target: scimGroupPatchTargetDisplayName, DisplayName: "New"},
	}
	scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, `W/"stale"`, testBaseURL)

	require.Nil(t, scimGroup)
	require.Equal(t, ErrorPreconditionFailed.Code, err.Code)
	mockGroupService.AssertNotCalled(t, "UpdateGroup", mock.Anything, mock.Anything, mock.Anything)
}

func TestDeleteGroup_IfMatch_Match(t *testing.T) {
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	existing := &group.Group{ID: "group-1", Name: "Team"}
	currentVersion := generateVersion(groupVersionState(*existing))

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(existing, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return(&group.MemberListResponse{}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("DeleteGroup", mock.Anything, "group-1").Return((*tidcommon.ServiceError)(nil))

	err := service.DeleteGroup(context.Background(), "group-1", currentVersion)

	require.Nil(t, err)
}

func TestDeleteGroup_IfMatch_Mismatch(t *testing.T) {
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return(&group.MemberListResponse{}, (*tidcommon.ServiceError)(nil))

	err := service.DeleteGroup(context.Background(), "group-1", `W/"stale"`)

	require.Equal(t, ErrorPreconditionFailed.Code, err.Code)
	mockGroupService.AssertNotCalled(t, "DeleteGroup", mock.Anything, mock.Anything)
}

func TestGetGroup_MembersFetchError(t *testing.T) {
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", true).
		Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return((*group.MemberListResponse)(nil), &group.ErrorGroupNotFound)

	scimGroup, err := service.GetGroup(context.Background(), "group-1", testBaseURL)
	require.Nil(t, scimGroup)
	require.Equal(t, ErrorResourceNotFound.Code, err.Code)
}

func TestReplaceGroup_Errors(t *testing.T) {
	t.Run("GetGroupError", func(t *testing.T) {
		mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
		service := newSCIMGroupsService(mockGroupService)

		mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
			Return((*group.Group)(nil), &group.ErrorGroupNotFound)

		scimGroup, err := service.ReplaceGroup(context.Background(), "group-1", "New Team", nil, "", testBaseURL)
		require.Nil(t, scimGroup)
		require.Equal(t, ErrorResourceNotFound.Code, err.Code)
	})

	t.Run("UpdateGroupError", func(t *testing.T) {
		mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
		service := newSCIMGroupsService(mockGroupService)

		mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
			Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
		mockGroupService.On("UpdateGroup", mock.Anything, "group-1", mock.Anything).
			Return((*group.Group)(nil), &group.ErrorGroupNotFound)

		scimGroup, err := service.ReplaceGroup(context.Background(), "group-1", "New Team", nil, "", testBaseURL)
		require.Nil(t, scimGroup)
		require.Equal(t, ErrorResourceNotFound.Code, err.Code)
	})
}

func TestReplaceGroup_IfMatch_MembersFetchError(t *testing.T) {
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return((*group.MemberListResponse)(nil), &group.ErrorGroupNotFound)

	scimGroup, err := service.ReplaceGroup(context.Background(), "group-1", "New Team", nil, `W/"v1"`, testBaseURL)
	require.Nil(t, scimGroup)
	require.Equal(t, ErrorResourceNotFound.Code, err.Code)
}

func TestPatchGroup_IfMatch_MembersFetchError(t *testing.T) {
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return((*group.MemberListResponse)(nil), &group.ErrorGroupNotFound)

	actions := []SCIMGroupPatchAction{
		{Op: scimPatchOpReplace, Target: scimGroupPatchTargetDisplayName, DisplayName: "New"},
	}
	scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, `W/"v1"`, testBaseURL)
	require.Nil(t, scimGroup)
	require.Equal(t, ErrorResourceNotFound.Code, err.Code)
}

func TestPatchGroup_ApplyErrors(t *testing.T) {
	t.Run("InvalidMemberType", func(t *testing.T) {
		mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
		service := newSCIMGroupsService(mockGroupService)

		mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
			Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))

		actions := []SCIMGroupPatchAction{
			{Op: scimPatchOpAdd, Target: scimGroupPatchTargetMembers,
				Members: []SCIMGroupMember{{Value: "u1", Type: "Bogus"}}},
		}
		scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, "", testBaseURL)
		require.Nil(t, scimGroup)
		require.Equal(t, ErrorUnsupportedMemberType.Code, err.Code)
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
		scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, "", testBaseURL)
		require.Nil(t, scimGroup)
		require.Equal(t, ErrorResourceNotFound.Code, err.Code)
	})
}

func TestDeleteGroup_IfMatch_MembersFetchError(t *testing.T) {
	mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
	service := newSCIMGroupsService(mockGroupService)

	mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
		Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
	mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
		Return((*group.MemberListResponse)(nil), &group.ErrorGroupNotFound)

	err := service.DeleteGroup(context.Background(), "group-1", `W/"v1"`)
	require.Equal(t, ErrorResourceNotFound.Code, err.Code)
}

func TestPatchGroup_ReplaceOpErrors(t *testing.T) {
	t.Run("InvalidMemberType", func(t *testing.T) {
		mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
		service := newSCIMGroupsService(mockGroupService)

		mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
			Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))

		actions := []SCIMGroupPatchAction{
			{Op: scimPatchOpReplace, Target: scimGroupPatchTargetMembers,
				Members: []SCIMGroupMember{{Value: "u1", Type: "Bogus"}}},
		}
		scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, "", testBaseURL)
		require.Nil(t, scimGroup)
		require.Equal(t, ErrorUnsupportedMemberType.Code, err.Code)
	})

	t.Run("FetchMembersError", func(t *testing.T) {
		mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
		service := newSCIMGroupsService(mockGroupService)

		mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
			Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
		mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
			Return((*group.MemberListResponse)(nil), &group.ErrorGroupNotFound)

		actions := []SCIMGroupPatchAction{
			{Op: scimPatchOpReplace, Target: scimGroupPatchTargetMembers,
				Members: []SCIMGroupMember{{Value: "u1", Type: "User"}}},
		}
		scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, "", testBaseURL)
		require.Nil(t, scimGroup)
		require.Equal(t, ErrorResourceNotFound.Code, err.Code)
	})

	t.Run("RemoveMembersError", func(t *testing.T) {
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
			{Op: scimPatchOpReplace, Target: scimGroupPatchTargetMembers,
				Members: []SCIMGroupMember{{Value: "u1", Type: "User"}}},
		}
		scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, "", testBaseURL)
		require.Nil(t, scimGroup)
		require.Equal(t, ErrorResourceNotFound.Code, err.Code)
	})

	t.Run("AddMembersError", func(t *testing.T) {
		mockGroupService := groupmock.NewGroupServiceInterfaceMock(t)
		service := newSCIMGroupsService(mockGroupService)

		mockGroupService.On("GetGroup", mock.Anything, "group-1", false).
			Return(&group.Group{ID: "group-1", Name: "Team"}, (*tidcommon.ServiceError)(nil))
		mockGroupService.On("GetGroupMembers", mock.Anything, "group-1", serverconst.MaxPageSize, 0, true).
			Return(&group.MemberListResponse{}, (*tidcommon.ServiceError)(nil))
		mockGroupService.On("AddGroupMembers", mock.Anything, "group-1", mock.Anything).
			Return((*group.Group)(nil), &group.ErrorGroupNotFound)

		actions := []SCIMGroupPatchAction{
			{Op: scimPatchOpReplace, Target: scimGroupPatchTargetMembers,
				Members: []SCIMGroupMember{{Value: "u1", Type: "User"}}},
		}
		scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, "", testBaseURL)
		require.Nil(t, scimGroup)
		require.Equal(t, ErrorResourceNotFound.Code, err.Code)
	})
}

func TestPatchGroup_RemoveOpErrors(t *testing.T) {
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
		scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, "", testBaseURL)
		require.Nil(t, scimGroup)
		require.Equal(t, ErrorResourceNotFound.Code, err.Code)
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
		scimGroup, err := service.PatchGroup(context.Background(), "group-1", actions, "", testBaseURL)
		require.Nil(t, scimGroup)
		require.Equal(t, ErrorResourceNotFound.Code, err.Code)
	})
}
