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

	"github.com/thunder-id/thunderid/internal/authn/otp"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// otpExecutor handles channel-agnostic OTP generation and verification.
// Generate mode: identifies the user, generates an OTP session token, and forwards
// the plaintext OTP via ForwardedData to a downstream sender executor.
// Verify mode: validates the OTP code against the session token and authenticates the user.
type otpExecutor struct {
	providers.Executor
	entityProvider entityprovider.EntityProviderInterface
	otpService     otp.OTPAuthnServiceInterface
	authnProvider  providers.AuthnProviderManager
	logger         *log.Logger
}

// newOTPExecutor creates a new instance of otpExecutor.
func newOTPExecutor(
	flowFactory core.FlowFactoryInterface,
	otpService otp.OTPAuthnServiceInterface,
	authnProvider providers.AuthnProviderManager,
	entityProvider entityprovider.EntityProviderInterface,
) *otpExecutor {
	defaultInputs := []providers.Input{
		{
			Ref:        "otp_input",
			Identifier: userInputOTP,
			Type:       providers.InputTypeOTP,
			Required:   true,
		},
	}
	prerequisites := []providers.Input{
		{
			Identifier: common.RuntimeKeyOTPSessionToken,
			Type:       providers.InputTypeHidden,
			Required:   true,
		},
	}

	logger := log.GetLogger().With(
		log.String(log.LoggerKeyComponentName, "OTPExecutor"),
		log.String(log.LoggerKeyExecutorName, ExecutorNameOTPExecutor),
	)

	base := flowFactory.CreateExecutor(ExecutorNameOTPExecutor, providers.ExecutorTypeAuthentication,
		defaultInputs, prerequisites, &providers.ExecutorMeta{
			SupportedModes: []string{
				ExecutorModeGenerate,
				ExecutorModeVerify,
			},
		})

	return &otpExecutor{
		Executor:       base,
		entityProvider: entityProvider,
		otpService:     otpService,
		authnProvider:  authnProvider,
		logger:         logger,
	}
}

// Execute dispatches to generate or verify mode based on ctx.ExecutorMode.
func (e *otpExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing OTP executor")

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		ForwardedData:  make(map[string]interface{}),
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
// forwards the session token and plaintext OTP value to downstream sender executors via ForwardedData.
//
// For authentication flows the recipient is the resolved userID. For registration flows
// the user does not exist yet, so when identification fails the first phone or email nodeInput
// attribute value is used directly as the recipient.
func (e *otpExecutor) executeGenerate(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	userID, err := e.resolveUserID(ctx, execResp)
	if err != nil {
		return execResp, err
	}

	recipient := userID
	recipientAttr := authnprovidercm.UserAttributeUserID

	if recipient == "" && execResp.Status == providers.ExecFailure {
		if ctx.FlowType != providers.FlowTypeRegistration {
			logger.Debug(ctx.Context, "OTP generate: user not found, non-registration flow — aborting")
			return execResp, nil
		}
		// Registration: user does not exist yet — use the declared nodeInput attribute value
		// directly as the OTP recipient so the session token is keyed to that identifier.
		destAttr, destValue := e.resolveOTPDestination(ctx)
		if destValue == "" {
			execResp.Status = providers.ExecUserInputRequired
			execResp.Error = nil
			execResp.Inputs = e.getGenerateInputs(ctx)
			return execResp, nil
		}

		execResp.Status = ""
		execResp.Error = nil
		recipient = destValue
		recipientAttr = destAttr
	}

	if execResp.Status == providers.ExecUserInputRequired {
		return execResp, nil
	}

	previousSessionToken := ctx.RuntimeData[common.RuntimeKeyOTPSessionToken]
	sessionToken, otpValue, expirySeconds, svcErr := e.otpService.GenerateOTP(
		ctx.Context, recipient, recipientAttr, previousSessionToken)
	if svcErr != nil {
		if svcErr.Code == otp.ErrorMaxOTPAttemptsExceeded.Code {
			logger.Debug(ctx.Context, "Maximum OTP generation attempts reached")
			execResp.Status = providers.ExecFailure
			execResp.Error = &ErrMaxOTPAttemptsReached
			return execResp, nil
		}
		return execResp, fmt.Errorf("failed to generate OTP: %s", svcErr.ErrorDescription.DefaultValue)
	}

	execResp.RuntimeData[common.RuntimeKeyOTPSessionToken] = sessionToken
	execResp.ForwardedData[common.ForwardedDataKeyTemplateData] = map[string]interface{}{
		common.ForwardedDataKeyOTPCode:       otpValue,
		common.ForwardedDataKeyExpiryMinutes: systemutils.SecondsToMinutes(expirySeconds),
	}
	execResp.Status = providers.ExecComplete

	logger.Debug(ctx.Context, "OTP generated successfully")
	return execResp, nil
}

// executeVerify validates the OTP code supplied by the user and authenticates them.
func (e *otpExecutor) executeVerify(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) (*providers.ExecutorResponse, error) {
	if !e.ValidatePrerequisites(ctx, execResp, e.authnProvider) {
		return execResp, nil
	}

	if err := e.getAuthenticatedUser(ctx, execResp); err != nil {
		return execResp, err
	}

	return execResp, nil
}

// resolveUserID identifies the user from the declared node inputs.
// For authenticated flows the userID is read directly from the AuthUser context.
// Otherwise all declared node inputs are collected and used to search the entity store.
func (e *otpExecutor) resolveUserID(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) (string, error) {
	if userID := ctx.RuntimeData[userAttributeUserID]; userID != "" {
		return userID, nil
	}

	if e.authnProvider != nil && ctx.AuthUser.IsAuthenticated() {
		authUser, entityRef, err := e.authnProvider.GetEntityReference(ctx.Context, ctx.AuthUser)
		execResp.AuthUser = authUser
		if err == nil && entityRef.EntityID != "" {
			execResp.RuntimeData[userAttributeUserID] = entityRef.EntityID
			return entityRef.EntityID, nil
		}
	}

	searchAttrs := e.buildSearchAttributes(ctx)
	if len(searchAttrs) == 0 {
		execResp.Status = providers.ExecUserInputRequired
		execResp.Inputs = e.getGenerateInputs(ctx)
		return "", nil
	}

	identifiedUserID, providerErr := e.entityProvider.IdentifyEntity(searchAttrs)
	if providerErr != nil {
		if providerErr.Code == entityprovider.ErrorCodeEntityNotFound {
			execResp.Status = providers.ExecFailure
			execResp.Error = &ErrUserNotFound
			return "", nil
		}
		return "", fmt.Errorf("failed to identify user: %s", providerErr.Error())
	}
	if identifiedUserID == nil || *identifiedUserID == "" {
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrUserNotFound
		return "", nil
	}

	execResp.RuntimeData[userAttributeUserID] = *identifiedUserID
	return *identifiedUserID, nil
}

// buildSearchAttributes collects non-OTP input values from UserInputs, RuntimeData, and ForwardedData
// using the identifiers declared in NodeInputs.
func (e *otpExecutor) buildSearchAttributes(ctx *providers.NodeContext) map[string]interface{} {
	attrs := make(map[string]interface{})

	inputs := ctx.NodeInputs
	if len(inputs) == 0 {
		inputs = e.getGenerateInputs(ctx)
	}

	for _, input := range inputs {
		if input.Identifier == userInputOTP || !isSearchableIdentifier(input.Identifier) {
			continue
		}
		if v, ok := ctx.UserInputs[input.Identifier]; ok && v != "" {
			attrs[input.Identifier] = v
			continue
		}
		if v, ok := ctx.RuntimeData[input.Identifier]; ok && v != "" {
			attrs[input.Identifier] = v
			continue
		}
		if v, ok := ctx.ForwardedData[input.Identifier]; ok {
			if strVal, isStr := v.(string); isStr && strVal != "" {
				attrs[input.Identifier] = strVal
			}
		}
	}

	return attrs
}

// getAuthenticatedUser verifies the OTP code via the authn provider and populates execResp.AuthUser
// so that downstream executors (e.g. AuthAssertExecutor) can resolve the entity reference.
func (e *otpExecutor) getAuthenticatedUser(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) error {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	providedOTP := ctx.UserInputs[userInputOTP]
	if providedOTP == "" {
		execResp.Status = providers.ExecUserInputRequired
		execResp.Inputs = e.GetRequiredInputs(ctx)
		execResp.Error = &ErrInvalidOTP
		return nil
	}

	sessionToken := ctx.RuntimeData[common.RuntimeKeyOTPSessionToken]
	if sessionToken == "" {
		return fmt.Errorf("no OTP session token found in runtime data")
	}

	credentials := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": sessionToken,
			"otp":          providedOTP,
		},
	}
	authUser, authenticatedClaims, svcErr := e.authnProvider.AuthenticateUser(
		ctx.Context, nil, credentials, nil, nil, execResp.AuthUser)
	if svcErr != nil {
		if svcErr.Code == authnprovidermgr.ErrorAuthenticationFailed.Code ||
			svcErr.Code == authnprovidermgr.ErrorInvalidRequest.Code {
			logger.Debug(ctx.Context, "OTP verification failed")
			execResp.Status = providers.ExecUserInputRequired
			execResp.Inputs = e.GetRequiredInputs(ctx)
			execResp.Error = &ErrInvalidOTP
			return nil
		}
		return fmt.Errorf("failed to verify OTP: %s", svcErr.ErrorDescription.DefaultValue)
	}

	execResp.AuthUser = authUser
	execResp.RuntimeData[common.RuntimeKeyOTPSessionToken] = ""
	for key, value := range authenticatedClaims {
		execResp.RuntimeData[key] = systemutils.ConvertInterfaceValueToString(value)
	}
	execResp.Status = providers.ExecComplete
	return nil
}

// resolveOTPDestination returns the identifier and value of the first phone or email node input.
// Channel is determined by input type or, as a fallback, by the well-known identifier names
// (mobile_number, email), so flows that use TEXT_INPUT for these attributes work correctly.
func (e *otpExecutor) resolveOTPDestination(ctx *providers.NodeContext) (attrName, attrValue string) {
	for _, input := range ctx.NodeInputs {
		isPhone := input.Type == providers.InputTypePhone || input.Identifier == common.AttributeMobileNumber
		isEmail := input.Type == providers.InputTypeEmail || input.Identifier == common.AttributeEmail
		if (!isPhone && !isEmail) || !isSearchableIdentifier(input.Identifier) {
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

func (e *otpExecutor) getGenerateInputs(ctx *providers.NodeContext) []providers.Input {
	if len(ctx.NodeInputs) > 0 {
		return ctx.NodeInputs
	}
	return []providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}
}
