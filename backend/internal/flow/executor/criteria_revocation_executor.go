// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"encoding/json"
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
	removed, err := removedHolderCriteria(ctx, plan)
	if err != nil {
		return nil, err
	}
	plan.Criteria = append(plan.Criteria, removed...)
	return e.write(ctx, plan, time.Now().UTC())
}

// removedHolderCriteria returns the criteria for the holders the committed change removed that the plan
// does not cover, such as a holder added after the plan was made. Only a system flow, whose inputs come
// from the service that made the change, may name them, and only for a plan recording its holder scopes.
func removedHolderCriteria(ctx *providers.NodeContext, plan revocationPlan) ([]revocation.Criterion, error) {
	if ctx.FlowType != providers.FlowTypeSystem || len(plan.HolderScopes) == 0 {
		return nil, nil
	}
	encoded, _ := ctx.ConsumeInput(revocationInputRemovedHolders)
	if encoded == "" {
		return nil, nil
	}
	var holders []string
	if err := json.Unmarshal([]byte(encoded), &holders); err != nil {
		return nil, fmt.Errorf("failed to decode the removed holders: %w", err)
	}
	covered := make(map[string]struct{}, len(plan.Criteria))
	for _, criterion := range plan.Criteria {
		covered[criterion.Value] = struct{}{}
	}
	var criteria []revocation.Criterion
	for _, entityID := range holders {
		for _, scope := range plan.HolderScopes {
			value := revocation.EntityScopeCriterionValue(entityID, scope.Audience, scope.Scope)
			if _, ok := covered[value]; ok {
				continue
			}
			covered[value] = struct{}{}
			criteria = append(criteria, revocation.Criterion{Type: revocation.CriterionTypeEntityScope, Value: value})
		}
	}
	return criteria, nil
}

// write records every criterion of the plan with the given cutoff.
func (e *criteriaRevocationExecutor) write(ctx *providers.NodeContext, plan revocationPlan,
	cutoff time.Time) (*providers.ExecutorResponse, error) {
	if len(plan.Criteria) > 0 {
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
		if svcErr := e.revoker.RevokeCriteriaBatch(ctx.Context, revocations); svcErr != nil {
			if svcErr.Type == tidcommon.ServerErrorType {
				return nil, fmt.Errorf("failed to revoke tokens by criteria: %s", svcErr.Error.DefaultValue)
			}
			return &providers.ExecutorResponse{Status: providers.ExecFailure, Error: svcErr}, nil
		}
	}
	return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
}
