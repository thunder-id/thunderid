// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

// The system flow removing an assignment runs, by handle, before the removal is written, and the inputs
// naming the assignee and the organization unit the assignment was made in. The revocation post flow
// follows it once the removal is written.
const (
	systemFlowRoleAssignmentRemovalPre = "system-role-assignment-removal-pre"
	systemFlowInputAssignee            = "targetAssigneeId"
	systemFlowInputAssignmentOU        = "targetAssignmentOuId"
)
