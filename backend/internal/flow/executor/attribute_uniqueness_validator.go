// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// attributeUniquenessValidator checks whether values supplied for unique schema attributes
// are already held by an existing entity.  It is intended to be placed in a flow immediately
// after a prompt node so that conflicts can be reported with the specific attribute name
// before any creation executor runs.
type attributeUniquenessValidator struct {
	providers.Executor
	entityTypeService entitytype.EntityTypeServiceInterface
	entityProvider    entityprovider.EntityProviderInterface
	authnProvider     providers.AuthnProviderManager
	logger            *log.Logger
}

// newAttributeUniquenessValidator creates a new instance of attributeUniquenessValidator.
func newAttributeUniquenessValidator(
	flowFactory core.FlowFactoryInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	entityProvider entityprovider.EntityProviderInterface,
	authnProvider providers.AuthnProviderManager,
) *attributeUniquenessValidator {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, ExecutorNameAttributeUniquenessValidator))
	prerequisites := []providers.Input{
		{
			Identifier: categoryNameKey,
			Required:   true,
		},
		{
			Identifier: categoryTypeKey,
			Required:   true,
		},
	}
	base := flowFactory.CreateExecutor(ExecutorNameAttributeUniquenessValidator, providers.ExecutorTypeUtility,
		[]providers.Input{}, prerequisites, &providers.ExecutorMeta{})
	return &attributeUniquenessValidator{
		Executor:          base,
		entityTypeService: entityTypeService,
		entityProvider:    entityProvider,
		authnProvider:     authnProvider,
		logger:            logger,
	}
}

// Execute iterates over the unique attributes defined in the entity type and checks whether
// any value already present in UserInputs belongs to an existing entity.
// Returns ExecUserInputRequired (triggering onIncomplete routing) with the specific attribute
// named in the structured error when a conflict is detected, or ExecComplete when all values are free.
func (e *attributeUniquenessValidator) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing uniqueness checker executor")

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	if !e.ValidatePrerequisites(ctx, execResp, e.authnProvider) {
		return execResp, nil
	}

	categoryName := ctx.RuntimeData[categoryNameKey]
	entityType := ctx.RuntimeData[categoryTypeKey]
	if categoryName == "" || entityType == "" {
		logger.Debug(ctx.Context, "No category or entity type in runtime data, nothing to check uniqueness against")
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}

	category := entitytype.TypeCategory(categoryName)
	if !category.IsValid() {
		return nil, fmt.Errorf("invalid category: %s", category)
	}

	svcCtx := security.WithRuntimeContext(ctx.Context)
	uniqueAttrs, svcErr := e.entityTypeService.GetUniqueAttributes(svcCtx, category, entityType)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to retrieve unique attributes from schema for %s type %s: %s",
			category, entityType, svcErr.Error.DefaultValue)
	}

	// Cross-OU provisioning gives one user an account per OU, so the same email legitimately recurs
	// across OUs. A deployment-wide email lookup would reject those, so email is checked against the
	// target OU instead. Every other unique attribute stays deployment-wide.
	scopeEmailToTargetOU := category == entitytype.TypeCategoryUser && isCrossOUProvisioningAllowed(ctx)

	// Uniqueness is deployment-wide: a holder outside the sign-in boundary must still be found.
	identifyCtx := security.WithRuntimeContext(context.Background())
	for _, attr := range uniqueAttrs {
		if scopeEmailToTargetOU && attr == userAttributeEmail {
			continue
		}
		value, exists := ctx.UserInputs[attr]
		if !exists || value == "" {
			continue
		}

		userID, svcErr := e.entityProvider.IdentifyEntity(identifyCtx, map[string]interface{}{attr: value})
		if svcErr != nil {
			if svcErr.Code == entityprovider.ErrorCodeEntityNotFound {
				continue
			}
			return nil, fmt.Errorf("failed to check uniqueness for attribute %s: %s", attr, svcErr.Message)
		}

		if userID != nil {
			logger.Debug(ctx.Context, "Unique attribute conflict detected", log.String("attribute", attr))
			execResp.Status = providers.ExecUserInputRequired
			execResp.Error = errAttributeNotUniqueFor(attr, category)
			// Ask for the value again: a dynamic prompt only offers inputs that are still missing.
			delete(ctx.UserInputs, attr)
			execResp.RuntimeData[attr] = ""
			execResp.Inputs = []providers.Input{e.reaskInput(svcCtx, category, entityType, attr)}
			execResp.ForwardedData = map[string]interface{}{common.ForwardedDataKeyInputs: execResp.Inputs}
			return execResp, nil
		}
	}

	if scopeEmailToTargetOU {
		inTargetOU, err := e.emailExistsInTargetOU(identifyCtx, ctx)
		if err != nil {
			return nil, err
		}
		if inTargetOU {
			logger.Debug(ctx.Context, "User with the provided email already exists in the target OU")
			execResp.Status = providers.ExecUserInputRequired
			execResp.Error = errForEntityCategory(ErrEntityAlreadyExistsInTargetOU, category)
			// Ask for the value again: a dynamic prompt only offers inputs that are still missing.
			delete(ctx.UserInputs, userAttributeEmail)
			execResp.RuntimeData[userAttributeEmail] = ""
			execResp.Inputs = []providers.Input{e.reaskInput(svcCtx, category, entityType, userAttributeEmail)}
			execResp.ForwardedData = map[string]interface{}{common.ForwardedDataKeyInputs: execResp.Inputs}
			return execResp, nil
		}
	}

	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// emailExistsInTargetOU reports whether a user with the provided email already belongs to the OU the
// flow provisions into. Users holding the same email in other OUs are allowed, since cross-OU
// provisioning gives one user an account per OU. It fails when no target OU has been resolved,
// because the check cannot rule out a duplicate without one.
func (e *attributeUniquenessValidator) emailExistsInTargetOU(identifyCtx context.Context,
	ctx *providers.NodeContext) (bool, error) {
	email := ctx.UserInputs[userAttributeEmail]
	if email == "" {
		return false, nil
	}
	targetOUID := targetOUIDForNode(ctx)
	if targetOUID == "" {
		return false, errors.New("cannot check the email against the target OU: no target OU is resolved")
	}

	matches, svcErr := e.entityProvider.SearchEntities(identifyCtx,
		map[string]interface{}{userAttributeEmail: email})
	if svcErr != nil {
		if svcErr.Code == entityprovider.ErrorCodeEntityNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to search users by email: %s", svcErr.Message)
	}

	for _, m := range matches {
		if m != nil && m.Category == providers.EntityCategoryUser && m.OUID == targetOUID {
			return true, nil
		}
	}
	return false, nil
}

// targetOUIDForNode resolves the organization unit a new entity would be created in, mirroring the
// placement rule the provisioning executor applies in getOUID so both read the same unit: the entity
// type's own unit when the node asks for that placement, and the unit resolved by the flow otherwise.
func targetOUIDForNode(ctx *providers.NodeContext) string {
	placement, _ := ctx.NodeProperties[propertyKeyProvisioningOU].(string)
	if provisioningOU(placement) == provisioningOUUserType {
		return ctx.RuntimeData[defaultOUIDKey]
	}
	if ouID := ctx.RuntimeData[ouIDKey]; ouID != "" {
		return ouID
	}
	return ctx.RuntimeData[defaultOUIDKey]
}

// reaskInput builds the input that asks for a conflicting value again. It takes the display name, type
// and choices from the schema attribute when the schema has it, and is a plain required text input otherwise.
func (e *attributeUniquenessValidator) reaskInput(ctx context.Context, category entitytype.TypeCategory,
	entityType, attr string) providers.Input {
	input := providers.Input{Identifier: attr, Type: providers.InputTypeText, Required: true}
	attrs, svcErr := e.entityTypeService.GetAttributes(ctx, category, entityType,
		entitytype.AttributeFilter{AllowNonCredential: true})
	if svcErr != nil {
		return input
	}
	for _, schemaAttr := range attrs {
		if schemaAttr.Attribute != attr {
			continue
		}
		input.DisplayName = schemaAttr.DisplayName
		input.Type = inputTypeForSchemaType(schemaAttr.Type)
		if len(schemaAttr.Enum) > 0 {
			input.Type = providers.InputTypeSelect
			input.Options = schemaAttr.Enum
			input.Ref = attr
		}
		break
	}
	return input
}
