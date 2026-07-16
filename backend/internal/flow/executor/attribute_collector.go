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
	"encoding/json"
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	attrCollectLoggerComponentName = "AttributeCollector"
)

// TODO: Need to handle complex attributes and nested structures in the user profile.
//  Currently executor only takes string inputs.

// attributeCollector is an executor that collects user attributes and updates the user profile.
type attributeCollector struct {
	providers.Executor
	entityProvider entityprovider.EntityProviderInterface
	authnProvider  providers.AuthnProviderManager
	logger         *log.Logger
}

var _ providers.Executor = (*attributeCollector)(nil)

// newAttributeCollector creates a new instance of AttributeCollector.
func newAttributeCollector(
	flowFactory core.FlowFactoryInterface,
	entityProvider entityprovider.EntityProviderInterface,
	authnProvider providers.AuthnProviderManager,
) *attributeCollector {
	prerequisites := []providers.Input{
		{
			Identifier: "userID",
			Type:       "string",
			Required:   true,
		},
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, attrCollectLoggerComponentName),
		log.String(log.LoggerKeyExecutorName, ExecutorNameAttributeCollect))

	base := flowFactory.CreateExecutor(ExecutorNameAttributeCollect, providers.ExecutorTypeUtility,
		[]providers.Input{}, prerequisites, &providers.ExecutorMeta{})

	return &attributeCollector{
		Executor:       base,
		entityProvider: entityProvider,
		authnProvider:  authnProvider,
		logger:         logger,
	}
}

// Execute executes the attribute collection logic.
func (a *attributeCollector) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := a.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing attribute collect executor")

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		AuthUser:       ctx.AuthUser,
	}

	if ctx.FlowType == providers.FlowTypeRegistration {
		logger.Debug(ctx.Context, "Flow type is registration, skipping attribute collection")
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}

	if !execResp.AuthUser.IsAuthenticated() {
		logger.Debug(ctx.Context, "User is not authenticated, cannot collect attributes")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrUserNotAuthenticated
		return execResp, nil
	}

	if !a.ValidatePrerequisites(ctx, execResp, a.authnProvider) {
		logger.Debug(ctx.Context, "Prerequisites validation failed for attribute collector")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrPrerequisitesFailed
		return execResp, nil
	}

	if !a.HasRequiredInputs(ctx, execResp) {
		logger.Debug(ctx.Context, "Required inputs for attribute collector is not provided")
		execResp.Status = providers.ExecUserInputRequired
		return execResp, nil
	}

	if err := a.updateUserInStore(ctx, execResp); err != nil {
		logger.Error(ctx.Context, "Failed to update user attributes", log.Error(err))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrAttributeCollectFailed
		return execResp, nil
	}

	logger.Debug(ctx.Context, "User attributes updated successfully")
	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// HasRequiredInputs checks if the required inputs are provided in the context and appends any
// missing inputs to the executor response. Returns true if required inputs are found, otherwise false.
func (a *attributeCollector) HasRequiredInputs(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) bool {
	logger := a.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Checking inputs for the attribute collector")

	if a.Executor.HasRequiredInputs(ctx, execResp) {
		return true
	}
	if len(execResp.Inputs) == 0 {
		return true
	}

	// Update the executor response with the required inputs retrieved from authenticated user attributes.
	authUser, authnUserAttrs, svcErr := a.authnProvider.GetUserAttributes(ctx.Context, nil, nil, execResp.AuthUser)
	if svcErr != nil {
		logger.Warn(ctx.Context, "Failed to retrieve authenticated user attributes")
	}
	execResp.AuthUser = authUser

	if authnUserAttrs != nil && len(authnUserAttrs.Attributes) > 0 {
		logger.Debug(ctx.Context,
			"Authenticated user attributes found, updating executor response required inputs")

		// Clear the required data in the executor response to avoid duplicates.
		missingAttributes := execResp.Inputs
		execResp.Inputs = make([]providers.Input, 0)
		if execResp.RuntimeData == nil {
			execResp.RuntimeData = make(map[string]string)
		}

		for _, input := range missingAttributes {
			attribute, exists := authnUserAttrs.Attributes[input.Identifier]
			if exists {
				// If the attribute is a password, do not retrieve it from the profile.
				if input.Identifier == userAttributePassword {
					continue
				}

				attributeStr, ok := attribute.Value.(string)
				if ok {
					logger.Debug(ctx.Context,
						"Input exists in authenticated user attributes, adding to runtime data",
						log.String("attributeName", input.Identifier))
					execResp.RuntimeData[input.Identifier] = attributeStr
				}
			} else {
				logger.Debug(ctx.Context,
					"Input does not exist in authenticated user attributes, adding to required inputs",
					log.String("attributeName", input.Identifier))
				execResp.Inputs = append(execResp.Inputs, input)
			}
		}

		if len(execResp.Inputs) == 0 {
			logger.Debug(ctx.Context, "All required inputs are available in authenticated user attributes, "+
				"no further action needed")
			return true
		}
	}

	// Update the executor response with the required inputs by checking the user profile.
	userAttributes, err := a.getUserAttributes(ctx, execResp)
	if err != nil {
		// Silently log the error and proceed with prompting for required inputs.
		logger.Error(ctx.Context, "Failed to retrieve user attributes", log.Error(err))
		return false
	}
	if userAttributes == nil {
		logger.Debug(ctx.Context,
			"No user attributes found in the user profile, proceeding with required inputs")
		return false
	}

	// Clear the required inputs in the executor response to avoid duplicates.
	missingInputs := execResp.Inputs
	execResp.Inputs = make([]providers.Input, 0)
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
			logger.Debug(ctx.Context, "Input exists in user profile, adding to runtime data",
				log.String("attributeName", input.Identifier))

			// TODO: This conversion should be modified according to the storage mechanism of the
			//  user store implementation.
			if strVal, ok := attribute.(string); ok {
				execResp.RuntimeData[input.Identifier] = strVal
			} else {
				execResp.RuntimeData[input.Identifier] = fmt.Sprintf("%v", attribute)
			}
		} else {
			logger.Debug(ctx.Context, "Input does not exist in user profile, adding to required inputs",
				log.String("attributeName", input.Identifier))
			execResp.Inputs = append(execResp.Inputs, input)
		}
	}

	if len(execResp.Inputs) == 0 {
		logger.Debug(ctx.Context,
			"All required inputs are available in the user profile, no further action needed")
		return true
	}

	return false
}

// getUserAttributes retrieves the user attributes from the user profile.
func (a *attributeCollector) getUserAttributes(
	ctx *providers.NodeContext, execResp *providers.ExecutorResponse,
) (map[string]interface{}, error) {
	logger := a.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Retrieving user attributes from the user profile")

	user, err := a.getUserFromStore(ctx, execResp)
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
	logger.Debug(ctx.Context, "User attributes retrieved successfully")

	return userAttributes, nil
}

// updateUserInStore updates the user profile with the collected attributes.
func (a *attributeCollector) updateUserInStore(ctx *providers.NodeContext, execResp *providers.ExecutorResponse) error {
	logger := a.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Updating user attributes")

	user, err := a.getUserFromStore(ctx, execResp)
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
		logger.Debug(ctx.Context, "No updates required for user attributes, skipping update")
		return nil
	}
	if updatedUser == nil {
		return errors.New("failed to create updated user object")
	}

	if err := a.entityProvider.UpdateAttributes(userID, updatedUser.Attributes); err != nil {
		return fmt.Errorf("failed to update user attributes: %s", err.Message)
	}
	logger.Debug(ctx.Context, "User attributes updated successfully",
		log.MaskedString(log.LoggerKeyUserID, userID))

	return nil
}

// getUserFromStore retrieves the user profile from the user store.
func (a *attributeCollector) getUserFromStore(
	ctx *providers.NodeContext, execResp *providers.ExecutorResponse,
) (*providers.Entity, error) {
	userID := a.GetUserIDFromContext(ctx, execResp, a.authnProvider)
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
func (a *attributeCollector) getUpdatedUserObject(ctx *providers.NodeContext,
	userData *providers.Entity) (bool, *providers.Entity, error) {
	logger := a.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	updatedUser := &providers.Entity{
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
		logger.Debug(ctx.Context, "No new attributes provided, returning existing user")
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
func (a *attributeCollector) getInputAttributes(ctx *providers.NodeContext) map[string]interface{} {
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
func (a *attributeCollector) getInputs(ctx *providers.NodeContext) []providers.Input {
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
