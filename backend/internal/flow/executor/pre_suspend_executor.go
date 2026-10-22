// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// preSuspendExecutor checks that the operator may suspend the target, then publishes the revocation
// plan the containment and suspend nodes consume.
type preSuspendExecutor struct {
	providers.Executor
	governance   providers.IdentityGovernanceProvider
	cutoffLeeway time.Duration
	// applications resolves the OAuth client the subject owns. Nil revokes by entity ID only.
	applications applicationAdminProvider
	logger       *log.Logger
}

var _ providers.Executor = (*preSuspendExecutor)(nil)

// newPreSuspendExecutor creates the preparatory executor for suspend flows.
func newPreSuspendExecutor(factory core.FlowFactoryInterface,
	governance providers.IdentityGovernanceProvider, cutoffLeeway time.Duration) *preSuspendExecutor {
	base := factory.CreateExecutor(ExecutorNamePreSuspend, providers.ExecutorTypeUtility,
		[]providers.Input{
			{Identifier: revocationInputSubject, Type: providers.InputTypeText, Required: true},
		}, nil, &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration},
		})
	return &preSuspendExecutor{
		Executor:     base,
		governance:   governance,
		cutoffLeeway: cutoffLeeway,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "PreSuspendExecutor"),
			log.String(log.LoggerKeyExecutorName, ExecutorNamePreSuspend)),
	}
}

// setApplicationProvider injects the application service. See ExecutorRegistryInterface.
func (e *preSuspendExecutor) setApplicationProvider(provider applicationAdminProvider) {
	e.applications = provider
}

// Execute validates the target and publishes the suspension plan.
func (e *preSuspendExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	execResp := &providers.ExecutorResponse{Status: providers.ExecComplete}
	if !e.HasRequiredInputs(ctx, execResp) {
		execResp.Status = providers.ExecUserInputRequired
		return execResp, nil
	}
	subject, ok := ctx.ConsumeInput(revocationInputSubject)
	if !ok || subject == "" {
		return nil, fmt.Errorf("subject is required")
	}
	if e.governance == nil {
		return nil, fmt.Errorf("identity governance provider is not configured")
	}

	// Checked before containment, so nothing acts on a target the operator may not govern.
	if svcErr := e.governance.ValidateSuspend(ctx.Context, subject); svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("failed to validate suspension: %s", svcErr.Error.DefaultValue)
		}
		e.logger.Debug(ctx.Context, "Identity suspension refused before containment",
			log.String(log.LoggerKeyExecutionID, ctx.ExecutionID), log.MaskedString(log.LoggerKeyEntityID, subject))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrIdentityGovernanceFailed
		return execResp, nil
	}

	clientID, err := e.oauthClientID(ctx.Context, subject)
	if err != nil {
		return nil, err
	}
	encoded, err := encodeRevocationPlan(buildSuspensionPlan(subject, clientID, time.Now(), e.cutoffLeeway))
	if err != nil {
		return nil, err
	}
	execResp.SharedRuntimeData = map[string]string{common.RuntimeKeyRevocationPlan: encoded}
	return execResp, nil
}

// oauthClientID returns the client ID of the OAuth client the subject owns, or "" when it owns none.
func (e *preSuspendExecutor) oauthClientID(ctx context.Context, subject string) (string, error) {
	if e.applications == nil {
		return "", nil
	}
	clientID, svcErr := e.applications.GetOAuthClientID(ctx, subject)
	if svcErr != nil {
		return "", fmt.Errorf("failed to resolve the subject's OAuth client: %s", svcErr.Error.DefaultValue)
	}
	return clientID, nil
}

// buildSuspensionPlan revokes tokens issued to the subject, and through its client ID when it owns
// one, before a cutoff of now plus the JWT leeway.
func buildSuspensionPlan(subject, clientID string, now time.Time, leeway time.Duration) revocationPlan {
	if leeway < 0 {
		leeway = 0
	}
	plan := revocationPlan{
		Criteria:   []revocation.Criterion{{Type: revocation.CriterionTypeSubject, Value: subject}},
		Mode:       revocation.ModeBeforeAction,
		Cutoff:     now.Add(leeway).UTC(),
		Reason:     revocation.ReasonIdentitySuspended,
		TargetID:   subject,
		TTLSeconds: int64(suspensionRevocationTTL / time.Second),
	}
	if clientID != "" {
		plan.Criteria = append(plan.Criteria,
			revocation.Criterion{Type: revocation.CriterionTypeApplicationKey, Value: clientID})
	}
	return plan
}
