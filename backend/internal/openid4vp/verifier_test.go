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
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/openid4vp/definition"
)

type OpenID4VPVerifierTestSuite struct {
	suite.Suite
}

func TestOpenID4VPVerifierTestSuite(t *testing.T) {
	suite.Run(t, new(OpenID4VPVerifierTestSuite))
}

// verifier composes a trust store and a policy; used in tests to exercise the
// SD-JWT VC verification pipeline directly.
type verifier struct {
	trust  *trustAnchorStore
	policy policy
}

func newVerifier(trust *trustAnchorStore, policy policy) (*verifier, error) {
	if trust == nil && policy.EnforceTrustedIssuer {
		return nil, fmt.Errorf("%w: trust resolver is required", ErrPolicy)
	}
	if policy.ExpectedVCT == "" {
		return nil, fmt.Errorf("%w: expected vct is required", ErrPolicy)
	}
	if policy.Audience == "" {
		return nil, fmt.Errorf("%w: audience is required", ErrPolicy)
	}
	return &verifier{trust: trust, policy: policy}, nil
}

func (v *verifier) verify(presentation, nonce string) (*VerifiedPresentation, error) {
	cred, err := verifySDJWTPresentation(
		presentation, v.trust, v.policy.Audience, nonce, v.policy.Leeway, v.policy.KeyBindingMaxAge,
		v.policy.EnforceTrustedIssuer, v.policy.EnforceKeyBinding, v.policy.TrustedAuthorities)
	if err != nil {
		return nil, err
	}
	return finalizePresentation(cred, v.policy)
}

// verifyResponse parses a decrypted OpenID4VP response body, extracts the
// presentation for credentialID, and verifies it against policy and nonce.
// The caller is responsible for decrypting the JWE (jwe.DecryptWithKey) and
// for correlating authorizationResponse.State with the issued request
// beforehand.
func (v *verifier) verifyResponse(
	body []byte, credentialID, nonce string,
) (*VerifiedPresentation, error) {
	resp, err := parseAuthorizationResponse(body)
	if err != nil {
		return nil, err
	}
	presentations, err := resp.presentationsFor(credentialID)
	if err != nil {
		return nil, err
	}
	return v.verify(presentations[0], nonce)
}

const (
	testIssuer   = "https://pid.bundesdruckerei.de"
	testAudience = "x509_hash:test-verifier"
	testNonce    = "n-0S6_WzA2Mj"
	testVCT      = "urn:eudi:pid:de:1"
)

// pidBuilder assembles signed SD-JWT VC presentations for OpenID4VP verifier tests.
type pidBuilder struct {
	t           *testing.T
	issuerKey   *ecdsa.PrivateKey
	holderKey   *ecdsa.PrivateKey
	rootCert    *x509.Certificate
	leafCertB64 string
	saltSeq     int
	vct         string
	iss         string
}

func newPIDBuilder(t *testing.T) *pidBuilder {
	t.Helper()
	issuerKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	holderKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	rootCert, rootKey := newTestRootCA(t)
	leafDER := newTestLeafCert(t, &issuerKey.PublicKey, rootCert, rootKey)
	return &pidBuilder{
		t:           t,
		issuerKey:   issuerKey,
		holderKey:   holderKey,
		rootCert:    rootCert,
		leafCertB64: base64.StdEncoding.EncodeToString(leafDER),
		vct:         testVCT,
		iss:         testIssuer,
	}
}

// newTestRootCA mints a self-signed root CA certificate (with a SubjectKeyId) and
// returns it together with its signing key.
func newTestRootCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-root-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		SubjectKeyId:          []byte{0x01, 0x02, 0x03, 0x04},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert, key
}

// newTestLeafCert mints an issuer (leaf) certificate for leafPub signed by the
// given root CA, returning its DER bytes.
func newTestLeafCert(
	t *testing.T, leafPub *ecdsa.PublicKey, root *x509.Certificate, rootKey *ecdsa.PrivateKey,
) []byte {
	t.Helper()
	tmpl := &x509.Certificate{
		SerialNumber:   big.NewInt(2),
		Subject:        pkix.Name{CommonName: "test-issuer"},
		NotBefore:      time.Now().Add(-time.Hour),
		NotAfter:       time.Now().Add(24 * time.Hour),
		KeyUsage:       x509.KeyUsageDigitalSignature,
		SubjectKeyId:   []byte{0x05, 0x06, 0x07, 0x08},
		AuthorityKeyId: root.SubjectKeyId,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, root, leafPub, rootKey)
	require.NoError(t, err)
	return der
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
	issuerHeader := map[string]interface{}{"alg": "ES256", "typ": "dc+sd-jwt"}
	if b.leafCertB64 != "" {
		issuerHeader["x5c"] = []string{b.leafCertB64}
	}
	issuerJWT := b.sign(issuerHeader, issuerPayload, b.issuerKey)

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

func (b *pidBuilder) trustStore() *trustAnchorStore {
	return newTrustAnchorStore([]*x509.Certificate{b.rootCert}, []string{"test-root"})
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

func (suite *OpenID4VPVerifierTestSuite) TestNewVerifierValidation() {
	b := newPIDBuilder(suite.T())

	_, err := newVerifier(nil, defaultPolicy())
	suite.ErrorIs(err, ErrPolicy)

	_, err = newVerifier(b.trustStore(), policy{})
	suite.ErrorIs(err, ErrPolicy)

	// Missing ExpectedVCT is rejected.
	_, err = newVerifier(b.trustStore(), policy{Audience: testAudience})
	suite.ErrorIs(err, ErrPolicy)

	v, err := newVerifier(b.trustStore(), policy{Audience: testAudience, ExpectedVCT: testVCT})
	suite.Require().NoError(err)
	suite.Equal(testVCT, v.policy.ExpectedVCT)
}

func (suite *OpenID4VPVerifierTestSuite) TestVerifyHappyPath() {
	b := newPIDBuilder(suite.T())
	v := newTestVerifier(suite.T(), b, defaultPolicy())
	presentation := b.build(testNonce, map[string]interface{}{
		"given_name":  "Erika",
		"family_name": "Mustermann",
		"birthdate":   "1984-01-26",
	})

	pid, err := v.verify(presentation, testNonce)
	suite.Require().NoError(err)

	suite.Equal(testIssuer, pid.Issuer)
	suite.Equal(testVCT, pid.VCT)
	suite.Equal("Erika", pid.Claims["given_name"])
	suite.Equal("Mustermann", pid.Claims["family_name"])
	suite.Equal("1984-01-26", pid.Claims["birthdate"])
	suite.NotContains(pid.Claims, "iss")
	suite.NotContains(pid.Claims, "vct")
	suite.NotContains(pid.Claims, "cnf")
	// No "sub" claim disclosed -> generic verifier leaves Subject empty.
	suite.Empty(pid.Subject)
	suite.ElementsMatch([]string{"given_name", "family_name", "birthdate"}, pid.DisclosedPaths)
}

func (suite *OpenID4VPVerifierTestSuite) TestVerifyUntrustedIssuer() {
	b := newPIDBuilder(suite.T())
	// Trust store roots in a DIFFERENT root CA than the one that signed the
	// credential's issuer (leaf) certificate.
	otherRoot, _ := newTestRootCA(suite.T())
	store := newTrustAnchorStore([]*x509.Certificate{otherRoot}, []string{"other-root"})
	v, err := newVerifier(store, defaultPolicy())
	suite.Require().NoError(err)

	presentation := b.build(testNonce, map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	_, err = v.verify(presentation, testNonce)
	suite.ErrorIs(err, ErrUntrustedIssuer)
}

func (suite *OpenID4VPVerifierTestSuite) TestVerifyMissingX5C() {
	b := newPIDBuilder(suite.T())
	// Drop the x5c header so the issuer JWT carries no certificate chain.
	b.leafCertB64 = ""
	v := newTestVerifier(suite.T(), b, defaultPolicy())

	presentation := b.build(testNonce, map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	_, err := v.verify(presentation, testNonce)
	suite.ErrorIs(err, ErrInvalidPresentation)
}

func (suite *OpenID4VPVerifierTestSuite) TestVerifyWrongVCT() {
	b := newPIDBuilder(suite.T())
	b.vct = "urn:eudi:pid:de:2"
	v := newTestVerifier(suite.T(), b, defaultPolicy())

	presentation := b.build(testNonce, map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	_, err := v.verify(presentation, testNonce)
	suite.ErrorIs(err, ErrUnexpectedVCT)
}

func (suite *OpenID4VPVerifierTestSuite) TestVerifyUnrequestedClaimRejected() {
	b := newPIDBuilder(suite.T())
	policy := defaultPolicy()
	policy.RequestedClaims = []string{"given_name", "family_name"} // birthdate not requested
	v := newTestVerifier(suite.T(), b, policy)

	presentation := b.build(testNonce, map[string]interface{}{
		"given_name":  "Erika",
		"family_name": "Mustermann",
		"birthdate":   "1984-01-26",
	})
	_, err := v.verify(presentation, testNonce)
	suite.ErrorIs(err, ErrUnrequestedClaim)
}

func (suite *OpenID4VPVerifierTestSuite) TestVerifyMissingMandatoryClaim() {
	b := newPIDBuilder(suite.T())
	v := newTestVerifier(suite.T(), b, defaultPolicy()) // family_name mandatory

	presentation := b.build(testNonce, map[string]interface{}{"given_name": "Erika"})
	_, err := v.verify(presentation, testNonce)
	suite.ErrorIs(err, ErrMissingMandatoryClaim)
}

func (suite *OpenID4VPVerifierTestSuite) TestVerifyWrongNonce() {
	b := newPIDBuilder(suite.T())
	v := newTestVerifier(suite.T(), b, defaultPolicy())

	presentation := b.build("issued-nonce", map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	_, err := v.verify(presentation, "expected-nonce")
	suite.ErrorIs(err, ErrInvalidPresentation)
}

func (suite *OpenID4VPVerifierTestSuite) TestVerifyTamperedPresentation() {
	b := newPIDBuilder(suite.T())
	v := newTestVerifier(suite.T(), b, defaultPolicy())

	presentation := b.build(testNonce, map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	// Flip a character in the issuer JWT signature region.
	tampered := presentation[:len(presentation)-10] + "AAAAAAAAAA"
	_, err := v.verify(tampered, testNonce)
	suite.ErrorIs(err, ErrInvalidPresentation)
}

func (suite *OpenID4VPVerifierTestSuite) TestVerifyMissingIss() {
	b := newPIDBuilder(suite.T())
	b.iss = ""
	v := newTestVerifier(suite.T(), b, defaultPolicy())

	presentation := b.build(testNonce, map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	_, err := v.verify(presentation, testNonce)
	suite.ErrorIs(err, ErrInvalidPresentation)
}

func (suite *OpenID4VPVerifierTestSuite) TestSubjectFromSubClaim() {
	b := newPIDBuilder(suite.T())
	policy := defaultPolicy()
	policy.RequestedClaims = []string{"sub", "given_name", "family_name"}
	policy.MandatoryClaims = nil
	v := newTestVerifier(suite.T(), b, policy)

	// The generic verifier sets Subject only from a credential "sub" claim;
	// credential-specific pseudonym derivation is the consumer's concern.
	presentation := b.build(testNonce, map[string]interface{}{
		"sub":         "stable-subject-123",
		"given_name":  "Erika",
		"family_name": "Mustermann",
	})
	pid, err := v.verify(presentation, testNonce)
	suite.Require().NoError(err)
	suite.Equal("stable-subject-123", pid.Subject)
}

func (suite *OpenID4VPVerifierTestSuite) TestVerifyRestrictsToNamedAuthority() {
	b := newPIDBuilder(suite.T())
	// Trust store knows the credential's root ("pid-root") plus an unrelated one.
	otherRoot, _ := newTestRootCA(suite.T())
	store := newTrustAnchorStore(
		[]*x509.Certificate{b.rootCert, otherRoot}, []string{"pid-root", "other-root"})

	// Restricting to the unrelated anchor fails closed.
	wrong := defaultPolicy()
	wrong.TrustedAuthorities = []string{"other-root"}
	v, err := newVerifier(store, wrong)
	suite.Require().NoError(err)
	presentation := b.build(testNonce, map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	_, err = v.verify(presentation, testNonce)
	suite.ErrorIs(err, ErrUntrustedIssuer)

	// Restricting to the correct anchor succeeds.
	right := defaultPolicy()
	right.TrustedAuthorities = []string{"pid-root"}
	v, err = newVerifier(store, right)
	suite.Require().NoError(err)
	presentation = b.build(testNonce, map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	_, err = v.verify(presentation, testNonce)
	suite.Require().NoError(err)
}

func (suite *OpenID4VPVerifierTestSuite) TestVerifyUnknownAuthorityFailsClosed() {
	b := newPIDBuilder(suite.T())
	p := defaultPolicy()
	// A name not in the store yields an empty pool, so verification must fail.
	p.TrustedAuthorities = []string{"does-not-exist"}
	v := newTestVerifier(suite.T(), b, p)
	presentation := b.build(testNonce, map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	_, err := v.verify(presentation, testNonce)
	suite.ErrorIs(err, ErrUntrustedIssuer)
}

func (suite *OpenID4VPVerifierTestSuite) TestDtoEnforceTrustedIssuerOverride() {
	b := newPIDBuilder(suite.T())
	store := b.trustStore()
	enabled := true
	disabled := false

	// PD disables enforcement: an untrusted (here: any) issuer is accepted because the chain is never validated.
	off := dtoToDefinition(definition.PresentationDefinitionDTO{
		Handle:               "h",
		VCT:                  testVCT,
		RequestedClaims:      []string{"given_name", "family_name"},
		EnforceTrustedIssuer: &disabled,
	}, testAudience, false)
	suite.False(off.policy.EnforceTrustedIssuer)
	v, err := newVerifier(store, off.policy)
	suite.Require().NoError(err)
	presentation := b.build(testNonce, map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	_, err = v.verify(presentation, testNonce)
	suite.Require().NoError(err)

	// PD enables enforcement: an untrusted root is rejected.
	on := dtoToDefinition(definition.PresentationDefinitionDTO{
		Handle:               "h",
		VCT:                  testVCT,
		RequestedClaims:      []string{"given_name", "family_name"},
		EnforceTrustedIssuer: &enabled,
	}, testAudience, false)
	suite.True(on.policy.EnforceTrustedIssuer)
	otherRoot, _ := newTestRootCA(suite.T())
	otherStore := newTrustAnchorStore([]*x509.Certificate{otherRoot}, []string{"other-root"})
	v, err = newVerifier(otherStore, on.policy)
	suite.Require().NoError(err)
	presentation = b.build(testNonce, map[string]interface{}{"given_name": "Erika", "family_name": "M"})
	_, err = v.verify(presentation, testNonce)
	suite.ErrorIs(err, ErrUntrustedIssuer)
}

func (suite *OpenID4VPVerifierTestSuite) TestFlattenNestedClaims() {
	b := newPIDBuilder(suite.T())
	policy := defaultPolicy()
	policy.RequestedClaims = []string{"address"}
	policy.MandatoryClaims = nil
	v := newTestVerifier(suite.T(), b, policy)

	presentation := b.build(testNonce, map[string]interface{}{
		"address": map[string]interface{}{"locality": "Berlin", "postal_code": "10115"},
	})
	pid, err := v.verify(presentation, testNonce)
	suite.Require().NoError(err)

	suite.Equal("Berlin", pid.Claims["address.locality"])
	suite.Equal("10115", pid.Claims["address.postal_code"])
}

func (suite *OpenID4VPVerifierTestSuite) TestDefaultSubjectDeriverPrefersSubClaim() {
	derive := defaultSubjectDeriver()
	got := derive(&VerifiedPresentation{
		Subject:              "stable-sub",
		Issuer:               "iss",
		KeyBindingThumbprint: "should-be-ignored",
	})
	suite.Equal("stable-sub", got)
}

// With no credential "sub", the holder key-binding thumbprint is used as a
// stable, namespaced fallback — distinct holder keys yield distinct subjects.
func (suite *OpenID4VPVerifierTestSuite) TestDefaultSubjectDeriverFallsBackToKeyBindingThumbprint() {
	derive := defaultSubjectDeriver()

	erika := derive(&VerifiedPresentation{
		Issuer:               "https://issuer.example",
		KeyBindingThumbprint: "thumb-erika",
	})
	suite.Equal(keyBindingSubjectPrefix+"thumb-erika", erika)

	max := derive(&VerifiedPresentation{
		Issuer:               "https://issuer.example",
		KeyBindingThumbprint: "thumb-max",
	})
	suite.Equal(keyBindingSubjectPrefix+"thumb-max", max)
	suite.NotEqual(erika, max, "different holder key -> different subject")
}

// With neither a "sub" claim nor a key-binding thumbprint, the subject does not resolve.
func (suite *OpenID4VPVerifierTestSuite) TestDefaultSubjectDeriverNoSubNoThumbprint() {
	derive := defaultSubjectDeriver()
	got := derive(&VerifiedPresentation{
		Issuer: "https://issuer.example",
		Claims: map[string]interface{}{"given_name": "Erika"},
	})
	suite.Empty(got)
}

// Nil input must not panic.
func (suite *OpenID4VPVerifierTestSuite) TestDefaultSubjectDeriverNilPresentation() {
	got := defaultSubjectDeriver()(nil)
	suite.Empty(got)
}
