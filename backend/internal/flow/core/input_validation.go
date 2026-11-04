// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"unicode/utf8"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// validateInputValues returns a FieldError per failing rule across the given inputs.
func validateInputValues(inputs []providers.Input, userInputs map[string]string) []common.FieldError {
	var fieldErrors []common.FieldError

	for _, input := range inputs {
		if len(input.Validation) == 0 {
			continue
		}
		value, ok := userInputs[input.Identifier]
		if !ok {
			continue
		}

		for _, rule := range input.Validation {
			if SatisfiesValidationRule(rule, value) {
				continue
			}
			fieldErrors = append(fieldErrors, common.FieldError{
				Identifier: input.Identifier,
				Message:    resolveRuleMessage(rule),
			})
		}
	}

	return fieldErrors
}

// SatisfiesValidationRule returns false when the value violates the rule. Unknown rule
// types and regex rules without a CompiledRegex pass through.
func SatisfiesValidationRule(rule providers.ValidationRule, value string) bool {
	switch rule.Type {
	case providers.ValidationTypeRegex:
		if rule.CompiledRegex == nil {
			return true
		}
		return rule.CompiledRegex.MatchString(value)

	case providers.ValidationTypeMinLength:
		minLen, ok := utils.ToFloat64(rule.Value)
		if !ok {
			return true
		}
		return utf8.RuneCountInString(value) >= int(minLen)

	case providers.ValidationTypeMaxLength:
		maxLen, ok := utils.ToFloat64(rule.Value)
		if !ok {
			return true
		}
		return utf8.RuneCountInString(value) <= int(maxLen)
	}
	return true
}

// resolveRuleMessage returns the rule's Message or the default key for its type.
func resolveRuleMessage(rule providers.ValidationRule) string {
	if rule.Message != "" {
		return rule.Message
	}
	switch rule.Type {
	case providers.ValidationTypeMinLength:
		return providers.DefaultValidationMessageMinLength
	case providers.ValidationTypeMaxLength:
		return providers.DefaultValidationMessageMaxLength
	default:
		return providers.DefaultValidationMessageRegex
	}
}
