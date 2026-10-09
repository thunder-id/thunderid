// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"errors"
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
	// restamp rewrites the plan's criteria with a cutoff of now instead of the plan's own.
	restamp bool
}

var _ providers.Executor = (*criteriaRevocationExecutor)(nil)

// newCriteriaRevocationExecutor creates an executor that persists a trusted criteria revocation plan.
func newCriteriaRevocationExecutor(factory core.FlowFactoryInterface,
	revoker revocation.CriteriaRevoker) *criteriaRevocationExecutor {
	return newCriteriaRevocationExecutorFor(factory, revoker, ExecutorNameCriteriaRevocation, false)
}

// newPostRevocationExecutor creates the post-action node that advances a boundary revocation's cutoff.
func newPostRevocationExecutor(factory core.FlowFactoryInterface,
	revoker revocation.CriteriaRevoker) *criteriaRevocationExecutor {
	return newCriteriaRevocationExecutorFor(factory, revoker, ExecutorNamePostRevocation, true)
}

func newCriteriaRevocationExecutorFor(factory core.FlowFactoryInterface,
	revoker revocation.CriteriaRevoker, name string, restamp bool) *criteriaRevocationExecutor {
	base := factory.CreateExecutor(name, providers.ExecutorTypeUtility,
		nil, nil, &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration},
		})
	return &criteriaRevocationExecutor{Executor: base, revoker: revoker, restamp: restamp}
}

// Execute records every criterion from the trusted revocation plan. A plan declaring NothingToRevoke
// writes nothing and completes; an empty plan without that declaration is rejected when decoded.
func (e *criteriaRevocationExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	if e.revoker == nil {
		return nil, fmt.Errorf("criteria revoker is not configured")
	}
	plan, err := decodeRevocationPlan(ctx.SharedRuntimeData)
	if err != nil {
		return nil, err
	}
	cutoff := plan.Cutoff
	if e.restamp {
		// Only a bounded revocation has a cutoff to advance.
		if plan.Mode != revocation.ModeBeforeAction {
			return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
		}
		cutoff = time.Now().UTC()
	}
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
		if err := e.revoker.RevokeCriteriaBatch(ctx.Context, revocations); err != nil {
			var limitErr *revocation.CriteriaLimitExceededError
			if errors.As(err, &limitErr) {
				return &providers.ExecutorResponse{
					Status: providers.ExecFailure,
					Error:  errRevocationFanOutTooLargeFor(limitErr.Criteria, limitErr.Max),
				}, nil
			}
			return nil, fmt.Errorf("failed to revoke tokens by criteria: %w", err)
		}
	}
	return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
}
