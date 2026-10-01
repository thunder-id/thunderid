// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package groups

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/group"
	scim "github.com/thunder-id/thunderid/internal/scim/common"
)

// ResourceTestSuite groups the tests in resource_test.go.
type ResourceTestSuite struct {
	suite.Suite
}

// TestResourceTestSuite runs ResourceTestSuite.
func TestResourceTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceTestSuite))
}

// TestThunderIDMemberTypeToSCIM tests Thunder ID Member Type To SCIM.
func (suite *ResourceTestSuite) TestThunderIDMemberTypeToSCIM() {
	t := suite.T()
	require.Equal(t, "Group", thunderIDMemberTypeToSCIM(group.MemberTypeGroup))
	require.Equal(t, "User", thunderIDMemberTypeToSCIM(group.MemberTypeUser))
	require.Equal(t, "App", thunderIDMemberTypeToSCIM(group.MemberTypeApp))
	require.Equal(t, "Agent", thunderIDMemberTypeToSCIM(group.MemberTypeAgent))
}

// TestBuildSCIMGroupMember tests Build SCIM Group Member.
func (suite *ResourceTestSuite) TestBuildSCIMGroupMember() {
	t := suite.T()
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

	// App and agent members carry no $ref
	for typ, want := range map[group.MemberType]string{group.MemberTypeApp: "App", group.MemberTypeAgent: "Agent"} {
		scimM := buildSCIMGroupMember(group.Member{ID: "entity-789", Type: typ}, baseURL)
		require.Equal(t, "entity-789", scimM.Value)
		require.Equal(t, want, scimM.Type)
		require.Empty(t, scimM.Ref)
	}
}

// TestBuildSCIMGroupListResponse tests Build SCIM Group List Response.
func (suite *ResourceTestSuite) TestBuildSCIMGroupListResponse() {
	t := suite.T()
	// Nil list should map to empty list
	resp := buildSCIMGroupListResponse(nil, 10, 1, 0)
	require.Equal(t, []string{scim.SCIMListResponseSchemaURN}, resp.Schemas)
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
