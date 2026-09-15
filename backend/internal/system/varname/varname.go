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

// Package varname derives the template variable names that a declarative resource's placeholders
// carry. It is a leaf so that both the exporter and whatever captures a credential can depend on it
// and derive the same name.
package varname

import "strings"

// DeriveVariableName derives the declarative template variable name for a resource field, and is the
// single source of truth for those names so that every caller derives the same one.
//
// The resource type leads the name because a name is only unique within its type: an application and
// an agent both called "dummy" would otherwise share DUMMY_CLIENT_ID. The result is sanitized, since
// a resource name routinely holds characters a template variable cannot carry.
//
// Example: ("application", "My App", "ClientSecret") -> "APPLICATION_MY_APP_CLIENT_SECRET".
// Example: ("user", "user@example.com", "password") -> "USER_USER_EXAMPLE_COM_PASSWORD".
func DeriveVariableName(resourceType, resourceName, fieldName string) string {
	prefix := toSnakeCaseUpper(strings.ReplaceAll(resourceName, " ", "_"))
	if resourceType != "" {
		prefix = toSnakeCaseUpper(strings.ReplaceAll(resourceType, " ", "_")) + "_" + prefix
	}
	return sanitizeVariableName(prefix + "_" + toSnakeCaseUpper(fieldName))
}

// sanitizeVariableName replaces every character a Go template identifier cannot carry with an
// underscore, collapses runs of underscores, and trims them from the ends.
func sanitizeVariableName(name string) string {
	var result strings.Builder
	lastWasUnderscore := false

	for _, r := range name {
		isAllowed := (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
		if !isAllowed {
			r = '_'
		}
		if r == '_' {
			if lastWasUnderscore {
				continue
			}
			lastWasUnderscore = true
		} else {
			lastWasUnderscore = false
		}
		result.WriteRune(r)
	}

	sanitized := strings.Trim(result.String(), "_")
	// A name must not start with a digit, so a leading digit is prefixed with an underscore. The
	// prefix matches the exporter's own sanitizer, because a placeholder and the variable captured for
	// it have to spell the same name.
	if sanitized != "" && sanitized[0] >= '0' && sanitized[0] <= '9' {
		return "_" + sanitized
	}
	return sanitized
}

// toSnakeCaseUpper converts camelCase/PascalCase to UPPER_SNAKE_CASE (e.g. "ClientID" -> "CLIENT_ID").
func toSnakeCaseUpper(s string) string {
	var result strings.Builder
	runes := []rune(s)

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Add an underscore before an uppercase letter when the previous character is lowercase, or
		// when the previous character is uppercase and the next is lowercase (handles acronym
		// boundaries such as "ClientID" -> "CLIENT_ID").
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := runes[i-1]
			if prev >= 'a' && prev <= 'z' {
				result.WriteRune('_')
			} else if prev != '_' && i+1 < len(runes) {
				next := runes[i+1]
				if next >= 'a' && next <= 'z' {
					result.WriteRune('_')
				}
			}
		}

		result.WriteRune(r)
	}

	return strings.ToUpper(result.String())
}
