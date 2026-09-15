// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const agentTypeResolverLoggerComponentName = "AgentTypeResolver"

// agentTypeResolver resolves the agent type to provision as, publishing it and the organization
// unit that type lives in.
//
// The flow types it serves are declared rather than checked at runtime, so the flow validator
// rejects a node placed elsewhere.
type agentTypeResolver struct {
	providers.Executor
	resolution *entityTypeResolution
	logger     *log.Logger
}

var _ providers.Executor = (*agentTypeResolver)(nil)

// newAgentTypeResolver creates a new instance of the AgentTypeResolver executor.
func newAgentTypeResolver(
	flowFactory core.FlowFactoryInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	ouService ou.OrganizationUnitServiceInterface,
) *agentTypeResolver {
	logger := log.GetLogger().With(
		log.String(log.LoggerKeyComponentName, agentTypeResolverLoggerComponentName),
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
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration},
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: propertyKeyAllowedAgentTypes},
			},
		})

	return &agentTypeResolver{
		Executor: base,
		resolution: &entityTypeResolution{
			entityTypeService: entityTypeService,
			ouService:         ouService,
		},
		logger: logger,
	}
}

// Execute resolves the agent type from inputs, or offers the choice when there is one.
func (a *agentTypeResolver) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := a.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing agent type resolver")

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		ForwardedData:  make(map[string]interface{}),
	}

	return a.resolution.resolve(ctx, entitytype.TypeCategoryAgent,
		allowedTypesFromProperties(ctx, propertyKeyAllowedAgentTypes), a.GetDefaultInputs(),
		execResp, logger)
}
