/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
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
// ValidatorTestSuite — pure (non-service) functions
// ---------------------------------------------------------------------------

type ValidatorTestSuite struct {
	suite.Suite
}

func TestValidatorTestSuite(t *testing.T) {
	suite.Run(t, new(ValidatorTestSuite))
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
	err := validateMetadata(nil)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidRequestFormat.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateMetadata_MissingHandle() {
	fd := minimalValidFlow()
	fd.Handle = ""
	err := validateMetadata(fd)
	s.Require().NotNil(err)
	s.Equal(ErrorMissingFlowHandle.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateMetadata_InvalidHandleFormat() {
	fd := minimalValidFlow()
	fd.Handle = "-invalid"
	err := validateMetadata(fd)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowHandleFormat.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateMetadata_MissingName() {
	fd := minimalValidFlow()
	fd.Name = ""
	err := validateMetadata(fd)
	s.Require().NotNil(err)
	s.Equal(ErrorMissingFlowName.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateMetadata_InvalidFlowType() {
	fd := minimalValidFlow()
	fd.FlowType = "UNKNOWN"
	err := validateMetadata(fd)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowType.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateMetadata_InvalidIDFormat() {
	fd := minimalValidFlow()
	fd.ID = "not-a-uuid"
	err := validateMetadata(fd)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowIDFormat.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateMetadata_ValidID() {
	fd := minimalValidFlow()
	fd.ID = "550e8400-e29b-41d4-a716-446655440000"
	err := validateMetadata(fd)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateMetadata_TooFewNodes_One() {
	fd := minimalValidFlow()
	fd.Nodes = []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart)},
	}
	err := validateMetadata(fd)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateMetadata_TooFewNodes_Two() {
	fd := minimalValidFlow()
	fd.Nodes = []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart)},
		{ID: "end", Type: string(common.NodeTypeEnd)},
	}
	err := validateMetadata(fd)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateMetadata_Valid() {
	err := validateMetadata(minimalValidFlow())
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
	s.Equal(ErrorDuplicateNodeID.Code, err.Code)
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
	err := validateNodeTypesAndCardinality(nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeType.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateNodeTypesAndCardinality_MissingStart() {
	nodes := []providers.NodeDefinition{
		{ID: "task", Type: string(common.NodeTypeTaskExecution)},
		{ID: "end", Type: string(common.NodeTypeEnd)},
	}
	err := validateNodeTypesAndCardinality(nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorMissingStartNode.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateNodeTypesAndCardinality_DuplicateStart() {
	nodes := []providers.NodeDefinition{
		{ID: "start1", Type: string(common.NodeTypeStart)},
		{ID: "start2", Type: string(common.NodeTypeStart)},
		{ID: "end", Type: string(common.NodeTypeEnd)},
	}
	err := validateNodeTypesAndCardinality(nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorDuplicateStartNode.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateNodeTypesAndCardinality_MissingEnd() {
	nodes := []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart)},
		{ID: "task", Type: string(common.NodeTypeTaskExecution)},
	}
	err := validateNodeTypesAndCardinality(nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorMissingEndNode.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateNodeTypesAndCardinality_DuplicateEnd() {
	nodes := []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart)},
		{ID: "end1", Type: string(common.NodeTypeEnd)},
		{ID: "end2", Type: string(common.NodeTypeEnd)},
	}
	err := validateNodeTypesAndCardinality(nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorDuplicateEndNode.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateNodeTypesAndCardinality_Valid() {
	err := validateNodeTypesAndCardinality(minimalValidNodes())
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
	err := validateNodeReferences(refs, index)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeReference.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateNodeReferences_Valid() {
	nodes := minimalValidNodes()
	index, _ := buildNodeIndex(nodes)
	refs := collectAllNodeReferences(nodes)
	err := validateNodeReferences(refs, index)
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
	err := validateReachability(nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorOrphanedNode.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateReachability_AllReachable() {
	err := validateReachability(minimalValidNodes())
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
	err := validateTermination(nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorNoTermination.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateTermination_AllReachEnd() {
	err := validateTermination(minimalValidNodes())
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// validateStructure
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateStructure_Valid() {
	_, err := validateStructure(minimalValidNodes())
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
	_, err := validateStructure(nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidNodeReference.Code, err.Code)
}

// ---------------------------------------------------------------------------
// validateStartNode
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateStartNode_MissingOnSuccess() {
	node := &providers.NodeDefinition{ID: "start", Type: string(common.NodeTypeStart)}
	err := validateStartNode(node)
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
	err := validateStartNode(node)
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
	err := validateStartNode(node)
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
	err := validateStartNode(node)
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
	err := validateStartNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateStartNode_Valid() {
	node := &providers.NodeDefinition{
		ID:        "start",
		Type:      string(common.NodeTypeStart),
		OnSuccess: "task",
	}
	err := validateStartNode(node)
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// validateEndNode
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateEndNode_HasOnSuccess() {
	node := &providers.NodeDefinition{ID: "end", Type: string(common.NodeTypeEnd), OnSuccess: "task"}
	err := validateEndNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateEndNode_HasOnFailure() {
	node := &providers.NodeDefinition{ID: "end", Type: string(common.NodeTypeEnd), OnFailure: "task"}
	err := validateEndNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateEndNode_HasOnIncomplete() {
	node := &providers.NodeDefinition{ID: "end", Type: string(common.NodeTypeEnd), OnIncomplete: "task"}
	err := validateEndNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateEndNode_HasExecutor() {
	node := &providers.NodeDefinition{
		ID:       "end",
		Type:     string(common.NodeTypeEnd),
		Executor: &providers.ExecutorDefinition{Name: "exec"},
	}
	err := validateEndNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateEndNode_HasPrompts() {
	node := &providers.NodeDefinition{
		ID:      "end",
		Type:    string(common.NodeTypeEnd),
		Prompts: []providers.PromptDefinition{{Action: &providers.ActionDefinition{NextNode: "x"}}},
	}
	err := validateEndNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateEndNode_HasNext() {
	node := &providers.NodeDefinition{ID: "end", Type: string(common.NodeTypeEnd), Next: "task"}
	err := validateEndNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateEndNode_Valid() {
	node := &providers.NodeDefinition{ID: "end", Type: string(common.NodeTypeEnd)}
	err := validateEndNode(node)
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// validateTaskExecutionNode
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateTaskExecutionNode_NilExecutor() {
	nodes := minimalValidNodes()
	index, _ := buildNodeIndex(nodes)
	node := &providers.NodeDefinition{ID: "task", Type: string(common.NodeTypeTaskExecution), OnSuccess: "end"}
	err := validateTaskExecutionNode(node, index)
	s.Require().NotNil(err)
	s.Equal(ErrorTaskNodeMissingExecutor.Code, err.Code)
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
	err := validateTaskExecutionNode(node, index)
	s.Require().NotNil(err)
	s.Equal(ErrorTaskNodeMissingExecutor.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateTaskExecutionNode_MissingOnSuccess() {
	nodes := minimalValidNodes()
	index, _ := buildNodeIndex(nodes)
	node := &providers.NodeDefinition{
		ID:       "task",
		Type:     string(common.NodeTypeTaskExecution),
		Executor: &providers.ExecutorDefinition{Name: "exec"},
	}
	err := validateTaskExecutionNode(node, index)
	s.Require().NotNil(err)
	s.Equal(ErrorTaskNodeMissingOnSuccess.Code, err.Code)
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
	err := validateTaskExecutionNode(node, index)
	s.Require().NotNil(err)
	s.Equal(ErrorTaskNodeInvalidFailureTarget.Code, err.Code)
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
	err := validateTaskExecutionNode(node, index)
	s.Require().NotNil(err)
	s.Equal(ErrorTaskNodeInvalidIncompleteTarget.Code, err.Code)
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
	err := validateTaskExecutionNode(node, index)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateTaskExecutionNode_Valid() {
	nodes := minimalValidNodes()
	index, _ := buildNodeIndex(nodes)
	node := &nodes[1]
	err := validateTaskExecutionNode(node, index)
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
	err := validatePromptNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorPromptNodeInvalidConfig.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidatePromptNode_NeitherPromptsNorNext() {
	node := &providers.NodeDefinition{ID: "prompt", Type: string(common.NodeTypePrompt)}
	err := validatePromptNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorPromptNodeInvalidConfig.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidatePromptNode_DisplayOnly_Valid() {
	node := &providers.NodeDefinition{
		ID:   "prompt",
		Type: string(common.NodeTypePrompt),
		Next: "end",
	}
	err := validatePromptNode(node)
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
	err := validatePromptNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorPromptMissingAction.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidatePromptNode_Interactive_EmptyNextNode() {
	node := &providers.NodeDefinition{
		ID:   "prompt",
		Type: string(common.NodeTypePrompt),
		Prompts: []providers.PromptDefinition{
			{Action: &providers.ActionDefinition{NextNode: ""}},
		},
	}
	err := validatePromptNode(node)
	s.Require().NotNil(err)
	s.Equal(ErrorPromptMissingAction.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidatePromptNode_Interactive_Valid() {
	node := &providers.NodeDefinition{
		ID:   "prompt",
		Type: string(common.NodeTypePrompt),
		Prompts: []providers.PromptDefinition{
			{Action: &providers.ActionDefinition{NextNode: "end"}},
		},
	}
	err := validatePromptNode(node)
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// validateInputDefinitions
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateInputDefinitions_InvalidInputType() {
	inputs := []providers.InputDefinition{
		{Type: "UNSUPPORTED_TYPE", Identifier: "field1"},
	}
	err := validateInputDefinitions("prompt", inputs)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidInputType.Code, err.Code)
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
	err := validateInputDefinitions("prompt", inputs)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidValidationRule.Code, err.Code)
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
	err := validateInputDefinitions("prompt", inputs)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidValidationRule.Code, err.Code)
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
	err := validateInputDefinitions("prompt", inputs)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidValidationRule.Code, err.Code)
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
	err := validateInputDefinitions("prompt", inputs)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateInputDefinitions_EmptyType_IsAllowed() {
	// Empty type is not validated against valid types (only non-empty types are checked).
	inputs := []providers.InputDefinition{
		{Type: "", Identifier: "field1"},
	}
	err := validateInputDefinitions("prompt", inputs)
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
	err := validateInputDefinitions("prompt", inputs)
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
	err := validateInterceptorFormat(0, ic)
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
	err := validateInterceptorFormat(0, ic)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateInterceptorFormat_InvalidMode() {
	ic := providers.InterceptorDefinition{
		Name: "my-interceptor",
		Mode: "INVALID_MODE",
	}
	err := validateInterceptorFormat(0, ic)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateInterceptorFormat_InvalidScope() {
	ic := providers.InterceptorDefinition{
		Name:  "my-interceptor",
		Mode:  providers.InterceptorModePreRequest,
		Scope: "INVALID_SCOPE",
	}
	err := validateInterceptorFormat(0, ic)
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
	err := validateInterceptorFormat(0, ic)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateInterceptorFormat_Valid_NoScope() {
	ic := providers.InterceptorDefinition{
		Name: "my-interceptor",
		Mode: providers.InterceptorModePreRequest,
	}
	err := validateInterceptorFormat(0, ic)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateInterceptorFormat_Valid_AllScope() {
	ic := providers.InterceptorDefinition{
		Name:  "my-interceptor",
		Mode:  providers.InterceptorModePreNode,
		Scope: providers.InterceptorScopeAll,
	}
	err := validateInterceptorFormat(0, ic)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateInterceptorFormat_Valid_SelectedScope() {
	ic := providers.InterceptorDefinition{
		Name:    "my-interceptor",
		Mode:    providers.InterceptorModePostNode,
		Scope:   providers.InterceptorScopeSelected,
		ApplyTo: []string{"task"},
	}
	err := validateInterceptorFormat(0, ic)
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
	err := validateInterceptorApplyTo(interceptors, index)
	s.Require().NotNil(err)
	s.Equal(ErrorInterceptorInvalidApplyTo.Code, err.Code)
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
	err := validateInterceptorApplyTo(interceptors, index)
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
	err := validateInterceptorApplyTo(interceptors, index)
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// validateFlowDefinitionBasic
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateFlowDefinitionBasic_Valid() {
	err := validateFlowDefinitionBasic(minimalValidFlow())
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateFlowDefinitionBasic_InvalidMetadata() {
	fd := minimalValidFlow()
	fd.Handle = ""
	err := validateFlowDefinitionBasic(fd)
	s.Require().NotNil(err)
	s.Equal(ErrorMissingFlowHandle.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateFlowDefinitionBasic_InvalidStructure() {
	fd := minimalValidFlow()
	// Duplicate node IDs.
	fd.Nodes = append(fd.Nodes, providers.NodeDefinition{ID: "start", Type: string(common.NodeTypeEnd)})
	err := validateFlowDefinitionBasic(fd)
	s.Require().NotNil(err)
	s.Equal(ErrorDuplicateNodeID.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateFlowDefinitionBasic_InvalidNodeFormat() {
	fd := minimalValidFlow()
	// START node with onFailure set — structure is valid but node format is not.
	fd.Nodes[0].OnFailure = "end"
	err := validateFlowDefinitionBasic(fd)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateFlowDefinitionBasic_InvalidInterceptorFormat() {
	fd := minimalValidFlow()
	fd.Interceptors = []providers.InterceptorDefinition{
		{Name: "", Mode: providers.InterceptorModePreRequest},
	}
	err := validateFlowDefinitionBasic(fd)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

// ---------------------------------------------------------------------------
// collectAllNodeReferences / buildAdjacencyList — edge cases
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
// ValidatorServiceTestSuite — service-level functions using mock registries
// ---------------------------------------------------------------------------

type ValidatorServiceTestSuite struct {
	suite.Suite
	svc                     *flowMgtService
	mockExecutorRegistry    *executormock.ExecutorRegistryInterfaceMock
	mockInterceptorRegistry *interceptormock.InterceptorRegistryInterfaceMock
	mockGraphBuilder        *graphBuilderInterfaceMock
}

func TestValidatorServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ValidatorServiceTestSuite))
}

func (s *ValidatorServiceTestSuite) SetupTest() {
	s.mockExecutorRegistry = executormock.NewExecutorRegistryInterfaceMock(s.T())
	s.mockInterceptorRegistry = interceptormock.NewInterceptorRegistryInterfaceMock(s.T())
	s.mockGraphBuilder = newGraphBuilderInterfaceMock(s.T())
	s.svc = &flowMgtService{
		executorRegistry:    s.mockExecutorRegistry,
		interceptorRegistry: s.mockInterceptorRegistry,
		graphBuilder:        s.mockGraphBuilder,
	}
}

func (s *ValidatorServiceTestSuite) TestValidateExecutors_NotRegistered() {
	s.mockExecutorRegistry.On("IsRegistered", "unknown-executor").Return(false)
	node := &providers.NodeDefinition{
		ID:       "task",
		Executor: &providers.ExecutorDefinition{Name: "unknown-executor"},
	}
	err := s.svc.validateExecutors(node, providers.FlowTypeAuthentication)
	s.Require().NotNil(err)
	s.Equal(ErrorExecutorNotRegistered.Code, err.Code)
}

func (s *ValidatorServiceTestSuite) TestValidateExecutors_Registered() {
	s.mockExecutorRegistry.On("IsRegistered", "known-executor").Return(true)
	s.mockExecutorRegistry.On("GetExecutorMeta", "known-executor").Return(&providers.ExecutorMeta{}, nil)
	node := &providers.NodeDefinition{
		ID:       "task",
		Executor: &providers.ExecutorDefinition{Name: "known-executor"},
	}
	err := s.svc.validateExecutors(node, providers.FlowTypeAuthentication)
	s.Nil(err)
}

func (s *ValidatorServiceTestSuite) TestValidateInterceptorDefinitions_NotRegistered() {
	s.mockInterceptorRegistry.On("IsRegistered", "my-ic").Return(false)
	interceptors := []providers.InterceptorDefinition{
		{Name: "my-ic", Mode: providers.InterceptorModePreRequest},
	}
	err := s.svc.validateInterceptorDefinitions(interceptors)
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}

func (s *ValidatorServiceTestSuite) TestValidateInterceptorDefinitions_Registered() {
	s.mockInterceptorRegistry.On("IsRegistered", "my-ic").Return(true)
	interceptors := []providers.InterceptorDefinition{
		{Name: "my-ic", Mode: providers.InterceptorModePreRequest},
	}
	err := s.svc.validateInterceptorDefinitions(interceptors)
	s.Nil(err)
}

func (s *ValidatorServiceTestSuite) TestValidateNodes_ExecutorNotRegistered() {
	s.mockExecutorRegistry.On("IsRegistered", "test-executor").Return(false)
	nodes := minimalValidNodes()
	index, _ := buildNodeIndex(nodes)
	err := s.svc.validateNodes(nodes, index, providers.FlowTypeAuthentication)
	s.Require().NotNil(err)
	s.Equal(ErrorExecutorNotRegistered.Code, err.Code)
}

func (s *ValidatorServiceTestSuite) TestValidateNodes_Valid() {
	s.mockExecutorRegistry.On("IsRegistered", "test-executor").Return(true)
	s.mockExecutorRegistry.On("GetExecutorMeta", "test-executor").Return(&providers.ExecutorMeta{}, nil)
	nodes := minimalValidNodes()
	index, _ := buildNodeIndex(nodes)
	err := s.svc.validateNodes(nodes, index, providers.FlowTypeAuthentication)
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// Tests for validateForbiddenExecutors
// ---------------------------------------------------------------------------

func (s *ValidatorTestSuite) TestValidateForbiddenExecutors_ForbiddenInRecovery() {
	nodes := []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart), OnSuccess: "task"},
		{
			ID: "task", Type: string(common.NodeTypeTaskExecution),
			Executor:  &providers.ExecutorDefinition{Name: "ProvisioningExecutor"},
			OnSuccess: "end",
		},
		{ID: "end", Type: string(common.NodeTypeEnd)},
	}
	err := validateForbiddenExecutors(providers.FlowTypeRecovery, nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorExecutorForbiddenForFlowType.Code, err.Code)
}

func (s *ValidatorTestSuite) TestValidateForbiddenExecutors_AllowedInRecovery() {
	nodes := []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart), OnSuccess: "task"},
		{
			ID: "task", Type: string(common.NodeTypeTaskExecution),
			Executor:  &providers.ExecutorDefinition{Name: "CredentialsAuthExecutor"},
			OnSuccess: "end",
		},
		{ID: "end", Type: string(common.NodeTypeEnd)},
	}
	err := validateForbiddenExecutors(providers.FlowTypeRecovery, nodes)
	s.Nil(err)
}

func (s *ValidatorTestSuite) TestValidateForbiddenExecutors_ProvisioningAllowedInAuthentication() {
	nodes := []providers.NodeDefinition{
		{ID: "start", Type: string(common.NodeTypeStart), OnSuccess: "task"},
		{
			ID: "task", Type: string(common.NodeTypeTaskExecution),
			Executor:  &providers.ExecutorDefinition{Name: "ProvisioningExecutor"},
			OnSuccess: "end",
		},
		{ID: "end", Type: string(common.NodeTypeEnd)},
	}
	err := validateForbiddenExecutors(providers.FlowTypeAuthentication, nodes)
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
	err := validateRequiredExecutors(providers.FlowTypeAuthentication, nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorRequiredExecutorMissing.Code, err.Code)
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
	err := validateRequiredExecutors(providers.FlowTypeAuthentication, nodes)
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
	err := validateRequiredExecutors(providers.FlowTypeRegistration, nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorRequiredExecutorMissing.Code, err.Code)
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
	err := validateRequiredExecutors(providers.FlowTypeRegistration, nodes)
	s.Require().NotNil(err)
	s.Equal(ErrorRequiredExecutorMissing.Code, err.Code)
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
	err := validateRequiredExecutors(providers.FlowTypeRegistration, nodes)
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
	err := validateRequiredExecutors(providers.FlowTypeRecovery, nodes)
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
	err := validateRequiredExecutors(providers.FlowTypeUserOnboarding, nodesWithBoth)
	s.Nil(err)
}

// ---------------------------------------------------------------------------
// Tests for validateExecutorConstraints
// ---------------------------------------------------------------------------

func (s *ValidatorServiceTestSuite) TestValidateExecutorConstraints_UnsupportedMode() {
	s.mockExecutorRegistry.On("GetExecutorMeta", "my-exec").Return(&providers.ExecutorMeta{
		SupportedModes: []string{"send", "verify"},
	}, nil)
	node := &providers.NodeDefinition{
		ID:       "task",
		Executor: &providers.ExecutorDefinition{Name: "my-exec", Mode: "generate"},
	}
	err := s.svc.validateExecutorConstraints(node, providers.FlowTypeAuthentication)
	s.Require().NotNil(err)
	s.Equal(ErrorUnsupportedExecutorMode.Code, err.Code)
}

func (s *ValidatorServiceTestSuite) TestValidateExecutorConstraints_SupportedMode() {
	s.mockExecutorRegistry.On("GetExecutorMeta", "my-exec").Return(&providers.ExecutorMeta{
		SupportedModes: []string{"send", "verify"},
	}, nil)
	node := &providers.NodeDefinition{
		ID:       "task",
		Executor: &providers.ExecutorDefinition{Name: "my-exec", Mode: "send"},
	}
	err := s.svc.validateExecutorConstraints(node, providers.FlowTypeAuthentication)
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
	err := s.svc.validateExecutorConstraints(node, providers.FlowTypeRecovery)
	s.Require().NotNil(err)
	s.Equal(ErrorUnsupportedExecutorFlowType.Code, err.Code)
}

func (s *ValidatorServiceTestSuite) TestValidateExecutorConstraints_SupportedFlowType() {
	s.mockExecutorRegistry.On("GetExecutorMeta", "my-exec").Return(&providers.ExecutorMeta{
		SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAuthentication},
	}, nil)
	node := &providers.NodeDefinition{
		ID:       "task",
		Executor: &providers.ExecutorDefinition{Name: "my-exec"},
	}
	err := s.svc.validateExecutorConstraints(node, providers.FlowTypeAuthentication)
	s.Nil(err)
}

func (s *ValidatorServiceTestSuite) TestValidateExecutorConstraints_MissingRequiredProperty() {
	s.mockExecutorRegistry.On("GetExecutorMeta", "my-exec").Return(&providers.ExecutorMeta{
		RequiredProperties: []string{"emailTemplate"},
	}, nil)
	node := &providers.NodeDefinition{
		ID:         "task",
		Executor:   &providers.ExecutorDefinition{Name: "my-exec"},
		Properties: map[string]interface{}{},
	}
	err := s.svc.validateExecutorConstraints(node, providers.FlowTypeAuthentication)
	s.Require().NotNil(err)
	s.Equal(ErrorMissingRequiredExecutorProperty.Code, err.Code)
}

func (s *ValidatorServiceTestSuite) TestValidateExecutorConstraints_RequiredPropertyPresent() {
	s.mockExecutorRegistry.On("GetExecutorMeta", "my-exec").Return(&providers.ExecutorMeta{
		RequiredProperties: []string{"emailTemplate"},
	}, nil)
	node := &providers.NodeDefinition{
		ID:         "task",
		Executor:   &providers.ExecutorDefinition{Name: "my-exec"},
		Properties: map[string]interface{}{"emailTemplate": "welcome-email"},
	}
	err := s.svc.validateExecutorConstraints(node, providers.FlowTypeAuthentication)
	s.Nil(err)
}

func (s *ValidatorServiceTestSuite) TestValidateExecutorConstraints_NoMetaSkipsChecks() {
	s.mockExecutorRegistry.On("GetExecutorMeta", "my-exec").Return(nil, fmt.Errorf("not found"))
	node := &providers.NodeDefinition{
		ID:       "task",
		Executor: &providers.ExecutorDefinition{Name: "my-exec", Mode: "anything"},
	}
	err := s.svc.validateExecutorConstraints(node, providers.FlowTypeAuthentication)
	s.Nil(err)
}
