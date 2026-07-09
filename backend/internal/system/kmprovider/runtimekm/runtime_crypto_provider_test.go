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

package runtimekm

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"testing"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

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

func TestEncrypt_RSAOAEP256_SuccessViaConstructor(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	cert := &x509.Certificate{PublicKey: &rsaKey.PublicKey}

	pkiMock := pkimock.NewPKIServiceInterfaceMock(t)
	pkiMock.EXPECT().
		GetX509Certificate(mock.Anything, testKeyID).
		Return(cert, nil)

	svc := NewRuntimeCryptoService(pkiMock, nil)

	alg, params := cryptolib.AlgorithmParams{
		Algorithm: cryptolib.AlgorithmRSAOAEP256,
		RSAOAEP256: cryptolib.RSAOAEP256Params{
			ContentEncryptionAlgorithm: "A256GCM",
		},
	}.ToParamsMap()
	wrappedCEK, details, err := svc.Encrypt(
		context.Background(), &providers.KeyRef{KeyID: testKeyID}, alg, params, nil,
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

	alg, params := cryptolib.AlgorithmParams{
		Algorithm: cryptolib.AlgorithmRSAOAEP256,
		RSAOAEP256: cryptolib.RSAOAEP256Params{
			ContentEncryptionAlgorithm: "A256GCM",
		},
	}.ToParamsMap()
	_, _, err := svc.Encrypt(context.Background(), &providers.KeyRef{KeyID: testKeyID}, alg, params, []byte("data"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), testKeyID)
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

	alg, params := cryptolib.AlgorithmParams{
		Algorithm: cryptolib.AlgorithmECDHES,
		ECDHES:    cryptolib.ECDHESParams{ContentEncryptionAlgorithm: "A128GCM"},
	}.ToParamsMap()
	_, details, err := svc.Encrypt(context.Background(), &providers.KeyRef{KeyID: testKeyID}, alg, params, nil)
	require.NoError(t, err)
	assert.NotNil(t, details)
}

func TestEncrypt_ECDHES_GetPublicKeyError(t *testing.T) {
	pkiMock := pkimock.NewPKIServiceInterfaceMock(t)
	pkiMock.EXPECT().
		GetX509Certificate(mock.Anything, testKeyID).
		Return(nil, newTestSvcErr())

	svc := NewRuntimeCryptoService(pkiMock, nil)

	alg, params := cryptolib.AlgorithmParams{
		Algorithm: cryptolib.AlgorithmECDHES,
		ECDHES:    cryptolib.ECDHESParams{ContentEncryptionAlgorithm: "A128GCM"},
	}.ToParamsMap()
	_, _, err := svc.Encrypt(context.Background(), &providers.KeyRef{KeyID: testKeyID}, alg, params, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), testKeyID)
}

func TestEncrypt_UnsupportedAlgorithm(t *testing.T) {
	svc := NewRuntimeCryptoService(nil, nil)

	_, _, err := svc.Encrypt(
		context.Background(), &providers.KeyRef{KeyID: testKeyID}, "UNSUPPORTED", nil, []byte("data"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported algorithm")
}

// Encrypt – RSA-OAEP-256
func TestEncrypt_RSAOAEP256_NilKeyRef(t *testing.T) {
	svc := &runtimeCryptoService{pkiService: pkimock.NewPKIServiceInterfaceMock(t)}
	alg, params := cryptolib.AlgorithmParams{
		Algorithm:  cryptolib.AlgorithmRSAOAEP256,
		RSAOAEP256: cryptolib.RSAOAEP256Params{ContentEncryptionAlgorithm: "A256GCM"},
	}.ToParamsMap()

	_, _, err := svc.Encrypt(context.Background(), nil, alg, params, []byte("data"))
	assert.EqualError(t, err, "keyRef required for RSA-OAEP-256")
}

func TestEncrypt_RSAOAEP256_PKIError(t *testing.T) {
	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetX509Certificate(mock.Anything, "key1").Return(nil, &tidcommon.InternalServerError)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &providers.KeyRef{KeyID: "key1"}
	alg, params := cryptolib.AlgorithmParams{
		Algorithm:  cryptolib.AlgorithmRSAOAEP256,
		RSAOAEP256: cryptolib.RSAOAEP256Params{ContentEncryptionAlgorithm: "A256GCM"},
	}.ToParamsMap()

	_, _, err := svc.Encrypt(context.Background(), keyRef, alg, params, []byte("data"))
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
	alg, params := cryptolib.AlgorithmParams{
		Algorithm:  cryptolib.AlgorithmRSAOAEP256,
		RSAOAEP256: cryptolib.RSAOAEP256Params{ContentEncryptionAlgorithm: "A256GCM"},
	}.ToParamsMap()

	_, _, err = svc.Encrypt(context.Background(), keyRef, alg, params, []byte("data"))
	assert.EqualError(t, err, "key is not an RSA public key")
}

//nolint:dupl // Similar test structure but different algorithm variants
func TestEncrypt_RSAVariants_Success(t *testing.T) {
	tests := []struct {
		name   string
		params cryptolib.AlgorithmParams
	}{
		{
			name: "RSA-OAEP-256",
			params: cryptolib.AlgorithmParams{
				Algorithm:  cryptolib.AlgorithmRSAOAEP256,
				RSAOAEP256: cryptolib.RSAOAEP256Params{ContentEncryptionAlgorithm: "A256GCM"},
			},
		},
		{
			name: "RSA-OAEP",
			params: cryptolib.AlgorithmParams{
				Algorithm: cryptolib.AlgorithmRSAOAEP,
				RSAOAEP:   cryptolib.RSAOAEPParams{ContentEncryptionAlgorithm: "A256GCM"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
			require.NoError(t, err)

			pki := pkimock.NewPKIServiceInterfaceMock(t)
			pki.EXPECT().GetX509Certificate(mock.Anything, "key1").Return(
				&x509.Certificate{PublicKey: &rsaKey.PublicKey}, nil,
			)

			svc := &runtimeCryptoService{pkiService: pki}
			keyRef := &providers.KeyRef{KeyID: "key1"}
			alg, params := tc.params.ToParamsMap()

			ciphertext, details, err := svc.Encrypt(context.Background(), keyRef, alg, params, nil)
			require.NoError(t, err)
			assert.NotEmpty(t, ciphertext)
			require.NotNil(t, details)
			assert.NotEmpty(t, details.CEK)
		})
	}
}

// Encrypt – ECDH-ES variants
func TestEncrypt_ECDHES_NilKeyRef(t *testing.T) {
	svc := &runtimeCryptoService{pkiService: pkimock.NewPKIServiceInterfaceMock(t)}
	alg, params := cryptolib.AlgorithmParams{
		Algorithm: cryptolib.AlgorithmECDHES,
		ECDHES:    cryptolib.ECDHESParams{ContentEncryptionAlgorithm: "A256GCM"},
	}.ToParamsMap()

	_, _, err := svc.Encrypt(context.Background(), nil, alg, params, []byte("data"))
	assert.EqualError(t, err, "keyRef required for ECDH-ES")
}

func TestEncrypt_ECDHES_PKIError(t *testing.T) {
	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetX509Certificate(mock.Anything, "key1").Return(nil, &tidcommon.InternalServerError)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &providers.KeyRef{KeyID: "key1"}
	alg, params := cryptolib.AlgorithmParams{
		Algorithm: cryptolib.AlgorithmECDHES,
		ECDHES:    cryptolib.ECDHESParams{ContentEncryptionAlgorithm: "A256GCM"},
	}.ToParamsMap()

	_, _, err := svc.Encrypt(context.Background(), keyRef, alg, params, []byte("data"))
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
	alg, params := cryptolib.AlgorithmParams{
		Algorithm: cryptolib.AlgorithmECDHES,
		ECDHES:    cryptolib.ECDHESParams{ContentEncryptionAlgorithm: "A256GCM"},
	}.ToParamsMap()

	_, _, err = svc.Encrypt(context.Background(), keyRef, alg, params, nil)
	assert.EqualError(t, err, "key is not an EC public key")
}

func TestEncrypt_ECDHESVariants_Success(t *testing.T) {
	algorithms := []cryptolib.Algorithm{
		cryptolib.AlgorithmECDHES,
		cryptolib.AlgorithmECDHESA128KW,
		cryptolib.AlgorithmECDHESA192KW,
		cryptolib.AlgorithmECDHESA256KW,
	}

	for _, algorithm := range algorithms {
		t.Run(string(algorithm), func(t *testing.T) {
			ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			require.NoError(t, err)

			pki := pkimock.NewPKIServiceInterfaceMock(t)
			pki.EXPECT().GetX509Certificate(mock.Anything, "key1").Return(
				&x509.Certificate{PublicKey: &ecKey.PublicKey}, nil,
			)

			svc := &runtimeCryptoService{pkiService: pki}
			keyRef := &providers.KeyRef{KeyID: "key1"}
			alg, params := cryptolib.AlgorithmParams{
				Algorithm: algorithm,
				ECDHES:    cryptolib.ECDHESParams{ContentEncryptionAlgorithm: "A256GCM"},
			}.ToParamsMap()

			_, details, err := svc.Encrypt(context.Background(), keyRef, alg, params, nil)
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
	alg, params := cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmRSAOAEP256}.ToParamsMap()

	_, err := svc.Decrypt(context.Background(), nil, alg, params, []byte("ciphertext"))
	assert.EqualError(t, err, "keyRef required for RSA-OAEP-256")
}

func TestDecrypt_RSAOAEP256_PKIError(t *testing.T) {
	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetPrivateKey(mock.Anything, "key1").Return(nil, &tidcommon.InternalServerError)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &providers.KeyRef{KeyID: "key1"}
	alg, params := cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmRSAOAEP256}.ToParamsMap()

	_, err := svc.Decrypt(context.Background(), keyRef, alg, params, []byte("ciphertext"))
	assert.Error(t, err)
}

func TestDecrypt_RSAOAEP256_NonRSAPrivateKey(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetPrivateKey(mock.Anything, "key1").Return(ecKey, nil)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &providers.KeyRef{KeyID: "key1"}
	alg, params := cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmRSAOAEP256}.ToParamsMap()

	_, err = svc.Decrypt(context.Background(), keyRef, alg, params, []byte("ciphertext"))
	assert.EqualError(t, err, "key is not an RSA private key")
}

// Decrypt – ECDH-ES variants
func TestDecrypt_ECDHES_NilKeyRef(t *testing.T) {
	svc := &runtimeCryptoService{pkiService: pkimock.NewPKIServiceInterfaceMock(t)}
	alg, params := cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmECDHES}.ToParamsMap()

	_, err := svc.Decrypt(context.Background(), nil, alg, params, nil)
	assert.EqualError(t, err, "keyRef required for ECDH-ES")
}

func TestDecrypt_ECDHES_PKIError(t *testing.T) {
	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetPrivateKey(mock.Anything, "key1").Return(nil, &tidcommon.InternalServerError)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &providers.KeyRef{KeyID: "key1"}
	alg, params := cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmECDHES}.ToParamsMap()

	_, err := svc.Decrypt(context.Background(), keyRef, alg, params, nil)
	assert.Error(t, err)
}

func TestDecrypt_ECDHES_NonECPrivateKey(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetPrivateKey(mock.Anything, "key1").Return(rsaKey, nil)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &providers.KeyRef{KeyID: "key1"}
	alg, params := cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmECDHES}.ToParamsMap()

	_, err = svc.Decrypt(context.Background(), keyRef, alg, params, nil)
	assert.EqualError(t, err, "key is not an EC private key")
}

//nolint:dupl // Similar test structure but different algorithm variants
func TestDecrypt_ECDHESVariants_RoundTrip(t *testing.T) {
	algorithms := []cryptolib.Algorithm{
		cryptolib.AlgorithmECDHES,
		cryptolib.AlgorithmECDHESA128KW,
		cryptolib.AlgorithmECDHESA192KW,
		cryptolib.AlgorithmECDHESA256KW,
	}

	for _, algorithm := range algorithms {
		t.Run(string(algorithm), func(t *testing.T) {
			ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			require.NoError(t, err)

			pki := pkimock.NewPKIServiceInterfaceMock(t)
			pki.EXPECT().GetX509Certificate(mock.Anything, "key1").Return(
				&x509.Certificate{PublicKey: &ecKey.PublicKey}, nil,
			)
			pki.EXPECT().GetPrivateKey(mock.Anything, "key1").Return(ecKey, nil)

			svc := &runtimeCryptoService{pkiService: pki}
			keyRef := &providers.KeyRef{KeyID: "key1"}

			encAlg, encParams := cryptolib.AlgorithmParams{
				Algorithm: algorithm,
				ECDHES:    cryptolib.ECDHESParams{ContentEncryptionAlgorithm: "A256GCM"},
			}.ToParamsMap()
			ciphertext, encDetails, err := svc.Encrypt(context.Background(), keyRef, encAlg, encParams, nil)
			require.NoError(t, err)
			require.NotNil(t, encDetails)

			decAlg, decParams := cryptolib.AlgorithmParams{
				Algorithm: algorithm,
				ECDHES:    cryptolib.ECDHESParams{EPK: encDetails.EPK, ContentEncryptionAlgorithm: "A256GCM"},
			}.ToParamsMap()
			derivedCEK, err := svc.Decrypt(context.Background(), keyRef, decAlg, decParams, ciphertext)
			require.NoError(t, err)
			assert.Equal(t, encDetails.CEK, derivedCEK)
		})
	}
}

// GetPublicKeys

func TestDecrypt_RSAVariants_RoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		encParams cryptolib.AlgorithmParams
		decParams cryptolib.AlgorithmParams
	}{
		{
			name: "RSA-OAEP-256",
			encParams: cryptolib.AlgorithmParams{
				Algorithm:  cryptolib.AlgorithmRSAOAEP256,
				RSAOAEP256: cryptolib.RSAOAEP256Params{ContentEncryptionAlgorithm: "A256GCM"},
			},
			decParams: cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmRSAOAEP256},
		},
		{
			name: "RSA-OAEP",
			encParams: cryptolib.AlgorithmParams{
				Algorithm: cryptolib.AlgorithmRSAOAEP,
				RSAOAEP:   cryptolib.RSAOAEPParams{ContentEncryptionAlgorithm: "A256GCM"},
			},
			decParams: cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmRSAOAEP},
		},
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

			encAlg, encParams := tc.encParams.ToParamsMap()
			wrappedCEK, details, err := svc.Encrypt(context.Background(), keyRef, encAlg, encParams, nil)
			require.NoError(t, err)
			require.NotNil(t, details)

			decAlg, decParams := tc.decParams.ToParamsMap()
			unwrappedCEK, err := svc.Decrypt(context.Background(), keyRef, decAlg, decParams, wrappedCEK)
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
	assert.Equal(t, "key1", keys[0].KeyID)
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
	assert.Equal(t, "good", keys[0].KeyID)
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
	assert.Equal(t, "rsa-key", keys[0].KeyID)
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
	assert.Equal(t, "ec-key", keys[0].KeyID)
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

// Encrypt/Decrypt – AES-GCM

func TestEncrypt_AESGCM_NilCfgService(t *testing.T) {
	svc := &runtimeCryptoService{cfgService: nil}
	alg, params := cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmAESGCM}.ToParamsMap()

	_, _, err := svc.Encrypt(context.Background(), nil, alg, params, []byte("data"))
	assert.EqualError(t, err, "config crypto service not initialized")
}

func TestEncrypt_AESGCM_Success(t *testing.T) {
	cfgMock := cryptomock.NewConfigCryptoProviderMock(t)
	cfgMock.EXPECT().Encrypt(mock.Anything, []byte("data")).Return([]byte("encrypted"), nil)

	svc := &runtimeCryptoService{cfgService: cfgMock}
	alg, params := cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmAESGCM}.ToParamsMap()

	encrypted, details, err := svc.Encrypt(context.Background(), nil, alg, params, []byte("data"))
	require.NoError(t, err)
	assert.Equal(t, []byte("encrypted"), encrypted)
	assert.Nil(t, details)
}

func TestDecrypt_AESGCM_NilCfgService(t *testing.T) {
	svc := &runtimeCryptoService{cfgService: nil}
	alg, params := cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmAESGCM}.ToParamsMap()

	_, err := svc.Decrypt(context.Background(), nil, alg, params, []byte("ciphertext"))
	assert.EqualError(t, err, "config crypto service not initialized")
}

func TestDecrypt_AESGCM_Success(t *testing.T) {
	cfgMock := cryptomock.NewConfigCryptoProviderMock(t)
	cfgMock.EXPECT().Decrypt(mock.Anything, []byte("ciphertext")).Return([]byte("plaintext"), nil)

	svc := &runtimeCryptoService{cfgService: cfgMock}
	alg, params := cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmAESGCM}.ToParamsMap()

	decrypted, err := svc.Decrypt(context.Background(), nil, alg, params, []byte("ciphertext"))
	require.NoError(t, err)
	assert.Equal(t, []byte("plaintext"), decrypted)
}

func TestDecrypt_InvalidAlgorithmParams(t *testing.T) {
	svc := &runtimeCryptoService{}
	_, err := svc.Decrypt(context.Background(), nil, "UNSUPPORTED", nil, []byte("ciphertext"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid algorithm parameters")
}

func TestEncrypt_AlgorithmNotHandledByEncrypt(t *testing.T) {
	svc := &runtimeCryptoService{}
	alg, params := cryptolib.AlgorithmParams{
		Algorithm: cryptolib.AlgorithmA128KW,
		AESKW:     cryptolib.AESKWParams{ContentEncryptionAlgorithm: "A256GCM"},
	}.ToParamsMap()

	_, _, err := svc.Encrypt(context.Background(), nil, alg, params, []byte("data"))
	assert.EqualError(t, err, "unsupported algorithm: A128KW")
}

func TestDecrypt_AlgorithmNotHandledByDecrypt(t *testing.T) {
	svc := &runtimeCryptoService{}
	alg, params := cryptolib.AlgorithmParams{
		Algorithm: cryptolib.AlgorithmA128KW,
		AESKW:     cryptolib.AESKWParams{ContentEncryptionAlgorithm: "A256GCM"},
	}.ToParamsMap()

	_, err := svc.Decrypt(context.Background(), nil, alg, params, []byte("data"))
	assert.EqualError(t, err, "unsupported algorithm: A128KW")
}

// GetPublicKeys – unsupported curve

func TestGetPublicKeys_UnsupportedECCurveSkipped(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	require.NoError(t, err)

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetAllX509Certificates(mock.Anything).Return(
		map[string]*x509.Certificate{"key1": {PublicKey: &ecKey.PublicKey}}, nil,
	)

	svc := &runtimeCryptoService{pkiService: pki, logger: newTestLogger()}
	keys, err := svc.GetPublicKeys(context.Background(), providers.PublicKeyFilter{})
	require.NoError(t, err)
	assert.Empty(t, keys)
}

// Key lookup helpers – nil PKI service

func TestEncrypt_RSAOAEP256_NilPKIService(t *testing.T) {
	svc := &runtimeCryptoService{pkiService: nil}
	alg, params := cryptolib.AlgorithmParams{
		Algorithm:  cryptolib.AlgorithmRSAOAEP256,
		RSAOAEP256: cryptolib.RSAOAEP256Params{ContentEncryptionAlgorithm: "A256GCM"},
	}.ToParamsMap()

	_, _, err := svc.Encrypt(context.Background(), &providers.KeyRef{KeyID: testKeyID}, alg, params, []byte("data"))
	assert.EqualError(t, err, "PKI service not initialized")
}

func TestEncrypt_ECDHES_NilPKIService(t *testing.T) {
	svc := &runtimeCryptoService{pkiService: nil}
	alg, params := cryptolib.AlgorithmParams{
		Algorithm: cryptolib.AlgorithmECDHES,
		ECDHES:    cryptolib.ECDHESParams{ContentEncryptionAlgorithm: "A256GCM"},
	}.ToParamsMap()

	_, _, err := svc.Encrypt(context.Background(), &providers.KeyRef{KeyID: testKeyID}, alg, params, []byte("data"))
	assert.EqualError(t, err, "PKI service not initialized")
}

func TestDecrypt_RSAOAEP256_NilPKIService(t *testing.T) {
	svc := &runtimeCryptoService{pkiService: nil}
	alg, params := cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmRSAOAEP256}.ToParamsMap()

	_, err := svc.Decrypt(context.Background(), &providers.KeyRef{KeyID: testKeyID}, alg, params, []byte("data"))
	assert.EqualError(t, err, "PKI service not initialized")
}

func TestDecrypt_ECDHES_NilPKIService(t *testing.T) {
	svc := &runtimeCryptoService{pkiService: nil}
	alg, params := cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmECDHES}.ToParamsMap()

	_, err := svc.Decrypt(context.Background(), &providers.KeyRef{KeyID: testKeyID}, alg, params, []byte("data"))
	assert.EqualError(t, err, "PKI service not initialized")
}

// toProviderCryptoDetails

func TestToProviderCryptoDetails_Nil(t *testing.T) {
	assert.Nil(t, toProviderCryptoDetails(nil))
}

// getRSAPublicKey / getECPublicKey – direct PublicKey (empty KeyID)

func TestEncrypt_RSAOAEP256_DirectPublicKeySuccess(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	svc := &runtimeCryptoService{pkiService: pkimock.NewPKIServiceInterfaceMock(t)}
	keyRef := &providers.KeyRef{PublicKey: &rsaKey.PublicKey}
	alg, params := cryptolib.AlgorithmParams{
		Algorithm:  cryptolib.AlgorithmRSAOAEP256,
		RSAOAEP256: cryptolib.RSAOAEP256Params{ContentEncryptionAlgorithm: "A256GCM"},
	}.ToParamsMap()

	wrappedCEK, details, err := svc.Encrypt(context.Background(), keyRef, alg, params, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, wrappedCEK)
	assert.NotNil(t, details)
}

func TestEncrypt_RSAOAEP256_DirectPublicKeyWrongType(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	svc := &runtimeCryptoService{pkiService: pkimock.NewPKIServiceInterfaceMock(t)}
	keyRef := &providers.KeyRef{PublicKey: &ecKey.PublicKey}
	alg, params := cryptolib.AlgorithmParams{
		Algorithm:  cryptolib.AlgorithmRSAOAEP256,
		RSAOAEP256: cryptolib.RSAOAEP256Params{ContentEncryptionAlgorithm: "A256GCM"},
	}.ToParamsMap()

	_, _, err = svc.Encrypt(context.Background(), keyRef, alg, params, []byte("data"))
	assert.EqualError(t, err, "key is not an RSA public key")
}

func TestEncrypt_ECDHES_DirectPublicKeySuccess(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	svc := &runtimeCryptoService{pkiService: pkimock.NewPKIServiceInterfaceMock(t)}
	keyRef := &providers.KeyRef{PublicKey: &ecKey.PublicKey}
	alg, params := cryptolib.AlgorithmParams{
		Algorithm: cryptolib.AlgorithmECDHES,
		ECDHES:    cryptolib.ECDHESParams{ContentEncryptionAlgorithm: "A256GCM"},
	}.ToParamsMap()

	_, details, err := svc.Encrypt(context.Background(), keyRef, alg, params, nil)
	require.NoError(t, err)
	assert.NotNil(t, details)
}

func TestEncrypt_ECDHES_DirectPublicKeyWrongType(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	svc := &runtimeCryptoService{pkiService: pkimock.NewPKIServiceInterfaceMock(t)}
	keyRef := &providers.KeyRef{PublicKey: &rsaKey.PublicKey}
	alg, params := cryptolib.AlgorithmParams{
		Algorithm: cryptolib.AlgorithmECDHES,
		ECDHES:    cryptolib.ECDHESParams{ContentEncryptionAlgorithm: "A256GCM"},
	}.ToParamsMap()

	_, _, err = svc.Encrypt(context.Background(), keyRef, alg, params, []byte("data"))
	assert.EqualError(t, err, "key is not an EC public key")
}

// IsSupportedSigningAlgorithm / IsSupportedEncAlgorithm

func TestIsSupportedSigningAlgorithm(t *testing.T) {
	svc := &runtimeCryptoService{}
	assert.True(t, svc.IsSupportedSigningAlgorithm("RS256"))
	assert.False(t, svc.IsSupportedSigningAlgorithm("not-an-alg"))
}

func TestIsSupportedEncAlgorithm(t *testing.T) {
	svc := &runtimeCryptoService{}
	assert.True(t, svc.IsSupportedEncAlgorithm("RSA-OAEP-256"))
	assert.False(t, svc.IsSupportedEncAlgorithm("not-an-alg"))
}
