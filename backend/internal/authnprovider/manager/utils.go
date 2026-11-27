// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"maps"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
)

// findPendingIdentity returns the first pending identity in authUser's provider states, with the provider
// holding it. A deferred sync signs in through the default provider, while the provider that verified the
// federated sign-in may be another one.
func findPendingIdentity(authUser providers.AuthUser) (string, *providers.FederatedIdentity) {
	for _, name := range authUser.ProviderNames() {
		if state, _ := authUser.StateFor(name); state.PendingIdentity != nil {
			return name, state.PendingIdentity
		}
	}
	return "", nil
}

// withSessionValues returns a copy of claims carrying values under the reserved session values claim.
func withSessionValues(claims providers.AuthenticatedClaims,
	values *authnprovidercm.SessionValues) providers.AuthenticatedClaims {
	withValues := make(providers.AuthenticatedClaims, len(claims)+1)
	maps.Copy(withValues, claims)
	withValues[authnprovidercm.ClaimSessionValues] = values
	return withValues
}
