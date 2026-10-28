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
			Identifier: categoryTypeKey,
			Required:   true,
		},
	}
	base := flowFactory.CreateExecutor(ExecutorNameAttributeUniquenessValidator, providers.ExecutorTypeUtility,
		[]providers.Input{}, prerequisites, &providers.ExecutorMeta{
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: propertyKeyProvisioningMode},
				{Property: common.NodePropertyAllowCrossOUProvisioning},
			},
		})
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

	category, err := categoryFromMode(ctx)
	if err != nil {
		return nil, err
	}

	if !e.ValidatePrerequisites(ctx, execResp, e.authnProvider) {
		return execResp, nil
	}

	entityType := ctx.RuntimeData[categoryTypeKey]
	if entityType == "" {
		logger.Debug(ctx.Context, "No entity type in runtime data, nothing to check uniqueness against")
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}

	svcCtx := security.WithRuntimeContext(context.Background())
	uniqueAttrs, svcErr := e.entityTypeService.GetUniqueAttributes(svcCtx, category, entityType)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to retrieve unique attributes from schema for %s type %s: %s",
			category, entityType, svcErr.Error.DefaultValue)
	}

	// With cross-OU provisioning the same email may belong to users in other OUs, so a deployment-wide
	// lookup would reject valid invites (or fail as ambiguous). Email is checked against the target OU below.
	scopeEmailToTargetOU := category == entitytype.TypeCategoryUser && isCrossOUProvisioningAllowed(ctx)

	for _, attr := range uniqueAttrs {
		if scopeEmailToTargetOU && attr == userAttributeEmail {
			continue
		}
		value, exists := ctx.UserInputs[attr]
		if !exists || value == "" {
			continue
		}

		userID, svcErr := e.entityProvider.IdentifyEntity(map[string]interface{}{attr: value})
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
			return execResp, nil
		}
	}

	if scopeEmailToTargetOU {
		inTargetOU, err := e.emailExistsInTargetOU(ctx)
		if err != nil {
			return nil, err
		}
		if inTargetOU {
			logger.Debug(ctx.Context, "User with the provided email already exists in the target OU")
			execResp.Status = providers.ExecUserInputRequired
			execResp.Error = errForEntityCategory(ErrEntityAlreadyExistsInTargetOU, category)
			return execResp, nil
		}
	}

	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// emailExistsInTargetOU reports whether a user with the provided email already belongs to the OU
// the flow provisions into. Users with the same email in other OUs are allowed, since cross-OU
// provisioning permits one identity per OU. It fails when no target OU has been resolved, since the
// check cannot rule out a duplicate without one.
func (e *attributeUniquenessValidator) emailExistsInTargetOU(ctx *providers.NodeContext) (bool, error) {
	email := ctx.UserInputs[userAttributeEmail]
	if email == "" {
		return false, nil
	}
	targetOUID := ctx.RuntimeData[ouIDKey]
	if targetOUID == "" {
		targetOUID = ctx.RuntimeData[defaultOUIDKey]
	}
	if targetOUID == "" {
		return false, errors.New("cannot check email against the target OU: no target OU is resolved")
	}

	matches, svcErr := e.entityProvider.SearchEntities(map[string]interface{}{userAttributeEmail: email})
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
