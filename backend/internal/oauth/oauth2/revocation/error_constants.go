// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"errors"
	"fmt"
)

// ErrTokenRevoked indicates the presented token is on a deny list, by its JTI or by a criterion it carries.
var ErrTokenRevoked = errors.New("token has been revoked")

// ErrTokenIDRevoked indicates the presented token's own JTI is on the deny list; it matches ErrTokenRevoked.
var ErrTokenIDRevoked = fmt.Errorf("%w by its jti", ErrTokenRevoked)

// ErrEnforcementUnavailable indicates the deny list could not be consulted (runtime persistent DB
// unavailable or the circuit is open). Under the fail-closed policy callers MUST reject the token.
var ErrEnforcementUnavailable = errors.New("token revocation enforcement is unavailable")
