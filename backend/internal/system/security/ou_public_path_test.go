// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The organization-unit-scoped token endpoint is the only thing mounted under /ou/, and the
// allow-list names it exactly rather than globbing the prefix.
//
// The point of the test is the negative half: an endpoint added under /ou/ later must not become
// public by inheriting a glob written before it existed. It has to be listed deliberately.
func TestOUScopedTokenEndpointIsTheOnlyPublicOUPath(t *testing.T) {
	compiled, err := compilePathPatterns(publicPaths)
	require.NoError(t, err)

	isPublic := func(path string) bool {
		for _, re := range compiled {
			if re.MatchString(path) {
				return true
			}
		}
		return false
	}

	tests := []struct {
		path string
		want bool
		why  string
	}{
		{"/ou/01900000-0000-7000-8000-000000000001/oauth2/token", true,
			"the organization-unit-scoped token endpoint is the route this entry exists for"},
		{"/ou/decl-m2m-root/oauth2/token", true, "an organization unit handle is one segment too"},

		{"/ou/some-ou/oauth2/introspect", false, "introspection is not mounted under /ou/"},
		{"/ou/some-ou/oauth2/revoke", false, "revocation is not mounted under /ou/"},
		{"/ou/some-ou/oauth2/token/extra", false, "the pattern matches one exact path, not a subtree"},
		{"/ou/a/b/oauth2/token", false, "the organization unit is a single path segment"},
		{"/ou/some-ou/users", false, "a future organization-unit-scoped API must opt in deliberately"},
		{"/ou//oauth2/token", false, "an empty organization unit segment matches nothing"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, isPublic(tt.path), tt.why)
		})
	}
}
