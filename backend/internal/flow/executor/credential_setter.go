// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"encoding/json"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// credentialSetter allows users to set their credentials for an existing user account.
type credentialSetter struct {
	providers.Executor
	entityProvider entityprovider.EntityProviderInterface
	authnProvider  providers.AuthnProviderManager
	logger         *log.Logger
}

// newCredentialSetter creates a new instance of the credential setter executor.
func newCredentialSetter(
	flowFactory core.FlowFactoryInterface,
	entityProvider entityprovider.EntityProviderInterface,
	authnProvider providers.AuthnProviderManager,
) *credentialSetter {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CredentialSetter"))
	base := flowFactory.CreateExecutor(
		ExecutorNameCredentialSetter,
		providers.ExecutorTypeRegistration,
		[]providers.Input{
			{
				Identifier: userAttributePassword,
				Type:       providers.InputTypePassword,
				Required:   true,
			},
		},
		[]providers.Input{
			{
				Identifier: userAttributeUserID,
				Type:       providers.InputTypeText,
				Required:   true,
			},
		},
		&providers.ExecutorMeta{},
	)
	return &credentialSetter{
		Executor:       base,
		entityProvider: entityProvider,
		authnProvider:  authnProvider,
		logger:         logger,
	}
}

// Execute sets the password for the user identified by userID in RuntimeData.
func (e *credentialSetter) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing credential set")

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	// Check if password is provided
	if !e.HasRequiredInputs(ctx, execResp) {
		logger.Debug(ctx.Context, "Requested credentials not provided, requesting input")
		execResp.Status = providers.ExecUserInputRequired
		return execResp, nil
	}

	// Validate prerequisites
	if !e.ValidatePrerequisites(ctx, execResp, e.authnProvider) {
		logger.Debug(ctx.Context, "Prerequisites not met for credential setter")
		return execResp, nil
	}

	// Get userID from context
	userID := e.GetUserIDFromContext(ctx, execResp, e.authnProvider)
	if userID == "" {
		logger.Debug(ctx.Context, "User ID not found in flow context")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrUserIDMissingInContext
		return execResp, nil
	}

	var credentialKey, credentialValue string
	requiredInputs := e.GetRequiredInputs(ctx)
	if len(requiredInputs) == 0 {
		logger.Debug(ctx.Context, "No required inputs configured for credential setter")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrCredentialInputMissing
		return execResp, nil
	}

	input := requiredInputs[0]
	credentialKey = input.Identifier
	if credentialKey == "" {
		logger.Debug(ctx.Context, "Required input has empty identifier in credential setter")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrCredentialInputInvalid
		return execResp, nil
	}
	credentialValue = ctx.UserInputs[credentialKey]

	if credentialValue == "" {
		logger.Debug(ctx.Context, "Credential value is empty", log.String("credentialKey", credentialKey))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrCredentialValueEmpty
		return execResp, nil
	}

	// Build credentials
	credentials, err := json.Marshal(map[string]string{
		credentialKey: credentialValue,
	})
	if err != nil {
		logger.Debug(ctx.Context, "Failed to marshal credentials", log.Error(err))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrCredentialProcessingFailed
		return execResp, nil
	}

	// Update user credentials
	svcErr := e.entityProvider.UpdateCredentials(ctx.Context, userID, credentials)
	if svcErr != nil {
		logger.Debug(ctx.Context, "Failed to update user credentials",
			log.MaskedString(log.LoggerKeyUserID, userID))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrCredentialSetFailed
		return execResp, nil
	}

	logger.Debug(ctx.Context, "Successfully set credentials for user",
		log.MaskedString(log.LoggerKeyUserID, userID))
	execResp.Status = providers.ExecComplete
	return execResp, nil
}
