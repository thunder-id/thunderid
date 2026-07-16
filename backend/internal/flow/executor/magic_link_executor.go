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

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/authn/magiclink"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// magicLinkExecutor implements the ExecutorInterface for Magic Link authentication.
type magicLinkExecutor struct {
	providers.Executor
	identifyingExecutorInterface
	entityProvider   entityprovider.EntityProviderInterface
	magicLinkService magiclink.MagicLinkAuthnServiceInterface
	authnProvider    providers.AuthnProviderManager
	logger           *log.Logger
}

var _ providers.Executor = (*magicLinkExecutor)(nil)
var _ identifyingExecutorInterface = (*magicLinkExecutor)(nil)

// newMagicLinkExecutorResponse creates a new instance of ExecutorResponse for Magic Link authentication.
func newMagicLinkExecutorResponse() *providers.ExecutorResponse {
	return &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		ForwardedData:  make(map[string]interface{}),
	}
}

// newMagicLinkExecutor creates a new instance of MagicLinkExecutor.
func newMagicLinkExecutor(
	flowFactory core.FlowFactoryInterface,
	magicLinkService magiclink.MagicLinkAuthnServiceInterface,
	authnProvider providers.AuthnProviderManager,
	entityProvider entityprovider.EntityProviderInterface,
) *magicLinkExecutor {
	defaultInputs := []providers.Input{{
		Ref:        "magic_link_token_input",
		Identifier: userInputMagicLinkToken,
		Type:       providers.InputTypeHidden,
		Required:   true,
	}}
	var prerequisites []providers.Input

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "MagicLinkExecutor"),
		log.String(log.LoggerKeyExecutorName, ExecutorNameMagicLink))

	identifyExec := newIdentifyingExecutor(ExecutorNameMagicLink, defaultInputs, prerequisites,
		flowFactory, entityProvider)
	base := flowFactory.CreateExecutor(ExecutorNameMagicLink, providers.ExecutorTypeAuthentication,
		defaultInputs, prerequisites, &providers.ExecutorMeta{
			SupportedModes: []string{ExecutorModeGenerate, ExecutorModeVerify},
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: propertyKeyTokenExpiry},
				{Property: propertyKeyMagicLinkURL},
			},
		})

	return &magicLinkExecutor{
		Executor:                     base,
		identifyingExecutorInterface: identifyExec,
		entityProvider:               entityProvider,
		magicLinkService:             magicLinkService,
		authnProvider:                authnProvider,
		logger:                       logger,
	}
}

// Execute executes the Magic Link authentication logic.
func (m *magicLinkExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := m.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing Magic Link authentication executor")

	execResp := newMagicLinkExecutorResponse()
	execResp.AuthUser = ctx.AuthUser

	if !m.ValidatePrerequisites(ctx, execResp, m.authnProvider) {
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
func (m *magicLinkExecutor) GetExecutionPolicy(mode string) *providers.ExecutionPolicy {
	if mode == ExecutorModeVerify {
		return &providers.ExecutionPolicy{
			SkipChallengeValidation: true,
			AllowSegmentRestart:     false,
		}
	}
	return nil
}

// executeGenerate handles the generation of the magic link
func (m *magicLinkExecutor) executeGenerate(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
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
func (m *magicLinkExecutor) InitiateMagicLink(ctx *providers.NodeContext,
	logger *log.Logger) (*providers.ExecutorResponse, error) {
	execResp := newMagicLinkExecutorResponse()
	execResp.AuthUser = ctx.AuthUser
	isRegistration := ctx.FlowType == providers.FlowTypeRegistration
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
			execResp.Status = providers.ExecComplete
			return execResp, nil
		}
		subject = destValue
	} else {
		var userID string
		if ctx.AuthUser.IsAuthenticated() {
			userID = m.GetUserIDFromContext(ctx, execResp, m.authnProvider)
			if userID == "" {
				return execResp, errors.New("user ID is empty in the context")
			}
		} else {
			identifiedUserID, providerErr := m.entityProvider.IdentifyEntity(searchAttrs)
			if providerErr != nil || identifiedUserID == nil || *identifiedUserID == "" {
				logger.Debug(ctx.Context, "User not found, completing without delivery for anti-enumeration")
				execResp.RuntimeData[common.RuntimeKeySkipDelivery] = dataValueTrue
				execResp.Status = providers.ExecComplete
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
		if svcErr.Type == tidcommon.ClientErrorType {
			execResp.Status = providers.ExecFailure
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

	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// getTokenExpiry returns the magic link token expiry in seconds from node properties,
// falling back to the default if not configured or invalid.
func (m *magicLinkExecutor) getTokenExpiry(ctx *providers.NodeContext) int64 {
	if ctx.NodeProperties != nil {
		if val, ok := ctx.NodeProperties[propertyKeyTokenExpiry]; ok {
			if str := utils.ConvertInterfaceValueToString(val); str != "" {
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
func (m *magicLinkExecutor) getMagicLinkURL(ctx *providers.NodeContext) string {
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
func (m *magicLinkExecutor) buildUserSearchAttributes(ctx *providers.NodeContext) map[string]interface{} {
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

// executeVerify handles the verification of the magic link token
func (m *magicLinkExecutor) executeVerify(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := m.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	execResp := newMagicLinkExecutorResponse()
	execResp.AuthUser = ctx.AuthUser

	if !m.HasRequiredInputs(ctx, execResp) {
		logger.Debug(ctx.Context, "Required inputs for Magic Link verification are not provided")
		execResp.Status = providers.ExecUserInputRequired
		return execResp, nil
	}

	token := ctx.UserInputs[userInputMagicLinkToken]

	subjectAttribute := ""
	if ctx.FlowType == providers.FlowTypeRegistration {
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

	newAuthUser, authenticatedClaims, svcErr := m.authnProvider.AuthenticateUser(
		ctx.Context, nil, creds, nil, nil, execResp.AuthUser)
	execResp.AuthUser = newAuthUser
	if svcErr != nil {
		if svcErr.Code == authnprovidermgr.ErrorAuthenticationFailed.Code {
			execResp.Status = providers.ExecFailure
			execResp.Error = svcErr
			return execResp, nil
		}
		return execResp, fmt.Errorf("failed to verify magic link: %s", svcErr.ErrorDescription.DefaultValue)
	}
	for key, value := range authenticatedClaims {
		execResp.RuntimeData[key] = utils.ConvertInterfaceValueToString(value)
	}

	tokenJTI, execErr := m.validateFlowClaims(ctx, token, logger)
	if execErr != nil {
		execResp.Status = providers.ExecFailure
		execResp.Error = execErr
		return execResp, nil
	}
	execResp.RuntimeData[common.RuntimeKeyMagicLinkUsedJti] = tokenJTI

	execResp.Status = providers.ExecComplete
	logger.Debug(ctx.Context, "Magic link verify completed successfully")
	return execResp, nil
}

// validateFlowClaims checks executionId and JTI claims in the magic link JWT token.
// These are flow-specific concerns and not part of the auth provider contract.
func (m *magicLinkExecutor) validateFlowClaims(ctx *providers.NodeContext,
	token string, logger *log.Logger) (string, *tidcommon.ServiceError) {
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
func (m *magicLinkExecutor) resolveDestinationAttribute(ctx *providers.NodeContext) string {
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
