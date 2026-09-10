// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package jwks

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const jwksEndpoint = testutils.TestServerURL + "/oauth2/jwks"

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	Crv string `json:"crv"`
}

type JWKSTestSuite struct {
	suite.Suite
	client *http.Client
}

func TestJWKSTestSuite(t *testing.T) {
	suite.Run(t, new(JWKSTestSuite))
}

func (ts *JWKSTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()
}

func (ts *JWKSTestSuite) fetchKeys() []jwk {
	resp, err := ts.client.Get(jwksEndpoint)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var body struct {
		Keys []jwk `json:"keys"`
	}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&body))
	ts.Require().NotEmpty(body.Keys)
	return body.Keys
}

// TestJWKS_EveryKeyCarriesKidAndAlg verifies the fix: each published JWK must
// carry its own key ID and the signing algorithm of the underlying key.
func (ts *JWKSTestSuite) TestJWKS_EveryKeyCarriesKidAndAlg() {
	for _, k := range ts.fetchKeys() {
		ts.NotEmpty(k.Kid, "JWK for %s must carry a kid", k.Kty)
		ts.NotEmpty(k.Alg, "JWK for %s must carry an alg", k.Kty)
		ts.Equal("sig", k.Use, "JWK for %s must be marked for signing", k.Kty)
	}
}

// TestJWKS_PublishesEveryConfiguredKeyType verifies the RSA, ECDSA, EdDSA and
// ML-DSA signing keys configured for the test server are all serialized.
func (ts *JWKSTestSuite) TestJWKS_PublishesEveryConfiguredKeyType() {
	ktyByAlg := map[string]string{}
	kids := map[string]bool{}
	for _, k := range ts.fetchKeys() {
		ktyByAlg[k.Alg] = k.Kty
		kids[k.Kid] = true
	}

	ts.Equal("RSA", ktyByAlg["RS256"], "RSA key should be published as RS256")
	ts.Equal("EC", ktyByAlg["ES256"], "ECDSA key should be published as ES256")
	ts.Equal("OKP", ktyByAlg["EdDSA"], "EdDSA key should be published as an OKP JWK")
	ts.Equal("AKP", ktyByAlg["ML-DSA-65"], "ML-DSA key should be published as an AKP JWK")

	ts.Len(kids, 4, "each configured key should publish under a distinct kid")
}
