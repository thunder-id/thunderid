/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
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
	"slices"
	"strconv"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/magiclink"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// magicLinkExecutor implements the ExecutorInterface for Magic Link authentication.
type magicLinkExecutor struct {
	core.ExecutorInterface
	identifyingExecutorInterface
	entityProvider   entityprovider.EntityProviderInterface
	magicLinkService magiclink.MagicLinkAuthnServiceInterface
	authnProvider    authnprovidermgr.AuthnProviderManagerInterface
	logger           *log.Logger
}

var _ core.ExecutorInterface = (*magicLinkExecutor)(nil)
var _ identifyingExecutorInterface = (*magicLinkExecutor)(nil)

// newMagicLinkExecutorResponse creates a new instance of ExecutorResponse for Magic Link authentication.
func newMagicLinkExecutorResponse() *common.ExecutorResponse {
	return &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		ForwardedData:  make(map[string]interface{}),
	}
}

// newMagicLinkExecutor creates a new instance of MagicLinkExecutor.
func newMagicLinkExecutor(
	flowFactory core.FlowFactoryInterface,
	magicLinkService magiclink.MagicLinkAuthnServiceInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
	entityProvider entityprovider.EntityProviderInterface,
) *magicLinkExecutor {
	defaultInputs := []common.Input{{
		Ref:        "magic_link_token_input",
		Identifier: userInputMagicLinkToken,
		Type:       common.InputTypeHidden,
		Required:   true,
	}}
	var prerequisites []common.Input

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "MagicLinkExecutor"),
		log.String(log.LoggerKeyExecutorName, ExecutorNameMagicLink))

	identifyExec := newIdentifyingExecutor(ExecutorNameMagicLink, defaultInputs, prerequisites,
		flowFactory, entityProvider)
	base := flowFactory.CreateExecutor(ExecutorNameMagicLink, common.ExecutorTypeAuthentication,
		defaultInputs, prerequisites)

	return &magicLinkExecutor{
		ExecutorInterface:            base,
		identifyingExecutorInterface: identifyExec,
		entityProvider:               entityProvider,
		magicLinkService:             magicLinkService,
		authnProvider:                authnProvider,
		logger:                       logger,
	}
}

// Execute executes the Magic Link authentication logic.
func (m *magicLinkExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := m.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing Magic Link authentication executor")

	execResp := newMagicLinkExecutorResponse()

	if !m.ValidatePrerequisites(ctx, execResp) {
		logger.Debug(ctx.Context, "Prerequisites not met for Magic Link authentication executor")
		return execResp, nil
	}

	switch ctx.ExecutorMode {
	case ExecutorModeGenerate:
		return m.executeGenerate(ctx)
	case ExecutorModeVerify:
		return m.executeVerify(ctx)
	default:
		return execResp, fmt.Errorf("invalid executor mode: %s", ctx.ExecutorMode)
	}
}

// GetExecutionPolicy returns the execution policy for the given mode.
// The verify mode skips challenge token validation because the magicLink token itself serves as the challenge.
func (m *magicLinkExecutor) GetExecutionPolicy(mode string) *core.ExecutionPolicy {
	if mode == ExecutorModeVerify {
		return &core.ExecutionPolicy{
			SkipChallengeValidation: true,
			AllowSegmentRestart:     false,
		}
	}
	return nil
}

// executeGenerate handles the generation of the magic link
func (m *magicLinkExecutor) executeGenerate(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := m.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	execResp, err := m.InitiateMagicLink(ctx, logger)
	if err != nil {
		return execResp, err
	}
	logger.Debug(ctx.Context, "Magic link generation completed",
		log.String("status", string(execResp.Status)))
	return execResp, nil
}

// InitiateMagicLink performs the core logic for generating a magic link
func (m *magicLinkExecutor) InitiateMagicLink(ctx *core.NodeContext,
	logger *log.Logger) (*common.ExecutorResponse, error) {
	execResp := newMagicLinkExecutorResponse()
	isRegistration := ctx.FlowType == common.FlowTypeRegistration
	searchAttrs := m.buildUserSearchAttributes(ctx)

	// 1. Resolve the destination attribute name
	destAttr := m.resolveDestinationAttribute(ctx)

	// 2. Look for the destination value using the attribute name
	destValue := utils.ConvertInterfaceValueToString(searchAttrs[destAttr])
	var subject string

	if isRegistration {
		execResp.RuntimeData[common.RuntimeKeyMagicLinkDestinationAttribute] = destAttr
		if destValue == "" {
			return execResp, fmt.Errorf("%s is required for magic link registration", destAttr)
		}
		userID, identifyErr := m.IdentifyUser(ctx.Context, searchAttrs, execResp)
		if identifyErr != nil {
			return execResp, fmt.Errorf("failed to identify user: %w", identifyErr)
		}
		if userID != nil && *userID != "" {
			// ANTI-ENUMERATION: Pretend it succeeded but skip sending the magic link.
			logger.Debug(ctx.Context,
				"Registration attempted for existing user. Skipping delivery to prevent enumeration.")
			execResp.RuntimeData[common.RuntimeKeySkipDelivery] = dataValueTrue
			execResp.Status = common.ExecComplete
			return execResp, nil
		}
		subject = destValue
	} else {
		var userID string
		if ctx.AuthenticatedUser.IsAuthenticated {
			userID = m.GetUserIDFromContext(ctx)
			if userID == "" {
				return execResp, errors.New("user ID is empty in the context")
			}
		} else {
			identifiedUserID, providerErr := m.entityProvider.IdentifyEntity(searchAttrs)
			if providerErr != nil || identifiedUserID == nil || *identifiedUserID == "" {
				logger.Debug(ctx.Context, "User not found, completing without delivery for anti-enumeration")
				execResp.RuntimeData[common.RuntimeKeySkipDelivery] = dataValueTrue
				execResp.Status = common.ExecComplete
				return execResp, nil
			}
			userID = *identifiedUserID
		}
		execResp.RuntimeData[userAttributeUserID] = userID
		subject = userID
	}

	claims := map[string]interface{}{"executionId": ctx.ExecutionID}

	expirySeconds := m.getTokenExpiry(ctx)
	magicLinkURL := m.getMagicLinkURL(ctx)

	queryParams := map[string]string{
		"id":            ctx.ExecutionID,
		"applicationId": ctx.Application.ID,
		"type":          string(ctx.FlowType),
	}

	generatedURL, svcErr := m.magicLinkService.GenerateMagicLink(
		ctx.Context, subject, expirySeconds, queryParams, claims, magicLinkURL)

	if svcErr != nil {
		if svcErr.Type == serviceerror.ClientErrorType {
			execResp.Status = common.ExecFailure
			execResp.Error = &ErrMagicLinkGeneration
			return execResp, nil
		}
		return execResp, errors.New("failed to generate magic link")
	}

	if destValue != "" {
		execResp.RuntimeData[destAttr] = destValue
	}

	execResp.ForwardedData[common.ForwardedDataKeyTemplateData] = map[string]interface{}{
		"magicLink":     generatedURL,
		"expiryMinutes": utils.SecondsToMinutes(expirySeconds),
		"appName":       ctx.Application.Name,
	}

	execResp.Status = common.ExecComplete
	return execResp, nil
}

// getTokenExpiry returns the magic link token expiry in seconds from node properties,
// falling back to the default if not configured or invalid.
func (m *magicLinkExecutor) getTokenExpiry(ctx *core.NodeContext) int64 {
	if ctx.NodeProperties != nil {
		if val, ok := ctx.NodeProperties[propertyKeyTokenExpiry]; ok {
			if str, valid := val.(string); valid && str != "" {
				if parsed, err := strconv.ParseInt(str, 10, 64); err == nil && parsed > 0 {
					return parsed
				}
			}
		}
	}

	return int64(magiclink.DefaultExpirySeconds)
}

// getMagicLinkURL returns the magic link URL prefix from node properties,
// returning nil if not configured.
func (m *magicLinkExecutor) getMagicLinkURL(ctx *core.NodeContext) string {
	if ctx.NodeProperties != nil {
		if val, ok := ctx.NodeProperties[propertyKeyMagicLinkURL]; ok {
			if str, valid := val.(string); valid && str != "" {
				return str
			}
		}
	}
	return ""
}

// buildUserSearchAttributes collects search attributes from node inputs,
// looking in user inputs, runtime data, and forwarded data.
func (m *magicLinkExecutor) buildUserSearchAttributes(ctx *core.NodeContext) map[string]interface{} {
	attrs := make(map[string]interface{})
	identifiers := make(map[string]struct{})

	for _, input := range ctx.NodeInputs {
		if isSearchableIdentifier(input.Identifier) {
			identifiers[input.Identifier] = struct{}{}
		}
	}

	if len(identifiers) == 0 {
		for key, value := range ctx.UserInputs {
			if value != "" && isSearchableIdentifier(key) {
				identifiers[key] = struct{}{}
			}
		}
	}

	for identifier := range identifiers {
		if value, ok := ctx.UserInputs[identifier]; ok && value != "" {
			attrs[identifier] = value
		}
	}

	return attrs
}

// isSearchableIdentifier checks if an identifier is searchable.
func isSearchableIdentifier(identifier string) bool {
	if identifier == "" {
		return false
	}

	if slices.Contains(nonSearchableInputs, identifier) {
		return false
	}

	return true
}

// getAuthenticatedUser retrieves the authenticated user details from the user provider.
func (m *magicLinkExecutor) getAuthenticatedUser(
	userID string) (*authncm.AuthenticatedUser, error) {
	if userID == "" {
		return nil, errors.New("user ID is empty")
	}

	user, err := m.entityProvider.GetEntity(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &authncm.AuthenticatedUser{
		IsAuthenticated: true,
		UserID:          user.ID,
		UserType:        user.Type,
		OUID:            user.OUID,
	}, nil
}

// executeVerify handles the verification of the magic link token
func (m *magicLinkExecutor) executeVerify(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := m.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	execResp := newMagicLinkExecutorResponse()

	if !m.HasRequiredInputs(ctx, execResp) {
		logger.Debug(ctx.Context, "Required inputs for Magic Link verification are not provided")
		execResp.Status = common.ExecUserInputRequired
		return execResp, nil
	}

	token := ctx.UserInputs[userInputMagicLinkToken]

	subjectAttribute := ""
	if ctx.FlowType == common.FlowTypeRegistration {
		subjectAttribute = ctx.RuntimeData[common.RuntimeKeyMagicLinkDestinationAttribute]
		if subjectAttribute == "" {
			return execResp, errors.New("magic link destination attribute missing from runtime data")
		}
	}

	creds := map[string]interface{}{
		"magiclink": map[string]interface{}{
			"token":            token,
			"subjectAttribute": subjectAttribute,
		},
	}

	newAuthUser, authnResult, svcErr := m.authnProvider.AuthenticateUser(
		ctx.Context, nil, creds, nil, nil, ctx.AuthUser)
	if svcErr != nil {
		if svcErr.Code == authnprovidermgr.ErrorAuthenticationFailed.Code {
			execResp.Status = common.ExecFailure
			execResp.Error = svcErr
			return execResp, nil
		}
		return execResp, fmt.Errorf("failed to verify magic link: %s", svcErr.ErrorDescription.DefaultValue)
	}
	execResp.AuthUser = newAuthUser

	tokenJTI, execErr := m.validateFlowClaims(ctx, token, logger)
	if execErr != nil {
		execResp.Status = common.ExecFailure
		execResp.Error = execErr
		return execResp, nil
	}
	execResp.RuntimeData[common.RuntimeKeyMagicLinkUsedJti] = tokenJTI

	if ctx.FlowType == common.FlowTypeRegistration {
		if authnResult.IsExistingUser {
			logger.Debug(ctx.Context, "User already exists during magic link registration verification.")
			execResp.Status = common.ExecFailure
			execResp.Error = &ErrUserAlreadyExists
			return execResp, nil
		}
		execResp.Status = common.ExecComplete
		return execResp, nil
	}

	userID := authnResult.UserID
	authenticatedUser, err := m.getAuthenticatedUser(userID)
	if err != nil {
		return execResp, fmt.Errorf("failed to get authenticated user details: %w", err)
	}
	execResp.AuthenticatedUser = *authenticatedUser

	execResp.Status = common.ExecComplete
	logger.Debug(ctx.Context, "Magic link verify completed successfully")
	return execResp, nil
}

// validateFlowClaims checks executionId and JTI claims in the magic link JWT token.
// These are flow-specific concerns and not part of the auth provider contract.
func (m *magicLinkExecutor) validateFlowClaims(ctx *core.NodeContext,
	token string, logger *log.Logger) (string, *serviceerror.ServiceError) {
	payload, decodeErr := jwt.DecodeJWTPayload(token)
	if decodeErr != nil {
		logger.Debug(ctx.Context, "Failed to decode magic link token", log.Error(decodeErr))
		return "", &ErrInvalidMagicLinkToken
	}

	executionIDClaim := utils.ConvertInterfaceValueToString(payload["executionId"])
	if executionIDClaim == "" || executionIDClaim != ctx.ExecutionID {
		logger.Debug(ctx.Context, "Magic link token executionId mismatch")
		return "", &ErrInvalidMagicLinkToken
	}

	jtiClaim := utils.ConvertInterfaceValueToString(payload["jti"])
	if jtiClaim == "" {
		return "", &ErrInvalidMagicLinkToken
	}
	if usedJti, exists := ctx.RuntimeData[common.RuntimeKeyMagicLinkUsedJti]; exists && usedJti == jtiClaim {
		logger.Debug(ctx.Context, "Magic link token has already been used", log.String("jti", jtiClaim))
		return "", &ErrInvalidMagicLinkToken
	}

	logger.Debug(ctx.Context, "Magic link token validated successfully")
	return jtiClaim, nil
}

// resolveDestinationAttribute infers the destination attribute from the first configured node input.
// Falls back to "email" if none is configured or if the first input is invalid.
func (m *magicLinkExecutor) resolveDestinationAttribute(ctx *core.NodeContext) string {
	// Explicitly check ONLY the first input (index 0) to prevent multi-input ambiguity
	if len(ctx.NodeInputs) > 0 {
		firstInput := ctx.NodeInputs[0]
		if isSearchableIdentifier(firstInput.Identifier) {
			return firstInput.Identifier
		}
	}

	// Fallback to email
	return common.AttributeEmail
}
