// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"math"
	"testing"
	"time"

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

// A timeout that is not positive falls back to the one default.json ships rather than reaching the
// HTTP server, which would read it as no limit and drop the guard against slow clients.
func TestServerTimeoutsFallBackWhenNotPositive(t *testing.T) {
	for name, timeouts := range map[string]ServerTimeouts{
		"unset":     {},
		"negative":  {ReadHeaderSeconds: -1, WriteSeconds: -5, IdleSeconds: -60},
		"too large": {ReadHeaderSeconds: math.MaxInt, WriteSeconds: 9223372037, IdleSeconds: math.MaxInt},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, 10*time.Second, timeouts.ReadHeader())
			assert.Equal(t, 10*time.Second, timeouts.Write())
			assert.Equal(t, 120*time.Second, timeouts.Idle())
		})
	}

	configured := ServerTimeouts{ReadHeaderSeconds: 7, WriteSeconds: 180, IdleSeconds: 90}
	assert.Equal(t, 7*time.Second, configured.ReadHeader())
	assert.Equal(t, 180*time.Second, configured.Write())
	assert.Equal(t, 90*time.Second, configured.Idle())
}

// ID-JAG issuance and acceptance stay on unless a deployment switches them off, so an engine embedder
// that sets no ID-JAG configuration keeps the per-client and per-connection behavior it had.
func TestIDJAGRolesAreOnUnlessSwitchedOff(t *testing.T) {
	var unset IDJAGConfig
	assert.True(t, unset.Issuance.IsEnabled())
	assert.True(t, unset.Acceptance.IsEnabled())

	assert.True(t, IDJAGRoleConfig{Enabled: new(true)}.IsEnabled())
	assert.False(t, IDJAGRoleConfig{Enabled: new(false)}.IsEnabled())
}
