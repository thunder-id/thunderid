// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"testing"

	"github.com/stretchr/testify/suite"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
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

	s.Len(criteria, 4*(2+len(serverconst.ValidPermissionDelimiters)))
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

func (s *RevocationUtilsTestSuite) TestScopeCriteria_IncludesFrozenSubtreeKeysForEveryDelimiter() {
	const subject, audience = "user-1", "https://api.example.com"
	for _, separator := range serverconst.ValidPermissionDelimiters {
		delimiter := string(separator)
		child := "reports" + delimiter + "view"
		criteria := ScopeCriteria(subject, []string{audience}, []string{child})
		s.Contains(criteria, Criterion{Type: CriterionTypeEntityScope,
			Value: EntityScopeCriterionValue(subject, audience, child)})
		s.Contains(criteria, Criterion{Type: CriterionTypeScope,
			Value: ScopeCriterionValue(audience, child)})
		for _, root := range []string{"reports", child} {
			s.Contains(criteria, Criterion{Type: CriterionTypeEntityScopeSubtree,
				Value: EntityScopeSubtreeCriterionValue(subject, audience, root, delimiter)})
		}
	}
}

func (s *RevocationUtilsTestSuite) TestOUValues_UnitSeparatesTheSameGrant() {
	s.NotEqual(SubjectOUCriterionValue("user-1", "ou-a"), SubjectOUCriterionValue("user-1", "ou-b"))
	s.NotEqual(ApplicationKeyOUCriterionValue("client-1", "ou-a"), ApplicationKeyOUCriterionValue("client-1", "ou-b"))
	s.NotEqual(EntityScopeOUCriterionValue("user-1", "aud", "read", "ou-a"),
		EntityScopeOUCriterionValue("user-1", "aud", "read", "ou-b"))
	s.NotEqual(EntityScopeSubtreeOUCriterionValue("user-1", "aud", "reports", ":", "ou-a"),
		EntityScopeSubtreeOUCriterionValue("user-1", "aud", "reports", ":", "ou-b"))
}

func (s *RevocationUtilsTestSuite) TestOUValues_DoNotCollideWithTheirUnscopedValues() {
	s.NotEqual(EntityScopeCriterionValue("user-1", "aud", "read"),
		EntityScopeOUCriterionValue("user-1", "aud", "read", ""))
	s.NotEqual(EntityScopeSubtreeCriterionValue("user-1", "aud", "reports", ":"),
		EntityScopeSubtreeOUCriterionValue("user-1", "aud", "reports", ":", ""))
}

func (s *RevocationUtilsTestSuite) TestTokenOUID_ReadsTheUnitTheTokenWasIssuedIn() {
	s.Equal("ou-a", TokenOUID(map[string]interface{}{"sub": "user-1", "ouId": "ou-a"}))
	s.Equal("ou-a", TokenOUID(map[string]interface{}{
		"sub": "client-1", "access_token_sub": "user-1", "access_token_ou_id": "ou-a", "ouId": "ou-b",
	}), "a refresh token states the unit its access tokens are issued in")
	s.Empty(TokenOUID(map[string]interface{}{"sub": "user-1"}))
	s.Empty(TokenOUID(map[string]interface{}{"sub": "client-1", "access_token_sub": "user-1", "ouId": "ou-b"}),
		"a refresh token's own ouId is not its access tokens' unit")
	s.Empty(TokenOUID(map[string]interface{}{"ouId": 7}))
}

func (s *RevocationUtilsTestSuite) TestOUCriteria_CoversSubjectClientAndEveryScopeWithinTheUnit() {
	const subject, client, ouID, audience = "user-1", "client-1", "ou-a", "https://api.example.com"
	criteria := OUCriteria(subject, client, ouID, []string{audience}, []string{"reports:view"})

	s.Contains(criteria, Criterion{Type: CriterionTypeSubjectOU, Value: SubjectOUCriterionValue(subject, ouID)})
	s.Contains(criteria, Criterion{Type: CriterionTypeApplicationKeyOU,
		Value: ApplicationKeyOUCriterionValue(client, ouID)})
	s.Contains(criteria, Criterion{Type: CriterionTypeEntityScopeOU,
		Value: EntityScopeOUCriterionValue(subject, audience, "reports:view", ouID)})
	for _, root := range []string{"reports", "reports:view"} {
		s.Contains(criteria, Criterion{Type: CriterionTypeEntityScopeSubtreeOU,
			Value: EntityScopeSubtreeOUCriterionValue(subject, audience, root, ":", ouID)})
	}
	for _, criterion := range criteria {
		s.NotContains([]CriterionType{CriterionTypeSubject, CriterionTypeApplicationKey, CriterionTypeEntityScope,
			CriterionTypeEntityScopeSubtree, CriterionTypeScope}, criterion.Type, "only unit-scoped criteria")
	}
}

func (s *RevocationUtilsTestSuite) TestOUCriteria_ATokenStatingNoUnitHasNone() {
	s.Nil(OUCriteria("user-1", "client-1", "", []string{"aud"}, []string{"read"}))
	s.Nil(OUScopeCriteria("user-1", "", []string{"aud"}, []string{"read"}))
}

func (s *RevocationUtilsTestSuite) TestOUCriteria_WithoutASubjectOnlyTheClientIsChecked() {
	criteria := OUCriteria("", "client-1", "ou-a", []string{"aud"}, []string{"read"})

	s.Equal([]Criterion{{Type: CriterionTypeApplicationKeyOU,
		Value: ApplicationKeyOUCriterionValue("client-1", "ou-a")}}, criteria)
}

func (s *RevocationUtilsTestSuite) TestOUScopeCriteria_ListsEachSubtreeKeyOnce() {
	criteria := OUScopeCriteria("user-1", "ou-a", []string{"aud"}, []string{"reports", "reports"})

	seen := map[Criterion]int{}
	for _, criterion := range criteria {
		if criterion.Type == CriterionTypeEntityScopeSubtreeOU {
			seen[criterion]++
		}
	}
	s.NotEmpty(seen)
	for criterion, count := range seen {
		s.Equal(1, count, "%v", criterion)
	}
}
