/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// attributeUniquenessValidator checks whether values supplied for unique schema attributes
// are already held by an existing user.  It is intended to be placed in a flow immediately
// after a prompt node so that conflicts can be reported with the specific attribute name
// before any creation executor runs.
type attributeUniquenessValidator struct {
	core.ExecutorInterface
	entityTypeService entitytype.EntityTypeServiceInterface
	entityProvider    entityprovider.EntityProviderInterface
	logger            *log.Logger
}

// newAttributeUniquenessValidator creates a new instance of attributeUniquenessValidator.
func newAttributeUniquenessValidator(
	flowFactory core.FlowFactoryInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	entityProvider entityprovider.EntityProviderInterface,
) *attributeUniquenessValidator {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, ExecutorNameAttributeUniquenessValidator))
	prerequisites := []common.Input{
		{
			Identifier: userTypeKey,
			Required:   true,
		},
	}
	base := flowFactory.CreateExecutor(ExecutorNameAttributeUniquenessValidator, common.ExecutorTypeUtility,
		[]common.Input{}, prerequisites)
	return &attributeUniquenessValidator{
		ExecutorInterface: base,
		entityTypeService: entityTypeService,
		entityProvider:    entityProvider,
		logger:            logger,
	}
}

// Execute iterates over the unique attributes defined in the user type and checks whether
// any value already present in UserInputs belongs to an existing user.
// Returns ExecUserInputRequired (triggering onIncomplete routing) with the specific attribute
// named in FailureReason when a conflict is detected, or ExecComplete when all values are free.
func (e *attributeUniquenessValidator) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Executing uniqueness checker executor")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	if !e.ValidatePrerequisites(ctx, execResp) {
		return execResp, nil
	}

	userType := ctx.RuntimeData[userTypeKey]

	svcCtx := security.WithRuntimeContext(context.Background())
	uniqueAttrs, svcErr := e.entityTypeService.GetUniqueAttributes(svcCtx, entitytype.TypeCategoryUser, userType)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to retrieve unique attributes from schema for user type %s: %s",
			userType, svcErr.Error.DefaultValue)
	}

	for _, attr := range uniqueAttrs {
		value, exists := ctx.UserInputs[attr]
		if !exists || value == "" {
			continue
		}

		userID, svcErr := e.entityProvider.IdentifyEntity(map[string]interface{}{attr: value})
		if svcErr != nil {
			if svcErr.Code == entityprovider.ErrorCodeEntityNotFound {
				continue
			}
			return nil, fmt.Errorf("failed to check uniqueness for attribute %s: %s", attr, svcErr.Message)
		}

		if userID != nil {
			logger.Debug("Unique attribute conflict detected", log.String("attribute", attr))
			execResp.Status = common.ExecUserInputRequired
			execResp.FailureReason = fmt.Sprintf(
				"A user with this %s already exists. Please use a different value.", attr)
			return execResp, nil
		}
	}

	execResp.Status = common.ExecComplete
	return execResp, nil
}
