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

// Package cryptolib provides pure cryptographic primitives: signing, encryption,
// decryption, hashing, and secure token utilities. It has no internal dependencies
// and operates only on keys and data passed directly by the caller.
package cryptolib

import (
	gocrypto "crypto"
)

// Algorithm represents a cryptographic algorithm identifier (JWA-aligned, RFC 7518).
type Algorithm string

const (
	// AlgorithmRS256 represents RSA PKCS1v15 signature with SHA-256.
	AlgorithmRS256 Algorithm = "RS256"
	// AlgorithmRS512 represents RSA PKCS1v15 signature with SHA-512.
	AlgorithmRS512 Algorithm = "RS512"
	// AlgorithmPS256 represents RSA-PSS signature with SHA-256.
	AlgorithmPS256 Algorithm = "PS256"
	// AlgorithmES256 represents ECDSA signature with SHA-256.
	AlgorithmES256 Algorithm = "ES256"
	// AlgorithmES384 represents ECDSA signature with SHA-384.
	AlgorithmES384 Algorithm = "ES384"
	// AlgorithmES512 represents ECDSA signature with SHA-512.
	AlgorithmES512 Algorithm = "ES512"
	// AlgorithmEdDSA represents EdDSA signature algorithm.
	AlgorithmEdDSA Algorithm = "EdDSA"
	// AlgorithmMLDSA44 represents the ML-DSA-44 post-quantum signature algorithm (RFC 9964).
	AlgorithmMLDSA44 Algorithm = "ML-DSA-44"
	// AlgorithmMLDSA65 represents the ML-DSA-65 post-quantum signature algorithm (RFC 9964).
	AlgorithmMLDSA65 Algorithm = "ML-DSA-65"
	// AlgorithmMLDSA87 represents the ML-DSA-87 post-quantum signature algorithm (RFC 9964).
	AlgorithmMLDSA87 Algorithm = "ML-DSA-87"
	// AlgorithmRSAOAEP256 represents RSA-OAEP key encryption with SHA-256.
	AlgorithmRSAOAEP256 Algorithm = "RSA-OAEP-256"
	// AlgorithmECDHES represents ECDH-ES direct key agreement.
	AlgorithmECDHES Algorithm = "ECDH-ES"
	// AlgorithmECDHESA128KW represents ECDH-ES with AES-128 key wrap.
	AlgorithmECDHESA128KW Algorithm = "ECDH-ES+A128KW"
	// AlgorithmECDHESA256KW represents ECDH-ES with AES-256 key wrap.
	AlgorithmECDHESA256KW Algorithm = "ECDH-ES+A256KW"
	// AlgorithmAESGCM represents AES-GCM symmetric authenticated encryption.
	AlgorithmAESGCM Algorithm = "AES-GCM"
	// AlgorithmRSAOAEP represents RSA-OAEP key encryption with SHA-1 (RFC 7518 §4.3).
	AlgorithmRSAOAEP Algorithm = "RSA-OAEP"
	// AlgorithmECDHESA192KW represents ECDH-ES with AES-192 key wrap.
	AlgorithmECDHESA192KW Algorithm = "ECDH-ES+A192KW"
	// AlgorithmA128KW represents AES-128 Key Wrap.
	AlgorithmA128KW Algorithm = "A128KW"
	// AlgorithmA192KW represents AES-192 Key Wrap.
	AlgorithmA192KW Algorithm = "A192KW"
	// AlgorithmA256KW represents AES-256 Key Wrap.
	AlgorithmA256KW Algorithm = "A256KW"
	// AlgorithmA128GCMKW represents AES-128 GCM Key Wrap.
	AlgorithmA128GCMKW Algorithm = "A128GCMKW"
	// AlgorithmA192GCMKW represents AES-192 GCM Key Wrap.
	AlgorithmA192GCMKW Algorithm = "A192GCMKW"
	// AlgorithmA256GCMKW represents AES-256 GCM Key Wrap.
	AlgorithmA256GCMKW Algorithm = "A256GCMKW"
)

// SignAlgorithm represents the supported digital signature algorithms.
type SignAlgorithm string

const (
	// RSASHA256 represents RSA signature with SHA-256 (PKCS1v15).
	RSASHA256 SignAlgorithm = "RSA-SHA256"
	// RSASHA512 represents RSA signature with SHA-512 (PKCS1v15).
	RSASHA512 SignAlgorithm = "RSA-SHA512"
	// RSAPSSSHA256 represents RSA-PSS signature with SHA-256.
	RSAPSSSHA256 SignAlgorithm = "RSA-PSS-SHA256"
	// ECDSASHA256 represents ECDSA signature with SHA-256.
	ECDSASHA256 SignAlgorithm = "ECDSA-SHA256"
	// ECDSASHA384 represents ECDSA signature with SHA-384.
	ECDSASHA384 SignAlgorithm = "ECDSA-SHA384"
	// ECDSASHA512 represents ECDSA signature with SHA-512.
	ECDSASHA512 SignAlgorithm = "ECDSA-SHA512"
	// ED25519 represents the Ed25519 signature algorithm.
	ED25519 SignAlgorithm = "ED25519"
	// MLDSA44 represents the ML-DSA-44 post-quantum signature algorithm.
	MLDSA44 SignAlgorithm = "ML-DSA-44"
	// MLDSA65 represents the ML-DSA-65 post-quantum signature algorithm.
	MLDSA65 SignAlgorithm = "ML-DSA-65"
	// MLDSA87 represents the ML-DSA-87 post-quantum signature algorithm.
	MLDSA87 SignAlgorithm = "ML-DSA-87"
)

// AlgorithmParams carries the algorithm and any algorithm-specific inputs for a crypto operation.
// The relevant algorithm-specific ContentEncryptionAlgorithm must be set to the content encryption
// algorithm (e.g. "A128GCM") for the following operations:
//   - RSA-OAEP and RSA-OAEP-256 encrypt (determines CEK size)
//   - ECDH-ES encrypt/decrypt (determines CEK size and is used as the KDF algorithm identifier)
//   - ECDH-ES+A128KW / ECDH-ES+A192KW / ECDH-ES+A256KW encrypt (determines CEK size)
//   - A128KW / A192KW / A256KW encrypt (determines CEK size)
//   - A128GCMKW / A192GCMKW / A256GCMKW encrypt (determines CEK size)
//
// ECDH-ES+KW decrypt does not require ContentEncryptionAlgorithm because
// the KDF uses the alg value (e.g. "ECDH-ES+A128KW") directly.
// For AES-GCM Key Wrap decrypt, AESGCMKW.IV and AESGCMKW.Tag must be populated.
type AlgorithmParams struct {
	Algorithm  Algorithm
	RSAOAEP256 RSAOAEP256Params
	RSAOAEP    RSAOAEPParams
	ECDHES     ECDHESParams
	AESKW      AESKWParams
	AESGCMKW   AESGCMKWParams
}

// RSAOAEP256Params carries RSA-OAEP-256-specific inputs.
type RSAOAEP256Params struct {
	ContentEncryptionAlgorithm Algorithm
}

// RSAOAEPParams carries RSA-OAEP (SHA-1)-specific inputs.
type RSAOAEPParams struct {
	ContentEncryptionAlgorithm Algorithm
}

// AESKWParams carries AES Key Wrap-specific inputs.
type AESKWParams struct {
	ContentEncryptionAlgorithm Algorithm
}

// AESGCMKWParams carries AES-GCM Key Wrap-specific inputs.
// ContentEncryptionAlgorithm must be set during Encrypt to determine the CEK size.
// IV and Tag must be populated during Decrypt with values from the JWE protected header.
type AESGCMKWParams struct {
	ContentEncryptionAlgorithm Algorithm
	IV                         []byte
	Tag                        []byte
}

// ECDHESParams carries ECDH-ES-specific inputs.
// For ECDH-ES decrypt, EPK must be populated with the ephemeral public key from the JWE header.
// APU and APV are the raw (already base64url-decoded) apu/apv header values; pass nil when absent.
type ECDHESParams struct {
	EPK                        gocrypto.PublicKey
	ContentEncryptionAlgorithm Algorithm
	APU                        []byte
	APV                        []byte
}

// CryptoDetails carries algorithm-specific outputs from an Encrypt operation.
// EPK is the generated ephemeral public key for ECDH-ES variants, to be embedded in the JWE header.
// CEK is the content encryption key generated or derived during key establishment.
// Nil CryptoDetails is returned for algorithms that produce no extra output (e.g. AES-GCM).
// For RSA-OAEP, RSA-OAEP-256 and ECDH-ES variants, both EPK (where applicable) and CEK are populated.
// CEK is nil for AES-GCM; EPK is nil for RSA-OAEP and RSA-OAEP-256 (no ephemeral key is generated).
// IV and Tag are set only for AES-GCM Key Wrap (A128GCMKW etc.) and must be embedded in the JWE protected header.
type CryptoDetails struct {
	EPK gocrypto.PublicKey // ECDH-ES variants only; nil for RSA-OAEP, RSA-OAEP-256 and AES-GCM
	CEK []byte             // Generated or derived CEK; nil for AES-GCM
	IV  []byte             // AES-GCM Key Wrap only: IV used to wrap the CEK
	Tag []byte             // AES-GCM Key Wrap only: authentication tag from wrapping the CEK
}
