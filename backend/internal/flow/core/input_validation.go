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
			if validateInput(rule, value) {
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

// validateInput returns false when the value violates the rule. Unknown rule
// types and regex rules without a CompiledRegex pass through.
func validateInput(rule providers.ValidationRule, value string) bool {
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
