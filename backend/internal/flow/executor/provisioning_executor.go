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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	authnprovidermgr "github.com/asgardeo/thunder/internal/authnprovider/manager"
	"github.com/asgardeo/thunder/internal/entityprovider"
	"github.com/asgardeo/thunder/internal/entitytype"
	"github.com/asgardeo/thunder/internal/flow/common"
	"github.com/asgardeo/thunder/internal/flow/core"
	"github.com/asgardeo/thunder/internal/group"
	"github.com/asgardeo/thunder/internal/role"
	"github.com/asgardeo/thunder/internal/system/log"
)

// provisioningExecutor implements the ExecutorInterface for user provisioning in a flow.
type provisioningExecutor struct {
	core.ExecutorInterface
	identifyingExecutorInterface
	authnProvider     authnprovidermgr.AuthnProviderManagerInterface
	entityProvider    entityprovider.EntityProviderInterface
	groupService      group.GroupServiceInterface
	roleService       role.RoleServiceInterface
	entityTypeService entitytype.EntityTypeServiceInterface
	logger            *log.Logger
}

var _ core.ExecutorInterface = (*provisioningExecutor)(nil)
var _ identifyingExecutorInterface = (*provisioningExecutor)(nil)

// newProvisioningExecutor creates a new instance of ProvisioningExecutor.
func newProvisioningExecutor(
	flowFactory core.FlowFactoryInterface,
	groupService group.GroupServiceInterface,
	roleService role.RoleServiceInterface,
	entityProvider entityprovider.EntityProviderInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
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
		entityTypeService:            entityTypeService,
		authnProvider:                authnProvider,
		logger:                       logger,
	}
}

// Execute executes the user provisioning logic based on the inputs provided.
func (p *provisioningExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Executing user provisioning executor")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	// If it's an authentication flow, skip execution if the user is not eligible for provisioning
	if ctx.FlowType == common.FlowTypeAuthentication {
		eligible, ok := ctx.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning]
		if !ok || eligible != dataValueTrue {
			logger.Debug("User is not eligible for provisioning, skipping execution")
			execResp.Status = common.ExecComplete
			return execResp, nil
		}
	}

	// If it's a registration flow, check if proceeding with an existing user
	if ctx.FlowType == common.FlowTypeRegistration {
		shouldSkip, ok := ctx.RuntimeData[common.RuntimeKeySkipProvisioning]
		if ok && shouldSkip == dataValueTrue {
			existingUserID := ctx.AuthUser.GetUserID()
			if existingUserID == "" {
				logger.Error("Skip provisioning flag is set but no existing user found in context")
				execResp.Status = common.ExecFailure
				execResp.FailureReason = "no existing user found"
				return execResp, nil
			}
			logger.Debug("Proceeding with an existing user in registration flow, skipping execution")
			execResp.RuntimeData[userAttributeUserID] = existingUserID
			execResp.Status = common.ExecComplete
			return execResp, nil
		}
	}

	if !p.HasRequiredInputs(ctx, execResp) {
		if execResp.Status == common.ExecFailure {
			return execResp, nil
		}

		logger.Debug("Required inputs for provisioning executor is not provided")
		execResp.Status = common.ExecUserInputRequired
		return execResp, nil
	}

	userAttributes, err := p.getAttributesForProvisioning(ctx)
	if err != nil {
		return nil, err
	}
	if len(userAttributes) == 0 {
		logger.Debug("No user attributes provided for provisioning")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "No user attributes provided for provisioning"
		return execResp, nil
	}

	userID, err := p.IdentifyUser(userAttributes, execResp)
	if err != nil {
		logger.Error("Failed to identify user", log.Error(err))
		execResp.Status = common.ExecFailure
		execResp.FailureReason = failureReasonFailedToIdentifyUser
		return execResp, nil
	}
	if execResp.Status == common.ExecFailure && execResp.FailureReason != failureReasonUserNotFound {
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

	// Create the user in the store.
	if err := p.appendCredentialAttributes(ctx, &userAttributes); err != nil {
		return nil, err
	}
	createdEntity, err := p.createUserInStore(ctx, userAttributes)
	if err != nil {
		logger.Error("Failed to create user in the store", log.Error(err))
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Failed to create user"
		return execResp, nil
	}
	if createdEntity == nil || createdEntity.ID == "" {
		logger.Error("Created user is nil or has no ID")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Something went wrong while creating the user"
		return execResp, nil
	}

	logger.Debug("User created successfully", log.MaskedString(log.LoggerKeyUserID, createdEntity.ID))

	// Assign user to groups and roles
	if err := p.assignGroupsAndRoles(ctx, createdEntity.ID); err != nil {
		logger.Error("Failed to assign groups and roles to provisioned user",
			log.MaskedString(log.LoggerKeyUserID, createdEntity.ID),
			log.Error(err))
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Failed to assign groups and roles"
		return execResp, nil
	}

	authUser, svcErr := p.authnProvider.AuthenticateResolvedUser(ctx.Context, createdEntity, ctx.AuthUser)
	if svcErr != nil {
		logger.Error("Failed to authenticate provisioned user")
		return nil, fmt.Errorf("failed to authenticate provisioned user")
	}

	execResp.AuthUser = authUser
	execResp.Status = common.ExecComplete

	// Set user id in runtime data
	execResp.RuntimeData[userAttributeUserID] = createdEntity.ID

	// Set the auto-provisioned flag if it's a user auto provisioning scenario
	if ctx.FlowType == common.FlowTypeAuthentication {
		execResp.RuntimeData[common.RuntimeKeyUserAutoProvisioned] = dataValueTrue
	}

	return execResp, nil
}

// handleExistingUser handles the case where a user with the given ID already exists.
// Returns true if provisioning should proceed (cross-OU case), false if execution should stop.
func (p *provisioningExecutor) handleExistingUser(ctx *core.NodeContext, userID string,
	execResp *common.ExecutorResponse, logger *log.Logger) (bool, error) {
	logger.Debug("User already exists", log.MaskedString(log.LoggerKeyUserID, userID))

	// If it's a registration flow, check if proceeding with an existing user
	if ctx.FlowType == common.FlowTypeRegistration {
		existing, ok := ctx.RuntimeData[common.RuntimeKeySkipProvisioning]
		if ok && existing == dataValueTrue {
			logger.Debug("Proceeding with an existing user in registration flow, skipping execution")
			execResp.RuntimeData[userAttributeUserID] = userID
			execResp.Status = common.ExecComplete
			return false, nil
		}
	}

	// Check if cross-OU provisioning is explicitly enabled for this node.
	allowCrossOUProvisioning := false
	if val, ok := ctx.NodeProperties[common.NodePropertyAllowCrossOUProvisioning]; ok {
		if boolVal, ok := val.(bool); ok {
			allowCrossOUProvisioning = boolVal
		}
	}

	if !allowCrossOUProvisioning {
		if ctx.FlowType == common.FlowTypeRegistration {
			execResp.Status = common.ExecUserInputRequired
			execResp.Inputs = p.GetRequiredInputs(ctx)
		} else {
			execResp.Status = common.ExecFailure
		}
		execResp.FailureReason = "User already exists"
		return false, nil
	}

	// Cross-OU provisioning: verify the existing user is in a different OU than the target.
	targetOUID := p.getOUID(ctx)
	if targetOUID == "" {
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Target OU is not set for cross-OU provisioning"
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
		execResp.FailureReason = "User already exists in the target organization"
		return false, nil
	}

	logger.Debug("Existing user is in a different OU, proceeding with cross-OU provisioning",
		log.String("existingOUID", existingUser.OUID),
		log.String("targetOUID", targetOUID))
	return true, nil
}

// HasRequiredInputs checks if the required inputs are provided in the context and appends any
// missing inputs to the executor response. Returns true if all required inputs — both
// node-defined and schema-derived — are satisfied, otherwise false.
func (p *provisioningExecutor) HasRequiredInputs(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) bool {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Checking inputs for the provisioning executor")

	if execResp.RuntimeData == nil {
		execResp.RuntimeData = make(map[string]string)
	}

	// run the base executor check for node-defined inputs.
	nodeInputsSatisfied := p.checkNodeInputs(ctx, execResp, logger)

	// fetch required non-credential attributes from the user type.
	schemaAttrs, err := p.fetchSchemaAttributes(ctx, logger)
	if err != nil {
		execResp.Status = common.ExecFailure
		return false
	}
	if len(schemaAttrs) == 0 {
		return nodeInputsSatisfied
	}

	// Load the set of optional attrs already prompted in previous iterations so they are not
	// re-prompted even when the user left them empty.
	alreadyPromptedOptionalAttrs := p.getPresentedOptionalAttrs(ctx)

	// Build required and optional missing lists separately so required ones are always shown first.
	requiredMissing := make([]common.Input, 0, len(schemaAttrs))
	optionalMissing := make([]common.Input, 0, len(schemaAttrs))
	for _, attr := range schemaAttrs {
		if p.isAttrSatisfied(ctx, attr.Attribute) {
			continue
		}
		if !attr.Required && alreadyPromptedOptionalAttrs[attr.Attribute] {
			continue
		}
		input := common.Input{
			Identifier:  attr.Attribute,
			Type:        common.InputTypeText,
			Required:    attr.Required,
			DisplayName: attr.DisplayName,
		}
		if attr.Required {
			requiredMissing = append(requiredMissing, input)
		} else {
			optionalMissing = append(optionalMissing, input)
		}
	}

	schemaMissingAttrs := make([]common.Input, 0, len(requiredMissing)+len(optionalMissing))
	schemaMissingAttrs = append(schemaMissingAttrs, requiredMissing...)
	schemaMissingAttrs = append(schemaMissingAttrs, optionalMissing...)
	if len(schemaMissingAttrs) == 0 {
		return nodeInputsSatisfied
	}

	if maxInputs := p.getMaxDynamicInputs(ctx); maxInputs > 0 && len(schemaMissingAttrs) > maxInputs {
		schemaMissingAttrs = schemaMissingAttrs[:maxInputs]
	}

	// Record which optional attrs are being presented in this iteration so future iterations
	// know not to re-prompt them even if the user left the value empty.
	p.storePresentedOptionalAttrs(execResp, schemaMissingAttrs, alreadyPromptedOptionalAttrs)

	execResp.Inputs = upsertInputs(execResp.Inputs, schemaMissingAttrs)
	if execResp.ForwardedData == nil {
		execResp.ForwardedData = make(map[string]interface{})
	}
	execResp.ForwardedData[common.ForwardedDataKeyInputs] = schemaMissingAttrs
	logger.Debug("Schema attributes are missing, requesting via prompt",
		log.Int("missingCount", len(schemaMissingAttrs)))
	return false
}

// checkNodeInputs runs the base executor's input check, then clears any inputs satisfied by authenticated user attrs.
func (p *provisioningExecutor) checkNodeInputs(ctx *core.NodeContext,
	execResp *common.ExecutorResponse, logger *log.Logger) bool {
	nodeInputsSatisfied := p.ExecutorInterface.HasRequiredInputs(ctx, execResp)
	if nodeInputsSatisfied || len(execResp.Inputs) == 0 {
		return nodeInputsSatisfied
	}

	authnAttrs := ctx.AuthUser.GetRuntimeAttributes()
	if len(authnAttrs) == 0 {
		return false
	}

	logger.Debug("Authenticated user attributes found, checking missing node inputs")
	remaining := execResp.Inputs[:0]
	for _, input := range execResp.Inputs {
		val, exists := authnAttrs[input.Identifier]
		strVal, isStr := val.(string)
		if exists && isStr && strVal != "" {
			continue
		}
		remaining = append(remaining, input)
	}
	execResp.Inputs = remaining
	return len(remaining) == 0
}

// fetchSchemaAttributes retrieves non-credential attributes from the entity type service.
// When promptOptionalAttributes is true it fetches all attributes; otherwise only required ones.
func (p *provisioningExecutor) fetchSchemaAttributes(
	ctx *core.NodeContext, logger *log.Logger,
) ([]entitytype.AttributeInfo, error) {
	if p.entityTypeService == nil {
		return nil, nil
	}
	userType := p.getUserType(ctx)
	if userType == "" {
		return nil, fmt.Errorf("user type not found")
	}
	requiredOnly := !p.isPromptOptionalAttributesEnabled(ctx)
	attrs, svcErr := p.entityTypeService.GetNonCredentialAttributes(ctx.Context,
		entitytype.TypeCategoryUser, userType, requiredOnly)
	if svcErr != nil {
		logger.Warn("Failed to fetch schema attributes for provisioning, skipping schema check",
			log.Any("error", svcErr))
		return nil, nil
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

// getPresentedOptionalAttrs returns the set of optional attr identifiers that have already been
// prompted to the user in previous iterations, loaded from RuntimeData.
func (p *provisioningExecutor) getPresentedOptionalAttrs(ctx *core.NodeContext) map[string]bool {
	result := make(map[string]bool)
	raw, ok := ctx.RuntimeData[common.RuntimeKeyPresentedOptionalAttrs]
	if !ok || raw == "" {
		return result
	}
	for _, id := range strings.Split(raw, " ") {
		if id != "" {
			result[id] = true
		}
	}
	return result
}

// storePresentedOptionalAttrs accumulates the optional attrs being shown in this iteration into
// RuntimeData so the next iteration can skip them.
func (p *provisioningExecutor) storePresentedOptionalAttrs(
	execResp *common.ExecutorResponse,
	toPrompt []common.Input,
	alreadyPresented map[string]bool,
) {
	for _, inp := range toPrompt {
		if !inp.Required {
			alreadyPresented[inp.Identifier] = true
		}
	}
	if len(alreadyPresented) == 0 {
		return
	}
	ids := make([]string, 0, len(alreadyPresented))
	for id := range alreadyPresented {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	execResp.RuntimeData[common.RuntimeKeyPresentedOptionalAttrs] = strings.Join(ids, " ")
}

// isAttrSatisfied returns true if the attribute has a non-empty usable value in any context source.
func (p *provisioningExecutor) isAttrSatisfied(ctx *core.NodeContext, attr string) bool {
	authUserAttributes := ctx.AuthUser.GetRuntimeAttributes()
	if val, ok := ctx.UserInputs[attr]; ok && val != "" {
		return true
	}
	if val, ok := ctx.RuntimeData[attr]; ok && val != "" {
		return true
	}
	if val, ok := authUserAttributes[attr]; ok {
		if strVal, ok := val.(string); ok && strVal != "" {
			return true
		}
	}
	return false
}

// getAttributesForProvisioning returns the user profile attributes to store.
// Schema is the whitelist: required attrs always collected, optional attrs only if in node inputs
// or if promptOptionalAttributes is enabled.
func (p *provisioningExecutor) getAttributesForProvisioning(ctx *core.NodeContext) (map[string]interface{}, error) {
	nodeInputAttrs := p.GetRequiredInputs(ctx)
	schemaAttrs, err := p.fetchAllNonCredentialAttributes(ctx)
	if err != nil {
		return nil, err
	}
	if len(schemaAttrs) == 0 {
		return make(map[string]interface{}), nil
	}
	nodeInputSet := make(map[string]struct{}, len(nodeInputAttrs))
	for _, inp := range nodeInputAttrs {
		nodeInputSet[inp.Identifier] = struct{}{}
	}
	promptOptional := p.isPromptOptionalAttributesEnabled(ctx)

	authUserAttributes := ctx.AuthUser.GetRuntimeAttributes()

	attributesMap := make(map[string]interface{})
	for _, a := range schemaAttrs {
		_, inNodeInputs := nodeInputSet[a.Attribute]
		if len(nodeInputSet) > 0 && !a.Required && !inNodeInputs && !promptOptional {
			continue
		}

		if value, exists := ctx.UserInputs[a.Attribute]; exists && value != "" {
			attributesMap[a.Attribute] = value
		} else if runtimeValue, exists := ctx.RuntimeData[a.Attribute]; exists && runtimeValue != "" {
			attributesMap[a.Attribute] = runtimeValue
		} else if authnValue, exists := authUserAttributes[a.Attribute]; exists {
			if strVal, ok := authnValue.(string); ok && strVal != "" {
				attributesMap[a.Attribute] = authnValue
			}
		}
	}

	return attributesMap, nil
}

// fetchAllNonCredentialAttributes retrieves all non-credential schema attributes with their required status.
func (p *provisioningExecutor) fetchAllNonCredentialAttributes(
	ctx *core.NodeContext,
) ([]entitytype.AttributeInfo, error) {
	if p.entityTypeService == nil {
		return nil, nil
	}
	userType := p.getUserType(ctx)
	if userType == "" {
		return nil, fmt.Errorf("user type not found")
	}
	attrs, svcErr := p.entityTypeService.GetNonCredentialAttributes(ctx.Context,
		entitytype.TypeCategoryUser, userType, false)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to fetch schema attributes for user type %q: %s",
			userType, svcErr.Error.DefaultValue)
	}
	return attrs, nil
}

// appendCredentialAttributes appends credential attributes defined in the user type to the provided
// attributes map. If the node declares specific credential inputs, only those are collected; otherwise
// all schema credential attributes are collected. Values are resolved from UserInputs then RuntimeData.
func (p *provisioningExecutor) appendCredentialAttributes(ctx *core.NodeContext,
	attributes *map[string]interface{}) error {
	schemaCredAttrs, err := p.fetchCredentialAttributes(ctx)
	if err != nil {
		return err
	}
	if len(schemaCredAttrs) == 0 {
		return nil
	}

	credentialAttrSet := make(map[string]struct{}, len(schemaCredAttrs))
	for _, attr := range schemaCredAttrs {
		credentialAttrSet[attr] = struct{}{}
	}

	var nodeCredentialInputs []string
	for _, input := range ctx.NodeInputs {
		if _, ok := credentialAttrSet[input.Identifier]; ok {
			nodeCredentialInputs = append(nodeCredentialInputs, input.Identifier)
		}
	}

	attrsToPopulate := schemaCredAttrs
	if len(nodeCredentialInputs) > 0 {
		attrsToPopulate = nodeCredentialInputs
	}

	for _, attr := range attrsToPopulate {
		if value, exists := ctx.UserInputs[attr]; exists {
			(*attributes)[attr] = value
		} else if runtimeValue, exists := ctx.RuntimeData[attr]; exists {
			(*attributes)[attr] = runtimeValue
		}
	}

	return nil
}

// fetchCredentialAttributes retrieves credential attribute names from the entity type service.
func (p *provisioningExecutor) fetchCredentialAttributes(ctx *core.NodeContext) ([]string, error) {
	if p.entityTypeService == nil {
		return nil, nil
	}
	userType := p.getUserType(ctx)
	if userType == "" {
		return nil, fmt.Errorf("user type not found")
	}
	attrs, svcErr := p.entityTypeService.GetCredentialAttributes(ctx.Context, entitytype.TypeCategoryUser, userType)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to fetch credential attributes for user type %q: %s",
			userType, svcErr.Error.DefaultValue)
	}
	return attrs, nil
}

// createUserInStore creates a new user in the user store with the provided attributes.
func (p *provisioningExecutor) createUserInStore(nodeCtx *core.NodeContext,
	userAttributes map[string]interface{}) (*entityprovider.Entity, error) {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, nodeCtx.ExecutionID))
	logger.Debug("Creating the user account")

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
		logger.Debug("User account created successfully", log.MaskedString(log.LoggerKeyUserID, retEntity.ID))
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
		logger.Debug("No group or role configured for assignment, skipping")
		return nil
	}

	logger.Debug("Assigning group and role to provisioned user",
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

	logger.Debug("Successfully assigned group and role", log.MaskedString(log.LoggerKeyUserID, userID))
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
	logger.Debug("Adding user to group",
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
		logger.Error("Failed to add user to group",
			log.String("groupID", groupID),
			log.MaskedString(log.LoggerKeyUserID, userID),
			log.String("error", svcErr.Error.DefaultValue))
		return fmt.Errorf("failed to add user to group: %s", svcErr.Error.DefaultValue)
	}

	logger.Debug("Successfully added user to group",
		log.MaskedString(log.LoggerKeyUserID, userID),
		log.String("groupID", groupID))
	return nil
}

// assignToRole adds the user to the specified role.
func (p *provisioningExecutor) assignToRole(
	ctx context.Context, userID string, roleID string, logger *log.Logger) error {
	logger.Debug("Adding user to role",
		log.MaskedString(log.LoggerKeyUserID, userID),
		log.String("roleID", roleID))

	// AddAssignments appends to existing assignments (doesn't replace)
	assignments := []role.RoleAssignment{
		{
			ID:   userID,
			Type: role.AssigneeTypeUser,
		},
	}

	svcErr := p.roleService.AddAssignments(ctx, roleID, assignments)
	if svcErr != nil {
		logger.Error("Failed to add role assignment",
			log.String("roleID", roleID),
			log.MaskedString(log.LoggerKeyUserID, userID),
			log.String("error", svcErr.Error.DefaultValue))
		return fmt.Errorf("failed to assign role: %s", svcErr.Error.DefaultValue)
	}

	logger.Debug("Successfully assigned role",
		log.MaskedString(log.LoggerKeyUserID, userID),
		log.String("roleID", roleID))
	return nil
}
