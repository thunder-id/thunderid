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
	"encoding/json"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// credentialSetter allows users to set their credentials for an existing user account.
type credentialSetter struct {
	core.ExecutorInterface
	entityProvider entityprovider.EntityProviderInterface
	logger         *log.Logger
}

// newCredentialSetter creates a new instance of the credential setter executor.
func newCredentialSetter(
	flowFactory core.FlowFactoryInterface,
	entityProvider entityprovider.EntityProviderInterface,
) *credentialSetter {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CredentialSetter"))
	base := flowFactory.CreateExecutor(
		ExecutorNameCredentialSetter,
		common.ExecutorTypeRegistration,
		[]common.Input{
			{
				Identifier: userAttributePassword,
				Type:       common.InputTypePassword,
				Required:   true,
			},
		},
		[]common.Input{
			{
				Identifier: userAttributeUserID,
				Type:       common.InputTypeText,
				Required:   true,
			},
		},
	)
	return &credentialSetter{
		ExecutorInterface: base,
		entityProvider:    entityProvider,
		logger:            logger,
	}
}

// Execute sets the password for the user identified by userID in RuntimeData.
func (e *credentialSetter) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Executing credential set")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	// Check if password is provided
	if !e.HasRequiredInputs(ctx, execResp) {
		logger.Debug("Requested credentials not provided, requesting input")
		execResp.Status = common.ExecUserInputRequired
		return execResp, nil
	}

	// Validate prerequisites
	if !e.ValidatePrerequisites(ctx, execResp) {
		logger.Debug("Prerequisites not met for credential setter")
		return execResp, nil
	}

	// Get userID from context
	userID := e.GetUserIDFromContext(ctx)
	if userID == "" {
		logger.Debug("User ID not found in flow context")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "User ID not found in flow context"
		return execResp, nil
	}

	var credentialKey, credentialValue string
	requiredInputs := e.GetRequiredInputs(ctx)
	if len(requiredInputs) == 0 {
		logger.Debug("No required inputs configured for credential setter")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "No credential input configured for credential setter"
		return execResp, nil
	}

	input := requiredInputs[0]
	credentialKey = input.Identifier
	if credentialKey == "" {
		logger.Debug("Required input has empty identifier in credential setter")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Invalid credential input configuration"
		return execResp, nil
	}
	credentialValue = ctx.UserInputs[credentialKey]

	if credentialValue == "" {
		logger.Debug("Credential value is empty", log.String("credentialKey", credentialKey))
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Credential value cannot be empty"
		return execResp, nil
	}

	// Build credentials
	credentials, err := json.Marshal(map[string]string{
		credentialKey: credentialValue,
	})
	if err != nil {
		logger.Debug("Failed to marshal credentials", log.Error(err))
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Failed to process credentials"
		return execResp, nil
	}

	// Update user credentials
	svcErr := e.entityProvider.UpdateCredentials(userID, credentials)
	if svcErr != nil {
		logger.Debug("Failed to update user credentials", log.MaskedString(log.LoggerKeyUserID, userID))
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Failed to set credentials"
		return execResp, nil
	}

	logger.Debug("Successfully set credentials for user", log.MaskedString(log.LoggerKeyUserID, userID))
	execResp.Status = common.ExecComplete
	return execResp, nil
}
