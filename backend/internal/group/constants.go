// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

// The system flow removing a group member runs before the removal is written, and the input naming the
// member it acts on.
const (
	systemFlowGroupMembershipRemovalPre = "system-group-membership-removal-pre"
	systemFlowInputMember               = "targetMemberId"
)
