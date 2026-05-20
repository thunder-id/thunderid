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

package core

import (
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// ExecutorBackedNodeInterface extends NodeInterface for nodes backed by executors.
// Only task execution nodes implement this interface to delegate their execution logic to executors.
type ExecutorBackedNodeInterface interface {
	NodeInterface
	GetExecutorName() string
	SetExecutorName(name string)
	GetExecutor() ExecutorInterface
	SetExecutor(executor ExecutorInterface)
	GetInputs() []common.Input
	SetInputs(inputs []common.Input)
	GetOnSuccess() string
	SetOnSuccess(nodeID string)
	GetOnFailure() string
	SetOnFailure(nodeID string)
	GetOnIncomplete() string
	SetOnIncomplete(nodeID string)
	GetMode() string
	SetMode(mode string)
}

// taskExecutionNode represents a node that executes a task via an executor
type taskExecutionNode struct {
	*node
	executorName string
	executor     ExecutorInterface
	mode         string
	inputs       []common.Input
	onSuccess    string
	onFailure    string
	onIncomplete string
	logger       *log.Logger
}

// Ensure taskExecutionNode implements ExecutorBackedNodeInterface
var _ ExecutorBackedNodeInterface = (*taskExecutionNode)(nil)

// newTaskExecutionNode creates a new TaskExecutionNode with the given details.
func newTaskExecutionNode(id string, properties map[string]interface{}, isStartNode bool,
	isFinalNode bool) NodeInterface {
	return &taskExecutionNode{
		node: &node{
			id:               id,
			_type:            common.NodeTypeTaskExecution,
			properties:       properties,
			isStartNode:      isStartNode,
			isFinalNode:      isFinalNode,
			nextNodeList:     []string{},
			previousNodeList: []string{},
		},
		executorName: "",
		executor:     nil,
		inputs:       []common.Input{},
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "TaskExecutionNode"),
			log.String(log.LoggerKeyNodeID, id)),
	}
}

// Execute executes the node's executor.
func (n *taskExecutionNode) Execute(ctx *NodeContext) (*common.NodeResponse, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Executing task execution node")

	if n.executor == nil {
		logger.Error("No executor configured for the node")
		return nil, &serviceerror.InternalServerError
	}

	// Set node properties in context
	if len(n.GetProperties()) > 0 {
		ctx.NodeProperties = n.GetProperties()
	} else {
		ctx.NodeProperties = make(map[string]interface{})
	}

	// Set executor mode in context
	ctx.ExecutorMode = n.mode

	n.enrichRuntimeData(ctx)

	execResp, svcErr := n.triggerExecutor(ctx, logger)
	if svcErr != nil {
		return nil, svcErr
	}

	nodeResp := n.buildNodeResponse(execResp)

	// Set the next node ID based on execution outcome
	if nodeResp.Status == common.NodeStatusComplete {
		if n.onSuccess != "" {
			nodeResp.NextNodeID = n.onSuccess
		}
	} else if nodeResp.FailureReason != "" && n.onFailure != "" {
		// Change status to Forward so engine forwards execution to onFailure node
		nodeResp.Status = common.NodeStatusForward
		nodeResp.NextNodeID = n.onFailure

		// Store failure reason in RuntimeData so it's available to the onFailure handler
		if nodeResp.RuntimeData == nil {
			nodeResp.RuntimeData = make(map[string]string)
		}
		nodeResp.RuntimeData["failureReason"] = nodeResp.FailureReason

		// Clear user inputs consumed by this executor
		for _, input := range n.inputs {
			delete(ctx.UserInputs, input.Identifier)
		}
	} else if nodeResp.Status == common.NodeStatusIncomplete && n.onIncomplete != "" {
		// Executor requires user input - forward to dedicated prompt node
		// Change status to Forward so engine forwards execution to onIncomplete node
		nodeResp.Status = common.NodeStatusForward
		nodeResp.NextNodeID = n.onIncomplete

		// Propagate failure reason if present
		if nodeResp.FailureReason != "" {
			if nodeResp.RuntimeData == nil {
				nodeResp.RuntimeData = make(map[string]string)
			}
			nodeResp.RuntimeData["failureReason"] = nodeResp.FailureReason

			// Clear user inputs consumed by this executor
			for _, input := range n.inputs {
				delete(ctx.UserInputs, input.Identifier)
			}
		}
	} else if nodeResp.Status == common.NodeStatusIncomplete && nodeResp.Type == common.NodeResponseTypeView &&
		len(nodeResp.Inputs) == 0 {
		// Executor returned INCOMPLETE+VIEW with no inputs — broken executor implementation.
		// There is nothing for the client to act on; surface as a server error.
		logger.Error("Executor returned INCOMPLETE with VIEW type but no inputs")
		return nil, &serviceerror.InternalServerError
	}

	return nodeResp, nil
}

// enrichRuntimeData initializes the runtime data map and attaches identifiers like application, IDP,
// and sender IDs so downstream executors and placeholders can use them.
func (n *taskExecutionNode) enrichRuntimeData(ctx *NodeContext) {
	if ctx.RuntimeData == nil {
		ctx.RuntimeData = make(map[string]string)
	}

	if ctx.EntityID != "" {
		ctx.RuntimeData["applicationId"] = ctx.EntityID
	}

	if idpID, ok := ctx.NodeProperties["idpId"].(string); ok && idpID != "" {
		ctx.RuntimeData["idpId"] = idpID
	}

	if senderID, ok := ctx.NodeProperties["senderId"].(string); ok && senderID != "" {
		ctx.RuntimeData["senderId"] = senderID
	}
}

// triggerExecutor triggers the executor configured for the node.
func (n *taskExecutionNode) triggerExecutor(ctx *NodeContext, logger *log.Logger) (
	*common.ExecutorResponse, *serviceerror.ServiceError) {
	execResp, err := n.executor.Execute(ctx)
	if err != nil {
		logger.Error("Error executing node executor", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	if execResp == nil {
		logger.Error("Executor returned a nil response")
		return nil, &serviceerror.InternalServerError
	}

	return execResp, nil
}

// buildNodeResponse constructs a NodeResponse from the ExecutorResponse.
func (n *taskExecutionNode) buildNodeResponse(execResp *common.ExecutorResponse) *common.NodeResponse {
	nodeResp := &common.NodeResponse{
		FailureReason:     execResp.FailureReason,
		Inputs:            execResp.Inputs,
		AdditionalData:    execResp.AdditionalData,
		RedirectURL:       execResp.RedirectURL,
		RuntimeData:       execResp.RuntimeData,
		ForwardedData:     execResp.ForwardedData,
		AuthenticatedUser: execResp.AuthenticatedUser,
		Assertion:         execResp.Assertion,
		AuthUser:          execResp.AuthUser,
	}
	if nodeResp.AdditionalData == nil {
		nodeResp.AdditionalData = make(map[string]string)
	}
	if nodeResp.RuntimeData == nil {
		nodeResp.RuntimeData = make(map[string]string)
	}
	if nodeResp.ForwardedData == nil {
		nodeResp.ForwardedData = make(map[string]interface{})
	}
	if nodeResp.Inputs == nil {
		nodeResp.Inputs = make([]common.Input, 0)
	}
	if nodeResp.Actions == nil {
		nodeResp.Actions = make([]common.Action, 0)
	}

	switch execResp.Status {
	case common.ExecComplete:
		nodeResp.Status = common.NodeStatusComplete
		nodeResp.Type = ""
	case common.ExecUserInputRequired:
		nodeResp.Status = common.NodeStatusIncomplete
		nodeResp.Type = common.NodeResponseTypeView
	case common.ExecExternalRedirection:
		nodeResp.Status = common.NodeStatusIncomplete
		nodeResp.Type = common.NodeResponseTypeRedirection
	case common.ExecRetry:
		nodeResp.Status = common.NodeStatusIncomplete
		nodeResp.Type = common.NodeResponseTypeRetry
	case common.ExecFailure:
		nodeResp.Status = common.NodeStatusFailure
		nodeResp.Type = ""
	default:
		nodeResp.Status = common.NodeStatusIncomplete
		nodeResp.Type = ""
	}

	return nodeResp
}

// GetExecutionPolicy returns the execution policy for the current node by delegating to the
// configured executor with the node's mode. Returns nil if no executor is set.
func (n *taskExecutionNode) GetExecutionPolicy() *ExecutionPolicy {
	if n.executor == nil {
		return nil
	}
	return n.executor.GetExecutionPolicy(n.mode)
}

// GetExecutorName returns the executor name for the task execution node
func (n *taskExecutionNode) GetExecutorName() string {
	return n.executorName
}

// SetExecutorName sets the executor name for the task execution node
func (n *taskExecutionNode) SetExecutorName(name string) {
	n.executorName = name
}

// GetExecutor returns the executor instance associated with the task execution node
func (n *taskExecutionNode) GetExecutor() ExecutorInterface {
	return n.executor
}

// SetExecutor sets the executor instance for the task execution node
func (n *taskExecutionNode) SetExecutor(executor ExecutorInterface) {
	n.executor = executor
	if executor != nil {
		n.executorName = executor.GetName()
	}
}

// GetOnSuccess returns the onSuccess node ID
func (n *taskExecutionNode) GetOnSuccess() string {
	return n.onSuccess
}

// SetOnSuccess sets the onSuccess node ID
func (n *taskExecutionNode) SetOnSuccess(nodeID string) {
	n.onSuccess = nodeID
}

// GetOnFailure returns the onFailure node ID
func (n *taskExecutionNode) GetOnFailure() string {
	return n.onFailure
}

// SetOnFailure sets the onFailure node ID
func (n *taskExecutionNode) SetOnFailure(nodeID string) {
	n.onFailure = nodeID
}

// GetOnIncomplete returns the onIncomplete node ID
func (n *taskExecutionNode) GetOnIncomplete() string {
	return n.onIncomplete
}

// SetOnIncomplete sets the onIncomplete node ID
func (n *taskExecutionNode) SetOnIncomplete(nodeID string) {
	n.onIncomplete = nodeID
}

// GetMode returns the mode for the executor that supports multi-step execution
func (n *taskExecutionNode) GetMode() string {
	return n.mode
}

// SetMode sets the mode for the executor that supports multi-step execution
func (n *taskExecutionNode) SetMode(mode string) {
	n.mode = mode
}

// GetInputs returns the inputs required for the task execution node
func (n *taskExecutionNode) GetInputs() []common.Input {
	return n.inputs
}

// SetInputs sets the inputs required for the task execution node
func (n *taskExecutionNode) SetInputs(inputs []common.Input) {
	n.inputs = inputs
}
