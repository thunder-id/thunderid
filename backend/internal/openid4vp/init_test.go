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
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
)

type OpenID4VPInitTestSuite struct {
	suite.Suite
}

func TestOpenID4VPInitTestSuite(t *testing.T) {
	suite.Run(t, new(OpenID4VPInitTestSuite))
}

// writeCACertPEM writes a CA certificate as PEM to a temp file and returns the path.
func writeCACertPEM(t *testing.T, cert *x509.Certificate) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ca.cert")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	require.NoError(t, os.WriteFile(path, pemBytes, 0o600))
	return path
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

func (suite *OpenID4VPInitTestSuite) TestBuildTrustStoreLoadsCertificate() {
	t := suite.T()
	root, rootKey := newTestRootCA(t)
	certPath := writeCACertPEM(t, root)

	store, err := buildTrustStore([]trustAnchor{
		{Name: "root-a", CertFile: certPath},
	})
	suite.Require().NoError(err)

	anchors := store.list()
	suite.Require().Len(anchors, 1)
	suite.Equal("root-a", anchors[0].Name)
	suite.Equal(root.Subject.String(), anchors[0].Subject)

	// A leaf chaining to the configured root verifies.
	leaf, err := store.verifyChain(leafChainTo(t, root, rootKey), time.Now(), nil)
	suite.Require().NoError(err)
	suite.Require().NotNil(leaf)

	// A leaf chaining to an unknown root is rejected.
	otherRoot, otherKey := newTestRootCA(t)
	_, err = store.verifyChain(leafChainTo(t, otherRoot, otherKey), time.Now(), nil)
	suite.ErrorIs(err, ErrUntrustedIssuer)
}

func (suite *OpenID4VPInitTestSuite) TestTrustStoreSKIsFor() {
	rootA := caCertWithSKI(suite.T(), []byte{0xaa, 0x01})
	rootB := caCertWithSKI(suite.T(), []byte{0xbb, 0x02})
	store := newTrustAnchorStore(
		[]*x509.Certificate{rootA, rootB}, []string{"root-a", "root-b"})

	skiA := base64.RawURLEncoding.EncodeToString(rootA.SubjectKeyId)
	skiB := base64.RawURLEncoding.EncodeToString(rootB.SubjectKeyId)

	// Known names resolve in order; unknown names and duplicates are skipped.
	got := store.skisFor([]string{"root-b", "missing", "root-a", "root-b"})
	suite.Equal([]string{skiB, skiA}, got)

	suite.Empty(store.skisFor(nil))
	suite.Empty(store.skisFor([]string{"missing"}))
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

func (suite *OpenID4VPInitTestSuite) TestBuildTrustStoreErrors() {
	suite.Run("no anchors", func() {
		_, err := buildTrustStore(nil)
		suite.ErrorIs(err, ErrPolicy)
	})

	suite.Run("missing fields", func() {
		_, err := buildTrustStore([]trustAnchor{{Name: "x"}})
		suite.ErrorIs(err, ErrPolicy)
	})

	suite.Run("unreadable cert", func() {
		_, err := buildTrustStore([]trustAnchor{
			{Name: "x", CertFile: filepath.Join(suite.T().TempDir(), "missing.cert")},
		})
		suite.Error(err)
	})
}

func (suite *OpenID4VPInitTestSuite) TestLoadCertificateRejectsNonPEM() {
	path := filepath.Join(suite.T().TempDir(), "garbage.cert")
	suite.Require().NoError(os.WriteFile(path, []byte("not pem"), 0o600))
	_, err := loadCertificate(path)
	suite.Error(err)
}

func (suite *OpenID4VPInitTestSuite) TestLoadCertificateMissingFile() {
	_, err := loadCertificate(filepath.Join(suite.T().TempDir(), "missing.cert"))
	suite.Error(err)
}

func (suite *OpenID4VPInitTestSuite) TestLoadCertificateRejectsPublicKeyPEM() {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	suite.Require().NoError(err)
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	suite.Require().NoError(err)
	path := filepath.Join(suite.T().TempDir(), "issuer.pub")
	suite.Require().NoError(os.WriteFile(path,
		pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0o600))

	_, err = loadCertificate(path)
	suite.Error(err)
}

func (suite *OpenID4VPInitTestSuite) TestLoadCertificateRejectsCorruptCertificate() {
	path := filepath.Join(suite.T().TempDir(), "corrupt.cert")
	suite.Require().NoError(os.WriteFile(path,
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("not der")}), 0o600))
	_, err := loadCertificate(path)
	suite.Error(err)
}

func (suite *OpenID4VPInitTestSuite) TestLoadVerifierInfoEmptyPath() {
	got, err := loadVerifierInfo("", "")
	suite.Require().NoError(err)
	suite.Nil(got)
}

func (suite *OpenID4VPInitTestSuite) TestLoadVerifierInfoReadsFile() {
	dir := suite.T().TempDir()
	path := filepath.Join(dir, "registration.jwt")
	suite.Require().NoError(os.WriteFile(path, []byte("  eyJhbGciOiJFUzI1NiJ9.payload.sig  \n"), 0o600))

	got, err := loadVerifierInfo(path, "")
	suite.Require().NoError(err)
	suite.Require().Len(got, 1)
	entry := got[0].(map[string]interface{})
	suite.Equal("registration_cert", entry["format"])
	suite.Equal("eyJhbGciOiJFUzI1NiJ9.payload.sig", entry["data"])
}

func (suite *OpenID4VPInitTestSuite) TestLoadVerifierInfoResolvesRelativePath() {
	home := suite.T().TempDir()
	rel := "regcert.jwt"
	suite.Require().NoError(os.WriteFile(filepath.Join(home, rel), []byte("jwt-data"), 0o600))

	got, err := loadVerifierInfo(rel, home)
	suite.Require().NoError(err)
	suite.Require().Len(got, 1)
	entry := got[0].(map[string]interface{})
	suite.Equal("jwt-data", entry["data"])
}

func (suite *OpenID4VPInitTestSuite) TestLoadVerifierInfoMissingFile() {
	_, err := loadVerifierInfo(filepath.Join(suite.T().TempDir(), "missing"), "")
	suite.Error(err)
}

func (suite *OpenID4VPInitTestSuite) TestLoadVerifierInfoEmptyFile() {
	path := filepath.Join(suite.T().TempDir(), "empty.jwt")
	suite.Require().NoError(os.WriteFile(path, []byte("   \n"), 0o600))
	_, err := loadVerifierInfo(path, "")
	suite.ErrorIs(err, ErrPolicy)
}

// Initialize disables the verifier engine (nil service) when no signing key is
// configured, but the presentation-definition management API stays available.
func (suite *OpenID4VPInitTestSuite) TestInitializeDisabledWithoutSigningKey() {
	config.ResetServerRuntime()
	suite.Require().NoError(config.InitializeServerRuntime("", &config.Config{}))
	defer config.ResetServerRuntime()

	svc, defSvc, exporter, err := Initialize(http.NewServeMux(), nil, nil, nil, nil)
	suite.Require().NoError(err)
	suite.Nil(svc)
	suite.NotNil(defSvc)
	suite.NotNil(exporter)
}

// Initialize fails when the presentation-definition store mode is invalid.
func (suite *OpenID4VPInitTestSuite) TestInitializeInvalidStoreMode() {
	config.ResetServerRuntime()
	cfg := &config.Config{}
	cfg.OpenID4VP.Store = "bogus"
	suite.Require().NoError(config.InitializeServerRuntime("", cfg))
	defer config.ResetServerRuntime()

	svc, defSvc, exporter, err := Initialize(http.NewServeMux(), nil, nil, nil, nil)
	suite.Require().Error(err)
	suite.Nil(svc)
	suite.Nil(defSvc)
	suite.Nil(exporter)
}

// Initialize fails when a signing key is configured but client_id is missing.
func (suite *OpenID4VPInitTestSuite) TestInitializeRequiresClientID() {
	config.ResetServerRuntime()
	cfg := &config.Config{}
	cfg.OpenID4VP.SigningKeyID = "signing-key"
	suite.Require().NoError(config.InitializeServerRuntime("", cfg))
	defer config.ResetServerRuntime()

	svc, defSvc, exporter, err := Initialize(http.NewServeMux(), nil, nil, nil, nil)
	suite.Require().Error(err)
	suite.ErrorIs(err, ErrPolicy)
	suite.Nil(svc)
	suite.Nil(defSvc)
	suite.Nil(exporter)
}

// buildSharedTrustStore builds one engine-wide trust anchor store from the configured trust anchors.
func (suite *OpenID4VPInitTestSuite) TestBuildSharedTrustStore() {
	t := suite.T()
	root, rootKey := newTestRootCA(t)
	certPath := writeCACertPEM(t, root)

	trust, err := buildSharedTrustStore([]config.TrustedAnchorEntry{
		{Name: "root-a", CertFile: certPath},
	}, "")
	suite.Require().NoError(err)
	suite.Require().NotNil(trust)

	anchors := trust.list()
	suite.Require().Len(anchors, 1)
	suite.Equal("root-a", anchors[0].Name)

	leaf, err := trust.verifyChain(leafChainTo(t, root, rootKey), time.Now(), nil)
	suite.Require().NoError(err)
	suite.Require().NotNil(leaf)

	// No anchors -> nil store, no error.
	store, err := buildSharedTrustStore(nil, "")
	suite.Require().NoError(err)
	suite.Require().Nil(store)
}

// resolvePath joins relative cert paths to serverHome and leaves absolute and
// empty paths untouched.
func (suite *OpenID4VPInitTestSuite) TestResolvePath() {
	home := filepath.Join(suite.T().TempDir(), "home")
	abs := filepath.Join(home, "abs", "cert")
	suite.Equal("", resolvePath(home, ""))
	suite.Equal(abs, resolvePath(home, abs))
	suite.Equal(filepath.Join(home, "rel", "cert"), resolvePath(home, filepath.Join("rel", "cert")))
	suite.Equal(filepath.Join("rel", "cert"), resolvePath("", filepath.Join("rel", "cert")))
}
