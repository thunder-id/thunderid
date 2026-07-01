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

package core

import (
	"testing"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
)

type InputValidationTestSuite struct {
	suite.Suite
}

func TestInputValidationTestSuite(t *testing.T) {
	suite.Run(t, new(InputValidationTestSuite))
}

// preparedRules pre-compiles regex patterns so tests bypassing the graph builder
// still exercise the regex code path.
func preparedRules(rules []providers.ValidationRule) []providers.ValidationRule {
	if err := common.PrepareValidationRules(rules); err != nil {
		panic(err)
	}
	return rules
}

func (s *InputValidationTestSuite) TestRegexPass() {
	inputs := []providers.Input{
		{
			Identifier: "email",
			Validation: preparedRules([]providers.ValidationRule{
				{Type: providers.ValidationTypeRegex, Value: "^[^@]+@[^@]+\\.[^@]+$",
					Message: "validation.email.invalid"},
			}),
		},
	}
	errs := validateInputValues(inputs, map[string]string{"email": "user@example.com"})
	s.Empty(errs)
}

func (s *InputValidationTestSuite) TestRegexFail() {
	inputs := []providers.Input{
		{
			Identifier: "email",
			Validation: preparedRules([]providers.ValidationRule{
				{Type: providers.ValidationTypeRegex, Value: "^[^@]+@[^@]+\\.[^@]+$",
					Message: "validation.email.invalid"},
			}),
		},
	}
	errs := validateInputValues(inputs, map[string]string{"email": "not-an-email"})
	s.Len(errs, 1)
	s.Equal("email", errs[0].Identifier)
	s.Equal("validation.email.invalid", errs[0].Message)
}

func (s *InputValidationTestSuite) TestMinLengthPass() {
	inputs := []providers.Input{
		{
			Identifier: "password",
			Validation: []providers.ValidationRule{
				{Type: providers.ValidationTypeMinLength, Value: float64(8),
					Message: "validation.password.minLength"},
			},
		},
	}
	errs := validateInputValues(inputs, map[string]string{"password": "12345678"})
	s.Empty(errs)
}

func (s *InputValidationTestSuite) TestMinLengthFail() {
	inputs := []providers.Input{
		{
			Identifier: "password",
			Validation: []providers.ValidationRule{
				{Type: providers.ValidationTypeMinLength, Value: float64(8),
					Message: "validation.password.minLength"},
			},
		},
	}
	errs := validateInputValues(inputs, map[string]string{"password": "abc"})
	s.Len(errs, 1)
	s.Equal("password", errs[0].Identifier)
	s.Equal("validation.password.minLength", errs[0].Message)
}

func (s *InputValidationTestSuite) TestMaxLengthPass() {
	inputs := []providers.Input{
		{
			Identifier: "username",
			Validation: []providers.ValidationRule{
				{Type: providers.ValidationTypeMaxLength, Value: float64(10),
					Message: "validation.username.maxLength"},
			},
		},
	}
	errs := validateInputValues(inputs, map[string]string{"username": "ten_chars_"})
	s.Empty(errs)
}

func (s *InputValidationTestSuite) TestMaxLengthFail() {
	inputs := []providers.Input{
		{
			Identifier: "username",
			Validation: []providers.ValidationRule{
				{Type: providers.ValidationTypeMaxLength, Value: float64(5),
					Message: "validation.username.maxLength"},
			},
		},
	}
	errs := validateInputValues(inputs, map[string]string{"username": "way_too_long"})
	s.Len(errs, 1)
	s.Equal("validation.username.maxLength", errs[0].Message)
}

func (s *InputValidationTestSuite) TestMultipleRulesPerFieldProduceSeparateEntries() {
	// Perl-style lookahead patterns are not supported by the regex engine; use alternation instead.
	inputs := []providers.Input{
		{
			Identifier: "password",
			Validation: preparedRules([]providers.ValidationRule{
				{Type: providers.ValidationTypeMinLength, Value: float64(8),
					Message: "validation.password.minLength"},
				{Type: providers.ValidationTypeRegex,
					Value:   "^(?:[^A-Z]*[A-Z][^0-9]*[0-9].*|[^0-9]*[0-9][^A-Z]*[A-Z].*)$",
					Message: "validation.password.complexity"},
			}),
		},
	}
	errs := validateInputValues(inputs, map[string]string{"password": "abc"})
	s.Len(errs, 2)
	s.Equal("password", errs[0].Identifier)
	s.Equal("validation.password.minLength", errs[0].Message)
	s.Equal("password", errs[1].Identifier)
	s.Equal("validation.password.complexity", errs[1].Message)
}

func (s *InputValidationTestSuite) TestMultipleFieldsErrorsInSinglePass() {
	inputs := []providers.Input{
		{
			Identifier: "username",
			Validation: preparedRules([]providers.ValidationRule{
				{Type: providers.ValidationTypeRegex, Value: "^[^@]+@[^@]+$",
					Message: "validation.email.invalid"},
			}),
		},
		{
			Identifier: "password",
			Validation: []providers.ValidationRule{
				{Type: providers.ValidationTypeMinLength, Value: float64(8),
					Message: "validation.password.minLength"},
			},
		},
	}
	errs := validateInputValues(inputs, map[string]string{
		"username": "no-at-sign",
		"password": "abc",
	})
	s.Len(errs, 2)
}

func (s *InputValidationTestSuite) TestDefaultMessageUsedWhenAbsent() {
	inputs := []providers.Input{
		{
			Identifier: "email",
			Validation: preparedRules([]providers.ValidationRule{
				{Type: providers.ValidationTypeRegex, Value: "^X+$"},
			}),
		},
	}
	errs := validateInputValues(inputs, map[string]string{"email": "abc"})
	s.Len(errs, 1)
	s.Equal(providers.DefaultValidationMessageRegex, errs[0].Message)
}

func (s *InputValidationTestSuite) TestMessagePassthroughI18nKey() {
	i18nKey := "{{i18n(validation:email.invalid)}}"
	inputs := []providers.Input{
		{
			Identifier: "email",
			Validation: preparedRules([]providers.ValidationRule{
				{Type: providers.ValidationTypeRegex, Value: "^X+$", Message: i18nKey},
			}),
		},
	}
	errs := validateInputValues(inputs, map[string]string{"email": "abc"})
	s.Len(errs, 1)
	s.Equal(i18nKey, errs[0].Message)
}

func (s *InputValidationTestSuite) TestAbsentInputSkipsValidation() {
	inputs := []providers.Input{
		{
			Identifier: "email",
			Validation: preparedRules([]providers.ValidationRule{
				{Type: providers.ValidationTypeRegex, Value: "^X+$",
					Message: "validation.email.invalid"},
			}),
		},
	}
	errs := validateInputValues(inputs, map[string]string{})
	s.Empty(errs)
}

func (s *InputValidationTestSuite) TestUnknownRuleTypeIgnored() {
	inputs := []providers.Input{
		{
			Identifier: "field",
			Validation: []providers.ValidationRule{
				{Type: "unknownRuleType", Value: "anything", Message: "should.not.appear"},
			},
		},
	}
	errs := validateInputValues(inputs, map[string]string{"field": "any value"})
	s.Empty(errs)
}

// An empty regex pattern has no effective constraint, so the rule passes any submitted value.
func (s *InputValidationTestSuite) TestRegexEmptyPatternPasses() {
	inputs := []providers.Input{
		{
			Identifier: "field",
			Validation: []providers.ValidationRule{
				{Type: providers.ValidationTypeRegex, Value: "", Message: "should.not.appear"},
			},
		},
	}
	errs := validateInputValues(inputs, map[string]string{"field": "anything"})
	s.Empty(errs)
}

// A non-numeric value (malformed flow definition) is treated as passing rather than failing every submission.
func (s *InputValidationTestSuite) TestMinLengthNonNumericValuePasses() {
	inputs := []providers.Input{
		{
			Identifier: "field",
			Validation: []providers.ValidationRule{
				{Type: providers.ValidationTypeMinLength, Value: "not-a-number", Message: "should.not.appear"},
			},
		},
	}
	errs := validateInputValues(inputs, map[string]string{"field": "abc"})
	s.Empty(errs)
}

func (s *InputValidationTestSuite) TestMaxLengthNonNumericValuePasses() {
	inputs := []providers.Input{
		{
			Identifier: "field",
			Validation: []providers.ValidationRule{
				{Type: providers.ValidationTypeMaxLength, Value: []int{1, 2}, Message: "should.not.appear"},
			},
		},
	}
	errs := validateInputValues(inputs, map[string]string{"field": "abcdefg"})
	s.Empty(errs)
}

func (s *InputValidationTestSuite) TestMinLengthDefaultMessageWhenMessageAbsent() {
	inputs := []providers.Input{
		{
			Identifier: "field",
			Validation: []providers.ValidationRule{
				{Type: providers.ValidationTypeMinLength, Value: float64(8)},
			},
		},
	}
	errs := validateInputValues(inputs, map[string]string{"field": "abc"})
	s.Len(errs, 1)
	s.Equal(providers.DefaultValidationMessageMinLength, errs[0].Message)
}

func (s *InputValidationTestSuite) TestMaxLengthDefaultMessageWhenMessageAbsent() {
	inputs := []providers.Input{
		{
			Identifier: "field",
			Validation: []providers.ValidationRule{
				{Type: providers.ValidationTypeMaxLength, Value: float64(3)},
			},
		},
	}
	errs := validateInputValues(inputs, map[string]string{"field": "abcdef"})
	s.Len(errs, 1)
	s.Equal(providers.DefaultValidationMessageMaxLength, errs[0].Message)
}

func (s *InputValidationTestSuite) TestNumericRuleValueAcceptsInt() {
	inputs := []providers.Input{
		{
			Identifier: "field",
			Validation: []providers.ValidationRule{
				{Type: providers.ValidationTypeMinLength, Value: 8, Message: "too short"},
			},
		},
	}
	errs := validateInputValues(inputs, map[string]string{"field": "abc"})
	s.Len(errs, 1)
	s.Equal("too short", errs[0].Message)
}

func (s *InputValidationTestSuite) TestNumericRuleValueAcceptsInt64() {
	inputs := []providers.Input{
		{
			Identifier: "field",
			Validation: []providers.ValidationRule{
				{Type: providers.ValidationTypeMaxLength, Value: int64(3), Message: "too long"},
			},
		},
	}
	errs := validateInputValues(inputs, map[string]string{"field": "abcdef"})
	s.Len(errs, 1)
	s.Equal("too long", errs[0].Message)
}

// Multi-byte UTF-8 values must be counted by rune (code point) length, not by bytes.
func (s *InputValidationTestSuite) TestMinLengthCountsRunesNotBytes() {
	inputs := []providers.Input{
		{
			Identifier: "field",
			Validation: []providers.ValidationRule{
				{Type: providers.ValidationTypeMinLength, Value: float64(5), Message: "too short"},
			},
		},
	}

	errs := validateInputValues(inputs, map[string]string{"field": "café"})
	s.Len(errs, 1, "café is 4 runes; must fail minLength: 5 even though byte length is 5")

	errs = validateInputValues(inputs, map[string]string{"field": "日本語!!"})
	s.Empty(errs, "日本語!! is 5 runes; must pass minLength: 5")
}

func (s *InputValidationTestSuite) TestMaxLengthCountsRunesNotBytes() {
	inputs := []providers.Input{
		{
			Identifier: "field",
			Validation: []providers.ValidationRule{
				{Type: providers.ValidationTypeMaxLength, Value: float64(3), Message: "too long"},
			},
		},
	}

	errs := validateInputValues(inputs, map[string]string{"field": "abcd"})
	s.Len(errs, 1, "abcd is 4 runes; must fail maxLength: 3")

	errs = validateInputValues(inputs, map[string]string{"field": "日本語"})
	s.Empty(errs, "日本語 is 3 runes; must pass maxLength: 3")

	// "𠮷𠮷" contains two non-BMP characters: each is one rune but four bytes, exercising 4-byte UTF-8.
	errs = validateInputValues(inputs, map[string]string{"field": "𠮷𠮷"})
	s.Empty(errs, "two non-BMP chars are 2 runes (8 bytes); must pass maxLength: 3")
}

// On a multi-prompt node sharing input identifiers, only the selected action's rules must fire.
func (s *InputValidationTestSuite) TestPromptNodeValidatesOnlySelectedActionInputs() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetPrompts([]common.Prompt{
		{
			// Prompt A: the "credential" field must contain '@'.
			Inputs: []providers.Input{
				{
					Identifier: "credential",
					Required:   true,
					Validation: preparedRules([]providers.ValidationRule{
						{Type: providers.ValidationTypeRegex, Value: "@",
							Message: "must.contain.at.sign"},
					}),
				},
			},
			Action: &common.Action{Ref: "submit_email", NextNode: "next"},
		},
		{
			// Prompt B: the same "credential" field must be all digits.
			Inputs: []providers.Input{
				{
					Identifier: "credential",
					Required:   true,
					Validation: preparedRules([]providers.ValidationRule{
						{Type: providers.ValidationTypeRegex, Value: "^[0-9]+$",
							Message: "must.be.digits"},
					}),
				},
			},
			Action: &common.Action{Ref: "submit_phone", NextNode: "next"},
		},
	})

	// submit_phone with an all-digit value must pass; the previous dedup behavior would
	// have applied Prompt A's '@' rule.
	ctx := &providers.NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit_phone",
		UserInputs:    map[string]string{"credential": "5551234"},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Empty(resp.FieldErrors, "phone-style value must not be validated against the email prompt's rule")
}

// The validation-failure response must include `meta` when `ctx.Verbose` is true.
func (s *InputValidationTestSuite) TestPromptNodeValidationFailureIncludesMetaWhenVerbose() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []providers.Input{
				{
					Ref:        "input_password",
					Identifier: "password",
					Required:   true,
					Validation: []providers.ValidationRule{
						{Type: providers.ValidationTypeMinLength, Value: float64(8),
							Message: "validation.password.minLength"},
					},
				},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})
	pn.SetMeta(map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{
				"id":   "input_password",
				"ref":  "input_password",
				"type": "PASSWORD_INPUT",
			},
			map[string]interface{}{
				"id":   "submit",
				"ref":  "submit",
				"type": "ACTION",
			},
		},
	})

	ctx := &providers.NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{"password": "short"},
		Verbose:       true,
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.NotEmpty(resp.FieldErrors)
	s.NotNil(resp.Meta, "Meta should be present in the response when verbose mode is enabled")
}

func (s *InputValidationTestSuite) TestInputWithoutValidationRulesNoOp() {
	inputs := []providers.Input{
		{Identifier: "any", Required: true},
	}
	errs := validateInputValues(inputs, map[string]string{"any": "value"})
	s.Empty(errs)
}

func (s *InputValidationTestSuite) TestPromptNodeReturnsFieldErrorsOnFailure() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []providers.Input{
				{
					Identifier: "password",
					Required:   true,
					Validation: []providers.ValidationRule{
						{Type: providers.ValidationTypeMinLength, Value: float64(8),
							Message: "validation.password.minLength"},
					},
				},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	ctx := &providers.NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{"password": "short"},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Equal(common.NodeResponseTypeView, resp.Type)
	s.Len(resp.FieldErrors, 1)
	s.Equal("password", resp.FieldErrors[0].Identifier)
	s.Equal("validation.password.minLength", resp.FieldErrors[0].Message)
	_, stillPresent := ctx.UserInputs["password"]
	s.False(stillPresent, "failed input should be cleared from UserInputs")
}

// A validation failure must re-prompt the same field set the user saw in the initial
// prompt (preserving form structure) and must include the selected action so the form
// can be re-submitted.
func (s *InputValidationTestSuite) TestPromptNodeRePromptsInitialFieldSetOnValidationFailure() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []providers.Input{
				{Identifier: "username", Required: true},
				{
					Identifier: "password",
					Required:   true,
					Validation: []providers.ValidationRule{
						{Type: providers.ValidationTypeMinLength, Value: float64(8),
							Message: "validation.password.minLength"},
					},
				},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	ctx := &providers.NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{"username": "valid_user", "password": "short"},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Equal(common.NodeResponseTypeView, resp.Type)

	s.Len(resp.FieldErrors, 1)
	s.Equal("password", resp.FieldErrors[0].Identifier)

	s.Len(resp.Inputs, 2, "the entire initial-prompt set must be re-prompted to preserve form structure")
	identifiers := map[string]bool{}
	for _, in := range resp.Inputs {
		identifiers[in.Identifier] = true
	}
	s.True(identifiers["username"], "username (in the initial prompt) must remain in the re-prompted inputs")
	s.True(identifiers["password"], "password (in the initial prompt) must remain in the re-prompted inputs")

	s.Len(resp.Actions, 1)
	s.Equal("submit", resp.Actions[0].Ref)

	_, passwordPresent := ctx.UserInputs["password"]
	s.False(passwordPresent, "failed input cleared from UserInputs")
	s.Equal("valid_user", ctx.UserInputs["username"], "passing input retained in UserInputs")
}

func (s *InputValidationTestSuite) TestPromptNodeAdvancesOnValidSubmission() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []providers.Input{
				{
					Identifier: "password",
					Required:   true,
					Validation: []providers.ValidationRule{
						{Type: providers.ValidationTypeMinLength, Value: float64(8),
							Message: "validation.password.minLength"},
					},
				},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	ctx := &providers.NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{"password": "longenough"},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusComplete, resp.Status)
	s.Empty(resp.FieldErrors)
	s.Equal("next", resp.NextNodeID)
}

// The LOGIN_OPTIONS variant must enforce validation rules and must not advance the flow on invalid inputs.
func (s *InputValidationTestSuite) TestLoginOptionsVariantReturnsFieldErrorsOnValidationFailure() {
	node := newPromptNode("login-chooser", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetVariant(providers.NodeVariantLoginOptions)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []providers.Input{
				{
					Identifier: "password",
					Required:   true,
					Validation: []providers.ValidationRule{
						{Type: providers.ValidationTypeMinLength, Value: float64(8),
							Message: "validation.password.minLength"},
					},
				},
			},
			Action: &common.Action{Ref: "pwd", NextNode: "pwd-node"},
		},
	})

	ctx := &providers.NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "pwd",
		UserInputs:    map[string]string{"password": "short"},
		RuntimeData:   map[string]string{},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status, "invalid input must not advance the LOGIN_OPTIONS flow")
	s.Len(resp.FieldErrors, 1)
	s.Equal("password", resp.FieldErrors[0].Identifier)
	s.Empty(resp.NextNodeID, "NextNodeID must be empty on validation failure")
}

func (s *InputValidationTestSuite) TestPromptNodeValidationDoesNotRunWhenRequiredInputMissing() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []providers.Input{
				{
					Identifier: "password",
					Required:   true,
					Validation: []providers.ValidationRule{
						{Type: providers.ValidationTypeMinLength, Value: float64(8),
							Message: "validation.password.minLength"},
					},
				},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})

	ctx := &providers.NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{},
	}
	resp, err := node.Execute(ctx)

	s.Nil(err)
	s.NotNil(resp)
	s.Equal(common.NodeStatusIncomplete, resp.Status)
	s.Empty(resp.FieldErrors)
	s.Len(resp.Inputs, 1)
}

// applyValidationFailureRePrompt direct unit tests below.

// newPromptNodeForReprompt builds a single-prompt node with one minLength rule
// on `password`, used by the focused tests of applyValidationFailureRePrompt.
func (s *InputValidationTestSuite) newPromptNodeForReprompt() (*promptNode, *common.NodeResponse) {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []providers.Input{
				{Identifier: "username", Required: true},
				{
					Identifier: "password",
					Required:   true,
					Validation: []providers.ValidationRule{
						{Type: providers.ValidationTypeMinLength, Value: float64(8),
							Message: "validation.password.minLength"},
					},
				},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})
	return node.(*promptNode), &common.NodeResponse{
		Inputs:  make([]providers.Input, 0),
		Actions: make([]common.Action, 0),
	}
}

// Returns false and leaves nodeResp untouched when every rule passes.
func (s *InputValidationTestSuite) TestApplyValidationFailureRePrompt_ReturnsFalseWhenAllRulesPass() {
	n, nodeResp := s.newPromptNodeForReprompt()
	ctx := &providers.NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{"username": "alice", "password": "longenough"},
	}

	handled := n.applyValidationFailureRePrompt(ctx, nodeResp)

	s.False(handled)
	s.Empty(nodeResp.FieldErrors)
	s.Empty(nodeResp.Inputs)
	s.Empty(nodeResp.Actions)
	s.Equal("submit", ctx.CurrentAction, "passing validation must not clear the selected action")
}

// Returns true, populates FieldErrors, clears the failing input value, resets
// CurrentAction, and re-prompts the initial field set (preserving form structure).
func (s *InputValidationTestSuite) TestApplyValidationFailureRePrompt_HandlesFailureAndRePromptsInitialFieldSet() {
	n, nodeResp := s.newPromptNodeForReprompt()
	ctx := &providers.NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{"username": "alice", "password": "short"},
	}

	handled := n.applyValidationFailureRePrompt(ctx, nodeResp)

	s.True(handled)
	s.Len(nodeResp.FieldErrors, 1)
	s.Equal("password", nodeResp.FieldErrors[0].Identifier)
	s.Equal("validation.password.minLength", nodeResp.FieldErrors[0].Message)
	s.Equal(common.NodeStatusIncomplete, nodeResp.Status)
	s.Equal(common.NodeResponseTypeView, nodeResp.Type)
	s.Len(nodeResp.Inputs, 2, "re-prompt must return the same field set the user saw initially")
	s.Len(nodeResp.Actions, 1, "re-prompt must return the action so the form has a submit button")
	s.Equal("", ctx.CurrentAction, "failing validation must reset CurrentAction")
	_, passwordPresent := ctx.UserInputs["password"]
	s.False(passwordPresent, "failing input value must be cleared from UserInputs")
	s.Equal("alice", ctx.UserInputs["username"], "passing input value must be retained in UserInputs")
}

// Inputs pre-satisfied from RuntimeData must not be re-prompted on validation failure.
func (s *InputValidationTestSuite) TestApplyValidationFailureRePrompt_PreservesStrippedDownInitialSet() {
	node := newPromptNode("prompt-1", map[string]interface{}{}, false, false)
	pn := node.(PromptNodeInterface)
	pn.SetPrompts([]common.Prompt{
		{
			Inputs: []providers.Input{
				{Identifier: "tenant", Required: true},
				{
					Identifier: "password",
					Required:   true,
					Validation: []providers.ValidationRule{
						{Type: providers.ValidationTypeMinLength, Value: float64(8),
							Message: "validation.password.minLength"},
					},
				},
			},
			Action: &common.Action{Ref: "submit", NextNode: "next"},
		},
	})
	pn2 := node.(*promptNode)
	nodeResp := &common.NodeResponse{
		Inputs:  make([]providers.Input, 0),
		Actions: make([]common.Action, 0),
	}
	ctx := &providers.NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{"password": "short"},
		RuntimeData:   map[string]string{"tenant": "acme"}, // tenant pre-satisfied
	}

	handled := pn2.applyValidationFailureRePrompt(ctx, nodeResp)

	s.True(handled)
	s.Len(nodeResp.Inputs, 1, "tenant came from RuntimeData and must NOT be re-prompted")
	s.Equal("password", nodeResp.Inputs[0].Identifier,
		"only the failing password (initially prompted) must be re-prompted")
	s.Equal("acme", ctx.RuntimeData["tenant"], "passing identifiers in RuntimeData must remain untouched")
}

// Failing values stored in RuntimeData and ForwardedData must also be cleared so the
// engine does not fall back to them on the next iteration. collectMissingInputs reads
// all three sources; missing any would mask the validation failure.
func (s *InputValidationTestSuite) TestApplyValidationFailureRePrompt_ClearsRuntimeDataAndForwardedData() {
	n, nodeResp := s.newPromptNodeForReprompt()
	ctx := &providers.NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{"username": "alice", "password": "short"},
		RuntimeData:   map[string]string{"password": "stale-from-runtime"},
		ForwardedData: map[string]interface{}{"password": "stale-from-forwarded"},
	}

	handled := n.applyValidationFailureRePrompt(ctx, nodeResp)

	s.True(handled)
	_, inUserInputs := ctx.UserInputs["password"]
	_, inRuntime := ctx.RuntimeData["password"]
	_, inForwarded := ctx.ForwardedData["password"]
	s.False(inUserInputs, "failing identifier must be cleared from UserInputs")
	s.False(inRuntime, "failing identifier must be cleared from RuntimeData")
	s.False(inForwarded, "failing identifier must be cleared from ForwardedData")
}

// Populates nodeResp.Meta when ctx.Verbose is true and the node has meta set.
func (s *InputValidationTestSuite) TestApplyValidationFailureRePrompt_PopulatesMetaWhenVerbose() {
	n, nodeResp := s.newPromptNodeForReprompt()
	n.SetMeta(map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{"id": "input_password", "ref": "input_password", "type": "PASSWORD_INPUT"},
			map[string]interface{}{"id": "submit", "ref": "submit", "type": "ACTION"},
		},
	})
	ctx := &providers.NodeContext{
		ExecutionID:   "test-flow",
		CurrentAction: "submit",
		UserInputs:    map[string]string{"username": "alice", "password": "short"},
		Verbose:       true,
	}

	handled := n.applyValidationFailureRePrompt(ctx, nodeResp)

	s.True(handled)
	s.NotNil(nodeResp.Meta, "verbose mode must populate Meta on the re-prompt response")
}
