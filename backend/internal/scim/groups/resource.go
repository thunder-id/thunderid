// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package groups

import (
	"fmt"

	"github.com/thunder-id/thunderid/internal/group"
	scim "github.com/thunder-id/thunderid/internal/scim/common"
)

// thunderIDMemberTypeToSCIM maps ThunderID member types to SCIM member types.
func thunderIDMemberTypeToSCIM(t group.MemberType) string {
	switch t {
	case group.MemberTypeGroup:
		return "Group"
	case group.MemberTypeApp:
		return "App"
	case group.MemberTypeAgent:
		return "Agent"
	}
	return "User"
}

// buildSCIMGroupMember converts a ThunderID Member to a SCIMGroupMember. App and agent members
// carry no $ref because SCIM exposes no endpoint for them.
func buildSCIMGroupMember(m group.Member, baseURL string) SCIMGroupMember {
	scimType := thunderIDMemberTypeToSCIM(m.Type)
	var ref string
	switch m.Type {
	case group.MemberTypeGroup:
		ref = fmt.Sprintf("%s%s/Groups/%s", baseURL, scim.SCIMBasePath, m.ID)
	case group.MemberTypeApp, group.MemberTypeAgent:
	default:
		ref = fmt.Sprintf("%s%s/Users/%s", baseURL, scim.SCIMBasePath, m.ID)
	}
	return SCIMGroupMember{
		Value:   m.ID,
		Ref:     ref,
		Display: m.Display,
		Type:    scimType,
	}
}

// buildSCIMGroupResource converts a ThunderID group.Group into a SCIMGroup wire response.
func buildSCIMGroupResource(g group.Group, baseURL string) SCIMGroup {
	location := fmt.Sprintf("%s%s/Groups/%s", baseURL, scim.SCIMBasePath, g.ID)
	members := make([]SCIMGroupMember, 0, len(g.Members))
	for _, m := range g.Members {
		members = append(members, buildSCIMGroupMember(m, baseURL))
	}
	return SCIMGroup{
		ID:          g.ID,
		Schemas:     []string{scim.SCIMCoreGroupSchemaURN},
		DisplayName: g.Name,
		Members:     members,
		Meta: scim.SCIMMeta{
			ResourceType: "Group",
			Location:     location,
		},
	}
}

// buildSCIMGroupListResponse wraps a slice of SCIMGroup into the ListResponse envelope.
func buildSCIMGroupListResponse(groups []SCIMGroup, totalResults, startIndex, itemsPerPage int) SCIMGroupListResponse {
	if groups == nil {
		groups = []SCIMGroup{}
	}
	return SCIMGroupListResponse{
		Schemas:      []string{scim.SCIMListResponseSchemaURN},
		TotalResults: totalResults,
		StartIndex:   startIndex,
		ItemsPerPage: itemsPerPage,
		Resources:    groups,
	}
}
