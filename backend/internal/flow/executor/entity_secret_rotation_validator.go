// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// clientSecretValidator decides whether an entity's client secret may be regenerated. The entity service
// implements it, asking the service that owns the entity's category.
type clientSecretValidator interface {
	ValidateClientSecretRegeneration(ctx context.Context, category providers.EntityCategory, entityID string) (
		*entity.ClientSecretProfile, *tidcommon.ServiceError)
}

// entitySecretRotationValidator is the preparatory node of the flow run before an entity's client secret is
// replaced, for an application or an agent alike: it validates the target and publishes the trusted plan
// that revokes what the old secret issued.
type entitySecretRotationValidator struct {
	providers.Executor
	validator clientSecretValidator
}

var _ providers.Executor = (*entitySecretRotationValidator)(nil)

// newEntitySecretRotationValidator creates the preparatory executor for a client secret rotation.
func newEntitySecretRotationValidator(factory core.FlowFactoryInterface,
	validator clientSecretValidator) *entitySecretRotationValidator {
	base := factory.CreateExecutor(ExecutorNameEntitySecretRotationValidator, providers.ExecutorTypeUtility,
		[]providers.Input{
			{Identifier: entity.SystemFlowInputEntity, Type: providers.InputTypeText, Required: true},
			{Identifier: entity.SystemFlowInputEntityCategory, Type: providers.InputTypeText, Required: true},
		}, nil, &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeSystem},
			DefaultMode:        ExecutorModeRevokeBeforeAction,
			SupportedModes:     []string{ExecutorModeRevokeBeforeAction},
		})
	return &entitySecretRotationValidator{Executor: base, validator: validator}
}

// Execute validates the target entity and publishes a plan revoking what its client issued up to now.
func (e *entitySecretRotationValidator) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	execResp := &providers.ExecutorResponse{Status: providers.ExecComplete}
	if !e.HasRequiredInputs(ctx, execResp) {
		execResp.Status = providers.ExecUserInputRequired
		return execResp, nil
	}
	entityID, ok := ctx.ConsumeInput(entity.SystemFlowInputEntity)
	if !ok || entityID == "" {
		return nil, fmt.Errorf("target entity is required")
	}
	category, ok := ctx.ConsumeInput(entity.SystemFlowInputEntityCategory)
	if !ok || category == "" {
		return nil, fmt.Errorf("target entity category is required")
	}
	if e.validator == nil {
		return nil, fmt.Errorf("entity service is not configured")
	}

	profile, svcErr := e.validator.ValidateClientSecretRegeneration(ctx.Context,
		providers.EntityCategory(category), entityID)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("failed to validate client secret regeneration: %s",
				svcErr.Error.DefaultValue)
		}
		// The owning service's own refusal says what the operator has to do about it; only one with no
		// code of its own falls back to this executor's.
		execResp.Status = providers.ExecFailure
		refusal := *svcErr
		if refusal.Code == "" {
			refusal = ErrSecretRegenerationNotAllowed
		}
		execResp.Error = &refusal
		return execResp, nil
	}

	plan := revocationPlan{
		Mode:     revocation.ModeBeforeAction,
		Reason:   revocation.ReasonApplicationSecretRegenerated,
		TargetID: entityID,
	}
	if profile == nil || profile.ClientID == "" {
		plan.NothingToRevoke = true
	} else {
		plan.Criteria = []revocation.Criterion{
			{Type: revocation.CriterionTypeApplicationKey, Value: profile.ClientID},
		}
		plan.TTLSeconds = profile.MaxLifetimeSeconds
		plan.Cutoff = time.Now().UTC()
	}
	encoded, err := encodeRevocationPlan(plan)
	if err != nil {
		return nil, err
	}
	execResp.SharedRuntimeData = map[string]string{common.RuntimeKeyRevocationPlan: encoded}
	return execResp, nil
}
