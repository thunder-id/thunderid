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

package cryptolib

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ----------------------------------------------------------------------------
// AES-GCM tests
// ----------------------------------------------------------------------------

func TestAESGCMEncryptDecryptRoundTrip(t *testing.T) {
	testCases := []struct {
		name      string
		plaintext []byte
	}{
		{"ASCII", []byte("Hello, World!")},
		{"Empty", []byte{}},
		{"Unicode", []byte("こんにちは世界 🌍")},
		{"JSON", []byte(`{"key":"value","num":42}`)},
		{"Binary", func() []byte { b := make([]byte, 64); _, _ = rand.Read(b); return b }()},
	}

	key := make([]byte, 32) // AES-256
	_, err := rand.Read(key)
	require.NoError(t, err)

	params := AlgorithmParams{Algorithm: AlgorithmAESGCM}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ciphertext, details, err := Encrypt(key, &params, tc.plaintext)
			require.NoError(t, err)
			assert.Nil(t, details)
			assert.NotEmpty(t, ciphertext)

			decrypted, err := Decrypt(key, params, ciphertext)
			require.NoError(t, err)
			if len(tc.plaintext) == 0 {
				assert.Empty(t, decrypted)
			} else {
				assert.Equal(t, tc.plaintext, decrypted)
			}
		})
	}
}

func TestAESGCMProducesUniqueCiphertexts(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)

	plaintext := []byte("same plaintext")
	params := AlgorithmParams{Algorithm: AlgorithmAESGCM}

	ct1, _, err := Encrypt(key, &params, plaintext)
	require.NoError(t, err)

	ct2, _, err := Encrypt(key, &params, plaintext)
	require.NoError(t, err)

	assert.NotEqual(t, ct1, ct2, "Each encryption should produce a unique ciphertext due to random nonce")
}

func TestAESGCMTamperDetection(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)

	plaintext := []byte("tamper test")
	params := AlgorithmParams{Algorithm: AlgorithmAESGCM}

	ciphertext, _, err := Encrypt(key, &params, plaintext)
	require.NoError(t, err)

	// Flip a byte in the ciphertext portion (after nonce)
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	tampered[len(tampered)-1] ^= 0xFF

	_, err = Decrypt(key, params, tampered)
	assert.Error(t, err, "Decryption of tampered ciphertext should fail")
}

func TestAESGCMCiphertextTooShort(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)

	params := AlgorithmParams{Algorithm: AlgorithmAESGCM}

	// Pass a ciphertext shorter than the nonce size
	_, err = Decrypt(key, params, []byte("short"))
	assert.Error(t, err, "Decryption of too-short ciphertext should fail")
}

func TestAESGCMWrongKeyFails(t *testing.T) {
	key1 := make([]byte, 32)
	_, err := rand.Read(key1)
	require.NoError(t, err)

	key2 := make([]byte, 32)
	_, err = rand.Read(key2)
	require.NoError(t, err)

	params := AlgorithmParams{Algorithm: AlgorithmAESGCM}

	ciphertext, _, err := Encrypt(key1, &params, []byte("secret"))
	require.NoError(t, err)

	_, err = Decrypt(key2, params, ciphertext)
	assert.Error(t, err, "Decryption with wrong key should fail")
}

func TestAESGCMWrongKeyTypeFails(t *testing.T) {
	params := AlgorithmParams{Algorithm: AlgorithmAESGCM}

	_, _, err := Encrypt("not-a-byte-slice", &params, []byte("secret"))
	assert.Error(t, err, "Encrypt with wrong key type should fail")

	_, err = Decrypt("not-a-byte-slice", params, []byte("some ciphertext"))
	assert.Error(t, err, "Decrypt with wrong key type should fail")
}

// ----------------------------------------------------------------------------
// RSA-OAEP-256 tests
// ----------------------------------------------------------------------------

func TestRSAOAEP256EncryptDecryptRoundTrip(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	params := AlgorithmParams{
		Algorithm: AlgorithmRSAOAEP256,
		RSAOAEP256: RSAOAEP256Params{
			ContentEncryptionAlgorithm: "A256GCM",
		},
	}

	wrappedCEK, details, err := Encrypt(&privKey.PublicKey, &params, nil)
	require.NoError(t, err)
	require.NotNil(t, details)
	assert.NotEmpty(t, wrappedCEK, "Wrapped CEK should not be empty")
	assert.NotEmpty(t, details.CEK, "CEK in details should not be empty")
	assert.Nil(t, details.EPK, "EPK should be nil for RSA-OAEP-256")

	decryptParams := AlgorithmParams{Algorithm: AlgorithmRSAOAEP256}
	unwrappedCEK, err := Decrypt(privKey, decryptParams, wrappedCEK)
	require.NoError(t, err)

	assert.Equal(t, details.CEK, unwrappedCEK, "Unwrapped CEK should match the original CEK")
}

func TestRSAOAEP256MissingContentEncryptionAlgorithmFails(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	params := AlgorithmParams{
		Algorithm:  AlgorithmRSAOAEP256,
		RSAOAEP256: RSAOAEP256Params{},
	}

	_, _, err = Encrypt(&privKey.PublicKey, &params, nil)
	assert.Error(t, err, "Encrypt without ContentEncryptionAlgorithm should fail")
}

func TestRSAOAEP256WrongKeyTypeFails(t *testing.T) {
	params := AlgorithmParams{Algorithm: AlgorithmRSAOAEP256}

	_, _, err := Encrypt("not-an-rsa-key", &params, nil)
	assert.Error(t, err, "Encrypt with wrong key type should fail")

	_, err = Decrypt("not-an-rsa-key", params, []byte("wrapped"))
	assert.Error(t, err, "Decrypt with wrong key type should fail")
}

// ----------------------------------------------------------------------------
// ECDH-ES tests
// ----------------------------------------------------------------------------

func TestECDHESEncryptDecryptRoundTrip(t *testing.T) {
	receiverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	encParams := AlgorithmParams{
		Algorithm: AlgorithmECDHES,
		ECDHES: ECDHESParams{
			ContentEncryptionAlgorithm: "A128GCM",
		},
	}

	ciphertext, details, err := Encrypt(&receiverKey.PublicKey, &encParams, nil)
	require.NoError(t, err)
	require.NotNil(t, details)
	assert.Nil(t, ciphertext, "ECDH-ES direct agreement returns nil ciphertext")
	assert.NotNil(t, details.EPK, "EPK should be set for ECDH-ES")
	assert.NotEmpty(t, details.CEK, "CEK should be set for ECDH-ES")

	decParams := AlgorithmParams{
		Algorithm: AlgorithmECDHES,
		ECDHES: ECDHESParams{
			EPK:                        details.EPK,
			ContentEncryptionAlgorithm: "A128GCM",
		},
	}

	derivedCEK, err := Decrypt(receiverKey, decParams, nil)
	require.NoError(t, err)

	assert.Equal(t, details.CEK, derivedCEK, "Derived CEK should match the original CEK")
}

func TestECDHESMissingEPKOnDecryptFails(t *testing.T) {
	receiverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	params := AlgorithmParams{
		Algorithm: AlgorithmECDHES,
		ECDHES: ECDHESParams{
			ContentEncryptionAlgorithm: "A128GCM",
		},
	}

	_, err = Decrypt(receiverKey, params, nil)
	assert.Error(t, err, "Decrypt without EPK should fail")
}

func TestECDHESWrongKeyTypeFails(t *testing.T) {
	params := AlgorithmParams{
		Algorithm: AlgorithmECDHES,
		ECDHES:    ECDHESParams{ContentEncryptionAlgorithm: "A128GCM"},
	}

	_, _, err := Encrypt("not-an-ec-key", &params, nil)
	assert.Error(t, err, "Encrypt with wrong key type should fail")

	_, err = Decrypt("not-an-ec-key", params, nil)
	assert.Error(t, err, "Decrypt with wrong key type should fail")
}

// ----------------------------------------------------------------------------
// ECDH-ES+A128KW tests
// ----------------------------------------------------------------------------

func TestECDHESA128KWEncryptDecryptRoundTrip(t *testing.T) {
	receiverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	encParams := AlgorithmParams{
		Algorithm: AlgorithmECDHESA128KW,
		ECDHES: ECDHESParams{
			ContentEncryptionAlgorithm: "A128GCM",
		},
	}

	wrappedCEK, details, err := Encrypt(&receiverKey.PublicKey, &encParams, nil)
	require.NoError(t, err)
	require.NotNil(t, details)
	assert.NotEmpty(t, wrappedCEK, "ECDH-ES+A128KW should return a wrapped CEK")
	assert.NotNil(t, details.EPK, "EPK should be set for ECDH-ES+KW")
	assert.NotEmpty(t, details.CEK, "CEK should be set for ECDH-ES+KW")

	decParams := AlgorithmParams{
		Algorithm: AlgorithmECDHESA128KW,
		ECDHES: ECDHESParams{
			EPK: details.EPK,
		},
	}

	unwrappedCEK, err := Decrypt(receiverKey, decParams, wrappedCEK)
	require.NoError(t, err)

	assert.Equal(t, details.CEK, unwrappedCEK, "Unwrapped CEK should match the original CEK")
}

func TestECDHESA256KWEncryptDecryptRoundTrip(t *testing.T) {
	receiverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	encParams := AlgorithmParams{
		Algorithm: AlgorithmECDHESA256KW,
		ECDHES: ECDHESParams{
			ContentEncryptionAlgorithm: "A256GCM",
		},
	}

	wrappedCEK, details, err := Encrypt(&receiverKey.PublicKey, &encParams, nil)
	require.NoError(t, err)
	require.NotNil(t, details)
	assert.NotEmpty(t, wrappedCEK, "ECDH-ES+A256KW should return a wrapped CEK")
	assert.NotNil(t, details.EPK, "EPK should be set for ECDH-ES+A256KW")
	assert.NotEmpty(t, details.CEK, "CEK should be set for ECDH-ES+A256KW")

	decParams := AlgorithmParams{
		Algorithm: AlgorithmECDHESA256KW,
		ECDHES: ECDHESParams{
			EPK: details.EPK,
		},
	}

	unwrappedCEK, err := Decrypt(receiverKey, decParams, wrappedCEK)
	require.NoError(t, err)

	assert.Equal(t, details.CEK, unwrappedCEK, "Unwrapped CEK should match the original CEK")
}

// ----------------------------------------------------------------------------
// Nil params tests
// ----------------------------------------------------------------------------

func TestEncryptNilParamsFails(t *testing.T) {
	_, _, err := Encrypt([]byte("key"), nil, []byte("content"))
	assert.Error(t, err, "Encrypt with nil params should fail")
	assert.Contains(t, err.Error(), "algorithm params required")
}

// ----------------------------------------------------------------------------
// ECDH-ES+KW wrong key type tests
// ----------------------------------------------------------------------------

func TestECDHESKWWrongKeyTypeFails(t *testing.T) {
	for _, alg := range []Algorithm{AlgorithmECDHESA128KW, AlgorithmECDHESA256KW} {
		t.Run(string(alg), func(t *testing.T) {
			params := AlgorithmParams{
				Algorithm: alg,
				ECDHES:    ECDHESParams{ContentEncryptionAlgorithm: "A128GCM"},
			}
			_, _, err := Encrypt("not-an-ec-key", &params, nil)
			assert.Error(t, err, "Encrypt with wrong key type should fail")
			assert.Contains(t, err.Error(), "*ecdsa.PublicKey")
		})
	}
}

// ----------------------------------------------------------------------------
// Unsupported algorithm tests
// ----------------------------------------------------------------------------

func TestEncryptUnsupportedAlgorithmFails(t *testing.T) {
	params := AlgorithmParams{Algorithm: "UNSUPPORTED"}

	_, _, err := Encrypt([]byte("key"), &params, []byte("content"))
	assert.Error(t, err)
}

func TestDecryptUnsupportedAlgorithmFails(t *testing.T) {
	params := AlgorithmParams{Algorithm: "UNSUPPORTED"}

	_, err := Decrypt([]byte("key"), params, []byte("ciphertext"))
	assert.Error(t, err)
}

// ----------------------------------------------------------------------------
// RSA-OAEP tests
// ----------------------------------------------------------------------------

func TestRSAOAEPEncryptDecryptRoundTrip(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	params := AlgorithmParams{
		Algorithm: AlgorithmRSAOAEP,
		RSAOAEP:   RSAOAEPParams{ContentEncryptionAlgorithm: "A256GCM"},
	}

	wrappedCEK, details, err := Encrypt(&privKey.PublicKey, &params, nil)
	require.NoError(t, err)
	require.NotNil(t, details)
	assert.NotEmpty(t, wrappedCEK)
	assert.NotEmpty(t, details.CEK)
	assert.Nil(t, details.EPK)

	decryptParams := AlgorithmParams{Algorithm: AlgorithmRSAOAEP}
	unwrappedCEK, err := Decrypt(privKey, decryptParams, wrappedCEK)
	require.NoError(t, err)
	assert.Equal(t, details.CEK, unwrappedCEK)
}

func TestRSAOAEPMissingContentEncryptionAlgorithmFails(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	params := AlgorithmParams{Algorithm: AlgorithmRSAOAEP, RSAOAEP: RSAOAEPParams{}}
	_, _, err = Encrypt(&privKey.PublicKey, &params, nil)
	assert.Error(t, err)
}

func TestRSAOAEPWrongKeyTypeFails(t *testing.T) {
	params := AlgorithmParams{Algorithm: AlgorithmRSAOAEP}

	_, _, err := Encrypt("not-an-rsa-key", &params, nil)
	assert.Error(t, err)

	_, err = Decrypt("not-an-rsa-key", params, []byte("wrapped"))
	assert.Error(t, err)
}

// ----------------------------------------------------------------------------
// ECDH-ES+A192KW tests
// ----------------------------------------------------------------------------

func TestECDHESA192KWEncryptDecryptRoundTrip(t *testing.T) {
	receiverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	encParams := AlgorithmParams{
		Algorithm: AlgorithmECDHESA192KW,
		ECDHES:    ECDHESParams{ContentEncryptionAlgorithm: "A192GCM"},
	}

	wrappedCEK, details, err := Encrypt(&receiverKey.PublicKey, &encParams, nil)
	require.NoError(t, err)
	require.NotNil(t, details)
	assert.NotEmpty(t, wrappedCEK)
	assert.NotNil(t, details.EPK)
	assert.NotEmpty(t, details.CEK)

	decParams := AlgorithmParams{
		Algorithm: AlgorithmECDHESA192KW,
		ECDHES:    ECDHESParams{EPK: details.EPK},
	}
	unwrappedCEK, err := Decrypt(receiverKey, decParams, wrappedCEK)
	require.NoError(t, err)
	assert.Equal(t, details.CEK, unwrappedCEK)
}

// ----------------------------------------------------------------------------
// AES Key Wrap tests
// ----------------------------------------------------------------------------

func TestAESKWEncryptDecryptRoundTrip(t *testing.T) {
	testCases := []struct {
		alg     Algorithm
		keySize int
	}{
		{AlgorithmA128KW, 16},
		{AlgorithmA192KW, 24},
		{AlgorithmA256KW, 32},
	}

	for _, tc := range testCases {
		t.Run(string(tc.alg), func(t *testing.T) {
			kek := make([]byte, tc.keySize)
			_, err := rand.Read(kek)
			require.NoError(t, err)

			encParams := AlgorithmParams{
				Algorithm: tc.alg,
				AESKW:     AESKWParams{ContentEncryptionAlgorithm: "A256GCM"},
			}

			wrappedCEK, details, err := Encrypt(kek, &encParams, nil)
			require.NoError(t, err)
			require.NotNil(t, details)
			assert.NotEmpty(t, wrappedCEK)
			assert.NotEmpty(t, details.CEK)
			assert.Nil(t, details.EPK)

			decParams := AlgorithmParams{Algorithm: tc.alg}
			unwrappedCEK, err := Decrypt(kek, decParams, wrappedCEK)
			require.NoError(t, err)
			assert.Equal(t, details.CEK, unwrappedCEK)
		})
	}
}

func TestAESKWWrongKeyTypeFails(t *testing.T) {
	for _, alg := range []Algorithm{AlgorithmA128KW, AlgorithmA192KW, AlgorithmA256KW} {
		t.Run(string(alg), func(t *testing.T) {
			params := AlgorithmParams{
				Algorithm: alg,
				AESKW:     AESKWParams{ContentEncryptionAlgorithm: "A256GCM"},
			}
			_, _, err := Encrypt("not-a-byte-slice", &params, nil)
			assert.Error(t, err)
		})
	}
}

func TestAESKWWrongKeySizeFails(t *testing.T) {
	wrongKey := make([]byte, 32) // A128KW expects 16 bytes
	params := AlgorithmParams{
		Algorithm: AlgorithmA128KW,
		AESKW:     AESKWParams{ContentEncryptionAlgorithm: "A256GCM"},
	}
	_, _, err := Encrypt(wrongKey, &params, nil)
	assert.Error(t, err)
}

// ----------------------------------------------------------------------------
// AES-GCM Key Wrap tests
// ----------------------------------------------------------------------------

func TestAESGCMKWEncryptDecryptRoundTrip(t *testing.T) {
	testCases := []struct {
		alg     Algorithm
		keySize int
	}{
		{AlgorithmA128GCMKW, 16},
		{AlgorithmA192GCMKW, 24},
		{AlgorithmA256GCMKW, 32},
	}

	for _, tc := range testCases {
		t.Run(string(tc.alg), func(t *testing.T) {
			kek := make([]byte, tc.keySize)
			_, err := rand.Read(kek)
			require.NoError(t, err)

			encParams := AlgorithmParams{
				Algorithm: tc.alg,
				AESGCMKW:  AESGCMKWParams{ContentEncryptionAlgorithm: "A256GCM"},
			}

			wrappedCEK, details, err := Encrypt(kek, &encParams, nil)
			require.NoError(t, err)
			require.NotNil(t, details)
			assert.NotEmpty(t, wrappedCEK)
			assert.NotEmpty(t, details.CEK)
			assert.NotEmpty(t, details.IV)
			assert.NotEmpty(t, details.Tag)
			assert.Nil(t, details.EPK)

			decParams := AlgorithmParams{
				Algorithm: tc.alg,
				AESGCMKW:  AESGCMKWParams{IV: details.IV, Tag: details.Tag},
			}
			unwrappedCEK, err := Decrypt(kek, decParams, wrappedCEK)
			require.NoError(t, err)
			assert.Equal(t, details.CEK, unwrappedCEK)
		})
	}
}

func TestAESGCMKWMissingIVFails(t *testing.T) {
	kek := make([]byte, 16)
	_, _ = rand.Read(kek)

	params := AlgorithmParams{
		Algorithm: AlgorithmA128GCMKW,
		AESGCMKW:  AESGCMKWParams{Tag: []byte("tag-tag-tag-tag-")},
	}
	_, err := Decrypt(kek, params, []byte("wrapped"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "IV required")
}

func TestAESGCMKWMissingTagFails(t *testing.T) {
	kek := make([]byte, 16)
	_, _ = rand.Read(kek)

	params := AlgorithmParams{
		Algorithm: AlgorithmA128GCMKW,
		AESGCMKW:  AESGCMKWParams{IV: []byte("iv-iv-iv-iv-iv--")},
	}
	_, err := Decrypt(kek, params, []byte("wrapped"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authentication tag required")
}

func TestEncryptionAlgorithmForSupportedAlgorithms(t *testing.T) {
	for _, alg := range []Algorithm{
		AlgorithmRSAOAEP, AlgorithmRSAOAEP256, AlgorithmECDHES, AlgorithmECDHESA128KW,
		AlgorithmECDHESA192KW, AlgorithmECDHESA256KW, AlgorithmA128KW, AlgorithmA192KW,
		AlgorithmA256KW, AlgorithmA128GCMKW, AlgorithmA192GCMKW, AlgorithmA256GCMKW,
	} {
		got, err := EncryptionAlgorithmFor(alg)
		assert.NoError(t, err)
		assert.Equal(t, alg, got)
	}
}

func TestEncryptionAlgorithmForUnsupportedAlgorithmFails(t *testing.T) {
	_, err := EncryptionAlgorithmFor(AlgorithmAESGCM)
	assert.ErrorIs(t, err, ErrUnsupportedAlgorithm)
}
