// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowexec

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
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

// definitionReader reads the definitions of the flows the engine runs with the server's authority. Running a flow
// needs its definition; who may run it is decided by the flow's type and the units it is shared with, not by the
// caller's permission to read flows.
type definitionReader struct {
	providers.FlowProvider
}

// GetFlow reads the flow with the server's authority.
func (r definitionReader) GetFlow(ctx context.Context, flowID string) (
	*providers.CompleteFlowDefinition, *tidcommon.ServiceError) {
	return r.FlowProvider.GetFlow(security.WithRuntimeContext(ctx), flowID)
}

// GetFlowByHandle reads the flow with the server's authority.
func (r definitionReader) GetFlowByHandle(ctx context.Context, handle string, flowType providers.FlowType) (
	*providers.CompleteFlowDefinition, *tidcommon.ServiceError) {
	return r.FlowProvider.GetFlowByHandle(security.WithRuntimeContext(ctx), handle, flowType)
}
