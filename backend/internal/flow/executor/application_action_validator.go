// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"fmt"
	"sort"
	"time"

	appmodel "github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// applicationValidator validates the target of one administrative application action and returns what a
// revocation against it needs.
type applicationValidator func(ctx context.Context, appID string) (
	*appmodel.ApplicationArtifactProfile, *tidcommon.ServiceError)

// applicationAction is what the validator does for one node mode: which validation runs, the reason of
// the bounded revocation it plans, and which refusal is reported when the validator names none.
type applicationAction struct {
	validate func(applicationAdminProvider) applicationValidator
	reason   revocation.Reason
	notFound tidcommon.ServiceError
}

// applicationActionsByMode maps the node's mode to the action it prepares. A client secret rotation has
// its own validator, for applications and agents alike: see entitySecretRotationValidator.
var applicationActionsByMode = map[string]applicationAction{
	ExecutorModeRevokeBeforeRemoval: {
		validate: func(p applicationAdminProvider) applicationValidator { return p.ValidateDeleteApplication },
		reason:   revocation.ReasonApplicationRemoved,
		notFound: ErrApplicationDeletionNotAllowed,
	},
}

// applicationActionValidator is the preparatory node of an application administration flow: it
// validates the target and publishes the trusted revocation plan the later nodes consume. The node's
// mode selects the action, so one executor serves every application administration flow.
type applicationActionValidator struct {
	providers.Executor
	provider applicationAdminProvider
}

var _ providers.Executor = (*applicationActionValidator)(nil)

// newApplicationActionValidator creates the preparatory executor for the application flows.
//
// No DefaultMode is declared, so flow creation requires the node to state its mode: the graph builder
// hands the executor the node's own mode and a default would never reach it.
func newApplicationActionValidator(
	factory core.FlowFactoryInterface) *applicationActionValidator {
	modes := make([]string, 0, len(applicationActionsByMode))
	for mode := range applicationActionsByMode {
		modes = append(modes, mode)
	}
	sort.Strings(modes)
	base := factory.CreateExecutor(ExecutorNameApplicationActionValidator, providers.ExecutorTypeUtility,
		[]providers.Input{
			{Identifier: revocationInputApplication, Type: providers.InputTypeText, Required: true},
		}, nil, &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration, providers.FlowTypeSystem},
			SupportedModes:     modes,
		})
	return &applicationActionValidator{Executor: base}
}

// setApplicationProvider injects the application service. See ExecutorRegistryInterface.
func (e *applicationActionValidator) setApplicationProvider(
	provider applicationAdminProvider) {
	e.provider = provider
}

// Execute validates the target application and publishes the trusted plan the later nodes act on. The
// node's mode selects which action is prepared.
func (e *applicationActionValidator) Execute(ctx *providers.NodeContext) (
	*providers.ExecutorResponse, error) {
	action, ok := applicationActionsByMode[ctx.ExecutorMode]
	if !ok {
		return nil, fmt.Errorf("unsupported revocation mode for %s: %q",
			ExecutorNameApplicationActionValidator, ctx.ExecutorMode)
	}
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

	profile, svcErr := action.validate(provider)(ctx.Context, appID)
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
			refusal = action.notFound
		}
		execResp.Error = &refusal
		return execResp, nil
	}

	built := e.buildPlan(action.reason, appID, profile)
	plan, err := encodeRevocationPlan(built)
	if err != nil {
		return nil, err
	}
	execResp.SharedRuntimeData = map[string]string{common.RuntimeKeyRevocationPlan: plan}
	return execResp, nil
}

// buildPlan records the trusted intent for this action, bounded at now. An application with no OAuth
// client publishes no criteria and says so explicitly, which keeps it distinguishable from a plan that
// lost its criteria.
func (e *applicationActionValidator) buildPlan(reason revocation.Reason,
	appID string, profile *appmodel.ApplicationArtifactProfile) revocationPlan {
	plan := revocationPlan{
		Mode:     revocation.ModeBeforeAction,
		Reason:   reason,
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
	plan.Cutoff = time.Now().UTC()
	return plan
}
