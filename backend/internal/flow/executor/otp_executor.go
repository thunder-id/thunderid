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
	"fmt"
	"strconv"

	"github.com/thunder-id/thunderid/internal/authn/otp"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// otpExecutor implements a channel-agnostic OTP generate/verify executor.
// Generate mode: identifies the user from declared node inputs, generates an OTP session
// token keyed to the userID, and stores the plaintext OTP in RuntimeData for a downstream
// sender executor (SMSExecutor, EmailExecutor, etc.).
// Verify mode: validates the OTP code against the session token and authenticates the user.
type otpExecutor struct {
	core.ExecutorInterface
	entityProvider entityprovider.EntityProviderInterface
	otpService     otp.OTPAuthnServiceInterface
	authnProvider  authnprovidermgr.AuthnProviderManagerInterface
	logger         *log.Logger
}

var _ core.ExecutorInterface = (*otpExecutor)(nil)

// newOTPExecutor creates a new instance of otpExecutor.
func newOTPExecutor(
	flowFactory core.FlowFactoryInterface,
	otpService otp.OTPAuthnServiceInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
	entityProvider entityprovider.EntityProviderInterface,
) *otpExecutor {
	defaultInputs := []common.Input{
		{
			Ref:        "otp_input",
			Identifier: userInputOTP,
			Type:       common.InputTypeOTP,
			Required:   true,
		},
	}
	prerequisites := []common.Input{
		{
			Identifier: common.RuntimeKeyOTPSessionToken,
			Type:       common.InputTypeHidden,
			Required:   true,
		},
	}

	logger := log.GetLogger().With(
		log.String(log.LoggerKeyComponentName, "OTPExecutor"),
		log.String(log.LoggerKeyExecutorName, ExecutorNameOTPExecutor),
	)

	base := flowFactory.CreateExecutor(ExecutorNameOTPExecutor, common.ExecutorTypeAuthentication,
		defaultInputs, prerequisites)

	return &otpExecutor{
		ExecutorInterface: base,
		entityProvider:    entityProvider,
		otpService:        otpService,
		authnProvider:     authnProvider,
		logger:            logger,
	}
}

// Execute dispatches to generate or verify mode based on ctx.ExecutorMode.
func (e *otpExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing OTP executor")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		AuthUser:       ctx.AuthUser,
	}

	switch ctx.ExecutorMode {
	case ExecutorModeGenerate:
		return e.executeGenerate(ctx, execResp)
	case ExecutorModeVerify:
		return e.executeVerify(ctx, execResp)
	default:
		return execResp, fmt.Errorf("invalid executor mode: %s", ctx.ExecutorMode)
	}
}

// executeGenerate identifies the user from declared node inputs, generates an OTP, and
// stores the session token and plaintext OTP value in RuntimeData for downstream executors.
//
// For authentication flows the recipient is the resolved userID. For registration flows
// the user does not exist yet, so when identification fails the first PHONE_INPUT nodeInput
// attribute value is used directly as the recipient (matching the MagicLink pattern).
func (e *otpExecutor) executeGenerate(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) (*common.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	attemptCount, err := e.validateAttempts(ctx, execResp, logger)
	if err != nil {
		return execResp, err
	}
	if execResp.Status == common.ExecFailure {
		return execResp, nil
	}

	userID, err := e.resolveUserID(ctx, execResp)
	if err != nil {
		return execResp, err
	}

	recipient := userID
	recipientAttr := authnprovidercm.UserAttributeUserID

	if recipient == "" && execResp.Status == common.ExecFailure {
		if ctx.FlowType != common.FlowTypeRegistration {
			return execResp, nil
		}
		// Registration: user does not exist yet — use the declared nodeInput attribute value
		// directly as the OTP recipient so the session token is keyed to that identifier.
		destAttr, destValue := e.resolveOTPDestination(ctx)
		if destValue == "" {
			logger.Debug(ctx.Context, "No destination value found for OTP generation in registration flow")
			execResp.Status = common.ExecUserInputRequired
			execResp.Error = nil
			execResp.Inputs = e.GetRequiredInputs(ctx)
			return execResp, nil
		}
		execResp.Status = ""
		execResp.Error = nil
		recipient = destValue
		recipientAttr = destAttr
	}

	if execResp.Status == common.ExecUserInputRequired {
		return execResp, nil
	}

	sessionToken, otpValue, svcErr := e.otpService.GenerateOTP(ctx.Context, recipient, recipientAttr)
	if svcErr != nil {
		return execResp, fmt.Errorf("failed to generate OTP: %s", svcErr.ErrorDescription.DefaultValue)
	}

	execResp.RuntimeData[common.RuntimeKeyOTPSessionToken] = sessionToken
	execResp.RuntimeData[common.RuntimeKeyOTPValue] = otpValue
	execResp.RuntimeData[common.RuntimeKeyOTPAttemptCount] = strconv.Itoa(attemptCount + 1)
	execResp.Status = common.ExecComplete

	logger.Debug(ctx.Context, "OTP generated successfully")
	return execResp, nil
}

// executeVerify validates the OTP code supplied by the user and authenticates them.
func (e *otpExecutor) executeVerify(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) (*common.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	if !e.ValidatePrerequisites(ctx, execResp, e.authnProvider) {
		logger.Debug(ctx.Context, "Prerequisites not met for OTP verification")
		return execResp, nil
	}

	if !e.HasRequiredInputs(ctx, execResp) {
		logger.Debug(ctx.Context, "Required inputs for OTP verification are not provided")
		execResp.Status = common.ExecUserInputRequired
		return execResp, nil
	}

	if err := e.getAuthenticatedUser(ctx, execResp); err != nil {
		return execResp, err
	}

	logger.Debug(ctx.Context, "OTP verify completed",
		log.String("status", string(execResp.Status)))
	return execResp, nil
}

// resolveUserID identifies the user from the declared node inputs.
// For authenticated flows the userID is read directly from the AuthUser context.
// Otherwise all declared node inputs are collected and used to search the entity store.
func (e *otpExecutor) resolveUserID(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) (string, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	if ctx.AuthUser.IsAuthenticated() {
		userID := ctx.RuntimeData[userAttributeUserID]
		if userID != "" {
			return userID, nil
		}
	}

	searchAttrs := e.buildSearchAttributes(ctx)
	if len(searchAttrs) == 0 {
		logger.Debug(ctx.Context, "No searchable inputs found for user identification")
		execResp.Status = common.ExecUserInputRequired
		execResp.Inputs = e.GetRequiredInputs(ctx)
		return "", nil
	}

	identifiedUserID, providerErr := e.entityProvider.IdentifyEntity(searchAttrs)
	if providerErr != nil {
		return "", fmt.Errorf("failed to identify user: %s", providerErr.Error())
	}
	if identifiedUserID == nil || *identifiedUserID == "" {
		logger.Debug(ctx.Context, "User not found for provided inputs")
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrUserNotFound
		return "", nil
	}

	execResp.RuntimeData[userAttributeUserID] = *identifiedUserID
	return *identifiedUserID, nil
}

// buildSearchAttributes collects non-OTP input values from UserInputs, RuntimeData, and ForwardedData
// using the identifiers declared in NodeInputs.
func (e *otpExecutor) buildSearchAttributes(ctx *core.NodeContext) map[string]interface{} {
	attrs := make(map[string]interface{})

	inputs := ctx.NodeInputs
	if len(inputs) == 0 {
		inputs = e.GetRequiredInputs(ctx)
	}

	for _, input := range inputs {
		if input.Identifier == userInputOTP || !isSearchableIdentifier(input.Identifier) {
			continue
		}
		if val, ok := ctx.UserInputs[input.Identifier]; ok && val != "" {
			attrs[input.Identifier] = val
			continue
		}
		if val, ok := ctx.RuntimeData[input.Identifier]; ok && val != "" {
			attrs[input.Identifier] = val
			continue
		}
		if val, ok := ctx.ForwardedData[input.Identifier]; ok {
			if strVal, isStr := val.(string); isStr && strVal != "" {
				attrs[input.Identifier] = strVal
			}
		}
	}

	return attrs
}

// getAuthenticatedUser verifies the OTP code and authenticates the user via the authn provider.
func (e *otpExecutor) getAuthenticatedUser(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) error {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	providedOTP := ctx.UserInputs[userInputOTP]
	if providedOTP == "" {
		execResp.Status = common.ExecUserInputRequired
		execResp.Inputs = e.GetRequiredInputs(ctx)
		execResp.Error = &ErrInvalidOTP
		return nil
	}

	sessionToken := ctx.RuntimeData[common.RuntimeKeyOTPSessionToken]
	if sessionToken == "" {
		return fmt.Errorf("no OTP session token found in runtime data")
	}

	creds := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": sessionToken,
			"otp":          providedOTP,
		},
	}

	newAuthUser, authenticatedClaims, svcErr := e.authnProvider.AuthenticateUser(
		ctx.Context, nil, creds, nil, nil, execResp.AuthUser)
	execResp.AuthUser = newAuthUser
	if svcErr != nil {
		if svcErr.Code == authnprovidermgr.ErrorAuthenticationFailed.Code {
			logger.Debug(ctx.Context, "OTP verification failed")
			execResp.Status = common.ExecUserInputRequired
			execResp.Inputs = e.GetRequiredInputs(ctx)
			execResp.Error = &ErrInvalidOTP
			return nil
		}
		return fmt.Errorf("failed to verify OTP: %s", svcErr.ErrorDescription.DefaultValue)
	}

	execResp.RuntimeData[common.RuntimeKeyOTPSessionToken] = ""
	for key, value := range authenticatedClaims {
		execResp.RuntimeData[key] = systemutils.ConvertInterfaceValueToString(value)
	}
	execResp.Status = common.ExecComplete
	return nil
}

// resolveOTPDestination returns the identifier name and its value from the first PHONE_INPUT
// declared in NodeInputs. The value is looked up in UserInputs, RuntimeData, and ForwardedData.
// Used in registration flows where the user does not yet exist and cannot be identified by userID.
func (e *otpExecutor) resolveOTPDestination(ctx *core.NodeContext) (attrName, attrValue string) {
	for _, input := range ctx.NodeInputs {
		if input.Type != common.InputTypePhone || !isSearchableIdentifier(input.Identifier) {
			continue
		}
		if val, ok := ctx.UserInputs[input.Identifier]; ok && val != "" {
			return input.Identifier, val
		}
		if val, ok := ctx.RuntimeData[input.Identifier]; ok && val != "" {
			return input.Identifier, val
		}
		if val, ok := ctx.ForwardedData[input.Identifier]; ok {
			if strVal, ok := val.(string); ok && strVal != "" {
				return input.Identifier, strVal
			}
		}
	}
	return "", ""
}

// validateAttempts checks the OTP generation attempt count against the maximum allowed.
func (e *otpExecutor) validateAttempts(ctx *core.NodeContext, execResp *common.ExecutorResponse,
	logger *log.Logger) (int, error) {
	attemptCount := 0
	if countStr := ctx.RuntimeData[common.RuntimeKeyOTPAttemptCount]; countStr != "" {
		count, err := strconv.Atoi(countStr)
		if err != nil {
			return 0, fmt.Errorf("failed to parse attempt count: %w", err)
		}
		attemptCount = count
	}

	if attemptCount >= getMaxOTPAttempts(ctx) {
		logger.Debug(ctx.Context, "Maximum OTP generation attempts reached",
			log.Int("attemptCount", attemptCount))
		execResp.Status = common.ExecFailure
		execResp.Error = errMaxOTPAttemptsReachedFor(attemptCount)
		return 0, nil
	}

	return attemptCount, nil
}

// getMaxOTPAttempts returns the maximum OTP generation attempts from NodeProperties,
// falling back to 3 if not set or invalid.
func getMaxOTPAttempts(ctx *core.NodeContext) int {
	const defaultMaxAttempts = 3
	if val, ok := ctx.NodeProperties[propertyKeyMaxOTPAttempts].(string); ok && val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			return n
		}
	}
	return defaultMaxAttempts
}
