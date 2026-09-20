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
	// restamp makes this node rewrite the plan's criteria with a cutoff of now instead of the plan's
	// own. It is how the flow closes the window between the revocation and the action it precedes.
	restamp bool
}

var _ providers.Executor = (*criteriaRevocationExecutor)(nil)

// newCriteriaRevocationExecutor creates an executor that persists a trusted criteria revocation plan.
func newCriteriaRevocationExecutor(factory core.FlowFactoryInterface,
	revoker revocation.CriteriaRevoker) *criteriaRevocationExecutor {
	return newCriteriaRevocationExecutorFor(factory, revoker, ExecutorNameCriteriaRevocation, false)
}

// newCriteriaRevocationRestampExecutor creates the post-action node that advances a boundary
// revocation's cutoff.
//
// The preparatory node stamps the cutoff before the action runs, which is the ordering that keeps a
// failed action safe. But the grant is still held until the action commits, so a token minted in that
// window carries the scopes and its iat is past the cutoff. Rewriting the same criteria afterwards
// moves the cutoff forward and sweeps those up. The write is the same idempotent upsert, and it only
// ever advances a boundary row, so running it changes nothing else.
func newCriteriaRevocationRestampExecutor(factory core.FlowFactoryInterface,
	revoker revocation.CriteriaRevoker) *criteriaRevocationExecutor {
	return newCriteriaRevocationExecutorFor(factory, revoker,
		ExecutorNameCriteriaRevocationRestamp, true)
}

// newCriteriaRevocationExecutorFor builds one of the two registered names. They are separate names
// rather than a node property so flow creation pins the behavior and the designer palette says which
// of the two a node is.
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
		// A terminal plan has no cutoff to advance, and rewriting one would be rejected as a mode
		// mismatch. Only a bounded revocation has a window to close.
		if plan.Mode != revocation.ModeBeforeAction {
			return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
		}
		cutoff = time.Now().UTC()
	}
	// A plan's criteria are written together in one batch rather than one call per criterion: an
	// administrative change can carry up to oauth.revocation.criteria.max_criteria of them, and a round
	// trip per row is the difference between this node returning and the caller giving up waiting.
	//
	// A plan with nothing to revoke — the application-artifact case with no client key, notably — calls
	// the revoker not at all, the same as the loop this replaced never entered its body for one.
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
			return nil, fmt.Errorf("failed to revoke tokens by criteria: %w", err)
		}
	}
	return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
}
