// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
)

// The system flow every group change whose pre flow revokes up to a cutoff runs after its write, and
// the input naming the group the flows act on.
const (
	// systemFlowRevocationPost moves the cutoff of a pre flow past the write.
	systemFlowRevocationPost = "system-revocation-post"
	systemFlowInputGroup     = "targetGroupId"
)

// systemFlowRunner runs a system flow to completion. The flow engine implements it.
type systemFlowRunner interface {
	ExecuteSystemFlow(ctx context.Context, sysCtx *flowcommon.SystemFlowContext) (*flowcommon.SystemFlowResult,
		*tidcommon.ServiceError)
}

// runSystemFlow runs a system flow with the given inputs, starting from what an earlier run shared, and
// returns what this run shared. With system flows off it runs nothing.
func (gs *groupService) runSystemFlow(ctx context.Context, handle string,
	inputs, runtimeData map[string]string) (map[string]string, *tidcommon.ServiceError) {
	if gs.systemFlows == nil {
		return nil, nil
	}
	result, svcErr := gs.systemFlows.ExecuteSystemFlow(ctx, &flowcommon.SystemFlowContext{
		Handle: handle, Inputs: inputs, RuntimeData: runtimeData})
	if svcErr != nil {
		return nil, svcErr
	}
	return result.RuntimeData, nil
}
