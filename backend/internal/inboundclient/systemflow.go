// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package inboundclient

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
)

// The system flows run around an inbound client's deletion, and the input naming the entity whose client it
// is. The pre flow revokes what the client issued, up to a cutoff, and detaches it from the sessions it took
// part in. The post flow follows the deletion, so an artifact issued while it ran is revoked too.
const (
	systemFlowInboundClientDeletePre = "system-inbound-client-delete-pre"
	// systemFlowRevocationPost moves the cutoff of a pre flow past the write.
	systemFlowRevocationPost = "system-revocation-post"
	systemFlowInputEntity    = "targetEntityId"
)

// systemFlowProvider runs a system flow to completion. The flow engine implements it.
type systemFlowProvider interface {
	ExecuteSystemFlow(ctx context.Context, sysCtx *flowcommon.SystemFlowContext) (*flowcommon.SystemFlowResult,
		*tidcommon.ServiceError)
}

// runSystemFlow runs a system flow with the given inputs, starting from what an earlier run shared, and
// returns what this run shared. With system flows off it runs nothing.
func (s *inboundClientService) runSystemFlow(ctx context.Context, handle string,
	inputs, runtimeData map[string]string) (map[string]string, *tidcommon.ServiceError) {
	if s.systemFlows == nil {
		return nil, nil
	}
	result, svcErr := s.systemFlows.ExecuteSystemFlow(ctx, &flowcommon.SystemFlowContext{
		Handle: handle, Inputs: inputs, RuntimeData: runtimeData})
	if svcErr != nil {
		return nil, svcErr
	}
	return result.RuntimeData, nil
}
