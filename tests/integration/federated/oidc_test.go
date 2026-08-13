// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federated

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"
)

/*
ID token and UserInfo handling on a generic OIDC connection.

Every rejection here surfaces as the same opaque error, so none of these assert an error code. The cause
is collapsed three times over: ValidateIDToken turns every verifier failure into
ErrorInvalidIDTokenSignature, the provider manager's default branch turns that into AUTHN-MGR-1001, and
the direct endpoint maps it again to AUTHN-FED-1001. That is G14. Each scenario is therefore distinguished
by its setup, and asserts only that authentication did not succeed.
*/

// authenticateKnown creates a local user carrying the identity's subject and then authenticates it.
//
// The local user matters: on the direct endpoint there is no provisioning, so an identity that matches
// nobody fails whatever its token contained. Without a user to resolve to, every scenario in this file
// would pass for that reason alone rather than because the token was rejected — the assertion would be
// vacuous. With one present, a valid token authenticates, so a failure here is the token's doing.
func (s *FederatedMappingSuite) authenticateKnown(user *OIDCUser) int {
	s.T().Helper()
	email := user.Sub + "@example.com"
	s.createLocalUser(map[string]interface{}{"username": email, "email": email, "sub": user.Sub})
	status, _, _ := s.authenticateDirect(mapping(fedPersonType.Name, pair("email", "email")), user)
	return status
}

// authFails asserts that an identity did not authenticate, whatever shape the failure takes. Some
// failures arrive as a non-200 finish and some as a 500 from the mapping layers (G18), and neither is
// the point of these scenarios.
func (s *FederatedMappingSuite) authFails(status int, message string, args ...interface{}) {
	s.T().Helper()
	s.NotEqual(http.StatusOK, status, append([]interface{}{message}, args...)...)
}

// B25: an expired ID token is rejected. verifyJWTClaims requires exp and checks it with the configured
// leeway.
func (s *FederatedMappingSuite) TestExpiredIDTokenRejected() {
	defer s.mockOIDC.ClearOverrides()
	s.mockOIDC.SetIDTokenOverride(func(user *OIDCUser, nonce string) (string, error) {
		return s.signedIDToken(map[string]interface{}{
			"sub":   user.Sub,
			"iss":   s.mockOIDC.GetURL(),
			"aud":   oidcClientID,
			"exp":   time.Now().Add(-1 * time.Hour).Unix(),
			"iat":   time.Now().Add(-2 * time.Hour).Unix(),
			"nonce": nonce,
		})
	})

	status := s.authenticateKnown(s.baseUser(s.nextSubject()))
	s.authFails(status, "an expired ID token must not authenticate")
}

// B26: an ID token whose signature does not verify against the published JWKS is rejected.
func (s *FederatedMappingSuite) TestInvalidIDTokenSignatureRejected() {
	defer s.mockOIDC.ClearOverrides()
	s.mockOIDC.SetIDTokenOverride(func(user *OIDCUser, nonce string) (string, error) {
		token, err := s.signedIDToken(map[string]interface{}{
			"sub":   user.Sub,
			"iss":   s.mockOIDC.GetURL(),
			"aud":   oidcClientID,
			"exp":   time.Now().Add(time.Hour).Unix(),
			"iat":   time.Now().Unix(),
			"nonce": nonce,
		})
		if err != nil {
			return "", err
		}
		// Corrupt the signature segment while leaving the token structurally valid.
		return token[:len(token)-4] + "AAAA", nil
	})

	status := s.authenticateKnown(s.baseUser(s.nextSubject()))
	s.authFails(status, "a token whose signature does not verify must not authenticate")
}

// B27: the nonce in the ID token must match the one the server generated. This is validated
// independently of signature verification, so it holds even where the signature check is skipped.
func (s *FederatedMappingSuite) TestNonceMismatchRejected() {
	defer s.mockOIDC.ClearOverrides()
	s.mockOIDC.SetIDTokenOverride(func(user *OIDCUser, nonce string) (string, error) {
		return s.signedIDToken(map[string]interface{}{
			"sub":   user.Sub,
			"iss":   s.mockOIDC.GetURL(),
			"aud":   oidcClientID,
			"exp":   time.Now().Add(time.Hour).Unix(),
			"iat":   time.Now().Unix(),
			"nonce": "a-nonce-the-server-never-issued",
		})
	})

	status := s.authenticateKnown(s.baseUser(s.nextSubject()))
	s.authFails(status, "a mismatched nonce must not authenticate")
}

// BO1: the token response carries no ID token at all.
func (s *FederatedMappingSuite) TestMissingIDTokenRejected() {
	defer s.mockOIDC.ClearOverrides()
	s.mockOIDC.SetTokenResponseOverride(func() (int, string) {
		return http.StatusOK, `{"access_token":"at","token_type":"Bearer","expires_in":3600}`
	})

	status := s.authenticateKnown(s.baseUser(s.nextSubject()))
	s.authFails(status, "a token response without an ID token must not authenticate")
}

// BO2: the ID token is not a JWT at all.
func (s *FederatedMappingSuite) TestMalformedIDTokenRejected() {
	defer s.mockOIDC.ClearOverrides()
	s.mockOIDC.SetIDTokenOverride(func(_ *OIDCUser, _ string) (string, error) {
		return "this-is-not-a-jwt", nil
	})

	status := s.authenticateKnown(s.baseUser(s.nextSubject()))
	s.authFails(status, "a structurally invalid ID token must not authenticate")
}

// BO3 and BO4: the subject is the identity. A token carrying none, or an empty one, identifies nobody.
func (s *FederatedMappingSuite) TestIDTokenWithoutUsableSubRejected() {
	for name, sub := range map[string]interface{}{"missing": nil, "empty": ""} {
		s.Run(name, func() {
			defer s.mockOIDC.ClearOverrides()
			s.mockOIDC.SetIDTokenOverride(func(_ *OIDCUser, nonce string) (string, error) {
				claims := map[string]interface{}{
					"iss":   s.mockOIDC.GetURL(),
					"aud":   oidcClientID,
					"exp":   time.Now().Add(time.Hour).Unix(),
					"iat":   time.Now().Unix(),
					"nonce": nonce,
				}
				if sub != nil {
					claims["sub"] = sub
				}
				return s.signedIDToken(claims)
			})

			status := s.authenticateKnown(s.baseUser(s.nextSubject()))
			s.authFails(status, "%s sub must not authenticate", name)
		})
	}
}

// BO7: the JWKS endpoint cannot be reached, so the signature cannot be verified.
func (s *FederatedMappingSuite) TestUnreachableJWKSRejected() {
	s.useFreshJWKS()
	defer s.mockOIDC.ClearOverrides()
	s.mockOIDC.SetJWKSOverride(func() (int, string) {
		return http.StatusServiceUnavailable, `{"error":"unavailable"}`
	})

	status := s.authenticateKnown(s.baseUser(s.nextSubject()))
	s.authFails(status, "an unreachable key set must not authenticate")
}

// BO8: the JWKS document is not valid JSON.
func (s *FederatedMappingSuite) TestMalformedJWKSRejected() {
	s.useFreshJWKS()
	defer s.mockOIDC.ClearOverrides()
	s.mockOIDC.SetJWKSOverride(func() (int, string) {
		return http.StatusOK, `{"keys": [`
	})

	status := s.authenticateKnown(s.baseUser(s.nextSubject()))
	s.authFails(status, "an unparseable key set must not authenticate")
}

// BO9: the key set is valid but contains no key matching the token's kid.
func (s *FederatedMappingSuite) TestUnknownKeyIDRejected() {
	s.useFreshJWKS()
	defer s.mockOIDC.ClearOverrides()
	s.mockOIDC.SetJWKSOverride(func() (int, string) {
		return http.StatusOK, `{"keys":[{"kty":"RSA","use":"sig","kid":"some-other-key",` +
			`"alg":"RS256","n":"AQAB","e":"AQAB"}]}`
	})

	status := s.authenticateKnown(s.baseUser(s.nextSubject()))
	s.authFails(status, "a token signed by an unpublished key must not authenticate")
}

// BO10: the token declares an algorithm the verifier does not accept.
func (s *FederatedMappingSuite) TestUnsupportedSigningAlgorithmRejected() {
	defer s.mockOIDC.ClearOverrides()
	s.mockOIDC.SetIDTokenOverride(func(user *OIDCUser, nonce string) (string, error) {
		return s.mockOIDC.SignJWT(
			map[string]interface{}{"alg": "none", "typ": "JWT"},
			map[string]interface{}{
				"sub":   user.Sub,
				"iss":   s.mockOIDC.GetURL(),
				"aud":   oidcClientID,
				"exp":   time.Now().Add(time.Hour).Unix(),
				"iat":   time.Now().Unix(),
				"nonce": nonce,
			})
	})

	status := s.authenticateKnown(s.baseUser(s.nextSubject()))
	s.authFails(status, "an unaccepted algorithm must not authenticate")
}

// rotatedKey is a signing key the mock knows nothing about, so a token signed with it verifies only if
// the verifier genuinely re-fetches and uses the newly published key set.
type rotatedKey struct {
	private *rsa.PrivateKey
	kid     string
}

func (s *FederatedMappingSuite) newRotatedKey(kid string) *rotatedKey {
	s.T().Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	s.Require().NoError(err, "failed to generate a rotation key")
	return &rotatedKey{private: key, kid: kid}
}

// jwks renders the public half as a key set the verifier can consume.
func (k *rotatedKey) jwks() string {
	n := base64.RawURLEncoding.EncodeToString(k.private.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(k.private.E)).Bytes())
	return fmt.Sprintf(
		`{"keys":[{"kty":"RSA","use":"sig","kid":%q,"alg":"RS256","n":%q,"e":%q}]}`, k.kid, n, e)
}

// sign produces an RS256 token over the given claims, carrying this key's kid.
func (k *rotatedKey) sign(claims map[string]interface{}) (string, error) {
	header, err := json.Marshal(map[string]interface{}{"alg": "RS256", "typ": "JWT", "kid": k.kid})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, k.private, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

// BO11: the provider rotates its signing key and publishes the new one. A token signed with the new key
// must verify.
//
// This is the positive half of rotation, and it is the half that proves the verifier actually re-fetches
// rather than trusting whatever it cached. An earlier version published an unrelated dummy key while the
// mock kept signing with its original one, which only re-proved BO9 — that an unpublished key is
// rejected. Here the second token is signed with a key the mock never had, so it can only succeed if the
// newly published set was fetched and used.
func (s *FederatedMappingSuite) TestKeyRotationToNewlyPublishedKey() {
	user := s.baseUser(s.nextSubject())

	// Key A: the mock's own, verified against the key set it publishes.
	status := s.authenticateKnown(user)
	s.Require().Equal(http.StatusOK, status, "the original key should verify the token")

	// Key B: generated here, published, and used to sign the next token.
	key := s.newRotatedKey("rotated-key-b")
	s.useFreshJWKS()
	defer s.mockOIDC.ClearOverrides()
	s.mockOIDC.SetJWKSOverride(func() (int, string) { return http.StatusOK, key.jwks() })
	s.mockOIDC.SetIDTokenOverride(func(u *OIDCUser, nonce string) (string, error) {
		return key.sign(map[string]interface{}{
			"sub":   u.Sub,
			"iss":   s.mockOIDC.GetURL(),
			"aud":   oidcClientID,
			"exp":   time.Now().Add(time.Hour).Unix(),
			"iat":   time.Now().Unix(),
			"nonce": nonce,
		})
	})

	rotated := s.baseUser(s.nextSubject())
	status = s.authenticateKnown(rotated)
	s.Equal(http.StatusOK, status,
		"a token signed with the newly published key should verify after rotation")
}

// BO11 (negative half): once the key set no longer contains the key a token was signed with, that token
// stops verifying. The mock still signs with its own key while a different set is published.
func (s *FederatedMappingSuite) TestKeyRotationInvalidatesOldSignature() {
	key := s.newRotatedKey("rotated-key-c")
	s.useFreshJWKS()
	defer s.mockOIDC.ClearOverrides()
	s.mockOIDC.SetJWKSOverride(func() (int, string) { return http.StatusOK, key.jwks() })

	status := s.authenticateKnown(s.baseUser(s.nextSubject()))
	s.authFails(status, "a token signed by a key that is no longer published must not authenticate")
}

// BO12: when UserInfo reports a different subject from the ID token, the whole merge is skipped rather
// than mixing two identities' claims.
//
// Proving that needs a claim only UserInfo carries. Asserting given_name survived would not: the ID token
// carries it too, so that holds equally when the merge runs and the ID token simply wins, which is BO15.
// The matching-subject half runs first as a control, because an absent claim proves nothing unless the
// same claim is known to arrive when the subjects agree — the merge is also gated on the connection
// carrying more than one scope, so without the control this would pass on a connection that never
// fetches UserInfo at all.
func (s *FederatedMappingSuite) TestUserInfoSubMismatchSkipsMerge() {
	defer s.mockOIDC.ClearOverrides()

	userInfoOnly := mapping(fedPersonType.Name,
		pair("email", "username"), pair("given_name", "firstName"), pair("locality", "city"),
	)

	matching := s.baseUser(s.nextSubject())
	s.mockOIDC.SetUserInfoResponseOverride(func() (int, string) {
		return http.StatusOK, `{"sub":"` + matching.Sub + `","locality":"Colombo"}`
	})
	attributes := s.register(userInfoOnly, matching)
	s.Require().Equal("Colombo", attributes["city"],
		"control: a UserInfo-only claim must merge when the subject matches, or the assertion below is vacuous")

	mismatched := s.baseUser(s.nextSubject())
	s.mockOIDC.SetUserInfoResponseOverride(func() (int, string) {
		return http.StatusOK,
			`{"sub":"a-different-subject","given_name":"FromUserInfo","locality":"Colombo"}`
	})
	attributes = s.register(userInfoOnly, mismatched)

	s.Equal("Federated", attributes["firstName"],
		"the ID token's claim should stand; a mismatched UserInfo must not contribute")
	s.NotContains(attributes, "city",
		"the whole merge should be skipped, so a UserInfo-only claim must not arrive either")
}

// BO14: a UserInfo request that fails does not fail the authentication. The claims already in the ID
// token are enough.
func (s *FederatedMappingSuite) TestUserInfoFailureIsTolerated() {
	user := s.baseUser(s.nextSubject())

	defer s.mockOIDC.ClearOverrides()
	s.mockOIDC.SetUserInfoResponseOverride(func() (int, string) {
		return http.StatusInternalServerError, `{"error":"boom"}`
	})

	attributes := s.register(mapping(fedPersonType.Name,
		pair("email", "username"), pair("given_name", "firstName"),
	), user)

	s.Equal("Federated", attributes["firstName"],
		"authentication should continue on the ID token's claims when UserInfo fails")
}

// BO15: where both carry the same claim, the ID token's value wins. UserInfo only fills gaps.
func (s *FederatedMappingSuite) TestIDTokenClaimWinsOverUserInfo() {
	user := s.baseUser(s.nextSubject())

	defer s.mockOIDC.ClearOverrides()
	s.mockOIDC.SetUserInfoResponseOverride(func() (int, string) {
		return http.StatusOK, `{"sub":"` + user.Sub + `","given_name":"FromUserInfo","locale":"fr"}`
	})

	attributes := s.register(mapping(fedPersonType.Name,
		pair("email", "username"), pair("given_name", "firstName"),
	), user)

	s.Equal("Federated", attributes["firstName"],
		"the ID token's given_name should win over the UserInfo one")
}
