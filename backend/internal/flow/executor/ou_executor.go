/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	ouExecLoggerComponentName = "OUExecutor"
)

// ouExecutor is responsible for creating organizational units (OUs) within the system.
type ouExecutor struct {
	core.ExecutorInterface
	ouService ou.OrganizationUnitServiceInterface
	logger    *log.Logger
}

var _ core.ExecutorInterface = (*ouExecutor)(nil)

// newOUExecutor creates a new instance of OUExecutor with the given parameters.
func newOUExecutor(
	flowFactory core.FlowFactoryInterface,
	ouService ou.OrganizationUnitServiceInterface,
) *ouExecutor {
	defaultInputs := []common.Input{
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

	base := flowFactory.CreateExecutor(ExecutorNameOUCreation, common.ExecutorTypeRegistration,
		defaultInputs, []common.Input{})

	return &ouExecutor{
		ExecutorInterface: base,
		ouService:         ouService,
		logger:            logger,
	}
}

// Execute executes the ou creation logic.
func (o *ouExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := o.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing OU creation executor")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	if !o.ValidatePrerequisites(ctx, execResp) {
		logger.Debug(ctx.Context, "Prerequisites validation failed for OU creation")
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrOUCreationPrereqFailed
		return execResp, nil
	}

	if !o.HasRequiredInputs(ctx, execResp) {
		logger.Debug(ctx.Context, "Required inputs for OU creation is not provided")
		execResp.Status = common.ExecUserInputRequired
		return execResp, nil
	}

	// Create the OU using the OU service.
	ouRequest, err := o.getOrganizationUnitRequest(ctx)
	if err != nil {
		logger.Error(ctx.Context, "Failed to build organization unit request",
			log.String("error", err.Error()))
		return nil, err
	}
	createdOU, svcErr := o.ouService.CreateOrganizationUnit(ctx.Context, ouRequest)
	if svcErr != nil {
		if svcErr.Type == serviceerror.ClientErrorType {
			execResp.Status = common.ExecUserInputRequired
			execResp.Inputs = o.GetRequiredInputs(ctx)

			switch svcErr.Code {
			case ou.ErrorOrganizationUnitNameConflict.Code:
				execResp.Error = &ErrOUNameConflict
			case ou.ErrorOrganizationUnitHandleConflict.Code:
				execResp.Error = &ErrOUHandleConflict
			default:
				execResp.Error = serviceerror.CustomServiceError(ErrOUCreationFailed, i18ncore.I18nMessage{
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

	logger.Debug(ctx.Context, "Organization unit created successfully", log.String(ouIDKey, createdOU.ID))
	execResp.Status = common.ExecComplete
	return execResp, nil
}

// getOrganizationUnitRequest constructs an OrganizationUnitRequest from the NodeContext.
func (o *ouExecutor) getOrganizationUnitRequest(ctx *core.NodeContext) (ou.OrganizationUnitRequestWithID, error) {
	ouRequest := ou.OrganizationUnitRequestWithID{
		Name:        ctx.UserInputs[userInputOuName],
		Handle:      ctx.UserInputs[userInputOuHandle],
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
	}

	return ouRequest, nil
}
