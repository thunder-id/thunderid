// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const (
	ouExecLoggerComponentName = "OUExecutor"
	ouCompensationTimeout     = 10 * time.Second
)

// ouExecutor is responsible for creating organizational units (OUs) within the system.
type ouExecutor struct {
	providers.Executor
	ouService         providers.OrganizationUnitProvider
	authnProvider     providers.AuthnProviderManager
	entityTypeService entitytype.EntityTypeServiceInterface
	logger            *log.Logger
}

var _ core.CompensatingExecutorInterface = (*ouExecutor)(nil)

// newOUExecutor creates a new instance of OUExecutor with the given parameters.
func newOUExecutor(
	flowFactory core.FlowFactoryInterface,
	ouService providers.OrganizationUnitProvider,
	authnProvider providers.AuthnProviderManager,
	entityTypeService entitytype.EntityTypeServiceInterface,
) *ouExecutor {
	defaultInputs := []providers.Input{
		{
			Identifier: userInputOuName,
			Type:       "string",
			Required:   true,
		},
		{
			Identifier: userInputOuHandle,
			Type:       "string",
			Required:   true,
		},
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, ouExecLoggerComponentName),
		log.String(log.LoggerKeyExecutorName, ExecutorNameOUCreation))

	base := flowFactory.CreateExecutor(ExecutorNameOUCreation, providers.ExecutorTypeRegistration,
		defaultInputs, []providers.Input{}, &providers.ExecutorMeta{
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: "parentOuId"},
				{Property: propertyKeyGenerateOUHandle},
			},
		})

	return &ouExecutor{
		Executor:          base,
		ouService:         ouService,
		authnProvider:     authnProvider,
		entityTypeService: entityTypeService,
		logger:            logger,
	}
}

// Execute executes the ou creation logic.
func (o *ouExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := o.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing OU creation executor")

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		AuthUser:       ctx.AuthUser,
	}

	if !o.ValidatePrerequisites(ctx, execResp, o.authnProvider) {
		logger.Debug(ctx.Context, "Prerequisites validation failed for OU creation")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrOUCreationPrereqFailed
		return execResp, nil
	}

	if execResp.AuthUser.IsAuthenticated() {
		// Check if the user already has an entity reference (existing user).
		// If so, skip OU creation as the user already belongs to an OU.
		authUser, entityRef, svcErr := o.authnProvider.GetEntityReference(ctx.Context, execResp.AuthUser)
		if svcErr != nil {
			if svcErr.Code != authnprovidermgr.ErrorUserNotFound.Code &&
				svcErr.Code != authnprovidermgr.ErrorAmbiguousUser.Code {
				execResp.Status = providers.ExecFailure
				execResp.Error = errForEntityCategory(ErrFailedToIdentifyEntity, categoryUnscoped)
				return execResp, nil
			}
			logger.Debug(ctx.Context, "User not found or ambiguous, proceeding with OU creation")
		}
		execResp.AuthUser = authUser
		if entityRef != nil {
			logger.Debug(ctx.Context, "User already has an entity reference, skipping OU creation")
			execResp.Status = providers.ExecComplete
			return execResp, nil
		}
	}

	if !o.hasRequiredInputs(ctx, execResp) {
		logger.Debug(ctx.Context, "Required inputs for OU creation is not provided")
		execResp.Status = providers.ExecUserInputRequired
		return execResp, nil
	}

	handle := ctx.UserInputs[userInputOuHandle]
	if strings.TrimSpace(handle) == "" && isOUHandleGenerationEnabled(ctx) {
		handle = systemutils.GenerateHandle(ctx.UserInputs[userInputOuName])
		if handle == "" {
			logger.Debug(ctx.Context, "Could not generate an organization unit handle from the name")
			execResp.Status = providers.ExecUserInputRequired
			execResp.Inputs = o.GetRequiredInputs(ctx)
			execResp.Error = &ErrOUHandleNotGenerated
			return execResp, nil
		}
	}

	// Create the OU using the OU service.
	ouRequest, err := o.getOrganizationUnitRequest(ctx, handle)
	if err != nil {
		logger.Error(ctx.Context, "Failed to build organization unit request",
			log.String("error", err.Error()))
		return nil, err
	}
	createdOU, svcErr := o.ouService.CreateOrganizationUnit(ctx.Context, ouRequest)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			execResp.Status = providers.ExecUserInputRequired
			execResp.Inputs = o.GetRequiredInputs(ctx)

			switch svcErr.Code {
			case providers.ErrorOrganizationUnitNameConflict.Code:
				execResp.Error = &ErrOUNameConflict
			case providers.ErrorOrganizationUnitHandleConflict.Code:
				execResp.Error = &ErrOUHandleConflict
			default:
				execResp.Error = tidcommon.CustomServiceError(ErrOUCreationFailed, tidcommon.I18nMessage{
					Key:          ErrOUCreationFailed.ErrorDescription.Key,
					DefaultValue: "Failed to create organization unit:" + svcErr.ErrorDescription.DefaultValue,
				})
			}

			return execResp, nil
		}

		logger.Error(ctx.Context, "Error occurred while creating organization unit: ",
			log.String("errorCode", svcErr.Code),
			log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
		return nil, errors.New("failed to create organization unit")
	}

	if createdOU.ID == "" {
		logger.Error(ctx.Context, "Organization unit creation failed: received empty OU ID")
		return nil, errors.New("failed to create organization unit")
	}

	// Set the created OU ID in the runtime data for further use in the flow.
	execResp.RuntimeData[ouIDKey] = createdOU.ID
	execResp.EngineData = map[string]string{common.EngineDataKeyCompensationRef: createdOU.ID}

	logger.Debug(ctx.Context, "Organization unit created successfully", log.String(ouIDKey, createdOU.ID))
	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// Compensate deletes the organization unit the executor created, when the flow it was created in fails.
// It runs with the server's authority and outlives a canceled request, since it undoes the flow's own create.
func (o *ouExecutor) Compensate(ctx context.Context, ref string) error {
	deleteCtx, cancel := context.WithTimeout(
		security.WithRuntimeContext(context.WithoutCancel(ctx)), ouCompensationTimeout)
	defer cancel()
	if svcErr := o.ouService.DeleteOrganizationUnit(deleteCtx, ref); svcErr != nil {
		return fmt.Errorf("failed to delete organization unit: %s", svcErr.Code)
	}
	return nil
}

// hasRequiredInputs checks the required inputs for OU creation. When the node generates the handle,
// a missing handle does not block the node, and the handle is requested as an optional input.
func (o *ouExecutor) hasRequiredInputs(ctx *providers.NodeContext, execResp *providers.ExecutorResponse) bool {
	if !isOUHandleGenerationEnabled(ctx) {
		return o.HasRequiredInputs(ctx, execResp)
	}

	withoutHandle := *ctx
	withoutHandle.NodeInputs = nil
	var optionalHandle []providers.Input
	for _, input := range o.GetRequiredInputs(ctx) {
		if input.Identifier == userInputOuHandle {
			input.Required = false
			optionalHandle = append(optionalHandle, input)
			continue
		}
		withoutHandle.NodeInputs = append(withoutHandle.NodeInputs, input)
	}

	// The base check falls back to the default inputs when the node inputs are empty.
	if len(withoutHandle.NodeInputs) == 0 || o.HasRequiredInputs(&withoutHandle, execResp) {
		return true
	}
	if _, provided := ctx.UserInputs[userInputOuHandle]; !provided {
		execResp.Inputs = append(execResp.Inputs, optionalHandle...)
	}
	return false
}

// isOUHandleGenerationEnabled reports whether the node generates the OU handle from the name.
func isOUHandleGenerationEnabled(ctx *providers.NodeContext) bool {
	enabled, _ := ctx.NodeProperties[propertyKeyGenerateOUHandle].(bool)
	return enabled
}

// getOrganizationUnitRequest constructs an OrganizationUnitRequest from the NodeContext and the handle.
func (o *ouExecutor) getOrganizationUnitRequest(
	ctx *providers.NodeContext, handle string,
) (providers.OrganizationUnitRequestWithID, error) {
	ouRequest := providers.OrganizationUnitRequestWithID{
		Name:        ctx.UserInputs[userInputOuName],
		Handle:      handle,
		Description: ctx.UserInputs[userInputOuDesc],
	}

	// Check if parentOuId is explicitly set in node properties.
	if val, ok := ctx.NodeProperties["parentOuId"]; ok {
		strVal, isStr := val.(string)
		if !isStr {
			return ouRequest, fmt.Errorf("parentOuId must be a string, got %T", val)
		}
		if strVal != "" {
			ouRequest.Parent = &strVal
		}
	} else if val, ok := ctx.RuntimeData[defaultOUIDKey]; ok && val != "" {
		ouRequest.Parent = &val
	} else {
		defaultOUID, err := o.getDefaultOUID(ctx)
		if err != nil {
			return ouRequest, err
		}
		if defaultOUID != "" {
			ouRequest.Parent = &defaultOUID
		}
	}

	return ouRequest, nil
}

// getDefaultOUID resolves the default OU ID from the application's allowed user types.
func (o *ouExecutor) getDefaultOUID(ctx *providers.NodeContext) (string, error) {
	logger := o.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	if len(ctx.Application.AllowedUserTypes) == 0 {
		logger.Debug(ctx.Context, "No allowed user types configured for the application")
		return "", nil
	}

	selfRegEnabledSchemas := make([]entitytype.EntityType, 0)
	for _, userType := range ctx.Application.AllowedUserTypes {
		et, svcErr := o.entityTypeService.GetEntityTypeByHandle(ctx.Context,
			entitytype.TypeCategoryUser, userType)
		if svcErr != nil {
			return "", fmt.Errorf("failed to retrieve entity type for user type %q: %s",
				userType, svcErr.ErrorDescription.DefaultValue)
		}
		if et.AllowSelfRegistration {
			selfRegEnabledSchemas = append(selfRegEnabledSchemas, *et)
		}
	}

	if len(selfRegEnabledSchemas) == 0 {
		logger.Debug(ctx.Context, "No user types with self-registration enabled, cannot resolve default OU")
		return "", nil
	}

	if len(selfRegEnabledSchemas) > 1 {
		logger.Debug(ctx.Context,
			"Multiple user types with self-registration enabled, cannot resolve default OU automatically")
		return "", nil
	}

	return selfRegEnabledSchemas[0].OUID, nil
}
