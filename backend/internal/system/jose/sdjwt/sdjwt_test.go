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

package sdjwt

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testVCT       = "urn:eudi:pid:de:1"
	testAudience  = "x509_hash:test-verifier"
	testNonce     = "n-0S6_WzA2Mj"
	testIssuerURL = "https://pid.bundesdruckerei.de"
)

// builder assembles an SD-JWT VC PID fixture (issuer JWT + disclosures + KB-JWT)
// from which the test matrix derives valid and tampered variants.
type builder struct {
	t          *testing.T
	issuerKey  *ecdsa.PrivateKey
	holderKey  *ecdsa.PrivateKey
	saltSeq    int
	audience   string
	nonce      string
	kbIssuedAt int64
}

func newBuilder(t *testing.T) *builder {
	t.Helper()
	issuerKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	holderKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return &builder{
		t:          t,
		issuerKey:  issuerKey,
		holderKey:  holderKey,
		audience:   testAudience,
		nonce:      testNonce,
		kbIssuedAt: time.Now().Unix(),
	}
}

func (b *builder) nextSalt() string {
	b.saltSeq++
	return base64.RawURLEncoding.EncodeToString([]byte{byte(b.saltSeq), 0x11, 0x22, 0x33})
}

// disclosure builds an object-property disclosure and returns its raw form and digest.
func (b *builder) disclosure(name string, value interface{}) (raw, digest string) {
	b.t.Helper()
	arr := []interface{}{b.nextSalt(), name, value}
	encoded, err := json.Marshal(arr)
	require.NoError(b.t, err)
	raw = base64.RawURLEncoding.EncodeToString(encoded)
	sum := sha256.Sum256([]byte(raw))
	return raw, base64.RawURLEncoding.EncodeToString(sum[:])
}

func (b *builder) jwk(key *ecdsa.PublicKey) map[string]interface{} {
	// Bytes() returns the uncompressed SEC1 point: 0x04 || X(32) || Y(32) for P-256.
	raw, err := key.Bytes()
	require.NoError(b.t, err)
	require.Len(b.t, raw, 65)
	return map[string]interface{}{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(raw[1:33]),
		"y":   base64.RawURLEncoding.EncodeToString(raw[33:]),
	}
}

// sign produces a compact JWS over the given header/payload using key.
// Signatures are encoded in P1363 format (r||s) as required by RFC 7518 §3.4.
func (b *builder) sign(header, payload map[string]interface{}, key *ecdsa.PrivateKey) string {
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
	// Fixed-length P1363 encoding: each coordinate zero-padded to 32 bytes for P-256.
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// build returns a valid combined SD-JWT presentation disclosing given_name,
// family_name, and birthdate, with a Key Binding JWT.
func (b *builder) build() string {
	b.t.Helper()

	gnRaw, gnDigest := b.disclosure("given_name", "Erika")
	fnRaw, fnDigest := b.disclosure("family_name", "Mustermann")
	bdRaw, bdDigest := b.disclosure("birthdate", "1984-01-26")

	issuerPayload := map[string]interface{}{
		"iss":     testIssuerURL,
		"vct":     testVCT,
		"_sd_alg": "sha-256",
		"_sd":     []interface{}{gnDigest, fnDigest, bdDigest},
		"cnf": map[string]interface{}{
			"jwk": b.jwk(&b.holderKey.PublicKey),
		},
	}
	issuerJWT := b.sign(
		map[string]interface{}{"alg": "ES256", "typ": "dc+sd-jwt"},
		issuerPayload, b.issuerKey,
	)

	issued := issuerJWT + "~" + gnRaw + "~" + fnRaw + "~" + bdRaw + "~"
	kbJWT := b.keyBinding(issued, b.holderKey)
	return issued + kbJWT
}

// buildWithoutKeyBinding returns a valid issuance (no KB-JWT).
func (b *builder) buildWithoutKeyBinding() string {
	b.t.Helper()
	gnRaw, gnDigest := b.disclosure("given_name", "Erika")
	issuerPayload := map[string]interface{}{
		"iss":     testIssuerURL,
		"vct":     testVCT,
		"_sd_alg": "sha-256",
		"_sd":     []interface{}{gnDigest},
		"cnf":     map[string]interface{}{"jwk": b.jwk(&b.holderKey.PublicKey)},
	}
	issuerJWT := b.sign(
		map[string]interface{}{"alg": "ES256", "typ": "dc+sd-jwt"},
		issuerPayload, b.issuerKey,
	)
	return issuerJWT + "~" + gnRaw + "~"
}

// keyBinding builds a KB-JWT whose sd_hash covers the presented issuance prefix.
func (b *builder) keyBinding(presented string, key *ecdsa.PrivateKey) string {
	b.t.Helper()
	sum := sha256.Sum256([]byte(presented))
	payload := map[string]interface{}{
		"aud":     b.audience,
		"nonce":   b.nonce,
		"iat":     b.kbIssuedAt,
		"sd_hash": base64.RawURLEncoding.EncodeToString(sum[:]),
	}
	return b.sign(map[string]interface{}{"alg": "ES256", "typ": "kb+jwt"}, payload, key)
}

// b64Array encodes a JSON array to a base64url disclosure string.
func b64Array(t *testing.T, arr []interface{}) string {
	t.Helper()
	encoded, err := json.Marshal(arr)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func defaultOpts(b *builder) VerifyOptions {
	return VerifyOptions{
		IssuerKey:         &b.issuerKey.PublicKey,
		RequireKeyBinding: true,
		ExpectedAudience:  testAudience,
		ExpectedNonce:     testNonce,
		Now:               time.Now(),
	}
}

func TestParse(t *testing.T) {
	b := newBuilder(t)
	combined := b.build()

	p, err := Parse(combined)
	require.NoError(t, err)
	assert.NotEmpty(t, p.IssuerJWT)
	assert.Len(t, p.Disclosures, 3)
	assert.True(t, p.HasKeyBinding())
}

func TestParseWithoutKeyBinding(t *testing.T) {
	b := newBuilder(t)
	p, err := Parse(b.buildWithoutKeyBinding())
	require.NoError(t, err)
	assert.Len(t, p.Disclosures, 1)
	assert.False(t, p.HasKeyBinding())
}

func TestParseInvalidFormat(t *testing.T) {
	for _, in := range []string{"", "no-separators-here"} {
		_, err := Parse(in)
		assert.ErrorIs(t, err, ErrInvalidFormat, "input %q", in)
	}
}

func TestVerifyHappyPath(t *testing.T) {
	b := newBuilder(t)
	p, err := Parse(b.build())
	require.NoError(t, err)

	cred, err := Verify(p, defaultOpts(b))
	require.NoError(t, err)

	assert.Equal(t, "Erika", cred.Claims["given_name"])
	assert.Equal(t, "Mustermann", cred.Claims["family_name"])
	assert.Equal(t, "1984-01-26", cred.Claims["birthdate"])
	assert.Equal(t, testVCT, cred.Claims["vct"])
	assert.NotContains(t, cred.Claims, "_sd")
	assert.NotContains(t, cred.Claims, "_sd_alg")
	assert.ElementsMatch(t, []string{"given_name", "family_name", "birthdate"}, cred.DisclosedPaths)
	assert.NotNil(t, cred.ConfirmationKey)
}

func TestVerifyTamperedDisclosureFailsAtSelectiveDisclosure(t *testing.T) {
	b := newBuilder(t)
	p, err := Parse(b.build())
	require.NoError(t, err)

	// Re-encode a disclosure with a different value; its digest no longer
	// matches any _sd entry, so it is an unused disclosure.
	tampered := []interface{}{"tampered-salt", "given_name", "Mallory"}
	encoded, err := json.Marshal(tampered)
	require.NoError(t, err)
	p.Disclosures[0].Raw = base64.RawURLEncoding.EncodeToString(encoded)
	sum := sha256.Sum256([]byte(p.Disclosures[0].Raw))
	p.Disclosures[0].Digest = base64.RawURLEncoding.EncodeToString(sum[:])

	_, err = Verify(p, defaultOpts(b))
	assert.ErrorIs(t, err, ErrUnusedDisclosure)
}

func TestVerifyWrongNonceFailsAtHolderBinding(t *testing.T) {
	b := newBuilder(t)
	p, err := Parse(b.build())
	require.NoError(t, err)

	opts := defaultOpts(b)
	opts.ExpectedNonce = "different-nonce"

	_, err = Verify(p, opts)
	assert.ErrorIs(t, err, ErrKeyBindingClaims)
}

func TestVerifyWrongAudienceFailsAtHolderBinding(t *testing.T) {
	b := newBuilder(t)
	p, err := Parse(b.build())
	require.NoError(t, err)

	opts := defaultOpts(b)
	opts.ExpectedAudience = "x509_hash:someone-else"

	_, err = Verify(p, opts)
	assert.ErrorIs(t, err, ErrKeyBindingClaims)
}

func TestVerifyUntrustedIssuerFailsAtCredential(t *testing.T) {
	b := newBuilder(t)
	p, err := Parse(b.build())
	require.NoError(t, err)

	wrongIssuer, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	opts := defaultOpts(b)
	opts.IssuerKey = &wrongIssuer.PublicKey

	_, err = Verify(p, opts)
	assert.ErrorIs(t, err, ErrIssuerSignature)
}

func TestVerifyBadKeyBindingSignatureFailsAtHolderBinding(t *testing.T) {
	b := newBuilder(t)

	// Build a presentation whose KB-JWT is signed by an attacker key rather
	// than the credential's confirmation key.
	gnRaw, gnDigest := b.disclosure("given_name", "Erika")
	issuerPayload := map[string]interface{}{
		"iss":     testIssuerURL,
		"vct":     testVCT,
		"_sd_alg": "sha-256",
		"_sd":     []interface{}{gnDigest},
		"cnf":     map[string]interface{}{"jwk": b.jwk(&b.holderKey.PublicKey)},
	}
	issuerJWT := b.sign(map[string]interface{}{"alg": "ES256", "typ": "dc+sd-jwt"}, issuerPayload, b.issuerKey)
	issued := issuerJWT + "~" + gnRaw + "~"

	attackerKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	combined := issued + b.keyBinding(issued, attackerKey)

	p, err := Parse(combined)
	require.NoError(t, err)

	_, err = Verify(p, defaultOpts(b))
	assert.ErrorIs(t, err, ErrKeyBindingSignature)
}

func TestVerifyTamperedSDHashFailsAtHolderBinding(t *testing.T) {
	b := newBuilder(t)
	p, err := Parse(b.build())
	require.NoError(t, err)

	// Drop a disclosure after the KB-JWT was computed: the sd_hash no longer
	// matches the presented disclosures.
	p.Disclosures = p.Disclosures[:2]

	_, err = Verify(p, defaultOpts(b))
	assert.ErrorIs(t, err, ErrKeyBindingClaims)
}

func TestVerifyMissingKeyBindingWhenRequired(t *testing.T) {
	b := newBuilder(t)
	p, err := Parse(b.buildWithoutKeyBinding())
	require.NoError(t, err)

	_, err = Verify(p, defaultOpts(b))
	assert.ErrorIs(t, err, ErrMissingKeyBinding)
}

func TestVerifyFutureIatFailsAtHolderBinding(t *testing.T) {
	b := newBuilder(t)
	b.kbIssuedAt = time.Now().Add(1 * time.Hour).Unix()
	p, err := Parse(b.build())
	require.NoError(t, err)

	_, err = Verify(p, defaultOpts(b))
	assert.ErrorIs(t, err, ErrKeyBindingClaims)
}

func TestResolveOmitsUndisclosedClaims(t *testing.T) {
	b := newBuilder(t)

	// Issue three SD claims but only present one disclosure.
	gnRaw, gnDigest := b.disclosure("given_name", "Erika")
	_, fnDigest := b.disclosure("family_name", "Mustermann")
	_, bdDigest := b.disclosure("birthdate", "1984-01-26")

	issuerPayload := map[string]interface{}{
		"iss":     testIssuerURL,
		"vct":     testVCT,
		"_sd_alg": "sha-256",
		"_sd":     []interface{}{gnDigest, fnDigest, bdDigest},
		"cnf":     map[string]interface{}{"jwk": b.jwk(&b.holderKey.PublicKey)},
	}
	issuerJWT := b.sign(map[string]interface{}{"alg": "ES256", "typ": "dc+sd-jwt"}, issuerPayload, b.issuerKey)
	combined := issuerJWT + "~" + gnRaw + "~"

	p, err := Parse(combined)
	require.NoError(t, err)

	opts := defaultOpts(b)
	opts.RequireKeyBinding = false
	cred, err := Verify(p, opts)
	require.NoError(t, err)

	assert.Equal(t, "Erika", cred.Claims["given_name"])
	assert.NotContains(t, cred.Claims, "family_name")
	assert.NotContains(t, cred.Claims, "birthdate")
}

func TestResolveNestedAndArrayDisclosures(t *testing.T) {
	b := newBuilder(t)

	// Array-element disclosure for a nationality entry.
	natSalt := b.nextSalt()
	natArr := []interface{}{natSalt, "DE"}
	natEncoded, err := json.Marshal(natArr)
	require.NoError(t, err)
	natRaw := base64.RawURLEncoding.EncodeToString(natEncoded)
	natSum := sha256.Sum256([]byte(natRaw))
	natDigest := base64.RawURLEncoding.EncodeToString(natSum[:])

	// Object-property disclosure for a nested address locality.
	locRaw, locDigest := b.disclosure("locality", "Berlin")

	issuerPayload := map[string]interface{}{
		"iss":     testIssuerURL,
		"vct":     testVCT,
		"_sd_alg": "sha-256",
		"nationalities": []interface{}{
			map[string]interface{}{"...": natDigest},
		},
		"address": map[string]interface{}{
			"_sd":         []interface{}{locDigest},
			"postal_code": "10115",
		},
		"cnf": map[string]interface{}{"jwk": b.jwk(&b.holderKey.PublicKey)},
	}
	issuerJWT := b.sign(map[string]interface{}{"alg": "ES256", "typ": "dc+sd-jwt"}, issuerPayload, b.issuerKey)
	combined := issuerJWT + "~" + natRaw + "~" + locRaw + "~"

	p, err := Parse(combined)
	require.NoError(t, err)

	opts := defaultOpts(b)
	opts.RequireKeyBinding = false
	cred, err := Verify(p, opts)
	require.NoError(t, err)

	nats, ok := cred.Claims["nationalities"].([]interface{})
	require.True(t, ok)
	assert.Equal(t, []interface{}{"DE"}, nats)

	addr, ok := cred.Claims["address"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Berlin", addr["locality"])
	assert.Equal(t, "10115", addr["postal_code"])
	assert.NotContains(t, addr, "_sd")
	assert.Contains(t, cred.DisclosedPaths, "address.locality")
}

func TestVerifyIssuerSignatureNilKey(t *testing.T) {
	b := newBuilder(t)
	p, err := Parse(b.build())
	require.NoError(t, err)

	err = VerifyIssuerSignature(p, nil)
	assert.ErrorIs(t, err, ErrIssuerSignature)
}

func TestParseMalformedDisclosures(t *testing.T) {
	b := newBuilder(t)
	issuerJWT := b.sign(
		map[string]interface{}{"alg": "ES256", "typ": "dc+sd-jwt"},
		map[string]interface{}{"iss": testIssuerURL, "vct": testVCT}, b.issuerKey,
	)

	tests := map[string]string{
		"not base64url":    issuerJWT + "~!!!notb64!!!~",
		"not json array":   issuerJWT + "~" + base64.RawURLEncoding.EncodeToString([]byte(`{"a":1}`)) + "~",
		"wrong arity":      issuerJWT + "~" + b64Array(t, []interface{}{"only-one"}) + "~",
		"empty disclosure": issuerJWT + "~~",
	}
	for name, combined := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Parse(combined)
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrInvalidDisclosure) || errors.Is(err, ErrInvalidFormat))
		})
	}
}

func TestParseUnsupportedSDAlg(t *testing.T) {
	b := newBuilder(t)
	issuerJWT := b.sign(
		map[string]interface{}{"alg": "ES256", "typ": "dc+sd-jwt"},
		map[string]interface{}{"iss": testIssuerURL, "_sd_alg": "md5"}, b.issuerKey,
	)
	_, err := Parse(issuerJWT + "~")
	assert.ErrorIs(t, err, ErrUnsupportedHashAlg)
}

func TestResolveDuplicateDigestFails(t *testing.T) {
	b := newBuilder(t)
	gnRaw, gnDigest := b.disclosure("given_name", "Erika")

	// The same digest listed twice in _sd; the single provided disclosure is
	// resolved once, then the second reference trips duplicate detection.
	issuerPayload := map[string]interface{}{
		"iss":     testIssuerURL,
		"_sd_alg": "sha-256",
		"_sd":     []interface{}{gnDigest, gnDigest},
		"cnf":     map[string]interface{}{"jwk": b.jwk(&b.holderKey.PublicKey)},
	}
	issuerJWT := b.sign(map[string]interface{}{"alg": "ES256", "typ": "dc+sd-jwt"}, issuerPayload, b.issuerKey)
	p, err := Parse(issuerJWT + "~" + gnRaw + "~")
	require.NoError(t, err)

	opts := defaultOpts(b)
	opts.RequireKeyBinding = false
	_, err = Verify(p, opts)
	assert.ErrorIs(t, err, ErrDuplicateDigest)
}

func TestResolveArrayDisclosureReferencedByObjectFails(t *testing.T) {
	b := newBuilder(t)
	// An array-element disclosure (2 elements) whose digest is placed in _sd.
	natSalt := b.nextSalt()
	natRaw := b64Array(t, []interface{}{natSalt, "DE"})
	natSum := sha256.Sum256([]byte(natRaw))
	natDigest := base64.RawURLEncoding.EncodeToString(natSum[:])

	issuerPayload := map[string]interface{}{
		"iss":     testIssuerURL,
		"_sd_alg": "sha-256",
		"_sd":     []interface{}{natDigest},
	}
	issuerJWT := b.sign(map[string]interface{}{"alg": "ES256", "typ": "dc+sd-jwt"}, issuerPayload, b.issuerKey)
	p, err := Parse(issuerJWT + "~" + natRaw + "~")
	require.NoError(t, err)

	opts := defaultOpts(b)
	opts.RequireKeyBinding = false
	_, err = Verify(p, opts)
	assert.ErrorIs(t, err, ErrInvalidDisclosure)
}

func TestResolveObjectDisclosureReferencedByArrayFails(t *testing.T) {
	b := newBuilder(t)
	// An object-property disclosure (3 elements) whose digest is referenced as
	// an array element {"...": digest}.
	objRaw, objDigest := b.disclosure("given_name", "Erika")

	issuerPayload := map[string]interface{}{
		"iss":     testIssuerURL,
		"_sd_alg": "sha-256",
		"items":   []interface{}{map[string]interface{}{"...": objDigest}},
	}
	issuerJWT := b.sign(map[string]interface{}{"alg": "ES256", "typ": "dc+sd-jwt"}, issuerPayload, b.issuerKey)
	p, err := Parse(issuerJWT + "~" + objRaw + "~")
	require.NoError(t, err)

	opts := defaultOpts(b)
	opts.RequireKeyBinding = false
	_, err = Verify(p, opts)
	assert.ErrorIs(t, err, ErrInvalidDisclosure)
}

func TestResolveClaimCollisionFails(t *testing.T) {
	b := newBuilder(t)
	gnRaw, gnDigest := b.disclosure("given_name", "Erika")

	// given_name appears both as a clear claim and as a disclosed claim.
	issuerPayload := map[string]interface{}{
		"iss":        testIssuerURL,
		"_sd_alg":    "sha-256",
		"given_name": "Clear",
		"_sd":        []interface{}{gnDigest},
	}
	issuerJWT := b.sign(map[string]interface{}{"alg": "ES256", "typ": "dc+sd-jwt"}, issuerPayload, b.issuerKey)
	p, err := Parse(issuerJWT + "~" + gnRaw + "~")
	require.NoError(t, err)

	opts := defaultOpts(b)
	opts.RequireKeyBinding = false
	_, err = Verify(p, opts)
	assert.ErrorIs(t, err, ErrClaimCollision)
}

func TestVerifyKeyBindingPresentButNotRequired(t *testing.T) {
	b := newBuilder(t)
	p, err := Parse(b.build())
	require.NoError(t, err)

	// Even when RequireKeyBinding is false, a present KB-JWT is still verified.
	opts := defaultOpts(b)
	opts.RequireKeyBinding = false
	_, err = Verify(p, opts)
	assert.NoError(t, err)
}

func TestVerifyKeyBindingWrongTypFails(t *testing.T) {
	b := newBuilder(t)
	gnRaw, gnDigest := b.disclosure("given_name", "Erika")
	issuerPayload := map[string]interface{}{
		"iss":     testIssuerURL,
		"_sd_alg": "sha-256",
		"_sd":     []interface{}{gnDigest},
		"cnf":     map[string]interface{}{"jwk": b.jwk(&b.holderKey.PublicKey)},
	}
	issuerJWT := b.sign(map[string]interface{}{"alg": "ES256", "typ": "dc+sd-jwt"}, issuerPayload, b.issuerKey)
	issued := issuerJWT + "~" + gnRaw + "~"

	// KB-JWT with the wrong typ header.
	sum := sha256.Sum256([]byte(issued))
	kb := b.sign(
		map[string]interface{}{"alg": "ES256", "typ": "JWT"},
		map[string]interface{}{
			"aud": b.audience, "nonce": b.nonce, "iat": b.kbIssuedAt,
			"sd_hash": base64.RawURLEncoding.EncodeToString(sum[:]),
		}, b.holderKey,
	)
	p, err := Parse(issued + kb)
	require.NoError(t, err)

	_, err = Verify(p, defaultOpts(b))
	assert.ErrorIs(t, err, ErrKeyBindingClaims)
}

func TestVerifyKeyBindingMissingConfirmationKey(t *testing.T) {
	b := newBuilder(t)
	gnRaw, gnDigest := b.disclosure("given_name", "Erika")
	// Issuer credential without a cnf.jwk.
	issuerPayload := map[string]interface{}{
		"iss":     testIssuerURL,
		"_sd_alg": "sha-256",
		"_sd":     []interface{}{gnDigest},
	}
	issuerJWT := b.sign(map[string]interface{}{"alg": "ES256", "typ": "dc+sd-jwt"}, issuerPayload, b.issuerKey)
	issued := issuerJWT + "~" + gnRaw + "~"
	p, err := Parse(issued + b.keyBinding(issued, b.holderKey))
	require.NoError(t, err)

	_, err = Verify(p, defaultOpts(b))
	assert.ErrorIs(t, err, ErrMissingConfirmationKey)
}

func TestJWKToECDSAPublicKeyErrors(t *testing.T) {
	tests := map[string]map[string]interface{}{
		"missing coords": {"kty": "EC", "crv": "P-256"},
		"bad curve":      {"kty": "EC", "crv": "P-999", "x": "AA", "y": "BB"},
		"bad base64":     {"kty": "EC", "crv": "P-256", "x": "!!!", "y": "BB"},
	}
	for name, jwk := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := jwkToECDSAPublicKey(jwk)
			assert.Error(t, err)
		})
	}
}

func TestErrorsAreDistinct(t *testing.T) {
	// Guard against accidental error aliasing across layers.
	all := []error{
		ErrInvalidFormat, ErrInvalidDisclosure, ErrUnsupportedHashAlg,
		ErrIssuerSignature, ErrUnusedDisclosure, ErrDuplicateDigest,
		ErrClaimCollision, ErrMissingKeyBinding, ErrMissingConfirmationKey,
		ErrKeyBindingSignature, ErrKeyBindingClaims,
	}
	for i := range all {
		for j := range all {
			if i != j {
				assert.False(t, errors.Is(all[i], all[j]))
			}
		}
	}
}
