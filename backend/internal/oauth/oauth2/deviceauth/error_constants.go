// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package deviceauth

import "errors"

var (
	// ErrDeviceAuthRequestNotFound is returned when a device authorization request is not found.
	ErrDeviceAuthRequestNotFound = errors.New("device authorization request not found")
	// ErrDeviceAuthUserCodeCollision is returned when a user code is already in use.
	ErrDeviceAuthUserCodeCollision = errors.New("device authorization user code already exists")
	// ErrDeviceAuthDeviceCodeCollision is returned when a device code is already in use.
	ErrDeviceAuthDeviceCodeCollision = errors.New("device authorization device code already exists")
	// ErrDeviceAuthRequestIDCollision is returned when a device request ID is already in use.
	ErrDeviceAuthRequestIDCollision = errors.New("device authorization request ID already exists")
)
