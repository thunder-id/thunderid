// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package otp

// VerificationFailure names the recipient of a one-time password that did not match.
type VerificationFailure struct {
	// RecipientAttr is the attribute that holds the recipient, for example mobile_number.
	RecipientAttr string
	// Recipient is the identifier the code was sent to.
	Recipient string
}
