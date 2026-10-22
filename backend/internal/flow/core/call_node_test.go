// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type CallNodeTestSuite struct {
	suite.Suite
}

func TestCallNodeTestSuite(t *testing.T) {
	suite.Run(t, new(CallNodeTestSuite))
}

func (s *CallNodeTestSuite) TestNewCallNode_ReturnsCallNodeInterface() {
	node := newCallNode("call-1", nil, false, false)

	s.NotNil(node)

	callNode, ok := node.(CallNodeInterface)
	s.True(ok, "Node should implement CallNodeInterface")
	s.Equal("call-1", callNode.GetID())
	s.Equal(common.NodeTypeCall, callNode.GetType())
}

func (s *CallNodeTestSuite) TestNewCallNode_WithProperties() {
	props := map[string]interface{}{
		"key": "value",
	}
	node := newCallNode("call-2", props, true, false)

	s.NotNil(node)
	s.Equal(props, node.GetProperties())
	s.True(node.IsStartNode())
	s.False(node.IsFinalNode())
}

func (s *CallNodeTestSuite) TestNewCallNode_NilPropertiesInitialized() {
	node := newCallNode("call-3", nil, false, true)

	s.NotNil(node)
	s.NotNil(node.GetProperties())
	s.False(node.IsStartNode())
	s.True(node.IsFinalNode())
}

func (s *CallNodeTestSuite) TestGetType_ReturnsNodeTypeCall() {
	node := newCallNode("call-1", nil, false, false)

	s.Equal(common.NodeTypeCall, node.GetType())
}

func (s *CallNodeTestSuite) TestGetAndSetReferencedFlow() {
	node := newCallNode("call-1", nil, false, false)
	callNode, ok := node.(CallNodeInterface)
	s.True(ok)

	s.Empty(callNode.GetReferencedFlow())

	callNode.SetReferencedFlow("flow-abc")
	s.Equal("flow-abc", callNode.GetReferencedFlow())

	callNode.SetReferencedFlow("flow-xyz")
	s.Equal("flow-xyz", callNode.GetReferencedFlow())

	callNode.SetReferencedFlow("")
	s.Empty(callNode.GetReferencedFlow())
}

func (s *CallNodeTestSuite) TestFlowTypeTargetUsesApplicationFlow() {
	application := providers.Application{InboundAuthProfile: providers.InboundAuthProfile{
		AuthFlowID:         "auth-flow",
		RegistrationFlowID: "registration-flow",
		RecoveryFlowID:     "recovery-flow",
		SignOutFlowID:      "sign-out-flow",
	}}
	tests := []struct {
		name     string
		flowType providers.FlowType
		flowID   string
	}{
		{name: "authentication", flowType: providers.FlowTypeAuthentication, flowID: "auth-flow"},
		{name: "registration", flowType: providers.FlowTypeRegistration, flowID: "registration-flow"},
		{name: "recovery", flowType: providers.FlowTypeRecovery, flowID: "recovery-flow"},
		{name: "sign-out", flowType: providers.FlowTypeSignOut, flowID: "sign-out-flow"},
	}
	for _, test := range tests {
		s.Run(test.name, func() {
			node := newCallNode("call-flow-type", nil, false, false).(CallNodeInterface)
			node.SetReferencedFlow(common.FlowTypeReferencePrefix + string(test.flowType))
			response, err := node.Execute(&providers.NodeContext{
				Context:     context.Background(),
				FlowType:    providers.FlowTypeSystem,
				Application: application,
			})
			s.Nil(err)
			s.Equal(test.flowID, response.CallTargetFlowID)
			s.Equal(test.flowType, response.CallTargetFlowType)
		})
	}
}

func (s *CallNodeTestSuite) TestFlowTypeTargetRejectsMissingApplicationFlow() {
	node := newCallNode("call-flow-type", nil, false, false).(CallNodeInterface)
	node.SetReferencedFlow(common.FlowTypeReferencePrefix + string(providers.FlowTypeRecovery))
	response, err := node.Execute(&providers.NodeContext{Context: context.Background(),
		FlowType: providers.FlowTypeSystem})
	s.Nil(response)
	s.NotNil(err)
}

func (s *CallNodeTestSuite) TestGetAndSetOnSuccess() {
	node := newCallNode("call-1", nil, false, false)
	callNode, ok := node.(CallNodeInterface)
	s.True(ok)

	s.Empty(callNode.GetOnSuccess())

	callNode.SetOnSuccess("next-node")
	s.Equal("next-node", callNode.GetOnSuccess())

	callNode.SetOnSuccess("another-node")
	s.Equal("another-node", callNode.GetOnSuccess())

	callNode.SetOnSuccess("")
	s.Empty(callNode.GetOnSuccess())
}

func (s *CallNodeTestSuite) TestGetAndSetOnFailure() {
	node := newCallNode("call-1", nil, false, false)
	callNode, ok := node.(CallNodeInterface)
	s.True(ok)

	s.Empty(callNode.GetOnFailure())

	callNode.SetOnFailure("error-node")
	s.Equal("error-node", callNode.GetOnFailure())

	callNode.SetOnFailure("another-error-node")
	s.Equal("another-error-node", callNode.GetOnFailure())

	callNode.SetOnFailure("")
	s.Empty(callNode.GetOnFailure())
}

func (s *CallNodeTestSuite) TestExecute_EmptyReferencedFlow_ReturnsInternalServerError() {
	node := newCallNode("call-1", nil, false, false)
	callNode, ok := node.(CallNodeInterface)
	s.True(ok)

	// referencedFlow is empty by default
	ctx := &providers.NodeContext{
		Context:     context.Background(),
		ExecutionID: "test-exec",
	}

	resp, err := callNode.Execute(ctx)

	s.Nil(resp)
	s.NotNil(err)
}

func (s *CallNodeTestSuite) TestExecute_WithReferencedFlow_ReturnsNodeStatusCall() {
	node := newCallNode("call-1", nil, false, false)
	callNode, ok := node.(CallNodeInterface)
	s.True(ok)

	callNode.SetReferencedFlow("target-flow-id")

	ctx := &providers.NodeContext{
		Context:     context.Background(),
		ExecutionID: "test-exec",
	}

	resp, err := callNode.Execute(ctx)

	s.NotNil(resp)
	s.Nil(err)
	s.Equal(common.NodeStatusCall, resp.Status)
	s.Equal("target-flow-id", resp.CallTargetFlowID)
}

func (s *CallNodeTestSuite) TestExecute_ReturnsCorrectCallTargetFlowID() {
	node := newCallNode("call-1", nil, false, false)
	callNode, ok := node.(CallNodeInterface)
	s.True(ok)

	callNode.SetReferencedFlow("registration-flow-123")

	ctx := &providers.NodeContext{
		Context:     context.Background(),
		ExecutionID: "exec-456",
	}

	resp, err := callNode.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal("registration-flow-123", resp.CallTargetFlowID)
}
