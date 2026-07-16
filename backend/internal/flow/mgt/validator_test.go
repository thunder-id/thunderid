/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package flowmgt

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/flow/executormock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/interceptormock"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// minimalValidNodes returns a minimal valid 3-node flow: START -> TASK_EXECUTION -> END.
func minimalValidNodes() []providers.NodeDefinition {
	return []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart), OnSuccess: "task"},
		{
			ID: "task", Type: string(common.NodeTypeTaskExecution),
			Executor:  &providers.ExecutorDefinition{Name: "test-executor"},
			OnSuccess: "end",
		},
		{ID: "end", Type: string(common.NodeTypeEnd)},
	}
}

// minimalValidFlow returns a minimal valid FlowDefinition.
func minimalValidFlow() *FlowDefinition {
	return &FlowDefinition{
		Handle:   "test-flow",
		Name:     "Test Flow",
		FlowType: providers.FlowTypeRecovery,
		Nodes:    minimalValidNodes(),
	}
}

// ---------------------------------------------------------------------------
// ValidatorTestSuite
// ---------------------------------------------------------------------------

type ValidatorTestSuite struct {
	suite.Suite
	v                       *flowValidator
	mockExecutorRegistry    *executormock.ExecutorRegistryInterfaceMock
	mockInterceptorRegistry *interceptormock.InterceptorRegistryInterfaceMock
	mockGraphBuilder        *graphBuilderInterfaceMock
}

func TestValidatorTestSuite(t *testing.T) {
	suite.Run(t, new(ValidatorTestSuite))
}

func (s *ValidatorTestSuite) SetupTest() {
	s.mockExecutorRegistry = executormock.NewExecutorRegistryInterfaceMock(s.T())
	s.mockInterceptorRegistry = interceptormock.NewInterceptorRegistryInterfaceMock(s.T())
	s.mockGraphBuilder = newGraphBuilderInterfaceMock(s.T())
	s.v = &flowValidator{
		executorRegistry:    s.mockExecutorRegistry,
		interceptorRegistry: s.mockInterceptorRegistry,
		graphBuilder:        s.mockGraphBuilder,
	}
}

// ---------------------------------------------------------------------------
// isValidHandleFormat
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestIsValidHandleFormat_ValidHandles() {
	valid := []string{
		"a",
		"z",
		"0",
		"9",
		"abc",
		"basic-login",
		"basic_login",
		"my-flow-123",
		"a1b2c3",
	}
	for _, h := range valid {
		s.True(isValidHandleFormat(h), "expected %q to be valid", h)
	}
}

func (s *ValidatorTestSuite) TestIsValidHandleFormat_InvalidHandles() {
	invalid := []string{
		"",
		"-start",
		"end-",
		"_start",
		"start_",
		"Has-Upper",
		"with space",
		"with.dot",
		"with@symbol",
	}
	for _, h := range invalid {
		s.False(isValidHandleFormat(h), "expected %q to be invalid", h)
	}
}

// ---------------------------------------------------------------------------
// isValidFlowType
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestIsValidFlowType_ValidTypes() {
	valid := []providers.FlowType{
		providers.FlowTypeAuthentication,
		providers.FlowTypeRegistration,
		providers.FlowTypeUserOnboarding,
		providers.FlowTypeRecovery,
	}
	for _, ft := range valid {
		s.True(isValidFlowType(ft), "expected %q to be valid", ft)
	}
}

func (s *ValidatorTestSuite) TestIsValidFlowType_InvalidTypes() {
	invalid := []providers.FlowType{
		"",
		"UNKNOWN",
		"authentication",
	}
	for _, ft := range invalid {
		s.False(isValidFlowType(ft), "expected %q to be invalid", ft)
	}
}

// ---------------------------------------------------------------------------
// validateMetadata
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateMetadata_NilFlowDef() {
	err := s.v.validateMetadata(nil)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidRequestFormat.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateMetadata_MissingHandle() {
	fd := minimalValidFlow()
	fd.Handle = ""
	err := s.v.validateMetadata(fd)
	s.Require().NotNil(err)
	s.Equal(ErrorMissingFlowHandle.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateMetadata_InvalidHandleFormat() {
	fd := minimalValidFlow()
	fd.Handle = "-invalid"
	err := s.v.validateMetadata(fd)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowHandleFormat.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateMetadata_MissingName() {
	fd := minimalValidFlow()
	fd.Name = ""
	err := s.v.validateMetadata(fd)
	s.Require().NotNil(err)
	s.Equal(ErrorMissingFlowName.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateMetadata_InvalidFlowType() {
	fd := minimalValidFlow()
	fd.FlowType = "UNKNOWN"
	err := s.v.validateMetadata(fd)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowType.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateMetadata_InvalidIDFormat() {
	fd := minimalValidFlow()
	fd.ID = "not-a-uuid"
	err := s.v.validateMetadata(fd)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowIDFormat.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateMetadata_ValidID() {
	fd := minimalValidFlow()
	fd.ID = "550e8400-e29b-41d4-a716-446655440000"
	err := s.v.validateMetadata(fd)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateMetadata_TooFewNodes_One() {
	fd := minimalValidFlow()
	fd.Nodes = []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart)},
	}
	err := s.v.validateMetadata(fd)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateMetadata_TooFewNodes_Two() {
	fd := minimalValidFlow()
	fd.Nodes = []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart)},
		{ID: "end", Type: string(common.NodeTypeEnd)},
	}
	err := s.v.validateMetadata(fd)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateMetadata_Valid() {
	err := s.v.validateMetadata(minimalValidFlow())
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// buildNodeIndex
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestBuildNodeIndex_DuplicateID() {
	nodes := []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart)},
		{ID: "start", Type: string(common.NodeTypeEnd)},
	}
	_, err := buildNodeIndex(nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowStructure.Code, err.Code)
}

func (s *ValidatorTestSuite) TestBuildNodeIndex_UniqueIDs() {
	nodes := minimalValidNodes()
	index, err := buildNodeIndex(nodes)
	s.Nil(err)
	s.Len(index, 3)
}

// ---------------------------------------------------------------------------
// validateNodeTypesAndCardinality
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateNodeTypesAndCardinality_InvalidType() {
	nodes := []providers.NodeDefinition{
		{ID: "start", Type: "INVALID"},
	}
	err := s.v.validateNodeTypesAndCardinality(nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeConfig.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateNodeTypesAndCardinality_MissingStart() {
	nodes := []providers.NodeDefinition{
		{ID: "task", Type: string(common.NodeTypeTaskExecution)},
		{ID: "end", Type: string(common.NodeTypeEnd)},
	}
	err := s.v.validateNodeTypesAndCardinality(nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowStructure.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateNodeTypesAndCardinality_DuplicateStart() {
	nodes := []providers.NodeDefinition{
		{ID: "start1", Type: string(common.NodeTypeStart)},
		{ID: "start2", Type: string(common.NodeTypeStart)},
		{ID: "end", Type: string(common.NodeTypeEnd)},
	}
	err := s.v.validateNodeTypesAndCardinality(nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowStructure.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateNodeTypesAndCardinality_MissingEnd() {
	nodes := []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart)},
		{ID: "task", Type: string(common.NodeTypeTaskExecution)},
	}
	err := s.v.validateNodeTypesAndCardinality(nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowStructure.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateNodeTypesAndCardinality_DuplicateEnd() {
	nodes := []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart)},
		{ID: "end1", Type: string(common.NodeTypeEnd)},
		{ID: "end2", Type: string(common.NodeTypeEnd)},
	}
	err := s.v.validateNodeTypesAndCardinality(nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowStructure.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateNodeTypesAndCardinality_Valid() {
	err := s.v.validateNodeTypesAndCardinality(minimalValidNodes())
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// validateNodeReferences
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateNodeReferences_InvalidOnSuccess() {
	nodes := minimalValidNodes()
	index, _ := buildNodeIndex(nodes)
	refs := []nodeReference{
		{sourceNodeID: "start", targetNodeID: "nonexistent", fieldName: "onSuccess"},
	}
	err := s.v.validateNodeReferences(refs, index)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeReference.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateNodeReferences_Valid() {
	nodes := minimalValidNodes()
	index, _ := buildNodeIndex(nodes)
	refs := collectAllNodeReferences(nodes)
	err := s.v.validateNodeReferences(refs, index)
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// validateReachability
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateReachability_OrphanedNode() {
	nodes := []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart), OnSuccess: "task"},
		{
			ID: "task", Type: string(common.NodeTypeTaskExecution),
			Executor:  &providers.ExecutorDefinition{Name: "exec"},
			OnSuccess: "end",
		},
		{ID: "end", Type: string(common.NodeTypeEnd)},
		// orphan: not reachable from start
		{ID: "orphan", Type: string(common.NodeTypePrompt), Next: "end"},
	}
	err := s.v.validateReachability(nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowStructure.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateReachability_AllReachable() {
	err := s.v.validateReachability(minimalValidNodes())
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// validateTermination
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateTermination_CycleWithNoPathToEnd() {
	// start -> a -> b -> a (cycle, no path to end)
	nodes := []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart), OnSuccess: "a"},
		{
			ID: "a", Type: string(common.NodeTypeTaskExecution),
			Executor:  &providers.ExecutorDefinition{Name: "exec"},
			OnSuccess: "b",
		},
		{
			ID: "b", Type: string(common.NodeTypeTaskExecution),
			Executor:  &providers.ExecutorDefinition{Name: "exec"},
			OnSuccess: "a", // cycle back, never reaches end
		},
		{ID: "end", Type: string(common.NodeTypeEnd)},
	}
	err := s.v.validateTermination(nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowStructure.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateTermination_AllReachEnd() {
	err := s.v.validateTermination(minimalValidNodes())
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// validateStructure
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateStructure_Valid() {
	_, err := s.v.validateStructure(minimalValidNodes())
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateStructure_InvalidReference() {
	nodes := []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart), OnSuccess: "nonexistent"},
		{
			ID: "task", Type: string(common.NodeTypeTaskExecution),
			Executor:  &providers.ExecutorDefinition{Name: "exec"},
			OnSuccess: "end",
		},
		{ID: "end", Type: string(common.NodeTypeEnd)},
	}
	_, err := s.v.validateStructure(nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeReference.Code, err.Code)
}

// ---------------------------------------------------------------------------
// validateStartNode
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateStartNode_MissingOnSuccess() {
	node := &providers.NodeDefinition{ID: "start", Type: string(common.NodeTypeStart)}
	err := s.v.validateStartNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateStartNode_HasExecutor() {
	node := &providers.NodeDefinition{
		ID:        "start",
		Type:      string(common.NodeTypeStart),
		OnSuccess: "task",
		Executor:  &providers.ExecutorDefinition{Name: "exec"},
	}
	err := s.v.validateStartNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateStartNode_HasPrompts() {
	node := &providers.NodeDefinition{
		ID:        "start",
		Type:      string(common.NodeTypeStart),
		OnSuccess: "task",
		Prompts:   []providers.PromptDefinition{{Action: &providers.ActionDefinition{NextNode: "task"}}},
	}
	err := s.v.validateStartNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateStartNode_HasOnFailure() {
	node := &providers.NodeDefinition{
		ID:        "start",
		Type:      string(common.NodeTypeStart),
		OnSuccess: "task",
		OnFailure: "end",
	}
	err := s.v.validateStartNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateStartNode_HasOnIncomplete() {
	node := &providers.NodeDefinition{
		ID:           "start",
		Type:         string(common.NodeTypeStart),
		OnSuccess:    "task",
		OnIncomplete: "prompt",
	}
	err := s.v.validateStartNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateStartNode_Valid() {
	node := &providers.NodeDefinition{
		ID:        "start",
		Type:      string(common.NodeTypeStart),
		OnSuccess: "task",
	}
	err := s.v.validateStartNode(node)
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// validateEndNode
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateEndNode_HasOnSuccess() {
	node := &providers.NodeDefinition{ID: "end", Type: string(common.NodeTypeEnd), OnSuccess: "task"}
	err := s.v.validateEndNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateEndNode_HasOnFailure() {
	node := &providers.NodeDefinition{ID: "end", Type: string(common.NodeTypeEnd), OnFailure: "task"}
	err := s.v.validateEndNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateEndNode_HasOnIncomplete() {
	node := &providers.NodeDefinition{ID: "end", Type: string(common.NodeTypeEnd), OnIncomplete: "task"}
	err := s.v.validateEndNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateEndNode_HasExecutor() {
	node := &providers.NodeDefinition{
		ID:       "end",
		Type:     string(common.NodeTypeEnd),
		Executor: &providers.ExecutorDefinition{Name: "exec"},
	}
	err := s.v.validateEndNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateEndNode_HasPrompts() {
	node := &providers.NodeDefinition{
		ID:      "end",
		Type:    string(common.NodeTypeEnd),
		Prompts: []providers.PromptDefinition{{Action: &providers.ActionDefinition{NextNode: "x"}}},
	}
	err := s.v.validateEndNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateEndNode_HasNext() {
	node := &providers.NodeDefinition{ID: "end", Type: string(common.NodeTypeEnd), Next: "task"}
	err := s.v.validateEndNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateEndNode_Valid() {
	node := &providers.NodeDefinition{ID: "end", Type: string(common.NodeTypeEnd)}
	err := s.v.validateEndNode(node)
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// validateTaskExecutionNode
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateTaskExecutionNode_NilExecutor() {
	nodes := minimalValidNodes()
	index, _ := buildNodeIndex(nodes)
	node := &providers.NodeDefinition{ID: "task", Type: string(common.NodeTypeTaskExecution), OnSuccess: "end"}
	err := s.v.validateTaskExecutionNode(node, index)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeConfig.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateTaskExecutionNode_EmptyExecutorName() {
	nodes := minimalValidNodes()
	index, _ := buildNodeIndex(nodes)
	node := &providers.NodeDefinition{
		ID:        "task",
		Type:      string(common.NodeTypeTaskExecution),
		Executor:  &providers.ExecutorDefinition{Name: ""},
		OnSuccess: "end",
	}
	err := s.v.validateTaskExecutionNode(node, index)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeConfig.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateTaskExecutionNode_MissingOnSuccess() {
	nodes := minimalValidNodes()
	index, _ := buildNodeIndex(nodes)
	node := &providers.NodeDefinition{
		ID:       "task",
		Type:     string(common.NodeTypeTaskExecution),
		Executor: &providers.ExecutorDefinition{Name: "exec"},
	}
	err := s.v.validateTaskExecutionNode(node, index)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeConfig.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateTaskExecutionNode_OnFailurePointsToNonPrompt() {
	nodes := []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart), OnSuccess: "task"},
		{
			ID: "task", Type: string(common.NodeTypeTaskExecution),
			Executor:  &providers.ExecutorDefinition{Name: "exec"},
			OnSuccess: "end",
			OnFailure: "end", // END is not a PROMPT node
		},
		{ID: "end", Type: string(common.NodeTypeEnd)},
	}
	index, _ := buildNodeIndex(nodes)
	node := &nodes[1]
	err := s.v.validateTaskExecutionNode(node, index)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeConfig.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateTaskExecutionNode_OnIncompletePointsToNonPrompt() {
	nodes := []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart), OnSuccess: "task"},
		{
			ID: "task", Type: string(common.NodeTypeTaskExecution),
			Executor:     &providers.ExecutorDefinition{Name: "exec"},
			OnSuccess:    "end",
			OnIncomplete: "end", // END is not a PROMPT node
		},
		{ID: "end", Type: string(common.NodeTypeEnd)},
	}
	index, _ := buildNodeIndex(nodes)
	node := &nodes[1]
	err := s.v.validateTaskExecutionNode(node, index)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeConfig.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateTaskExecutionNode_OnFailurePointsToPrompt() {
	nodes := []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart), OnSuccess: "task"},
		{
			ID: "task", Type: string(common.NodeTypeTaskExecution),
			Executor:  &providers.ExecutorDefinition{Name: "exec"},
			OnSuccess: "end",
			OnFailure: "prompt",
		},
		{ID: "prompt", Type: string(common.NodeTypePrompt), Prompts: []providers.PromptDefinition{
			{Action: &providers.ActionDefinition{NextNode: "end"}},
		}},
		{ID: "end", Type: string(common.NodeTypeEnd)},
	}
	index, _ := buildNodeIndex(nodes)
	node := &nodes[1]
	err := s.v.validateTaskExecutionNode(node, index)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateTaskExecutionNode_Valid() {
	nodes := minimalValidNodes()
	index, _ := buildNodeIndex(nodes)
	node := &nodes[1]
	err := s.v.validateTaskExecutionNode(node, index)
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// validatePromptNode
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidatePromptNode_BothPromptsAndNext() {
	node := &providers.NodeDefinition{
		ID:   "prompt",
		Type: string(common.NodeTypePrompt),
		Prompts: []providers.PromptDefinition{
			{Action: &providers.ActionDefinition{NextNode: "end"}},
		},
		Next: "end",
	}
	err := s.v.validatePromptNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeConfig.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidatePromptNode_NeitherPromptsNorNext() {
	node := &providers.NodeDefinition{ID: "prompt", Type: string(common.NodeTypePrompt)}
	err := s.v.validatePromptNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeConfig.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidatePromptNode_DisplayOnly_Valid() {
	node := &providers.NodeDefinition{
		ID:   "prompt",
		Type: string(common.NodeTypePrompt),
		Next: "end",
	}
	err := s.v.validatePromptNode(node)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidatePromptNode_Interactive_MissingAction() {
	node := &providers.NodeDefinition{
		ID:   "prompt",
		Type: string(common.NodeTypePrompt),
		Prompts: []providers.PromptDefinition{
			{Action: nil}, // no action
		},
	}
	err := s.v.validatePromptNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeConfig.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidatePromptNode_Interactive_EmptyNextNode() {
	node := &providers.NodeDefinition{
		ID:   "prompt",
		Type: string(common.NodeTypePrompt),
		Prompts: []providers.PromptDefinition{
			{Action: &providers.ActionDefinition{NextNode: ""}},
		},
	}
	err := s.v.validatePromptNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeConfig.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidatePromptNode_Interactive_Valid() {
	node := &providers.NodeDefinition{
		ID:   "prompt",
		Type: string(common.NodeTypePrompt),
		Prompts: []providers.PromptDefinition{
			{Action: &providers.ActionDefinition{NextNode: "end"}},
		},
	}
	err := s.v.validatePromptNode(node)
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// validateCallNode
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateCallNode_Valid() {
	node := &providers.NodeDefinition{
		ID:        "call-sub",
		Type:      string(common.NodeTypeCall),
		Flow:      &providers.FlowReferenceDefinition{Ref: "sub-flow-id"},
		OnSuccess: "end",
	}
	err := s.v.validateCallNode(node)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateCallNode_ValidWithOnFailure() {
	node := &providers.NodeDefinition{
		ID:        "call-sub",
		Type:      string(common.NodeTypeCall),
		Flow:      &providers.FlowReferenceDefinition{Ref: "sub-flow-id"},
		OnSuccess: "end",
		OnFailure: "error-handler",
	}
	err := s.v.validateCallNode(node)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateCallNode_MissingFlowRef() {
	node := &providers.NodeDefinition{
		ID:        "call-sub",
		Type:      string(common.NodeTypeCall),
		OnSuccess: "end",
	}
	err := s.v.validateCallNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeConfig.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "flow reference")
}

func (s *ValidatorTestSuite) TestValidateCallNode_EmptyFlowRef() {
	node := &providers.NodeDefinition{
		ID:        "call-sub",
		Type:      string(common.NodeTypeCall),
		Flow:      &providers.FlowReferenceDefinition{Ref: ""},
		OnSuccess: "end",
	}
	err := s.v.validateCallNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeConfig.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "flow reference")
}

func (s *ValidatorTestSuite) TestValidateCallNode_MissingOnSuccess() {
	node := &providers.NodeDefinition{
		ID:   "call-sub",
		Type: string(common.NodeTypeCall),
		Flow: &providers.FlowReferenceDefinition{Ref: "sub-flow-id"},
	}
	err := s.v.validateCallNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeConfig.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "onSuccess")
}

func (s *ValidatorTestSuite) TestValidateCallNode_HasExecutor() {
	node := &providers.NodeDefinition{
		ID:        "call-sub",
		Type:      string(common.NodeTypeCall),
		Flow:      &providers.FlowReferenceDefinition{Ref: "sub-flow-id"},
		OnSuccess: "end",
		Executor:  &providers.ExecutorDefinition{Name: "some-executor"},
	}
	err := s.v.validateCallNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeConfig.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "must not have an executor")
}

func (s *ValidatorTestSuite) TestValidateCallNode_HasPrompts() {
	node := &providers.NodeDefinition{
		ID:        "call-sub",
		Type:      string(common.NodeTypeCall),
		Flow:      &providers.FlowReferenceDefinition{Ref: "sub-flow-id"},
		OnSuccess: "end",
		Prompts:   []providers.PromptDefinition{{Action: &providers.ActionDefinition{NextNode: "x"}}},
	}
	err := s.v.validateCallNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeConfig.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "must not have prompts")
}

func (s *ValidatorTestSuite) TestValidateCallNode_HasNext() {
	node := &providers.NodeDefinition{
		ID:        "call-sub",
		Type:      string(common.NodeTypeCall),
		Flow:      &providers.FlowReferenceDefinition{Ref: "sub-flow-id"},
		OnSuccess: "end",
		Next:      "some-node",
	}
	err := s.v.validateCallNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeConfig.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "must not have next")
}

func (s *ValidatorTestSuite) TestValidateCallNode_HasOnIncomplete() {
	node := &providers.NodeDefinition{
		ID:           "call-sub",
		Type:         string(common.NodeTypeCall),
		Flow:         &providers.FlowReferenceDefinition{Ref: "sub-flow-id"},
		OnSuccess:    "end",
		OnIncomplete: "some-node",
	}
	err := s.v.validateCallNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeConfig.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "must not have onIncomplete")
}

// ---------------------------------------------------------------------------
// validateInputDefinitions
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateInputDefinitions_InvalidInputType() {
	inputs := []providers.InputDefinition{
		{Type: "UNSUPPORTED_TYPE", Identifier: "field1"},
	}
	err := s.v.validateInputDefinitions("prompt", inputs)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidInputConfig.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateInputDefinitions_InvalidValidationRuleType() {
	inputs := []providers.InputDefinition{
		{
			Type:       providers.InputTypeText,
			Identifier: "field1",
			Validation: []providers.ValidationRuleDefinition{
				{Type: "unknownRule", Value: "something"},
			},
		},
	}
	err := s.v.validateInputDefinitions("prompt", inputs)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidInputConfig.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateInputDefinitions_RegexRule_NonStringValue() {
	inputs := []providers.InputDefinition{
		{
			Type:       providers.InputTypeText,
			Identifier: "field1",
			Validation: []providers.ValidationRuleDefinition{
				{Type: string(providers.ValidationTypeRegex), Value: 42}, // not a string
			},
		},
	}
	err := s.v.validateInputDefinitions("prompt", inputs)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidInputConfig.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateInputDefinitions_RegexRule_InvalidPattern() {
	inputs := []providers.InputDefinition{
		{
			Type:       providers.InputTypeText,
			Identifier: "field1",
			Validation: []providers.ValidationRuleDefinition{
				{Type: string(providers.ValidationTypeRegex), Value: "[invalid(regex"},
			},
		},
	}
	err := s.v.validateInputDefinitions("prompt", inputs)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidInputConfig.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateInputDefinitions_RegexRule_ValidPattern() {
	inputs := []providers.InputDefinition{
		{
			Type:       providers.InputTypeEmail,
			Identifier: "email",
			Validation: []providers.ValidationRuleDefinition{
				{Type: string(providers.ValidationTypeRegex), Value: `^[a-z]+$`},
			},
		},
	}
	err := s.v.validateInputDefinitions("prompt", inputs)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateInputDefinitions_EmptyType_IsAllowed() {
	// Empty type is not validated against valid types (only non-empty types are checked).
	inputs := []providers.InputDefinition{
		{Type: "", Identifier: "field1"},
	}
	err := s.v.validateInputDefinitions("prompt", inputs)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateInputDefinitions_MinLengthRule_Valid() {
	inputs := []providers.InputDefinition{
		{
			Type:       providers.InputTypePassword,
			Identifier: "password",
			Validation: []providers.ValidationRuleDefinition{
				{Type: string(providers.ValidationTypeMinLength), Value: 8},
			},
		},
	}
	err := s.v.validateInputDefinitions("prompt", inputs)
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// validateValidationRules
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateValidationRules_InvalidRuleType() {
	rules := []providers.ValidationRuleDefinition{
		{Type: "unknownRule", Value: "something"},
	}
	err := s.v.validateValidationRules("prompt", "field1", rules)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidInputConfig.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "invalid validation rule type")
	s.Equal("unknownRule", err.ErrorDescription.Params["ruleType"])
}

func (s *ValidatorTestSuite) TestValidateValidationRules_RegexNonStringValue() {
	rules := []providers.ValidationRuleDefinition{
		{Type: string(providers.ValidationTypeRegex), Value: 42},
	}
	err := s.v.validateValidationRules("prompt", "field1", rules)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidInputConfig.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "regex validation rule value must be a string")
}

func (s *ValidatorTestSuite) TestValidateValidationRules_RegexInvalidPattern() {
	rules := []providers.ValidationRuleDefinition{
		{Type: string(providers.ValidationTypeRegex), Value: "[invalid(regex"},
	}
	err := s.v.validateValidationRules("prompt", "field1", rules)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidInputConfig.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "invalid regex pattern")
}

func (s *ValidatorTestSuite) TestValidateValidationRules_RegexValidPattern() {
	rules := []providers.ValidationRuleDefinition{
		{Type: string(providers.ValidationTypeRegex), Value: `^[a-z]+$`},
	}
	err := s.v.validateValidationRules("prompt", "field1", rules)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateValidationRules_MinLengthValid() {
	rules := []providers.ValidationRuleDefinition{
		{Type: string(providers.ValidationTypeMinLength), Value: 8},
	}
	err := s.v.validateValidationRules("prompt", "password", rules)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateValidationRules_EmptyRules() {
	err := s.v.validateValidationRules("prompt", "field1", nil)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateValidationRules_MultipleRulesFirstInvalid() {
	rules := []providers.ValidationRuleDefinition{
		{Type: "badType", Value: "x"},
		{Type: string(providers.ValidationTypeMinLength), Value: 5},
	}
	err := s.v.validateValidationRules("prompt", "field1", rules)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidInputConfig.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateValidationRules_MultipleValidRules() {
	rules := []providers.ValidationRuleDefinition{
		{Type: string(providers.ValidationTypeMinLength), Value: 3},
		{Type: string(providers.ValidationTypeRegex), Value: `^[a-z]+$`},
	}
	err := s.v.validateValidationRules("prompt", "field1", rules)
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// validateInterceptorFormat
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateInterceptorFormat_MissingName() {
	ic := providers.InterceptorDefinition{
		Name: "",
		Mode: providers.InterceptorModePreRequest,
	}
	err := s.v.validateInterceptorFormat(0, ic)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateInterceptorFormat_DefaultInterceptor() {
	// Temporarily populate DefaultInterceptorNames to test the check.
	original := interceptor.DefaultInterceptorNames
	interceptor.DefaultInterceptorNames = map[string]struct{}{"MyDefaultInterceptor": {}}
	defer func() { interceptor.DefaultInterceptorNames = original }()

	ic := providers.InterceptorDefinition{
		Name: "MyDefaultInterceptor",
		Mode: providers.InterceptorModePreRequest,
	}
	err := s.v.validateInterceptorFormat(0, ic)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateInterceptorFormat_InvalidMode() {
	ic := providers.InterceptorDefinition{
		Name: "my-interceptor",
		Mode: "INVALID_MODE",
	}
	err := s.v.validateInterceptorFormat(0, ic)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateInterceptorFormat_InvalidScope() {
	ic := providers.InterceptorDefinition{
		Name:  "my-interceptor",
		Mode:  providers.InterceptorModePreRequest,
		Scope: "INVALID_SCOPE",
	}
	err := s.v.validateInterceptorFormat(0, ic)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateInterceptorFormat_SelectedScopeWithoutApplyTo() {
	ic := providers.InterceptorDefinition{
		Name:    "my-interceptor",
		Mode:    providers.InterceptorModePreRequest,
		Scope:   providers.InterceptorScopeSelected,
		ApplyTo: nil,
	}
	err := s.v.validateInterceptorFormat(0, ic)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateInterceptorFormat_Valid_NoScope() {
	ic := providers.InterceptorDefinition{
		Name: "my-interceptor",
		Mode: providers.InterceptorModePreRequest,
	}
	err := s.v.validateInterceptorFormat(0, ic)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateInterceptorFormat_Valid_AllScope() {
	ic := providers.InterceptorDefinition{
		Name:  "my-interceptor",
		Mode:  providers.InterceptorModePreNode,
		Scope: providers.InterceptorScopeAll,
	}
	err := s.v.validateInterceptorFormat(0, ic)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateInterceptorFormat_Valid_SelectedScope() {
	ic := providers.InterceptorDefinition{
		Name:    "my-interceptor",
		Mode:    providers.InterceptorModePostNode,
		Scope:   providers.InterceptorScopeSelected,
		ApplyTo: []string{"task"},
	}
	err := s.v.validateInterceptorFormat(0, ic)
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// validateInterceptorApplyTo
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateInterceptorApplyTo_InvalidNodeReference() {
	nodes := minimalValidNodes()
	index, _ := buildNodeIndex(nodes)
	interceptors := []providers.InterceptorDefinition{
		{
			Name:    "my-interceptor",
			Mode:    providers.InterceptorModePreNode,
			Scope:   providers.InterceptorScopeSelected,
			ApplyTo: []string{"nonexistent"},
		},
	}
	err := s.v.validateInterceptorApplyTo(interceptors, index)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeReference.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateInterceptorApplyTo_ValidReference() {
	nodes := minimalValidNodes()
	index, _ := buildNodeIndex(nodes)
	interceptors := []providers.InterceptorDefinition{
		{
			Name:    "my-interceptor",
			Mode:    providers.InterceptorModePreNode,
			Scope:   providers.InterceptorScopeSelected,
			ApplyTo: []string{"task"},
		},
	}
	err := s.v.validateInterceptorApplyTo(interceptors, index)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateInterceptorApplyTo_NonSelectedScopeSkipsCheck() {
	nodes := minimalValidNodes()
	index, _ := buildNodeIndex(nodes)
	interceptors := []providers.InterceptorDefinition{
		{
			Name:    "my-interceptor",
			Mode:    providers.InterceptorModePreNode,
			Scope:   providers.InterceptorScopeAll,
			ApplyTo: []string{"nonexistent"}, // would fail if checked, but ALL scope skips applyTo check
		},
	}
	err := s.v.validateInterceptorApplyTo(interceptors, index)
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// ValidateFlowDefinition
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateFlowDefinition_Valid() {
	s.mockExecutorRegistry.EXPECT().IsRegistered("test-executor").Return(true)
	s.mockExecutorRegistry.EXPECT().GetExecutorMeta("test-executor").Return(nil, nil)
	s.mockGraphBuilder.EXPECT().ValidateGraph(mock.Anything, mock.Anything).Return(nil)
	err := s.v.ValidateFlowDefinition(context.Background(), minimalValidFlow())
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateFlowDefinition_InvalidMetadata() {
	fd := minimalValidFlow()
	fd.Handle = ""
	err := s.v.ValidateFlowDefinition(context.Background(), fd)
	s.Require().NotNil(err)
	s.Equal(ErrorMissingFlowHandle.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateFlowDefinition_InvalidStructure() {
	fd := minimalValidFlow()
	// Duplicate node IDs.
	fd.Nodes = append(fd.Nodes, providers.NodeDefinition{ID: "start", Type: string(common.NodeTypeEnd)})
	err := s.v.ValidateFlowDefinition(context.Background(), fd)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowStructure.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateFlowDefinition_InvalidNodeFormat() {
	fd := minimalValidFlow()
	// START node with onFailure set -- structure is valid but node format is not.
	fd.Nodes[0].OnFailure = "end"
	err := s.v.ValidateFlowDefinition(context.Background(), fd)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateFlowDefinition_InvalidInterceptorFormat() {
	s.mockExecutorRegistry.EXPECT().IsRegistered("test-executor").Return(true)
	s.mockExecutorRegistry.EXPECT().GetExecutorMeta("test-executor").Return(nil, nil)
	fd := minimalValidFlow()
	fd.Interceptors = []providers.InterceptorDefinition{
		{Name: "", Mode: providers.InterceptorModePreRequest},
	}
	err := s.v.ValidateFlowDefinition(context.Background(), fd)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

// ---------------------------------------------------------------------------
// collectAllNodeReferences / buildAdjacencyList -- edge cases
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestCollectAllNodeReferences_ConditionOnSkip() {
	nodes := []providers.NodeDefinition{
		{
			ID:        "task",
			Type:      string(common.NodeTypeTaskExecution),
			Condition: &providers.ConditionDefinition{Key: "k", Value: "v", OnSkip: "end"},
		},
	}
	refs := collectAllNodeReferences(nodes)
	s.Len(refs, 1)
	s.Equal("condition.onSkip", refs[0].fieldName)
	s.Equal("end", refs[0].targetNodeID)
}

func (s *ValidatorTestSuite) TestCollectAllNodeReferences_PromptActionNextNode() {
	nodes := []providers.NodeDefinition{
		{
			ID:   "prompt",
			Type: string(common.NodeTypePrompt),
			Prompts: []providers.PromptDefinition{
				{Action: &providers.ActionDefinition{NextNode: "end"}},
			},
		},
	}
	refs := collectAllNodeReferences(nodes)
	s.Len(refs, 1)
	s.Equal("action.nextNode", refs[0].fieldName)
	s.Equal("end", refs[0].targetNodeID)
}

// ---------------------------------------------------------------------------
// validateExecutors
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateExecutors_NotRegistered() {
	s.mockExecutorRegistry.On("IsRegistered", "unknown-executor").Return(false)
	node := &providers.NodeDefinition{
		ID:       "task",
		Executor: &providers.ExecutorDefinition{Name: "unknown-executor"},
	}
	err := s.v.validateExecutorGenericConstraints(node, providers.FlowTypeAuthentication)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidExecutorConfig.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateExecutors_Registered() {
	s.mockExecutorRegistry.On("IsRegistered", "known-executor").Return(true)
	s.mockExecutorRegistry.On("GetExecutorMeta", "known-executor").Return(&providers.ExecutorMeta{}, nil)
	node := &providers.NodeDefinition{
		ID:       "task",
		Executor: &providers.ExecutorDefinition{Name: "known-executor"},
	}
	err := s.v.validateExecutorGenericConstraints(node, providers.FlowTypeAuthentication)
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// validateInterceptorDefinitions
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateInterceptorDefinitions_NotRegistered() {
	s.mockInterceptorRegistry.On("IsRegistered", "my-ic").Return(false)
	interceptors := []providers.InterceptorDefinition{
		{Name: "my-ic", Mode: providers.InterceptorModePreRequest},
	}
	err := s.v.validateInterceptorDefinitions(interceptors)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateInterceptorDefinitions_Registered() {
	s.mockInterceptorRegistry.On("IsRegistered", "my-ic").Return(true)
	interceptors := []providers.InterceptorDefinition{
		{Name: "my-ic", Mode: providers.InterceptorModePreRequest},
	}
	err := s.v.validateInterceptorDefinitions(interceptors)
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// validateNodes
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateNodes_ExecutorNotRegistered() {
	s.mockExecutorRegistry.On("IsRegistered", "test-executor").Return(false)
	nodes := minimalValidNodes()
	index, _ := buildNodeIndex(nodes)
	err := s.v.validateNodes(nodes, index, providers.FlowTypeAuthentication)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidExecutorConfig.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateNodes_Valid() {
	s.mockExecutorRegistry.On("IsRegistered", "test-executor").Return(true)
	s.mockExecutorRegistry.On("GetExecutorMeta", "test-executor").Return(&providers.ExecutorMeta{}, nil)
	nodes := minimalValidNodes()
	index, _ := buildNodeIndex(nodes)
	err := s.v.validateNodes(nodes, index, providers.FlowTypeAuthentication)
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// Tests for validateRequiredExecutors
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateRequiredExecutors_AuthAssertMissingInAuthentication() {
	nodes := []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart), OnSuccess: "task"},
		{
			ID: "task", Type: string(common.NodeTypeTaskExecution),
			Executor:  &providers.ExecutorDefinition{Name: "CredentialsAuthExecutor"},
			OnSuccess: "end",
		},
		{ID: "end", Type: string(common.NodeTypeEnd)},
	}
	err := s.v.validateRequiredExecutors(providers.FlowTypeAuthentication, nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidExecutorConfig.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateRequiredExecutors_AuthAssertPresentInAuthentication() {
	nodes := []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart), OnSuccess: "task"},
		{
			ID: "task", Type: string(common.NodeTypeTaskExecution),
			Executor:  &providers.ExecutorDefinition{Name: "AuthAssertExecutor"},
			OnSuccess: "end",
		},
		{ID: "end", Type: string(common.NodeTypeEnd)},
	}
	err := s.v.validateRequiredExecutors(providers.FlowTypeAuthentication, nodes)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateRequiredExecutors_ProvisioningMissingInRegistration() {
	nodes := []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart), OnSuccess: "task"},
		{
			ID: "task", Type: string(common.NodeTypeTaskExecution),
			Executor:  &providers.ExecutorDefinition{Name: "UserTypeResolver"},
			OnSuccess: "end",
		},
		{ID: "end", Type: string(common.NodeTypeEnd)},
	}
	err := s.v.validateRequiredExecutors(providers.FlowTypeRegistration, nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidExecutorConfig.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateRequiredExecutors_UserTypeResolverMissingInRegistration() {
	nodes := []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart), OnSuccess: "task"},
		{
			ID: "task", Type: string(common.NodeTypeTaskExecution),
			Executor:  &providers.ExecutorDefinition{Name: "ProvisioningExecutor"},
			OnSuccess: "end",
		},
		{ID: "end", Type: string(common.NodeTypeEnd)},
	}
	err := s.v.validateRequiredExecutors(providers.FlowTypeRegistration, nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidExecutorConfig.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateRequiredExecutors_AllRequiredPresentInRegistration() {
	nodes := []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart), OnSuccess: "task1"},
		{
			ID: "task1", Type: string(common.NodeTypeTaskExecution),
			Executor:  &providers.ExecutorDefinition{Name: "ProvisioningExecutor"},
			OnSuccess: "task2",
		},
		{
			ID: "task2", Type: string(common.NodeTypeTaskExecution),
			Executor:  &providers.ExecutorDefinition{Name: "UserTypeResolver"},
			OnSuccess: "end",
		},
		{ID: "end", Type: string(common.NodeTypeEnd)},
	}
	err := s.v.validateRequiredExecutors(providers.FlowTypeRegistration, nodes)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateRequiredExecutors_RecoveryHasNoRequirements() {
	nodes := []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart), OnSuccess: "task"},
		{
			ID: "task", Type: string(common.NodeTypeTaskExecution),
			Executor:  &providers.ExecutorDefinition{Name: "CredentialsAuthExecutor"},
			OnSuccess: "end",
		},
		{ID: "end", Type: string(common.NodeTypeEnd)},
	}
	err := s.v.validateRequiredExecutors(providers.FlowTypeRecovery, nodes)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateRequiredExecutors_UserOnboardingBothRequired() {
	nodesWithBoth := []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart), OnSuccess: "task1"},
		{
			ID: "task1", Type: string(common.NodeTypeTaskExecution),
			Executor:  &providers.ExecutorDefinition{Name: "ProvisioningExecutor"},
			OnSuccess: "task2",
		},
		{
			ID: "task2", Type: string(common.NodeTypeTaskExecution),
			Executor:  &providers.ExecutorDefinition{Name: "UserTypeResolver"},
			OnSuccess: "end",
		},
		{ID: "end", Type: string(common.NodeTypeEnd)},
	}
	err := s.v.validateRequiredExecutors(providers.FlowTypeUserOnboarding, nodesWithBoth)
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// Tests for validateExecutorConstraints
// ---------------------------------------------------------------------------

type ValidatorServiceTestSuite struct {
	suite.Suite
	v                    *flowValidator
	mockExecutorRegistry *executormock.ExecutorRegistryInterfaceMock
}

func TestValidatorServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ValidatorServiceTestSuite))
}

func (s *ValidatorServiceTestSuite) SetupTest() {
	s.mockExecutorRegistry = executormock.NewExecutorRegistryInterfaceMock(s.T())
	s.v = &flowValidator{
		executorRegistry: s.mockExecutorRegistry,
	}
}

func (s *ValidatorServiceTestSuite) TestValidateExecutorConstraints_UnsupportedMode() {
	s.mockExecutorRegistry.On("GetExecutorMeta", "my-exec").Return(&providers.ExecutorMeta{
		SupportedModes: []string{"send", "verify"},
	}, nil)
	node := &providers.NodeDefinition{
		ID:       "task",
		Executor: &providers.ExecutorDefinition{Name: "my-exec", Mode: "generate"},
	}
	err := s.v.validateExecutorMeta(node, providers.FlowTypeAuthentication)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidExecutorConfig.Code, err.Code)
}

func (s *ValidatorServiceTestSuite) TestValidateExecutorConstraints_SupportedMode() {
	s.mockExecutorRegistry.On("GetExecutorMeta", "my-exec").Return(&providers.ExecutorMeta{
		SupportedModes: []string{"send", "verify"},
	}, nil)
	node := &providers.NodeDefinition{
		ID:       "task",
		Executor: &providers.ExecutorDefinition{Name: "my-exec", Mode: "send"},
	}
	err := s.v.validateExecutorMeta(node, providers.FlowTypeAuthentication)
	s.Nil(err)
}

func (s *ValidatorServiceTestSuite) TestValidateExecutorConstraints_ModeRequiredWhenNotSpecified() {
	s.mockExecutorRegistry.On("GetExecutorMeta", "my-exec").Return(&providers.ExecutorMeta{
		SupportedModes: []string{"send", "verify"},
	}, nil)
	node := &providers.NodeDefinition{
		ID:       "task",
		Executor: &providers.ExecutorDefinition{Name: "my-exec"},
	}
	err := s.v.validateExecutorMeta(node, providers.FlowTypeAuthentication)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidExecutorConfig.Code, err.Code)
}

func (s *ValidatorServiceTestSuite) TestValidateExecutorConstraints_DefaultModeAccepted() {
	s.mockExecutorRegistry.On("GetExecutorMeta", "my-exec").Return(&providers.ExecutorMeta{
		DefaultMode:    "send",
		SupportedModes: []string{"send", "verify"},
	}, nil)
	node := &providers.NodeDefinition{
		ID:       "task",
		Executor: &providers.ExecutorDefinition{Name: "my-exec"},
	}
	err := s.v.validateExecutorMeta(node, providers.FlowTypeAuthentication)
	s.Nil(err)
}

func (s *ValidatorServiceTestSuite) TestValidateExecutorConstraints_UnsupportedFlowType() {
	s.mockExecutorRegistry.On("GetExecutorMeta", "my-exec").Return(&providers.ExecutorMeta{
		SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAuthentication},
	}, nil)
	node := &providers.NodeDefinition{
		ID:       "task",
		Executor: &providers.ExecutorDefinition{Name: "my-exec"},
	}
	err := s.v.validateExecutorMeta(node, providers.FlowTypeRecovery)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidExecutorConfig.Code, err.Code)
}

func (s *ValidatorServiceTestSuite) TestValidateExecutorConstraints_SupportedFlowType() {
	s.mockExecutorRegistry.On("GetExecutorMeta", "my-exec").Return(&providers.ExecutorMeta{
		SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAuthentication},
	}, nil)
	node := &providers.NodeDefinition{
		ID:       "task",
		Executor: &providers.ExecutorDefinition{Name: "my-exec"},
	}
	err := s.v.validateExecutorMeta(node, providers.FlowTypeAuthentication)
	s.Nil(err)
}

func (s *ValidatorServiceTestSuite) TestValidateExecutorConstraints_MissingRequiredProperty() {
	s.mockExecutorRegistry.On("GetExecutorMeta", "my-exec").Return(&providers.ExecutorMeta{
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "emailTemplate", IsRequired: true},
		},
	}, nil)
	node := &providers.NodeDefinition{
		ID:         "task",
		Executor:   &providers.ExecutorDefinition{Name: "my-exec"},
		Properties: map[string]interface{}{},
	}
	err := s.v.validateExecutorMeta(node, providers.FlowTypeAuthentication)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidExecutorConfig.Code, err.Code)
}

func (s *ValidatorServiceTestSuite) TestValidateExecutorConstraints_RequiredPropertyPresent() {
	s.mockExecutorRegistry.On("GetExecutorMeta", "my-exec").Return(&providers.ExecutorMeta{
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "emailTemplate", IsRequired: true},
		},
	}, nil)
	node := &providers.NodeDefinition{
		ID:         "task",
		Executor:   &providers.ExecutorDefinition{Name: "my-exec"},
		Properties: map[string]interface{}{"emailTemplate": "welcome-email"},
	}
	err := s.v.validateExecutorMeta(node, providers.FlowTypeAuthentication)
	s.Nil(err)
}

func (s *ValidatorServiceTestSuite) TestValidateExecutorConstraints_UnsupportedProperty() {
	s.mockExecutorRegistry.On("GetExecutorMeta", "my-exec").Return(&providers.ExecutorMeta{
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "emailTemplate", IsRequired: true},
		},
	}, nil)
	node := &providers.NodeDefinition{
		ID:         "task",
		Executor:   &providers.ExecutorDefinition{Name: "my-exec"},
		Properties: map[string]interface{}{"emailTemplate": "welcome-email", "unknownProp": "value"},
	}
	err := s.v.validateExecutorMeta(node, providers.FlowTypeAuthentication)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidExecutorConfig.Code, err.Code)
}

func (s *ValidatorServiceTestSuite) TestValidateExecutorConstraints_OptionalPropertyOmitted() {
	s.mockExecutorRegistry.On("GetExecutorMeta", "my-exec").Return(&providers.ExecutorMeta{
		SupportedProperties: []providers.ExecutorSupportedProperties{
			{Property: "emailTemplate", IsRequired: true},
			{Property: "timeout"},
		},
	}, nil)
	node := &providers.NodeDefinition{
		ID:         "task",
		Executor:   &providers.ExecutorDefinition{Name: "my-exec"},
		Properties: map[string]interface{}{"emailTemplate": "welcome-email"},
	}
	err := s.v.validateExecutorMeta(node, providers.FlowTypeAuthentication)
	s.Nil(err)
}

func (s *ValidatorServiceTestSuite) TestValidateExecutorConstraints_NoMetaSkipsChecks() {
	s.mockExecutorRegistry.On("GetExecutorMeta", "my-exec").Return(nil, fmt.Errorf("not found"))
	node := &providers.NodeDefinition{
		ID:       "task",
		Executor: &providers.ExecutorDefinition{Name: "my-exec", Mode: "anything"},
	}
	err := s.v.validateExecutorMeta(node, providers.FlowTypeAuthentication)
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// Tests for validateSSOCheckExecutor
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateSSOCheckExecutor_ValidPair() {
	nodes := []providers.NodeDefinition{
		{
			ID: "sso-check", Type: string(common.NodeTypeTaskExecution),
			Executor:   &providers.ExecutorDefinition{Name: executor.ExecutorNameSSOCheck},
			Properties: map[string]interface{}{common.NodePropertyCheckpointRef: "session"},
		},
		{
			ID: "session", Type: string(common.NodeTypeTaskExecution),
			Executor: &providers.ExecutorDefinition{Name: executor.ExecutorNameSession},
		},
	}
	index, _ := buildNodeIndex(nodes)
	err := s.v.validateSSOCheckExecutor(&nodes[0], index)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateSSOCheckExecutor_CheckpointRefNotString() {
	node := &providers.NodeDefinition{
		ID:         "sso-check",
		Type:       string(common.NodeTypeTaskExecution),
		Executor:   &providers.ExecutorDefinition{Name: executor.ExecutorNameSSOCheck},
		Properties: map[string]interface{}{common.NodePropertyCheckpointRef: 123},
	}
	err := s.v.validateSSOCheckExecutor(node, nil)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidExecutorConfig.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "checkpointRef must be a string")
}

func (s *ValidatorTestSuite) TestValidateSSOCheckExecutor_CheckpointRefNonExistentNode() {
	node := &providers.NodeDefinition{
		ID:         "sso-check",
		Type:       string(common.NodeTypeTaskExecution),
		Executor:   &providers.ExecutorDefinition{Name: executor.ExecutorNameSSOCheck},
		Properties: map[string]interface{}{common.NodePropertyCheckpointRef: "missing-node"},
	}
	index := map[string]*providers.NodeDefinition{"sso-check": node}
	err := s.v.validateSSOCheckExecutor(node, index)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeReference.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "non-existent node")
}

func (s *ValidatorTestSuite) TestValidateSSOCheckExecutor_CheckpointRefNotSessionExecutor() {
	otherTask := &providers.NodeDefinition{
		ID:       "other-task",
		Type:     string(common.NodeTypeTaskExecution),
		Executor: &providers.ExecutorDefinition{Name: "CredentialsAuthExecutor"},
	}
	node := &providers.NodeDefinition{
		ID:         "sso-check",
		Type:       string(common.NodeTypeTaskExecution),
		Executor:   &providers.ExecutorDefinition{Name: executor.ExecutorNameSSOCheck},
		Properties: map[string]interface{}{common.NodePropertyCheckpointRef: "other-task"},
	}
	index := map[string]*providers.NodeDefinition{"sso-check": node, "other-task": otherTask}
	err := s.v.validateSSOCheckExecutor(node, index)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidExecutorConfig.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "must reference a SessionExecutor node")
}

// ---------------------------------------------------------------------------
// Tests for validateSessionExecutor
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateSessionExecutor_ReferencedBySSOCheck() {
	nodes := []providers.NodeDefinition{
		{
			ID: "sso-check", Type: string(common.NodeTypeTaskExecution),
			Executor:   &providers.ExecutorDefinition{Name: executor.ExecutorNameSSOCheck},
			Properties: map[string]interface{}{common.NodePropertyCheckpointRef: "session"},
		},
		{
			ID: "session", Type: string(common.NodeTypeTaskExecution),
			Executor: &providers.ExecutorDefinition{Name: executor.ExecutorNameSession},
		},
	}
	err := s.v.validateSessionExecutor(&nodes[1], nodes)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateSessionExecutor_OrphanSessionExecutor() {
	nodes := []providers.NodeDefinition{
		{
			ID: "session", Type: string(common.NodeTypeTaskExecution),
			Executor: &providers.ExecutorDefinition{Name: executor.ExecutorNameSession},
		},
	}
	err := s.v.validateSessionExecutor(&nodes[0], nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidExecutorConfig.Code, err.Code)
	s.Contains(err.ErrorDescription.DefaultValue, "not referenced by any SSOCheckExecutor")
}

func (s *ValidatorTestSuite) TestValidateSessionExecutor_MultiplePairsValid() {
	nodes := []providers.NodeDefinition{
		{
			ID: "sso-check-1", Type: string(common.NodeTypeTaskExecution),
			Executor:   &providers.ExecutorDefinition{Name: executor.ExecutorNameSSOCheck},
			Properties: map[string]interface{}{common.NodePropertyCheckpointRef: "session-1"},
		},
		{
			ID: "session-1", Type: string(common.NodeTypeTaskExecution),
			Executor: &providers.ExecutorDefinition{Name: executor.ExecutorNameSession},
		},
		{
			ID: "sso-check-2", Type: string(common.NodeTypeTaskExecution),
			Executor:   &providers.ExecutorDefinition{Name: executor.ExecutorNameSSOCheck},
			Properties: map[string]interface{}{common.NodePropertyCheckpointRef: "session-2"},
		},
		{
			ID: "session-2", Type: string(common.NodeTypeTaskExecution),
			Executor: &providers.ExecutorDefinition{Name: executor.ExecutorNameSession},
		},
	}
	err := s.v.validateSessionExecutor(&nodes[1], nodes)
	s.Nil(err)
	err = s.v.validateSessionExecutor(&nodes[3], nodes)
	s.Nil(err)
}
