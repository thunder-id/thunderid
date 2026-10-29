// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package jws provides functionalities for handling JSON Web Signatures (JWS).
package jws

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// privateJWKMembers lists JWK parameter names that indicate private-key material: RSA, EC and OKP
// members (RFC 7518, RFC 8037), the symmetric key, and the AKP (ML-DSA) private key (RFC 9964).
var privateJWKMembers = []string{"d", "p", "q", "dp", "dq", "qi", "oth", "k", "priv"}

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

// ContainsPrivateMember reports whether the JWK contains any private-key
// parameter. Returns the offending member name when found.
func ContainsPrivateMember(jwk map[string]interface{}) (string, bool) {
	for _, m := range privateJWKMembers {
		if _, ok := jwk[m]; ok {
			return m, true
		}
	}
	return "", false
}

// ComputeJKT computes the RFC 7638 SHA-256 JWK thumbprint of a public key JWK.
func ComputeJKT(jwk map[string]interface{}) (string, error) {
	canonical, err := canonicalJWK(jwk)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// IsValidJKT reports whether s is a well-formed SHA-256 JWK thumbprint:
// 43 base64url characters with no padding (RFC 7638 with SHA-256).
func IsValidJKT(s string) bool {
	if len(s) != 43 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		case c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

func canonicalJWK(jwk map[string]interface{}) ([]byte, error) {
	kty, ok := jwk["kty"].(string)
	if !ok || kty == "" {
		return nil, errors.New("JWK missing kty")
	}
	var ordered []struct{ k, v string }
	switch kty {
	case "RSA":
		e, _ := jwk["e"].(string)
		n, _ := jwk["n"].(string)
		if e == "" || n == "" {
			return nil, errors.New("RSA JWK missing required members e/n")
		}
		ordered = []struct{ k, v string }{{"e", e}, {"kty", "RSA"}, {"n", n}}
	case "EC":
		crv, _ := jwk["crv"].(string)
		x, _ := jwk["x"].(string)
		y, _ := jwk["y"].(string)
		if crv == "" || x == "" || y == "" {
			return nil, errors.New("EC JWK missing required members crv/x/y")
		}
		ordered = []struct{ k, v string }{{"crv", crv}, {"kty", "EC"}, {"x", x}, {"y", y}}
	case "OKP":
		crv, _ := jwk["crv"].(string)
		x, _ := jwk["x"].(string)
		if crv == "" || x == "" {
			return nil, errors.New("OKP JWK missing required members crv/x")
		}
		ordered = []struct{ k, v string }{{"crv", crv}, {"kty", "OKP"}, {"x", x}}
	case "AKP":
		alg, _ := jwk["alg"].(string)
		pub, _ := jwk["pub"].(string)
		if alg == "" || pub == "" {
			return nil, errors.New("AKP JWK missing required members alg/pub")
		}
		ordered = []struct{ k, v string }{{"alg", alg}, {"kty", "AKP"}, {"pub", pub}}
	default:
		return nil, fmt.Errorf("unsupported JWK kty for thumbprint: %s", kty)
	}

	buf := make([]byte, 0, 256)
	buf = append(buf, '{')
	for i, m := range ordered {
		if i > 0 {
			buf = append(buf, ',')
		}
		kBytes, err := json.Marshal(m.k)
		if err != nil {
			return nil, err
		}
		vBytes, err := json.Marshal(m.v)
		if err != nil {
			return nil, err
		}
		buf = append(buf, kBytes...)
		buf = append(buf, ':')
		buf = append(buf, vBytes...)
	}
	buf = append(buf, '}')
	return buf, nil
}
