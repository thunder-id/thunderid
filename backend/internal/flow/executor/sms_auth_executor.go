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
	"strconv"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	notifcommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// mobileNumberInput is the default input definition for mobile number collection.
var mobileNumberInput = common.Input{
	Ref:        "mobile_number_input",
	Identifier: common.AttributeMobileNumber,
	Type:       common.InputTypePhone,
	Required:   true,
}

// smsOTPAuthExecutor implements the ExecutorInterface for SMS OTP authentication.
type smsOTPAuthExecutor struct {
	core.ExecutorInterface
	identifyingExecutorInterface
	entityProvider entityprovider.EntityProviderInterface
	otpService     otp.OTPAuthnServiceInterface
	authnProvider  authnprovidermgr.AuthnProviderManagerInterface
	logger         *log.Logger
}

var _ core.ExecutorInterface = (*smsOTPAuthExecutor)(nil)
var _ identifyingExecutorInterface = (*smsOTPAuthExecutor)(nil)

// newSMSOTPAuthExecutor creates a new instance of SMSOTPAuthExecutor.
func newSMSOTPAuthExecutor(
	flowFactory core.FlowFactoryInterface,
	otpService otp.OTPAuthnServiceInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
	entityProvider entityprovider.EntityProviderInterface,
) *smsOTPAuthExecutor {
	defaultInputs := []common.Input{
		{
			Ref:        "otp_input",
			Identifier: userInputOTP,
			Type:       common.InputTypeOTP,
			Required:   true,
		},
	}
	var prerequisites []common.Input

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "SMSOTPAuthExecutor"),
		log.String(log.LoggerKeyExecutorName, ExecutorNameSMSAuth))

	identifyExec := newIdentifyingExecutor(ExecutorNameSMSAuth, defaultInputs, prerequisites,
		flowFactory, entityProvider)
	base := flowFactory.CreateExecutor(ExecutorNameSMSAuth, common.ExecutorTypeAuthentication,
		defaultInputs, prerequisites)

	return &smsOTPAuthExecutor{
		ExecutorInterface:            base,
		identifyingExecutorInterface: identifyExec,
		entityProvider:               entityProvider,
		otpService:                   otpService,
		authnProvider:                authnProvider,
		logger:                       logger,
	}
}

// Execute executes the SMS OTP authentication logic.
func (s *smsOTPAuthExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Executing SMS OTP authentication executor")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	// Determine the executor mode
	switch ctx.ExecutorMode {
	case ExecutorModeSend:
		if !s.ValidatePrerequisites(ctx, execResp) {
			logger.Debug("Prerequisites not met for SMS OTP authentication executor")
			return execResp, nil
		}
		return s.executeSend(ctx, execResp)
	case ExecutorModeVerify:
		return s.executeVerify(ctx, execResp)
	default:
		return execResp, fmt.Errorf("invalid executor mode: %s", ctx.ExecutorMode)
	}
}

// executeSend executes the OTP sending step.
func (s *smsOTPAuthExecutor) executeSend(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) (*common.ExecutorResponse, error) {
	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	err := s.InitiateOTP(ctx, execResp)
	if err != nil {
		return execResp, err
	}

	logger.Debug("SMS OTP send completed", log.String("status", string(execResp.Status)))

	return execResp, nil
}

// executeVerify executes the OTP verification step.
func (s *smsOTPAuthExecutor) executeVerify(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) (*common.ExecutorResponse, error) {
	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	if !s.HasRequiredInputs(ctx, execResp) {
		logger.Debug("Required inputs for SMS OTP verification are not provided")
		execResp.Status = common.ExecUserInputRequired
		return execResp, nil
	}

	err := s.ProcessAuthFlowResponse(ctx, execResp)
	if err != nil {
		return execResp, err
	}

	logger.Debug("SMS OTP verify completed",
		log.String("status", string(execResp.Status)),
		log.Bool("isAuthenticated", execResp.AuthenticatedUser.IsAuthenticated))

	return execResp, nil
}

// InitiateOTP initiates the OTP sending process to the user's mobile number.
func (s *smsOTPAuthExecutor) InitiateOTP(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) error {
	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Sending SMS OTP to user")

	phoneAttr := s.resolvePhoneInput(ctx, mobileNumberInput).Identifier
	mobileNumber, err := s.getUserMobileFromContext(ctx, phoneAttr)
	if err != nil {
		return err
	}

	var userID *string
	if ctx.AuthenticatedUser.IsAuthenticated {
		userIDVal := s.GetUserIDFromContext(ctx)
		if userIDVal == "" {
			return errors.New("user ID is empty in the context")
		}
		userID = &userIDVal
	} else {
		// Identify user by mobile number if not authenticated
		if mobileNumber == "" {
			logger.Error("Mobile number is empty in the context")
		}

		filter := map[string]interface{}{phoneAttr: mobileNumber}
		userID, err = s.IdentifyUser(filter, execResp)
		if err != nil {
			logger.Error("Failed to identify user", log.Error(err))
			return fmt.Errorf("failed to identify user: %w", err)
		}
	}

	// Handle registration flows.
	if ctx.FlowType == common.FlowTypeRegistration {
		if execResp.Status == common.ExecFailure && execResp.FailureReason != failureReasonUserNotFound {
			logger.Error("Failed to identify user during registration flow", log.Error(err))
			return fmt.Errorf("failed to identify user during registration flow: %w", err)
		}

		if userID != nil && *userID != "" {
			// At this point, a unique user is found in the system.
			// Prompt the user to provide a different mobile number.
			execResp.Status = common.ExecUserInputRequired
			execResp.Inputs = []common.Input{s.resolvePhoneInput(ctx, mobileNumberInput)}
			execResp.FailureReason = "User already exists with the provided mobile number."
			return nil
		}

		execResp.Status = ""
		execResp.FailureReason = ""
	} else {
		if execResp.Status == common.ExecFailure {
			return nil
		}
		execResp.RuntimeData[userAttributeUserID] = *userID
	}

	// Send the OTP to the user's mobile number.
	if err := s.generateAndSendOTP(mobileNumber, ctx, execResp, logger); err != nil {
		logger.Error("Failed to send OTP", log.Error(err))
		return fmt.Errorf("failed to send OTP: %w", err)
	}
	if execResp.Status == common.ExecFailure {
		return nil
	}

	logger.Debug("SMS OTP sent successfully")
	execResp.RuntimeData[common.RuntimeKeySMSOTPMobileNumber] = mobileNumber
	execResp.RuntimeData[common.RuntimeKeySMSOTPPhoneAttr] = phoneAttr
	execResp.Status = common.ExecComplete

	return nil
}

// ProcessAuthFlowResponse processes the authentication flow response for SMS OTP.
func (s *smsOTPAuthExecutor) ProcessAuthFlowResponse(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) error {
	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Processing authentication flow response for SMS OTP")

	authenticatedUser, err := s.getAuthenticatedUser(ctx, execResp)
	if err != nil {
		logger.Error("Failed to get authenticated user details", log.Error(err))
		return fmt.Errorf("failed to get authenticated user details: %w", err)
	}
	if execResp.Status == common.ExecFailure || execResp.Status == common.ExecUserInputRequired {
		return nil
	}

	execResp.AuthenticatedUser = *authenticatedUser
	execResp.Status = common.ExecComplete

	return nil
}

// ValidatePrerequisites validates whether the prerequisites for the SMSOTPAuthExecutor are met.
func (s *smsOTPAuthExecutor) ValidatePrerequisites(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) bool {
	if s.isPhonePrerequisiteMet(ctx) {
		return true
	}

	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	if ctx.FlowType == common.FlowTypeRegistration {
		logger.Debug("Prerequisites not met for registration flow, prompting for mobile number")
		execResp.Status = common.ExecUserInputRequired
		execResp.Inputs = []common.Input{s.resolvePhoneInput(ctx, mobileNumberInput)}
		return false
	}

	logger.Debug("Trying to satisfy prerequisites for SMS OTP authentication executor")

	s.satisfyPrerequisites(ctx, execResp)
	if execResp.Status == common.ExecFailure {
		return false
	}

	return s.isPhonePrerequisiteMet(ctx)
}

// isPhonePrerequisiteMet checks whether the resolved phone attribute is present in the context.
func (s *smsOTPAuthExecutor) isPhonePrerequisiteMet(ctx *core.NodeContext) bool {
	phoneAttr := s.resolvePhoneInput(ctx, mobileNumberInput).Identifier
	if val, ok := ctx.UserInputs[phoneAttr]; ok && val != "" {
		return true
	}
	if val, ok := ctx.RuntimeData[phoneAttr]; ok && val != "" {
		return true
	}
	if val, ok := ctx.ForwardedData[phoneAttr]; ok {
		if strVal, isString := val.(string); isString && strVal != "" {
			return true
		}
	}
	return false
}

// resolvePhoneInput returns the PHONE_INPUT definition from the node context inputs,
// falling back to the provided default if none is found.
func (s *smsOTPAuthExecutor) resolvePhoneInput(ctx *core.NodeContext, fallback common.Input) common.Input {
	for _, input := range ctx.NodeInputs {
		if input.Type == common.InputTypePhone {
			return input
		}
	}
	return fallback
}

// getUserMobileFromContext retrieves the user's mobile number from the context.
func (s *smsOTPAuthExecutor) getUserMobileFromContext(ctx *core.NodeContext, phoneAttr string) (string, error) {
	mobileNumber := ctx.RuntimeData[phoneAttr]

	if mobileNumber == "" {
		mobileNumber = ctx.UserInputs[phoneAttr]
	}

	if mobileNumber == "" && ctx.AuthenticatedUser.Attributes != nil {
		if mobile, ok := ctx.AuthenticatedUser.Attributes[phoneAttr]; ok {
			if mobileStr, valid := mobile.(string); valid && mobileStr != "" {
				mobileNumber = mobileStr
			}
		}
	}

	if mobileNumber == "" {
		return "", errors.New("mobile number not found in context")
	}
	return mobileNumber, nil
}

// satisfyPrerequisites tries to satisfy the prerequisites for the SMSOTPAuthExecutor.
func (s *smsOTPAuthExecutor) satisfyPrerequisites(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) {
	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	execResp.Status = ""
	execResp.FailureReason = ""

	logger.Debug("Trying to resolve user ID from context data")
	userIDResolved, err := s.resolveUserID(ctx)
	if err != nil {
		logger.Error("Failed to resolve user ID from context data", log.Error(err))
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Failed to resolve user ID from context data"
		return
	}
	if !userIDResolved {
		logger.Debug("User ID could not be resolved from context data")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "User ID could not be resolved from context data"
		return
	}
	userID := ctx.RuntimeData[userAttributeUserID]

	// TODO: If the mobile number is not found, but the user is authenticated, this method will
	//  prompt the user to enter their mobile number.
	//  We should verify whether this is the expected behavior.

	logger.Debug("Retrieving mobile number from user ID", log.MaskedString(log.LoggerKeyUserID, userID))
	mobileNumber, err := s.getUserMobileNumber(userID, ctx, execResp)
	if err != nil {
		logger.Error("Failed to retrieve mobile number", log.MaskedString(log.LoggerKeyUserID, userID), log.Error(err))
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Failed to retrieve mobile number"
		return
	}
	if execResp.Status == common.ExecFailure {
		return
	}

	logger.Debug("Mobile number retrieved successfully", log.MaskedString(log.LoggerKeyUserID, userID))
	ctx.RuntimeData[s.resolvePhoneInput(ctx, mobileNumberInput).Identifier] = mobileNumber

	// Reset the executor response status and failure reason.
	execResp.Status = ""
	execResp.FailureReason = ""
}

// resolveUserID resolves the user ID from the context based on various attributes.
// TODO: Move to a separate resolver when the support is added.
func (s *smsOTPAuthExecutor) resolveUserID(ctx *core.NodeContext) (bool, error) {
	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	// First, check if the user ID is already available in the context.
	userID := s.GetUserIDFromContext(ctx)
	if userID != "" {
		logger.Debug("User ID found in context data", log.MaskedString(log.LoggerKeyUserID, userID))
		if ctx.RuntimeData == nil {
			ctx.RuntimeData = make(map[string]string)
		}
		ctx.RuntimeData[userAttributeUserID] = userID

		return true, nil
	}

	userIDResolved := false

	// Try to resolve user ID from mobile number next.
	phoneAttr := s.resolvePhoneInput(ctx, mobileNumberInput).Identifier
	userIDResolved, err := s.resolveUserIDFromAttribute(ctx, phoneAttr, logger)
	if err != nil {
		return false, err
	}
	if userIDResolved {
		return true, nil
	}

	// Try to resolve user ID from username first.
	userIDResolved, err = s.resolveUserIDFromAttribute(ctx, userAttributeUsername, logger)
	if err != nil {
		return false, err
	}
	if userIDResolved {
		return true, nil
	}

	// Try to resolve user ID from email next.
	userIDResolved, err = s.resolveUserIDFromAttribute(ctx, userAttributeEmail, logger)
	if err != nil {
		return false, err
	}
	if userIDResolved {
		return true, nil
	}

	return false, nil
}

// resolveUserIDFromAttribute attempts to resolve the user ID from a specific attribute in the context.
func (s *smsOTPAuthExecutor) resolveUserIDFromAttribute(ctx *core.NodeContext,
	attributeName string, logger *log.Logger) (bool, error) {
	logger.Debug("Resolving user ID from attribute", log.String("attributeName", attributeName))

	attributeValue := ctx.UserInputs[attributeName]
	if attributeValue == "" {
		attributeValue = ctx.RuntimeData[attributeName]
	}
	if attributeValue != "" {
		filters := map[string]interface{}{attributeName: attributeValue}
		userID, providerErr := s.entityProvider.IdentifyEntity(filters)
		if providerErr != nil {
			return false, fmt.Errorf("failed to identify user by %s: %s", attributeName, providerErr.Error())
		}
		if userID != nil && *userID != "" {
			logger.Debug("User ID resolved from attribute", log.String("attributeName", attributeName),
				log.MaskedString(log.LoggerKeyUserID, *userID))
			if ctx.RuntimeData == nil {
				ctx.RuntimeData = make(map[string]string)
			}
			ctx.RuntimeData[userAttributeUserID] = *userID
			return true, nil
		}
	}

	return false, nil
}

// getUserMobileNumber retrieves the mobile number for the given user ID.
func (s *smsOTPAuthExecutor) getUserMobileNumber(userID string, ctx *core.NodeContext,
	execResp *common.ExecutorResponse) (string, error) {
	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID),
		log.MaskedString(log.LoggerKeyUserID, userID))
	logger.Debug("Retrieving user mobile number")

	// Try to get mobile number from context
	phoneAttr := s.resolvePhoneInput(ctx, mobileNumberInput).Identifier
	mobileNumber, err := s.getUserMobileFromContext(ctx, phoneAttr)
	if err == nil && mobileNumber != "" {
		logger.Debug("Mobile number found in context, skipping user store call")
		return mobileNumber, nil
	}

	// Mobile number not in context, fetch from user store
	logger.Debug("Mobile number not in context, fetching from user store")
	user, providerErr := s.entityProvider.GetEntity(userID)
	if providerErr != nil {
		return "", fmt.Errorf("failed to retrieve user details: %s", providerErr.Error())
	}

	// Extract mobile number from user attributes
	attrs := make(map[string]interface{})
	if len(user.Attributes) > 0 {
		if err := json.Unmarshal(user.Attributes, &attrs); err != nil {
			return "", fmt.Errorf("failed to unmarshal user attributes: %w", err)
		}
	}

	mobileNumber = ""
	mobileNumberAttr := attrs[s.resolvePhoneInput(ctx, mobileNumberInput).Identifier]
	if mobileStr, ok := mobileNumberAttr.(string); ok && mobileStr != "" {
		mobileNumber = mobileStr
	}

	if mobileNumber == "" {
		logger.Debug("Mobile number not found in user attributes or context")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Mobile number not found in user attributes or context"
		return "", nil
	}

	return mobileNumber, nil
}

// generateAndSendOTP generates an OTP and sends it to the user's mobile number.
func (s *smsOTPAuthExecutor) generateAndSendOTP(mobileNumber string, ctx *core.NodeContext,
	execResp *common.ExecutorResponse, logger *log.Logger) error {
	attemptCount, err := s.validateAttempts(ctx, execResp, logger)
	if err != nil {
		return fmt.Errorf("failed to validate OTP attempts: %w", err)
	}
	if execResp.Status == common.ExecFailure {
		return nil
	}

	// Get the message sender id from node properties.
	if len(ctx.NodeProperties) == 0 {
		return errors.New("message sender id is not configured in node properties")
	}

	senderID := ""
	if senderIDVal, ok := ctx.NodeProperties["senderId"]; ok {
		if sid, valid := senderIDVal.(string); valid && sid != "" {
			senderID = sid
		}
	}
	if senderID == "" {
		return errors.New("senderId is not configured in node properties")
	}

	// Send the OTP
	sessionToken, svcErr := s.otpService.SendOTP(ctx.Context, senderID, notifcommon.ChannelTypeSMS, mobileNumber)
	if svcErr != nil {
		return fmt.Errorf("failed to send OTP: %s", svcErr.ErrorDescription.DefaultValue)
	}

	// Store runtime data
	if execResp.RuntimeData == nil {
		execResp.RuntimeData = make(map[string]string)
	}
	execResp.RuntimeData["otpSessionToken"] = sessionToken
	execResp.RuntimeData["attemptCount"] = strconv.Itoa(attemptCount + 1)

	return nil
}

// validateAttempts checks if the maximum number of OTP attempts has been reached.
func (s *smsOTPAuthExecutor) validateAttempts(ctx *core.NodeContext, execResp *common.ExecutorResponse,
	logger *log.Logger) (int, error) {
	userID := ctx.RuntimeData[userAttributeUserID]
	attemptCount := 0

	attemptCountStr := ctx.RuntimeData["attemptCount"]
	if attemptCountStr != "" {
		count, err := strconv.Atoi(attemptCountStr)
		if err != nil {
			logger.Error("Failed to parse attempt count", log.Error(err))
			return 0, fmt.Errorf("failed to parse attempt count: %w", err)
		}
		attemptCount = count
	}

	if attemptCount >= s.getOTPMaxAttempts() {
		logger.Debug("Maximum OTP attempts reached", log.MaskedString(log.LoggerKeyUserID, userID),
			log.Int("attemptCount", attemptCount))
		execResp.Status = common.ExecFailure
		execResp.FailureReason = fmt.Sprintf("maximum OTP attempts reached: %d", attemptCount)
		return 0, nil
	}

	return attemptCount, nil
}

// getOTPMaxAttempts returns the maximum number of attempts allowed for OTP validation.
func (s *smsOTPAuthExecutor) getOTPMaxAttempts() int {
	// TODO: This needs to be configured as a IDP property.
	return 3
}

// getAuthenticatedUser returns the authenticated user details for the given user ID.
func (s *smsOTPAuthExecutor) getAuthenticatedUser(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) (*authncm.AuthenticatedUser, error) {
	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	phoneAttr := ctx.RuntimeData[common.RuntimeKeySMSOTPPhoneAttr]
	if phoneAttr == "" {
		phoneAttr = s.resolvePhoneInput(ctx, mobileNumberInput).Identifier
	}
	mobileNumber := ctx.RuntimeData[common.RuntimeKeySMSOTPMobileNumber]
	if mobileNumber == "" {
		return nil, errors.New("mobile number not found in context")
	}

	userID := ctx.RuntimeData[userAttributeUserID]

	logger.Debug("Validating OTP", log.MaskedString(log.LoggerKeyUserID, userID))

	providedOTP := ctx.UserInputs[userInputOTP]
	if providedOTP == "" {
		logger.Debug("Provided OTP is empty", log.MaskedString(log.LoggerKeyUserID, userID))
		execResp.Status = common.ExecUserInputRequired
		execResp.Inputs = s.GetRequiredInputs(ctx)
		execResp.FailureReason = failureReasonInvalidOTP
		return nil, nil
	}

	sessionToken := ctx.RuntimeData["otpSessionToken"]
	if sessionToken == "" {
		logger.Error("No session token found for OTP validation", log.MaskedString(log.LoggerKeyUserID, userID))
		return nil, fmt.Errorf("no session token found for OTP validation")
	}

	// Handle registration flows.
	if ctx.FlowType == common.FlowTypeRegistration {
		// For registration flows, we don't have a user in the system yet.
		// So we just validate the OTP and return an authenticated user with the mobile number as an attribute.
		svcErr := s.otpService.VerifyOTP(ctx.Context, sessionToken, providedOTP)
		if svcErr != nil {
			if svcErr.Code == otp.ErrorIncorrectOTP.Code {
				logger.Debug("OTP verification failed", log.MaskedString(log.LoggerKeyUserID, userID))
				execResp.Status = common.ExecUserInputRequired
				execResp.Inputs = s.GetRequiredInputs(ctx)
				execResp.FailureReason = failureReasonInvalidOTP
				return nil, nil
			}
			logger.Error("Failed to verify OTP",
				log.MaskedString(log.LoggerKeyUserID, userID), log.Any("serviceError", svcErr))
			return nil, fmt.Errorf("failed to verify OTP: %s", svcErr.ErrorDescription.DefaultValue)
		}

		execResp.Status = common.ExecComplete
		execResp.FailureReason = ""
		return &authncm.AuthenticatedUser{
			IsAuthenticated: false,
			Attributes: map[string]interface{}{
				phoneAttr: mobileNumber,
			},
		}, nil
	}

	creds := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": sessionToken,
			"otp":          providedOTP,
		},
	}
	newAuthUser, authnResult, svcErr := s.authnProvider.AuthenticateUser(
		ctx.Context, nil, creds, nil, nil, ctx.AuthUser)
	if svcErr != nil {
		if svcErr.Code == authnprovidermgr.ErrorAuthenticationFailed.Code {
			logger.Debug("OTP verification failed", log.MaskedString(log.LoggerKeyUserID, userID))
			execResp.Status = common.ExecUserInputRequired
			execResp.Inputs = s.GetRequiredInputs(ctx)
			execResp.FailureReason = failureReasonInvalidOTP
			return nil, nil
		}
		logger.Error("Failed to verify OTP",
			log.MaskedString(log.LoggerKeyUserID, userID), log.Any("serviceError", svcErr))
		return nil, fmt.Errorf("failed to verify OTP: %s", svcErr.ErrorDescription.DefaultValue)
	}
	execResp.AuthUser = newAuthUser

	execResp.RuntimeData["otpSessionToken"] = ""
	logger.Debug("OTP validated successfully", log.MaskedString(log.LoggerKeyUserID, userID))

	// Check if user is already authenticated
	if ctx.AuthenticatedUser.IsAuthenticated && ctx.AuthenticatedUser.UserID != "" {
		if ctx.AuthenticatedUser.Attributes == nil {
			ctx.AuthenticatedUser.Attributes = make(map[string]interface{})
		}
		ctx.AuthenticatedUser.Attributes[phoneAttr] = mobileNumber
		return &ctx.AuthenticatedUser, nil
	}

	// User not available in context, try to retrieve the user and get the attributes
	userID = authnResult.UserID

	logger.Debug("Fetching user details from user store", log.MaskedString(log.LoggerKeyUserID, userID))

	attrs := map[string]interface{}{}
	user, err := s.entityProvider.GetEntity(userID)
	if err != nil {
		if err.Code != entityprovider.ErrorCodeNotImplemented {
			logger.Error("Failed to get user attributes", log.Error(err))
			return nil, errors.New("failed to get user attributes")
		}
		logger.Debug("User provider is not implemented. User attributes will be empty.")
	}

	if err == nil && user != nil {
		if err := json.Unmarshal(user.Attributes, &attrs); err != nil {
			logger.Error("Failed to unmarshal user attributes", log.Error(err))
			return nil, errors.New("failed to unmarshal user attributes")
		}
	}

	authenticatedUser := &authncm.AuthenticatedUser{
		IsAuthenticated: true,
		UserID:          user.ID,
		OUID:            user.OUID,
		UserType:        user.Type,
		Attributes:      attrs,
	}

	return authenticatedUser, nil
}
