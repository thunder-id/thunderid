// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func viewDocument(t *testing.T, content string) *yaml.Node {
	t.Helper()
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(content), &root))
	return root.Content[0]
}

func exportedGroup(t *testing.T, members []Member) *yaml.Node {
	t.Helper()
	exported, err := yaml.Marshal(&groupDeclarativeResource{
		ID: "group-1", Name: "Admins", Description: "var:GROUP_ADMINS_DESCRIPTION", OUID: "ou-1",
		Members: members,
	})
	require.NoError(t, err)
	return viewDocument(t, "resource_type: group\n"+string(exported))
}

// An exported group is shown as a read returns it, with its references as they are and no members.
func TestViewResourceShowsAnExportedGroupAsItsRead(t *testing.T) {
	view, err := newGroupExporter(nil).ViewResource(context.Background(),
		exportedGroup(t, []Member{{ID: "user-1", Type: MemberTypeUser}}))

	require.NoError(t, err)
	grp, ok := view.(*Group)
	require.True(t, ok)
	assert.Equal(t, "group-1", grp.ID)
	assert.Equal(t, "Admins", grp.Name)
	assert.Equal(t, "var:GROUP_ADMINS_DESCRIPTION", grp.Description)
	assert.Equal(t, "ou-1", grp.OUID)
	assert.Empty(t, grp.Members)
}

// A group's one part, its members, is shown as one page holding every member the export carries.
func TestViewResourcePartsShowsAnExportedGroupsMembers(t *testing.T) {
	members := []Member{{ID: "user-1", Type: MemberTypeUser}, {ID: "group-2", Type: MemberTypeGroup}}

	parts, err := newGroupExporter(nil).ViewResourceParts(context.Background(), exportedGroup(t, members))

	require.NoError(t, err)
	require.Len(t, parts, 1)
	page, ok := parts["members"].(*MemberListResponse)
	require.True(t, ok)
	assert.Equal(t, 2, page.TotalResults)
	assert.Equal(t, 1, page.StartIndex)
	assert.Equal(t, 2, page.Count)
	assert.Equal(t, members, page.Members)
	assert.Empty(t, page.Links)

	parts, err = newGroupExporter(nil).ViewResourceParts(context.Background(), exportedGroup(t, nil))

	require.NoError(t, err)
	page, ok = parts["members"].(*MemberListResponse)
	require.True(t, ok)
	assert.Equal(t, 0, page.TotalResults)
	assert.NotNil(t, page.Members)
	assert.Empty(t, page.Members)
}

// A document that is not a group is refused.
func TestViewResourceRefusesAnUnreadableDocument(t *testing.T) {
	exporter := newGroupExporter(nil)

	_, err := exporter.ViewResource(context.Background(), viewDocument(t, "name: [not, a, name]"))
	assert.Error(t, err)

	_, err = exporter.ViewResourceParts(context.Background(), viewDocument(t, "members: not-a-list"))
	assert.Error(t, err)
}
