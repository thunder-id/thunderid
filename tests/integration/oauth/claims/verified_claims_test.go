// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package claims

import (
	"github.com/stretchr/testify/assert"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// TestVerifiedClaims_ObjectForm_Accepted verifies that a well-formed verified_claims request
// (OIDC Identity Assurance), given as a single object, is accepted end to end: it does not break
// the authorization flow or token issuance, even though the requested verified claim is not
// currently resolved into the issued token's attributes.
func (ts *ClaimsParameterTestSuite) TestVerifiedClaims_ObjectForm_Accepted() {
	claimsParam := `{"userinfo":{"verified_claims":{
		"verification":{"trust_framework":{"value":"de_aml"}},
		"claims":{"given_name":null}
	}}}`

	accessToken, _, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "A well-formed verified_claims request should not break token issuance")
	ts.Require().NotEmpty(accessToken)
}

// TestVerifiedClaims_ArrayForm_Accepted verifies that the array form of verified_claims (multiple
// verification contexts) is also accepted.
func (ts *ClaimsParameterTestSuite) TestVerifiedClaims_ArrayForm_Accepted() {
	claimsParam := `{"userinfo":{"verified_claims":[
		{"verification":{"trust_framework":{"value":"de_aml"}},"claims":{"given_name":null}},
		{"verification":{"trust_framework":null},"claims":{"family_name":null}}
	]}}`

	accessToken, _, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "The array form of verified_claims should be accepted")
	ts.Require().NotEmpty(accessToken)
}

// TestVerifiedClaims_MissingVerification_InvalidRequest verifies that a verified_claims entry
// missing the required verification member is rejected as invalid_request.
func (ts *ClaimsParameterTestSuite) TestVerifiedClaims_MissingVerification_InvalidRequest() {
	claimsParam := `{"userinfo":{"verified_claims":{
		"claims":{"given_name":null}
	}}}`

	ts.assertVerifiedClaimsRejected(claimsParam)
}

// TestVerifiedClaims_MissingTrustFramework_InvalidRequest verifies that a verification element
// missing the required trust_framework member is rejected as invalid_request.
func (ts *ClaimsParameterTestSuite) TestVerifiedClaims_MissingTrustFramework_InvalidRequest() {
	claimsParam := `{"userinfo":{"verified_claims":{
		"verification":{},
		"claims":{"given_name":null}
	}}}`

	ts.assertVerifiedClaimsRejected(claimsParam)
}

// TestVerifiedClaims_EmptyClaims_InvalidRequest verifies that a verified_claims entry with an
// empty claims object (requesting nothing) is rejected as invalid_request.
func (ts *ClaimsParameterTestSuite) TestVerifiedClaims_EmptyClaims_InvalidRequest() {
	claimsParam := `{"userinfo":{"verified_claims":{
		"verification":{"trust_framework":"de_aml"},
		"claims":{}
	}}}`

	ts.assertVerifiedClaimsRejected(claimsParam)
}

// TestVerifiedClaims_ClaimValueAndValuesConflict_InvalidRequest verifies that a nested claim
// inside verified_claims.claims specifying both value and values (mutually exclusive per OIDC) is
// rejected as invalid_request.
func (ts *ClaimsParameterTestSuite) TestVerifiedClaims_ClaimValueAndValuesConflict_InvalidRequest() {
	claimsParam := `{"userinfo":{"verified_claims":{
		"verification":{"trust_framework":"de_aml"},
		"claims":{"given_name":{"value":"Ada","values":["Ada","Grace"]}}
	}}}`

	ts.assertVerifiedClaimsRejected(claimsParam)
}

// TestVerifiedClaims_TrustFrameworkValueAndValuesConflict_InvalidRequest verifies that a
// constrained trust_framework specifying both value and values is rejected as invalid_request.
func (ts *ClaimsParameterTestSuite) TestVerifiedClaims_TrustFrameworkValueAndValuesConflict_InvalidRequest() {
	claimsParam := `{"userinfo":{"verified_claims":{
		"verification":{"trust_framework":{"value":"de_aml","values":["de_aml","de_jlt"]}},
		"claims":{"given_name":null}
	}}}`

	ts.assertVerifiedClaimsRejected(claimsParam)
}

// assertVerifiedClaimsRejected initiates the authorization flow with the given malformed
// verified_claims request and asserts it is rejected as invalid_request.
func (ts *ClaimsParameterTestSuite) assertVerifiedClaimsRejected(claimsParam string) {
	authzResp, err := testutils.InitiateAuthorizationFlowWithClaims(
		clientID, redirectURI, "code", "openid", "test_state", claimsParam,
	)
	ts.Require().NoError(err, "Failed to initiate authorization")
	defer authzResp.Body.Close()

	location := authzResp.Header.Get("Location")
	ts.Require().NotEmpty(location, "Expected a redirect for the malformed verified_claims request")

	err = testutils.ValidateOAuth2ErrorRedirect(location, "invalid_request", "")
	assert.NoError(ts.T(), err, "Should receive invalid_request error for malformed verified_claims")
}
