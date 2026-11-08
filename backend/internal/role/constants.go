// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

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
