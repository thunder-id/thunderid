// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

var testScopes = []string{"license"}

type RevocationUtilsTestSuite struct {
	suite.Suite
}

func TestRevocationUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(RevocationUtilsTestSuite))
}

func (s *RevocationUtilsTestSuite) TestEntityScopeCriterionValue_AudienceSeparatesIdenticalScopes() {
	california := EntityScopeCriterionValue("entity-1", "https://api.dmv.ca.gov", "license")
	ohio := EntityScopeCriterionValue("entity-1", "https://api.dmv.oh.gov", "license")

	s.NotEqual(california, ohio)
}

func (s *RevocationUtilsTestSuite) TestEntityScopeCriterionValue_EntitySeparatesPrincipals() {
	s.NotEqual(
		EntityScopeCriterionValue("entity-1", "https://api.dmv.ca.gov", "license"),
		EntityScopeCriterionValue("entity-2", "https://api.dmv.ca.gov", "license"))
}

func (s *RevocationUtilsTestSuite) TestEntityScopeCriterionValue_IsStable() {
	s.Equal(
		EntityScopeCriterionValue("entity-1", "https://api.dmv.ca.gov", "license"),
		EntityScopeCriterionValue("entity-1", "https://api.dmv.ca.gov", "license"))
}

func (s *RevocationUtilsTestSuite) TestScopeAndEntityScopeValuesDoNotCollide() {
	s.NotEqual(
		ScopeCriterionValue("https://api.dmv.ca.gov", "license"),
		EntityScopeCriterionValue("", "https://api.dmv.ca.gov", "license"))
}

func (s *RevocationUtilsTestSuite) TestValueDoesNotCollideAcrossPartBoundaries() {
	s.NotEqual(
		EntityScopeCriterionValue("a", "b", "c"),
		EntityScopeCriterionValue("a|b", "", "c"))
}

func (s *RevocationUtilsTestSuite) TestValueFitsTheColumn() {
	long := make([]byte, 2048)
	for i := range long {
		long[i] = 'a'
	}
	s.Len(EntityScopeCriterionValue("entity-1", string(long), string(long)), 64)
}

func (s *RevocationUtilsTestSuite) TestEntityScopeCriterionValue_SeparatorInAPartDoesNotCollide() {
	s.NotEqual(
		EntityScopeCriterionValue("entity", "https://api|dmv", "license"),
		EntityScopeCriterionValue("entity|https://api", "dmv", "license"),
		"shifting the separator between parts must not produce the same criterion")
}

func (s *RevocationUtilsTestSuite) TestScopeCriterionValue_DoesNotCollideWithAnEntityScopeValue() {
	s.NotEqual(
		ScopeCriterionValue("entity|https://api.dmv", "license"),
		EntityScopeCriterionValue("entity", "https://api.dmv", "license"))
}

func (s *RevocationUtilsTestSuite) TestTokenAudiences_ReadsBothEncodings() {
	auds, err := TokenAudiences(map[string]interface{}{"aud": "https://api.dmv.ca.gov"}, testScopes)
	s.Require().NoError(err)
	s.Equal([]string{"https://api.dmv.ca.gov"}, auds)

	auds, err = TokenAudiences(map[string]interface{}{
		"aud": []interface{}{"https://api.dmv.ca.gov", "https://api.dmv.oh.gov"}}, testScopes)
	s.Require().NoError(err)
	s.Equal([]string{"https://api.dmv.ca.gov", "https://api.dmv.oh.gov"}, auds)
}

func (s *RevocationUtilsTestSuite) TestTokenAudiences_RefreshTokenReadsAccessTokenAudience() {
	auds, err := TokenAudiences(map[string]interface{}{
		"aud":              "https://thunderid.example.com",
		"access_token_sub": "user-1",
		"access_token_aud": []interface{}{"https://api.dmv.ca.gov"},
	}, testScopes)

	s.Require().NoError(err)
	s.Equal([]string{"https://api.dmv.ca.gov"}, auds)
}

func (s *RevocationUtilsTestSuite) TestTokenAudiences_MalformedRefreshTokenAudienceIsAnError() {
	auds, err := TokenAudiences(map[string]interface{}{
		"aud":              "https://thunderid.example.com",
		"access_token_sub": "user-1",
		"access_token_aud": "",
	}, testScopes)

	s.ErrorIs(err, errMalformedAudience)
	s.Nil(auds)
}

func (s *RevocationUtilsTestSuite) TestTokenAudiences_AbsentClaimNamesNone() {
	auds, err := TokenAudiences(map[string]interface{}{}, testScopes)

	s.Require().NoError(err)
	s.Empty(auds)
}

func (s *RevocationUtilsTestSuite) TestTokenAudiences_MalformedClaimIsAnError() {
	testCases := []struct {
		name  string
		claim interface{}
	}{
		{"EmptyString", ""},
		{"EmptyArray", []interface{}{}},
		{"NonStringElement", []interface{}{"https://api.dmv.ca.gov", float64(1)}},
		{"EmptyElement", []interface{}{"https://api.dmv.ca.gov", ""}},
		{"UnsupportedType", float64(1)},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			auds, err := TokenAudiences(map[string]interface{}{"aud": tc.claim}, testScopes)

			s.ErrorIs(err, errMalformedAudience)
			s.Nil(auds)
		})
	}
}

func (s *RevocationUtilsTestSuite) TestTokenAudiences_TokenWithoutScopesIsNotRead() {
	auds, err := TokenAudiences(map[string]interface{}{"aud": float64(1)}, nil)

	s.Require().NoError(err)
	s.Empty(auds)
}

func (s *RevocationUtilsTestSuite) TestScopeCriteria_CoversEveryAudienceAndScope() {
	criteria := ScopeCriteria("user-1",
		[]string{"https://api.dmv.ca.gov", "https://api.dmv.oh.gov"}, []string{"license", "registration"})

	s.Len(criteria, 8)
	for _, audience := range []string{"https://api.dmv.ca.gov", "https://api.dmv.oh.gov"} {
		for _, scope := range []string{"license", "registration"} {
			s.Contains(criteria, Criterion{
				Type: CriterionTypeEntityScope, Value: EntityScopeCriterionValue("user-1", audience, scope)})
			s.Contains(criteria, Criterion{Type: CriterionTypeScope, Value: ScopeCriterionValue(audience, scope)})
		}
	}
}

func (s *RevocationUtilsTestSuite) TestScopeCriteria_NoSubjectYieldsOnlyScopeCriteria() {
	criteria := ScopeCriteria("", []string{"https://api.dmv.ca.gov"}, []string{"license"})

	s.Equal([]Criterion{{
		Type: CriterionTypeScope, Value: ScopeCriterionValue("https://api.dmv.ca.gov", "license")}}, criteria)
}

func (s *RevocationUtilsTestSuite) TestScopeCriteria_NoAudienceOrNoScopeYieldsNone() {
	s.Empty(ScopeCriteria("user-1", nil, []string{"license"}))
	s.Empty(ScopeCriteria("user-1", []string{"https://api.dmv.ca.gov"}, nil))
}
