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

package defaultkm

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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/pki/pkimock"
)

const testKeyID = "test-key-id"

func newTestSvcErr() *serviceerror.ServiceError {
	return &serviceerror.ServiceError{
		Code:  "TEST-001",
		Error: core.I18nMessage{DefaultValue: "key not found"},
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

	params := cryptolib.AlgorithmParams{
		Algorithm: cryptolib.AlgorithmRSAOAEP256,
		RSAOAEP256: cryptolib.RSAOAEP256Params{
			ContentEncryptionAlgorithm: "A256GCM",
		},
	}
	wrappedCEK, details, err := svc.Encrypt(
		context.Background(), &kmprovider.KeyRef{KeyID: testKeyID}, params, nil,
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

	params := cryptolib.AlgorithmParams{
		Algorithm: cryptolib.AlgorithmRSAOAEP256,
		RSAOAEP256: cryptolib.RSAOAEP256Params{
			ContentEncryptionAlgorithm: "A256GCM",
		},
	}
	_, _, err := svc.Encrypt(context.Background(), &kmprovider.KeyRef{KeyID: testKeyID}, params, []byte("data"))
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

	params := cryptolib.AlgorithmParams{
		Algorithm: cryptolib.AlgorithmECDHES,
		ECDHES: cryptolib.ECDHESParams{
			ContentEncryptionAlgorithm: "A128GCM",
		},
	}
	_, details, err := svc.Encrypt(context.Background(), &kmprovider.KeyRef{KeyID: testKeyID}, params, nil)
	require.NoError(t, err)
	assert.NotNil(t, details)
}

func TestEncrypt_ECDHES_GetPublicKeyError(t *testing.T) {
	pkiMock := pkimock.NewPKIServiceInterfaceMock(t)
	pkiMock.EXPECT().
		GetX509Certificate(mock.Anything, testKeyID).
		Return(nil, newTestSvcErr())

	svc := NewRuntimeCryptoService(pkiMock, nil)

	params := cryptolib.AlgorithmParams{
		Algorithm: cryptolib.AlgorithmECDHES,
		ECDHES: cryptolib.ECDHESParams{
			ContentEncryptionAlgorithm: "A128GCM",
		},
	}
	_, _, err := svc.Encrypt(context.Background(), &kmprovider.KeyRef{KeyID: testKeyID}, params, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), testKeyID)
}

func TestEncrypt_UnsupportedAlgorithm(t *testing.T) {
	svc := NewRuntimeCryptoService(nil, nil)

	params := cryptolib.AlgorithmParams{Algorithm: "UNSUPPORTED"}
	_, _, err := svc.Encrypt(context.Background(), &kmprovider.KeyRef{KeyID: testKeyID}, params, []byte("data"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported algorithm")
}

// Encrypt – RSA-OAEP-256
func TestEncrypt_RSAOAEP256_NilKeyRef(t *testing.T) {
	svc := &runtimeCryptoService{pkiService: pkimock.NewPKIServiceInterfaceMock(t)}
	params := cryptolib.AlgorithmParams{
		Algorithm:  cryptolib.AlgorithmRSAOAEP256,
		RSAOAEP256: cryptolib.RSAOAEP256Params{ContentEncryptionAlgorithm: "A256GCM"},
	}

	_, _, err := svc.Encrypt(context.Background(), nil, params, []byte("data"))
	assert.EqualError(t, err, "keyRef required for RSA-OAEP-256")
}

func TestEncrypt_RSAOAEP256_PKIError(t *testing.T) {
	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetX509Certificate(mock.Anything, "key1").Return(nil, &serviceerror.InternalServerError)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &kmprovider.KeyRef{KeyID: "key1"}
	params := cryptolib.AlgorithmParams{
		Algorithm:  cryptolib.AlgorithmRSAOAEP256,
		RSAOAEP256: cryptolib.RSAOAEP256Params{ContentEncryptionAlgorithm: "A256GCM"},
	}

	_, _, err := svc.Encrypt(context.Background(), keyRef, params, []byte("data"))
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
	keyRef := &kmprovider.KeyRef{KeyID: "key1"}
	params := cryptolib.AlgorithmParams{
		Algorithm:  cryptolib.AlgorithmRSAOAEP256,
		RSAOAEP256: cryptolib.RSAOAEP256Params{ContentEncryptionAlgorithm: "A256GCM"},
	}

	_, _, err = svc.Encrypt(context.Background(), keyRef, params, []byte("data"))
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
	keyRef := &kmprovider.KeyRef{KeyID: "key1"}
	params := cryptolib.AlgorithmParams{
		Algorithm:  cryptolib.AlgorithmRSAOAEP256,
		RSAOAEP256: cryptolib.RSAOAEP256Params{ContentEncryptionAlgorithm: "A256GCM"},
	}

	ciphertext, details, err := svc.Encrypt(context.Background(), keyRef, params, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)
	require.NotNil(t, details)
	assert.NotEmpty(t, details.CEK)
}

// Encrypt – ECDH-ES variants
func TestEncrypt_ECDHES_NilKeyRef(t *testing.T) {
	svc := &runtimeCryptoService{pkiService: pkimock.NewPKIServiceInterfaceMock(t)}
	params := cryptolib.AlgorithmParams{
		Algorithm: cryptolib.AlgorithmECDHES,
		ECDHES:    cryptolib.ECDHESParams{ContentEncryptionAlgorithm: "A256GCM"},
	}

	_, _, err := svc.Encrypt(context.Background(), nil, params, []byte("data"))
	assert.EqualError(t, err, "keyRef required for ECDH-ES")
}

func TestEncrypt_ECDHES_PKIError(t *testing.T) {
	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetX509Certificate(mock.Anything, "key1").Return(nil, &serviceerror.InternalServerError)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &kmprovider.KeyRef{KeyID: "key1"}
	params := cryptolib.AlgorithmParams{
		Algorithm: cryptolib.AlgorithmECDHES,
		ECDHES:    cryptolib.ECDHESParams{ContentEncryptionAlgorithm: "A256GCM"},
	}

	_, _, err := svc.Encrypt(context.Background(), keyRef, params, []byte("data"))
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
	keyRef := &kmprovider.KeyRef{KeyID: "key1"}
	params := cryptolib.AlgorithmParams{
		Algorithm: cryptolib.AlgorithmECDHES,
		ECDHES:    cryptolib.ECDHESParams{ContentEncryptionAlgorithm: "A256GCM"},
	}

	_, _, err = svc.Encrypt(context.Background(), keyRef, params, nil)
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
			keyRef := &kmprovider.KeyRef{KeyID: "key1"}
			params := cryptolib.AlgorithmParams{
				Algorithm: alg,
				ECDHES:    cryptolib.ECDHESParams{ContentEncryptionAlgorithm: "A256GCM"},
			}

			_, details, err := svc.Encrypt(context.Background(), keyRef, params, nil)
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
	params := cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmRSAOAEP256}

	_, err := svc.Decrypt(context.Background(), nil, params, []byte("ciphertext"))
	assert.EqualError(t, err, "keyRef required for RSA-OAEP-256")
}

func TestDecrypt_RSAOAEP256_PKIError(t *testing.T) {
	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetPrivateKey(mock.Anything, "key1").Return(nil, &serviceerror.InternalServerError)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &kmprovider.KeyRef{KeyID: "key1"}
	params := cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmRSAOAEP256}

	_, err := svc.Decrypt(context.Background(), keyRef, params, []byte("ciphertext"))
	assert.Error(t, err)
}

func TestDecrypt_RSAOAEP256_NonRSAPrivateKey(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetPrivateKey(mock.Anything, "key1").Return(ecKey, nil)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &kmprovider.KeyRef{KeyID: "key1"}
	params := cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmRSAOAEP256}

	_, err = svc.Decrypt(context.Background(), keyRef, params, []byte("ciphertext"))
	assert.EqualError(t, err, "key is not an RSA private key")
}

// Decrypt – ECDH-ES variants
func TestDecrypt_ECDHES_NilKeyRef(t *testing.T) {
	svc := &runtimeCryptoService{pkiService: pkimock.NewPKIServiceInterfaceMock(t)}
	params := cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmECDHES}

	_, err := svc.Decrypt(context.Background(), nil, params, nil)
	assert.EqualError(t, err, "keyRef required for ECDH-ES")
}

func TestDecrypt_ECDHES_PKIError(t *testing.T) {
	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetPrivateKey(mock.Anything, "key1").Return(nil, &serviceerror.InternalServerError)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &kmprovider.KeyRef{KeyID: "key1"}
	params := cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmECDHES}

	_, err := svc.Decrypt(context.Background(), keyRef, params, nil)
	assert.Error(t, err)
}

func TestDecrypt_ECDHES_NonECPrivateKey(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetPrivateKey(mock.Anything, "key1").Return(rsaKey, nil)

	svc := &runtimeCryptoService{pkiService: pki}
	keyRef := &kmprovider.KeyRef{KeyID: "key1"}
	params := cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmECDHES}

	_, err = svc.Decrypt(context.Background(), keyRef, params, nil)
	assert.EqualError(t, err, "key is not an EC private key")
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
			keyRef := &kmprovider.KeyRef{KeyID: "key1"}

			encParams := cryptolib.AlgorithmParams{
				Algorithm: alg,
				ECDHES:    cryptolib.ECDHESParams{ContentEncryptionAlgorithm: "A256GCM"},
			}
			ciphertext, encDetails, err := svc.Encrypt(context.Background(), keyRef, encParams, nil)
			require.NoError(t, err)
			require.NotNil(t, encDetails)

			decParams := cryptolib.AlgorithmParams{
				Algorithm: alg,
				ECDHES:    cryptolib.ECDHESParams{EPK: encDetails.EPK, ContentEncryptionAlgorithm: "A256GCM"},
			}
			derivedCEK, err := svc.Decrypt(context.Background(), keyRef, decParams, ciphertext)
			require.NoError(t, err)
			assert.Equal(t, encDetails.CEK, derivedCEK)
		})
	}
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
	keyRef := &kmprovider.KeyRef{KeyID: "key1"}
	params := cryptolib.AlgorithmParams{
		Algorithm: cryptolib.AlgorithmRSAOAEP,
		RSAOAEP:   cryptolib.RSAOAEPParams{ContentEncryptionAlgorithm: "A256GCM"},
	}

	ciphertext, details, err := svc.Encrypt(context.Background(), keyRef, params, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)
	require.NotNil(t, details)
	assert.NotEmpty(t, details.CEK)
}

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
			keyRef := &kmprovider.KeyRef{KeyID: "key1"}

			wrappedCEK, details, err := svc.Encrypt(context.Background(), keyRef, tc.encParams, nil)
			require.NoError(t, err)
			require.NotNil(t, details)

			unwrappedCEK, err := svc.Decrypt(context.Background(), keyRef, tc.decParams, wrappedCEK)
			require.NoError(t, err)
			assert.Equal(t, details.CEK, unwrappedCEK)
		})
	}
}

// GetPublicKeys

func TestGetPublicKeys_NilPKIService(t *testing.T) {
	svc := &runtimeCryptoService{pkiService: nil}
	_, err := svc.GetPublicKeys(context.Background(), kmprovider.PublicKeyFilter{})
	assert.EqualError(t, err, "PKI service not initialized")
}

func TestGetPublicKeys_GetAllX509CertificatesError(t *testing.T) {
	pki := pkimock.NewPKIServiceInterfaceMock(t)
	pki.EXPECT().GetAllX509Certificates(mock.Anything).Return(nil, &serviceerror.InternalServerError)

	svc := &runtimeCryptoService{pkiService: pki, logger: newTestLogger()}
	_, err := svc.GetPublicKeys(context.Background(), kmprovider.PublicKeyFilter{})
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
	keys, err := svc.GetPublicKeys(context.Background(), kmprovider.PublicKeyFilter{})
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, "key1", keys[0].KeyID)
	assert.Equal(t, cryptolib.AlgorithmRS256, keys[0].Algorithm)
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
			keys, err := svc.GetPublicKeys(context.Background(), kmprovider.PublicKeyFilter{})
			require.NoError(t, err)
			require.Len(t, keys, 1)
			assert.Equal(t, tt.alg, keys[0].Algorithm)
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
	keys, err := svc.GetPublicKeys(context.Background(), kmprovider.PublicKeyFilter{})
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, cryptolib.AlgorithmEdDSA, keys[0].Algorithm)
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
	keys, err := svc.GetPublicKeys(context.Background(), kmprovider.PublicKeyFilter{})
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
	keys, err := svc.GetPublicKeys(context.Background(), kmprovider.PublicKeyFilter{KeyID: "rsa-key"})
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
		kmprovider.PublicKeyFilter{Algorithm: cryptolib.AlgorithmES256})
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, "ec-key", keys[0].KeyID)
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
