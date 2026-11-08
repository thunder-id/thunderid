// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"

	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// The system flow that follows every change whose pre flow revokes up to a cutoff, moving the cutoff past
// the write, the input naming the role the flows act on, and the input listing, as a JSON array, the
// entities whose assignments a role deletion removed.
const (
	systemFlowRevocationPost      = "system-revocation-post"
	systemFlowInputRole           = "targetRoleId"
	systemFlowInputRemovedHolders = "removedHolderIds"
)

// systemFlowProvider runs a system flow to completion. The flow engine implements it.
type systemFlowProvider interface {
	ExecuteSystemFlow(ctx context.Context, sysCtx *flowcommon.SystemFlowContext) (*flowcommon.SystemFlowResult,
		*tidcommon.ServiceError)
}

// runSystemFlow runs a system flow with the given inputs, starting from what an earlier run shared, and
// returns what this run shared. With no provider, where system flows are off, it runs nothing.
func runSystemFlow(ctx context.Context, provider systemFlowProvider, handle string,
	inputs, runtimeData map[string]string) (map[string]string, *tidcommon.ServiceError) {
	if provider == nil {
		return nil, nil
	}
	result, svcErr := provider.ExecuteSystemFlow(ctx, &flowcommon.SystemFlowContext{
		Handle: handle, Inputs: inputs, RuntimeData: runtimeData})
	if svcErr != nil {
		return nil, svcErr
	}
	return result.RuntimeData, nil
}
