// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// scopeValueSeparator joins the parts of a scope criterion before they are digested.
const scopeValueSeparator = "|"

// EntityScopeCriterionValue returns the CriterionTypeEntityScope value denying one scope for one
// principal on one resource server.
//
// The audience is part of the identity because a permission string is unique only within its resource
// server: two servers can each define "license" and they are different scopes. Both halves are trusted
// token claims (sub and aud), so an enforcement point can derive this from a token alone.
func EntityScopeCriterionValue(entityID, audience, scope string) string {
	return digest(entityID, audience, scope)
}

// ScopeCriterionValue returns the CriterionTypeScope value denying one scope on one resource server
// for every principal, used when the scope itself stops existing.
func ScopeCriterionValue(audience, scope string) string {
	return digest(audience, scope)
}

// digest returns the SHA-256 of the parts as lower-case hex. The parts are hashed because they do not
// fit CRITERION_VALUE (VARCHAR(255)), and nothing is lost since every match is an equality test.
//
// Each part is length-prefixed rather than merely joined: a resource server identifier has no character
// restriction, so ("a|b", "c") and ("a", "b|c") would otherwise render alike and one revocation would
// match a scope it was never written for.
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
