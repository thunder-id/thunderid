// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import "github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

// verificationReceipt records one delivery of a verification challenge: the channel it was sent on and
// the canonical contact it was sent to.
type verificationReceipt struct {
	Channel providers.VerificationChannel `json:"channel"`
	Value   string                        `json:"value"`
}

// verificationEvidence is the trusted proof an attribute verification flow builds on shared runtime
// data, never from request input: the pinned contact, the challenge issued after the pin, whether it
// was passed, and where it was delivered.
type verificationEvidence struct {
	Pin       *providers.AttributeVerificationPin `json:"pin"`
	Challenge string                              `json:"challenge"`
	Passed    bool                                `json:"passed"`
	Receipts  []verificationReceipt               `json:"receipts"`
}

// verificationClaim is a verification claim the server issues: the contact attribute it describes and
// whether that contact is verified.
type verificationClaim struct {
	contact  string
	verified bool
}
