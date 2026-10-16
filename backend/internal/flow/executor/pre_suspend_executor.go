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

// suspensionRevocationTTL keeps a suspension's deny-list rows long enough to outlast any token the
// subject held, whichever client issued it.
// TODO(#5805): use the server-wide maximum token lifetime once it is introduced.
const suspensionRevocationTTL = 7 * 24 * time.Hour

// preSuspendExecutor is the preparatory node of a suspend flow, as PreDeleteExecutor is for deletion.
// It checks that the operator may suspend the target before any revocation runs, then publishes the
// trusted revocation plan the containment and suspend nodes consume.
type preSuspendExecutor struct {
	providers.Executor
	governance   providers.IdentityGovernanceProvider
	cutoffLeeway time.Duration
	// applications resolves the OAuth client a subject owns. An agent's client is resolved here too, as
	// OAuth resolves it, so no agent seam is needed. Injected after construction; nil revokes by entity
	// ID only.
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

	// The organization-unit boundary, checked here so nothing downstream acts on a target the
	// operator may not govern. The suspend node checks it again on the write.
	if svcErr := e.governance.ValidateSuspend(ctx.Context, subject); svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("failed to validate suspension: %s", svcErr.Error.DefaultValue)
		}
		e.logger.Debug(ctx.Context, "Identity suspension refused before containment",
			log.String(log.LoggerKeyExecutionID, ctx.ExecutionID), log.MaskedString("subject", subject))
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

// oauthClientID returns the client ID of the OAuth client the subject owns, or "" for a subject with
// none, such as a user, which is revoked by entity ID alone.
func (e *preSuspendExecutor) oauthClientID(ctx context.Context, subject string) (string, error) {
	if e.applications == nil {
		return "", nil
	}
	clientID, svcErr := e.applications.GetOAuthClientID(ctx, subject)
	if svcErr == nil {
		return clientID, nil
	}
	if svcErr.Type == tidcommon.ClientErrorType {
		return "", nil
	}
	return "", fmt.Errorf("failed to resolve the subject's OAuth client: %s", svcErr.Error.DefaultValue)
}

// buildSuspensionPlan revokes what the subject held before the hold: tokens whose sub is the entity
// ID and, for an application or an agent, tokens issued through its client ID. The cutoff is this
// node's time plus the JWT leeway, which covers the gap until the suspend node writes the hold.
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
