// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package jws

// Algorithm represents the JSON Web Signature algorithm used for signing
type Algorithm string

const (
	// RS256 represents RSA PKCS1v15 signature with SHA-256 hash for JWS
	RS256 Algorithm = "RS256"
	// RS512 represents RSA PKCS1v15 signature with SHA-512 hash for JWS
	RS512 Algorithm = "RS512"
	// PS256 represents RSA-PSS signature with SHA-256 hash for JWS
	PS256 Algorithm = "PS256"
	// PS384 represents RSA-PSS signature with SHA-384 hash for JWS
	PS384 Algorithm = "PS384"
	// ES256 represents ECDSA signature with SHA-256 hash for JWS
	ES256 Algorithm = "ES256"
	// ES384 represents ECDSA signature with SHA-384 hash for JWS
	ES384 Algorithm = "ES384"
	// ES512 represents ECDSA signature with SHA-512 hash for JWS
	ES512 Algorithm = "ES512"
	// EdDSA represents ED25519 signature algorithm for JWS
	EdDSA Algorithm = "EdDSA"
	// MLDSA44 represents the ML-DSA-44 post-quantum signature algorithm for JWS (RFC 9964)
	MLDSA44 Algorithm = "ML-DSA-44"
	// MLDSA65 represents the ML-DSA-65 post-quantum signature algorithm for JWS (RFC 9964)
	MLDSA65 Algorithm = "ML-DSA-65"
	// MLDSA87 represents the ML-DSA-87 post-quantum signature algorithm for JWS (RFC 9964)
	MLDSA87 Algorithm = "ML-DSA-87"

	// P256 represents the NIST P-256 curve
	P256 string = "P-256"
	// P384 represents the NIST P-384 curve
	P384 string = "P-384"
	// P521 represents the NIST P-521 curve
	P521 string = "P-521"
)
