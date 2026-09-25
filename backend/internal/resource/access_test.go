// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The filter is what stops a withheld branch showing up in a sharee's listings, so its containment
// has to follow the server's own delimiter rather than raw string prefixing.
func TestPermissionFilterPermits(t *testing.T) {
	tests := []struct {
		name   string
		filter permissionFilter
		perm   string
		want   bool
	}{
		{"unrestricted admits anything", permissionFilter{unrestricted: true}, "anything", true},
		{
			"no allowed set admits anything not excluded",
			permissionFilter{delimiter: ":"}, "bookings", true,
		},
		{
			"an allowed branch admits itself",
			permissionFilter{allowed: []string{"bookings"}, delimiter: ":"}, "bookings", true,
		},
		{
			"an allowed branch admits what lies beneath it",
			permissionFilter{allowed: []string{"bookings"}, delimiter: ":"}, "bookings:create", true,
		},
		{
			"an allowed branch does not admit a sibling sharing its characters",
			permissionFilter{allowed: []string{"bookings"}, delimiter: ":"}, "bookingsx", false,
		},
		{
			"an unrelated branch is refused",
			permissionFilter{allowed: []string{"bookings"}, delimiter: ":"}, "billing", false,
		},
		{
			"an exclusion removes the branch",
			permissionFilter{excluded: []string{"bookings:refund"}, delimiter: ":"},
			"bookings:refund", false,
		},
		{
			"an exclusion removes what lies beneath it",
			permissionFilter{excluded: []string{"bookings:refund"}, delimiter: ":"},
			"bookings:refund:approve", false,
		},
		{
			"an exclusion leaves the rest of the branch",
			permissionFilter{excluded: []string{"bookings:refund"}, delimiter: ":"},
			"bookings:create", true,
		},
		{
			"an exclusion beats an allowance",
			permissionFilter{
				allowed:   []string{"bookings"},
				excluded:  []string{"bookings:refund"},
				delimiter: ":",
			},
			"bookings:refund", false,
		},
		{
			"a server using another separator is not matched on the default one",
			permissionFilter{allowed: []string{"bookings"}, delimiter: "/"}, "bookings:create", false,
		},
		{
			"a server using another separator matches on its own",
			permissionFilter{allowed: []string{"bookings"}, delimiter: "/"}, "bookings/create", true,
		},
		{
			"an explicitly empty allowed set permits nothing",
			permissionFilter{allowed: []string{}, delimiter: ":"}, "bookings", false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.filter.permits(tt.perm))
		})
	}
}

// covers underpins both the filter and the sharing rule, so its boundary is pinned directly.
func TestCovers(t *testing.T) {
	assert.True(t, covers("a", "a", ":"), "a path denotes itself")
	assert.True(t, covers("a", "a:b", ":"))
	assert.False(t, covers("a", "ab", ":"), "a sibling sharing characters is not beneath it")
	assert.False(t, covers("a", "a:b", ""), "with no separator only equality holds")
	assert.False(t, covers("a:b", "a", ":"), "containment does not run upward")
}
