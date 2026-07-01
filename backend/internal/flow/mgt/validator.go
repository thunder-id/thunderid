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

package flowmgt

import (
	"context"
	"fmt"
	"regexp"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// handleFormatRegex matches valid handle format:
// - starts with lowercase letter or digit
// - contains only lowercase letters, digits, underscores, or dashes
// - ends with lowercase letter or digit
var handleFormatRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*[a-z0-9]$|^[a-z0-9]$`)

// validateFlowDefinition runs all validation scopes on the flow definition.
// New validation scopes can be added by appending a call here.
func (s *flowMgtService) validateFlowDefinition(
	ctx context.Context, flowDef *FlowDefinition,
) *tidcommon.ServiceError {
	if err := validateMetadata(flowDef); err != nil {
		return err
	}

	nodeIndex, err := validateStructure(flowDef.Nodes)
	if err != nil {
		return err
	}

	if err := s.validateNodes(flowDef.Nodes, nodeIndex); err != nil {
		return err
	}

	if err := s.validateInterceptors(flowDef.Interceptors, nodeIndex); err != nil {
		return err
	}

	if err := s.validateGraph(ctx, flowDef); err != nil {
		return err
	}

	return nil
}

// validateGraph builds the flow graph as a defense-in-depth validation step.
func (s *flowMgtService) validateGraph(
	ctx context.Context, flowDef *FlowDefinition,
) *tidcommon.ServiceError {
	tempFlow := &providers.CompleteFlowDefinition{
		Handle:       flowDef.Handle,
		Name:         flowDef.Name,
		FlowType:     flowDef.FlowType,
		Interceptors: flowDef.Interceptors,
		Nodes:        flowDef.Nodes,
	}
	return s.graphBuilder.ValidateGraph(ctx, tempFlow)
}

// validateFlowDefinitionBasic validates the flow definition without requiring registry access.
// Used by declarative resource validation where registries are not available.
func validateFlowDefinitionBasic(flowDef *FlowDefinition) *tidcommon.ServiceError {
	if err := validateMetadata(flowDef); err != nil {
		return err
	}

	nodeIndex, err := validateStructure(flowDef.Nodes)
	if err != nil {
		return err
	}

	// Validate node and interceptor definitions for correct format, but skip registry checks.
	for i := range flowDef.Nodes {
		if err := validateNodeFormat(&flowDef.Nodes[i], nodeIndex); err != nil {
			return err
		}
	}

	for i, ic := range flowDef.Interceptors {
		if err := validateInterceptorFormat(i, ic); err != nil {
			return err
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Scope: Metadata validation
// ---------------------------------------------------------------------------

// validateMetadata validates flow-level fields: handle, name, type, ID format, and minimum node count.
func validateMetadata(flowDef *FlowDefinition) *tidcommon.ServiceError {
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

// isValidHandleFormat validates that the handle follows the required format.
func isValidHandleFormat(handle string) bool {
	return handleFormatRegex.MatchString(handle)
}

// isValidFlowType checks if the provided flow type is valid.
func isValidFlowType(flowType providers.FlowType) bool {
	return flowType == providers.FlowTypeAuthentication ||
		flowType == providers.FlowTypeRegistration ||
		flowType == providers.FlowTypeUserOnboarding ||
		flowType == providers.FlowTypeRecovery
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
func validateStructure(nodes []providers.NodeDefinition) (
	map[string]*providers.NodeDefinition, *tidcommon.ServiceError,
) {
	nodeIndex, err := buildNodeIndex(nodes)
	if err != nil {
		return nil, err
	}

	if err := validateNodeTypesAndCardinality(nodes); err != nil {
		return nil, err
	}

	refs := collectAllNodeReferences(nodes)
	if err := validateNodeReferences(refs, nodeIndex); err != nil {
		return nil, err
	}

	if err := validateReachability(nodes); err != nil {
		return nil, err
	}

	if err := validateTermination(nodes); err != nil {
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
			return nil, tidcommon.CustomServiceError(ErrorDuplicateNodeID, tidcommon.I18nMessage{
				Key:          "error.flowmgtservice.duplicate_node_id_description",
				DefaultValue: fmt.Sprintf("Duplicate node ID: '%s'", node.ID),
			})
		}
		index[node.ID] = node
	}
	return index, nil
}

// validateNodeTypesAndCardinality checks that every node has a valid type
// and that exactly one START and one END node exist.
func validateNodeTypesAndCardinality(nodes []providers.NodeDefinition) *tidcommon.ServiceError {
	startCount := 0
	endCount := 0
	for _, node := range nodes {
		if !common.ValidNodeTypes[node.Type] {
			return tidcommon.CustomServiceError(ErrorInvalidNodeType, tidcommon.I18nMessage{
				Key:          "error.flowmgtservice.invalid_node_type_description",
				DefaultValue: fmt.Sprintf("Node '%s' has invalid type '%s'", node.ID, node.Type),
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
		return &ErrorMissingStartNode
	}
	if startCount > 1 {
		return &ErrorDuplicateStartNode
	}
	if endCount == 0 {
		return &ErrorMissingEndNode
	}
	if endCount > 1 {
		return &ErrorDuplicateEndNode
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
func validateNodeReferences(
	refs []nodeReference, nodeIndex map[string]*providers.NodeDefinition,
) *tidcommon.ServiceError {
	for _, ref := range refs {
		if _, exists := nodeIndex[ref.targetNodeID]; !exists {
			return tidcommon.CustomServiceError(ErrorInvalidNodeReference, tidcommon.I18nMessage{
				Key: "error.flowmgtservice.invalid_node_reference_description",
				DefaultValue: fmt.Sprintf(
					"Node '%s' references non-existent node '%s' in '%s'",
					ref.sourceNodeID, ref.targetNodeID, ref.fieldName),
			})
		}
	}
	return nil
}

// validateReachability checks that all nodes are reachable from the START node via BFS.
func validateReachability(nodes []providers.NodeDefinition) *tidcommon.ServiceError {
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
			return tidcommon.CustomServiceError(ErrorOrphanedNode, tidcommon.I18nMessage{
				Key:          "error.flowmgtservice.orphaned_node_description",
				DefaultValue: fmt.Sprintf("Node '%s' is not reachable from the START node", node.ID),
			})
		}
	}
	return nil
}

// validateTermination checks that all reachable nodes can reach the END node via reverse BFS.
func validateTermination(nodes []providers.NodeDefinition) *tidcommon.ServiceError {
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
			return tidcommon.CustomServiceError(ErrorNoTermination, tidcommon.I18nMessage{
				Key:          "error.flowmgtservice.no_termination_description",
				DefaultValue: fmt.Sprintf("Node '%s' has no path to the END node", node.ID),
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
func (s *flowMgtService) validateNodes(
	nodes []providers.NodeDefinition, nodeIndex map[string]*providers.NodeDefinition,
) *tidcommon.ServiceError {
	for i := range nodes {
		node := &nodes[i]
		if err := validateNodeFormat(node, nodeIndex); err != nil {
			return err
		}
		if node.Type == string(common.NodeTypeTaskExecution) {
			if err := s.validateExecutors(node); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateNodeFormat validates a single node's format based on its type.
// Does not require registry access.
func validateNodeFormat(
	node *providers.NodeDefinition, nodeIndex map[string]*providers.NodeDefinition,
) *tidcommon.ServiceError {
	switch node.Type {
	case string(common.NodeTypeStart):
		return validateStartNode(node)
	case string(common.NodeTypeEnd):
		return validateEndNode(node)
	case string(common.NodeTypeTaskExecution):
		return validateTaskExecutionNode(node, nodeIndex)
	case string(common.NodeTypePrompt):
		return validatePromptNode(node)
	}
	return nil
}

// validateStartNode validates that a START node has onSuccess and no inapplicable properties.
func validateStartNode(node *providers.NodeDefinition) *tidcommon.ServiceError {
	if node.OnSuccess == "" {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.start_node_missing_on_success_description",
			DefaultValue: fmt.Sprintf("START node '%s' must have onSuccess", node.ID),
		})
	}
	if node.Executor != nil {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.start_node_invalid_property_description",
			DefaultValue: fmt.Sprintf("START node '%s' must not have an executor", node.ID),
		})
	}
	if len(node.Prompts) > 0 {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.start_node_invalid_property_description",
			DefaultValue: fmt.Sprintf("START node '%s' must not have prompts", node.ID),
		})
	}
	if node.OnFailure != "" {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.start_node_invalid_property_description",
			DefaultValue: fmt.Sprintf("START node '%s' must not have onFailure", node.ID),
		})
	}
	if node.OnIncomplete != "" {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.start_node_invalid_property_description",
			DefaultValue: fmt.Sprintf("START node '%s' must not have onIncomplete", node.ID),
		})
	}
	return nil
}

// validateEndNode validates that an END node has no outgoing edges, executor, or prompts.
func validateEndNode(node *providers.NodeDefinition) *tidcommon.ServiceError {
	if node.OnSuccess != "" {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.end_node_invalid_property_description",
			DefaultValue: fmt.Sprintf("END node '%s' must not have onSuccess", node.ID),
		})
	}
	if node.OnFailure != "" {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.end_node_invalid_property_description",
			DefaultValue: fmt.Sprintf("END node '%s' must not have onFailure", node.ID),
		})
	}
	if node.OnIncomplete != "" {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.end_node_invalid_property_description",
			DefaultValue: fmt.Sprintf("END node '%s' must not have onIncomplete", node.ID),
		})
	}
	if node.Executor != nil {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.end_node_invalid_property_description",
			DefaultValue: fmt.Sprintf("END node '%s' must not have an executor", node.ID),
		})
	}
	if len(node.Prompts) > 0 {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.end_node_invalid_property_description",
			DefaultValue: fmt.Sprintf("END node '%s' must not have prompts", node.ID),
		})
	}
	if node.Next != "" {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.end_node_invalid_property_description",
			DefaultValue: fmt.Sprintf("END node '%s' must not have next", node.ID),
		})
	}
	return nil
}

// validateTaskExecutionNode validates the format of a TASK_EXECUTION node.
func validateTaskExecutionNode(
	node *providers.NodeDefinition, nodeIndex map[string]*providers.NodeDefinition,
) *tidcommon.ServiceError {
	if node.Executor == nil || node.Executor.Name == "" {
		return tidcommon.CustomServiceError(ErrorTaskNodeMissingExecutor, tidcommon.I18nMessage{
			Key: "error.flowmgtservice.task_node_missing_executor_description",
			DefaultValue: fmt.Sprintf(
				"TASK_EXECUTION node '%s' must have an executor with a non-empty name", node.ID),
		})
	}
	if node.OnSuccess == "" {
		return tidcommon.CustomServiceError(ErrorTaskNodeMissingOnSuccess, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.task_node_missing_on_success_description",
			DefaultValue: fmt.Sprintf("TASK_EXECUTION node '%s' must have onSuccess", node.ID),
		})
	}
	if node.OnFailure != "" {
		if target, ok := nodeIndex[node.OnFailure]; ok && target.Type != string(common.NodeTypePrompt) {
			return tidcommon.CustomServiceError(ErrorTaskNodeInvalidFailureTarget, tidcommon.I18nMessage{
				Key: "error.flowmgtservice.task_node_invalid_failure_target_description",
				DefaultValue: fmt.Sprintf(
					"TASK_EXECUTION node '%s': onFailure must point to a PROMPT node", node.ID),
			})
		}
	}
	if node.OnIncomplete != "" {
		if target, ok := nodeIndex[node.OnIncomplete]; ok &&
			target.Type != string(common.NodeTypePrompt) {
			return tidcommon.CustomServiceError(
				ErrorTaskNodeInvalidIncompleteTarget, tidcommon.I18nMessage{
					Key: "error.flowmgtservice.task_node_invalid_incomplete_target_description",
					DefaultValue: fmt.Sprintf(
						"TASK_EXECUTION node '%s': onIncomplete must point to a PROMPT node", node.ID),
				})
		}
	}
	return nil
}

// validatePromptNode validates a PROMPT node by dispatching to the appropriate sub-validator.
func validatePromptNode(node *providers.NodeDefinition) *tidcommon.ServiceError {
	hasPrompts := len(node.Prompts) > 0
	hasNext := node.Next != ""

	if hasPrompts && hasNext {
		return tidcommon.CustomServiceError(ErrorPromptNodeInvalidConfig, tidcommon.I18nMessage{
			Key: "error.flowmgtservice.prompt_node_invalid_config_description",
			DefaultValue: fmt.Sprintf(
				"PROMPT node '%s' must have either prompts or next, not both", node.ID),
		})
	}
	if !hasPrompts && !hasNext {
		return tidcommon.CustomServiceError(ErrorPromptNodeInvalidConfig, tidcommon.I18nMessage{
			Key: "error.flowmgtservice.prompt_node_invalid_config_description",
			DefaultValue: fmt.Sprintf(
				"PROMPT node '%s' must have either prompts or next", node.ID),
		})
	}

	if hasNext {
		return validateDisplayOnlyPromptNode(node)
	}
	return validateInteractivePromptNode(node)
}

// validateDisplayOnlyPromptNode validates a display-only PROMPT node (has next, no prompts).
func validateDisplayOnlyPromptNode(_ *providers.NodeDefinition) *tidcommon.ServiceError {
	return nil
}

// validateInteractivePromptNode validates an interactive PROMPT node (has prompts with actions).
func validateInteractivePromptNode(node *providers.NodeDefinition) *tidcommon.ServiceError {
	for i, prompt := range node.Prompts {
		if prompt.Action == nil || prompt.Action.NextNode == "" {
			return tidcommon.CustomServiceError(ErrorPromptMissingAction, tidcommon.I18nMessage{
				Key: "error.flowmgtservice.prompt_missing_action_description",
				DefaultValue: fmt.Sprintf(
					"PROMPT node '%s': prompt at index %d must have an action with nextNode", node.ID, i),
			})
		}
		if err := validateInputDefinitions(node.ID, prompt.Inputs); err != nil {
			return err
		}
	}
	return nil
}

// validInputTypes is the set of valid input type strings.
var validInputTypes = map[string]bool{
	providers.InputTypeText:     true,
	providers.InputTypeEmail:    true,
	providers.InputTypePassword: true,
	providers.InputTypeOTP:      true,
	providers.InputTypePhone:    true,
	providers.InputTypeConsent:  true,
	providers.InputTypeHidden:   true,
	providers.InputTypeSelect:   true,
	providers.InputTypeOUSelect: true,
	providers.InputTypeNumber:   true,
	providers.InputTypeDate:     true,
}

// validateInputDefinitions validates input definitions for valid types and validation rules.
func validateInputDefinitions(nodeID string, inputs []providers.InputDefinition) *tidcommon.ServiceError {
	for _, input := range inputs {
		if input.Type != "" && !validInputTypes[input.Type] {
			return tidcommon.CustomServiceError(ErrorInvalidInputType, tidcommon.I18nMessage{
				Key: "error.flowmgtservice.invalid_input_type_description",
				DefaultValue: fmt.Sprintf(
					"Node '%s': input '%s' has invalid type '%s'", nodeID, input.Identifier, input.Type),
			})
		}
		for _, rule := range input.Validation {
			if !providers.ValidValidationRuleTypes[rule.Type] {
				return tidcommon.CustomServiceError(ErrorInvalidValidationRule, tidcommon.I18nMessage{
					Key: "error.flowmgtservice.invalid_validation_rule_description",
					DefaultValue: fmt.Sprintf(
						"Node '%s': input '%s' has invalid validation rule type '%s'",
						nodeID, input.Identifier, rule.Type),
				})
			}
			if rule.Type == "regex" {
				pattern, ok := rule.Value.(string)
				if !ok {
					return tidcommon.CustomServiceError(ErrorInvalidValidationRule, tidcommon.I18nMessage{
						Key: "error.flowmgtservice.invalid_validation_rule_description",
						DefaultValue: fmt.Sprintf(
							"Node '%s': input '%s' regex validation rule value must be a string",
							nodeID, input.Identifier),
					})
				}
				if _, err := regexp.Compile(pattern); err != nil {
					return tidcommon.CustomServiceError(ErrorInvalidValidationRule, tidcommon.I18nMessage{
						Key: "error.flowmgtservice.invalid_validation_rule_description",
						DefaultValue: fmt.Sprintf(
							"Node '%s': input '%s' has invalid regex pattern: %s",
							nodeID, input.Identifier, err.Error()),
					})
				}
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Scope: Executor validation
// ---------------------------------------------------------------------------

// validateExecutors validates that the executor referenced in a task execution node
// is registered in the executor registry.
func (s *flowMgtService) validateExecutors(node *providers.NodeDefinition) *tidcommon.ServiceError {
	if !s.executorRegistry.IsRegistered(node.Executor.Name) {
		return tidcommon.CustomServiceError(ErrorExecutorNotRegistered, tidcommon.I18nMessage{
			Key: "error.flowmgtservice.executor_not_registered_description",
			DefaultValue: fmt.Sprintf(
				"Node '%s': executor '%s' is not registered", node.ID, node.Executor.Name),
		})
	}
	return nil
}

// ---------------------------------------------------------------------------
// Scope: Interceptor validation
// ---------------------------------------------------------------------------

// validateInterceptors validates interceptor definitions including format, registry
// registration, and applyTo node references.
func (s *flowMgtService) validateInterceptors(
	interceptors []providers.InterceptorDefinition, nodeIndex map[string]*providers.NodeDefinition,
) *tidcommon.ServiceError {
	if err := s.validateInterceptorDefinitions(interceptors); err != nil {
		return err
	}

	if err := validateInterceptorApplyTo(interceptors, nodeIndex); err != nil {
		return err
	}

	return nil
}

// validateInterceptorDefinitions validates the format of each interceptor definition
// and checks that it is registered in the interceptor registry.
func (s *flowMgtService) validateInterceptorDefinitions(
	interceptors []providers.InterceptorDefinition,
) *tidcommon.ServiceError {
	for i, ic := range interceptors {
		if err := validateInterceptorFormat(i, ic); err != nil {
			return err
		}
		if !s.interceptorRegistry.IsRegistered(ic.Name) {
			return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
				Key: "error.flowmgtservice.interceptor_not_registered",
				DefaultValue: fmt.Sprintf(
					"Interceptor '%s' is not registered", ic.Name),
			})
		}
	}
	return nil
}

// validateInterceptorFormat validates the format of a single interceptor definition
// (name, mode, scope, applyTo constraints) without requiring registry access.
func validateInterceptorFormat(index int, ic providers.InterceptorDefinition) *tidcommon.ServiceError {
	if ic.Name == "" {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key:          "error.flowmgtservice.interceptor_name_required",
			DefaultValue: fmt.Sprintf("Interceptor at index %d must have a name", index),
		})
	}
	if isDefaultInterceptor(ic.Name) {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key: "error.flowmgtservice.interceptor_default_not_configurable",
			DefaultValue: fmt.Sprintf(
				"Default interceptor '%s' cannot be configured in a flow definition", ic.Name),
		})
	}
	if !providers.ValidInterceptorModes[ic.Mode] {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key: "error.flowmgtservice.interceptor_invalid_mode",
			DefaultValue: fmt.Sprintf(
				"Interceptor '%s' has invalid mode '%s'", ic.Name, ic.Mode),
		})
	}
	if ic.Scope != "" && !providers.ValidInterceptorScopes[ic.Scope] {
		return tidcommon.CustomServiceError(ErrorInvalidFlowData, tidcommon.I18nMessage{
			Key: "error.flowmgtservice.interceptor_invalid_scope",
			DefaultValue: fmt.Sprintf(
				"Interceptor '%s' has invalid scope '%s'", ic.Name, ic.Scope),
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
func validateInterceptorApplyTo(
	interceptors []providers.InterceptorDefinition, nodeIndex map[string]*providers.NodeDefinition,
) *tidcommon.ServiceError {
	for _, ic := range interceptors {
		if ic.Scope != providers.InterceptorScopeSelected {
			continue
		}
		for _, nodeID := range ic.ApplyTo {
			if _, exists := nodeIndex[nodeID]; !exists {
				return tidcommon.CustomServiceError(
					ErrorInterceptorInvalidApplyTo, tidcommon.I18nMessage{
						Key: "error.flowmgtservice.interceptor_invalid_apply_to_description",
						DefaultValue: fmt.Sprintf(
							"Interceptor '%s': applyTo references non-existent node '%s'",
							ic.Name, nodeID),
					})
			}
		}
	}
	return nil
}
