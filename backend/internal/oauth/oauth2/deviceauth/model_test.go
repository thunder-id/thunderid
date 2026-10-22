// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package deviceauth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeviceRequestStateValues(t *testing.T) {
	require.Equal(t, DeviceRequestState("PENDING"), DeviceStatePending)
	require.Equal(t, DeviceRequestState("REDEEMING"), DeviceStateRedeeming)
	require.Equal(t, DeviceRequestState("APPROVED"), DeviceStateApproved)
	require.Equal(t, DeviceRequestState("CONSUMED"), DeviceStateConsumed)
	require.Equal(t, DeviceRequestState("DENIED"), DeviceStateDenied)
	require.Equal(t, DeviceRequestState("FAILED"), DeviceStateFailed)
	require.Equal(t, DeviceRequestState("EXPIRED"), DeviceStateExpired)
}
