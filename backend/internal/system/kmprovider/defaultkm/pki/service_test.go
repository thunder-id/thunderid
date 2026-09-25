// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package pki

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/mldsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// TestLoadCertKeyPairMLDSA loads the RFC 9881 ML-DSA cert/key fixtures (generated
// with OpenSSL, seed-only private-key format) and exercises the full path: detection,
// key parsing, certificate thumbprint, and a sign/verify round trip.
func TestLoadCertKeyPairMLDSA(t *testing.T) {
	cases := []struct {
		file    string
		pkiAlg  PKIAlgorithm
		signAlg cryptolib.SignAlgorithm
	}{
		{"mldsa44", MLDSA44, cryptolib.MLDSA44},
		{"mldsa65", MLDSA65, cryptolib.MLDSA65},
		{"mldsa87", MLDSA87, cryptolib.MLDSA87},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			certPath := filepath.Join("testdata", tc.file+".cert")
			keyPath := filepath.Join("testdata", tc.file+".key")

			tlsCert, alg, err := loadCertKeyPair(certPath, keyPath)
			require.NoError(t, err)
			assert.Equal(t, tc.pkiAlg, alg)
			require.NotEmpty(t, tlsCert.Certificate)

			thumb, err := getThumbprint(tlsCert)
			require.NoError(t, err)
			assert.NotEmpty(t, thumb)

			signer, ok := tlsCert.PrivateKey.(crypto.Signer)
			require.True(t, ok)

			data := []byte("token signing input")
			sig, err := cryptolib.Generate(data, tc.signAlg, signer)
			require.NoError(t, err)
			assert.NoError(t, cryptolib.Verify(data, sig, tc.signAlg, signer.Public()))
		})
	}
}

// TestLoadCertKeyPairMLDSARejectsBothFormat confirms a private key in the RFC 9881
// "both" (seed + expandedKey) encoding is rejected; only seed-only keys load.
func TestLoadCertKeyPairMLDSARejectsBothFormat(t *testing.T) {
	keyPEM, err := os.ReadFile(filepath.Join("testdata", "mldsa65.key"))
	require.NoError(t, err)
	block, _ := pem.Decode(keyPEM)
	require.NotNil(t, block)
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	require.NoError(t, err)

	var pkcs8 struct {
		Version    int
		Algo       pkix.AlgorithmIdentifier
		PrivateKey []byte
	}
	_, err = asn1.Unmarshal(block.Bytes, &pkcs8)
	require.NoError(t, err)
	pkcs8.PrivateKey, err = asn1.Marshal(struct {
		Seed        []byte
		ExpandedKey []byte
	}{Seed: key.(*mldsa.PrivateKey).Bytes(), ExpandedKey: []byte{0x01}})
	require.NoError(t, err)
	der, err := asn1.Marshal(pkcs8)
	require.NoError(t, err)

	keyPath := filepath.Join(t.TempDir(), "both.key")
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600))

	_, _, err = loadCertKeyPair(filepath.Join("testdata", "mldsa65.cert"), keyPath)
	assert.ErrorContains(t, err, "seed-only")
}

func TestLoadCertKeyPairMLDSARejectsMismatchedKey(t *testing.T) {
	_, _, err := loadCertKeyPair(filepath.Join("testdata", "mldsa65.cert"), filepath.Join("testdata", "mldsa44.key"))
	assert.ErrorContains(t, err, "private key does not match public key")
}

func TestLoadCertKeyPairMLDSAKeepsCertificateChain(t *testing.T) {
	leaf, err := os.ReadFile(filepath.Join("testdata", "mldsa65.cert"))
	require.NoError(t, err)
	intermediate, err := os.ReadFile(filepath.Join("testdata", "mldsa44.cert"))
	require.NoError(t, err)
	chainPath := filepath.Join(t.TempDir(), "chain.cert")
	require.NoError(t, os.WriteFile(chainPath, append(leaf, intermediate...), 0o600))

	tlsCert, alg, err := loadCertKeyPair(chainPath, filepath.Join("testdata", "mldsa65.key"))
	require.NoError(t, err)
	assert.Equal(t, MLDSA65, alg)
	assert.Len(t, tlsCert.Certificate, 2)
}

func TestLoadCertKeyPairClassicalUnaffected(t *testing.T) {
	// A non-ML-DSA key file must fall through to the standard loader; a missing
	// classical pair simply errors rather than being misdetected as ML-DSA.
	_, _, err := loadCertKeyPair(
		filepath.Join("testdata", "does-not-exist.cert"),
		filepath.Join("testdata", "does-not-exist.key"),
	)
	assert.Error(t, err)
}

func TestPKIAlgorithmToJWSAlgorithmsMLDSA(t *testing.T) {
	assert.Equal(t, []string{"ML-DSA-44"}, pkiAlgorithmToJWSAlgorithms(MLDSA44))
	assert.Equal(t, []string{"ML-DSA-65"}, pkiAlgorithmToJWSAlgorithms(MLDSA65))
	assert.Equal(t, []string{"ML-DSA-87"}, pkiAlgorithmToJWSAlgorithms(MLDSA87))
}

func setupServerRuntime(t *testing.T, keys []engineconfig.KeyConfig) {
	config.ResetServerRuntime()
	t.Cleanup(config.ResetServerRuntime)
	err := config.InitializeServerRuntime(".", &config.Config{
		Crypto: config.CryptoConfig{Keys: keys},
	})
	require.NoError(t, err)
}

// TestNewPKIServiceMLDSA loads an ML-DSA key/cert pair through the full
// newPKIService construction path and exercises every PKIServiceInterface method.
func TestNewPKIServiceMLDSA(t *testing.T) {
	setupServerRuntime(t, []engineconfig.KeyConfig{
		{ID: "mldsa65", CertFile: "testdata/mldsa65.cert", KeyFile: "testdata/mldsa65.key"},
	})

	svc, err := newPKIService()
	require.NoError(t, err)

	ctx := context.Background()
	privKey, svcErr := svc.GetPrivateKey(ctx, "mldsa65")
	require.Nil(t, svcErr)
	assert.NotNil(t, privKey)

	assert.NotEmpty(t, svc.GetCertificateChain("mldsa65"))
	assert.Nil(t, svc.GetCertificateChain("missing"))

	assert.NotEmpty(t, svc.GetCertThumbprint("mldsa65"))
	assert.Empty(t, svc.GetCertThumbprint("missing"))

	cert, svcErr := svc.GetX509Certificate(ctx, "mldsa65")
	require.Nil(t, svcErr)
	assert.NotNil(t, cert)

	_, svcErr = svc.GetX509Certificate(ctx, "missing")
	assert.NotNil(t, svcErr)

	allCerts, svcErr := svc.GetAllX509Certificates(ctx)
	require.Nil(t, svcErr)
	assert.Len(t, allCerts, 1)

	assert.Equal(t, []string{"ML-DSA-65"}, svc.GetSupportedSigningAlgorithms())
}

func TestNewPKIServiceNoKeyConfigs(t *testing.T) {
	setupServerRuntime(t, nil)

	_, err := newPKIService()
	assert.EqualError(t, err, "no key configurations found in the system configuration")
}

func TestNewPKIServiceEmptyID(t *testing.T) {
	setupServerRuntime(t, []engineconfig.KeyConfig{
		{ID: "", CertFile: "testdata/mldsa65.cert", KeyFile: "testdata/mldsa65.key"},
	})

	_, err := newPKIService()
	assert.EqualError(t, err, "key configuration has empty ID")
}

func TestNewPKIServiceMissingCertFile(t *testing.T) {
	setupServerRuntime(t, []engineconfig.KeyConfig{
		{ID: "missing-cert", CertFile: "testdata/does-not-exist.cert", KeyFile: "testdata/mldsa65.key"},
	})

	_, err := newPKIService()
	assert.ErrorContains(t, err, "certificate file not found")
}

func TestNewPKIServiceMissingKeyFile(t *testing.T) {
	setupServerRuntime(t, []engineconfig.KeyConfig{
		{ID: "missing-key", CertFile: "testdata/mldsa65.cert", KeyFile: "testdata/does-not-exist.key"},
	})

	_, err := newPKIService()
	assert.ErrorContains(t, err, "key file not found")
}

func TestGetAlgorithmFromKey(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	alg, err := getAlgorithmFromKey(rsaKey)
	require.NoError(t, err)
	assert.Equal(t, RSA, alg)

	curves := []struct {
		curve elliptic.Curve
		want  PKIAlgorithm
	}{
		{elliptic.P256(), P256},
		{elliptic.P384(), P384},
		{elliptic.P521(), P521},
	}
	for _, tc := range curves {
		ecKey, err := ecdsa.GenerateKey(tc.curve, rand.Reader)
		require.NoError(t, err)
		alg, err := getAlgorithmFromKey(ecKey)
		require.NoError(t, err)
		assert.Equal(t, tc.want, alg)
	}

	_, edKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	alg, err = getAlgorithmFromKey(edKey)
	require.NoError(t, err)
	assert.Equal(t, Ed25519, alg)

	_, err = getAlgorithmFromKey("unsupported")
	assert.ErrorContains(t, err, "unsupported key type")
}

func TestGetAlgorithmFromKeyUnsupportedCurve(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	require.NoError(t, err)

	_, err = getAlgorithmFromKey(ecKey)
	assert.ErrorContains(t, err, "unsupported ECDSA curve")
}
