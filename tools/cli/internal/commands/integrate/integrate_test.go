// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package integrate_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/tools/cli/internal/commands/integrate"
)

func TestPlatforms_CoversExpectedKeysAndSlugs(t *testing.T) {
	want := map[string]string{
		"react":      "react",
		"nextjs":     "nextjs",
		"express":    "express",
		"vue":        "vue",
		"nuxt":       "nuxt",
		"nodejs":     "node",
		"javascript": "browser",
		"ios":        "ios",
		"android":    "android",
		"flutter":    "flutter",
	}

	got := map[string]string{}
	for _, p := range integrate.Platforms {
		got[p.Key] = p.Slug
		assert.NotEmpty(t, p.Label, "platform %q must have a display label", p.Key)
	}
	assert.Equal(t, want, got)
}
