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
	"context"
	"encoding/json"
	"errors"
	"fmt"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// provisioningExecutor implements the ExecutorInterface for user provisioning in a flow.
type provisioningExecutor struct {
	core.ExecutorInterface
	identifyingExecutorInterface
	entityProvider        entityprovider.EntityProviderInterface
	groupService          group.GroupServiceInterface
	roleService           role.RoleServiceInterface
	roleAssignmentService role.RoleAssignmentServiceInterface
	entityTypeService     entitytype.EntityTypeServiceInterface
	logger                *log.Logger
}

var _ core.ExecutorInterface = (*provisioningExecutor)(nil)
var _ identifyingExecutorInterface = (*provisioningExecutor)(nil)

// newProvisioningExecutor creates a new instance of ProvisioningExecutor.
func newProvisioningExecutor(
	flowFactory core.FlowFactoryInterface,
	groupService group.GroupServiceInterface,
	roleService role.RoleServiceInterface,
	roleAssignmentService role.RoleAssignmentServiceInterface,
	entityProvider entityprovider.EntityProviderInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
) *provisioningExecutor {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, ExecutorNameProvisioning),
		log.String(log.LoggerKeyExecutorName, ExecutorNameProvisioning))

	base := flowFactory.CreateExecutor(ExecutorNameProvisioning, common.ExecutorTypeRegistration,
		[]common.Input{}, []common.Input{})

	identifyingExec := newIdentifyingExecutor(ExecutorNameProvisioning,
		[]common.Input{}, []common.Input{}, flowFactory, entityProvider)

	return &provisioningExecutor{
		ExecutorInterface:            base,
		identifyingExecutorInterface: identifyingExec,
		entityProvider:               entityProvider,
		groupService:                 groupService,
		roleService:                  roleService,
		roleAssignmentService:        roleAssignmentService,
		entityTypeService:            entityTypeService,
		logger:                       logger,
	}
}

// Execute executes the user provisioning logic based on the inputs provided.
func (p *provisioningExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing user provisioning executor")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	// If it's an authentication flow, skip execution if the user is not eligible for provisioning
	if ctx.FlowType == common.FlowTypeAuthentication {
		eligible, ok := ctx.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning]
		if !ok || eligible != dataValueTrue {
			logger.Debug(ctx.Context, "User is not eligible for provisioning, skipping execution")
			execResp.Status = common.ExecComplete
			return execResp, nil
		}
	}

	if !p.HasRequiredInputs(ctx, execResp) {
		if execResp.Status == common.ExecFailure {
			return execResp, nil
		}

		logger.Debug(ctx.Context, "Required inputs for provisioning executor is not provided")
		execResp.Status = common.ExecUserInputRequired
		return execResp, nil
	}

	identifyingAttrs, credentialAttrs, err := p.getAttributesForProvisioning(ctx)
	if err != nil {
		return nil, err
	}
	if len(identifyingAttrs) == 0 && len(credentialAttrs) == 0 {
		logger.Debug(ctx.Context, "No user attributes provided for provisioning")
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrProvisioningUserAttrsMissing
		return execResp, nil
	}

	userID, err := p.IdentifyUser(ctx.Context, identifyingAttrs, execResp)
	if err != nil {
		logger.Error(ctx.Context, "Failed to identify user", log.Error(err))
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrFailedToIdentifyUser
		return execResp, nil
	}
	if execResp.Status == common.ExecFailure &&
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
	if execResp.Status == common.ExecFailure && (execResp.Error == nil || execResp.Error.Code != ErrUserNotFound.Code) {
		return execResp, nil
	}
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
		logger.Error(ctx.Context, "Failed to create user in the store", log.Error(err))
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrProvisioningFailed
		return execResp, nil
	}
	if createdEntity == nil || createdEntity.ID == "" {
		logger.Error(ctx.Context, "Created user is nil or has no ID")
		execResp.Status = common.ExecFailure
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
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrProvisioningAssignmentFailed
		return execResp, nil
	}

	if err := p.buildProvisioningResponse(ctx, createdEntity, execResp, logger); err != nil {
		return nil, err
	}

	return execResp, nil
}

// handleExistingUser handles the case where a user with the given ID already exists.
// Returns true if provisioning should proceed (cross-OU case), false if execution should stop.
func (p *provisioningExecutor) handleExistingUser(ctx *core.NodeContext, userID string,
	execResp *common.ExecutorResponse, logger *log.Logger) (bool, error) {
	logger.Debug(ctx.Context, "User already exists", log.MaskedString(log.LoggerKeyUserID, userID))

	// If it's a registration flow, check if proceeding with an existing user
	if ctx.FlowType == common.FlowTypeRegistration {
		existing, ok := ctx.RuntimeData[common.RuntimeKeySkipProvisioning]
		if ok && existing == dataValueTrue {
			logger.Debug(ctx.Context,
				"Proceeding with an existing user in registration flow, skipping execution")
			execResp.RuntimeData[userAttributeUserID] = userID
			execResp.Status = common.ExecComplete
			return false, nil
		}
	}

	// Check if cross-OU provisioning is explicitly enabled for this node
	if !isCrossOUProvisioningAllowed(ctx) {
		if ctx.FlowType == common.FlowTypeRegistration {
			execResp.Status = common.ExecUserInputRequired
			execResp.Inputs = p.GetRequiredInputs(ctx)
		} else {
			execResp.Status = common.ExecFailure
		}
		execResp.Error = &ErrUserAlreadyExists
		return false, nil
	}

	// Cross-OU provisioning: verify the existing user is in a different OU than the target.
	targetOUID := p.getOUID(ctx)
	if targetOUID == "" {
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrCrossOUProvisioningTargetMissing
		return false, nil
	}

	existingUser, getUserErr := p.entityProvider.GetEntity(userID)
	if getUserErr != nil {
		return false, errors.New("failed to retrieve existing user")
	}

	if existingUser.OUID == targetOUID {
		if ctx.FlowType == common.FlowTypeRegistration {
			execResp.Status = common.ExecUserInputRequired
			execResp.Inputs = p.GetRequiredInputs(ctx)
		} else {
			execResp.Status = common.ExecFailure
		}
		execResp.Error = &ErrUserAlreadyExistsInTargetOU
		return false, nil
	}

	logger.Debug(ctx.Context, "Existing user is in a different OU, proceeding with cross-OU provisioning",
		log.String("existingOUID", existingUser.OUID),
		log.String("targetOUID", targetOUID))
	return true, nil
}

// buildProvisioningResponse populates execResp after successful user creation.
func (p *provisioningExecutor) buildProvisioningResponse(ctx *core.NodeContext, createdEntity *entityprovider.Entity,
	execResp *common.ExecutorResponse, logger *log.Logger) error {
	retAttributes := make(map[string]interface{})
	if len(createdEntity.Attributes) > 0 {
		if err := json.Unmarshal(createdEntity.Attributes, &retAttributes); err != nil {
			logger.Error(ctx.Context, "Failed to unmarshal user attributes", log.Error(err))
			return err
		}
	}

	execResp.AuthenticatedUser = authncm.AuthenticatedUser{
		IsAuthenticated: true,
		UserID:          createdEntity.ID,
		OUID:            createdEntity.OUID,
		UserType:        createdEntity.Type,
		Attributes:      retAttributes,
	}
	execResp.Status = common.ExecComplete
	execResp.RuntimeData[userAttributeUserID] = createdEntity.ID

	if ctx.FlowType == common.FlowTypeAuthentication {
		execResp.RuntimeData[common.RuntimeKeyUserAutoProvisioned] = dataValueTrue
	}
	return nil
}

// resolveAmbiguousUserForProvisioning is called when IdentifyUser reports ambiguity and cross-OU
// provisioning is allowed. It searches for all matching users and returns the ID of the one in the
// target OU, or nil if none exists there.
func (p *provisioningExecutor) resolveAmbiguousUserForProvisioning(ctx *core.NodeContext,
	identifyingAttrs map[string]interface{}) (*string, error) {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	matches, searchErr := p.entityProvider.SearchEntities(identifyingAttrs)
	if searchErr != nil {
		return nil, fmt.Errorf("failed to search for matching users: code=%s, description=%s",
			searchErr.Code, searchErr.Description)
	}

	targetOUID := p.getOUID(ctx)
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
func (p *provisioningExecutor) HasRequiredInputs(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) bool {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Checking inputs for the provisioning executor")

	if execResp.RuntimeData == nil {
		execResp.RuntimeData = make(map[string]string)
	}

	// Build a lookup map of node-defined inputs for the required/optional override rule:
	// node can upgrade optional → required, but cannot lower schema-required to optional.
	nodeInputMap := make(map[string]common.Input, len(ctx.NodeInputs))
	for _, inp := range ctx.NodeInputs {
		nodeInputMap[inp.Identifier] = inp
	}

	// Fetch all schema attributes (credential and non-credential) in a single call.
	allSchemaAttrs, err := p.fetchSchemaAttributes(ctx, true, true)
	if err != nil {
		logger.Warn(ctx.Context, "Failed to fetch schema attributes for provisioning", log.Any("error", err))
		execResp.Status = common.ExecFailure
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
	allSchemaMissing := make([]common.Input, 0,
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
	ctx *core.NodeContext,
	schemaAttrs []entitytype.AttributeInfo,
	nodeInputMap map[string]common.Input,
) (credRequired, credOptional, ncRequired, ncOptional []common.Input) {
	promptOptional := p.isPromptOptionalAttributesEnabled(ctx)
	promptOptionalCredentials := p.isPromptOptionalCredentialsEnabled(ctx)
	presentedOptionalInputs := core.GetPresentedOptionalInputs(ctx.RuntimeData)

	for _, attr := range schemaAttrs {
		if p.isAttrSatisfied(ctx, attr.Attribute, attr.Credential) {
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
			input := common.Input{
				Identifier:  attr.Attribute,
				Type:        common.InputTypePassword,
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
			input := common.Input{
				Identifier:  attr.Attribute,
				Type:        common.InputTypeText,
				DisplayName: attr.DisplayName,
			}
			if inNodeInputs {
				input = nodeInp
				input.Identifier = attr.Attribute
				if input.Type == "" {
					input.Type = common.InputTypeText
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
	ctx *core.NodeContext, allowCredential, allowNonCredential bool,
) ([]entitytype.AttributeInfo, error) {
	if p.entityTypeService == nil {
		return nil, nil
	}
	userType := p.getUserType(ctx)
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
func (p *provisioningExecutor) isPromptOptionalAttributesEnabled(ctx *core.NodeContext) bool {
	if val, ok := ctx.NodeProperties[propertyKeyDynamicInputsIncludeOptional]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return false
}

// isPromptOptionalCredentialsEnabled reads the includeOptionalCredentials node property.
// Returns false when the property is absent. Only the required credentials are prompted by default.
func (p *provisioningExecutor) isPromptOptionalCredentialsEnabled(ctx *core.NodeContext) bool {
	if val, ok := ctx.NodeProperties[propertyKeyDynamicInputsIncludeOptionalCredentials]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return false
}

// getMaxDynamicInputs reads the maxPerPrompt node property.
// Returns 0 when absent, meaning all missing inputs are prompted at once (current default behavior).
func (p *provisioningExecutor) getMaxDynamicInputs(ctx *core.NodeContext) int {
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
func (p *provisioningExecutor) isAttrSatisfied(ctx *core.NodeContext, attr string, credential bool) bool {
	if val, ok := ctx.UserInputs[attr]; ok && val != "" {
		return true
	}
	if val, ok := ctx.RuntimeData[attr]; ok && val != "" {
		return true
	}
	if !credential {
		if val, ok := ctx.AuthenticatedUser.Attributes[attr]; ok {
			if strVal, ok := val.(string); ok && strVal != "" {
				return true
			}
		}
	}
	return false
}

// getAttributesForProvisioning collects user attributes from context in a single schema pass,
// returning identifying (non-credential) and credential attributes as separate maps.
// Schema is the whitelist for both maps.
// Credential values are resolved from non-empty UserInputs then non-empty RuntimeData only.
// Non-credential values additionally fall back to AuthenticatedUser.Attributes.
func (p *provisioningExecutor) getAttributesForProvisioning(
	ctx *core.NodeContext,
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
			} else if authnValue, exists := ctx.AuthenticatedUser.Attributes[a.Attribute]; exists {
				if strVal, ok := authnValue.(string); ok && strVal != "" {
					identifyingAttrs[a.Attribute] = authnValue
				}
			}
		}
	}

	return identifyingAttrs, credentialAttrs, nil
}

// createUserInStore creates a new user in the user store with the provided attributes.
func (p *provisioningExecutor) createUserInStore(nodeCtx *core.NodeContext,
	userAttributes map[string]interface{}) (*entityprovider.Entity, error) {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, nodeCtx.ExecutionID))
	logger.Debug(nodeCtx.Context, "Creating the user account")

	ouID := p.getOUID(nodeCtx)
	if ouID == "" {
		return nil, fmt.Errorf("organization unit ID not found")
	}
	userType := p.getUserType(nodeCtx)
	if userType == "" {
		return nil, fmt.Errorf("user type not found")
	}

	newEntity := entityprovider.Entity{
		Category: entityprovider.EntityCategoryUser,
		State:    entityprovider.EntityStateActive,
		OUID:     ouID,
		Type:     userType,
	}

	// Convert the user attributes to JSON.
	attributesJSON, err := json.Marshal(userAttributes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user attributes: %w", err)
	}
	newEntity.Attributes = attributesJSON

	retEntity, svcErr := p.entityProvider.CreateEntity(&newEntity, nil)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to create user in the store: %s", svcErr.Message)
	}
	if retEntity != nil && retEntity.ID != "" {
		logger.Debug(nodeCtx.Context, "User account created successfully",
			log.MaskedString(log.LoggerKeyUserID, retEntity.ID))
	}

	return retEntity, nil
}

// getOUID retrieves the organization unit ID from runtime data.
// Priority: RuntimeData["ouId"] (set by OUResolverExecutor) > RuntimeData["defaultOUID"] (set by UserTypeResolver).
func (p *provisioningExecutor) getOUID(ctx *core.NodeContext) string {
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
func (p *provisioningExecutor) getUserType(ctx *core.NodeContext) string {
	userType := ""
	if val, ok := ctx.RuntimeData[userTypeKey]; ok && val != "" {
		userType = val
	}

	return userType
}

// assignGroupsAndRoles assigns the newly created user to configured group and role.
// If no group or role is configured, the assignments are skipped.
func (p *provisioningExecutor) assignGroupsAndRoles(
	ctx *core.NodeContext,
	userID string,
) error {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	// Get configured group and role from properties
	groupID := p.getGroupToAssign(ctx)
	roleID := p.getRoleToAssign(ctx)

	// Skip if no group or role configured
	if groupID == "" && roleID == "" {
		logger.Debug(ctx.Context, "No group or role configured for assignment, skipping")
		return nil
	}

	logger.Debug(ctx.Context, "Assigning group and role to provisioned user",
		log.MaskedString(log.LoggerKeyUserID, userID),
		log.String("groupID", groupID),
		log.String("roleID", roleID))

	var groupErr, roleErr error
	// Assign to group
	if groupID != "" {
		if err := p.assignToGroup(ctx.Context, userID, groupID, logger); err != nil {
			groupErr = fmt.Errorf("failed to assign user to group %s: %w", groupID, err)
		}
	}
	// Assign to role
	if roleID != "" {
		if err := p.assignToRole(ctx.Context, userID, roleID, logger); err != nil {
			roleErr = fmt.Errorf("failed to assign user to role %s: %w", roleID, err)
		}
	}
	if groupErr != nil || roleErr != nil {
		if groupErr != nil && roleErr != nil {
			return fmt.Errorf("group assignment error: %w; role assignment error: %s", groupErr, roleErr.Error())
		}
		if groupErr != nil {
			return groupErr
		}
		return roleErr
	}

	logger.Debug(ctx.Context, "Successfully assigned group and role",
		log.MaskedString(log.LoggerKeyUserID, userID))
	return nil
}

// getGroupToAssign retrieves the group ID from node properties.
func (p *provisioningExecutor) getGroupToAssign(ctx *core.NodeContext) string {
	if len(ctx.NodeProperties) == 0 {
		return ""
	}

	groupValue, ok := ctx.NodeProperties[propertyKeyAssignGroup]
	if !ok {
		return ""
	}

	// Handle string value
	if strVal, ok := groupValue.(string); ok {
		return strVal
	}

	return ""
}

// getRoleToAssign retrieves the role ID from node properties.
func (p *provisioningExecutor) getRoleToAssign(ctx *core.NodeContext) string {
	if len(ctx.NodeProperties) == 0 {
		return ""
	}

	roleValue, ok := ctx.NodeProperties[propertyKeyAssignRole]
	if !ok {
		return ""
	}

	// Handle string value
	if strVal, ok := roleValue.(string); ok {
		return strVal
	}

	return ""
}

// assignToGroup adds the user to the specified group.
func (p *provisioningExecutor) assignToGroup(
	ctx context.Context,
	userID string,
	groupID string,
	logger *log.Logger,
) error {
	logger.Debug(ctx, "Adding user to group",
		log.MaskedString(log.LoggerKeyUserID, userID),
		log.String("groupID", groupID))

	members := []group.Member{
		{
			ID:   userID,
			Type: group.MemberTypeUser,
		},
	}

	_, svcErr := p.groupService.AddGroupMembers(ctx, groupID, members)
	if svcErr != nil {
		logger.Error(ctx, "Failed to add user to group",
			log.String("groupID", groupID),
			log.MaskedString(log.LoggerKeyUserID, userID),
			log.String("error", svcErr.Error.DefaultValue))
		return fmt.Errorf("failed to add user to group: %s", svcErr.Error.DefaultValue)
	}

	logger.Debug(ctx, "Successfully added user to group",
		log.MaskedString(log.LoggerKeyUserID, userID),
		log.String("groupID", groupID))
	return nil
}

// assignToRole adds the user to the specified role.
func (p *provisioningExecutor) assignToRole(
	ctx context.Context, userID string, roleID string, logger *log.Logger) error {
	if p.roleAssignmentService == nil {
		logger.Error(ctx, "Role assignment service is not configured",
			log.String("roleID", roleID),
			log.MaskedString(log.LoggerKeyUserID, userID))
		return fmt.Errorf("role assignment service not configured")
	}

	logger.Debug(ctx, "Adding user to role",
		log.MaskedString(log.LoggerKeyUserID, userID),
		log.String("roleID", roleID))

	// AddAssignments appends to existing assignments (doesn't replace)
	assignments := []role.RoleAssignment{
		{
			ID:   userID,
			Type: role.AssigneeTypeUser,
		},
	}

	svcErr := p.roleAssignmentService.AddAssignments(ctx, roleID, assignments)
	if svcErr != nil {
		logger.Error(ctx, "Failed to add role assignment",
			log.String("roleID", roleID),
			log.MaskedString(log.LoggerKeyUserID, userID),
			log.String("error", svcErr.Error.DefaultValue))
		return fmt.Errorf("failed to assign role: %s", svcErr.Error.DefaultValue)
	}

	logger.Debug(ctx, "Successfully assigned role",
		log.MaskedString(log.LoggerKeyUserID, userID),
		log.String("roleID", roleID))
	return nil
}
