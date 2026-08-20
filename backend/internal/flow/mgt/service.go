// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package flowmgt provides flow definition management functionality.
package flowmgt

import (
	"context"
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/flow/common"
	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executormeta"
	"github.com/thunder-id/thunderid/internal/flow/graphbuilder"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const loggerComponentName = "FlowMgtService"

var (
	errFlowHandleExists = errors.New("flow with handle already exists")
	errFlowIDExists     = errors.New("flow with id already exists")
	errClientValidation = errors.New("client validation failed")
)

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
	GetReachableCallTargets(ctx context.Context, flowID string) ([]CallTarget, *tidcommon.ServiceError)
	SetDependencyRegistry(r resourcedependency.Registry)
	GetFlowUsages(ctx context.Context, flowID string) (
		*resourcedependency.DependenciesResponse, *tidcommon.ServiceError)
	GetResourceDependencies(
		ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error)
	ResolveEffectiveFlowID(ctx context.Context, overriddenFlowID, ouID string, flowType providers.FlowType) (
		string, *tidcommon.ServiceError)
}

// ouProvider is the minimal subset of the OU service consumed by flowmgt for OU-level default
// flow resolution. Defined locally so flowmgt does not import internal/ou (avoids an import cycle
// since ou already depends on flowmgt via SetOUFlowResolver).
type ouProvider interface {
	GetOrganizationUnit(ctx context.Context, id string) (providers.OrganizationUnit, *tidcommon.ServiceError)
}

// flowMgtService is the default implementation of the FlowMgtServiceInterface.
type flowMgtService struct {
	store               flowStoreInterface
	inferenceService    flowInferenceServiceInterface
	graphBuilder        graphbuilder.GraphBuilderInterface
	executorRegistry    core.ExecutorMetadataProvider
	interceptorRegistry interceptor.InterceptorRegistryInterface
	flowValidator       FlowValidatorInterface
	compositeStore      *compositeFlowStore
	transactioner       providers.Transactioner
	dependencyRegistry  resourcedependency.Registry
	serverConfigSvc     serverConfigProvider
	ouSvc               ouProvider
	logger              *log.Logger
}

// newFlowMgtService creates a new instance of flowMgtService.
func newFlowMgtService(
	store flowStoreInterface,
	inferenceService flowInferenceServiceInterface,
	graphBuilder graphbuilder.GraphBuilderInterface,
	executorRegistry core.ExecutorMetadataProvider,
	interceptorRegistry interceptor.InterceptorRegistryInterface,
	flowValidator FlowValidatorInterface,
	compositeStore *compositeFlowStore,
	transactioner providers.Transactioner,
	serverConfigSvc serverConfigProvider,
	ouSvc ouProvider,
) FlowMgtServiceInterface {
	return &flowMgtService{
		store:               store,
		inferenceService:    inferenceService,
		graphBuilder:        graphBuilder,
		executorRegistry:    executorRegistry,
		interceptorRegistry: interceptorRegistry,
		flowValidator:       flowValidator,
		compositeStore:      compositeStore,
		transactioner:       transactioner,
		serverConfigSvc:     serverConfigSvc,
		ouSvc:               ouSvc,
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

	markManagedFlows(ctx, flows)

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
	if err := s.flowValidator.ValidateFlowDefinition(ctx, flowDef); err != nil {
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

// GetReachableCallTargets returns every flow reachable from flowID via CALL nodes, transitively.
// The starting flow itself is not included. Cycles are safe: each flow is visited at most once.
// A missing intermediate flow is treated as a hard error since it means the graph is unbuildable.
func (s *flowMgtService) GetReachableCallTargets(ctx context.Context, flowID string) (
	[]CallTarget, *tidcommon.ServiceError) {
	if flowID == "" {
		return nil, &ErrorMissingFlowID
	}

	visited := map[string]struct{}{flowID: {}}
	results := make([]CallTarget, 0)
	queue := []string{flowID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		flow, err := s.store.GetFlowByID(ctx, current)
		if err != nil {
			if errors.Is(err, errFlowNotFound) {
				if current == flowID {
					return nil, &ErrorFlowNotFound
				}
				return nil, &ErrorCallTargetFlowNotFound
			}
			s.logger.Error(ctx, "Failed to load flow while walking call targets",
				log.String(logKeyFlowID, current), log.Error(err))
			return nil, &tidcommon.InternalServerError
		}

		if current != flowID {
			results = append(results, CallTarget{
				FlowID:   flow.ID,
				FlowType: flow.FlowType,
			})
		}

		for i := range flow.Nodes {
			node := &flow.Nodes[i]
			if node.Type != string(common.NodeTypeCall) || node.Flow == nil || node.Flow.Ref == "" {
				continue
			}
			targetID := node.Flow.Ref
			if _, seen := visited[targetID]; seen {
				continue
			}
			visited[targetID] = struct{}{}
			queue = append(queue, targetID)
		}
	}

	return results, nil
}

// SetDependencyRegistry injects the dependency registry. Called by servicemanager after the
// provider services are initialized to avoid a cyclic import.
func (s *flowMgtService) SetDependencyRegistry(r resourcedependency.Registry) {
	s.dependencyRegistry = r
}

// GetFlowUsages returns the resources that reference this flow.
func (s *flowMgtService) GetFlowUsages(
	ctx context.Context, flowID string) (*resourcedependency.DependenciesResponse, *tidcommon.ServiceError) {
	if flowID == "" {
		return nil, &ErrorMissingFlowID
	}

	if _, err := s.store.GetFlowByID(ctx, flowID); err != nil {
		if errors.Is(err, errFlowNotFound) {
			return nil, &ErrorFlowNotFound
		}
		s.logger.Error(ctx, "Failed to get flow", log.String(logKeyFlowID, flowID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	if s.dependencyRegistry == nil {
		s.logger.Warn(ctx, "Dependency registry not set; returning unknown usages",
			log.String(logKeyFlowID, flowID))
		return &resourcedependency.DependenciesResponse{
			TotalResults: nil,
			Count:        0,
			Summary:      nil,
			Usages:       []resourcedependency.ResourceDependency{},
		}, nil
	}

	result, err := s.dependencyRegistry.GetDependencies(ctx, resourcedependency.ResourceTypeFlow, flowID)
	if err != nil {
		s.logger.Error(ctx, "Failed to get flow usages", log.String(logKeyFlowID, flowID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return result, nil
}

// GetResourceDependencies returns the flows that reference the resource identified by
// (resourceType, id). It implements the resourcedependency.Provider interface: an active flow
// references an identity provider or notification sender when one of its nodes carries the matching
// ID in its properties. Such a reference blocks deletion of the target, since the flow would break
// without it.
func (s *flowMgtService) GetResourceDependencies(
	ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error) {
	var propertyKey string
	switch resourceType {
	case resourcedependency.ResourceTypeIDP:
		propertyKey = nodePropertyKeyIDPID
	case resourcedependency.ResourceTypeNotificationSender:
		propertyKey = nodePropertyKeyNotificationSenderID
	default:
		return []resourcedependency.ResourceDependency{}, nil
	}

	flows, err := s.store.ListActiveFlowsWithNodes(ctx)
	if err != nil {
		s.logger.Error(ctx, "Failed to list flows for dependency lookup", log.Error(err))
		return nil, err
	}

	usages := make([]resourcedependency.ResourceDependency, 0)
	for _, flow := range flows {
		if flowReferencesResource(flow, propertyKey, id) {
			usages = append(usages, resourcedependency.ResourceDependency{
				ResourceType:     resourcedependency.ResourceTypeFlow,
				ID:               flow.ID,
				DisplayName:      flow.Name,
				BehaviorOnDelete: resourcedependency.BehaviorRestrict,
			})
		}
	}

	return usages, nil
}

// flowReferencesResource reports whether any node in the flow carries the given ID in the named
// property key.
func flowReferencesResource(flow *providers.CompleteFlowDefinition, propertyKey, id string) bool {
	for i := range flow.Nodes {
		if flow.Nodes[i].Properties == nil {
			continue
		}
		if val, ok := flow.Nodes[i].Properties[propertyKey].(string); ok && val == id {
			return true
		}
	}
	return false
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
	// A resource applied from the control plane is owned there. Changing it here would last only
	// until the next promotion overwrote it, so the change is refused instead.
	if svcErr := managedresource.Guard(ctx, managedresource.TypeFlow, flowID); svcErr != nil {
		return nil, svcErr
	}
	if flowID == "" {
		return nil, &ErrorMissingFlowID
	}
	if err := s.flowValidator.ValidateFlowDefinition(ctx, flowDef); err != nil {
		return nil, err
	}

	logger := s.logger.With(log.String(logKeyFlowID, flowID))

	var updatedFlow *providers.CompleteFlowDefinition
	var validationSvcErr *tidcommon.ServiceError
	var storeWriteAttempted bool
	var existingHandle string
	var existingType providers.FlowType
	txErr := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		existingFlow, err := s.store.GetFlowByID(txCtx, flowID)
		if err != nil {
			return err
		}
		existingHandle = existingFlow.Handle
		existingType = existingFlow.FlowType

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

		storeWriteAttempted = true
		var updateErr error
		updatedFlow, updateErr = s.store.UpdateFlow(txCtx, flowID, flowDef)
		if updateErr != nil {
			return updateErr
		}

		if s.dependencyRegistry != nil {
			if vErr := s.dependencyRegistry.ValidateReferenceUpdate(
				txCtx, resourcedependency.ResourceTypeFlow, flowID); vErr != nil {
				if vErr.Type == tidcommon.ClientErrorType {
					logger.Debug(ctx, "Flow update blocked by dependent resource validation",
						log.String("dependentCode", vErr.Code))
					validationSvcErr = &ErrorFlowUpdateBlockedByDependent
					return errClientValidation
				}

				return fmt.Errorf("failed to validate flow update against dependent resources: code=%s, error=%s",
					vErr.Code, vErr.ErrorDescription)
			}
		}

		return nil
	})
	// If the store write was attempted, both the flow-store cache and the graph-builder cache may
	// have been populated with the uncommitted new definition during dependent-resource
	// validation (mid-transaction reads see the write inside the same tx). Whether the transaction
	// committed or rolled back, purge both caches so subsequent reads rebuild from the on-disk row.
	if storeWriteAttempted {
		s.store.InvalidateCache(ctx, flowID, existingHandle, existingType)
		s.graphBuilder.InvalidateCache(ctx, flowID)
	}

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

	return updatedFlow, nil
}

// DeleteFlow deletes a flow definition and all its version history.
func (s *flowMgtService) DeleteFlow(ctx context.Context, flowID string) *tidcommon.ServiceError {
	// A resource applied from the control plane is owned there. Changing it here would last only
	// until the next promotion overwrote it, so the change is refused instead.
	if svcErr := managedresource.Guard(ctx, managedresource.TypeFlow, flowID); svcErr != nil {
		return svcErr
	}
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

// ResolveEffectiveFlowID resolves the effective flow ID to use based on the provided overridden flow ID,
// organization unit ID, and flow type.
func (s *flowMgtService) ResolveEffectiveFlowID(ctx context.Context, overriddenFlowID, ouID string,
	flowType providers.FlowType) (string, *tidcommon.ServiceError) {
	if overriddenFlowID != "" {
		return overriddenFlowID, nil
	}

	if ouID != "" && s.ouSvc != nil {
		ou, ouErr := s.ouSvc.GetOrganizationUnit(ctx, ouID)
		if ouErr != nil {
			s.logger.Warn(ctx, "Failed to look up OU for flow resolution; falling back to server default",
				log.String("ouID", ouID), log.String("error", ouErr.Error.DefaultValue))
		} else if id := ouFlowIDForType(ou, flowType); id != "" {
			return id, nil
		}
	}

	handle := s.resolveDefaultFlowHandle(ctx, flowType)
	if handle == "" {
		return "", nil
	}

	flow, svcErr := s.GetFlowByHandle(ctx, handle, flowType)
	if svcErr != nil {
		return "", svcErr
	}

	return flow.ID, nil
}

// ouFlowIDForType returns the flow ID for the given flow type from the organization unit.
func ouFlowIDForType(ou providers.OrganizationUnit, flowType providers.FlowType) string {
	switch flowType {
	case providers.FlowTypeAuthentication:
		return ou.AuthFlowID
	case providers.FlowTypeRegistration:
		return ou.RegistrationFlowID
	case providers.FlowTypeUserOnboarding:
		return ou.UserOnboardingFlowID
	case providers.FlowTypeRecovery:
		return ou.RecoveryFlowID
	case providers.FlowTypeSignOut:
		return ou.SignOutFlowID
	}
	return ""
}

// getFlowSectionConfig returns the merged FlowSectionConfig from the server-config "flow" section.
func (s *flowMgtService) getFlowSectionConfig(ctx context.Context) flowconfig.FlowSectionConfig {
	if s.serverConfigSvc == nil {
		return flowconfig.FlowSectionConfig{}
	}

	merged, svcErr := s.serverConfigSvc.GetMergedConfig(ctx, "flow")
	if svcErr != nil {
		s.logger.Warn(ctx, "Failed to read flow server config; using empty section defaults")
		return flowconfig.FlowSectionConfig{}
	}

	cfg, ok := merged.(flowconfig.FlowSectionConfig)
	if !ok {
		return flowconfig.FlowSectionConfig{}
	}

	return cfg
}

// resolveDefaultFlowHandle returns the server-level default handle configured for the given flow
// type, or "" when no default is configured.
func (s *flowMgtService) resolveDefaultFlowHandle(ctx context.Context, flowType providers.FlowType) string {
	cfg := s.getFlowSectionConfig(ctx)

	switch flowType {
	case providers.FlowTypeAuthentication:
		return cfg.AuthFlow.DefaultHandle
	case providers.FlowTypeRegistration:
		return cfg.RegistrationFlow.DefaultHandle
	case providers.FlowTypeUserOnboarding:
		return cfg.UserOnboardingFlow.DefaultHandle
	case providers.FlowTypeRecovery:
		return cfg.RecoveryFlow.DefaultHandle
	case providers.FlowTypeSignOut:
		return cfg.SignOutFlow.DefaultHandle
	default:
		return ""
	}
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
		if node.Executor != nil && node.Executor.Name == executormeta.ExecutorNamePasskeyAuth {
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

// markManagedFlows reports the control plane owned flows as read only, which is what a client renders
// its edit and delete controls from.
func markManagedFlows(ctx context.Context, flows []BasicFlowDefinition) {
	managed := managedresource.Default().ManagedIDs(ctx, managedresource.TypeFlow)
	if len(managed) == 0 {
		return
	}
	for i := range flows {
		if managed[flows[i].ID] {
			flows[i].IsReadOnly = true
		}
	}
}
