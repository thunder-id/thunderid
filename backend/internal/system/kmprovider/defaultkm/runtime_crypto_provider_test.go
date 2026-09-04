// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package defaultkm

import (
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"math/big"
	"testing"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/pki/pkimock"
)

const testKeyID = "test-key-id"

func newTestSvcErr() *tidcommon.ServiceError {
	return &tidcommon.ServiceError{
		Code:  "TEST-001",
		Error: tidcommon.I18nMessage{DefaultValue: "key not found"},
	}
}

func newTestLogger() *log.Logger {
	return log.GetLogger()
}

// encParamsMap builds the generic params map for a key-establishment Encrypt call, mirroring what
// jwe/service.go sends across the providers.RuntimeCryptoProvider boundary.
func encParamsMap(contentEncAlg string) map[string]interface{} {
	return map[string]interface{}{providers.ParamContentEncryptionAlgorithm: contentEncAlg}
}

// decParamsMap builds the generic params map for a CEK-unwrap Decrypt call. epk is nil for
// RSA-OAEP/RSA-OAEP-256, and the ECDH-ES ephemeral public key for ECDH-ES variants.
func decParamsMap(epk any) map[string]interface{} {
	m := map[string]interface{}{providers.ParamContentEncryptionAlgorithm: "A256GCM"}
	if epk != nil {
		m[providers.ParamEPK] = epk
	}
	return m
}

func TestEncrypt_RSAOAEP256_SuccessViaConstructor(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	cert := &x509.Certificate{PublicKey: &rsaKey.PublicKey}

	pkiMock := pkimock.NewPKIServiceInterfaceMock(t)
	pkiMock.EXPECT().
		GetX509Certificate(mock.Anything, testKeyID).
		Return(cert, nil)

	svc := NewRuntimeCryptoService(pkiMock, nil)

	wrappedCEK, details, err := svc.Encrypt(
		context.Background(), &providers.KeyRef{KeyID: testKeyID},
		string(cryptolib.AlgorithmRSAOAEP256), encParamsMap("A256GCM"), nil,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, wrappedCEK)
	assert.NotNil(t, details)
}

func TestEncrypt_RSAOAEP256_GetPublicKeyError(t *testing.T) {
	pkiMock := pkimock.NewPKIServiceInterfaceMock(t)
	pkiMock.EXPECT().
		GetX509Certificate(mock.Anything, testKeyID).
		Return(nil, newTestSvcErr())

	svc := NewRuntimeCryptoService(pkiMock, nil)

	_, _, err := svc.Encrypt(context.Background(), &providers.KeyRef{KeyID: testKeyID},
		string(cryptolib.AlgorithmRSAOAEP256), encParamsMap("A256GCM"), []byte("data"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), testKeyID)
}

func TestEncrypt_AESGCM_NilConfigCryptoService(t *testing.T) {
	svc := NewRuntimeCryptoService(pkimock.NewPKIServiceInterfaceMock(t), nil)

	_, _, err := svc.Encrypt(context.Background(), nil,
		string(cryptolib.AlgorithmAESGCM), nil, []byte("data"))
	assert.EqualError(t, err, "config crypto service not initialized")
}

func TestEncrypt_AESGCM_Success(t *testing.T) {
	cfg := cryptomock.NewConfigCryptoProviderMock(t)
	cfg.EXPECT().Encrypt(mock.Anything, []byte("data")).Return([]byte("encrypted"), nil)

	svc := NewRuntimeCryptoService(pkimock.NewPKIServiceInterfaceMock(t), cfg)

	encrypted, details, err := svc.Encrypt(context.Background(), nil,
		string(cryptolib.AlgorithmAESGCM), nil, []byte("data"))
	require.NoError(t, err)
	assert.Equal(t, []byte("encrypted"), encrypted)
	assert.Nil(t, details)
}

func TestDecrypt_AESGCM_NilConfigCryptoService(t *testing.T) {
	svc := NewRuntimeCryptoService(pkimock.NewPKIServiceInterfaceMock(t), nil)

	_, err := svc.Decrypt(context.Background(), nil, string(cryptolib.AlgorithmAESGCM), nil, []byte("data"))
	assert.EqualError(t, err, "config crypto service not initialized")
}

func TestDecrypt_AESGCM_Success(t *testing.T) {
	cfg := cryptomock.NewConfigCryptoProviderMock(t)
	cfg.EXPECT().Decrypt(mock.Anything, []byte("encrypted")).Return([]byte("data"), nil)

	svc := NewRuntimeCryptoService(pkimock.NewPKIServiceInterfaceMock(t), cfg)

	decrypted, err := svc.Decrypt(context.Background(), nil,
		string(cryptolib.AlgorithmAESGCM), nil, []byte("encrypted"))
	require.NoError(t, err)
	assert.Equal(t, []byte("data"), decrypted)
}

func TestEncrypt_ECDHES_Success(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	cert := &x509.Certificate{PublicKey: &ecKey.PublicKey}

	pkiMock := pkimock.NewPKIServiceInterfaceMock(t)
	pkiMock.EXPECT().
		GetX509Certificate(mock.Anything, testKeyID).
		Return(cert, nil)

	svc := NewRuntimeCryptoService(pkiMock, nil)

	_, details, err := svc.Encrypt(context.Background(), &providers.KeyRef{KeyID: testKeyID},
		string(cryptolib.AlgorithmECDHES), encParamsMap("A128GCM"), nil)
	require.NoError(t, err)
	assert.NotNil(t, details)
}

func TestEncrypt_ECDHES_GetPublicKeyError(t *testing.T) {
	pkiMock := pkimock.NewPKIServiceInterfaceMock(t)
	pkiMock.EXPECT().
		GetX509Certificate(mock.Anything, testKeyID).
		Return(nil, newTestSvcErr())

	svc := NewRuntimeCryptoService(pkiMock, nil)

	_, _, err := svc.Encrypt(context.Background(), &providers.KeyRef{KeyID: testKeyID},
		string(cryptolib.AlgorithmECDHES), encParamsMap("A128GCM"), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), testKeyID)
}

func TestEncrypt_UnsupportedAlgorithm(t *testing.T) {
	svc := NewRuntimeCryptoService(nil, nil)

	_, _, err := svc.Encrypt(
		context.Background(), &providers.KeyRef{KeyID: testKeyID}, "UNSUPPORTED", nil, []byte("data"),
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported algorithm")
}

// Encrypt – RSA-OAEP-256
func TestEncrypt_RSAOAEP256_NilKeyRef(t *testing.T) {
	svc := &runtimeCryptoService{pkiService: pkimock.NewPKIServiceInterfaceMock(t)}

	_, _, err := svc.Encrypt(context.Background(), nil,
		string(cryptolib.AlgorithmRSAOAEP256), encParamsMap("A256GCM"), []byte("data"))
	assert.EqualError(t, err, "keyRef required for RSA-OAEP-256")
}

func TestEncrypt_RSAOAEP256_PKIError(t *testing.T) {
	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetX509Certificate(mock.Anything, "key1").Return(nil, &tidcommon.InternalServerError)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &providers.KeyRef{KeyID: "key1"}

	_, _, err := svc.Encrypt(context.Background(), keyRef,
		string(cryptolib.AlgorithmRSAOAEP256), encParamsMap("A256GCM"), []byte("data"))
	assert.Error(t, err)
}

func TestEncrypt_RSAOAEP256_NonRSAPublicKey(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetX509Certificate(mock.Anything, "key1").Return(
		&x509.Certificate{PublicKey: &ecKey.PublicKey}, nil,
	)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &providers.KeyRef{KeyID: "key1"}

	_, _, err = svc.Encrypt(context.Background(), keyRef,
		string(cryptolib.AlgorithmRSAOAEP256), encParamsMap("A256GCM"), []byte("data"))
	assert.EqualError(t, err, "key is not an RSA public key")
}

func TestEncrypt_RSAOAEP256_Success(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetX509Certificate(mock.Anything, "key1").Return(
		&x509.Certificate{PublicKey: &rsaKey.PublicKey}, nil,
	)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &providers.KeyRef{KeyID: "key1"}

	ciphertext, details, err := svc.Encrypt(context.Background(), keyRef,
		string(cryptolib.AlgorithmRSAOAEP256), encParamsMap("A256GCM"), nil)
	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)
	require.NotNil(t, details)
	assert.NotEmpty(t, details.CEK)
}

// Encrypt with a caller-supplied public key (no KeyID / PKI lookup).
func TestEncrypt_RSAOAEP256_WithPublicKeyOnly(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	svc := &runtimeCryptoService{}
	keyRef := &providers.KeyRef{PublicKey: &rsaKey.PublicKey}

	ciphertext, details, err := svc.Encrypt(context.Background(), keyRef,
		string(cryptolib.AlgorithmRSAOAEP256), encParamsMap("A256GCM"), nil)
	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)
	require.NotNil(t, details)
	assert.NotEmpty(t, details.CEK)
}

func TestEncrypt_ECDHES_WithPublicKeyOnly(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	svc := &runtimeCryptoService{}
	keyRef := &providers.KeyRef{PublicKey: &ecKey.PublicKey}

	_, details, err := svc.Encrypt(context.Background(), keyRef,
		string(cryptolib.AlgorithmECDHES), encParamsMap("A128GCM"), nil)
	require.NoError(t, err)
	assert.NotNil(t, details)
}

func TestEncrypt_NoKeyIDOrPublicKey(t *testing.T) {
	svc := &runtimeCryptoService{}
	keyRef := &providers.KeyRef{}

	_, _, err := svc.Encrypt(context.Background(), keyRef,
		string(cryptolib.AlgorithmRSAOAEP256), encParamsMap("A256GCM"), nil)
	assert.EqualError(t, err, "keyRef has neither a key ID nor a public key")
}

// Encrypt – ECDH-ES variants
func TestEncrypt_ECDHES_NilKeyRef(t *testing.T) {
	svc := &runtimeCryptoService{pkiService: pkimock.NewPKIServiceInterfaceMock(t)}

	_, _, err := svc.Encrypt(context.Background(), nil,
		string(cryptolib.AlgorithmECDHES), encParamsMap("A256GCM"), []byte("data"))
	assert.EqualError(t, err, "keyRef required for ECDH-ES")
}

func TestEncrypt_ECDHES_PKIError(t *testing.T) {
	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetX509Certificate(mock.Anything, "key1").Return(nil, &tidcommon.InternalServerError)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &providers.KeyRef{KeyID: "key1"}

	_, _, err := svc.Encrypt(context.Background(), keyRef,
		string(cryptolib.AlgorithmECDHES), encParamsMap("A256GCM"), []byte("data"))
	assert.Error(t, err)
}

func TestEncrypt_ECDHES_NonECPublicKey(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetX509Certificate(mock.Anything, "key1").Return(
		&x509.Certificate{PublicKey: &rsaKey.PublicKey}, nil,
	)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &providers.KeyRef{KeyID: "key1"}

	_, _, err = svc.Encrypt(context.Background(), keyRef,
		string(cryptolib.AlgorithmECDHES), encParamsMap("A256GCM"), nil)
	assert.EqualError(t, err, "key is not an EC public key")
}

func TestEncrypt_ECDHESVariants_Success(t *testing.T) {
	algorithms := []cryptolib.Algorithm{
		cryptolib.AlgorithmECDHES,
		cryptolib.AlgorithmECDHESA128KW,
		cryptolib.AlgorithmECDHESA192KW,
		cryptolib.AlgorithmECDHESA256KW,
	}

	for _, alg := range algorithms {
		t.Run(string(alg), func(t *testing.T) {
			ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			require.NoError(t, err)

			pki := pkimock.NewPKIServiceInterfaceMock(t)
			pki.EXPECT().GetX509Certificate(mock.Anything, "key1").Return(
				&x509.Certificate{PublicKey: &ecKey.PublicKey}, nil,
			)

			svc := &runtimeCryptoService{pkiService: pki}
			keyRef := &providers.KeyRef{KeyID: "key1"}

			_, details, err := svc.Encrypt(context.Background(), keyRef, string(alg), encParamsMap("A256GCM"), nil)
			require.NoError(t, err)
			require.NotNil(t, details)
			assert.NotNil(t, details.EPK)
			assert.NotEmpty(t, details.CEK)
		})
	}
}

// Decrypt – RSA-OAEP-256
func TestDecrypt_RSAOAEP256_NilKeyRef(t *testing.T) {
	svc := &runtimeCryptoService{pkiService: pkimock.NewPKIServiceInterfaceMock(t)}

	_, err := svc.Decrypt(context.Background(), nil, string(cryptolib.AlgorithmRSAOAEP256), nil, []byte("ciphertext"))
	assert.EqualError(t, err, "keyRef required for RSA-OAEP-256")
}

func TestDecrypt_RSAOAEP256_PKIError(t *testing.T) {
	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetPrivateKey(mock.Anything, "key1").Return(nil, &tidcommon.InternalServerError)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &providers.KeyRef{KeyID: "key1"}

	_, err := svc.Decrypt(
		context.Background(), keyRef, string(cryptolib.AlgorithmRSAOAEP256), nil, []byte("ciphertext"),
	)
	assert.Error(t, err)
}

func TestDecrypt_RSAOAEP256_NonRSAPrivateKey(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetPrivateKey(mock.Anything, "key1").Return(ecKey, nil)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &providers.KeyRef{KeyID: "key1"}

	_, err = svc.Decrypt(context.Background(), keyRef, string(cryptolib.AlgorithmRSAOAEP256), nil, []byte("ciphertext"))
	assert.EqualError(t, err, "key is not an RSA private key")
}

// Decrypt – ECDH-ES variants
func TestDecrypt_ECDHES_NilKeyRef(t *testing.T) {
	svc := &runtimeCryptoService{pkiService: pkimock.NewPKIServiceInterfaceMock(t)}

	_, err := svc.Decrypt(context.Background(), nil, string(cryptolib.AlgorithmECDHES), nil, nil)
	assert.EqualError(t, err, "keyRef required for ECDH-ES")
}

func TestDecrypt_ECDHES_PKIError(t *testing.T) {
	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetPrivateKey(mock.Anything, "key1").Return(nil, &tidcommon.InternalServerError)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &providers.KeyRef{KeyID: "key1"}

	_, err := svc.Decrypt(context.Background(), keyRef, string(cryptolib.AlgorithmECDHES), nil, nil)
	assert.Error(t, err)
}

func TestDecrypt_ECDHES_NonECPrivateKey(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetPrivateKey(mock.Anything, "key1").Return(rsaKey, nil)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &providers.KeyRef{KeyID: "key1"}

	_, err = svc.Decrypt(context.Background(), keyRef, string(cryptolib.AlgorithmECDHES), nil, nil)
	assert.EqualError(t, err, "key is not an EC private key")
}

func TestDecrypt_ECDHES_InvalidEPKJWK(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetPrivateKey(mock.Anything, "key1").Return(ecKey, nil)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &providers.KeyRef{KeyID: "key1"}

	// A malformed epk JWK (missing kty) must surface a parse error rather than being
	// silently treated as an absent epk.
	badEPK := map[string]interface{}{"crv": "P-256", "x": "x", "y": "y"}
	_, err = svc.Decrypt(context.Background(), keyRef, string(cryptolib.AlgorithmECDHES),
		decParamsMap(badEPK), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWK missing kty")
}

func TestDecrypt_ECDHES_EPKWrongKeyType(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetPrivateKey(mock.Anything, "key1").Return(ecKey, nil)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &providers.KeyRef{KeyID: "key1"}

	// epk resolves to a non-EC public key type; must surface an error instead of nil.
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	eBytes := big.NewInt(int64(rsaKey.PublicKey.E)).Bytes()
	badEPK := map[string]interface{}{
		"kty": "RSA",
		"n":   base64.RawURLEncoding.EncodeToString(rsaKey.PublicKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(eBytes),
	}
	_, err = svc.Decrypt(context.Background(), keyRef, string(cryptolib.AlgorithmECDHES),
		decParamsMap(badEPK), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "epk is not an EC public key")
}

func TestDecrypt_ECDHESVariants_RoundTrip(t *testing.T) {
	algorithms := []cryptolib.Algorithm{
		cryptolib.AlgorithmECDHES,
		cryptolib.AlgorithmECDHESA128KW,
		cryptolib.AlgorithmECDHESA192KW,
		cryptolib.AlgorithmECDHESA256KW,
	}

	for _, alg := range algorithms {
		t.Run(string(alg), func(t *testing.T) {
			ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			require.NoError(t, err)

			pki := pkimock.NewPKIServiceInterfaceMock(t)
			pki.EXPECT().GetX509Certificate(mock.Anything, "key1").Return(
				&x509.Certificate{PublicKey: &ecKey.PublicKey}, nil,
			)
			pki.EXPECT().GetPrivateKey(mock.Anything, "key1").Return(ecKey, nil)

			svc := &runtimeCryptoService{pkiService: pki}
			keyRef := &providers.KeyRef{KeyID: "key1"}

			ciphertext, encDetails, err := svc.Encrypt(
				context.Background(), keyRef, string(alg), encParamsMap("A256GCM"), nil,
			)
			require.NoError(t, err)
			require.NotNil(t, encDetails)

			derivedCEK, err := svc.Decrypt(
				context.Background(), keyRef, string(alg), decParamsMap(encDetails.EPK), ciphertext,
			)
			require.NoError(t, err)
			assert.Equal(t, encDetails.CEK, derivedCEK)
		})
	}
}

// TestDecrypt_ECDHES_EPKAsJWKMap_RoundTrip mirrors the real JWE flow (jwe/service.go), where the
// ephemeral public key arrives as a JWK map decoded from the JWE header's "epk" field, rather than
// as an already-typed crypto.PublicKey. This exercises the JWKToPublicKey resolution branch of
// epkFromParam.
func TestDecrypt_ECDHES_EPKAsJWKMap_RoundTrip(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetX509Certificate(mock.Anything, "key1").Return(
		&x509.Certificate{PublicKey: &ecKey.PublicKey}, nil,
	)
	pki.EXPECT().GetPrivateKey(mock.Anything, "key1").Return(ecKey, nil)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &providers.KeyRef{KeyID: "key1"}

	ciphertext, encDetails, err := svc.Encrypt(
		context.Background(), keyRef, string(cryptolib.AlgorithmECDHES), encParamsMap("A256GCM"), nil,
	)
	require.NoError(t, err)
	require.NotNil(t, encDetails)

	ephemeralPub, ok := encDetails.EPK.(*ecdh.PublicKey)
	require.True(t, ok)
	raw := ephemeralPub.Bytes()
	epkMap := map[string]interface{}{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(raw[1:33]),
		"y":   base64.RawURLEncoding.EncodeToString(raw[33:]),
	}

	derivedCEK, err := svc.Decrypt(
		context.Background(), keyRef, string(cryptolib.AlgorithmECDHES), decParamsMap(epkMap), ciphertext,
	)
	require.NoError(t, err)
	assert.Equal(t, encDetails.CEK, derivedCEK)
}

// GetPublicKeys

func TestEncrypt_RSAOAEP_Success(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetX509Certificate(mock.Anything, "key1").Return(
		&x509.Certificate{PublicKey: &rsaKey.PublicKey}, nil,
	)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &providers.KeyRef{KeyID: "key1"}

	ciphertext, details, err := svc.Encrypt(context.Background(), keyRef,
		string(cryptolib.AlgorithmRSAOAEP), encParamsMap("A256GCM"), nil)
	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)
	require.NotNil(t, details)
	assert.NotEmpty(t, details.CEK)
}

func TestDecrypt_RSAVariants_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		alg  cryptolib.Algorithm
	}{
		{"RSA-OAEP-256", cryptolib.AlgorithmRSAOAEP256},
		{"RSA-OAEP", cryptolib.AlgorithmRSAOAEP},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
			require.NoError(t, err)

			pki := pkimock.NewPKIServiceInterfaceMock(t)
			pki.EXPECT().GetX509Certificate(mock.Anything, "key1").Return(
				&x509.Certificate{PublicKey: &rsaKey.PublicKey}, nil,
			)
			pki.EXPECT().GetPrivateKey(mock.Anything, "key1").Return(rsaKey, nil)

			svc := &runtimeCryptoService{pkiService: pki}
			keyRef := &providers.KeyRef{KeyID: "key1"}

			wrappedCEK, details, err := svc.Encrypt(
				context.Background(), keyRef, string(tc.alg), encParamsMap("A256GCM"), nil,
			)
			require.NoError(t, err)
			require.NotNil(t, details)

			unwrappedCEK, err := svc.Decrypt(context.Background(), keyRef, string(tc.alg), nil, wrappedCEK)
			require.NoError(t, err)
			assert.Equal(t, details.CEK, unwrappedCEK)
		})
	}
}

// GetPublicKeys

func TestGetPublicKeys_NilPKIService(t *testing.T) {
	svc := &runtimeCryptoService{pkiService: nil}
	_, err := svc.GetPublicKeys(context.Background(), providers.PublicKeyFilter{})
	assert.EqualError(t, err, "PKI service not initialized")
}

func TestGetPublicKeys_GetAllX509CertificatesError(t *testing.T) {
	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetAllX509Certificates(mock.Anything).Return(nil, &tidcommon.InternalServerError)

	svc := &runtimeCryptoService{pkiService: pki, logger: newTestLogger()}
	_, err := svc.GetPublicKeys(context.Background(), providers.PublicKeyFilter{})
	assert.Error(t, err)
}

func TestGetPublicKeys_RSA(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetAllX509Certificates(mock.Anything).Return(
		map[string]*x509.Certificate{"key1": {Raw: []byte("der"), PublicKey: &rsaKey.PublicKey}}, nil,
	)
	pki.EXPECT().GetCertThumbprint("key1").Return("thumbprint-1")
	pki.EXPECT().GetCertificateChain("key1").Return(nil)

	svc := &runtimeCryptoService{pkiService: pki, logger: newTestLogger()}
	keys, err := svc.GetPublicKeys(context.Background(), providers.PublicKeyFilter{})
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, "thumbprint-1", keys[0].KeyID)
	assert.Equal(t, string(cryptolib.AlgorithmRS256), keys[0].Algorithm)
	assert.Equal(t, &rsaKey.PublicKey, keys[0].PublicKey)
	assert.Equal(t, "thumbprint-1", keys[0].Thumbprint)
	assert.Equal(t, []byte("der"), keys[0].CertificateDER)
}

func TestGetPublicKeys_ECDSA(t *testing.T) {
	tests := []struct {
		name  string
		curve elliptic.Curve
		alg   cryptolib.Algorithm
	}{
		{"P-256", elliptic.P256(), cryptolib.AlgorithmES256},
		{"P-384", elliptic.P384(), cryptolib.AlgorithmES384},
		{"P-521", elliptic.P521(), cryptolib.AlgorithmES512},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ecKey, err := ecdsa.GenerateKey(tt.curve, rand.Reader)
			require.NoError(t, err)

			pki := pkimock.NewPKIServiceInterfaceMock(t)
			pki.EXPECT().GetAllX509Certificates(mock.Anything).Return(
				map[string]*x509.Certificate{"key1": {Raw: []byte("der"), PublicKey: &ecKey.PublicKey}}, nil,
			)
			pki.EXPECT().GetCertThumbprint("key1").Return("tp")
			pki.EXPECT().GetCertificateChain("key1").Return(nil)

			svc := &runtimeCryptoService{pkiService: pki, logger: newTestLogger()}
			keys, err := svc.GetPublicKeys(context.Background(), providers.PublicKeyFilter{})
			require.NoError(t, err)
			require.Len(t, keys, 1)
			assert.Equal(t, string(tt.alg), keys[0].Algorithm)
		})
	}
}

func TestGetPublicKeys_EdDSA(t *testing.T) {
	_, edPriv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetAllX509Certificates(mock.Anything).Return(
		map[string]*x509.Certificate{"key1": {Raw: []byte("der"), PublicKey: edPriv.Public()}}, nil,
	)
	pki.EXPECT().GetCertThumbprint("key1").Return("tp")
	pki.EXPECT().GetCertificateChain("key1").Return(nil)

	svc := &runtimeCryptoService{pkiService: pki, logger: newTestLogger()}
	keys, err := svc.GetPublicKeys(context.Background(), providers.PublicKeyFilter{})
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, string(cryptolib.AlgorithmEdDSA), keys[0].Algorithm)
}

func TestGetPublicKeys_UnsupportedKeyTypeSkipped(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetAllX509Certificates(mock.Anything).Return(
		map[string]*x509.Certificate{
			"good": {Raw: []byte("der"), PublicKey: &rsaKey.PublicKey},
			"bad":  {PublicKey: "unsupported"},
		}, nil,
	)
	pki.EXPECT().GetCertThumbprint("good").Return("tp")
	pki.EXPECT().GetCertificateChain("good").Return(nil)

	svc := &runtimeCryptoService{pkiService: pki, logger: newTestLogger()}
	keys, err := svc.GetPublicKeys(context.Background(), providers.PublicKeyFilter{})
	require.NoError(t, err)
	assert.Len(t, keys, 1)
	assert.Equal(t, "tp", keys[0].KeyID)
}

func TestGetPublicKeys_FilterByKeyID(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetAllX509Certificates(mock.Anything).Return(
		map[string]*x509.Certificate{
			"rsa-key": {Raw: []byte("rsa-der"), PublicKey: &rsaKey.PublicKey},
			"ec-key":  {Raw: []byte("ec-der"), PublicKey: &ecKey.PublicKey},
		}, nil,
	)
	pki.EXPECT().GetCertThumbprint("rsa-key").Return("rsa-tp")
	pki.EXPECT().GetCertificateChain("rsa-key").Return(nil)

	svc := &runtimeCryptoService{pkiService: pki, logger: newTestLogger()}
	keys, err := svc.GetPublicKeys(context.Background(), providers.PublicKeyFilter{KeyID: "rsa-key"})
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, "rsa-tp", keys[0].KeyID)
}

func TestGetPublicKeys_FilterByAlgorithm(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetAllX509Certificates(mock.Anything).Return(
		map[string]*x509.Certificate{
			"rsa-key": {Raw: []byte("rsa-der"), PublicKey: &rsaKey.PublicKey},
			"ec-key":  {Raw: []byte("ec-der"), PublicKey: &ecKey.PublicKey},
		}, nil,
	)
	pki.EXPECT().GetCertThumbprint("ec-key").Return("ec-tp")
	pki.EXPECT().GetCertificateChain("ec-key").Return(nil)

	svc := &runtimeCryptoService{pkiService: pki, logger: newTestLogger()}
	keys, err := svc.GetPublicKeys(context.Background(),
		providers.PublicKeyFilter{Algorithm: string(cryptolib.AlgorithmES256)})
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, "ec-tp", keys[0].KeyID)
}

// Sign

func TestSign_NilPKIService(t *testing.T) {
	svc := &runtimeCryptoService{pkiService: nil}
	_, err := svc.Sign(context.Background(), providers.KeyRef{KeyID: testKeyID}, "ES256", []byte("data"))
	assert.EqualError(t, err, "PKI service not initialized")
}

func TestSign_UnsupportedAlgorithm(t *testing.T) {
	pki := pkimock.NewPKIServiceInterfaceMock(t)
	svc := &runtimeCryptoService{pkiService: pki, logger: newTestLogger()}
	_, err := svc.Sign(context.Background(), providers.KeyRef{KeyID: testKeyID}, "none", []byte("data"))
	assert.ErrorIs(t, err, providers.ErrUnsupportedAlgorithm)
}

func TestSign_KeyNotFound(t *testing.T) {
	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetPrivateKey(mock.Anything, testKeyID).Return(nil, newTestSvcErr())

	svc := &runtimeCryptoService{pkiService: pki, logger: newTestLogger()}
	_, err := svc.Sign(context.Background(), providers.KeyRef{KeyID: testKeyID}, "ES256", []byte("data"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), testKeyID)
}

func TestSign_Success(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetPrivateKey(mock.Anything, testKeyID).Return(ecKey, nil)

	svc := &runtimeCryptoService{pkiService: pki, logger: newTestLogger()}
	sig, err := svc.Sign(context.Background(), providers.KeyRef{KeyID: testKeyID}, "ES256", []byte("data"))
	require.NoError(t, err)
	assert.NotEmpty(t, sig)
}

// Verify

func TestVerify_NilPKIService(t *testing.T) {
	svc := &runtimeCryptoService{pkiService: nil}
	err := svc.Verify(context.Background(), providers.KeyRef{KeyID: "kid-1"}, "ES256", []byte("data"), []byte("sig"))
	assert.EqualError(t, err, "PKI service not initialized")
}

func TestVerify_UnsupportedAlgorithm(t *testing.T) {
	pki := pkimock.NewPKIServiceInterfaceMock(t)
	svc := &runtimeCryptoService{pkiService: pki, logger: newTestLogger()}
	err := svc.Verify(context.Background(), providers.KeyRef{KeyID: "kid-1"}, "none", []byte("data"), []byte("sig"))
	assert.ErrorIs(t, err, providers.ErrUnsupportedAlgorithm)
}

func TestVerify_GetPublicKeysError(t *testing.T) {
	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetAllX509Certificates(mock.Anything).Return(nil, &common.InternalServerError)

	svc := &runtimeCryptoService{pkiService: pki, logger: newTestLogger()}
	err := svc.Verify(context.Background(), providers.KeyRef{KeyID: "kid-1"}, "ES256", []byte("data"), []byte("sig"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to retrieve public keys")
}

func TestVerify_Success(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	content := []byte("signing input")
	sig, err := cryptolib.Generate(content, cryptolib.ECDSASHA256, ecKey)
	require.NoError(t, err)

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetAllX509Certificates(mock.Anything).Return(
		map[string]*x509.Certificate{"key1": {Raw: []byte("der"), PublicKey: &ecKey.PublicKey}}, nil,
	)
	pki.EXPECT().GetCertThumbprint("key1").Return("kid-1")
	pki.EXPECT().GetCertificateChain("key1").Return(nil)

	svc := &runtimeCryptoService{pkiService: pki, logger: newTestLogger()}
	err = svc.Verify(context.Background(), providers.KeyRef{KeyID: "kid-1"}, "ES256", content, sig)
	assert.NoError(t, err)
}

func TestVerify_PublicKeyJWK_Success(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	content := []byte("signing input")
	sig, err := cryptolib.Generate(content, cryptolib.ECDSASHA256, ecKey)
	require.NoError(t, err)

	xBytes := make([]byte, 32)
	yBytes := make([]byte, 32)
	ecKey.X.FillBytes(xBytes)
	ecKey.Y.FillBytes(yBytes)
	jwk := map[string]interface{}{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(xBytes),
		"y":   base64.RawURLEncoding.EncodeToString(yBytes),
	}

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	svc := &runtimeCryptoService{pkiService: pki, logger: newTestLogger()}
	err = svc.Verify(context.Background(), providers.KeyRef{PublicKeyJWK: jwk}, "ES256", content, sig)
	assert.NoError(t, err)
}

func TestVerify_PublicKeyJWK_InvalidJWK(t *testing.T) {
	pki := pkimock.NewPKIServiceInterfaceMock(t)
	svc := &runtimeCryptoService{pkiService: pki, logger: newTestLogger()}
	jwk := map[string]interface{}{"kty": "UNKNOWN"}
	err := svc.Verify(context.Background(), providers.KeyRef{PublicKeyJWK: jwk}, "ES256", []byte("data"), []byte("sig"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JWK public key")
}

func TestVerify_KeyNotFound(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetAllX509Certificates(mock.Anything).Return(
		map[string]*x509.Certificate{"key1": {Raw: []byte("der"), PublicKey: &ecKey.PublicKey}}, nil,
	)
	pki.EXPECT().GetCertThumbprint("key1").Return("kid-1")
	pki.EXPECT().GetCertificateChain("key1").Return(nil)

	svc := &runtimeCryptoService{pkiService: pki, logger: newTestLogger()}
	err = svc.Verify(context.Background(), providers.KeyRef{KeyID: "other-kid"}, "ES256", []byte("data"), []byte("sig"))
	assert.ErrorIs(t, err, providers.ErrKeyNotFound)
}

// GetTLSMaterial

func TestGetTLSMaterial_NilPKIService(t *testing.T) {
	svc := &runtimeCryptoService{pkiService: nil}
	material, err := svc.GetTLSMaterial(context.Background())
	assert.Nil(t, material)
	assert.EqualError(t, err, "PKI service not initialized")
}

func TestGetTLSMaterial_GetTLSConfigFailure(t *testing.T) {
	pkiMock := pkimock.NewPKIServiceInterfaceMock(t)
	pkiMock.EXPECT().GetTLSConfig().Return(nil, errors.New("cert file not found"))

	svc := &runtimeCryptoService{pkiService: pkiMock}
	material, err := svc.GetTLSMaterial(context.Background())
	assert.Nil(t, material)
	assert.ErrorContains(t, err, "cert file not found")
}

func TestGetTLSMaterial_Success(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tlsCert := tls.Certificate{
		PrivateKey: rsaKey,
	}
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		MinVersion:   tls.VersionTLS12,
	}

	pkiMock := pkimock.NewPKIServiceInterfaceMock(t)
	pkiMock.EXPECT().GetTLSConfig().Return(tlsCfg, nil)

	svc := &runtimeCryptoService{pkiService: pkiMock}
	material, err := svc.GetTLSMaterial(context.Background())
	require.NoError(t, err)
	require.NotNil(t, material)
	assert.Equal(t, tlsCert, material.Certificate)
	assert.Equal(t, uint16(tls.VersionTLS12), material.MinVersion)
}

// TestMLDSA_RuntimeSignVerifyAndGetPublicKeys exercises the full ML-DSA path
// through the runtime provider: GetPublicKeys derives the public key from the
// private key (the certificate carries no parseable public key), and a signature
// produced by Sign verifies through Verify.
func TestMLDSA_RuntimeSignVerifyAndGetPublicKeys(t *testing.T) {
	signer, err := cryptolib.GenerateMLDSAKey(cryptolib.AlgorithmMLDSA65)
	require.NoError(t, err)

	const keyID = "mldsa-key"
	const thumbprint = "mldsa-thumbprint"
	cert := &x509.Certificate{Raw: []byte("mldsa-cert-raw")} // PublicKey is nil for ML-DSA.

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetAllX509Certificates(mock.Anything).
		Return(map[string]*x509.Certificate{keyID: cert}, nil)
	pki.EXPECT().GetPrivateKey(mock.Anything, keyID).Return(signer, nil)
	pki.EXPECT().GetCertThumbprint(keyID).Return(thumbprint)
	pki.EXPECT().GetCertificateChain(keyID).Return([][]byte{cert.Raw})

	svc := &runtimeCryptoService{pkiService: pki, logger: newTestLogger()}
	ctx := context.Background()

	keys, err := svc.GetPublicKeys(ctx, providers.PublicKeyFilter{})
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, string(cryptolib.AlgorithmMLDSA65), keys[0].Algorithm)
	assert.Equal(t, thumbprint, keys[0].Thumbprint)

	data := []byte("header.payload")
	sig, err := svc.Sign(ctx, providers.KeyRef{KeyID: keyID}, string(cryptolib.AlgorithmMLDSA65), data)
	require.NoError(t, err)
	assert.NoError(
		t, svc.Verify(ctx, providers.KeyRef{KeyID: thumbprint}, string(cryptolib.AlgorithmMLDSA65), data, sig),
	)
}

// TestGetPublicKeys_DerivePublicKeyPrivateKeyError covers the case where the
// certificate carries no parseable public key (nil, as for ML-DSA) and the
// configured private key cannot be retrieved either.
func TestGetPublicKeys_DerivePublicKeyPrivateKeyError(t *testing.T) {
	const keyID = "mldsa-key"
	cert := &x509.Certificate{Raw: []byte("mldsa-cert-raw")}

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetAllX509Certificates(mock.Anything).
		Return(map[string]*x509.Certificate{keyID: cert}, nil)
	pki.EXPECT().GetPrivateKey(mock.Anything, keyID).Return(nil, newTestSvcErr())

	svc := &runtimeCryptoService{pkiService: pki, logger: newTestLogger()}
	keys, err := svc.GetPublicKeys(context.Background(), providers.PublicKeyFilter{})
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestGetSupportedSigningAlgorithms(t *testing.T) {
	svc := &runtimeCryptoService{}
	algs := svc.GetSupportedSigningAlgorithms()
	assert.ElementsMatch(t, []string{
		string(RS256), string(RS512), string(PS256),
		string(ES256), string(ES384), string(ES512),
		string(EdDSA), string(MLDSA44), string(MLDSA65), string(MLDSA87),
	}, algs)
}

func TestGetSupportedEncryptionAlgorithms(t *testing.T) {
	svc := &runtimeCryptoService{}
	algs := svc.GetSupportedEncryptionAlgorithms()
	assert.ElementsMatch(t, []string{
		string(cryptolib.AlgorithmAESGCM),
		string(cryptolib.AlgorithmRSAOAEP), string(cryptolib.AlgorithmRSAOAEP256),
		string(cryptolib.AlgorithmECDHES),
		string(cryptolib.AlgorithmECDHESA128KW),
		string(cryptolib.AlgorithmECDHESA192KW),
		string(cryptolib.AlgorithmECDHESA256KW),
	}, algs)
}

// TestGetPublicKeys_DerivePublicKeyNonSignerPrivateKey covers the case where the
// configured private key does not implement crypto.Signer.
func TestGetPublicKeys_DerivePublicKeyNonSignerPrivateKey(t *testing.T) {
	const keyID = "mldsa-key"
	cert := &x509.Certificate{Raw: []byte("mldsa-cert-raw")}

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetAllX509Certificates(mock.Anything).
		Return(map[string]*x509.Certificate{keyID: cert}, nil)
	pki.EXPECT().GetPrivateKey(mock.Anything, keyID).Return([]byte("not-a-signer"), nil)

	svc := &runtimeCryptoService{pkiService: pki, logger: newTestLogger()}
	keys, err := svc.GetPublicKeys(context.Background(), providers.PublicKeyFilter{})
	require.NoError(t, err)
	assert.Empty(t, keys)
}

type JWKTestSuite struct {
	suite.Suite
	rsaPublicKey *rsa.PublicKey
	ecPublicKey  *ecdsa.PublicKey
	edPublicKey  ed25519.PublicKey
}

func TestJWKTestSuite(t *testing.T) {
	suite.Run(t, new(JWKTestSuite))
}

func (suite *JWKTestSuite) SetupSuite() {
	rsaPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(suite.T(), err)
	suite.rsaPublicKey = &rsaPrivateKey.PublicKey

	ecPrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(suite.T(), err)
	suite.ecPublicKey = &ecPrivateKey.PublicKey

	suite.edPublicKey, _, err = ed25519.GenerateKey(rand.Reader)
	require.NoError(suite.T(), err)
}

func (suite *JWKTestSuite) TestJWKToPublicKeyRSA() {
	n := suite.rsaPublicKey.N
	e := suite.rsaPublicKey.E

	jwk := map[string]interface{}{
		"kty": "RSA",
		"n":   base64.RawURLEncoding.EncodeToString(n.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(new(big.Int).SetInt64(int64(e)).Bytes()),
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), publicKey)
	rsaKey, ok := publicKey.(*rsa.PublicKey)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), n, rsaKey.N)
	assert.Equal(suite.T(), e, rsaKey.E)
}

func (suite *JWKTestSuite) TestJWKToPublicKeyEd25519() {
	jwk := map[string]interface{}{
		"kty": "OKP",
		"crv": "Ed25519",
		"x":   base64.RawURLEncoding.EncodeToString(suite.edPublicKey),
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), publicKey)
	edKey, ok := publicKey.(ed25519.PublicKey)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), suite.edPublicKey, edKey)
}

func (suite *JWKTestSuite) TestJWKToPublicKeyMissingKty() {
	jwk := map[string]interface{}{
		"n": "value",
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
	assert.Contains(suite.T(), err.Error(), "JWK missing kty")
}

func (suite *JWKTestSuite) TestJWKToPublicKeyInvalidKty() {
	jwk := map[string]interface{}{
		"kty": "UNKNOWN",
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
	assert.Contains(suite.T(), err.Error(), "unsupported JWK kty")
}

func (suite *JWKTestSuite) TestJWKToRSAPublicKeyMissingModulus() {
	jwk := map[string]interface{}{
		"kty": "RSA",
		"e":   "AQAB",
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
	assert.Contains(suite.T(), err.Error(), "JWK missing RSA modulus or exponent")
}

func (suite *JWKTestSuite) TestJWKToRSAPublicKeyMissingExponent() {
	jwk := map[string]interface{}{
		"kty": "RSA",
		"n":   base64.RawURLEncoding.EncodeToString(suite.rsaPublicKey.N.Bytes()),
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
}

func (suite *JWKTestSuite) TestJWKToRSAPublicKeyInvalidExponent() {
	jwk := map[string]interface{}{
		"kty": "RSA",
		"n":   base64.RawURLEncoding.EncodeToString(suite.rsaPublicKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString([]byte{0, 0, 0, 0}),
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
}

func (suite *JWKTestSuite) TestJWKToRSAPublicKeyInvalidBase64() {
	jwk := map[string]interface{}{
		"kty": "RSA",
		"n":   "invalid!base64",
		"e":   "AQAB",
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
	assert.Contains(suite.T(), err.Error(), "failed to decode RSA modulus")
}

func (suite *JWKTestSuite) TestJWKToECPublicKeyMissingCurve() {
	jwk := map[string]interface{}{
		"kty": "EC",
		"x":   "value",
		"y":   "value",
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
	assert.Contains(suite.T(), err.Error(), "EC JWK missing crv/x/y")
}

func (suite *JWKTestSuite) TestJWKToECPublicKeyUnsupportedCurve() {
	jwk := map[string]interface{}{
		"kty": "EC",
		"crv": "P-999",
		"x":   base64.RawURLEncoding.EncodeToString([]byte("value")),
		"y":   base64.RawURLEncoding.EncodeToString([]byte("value")),
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
	assert.Contains(suite.T(), err.Error(), "unsupported EC curve")
}

func (suite *JWKTestSuite) TestJWKToECPublicKeyInvalidBase64() {
	jwk := map[string]interface{}{
		"kty": "EC",
		"crv": "P-256",
		"x":   "invalid!base64",
		"y":   base64.RawURLEncoding.EncodeToString(make([]byte, 32)),
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
	assert.Contains(suite.T(), err.Error(), "decode EC x")
}

func (suite *JWKTestSuite) TestJWKToECPublicKeyCoordinateExceedsCurveSize() {
	jwk := map[string]interface{}{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(make([]byte, 33)),
		"y":   base64.RawURLEncoding.EncodeToString(make([]byte, 33)),
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
	assert.Contains(suite.T(), err.Error(), "EC coordinate length mismatch")
}

func (suite *JWKTestSuite) TestJWKToECPublicKeyPointNotOnCurve() {
	jwk := map[string]interface{}{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString([]byte("short")),
		"y":   base64.RawURLEncoding.EncodeToString([]byte("short")),
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
	assert.Contains(suite.T(), err.Error(), "EC coordinate length mismatch")
}

func (suite *JWKTestSuite) TestJWKToOKPPublicKeyMissingCurve() {
	jwk := map[string]interface{}{
		"kty": "OKP",
		"x":   base64.RawURLEncoding.EncodeToString(suite.edPublicKey),
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
	assert.Contains(suite.T(), err.Error(), "JWK missing OKP parameters")
}

func (suite *JWKTestSuite) TestJWKToOKPPublicKeyUnsupportedCurve() {
	jwk := map[string]interface{}{
		"kty": "OKP",
		"crv": "X25519",
		"x":   base64.RawURLEncoding.EncodeToString(suite.edPublicKey),
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
	assert.Contains(suite.T(), err.Error(), "unsupported OKP curve")
}

func (suite *JWKTestSuite) TestJWKToOKPPublicKeyInvalidBase64() {
	jwk := map[string]interface{}{
		"kty": "OKP",
		"crv": "Ed25519",
		"x":   "invalid!base64",
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
	assert.Contains(suite.T(), err.Error(), "failed to decode Ed25519 x")
}

func (suite *JWKTestSuite) TestJWKToOKPPublicKeyInvalidKeyLength() {
	jwk := map[string]interface{}{
		"kty": "OKP",
		"crv": "Ed25519",
		"x":   base64.RawURLEncoding.EncodeToString([]byte("short")),
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
	assert.Contains(suite.T(), err.Error(), "invalid Ed25519 public key length")
}

func (suite *JWKTestSuite) TestJWKToPublicKeyEC() {
	xBytes := make([]byte, 32)
	yBytes := make([]byte, 32)
	suite.ecPublicKey.X.FillBytes(xBytes)
	suite.ecPublicKey.Y.FillBytes(yBytes)

	jwk := map[string]interface{}{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(xBytes),
		"y":   base64.RawURLEncoding.EncodeToString(yBytes),
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), publicKey)
	ecKey, ok := publicKey.(*ecdsa.PublicKey)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), suite.ecPublicKey.X, ecKey.X)
	assert.Equal(suite.T(), suite.ecPublicKey.Y, ecKey.Y)
}

func (suite *JWKTestSuite) TestJWKToPublicKeyInvalidEC() {
	// A valid-looking but mismatched (x, y) pair that is not on the P-256 curve.
	validX := new(big.Int).SetBytes([]byte{
		0x6b, 0x17, 0xd1, 0xf2, 0xe1, 0x2c, 0x42, 0x47, 0xf8, 0xbc, 0xe6, 0xe5, 0x63, 0xa4, 0x40, 0xf2,
		0x77, 0x03, 0x7d, 0x81, 0x2d, 0xeb, 0x33, 0xa0, 0xf4, 0xa1, 0x39, 0x45, 0xd8, 0x98, 0xc2, 0x96,
	})
	validY := new(big.Int).SetBytes([]byte{
		0x4f, 0xe3, 0x42, 0xe2, 0xfe, 0x61, 0xa7, 0xf5, 0x73, 0xb3, 0x5b, 0x0b, 0x82, 0x41, 0xa8, 0xc2,
		0x8e, 0x4f, 0xb5, 0x35, 0x86, 0x4a, 0xf3, 0xd4, 0x0f, 0x55, 0xd5, 0x96, 0xb6, 0x7f, 0x4c, 0x8b,
	})

	jwk := map[string]interface{}{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(validX.Bytes()),
		"y":   base64.RawURLEncoding.EncodeToString(validY.Bytes()),
	}

	publicKey, err := JWKToPublicKey(jwk)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "point not on curve")
	assert.Nil(suite.T(), publicKey)
}

func TestJWKToPublicKeyAKPRoundTrip(t *testing.T) {
	for _, alg := range []cryptolib.Algorithm{
		cryptolib.AlgorithmMLDSA44, cryptolib.AlgorithmMLDSA65, cryptolib.AlgorithmMLDSA87,
	} {
		signer, err := cryptolib.GenerateMLDSAKey(alg)
		require.NoError(t, err)
		pubBytes, ok := cryptolib.MLDSAPublicKeyBytes(signer.Public())
		require.True(t, ok)

		jwk := map[string]interface{}{
			"kty": "AKP",
			"alg": string(alg),
			"pub": base64.RawURLEncoding.EncodeToString(pubBytes),
		}

		pub, err := JWKToPublicKey(jwk)
		require.NoError(t, err, "AKP JWK should parse for %s", alg)

		// A signature produced by the signer must verify against the parsed key.
		signAlg, err := cryptolib.SignAlgorithmFor(alg)
		require.NoError(t, err)
		data := []byte("external issuer token")
		sig, err := cryptolib.Generate(data, signAlg, signer)
		require.NoError(t, err)
		assert.NoError(t, cryptolib.Verify(data, sig, signAlg, pub))
	}
}

func TestJWKToPublicKeyAKPMissingMembers(t *testing.T) {
	_, err := JWKToPublicKey(map[string]interface{}{"kty": "AKP", "alg": "ML-DSA-65"})
	assert.Error(t, err)

	_, err = JWKToPublicKey(map[string]interface{}{"kty": "AKP", "pub": "AAAA"})
	assert.Error(t, err)
}

func TestJWKToPublicKeyAKPInvalidPub(t *testing.T) {
	_, err := JWKToPublicKey(map[string]interface{}{
		"kty": "AKP", "alg": "ML-DSA-65", "pub": "not!base64url",
	})
	assert.Error(t, err)

	// Correctly encoded but wrong-length public key.
	_, err = JWKToPublicKey(map[string]interface{}{
		"kty": "AKP", "alg": "ML-DSA-65", "pub": base64.RawURLEncoding.EncodeToString([]byte("too short")),
	})
	assert.Error(t, err)
}
