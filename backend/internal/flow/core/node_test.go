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

type NodeTestSuite struct {
	suite.Suite
}

func TestNodeTestSuite(t *testing.T) {
	suite.Run(t, new(NodeTestSuite))
}

func (s *NodeTestSuite) TestExecuteBaseNodeReturnsError() {
	node := newTaskExecutionNode("node-1", nil, false, false)

	resp, err := node.Execute(&NodeContext{ExecutionID: "f1"})

	s.NotNil(err)
	s.Nil(resp)
}

func (s *NodeTestSuite) TestStartAndFinalFlags() {
	node := newPromptNode("p1", nil, false, false)

	s.False(node.IsStartNode())
	s.False(node.IsFinalNode())

	node.SetAsStartNode()
	s.True(node.IsStartNode())

	node.SetAsFinalNode()
	s.True(node.IsFinalNode())
}

func (s *NodeTestSuite) TestNextAndPreviousNodeListBehavior() {
	// next node list behavior
	n := newPromptNode("p1", nil, false, false)

	s.Empty(n.GetNextNodeList())

	n.SetNextNodeList(nil)
	s.Empty(n.GetNextNodeList())

	n.AddNextNode("")
	s.Empty(n.GetNextNodeList())

	n.AddNextNode("n1")
	n.AddNextNode("n1")
	n.AddNextNode("n2")
	s.Len(n.GetNextNodeList(), 2)
	s.Contains(n.GetNextNodeList(), "n1")
	s.Contains(n.GetNextNodeList(), "n2")

	n.RemoveNextNode("n1")
	s.Len(n.GetNextNodeList(), 1)
	s.NotContains(n.GetNextNodeList(), "n1")

	n.RemoveNextNode("")
	n.RemoveNextNode("nope")
	s.Len(n.GetNextNodeList(), 1)

	// previous node list behavior
	p := newPromptNode("p2", nil, false, false)

	s.Empty(p.GetPreviousNodeList())

	p.SetPreviousNodeList(nil)
	s.Empty(p.GetPreviousNodeList())

	p.AddPreviousNode("")
	s.Empty(p.GetPreviousNodeList())

	p.AddPreviousNode("p1")
	p.AddPreviousNode("p2")
	p.AddPreviousNode("p2")
	s.Len(p.GetPreviousNodeList(), 2)
	s.Contains(p.GetPreviousNodeList(), "p1")
	s.Contains(p.GetPreviousNodeList(), "p2")

	p.RemovePreviousNode("p1")
	s.Len(p.GetPreviousNodeList(), 1)
	s.NotContains(p.GetPreviousNodeList(), "p1")

	p.RemovePreviousNode("")
	p.RemovePreviousNode("nope")
	s.Len(p.GetPreviousNodeList(), 1)
}

func (s *NodeTestSuite) TestInputsAndProperties() {
	props := map[string]interface{}{"k": "v"}
	node := newTaskExecutionNode("t1", props, false, false)

	s.Equal(props, node.GetProperties())

	// Cast to ExecutorBackedNodeInterface to access inputs
	execNode, ok := node.(ExecutorBackedNodeInterface)
	s.True(ok)

	inputs := []common.Input{{Identifier: "i1", Required: true}}
	execNode.SetInputs(inputs)
	s.Equal(inputs, execNode.GetInputs())
}
