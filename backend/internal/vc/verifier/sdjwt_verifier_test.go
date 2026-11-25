// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package verifier

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/jose/sdjwt"
	"github.com/thunder-id/thunderid/internal/vc/truststore"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	testVCT      = "urn:eudi:pid:de:1"
	testIssuer   = "https://issuer.example"
	testAudience = "x509_hash:test-verifier"
	testNonce    = "n-0S6_WzA2Mj"
)

type SDJWTVerifierTestSuite struct {
	suite.Suite
}

func TestSDJWTVerifierTestSuite(t *testing.T) {
	suite.Run(t, new(SDJWTVerifierTestSuite))
}

// credentialBuilder mints SD-JWT VCs signed by an issuer certificate chaining to rootCert.
type credentialBuilder struct {
	t          *testing.T
	rootCert   *x509.Certificate
	issuerKey  *ecdsa.PrivateKey
	issuerCert *x509.Certificate
	holderKey  *ecdsa.PrivateKey
}

func newCredentialBuilder(t *testing.T) *credentialBuilder {
	t.Helper()
	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		SubjectKeyId:          []byte{1, 2, 3, 4},
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTmpl, rootTmpl, &rootKey.PublicKey, rootKey)
	require.NoError(t, err)
	rootCert, err := x509.ParseCertificate(rootDER)
	require.NoError(t, err)

	issuerKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	issuerTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test-issuer"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(10 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	issuerDER, err := x509.CreateCertificate(rand.Reader, issuerTmpl, rootCert, &issuerKey.PublicKey, rootKey)
	require.NoError(t, err)
	issuerCert, err := x509.ParseCertificate(issuerDER)
	require.NoError(t, err)

	holderKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	return &credentialBuilder{
		t: t, rootCert: rootCert, issuerKey: issuerKey, issuerCert: issuerCert, holderKey: holderKey,
	}
}

// trustStore returns a trust store holding rootCert under the name "test-root".
func (b *credentialBuilder) trustStore() truststore.TrustStoreInterface {
	b.t.Helper()
	path := filepath.Join(b.t.TempDir(), "root.pem")
	require.NoError(b.t, os.WriteFile(path,
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: b.rootCert.Raw}), 0o600))
	store, err := truststore.Initialize([]engineconfig.TrustedAnchorEntry{{Name: "test-root", CertFile: path}}, "")
	require.NoError(b.t, err)
	return store
}

// issue creates an SD-JWT VC with the given issuer header, without a key binding JWT.
func (b *credentialBuilder) issue(header map[string]interface{}) string {
	b.t.Helper()
	raw, err := b.holderKey.PublicKey.Bytes()
	require.NoError(b.t, err)
	cnf := map[string]interface{}{
		"kty": "EC", "crv": "P-256",
		"x": base64.RawURLEncoding.EncodeToString(raw[1:33]),
		"y": base64.RawURLEncoding.EncodeToString(raw[33:]),
	}
	combined, _, err := sdjwt.Issue(sdjwt.IssueParams{
		Header:          header,
		Issuer:          testIssuer,
		VCT:             testVCT,
		IssuedAt:        time.Now(),
		ExpiresAt:       time.Now().Add(time.Hour),
		SelectiveClaims: map[string]interface{}{"given_name": "Erika", "family_name": "Mustermann"},
		ConfirmationJWK: cnf,
	}, func(signingInput string) ([]byte, error) {
		return signES256(b.issuerKey, signingInput)
	})
	require.NoError(b.t, err)
	return combined
}

// defaultHeader returns an issuer header carrying the issuer certificate as x5c.
func (b *credentialBuilder) defaultHeader() map[string]interface{} {
	return map[string]interface{}{
		"alg": "ES256", "typ": "dc+sd-jwt",
		"x5c": []string{base64.StdEncoding.EncodeToString(b.issuerCert.Raw)},
	}
}

// build creates an SD-JWT VC presentation with a key binding JWT over testNonce.
func (b *credentialBuilder) build() string {
	b.t.Helper()
	combined := b.issue(b.defaultHeader())
	sdHash := sha256.Sum256([]byte(combined))
	kbHeader, err := json.Marshal(map[string]interface{}{"alg": "ES256", "typ": "kb+jwt"})
	require.NoError(b.t, err)
	kbPayload, err := json.Marshal(map[string]interface{}{
		"aud":     testAudience,
		"nonce":   testNonce,
		"iat":     time.Now().Unix(),
		"sd_hash": base64.RawURLEncoding.EncodeToString(sdHash[:]),
	})
	require.NoError(b.t, err)
	kbInput := base64.RawURLEncoding.EncodeToString(kbHeader) + "." +
		base64.RawURLEncoding.EncodeToString(kbPayload)
	sig, err := signES256(b.holderKey, kbInput)
	require.NoError(b.t, err)
	return combined + kbInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// signES256 returns the raw R||S ES256 signature over input.
func signES256(key *ecdsa.PrivateKey, input string) ([]byte, error) {
	h := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, key, h[:])
	if err != nil {
		return nil, err
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return sig, nil
}

func defaultParams() providers.VCVerifierParams {
	return providers.VCVerifierParams{
		CredentialQuery:      providers.DCQLCredentialQuery{Meta: &providers.DCQLMeta{VCTValues: []string{testVCT}}},
		Nonce:                testNonce,
		Audience:             testAudience,
		EnforceTrustedIssuer: true,
		EnforceKeyBinding:    true,
	}
}

func (suite *SDJWTVerifierTestSuite) TestVerifyHappyPath() {
	b := newCredentialBuilder(suite.T())
	v := newSDJWTVCVerifier(b.trustStore())

	cred, err := v.Verify(context.Background(), b.build(), defaultParams())
	suite.Require().NoError(err)
	suite.Equal(FormatSDJWTVC, cred.Format)
	suite.Equal(testIssuer, cred.Issuer)
	suite.Equal([]string{testVCT}, cred.Type)
	suite.Equal("Erika", cred.Claims["given_name"])
	suite.ElementsMatch([]string{"given_name", "family_name"}, cred.DisclosedPaths)
	suite.NotEmpty(cred.KeyBindingThumbprint)
}

func (suite *SDJWTVerifierTestSuite) TestVerifyAllowedTrustedAuthority() {
	b := newCredentialBuilder(suite.T())
	v := newSDJWTVCVerifier(b.trustStore())

	params := defaultParams()
	params.TrustedAuthorities = map[string]*x509.Certificate{"test-root-ski": b.rootCert}
	_, err := v.Verify(context.Background(), b.build(), params)
	suite.NoError(err)
}

func (suite *SDJWTVerifierTestSuite) TestVerifyFailures() {
	b := newCredentialBuilder(suite.T())
	v := newSDJWTVCVerifier(b.trustStore())

	cases := []struct {
		name    string
		cred    func() string
		params  func(p *providers.VCVerifierParams)
		wantErr error
	}{
		{
			name:    "malformed credential",
			cred:    func() string { return "not-an-sd-jwt" },
			wantErr: ErrInvalidCredential,
		},
		{
			name:    "wrong nonce",
			params:  func(p *providers.VCVerifierParams) { p.Nonce = "other-nonce" },
			wantErr: ErrInvalidCredential,
		},
		{
			name: "unexpected vct",
			params: func(p *providers.VCVerifierParams) {
				p.CredentialQuery.Meta = &providers.DCQLMeta{VCTValues: []string{"urn:other"}}
			},
			wantErr: ErrUnexpectedCredentialType,
		},
		{
			name: "missing x5c",
			cred: func() string {
				return b.issue(map[string]interface{}{"alg": "ES256", "typ": "dc+sd-jwt"})
			},
			wantErr: ErrInvalidCredential,
		},
		{
			name: "malformed x5c entry",
			cred: func() string {
				return b.issue(map[string]interface{}{"alg": "ES256", "typ": "dc+sd-jwt", "x5c": []int{1}})
			},
			wantErr: ErrInvalidCredential,
		},
		{
			name: "x5c not base64",
			cred: func() string {
				return b.issue(map[string]interface{}{"alg": "ES256", "typ": "dc+sd-jwt", "x5c": []string{"%%%"}})
			},
			wantErr: ErrInvalidCredential,
		},
		{
			name: "x5c not a certificate",
			cred: func() string {
				return b.issue(map[string]interface{}{
					"alg": "ES256", "typ": "dc+sd-jwt",
					"x5c": []string{base64.StdEncoding.EncodeToString([]byte("junk"))},
				})
			},
			wantErr: ErrInvalidCredential,
		},
		{
			name: "trusted authority not allowed",
			params: func(p *providers.VCVerifierParams) {
				p.TrustedAuthorities = map[string]*x509.Certificate{
					"other-root-ski": newCredentialBuilder(suite.T()).rootCert,
				}
			},
			wantErr: truststore.ErrUntrustedIssuer,
		},
		{
			name:    "key binding required but absent",
			cred:    func() string { return b.issue(b.defaultHeader()) },
			wantErr: ErrInvalidCredential,
		},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			cred := b.build()
			if tc.cred != nil {
				cred = tc.cred()
			}
			params := defaultParams()
			if tc.params != nil {
				tc.params(&params)
			}
			_, err := v.Verify(context.Background(), cred, params)
			suite.ErrorIs(err, tc.wantErr)
		})
	}
}

func (suite *SDJWTVerifierTestSuite) TestVerifyUntrustedIssuer() {
	b := newCredentialBuilder(suite.T())
	other := newCredentialBuilder(suite.T())
	v := newSDJWTVCVerifier(other.trustStore())

	_, err := v.Verify(context.Background(), b.build(), defaultParams())
	suite.ErrorIs(err, truststore.ErrUntrustedIssuer)

	// Without trust enforcement the x5c leaf key is still used to check the signature.
	params := defaultParams()
	params.EnforceTrustedIssuer = false
	_, err = v.Verify(context.Background(), b.build(), params)
	suite.NoError(err)
}

func (suite *SDJWTVerifierTestSuite) TestVerifyOptionalKeyBinding() {
	b := newCredentialBuilder(suite.T())
	v := newSDJWTVCVerifier(b.trustStore())

	params := defaultParams()
	params.EnforceKeyBinding = false
	params.Nonce = ""
	cred, err := v.Verify(context.Background(), b.issue(b.defaultHeader()), params)
	suite.Require().NoError(err)
	suite.Equal(testIssuer, cred.Issuer)
}

func (suite *SDJWTVerifierTestSuite) TestVerifyNoVCTValuesSkipsTypeCheck() {
	b := newCredentialBuilder(suite.T())
	v := newSDJWTVCVerifier(b.trustStore())

	params := defaultParams()
	params.CredentialQuery.Meta = nil
	cred, err := v.Verify(context.Background(), b.build(), params)
	suite.Require().NoError(err)
	suite.Equal([]string{testVCT}, cred.Type)
}

func (suite *SDJWTVerifierTestSuite) TestInitializeRegistersSDJWTVerifier() {
	b := newCredentialBuilder(suite.T())
	registry := Initialize(b.trustStore())

	got, err := registry.GetVerifier(FormatSDJWTVC)
	suite.Require().NoError(err)
	suite.IsType(&sdjwtVCVerifier{}, got)
}

// unsignedSDJWT assembles an SD-JWT with the given header and payload segments and a dummy signature.
func unsignedSDJWT(t *testing.T, header string, payload map[string]interface{}) string {
	t.Helper()
	payloadJSON, err := json.Marshal(payload)
	require.NoError(t, err)
	return header + "." + base64.RawURLEncoding.EncodeToString(payloadJSON) + ".c2ln~"
}

func (suite *SDJWTVerifierTestSuite) TestVerifyIssuerJWTFailures() {
	b := newCredentialBuilder(suite.T())
	v := newSDJWTVCVerifier(b.trustStore())
	headerJSON, err := json.Marshal(b.defaultHeader())
	suite.Require().NoError(err)
	validHeader := base64.RawURLEncoding.EncodeToString(headerJSON)

	forged := *b
	forgedKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	suite.Require().NoError(err)
	forged.issuerKey = forgedKey

	withDuplicateDisclosure := func() string {
		combined := b.issue(b.defaultHeader())
		disclosure := strings.Split(combined, "~")[1]
		return combined + disclosure + "~"
	}

	for _, tc := range []struct {
		name string
		cred string
	}{
		{"missing iss", unsignedSDJWT(suite.T(), validHeader, map[string]interface{}{"vct": testVCT})},
		{"undecodable issuer header", unsignedSDJWT(suite.T(), "%%%", map[string]interface{}{"iss": testIssuer})},
		{"issuer signature mismatch", forged.issue(b.defaultHeader())},
		{"duplicate disclosure", withDuplicateDisclosure()},
	} {
		suite.Run(tc.name, func() {
			params := defaultParams()
			params.EnforceKeyBinding = false
			_, err := v.Verify(context.Background(), tc.cred, params)
			suite.ErrorIs(err, ErrInvalidCredential)
		})
	}
}
