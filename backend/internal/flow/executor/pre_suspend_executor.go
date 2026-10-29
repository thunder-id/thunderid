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
	governance       providers.IdentityGovernanceProvider
	cutoffLeeway     time.Duration
	actors           providers.ActorProvider
	artifactLifetime artifactLifetimeResolver
	logger           *log.Logger
}

var _ providers.Executor = (*preSuspendExecutor)(nil)

// newPreSuspendExecutor creates the preparatory executor for suspend flows.
func newPreSuspendExecutor(factory core.FlowFactoryInterface, governance providers.IdentityGovernanceProvider,
	cutoffLeeway time.Duration, artifactLifetime artifactLifetimeResolver) *preSuspendExecutor {
	base := factory.CreateExecutor(ExecutorNamePreSuspend, providers.ExecutorTypeUtility,
		[]providers.Input{
			{Identifier: revocationInputSubject, Type: providers.InputTypeText, Required: true},
		}, nil, &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration},
		})
	return &preSuspendExecutor{
		Executor:         base,
		governance:       governance,
		cutoffLeeway:     cutoffLeeway,
		artifactLifetime: artifactLifetime,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "PreSuspendExecutor"),
			log.String(log.LoggerKeyExecutorName, ExecutorNamePreSuspend)),
	}
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
	if e.actors == nil {
		return nil, fmt.Errorf("actor provider is not configured")
	}
	if e.artifactLifetime == nil {
		return nil, fmt.Errorf("artifact lifetime resolver is not configured")
	}

	// Checked before containment, so nothing acts on a target the operator may not govern. The note is
	// read, not consumed, since the suspend node consumes it.
	operatorNote := ctx.UserInputs[governanceInputOperatorNote]
	if svcErr := e.governance.ValidateSuspend(ctx.Context, subject, operatorNote); svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("failed to validate suspension: %s", svcErr.Error.DefaultValue)
		}
		e.logger.Debug(ctx.Context, "Identity suspension refused before containment",
			log.String(log.LoggerKeyExecutionID, ctx.ExecutionID), log.MaskedString(log.LoggerKeyEntityID, subject))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrIdentityGovernanceFailed
		return execResp, nil
	}

	clientID, clientLifetime, err := e.oauthClient(ctx.Context, subject)
	if err != nil {
		return nil, err
	}
	encoded, err := encodeRevocationPlan(
		buildSuspensionPlan(subject, clientID, clientLifetime, time.Now(), e.cutoffLeeway))
	if err != nil {
		return nil, err
	}
	execResp.SharedRuntimeData = map[string]string{common.RuntimeKeyRevocationPlan: encoded}
	return execResp, nil
}

// setActorProvider injects the actor provider. See SetActorProvider.
func (e *preSuspendExecutor) setActorProvider(provider providers.ActorProvider) {
	e.actors = provider
}

// oauthClient returns the client ID of the OAuth client the subject acts through and the longest an
// artifact issued to it can live. A user yields no client.
func (e *preSuspendExecutor) oauthClient(ctx context.Context, subject string) (string, time.Duration, error) {
	client, svcErr := e.actors.GetOAuthClientByID(ctx, subject)
	if svcErr != nil {
		return "", 0, fmt.Errorf("failed to resolve the subject's OAuth client: %s", svcErr.Error.DefaultValue)
	}
	if client == nil {
		return "", 0, nil
	}
	return client.ClientID, e.artifactLifetime(client), nil
}

// buildSuspensionPlan revokes tokens issued to the subject, and through its client ID when it has
// one, before a cutoff of now plus the JWT leeway. The rows are kept for suspensionRevocationTTL, or
// for the client's artifact lifetime when that is longer.
func buildSuspensionPlan(subject, clientID string, clientLifetime time.Duration, now time.Time,
	leeway time.Duration) revocationPlan {
	plan := revocationPlan{
		Criteria:   []revocation.Criterion{{Type: revocation.CriterionTypeSubject, Value: subject}},
		Mode:       revocation.ModeBeforeAction,
		Cutoff:     now.Add(max(leeway, 0)).UTC(),
		Reason:     revocation.ReasonIdentitySuspended,
		TargetID:   subject,
		TTLSeconds: int64(max(suspensionRevocationTTL, clientLifetime) / time.Second),
	}
	if clientID != "" {
		plan.Criteria = append(plan.Criteria,
			revocation.Criterion{Type: revocation.CriterionTypeApplicationKey, Value: clientID})
	}
	return plan
}
