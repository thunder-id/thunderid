// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

// The system flows this service runs, by handle, before a change is written and after, and the inputs
// naming the action they act on.
const (
	systemFlowActionDeletePre = "system-action-delete-pre"
	// systemFlowRevocationPost follows every change whose pre flow revokes up to a cutoff, moving the
	// cutoff past the write.
	systemFlowRevocationPost      = "system-revocation-post"
	systemFlowInputResourceServer = "targetResourceServerId"
	systemFlowInputResource       = "targetResourceId"
	systemFlowInputAction         = "targetActionId"
)
