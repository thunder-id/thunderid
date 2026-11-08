// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

// systemFlowGroupDeletePre is the system flow a group deletion runs before the group is removed.
const systemFlowGroupDeletePre = "system-group-delete-pre"

// groupMembershipRemovalFlowKey is the server configuration key of the administration flow that removes a
// group member.
const groupMembershipRemovalFlowKey = "groupMembershipRemovalFlow"

// Inputs the group membership removal flow reads.
const (
	flowInputGroup  = "targetGroupId"
	flowInputMember = "targetMemberId"
)

// Group MCP tool defaults.
const (
	// groupDetailMembers is the section get_group adds only when asked.
	groupDetailMembers = "members"
	// defaultOUHandle is the handle of the organization unit a group is created in when none is named.
	defaultOUHandle = "default"
	// defaultGroupPageSize is the number of groups a list returns when no limit is given.
	defaultGroupPageSize = 30
)
