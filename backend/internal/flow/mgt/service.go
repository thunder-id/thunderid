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

// Package flowmgt provides flow definition management functionality.
package flowmgt

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/flow/graphbuilder"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const loggerComponentName = "FlowMgtService"

var (
	errFlowHandleExists = errors.New("flow with handle already exists")
	errFlowIDExists     = errors.New("flow with id already exists")
	errClientValidation = errors.New("client validation failed")
)

// handleFormatRegex matches valid handle format:
// - starts with lowercase letter or digit
// - contains only lowercase letters, digits, underscores, or dashes
// - ends with lowercase letter or digit
var handleFormatRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*[a-z0-9]$|^[a-z0-9]$`)

// FlowMgtServiceInterface defines the interface for the flow management service.
type FlowMgtServiceInterface interface {
	ListFlows(ctx context.Context, limit, offset int, flowType providers.FlowType) (
		*FlowListResponse, *tidcommon.ServiceError)
	CreateFlow(
		ctx context.Context,
		flowDef *FlowDefinition,
	) (*providers.CompleteFlowDefinition, *tidcommon.ServiceError)
	GetFlow(ctx context.Context, flowID string) (*providers.CompleteFlowDefinition, *tidcommon.ServiceError)
	GetFlowByHandle(ctx context.Context, handle string, flowType providers.FlowType) (
		*providers.CompleteFlowDefinition, *tidcommon.ServiceError)
	UpdateFlow(ctx context.Context, flowID string, flowDef *FlowDefinition) (
		*providers.CompleteFlowDefinition, *tidcommon.ServiceError)
	DeleteFlow(ctx context.Context, flowID string) *tidcommon.ServiceError
	ListFlowVersions(ctx context.Context, flowID string) (*FlowVersionListResponse, *tidcommon.ServiceError)
	GetFlowVersion(ctx context.Context, flowID string, version int) (*FlowVersion, *tidcommon.ServiceError)
	RestoreFlowVersion(ctx context.Context, flowID string, version int) (
		*providers.CompleteFlowDefinition, *tidcommon.ServiceError)
	GetGraph(ctx context.Context, flowID string) (core.GraphInterface, *tidcommon.ServiceError)
	IsValidFlow(ctx context.Context, flowID string, flowType providers.FlowType) (bool, *tidcommon.ServiceError)
}

// flowMgtService is the default implementation of the FlowMgtServiceInterface.
type flowMgtService struct {
	store               flowStoreInterface
	inferenceService    flowInferenceServiceInterface
	graphBuilder        graphbuilder.GraphBuilderInterface
	executorRegistry    executor.ExecutorRegistryInterface
	interceptorRegistry interceptor.InterceptorRegistryInterface
	compositeStore      *compositeFlowStore
	transactioner       transaction.Transactioner
	logger              *log.Logger
}

// newFlowMgtService creates a new instance of flowMgtService.
func newFlowMgtService(
	store flowStoreInterface,
	inferenceService flowInferenceServiceInterface,
	graphBuilder graphbuilder.GraphBuilderInterface,
	executorRegistry executor.ExecutorRegistryInterface,
	interceptorRegistry interceptor.InterceptorRegistryInterface,
	compositeStore *compositeFlowStore,
	transactioner transaction.Transactioner,
) FlowMgtServiceInterface {
	return &flowMgtService{
		store:               store,
		inferenceService:    inferenceService,
		graphBuilder:        graphBuilder,
		executorRegistry:    executorRegistry,
		interceptorRegistry: interceptorRegistry,
		compositeStore:      compositeStore,
		transactioner:       transactioner,
		logger:              log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
	}
}

// Flow management methods

// ListFlows retrieves a paginated list of flow definitions. Supports optional filtering by flow type.
func (s *flowMgtService) ListFlows(ctx context.Context, limit, offset int, flowType providers.FlowType) (
	*FlowListResponse, *tidcommon.ServiceError) {
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	if offset < 0 {
		offset = 0
	}

	if flowType != "" && !isValidFlowType(flowType) {
		return nil, &ErrorInvalidFlowType
	}

	flows, totalCount, err := s.store.ListFlows(ctx, limit, offset, string(flowType))
	if err != nil {
		s.logger.Error(ctx, "Failed to list flows", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	listResponse := &FlowListResponse{
		TotalResults: totalCount,
		StartIndex:   offset + 1,
		Count:        len(flows),
		Flows:        flows,
		Links:        buildPaginationLinks(limit, offset, totalCount),
	}

	return listResponse, nil
}

// CreateFlow creates a new flow definition with version 1.
func (s *flowMgtService) CreateFlow(ctx context.Context, flowDef *FlowDefinition) (
	*providers.CompleteFlowDefinition, *tidcommon.ServiceError) {
	if err := validateFlowDefinition(flowDef); err != nil {
		return nil, err
	}
	if err := s.validateInterceptorRegistration(flowDef.Interceptors); err != nil {
		return nil, err
	}

	flowID := flowDef.ID
	if flowID == "" {
		generated, genErr := utils.GenerateUUIDv7()
		if genErr != nil {
			s.logger.Error(ctx, "Failed to generate UUID v7", log.Error(genErr))
			return nil, &tidcommon.InternalServerError
		}
		flowID = generated
	}

	var createdFlow *providers.CompleteFlowDefinition
	txErr := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if flowDef.ID != "" {
			_, err := s.store.GetFlowByID(txCtx, flowID)
			if err == nil {
				return errFlowIDExists
			}
			if !errors.Is(err, errFlowNotFound) {
				return err
			}
		}

		exists, err := s.store.IsFlowExistsByHandle(txCtx, flowDef.Handle, flowDef.FlowType)
		if err != nil {
			return err
		}
		if exists {
			return errFlowHandleExists
		}

		var storeErr error
		createdFlow, storeErr = s.store.CreateFlow(txCtx, flowID, flowDef)
		return storeErr
	})
	if txErr != nil {
		if errors.Is(txErr, errFlowIDExists) {
			return nil, &ErrorDuplicateFlowID
		}
		if errors.Is(txErr, errFlowHandleExists) {
			return nil, &ErrorDuplicateFlowHandle
		}
		s.logger.Error(ctx, "Failed to create flow", log.Error(txErr))
		return nil, &tidcommon.InternalServerError
	}

	s.logger.Debug(ctx, "Flow created successfully", log.String(logKeyFlowID, flowID))

	s.tryInferRegistrationFlow(ctx, flowID, flowDef)

	return createdFlow, nil
}

// GetFlow retrieves a flow definition by its ID.
func (s *flowMgtService) GetFlow(ctx context.Context, flowID string) (
	*providers.CompleteFlowDefinition, *tidcommon.ServiceError) {
	if flowID == "" {
		return nil, &ErrorMissingFlowID
	}

	flow, err := s.store.GetFlowByID(ctx, flowID)
	if err != nil {
		if errors.Is(err, errFlowNotFound) {
			return nil, &ErrorFlowNotFound
		}
		s.logger.Error(ctx, "Failed to get flow", log.String(logKeyFlowID, flowID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return flow, nil
}

// GetFlowByHandle retrieves a flow definition by its handle and type.
func (s *flowMgtService) GetFlowByHandle(ctx context.Context, handle string, flowType providers.FlowType) (
	*providers.CompleteFlowDefinition, *tidcommon.ServiceError) {
	if handle == "" {
		return nil, &ErrorMissingFlowHandle
	}
	if !isValidFlowType(flowType) {
		return nil, &ErrorInvalidFlowType
	}

	flow, err := s.store.GetFlowByHandle(ctx, handle, flowType)
	if err != nil {
		if errors.Is(err, errFlowNotFound) {
			return nil, &ErrorFlowNotFound
		}
		s.logger.Error(ctx, "Failed to get flow by handle", log.String("handle", handle),
			log.String("flowType", string(flowType)), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return flow, nil
}

// UpdateFlow updates an existing flow definition with the incremented version.
// Old versions are retained up to the configured max_version_history limit.
func (s *flowMgtService) UpdateFlow(ctx context.Context, flowID string, flowDef *FlowDefinition) (
	*providers.CompleteFlowDefinition, *tidcommon.ServiceError) {
	if flowID == "" {
		return nil, &ErrorMissingFlowID
	}
	if err := validateFlowDefinition(flowDef); err != nil {
		return nil, err
	}
	if err := s.validateInterceptorRegistration(flowDef.Interceptors); err != nil {
		return nil, err
	}

	logger := s.logger.With(log.String(logKeyFlowID, flowID))

	var updatedFlow *providers.CompleteFlowDefinition
	var validationSvcErr *tidcommon.ServiceError
	txErr := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		existingFlow, err := s.store.GetFlowByID(txCtx, flowID)
		if err != nil {
			return err
		}

		if existingFlow.IsReadOnly {
			validationSvcErr = &ErrorFlowDeclarativeReadOnly
			return errClientValidation
		}

		// Prevent changing the flow type
		if existingFlow.FlowType != flowDef.FlowType {
			validationSvcErr = &ErrorCannotUpdateFlowType
			return errClientValidation
		}

		// Prevent changing the handle
		if existingFlow.Handle != flowDef.Handle {
			validationSvcErr = &ErrorHandleUpdateNotAllowed
			return errClientValidation
		}

		var updateErr error
		updatedFlow, updateErr = s.store.UpdateFlow(txCtx, flowID, flowDef)
		return updateErr
	})
	if txErr != nil {
		if errors.Is(txErr, errClientValidation) {
			return nil, validationSvcErr
		}
		if errors.Is(txErr, errFlowNotFound) {
			return nil, &ErrorFlowNotFound
		}
		logger.Error(ctx, "Failed to update flow", log.Error(txErr))
		return nil, &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Flow updated successfully")

	// Invalidate the cached graph since the flow has been updated
	s.graphBuilder.InvalidateCache(ctx, flowID)

	return updatedFlow, nil
}

// DeleteFlow deletes a flow definition and all its version history.
func (s *flowMgtService) DeleteFlow(ctx context.Context, flowID string) *tidcommon.ServiceError {
	if flowID == "" {
		return &ErrorMissingFlowID
	}

	logger := s.logger.With(log.String(logKeyFlowID, flowID))

	existingFlow, err := s.store.GetFlowByID(ctx, flowID)
	if err != nil {
		if errors.Is(err, errFlowNotFound) {
			// Silently return if the flow does not exist
			return nil
		}
		logger.Error(ctx, "Failed to get existing flow", log.Error(err))
		return &tidcommon.InternalServerError
	}

	if existingFlow.IsReadOnly {
		return &ErrorFlowDeclarativeReadOnly
	}

	err = s.store.DeleteFlow(ctx, flowID)
	if err != nil {
		logger.Error(ctx, "Failed to delete flow", log.Error(err))
		return &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Flow deleted successfully")

	// Invalidate the cached graph since the flow has been deleted
	s.graphBuilder.InvalidateCache(ctx, flowID)

	return nil
}

// Flow version management methods

// ListFlowVersions retrieves all versions of a flow definition.
func (s *flowMgtService) ListFlowVersions(ctx context.Context, flowID string) (
	*FlowVersionListResponse, *tidcommon.ServiceError) {
	if flowID == "" {
		return nil, &ErrorMissingFlowID
	}

	logger := s.logger.With(log.String(logKeyFlowID, flowID))

	_, err := s.store.GetFlowByID(ctx, flowID)
	if err != nil {
		if errors.Is(err, errFlowNotFound) {
			return nil, &ErrorFlowNotFound
		}
		logger.Error(ctx, "Failed to get existing flow", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	versions, err := s.store.ListFlowVersions(ctx, flowID)
	if err != nil {
		logger.Error(ctx, "Failed to list flow versions", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	response := &FlowVersionListResponse{
		TotalVersions: len(versions),
		Versions:      versions,
	}

	return response, nil
}

// GetFlowVersion retrieves a specific version of a flow definition.
func (s *flowMgtService) GetFlowVersion(ctx context.Context, flowID string, version int) (
	*FlowVersion, *tidcommon.ServiceError) {
	if flowID == "" {
		return nil, &ErrorMissingFlowID
	}
	if version <= 0 {
		return nil, &ErrorInvalidVersion
	}

	flowVersion, err := s.store.GetFlowVersion(ctx, flowID, version)
	if err != nil {
		if errors.Is(err, errFlowNotFound) {
			return nil, &ErrorFlowNotFound
		}
		if errors.Is(err, errVersionNotFound) {
			return nil, &ErrorVersionNotFound
		}
		s.logger.Error(ctx, "Failed to get flow version", log.String(logKeyFlowID, flowID),
			log.Int(logKeyVersion, version), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return flowVersion, nil
}

// RestoreFlowVersion restores a specific version as the active version.
// Creates a new version by copying the configuration from the specified version.
func (s *flowMgtService) RestoreFlowVersion(ctx context.Context, flowID string, version int) (
	*providers.CompleteFlowDefinition, *tidcommon.ServiceError) {
	if flowID == "" {
		return nil, &ErrorMissingFlowID
	}
	if version <= 0 {
		return nil, &ErrorInvalidVersion
	}

	logger := s.logger.With(log.String(logKeyFlowID, flowID), log.Int(logKeyVersion, version))

	var restoredFlow *providers.CompleteFlowDefinition
	txErr := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		_, err := s.store.GetFlowVersion(txCtx, flowID, version)
		if err != nil {
			return err
		}

		restoredFlow, err = s.store.RestoreFlowVersion(txCtx, flowID, version)
		return err
	})
	if txErr != nil {
		if errors.Is(txErr, errFlowNotFound) {
			return nil, &ErrorFlowNotFound
		}
		if errors.Is(txErr, errVersionNotFound) {
			return nil, &ErrorVersionNotFound
		}
		logger.Error(ctx, "Failed to restore flow version", log.Error(txErr))
		return nil, &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Flow version restored successfully")

	// Invalidate the cached graph since a version has been restored
	s.graphBuilder.InvalidateCache(ctx, flowID)

	return restoredFlow, nil
}

// Graph building methods

// GetGraph retrieves or builds a graph for the given flow ID.
func (s *flowMgtService) GetGraph(ctx context.Context, flowID string) (
	core.GraphInterface, *tidcommon.ServiceError) {
	if flowID == "" {
		return nil, &ErrorMissingFlowID
	}

	// Fetch flow definition from store
	flow, err := s.store.GetFlowByID(ctx, flowID)
	if err != nil {
		if errors.Is(err, errFlowNotFound) {
			return nil, &ErrorFlowNotFound
		}
		s.logger.Error(ctx, "Failed to get flow for graph building", log.String(logKeyFlowID, flowID),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return s.graphBuilder.GetGraph(ctx, flow)
}

// IsValidFlow checks if a flow exists for the given flow ID and matches the expected type.
// Returns (false, nil) when the flow is not found or the type does not match (client error).
// Returns (false, *tidcommon.ServiceError) when a store failure occurs (server error).
func (s *flowMgtService) IsValidFlow(
	ctx context.Context, flowID string, flowType providers.FlowType) (bool, *tidcommon.ServiceError) {
	if flowID == "" {
		return false, nil
	}

	flow, err := s.store.GetFlowByID(ctx, flowID)
	if err != nil {
		if errors.Is(err, errFlowNotFound) {
			return false, nil
		}
		return false, &tidcommon.InternalServerError
	}

	return flow.FlowType == flowType, nil
}

// Helper functions

// isValidFlowType checks if the provided flow type is valid.
func isValidFlowType(flowType providers.FlowType) bool {
	return flowType == providers.FlowTypeAuthentication ||
		flowType == providers.FlowTypeRegistration ||
		flowType == providers.FlowTypeUserOnboarding ||
		flowType == providers.FlowTypeRecovery
}

// buildPaginationLinks constructs pagination links for the flow list response.
func buildPaginationLinks(limit, offset, totalCount int) []Link {
	links := make([]Link, 0)

	// Add first and previous links if not on first page
	if offset > 0 {
		links = append(links, Link{
			Href: fmt.Sprintf("/flows?offset=0&limit=%d", limit),
			Rel:  "first",
		})

		prevOffset := offset - limit
		if prevOffset < 0 {
			prevOffset = 0
		}
		links = append(links, Link{
			Href: fmt.Sprintf("/flows?offset=%d&limit=%d", prevOffset, limit),
			Rel:  "prev",
		})
	}

	// Add next link if there are more results
	if offset+limit < totalCount {
		nextOffset := offset + limit
		links = append(links, Link{
			Href: fmt.Sprintf("/flows?offset=%d&limit=%d", nextOffset, limit),
			Rel:  "next",
		})
	}

	// Add last link if not on last page
	lastPageOffset := ((totalCount - 1) / limit) * limit
	if totalCount > 0 && offset < lastPageOffset {
		links = append(links, Link{
			Href: fmt.Sprintf("/flows?offset=%d&limit=%d", lastPageOffset, limit),
			Rel:  "last",
		})
	}

	return links
}

// validateFlowDefinition validates the flow definition request.
func validateFlowDefinition(flowDef *FlowDefinition) *tidcommon.ServiceError {
	if flowDef == nil {
		return &ErrorInvalidRequestFormat
	}
	if flowDef.Handle == "" {
		return &ErrorMissingFlowHandle
	}
	if !isValidHandleFormat(flowDef.Handle) {
		return &ErrorInvalidFlowHandleFormat
	}
	if flowDef.Name == "" {
		return &ErrorMissingFlowName
	}
	if !isValidFlowType(flowDef.FlowType) {
		return &ErrorInvalidFlowType
	}
	if flowDef.ID != "" && !utils.IsValidUUID(flowDef.ID) {
		return &ErrorInvalidFlowIDFormat
	}

	if len(flowDef.Nodes) < 2 {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.flow_requires_start_and_end_nodes_description",
			DefaultValue: "Flow definition must contain at least a start and an end node",
		})
	} else if len(flowDef.Nodes) == 2 {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.flow_requires_intermediate_nodes_description",
			DefaultValue: "Flow definition must contain nodes between start and end nodes",
		})
	}

	if err := validateInterceptors(flowDef.Interceptors); err != nil {
		return err
	}

	return nil
}

// validateInterceptors validates the interceptor definitions declared in a flow.
func validateInterceptors(interceptors []providers.InterceptorDefinition) *tidcommon.ServiceError {
	validModes := providers.ValidInterceptorModes
	validScopes := providers.ValidInterceptorScopes

	for i, ic := range interceptors {
		if ic.Name == "" {
			return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
				Key:          "error.flowmgtservice.interceptor_name_required",
				DefaultValue: fmt.Sprintf("Interceptor at index %d must have a name", i),
			})
		}
		if isDefaultInterceptor(ic.Name) {
			msg := fmt.Sprintf("Default interceptor '%s' cannot be configured in a flow definition", ic.Name)
			return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
				Key:          "error.flowmgtservice.interceptor_default_not_configurable",
				DefaultValue: msg,
			})
		}
		if !validModes[ic.Mode] {
			return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
				Key:          "error.flowmgtservice.interceptor_invalid_mode",
				DefaultValue: fmt.Sprintf("Interceptor '%s' has invalid mode '%s'", ic.Name, ic.Mode),
			})
		}
		if ic.Scope != "" && !validScopes[ic.Scope] {
			return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
				Key:          "error.flowmgtservice.interceptor_invalid_scope",
				DefaultValue: fmt.Sprintf("Interceptor '%s' has invalid scope '%s'", ic.Name, ic.Scope),
			})
		}
		if ic.Scope == providers.InterceptorScopeSelected && len(ic.ApplyTo) == 0 {
			return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
				Key:          "error.flowmgtservice.interceptor_selected_scope_requires_apply_to",
				DefaultValue: "Interceptor with scope SELECTED must specify at least one node in applyTo",
			})
		}
	}

	return nil
}

// isDefaultInterceptor checks whether the given interceptor name matches any default interceptor.
func isDefaultInterceptor(name string) bool {
	_, ok := interceptor.DefaultInterceptorNames[name]
	return ok
}

// validateInterceptorRegistration checks that every interceptor referenced in the flow
// is registered in the interceptor registry.
func (s *flowMgtService) validateInterceptorRegistration(
	interceptors []providers.InterceptorDefinition,
) *tidcommon.ServiceError {
	for _, ic := range interceptors {
		if !s.interceptorRegistry.IsRegistered(ic.Name) {
			return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
				Key:          "error.flowmgtservice.interceptor_not_registered",
				DefaultValue: fmt.Sprintf("Interceptor '%s' is not registered", ic.Name),
			})
		}
	}
	return nil
}

// isValidHandleFormat validates that the handle follows the required format:
// - all lowercase
// - alphanumeric characters
// - can contain underscores (_) or dashes (-)
// - cannot start or end with underscore or dash
func isValidHandleFormat(handle string) bool {
	return handleFormatRegex.MatchString(handle)
}

// tryInferRegistrationFlow attempts to infer and create a registration flow from an authentication flow
func (s *flowMgtService) tryInferRegistrationFlow(ctx context.Context, authFlowID string, authFlowDef *FlowDefinition) {
	logger := s.logger.With(log.String("authFlowID", authFlowID))

	if !config.GetServerRuntime().Config.Flow.AutoInferRegistration {
		logger.Debug(ctx, "Automatic registration flow inference is disabled")
		return
	}

	if authFlowDef.FlowType != providers.FlowTypeAuthentication {
		logger.Debug(ctx, "Flow is not an authentication flow, skipping registration inference",
			log.String("flowType", string(authFlowDef.FlowType)))
		return
	}

	// Check if auth flow already contains PasskeyAuthExecutor with registration modes
	// If so, skip registration flow inference as the auth flow handles registration internally
	if s.hasPasskeyRegistrationModes(authFlowDef) {
		logger.Debug(ctx, "Authentication flow contains PasskeyAuthExecutor with "+
			"register_start and register_finish modes, skipping registration inference")
		return
	}

	logger.Debug(ctx, "Inferring registration flow from authentication flow",
		log.String("flowName", authFlowDef.Name))

	regFlowDef, inferErr := s.inferenceService.InferRegistrationFlow(ctx, authFlowDef)
	if inferErr != nil {
		logger.Error(ctx, "Failed to infer registration flow", log.Error(inferErr))
		return
	}

	regFlowID, uuidErr := utils.GenerateUUIDv7()
	if uuidErr != nil {
		logger.Error(ctx, "Failed to generate UUID for inferred registration flow", log.Error(uuidErr))
		return
	}

	_, storeErr := s.store.CreateFlow(ctx, regFlowID, regFlowDef)
	if storeErr != nil {
		logger.Error(ctx, "Failed to create inferred registration flow", log.Error(storeErr))
		return
	}

	logger.Debug(ctx, "Successfully inferred and created registration flow",
		log.String("authFlowName", authFlowDef.Name), log.String("regFlowID", regFlowID),
		log.String("regFlowName", regFlowDef.Name))
}

// hasPasskeyRegistrationModes checks if the flow contains PasskeyAuthExecutor with both
// register_start and register_finish modes, indicating the auth flow handles passkey registration internally.
func (s *flowMgtService) hasPasskeyRegistrationModes(flowDef *FlowDefinition) bool {
	hasRegStart := false
	hasRegFinish := false

	for _, node := range flowDef.Nodes {
		if node.Executor != nil && node.Executor.Name == executor.ExecutorNamePasskeyAuth {
			switch node.Executor.Mode {
			case "register_start":
				hasRegStart = true
			case "register_finish":
				hasRegFinish = true
			}
		}
		// Early exit if both modes are found
		if hasRegStart && hasRegFinish {
			return true
		}
	}

	return hasRegStart && hasRegFinish
}
