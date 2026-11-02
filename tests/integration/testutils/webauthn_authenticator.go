// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// VirtualAuthenticator is a software WebAuthn authenticator for integration tests. It holds an
// ES256 key pair and produces the byte structures a browser would return from a real authenticator,
// so tests can complete passkey registration and authentication ceremonies end to end.
//
// One instance owns exactly one credential ID. A test that needs a second distinct credential must
// construct a second instance.
type VirtualAuthenticator struct {
	key       *ecdsa.PrivateKey
	aaguid    [16]byte
	credID    []byte
	signCount uint32
	rpID      string
	origin    string
}

// WebAuthn authenticator data flags.
const (
	flagUserPresent            = 0x01
	flagUserVerified           = 0x04
	flagBackupEligible         = 0x08
	flagBackupState            = 0x10
	flagAttestedCredentialData = 0x40
)

// NewVirtualAuthenticator creates an authenticator bound to a relying party ID and origin. The
// origin must be one the server accepts: for the direct passkey APIs that is an entry under
// passkey.allowed_origins in deployment.yaml, and for flow-based passkey it is an entry in the
// application's passkeyAllowedOrigins.
func NewVirtualAuthenticator(rpID, origin string) (*VirtualAuthenticator, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate authenticator key: %w", err)
	}

	credID := make([]byte, 32)
	if _, err := rand.Read(credID); err != nil {
		return nil, fmt.Errorf("failed to generate credential ID: %w", err)
	}

	return &VirtualAuthenticator{
		key:    key,
		credID: credID,
		rpID:   rpID,
		origin: origin,
	}, nil
}

// CredentialID returns the base64url encoded credential ID held by this authenticator.
func (a *VirtualAuthenticator) CredentialID() string {
	return base64.RawURLEncoding.EncodeToString(a.credID)
}

// SetSignCount overrides the signature counter, so tests can simulate a counter regression.
func (a *VirtualAuthenticator) SetSignCount(count uint32) {
	a.signCount = count
}

// CreateAttestationResponse produces the credential a browser returns from navigator.credentials
// .create(), for the registration ceremony. The challenge must be the value from the corresponding
// start response, passed through unchanged. All returned values are base64url encoded.
func (a *VirtualAuthenticator) CreateAttestationResponse(challenge string, userVerified bool) (
	credentialID, clientDataJSON, attestationObject string, err error) {
	clientData := a.buildClientData("webauthn.create", challenge)

	flags := byte(flagUserPresent | flagBackupEligible | flagBackupState | flagAttestedCredentialData)
	if userVerified {
		flags |= flagUserVerified
	}
	authData := a.buildAuthenticatorData(flags, true)

	attestation := cborMap3(
		cborTextString("fmt"), cborTextString("none"),
		cborTextString("attStmt"), []byte{0xA0},
		cborTextString("authData"), cborByteString(authData),
	)

	return a.CredentialID(),
		base64.RawURLEncoding.EncodeToString(clientData),
		base64.RawURLEncoding.EncodeToString(attestation),
		nil
}

// CreateAssertionResponse produces the credential a browser returns from navigator.credentials.get(),
// for the authentication ceremony. The challenge must be the value from the corresponding start
// response, passed through unchanged. All returned values are base64url encoded.
func (a *VirtualAuthenticator) CreateAssertionResponse(challenge string, userVerified bool) (
	credentialID, clientDataJSON, authenticatorData, signature string, err error) {
	clientData := a.buildClientData("webauthn.get", challenge)

	flags := byte(flagUserPresent | flagBackupEligible | flagBackupState)
	if userVerified {
		flags |= flagUserVerified
	}
	a.signCount++
	authData := a.buildAuthenticatorData(flags, false)

	// The assertion is signed over the authenticator data concatenated with the hash of the client
	// data, per the WebAuthn assertion signature format.
	clientDataHash := sha256.Sum256(clientData)
	signed := append(append([]byte{}, authData...), clientDataHash[:]...)
	digest := sha256.Sum256(signed)

	sig, err := ecdsa.SignASN1(rand.Reader, a.key, digest[:])
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to sign assertion: %w", err)
	}

	return a.CredentialID(),
		base64.RawURLEncoding.EncodeToString(clientData),
		base64.RawURLEncoding.EncodeToString(authData),
		base64.RawURLEncoding.EncodeToString(sig),
		nil
}

// buildClientData assembles the collected client data a browser sends. The challenge is echoed
// exactly as received, since the server compares it against the base64url value it issued.
func (a *VirtualAuthenticator) buildClientData(ceremonyType, challenge string) []byte {
	clientData := map[string]interface{}{
		"type":        ceremonyType,
		"challenge":   challenge,
		"origin":      a.origin,
		"crossOrigin": false,
	}
	encoded, err := json.Marshal(clientData)
	if err != nil {
		// The map contains only strings and a bool, so marshalling cannot fail.
		panic(fmt.Sprintf("failed to marshal client data: %v", err))
	}
	return encoded
}

// buildAuthenticatorData assembles the authenticator data structure. When includeCredential is set
// the attested credential data (AAGUID, credential ID and COSE public key) is appended, which is
// required for registration and absent for authentication.
func (a *VirtualAuthenticator) buildAuthenticatorData(flags byte, includeCredential bool) []byte {
	rpIDHash := sha256.Sum256([]byte(a.rpID))

	authData := make([]byte, 0, 37)
	authData = append(authData, rpIDHash[:]...)
	authData = append(authData, flags)
	authData = binary.BigEndian.AppendUint32(authData, a.signCount)

	if !includeCredential {
		return authData
	}

	authData = append(authData, a.aaguid[:]...)
	authData = binary.BigEndian.AppendUint16(authData, uint16(len(a.credID)))
	authData = append(authData, a.credID...)
	authData = append(authData, a.coseKey()...)

	return authData
}

// coseKey encodes the public key as a COSE_Key structure for an ES256 key on the P-256 curve. The
// map is written directly rather than through a CBOR library, since its shape is fixed: the test
// module depends only on testify, and this keeps the encoded bytes inspectable when a verification
// failure needs debugging.
func (a *VirtualAuthenticator) coseKey() []byte {
	x := make([]byte, 32)
	y := make([]byte, 32)
	a.key.PublicKey.X.FillBytes(x)
	a.key.PublicKey.Y.FillBytes(y)

	key := []byte{
		0xA5,       // map with 5 pairs
		0x01, 0x02, // kty: EC2
		0x03, 0x26, // alg: -7 (ES256)
		0x20, 0x01, // crv: 1 (P-256)
		0x21, 0x58, 0x20, // x: byte string of 32
	}
	key = append(key, x...)
	key = append(key, 0x22, 0x58, 0x20) // y: byte string of 32
	key = append(key, y...)

	return key
}

// cborTextString encodes a short text string. Only lengths below 24 occur here, so the length is
// packed into the initial byte.
func cborTextString(s string) []byte {
	if len(s) >= 24 {
		panic(fmt.Sprintf("cborTextString only supports strings shorter than 24 bytes, got %d", len(s)))
	}
	return append([]byte{0x60 | byte(len(s))}, []byte(s)...)
}

// cborByteString encodes a byte string with the smallest length header that fits.
func cborByteString(b []byte) []byte {
	var header []byte
	switch {
	case len(b) < 24:
		header = []byte{0x40 | byte(len(b))}
	case len(b) < 256:
		header = []byte{0x58, byte(len(b))}
	default:
		header = binary.BigEndian.AppendUint16([]byte{0x59}, uint16(len(b)))
	}
	return append(header, b...)
}

// cborMap3 assembles a CBOR map of exactly three already encoded key and value pairs.
func cborMap3(k1, v1, k2, v2, k3, v3 []byte) []byte {
	out := []byte{0xA3}
	for _, part := range [][]byte{k1, v1, k2, v2, k3, v3} {
		out = append(out, part...)
	}
	return out
}
