package scim

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/group"
)

func TestThunderMemberTypeToSCIM(t *testing.T) {
	require.Equal(t, "Group", thunderMemberTypeToSCIM(group.MemberTypeGroup))
	require.Equal(t, "User", thunderMemberTypeToSCIM(group.MemberTypeUser))
	require.Equal(t, "User", thunderMemberTypeToSCIM("app"))
}

func TestBuildSCIMGroupMember(t *testing.T) {
	baseURL := "https://api.example.com"

	// Group type member
	mGroup := group.Member{
		ID:      "group-123",
		Type:    group.MemberTypeGroup,
		Display: "Subgroup",
	}
	scimMGroup := buildSCIMGroupMember(mGroup, baseURL)
	require.Equal(t, "group-123", scimMGroup.Value)
	require.Equal(t, "https://api.example.com/scim/v2/Groups/group-123", scimMGroup.Ref)
	require.Equal(t, "Subgroup", scimMGroup.Display)
	require.Equal(t, "Group", scimMGroup.Type)

	// User type member
	mUser := group.Member{
		ID:      "user-456",
		Type:    group.MemberTypeUser,
		Display: "John Doe",
	}
	scimMUser := buildSCIMGroupMember(mUser, baseURL)
	require.Equal(t, "user-456", scimMUser.Value)
	require.Equal(t, "https://api.example.com/scim/v2/Users/user-456", scimMUser.Ref)
	require.Equal(t, "John Doe", scimMUser.Display)
	require.Equal(t, "User", scimMUser.Type)
}

func TestBuildSCIMGroupListResponse(t *testing.T) {
	// Nil list should map to empty list
	resp := buildSCIMGroupListResponse(nil, 10, 1, 0)
	require.Equal(t, []string{SCIMListResponseSchemaURN}, resp.Schemas)
	require.Equal(t, 10, resp.TotalResults)
	require.Equal(t, 1, resp.StartIndex)
	require.Equal(t, 0, resp.ItemsPerPage)
	require.NotNil(t, resp.Resources)
	require.Empty(t, resp.Resources)

	// Non-nil list
	groups := []SCIMGroup{
		{ID: "group-1"},
	}
	resp2 := buildSCIMGroupListResponse(groups, 1, 1, 1)
	require.Len(t, resp2.Resources, 1)
	require.Equal(t, "group-1", resp2.Resources[0].ID)
}
