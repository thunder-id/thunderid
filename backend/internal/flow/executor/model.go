// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import "github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

// verificationEvidence is the trusted proof an attribute verification flow builds on shared runtime
// data, never from request input: the pinned contact and whether a challenge issued for it was passed.
type verificationEvidence struct {
	Pin    *providers.AttributeVerificationPin `json:"pin"`
	Passed bool                                `json:"passed"`
}
