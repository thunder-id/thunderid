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
	"encoding/json"
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	attrCollectLoggerComponentName = "AttributeCollector"
)

// TODO: Need to handle complex attributes and nested structures in the user profile.
//  Currently executor only takes string inputs.

// attributeCollector is an executor that collects user attributes and updates the user profile.
type attributeCollector struct {
	core.ExecutorInterface
	entityProvider entityprovider.EntityProviderInterface
	logger         *log.Logger
}

var _ core.ExecutorInterface = (*attributeCollector)(nil)

// newAttributeCollector creates a new instance of AttributeCollector.
func newAttributeCollector(
	flowFactory core.FlowFactoryInterface,
	entityProvider entityprovider.EntityProviderInterface,
) *attributeCollector {
	prerequisites := []common.Input{
		{
			Identifier: "userID",
			Type:       "string",
			Required:   true,
		},
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, attrCollectLoggerComponentName),
		log.String(log.LoggerKeyExecutorName, ExecutorNameAttributeCollect))

	base := flowFactory.CreateExecutor(ExecutorNameAttributeCollect, common.ExecutorTypeUtility,
		[]common.Input{}, prerequisites)

	return &attributeCollector{
		ExecutorInterface: base,
		entityProvider:    entityProvider,
		logger:            logger,
	}
}

// Execute executes the attribute collection logic.
func (a *attributeCollector) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := a.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Executing attribute collect executor")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	if ctx.FlowType == common.FlowTypeRegistration {
		logger.Debug("Flow type is registration, skipping attribute collection")
		execResp.Status = common.ExecComplete
		return execResp, nil
	}

	if !ctx.AuthenticatedUser.IsAuthenticated {
		logger.Debug("User is not authenticated, cannot collect attributes")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = failureReasonUserNotAuthenticated
		return execResp, nil
	}

	if !a.ValidatePrerequisites(ctx, execResp) {
		logger.Debug("Prerequisites validation failed for attribute collector")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Prerequisites validation failed for attribute collector"
		return execResp, nil
	}

	if !a.HasRequiredInputs(ctx, execResp) {
		logger.Debug("Required inputs for attribute collector is not provided")
		execResp.Status = common.ExecUserInputRequired
		return execResp, nil
	}

	if err := a.updateUserInStore(ctx); err != nil {
		logger.Error("Failed to update user attributes", log.Error(err))
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Failed to update user attributes"
		return execResp, nil
	}

	logger.Debug("User attributes updated successfully")
	execResp.Status = common.ExecComplete
	return execResp, nil
}

// HasRequiredInputs checks if the required inputs are provided in the context and appends any
// missing inputs to the executor response. Returns true if required inputs are found, otherwise false.
func (a *attributeCollector) HasRequiredInputs(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) bool {
	logger := a.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Checking inputs for the attribute collector")

	if a.ExecutorInterface.HasRequiredInputs(ctx, execResp) {
		return true
	}
	if len(execResp.Inputs) == 0 {
		return true
	}

	// Update the executor response with the required inputs retrieved from authenticated user attributes.
	authnUserAttrs := ctx.AuthenticatedUser.Attributes
	if len(authnUserAttrs) > 0 {
		logger.Debug("Authenticated user attributes found, updating executor response required inputs")

		// Clear the required data in the executor response to avoid duplicates.
		missingAttributes := execResp.Inputs
		execResp.Inputs = make([]common.Input, 0)
		if execResp.RuntimeData == nil {
			execResp.RuntimeData = make(map[string]string)
		}

		for _, input := range missingAttributes {
			attribute, exists := authnUserAttrs[input.Identifier]
			if exists {
				// If the attribute is a password, do not retrieve it from the profile.
				if input.Identifier == userAttributePassword {
					continue
				}

				attributeStr, ok := attribute.(string)
				if ok {
					logger.Debug("Input exists in authenticated user attributes, adding to runtime data",
						log.String("attributeName", input.Identifier))
					execResp.RuntimeData[input.Identifier] = attributeStr
				}
			} else {
				logger.Debug("Input does not exist in authenticated user attributes, adding to required inputs",
					log.String("attributeName", input.Identifier))
				execResp.Inputs = append(execResp.Inputs, input)
			}
		}

		if len(execResp.Inputs) == 0 {
			logger.Debug("All required inputs are available in authenticated user attributes, " +
				"no further action needed")
			return true
		}
	}

	// Update the executor response with the required inputs by checking the user profile.
	userAttributes, err := a.getUserAttributes(ctx)
	if err != nil {
		// Silently log the error and proceed with prompting for required inputs.
		logger.Error("Failed to retrieve user attributes", log.Error(err))
		return false
	}
	if userAttributes == nil {
		logger.Debug("No user attributes found in the user profile, proceeding with required inputs")
		return false
	}

	// Clear the required inputs in the executor response to avoid duplicates.
	missingInputs := execResp.Inputs
	execResp.Inputs = make([]common.Input, 0)
	if execResp.RuntimeData == nil {
		execResp.RuntimeData = make(map[string]string)
	}

	for _, input := range missingInputs {
		attribute, exists := userAttributes[input.Identifier]
		if exists {
			// If the attribute is a password, do not retrieve it from the profile.
			if input.Identifier == userAttributePassword {
				continue
			}
			logger.Debug("Input exists in user profile, adding to runtime data",
				log.String("attributeName", input.Identifier))

			// TODO: This conversion should be modified according to the storage mechanism of the
			//  user store implementation.
			if strVal, ok := attribute.(string); ok {
				execResp.RuntimeData[input.Identifier] = strVal
			} else {
				execResp.RuntimeData[input.Identifier] = fmt.Sprintf("%v", attribute)
			}
		} else {
			logger.Debug("Input does not exist in user profile, adding to required inputs",
				log.String("attributeName", input.Identifier))
			execResp.Inputs = append(execResp.Inputs, input)
		}
	}

	if len(execResp.Inputs) == 0 {
		logger.Debug("All required inputs are available in the user profile, no further action needed")
		return true
	}

	return false
}

// getUserAttributes retrieves the user attributes from the user profile.
func (a *attributeCollector) getUserAttributes(ctx *core.NodeContext) (map[string]interface{}, error) {
	logger := a.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Retrieving user attributes from the user profile")

	user, err := a.getUserFromStore(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve user from store: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Unmarshal the user attributes if they exist
	var userAttributes map[string]interface{}
	if user.Attributes != nil {
		if err := json.Unmarshal(user.Attributes, &userAttributes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal user attributes: %w", err)
		}
	} else {
		userAttributes = make(map[string]interface{})
	}
	logger.Debug("User attributes retrieved successfully")

	return userAttributes, nil
}

// updateUserInStore updates the user profile with the collected attributes.
func (a *attributeCollector) updateUserInStore(ctx *core.NodeContext) error {
	logger := a.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Updating user attributes")

	user, err := a.getUserFromStore(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve user from store: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user not found")
	}
	userID := user.ID

	updateRequired, updatedUser, err := a.getUpdatedUserObject(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to get updated user object: %w", err)
	}
	if !updateRequired {
		logger.Debug("No updates required for user attributes, skipping update")
		return nil
	}
	if updatedUser == nil {
		return errors.New("failed to create updated user object")
	}

	if err := a.entityProvider.UpdateAttributes(userID, updatedUser.Attributes); err != nil {
		return fmt.Errorf("failed to update user attributes: %s", err.Message)
	}
	logger.Debug("User attributes updated successfully", log.MaskedString(log.LoggerKeyUserID, userID))

	return nil
}

// getUserFromStore retrieves the user profile from the user store.
func (a *attributeCollector) getUserFromStore(ctx *core.NodeContext) (*entityprovider.Entity, error) {
	userID := a.GetUserIDFromContext(ctx)
	if userID == "" {
		return nil, errors.New("user ID is not available in the context")
	}

	user, err := a.entityProvider.GetEntity(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by ID: %s", err.Message)
	}

	return user, nil
}

// getUpdatedUserObject creates a new user object with the updated attributes.
func (a *attributeCollector) getUpdatedUserObject(ctx *core.NodeContext,
	userData *entityprovider.Entity) (bool, *entityprovider.Entity, error) {
	logger := a.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	updatedUser := &entityprovider.Entity{
		ID:       userData.ID,
		Category: userData.Category,
		OUID:     userData.OUID,
		Type:     userData.Type,
		State:    userData.State,
	}

	// Get the existing attributes
	var existingAttrs map[string]interface{}
	if userData.Attributes != nil {
		if err := json.Unmarshal(userData.Attributes, &existingAttrs); err != nil {
			return false, nil, fmt.Errorf("failed to unmarshal existing user attributes: %w", err)
		}
	} else {
		existingAttrs = make(map[string]interface{})
	}

	// Get new attributes from input
	newAttrs := a.getInputAttributes(ctx)
	if len(newAttrs) == 0 {
		logger.Debug("No new attributes provided, returning existing user")
		return false, userData, nil
	}

	// Merge attributes
	for k, v := range newAttrs {
		existingAttrs[k] = v
	}

	// Marshal the merged attributes back to JSON
	if len(existingAttrs) > 0 {
		mergedAttrs, err := json.Marshal(existingAttrs)
		if err != nil {
			return false, nil, fmt.Errorf("failed to marshal merged attributes: %w", err)
		} else {
			updatedUser.Attributes = mergedAttrs
		}
	}

	return true, updatedUser, nil
}

// getInputAttributes retrieves the input attributes from the context.
func (a *attributeCollector) getInputAttributes(ctx *core.NodeContext) map[string]interface{} {
	attributesMap := make(map[string]interface{})
	requiredInputAttrs := a.getInputs(ctx)

	for _, inputAttr := range requiredInputAttrs {
		// Skip special attributes that shouldn't be stored/ updated in the user profile
		if inputAttr.Identifier == userAttributeUserID {
			continue
		}

		value, exists := ctx.UserInputs[inputAttr.Identifier]
		if exists {
			attributesMap[inputAttr.Identifier] = value
		} else if runtimeValue, exists := ctx.RuntimeData[inputAttr.Identifier]; exists {
			attributesMap[inputAttr.Identifier] = runtimeValue
		}
	}

	return attributesMap
}

// getInputs returns the required inputs for the AttributeCollector.
func (a *attributeCollector) getInputs(ctx *core.NodeContext) []common.Input {
	executorReqData := a.GetDefaultInputs()
	requiredData := ctx.NodeInputs

	if len(requiredData) == 0 {
		requiredData = executorReqData
	} else {
		// Append the default required data if not already present.
		for _, input := range executorReqData {
			exists := false
			for _, existingInput := range requiredData {
				if existingInput.Identifier == input.Identifier {
					exists = true
					break
				}
			}
			// If the inputs already exists, skip adding it again.
			if !exists {
				requiredData = append(requiredData, input)
			}
		}
	}

	return requiredData
}
