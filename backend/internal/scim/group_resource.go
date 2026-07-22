package scim

import (
	"fmt"

	"github.com/thunder-id/thunderid/internal/group"
)

// thunderMemberTypeToSCIM maps Thunder member types to SCIM member types.
// Thunder: "user"/"app"/"agent" → SCIM "User"; Thunder "group" → SCIM "Group".
func thunderMemberTypeToSCIM(t group.MemberType) string {
	if t == group.MemberTypeGroup {
		return "Group"
	}
	return "User"
}

// buildSCIMGroupMember converts a Thunder Member to a SCIMGroupMember.
func buildSCIMGroupMember(m group.Member, baseURL string) SCIMGroupMember {
	scimType := thunderMemberTypeToSCIM(m.Type)
	var ref string
	if m.Type == group.MemberTypeGroup {
		ref = fmt.Sprintf("%s%s/Groups/%s", baseURL, SCIMBasePath, m.ID)
	} else {
		ref = fmt.Sprintf("%s%s/Users/%s", baseURL, SCIMBasePath, m.ID)
	}
	return SCIMGroupMember{
		Value:   m.ID,
		Ref:     ref,
		Display: m.Display,
		Type:    scimType,
	}
}

// buildSCIMGroupResource converts a Thunder group.Group into a SCIMGroup wire response.
func buildSCIMGroupResource(g group.Group, baseURL string) SCIMGroup {
	location := fmt.Sprintf("%s%s/Groups/%s", baseURL, SCIMBasePath, g.ID)
	members := make([]SCIMGroupMember, 0, len(g.Members))
	for _, m := range g.Members {
		members = append(members, buildSCIMGroupMember(m, baseURL))
	}
	return SCIMGroup{
		ID:          g.ID,
		Schemas:     []string{SCIMCoreGroupSchemaURN},
		DisplayName: g.Name,
		Members:     members,
		Meta: SCIMMeta{
			ResourceType: "Group",
			Location:     location,
			Version:      generateVersion(groupVersionState(g)),
		},
	}
}

// buildSCIMGroupListResponse wraps a slice of SCIMGroup into the ListResponse envelope.
func buildSCIMGroupListResponse(groups []SCIMGroup, totalResults, startIndex, itemsPerPage int) SCIMGroupListResponse {
	if groups == nil {
		groups = []SCIMGroup{}
	}
	return SCIMGroupListResponse{
		Schemas:      []string{SCIMListResponseSchemaURN},
		TotalResults: totalResults,
		StartIndex:   startIndex,
		ItemsPerPage: itemsPerPage,
		Resources:    groups,
	}
}

func groupVersionState(g group.Group) any {
	return struct {
		DisplayName string
		Members     []group.Member
	}{
		DisplayName: g.Name,
		Members:     g.Members,
	}
}
