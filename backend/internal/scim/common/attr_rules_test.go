// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCandidatesForSubAttr_MappedSubAttr_ReturnsItsCandidate tests that a sub-attribute backed
// by its own CoreAttrRule (e.g. name.givenName) returns exactly that candidate.
func TestCandidatesForSubAttr_MappedSubAttr_ReturnsItsCandidate(t *testing.T) {
	require.Equal(t, []string{"given_name"}, CandidatesForSubAttr(fieldName, "givenName"))
	require.Equal(t, []string{"family_name"}, CandidatesForSubAttr(fieldName, "familyName"))
	require.Equal(t, []string{"street_address"}, CandidatesForSubAttr(fieldAddresses, "streetAddress"))
}

// TestCandidatesForSubAttr_ProtocolOnlySubAttr_ReturnsNil tests that sub-attributes with no
// ThunderID-mapped candidate of their own (protocol metadata, or a value key that is really
// the parent rule's ValueKey) return nil rather than an empty non-nil slice or a false match.
func TestCandidatesForSubAttr_ProtocolOnlySubAttr_ReturnsNil(t *testing.T) {
	require.Nil(t, CandidatesForSubAttr(fieldEmails, "value"))
	require.Nil(t, CandidatesForSubAttr(fieldEmails, "type"))
	require.Nil(t, CandidatesForSubAttr(fieldEmails, "primary"))
	require.Nil(t, CandidatesForSubAttr(fieldAddresses, "formatted"))
}

// TestCandidatesForEnterpriseField tests Enterprise attribute candidate lookups.
func TestCandidatesForEnterpriseField(t *testing.T) {
	require.Equal(t, []string{"department"}, CandidatesForEnterpriseField(EnterpriseFieldDepartment))
	require.Equal(t, []string{"manager_id"}, CandidatesForEnterpriseField(EnterpriseFieldManager))
	require.Equal(t, []string{"employee_number"}, CandidatesForEnterpriseField(EnterpriseFieldEmployeeNumber))
	require.Equal(t, []string{"cost_center"}, CandidatesForEnterpriseField(EnterpriseFieldCostCenter))
	require.Equal(t, []string{"organization"}, CandidatesForEnterpriseField(EnterpriseFieldOrganization))
	require.Equal(t, []string{"division"}, CandidatesForEnterpriseField(EnterpriseFieldDivision))
}

// TestIsEnterpriseCandidate tests checking if an attribute name is an enterprise candidate.
func TestIsEnterpriseCandidate(t *testing.T) {
	require.True(t, IsEnterpriseCandidate("department"))
	require.True(t, IsEnterpriseCandidate("DEPARTMENT"))
	require.True(t, IsEnterpriseCandidate("manager_id"))
	require.False(t, IsEnterpriseCandidate("username"))
	require.False(t, IsEnterpriseCandidate("custom_attr"))
}

// TestIsCoreCandidate tests checking if an attribute name is a core candidate.
func TestIsCoreCandidate(t *testing.T) {
	require.True(t, IsCoreCandidate("username"))
	require.True(t, IsCoreCandidate("given_name"))
	require.True(t, IsCoreCandidate("email"))
	require.False(t, IsCoreCandidate("department"))
	require.False(t, IsCoreCandidate("custom_attr"))
}

// TestHasSchemaMatch_CredentialCandidate_ReturnsCredentialTrue tests that when a matching
// candidate's own schema property is flagged credential, HasSchemaMatch surfaces that so
// callers can derive Returned/Mutability characteristics.
func TestHasSchemaMatch_CredentialCandidate_ReturnsCredentialTrue(t *testing.T) {
	rawProps := map[string]RawPropertyDef{
		"username": {Required: true, Credential: true},
	}

	matched, required, credential := HasSchemaMatch(rawProps, []string{"username"})

	require.True(t, matched)
	require.True(t, required)
	require.True(t, credential)
}

// TestHasSchemaMatch_NoMatch_ReturnsFalse tests that a non-matching candidate list reports
// no match, not required, not credential.
func TestHasSchemaMatch_NoMatch_ReturnsFalse(t *testing.T) {
	rawProps := map[string]RawPropertyDef{
		"username": {Required: true},
	}

	matched, required, credential := HasSchemaMatch(rawProps, []string{"nickname"})

	require.False(t, matched)
	require.False(t, required)
	require.False(t, credential)
}

// TestHasSchemaMatch_CredentialAggregatesAcrossAllMatchingCandidates tests that when several
// candidates match (e.g. the sub-attributes rolling into a single complex parent like "name"),
// Credential is true if ANY matching property is credential-flagged - not just whichever
// candidate a map iteration happens to visit last. Run repeatedly because Go randomizes map
// iteration order per call, so a "last match wins" bug would fail this intermittently rather
// than every time.
func TestHasSchemaMatch_CredentialAggregatesAcrossAllMatchingCandidates(t *testing.T) {
	rawProps := map[string]RawPropertyDef{
		"given_name":  {},
		"family_name": {},
		"middle_name": {Credential: true},
		"name":        {},
	}
	candidates := []string{"given_name", "family_name", "middle_name", "name"}

	for i := 0; i < 50; i++ {
		_, _, credential := HasSchemaMatch(rawProps, candidates)
		require.True(t, credential,
			"credential must aggregate across all matching candidates regardless of map iteration order")
	}
}
