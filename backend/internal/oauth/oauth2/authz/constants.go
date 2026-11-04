// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import "time"

// defaultAuthzRequestValidity is the authorization request context validity used when
// oauth.authorization_request.validity_period is not configured.
const defaultAuthzRequestValidity = 60 * time.Minute

// maxDeviceSessions bounds how many posted SSO sessions a code keeps, so a caller cannot inflate the code
// and the device secret.
const maxDeviceSessions = 10

// maxDeviceSessionFieldLength bounds each field of a posted SSO session the code keeps. Real flow identifiers
// and session handles are far shorter.
const maxDeviceSessionFieldLength = 128

// Authorization code states.
const (
	AuthCodeStateActive   = "ACTIVE"
	AuthCodeStateInactive = "INACTIVE"
	AuthCodeStateExpired  = "EXPIRED"
	AuthCodeStateRevoked  = "REVOKED"
)
