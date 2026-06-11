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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// writeSelfSignedCertPEM writes a self-signed cert for key to a temp file and
// returns its path.
func writeSelfSignedCertPEM(t *testing.T, key *ecdsa.PrivateKey) string {
	t.Helper()
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-issuer"},
		NotBefore:    time.Unix(1_700_000_000, 0),
		NotAfter:     time.Unix(1_900_000_000, 0),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "issuer.cert")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	require.NoError(t, os.WriteFile(path, pemBytes, 0o600))
	return path
}

func TestBuildTrustStoreLoadsCertificate(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	certPath := writeSelfSignedCertPEM(t, key)

	store, err := buildTrustStore([]trustedIssuer{
		{Issuer: "https://issuer.example", CertFile: certPath},
	})
	require.NoError(t, err)

	got, err := store.resolveIssuerKey(context.Background(), "https://issuer.example")
	require.NoError(t, err)
	pub, ok := got.(*ecdsa.PublicKey)
	require.True(t, ok)
	assert.True(t, key.PublicKey.Equal(pub))

	_, err = store.resolveIssuerKey(context.Background(), "https://unknown.example")
	assert.ErrorIs(t, err, ErrUntrustedIssuer)
}

func TestBuildTrustStoreErrors(t *testing.T) {
	t.Run("no issuers", func(t *testing.T) {
		_, err := buildTrustStore(nil)
		assert.ErrorIs(t, err, ErrPolicy)
	})

	t.Run("missing fields", func(t *testing.T) {
		_, err := buildTrustStore([]trustedIssuer{{Issuer: "x"}})
		assert.ErrorIs(t, err, ErrPolicy)
	})

	t.Run("unreadable cert", func(t *testing.T) {
		_, err := buildTrustStore([]trustedIssuer{
			{Issuer: "x", CertFile: filepath.Join(t.TempDir(), "missing.cert")},
		})
		assert.Error(t, err)
	})
}

func TestLoadIssuerKeyRejectsNonPEM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "garbage.cert")
	require.NoError(t, os.WriteFile(path, []byte("not pem"), 0o600))
	_, err := loadIssuerKey(path)
	assert.Error(t, err)
}

func TestLoadIssuerKeyMissingFile(t *testing.T) {
	_, err := loadIssuerKey(filepath.Join(t.TempDir(), "missing.cert"))
	assert.Error(t, err)
}

func TestLoadIssuerKeyAcceptsPublicKeyPEM(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "issuer.pub")
	require.NoError(t, os.WriteFile(path,
		pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0o600))

	got, err := loadIssuerKey(path)
	require.NoError(t, err)
	pub, ok := got.(*ecdsa.PublicKey)
	require.True(t, ok)
	assert.True(t, key.PublicKey.Equal(pub))
}

func TestLoadIssuerKeyRejectsUnsupportedPEMBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unknown.pem")
	require.NoError(t, os.WriteFile(path,
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte("xx")}), 0o600))
	_, err := loadIssuerKey(path)
	assert.Error(t, err)
}

func TestLoadIssuerKeyRejectsCorruptCertificate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.cert")
	require.NoError(t, os.WriteFile(path,
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("not der")}), 0o600))
	_, err := loadIssuerKey(path)
	assert.Error(t, err)
}

func TestLoadVerifierInfoEmptyPath(t *testing.T) {
	got, err := loadVerifierInfo("", "")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestLoadVerifierInfoReadsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registration.jwt")
	require.NoError(t, os.WriteFile(path, []byte("  eyJhbGciOiJFUzI1NiJ9.payload.sig  \n"), 0o600))

	got, err := loadVerifierInfo(path, "")
	require.NoError(t, err)
	require.Len(t, got, 1)
	entry := got[0].(map[string]interface{})
	assert.Equal(t, "registration_cert", entry["format"])
	assert.Equal(t, "eyJhbGciOiJFUzI1NiJ9.payload.sig", entry["data"])
}

func TestLoadVerifierInfoResolvesRelativePath(t *testing.T) {
	home := t.TempDir()
	rel := "regcert.jwt"
	require.NoError(t, os.WriteFile(filepath.Join(home, rel), []byte("jwt-data"), 0o600))

	got, err := loadVerifierInfo(rel, home)
	require.NoError(t, err)
	require.Len(t, got, 1)
	entry := got[0].(map[string]interface{})
	assert.Equal(t, "jwt-data", entry["data"])
}

func TestLoadVerifierInfoMissingFile(t *testing.T) {
	_, err := loadVerifierInfo(filepath.Join(t.TempDir(), "missing"), "")
	assert.Error(t, err)
}

func TestLoadVerifierInfoEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.jwt")
	require.NoError(t, os.WriteFile(path, []byte("   \n"), 0o600))
	_, err := loadVerifierInfo(path, "")
	assert.ErrorIs(t, err, ErrPolicy)
}

// testEngineService returns a Service without registering HTTP routes so the
// build-definition tests can exercise the config -> registry path without
// running the full Initialize wiring.
func testEngineService(t *testing.T) *service {
	t.Helper()
	svc, err := newService(serviceConfig{
		RequestURIBase:  "https://verifier.example/openid4vp/request",
		ResponseURIBase: "https://verifier.example/openid4vp/response",
	}, newInMemoryStateStore(), "x509_hash:test", stubRequestSigner{})
	require.NoError(t, err)
	return svc
}

// stubRequestSigner satisfies requestSigner for tests that only inspect
// definition wiring; it produces no real signature.
type stubRequestSigner struct{}

func (stubRequestSigner) signRequestObject(_ context.Context, _ map[string]interface{}) (string, error) {
	return "", nil
}

// Building a definition from config registers the SD-JWT format plug-in,
// derives the policy from the credential-specific config, and wires a static
// trust store from the trusted_issuers list.
func TestBuildDefinitionFromConfig(t *testing.T) {
	svc := testEngineService(t)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	certPath := writeSelfSignedCertPEM(t, key)

	dc := config.DefinitionConfig{
		ID:              "eudi-pid",
		DisplayName:     "EUDI Wallet PID",
		CredentialID:    "pid-sd-jwt",
		VCT:             "urn:eudi:pid:de:1",
		RequestedClaims: []string{"given_name", "family_name", "birthdate"},
		MandatoryClaims: []string{"given_name", "family_name"},
		SubjectClaims:   []string{"family_name", "given_name", "birthdate"},
		TrustedIssuers: []config.TrustedIssuerEntry{
			{Issuer: "https://issuer.example", CertFile: certPath},
		},
	}
	def, err := buildDefinition(dc, svc, "")
	require.NoError(t, err)
	assert.Equal(t, "eudi-pid", def.ID)
	assert.Equal(t, "EUDI Wallet PID", def.DisplayName)
	assert.Equal(t, "pid-sd-jwt", def.DCQL.CredentialID)
	assert.Equal(t, "urn:eudi:pid:de:1", def.DCQL.VCT)
	assert.Equal(t, dc.RequestedClaims, def.DCQL.Claims)
	assert.Equal(t, dc.RequestedClaims, def.policy.RequestedClaims)
	assert.Equal(t, dc.MandatoryClaims, def.policy.MandatoryClaims)
	assert.Equal(t, "urn:eudi:pid:de:1", def.policy.ExpectedVCT)
	assert.Equal(t, svc.clientID, def.policy.Audience)
	assert.Equal(t, dc.SubjectClaims, def.SubjectClaims)
	require.NotNil(t, def.Trust)
	require.NotNil(t, def.DeriveSubject)
}

// resolvePath joins relative cert paths to serverHome and leaves absolute and
// empty paths untouched.
func TestResolvePath(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	abs := filepath.Join(home, "abs", "cert")
	assert.Equal(t, "", resolvePath(home, ""))
	assert.Equal(t, abs, resolvePath(home, abs))
	assert.Equal(t, filepath.Join(home, "rel", "cert"), resolvePath(home, filepath.Join("rel", "cert")))
	assert.Equal(t, filepath.Join("rel", "cert"), resolvePath("", filepath.Join("rel", "cert")))
}

// buildDefinition surfaces a clear error when ID is missing — the registry
// would otherwise reject it later.
func TestBuildDefinitionRequiresID(t *testing.T) {
	svc := testEngineService(t)
	_, err := buildDefinition(config.DefinitionConfig{}, svc, "")
	assert.ErrorIs(t, err, ErrPolicy)
}
