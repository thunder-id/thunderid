// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package openid4vp

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/vc/presentation"
)

type OpenID4VPTrustStoreTestSuite struct {
	suite.Suite
}

func TestOpenID4VPTrustStoreTestSuite(t *testing.T) {
	suite.Run(t, new(OpenID4VPTrustStoreTestSuite))
}

// certPEM encodes a certificate as a PEM string.
func certPEM(cert *x509.Certificate) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))
}

// leafChainTo mints a leaf certificate signed by root and returns the leaf-first
// chain (leaf only; the root is the trust anchor and is not part of x5c).
func leafChainTo(t *testing.T, root *x509.Certificate, rootKey *ecdsa.PrivateKey) []*x509.Certificate {
	t.Helper()
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:   big.NewInt(2),
		Subject:        pkix.Name{CommonName: "test-issuer"},
		NotBefore:      time.Now().Add(-time.Hour),
		NotAfter:       time.Now().Add(24 * time.Hour),
		KeyUsage:       x509.KeyUsageDigitalSignature,
		SubjectKeyId:   []byte{0x11, 0x22, 0x33, 0x44},
		AuthorityKeyId: root.SubjectKeyId,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, root, &leafKey.PublicKey, rootKey)
	require.NoError(t, err)
	leaf, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return []*x509.Certificate{leaf}
}

// caCertWithSKI mints a self-signed CA certificate carrying the given SubjectKeyId.
func caCertWithSKI(t *testing.T, ski []byte) *x509.Certificate {
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
		SubjectKeyId:          ski,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert
}

func (suite *OpenID4VPTrustStoreTestSuite) TestBuildTrustStoreVerifiesChain() {
	t := suite.T()
	root, rootKey := newTestRootCA(t)

	store, err := buildTrustStore([]presentation.TrustedAuthority{{Name: "root-a", Certificate: certPEM(root)}})
	suite.Require().NoError(err)
	suite.Require().NotNil(store)

	leaf, err := store.verifyChain(leafChainTo(t, root, rootKey), time.Now())
	suite.Require().NoError(err)
	suite.NotNil(leaf)

	otherRoot, otherKey := newTestRootCA(t)
	_, err = store.verifyChain(leafChainTo(t, otherRoot, otherKey), time.Now())
	suite.ErrorIs(err, ErrUntrustedIssuer)

	_, err = store.verifyChain(nil, time.Now())
	suite.ErrorIs(err, ErrUntrustedIssuer)
}

func (suite *OpenID4VPTrustStoreTestSuite) TestBuildTrustStoreAcceptsPinnedIssuerCertificate() {
	t := suite.T()
	root, rootKey := newTestRootCA(t)
	chain := leafChainTo(t, root, rootKey)

	store, err := buildTrustStore([]presentation.TrustedAuthority{{Name: "issuer", Certificate: certPEM(chain[0])}})
	suite.Require().NoError(err)

	leaf, err := store.verifyChain(chain, time.Now())
	suite.Require().NoError(err)
	suite.Equal(chain[0], leaf)
}

func (suite *OpenID4VPTrustStoreTestSuite) TestBuildTrustStoreWithoutAuthorities() {
	store, err := buildTrustStore(nil)
	suite.NoError(err)
	suite.Nil(store)
}

func (suite *OpenID4VPTrustStoreTestSuite) TestBuildTrustStoreRejectsInvalidCertificate() {
	_, err := buildTrustStore([]presentation.TrustedAuthority{{Name: "broken", Certificate: "not pem"}})
	suite.ErrorContains(err, "broken")
}

func (suite *OpenID4VPTrustStoreTestSuite) TestKeyIDs() {
	rootA := caCertWithSKI(suite.T(), []byte{0xaa, 0x01})
	rootB := caCertWithSKI(suite.T(), []byte{0xbb, 0x02})
	skiA := base64.RawURLEncoding.EncodeToString(rootA.SubjectKeyId)
	skiB := base64.RawURLEncoding.EncodeToString(rootB.SubjectKeyId)

	suite.Run("deduplicated in order", func() {
		store := newTrustAnchorStore([]*x509.Certificate{rootB, rootA, rootB})
		suite.Equal([]string{skiB, skiA}, store.keyIDs())
	})

	suite.Run("nil when an anchor has no SubjectKeyId", func() {
		noSKI := caCertWithSKI(suite.T(), nil)
		noSKI.SubjectKeyId = nil
		store := newTrustAnchorStore([]*x509.Certificate{rootA, noSKI})
		suite.Nil(store.keyIDs())
	})
}
