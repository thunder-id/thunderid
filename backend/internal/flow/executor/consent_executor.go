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
	"html"
	"slices"
	"strconv"
	"strings"
	"time"

	consentauthn "github.com/thunder-id/thunderid/internal/authn/consent"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/consent"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// consentExecutor handles consent collection during identity journeys.
// It checks whether the authenticated user has the required consents for the application,
// prompts if not, and records the user's decisions after they are collected by the prompt node.
type consentExecutor struct {
	core.ExecutorInterface
	consentEnforcer consentauthn.ConsentEnforcerServiceInterface
	authnProvider   authnprovidermgr.AuthnProviderManagerInterface
	logger          *log.Logger
}

var _ core.ExecutorInterface = (*consentExecutor)(nil)

// newConsentExecutor creates a new instance of consentExecutor.
func newConsentExecutor(
	flowFactory core.FlowFactoryInterface,
	consentEnforcer consentauthn.ConsentEnforcerServiceInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
) *consentExecutor {
	logger := log.GetLogger().With(
		log.String(log.LoggerKeyComponentName, "ConsentExecutor"),
		log.String(log.LoggerKeyExecutorName, ExecutorNameConsent),
	)
	defaultInputs := []common.Input{
		{
			Identifier: userInputConsentDecisions,
			Type:       common.InputTypeConsent,
			Required:   true,
		},
	}
	prerequisites := []common.Input{
		{
			Identifier: userAttributeUserID,
			Type:       common.InputTypeText,
			Required:   true,
		},
	}

	base := flowFactory.CreateExecutor(ExecutorNameConsent, common.ExecutorTypeUtility,
		defaultInputs, prerequisites)

	return &consentExecutor{
		ExecutorInterface: base,
		consentEnforcer:   consentEnforcer,
		authnProvider:     authnProvider,
		logger:            logger,
	}
}

// Execute runs the consent enforcement logic.
func (e *consentExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing consent executor")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		ForwardedData:  make(map[string]interface{}),
		AuthUser:       ctx.AuthUser,
	}

	if !e.ValidatePrerequisites(ctx, execResp, e.authnProvider) {
		logger.Debug(ctx.Context, "Prerequisites validation failed for consent executor")
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrConsentPrereqFailed
		return execResp, nil
	}

	if !execResp.AuthUser.IsAuthenticated() {
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrUserNotAuthenticated
		return execResp, nil
	}

	authUser, entityRef, svcErr := e.authnProvider.GetEntityReference(ctx.Context, execResp.AuthUser)
	execResp.AuthUser = authUser
	if svcErr != nil {
		logger.Error(ctx.Context, "Failed to get entity reference from AuthUser", log.Any("error", svcErr))
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrFailedToIdentifyUser
		return execResp, nil
	}

	availableAttrs, svcErr := e.authnProvider.GetUserAvailableAttributes(ctx.Context, execResp.AuthUser)
	execResp.AuthUser = authUser
	if svcErr != nil {
		e.logger.Debug(ctx.Context, "Failed to get available attributes from AuthUser",
			log.Any("error", svcErr))
	}

	// TODO: Replace with application's actual OU when OU support is added
	ouID := "default"
	appID := ctx.EntityID
	entityID := entityRef.EntityID

	if !e.HasRequiredInputs(ctx, execResp) {
		logger.Debug(ctx.Context, "Required consent decisions not provided; checking if consent is needed")
		return e.checkConsent(ctx, execResp, ouID, appID, availableAttrs, entityRef)
	}

	logger.Debug(ctx.Context, "Consent decisions provided; processing consent decisions")
	return e.handleConsentDecisions(ctx, execResp, ouID, appID, entityID)
}

// checkConsent resolves whether consent is needed and either completes or forwards to a prompt.
func (e *consentExecutor) checkConsent(ctx *core.NodeContext, execResp *common.ExecutorResponse,
	ouID, appID string,
	availableAttrResp *authnprovidercm.AttributesResponse,
	entityRef *authnprovidercm.EntityReference,
) (*common.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Checking if user consent is required")

	essentialAttributes, optionalAttributes := e.getRequiredAttributes(ctx)
	authorizedPermissions := strings.Fields(ctx.RuntimeData["authorized_permissions"])
	availableAttributes := e.buildAugmentedAvailableAttributes(availableAttrResp, entityRef)
	appName := ctx.Application.Name

	// Resolve consent to determine if any required consents are missing and need to be prompted
	promptData, svcErr := e.consentEnforcer.ResolveConsent(
		ctx.Context, ouID, appID, appName, entityRef.EntityID,
		essentialAttributes, optionalAttributes, authorizedPermissions,
		availableAttributes)
	if svcErr != nil {
		if svcErr.Type == serviceerror.ClientErrorType {
			logger.Debug(ctx.Context, "Client error while resolving user consent", log.Any("error", svcErr))
			execResp.Status = common.ExecFailure
			execResp.Error = &ErrConsentResolutionFailed
			return execResp, nil
		}

		logger.Error(ctx.Context, "Failed to resolve consent", log.Any("error", svcErr))
		return nil, errors.New("failed to resolve consent")
	}

	// All consents are active — nothing to prompt
	if promptData == nil {
		logger.Debug(ctx.Context, "All required consents are active; completing consent executor")
		execResp.Status = common.ExecComplete
		return execResp, nil
	}

	// Consent is needed — forward prompt data to the prompt node via ForwardedData
	promptJSON, err := json.Marshal(promptData.Purposes)
	if err != nil {
		logger.Error(ctx.Context, "Failed to marshal consent prompt data", log.Error(err))
		return nil, errors.New("failed to prepare consent prompt data")
	}

	execResp.ForwardedData[common.ForwardedDataKeyConsentPrompt] = promptData.Purposes
	execResp.AdditionalData[common.DataConsentPrompt] = string(promptJSON)

	// Store the session token in RuntimeData for validation during consent recording
	if promptData.SessionToken != "" {
		execResp.RuntimeData[common.RuntimeKeyConsentSessionToken] = promptData.SessionToken
	}

	// Check if a timeout is configured (in seconds)
	if timeoutStr, ok := ctx.NodeProperties["timeout"].(string); ok && timeoutStr != "" {
		if timeoutSec, err := strconv.ParseInt(timeoutStr, 10, 64); err == nil && timeoutSec > 0 {
			expiresAt := time.Now().Add(time.Duration(timeoutSec) * time.Second).UnixMilli()
			expiresAtStr := strconv.FormatInt(expiresAt, 10)
			logger.Debug(ctx.Context, "Consent timeout configured", log.String("expiresAt", expiresAtStr))

			execResp.AdditionalData[common.DataStepTimeout] = expiresAtStr
			execResp.RuntimeData[common.RuntimeKeyStepTimeout] = expiresAtStr
		}
	}

	logger.Debug(ctx.Context, "Prompting for user consent",
		log.Int("purposeCount", len(promptData.Purposes)))
	execResp.Status = common.ExecUserInputRequired
	return execResp, nil
}

// handleConsentDecisions processes the user's consent decisions.
func (e *consentExecutor) handleConsentDecisions(ctx *core.NodeContext, execResp *common.ExecutorResponse,
	ouID, appID, userID string) (*common.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Processing consent decisions from user")

	decisionsJSON, ok := ctx.UserInputs[userInputConsentDecisions]
	if !ok || decisionsJSON == "" {
		logger.Debug(ctx.Context, "Consent decisions input is missing or empty")
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrConsentDecisionsMissing
		return execResp, nil
	}

	// SanitizeStringMap HTML-escapes all user inputs as an XSS prevention measure.
	// For the consent_decisions field the value is a JSON string, so HTML entities
	// must be unescaped before parsing
	decisionsJSON = html.UnescapeString(decisionsJSON)

	var decisions consentauthn.ConsentDecisions
	if err := json.Unmarshal([]byte(decisionsJSON), &decisions); err != nil {
		logger.Error(ctx.Context, "Failed to parse consent decisions", log.Error(err))
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrConsentDecisionsParseFail
		return execResp, nil
	}

	// Check if the consent prompt has timed out
	if expiresAtStr, ok := ctx.RuntimeData[common.RuntimeKeyStepTimeout]; ok && expiresAtStr != "" {
		if expiresAt, err := strconv.ParseInt(expiresAtStr, 10, 64); err == nil {
			if time.Now().UnixMilli() > expiresAt {
				logger.Debug(ctx.Context, "Consent prompt has timed out", log.Any("expiresAt", expiresAt))
				execResp.Status = common.ExecFailure
				execResp.Error = &ErrConsentPromptTimedOut
				return execResp, nil
			}
		}
	}

	// Determine validity period from the application config
	validityPeriod := int64(0)
	if ctx.Application.LoginConsent != nil {
		validityPeriod = ctx.Application.LoginConsent.ValidityPeriod
	}

	// Retrieve the consent session token from RuntimeData for server-side validation
	sessionToken := ctx.RuntimeData[common.RuntimeKeyConsentSessionToken]

	// Always record consent decisions (including denials) for audit/compliance purposes.
	// The session token is used to verify completeness and enforce essential attribute rules
	consentRecord, svcErr := e.consentEnforcer.RecordConsent(ctx.Context, ouID, appID, userID,
		&decisions, sessionToken, validityPeriod)
	if svcErr != nil {
		// Essential consent denied: the consent record was persisted but the user denied
		// a required attribute, so the flow cannot proceed
		if svcErr.Code == consentauthn.ErrorEssentialConsentDenied.Code {
			logger.Debug(ctx.Context, "User denied essential consent attributes")
			execResp.Status = common.ExecFailure
			execResp.Error = &ErrConsentDenied
			return execResp, nil
		}

		if svcErr.Type == serviceerror.ClientErrorType {
			logger.Debug(ctx.Context, "Client error while recording user consent", log.Any("error", svcErr))
			execResp.Status = common.ExecFailure
			execResp.Error = &ErrConsentRecordFailed
			return execResp, nil
		}

		logger.Error(ctx.Context, "Failed to record consent", log.Any("error", svcErr))
		return nil, errors.New("failed to record consent")
	}

	// Store the consent ID in RuntimeData for downstream usage
	execResp.RuntimeData[common.RuntimeKeyConsentID] = consentRecord.ID

	// Derive approved attribute and permission names from the full (merged) consent record so
	// downstream executors can easily restrict to only consented values without needing to
	// understand the full consent data structure. Both keys are always set (even if empty) so
	// auth assert knows that the consent step ran and can apply the appropriate precedence chain.
	consentedAttrs := collectConsentedAttributes(consentRecord)
	execResp.RuntimeData[common.RuntimeKeyConsentedAttributes] = strings.Join(consentedAttrs, " ")
	consentedPerms := collectConsentedPermissions(consentRecord)
	execResp.RuntimeData[common.RuntimeKeyConsentedPermissions] = strings.Join(consentedPerms, " ")

	logger.Debug(ctx.Context, "Consent recorded successfully", log.String("consentID", consentRecord.ID))
	execResp.Status = common.ExecComplete
	return execResp, nil
}

// getRequiredAttributes retrieves the essential and optional attributes required for consent from the
// runtime data or application assertion.
func (e *consentExecutor) getRequiredAttributes(ctx *core.NodeContext) (
	essentialAttributes, optionalAttributes []string) {
	essentialAttributes = []string{}
	optionalAttributes = []string{}
	requiredAttributesProvided := false

	// Get required attributes from essential and optional attributes if present in runtime data
	if essentialAttrsStr, exists := ctx.RuntimeData[common.RuntimeKeyRequiredEssentialAttributes]; exists {
		requiredAttributesProvided = true
		essentialAttributes = strings.Fields(essentialAttrsStr)
	}
	if optionalAttrsStr, exists := ctx.RuntimeData[common.RuntimeKeyRequiredOptionalAttributes]; exists {
		requiredAttributesProvided = true
		optionalAttributes = strings.Fields(optionalAttrsStr)
	}

	// If neither runtime key was provided but the application has an assertion with user attributes,
	// take those attributes. We treat all assertion attributes as optional
	if !requiredAttributesProvided && ctx.Application.Assertion != nil {
		optionalAttributes = ctx.Application.Assertion.UserAttributes
	}

	return essentialAttributes, optionalAttributes
}

// buildAugmentedAvailableAttributes returns an AttributesResponse value augmented with
// special attribute keys (groups, userType, ouId, ouName, ouHandle) that are present by
// construction in the authenticated user context but are never included in AttributesResponse
// by authentication providers.
//
// When the source is empty, nil is returned so that the downstream consent enforcer
// skips profile-presence filtering entirely.
func (e *consentExecutor) buildAugmentedAvailableAttributes(
	availableAttrResp *authnprovidercm.AttributesResponse, entityRef *authnprovidercm.EntityReference,
) *authnprovidercm.AttributesResponse {
	augmented := make(map[string]*authnprovidercm.AttributeResponse)
	baseVerifications := make(map[string]*authnprovidercm.VerificationResponse)
	hasSource := false

	if base := availableAttrResp; base != nil {
		hasSource = true
		for k, v := range base.Attributes {
			augmented[k] = v
		}
		for k, v := range base.Verifications {
			baseVerifications[k] = v
		}
	}

	if !hasSource {
		return nil
	}

	// Inject special attribute keys.
	// Value is set to empty since the consent enforcer only checks for presence of the key, and the actual values
	// can be obtained from the authenticated user context if needed
	if entityRef.EntityType != "" {
		augmented[oauth2const.ClaimUserType] = &authnprovidercm.AttributeResponse{}
	}
	if entityRef.OUID != "" {
		augmented[oauth2const.ClaimOUID] = &authnprovidercm.AttributeResponse{}
		augmented[oauth2const.ClaimOUName] = &authnprovidercm.AttributeResponse{}
		augmented[oauth2const.ClaimOUHandle] = &authnprovidercm.AttributeResponse{}
	}
	if entityRef.EntityID != "" {
		augmented[oauth2const.UserAttributeGroups] = &authnprovidercm.AttributeResponse{}
	}

	return &authnprovidercm.AttributesResponse{
		Attributes:    augmented,
		Verifications: baseVerifications,
	}
}

// collectConsentedAttributes extracts all approved attribute names from a consent record.
func collectConsentedAttributes(c *consent.Consent) []string {
	return collectApprovedByPurposeNamespace(c, consent.NamespaceAttribute)
}

// collectConsentedPermissions extracts all approved permission names from a consent record.
func collectConsentedPermissions(c *consent.Consent) []string {
	return collectApprovedByPurposeNamespace(c, consent.NamespacePermission)
}

// collectApprovedByPurposeNamespace returns the deduped approved element names across all
// consent purposes in the given namespace. The upstream consent service does not round-trip the
// purpose namespace on reads, so it is derived from the purpose name via
// consent.NamespaceFromPurposeName.
func collectApprovedByPurposeNamespace(c *consent.Consent, ns consent.Namespace) []string {
	var out []string
	for _, p := range c.Purposes {
		if consent.NamespaceFromPurposeName(p.Name) != ns {
			continue
		}
		for _, e := range p.Elements {
			if e.IsUserApproved && !slices.Contains(out, e.Name) {
				out = append(out, e.Name)
			}
		}
	}
	return out
}
