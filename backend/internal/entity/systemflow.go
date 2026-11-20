// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"fmt"

	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// The system flows the entity service runs around a change that revokes what was issued before it, and the
// input naming the entity they act on. Each pre flow revokes up to a cutoff before the change is stored, and
// its post flow moves the cutoff past the write, so an artifact issued while it ran is revoked too.
const (
	// systemFlowEntityDeletePre revokes what was issued to the entity as a subject and ends its sessions.
	systemFlowEntityDeletePre = "system-entity-delete-pre"
	// systemFlowCredentialUpdatePre revokes what was issued to the entity as a subject and ends its sessions,
	// before a credential it holds is replaced.
	systemFlowCredentialUpdatePre = "system-entity-credential-update-pre" //nolint:gosec // flow handle
	// systemFlowCredentialUpdatePost also ends the sessions started while the credential was replaced.
	systemFlowCredentialUpdatePost = "system-entity-credential-update-post" //nolint:gosec // flow handle
	// systemFlowSecretRotatePre revokes what the entity's OAuth client issued before its secret is replaced.
	systemFlowSecretRotatePre = "system-entity-secret-rotate-pre"
	// systemFlowRevocationPost follows every pre flow, moving its cutoff past the write.
	systemFlowRevocationPost = "system-revocation-post"
	// systemFlowInputEntity names the entity the flows act on.
	systemFlowInputEntity = "targetEntityId"
)

// systemFlowProvider runs a system flow to completion. The flow engine implements it.
type systemFlowProvider interface {
	ExecuteSystemFlow(ctx context.Context, sysCtx *flowcommon.SystemFlowContext) (*flowcommon.SystemFlowResult,
		*tidcommon.ServiceError)
}

// runSystemFlow runs a system flow with the given inputs, starting from what an earlier run shared, and
// returns what this run shared. Where system flows are off it runs nothing.
func (s *entityService) runSystemFlow(ctx context.Context, handle string,
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

// revokeAround runs the pre flow, then change, then the post flow, all for the entity. A failed pre flow
// makes no change and is reported as errRevocationFailed. A failed post flow comes after the change is stored
// and is reported as ErrRevocationIncomplete. Where system flows are off only the change runs.
func (s *entityService) revokeAround(ctx context.Context, entityID, preFlow, postFlow string,
	change func() error) error {
	inputs := map[string]string{systemFlowInputEntity: entityID}
	shared, svcErr := s.runSystemFlow(ctx, preFlow, inputs, nil)
	if svcErr != nil {
		return fmt.Errorf("%w: %s", errRevocationFailed, svcErr.Code)
	}
	if err := change(); err != nil {
		return err
	}
	if _, svcErr = s.runSystemFlow(ctx, postFlow, inputs, shared); svcErr != nil {
		return fmt.Errorf("%w: %s", ErrRevocationIncomplete, svcErr.Code)
	}
	return nil
}
