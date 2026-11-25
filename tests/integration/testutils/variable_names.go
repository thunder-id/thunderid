// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// hashedIDBytes is how many bytes of its hash an ID that is not a UUID keeps in a variable name.
const hashedIDBytes = 16

// canonicalUUID matches a UUID written in lower case with hyphens, which is what the server generates.
var canonicalUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// ExportVariableName is how an export names a value of a resource: its type, its ID and the field, each
// in upper case and joined by double underscores. A UUID is written without hyphens, and any other ID,
// such as one a document names, is hashed and written as "H" and the hash.
func ExportVariableName(resourceType, id, field string) string {
	return resourceType + "__" + variableNameIDSegment(id) + "__" + field
}

// variableNameIDSegment spells a resource ID as an export writes it in a variable name.
func variableNameIDSegment(id string) string {
	if canonicalUUID.MatchString(id) {
		return strings.ToUpper(strings.ReplaceAll(id, "-", ""))
	}
	sum := sha256.Sum256([]byte(id))
	return "H" + strings.ToUpper(hex.EncodeToString(sum[:hashedIDBytes]))
}
