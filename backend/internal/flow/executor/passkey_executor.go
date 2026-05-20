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
	"errors"
	"fmt"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	passkeyExecutorModeChallenge = "challenge"
	passkeyExecutorModeVerify    = "verify"
	passkeyExecutorModeRegStart  = "register_start"
	passkeyExecutorModeRegFinish = "register_finish"
	errorInvalidPasskey          = "invalid passkey credentials provided"
)

// Passkey authentication input identifiers
const (
	// nolint:gosec // G101: This is a JSON field identifier, not a credential
	inputCredentialID      = "credentialId"
	inputClientDataJSON    = "clientDataJSON"
	inputAuthenticatorData = "authenticatorData"
	inputSignature         = "signature"
	inputUserHandle        = "userHandle"
)

// Passkey registration input identifiers
const (
	inputAttestationObject = "attestationObject"
	// nolint:gosec // G101: This is a JSON field identifier, not a credential
	inputCredentialName = "credentialName"
)

// Runtime data keys
const (
	runtimePasskeySessionToken    = "passkeySessionToken"
	runtimePasskeyChallenge       = "passkeyChallenge"
	runtimePasskeyCreationOptions = "passkeyCreationOptions"
	runtimePasskeyCredentialID    = "passkeyCredentialID"
	runtimePasskeyCredentialName  = "passkeyCredentialName"
)

// passkeyAuthExecutor implements the ExecutorInterface for passkey authentication.
type passkeyAuthExecutor struct {
	core.ExecutorInterface
	identifyingExecutorInterface
	passkeyService passkey.PasskeyServiceInterface
	authnProvider  authnprovidermgr.AuthnProviderManagerInterface
	entityProvider entityprovider.EntityProviderInterface
	logger         *log.Logger
}

var _ core.ExecutorInterface = (*passkeyAuthExecutor)(nil)
var _ identifyingExecutorInterface = (*passkeyAuthExecutor)(nil)

// newPasskeyAuthExecutor creates a new instance of PasskeyAuthExecutor.
func newPasskeyAuthExecutor(
	flowFactory core.FlowFactoryInterface,
	passkeyService passkey.PasskeyServiceInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
	entityProvider entityprovider.EntityProviderInterface,
) *passkeyAuthExecutor {
	defaultInputs := []common.Input{
		{
			Identifier: inputCredentialID,
			Type:       "string",
			Required:   true,
		},
		{
			Identifier: inputClientDataJSON,
			Type:       "string",
			Required:   true,
		},
		{
			Identifier: inputAuthenticatorData,
			Type:       "string",
			Required:   true,
		},
		{
			Identifier: inputSignature,
			Type:       "string",
			Required:   true,
		},
		{
			Identifier: inputUserHandle,
			Type:       "string",
			Required:   false,
		},
	}

	prerequisites := []common.Input{
		{
			Identifier: userAttributeUserID,
			Type:       "string",
			Required:   false, // Optional to support usernameless flow
		},
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "PasskeyAuthExecutor"),
		log.String(log.LoggerKeyExecutorName, ExecutorNamePasskeyAuth))

	identifyExec := newIdentifyingExecutor(ExecutorNamePasskeyAuth, defaultInputs, prerequisites,
		flowFactory, entityProvider)
	base := flowFactory.CreateExecutor(ExecutorNamePasskeyAuth, common.ExecutorTypeAuthentication,
		defaultInputs, prerequisites)

	return &passkeyAuthExecutor{
		ExecutorInterface:            base,
		identifyingExecutorInterface: identifyExec,
		passkeyService:               passkeyService,
		authnProvider:                authnProvider,
		entityProvider:               entityProvider,
		logger:                       logger,
	}
}

// Execute executes the passkey authentication logic.
func (p *passkeyAuthExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Executing passkey authentication executor")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	if !p.ValidatePrerequisites(ctx, execResp) {
		logger.Debug("Prerequisites not met for passkey authentication executor")
		return execResp, nil
	}

	// Determine the executor mode
	switch ctx.ExecutorMode {
	case passkeyExecutorModeChallenge:
		return p.executeChallenge(ctx, execResp)
	case passkeyExecutorModeVerify:
		return p.executeVerify(ctx, execResp)
	case passkeyExecutorModeRegStart:
		return p.executeRegisterStart(ctx, execResp)
	case passkeyExecutorModeRegFinish:
		return p.executeRegisterFinish(ctx, execResp)
	default:
		return execResp, fmt.Errorf("invalid executor mode: %s", ctx.ExecutorMode)
	}
}

// executeChallenge generates and returns a passkey authentication challenge.
func (p *passkeyAuthExecutor) executeChallenge(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) (*common.ExecutorResponse, error) {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	// Get userID from context (may be empty for usernameless flow)
	userID := p.GetUserIDFromContext(ctx)

	if userID == "" {
		logger.Debug("Generating usernameless passkey authentication challenge")
	} else {
		logger.Debug("Generating passkey authentication challenge", log.MaskedString(log.LoggerKeyUserID, userID))
	}

	// Get relying party ID from node properties or use a default
	relyingPartyID := p.getRelyingPartyID(ctx)
	if relyingPartyID == "" {
		logger.Error("Relying party ID not configured")
		return execResp, errors.New("relying party ID is not configured in node properties")
	}

	// Start passkey authentication (service will detect usernameless flow if userID is empty)
	startReq := &passkey.PasskeyAuthenticationStartRequest{
		UserID:         userID, // May be empty for usernameless flow
		RelyingPartyID: relyingPartyID,
	}
	startData, svcErr := p.passkeyService.StartAuthentication(ctx.Context, startReq)
	if svcErr != nil {
		if svcErr.Type == serviceerror.ClientErrorType {
			logger.Debug("Failed to start passkey authentication",
				log.MaskedString(log.LoggerKeyUserID, userID),
				log.String("error", svcErr.ErrorDescription.DefaultValue))
			execResp.Status = common.ExecFailure
			execResp.FailureReason = svcErr.ErrorDescription.DefaultValue
			return execResp, nil
		}
		logger.Error("Failed to start passkey authentication",
			log.MaskedString(log.LoggerKeyUserID, userID), log.Error(errors.New(svcErr.ErrorDescription.DefaultValue)))
		return execResp, fmt.Errorf("failed to start passkey authentication: %s", svcErr.ErrorDescription.DefaultValue)
	}

	// Store session token in runtime data for verification phase
	execResp.RuntimeData[runtimePasskeySessionToken] = startData.SessionToken

	// Marshal the challenge options to JSON
	challengeJSON, err := json.Marshal(startData.PublicKeyCredentialRequestOptions)
	if err != nil {
		logger.Error("Failed to marshal challenge options", log.Error(err))
		return execResp, fmt.Errorf("failed to marshal challenge options: %w", err)
	}

	// Return challenge data to client
	execResp.AdditionalData[runtimePasskeyChallenge] = string(challengeJSON)
	execResp.Status = common.ExecComplete

	if userID == "" {
		logger.Debug("Usernameless passkey challenge generated successfully")
	} else {
		logger.Debug("Passkey challenge generated successfully", log.MaskedString(log.LoggerKeyUserID, userID))
	}
	return execResp, nil
}

// executeVerify verifies the passkey authentication response.
func (p *passkeyAuthExecutor) executeVerify(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) (*common.ExecutorResponse, error) {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Verifying passkey authentication response")

	// Check for required inputs
	if !p.HasRequiredInputs(ctx, execResp) {
		logger.Debug("Required inputs for passkey verification are not provided")
		execResp.Status = common.ExecUserInputRequired
		return execResp, nil
	}

	// Validate the passkey
	err := p.validatePasskey(ctx, execResp, logger)
	if err != nil {
		logger.Error("Error validating passkey", log.Error(err))
		return execResp, fmt.Errorf("error validating passkey: %w", err)
	}
	if execResp.Status == common.ExecFailure || execResp.Status == common.ExecUserInputRequired {
		return execResp, nil
	}

	// Get authenticated user details
	authenticatedUser, err := p.getAuthenticatedUser(ctx, execResp)
	if err != nil {
		logger.Error("Failed to get authenticated user details", log.Error(err))
		return execResp, fmt.Errorf("failed to get authenticated user details: %w", err)
	}

	execResp.AuthenticatedUser = *authenticatedUser
	execResp.Status = common.ExecComplete

	logger.Debug("Passkey verification completed successfully",
		log.String("status", string(execResp.Status)),
		log.Bool("isAuthenticated", execResp.AuthenticatedUser.IsAuthenticated))

	return execResp, nil
}

// validatePasskey validates the passkey authentication response.
func (p *passkeyAuthExecutor) validatePasskey(ctx *core.NodeContext, execResp *common.ExecutorResponse,
	logger *log.Logger) error {
	userID := p.GetUserIDFromContext(ctx)

	// Extract passkey response data from user inputs
	credentialID := ctx.UserInputs[inputCredentialID]
	clientDataJSON := ctx.UserInputs[inputClientDataJSON]
	authenticatorData := ctx.UserInputs[inputAuthenticatorData]
	signature := ctx.UserInputs[inputSignature]
	userHandle := ctx.UserInputs[inputUserHandle]

	logger.Debug("Validating passkey", log.MaskedString(log.LoggerKeyUserID, userID))

	// Get session token from runtime data
	sessionToken := ctx.RuntimeData[runtimePasskeySessionToken]
	if sessionToken == "" {
		logger.Error("No session token found for passkey authentication", log.MaskedString(log.LoggerKeyUserID, userID))
		return fmt.Errorf("no session token found for passkey authentication")
	}

	passkeyCredential := &passkey.PasskeyAuthenticationFinishRequest{
		CredentialID:      credentialID,
		CredentialType:    "public-key",
		ClientDataJSON:    clientDataJSON,
		AuthenticatorData: authenticatorData,
		Signature:         signature,
		UserHandle:        userHandle,
		SessionToken:      sessionToken,
	}
	credentials := map[string]interface{}{"passkey": passkeyCredential}
	newAuthUser, authResp, svcErr := p.authnProvider.AuthenticateUser(
		ctx.Context, nil, credentials, nil, nil, ctx.AuthUser)
	if svcErr != nil {
		if svcErr.Type == serviceerror.ClientErrorType {
			logger.Debug("Passkey verification failed", log.MaskedString(log.LoggerKeyUserID, userID),
				log.String("error", svcErr.ErrorDescription.DefaultValue))
			// Return USER_INPUT_REQUIRED to allow retry on invalid passkey
			execResp.Status = common.ExecUserInputRequired
			execResp.Inputs = p.GetRequiredInputs(ctx)
			execResp.FailureReason = errorInvalidPasskey
			return nil
		}
		logger.Error("Failed to verify passkey", log.MaskedString(log.LoggerKeyUserID, userID),
			log.String("error", svcErr.ErrorDescription.DefaultValue))
		return fmt.Errorf("failed to verify passkey: %s", svcErr.ErrorDescription.DefaultValue)
	}
	execResp.AuthUser = newAuthUser

	// Store authenticated user ID in runtime data
	if authResp.UserID != "" {
		execResp.RuntimeData[userAttributeUserID] = authResp.UserID
	}

	// Clear session token after successful verification
	execResp.RuntimeData[runtimePasskeySessionToken] = ""

	logger.Debug("Passkey validated successfully", log.MaskedString(log.LoggerKeyUserID, userID))
	return nil
}

// getAuthenticatedUser returns the authenticated user details.
func (p *passkeyAuthExecutor) getAuthenticatedUser(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) (*authncm.AuthenticatedUser, error) {
	userID := execResp.RuntimeData[userAttributeUserID]
	if userID == "" {
		userID = p.GetUserIDFromContext(ctx)
	}
	if userID == "" {
		return nil, errors.New("user ID is empty after passkey authentication")
	}

	// Get user details from user provider
	user, providerErr := p.entityProvider.GetEntity(userID)
	if providerErr != nil {
		return nil, fmt.Errorf("failed to get user details: %s", providerErr.Error())
	}

	// Extract user attributes
	attrs := make(map[string]interface{})
	if len(user.Attributes) > 0 {
		if err := json.Unmarshal(user.Attributes, &attrs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal user attributes: %w", err)
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

// executeRegisterStart initiates passkey credential registration.
func (p *passkeyAuthExecutor) executeRegisterStart(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) (*common.ExecutorResponse, error) {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Starting passkey registration")

	userID := p.GetUserIDFromContext(ctx)
	if userID == "" {
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "User ID is required for passkey registration"
		return execResp, nil
	}

	relyingPartyID := p.getRelyingPartyID(ctx)
	if relyingPartyID == "" {
		logger.Error("Relying party ID not configured")
		return execResp, errors.New("relying party ID is not configured in node properties")
	}

	relyingPartyName := p.getRelyingPartyName(ctx)
	if relyingPartyName == "" {
		relyingPartyName = relyingPartyID // Default to ID if name not specified
	}

	// Build registration request
	regReq := &passkey.PasskeyRegistrationStartRequest{
		UserID:           userID,
		RelyingPartyID:   relyingPartyID,
		RelyingPartyName: relyingPartyName,
		// Optional: Get authenticator selection and attestation from node properties
		AuthenticatorSelection: p.getAuthenticatorSelection(ctx),
		Attestation:            p.getAttestation(ctx),
	}

	// Start passkey registration
	startData, svcErr := p.passkeyService.StartRegistration(ctx.Context, regReq)
	if svcErr != nil {
		if svcErr.Type == serviceerror.ClientErrorType {
			logger.Debug("Failed to start passkey registration",
				log.MaskedString(log.LoggerKeyUserID, userID),
				log.String("error", svcErr.ErrorDescription.DefaultValue))
			execResp.Status = common.ExecFailure
			execResp.FailureReason = svcErr.ErrorDescription.DefaultValue
			return execResp, nil
		}
		logger.Error("Failed to start passkey registration",
			log.MaskedString(log.LoggerKeyUserID, userID), log.Error(errors.New(svcErr.ErrorDescription.DefaultValue)))
		return execResp, fmt.Errorf("failed to start passkey registration: %s", svcErr.ErrorDescription.DefaultValue)
	}

	// Store session token in runtime data for finish phase
	execResp.RuntimeData[runtimePasskeySessionToken] = startData.SessionToken

	// Marshal the creation options to JSON
	creationJSON, err := json.Marshal(startData.PublicKeyCredentialCreationOptions)
	if err != nil {
		logger.Error("Failed to marshal creation options", log.Error(err))
		return execResp, fmt.Errorf("failed to marshal creation options: %w", err)
	}

	// Return creation options to client
	execResp.AdditionalData[runtimePasskeyCreationOptions] = string(creationJSON)
	execResp.Status = common.ExecComplete

	logger.Debug("Passkey registration options generated successfully", log.MaskedString(log.LoggerKeyUserID, userID))
	return execResp, nil
}

// executeRegisterFinish completes passkey credential registration.
func (p *passkeyAuthExecutor) executeRegisterFinish(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) (*common.ExecutorResponse, error) {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Finishing passkey registration")

	// Check for required inputs
	allInputs := []common.Input{
		{Identifier: inputCredentialID, Required: true},
		{Identifier: inputClientDataJSON, Required: true},
		{Identifier: inputAttestationObject, Required: true},
		{Identifier: inputCredentialName, Required: false}, // Optional: user-friendly name for the passkey
	}

	// Validate inputs - only block on missing REQUIRED inputs
	missingRequiredInputs := false
	for _, input := range allInputs {
		if _, ok := ctx.UserInputs[input.Identifier]; !ok {
			execResp.Inputs = append(execResp.Inputs, input)
			if input.Required {
				missingRequiredInputs = true
			}
		}
	}

	if missingRequiredInputs {
		logger.Debug("Required inputs for passkey registration are not provided")
		execResp.Status = common.ExecUserInputRequired
		execResp.Inputs = allInputs
		return execResp, nil
	}

	// Extract registration response data from user inputs
	credentialID := ctx.UserInputs[inputCredentialID]
	clientDataJSON := ctx.UserInputs[inputClientDataJSON]
	attestationObject := ctx.UserInputs[inputAttestationObject]
	credentialName := ctx.UserInputs[inputCredentialName]
	if credentialName == "" {
		credentialName = "Passkey" // Default name if not provided
	}

	// Get session token from runtime data
	sessionToken := ctx.RuntimeData[runtimePasskeySessionToken]
	if sessionToken == "" {
		logger.Error("No session token found for passkey registration")
		return execResp, fmt.Errorf("no session token found for passkey registration")
	}

	// Build finish registration request
	finishReq := &passkey.PasskeyRegistrationFinishRequest{
		CredentialID:      credentialID,
		CredentialType:    "public-key",
		ClientDataJSON:    clientDataJSON,
		AttestationObject: attestationObject,
		SessionToken:      sessionToken,
		CredentialName:    credentialName,
	}

	// Call passkey service to finish registration
	finishData, svcErr := p.passkeyService.FinishRegistration(ctx.Context, finishReq)
	if svcErr != nil {
		if svcErr.Type == serviceerror.ClientErrorType {
			logger.Debug("Passkey registration failed", log.String("error", svcErr.ErrorDescription.DefaultValue))
			// Return USER_INPUT_REQUIRED to allow retry on invalid registration
			execResp.Status = common.ExecUserInputRequired
			execResp.Inputs = allInputs
			execResp.FailureReason = svcErr.ErrorDescription.DefaultValue
			return execResp, nil
		}
		logger.Error("Failed to finish passkey registration", log.String("error", svcErr.ErrorDescription.DefaultValue))
		return execResp, fmt.Errorf("failed to finish passkey registration: %s", svcErr.ErrorDescription.DefaultValue)
	}

	// Store credential info in runtime data
	execResp.RuntimeData[runtimePasskeyCredentialID] = finishData.CredentialID
	execResp.RuntimeData[runtimePasskeyCredentialName] = finishData.CredentialName

	// Clear session token after successful registration
	execResp.RuntimeData[runtimePasskeySessionToken] = ""

	// For registration flows, return the credential info in additional data
	execResp.AdditionalData[runtimePasskeyCredentialID] = finishData.CredentialID
	execResp.AdditionalData[runtimePasskeyCredentialName] = finishData.CredentialName
	execResp.AdditionalData["credentialCreatedAt"] = finishData.CreatedAt

	// Handle flow completion based on flow type
	if ctx.FlowType == common.FlowTypeRegistration {
		// For registration flows, user may not be fully authenticated yet
		// Return credential info but don't set authenticated user
		execResp.Status = common.ExecComplete
		logger.Debug("Passkey registration completed for registration flow")
	} else {
		// For authentication flows (adding passkey to existing account)
		// Get and return authenticated user details
		authenticatedUser, err := p.getAuthenticatedUser(ctx, execResp)
		if err != nil {
			logger.Error("Failed to get authenticated user details", log.Error(err))
			return execResp, fmt.Errorf("failed to get authenticated user details: %w", err)
		}
		execResp.AuthenticatedUser = *authenticatedUser
		execResp.Status = common.ExecComplete
		logger.Debug("Passkey registration completed for existing user")
	}

	logger.Debug("Passkey registration finished successfully",
		log.String("credentialID", finishData.CredentialID))
	return execResp, nil
}

// getRelyingPartyID retrieves the relying party ID from node properties.
func (p *passkeyAuthExecutor) getRelyingPartyID(ctx *core.NodeContext) string {
	if len(ctx.NodeProperties) == 0 {
		return ""
	}

	if rpID, ok := ctx.NodeProperties["relyingPartyId"]; ok {
		if rpIDStr, valid := rpID.(string); valid && rpIDStr != "" {
			return rpIDStr
		}
	}

	return ""
}

// getRelyingPartyName retrieves the relying party name from node properties.
func (p *passkeyAuthExecutor) getRelyingPartyName(ctx *core.NodeContext) string {
	if len(ctx.NodeProperties) == 0 {
		return ""
	}

	if rpName, ok := ctx.NodeProperties["relyingPartyName"]; ok {
		if rpNameStr, valid := rpName.(string); valid && rpNameStr != "" {
			return rpNameStr
		}
	}

	return ""
}

// getAuthenticatorSelection retrieves authenticator selection criteria from node properties.
func (p *passkeyAuthExecutor) getAuthenticatorSelection(ctx *core.NodeContext) *passkey.AuthenticatorSelection {
	if len(ctx.NodeProperties) == 0 {
		return nil
	}
	if authSel, ok := ctx.NodeProperties["authenticatorSelection"]; ok {
		if authSelMap, valid := authSel.(map[string]interface{}); valid {
			selection := &passkey.AuthenticatorSelection{}
			if authAttachment, ok := authSelMap["authenticatorAttachment"].(string); ok {
				selection.AuthenticatorAttachment = authAttachment
			}
			if reqResidentKey, ok := authSelMap["requireResidentKey"].(bool); ok {
				selection.RequireResidentKey = reqResidentKey
			}
			if residentKey, ok := authSelMap["residentKey"].(string); ok {
				selection.ResidentKey = residentKey
			}
			if userVerification, ok := authSelMap["userVerification"].(string); ok {
				selection.UserVerification = userVerification
			}
			return selection
		}
	}
	return nil
}

// getAttestation retrieves attestation conveyance preference from node properties.
func (p *passkeyAuthExecutor) getAttestation(ctx *core.NodeContext) string {
	if len(ctx.NodeProperties) == 0 {
		return "none" // Default to "none"
	}

	if attestation, ok := ctx.NodeProperties["attestation"]; ok {
		if attestationStr, valid := attestation.(string); valid && attestationStr != "" {
			return attestationStr
		}
	}

	return "none"
}
