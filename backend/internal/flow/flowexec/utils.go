// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowexec

import (
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// actsForCaller reports whether a flow of flowType acts with the authority of the caller who started it. The
// other flows act for whoever drives them, often an anonymous caller, so their nodes run with the server's
// authority. An unknown type acts for its caller.
func actsForCaller(flowType providers.FlowType) bool {
	switch flowType {
	case providers.FlowTypeAuthentication, providers.FlowTypeRegistration, providers.FlowTypeRecovery,
		providers.FlowTypeSignOut, providers.FlowTypeDeviceGrant, providers.FlowTypeSelf, providers.FlowTypeSystem:
		return false
	default:
		return true
	}
}

// recordsAuthority reports whether a flow of flowType records the caller who started it, to run its later steps
// with. An invitation flow does not: each step runs with the request's own context, the admin's token on the
// admin's steps and the server's authority on the invitee's, and its role step records the admin itself.
func recordsAuthority(flowType providers.FlowType) bool {
	return actsForCaller(flowType) && flowType != providers.FlowTypeInvitation
}
