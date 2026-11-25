// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package truststore

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

type TrustStoreTestSuite struct {
	suite.Suite
}

func TestTrustStoreTestSuite(t *testing.T) {
	suite.Run(t, new(TrustStoreTestSuite))
}

// newRootCA mints a self-signed root CA carrying the given SubjectKeyId.
func newRootCA(t *testing.T, ski []byte) (*x509.Certificate, *ecdsa.PrivateKey) {
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
	return cert, key
}

// leafChainTo mints a leaf certificate signed by root and returns the leaf-first chain.
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
		AuthorityKeyId: root.SubjectKeyId,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, root, &leafKey.PublicKey, rootKey)
	require.NoError(t, err)
	leaf, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return []*x509.Certificate{leaf}
}

// writeCertPEM writes cert as PEM into dir and returns the file name.
func writeCertPEM(t *testing.T, dir, name string, cert *x509.Certificate) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path,
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}), 0o600))
	return path
}

func (suite *TrustStoreTestSuite) newStore(certs map[string]*x509.Certificate) TrustStoreInterface {
	dir := suite.T().TempDir()
	entries := make([]engineconfig.TrustedAnchorEntry, 0, len(certs))
	for _, name := range []string{"root-a", "root-b"} {
		if cert, ok := certs[name]; ok {
			entries = append(entries, engineconfig.TrustedAnchorEntry{
				Name: name, CertFile: writeCertPEM(suite.T(), dir, name+".pem", cert),
			})
		}
	}
	store, err := Initialize(entries, "")
	suite.Require().NoError(err)
	return store
}

func (suite *TrustStoreTestSuite) TestListAnchors() {
	root, _ := newRootCA(suite.T(), []byte{0xaa, 0x01})
	store := suite.newStore(map[string]*x509.Certificate{"root-a": root})

	anchors := store.ListAnchors()
	suite.Require().Len(anchors, 1)
	suite.Equal("root-a", anchors[0].Name)
	suite.Equal(root.Subject.String(), anchors[0].Subject)
	suite.Equal(base64.RawURLEncoding.EncodeToString(root.SubjectKeyId), anchors[0].SKI)
	suite.Equal(root.NotAfter, anchors[0].NotAfter)
}

func (suite *TrustStoreTestSuite) TestInitializeEmpty() {
	store, err := Initialize(nil, "")
	suite.Require().NoError(err)
	suite.Empty(store.ListAnchors())

	root, rootKey := newRootCA(suite.T(), []byte{0x01})
	_, err = store.VerifyChain(leafChainTo(suite.T(), root, rootKey), time.Now(), nil)
	suite.ErrorIs(err, ErrUntrustedIssuer)
}

func (suite *TrustStoreTestSuite) TestInitializeResolvesRelativePath() {
	home := suite.T().TempDir()
	root, _ := newRootCA(suite.T(), []byte{0x01})
	writeCertPEM(suite.T(), home, "root.pem", root)

	store, err := Initialize([]engineconfig.TrustedAnchorEntry{{Name: "root-a", CertFile: "root.pem"}}, home)
	suite.Require().NoError(err)
	suite.Len(store.ListAnchors(), 1)
}

func (suite *TrustStoreTestSuite) TestInitializeErrors() {
	suite.Run("missing fields", func() {
		_, err := Initialize([]engineconfig.TrustedAnchorEntry{{Name: "x"}}, "")
		suite.ErrorIs(err, ErrInvalidConfig)
	})

	suite.Run("unreadable cert", func() {
		_, err := Initialize([]engineconfig.TrustedAnchorEntry{
			{Name: "x", CertFile: filepath.Join(suite.T().TempDir(), "missing.cert")},
		}, "")
		suite.Error(err)
	})
}

func (suite *TrustStoreTestSuite) TestVerifyChain() {
	rootA, keyA := newRootCA(suite.T(), []byte{0xaa, 0x01})
	rootB, keyB := newRootCA(suite.T(), []byte{0xbb, 0x02})
	store := suite.newStore(map[string]*x509.Certificate{"root-a": rootA, "root-b": rootB})
	chainA := leafChainTo(suite.T(), rootA, keyA)

	suite.Run("chains to any configured anchor", func() {
		leaf, err := store.VerifyChain(chainA, time.Now(), nil)
		suite.Require().NoError(err)
		suite.Same(chainA[0], leaf)
	})

	suite.Run("chains to allowed anchor", func() {
		_, err := store.VerifyChain(chainA, time.Now(), map[string]*x509.Certificate{"a": rootA})
		suite.NoError(err)
	})

	suite.Run("rejects anchor outside allowed set", func() {
		_, err := store.VerifyChain(chainA, time.Now(), map[string]*x509.Certificate{"b": rootB})
		suite.ErrorIs(err, ErrUntrustedIssuer)
	})

	suite.Run("allowed set overrides configured anchors", func() {
		other, otherKey := newRootCA(suite.T(), []byte{0xcc})
		_, err := store.VerifyChain(leafChainTo(suite.T(), other, otherKey), time.Now(),
			map[string]*x509.Certificate{"c": other})
		suite.NoError(err)
	})

	suite.Run("rejects unknown root", func() {
		other, otherKey := newRootCA(suite.T(), []byte{0xcc})
		_, err := store.VerifyChain(leafChainTo(suite.T(), other, otherKey), time.Now(), nil)
		suite.ErrorIs(err, ErrUntrustedIssuer)
	})

	suite.Run("rejects empty chain", func() {
		_, err := store.VerifyChain(nil, time.Now(), nil)
		suite.ErrorIs(err, ErrUntrustedIssuer)
	})

	suite.Run("chains through intermediate", func() {
		interKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		suite.Require().NoError(err)
		interTmpl := &x509.Certificate{
			SerialNumber:          big.NewInt(3),
			Subject:               pkix.Name{CommonName: "test-intermediate-ca"},
			NotBefore:             time.Now().Add(-time.Hour),
			NotAfter:              time.Now().Add(24 * time.Hour),
			IsCA:                  true,
			BasicConstraintsValid: true,
			KeyUsage:              x509.KeyUsageCertSign,
			SubjectKeyId:          []byte{0xdd, 0x03},
		}
		interDER, err := x509.CreateCertificate(rand.Reader, interTmpl, rootA, &interKey.PublicKey, keyA)
		suite.Require().NoError(err)
		inter, err := x509.ParseCertificate(interDER)
		suite.Require().NoError(err)

		chain := append(leafChainTo(suite.T(), inter, interKey), inter)
		leaf, err := store.VerifyChain(chain, time.Now(), nil)
		suite.Require().NoError(err)
		suite.Same(chain[0], leaf)

		_, err = store.VerifyChain(chain[:1], time.Now(), nil)
		suite.ErrorIs(err, ErrUntrustedIssuer, "leaf alone must not chain without the intermediate")
	})

	suite.Run("chains to second anchor", func() {
		_, err := store.VerifyChain(leafChainTo(suite.T(), rootB, keyB), time.Now(),
			map[string]*x509.Certificate{"b": rootB})
		suite.NoError(err)
	})
}

func (suite *TrustStoreTestSuite) TestAnchorsFor() {
	rootA, _ := newRootCA(suite.T(), []byte{0xaa, 0x01})
	rootB, _ := newRootCA(suite.T(), []byte{0xbb, 0x02})
	store := suite.newStore(map[string]*x509.Certificate{"root-a": rootA, "root-b": rootB})

	skiA := base64.RawURLEncoding.EncodeToString(rootA.SubjectKeyId)

	anchors, err := store.AnchorsFor([]string{"root-a", "root-a"})
	suite.Require().NoError(err)
	suite.Len(anchors, 1)
	suite.True(rootA.Equal(anchors[skiA]))

	anchors, err = store.AnchorsFor(nil)
	suite.Require().NoError(err)
	suite.Empty(anchors)

	_, err = store.AnchorsFor([]string{"root-a", "missing"})
	suite.ErrorIs(err, ErrUnknownAnchor)
}

func (suite *TrustStoreTestSuite) TestValidateNames() {
	rootA, _ := newRootCA(suite.T(), []byte{0xaa, 0x01})
	store := suite.newStore(map[string]*x509.Certificate{"root-a": rootA})

	suite.NoError(store.ValidateNames(nil))
	suite.NoError(store.ValidateNames([]string{"root-a"}))

	err := store.ValidateNames([]string{"root-a", "missing", "other"})
	suite.ErrorIs(err, ErrUnknownAnchor)
	suite.Contains(err.Error(), "missing, other")
}

func (suite *TrustStoreTestSuite) TestLoadCertificateErrors() {
	dir := suite.T().TempDir()

	suite.Run("missing file", func() {
		_, err := loadCertificate(filepath.Join(dir, "missing.cert"))
		suite.Error(err)
	})

	suite.Run("not PEM", func() {
		path := filepath.Join(dir, "garbage.cert")
		suite.Require().NoError(os.WriteFile(path, []byte("not pem"), 0o600))
		_, err := loadCertificate(path)
		suite.Error(err)
	})

	suite.Run("public key PEM", func() {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		suite.Require().NoError(err)
		der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
		suite.Require().NoError(err)
		path := filepath.Join(dir, "issuer.pub")
		suite.Require().NoError(os.WriteFile(path,
			pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0o600))
		_, err = loadCertificate(path)
		suite.Error(err)
	})

	suite.Run("corrupt certificate", func() {
		path := filepath.Join(dir, "corrupt.cert")
		suite.Require().NoError(os.WriteFile(path,
			pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("not der")}), 0o600))
		_, err := loadCertificate(path)
		suite.Error(err)
	})
}

func (suite *TrustStoreTestSuite) TestResolvePath() {
	home := filepath.Join(suite.T().TempDir(), "home")
	abs := filepath.Join(home, "abs", "cert")
	suite.Equal("", resolvePath(home, ""))
	suite.Equal(abs, resolvePath(home, abs))
	suite.Equal(filepath.Join(home, "rel", "cert"), resolvePath(home, filepath.Join("rel", "cert")))
	suite.Equal(filepath.Join("rel", "cert"), resolvePath("", filepath.Join("rel", "cert")))
}
