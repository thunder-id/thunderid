// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

// systemFlowGroupDeletePre is the system flow a group deletion runs before the group is removed.
const systemFlowGroupDeletePre = "system-group-delete-pre"

// The system flow removing a group member runs before the removal is written, and the input naming the
// member it acts on.
const (
	systemFlowGroupMembershipRemovalPre = "system-group-membership-removal-pre"
	systemFlowInputMember               = "targetMemberId"
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
