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

package flowmgt

import (
	"context"
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// graphBuilderInterface defines the interface for building flow graphs.
type graphBuilderInterface interface {
	GetGraph(ctx context.Context, flow *CompleteFlowDefinition) (core.GraphInterface, *serviceerror.ServiceError)
	InvalidateCache(ctx context.Context, flowID string)
}

// graphBuilder is the implementation of graphBuilderInterface.
type graphBuilder struct {
	flowFactory      core.FlowFactoryInterface
	executorRegistry executor.ExecutorRegistryInterface
	graphCache       core.GraphCacheInterface
	logger           *log.Logger
}

// newGraphBuilder creates a new instance of graphBuilder.
func newGraphBuilder(
	flowFactory core.FlowFactoryInterface,
	executorRegistry executor.ExecutorRegistryInterface,
	graphCache core.GraphCacheInterface,
) graphBuilderInterface {
	return &graphBuilder{
		flowFactory:      flowFactory,
		executorRegistry: executorRegistry,
		graphCache:       graphCache,
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowGraphBuilder")),
	}
}

// GetGraph retrieves a cached graph or builds a new one from the flow definition.
func (b *graphBuilder) GetGraph(ctx context.Context, flow *CompleteFlowDefinition) (
	core.GraphInterface, *serviceerror.ServiceError) {
	if flow == nil || len(flow.Nodes) == 0 {
		return nil, serviceerror.CustomServiceError(ErrorInvalidFlowData, i18ncore.I18nMessage{
			Key:          "error.flowmgtservice.flow_definition_nil_or_empty_description",
			DefaultValue: "Flow definition is nil or has no nodes",
		})
	}

	logger := b.logger.With(log.String("flowID", flow.ID))
	// Check cache first
	if cachedGraph, ok := b.graphCache.Get(ctx, flow.ID); ok {
		logger.Debug("Graph retrieved from cache")
		return cachedGraph, nil
	}

	graph, err := b.buildGraph(flow)
	if err != nil {
		logger.Error("Failed to build graph", log.Error(err))
		return nil, serviceerror.CustomServiceError(ErrorGraphBuildFailure, i18ncore.I18nMessage{
			Key:          "error.flowmgtservice.graph_build_failure_description",
			DefaultValue: err.Error(),
		})
	}

	// Cache the built graph
	if cacheErr := b.graphCache.Set(ctx, flow.ID, graph); cacheErr != nil {
		logger.Error("Failed to cache graph", log.Error(cacheErr))
	}
	logger.Debug("Graph built and cached successfully")

	return graph, nil
}

// InvalidateCache invalidates the cached graph for the given flow ID.
func (b *graphBuilder) InvalidateCache(ctx context.Context, flowID string) {
	if flowID == "" {
		return
	}

	if err := b.graphCache.Invalidate(ctx, flowID); err != nil {
		b.logger.Error("Failed to delete graph from cache", log.String("flowID", flowID), log.Error(err))
	}
	b.logger.Debug("Graph cache invalidated", log.String("flowID", flowID))
}

// buildGraph converts a CompleteFlowDefinition to a core.GraphInterface for execution.
func (b *graphBuilder) buildGraph(flow *CompleteFlowDefinition) (core.GraphInterface, error) {
	if flow == nil || len(flow.Nodes) == 0 {
		return nil, fmt.Errorf("flow definition is nil or has no nodes")
	}

	// Create a graph
	graph := b.flowFactory.CreateGraph(flow.ID, flow.FlowType)

	// Process all nodes and build the graph structure
	edges := make(map[string][]string)
	boundaries := make([]segmentBoundary, 0)
	for i := range flow.Nodes {
		if err := b.processNode(&flow.Nodes[i], flow.Nodes, graph, edges, &boundaries); err != nil {
			return nil, fmt.Errorf("failed to process node %s: %w", flow.Nodes[i].ID, err)
		}
	}

	if err := b.addGraphEdges(graph, edges); err != nil {
		return nil, err
	}

	if err := b.determineAndSetStartNode(graph); err != nil {
		return nil, err
	}

	b.computeSegments(graph, boundaries)

	return graph, nil
}

// processNode processes a single node definition and adds it to the graph.
func (b *graphBuilder) processNode(nodeDef *NodeDefinition, allNodes []NodeDefinition,
	graph core.GraphInterface, edges map[string][]string, boundaries *[]segmentBoundary) error {
	isFinalNode := nodeDef.OnSuccess == "" &&
		nodeDef.OnFailure == "" &&
		len(nodeDef.Prompts) == 0 &&
		nodeDef.Next == ""

	// Construct a new node. Here we set isStartNode to false by default
	node, err := b.flowFactory.CreateNode(nodeDef.ID, nodeDef.Type, nodeDef.Properties,
		false, isFinalNode)
	if err != nil {
		return fmt.Errorf("failed to create node %s: %w", nodeDef.ID, err)
	}

	if err := b.configureNodeNavigation(nodeDef, allNodes, node, edges); err != nil {
		return err
	}

	b.configureNodeInputs(nodeDef, node)
	b.configureNodeMeta(nodeDef, node)
	b.configureNodeVariant(nodeDef, node)
	b.configureNodeCondition(nodeDef, node)

	if err := b.configureNodePrompts(nodeDef, node, edges); err != nil {
		return err
	}
	if err := b.configureDisplayOnlyProperties(nodeDef, node, edges, boundaries); err != nil {
		return err
	}
	if err := b.configureNodeExecutor(nodeDef, node); err != nil {
		return err
	}

	// Add node to the graph
	if err := graph.AddNode(node); err != nil {
		return fmt.Errorf("failed to add node %s to the graph: %w", nodeDef.ID, err)
	}

	return nil
}

// configureNodeNavigation configures the onSuccess and onFailure properties for a node.
func (b *graphBuilder) configureNodeNavigation(nodeDef *NodeDefinition, allNodes []NodeDefinition,
	node core.NodeInterface, edges map[string][]string) error {
	// Set onSuccess if defined
	if nodeDef.OnSuccess != "" {
		if nodeWithOnSuccess, ok := node.(interface{ SetOnSuccess(string) }); ok {
			nodeWithOnSuccess.SetOnSuccess(nodeDef.OnSuccess)
		}

		// Add edge for graph structure
		if _, exists := edges[nodeDef.ID]; !exists {
			edges[nodeDef.ID] = []string{}
		}
		edges[nodeDef.ID] = append(edges[nodeDef.ID], nodeDef.OnSuccess)
	}

	// Set onFailure if defined
	if nodeDef.OnFailure != "" {
		if err := b.validateOnFailureTarget(allNodes, nodeDef.OnFailure); err != nil {
			return fmt.Errorf("invalid onFailure configuration for node %s: %w", nodeDef.ID, err)
		}
		if taskNode, ok := node.(core.ExecutorBackedNodeInterface); ok {
			taskNode.SetOnFailure(nodeDef.OnFailure)
		}

		// Add edge for graph structure
		if _, exists := edges[nodeDef.ID]; !exists {
			edges[nodeDef.ID] = []string{}
		}
		edges[nodeDef.ID] = append(edges[nodeDef.ID], nodeDef.OnFailure)
	}

	// Set onIncomplete if defined
	if nodeDef.OnIncomplete != "" {
		if err := b.validateOnIncompleteTarget(allNodes, nodeDef.OnIncomplete); err != nil {
			return fmt.Errorf("invalid onIncomplete configuration for node %s: %w", nodeDef.ID, err)
		}
		if taskNode, ok := node.(core.ExecutorBackedNodeInterface); ok {
			taskNode.SetOnIncomplete(nodeDef.OnIncomplete)
		}

		// Add edge for graph structure
		if _, exists := edges[nodeDef.ID]; !exists {
			edges[nodeDef.ID] = []string{}
		}
		edges[nodeDef.ID] = append(edges[nodeDef.ID], nodeDef.OnIncomplete)
	}

	return nil
}

// validateOnFailureTarget validates that the onFailure target node is a PROMPT node.
func (b *graphBuilder) validateOnFailureTarget(nodes []NodeDefinition, targetNodeID string) error {
	for _, node := range nodes {
		if node.ID == targetNodeID {
			if node.Type != "PROMPT" {
				return errors.New("onFailure must point to a PROMPT node")
			}
			return nil
		}
	}
	return errors.New("onFailure target node not found")
}

// validateOnIncompleteTarget validates that the onIncomplete target node is a PROMPT node.
func (b *graphBuilder) validateOnIncompleteTarget(nodes []NodeDefinition, targetNodeID string) error {
	for _, node := range nodes {
		if node.ID == targetNodeID {
			if node.Type != "PROMPT" {
				return errors.New("onIncomplete must point to a PROMPT node")
			}
			return nil
		}
	}
	return errors.New("onIncomplete target node not found")
}

// configureNodeInputs configures the inputs for executor-backed nodes.
func (b *graphBuilder) configureNodeInputs(nodeDef *NodeDefinition, node core.NodeInterface) {
	logger := b.logger.With(log.String("nodeID", nodeDef.ID))

	executorNode, ok := node.(core.ExecutorBackedNodeInterface)
	if !ok {
		logger.Debug("Node is not executor-backed; skipping input configuration")
		return
	}

	// Get inputs from executor definition if available
	if nodeDef.Executor == nil || len(nodeDef.Executor.Inputs) == 0 {
		logger.Debug("No inputs defined for executor; setting empty input list")
		executorNode.SetInputs([]common.Input{})
		return
	}

	inputs := make([]common.Input, len(nodeDef.Executor.Inputs))
	for i, input := range nodeDef.Executor.Inputs {
		inputs[i] = common.Input{
			Ref:        input.Ref,
			Identifier: input.Identifier,
			Type:       input.Type,
			Required:   input.Required,
		}
	}
	executorNode.SetInputs(inputs)
}

// configureNodeVariant sets the prompt node's variant from the node definition.
func (b *graphBuilder) configureNodeVariant(nodeDef *NodeDefinition, node core.NodeInterface) {
	if nodeDef.Variant == "" {
		return
	}
	if promptNode, ok := node.(core.PromptNodeInterface); ok {
		promptNode.SetVariant(nodeDef.Variant)
	}
}

// configureNodeMeta configures the meta object for a prompt node.
func (b *graphBuilder) configureNodeMeta(nodeDef *NodeDefinition, node core.NodeInterface) {
	if nodeDef.Meta == nil {
		return
	}

	// Set meta only if the node is a prompt node
	if promptNode, ok := node.(core.PromptNodeInterface); ok {
		promptNode.SetMeta(nodeDef.Meta)
	}
}

// configureNodeCondition configures the condition for a node.
func (b *graphBuilder) configureNodeCondition(nodeDef *NodeDefinition, node core.NodeInterface) {
	if nodeDef.Condition != nil && (nodeDef.Condition.Key != "" || nodeDef.Condition.Value != "") {
		node.SetCondition(&core.NodeCondition{
			Key:    nodeDef.Condition.Key,
			Value:  nodeDef.Condition.Value,
			OnSkip: nodeDef.Condition.OnSkip,
		})
	}
}

// configureNodePrompts configures the prompts for a prompt node.
func (b *graphBuilder) configureNodePrompts(nodeDef *NodeDefinition, node core.NodeInterface,
	edges map[string][]string) error {
	logger := b.logger.With(log.String("nodeID", nodeDef.ID))

	if len(nodeDef.Prompts) == 0 {
		logger.Debug("No prompts to configure for this node")
		return nil
	}

	promptNode, ok := node.(core.PromptNodeInterface)
	if !ok {
		logger.Debug("Node is not a prompt node; skipping prompt configuration")
		return nil
	}

	prompts := make([]common.Prompt, len(nodeDef.Prompts))
	for i, promptDef := range nodeDef.Prompts {
		// Convert inputs
		inputs := make([]common.Input, len(promptDef.Inputs))
		for j, inputDef := range promptDef.Inputs {
			inputs[j] = common.Input{
				Ref:        inputDef.Ref,
				Identifier: inputDef.Identifier,
				Type:       inputDef.Type,
				Required:   inputDef.Required,
			}
		}
		prompts[i].Inputs = inputs

		// Convert action if present
		if promptDef.Action != nil {
			prompts[i].Action = &common.Action{
				Ref:      promptDef.Action.Ref,
				Type:     promptDef.Action.Type,
				NextNode: promptDef.Action.NextNode,
			}

			// Add edge for the action's next node
			if _, exists := edges[nodeDef.ID]; !exists {
				edges[nodeDef.ID] = []string{}
			}
			edges[nodeDef.ID] = append(edges[nodeDef.ID], promptDef.Action.NextNode)
		}
	}

	promptNode.SetPrompts(prompts)

	return nil
}

// configureDisplayOnlyProperties configures the 'next' and 'message' fields for display-only prompt nodes.
// It also records segment boundaries for later segment computation.
func (b *graphBuilder) configureDisplayOnlyProperties(nodeDef *NodeDefinition, node core.NodeInterface,
	edges map[string][]string, boundaries *[]segmentBoundary) error {
	logger := b.logger.With(log.String("nodeID", nodeDef.ID))

	if nodeDef.Next == "" {
		return nil
	}

	promptNode, ok := node.(core.PromptNodeInterface)
	if !ok {
		return fmt.Errorf("'next' field is only valid on PROMPT nodes, but node %s is of type %s",
			nodeDef.ID, nodeDef.Type)
	}

	if len(nodeDef.Prompts) > 0 {
		return fmt.Errorf("node %s has both 'prompts' and 'next'; these are mutually exclusive",
			nodeDef.ID)
	}

	logger.Debug("Configuring display-only next for prompt node", log.String("next", nodeDef.Next))
	promptNode.SetNextNode(nodeDef.Next)

	if nodeDef.Message != "" {
		promptNode.SetMessage(nodeDef.Message)
	}

	if _, exists := edges[nodeDef.ID]; !exists {
		edges[nodeDef.ID] = []string{}
	}
	edges[nodeDef.ID] = append(edges[nodeDef.ID], nodeDef.Next)

	if boundaries != nil {
		*boundaries = append(*boundaries, segmentBoundary{
			boundaryNodeID: nodeDef.ID,
			nextNodeID:     nodeDef.Next,
		})
	}

	return nil
}

// computeSegments builds the segments slice from detected display-only prompt boundaries.
// Segment 0 starts at the graph start node; each boundary yields a subsequent segment
// starting at the boundary's next node.
func (b *graphBuilder) computeSegments(g core.GraphInterface, boundaries []segmentBoundary) {
	if len(boundaries) == 0 {
		return
	}

	startNode, err := g.GetStartNode()
	if err != nil {
		return
	}

	segments := make([]core.Segment, 0, len(boundaries)+1)
	segments = append(segments, core.Segment{
		ID:          "seg-0",
		StartNodeID: startNode.GetID(),
	})
	for i, bnd := range boundaries {
		segments = append(segments, core.Segment{
			ID:          fmt.Sprintf("seg-%d", i+1),
			StartNodeID: bnd.nextNodeID,
		})
	}
	g.SetSegments(segments)
}

// configureNodeExecutor configures the executor for a node.
func (b *graphBuilder) configureNodeExecutor(nodeDef *NodeDefinition, node core.NodeInterface) error {
	logger := b.logger.With(log.String("nodeID", nodeDef.ID))

	if nodeDef.Executor == nil {
		logger.Debug("No executor to configure for this node")
		return nil
	}

	executableNode, ok := node.(core.ExecutorBackedNodeInterface)
	if !ok {
		logger.Debug("Node does not support executors; skipping executor configuration")
		return nil
	}

	executorName := nodeDef.Executor.Name
	if executorName != "" {
		if err := b.validateExecutorName(executorName); err != nil {
			return fmt.Errorf("error while validating executor %s: %w", executorName, err)
		}

		executableNode.SetExecutorName(executorName)

		// Set executor mode if specified
		if nodeDef.Executor.Mode != "" {
			executableNode.SetMode(nodeDef.Executor.Mode)
		}
	}

	return nil
}

// validateExecutorName validates that an executor with the given name is registered.
func (b *graphBuilder) validateExecutorName(executorName string) error {
	if executorName == "" {
		return fmt.Errorf("executor name cannot be empty")
	}
	if !b.executorRegistry.IsRegistered(executorName) {
		return fmt.Errorf("executor with name %s not registered", executorName)
	}

	return nil
}

// addGraphEdges adds all collected edges to the graph.
func (b *graphBuilder) addGraphEdges(graph core.GraphInterface, edges map[string][]string) error {
	for sourceID, targetIDs := range edges {
		for _, targetID := range targetIDs {
			if err := graph.AddEdge(sourceID, targetID); err != nil {
				return fmt.Errorf("failed to add edge from %s to %s: %w", sourceID, targetID, err)
			}
		}
	}
	return nil
}

// determineAndSetStartNode determines the start node and sets it in the graph.
func (b *graphBuilder) determineAndSetStartNode(graph core.GraphInterface) error {
	for _, node := range graph.GetNodes() {
		if node.GetType() == common.NodeTypeStart {
			return graph.SetStartNode(node.GetID())
		}
	}
	return fmt.Errorf("no start node found in the graph definition")
}
