// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

// Error codes.
const (
	ErrorCodeSystemError          = "AUP-0001"
	ErrorCodeAuthenticationFailed = "AUP-0002"
	ErrorCodeUserNotFound         = "AUP-0003"
	ErrorCodeInvalidToken         = "AUP-0004"
	ErrorCodeNotImplemented       = "AUP-0005"
	ErrorCodeInvalidRequest       = "AUP-0006"
	ErrorCodeAmbiguousUser        = "AUP-0007"
	ErrorCodeEnrollmentFailed     = "AUP-0008"
	// ErrorCodeAccountLocked and ErrorCodeAccountSuspended say the proof was correct but a hold
	// refused the identity. A refusal that names no hold raises ErrorCodeAuthenticationFailed, like a
	// wrong secret.
	ErrorCodeAccountLocked    = "AUP-0009"
	ErrorCodeAccountSuspended = "AUP-0010"
)
