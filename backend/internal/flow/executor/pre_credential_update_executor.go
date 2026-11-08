// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// preCredentialUpdateExecutor plans the revocation of what a user holds before their credentials change.
type preCredentialUpdateExecutor struct {
	providers.Executor
}

var _ providers.Executor = (*preCredentialUpdateExecutor)(nil)

// newPreCredentialUpdateExecutor creates the executor that plans a credential update's revocation.
func newPreCredentialUpdateExecutor(factory core.FlowFactoryInterface) *preCredentialUpdateExecutor {
	base := factory.CreateExecutor(ExecutorNamePreCredentialUpdate, providers.ExecutorTypeUtility,
		[]providers.Input{
			{Identifier: revocationInputSubject, Type: providers.InputTypeText, Required: true},
		}, nil, &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeSystem},
			DefaultMode:        string(revocation.ModeBeforeAction),
			SupportedModes:     []string{string(revocation.ModeBeforeAction)},
		})
	return &preCredentialUpdateExecutor{Executor: base}
}

// Execute publishes a plan revoking every token and session the subject was given up to now.
func (e *preCredentialUpdateExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	execResp := &providers.ExecutorResponse{Status: providers.ExecComplete}
	if !e.HasRequiredInputs(ctx, execResp) {
		execResp.Status = providers.ExecUserInputRequired
		return execResp, nil
	}
	subject, ok := ctx.ConsumeInput(revocationInputSubject)
	if !ok || subject == "" {
		return nil, fmt.Errorf("subject is required")
	}
	plan, err := encodeRevocationPlan(revocationPlan{
		Criteria: []revocation.Criterion{{Type: revocation.CriterionTypeSubject, Value: subject}},
		Mode:     revocation.ModeBeforeAction,
		Cutoff:   time.Now().UTC(),
		Reason:   revocation.ReasonCredentialUpdated,
	})
	if err != nil {
		return nil, err
	}
	execResp.SharedRuntimeData = map[string]string{common.RuntimeKeyRevocationPlan: plan}
	return execResp, nil
}
