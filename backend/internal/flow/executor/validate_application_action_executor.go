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
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// applicationValidator validates the target of one administrative application action and returns what a
// revocation against it needs.
type applicationValidator func(ctx context.Context, appID string) (
	*providers.ApplicationArtifactProfile, *tidcommon.ServiceError)

// validateApplicationActionExecutor is the preparatory node of an application administration flow. It
// validates the target and publishes the trusted revocation plan the nodes after it consume, the same
// contract PreDeleteExecutor holds for user deletion.
//
// One type backs both registered names. The nodes differ only in which validation runs, which reason is
// recorded, and whether a cutoff is stamped, so they are parameterized at construction rather than
// duplicated. They stay two registered executors so each pins a single supported mode, which flow
// creation validates, and so the designer palette names the action rather than hiding it in a property.
type validateApplicationActionExecutor struct {
	providers.Executor
	provider providers.ApplicationAdminProvider
	validate func(providers.ApplicationAdminProvider) applicationValidator
	reason   revocation.Reason
	mode     revocation.Mode
	notFound tidcommon.ServiceError
}

var _ providers.Executor = (*validateApplicationActionExecutor)(nil)

// newValidateApplicationDeletionExecutor creates the preparatory executor for an application deletion.
func newValidateApplicationDeletionExecutor(
	factory core.FlowFactoryInterface) *validateApplicationActionExecutor {
	return newValidateApplicationActionExecutor(factory, ExecutorNameValidateApplicationDeletion,
		revocation.ReasonApplicationDeleted, revocation.ModeAll, ErrApplicationDeletionNotAllowed,
		func(p providers.ApplicationAdminProvider) applicationValidator { return p.ValidateDeleteApplication })
}

// newValidateSecretRegenerationExecutor creates the preparatory executor for a client secret regeneration.
func newValidateSecretRegenerationExecutor(
	factory core.FlowFactoryInterface) *validateApplicationActionExecutor {
	return newValidateApplicationActionExecutor(factory, ExecutorNameValidateSecretRegeneration,
		revocation.ReasonApplicationSecretRegenerated, revocation.ModeBeforeAction,
		ErrSecretRegenerationNotAllowed,
		func(p providers.ApplicationAdminProvider) applicationValidator {
			return func(ctx context.Context, appID string) (
				*providers.ApplicationArtifactProfile, *tidcommon.ServiceError) {
				return p.ValidateCredentialAction(ctx, appID, providers.CredentialActionRegenerate)
			}
		})
}

// newValidateApplicationActionExecutor creates a preparatory executor pinned to one action.
func newValidateApplicationActionExecutor(factory core.FlowFactoryInterface,
	name string, reason revocation.Reason,
	mode revocation.Mode, notFound tidcommon.ServiceError,
	validate func(providers.ApplicationAdminProvider) applicationValidator) *validateApplicationActionExecutor {
	base := factory.CreateExecutor(name, providers.ExecutorTypeUtility,
		[]providers.Input{
			{Identifier: revocationInputApplication, Type: providers.InputTypeText, Required: true},
		}, nil, &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration},
			DefaultMode:        string(mode),
			SupportedModes:     []string{string(mode)},
		})
	return &validateApplicationActionExecutor{
		Executor: base,
		validate: validate,
		reason:   reason,
		mode:     mode,
		notFound: notFound,
	}
}

// setApplicationProvider injects the application service. See ExecutorRegistryInterface.
func (e *validateApplicationActionExecutor) setApplicationProvider(
	provider providers.ApplicationAdminProvider) {
	e.provider = provider
}

// Execute validates the target application and publishes the trusted plan the later nodes act on.
//
// The mode is not read from the node: each registered name supports exactly one, so a node carrying
// anything else is rejected when the flow is created. Reading it here would only re-derive what the
// executor already knows, and the reason it must agree with is fixed by which executor this is.
func (e *validateApplicationActionExecutor) Execute(ctx *providers.NodeContext) (
	*providers.ExecutorResponse, error) {
	execResp := &providers.ExecutorResponse{Status: providers.ExecComplete}
	if !e.HasRequiredInputs(ctx, execResp) {
		execResp.Status = providers.ExecUserInputRequired
		return execResp, nil
	}
	appID, ok := ctx.ConsumeInput(revocationInputApplication)
	if !ok || appID == "" {
		return nil, fmt.Errorf("target application is required")
	}
	if e.provider == nil {
		return nil, fmt.Errorf("application service is not configured")
	}
	provider := e.provider

	profile, svcErr := e.validate(provider)(ctx.Context, appID)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("failed to validate application action: %s", svcErr.Error.DefaultValue)
		}
		// Carry the validator's own error rather than this executor's generic one. The refusals differ
		// in what the operator has to do about them — an application that authenticates without a
		// secret, one owned by a declarative file, one that is simply gone — and collapsing them
		// reports the wrong reason for two of the three. Only a service error with no code of its own
		// falls back, so the response always names something.
		execResp.Status = providers.ExecFailure
		refusal := *svcErr
		if refusal.Code == "" {
			refusal = e.notFound
		}
		execResp.Error = &refusal
		return execResp, nil
	}

	plan, err := encodeRevocationPlan(e.buildPlan(appID, profile))
	if err != nil {
		return nil, err
	}
	execResp.SharedRuntimeData = map[string]string{common.RuntimeKeyRevocationPlan: plan}
	return execResp, nil
}

// buildPlan records the trusted intent for this action.
//
// An application with no OAuth client publishes no criteria and says so explicitly: it issues no
// artifacts, so revocation has nothing to match, and the flow still deletes the record. The flag is what
// keeps that distinguishable from a plan that lost its criteria, which the consumers reject.
func (e *validateApplicationActionExecutor) buildPlan(
	appID string, profile *providers.ApplicationArtifactProfile) revocationPlan {
	plan := revocationPlan{
		Mode:     e.mode,
		Reason:   e.reason,
		TargetID: appID,
	}
	if profile == nil || profile.ClientKey == "" {
		plan.NothingToRevoke = true
		return plan
	}
	plan.Criteria = []revocation.Criterion{
		{Type: revocation.CriterionTypeApplicationKey, Value: profile.ClientKey},
	}
	plan.TTLSeconds = profile.MaxLifetimeSeconds
	if e.mode == revocation.ModeBeforeAction {
		plan.Cutoff = time.Now().UTC()
	}
	return plan
}
