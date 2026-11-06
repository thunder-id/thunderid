// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

// Server configuration keys of the administration flows the role MCP tools run.
const (
	roleAssignmentRemovalFlowKey = "roleAssignmentRemovalFlow"
	rolePermissionRemovalFlowKey = "rolePermissionRemovalFlow"
)

// Inputs the role administration flows read.
const (
	flowInputRole         = "targetRoleId"
	flowInputAssignee     = "targetAssigneeId"
	flowInputAssignmentOU = "targetAssignmentOuId"
	flowInputPermissions  = "targetPermissions"
)

// Role MCP tool defaults.
const (
	// roleDetailHolders is the section get_role adds only when asked.
	roleDetailHolders = "holders"
	// defaultOUHandle is the handle of the organization unit a role is created in when none is named.
	defaultOUHandle = "default"
	// defaultRolePageSize is the number of roles a list returns when no limit is given.
	defaultRolePageSize = 30
)
