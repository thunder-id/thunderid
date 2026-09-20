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

// The audience is what separates two resource servers that define the same permission string, which
// is the case this dimension exists for. Our own deployment has "license" on both a California and an
// Ohio DMV API, and revoking one must not touch the other.
func (s *RevocationUtilsTestSuite) TestEntityScopeCriterionValue_AudienceSeparatesIdenticalScopes() {
	california := EntityScopeCriterionValue("entity-1", "https://api.dmv.ca.gov", "license")
	ohio := EntityScopeCriterionValue("entity-1", "https://api.dmv.oh.gov", "license")

	s.NotEqual(california, ohio)
}

// Two principals holding the same scope on the same server must not share a row either.
func (s *RevocationUtilsTestSuite) TestEntityScopeCriterionValue_EntitySeparatesPrincipals() {
	s.NotEqual(
		EntityScopeCriterionValue("entity-1", "https://api.dmv.ca.gov", "license"),
		EntityScopeCriterionValue("entity-2", "https://api.dmv.ca.gov", "license"))
}

// The write path and both enforcement points derive the value independently, so the same inputs must
// always render the same digest.
func (s *RevocationUtilsTestSuite) TestEntityScopeCriterionValue_IsStable() {
	s.Equal(
		EntityScopeCriterionValue("entity-1", "https://api.dmv.ca.gov", "license"),
		EntityScopeCriterionValue("entity-1", "https://api.dmv.ca.gov", "license"))
}

// The two dimensions are stored in separate columns and separate cache maps, but a shared digest
// space would still be a latent hazard: keep them distinct at the source.
func (s *RevocationUtilsTestSuite) TestScopeAndEntityScopeValuesDoNotCollide() {
	s.NotEqual(
		ScopeCriterionValue("https://api.dmv.ca.gov", "license"),
		EntityScopeCriterionValue("", "https://api.dmv.ca.gov", "license"))
}

// A separator that could appear inside a part would let two different triples render one digest.
// Rendering the parts must keep them distinguishable.
func (s *RevocationUtilsTestSuite) TestValueDoesNotCollideAcrossPartBoundaries() {
	s.NotEqual(
		EntityScopeCriterionValue("a", "b", "c"),
		EntityScopeCriterionValue("a|b", "", "c"))
}

// The value must fit CRITERION_VALUE, which is what the digest exists for: a resource server
// identifier alone can be 2048 characters.
func (s *RevocationUtilsTestSuite) TestValueFitsTheColumn() {
	long := make([]byte, 2048)
	for i := range long {
		long[i] = 'a'
	}
	s.Len(EntityScopeCriterionValue("entity-1", string(long), string(long)), 64)
}

// A resource server identifier carries no character restriction, so it may contain the separator. The
// rendering must still be unambiguous: joining the parts alone made these two triples digest alike, so
// a revocation written for one would have matched a scope it was never written for.
func (s *RevocationUtilsTestSuite) TestEntityScopeCriterionValue_SeparatorInAPartDoesNotCollide() {
	s.NotEqual(
		EntityScopeCriterionValue("entity", "https://api|dmv", "license"),
		EntityScopeCriterionValue("entity|https://api", "dmv", "license"),
		"shifting the separator between parts must not produce the same criterion")
}

// The two dimensions are looked up by type as well as value, but their renderings should not collide
// either: a two-part scope value must differ from a three-part entity-scope value.
func (s *RevocationUtilsTestSuite) TestScopeCriterionValue_DoesNotCollideWithAnEntityScopeValue() {
	s.NotEqual(
		ScopeCriterionValue("entity|https://api.dmv", "license"),
		EntityScopeCriterionValue("entity", "https://api.dmv", "license"))
}
