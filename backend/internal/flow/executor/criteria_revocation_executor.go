// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// criteriaRevocationExecutor persists the criteria carried by the trusted revocation plan.
type criteriaRevocationExecutor struct {
	providers.Executor
	revoker revocation.CriteriaRevoker
}

var _ providers.Executor = (*criteriaRevocationExecutor)(nil)

// newCriteriaRevocationExecutor creates an executor that persists a trusted criteria revocation plan.
func newCriteriaRevocationExecutor(factory core.FlowFactoryInterface,
	revoker revocation.CriteriaRevoker) *criteriaRevocationExecutor {
	base := factory.CreateExecutor(ExecutorNameCriteriaRevocation, providers.ExecutorTypeUtility,
		nil, nil, &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration, providers.FlowTypeSystem},
			DefaultMode:        ExecutorModeRevoke,
			SupportedModes:     []string{ExecutorModeAdvanceCutoff, ExecutorModeRevoke},
		})
	return &criteriaRevocationExecutor{Executor: base, revoker: revoker}
}

// Execute records every criterion from the trusted revocation plan. A plan declaring NothingToRevoke
// writes nothing and completes; an empty plan without that declaration is rejected when decoded.
func (e *criteriaRevocationExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	if e.revoker == nil {
		return nil, fmt.Errorf("criteria revoker is not configured")
	}
	switch ctx.ExecutorMode {
	// An omitted mode is the default mode, revoke.
	case "", ExecutorModeRevoke:
		return e.revoke(ctx)
	case ExecutorModeAdvanceCutoff:
		return e.advanceCutoff(ctx)
	default:
		return nil, fmt.Errorf("unsupported mode for %s: %q", ExecutorNameCriteriaRevocation, ctx.ExecutorMode)
	}
}

// revoke writes the plan's criteria with the plan's own cutoff.
func (e *criteriaRevocationExecutor) revoke(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	plan, err := decodeRevocationPlan(ctx.SharedRuntimeData)
	if err != nil {
		return nil, err
	}
	return e.write(ctx, plan, plan.Cutoff)
}

// advanceCutoff rewrites a before-action plan's criteria with a cutoff of now, once the action has committed.
func (e *criteriaRevocationExecutor) advanceCutoff(ctx *providers.NodeContext) (
	*providers.ExecutorResponse, error) {
	plan, err := decodeRevocationPlan(ctx.SharedRuntimeData)
	if err != nil {
		return nil, err
	}
	// Only a bounded revocation has a cutoff to advance.
	if plan.Mode != revocation.ModeBeforeAction {
		return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
	}
	return e.write(ctx, plan, time.Now().UTC())
}

// write records every criterion of the plan with the given cutoff.
func (e *criteriaRevocationExecutor) write(ctx *providers.NodeContext, plan revocationPlan,
	cutoff time.Time) (*providers.ExecutorResponse, error) {
	if svcErr := writeRevocationPlan(ctx.Context, e.revoker, plan, cutoff); svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("failed to revoke tokens by criteria: %s", svcErr.Error.DefaultValue)
		}
		return &providers.ExecutorResponse{Status: providers.ExecFailure, Error: svcErr}, nil
	}
	return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
}

// AdvanceRevocationCutoff does what the advance_cutoff mode does, for a change its service writes after
// the system flow that planned the revocation: a before-action plan's criteria are recorded again with a
// cutoff of now, so a token issued while the change was being written loses what the change removed too.
func AdvanceRevocationCutoff(ctx context.Context, revoker revocation.CriteriaRevoker,
	sharedRuntimeData map[string]string) error {
	plan, err := decodeRevocationPlan(sharedRuntimeData)
	if err != nil {
		return err
	}
	if plan.Mode != revocation.ModeBeforeAction {
		return nil
	}
	if svcErr := writeRevocationPlan(ctx, revoker, plan, time.Now().UTC()); svcErr != nil {
		return fmt.Errorf("failed to revoke tokens by criteria: %s", svcErr.Error.DefaultValue)
	}
	return nil
}

// writeRevocationPlan records every criterion of the plan with the given cutoff.
func writeRevocationPlan(ctx context.Context, revoker revocation.CriteriaRevoker, plan revocationPlan,
	cutoff time.Time) *tidcommon.ServiceError {
	if len(plan.Criteria) == 0 {
		return nil
	}
	ttl := time.Duration(plan.TTLSeconds) * time.Second
	revocations := make([]revocation.CriteriaRevocation, len(plan.Criteria))
	for i, criterion := range plan.Criteria {
		revocations[i] = revocation.CriteriaRevocation{
			Criterion: criterion,
			Mode:      plan.Mode,
			Cutoff:    cutoff,
			Reason:    plan.Reason,
			TTL:       ttl,
		}
	}
	return revoker.RevokeCriteriaBatch(ctx, revocations)
}
