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
// carry. It depends only on the utils package, so that both the exporter and whatever captures a credential
// can depend on it and derive the same name.
package varname

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/utils"
)

const (
	// segmentSeparator joins the parts of a name. The sanitizer collapses every run of underscores,
	// so no part can contain it, which is what makes a name read back as exactly one resource type,
	// ID and field.
	segmentSeparator = "__"
	// hashedIDPrefix marks an ID segment made from a hash. A hex ID never contains this letter, so a
	// hashed segment cannot equal one written from an ID.
	hashedIDPrefix = "H"
	// hashedIDBytes is how many bytes of the hash a hashed ID segment keeps.
	hashedIDBytes = 16
)

// DeriveVariableName derives the declarative template variable name for a resource field, and is the
// single source of truth for those names so that every caller derives the same one.
//
// A name is "<TYPE>__<ID>__<FIELD>". The ID is what makes it belong to one resource: names and
// usernames are not unique once case, dots, hyphens and similar characters are folded away, so
// "alice@example.com" and "alice.example.com" would read the same, but their IDs never do. The name of
// the resource is left out for that reason: it can change and it can repeat, and the ID cannot.
//
// The ID is written in full. A shortened one could be matched on purpose by whoever chooses the ID
// of a resource they import.
//
// Example: ("user", "0198f3a1-7c2e-7b10-9a44-5d3e8f21c6b7", "password") ->
// "USER__0198F3A17C2E7B109A445D3E8F21C6B7__PASSWORD".
func DeriveVariableName(resourceType, resourceID, fieldName string) string {
	segments := make([]string, 0, 3)
	if resourceType != "" {
		segments = append(segments, sanitizeSegment(toSnakeCaseUpper(strings.ReplaceAll(resourceType, " ", "_"))))
	}
	segments = append(segments, idSegment(resourceID), sanitizeSegment(toSnakeCaseUpper(fieldName)))

	name := strings.Join(segments, segmentSeparator)
	// A name must not start with a digit.
	if name[0] >= '0' && name[0] <= '9' {
		return "_" + name
	}
	return name
}

// idSegment spells a resource ID so that two IDs never share a spelling. A lower-case UUID, which
// is what the server generates, is written as its hex digits in upper case. Any other ID, such as one
// a declarative file names, is hashed, because upper-casing and sanitizing would let "my-app" and
// "my_app" read the same.
func idSegment(resourceID string) string {
	if utils.IsValidUUID(resourceID) && resourceID == strings.ToLower(resourceID) {
		return strings.ToUpper(strings.ReplaceAll(resourceID, "-", ""))
	}
	sum := sha256.Sum256([]byte(resourceID))
	return hashedIDPrefix + strings.ToUpper(hex.EncodeToString(sum[:hashedIDBytes]))
}

// sanitizeSegment replaces every character a Go template identifier cannot carry with an
// underscore, collapses runs of underscores, and trims them from the ends.
func sanitizeSegment(name string) string {
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

	return strings.Trim(result.String(), "_")
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
