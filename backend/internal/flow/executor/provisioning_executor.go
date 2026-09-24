// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// entityRef locates where an entity is provisioned: its entity type within the category, and the
// organization unit that owns it.
type entityRef struct {
	entityType string
	ouID       string
}

// provisioningExecutor implements the ExecutorInterface for entity provisioning in a flow.
//
// The node's mode property names the entity category to provision. Only the create step is bound
// to that category; the steps around it take it as an argument.
type provisioningExecutor struct {
	providers.Executor
	identifyingExecutorInterface
	entityProvider        entityprovider.EntityProviderInterface
	userMgtProvider       providers.UserMgtProvider
	agentMgtProvider      providers.AgentMgtProvider
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
	userMgtProvider providers.UserMgtProvider,
	agentMgtProvider providers.AgentMgtProvider,
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
				providers.FlowTypeAdministration,
			},
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: propertyKeyProvisioningMode},
				{Property: propertyKeyDynamicInputsIncludeOptional},
				{Property: propertyKeyDynamicInputsIncludeOptionalCredentials},
				{Property: propertyKeyMaxDynamicInputsPerPrompt},
				{Property: propertyKeyAssignGroup},
				{Property: propertyKeyAssignRole},
				{Property: propertyKeySeedGroupsFromMapping},
				{Property: propertyKeySeedRolesFromMapping},
				{Property: common.NodePropertyAllowCrossOUProvisioning},
			},
		})

	identifyingExec := newIdentifyingExecutor(ExecutorNameProvisioning,
		[]providers.Input{}, []providers.Input{}, flowFactory, entityProvider)

	return &provisioningExecutor{
		Executor:                     base,
		identifyingExecutorInterface: identifyingExec,
		entityProvider:               entityProvider,
		userMgtProvider:              userMgtProvider,
		agentMgtProvider:             agentMgtProvider,
		groupService:                 groupService,
		roleService:                  roleService,
		roleAssignmentService:        roleAssignmentService,
		entityTypeService:            entityTypeService,
		authnProvider:                authnProvider,
		logger:                       logger,
	}
}

// Execute provisions an entity of the node's category. It runs the category-independent
// preparation, hands the collected attributes to the category's create step, and finishes with the
// category-independent work that follows a create.
func (p *provisioningExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing provisioning executor")

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		AuthUser:       ctx.AuthUser,
	}

	category, err := categoryFromMode(ctx)
	if err != nil {
		return execResp, err
	}

	// Agents do not self-register yet. The capability is not blocked anywhere structural, so
	// lifting this is a matter of deleting the guard once the rules for it are settled.
	if category == entitytype.TypeCategoryAgent && ctx.FlowType == providers.FlowTypeRegistration {
		logger.Debug(ctx.Context, "Agent provisioning is not available in a registration flow")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrSelfRegNotSupportedForAgents

		return execResp, nil
	}

	// Authentication only auto-provisions the categories it can authenticate, and only when it
	// marked the entity eligible.
	if ctx.FlowType == providers.FlowTypeAuthentication && traitsFor(category).autoProvisionedAtAuthentication {
		eligible, ok := ctx.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning]
		if !ok || eligible != dataValueTrue {
			logger.Debug(ctx.Context, "Entity is not eligible for auto provisioning, skipping execution")
			execResp.Status = providers.ExecComplete
			return execResp, nil
		}
	}

	attributes, proceed, err := p.prepareProvisioning(ctx, category, execResp, logger)
	if err != nil {
		return nil, err
	}
	if !proceed {
		return execResp, nil
	}

	entityID, svcErr := p.createEntity(ctx, category, attributes, execResp, logger)
	if svcErr != nil {
		execResp.Status = providers.ExecFailure
		execResp.Error = svcErr
		return execResp, nil
	}

	return p.completeProvisioning(ctx, category, entityID, execResp, logger)
}

// categoryFromMode maps the node's mode property onto the entity category it works on. A mode is
// the category name, and a node that sets none works on the default category.
func categoryFromMode(ctx *providers.NodeContext) (entitytype.TypeCategory, error) {
	mode, ok := ctx.NodeProperties[propertyKeyProvisioningMode].(string)
	if !ok || mode == "" {
		return defaultProvisioningCategory, nil
	}
	category := entitytype.TypeCategory(mode)
	if !category.IsValid() {
		return "", fmt.Errorf("invalid provisioning mode: %s", mode)
	}
	return category, nil
}

// prepareProvisioning runs everything that happens before an entity exists: input collection,
// attribute resolution against the entity type schema, and identification of an entity that
// already matches. It returns the attributes to create the entity with. A false proceed means
// execResp already carries the outcome of the node.
func (p *provisioningExecutor) prepareProvisioning(ctx *providers.NodeContext,
	category entitytype.TypeCategory, execResp *providers.ExecutorResponse, logger *log.Logger,
) (map[string]interface{}, bool, error) {
	if !p.hasRequiredInputs(ctx, category, execResp) {
		if execResp.Status == providers.ExecFailure {
			return nil, false, nil
		}

		logger.Debug(ctx.Context, "Required inputs for provisioning executor is not provided")
		execResp.Status = providers.ExecUserInputRequired
		return nil, false, nil
	}

	identifyingAttrs, credentialAttrs, err := p.getAttributesForProvisioning(ctx, category)
	if err != nil {
		return nil, false, err
	}
	if len(identifyingAttrs) == 0 && len(credentialAttrs) == 0 &&
		!hasRecordValues(ctx, category) {
		logger.Debug(ctx.Context, "No entity attributes provided for provisioning")
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrProvisioningAttrsMissing, category)
		return nil, false, nil
	}

	proceed, err := p.resolveExistingEntity(ctx, category, identifyingAttrs, execResp, logger)
	if err != nil || !proceed {
		return nil, false, err
	}

	// Merge identifying and credential attributes for entity creation
	attributes := make(map[string]interface{}, len(identifyingAttrs)+len(credentialAttrs))
	for k, v := range identifyingAttrs {
		attributes[k] = v
	}
	for k, v := range credentialAttrs {
		attributes[k] = v
	}
	return attributes, true, nil
}

// resolveExistingEntity identifies an entity that already matches the provisioning attributes and
// decides whether provisioning can still go ahead. A false return means execResp already carries
// the outcome of the node.
//
// With no attributes to match on there is nothing to identify by, which is reachable whenever a
// schema declares every attribute optional and none is supplied. Identification is skipped rather
// than attempted, since an empty filter matches on nothing rather than on everything.
func (p *provisioningExecutor) resolveExistingEntity(ctx *providers.NodeContext,
	category entitytype.TypeCategory, identifyingAttrs map[string]interface{},
	execResp *providers.ExecutorResponse, logger *log.Logger,
) (bool, error) {
	if len(identifyingAttrs) == 0 {
		logger.Debug(ctx.Context, "No identifying attributes provided, skipping identification")
		return true, nil
	}

	entityID, err := p.IdentifyEntity(ctx.Context, identifyingAttrs, execResp, category)
	if err != nil {
		logger.Error(ctx.Context, "Failed to identify the entity", log.Error(err))
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrFailedToIdentifyEntity, category)
		return false, nil
	}
	if execResp.Status == providers.ExecFailure &&
		execResp.Error != nil && execResp.Error.Code == ErrAmbiguousEntityIdentity.Code &&
		isCrossOUProvisioningAllowed(ctx) {
		resolved, resolveErr := p.resolveAmbiguousEntityForProvisioning(ctx, category, identifyingAttrs)
		if resolveErr != nil {
			return false, resolveErr
		}
		entityID = resolved
		execResp.Status = ""
		execResp.Error = nil
	}
	if execResp.Status == providers.ExecFailure &&
		(execResp.Error == nil || execResp.Error.Code != ErrEntityNotFound.Code) {
		return false, nil
	}
	// clear execResp set by IdentifyEntity
	execResp.Status = ""
	execResp.Error = nil
	if entityID != nil && *entityID != "" {
		return p.handleExistingEntity(ctx, category, *entityID, execResp, logger)
	}

	return true, nil
}

// createEntity provisions the entity through the management service its category owns, and returns
// the identifier the store assigned.
func (p *provisioningExecutor) createEntity(ctx *providers.NodeContext, category entitytype.TypeCategory,
	attributes map[string]interface{}, execResp *providers.ExecutorResponse,
	logger *log.Logger) (string, *tidcommon.ServiceError) {
	var entityID string

	switch category {
	case entitytype.TypeCategoryUser:
		createdUser, svcErr := p.createUserInStore(ctx, attributes)
		if svcErr != nil {
			return "", p.handleCreateEntityError(ctx, category, svcErr, logger)
		}
		if createdUser != nil {
			entityID = createdUser.ID
		}
	case entitytype.TypeCategoryAgent:
		createdAgent, svcErr := p.createAgentInStore(ctx, attributes)
		if svcErr != nil {
			return "", p.handleCreateEntityError(ctx, category, svcErr, logger)
		}
		if createdAgent != nil {
			entityID = createdAgent.ID
			addAgentDataToResponse(execResp, createdAgent)
		}
	default:
		logger.Error(ctx.Context, "Provisioning is not supported for the entity category",
			log.String("category", string(category)))
		return "", errForEntityCategory(ErrProvisioningFailed, category)
	}

	if entityID == "" {
		logger.Error(ctx.Context, "Provisioned entity has no identifier")
		return "", errForEntityCategory(ErrProvisioningFailed, category)
	}

	logger.Debug(ctx.Context, "Entity created successfully",
		log.MaskedString(log.LoggerKeyEntityID, entityID))
	return entityID, nil
}

// completeProvisioning runs the steps that follow a create: group and role assignment,
// authentication as the provisioned entity, and the runtime flags the rest of the flow reads.
func (p *provisioningExecutor) completeProvisioning(ctx *providers.NodeContext,
	category entitytype.TypeCategory, entityID string, execResp *providers.ExecutorResponse,
	logger *log.Logger) (*providers.ExecutorResponse, error) {
	if err := p.assignGroupsAndRoles(ctx, category, entityID); err != nil {
		logger.Error(ctx.Context, "Failed to assign groups and roles to the provisioned entity",
			log.MaskedString(log.LoggerKeyEntityID, entityID),
			log.Error(err))
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrProvisioningAssignmentFailed, category)
		return execResp, nil
	}

	p.authenticateProvisionedEntity(ctx, category, entityID, execResp)
	if execResp.Status == providers.ExecFailure {
		return execResp, nil
	}

	execResp.Status = providers.ExecComplete

	// Record the auto-provisioning so later nodes can tell it from an ordinary sign-in.
	if ctx.FlowType == providers.FlowTypeAuthentication && traitsFor(category).autoProvisionedAtAuthentication {
		execResp.RuntimeData[common.RuntimeKeyUserAutoProvisioned] = dataValueTrue
	}

	return execResp, nil
}

// authenticateProvisionedEntity authenticates the newly provisioned entity and updates the
// executor response.
func (p *provisioningExecutor) authenticateProvisionedEntity(ctx *providers.NodeContext,
	category entitytype.TypeCategory, entityID string, execResp *providers.ExecutorResponse) {
	credential := map[string]interface{}{
		authnprovidercm.CredentialTypeProvisionedEntityID: entityID,
	}
	authUser, authenticatedClaims, err := p.authnProvider.AuthenticateUser(ctx.Context, nil, credential,
		nil, nil, execResp.AuthUser)
	if !authUser.IsAuthenticated() || err != nil {
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrEntityAuthFailed, category)
		return
	}
	execResp.AuthUser = authUser
	for key, value := range authenticatedClaims {
		execResp.RuntimeData[key] = systemutils.ConvertInterfaceValueToString(value)
	}
}

// handleNonProvisionableEntityInAuthentication sets the exec response when an existing entity is
// found during an authentication flow and provisioning cannot proceed.
// Provisioning is simply skipped and the flow continues with the existing entity.
func (p *provisioningExecutor) handleNonProvisionableEntityInAuthentication(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) {
	p.logger.Debug(ctx.Context, "Skipping provisioning and continuing with the existing entity")
	execResp.Status = providers.ExecComplete
}

// handleNonProvisionableEntityInRegistration sets the exec response when an existing entity is
// found during a registration or onboarding flow and provisioning cannot proceed.
// It either allows the flow to skip provisioning, prompts for different input, or fails immediately.
func (p *provisioningExecutor) handleNonProvisionableEntityInRegistration(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse, existsErr *tidcommon.ServiceError) {
	if isAllowRegistrationWithExistingUserRuntimeFlagSet(ctx) {
		execResp.Status = providers.ExecComplete
		return
	}
	requiredInputs := p.GetRequiredInputs(ctx)
	if len(requiredInputs) > 0 {
		// Existing entity identified based on user input attributes.
		// Allow the user to input different attributes for registration.
		execResp.Status = providers.ExecUserInputRequired
		execResp.Inputs = requiredInputs
		execResp.Error = existsErr
		return
	}
	// Existing entity identified without user input attributes.
	// The user cannot recover from the error by changing input, so fail immediately.
	execResp.Status = providers.ExecFailure
	execResp.Error = existsErr
}

// handleExistingEntity handles the case where an entity with the given ID already exists.
// Returns true if provisioning should proceed (cross-OU case), false if execution should stop.
func (p *provisioningExecutor) handleExistingEntity(ctx *providers.NodeContext,
	category entitytype.TypeCategory, entityID string, execResp *providers.ExecutorResponse,
	logger *log.Logger) (bool, error) {
	logger.Debug(ctx.Context, "Entity already exists", log.MaskedString(log.LoggerKeyEntityID, entityID))

	if !isCrossOUProvisioningAllowed(ctx) {
		logger.Debug(ctx.Context, "Cross OU provisioning is not allowed")
		if ctx.FlowType == providers.FlowTypeAuthentication {
			p.handleNonProvisionableEntityInAuthentication(ctx, execResp)
			return false, nil
		}
		p.handleNonProvisionableEntityInRegistration(ctx, execResp,
			errForEntityCategory(ErrEntityAlreadyExists, category))
		return false, nil
	}

	// Cross-OU provisioning is allowed.
	ref, err := p.getTargetEntityRef(ctx, category)
	if err != nil {
		return false, err
	}
	targetOUID := ref.ouID
	if targetOUID == "" {
		logger.Debug(ctx.Context, "Target OU for cross-OU provisioning is not set")
		// Cross-OU provisioning is not intended.
		if ctx.FlowType == providers.FlowTypeAuthentication {
			p.handleNonProvisionableEntityInAuthentication(ctx, execResp)
			return false, nil
		}
		p.handleNonProvisionableEntityInRegistration(ctx, execResp,
			errForEntityCategory(ErrCrossOUProvisioningTargetMissing, category))
		return false, nil
	}

	existingEntity, getEntityErr := p.entityProvider.GetEntity(entityID)
	if getEntityErr != nil {
		return false, errors.New("failed to retrieve the existing entity")
	}

	if existingEntity.OUID == targetOUID {
		logger.Debug(ctx.Context, "Existing entity is in the target OU")
		// Cross-OU provisioning is not intended.
		if ctx.FlowType == providers.FlowTypeAuthentication {
			p.handleNonProvisionableEntityInAuthentication(ctx, execResp)
			return false, nil
		}
		p.handleNonProvisionableEntityInRegistration(ctx, execResp,
			errForEntityCategory(ErrEntityAlreadyExistsInTargetOU, category))
		return false, nil
	}

	logger.Debug(ctx.Context, "Existing entity is in a different OU, proceeding with cross-OU provisioning",
		log.String("existingOUID", existingEntity.OUID),
		log.String("targetOUID", targetOUID))
	return true, nil
}

// resolveAmbiguousEntityForProvisioning is called when IdentifyEntity reports ambiguity and cross-OU
// provisioning is allowed. It searches for all matching entities and returns the ID of the one in
// the target OU, or nil if none exists there.
func (p *provisioningExecutor) resolveAmbiguousEntityForProvisioning(ctx *providers.NodeContext,
	category entitytype.TypeCategory, identifyingAttrs map[string]interface{}) (*string, error) {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	matches, searchErr := p.entityProvider.SearchEntities(identifyingAttrs)
	if searchErr != nil {
		return nil, fmt.Errorf("failed to search for matching entities: code=%s, description=%s",
			searchErr.Code, searchErr.Description)
	}

	targetRef, err := p.getTargetEntityRef(ctx, category)
	if err != nil {
		return nil, err
	}
	targetOUID := targetRef.ouID
	for _, m := range matches {
		if m == nil || m.OUID == "" {
			return nil, fmt.Errorf("ambiguous entity search returned an entity with missing OUID")
		}
		if m.OUID == targetOUID {
			logger.Debug(ctx.Context, "Ambiguous entity has a match in the target OU",
				log.MaskedString(log.LoggerKeyEntityID, m.ID))
			return &m.ID, nil
		}
	}

	logger.Debug(ctx.Context, "Ambiguous entity has no match in target OU",
		log.Int("matchCount", len(matches)))
	return nil, nil
}

// HasRequiredInputs satisfies the executor interface, which carries no category. Execute resolves
// the category once and calls hasRequiredInputs with it directly.
func (p *provisioningExecutor) HasRequiredInputs(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) bool {
	category, err := categoryFromMode(ctx)
	if err != nil {
		p.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID)).
			Warn(ctx.Context, "Failed to resolve the provisioning category", log.Any("error", err))
		execResp.Status = providers.ExecFailure
		return false
	}
	return p.hasRequiredInputs(ctx, category, execResp)
}

// hasRequiredInputs checks whether all schema-driven provisioning inputs are satisfied and appends
// any missing promptable schema attrs to the executor response. Node inputs influence requiredness
// and prompt metadata for schema attrs, but schema-absent node inputs are ignored.
//
// Missing inputs are ordered as: required non-credentials -> optional non-credentials ->
// required credentials -> optional credentials. maxPerPrompt caps the forwarded
// prompt batch after this list is built. includeOptional only affects optional
// non-credential attrs.
func (p *provisioningExecutor) hasRequiredInputs(ctx *providers.NodeContext,
	category entitytype.TypeCategory, execResp *providers.ExecutorResponse) bool {
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
	allSchemaAttrs, err := p.fetchSchemaAttributes(ctx, category, true, true)
	if err != nil {
		logger.Warn(ctx.Context, "Failed to fetch schema attributes for provisioning", log.Any("error", err))
		execResp.Status = providers.ExecFailure
		return false
	}
	// Collected in the same prompt as the schema attrs rather than a pass of their own.
	missing := missingNonSchemaAttributes(ctx, category)

	if len(allSchemaAttrs) > 0 {
		credRequiredMissing, credOptionalMissing, ncRequiredMissing, ncOptionalMissing :=
			p.buildMissingInputs(ctx, allSchemaAttrs, nodeInputMap)

		// Build the full schema missing list: required non-creds first, then optional non-creds,
		// followed by required creds, then optional creds.
		// Node-defined inputs not present in the schema are ignored — provisioning is schema-driven
		// and can only store attributes defined by the entity type.
		schemaMissing := make([]providers.Input, 0,
			len(ncRequiredMissing)+len(credRequiredMissing)+len(ncOptionalMissing)+len(credOptionalMissing))
		schemaMissing = append(schemaMissing, ncRequiredMissing...)
		schemaMissing = append(schemaMissing, ncOptionalMissing...)
		schemaMissing = append(schemaMissing, credRequiredMissing...)
		schemaMissing = append(schemaMissing, credOptionalMissing...)

		// Schema attrs lead: a record field is asked for last so the prompt reads schema first.
		missing = append(schemaMissing, missing...)
	}

	if len(missing) == 0 {
		return true
	}

	// Apply maxPerPrompt to the forwarded prompt batch.
	toForward := missing
	if maxInputs := p.getMaxDynamicInputs(ctx); maxInputs > 0 && len(toForward) > maxInputs {
		toForward = toForward[:maxInputs]
	}

	execResp.Inputs = missing
	if execResp.ForwardedData == nil {
		execResp.ForwardedData = make(map[string]interface{})
	}
	execResp.ForwardedData[common.ForwardedDataKeyInputs] = toForward
	logger.Debug(ctx.Context, "Provisioning inputs are missing, requesting via prompt",
		log.Int("missingCount", len(missing)))
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
				Type:        inputTypeForSchemaType(attr.Type),
				DisplayName: attr.DisplayName,
			}
			if inNodeInputs {
				input = nodeInp
				input.Identifier = attr.Attribute
				if input.Type == "" {
					input.Type = inputTypeForSchemaType(attr.Type)
				}
				if input.DisplayName == "" {
					input.DisplayName = attr.DisplayName
				}
			}
			// An attribute restricted to a fixed set is offered as a choice rather than as free
			// text. A prompt node declaring the field itself is enriched with these options by
			// identifier, so the permitted values need not be repeated in the flow.
			if len(attr.Enum) > 0 {
				input.Type = providers.InputTypeSelect
				input.Options = attr.Enum
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

// fetchSchemaAttributes retrieves schema attributes from the entity type service for the target
// entity type of the given category. allowCredential and allowNonCredential control which
// attribute classes are returned.
func (p *provisioningExecutor) fetchSchemaAttributes(
	ctx *providers.NodeContext, category entitytype.TypeCategory, allowCredential, allowNonCredential bool,
) ([]entitytype.AttributeInfo, error) {
	if p.entityTypeService == nil {
		return nil, nil
	}
	targetRef, err := p.getTargetEntityRef(ctx, category)
	if err != nil {
		return nil, err
	}
	entityType := targetRef.entityType
	if entityType == "" {
		return nil, fmt.Errorf("entity type not found")
	}
	attrs, svcErr := p.entityTypeService.GetAttributes(ctx.Context,
		category, entityType,
		entitytype.AttributeFilter{AllowCredential: allowCredential, AllowNonCredential: allowNonCredential})
	if svcErr != nil {
		return nil, fmt.Errorf("failed to fetch schema attributes for entity type %q: %s",
			entityType, svcErr.Error.DefaultValue)
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

// isAttrSatisfied returns true if the attribute has a non-empty usable value in the user inputs or
// the runtime data.
func (p *provisioningExecutor) isAttrSatisfied(ctx *providers.NodeContext, attr string) bool {
	if val, ok := ctx.UserInputs[attr]; ok && val != "" {
		return true
	}
	if val, ok := ctx.RuntimeData[attr]; ok && val != "" {
		return true
	}
	return false
}

// getAttributesForProvisioning collects entity attributes from context in a single schema pass,
// returning identifying (non-credential) and credential attributes as separate maps.
// Schema is the whitelist for both maps.
// Values are resolved from non-empty UserInputs then non-empty RuntimeData. Non-credential values
// are converted from the engine's string representation to the type declared by the schema
// attribute.
func (p *provisioningExecutor) getAttributesForProvisioning(
	ctx *providers.NodeContext, category entitytype.TypeCategory,
) (identifyingAttrs map[string]interface{}, credentialAttrs map[string]interface{}, err error) {
	schemaAttrs, fetchErr := p.fetchSchemaAttributes(ctx, category, true, true)
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
				identifyingAttrs[a.Attribute] = convertToSchemaType(value, a.Type)
			} else if runtimeValue, exists := ctx.RuntimeData[a.Attribute]; exists && runtimeValue != "" {
				identifyingAttrs[a.Attribute] = convertToSchemaType(runtimeValue, a.Type)
			}
		}
	}

	return identifyingAttrs, credentialAttrs, nil
}

// createUserInStore provisions a user through the user management provider. The organization unit
// and user type carried on the request are validated by the user service, which owns those rules.
func (p *provisioningExecutor) createUserInStore(nodeCtx *providers.NodeContext,
	userAttributes map[string]interface{}) (*providers.User, *tidcommon.ServiceError) {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, nodeCtx.ExecutionID))
	logger.Debug(nodeCtx.Context, "Creating the user account")

	if p.userMgtProvider == nil {
		logger.Error(nodeCtx.Context, "User management provider is not configured")
		return nil, errForEntityCategory(ErrProvisioningFailed, entitytype.TypeCategoryUser)
	}

	targetRef, err := p.getTargetEntityRef(nodeCtx, entitytype.TypeCategoryUser)
	if err != nil {
		logger.Error(nodeCtx.Context, "Failed to resolve the provisioning target", log.Error(err))
		return nil, errForEntityCategory(ErrProvisioningFailed, entitytype.TypeCategoryUser)
	}

	attributesJSON, err := json.Marshal(userAttributes)
	if err != nil {
		logger.Error(nodeCtx.Context, "Failed to marshal user attributes", log.Error(err))
		return nil, errForEntityCategory(ErrProvisioningFailed, entitytype.TypeCategoryUser)
	}

	createdUser, svcErr := p.userMgtProvider.CreateUser(nodeCtx.Context, &providers.User{
		OUID:       targetRef.ouID,
		Type:       targetRef.entityType,
		Attributes: attributesJSON,
	})
	if svcErr != nil {
		return nil, svcErr
	}
	if createdUser != nil && createdUser.ID != "" {
		logger.Debug(nodeCtx.Context, "User account created successfully",
			log.MaskedString(log.LoggerKeyUserID, createdUser.ID))
	}

	return createdUser, nil
}

// createAgentInStore provisions an agent through the agent management provider. Name, description,
// logo and owner are columns on the agent record rather than schema attributes, and are forwarded
// as collected: the agent service owns their validation.
func (p *provisioningExecutor) createAgentInStore(nodeCtx *providers.NodeContext,
	agentAttributes map[string]interface{}) (*providers.Agent, *tidcommon.ServiceError) {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, nodeCtx.ExecutionID))
	logger.Debug(nodeCtx.Context, "Creating the agent")

	if p.agentMgtProvider == nil {
		logger.Error(nodeCtx.Context, "Agent management provider is not configured")
		return nil, errForEntityCategory(ErrProvisioningFailed, entitytype.TypeCategoryAgent)
	}

	targetRef, err := p.getTargetEntityRef(nodeCtx, entitytype.TypeCategoryAgent)
	if err != nil {
		logger.Error(nodeCtx.Context, "Failed to resolve the provisioning target", log.Error(err))
		return nil, errForEntityCategory(ErrProvisioningFailed, entitytype.TypeCategoryAgent)
	}

	attributesJSON, err := json.Marshal(agentAttributes)
	if err != nil {
		logger.Error(nodeCtx.Context, "Failed to marshal agent attributes", log.Error(err))
		return nil, errForEntityCategory(ErrProvisioningFailed, entitytype.TypeCategoryAgent)
	}

	agent := &providers.Agent{
		OUID:        targetRef.ouID,
		Type:        targetRef.entityType,
		Name:        collectedValue(nodeCtx, nameKey),
		Description: collectedValue(nodeCtx, descriptionKey),
		LogoURL:     collectedValue(nodeCtx, logoURLKey),
		Owner:       nodeCtx.RuntimeData[ownerIDKey],
		Attributes:  attributesJSON,
	}
	// Redirect URIs are the only OAuth value a caller may supply, and are attached only when
	// collected so the provider applies its default otherwise.
	if uris := splitTrimmed(collectedValue(nodeCtx, redirectURIsKey)); len(uris) > 0 {
		agent.InboundAuthConfig = []providers.InboundAuthConfigWithSecret{
			{
				Type:        providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{RedirectURIs: uris},
			},
		}
	}

	delegated, flagErr := collectedFlag(nodeCtx, delegatedKey)
	if flagErr != nil {
		logger.Debug(nodeCtx.Context, "Delegation flag could not be read", log.String("key", delegatedKey))
		return nil, flagErr
	}

	createdAgent, svcErr := p.agentMgtProvider.CreateAgent(nodeCtx.Context, agent, delegated)
	if svcErr != nil {
		return nil, svcErr
	}
	if createdAgent != nil && createdAgent.ID != "" {
		logger.Debug(nodeCtx.Context, "Agent created successfully",
			log.MaskedString(log.LoggerKeyEntityID, createdAgent.ID))
	}

	return createdAgent, nil
}

// collectedFlag reads a boolean the flow collected. Values arrive as strings and are parsed the
// way a boolean schema attribute is, accepting the casings and 1/0 forms a hand-authored flow may
// carry. An absent value is false; one that cannot be read is an error rather than a silent false,
// which would provision the entity in the shape the caller did not ask for.
func collectedFlag(ctx *providers.NodeContext, key string) (bool, *tidcommon.ServiceError) {
	raw := collectedValue(ctx, key)
	if raw == "" {
		return false, nil
	}

	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return false, errInvalidFlagValueFor(key)
	}

	return parsed, nil
}

// collectedValue reads a value the flow collected for key, preferring what an executor resolved
// over what the caller submitted. Submitted values are unfiltered, so a resolved one wins.
func collectedValue(ctx *providers.NodeContext, key string) string {
	if val, ok := ctx.RuntimeData[key]; ok && val != "" {
		return val
	}
	if val, ok := ctx.UserInputs[key]; ok && val != "" {
		return val
	}
	return ""
}

// addAgentDataToResponse publishes the provisioned agent's identifier and the client credentials the
// agent service generated. The secret is returned only on create, so the flow response is the one
// chance the caller has to read it.
func addAgentDataToResponse(execResp *providers.ExecutorResponse, agent *providers.Agent) {
	if execResp.AdditionalData == nil {
		execResp.AdditionalData = make(map[string]string)
	}
	execResp.AdditionalData[common.DataAgentID] = agent.ID
	for _, inboundAuth := range agent.InboundAuthConfig {
		if inboundAuth.OAuthConfig == nil {
			continue
		}
		if inboundAuth.OAuthConfig.ClientID != "" {
			execResp.AdditionalData[common.DataAgentClientID] = inboundAuth.OAuthConfig.ClientID
		}
		if inboundAuth.OAuthConfig.ClientSecret != "" {
			execResp.AdditionalData[common.DataAgentClientSecret] = inboundAuth.OAuthConfig.ClientSecret
		}
	}
}

// handleCreateEntityError surfaces a create failure to the flow. Errors raised by the management
// service are returned unchanged so the caller can tell an attribute clash it can retry from a
// configuration problem it cannot. Only the attribute conflict is rewritten, to keep the
// flow-facing wording it already had.
func (p *provisioningExecutor) handleCreateEntityError(
	ctx *providers.NodeContext,
	category entitytype.TypeCategory,
	svcErr *tidcommon.ServiceError,
	logger *log.Logger,
) *tidcommon.ServiceError {
	if svcErr == nil {
		return errForEntityCategory(ErrProvisioningFailed, category)
	}
	if svcErr.Code == traitsFor(category).attributeConflictCode {
		return errForEntityCategory(ErrProvisioningAttributeConflict, category)
	}
	logger.Error(ctx.Context, "Failed to create the entity in the store",
		log.String("errorCode", svcErr.Code), log.String("message", svcErr.Error.DefaultValue))
	return svcErr
}

// getTargetEntityRef retrieves the target entity reference (entity type and OU ID) for provisioning.
func (p *provisioningExecutor) getTargetEntityRef(ctx *providers.NodeContext,
	category entitytype.TypeCategory) (*entityRef, error) {
	ouID := p.getOUID(ctx)
	entityType := p.getEntityType(ctx)

	if ouID == "" || entityType == "" {
		defaultEntityRef, err := p.getDefaultEntityRef(ctx, category)
		if err != nil {
			return nil, err
		}
		if defaultEntityRef != nil {
			if ouID == "" {
				ouID = defaultEntityRef.ouID
			}
			if entityType == "" {
				entityType = defaultEntityRef.entityType
			}
		}
	}

	return &entityRef{
		entityType: entityType,
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

// getEntityType retrieves the entity type to provision into from runtime data, which a type
// resolver populates. An empty result leaves the target to be resolved from the application's
// configuration.
func (p *provisioningExecutor) getEntityType(ctx *providers.NodeContext) string {
	if val, ok := ctx.RuntimeData[categoryTypeKey]; ok && val != "" {
		return val
	}

	return ""
}

// assignGroupsAndRoles assigns the newly created entity to configured groups and roles.
// If no group or role is configured, the assignments are skipped.
func (p *provisioningExecutor) assignGroupsAndRoles(
	ctx *providers.NodeContext,
	category entitytype.TypeCategory,
	entityID string,
) error {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	groupIDs := p.getGroupsToAssign(ctx)
	roleIDs := p.getRolesToAssign(ctx)

	// Seeding from the authorization mapping (rule-based or direct) is a one-time assignment at
	// creation, it opts in to whatever the federated login's mapping already resolved for this
	// entity's claims, alongside the fixed lists above.
	if isSeedGroupsFromMappingEnabled(ctx) {
		mappedGroupIDs := systemutils.ParseStringArray(ctx.RuntimeData[common.RuntimeKeyMappedGroupIDs], " ")
		groupIDs = systemutils.MergeUniqueStrings(groupIDs, mappedGroupIDs)
	}
	if isSeedRolesFromMappingEnabled(ctx) {
		mappedRoleIDs := systemutils.ParseStringArray(ctx.RuntimeData[common.RuntimeKeyMappedRoleIDs], " ")
		roleIDs = systemutils.MergeUniqueStrings(roleIDs, mappedRoleIDs)
	}

	if len(groupIDs) == 0 && len(roleIDs) == 0 {
		logger.Debug(ctx.Context, "No group or role configured for assignment, skipping")
		return nil
	}

	logger.Debug(ctx.Context, "Assigning groups and roles to the provisioned entity",
		log.MaskedString(log.LoggerKeyEntityID, entityID),
		log.String("groupIDs", strings.Join(groupIDs, ",")),
		log.String("roleIDs", strings.Join(roleIDs, ",")))

	if len(groupIDs) > 0 {
		if svcErr := p.groupService.AddMembersToGroups(ctx.Context,
			[]group.Member{{ID: entityID, Type: traitsFor(category).groupMemberType}}, groupIDs); svcErr != nil {
			return fmt.Errorf("group assignment failed: %s", svcErr.Error.DefaultValue)
		}
	}

	if len(roleIDs) > 0 {
		if svcErr := p.roleAssignmentService.AddAssigneesToRoles(ctx.Context,
			[]role.RoleAssignment{{ID: entityID, Type: traitsFor(category).roleAssigneeType}}, roleIDs); svcErr != nil {
			return fmt.Errorf("role assignment failed: %s", svcErr.Error.DefaultValue)
		}
	}

	logger.Debug(ctx.Context, "Successfully assigned groups and roles",
		log.MaskedString(log.LoggerKeyEntityID, entityID))
	return nil
}

// isSeedGroupsFromMappingEnabled returns the value of the seedGroupsFromMapping node property,
// defaulting to false if absent or not a bool.
func isSeedGroupsFromMappingEnabled(ctx *providers.NodeContext) bool {
	if val, ok := ctx.NodeProperties[propertyKeySeedGroupsFromMapping]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return false
}

// isSeedRolesFromMappingEnabled returns the value of the seedRolesFromMapping node property,
// defaulting to false if absent or not a bool.
func isSeedRolesFromMappingEnabled(ctx *providers.NodeContext) bool {
	if val, ok := ctx.NodeProperties[propertyKeySeedRolesFromMapping]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return false
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

// getDefaultEntityRef resolves the provisioning target when the flow put none in runtime data.
// A nil ref means the target could not be resolved automatically, which is not an error.
func (p *provisioningExecutor) getDefaultEntityRef(ctx *providers.NodeContext,
	category entitytype.TypeCategory) (*entityRef, error) {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Resolving the entity type for automatic provisioning")

	candidates, err := p.selfRegistrableEntityTypes(ctx, category)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		logger.Debug(ctx.Context,
			"No entity type with self-registration enabled, cannot provision automatically")
		return nil, nil
	}

	if len(candidates) > 1 {
		logger.Debug(ctx.Context,
			"Multiple entity types with self-registration enabled, cannot resolve the target automatically")
		return nil, nil
	}

	return &entityRef{
		entityType: candidates[0].Name,
		ouID:       candidates[0].OUID,
	}, nil
}

// selfRegistrableEntityTypes returns the types the node may provision into when none was named in
// runtime data: those the application admits for the category that also permit self-registration.
// An application admitting none provisions nothing, whichever category it is.
func (p *provisioningExecutor) selfRegistrableEntityTypes(ctx *providers.NodeContext,
	category entitytype.TypeCategory) ([]entitytype.EntityType, error) {
	logger := p.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	if p.entityTypeService == nil {
		return nil, nil
	}

	allowed := traitsFor(category).applicationAllowedTypes(ctx)
	if len(allowed) == 0 {
		logger.Debug(ctx.Context, "No allowed entity types configured for the application",
			log.String("category", string(category)))
		return nil, nil
	}

	types := make([]entitytype.EntityType, 0, len(allowed))
	for _, name := range allowed {
		entityType, svcErr := p.entityTypeService.GetEntityTypeByName(ctx.Context, category, name)
		if svcErr != nil {
			return nil, fmt.Errorf("failed to retrieve entity type %q in category %q: %s",
				name, category, svcErr.Error.DefaultValue)
		}
		if entityType.AllowSelfRegistration {
			types = append(types, *entityType)
		}
	}
	return types, nil
}
