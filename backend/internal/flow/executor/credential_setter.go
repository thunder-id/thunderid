// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/user"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// credentialSetterModeFlowTypes maps each mode to the flow types it runs in: set in any flow, and validate and
// invalidate, the administrator's stage of a credential reset, only in an administrator-initiated flow.
var credentialSetterModeFlowTypes = map[string][]providers.FlowType{
	ExecutorModeSet:        nil,
	ExecutorModeValidate:   {providers.FlowTypeAdminInitiated},
	ExecutorModeInvalidate: {providers.FlowTypeAdminInitiated},
}

// credentialSetter allows users to set their credentials for an existing user account. In an
// administrator-initiated flow it also runs a credential reset of a user or an agent: it checks the reset,
// invalidates the credential once the link was sent, and sets the credential the reset invalidated unless a later
// credential change voided the reset. An agent's reset link goes to the agent's owner.
type credentialSetter struct {
	providers.Executor
	entityProvider    entityprovider.EntityProviderInterface
	entityTypeService entitytype.EntityTypeServiceInterface
	userMgtProvider   providers.UserMgtProvider
	agentMgtProvider  providers.AgentMgtProvider
	authnProvider     providers.AuthnProviderManager
	accessGuard       *accessGuard
	logger            *log.Logger
}

// newCredentialSetter creates a new instance of the credential setter executor.
func newCredentialSetter(
	flowFactory core.FlowFactoryInterface,
	entityProvider entityprovider.EntityProviderInterface,
	userMgtProvider providers.UserMgtProvider,
	agentMgtProvider providers.AgentMgtProvider,
	authnProvider providers.AuthnProviderManager,
	accessGuard *accessGuard,
	entityTypeService entitytype.EntityTypeServiceInterface,
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
		&providers.ExecutorMeta{
			DefaultMode:    ExecutorModeSet,
			SupportedModes: slices.Sorted(maps.Keys(credentialSetterModeFlowTypes)),
		},
	)
	return &credentialSetter{
		Executor:          base,
		entityProvider:    entityProvider,
		entityTypeService: entityTypeService,
		userMgtProvider:   userMgtProvider,
		agentMgtProvider:  agentMgtProvider,
		authnProvider:     authnProvider,
		accessGuard:       accessGuard,
		logger:            logger,
	}
}

// setAgentMgtProvider injects the agent provider. See SetAgentMgtProvider.
func (e *credentialSetter) setAgentMgtProvider(provider providers.AgentMgtProvider) {
	e.agentMgtProvider = provider
}

// Execute runs the node's mode. Without a mode it sets the credential.
func (e *credentialSetter) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	mode := cmp.Or(ctx.ExecutorMode, ExecutorModeSet)
	if flowTypes := credentialSetterModeFlowTypes[mode]; flowTypes != nil && !slices.Contains(flowTypes, ctx.FlowType) {
		return nil, fmt.Errorf("mode %q of %s does not run in a %s flow", mode, ExecutorNameCredentialSetter,
			ctx.FlowType)
	}

	switch mode {
	case ExecutorModeSet:
		if ctx.FlowType == providers.FlowTypeAdminInitiated {
			return e.setResetCredential(ctx, logger)
		}
		return e.set(ctx, logger)
	case ExecutorModeValidate:
		return e.validateReset(ctx, logger)
	case ExecutorModeInvalidate:
		return e.invalidate(ctx, logger)
	default:
		return nil, fmt.Errorf("invalid executor mode for %s: %s", ExecutorNameCredentialSetter, mode)
	}
}

// set sets the credential the node's input names for the user the flow identified.
func (e *credentialSetter) set(ctx *providers.NodeContext, logger *log.Logger) (*providers.ExecutorResponse, error) {
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

	entity, providerErr := e.entityProvider.GetEntity(ctx.Context, userID)
	if providerErr != nil {
		logger.Debug(ctx.Context, "Failed to get the entity", log.MaskedString(log.LoggerKeyUserID, userID),
			log.Error(providerErr))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrCredentialSetFailed
		return execResp, nil
	}

	return e.writeCredential(ctx, execResp, userID, entity.Category, input, credentials, logger), nil
}

// writeCredential writes the credential for the user unless the user is held at the recovery plane, and
// publishes proof of control once it is written. A value the schema refuses asks for the input again.
func (e *credentialSetter) writeCredential(ctx *providers.NodeContext, execResp *providers.ExecutorResponse,
	userID string, category providers.EntityCategory, input providers.Input, credentials json.RawMessage,
	logger *log.Logger) *providers.ExecutorResponse {
	credentialKey := input.Identifier
	// Checked again here, since a hold can be placed after the identifying step's request.
	if held := e.recoveryHold(ctx, execResp, userID, logger); held {
		return execResp
	}

	refused, err := e.setCredentials(ctx, userID, category, credentials)
	if err != nil {
		logger.Debug(ctx.Context, "Failed to update user credentials",
			log.MaskedString(log.LoggerKeyUserID, userID), log.Error(err))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrCredentialSetFailed
		return execResp
	}
	if refused {
		logger.Debug(ctx.Context, "Credential value does not meet the schema requirements",
			log.String("credentialKey", credentialKey))
		execResp.Status = providers.ExecUserInputRequired
		execResp.Inputs = []providers.Input{input}
		execResp.Error = errForAttribute(ErrCredentialValueInvalid, credentialKey)
		return execResp
	}

	// Proof of control for a later node, such as an unlock.
	execResp.SharedRuntimeData = map[string]string{common.RuntimeKeyProvenSubject: userID}

	logger.Debug(ctx.Context, "Successfully set credentials for user",
		log.MaskedString(log.LoggerKeyUserID, userID))
	execResp.Status = providers.ExecComplete
	return execResp
}

// recoveryHold fails the response and reports true when the user is held at the recovery plane, such as by
// a suspension, stating the hold where the decision names one. An automatic lock does not hold there.
func (e *credentialSetter) recoveryHold(ctx *providers.NodeContext, execResp *providers.ExecutorResponse,
	userID string, logger *log.Logger) bool {
	held, stated, fault := e.accessGuard.heldAt(ctx.Context, userID, providers.AccessPlaneRecovery, "", logger)
	if fault != nil {
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrCredentialSetFailed
		return true
	}
	if held {
		execResp.Status = providers.ExecFailure
		execResp.Error = cmp.Or(stated, &ErrCredentialSetFailed)
		return true
	}
	return false
}

// setCredentials writes the entity's credentials and reports whether the value was refused. A user's go
// through the user management provider, so the user service revokes what the user holds. An agent's in an
// administrator-initiated reset go through the agent management provider, which checks the administrator's
// control of the agent and audits the change. Any other entity's are written through the entity provider.
func (e *credentialSetter) setCredentials(ctx *providers.NodeContext, entityID string,
	category providers.EntityCategory, credentials json.RawMessage) (bool, error) {
	switch {
	case category == providers.EntityCategoryUser:
		if svcErr := e.userMgtProvider.UpdateUserCredentials(ctx.Context, entityID, credentials); svcErr != nil {
			if svcErr.Code == user.ErrorInvalidCredentialValue.Code {
				return true, nil
			}
			return false, fmt.Errorf("failed to update user credentials: %s", svcErr.Code)
		}
		return false, nil
	case category == providers.EntityCategoryAgent && ctx.FlowType == providers.FlowTypeAdminInitiated:
		if e.agentMgtProvider == nil {
			return false, errors.New("agent management provider is not configured")
		}
		if svcErr := e.agentMgtProvider.UpdateAgentCredentials(ctx.Context, entityID, credentials); svcErr != nil {
			if svcErr.Code == agentSchemaValidationFailedCode {
				return true, nil
			}
			return false, fmt.Errorf("failed to update agent credentials: %s", svcErr.Code)
		}
		return false, nil
	}
	if providerErr := e.entityProvider.UpdateCredentials(ctx.Context, entityID, credentials); providerErr != nil {
		if providerErr.Code == entityprovider.ErrorCodeSchemaValidationFailed {
			return true, nil
		}
		return false, fmt.Errorf("failed to update entity credentials: %w", providerErr)
	}
	return false, nil
}

// validateReset checks, changing nothing, that the administrator may invalidate the chosen credential of the
// user or agent the flow was started for, and that it is not held. The checks run before anything is sent, so a
// refused reset sends nothing. An agent's link goes to its owner's email address, which the email step is given as
// the expected recipient, so it never reads an address from the agent.
func (e *credentialSetter) validateReset(ctx *providers.NodeContext,
	logger *log.Logger) (*providers.ExecutorResponse, error) {
	execResp := e.newResetResponse()
	subject := ctx.GetBoundSubject()
	if subject == "" {
		return e.resetFailed(execResp, &ErrUserIDMissingInContext), nil
	}
	if ctx.RuntimeData[common.RuntimeKeyResetCredentialMarker] != "" {
		return e.resetFailed(execResp, &ErrCredentialSetFailed), nil
	}
	input := e.credentialTypeInput(ctx)
	credentialType := strings.TrimSpace(ctx.UserInputs[input.Identifier])
	if credentialType == "" {
		execResp.Status = providers.ExecUserInputRequired
		execResp.Inputs = []providers.Input{input}
		return execResp, nil
	}

	category, err := e.resetSubjectCategory(ctx, subject)
	if err != nil {
		return nil, err
	}
	if category == providers.EntityCategoryAgent {
		recipient, svcErr := e.agentMgtProvider.ValidateRemoveAgentCredential(ctx.Context, subject, credentialType)
		if svcErr != nil {
			return refusalResponse(execResp, svcErr, ErrCredentialSetFailed, "failed to check the credential reset")
		}
		execResp.RuntimeData[common.RuntimeKeyRecipientType] = entitytypemodel.TypeEmail
		execResp.RuntimeData[common.RuntimeKeyExpectedRecipient] = recipient
		execResp.RuntimeData[common.RuntimeKeyResetSubjectCategory] = string(providers.EntityCategoryAgent)
	} else if svcErr := e.userMgtProvider.ValidateRemoveUserCredential(ctx.Context, subject,
		credentialType); svcErr != nil {
		return refusalResponse(execResp, svcErr, ErrCredentialSetFailed, "failed to check the credential reset")
	}
	if e.recoveryHold(ctx, execResp, subject, logger) {
		return execResp, nil
	}

	// Both administrator inputs are forgotten, so the set step cannot read either as the credential's new value.
	delete(ctx.UserInputs, revocationInputSubject)
	delete(ctx.UserInputs, input.Identifier)
	execResp.RuntimeData[userAttributeUserID] = subject
	execResp.RuntimeData[common.RuntimeKeyResetCredential] = credentialType
	execResp.RuntimeData[common.RuntimeKeyStoredInviteToken] = ""
	execResp.RuntimeData[common.RuntimeKeyInviteLink] = ""
	execResp.SharedRuntimeData = map[string]string{common.RuntimeKeyDeliveredRecipients: ""}
	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// invalidate removes the credential the reset checked, once its link was delivered to the stored address,
// and records the marker the removal wrote. A failure voids the link that was sent, so it never completes a
// reset whose credential was not invalidated.
func (e *credentialSetter) invalidate(ctx *providers.NodeContext,
	logger *log.Logger) (*providers.ExecutorResponse, error) {
	execResp := e.newResetResponse()
	subject := ctx.GetBoundSubject()
	credentialType := ctx.RuntimeData[common.RuntimeKeyResetCredential]
	if subject == "" || credentialType == "" || ctx.RuntimeData[common.RuntimeKeyResetCredentialMarker] != "" {
		return e.voidedReset(execResp, &ErrCredentialInvalidationFailed), nil
	}
	delivered, err := deliveredRecipients(ctx)
	if err != nil || ctx.RuntimeData[common.RuntimeKeyStoredInviteToken] == "" || len(delivered) == 0 {
		return e.voidedReset(execResp, &ErrCredentialResetNotDelivered), nil
	}

	var marker string
	var svcErr *tidcommon.ServiceError
	if e.resetsAgent(ctx) {
		marker, svcErr = e.agentMgtProvider.RemoveAgentCredential(ctx.Context, subject, credentialType)
	} else {
		marker, svcErr = e.userMgtProvider.RemoveUserCredential(ctx.Context, subject, credentialType)
	}
	if svcErr != nil {
		logger.Debug(ctx.Context, "Failed to invalidate the credential",
			log.MaskedString(log.LoggerKeyUserID, subject), log.String("errorCode", svcErr.Code))
		return e.voidedReset(execResp, &ErrCredentialInvalidationFailed), nil
	}
	if marker == "" {
		logger.Debug(ctx.Context, "Credential invalidation returned no marker",
			log.MaskedString(log.LoggerKeyUserID, subject))
		return e.voidedReset(execResp, &ErrCredentialInvalidationFailed), nil
	}

	execResp.RuntimeData[common.RuntimeKeyResetCredentialMarker] = marker
	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// setResetCredential sets the credential an administrator-initiated reset invalidated, from the input of the
// same name, while the reset is current and the user is not held. Both are checked on every request, before the
// credential is asked for and before it is written. No other credential, and no other user, can be set this way,
// and the value is read only from a request after the one that opened the link.
func (e *credentialSetter) setResetCredential(ctx *providers.NodeContext,
	logger *log.Logger) (*providers.ExecutorResponse, error) {
	execResp := e.newResetResponse()
	subject, current, err := e.currentReset(ctx)
	if err != nil {
		return nil, err
	}
	if !current {
		return e.resetFailed(execResp, &ErrInvalidInviteToken), nil
	}

	credentialType := ctx.RuntimeData[common.RuntimeKeyResetCredential]
	// The set step first runs in the request that opened the link. The value is read only from a request after
	// it, so an input left in the execution by an earlier request is never written.
	if ctx.RuntimeData[common.RuntimeKeyResetCredentialRequested] == "" {
		delete(ctx.UserInputs, credentialType)
		execResp.RuntimeData[common.RuntimeKeyResetCredentialRequested] = dataValueTrue
	}
	value := ctx.UserInputs[credentialType]
	// The hold is checked once per request: here before the credential is asked for, or by writeCredential
	// before the write.
	if value == "" && e.recoveryHold(ctx, execResp, subject, logger) {
		return execResp, nil
	}

	entity, attributes, err := e.entityAttributes(ctx, subject)
	if err != nil {
		return nil, err
	}
	input, ok := e.resetCredentialInput(attributes, credentialType)
	if !ok {
		return e.resetFailed(execResp, &ErrCredentialSetFailed), nil
	}
	if value == "" {
		execResp.Status = providers.ExecUserInputRequired
		execResp.Inputs = []providers.Input{input}
		return e.withForwardedInputs(execResp), nil
	}
	credentials, err := json.Marshal(map[string]string{credentialType: value})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal credentials: %w", err)
	}
	return e.withForwardedInputs(e.writeCredential(ctx, execResp, subject, entity.Category, input, credentials,
		logger)), nil
}

// entityAttributes returns the entity's profile and the top-level attributes of its type, credentials included.
func (e *credentialSetter) entityAttributes(ctx *providers.NodeContext,
	entityID string) (*providers.Entity, []entitytypemodel.AttributeInfo, error) {
	if e.entityTypeService == nil {
		return nil, nil, errors.New("entity type service is not available")
	}
	entity, providerErr := e.entityProvider.GetEntityProfile(ctx.Context, entityID)
	if providerErr != nil {
		return nil, nil, fmt.Errorf("failed to get entity profile: %w", providerErr)
	}
	attributes, svcErr := e.entityTypeService.GetAttributes(ctx.Context, entitytype.TypeCategory(entity.Category),
		entity.Type, entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: true})
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to get entity type attributes: %s", svcErr.Code)
	}
	return entity, attributes, nil
}

// currentReset returns the user or agent of the flow's credential reset and whether the reset is still current:
// its invalidation was recorded, and the management provider of its category says no credential change followed
// it.
func (e *credentialSetter) currentReset(ctx *providers.NodeContext) (string, bool, error) {
	subject := ctx.GetBoundSubject()
	recorded := ctx.RuntimeData[common.RuntimeKeyResetCredentialMarker]
	if subject == "" || recorded == "" || ctx.RuntimeData[common.RuntimeKeyResetCredential] == "" {
		return subject, false, nil
	}
	var current bool
	var svcErr *tidcommon.ServiceError
	if e.resetsAgent(ctx) {
		current, svcErr = e.agentMgtProvider.IsAgentCredentialUnchangedSince(ctx.Context, subject, recorded)
	} else {
		current, svcErr = e.userMgtProvider.IsUserCredentialUnchangedSince(ctx.Context, subject, recorded)
	}
	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			return subject, false, nil
		}
		return "", false, fmt.Errorf("failed to check the credential reset: %s", svcErr.Code)
	}
	return subject, current, nil
}

// resetSubjectCategory returns the category of the entity a credential reset is for: agent for an agent, and user
// for any other entity or one that does not exist, so the user management provider refuses it as it always has.
// An agent's reset fails when no agent management provider is configured.
func (e *credentialSetter) resetSubjectCategory(ctx *providers.NodeContext,
	subject string) (providers.EntityCategory, error) {
	entity, providerErr := e.entityProvider.GetEntity(ctx.Context, subject)
	if providerErr != nil {
		if providerErr.Code == entityprovider.ErrorCodeEntityNotFound {
			return providers.EntityCategoryUser, nil
		}
		return "", fmt.Errorf("failed to get the entity: %w", providerErr)
	}
	if entity.Category != providers.EntityCategoryAgent {
		return providers.EntityCategoryUser, nil
	}
	if e.agentMgtProvider == nil {
		return "", errors.New("agent management provider is not configured")
	}
	return providers.EntityCategoryAgent, nil
}

// resetsAgent reports whether the flow's credential reset is for an agent, as validateReset recorded.
func (e *credentialSetter) resetsAgent(ctx *providers.NodeContext) bool {
	return ctx.RuntimeData[common.RuntimeKeyResetSubjectCategory] == string(providers.EntityCategoryAgent)
}

// credentialTypeInput returns the input that names the credential to reset: the node's first input, or
// credential when the node declares none.
func (e *credentialSetter) credentialTypeInput(ctx *providers.NodeContext) providers.Input {
	if len(ctx.NodeInputs) > 0 && ctx.NodeInputs[0].Identifier != "" {
		return ctx.NodeInputs[0]
	}
	return providers.Input{Identifier: userInputCredential, Type: providers.InputTypeText, Required: true}
}

// withForwardedInputs forwards the inputs a response asks for, so the prompt its node continues to renders them.
func (e *credentialSetter) withForwardedInputs(execResp *providers.ExecutorResponse) *providers.ExecutorResponse {
	if execResp.Status != providers.ExecUserInputRequired || len(execResp.Inputs) == 0 {
		return execResp
	}
	execResp.ForwardedData = map[string]any{common.ForwardedDataKeyInputs: execResp.Inputs}
	return execResp
}

// resetCredentialInput returns the masked input for the string credential of that name. The user service refuses
// to remove any other kind, so a reset never reaches the set step for one.
func (e *credentialSetter) resetCredentialInput(attributes []entitytypemodel.AttributeInfo,
	credential string) (providers.Input, bool) {
	for _, attribute := range attributes {
		if attribute.Attribute != credential || !attribute.Credential ||
			attribute.Type != entitytypemodel.TypeString {
			continue
		}
		return providers.Input{Identifier: credential, Type: providers.InputTypePassword, Required: true,
			DisplayName: cmp.Or(attribute.DisplayName, credential)}, true
	}
	return providers.Input{}, false
}

// newResetResponse returns an empty response for a credential reset mode.
func (e *credentialSetter) newResetResponse() *providers.ExecutorResponse {
	return &providers.ExecutorResponse{AdditionalData: map[string]string{}, RuntimeData: map[string]string{}}
}

// resetFailed fails the response with svcErr.
func (e *credentialSetter) resetFailed(execResp *providers.ExecutorResponse,
	svcErr *tidcommon.ServiceError) *providers.ExecutorResponse {
	execResp.Status = providers.ExecFailure
	execResp.Error = svcErr
	return execResp
}

// voidedReset fails the response with svcErr and clears the stored link token, so a link that was sent
// cannot be verified.
func (e *credentialSetter) voidedReset(execResp *providers.ExecutorResponse,
	svcErr *tidcommon.ServiceError) *providers.ExecutorResponse {
	execResp.RuntimeData[common.RuntimeKeyStoredInviteToken] = ""
	return e.resetFailed(execResp, svcErr)
}
