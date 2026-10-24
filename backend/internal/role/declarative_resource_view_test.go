// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/utils"
)

func viewDocument(t *testing.T, content string) *yaml.Node {
	t.Helper()
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(content), &root))
	return root.Content[0]
}

func exportedRoleDocument(t *testing.T) *yaml.Node {
	t.Helper()
	exported, err := yaml.Marshal(&roleDeclarativeResource{
		ID: "role-1", Name: "Admins", Description: "var:ROLE_ADMINS_DESCRIPTION", OUID: "ou-1",
		Permissions: []roleDeclarativePermission{{ResourceServerID: "rs-1", Permissions: []string{"read"}}},
		Assignments: []RoleAssignment{
			{ID: "user-1", Type: AssigneeTypeUser},
			{ID: "group-1", Type: AssigneeTypeGroup},
		},
	})
	require.NoError(t, err)
	return viewDocument(t, "resource_type: role\n"+string(exported))
}

// An exported role is shown as a read returns it, with its references as they are.
func TestViewResourceShowsAnExportedRoleAsItsRead(t *testing.T) {
	view, err := newRoleExporter(nil, nil).ViewResource(context.Background(), exportedRoleDocument(t))

	require.NoError(t, err)
	assert.Equal(t, &RoleResponse{
		ID: "role-1", Name: "Admins", Description: "var:ROLE_ADMINS_DESCRIPTION", OUID: "ou-1",
		Permissions: []ResourcePermissions{{ResourceServerID: "rs-1", Permissions: []string{"read"}}},
	}, view)
}

// A role's one part, its assignments, is shown as a read of them returns them, all on one page.
func TestViewResourcePartsShowsTheAssignmentsOfAnExportedRole(t *testing.T) {
	parts, err := newRoleExporter(nil, nil).ViewResourceParts(context.Background(), exportedRoleDocument(t))

	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"assignments": &AssignmentListResponse{
		TotalResults: 2, StartIndex: 1, Count: 2,
		Assignments: []AssignmentResponse{
			{ID: "user-1", Type: AssigneeTypeUser},
			{ID: "group-1", Type: AssigneeTypeGroup},
		},
		Links: []utils.Link{},
	}}, parts)
}

// A role without assignments shows an empty page of them.
func TestViewResourcePartsShowsNoAssignmentsAsAnEmptyPage(t *testing.T) {
	parts, err := newRoleExporter(nil, nil).ViewResourceParts(context.Background(),
		viewDocument(t, "id: role-1\nname: Admins\npermissions: []"))

	require.NoError(t, err)
	list, ok := parts["assignments"].(*AssignmentListResponse)
	require.True(t, ok)
	assert.Equal(t, 0, list.TotalResults)
	assert.Equal(t, []AssignmentResponse{}, list.Assignments)
}

// A document that is not a role is refused.
func TestViewResourceRefusesAnUnreadableDocument(t *testing.T) {
	exporter := newRoleExporter(nil, nil)

	_, err := exporter.ViewResource(context.Background(), viewDocument(t, "name: [not, a, name]"))
	assert.Error(t, err)

	_, err = exporter.ViewResourceParts(context.Background(), viewDocument(t, "assignments: not-a-list"))
	assert.Error(t, err)
}
