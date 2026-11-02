// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// userTypeResolver is a registration-flow executor that resolves the user type at flow start.
type userTypeResolver struct {
	providers.Executor
	*entityTypeResolution
	logger *log.Logger
}

var _ providers.Executor = (*userTypeResolver)(nil)

// newUserTypeResolver creates a new instance of the UserTypeResolver executor.
func newUserTypeResolver(
	flowFactory core.FlowFactoryInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	ouService providers.OrganizationUnitProvider,
) *userTypeResolver {
	logger := log.GetLogger().With(
		log.String(log.LoggerKeyComponentName, "UserTypeResolver"),
		log.String(log.LoggerKeyExecutorName, ExecutorNameUserTypeResolver))

	defaultInputs := []providers.Input{
		{
			Ref:        "usertype_input",
			Identifier: userTypeKey,
			Type:       providers.InputTypeSelect,
			Required:   true,
		},
	}

	base := flowFactory.CreateExecutor(ExecutorNameUserTypeResolver, providers.ExecutorTypeRegistration,
		defaultInputs, []providers.Input{}, &providers.ExecutorMeta{
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: propertyKeyAllowedUserTypes},
			},
		})

	return &userTypeResolver{
		Executor: base,
		entityTypeResolution: &entityTypeResolution{
			base:              base,
			entityTypeService: entityTypeService,
			ouService:         ouService,
		},
		logger: logger,
	}
}

// Execute resolves the user type from inputs or prompts the user to select one.
func (u *userTypeResolver) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := u.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing user type resolver")

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		ForwardedData:  make(map[string]interface{}),
	}

	switch ctx.FlowType {
	case providers.FlowTypeAuthentication:
		return u.resolveAuthFlow(ctx, entitytype.TypeCategoryUser, execResp), nil
	case providers.FlowTypeRegistration:
		return u.resolveRegistrationFlow(ctx, entitytype.TypeCategoryUser, execResp, logger)
	case providers.FlowTypeUserOnboarding:
		return u.resolveOnboardingFlow(ctx, entitytype.TypeCategoryUser, execResp, logger)
	default:
		logger.Debug(ctx.Context, "User type resolver is not applicable for the flow type",
			log.String("flowType", string(ctx.FlowType)))
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}
}
