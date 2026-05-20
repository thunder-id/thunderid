/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

// Package jws provides functionalities for handling JSON Web Signatures (JWS).
package jws

import (
	"crypto"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/cryptolab"
)

// DecodeHeader decodes the header of a JWS token and returns it as a map.
func DecodeHeader(token string) (map[string]interface{}, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid JWS token format")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWS header: %w", err)
	}

	var header map[string]interface{}
	if err = json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JWS header: %w", err)
	}

	return header, nil
}

// MapAlgorithmToSignAlg maps JWS alg header values to internal SignAlgorithm.
func MapAlgorithmToSignAlg(jwsAlg Algorithm) (cryptolab.SignAlgorithm, error) {
	switch jwsAlg {
	case RS256:
		return cryptolab.RSASHA256, nil
	case RS512:
		return cryptolab.RSASHA512, nil
	case PS256:
		return cryptolab.RSAPSSSHA256, nil
	case ES256:
		return cryptolab.ECDSASHA256, nil
	case ES384:
		return cryptolab.ECDSASHA384, nil
	case ES512:
		return cryptolab.ECDSASHA512, nil
	case EdDSA:
		return cryptolab.ED25519, nil
	default:
		return "", fmt.Errorf("unsupported JWS alg: %s", jwsAlg)
	}
}

// JWKToPublicKey converts a JWK map to a crypto.PublicKey supporting RSA, EC, and Ed25519.
func JWKToPublicKey(jwk map[string]interface{}) (crypto.PublicKey, error) {
	kty, ok := jwk["kty"].(string)
	if !ok {
		return nil, errors.New("JWK missing kty")
	}

	switch kty {
	case "RSA":
		return jwkToRSAPublicKey(jwk)
	case "EC":
		return JWKToECPublicKey(jwk)
	case "OKP":
		return jwkToOKPPublicKey(jwk)
	default:
		return nil, fmt.Errorf("unsupported JWK kty: %s", kty)
	}
}

// jwkToRSAPublicKey converts a JWK to an RSA public key.
func jwkToRSAPublicKey(jwk map[string]interface{}) (*rsa.PublicKey, error) {
	nStr, nOK := jwk["n"].(string)
	eStr, eOK := jwk["e"].(string)
	if !nOK || !eOK {
		return nil, errors.New("JWK missing RSA modulus or exponent")
	}

	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode RSA modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode RSA exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes).Int64()
	if e <= 0 {
		return nil, errors.New("invalid RSA exponent")
	}

	return &rsa.PublicKey{N: n, E: int(e)}, nil
}

// JWKToECPublicKey converts a JWK to an EC public key.
func JWKToECPublicKey(jwk map[string]interface{}) (*ecdh.PublicKey, error) {
	crv, crvOK := jwk["crv"].(string)
	xStr, xOK := jwk["x"].(string)
	yStr, yOK := jwk["y"].(string)
	if !crvOK || !xOK || !yOK {
		return nil, errors.New("JWK missing EC parameters")
	}

	curve, expectedKeySize, err := getECCurveInfo(crv)
	if err != nil {
		return nil, err
	}

	xBytes, err := base64.RawURLEncoding.DecodeString(xStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode EC x: %w", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(yStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode EC y: %w", err)
	}

	if len(xBytes) != expectedKeySize || len(yBytes) != expectedKeySize {
		return nil, errors.New("invalid EC coordinate length")
	}

	// Construct the uncompressed point encoding: 0x04 || x || y
	uncompressed := make([]byte, 1+len(xBytes)+len(yBytes))
	uncompressed[0] = 0x04 // uncompressed point marker
	copy(uncompressed[1:], xBytes)
	copy(uncompressed[1+len(xBytes):], yBytes)

	// NewPublicKey performs on-curve validation automatically
	return curve.NewPublicKey(uncompressed)
}

// getECCurveInfo returns the elliptic curve and expected key size for a given curve name.
func getECCurveInfo(crv string) (ecdh.Curve, int, error) {
	switch crv {
	case P256:
		return ecdh.P256(), 32, nil
	case P384:
		return ecdh.P384(), 48, nil
	case P521:
		return ecdh.P521(), 66, nil
	default:
		return nil, 0, fmt.Errorf("unsupported EC curve: %s", crv)
	}
}

// jwkToOKPPublicKey converts a JWK to an OKP public key.
func jwkToOKPPublicKey(jwk map[string]interface{}) (ed25519.PublicKey, error) {
	crv, crvOK := jwk["crv"].(string)
	xStr, xOK := jwk["x"].(string)
	if !crvOK || !xOK {
		return nil, errors.New("JWK missing OKP parameters")
	}

	switch crv {
	case "Ed25519":
		xBytes, err := base64.RawURLEncoding.DecodeString(xStr)
		if err != nil {
			return nil, fmt.Errorf("failed to decode Ed25519 x: %w", err)
		}
		if l := len(xBytes); l != ed25519.PublicKeySize {
			return nil, fmt.Errorf("invalid Ed25519 public key length: %d", l)
		}
		return ed25519.PublicKey(xBytes), nil
	default:
		return nil, fmt.Errorf("unsupported OKP curve: %s", crv)
	}
}
