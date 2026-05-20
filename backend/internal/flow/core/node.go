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

// NodeInterface defines the interface for nodes in the graph
type NodeInterface interface {
	Execute(ctx *NodeContext) (*common.NodeResponse, *serviceerror.ServiceError)
	ShouldExecute(ctx *NodeContext) bool
	GetID() string
	GetType() common.NodeType
	GetProperties() map[string]interface{}
	IsStartNode() bool
	SetAsStartNode()
	IsFinalNode() bool
	SetAsFinalNode()
	GetNextNodeList() []string
	SetNextNodeList(nextNodeIDList []string)
	AddNextNode(nextNodeID string)
	RemoveNextNode(nextNodeID string)
	GetPreviousNodeList() []string
	SetPreviousNodeList(previousNodeIDList []string)
	AddPreviousNode(previousNodeID string)
	RemovePreviousNode(previousNodeID string)
	GetCondition() *NodeCondition
	SetCondition(condition *NodeCondition)
	GetExecutionPolicy() *ExecutionPolicy
}

// node implements the NodeInterface
type node struct {
	id               string
	_type            common.NodeType
	properties       map[string]interface{}
	isStartNode      bool
	isFinalNode      bool
	nextNodeList     []string
	previousNodeList []string
	condition        *NodeCondition
}

var _ NodeInterface = (*node)(nil)

// Execute is a default implementation that should be overridden by specific node types
func (n *node) Execute(ctx *NodeContext) (*common.NodeResponse, *serviceerror.ServiceError) {
	return nil, nil
}

// ShouldExecute checks if the node's condition is satisfied and the node should execute.
// Returns true if no condition is set or if the condition is met.
func (n *node) ShouldExecute(ctx *NodeContext) bool {
	if n.condition == nil {
		return true
	}

	resolvedKey := ResolvePlaceholder(ctx, n.condition.Key)
	return resolvedKey == n.condition.Value
}

// GetID returns the node's ID
func (n *node) GetID() string {
	return n.id
}

// GetType returns the node's type
func (n *node) GetType() common.NodeType {
	return n._type
}

// GetProperties returns the node's properties
func (n *node) GetProperties() map[string]interface{} {
	return n.properties
}

// IsStartNode checks if the node is a start node
func (n *node) IsStartNode() bool {
	return n.isStartNode
}

// SetAsStartNode sets the node as a start node
func (n *node) SetAsStartNode() {
	n.isStartNode = true
}

// IsFinalNode checks if the node is a final node
func (n *node) IsFinalNode() bool {
	return n.isFinalNode
}

// SetAsFinalNode sets the node as a final node
func (n *node) SetAsFinalNode() {
	n.isFinalNode = true
}

// GetNextNodeList returns the list of next node IDs
func (n *node) GetNextNodeList() []string {
	if n.nextNodeList == nil {
		return []string{}
	}
	return n.nextNodeList
}

// SetNextNodeList sets the list of next node IDs
func (n *node) SetNextNodeList(nextNodeIDList []string) {
	if nextNodeIDList == nil {
		n.nextNodeList = []string{}
	} else {
		n.nextNodeList = nextNodeIDList
	}
}

// AddNextNode adds a next node ID to the list
func (n *node) AddNextNode(nextNodeID string) {
	if nextNodeID == "" {
		return
	}
	if n.nextNodeList == nil {
		n.nextNodeList = []string{}
	}
	// Check for duplicates before adding
	for _, id := range n.nextNodeList {
		if id == nextNodeID {
			return
		}
	}
	n.nextNodeList = append(n.nextNodeList, nextNodeID)
}

// RemoveNextNode removes a next node ID from the list
func (n *node) RemoveNextNode(nextNodeID string) {
	if nextNodeID == "" || n.nextNodeList == nil {
		return
	}

	for i, id := range n.nextNodeList {
		if id == nextNodeID {
			n.nextNodeList = append(n.nextNodeList[:i], n.nextNodeList[i+1:]...)
			return
		}
	}
}

// GetPreviousNodeList returns the list of previous node IDs
func (n *node) GetPreviousNodeList() []string {
	if n.previousNodeList == nil {
		return []string{}
	}
	return n.previousNodeList
}

// SetPreviousNodeList sets the list of previous node IDs
func (n *node) SetPreviousNodeList(previousNodeIDList []string) {
	if previousNodeIDList == nil {
		n.previousNodeList = []string{}
	} else {
		n.previousNodeList = previousNodeIDList
	}
}

// AddPreviousNode adds a previous node ID to the list
func (n *node) AddPreviousNode(previousNodeID string) {
	if previousNodeID == "" {
		return
	}
	if n.previousNodeList == nil {
		n.previousNodeList = []string{}
	}
	// Check for duplicates before adding
	for _, id := range n.previousNodeList {
		if id == previousNodeID {
			return
		}
	}
	n.previousNodeList = append(n.previousNodeList, previousNodeID)
}

// RemovePreviousNode removes a previous node ID from the list
func (n *node) RemovePreviousNode(previousNodeID string) {
	if previousNodeID == "" || n.previousNodeList == nil {
		return
	}

	for i, id := range n.previousNodeList {
		if id == previousNodeID {
			n.previousNodeList = append(n.previousNodeList[:i], n.previousNodeList[i+1:]...)
			return
		}
	}
}

// GetCondition returns the execution condition for the node
func (n *node) GetCondition() *NodeCondition {
	return n.condition
}

// SetCondition sets the execution condition for the node
func (n *node) SetCondition(condition *NodeCondition) {
	n.condition = condition
}

// GetExecutionPolicy returns the execution policy for the node. By default, it returns nil, indicating
// no special execution policy. Nodes that need special execution behavior should override this method.
func (n *node) GetExecutionPolicy() *ExecutionPolicy {
	return nil
}
