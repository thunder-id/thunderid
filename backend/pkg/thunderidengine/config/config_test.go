// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The organization-unit-qualified endpoints are off unless a deployment says otherwise. They are
// no safer than the actor provider's answer about which units a client may act for, so an engine
// embedder that has not implemented that answer must not end up serving them by doing nothing at
// all.
func TestOUQualifiedEndpointsAreOffByDefault(t *testing.T) {
	var cfg ServerConfig

	assert.False(t, cfg.EnableOUQualifiedEndpoints)
}
