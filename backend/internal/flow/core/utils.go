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
	"strings"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/log"
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
			if ctx.AuthenticatedUser.UserID != "" {
				return ctx.AuthenticatedUser.UserID
			}
			if runtimeValue, ok := ctx.RuntimeData["userId"]; ok && runtimeValue != "" {
				return runtimeValue
			}
			return match // Keep placeholder if not found
		}

		// Special handling for ouId - only resolve from runtime data or authenticated user
		if key == "ouId" {
			if ctx.AuthenticatedUser.OUID != "" {
				return ctx.AuthenticatedUser.OUID
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

// ParsePresentedOptionalInputIdentifiers converts a space-separated identifier list into a set.
func ParsePresentedOptionalInputIdentifiers(raw string) map[string]struct{} {
	result := make(map[string]struct{})
	if raw == "" {
		return result
	}

	for _, identifier := range strings.Fields(raw) {
		if identifier != "" {
			result[identifier] = struct{}{}
		}
	}

	return result
}

// GetPresentedOptionalInputs extracts and parses the presented optional input identifiers from
// runtime data into a set. Call this once before a loop to avoid repeated string parsing.
func GetPresentedOptionalInputs(runtimeData map[string]string) map[string]struct{} {
	return ParsePresentedOptionalInputIdentifiers(runtimeData[common.RuntimeKeyPresentedOptionalInputs])
}

// hasPresentedOptionalInput returns true when the given identifier appears in the presented-input set.
func hasPresentedOptionalInput(presentedInputs map[string]struct{}, identifier string) bool {
	if identifier == "" || len(presentedInputs) == 0 {
		return false
	}

	_, ok := presentedInputs[identifier]
	return ok
}

// IsOptionalInputPrompted returns true when an optional input identifier has already been shown
// to the user in a prior prompt step.
func IsOptionalInputPrompted(presentedOptionalInputs map[string]struct{}, identifier string) bool {
	return hasPresentedOptionalInput(presentedOptionalInputs, identifier)
}

// collectMissingInputs returns inputs from requiredInputs that are not satisfied by user inputs,
// runtime data, forwarded data, or already-presented optional inputs.
func collectMissingInputs(ctx *NodeContext, presentedOptionalInputs map[string]struct{},
	requiredInputs []common.Input, logger *log.Logger) []common.Input {
	missing := make([]common.Input, 0, len(requiredInputs))
	for _, input := range requiredInputs {
		if _, ok := ctx.UserInputs[input.Identifier]; ok {
			continue
		}
		if _, ok := ctx.RuntimeData[input.Identifier]; ok {
			logger.Debug(ctx.Context, "Input available in runtime data, skipping",
				log.String("identifier", input.Identifier), log.Bool("isRequired", input.Required))
			continue
		}
		if value, ok := ctx.ForwardedData[input.Identifier]; ok {
			if _, isString := value.(string); isString {
				logger.Debug(ctx.Context, "Input available in forwarded data, skipping",
					log.String("identifier", input.Identifier), log.Bool("isRequired", input.Required))
				continue
			}
		}
		if !input.Required && IsOptionalInputPrompted(presentedOptionalInputs, input.Identifier) {
			logger.Debug(ctx.Context, "Optional input already prompted, skipping",
				log.String("identifier", input.Identifier))
			continue
		}
		logger.Debug(ctx.Context, "Input not available in the context",
			log.String("identifier", input.Identifier), log.Bool("isRequired", input.Required))
		missing = append(missing, input)
	}
	return missing
}

// MergePresentedOptionalInputIdentifiers appends identifiers to an existing serialized identifier string.
// Duplicates are acceptable since ParsePresentedOptionalInputIdentifiers deduplicates on read.
func MergePresentedOptionalInputIdentifiers(raw string, identifiers []string) string {
	parts := make([]string, 0, len(identifiers)+1)
	if raw != "" {
		parts = append(parts, raw)
	}
	for _, identifier := range identifiers {
		if identifier != "" {
			parts = append(parts, identifier)
		}
	}
	return strings.Join(parts, " ")
}
