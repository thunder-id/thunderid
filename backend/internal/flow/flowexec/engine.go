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

package flowexec

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"time"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/flow/graphbuilder"
	"github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// maxCallDepth is the maximum number of nested call frames allowed
const maxCallDepth = 10

// flowEngineInterface defines the interface for the flow engine.
type flowEngineInterface interface {
	Execute(ctx *EngineContext) (FlowStep, *tidcommon.ServiceError)
}

// FlowEngine is the main engine implementation for orchestrating flow executions.
type flowEngine struct {
	executorRegistry  executor.ExecutorRegistryInterface
	interceptorRunner InterceptorRunnerInterface
	observabilitySvc  providers.ObservabilityProvider
	flowProvider      providers.FlowProvider
	graphBuilder      graphbuilder.GraphBuilderInterface
	logger            *log.Logger
}

// newFlowEngine creates a new flow engine with the given dependencies.
func newFlowEngine(
	executorRegistry executor.ExecutorRegistryInterface,
	interceptorRunner InterceptorRunnerInterface,
	observabilitySvc providers.ObservabilityProvider,
	flowProvider providers.FlowProvider,
	graphBuilder graphbuilder.GraphBuilderInterface,
) flowEngineInterface {
	return &flowEngine{
		executorRegistry:  executorRegistry,
		interceptorRunner: interceptorRunner,
		observabilitySvc:  observabilitySvc,
		flowProvider:      flowProvider,
		graphBuilder:      graphBuilder,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}
}

// Execute executes a step in the flow
func (fe *flowEngine) Execute(ctx *EngineContext) (FlowStep, *tidcommon.ServiceError) {
	logger := fe.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	flowStep := FlowStep{
		ExecutionID: ctx.ExecutionID,
	}

	// Track flow execution start time
	flowStartTime := time.Now().UnixMilli()

	// Publish flow started event (only if this is the first execution - check if ExecutionHistory is empty)
	if len(ctx.ExecutionHistory) == 0 {
		publishFlowStartedEvent(ctx, fe.observabilitySvc)
	}

	if err := fe.setCurrentExecutionNode(ctx, logger); err != nil {
		// Publish flow failed event before returning error
		publishFlowFailedEvent(ctx, err, flowStartTime, time.Now().UnixMilli(), fe.observabilitySvc)
		return flowStep, err
	}

	currentNode := ctx.CurrentNode

	// Run PRE_REQUEST interceptors once before any node executes.
	isSuccess, svcErr := fe.runInterceptors(
		providers.InterceptorModePreRequest, ctx, nil, &flowStep)
	if svcErr != nil {
		publishFlowFailedEvent(ctx, svcErr, flowStartTime, time.Now().UnixMilli(), fe.observabilitySvc)
		return flowStep, svcErr
	}
	if !isSuccess {
		// Check if flow failed or just incomplete
		if flowStep.Status == providers.FlowStatusError {
			publishFlowFailedEvent(ctx, nil, flowStartTime, time.Now().UnixMilli(), fe.observabilitySvc)
		}
		if _, svcErr := fe.runPostRequestInterceptorsOnExit(ctx, &flowStep, flowStartTime); svcErr != nil {
			// Ignore POST_REQUEST interceptor failures or continuation conditions,
			// since this is already a flow exit scenario.
			return flowStep, svcErr
		}
		return flowStep, nil
	}

	// Execute the graph nodes until a terminal condition is met or currentNode is nil
	for currentNode != nil {
		nodeForCleanup := currentNode
		nextNode, exitFlow, svcErr := fe.executeNodePackage(ctx, currentNode, &flowStep, flowStartTime)

		fe.clearNodePackageOneTimeUseInputs(ctx, nodeForCleanup)
		if svcErr != nil {
			return flowStep, svcErr
		}

		if exitFlow {
			return flowStep, nil
		}
		currentNode = nextNode
	}

	// If we reach here, it means the all flow nodes has been executed successfully
	flowStep.Status = providers.FlowStatusComplete
	if ctx.Assertion != "" {
		flowStep.Assertion = ctx.Assertion
	}
	if len(ctx.AdditionalData) > 0 {
		flowStep.Data.AdditionalData = ctx.AdditionalData
	}

	// Run POST_REQUEST interceptors after all nodes have been processed
	isSuccess, svcErr = fe.runPostRequestInterceptorsOnExit(ctx, &flowStep, flowStartTime)
	if svcErr != nil {
		return flowStep, svcErr
	}
	if !isSuccess {
		return flowStep, nil
	}

	// Publish flow completed event
	flowEndTime := time.Now().UnixMilli()
	publishFlowCompletedEvent(ctx, flowStartTime, flowEndTime, fe.observabilitySvc)

	return flowStep, nil
}

// executeNodePackage runs the PRE_NODE interceptors, the node, and the POST_NODE
// interceptors as a single package. It returns the next node to execute, whether the outer
// flow loop should exit (in which case the caller should return without treating it as an
// error), and any service error.
func (fe *flowEngine) executeNodePackage(ctx *EngineContext,
	currentNode core.NodeInterface, flowStep *FlowStep, flowStartTime int64) (
	core.NodeInterface, bool, *tidcommon.ServiceError) {
	logger := fe.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID),
		log.String("nodeID", currentNode.GetID()), log.String("nodeType", string(currentNode.GetType())))

	logger.Debug(ctx.Context, "Executing node")

	// SSO inputs ride on the context (transient, never persisted, and off the engine contract);
	// only the SSO-Check and Session nodes read them.
	ssoCtx := session.WithSSOInputs(ctx.Context, session.SSOInputs{
		Handle:      ctx.SSOHandleIn,
		FlowID:      ssoFlowID(ctx),
		FlowVersion: ctx.SSOFlowVersion,
	})
	nodeCtx := &providers.NodeContext{
		Context:          ssoCtx,
		ExecutionID:      ctx.ExecutionID,
		FlowType:         ctx.FlowType,
		EntityID:         ctx.AppID,
		CurrentAction:    ctx.CurrentAction,
		Verbose:          ctx.Verbose,
		NodeInputs:       getNodeInputs(ctx.CurrentNode),
		UserInputs:       ctx.UserInputs,
		CurrentNodeID:    ctx.CurrentNode.GetID(),
		RuntimeData:      ctx.RuntimeData,
		ForwardedData:    ctx.ForwardedData,
		Application:      ctx.Application,
		AuthUser:         ctx.AuthUser,
		ExecutionHistory: ctx.ExecutionHistory,
	}
	if nodeCtx.NodeInputs == nil {
		nodeCtx.NodeInputs = make([]providers.Input, 0)
	}
	if nodeCtx.UserInputs == nil {
		nodeCtx.UserInputs = make(map[string]string)
	}
	if nodeCtx.RuntimeData == nil {
		nodeCtx.RuntimeData = make(map[string]string)
	}
	if nodeCtx.ForwardedData == nil {
		nodeCtx.ForwardedData = make(map[string]interface{})
	}

	// Clear ForwardedData from engine context after passing to node context
	// This ensures ForwardedData is only available to the immediate next node
	ctx.ForwardedData = nil

	// Check if the node should be executed based on its condition
	if !currentNode.ShouldExecute(nodeCtx) {
		logger.Debug(ctx.Context, "Skipping node due to unmet condition",
			log.String("nodeID", currentNode.GetID()))
		nextNode, svcErr := fe.skipToNextNode(ctx, currentNode, logger)
		if svcErr != nil {
			return nil, false, svcErr
		}
		return nextNode, false, nil
	}

	if svcErr := fe.setNodeExecutor(ctx.Context, currentNode, logger); svcErr != nil {
		return nil, false, svcErr
	}

	// Run PRE_NODE interceptors before the node executes
	isSuccess, svcErr := fe.runInterceptors(providers.InterceptorModePreNode, ctx, currentNode, flowStep)
	if svcErr != nil {
		publishFlowFailedEvent(ctx, svcErr, flowStartTime, time.Now().UnixMilli(), fe.observabilitySvc)
		return nil, false, svcErr
	}
	if !isSuccess {
		// Drain node-scope consumed inputs before the request-scope cleanup wipes the list
		fe.clearNodePackageOneTimeUseInputs(ctx, currentNode)
		if _, svcErr := fe.runPostRequestInterceptorsOnExit(ctx, flowStep, flowStartTime); svcErr != nil {
			// Ignore POST_REQUEST interceptor failures or continuation conditions,
			// since this is already a flow exit scenario.
			return nil, false, svcErr
		}
		return nil, true, nil
	}

	executionStartTime := time.Now().UnixMilli()

	// Publish node execution started event
	publishNodeExecutionStartedEvent(ctx, currentNode, fe.observabilitySvc)

	nodeResp, nodeErr := currentNode.Execute(nodeCtx)
	executionEndTime := time.Now().UnixMilli()

	if consumed := nodeCtx.GetConsumedInputs(); len(consumed) > 0 {
		ctx.consumedInputs = append(ctx.consumedInputs, consumed...)
	}

	// Clear sensitive inputs from context after executor has consumed them
	fe.clearSensitiveInputs(ctx, currentNode)

	recordNodeExecution(ctx, currentNode, nodeResp, nodeErr, executionStartTime, executionEndTime)

	// Publish node execution completed or failed event
	publishNodeExecutionCompletedEvent(
		ctx, currentNode, nodeResp, nodeErr,
		executionStartTime, executionEndTime, fe.observabilitySvc,
	)

	if nodeErr != nil {
		// Publish flow failed event before returning error
		publishFlowFailedEvent(ctx, nodeErr, flowStartTime, time.Now().UnixMilli(), fe.observabilitySvc)
		return nil, false, nodeErr
	}

	fe.trackPresentedOptionalInputs(ctx, nodeResp)
	fe.updateContextWithNodeResponse(ctx, nodeResp)

	nextNode, continueExecution, svcErr := fe.processNodeResponse(ctx, nodeResp, flowStep, logger)
	if svcErr != nil {
		// Publish flow failed event before returning error
		publishFlowFailedEvent(ctx, svcErr, flowStartTime, time.Now().UnixMilli(), fe.observabilitySvc)
		return nil, false, svcErr
	}
	if !continueExecution {
		// Drain node-scope consumed inputs before the request-scope cleanup wipes the list
		fe.clearNodePackageOneTimeUseInputs(ctx, currentNode)
		// Run POST_REQUEST interceptors for incomplete flows to allow cleanup
		if _, svcErr := fe.runPostRequestInterceptorsOnExit(ctx, flowStep, flowStartTime); svcErr != nil {
			// Ignore POST_REQUEST interceptor failures or continuation conditions,
			// since this is already a flow exit scenario
			return nil, false, svcErr
		}
		return nil, true, nil
	}

	// Run POST_NODE interceptors after the node executes
	isSuccess, svcErr = fe.runInterceptors(providers.InterceptorModePostNode, ctx, currentNode, flowStep)
	if svcErr != nil {
		publishFlowFailedEvent(ctx, svcErr, flowStartTime, time.Now().UnixMilli(), fe.observabilitySvc)
		return nil, false, svcErr
	}
	if !isSuccess {
		// Drain node-scope consumed inputs before the request-scope cleanup wipes the list
		fe.clearNodePackageOneTimeUseInputs(ctx, currentNode)
		if _, svcErr := fe.runPostRequestInterceptorsOnExit(ctx, flowStep, flowStartTime); svcErr != nil {
			// Ignore POST_REQUEST interceptor failures or continuation conditions,
			// since this is already a flow exit scenario
			return nil, false, svcErr
		}
		return nil, true, nil
	}

	return nextNode, false, nil
}

// runInterceptors builds an InterceptorRunnerContext from the engine context, delegates to the
// interceptor runner, updates the engine context with the response, and processes the response
// to populate the FlowStep if needed.
// Returns (true, nil) if execution should continue, (false, nil) if the flow should stop
// (INCOMPLETE), or (false, svcErr) on failure.
func (fe *flowEngine) runInterceptors(
	mode providers.InterceptorMode, ctx *EngineContext, node core.NodeInterface,
	flowStep *FlowStep) (bool, *tidcommon.ServiceError) {
	if ctx.Graph == nil {
		return true, nil
	}

	// Determine current node info for scoping and interceptor execution context.
	currentNode := ctx.CurrentNode
	if node != nil {
		currentNode = node
	}

	var currentNodeID string
	var nodeType common.NodeType
	var skipInterceptors []string
	var executionPolicy *providers.ExecutionPolicy
	if currentNode != nil {
		currentNodeID = currentNode.GetID()
		nodeType = currentNode.GetType()
		skipInterceptors = extractSkipInterceptors(currentNode)
		executionPolicy = currentNode.GetExecutionPolicy()
	}

	isSegRestartAllowed := fe.isSegmentRestartAllowed(ctx, fe.logger)

	execCtx := &InterceptorRunnerContext{
		Ctx:                  ctx.Context,
		ExecutionID:          ctx.ExecutionID,
		AppID:                ctx.AppID,
		FlowType:             ctx.FlowType,
		FlowStatus:           flowStep.Status,
		CurrentNodeID:        currentNodeID,
		NodeType:             nodeType,
		SkipInterceptors:     skipInterceptors,
		ExecutionPolicy:      executionPolicy,
		AllowSegmentRestart:  isSegRestartAllowed,
		UserInputs:           maps.Clone(ctx.UserInputs),
		ForwardedData:        maps.Clone(ctx.ForwardedData),
		AdditionalData:       maps.Clone(ctx.AdditionalData),
		CurrentNodeInputs:    getNodeInputs(currentNode),
		ResolvedInterceptors: ctx.Graph.GetInterceptors(mode),
		SharedData:           ctx.InterceptorSharedData,
	}
	if execCtx.SharedData == nil {
		execCtx.SharedData = make(map[string]string)
	}

	resp, svcErr := fe.interceptorRunner.runInterceptors(mode, execCtx)
	if svcErr != nil {
		return false, svcErr
	}

	if consumed := execCtx.GetConsumedInputs(); len(consumed) > 0 {
		ctx.consumedInputs = append(ctx.consumedInputs, consumed...)
	}
	fe.updateContextWithInterceptorResponse(ctx, resp)

	continueExecution := fe.processInterceptorResponse(resp, flowStep)
	return continueExecution, nil
}

// extractSkipInterceptors reads the skipInterceptors property from a node's properties
// and returns it as a string slice.
func extractSkipInterceptors(node core.NodeInterface) []string {
	props := node.GetProperties()
	if props == nil {
		return nil
	}
	val, ok := props[common.NodePropertySkipInterceptors]
	if !ok {
		return nil
	}
	skipList, ok := val.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(skipList))
	for _, item := range skipList {
		if name, ok := item.(string); ok {
			result = append(result, name)
		}
	}
	return result
}

// runPostRequestInterceptorsOnExit runs POST_REQUEST interceptors when the flow is about to return.
// It handles error publishing for interceptor failures and flow error statuses. Consumed
// one-time-use inputs declared by request-scoped interceptors are cleared after the
// POST_REQUEST stage completes.
// Returns (true, nil) if execution should continue, (false, nil) if the flow should stop,
// or (false, svcErr) on interceptor failure.
func (fe *flowEngine) runPostRequestInterceptorsOnExit(
	ctx *EngineContext, flowStep *FlowStep, flowStartTime int64) (bool, *tidcommon.ServiceError) {
	interceptorExecSuccess, svcErr := fe.runInterceptors(
		providers.InterceptorModePostRequest, ctx, nil, flowStep)

	fe.clearRequestPackageOneTimeUseInputs(ctx)

	if svcErr != nil {
		publishFlowFailedEvent(ctx, svcErr, flowStartTime, time.Now().UnixMilli(), fe.observabilitySvc)
		return false, svcErr
	}
	if !interceptorExecSuccess {
		if flowStep.Status == providers.FlowStatusError {
			publishFlowFailedEvent(ctx, nil, flowStartTime, time.Now().UnixMilli(), fe.observabilitySvc)
		}
		return false, nil
	}

	return true, nil
}

// trackPresentedOptionalInputs records the optional inputs presented in an incomplete view response
// into the node response's runtime data so they can be skipped in subsequent execution steps.
func (fe *flowEngine) trackPresentedOptionalInputs(ctx *EngineContext, nodeResp *common.NodeResponse) {
	if nodeResp == nil || nodeResp.Status != common.NodeStatusIncomplete ||
		nodeResp.Type != common.NodeResponseTypeView || len(nodeResp.Inputs) == 0 {
		return
	}

	optionalIdentifiers := make([]string, 0, len(nodeResp.Inputs))
	for _, input := range nodeResp.Inputs {
		if !input.Required {
			optionalIdentifiers = append(optionalIdentifiers, input.Identifier)
		}
	}
	if len(optionalIdentifiers) == 0 {
		return
	}

	if nodeResp.RuntimeData == nil {
		nodeResp.RuntimeData = make(map[string]string)
	}

	raw := nodeResp.RuntimeData[common.RuntimeKeyPresentedOptionalInputs]
	if raw == "" && ctx != nil {
		raw = ctx.RuntimeData[common.RuntimeKeyPresentedOptionalInputs]
	}

	nodeResp.RuntimeData[common.RuntimeKeyPresentedOptionalInputs] =
		core.MergePresentedOptionalInputIdentifiers(raw, optionalIdentifiers)
}

// setCurrentExecutionNode sets the current execution node in the context.
func (fe *flowEngine) setCurrentExecutionNode(ctx *EngineContext,
	logger *log.Logger) *tidcommon.ServiceError {
	graph := ctx.Graph
	if graph == nil {
		logger.Error(ctx.Context, "Flow graph is not initialized in the context")
		return &tidcommon.InternalServerError
	}

	currentNode := ctx.CurrentNode
	if currentNode == nil {
		logger.Debug(ctx.Context, "Current node is nil. Setting start node as the current node.")
		var err error
		currentNode, err = graph.GetStartNode()
		if err != nil {
			logger.Error(ctx.Context, "Start node not found in the flow graph", log.Error(err))
			return &tidcommon.InternalServerError
		}
		ctx.CurrentNode = currentNode
	}

	// Initialize execution history map if needed
	if ctx.ExecutionHistory == nil {
		ctx.ExecutionHistory = make(map[string]*providers.NodeExecutionRecord)
	}

	return nil
}

// getNodeInputs extracts required inputs for a node.
func getNodeInputs(node core.NodeInterface) []providers.Input {
	if execNode, ok := node.(core.ExecutorBackedNodeInterface); ok {
		return execNode.GetInputs()
	}
	if promptNode, ok := node.(core.PromptNodeInterface); ok {
		var inputs []providers.Input
		for _, prompt := range promptNode.GetPrompts() {
			inputs = append(inputs, prompt.Inputs...)
		}
		return inputs
	}
	return nil
}

// setNodeExecutor sets the executor for the given node if it is not already set.
func (fe *flowEngine) setNodeExecutor(
	ctx context.Context, node core.NodeInterface, logger *log.Logger) *tidcommon.ServiceError {
	if node.GetType() != common.NodeTypeTaskExecution {
		return nil
	}
	executableNode, ok := node.(core.ExecutorBackedNodeInterface)
	if !ok {
		logger.Error(ctx, "Task execution node does not implement ExecutorBackedNodeInterface",
			log.String("nodeID", node.GetID()))
		return &tidcommon.InternalServerError
	}

	// Return if executor is already set
	if executableNode.GetExecutor() != nil {
		return nil
	}

	logger.Debug(ctx, "Executor not set for the node. Constructing executor.",
		log.String("nodeID", node.GetID()))

	executorName := executableNode.GetExecutorName()
	if executorName == "" {
		logger.Error(ctx, "Executor name not configured for executable node",
			log.String("nodeID", node.GetID()))
		return &tidcommon.InternalServerError
	}

	executor, err := fe.getExecutorByName(executorName)
	if err != nil {
		logger.Error(ctx, "Error constructing executor for node", log.String("nodeID", node.GetID()),
			log.String("executorName", executorName), log.Error(err))
		return &tidcommon.InternalServerError
	}

	executableNode.SetExecutor(executor)
	return nil
}

// getExecutorByName retrieves executor instance from the executor registry.
func (fe *flowEngine) getExecutorByName(executorName string) (providers.Executor, error) {
	exec, err := fe.executorRegistry.GetExecutor(executorName)
	if err != nil {
		return nil, fmt.Errorf("failed to get executor '%s': %w", executorName, err)
	}

	return exec, nil
}

// clearSensitiveInputs removes sensitive user inputs from the engine context after a node has executed.
// This cleanup is only applied for authentication flows.
func (fe *flowEngine) clearSensitiveInputs(ctx *EngineContext, node core.NodeInterface) {
	if ctx.FlowType != providers.FlowTypeAuthentication {
		return
	}

	execNode, ok := node.(core.ExecutorBackedNodeInterface)
	if !ok {
		return
	}

	// Get inputs from the node configuration. If the node does not define its own inputs,
	// fall back to the executor's default inputs.
	inputs := execNode.GetInputs()
	if len(inputs) == 0 {
		if executor := execNode.GetExecutor(); executor != nil {
			inputs = executor.GetDefaultInputs()
		}
	}

	for _, input := range inputs {
		if input.IsSensitive() {
			delete(ctx.UserInputs, input.Identifier)
		}
	}
}

// collectOneTimeUseIdentifiers appends identifiers declared as OneTimeUse from inputs into set.
func (fe *flowEngine) collectOneTimeUseIdentifiers(inputs []providers.Input, set map[string]struct{}) {
	for _, input := range inputs {
		if input.OneTimeUse {
			set[input.Identifier] = struct{}{}
		}
	}
}

// clearOneTimeUseInputsScope removes identifiers from ctx.consumedInputs that are declared
// OneTimeUse in scoped, deleting each matched identifier from ctx.UserInputs. Unmatched
// entries are preserved in ctx.consumedInputs for a later scope to handle.
func (fe *flowEngine) clearOneTimeUseInputsScope(ctx *EngineContext,
	scoped map[string]struct{}) {
	if len(ctx.consumedInputs) == 0 || len(scoped) == 0 {
		return
	}

	remaining := ctx.consumedInputs[:0]
	for _, id := range ctx.consumedInputs {
		if _, ok := scoped[id]; ok {
			delete(ctx.UserInputs, id)
			continue
		}
		remaining = append(remaining, id)
	}
	ctx.consumedInputs = remaining
}

// clearNodePackageOneTimeUseInputs removes any consumed identifier declared OneTimeUse on the
// node or on an applicable PRE_NODE/POST_NODE interceptor from ctx.UserInputs, and drops it
// from the shared consumed list.
func (fe *flowEngine) clearNodePackageOneTimeUseInputs(ctx *EngineContext, node core.NodeInterface) {
	if len(ctx.consumedInputs) == 0 || node == nil {
		return
	}

	scoped := make(map[string]struct{})
	if execNode, ok := node.(core.ExecutorBackedNodeInterface); ok {
		// Use the node's declared inputs when present, falling back to the executor's default
		// inputs otherwise.
		inputs := execNode.GetInputs()
		if len(inputs) == 0 {
			if executor := execNode.GetExecutor(); executor != nil {
				inputs = executor.GetDefaultInputs()
			}
		}
		fe.collectOneTimeUseIdentifiers(inputs, scoped)
	}

	if ctx.Graph != nil {
		nodeID := node.GetID()
		skipInterceptors := extractSkipInterceptors(node)
		for _, mode := range []providers.InterceptorMode{
			providers.InterceptorModePreNode,
			providers.InterceptorModePostNode,
		} {
			for _, unit := range ctx.Graph.GetInterceptors(mode) {
				if !shouldApplyToNode(unit, nodeID, skipInterceptors) {
					continue
				}
				ic := unit.GetInterceptor()
				if ic == nil {
					continue
				}
				fe.collectOneTimeUseIdentifiers(ic.GetInputs(), scoped)
			}
		}
	}

	fe.clearOneTimeUseInputsScope(ctx, scoped)
}

// clearRequestPackageOneTimeUseInputs removes any remaining consumed identifier declared
// OneTimeUse on a PRE_REQUEST/POST_REQUEST interceptor from ctx.UserInputs, then drains the
// consumed list.
func (fe *flowEngine) clearRequestPackageOneTimeUseInputs(ctx *EngineContext) {
	if len(ctx.consumedInputs) == 0 {
		return
	}

	if ctx.Graph != nil {
		scoped := make(map[string]struct{})
		for _, mode := range []providers.InterceptorMode{
			providers.InterceptorModePreRequest,
			providers.InterceptorModePostRequest,
		} {
			for _, unit := range ctx.Graph.GetInterceptors(mode) {
				ic := unit.GetInterceptor()
				if ic == nil {
					continue
				}
				fe.collectOneTimeUseIdentifiers(ic.GetInputs(), scoped)
			}
		}
		fe.clearOneTimeUseInputsScope(ctx, scoped)
	}

	// Drop anything left over. Non-OneTimeUse signals are ignored by design; leftover entries
	// have no declared scope in this graph and would otherwise leak across requests.
	ctx.consumedInputs = nil
}

// updateContextWithNodeResponse updates the engine context with the node response and authenticated user.
func (fe *flowEngine) updateContextWithNodeResponse(engineCtx *EngineContext, nodeResp *common.NodeResponse) {
	engineCtx.CurrentNodeResponse = nodeResp

	// Clear action only when node completes or forwards
	if nodeResp.Status == common.NodeStatusComplete || nodeResp.Status == common.NodeStatusForward {
		engineCtx.CurrentAction = ""
	}

	// Handle runtime data from the node response
	if len(nodeResp.RuntimeData) > 0 {
		if engineCtx.RuntimeData == nil {
			engineCtx.RuntimeData = make(map[string]string)
		}
		engineCtx.RuntimeData = sysutils.MergeStringMaps(engineCtx.RuntimeData, nodeResp.RuntimeData)
	}

	// Handle additional data from the node response (e.g., passkeyCreationOptions, passkeyChallenge)
	if len(nodeResp.AdditionalData) > 0 {
		if engineCtx.AdditionalData == nil {
			engineCtx.AdditionalData = make(map[string]string)
		}
		engineCtx.AdditionalData = sysutils.MergeStringMaps(engineCtx.AdditionalData, nodeResp.AdditionalData)
	}

	// Add assertion to the context
	if nodeResp.Assertion != "" {
		engineCtx.Assertion = nodeResp.Assertion
	}

	// Handle forwarded data from the node response
	// It replaces any existing forwarded data rather than merging
	if len(nodeResp.ForwardedData) > 0 {
		engineCtx.ForwardedData = nodeResp.ForwardedData
	}

	if nodeResp.AuthUser.IsAuthenticated() {
		engineCtx.AuthUser = nodeResp.AuthUser
	}
}

// updateContextWithInterceptorResponse merges interceptor response data into the engine context.
func (fe *flowEngine) updateContextWithInterceptorResponse(
	engineCtx *EngineContext, resp *common.InterceptorResponse,
) {
	if resp == nil {
		return
	}

	if len(resp.EngineOutputs) > 0 {
		engineCtx.mergeRuntimeData(resp.EngineOutputs)
	}
}

// processInterceptorResponse processes the interceptor response and determines whether to continue
// execution. Returns true if execution should continue, false if it should stop.
func (fe *flowEngine) processInterceptorResponse(resp *common.InterceptorResponse,
	flowStep *FlowStep) bool {
	if resp == nil {
		return true
	}

	switch resp.Status {
	case common.InterceptorStatusComplete:
		fe.updateFlowStepWithInterceptorResponse(flowStep, resp)
		return true
	case common.InterceptorStatusIncomplete:
		fe.updateFlowStepWithInterceptorResponse(flowStep, resp)
		flowStep.Status = providers.FlowStatusIncomplete
		return false
	case common.InterceptorStatusFailure:
		flowStep.Status = providers.FlowStatusError
		flowStep.Error = resp.Error
		return false
	default:
		return true
	}
}

// updateFlowStepWithInterceptorResponse updates the FlowStep with relevant information from the interceptor response,
// such as errors, field errors, and challenge tokens.
func (fe *flowEngine) updateFlowStepWithInterceptorResponse(flowStep *FlowStep, resp *common.InterceptorResponse) {
	if resp == nil {
		return
	}
	if resp.Error != nil {
		flowStep.Error = resp.Error
	}
	if len(resp.FieldErrors) > 0 {
		flowStep.Data.FieldErrors = resp.FieldErrors
	}
	if resp.ChallengeToken != "" {
		flowStep.ChallengeToken = resp.ChallengeToken
	}
}

// processNodeResponse processes the node response and determines the next action.
// Returns:
// - The next node to execute.
// - Whether to continue execution.
// - Any service error.
func (fe *flowEngine) processNodeResponse(ctx *EngineContext, nodeResp *common.NodeResponse,
	flowStep *FlowStep, logger *log.Logger) (
	core.NodeInterface, bool, *tidcommon.ServiceError) {
	if nodeResp.Status == "" {
		logger.Error(ctx.Context, "Node response status not found in the flow graph")
		return nil, false, &tidcommon.InternalServerError
	}

	// Carry any SSO handle minted by this node onto the flow step so the transport layer can emit
	// it. The Session node emits the handle on the engine-only EngineData channel (never returned to
	// the client and off the engine contract). Stamped here (not only at completion) so it survives
	// an immediately following prompt step that returns the flow as incomplete.
	if handle := nodeResp.EngineData[common.RuntimeKeySSOSessionHandle]; handle != "" {
		flowStep.SSOHandleOut = handle
		flowStep.SSOFlowID = ssoFlowID(ctx)
	}

	switch nodeResp.Status {
	case common.NodeStatusComplete:
		if fe.isDisplayOnlyPromptNode(ctx.CurrentNode) {
			return fe.handleDisplayOnlyPromptResponse(ctx, nodeResp, flowStep, logger)
		}

		nextNode, svcErr := fe.handleCompletedResponse(ctx, nodeResp, logger)
		if svcErr != nil {
			return nil, false, svcErr
		}
		return nextNode, true, nil
	case common.NodeStatusIncomplete:
		svcErr := fe.handleIncompleteResponse(ctx, nodeResp, flowStep, logger)
		if svcErr != nil {
			return nil, false, svcErr
		}
		return nil, false, nil
	case common.NodeStatusForward:
		nextNode, svcErr := fe.handleForwardResponse(ctx, nodeResp, logger)
		if svcErr != nil {
			return nil, false, svcErr
		}
		return nextNode, true, nil
	case common.NodeStatusCall:
		nextNode, svcErr := fe.handleCallResponse(ctx, nodeResp, logger)
		if svcErr != nil {
			return nil, false, svcErr
		}
		return nextNode, true, nil
	case common.NodeStatusFailure:
		if ctx.frameDepth() > 0 {
			return fe.handleCalleeFailure(ctx, nodeResp, flowStep, logger)
		}
		flowStep.Status = providers.FlowStatusError
		flowStep.Error = nodeResp.Error
		return nil, false, nil
	default:
		logger.Error(ctx.Context, "Unsupported response status returned from the node",
			log.String("status", string(nodeResp.Status)))
		return nil, false, &tidcommon.InternalServerError
	}
}

// isDisplayOnlyPromptNode checks if the current node is a display-only prompt node.
func (fe *flowEngine) isDisplayOnlyPromptNode(node core.NodeInterface) bool {
	promptNode, ok := node.(core.PromptNodeInterface)
	return ok && promptNode.IsDisplayOnly()
}

// handleDisplayOnlyPromptResponse handles the response for display-only prompt nodes.
// It checks the next node and updates the flow step accordingly.
func (fe *flowEngine) handleDisplayOnlyPromptResponse(ctx *EngineContext,
	nodeResp *common.NodeResponse, flowStep *FlowStep, logger *log.Logger) (
	core.NodeInterface, bool, *tidcommon.ServiceError) {
	promptNode := ctx.CurrentNode.(core.PromptNodeInterface)
	nextNodeID := promptNode.GetNextNode()
	continueExecution := false

	nextNode, exists := ctx.Graph.GetNode(nextNodeID)
	if !exists || nextNode == nil {
		logger.Error(ctx.Context, "Display-only prompt references unknown next node",
			log.String("nextNodeID", nextNodeID))
		return nil, continueExecution, &tidcommon.InternalServerError
	}

	// If the next node is END, complete the flow
	if nextNode.GetType() == common.NodeTypeEnd {
		flowStep.Status = providers.FlowStatusComplete
		continueExecution = true
	} else {
		// Set current node to the next node so that flow can be resumed from there in the next execution
		ctx.CurrentNode = nextNode
		flowStep.Status = providers.FlowStatusIncomplete
		flowStep.Type = common.StepTypeView

		// Set current segment to the next node's segment if segments are defined in the graph
		if ctx.Graph.HasSegments() {
			if seg := ctx.Graph.GetSegmentByStartNode(nextNode.GetID()); seg != nil {
				ctx.CurrentSegmentID = seg.ID
			}
		}
	}

	// Copy additional data from context
	if len(ctx.AdditionalData) > 0 {
		flowStep.Data.AdditionalData = ctx.AdditionalData
	}

	// Include meta in the flow step if present
	if nodeResp.Meta != nil {
		flowStep.Data.Meta = nodeResp.Meta
	}

	// Include additionalData in the flow step if present
	if len(nodeResp.AdditionalData) > 0 {
		if flowStep.Data.AdditionalData == nil {
			flowStep.Data.AdditionalData = make(map[string]string)
		}
		maps.Copy(flowStep.Data.AdditionalData, nodeResp.AdditionalData)
	}

	return nil, continueExecution, nil
}

// handleCompletedResponse handles the completed node and returns the next node to execute.
func (fe *flowEngine) handleCompletedResponse(ctx *EngineContext,
	nodeResp *common.NodeResponse, logger *log.Logger) (
	core.NodeInterface, *tidcommon.ServiceError) {
	// If we're in a callee flow and the completed node is END, pop the frame and
	// route to the caller call node's onSuccess
	if ctx.frameDepth() > 0 && ctx.CurrentNode.GetType() == common.NodeTypeEnd {
		return fe.handleCalleeReturn(ctx, logger)
	}

	nextNode, err := fe.resolveToNextNode(ctx, nodeResp)
	if err != nil {
		logger.Error(ctx.Context, "Error moving to the next node", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	ctx.CurrentNode = nextNode
	return nextNode, nil
}

// handleIncompleteResponse handles the node response when the status is incomplete.
// It resolves the flow step details based on the type of node response. The same node will be executed again
// in the next request with the required data.
func (fe *flowEngine) handleIncompleteResponse(ctx *EngineContext, nodeResp *common.NodeResponse,
	flowStep *FlowStep, logger *log.Logger) *tidcommon.ServiceError {
	switch nodeResp.Type {
	case common.NodeResponseTypeRedirection:
		err := fe.resolveStepForRedirection(ctx, nodeResp, flowStep)
		if err != nil {
			logger.Error(ctx.Context, "Error while resolving step for redirection", log.Error(err))
			return &tidcommon.InternalServerError
		}
		return nil
	case common.NodeResponseTypeView:
		err := fe.resolveStepDetailsForPrompt(ctx, nodeResp, flowStep)
		if err != nil {
			logger.Error(ctx.Context, "Error while resolving step details for prompt", log.Error(err))
			return &tidcommon.InternalServerError
		}
		return nil
	default:
		logger.Error(ctx.Context, "Unsupported response type returned from the node",
			log.String("responseType", string(nodeResp.Type)))
		return &tidcommon.InternalServerError
	}
	// TODO: Handle retry scenarios with nodeResp.Type == common.NodeResponseTypeRetry
}

// handleForwardResponse handles forwarding to the next node (e.g., onFailure handler)
func (fe *flowEngine) handleForwardResponse(ctx *EngineContext,
	nodeResp *common.NodeResponse, logger *log.Logger) (
	core.NodeInterface, *tidcommon.ServiceError) {
	errorMsg := ""
	if nodeResp.Error != nil {
		errorMsg = nodeResp.Error.Error.DefaultValue
	}
	logger.Debug(ctx.Context, "Forwarding to next node",
		log.String("nextNodeID", nodeResp.NextNodeID),
		log.String("error", errorMsg))

	nextNode, err := fe.resolveToNextNode(ctx, nodeResp)
	if err != nil {
		logger.Error(ctx.Context, "Error resolving to next node", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	ctx.CurrentNode = nextNode
	return nextNode, nil
}

// handleCallResponse handles a NodeStatusCall response by pushing a frame and switching
// context to the callee flow's start node.
func (fe *flowEngine) handleCallResponse(ctx *EngineContext,
	nodeResp *common.NodeResponse, logger *log.Logger) (
	core.NodeInterface, *tidcommon.ServiceError) {
	if ctx.frameDepth() >= maxCallDepth {
		logger.Debug(ctx.Context, "Maximum call depth exceeded", log.Int("frameDepth", ctx.frameDepth()),
			log.Int("maxCallDepth", maxCallDepth))
		return nil, &ErrorMaxCallDepthExceeded
	}

	flow, svcErr := fe.flowProvider.GetFlow(ctx.Context, nodeResp.CallTargetFlowID)
	if svcErr != nil {
		logger.Error(ctx.Context, "Failed to get call target flow",
			log.String("targetFlowID", nodeResp.CallTargetFlowID))
		return nil, &tidcommon.InternalServerError
	}

	calleeGraph, svcErr := fe.graphBuilder.GetGraph(ctx.Context, flow)
	if svcErr != nil {
		logger.Error(ctx.Context, "Failed to build call target graph",
			log.String("targetFlowID", nodeResp.CallTargetFlowID))
		return nil, &tidcommon.InternalServerError
	}

	return fe.switchContextToCallee(ctx, nodeResp, calleeGraph, logger)
}

// switchContextToCallee switches the engine context to the callee flow's graph and
// sets the current node to the start node.
func (fe *flowEngine) switchContextToCallee(ctx *EngineContext,
	nodeResp *common.NodeResponse, calleeGraph core.GraphInterface, logger *log.Logger) (
	core.NodeInterface, *tidcommon.ServiceError) {
	// Push the current frame before switching to the callee
	resumeCallNodeID := ctx.CurrentNode.GetID()
	ctx.pushFrame(resumeCallNodeID)

	// Switch context to the callee flow. Per-frame transient state is reset so the callee
	// starts with a clean slate; shared identity/input state (UserInputs, AuthUser, etc.) is
	// intentionally left in place
	ctx.Graph = calleeGraph
	ctx.FlowType = calleeGraph.GetType()
	ctx.RuntimeData = make(map[string]string)
	ctx.ForwardedData = nil
	ctx.AdditionalData = make(map[string]string)
	ctx.CurrentNodeResponse = nil
	ctx.CurrentSegmentID = ""
	ctx.CurrentAction = ""

	// Set the current node to the start node of the callee graph
	startNode, startErr := calleeGraph.GetStartNode()
	if startErr != nil {
		logger.Error(ctx.Context, "Callee flow has no start node",
			log.String("targetFlowID", nodeResp.CallTargetFlowID), log.Error(startErr))
		return nil, &tidcommon.InternalServerError
	}
	ctx.CurrentNode = startNode

	return startNode, nil
}

// handleCalleeReturn is called when the callee flow's END node completes while there is a
// caller frame on the stack. It pops the frame and routes to the caller call node's onSuccess.
func (fe *flowEngine) handleCalleeReturn(ctx *EngineContext, logger *log.Logger) (
	core.NodeInterface, *tidcommon.ServiceError) {
	savedFrame := ctx.popFrame()
	if savedFrame == nil {
		logger.Error(ctx.Context, "Frame stack underflow on callee return")
		return nil, &tidcommon.InternalServerError
	}

	// Find the call node in the restored caller graph
	callNode, exists := ctx.Graph.GetNode(savedFrame.resumeCallNodeID)
	if !exists {
		logger.Error(ctx.Context, "Caller call node not found after frame pop",
			log.String("callNodeID", savedFrame.resumeCallNodeID))
		return nil, &tidcommon.InternalServerError
	}

	cn, ok := callNode.(core.CallNodeInterface)
	if !ok {
		logger.Error(ctx.Context, "Caller resume node is not a CallNodeInterface",
			log.String("callNodeID", savedFrame.resumeCallNodeID))
		return nil, &tidcommon.InternalServerError
	}

	onSuccessID := cn.GetOnSuccess()
	if onSuccessID == "" {
		logger.Error(ctx.Context, "call node has no onSuccess target",
			log.String("callNodeID", savedFrame.resumeCallNodeID))
		return nil, &tidcommon.InternalServerError
	}

	nextNode, ok := ctx.Graph.GetNode(onSuccessID)
	if !ok {
		logger.Error(ctx.Context, "call onSuccess node not found",
			log.String("onSuccessID", onSuccessID))
		return nil, &tidcommon.InternalServerError
	}
	ctx.CurrentNode = nextNode

	return nextNode, nil
}

// handleCalleeFailure is called when the callee flow ends with NodeStatusFailure while
// there is a caller frame on the stack. It pops the frame and either routes to the caller
// call node's onFailure (forwarding the error) or terminates the whole flow.
func (fe *flowEngine) handleCalleeFailure(ctx *EngineContext, nodeResp *common.NodeResponse,
	flowStep *FlowStep, logger *log.Logger) (
	core.NodeInterface, bool, *tidcommon.ServiceError) {
	savedFrame := ctx.popFrame()
	if savedFrame == nil {
		logger.Error(ctx.Context, "Frame stack underflow on callee failure")
		return nil, false, &tidcommon.InternalServerError
	}

	callNode, exists := ctx.Graph.GetNode(savedFrame.resumeCallNodeID)
	if !exists {
		logger.Error(ctx.Context, "Caller call node not found after failure frame pop",
			log.String("callNodeID", savedFrame.resumeCallNodeID))
		return nil, false, &tidcommon.InternalServerError
	}

	cn, ok := callNode.(core.CallNodeInterface)
	if !ok {
		logger.Error(ctx.Context, "Caller resume node is not a CallNodeInterface on failure",
			log.String("callNodeID", savedFrame.resumeCallNodeID))
		return nil, false, &tidcommon.InternalServerError
	}

	// If the call node has no onFailure target, terminate the flow with error
	onFailureID := cn.GetOnFailure()
	if onFailureID == "" {
		flowStep.Status = providers.FlowStatusError
		flowStep.Error = nodeResp.Error
		return nil, false, nil
	}

	// If the call node has an onFailure target, forward to that node and continue execution
	nextNode, exists := ctx.Graph.GetNode(onFailureID)
	if !exists {
		logger.Error(ctx.Context, "Call onFailure node not found",
			log.String("onFailureID", onFailureID))
		return nil, false, &tidcommon.InternalServerError
	}

	ctx.CurrentNode = nextNode
	ctx.CurrentNodeResponse = &common.NodeResponse{
		Status:     common.NodeStatusForward,
		NextNodeID: onFailureID,
		Error:      nodeResp.Error,
	}

	return nextNode, true, nil
}

// skipToNextNode skips the current node and moves to the next node. It updates the context with the
// next node and returns it.
func (fe *flowEngine) skipToNextNode(ctx *EngineContext, currentNode core.NodeInterface,
	logger *log.Logger) (core.NodeInterface, *tidcommon.ServiceError) {
	condition := currentNode.GetCondition()

	// Condition must specify where to skip to
	if condition == nil || condition.OnSkip == "" {
		logger.Error(ctx.Context, "Node has condition but onSkip is not specified",
			log.String("nodeID", currentNode.GetID()))
		return nil, &tidcommon.InternalServerError
	}

	logger.Debug(ctx.Context, "Using condition's onSkip for skipped node",
		log.String("nodeID", currentNode.GetID()), log.String("onSkip", condition.OnSkip))

	nodeResp := &common.NodeResponse{NextNodeID: condition.OnSkip}

	// Resolve to the next node
	nextNode, err := fe.resolveToNextNode(ctx, nodeResp)
	if err != nil {
		logger.Error(ctx.Context, "Error moving to the next node after skipping", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	ctx.CurrentNode = nextNode
	return nextNode, nil
}

// resolveToNextNode resolves the next node to execute based on nodeResp.NextNodeID.
func (fe *flowEngine) resolveToNextNode(engineCtx *EngineContext, nodeResp *common.NodeResponse) (
	core.NodeInterface, error) {
	logger := fe.logger.With(log.String(log.LoggerKeyExecutionID, engineCtx.ExecutionID))
	graph := engineCtx.Graph
	if nodeResp == nil || nodeResp.NextNodeID == "" {
		logger.Debug(engineCtx.Context, "No next node ID in response. Returning nil.")
		return nil, nil
	}

	nextNode, ok := graph.GetNode(nodeResp.NextNodeID)
	if !ok {
		return nil, errors.New("next node not found in the graph")
	}

	logger.Debug(engineCtx.Context, "Moving to next node", log.String("nextNodeID", nextNode.GetID()))
	return nextNode, nil
}

// resolveStepForRedirection resolves the flow step details for a redirection response.
func (fe *flowEngine) resolveStepForRedirection(ctx *EngineContext, nodeResp *common.NodeResponse,
	flowStep *FlowStep) error {
	if nodeResp == nil {
		return errors.New("node response is nil")
	}
	if nodeResp.RedirectURL == "" {
		return errors.New("redirect URL not found in the node response")
	}

	// Copy additional data from context (accumulated from all node responses)
	if len(ctx.AdditionalData) > 0 {
		flowStep.Data.AdditionalData = ctx.AdditionalData
	}

	flowStep.Data.RedirectURL = nodeResp.RedirectURL

	if flowStep.Data.Inputs == nil {
		flowStep.Data.Inputs = make([]providers.Input, 0)
		flowStep.Data.Inputs = nodeResp.Inputs
	} else {
		// Append to the existing inputs
		flowStep.Data.Inputs = append(flowStep.Data.Inputs, nodeResp.Inputs...)
	}

	flowStep.Status = providers.FlowStatusIncomplete
	flowStep.Type = common.StepTypeRedirection
	return nil
}

// resolveStepDetailsForPrompt resolves the step details for a user prompt response.
func (fe *flowEngine) resolveStepDetailsForPrompt(ctx *EngineContext, nodeResp *common.NodeResponse,
	flowStep *FlowStep) error {
	if nodeResp == nil {
		return errors.New("node response is nil")
	}
	if len(nodeResp.Inputs) == 0 && len(nodeResp.Actions) == 0 {
		return errors.New("no required data or actions found in the node response")
	}

	if len(nodeResp.Inputs) > 0 {
		if flowStep.Data.Inputs == nil {
			flowStep.Data.Inputs = make([]providers.Input, 0)
			flowStep.Data.Inputs = nodeResp.Inputs
		} else {
			// Append to the existing inputs
			flowStep.Data.Inputs = append(flowStep.Data.Inputs, nodeResp.Inputs...)
		}
	}

	if len(nodeResp.Actions) > 0 {
		if flowStep.Data.Actions == nil {
			flowStep.Data.Actions = make([]common.Action, 0)
		}
		flowStep.Data.Actions = nodeResp.Actions
	}

	// Copy additional data from context (accumulated from all node responses)
	if len(ctx.AdditionalData) > 0 {
		flowStep.Data.AdditionalData = ctx.AdditionalData
	}

	// Include meta in the flow step if present
	if nodeResp.Meta != nil {
		flowStep.Data.Meta = nodeResp.Meta
	}

	// Include additionalData in the flow step if present
	if len(nodeResp.AdditionalData) > 0 {
		if flowStep.Data.AdditionalData == nil {
			flowStep.Data.AdditionalData = make(map[string]string)
		}
		for key, value := range nodeResp.AdditionalData {
			flowStep.Data.AdditionalData[key] = value
		}
	}

	// Set error if present (e.g., when handling onFailure)
	if nodeResp.Error != nil {
		flowStep.Error = nodeResp.Error
	}

	if len(nodeResp.FieldErrors) > 0 {
		flowStep.Data.FieldErrors = nodeResp.FieldErrors
	}

	flowStep.Status = providers.FlowStatusIncomplete
	flowStep.Type = common.StepTypeView
	return nil
}

// isSegmentRestartAllowed checks whether the current flow is resuming inside a segment whose
// start node allows segment restart.
func (fe *flowEngine) isSegmentRestartAllowed(ctx *EngineContext, logger *log.Logger) bool {
	if ctx.Graph == nil || !ctx.Graph.HasSegments() || ctx.CurrentSegmentID == "" {
		return false
	}

	seg := ctx.Graph.GetSegmentByID(ctx.CurrentSegmentID)
	if seg == nil {
		return false
	}

	segStartNode, exists := ctx.Graph.GetNode(seg.StartNodeID)
	if !exists {
		return false
	}

	if svcErr := fe.setNodeExecutor(ctx.Context, segStartNode, logger); svcErr != nil {
		return false
	}

	policy := segStartNode.GetExecutionPolicy()
	return policy != nil && policy.AllowSegmentRestart
}

// recordNodeExecution adds or updates execution record for the node.
func recordNodeExecution(ctx *EngineContext, node core.NodeInterface, nodeResp *common.NodeResponse,
	nodeErr *tidcommon.ServiceError, executionStartTime int64, executionEndTime int64) {
	nodeID := node.GetID()
	record := ctx.ExecutionHistory[nodeID]

	// Create new record if it does not exist
	if record == nil {
		nextStep := len(ctx.ExecutionHistory) + 1
		newRecord := createExecutionRecord(node, nextStep)
		ctx.ExecutionHistory[nodeID] = &newRecord
		record = &newRecord
	}

	attempt := createExecutionAttempt(record, nodeResp, nodeErr, executionStartTime, executionEndTime)
	record.Executions = append(record.Executions, attempt)

	record.Status = attempt.Status
	record.EndTime = attempt.EndTime
}

// createExecutionRecord creates a new node execution record.
func createExecutionRecord(node core.NodeInterface, step int) providers.NodeExecutionRecord {
	record := providers.NodeExecutionRecord{
		NodeID:     node.GetID(),
		NodeType:   string(node.GetType()),
		Step:       step,
		Status:     providers.FlowStatusIncomplete,
		Executions: make([]providers.ExecutionAttempt, 0),
		StartTime:  time.Now().Unix(),
	}

	// Set executor details if applicable (only for executor-backed nodes)
	if node.GetType() == common.NodeTypeTaskExecution {
		if executableNode, ok := node.(core.ExecutorBackedNodeInterface); ok {
			executor := executableNode.GetExecutor()
			if executor != nil {
				record.ExecutorName = executor.GetName()
				record.ExecutorType = executor.GetType()
			}
			record.ExecutorMode = executableNode.GetMode()
		}
	}

	return record
}

// createExecutionAttempt creates a new execution attempt.
func createExecutionAttempt(nodeRecord *providers.NodeExecutionRecord, nodeResp *common.NodeResponse,
	nodeErr *tidcommon.ServiceError, executionStartTime int64, executionEndTime int64) providers.ExecutionAttempt {
	attempt := providers.ExecutionAttempt{
		Attempt:   len(nodeRecord.Executions) + 1,
		Timestamp: executionEndTime,
		StartTime: executionStartTime,
		EndTime:   executionEndTime,
	}

	// Determine status
	if nodeErr != nil {
		attempt.Status = providers.FlowStatusError
	} else if nodeResp != nil {
		switch nodeResp.Status {
		case common.NodeStatusComplete:
			attempt.Status = providers.FlowStatusComplete
		case common.NodeStatusIncomplete:
			attempt.Status = providers.FlowStatusIncomplete
		case common.NodeStatusFailure:
			attempt.Status = providers.FlowStatusError
		default:
			attempt.Status = providers.FlowStatusIncomplete
		}
	}

	return attempt
}

// publishNodeExecutionStartedEvent publishes an observability event when node execution starts.
func publishNodeExecutionStartedEvent(
	ctx *EngineContext,
	node core.NodeInterface,
	obsSvc providers.ObservabilityProvider,
) {
	if obsSvc == nil || !obsSvc.IsEnabled() {
		return
	}

	// Get node execution record to determine step number and attempt
	record := ctx.ExecutionHistory[node.GetID()]
	stepNumber := len(ctx.ExecutionHistory) + 1
	attemptNumber := 1
	if record != nil {
		stepNumber = record.Step
		attemptNumber = len(record.Executions) + 1
	}

	evt := event.NewEvent(
		ctx.ExecutionID, // Use ExecutionID as TraceID
		string(event.EventTypeFlowNodeExecutionStarted),
		event.ComponentFlowEngine,
	).
		WithStatus(providers.StatusInProgress).
		WithData(event.DataKey.ExecutionID, ctx.ExecutionID).
		WithData(event.DataKey.FlowType, string(ctx.FlowType)).
		WithData(event.DataKey.NodeID, node.GetID()).
		WithData(event.DataKey.NodeType, string(node.GetType())).
		WithData(event.DataKey.StepNumber, fmt.Sprintf("%d", stepNumber)).
		WithData(event.DataKey.AttemptNumber, fmt.Sprintf("%d", attemptNumber)).
		WithData(event.DataKey.EntityID, ctx.AppID)

	obsSvc.PublishEvent(ctx.Context, evt)
}

// publishNodeExecutionCompletedEvent publishes an observability event when node execution completes or fails.
func publishNodeExecutionCompletedEvent(ctx *EngineContext, node core.NodeInterface,
	nodeResp *common.NodeResponse, nodeErr *tidcommon.ServiceError,
	executionStartTime int64, executionEndTime int64, obsSvc providers.ObservabilityProvider) {
	if obsSvc == nil || !obsSvc.IsEnabled() {
		return
	}

	// Get node execution record to determine step number and attempt
	record := ctx.ExecutionHistory[node.GetID()]
	if record == nil {
		return
	}

	stepNumber := record.Step
	attemptNumber := len(record.Executions)

	// Determine event type and status based on outcome
	var eventType providers.EventType
	var status string
	var nodeStatus string

	if nodeErr != nil {
		eventType = event.EventTypeFlowNodeExecutionFailed
		status = providers.StatusFailure
		nodeStatus = string(providers.FlowStatusError)
	} else if nodeResp != nil {
		switch nodeResp.Status {
		case common.NodeStatusComplete:
			eventType = event.EventTypeFlowNodeExecutionCompleted
			status = providers.StatusSuccess
			nodeStatus = string(providers.FlowStatusComplete)
		case common.NodeStatusIncomplete:
			eventType = event.EventTypeFlowNodeExecutionCompleted
			status = providers.StatusSuccess
			nodeStatus = string(providers.FlowStatusIncomplete)
		case common.NodeStatusFailure:
			eventType = event.EventTypeFlowNodeExecutionFailed
			status = providers.StatusFailure
			nodeStatus = string(providers.FlowStatusError)
		default:
			eventType = event.EventTypeFlowNodeExecutionCompleted
			status = providers.StatusSuccess
			nodeStatus = string(nodeResp.Status)
		}
	} else {
		eventType = event.EventTypeFlowNodeExecutionCompleted
		status = providers.StatusSuccess
		nodeStatus = string(providers.FlowStatusComplete)
	}

	// Calculate duration in milliseconds
	durationMs := executionEndTime - executionStartTime

	evt := event.NewEvent(
		ctx.ExecutionID, // Use ExecutionID as TraceID
		string(eventType),
		event.ComponentFlowEngine,
	).
		WithStatus(status).
		WithData(event.DataKey.ExecutionID, ctx.ExecutionID).
		WithData(event.DataKey.FlowType, string(ctx.FlowType)).
		WithData(event.DataKey.NodeID, node.GetID()).
		WithData(event.DataKey.NodeType, string(node.GetType())).
		WithData(event.DataKey.NodeStatus, nodeStatus).
		WithData(event.DataKey.StepNumber, fmt.Sprintf("%d", stepNumber)).
		WithData(event.DataKey.AttemptNumber, fmt.Sprintf("%d", attemptNumber)).
		WithData(event.DataKey.DurationMs, fmt.Sprintf("%d", durationMs)).
		WithData(event.DataKey.EntityID, ctx.AppID)

	// Add error or failure details
	if nodeErr != nil {
		evt.WithData(event.DataKey.Error, processServiceErrorForEventPublish(nodeErr))
	} else if nodeResp != nil && nodeResp.Error != nil {
		evt.WithData(event.DataKey.Error, processNodeResponseErrorForEventPublish(nodeResp))
	}

	// Add user ID if authenticated
	if ctx.AuthenticatedUser.IsAuthenticated && ctx.AuthenticatedUser.UserID != "" {
		evt.WithData(event.DataKey.UserID, ctx.AuthenticatedUser.UserID)
	}

	obsSvc.PublishEvent(ctx.Context, evt)
}

// publishFlowStartedEvent publishes an observability event when flow execution starts.
func publishFlowStartedEvent(ctx *EngineContext, obsSvc providers.ObservabilityProvider) {
	if obsSvc == nil || !obsSvc.IsEnabled() {
		return
	}

	evt := event.NewEvent(
		ctx.TraceID, // Use TraceID from context
		string(event.EventTypeFlowStarted),
		event.ComponentFlowEngine,
	).
		WithStatus(providers.StatusInProgress).
		WithData(event.DataKey.ExecutionID, ctx.ExecutionID).
		WithData(event.DataKey.FlowType, string(ctx.FlowType)).
		WithData(event.DataKey.EntityID, ctx.AppID)

	// Add user ID if already authenticated
	if ctx.AuthenticatedUser.IsAuthenticated && ctx.AuthenticatedUser.UserID != "" {
		evt.WithData(event.DataKey.UserID, ctx.AuthenticatedUser.UserID)
	}

	obsSvc.PublishEvent(ctx.Context, evt)
}

// publishFlowCompletedEvent publishes an observability event when flow execution completes successfully.
func publishFlowCompletedEvent(
	ctx *EngineContext,
	flowStartTime int64,
	flowEndTime int64,
	obsSvc providers.ObservabilityProvider,
) {
	if obsSvc == nil || !obsSvc.IsEnabled() {
		return
	}

	// Calculate duration in milliseconds
	durationMs := flowEndTime - flowStartTime

	evt := event.NewEvent(
		ctx.ExecutionID, // Use ExecutionID as TraceID
		string(event.EventTypeFlowCompleted),
		event.ComponentFlowEngine,
	).
		WithStatus(providers.StatusSuccess).
		WithData(event.DataKey.ExecutionID, ctx.ExecutionID).
		WithData(event.DataKey.FlowType, string(ctx.FlowType)).
		WithData(event.DataKey.EntityID, ctx.AppID).
		WithData(event.DataKey.DurationMs, fmt.Sprintf("%d", durationMs))

	// Add user ID if authenticated
	if ctx.AuthenticatedUser.IsAuthenticated && ctx.AuthenticatedUser.UserID != "" {
		evt.WithData(event.DataKey.UserID, ctx.AuthenticatedUser.UserID)
	}

	obsSvc.PublishEvent(ctx.Context, evt)
}

// publishFlowFailedEvent publishes an observability event when flow execution fails.
func publishFlowFailedEvent(ctx *EngineContext, svcErr *tidcommon.ServiceError,
	flowStartTime int64, flowEndTime int64, obsSvc providers.ObservabilityProvider) {
	if obsSvc == nil || !obsSvc.IsEnabled() {
		return
	}

	// Calculate duration in milliseconds
	durationMs := flowEndTime - flowStartTime

	evt := event.NewEvent(
		ctx.TraceID, // Use TraceID from context
		string(event.EventTypeFlowFailed),
		event.ComponentFlowEngine,
	).
		WithStatus(providers.StatusFailure).
		WithData(event.DataKey.ExecutionID, ctx.ExecutionID).
		WithData(event.DataKey.FlowType, string(ctx.FlowType)).
		WithData(event.DataKey.EntityID, ctx.AppID).
		WithData(event.DataKey.DurationMs, fmt.Sprintf("%d", durationMs))

	// Add error details if available
	if svcErr != nil {
		evt.WithData(event.DataKey.Error, processServiceErrorForEventPublish(svcErr))
	}

	// Add user ID if authenticated
	if ctx.AuthenticatedUser.IsAuthenticated && ctx.AuthenticatedUser.UserID != "" {
		evt.WithData(event.DataKey.UserID, ctx.AuthenticatedUser.UserID)
	}

	obsSvc.PublishEvent(ctx.Context, evt)
}

// processServiceErrorForEventPublish processes a service error to extract relevant information
// for observability events.
func processServiceErrorForEventPublish(svcErr *tidcommon.ServiceError) map[string]interface{} {
	if svcErr == nil {
		return nil
	}

	return map[string]interface{}{
		"code": svcErr.Code,
		"message": map[string]string{
			"key":          svcErr.Error.Key,
			"defaultValue": svcErr.Error.DefaultValue,
		},
		"description": map[string]string{
			"key":          svcErr.ErrorDescription.Key,
			"defaultValue": svcErr.ErrorDescription.DefaultValue,
		},
	}
}

// processNodeResponseErrorForEventPublish processes the node response error to extract relevant information
// for observability events.
func processNodeResponseErrorForEventPublish(nodeResp *common.NodeResponse) map[string]interface{} {
	if nodeResp == nil || nodeResp.Error == nil {
		return nil
	}

	return map[string]interface{}{
		"code": nodeResp.Error.Code,
		"message": map[string]string{
			"key":          nodeResp.Error.Error.Key,
			"defaultValue": nodeResp.Error.Error.DefaultValue,
		},
		"description": map[string]string{
			"key":          nodeResp.Error.ErrorDescription.Key,
			"defaultValue": nodeResp.Error.ErrorDescription.DefaultValue,
		},
	}
}

// ssoFlowID returns the current flow's ID (used as the SSO group key), or "" if no graph
// is set on the context.
func ssoFlowID(ctx *EngineContext) string {
	if ctx == nil || ctx.Graph == nil {
		return ""
	}
	return ctx.Graph.GetID()
}
