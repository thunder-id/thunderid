// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package flowmgt provides flow definition management functionality.
package flowmgt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/connection"
	"github.com/thunder-id/thunderid/internal/flow/common"
	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executormeta"
	"github.com/thunder-id/thunderid/internal/flow/graphbuilder"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	"github.com/thunder-id/thunderid/internal/notificationtemplate"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/filter"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
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
	GetSystemFlow(ctx context.Context, handle string) (*providers.CompleteFlowDefinition, *tidcommon.ServiceError)
	ResolveFlowOU(ctx context.Context, flowDef *FlowDefinition) *tidcommon.ServiceError

	// ListFlowsForOU lists the flows an organization unit owns and the ones shared with it.
	ListFlowsForOU(ctx context.Context, ouID string, flowType providers.FlowType, limit, offset int) (
		*FlowListResponse, *tidcommon.ServiceError)
	// GetFlowForOU returns a flow as one organization unit sees it: whole when it owns it, its variables when the
	// flow was shared with it.
	GetFlowForOU(ctx context.Context, flowID, ouID string) (*FlowView, *tidcommon.ServiceError)
	// UpdateFlowWithSharing updates a flow and replaces its sharing policies with the ones a document carries, in one
	// transaction. The policies are checked against the new definition, in place of the policies the flow had, so a
	// document whose definition and policies agree with each other is accepted whatever the flow held before.
	UpdateFlowWithSharing(
		ctx context.Context, flowID string, flowDef *FlowDefinition, policies []sharing.PolicyRequest,
	) (*providers.CompleteFlowDefinition, *tidcommon.ServiceError)
	// CheckFlowAccess checks that an organization unit may read, or with write change, the definition of a flow.
	CheckFlowAccess(ctx context.Context, flowID, ouID string, write bool) *tidcommon.ServiceError

	CreateSharingPolicy(ctx context.Context, flowID string, req sharing.PolicyRequest) (
		sharing.Policy, *tidcommon.ServiceError)
	GetSharingPolicy(ctx context.Context, flowID, policyID string) (sharing.Policy, *tidcommon.ServiceError)
	ListSharingPolicies(ctx context.Context, flowID string, limit, offset int) (
		sharing.PolicyList, *tidcommon.ServiceError)
	UpdateSharingPolicy(ctx context.Context, flowID, policyID string, req sharing.PolicyRequest) (
		sharing.Policy, *tidcommon.ServiceError)
	DeleteSharingPolicy(ctx context.Context, flowID, policyID string) *tidcommon.ServiceError
	// ValidateSharingPolicies checks the terms policies state for the variables of a flow, without recording them.
	ValidateSharingPolicies(
		ctx context.Context, flowID string, policies []sharing.PolicyRequest) *tidcommon.ServiceError
	// GetFlowOverlayRules answers what an organization unit may do with the variables of a flow.
	GetFlowOverlayRules(ctx context.Context, flowID, ouID string) (sharing.ResolvedOverlay, *tidcommon.ServiceError)
	// GetVariables returns the values an organization unit has set for the variables of a flow.
	GetVariables(ctx context.Context, flowID, ouID string) (*VariablesResponse, *tidcommon.ServiceError)
	// SetVariables sets the values an organization unit holds for the variables of a flow.
	SetVariables(ctx context.Context, flowID, ouID string, req SetVariablesRequest) (
		*VariablesResponse, *tidcommon.ServiceError)
	// GetEffectiveVariableValues returns the raw value of every variable of a flow that is not at the owner's
	// default for an organization unit.
	GetEffectiveVariableValues(
		ctx context.Context, flowID string, app providers.Application, ouID string,
	) (map[string]string, *tidcommon.ServiceError)
}

// ouProvider is the minimal subset of the OU service consumed by flowmgt. It reads the organization unit's
// per-flow-type default flow IDs for OU-level default flow resolution, which belong to the management model rather
// than the runtime provider contract, and it checks and resolves the organization unit a flow is owned by. If those
// IDs are ever promoted into providers.OrganizationUnit, this can return the contract type and the internal/ou
// import here goes away.
type ouProvider interface {
	GetOrganizationUnit(ctx context.Context, id string) (ou.OrganizationUnit, *tidcommon.ServiceError)
	ResolveOwner(ctx context.Context, ouID, ouHandle string, errs ou.OwnerErrors) (string, *tidcommon.ServiceError)
	ResolveOwnerForUpdate(
		ctx context.Context, requestedID, requestedHandle, currentID string, errs ou.OwnerErrors,
	) (string, *tidcommon.ServiceError)
}

// flowOwnerErrors are the errors given when the organization unit of a flow is refused.
var flowOwnerErrors = ou.OwnerErrors{
	Required: ErrorFlowOURequired,
	NotFound: ErrorInvalidFlowOU,
	Changed:  ErrorOUUpdateNotAllowed,
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
	templateResolver    notificationTemplateProvider
	serverConfigSvc     serverConfigProvider
	ouSvc               ouProvider
	// systemFlows are the system flows, by handle, as validated at startup.
	systemFlows map[string]json.RawMessage
	sharingSvc  sharing.SharingServiceInterface
	idps        idpProvider
	logger      *log.Logger
}

// notificationTemplateProvider maps a template handle (within a channel) to its id.
type notificationTemplateProvider interface {
	GetTemplateByHandle(ctx context.Context, channel notificationtemplate.ChannelType, handle string) (
		*notificationtemplate.Template, *tidcommon.ServiceError)
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
	templateResolver notificationTemplateProvider,
	systemFlows map[string]json.RawMessage,
	sharingSvc sharing.SharingServiceInterface,
	idps idpProvider,
) *flowMgtService {
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
		templateResolver:    templateResolver,
		systemFlows:         systemFlows,
		sharingSvc:          sharingSvc,
		idps:                idps,
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

	for i := range flows {
		flows[i].IsOwnedByOU = new(true)
	}

	listResponse := &FlowListResponse{
		TotalResults: totalCount,
		StartIndex:   offset + 1,
		Count:        len(flows),
		Flows:        flows,
		Links:        buildPaginationLinks(limit, offset, totalCount, flowTypeQueryParam(flowType)),
	}

	return listResponse, nil
}

// CreateFlow creates a new flow definition with version 1.
func (s *flowMgtService) CreateFlow(ctx context.Context, flowDef *FlowDefinition) (
	*providers.CompleteFlowDefinition, *tidcommon.ServiceError) {
	if err := s.flowValidator.ValidateFlowDefinition(ctx, flowDef); err != nil {
		return nil, err
	}
	if svcErr := s.ResolveFlowOU(ctx, flowDef); svcErr != nil {
		return nil, svcErr
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

// ResolveFlowOU sets the organization unit a flow is owned by. A flow has to name one, by id or by handle, and it has
// to exist. The handle is replaced by the id it resolves to. It is only recorded: nothing restricts what an
// organization unit can do with a flow it does not own.
func (s *flowMgtService) ResolveFlowOU(ctx context.Context, flowDef *FlowDefinition) *tidcommon.ServiceError {
	ouID, svcErr := s.ouSvc.ResolveOwner(ctx, flowDef.OUID, flowDef.OUHandle, flowOwnerErrors)
	if svcErr != nil {
		return svcErr
	}
	flowDef.OUID = ouID
	flowDef.OUHandle = ""
	return nil
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
// ID in its properties, an HTTP connection when one of its extension steps does, and a notification
// template when a node carries the matching email/SMS template handle. An organization unit is used by the flows
// it owns and by the flows whose nodes name it as their parentOuId. Such a reference blocks deletion of the
// target, since the flow would break without it.
func (s *flowMgtService) GetResourceDependencies(
	ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error) {
	// Templates are referenced by handle, not id, so they need a channel-aware path.
	if resourceType == resourcedependency.ResourceTypeNotificationTemplate {
		return s.notificationTemplateUsages(ctx, id)
	}
	if resourceType == resourcedependency.ResourceTypeOU {
		return s.organizationUnitUsages(ctx, id)
	}

	var propertyKey, executorName string
	switch resourceType {
	case resourcedependency.ResourceTypeIDP:
		return s.identityProviderUsages(ctx, id)
	case resourcedependency.ResourceTypeNotificationSender:
		propertyKey = nodePropertyKeyNotificationSenderID
	case resourcedependency.ResourceTypeHTTPConnection:
		propertyKey, executorName = common.NodePropertyConnectionID, executormeta.ExecutorNameExtension
	default:
		return []resourcedependency.ResourceDependency{}, nil
	}

	return s.referencingFlows(ctx, executorName, propertyKey, id)
}

// identityProviderUsages lists the active flows that use a connection: through a node that carries its ID, or through
// a variable that names it. A variable names it as its default, in a sharing policy of the flow that gives it as a
// value, or in a value an organization unit the flow was shared with saved for it. The flow is listed once whichever
// way it uses the connection. Flows that were only shared into an organization unit are not looked at for themselves:
// the flow is the owner's, and what a unit holds of it is its saved values.
func (s *flowMgtService) identityProviderUsages(
	ctx context.Context, id string) ([]resourcedependency.ResourceDependency, error) {
	flows, err := s.store.ListActiveFlowsWithNodes(ctx)
	if err != nil {
		// The registry logs provider errors; don't double-log here.
		return nil, err
	}

	usages := make([]resourcedependency.ResourceDependency, 0)
	for _, flow := range flows {
		used := flowReferencesResource(flow, "", nodePropertyKeyIDPID, id)
		if !used {
			if used, err = s.flowVariablesUseConnection(ctx, flow, id); err != nil {
				return nil, err
			}
		}
		if used {
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

// referencingFlows lists the active flows whose nodes carry the given ID in the named property key. A non-empty
// executorName counts only the nodes that run that executor.
func (s *flowMgtService) referencingFlows(
	ctx context.Context, executorName, propertyKey, id string) ([]resourcedependency.ResourceDependency, error) {
	flows, err := s.store.ListActiveFlowsWithNodes(ctx)
	if err != nil {
		// The registry logs provider errors; don't double-log here.
		return nil, err
	}

	usages := make([]resourcedependency.ResourceDependency, 0)
	for _, flow := range flows {
		if flowReferencesResource(flow, executorName, propertyKey, id) {
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

// organizationUnitUsages lists the flows that block the deletion of an organization unit: the flows it owns, since a
// flow cannot exist without its owner, and the flows whose nodes name it as their parentOuId, since those would fail
// to resolve it. A flow that does both is listed once.
func (s *flowMgtService) organizationUnitUsages(
	ctx context.Context, ouID string) ([]resourcedependency.ResourceDependency, error) {
	usages, err := s.ownedFlows(ctx, ouID)
	if err != nil {
		return nil, err
	}
	referencing, err := s.referencingFlows(ctx, "", common.NodePropertyOUParent, ouID)
	if err != nil {
		return nil, err
	}

	listed := make(map[string]struct{}, len(usages))
	for _, usage := range usages {
		listed[usage.ID] = struct{}{}
	}
	for _, usage := range referencing {
		if _, ok := listed[usage.ID]; !ok {
			usages = append(usages, usage)
		}
	}

	return usages, nil
}

// ownedFlows lists the flows an organization unit owns, which block the deletion of the organization unit.
func (s *flowMgtService) ownedFlows(ctx context.Context, ouID string) ([]resourcedependency.ResourceDependency, error) {
	owned := make([]resourcedependency.ResourceDependency, 0)
	for offset := 0; ; offset += maxPageSize {
		flows, total, err := s.store.ListFlows(ctx, maxPageSize, offset, "")
		if err != nil {
			// The registry logs provider errors; don't double-log here.
			return nil, err
		}
		for _, flow := range flows {
			if flow.OUID == ouID {
				owned = append(owned, resourcedependency.ResourceDependency{
					ResourceType:     resourcedependency.ResourceTypeFlow,
					ID:               flow.ID,
					DisplayName:      flow.Name,
					BehaviorOnDelete: resourcedependency.BehaviorRestrict,
				})
			}
		}
		if len(flows) == 0 || offset+len(flows) >= total {
			return owned, nil
		}
	}
}

// flowReferencesResource reports whether any node in the flow carries the given ID, ignoring
// surrounding whitespace, in the named property key. A non-empty executorName counts only the nodes
// that run that executor.
func flowReferencesResource(flow *providers.CompleteFlowDefinition, executorName, propertyKey, id string) bool {
	for i := range flow.Nodes {
		if flow.Nodes[i].Properties == nil {
			continue
		}
		if executorName != "" &&
			(flow.Nodes[i].Executor == nil || flow.Nodes[i].Executor.Name != executorName) {
			continue
		}
		if val, ok := flow.Nodes[i].Properties[propertyKey].(string); ok && strings.TrimSpace(val) == id {
			return true
		}
	}
	return false
}

// notificationTemplateUsages reports active flows that reference the template id. References are by
// handle, so each is resolved to an id and matched. The resolver is always injected by the service
// manager; a missing one is a wiring fault and fails closed (errors out) so the registry reports
// usage as unknown and the delete is blocked rather than letting a referenced template be removed.
func (s *flowMgtService) notificationTemplateUsages(ctx context.Context, id string) (
	[]resourcedependency.ResourceDependency, error) {
	if s.templateResolver == nil {
		// The registry logs provider errors; return the error so the delete fails closed.
		return nil, fmt.Errorf("notification template resolver not configured")
	}

	flows, err := s.store.ListActiveFlowsWithNodes(ctx)
	if err != nil {
		// The registry logs provider errors; don't double-log here.
		return nil, err
	}

	// Memoize handle->id within this call; a template may be referenced by many nodes.
	resolved := make(map[string]string)
	resolve := func(channel notificationtemplate.ChannelType, handle string) (string, error) {
		key := string(channel) + ":" + handle
		if tid, ok := resolved[key]; ok {
			return tid, nil
		}
		tmpl, svcErr := s.templateResolver.GetTemplateByHandle(ctx, channel, handle)
		if svcErr != nil {
			if svcErr.Code == notificationtemplate.ErrorTemplateNotFound.Code {
				// Dangling handle: cannot be the target. Cache the miss.
				resolved[key] = ""
				return "", nil
			}
			return "", fmt.Errorf("resolve template %s/%s: %s", channel, handle, svcErr.Code)
		}
		resolved[key] = tmpl.ID
		return tmpl.ID, nil
	}

	usages := make([]resourcedependency.ResourceDependency, 0)
	for _, flow := range flows {
		references, err := flowReferencesTemplate(flow, id, resolve)
		if err != nil {
			return nil, err
		}
		if references {
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

// flowReferencesTemplate reports whether any node references template id via its email/SMS template
// handle property.
func flowReferencesTemplate(flow *providers.CompleteFlowDefinition, id string,
	resolve func(notificationtemplate.ChannelType, string) (string, error)) (bool, error) {
	channelByProp := map[string]notificationtemplate.ChannelType{
		nodePropertyKeyEmailTemplate: notificationtemplate.ChannelTypeEmail,
		nodePropertyKeySMSTemplate:   notificationtemplate.ChannelTypeSMS,
	}
	for i := range flow.Nodes {
		if flow.Nodes[i].Properties == nil {
			continue
		}
		for prop, channel := range channelByProp {
			handle, ok := flow.Nodes[i].Properties[prop].(string)
			if !ok || handle == "" {
				continue
			}
			tid, err := resolve(channel, handle)
			if err != nil {
				return false, err
			}
			if tid != "" && tid == id {
				return true, nil
			}
		}
	}
	return false, nil
}

// GetSystemFlow returns the system flow with the given handle. Each call decodes a fresh copy, so a
// caller changing it changes nothing another call sees.
func (s *flowMgtService) GetSystemFlow(_ context.Context, handle string) (
	*providers.CompleteFlowDefinition, *tidcommon.ServiceError) {
	data, ok := s.systemFlows[handle]
	if !ok {
		return nil, &ErrorFlowNotFound
	}
	var flow providers.CompleteFlowDefinition
	if err := json.Unmarshal(data, &flow); err != nil {
		return nil, &tidcommon.InternalServerError
	}
	return &flow, nil
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
	return s.updateFlow(ctx, flowID, flowDef, nil)
}

// UpdateFlowWithSharing updates a flow and replaces its sharing policies with the ones a document carries, in one
// transaction. Without policies it is UpdateFlow.
func (s *flowMgtService) UpdateFlowWithSharing(
	ctx context.Context, flowID string, flowDef *FlowDefinition, policies []sharing.PolicyRequest,
) (*providers.CompleteFlowDefinition, *tidcommon.ServiceError) {
	return s.updateFlow(ctx, flowID, flowDef, policies)
}

// updateFlow is UpdateFlow, optionally with the sharing policies of a document. Policies a document states take the
// place of the checks of the new definition against the policies and saved values the flow holds: they are checked
// against the new definition and applied, and then what the flow holds is checked, so only a conflict that survives
// the document is refused.
func (s *flowMgtService) updateFlow(
	ctx context.Context, flowID string, flowDef *FlowDefinition, policies []sharing.PolicyRequest,
) (*providers.CompleteFlowDefinition, *tidcommon.ServiceError) {
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

		// A flow is changed through the organization unit that owns it. One that was only shared with the unit named
		// in the request is read only to it.
		if flowDef.OUID != "" && flowDef.OUID != existingFlow.OUID {
			visible, sharingErr := s.sharingSvc.IsVisible(txCtx, FlowSharingType, flowID, flowDef.OUID)
			if sharingErr != nil {
				return fmt.Errorf("failed to check whether the flow is shared: code=%s", sharingErr.Code)
			}
			if visible {
				validationSvcErr = &ErrorSharedFlowReadOnly
				return errClientValidation
			}
		}

		// The organization unit is kept when left out, and cannot be changed when given.
		ouID, ouErr := s.ouSvc.ResolveOwnerForUpdate(
			txCtx, flowDef.OUID, flowDef.OUHandle, existingFlow.OUID, flowOwnerErrors)
		if ouErr != nil {
			if ouErr.Type == tidcommon.ClientErrorType {
				validationSvcErr = ouErr
				return errClientValidation
			}
			return fmt.Errorf("failed to check the organization unit of the flow: code=%s", ouErr.Code)
		}
		flowDef.OUID = ouID
		flowDef.OUHandle = ""

		// A variable the flow no longer declares takes its rules and saved values with it. The ones it still
		// declares have to still satisfy the sharing policies recorded for it, and the values organization units
		// saved for them.
		if svcErr := s.dropRemovedVariables(txCtx, flowID, existingFlow, flowDef.Nodes); svcErr != nil {
			validationSvcErr = svcErr
			return errClientValidation
		}
		if len(policies) == 0 {
			if svcErr := s.checkHeldSharing(txCtx, flowID, flowDef.Nodes); svcErr != nil {
				validationSvcErr = svcErr
				return errClientValidation
			}
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

		if len(policies) > 0 {
			if svcErr := s.replaceSharing(txCtx, flowID, existingFlow.OUID, flowDef.Nodes, policies); svcErr != nil {
				validationSvcErr = svcErr
				return errClientValidation
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
	if flowID == "" {
		return &ErrorMissingFlowID
	}

	logger := s.logger.With(log.String(logKeyFlowID, flowID))

	existingFlow, err := s.store.GetFlowByID(ctx, flowID)
	if err != nil {
		if errors.Is(err, errFlowNotFound) {
			// A flow that is already gone is deleted; what is left of its sharing is not.
			return s.purgeOrphanedSharing(ctx, flowID)
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

	// Invalidate the cached graph since the flow has been deleted
	s.graphBuilder.InvalidateCache(ctx, flowID)

	if svcErr := s.removeSharing(ctx, flowID); svcErr != nil {
		logger.Error(ctx, "Failed to remove the sharing policies of a deleted flow", log.String("code", svcErr.Code))
		return svcErr
	}

	logger.Debug(ctx, "Flow deleted successfully")

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
	var conflictSvcErr *tidcommon.ServiceError
	txErr := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		target, err := s.store.GetFlowVersion(txCtx, flowID, version)
		if err != nil {
			return err
		}

		// A restore that takes a variable out of the flow drops its rules and saved values, and the version has to
		// satisfy the sharing policies recorded for the flow and the values organization units saved for its other
		// variables, as an update does.
		current, err := s.store.GetFlowByID(txCtx, flowID)
		if err != nil {
			return err
		}
		if svcErr := s.dropRemovedVariables(txCtx, flowID, current, target.Nodes); svcErr != nil {
			conflictSvcErr = svcErr
			return errClientValidation
		}
		if svcErr := s.checkSharingPolicies(txCtx, flowID, target.Nodes); svcErr != nil {
			conflictSvcErr = svcErr
			return errClientValidation
		}
		if svcErr := s.checkSavedValues(txCtx, flowID, target.Nodes); svcErr != nil {
			conflictSvcErr = svcErr
			return errClientValidation
		}

		restoredFlow, err = s.store.RestoreFlowVersion(txCtx, flowID, version)
		return err
	})
	if txErr != nil {
		if errors.Is(txErr, errClientValidation) {
			return nil, conflictSvcErr
		}
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
func ouFlowIDForType(orgUnit ou.OrganizationUnit, flowType providers.FlowType) string {
	switch flowType {
	case providers.FlowTypeAuthentication:
		return orgUnit.AuthFlowID
	case providers.FlowTypeRegistration:
		return orgUnit.RegistrationFlowID
	case providers.FlowTypeUserOnboarding:
		return orgUnit.UserOnboardingFlowID
	case providers.FlowTypeRecovery:
		return orgUnit.RecoveryFlowID
	case providers.FlowTypeSignOut:
		return orgUnit.SignOutFlowID
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
func buildPaginationLinks(limit, offset, totalCount int, extraQuery string) []Link {
	links := make([]Link, 0)

	// Add first and previous links if not on first page
	if offset > 0 {
		links = append(links, Link{
			Href: fmt.Sprintf("/flows?offset=0&limit=%d%s", limit, extraQuery),
			Rel:  "first",
		})

		prevOffset := offset - limit
		if prevOffset < 0 {
			prevOffset = 0
		}
		links = append(links, Link{
			Href: fmt.Sprintf("/flows?offset=%d&limit=%d%s", prevOffset, limit, extraQuery),
			Rel:  "prev",
		})
	}

	// Add next link if there are more results
	if offset+limit < totalCount {
		nextOffset := offset + limit
		links = append(links, Link{
			Href: fmt.Sprintf("/flows?offset=%d&limit=%d%s", nextOffset, limit, extraQuery),
			Rel:  "next",
		})
	}

	// Add last link if not on last page
	lastPageOffset := ((totalCount - 1) / limit) * limit
	if totalCount > 0 && offset < lastPageOffset {
		links = append(links, Link{
			Href: fmt.Sprintf("/flows?offset=%d&limit=%d%s", lastPageOffset, limit, extraQuery),
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

	// The inferred flow belongs to the organization unit of the flow it was inferred from.
	regFlowDef.OUID = authFlowDef.OUID

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

// ---------------------------------------------------------------------------
// Organization unit access
// ---------------------------------------------------------------------------
// requireVisibleFlow fetches a flow for an organization unit that owns it or was shared it. A flow it neither owns nor
// was shared is reported as not found, the same as one that does not exist, so the answer does not say which flows
// exist.
func (s *flowMgtService) requireVisibleFlow(
	ctx context.Context, flowID, ouID string,
) (*providers.CompleteFlowDefinition, *tidcommon.ServiceError) {
	if ouID == "" {
		return nil, &ErrorMissingOUID
	}
	flow, svcErr := s.GetFlow(ctx, flowID)
	if svcErr != nil {
		return nil, svcErr
	}
	if flow.OUID == ouID {
		return flow, nil
	}
	visible, svcErr := s.sharingSvc.IsVisible(ctx, FlowSharingType, flowID, ouID)
	if svcErr != nil {
		return nil, translateSharingError(svcErr)
	}
	if !visible {
		return nil, &ErrorFlowNotFound
	}
	return flow, nil
}

// GetFlowForOU returns a flow as one organization unit sees it. The one that owns it sees the whole definition; one
// it was shared with sees the variables that apply to it, resolved for it, and nothing else. A flow the organization
// unit neither owns nor was shared is reported as not found, the same as one that does not exist.
func (s *flowMgtService) GetFlowForOU(
	ctx context.Context, flowID, ouID string,
) (*FlowView, *tidcommon.ServiceError) {
	flow, svcErr := s.requireVisibleFlow(ctx, flowID, ouID)
	if svcErr != nil {
		return nil, svcErr
	}
	if flow.OUID == ouID {
		return &FlowView{Owned: &OwnedFlow{CompleteFlowDefinition: flow, IsOwnedByOU: true}}, nil
	}
	variables, svcErr := s.sharedVariables(ctx, flow, ouID)
	if svcErr != nil {
		return nil, svcErr
	}
	return &FlowView{Shared: &SharedFlow{
		ID:          flow.ID,
		Handle:      flow.Handle,
		Name:        flow.Name,
		FlowType:    flow.FlowType,
		OUID:        flow.OUID,
		IsOwnedByOU: false,
		IsReadOnly:  true,
		Variables:   variables,
	}}, nil
}

// CheckFlowAccess checks that an organization unit may read, or with write change, the definition of a flow: only the
// one that owns it may. A flow the unit was only shared with answers that it is not found for a read and that it is
// read only for a write. With no organization unit named, nothing is checked.
func (s *flowMgtService) CheckFlowAccess(
	ctx context.Context, flowID, ouID string, write bool,
) *tidcommon.ServiceError {
	if ouID == "" {
		return nil
	}
	flow, svcErr := s.requireVisibleFlow(ctx, flowID, ouID)
	if svcErr != nil {
		return svcErr
	}
	if flow.OUID == ouID {
		return nil
	}
	if write {
		return &ErrorSharedFlowReadOnly
	}
	return &ErrorFlowNotFound
}

// ListFlowsForOU lists the flows an organization unit owns, followed by the ones that were shared with it, each entry
// saying which in isOwnedByOU. A flow shared with it is read only to it. A flow it neither owns nor was shared is
// not listed. An organization unit the caller may not act for has no flows to show.
func (s *flowMgtService) ListFlowsForOU(
	ctx context.Context, ouID string, flowType providers.FlowType, limit, offset int,
) (*FlowListResponse, *tidcommon.ServiceError) {
	if ouID == "" {
		return nil, &ErrorMissingOUID
	}
	if flowType != "" && !isValidFlowType(flowType) {
		return nil, &ErrorInvalidFlowType
	}
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	if offset < 0 {
		offset = 0
	}
	extraQuery := utils.FilterQueryParam(map[string]interface{}{filter.AttributeOUID: ouID}) +
		flowTypeQueryParam(flowType)

	// One page of what the unit owns, with how many it owns, then what was shared with it, read by ID.
	ownedFirst, ownedCount, err := s.store.ListFlowsByOU(ctx, ouID, limit, offset, string(flowType))
	if err != nil {
		s.logger.Error(ctx, "Failed to list the flows an organization unit owns", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	sharedIDs := make([]string, 0)
	shared := make(map[string]BasicFlowDefinition)
	visible, svcErr := s.sharingSvc.ListVisibleResourceIDs(ctx, FlowSharingType, ouID)
	if svcErr != nil && svcErr.Code != ou.ErrorOrganizationUnitNotFound.Code {
		return nil, translateSharingError(svcErr)
	}
	var found []BasicFlowDefinition
	if len(visible) > 0 {
		found, err = s.store.ListFlowsByIDs(ctx, visible, string(flowType))
	}
	if err != nil {
		s.logger.Error(ctx, "Failed to read the flows shared with an organization unit", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	for _, flow := range found {
		if flow.OUID != ouID {
			shared[flow.ID] = flow
		}
	}
	// The order the sharing framework gives is kept, and a shared flow that no longer exists is left out.
	for _, id := range visible {
		if _, ok := shared[id]; ok {
			sharedIDs = append(sharedIDs, id)
		}
	}

	ownedPage, sharedPage, err := utils.PaginateOwnedThenShared(
		ownedCount,
		// The page of owned flows is the one already read, as the framework asks for the part of the page the owned
		// flows fill, which is what that read returned.
		func(limit, _ int) ([]BasicFlowDefinition, error) {
			return ownedFirst[:min(limit, len(ownedFirst))], nil
		},
		sharedIDs,
		func(ids []string) ([]BasicFlowDefinition, error) {
			flows := make([]BasicFlowDefinition, 0, len(ids))
			for _, id := range ids {
				flows = append(flows, shared[id])
			}
			return flows, nil
		},
		func(flow BasicFlowDefinition) string { return flow.ID },
		limit, offset,
	)
	if err != nil {
		s.logger.Error(ctx, "Failed to page the flows of an organization unit", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	entries := make([]BasicFlowDefinition, 0, len(ownedPage)+len(sharedPage))
	for _, flow := range ownedPage {
		flow.IsOwnedByOU = new(true)
		entries = append(entries, flow)
	}
	for _, flow := range sharedPage {
		// A flow shared with the unit is read only to it.
		flow.IsReadOnly = true
		flow.IsOwnedByOU = new(false)
		entries = append(entries, flow)
	}
	return buildFlowListResponse(entries, ownedCount+len(sharedIDs), limit, offset, extraQuery), nil
}

// buildFlowListResponse builds the response for one page of a flow list.
func buildFlowListResponse(
	flows []BasicFlowDefinition, total, limit, offset int, extraQuery string,
) *FlowListResponse {
	return &FlowListResponse{
		TotalResults: total,
		StartIndex:   offset + 1,
		Count:        len(flows),
		Flows:        flows,
		Links:        buildPaginationLinks(limit, offset, total, extraQuery),
	}
}

// flowTypeQueryParam returns the query string fragment that keeps a flow type filter on pagination links.
func flowTypeQueryParam(flowType providers.FlowType) string {
	if flowType == "" {
		return ""
	}
	return "&" + queryParamFlowType + "=" + url.QueryEscape(string(flowType))
}

// ---------------------------------------------------------------------------
// Variables
// ---------------------------------------------------------------------------
// declaredVariables reads the variables a flow declares, on its VARIABLES nodes.
func declaredVariables(flow *providers.CompleteFlowDefinition) []providers.VariableDefinition {
	var all []providers.VariableDefinition
	for _, n := range flow.Nodes {
		if n.Type == string(common.NodeTypeVariables) {
			all = append(all, n.Inputs...)
		}
	}
	return all
}

// jsonValue turns a raw value into a value of the variable's own type, as JSON carries it.
func jsonValue(def providers.VariableDefinition, raw string) (interface{}, error) {
	encoded, err := core.EncodeRuntime(def, raw)
	if err != nil {
		return nil, err
	}
	var v interface{}
	if err := json.Unmarshal([]byte(encoded), &v); err != nil {
		return nil, err
	}
	return v, nil
}

// pinnedValue is the single value a rule carries, whether it pins the variable or only starts it there.
func pinnedValue(rule sharing.OverlayRule) (string, bool) {
	if rule.Value == nil || len(*rule.Value) != 1 {
		return "", false
	}
	raw := (*rule.Value)[0]
	if rule.ExcludedValues != nil && slices.Contains(*rule.ExcludedValues, raw) {
		return "", false
	}
	return raw, true
}

// permitted reports whether a rule lets an organization unit choose a value: it is among the values the rule offers,
// when it offers some, and not among those it withholds.
func permitted(rule sharing.OverlayRule, raw string) bool {
	if rule.AllowedValues != nil && !slices.Contains(*rule.AllowedValues, raw) {
		return false
	}
	return rule.ExcludedValues == nil || !slices.Contains(*rule.ExcludedValues, raw)
}

// resolvedDefinition is a variable as an organization unit it was shared with sees it: its default is the value the
// rule that names it for the unit sets, or the owner's default when the rule sets none, and a select lists only the
// options the unit may choose from.
func resolvedDefinition(
	def providers.VariableDefinition, rule sharing.OverlayRule,
) (providers.VariableDefinition, error) {
	resolved := def
	if raw, ok := pinnedValue(rule); ok {
		value, err := jsonValue(def, raw)
		if err != nil {
			return providers.VariableDefinition{}, err
		}
		resolved.DefaultValue = value
	}
	if def.Type == providers.InputTypeSelect {
		options := make([]interface{}, 0, len(def.Options))
		for _, option := range def.Options {
			raw, err := core.RawFromJSONValue(def, option)
			if err != nil {
				return providers.VariableDefinition{}, err
			}
			if permitted(rule, raw) {
				options = append(options, option)
			}
		}
		resolved.Options = options
	}
	return resolved, nil
}

// variableRule reads the rule that names a variable for an organization unit. The rule is absent when no policy names
// the variable for it.
func (s *flowMgtService) variableRule(
	ctx context.Context, flowID, ouID, identifier string,
) (sharing.FieldRule, *tidcommon.ServiceError) {
	rule, svcErr := s.sharingSvc.ResolveFieldRule(ctx, FlowSharingType, flowID, ouID, sharingKey(identifier))
	if svcErr != nil {
		return sharing.FieldRule{}, translateSharingError(svcErr)
	}
	return rule, nil
}

// sharedVariables works out the variables a flow shows an organization unit it was shared with: each one a policy
// names for it, resolved for it.
func (s *flowMgtService) sharedVariables(
	ctx context.Context, flow *providers.CompleteFlowDefinition, ouID string,
) ([]providers.VariableDefinition, *tidcommon.ServiceError) {
	defs := declaredVariables(flow)
	out := make([]providers.VariableDefinition, 0, len(defs))
	for _, def := range defs {
		fieldRule, svcErr := s.variableRule(ctx, flow.ID, ouID, def.Identifier)
		if svcErr != nil {
			return nil, svcErr
		}
		if fieldRule.Source != sharing.SourcePolicy {
			continue
		}
		resolved, err := resolvedDefinition(def, fieldRule.Rule)
		if err != nil {
			s.logger.Error(ctx, "The variable of a flow is not readable", log.String(logKeyFlowID, flow.ID),
				log.String("variable", def.Identifier), log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		out = append(out, resolved)
	}
	return out, nil
}

// GetFlowOverlayRules answers what one organization unit may do with the variables of a flow: whether it owns the
// flow, which policies reached it, and the rule for each variable they name. An organization unit no policy reaches is
// refused, because the flow is not available there and so has no terms to resolve.
func (s *flowMgtService) GetFlowOverlayRules(
	ctx context.Context, flowID, ouID string,
) (sharing.ResolvedOverlay, *tidcommon.ServiceError) {
	flow, svcErr := s.requireVisibleFlow(ctx, flowID, ouID)
	if svcErr != nil {
		return sharing.ResolvedOverlay{}, svcErr
	}
	overlay, svcErr := s.sharingSvc.ResolveOverlayRules(ctx, FlowSharingType, flowID, ouID)
	if svcErr != nil {
		return sharing.ResolvedOverlay{}, translateSharingError(svcErr)
	}

	// The framework answers for the fields a type declares, and a variable is a member of one, so each variable is
	// asked for on its own.
	rules := map[string]sharing.OverlayRule{}
	for _, def := range declaredVariables(flow) {
		fieldRule, svcErr := s.variableRule(ctx, flowID, ouID, def.Identifier)
		if svcErr != nil {
			return sharing.ResolvedOverlay{}, svcErr
		}
		if fieldRule.Source == sharing.SourcePolicy {
			rules[sharingKey(def.Identifier)] = fieldRule.Rule
		}
	}
	overlay.Rules = rules
	return overlay, nil
}

// GetVariables returns the values an organization unit has set for the variables of a flow: its overrides. What a
// policy fixes for the unit is not an override, and a unit that has set nothing has none.
func (s *flowMgtService) GetVariables(
	ctx context.Context, flowID, ouID string,
) (*VariablesResponse, *tidcommon.ServiceError) {
	flow, svcErr := s.requireVisibleFlow(ctx, flowID, ouID)
	if svcErr != nil {
		return nil, svcErr
	}
	held, svcErr := s.sharingSvc.ResolveOverlayValues(ctx, FlowSharingType, flowID, ouID)
	if svcErr != nil {
		return nil, translateSharingError(svcErr)
	}

	values := map[string]interface{}{}
	for _, def := range declaredVariables(flow) {
		v, ok := held.Values[sharingKey(def.Identifier)]
		if !ok || v.Source != sharing.SourceOverlay || len(v.Value) != 1 {
			continue
		}
		value, err := jsonValue(def, v.Value[0])
		if err != nil {
			s.logger.Error(ctx, "A variable value of an organization unit is not readable",
				log.String(logKeyFlowID, flowID), log.String("variable", def.Identifier), log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		values[def.Identifier] = value
	}
	return &VariablesResponse{Values: values}, nil
}

// SetVariables sets the values an organization unit holds for the variables of a flow, replacing what it held: a
// variable left out returns to its default, or to the value a policy fixes for it. Every value is checked before
// anything is written, against the variable and the rule that names it for the unit, and for a connection against the
// connection itself.
func (s *flowMgtService) SetVariables(
	ctx context.Context, flowID, ouID string, req SetVariablesRequest,
) (*VariablesResponse, *tidcommon.ServiceError) {
	flow, svcErr := s.requireVisibleFlow(ctx, flowID, ouID)
	if svcErr != nil {
		return nil, svcErr
	}
	defs := variablesByID(declaredVariables(flow))

	raws, svcErr := s.checkValues(ctx, flowID, ouID, defs, req.Values)
	if svcErr != nil {
		return nil, svcErr
	}

	// The values are replaced together, in one transaction of the sharing store, which is where they live whichever
	// store holds the flow: a write that fails leaves the unit with what it held before.
	declared := declaredVariables(flow)
	keys := make([]string, 0, len(declared))
	values := make(map[string][]string, len(raws))
	for _, def := range declared {
		key := sharingKey(def.Identifier)
		keys = append(keys, key)
		if raw, set := raws[def.Identifier]; set {
			values[key] = []string{raw}
		}
	}
	replaceErr := s.sharingSvc.ReplaceOverlayValues(ctx, FlowSharingType, flowID, ouID, keys, values)
	if replaceErr != nil {
		if identifier, isVariable := strings.CutPrefix(replaceErr.FieldKey, sharingFieldKey+"."); isVariable {
			return nil, translateValueError(identifier, replaceErr.Err)
		}
		return nil, translateSharingError(replaceErr.Err)
	}
	return s.GetVariables(ctx, flowID, ouID)
}

// checkValues validates every value of a request, and returns the raw form of each.
func (s *flowMgtService) checkValues(
	ctx context.Context, flowID, ouID string, defs map[string]providers.VariableDefinition,
	values map[string]interface{},
) (map[string]string, *tidcommon.ServiceError) {
	raws := make(map[string]string, len(values))
	for id, value := range values {
		def, declared := defs[id]
		if !declared {
			return nil, unknownVariable(id)
		}
		fieldRule, svcErr := s.variableRule(ctx, flowID, ouID, id)
		if svcErr != nil {
			return nil, svcErr
		}
		if fieldRule.Source != sharing.SourcePolicy {
			return nil, unknownVariable(id)
		}
		if !fieldRule.Rule.Editable {
			return nil, variableNotEditable(id)
		}
		if value == nil {
			return nil, invalidValue(id, "is null, and a variable always has a value")
		}
		raw, err := core.RawFromJSONValue(def, value)
		if err == nil {
			err = core.ValidateRaw(def, raw)
		}
		if err != nil {
			return nil, invalidValue(id, err.Error())
		}
		if !permitted(fieldRule.Rule, raw) {
			return nil, invalidValue(id, "is not among the values the flow's owner allows")
		}
		if def.Type == providers.InputTypeConnectionSelect {
			reason, svcErr := s.checkConnection(ctx, def, raw)
			if svcErr != nil {
				return nil, svcErr
			}
			if reason == "" {
				reason, svcErr = s.checkConnectionVisible(ctx, raw, ouID)
				if svcErr != nil {
					return nil, svcErr
				}
			}
			if reason != "" {
				return nil, invalidValue(id, reason)
			}
		}
		raws[id] = raw
	}
	return raws, nil
}

// GetEffectiveVariableValues returns the raw value of every variable of a flow that is not at the owner's default for
// an organization unit: the one it set itself, or the one a policy fixes or starts it with. It is what a running flow
// loads once the organization unit is known. A flow the organization unit neither owns nor was shared holds none, and
// neither does an organization unit for an application it neither owns nor was shared: naming a unit on a prompt is not
// enough to run on its values.
func (s *flowMgtService) GetEffectiveVariableValues(
	ctx context.Context, flowID string, app providers.Application, ouID string,
) (map[string]string, *tidcommon.ServiceError) {
	effective := map[string]string{}
	if flowID == "" || ouID == "" {
		return effective, nil
	}
	ctx = security.WithRuntimeContext(ctx)

	if ouID != app.OUID {
		appVisible, svcErr := s.sharingSvc.IsVisible(ctx, applicationSharingType, app.ID, ouID)
		if svcErr != nil {
			return nil, translateSharingError(svcErr)
		}
		if !appVisible {
			return effective, nil
		}
	}

	// This runs on every sign in through an application of another unit, so the cheap answers come first. The
	// visibility of the flow is cached, and a unit that cannot see it holds nothing of it.
	visible, svcErr := s.sharingSvc.IsVisible(ctx, FlowSharingType, flowID, ouID)
	if svcErr != nil {
		return nil, translateSharingError(svcErr)
	}
	if !visible {
		return effective, nil
	}
	flow, svcErr := s.GetFlow(ctx, flowID)
	if svcErr != nil {
		return nil, svcErr
	}
	defs := declaredVariables(flow)
	if len(defs) == 0 || flow.OUID == ouID {
		return effective, nil
	}

	// The rules of every variable are resolved together, and the values the unit saved are read only when a policy
	// names a variable, as nothing else can have one.
	keys := make([]string, len(defs))
	for i, def := range defs {
		keys[i] = sharingKey(def.Identifier)
	}
	rules, svcErr := s.sharingSvc.ResolveFieldRules(ctx, FlowSharingType, flowID, ouID, keys)
	if svcErr != nil {
		return nil, translateSharingError(svcErr)
	}
	named := slices.ContainsFunc(keys, func(key string) bool { return rules[key].Source == sharing.SourcePolicy })
	if !named {
		return effective, nil
	}
	held, svcErr := s.sharingSvc.ResolveOverlayValues(ctx, FlowSharingType, flowID, ouID)
	if svcErr != nil {
		return nil, translateSharingError(svcErr)
	}
	for _, def := range defs {
		key := sharingKey(def.Identifier)
		fieldRule := rules[key]
		if fieldRule.Source != sharing.SourcePolicy {
			continue
		}
		if v, ok := held.Values[key]; ok && v.Source == sharing.SourceOverlay && len(v.Value) == 1 {
			effective[def.Identifier] = v.Value[0]
			continue
		}
		if raw, ok := pinnedValue(fieldRule.Rule); ok {
			effective[def.Identifier] = raw
		}
	}
	return effective, nil
}

// translateValueError maps a framework refusal of a value to the variable's own.
func translateValueError(variable string, svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	switch svcErr.Code {
	case sharing.ErrorFieldNotEditable.Code:
		return variableNotEditable(variable)
	case sharing.ErrorValueNotPermitted.Code:
		return invalidValue(variable, "is not among the values the flow's owner allows")
	case sharing.ErrorNotShared.Code:
		return &ErrorFlowNotFound
	}
	return translateSharingError(svcErr)
}

// unknownVariable is the error for a variable the flow does not declare, or that no policy names for the unit.
func unknownVariable(variable string) *tidcommon.ServiceError {
	return tidcommon.CustomServiceError(ErrorUnknownVariable, tidcommon.I18nMessage{
		Key:          ErrorUnknownVariable.ErrorDescription.Key,
		DefaultValue: ErrorUnknownVariable.ErrorDescription.DefaultValue,
		Params:       map[string]string{"variable": variable},
	})
}

// variableNotEditable is the error for a variable the organization unit may not set.
func variableNotEditable(variable string) *tidcommon.ServiceError {
	return tidcommon.CustomServiceError(ErrorVariableNotEditable, tidcommon.I18nMessage{
		Key:          ErrorVariableNotEditable.ErrorDescription.Key,
		DefaultValue: ErrorVariableNotEditable.ErrorDescription.DefaultValue,
		Params:       map[string]string{"variable": variable},
	})
}

// invalidValue is the error for a value that is not valid for its variable.
func invalidValue(variable, reason string) *tidcommon.ServiceError {
	return tidcommon.CustomServiceError(ErrorInvalidVariableValue, tidcommon.I18nMessage{
		Key:          ErrorInvalidVariableValue.ErrorDescription.Key,
		DefaultValue: ErrorInvalidVariableValue.ErrorDescription.DefaultValue,
		Params:       map[string]string{"variable": variable, "reason": reason},
	})
}

// ---------------------------------------------------------------------------
// Sharing
// ---------------------------------------------------------------------------
// requireOwnPolicy fetches a policy and refuses one that belongs to another resource, which a policy id alone
// would otherwise reach through any flow's path.
func (s *flowMgtService) requireOwnPolicy(
	ctx context.Context, flowID, policyID string,
) (sharing.Policy, *tidcommon.ServiceError) {
	if _, svcErr := s.GetFlow(ctx, flowID); svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	policy, svcErr := s.sharingSvc.GetPolicy(ctx, policyID)
	if svcErr != nil {
		return sharing.Policy{}, translateSharingError(svcErr)
	}
	if policy.ResourceType != FlowSharingType || policy.ResourceID != flowID {
		return sharing.Policy{}, &ErrorSharingPolicyNotFound
	}
	return policy, nil
}

// CreateSharingPolicy records which organization units a flow is shared with, and on what terms.
func (s *flowMgtService) CreateSharingPolicy(
	ctx context.Context, flowID string, req sharing.PolicyRequest,
) (sharing.Policy, *tidcommon.ServiceError) {
	flow, svcErr := s.GetFlow(ctx, flowID)
	if svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	if svcErr := s.validatePolicyVariables(ctx, flow, req); svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	policy, svcErr := s.sharingSvc.CreatePolicy(ctx, FlowSharingType, flowID, flow.OUID, req)
	return policy, translateSharingError(svcErr)
}

// GetSharingPolicy returns one of a flow's sharing policies.
func (s *flowMgtService) GetSharingPolicy(
	ctx context.Context, flowID, policyID string,
) (sharing.Policy, *tidcommon.ServiceError) {
	return s.requireOwnPolicy(ctx, flowID, policyID)
}

// ListSharingPolicies returns one page of a flow's sharing policies.
func (s *flowMgtService) ListSharingPolicies(
	ctx context.Context, flowID string, limit, offset int,
) (sharing.PolicyList, *tidcommon.ServiceError) {
	if _, svcErr := s.GetFlow(ctx, flowID); svcErr != nil {
		return sharing.PolicyList{}, svcErr
	}
	list, svcErr := s.sharingSvc.GetPolicyList(ctx, FlowSharingType, flowID, limit, offset)
	return list, translateSharingError(svcErr)
}

// UpdateSharingPolicy replaces a sharing policy's contents.
func (s *flowMgtService) UpdateSharingPolicy(
	ctx context.Context, flowID, policyID string, req sharing.PolicyRequest,
) (sharing.Policy, *tidcommon.ServiceError) {
	policy, svcErr := s.requireOwnPolicy(ctx, flowID, policyID)
	if svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	flow, svcErr := s.GetFlow(ctx, flowID)
	if svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	// The issuer of a policy is not part of an edit: it is the one the policy already has.
	req.InitiatingOUID = policy.InitiatingOUID
	if svcErr := s.validatePolicyVariables(ctx, flow, req); svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	updated, svcErr := s.sharingSvc.UpdatePolicy(ctx, policyID, req)
	return updated, translateSharingError(svcErr)
}

// DeleteSharingPolicy removes a sharing policy and what depended on it. Deleting one that is gone succeeds.
func (s *flowMgtService) DeleteSharingPolicy(ctx context.Context, flowID, policyID string) *tidcommon.ServiceError {
	policy, svcErr := s.requireOwnPolicy(ctx, flowID, policyID)
	if svcErr != nil {
		if svcErr.Code == ErrorSharingPolicyNotFound.Code {
			return nil
		}
		return svcErr
	}
	if _, svcErr := s.GetFlow(ctx, flowID); svcErr != nil {
		return svcErr
	}
	if policy.Declared {
		return &ErrorSharingPolicyDeclared
	}
	return translateSharingError(s.sharingSvc.DeletePolicy(ctx, policyID))
}

// ValidateSharingPolicies checks the terms policies state for the variables of a flow, without recording them. It is
// what a replay of policies from a document is checked with before it is applied.
func (s *flowMgtService) ValidateSharingPolicies(
	ctx context.Context, flowID string, policies []sharing.PolicyRequest,
) *tidcommon.ServiceError {
	flow, svcErr := s.GetFlow(ctx, flowID)
	if svcErr != nil {
		return svcErr
	}
	for _, policy := range policies {
		if svcErr := s.validatePolicyVariables(ctx, flow, policy); svcErr != nil {
			return svcErr
		}
	}
	return nil
}

// removeSharing drops every sharing policy of a flow that has just been deleted, so a flow later created with the same
// id does not come back already shared. It runs after the flow's own delete, because only that delete knows whether it
// will be refused. The two cannot share a transaction, so a failure here leaves policies behind a flow that is gone.
// The caller is told, and retrying the delete removes them (see purgeOrphanedSharing).
func (s *flowMgtService) removeSharing(ctx context.Context, flowID string) *tidcommon.ServiceError {
	return translateSharingError(s.sharingSvc.DeletePoliciesForResource(ctx, FlowSharingType, flowID))
}

// purgeOrphanedSharing finishes a delete whose policy cleanup failed. It is called when a delete finds the flow already
// gone: policies still recorded for it are removed and the delete succeeds, since the flow being gone is what the
// caller asked for.
func (s *flowMgtService) purgeOrphanedSharing(ctx context.Context, flowID string) *tidcommon.ServiceError {
	policies, svcErr := s.sharingSvc.ListPolicies(ctx, FlowSharingType, flowID)
	if svcErr != nil {
		return translateSharingError(svcErr)
	}
	if len(policies) == 0 {
		return nil
	}
	return s.removeSharing(ctx, flowID)
}

// validatePolicyVariables checks the terms a sharing policy states for the variables of a flow: each rule names a
// variable the flow declares, a variable holds one value, and every value is valid for its variable. A value naming a
// connection has to name one that exists and is of a kind the variable takes.
func (s *flowMgtService) validatePolicyVariables(
	ctx context.Context, flow *providers.CompleteFlowDefinition, req sharing.PolicyRequest,
) *tidcommon.ServiceError {
	defs := variablesByID(declaredVariables(flow))

	for _, rules := range policyRuleSets(req) {
		for key, rule := range rules {
			id, isVariable := strings.CutPrefix(key, sharingFieldKey+".")
			def, declared := defs[id]
			if !isVariable || !declared {
				return sharingPolicyError(fmt.Sprintf("the rule %q does not name a variable the flow declares", key))
			}
			if rule.Value != nil && len(*rule.Value) > 1 {
				return sharingPolicyError(fmt.Sprintf("the variable %q holds one value, and the rule gives %d",
					id, len(*rule.Value)))
			}
			for _, group := range []struct {
				values  *[]string
				connect bool
			}{{rule.Value, true}, {rule.AllowedValues, true}, {rule.ExcludedValues, false}} {
				if group.values == nil {
					continue
				}
				for _, raw := range *group.values {
					if svcErr := s.checkPolicyValue(ctx, def, raw, group.connect); svcErr != nil {
						return svcErr
					}
				}
			}
		}
	}
	return nil
}

// dropRemovedVariables removes what the sharing of a flow holds for each variable the change takes out of it: the rule
// every policy states for the variable and the value every organization unit saved for it. They name a variable that
// no longer exists, so nothing is left to keep them for, and leaving them would refuse the change.
func (s *flowMgtService) dropRemovedVariables(
	ctx context.Context, flowID string, before *providers.CompleteFlowDefinition, nodes []providers.NodeDefinition,
) *tidcommon.ServiceError {
	kept := variablesByID(declaredVariables(&providers.CompleteFlowDefinition{ID: flowID, Nodes: nodes}))
	for _, def := range declaredVariables(before) {
		if _, stays := kept[def.Identifier]; stays {
			continue
		}
		if svcErr := s.sharingSvc.RemoveField(ctx, FlowSharingType, flowID, sharingKey(def.Identifier)); svcErr != nil {
			return translateSharingError(svcErr)
		}
	}
	return nil
}

// checkHeldSharing checks a new definition of a flow against the sharing policies recorded for it and the values
// organization units saved for its variables.
func (s *flowMgtService) checkHeldSharing(
	ctx context.Context, flowID string, nodes []providers.NodeDefinition,
) *tidcommon.ServiceError {
	if svcErr := s.checkSharingPolicies(ctx, flowID, nodes); svcErr != nil {
		return svcErr
	}
	return s.checkSavedValues(ctx, flowID, nodes)
}

// replaceSharing replaces the sharing policies of a flow with the ones a document carries, for a new definition of the
// flow. The policies are checked against that definition, applied, and then what the flow holds is checked against it
// again, as policies the document does not name are kept and may still conflict. It runs in the transaction of the
// update, so a refusal undoes the update too.
func (s *flowMgtService) replaceSharing(
	ctx context.Context, flowID, owningOUID string, nodes []providers.NodeDefinition,
	policies []sharing.PolicyRequest,
) *tidcommon.ServiceError {
	changed := &providers.CompleteFlowDefinition{ID: flowID, Nodes: nodes}
	for _, policy := range policies {
		if svcErr := s.validatePolicyVariables(ctx, changed, policy); svcErr != nil {
			return svcErr
		}
	}

	replay, replayErr := s.sharingSvc.ReplacePoliciesForResource(ctx, FlowSharingType, flowID, owningOUID, policies)
	for _, skipped := range replay.Skipped {
		s.logger.Warn(ctx, "Sharing policy in the document was skipped; a declared policy governs this unit",
			log.String(logKeyFlowID, flowID), log.Int("policyIndex", skipped.Index+1),
			log.String("initiatingOUID", skipped.InitiatingOUID),
			log.String("declaredPolicyID", skipped.DeclaredPolicyID))
	}
	if replayErr != nil {
		// A document names its policies only by position, so the refusal says which one.
		return tidcommon.CustomServiceError(*replayErr.Err, tidcommon.I18nMessage{
			Key: replayErr.Err.ErrorDescription.Key,
			DefaultValue: fmt.Sprintf("sharing policy %d (organization unit %s): %s",
				replayErr.Index+1, replayErr.InitiatingOUID, replayErr.Err.ErrorDescription.DefaultValue),
		})
	}
	return s.checkHeldSharing(ctx, flowID, nodes)
}

// checkSharingPolicies checks the variables a changed flow declares against the sharing policies recorded for it. Each
// rule has to still name a variable the flow declares, and every value it states has to still be valid for that
// variable. A change that breaks one is refused, naming the policy, so its owner updates or deletes the policy first.
func (s *flowMgtService) checkSharingPolicies(
	ctx context.Context, flowID string, nodes []providers.NodeDefinition,
) *tidcommon.ServiceError {
	policies, svcErr := s.sharingSvc.ListPolicies(ctx, FlowSharingType, flowID)
	if svcErr != nil {
		return translateSharingError(svcErr)
	}
	changed := &providers.CompleteFlowDefinition{ID: flowID, Nodes: nodes}
	for _, policy := range policies {
		sections := sharing.ToPolicyResponse(policy)
		svcErr := s.validatePolicyVariables(ctx, changed, sharing.PolicyRequest{
			AllUnits: sections.AllUnits, ChildUnits: sections.ChildUnits, OtherTreeRoots: sections.OtherTreeRoots,
		})
		if svcErr == nil {
			continue
		}
		if svcErr.Type != tidcommon.ClientErrorType {
			return svcErr
		}
		return tidcommon.CustomServiceError(ErrorFlowConflictsWithSharingPolicy, tidcommon.I18nMessage{
			Key: "error.flowmgtservice.flow_conflicts_with_sharing_policy_terms_description",
			DefaultValue: "The change breaks the sharing policy '{{param(policy)}}': {{param(reason)}}. " +
				"Update or delete the policy first",
			Params: map[string]string{"policy": policy.ID, "reason": svcErr.ErrorDescription.Params["reason"]},
		})
	}
	return nil
}

// maxReportedValueConflicts bounds how many saved values a refusal names, so a flow shared widely answers with a
// message of a readable size.
const maxReportedValueConflicts = 5

// checkSavedValues checks the values organization units have saved for the variables of a flow against the variables
// the changed flow declares: each has to still name a variable the flow declares, and still be valid for it. A change
// that invalidates one is refused, naming the organization unit and the variable, so the value is changed, or the
// policy that reaches the unit updated or deleted, first.
func (s *flowMgtService) checkSavedValues(
	ctx context.Context, flowID string, nodes []providers.NodeDefinition,
) *tidcommon.ServiceError {
	held, svcErr := s.sharingSvc.ListOverlayValues(ctx, FlowSharingType, flowID)
	if svcErr != nil {
		return translateSharingError(svcErr)
	}
	defs := variablesByID(declaredVariables(&providers.CompleteFlowDefinition{ID: flowID, Nodes: nodes}))

	var conflicts []string
	for _, ouID := range slices.Sorted(maps.Keys(held)) {
		for _, key := range slices.Sorted(maps.Keys(held[ouID])) {
			values := held[ouID][key]
			// A field the unit cleared holds no value to check.
			if len(values) != 1 {
				continue
			}
			id, isVariable := strings.CutPrefix(key, sharingFieldKey+".")
			def, declared := defs[id]
			reason := ""
			switch {
			case !isVariable || !declared:
				reason = "the variable is no longer declared"
			default:
				if err := core.ValidateRaw(def, values[0]); err != nil {
					reason = fmt.Sprintf("the value %q %s", values[0], err.Error())
				} else if def.Type == providers.InputTypeConnectionSelect {
					var connErr *tidcommon.ServiceError
					if reason, connErr = s.checkConnection(ctx, def, values[0]); connErr != nil {
						return connErr
					}
					if reason != "" {
						reason = fmt.Sprintf("the value %q %s", values[0], reason)
					}
				}
			}
			if reason != "" {
				conflicts = append(conflicts, fmt.Sprintf("%s of %s (%s)", id, ouID, reason))
			}
		}
	}
	if len(conflicts) == 0 {
		return nil
	}
	details := strings.Join(conflicts[:min(len(conflicts), maxReportedValueConflicts)], "; ")
	if extra := len(conflicts) - maxReportedValueConflicts; extra > 0 {
		details += fmt.Sprintf("; and %d more", extra)
	}
	return tidcommon.CustomServiceError(ErrorFlowConflictsWithSavedValues, tidcommon.I18nMessage{
		Key: "error.flowmgtservice.flow_conflicts_with_saved_values_terms_description",
		DefaultValue: "The change invalidates values that organization units saved: {{param(details)}}. Ask them to " +
			"change those values, or update or delete the sharing policy that reaches them, first",
		Params: map[string]string{"details": details},
	})
}

// checkPolicyValue checks one value a policy names for a variable. A value the policy withholds is checked for its
// form only, as nothing is stored for it.
func (s *flowMgtService) checkPolicyValue(
	ctx context.Context, def providers.VariableDefinition, raw string, usable bool,
) *tidcommon.ServiceError {
	if err := core.ValidateRaw(def, raw); err != nil {
		return sharingPolicyError(fmt.Sprintf("the value %q of the variable %q %s", raw, def.Identifier, err.Error()))
	}
	if !usable || def.Type != providers.InputTypeConnectionSelect {
		return nil
	}
	reason, svcErr := s.checkConnection(ctx, def, raw)
	if svcErr != nil {
		return svcErr
	}
	if reason != "" {
		return sharingPolicyError(fmt.Sprintf("the value %q of the variable %q %s", raw, def.Identifier, reason))
	}
	return nil
}

// checkConnection checks that a connection a variable is given exists and is of a kind the variable takes. Without the
// connections to check against it is refused, as there is nothing to vouch for the value.
func (s *flowMgtService) checkConnection(
	ctx context.Context, def providers.VariableDefinition, connectionID string,
) (string, *tidcommon.ServiceError) {
	idp, svcErr := s.idps.GetIdentityProvider(security.WithRuntimeContext(ctx), connectionID)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			return "is not an existing connection", nil
		}
		return "", &tidcommon.InternalServerError
	}
	vendor := strings.ToLower(string(idp.Type))
	if !utils.ContainsFold(identityProviderVendors, vendor) {
		return "is not an identity provider connection", nil
	}
	if len(def.ConnectionTypes) > 0 && !utils.ContainsFold(def.ConnectionTypes, vendor) {
		return fmt.Sprintf("is a %s connection, which the variable does not take", vendor), nil
	}
	return "", nil
}

// flowVariablesUseConnection reports whether a connection variable of a flow names a connection: as its default, in a
// sharing policy of the flow that gives it as a value or one of the values to choose from, or in a value an
// organization unit saved for it. A value a policy withholds is not a use.
func (s *flowMgtService) flowVariablesUseConnection(
	ctx context.Context, flow *providers.CompleteFlowDefinition, connectionID string,
) (bool, error) {
	keys := map[string]struct{}{}
	for _, def := range declaredVariables(flow) {
		if def.Type != providers.InputTypeConnectionSelect {
			continue
		}
		if raw, err := core.DefaultRaw(def); err == nil && raw == connectionID {
			return true, nil
		}
		keys[sharingKey(def.Identifier)] = struct{}{}
	}
	if len(keys) == 0 {
		return false, nil
	}

	// What a policy or a unit holds is read for every flow, whoever asks, as the connection is used all the same.
	ctx = security.WithRuntimeContext(ctx)
	policies, svcErr := s.sharingSvc.ListPolicies(ctx, FlowSharingType, flow.ID)
	if svcErr != nil {
		return false, fmt.Errorf("failed to list the sharing policies of flow %s: code=%s", flow.ID, svcErr.Code)
	}
	for _, policy := range policies {
		for _, rule := range policy.Rules {
			if _, isConnection := keys[rule.FieldKey]; !isConnection {
				continue
			}
			for _, terms := range []sharing.OverlayRule{rule.Requested, rule.Resolved} {
				if namesValue(terms.Value, connectionID) || namesValue(terms.AllowedValues, connectionID) {
					return true, nil
				}
			}
		}
	}

	held, svcErr := s.sharingSvc.ListOverlayValues(ctx, FlowSharingType, flow.ID)
	if svcErr != nil {
		return false, fmt.Errorf("failed to list the values saved for flow %s: code=%s", flow.ID, svcErr.Code)
	}
	for _, byField := range held {
		for key, values := range byField {
			if _, isConnection := keys[key]; isConnection && slices.Contains(values, connectionID) {
				return true, nil
			}
		}
	}
	return false, nil
}

// namesValue reports whether a list of values a rule states holds a value.
func namesValue(values *[]string, value string) bool {
	return values != nil && slices.Contains(*values, value)
}

// checkConnectionVisible checks that an organization unit can use a connection it sets a variable to: it owns the
// connection or the connection was shared with it. Only what a unit sets itself is checked here. The values a policy
// states are checked by the flow's sharing declaration.
func (s *flowMgtService) checkConnectionVisible(
	ctx context.Context, connectionID, ouID string,
) (string, *tidcommon.ServiceError) {
	visible, svcErr := s.sharingSvc.IsVisible(security.WithRuntimeContext(ctx),
		connection.ConnectionSharingType, connectionID, ouID)
	if svcErr != nil {
		return "", translateSharingError(svcErr)
	}
	if !visible {
		return "is a connection the organization unit neither owns nor was shared", nil
	}
	return "", nil
}

// policyRuleSets lists the per-field terms a policy request states, section by section and unit by unit.
func policyRuleSets(req sharing.PolicyRequest) []map[string]sharing.OverlayRule {
	var sets []map[string]sharing.OverlayRule
	if req.AllUnits != nil {
		sets = append(sets, req.AllUnits.OverlayRules)
	}
	if c := req.ChildUnits; c != nil {
		sets = append(sets, c.OverlayRules)
		for _, unit := range c.Units {
			sets = append(sets, unit.OverlayRules)
		}
	}
	if t := req.OtherTreeRoots; t != nil {
		sets = append(sets, t.OverlayRules)
		for _, root := range t.Units {
			sets = append(sets, root.OverlayRules)
		}
	}
	return sets
}

// variablesByID indexes variable declarations by identifier.
func variablesByID(defs []providers.VariableDefinition) map[string]providers.VariableDefinition {
	byID := make(map[string]providers.VariableDefinition, len(defs))
	for _, def := range defs {
		byID[def.Identifier] = def
	}
	return byID
}

// sharingPolicyError is the error for a sharing policy whose terms are not valid for the flow.
func sharingPolicyError(reason string) *tidcommon.ServiceError {
	return tidcommon.CustomServiceError(ErrorInvalidSharingPolicy, tidcommon.I18nMessage{
		Key:          "error.flowmgtservice.invalid_sharing_policy_terms_description",
		DefaultValue: "{{param(reason)}}",
		Params:       map[string]string{"reason": reason},
	})
}

// sharingErrors answers the framework's client errors in the flow's own codes.
var sharingErrors = sharing.ErrorTranslation{
	PolicyNotFound:  ErrorSharingPolicyNotFound,
	PolicyExists:    ErrorSharingPolicyExists,
	VersionMismatch: ErrorSharingPolicyVersionMismatch,
	PolicyDeclared:  ErrorSharingPolicyDeclared,
	InvalidLimit:    ErrorInvalidLimit,
	InvalidOffset:   ErrorInvalidOffset,
	InvalidPolicy:   ErrorInvalidSharingPolicy,
}

// translateSharingError maps a framework error to the flow's own. Anything that is not a refusal of the request is
// reported as an internal failure.
func translateSharingError(svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	return sharingErrors.Translate(svcErr)
}
