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

type NodeInterfaceTestSuite struct {
	suite.Suite
}

func TestNodeInterfaceTestSuite(t *testing.T) {
	suite.Run(t, new(NodeInterfaceTestSuite))
}

func (s *NodeInterfaceTestSuite) TestStartNodeImplementsRepresentationNodeInterface() {
	node := newRepresentationNode("start", common.NodeTypeStart, nil, true, false)
	_, ok := node.(RepresentationNodeInterface)
	s.True(ok, "START node should implement RepresentationNodeInterface")
}

func (s *NodeInterfaceTestSuite) TestEndNodeImplementsRepresentationNodeInterface() {
	node := newRepresentationNode("end", common.NodeTypeEnd, nil, false, true)
	_, ok := node.(RepresentationNodeInterface)
	s.True(ok, "END node should implement RepresentationNodeInterface")
}

func (s *NodeInterfaceTestSuite) TestPromptNodeDoesNotImplementRepresentationNodeInterface() {
	node := newPromptNode("prompt", nil, false, false)
	_, ok := node.(RepresentationNodeInterface)
	s.False(ok, "PROMPT node should NOT implement RepresentationNodeInterface")
}

func (s *NodeInterfaceTestSuite) TestPromptNodeDoesNotImplementExecutorBackedNodeInterface() {
	node := newPromptNode("prompt", nil, false, false)
	_, ok := node.(ExecutorBackedNodeInterface)
	s.False(ok, "PROMPT node should NOT implement ExecutorBackedNodeInterface")
}

func (s *NodeInterfaceTestSuite) TestTaskExecutionNodeImplementsExecutorBackedNodeInterface() {
	node := newTaskExecutionNode("task", nil, false, false)
	_, ok := node.(ExecutorBackedNodeInterface)
	s.True(ok, "TASK_EXECUTION node should implement ExecutorBackedNodeInterface")
}

func (s *NodeInterfaceTestSuite) TestPromptNodeImplementsPromptNodeInterface() {
	node := newPromptNode("prompt", nil, false, false)
	_, ok := node.(PromptNodeInterface)
	s.True(ok, "PROMPT node should implement PromptNodeInterface")
}
