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

// Resource-side validation at /oauth2/userinfo and /oauth2/introspect.
// At the userinfo endpoint:
//   * DPoP-bound token under the DPoP scheme MUST verify the proof (htm/htu/ath/jkt).
//   * DPoP-bound token under the Bearer scheme MUST be rejected with WWW-Authenticate: DPoP.
//   * Plain bearer behaviour is unchanged.
// At /oauth2/introspect, a DPoP-bound token MUST surface cnf.jkt and report
// token_type=DPoP.

package dpop

import (
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// issueDPoPBoundAccessToken returns a freshly issued DPoP-bound access token
// signed by the supplied key. Used as a precondition for the resource-side tests.
func (ts *DPoPTestSuite) issueDPoPBoundAccessToken(key *testutils.DPoPKey) string {
	code, verifier := ts.obtainAuthorizationCode(voluntaryClientID, nil)
	proof, err := key.CreateProof(http.MethodPost, tokenEndpoint, testutils.DPoPProofOptions{})
	ts.Require().NoError(err)
	res, err := requestTokenWithDPoP(
		voluntaryClientID, voluntaryClientSecret, code, dpopRedirectURI, verifier, proof, nil)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "issue DPoP token: %s", string(res.Body))
	ts.Require().NotNil(res.Token)
	ts.Require().Equal("DPoP", res.Token.TokenType)
	return res.Token.AccessToken
}

// issuePlainBearerAccessToken issues an unbound bearer access token (no DPoP
// header on the /token request).
func (ts *DPoPTestSuite) issuePlainBearerAccessToken() string {
	code, verifier := ts.obtainAuthorizationCode(voluntaryClientID, nil)
	res, err := requestTokenWithDPoP(
		voluntaryClientID, voluntaryClientSecret, code, dpopRedirectURI, verifier, "", nil)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "issue bearer: %s", string(res.Body))
	ts.Require().NotNil(res.Token)
	ts.Require().Equal("Bearer", res.Token.TokenType)
	return res.Token.AccessToken
}

// TestUserInfo_DPoP_Success — bound token + valid proof + correct ath
// → 200 with userinfo body.
func (ts *DPoPTestSuite) TestUserInfo_DPoP_Success() {
	key, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)
	token := ts.issueDPoPBoundAccessToken(key)

	proof, err := key.CreateProof(http.MethodGet, userInfoEndpoint, testutils.DPoPProofOptions{
		AccessToken: token,
	})
	ts.Require().NoError(err)

	res, err := callUserInfoDPoP(token, []string{proof})
	ts.Require().NoError(err)
	ts.Equalf(http.StatusOK, res.StatusCode, "userinfo body: %s", string(res.Body))
	ts.Require().NotNil(res.JSON)
	ts.NotEmpty(res.JSON["sub"], "userinfo must include sub")
}

// TestUserInfo_BearerOnBoundToken_Downgrade — a DPoP-bound access token
// presented under the Bearer scheme must be rejected with
// WWW-Authenticate: DPoP and 401.
func (ts *DPoPTestSuite) TestUserInfo_BearerOnBoundToken_Downgrade() {
	key, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)
	token := ts.issueDPoPBoundAccessToken(key)

	res, err := callUserInfoBearer(token)
	ts.Require().NoError(err)
	ts.Equal(http.StatusUnauthorized, res.StatusCode)
	ts.True(strings.HasPrefix(res.WWWAuthenticate, "DPoP"),
		"bearer-on-bound-token must surface WWW-Authenticate: DPoP, got %q", res.WWWAuthenticate)
}

// TestUserInfo_DPoPSchemeOnBearerToken_Rejected — a non-bound token
// presented under the DPoP scheme must fail (the token does not carry cnf.jkt).
func (ts *DPoPTestSuite) TestUserInfo_DPoPSchemeOnBearerToken_Rejected() {
	key, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)
	bearer := ts.issuePlainBearerAccessToken()

	proof, err := key.CreateProof(http.MethodGet, userInfoEndpoint, testutils.DPoPProofOptions{
		AccessToken: bearer,
	})
	ts.Require().NoError(err)

	res, err := callUserInfoDPoP(bearer, []string{proof})
	ts.Require().NoError(err)
	ts.Equal(http.StatusUnauthorized, res.StatusCode)
	ts.True(strings.HasPrefix(res.WWWAuthenticate, "DPoP"),
		"non-bound token under DPoP scheme must surface WWW-Authenticate: DPoP, got %q", res.WWWAuthenticate)
}

// TestUserInfo_DPoP_MissingDPoPHeader — DPoP scheme with no DPoP
// header → 401.
func (ts *DPoPTestSuite) TestUserInfo_DPoP_MissingDPoPHeader() {
	key, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)
	token := ts.issueDPoPBoundAccessToken(key)

	res, err := callUserInfoDPoP(token, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusUnauthorized, res.StatusCode)
}

// TestUserInfo_DPoP_MultipleHeaders — multiple DPoP headers at /userinfo → 401.
func (ts *DPoPTestSuite) TestUserInfo_DPoP_MultipleHeaders() {
	key, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)
	token := ts.issueDPoPBoundAccessToken(key)

	a, err := key.CreateProof(http.MethodGet, userInfoEndpoint, testutils.DPoPProofOptions{
		AccessToken: token,
	})
	ts.Require().NoError(err)
	b, err := key.CreateProof(http.MethodGet, userInfoEndpoint, testutils.DPoPProofOptions{
		AccessToken: token,
	})
	ts.Require().NoError(err)

	res, err := callUserInfoDPoP(token, []string{a, b})
	ts.Require().NoError(err)
	ts.Equal(http.StatusUnauthorized, res.StatusCode)
}

// TestUserInfo_DPoP_JktMismatch — proof signed by a different key
// than the token's cnf.jkt → 401.
func (ts *DPoPTestSuite) TestUserInfo_DPoP_JktMismatch() {
	bound, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)
	other, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)
	token := ts.issueDPoPBoundAccessToken(bound)

	proof, err := other.CreateProof(http.MethodGet, userInfoEndpoint, testutils.DPoPProofOptions{
		AccessToken: token,
	})
	ts.Require().NoError(err)

	res, err := callUserInfoDPoP(token, []string{proof})
	ts.Require().NoError(err)
	ts.Equal(http.StatusUnauthorized, res.StatusCode)
	ts.True(strings.HasPrefix(res.WWWAuthenticate, "DPoP"),
		"jkt mismatch must surface WWW-Authenticate: DPoP, got %q", res.WWWAuthenticate)
}

// TestUserInfo_DPoP_AthMismatch — proof's ath does not equal
// SHA-256 of the access token → 401.
func (ts *DPoPTestSuite) TestUserInfo_DPoP_AthMismatch() {
	key, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)
	token := ts.issueDPoPBoundAccessToken(key)

	proof, err := key.CreateProof(http.MethodGet, userInfoEndpoint, testutils.DPoPProofOptions{
		AthOverride: testutils.AthFor("a different access token"),
	})
	ts.Require().NoError(err)

	res, err := callUserInfoDPoP(token, []string{proof})
	ts.Require().NoError(err)
	ts.Equal(http.StatusUnauthorized, res.StatusCode)
}

// TestUserInfo_DPoP_MissingAth — proof missing the ath claim → 401.
func (ts *DPoPTestSuite) TestUserInfo_DPoP_MissingAth() {
	key, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)
	token := ts.issueDPoPBoundAccessToken(key)

	proof, err := key.CreateProof(http.MethodGet, userInfoEndpoint, testutils.DPoPProofOptions{
		OmitAth: true,
	})
	ts.Require().NoError(err)

	res, err := callUserInfoDPoP(token, []string{proof})
	ts.Require().NoError(err)
	ts.Equal(http.StatusUnauthorized, res.StatusCode)
}

// TestUserInfo_DPoP_HtmHtuMismatch — proof bound to /token (POST) but
// presented to /userinfo (GET) → 401.
func (ts *DPoPTestSuite) TestUserInfo_DPoP_HtmHtuMismatch() {
	key, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)
	token := ts.issueDPoPBoundAccessToken(key)

	wrong, err := key.CreateProof(http.MethodPost, tokenEndpoint, testutils.DPoPProofOptions{
		AccessToken: token,
	})
	ts.Require().NoError(err)

	res, err := callUserInfoDPoP(token, []string{wrong})
	ts.Require().NoError(err)
	ts.Equal(http.StatusUnauthorized, res.StatusCode)
}

// TestBearer_PlainToken_Unchanged — regression: a plain bearer token
// at /userinfo under Bearer scheme still works.
func (ts *DPoPTestSuite) TestBearer_PlainToken_Unchanged() {
	bearer := ts.issuePlainBearerAccessToken()
	res, err := callUserInfoBearer(bearer)
	ts.Require().NoError(err)
	ts.Equalf(http.StatusOK, res.StatusCode, "userinfo body: %s", string(res.Body))
	ts.Require().NotNil(res.JSON)
	ts.NotEmpty(res.JSON["sub"])
}

// TestIntrospect_DPoPBoundToken_Returns_Cnf — the introspection endpoint
// returns cnf.jkt and reports token_type=DPoP for a DPoP-bound token.
func (ts *DPoPTestSuite) TestIntrospect_DPoPBoundToken_Returns_Cnf() {
	key, err := testutils.GenerateDPoPKey("PS256")
	ts.Require().NoError(err)
	token := ts.issueDPoPBoundAccessToken(key)

	res, err := introspectToken(voluntaryClientID, voluntaryClientSecret, token)
	ts.Require().NoError(err)
	ts.True(res.Active, "introspection should mark DPoP token active")
	ts.Equal("DPoP", res.TokenType, "introspection token_type must be DPoP")
	ts.Require().NotNil(res.Cnf, "introspection must surface cnf claim")
	ts.Equal(key.JKT, res.Cnf["jkt"])
}

// TestIntrospect_BearerToken_NoCnf — bearer token introspection must
// keep token_type=Bearer and not invent a cnf claim.
func (ts *DPoPTestSuite) TestIntrospect_BearerToken_NoCnf() {
	bearer := ts.issuePlainBearerAccessToken()

	res, err := introspectToken(voluntaryClientID, voluntaryClientSecret, bearer)
	ts.Require().NoError(err)
	ts.True(res.Active)
	ts.Equal("Bearer", res.TokenType)
	ts.Nil(res.Cnf, "Bearer introspection must not include cnf")
}
