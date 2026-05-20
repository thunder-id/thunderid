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
)

type GraphTestSuite struct {
	suite.Suite
	factory FlowFactoryInterface
	graph   GraphInterface
}

func TestGraphTestSuite(t *testing.T) {
	suite.Run(t, new(GraphTestSuite))
}

func (s *GraphTestSuite) SetupTest() {
	s.factory = newFlowFactory()
	s.graph = s.factory.CreateGraph("test-graph", common.FlowTypeAuthentication)
}

func (s *GraphTestSuite) TestGetID() {
	s.Equal("test-graph", s.graph.GetID())
}

func (s *GraphTestSuite) TestGetType() {
	graph := s.factory.CreateGraph("test-graph", common.FlowTypeRegistration)
	s.Equal(common.FlowTypeRegistration, graph.GetType())
}

func (s *GraphTestSuite) TestAddNodeSuccess() {
	node, _ := s.factory.CreateNode("node-1", string(common.NodeTypeTaskExecution),
		map[string]interface{}{}, true, false)

	err := s.graph.AddNode(node)

	s.NoError(err)
	retrievedNode, exists := s.graph.GetNode("node-1")
	s.True(exists)
	s.Equal("node-1", retrievedNode.GetID())
}

func (s *GraphTestSuite) TestAddNodeNil() {
	err := s.graph.AddNode(nil)
	s.Error(err)
}

func (s *GraphTestSuite) TestGetNode() {
	node, _ := s.factory.CreateNode("node-1", string(common.NodeTypeTaskExecution),
		map[string]interface{}{}, true, false)
	_ = s.graph.AddNode(node)

	tests := []struct {
		name         string
		nodeID       string
		expectExists bool
	}{
		{"Get existing node", "node-1", true},
		{"Get non-existing node", "node-999", false},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			node, exists := s.graph.GetNode(tt.nodeID)
			s.Equal(tt.expectExists, exists)
			if tt.expectExists {
				s.NotNil(node)
				s.Equal(tt.nodeID, node.GetID())
			} else {
				s.Nil(node)
			}
		})
	}
}

func (s *GraphTestSuite) TestAddEdgeSuccess() {
	node1, _ := s.factory.CreateNode("node-1", string(common.NodeTypeTaskExecution),
		map[string]interface{}{}, true, false)
	node2, _ := s.factory.CreateNode("node-2", string(common.NodeTypePrompt),
		map[string]interface{}{}, false, false)
	_ = s.graph.AddNode(node1)
	_ = s.graph.AddNode(node2)

	err := s.graph.AddEdge("node-1", "node-2")

	s.NoError(err)
	edges := s.graph.GetEdges()
	s.Contains(edges["node-1"], "node-2")
}

func (s *GraphTestSuite) TestAddEdgeFailure() {
	node1, _ := s.factory.CreateNode("node-1", string(common.NodeTypeTaskExecution),
		map[string]interface{}{}, true, false)
	node2, _ := s.factory.CreateNode("node-2", string(common.NodeTypePrompt),
		map[string]interface{}{}, false, false)
	_ = s.graph.AddNode(node1)
	_ = s.graph.AddNode(node2)

	tests := []struct {
		name       string
		fromNodeID string
		toNodeID   string
		errorMsg   string
	}{
		{"Empty fromNodeID", "", "node-2", "fromNodeID and toNodeID cannot be empty"},
		{"Empty toNodeID", "node-1", "", "fromNodeID and toNodeID cannot be empty"},
		{"Non-existing fromNode", "node-999", "node-2", "node with fromNodeID does not exist"},
		{"Non-existing toNode", "node-1", "node-999", "node with toNodeID does not exist"},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			err := s.graph.AddEdge(tt.fromNodeID, tt.toNodeID)
			s.Error(err)
			s.Contains(err.Error(), tt.errorMsg)
		})
	}
}

func (s *GraphTestSuite) TestRemoveEdgeSuccess() {
	node1, _ := s.factory.CreateNode("node-1", string(common.NodeTypeTaskExecution),
		map[string]interface{}{}, true, false)
	node2, _ := s.factory.CreateNode("node-2", string(common.NodeTypePrompt),
		map[string]interface{}{}, false, false)
	_ = s.graph.AddNode(node1)
	_ = s.graph.AddNode(node2)
	_ = s.graph.AddEdge("node-1", "node-2")

	err := s.graph.RemoveEdge("node-1", "node-2")

	s.NoError(err)
}

func (s *GraphTestSuite) TestRemoveEdgeFailure() {
	node1, _ := s.factory.CreateNode("node-1", string(common.NodeTypeTaskExecution),
		map[string]interface{}{}, true, false)
	_ = s.graph.AddNode(node1)

	tests := []struct {
		name       string
		fromNodeID string
		toNodeID   string
		errorMsg   string
	}{
		{"Empty fromNodeID", "", "node-2", "fromNodeID and toNodeID cannot be empty"},
		{"Non-existing fromNode", "node-999", "node-2", "node with fromNodeID does not exist"},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			err := s.graph.RemoveEdge(tt.fromNodeID, tt.toNodeID)
			s.Error(err)
			s.Contains(err.Error(), tt.errorMsg)
		})
	}
}

func (s *GraphTestSuite) TestGetNodes() {
	node1, _ := s.factory.CreateNode("node-1", string(common.NodeTypeTaskExecution),
		map[string]interface{}{}, true, false)
	node2, _ := s.factory.CreateNode("node-2", string(common.NodeTypePrompt),
		map[string]interface{}{}, false, false)
	_ = s.graph.AddNode(node1)
	_ = s.graph.AddNode(node2)

	nodes := s.graph.GetNodes()

	s.NotNil(nodes)
	s.Len(nodes, 2)
	s.Contains(nodes, "node-1")
	s.Contains(nodes, "node-2")
}

func (s *GraphTestSuite) TestSetNodes() {
	node1, _ := s.factory.CreateNode("node-1", string(common.NodeTypeTaskExecution),
		map[string]interface{}{}, true, false)
	nodes := map[string]NodeInterface{"node-1": node1}

	s.graph.SetNodes(nodes)

	retrievedNodes := s.graph.GetNodes()
	s.Len(retrievedNodes, 1)
	s.Contains(retrievedNodes, "node-1")

	s.graph.SetNodes(nil)
	retrievedNodes = s.graph.GetNodes()
	s.NotNil(retrievedNodes)
	s.Empty(retrievedNodes)
}

func (s *GraphTestSuite) TestGetEdges() {
	node1, _ := s.factory.CreateNode("node-1", string(common.NodeTypeTaskExecution),
		map[string]interface{}{}, true, false)
	node2, _ := s.factory.CreateNode("node-2", string(common.NodeTypePrompt),
		map[string]interface{}{}, false, false)
	_ = s.graph.AddNode(node1)
	_ = s.graph.AddNode(node2)
	_ = s.graph.AddEdge("node-1", "node-2")

	edges := s.graph.GetEdges()

	s.NotNil(edges)
	s.Contains(edges, "node-1")
	s.Contains(edges["node-1"], "node-2")
}

func (s *GraphTestSuite) TestSetEdges() {
	edges := map[string][]string{"node-1": {"node-2", "node-3"}}

	s.graph.SetEdges(edges)

	retrievedEdges := s.graph.GetEdges()
	s.Len(retrievedEdges, 1)
	s.Len(retrievedEdges["node-1"], 2)

	s.graph.SetEdges(nil)
	retrievedEdges = s.graph.GetEdges()
	s.NotNil(retrievedEdges)
	s.Empty(retrievedEdges)
}

func (s *GraphTestSuite) TestSetStartNodeSuccess() {
	node, _ := s.factory.CreateNode("node-1", string(common.NodeTypeTaskExecution),
		map[string]interface{}{}, false, false)
	_ = s.graph.AddNode(node)

	err := s.graph.SetStartNode("node-1")

	s.NoError(err)
	s.Equal("node-1", s.graph.GetStartNodeID())
	s.True(node.IsStartNode())
}

func (s *GraphTestSuite) TestSetStartNodeFailure() {
	err := s.graph.SetStartNode("node-999")

	s.Error(err)
	s.Contains(err.Error(), "node with startNodeID does not exist")
}

func (s *GraphTestSuite) TestGetStartNodeID() {
	node, _ := s.factory.CreateNode("node-1", string(common.NodeTypeTaskExecution),
		map[string]interface{}{}, false, false)
	_ = s.graph.AddNode(node)
	_ = s.graph.SetStartNode("node-1")

	startNodeID := s.graph.GetStartNodeID()
	s.Equal("node-1", startNodeID)
}

func (s *GraphTestSuite) TestGetStartNodeSuccess() {
	node, _ := s.factory.CreateNode("node-1", string(common.NodeTypeTaskExecution),
		map[string]interface{}{}, false, false)
	_ = s.graph.AddNode(node)
	_ = s.graph.SetStartNode("node-1")

	startNode, err := s.graph.GetStartNode()

	s.NoError(err)
	s.NotNil(startNode)
	s.Equal("node-1", startNode.GetID())
}

func (s *GraphTestSuite) TestGetStartNodeFailure() {
	startNode, err := s.graph.GetStartNode()

	s.Error(err)
	s.Nil(startNode)
	s.Contains(err.Error(), "start node not set for the graph")
}

func (s *GraphTestSuite) TestHasSegments_NoSegments() {
	s.False(s.graph.HasSegments())
}

func (s *GraphTestSuite) TestHasSegments_OneSegmentIsNotMultiple() {
	s.graph.SetSegments([]Segment{{ID: "seg-0", StartNodeID: "node-1"}})
	s.False(s.graph.HasSegments())
}

func (s *GraphTestSuite) TestHasSegments_TwoSegments() {
	s.graph.SetSegments([]Segment{
		{ID: "seg-0", StartNodeID: "node-1"},
		{ID: "seg-1", StartNodeID: "node-2"},
	})
	s.True(s.graph.HasSegments())
}

func (s *GraphTestSuite) TestGetSegments_Empty() {
	segments := s.graph.GetSegments()
	s.Empty(segments)
}

func (s *GraphTestSuite) TestSetAndGetSegments() {
	input := []Segment{
		{ID: "seg-0", StartNodeID: "node-a"},
		{ID: "seg-1", StartNodeID: "node-b"},
	}
	s.graph.SetSegments(input)
	segments := s.graph.GetSegments()
	s.Len(segments, 2)
	s.Equal("seg-0", segments[0].ID)
	s.Equal("node-a", segments[0].StartNodeID)
	s.Equal("seg-1", segments[1].ID)
	s.Equal("node-b", segments[1].StartNodeID)
}

func (s *GraphTestSuite) TestGetSegmentByID_Found() {
	s.graph.SetSegments([]Segment{
		{ID: "seg-0", StartNodeID: "node-a"},
		{ID: "seg-1", StartNodeID: "node-b"},
	})
	seg := s.graph.GetSegmentByID("seg-1")
	s.NotNil(seg)
	s.Equal("seg-1", seg.ID)
	s.Equal("node-b", seg.StartNodeID)
}

func (s *GraphTestSuite) TestGetSegmentByID_NotFound() {
	s.graph.SetSegments([]Segment{
		{ID: "seg-0", StartNodeID: "node-a"},
	})
	seg := s.graph.GetSegmentByID("seg-99")
	s.Nil(seg)
}

func (s *GraphTestSuite) TestGetSegmentByID_EmptyList() {
	seg := s.graph.GetSegmentByID("seg-0")
	s.Nil(seg)
}

func (s *GraphTestSuite) TestGetSegmentByStartNode_Found() {
	s.graph.SetSegments([]Segment{
		{ID: "seg-0", StartNodeID: "node-a"},
		{ID: "seg-1", StartNodeID: "node-b"},
	})
	seg := s.graph.GetSegmentByStartNode("node-b")
	s.NotNil(seg)
	s.Equal("seg-1", seg.ID)
}

func (s *GraphTestSuite) TestGetSegmentByStartNode_NotFound() {
	s.graph.SetSegments([]Segment{
		{ID: "seg-0", StartNodeID: "node-a"},
	})
	seg := s.graph.GetSegmentByStartNode("node-z")
	s.Nil(seg)
}

func (s *GraphTestSuite) TestGetSegmentByStartNode_EmptyList() {
	seg := s.graph.GetSegmentByStartNode("node-a")
	s.Nil(seg)
}

func (s *GraphTestSuite) TestToJSON() {
	node1, _ := s.factory.CreateNode("node-1", string(common.NodeTypeTaskExecution),
		map[string]interface{}{"key": "value"}, true, false)
	node2, _ := s.factory.CreateNode("node-2", string(common.NodeTypePrompt),
		map[string]interface{}{}, false, true)

	if execNode, ok := node1.(ExecutorBackedNodeInterface); ok {
		execNode.SetExecutorName("test-executor")
		execNode.SetInputs([]common.Input{
			{Identifier: "username", Type: "string", Required: true,
				DisplayName: "User Name"},
		})
	}

	_ = s.graph.AddNode(node1)
	_ = s.graph.AddNode(node2)
	_ = s.graph.AddEdge("node-1", "node-2")
	_ = s.graph.SetStartNode("node-1")

	jsonStr, err := s.graph.ToJSON()

	s.NoError(err)
	s.NotEmpty(jsonStr)
	s.Contains(jsonStr, "test-graph")
	s.Contains(jsonStr, "node-1")
	s.Contains(jsonStr, "node-2")
	s.Contains(jsonStr, "test-executor")
	s.Contains(jsonStr, "username")
	s.NotContains(jsonStr, "displayName", "DisplayName must not appear in serialized graph JSON")
	s.NotContains(jsonStr, "User Name", "DisplayName value must not appear in serialized graph JSON")
}
