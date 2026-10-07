// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"errors"
	"fmt"
	"time"
)

// ErrTokenRevoked indicates the presented token's JTI is on the deny list.
var ErrTokenRevoked = errors.New("token has been revoked")

// ErrEnforcementUnavailable indicates the deny list could not be consulted (runtime persistent DB
// unavailable or the circuit is open). Under the fail-closed policy callers MUST reject the token.
var ErrEnforcementUnavailable = errors.New("token revocation enforcement is unavailable")

// ErrTokenRotated indicates the presented token was revoked by refresh token rotation rather than
// by an explicit revocation. It is a denial: every caller MUST reject the token, except the
// refresh_token grant, which may accept it inside a grace window so that legitimate concurrent
// refresh requests succeed. It wraps ErrTokenRevoked so that callers testing with errors.Is(err,
// ErrTokenRevoked) keep rejecting it without change.
var ErrTokenRotated = fmt.Errorf("%w by refresh token rotation", ErrTokenRevoked)

// RotatedTokenError reports a denial caused by refresh token rotation, carrying the instant the
// rotation happened so a caller can decide whether the presentation falls inside a grace window.
//
// The enforcement service reports this fact rather than applying a window itself: the window is
// per-application policy, and the enforcement service is a process-wide singleton shared with
// introspection and token exchange, neither of which has an application in hand. Deciding here
// would force one deployment-wide answer on every caller.
type RotatedTokenError struct {
	// RotatedAt is the instant the token was rotated out of use, read from the deny-list row. It is
	// immutable for the life of the row because the deny-list insert is idempotent, so redeeming a
	// token inside its window cannot move it.
	RotatedAt time.Time
}

// Error implements error.
func (e *RotatedTokenError) Error() string {
	return ErrTokenRotated.Error()
}

// Unwrap reports ErrTokenRotated, and transitively ErrTokenRevoked, so every existing caller that
// tests errors.Is(err, ErrTokenRevoked) continues to reject a rotated token with no change.
func (e *RotatedTokenError) Unwrap() error {
	return ErrTokenRotated
}
