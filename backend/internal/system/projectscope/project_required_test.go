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
		security.ResourceTypeApplication:             true,
		security.ResourceTypeAuthorizationDetailType: true,
		security.ResourceTypeAgent:                   true,
		security.ResourceTypeDesign:                  true,
		security.ResourceTypeFlow:                    true,
		security.ResourceTypeGroup:                   false,
		security.ResourceTypeNotificationTemplate:    true,
		security.ResourceTypeOU:                      true,
		security.ResourceTypeVerifiableCredential:    true,
	} {
		assert.Equal(t, expected, ProjectRequired(resourceType), resourceType)
	}
}
