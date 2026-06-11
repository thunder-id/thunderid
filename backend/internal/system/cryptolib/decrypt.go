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
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	"errors"
	"fmt"
)

// Decrypt performs key unwrapping or symmetric decryption for the algorithm in params.
// The key type must match the algorithm:
//
//   - AlgorithmAESGCM: key must be []byte (AES key). ciphertext is nonce+ciphertext. Returns plaintext.
//   - AlgorithmRSAOAEP: key must be *rsa.PrivateKey. ciphertext is the wrapped CEK. Returns unwrapped CEK.
//   - AlgorithmRSAOAEP256: key must be *rsa.PrivateKey. ciphertext is the wrapped CEK. Returns unwrapped CEK.
//   - AlgorithmECDHES: key must be *ecdsa.PrivateKey. ciphertext is ignored.
//     params.ECDHES.EPK and params.ECDHES.ContentEncryptionAlgorithm must be set. Returns derived CEK.
//   - AlgorithmECDHESA128KW / AlgorithmECDHESA192KW / AlgorithmECDHESA256KW: key must be *ecdsa.PrivateKey.
//     ciphertext is wrapped CEK.
//     params.ECDHES.EPK must be set. Returns unwrapped CEK.
//   - AlgorithmA128KW / AlgorithmA192KW / AlgorithmA256KW: key must be []byte (symmetric KEK).
//     ciphertext is wrapped CEK. Returns unwrapped CEK.
//   - AlgorithmA128GCMKW / AlgorithmA192GCMKW / AlgorithmA256GCMKW: key must be []byte (symmetric KEK).
//     ciphertext is wrapped CEK.
//     params.AESGCMKW.IV and params.AESGCMKW.Tag must be set. Returns unwrapped CEK.
func Decrypt(key any, params AlgorithmParams, ciphertext []byte) ([]byte, error) {
	switch params.Algorithm {
	case AlgorithmAESGCM:
		aesKey, ok := key.([]byte)
		if !ok {
			return nil, errors.New("AES-GCM requires a []byte key")
		}
		return decryptAESGCM(aesKey, ciphertext)
	case AlgorithmRSAOAEP:
		rsaPriv, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("RSA-OAEP requires a *rsa.PrivateKey")
		}
		return decryptRSAOAEP(rsaPriv, ciphertext)
	case AlgorithmRSAOAEP256:
		rsaPriv, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("RSA-OAEP-256 requires a *rsa.PrivateKey")
		}
		return decryptRSAOAEP256(rsaPriv, ciphertext)
	case AlgorithmECDHES:
		ecPriv, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, errors.New("ECDH-ES requires a *ecdsa.PrivateKey")
		}
		return decryptECDHES(ecPriv, params)
	case AlgorithmECDHESA128KW, AlgorithmECDHESA192KW, AlgorithmECDHESA256KW:
		ecPriv, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("%s requires a *ecdsa.PrivateKey", params.Algorithm)
		}
		return decryptECDHESKW(ecPriv, params, ciphertext)
	case AlgorithmA128KW, AlgorithmA192KW, AlgorithmA256KW:
		kek, ok := key.([]byte)
		if !ok {
			return nil, fmt.Errorf("%s requires a []byte key", params.Algorithm)
		}
		return decryptAESKW(kek, params.Algorithm, ciphertext)
	case AlgorithmA128GCMKW, AlgorithmA192GCMKW, AlgorithmA256GCMKW:
		kek, ok := key.([]byte)
		if !ok {
			return nil, fmt.Errorf("%s requires a []byte key", params.Algorithm)
		}
		return decryptAESGCMKW(kek, params.Algorithm, params, ciphertext)
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", params.Algorithm)
	}
}

func decryptAESGCM(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM mode: %w", err)
	}
	nonceSize := aesgcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, encryptedData := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesgcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

func decryptRSAOAEP256(rsaPriv *rsa.PrivateKey, content []byte) ([]byte, error) {
	return rsa.DecryptOAEP(sha256.New(), rand.Reader, rsaPriv, content, nil)
}

func decryptECDHES(ecdsaPriv *ecdsa.PrivateKey, params AlgorithmParams) ([]byte, error) {
	epk, err := requireECDHEPK(params, "ECDH-ES")
	if err != nil {
		return nil, err
	}
	z, err := ecdhComputeSharedSecretForRecipient(ecdsaPriv, epk)
	if err != nil {
		return nil, fmt.Errorf("ECDH key agreement failed: %w", err)
	}
	if params.ECDHES.ContentEncryptionAlgorithm == "" {
		return nil, errors.New("ContentEncryptionAlgorithm required for ECDH-ES key derivation")
	}
	keyLen, err := ecdhContentEncKeyLen(params.ECDHES.ContentEncryptionAlgorithm)
	if err != nil {
		return nil, err
	}
	return ecdhConcatKDF(
		z, string(params.ECDHES.ContentEncryptionAlgorithm), keyLen, params.ECDHES.APU, params.ECDHES.APV)
}

func decryptECDHESKW(ecdsaPriv *ecdsa.PrivateKey, params AlgorithmParams, content []byte) ([]byte, error) {
	epk, err := requireECDHEPK(params, "ECDH-ES+KW")
	if err != nil {
		return nil, err
	}
	z, err := ecdhComputeSharedSecretForRecipient(ecdsaPriv, epk)
	if err != nil {
		return nil, fmt.Errorf("ECDH key agreement failed: %w", err)
	}
	kekLen := 16
	switch params.Algorithm {
	case AlgorithmECDHESA192KW:
		kekLen = 24
	case AlgorithmECDHESA256KW:
		kekLen = 32
	}
	kek, err := ecdhConcatKDF(z, string(params.Algorithm), kekLen, params.ECDHES.APU, params.ECDHES.APV)
	if err != nil {
		return nil, fmt.Errorf("key derivation failed: %w", err)
	}
	return ecdhAESKeyUnwrap(kek, content)
}

func requireECDHEPK(params AlgorithmParams, algorithm string) (*ecdh.PublicKey, error) {
	if params.ECDHES.EPK == nil {
		return nil, fmt.Errorf("EPK required for %s decryption", algorithm)
	}
	epk, ok := params.ECDHES.EPK.(*ecdh.PublicKey)
	if !ok {
		return nil, errors.New("EPK must be an *ecdh.PublicKey")
	}
	if epk == nil {
		return nil, errors.New("EPK must not be nil")
	}
	return epk, nil
}

func decryptRSAOAEP(rsaPriv *rsa.PrivateKey, content []byte) ([]byte, error) {
	return rsa.DecryptOAEP(sha1.New(), rand.Reader, rsaPriv, content, nil) //nolint:gosec
}

func decryptAESKW(kek []byte, alg Algorithm, content []byte) ([]byte, error) {
	expectedLen := 16
	switch alg {
	case AlgorithmA192KW:
		expectedLen = 24
	case AlgorithmA256KW:
		expectedLen = 32
	}
	if len(kek) != expectedLen {
		return nil, fmt.Errorf("%s requires a %d-byte key, got %d", alg, expectedLen, len(kek))
	}
	return ecdhAESKeyUnwrap(kek, content)
}

func decryptAESGCMKW(kek []byte, alg Algorithm, params AlgorithmParams, content []byte) ([]byte, error) {
	expectedLen := 16
	switch alg {
	case AlgorithmA192GCMKW:
		expectedLen = 24
	case AlgorithmA256GCMKW:
		expectedLen = 32
	}
	if len(kek) != expectedLen {
		return nil, fmt.Errorf("%s requires a %d-byte key, got %d", alg, expectedLen, len(kek))
	}
	if len(params.AESGCMKW.IV) == 0 {
		return nil, fmt.Errorf("IV required for %s decryption", alg)
	}
	if len(params.AESGCMKW.Tag) == 0 {
		return nil, fmt.Errorf("authentication tag required for %s decryption", alg)
	}
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	sealed := append(content, params.AESGCMKW.Tag...) //nolint:gocritic
	return gcm.Open(nil, params.AESGCMKW.IV, sealed, nil)
}
