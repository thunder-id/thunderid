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
	// ErrorCodeAccountLocked is returned when an automatic lock refuses the identity.
	ErrorCodeAccountLocked = "AUP-0009"
	// ErrorCodeAccountSuspended is returned when a suspension refuses the identity.
	ErrorCodeAccountSuspended = "AUP-0010"
)
