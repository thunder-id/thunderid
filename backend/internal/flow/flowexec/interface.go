// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowexec

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// FlowExecServiceInterface defines the interface for flow orchestration and acts as the
// entry point for flow execution.
type FlowExecServiceInterface interface {
	Execute(ctx context.Context, appID, executionID, flowType string, verbose bool,
		action string, inputs map[string]string, challengeToken, flowSecret, attestationToken string,
		sessionData []SessionData) (*FlowStep, *tidcommon.ServiceError)
	ExecuteByID(ctx context.Context, flowID, executionID string, verbose bool,
		action string, inputs map[string]string, challengeToken string, sessionData []SessionData) (
		*FlowStep, *tidcommon.ServiceError)
	InitiateFlow(ctx context.Context, initContext *FlowInitContext) (string, *tidcommon.ServiceError)
	InitiateAndExecute(ctx context.Context, initContext *FlowInitContext) (*FlowStep, *tidcommon.ServiceError)
	ExecuteSystemFlow(ctx context.Context, sysCtx *SystemFlowContext) (*SystemFlowResult, *tidcommon.ServiceError)
}
