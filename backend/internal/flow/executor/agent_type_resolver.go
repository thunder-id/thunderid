// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// agentTypeResolver resolves the agent type to provision as, publishing it and the organization
// unit that type lives in.
//
// The flow types it serves are declared rather than checked at runtime, so the flow validator
// rejects a node placed elsewhere.
type agentTypeResolver struct {
	providers.Executor
	*entityTypeResolution
	logger *log.Logger
}

var _ providers.Executor = (*agentTypeResolver)(nil)

// newAgentTypeResolver creates a new instance of the AgentTypeResolver executor.
func newAgentTypeResolver(
	flowFactory core.FlowFactoryInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	ouService providers.OrganizationUnitProvider,
) *agentTypeResolver {
	logger := log.GetLogger().With(
		log.String(log.LoggerKeyComponentName, "AgentTypeResolver"),
		log.String(log.LoggerKeyExecutorName, ExecutorNameAgentTypeResolver))

	defaultInputs := []providers.Input{
		{
			Ref:        "agenttype_input",
			Identifier: agentTypeKey,
			Type:       providers.InputTypeSelect,
			Required:   true,
		},
	}

	base := flowFactory.CreateExecutor(ExecutorNameAgentTypeResolver, providers.ExecutorTypeUtility,
		defaultInputs, []providers.Input{}, &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{
				providers.FlowTypeAdministration,
				providers.FlowTypeAuthentication,
				providers.FlowTypeRegistration,
			},
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: propertyKeyAllowedAgentTypes},
			},
		})

	return &agentTypeResolver{
		Executor: base,
		entityTypeResolution: &entityTypeResolution{
			base:              base,
			entityTypeService: entityTypeService,
			ouService:         ouService,
		},
		logger: logger,
	}
}

// Execute resolves the agent type for the flow type in play.
func (a *agentTypeResolver) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := a.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing agent type resolver")

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		ForwardedData:  make(map[string]interface{}),
	}

	switch ctx.FlowType {
	case providers.FlowTypeAuthentication:
		return a.resolveAuthFlow(ctx, entitytype.TypeCategoryAgent, execResp), nil
	case providers.FlowTypeAdministration, providers.FlowTypeRegistration:
		return a.resolveOnboardingFlow(ctx, entitytype.TypeCategoryAgent, execResp, logger)
	default:
		logger.Debug(ctx.Context, "Agent type resolver is not applicable for the flow type",
			log.String("flowType", string(ctx.FlowType)))
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}
}
