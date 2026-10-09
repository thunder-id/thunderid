// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

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

// A separator that could appear inside a part would let two different triples render one digest.
func (s *RevocationUtilsTestSuite) TestValueDoesNotCollideAcrossPartBoundaries() {
	s.NotEqual(
		EntityScopeCriterionValue("a", "b", "c"),
		EntityScopeCriterionValue("a|b", "", "c"))
}

// The value must fit CRITERION_VALUE, which is what the digest exists for.
func (s *RevocationUtilsTestSuite) TestValueFitsTheColumn() {
	long := make([]byte, 2048)
	for i := range long {
		long[i] = 'a'
	}
	s.Len(EntityScopeCriterionValue("entity-1", string(long), string(long)), 64)
}

// A resource server identifier carries no character restriction, so it may contain the separator.
func (s *RevocationUtilsTestSuite) TestEntityScopeCriterionValue_SeparatorInAPartDoesNotCollide() {
	s.NotEqual(
		EntityScopeCriterionValue("entity", "https://api|dmv", "license"),
		EntityScopeCriterionValue("entity|https://api", "dmv", "license"),
		"shifting the separator between parts must not produce the same criterion")
}

// The two dimensions are looked up by type as well as value, but their renderings should not collide either.
func (s *RevocationUtilsTestSuite) TestScopeCriterionValue_DoesNotCollideWithAnEntityScopeValue() {
	s.NotEqual(
		ScopeCriterionValue("entity|https://api.dmv", "license"),
		EntityScopeCriterionValue("entity", "https://api.dmv", "license"))
}
