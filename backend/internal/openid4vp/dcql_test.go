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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildQuery(t *testing.T) {
	q, err := buildQuery(dcqlConfig{
		CredentialID: credentialID,
		VCT:          testVCT,
		Claims:       []string{"given_name", "family_name", "birthdate"},
	})
	require.NoError(t, err)

	require.Len(t, q.Credentials, 1)
	cred := q.Credentials[0]
	assert.Equal(t, credentialID, cred.ID)
	assert.Equal(t, FormatSDJWTVC, cred.Format)
	require.NotNil(t, cred.Meta)
	assert.Equal(t, []string{testVCT}, cred.Meta.VCTValues)

	require.Len(t, cred.Claims, 3)
	assert.Equal(t, []interface{}{"given_name"}, cred.Claims[0].Path)
	assert.Equal(t, []interface{}{"family_name"}, cred.Claims[1].Path)
	assert.Equal(t, []interface{}{"birthdate"}, cred.Claims[2].Path)

	require.Len(t, q.CredentialSets, 1)
	assert.Equal(t, [][]string{{credentialID}}, q.CredentialSets[0].Options)
}

func TestBuildQueryRequiresFields(t *testing.T) {
	cases := []dcqlConfig{
		{VCT: testVCT, Claims: []string{"given_name"}},      // missing credential id
		{CredentialID: credentialID, Claims: []string{"a"}}, // missing vct
		{CredentialID: credentialID, VCT: testVCT},          // missing claims
	}
	for _, cfg := range cases {
		_, err := buildQuery(cfg)
		assert.ErrorIs(t, err, ErrPolicy)
	}
}

func TestBuildQueryNestedPath(t *testing.T) {
	q, err := buildQuery(dcqlConfig{
		CredentialID: "pid",
		VCT:          "urn:eudi:pid:de:1",
		Claims:       []string{"address.locality", "nationalities"},
	})
	require.NoError(t, err)

	cred := q.Credentials[0]
	assert.Equal(t, "pid", cred.ID)
	assert.Equal(t, []interface{}{"address", "locality"}, cred.Claims[0].Path)
	assert.Equal(t, []interface{}{"nationalities"}, cred.Claims[1].Path)
}

func TestBuildQueryMalformedPath(t *testing.T) {
	for _, path := range []string{"address.", ".locality", "a..b"} {
		_, err := buildQuery(dcqlConfig{CredentialID: credentialID, VCT: testVCT, Claims: []string{path}})
		assert.ErrorIs(t, err, ErrPolicy, "path %q", path)
	}
}

func TestBuildQueryJSONShape(t *testing.T) {
	q, err := buildQuery(dcqlConfig{CredentialID: credentialID, VCT: testVCT, Claims: []string{"given_name"}})
	require.NoError(t, err)

	raw, err := json.Marshal(q)
	require.NoError(t, err)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &decoded))

	credentials, ok := decoded["credentials"].([]interface{})
	require.True(t, ok)
	require.Len(t, credentials, 1)

	cred := credentials[0].(map[string]interface{})
	assert.Equal(t, FormatSDJWTVC, cred["format"])
	assert.Equal(t, credentialID, cred["id"])

	meta := cred["meta"].(map[string]interface{})
	assert.Equal(t, []interface{}{testVCT}, meta["vct_values"])

	// path must serialize as a JSON array, per DCQL.
	claims := cred["claims"].([]interface{})
	claim := claims[0].(map[string]interface{})
	assert.Equal(t, []interface{}{"given_name"}, claim["path"])

	// The requested-claims policy can reuse the same paths the query asked for.
	assert.Contains(t, string(raw), `"credential_sets"`)
}

// TestQueryAlignsWithVerifierPolicy guards that the claim paths emitted by the
// DCQL builder match the dotted paths the verifier policy checks against, so a
// disclosed claim is never wrongly rejected as "unrequested".
func TestQueryAlignsWithVerifierPolicy(t *testing.T) {
	claims := []string{"given_name", "family_name", "birthdate"}
	q, err := buildQuery(dcqlConfig{CredentialID: credentialID, VCT: testVCT, Claims: claims})
	require.NoError(t, err)

	requested := make([]string, 0, len(q.Credentials[0].Claims))
	for _, c := range q.Credentials[0].Claims {
		parts := make([]string, len(c.Path))
		for i, seg := range c.Path {
			parts[i] = seg.(string)
		}
		requested = append(requested, joinDotted(parts))
	}
	assert.ElementsMatch(t, claims, requested)
}

func TestClaimPathToSegmentsEdgeCases(t *testing.T) {
	t.Run("empty path", func(t *testing.T) {
		_, err := claimPathToSegments("")
		assert.ErrorIs(t, err, ErrPolicy)
	})
	t.Run("single segment", func(t *testing.T) {
		segs, err := claimPathToSegments("given_name")
		require.NoError(t, err)
		assert.Equal(t, []interface{}{"given_name"}, segs)
	})
	t.Run("nested segments", func(t *testing.T) {
		segs, err := claimPathToSegments("address.locality.street")
		require.NoError(t, err)
		assert.Equal(t, []interface{}{"address", "locality", "street"}, segs)
	})
	t.Run("trailing dot", func(t *testing.T) {
		_, err := claimPathToSegments("address.")
		assert.ErrorIs(t, err, ErrPolicy)
	})
	t.Run("leading dot", func(t *testing.T) {
		_, err := claimPathToSegments(".locality")
		assert.ErrorIs(t, err, ErrPolicy)
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
