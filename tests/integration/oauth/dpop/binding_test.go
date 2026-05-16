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

// DPoP binding flows at /oauth2/par and /oauth2/token.
//
// Two flow shapes are exercised:
//   1. Token-only binding — voluntary DPoP at /oauth2/token without prior
//      auth-code binding. The issued access token MUST carry cnf.jkt and
//      token_type=DPoP.
//   2. Auth-code + token binding — the auth code is bound to a key via either
//      `dpop_jkt` at /authorize or a verified DPoP header at /par; the /token
//      exchange MUST present a proof signed by the bound key, which also
//      drives token binding on the issued access token.

package dpop

import (
	"net/http"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// ---------------------------------------------------------------------------
// Section 1 — token-only binding (no auth-code binding via dpop_jkt or PAR).
// ---------------------------------------------------------------------------

// TestTokenBinding_Algorithms — happy path across the supported alg families.
// Each case yields token_type=DPoP and cnf.jkt = sha256_thumbprint(jwk).
func (ts *DPoPTestSuite) TestTokenBinding_Algorithms() {
	for _, alg := range []string{"PS256", "EdDSA"} {
		ts.Run(alg, func() {
			key, err := testutils.GenerateDPoPKey(alg)
			ts.Require().NoError(err)

			code, verifier := ts.obtainAuthorizationCode(voluntaryClientID, nil)
			proof, err := key.CreateProof(http.MethodPost, tokenEndpoint, testutils.DPoPProofOptions{})
			ts.Require().NoError(err)

			res, err := requestTokenWithDPoP(
				voluntaryClientID, voluntaryClientSecret, code, dpopRedirectURI, verifier, proof, nil)
			ts.Require().NoError(err)
			ts.Require().Equalf(http.StatusOK, res.StatusCode, "token: %s", string(res.Body))
			ts.Require().NotNil(res.Token)
			ts.Equal("DPoP", res.Token.TokenType)

			claims, err := testutils.DecodeJWTPayloadMap(res.Token.AccessToken)
			ts.Require().NoError(err)
			cnf, _ := claims["cnf"].(map[string]any)
			ts.Require().NotNil(cnf, "access token must be DPoP-bound")
			ts.Equal(key.JKT, cnf["jkt"])
		})
	}
}

// TestTokenWithoutProof_StaysBearer — a /token call with no DPoP header
// preserves the legacy Bearer behaviour for the voluntary client.
func (ts *DPoPTestSuite) TestTokenWithoutProof_StaysBearer() {
	code, verifier := ts.obtainAuthorizationCode(voluntaryClientID, nil)

	res, err := requestTokenWithDPoP(
		voluntaryClientID, voluntaryClientSecret, code, dpopRedirectURI, verifier, "", nil)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "token: %s", string(res.Body))
	ts.Require().NotNil(res.Token)
	ts.Equal("Bearer", res.Token.TokenType)

	claims, err := testutils.DecodeJWTPayloadMap(res.Token.AccessToken)
	ts.Require().NoError(err)
	_, hasCnf := claims["cnf"]
	ts.False(hasCnf, "Bearer token must not carry cnf")
}

// TestTokenMultipleDPoPHeaders_Rejected — exactly one DPoP header is
// permitted on a request.
func (ts *DPoPTestSuite) TestTokenMultipleDPoPHeaders_Rejected() {
	key, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)
	proofA, err := key.CreateProof(http.MethodPost, tokenEndpoint, testutils.DPoPProofOptions{})
	ts.Require().NoError(err)
	proofB, err := key.CreateProof(http.MethodPost, tokenEndpoint, testutils.DPoPProofOptions{})
	ts.Require().NoError(err)

	code, verifier := ts.obtainAuthorizationCode(voluntaryClientID, nil)

	res, err := requestTokenWithDPoP(
		voluntaryClientID, voluntaryClientSecret, code, dpopRedirectURI, verifier, "",
		[]string{proofA, proofB})
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, res.StatusCode)
	ts.Require().NotNil(res.Error)
	ts.Equal("invalid_dpop_proof", res.Error.Error)
}

// TestTokenInvalidProofs_Rejected — table-driven negative cases for every
// proof-validation failure mode that should surface at /oauth2/token.
func (ts *DPoPTestSuite) TestTokenInvalidProofs_Rejected() {
	type tampered func(k *testutils.DPoPKey) (string, error)

	cases := []struct {
		name string
		make tampered
	}{
		{
			name: "wrong typ header",
			make: func(k *testutils.DPoPKey) (string, error) {
				return k.CreateProof(http.MethodPost, tokenEndpoint, testutils.DPoPProofOptions{
					Typ: "JWT",
				})
			},
		},
		{
			name: "missing typ header",
			make: func(k *testutils.DPoPKey) (string, error) {
				return k.CreateProof(http.MethodPost, tokenEndpoint, testutils.DPoPProofOptions{
					OmitTyp: true,
				})
			},
		},
		{
			name: "alg none",
			make: func(k *testutils.DPoPKey) (string, error) {
				return k.MakeUnsignedDPoPProof(http.MethodPost, tokenEndpoint)
			},
		},
		{
			name: "alg HS256 (symmetric, not allowlisted)",
			make: func(k *testutils.DPoPKey) (string, error) {
				return k.CreateProof(http.MethodPost, tokenEndpoint, testutils.DPoPProofOptions{
					Alg: "HS256",
				})
			},
		},
		{
			name: "JWK leaks private key (d member)",
			make: func(k *testutils.DPoPKey) (string, error) {
				return k.CreateProof(http.MethodPost, tokenEndpoint, testutils.DPoPProofOptions{
					IncludePrivateInJWK: true,
				})
			},
		},
		{
			name: "JWK header missing",
			make: func(k *testutils.DPoPKey) (string, error) {
				return k.CreateProof(http.MethodPost, tokenEndpoint, testutils.DPoPProofOptions{
					OmitJWK: true,
				})
			},
		},
		{
			name: "tampered signature",
			make: func(k *testutils.DPoPKey) (string, error) {
				return k.CreateProof(http.MethodPost, tokenEndpoint, testutils.DPoPProofOptions{
					TamperSignature: true,
				})
			},
		},
		{
			name: "htm mismatch",
			make: func(k *testutils.DPoPKey) (string, error) {
				return k.CreateProof("GET", tokenEndpoint, testutils.DPoPProofOptions{})
			},
		},
		{
			name: "htu mismatch",
			make: func(k *testutils.DPoPKey) (string, error) {
				return k.CreateProof(http.MethodPost,
					"https://attacker.example.com/oauth2/token", testutils.DPoPProofOptions{})
			},
		},
		{
			name: "iat too old (outside acceptance window)",
			make: func(k *testutils.DPoPKey) (string, error) {
				old := time.Now().Add(-10 * time.Minute).Unix()
				return k.CreateProof(http.MethodPost, tokenEndpoint, testutils.DPoPProofOptions{
					Iat: old,
				})
			},
		},
		{
			name: "iat in the future (beyond leeway)",
			make: func(k *testutils.DPoPKey) (string, error) {
				future := time.Now().Add(10 * time.Minute).Unix()
				return k.CreateProof(http.MethodPost, tokenEndpoint, testutils.DPoPProofOptions{
					Iat: future,
				})
			},
		},
		{
			name: "missing jti",
			make: func(k *testutils.DPoPKey) (string, error) {
				return k.CreateProof(http.MethodPost, tokenEndpoint, testutils.DPoPProofOptions{
					OmitJTI: true,
				})
			},
		},
		{
			name: "jti exceeds max length",
			make: func(k *testutils.DPoPKey) (string, error) {
				return k.CreateProof(http.MethodPost, tokenEndpoint, testutils.DPoPProofOptions{
					Jti: strings.Repeat("a", 257),
				})
			},
		},
	}

	for _, tc := range cases {
		ts.Run(tc.name, func() {
			key, err := testutils.GenerateDPoPKey("PS256")
			ts.Require().NoError(err)
			proof, err := tc.make(key)
			ts.Require().NoError(err)

			code, verifier := ts.obtainAuthorizationCode(voluntaryClientID, nil)

			res, err := requestTokenWithDPoP(
				voluntaryClientID, voluntaryClientSecret, code, dpopRedirectURI, verifier, proof, nil)
			ts.Require().NoError(err)
			ts.Equalf(http.StatusBadRequest, res.StatusCode,
				"expected 400 for %q; got %d body=%s", tc.name, res.StatusCode, string(res.Body))
			ts.Require().NotNil(res.Error, "%q: error body", tc.name)
			ts.Equal("invalid_dpop_proof", res.Error.Error, tc.name)
		})
	}
}

// TestTokenReplayedProof_Rejected — a proof reused across two token requests
// (same jti+jkt) must be rejected on the second use.
func (ts *DPoPTestSuite) TestTokenReplayedProof_Rejected() {
	key, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)
	proof, err := key.CreateProof(http.MethodPost, tokenEndpoint, testutils.DPoPProofOptions{})
	ts.Require().NoError(err)

	code1, verifier1 := ts.obtainAuthorizationCode(voluntaryClientID, nil)
	res1, err := requestTokenWithDPoP(
		voluntaryClientID, voluntaryClientSecret, code1, dpopRedirectURI, verifier1, proof, nil)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res1.StatusCode, "first use: %s", string(res1.Body))
	ts.Require().NotNil(res1.Token)
	ts.Equal("DPoP", res1.Token.TokenType)

	// Replay the same proof against a fresh authorization code. Replay
	// detection is keyed on (jkt, jti); the second exchange must fail
	// regardless of code freshness.
	code2, verifier2 := ts.obtainAuthorizationCode(voluntaryClientID, nil)
	res2, err := requestTokenWithDPoP(
		voluntaryClientID, voluntaryClientSecret, code2, dpopRedirectURI, verifier2, proof, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, res2.StatusCode)
	ts.Require().NotNil(res2.Error)
	ts.Equal("invalid_dpop_proof", res2.Error.Error)
}

// ---------------------------------------------------------------------------
// Section 2 — auth-code binding (/authorize + /par) combined with token binding.
// The /token proof both satisfies the auth-code binding and drives cnf.jkt
// on the issued access token.
// ---------------------------------------------------------------------------

// TestAuthcodeBinding_DPoPJkt_Match — happy path: dpop_jkt at /authorize
// binds the code; matching proof at /token succeeds and the access token is
// DPoP-bound to the same key.
func (ts *DPoPTestSuite) TestAuthcodeBinding_DPoPJkt_Match() {
	key, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)

	code, verifier := ts.obtainAuthorizationCode(voluntaryClientID, map[string]string{
		"dpop_jkt": key.JKT,
	})

	proof, err := key.CreateProof(http.MethodPost, tokenEndpoint, testutils.DPoPProofOptions{})
	ts.Require().NoError(err)

	res, err := requestTokenWithDPoP(
		voluntaryClientID, voluntaryClientSecret, code, dpopRedirectURI, verifier, proof, nil)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "token: %s", string(res.Body))
	ts.Require().NotNil(res.Token)
	ts.Equal("DPoP", res.Token.TokenType)

	claims, err := testutils.DecodeJWTPayloadMap(res.Token.AccessToken)
	ts.Require().NoError(err)
	cnf, _ := claims["cnf"].(map[string]any)
	ts.Require().NotNil(cnf, "access token must be DPoP-bound")
	ts.Equal(key.JKT, cnf["jkt"])
}

// TestAuthcodeBinding_DPoPJkt_Mismatch — different key at /token must fail
// with invalid_grant when the auth code carries a dpop_jkt binding.
func (ts *DPoPTestSuite) TestAuthcodeBinding_DPoPJkt_Mismatch() {
	bound, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)
	other, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)

	code, verifier := ts.obtainAuthorizationCode(voluntaryClientID, map[string]string{
		"dpop_jkt": bound.JKT,
	})

	proof, err := other.CreateProof(http.MethodPost, tokenEndpoint, testutils.DPoPProofOptions{})
	ts.Require().NoError(err)

	res, err := requestTokenWithDPoP(
		voluntaryClientID, voluntaryClientSecret, code, dpopRedirectURI, verifier, proof, nil)
	ts.Require().NoError(err)
	ts.Equalf(http.StatusBadRequest, res.StatusCode, "body: %s", string(res.Body))
	ts.Require().NotNil(res.Error)
	ts.Equal("invalid_grant", res.Error.Error)
}

// TestAuthcodeBinding_DPoPJkt_NoTokenProof — auth code carries dpop_jkt but
// the /token request omits the DPoP proof entirely → invalid_grant.
// Negative case for "auth-code binding without token binding".
func (ts *DPoPTestSuite) TestAuthcodeBinding_DPoPJkt_NoTokenProof() {
	key, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)

	code, verifier := ts.obtainAuthorizationCode(voluntaryClientID, map[string]string{
		"dpop_jkt": key.JKT,
	})

	res, err := requestTokenWithDPoP(
		voluntaryClientID, voluntaryClientSecret, code, dpopRedirectURI, verifier, "", nil)
	ts.Require().NoError(err)
	ts.Equalf(http.StatusBadRequest, res.StatusCode, "body: %s", string(res.Body))
	ts.Require().NotNil(res.Error)
	ts.Equal("invalid_grant", res.Error.Error)
}

// TestAuthcodeBinding_PAR_Match — DPoP header at /par binds the code; the
// same key at /token succeeds and the access token is bound.
func (ts *DPoPTestSuite) TestAuthcodeBinding_PAR_Match() {
	key, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)

	parProof, err := key.CreateProof(http.MethodPost, parEndpoint, testutils.DPoPProofOptions{})
	ts.Require().NoError(err)
	code, verifier := ts.obtainAuthorizationCodeViaPAR(voluntaryClientID, voluntaryClientSecret, parProof)

	tokenProof, err := key.CreateProof(http.MethodPost, tokenEndpoint, testutils.DPoPProofOptions{})
	ts.Require().NoError(err)

	res, err := requestTokenWithDPoP(
		voluntaryClientID, voluntaryClientSecret, code, dpopRedirectURI, verifier, tokenProof, nil)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "token: %s", string(res.Body))
	ts.Require().NotNil(res.Token)

	claims, err := testutils.DecodeJWTPayloadMap(res.Token.AccessToken)
	ts.Require().NoError(err)
	cnf, _ := claims["cnf"].(map[string]any)
	ts.Require().NotNil(cnf)
	ts.Equal(key.JKT, cnf["jkt"])
}

// TestAuthcodeBinding_PAR_Mismatch — different key at /token after PAR-bound
// code → invalid_grant.
func (ts *DPoPTestSuite) TestAuthcodeBinding_PAR_Mismatch() {
	bound, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)
	other, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)

	parProof, err := bound.CreateProof(http.MethodPost, parEndpoint, testutils.DPoPProofOptions{})
	ts.Require().NoError(err)
	code, verifier := ts.obtainAuthorizationCodeViaPAR(voluntaryClientID, voluntaryClientSecret, parProof)

	tokenProof, err := other.CreateProof(http.MethodPost, tokenEndpoint, testutils.DPoPProofOptions{})
	ts.Require().NoError(err)

	res, err := requestTokenWithDPoP(
		voluntaryClientID, voluntaryClientSecret, code, dpopRedirectURI, verifier, tokenProof, nil)
	ts.Require().NoError(err)
	ts.Equalf(http.StatusBadRequest, res.StatusCode, "body: %s", string(res.Body))
	ts.Require().NotNil(res.Error)
	ts.Equal("invalid_grant", res.Error.Error)
}

// TestPAR_DPoPInvalidProof_Rejected — full proof verification runs at /par.
// A bad proof must surface as invalid_dpop_proof on PAR submission.
func (ts *DPoPTestSuite) TestPAR_DPoPInvalidProof_Rejected() {
	key, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)

	bad, err := key.CreateProof(http.MethodPost, "https://attacker.example.com/oauth2/par",
		testutils.DPoPProofOptions{})
	ts.Require().NoError(err)

	res, err := submitPARWithDPoP(voluntaryClientID, voluntaryClientSecret, bad, map[string]string{
		"response_type":         "code",
		"redirect_uri":          dpopRedirectURI,
		"scope":                 "openid email",
		"state":                 "dpop-bad-par",
		"code_challenge":        testutils.GenerateCodeChallenge("verifier-that-is-at-least-43-characters-long-aaaaaa"),
		"code_challenge_method": "S256",
	})
	ts.Require().NoError(err)
	ts.Equalf(http.StatusBadRequest, res.StatusCode, "body: %s", string(res.Body))
	ts.Require().NotNil(res.Error)
	ts.Equal("invalid_dpop_proof", res.Error.Error)
}

// TestPAR_DPoPProofReplayed_Rejected — replaying the same DPoP proof at /par
// → invalid_dpop_proof on the second call.
func (ts *DPoPTestSuite) TestPAR_DPoPProofReplayed_Rejected() {
	key, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)
	proof, err := key.CreateProof(http.MethodPost, parEndpoint, testutils.DPoPProofOptions{})
	ts.Require().NoError(err)

	parParams := func(state string) map[string]string {
		return map[string]string{
			"response_type":         "code",
			"redirect_uri":          dpopRedirectURI,
			"scope":                 "openid",
			"state":                 state,
			"code_challenge":        testutils.GenerateCodeChallenge("verifier-that-is-at-least-43-characters-long-bbbbbb"),
			"code_challenge_method": "S256",
		}
	}

	first, err := submitPARWithDPoP(voluntaryClientID, voluntaryClientSecret, proof, parParams("dpop-replay-1"))
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusCreated, first.StatusCode, "first PAR body: %s", string(first.Body))

	second, err := submitPARWithDPoP(voluntaryClientID, voluntaryClientSecret, proof, parParams("dpop-replay-2"))
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, second.StatusCode)
	ts.Require().NotNil(second.Error)
	ts.Equal("invalid_dpop_proof", second.Error.Error)
}

// TestPAR_MultipleDPoPHeaders_Rejected — exactly one DPoP header at /par;
// multiple → invalid_dpop_proof.
func (ts *DPoPTestSuite) TestPAR_MultipleDPoPHeaders_Rejected() {
	key, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)
	a, err := key.CreateProof(http.MethodPost, parEndpoint, testutils.DPoPProofOptions{})
	ts.Require().NoError(err)
	b, err := key.CreateProof(http.MethodPost, parEndpoint, testutils.DPoPProofOptions{})
	ts.Require().NoError(err)

	res, err := submitPAR(voluntaryClientID, voluntaryClientSecret, "", []string{a, b}, map[string]string{
		"response_type":         "code",
		"redirect_uri":          dpopRedirectURI,
		"scope":                 "openid",
		"state":                 "dpop-multi-par",
		"code_challenge":        testutils.GenerateCodeChallenge("verifier-that-is-at-least-43-characters-long-cccccc"),
		"code_challenge_method": "S256",
	})
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, res.StatusCode)
	ts.Require().NotNil(res.Error)
	ts.Equal("invalid_dpop_proof", res.Error.Error)
}

// TestPAR_DPoPHeaderPlusDPoPJkt_Mismatch — when both the dpop_jkt
// request param and the DPoP-derived thumbprint are present at /par, they
// MUST equal.
func (ts *DPoPTestSuite) TestPAR_DPoPHeaderPlusDPoPJkt_Mismatch() {
	headerKey, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)
	otherKey, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)

	parProof, err := headerKey.CreateProof(http.MethodPost, parEndpoint, testutils.DPoPProofOptions{})
	ts.Require().NoError(err)

	res, err := submitPARWithDPoP(voluntaryClientID, voluntaryClientSecret, parProof, map[string]string{
		"response_type":         "code",
		"redirect_uri":          dpopRedirectURI,
		"scope":                 "openid",
		"state":                 "dpop-pair-mismatch",
		"code_challenge":        testutils.GenerateCodeChallenge("verifier-that-is-at-least-43-characters-long-dddddd"),
		"code_challenge_method": "S256",
		"dpop_jkt":              otherKey.JKT,
	})
	ts.Require().NoError(err)
	ts.Equalf(http.StatusBadRequest, res.StatusCode, "body: %s", string(res.Body))
	ts.Require().NotNil(res.Error)
	ts.Equal("invalid_dpop_proof", res.Error.Error)
}

// TestPAR_DPoPHeaderPlusDPoPJkt_Match — when both equal, /par accepts and
// the binding flows through to /token.
func (ts *DPoPTestSuite) TestPAR_DPoPHeaderPlusDPoPJkt_Match() {
	key, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)
	parProof, err := key.CreateProof(http.MethodPost, parEndpoint, testutils.DPoPProofOptions{})
	ts.Require().NoError(err)

	verifier, err := testutils.GenerateCodeVerifier()
	ts.Require().NoError(err)
	challenge := testutils.GenerateCodeChallenge(verifier)

	parResult, err := submitPARWithDPoP(voluntaryClientID, voluntaryClientSecret, parProof, map[string]string{
		"response_type":         "code",
		"redirect_uri":          dpopRedirectURI,
		"scope":                 "openid email",
		"state":                 "dpop-pair-match",
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
		"dpop_jkt":              key.JKT,
	})
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusCreated, parResult.StatusCode, "body: %s", string(parResult.Body))
	ts.Require().NotNil(parResult.PAR)

	noRedirect := testutils.GetNoRedirectHTTPClient()
	resp, err := noRedirect.Get(authzEndpoint + "?client_id=" + voluntaryClientID +
		"&request_uri=" + parResult.PAR.RequestURI)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode)
	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err)
	initial, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err)
	step, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": dpopTestUsername, "password": dpopTestPassword,
	}, "action_001", initial.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", step.FlowStatus)
	authzResp, err := testutils.CompleteAuthorization(authID, step.Assertion)
	ts.Require().NoError(err)
	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err)

	tokenProof, err := key.CreateProof(http.MethodPost, tokenEndpoint, testutils.DPoPProofOptions{})
	ts.Require().NoError(err)
	tokenRes, err := requestTokenWithDPoP(
		voluntaryClientID, voluntaryClientSecret, code, dpopRedirectURI, verifier, tokenProof, nil)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, tokenRes.StatusCode, "token body: %s", string(tokenRes.Body))
	ts.Require().NotNil(tokenRes.Token)
	ts.Equal("DPoP", tokenRes.Token.TokenType)
}
