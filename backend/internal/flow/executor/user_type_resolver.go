/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package executor

import (
	"context"
	"fmt"
	"slices"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
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
	core.ExecutorInterface
	entityTypeService entitytype.EntityTypeServiceInterface
	ouService         ou.OrganizationUnitServiceInterface
	logger            *log.Logger
}

var _ core.ExecutorInterface = (*userTypeResolver)(nil)

// newUserTypeResolver creates a new instance of the UserTypeResolver executor.
func newUserTypeResolver(
	flowFactory core.FlowFactoryInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	ouService ou.OrganizationUnitServiceInterface,
) *userTypeResolver {
	logger := log.GetLogger().With(
		log.String(log.LoggerKeyComponentName, userTypeResolverLoggerComponentName),
		log.String(log.LoggerKeyExecutorName, ExecutorNameUserTypeResolver))

	defaultInputs := []common.Input{
		{
			Ref:        "usertype_input",
			Identifier: userTypeKey,
			Type:       "SELECT",
			Required:   true,
		},
	}

	base := flowFactory.CreateExecutor(ExecutorNameUserTypeResolver, common.ExecutorTypeRegistration,
		defaultInputs, []common.Input{})

	return &userTypeResolver{
		ExecutorInterface: base,
		entityTypeService: entityTypeService,
		ouService:         ouService,
		logger:            logger,
	}
}

// Execute resolves the user type from inputs or prompts the user to select one.
func (u *userTypeResolver) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := u.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Executing user type resolver")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		ForwardedData:  make(map[string]interface{}),
	}

	switch ctx.FlowType {
	case common.FlowTypeAuthentication:
		return u.handleAuthenticationFlows(ctx, execResp)
	case common.FlowTypeRegistration:
		return u.handleRegistrationFlows(ctx, execResp)
	case common.FlowTypeUserOnboarding:
		return u.handleUserOnboardingFlows(ctx, execResp)
	default:
		logger.Debug("User type resolver is not applicable for the flow type",
			log.String("flowType", string(ctx.FlowType)))
		execResp.Status = common.ExecComplete
		return execResp, nil
	}
}

// handleAuthenticationFlows handles user type resolution for authentication flows.
func (u *userTypeResolver) handleAuthenticationFlows(ctx *core.NodeContext, execResp *common.ExecutorResponse) (
	*common.ExecutorResponse, error) {
	logger := u.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	// Validate that allowed user types are defined
	if len(ctx.Application.AllowedUserTypes) == 0 {
		logger.Debug("No allowed user types configured for authentication")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Authentication not available for this application"
		return execResp, nil
	}

	execResp.Status = common.ExecComplete
	return execResp, nil
}

// handleRegistrationFlows handles user type resolution for registration flows.
func (u *userTypeResolver) handleRegistrationFlows(ctx *core.NodeContext, execResp *common.ExecutorResponse) (
	*common.ExecutorResponse, error) {
	reqCtx := ctx.Context
	logger := u.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	allowed := ctx.Application.AllowedUserTypes

	// Check for allowed user types to decide next steps
	if len(allowed) == 0 {
		// TODO: This should be improved to fallback to the application's ou when the support is available.
		//  userType has an attached ou. Need to find userType from the application's ou.
		//  Also should check if self registration is enabled for the user type when the support is available.

		logger.Debug("No allowed user types found for the application")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Self-registration not available for this application"
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
			logger.Debug("No valid user types after filtering with node allowedUserTypes",
				log.Any("applicationAllowed", allowed), log.Any("nodeAllowed", nodeAllowedUserTypes))
			execResp.Status = common.ExecFailure
			execResp.FailureReason = "No valid user types available for this flow"
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
func (u *userTypeResolver) handleUserOnboardingFlows(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) (*common.ExecutorResponse, error) {
	logger := u.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	// Read optional allowedUserTypes from node properties
	allowedUserTypes := u.getAllowedUserTypesFromProperties(ctx)

	// If userType already provided, validate and set runtime data
	if userType, ok := ctx.UserInputs[userTypeKey]; ok && userType != "" {
		// If allowedUserTypes is configured, validate the input against it
		if len(allowedUserTypes) > 0 && !slices.Contains(allowedUserTypes, userType) {
			logger.Debug("User type not in allowed list", log.String(userTypeKey, userType),
				log.Any("allowedUserTypes", allowedUserTypes))
			execResp.Status = common.ExecFailure
			execResp.FailureReason = "User type not allowed for this flow"
			return execResp, nil
		}

		entityType, ouID, err := u.getEntityTypeAndOU(ctx.Context, userType)
		if err != nil {
			execResp.Status = common.ExecFailure
			execResp.FailureReason = "Invalid user type"
			return execResp, nil
		}

		// If an OU was already selected (OU-first onboarding flow), validate the user type is valid for that OU.
		if selectedOUID, exists := ctx.RuntimeData[ouIDKey]; exists && selectedOUID != "" {
			isValid, svcErr := u.ouService.IsParent(ctx.Context, ouID, selectedOUID)
			if svcErr != nil {
				logger.Error("Failed to validate user type against selected OU",
					log.String(userTypeKey, userType), log.String(ouIDKey, selectedOUID),
					log.String("error", svcErr.Error.DefaultValue))
				return nil, fmt.Errorf("failed to validate user type against selected OU: %s",
					svcErr.Error.DefaultValue)
			}
			if !isValid {
				logger.Debug("User type not valid for selected OU",
					log.String(userTypeKey, userType), log.String(ouIDKey, selectedOUID))
				execResp.Status = common.ExecFailure
				execResp.FailureReason = "User type is not valid for the selected organization unit"
				return execResp, nil
			}
		}

		execResp.RuntimeData[userTypeKey] = userType
		execResp.RuntimeData[defaultOUIDKey] = ouID
		logger.Debug("User type resolved for user onboarding", log.String(userTypeKey, userType),
			log.String(ouIDKey, entityType.OUID))
		execResp.Status = common.ExecComplete
		return execResp, nil
	}

	// List all available user types
	schemas, svcErr := u.entityTypeService.GetEntityTypeList(ctx.Context,
		entitytype.TypeCategoryUser, 100, 0, false)
	if svcErr != nil {
		logger.Debug("Failed to list user types", log.String("error", svcErr.Error.DefaultValue))
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Failed to retrieve user types"
		return execResp, nil
	}

	if len(schemas.Types) == 0 {
		logger.Debug("No user types available")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "No user types available"
		return execResp, nil
	}

	// Build the list of available schema names, filtering by allowedUserTypes if configured
	availableSchemas := u.filterSchemasByAllowedTypes(schemas.Types, allowedUserTypes)

	// If an OU was already selected (OU-first onboarding flow), filter schemas to those valid for that OU.
	// This only applies to USER_ONBOARDING flows where OUResolver with "promptAll" runs first.
	// Registration flows derive the OU from the user type's schema, so ouId is never in RuntimeData.
	if selectedOUID, exists := ctx.RuntimeData[ouIDKey]; exists && selectedOUID != "" {
		var err error
		availableSchemas, err = u.filterSchemasByOU(ctx, availableSchemas, selectedOUID, logger)
		if err != nil {
			return nil, err
		}
	}

	if len(availableSchemas) == 0 {
		logger.Debug("No valid user types found after filtering",
			log.Any("allowedUserTypes", allowedUserTypes))
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "No valid user types available for this flow"
		return execResp, nil
	}

	// If only one user type is available, select it automatically
	if len(availableSchemas) == 1 {
		schema := availableSchemas[0]
		logger.Debug("User type auto-selected for user onboarding", log.String(userTypeKey, schema.Name),
			log.String(ouIDKey, schema.OUID))

		execResp.RuntimeData[userTypeKey] = schema.Name
		execResp.RuntimeData[defaultOUIDKey] = schema.OUID
		execResp.Status = common.ExecComplete
		return execResp, nil
	}

	options := make([]string, 0, len(availableSchemas))
	for _, schema := range availableSchemas {
		options = append(options, schema.Name)
	}

	u.promptUserSelection(execResp, options)
	return execResp, nil
}

// getAllowedUserTypesFromProperties reads the optional allowedUserTypes property from node properties.
func (u *userTypeResolver) getAllowedUserTypesFromProperties(ctx *core.NodeContext) []string {
	if ctx.NodeProperties == nil {
		return nil
	}

	val, exists := ctx.NodeProperties[propertyKeyAllowedUserTypes]
	if !exists {
		return nil
	}

	items, ok := val.([]interface{})
	if !ok {
		u.logger.Debug("allowedUserTypes property is not a valid array")
		return nil
	}

	userTypes := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok && s != "" {
			userTypes = append(userTypes, s)
		}
	}

	if len(userTypes) > 0 {
		u.logger.Debug("Allowed user types configured from node properties",
			log.Any("allowedUserTypes", userTypes))
	}

	return userTypes
}

// filterSchemasByAllowedTypes filters schemas by the allowedUserTypes list.
// If allowedUserTypes is empty, all schemas are returned.
func (u *userTypeResolver) filterSchemasByAllowedTypes(
	schemas []entitytype.EntityTypeListItem, allowedUserTypes []string,
) []entitytype.EntityTypeListItem {
	if len(allowedUserTypes) == 0 {
		return schemas
	}

	filtered := make([]entitytype.EntityTypeListItem, 0, len(schemas))
	for _, schema := range schemas {
		if slices.Contains(allowedUserTypes, schema.Name) {
			filtered = append(filtered, schema)
		}
	}

	return filtered
}

// filterSchemasByOU filters schemas to only those valid for the given OU.
// A schema is valid if its OUID is an ancestor of (or equal to) the selected OU.
func (u *userTypeResolver) filterSchemasByOU(ctx *core.NodeContext,
	schemas []entitytype.EntityTypeListItem, selectedOUID string, logger *log.Logger,
) ([]entitytype.EntityTypeListItem, error) {
	filtered := make([]entitytype.EntityTypeListItem, 0, len(schemas))
	for _, schema := range schemas {
		isValid, svcErr := u.ouService.IsParent(ctx.Context, schema.OUID, selectedOUID)
		if svcErr != nil {
			logger.Error("Failed to check OU ancestry for schema",
				log.String("schema", schema.Name), log.String("error", svcErr.Error.DefaultValue))
			return nil, fmt.Errorf("failed to check OU ancestry for schema %s: %s",
				schema.Name, svcErr.Error.DefaultValue)
		}
		if isValid {
			filtered = append(filtered, schema)
		}
	}

	logger.Debug("Filtered schemas by selected OU",
		log.String(ouIDKey, selectedOUID),
		log.Int("before", len(schemas)),
		log.Int("after", len(filtered)))

	return filtered, nil
}

// resolveUserTypeFromInput resolves the user type from input and updates the executor response.
func (u *userTypeResolver) resolveUserTypeFromInput(ctx context.Context, execResp *common.ExecutorResponse,
	userType string, allowed []string) error {
	logger := u.logger
	if slices.Contains(allowed, userType) {
		logger.Debug("User type resolved from input", log.String(userTypeKey, userType))

		entityType, ouID, err := u.getEntityTypeAndOU(ctx, userType)
		if err != nil {
			return err
		}
		if !entityType.AllowSelfRegistration {
			logger.Debug("Self registration not enabled for user type", log.String(userTypeKey, userType))
			execResp.Status = common.ExecFailure
			execResp.FailureReason = "Self-registration not enabled for the user type"
			return nil
		}

		// Add userType and ouID to runtime data
		execResp.RuntimeData[userTypeKey] = userType
		execResp.RuntimeData[defaultOUIDKey] = ouID

		execResp.Status = common.ExecComplete
		return nil
	}

	execResp.Status = common.ExecFailure
	execResp.FailureReason = "Application does not allow registration for the user type"
	return nil
}

// resolveUserTypeFromSingleAllowed resolves the user type when there is only a single allowed user type.
func (u *userTypeResolver) resolveUserTypeFromSingleAllowed(ctx context.Context, execResp *common.ExecutorResponse,
	allowedUserType string) error {
	logger := u.logger
	entityType, ouID, err := u.getEntityTypeAndOU(ctx, allowedUserType)
	if err != nil {
		return err
	}

	if !entityType.AllowSelfRegistration {
		logger.Debug("Self registration not enabled for user type", log.String(userTypeKey, allowedUserType))
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Self-registration not enabled for the user type"
		return nil
	}

	logger.Debug("User type resolved from allowed list", log.String(userTypeKey, allowedUserType))

	// Add userType and ouID to runtime data
	execResp.RuntimeData[userTypeKey] = allowedUserType
	execResp.RuntimeData[defaultOUIDKey] = ouID

	execResp.Status = common.ExecComplete
	return nil
}

// resolveUserTypeFromMultipleAllowed resolves the user type when multiple allowed user types exist.
func (u *userTypeResolver) resolveUserTypeFromMultipleAllowed(ctx context.Context, execResp *common.ExecutorResponse,
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
		logger.Debug("No user types with self registration enabled")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Self-registration not available for this application"
		return nil
	}

	// If only one user type has self registration enabled, select it automatically
	if len(selfRegEnabledUserTypes) == 1 {
		record := selfRegEnabledUserTypes[0]
		logger.Debug("User type auto-selected", log.String(userTypeKey, record.entityType.Name))

		// Add userType and ouID to runtime data
		execResp.RuntimeData[userTypeKey] = record.entityType.Name
		execResp.RuntimeData[defaultOUIDKey] = record.ouID

		execResp.Status = common.ExecComplete
		return nil
	}

	// If multiple user types are allowed, prompt the user to select one
	selfRegUserTypes := make([]string, 0, len(selfRegEnabledUserTypes))
	for _, record := range selfRegEnabledUserTypes {
		selfRegUserTypes = append(selfRegUserTypes, record.entityType.Name)
	}

	logger.Debug("Prompting for user type selection as multiple user types are available for self registration",
		log.Any("userTypes", selfRegUserTypes))

	u.promptUserSelection(execResp, selfRegUserTypes)
	return nil
}

// getEntityTypeAndOU retrieves the entity type by name and returns the entity type and organization unit ID.
func (u *userTypeResolver) getEntityTypeAndOU(
	ctx context.Context, userType string,
) (*entitytype.EntityType, string, error) {
	logger := u.logger.With(log.String(userTypeKey, userType))

	entityType, svcErr := u.entityTypeService.GetEntityTypeByName(ctx, entitytype.TypeCategoryUser, userType)
	if svcErr != nil {
		logger.Error("Failed to resolve user type",
			log.String(userTypeKey, userType), log.String("error", svcErr.Error.DefaultValue))
		return nil, "", fmt.Errorf("failed to resolve user type: %s", userType)
	}

	if entityType.OUID == "" {
		logger.Error("No organization unit found for user type", log.String(userTypeKey, userType))
		return nil, "", fmt.Errorf("no organization unit found for user type: %s", userType)
	}

	logger.Debug("Entity type resolved for user type", log.String(userTypeKey, userType),
		log.String(ouIDKey, entityType.OUID))
	return entityType, entityType.OUID, nil
}

// promptUserSelection prompts the user to select a user type from the provided options.
func (u *userTypeResolver) promptUserSelection(execResp *common.ExecutorResponse, options []string) {
	u.logger.Debug("Prompting user for user type selection", log.Any("userTypes", options))

	execResp.Status = common.ExecUserInputRequired

	// Use the default input configuration
	inputs := u.GetDefaultInputs()
	if len(inputs) > 0 {
		input := inputs[0]
		input.Options = options
		execResp.Inputs = []common.Input{input}

		// Forward the input with options to the next node
		execResp.ForwardedData[common.ForwardedDataKeyInputs] = execResp.Inputs
	}
}
