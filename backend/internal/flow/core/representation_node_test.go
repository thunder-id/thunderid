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

type RepresentationNodeTestSuite struct {
	suite.Suite
}

func TestRepresentationNodeTestSuite(t *testing.T) {
	suite.Run(t, new(RepresentationNodeTestSuite))
}

func (s *RepresentationNodeTestSuite) TestNewRepresentationNode() {
	node := newRepresentationNode("start", common.NodeTypeStart, nil, true, false)

	s.NotNil(node)
	s.Equal("start", node.GetID())
	s.Equal(common.NodeTypeStart, node.GetType())
	s.True(node.IsStartNode())
	s.False(node.IsFinalNode())
}

func (s *RepresentationNodeTestSuite) TestNewRepresentationNodeWithProperties() {
	props := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
	}

	node := newRepresentationNode("end", common.NodeTypeEnd, props, false, true)

	s.NotNil(node)
	s.Equal("end", node.GetID())
	s.Equal(common.NodeTypeEnd, node.GetType())
	s.False(node.IsStartNode())
	s.True(node.IsFinalNode())
	s.Equal(props, node.GetProperties())
}

func (s *RepresentationNodeTestSuite) TestExecuteWithOnSuccess() {
	node := newRepresentationNode("start", common.NodeTypeStart, nil, true, false)
	repNode, ok := node.(RepresentationNodeInterface)
	s.True(ok)
	repNode.SetOnSuccess("next_node")

	ctx := &NodeContext{
		ExecutionID: "test-flow",
	}

	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusComplete, resp.Status)
	s.Equal("next_node", resp.NextNodeID)
	s.NotNil(resp.RuntimeData)
	s.NotNil(resp.AdditionalData)
}

func (s *RepresentationNodeTestSuite) TestExecuteWithoutOnSuccess() {
	node := newRepresentationNode("end", common.NodeTypeEnd, nil, false, true)

	ctx := &NodeContext{
		ExecutionID: "test-flow",
	}

	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusComplete, resp.Status)
	s.Empty(resp.NextNodeID)
	s.NotNil(resp.RuntimeData)
	s.NotNil(resp.AdditionalData)
}

func (s *RepresentationNodeTestSuite) TestGetAndSetOnSuccess() {
	node := newRepresentationNode("test", common.NodeTypeStart, nil, true, false)
	repNode, ok := node.(RepresentationNodeInterface)
	s.True(ok)

	// Initially empty
	s.Empty(repNode.GetOnSuccess())

	// Set onSuccess
	repNode.SetOnSuccess("next_node")
	s.Equal("next_node", repNode.GetOnSuccess())

	// Update onSuccess
	repNode.SetOnSuccess("another_node")
	s.Equal("another_node", repNode.GetOnSuccess())

	// Set empty string
	repNode.SetOnSuccess("")
	s.Empty(repNode.GetOnSuccess())
}

func (s *RepresentationNodeTestSuite) TestShouldExecuteWithoutCondition() {
	node := newRepresentationNode("test", common.NodeTypeStart, nil, true, false)

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		RuntimeData: map[string]string{},
	}

	s.True(node.ShouldExecute(ctx))
}

func (s *RepresentationNodeTestSuite) TestShouldExecuteWithConditionMet() {
	node := newRepresentationNode("test", common.NodeTypeStart, nil, true, false)
	node.SetCondition(&NodeCondition{
		Key:    "{{ context.key1 }}",
		Value:  "value1",
		OnSkip: "skip_node",
	})

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		RuntimeData: map[string]string{
			"key1": "value1",
		},
	}

	s.True(node.ShouldExecute(ctx))
}

func (s *RepresentationNodeTestSuite) TestShouldExecuteWithConditionNotMet() {
	node := newRepresentationNode("test", common.NodeTypeStart, nil, true, false)
	node.SetCondition(&NodeCondition{
		Key:    "{{ context.key1 }}",
		Value:  "value1",
		OnSkip: "skip_node",
	})

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		RuntimeData: map[string]string{
			"key1": "different_value",
		},
	}

	s.False(node.ShouldExecute(ctx))
}

func (s *RepresentationNodeTestSuite) TestStartNodeType() {
	node := newRepresentationNode("start", common.NodeTypeStart, nil, true, false)

	s.Equal(common.NodeTypeStart, node.GetType())
	s.True(node.IsStartNode())
	s.False(node.IsFinalNode())
}

func (s *RepresentationNodeTestSuite) TestEndNodeType() {
	node := newRepresentationNode("end", common.NodeTypeEnd, nil, false, true)

	s.Equal(common.NodeTypeEnd, node.GetType())
	s.False(node.IsStartNode())
	s.True(node.IsFinalNode())
}
