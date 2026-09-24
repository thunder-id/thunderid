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
	entityTypeResolutionInterface
	logger *log.Logger
}

var _ providers.Executor = (*agentTypeResolver)(nil)
var _ entityTypeResolutionInterface = (*agentTypeResolver)(nil)

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
			SupportedFlowTypes: []providers.FlowType{
				providers.FlowTypeAdministration,
				providers.FlowTypeAuthentication,
			},
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: propertyKeyAllowedAgentTypes},
			},
		})

	return &agentTypeResolver{
		Executor: base,
		entityTypeResolutionInterface: &entityTypeResolution{
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
		return a.handleAuthenticationFlows(ctx, execResp)
	case providers.FlowTypeAdministration:
		return a.handleAdministrationFlows(ctx, execResp)
	case providers.FlowTypeRegistration:
		return a.handleRegistrationFlows(ctx, execResp)
	default:
		logger.Debug(ctx.Context, "Agent type resolver is not applicable for the flow type",
			log.String("flowType", string(ctx.FlowType)))
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}
}

// handleAuthenticationFlows gates authentication on the agent types the application accepts. An
// empty list means the application admits no agent at all, which is the same shape as the user
// gate and what the allowedAgentTypes contract states.
func (a *agentTypeResolver) handleAuthenticationFlows(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) (*providers.ExecutorResponse, error) {
	logger := a.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	if len(ctx.Application.AllowedAgentTypes) == 0 {
		logger.Debug(ctx.Context, "No allowed agent types configured for authentication")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrAuthNotAvailableForApp

		return execResp, nil
	}

	execResp.Status = providers.ExecComplete

	return execResp, nil
}

// handleAdministrationFlows resolves the agent type for an administrator-driven flow. The
// candidates come from the agent type catalog, narrowed by the node's allowedAgentTypes property
// and by the selected organization unit. The application's allowedAgentTypes is deliberately not
// consulted: it governs which agent types may authenticate to an application, not which an
// administrator may create.
func (a *agentTypeResolver) handleAdministrationFlows(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) (*providers.ExecutorResponse, error) {
	logger := a.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	return a.resolve(ctx, entitytype.TypeCategoryAgent,
		allowedTypesFromProperties(ctx, propertyKeyAllowedAgentTypes), a.GetDefaultInputs(),
		execResp, logger)
}

// handleRegistrationFlows refuses self-registration. An agent is provisioned on an
// administrator's behalf, so no agent type carries a self-registration path the way a user type
// can. Should agents ever self-register, the resolution belongs here.
func (a *agentTypeResolver) handleRegistrationFlows(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) (*providers.ExecutorResponse, error) {
	logger := a.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	logger.Debug(ctx.Context, "Agent self-registration attempted, which agents do not support")
	execResp.Status = providers.ExecFailure
	execResp.Error = &ErrSelfRegNotSupportedForAgents

	return execResp, nil
}
