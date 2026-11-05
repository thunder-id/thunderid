// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

// Role MCP tool defaults.
const (
	// roleDetailHolders is the section get_role adds only when asked.
	roleDetailHolders = "holders"
	// defaultOUHandle is the handle of the organization unit a role is created in when none is named.
	defaultOUHandle = "default"
	// defaultRolePageSize is the number of roles a list returns when no limit is given.
	defaultRolePageSize = 30
)

// systemFlowRoleDeletePre is the system flow a role deletion runs before the role is removed.
const systemFlowRoleDeletePre = "system-role-delete-pre"

// projectIDQueryParam is the query parameter that lists the roles of one project.
const projectIDQueryParam = "projectId"

// The role assignment removal pre flow handle and its assignee and organization unit inputs.
const (
	systemFlowRoleAssignmentRemovalPre = "system-role-assignment-removal-pre"
	systemFlowInputAssignee            = "targetAssigneeId"
	systemFlowInputAssignmentOU        = "targetAssignmentOuId"
)

// maxRemovedAssignments is the most assignments a single removal request may name.
const maxRemovedAssignments = 100

// The system flow a role update runs before it drops a permission, and the input carrying the role's new
// permission set.
const (
	systemFlowRolePermissionRemovalPre = "system-role-permission-removal-pre"
	systemFlowInputPermissions         = "targetPermissions"
)
