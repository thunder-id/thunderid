// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package graphbuilder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/executormock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/interceptormock"
)

type VariablesNodesTestSuite struct {
	suite.Suite
}

func TestVariablesNodesTestSuite(t *testing.T) {
	suite.Run(t, new(VariablesNodesTestSuite))
}

func (s *VariablesNodesTestSuite) SetupSuite() {
	_ = config.InitializeServerRuntime("test", &config.Config{
		Server: engineconfig.ServerConfig{Identifier: "test-deployment"},
	})
}

func (s *VariablesNodesTestSuite) twoVariablesFlow() []providers.NodeDefinition {
	return []providers.NodeDefinition{
		{ID: "start", Type: "START", OnSuccess: "first"},
		{
			ID: "first", Type: string(common.NodeTypeVariables), OnSuccess: "second",
			Inputs: []providers.VariableDefinition{
				{Identifier: "var1", Type: providers.InputTypeBoolean, DefaultValue: true},
				{Identifier: "var2", Type: providers.InputTypeText, DefaultValue: "a"},
			},
		},
		{
			ID: "second", Type: string(common.NodeTypeVariables), OnSuccess: "end",
			Inputs: []providers.VariableDefinition{
				{Identifier: "var3", Type: providers.InputTypeNumber, DefaultValue: 3.0},
			},
		},
		{ID: "end", Type: "END"},
	}
}

// resolvedVariables runs a VARIABLES node built from the properties and returns what it resolved into the runtime
// data.
func (s *VariablesNodesTestSuite) resolvedVariables(properties map[string]interface{}) map[string]string {
	factory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	node, err := factory.CreateNode("vars", string(common.NodeTypeVariables), properties, false, false)
	s.Require().NoError(err)

	return s.run(node)
}

func (s *VariablesNodesTestSuite) run(node core.NodeInterface) map[string]string {
	resp, svcErr := node.Execute(&providers.NodeContext{Context: context.Background(), ExecutionID: "exec-1"})
	s.Require().Nil(svcErr)
	return resp.RuntimeData
}

func (s *VariablesNodesTestSuite) TestVariablesNodeProperties() {
	original := s.twoVariablesFlow()

	properties, err := (&graphBuilder{}).variablesNodeProperties(original)

	s.NoError(err)
	values := s.resolvedVariables(properties)
	s.Len(values, 3, "every node carries the whole set, so each resolves all of the variables")
	s.Equal("true", values["var.var1"])
	s.Equal("3", values["var.var3"])

	s.Len(original[1].Inputs, 2, "the stored flow keeps each node with what it declares")
	s.Empty(original[1].Properties)
}

func (s *VariablesNodesTestSuite) TestVariablesNodeProperties_NoVariables() {
	nodes := []providers.NodeDefinition{{ID: "start", Type: "START"}}

	properties, err := (&graphBuilder{}).variablesNodeProperties(nodes)

	s.NoError(err)
	s.Empty(s.resolvedVariables(properties))
}

// The graph is built through the real node factory, and a VARIABLES node needs no executor.
func (s *VariablesNodesTestSuite) TestBuildGraph_WithVariablesNodes() {
	factory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	registry := executormock.NewExecutorRegistryInterfaceMock(s.T())
	registry.EXPECT().IsRegistered(mock.Anything).Return(false).Maybe()
	builder := &graphBuilder{
		flowFactory:         factory,
		executorRegistry:    registry,
		interceptorRegistry: interceptormock.NewInterceptorRegistryInterfaceMock(s.T()),
		graphCache:          coremock.NewGraphCacheInterfaceMock(s.T()),
		logger:              log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowGraphBuilder")),
	}

	graph, err := builder.buildGraph(context.Background(), &providers.CompleteFlowDefinition{
		ID: "flow-1", Handle: "h", Name: "n", FlowType: providers.FlowTypeAuthentication,
		Nodes: s.twoVariablesFlow(),
	})

	s.NoError(err)
	node, ok := graph.GetNode("first")
	s.True(ok)
	s.Equal(common.NodeTypeVariables, node.GetType())
	representation, ok := node.(core.RepresentationNodeInterface)
	s.True(ok)
	s.Equal("second", representation.GetOnSuccess())
	s.False(node.IsFinalNode())
	s.Len(s.run(node), 3)
}
