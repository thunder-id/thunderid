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
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // SHA-1 is required by RSA-OAEP (RFC 7518)
	"crypto/sha256"
	"errors"
	"fmt"
)

// Encrypt performs cryptographic key establishment or symmetric encryption for the algorithm
// specified in params. The key type must match the algorithm:
//
//   - AlgorithmAESGCM: key must be []byte (AES key). Returns nonce+ciphertext, nil details, nil error.
//   - AlgorithmRSAOAEP: key must be *rsa.PublicKey. content is ignored (key establishment only).
//     Returns wrappedCEK, CryptoDetails{CEK}, nil. params.RSAOAEP.ContentEncryptionAlgorithm must be set.
//   - AlgorithmRSAOAEP256: key must be *rsa.PublicKey. content is ignored (key establishment only).
//     Returns wrappedCEK, CryptoDetails{CEK}, nil. params.RSAOAEP256.ContentEncryptionAlgorithm must be set.
//   - AlgorithmECDHES: key must be *ecdsa.PublicKey. content is ignored.
//     Returns nil, CryptoDetails{EPK, CEK}, nil. params.ECDHES.ContentEncryptionAlgorithm must be set.
//   - AlgorithmECDHESA128KW / AlgorithmECDHESA192KW / AlgorithmECDHESA256KW: key must be *ecdsa.PublicKey.
//     content is ignored.
//     Returns wrappedCEK, CryptoDetails{EPK, CEK}, nil. params.ECDHES.ContentEncryptionAlgorithm must be set.
//   - AlgorithmA128KW / AlgorithmA192KW / AlgorithmA256KW: key must be []byte (symmetric KEK). content is ignored.
//     Returns wrappedCEK, CryptoDetails{CEK}, nil. params.AESKW.ContentEncryptionAlgorithm must be set.
//   - AlgorithmA128GCMKW / AlgorithmA192GCMKW / AlgorithmA256GCMKW: key must be []byte (symmetric KEK).
//     content is ignored.
//     Returns wrappedCEK, CryptoDetails{CEK, IV, Tag}, nil. params.AESGCMKW.ContentEncryptionAlgorithm must be set.
func Encrypt(key any, params *AlgorithmParams, content []byte) ([]byte, *CryptoDetails, error) {
	if params == nil {
		return nil, nil, errors.New("algorithm params required")
	}
	switch params.Algorithm {
	case AlgorithmAESGCM:
		aesKey, ok := key.([]byte)
		if !ok {
			return nil, nil, errors.New("AES-GCM requires a []byte key")
		}
		ciphertext, err := encryptAESGCM(aesKey, content)
		return ciphertext, nil, err
	case AlgorithmRSAOAEP:
		rsaPub, ok := key.(*rsa.PublicKey)
		if !ok {
			return nil, nil, errors.New("RSA-OAEP requires a *rsa.PublicKey")
		}
		return encryptRSAOAEP(rsaPub, *params)
	case AlgorithmRSAOAEP256:
		rsaPub, ok := key.(*rsa.PublicKey)
		if !ok {
			return nil, nil, errors.New("RSA-OAEP-256 requires a *rsa.PublicKey")
		}
		return encryptRSAOAEP256(rsaPub, *params)
	case AlgorithmECDHES:
		ecPub, ok := key.(*ecdsa.PublicKey)
		if !ok {
			return nil, nil, errors.New("ECDH-ES requires a *ecdsa.PublicKey")
		}
		return encryptECDHES(ecPub, *params)
	case AlgorithmECDHESA128KW, AlgorithmECDHESA192KW, AlgorithmECDHESA256KW:
		ecPub, ok := key.(*ecdsa.PublicKey)
		if !ok {
			return nil, nil, fmt.Errorf("%s requires a *ecdsa.PublicKey", params.Algorithm)
		}
		return encryptECDHESKW(ecPub, *params)
	case AlgorithmA128KW, AlgorithmA192KW, AlgorithmA256KW:
		kek, ok := key.([]byte)
		if !ok {
			return nil, nil, fmt.Errorf("%s requires a []byte key", params.Algorithm)
		}
		return encryptAESKW(kek, params.Algorithm, *params)
	case AlgorithmA128GCMKW, AlgorithmA192GCMKW, AlgorithmA256GCMKW:
		kek, ok := key.([]byte)
		if !ok {
			return nil, nil, fmt.Errorf("%s requires a []byte key", params.Algorithm)
		}
		return encryptAESGCMKW(kek, params.Algorithm, *params)
	default:
		return nil, nil, fmt.Errorf("unsupported algorithm: %s", params.Algorithm)
	}
}

func encryptAESGCM(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM mode: %w", err)
	}
	nonce := make([]byte, aesgcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	return aesgcm.Seal(nonce, nonce, plaintext, nil), nil
}

func encryptRSAOAEP256(rsaPub *rsa.PublicKey, params AlgorithmParams) ([]byte, *CryptoDetails, error) {
	if params.RSAOAEP256.ContentEncryptionAlgorithm == "" {
		return nil, nil, errors.New("ContentEncryptionAlgorithm required for RSA-OAEP-256 CEK generation")
	}
	cekLen, err := ecdhContentEncKeyLen(params.RSAOAEP256.ContentEncryptionAlgorithm)
	if err != nil {
		return nil, nil, err
	}
	cek := make([]byte, cekLen)
	if _, err := rand.Read(cek); err != nil {
		return nil, nil, fmt.Errorf("CEK generation failed: %w", err)
	}
	encryptedCEK, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, rsaPub, cek, nil)
	if err != nil {
		for i := range cek {
			cek[i] = 0
		}
		return nil, nil, err
	}
	return encryptedCEK, &CryptoDetails{CEK: cek}, nil
}

func encryptECDHES(ecdsaPub *ecdsa.PublicKey, params AlgorithmParams) ([]byte, *CryptoDetails, error) {
	ephemeralPriv, ephemeralPub, err := ecdhGenerateEphemeralKeyPair(ecdsaPub)
	if err != nil {
		return nil, nil, fmt.Errorf("ephemeral key generation failed: %w", err)
	}
	z, err := ecdhComputeSharedSecret(ephemeralPriv, ecdsaPub)
	if err != nil {
		return nil, nil, fmt.Errorf("ECDH key agreement failed: %w", err)
	}
	if params.ECDHES.ContentEncryptionAlgorithm == "" {
		return nil, nil, errors.New("ContentEncryptionAlgorithm required for ECDH-ES key derivation")
	}
	keyLen, err := ecdhContentEncKeyLen(params.ECDHES.ContentEncryptionAlgorithm)
	if err != nil {
		return nil, nil, err
	}
	derivedCEK, err := ecdhConcatKDF(
		z, string(params.ECDHES.ContentEncryptionAlgorithm), keyLen, params.ECDHES.APU, params.ECDHES.APV)
	if err != nil {
		return nil, nil, fmt.Errorf("key derivation failed: %w", err)
	}
	return nil, &CryptoDetails{EPK: ephemeralPub, CEK: derivedCEK}, nil
}

func encryptECDHESKW(ecdsaPub *ecdsa.PublicKey, params AlgorithmParams) ([]byte, *CryptoDetails, error) {
	ephemeralPriv, ephemeralPub, err := ecdhGenerateEphemeralKeyPair(ecdsaPub)
	if err != nil {
		return nil, nil, fmt.Errorf("ephemeral key generation failed: %w", err)
	}
	z, err := ecdhComputeSharedSecret(ephemeralPriv, ecdsaPub)
	if err != nil {
		return nil, nil, fmt.Errorf("ECDH key agreement failed: %w", err)
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
		return nil, nil, fmt.Errorf("key derivation failed: %w", err)
	}
	if params.ECDHES.ContentEncryptionAlgorithm == "" {
		return nil, nil, errors.New("ContentEncryptionAlgorithm required for ECDH-ES+KW CEK generation")
	}
	cekLen, err := ecdhContentEncKeyLen(params.ECDHES.ContentEncryptionAlgorithm)
	if err != nil {
		return nil, nil, err
	}
	cek := make([]byte, cekLen)
	if _, err := rand.Read(cek); err != nil {
		return nil, nil, fmt.Errorf("CEK generation failed: %w", err)
	}
	wrappedKey, err := ecdhAESKeyWrap(kek, cek)
	if err != nil {
		for i := range cek {
			cek[i] = 0
		}
		for i := range kek {
			kek[i] = 0
		}
		return nil, nil, fmt.Errorf("AES key wrap failed: %w", err)
	}
	return wrappedKey, &CryptoDetails{EPK: ephemeralPub, CEK: cek}, nil
}

func encryptRSAOAEP(rsaPub *rsa.PublicKey, params AlgorithmParams) ([]byte, *CryptoDetails, error) {
	if params.RSAOAEP.ContentEncryptionAlgorithm == "" {
		return nil, nil, errors.New("ContentEncryptionAlgorithm required for RSA-OAEP CEK generation")
	}
	cekLen, err := ecdhContentEncKeyLen(params.RSAOAEP.ContentEncryptionAlgorithm)
	if err != nil {
		return nil, nil, err
	}
	cek := make([]byte, cekLen)
	if _, err := rand.Read(cek); err != nil {
		return nil, nil, fmt.Errorf("CEK generation failed: %w", err)
	}
	encryptedCEK, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, rsaPub, cek, nil) //nolint:gosec
	if err != nil {
		for i := range cek {
			cek[i] = 0
		}
		return nil, nil, err
	}
	return encryptedCEK, &CryptoDetails{CEK: cek}, nil
}

func encryptAESKW(kek []byte, alg Algorithm, params AlgorithmParams) ([]byte, *CryptoDetails, error) {
	expectedLen := 16
	switch alg {
	case AlgorithmA192KW:
		expectedLen = 24
	case AlgorithmA256KW:
		expectedLen = 32
	}
	if len(kek) != expectedLen {
		return nil, nil, fmt.Errorf("%s requires a %d-byte key, got %d", alg, expectedLen, len(kek))
	}
	if params.AESKW.ContentEncryptionAlgorithm == "" {
		return nil, nil, fmt.Errorf("ContentEncryptionAlgorithm required for %s CEK generation", alg)
	}
	cekLen, err := ecdhContentEncKeyLen(params.AESKW.ContentEncryptionAlgorithm)
	if err != nil {
		return nil, nil, err
	}
	cek := make([]byte, cekLen)
	if _, err := rand.Read(cek); err != nil {
		return nil, nil, fmt.Errorf("CEK generation failed: %w", err)
	}
	wrappedKey, err := ecdhAESKeyWrap(kek, cek)
	if err != nil {
		for i := range cek {
			cek[i] = 0
		}
		return nil, nil, fmt.Errorf("AES key wrap failed: %w", err)
	}
	return wrappedKey, &CryptoDetails{CEK: cek}, nil
}

func encryptAESGCMKW(kek []byte, alg Algorithm, params AlgorithmParams) ([]byte, *CryptoDetails, error) {
	expectedLen := 16
	switch alg {
	case AlgorithmA192GCMKW:
		expectedLen = 24
	case AlgorithmA256GCMKW:
		expectedLen = 32
	}
	if len(kek) != expectedLen {
		return nil, nil, fmt.Errorf("%s requires a %d-byte key, got %d", alg, expectedLen, len(kek))
	}
	if params.AESGCMKW.ContentEncryptionAlgorithm == "" {
		return nil, nil, fmt.Errorf("ContentEncryptionAlgorithm required for %s CEK generation", alg)
	}
	cekLen, err := ecdhContentEncKeyLen(params.AESGCMKW.ContentEncryptionAlgorithm)
	if err != nil {
		return nil, nil, err
	}
	cek := make([]byte, cekLen)
	if _, err := rand.Read(cek); err != nil {
		return nil, nil, fmt.Errorf("CEK generation failed: %w", err)
	}
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	iv := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(iv); err != nil {
		return nil, nil, err
	}
	sealed := gcm.Seal(nil, iv, cek, nil)
	tagSize := gcm.Overhead()
	encryptedKey := sealed[:len(sealed)-tagSize]
	tag := sealed[len(sealed)-tagSize:]
	return encryptedKey, &CryptoDetails{CEK: cek, IV: iv, Tag: tag}, nil
}
