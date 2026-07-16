/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package flowmgt

import (
	"context"
	"regexp"
	"strconv"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/flow/graphbuilder"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// FlowValidatorInterface defines the contract for validating flow definitions.
type FlowValidatorInterface interface {
	// ValidateFlowDefinition validates the provided flow definition and returns a ServiceError if validation fails.
	ValidateFlowDefinition(ctx context.Context, flowDef *FlowDefinition) *tidcommon.ServiceError
}

// flowValidator is responsible for validating flow definitions,
// including metadata, structure, node configurations, and registry-dependent checks.
type flowValidator struct {
	executorRegistry    executor.ExecutorRegistryInterface
	interceptorRegistry interceptor.InterceptorRegistryInterface
	graphBuilder        graphbuilder.GraphBuilderInterface
	logger              *log.Logger
}

// newFlowValidator creates a new instance of flowValidator with the provided dependencies.
func newFlowValidator(
	executorRegistry executor.ExecutorRegistryInterface,
	interceptorRegistry interceptor.InterceptorRegistryInterface,
	graphBuilder graphbuilder.GraphBuilderInterface,
) *flowValidator {
	return &flowValidator{
		executorRegistry:    executorRegistry,
		interceptorRegistry: interceptorRegistry,
		graphBuilder:        graphBuilder,
		logger:              log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowValidator")),
	}
}

// ValidateFlowDefinition runs all validation scopes on the flow definition.
func (v *flowValidator) ValidateFlowDefinition(
	ctx context.Context, flowDef *FlowDefinition,
) *tidcommon.ServiceError {
	if err := v.validateMetadata(flowDef); err != nil {
		return err
	}

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

	if err := v.validateInterceptors(flowDef.Interceptors, nodeIndex); err != nil {
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
func (v *flowValidator) validateMetadata(flowDef *FlowDefinition) *tidcommon.ServiceError {
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

	return nil
}

// ---------------------------------------------------------------------------
// Scope: Flow type-based constraint validation (static maps, no registry)
// ---------------------------------------------------------------------------

// requiredExecutorsByFlowType maps flow type → executor names that must appear at least once.
var requiredExecutorsByFlowType = map[providers.FlowType][]string{
	providers.FlowTypeAuthentication: {executor.ExecutorNameAuthAssert},
	providers.FlowTypeRegistration:   {executor.ExecutorNameProvisioning, executor.ExecutorNameUserTypeResolver},
	providers.FlowTypeUserOnboarding: {executor.ExecutorNameProvisioning, executor.ExecutorNameUserTypeResolver},
}

// validateFlowTypeBasedConstraints checks forbidden and required executor rules for the flow type.
// Uses static maps only — no registry access required.
func (v *flowValidator) validateFlowTypeBasedConstraints(
	flowType providers.FlowType, nodes []providers.NodeDefinition,
) *tidcommon.ServiceError {
	return v.validateRequiredExecutors(flowType, nodes)
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
			if err := v.validateExecutorSpecificConstraints(node, nodeIndex, nodes); err != nil {
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
) *tidcommon.ServiceError {
	switch node.Executor.Name {
	case executor.ExecutorNameSSOCheck:
		return v.validateSSOCheckExecutor(node, nodeIndex)
	case executor.ExecutorNameSession:
		return v.validateSessionExecutor(node, nodes)
	}
	return nil
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

	supported := make(map[string]bool, len(meta.SupportedProperties))
	for _, prop := range meta.SupportedProperties {
		supported[prop.Property] = true
		// Check required properties
		if prop.IsRequired {
			val, ok := node.Properties[prop.Property]
			if !ok || val == nil || val == "" {
				return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
					Key: "error.flowmgtservice.missing_required_executor_property_description",
					DefaultValue: "Node '{{param(nodeID)}}': executor '{{param(executorName)}}' " +
						"requires property '{{param(propertyKey)}}'",
					Params: map[string]string{
						"nodeID":       node.ID,
						"executorName": node.Executor.Name,
						"propertyKey":  prop.Property,
					},
				})
			}
		}
	}

	// Check for unsupported properties
	for key := range node.Properties {
		if !supported[key] {
			return tidcommon.CustomServiceError(ErrorInvalidExecutorConfig, tidcommon.I18nMessage{
				Key: "error.flowmgtservice.unsupported_executor_property_description",
				DefaultValue: "Node '{{param(nodeID)}}': executor '{{param(executorName)}}' " +
					"does not support property '{{param(propertyKey)}}'",
				Params: map[string]string{
					"nodeID":       node.ID,
					"executorName": node.Executor.Name,
					"propertyKey":  key,
				},
			})
		}
	}
	return nil
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
		target.Executor == nil || target.Executor.Name != executor.ExecutorNameSession {
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
			n.Executor != nil && n.Executor.Name == executor.ExecutorNameSSOCheck {
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
