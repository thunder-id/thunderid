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

// entityTypeResolutionInterface defines the resolution steps the per-category resolvers delegate
// to. They embed it, so the steps are reached on the resolver itself.
type entityTypeResolutionInterface interface {
	resolve(ctx *providers.NodeContext, category entitytype.TypeCategory, allowedTypes []string,
		defaultInputs []providers.Input, execResp *providers.ExecutorResponse,
		logger *log.Logger) (*providers.ExecutorResponse, error)
	entityTypeAndOU(ctx context.Context, category entitytype.TypeCategory, name string,
		logger *log.Logger) (*entitytype.EntityType, string, error)
	promptOptions(ctx context.Context, execResp *providers.ExecutorResponse, options []string,
		defaultInputs []providers.Input, logger *log.Logger)
}

// entityTypeResolution resolves the entity type to provision as, for any category. The
// per-category resolvers own the flow-type handling and delegate the resolution here.
//
// The outcome follows the number of candidates that survive filtering: none fails, one is taken
// without asking, several are offered as a choice.
type entityTypeResolution struct {
	entityTypeService entitytype.EntityTypeServiceInterface
	ouService         providers.OrganizationUnitProvider
}

var _ entityTypeResolutionInterface = (*entityTypeResolution)(nil)

// resolve runs the resolution for a category. defaultInputs carries the input to offer when there
// is a choice, and allowedTypes narrows the candidates.
func (r *entityTypeResolution) resolve(ctx *providers.NodeContext, category entitytype.TypeCategory,
	allowedTypes []string, defaultInputs []providers.Input, execResp *providers.ExecutorResponse,
	logger *log.Logger,
) (*providers.ExecutorResponse, error) {
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
		return r.accept(ctx, candidates[0].Name, candidates[0].OUID, execResp, logger), nil
	}

	options := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		options = append(options, candidate.Name)
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

	entityType, ouID, err := r.entityTypeAndOU(ctx.Context, category, selected, logger)
	if err != nil {
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrInvalidUserType, category)
		return execResp, nil
	}

	// A selected organization unit constrains the type to one at or above it.
	if selectedOUID, exists := ctx.RuntimeData[ouIDKey]; exists && selectedOUID != "" {
		isValid, svcErr := r.ouService.IsParent(ctx.Context, ouID, selectedOUID)
		if svcErr != nil {
			logger.Error(ctx.Context, "Failed to validate the entity type against the selected OU",
				log.String(categoryTypeKey, selected), log.String(ouIDKey, selectedOUID),
				log.String("error", svcErr.Error.DefaultValue))
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

	return r.accept(ctx, selected, ouID, execResp, logger), nil
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

// accept records the resolved type and the organization unit it lives in.
func (r *entityTypeResolution) accept(ctx *providers.NodeContext, name, ouID string,
	execResp *providers.ExecutorResponse, logger *log.Logger,
) *providers.ExecutorResponse {
	logger.Debug(ctx.Context, "Entity type resolved", log.String(categoryTypeKey, name),
		log.String(ouIDKey, ouID))
	execResp.RuntimeData[categoryTypeKey] = name
	execResp.RuntimeData[defaultOUIDKey] = ouID
	execResp.Status = providers.ExecComplete

	return execResp
}

// prompt offers the surviving candidates as a choice.
func (r *entityTypeResolution) prompt(ctx *providers.NodeContext, category entitytype.TypeCategory,
	options []string, defaultInputs []providers.Input, execResp *providers.ExecutorResponse,
	logger *log.Logger,
) {
	logger.Debug(ctx.Context, "Prompting for entity type selection",
		log.String("category", string(category)), log.Any("options", options))
	r.promptOptions(ctx.Context, execResp, options, defaultInputs, logger)
}

// promptOptions offers the options on the executor's first declared input, forwarding it so a
// prompt node renders the same choice.
func (r *entityTypeResolution) promptOptions(ctx context.Context, execResp *providers.ExecutorResponse,
	options []string, defaultInputs []providers.Input, logger *log.Logger,
) {
	logger.Debug(ctx, "Prompting for type selection", log.Any("options", options))

	execResp.Status = providers.ExecUserInputRequired
	if len(defaultInputs) > 0 {
		input := defaultInputs[0]
		input.Options = options
		execResp.Inputs = []providers.Input{input}
		execResp.ForwardedData[common.ForwardedDataKeyInputs] = execResp.Inputs
	}
}

// entityTypeAndOU reads a type by name and the organization unit it lives in.
func (r *entityTypeResolution) entityTypeAndOU(ctx context.Context, category entitytype.TypeCategory,
	name string, logger *log.Logger,
) (*entitytype.EntityType, string, error) {
	entityType, svcErr := r.entityTypeService.GetEntityTypeByName(ctx, category, name)
	if svcErr != nil {
		logger.Error(ctx, "Failed to resolve the entity type", log.String("name", name),
			log.String("error", svcErr.Error.DefaultValue))
		return nil, "", fmt.Errorf("failed to resolve %s type: %s", category, name)
	}
	if entityType.OUID == "" {
		logger.Error(ctx, "No organization unit found for the entity type", log.String("name", name))
		return nil, "", fmt.Errorf("no organization unit found for %s type: %s", category, name)
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
		if slices.Contains(allowedTypes, entityType.Name) {
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
			logger.Error(ctx.Context, "Failed to check OU ancestry for schema",
				log.String("schema", entityType.Name), log.String("error", svcErr.Error.DefaultValue))
			return nil, fmt.Errorf("failed to check OU ancestry for schema %s: %s",
				entityType.Name, svcErr.Error.DefaultValue)
		}
		if isValid {
			filtered = append(filtered, entityType)
		}
	}

	logger.Debug(ctx.Context, "Filtered entity types by the selected OU",
		log.String(ouIDKey, selectedOUID), log.Int("before", len(types)), log.Int("after", len(filtered)))

	return filtered, nil
}

// allowedTypesFromProperties reads an optional list of type names from the node property at key.
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

	names := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok && s != "" {
			names = append(names, s)
		}
	}

	return names
}
