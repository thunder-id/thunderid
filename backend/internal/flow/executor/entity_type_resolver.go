// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"fmt"
	"slices"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// entityTypeCandidateLimit bounds the candidate list a resolver considers.
const entityTypeCandidateLimit = 100

// entityTypeResolution resolves the entity type to provision as, for any category. The
// per-category resolvers embed it, pick the flow handler for the flow type, and pass their category.
//
// The outcome follows the number of candidates that survive filtering: none fails, one is taken
// without asking, several are offered as a choice.
type entityTypeResolution struct {
	// base is the resolver's own executor, which declares the type input.
	base              providers.Executor
	entityTypeService entitytype.EntityTypeServiceInterface
	ouService         providers.OrganizationUnitProvider
}

// resolveAuthFlow admits authentication only when the application allows at least one type of the
// category.
func (r *entityTypeResolution) resolveAuthFlow(ctx *providers.NodeContext, category entitytype.TypeCategory,
	execResp *providers.ExecutorResponse,
) *providers.ExecutorResponse {
	if len(traitsFor(category).applicationAllowedTypes(ctx)) == 0 {
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrAuthNotAvailableForApp
		return execResp
	}

	execResp.Status = providers.ExecComplete
	return execResp
}

// resolveOnboardingFlow resolves the type for an administrator-driven flow. The candidates come from
// the type catalog, narrowed by the node's allowed types and by the selected organization unit. The
// application's allowed types are not consulted: they govern which types may authenticate to an
// application, not which an administrator may create.
func (r *entityTypeResolution) resolveOnboardingFlow(ctx *providers.NodeContext, category entitytype.TypeCategory,
	execResp *providers.ExecutorResponse, logger *log.Logger,
) (*providers.ExecutorResponse, error) {
	allowedTypes := allowedTypesFromProperties(ctx, traitsFor(category).allowedTypesProperty)
	defaultInputs := r.base.GetDefaultInputs()

	// The first default input is the type input, the same one promptOptions offers the choice on.
	var submitted string
	if len(defaultInputs) > 0 {
		submitted = ctx.UserInputs[defaultInputs[0].Identifier]
	}
	if submitted != "" {
		return r.resolveSubmitted(ctx, category, submitted, allowedTypes, execResp, logger)
	}

	candidates, ok, err := r.candidates(ctx, category, allowedTypes, execResp, logger)
	if err != nil {
		return nil, err
	}
	if !ok {
		return execResp, nil
	}

	if len(candidates) == 1 {
		return r.accept(ctx, category, candidates[0].Handle, candidates[0].OUID, execResp, logger), nil
	}

	options := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		options = append(options, candidate.Handle)
	}
	r.prompt(ctx, category, options, defaultInputs, execResp, logger)

	return execResp, nil
}

// resolveSubmitted validates a type the caller already chose.
func (r *entityTypeResolution) resolveSubmitted(ctx *providers.NodeContext,
	category entitytype.TypeCategory, selected string, allowedTypes []string,
	execResp *providers.ExecutorResponse, logger *log.Logger,
) (*providers.ExecutorResponse, error) {
	if len(allowedTypes) > 0 && !slices.Contains(allowedTypes, selected) {
		logger.Debug(ctx.Context, "Entity type not in allowed list", log.String(categoryTypeKey, selected),
			log.Any("allowedTypes", allowedTypes))
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrUserTypeNotAllowed, category)
		return execResp, nil
	}

	entityType, ouID, err := r.entityTypeAndOU(ctx.Context, category, selected)
	if err != nil {
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrInvalidUserType, category)
		return execResp, nil
	}

	// A selected organization unit constrains the type to one at or above it.
	if selectedOUID, exists := ctx.RuntimeData[ouIDKey]; exists && selectedOUID != "" {
		isValid, svcErr := r.ouService.IsParent(ctx.Context, ouID, selectedOUID)
		if svcErr != nil {
			return nil, fmt.Errorf("failed to validate %s type against selected OU: %s",
				category, svcErr.Error.DefaultValue)
		}
		if !isValid {
			logger.Debug(ctx.Context, "Entity type not valid for the selected OU",
				log.String(categoryTypeKey, selected), log.String(ouIDKey, selectedOUID))
			execResp.Status = providers.ExecFailure
			execResp.Error = errForEntityCategory(ErrUserTypeNotValidForOU, category)
			return execResp, nil
		}
	}

	logger.Debug(ctx.Context, "Entity type resolved from input", log.String(categoryTypeKey, selected),
		log.String(ouIDKey, entityType.OUID))

	return r.accept(ctx, category, selected, ouID, execResp, logger), nil
}

// candidates lists the types the node may resolve to, narrowed by the allowed list and by the
// selected organization unit. A false second return means execResp already carries the outcome.
func (r *entityTypeResolution) candidates(ctx *providers.NodeContext,
	category entitytype.TypeCategory, allowedTypes []string, execResp *providers.ExecutorResponse,
	logger *log.Logger,
) ([]entitytype.EntityTypeListItem, bool, error) {
	list, svcErr := r.entityTypeService.GetEntityTypeList(
		ctx.Context, category, entityTypeCandidateLimit, 0, false)
	if svcErr != nil {
		logger.Debug(ctx.Context, "Failed to list entity types",
			log.String("error", svcErr.Error.DefaultValue))
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrUserTypeRetrievalFailed, category)
		return nil, false, nil
	}
	if len(list.Types) == 0 {
		logger.Debug(ctx.Context, "No entity types available")
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrNoUserTypesAvailable, category)
		return nil, false, nil
	}

	candidates := filterTypesByAllowed(list.Types, allowedTypes)

	if selectedOUID, exists := ctx.RuntimeData[ouIDKey]; exists && selectedOUID != "" {
		var err error
		candidates, err = r.filterTypesByOU(ctx, candidates, selectedOUID, logger)
		if err != nil {
			return nil, false, err
		}
	}

	if len(candidates) == 0 {
		logger.Debug(ctx.Context, "No valid entity types after filtering",
			log.Any("allowedTypes", allowedTypes))
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrNoValidUserTypes, category)
		return nil, false, nil
	}

	return candidates, true, nil
}

// resolveRegistrationFlow resolves the type an entity registers itself as. The candidates are the
// application's allowed types for the category, narrowed by the node's allowed types, and only a type
// open to self-registration is accepted.
func (r *entityTypeResolution) resolveRegistrationFlow(ctx *providers.NodeContext,
	category entitytype.TypeCategory, execResp *providers.ExecutorResponse, logger *log.Logger,
) (*providers.ExecutorResponse, error) {
	allowed := traitsFor(category).applicationAllowedTypes(ctx)
	if len(allowed) == 0 {
		// TODO: This should be improved to fallback to the application's ou when the support is available.
		//  A type has an attached ou. Need to find the type from the application's ou.
		//  Also should check if self registration is enabled for the type when the support is available.
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrSelfRegNotAvailableForApp
		return execResp, nil
	}

	if nodeAllowed := allowedTypesFromProperties(ctx, traitsFor(category).allowedTypesProperty); len(nodeAllowed) > 0 {
		allowed = slices.DeleteFunc(slices.Clone(allowed), func(handle string) bool {
			return !slices.Contains(nodeAllowed, handle)
		})
		if len(allowed) == 0 {
			execResp.Status = providers.ExecFailure
			execResp.Error = errForEntityCategory(ErrNoValidUserTypes, category)
			return execResp, nil
		}
	}

	defaultInputs := r.base.GetDefaultInputs()
	if r.base.HasRequiredInputs(ctx, execResp) {
		var selected string
		if len(defaultInputs) > 0 {
			selected = ctx.UserInputs[defaultInputs[0].Identifier]
		}
		if !slices.Contains(allowed, selected) {
			execResp.Status = providers.ExecFailure
			execResp.Error = errForEntityCategory(ErrUserTypeNotAllowed, category)
			return execResp, nil
		}
		allowed = []string{selected}
	}

	selfRegEnabled := make([]*entitytype.EntityType, 0, len(allowed))
	for _, handle := range allowed {
		entityType, _, err := r.entityTypeAndOU(ctx.Context, category, handle)
		if err != nil {
			return execResp, err
		}
		if entityType.AllowSelfRegistration {
			selfRegEnabled = append(selfRegEnabled, entityType)
		}
	}

	switch len(selfRegEnabled) {
	case 0:
		execResp.Status = providers.ExecFailure
		// A single named type reports that its own self-registration is off.
		execResp.Error = &ErrSelfRegNotAvailableForApp
		if len(allowed) == 1 {
			execResp.Error = &ErrSelfRegDisabledForUserType
		}
		return execResp, nil
	case 1:
		return r.accept(ctx, category, selfRegEnabled[0].Handle, selfRegEnabled[0].OUID, execResp, logger), nil
	}

	options := make([]string, 0, len(selfRegEnabled))
	for _, entityType := range selfRegEnabled {
		options = append(options, entityType.Handle)
	}
	r.prompt(ctx, category, options, defaultInputs, execResp, logger)

	return execResp, nil
}

// accept records the resolved type handle, its category and the organization unit it lives in.
func (r *entityTypeResolution) accept(ctx *providers.NodeContext, category entitytype.TypeCategory,
	handle, ouID string, execResp *providers.ExecutorResponse, logger *log.Logger,
) *providers.ExecutorResponse {
	logger.Debug(ctx.Context, "Entity type resolved", log.String(categoryTypeKey, handle),
		log.String(ouIDKey, ouID))
	execResp.RuntimeData[categoryNameKey] = string(category)
	execResp.RuntimeData[categoryTypeKey] = handle
	execResp.RuntimeData[defaultOUIDKey] = ouID
	execResp.Status = providers.ExecComplete

	return execResp
}

// prompt offers the options on the executor's first declared input, forwarding it so a prompt node
// renders the same choice.
func (r *entityTypeResolution) prompt(ctx *providers.NodeContext, category entitytype.TypeCategory,
	options []string, defaultInputs []providers.Input, execResp *providers.ExecutorResponse,
	logger *log.Logger,
) {
	logger.Debug(ctx.Context, "Prompting for entity type selection",
		log.String("category", string(category)), log.Any("options", options))

	execResp.Status = providers.ExecUserInputRequired
	if len(defaultInputs) > 0 {
		input := defaultInputs[0]
		input.Options = options
		execResp.Inputs = []providers.Input{input}
		execResp.ForwardedData[common.ForwardedDataKeyInputs] = execResp.Inputs
	}
}

// entityTypeAndOU reads a type by handle and the organization unit it lives in.
func (r *entityTypeResolution) entityTypeAndOU(ctx context.Context, category entitytype.TypeCategory,
	handle string,
) (*entitytype.EntityType, string, error) {
	entityType, svcErr := r.entityTypeService.GetEntityTypeByHandle(ctx, category, handle)
	if svcErr != nil {
		return nil, "", fmt.Errorf("failed to resolve %s type: %s", category, handle)
	}
	if entityType.OUID == "" {
		return nil, "", fmt.Errorf("no organization unit found for %s type: %s", category, handle)
	}

	return entityType, entityType.OUID, nil
}

// filterTypesByAllowed narrows the types to the allowed list, or returns them all when it is
// empty.
func filterTypesByAllowed(
	types []entitytype.EntityTypeListItem, allowedTypes []string,
) []entitytype.EntityTypeListItem {
	if len(allowedTypes) == 0 {
		return types
	}

	filtered := make([]entitytype.EntityTypeListItem, 0, len(types))
	for _, entityType := range types {
		if slices.Contains(allowedTypes, entityType.Handle) {
			filtered = append(filtered, entityType)
		}
	}

	return filtered
}

// filterTypesByOU keeps the types whose own organization unit is an ancestor of, or equal to, the
// selected one.
func (r *entityTypeResolution) filterTypesByOU(ctx *providers.NodeContext,
	types []entitytype.EntityTypeListItem, selectedOUID string, logger *log.Logger,
) ([]entitytype.EntityTypeListItem, error) {
	filtered := make([]entitytype.EntityTypeListItem, 0, len(types))
	for _, entityType := range types {
		isValid, svcErr := r.ouService.IsParent(ctx.Context, entityType.OUID, selectedOUID)
		if svcErr != nil {
			return nil, fmt.Errorf("failed to check OU ancestry for schema %s: %s",
				entityType.Handle, svcErr.Error.DefaultValue)
		}
		if isValid {
			filtered = append(filtered, entityType)
		}
	}

	logger.Debug(ctx.Context, "Filtered entity types by the selected OU",
		log.String(ouIDKey, selectedOUID), log.Int("before", len(types)), log.Int("after", len(filtered)))

	return filtered, nil
}

// allowedTypesFromProperties reads an optional list of type handles from the node property at key.
func allowedTypesFromProperties(ctx *providers.NodeContext, key string) []string {
	if ctx.NodeProperties == nil {
		return nil
	}

	val, exists := ctx.NodeProperties[key]
	if !exists {
		return nil
	}
	items, ok := val.([]interface{})
	if !ok {
		return nil
	}

	handles := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok && s != "" {
			handles = append(handles, s)
		}
	}

	return handles
}
