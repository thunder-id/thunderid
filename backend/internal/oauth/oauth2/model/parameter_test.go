/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type ClaimsRequestTestSuite struct {
	suite.Suite
}

func TestClaimsRequestSuite(t *testing.T) {
	suite.Run(t, new(ClaimsRequestTestSuite))
}

// TestUnmarshalJSON_NormalizesUserInfo verifies that a plain json.Unmarshal (the path used by
// the Redis-backed request/code/PAR stores) splits the userinfo object into typed normal
// claims in UserInfo and the opaque verified_claims member in VerifiedUserInfo.
func (suite *ClaimsRequestTestSuite) TestUnmarshalJSON_NormalizesUserInfo() {
	raw := `{
		"userinfo": {
			"email": {"essential": true},
			"name": null,
			"sub": {"value": "alice"},
			"verified_claims": {
				"verification": {"trust_framework": {"value": "eidas"}},
				"claims": {"given_name": null}
			}
		},
		"id_token": {
			"auth_time": {"essential": true}
		}
	}`

	var cr ClaimsRequest
	err := json.Unmarshal([]byte(raw), &cr)
	assert.NoError(suite.T(), err)

	normal := cr.UserInfo
	assert.Len(suite.T(), normal, 3)

	assert.NotNil(suite.T(), normal["email"])
	assert.True(suite.T(), normal["email"].Essential)

	// "name": null is a requested-without-constraint claim.
	val, present := normal["name"]
	assert.True(suite.T(), present)
	assert.Nil(suite.T(), val)

	// sub value constraint must survive the plain-unmarshal path (security: it gates auth).
	assert.NotNil(suite.T(), normal["sub"])
	assert.True(suite.T(), normal["sub"].MatchesValue("alice"))
	assert.False(suite.T(), normal["sub"].MatchesValue("bob"))

	// verified_claims is held separately and retained opaquely.
	assert.NotContains(suite.T(), normal, VerifiedClaimsMember)
	assert.NotEmpty(suite.T(), cr.VerifiedUserInfo)

	// id_token stays typed.
	assert.True(suite.T(), cr.IDToken["auth_time"].Essential)
}

// TestMarshalUnmarshal_RoundTrip mirrors the Redis store flow: marshal a normalized request,
// then unmarshal it back, and confirm the normal claims survive.
func (suite *ClaimsRequestTestSuite) TestMarshalUnmarshal_RoundTrip() {
	original := ClaimsRequest{
		UserInfo: map[string]*IndividualClaimRequest{
			"email": {Essential: true},
			"sub":   {Value: "alice"},
		},
		VerifiedUserInfo: []*VerifiedClaimsRequest{{
			Verification: &VerificationRequest{TrustFramework: &TrustFrameworkRequest{Value: "eidas"}},
			Claims:       map[string]*IndividualClaimRequest{"given_name": nil},
		}},
	}

	data, err := json.Marshal(&original)
	assert.NoError(suite.T(), err)

	var reloaded ClaimsRequest
	err = json.Unmarshal(data, &reloaded)
	assert.NoError(suite.T(), err)

	normal := reloaded.UserInfo
	assert.Len(suite.T(), normal, 2)
	assert.True(suite.T(), normal["email"].Essential)
	assert.True(suite.T(), normal["sub"].MatchesValue("alice"))
	assert.NotEmpty(suite.T(), reloaded.VerifiedUserInfo)
}

func (suite *ClaimsRequestTestSuite) TestUnmarshalJSON_MalformedUserInfoClaim() {
	raw := `{"userinfo": {"email": "not-an-object"}}`

	var cr ClaimsRequest
	err := json.Unmarshal([]byte(raw), &cr)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "email")
}

// TestUnmarshalJSON_NormalizesIDToken verifies that a plain json.Unmarshal splits the id_token
// object into typed normal claims in IDToken and the opaque verified_claims member in
// VerifiedIDToken, mirroring the userinfo handling.
func (suite *ClaimsRequestTestSuite) TestUnmarshalJSON_NormalizesIDToken() {
	raw := `{
		"id_token": {
			"auth_time": {"essential": true},
			"acr": null,
			"verified_claims": {
				"verification": {"trust_framework": {"value": "eidas"}},
				"claims": {"given_name": null}
			}
		}
	}`

	var cr ClaimsRequest
	err := json.Unmarshal([]byte(raw), &cr)
	assert.NoError(suite.T(), err)

	normal := cr.IDToken
	assert.Len(suite.T(), normal, 2)

	assert.NotNil(suite.T(), normal["auth_time"])
	assert.True(suite.T(), normal["auth_time"].Essential)

	// "acr": null is a requested-without-constraint claim.
	val, present := normal["acr"]
	assert.True(suite.T(), present)
	assert.Nil(suite.T(), val)

	// verified_claims is held separately and retained opaquely.
	assert.NotContains(suite.T(), normal, VerifiedClaimsMember)
	assert.NotEmpty(suite.T(), cr.VerifiedIDToken)
}

// TestMarshalUnmarshal_RoundTripIDToken confirms the id_token verified_claims survives the
// marshal/unmarshal round trip used by the Redis stores.
func (suite *ClaimsRequestTestSuite) TestMarshalUnmarshal_RoundTripIDToken() {
	original := ClaimsRequest{
		IDToken: map[string]*IndividualClaimRequest{
			"auth_time": {Essential: true},
		},
		VerifiedIDToken: []*VerifiedClaimsRequest{{
			Verification: &VerificationRequest{TrustFramework: &TrustFrameworkRequest{Value: "eidas"}},
			Claims:       map[string]*IndividualClaimRequest{"given_name": nil},
		}},
	}

	data, err := json.Marshal(&original)
	assert.NoError(suite.T(), err)

	var reloaded ClaimsRequest
	err = json.Unmarshal(data, &reloaded)
	assert.NoError(suite.T(), err)

	normal := reloaded.IDToken
	assert.Len(suite.T(), normal, 1)
	assert.True(suite.T(), normal["auth_time"].Essential)
	assert.NotContains(suite.T(), normal, VerifiedClaimsMember)
	assert.NotEmpty(suite.T(), reloaded.VerifiedIDToken)
}

func (suite *ClaimsRequestTestSuite) TestUnmarshalJSON_MalformedIDTokenClaim() {
	raw := `{"id_token": {"auth_time": "not-an-object"}}`

	var cr ClaimsRequest
	err := json.Unmarshal([]byte(raw), &cr)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "auth_time")
}
