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
	"strconv"

	"github.com/thunder-id/thunderid/internal/authn/otp"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	notifcommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
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
	logger.Debug(ctx.Context, "Executing SMS OTP authentication executor")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		AuthUser:       ctx.AuthUser,
	}

	// Determine the executor mode
	switch ctx.ExecutorMode {
	case ExecutorModeSend:
		mobileNumber, err := s.getUserMobileFromContext(ctx, execResp)
		if err != nil {
			logger.Error(ctx.Context, "Failed to retrieve user mobile number from context",
				log.Error(err))
			return execResp, fmt.Errorf("failed to retrieve user mobile number from context: %w", err)
		}
		if mobileNumber == "" {
			logger.Debug(ctx.Context,
				"Prerequisites not met for SMS OTP authentication executor")
			if ctx.FlowType == common.FlowTypeRegistration {
				logger.Debug(ctx.Context,
					"Prerequisites not met for registration flow, prompting for mobile number")
				execResp.Status = common.ExecUserInputRequired
				execResp.Inputs = []common.Input{s.resolvePhoneInput(ctx, mobileNumberInput)}
				return execResp, nil
			}
			execResp.Status = common.ExecFailure
			execResp.Error = &ErrPrerequisitesFailed
		}
		return s.executeSend(ctx, execResp, mobileNumber)
	case ExecutorModeVerify:
		return s.executeVerify(ctx, execResp)
	default:
		return execResp, fmt.Errorf("invalid executor mode: %s", ctx.ExecutorMode)
	}
}

// executeSend executes the OTP sending step.
func (s *smsOTPAuthExecutor) executeSend(ctx *core.NodeContext,
	execResp *common.ExecutorResponse, mobileNumber string) (*common.ExecutorResponse, error) {
	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	err := s.InitiateOTP(ctx, execResp, mobileNumber)
	if err != nil {
		return execResp, err
	}

	logger.Debug(ctx.Context, "SMS OTP send completed", log.String("status", string(execResp.Status)))

	return execResp, nil
}

// executeVerify executes the OTP verification step.
func (s *smsOTPAuthExecutor) executeVerify(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) (*common.ExecutorResponse, error) {
	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	if !s.HasRequiredInputs(ctx, execResp) {
		logger.Debug(ctx.Context, "Required inputs for SMS OTP verification are not provided")
		execResp.Status = common.ExecUserInputRequired
		return execResp, nil
	}

	err := s.ProcessAuthFlowResponse(ctx, execResp)
	if err != nil {
		return execResp, err
	}

	logger.Debug(ctx.Context, "SMS OTP verify completed",
		log.String("status", string(execResp.Status)),
		log.Bool("isAuthenticated", execResp.AuthUser.IsAuthenticated()))

	return execResp, nil
}

// InitiateOTP initiates the OTP sending process to the user's mobile number.
func (s *smsOTPAuthExecutor) InitiateOTP(ctx *core.NodeContext,
	execResp *common.ExecutorResponse, mobileNumber string) error {
	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Sending SMS OTP to user")

	phoneAttr := s.resolvePhoneInput(ctx, mobileNumberInput).Identifier

	var userID *string
	if ctx.AuthUser.IsAuthenticated() {
		userIDVal := s.GetUserIDFromContext(ctx, execResp, s.authnProvider)
		if userIDVal != "" {
			userID = &userIDVal
		}
	}

	if userID == nil {
		// Identify user by mobile number if not already identified from context
		if mobileNumber == "" {
			logger.Error(ctx.Context, "Mobile number is empty in the context")
		}

		filter := map[string]interface{}{phoneAttr: mobileNumber}
		var err error
		userID, err = s.IdentifyUser(ctx.Context, filter, execResp)
		if err != nil {
			logger.Error(ctx.Context, "Failed to identify user", log.Error(err))
			return fmt.Errorf("failed to identify user: %w", err)
		}
	}

	// Handle registration flows.
	if ctx.FlowType == common.FlowTypeRegistration {
		if execResp.Status == common.ExecFailure &&
			(execResp.Error == nil || execResp.Error.Code != ErrUserNotFound.Code) {
			if execResp.Error != nil {
				return fmt.Errorf("failed to identify user during registration flow: %s, error code: %s",
					execResp.Error.ErrorDescription.DefaultValue, execResp.Error.Code)
			}
			return fmt.Errorf("failed to identify user during registration flow")
		}

		if userID != nil && *userID != "" {
			// At this point, a unique user is found in the system.
			// Prompt the user to provide a different mobile number.
			execResp.Status = common.ExecUserInputRequired
			execResp.Inputs = []common.Input{s.resolvePhoneInput(ctx, mobileNumberInput)}
			execResp.Error = serviceerror.CustomServiceError(ErrUserAlreadyExists, i18ncore.I18nMessage{
				Key:          ErrUserAlreadyExists.ErrorDescription.Key,
				DefaultValue: "User already exists with the provided mobile number",
			})
			return nil
		}

		execResp.Status = ""
		execResp.Error = nil
	} else {
		if execResp.Status == common.ExecFailure {
			return nil
		}
		execResp.RuntimeData[userAttributeUserID] = *userID
	}

	// Send the OTP to the user's mobile number.
	if err := s.generateAndSendOTP(mobileNumber, ctx, execResp, logger); err != nil {
		logger.Error(ctx.Context, "Failed to send OTP", log.Error(err))
		return fmt.Errorf("failed to send OTP: %w", err)
	}
	if execResp.Status == common.ExecFailure {
		return nil
	}

	logger.Debug(ctx.Context, "SMS OTP sent successfully")
	execResp.RuntimeData[common.RuntimeKeySMSOTPMobileNumber] = mobileNumber
	execResp.RuntimeData[common.RuntimeKeySMSOTPPhoneAttr] = phoneAttr
	execResp.Status = common.ExecComplete

	return nil
}

// ProcessAuthFlowResponse processes the authentication flow response for SMS OTP.
func (s *smsOTPAuthExecutor) ProcessAuthFlowResponse(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) error {
	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Processing authentication flow response for SMS OTP")

	err := s.getAuthenticatedUser(ctx, execResp)
	if err != nil {
		logger.Error(ctx.Context, "Failed to get authenticated user details", log.Error(err))
		return fmt.Errorf("failed to get authenticated user details: %w", err)
	}
	if execResp.Status == common.ExecFailure || execResp.Status == common.ExecUserInputRequired {
		return nil
	}
	execResp.Status = common.ExecComplete

	return nil
}

// ValidatePrerequisites validates whether the prerequisites for the SMSOTPAuthExecutor are met.
func (s *smsOTPAuthExecutor) ValidatePrerequisites(ctx *core.NodeContext,
	execResp *common.ExecutorResponse,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface) bool {
	if s.isPhonePrerequisiteMet(ctx) {
		return true
	}

	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	if ctx.FlowType == common.FlowTypeRegistration {
		logger.Debug(ctx.Context,
			"Prerequisites not met for registration flow, prompting for mobile number")
		execResp.Status = common.ExecUserInputRequired
		execResp.Inputs = []common.Input{s.resolvePhoneInput(ctx, mobileNumberInput)}
		return false
	}

	logger.Debug(ctx.Context,
		"Trying to satisfy prerequisites for SMS OTP authentication executor")

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
func (s *smsOTPAuthExecutor) getUserMobileFromContext(
	ctx *core.NodeContext, execResp *common.ExecutorResponse) (string, error) {
	mobileAttrName := s.resolvePhoneInput(ctx, mobileNumberInput).Identifier

	mobileNumber := ctx.RuntimeData[mobileAttrName]

	if mobileNumber == "" {
		mobileNumber = ctx.UserInputs[mobileAttrName]
	}

	if mobileNumber == "" {
		if val, ok := ctx.ForwardedData[mobileAttrName]; ok {
			if strVal, isString := val.(string); isString && strVal != "" {
				mobileNumber = strVal
			}
		}
	}

	if mobileNumber == "" {
		mobileNumber = s.resolveUserMobileNumber(ctx, mobileAttrName)
	}

	if mobileNumber == "" && execResp.AuthUser.IsAuthenticated() {
		authUser, attributes, svsErr := s.authnProvider.GetUserAttributes(ctx.Context, nil, nil, execResp.AuthUser)
		execResp.AuthUser = authUser
		if svsErr != nil {
			return "", fmt.Errorf("failed to get authenticated user attributes: %v", svsErr)
		}
		if attributes != nil && attributes.Attributes != nil {
			if attr, ok := attributes.Attributes[mobileAttrName]; ok {
				if strVal, isString := attr.Value.(string); isString && strVal != "" {
					mobileNumber = strVal
				}
			}
		}
	}

	// Store the resolved mobile number in runtime data for downstream use.
	if ctx.RuntimeData == nil {
		ctx.RuntimeData = make(map[string]string)
	}
	ctx.RuntimeData[mobileAttrName] = mobileNumber

	return mobileNumber, nil
}

// satisfyPrerequisites tries to satisfy the prerequisites for the SMSOTPAuthExecutor.
func (s *smsOTPAuthExecutor) satisfyPrerequisites(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) {
	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	execResp.Status = ""
	execResp.Error = nil

	logger.Debug(ctx.Context, "Trying to resolve user ID from context data")
	userIDResolved, err := s.resolveUserID(ctx)
	if err != nil {
		logger.Error(ctx.Context, "Failed to resolve user ID from context data", log.Error(err))
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrUserIDMissingInContext
		return
	}
	if !userIDResolved {
		logger.Debug(ctx.Context, "User ID could not be resolved from context data")
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrUserIDMissingInContext
		return
	}
	userID := ctx.RuntimeData[userAttributeUserID]

	logger.Debug(ctx.Context, "Retrieving mobile number from user ID",
		log.MaskedString(log.LoggerKeyUserID, userID))
	mobileNumber, err := s.getUserMobileNumber(userID, ctx, execResp)
	if err != nil {
		logger.Error(ctx.Context, "Failed to retrieve mobile number",
			log.MaskedString(log.LoggerKeyUserID, userID), log.Error(err))
		execResp.Status = common.ExecFailure
		execResp.Error = errFailedToRetrieveAttribute("mobile number")
		return
	}
	if execResp.Status == common.ExecFailure {
		return
	}

	logger.Debug(ctx.Context, "Mobile number retrieved successfully",
		log.MaskedString(log.LoggerKeyUserID, userID))
	ctx.RuntimeData[s.resolvePhoneInput(ctx, mobileNumberInput).Identifier] = mobileNumber

	// Reset the executor response status and error.
	execResp.Status = ""
	execResp.Error = nil
}

// resolveUserID resolves the user ID from the context based on various attributes.
func (s *smsOTPAuthExecutor) resolveUserID(ctx *core.NodeContext) (bool, error) {
	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	// First, check if the user ID is already available in the context.
	userID := s.GetUserIDFromContext(ctx, nil, s.authnProvider)
	if userID != "" {
		logger.Debug(ctx.Context, "User ID found in context data",
			log.MaskedString(log.LoggerKeyUserID, userID))
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
	logger.Debug(ctx.Context, "Resolving user ID from attribute",
		log.String("attributeName", attributeName))

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
			logger.Debug(ctx.Context, "User ID resolved from attribute",
				log.String("attributeName", attributeName),
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
	logger.Debug(ctx.Context, "Retrieving user mobile number")

	// Try to get mobile number from context
	phoneAttr := s.resolvePhoneInput(ctx, mobileNumberInput).Identifier
	mobileNumber := ctx.RuntimeData[phoneAttr]
	if mobileNumber == "" {
		mobileNumber = ctx.UserInputs[phoneAttr]
	}
	if mobileNumber == "" {
		if val, ok := ctx.ForwardedData[phoneAttr]; ok {
			if strVal, isString := val.(string); isString && strVal != "" {
				mobileNumber = strVal
			}
		}
	}
	if mobileNumber != "" {
		logger.Debug(ctx.Context, "Mobile number found in context, skipping user store call")
		return mobileNumber, nil
	}

	// Mobile number not in context, fetch from user store
	logger.Debug(ctx.Context, "Mobile number not in context, fetching from user store")
	user, providerErr := s.entityProvider.GetEntity(userID)
	if providerErr != nil {
		return "", fmt.Errorf("failed to retrieve user details: %s", providerErr.Error())
	}

	mobileNumber, err := GetUserAttribute(user, s.resolvePhoneInput(ctx, mobileNumberInput).Identifier)
	if err != nil {
		logger.Debug(ctx.Context, "Mobile number not found in user attributes or context")
		execResp.Status = common.ExecFailure
		execResp.Error = errAttributeNotFoundFor("mobile")
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
			logger.Error(ctx.Context, "Failed to parse attempt count", log.Error(err))
			return 0, fmt.Errorf("failed to parse attempt count: %w", err)
		}
		attemptCount = count
	}

	if attemptCount >= s.getOTPMaxAttempts() {
		logger.Debug(ctx.Context, "Maximum OTP attempts reached",
			log.MaskedString(log.LoggerKeyUserID, userID),
			log.Int("attemptCount", attemptCount))
		execResp.Status = common.ExecFailure
		execResp.Error = errMaxOTPAttemptsReachedFor(attemptCount)
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
	execResp *common.ExecutorResponse) error {
	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	mobileNumber := ctx.RuntimeData[common.RuntimeKeySMSOTPMobileNumber]
	if mobileNumber == "" {
		return errors.New("mobile number not found in context")
	}

	userID := ctx.RuntimeData[userAttributeUserID]

	logger.Debug(ctx.Context, "Validating OTP", log.MaskedString(log.LoggerKeyUserID, userID))

	providedOTP := ctx.UserInputs[userInputOTP]
	if providedOTP == "" {
		logger.Debug(ctx.Context, "Provided OTP is empty",
			log.MaskedString(log.LoggerKeyUserID, userID))
		execResp.Status = common.ExecUserInputRequired
		execResp.Inputs = s.GetRequiredInputs(ctx)
		execResp.Error = &ErrInvalidOTP
		return nil
	}

	sessionToken := ctx.RuntimeData["otpSessionToken"]
	if sessionToken == "" {
		logger.Error(ctx.Context, "No session token found for OTP validation",
			log.MaskedString(log.LoggerKeyUserID, userID))
		return fmt.Errorf("no session token found for OTP validation")
	}

	creds := map[string]interface{}{
		"otp": map[string]interface{}{
			"sessionToken": sessionToken,
			"otp":          providedOTP,
		},
	}

	newAuthUser, authenticatedClaims, svcErr := s.authnProvider.AuthenticateUser(
		ctx.Context, nil, creds, nil, nil, execResp.AuthUser)
	execResp.AuthUser = newAuthUser
	if svcErr != nil {
		if svcErr.Code == authnprovidermgr.ErrorAuthenticationFailed.Code {
			logger.Debug(ctx.Context, "OTP verification failed",
				log.MaskedString(log.LoggerKeyUserID, userID))
			execResp.Status = common.ExecUserInputRequired
			execResp.Inputs = s.GetRequiredInputs(ctx)
			execResp.Error = &ErrInvalidOTP
			return nil
		}
		logger.Error(ctx.Context, "Failed to verify OTP",
			log.MaskedString(log.LoggerKeyUserID, userID), log.Any("serviceError", svcErr))
		return fmt.Errorf("failed to verify OTP: %s", svcErr.ErrorDescription.DefaultValue)
	}
	execResp.RuntimeData["otpSessionToken"] = ""
	logger.Debug(ctx.Context, "OTP validated successfully",
		log.MaskedString(log.LoggerKeyUserID, userID))
	for key, value := range authenticatedClaims {
		execResp.RuntimeData[key] = systemutils.ConvertInterfaceValueToString(value)
	}
	execResp.Status = common.ExecComplete
	execResp.Error = nil
	return nil
}

// resolveUserMobileNumber attempts to resolve the user's mobile number by identifying
// the user from available context attributes (username, email) and fetching
// the mobile number from the user store.
func (s *smsOTPAuthExecutor) resolveUserMobileNumber(ctx *core.NodeContext, mobileAttrName string) string {
	logger := s.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	userID := ctx.RuntimeData[userAttributeUserID]
	if userID == "" {
		userID = s.resolveUserIDFromAttributeSimple(ctx, userAttributeUsername, logger)
	}
	if userID == "" {
		userID = s.resolveUserIDFromAttributeSimple(ctx, userAttributeEmail, logger)
	}
	if userID == "" {
		return ""
	}

	user, providerErr := s.entityProvider.GetEntity(userID)
	if providerErr != nil {
		logger.Error(ctx.Context, "Failed to retrieve user details", log.Error(providerErr))
		return ""
	}

	mobileNumber, err := GetUserAttribute(user, mobileAttrName)
	if err != nil {
		logger.Debug(ctx.Context, "Mobile number not found in user attributes", log.Error(err))
		return ""
	}

	return mobileNumber
}

// resolveUserIDFromAttributeSimple attempts to resolve a user ID by looking up a specific
// attribute value from user inputs or runtime data. Returns the user ID string directly.
func (s *smsOTPAuthExecutor) resolveUserIDFromAttributeSimple(ctx *core.NodeContext,
	attributeName string, logger *log.Logger) string {
	attributeValue := ctx.UserInputs[attributeName]
	if attributeValue == "" {
		attributeValue = ctx.RuntimeData[attributeName]
	}
	if attributeValue == "" {
		return ""
	}

	filters := map[string]interface{}{attributeName: attributeValue}
	userID, providerErr := s.entityProvider.IdentifyEntity(filters)
	if providerErr != nil {
		logger.Error(ctx.Context, "Failed to identify user by attribute",
			log.String("attributeName", attributeName), log.Error(providerErr))
		return ""
	}
	if userID != nil && *userID != "" {
		return *userID
	}

	return ""
}
