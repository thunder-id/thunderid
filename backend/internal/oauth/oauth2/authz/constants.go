// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import "time"

// defaultAuthzRequestValidity is the authorization request context validity used when
// oauth.authorization_request.validity_period is not configured.
const defaultAuthzRequestValidity = 60 * time.Minute

// Authorization code states.
const (
	AuthCodeStateActive   = "ACTIVE"
	AuthCodeStateInactive = "INACTIVE"
	AuthCodeStateExpired  = "EXPIRED"
	AuthCodeStateRevoked  = "REVOKED"
)
