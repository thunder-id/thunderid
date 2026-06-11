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

package core

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
)

const testEmailAttr = "email"

type PromptOnlyNodeTestSuite struct {
	suite.Suite
}

func TestPromptOnlyNodeTestSuite(t *testing.T) {
	suite.Run(t, new(PromptOnlyNodeTestSuite))
}

func (s *PromptOnlyNodeTestSuite) TestNewPromptOnlyNode() {
	node := newPromptNode("prompt-1", map[string]interface{}{"key": "value"}, true, false)

	s.NotNil(node)
	s.Equal("prompt-1", node.GetID())
	s.Equal(common.NodeTypePrompt, node.GetType())
	s.True(node.IsStartNode())
	s.False(node.IsFinalNode())
}

func (s *PromptOnlyNodeTestSuite) TestExecuteNoInputs() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	ctx := &NodeContext{ExecutionID: "test-flow", UserInputs: map[string]string{}}

	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusComplete, resp.Status)
	s.Equal(common.NodeResponseType(""), resp.Type)
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithRequiredData() {
	tests := []struct {
		name           string
		userInputs     map[string]string
		expectComplete bool
		requiredCount  int
	}{
		{"No user input provided", map[string]string{}, false, 2},
		{
			"All required data provided",
			map[string]string{"username": "testuser", testEmailAttr: "test@example.com"},
			true,
			0,
		},
		{"Partial data provided", map[string]string{"username": "testuser"}, false, 1},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
			promptNode := node.(PromptNodeInterface)
			promptNode.SetPrompts([]common.Prompt{
				{
					Inputs: []common.Input{
						{Identifier: "username", Required: true},
						{Identifier: testEmailAttr, Required: true},
					},
					Action: &common.Action{Ref: "submit", NextNode: "next"},
				},
			})

			ctx := &NodeContext{ExecutionID: "test-flow", CurrentAction: "submit", UserInputs: tt.userInputs}
			resp, err := node.Execute(ctx)

			s.Nil(err)
			s.NotNil(resp)

			if tt.expectComplete {
				s.Equal(common.NodeStatusComplete, resp.Status)
				s.Equal(common.NodeResponseType(""), resp.Type)
			} else {
				s.Equal(common.NodeStatusIncomplete, resp.Status)
				s.Equal(common.NodeResponseTypeView, resp.Type)
				s.Len(resp.Inputs, tt.requiredCount)
			}
		})
	}
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithOptionalData() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
				{Identifier: "nickname", Required: false},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{"username": "testuser"},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Equal(common.NodeResponseTypeView, resp.Type)
	s.Len(resp.Inputs, 1)
	s.Equal("nickname", resp.Inputs[0].Identifier)
	s.False(resp.Inputs[0].Required)
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithAlreadyPromptedOptionalData() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
				{Identifier: "nickname", Required: false},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{"username": "testuser"},
		RuntimeData: map[string]string{
			common.RuntimeKeyPresentedOptionalInputs: "nickname",
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusComplete, resp.Status)
	s.Equal(common.NodeResponseType(""), resp.Type)
}

func (s *PromptOnlyNodeTestSuite) TestExecuteMissingRequiredOnly() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
				{Identifier: "nickname", Required: false},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{"nickname": "testnick"},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Equal(common.NodeResponseTypeView, resp.Type)
	s.Len(resp.Inputs, 1)

	foundRequired := false
	for _, data := range resp.Inputs {
		if data.Identifier == "username" && data.Required {
			foundRequired = true
		}
	}
	s.True(foundRequired)
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithVerboseModeEnabled() {
	meta := map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{
				"type":  "TEXT",
				"id":    "text_001",
				"label": "Welcome",
			},
		},
	}

	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetMeta(meta)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	// Test with verbose mode enabled
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		Verbose:     true,
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Equal(common.NodeResponseTypeView, resp.Type)
	s.NotNil(resp.Meta)
	s.Equal(meta, resp.Meta)
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithVerboseModeDisabled() {
	meta := map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{
				"type":  "TEXT",
				"id":    "text_001",
				"label": "Welcome",
			},
		},
	}

	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetMeta(meta)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	// Test with verbose mode disabled (default)
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		Verbose:     false,
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Equal(common.NodeResponseTypeView, resp.Type)
	s.Nil(resp.Meta)
}

func (s *PromptOnlyNodeTestSuite) TestExecuteVerboseModeNoMeta() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	// Test with verbose mode enabled but no meta defined
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		Verbose:     true,
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Equal(common.NodeResponseTypeView, resp.Type)
	s.Nil(resp.Meta)
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithSets_ActionWithInputs() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
				{Identifier: "password", Required: true},
			},
			Action: &common.Action{Ref: "action_001", NextNode: "basic_auth"},
		},
		{
			Action: &common.Action{Ref: "action_002", NextNode: "google_auth"},
		},
	})

	// Select action_001 but don't provide inputs
	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "action_001",
		UserInputs:    map[string]string{},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Len(resp.Inputs, 2)
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithSets_ActionWithoutInputs() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
				{Identifier: "password", Required: true},
			},
			Action: &common.Action{Ref: "action_001", NextNode: "basic_auth"},
		},
		{
			Action: &common.Action{Ref: "action_002", NextNode: "google_auth"},
		},
	})

	// Select action_002 which has no inputs
	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "action_002",
		UserInputs:    map[string]string{},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusComplete, resp.Status)
	s.Equal("google_auth", resp.NextNodeID)
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithSets_ActionWithInputsProvided() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
				{Identifier: "password", Required: true},
			},
			Action: &common.Action{Ref: "action_001", NextNode: "basic_auth"},
		},
	})

	// Select action_001 with all inputs provided
	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "action_001",
		UserInputs: map[string]string{
			"username": "testuser",
			"password": "testpass",
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusComplete, resp.Status)
	s.Equal("basic_auth", resp.NextNodeID)
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithSets_NoActionSelected() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{{Identifier: "username", Required: true}},
			Action: &common.Action{Ref: "action_001", NextNode: "basic_auth"},
		},
		{
			Action: &common.Action{Ref: "action_002", NextNode: "google_auth"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "",
		UserInputs:    map[string]string{},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Len(resp.Actions, 2)
	s.Len(resp.Inputs, 1, "Should return all inputs from sets when no action selected")
	s.Equal("username", resp.Inputs[0].Identifier)
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithInvalidAction() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
			},
			Action: &common.Action{Ref: "login", NextNode: "auth"},
		},
	})

	// Select an action that doesn't exist
	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "unknown_action",
		UserInputs:    map[string]string{},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	// Should treat as no action selected - return both inputs and actions
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Len(resp.Inputs, 1)
	s.Equal("username", resp.Inputs[0].Identifier)
	s.Len(resp.Actions, 1, "Should return actions when invalid action is provided")
	s.Equal("login", resp.Actions[0].Ref)
}

func (s *PromptOnlyNodeTestSuite) TestAutoSelectSingleAction_NoInputs() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	// Single action with no inputs - should NOT auto-complete (confirmation prompts wait for explicit action)
	promptNode.SetPrompts([]common.Prompt{
		{
			Action: &common.Action{Ref: "continue", NextNode: "next_node"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "", // No action selected
		UserInputs:    map[string]string{},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status, "Confirmation prompt should wait for explicit action")
	s.Len(resp.Actions, 1, "Should return the action for user to select")
	s.Equal("continue", resp.Actions[0].Ref)
	s.Equal("", ctx.CurrentAction, "Context should NOT have auto-selected action for confirmation prompts")
}

func (s *PromptOnlyNodeTestSuite) TestAutoSelectSingleAction_WithInputsProvided() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	// Single action with inputs - should auto-select and validate inputs
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
				{Identifier: "password", Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "auth_node"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "", // No action selected
		UserInputs: map[string]string{
			"username": "testuser",
			"password": "testpass",
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusComplete, resp.Status,
		"Should complete when single action auto-selected and inputs provided")
	s.Equal("auth_node", resp.NextNodeID)
	s.Equal("submit", ctx.CurrentAction, "Context should have the auto-selected action")
}

func (s *PromptOnlyNodeTestSuite) TestAutoSelectSingleAction_WithMissingInputs() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	// Single action with required inputs missing - should NOT auto-select
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
				{Identifier: "password", Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "auth_node"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "",                  // No action selected
		UserInputs:    map[string]string{}, // No inputs
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status, "Should be incomplete when inputs are missing")
	s.Len(resp.Inputs, 2, "Should return missing inputs")
	s.Len(resp.Actions, 1, "Actions should be returned when inputs are missing (no auto-select)")
	s.Equal("submit", resp.Actions[0].Ref, "Action should be in the response")
	s.Equal("", ctx.CurrentAction, "Context should NOT have auto-selected action when inputs missing")
}

func (s *PromptOnlyNodeTestSuite) TestAutoSelectSingleAction_MultipleActionsNoAutoSelect() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	// Multiple actions - should not auto-select
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{{Identifier: "username", Required: true}},
			Action: &common.Action{Ref: "basic_auth", NextNode: "basic_node"},
		},
		{
			Action: &common.Action{Ref: "social_auth", NextNode: "social_node"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "", // No action selected
		UserInputs:    map[string]string{},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status, "Should be incomplete when multiple actions exist")
	s.Len(resp.Actions, 2, "Should return all actions when multiple exist")
	s.Equal("", ctx.CurrentAction, "Context should NOT have an auto-selected action with multiple actions")
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithFailureReason() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	// Context with failure reason in runtime data
	svcErr := serviceerror.ServiceError{Error: i18ncore.I18nMessage{DefaultValue: "Authentication failed"}}
	svcErrJSON, _ := json.Marshal(svcErr)
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{
			"failureReasonJSON": string(svcErrJSON),
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.NotNil(resp.Error, "Should include failure error in response")
	s.Equal("Authentication failed", resp.Error.Error.DefaultValue, "Should include failure reason in response")
	s.NotContains(ctx.RuntimeData, "failureReasonJSON", "Should delete failureReasonJSON from runtime data")
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithFailureReason_ClearsUserInputs() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
				{Identifier: "password", Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	// User submitted inputs, but downstream task failed - routed back with failureReasonJSON
	svcErr := serviceerror.ServiceError{
		Error: i18ncore.I18nMessage{DefaultValue: "A user with this username already exists"},
	}
	svcErrJSON, _ := json.Marshal(svcErr)
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			"username": "takenuser",
			"password": "secret",
		},
		RuntimeData: map[string]string{
			"failureReasonJSON": string(svcErrJSON),
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.NotNil(resp.Error)
	s.Equal("A user with this username already exists", resp.Error.Error.DefaultValue)
	s.NotContains(ctx.UserInputs, "username", "Prompt inputs should be cleared to force re-prompt")
	s.NotContains(ctx.UserInputs, "password", "Prompt inputs should be cleared to force re-prompt")
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithFailureReason_ClearsCurrentAction() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: testEmailAttr, Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	svcErr := serviceerror.ServiceError{
		Error: i18ncore.I18nMessage{DefaultValue: "A user with this email already exists"},
	}
	svcErrJSON, _ := json.Marshal(svcErr)
	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs: map[string]string{
			testEmailAttr: "existing@example.com",
		},
		RuntimeData: map[string]string{
			"failureReasonJSON": string(svcErrJSON),
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.NotNil(resp.Error)
	s.Equal("A user with this email already exists", resp.Error.Error.DefaultValue)
	s.Equal("", ctx.CurrentAction, "CurrentAction should be cleared to force re-prompt")
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithEmptyFailureReason() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	// Context with empty failureReasonJSON (absent) — no failure path triggered
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Nil(resp.Error, "Should not set error when failureReasonJSON is absent")
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithNilRuntimeData() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	// Context with nil runtime data
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		RuntimeData: nil,
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Nil(resp.Error, "Should handle nil runtime data gracefully")
}

func (s *PromptOnlyNodeTestSuite) TestExecuteInvalidActionReturnsFailure() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	// Setup prompts with specific actions
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
				{Identifier: "password", Required: true},
			},
			Action: &common.Action{Ref: "valid_action", NextNode: "next_node"},
		},
	})

	// Provide all required inputs but with an action that matches but has no nextNode
	// This simulates when getNextNodeForActionRef returns empty string
	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "valid_action",
		UserInputs: map[string]string{
			"username": "testuser",
			"password": "testpass",
		},
	}

	// Temporarily modify the prompt to have empty nextNode
	prompts := promptNode.GetPrompts()
	originalNextNode := prompts[0].Action.NextNode
	prompts[0].Action.NextNode = ""
	promptNode.SetPrompts(prompts)

	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusFailure, resp.Status, "Should return failure status")
	s.NotNil(resp.Error, "Should set error on invalid action")

	// Restore for other tests
	prompts[0].Action.NextNode = originalNextNode
	promptNode.SetPrompts(prompts)
}

func (s *PromptOnlyNodeTestSuite) TestGetAndSetPrompts() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	// Initially should be empty
	prompts := promptNode.GetPrompts()
	s.NotNil(prompts)
	s.Len(prompts, 0)

	// Set prompts
	testPrompts := []common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
				{Identifier: "password", Required: true},
			},
			Action: &common.Action{Ref: "login", NextNode: "auth_node"},
		},
		{
			Action: &common.Action{Ref: "signup", NextNode: "register_node"},
		},
	}
	promptNode.SetPrompts(testPrompts)

	// Verify prompts are set
	retrievedPrompts := promptNode.GetPrompts()
	s.Len(retrievedPrompts, 2)
	s.Equal("username", retrievedPrompts[0].Inputs[0].Identifier)
	s.Equal("login", retrievedPrompts[0].Action.Ref)
	s.Equal("signup", retrievedPrompts[1].Action.Ref)
}

func (s *PromptOnlyNodeTestSuite) TestGetAllInputs() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(*promptNode)

	// Set multiple prompts with various inputs
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
				{Identifier: "password", Required: true},
			},
			Action: &common.Action{Ref: "login", NextNode: "auth_node"},
		},
		{
			Inputs: []common.Input{
				{Identifier: testEmailAttr, Required: true},
			},
			Action: &common.Action{Ref: "signup", NextNode: "register_node"},
		},
		{
			// Prompt with no inputs
			Action: &common.Action{Ref: "cancel", NextNode: "exit_node"},
		},
	})

	// Test getAllInputs
	allInputs := promptNode.getAllInputs()
	s.Len(allInputs, 3, "Should return all inputs from all prompts")
	s.Equal("username", allInputs[0].Identifier)
	s.Equal("password", allInputs[1].Identifier)
	s.Equal(testEmailAttr, allInputs[2].Identifier)
}

func (s *PromptOnlyNodeTestSuite) TestGetAllInputsEmpty() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(*promptNode)

	// No prompts set
	allInputs := promptNode.getAllInputs()
	s.NotNil(allInputs)
	s.Len(allInputs, 0, "Should return empty slice when no prompts")
}

func (s *PromptOnlyNodeTestSuite) TestGetAllActions() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(*promptNode)

	// Set multiple prompts with actions
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
			},
			Action: &common.Action{Ref: "login", NextNode: "auth_node"},
		},
		{
			Action: &common.Action{Ref: "signup", NextNode: "register_node"},
		},
		{
			Inputs: []common.Input{
				{Identifier: testEmailAttr, Required: true},
			},
			Action: &common.Action{Ref: "reset", NextNode: "reset_node"},
		},
	})

	// Test getAllActions
	allActions := promptNode.getAllActions()
	s.Len(allActions, 3, "Should return all actions from all prompts")
	s.Equal("login", allActions[0].Ref)
	s.Equal("signup", allActions[1].Ref)
	s.Equal("reset", allActions[2].Ref)
}

func (s *PromptOnlyNodeTestSuite) TestGetAllActionsWithNilAction() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(*promptNode)

	// Set prompts with some nil actions
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
			},
			Action: &common.Action{Ref: "login", NextNode: "auth_node"},
		},
		{
			Inputs: []common.Input{
				{Identifier: testEmailAttr, Required: true},
			},
			Action: nil, // No action
		},
		{
			Action: &common.Action{Ref: "signup", NextNode: "register_node"},
		},
	})

	// Test getAllActions - should only return non-nil actions
	allActions := promptNode.getAllActions()
	s.Len(allActions, 2, "Should only return non-nil actions")
	s.Equal("login", allActions[0].Ref)
	s.Equal("signup", allActions[1].Ref)
}

func (s *PromptOnlyNodeTestSuite) TestGetAllActionsEmpty() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(*promptNode)

	// No prompts set
	allActions := promptNode.getAllActions()
	s.NotNil(allActions)
	s.Len(allActions, 0, "Should return empty slice when no prompts")
}

func (s *PromptOnlyNodeTestSuite) TestGetNextNodeForActionRef() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(*promptNode)

	// Set prompts with multiple actions
	promptNode.SetPrompts([]common.Prompt{
		{
			Action: &common.Action{Ref: "login", NextNode: "auth_node"},
		},
		{
			Action: &common.Action{Ref: "signup", NextNode: "register_node"},
		},
		{
			Action: &common.Action{Ref: "cancel", NextNode: "exit_node"},
		},
	})

	// Test finding existing actions
	nextNode := promptNode.getNextNodeForActionRef(context.Background(), "login")
	s.Equal("auth_node", nextNode)

	nextNode = promptNode.getNextNodeForActionRef(context.Background(), "signup")
	s.Equal("register_node", nextNode)

	nextNode = promptNode.getNextNodeForActionRef(context.Background(), "cancel")
	s.Equal("exit_node", nextNode)
}

func (s *PromptOnlyNodeTestSuite) TestGetNextNodeForActionRefNotFound() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(*promptNode)

	// Set prompts with actions
	promptNode.SetPrompts([]common.Prompt{
		{
			Action: &common.Action{Ref: "login", NextNode: "auth_node"},
		},
		{
			Action: &common.Action{Ref: "signup", NextNode: "register_node"},
		},
	})

	// Test finding non-existent action
	nextNode := promptNode.getNextNodeForActionRef(context.Background(), "nonexistent")
	s.Equal("", nextNode, "Should return empty string when action not found")
}

func (s *PromptOnlyNodeTestSuite) TestGetNextNodeForActionRefEmptyRef() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(*promptNode)

	// Set prompts with actions
	promptNode.SetPrompts([]common.Prompt{
		{
			Action: &common.Action{Ref: "login", NextNode: "auth_node"},
		},
	})

	// Test with empty action ref
	nextNode := promptNode.getNextNodeForActionRef(context.Background(), "")
	s.Equal("", nextNode, "Should return empty string for empty action ref")
}

func (s *PromptOnlyNodeTestSuite) TestAutoSelectClearsActionsFromResponse() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	// Single action with inputs - should auto-select
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "auth_node"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "", // No action selected
		UserInputs: map[string]string{
			"username": "testuser",
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusComplete, resp.Status)
	s.Len(resp.Actions, 0, "Actions should be cleared after auto-selection")
	s.Equal("submit", ctx.CurrentAction, "Action should be auto-selected in context")
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithFailureAndRecovery() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
				{Identifier: "password", Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	// First execution with failure
	svcErr := serviceerror.ServiceError{Error: i18ncore.I18nMessage{DefaultValue: "Invalid credentials"}}
	svcErrJSON, _ := json.Marshal(svcErr)
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{
			"failureReasonJSON": string(svcErrJSON),
			"otherData":         "should remain",
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.NotNil(resp.Error)
	s.Equal("Invalid credentials", resp.Error.Error.DefaultValue)
	s.NotContains(ctx.RuntimeData, "failureReasonJSON", "Failure reason should be removed")
	s.Contains(ctx.RuntimeData, "otherData", "Other runtime data should remain")

	// Second execution with correct inputs (recovery)
	ctx.CurrentAction = "submit"
	ctx.UserInputs = map[string]string{
		"username": "testuser",
		"password": "testpass",
	}
	resp, err = node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusComplete, resp.Status)
	s.Nil(resp.Error, "Should not have failure error on success")
	s.Equal("next", resp.NextNodeID)
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithForwardedDataOptions() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{
					Ref:        "usertype_input",
					Identifier: "userType",
					Type:       "SELECT",
					Required:   true,
					Options:    []string{}, // Empty in prompt definition
				},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	// Execute with ForwardedData containing inputs with options
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyInputs: []common.Input{
				{
					Identifier: "userType",
					Type:       "SELECT",
					Options:    []string{"employee", "customer", "partner"},
				},
			},
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Len(resp.Inputs, 1)

	// Verify the input is enriched with options from ForwardedData
	enrichedInput := resp.Inputs[0]
	s.Equal("userType", enrichedInput.Identifier)
	s.Equal("usertype_input", enrichedInput.Ref, "Ref from prompt definition should be preserved")
	s.Equal("SELECT", enrichedInput.Type, "Type from prompt definition should be preserved")
	s.True(enrichedInput.Required, "Required from prompt definition should be preserved")
	s.ElementsMatch([]string{"employee", "customer", "partner"}, enrichedInput.Options,
		"Options should be enriched from ForwardedData")
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithForwardedDataNoMatch() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	// ForwardedData has an input with a different Identifier — it is a schema-derived input
	// that is not in the node's prompt definition, so it gets appended to the response.
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyInputs: []common.Input{
				{
					Identifier: "userType",
					Options:    []string{"option1", "option2"},
				},
			},
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	// username (from prompt) + userType (schema-derived from ForwardedData) are both missing
	s.Len(resp.Inputs, 2)

	inputMap := make(map[string]common.Input, len(resp.Inputs))
	for _, inp := range resp.Inputs {
		inputMap[inp.Identifier] = inp
	}
	s.Contains(inputMap, "username", "prompt-defined input must be present")
	s.Empty(inputMap["username"].Options, "username Options should remain empty")
	s.Contains(inputMap, "userType", "schema-derived input must be appended")
	s.ElementsMatch([]string{"option1", "option2"}, inputMap["userType"].Options,
		"forwarded options must be propagated to the appended input")
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithForwardedDataSchemaInputSkippedWhenScalarResolved() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	// ForwardedDataKeyInputs lists "userType" as a schema-derived input, but
	// ForwardedData also carries a resolved scalar string for "userType". The
	// scalar wins: the input must NOT be re-appended to the response.
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{"username": "alice"},
		ForwardedData: map[string]interface{}{
			"userType": "customer",
			common.ForwardedDataKeyInputs: []common.Input{
				{Identifier: "userType", Options: []string{"customer", "admin"}},
			},
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	for _, inp := range resp.Inputs {
		s.NotEqual("userType", inp.Identifier,
			"schema-derived input must be skipped when a scalar value is already forwarded")
	}
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithNoForwardedData() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "userType", Type: "SELECT", Required: true, Options: []string{}},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	// No ForwardedData
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Len(resp.Inputs, 1)

	// Verify options remain empty
	promptInput := resp.Inputs[0]
	s.Equal("userType", promptInput.Identifier)
	s.Empty(promptInput.Options, "Options should remain empty without ForwardedData")
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithForwardedDataMultipleInputs() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "userType", Type: "SELECT", Required: true, Options: []string{}},
				{Identifier: "region", Type: "SELECT", Required: true, Options: []string{}},
				{Identifier: "username", Type: "TEXT", Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	// ForwardedData has options for only userType
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyInputs: []common.Input{
				{
					Identifier: "userType",
					Type:       "SELECT",
					Options:    []string{"employee", "customer"},
				},
			},
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Len(resp.Inputs, 3)

	// Find each input and verify
	var userTypeInput, regionInput, usernameInput *common.Input
	for i := range resp.Inputs {
		switch resp.Inputs[i].Identifier {
		case "userType":
			userTypeInput = &resp.Inputs[i]
		case "region":
			regionInput = &resp.Inputs[i]
		case "username":
			usernameInput = &resp.Inputs[i]
		}
	}

	s.NotNil(userTypeInput)
	s.NotNil(regionInput)
	s.NotNil(usernameInput)

	// Only userType should be enriched
	s.ElementsMatch([]string{"employee", "customer"}, userTypeInput.Options)
	s.Empty(regionInput.Options, "Region options should remain empty")
	s.Empty(usernameInput.Options, "Username should have no options")
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithForwardedDataNonInputType() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "userType", Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	// ForwardedData has wrong type (string instead of []common.Input)
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyInputs: "not-an-input-slice",
		},
	}

	// Should not panic, should handle gracefully
	s.NotPanics(func() {
		resp, err := node.Execute(ctx)
		s.Nil(err)
		s.NotNil(resp)
		s.Len(resp.Inputs, 1)
		s.Empty(resp.Inputs[0].Options, "Options should remain empty with invalid ForwardedData type")
	})
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithForwardedDataPreservesPromptFields() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{
					Ref:        "usertype_input_custom",
					Identifier: "userType",
					Type:       "SELECT",
					Required:   true,
					Options:    []string{},
				},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	// ForwardedData has different Ref and Type (should NOT overwrite)
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyInputs: []common.Input{
				{
					Ref:        "different_ref",     // Should NOT overwrite
					Identifier: "userType",          // Match by this
					Type:       "DIFFERENT_TYPE",    // Should NOT overwrite
					Required:   false,               // Should NOT overwrite
					Options:    []string{"option1"}, // Should enrich
				},
			},
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Len(resp.Inputs, 1)

	// Verify prompt definition fields are preserved; options are NOT enriched because the
	// forwarded input type ("DIFFERENT_TYPE") does not match the node input type ("SELECT").
	enrichedInput := resp.Inputs[0]
	s.Equal("usertype_input_custom", enrichedInput.Ref, "Ref should NOT be overwritten")
	s.Equal("userType", enrichedInput.Identifier)
	s.Equal("SELECT", enrichedInput.Type, "Type should NOT be overwritten")
	s.True(enrichedInput.Required, "Required should NOT be overwritten")
	s.Empty(enrichedInput.Options, "Options should NOT be enriched when forwarded type does not match")
}

func (s *PromptOnlyNodeTestSuite) TestExecuteWithForwardedDataEmptyOptions() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "userType", Type: "SELECT", Required: true, Options: []string{"default"}},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	// ForwardedData has matching input but with empty options
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyInputs: []common.Input{
				{
					Identifier: "userType",
					Options:    []string{}, // Empty options
				},
			},
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Len(resp.Inputs, 1)

	// Verify options are NOT enriched when ForwardedData has empty options
	promptInput := resp.Inputs[0]
	s.Equal("userType", promptInput.Identifier)
	s.ElementsMatch([]string{"default"}, promptInput.Options,
		"Options should not be overwritten with empty options from ForwardedData")
}

func (s *PromptOnlyNodeTestSuite) TestSetAndGetNextNode() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	promptNode.SetNextNode("next-node-id")

	s.Equal("next-node-id", promptNode.GetNextNode())
}

func (s *PromptOnlyNodeTestSuite) TestSetAndGetMessage() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	message := "Welcome to the system"
	promptNode.SetMessage(message)

	s.Equal(message, promptNode.GetMessage())
}

func (s *PromptOnlyNodeTestSuite) TestIsDisplayOnly_False_WhenNoNextNode() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	s.False(promptNode.IsDisplayOnly(), "Should not be display-only without next node")
}

func (s *PromptOnlyNodeTestSuite) TestIsDisplayOnly_False_WhenHasPrompts() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	promptNode.SetNextNode("next-node")
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
			},
		},
	})

	s.False(promptNode.IsDisplayOnly(), "Should not be display-only when has prompts")
}

func (s *PromptOnlyNodeTestSuite) TestIsDisplayOnly_True_WithNextNodeAndNoPrompts() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	promptNode.SetNextNode("next-node")
	promptNode.SetPrompts([]common.Prompt{})

	s.True(promptNode.IsDisplayOnly(), "Should be display-only with next node and no prompts")
}

func (s *PromptOnlyNodeTestSuite) TestExecuteDisplayOnlyPrompt_WithMessage() {
	meta := map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{
				"type": "TEXT",
				"text": "Display only content",
			},
		},
	}

	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	promptNode.SetNextNode("next-node")
	promptNode.SetMessage("Please wait...")
	promptNode.SetMeta(meta)
	promptNode.SetPrompts([]common.Prompt{})

	// Execute with verbose mode to get meta
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		Verbose:     true,
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusComplete, resp.Status)
	s.Equal(common.NodeResponseTypeView, resp.Type)
	s.NotNil(resp.AdditionalData)
	s.Equal("Please wait...", resp.AdditionalData[common.DataPromptMessage])
	s.Equal(meta, resp.Meta)
}

func (s *PromptOnlyNodeTestSuite) TestExecuteDisplayOnlyPrompt_WithoutMessage() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	promptNode.SetNextNode("next-node")
	promptNode.SetPrompts([]common.Prompt{})

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusComplete, resp.Status)
	s.Equal(common.NodeResponseTypeView, resp.Type)
	// AdditionalData should not have message key if message is empty
	if resp.AdditionalData != nil {
		_, exists := resp.AdditionalData[common.DataPromptMessage]
		s.False(exists, "Message should not be in AdditionalData when empty")
	}
}

func (s *PromptOnlyNodeTestSuite) TestExecuteDisplayOnlyPrompt_IgnoresUserInputs() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	promptNode.SetNextNode("next-node")
	promptNode.SetPrompts([]common.Prompt{})

	// Even though user inputs are provided, display-only prompt should ignore them
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{"username": "user123"},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusComplete, resp.Status)
	s.Equal(common.NodeResponseTypeView, resp.Type)
}

func (s *PromptOnlyNodeTestSuite) TestExecuteDisplayOnlyPrompt_WithVerboseModeDisabled() {
	meta := map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{
				"type": "TEXT",
			},
		},
	}

	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	promptNode.SetNextNode("next-node")
	promptNode.SetMeta(meta)
	promptNode.SetPrompts([]common.Prompt{})

	// Execute with verbose mode disabled
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		Verbose:     false,
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusComplete, resp.Status)
	s.Nil(resp.Meta, "Meta should not be included when verbose mode is disabled")
}

func (s *PromptOnlyNodeTestSuite) TestGetActionTypeForRef_FoundWithType() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(*promptNode)

	pn.SetPrompts([]common.Prompt{
		{
			Action: &common.Action{Ref: "action_1", Type: "login", NextNode: "auth"},
		},
		{
			Action: &common.Action{Ref: "action_2", Type: "social", NextNode: "social_auth"},
		},
	})

	s.Equal("login", pn.getActionTypeForRef("action_1"))
	s.Equal("social", pn.getActionTypeForRef("action_2"))
}

func (s *PromptOnlyNodeTestSuite) TestExecuteActionTypeForwarding() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
			},
			Action: &common.Action{Ref: "login_action", Type: "password_login", NextNode: "auth_node"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "login_action",
		UserInputs:    map[string]string{"username": "testuser"},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusComplete, resp.Status)
	// Verify action type is forwarded in ForwardedData
	s.NotNil(resp.ForwardedData)
	s.Equal("password_login", resp.ForwardedData[common.ForwardedDataKeyActionType])
}

func (s *PromptOnlyNodeTestSuite) TestExecuteActionTypeForwarding_MultipleActions() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	promptNode.SetPrompts([]common.Prompt{
		{
			Action: &common.Action{Ref: "google", Type: "social_google", NextNode: "google_auth"},
		},
		{
			Action: &common.Action{Ref: "github", Type: "social_github", NextNode: "github_auth"},
		},
	})

	// Test with google action
	ctx1 := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "google",
		UserInputs:    map[string]string{},
	}
	resp1, err1 := node.Execute(ctx1)

	s.Nil(err1)
	s.NotNil(resp1)
	s.NotNil(resp1.ForwardedData)
	s.Equal("social_google", resp1.ForwardedData[common.ForwardedDataKeyActionType])

	// Test with github action
	ctx2 := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "github",
		UserInputs:    map[string]string{},
	}
	resp2, err2 := node.Execute(ctx2)

	s.Nil(err2)
	s.NotNil(resp2)
	s.NotNil(resp2.ForwardedData)
	s.Equal("social_github", resp2.ForwardedData[common.ForwardedDataKeyActionType])
}

func (s *PromptOnlyNodeTestSuite) TestAppendMissingInputs_SkipsInputInRuntimeData() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: testEmailAttr, Ref: "input_email", Required: true},
				{Identifier: "username", Ref: "input_username", Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{},
		RuntimeData:   map[string]string{testEmailAttr: "user@example.com"},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Len(resp.Inputs, 1)
	s.Equal("username", resp.Inputs[0].Identifier, "email should be skipped because it is in RuntimeData")
}

func (s *PromptOnlyNodeTestSuite) TestAppendMissingInputs_SkipsInputInForwardedDataString() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: testEmailAttr, Ref: "input_email", Required: true},
				{Identifier: "username", Ref: "input_username", Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{},
		ForwardedData: map[string]interface{}{testEmailAttr: "user@example.com"},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Len(resp.Inputs, 1)
	s.Equal("username", resp.Inputs[0].Identifier, "email should be skipped because it is a string in ForwardedData")
}

func (s *PromptOnlyNodeTestSuite) TestAppendMissingInputs_DoesNotSkipForwardedDataNonString() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: testEmailAttr, Ref: "input_email", Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{},
		ForwardedData: map[string]interface{}{
			testEmailAttr: []common.Input{{Identifier: testEmailAttr}},
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Len(resp.Inputs, 1, "email should NOT be skipped because forwarded value is not a string")
}

func (s *PromptOnlyNodeTestSuite) TestAppendMissingInputs_RuntimeDataDoesNotAffectNonMatchingInputs() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)
	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: testEmailAttr, Ref: "input_email", Required: true},
				{Identifier: "username", Ref: "input_username", Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{},
		RuntimeData:   map[string]string{"someOtherKey": "value"},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Len(resp.Inputs, 2, "both inputs should appear because RuntimeData has no matching keys")
}

func (s *PromptOnlyNodeTestSuite) TestVerboseMetaTrimming_PartialInputSet() {
	meta := map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{"type": "TEXT", "id": "heading"},
			map[string]interface{}{
				"type": common.MetaComponentTypeBlock,
				"id":   "form_block",
				"components": []interface{}{
					map[string]interface{}{"type": "TEXT_INPUT", "id": "input_given_name"},
					map[string]interface{}{"type": "TEXT_INPUT", "id": "input_family_name"},
					map[string]interface{}{"type": "TEXT_INPUT", "id": "input_email"},
					map[string]interface{}{"type": "ACTION", "id": "action_submit"},
				},
			},
		},
	}

	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetMeta(meta)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "given_name", Ref: "input_given_name", Required: true},
				{Identifier: "family_name", Ref: "input_family_name", Required: true},
				{Identifier: testEmailAttr, Ref: "input_email", Required: true},
			},
			Action: &common.Action{Ref: "action_submit", NextNode: "next"},
		},
	})

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{testEmailAttr: "user@example.com"},
		Verbose:     true,
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Len(resp.Inputs, 2)
	s.NotNil(resp.Meta)

	respMeta, ok := resp.Meta.(map[string]interface{})
	s.True(ok)
	topComps, ok := respMeta["components"].([]interface{})
	s.True(ok)
	s.Len(topComps, 2)

	// TEXT heading is always kept
	headingComp, ok := topComps[0].(map[string]interface{})
	s.True(ok)
	s.Equal("heading", headingComp["id"])

	// BLOCK contains only the two remaining inputs and the action
	blockComp, ok := topComps[1].(map[string]interface{})
	s.True(ok)
	s.Equal("form_block", blockComp["id"])
	nestedComps, ok := blockComp["components"].([]interface{})
	s.True(ok)
	s.Len(nestedComps, 3)

	ids := make([]string, 0, len(nestedComps))
	for _, c := range nestedComps {
		if m, ok := c.(map[string]interface{}); ok {
			ids = append(ids, m["id"].(string))
		}
	}
	s.ElementsMatch([]string{"input_given_name", "input_family_name", "action_submit"}, ids)
}

func (s *PromptOnlyNodeTestSuite) TestVerboseMetaTrimming_AllInputsMissing() {
	meta := map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{"type": "TEXT", "id": "heading"},
			map[string]interface{}{
				"type": common.MetaComponentTypeBlock,
				"id":   "form_block",
				"components": []interface{}{
					map[string]interface{}{"type": "TEXT_INPUT", "id": "input_given_name"},
					map[string]interface{}{"type": "TEXT_INPUT", "id": "input_family_name"},
					map[string]interface{}{"type": "TEXT_INPUT", "id": "input_email"},
					map[string]interface{}{"type": "ACTION", "id": "action_submit"},
				},
			},
		},
	}

	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetMeta(meta)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "given_name", Ref: "input_given_name", Required: true},
				{Identifier: "family_name", Ref: "input_family_name", Required: true},
				{Identifier: testEmailAttr, Ref: "input_email", Required: true},
			},
			Action: &common.Action{Ref: "action_submit", NextNode: "next"},
		},
	})

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
		Verbose:     true,
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Len(resp.Inputs, 3)
	s.NotNil(resp.Meta)

	respMeta, ok := resp.Meta.(map[string]interface{})
	s.True(ok)
	topComps, ok := respMeta["components"].([]interface{})
	s.True(ok)
	s.Len(topComps, 2)

	blockComp, ok := topComps[1].(map[string]interface{})
	s.True(ok)
	nestedComps, ok := blockComp["components"].([]interface{})
	s.True(ok)
	s.Len(nestedComps, 4, "all components should be present when all inputs are missing")
}

func (s *PromptOnlyNodeTestSuite) TestVerboseMetaTrimming_AllInputsSatisfied_ActionOnly() {
	meta := map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{"type": "TEXT_INPUT", "id": "input_given_name"},
			map[string]interface{}{"type": "ACTION", "id": "action_submit"},
			map[string]interface{}{"type": "ACTION", "id": "action_cancel"},
		},
	}

	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetMeta(meta)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "given_name", Ref: "input_given_name", Required: true},
			},
			Action: &common.Action{Ref: "action_submit", NextNode: "next"},
		},
		{
			Action: &common.Action{Ref: "action_cancel", NextNode: "exit"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "",
		UserInputs:    map[string]string{"given_name": "Alice"},
		Verbose:       true,
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Len(resp.Inputs, 0, "all inputs satisfied")
	s.Len(resp.Actions, 2)
	s.NotNil(resp.Meta)

	respMeta, ok := resp.Meta.(map[string]interface{})
	s.True(ok)
	comps, ok := respMeta["components"].([]interface{})
	s.True(ok)
	s.Len(comps, 2, "input component should be dropped; only action components remain")

	ids := make([]string, 0, len(comps))
	for _, c := range comps {
		if m, ok := c.(map[string]interface{}); ok {
			ids = append(ids, m["id"].(string))
		}
	}
	s.ElementsMatch([]string{"action_submit", "action_cancel"}, ids)
}

func (s *PromptOnlyNodeTestSuite) TestVerboseMetaTrimming_DisabledWhenVerboseFalse() {
	meta := map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{"type": "TEXT_INPUT", "id": "input_email"},
			map[string]interface{}{"type": "TEXT_INPUT", "id": "input_username"},
			map[string]interface{}{"type": "ACTION", "id": "action_submit"},
		},
	}

	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetMeta(meta)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: testEmailAttr, Ref: "input_email", Required: true},
				{Identifier: "username", Ref: "input_username", Required: true},
			},
			Action: &common.Action{Ref: "action_submit", NextNode: "next"},
		},
	})

	// email is satisfied via RuntimeData; username is still missing — partial inputs
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{testEmailAttr: "user@example.com"},
		Verbose:     false,
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Nil(resp.Meta, "Meta should be nil when verbose is false regardless of input state")
}

func (s *PromptOnlyNodeTestSuite) TestVerboseMetaTrimming_MetaNotMapStructure() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetMeta("plain string meta")
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Ref: "input_username", Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		Verbose:     true,
	}

	s.NotPanics(func() {
		resp, err := node.Execute(ctx)
		s.Nil(err)
		s.NotNil(resp)
		s.Equal("plain string meta", resp.Meta, "non-map meta should be returned unchanged")
	})
}

func (s *PromptOnlyNodeTestSuite) TestExecuteActionTypeForwarding_NoTypeField() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	promptNode := node.(PromptNodeInterface)

	promptNode.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{
				{Identifier: "username", Required: true},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
			// No Type field
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{"username": "testuser"},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusComplete, resp.Status)
	// ForwardedData should not have actionType when action has no type
	if resp.ForwardedData != nil {
		actionType, exists := resp.ForwardedData[common.ForwardedDataKeyActionType]
		if exists {
			s.Empty(actionType, "Action type should be empty when not defined")
		}
	}
}

// ── enrichInputsFromForwardedData — schema-derived input injection ────────────

func (s *PromptOnlyNodeTestSuite) TestEnrichInputsFromForwardedData_AddsInputNotInNodeResponse() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	// No prompt inputs configured — the schema-derived input comes only from ForwardedData.

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyInputs: []common.Input{
				{Identifier: testEmailAttr, Type: "TEXT_INPUT", Required: true, DisplayName: "Email Address"},
			},
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Len(resp.Inputs, 1)
	s.Equal(testEmailAttr, resp.Inputs[0].Identifier)
}

func (s *PromptOnlyNodeTestSuite) TestEnrichInputsFromForwardedData_DoesNotDuplicateExistingInput() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{{Identifier: testEmailAttr, Required: true}},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{},
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyInputs: []common.Input{
				{Identifier: testEmailAttr, Type: "TEXT_INPUT", Required: true},
			},
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	emailCount := 0
	for _, inp := range resp.Inputs {
		if inp.Identifier == testEmailAttr {
			emailCount++
		}
	}
	s.Equal(1, emailCount, "email must appear exactly once, not duplicated from ForwardedData")
}

func (s *PromptOnlyNodeTestSuite) TestEnrichInputsFromForwardedData_MixedNewAndExisting() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{{Identifier: "username", Required: true}},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{},
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyInputs: []common.Input{
				{Identifier: "username", Required: true},
				{Identifier: testEmailAttr, Type: "TEXT_INPUT", Required: true},
			},
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)

	identifiers := make(map[string]int)
	for _, inp := range resp.Inputs {
		identifiers[inp.Identifier]++
	}
	s.Equal(1, identifiers["username"], "username must appear exactly once")
	s.Equal(1, identifiers[testEmailAttr], "email (schema-derived) must be appended once")
}

func (s *PromptOnlyNodeTestSuite) TestEnrichInputsFromForwardedData_NilForwardedData_NoChange() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{{Identifier: "username", Required: true}},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{},
		ForwardedData: nil,
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Len(resp.Inputs, 1)
	s.Equal("username", resp.Inputs[0].Identifier)
}

func (s *PromptOnlyNodeTestSuite) TestEnrichInputsFromForwardedData_UpdatesRequiredFlag() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{{Identifier: "email", Type: "TEXT_INPUT", Required: false}},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{},
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyInputs: []common.Input{
				{Identifier: "email", Type: "TEXT_INPUT", Required: true},
			},
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.Require().Len(resp.Inputs, 1)
	s.True(resp.Inputs[0].Required, "required flag must be promoted to true by ForwardedData")
}

func (s *PromptOnlyNodeTestSuite) TestEnrichInputsFromForwardedData_PropagatesSelectOptions() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{{Identifier: "role", Type: common.InputTypeSelect, Required: true}},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{},
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyInputs: []common.Input{
				{Identifier: "role", Type: common.InputTypeSelect, Required: true, Options: []string{"admin", "user"}},
			},
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.Require().Len(resp.Inputs, 1)
	s.Equal([]string{"admin", "user"}, resp.Inputs[0].Options, "options must be propagated from ForwardedData")
}

func (s *PromptOnlyNodeTestSuite) TestHasRequiredInputs_UnknownActionFallsBackToAllInputs() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{{Identifier: testInputName, Type: "TEXT_INPUT", Required: true}},
			Action: &common.Action{Ref: "known_action", NextNode: "next"},
		},
	})

	// CurrentAction is set but does not match any prompt action.
	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "unknown_action",
		UserInputs:    map[string]string{},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.Require().NotNil(resp)
	// Falls back to all inputs — username must be in the response.
	found := false
	for _, inp := range resp.Inputs {
		if inp.Identifier == testInputName {
			found = true
		}
	}
	s.True(found, "when action is unknown, all prompt inputs must be requested")
}

// ── appendSyntheticMetaComponents — meta synthesis ───────────────────────────

func (s *PromptOnlyNodeTestSuite) TestSyntheticMeta_CreatedForSchemaInputWithDisplayName() {
	meta := map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{"id": "input_username", "type": "TEXT_INPUT"},
		},
	}
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetMeta(meta)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{{Ref: "input_username", Identifier: "username", Required: true}},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{},
		Verbose:       true,
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyInputs: []common.Input{
				{Identifier: testEmailAttr, Type: "TEXT_INPUT", Required: true, DisplayName: "Email Address"},
			},
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.NotNil(resp.Meta)

	metaMap, ok := resp.Meta.(map[string]interface{})
	s.Require().True(ok)
	comps, ok := metaMap["components"].([]interface{})
	s.Require().True(ok)

	// No BLOCK in original meta — synthetic input must be inside a generated BLOCK.
	var emailComp map[string]interface{}
	for _, c := range comps {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if cm["type"] == common.MetaComponentTypeBlock {
			for _, child := range cm["components"].([]interface{}) {
				if childMap, ok := child.(map[string]interface{}); ok && childMap["id"] == testEmailAttr {
					emailComp = childMap
				}
			}
		}
	}
	s.Require().NotNil(emailComp, "synthetic component for email should be present inside a BLOCK")
	s.Equal("Email Address", emailComp["label"], "label should use DisplayName")
	s.Equal("TEXT_INPUT", emailComp["type"], "type should match input.Type, not a generic INPUT string")
	s.Equal(testEmailAttr, emailComp["ref"], "ref should be set to the identifier")
	s.Equal(true, emailComp["required"], "required should be propagated")
}

func (s *PromptOnlyNodeTestSuite) TestSyntheticMeta_FallsBackToIdentifierWhenNoDisplayName() {
	meta := map[string]interface{}{
		"components": []interface{}{},
	}
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetMeta(meta)
	// No prompt inputs — schema-derived input comes from ForwardedData
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		Verbose:     true,
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyInputs: []common.Input{
				{Identifier: testEmailAttr, Type: "TEXT_INPUT", Required: true, DisplayName: ""},
			},
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp.Meta)

	metaMap, ok := resp.Meta.(map[string]interface{})
	s.Require().True(ok)
	comps, ok := metaMap["components"].([]interface{})
	s.Require().True(ok)
	s.Require().Len(comps, 1, "a single generated BLOCK should be present at the top level")

	block, ok := comps[0].(map[string]interface{})
	s.Require().True(ok)
	s.Equal(common.MetaComponentTypeBlock, block["type"], "synthetic inputs must be wrapped in a BLOCK")
	blockChildren, ok := block["components"].([]interface{})
	s.Require().True(ok)
	s.Require().Len(blockChildren, 1)

	comp := blockChildren[0].(map[string]interface{})
	s.Equal(testEmailAttr, comp["label"], "label must fall back to Identifier when DisplayName is empty")
}

func (s *PromptOnlyNodeTestSuite) TestSyntheticMeta_NotAddedWhenMetaComponentAlreadyExists() {
	meta := map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{"id": testEmailAttr, "type": "TEXT_INPUT", "label": "E-mail"},
		},
	}
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetMeta(meta)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{{Ref: testEmailAttr, Identifier: testEmailAttr, Required: true}},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{},
		Verbose:       true,
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyInputs: []common.Input{
				{Identifier: testEmailAttr, Type: "TEXT_INPUT", Required: true, DisplayName: "Email Address"},
			},
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp.Meta)

	metaMap, ok := resp.Meta.(map[string]interface{})
	s.Require().True(ok)
	comps, ok := metaMap["components"].([]interface{})
	s.Require().True(ok)

	emailCount := 0
	for _, c := range comps {
		if cm, ok := c.(map[string]interface{}); ok && cm["id"] == testEmailAttr {
			emailCount++
		}
	}
	s.Equal(1, emailCount, "email component must not be duplicated when it already exists in meta")
}

func (s *PromptOnlyNodeTestSuite) TestSyntheticMeta_NotAddedWhenMetaComponentExistsByRef() {
	meta := map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{"id": "input_email", "ref": testEmailAttr, "type": "TEXT_INPUT", "label": "E-mail"},
		},
	}
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetMeta(meta)

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		Verbose:     true,
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyInputs: []common.Input{
				{Identifier: testEmailAttr, Type: "TEXT_INPUT", Required: true, DisplayName: "Email Address"},
			},
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp.Meta)

	metaMap, ok := resp.Meta.(map[string]interface{})
	s.Require().True(ok)
	comps, ok := metaMap["components"].([]interface{})
	s.Require().True(ok)

	emailCount := 0
	for _, c := range comps {
		if cm, ok := c.(map[string]interface{}); ok {
			if cm["ref"] == testEmailAttr || cm["id"] == testEmailAttr {
				emailCount++
			}
		}
	}
	s.Equal(1, emailCount, "email component must not be duplicated when meta component ref matches identifier")
}

func (s *PromptOnlyNodeTestSuite) TestSyntheticMeta_ExistingComponentPromotedToPasswordType() {
	meta := map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{
				"id":       "input_pin",
				"ref":      "pin",
				"type":     "TEXT_INPUT",
				"label":    "PIN",
				"required": false,
			},
		},
	}
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetMeta(meta)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{{Ref: "pin", Identifier: "pin", Required: true}},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{},
		Verbose:       true,
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyInputs: []common.Input{
				{Identifier: "pin", Type: common.InputTypePassword, Required: true, DisplayName: "PIN"},
			},
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.NotNil(resp.Meta)
	s.Require().Len(resp.Inputs, 1)
	s.Equal(common.InputTypePassword, resp.Inputs[0].Type)

	metaMap, ok := resp.Meta.(map[string]interface{})
	s.Require().True(ok)
	comps, ok := metaMap["components"].([]interface{})
	s.Require().True(ok)
	s.Require().Len(comps, 1)

	comp, ok := comps[0].(map[string]interface{})
	s.Require().True(ok)
	s.Equal(common.InputTypePassword, comp["type"], "existing meta component must be promoted to password type")
	s.Equal(true, comp["required"], "existing meta component must reflect promoted required flag")
}

func (s *PromptOnlyNodeTestSuite) TestSyntheticMeta_PromotionDoesNotMutateSharedMeta() {
	original := map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{
				"id":       "input_pin",
				"ref":      "pin",
				"type":     "TEXT_INPUT",
				"required": false,
			},
		},
	}
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetMeta(original)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{{Ref: "pin", Identifier: "pin", Required: true}},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{},
		Verbose:       true,
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyInputs: []common.Input{
				{Identifier: "pin", Type: common.InputTypePassword, Required: true},
			},
		},
	}

	_, err := node.Execute(ctx)
	s.Require().Nil(err)

	// The original meta map must be untouched after the first execution.
	origComps, _ := original["components"].([]interface{})
	origComp, _ := origComps[0].(map[string]interface{})
	s.Equal("TEXT_INPUT", origComp["type"], "original meta component type must not be mutated by promotion")
	s.Equal(false, origComp["required"], "original meta component required must not be mutated by promotion")

	// A second execution must still see the correct (unpromoted) original.
	resp2, err := node.Execute(ctx)
	s.Require().Nil(err)
	metaMap, _ := resp2.Meta.(map[string]interface{})
	comps, _ := metaMap["components"].([]interface{})
	s.Require().Len(comps, 1)
	comp, _ := comps[0].(map[string]interface{})
	s.Equal(common.InputTypePassword, comp["type"], "second execution must still promote the type correctly")
	s.Equal(true, comp["required"], "second execution must still promote required correctly")
}

func (s *PromptOnlyNodeTestSuite) TestSyntheticMeta_InsertedInsideBlockBeforeAction() {
	meta := map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{"id": "image", "type": "IMAGE"},
			map[string]interface{}{
				"id":   "block_schema",
				"type": common.MetaComponentTypeBlock,
				"components": []interface{}{
					map[string]interface{}{"id": "action_submit", "type": "ACTION", "ref": "action_submit"},
				},
			},
		},
	}
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetMeta(meta)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{},
			Action: &common.Action{Ref: "action_submit", NextNode: "provisioning"},
		},
	})

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		Verbose:     true,
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyInputs: []common.Input{
				{Identifier: "mobileNumber", Type: "TEXT_INPUT", Required: true, DisplayName: "Mobile Number"},
			},
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp.Meta)

	metaMap, ok := resp.Meta.(map[string]interface{})
	s.Require().True(ok)
	topComps, ok := metaMap["components"].([]interface{})
	s.Require().True(ok)
	s.Len(topComps, 2, "top-level component count must not change — synthetic input goes inside the BLOCK")

	// Find the BLOCK and inspect its children.
	var blockChildren []interface{}
	for _, c := range topComps {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if cm["type"] == common.MetaComponentTypeBlock {
			blockChildren, _ = cm["components"].([]interface{})
		}
	}
	s.Require().Len(blockChildren, 2, "BLOCK must contain the synthetic input and the original action")

	inputComp, ok := blockChildren[0].(map[string]interface{})
	s.Require().True(ok)
	s.Equal("mobileNumber", inputComp["id"], "synthetic input must be first — before the ACTION")
	s.Equal("TEXT_INPUT", inputComp["type"])
	s.Equal("mobileNumber", inputComp["ref"])
	s.Equal("Mobile Number", inputComp["label"])
	s.Equal(true, inputComp["required"])

	actionComp, ok := blockChildren[1].(map[string]interface{})
	s.Require().True(ok)
	s.Equal("ACTION", actionComp["type"], "ACTION must remain after the synthetic input")
}

func (s *PromptOnlyNodeTestSuite) TestSyntheticMeta_PlaceholderReplacedWithSyntheticInputs() {
	meta := map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{
				"id":   "block_dynamic",
				"type": common.MetaComponentTypeBlock,
				"components": []interface{}{
					map[string]interface{}{
						"id":   "dynamic_inputs",
						"type": common.MetaComponentTypeDynamicInputPlaceholder,
					},
					map[string]interface{}{"id": "action_submit", "type": "ACTION", "ref": "action_submit"},
				},
			},
		},
	}
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetMeta(meta)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{},
			Action: &common.Action{Ref: "action_submit", NextNode: "next"},
		},
	})

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		Verbose:     true,
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyInputs: []common.Input{
				{Identifier: testEmailAttr, Type: "TEXT_INPUT", Required: true, DisplayName: "Email"},
				{Identifier: "given_name", Type: "TEXT_INPUT", Required: true, DisplayName: "First Name"},
			},
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp.Meta)

	metaMap, ok := resp.Meta.(map[string]interface{})
	s.Require().True(ok)
	topComps, ok := metaMap["components"].([]interface{})
	s.Require().True(ok)
	s.Len(topComps, 1)

	block, ok := topComps[0].(map[string]interface{})
	s.Require().True(ok)
	s.Equal(common.MetaComponentTypeBlock, block["type"])

	children, ok := block["components"].([]interface{})
	s.Require().True(ok)
	// placeholder replaced by 2 synthetic inputs + 1 action = 3 total
	s.Require().Len(children, 3, "placeholder must be replaced by synthetic inputs; ACTION remains at end")

	// No placeholder in final output.
	for _, c := range children {
		if cm, ok := c.(map[string]interface{}); ok {
			s.NotEqual(common.MetaComponentTypeDynamicInputPlaceholder, cm["type"],
				"placeholder must not appear in the final meta")
		}
	}

	email, ok := children[0].(map[string]interface{})
	s.Require().True(ok)
	s.Equal(testEmailAttr, email["id"])
	s.Equal("Email", email["label"])

	action, ok := children[2].(map[string]interface{})
	s.Require().True(ok)
	s.Equal("ACTION", action["type"], "ACTION must stay after synthetic inputs")
}

func (s *PromptOnlyNodeTestSuite) TestSyntheticMeta_PlaceholderStrippedWhenNoSyntheticInputs() {
	meta := map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{
				"id":   "block_dynamic",
				"type": common.MetaComponentTypeBlock,
				"components": []interface{}{
					map[string]interface{}{
						"id":   "dynamic_inputs",
						"type": common.MetaComponentTypeDynamicInputPlaceholder,
					},
					map[string]interface{}{"id": "action_submit", "type": "ACTION", "ref": "action_submit"},
				},
			},
		},
	}
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetMeta(meta)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{},
			Action: &common.Action{Ref: "action_submit", NextNode: "next"},
		},
	})

	// No ForwardedData inputs — no synthetic inputs will be generated.
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		Verbose:     true,
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp.Meta)

	metaMap, ok := resp.Meta.(map[string]interface{})
	s.Require().True(ok)
	topComps, ok := metaMap["components"].([]interface{})
	s.Require().True(ok)

	block, ok := topComps[0].(map[string]interface{})
	s.Require().True(ok)
	children, ok := block["components"].([]interface{})
	s.Require().True(ok)
	// placeholder removed, only ACTION remains
	s.Require().Len(children, 1, "placeholder must be stripped even when there are no synthetic inputs")
	action, ok := children[0].(map[string]interface{})
	s.Require().True(ok)
	s.Equal("ACTION", action["type"])
	for _, c := range children {
		if cm, ok := c.(map[string]interface{}); ok {
			s.NotEqual(common.MetaComponentTypeDynamicInputPlaceholder, cm["type"],
				"placeholder must never appear in the final meta")
		}
	}
}

func (s *PromptOnlyNodeTestSuite) TestFilterMetaComponents_NonMapComponentPassedThrough() {
	// filterMetaComponents is exercised via trimMetaToRequestedInputs in verbose mode.
	// A non-map element (plain string) in the components list covers the !ok branch
	// that passes non-map items through unchanged. The node must be in an incomplete
	// state (no action selected, required input missing) so that meta is rendered.
	meta := map[string]interface{}{
		"components": []interface{}{
			"plain-string-component",
			map[string]interface{}{
				"id":   "block_inputs",
				"type": common.MetaComponentTypeBlock,
				"components": []interface{}{
					map[string]interface{}{"id": "input_username", "ref": "input_username", "type": "TEXT_INPUT"},
					map[string]interface{}{"id": "action_submit", "type": "ACTION", "ref": "action_submit"},
				},
			},
		},
	}
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetMeta(meta)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{{Ref: "input_username", Identifier: "username", Required: true}},
			Action: &common.Action{Ref: "action_submit", NextNode: "next"},
		},
	})

	// No action selected + required input missing → node is INCOMPLETE → meta rendered.
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		Verbose:     true,
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp.Meta)
	metaMap, ok := resp.Meta.(map[string]interface{})
	s.Require().True(ok)
	comps, ok := metaMap["components"].([]interface{})
	s.Require().True(ok)
	found := false
	for _, c := range comps {
		if str, ok := c.(string); ok && str == "plain-string-component" {
			found = true
		}
	}
	s.True(found, "non-map components must pass through filterMetaComponents unchanged")
}

func (s *PromptOnlyNodeTestSuite) TestEnrichInputsFromForwardedData_SkipsInputAlreadyInUserInputs() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{testEmailAttr: "user@example.com"},
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyInputs: []common.Input{
				{Identifier: testEmailAttr, Type: "TEXT_INPUT", Required: true},
				{Identifier: "username", Type: "TEXT_INPUT", Required: true},
			},
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	for _, inp := range resp.Inputs {
		s.NotEqual(testEmailAttr, inp.Identifier,
			"email already in UserInputs must not appear in missing inputs")
	}
	identifiers := make(map[string]bool)
	for _, inp := range resp.Inputs {
		identifiers[inp.Identifier] = true
	}
	s.True(identifiers["username"], "username not in UserInputs must be appended")
}

func (s *PromptOnlyNodeTestSuite) TestEnrichInputsFromForwardedData_SkipsInputAlreadyInRuntimeData() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{testEmailAttr: "user@example.com"},
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyInputs: []common.Input{
				{Identifier: testEmailAttr, Type: "TEXT_INPUT", Required: true},
				{Identifier: "username", Type: "TEXT_INPUT", Required: true},
			},
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	for _, inp := range resp.Inputs {
		s.NotEqual(testEmailAttr, inp.Identifier,
			"email already in RuntimeData must not appear in missing inputs")
	}
}

func (s *PromptOnlyNodeTestSuite) TestSyntheticMeta_EmptyInputType_FallsBackToText() {
	meta := map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{
				"id":   "block_inputs",
				"type": common.MetaComponentTypeBlock,
				"components": []interface{}{
					map[string]interface{}{
						"id":   "dynamic_inputs",
						"type": common.MetaComponentTypeDynamicInputPlaceholder,
					},
					map[string]interface{}{"id": "action_submit", "type": "ACTION", "ref": "action_submit"},
				},
			},
		},
	}
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetMeta(meta)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []common.Input{{Ref: "input_email", Identifier: testEmailAttr, Required: true}},
			Action: &common.Action{Ref: "action_submit", NextNode: "next"},
		},
	})

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyInputs: []common.Input{
				{Identifier: "schemaField", Type: "", Required: true, DisplayName: "Schema Field"},
			},
		},
		Verbose: true,
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp.Meta)

	metaMap, ok := resp.Meta.(map[string]interface{})
	s.Require().True(ok)
	comps, ok := metaMap["components"].([]interface{})
	s.Require().True(ok)

	for _, comp := range comps {
		blockMap, ok := comp.(map[string]interface{})
		if !ok || blockMap["type"] != common.MetaComponentTypeBlock {
			continue
		}
		inner, ok := blockMap["components"].([]interface{})
		s.Require().True(ok)
		for _, ic := range inner {
			icMap, ok := ic.(map[string]interface{})
			if !ok {
				continue
			}
			if icMap["ref"] == "schemaField" {
				s.Equal(common.InputTypeText, icMap["type"],
					"empty input type must fall back to TEXT")
				return
			}
		}
	}
	s.Fail("synthetic component for schemaField not found in meta")
}

func loginOptionsProps() map[string]interface{} {
	return map[string]interface{}{
		common.NodePropertyAuthMethodMapping: map[string]interface{}{
			"urn:thunder:acr:password":       "pwd",
			"urn:thunder:acr:generated-code": "otp",
			"urn:thunder:acr:linked-wallet":  "wallet",
		},
	}
}

func (s *PromptOnlyNodeTestSuite) TestLoginOptionsVariant_GetSetVariant() {
	node := newPromptNode("login-chooser", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)

	s.Equal(common.NodeVariant(""), pn.GetVariant())
	pn.SetVariant(common.NodeVariantLoginOptions)
	s.Equal(common.NodeVariantLoginOptions, pn.GetVariant())
}

func (s *PromptOnlyNodeTestSuite) TestLoginOptionsVariant_NoACRFilter_AllActionsReturned() {
	node := newPromptNode("login-chooser", loginOptionsProps(), false, false)
	pn := node.(PromptNodeInterface)
	pn.SetVariant(common.NodeVariantLoginOptions)
	pn.SetPrompts([]common.Prompt{
		{Action: &common.Action{Ref: "pwd", NextNode: "pwd-node"}},
		{Action: &common.Action{Ref: "otp", NextNode: "otp-node"}},
	})

	// No requested_acr_values in RuntimeData → all actions returned
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Len(resp.Actions, 2)
	refs := []string{resp.Actions[0].Ref, resp.Actions[1].Ref}
	s.ElementsMatch([]string{"pwd", "otp"}, refs)
	// On the prompt-out leg, allowed_login_options should record the action refs.
	s.NotEmpty(resp.RuntimeData[common.RuntimeKeyAllowedLoginOptions],
		"allowed_login_options must be set on the prompt-out leg")
}

func (s *PromptOnlyNodeTestSuite) TestLoginOptionsVariant_SingleACRFilter() {
	node := newPromptNode("login-chooser", loginOptionsProps(), false, false)
	pn := node.(PromptNodeInterface)
	pn.SetVariant(common.NodeVariantLoginOptions)
	pn.SetPrompts([]common.Prompt{
		{Action: &common.Action{Ref: "pwd", NextNode: "pwd-node"}},
		{Action: &common.Action{Ref: "otp", NextNode: "otp-node"}},
		{Action: &common.Action{Ref: "wallet", NextNode: "wallet-node"}},
	})

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{
			common.RuntimeKeyRequestedAuthClasses: "urn:thunder:acr:generated-code",
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.Equal(common.NodeStatusComplete, resp.Status,
		"login_options node must auto-select when only one ACR option remains after filtering")
	s.Equal("otp-node", resp.NextNodeID, "must forward to the next node for the auto-selected action")
	s.Empty(resp.Actions, "chooser actions must not be returned after auto-selection")
	s.Equal("otp", ctx.CurrentAction, "context must have the auto-selected action")
	s.Equal("urn:thunder:acr:generated-code", resp.RuntimeData[common.RuntimeKeySelectedAuthClass],
		"selected_auth_class must be recorded for the auto-selected ACR")
}

func (s *PromptOnlyNodeTestSuite) TestLoginOptionsVariant_PreferenceOrder() {
	node := newPromptNode("login-chooser", loginOptionsProps(), false, false)
	pn := node.(PromptNodeInterface)
	pn.SetVariant(common.NodeVariantLoginOptions)
	// Graph order: password first, then OTP
	pn.SetPrompts([]common.Prompt{
		{Action: &common.Action{Ref: "pwd", NextNode: "pwd-node"}},
		{Action: &common.Action{Ref: "otp", NextNode: "otp-node"}},
	})

	// Preference order: OTP first, then password
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{
			common.RuntimeKeyRequestedAuthClasses: "urn:thunder:acr:generated-code urn:thunder:acr:password",
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.Require().Len(resp.Actions, 2)
	s.Equal("otp", resp.Actions[0].Ref, "OTP should be first per preference order")
	s.Equal("pwd", resp.Actions[1].Ref, "password should be second per preference order")
}

func (s *PromptOnlyNodeTestSuite) TestLoginOptionsVariant_UntaggedPromptsAlwaysIncluded() {
	// authMethodMapping covers only "pwd"; "other" is not gated by ACR and should always appear.
	node := newPromptNode("login-chooser", map[string]interface{}{
		common.NodePropertyAuthMethodMapping: map[string]interface{}{
			"urn:thunder:acr:password": "pwd",
		},
	}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetVariant(common.NodeVariantLoginOptions)
	pn.SetPrompts([]common.Prompt{
		{Action: &common.Action{Ref: "pwd", NextNode: "pwd-node"}},
		// Action ref not in authMethodMapping — non-ACR-gated, should always be included
		{Action: &common.Action{Ref: "other", NextNode: "other-node"}},
	})

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{
			// Only password requested; non-gated prompt should still appear
			common.RuntimeKeyRequestedAuthClasses: "urn:thunder:acr:password",
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.Require().Len(resp.Actions, 2)
	refs := []string{resp.Actions[0].Ref, resp.Actions[1].Ref}
	s.Contains(refs, "pwd")
	s.Contains(refs, "other")
}

func (s *PromptOnlyNodeTestSuite) TestLoginOptionsVariant_GracefulFallback_NoMatchingACR() {
	node := newPromptNode("login-chooser", loginOptionsProps(), false, false)
	pn := node.(PromptNodeInterface)
	pn.SetVariant(common.NodeVariantLoginOptions)
	pn.SetPrompts([]common.Prompt{
		{Action: &common.Action{Ref: "pwd", NextNode: "pwd-node"}},
		{Action: &common.Action{Ref: "otp", NextNode: "otp-node"}},
	})

	// Requested ACR not present in any prompt → graceful fallback returns all
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{
			common.RuntimeKeyRequestedAuthClasses: "urn:thunder:acr:biometrics",
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.Require().Len(resp.Actions, 2, "all prompts should be returned as fallback")
}

func (s *PromptOnlyNodeTestSuite) TestLoginOptionsVariant_CompletedACRWritten() {
	node := newPromptNode("login-chooser", loginOptionsProps(), false, false)
	pn := node.(PromptNodeInterface)
	pn.SetVariant(common.NodeVariantLoginOptions)
	pn.SetPrompts([]common.Prompt{
		{Action: &common.Action{Ref: "pwd", NextNode: "pwd-node"}},
		{Action: &common.Action{Ref: "otp", NextNode: "otp-node"}},
	})

	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		UserInputs:    map[string]string{},
		CurrentAction: "pwd",
		RuntimeData:   map[string]string{},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.Equal(common.NodeStatusComplete, resp.Status)
	s.Equal("pwd-node", resp.NextNodeID)
	s.Equal("urn:thunder:acr:password", resp.RuntimeData[common.RuntimeKeySelectedAuthClass])
}

func (s *PromptOnlyNodeTestSuite) TestLoginOptionsVariant_CompletedACRWritten_WithACRFilter() {
	node := newPromptNode("login-chooser", loginOptionsProps(), false, false)
	pn := node.(PromptNodeInterface)
	pn.SetVariant(common.NodeVariantLoginOptions)
	pn.SetPrompts([]common.Prompt{
		{Action: &common.Action{Ref: "pwd", NextNode: "pwd-node"}},
		{Action: &common.Action{Ref: "otp", NextNode: "otp-node"}},
	})

	// Only OTP requested; user picks otp
	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		UserInputs:    map[string]string{},
		CurrentAction: "otp",
		RuntimeData: map[string]string{
			common.RuntimeKeyRequestedAuthClasses: "urn:thunder:acr:generated-code",
			common.RuntimeKeyAllowedLoginOptions:  "otp",
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.Equal(common.NodeStatusComplete, resp.Status)
	s.Equal("otp-node", resp.NextNodeID)
	s.Equal("urn:thunder:acr:generated-code", resp.RuntimeData[common.RuntimeKeySelectedAuthClass])
}

func (s *PromptOnlyNodeTestSuite) TestLoginOptionsVariant_DisallowedActionRejected() {
	node := newPromptNode("login-chooser", loginOptionsProps(), false, false)
	pn := node.(PromptNodeInterface)
	pn.SetVariant(common.NodeVariantLoginOptions)
	pn.SetPrompts([]common.Prompt{
		{Action: &common.Action{Ref: "pwd", NextNode: "pwd-node"}},
		{Action: &common.Action{Ref: "otp", NextNode: "otp-node"}},
	})

	// Allowed list (from a prior prompt-out leg) restricts to "otp" only.
	ctx := &NodeContext{
		ExecutionID:   "test-flow",
		UserInputs:    map[string]string{},
		CurrentAction: "pwd",
		RuntimeData: map[string]string{
			common.RuntimeKeyAllowedLoginOptions: "otp",
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.Equal(common.NodeStatusFailure, resp.Status,
		"selecting an action outside allowed_login_options must fail")
	s.NotNil(resp.Error, "Should set error when action is not in allowed_login_options")
}

func (s *PromptOnlyNodeTestSuite) TestLoginOptionsVariant_AllowedLoginOptionsCaptured() {
	node := newPromptNode("login-chooser", loginOptionsProps(), false, false)
	pn := node.(PromptNodeInterface)
	pn.SetVariant(common.NodeVariantLoginOptions)
	pn.SetPrompts([]common.Prompt{
		{Action: &common.Action{Ref: "pwd", NextNode: "pwd-node"}},
		{Action: &common.Action{Ref: "otp", NextNode: "otp-node"}},
		{Action: &common.Action{Ref: "wallet", NextNode: "wallet-node"}},
	})

	// Two requested ACRs ⇒ two allowed options on the prompt-out leg.
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{
			common.RuntimeKeyRequestedAuthClasses: "urn:thunder:acr:generated-code urn:thunder:acr:password",
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Equal("otp pwd", resp.RuntimeData[common.RuntimeKeyAllowedLoginOptions],
		"allowed_login_options must list refs in ACR-preference order")
}

func (s *PromptOnlyNodeTestSuite) TestNonLoginOptionsVariant_UnaffectedByACRValues() {
	node := newPromptNode("standard-prompt", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	// No variant set
	pn.SetPrompts([]common.Prompt{
		{Action: &common.Action{Ref: "pwd", NextNode: "pwd-node"}},
		{Action: &common.Action{Ref: "otp", NextNode: "otp-node"}},
	})

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{
			common.RuntimeKeyRequestedAuthClasses: "urn:thunder:acr:generated-code",
		},
	}
	resp, err := node.Execute(ctx)

	// Both actions must be returned — no filtering for non-login_options nodes
	s.Nil(err)
	s.Require().Len(resp.Actions, 2)
	s.Empty(resp.RuntimeData[common.RuntimeKeyAllowedLoginOptions],
		"allowed_login_options must not be set for non-login_options nodes")
}

func (s *PromptOnlyNodeTestSuite) TestFilteredMeta_ActionComponentsReorderedByACR() {
	node := newPromptNode("login-chooser", loginOptionsProps(), false, false)
	pn := node.(*promptNode)
	pn.SetVariant(common.NodeVariantLoginOptions)
	// Graph order: password, otp, wallet
	pn.SetPrompts([]common.Prompt{
		{Action: &common.Action{Ref: "pwd", NextNode: "pwd-node"}},
		{Action: &common.Action{Ref: "otp", NextNode: "otp-node"}},
		{Action: &common.Action{Ref: "wallet", NextNode: "wallet-node"}},
	})
	pn.SetMeta(map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{"type": "ACTION", "id": "pwd"},
			map[string]interface{}{"type": "ACTION", "id": "otp"},
			map[string]interface{}{"type": "ACTION", "id": "wallet"},
		},
	})

	// ACR preference: otp first, then wallet, then password
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		Verbose:     true,
		RuntimeData: map[string]string{
			common.RuntimeKeyRequestedAuthClasses: "urn:thunder:acr:generated-code " +
				"urn:thunder:acr:linked-wallet " + "urn:thunder:acr:password",
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Require().NotNil(resp.Meta)

	metaMap, ok := resp.Meta.(map[string]interface{})
	s.Require().True(ok)
	components, ok := metaMap["components"].([]interface{})
	s.Require().True(ok)
	s.Require().Len(components, 3)

	ids := make([]string, 3)
	for i, c := range components {
		ids[i], _ = c.(map[string]interface{})["id"].(string)
	}
	s.Equal([]string{"otp", "wallet", "pwd"}, ids, "ACTION components should follow ACR preference order")
}

func (s *PromptOnlyNodeTestSuite) TestFilteredMeta_NonActionComponentsRetainPosition() {
	node := newPromptNode("login-chooser", loginOptionsProps(), false, false)
	pn := node.(*promptNode)
	pn.SetVariant(common.NodeVariantLoginOptions)
	pn.SetPrompts([]common.Prompt{
		{Action: &common.Action{Ref: "pwd", NextNode: "pwd-node"}},
		{Action: &common.Action{Ref: "otp", NextNode: "otp-node"}},
	})
	pn.SetMeta(map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{"type": "TEXT", "id": "heading"},
			map[string]interface{}{"type": "ACTION", "id": "pwd"},
			map[string]interface{}{"type": "DIVIDER", "id": "div1"},
			map[string]interface{}{"type": "ACTION", "id": "otp"},
			map[string]interface{}{"type": "TEXT", "id": "footer"},
		},
	})

	// Preference: otp first, then password
	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		Verbose:     true,
		RuntimeData: map[string]string{
			common.RuntimeKeyRequestedAuthClasses: "urn:thunder:acr:generated-code urn:thunder:acr:password",
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.Require().NotNil(resp.Meta)

	metaMap := resp.Meta.(map[string]interface{})
	components := metaMap["components"].([]interface{})
	s.Require().Len(components, 5)

	ids := make([]string, 5)
	for i, c := range components {
		ids[i], _ = c.(map[string]interface{})["id"].(string)
	}
	// Non-ACTION components stay; ACTION slots are filled in ACR preference order
	s.Equal([]string{"heading", "otp", "div1", "pwd", "footer"}, ids)
}

func (s *PromptOnlyNodeTestSuite) TestFilteredMeta_FilteredOutActionsDropped() {
	node := newPromptNode("login-chooser", loginOptionsProps(), false, false)
	pn := node.(*promptNode)
	pn.SetVariant(common.NodeVariantLoginOptions)
	pn.SetPrompts([]common.Prompt{
		{Action: &common.Action{Ref: "pwd", NextNode: "pwd-node"}},
		{Action: &common.Action{Ref: "otp", NextNode: "otp-node"}},
		{Action: &common.Action{Ref: "wallet", NextNode: "wallet-node"}},
	})
	pn.SetMeta(map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{"type": "ACTION", "id": "pwd"},
			map[string]interface{}{"type": "ACTION", "id": "otp"},
			map[string]interface{}{"type": "ACTION", "id": "wallet"},
		},
	})

	ctx := &NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{},
		Verbose:     true,
		RuntimeData: map[string]string{
			common.RuntimeKeyRequestedAuthClasses: "urn:thunder:acr:generated-code",
		},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.Equal(common.NodeStatusComplete, resp.Status,
		"single ACR filter must trigger auto-selection and complete the node")
	s.Equal("otp-node", resp.NextNodeID)
	s.Nil(resp.Meta, "meta is not returned for a completed (auto-selected) chooser node")
}
