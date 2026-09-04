// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package groups

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/thunder-id/thunderid/internal/group"
	scim "github.com/thunder-id/thunderid/internal/scim/common"
)

// thunderIDMemberTypeToSCIM maps ThunderID member types to SCIM member types.
// ThunderID: "user"/"app"/"agent" → SCIM "User"; ThunderID "group" → SCIM "Group".
func thunderIDMemberTypeToSCIM(t group.MemberType) string {
	if t == group.MemberTypeGroup {
		return "Group"
	}
	return "User"
}

// buildSCIMGroupMember converts a ThunderID Member to a SCIMGroupMember.
func buildSCIMGroupMember(m group.Member, baseURL string) SCIMGroupMember {
	scimType := thunderIDMemberTypeToSCIM(m.Type)
	var ref string
	if m.Type == group.MemberTypeGroup {
		ref = fmt.Sprintf("%s%s/Groups/%s", baseURL, scim.SCIMBasePath, m.ID)
	} else {
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
			Version:      scim.GenerateVersion(groupVersionState(g)),
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

// groupMemberIdentity captures the canonical identity of a group member for ETag computation.
type groupMemberIdentity struct {
	ID   string           `json:"id"`
	Type group.MemberType `json:"type"`
}

// groupVersionState extracts the state of a group that determines its ETag version.
// The ETag covers the group's DisplayName and canonical member identities sorted deterministically.
func groupVersionState(g group.Group) any {
	members := make([]groupMemberIdentity, 0, len(g.Members))
	for _, m := range g.Members {
		members = append(members, groupMemberIdentity{
			ID:   m.ID,
			Type: m.Type,
		})
	}
	slices.SortFunc(members, func(a, b groupMemberIdentity) int {
		if a.ID != b.ID {
			return cmp.Compare(a.ID, b.ID)
		}
		return cmp.Compare(a.Type, b.Type)
	})
	return struct {
		DisplayName string                `json:"displayName"`
		Members     []groupMemberIdentity `json:"members"`
	}{
		DisplayName: g.Name,
		Members:     members,
	}
}
