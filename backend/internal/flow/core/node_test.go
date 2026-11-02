// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"testing"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/suite"
)

type NodeTestSuite struct {
	suite.Suite
}

func TestNodeTestSuite(t *testing.T) {
	suite.Run(t, new(NodeTestSuite))
}

func (s *NodeTestSuite) TestExecuteBaseNodeReturnsError() {
	node := newTaskExecutionNode("node-1", nil, false, false)

	resp, err := node.Execute(&providers.NodeContext{ExecutionID: "f1"})

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

	inputs := []providers.Input{{Identifier: "i1", Required: true}}
	execNode.SetInputs(inputs)
	s.Equal(inputs, execNode.GetInputs())
}

// A claim named after flow control state can neither select nor skip a node, while the same state set
// in runtime data still decides the condition.
func (s *NodeTestSuite) TestShouldExecuteIgnoresExternalClaims() {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{"ProvisioningEligibility", common.RuntimeKeyUserEligibleForProvisioning, "true"},
		{"EntityState", common.RuntimeKeyEntityState, "exists"},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			node := newTaskExecutionNode("n1", nil, false, false)
			node.SetCondition(&NodeCondition{Key: "{{ctx(" + tt.key + ")}}", Value: tt.value})

			ctx := &providers.NodeContext{
				RuntimeData: externalIdentityData(map[string]interface{}{tt.key: tt.value}),
				UserInputs:  map[string]string{},
			}
			s.False(node.ShouldExecute(ctx), "a claim satisfied the condition")

			ctx.RuntimeData[tt.key] = tt.value
			s.True(node.ShouldExecute(ctx), "runtime data no longer satisfies the condition")
		})
	}
}
