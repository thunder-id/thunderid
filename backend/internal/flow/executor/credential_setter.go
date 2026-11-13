// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
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

// credentialResetModes are the modes of an administrator-initiated credential reset, which run only in an
// administrator-initiated flow, in the administrator's stage.
var credentialResetModes = []string{ExecutorModeValidate, ExecutorModeInvalidate}

// credentialSetter allows users to set their credentials for an existing user account. In an
// administrator-initiated flow it also runs a credential reset: it checks the reset, invalidates the credential
// once the link was sent, and sets the credential the reset invalidated unless a later credential change voided
// the reset.
type credentialSetter struct {
	providers.Executor
	entityProvider    entityprovider.EntityProviderInterface
	entityTypeService entitytype.EntityTypeServiceInterface
	userMgtProvider   providers.UserMgtProvider
	authnProvider     providers.AuthnProviderManager
	accessGuard       *accessGuard
	logger            *log.Logger
}

// newCredentialSetter creates a new instance of the credential setter executor.
func newCredentialSetter(
	flowFactory core.FlowFactoryInterface,
	entityProvider entityprovider.EntityProviderInterface,
	userMgtProvider providers.UserMgtProvider,
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
			SupportedModes: append([]string{ExecutorModeSet}, credentialResetModes...),
		},
	)
	return &credentialSetter{
		Executor:          base,
		entityProvider:    entityProvider,
		entityTypeService: entityTypeService,
		userMgtProvider:   userMgtProvider,
		authnProvider:     authnProvider,
		accessGuard:       accessGuard,
		logger:            logger,
	}
}

// Execute runs the node's mode. Without a mode it sets the credential.
func (e *credentialSetter) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	mode := cmp.Or(ctx.ExecutorMode, ExecutorModeSet)
	if slices.Contains(credentialResetModes, mode) && ctx.FlowType != providers.FlowTypeAdminInitiated {
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

	return e.writeCredential(ctx, execResp, userID, input, credentials, logger), nil
}

// writeCredential writes the credential for the user unless the user is held at the recovery plane, and
// publishes proof of control once it is written. A value the schema refuses asks for the input again.
func (e *credentialSetter) writeCredential(ctx *providers.NodeContext, execResp *providers.ExecutorResponse,
	userID string, input providers.Input, credentials json.RawMessage,
	logger *log.Logger) *providers.ExecutorResponse {
	credentialKey := input.Identifier
	// Checked again here, since a hold can be placed after the identifying step's request.
	if held := e.recoveryHold(ctx, execResp, userID, logger); held {
		return execResp
	}

	writeCtx := ctx.Context
	if ctx.FlowType == providers.FlowTypeAdminInitiated {
		writeCtx = user.WithCredentialResetCompletion(writeCtx)
	}
	refused, err := e.setCredentials(writeCtx, userID, credentials)
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
// through the user management provider, so the user service revokes what the user holds. Any other
// entity's are written through the entity provider.
func (e *credentialSetter) setCredentials(ctx context.Context, entityID string,
	credentials json.RawMessage) (bool, error) {
	entity, providerErr := e.entityProvider.GetEntity(ctx, entityID)
	if providerErr != nil {
		return false, fmt.Errorf("failed to get entity: %w", providerErr)
	}
	if entity.Category == providers.EntityCategoryUser {
		if svcErr := e.userMgtProvider.UpdateUserCredentials(ctx, entityID, credentials); svcErr != nil {
			if svcErr.Code == user.ErrorInvalidCredentialValue.Code {
				return true, nil
			}
			return false, fmt.Errorf("failed to update user credentials: %s", svcErr.Code)
		}
		return false, nil
	}
	if providerErr := e.entityProvider.UpdateCredentials(ctx, entityID, credentials); providerErr != nil {
		if providerErr.Code == entityprovider.ErrorCodeSchemaValidationFailed {
			return true, nil
		}
		return false, fmt.Errorf("failed to update entity credentials: %w", providerErr)
	}
	return false, nil
}

// validateReset checks, changing nothing, that the administrator may invalidate the chosen credential of the
// user the flow was started for, that the user is not held, and that the request names only the user and the
// credential, so the sender cannot read a recipient from it. The checks run before anything is sent, so a refused
// reset sends nothing.
func (e *credentialSetter) validateReset(ctx *providers.NodeContext,
	logger *log.Logger) (*providers.ExecutorResponse, error) {
	execResp := newCredentialResetResponse()
	subject := ctx.GetBoundSubject()
	if subject == "" {
		return resetFailed(execResp, &ErrUserIDMissingInContext), nil
	}
	if ctx.RuntimeData[common.RuntimeKeyResetCredentialMarker] != "" {
		return resetFailed(execResp, &ErrCredentialSetFailed), nil
	}
	input := e.credentialTypeInput(ctx)
	// The flow's sender reads a request input before the stored profile, so no other input is accepted.
	for name := range ctx.UserInputs {
		if name != userInputSubject && name != input.Identifier {
			return resetFailed(execResp, &ErrResetInputNotAllowed), nil
		}
	}
	credentialType := strings.TrimSpace(ctx.UserInputs[input.Identifier])
	if credentialType == "" {
		execResp.Status = providers.ExecUserInputRequired
		execResp.Inputs = []providers.Input{input}
		return execResp, nil
	}

	if svcErr := e.userMgtProvider.ValidateRemoveUserCredential(ctx.Context, subject,
		credentialType); svcErr != nil {
		return resetRefused(execResp, svcErr)
	}
	if e.recoveryHold(ctx, execResp, subject, logger) {
		return execResp, nil
	}

	attributes, err := e.userAttributes(ctx, subject)
	if err != nil {
		return nil, err
	}
	if _, ok := resetCredentialInput(attributes, credentialType); !ok {
		return resetFailed(execResp, &ErrCredentialInputInvalid), nil
	}

	execResp.RuntimeData[userAttributeUserID] = subject
	execResp.RuntimeData[common.RuntimeKeyResetCredential] = credentialType
	execResp.RuntimeData[common.RuntimeKeyStoredInviteToken] = ""
	execResp.RuntimeData[common.RuntimeKeyInviteLink] = ""
	execResp.SharedRuntimeData = map[string]string{common.RuntimeKeyDeliveredRecipients: ""}
	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// invalidate removes the credential the reset checked, once its link was delivered,
// and records the marker the removal wrote. A failure voids the link that was sent, so it never completes a
// reset whose credential was not invalidated.
func (e *credentialSetter) invalidate(ctx *providers.NodeContext,
	logger *log.Logger) (*providers.ExecutorResponse, error) {
	execResp := newCredentialResetResponse()
	subject := ctx.GetBoundSubject()
	credentialType := ctx.RuntimeData[common.RuntimeKeyResetCredential]
	if subject == "" || credentialType == "" || ctx.RuntimeData[common.RuntimeKeyResetCredentialMarker] != "" {
		return voidedReset(execResp, &ErrCredentialInvalidationFailed), nil
	}
	delivered, err := deliveredRecipients(ctx)
	if err != nil || ctx.RuntimeData[common.RuntimeKeyStoredInviteToken] == "" || len(delivered) == 0 {
		return voidedReset(execResp, &ErrCredentialResetNotDelivered), nil
	}

	marker, svcErr := e.userMgtProvider.RemoveUserCredential(ctx.Context, subject, credentialType)
	if svcErr != nil {
		logger.Debug(ctx.Context, "Failed to invalidate the user's credential",
			log.MaskedString(log.LoggerKeyUserID, subject), log.String("errorCode", svcErr.Code))
		return voidedReset(execResp, &ErrCredentialInvalidationFailed), nil
	}
	if marker == "" {
		logger.Debug(ctx.Context, "Credential invalidation returned no marker",
			log.MaskedString(log.LoggerKeyUserID, subject))
		return voidedReset(execResp, &ErrCredentialInvalidationFailed), nil
	}

	execResp.RuntimeData[common.RuntimeKeyResetCredentialMarker] = marker
	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// setResetCredential sets the credential an administrator-initiated reset invalidated, from the input of the
// same name, while the reset is current and the user is not held. Both are checked on every request, before the
// credential is asked for and before it is written. No other credential, and no other user, can be set this way.
func (e *credentialSetter) setResetCredential(ctx *providers.NodeContext,
	logger *log.Logger) (*providers.ExecutorResponse, error) {
	execResp := newCredentialResetResponse()
	subject, current, err := e.currentReset(ctx)
	if err != nil {
		return nil, err
	}
	if !current {
		return resetFailed(execResp, &ErrInvalidInviteToken), nil
	}
	// Refused before the credential is asked for; writeCredential checks again before the write.
	if e.recoveryHold(ctx, execResp, subject, logger) {
		return execResp, nil
	}

	credentialType := ctx.RuntimeData[common.RuntimeKeyResetCredential]
	attributes, err := e.userAttributes(ctx, subject)
	if err != nil {
		return nil, err
	}
	input, ok := resetCredentialInput(attributes, credentialType)
	if !ok {
		return resetFailed(execResp, &ErrCredentialSetFailed), nil
	}
	value := ctx.UserInputs[credentialType]
	if value == "" {
		execResp.Status = providers.ExecUserInputRequired
		execResp.Inputs = []providers.Input{input}
		return withForwardedInputs(execResp), nil
	}
	credentials, err := json.Marshal(map[string]string{credentialType: value})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal credentials: %w", err)
	}
	return withForwardedInputs(e.writeCredential(ctx, execResp, subject, input, credentials, logger)), nil
}

// withForwardedInputs forwards the inputs a response asks for, so the prompt its node continues to renders them.
func withForwardedInputs(execResp *providers.ExecutorResponse) *providers.ExecutorResponse {
	if execResp.Status != providers.ExecUserInputRequired || len(execResp.Inputs) == 0 {
		return execResp
	}
	execResp.ForwardedData = map[string]interface{}{common.ForwardedDataKeyInputs: execResp.Inputs}
	return execResp
}

// userAttributes returns the top-level attributes of the user's type, credentials included.
func (e *credentialSetter) userAttributes(ctx *providers.NodeContext,
	userID string) ([]entitytypemodel.AttributeInfo, error) {
	if e.entityTypeService == nil {
		return nil, errors.New("entity type service is not available")
	}
	entity, providerErr := e.entityProvider.GetEntityProfile(ctx.Context, userID)
	if providerErr != nil {
		return nil, fmt.Errorf("failed to get entity profile: %w", providerErr)
	}
	attributes, svcErr := e.entityTypeService.GetAttributes(ctx.Context, entitytype.TypeCategoryUser,
		entity.Type, entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: true})
	if svcErr != nil {
		return nil, fmt.Errorf("failed to get user attributes: %s", svcErr.Code)
	}
	return attributes, nil
}

// resetCredentialInput returns the masked input for the string credential of that name. A number credential is
// not hashed when a user is created, so it is not reset this way.
func resetCredentialInput(attributes []entitytypemodel.AttributeInfo, credential string) (providers.Input, bool) {
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

// currentReset returns the user of the flow's credential reset and whether the reset is still current: its
// invalidation was recorded, and the user's credential marker still holds the value the invalidation wrote.
func (e *credentialSetter) currentReset(ctx *providers.NodeContext) (string, bool, error) {
	subject := ctx.GetBoundSubject()
	recorded := ctx.RuntimeData[common.RuntimeKeyResetCredentialMarker]
	if subject == "" || recorded == "" || ctx.RuntimeData[common.RuntimeKeyResetCredential] == "" {
		return subject, false, nil
	}
	entity, providerErr := e.entityProvider.GetEntity(ctx.Context, subject)
	if providerErr != nil {
		if providerErr.Code == entityprovider.ErrorCodeEntityNotFound {
			return subject, false, nil
		}
		return "", false, fmt.Errorf("failed to get entity: %w", providerErr)
	}
	var attrs map[string]any
	if len(entity.SystemAttributes) > 0 {
		if err := json.Unmarshal(entity.SystemAttributes, &attrs); err != nil {
			return "", false, fmt.Errorf("failed to parse system attributes: %w", err)
		}
	}
	marker, _ := attrs[authnprovidercm.SystemAttrCredentialUpdatedAt].(string)
	return subject, marker == recorded, nil
}

// credentialTypeInput returns the input that names the credential to reset: the node's first input, or
// credential when the node declares none.
func (e *credentialSetter) credentialTypeInput(ctx *providers.NodeContext) providers.Input {
	if len(ctx.NodeInputs) > 0 && ctx.NodeInputs[0].Identifier != "" {
		return ctx.NodeInputs[0]
	}
	return providers.Input{Identifier: userInputCredential, Type: providers.InputTypeText, Required: true}
}

// newCredentialResetResponse returns an empty response for a credential reset mode.
func newCredentialResetResponse() *providers.ExecutorResponse {
	return &providers.ExecutorResponse{AdditionalData: map[string]string{}, RuntimeData: map[string]string{}}
}

// resetFailed fails the response with svcErr.
func resetFailed(execResp *providers.ExecutorResponse,
	svcErr *tidcommon.ServiceError) *providers.ExecutorResponse {
	execResp.Status = providers.ExecFailure
	execResp.Error = svcErr
	return execResp
}

// resetRefused fails the response with the user service's client error, or returns a server error.
func resetRefused(execResp *providers.ExecutorResponse,
	svcErr *tidcommon.ServiceError) (*providers.ExecutorResponse, error) {
	if svcErr.Type != tidcommon.ClientErrorType {
		return nil, fmt.Errorf("failed to check the credential reset: %s", svcErr.Code)
	}
	return resetFailed(execResp, svcErr), nil
}

// voidedReset fails the response with svcErr and clears the stored link token, so a link that was sent
// cannot be verified.
func voidedReset(execResp *providers.ExecutorResponse,
	svcErr *tidcommon.ServiceError) *providers.ExecutorResponse {
	execResp.RuntimeData[common.RuntimeKeyStoredInviteToken] = ""
	return resetFailed(execResp, svcErr)
}
