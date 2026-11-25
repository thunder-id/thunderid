// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package plane_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/tools/cli/internal/plane"
)

func TestParse_AcceptsEachPlane(t *testing.T) {
	for value, want := range map[string]plane.Plane{
		"quickstart": plane.Quickstart,
		"dp":         plane.Data,
		"DP":         plane.Data,
	} {
		got, err := plane.Parse(value)
		require.NoError(t, err, value)
		assert.Equal(t, want, got, value)
	}
}

// The Control Plane ships inside the quickstart only, so it is not a plane of its own.
func TestParse_RejectsAnythingElse(t *testing.T) {
	for _, value := range []string{"", "cp", "data-plane", "all"} {
		_, err := plane.Parse(value)
		require.Error(t, err, value)
		assert.Contains(t, err.Error(), "quickstart or dp")
	}
}
