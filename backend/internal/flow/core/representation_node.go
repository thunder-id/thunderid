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
)

// RepresentationNodeInterface extends NodeInterface for representation nodes (START/END).
// These nodes use simple onSuccess navigation for linear flow.
type RepresentationNodeInterface interface {
	NodeInterface
	GetOnSuccess() string
	SetOnSuccess(nodeID string)
}

// representationNode implements the RepresentationNodeInterface
type representationNode struct {
	*node
	onSuccess string
}

// Ensure representationNode implements RepresentationNodeInterface
var _ RepresentationNodeInterface = (*representationNode)(nil)

// newRepresentationNode creates a new representation node
func newRepresentationNode(id string, nodeType common.NodeType, properties map[string]interface{},
	isStartNode bool, isFinalNode bool) NodeInterface {
	if properties == nil {
		properties = make(map[string]interface{})
	}
	return &representationNode{
		node: &node{
			id:               id,
			_type:            nodeType,
			properties:       properties,
			isStartNode:      isStartNode,
			isFinalNode:      isFinalNode,
			nextNodeList:     []string{},
			previousNodeList: []string{},
		},
		onSuccess: "",
	}
}

// Execute executes representation nodes with simple onSuccess navigation
func (n *representationNode) Execute(ctx *NodeContext) (*common.NodeResponse, *serviceerror.ServiceError) {
	response := &common.NodeResponse{
		Status:         common.NodeStatusComplete,
		RuntimeData:    make(map[string]string),
		AdditionalData: make(map[string]string),
	}

	// Set next node using onSuccess property
	if n.onSuccess != "" {
		response.NextNodeID = n.onSuccess
	}

	return response, nil
}

// GetOnSuccess returns the onSuccess node ID for representation nodes
func (n *representationNode) GetOnSuccess() string {
	return n.onSuccess
}

// SetOnSuccess sets the onSuccess node ID for representation nodes
func (n *representationNode) SetOnSuccess(nodeID string) {
	n.onSuccess = nodeID
}
