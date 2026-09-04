// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"testing"

	"github.com/stretchr/testify/require"

	scim "github.com/thunder-id/thunderid/internal/scim/common"
)

// TestMapRawPropertyToSCIMAttribute_CredentialArrayItems_PropagatesNeverReturned tests Map Raw Property To
// SCIM Attribute for Credential Array Items Propagates Never Returned.
func TestMapRawPropertyToSCIMAttribute_CredentialArrayItems_PropagatesNeverReturned(t *testing.T) {
	def := scim.RawPropertyDef{
		Type: scim.RawPropertyTypeArray,
		Items: &scim.RawPropertyDef{
			Type:       scim.RawPropertyTypeObject,
			Credential: true,
			Properties: map[string]scim.RawPropertyDef{
				"secret": {Type: "string", Credential: true},
			},
		},
	}
	attr := mapRawPropertyToSCIMAttribute("recovery_codes", def)
	require.Equal(t, scimReturnedNever, attr.Returned)
	require.Equal(t, scimMutabilityWriteOnly, attr.Mutability)
}
