// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowexec

import (
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// recordsAuthority reports whether a flow of flowType records the caller who started it, to run its later steps
// with. An invitation flow does not: each step runs with the request's own context, the admin's token on the
// admin's steps and the server's authority on the invitee's, and its role step records the admin itself.
func recordsAuthority(flowType providers.FlowType) bool {
	return common.ActsForCaller(flowType) && flowType != providers.FlowTypeInvitation
}
