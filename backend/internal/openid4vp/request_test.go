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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRequestConfig() requestConfig {
	return requestConfig{
		ClientID:    "x509_hash:abc123",
		ResponseURI: "https://verifier.example/openid4vp/response",
		DCQL: dcqlConfig{
			CredentialID: "pid-sd-jwt",
			VCT:          "urn:eudi:pid:de:1",
			Claims:       []string{"given_name", "family_name", "birthdate"},
		},
	}
}

func testRequestParams(t *testing.T) requestParams {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return requestParams{
		Nonce:          "nonce-abc",
		State:          "state-xyz",
		EphemeralKey:   &key.PublicKey,
		EphemeralKeyID: "enc-key-1",
		IssuedAt:       time.Unix(1_700_000_000, 0),
	}
}

func TestBuildRequestObjectHappyPath(t *testing.T) {
	params := testRequestParams(t)
	req, err := buildRequestObject(testRequestConfig(), params)
	require.NoError(t, err)

	assert.Equal(t, ResponseTypeVPToken, req["response_type"])
	assert.Equal(t, ResponseModeDirectPostJWT, req["response_mode"])
	assert.Equal(t, "x509_hash:abc123", req["client_id"])
	assert.Equal(t, "https://verifier.example/openid4vp/response", req["response_uri"])
	assert.Equal(t, "nonce-abc", req["nonce"])
	assert.Equal(t, "state-xyz", req["state"])
	assert.Equal(t, DefaultRequestAudience, req["aud"])
	assert.Equal(t, int64(1_700_000_000), req["iat"])
	assert.Equal(t, int64(1_700_000_000)+int64(defaultRequestValidity.Seconds()), req["exp"])
	assert.NotContains(t, req, "verifier_info")

	_, ok := req["dcql_query"].(*dcqlQuery)
	assert.True(t, ok)
}

func TestBuildRequestObjectClientMetadata(t *testing.T) {
	params := testRequestParams(t)
	req, err := buildRequestObject(testRequestConfig(), params)
	require.NoError(t, err)

	meta := req["client_metadata"].(map[string]interface{})
	assert.Equal(t, []string{DefaultResponseEncValue}, meta["encrypted_response_enc_values_supported"])

	formats := meta["vp_formats_supported"].(map[string]interface{})
	assert.Contains(t, formats, FormatSDJWTVC)

	jwks := meta["jwks"].(map[string]interface{})
	keys := jwks["keys"].([]interface{})
	require.Len(t, keys, 1)
	jwk := keys[0].(map[string]interface{})
	assert.Equal(t, "EC", jwk["kty"])
	assert.Equal(t, "P-256", jwk["crv"])
	assert.Equal(t, "enc", jwk["use"])
	assert.Equal(t, "ECDH-ES", jwk["alg"])
	assert.Equal(t, "enc-key-1", jwk["kid"])
	assert.NotEmpty(t, jwk["x"])
	assert.NotEmpty(t, jwk["y"])
}

func TestBuildRequestObjectEphemeralKeyMatchesInput(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	params := requestParams{
		Nonce: "n", State: "s", EphemeralKey: &key.PublicKey, EphemeralKeyID: "k",
	}
	req, err := buildRequestObject(testRequestConfig(), params)
	require.NoError(t, err)

	clientMetadata := req["client_metadata"].(map[string]interface{})
	jwks := clientMetadata["jwks"].(map[string]interface{})
	jwk := jwks["keys"].([]interface{})[0].(map[string]interface{})

	// The advertised JWK coordinates must match the EC public key the wallet
	// will encrypt to.
	raw, err := key.PublicKey.Bytes()
	require.NoError(t, err)
	require.Len(t, raw, 65)
	assert.Equal(t, base64.RawURLEncoding.EncodeToString(raw[1:33]), jwk["x"])
	assert.Equal(t, base64.RawURLEncoding.EncodeToString(raw[33:]), jwk["y"])
}

func TestBuildRequestObjectVerifierInfoAndOverrides(t *testing.T) {
	cfg := testRequestConfig()
	cfg.Audience = "https://wallet.example"
	cfg.Validity = 10 * time.Minute
	cfg.ResponseEncValues = []string{"A128GCM", "A256GCM"}
	cfg.VerifierInfo = []interface{}{map[string]interface{}{"registration": "cert"}}

	params := testRequestParams(t)
	req, err := buildRequestObject(cfg, params)
	require.NoError(t, err)

	assert.Equal(t, "https://wallet.example", req["aud"])
	assert.Equal(t, int64(1_700_000_000)+int64((10*time.Minute).Seconds()), req["exp"])
	assert.Contains(t, req, "verifier_attestations")

	meta := req["client_metadata"].(map[string]interface{})
	assert.Equal(t, []string{"A128GCM", "A256GCM"}, meta["encrypted_response_enc_values_supported"])
}

func TestBuildRequestObjectValidation(t *testing.T) {
	valid := testRequestParams(t)
	tests := map[string]struct {
		cfg    requestConfig
		params requestParams
	}{
		"missing client_id":    {requestConfig{ResponseURI: "https://x"}, valid},
		"missing response_uri": {requestConfig{ClientID: "x509_hash:x"}, valid},
		"missing nonce":        {testRequestConfig(), requestParams{State: "s", EphemeralKey: valid.EphemeralKey}},
		"missing ephemeral":    {testRequestConfig(), requestParams{Nonce: "n", State: "s"}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := buildRequestObject(tc.cfg, tc.params)
			assert.ErrorIs(t, err, ErrPolicy)
		})
	}
}

func TestEcdsaPublicKeyToEncJWKCurves(t *testing.T) {
	cases := []struct {
		name     string
		curve    elliptic.Curve
		expected string
	}{
		{"P-256", elliptic.P256(), "P-256"},
		{"P-384", elliptic.P384(), "P-384"},
		{"P-521", elliptic.P521(), "P-521"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			key, err := ecdsa.GenerateKey(c.curve, rand.Reader)
			require.NoError(t, err)
			jwk, err := ecdsaPublicKeyToEncJWK(&key.PublicKey, "kid-1")
			require.NoError(t, err)
			assert.Equal(t, "EC", jwk["kty"])
			assert.Equal(t, c.expected, jwk["crv"])
			assert.Equal(t, "kid-1", jwk["kid"])
			assert.NotEmpty(t, jwk["x"])
			assert.NotEmpty(t, jwk["y"])
		})
	}
}

func TestEcdsaPublicKeyToEncJWKOmitsEmptyKid(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	jwk, err := ecdsaPublicKeyToEncJWK(&key.PublicKey, "")
	require.NoError(t, err)
	_, ok := jwk["kid"]
	assert.False(t, ok)
}

func TestEcdsaPublicKeyToEncJWKUnsupportedCurve(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	require.NoError(t, err)
	_, err = ecdsaPublicKeyToEncJWK(&key.PublicKey, "kid")
	assert.ErrorIs(t, err, ErrPolicy)
}

func TestBuildRequestObjectSerialises(t *testing.T) {
	params := testRequestParams(t)
	req, err := buildRequestObject(testRequestConfig(), params)
	require.NoError(t, err)

	raw, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"dcql_query"`)
	assert.Contains(t, string(raw), `"response_mode":"direct_post.jwt"`)
}
