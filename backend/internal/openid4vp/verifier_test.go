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
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testIssuer   = "https://pid.bundesdruckerei.de"
	testAudience = "x509_hash:test-verifier"
	testNonce    = "n-0S6_WzA2Mj"
	testVCT      = "urn:eudi:pid:de:1"
)

// pidBuilder assembles signed SD-JWT VC presentations for OpenID4VP verifier tests.
type pidBuilder struct {
	t         *testing.T
	issuerKey *ecdsa.PrivateKey
	holderKey *ecdsa.PrivateKey
	saltSeq   int
	vct       string
	iss       string
}

func newPIDBuilder(t *testing.T) *pidBuilder {
	t.Helper()
	issuerKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	holderKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return &pidBuilder{t: t, issuerKey: issuerKey, holderKey: holderKey, vct: testVCT, iss: testIssuer}
}

func (b *pidBuilder) nextSalt() string {
	b.saltSeq++
	return base64.RawURLEncoding.EncodeToString([]byte{byte(b.saltSeq), 0x11, 0x22, 0x33})
}

func (b *pidBuilder) disclosure(name string, value interface{}) (raw, digest string) {
	b.t.Helper()
	encoded, err := json.Marshal([]interface{}{b.nextSalt(), name, value})
	require.NoError(b.t, err)
	raw = base64.RawURLEncoding.EncodeToString(encoded)
	sum := sha256.Sum256([]byte(raw))
	return raw, base64.RawURLEncoding.EncodeToString(sum[:])
}

func (b *pidBuilder) sign(header, payload map[string]interface{}, key *ecdsa.PrivateKey) string {
	b.t.Helper()
	headerJSON, err := json.Marshal(header)
	require.NoError(b.t, err)
	payloadJSON, err := json.Marshal(payload)
	require.NoError(b.t, err)
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(payloadJSON)
	hashed := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, key, hashed[:])
	require.NoError(b.t, err)
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func (b *pidBuilder) holderJWK() map[string]interface{} {
	raw, err := b.holderKey.PublicKey.Bytes()
	require.NoError(b.t, err)
	require.Len(b.t, raw, 65)
	return map[string]interface{}{
		"kty": "EC", "crv": "P-256",
		"x": base64.RawURLEncoding.EncodeToString(raw[1:33]),
		"y": base64.RawURLEncoding.EncodeToString(raw[33:]),
	}
}

// build assembles a presentation disclosing the named claims (value "Erika"-style
// defaults) with a valid Key Binding JWT for the given nonce.
func (b *pidBuilder) build(nonce string, claims map[string]interface{}) string {
	b.t.Helper()
	digests := make([]interface{}, 0, len(claims))
	disclosures := make([]string, 0, len(claims))
	for name, value := range claims {
		raw, digest := b.disclosure(name, value)
		disclosures = append(disclosures, raw)
		digests = append(digests, digest)
	}

	issuerPayload := map[string]interface{}{
		"iss":     b.iss,
		"vct":     b.vct,
		"_sd_alg": "sha-256",
		"_sd":     digests,
		"cnf":     map[string]interface{}{"jwk": b.holderJWK()},
	}
	issuerJWT := b.sign(map[string]interface{}{"alg": "ES256", "typ": "dc+sd-jwt"}, issuerPayload, b.issuerKey)

	issued := issuerJWT + "~"
	for _, d := range disclosures {
		issued += d + "~"
	}

	sum := sha256.Sum256([]byte(issued))
	kbPayload := map[string]interface{}{
		"aud": testAudience, "nonce": nonce, "iat": time.Now().Unix(),
		"sd_hash": base64.RawURLEncoding.EncodeToString(sum[:]),
	}
	kb := b.sign(map[string]interface{}{"alg": "ES256", "typ": "kb+jwt"}, kbPayload, b.holderKey)
	return issued + kb
}

func (b *pidBuilder) trustStore() *staticTrustStore {
	return newStaticTrustStore(map[string]crypto.PublicKey{b.iss: &b.issuerKey.PublicKey})
}

func defaultPolicy() policy {
	return policy{
		ExpectedVCT:          testVCT,
		Audience:             testAudience,
		RequestedClaims:      []string{"given_name", "family_name", "birthdate"},
		MandatoryClaims:      []string{"given_name", "family_name"},
		EnforceTrustedIssuer: true,
		EnforceKeyBinding:    true,
	}
}

func newTestVerifier(t *testing.T, b *pidBuilder, policy policy) *verifier {
	t.Helper()
	v, err := newVerifier(b.trustStore(), policy)
	require.NoError(t, err)
	return v
}

func TestNewVerifierValidation(t *testing.T) {
	b := newPIDBuilder(t)

	_, err := newVerifier(nil, defaultPolicy())
	assert.ErrorIs(t, err, ErrPolicy)

	_, err = newVerifier(b.trustStore(), policy{})
	assert.ErrorIs(t, err, ErrPolicy)

	// Missing ExpectedVCT is rejected.
	_, err = newVerifier(b.trustStore(), policy{Audience: testAudience})
	assert.ErrorIs(t, err, ErrPolicy)

	v, err := newVerifier(b.trustStore(), policy{Audience: testAudience, ExpectedVCT: testVCT})
	require.NoError(t, err)
	assert.Equal(t, testVCT, v.policy.ExpectedVCT)
}

func TestVerifyHappyPath(t *testing.T) {
	b := newPIDBuilder(t)
	v := newTestVerifier(t, b, defaultPolicy())
	presentation := b.build(testNonce, map[string]interface{}{
		"given_name":  "Erika",
		"family_name": "Mustermann",
		"birthdate":   "1984-01-26",
	})

	pid, err := v.verify(context.Background(), presentation, testNonce)
	require.NoError(t, err)

	assert.Equal(t, testIssuer, pid.Issuer)
	assert.Equal(t, testVCT, pid.VCT)
	assert.Equal(t, "Erika", pid.Claims["given_name"])
	assert.Equal(t, "Mustermann", pid.Claims["family_name"])
	assert.Equal(t, "1984-01-26", pid.Claims["birthdate"])
	assert.NotContains(t, pid.Claims, "iss")
	assert.NotContains(t, pid.Claims, "vct")
	assert.NotContains(t, pid.Claims, "cnf")
	// No "sub" claim disclosed -> generic verifier leaves Subject empty.
	assert.Empty(t, pid.Subject)
	assert.ElementsMatch(t, []string{"given_name", "family_name", "birthdate"}, pid.DisclosedPaths)
}

func TestVerifyUntrustedIssuer(t *testing.T) {
	b := newPIDBuilder(t)
	// Trust store pins a different issuer URL than the credential carries.
	store := newStaticTrustStore(map[string]crypto.PublicKey{"https://other.example": &b.issuerKey.PublicKey})
	v, err := newVerifier(store, defaultPolicy())
	require.NoError(t, err)

	presentation := b.build(testNonce, map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	_, err = v.verify(context.Background(), presentation, testNonce)
	assert.ErrorIs(t, err, ErrUntrustedIssuer)
}

func TestVerifyWrongVCT(t *testing.T) {
	b := newPIDBuilder(t)
	b.vct = "urn:eudi:pid:de:2"
	v := newTestVerifier(t, b, defaultPolicy())

	presentation := b.build(testNonce, map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	_, err := v.verify(context.Background(), presentation, testNonce)
	assert.ErrorIs(t, err, ErrUnexpectedVCT)
}

func TestVerifyUnrequestedClaimRejected(t *testing.T) {
	b := newPIDBuilder(t)
	policy := defaultPolicy()
	policy.RequestedClaims = []string{"given_name", "family_name"} // birthdate not requested
	v := newTestVerifier(t, b, policy)

	presentation := b.build(testNonce, map[string]interface{}{
		"given_name":  "Erika",
		"family_name": "Mustermann",
		"birthdate":   "1984-01-26",
	})
	_, err := v.verify(context.Background(), presentation, testNonce)
	assert.ErrorIs(t, err, ErrUnrequestedClaim)
}

func TestVerifyMissingMandatoryClaim(t *testing.T) {
	b := newPIDBuilder(t)
	v := newTestVerifier(t, b, defaultPolicy()) // family_name mandatory

	presentation := b.build(testNonce, map[string]interface{}{"given_name": "Erika"})
	_, err := v.verify(context.Background(), presentation, testNonce)
	assert.ErrorIs(t, err, ErrMissingMandatoryClaim)
}

func TestVerifyWrongNonce(t *testing.T) {
	b := newPIDBuilder(t)
	v := newTestVerifier(t, b, defaultPolicy())

	presentation := b.build("issued-nonce", map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	_, err := v.verify(context.Background(), presentation, "expected-nonce")
	assert.ErrorIs(t, err, ErrInvalidPresentation)
}

func TestVerifyTamperedPresentation(t *testing.T) {
	b := newPIDBuilder(t)
	v := newTestVerifier(t, b, defaultPolicy())

	presentation := b.build(testNonce, map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	// Flip a character in the issuer JWT signature region.
	tampered := presentation[:len(presentation)-10] + "AAAAAAAAAA"
	_, err := v.verify(context.Background(), tampered, testNonce)
	assert.ErrorIs(t, err, ErrInvalidPresentation)
}

func TestVerifyMissingIss(t *testing.T) {
	b := newPIDBuilder(t)
	b.iss = ""
	store := newStaticTrustStore(map[string]crypto.PublicKey{"": &b.issuerKey.PublicKey})
	v, err := newVerifier(store, defaultPolicy())
	require.NoError(t, err)

	presentation := b.build(testNonce, map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	_, err = v.verify(context.Background(), presentation, testNonce)
	assert.ErrorIs(t, err, ErrInvalidPresentation)
}

func TestSubjectFromSubClaim(t *testing.T) {
	b := newPIDBuilder(t)
	policy := defaultPolicy()
	policy.RequestedClaims = []string{"sub", "given_name", "family_name"}
	policy.MandatoryClaims = nil
	v := newTestVerifier(t, b, policy)

	// The generic verifier sets Subject only from a credential "sub" claim;
	// credential-specific pseudonym derivation is the consumer's concern.
	presentation := b.build(testNonce, map[string]interface{}{
		"sub":         "stable-subject-123",
		"given_name":  "Erika",
		"family_name": "Mustermann",
	})
	pid, err := v.verify(context.Background(), presentation, testNonce)
	require.NoError(t, err)
	assert.Equal(t, "stable-subject-123", pid.Subject)
}

func TestFlattenNestedClaims(t *testing.T) {
	b := newPIDBuilder(t)
	policy := defaultPolicy()
	policy.RequestedClaims = []string{"address"}
	policy.MandatoryClaims = nil
	v := newTestVerifier(t, b, policy)

	presentation := b.build(testNonce, map[string]interface{}{
		"address": map[string]interface{}{"locality": "Berlin", "postal_code": "10115"},
	})
	pid, err := v.verify(context.Background(), presentation, testNonce)
	require.NoError(t, err)

	assert.Equal(t, "Berlin", pid.Claims["address.locality"])
	assert.Equal(t, "10115", pid.Claims["address.postal_code"])
}
