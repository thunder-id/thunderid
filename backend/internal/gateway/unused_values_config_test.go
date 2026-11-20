// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// configureUnusedValues puts the grace and the sweep interval in the runtime configuration, as loading
// a config file would, and restores what was there after the test.
func configureUnusedValues(t *testing.T, grace, interval *int) {
	t.Helper()
	config.ResetServerRuntime()
	cfg := &config.Config{}
	cfg.Gateway.UnusedValueGraceSeconds = grace
	cfg.Gateway.UnusedValueSweepIntervalSeconds = interval
	require.NoError(t, config.InitializeServerRuntime("", cfg))
	t.Cleanup(func() {
		config.ResetServerRuntime()
		configureGatewayBound(shippedGatewayBound)
	})
}

// The grace and the interval are read from the runtime configuration. A grace of 0 is raised to the
// one-second minimum, a negative one turns the removal off, and without a runtime nothing is removed.
func TestUnusedValueSettingsAreReadFromTheRuntime(t *testing.T) {
	zero, negative, hour, minute := 0, -1, 3600, 60

	configureUnusedValues(t, &hour, &minute)
	grace, on := unusedValueGrace(context.Background())
	assert.True(t, on)
	assert.Equal(t, time.Hour, grace)
	assert.Equal(t, time.Minute, unusedValueSweepInterval())

	configureUnusedValues(t, &zero, &zero)
	grace, on = unusedValueGrace(context.Background())
	assert.True(t, on)
	assert.Equal(t, time.Second, grace)
	assert.Zero(t, unusedValueSweepInterval())

	configureUnusedValues(t, &negative, nil)
	_, on = unusedValueGrace(context.Background())
	assert.False(t, on)

	config.ResetServerRuntime()
	_, on = unusedValueGrace(context.Background())
	assert.False(t, on)
	assert.Zero(t, unusedValueSweepInterval())
}
