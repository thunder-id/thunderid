// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// AttrRulesTestSuite groups the tests in attr_rules_test.go.
type AttrRulesTestSuite struct {
	suite.Suite
}

// TestAttrRulesTestSuite runs AttrRulesTestSuite.
func TestAttrRulesTestSuite(t *testing.T) {
	suite.Run(t, new(AttrRulesTestSuite))
}

var testCoreRules = []CoreAttrRule{
	{Candidate: "email", SCIMField: fieldEmails, Kind: KindMultiComplex, ValueKey: "value"},
	{Candidate: "given_name", Kind: KindSubAttr, ParentField: fieldName, SubAttr: "givenName"},
	{Candidate: "family_name", Kind: KindSubAttr, ParentField: fieldName, SubAttr: "familyName"},
	{Candidate: "street", Kind: KindMultiComplexPart, ParentField: fieldAddresses, SubAttr: "streetAddress"},
	{Candidate: "username", SCIMField: fieldUserName, Kind: KindSimpleString},
}

var testEnterpriseRules = []EnterpriseAttrRule{
	{Candidate: "dept", SCIMField: EnterpriseFieldDepartment},
	{Candidate: "mgr", SCIMField: EnterpriseFieldManager, IsComplex: true},
}

// TestCandidatesForField returns the candidates rolling into a core field.
func (suite *AttrRulesTestSuite) TestCandidatesForField() {
	t := suite.T()
	require.Equal(t, []string{"username"}, CandidatesForField(testCoreRules, fieldUserName))
	require.Equal(t, []string{"given_name", "family_name"}, CandidatesForField(testCoreRules, fieldName))
	require.Nil(t, CandidatesForField(testCoreRules, fieldTitle))
	require.Nil(t, CandidatesForField(nil, fieldUserName))
}

// TestCandidatesForSubAttr_MappedSubAttr_ReturnsItsCandidate tests that a sub-attribute backed
// by its own CoreAttrRule (e.g. name.givenName) returns exactly that candidate.
func (suite *AttrRulesTestSuite) TestCandidatesForSubAttr_MappedSubAttr_ReturnsItsCandidate() {
	t := suite.T()
	require.Equal(t, []string{"given_name"}, CandidatesForSubAttr(testCoreRules, fieldName, "givenName"))
	require.Equal(t, []string{"family_name"}, CandidatesForSubAttr(testCoreRules, fieldName, "familyName"))
	require.Equal(t, []string{"street"}, CandidatesForSubAttr(testCoreRules, fieldAddresses, "streetAddress"))
}

// TestCandidatesForSubAttr_ProtocolOnlySubAttr_ReturnsNil tests that sub-attributes with no
// rule of their own return nil.
func (suite *AttrRulesTestSuite) TestCandidatesForSubAttr_ProtocolOnlySubAttr_ReturnsNil() {
	t := suite.T()
	require.Nil(t, CandidatesForSubAttr(testCoreRules, fieldEmails, "value"))
	require.Nil(t, CandidatesForSubAttr(testCoreRules, fieldEmails, "type"))
	require.Nil(t, CandidatesForSubAttr(testCoreRules, fieldEmails, "primary"))
}

// TestCandidatesForEnterpriseField tests Enterprise attribute candidate lookups.
func (suite *AttrRulesTestSuite) TestCandidatesForEnterpriseField() {
	t := suite.T()
	require.Equal(t, []string{"dept"}, CandidatesForEnterpriseField(testEnterpriseRules, EnterpriseFieldDepartment))
	require.Equal(t, []string{"mgr"}, CandidatesForEnterpriseField(testEnterpriseRules, EnterpriseFieldManager))
	require.Nil(t, CandidatesForEnterpriseField(testEnterpriseRules, EnterpriseFieldDivision))
}

// TestIsEnterpriseCandidate tests checking if an attribute name is an enterprise candidate.
func (suite *AttrRulesTestSuite) TestIsEnterpriseCandidate() {
	t := suite.T()
	require.True(t, IsEnterpriseCandidate(testEnterpriseRules, "dept"))
	require.True(t, IsEnterpriseCandidate(testEnterpriseRules, "DEPT"))
	require.False(t, IsEnterpriseCandidate(testEnterpriseRules, "username"))
	require.False(t, IsEnterpriseCandidate(nil, "dept"))
}

// TestIsCoreCandidate tests checking if an attribute name is a core candidate.
func (suite *AttrRulesTestSuite) TestIsCoreCandidate() {
	t := suite.T()
	require.True(t, IsCoreCandidate(testCoreRules, "username"))
	require.True(t, IsCoreCandidate(testCoreRules, "USERNAME"))
	require.True(t, IsCoreCandidate(testCoreRules, "given_name"))
	require.False(t, IsCoreCandidate(testCoreRules, "dept"))
	require.False(t, IsCoreCandidate(nil, "username"))
}

// TestHasSchemaMatch_CredentialCandidate_ReturnsCredentialTrue tests that when a matching
// candidate's own schema property is flagged credential, HasSchemaMatch surfaces that so
// callers can derive Returned/Mutability characteristics.
func (suite *AttrRulesTestSuite) TestHasSchemaMatch_CredentialCandidate_ReturnsCredentialTrue() {
	t := suite.T()
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
func (suite *AttrRulesTestSuite) TestHasSchemaMatch_NoMatch_ReturnsFalse() {
	t := suite.T()
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
func (suite *AttrRulesTestSuite) TestHasSchemaMatch_CredentialAggregatesAcrossAllMatchingCandidates() {
	t := suite.T()
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
