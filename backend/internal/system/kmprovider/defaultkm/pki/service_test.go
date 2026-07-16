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

package pki

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// TestLoadCertKeyPairMLDSA loads the RFC 9881 ML-DSA cert/key fixtures (generated
// with OpenSSL, "both" private-key format) and exercises the full path: detection,
// key reconstruction, certificate thumbprint, and a sign/verify round trip.
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
