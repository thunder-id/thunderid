// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package verifier

import (
	"github.com/thunder-id/thunderid/internal/vc/truststore"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize builds the VC verifier registry with the built-in format verifiers registered.
func Initialize(trust truststore.TrustStoreInterface) VCVerifierRegistryInterface {
	return &vcVerifierRegistry{verifiers: map[string]providers.VCVerifierProvider{
		FormatSDJWTVC: newSDJWTVCVerifier(trust),
	}}
}
