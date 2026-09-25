// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// M2MOUScopedTokenTestSuite covers the accessing-organization-unit form of the token endpoint,
// /ou/{ouId}/oauth2/token, against the declaratively defined M2M service applications in
// resources/declarative_resources/applications/m2m-*.yaml.
//
// The applications are owned by decl-m2m-root and shared outward three different ways: every
// organization unit in the deployment, the owning subtree, and the owning subtree with one branch
// carved out. That is what lets a single credential pair serve many organizations while remaining
// invisible inside them.
type M2MOUScopedTokenTestSuite struct {
	suite.Suite
}

func TestM2MOUScopedTokenTestSuite(t *testing.T) {
	suite.Run(t, new(M2MOUScopedTokenTestSuite))
}

const (
	m2mRootOUID   = "decl-m2m-root"
	m2mChildAOUID = "decl-m2m-child-a"
	m2mChildBOUID = "decl-m2m-child-b"

	m2mAllOUsClientID     = "decl-m2m-all-ous-client"
	m2mAllOUsSecret       = "decl-m2m-all-ous-secret"
	m2mSubtreeClientID    = "decl-m2m-subtree-client"
	m2mSubtreeSecret      = "decl-m2m-subtree-secret"
	m2mCarvedOutClientID  = "decl-m2m-carved-out-client"
	m2mCarvedOutSecret    = "decl-m2m-carved-out-secret"
	unrelatedDeclOUHandle = "decl-ou-1"
)

// requestToken issues a client_credentials request. When ouID is non-empty the request goes to the
// /ou/{ouId} form of the endpoint, otherwise to the bare one.
func (suite *M2MOUScopedTokenTestSuite) requestToken(
	ouID, clientID, clientSecret string,
) (int, map[string]interface{}) {
	suite.T().Helper()

	form := url.Values{}
	form.Set("grant_type", "client_credentials")

	endpoint := testutils.TestServerURL + "/oauth2/token"
	if ouID != "" {
		endpoint = testutils.TestServerURL + "/ou/" + ouID + "/oauth2/token"
	}

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(form.Encode()))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	// Raw client: this test sets its own Authorization header (client_secret_basic), so the
	// harness must not inject an admin bearer over it.
	resp, err := testutils.GetRawHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	parsed := map[string]interface{}{}
	if len(body) > 0 {
		_ = json.Unmarshal(body, &parsed)
	}
	return resp.StatusCode, parsed
}

// TestBareEndpointStillWorks pins the backwards-compatibility guarantee: a request that does not
// name an accessing organization unit behaves exactly as it did before the prefix existed, with no
// policy required.
func (suite *M2MOUScopedTokenTestSuite) TestBareEndpointStillWorks() {
	status, body := suite.requestToken("", m2mCarvedOutClientID, m2mCarvedOutSecret)

	suite.Equal(http.StatusOK, status, "body: %v", body)
	suite.NotEmpty(body["access_token"])
}

// TestAllOUsPolicyReachesEveryOU proves an allOus policy lets the application be used for any
// organization unit, including ones in an unrelated tree.
func (suite *M2MOUScopedTokenTestSuite) TestAllOUsPolicyReachesEveryOU() {
	for _, ouID := range []string{m2mRootOUID, m2mChildAOUID, m2mChildBOUID, unrelatedDeclOUHandle} {
		suite.Run(ouID, func() {
			status, body := suite.requestToken(ouID, m2mAllOUsClientID, m2mAllOUsSecret)

			suite.Equal(http.StatusOK, status, "body: %v", body)
			suite.NotEmpty(body["access_token"])
		})
	}
}

// TestOwnerMayAlwaysNameItself proves the owning organization unit needs no policy of its own.
func (suite *M2MOUScopedTokenTestSuite) TestOwnerMayAlwaysNameItself() {
	status, body := suite.requestToken(m2mRootOUID, m2mCarvedOutClientID, m2mCarvedOutSecret)

	suite.Equal(http.StatusOK, status, "body: %v", body)
	suite.NotEmpty(body["access_token"])
}

// TestSubtreePolicyCoversChildren proves an allChildren policy reaches the owning root's descendants.
func (suite *M2MOUScopedTokenTestSuite) TestSubtreePolicyCoversChildren() {
	for _, ouID := range []string{m2mChildAOUID, m2mChildBOUID} {
		suite.Run(ouID, func() {
			status, body := suite.requestToken(ouID, m2mSubtreeClientID, m2mSubtreeSecret)

			suite.Equal(http.StatusOK, status, "body: %v", body)
			suite.NotEmpty(body["access_token"])
		})
	}
}

// TestSubtreePolicyDoesNotReachAnotherTree proves allChildren stays inside the owning subtree.
func (suite *M2MOUScopedTokenTestSuite) TestSubtreePolicyDoesNotReachAnotherTree() {
	status, body := suite.requestToken(unrelatedDeclOUHandle, m2mSubtreeClientID, m2mSubtreeSecret)

	suite.Equal(http.StatusBadRequest, status, "body: %v", body)
	suite.Equal("invalid_request", body["error"])
}

// TestCarveOutExcludesOneBranch is the case the carve-out exists to express: the whole subtree is
// reached except child B, which must be refused even though it sits beside a sibling that is not.
func (suite *M2MOUScopedTokenTestSuite) TestCarveOutExcludesOneBranch() {
	reachedStatus, reachedBody := suite.requestToken(m2mChildAOUID, m2mCarvedOutClientID, m2mCarvedOutSecret)
	suite.Equal(http.StatusOK, reachedStatus, "body: %v", reachedBody)
	suite.NotEmpty(reachedBody["access_token"])

	refusedStatus, refusedBody := suite.requestToken(m2mChildBOUID, m2mCarvedOutClientID, m2mCarvedOutSecret)
	suite.Equal(http.StatusBadRequest, refusedStatus, "body: %v", refusedBody)
	suite.Equal("invalid_request", refusedBody["error"],
		"a carved-out organization unit is refused exactly as an unreached one is")
}

// TestUnknownOUIsRefused proves an organization unit id that resolves to nothing is refused, under a
// blanket allOus policy just as much as a narrower one. allOus means "every organization unit in the
// deployment", and a policy check alone cannot reject an id that names none of them, so the accessing
// organization unit is resolved before any policy is consulted.
func (suite *M2MOUScopedTokenTestSuite) TestUnknownOUIsRefused() {
	for _, tc := range []struct{ name, clientID, secret string }{
		{"carved-out policy", m2mCarvedOutClientID, m2mCarvedOutSecret},
		{"allOus policy", m2mAllOUsClientID, m2mAllOUsSecret},
	} {
		suite.Run(tc.name, func() {
			status, body := suite.requestToken("no-such-ou", tc.clientID, tc.secret)

			suite.Equal(http.StatusBadRequest, status, "body: %v", body)
			suite.Equal("invalid_request", body["error"])
		})
	}
}

// decodeAccessTokenClaims returns the JWT payload claims of an issued access token.
func (suite *M2MOUScopedTokenTestSuite) decodeAccessTokenClaims(body map[string]interface{}) map[string]interface{} {
	suite.T().Helper()

	token, ok := body["access_token"].(string)
	suite.Require().True(ok, "no access_token in body: %v", body)
	parts := strings.Split(token, ".")
	suite.Require().Len(parts, 3, "access token is not a JWS")

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	suite.Require().NoError(err)
	claims := map[string]interface{}{}
	suite.Require().NoError(json.Unmarshal(payload, &claims))
	return claims
}

// TestClaimsNameTheAccessingOU proves the token states which organization it was issued for: on an
// /ou/{ouId} request the ouId/ouHandle claims resolve to the accessing organization unit, not to the
// application's own owner. Without the prefix they fall back to the owner, unchanged.
func (suite *M2MOUScopedTokenTestSuite) TestClaimsNameTheAccessingOU() {
	_, accessing := suite.requestToken(m2mChildAOUID, m2mAllOUsClientID, m2mAllOUsSecret)
	accessingClaims := suite.decodeAccessTokenClaims(accessing)
	suite.Equal(m2mChildAOUID, accessingClaims["ouId"],
		"the token must name the organization it was requested against")
	suite.Equal(m2mChildAOUID, accessingClaims["ouHandle"])

	_, bare := suite.requestToken("", m2mAllOUsClientID, m2mAllOUsSecret)
	bareClaims := suite.decodeAccessTokenClaims(bare)
	suite.Equal(m2mRootOUID, bareClaims["ouId"],
		"without an accessing organization unit the claims stay with the application's owner")
}

// TestUnknownAndUnreachedOUsAreIndistinguishable is the property the refusal wording exists for.
//
// This check runs before client credentials are verified, so anyone can probe it. If an organization
// unit that does not exist answered differently from one the client simply may not act for, the
// endpoint would be an unauthenticated oracle for which organization units exist.
func (suite *M2MOUScopedTokenTestSuite) TestUnknownAndUnreachedOUsAreIndistinguishable() {
	// The subtree policy reaches neither: one is in another tree, the other names nothing at all.
	unreachedStatus, unreached := suite.requestToken(
		unrelatedDeclOUHandle, m2mSubtreeClientID, m2mSubtreeSecret)
	unknownStatus, unknown := suite.requestToken("no-such-ou", m2mSubtreeClientID, m2mSubtreeSecret)

	suite.Equal(unreachedStatus, unknownStatus)
	suite.Equal(unreached["error"], unknown["error"])
	// Only the organization unit each request named may differ, and that is the caller's own input.
	suite.Equal(
		strings.Replace(unreached["error_description"].(string), unrelatedDeclOUHandle, "OU", 1),
		strings.Replace(unknown["error_description"].(string), "no-such-ou", "OU", 1),
		"the two refusals must differ only in the id the caller supplied")
}

// TestWrongSecretFailsAsInvalidClient proves client authentication is not bypassed by the
// organization unit logic: a bad secret is refused as invalid_client regardless of how broadly the
// application is shared, and stays distinguishable from the invalid_request an organization unit
// the client may not act for gets.
func (suite *M2MOUScopedTokenTestSuite) TestWrongSecretFailsAsInvalidClient() {
	status, body := suite.requestToken(m2mChildAOUID, m2mAllOUsClientID, "definitely-not-the-secret")

	suite.Equal(http.StatusUnauthorized, status, "body: %v", body)
	suite.Equal("invalid_client", body["error"])
}
