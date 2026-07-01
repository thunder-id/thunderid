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

package openid4vp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

type OpenID4VPDCQLTestSuite struct {
	suite.Suite
}

func TestOpenID4VPDCQLTestSuite(t *testing.T) {
	suite.Run(t, new(OpenID4VPDCQLTestSuite))
}

func (suite *OpenID4VPDCQLTestSuite) TestBuildQuery() {
	q, err := buildQuery(dcqlConfig{
		CredentialID: credentialID,
		VCT:          testVCT,
		Claims:       []string{"given_name", "family_name", "birthdate"},
	})
	suite.Require().NoError(err)

	suite.Require().Len(q.Credentials, 1)
	cred := q.Credentials[0]
	suite.Equal(credentialID, cred.ID)
	suite.Equal(FormatSDJWTVC, cred.Format)
	suite.Require().NotNil(cred.Meta)
	suite.Equal([]string{testVCT}, cred.Meta.VCTValues)

	suite.Require().Len(cred.Claims, 3)
	suite.Equal([]interface{}{"given_name"}, cred.Claims[0].Path)
	suite.Equal([]interface{}{"family_name"}, cred.Claims[1].Path)
	suite.Equal([]interface{}{"birthdate"}, cred.Claims[2].Path)
}

func (suite *OpenID4VPDCQLTestSuite) TestBuildQueryRequiresFields() {
	cases := []dcqlConfig{
		{VCT: testVCT, Claims: []string{"given_name"}},      // missing credential id
		{CredentialID: credentialID, Claims: []string{"a"}}, // missing vct
		{CredentialID: credentialID, VCT: testVCT},          // missing claims
	}
	for _, cfg := range cases {
		_, err := buildQuery(cfg)
		suite.ErrorIs(err, ErrPolicy)
	}
}

func (suite *OpenID4VPDCQLTestSuite) TestBuildQueryNestedPath() {
	q, err := buildQuery(dcqlConfig{
		CredentialID: "pid",
		VCT:          "urn:eudi:pid:de:1",
		Claims:       []string{"address.locality", "nationalities"},
	})
	suite.Require().NoError(err)

	cred := q.Credentials[0]
	suite.Equal("pid", cred.ID)
	suite.Equal([]interface{}{"address", "locality"}, cred.Claims[0].Path)
	suite.Equal([]interface{}{"nationalities"}, cred.Claims[1].Path)
}

func (suite *OpenID4VPDCQLTestSuite) TestBuildQueryMalformedPath() {
	for _, path := range []string{"address.", ".locality", "a..b"} {
		_, err := buildQuery(dcqlConfig{CredentialID: credentialID, VCT: testVCT, Claims: []string{path}})
		suite.ErrorIs(err, ErrPolicy, "path %q", path)
	}
}

func (suite *OpenID4VPDCQLTestSuite) TestBuildQueryJSONShape() {
	q, err := buildQuery(dcqlConfig{CredentialID: credentialID, VCT: testVCT, Claims: []string{"given_name"}})
	suite.Require().NoError(err)

	raw, err := json.Marshal(q)
	suite.Require().NoError(err)

	var decoded map[string]interface{}
	suite.Require().NoError(json.Unmarshal(raw, &decoded))

	credentials, ok := decoded["credentials"].([]interface{})
	suite.Require().True(ok)
	suite.Require().Len(credentials, 1)

	cred := credentials[0].(map[string]interface{})
	suite.Equal(FormatSDJWTVC, cred["format"])
	suite.Equal(credentialID, cred["id"])

	meta := cred["meta"].(map[string]interface{})
	suite.Equal([]interface{}{testVCT}, meta["vct_values"])

	// path must serialize as a JSON array, per DCQL.
	claims := cred["claims"].([]interface{})
	claim := claims[0].(map[string]interface{})
	suite.Equal([]interface{}{"given_name"}, claim["path"])

	// credential_sets is omitted for a single required credential.
	suite.NotContains(string(raw), `"credential_sets"`)
}

func (suite *OpenID4VPDCQLTestSuite) TestBuildQueryTrustedAuthorities() {
	q, err := buildQuery(dcqlConfig{
		CredentialID:           credentialID,
		VCT:                    testVCT,
		Claims:                 []string{"given_name"},
		TrustedAuthorityKeyIDs: []string{"AQIDBA", "BQYHCA"},
	})
	suite.Require().NoError(err)

	cred := q.Credentials[0]
	suite.Require().Len(cred.TrustedAuthorities, 1)
	suite.Equal("aki", cred.TrustedAuthorities[0].Type)
	suite.Equal([]string{"AQIDBA", "BQYHCA"}, cred.TrustedAuthorities[0].Values)
}

func (suite *OpenID4VPDCQLTestSuite) TestBuildQueryOmitsTrustedAuthoritiesWhenEmpty() {
	q, err := buildQuery(dcqlConfig{CredentialID: credentialID, VCT: testVCT, Claims: []string{"given_name"}})
	suite.Require().NoError(err)
	suite.Empty(q.Credentials[0].TrustedAuthorities)

	raw, err := json.Marshal(q)
	suite.Require().NoError(err)
	suite.NotContains(string(raw), `"trusted_authorities"`)
}

// TestQueryAlignsWithVerifierPolicy guards that the claim paths emitted by the
// DCQL builder match the dotted paths the verifier policy checks against, so a
// disclosed claim is never wrongly rejected as "unrequested".
func (suite *OpenID4VPDCQLTestSuite) TestQueryAlignsWithVerifierPolicy() {
	claims := []string{"given_name", "family_name", "birthdate"}
	q, err := buildQuery(dcqlConfig{CredentialID: credentialID, VCT: testVCT, Claims: claims})
	suite.Require().NoError(err)

	requested := make([]string, 0, len(q.Credentials[0].Claims))
	for _, c := range q.Credentials[0].Claims {
		parts := make([]string, len(c.Path))
		for i, seg := range c.Path {
			parts[i] = seg.(string)
		}
		requested = append(requested, joinDotted(parts))
	}
	suite.ElementsMatch(claims, requested)
}

func (suite *OpenID4VPDCQLTestSuite) TestClaimPathToSegmentsEdgeCases() {
	suite.Run("empty path", func() {
		_, err := claimPathToSegments("")
		suite.ErrorIs(err, ErrPolicy)
	})
	suite.Run("single segment", func() {
		segs, err := claimPathToSegments("given_name")
		suite.Require().NoError(err)
		suite.Equal([]interface{}{"given_name"}, segs)
	})
	suite.Run("nested segments", func() {
		segs, err := claimPathToSegments("address.locality.street")
		suite.Require().NoError(err)
		suite.Equal([]interface{}{"address", "locality", "street"}, segs)
	})
	suite.Run("trailing dot", func() {
		_, err := claimPathToSegments("address.")
		suite.ErrorIs(err, ErrPolicy)
	})
	suite.Run("leading dot", func() {
		_, err := claimPathToSegments(".locality")
		suite.ErrorIs(err, ErrPolicy)
	})
}

func joinDotted(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "."
		}
		out += p
	}
	return out
}
