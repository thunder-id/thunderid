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
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
)

type entityRef struct {
	entityType string
	ouID       string
}

// provisioningExecutor implements the ExecutorInterface for user provisioning in a flow.
type provisioningExecutor struct {
	providers.Executor
	identifyingExecutorInterface
	entityProvider        entityprovider.EntityProviderInterface
	groupService          group.GroupServiceInterface
	roleService           role.RoleServiceInterface
	roleAssignmentService role.RoleAssignmentServiceInterface
	entityTypeService     entitytype.EntityTypeServiceInterface
	authnProvider         providers.AuthnProviderManager
	logger                *log.Logger
}

var _ providers.Executor = (*provisioningExecutor)(nil)
var _ identifyingExecutorInterface = (*provisioningExecutor)(nil)

// newProvisioningExecutor creates a new instance of ProvisioningExecutor.
func newProvisioningExecutor(
	flowFactory core.FlowFactoryInterface,
	groupService group.GroupServiceInterface,
	roleService role.RoleServiceInterface,
	roleAssignmentService role.RoleAssignmentServiceInterface,
	entityProvider entityprovider.EntityProviderInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	authnProvider providers.AuthnProviderManager,
) *provisioningExecutor {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, ExecutorNameProvisioning),
		log.String(log.LoggerKeyExecutorName, ExecutorNameProvisioning))

	base := flowFactory.CreateExecutor(ExecutorNameProvisioning, providers.ExecutorTypeRegistration,
		[]providers.Input{}, []providers.Input{}, &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{
				providers.FlowTypeAuthentication,
				providers.FlowTypeRegistration,
				providers.FlowTypeUserOnboarding,
			},
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: propertyKeyDynamicInputsIncludeOptional},
				{Property: propertyKeyDynamicInputsIncludeOptionalCredentials},
				{Property: propertyKeyMaxDynamicInputsPerPrompt},
				{Property: propertyKeyAssignGroup},
				{Property: propertyKeyAssignRole},
				{Property: common.NodePropertyAllowCrossOUProvisioning},
			},
		})

	identifyingExec := newIdentifyingExecutor(ExecutorNameProvisioning,
		[]providers.Input{}, []providers.Input{}, flowFactory, entityProvider)

	return &provisioningExecutor{
		Executor:                     base,
		identifyingExecutorInterface: identifyingExec,
		entityProvider:               entityProvider,
		groupService:                 groupService,
		roleService:                  roleService,
		roleAssignmentService:        roleAssignmentService,
		entityTypeService:            entityTypeService,
		authnProvider:                authnProvider,
		logger:                       logger,
	}
}

// Execute executes the user provisioning logic based on the inputs provided.
func (p *provisioningExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing user provisioning executor")

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		AuthUser:       ctx.AuthUser,
	}

	// If it's an authentication flow, skip execution if the user is not eligible for provisioning
	if ctx.FlowType == providers.FlowTypeAuthentication {
		eligible, ok := ctx.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning]
		if !ok || eligible != dataValueTrue {
			logger.Debug(ctx.Context, "User is not eligible for provisioning, skipping execution")
			execResp.Status = providers.ExecComplete
			return execResp, nil
		}
	}

	if !p.HasRequiredInputs(ctx, execResp) {
		if execResp.Status == providers.ExecFailure {
			return execResp, nil
		}

		logger.Debug(ctx.Context, "Required inputs for provisioning executor is not provided")
		execResp.Status = providers.ExecUserInputRequired
		return execResp, nil
	}

	identifyingAttrs, credentialAttrs, err := p.getAttributesForProvisioning(ctx)
	if err != nil {
		return nil, err
	}
	if len(identifyingAttrs) == 0 && len(credentialAttrs) == 0 {
		logger.Debug(ctx.Context, "No user attributes provided for provisioning")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrProvisioningUserAttrsMissing
		return execResp, nil
	}

	userID, err := p.IdentifyUser(ctx.Context, identifyingAttrs, execResp)
	if err != nil {
		logger.Error(ctx.Context, "Failed to identify user", log.Error(err))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrFailedToIdentifyUser
		return execResp, nil
	}
	if execResp.Status == providers.ExecFailure &&
		execResp.Error != nil && execResp.Error.Code == ErrAmbiguousUserIdentity.Code &&
		isCrossOUProvisioningAllowed(ctx) {
		resolved, err := p.resolveAmbiguousUserForProvisioning(ctx, identifyingAttrs)
		if err != nil {
			return nil, err
		}
		userID = resolved
		execResp.Status = ""
		execResp.Error = nil
	}
	if execResp.Status == providers.ExecFailure &&
		(execResp.Error == nil || execResp.Error.Code != ErrUserNotFound.Code) {
		return execResp, nil
	}
	// clear execResp set by IdentifyUser
	execResp.Status = ""
	execResp.Error = nil
	if userID != nil && *userID != "" {
		shouldContinue, err := p.handleExistingUser(ctx, *userID, execResp, logger)
		if err != nil {
			return nil, err
		}
		if !shouldContinue {
			return execResp, nil
		}
	}

	// Merge identifying and credential attributes for user creation
	userAttributes := make(map[string]interface{}, len(identifyingAttrs)+len(credentialAttrs))
	for k, v := range identifyingAttrs {
		userAttributes[k] = v
	}
	for k, v := range credentialAttrs {
		userAttributes[k] = v
	}
	createdEntity, err := p.createUserInStore(ctx, userAttributes)
	if err != nil {
		execResp.Status = providers.ExecFailure
		execResp.Error = p.handleCreateUserError(ctx, err, logger)
		return execResp, nil
	}
	if createdEntity == nil || createdEntity.ID == "" {
		logger.Error(ctx.Context, "Created user is nil or has no ID")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrProvisioningFailed
		return execResp, nil
	}

	logger.Debug(ctx.Context, "User created successfully",
		log.MaskedString(log.LoggerKeyUserID, createdEntity.ID))

	// Assign user to groups and roles
	if err := p.assignGroupsAndRoles(ctx, createdEntity.ID); err != nil {
		logger.Error(ctx.Context, "Failed to assign groups and roles to provisioned user",
			log.MaskedString(log.LoggerKeyUserID, createdEntity.ID),
			log.Error(err))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrProvisioningAssignmentFailed
		return execResp, nil
	}

	p.authenticateProvisionedUser(ctx, createdEntity.ID, execResp)
	if execResp.Status == providers.ExecFailure {
		return execResp, nil
	}

	execResp.Status = providers.ExecComplete

	// Set the auto-provisioned flag if it's a user auto provisioning scenario
	if ctx.FlowType == providers.FlowTypeAuthentication {
		execResp.RuntimeData[common.RuntimeKeyUserAutoProvisioned] = dataValueTrue
	}

	return execResp, nil
}

// authenticateProvisionedUser authenticates the newly provisioned user and updates the executor response.
func (p *provisioningExecutor) authenticateProvisionedUser(ctx *providers.NodeContext, userID string,
	execResp *providers.ExecutorResponse) {
	credential := map[string]interface{}{
		"provisionedEntityID": userID,
	}
	authUser, authenticatedClaims, err := p.authnProvider.AuthenticateUser(ctx.Context, nil, credential,
		nil, nil, execResp.AuthUser)
	if !authUser.IsAuthenticated() || err != nil {
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrUserAuthFailed
		return
	}
	execResp.AuthUser = authUser
	for key, value := range authenticatedClaims {
		execResp.RuntimeData[key] = systemutils.ConvertInterfaceValueToString(value)
	}
}

// handleNonProvisionableUserInAuthentication sets the exec response when an existing user is found
// during an authentication flow and provisioning cannot proceed.
// Provisioning is simply skipped and the flow continues with the existing user.
func (p *provisioningExecutor) handleNonProvisionableUserInAuthentication(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) {
	p.logger.Debug(ctx.Context, "Skipping provisioning and continuing with existing user")
	execResp.Status = providers.ExecComplete
}

// handleNonProvisionableUserInRegistration sets the exec response when an existing user is found
// during a registration or onboarding flow and provisioning cannot proceed.
// It either allows the flow to skip provisioning, prompts for different input, or fails immediately.
func (p *provisioningExecutor) handleNonProvisionableUserInRegistration(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse, existsErr *tidcommon.ServiceError) {
	if isAllowRegistrationWithExistingUserRuntimeFlagSet(ctx) {
		execResp.Status = providers.ExecComplete
		return
	}
	requiredInputs := p.GetRequiredInputs(ctx)
	if len(requiredInputs) > 0 {
		// Existing user identified based on user input attributes.
		// Allow the user to input different attributes for registration.
		execResp.Status = providers.ExecUserInputRequired
		execResp.Inputs = requiredInputs
		execResp.Error = existsErr
		return
	}
	// Existing user identified without user input attributes.
	// User cannot recover from error by changing input, so fail immediately.
	execResp.Status = providers.ExecFailure
	execResp.Error = existsErr
}

// handleExistingUser handles the case where a user with the given ID already exists.
// Returns true if provisioning should proceed (cross-OU case), false if execution should stop.
func (p *provisioningExecutor) handleExistingUser(ctx *providers.NodeContext, userID string,
	execResp *providers.ExecutorResponse, logger *log.Logger) (bool, error) {
	logger.Debug(ctx.Context, "User already exists", log.MaskedString(log.LoggerKeyUserID, userID))

	if !isCrossOUProvisioningAllowed(ctx) {
		logger.Debug(ctx.Context, "Cross OU provisioning is not allowed")
		if ctx.FlowType == providers.FlowTypeAuthentication {
			p.handleNonProvisionableUserInAuthentication(ctx, execResp)
			return false, nil
		}
		p.handleNonProvisionableUserInRegistration(ctx, execResp, &ErrUserAlreadyExists)
		return false, nil
	}

	// Cross-OU provisioning is allowed.
	ref, err := p.getTargetEntityRef(ctx)
	if err != nil {
		return false, err
	}
	targetOUID := ref.ouID
	if targetOUID == "" {
		logger.Debug(ctx.Context, "Target OU for cross-OU provisioning is not set")
		// Cross-OU provisioning is not intended.
		if ctx.FlowType == providers.FlowTypeAuthentication {
			p.handleNonProvisionableUserInAuthentication(ctx, execResp)
			return false, nil
		}
		p.handleNonProvisionableUserInRegistration(ctx, execResp, &ErrCrossOUProvisioningTargetMissing)
		return false, nil
	}

	existingUser, getUserErr := p.entityProvider.GetEntity(userID)
	if getUserErr != nil {
		return false, errors.New("failed to retrieve existing user")
	}

	if existingUser.OUID == targetOUID {
		logger.Debug(ctx.Context, "Existing user is in the target OU")
		// Cross-OU provisioning is not intended.
		if ctx.FlowType == providers.FlowTypeAuthentication {
			p.handleNonProvisionableUserInAuthentication(ctx, execResp)
			return false, nil
		}
		p.handleNonProvisionableUserInRegistration(ctx, execResp, &ErrUserAlreadyExistsInTargetOU)
		return false, nil
	}

	logger.Debug(ctx.Context, "Existing user is in a different OU, proceeding with cross-OU provisioning",
		log.String("existingOUID", existingUser.OUID),
		log.String("targetOUID", targetOUID))
	return true, nil
}

// resolveAmbiguousUserForProvisioning is called when IdentifyUser reports ambiguity and cross-OU
// provisioning is allowed. It searches for all matching users and returns the ID of the one in the
// target OU, or nil if none exists there.
func (p *provisioningExecutor) resolveAmbiguousUserForProvisioning(ctx *providers.NodeContext,
	identifyingAttrs map[string]interface{}) (*string, error) {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	matches, searchErr := p.entityProvider.SearchEntities(identifyingAttrs)
	if searchErr != nil {
		return nil, fmt.Errorf("failed to search for matching users: code=%s, description=%s",
			searchErr.Code, searchErr.Description)
	}

	entityRef, err := p.getTargetEntityRef(ctx)
	if err != nil {
		return nil, err
	}
	targetOUID := entityRef.ouID
	for _, m := range matches {
		if m == nil || m.OUID == "" {
			return nil, fmt.Errorf("ambiguous user search returned an entity with missing OUID")
		}
		if m.OUID == targetOUID {
			logger.Debug(ctx.Context, "Ambiguous user has a match in the target OU",
				log.MaskedString(log.LoggerKeyUserID, m.ID))
			return &m.ID, nil
		}
	}

	logger.Debug(ctx.Context, "Ambiguous user has no match in target OU",
		log.Int("matchCount", len(matches)))
	return nil, nil
}

// HasRequiredInputs checks whether all schema-driven provisioning inputs are satisfied and appends
// any missing promptable schema attrs to the executor response. Node inputs influence requiredness
// and prompt metadata for schema attrs, but schema-absent node inputs are ignored.
//
// Missing inputs are ordered as: required non-credentials -> optional non-credentials ->
// required credentials -> optional credentials. maxPerPrompt caps the forwarded
// prompt batch after this list is built. includeOptional only affects optional
// non-credential attrs.
func (p *provisioningExecutor) HasRequiredInputs(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) bool {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Checking inputs for the provisioning executor")

	if execResp.RuntimeData == nil {
		execResp.RuntimeData = make(map[string]string)
	}

	// Build a lookup map of node-defined inputs for the required/optional override rule:
	// node can upgrade optional → required, but cannot lower schema-required to optional.
	nodeInputMap := make(map[string]providers.Input, len(ctx.NodeInputs))
	for _, inp := range ctx.NodeInputs {
		nodeInputMap[inp.Identifier] = inp
	}

	// Fetch all schema attributes (credential and non-credential) in a single call.
	allSchemaAttrs, err := p.fetchSchemaAttributes(ctx, true, true)
	if err != nil {
		logger.Warn(ctx.Context, "Failed to fetch schema attributes for provisioning", log.Any("error", err))
		execResp.Status = providers.ExecFailure
		return false
	}
	if len(allSchemaAttrs) == 0 {
		return true
	}

	credRequiredMissing, credOptionalMissing, ncRequiredMissing, ncOptionalMissing :=
		p.buildMissingInputs(ctx, allSchemaAttrs, nodeInputMap)

	// Build the full schema missing list: required non-creds first, then optional non-creds,
	// followed by required creds, then optional creds.
	// Node-defined inputs not present in the schema are ignored — provisioning is schema-driven
	// and can only store attributes defined by the entity type.
	allSchemaMissing := make([]providers.Input, 0,
		len(ncRequiredMissing)+len(credRequiredMissing)+len(ncOptionalMissing)+len(credOptionalMissing))
	allSchemaMissing = append(allSchemaMissing, ncRequiredMissing...)
	allSchemaMissing = append(allSchemaMissing, ncOptionalMissing...)
	allSchemaMissing = append(allSchemaMissing, credRequiredMissing...)
	allSchemaMissing = append(allSchemaMissing, credOptionalMissing...)

	if len(allSchemaMissing) == 0 {
		return true
	}

	// Apply maxPerPrompt to the forwarded prompt batch.
	toForward := allSchemaMissing
	if maxInputs := p.getMaxDynamicInputs(ctx); maxInputs > 0 && len(toForward) > maxInputs {
		toForward = toForward[:maxInputs]
	}

	execResp.Inputs = allSchemaMissing
	if execResp.ForwardedData == nil {
		execResp.ForwardedData = make(map[string]interface{})
	}
	execResp.ForwardedData[common.ForwardedDataKeyInputs] = toForward
	logger.Debug(ctx.Context, "Schema attributes are missing, requesting via prompt",
		log.Int("missingCount", len(allSchemaMissing)))
	return false
}

// buildMissingInputs categorizes all schema attributes into four missing-input buckets in a single
// pass. attr.Credential drives the input type (password vs text) and optional-inclusion rules.
func (p *provisioningExecutor) buildMissingInputs(
	ctx *providers.NodeContext,
	schemaAttrs []entitytype.AttributeInfo,
	nodeInputMap map[string]providers.Input,
) (credRequired, credOptional, ncRequired, ncOptional []providers.Input) {
	promptOptional := p.isPromptOptionalAttributesEnabled(ctx)
	promptOptionalCredentials := p.isPromptOptionalCredentialsEnabled(ctx)
	presentedOptionalInputs := core.GetPresentedOptionalInputs(ctx.RuntimeData)

	for _, attr := range schemaAttrs {
		if p.isAttrSatisfied(ctx, attr.Attribute) {
			continue
		}
		nodeInp, inNodeInputs := nodeInputMap[attr.Attribute]
		effectiveRequired := attr.Required
		if inNodeInputs {
			effectiveRequired = attr.Required || nodeInp.Required
		}

		if attr.Credential {
			if !effectiveRequired && !promptOptionalCredentials && !inNodeInputs {
				continue
			}
			if !effectiveRequired && core.IsOptionalInputPrompted(presentedOptionalInputs, attr.Attribute) {
				continue
			}
			input := providers.Input{
				Identifier:  attr.Attribute,
				Type:        providers.InputTypePassword,
				Required:    effectiveRequired,
				DisplayName: attr.DisplayName,
			}
			if effectiveRequired {
				credRequired = append(credRequired, input)
			} else {
				credOptional = append(credOptional, input)
			}
		} else {
			if !attr.Required && !promptOptional && !inNodeInputs {
				continue
			}
			if !effectiveRequired && core.IsOptionalInputPrompted(presentedOptionalInputs, attr.Attribute) {
				continue
			}
			input := providers.Input{
				Identifier:  attr.Attribute,
				Type:        providers.InputTypeText,
				DisplayName: attr.DisplayName,
			}
			if inNodeInputs {
				input = nodeInp
				input.Identifier = attr.Attribute
				if input.Type == "" {
					input.Type = providers.InputTypeText
				}
				if input.DisplayName == "" {
					input.DisplayName = attr.DisplayName
				}
			}
			input.Required = effectiveRequired
			if effectiveRequired {
				ncRequired = append(ncRequired, input)
			} else {
				ncOptional = append(ncOptional, input)
			}
		}
	}
	return credRequired, credOptional, ncRequired, ncOptional
}

// fetchSchemaAttributes retrieves schema attributes from the entity type service for the
// current user type. allowCredential and allowNonCredential control which attribute classes
// are returned.
func (p *provisioningExecutor) fetchSchemaAttributes(
	ctx *providers.NodeContext, allowCredential, allowNonCredential bool,
) ([]entitytype.AttributeInfo, error) {
	if p.entityTypeService == nil {
		return nil, nil
	}
	entityRef, err := p.getTargetEntityRef(ctx)
	if err != nil {
		return nil, err
	}
	userType := entityRef.entityType
	if userType == "" {
		return nil, fmt.Errorf("user type not found")
	}
	attrs, svcErr := p.entityTypeService.GetAttributes(ctx.Context,
		entitytype.TypeCategoryUser, userType, allowCredential, allowNonCredential, false)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to fetch schema attributes for user type %q: %s",
			userType, svcErr.Error.DefaultValue)
	}
	return attrs, nil
}

// isPromptOptionalAttributesEnabled reads the includeOptional node property.
// Returns false when the property is absent, preserving the default behavior of prompting only required attributes.
func (p *provisioningExecutor) isPromptOptionalAttributesEnabled(ctx *providers.NodeContext) bool {
	if val, ok := ctx.NodeProperties[propertyKeyDynamicInputsIncludeOptional]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return false
}

// isPromptOptionalCredentialsEnabled reads the includeOptionalCredentials node property.
// Returns false when the property is absent. Only the required credentials are prompted by default.
func (p *provisioningExecutor) isPromptOptionalCredentialsEnabled(ctx *providers.NodeContext) bool {
	if val, ok := ctx.NodeProperties[propertyKeyDynamicInputsIncludeOptionalCredentials]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return false
}

// getMaxDynamicInputs reads the maxPerPrompt node property.
// Returns 0 when absent, meaning all missing inputs are prompted at once (current default behavior).
func (p *provisioningExecutor) getMaxDynamicInputs(ctx *providers.NodeContext) int {
	if val, ok := ctx.NodeProperties[propertyKeyMaxDynamicInputsPerPrompt]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		}
	}
	return 0
}

// isAttrSatisfied returns true if the attribute has a non-empty usable value.
// Credential attrs are satisfied only by UserInputs or RuntimeData.
// Non-credential attrs also fall back to AuthenticatedUser.Attributes.
func (p *provisioningExecutor) isAttrSatisfied(ctx *providers.NodeContext, attr string) bool {
	if val, ok := ctx.UserInputs[attr]; ok && val != "" {
		return true
	}
	if val, ok := ctx.RuntimeData[attr]; ok && val != "" {
		return true
	}
	return false
}

// getAttributesForProvisioning collects user attributes from context in a single schema pass,
// returning identifying (non-credential) and credential attributes as separate maps.
// Schema is the whitelist for both maps.
// Credential values are resolved from non-empty UserInputs then non-empty RuntimeData only.
// Non-credential values additionally fall back to AuthenticatedUser.Attributes.
func (p *provisioningExecutor) getAttributesForProvisioning(
	ctx *providers.NodeContext,
) (identifyingAttrs map[string]interface{}, credentialAttrs map[string]interface{}, err error) {
	schemaAttrs, fetchErr := p.fetchSchemaAttributes(ctx, true, true)
	if fetchErr != nil {
		return nil, nil, fetchErr
	}

	identifyingAttrs = make(map[string]interface{})
	credentialAttrs = make(map[string]interface{})

	if len(schemaAttrs) == 0 {
		return identifyingAttrs, credentialAttrs, nil
	}

	for _, a := range schemaAttrs {
		if a.Credential {
			if value, exists := ctx.UserInputs[a.Attribute]; exists && value != "" {
				credentialAttrs[a.Attribute] = value
			} else if runtimeValue, exists := ctx.RuntimeData[a.Attribute]; exists && runtimeValue != "" {
				credentialAttrs[a.Attribute] = runtimeValue
			}
		} else {
			if value, exists := ctx.UserInputs[a.Attribute]; exists && value != "" {
				identifyingAttrs[a.Attribute] = value
			} else if runtimeValue, exists := ctx.RuntimeData[a.Attribute]; exists && runtimeValue != "" {
				identifyingAttrs[a.Attribute] = runtimeValue
			}
		}
	}

	return identifyingAttrs, credentialAttrs, nil
}

// createUserInStore creates a new user in the user store with the provided attributes.
func (p *provisioningExecutor) createUserInStore(nodeCtx *providers.NodeContext,
	userAttributes map[string]interface{}) (*providers.Entity, error) {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, nodeCtx.ExecutionID))
	logger.Debug(nodeCtx.Context, "Creating the user account")

	entityRef, err := p.getTargetEntityRef(nodeCtx)
	if err != nil {
		return nil, err
	}
	ouID := entityRef.ouID
	if ouID == "" {
		return nil, fmt.Errorf("organization unit ID not found")
	}
	userType := entityRef.entityType
	if userType == "" {
		return nil, fmt.Errorf("user type not found")
	}

	newEntity := providers.Entity{
		Category: providers.EntityCategoryUser,
		State:    providers.EntityStateActive,
		OUID:     ouID,
		Type:     userType,
	}

	attributesJSON, err := json.Marshal(userAttributes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user attributes: %w", err)
	}
	newEntity.Attributes = attributesJSON

	retEntity, svcErr := p.entityProvider.CreateEntity(&newEntity, nil)
	if svcErr != nil {
		return nil, svcErr
	}
	if retEntity != nil && retEntity.ID != "" {
		logger.Debug(nodeCtx.Context, "User account created successfully",
			log.MaskedString(log.LoggerKeyUserID, retEntity.ID))
	}

	return retEntity, nil
}

// handleCreateUserError maps an entity provider error during user creation to the appropriate ServiceError.
func (p *provisioningExecutor) handleCreateUserError(
	ctx *providers.NodeContext,
	err error,
	logger *log.Logger,
) *tidcommon.ServiceError {
	var epErr *entityprovider.EntityProviderError
	if errors.As(err, &epErr) {
		if epErr.Code == entityprovider.ErrorCodeAttributeConflict {
			return &ErrProvisioningAttributeConflict
		}
		logger.Error(ctx.Context, "Failed to create user in the store",
			log.String("errorCode", string(epErr.Code)), log.String("message", epErr.Message))
		return &ErrProvisioningFailed
	}
	logger.Error(ctx.Context, "Failed to create user in the store", log.Error(err))
	return &ErrProvisioningFailed
}

// getTargetEntityRef retrieves the target entity reference (user type and OU ID) for provisioning.
func (p *provisioningExecutor) getTargetEntityRef(ctx *providers.NodeContext) (*entityRef, error) {
	ouID := p.getOUID(ctx)
	userType := p.getUserType(ctx)

	if ouID == "" || userType == "" {
		defaultEntityRef, err := p.getDefaultEntityRef(ctx)
		if err != nil {
			return nil, err
		}
		if defaultEntityRef != nil {
			if ouID == "" {
				ouID = defaultEntityRef.ouID
			}
			if userType == "" {
				userType = defaultEntityRef.entityType
			}
		}
	}

	return &entityRef{
		entityType: userType,
		ouID:       ouID,
	}, nil
}

// getOUID retrieves the organization unit ID from runtime data.
// Priority: RuntimeData["ouId"] (set by OUResolverExecutor) > RuntimeData["defaultOUID"] (set by UserTypeResolver).
func (p *provisioningExecutor) getOUID(ctx *providers.NodeContext) string {
	// Check for ouId in runtime data (e.g. from OUResolverExecutor).
	if val, ok := ctx.RuntimeData[ouIDKey]; ok && val != "" {
		return val
	}
	// Fallback: check for defaultOUID in runtime data (set by UserTypeResolver).
	if val, ok := ctx.RuntimeData[defaultOUIDKey]; ok && val != "" {
		return val
	}

	return ""
}

// getUserType retrieves the user type from runtime data.
func (p *provisioningExecutor) getUserType(ctx *providers.NodeContext) string {
	userType := ""
	if val, ok := ctx.RuntimeData[userTypeKey]; ok && val != "" {
		userType = val
	}

	return userType
}

// assignGroupsAndRoles assigns the newly created user to configured groups and roles.
// If no group or role is configured, the assignments are skipped.
func (p *provisioningExecutor) assignGroupsAndRoles(
	ctx *providers.NodeContext,
	userID string,
) error {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	groupIDs := p.getGroupsToAssign(ctx)
	roleIDs := p.getRolesToAssign(ctx)

	if len(groupIDs) == 0 && len(roleIDs) == 0 {
		logger.Debug(ctx.Context, "No group or role configured for assignment, skipping")
		return nil
	}

	logger.Debug(ctx.Context, "Assigning groups and roles to provisioned user",
		log.MaskedString(log.LoggerKeyUserID, userID),
		log.String("groupIDs", strings.Join(groupIDs, ",")),
		log.String("roleIDs", strings.Join(roleIDs, ",")))

	if len(groupIDs) > 0 {
		if svcErr := p.groupService.AddMembersToGroups(
			ctx.Context, []group.Member{{ID: userID, Type: group.MemberTypeUser}}, groupIDs); svcErr != nil {
			return fmt.Errorf("group assignment failed: %s", svcErr.Error.DefaultValue)
		}
	}

	if len(roleIDs) > 0 {
		if svcErr := p.roleAssignmentService.AddAssigneesToRoles(
			ctx.Context, []role.RoleAssignment{{ID: userID, Type: role.AssigneeTypeUser}}, roleIDs); svcErr != nil {
			return fmt.Errorf("role assignment failed: %s", svcErr.Error.DefaultValue)
		}
	}

	logger.Debug(ctx.Context, "Successfully assigned groups and roles",
		log.MaskedString(log.LoggerKeyUserID, userID))
	return nil
}

// getGroupsToAssign parses the assignGroup node property into a slice of group IDs.
// The property value is a comma-separated string; a single ID produces a one-element slice.
func (p *provisioningExecutor) getGroupsToAssign(ctx *providers.NodeContext) []string {
	if len(ctx.NodeProperties) == 0 {
		return nil
	}
	val, ok := ctx.NodeProperties[propertyKeyAssignGroup]
	if !ok {
		return nil
	}
	strVal, ok := val.(string)
	if !ok {
		return nil
	}
	return splitTrimmed(strVal)
}

// getRolesToAssign parses the assignRole node property into a slice of role IDs.
// The property value is a comma-separated string; a single ID produces a one-element slice.
func (p *provisioningExecutor) getRolesToAssign(ctx *providers.NodeContext) []string {
	if len(ctx.NodeProperties) == 0 {
		return nil
	}
	val, ok := ctx.NodeProperties[propertyKeyAssignRole]
	if !ok {
		return nil
	}
	strVal, ok := val.(string)
	if !ok {
		return nil
	}
	return splitTrimmed(strVal)
}

// splitTrimmed splits s by commas and trims whitespace from each element, discarding empty entries.
func splitTrimmed(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
}

// getDefaultEntityRef resolves the user type for auto provisioning in authentication flows.
func (p *provisioningExecutor) getDefaultEntityRef(ctx *providers.NodeContext) (*entityRef, error) {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Resolving user type for automatic provisioning")

	if len(ctx.Application.AllowedUserTypes) == 0 {
		logger.Debug(ctx.Context, "No allowed user types configured for the application")
		return nil, nil
	}

	// Filter allowed user types to only those with self-registration enabled
	selfRegEnabledSchemas := make([]entitytype.EntityType, 0)
	for _, userType := range ctx.Application.AllowedUserTypes {
		entityType, svcErr := p.entityTypeService.GetEntityTypeByName(ctx.Context,
			entitytype.TypeCategoryUser, userType)
		if svcErr != nil {
			return nil, fmt.Errorf("failed to retrieve entity type for user type %q: %s",
				userType, svcErr.Error.DefaultValue)
		}
		if entityType.AllowSelfRegistration {
			selfRegEnabledSchemas = append(selfRegEnabledSchemas, *entityType)
		}
	}

	// Fail if no user types have self-registration enabled
	if len(selfRegEnabledSchemas) == 0 {
		logger.Debug(ctx.Context, "No user types with self-registration enabled, cannot provision automatically")
		return nil, nil
	}

	// Fail if multiple user types have self-registration enabled
	if len(selfRegEnabledSchemas) > 1 {
		logger.Debug(ctx.Context,
			"Multiple user types with self-registration enabled, cannot resolve user type automatically")
		return nil, nil
	}

	return &entityRef{
		entityType: selfRegEnabledSchemas[0].Name,
		ouID:       selfRegEnabledSchemas[0].OUID,
	}, nil
}
