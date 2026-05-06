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
	"regexp"
)

// placeholderPattern matches {{ context.key }} with optional whitespace.
// TODO: Extend to support {{ user.key }}, {{ env.key }}, etc.
var placeholderPattern = regexp.MustCompile(`{{\s*context\.\s*(\w+)\s*}}`)

// ResolvePlaceholder resolves a single placeholder string using the "{{ context.key }}" syntax.
// If no placeholder is found, the original value is returned.
// If a placeholder is found but the key doesn't exist in any data source, the placeholder is kept as-is.
func ResolvePlaceholder(ctx *NodeContext, value string) string {
	if ctx == nil {
		return value
	}

	return placeholderPattern.ReplaceAllStringFunc(value, func(match string) string {
		submatches := placeholderPattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		key := submatches[1]

		// Special handling for userId - only resolve from runtime data or authenticated user
		if key == "userId" {
			if ctx.AuthUser.GetUserID() != "" {
				return ctx.AuthUser.GetUserID()
			}
			if runtimeValue, ok := ctx.RuntimeData["userId"]; ok && runtimeValue != "" {
				return runtimeValue
			}
			return match // Keep placeholder if not found
		}

		// Special handling for ouId - only resolve from runtime data or authenticated user
		if key == "ouId" {
			if ctx.AuthUser.GetOUID() != "" {
				return ctx.AuthUser.GetOUID()
			}
			if runtimeValue, ok := ctx.RuntimeData["ouId"]; ok && runtimeValue != "" {
				return runtimeValue
			}
			return match // Keep placeholder if not found
		}

		// Check runtime data first
		if runtimeValue, ok := ctx.RuntimeData[key]; ok && runtimeValue != "" {
			return runtimeValue
		}

		// Check user inputs next
		if userInputValue, ok := ctx.UserInputs[key]; ok && userInputValue != "" {
			return userInputValue
		}

		// If not found, keep the placeholder as-is
		return match
	})
}
