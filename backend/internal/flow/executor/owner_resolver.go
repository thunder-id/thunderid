// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"fmt"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const ownerResolverLoggerComponentName = "OwnerResolver"

// ownerResolver offers an owner for an entity being provisioned. The input is optional: with none
// submitted no owner reaches runtime data and the management provider falls back to the caller.
//
// The input is a USER_SELECT, so the client sources the candidates itself. This executor declares
// the input and verifies what comes back.
type ownerResolver struct {
	providers.Executor
	entityProvider entityprovider.EntityProviderInterface
	logger         *log.Logger
}

var _ providers.Executor = (*ownerResolver)(nil)

// newOwnerResolver creates a new instance of the OwnerResolver executor.
func newOwnerResolver(
	flowFactory core.FlowFactoryInterface,
	entityProvider entityprovider.EntityProviderInterface,
) *ownerResolver {
	logger := log.GetLogger().With(
		log.String(log.LoggerKeyComponentName, ownerResolverLoggerComponentName),
		log.String(log.LoggerKeyExecutorName, ExecutorNameOwnerResolver))

	defaultInputs := []providers.Input{
		{
			Ref:        "owner_input",
			Identifier: ownerKey,
			Type:       providers.InputTypeUserSelect,
			Required:   false,
		},
	}

	base := flowFactory.CreateExecutor(ExecutorNameOwnerResolver, providers.ExecutorTypeUtility,
		defaultInputs, []providers.Input{}, &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{
				providers.FlowTypeAdministration,
				providers.FlowTypeRegistration,
			},
		})

	return &ownerResolver{
		Executor:       base,
		entityProvider: entityProvider,
		logger:         logger,
	}
}

// Execute resolves the owner from the submitted selection, or offers the choice once.
func (e *ownerResolver) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		ForwardedData:  make(map[string]interface{}),
	}

	if selected, ok := ctx.UserInputs[ownerKey]; ok && selected != "" {
		return e.resolveSelectedOwner(ctx, execResp, selected, logger)
	}

	// The choice was already offered and declined. Completing without an owner is a valid
	// outcome, the provider filling it with the caller.
	if core.IsOptionalInputPrompted(core.GetPresentedOptionalInputs(ctx.RuntimeData), ownerKey) {
		logger.Debug(ctx.Context, "No owner selected, leaving it to the authenticated caller")
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}

	return e.promptOwnerSelection(ctx, execResp, logger)
}

// resolveSelectedOwner verifies the submitted owner and records it for the provisioning node.
//
// Entity lookups are not scoped by category, so the category is checked rather than assumed. An
// identifier of another category is reported the same way a missing one is, which keeps the
// response from disclosing what it does name.
func (e *ownerResolver) resolveSelectedOwner(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse, selected string, logger *log.Logger,
) (*providers.ExecutorResponse, error) {
	entity, epErr := e.entityProvider.GetEntity(selected)
	if epErr != nil {
		if epErr.Code == entityprovider.ErrorCodeEntityNotFound {
			return e.rejectOwner(ctx, execResp, "Selected owner does not exist", logger), nil
		}
		return nil, fmt.Errorf("failed to resolve the selected owner: %s", epErr.Error())
	}
	if entity == nil || entity.Category != providers.EntityCategoryUser {
		return e.rejectOwner(ctx, execResp, "Selected owner is not a user", logger), nil
	}

	logger.Debug(ctx.Context, "Owner resolved from selection", log.MaskedString(ownerKey, entity.ID))
	execResp.RuntimeData[ownerIDKey] = entity.ID
	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// rejectOwner offers the choice again rather than failing the flow.
func (e *ownerResolver) rejectOwner(ctx *providers.NodeContext, execResp *providers.ExecutorResponse,
	reason string, logger *log.Logger,
) *providers.ExecutorResponse {
	logger.Debug(ctx.Context, reason)
	execResp.Status = providers.ExecUserInputRequired
	execResp.Inputs = e.GetDefaultInputs()
	execResp.ForwardedData[common.ForwardedDataKeyInputs] = execResp.Inputs
	execResp.Error = &ErrOwnerNotFound

	return execResp
}

// promptOwnerSelection offers the choice once, carrying the input alone since the client fetches
// the candidates.
func (e *ownerResolver) promptOwnerSelection(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse, logger *log.Logger,
) (*providers.ExecutorResponse, error) {
	logger.Debug(ctx.Context, "Prompting for owner selection")
	execResp.Status = providers.ExecUserInputRequired
	execResp.Inputs = e.GetDefaultInputs()
	execResp.ForwardedData[common.ForwardedDataKeyInputs] = execResp.Inputs

	return execResp, nil
}
