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

// Package executor defines executors that can be used during flow executions for authentication, registration
// and other purposes.
package executor

import (
	"errors"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// basicAuthExecutor implements the ExecutorInterface for basic authentication.
type basicAuthExecutor struct {
	core.ExecutorInterface
	identifyingExecutorInterface
	entityProvider entityprovider.EntityProviderInterface
	authnProvider  authnprovidermgr.AuthnProviderManagerInterface
	logger         *log.Logger
}

var _ core.ExecutorInterface = (*basicAuthExecutor)(nil)
var _ identifyingExecutorInterface = (*basicAuthExecutor)(nil)

// newBasicAuthExecutor creates a new instance of BasicAuthExecutor.
func newBasicAuthExecutor(
	flowFactory core.FlowFactoryInterface,
	entityProvider entityprovider.EntityProviderInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
) *basicAuthExecutor {
	defaultInputs := []common.Input{
		{
			Identifier: userAttributeUsername,
			Type:       common.InputTypeText,
			Required:   true,
		},
		{
			Identifier: userAttributePassword,
			Type:       common.InputTypePassword,
			Required:   true,
		},
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "BasicAuthExecutor"),
		log.String(log.LoggerKeyExecutorName, ExecutorNameBasicAuth))

	identifyExec := newIdentifyingExecutor(ExecutorNameBasicAuth, defaultInputs, []common.Input{},
		flowFactory, entityProvider)
	base := flowFactory.CreateExecutor(ExecutorNameBasicAuth, common.ExecutorTypeAuthentication,
		defaultInputs, []common.Input{})

	return &basicAuthExecutor{
		ExecutorInterface:            base,
		identifyingExecutorInterface: identifyExec,
		entityProvider:               entityProvider,
		authnProvider:                authnProvider,
		logger:                       logger,
	}
}

// Execute executes the basic authentication logic.
func (b *basicAuthExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := b.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing basic authentication executor")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		AuthUser:       ctx.AuthUser,
	}

	// When a userID is pre-resolved (e.g., by an IdentifyingExecutor in resolve mode),
	// only credential inputs are required — skip the identifier input check.
	hasPreResolvedUser := ctx.RuntimeData[userAttributeUserID] != ""
	if hasPreResolvedUser {
		credentialInputs := b.getCredentialInputs(ctx)
		hasMissingCredentials := false
		for _, input := range credentialInputs {
			if ctx.UserInputs[input.Identifier] == "" {
				hasMissingCredentials = true
				break
			}
		}
		if hasMissingCredentials {
			execResp.Status = common.ExecUserInputRequired
			execResp.Inputs = credentialInputs
			return execResp, nil
		}
	} else if !b.HasRequiredInputs(ctx, execResp) {
		logger.Debug(ctx.Context, "Required inputs for basic authentication executor is not provided")
		execResp.Status = common.ExecUserInputRequired
		return execResp, nil
	}

	// TODO: Should handle client errors here. Service should return a ServiceError and
	//  client errors should be appended as a failure.
	//  For the moment handling returned error as a authentication failure.
	err := b.authenticateUser(ctx, execResp)
	if err != nil {
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrUserAuthFailed
		return execResp, nil
	}
	if execResp.Status == common.ExecFailure || execResp.Status == common.ExecUserInputRequired {
		return execResp, nil
	}
	if !execResp.AuthUser.IsAuthenticated() && ctx.FlowType != common.FlowTypeRegistration {
		execResp.Status = common.ExecUserInputRequired
		if hasPreResolvedUser {
			execResp.Inputs = b.getCredentialInputs(ctx)
		} else {
			execResp.Inputs = b.GetRequiredInputs(ctx)
		}
		execResp.Error = &ErrInvalidCredentials
		return execResp, nil
	}

	execResp.Status = common.ExecComplete

	logger.Debug(ctx.Context, "Basic authentication executor execution completed",
		log.String("status", string(execResp.Status)),
		log.Bool("isAuthenticated", execResp.AuthUser.IsAuthenticated()))

	return execResp, nil
}

// getCredentialInputs returns the sensitive (credential) inputs from the node's required inputs.
func (b *basicAuthExecutor) getCredentialInputs(ctx *core.NodeContext) []common.Input {
	var credentials []common.Input
	for _, input := range b.GetRequiredInputs(ctx) {
		if input.IsSensitive() {
			credentials = append(credentials, input)
		}
	}
	return credentials
}

// authenticateUser perform authentication based on the provided identifying and
// credential attributes and returns the authenticated user details.
func (b *basicAuthExecutor) authenticateUser(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) error {
	logger := b.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	userIdentifiers := map[string]interface{}{}
	userCredentials := map[string]interface{}{}

	// When a userID is pre-resolved, use it as the identifier for authentication.
	if preResolvedUserID, ok := ctx.RuntimeData[userAttributeUserID]; ok {
		userIdentifiers[userAttributeUserID] = preResolvedUserID
	}

	for _, inputData := range b.GetRequiredInputs(ctx) {
		if value, ok := ctx.UserInputs[inputData.Identifier]; ok {
			if inputData.IsSensitive() {
				userCredentials[inputData.Identifier] = value
			} else {
				userIdentifiers[inputData.Identifier] = value
			}
		}
	}

	// For registration flows, only check if user exists.
	if ctx.FlowType == common.FlowTypeRegistration {
		_, err := b.IdentifyUser(ctx.Context, userIdentifiers, execResp)
		if err != nil {
			return err
		}
		if execResp.Status == common.ExecFailure {
			if execResp.Error != nil && execResp.Error.Code == ErrUserNotFound.Code {
				logger.Debug(ctx.Context,
					"User not found for the provided attributes. Proceeding with registration flow.")
				execResp.Status = common.ExecComplete
				return nil
			}
			return nil
		}
		// User found - fail registration.
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrUserAlreadyExists
		return nil
	}

	// For authentication flows, call Authenticate directly.
	metadata := b.buildAuthnMetadata(ctx)
	authUser, authenticatedClaims, svcErr := b.authnProvider.AuthenticateUser(ctx.Context, userIdentifiers,
		userCredentials, nil, metadata, execResp.AuthUser)
	execResp.AuthUser = authUser
	if svcErr != nil {
		if svcErr.Type == serviceerror.ClientErrorType {
			execResp.Status = common.ExecUserInputRequired
			execResp.Inputs = b.GetRequiredInputs(ctx)

			switch svcErr.Code {
			case authnprovidermgr.ErrorUserNotFound.Code:
				execResp.Error = &ErrUserNotFound
			case authnprovidermgr.ErrorAuthenticationFailed.Code:
				execResp.Error = &ErrInvalidCredentials
			default:
				execResp.Error = &ErrUserAuthFailed
			}

			return nil
		}

		logger.Error(ctx.Context, "Failed to authenticate user",
			log.String("errorCode", svcErr.Code), log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
		return errors.New("failed to authenticate user")
	}
	for key, value := range authenticatedClaims {
		if strVal, ok := value.(string); ok {
			execResp.RuntimeData[key] = strVal
		}
	}

	return nil
}

// buildAuthnMetadata constructs the metadata for authentication.
func (b *basicAuthExecutor) buildAuthnMetadata(ctx *core.NodeContext) *authnprovidercm.AuthnMetadata {
	metadata := &authnprovidercm.AuthnMetadata{
		AppMetadata: make(map[string]interface{}),
	}

	// Copy application metadata if present
	if ctx.Application.Metadata != nil {
		for key, value := range ctx.Application.Metadata {
			metadata.AppMetadata[key] = value
		}
	}

	// Extract client IDs from InboundAuthConfig
	var clientIDs []string
	for _, inboundConfig := range ctx.Application.InboundAuthConfig {
		if inboundConfig.OAuthConfig != nil && inboundConfig.OAuthConfig.ClientID != "" {
			clientIDs = append(clientIDs, inboundConfig.OAuthConfig.ClientID)
		}
	}

	// Add client IDs to metadata if present
	if len(clientIDs) > 0 {
		metadata.AppMetadata["client_ids"] = clientIDs
	}

	return metadata
}
