// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package openid4vp

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type ClaimPolicyTestSuite struct {
	suite.Suite
}

func TestClaimPolicyTestSuite(t *testing.T) {
	suite.Run(t, new(ClaimPolicyTestSuite))
}

func testClaims() map[string]interface{} {
	return map[string]interface{}{
		"given_name":  "Erika",
		"address":     map[string]interface{}{"locality": "Berlin"},
		"nationality": []interface{}{"DE", "FR"},
	}
}

func (suite *ClaimPolicyTestSuite) TestAllClaimsRequiredWithoutClaimSets() {
	query := providers.DCQLCredentialQuery{ID: "pid", Claims: []providers.DCQLClaimsQuery{
		{Path: []interface{}{"given_name"}},
		{Path: []interface{}{"address", "locality"}},
	}}
	suite.NoError(verifyClaims(testClaims(), query))

	query.Claims = append(query.Claims, providers.DCQLClaimsQuery{Path: []interface{}{"birthdate"}})
	err := verifyClaims(testClaims(), query)
	suite.ErrorIs(err, ErrMissingMandatoryClaim)
	suite.ErrorContains(err, "birthdate")
}

func (suite *ClaimPolicyTestSuite) TestClaimSets() {
	query := providers.DCQLCredentialQuery{
		ID: "pid",
		Claims: []providers.DCQLClaimsQuery{
			{ID: "claim_0", Path: []interface{}{"given_name"}},
			{ID: "claim_1", Path: []interface{}{"birthdate"}},
		},
		ClaimSets: [][]string{{"claim_0", "claim_1"}, {"claim_0"}},
	}

	suite.Run("fallback set satisfied", func() {
		suite.NoError(verifyClaims(testClaims(), query))
	})
	suite.Run("no set satisfied", func() {
		claims := testClaims()
		delete(claims, "given_name")
		suite.ErrorIs(verifyClaims(claims, query), ErrMissingMandatoryClaim)
	})
}

func (suite *ClaimPolicyTestSuite) TestClaimValues() {
	query := providers.DCQLCredentialQuery{ID: "pid", Claims: []providers.DCQLClaimsQuery{
		{Path: []interface{}{"given_name"}, Values: []interface{}{"Erika", "Max"}},
	}}
	suite.NoError(verifyClaims(testClaims(), query))

	query.Claims[0].Values = []interface{}{"Max"}
	err := verifyClaims(testClaims(), query)
	suite.ErrorIs(err, ErrClaimValueNotAllowed)
	suite.ErrorContains(err, "given_name")
}

func (suite *ClaimPolicyTestSuite) TestOptionalClaimWithDisallowedValueRejected() {
	query := providers.DCQLCredentialQuery{
		ID: "pid",
		Claims: []providers.DCQLClaimsQuery{
			{ID: "claim_0", Path: []interface{}{"address", "locality"}},
			{ID: "claim_1", Path: []interface{}{"given_name"}, Values: []interface{}{"Max"}},
		},
		ClaimSets: [][]string{{"claim_0", "claim_1"}, {"claim_0"}},
	}
	suite.ErrorIs(verifyClaims(testClaims(), query), ErrClaimValueNotAllowed)
}

func (suite *ClaimPolicyTestSuite) TestLookupClaim() {
	claims := testClaims()

	value, ok := lookupClaim(claims, []interface{}{"address", "locality"})
	suite.True(ok)
	suite.Equal("Berlin", value)

	value, ok = lookupClaim(claims, []interface{}{"nationality", 1})
	suite.True(ok)
	suite.Equal("FR", value)

	for _, path := range [][]interface{}{
		nil,
		{"missing"},
		{"given_name", "x"},
		{"nationality", 5},
		{"address", 0},
		{"nationality", nil},
	} {
		_, ok = lookupClaim(claims, path)
		suite.False(ok, "path %v", path)
	}
}
