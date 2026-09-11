// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/entitytype"
	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	scim "github.com/thunder-id/thunderid/internal/scim/common"
)

// TestMapRawPropertyToSCIMAttribute_CredentialArrayItems_PropagatesNeverReturned tests Map Raw Property To
// SCIM Attribute for Credential Array Items Propagates Never Returned.
func TestMapRawPropertyToSCIMAttribute_CredentialArrayItems_PropagatesNeverReturned(t *testing.T) {
	def := scim.RawPropertyDef{
		Type: entitytypemodel.TypeArray,
		Items: &scim.RawPropertyDef{
			Type:       entitytypemodel.TypeObject,
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

// TestBuildEnterpriseUserSchema_Success tests buildEnterpriseUserSchema for an entity type with enterprise attributes.
func TestBuildEnterpriseUserSchema_Success(t *testing.T) {
	et := entitytype.EntityType{
		ID:   "et-emp",
		Name: "Employee",
		Schema: []byte(`{
			"employee_number": {"type": "string", "required": true},
			"cost_center": {"type": "string"},
			"department": {"type": "string"},
			"manager_id": {"type": "string"}
		}`),
	}

	schema, err := buildEnterpriseUserSchema("https://example.com", et)
	require.NoError(t, err)
	require.Equal(t, scim.SCIMEnterpriseUserSchemaURN, schema.ID)
	require.Equal(t, "EnterpriseUser", schema.Name)
	require.Equal(t, "https://example.com/scim/v2/Schemas/"+scim.SCIMEnterpriseUserSchemaURN, schema.Meta.Location)
	require.Len(t, schema.Attributes, 4)

	var mgrAttr *scimSchemaAttribute
	for i := range schema.Attributes {
		if schema.Attributes[i].Name == "manager" {
			mgrAttr = &schema.Attributes[i]
			break
		}
	}
	require.NotNil(t, mgrAttr)
	require.Equal(t, scimAttrTypeComplex, mgrAttr.Type)
	require.Len(t, mgrAttr.SubAttributes, 2)
	require.Equal(t, "value", mgrAttr.SubAttributes[0].Name)
	require.Equal(t, "$ref", mgrAttr.SubAttributes[1].Name)
	require.Equal(t, scimAttrTypeReference, mgrAttr.SubAttributes[1].Type)
	require.Equal(t, []string{"User"}, mgrAttr.SubAttributes[1].ReferenceTypes)
}

// TestBuildEnterpriseUserSchema_NoEnterpriseAttrs tests buildEnterpriseUserSchema when user type
// has no enterprise attributes.
func TestBuildEnterpriseUserSchema_NoEnterpriseAttrs(t *testing.T) {
	et := entitytype.EntityType{
		ID:   "et-user",
		Name: "User",
		Schema: []byte(`{
			"username": {"type": "string"},
			"email": {"type": "string"}
		}`),
	}

	schema, err := buildEnterpriseUserSchema("https://example.com", et)
	require.NoError(t, err)
	require.Empty(t, schema.Attributes)
}

// TestBuildEnterpriseUserSchema_MalformedJSON tests buildEnterpriseUserSchema with invalid JSON.
func TestBuildEnterpriseUserSchema_MalformedJSON(t *testing.T) {
	et := entitytype.EntityType{
		ID:     "et-bad",
		Name:   "Bad",
		Schema: []byte(`invalid json`),
	}

	_, err := buildEnterpriseUserSchema("https://example.com", et)
	require.Error(t, err)
}

// TestBuildCoreUserSchema_CredentialMappedCandidate_DeclaresNeverWriteOnly tests that a
// core-mapped candidate (username) flagged credential in the designated core type's own
// schema is declared never-returned/write-only in /Schemas, matching what response
// filtering already omits at runtime. Before this change coreUserAttributes never
// consulted a matched candidate's own Credential flag, so /Schemas could disagree with
// the actual response for this case.
func TestBuildCoreUserSchema_CredentialMappedCandidate_DeclaresNeverWriteOnly(t *testing.T) {
	et := entitytype.EntityType{
		ID:   "et-user",
		Name: "User",
		Schema: []byte(`{
			"username": {"type": "string", "required": true, "credential": true},
			"email": {"type": "string"}
		}`),
	}

	schema, err := buildCoreUserSchema("https://example.com", et)
	require.NoError(t, err)

	var userNameAttr *scimSchemaAttribute
	for i := range schema.Attributes {
		if schema.Attributes[i].Name == "userName" {
			userNameAttr = &schema.Attributes[i]
			break
		}
	}
	require.NotNil(t, userNameAttr)
	require.Equal(t, scimReturnedNever, userNameAttr.Returned)
	require.Equal(t, scimMutabilityWriteOnly, userNameAttr.Mutability)
}
