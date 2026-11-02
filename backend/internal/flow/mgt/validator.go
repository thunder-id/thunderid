// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executormeta"
	"github.com/thunder-id/thunderid/internal/flow/graphbuilder"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// ouResolveFromPrompt is the resolveFrom strategy that asks the user to select an OU.
const ouResolveFromPrompt = "prompt"

// Input identifiers the OU resolver reads in direct children mode.
const (
	ouIDInputIdentifier     = "ouId"
	ouHandleInputIdentifier = "ouHandle"
)

// FlowValidatorInterface defines the contract for validating flow definitions.
type FlowValidatorInterface interface {
	// ValidateFlowDefinition validates the provided flow definition and returns a ServiceError if validation fails.
	ValidateFlowDefinition(ctx context.Context, flowDef *FlowDefinition) *tidcommon.ServiceError
	// ValidateSystemFlowDefinition validates a system flow definition, which must be of the SYSTEM type
	// and must not wait for input.
	ValidateSystemFlowDefinition(ctx context.Context, flowDef *FlowDefinition) *tidcommon.ServiceError
}

// httpConnectionProvider checks that an extension step's HTTP connection exists.
type httpConnectionProvider interface {
	GetHTTPConnection(ctx context.Context, id string) (*httpconnection.HTTPConnection, *tidcommon.ServiceError)
}

// idpProvider reads the identity provider connections a connection variable names.
type idpProvider interface {
	GetIdentityProvider(ctx context.Context, idpID string) (*providers.IDPDTO, *tidcommon.ServiceError)
}

// flowValidator is responsible for validating flow definitions,
// including metadata, structure, node configurations, and registry-dependent checks.
type flowValidator struct {
	executorRegistry       core.ExecutorMetadataProvider
	interceptorRegistry    interceptor.InterceptorRegistryInterface
	graphBuilder           graphbuilder.GraphBuilderInterface
	httpConnectionProvider httpConnectionProvider
	idpProvider            idpProvider
	logger                 *log.Logger
}

// newFlowValidator creates a new instance of flowValidator with the provided dependencies. The IdP provider is
// optional: without one the default of a connection variable is not checked against a connection.
func newFlowValidator(
	executorRegistry core.ExecutorMetadataProvider,
	interceptorRegistry interceptor.InterceptorRegistryInterface,
	graphBuilder graphbuilder.GraphBuilderInterface,
	connectionProvider httpConnectionProvider,
	idpProvider idpProvider,
) *flowValidator {
	return &flowValidator{
		executorRegistry:       executorRegistry,
		interceptorRegistry:    interceptorRegistry,
		graphBuilder:           graphBuilder,
		httpConnectionProvider: connectionProvider,
		idpProvider:            idpProvider,
		logger:                 log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowValidator")),
	}
}

// ValidateFlowDefinition runs all validation scopes on the flow definition.
func (v *flowValidator) ValidateFlowDefinition(
	ctx context.Context, flowDef *FlowDefinition,
) *tidcommon.ServiceError {
	if err := v.validateMetadata(flowDef, isValidFlowType); err != nil {
		return err
	}
	return v.validateFlow(ctx, flowDef)
}

// ValidateSystemFlowDefinition runs all validation scopes on a system flow definition.
func (v *flowValidator) ValidateSystemFlowDefinition(
	ctx context.Context, flowDef *FlowDefinition,
) *tidcommon.ServiceError {
	isSystem := func(flowType providers.FlowType) bool { return flowType == providers.FlowTypeSystem }
	if err := v.validateMetadata(flowDef, isSystem); err != nil {
		return err
	}
	if err := v.validateSystemNodes(flowDef.Nodes); err != nil {
		return err
	}
	return v.validateFlow(ctx, flowDef)
}

// validateSystemNodes refuses a node that waits for input, which no one could give a system flow.
func (v *flowValidator) validateSystemNodes(nodes []providers.NodeDefinition) *tidcommon.ServiceError {
	for _, node := range nodes {
		if node.Type == string(common.NodeTypePrompt) {
			return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
				Key:          "error.flowmgtservice.system_flow_prompt_node_description",
				DefaultValue: "System flow node '{{param(nodeID)}}' waits for input, which a system flow cannot",
				Params:       map[string]string{"nodeID": node.ID},
			})
		}
	}
	return nil
}

// validateFlow runs the validation scopes after the metadata.
func (v *flowValidator) validateFlow(ctx context.Context, flowDef *FlowDefinition) *tidcommon.ServiceError {
	nodeIndex, err := v.validateStructure(flowDef.Nodes)
	if err != nil {
		return err
	}

	if err := v.validateFlowTypeBasedConstraints(flowDef.FlowType, flowDef.Nodes); err != nil {
		return err
	}

	if err := v.validateNodes(flowDef.Nodes, nodeIndex, flowDef.FlowType); err != nil {
		return err
	}

	if err := v.validateVariables(ctx, flowDef); err != nil {
		return err
	}

	if err := v.validateInterceptors(flowDef.Interceptors, nodeIndex); err != nil {
		return err
	}

	if err := v.validateExtensionSteps(ctx, flowDef.Nodes); err != nil {
		return err
	}

	if err := v.validateGraph(ctx, flowDef); err != nil {
		return err
	}

	return nil
}

// validateGraph builds the flow graph as a defense-in-depth validation step.
func (v *flowValidator) validateGraph(
	ctx context.Context, flowDef *FlowDefinition,
) *tidcommon.ServiceError {
	tempFlow := &providers.CompleteFlowDefinition{
		Handle:       flowDef.Handle,
		Name:         flowDef.Name,
		FlowType:     flowDef.FlowType,
		Interceptors: flowDef.Interceptors,
		Nodes:        flowDef.Nodes,
	}
	return v.graphBuilder.ValidateGraph(ctx, tempFlow)
}

// ---------------------------------------------------------------------------
// Scope: Metadata validation
// ---------------------------------------------------------------------------

// validateMetadata validates flow-level fields: handle, name, type, ID format, and minimum node count.
// validType says which flow types the definition may take.
func (v *flowValidator) validateMetadata(
	flowDef *FlowDefinition, validType func(providers.FlowType) bool,
) *tidcommon.ServiceError {
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
	if !validType(flowDef.FlowType) {
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

	return nil
}

// ---------------------------------------------------------------------------
// Scope: Flow type-based constraint validation (static maps, no registry)
// ---------------------------------------------------------------------------

// requiredExecutorsByFlowType maps flow type → executor names that must appear at least once.
var requiredExecutorsByFlowType = map[providers.FlowType][]string{
	providers.FlowTypeAuthentication: {executormeta.ExecutorNameAuthAssert},
	providers.FlowTypeRegistration: {
		executormeta.ExecutorNameProvisioning, executormeta.ExecutorNameUserTypeResolver,
	},
	providers.FlowTypeUserOnboarding: {
		executormeta.ExecutorNameProvisioning, executormeta.ExecutorNameUserTypeResolver,
	},
}

// companionExecutor is an executor that must appear alongside another, running in the given mode.
type companionExecutor struct {
	name string
	mode string
}

// companionExecutors maps an executor to executors that must appear alongside it in the same flow,
// independent of flow type.
//
// SessionRevocationExecutor terminates a subject's SSO sessions but records no revocation itself. The
// tokens those sessions hold are covered by the subject criterion that CriteriaRevocationExecutor
// persists, which is why session termination does not enumerate token families per application. A
// flow that terminates without it would therefore delete sessions while their tokens stay live, so
// the pairing is required rather than conventional. Only the revoke mode counts: the advance_cutoff
// mode writes nothing for a revoke-all plan, so it alone would leave those tokens live.
var companionExecutors = map[string][]companionExecutor{
	executormeta.ExecutorNameSessionRevocation: {
		{name: executormeta.ExecutorNameCriteriaRevocation, mode: executormeta.ExecutorModeRevoke},
	},
}

// modeFlowTypes limits an executor mode to fewer flow types than the executor supports. Identity
// governance suspends and unsuspends only in an administration flow; a recovery flow runs only unlock.
var modeFlowTypes = map[string]map[string][]providers.FlowType{
	executormeta.ExecutorNameIdentityGovernance: {
		executormeta.ExecutorModeSuspend:   {providers.FlowTypeAdministration},
		executormeta.ExecutorModeUnsuspend: {providers.FlowTypeAdministration},
	},
}

// validateFlowTypeBasedConstraints checks forbidden and required executor rules for the flow type.
// Uses static maps only — no registry access required.
func (v *flowValidator) validateFlowTypeBasedConstraints(
	flowType providers.FlowType, nodes []providers.NodeDefinition,
) *tidcommon.ServiceError {
	if svcErr := v.validateRequiredExecutors(flowType, nodes); svcErr != nil {
		return svcErr
	}
	if svcErr := v.validateModeFlowTypes(flowType, nodes); svcErr != nil {
		return svcErr
	}
	return v.validateCompanionExecutors(nodes)
}

// validateModeFlowTypes returns an error when a node runs an executor in a mode the flow type cannot
// run.
func (v *flowValidator) validateModeFlowTypes(
	flowType providers.FlowType, nodes []providers.NodeDefinition,
) *tidcommon.ServiceError {
	for _, node := range nodes {
		if node.Type != string(common.NodeTypeTaskExecution) || node.Executor == nil {
			continue
		}
		flowTypes, ok := modeFlowTypes[node.Executor.Name][node.Executor.Mode]
		if !ok || slices.Contains(flowTypes, flowType) {
			continue
		}
		return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
			Key: "error.flowmgtservice.unsupported_executor_mode_flow_type_description",
			DefaultValue: "Node '{{param(nodeID)}}': executor '{{param(executorName)}}' mode " +
				"'{{param(mode)}}' is not compatible with flow type '{{param(flowType)}}'",
			Params: map[string]string{
				"nodeID":       node.ID,
				"executorName": node.Executor.Name,
				"mode":         node.Executor.Mode,
				"flowType":     string(flowType),
			},
		})
	}
	return nil
}

// validateCompanionExecutors returns an error when an executor is present without an executor it
// depends on.
func (v *flowValidator) validateCompanionExecutors(
	nodes []providers.NodeDefinition,
) *tidcommon.ServiceError {
	presentExecutors := collectPresentExecutors(nodes)
	for name, companions := range companionExecutors {
		if !presentExecutors[name] {
			continue
		}
		for _, companion := range companions {
			if hasExecutorInMode(nodes, companion.name, companion.mode) {
				continue
			}
			return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
				Key: "error.flowmgtservice.companion_executor_missing_description",
				DefaultValue: "Executor '{{param(executorName)}}' requires executor " +
					"'{{param(companionExecutorName)}}' in the same flow",
				Params: map[string]string{
					"executorName":          name,
					"companionExecutorName": companion.name,
				},
			})
		}
	}
	return nil
}

// validateRequiredExecutors returns an error if a TASK_EXECUTION node with a required executor
// is absent from the flow for the given flow type.
func (v *flowValidator) validateRequiredExecutors(
	flowType providers.FlowType, nodes []providers.NodeDefinition,
) *tidcommon.ServiceError {
	required, ok := requiredExecutorsByFlowType[flowType]
	if !ok {
		return nil
	}
	presentExecutors := collectPresentExecutors(nodes)
	for _, name := range required {
		if !presentExecutors[name] {
			return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
				Key:          "error.flowmgtservice.required_executor_missing_description",
				DefaultValue: "Flow type {{param(flowType)}} requires executor '{{param(executorName)}}'",
				Params:       map[string]string{"flowType": string(flowType), "executorName": name},
			})
		}
	}
	return nil
}

// collectPresentExecutors returns a set of executor names present in TASK_EXECUTION nodes.
func collectPresentExecutors(nodes []providers.NodeDefinition) map[string]bool {
	present := make(map[string]bool)
	for _, node := range nodes {
		if node.Type == string(common.NodeTypeTaskExecution) && node.Executor != nil {
			present[node.Executor.Name] = true
		}
	}
	return present
}

// hasExecutorInMode reports whether a TASK_EXECUTION node runs the named executor in the given mode,
// taking the executor's default mode for a node that omits one.
func hasExecutorInMode(nodes []providers.NodeDefinition, name, mode string) bool {
	meta, _ := executormeta.MetaFor(name)
	for _, node := range nodes {
		if node.Type != string(common.NodeTypeTaskExecution) || node.Executor == nil ||
			node.Executor.Name != name {
			continue
		}
		effectiveMode := node.Executor.Mode
		if effectiveMode == "" {
			effectiveMode = meta.DefaultMode
		}
		if effectiveMode == mode {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Scope: Structural validation (graph connectivity, reachability, termination)
// ---------------------------------------------------------------------------

// nodeReference captures a reference from one node to another, including which field holds the reference.
type nodeReference struct {
	sourceNodeID string
	targetNodeID string
	fieldName    string
}

// validateStructure validates the structural integrity of the flow graph.
// Returns a node index (nodeID -> *providers.NodeDefinition) for use by downstream scopes.
func (v *flowValidator) validateStructure(nodes []providers.NodeDefinition) (
	map[string]*providers.NodeDefinition, *tidcommon.ServiceError,
) {
	nodeIndex, err := buildNodeIndex(nodes)
	if err != nil {
		return nil, err
	}

	if err := v.validateNodeTypesAndCardinality(nodes); err != nil {
		return nil, err
	}

	refs := collectAllNodeReferences(nodes)
	if err := v.validateNodeReferences(refs, nodeIndex); err != nil {
		return nil, err
	}

	if err := v.validateReachability(nodes); err != nil {
		return nil, err
	}

	if err := v.validateTermination(nodes); err != nil {
		return nil, err
	}

	return nodeIndex, nil
}

// buildNodeIndex builds a map of nodeID -> *providers.NodeDefinition and checks for duplicate IDs.
func buildNodeIndex(nodes []providers.NodeDefinition) (map[string]*providers.NodeDefinition, *tidcommon.ServiceError) {
	index := make(map[string]*providers.NodeDefinition, len(nodes))
	for i := range nodes {
		node := &nodes[i]
		if _, exists := index[node.ID]; exists {
			return nil, tidcommon.CustomServiceError(ErrorInvalidFlowStructure, tidcommon.I18nMessage{
				Key:          "error.flowmgtservice.duplicate_node_id_description",
				DefaultValue: "Duplicate node ID: '{{param(nodeID)}}'",
				Params:       map[string]string{"nodeID": node.ID},
			})
		}
		index[node.ID] = node
	}
	return index, nil
}

// validateNodeTypesAndCardinality checks that every node has a valid type
// and that exactly one START and one END node exist.
func (v *flowValidator) validateNodeTypesAndCardinality(nodes []providers.NodeDefinition) *tidcommon.ServiceError {
	startCount := 0
	endCount := 0
	for _, node := range nodes {
		if !common.ValidNodeTypes[node.Type] {
			return tidcommon.CustomServiceError(ErrorInvalidNodeConfig, tidcommon.I18nMessage{
				Key:          "error.flowmgtservice.invalid_node_type_description",
				DefaultValue: "Node '{{param(nodeID)}}' has invalid type '{{param(type)}}'",
				Params:       map[string]string{"nodeID": node.ID, "type": node.Type},
			})
		}
		switch node.Type {
		case string(common.NodeTypeStart):
			startCount++
		case string(common.NodeTypeEnd):
			endCount++
		}
	}

	if startCount == 0 {
		return tidcommon.CustomServiceError(ErrorInvalidFlowStructure, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.missing_start_node_description",
			DefaultValue: "Flow definition must have exactly one START node",
		})
	}
	if startCount > 1 {
		return tidcommon.CustomServiceError(ErrorInvalidFlowStructure, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.duplicate_start_node_description",
			DefaultValue: "Flow definition must have exactly one START node, found multiple",
		})
	}
	if endCount == 0 {
		return tidcommon.CustomServiceError(ErrorInvalidFlowStructure, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.missing_end_node_description",
			DefaultValue: "Flow definition must have exactly one END node",
		})
	}
	if endCount > 1 {
		return tidcommon.CustomServiceError(ErrorInvalidFlowStructure, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.duplicate_end_node_description",
			DefaultValue: "Flow definition must have exactly one END node, found multiple",
		})
	}
	return nil
}

// collectAllNodeReferences collects all outgoing node references from all nodes.
func collectAllNodeReferences(nodes []providers.NodeDefinition) []nodeReference {
	var refs []nodeReference
	for _, node := range nodes {
		if node.OnSuccess != "" {
			refs = append(refs, nodeReference{
				sourceNodeID: node.ID, targetNodeID: node.OnSuccess, fieldName: "onSuccess",
			})
		}
		if node.OnFailure != "" {
			refs = append(refs, nodeReference{
				sourceNodeID: node.ID, targetNodeID: node.OnFailure, fieldName: "onFailure",
			})
		}
		if node.OnIncomplete != "" {
			refs = append(refs, nodeReference{
				sourceNodeID: node.ID, targetNodeID: node.OnIncomplete, fieldName: "onIncomplete",
			})
		}
		if node.Next != "" {
			refs = append(refs, nodeReference{
				sourceNodeID: node.ID, targetNodeID: node.Next, fieldName: "next",
			})
		}
		if node.Condition != nil && node.Condition.OnSkip != "" {
			refs = append(refs, nodeReference{
				sourceNodeID: node.ID, targetNodeID: node.Condition.OnSkip, fieldName: "condition.onSkip",
			})
		}
		for _, prompt := range node.Prompts {
			if prompt.Action != nil && prompt.Action.NextNode != "" {
				refs = append(refs, nodeReference{
					sourceNodeID: node.ID, targetNodeID: prompt.Action.NextNode, fieldName: "action.nextNode",
				})
			}
		}
	}
	return refs
}

// validateNodeReferences checks that all node references point to existing nodes.
func (v *flowValidator) validateNodeReferences(
	refs []nodeReference, nodeIndex map[string]*providers.NodeDefinition,
) *tidcommon.ServiceError {
	for _, ref := range refs {
		if _, exists := nodeIndex[ref.targetNodeID]; !exists {
			return tidcommon.CustomServiceError(ErrorInvalidNodeReference, tidcommon.I18nMessage{
				Key: "error.flowmgtservice.node_references_nonexistent_description",
				DefaultValue: "Node '{{param(sourceNodeID)}}' references non-existent node " +
					"'{{param(targetNodeID)}}' in '{{param(fieldName)}}'",
				Params: map[string]string{
					"sourceNodeID": ref.sourceNodeID,
					"targetNodeID": ref.targetNodeID,
					"fieldName":    ref.fieldName,
				},
			})
		}
	}
	return nil
}

// validateReachability checks that all nodes are reachable from the START node via BFS.
func (v *flowValidator) validateReachability(nodes []providers.NodeDefinition) *tidcommon.ServiceError {
	var startNodeID string
	for _, node := range nodes {
		if node.Type == string(common.NodeTypeStart) {
			startNodeID = node.ID
			break
		}
	}

	adjacency := buildAdjacencyList(nodes)

	visited := make(map[string]bool)
	queue := []string{startNodeID}
	visited[startNodeID] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, neighbor := range adjacency[current] {
			if !visited[neighbor] {
				visited[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}

	for _, node := range nodes {
		if !visited[node.ID] {
			return tidcommon.CustomServiceError(ErrorInvalidFlowStructure, tidcommon.I18nMessage{
				Key:          "error.flowmgtservice.orphaned_node_description",
				DefaultValue: "Node '{{param(nodeID)}}' is not reachable from the START node",
				Params:       map[string]string{"nodeID": node.ID},
			})
		}
	}
	return nil
}

// validateTermination checks that all reachable nodes can reach the END node via reverse BFS.
func (v *flowValidator) validateTermination(nodes []providers.NodeDefinition) *tidcommon.ServiceError {
	var endNodeID string
	for _, node := range nodes {
		if node.Type == string(common.NodeTypeEnd) {
			endNodeID = node.ID
			break
		}
	}

	reverseAdj := make(map[string][]string)
	adjacency := buildAdjacencyList(nodes)
	for source, targets := range adjacency {
		for _, target := range targets {
			reverseAdj[target] = append(reverseAdj[target], source)
		}
	}

	canReachEnd := make(map[string]bool)
	queue := []string{endNodeID}
	canReachEnd[endNodeID] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, predecessor := range reverseAdj[current] {
			if !canReachEnd[predecessor] {
				canReachEnd[predecessor] = true
				queue = append(queue, predecessor)
			}
		}
	}

	for _, node := range nodes {
		if !canReachEnd[node.ID] {
			return tidcommon.CustomServiceError(ErrorInvalidFlowStructure, tidcommon.I18nMessage{
				Key:          "error.flowmgtservice.no_termination_description",
				DefaultValue: "Node '{{param(nodeID)}}' has no path to the END node",
				Params:       map[string]string{"nodeID": node.ID},
			})
		}
	}
	return nil
}

// buildAdjacencyList builds a forward adjacency list from node definitions.
func buildAdjacencyList(nodes []providers.NodeDefinition) map[string][]string {
	adj := make(map[string][]string)
	for _, node := range nodes {
		if node.OnSuccess != "" {
			adj[node.ID] = append(adj[node.ID], node.OnSuccess)
		}
		if node.OnFailure != "" {
			adj[node.ID] = append(adj[node.ID], node.OnFailure)
		}
		if node.OnIncomplete != "" {
			adj[node.ID] = append(adj[node.ID], node.OnIncomplete)
		}
		if node.Next != "" {
			adj[node.ID] = append(adj[node.ID], node.Next)
		}
		if node.Condition != nil && node.Condition.OnSkip != "" {
			adj[node.ID] = append(adj[node.ID], node.Condition.OnSkip)
		}
		for _, prompt := range node.Prompts {
			if prompt.Action != nil && prompt.Action.NextNode != "" {
				adj[node.ID] = append(adj[node.ID], prompt.Action.NextNode)
			}
		}
	}
	return adj
}

// ---------------------------------------------------------------------------
// Scope: Node-type-specific validation
// ---------------------------------------------------------------------------

// validateNodes validates each node's configuration and registry-dependent checks.
func (v *flowValidator) validateNodes(
	nodes []providers.NodeDefinition, nodeIndex map[string]*providers.NodeDefinition,
	flowType providers.FlowType,
) *tidcommon.ServiceError {
	for i := range nodes {
		node := &nodes[i]
		if err := v.validateNodeFormat(node, nodeIndex); err != nil {
			return err
		}
		if node.Type == string(common.NodeTypeTaskExecution) {
			if err := v.validateExecutorGenericConstraints(node, flowType); err != nil {
				return err
			}
			if err := v.validateExecutorSpecificConstraints(node, nodeIndex, nodes, flowType); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateNodeFormat validates a single node's format based on its type.
// Does not require registry access.
func (v *flowValidator) validateNodeFormat(
	node *providers.NodeDefinition, nodeIndex map[string]*providers.NodeDefinition,
) *tidcommon.ServiceError {
	switch node.Type {
	case string(common.NodeTypeStart):
		return v.validateStartNode(node)
	case string(common.NodeTypeEnd):
		return v.validateEndNode(node)
	case string(common.NodeTypeTaskExecution):
		return v.validateTaskExecutionNode(node, nodeIndex)
	case string(common.NodeTypePrompt):
		return v.validatePromptNode(node)
	case string(common.NodeTypeCall):
		return v.validateCallNode(node)
	}
	return nil
}

// validateStartNode validates that a START node has onSuccess and no inapplicable properties.
func (v *flowValidator) validateStartNode(node *providers.NodeDefinition) *tidcommon.ServiceError {
	if node.OnSuccess == "" {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.start_node_missing_on_success_description",
			DefaultValue: "START node '{{param(nodeID)}}' must have onSuccess",
			Params:       map[string]string{"nodeID": node.ID},
		})
	}
	if node.Executor != nil {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.start_node_has_executor_description",
			DefaultValue: "START node '{{param(nodeID)}}' must not have an executor",
			Params:       map[string]string{"nodeID": node.ID},
		})
	}
	if len(node.Prompts) > 0 {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.start_node_has_prompts_description",
			DefaultValue: "START node '{{param(nodeID)}}' must not have prompts",
			Params:       map[string]string{"nodeID": node.ID},
		})
	}
	if node.OnFailure != "" {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.start_node_has_on_failure_description",
			DefaultValue: "START node '{{param(nodeID)}}' must not have onFailure",
			Params:       map[string]string{"nodeID": node.ID},
		})
	}
	if node.OnIncomplete != "" {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.start_node_has_on_incomplete_description",
			DefaultValue: "START node '{{param(nodeID)}}' must not have onIncomplete",
			Params:       map[string]string{"nodeID": node.ID},
		})
	}
	return nil
}

// validateEndNode validates that an END node has no outgoing edges, executor, or prompts.
func (v *flowValidator) validateEndNode(node *providers.NodeDefinition) *tidcommon.ServiceError {
	if node.OnSuccess != "" {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.end_node_has_on_success_description",
			DefaultValue: "END node '{{param(nodeID)}}' must not have onSuccess",
			Params:       map[string]string{"nodeID": node.ID},
		})
	}
	if node.OnFailure != "" {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.end_node_has_on_failure_description",
			DefaultValue: "END node '{{param(nodeID)}}' must not have onFailure",
			Params:       map[string]string{"nodeID": node.ID},
		})
	}
	if node.OnIncomplete != "" {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.end_node_has_on_incomplete_description",
			DefaultValue: "END node '{{param(nodeID)}}' must not have onIncomplete",
			Params:       map[string]string{"nodeID": node.ID},
		})
	}
	if node.Executor != nil {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.end_node_has_executor_description",
			DefaultValue: "END node '{{param(nodeID)}}' must not have an executor",
			Params:       map[string]string{"nodeID": node.ID},
		})
	}
	if len(node.Prompts) > 0 {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.end_node_has_prompts_description",
			DefaultValue: "END node '{{param(nodeID)}}' must not have prompts",
			Params:       map[string]string{"nodeID": node.ID},
		})
	}
	if node.Next != "" {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.end_node_has_next_description",
			DefaultValue: "END node '{{param(nodeID)}}' must not have next",
			Params:       map[string]string{"nodeID": node.ID},
		})
	}
	return nil
}

// validateTaskExecutionNode validates the format of a TASK_EXECUTION node.
func (v *flowValidator) validateTaskExecutionNode(
	node *providers.NodeDefinition, nodeIndex map[string]*providers.NodeDefinition,
) *tidcommon.ServiceError {
	if node.Executor == nil || node.Executor.Name == "" {
		return tidcommon.CustomServiceError(ErrorInvalidNodeConfig, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.task_node_missing_executor_description",
			DefaultValue: "TASK_EXECUTION node '{{param(nodeID)}}' must have an executor with a non-empty name",
			Params:       map[string]string{"nodeID": node.ID},
		})
	}
	if node.OnSuccess == "" {
		return tidcommon.CustomServiceError(ErrorInvalidNodeConfig, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.task_node_missing_on_success_description",
			DefaultValue: "TASK_EXECUTION node '{{param(nodeID)}}' must have onSuccess",
			Params:       map[string]string{"nodeID": node.ID},
		})
	}
	if node.OnFailure != "" {
		if target, ok := nodeIndex[node.OnFailure]; ok && target.Type != string(common.NodeTypePrompt) {
			return tidcommon.CustomServiceError(ErrorInvalidNodeConfig, tidcommon.I18nMessage{
				Key:          "error.flowmgtservice.task_node_invalid_failure_target_description",
				DefaultValue: "TASK_EXECUTION node '{{param(nodeID)}}': onFailure must point to a PROMPT node",
				Params:       map[string]string{"nodeID": node.ID},
			})
		}
	}
	if node.OnIncomplete != "" {
		if target, ok := nodeIndex[node.OnIncomplete]; ok &&
			target.Type != string(common.NodeTypePrompt) {
			return tidcommon.CustomServiceError(
				ErrorInvalidNodeConfig, tidcommon.I18nMessage{
					Key:          "error.flowmgtservice.task_node_invalid_incomplete_target_description",
					DefaultValue: "TASK_EXECUTION node '{{param(nodeID)}}': onIncomplete must point to a PROMPT node",
					Params:       map[string]string{"nodeID": node.ID},
				})
		}
	}
	return nil
}

// validatePromptNode validates a PROMPT node by dispatching to the appropriate sub-validator.
func (v *flowValidator) validatePromptNode(node *providers.NodeDefinition) *tidcommon.ServiceError {
	hasPrompts := len(node.Prompts) > 0
	hasNext := node.Next != ""

	if hasPrompts && hasNext {
		return tidcommon.CustomServiceError(ErrorInvalidNodeConfig, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.prompt_node_has_both_prompts_and_next_description",
			DefaultValue: "PROMPT node '{{param(nodeID)}}' must have either prompts or next, not both",
			Params:       map[string]string{"nodeID": node.ID},
		})
	}
	if !hasPrompts && !hasNext {
		return tidcommon.CustomServiceError(ErrorInvalidNodeConfig, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.prompt_node_missing_prompts_or_next_description",
			DefaultValue: "PROMPT node '{{param(nodeID)}}' must have either prompts or next",
			Params:       map[string]string{"nodeID": node.ID},
		})
	}

	if hasNext {
		return v.validateDisplayOnlyPromptNode(node)
	}
	return v.validateInteractivePromptNode(node)
}

// validateDisplayOnlyPromptNode validates a display-only PROMPT node (has next, no prompts).
func (v *flowValidator) validateDisplayOnlyPromptNode(node *providers.NodeDefinition) *tidcommon.ServiceError {
	return nil
}

// validateInteractivePromptNode validates an interactive PROMPT node (has prompts with actions).
func (v *flowValidator) validateInteractivePromptNode(node *providers.NodeDefinition) *tidcommon.ServiceError {
	for i, prompt := range node.Prompts {
		if prompt.Action == nil || prompt.Action.NextNode == "" {
			return tidcommon.CustomServiceError(ErrorInvalidNodeConfig, tidcommon.I18nMessage{
				Key: "error.flowmgtservice.prompt_missing_action_description",
				DefaultValue: "PROMPT node '{{param(nodeID)}}': prompt at index " +
					"{{param(index)}} must have an action with nextNode",
				Params: map[string]string{"nodeID": node.ID, "index": strconv.Itoa(i)},
			})
		}
		if err := v.validateInputDefinitions(node.ID, prompt.Inputs); err != nil {
			return err
		}
	}
	return nil
}

// validateInputDefinitions validates input definitions for valid types and validation rules.
func (v *flowValidator) validateInputDefinitions(
	nodeID string, inputs []providers.InputDefinition,
) *tidcommon.ServiceError {
	for _, input := range inputs {
		if input.Type != "" && !providers.ValidInputTypes[input.Type] {
			return tidcommon.CustomServiceError(ErrorInvalidInputConfig, tidcommon.I18nMessage{
				Key:          "error.flowmgtservice.invalid_input_type_description",
				DefaultValue: "Node '{{param(nodeID)}}': input '{{param(inputID)}}' has invalid type '{{param(type)}}'",
				Params:       map[string]string{"nodeID": nodeID, "inputID": input.Identifier, "type": input.Type},
			})
		}
		if err := v.validateValidationRules(nodeID, input.Identifier, input.Validation); err != nil {
			return err
		}
	}
	return nil
}

// validateValidationRules validates the validation rules for a single input definition.
func (v *flowValidator) validateValidationRules(
	nodeID string, inputID string, rules []providers.ValidationRuleDefinition,
) *tidcommon.ServiceError {
	for _, rule := range rules {
		if !providers.ValidValidationRuleTypes[rule.Type] {
			return tidcommon.CustomServiceError(ErrorInvalidInputConfig, tidcommon.I18nMessage{
				Key: "error.flowmgtservice.invalid_validation_rule_type_description",
				DefaultValue: "Node '{{param(nodeID)}}': input '{{param(inputID)}}' " +
					"has invalid validation rule type '{{param(ruleType)}}'",
				Params: map[string]string{"nodeID": nodeID, "inputID": inputID, "ruleType": rule.Type},
			})
		}
		if rule.Type == "regex" {
			pattern, ok := rule.Value.(string)
			if !ok {
				return tidcommon.CustomServiceError(ErrorInvalidInputConfig, tidcommon.I18nMessage{
					Key: "error.flowmgtservice.regex_rule_value_not_string_description",
					DefaultValue: "Node '{{param(nodeID)}}': input '{{param(inputID)}}' " +
						"regex validation rule value must be a string",
					Params: map[string]string{"nodeID": nodeID, "inputID": inputID},
				})
			}
			if _, err := regexp.Compile(pattern); err != nil {
				return tidcommon.CustomServiceError(ErrorInvalidInputConfig, tidcommon.I18nMessage{
					Key: "error.flowmgtservice.invalid_regex_pattern_description",
					DefaultValue: "Node '{{param(nodeID)}}': input '{{param(inputID)}}' " +
						"has invalid regex pattern: {{param(error)}}",
					Params: map[string]string{"nodeID": nodeID, "inputID": inputID, "error": err.Error()},
				})
			}
		}
	}
	return nil
}

// validateCallNode validates the format of a CALL node.
func (v *flowValidator) validateCallNode(node *providers.NodeDefinition) *tidcommon.ServiceError {
	if node.Flow == nil || node.Flow.Ref == "" {
		return tidcommon.CustomServiceError(ErrorInvalidNodeConfig, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.call_node_missing_flow_ref_description",
			DefaultValue: "CALL node '{{param(nodeID)}}' must have a flow reference with a non-empty ref",
			Params:       map[string]string{"nodeID": node.ID},
		})
	}
	if node.OnSuccess == "" {
		return tidcommon.CustomServiceError(ErrorInvalidNodeConfig, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.call_node_missing_on_success_description",
			DefaultValue: "CALL node '{{param(nodeID)}}' must have onSuccess",
			Params:       map[string]string{"nodeID": node.ID},
		})
	}
	if node.Executor != nil {
		return tidcommon.CustomServiceError(ErrorInvalidNodeConfig, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.call_node_has_executor_description",
			DefaultValue: "CALL node '{{param(nodeID)}}' must not have an executor",
			Params:       map[string]string{"nodeID": node.ID},
		})
	}
	if len(node.Prompts) > 0 {
		return tidcommon.CustomServiceError(ErrorInvalidNodeConfig, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.call_node_has_prompts_description",
			DefaultValue: "CALL node '{{param(nodeID)}}' must not have prompts",
			Params:       map[string]string{"nodeID": node.ID},
		})
	}
	if node.Next != "" {
		return tidcommon.CustomServiceError(ErrorInvalidNodeConfig, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.call_node_has_next_description",
			DefaultValue: "CALL node '{{param(nodeID)}}' must not have next",
			Params:       map[string]string{"nodeID": node.ID},
		})
	}
	if node.OnIncomplete != "" {
		return tidcommon.CustomServiceError(ErrorInvalidNodeConfig, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.call_node_has_on_incomplete_description",
			DefaultValue: "CALL node '{{param(nodeID)}}' must not have onIncomplete",
			Params:       map[string]string{"nodeID": node.ID},
		})
	}
	return nil
}

// ---------------------------------------------------------------------------
// Scope: Executor validation
// ---------------------------------------------------------------------------

// validateExecutorGenericConstraints validates that the executor referenced in a task execution node
// is registered in the executor registry, then checks meta-based constraints.
func (v *flowValidator) validateExecutorGenericConstraints(
	node *providers.NodeDefinition, flowType providers.FlowType,
) *tidcommon.ServiceError {
	if !v.executorRegistry.IsRegistered(node.Executor.Name) {
		return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.executor_not_registered_description",
			DefaultValue: "Node '{{param(nodeID)}}': executor '{{param(executorName)}}' is not registered",
			Params:       map[string]string{"nodeID": node.ID, "executorName": node.Executor.Name},
		})
	}
	return v.validateExecutorMeta(node, flowType)
}

// validateExecutorMeta performs meta-based validation for an executor node:
// mode, flow type compatibility, and required node properties.
// If no meta is declared for the executor, all checks are skipped (backward-compatible).
func (v *flowValidator) validateExecutorMeta(
	node *providers.NodeDefinition, flowType providers.FlowType,
) *tidcommon.ServiceError {
	meta, err := v.executorRegistry.GetExecutorMeta(node.Executor.Name)
	if err != nil || meta == nil {
		return nil
	}

	if err := v.validateExecutorMode(node, meta); err != nil {
		return err
	}
	if err := v.validateExecutorFlowType(node, meta, flowType); err != nil {
		return err
	}
	return v.validateExecutorProperties(node, meta)
}

// validateExecutorSpecificConstraints dispatches executor-specific validation rules
// that go beyond the generic meta-based checks.
func (v *flowValidator) validateExecutorSpecificConstraints(
	node *providers.NodeDefinition,
	nodeIndex map[string]*providers.NodeDefinition, nodes []providers.NodeDefinition,
	flowType providers.FlowType,
) *tidcommon.ServiceError {
	switch node.Executor.Name {
	case executormeta.ExecutorNameSSOCheck:
		return v.validateSSOCheckExecutor(node, nodeIndex)
	case executormeta.ExecutorNameSession:
		return v.validateSessionExecutor(node, nodes)
	case executormeta.ExecutorNameAccountLinking:
		return v.validateAccountLinkingExecutor(node, nodeIndex)
	case executormeta.ExecutorNameOUResolver:
		return v.validateOUResolverExecutor(node, flowType)
	}
	return nil
}

// validateOUResolverExecutor rejects OU resolver properties and inputs that the node's strategy or flow type would
// ignore, and requires parentOuId on a node that prompts in an authentication flow, where UserTypeResolver sets no
// default OU.
func (v *flowValidator) validateOUResolverExecutor(
	node *providers.NodeDefinition, flowType providers.FlowType,
) *tidcommon.ServiceError {
	if err := validateOUResolverPropertyValues(node); err != nil {
		return err
	}

	resolveFrom, _ := node.Properties[common.NodePropertyOUResolveFrom].(string)
	parentOU, _ := node.Properties[common.NodePropertyOUParent].(string)
	hasParentOU := strings.TrimSpace(parentOU) != ""
	directChildrenOnly, _ := node.Properties[common.NodePropertyOUDirectChildrenOnly].(bool)

	if hasParentOU && (resolveFrom != ouResolveFromPrompt || flowType != providers.FlowTypeAuthentication) {
		return ouResolverUnsupportedProperty(node, common.NodePropertyOUParent,
			"with the prompt strategy in authentication flows")
	}
	if directChildrenOnly && resolveFrom != ouResolveFromPrompt {
		return ouResolverUnsupportedProperty(node, common.NodePropertyOUDirectChildrenOnly,
			"with the prompt strategy")
	}
	if err := validateOUResolverInputs(node, resolveFrom == ouResolveFromPrompt && directChildrenOnly); err != nil {
		return err
	}

	if resolveFrom == ouResolveFromPrompt && flowType == providers.FlowTypeAuthentication && !hasParentOU {
		return missingRequiredPropertyError(node, common.NodePropertyOUParent)
	}
	return nil
}

// validateOUResolverPropertyValues rejects the OU resolver properties added with direct children selection whose
// value the executor would not read: a non-string parentOuId or a non-boolean directChildrenOnly.
func validateOUResolverPropertyValues(node *providers.NodeDefinition) *tidcommon.ServiceError {
	if val, ok := node.Properties[common.NodePropertyOUParent]; ok {
		if _, isString := val.(string); !isString {
			return invalidPropertyValueError(node, common.NodePropertyOUParent, "a string")
		}
	}
	if val, ok := node.Properties[common.NodePropertyOUDirectChildrenOnly]; ok {
		if _, isBool := val.(bool); !isBool {
			return invalidPropertyValueError(node, common.NodePropertyOUDirectChildrenOnly, "a boolean")
		}
	}
	return nil
}

// invalidPropertyValueError reports a property whose value is not of the kind the executor reads.
func invalidPropertyValueError(
	node *providers.NodeDefinition, propertyKey, expected string,
) *tidcommon.ServiceError {
	return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
		Key: "error.flowmgtservice.invalid_ou_resolver_property_value_description",
		DefaultValue: "Node '{{param(nodeID)}}': executor '{{param(executorName)}}' " +
			"requires property '{{param(propertyKey)}}' to be {{param(expected)}}",
		Params: map[string]string{
			"nodeID":       node.ID,
			"executorName": node.Executor.Name,
			"propertyKey":  propertyKey,
			"expected":     expected,
		},
	})
}

// validateOUResolverInputs checks the inputs declared on an OU resolver node. In direct children mode the node may
// declare a single required input, ouId or ouHandle, which decides how the user names the OU. Declaring ouHandle
// elsewhere is rejected because only direct children mode reads it.
func validateOUResolverInputs(node *providers.NodeDefinition, directChildrenMode bool) *tidcommon.ServiceError {
	inputs := node.Executor.Inputs
	if !directChildrenMode {
		for _, input := range inputs {
			if input.Identifier == ouHandleInputIdentifier {
				return ouResolverUnsupportedProperty(node, input.Identifier,
					"with the prompt strategy and directChildrenOnly")
			}
		}
		return nil
	}
	if len(inputs) == 0 {
		return nil
	}
	if len(inputs) == 1 && inputs[0].Required &&
		(inputs[0].Identifier == ouIDInputIdentifier || inputs[0].Identifier == ouHandleInputIdentifier) {
		return nil
	}

	return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
		Key: "error.flowmgtservice.invalid_ou_resolver_inputs_description",
		DefaultValue: "Node '{{param(nodeID)}}': executor '{{param(executorName)}}' must declare a single " +
			"required input, '{{param(ouIDInput)}}' or '{{param(ouHandleInput)}}', when directChildrenOnly is set",
		Params: map[string]string{
			"nodeID":        node.ID,
			"executorName":  node.Executor.Name,
			"ouIDInput":     ouIDInputIdentifier,
			"ouHandleInput": ouHandleInputIdentifier,
		},
	})
}

// ouResolverUnsupportedProperty reports an OU resolver property used outside the configuration it supports, and
// names the configuration that does.
func ouResolverUnsupportedProperty(
	node *providers.NodeDefinition, propertyKey, supportedWith string,
) *tidcommon.ServiceError {
	return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
		Key: "error.flowmgtservice.unsupported_ou_resolver_property_description",
		DefaultValue: "Node '{{param(nodeID)}}': executor '{{param(executorName)}}' supports " +
			"'{{param(propertyKey)}}' only {{param(supportedWith)}}",
		Params: map[string]string{
			"nodeID":        node.ID,
			"executorName":  node.Executor.Name,
			"propertyKey":   propertyKey,
			"supportedWith": supportedWith,
		},
	})
}

// missingRequiredPropertyError reports a property the executor requires that the node does not set.
func missingRequiredPropertyError(node *providers.NodeDefinition, propertyKey string) *tidcommon.ServiceError {
	return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
		Key: "error.flowmgtservice.missing_required_executor_property_description",
		DefaultValue: "Node '{{param(nodeID)}}': executor '{{param(executorName)}}' " +
			"requires property '{{param(propertyKey)}}'",
		Params: map[string]string{
			"nodeID":       node.ID,
			"executorName": node.Executor.Name,
			"propertyKey":  propertyKey,
		},
	})
}

// unsupportedPropertyError reports a property the executor does not support in the node's configuration.
func unsupportedPropertyError(node *providers.NodeDefinition, propertyKey string) *tidcommon.ServiceError {
	return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
		Key: "error.flowmgtservice.unsupported_executor_property_description",
		DefaultValue: "Node '{{param(nodeID)}}': executor '{{param(executorName)}}' " +
			"does not support property '{{param(propertyKey)}}'",
		Params: map[string]string{
			"nodeID":       node.ID,
			"executorName": node.Executor.Name,
			"propertyKey":  propertyKey,
		},
	})
}

// validateExecutorMode checks that the executor mode configured in the node is supported.
// If the executor declares supported modes and the node omits a mode, the executor's default
// mode (if any) is accepted; otherwise the node must specify one of the supported modes.
func (v *flowValidator) validateExecutorMode(
	node *providers.NodeDefinition, meta *providers.ExecutorMeta,
) *tidcommon.ServiceError {
	if len(meta.SupportedModes) == 0 {
		return nil
	}
	if node.Executor.Mode == "" {
		if meta.DefaultMode != "" {
			return nil
		}
		return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.executor_mode_required_description",
			DefaultValue: "Node '{{param(nodeID)}}': executor '{{param(executorName)}}' requires a mode",
			Params:       map[string]string{"nodeID": node.ID, "executorName": node.Executor.Name},
		})
	}
	for _, m := range meta.SupportedModes {
		if m == node.Executor.Mode {
			return nil
		}
	}
	return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
		Key: "error.flowmgtservice.unsupported_executor_mode_description",
		DefaultValue: "Node '{{param(nodeID)}}': executor '{{param(executorName)}}' " +
			"does not support mode '{{param(mode)}}'",
		Params: map[string]string{
			"nodeID":       node.ID,
			"executorName": node.Executor.Name,
			"mode":         node.Executor.Mode,
		},
	})
}

// validateExecutorFlowType checks that the executor supports the current flow type.
func (v *flowValidator) validateExecutorFlowType(
	node *providers.NodeDefinition, meta *providers.ExecutorMeta, flowType providers.FlowType,
) *tidcommon.ServiceError {
	if len(meta.SupportedFlowTypes) == 0 {
		return nil
	}
	for _, ft := range meta.SupportedFlowTypes {
		if ft == flowType {
			return nil
		}
	}
	return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
		Key: "error.flowmgtservice.unsupported_executor_flow_type_description",
		DefaultValue: "Node '{{param(nodeID)}}': executor '{{param(executorName)}}' " +
			"is not compatible with flow type '{{param(flowType)}}'",
		Params: map[string]string{
			"nodeID":       node.ID,
			"executorName": node.Executor.Name,
			"flowType":     string(flowType),
		},
	})
}

// validateExecutorProperties validates node properties against executor metadata.
// It checks that all properties defined in the node are supported by the executor,
// and that all required properties are present and non-empty.
func (v *flowValidator) validateExecutorProperties(
	node *providers.NodeDefinition, meta *providers.ExecutorMeta,
) *tidcommon.ServiceError {
	if len(meta.SupportedProperties) == 0 {
		return nil
	}

	// Resolve effective mode: explicit node mode, or executor default.
	effectiveMode := node.Executor.Mode
	if effectiveMode == "" {
		effectiveMode = meta.DefaultMode
	}

	supported := make(map[string]bool, len(meta.SupportedProperties))
	for _, prop := range meta.SupportedProperties {
		supported[prop.Property] = true
		// Check required properties
		if prop.IsRequired {
			// If RequiredForModes is specified, only enforce for those modes.
			if len(prop.ApplicableModes) > 0 && !containsMode(prop.ApplicableModes, effectiveMode) {
				continue
			}
			val, ok := node.Properties[prop.Property]
			if !ok || val == nil || val == "" {
				return missingRequiredPropertyError(node, prop.Property)
			}
		}
	}

	// Check for unsupported properties
	for key := range node.Properties {
		if !supported[key] {
			return unsupportedPropertyError(node, key)
		}
	}
	return nil
}

// containsMode returns true if the given mode is present in the modes slice.
func containsMode(modes []string, mode string) bool {
	for _, m := range modes {
		if m == mode {
			return true
		}
	}
	return false
}

// validateSSOCheckExecutor validates that an SSOCheck node's checkpointRef property references
// a valid SessionExecutor node.
func (v *flowValidator) validateSSOCheckExecutor(
	node *providers.NodeDefinition, nodeIndex map[string]*providers.NodeDefinition,
) *tidcommon.ServiceError {
	ref, ok := node.Properties[common.NodePropertyCheckpointRef]
	if !ok || ref == nil {
		return nil // Already caught by ExecutorMeta required-property validation.
	}
	refStr, ok := ref.(string)
	if !ok {
		return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.checkpoint_ref_not_string_description",
			DefaultValue: "Node '{{param(nodeID)}}': checkpointRef must be a string",
			Params:       map[string]string{"nodeID": node.ID},
		})
	}
	target, exists := nodeIndex[refStr]
	if !exists {
		return tidcommon.CustomServiceError(ErrorInvalidNodeReference, tidcommon.I18nMessage{
			Key: "error.flowmgtservice.checkpoint_ref_invalid_target_description",
			DefaultValue: "Node '{{param(nodeID)}}': checkpointRef references " +
				"non-existent node '{{param(targetNodeID)}}'",
			Params: map[string]string{"nodeID": node.ID, "targetNodeID": refStr},
		})
	}
	if target.Type != string(common.NodeTypeTaskExecution) ||
		target.Executor == nil || target.Executor.Name != executormeta.ExecutorNameSession {
		return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
			Key: "error.flowmgtservice.checkpoint_ref_not_session_description",
			DefaultValue: "Node '{{param(nodeID)}}': checkpointRef must reference " +
				"a SessionExecutor node, got '{{param(targetNodeID)}}'",
			Params: map[string]string{"nodeID": node.ID, "targetNodeID": refStr},
		})
	}
	return nil
}

// validateSessionExecutor validates that a SessionExecutor node is referenced by at least one
// SSOCheckExecutor via checkpointRef.
func (v *flowValidator) validateSessionExecutor(
	node *providers.NodeDefinition, nodes []providers.NodeDefinition,
) *tidcommon.ServiceError {
	for _, n := range nodes {
		if n.Type == string(common.NodeTypeTaskExecution) &&
			n.Executor != nil && n.Executor.Name == executormeta.ExecutorNameSSOCheck {
			if ref, ok := n.Properties[common.NodePropertyCheckpointRef].(string); ok && ref == node.ID {
				return nil
			}
		}
	}
	return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
		Key: "error.flowmgtservice.orphan_session_executor_description",
		DefaultValue: "SessionExecutor node '{{param(nodeID)}}' is not referenced " +
			"by any SSOCheckExecutor via checkpointRef",
		Params: map[string]string{"nodeID": node.ID},
	})
}

// validateExtensionSteps refuses a flow whose extension step has an unusable property value or names
// an HTTP connection that does not exist.
func (v *flowValidator) validateExtensionSteps(
	ctx context.Context, nodes []providers.NodeDefinition,
) *tidcommon.ServiceError {
	for i := range nodes {
		if nodes[i].Executor == nil || nodes[i].Executor.Name != executormeta.ExecutorNameExtension {
			continue
		}
		if err := v.validateExtensionExecutor(ctx, &nodes[i]); err != nil {
			return err
		}
	}
	return nil
}

// validateExtensionExecutor applies the executor's own property rule to an extension step and checks
// that its HTTP connection exists.
func (v *flowValidator) validateExtensionExecutor(
	ctx context.Context, node *providers.NodeDefinition,
) *tidcommon.ServiceError {
	props, invalidProperty := common.ParseExtensionProperties(node.Properties)
	if invalidProperty != "" {
		return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
			Key: "error.flowmgtservice.invalid_executor_property_value_description",
			DefaultValue: "Node '{{param(nodeID)}}': executor '{{param(executorName)}}' " +
				"has an invalid value for property '{{param(propertyKey)}}'",
			Params: map[string]string{
				"nodeID":       node.ID,
				"executorName": node.Executor.Name,
				"propertyKey":  invalidProperty,
			},
		})
	}
	_, svcErr := v.httpConnectionProvider.GetHTTPConnection(ctx, props.ConnectionID)
	if svcErr == nil {
		return nil
	}
	if svcErr.Code == httpconnection.ErrorNotFound.Code {
		return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
			Key: "error.flowmgtservice.extension_connection_not_found_description",
			DefaultValue: "Node '{{param(nodeID)}}': HTTP connection '{{param(connectionID)}}' " +
				"does not exist",
			Params: map[string]string{
				"nodeID":       node.ID,
				"connectionID": props.ConnectionID,
				"propertyKey":  common.NodePropertyConnectionID,
			},
		})
	}
	return &tidcommon.InternalServerError
}

// validateAccountLinkingExecutor checks that a linking node has an onIncomplete prompt whose
// refusal reaches this node.
func (v *flowValidator) validateAccountLinkingExecutor(
	node *providers.NodeDefinition, nodeIndex map[string]*providers.NodeDefinition,
) *tidcommon.ServiceError {
	if node.OnIncomplete == "" {
		return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
			Key: "error.flowmgtservice.linking_requires_on_incomplete_description",
			DefaultValue: "Node '{{param(nodeID)}}': AccountLinkingExecutor requires " +
				"onIncomplete, which must point to the step that verifies the account",
			Params: map[string]string{"nodeID": node.ID},
		})
	}
	prompt := nodeIndex[node.OnIncomplete]
	if err := validateLinkingRejectActions(node, prompt); err != nil {
		return err
	}
	if err := validateLinkingConfirmActions(node, prompt, nodeIndex); err != nil {
		return err
	}
	return validateLinkingFailurePath(node, nodeIndex)
}

// validateLinkingRejectActions checks that every REJECT action on the linking prompt points back at
// the linking node, since a forwarded action type reaches only the node it points at.
func validateLinkingRejectActions(
	node *providers.NodeDefinition, prompt *providers.NodeDefinition,
) *tidcommon.ServiceError {
	if prompt == nil {
		return nil
	}
	for _, p := range prompt.Prompts {
		if p.Action == nil || common.ActionType(p.Action.Type) != common.ActionTypeReject {
			continue
		}
		if p.Action.NextNode == node.ID {
			continue
		}
		return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
			Key: "error.flowmgtservice.linking_decision_action_misrouted_description",
			DefaultValue: "Node '{{param(nodeID)}}': the '{{param(actionType)}}' action " +
				"'{{param(actionRef)}}' on '{{param(promptNodeID)}}' must point back to this node, " +
				"since a forwarded action type reaches only the node it points at",
			Params: map[string]string{
				"nodeID":       node.ID,
				"actionType":   string(common.ActionTypeReject),
				"actionRef":    p.Action.Ref,
				"promptNodeID": prompt.ID,
			},
		})
	}
	return nil
}

// validateLinkingConfirmActions checks that every CONFIRM action on the linking prompt starts a
// verification, through a CALL node or steps in this flow, that returns to the linking node.
func validateLinkingConfirmActions(node, prompt *providers.NodeDefinition,
	nodeIndex map[string]*providers.NodeDefinition) *tidcommon.ServiceError {
	if prompt == nil {
		return nil
	}
	for _, p := range prompt.Prompts {
		if p.Action == nil || common.ActionType(p.Action.Type) != common.ActionTypeConfirm {
			continue
		}
		target := nodeIndex[p.Action.NextNode]
		if target == nil {
			continue
		}
		params := map[string]string{
			"nodeID":       node.ID,
			"actionType":   string(common.ActionTypeConfirm),
			"actionRef":    p.Action.Ref,
			"promptNodeID": prompt.ID,
		}
		if target.Type == string(common.NodeTypeCall) {
			if target.OnSuccess != node.ID {
				params["callNodeID"] = target.ID
				return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
					Key: "error.flowmgtservice.linking_verification_call_not_returning_description",
					DefaultValue: "Node '{{param(nodeID)}}': the CALL node '{{param(callNodeID)}}' that " +
						"the '{{param(actionType)}}' action '{{param(actionRef)}}' on " +
						"'{{param(promptNodeID)}}' points to must return to this node through onSuccess",
					Params: params,
				})
			}
			continue
		}
		if err := validateLinkingVerificationSegment(node, target, nodeIndex, params); err != nil {
			return err
		}
	}
	return nil
}

// linkingSegmentForbiddenExecutors are the executors that must not run between CONFIRM and the
// return to the linking node: each would act before the checkpoint is restored and the verified
// account is checked against the candidates.
var linkingSegmentForbiddenExecutors = map[string]bool{
	executormeta.ExecutorNameAccountLinking: true,
	executormeta.ExecutorNameProvisioning:   true,
	executormeta.ExecutorNameAuthAssert:     true,
}

// validateLinkingVerificationSegment checks the verification steps a CONFIRM action runs in this
// flow. The segment is every node reachable from the action's target without passing through the
// linking node. It must not end the flow or run an executor that acts before the linking node settles
// the verification, and it must lead back to the linking node.
func validateLinkingVerificationSegment(node, start *providers.NodeDefinition,
	nodeIndex map[string]*providers.NodeDefinition, params map[string]string) *tidcommon.ServiceError {
	returns := false
	visited := map[string]bool{start.ID: true}
	queue := []*providers.NodeDefinition{start}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		forbidden := current.Type == string(common.NodeTypeEnd) ||
			(current.Executor != nil && linkingSegmentForbiddenExecutors[current.Executor.Name])
		if forbidden {
			params["segmentNodeID"] = current.ID
			return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
				Key: "error.flowmgtservice.linking_verification_segment_forbidden_node_description",
				DefaultValue: "Node '{{param(nodeID)}}': the verification that the '{{param(actionType)}}' " +
					"action '{{param(actionRef)}}' on '{{param(promptNodeID)}}' starts reaches " +
					"'{{param(segmentNodeID)}}' before returning to this node, and must not end the flow, " +
					"link, provision or sign in before it returns",
				Params: params,
			})
		}

		for _, ref := range collectAllNodeReferences([]providers.NodeDefinition{*current}) {
			if ref.targetNodeID == node.ID {
				returns = true
				continue
			}
			next := nodeIndex[ref.targetNodeID]
			if next == nil || visited[next.ID] {
				continue
			}
			visited[next.ID] = true
			queue = append(queue, next)
		}
	}

	if !returns {
		return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
			Key: "error.flowmgtservice.linking_verification_segment_not_returning_description",
			DefaultValue: "Node '{{param(nodeID)}}': the verification that the '{{param(actionType)}}' " +
				"action '{{param(actionRef)}}' on '{{param(promptNodeID)}}' starts never returns to " +
				"this node",
			Params: params,
		})
	}
	return nil
}

// linkingFederatedExecutors are the executors that sign in at a connection, replacing whoever
// verified with the identity the connection returns.
var linkingFederatedExecutors = map[string]bool{
	executormeta.ExecutorNameOAuth:      true,
	executormeta.ExecutorNameOIDCAuth:   true,
	executormeta.ExecutorNameGoogleAuth: true,
	executormeta.ExecutorNameGitHubAuth: true,
}

// validateLinkingFailurePath checks that the linking node's onFailure does not lead back to it
// without passing through a federated executor's onSuccess, which signs in again first.
func validateLinkingFailurePath(node *providers.NodeDefinition,
	nodeIndex map[string]*providers.NodeDefinition) *tidcommon.ServiceError {
	start := nodeIndex[node.OnFailure]
	if start == nil {
		return nil
	}
	visited := map[string]bool{start.ID: true}
	queue := []*providers.NodeDefinition{start}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		federated := current.Executor != nil && linkingFederatedExecutors[current.Executor.Name]

		for _, ref := range collectAllNodeReferences([]providers.NodeDefinition{*current}) {
			if federated && ref.fieldName == "onSuccess" {
				continue
			}
			if ref.targetNodeID == node.ID {
				return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
					Key: "error.flowmgtservice.linking_failure_path_returns_description",
					DefaultValue: "Node '{{param(nodeID)}}': the path from its onFailure target " +
						"'{{param(failureNodeID)}}' must sign in at a connection again before it returns to " +
						"this node",
					Params: map[string]string{"nodeID": node.ID, "failureNodeID": start.ID},
				})
			}
			next := nodeIndex[ref.targetNodeID]
			if next == nil || visited[next.ID] {
				continue
			}
			visited[next.ID] = true
			queue = append(queue, next)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Scope: Interceptor validation
// ---------------------------------------------------------------------------

// validateInterceptors validates interceptor definitions including format, registry
// registration, and applyTo node references.
func (v *flowValidator) validateInterceptors(
	interceptors []providers.InterceptorDefinition, nodeIndex map[string]*providers.NodeDefinition,
) *tidcommon.ServiceError {
	if err := v.validateInterceptorDefinitions(interceptors); err != nil {
		return err
	}

	if err := v.validateInterceptorApplyTo(interceptors, nodeIndex); err != nil {
		return err
	}

	return nil
}

// validateInterceptorDefinitions validates the format of each interceptor definition
// and checks that it is registered in the interceptor registry.
func (v *flowValidator) validateInterceptorDefinitions(
	interceptors []providers.InterceptorDefinition,
) *tidcommon.ServiceError {
	for i, ic := range interceptors {
		if err := v.validateInterceptorFormat(i, ic); err != nil {
			return err
		}
		if !v.interceptorRegistry.IsRegistered(ic.Name) {
			return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
				Key:          "error.flowmgtservice.interceptor_not_registered",
				DefaultValue: "Interceptor '{{param(interceptorName)}}' is not registered",
				Params:       map[string]string{"interceptorName": ic.Name},
			})
		}
	}
	return nil
}

// validateInterceptorFormat validates the format of a single interceptor definition
// (name, mode, scope, applyTo constraints) without requiring registry access.
func (v *flowValidator) validateInterceptorFormat(
	index int, ic providers.InterceptorDefinition,
) *tidcommon.ServiceError {
	if ic.Name == "" {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.interceptor_name_required",
			DefaultValue: "Interceptor at index {{param(index)}} must have a name",
			Params:       map[string]string{"index": strconv.Itoa(index)},
		})
	}
	if isDefaultInterceptor(ic.Name) {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.interceptor_default_not_configurable",
			DefaultValue: "Default interceptor '{{param(interceptorName)}}' cannot be configured in a flow definition",
			Params:       map[string]string{"interceptorName": ic.Name},
		})
	}
	if !providers.ValidInterceptorModes[ic.Mode] {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.interceptor_invalid_mode",
			DefaultValue: "Interceptor '{{param(interceptorName)}}' has invalid mode '{{param(mode)}}'",
			Params:       map[string]string{"interceptorName": ic.Name, "mode": string(ic.Mode)},
		})
	}
	if ic.Scope != "" && !providers.ValidInterceptorScopes[ic.Scope] {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.interceptor_invalid_scope",
			DefaultValue: "Interceptor '{{param(interceptorName)}}' has invalid scope '{{param(scope)}}'",
			Params:       map[string]string{"interceptorName": ic.Name, "scope": string(ic.Scope)},
		})
	}
	if ic.Scope == providers.InterceptorScopeSelected && len(ic.ApplyTo) == 0 {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.interceptor_selected_scope_requires_apply_to",
			DefaultValue: "Interceptor with scope SELECTED must specify at least one node in applyTo",
		})
	}
	return nil
}

// isDefaultInterceptor checks whether the given interceptor name matches any default interceptor.
func isDefaultInterceptor(name string) bool {
	_, ok := interceptor.DefaultInterceptorNames[name]
	return ok
}

// validateInterceptorApplyTo validates that interceptor applyTo node IDs reference existing nodes.
func (v *flowValidator) validateInterceptorApplyTo(
	interceptors []providers.InterceptorDefinition, nodeIndex map[string]*providers.NodeDefinition,
) *tidcommon.ServiceError {
	for _, ic := range interceptors {
		if ic.Scope != providers.InterceptorScopeSelected {
			continue
		}
		for _, nodeID := range ic.ApplyTo {
			if _, exists := nodeIndex[nodeID]; !exists {
				return tidcommon.CustomServiceError(
					ErrorInvalidNodeReference, tidcommon.I18nMessage{
						Key: "error.flowmgtservice.interceptor_invalid_apply_to_description",
						DefaultValue: "Interceptor '{{param(interceptorName)}}': " +
							"applyTo references non-existent node '{{param(nodeID)}}'",
						Params: map[string]string{"interceptorName": ic.Name, "nodeID": nodeID},
					})
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Scope: Variable validation
// ---------------------------------------------------------------------------

// identityProviderCategory is the category of a connection variable that takes a federated sign-in connection:
// google, github, oidc or oauth.
const identityProviderCategory = "IDENTITY_PROVIDER"

// identityProviderVendors are the connection vendors a connection variable of the identity provider category may name.
var identityProviderVendors = []string{"google", "github", "oidc", "oauth"}

// validateVariables checks the variables a flow declares and the placeholders that use them:
//   - a VARIABLES node goes on to a next node and does nothing else, and the VARIABLES nodes of a flow declare at
//     least one variable between them;
//   - each declaration is well formed with a valid default, and a variable is declared once across all the nodes;
//   - the default of a connection variable is a connection of the kind the variable takes;
//   - every {{var(name)}} in the properties of a step, or in the text of a prompt, names a declared variable, and
//     sits in a step that every route from the start reaches only through the node that declares it.
func (v *flowValidator) validateVariables(
	ctx context.Context, flowDef *FlowDefinition,
) *tidcommon.ServiceError {
	nodes := flowDef.Nodes
	declaredAt := map[string]string{}
	var firstVariablesNodeID string
	for i := range nodes {
		node := &nodes[i]
		if node.Type != string(common.NodeTypeVariables) {
			continue
		}
		if firstVariablesNodeID == "" {
			firstVariablesNodeID = node.ID
		}
		if err := v.validateVariablesNodeFormat(node); err != nil {
			return v.variableError(node.ID, err.Error())
		}
		if err := core.ValidateDefinitions(node.Inputs); err != nil {
			return v.variableError(node.ID, err.Error())
		}
		for _, def := range node.Inputs {
			if other, seen := declaredAt[def.Identifier]; seen {
				return v.variableError(node.ID, fmt.Sprintf(
					"variable %q is already declared on node '%s'", def.Identifier, other))
			}
			declaredAt[def.Identifier] = node.ID
			if svcErr := v.validateConnectionVariable(ctx, def); svcErr != nil {
				return svcErr
			}
		}
	}
	if firstVariablesNodeID != "" && len(declaredAt) == 0 {
		return v.variableError(firstVariablesNodeID,
			"the VARIABLES nodes of a flow declare at least one variable in 'inputs'")
	}
	if err := v.validateOUResolverBeforeVariables(nodes); err != nil {
		return err
	}
	return v.validateVariableReferences(nodes, declaredAt)
}

// validateOUResolverBeforeVariables checks that every route from the start to a VARIABLES node passes an organization
// unit resolver. A VARIABLES node resolves the values of the unit the resolver picked, so a route that reaches it
// without one would leave the flow with no unit to resolve for.
func (v *flowValidator) validateOUResolverBeforeVariables(nodes []providers.NodeDefinition) *tidcommon.ServiceError {
	var startID string
	var resolvers []string
	for i := range nodes {
		switch {
		case nodes[i].Type == string(common.NodeTypeStart) && startID == "":
			startID = nodes[i].ID
		case nodes[i].Executor != nil && nodes[i].Executor.Name == executormeta.ExecutorNameOUResolver:
			resolvers = append(resolvers, nodes[i].ID)
		}
	}
	adjacency := buildAdjacencyList(nodes)
	reachable := v.reachableAvoiding(adjacency, startID, resolvers...)
	for i := range nodes {
		if nodes[i].Type == string(common.NodeTypeVariables) && reachable[nodes[i].ID] {
			return v.variableError(nodes[i].ID, fmt.Sprintf(
				"a route reaches it without passing an %s step, and the node resolves the values of the "+
					"organization unit that step picks", executormeta.ExecutorNameOUResolver))
		}
	}
	return nil
}

// validateVariablesNodeFormat checks that a VARIABLES node goes on to a next node and carries nothing a node that
// shows and asks for nothing could not use.
func (v *flowValidator) validateVariablesNodeFormat(node *providers.NodeDefinition) error {
	switch {
	case node.OnSuccess == "":
		return fmt.Errorf("a VARIABLES node must have onSuccess")
	case node.OnFailure != "" || node.OnIncomplete != "" || node.Next != "":
		return fmt.Errorf("a VARIABLES node goes on through onSuccess only, with no onFailure, onIncomplete or next")
	case node.Executor != nil || len(node.Prompts) > 0 || len(node.Properties) > 0 || node.Meta != nil ||
		node.Flow != nil || node.Condition != nil || node.Variant != "" || node.Message != "":
		return fmt.Errorf(
			"a VARIABLES node has no executor, prompts, properties, meta, flow, condition, variant or message")
	}
	return nil
}

// validateConnectionVariable checks the declaration of a connection variable: it asks for a category and kinds of
// connection a value can be checked against, and its default is a connection of a kind it takes.
func (v *flowValidator) validateConnectionVariable(
	ctx context.Context, def providers.VariableDefinition,
) *tidcommon.ServiceError {
	if def.Type != providers.InputTypeConnectionSelect {
		return nil
	}
	if def.Category != identityProviderCategory {
		return v.variableError("", fmt.Sprintf("variable %q: the connection category %q is not supported",
			def.Identifier, def.Category))
	}
	for _, t := range def.ConnectionTypes {
		if !utils.ContainsFold(identityProviderVendors, t) {
			return v.variableError("", fmt.Sprintf(
				"variable %q: %q is not an identity provider connection type", def.Identifier, t))
		}
	}
	if v.idpProvider == nil {
		return nil
	}
	raw, err := core.DefaultRaw(def)
	if err != nil {
		return v.variableError("", fmt.Sprintf("variable %q: its default %s", def.Identifier, err.Error()))
	}
	if raw == "" {
		return nil
	}
	reason, svcErr := v.checkConnection(ctx, def, raw)
	if svcErr != nil {
		return svcErr
	}
	if reason != "" {
		return v.variableError("", fmt.Sprintf("variable %q: its default %s", def.Identifier, reason))
	}
	return nil
}

// checkConnection checks that a connection variable's value names an existing identity provider connection of a kind
// the variable takes. It returns what is wrong, or nothing, and reports a failure to read the connection as the error
// it is.
func (v *flowValidator) checkConnection(
	ctx context.Context, def providers.VariableDefinition, connectionID string,
) (string, *tidcommon.ServiceError) {
	idp, svcErr := v.idpProvider.GetIdentityProvider(security.WithRuntimeContext(ctx), connectionID)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			return "is not an existing connection", nil
		}
		return "", svcErr
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

// validateVariableReferences checks every {{var(name)}} of the steps: the variable is declared, and the step can only
// be reached through the node that declares it.
func (v *flowValidator) validateVariableReferences(
	nodes []providers.NodeDefinition, declaredAt map[string]string,
) *tidcommon.ServiceError {
	var startID string
	for i := range nodes {
		if nodes[i].Type == string(common.NodeTypeStart) {
			startID = nodes[i].ID
			break
		}
	}
	adjacency := buildAdjacencyList(nodes)
	// reachableWithout holds, for a declaring node, the nodes a route from the start can reach without passing it.
	reachableWithout := map[string]map[string]bool{}

	for i := range nodes {
		node := &nodes[i]
		refs := core.References(node.Properties)
		for _, id := range refs {
			declaringNode, declared := declaredAt[id]
			if !declared {
				return v.variableError(node.ID, fmt.Sprintf(
					"it refers to the variable %q, which the flow does not declare", id))
			}
			if _, done := reachableWithout[declaringNode]; !done {
				reachableWithout[declaringNode] = v.reachableAvoiding(adjacency, startID, declaringNode)
			}
			if node.ID == declaringNode || reachableWithout[declaringNode][node.ID] {
				return v.variableError(node.ID, fmt.Sprintf(
					"it refers to the variable %q, which the node '%s' declares, but a route reaches it "+
						"without passing that node", id, declaringNode))
			}
		}
	}
	return nil
}

// reachableAvoiding returns the nodes a route from start can reach without passing any of the avoided nodes.
func (v *flowValidator) reachableAvoiding(
	adjacency map[string][]string, start string, avoid ...string,
) map[string]bool {
	visited := map[string]bool{}
	if slices.Contains(avoid, start) {
		return visited
	}
	visited[start] = true
	queue := []string{start}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range adjacency[current] {
			if slices.Contains(avoid, next) || visited[next] {
				continue
			}
			visited[next] = true
			queue = append(queue, next)
		}
	}
	return visited
}

// variableError is the error for a flow whose variables are not valid.
func (v *flowValidator) variableError(nodeID, reason string) *tidcommon.ServiceError {
	if nodeID == "" {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.invalid_variable_description",
			DefaultValue: "{{param(reason)}}",
			Params:       map[string]string{"reason": reason},
		})
	}
	return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
		Key:          "error.flowmgtservice.invalid_variables_description",
		DefaultValue: "Node '{{param(nodeID)}}': {{param(reason)}}",
		Params:       map[string]string{"nodeID": nodeID, "reason": reason},
	})
}
