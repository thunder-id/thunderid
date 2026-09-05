// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
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
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration},
		})
	return &criteriaRevocationExecutor{Executor: base, revoker: revoker}
}

// Execute records every criterion from the trusted revocation plan.
//
// A plan that declares NothingToRevoke writes nothing and completes: the preparatory node established
// there is no artifact to deny, which is the case for an application with no OAuth component. An empty
// plan that makes no such declaration is rejected by decodeRevocationPlan instead.
//
// The plan's TTL is passed through so a row outlasts the artifacts it denies. It only ever raises the
// lifetime: the revocation service keeps its configured default when the plan states none.
func (e *criteriaRevocationExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	if e.revoker == nil {
		return nil, fmt.Errorf("criteria revoker is not configured")
	}
	plan, err := decodeRevocationPlan(ctx.SharedRuntimeData)
	if err != nil {
		return nil, err
	}
	for _, criterion := range plan.Criteria {
		if err := e.revoker.RevokeByCriteria(ctx.Context, revocation.CriteriaRevocation{
			Criterion: criterion,
			Mode:      plan.Mode,
			Cutoff:    plan.Cutoff,
			Reason:    plan.Reason,
			TTL:       time.Duration(plan.TTLSeconds) * time.Second,
		}); err != nil {
			return nil, fmt.Errorf("failed to revoke tokens by criteria: %w", err)
		}
	}
	return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
}
