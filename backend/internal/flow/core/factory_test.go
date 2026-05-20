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
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

type FlowFactoryTestSuite struct {
	suite.Suite
	factory FlowFactoryInterface
}

func TestFlowFactoryTestSuite(t *testing.T) {
	suite.Run(t, new(FlowFactoryTestSuite))
}

func (s *FlowFactoryTestSuite) SetupTest() {
	s.factory = newFlowFactory()
}

func (s *FlowFactoryTestSuite) TestNewFlowFactory() {
	s.NotNil(s.factory)
}

func (s *FlowFactoryTestSuite) TestCreateNodeSuccess() {
	tests := []struct {
		name         string
		nodeID       string
		nodeType     string
		properties   map[string]interface{}
		isStartNode  bool
		isFinalNode  bool
		expectedType common.NodeType
	}{
		{"Create task execution node", "node-1", string(common.NodeTypeTaskExecution),
			map[string]interface{}{"key": "value"}, true, false, common.NodeTypeTaskExecution},
		{"Create prompt only node", "node-3", string(common.NodeTypePrompt),
			nil, false, true, common.NodeTypePrompt},
		{"Create start node", "node-4", string(common.NodeTypeStart),
			map[string]interface{}{}, true, false, common.NodeTypeStart},
		{"Create end node", "node-5", string(common.NodeTypeEnd),
			map[string]interface{}{}, false, true, common.NodeTypeEnd},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			node, err := s.factory.CreateNode(tt.nodeID, tt.nodeType, tt.properties, tt.isStartNode,
				tt.isFinalNode)

			s.NoError(err)
			s.NotNil(node)
			s.Equal(tt.nodeID, node.GetID())
			s.Equal(tt.expectedType, node.GetType())
			s.Equal(tt.isStartNode, node.IsStartNode())
			s.Equal(tt.isFinalNode, node.IsFinalNode())
			if tt.properties != nil {
				s.NotNil(node.GetProperties())
			}
		})
	}
}

func (s *FlowFactoryTestSuite) TestCreateNodeFailure() {
	tests := []struct {
		name     string
		nodeType string
	}{
		{"Empty node type", ""},
		{"Unsupported node type", "UNSUPPORTED_TYPE"},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			node, err := s.factory.CreateNode("node-1", tt.nodeType, map[string]interface{}{}, false, false)
			s.Error(err)
			s.Nil(node)
		})
	}
}

func (s *FlowFactoryTestSuite) TestCreateStartNodeWithOnSuccess() {
	node, err := s.factory.CreateNode("start", string(common.NodeTypeStart),
		map[string]interface{}{}, true, false)

	s.NoError(err)
	s.NotNil(node)
	s.Equal(common.NodeTypeStart, node.GetType())
	s.True(node.IsStartNode())

	// Test that we can set and get onSuccess (START is a representation node)
	if repNode, ok := node.(RepresentationNodeInterface); ok {
		repNode.SetOnSuccess("next-node")
		s.Equal("next-node", repNode.GetOnSuccess())
	} else {
		s.Fail("Node should implement RepresentationNodeInterface")
	}
}

func (s *FlowFactoryTestSuite) TestCreateEndNodeWithOnSuccess() {
	node, err := s.factory.CreateNode("end", string(common.NodeTypeEnd),
		map[string]interface{}{}, false, true)

	s.NoError(err)
	s.NotNil(node)
	s.Equal(common.NodeTypeEnd, node.GetType())
	s.True(node.IsFinalNode())

	// Test that we can set and get onSuccess (END is a representation node)
	if repNode, ok := node.(RepresentationNodeInterface); ok {
		repNode.SetOnSuccess("should-be-empty")
		s.Equal("should-be-empty", repNode.GetOnSuccess())
	} else {
		s.Fail("Node should implement RepresentationNodeInterface")
	}
}

func (s *FlowFactoryTestSuite) TestCreateGraph() {
	tests := []struct {
		name         string
		graphID      string
		flowType     common.FlowType
		expectUUID   bool
		expectedType common.FlowType
	}{
		{"Create graph with ID and type", "graph-1", common.FlowTypeAuthentication,
			false, common.FlowTypeAuthentication},
		{"Create graph with registration flow type", "graph-2", common.FlowTypeRegistration,
			false, common.FlowTypeRegistration},
		{"Empty graph ID generates UUID", "", common.FlowTypeAuthentication,
			true, common.FlowTypeAuthentication},
		{"Empty flow type defaults to authentication", "graph-3", "",
			false, common.FlowTypeAuthentication},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			graph := s.factory.CreateGraph(tt.graphID, tt.flowType)

			s.NotNil(graph)
			if tt.expectUUID {
				s.NotEmpty(graph.GetID())
			} else {
				s.Equal(tt.graphID, graph.GetID())
			}
			s.Equal(tt.expectedType, graph.GetType())
			s.NotNil(graph.GetNodes())
			s.NotNil(graph.GetEdges())
		})
	}
}

func (s *FlowFactoryTestSuite) TestCreateExecutor() {
	defaultInputs := []common.Input{{Identifier: "input1", Required: true}}
	prerequisites := []common.Input{{Identifier: "prereq1", Required: true}}

	executor := s.factory.CreateExecutor("test-executor", common.ExecutorTypeAuthentication,
		defaultInputs, prerequisites)

	s.NotNil(executor)
	s.Equal("test-executor", executor.GetName())
	s.Equal(common.ExecutorTypeAuthentication, executor.GetType())
	s.Equal(defaultInputs, executor.GetDefaultInputs())
	s.Equal(prerequisites, executor.GetPrerequisites())
}

func (s *FlowFactoryTestSuite) TestCloneNodeSuccess() {
	node, _ := s.factory.CreateNode("node-1", string(common.NodeTypeTaskExecution),
		map[string]interface{}{"key": "value"}, true, false)
	node.AddNextNode("next-1")
	node.AddPreviousNode("prev-1")
	if execNode, ok := node.(ExecutorBackedNodeInterface); ok {
		execNode.SetExecutorName("test-executor")
		execNode.SetInputs([]common.Input{{Identifier: "input1", Required: true}})
	}

	clonedNode, err := s.factory.CloneNode(node)

	s.NoError(err)
	s.NotNil(clonedNode)
	s.Equal(node.GetID(), clonedNode.GetID())
	s.Equal(node.GetType(), clonedNode.GetType())
	s.Equal(node.IsStartNode(), clonedNode.IsStartNode())
	s.Equal(node.IsFinalNode(), clonedNode.IsFinalNode())
	s.Equal(node.GetNextNodeList(), clonedNode.GetNextNodeList())
	s.Equal(node.GetPreviousNodeList(), clonedNode.GetPreviousNodeList())

	if sourceExecNode, ok := node.(ExecutorBackedNodeInterface); ok {
		if clonedExecNode, ok := clonedNode.(ExecutorBackedNodeInterface); ok {
			s.Equal(sourceExecNode.GetExecutorName(), clonedExecNode.GetExecutorName())
			s.Len(clonedExecNode.GetInputs(), len(sourceExecNode.GetInputs()))
		}
	}

	clonedNode.AddNextNode("new-next")
	s.NotEqual(len(node.GetNextNodeList()), len(clonedNode.GetNextNodeList()))
}

func (s *FlowFactoryTestSuite) TestCloneNodeNil() {
	clonedNode, err := s.factory.CloneNode(nil)

	s.Error(err)
	s.Nil(clonedNode)
	s.Contains(err.Error(), "source node cannot be nil")
}

func (s *FlowFactoryTestSuite) TestCloneNodeWithCondition() {
	node, _ := s.factory.CreateNode("node-1", string(common.NodeTypeTaskExecution),
		map[string]interface{}{}, false, false)
	node.SetCondition(&NodeCondition{
		Key:   "{{ context.status }}",
		Value: "active",
	})

	clonedNode, err := s.factory.CloneNode(node)

	s.NoError(err)
	s.NotNil(clonedNode)
	s.NotNil(clonedNode.GetCondition())
	s.Equal(node.GetCondition().Key, clonedNode.GetCondition().Key)
	s.Equal(node.GetCondition().Value, clonedNode.GetCondition().Value)

	// Verify deep copy - modifying cloned condition doesn't affect source
	clonedNode.SetCondition(&NodeCondition{
		Key:   "{{ context.newKey }}",
		Value: "newValue",
	})
	s.NotEqual(node.GetCondition().Key, clonedNode.GetCondition().Key)
}

func (s *FlowFactoryTestSuite) TestCloneStartNodeWithOnSuccess() {
	node, _ := s.factory.CreateNode("start", string(common.NodeTypeStart),
		map[string]interface{}{}, true, false)

	repNode, ok := node.(RepresentationNodeInterface)
	s.True(ok, "Node should implement RepresentationNodeInterface")
	repNode.SetOnSuccess("next-node")

	clonedNode, err := s.factory.CloneNode(node)

	s.NoError(err)
	s.NotNil(clonedNode)

	clonedRepNode, ok := clonedNode.(RepresentationNodeInterface)
	s.True(ok, "Cloned node should implement RepresentationNodeInterface")
	s.Equal(repNode.GetOnSuccess(), clonedRepNode.GetOnSuccess())
	s.Equal("next-node", clonedRepNode.GetOnSuccess())

	// Verify deep copy - modifying cloned onSuccess doesn't affect source
	clonedRepNode.SetOnSuccess("different-node")
	s.Equal("next-node", repNode.GetOnSuccess())
	s.Equal("different-node", clonedRepNode.GetOnSuccess())
}

func (s *FlowFactoryTestSuite) TestCloneTaskExecutionNodeWithOnSuccess() {
	node, _ := s.factory.CreateNode("task", string(common.NodeTypeTaskExecution),
		map[string]interface{}{}, false, false)

	if execNode, ok := node.(ExecutorBackedNodeInterface); ok {
		execNode.SetOnSuccess("success-node")
		execNode.SetOnFailure("failure-node")
		execNode.SetExecutorName("test-executor")
	}

	clonedNode, err := s.factory.CloneNode(node)

	s.NoError(err)
	s.NotNil(clonedNode)

	if clonedExecNode, ok := clonedNode.(ExecutorBackedNodeInterface); ok {
		s.Equal("success-node", clonedExecNode.GetOnSuccess())
		s.Equal("failure-node", clonedExecNode.GetOnFailure())
		s.Equal("test-executor", clonedExecNode.GetExecutorName())
	} else {
		s.Fail("Cloned node should be ExecutorBackedNodeInterface")
	}
}

func (s *FlowFactoryTestSuite) TestCloneNodeWithMeta() {
	promptNode, _ := s.factory.CreateNode("prompt-1", string(common.NodeTypePrompt),
		map[string]interface{}{}, false, false)
	promptMeta := map[string]interface{}{"components": []string{"input1", "button1"}}

	// Type assert to PromptNodeInterface to access SetMeta
	if pn, ok := promptNode.(PromptNodeInterface); ok {
		pn.SetMeta(promptMeta)
	} else {
		s.Fail("Prompt node should implement PromptNodeInterface")
	}

	clonedPromptNode, err := s.factory.CloneNode(promptNode)

	s.NoError(err)
	s.NotNil(clonedPromptNode)

	// Type assert cloned node to verify meta was copied
	if cpn, ok := clonedPromptNode.(PromptNodeInterface); ok {
		s.Equal(promptMeta, cpn.GetMeta())

		// Verify deep-copy: mutating the cloned meta should not affect the source
		clonedMeta := cpn.GetMeta().(map[string]interface{})
		clonedMeta["extra"] = "added"
		s.NotEqual(promptMeta, clonedMeta, "Mutated clone meta should differ from source")
		_, exists := promptMeta["extra"]
		s.False(exists, "Source meta should not be affected by mutations to cloned meta")
	} else {
		s.Fail("Cloned prompt node should implement PromptNodeInterface")
	}

	// Verify that task nodes do not have meta
	taskNode, _ := s.factory.CreateNode("task-1", string(common.NodeTypeTaskExecution),
		map[string]interface{}{}, false, false)
	clonedTaskNode, err := s.factory.CloneNode(taskNode)
	s.NoError(err)
	s.NotNil(clonedTaskNode)
	// Should not be able to type assert to PromptNodeInterface
	_, isPromptNode := clonedTaskNode.(PromptNodeInterface)
	s.False(isPromptNode, "Task node should not implement PromptNodeInterface")
}

func (s *FlowFactoryTestSuite) TestCloneNodesSuccess() {
	nodes := make(map[string]NodeInterface)
	node1, _ := s.factory.CreateNode("node-1", string(common.NodeTypeTaskExecution),
		map[string]interface{}{}, true, false)
	node2, _ := s.factory.CreateNode("node-2", string(common.NodeTypePrompt),
		map[string]interface{}{}, false, false)
	nodes["node-1"] = node1
	nodes["node-2"] = node2

	clonedNodes, err := s.factory.CloneNodes(nodes)

	s.NoError(err)
	s.NotNil(clonedNodes)
	s.Len(clonedNodes, len(nodes))

	for id, sourceNode := range nodes {
		clonedNode, exists := clonedNodes[id]
		s.True(exists)
		s.Equal(sourceNode.GetID(), clonedNode.GetID())
		s.Equal(sourceNode.GetType(), clonedNode.GetType())
	}
}

func (s *FlowFactoryTestSuite) TestCloneNodesNilOrEmpty() {
	clonedNodes, err := s.factory.CloneNodes(nil)
	s.NoError(err)
	s.Nil(clonedNodes)

	clonedNodes, err = s.factory.CloneNodes(make(map[string]NodeInterface))
	s.NoError(err)
	s.NotNil(clonedNodes)
	s.Empty(clonedNodes)
}

// fakeExecutorBackedNode implements ExecutorBackedNodeInterface but will report a
// NodeType that CreateNode maps to a non-executor-backed node. This allows
// exercising the defensive mismatch branch in CloneNode.
type fakeExecutorBackedNode struct {
	id string
}

func (f *fakeExecutorBackedNode) Execute(ctx *NodeContext) (*common.NodeResponse, *serviceerror.ServiceError) {
	return nil, nil
}

func (f *fakeExecutorBackedNode) GetID() string {
	return f.id
}

func (f *fakeExecutorBackedNode) GetType() common.NodeType {
	return common.NodeTypePrompt
}

func (f *fakeExecutorBackedNode) GetProperties() map[string]interface{} {
	return nil
}

func (f *fakeExecutorBackedNode) IsStartNode() bool {
	return false
}

func (f *fakeExecutorBackedNode) SetAsStartNode() {}

func (f *fakeExecutorBackedNode) IsFinalNode() bool {
	return false
}

func (f *fakeExecutorBackedNode) SetAsFinalNode() {}

func (f *fakeExecutorBackedNode) GetNextNodeList() []string {
	return []string{}
}

func (f *fakeExecutorBackedNode) SetNextNodeList(nextNodeIDList []string) {}

func (f *fakeExecutorBackedNode) AddNextNode(nextNodeID string) {}

func (f *fakeExecutorBackedNode) RemoveNextNode(nextNodeID string) {}

func (f *fakeExecutorBackedNode) GetPreviousNodeList() []string {
	return []string{}
}

func (f *fakeExecutorBackedNode) SetPreviousNodeList(previousNodeIDList []string) {}

func (f *fakeExecutorBackedNode) AddPreviousNode(previousNodeID string) {}

func (f *fakeExecutorBackedNode) RemovePreviousNode(previousNodeID string) {}

func (f *fakeExecutorBackedNode) GetInputs() []common.Input {
	return nil
}

func (f *fakeExecutorBackedNode) SetInputs(inputs []common.Input) {}

func (f *fakeExecutorBackedNode) GetCondition() *NodeCondition {
	return nil
}

func (f *fakeExecutorBackedNode) SetCondition(condition *NodeCondition) {}

func (f *fakeExecutorBackedNode) ShouldExecute(ctx *NodeContext) bool {
	return true
}

func (f *fakeExecutorBackedNode) GetExecutionPolicy() *ExecutionPolicy {
	return nil
}

func (f *fakeExecutorBackedNode) GetExecutorName() string {
	return "fake-exec"
}

func (f *fakeExecutorBackedNode) SetExecutorName(name string) {}

func (f *fakeExecutorBackedNode) GetExecutor() ExecutorInterface {
	return nil
}

func (f *fakeExecutorBackedNode) SetExecutor(executor ExecutorInterface) {}

func (f *fakeExecutorBackedNode) GetOnSuccess() string {
	return ""
}

func (f *fakeExecutorBackedNode) SetOnSuccess(nodeID string) {}

func (f *fakeExecutorBackedNode) GetOnFailure() string {
	return ""
}

func (f *fakeExecutorBackedNode) SetOnFailure(nodeID string) {}

func (f *fakeExecutorBackedNode) GetOnIncomplete() string {
	return ""
}

func (f *fakeExecutorBackedNode) SetOnIncomplete(nodeID string) {}

func (f *fakeExecutorBackedNode) GetMode() string {
	return ""
}

func (f *fakeExecutorBackedNode) SetMode(mode string) {}

func (s *FlowFactoryTestSuite) TestCloneNodeMismatchExecutorBacked() {
	// source claims to be executor-backed but GetType returns Prompt which
	// CreateNode maps to a non-executor-backed node. This should trigger the
	// mismatch error in CloneNode.
	src := &fakeExecutorBackedNode{id: "fake-1"}

	cloned, err := s.factory.CloneNode(src)

	s.Error(err)
	s.Nil(cloned)
	s.Contains(err.Error(), "mismatch in node types during cloning. copy is not executor-backed")
}
