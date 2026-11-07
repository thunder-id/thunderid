// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package projectscope

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/internal/system/security"
)

func TestProjectRequired(t *testing.T) {
	for resourceType, expected := range map[security.ResourceType]bool{
		security.ResourceTypeApplication: false,
		security.ResourceTypeAgent:       false,
		security.ResourceTypeGroup:       false,
	} {
		assert.Equal(t, expected, ProjectRequired(resourceType), resourceType)
	}
}
