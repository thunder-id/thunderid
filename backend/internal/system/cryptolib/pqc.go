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

// ML-DSA PKCS#8/ASN.1 key encoding helpers (RFC 9881). These fill the gap left
// by the Go standard library's lack of ML-DSA support. When crypto/x509 gains
// ML-DSA support (Go 1.27), delete this file and replace callers with
// x509.ParsePKCS8PrivateKey / x509.MarshalPKCS8PrivateKey.

import (
	"crypto"
	"encoding/asn1"
	"errors"
	"fmt"

	"github.com/cloudflare/circl/sign"
	"github.com/cloudflare/circl/sign/schemes"
)

// errNotMLDSAKey indicates that a PKCS#8 structure does not carry an ML-DSA key.
var errNotMLDSAKey = errors.New("not an ML-DSA key")

// ML-DSA X.509 algorithm OIDs (RFC 9881, sigAlgs branch).
var (
	oidMLDSA44 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 3, 17}
	oidMLDSA65 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 3, 18}
	oidMLDSA87 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 3, 19}
)

// mldsaAlgFromOID maps an RFC 9881 ML-DSA OID to its JWA algorithm identifier.
func mldsaAlgFromOID(oid asn1.ObjectIdentifier) (Algorithm, bool) {
	switch {
	case oid.Equal(oidMLDSA44):
		return AlgorithmMLDSA44, true
	case oid.Equal(oidMLDSA65):
		return AlgorithmMLDSA65, true
	case oid.Equal(oidMLDSA87):
		return AlgorithmMLDSA87, true
	default:
		return "", false
	}
}

// mldsaOIDForAlg maps an ML-DSA JWA algorithm identifier to its RFC 9881 OID.
func mldsaOIDForAlg(alg Algorithm) (asn1.ObjectIdentifier, bool) {
	switch alg {
	case AlgorithmMLDSA44:
		return oidMLDSA44, true
	case AlgorithmMLDSA65:
		return oidMLDSA65, true
	case AlgorithmMLDSA87:
		return oidMLDSA87, true
	default:
		return nil, false
	}
}

// pkcs8ML mirrors the RFC 5958 OneAsymmetricKey structure (version,
// privateKeyAlgorithm, privateKey). The ML-DSA AlgorithmIdentifier carries no
// parameters (RFC 9881), so only the OID is decoded.
type pkcs8ML struct {
	Version    int
	Algo       struct{ Algorithm asn1.ObjectIdentifier }
	PrivateKey []byte
}

// MLDSAAlgFromPKCS8 peeks at the algorithm OID of a DER-encoded PKCS#8 key and
// reports the ML-DSA algorithm if it is one. It never errors; a non-ML-DSA or
// malformed input simply returns ("", false).
func MLDSAAlgFromPKCS8(der []byte) (Algorithm, bool) {
	var k pkcs8ML
	if _, err := asn1.Unmarshal(der, &k); err != nil {
		return "", false
	}
	return mldsaAlgFromOID(k.Algo.Algorithm)
}

// ParseMLDSAPKCS8 parses a DER-encoded RFC 9881 PKCS#8 ML-DSA private key. It
// accepts all three private-key CHOICE encodings (seed [0], expandedKey, and
// the "both" SEQUENCE), preferring the seed when present. It returns
// ErrNotMLDSAKey when the OID is not an ML-DSA OID.
func ParseMLDSAPKCS8(der []byte) (sign.PrivateKey, Algorithm, error) {
	var k pkcs8ML
	if _, err := asn1.Unmarshal(der, &k); err != nil {
		return nil, "", fmt.Errorf("failed to parse PKCS#8 structure: %w", err)
	}
	alg, ok := mldsaAlgFromOID(k.Algo.Algorithm)
	if !ok {
		return nil, "", errNotMLDSAKey
	}
	scheme := schemes.ByName(string(alg))
	if scheme == nil {
		return nil, "", ErrUnsupportedAlgorithm
	}

	seed, expanded, err := decodeMLDSAPrivateKeyChoice(k.PrivateKey, scheme.SeedSize())
	if err != nil {
		return nil, "", err
	}
	if seed != nil {
		_, sk := scheme.DeriveKey(seed)
		return sk, alg, nil
	}
	sk, err := scheme.UnmarshalBinaryPrivateKey(expanded)
	if err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal ML-DSA expanded private key: %w", err)
	}
	return sk, alg, nil
}

// decodeMLDSAPrivateKeyChoice decodes the ML-DSA-PrivateKey CHOICE (RFC 9881)
// carried inside the PKCS#8 privateKey OCTET STRING. Exactly one of seed or
// expanded is returned.
func decodeMLDSAPrivateKeyChoice(inner []byte, seedSize int) (seed, expanded []byte, err error) {
	var rv asn1.RawValue
	if _, err = asn1.Unmarshal(inner, &rv); err != nil {
		return nil, nil, fmt.Errorf("failed to parse ML-DSA private key CHOICE: %w", err)
	}

	switch {
	case rv.Class == asn1.ClassContextSpecific && rv.Tag == 0:
		// seed [0] OCTET STRING (SIZE (32)).
		seedBytes := rv.Bytes
		if rv.IsCompound {
			if _, err = asn1.Unmarshal(rv.Bytes, &seedBytes); err != nil {
				return nil, nil, fmt.Errorf("failed to parse ML-DSA seed: %w", err)
			}
		}
		if len(seedBytes) != seedSize {
			return nil, nil, fmt.Errorf("invalid ML-DSA seed length: %d", len(seedBytes))
		}
		return seedBytes, nil, nil
	case rv.Class == asn1.ClassUniversal && rv.Tag == asn1.TagSequence:
		// both ::= SEQUENCE { seed OCTET STRING, expandedKey OCTET STRING }.
		var both struct {
			Seed        []byte
			ExpandedKey []byte
		}
		if _, err = asn1.Unmarshal(inner, &both); err != nil {
			return nil, nil, fmt.Errorf("failed to parse ML-DSA both format: %w", err)
		}
		if len(both.Seed) != seedSize {
			return nil, nil, fmt.Errorf("invalid ML-DSA seed length: %d", len(both.Seed))
		}
		return both.Seed, nil, nil
	case rv.Class == asn1.ClassUniversal && rv.Tag == asn1.TagOctetString:
		// expandedKey ::= OCTET STRING.
		var expandedKey []byte
		if _, err = asn1.Unmarshal(inner, &expandedKey); err != nil {
			return nil, nil, fmt.Errorf("failed to parse ML-DSA expandedKey: %w", err)
		}
		return nil, expandedKey, nil
	default:
		return nil, nil, errors.New("unsupported ML-DSA private key CHOICE encoding")
	}
}

// mldsaSchemeFor returns the circl scheme for alg, or nil if alg is not ML-DSA.
func mldsaSchemeFor(alg Algorithm) sign.Scheme {
	switch alg {
	case AlgorithmMLDSA44, AlgorithmMLDSA65, AlgorithmMLDSA87:
		return schemes.ByName(string(alg))
	default:
		return nil
	}
}

// MLDSAAlgForPublicKey returns the JWA algorithm identifier for an ML-DSA public
// key (e.g. "ML-DSA-65"). It returns ("", false) when pub is not an ML-DSA key.
// This keeps the circl types confined to this package.
func MLDSAAlgForPublicKey(pub crypto.PublicKey) (Algorithm, bool) {
	mldsaPub, ok := pub.(sign.PublicKey)
	if !ok {
		return "", false
	}
	switch mldsaPub.Scheme().Name() {
	case string(AlgorithmMLDSA44):
		return AlgorithmMLDSA44, true
	case string(AlgorithmMLDSA65):
		return AlgorithmMLDSA65, true
	case string(AlgorithmMLDSA87):
		return AlgorithmMLDSA87, true
	default:
		return "", false
	}
}

// MLDSAPublicKeyBytes returns the raw encoding of an ML-DSA public key (for the
// AKP JWK "pub" member, RFC 9964). It returns (nil, false) when pub is not an
// ML-DSA key.
func MLDSAPublicKeyBytes(pub crypto.PublicKey) ([]byte, bool) {
	mldsaPub, ok := pub.(sign.PublicKey)
	if !ok {
		return nil, false
	}
	b, err := mldsaPub.MarshalBinary()
	if err != nil {
		return nil, false
	}
	return b, true
}

// GenerateMLDSAKey generates a new ML-DSA key pair for the given algorithm and
// returns the private key as a crypto.Signer (its public key is available via
// Public()).
func GenerateMLDSAKey(alg Algorithm) (crypto.Signer, error) {
	scheme := mldsaSchemeFor(alg)
	if scheme == nil {
		return nil, ErrUnsupportedAlgorithm
	}
	_, sk, err := scheme.GenerateKey()
	if err != nil {
		return nil, err
	}
	return sk, nil
}

// MLDSAPublicKeyFromBytes reconstructs an ML-DSA public key from its raw encoding
// (as carried in an AKP JWK "pub" member, RFC 9964).
func MLDSAPublicKeyFromBytes(alg Algorithm, pub []byte) (sign.PublicKey, error) {
	scheme := mldsaSchemeFor(alg)
	if scheme == nil {
		return nil, ErrUnsupportedAlgorithm
	}
	return scheme.UnmarshalBinaryPublicKey(pub)
}

// MarshalMLDSAPKCS8Seed encodes an ML-DSA private key as an RFC 9881 PKCS#8
// structure using the recommended seed format (privateKey ::= seed [0] OCTET
// STRING). Used to produce ML-DSA key material without external tooling.
func marshalMLDSAPKCS8Seed(alg Algorithm, seed []byte) ([]byte, error) {
	scheme := mldsaSchemeFor(alg)
	if scheme == nil {
		return nil, ErrUnsupportedAlgorithm
	}
	if len(seed) != scheme.SeedSize() {
		return nil, fmt.Errorf("invalid ML-DSA seed length: %d", len(seed))
	}
	oid, ok := mldsaOIDForAlg(alg)
	if !ok {
		return nil, ErrUnsupportedAlgorithm
	}

	inner, err := asn1.MarshalWithParams(seed, "tag:0")
	if err != nil {
		return nil, fmt.Errorf("failed to encode ML-DSA seed: %w", err)
	}
	var k pkcs8ML
	k.Version = 0
	k.Algo.Algorithm = oid
	k.PrivateKey = inner
	return asn1.Marshal(k)
}
