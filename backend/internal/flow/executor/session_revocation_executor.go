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

// Execute acts on every session dimension the trusted plan selects.
//
// A subject plan ends that subject's sessions outright, since all of them belong to the user being acted
// on. An application plan instead detaches only that application's participation, because an SSO session
// routinely spans several applications and ending it would sign the user out of ones that still exist.
//
// The application branch keys off the plan's target rather than its criteria, and therefore runs even when
// there is nothing to revoke: participation is recorded per application whether or not it ever held a
// token, so an application with no OAuth component can still be a participant. It applies to a deletion
// only, never to a secret rotation.
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
		if err := e.sessionSvc.RemoveApplication(ctx.Context, plan.TargetID); err != nil {
			return nil, fmt.Errorf("failed to detach application from sessions: %w", err)
		}
	}
	return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
}

// isApplicationDeletionPlan reports whether the plan deletes an application, which is the only case that
// detaches session participation.
//
// A secret regeneration deliberately does not: the artifacts issued under the old secret are denied, but
// the user's SSO session is still legitimately theirs and they re-authorize under the new secret. Keying
// on the reason keeps that true even in a hand-built flow that wires this node into a rotation, and stops
// a future target type inheriting the branch by virtue of having a target.
func isApplicationDeletionPlan(plan revocationPlan) bool {
	return plan.TargetID != "" && plan.Reason == revocation.ReasonApplicationDeleted
}
