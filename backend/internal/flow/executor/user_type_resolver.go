// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"slices"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	userTypeResolverLoggerComponentName = "UserTypeResolver"
)

// entityTypeWithOU represents an entity type along with its associated organization unit ID.
type entityTypeWithOU struct {
	entityType *entitytype.EntityType
	ouID       string
}

// userTypeResolver is a registration-flow executor that resolves the user type at flow start.
type userTypeResolver struct {
	providers.Executor
	entityTypeResolutionInterface
	logger *log.Logger
}

var _ providers.Executor = (*userTypeResolver)(nil)
var _ entityTypeResolutionInterface = (*userTypeResolver)(nil)

// newUserTypeResolver creates a new instance of the UserTypeResolver executor.
func newUserTypeResolver(
	flowFactory core.FlowFactoryInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	ouService ou.OrganizationUnitServiceInterface,
) *userTypeResolver {
	logger := log.GetLogger().With(
		log.String(log.LoggerKeyComponentName, userTypeResolverLoggerComponentName),
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
		entityTypeResolutionInterface: &entityTypeResolution{
			entityTypeService: entityTypeService,
			ouService:         ouService,
		},
		logger: logger,
	}
}

// getEntityTypeAndOU reads a user type and the organization unit it lives in.
func (u *userTypeResolver) getEntityTypeAndOU(
	ctx context.Context, userType string,
) (*entitytype.EntityType, string, error) {
	return u.entityTypeAndOU(ctx, entitytype.TypeCategoryUser, userType, u.logger)
}

// promptUserSelection offers the available user types as a choice.
func (u *userTypeResolver) promptUserSelection(
	ctx context.Context, execResp *providers.ExecutorResponse, options []string) {
	u.promptOptions(ctx, execResp, options, u.GetDefaultInputs(), u.logger)
}

// getAllowedUserTypesFromProperties reads the optional allowedUserTypes node property.
func (u *userTypeResolver) getAllowedUserTypesFromProperties(ctx *providers.NodeContext) []string {
	return allowedTypesFromProperties(ctx, propertyKeyAllowedUserTypes)
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
		return u.handleAuthenticationFlows(ctx, execResp)
	case providers.FlowTypeRegistration:
		return u.handleRegistrationFlows(ctx, execResp)
	case providers.FlowTypeUserOnboarding:
		return u.handleUserOnboardingFlows(ctx, execResp)
	default:
		logger.Debug(ctx.Context, "User type resolver is not applicable for the flow type",
			log.String("flowType", string(ctx.FlowType)))
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}
}

// handleAuthenticationFlows handles user type resolution for authentication flows.
func (u *userTypeResolver) handleAuthenticationFlows(ctx *providers.NodeContext, execResp *providers.ExecutorResponse) (
	*providers.ExecutorResponse, error) {
	logger := u.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	// Validate that allowed user types are defined
	if len(ctx.Application.AllowedUserTypes) == 0 {
		logger.Debug(ctx.Context, "No allowed user types configured for authentication")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrAuthNotAvailableForApp
		return execResp, nil
	}

	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// handleRegistrationFlows handles user type resolution for registration flows.
func (u *userTypeResolver) handleRegistrationFlows(ctx *providers.NodeContext, execResp *providers.ExecutorResponse) (
	*providers.ExecutorResponse, error) {
	reqCtx := ctx.Context
	logger := u.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	allowed := ctx.Application.AllowedUserTypes

	// Check for allowed user types to decide next steps
	if len(allowed) == 0 {
		// TODO: This should be improved to fallback to the application's ou when the support is available.
		//  userType has an attached ou. Need to find userType from the application's ou.
		//  Also should check if self registration is enabled for the user type when the support is available.

		logger.Debug(ctx.Context, "No allowed user types found for the application")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrSelfRegNotAvailableForApp
		return execResp, nil
	}

	// If allowedUserTypes is configured in node properties, filter the application-level allowed list
	nodeAllowedUserTypes := u.getAllowedUserTypesFromProperties(ctx)
	if len(nodeAllowedUserTypes) > 0 {
		filtered := make([]string, 0, len(allowed))
		for _, userType := range allowed {
			if slices.Contains(nodeAllowedUserTypes, userType) {
				filtered = append(filtered, userType)
			}
		}

		if len(filtered) == 0 {
			logger.Debug(ctx.Context, "No valid user types after filtering with node allowedUserTypes",
				log.Any("applicationAllowed", allowed), log.Any("nodeAllowed", nodeAllowedUserTypes))
			execResp.Status = providers.ExecFailure
			execResp.Error = &ErrNoValidUserTypes
			return execResp, nil
		}

		allowed = filtered
	}

	// Check if userType is provided in inputs
	if u.HasRequiredInputs(ctx, execResp) {
		err := u.resolveUserTypeFromInput(reqCtx, execResp, ctx.UserInputs[userTypeKey], allowed)
		return execResp, err
	}

	// If only one allowed user type, select it automatically
	if len(allowed) == 1 {
		err := u.resolveUserTypeFromSingleAllowed(reqCtx, execResp, allowed[0])
		return execResp, err
	}

	// If multiple allowed user types, prompt the user to select one
	err := u.resolveUserTypeFromMultipleAllowed(reqCtx, execResp, allowed)

	return execResp, err
}

// handleUserOnboardingFlows handles user type resolution for user onboarding flows.
func (u *userTypeResolver) handleUserOnboardingFlows(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) (*providers.ExecutorResponse, error) {
	logger := u.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	return u.resolve(ctx, entitytype.TypeCategoryUser,
		u.getAllowedUserTypesFromProperties(ctx), u.GetDefaultInputs(), execResp, logger)
}

// resolveUserTypeFromInput resolves the user type from input and updates the executor response.
func (u *userTypeResolver) resolveUserTypeFromInput(ctx context.Context, execResp *providers.ExecutorResponse,
	userType string, allowed []string) error {
	logger := u.logger
	if slices.Contains(allowed, userType) {
		logger.Debug(ctx, "User type resolved from input", log.String(categoryTypeKey, userType))

		entityType, ouID, err := u.getEntityTypeAndOU(ctx, userType)
		if err != nil {
			return err
		}
		if !entityType.AllowSelfRegistration {
			logger.Debug(ctx, "Self registration not enabled for user type",
				log.String(categoryTypeKey, userType))
			execResp.Status = providers.ExecFailure
			execResp.Error = &ErrSelfRegDisabledForUserType
			return nil
		}

		// Add userType and ouID to runtime data
		execResp.RuntimeData[categoryTypeKey] = userType
		execResp.RuntimeData[defaultOUIDKey] = ouID

		execResp.Status = providers.ExecComplete
		return nil
	}

	execResp.Status = providers.ExecFailure
	execResp.Error = &ErrUserTypeNotAllowed
	return nil
}

// resolveUserTypeFromSingleAllowed resolves the user type when there is only a single allowed user type.
func (u *userTypeResolver) resolveUserTypeFromSingleAllowed(ctx context.Context, execResp *providers.ExecutorResponse,
	allowedUserType string) error {
	logger := u.logger
	entityType, ouID, err := u.getEntityTypeAndOU(ctx, allowedUserType)
	if err != nil {
		return err
	}

	if !entityType.AllowSelfRegistration {
		logger.Debug(ctx, "Self registration not enabled for user type",
			log.String(categoryTypeKey, allowedUserType))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrSelfRegDisabledForUserType
		return nil
	}

	logger.Debug(ctx, "User type resolved from allowed list", log.String(categoryTypeKey, allowedUserType))

	// Add userType and ouID to runtime data
	execResp.RuntimeData[categoryTypeKey] = allowedUserType
	execResp.RuntimeData[defaultOUIDKey] = ouID

	execResp.Status = providers.ExecComplete
	return nil
}

// resolveUserTypeFromMultipleAllowed resolves the user type when multiple allowed user types exist.
func (u *userTypeResolver) resolveUserTypeFromMultipleAllowed(ctx context.Context, execResp *providers.ExecutorResponse,
	allowed []string) error {
	logger := u.logger

	// Filter self registration enabled user types
	selfRegEnabledUserTypes := make([]entityTypeWithOU, 0)
	for _, userType := range allowed {
		entityType, ouID, err := u.getEntityTypeAndOU(ctx, userType)
		if err != nil {
			return err
		}
		if entityType.AllowSelfRegistration {
			selfRegEnabledUserTypes = append(selfRegEnabledUserTypes, entityTypeWithOU{
				entityType: entityType,
				ouID:       ouID,
			})
		}
	}

	// Fail if no user types have self registration enabled
	if len(selfRegEnabledUserTypes) == 0 {
		logger.Debug(ctx, "No user types with self registration enabled")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrSelfRegNotAvailableForApp
		return nil
	}

	// If only one user type has self registration enabled, select it automatically
	if len(selfRegEnabledUserTypes) == 1 {
		record := selfRegEnabledUserTypes[0]
		logger.Debug(ctx, "User type auto-selected", log.String(categoryTypeKey, record.entityType.Name))

		// Add userType and ouID to runtime data
		execResp.RuntimeData[categoryTypeKey] = record.entityType.Name
		execResp.RuntimeData[defaultOUIDKey] = record.ouID

		execResp.Status = providers.ExecComplete
		return nil
	}

	// If multiple user types are allowed, prompt the user to select one
	selfRegUserTypes := make([]string, 0, len(selfRegEnabledUserTypes))
	for _, record := range selfRegEnabledUserTypes {
		selfRegUserTypes = append(selfRegUserTypes, record.entityType.Name)
	}

	logger.Debug(ctx,
		"Prompting for user type selection as multiple user types are available for self registration",
		log.Any("userTypes", selfRegUserTypes))

	u.promptUserSelection(ctx, execResp, selfRegUserTypes)
	return nil
}
