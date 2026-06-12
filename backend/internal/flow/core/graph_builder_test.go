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
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

type GraphBuilderTestSuite struct {
	suite.Suite
	mockFlowFactory      *FlowFactoryInterfaceMock
	mockExecutorRegistry *ExecutorRegistryInterfaceMock
	mockGraphCache       *GraphCacheInterfaceMock
	builder              *graphBuilder
}

func TestGraphBuilderTestSuite(t *testing.T) {
	suite.Run(t, new(GraphBuilderTestSuite))
}

func (s *GraphBuilderTestSuite) SetupTest() {
	_ = config.InitializeServerRuntime("test", &config.Config{
		Server: config.ServerConfig{Identifier: "test-deployment"},
	})

	s.mockFlowFactory = NewFlowFactoryInterfaceMock(s.T())
	s.mockExecutorRegistry = NewExecutorRegistryInterfaceMock(s.T())
	s.mockGraphCache = NewGraphCacheInterfaceMock(s.T())

	s.builder = &graphBuilder{
		flowFactory:      s.mockFlowFactory,
		executorRegistry: s.mockExecutorRegistry,
		graphCache:       s.mockGraphCache,
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowGraphBuilder")),
	}
}

// Test GetGraph method

func (s *GraphBuilderTestSuite) TestGetGraph_NilFlow() {
	graph, err := s.builder.GetGraph(context.Background(), nil)

	s.Nil(graph)
	s.NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "Flow definition is nil or has no nodes")
}

func (s *GraphBuilderTestSuite) TestGetGraph_EmptyNodes() {
	flow := &common.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes:    []common.NodeDefinition{},
	}

	graph, err := s.builder.GetGraph(context.Background(), flow)

	s.Nil(graph)
	s.NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "Flow definition is nil or has no nodes")
}

func (s *GraphBuilderTestSuite) TestGetGraph_CacheHit() {
	flow := &common.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []common.NodeDefinition{
			{ID: "start", Type: "START"},
		},
	}

	mockGraph := NewGraphInterfaceMock(s.T())
	s.mockGraphCache.EXPECT().Get(mock.Anything, "flow-1").Return(mockGraph, true)

	graph, err := s.builder.GetGraph(context.Background(), flow)

	s.NotNil(graph)
	s.Nil(err)
	s.Equal(mockGraph, graph)
}

func (s *GraphBuilderTestSuite) TestGetGraph_BuildAndCache() {
	flow := &common.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []common.NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "end"},
			{ID: "end", Type: "END"},
		},
	}

	mockGraph := NewGraphInterfaceMock(s.T())
	mockStartNode := NewRepresentationNodeInterfaceMock(s.T())
	mockEndNode := NewRepresentationNodeInterfaceMock(s.T())

	s.mockGraphCache.EXPECT().Get(mock.Anything, "flow-1").Return(nil, false)
	s.mockFlowFactory.EXPECT().CreateGraph(
		"flow-1", common.FlowTypeAuthentication).Return(mockGraph)
	s.mockFlowFactory.EXPECT().CreateNode(
		"start", "START", map[string]interface{}(nil), false, false).Return(
		mockStartNode, nil)
	s.mockFlowFactory.EXPECT().CreateNode(
		"end", "END", map[string]interface{}(nil), false, true).Return(
		mockEndNode, nil)

	mockStartNode.EXPECT().SetOnSuccess("end")

	mockGraph.EXPECT().AddNode(mockStartNode).Return(nil)
	mockGraph.EXPECT().AddNode(mockEndNode).Return(nil)
	mockGraph.EXPECT().AddEdge("start", "end").Return(nil)
	mockGraph.EXPECT().GetNodes().Return(
		map[string]NodeInterface{"start": mockStartNode, "end": mockEndNode})
	// Map iteration order is non-deterministic, so other nodes might be checked before START is found
	mockStartNode.EXPECT().GetType().Return(common.NodeTypeStart)
	mockEndNode.EXPECT().GetType().Return(common.NodeTypeEnd).Maybe()
	mockStartNode.EXPECT().GetID().Return("start")
	mockGraph.EXPECT().SetStartNode("start").Return(nil)

	s.mockGraphCache.EXPECT().Set(mock.Anything, "flow-1", mockGraph).Return(nil)

	graph, err := s.builder.GetGraph(context.Background(), flow)

	s.NotNil(graph)
	s.Nil(err)
	s.Equal(mockGraph, graph)
}

func (s *GraphBuilderTestSuite) TestGetGraph_BuildFailure() {
	flow := &common.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []common.NodeDefinition{
			{ID: "start", Type: "START"},
		},
	}

	mockGraph := NewGraphInterfaceMock(s.T())
	s.mockGraphCache.EXPECT().Get(mock.Anything, "flow-1").Return(nil, false)
	s.mockFlowFactory.EXPECT().CreateGraph("flow-1", common.FlowTypeAuthentication).Return(
		mockGraph)
	s.mockFlowFactory.EXPECT().CreateNode(
		"start", "START", map[string]interface{}(nil), false, true).Return(
		nil, errors.New("node creation error"))

	graph, err := s.builder.GetGraph(context.Background(), flow)

	s.Nil(graph)
	s.NotNil(err)
	s.Equal(ErrorGraphBuildFailure.Code, err.Code)
	s.Equal(ErrorGraphBuildFailure.ErrorDescription.DefaultValue, err.ErrorDescription.DefaultValue)
}

func (s *GraphBuilderTestSuite) TestGetGraph_CacheSetError() {
	flow := &common.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []common.NodeDefinition{
			{ID: "start", Type: "START"},
		},
	}

	mockGraph := NewGraphInterfaceMock(s.T())
	mockStartNode := NewNodeInterfaceMock(s.T())

	s.mockGraphCache.EXPECT().Get(mock.Anything, "flow-1").Return(nil, false)
	s.mockFlowFactory.EXPECT().CreateGraph(
		"flow-1", common.FlowTypeAuthentication).Return(
		mockGraph)
	s.mockFlowFactory.EXPECT().CreateNode(
		"start", "START", map[string]interface{}(nil), false, true).Return(
		mockStartNode, nil)

	mockGraph.EXPECT().AddNode(mockStartNode).Return(nil)
	mockGraph.EXPECT().GetNodes().Return(map[string]NodeInterface{"start": mockStartNode})
	mockStartNode.EXPECT().GetType().Return(common.NodeTypeStart)
	mockStartNode.EXPECT().GetID().Return("start")
	mockGraph.EXPECT().SetStartNode("start").Return(nil)

	s.mockGraphCache.EXPECT().Set(mock.Anything, "flow-1", mockGraph).Return(errors.New("cache error"))

	graph, err := s.builder.GetGraph(context.Background(), flow)

	// Should still return graph even if caching fails
	s.NotNil(graph)
	s.Nil(err)
	s.Equal(mockGraph, graph)
}

// Test InvalidateCache method

func (s *GraphBuilderTestSuite) TestInvalidateCache_EmptyFlowID() {
	// Should not panic or error
	s.builder.InvalidateCache(context.Background(), "")
}

func (s *GraphBuilderTestSuite) TestInvalidateCache_Success() {
	s.mockGraphCache.EXPECT().Invalidate(mock.Anything, "flow-1").Return(nil)

	s.builder.InvalidateCache(context.Background(), "flow-1")
}

func (s *GraphBuilderTestSuite) TestInvalidateCache_Error() {
	s.mockGraphCache.EXPECT().Invalidate(mock.Anything, "flow-1").Return(errors.New("cache error"))

	// Should log error but not panic
	s.builder.InvalidateCache(context.Background(), "flow-1")
}

// Test buildGraph method

func (s *GraphBuilderTestSuite) TestBuildGraph_WithExecutor() {
	flow := &common.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []common.NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "task"},
			{
				ID:       "task",
				Type:     "TASK_EXECUTION",
				Executor: &common.ExecutorDefinition{Name: "test-executor"},
			},
		},
	}

	mockGraph := NewGraphInterfaceMock(s.T())
	mockStartNode := NewNodeInterfaceMock(s.T())
	mockTaskNode := NewExecutorBackedNodeInterfaceMock(s.T())

	s.mockFlowFactory.EXPECT().CreateGraph(
		"flow-1", common.FlowTypeAuthentication).Return(
		mockGraph)
	s.mockFlowFactory.EXPECT().CreateNode(
		"start", "START", map[string]interface{}(nil), false, false).Return(
		mockStartNode, nil)
	s.mockFlowFactory.EXPECT().CreateNode(
		"task", "TASK_EXECUTION", map[string]interface{}(nil), false, true).Return(
		mockTaskNode, nil)

	s.mockExecutorRegistry.EXPECT().IsRegistered("test-executor").Return(true)
	mockTaskNode.EXPECT().SetExecutorName("test-executor")
	mockTaskNode.EXPECT().SetInputs([]common.Input{})

	mockGraph.EXPECT().AddNode(mockStartNode).Return(nil)
	mockGraph.EXPECT().AddNode(mockTaskNode).Return(nil)
	mockGraph.EXPECT().AddEdge("start", "task").Return(nil)
	mockGraph.EXPECT().GetNodes().Return(
		map[string]NodeInterface{"start": mockStartNode, "task": mockTaskNode})
	// Map iteration order is non-deterministic, so other nodes might be checked before START is found
	mockStartNode.EXPECT().GetType().Return(common.NodeTypeStart)
	mockTaskNode.EXPECT().GetType().Return(common.NodeTypeTaskExecution).Maybe()
	mockStartNode.EXPECT().GetID().Return("start")
	mockGraph.EXPECT().SetStartNode("start").Return(nil)

	graph, err := s.builder.buildGraph(context.Background(), flow)

	s.NotNil(graph)
	s.Nil(err)
}

func (s *GraphBuilderTestSuite) TestBuildGraph_ExecutorNotRegistered() {
	flow := &common.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []common.NodeDefinition{
			{
				ID:       "task",
				Type:     "TASK_EXECUTION",
				Executor: &common.ExecutorDefinition{Name: "unknown-executor"},
			},
		},
	}

	mockGraph := NewGraphInterfaceMock(s.T())
	mockTaskNode := NewExecutorBackedNodeInterfaceMock(s.T())

	s.mockFlowFactory.EXPECT().CreateGraph(
		"flow-1", common.FlowTypeAuthentication).Return(
		mockGraph)
	s.mockFlowFactory.EXPECT().CreateNode(
		"task", "TASK_EXECUTION", map[string]interface{}(nil), false, true).Return(
		mockTaskNode, nil)
	mockTaskNode.EXPECT().SetInputs([]common.Input{})

	s.mockExecutorRegistry.EXPECT().IsRegistered("unknown-executor").Return(false)

	graph, err := s.builder.buildGraph(context.Background(), flow)

	s.Nil(graph)
	s.NotNil(err)
	s.Contains(err.Error(), "executor with name unknown-executor not registered")
}

func (s *GraphBuilderTestSuite) TestBuildGraph_WithOnFailure() {
	flow := &common.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []common.NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "task"},
			{
				ID:        "task",
				Type:      "TASK_EXECUTION",
				OnSuccess: "end",
				OnFailure: "error-prompt",
				Executor:  &common.ExecutorDefinition{Name: "test-executor"},
			},
			{ID: "error-prompt", Type: "PROMPT"},
			{ID: "end", Type: "END"},
		},
	}

	mockGraph := NewGraphInterfaceMock(s.T())
	mockStartNode := NewRepresentationNodeInterfaceMock(s.T())
	mockTaskNode := NewExecutorBackedNodeInterfaceMock(s.T())
	mockPromptNode := NewPromptNodeInterfaceMock(s.T())
	mockEndNode := NewRepresentationNodeInterfaceMock(s.T())

	s.mockFlowFactory.EXPECT().CreateGraph(
		"flow-1", common.FlowTypeAuthentication).Return(
		mockGraph)
	s.mockFlowFactory.EXPECT().CreateNode(
		"start", "START", map[string]interface{}(nil), false, false).Return(
		mockStartNode, nil)
	s.mockFlowFactory.EXPECT().CreateNode(
		"task", "TASK_EXECUTION", map[string]interface{}(nil), false, false).Return(
		mockTaskNode, nil)
	s.mockFlowFactory.EXPECT().CreateNode(
		"error-prompt", "PROMPT", map[string]interface{}(nil), false, true).Return(
		mockPromptNode, nil)
	s.mockFlowFactory.EXPECT().CreateNode(
		"end", "END", map[string]interface{}(nil), false, true).Return(
		mockEndNode, nil)

	mockStartNode.EXPECT().SetOnSuccess("task")
	mockTaskNode.EXPECT().SetOnSuccess("end")
	mockTaskNode.EXPECT().SetOnFailure("error-prompt")
	mockTaskNode.EXPECT().SetInputs([]common.Input{})

	s.mockExecutorRegistry.EXPECT().IsRegistered("test-executor").Return(true)
	mockTaskNode.EXPECT().SetExecutorName("test-executor")

	mockGraph.EXPECT().AddNode(mockStartNode).Return(nil)
	mockGraph.EXPECT().AddNode(mockTaskNode).Return(nil)
	mockGraph.EXPECT().AddNode(mockPromptNode).Return(nil)
	mockGraph.EXPECT().AddNode(mockEndNode).Return(nil)
	mockGraph.EXPECT().AddEdge("start", "task").Return(nil)
	mockGraph.EXPECT().AddEdge("task", "end").Return(nil)
	mockGraph.EXPECT().AddEdge("task", "error-prompt").Return(nil)
	mockGraph.EXPECT().GetNodes().Return(
		map[string]NodeInterface{"start": mockStartNode, "task": mockTaskNode,
			"error-prompt": mockPromptNode, "end": mockEndNode})
	// Map iteration order is non-deterministic, so other nodes might be checked before START is found
	mockStartNode.EXPECT().GetType().Return(common.NodeTypeStart)
	mockTaskNode.EXPECT().GetType().Return(common.NodeTypeTaskExecution).Maybe()
	mockPromptNode.EXPECT().GetType().Return(common.NodeTypePrompt).Maybe()
	mockEndNode.EXPECT().GetType().Return(common.NodeTypeEnd).Maybe()
	mockStartNode.EXPECT().GetID().Return("start")
	mockGraph.EXPECT().SetStartNode("start").Return(nil)

	graph, err := s.builder.buildGraph(context.Background(), flow)

	s.NotNil(graph)
	s.Nil(err)
}

func (s *GraphBuilderTestSuite) TestBuildGraph_OnFailureNotPromptNode() {
	flow := &common.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []common.NodeDefinition{
			{
				ID:        "task",
				Type:      "TASK_EXECUTION",
				OnFailure: "end",
			},
			{ID: "end", Type: "END"},
		},
	}

	mockGraph := NewGraphInterfaceMock(s.T())
	mockTaskNode := NewExecutorBackedNodeInterfaceMock(s.T())

	s.mockFlowFactory.EXPECT().CreateGraph(
		"flow-1", common.FlowTypeAuthentication).Return(
		mockGraph)
	s.mockFlowFactory.EXPECT().CreateNode(
		"task", "TASK_EXECUTION", map[string]interface{}(nil), false, false).Return(
		mockTaskNode, nil)

	// Validation fails during configureNodeNavigation, before SetInputs is called
	// END node is not created because task node processing fails first

	graph, err := s.builder.buildGraph(context.Background(), flow)

	s.Nil(graph)
	s.NotNil(err)
	s.Contains(err.Error(), "onFailure must point to a PROMPT node")
}

func (s *GraphBuilderTestSuite) TestBuildGraph_OnFailureTargetNotFound() {
	flow := &common.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []common.NodeDefinition{
			{
				ID:        "task",
				Type:      "TASK_EXECUTION",
				OnFailure: "non-existent",
			},
		},
	}

	mockGraph := NewGraphInterfaceMock(s.T())
	mockTaskNode := NewExecutorBackedNodeInterfaceMock(s.T())

	s.mockFlowFactory.EXPECT().CreateGraph(
		"flow-1", common.FlowTypeAuthentication).Return(
		mockGraph)
	s.mockFlowFactory.EXPECT().CreateNode(
		"task", "TASK_EXECUTION", map[string]interface{}(nil), false, false).Return(
		mockTaskNode, nil)

	// Validation fails during configureNodeNavigation, before SetInputs is called

	graph, err := s.builder.buildGraph(context.Background(), flow)

	s.Nil(graph)
	s.NotNil(err)
	s.Contains(err.Error(), "onFailure target node not found")
}

func (s *GraphBuilderTestSuite) TestBuildGraph_WithInputs() {
	flow := &common.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []common.NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "task"},
			{
				ID:   "task",
				Type: "TASK_EXECUTION",
				Executor: &common.ExecutorDefinition{
					Inputs: []common.InputDefinition{
						{Ref: "username", Type: "string", Identifier: "user", Required: true},
						{Ref: "password", Type: "string", Identifier: "pass", Required: true},
					},
				},
			},
		},
	}

	mockGraph := NewGraphInterfaceMock(s.T())
	mockStartNode := NewRepresentationNodeInterfaceMock(s.T())
	mockTaskNode := NewExecutorBackedNodeInterfaceMock(s.T())

	s.mockFlowFactory.EXPECT().CreateGraph(
		"flow-1", common.FlowTypeAuthentication).Return(
		mockGraph)
	s.mockFlowFactory.EXPECT().CreateNode(
		"start", "START", map[string]interface{}(nil), false, false).Return(
		mockStartNode, nil)
	s.mockFlowFactory.EXPECT().CreateNode(
		"task", "TASK_EXECUTION", map[string]interface{}(nil), false, true).Return(
		mockTaskNode, nil)

	mockStartNode.EXPECT().SetOnSuccess("task")
	mockTaskNode.EXPECT().SetInputs([]common.Input{
		{Ref: "username", Type: "string", Identifier: "user", Required: true},
		{Ref: "password", Type: "string", Identifier: "pass", Required: true},
	})

	mockGraph.EXPECT().AddNode(mockStartNode).Return(nil)
	mockGraph.EXPECT().AddNode(mockTaskNode).Return(nil)
	mockGraph.EXPECT().AddEdge("start", "task").Return(nil)
	mockGraph.EXPECT().GetNodes().Return(
		map[string]NodeInterface{"start": mockStartNode, "task": mockTaskNode})
	// Map iteration order is non-deterministic, so other nodes might be checked before START is found
	mockStartNode.EXPECT().GetType().Return(common.NodeTypeStart)
	mockTaskNode.EXPECT().GetType().Return(common.NodeTypeTaskExecution).Maybe()
	mockStartNode.EXPECT().GetID().Return("start")
	mockGraph.EXPECT().SetStartNode("start").Return(nil)

	graph, err := s.builder.buildGraph(context.Background(), flow)

	s.NotNil(graph)
	s.Nil(err)
}

func (s *GraphBuilderTestSuite) TestBuildGraph_WithActions() {
	flow := &common.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []common.NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "prompt"},
			{
				ID:   "prompt",
				Type: "PROMPT",
				Prompts: []common.PromptDefinition{
					{Action: &common.ActionDefinition{Ref: "login", NextNode: "task1"}},
					{Action: &common.ActionDefinition{Ref: "signup", NextNode: "task2"}},
				},
			},
			{ID: "task1", Type: "TASK_EXECUTION"},
			{ID: "task2", Type: "TASK_EXECUTION"},
		},
	}

	mockGraph := NewGraphInterfaceMock(s.T())
	mockStartNode := NewRepresentationNodeInterfaceMock(s.T())
	mockPromptNode := NewPromptNodeInterfaceMock(s.T())
	mockTask1Node := NewExecutorBackedNodeInterfaceMock(s.T())
	mockTask2Node := NewExecutorBackedNodeInterfaceMock(s.T())

	s.mockFlowFactory.EXPECT().CreateGraph(
		"flow-1", common.FlowTypeAuthentication).Return(
		mockGraph)
	s.mockFlowFactory.EXPECT().CreateNode(
		"start", "START", map[string]interface{}(nil), false, false).Return(
		mockStartNode, nil)
	s.mockFlowFactory.EXPECT().CreateNode(
		"prompt", "PROMPT", map[string]interface{}(nil), false, false).Return(
		mockPromptNode, nil)
	s.mockFlowFactory.EXPECT().CreateNode(
		"task1", "TASK_EXECUTION", map[string]interface{}(nil), false, true).Return(
		mockTask1Node, nil)
	s.mockFlowFactory.EXPECT().CreateNode(
		"task2", "TASK_EXECUTION", map[string]interface{}(nil), false, true).Return(
		mockTask2Node, nil)

	mockStartNode.EXPECT().SetOnSuccess("prompt")

	mockPromptNode.EXPECT().SetPrompts(mock.MatchedBy(func(prompts []common.Prompt) bool {
		if len(prompts) != 2 {
			return false
		}
		if prompts[0].Action.Ref != "login" || prompts[0].Action.NextNode != "task1" {
			return false
		}
		if prompts[1].Action.Ref != "signup" || prompts[1].Action.NextNode != "task2" {
			return false
		}
		return true
	}))
	mockTask1Node.EXPECT().SetInputs([]common.Input{})
	mockTask2Node.EXPECT().SetInputs([]common.Input{})

	mockGraph.EXPECT().AddNode(mockStartNode).Return(nil)
	mockGraph.EXPECT().AddNode(mockPromptNode).Return(nil)
	mockGraph.EXPECT().AddNode(mockTask1Node).Return(nil)
	mockGraph.EXPECT().AddNode(mockTask2Node).Return(nil)
	mockGraph.EXPECT().AddEdge("start", "prompt").Return(nil)
	mockGraph.EXPECT().AddEdge("prompt", "task1").Return(nil)
	mockGraph.EXPECT().AddEdge("prompt", "task2").Return(nil)
	mockGraph.EXPECT().GetNodes().Return(
		map[string]NodeInterface{"start": mockStartNode, "prompt": mockPromptNode,
			"task1": mockTask1Node, "task2": mockTask2Node})
	// Map iteration order is non-deterministic, so other nodes might be checked before START is found
	mockStartNode.EXPECT().GetType().Return(common.NodeTypeStart)
	mockPromptNode.EXPECT().GetType().Return(common.NodeTypePrompt).Maybe()
	mockTask1Node.EXPECT().GetType().Return(common.NodeTypeTaskExecution).Maybe()
	mockTask2Node.EXPECT().GetType().Return(common.NodeTypeTaskExecution).Maybe()
	mockStartNode.EXPECT().GetID().Return("start")
	mockGraph.EXPECT().SetStartNode("start").Return(nil)

	graph, err := s.builder.buildGraph(context.Background(), flow)

	s.NotNil(graph)
	s.Nil(err)
}

func (s *GraphBuilderTestSuite) TestBuildGraph_WithMeta() {
	flow := &common.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []common.NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "prompt"},
			{
				ID:   "prompt",
				Type: "PROMPT",
				Meta: map[string]interface{}{"title": "Login", "description": "Enter credentials"},
			},
		},
	}

	mockGraph := NewGraphInterfaceMock(s.T())
	mockStartNode := NewRepresentationNodeInterfaceMock(s.T())
	mockPromptNode := NewPromptNodeInterfaceMock(s.T())

	s.mockFlowFactory.EXPECT().CreateGraph(
		"flow-1", common.FlowTypeAuthentication).Return(
		mockGraph)
	s.mockFlowFactory.EXPECT().CreateNode(
		"start", "START", map[string]interface{}(nil), false, false).Return(
		mockStartNode, nil)
	s.mockFlowFactory.EXPECT().CreateNode(
		"prompt", "PROMPT", map[string]interface{}(nil), false, true).Return(
		mockPromptNode, nil)

	mockStartNode.EXPECT().SetOnSuccess("prompt")

	mockPromptNode.EXPECT().SetMeta(map[string]interface{}{
		"title": "Login", "description": "Enter credentials"})

	mockGraph.EXPECT().AddNode(mockStartNode).Return(nil)
	mockGraph.EXPECT().AddNode(mockPromptNode).Return(nil)
	mockGraph.EXPECT().AddEdge("start", "prompt").Return(nil)
	mockGraph.EXPECT().GetNodes().Return(
		map[string]NodeInterface{"start": mockStartNode, "prompt": mockPromptNode})
	// Map iteration order is non-deterministic, so other nodes might be checked before START is found
	mockStartNode.EXPECT().GetType().Return(common.NodeTypeStart)
	mockPromptNode.EXPECT().GetType().Return(common.NodeTypePrompt).Maybe()
	mockStartNode.EXPECT().GetID().Return("start")
	mockGraph.EXPECT().SetStartNode("start").Return(nil)

	graph, err := s.builder.buildGraph(context.Background(), flow)

	s.NotNil(graph)
	s.Nil(err)
}

func (s *GraphBuilderTestSuite) TestBuildGraph_VariantExplicitlySet() {
	flow := &common.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []common.NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "chooser"},
			{
				ID:      "chooser",
				Type:    "PROMPT",
				Variant: common.NodeVariantLoginOptions,
			},
		},
	}

	mockGraph := NewGraphInterfaceMock(s.T())
	mockStartNode := NewRepresentationNodeInterfaceMock(s.T())
	mockPromptNode := NewPromptNodeInterfaceMock(s.T())

	s.mockFlowFactory.EXPECT().CreateGraph(
		"flow-1", common.FlowTypeAuthentication).Return(mockGraph)
	s.mockFlowFactory.EXPECT().CreateNode(
		"start", "START", map[string]interface{}(nil), false, false).Return(mockStartNode, nil)
	s.mockFlowFactory.EXPECT().CreateNode(
		"chooser", "PROMPT", map[string]interface{}(nil), false, true).Return(mockPromptNode, nil)

	mockStartNode.EXPECT().SetOnSuccess("chooser")
	mockPromptNode.EXPECT().SetVariant(common.NodeVariantLoginOptions)

	mockGraph.EXPECT().AddNode(mockStartNode).Return(nil)
	mockGraph.EXPECT().AddNode(mockPromptNode).Return(nil)
	mockGraph.EXPECT().AddEdge("start", "chooser").Return(nil)
	mockGraph.EXPECT().GetNodes().Return(
		map[string]NodeInterface{"start": mockStartNode, "chooser": mockPromptNode})
	mockStartNode.EXPECT().GetType().Return(common.NodeTypeStart)
	mockPromptNode.EXPECT().GetType().Return(common.NodeTypePrompt).Maybe()
	mockStartNode.EXPECT().GetID().Return("start")
	mockGraph.EXPECT().SetStartNode("start").Return(nil)

	graph, err := s.builder.buildGraph(context.Background(), flow)

	s.NotNil(graph)
	s.Nil(err)
}

func (s *GraphBuilderTestSuite) TestBuildGraph_WithCondition() {
	flow := &common.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []common.NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "task"},
			{
				ID:   "task",
				Type: "TASK_EXECUTION",
				Condition: &common.ConditionDefinition{
					Key:    "userType",
					Value:  "premium",
					OnSkip: "end",
				},
			},
			{ID: "end", Type: "END"},
		},
	}

	mockGraph := NewGraphInterfaceMock(s.T())
	mockStartNode := NewRepresentationNodeInterfaceMock(s.T())
	mockTaskNode := NewExecutorBackedNodeInterfaceMock(s.T())
	mockEndNode := NewRepresentationNodeInterfaceMock(s.T())

	s.mockFlowFactory.EXPECT().CreateGraph(
		"flow-1", common.FlowTypeAuthentication).Return(
		mockGraph)
	s.mockFlowFactory.EXPECT().CreateNode(
		"start", "START", map[string]interface{}(nil), false, false).Return(
		mockStartNode, nil)
	s.mockFlowFactory.EXPECT().CreateNode(
		"task", "TASK_EXECUTION", map[string]interface{}(nil), false, true).Return(
		mockTaskNode, nil)
	s.mockFlowFactory.EXPECT().CreateNode(
		"end", "END", map[string]interface{}(nil), false, true).Return(
		mockEndNode, nil)

	mockStartNode.EXPECT().SetOnSuccess("task")
	mockTaskNode.EXPECT().SetInputs([]common.Input{})
	mockTaskNode.EXPECT().SetCondition(&NodeCondition{
		Key:    "userType",
		Value:  "premium",
		OnSkip: "end",
	})

	mockGraph.EXPECT().AddNode(mockStartNode).Return(nil)
	mockGraph.EXPECT().AddNode(mockTaskNode).Return(nil)
	mockGraph.EXPECT().AddNode(mockEndNode).Return(nil)
	mockGraph.EXPECT().AddEdge("start", "task").Return(nil)
	mockGraph.EXPECT().GetNodes().Return(
		map[string]NodeInterface{"start": mockStartNode, "task": mockTaskNode,
			"end": mockEndNode})
	// Map iteration order is non-deterministic, so other nodes might be checked before START is found
	mockStartNode.EXPECT().GetType().Return(common.NodeTypeStart)
	mockTaskNode.EXPECT().GetType().Return(common.NodeTypeTaskExecution).Maybe()
	mockEndNode.EXPECT().GetType().Return(common.NodeTypeEnd).Maybe()
	mockStartNode.EXPECT().GetID().Return("start")
	mockGraph.EXPECT().SetStartNode("start").Return(nil)

	graph, err := s.builder.buildGraph(context.Background(), flow)

	s.NotNil(graph)
	s.Nil(err)
}

func (s *GraphBuilderTestSuite) TestBuildGraph_NoStartNode() {
	flow := &common.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []common.NodeDefinition{
			{ID: "task", Type: "TASK_EXECUTION"},
			{ID: "end", Type: "END"},
		},
	}

	mockGraph := NewGraphInterfaceMock(s.T())
	mockTaskNode := NewNodeInterfaceMock(s.T())
	mockEndNode := NewNodeInterfaceMock(s.T())

	s.mockFlowFactory.EXPECT().CreateGraph(
		"flow-1", common.FlowTypeAuthentication).Return(
		mockGraph)
	s.mockFlowFactory.EXPECT().CreateNode(
		"task", "TASK_EXECUTION", map[string]interface{}(nil), false, true).Return(
		mockTaskNode, nil)
	s.mockFlowFactory.EXPECT().CreateNode(
		"end", "END", map[string]interface{}(nil), false, true).Return(
		mockEndNode, nil)

	mockGraph.EXPECT().AddNode(mockTaskNode).Return(nil)
	mockGraph.EXPECT().AddNode(mockEndNode).Return(nil)
	mockGraph.EXPECT().GetNodes().Return(
		map[string]NodeInterface{"task": mockTaskNode, "end": mockEndNode})
	mockTaskNode.EXPECT().GetType().Return(common.NodeTypeTaskExecution)
	mockEndNode.EXPECT().GetType().Return(common.NodeTypeEnd)

	graph, err := s.builder.buildGraph(context.Background(), flow)

	s.Nil(graph)
	s.NotNil(err)
	s.Contains(err.Error(), "no start node found")
}

func (s *GraphBuilderTestSuite) TestBuildGraph_AddNodeError() {
	flow := &common.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []common.NodeDefinition{
			{ID: "start", Type: "START"},
		},
	}

	mockGraph := NewGraphInterfaceMock(s.T())
	mockStartNode := NewNodeInterfaceMock(s.T())

	s.mockFlowFactory.EXPECT().CreateGraph(
		"flow-1", common.FlowTypeAuthentication).Return(
		mockGraph)
	s.mockFlowFactory.EXPECT().CreateNode(
		"start", "START", map[string]interface{}(nil), false, true).Return(
		mockStartNode, nil)

	mockGraph.EXPECT().AddNode(mockStartNode).Return(errors.New("duplicate node"))

	graph, err := s.builder.buildGraph(context.Background(), flow)

	s.Nil(graph)
	s.NotNil(err)
	s.Contains(err.Error(), "failed to add node start to the graph")
}

func (s *GraphBuilderTestSuite) TestBuildGraph_AddEdgeError() {
	flow := &common.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []common.NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "end"},
			{ID: "end", Type: "END"},
		},
	}

	mockGraph := NewGraphInterfaceMock(s.T())
	mockStartNode := NewNodeInterfaceMock(s.T())
	mockEndNode := NewNodeInterfaceMock(s.T())

	s.mockFlowFactory.EXPECT().CreateGraph(
		"flow-1", common.FlowTypeAuthentication).Return(
		mockGraph)
	s.mockFlowFactory.EXPECT().CreateNode(
		"start", "START", map[string]interface{}(nil), false, false).Return(
		mockStartNode, nil)
	s.mockFlowFactory.EXPECT().CreateNode(
		"end", "END", map[string]interface{}(nil), false, true).Return(
		mockEndNode, nil)

	mockGraph.EXPECT().AddNode(mockStartNode).Return(nil)
	mockGraph.EXPECT().AddNode(mockEndNode).Return(nil)
	mockGraph.EXPECT().AddEdge("start", "end").Return(errors.New("edge creation error"))

	graph, err := s.builder.buildGraph(context.Background(), flow)

	s.Nil(graph)
	s.NotNil(err)
	s.Contains(err.Error(), "failed to add edge from start to end")
}

func (s *GraphBuilderTestSuite) TestBuildGraph_SetStartNodeError() {
	flow := &common.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []common.NodeDefinition{
			{ID: "start", Type: "START"},
		},
	}

	mockGraph := NewGraphInterfaceMock(s.T())
	mockStartNode := NewNodeInterfaceMock(s.T())

	s.mockFlowFactory.EXPECT().CreateGraph(
		"flow-1", common.FlowTypeAuthentication).Return(
		mockGraph)
	s.mockFlowFactory.EXPECT().CreateNode(
		"start", "START", map[string]interface{}(nil), false, true).Return(
		mockStartNode, nil)

	mockGraph.EXPECT().AddNode(mockStartNode).Return(nil)
	mockGraph.EXPECT().GetNodes().Return(map[string]NodeInterface{"start": mockStartNode})
	mockStartNode.EXPECT().GetType().Return(common.NodeTypeStart)
	mockStartNode.EXPECT().GetID().Return("start")
	mockGraph.EXPECT().SetStartNode("start").Return(errors.New("start node already set"))

	graph, err := s.builder.buildGraph(context.Background(), flow)

	s.Nil(graph)
	s.NotNil(err)
	s.Contains(err.Error(), "start node already set")
}

func (s *GraphBuilderTestSuite) TestBuildGraph_WithProperties() {
	properties := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
	}

	flow := &common.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []common.NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "task", Properties: properties},
			{ID: "task", Type: "TASK_EXECUTION"},
		},
	}

	mockGraph := NewGraphInterfaceMock(s.T())
	mockStartNode := NewRepresentationNodeInterfaceMock(s.T())
	mockTaskNode := NewRepresentationNodeInterfaceMock(s.T())

	s.mockFlowFactory.EXPECT().CreateGraph(
		"flow-1", common.FlowTypeAuthentication).Return(
		mockGraph)
	s.mockFlowFactory.EXPECT().CreateNode(
		"start", "START", properties, false, false).Return(
		mockStartNode, nil)
	s.mockFlowFactory.EXPECT().CreateNode(
		"task", "TASK_EXECUTION", map[string]interface{}(nil), false, true).Return(
		mockTaskNode, nil)

	mockStartNode.EXPECT().SetOnSuccess("task")

	mockGraph.EXPECT().AddNode(mockStartNode).Return(nil)
	mockGraph.EXPECT().AddNode(mockTaskNode).Return(nil)
	mockGraph.EXPECT().AddEdge("start", "task").Return(nil)
	mockGraph.EXPECT().GetNodes().Return(
		map[string]NodeInterface{"start": mockStartNode, "task": mockTaskNode})
	// Map iteration order is non-deterministic, so other nodes might be checked before START is found
	mockStartNode.EXPECT().GetType().Return(common.NodeTypeStart)
	mockTaskNode.EXPECT().GetType().Return(common.NodeTypeTaskExecution).Maybe()
	mockStartNode.EXPECT().GetID().Return("start")
	mockGraph.EXPECT().SetStartNode("start").Return(nil)

	graph, err := s.builder.buildGraph(context.Background(), flow)

	s.NotNil(graph)
	s.Nil(err)
}

func (s *GraphBuilderTestSuite) TestValidateExecutorName_EmptyName() {
	err := s.builder.validateExecutorName("")

	s.NotNil(err)
	s.Contains(err.Error(), "executor name cannot be empty")
}

func (s *GraphBuilderTestSuite) TestValidateExecutorName_NotRegistered() {
	s.mockExecutorRegistry.EXPECT().IsRegistered("unknown").Return(false)

	err := s.builder.validateExecutorName("unknown")

	s.NotNil(err)
	s.Contains(err.Error(), "executor with name unknown not registered")
}

func (s *GraphBuilderTestSuite) TestValidateExecutorName_Success() {
	s.mockExecutorRegistry.EXPECT().IsRegistered("test-executor").Return(true)

	err := s.builder.validateExecutorName("test-executor")

	s.Nil(err)
}

func (s *GraphBuilderTestSuite) TestBuildGraph_WithExecutorMode() {
	flow := &common.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []common.NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "task"},
			{
				ID:       "task",
				Type:     "TASK_EXECUTION",
				Executor: &common.ExecutorDefinition{Name: "SMSOTPAuthExecutor", Mode: "send"},
			},
		},
	}

	mockGraph := NewGraphInterfaceMock(s.T())
	mockStartNode := NewNodeInterfaceMock(s.T())
	mockTaskNode := NewExecutorBackedNodeInterfaceMock(s.T())

	s.mockFlowFactory.EXPECT().CreateGraph(
		"flow-1", common.FlowTypeAuthentication).Return(
		mockGraph)
	s.mockFlowFactory.EXPECT().CreateNode(
		"start", "START", map[string]interface{}(nil), false, false).Return(
		mockStartNode, nil)
	s.mockFlowFactory.EXPECT().CreateNode(
		"task", "TASK_EXECUTION", map[string]interface{}(nil), false, true).Return(
		mockTaskNode, nil)

	s.mockExecutorRegistry.EXPECT().IsRegistered("SMSOTPAuthExecutor").Return(true)
	mockTaskNode.EXPECT().SetExecutorName("SMSOTPAuthExecutor")
	mockTaskNode.EXPECT().SetMode("send") // Verify mode is set
	mockTaskNode.EXPECT().SetInputs([]common.Input{})

	mockGraph.EXPECT().AddNode(mockStartNode).Return(nil)
	mockGraph.EXPECT().AddNode(mockTaskNode).Return(nil)
	mockGraph.EXPECT().AddEdge("start", "task").Return(nil)
	mockGraph.EXPECT().GetNodes().Return(
		map[string]NodeInterface{"start": mockStartNode, "task": mockTaskNode})
	mockStartNode.EXPECT().GetType().Return(common.NodeTypeStart)
	mockTaskNode.EXPECT().GetType().Return(common.NodeTypeTaskExecution).Maybe()
	mockStartNode.EXPECT().GetID().Return("start")
	mockGraph.EXPECT().SetStartNode("start").Return(nil)

	graph, err := s.builder.buildGraph(context.Background(), flow)

	s.NotNil(graph)
	s.Nil(err)
}

func (s *GraphBuilderTestSuite) TestConfigureNodeExecutor_NilExecutor() {
	nodeDef := &common.NodeDefinition{
		ID:       "task",
		Type:     "TASK_EXECUTION",
		Executor: nil, // Nil executor
	}

	mockTaskNode := NewExecutorBackedNodeInterfaceMock(s.T())

	err := s.builder.configureNodeExecutor(context.Background(), nodeDef, mockTaskNode)

	s.Nil(err)
	// No mock expectations should be called
}

func (s *GraphBuilderTestSuite) TestConfigureNodeExecutor_EmptyExecutorName() {
	nodeDef := &common.NodeDefinition{
		ID:   "task",
		Type: "TASK_EXECUTION",
		Executor: &common.ExecutorDefinition{
			Name: "", // Empty executor name
			Mode: "send",
		},
	}

	mockTaskNode := NewExecutorBackedNodeInterfaceMock(s.T())

	err := s.builder.configureNodeExecutor(context.Background(), nodeDef, mockTaskNode)

	s.Nil(err)
	// No mock expectations should be called since name is empty
}

func (s *GraphBuilderTestSuite) TestConfigureNodeExecutor_NodeDoesNotSupportExecutors() {
	nodeDef := &common.NodeDefinition{
		ID:   "prompt",
		Type: "PROMPT",
		Executor: &common.ExecutorDefinition{
			Name: "test-executor",
		},
	}

	// Use a regular NodeInterface that doesn't support executors
	mockPromptNode := NewNodeInterfaceMock(s.T())

	// Should silently skip executor configuration for non-executor nodes
	err := s.builder.configureNodeExecutor(context.Background(), nodeDef, mockPromptNode)
	s.Nil(err)
}

func (s *GraphBuilderTestSuite) TestConfigureNodeExecutor_ExecutorNameValidationFails() {
	nodeDef := &common.NodeDefinition{
		ID:   "task",
		Type: "TASK_EXECUTION",
		Executor: &common.ExecutorDefinition{
			Name: "unregistered-executor",
		},
	}

	mockTaskNode := NewExecutorBackedNodeInterfaceMock(s.T())

	s.mockExecutorRegistry.EXPECT().IsRegistered("unregistered-executor").Return(false)

	err := s.builder.configureNodeExecutor(context.Background(), nodeDef, mockTaskNode)

	s.NotNil(err)
	s.Contains(err.Error(), "error while validating executor")
	s.Contains(err.Error(), "executor with name unregistered-executor not registered")
}

func (s *GraphBuilderTestSuite) TestConfigureNodeExecutor_WithModeSuccess() {
	nodeDef := &common.NodeDefinition{
		ID:   "task",
		Type: "TASK_EXECUTION",
		Executor: &common.ExecutorDefinition{
			Name: "test-executor",
			Mode: "verify",
		},
	}

	mockTaskNode := NewExecutorBackedNodeInterfaceMock(s.T())

	s.mockExecutorRegistry.EXPECT().IsRegistered("test-executor").Return(true)
	mockTaskNode.EXPECT().SetExecutorName("test-executor")
	mockTaskNode.EXPECT().SetMode("verify")

	err := s.builder.configureNodeExecutor(context.Background(), nodeDef, mockTaskNode)

	s.Nil(err)
}

func (s *GraphBuilderTestSuite) TestConfigureNodeExecutor_WithoutModeSuccess() {
	nodeDef := &common.NodeDefinition{
		ID:   "task",
		Type: "TASK_EXECUTION",
		Executor: &common.ExecutorDefinition{
			Name: "test-executor",
			Mode: "", // Empty mode - should not call SetMode
		},
	}

	mockTaskNode := NewExecutorBackedNodeInterfaceMock(s.T())

	s.mockExecutorRegistry.EXPECT().IsRegistered("test-executor").Return(true)
	mockTaskNode.EXPECT().SetExecutorName("test-executor")
	// SetMode should NOT be called when mode is empty

	err := s.builder.configureNodeExecutor(context.Background(), nodeDef, mockTaskNode)

	s.Nil(err)
}

func (s *GraphBuilderTestSuite) TestConfigureNodeExecutor_EmptyExecutorNameInValidation() {
	// This tests the validateExecutorName method with empty name
	err := s.builder.validateExecutorName("")

	s.NotNil(err)
	s.Contains(err.Error(), "executor name cannot be empty")
}

// Tests for display-only prompt node properties

func (s *GraphBuilderTestSuite) TestConfigureDisplayOnlyProperties_NoNextNodeDefined() {
	t := s.T()
	nodeDef := &common.NodeDefinition{
		ID:   "prompt-1",
		Type: "PROMPT",
		Next: "", // No next node
	}

	mockPromptNode := NewPromptNodeInterfaceMock(t)

	edges := map[string][]string{}

	err := s.builder.configureDisplayOnlyProperties(context.Background(), nodeDef, mockPromptNode, edges, nil)

	s.Nil(err)
	// SetNextNode should not be called
}

func (s *GraphBuilderTestSuite) TestConfigureDisplayOnlyProperties_WithNextNode() {
	t := s.T()
	nodeDef := &common.NodeDefinition{
		ID:   "prompt-1",
		Type: "PROMPT",
		Next: "next-node",
	}

	mockPromptNode := NewPromptNodeInterfaceMock(t)
	mockPromptNode.EXPECT().SetNextNode("next-node")

	edges := map[string][]string{}

	err := s.builder.configureDisplayOnlyProperties(context.Background(), nodeDef, mockPromptNode, edges, nil)

	s.Nil(err)
	// Verify edge is added
	s.Len(edges["prompt-1"], 1)
	s.Equal("next-node", edges["prompt-1"][0])
}

func (s *GraphBuilderTestSuite) TestConfigureDisplayOnlyProperties_WithMessage() {
	t := s.T()
	nodeDef := &common.NodeDefinition{
		ID:      "prompt-1",
		Type:    "PROMPT",
		Next:    "next-node",
		Message: "Please wait...",
	}

	mockPromptNode := NewPromptNodeInterfaceMock(t)
	mockPromptNode.EXPECT().SetNextNode("next-node")
	mockPromptNode.EXPECT().SetMessage("Please wait...")

	edges := map[string][]string{}

	err := s.builder.configureDisplayOnlyProperties(context.Background(), nodeDef, mockPromptNode, edges, nil)

	s.Nil(err)
	s.Len(edges["prompt-1"], 1)
	s.Equal("next-node", edges["prompt-1"][0])
}

func (s *GraphBuilderTestSuite) TestConfigureDisplayOnlyProperties_OnNonPromptNode() {
	t := s.T()
	nodeDef := &common.NodeDefinition{
		ID:   "task-1",
		Type: "TASK_EXECUTION",
		Next: "next-node", // Not allowed on non-prompt nodes
	}

	mockTaskNode := NewExecutorBackedNodeInterfaceMock(t)

	edges := map[string][]string{}

	err := s.builder.configureDisplayOnlyProperties(context.Background(), nodeDef, mockTaskNode, edges, nil)

	s.NotNil(err)
	s.Contains(err.Error(), "'next' field is only valid on PROMPT nodes")
}

func (s *GraphBuilderTestSuite) TestConfigureDisplayOnlyProperties_WithPromptsConflict() {
	t := s.T()
	nodeDef := &common.NodeDefinition{
		ID:   "prompt-1",
		Type: "PROMPT",
		Next: "next-node",
		Prompts: []common.PromptDefinition{
			{
				Inputs: []common.InputDefinition{
					{Identifier: "username"},
				},
			},
		},
	}

	mockPromptNode := NewPromptNodeInterfaceMock(t)

	edges := map[string][]string{}

	err := s.builder.configureDisplayOnlyProperties(context.Background(), nodeDef, mockPromptNode, edges, nil)

	s.NotNil(err)
	s.Contains(err.Error(), "has both 'prompts' and 'next'; these are mutually exclusive")
}

func (s *GraphBuilderTestSuite) TestProcessNode_IsFinalNode_WithNextField() {
	t := s.T()
	nodeDef := &common.NodeDefinition{
		ID:        "node-1",
		Type:      "PROMPT",
		OnSuccess: "",
		OnFailure: "",
		Next:      "next-node", // Has next
		Prompts:   []common.PromptDefinition{},
	}

	mockGraph := NewGraphInterfaceMock(t)
	mockPromptNode := NewPromptNodeInterfaceMock(t)

	// When Next is defined, isFinalNode should be false
	s.mockFlowFactory.EXPECT().CreateNode(
		"node-1", "PROMPT", nodeDef.Properties, false, false).
		Return(mockPromptNode, nil)

	mockPromptNode.EXPECT().SetNextNode("next-node")
	mockGraph.EXPECT().AddNode(mockPromptNode).Return(nil)

	allNodes := []common.NodeDefinition{*nodeDef}
	edges := map[string][]string{}

	err := s.builder.processNode(context.Background(), nodeDef, allNodes, mockGraph, edges, nil)

	s.Nil(err)
}

func (s *GraphBuilderTestSuite) TestConfigureNodePrompts_IncludesActionType() {
	t := s.T()
	nodeDef := &common.NodeDefinition{
		ID:   "prompt-1",
		Type: "PROMPT",
		Prompts: []common.PromptDefinition{
			{
				Inputs: []common.InputDefinition{
					{Identifier: "username"},
				},
				Action: &common.ActionDefinition{
					Ref:      "login",
					Type:     "password_auth",
					NextNode: "auth-node",
				},
			},
		},
	}

	mockPromptNode := NewPromptNodeInterfaceMock(t)
	mockPromptNode.EXPECT().SetPrompts(mock.MatchedBy(func(prompts []common.Prompt) bool {
		// Verify the action type is included
		if len(prompts) != 1 {
			return false
		}
		if prompts[0].Action == nil {
			return false
		}
		return prompts[0].Action.Type == "password_auth"
	}))

	edges := map[string][]string{}

	err := s.builder.configureNodePrompts(context.Background(), nodeDef, mockPromptNode, edges)

	s.Nil(err)
}

func (s *GraphBuilderTestSuite) TestConfigureNodePrompts_WithMultipleActionsWithTypes() {
	t := s.T()
	nodeDef := &common.NodeDefinition{
		ID:   "prompt-1",
		Type: "PROMPT",
		Prompts: []common.PromptDefinition{
			{
				Action: &common.ActionDefinition{
					Ref:      "google",
					Type:     "social_google",
					NextNode: "google-auth",
				},
			},
			{
				Action: &common.ActionDefinition{
					Ref:      "github",
					Type:     "social_github",
					NextNode: "github-auth",
				},
			},
		},
	}

	mockPromptNode := NewPromptNodeInterfaceMock(t)
	mockPromptNode.EXPECT().SetPrompts(mock.MatchedBy(func(prompts []common.Prompt) bool {
		// Verify both actions have their types
		if len(prompts) != 2 {
			return false
		}
		return (prompts[0].Action.Type == "social_google" &&
			prompts[1].Action.Type == "social_github")
	}))

	edges := map[string][]string{}

	err := s.builder.configureNodePrompts(context.Background(), nodeDef, mockPromptNode, edges)

	s.Nil(err)
}

func (s *GraphBuilderTestSuite) TestConfigureDisplayOnlyProperties_RecordsBoundary() {
	t := s.T()
	nodeDef := &common.NodeDefinition{
		ID:   "prompt-1",
		Type: "PROMPT",
		Next: "task-1",
	}
	mockPromptNode := NewPromptNodeInterfaceMock(t)
	mockPromptNode.EXPECT().SetNextNode("task-1")

	edges := map[string][]string{}
	boundaries := make([]segmentBoundary, 0)

	err := s.builder.configureDisplayOnlyProperties(context.Background(), nodeDef, mockPromptNode, edges, &boundaries)

	s.Nil(err)
	s.Len(boundaries, 1)
	s.Equal("prompt-1", boundaries[0].boundaryNodeID)
	s.Equal("task-1", boundaries[0].nextNodeID)
}

func (s *GraphBuilderTestSuite) TestConfigureDisplayOnlyProperties_RecordsMultipleBoundaries() {
	t := s.T()
	edges := map[string][]string{}
	boundaries := make([]segmentBoundary, 0)

	for _, tc := range []struct{ id, next string }{
		{"prompt-1", "task-1"},
		{"prompt-2", "task-2"},
	} {
		nodeDef := &common.NodeDefinition{ID: tc.id, Type: "PROMPT", Next: tc.next}
		mockPN := NewPromptNodeInterfaceMock(t)
		mockPN.EXPECT().SetNextNode(tc.next)
		err := s.builder.configureDisplayOnlyProperties(context.Background(), nodeDef, mockPN, edges, &boundaries)
		s.Nil(err)
	}

	s.Len(boundaries, 2)
	s.Equal("prompt-1", boundaries[0].boundaryNodeID)
	s.Equal("prompt-2", boundaries[1].boundaryNodeID)
}

func (s *GraphBuilderTestSuite) TestConfigureDisplayOnlyProperties_NilBoundariesDoesNotPanic() {
	t := s.T()
	nodeDef := &common.NodeDefinition{ID: "prompt-1", Type: "PROMPT", Next: "task-1"}
	mockPromptNode := NewPromptNodeInterfaceMock(t)
	mockPromptNode.EXPECT().SetNextNode("task-1")

	edges := map[string][]string{}

	// nil boundaries must not panic
	err := s.builder.configureDisplayOnlyProperties(context.Background(), nodeDef, mockPromptNode, edges, nil)
	s.Nil(err)
}

func (s *GraphBuilderTestSuite) TestComputeSegments_NoBoundaries() {
	// computeSegments returns early with no boundaries; SetSegments/GetStartNode must NOT be called
	mockGraph := NewGraphInterfaceMock(s.T())

	s.builder.computeSegments(mockGraph, []segmentBoundary{})
}

func (s *GraphBuilderTestSuite) TestComputeSegments_OneBoundary() {
	t := s.T()
	mockGraph := NewGraphInterfaceMock(t)
	mockStartNode := NewNodeInterfaceMock(t)
	mockStartNode.On("GetID").Return("node-start")
	mockGraph.On("GetStartNode").Return(mockStartNode, nil)
	mockGraph.On("SetSegments", []Segment{
		{ID: "seg-0", StartNodeID: "node-start"},
		{ID: "seg-1", StartNodeID: "node-task"},
	}).Return()

	s.builder.computeSegments(mockGraph, []segmentBoundary{
		{boundaryNodeID: "node-prompt", nextNodeID: "node-task"},
	})
}

func (s *GraphBuilderTestSuite) TestComputeSegments_TwoBoundaries() {
	t := s.T()
	mockGraph := NewGraphInterfaceMock(t)
	mockStartNode := NewNodeInterfaceMock(t)
	mockStartNode.On("GetID").Return("node-start")
	mockGraph.On("GetStartNode").Return(mockStartNode, nil)
	mockGraph.On("SetSegments", []Segment{
		{ID: "seg-0", StartNodeID: "node-start"},
		{ID: "seg-1", StartNodeID: "node-task-1"},
		{ID: "seg-2", StartNodeID: "node-task-2"},
	}).Return()

	s.builder.computeSegments(mockGraph, []segmentBoundary{
		{boundaryNodeID: "node-prompt-1", nextNodeID: "node-task-1"},
		{boundaryNodeID: "node-prompt-2", nextNodeID: "node-task-2"},
	})
}

func (s *GraphBuilderTestSuite) TestComputeSegments_GetStartNodeFails() {
	t := s.T()
	mockGraph := NewGraphInterfaceMock(t)
	mockGraph.On("GetStartNode").Return(nil, errors.New("start node not set"))

	// SetSegments must NOT be called when GetStartNode fails
	s.builder.computeSegments(mockGraph, []segmentBoundary{
		{boundaryNodeID: "prompt", nextNodeID: "task"},
	})
}

// Prompt inputs' regex rules must be compiled at graph-build time so the
// request-path validator never recompiles.
func (s *GraphBuilderTestSuite) TestConfigureNodePrompts_ValidationRulesCompiled() {
	nodeDef := &common.NodeDefinition{
		ID:   "prompt-1",
		Type: "PROMPT",
		Prompts: []common.PromptDefinition{
			{
				Inputs: []common.InputDefinition{
					{
						Identifier: "password",
						Validation: []common.ValidationRuleDefinition{
							{Type: "regex", Value: "[A-Z]+", Message: "needs.upper"},
						},
					},
				},
				Action: &common.ActionDefinition{Ref: "submit", NextNode: "next"},
			},
		},
	}

	mockPromptNode := NewPromptNodeInterfaceMock(s.T())
	mockPromptNode.EXPECT().SetPrompts(mock.MatchedBy(func(prompts []common.Prompt) bool {
		if len(prompts) != 1 || len(prompts[0].Inputs) != 1 || len(prompts[0].Inputs[0].Validation) != 1 {
			return false
		}
		rule := prompts[0].Inputs[0].Validation[0]
		return rule.Type == common.ValidationTypeRegex && rule.CompiledRegex != nil
	}))

	err := s.builder.configureNodePrompts(context.Background(), nodeDef, mockPromptNode, map[string][]string{})
	s.Nil(err)
}

// Invalid regex on a prompt input must fail the build with a useful error message.
func (s *GraphBuilderTestSuite) TestConfigureNodePrompts_InvalidRegexFailsBuild() {
	nodeDef := &common.NodeDefinition{
		ID:   "prompt-1",
		Type: "PROMPT",
		Prompts: []common.PromptDefinition{
			{
				Inputs: []common.InputDefinition{
					{
						Identifier: "password",
						Validation: []common.ValidationRuleDefinition{
							{Type: "regex", Value: "[unterminated", Message: "bad"},
						},
					},
				},
				Action: &common.ActionDefinition{Ref: "submit", NextNode: "next"},
			},
		},
	}

	mockPromptNode := NewPromptNodeInterfaceMock(s.T())
	// SetPrompts must NOT be called.

	err := s.builder.configureNodePrompts(context.Background(), nodeDef, mockPromptNode, map[string][]string{})
	s.NotNil(err)
	s.Contains(err.Error(), "prompt-1")
	s.Contains(err.Error(), "password")
	s.Contains(err.Error(), "invalid validation regex")
}
