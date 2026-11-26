// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"encoding/json"
	"maps"
	"reflect"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/role"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
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

// resolvedMappedAttributes returns the local attributes whose connection mapping finds its external
// attribute in claims.
func resolvedMappedAttributes(claims map[string]interface{},
	mappings []providers.AttributeMapping) map[string]bool {
	mapped := make(map[string]bool, len(mappings))
	for _, mapping := range mappings {
		if _, ok := systemutils.GetNestedValue(claims, mapping.ExternalAttribute); ok {
			mapped[mapping.LocalAttribute] = true
		}
	}
	return mapped
}

// jsonEqual reports whether a and b hold the same value once encoded, so a stored number and a received
// one of a different Go type compare equal. Values that can't be encoded are never equal.
func jsonEqual(a, b interface{}) bool {
	normalize := func(v interface{}) (interface{}, bool) {
		encoded, err := json.Marshal(v)
		if err != nil {
			return nil, false
		}
		var out interface{}
		if err := json.Unmarshal(encoded, &out); err != nil {
			return nil, false
		}
		return out, true
	}
	normalizedA, okA := normalize(a)
	normalizedB, okB := normalize(b)
	return okA && okB && reflect.DeepEqual(normalizedA, normalizedB)
}

// groupMemberType returns how an entity of the category is named as a group member.
func groupMemberType(category providers.EntityCategory) group.MemberType {
	switch category {
	case providers.EntityCategoryApp:
		return group.MemberTypeApp
	case providers.EntityCategoryAgent:
		return group.MemberTypeAgent
	default:
		return group.MemberTypeUser
	}
}

// roleAssigneeType returns how an entity of the category is named as a role assignee.
func roleAssigneeType(category providers.EntityCategory) role.AssigneeType {
	switch category {
	case providers.EntityCategoryApp:
		return role.AssigneeTypeApp
	case providers.EntityCategoryAgent:
		return role.AssigneeTypeAgent
	default:
		return role.AssigneeTypeUser
	}
}
