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
	"slices"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const (
	idfExecLoggerComponentName = "IdentifyingExecutor"
)

// identifyingExecutorInterface defines the interface for identifying executors.
type identifyingExecutorInterface interface {
	IdentifyUser(ctx context.Context, filters map[string]interface{},
		execResp *common.ExecutorResponse) (*string, error)
}

// identifyingExecutor implements the ExecutorInterface for identifying users based on provided attributes.
type identifyingExecutor struct {
	core.ExecutorInterface
	entityProvider entityprovider.EntityProviderInterface
	logger         *log.Logger
}

var _ core.ExecutorInterface = (*identifyingExecutor)(nil)
var _ identifyingExecutorInterface = (*identifyingExecutor)(nil)

// newIdentifyingExecutor creates a new instance of IdentifyingExecutor.
func newIdentifyingExecutor(
	name string,
	defaultInputs, prerequisites []common.Input,
	flowFactory core.FlowFactoryInterface,
	entityProvider entityprovider.EntityProviderInterface,
) *identifyingExecutor {
	if name == "" {
		name = ExecutorNameIdentifying
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, idfExecLoggerComponentName),
		log.String(log.LoggerKeyExecutorName, name))

	base := flowFactory.CreateExecutor(ExecutorNameIdentifying, common.ExecutorTypeUtility,
		defaultInputs, prerequisites)
	return &identifyingExecutor{
		ExecutorInterface: base,
		entityProvider:    entityProvider,
		logger:            logger,
	}
}

// IdentifyUser identifies a user based on the provided attributes.
func (i *identifyingExecutor) IdentifyUser(ctx context.Context, filters map[string]interface{},
	execResp *common.ExecutorResponse) (*string, error) {
	logger := i.logger
	logger.Debug(ctx, "Identifying user with filters")

	// filter out non-searchable attributes
	var searchableFilter = make(map[string]interface{})
	for key, value := range filters {
		if !slices.Contains(nonSearchableInputs, key) {
			searchableFilter[key] = value
		}
	}

	userID, err := i.entityProvider.IdentifyEntity(searchableFilter)
	if err != nil {
		switch err.Code {
		case entityprovider.ErrorCodeEntityNotFound:
			logger.Debug(ctx, "User not found for the provided filters")
			execResp.Error = &ErrUserNotFound
		case entityprovider.ErrorCodeAmbiguousEntity:
			logger.Debug(ctx, "Multiple users found for the provided filters")
			execResp.Error = &ErrAmbiguousUserIdentity
		default:
			logger.Debug(ctx, "Failed to identify user due to error: "+err.Error())
			execResp.Error = &ErrFailedToIdentifyUser
		}
		execResp.Status = common.ExecFailure
		return nil, nil
	}

	if userID == nil || *userID == "" {
		logger.Debug(ctx, "User not found for the provided filter")
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrUserNotFound
		return nil, nil
	}

	return userID, nil
}

// Execute executes the identifying executor logic.
func (i *identifyingExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := i.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing identifying executor")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	if !i.HasRequiredInputs(ctx, execResp) {
		logger.Debug(ctx.Context, "Required inputs for identifying executor are not provided")
		execResp.Status = common.ExecUserInputRequired
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
func (i *identifyingExecutor) executeIdentify(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) (*common.ExecutorResponse, error) {
	logger := i.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	userSearchAttributes := i.buildSearchAttributes(ctx)

	userID, err := i.IdentifyUser(ctx.Context, userSearchAttributes, execResp)
	if err != nil {
		logger.Debug(ctx.Context, "Failed to identify user due to error: "+err.Error())
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrFailedToIdentifyUser
		return execResp, nil
	}

	// Only promote ExecFailure to ExecUserInputRequired for recoverable user-input
	// errors (i.e. user not found). Other failures reported by IdentifyUser — such
	// as ambiguous matches or system errors — are not recoverable in identify mode
	// and must be returned as-is so the caller can handle them appropriately.
	// When loginHintAttribute is set the identifier was supplied externally (e.g. CIBA
	// login_hint) — there is no interactive user to re-enter it, so keep ExecFailure.
	_, loginHintAttrSet := ctx.NodeProperties[propertyKeyLoginHintAttribute]
	if execResp.Status == common.ExecFailure &&
		execResp.Error != nil && execResp.Error.Code == ErrUserNotFound.Code && !loginHintAttrSet {
		logger.Debug(ctx.Context, "User not found — promoting to user input required",
			log.Int("searchAttributeCount", len(userSearchAttributes)))
		execResp.Status = common.ExecUserInputRequired
		execResp.Inputs = i.GetRequiredInputs(ctx)
		return execResp, nil
	}
	if execResp.Status == common.ExecFailure {
		return execResp, nil
	}

	if userID == nil || *userID == "" {
		logger.Debug(ctx.Context, "User not found for the provided attributes")
		if !loginHintAttrSet {
			execResp.Status = common.ExecUserInputRequired
			execResp.Inputs = i.GetRequiredInputs(ctx)
		} else {
			execResp.Status = common.ExecFailure
		}
		execResp.Error = &ErrUserNotFound
		return execResp, nil
	}

	execResp.RuntimeData[userAttributeUserID] = *userID
	execResp.Status = common.ExecComplete

	logger.Debug(ctx.Context, "Identifying executor completed successfully",
		log.MaskedString(log.LoggerKeyUserID, *userID))

	return execResp, nil
}

// executeResolve handles the resolve mode for user disambiguation.
func (i *identifyingExecutor) executeResolve(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) (*common.ExecutorResponse, error) {
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
		execResp.Status = common.ExecFailure
		execResp.Error = serviceerror.CustomServiceError(ErrFailedToIdentifyUser, i18ncore.I18nMessage{
			Key:          ErrFailedToIdentifyUser.ErrorDescription.Key,
			DefaultValue: err.Error(),
		})
		return execResp, nil
	}

	switch len(candidates) {
	case 0:
		logger.Debug(ctx.Context, "No matching users after filtering")
		execResp.Status = common.ExecUserInputRequired
		execResp.Inputs = i.GetRequiredInputs(ctx)
		execResp.Error = &ErrUserNotFound
		return execResp, nil
	case 1:
		execResp.RuntimeData[userAttributeUserID] = candidates[0].ID
		execResp.Status = common.ExecComplete
		logger.Debug(ctx.Context, "User resolved successfully",
			log.MaskedString("userID", candidates[0].ID))
		return execResp, nil
	default:
		return i.handleAmbiguousCandidates(ctx.Context, candidates, execResp, logger)
	}
}

// executeCheckState handles the check_state mode which looks up candidates and sets a
// runtime flag indicating whether zero, one, or multiple entities match. Always returns ExecComplete.
func (i *identifyingExecutor) executeCheckState(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) (*common.ExecutorResponse, error) {
	logger := i.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing identifying executor in check_state mode")

	userSearchAttributes := i.buildSearchAttributes(ctx)

	candidates, err := i.getCandidates(ctx, userSearchAttributes, logger)
	if err != nil {
		execResp.Status = common.ExecFailure
		execResp.Error = serviceerror.CustomServiceError(ErrFailedToIdentifyUser, i18ncore.I18nMessage{
			Key:          ErrFailedToIdentifyUser.ErrorDescription.Key,
			DefaultValue: err.Error(),
		})
		return execResp, nil
	}

	switch len(candidates) {
	case 0:
		execResp.RuntimeData[common.RuntimeKeyEntityState] = entityStateNotExists
		logger.Debug(ctx.Context, "No users found for the provided attributes")
	case 1:
		execResp.RuntimeData[common.RuntimeKeyEntityState] = entityStateExists
		logger.Debug(ctx.Context, "Single user found",
			log.MaskedString("userID", candidates[0].ID))
	default:
		execResp.RuntimeData[common.RuntimeKeyEntityState] = entityStateAmbiguous
		logger.Debug(ctx.Context, "Multiple users found",
			log.Int("candidateCount", len(candidates)))
	}

	execResp.Status = common.ExecComplete
	return execResp, nil
}

// buildSearchAttributes collects search attributes from user inputs and runtime data.
// When the loginHintAttribute node property is set, the login_hint value from UserInputs is
// mapped to the configured attribute so the flow admin controls which user attribute resolves
// the CIBA hint without hardcoding a specific attribute in the CIBA service.
func (i *identifyingExecutor) buildSearchAttributes(ctx *core.NodeContext) map[string]interface{} {
	if hintAttr, ok := ctx.NodeProperties[propertyKeyLoginHintAttribute].(string); ok && hintAttr != "" {
		if hint, exists := ctx.UserInputs[common.UserInputKeyLoginHint]; exists && hint != "" {
			return map[string]interface{}{hintAttr: hint}
		}
		// login_hint absent — fall through to the normal input/runtime-data path below
	}

	attrs := map[string]interface{}{}
	for _, inputData := range i.GetRequiredInputs(ctx) {
		if value, ok := ctx.UserInputs[inputData.Identifier]; ok {
			attrs[inputData.Identifier] = value
		} else if value, ok := ctx.RuntimeData[inputData.Identifier]; ok {
			attrs[inputData.Identifier] = value
		}
	}
	return attrs
}

// getCandidates retrieves candidate users either from the store (first call) or from
// stored candidates in RuntimeData (subsequent calls), filtering in-memory.
func (i *identifyingExecutor) getCandidates(ctx *core.NodeContext,
	searchAttrs map[string]interface{}, logger *log.Logger) ([]*entityprovider.Entity, error) {
	storedCandidates, hasCandidates := ctx.RuntimeData[common.RuntimeKeyCandidateUsers]
	if hasCandidates {
		return i.getFilteredCandidates(ctx.Context, storedCandidates, searchAttrs, logger)
	}
	return i.searchCandidates(ctx.Context, searchAttrs, logger)
}

// searchCandidates performs the initial database search for matching users.
func (i *identifyingExecutor) searchCandidates(ctx context.Context,
	searchAttrs map[string]interface{}, logger *log.Logger) ([]*entityprovider.Entity, error) {
	searchableFilters := make(map[string]interface{})
	for key, value := range searchAttrs {
		if !slices.Contains(nonSearchableInputs, key) {
			searchableFilters[key] = value
		}
	}

	users, err := i.entityProvider.SearchEntities(searchableFilters)
	if err != nil {
		if err.Code == entityprovider.ErrorCodeEntityNotFound {
			logger.Debug(ctx, "No users found for the provided filters")
			return []*entityprovider.Entity{}, nil
		}
		logger.Debug(ctx, "Failed to search users: "+err.Error())
		return nil, errors.New(ErrFailedToIdentifyUser.Error.DefaultValue)
	}

	return users, nil
}

// getFilteredCandidates deserializes stored candidates and filters them in-memory.
func (i *identifyingExecutor) getFilteredCandidates(ctx context.Context,
	storedCandidates string, searchAttrs map[string]interface{},
	logger *log.Logger) ([]*entityprovider.Entity, error) {
	var candidates []*entityprovider.Entity
	if err := json.Unmarshal([]byte(storedCandidates), &candidates); err != nil {
		logger.Debug(ctx, "Failed to deserialize candidate users")
		return nil, errors.New(ErrFailedToIdentifyUser.Error.DefaultValue)
	}

	return filterUsersByAttributes(candidates, searchAttrs), nil
}

// handleAmbiguousCandidates processes the case where multiple candidates still match.
// It extracts disambiguation options and either requests more input or fails if
// candidates are indistinguishable.
func (i *identifyingExecutor) handleAmbiguousCandidates(ctx context.Context,
	candidates []*entityprovider.Entity, execResp *common.ExecutorResponse,
	logger *log.Logger) (*common.ExecutorResponse, error) {
	options := extractDisambiguationOptions(candidates)
	if len(options) == 0 {
		logger.Debug(ctx, "Candidates are indistinguishable, no disambiguation options available",
			log.Int("candidateCount", len(candidates)))
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrFailedToIdentifyUser
		return execResp, nil
	}

	candidatesJSON, err := json.Marshal(candidates)
	if err != nil {
		logger.Debug(ctx, "Failed to serialize candidate users")
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrFailedToIdentifyUser
		return execResp, nil
	}

	execResp.RuntimeData[common.RuntimeKeyCandidateUsers] = string(candidatesJSON)
	execResp.Status = common.ExecUserInputRequired
	execResp.ForwardedData = map[string]interface{}{
		common.ForwardedDataKeyInputs: options,
	}

	logger.Debug(ctx, "Multiple users still match, requesting additional attributes",
		log.Int("candidateCount", len(candidates)))
	return execResp, nil
}

// filterUsersByAttributes filters users by matching their attributes against the provided filters.
func filterUsersByAttributes(users []*entityprovider.Entity, filters map[string]interface{}) []*entityprovider.Entity {
	var matched []*entityprovider.Entity
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
// and returns them as []common.Input with Options populated. This allows downstream prompt
// nodes to render dropdowns when enriched via ForwardedData.
func extractDisambiguationOptions(candidates []*entityprovider.Entity) []common.Input {
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

	// Convert to []common.Input — only include attributes with more than one distinct value
	// (single-value attributes don't help with disambiguation)
	inputs := make([]common.Input, 0, len(optionsMap))
	for key, valuesSet := range optionsMap {
		if len(valuesSet) <= 1 {
			continue
		}
		options := make([]string, 0, len(valuesSet))
		for v := range valuesSet {
			options = append(options, v)
		}
		inputs = append(inputs, common.Input{
			Identifier: key,
			Type:       common.InputTypeSelect,
			Options:    options,
		})
	}

	return inputs
}
