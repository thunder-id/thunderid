// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const (
	idfExecLoggerComponentName = "IdentifyingExecutor"
)

// identifyingExecutorInterface defines the interface for identifying executors.
type identifyingExecutorInterface interface {
	IdentifyEntity(ctx context.Context, filters map[string]interface{},
		execResp *providers.ExecutorResponse, category entitytype.TypeCategory) (*string, error)
}

// identifyingExecutor implements the ExecutorInterface for identifying users based on provided attributes.
type identifyingExecutor struct {
	providers.Executor
	entityProvider entityprovider.EntityProviderInterface
	authnProvider  providers.AuthnProviderManager
	accessGuard    *accessGuard
	logger         *log.Logger
}

var _ providers.Executor = (*identifyingExecutor)(nil)
var _ identifyingExecutorInterface = (*identifyingExecutor)(nil)

// newIdentifyingExecutor creates a new instance of IdentifyingExecutor. The authentication provider
// binds the user a federated identity's link holders are narrowed to, and may be nil for an executor
// that never resolves between link holders.
func newIdentifyingExecutor(
	name string,
	defaultInputs, prerequisites []providers.Input,
	flowFactory core.FlowFactoryInterface,
	entityProvider entityprovider.EntityProviderInterface,
	authnProvider providers.AuthnProviderManager,
	accessGuard *accessGuard,
) *identifyingExecutor {
	if name == "" {
		name = ExecutorNameIdentifying
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, idfExecLoggerComponentName),
		log.String(log.LoggerKeyExecutorName, name))

	base := flowFactory.CreateExecutor(ExecutorNameIdentifying, providers.ExecutorTypeUtility,
		defaultInputs, prerequisites, &providers.ExecutorMeta{
			DefaultMode:    ExecutorModeIdentify,
			SupportedModes: []string{ExecutorModeIdentify, ExecutorModeResolve, ExecutorModeCheckState},
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: propertyKeyLoginHintAttribute},
			},
		})
	return &identifyingExecutor{
		Executor:       base,
		entityProvider: entityProvider,
		authnProvider:  authnProvider,
		accessGuard:    accessGuard,
		logger:         logger,
	}
}

// IdentifyEntity identifies an entity based on the provided attributes. The lookup goes through the
// entity provider, so it is not scoped to a single entity category. The category names the kind of
// entity the caller is looking for, and the errors raised here are worded for it.
func (i *identifyingExecutor) IdentifyEntity(ctx context.Context, filters map[string]interface{},
	execResp *providers.ExecutorResponse, category entitytype.TypeCategory) (*string, error) {
	logger := i.logger
	logger.Debug(ctx, "Identifying entity with filters")

	if entityID, ok := filters[userAttributeUserID].(string); ok && entityID != "" {
		entity, epErr := i.entityProvider.GetEntityProfile(ctx, entityID)
		if epErr != nil {
			if epErr.Code == entityprovider.ErrorCodeEntityNotFound {
				execResp.Error = errForEntityCategory(ErrEntityNotFound, category)
			} else {
				execResp.Error = errForEntityCategory(ErrFailedToIdentifyEntity, category)
			}
			execResp.Status = providers.ExecFailure
			return nil, nil
		}
		return &entity.ID, nil
	}

	// filter out non-searchable attributes
	var searchableFilter = make(map[string]interface{})
	for key, value := range filters {
		if !slices.Contains(nonSearchableInputs, key) {
			searchableFilter[key] = value
		}
	}

	entityID, err := i.entityProvider.IdentifyEntity(ctx, searchableFilter)
	if err != nil {
		switch err.Code {
		case entityprovider.ErrorCodeEntityNotFound:
			logger.Debug(ctx, "Entity not found for the provided filters")
			execResp.Error = errForEntityCategory(ErrEntityNotFound, category)
		case entityprovider.ErrorCodeAmbiguousEntity:
			logger.Debug(ctx, "Multiple entities found for the provided filters")
			execResp.Error = errForEntityCategory(ErrAmbiguousEntityIdentity, category)
		default:
			logger.Debug(ctx, "Failed to identify the entity due to error: "+err.Error())
			execResp.Error = errForEntityCategory(ErrFailedToIdentifyEntity, category)
		}
		execResp.Status = providers.ExecFailure
		return nil, nil
	}

	if entityID == nil || *entityID == "" {
		logger.Debug(ctx, "Entity not found for the provided filter")
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrEntityNotFound, category)
		return nil, nil
	}

	return entityID, nil
}

// Execute executes the identifying executor logic.
func (i *identifyingExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := i.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing identifying executor")

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	// After a federated sign-in the link decides the user, so no mode takes typed inputs or matches on
	// attributes: the candidates come from the link.
	if federated := federatedIdentityFrom(ctx); federated != nil {
		switch ctx.ExecutorMode {
		case ExecutorModeResolve:
			return i.executeResolveFederated(ctx, execResp)
		case ExecutorModeCheckState:
			return i.executeCheckStateFederated(ctx, execResp, federated)
		default:
			return i.executeIdentifyFederated(ctx, execResp, federated)
		}
	}

	loginHint := ctx.UserInputs[userInputLoginHint]
	loginHintAttr, _ := ctx.NodeProperties[propertyKeyLoginHintAttribute].(string)

	if loginHintAttr != "" {
		if loginHint == "" {
			execResp.Inputs = append(execResp.Inputs,
				providers.Input{Identifier: userInputLoginHint, Required: true})
			execResp.Status = providers.ExecUserInputRequired
			return execResp, nil
		}
	} else if !i.HasRequiredInputs(ctx, execResp) {
		logger.Debug(ctx.Context, "Required inputs for identifying executor are not provided")
		execResp.Status = providers.ExecUserInputRequired
		return execResp, nil
	}

	switch ctx.ExecutorMode {
	case ExecutorModeResolve:
		return i.executeResolve(ctx, execResp)
	case ExecutorModeCheckState:
		return i.executeCheckState(ctx, execResp)
	default:
		// Default identify behavior (including explicit "identify" mode and unset).
		// Fails if zero or more than one user matches.
		return i.executeIdentify(ctx, execResp)
	}
}

// executeIdentify handles the default identify mode which expects exactly one user match.
func (i *identifyingExecutor) executeIdentify(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) (*providers.ExecutorResponse, error) {
	logger := i.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	userSearchAttributes := i.buildSearchAttributes(ctx)

	userID, err := i.IdentifyEntity(ctx.Context, userSearchAttributes, execResp, categoryUnscoped)
	if err != nil {
		logger.Debug(ctx.Context, "Failed to identify user due to error: "+err.Error())
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrFailedToIdentifyEntity, categoryUnscoped)
		return execResp, nil
	}

	// Only promote ExecFailure to ExecUserInputRequired for recoverable user-input
	// errors (i.e. user not found). Other failures reported by IdentifyEntity — such
	// as ambiguous matches or system errors — are not recoverable in identify mode
	// and must be returned as-is so the caller can handle them appropriately.
	// When loginHintAttribute is set the identifier was supplied externally — there is no
	// interactive user to re-enter it, so keep ExecFailure.
	loginHintAttr, _ := ctx.NodeProperties[propertyKeyLoginHintAttribute].(string)
	if execResp.Status == providers.ExecFailure &&
		execResp.Error != nil && execResp.Error.Code == ErrEntityNotFound.Code && loginHintAttr == "" {
		logger.Debug(ctx.Context, "User not found — promoting to user input required",
			log.Int("searchAttributeCount", len(userSearchAttributes)))
		execResp.Status = providers.ExecUserInputRequired
		execResp.Inputs = i.GetRequiredInputs(ctx)
		return execResp, nil
	}
	if execResp.Status == providers.ExecFailure {
		return execResp, nil
	}

	if userID == nil || *userID == "" {
		logger.Debug(ctx.Context, "User not found for the provided attributes")
		if loginHintAttr == "" {
			execResp.Status = providers.ExecUserInputRequired
			execResp.Inputs = i.GetRequiredInputs(ctx)
		} else {
			execResp.Status = providers.ExecFailure
		}
		execResp.Error = errForEntityCategory(ErrEntityNotFound, categoryUnscoped)
		return execResp, nil
	}

	// A recovery flow never reaches the assertion check, so a held account is refused here.
	if ctx.FlowType == providers.FlowTypeRecovery {
		held, stated, fault := i.accessGuard.heldAt(ctx.Context, *userID, providers.AccessPlaneRecovery, "",
			logger)
		if fault != nil {
			// Answered as a failed account lookup.
			execResp.Status = providers.ExecFailure
			execResp.Error = errForEntityCategory(ErrFailedToIdentifyEntity, categoryUnscoped)
			return execResp, nil
		}
		if held {
			// A refusal that names no hold takes the unknown-user branch.
			if stated != nil {
				execResp.Status = providers.ExecFailure
				execResp.Error = stated
				return execResp, nil
			}
			if loginHintAttr == "" {
				execResp.Status = providers.ExecUserInputRequired
				execResp.Inputs = i.GetRequiredInputs(ctx)
			} else {
				execResp.Status = providers.ExecFailure
			}
			execResp.Error = errForEntityCategory(ErrEntityNotFound, categoryUnscoped)
			return execResp, nil
		}
	}

	execResp.RuntimeData[userAttributeUserID] = *userID
	execResp.Status = providers.ExecComplete

	logger.Debug(ctx.Context, "Identifying executor completed successfully",
		log.MaskedString(log.LoggerKeyUserID, *userID))

	return execResp, nil
}

// executeResolve handles the resolve mode for user disambiguation.
func (i *identifyingExecutor) executeResolve(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) (*providers.ExecutorResponse, error) {
	logger := i.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing identifying executor in resolve mode")

	userSearchAttributes := i.buildSearchAttributes(ctx)

	// Include dynamic user inputs from disambiguation prompts. The disambiguation step
	// may generate inputs (e.g., ouHandle, userType) that are not defined in the node's
	// required inputs, so we merge user inputs to ensure they are used for filtering.
	// We exclude non-searchable inputs and internal identifiers to prevent injection.
	for key, value := range ctx.UserInputs {
		if _, exists := userSearchAttributes[key]; !exists && value != "" &&
			!slices.Contains(nonSearchableInputs, key) && key != userAttributeUserID {
			userSearchAttributes[key] = value
		}
	}

	candidates, err := i.getCandidates(ctx, userSearchAttributes, logger)
	if err != nil {
		execResp.Status = providers.ExecFailure
		execResp.Error = tidcommon.CustomServiceError(ErrFailedToIdentifyEntity, tidcommon.I18nMessage{
			Key:          ErrFailedToIdentifyEntity.ErrorDescription.Key,
			DefaultValue: err.Error(),
		})
		return execResp, nil
	}

	switch len(candidates) {
	case 0:
		logger.Debug(ctx.Context, "No matching users after filtering")
		execResp.Status = providers.ExecUserInputRequired
		execResp.Inputs = i.GetRequiredInputs(ctx)
		execResp.Error = errForEntityCategory(ErrEntityNotFound, categoryUnscoped)
		return execResp, nil
	case 1:
		execResp.RuntimeData[userAttributeUserID] = candidates[0].ID
		execResp.Status = providers.ExecComplete
		logger.Debug(ctx.Context, "User resolved successfully",
			log.MaskedString("userID", candidates[0].ID))
		return execResp, nil
	default:
		return i.handleAmbiguousCandidates(ctx.Context, candidates, execResp, logger)
	}
}

// executeResolveFederated lets the user pick between the users holding a federated identity's link at
// sign-in. Anything else passes through: an identity with at most one holder needs no pick, and
// registration settles the holders against the OU it registers into at provisioning.
func (i *identifyingExecutor) executeResolveFederated(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) (*providers.ExecutorResponse, error) {
	if ctx.FlowType != providers.FlowTypeAuthentication ||
		ctx.RuntimeData[common.RuntimeKeyEntityState] != entityStateAmbiguous {
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}
	return i.executeResolveLinkHolders(ctx, execResp)
}

// executeResolveLinkHolders narrows the users holding a federated identity's link down to one and
// binds it to the AuthUser. The candidates come from the link, and only the options offered for them
// filter it, so the user picks between its holders and nothing else.
func (i *identifyingExecutor) executeResolveLinkHolders(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) (*providers.ExecutorResponse, error) {
	logger := i.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Resolving between the users linked to the federated identity")

	// The holders are loaded from the link on every round, never from stored candidates, which an
	// attribute-based resolve may have left with users who do not hold this link. The picks carried in
	// the user inputs narrow them again each round.
	federated := federatedIdentityFrom(ctx)
	holders, epErr := i.entityProvider.GetLinkedEntities(ctx.Context, federated.IdpID, federated.Sub)
	if epErr != nil {
		logger.Debug(ctx.Context, "Failed to retrieve the users linked to the federated identity",
			log.String("errorCode", string(epErr.Code)))
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrFailedToIdentifyEntity, categoryUnscoped)
		return execResp, nil
	}

	picked := make(map[string]interface{})
	for _, option := range extractDisambiguationOptions(holders) {
		if value := ctx.UserInputs[option.Identifier]; value != "" {
			picked[option.Identifier] = value
		}
	}
	candidates := holders
	if len(picked) > 0 {
		candidates = filterUsersByAttributes(holders, picked)
	}

	switch len(candidates) {
	case 0:
		if len(holders) == 0 {
			logger.Debug(ctx.Context, "No user is linked to the federated identity")
			execResp.Status = providers.ExecFailure
			execResp.Error = errForEntityCategory(ErrEntityNotFound, categoryUnscoped)
			return execResp, nil
		}
		// A pick that matches no holder is offered the holders' options again. The error makes the
		// prompt drop the stale pick, which it would otherwise resubmit.
		resp, err := i.promptLinkHolders(ctx.Context, holders, execResp, logger)
		if err == nil && resp.Status == providers.ExecUserInputRequired {
			resp.Error = errForEntityCategory(ErrEntityNotFound, categoryUnscoped)
		}
		return resp, err
	case 1:
		return i.bindLinkHolder(ctx, execResp, candidates[0].ID, logger)
	default:
		return i.promptLinkHolders(ctx.Context, candidates, execResp, logger)
	}
}

// promptLinkHolders asks the user to pick between link holders. The holders are not stored as
// candidates, so an attribute-based resolve later in the flow does not pick from them.
func (i *identifyingExecutor) promptLinkHolders(ctx context.Context, holders []*providers.Entity,
	execResp *providers.ExecutorResponse, logger *log.Logger) (*providers.ExecutorResponse, error) {
	resp, err := i.handleAmbiguousCandidates(ctx, holders, execResp, logger)
	if resp != nil {
		delete(resp.RuntimeData, common.RuntimeKeyCandidateUsers)
	}
	return resp, err
}

// bindLinkHolder makes the AuthUser name the link holder the user picked. The holder came from the
// link itself, so it is authenticated by entity ID, which the provider admits as a federated sign-in.
func (i *identifyingExecutor) bindLinkHolder(ctx *providers.NodeContext, execResp *providers.ExecutorResponse,
	entityID string, logger *log.Logger) (*providers.ExecutorResponse, error) {
	if i.authnProvider == nil {
		return nil, errors.New("no authentication provider to bind the resolved link holder")
	}
	if svcErr := authenticateEntityByID(ctx, i.authnProvider, entityID, ctx.AuthUser, execResp); svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("failed to bind the resolved link holder: %s", svcErr.Code)
		}
		logger.Debug(ctx.Context, "Failed to bind the resolved link holder", log.String("errorCode", svcErr.Code))
		execResp.Status = providers.ExecFailure
		execResp.Error = cmp.Or(statedHold(svcErr), errForEntityCategory(ErrFailedToIdentifyEntity, categoryUnscoped))
		return execResp, nil
	}

	execResp.RuntimeData[userAttributeUserID] = entityID
	execResp.RuntimeData[common.RuntimeKeyEntityState] = entityStateExists
	execResp.Status = providers.ExecComplete
	logger.Debug(ctx.Context, "Link holder resolved", log.MaskedString(log.LoggerKeyUserID, entityID))
	return execResp, nil
}

// executeCheckState handles the check_state mode which looks up candidates and sets a
// runtime flag indicating whether zero, one, or multiple entities match. Always returns ExecComplete.
func (i *identifyingExecutor) executeCheckState(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) (*providers.ExecutorResponse, error) {
	logger := i.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing identifying executor in check_state mode")

	userSearchAttributes := i.buildSearchAttributes(ctx)

	candidates, err := i.getCandidates(ctx, userSearchAttributes, logger)
	if err != nil {
		execResp.Status = providers.ExecFailure
		execResp.Error = tidcommon.CustomServiceError(ErrFailedToIdentifyEntity, tidcommon.I18nMessage{
			Key:          ErrFailedToIdentifyEntity.ErrorDescription.Key,
			DefaultValue: err.Error(),
		})
		return execResp, nil
	}

	setEntityStateFromCount(execResp, len(candidates))
	logger.Debug(ctx.Context, "Entity state checked", log.Int("candidateCount", len(candidates)))

	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// executeCheckStateFederated sets the entity state from the users holding the federated identity's
// link, never from attributes or claims. A provider that cannot list the holders keeps the state the
// federated executor recorded.
func (i *identifyingExecutor) executeCheckStateFederated(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse, federated *core.ExternalIdentity) (*providers.ExecutorResponse, error) {
	logger := i.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	holders, epErr := i.entityProvider.GetLinkedEntities(ctx.Context, federated.IdpID, federated.Sub)
	if epErr != nil {
		if epErr.Code == entityprovider.ErrorCodeNotImplemented {
			execResp.Status = providers.ExecComplete
			return execResp, nil
		}
		logger.Debug(ctx.Context, "Failed to retrieve the users linked to the federated identity",
			log.String("errorCode", string(epErr.Code)))
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrFailedToIdentifyEntity, categoryUnscoped)
		return execResp, nil
	}

	setEntityStateFromCount(execResp, len(holders))
	logger.Debug(ctx.Context, "Entity state checked from the federated identity's link",
		log.Int("holderCount", len(holders)))
	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// executeIdentifyFederated identifies the one user holding the federated identity's link. No holder
// fails rather than prompting, since a typed identifier would name a user the sign-in never proved.
// Several holders fail too: picking between them is resolve mode's job.
func (i *identifyingExecutor) executeIdentifyFederated(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse, federated *core.ExternalIdentity) (*providers.ExecutorResponse, error) {
	logger := i.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	holders, epErr := i.entityProvider.GetLinkedEntities(ctx.Context, federated.IdpID, federated.Sub)
	if epErr != nil {
		logger.Debug(ctx.Context, "Failed to retrieve the users linked to the federated identity",
			log.String("errorCode", string(epErr.Code)))
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrFailedToIdentifyEntity, categoryUnscoped)
		return execResp, nil
	}

	switch len(holders) {
	case 0:
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrEntityNotFound, categoryUnscoped)
	case 1:
		execResp.RuntimeData[userAttributeUserID] = holders[0].ID
		execResp.Status = providers.ExecComplete
	default:
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrAmbiguousEntityIdentity, categoryUnscoped)
	}
	logger.Debug(ctx.Context, "Identified from the federated identity's link", log.Int("holderCount", len(holders)))
	return execResp, nil
}

// setEntityStateFromCount records whether no entity, one, or several matched.
func setEntityStateFromCount(execResp *providers.ExecutorResponse, count int) {
	switch count {
	case 0:
		execResp.RuntimeData[common.RuntimeKeyEntityState] = entityStateNotExists
	case 1:
		execResp.RuntimeData[common.RuntimeKeyEntityState] = entityStateExists
	default:
		execResp.RuntimeData[common.RuntimeKeyEntityState] = entityStateAmbiguous
	}
}

// buildSearchAttributes collects search attributes from user inputs and runtime data.
// When the loginHintAttribute node property is set, the login_hint value from UserInputs is
// mapped to the configured attribute so the flow admin controls which user attribute resolves
// the identifier without hardcoding a specific attribute.
func (i *identifyingExecutor) buildSearchAttributes(ctx *providers.NodeContext) map[string]interface{} {
	if hintAttr, ok := ctx.NodeProperties[propertyKeyLoginHintAttribute].(string); ok && hintAttr != "" {
		if hint, exists := ctx.UserInputs[userInputLoginHint]; exists && hint != "" {
			return map[string]interface{}{hintAttr: hint}
		}
	}

	attrs := map[string]interface{}{}
	extIdentity := core.GetExternalIdentity(ctx.RuntimeData)
	for _, inputData := range i.GetRequiredInputs(ctx) {
		// External claims are only a fallback and must never take priority over runtime data.
		if value, ok := ctx.UserInputs[inputData.Identifier]; ok {
			attrs[inputData.Identifier] = value
		} else if value, ok := ctx.RuntimeData[inputData.Identifier]; ok {
			attrs[inputData.Identifier] = value
		} else if value, ok := extIdentity.Claim(inputData.Identifier); ok {
			attrs[inputData.Identifier] = value
		}
	}
	return attrs
}

// getCandidates retrieves candidate users either from the store (first call) or from
// stored candidates in RuntimeData (subsequent calls), filtering in-memory.
func (i *identifyingExecutor) getCandidates(ctx *providers.NodeContext,
	searchAttrs map[string]interface{}, logger *log.Logger) ([]*providers.Entity, error) {
	storedCandidates, hasCandidates := ctx.RuntimeData[common.RuntimeKeyCandidateUsers]
	if hasCandidates {
		return i.getFilteredCandidates(ctx.Context, storedCandidates, searchAttrs, logger)
	}
	return i.searchCandidates(ctx.Context, searchAttrs, logger)
}

// searchCandidates performs the initial database search for matching users.
func (i *identifyingExecutor) searchCandidates(ctx context.Context,
	searchAttrs map[string]interface{}, logger *log.Logger) ([]*providers.Entity, error) {
	searchableFilters := make(map[string]interface{})
	for key, value := range searchAttrs {
		if !slices.Contains(nonSearchableInputs, key) {
			searchableFilters[key] = value
		}
	}

	users, err := i.entityProvider.SearchEntities(ctx, searchableFilters)
	if err != nil {
		if err.Code == entityprovider.ErrorCodeEntityNotFound {
			logger.Debug(ctx, "No users found for the provided filters")
			return []*providers.Entity{}, nil
		}
		logger.Debug(ctx, "Failed to search users: "+err.Error())
		return nil, errors.New(ErrFailedToIdentifyEntity.Error.DefaultValue)
	}

	return users, nil
}

// getFilteredCandidates deserializes stored candidates and filters them in-memory.
func (i *identifyingExecutor) getFilteredCandidates(ctx context.Context,
	storedCandidates string, searchAttrs map[string]interface{},
	logger *log.Logger) ([]*providers.Entity, error) {
	var candidates []*providers.Entity
	if err := json.Unmarshal([]byte(storedCandidates), &candidates); err != nil {
		logger.Debug(ctx, "Failed to deserialize candidate users")
		return nil, errors.New(ErrFailedToIdentifyEntity.Error.DefaultValue)
	}

	return filterUsersByAttributes(candidates, searchAttrs), nil
}

// handleAmbiguousCandidates processes the case where multiple candidates still match.
// It extracts disambiguation options and either requests more input or fails if
// candidates are indistinguishable.
func (i *identifyingExecutor) handleAmbiguousCandidates(ctx context.Context,
	candidates []*providers.Entity, execResp *providers.ExecutorResponse,
	logger *log.Logger) (*providers.ExecutorResponse, error) {
	options := extractDisambiguationOptions(candidates)
	if len(options) == 0 {
		logger.Debug(ctx, "Candidates are indistinguishable, no disambiguation options available",
			log.Int("candidateCount", len(candidates)))
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrFailedToIdentifyEntity, categoryUnscoped)
		return execResp, nil
	}

	candidatesJSON, err := json.Marshal(candidates)
	if err != nil {
		logger.Debug(ctx, "Failed to serialize candidate users")
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrFailedToIdentifyEntity, categoryUnscoped)
		return execResp, nil
	}

	execResp.RuntimeData[common.RuntimeKeyCandidateUsers] = string(candidatesJSON)
	execResp.Status = providers.ExecUserInputRequired
	execResp.ForwardedData = map[string]interface{}{
		common.ForwardedDataKeyInputs: options,
	}

	logger.Debug(ctx, "Multiple users still match, requesting additional attributes",
		log.Int("candidateCount", len(candidates)))
	return execResp, nil
}

// filterUsersByAttributes filters users by matching their attributes against the provided filters.
func filterUsersByAttributes(users []*providers.Entity, filters map[string]interface{}) []*providers.Entity {
	var matched []*providers.Entity
	for _, u := range users {
		var attrs map[string]interface{}
		if len(u.Attributes) > 0 {
			if err := json.Unmarshal(u.Attributes, &attrs); err != nil {
				continue
			}
		}

		allMatch := true
		for key, expected := range filters {
			if slices.Contains(nonSearchableInputs, key) {
				continue
			}

			if !utils.IsScalar(expected) {
				continue
			}
			expectedStr := utils.ConvertInterfaceValueToString(expected)

			// Check top-level User fields first
			switch key {
			case "userType":
				if u.Type != expectedStr {
					allMatch = false
				}
			case "ouHandle":
				if u.OUHandle != expectedStr {
					allMatch = false
				}
			default:
				// Check in JSON attributes
				if attrs == nil {
					allMatch = false
				} else if value, ok := attrs[key]; !ok {
					allMatch = false
				} else if !utils.IsScalar(value) || utils.ConvertInterfaceValueToString(value) != expectedStr {
					allMatch = false
				}
			}

			if !allMatch {
				break
			}
		}

		if allMatch {
			matched = append(matched, u)
		}
	}
	return matched
}

// extractDisambiguationOptions extracts distinct attribute values from candidate users
// and returns them as []providers.Input with Options populated. This allows downstream prompt
// nodes to render dropdowns when enriched via ForwardedData.
func extractDisambiguationOptions(candidates []*providers.Entity) []providers.Input {
	// Collect distinct values per attribute key (including top-level fields)
	optionsMap := make(map[string]map[string]struct{})

	for _, u := range candidates {
		// Top-level fields
		if u.Type != "" {
			if optionsMap["userType"] == nil {
				optionsMap["userType"] = make(map[string]struct{})
			}
			optionsMap["userType"][u.Type] = struct{}{}
		}
		if u.OUHandle != "" {
			if optionsMap["ouHandle"] == nil {
				optionsMap["ouHandle"] = make(map[string]struct{})
			}
			optionsMap["ouHandle"][u.OUHandle] = struct{}{}
		}

		// JSON attributes
		var attrs map[string]interface{}
		if len(u.Attributes) > 0 {
			if err := json.Unmarshal(u.Attributes, &attrs); err != nil {
				continue
			}
		}
		for key, value := range attrs {
			if slices.Contains(nonSearchableInputs, key) {
				continue
			}
			if utils.IsScalar(value) {
				valueStr := utils.ConvertInterfaceValueToString(value)
				if optionsMap[key] == nil {
					optionsMap[key] = make(map[string]struct{})
				}
				optionsMap[key][valueStr] = struct{}{}
			}
		}
	}

	// Convert to []providers.Input — only include attributes with more than one distinct value
	// (single-value attributes don't help with disambiguation)
	inputs := make([]providers.Input, 0, len(optionsMap))
	for key, valuesSet := range optionsMap {
		if len(valuesSet) <= 1 {
			continue
		}
		options := make([]string, 0, len(valuesSet))
		for v := range valuesSet {
			options = append(options, v)
		}
		inputs = append(inputs, providers.Input{
			Identifier: key,
			Type:       providers.InputTypeSelect,
			Options:    options,
		})
	}

	return inputs
}
