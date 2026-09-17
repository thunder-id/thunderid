// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"fmt"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// sessionRevocationExecutor terminates the SSO sessions of the subjects named in the trusted
// revocation plan.
type sessionRevocationExecutor struct {
	providers.Executor
	sessionSvc session.Service
}

var _ providers.Executor = (*sessionRevocationExecutor)(nil)

// newSessionRevocationExecutor creates an executor that terminates sessions matching a trusted plan.
func newSessionRevocationExecutor(factory core.FlowFactoryInterface,
	sessionSvc session.Service) *sessionRevocationExecutor {
	base := factory.CreateExecutor(ExecutorNameSessionRevocation, providers.ExecutorTypeUtility,
		nil, nil, &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration},
		})
	return &sessionRevocationExecutor{Executor: base, sessionSvc: sessionSvc}
}

// Execute acts on every session dimension the trusted plan selects. A subject plan ends that subject's
// sessions outright; an application plan detaches only that application's participation, since a session
// routinely spans several applications. The application branch keys off the plan's target rather than its
// criteria, so it runs even when there is nothing to revoke, and applies to a deletion only.
func (e *sessionRevocationExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	if e.sessionSvc == nil {
		return nil, fmt.Errorf("session service is not configured")
	}
	plan, err := decodeRevocationPlan(ctx.SharedRuntimeData)
	if err != nil {
		return nil, err
	}
	for _, criterion := range plan.Criteria {
		if criterion.Type != revocation.CriterionTypeSubject {
			continue
		}
		if err := e.sessionSvc.TerminateBySubject(ctx.Context, criterion.Value); err != nil {
			return nil, fmt.Errorf("failed to terminate sessions by subject: %w", err)
		}
	}
	if isApplicationDeletionPlan(plan) {
		if err := e.sessionSvc.DetachApplication(ctx.Context, plan.TargetID); err != nil {
			return nil, fmt.Errorf("failed to detach application from sessions: %w", err)
		}
	}
	return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
}

// isApplicationDeletionPlan reports whether the plan deletes an application, the only case that detaches
// session participation. A secret regeneration does not: the session is still legitimately the user's.
func isApplicationDeletionPlan(plan revocationPlan) bool {
	return plan.TargetID != "" && plan.Reason == revocation.ReasonApplicationDeleted
}
