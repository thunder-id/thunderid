// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package openid4vp

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/vc/verifier"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
	"github.com/thunder-id/thunderid/tests/mocks/vc/vcverifierclientconfigprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/vc/verifiermock"
)

const testSigningKeyID = "signing-key"

type OpenID4VPInitTestSuite struct {
	suite.Suite
}

func TestOpenID4VPInitTestSuite(t *testing.T) {
	suite.Run(t, new(OpenID4VPInitTestSuite))
}

// newTestSigningCertDER returns a self-signed ECDSA certificate carrying dnsNames as SAN entries.
func newTestSigningCertDER(t *testing.T, dnsNames ...string) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-verifier"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     dnsNames,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return der
}

// newInitCryptoProvider returns a crypto provider mock that serves keys for the signing key lookup.
func newInitCryptoProvider(
	t *testing.T, keys []providers.PublicKeyInfo, err error, algs ...string,
) *cryptomock.RuntimeCryptoProviderMock {
	t.Helper()
	m := cryptomock.NewRuntimeCryptoProviderMock(t)
	m.EXPECT().GetPublicKeys(mock.Anything, providers.PublicKeyFilter{KeyID: testSigningKeyID}).
		Return(keys, err).Maybe()
	m.EXPECT().GetSupportedSigningAlgorithms().Return(algs).Maybe()
	return m
}

func (suite *OpenID4VPInitTestSuite) initialize(
	mux *http.ServeMux, cryptoProvider providers.RuntimeCryptoProvider, withRegistry bool,
) (OpenID4VPServiceInterface, error) {
	configProvider := vcverifierclientconfigprovidermock.NewVCVerifierClientConfigProviderMock(suite.T())
	var registry verifier.VCVerifierRegistryInterface
	if withRegistry {
		registry = verifiermock.NewVCVerifierRegistryInterfaceMock(suite.T())
	}
	return Initialize(mux, "https://verifier.example/", "", cryptoProvider, nil,
		engineconfig.OpenID4VPConfig{SigningKeyID: testSigningKeyID}, configProvider, nil, registry)
}

// Initialize fails when no signing key is configured.
func (suite *OpenID4VPInitTestSuite) TestInitializeRequiresSigningKeyID() {
	svc, err := Initialize(http.NewServeMux(), "https://verifier.example", "", nil, nil,
		engineconfig.OpenID4VPConfig{}, nil, nil, nil)
	suite.Require().Error(err)
	suite.ErrorIs(err, ErrPolicy)
	suite.Nil(svc)
}

func (suite *OpenID4VPInitTestSuite) TestInitializeSigningKeyErrors() {
	certDER := newTestSigningCertDER(suite.T(), "verifier.example")
	for _, tc := range []struct {
		name     string
		keys     []providers.PublicKeyInfo
		err      error
		algs     []string
		isPolicy bool
	}{
		{name: "key lookup fails", err: errors.New("boom")},
		{name: "no key found", isPolicy: true},
		{
			name:     "unsupported algorithm",
			keys:     []providers.PublicKeyInfo{{KeyID: testSigningKeyID, Algorithm: "HS256", CertificateDER: certDER}},
			algs:     []string{"ES256"},
			isPolicy: true,
		},
		{
			name:     "not certificate backed",
			keys:     []providers.PublicKeyInfo{{KeyID: testSigningKeyID, Algorithm: "ES256"}},
			algs:     []string{"ES256"},
			isPolicy: true,
		},
	} {
		suite.Run(tc.name, func() {
			provider := newInitCryptoProvider(suite.T(), tc.keys, tc.err, tc.algs...)
			svc, err := suite.initialize(http.NewServeMux(), provider, true)
			suite.Require().Error(err)
			suite.Nil(svc)
			if tc.isPolicy {
				suite.ErrorIs(err, ErrPolicy)
			} else {
				suite.ErrorContains(err, "failed to load signing key")
			}
		})
	}
}

// Initialize surfaces service construction failures, such as a missing verifier registry.
func (suite *OpenID4VPInitTestSuite) TestInitializeServiceConstructionFails() {
	certDER := newTestSigningCertDER(suite.T(), "verifier.example")
	provider := newInitCryptoProvider(suite.T(),
		[]providers.PublicKeyInfo{{KeyID: testSigningKeyID, Algorithm: "ES256", CertificateDER: certDER}},
		nil, "ES256")
	svc, err := suite.initialize(http.NewServeMux(), provider, false)
	suite.Require().ErrorIs(err, ErrPolicy)
	suite.Nil(svc)
}

func (suite *OpenID4VPInitTestSuite) TestInitializeSuccess() {
	certDER := newTestSigningCertDER(suite.T(), "verifier.example")
	for _, tc := range []struct {
		name  string
		chain [][]byte
	}{
		{name: "leaf only falls back to certificate"},
		{name: "explicit chain", chain: [][]byte{certDER, certDER}},
	} {
		suite.Run(tc.name, func() {
			provider := newInitCryptoProvider(suite.T(), []providers.PublicKeyInfo{{
				KeyID: testSigningKeyID, Algorithm: "ES256", CertificateDER: certDER,
				CertificateChainDER: tc.chain,
			}}, nil, "ES256")
			mux := http.NewServeMux()
			svc, err := suite.initialize(mux, provider, true)
			suite.Require().NoError(err)
			suite.Require().NotNil(svc)

			impl, ok := svc.(*openid4vpService)
			suite.Require().True(ok)
			suite.Equal("https://verifier.example"+requestURIPath, impl.requestURL)
			suite.Equal("https://verifier.example"+responseURIPath, impl.responseURL)
			suite.Equal("https://verifier.example", impl.issuerURL)
			suite.Equal("ES256", impl.signingAlg)
			expectedLen := len(tc.chain)
			if expectedLen == 0 {
				expectedLen = 1
			}
			suite.Len(impl.x5c, expectedLen)

			for _, path := range []string{requestURIPath, responseURIPath} {
				req := httptest.NewRequest(http.MethodOptions, path, nil)
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, req)
				suite.Equal(http.StatusNoContent, rec.Code, path)
			}
		})
	}
}
