// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"fmt"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
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
			Identifier: userTypeKey,
			Required:   true,
		},
	}
	base := flowFactory.CreateExecutor(ExecutorNameAttributeUniquenessValidator, providers.ExecutorTypeUtility,
		[]providers.Input{}, prerequisites, &providers.ExecutorMeta{
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: propertyKeyProvisioningMode},
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

	// The declared prerequisite names the user type, so it only describes the user category. Other
	// categories carry their type under their own runtime key and are checked directly.
	if category == entitytype.TypeCategoryUser {
		if !e.ValidatePrerequisites(ctx, execResp, e.authnProvider) {
			return execResp, nil
		}
	}

	entityType := ctx.RuntimeData[runtimeEntityTypeKey(category)]
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

	for _, attr := range uniqueAttrs {
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

	execResp.Status = providers.ExecComplete
	return execResp, nil
}
