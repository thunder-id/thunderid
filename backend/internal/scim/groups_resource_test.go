// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/group"
)

// TestThunderIDMemberTypeToSCIM tests Thunder ID Member Type To SCIM.
func TestThunderIDMemberTypeToSCIM(t *testing.T) {
	require.Equal(t, "Group", thunderIDMemberTypeToSCIM(group.MemberTypeGroup))
	require.Equal(t, "User", thunderIDMemberTypeToSCIM(group.MemberTypeUser))
	require.Equal(t, "User", thunderIDMemberTypeToSCIM("app"))
}

// TestBuildSCIMGroupMember tests Build SCIM Group Member.
func TestBuildSCIMGroupMember(t *testing.T) {
	baseURL := testAPIBaseURL

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

// TestBuildSCIMGroupListResponse tests Build SCIM Group List Response.
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

// TestGroupVersionState tests Group Version State.
func TestGroupVersionState(t *testing.T) {
	t.Run("deterministic regardless of member order", func(t *testing.T) {
		g1 := group.Group{
			Name: "Engineering",
			Members: []group.Member{
				{ID: "user-2", Type: group.MemberTypeUser, Display: "Bob"},
				{ID: "user-1", Type: group.MemberTypeUser, Display: "Alice"},
				{ID: "group-1", Type: group.MemberTypeGroup, Display: "Subgroup"},
			},
		}
		g2 := group.Group{
			Name: "Engineering",
			Members: []group.Member{
				{ID: "group-1", Type: group.MemberTypeGroup, Display: "Subgroup"},
				{ID: "user-1", Type: group.MemberTypeUser, Display: "Alice"},
				{ID: "user-2", Type: group.MemberTypeUser, Display: "Bob"},
			},
		}
		require.Equal(t, groupVersionState(g1), groupVersionState(g2))
		require.Equal(t, generateVersion(groupVersionState(g1)), generateVersion(groupVersionState(g2)))
	})

	t.Run("excludes member display names", func(t *testing.T) {
		g1 := group.Group{
			Name: "Engineering",
			Members: []group.Member{
				{ID: "user-1", Type: group.MemberTypeUser, Display: "Alice Smith"},
				{ID: "user-2", Type: group.MemberTypeUser, Display: "Bob Jones"},
			},
		}
		g2 := group.Group{
			Name: "Engineering",
			Members: []group.Member{
				{ID: "user-1", Type: group.MemberTypeUser, Display: "Alice"},
				{ID: "user-2", Type: group.MemberTypeUser, Display: ""},
			},
		}
		require.Equal(t, groupVersionState(g1), groupVersionState(g2))
		require.Equal(t, generateVersion(groupVersionState(g1)), generateVersion(groupVersionState(g2)))
	})

	t.Run("group name change changes version state", func(t *testing.T) {
		g1 := group.Group{
			Name: "Engineering",
			Members: []group.Member{
				{ID: "user-1", Type: group.MemberTypeUser},
			},
		}
		g2 := group.Group{
			Name: "Platform Engineering",
			Members: []group.Member{
				{ID: "user-1", Type: group.MemberTypeUser},
			},
		}
		require.NotEqual(t, groupVersionState(g1), groupVersionState(g2))
		require.NotEqual(t, generateVersion(groupVersionState(g1)), generateVersion(groupVersionState(g2)))
	})

	t.Run("member type difference changes version state", func(t *testing.T) {
		g1 := group.Group{
			Name: "Engineering",
			Members: []group.Member{
				{ID: "id-1", Type: group.MemberTypeUser},
			},
		}
		g2 := group.Group{
			Name: "Engineering",
			Members: []group.Member{
				{ID: "id-1", Type: group.MemberTypeGroup},
			},
		}
		require.NotEqual(t, groupVersionState(g1), groupVersionState(g2))
		require.NotEqual(t, generateVersion(groupVersionState(g1)), generateVersion(groupVersionState(g2)))
	})

	t.Run("deterministic tie-breaking by member type", func(t *testing.T) {
		g1 := group.Group{
			Name: "Engineering",
			Members: []group.Member{
				{ID: "same-id", Type: group.MemberTypeUser},
				{ID: "same-id", Type: group.MemberTypeGroup},
			},
		}
		g2 := group.Group{
			Name: "Engineering",
			Members: []group.Member{
				{ID: "same-id", Type: group.MemberTypeGroup},
				{ID: "same-id", Type: group.MemberTypeUser},
			},
		}
		require.Equal(t, groupVersionState(g1), groupVersionState(g2))
		require.Equal(t, generateVersion(groupVersionState(g1)), generateVersion(groupVersionState(g2)))
	})

	t.Run("empty and nil members produce equivalent state", func(t *testing.T) {
		gNil := group.Group{Name: "Empty Group", Members: nil}
		gEmpty := group.Group{Name: "Empty Group", Members: []group.Member{}}
		require.Equal(t, groupVersionState(gNil), groupVersionState(gEmpty))
		require.Equal(t, generateVersion(groupVersionState(gNil)), generateVersion(groupVersionState(gEmpty)))
	})
}
