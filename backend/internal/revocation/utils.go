// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

const scopeValueSeparator = "|"

// EntityScopeCriterionValue returns the value denying one scope for one principal on one resource server.
func EntityScopeCriterionValue(entityID, audience, scope string) string {
	return digest(entityID, audience, scope)
}

// ScopeCriterionValue returns the value denying one scope on one resource server for every principal.
func ScopeCriterionValue(audience, scope string) string {
	return digest(audience, scope)
}

// digest returns the hex SHA-256 of the length-prefixed parts, so differently split parts never collide.
func digest(parts ...string) string {
	var rendered strings.Builder
	for _, part := range parts {
		rendered.WriteString(strconv.Itoa(len(part)))
		rendered.WriteString(scopeValueSeparator)
		rendered.WriteString(part)
	}
	sum := sha256.Sum256([]byte(rendered.String()))
	return hex.EncodeToString(sum[:])
}
