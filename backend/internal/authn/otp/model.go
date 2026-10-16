// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package otp

// VerificationFailure names the recipient a one-time password was compared for and did not match.
// It travels beside the authentication error, never inside it, which identifies nobody. Returned
// only for a wrong code; nil when nothing was compared, such as an unreadable session or an expired
// code.
type VerificationFailure struct {
	// RecipientAttr is the attribute the recipient is, for example mobile_number.
	RecipientAttr string
	// Recipient is the identifier the code was sent to.
	Recipient string
}
