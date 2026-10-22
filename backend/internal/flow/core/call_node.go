// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"strings"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// CallNodeInterface extends NodeInterface for CALL nodes, which transfer execution to
// a referenced flow and return control to the caller when the callee's END node is reached.
type CallNodeInterface interface {
	NodeInterface
	GetReferencedFlow() string
	SetReferencedFlow(flowID string)
	GetOnSuccess() string
	SetOnSuccess(nodeID string)
	GetOnFailure() string
	SetOnFailure(nodeID string)
}

// callNode implements CallNodeInterface and represents a CALL node in the flow graph.
type callNode struct {
	*node
	referencedFlow string
	onSuccess      string
	onFailure      string
	logger         *log.Logger
}

var _ CallNodeInterface = (*callNode)(nil)

// newCallNode creates a new instance of callNode with the given parameters.
func newCallNode(id string, properties map[string]interface{}, isStartNode, isFinalNode bool) NodeInterface {
	if properties == nil {
		properties = make(map[string]interface{})
	}
	return &callNode{
		node: &node{
			id:               id,
			_type:            common.NodeTypeCall,
			properties:       properties,
			isStartNode:      isStartNode,
			isFinalNode:      isFinalNode,
			nextNodeList:     []string{},
			previousNodeList: []string{},
		},
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CallNode"),
			log.String(log.LoggerKeyNodeID, id)),
	}
}

// Execute executes the CALL node logic, transferring control to the referenced flow.
func (n *callNode) Execute(ctx *providers.NodeContext) (*common.NodeResponse, *tidcommon.ServiceError) {
	if n.referencedFlow == "" {
		n.logger.Error(ctx.Context, "Referenced flow ID is not set for CALL node")
		return nil, &tidcommon.InternalServerError
	}
	targetFlowID := n.referencedFlow
	targetFlowType := providers.FlowType("")
	if flowType, dynamic := strings.CutPrefix(targetFlowID, common.FlowTypeReferencePrefix); dynamic {
		targetFlowType = providers.FlowType(flowType)
		targetFlowID = ctx.Application.FlowID(targetFlowType)
		if targetFlowID == "" {
			n.logger.Error(ctx.Context, "Application flow is not configured for CALL node",
				log.String("flowType", flowType))
			return nil, &tidcommon.InternalServerError
		}
	}
	return &common.NodeResponse{
		Status:             common.NodeStatusCall,
		CallTargetFlowID:   targetFlowID,
		CallTargetFlowType: targetFlowType,
	}, nil
}

// GetReferencedFlow returns the ID of the flow referenced by this CALL node.
func (n *callNode) GetReferencedFlow() string {
	return n.referencedFlow
}

// SetReferencedFlow sets the ID of the flow to be referenced by this CALL node.
func (n *callNode) SetReferencedFlow(flowID string) {
	n.referencedFlow = flowID
}

// GetOnSuccess returns the ID of the node to transition to upon successful completion of the referenced flow.
func (n *callNode) GetOnSuccess() string {
	return n.onSuccess
}

// SetOnSuccess sets the ID of the node to transition to upon successful completion of the referenced flow.
func (n *callNode) SetOnSuccess(nodeID string) {
	n.onSuccess = nodeID
}

// GetOnFailure returns the ID of the node to transition to upon failure of the referenced flow.
func (n *callNode) GetOnFailure() string {
	return n.onFailure
}

// SetOnFailure sets the ID of the node to transition to upon failure of the referenced flow.
func (n *callNode) SetOnFailure(nodeID string) {
	n.onFailure = nodeID
}
