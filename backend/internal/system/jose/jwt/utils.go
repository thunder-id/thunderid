// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package jwt

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// DecodeJWT decodes a JWT string and returns its header and payload as maps.
func DecodeJWT(token string) (map[string]interface{}, map[string]interface{}, error) {
	header, err := DecodeJWTHeader(token)
	if err != nil {
		return nil, nil, err
	}

	payload, err := DecodeJWTPayload(token)
	if err != nil {
		return nil, nil, err
	}

	return header, payload, nil
}

// DecodeJWTPayload decodes the payload of a JWT token and returns it as a map.
func DecodeJWTPayload(jwtToken string) (map[string]interface{}, error) {
	parts := strings.Split(jwtToken, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid JWT format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims map[string]interface{}
	if err = json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JWT claims: %w", err)
	}

	return claims, nil
}

// DecodeJWTHeader decodes the header of a JWT token and returns it as a map.
func DecodeJWTHeader(jwtToken string) (map[string]interface{}, error) {
	parts := strings.Split(jwtToken, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid JWT format")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT header: %w", err)
	}

	var header map[string]interface{}
	if err = json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JWT header: %w", err)
	}

	return header, nil
}

// IsRefreshTokenType reports whether a typ header names a refresh token.
func IsRefreshTokenType(typ string) bool {
	return strings.EqualFold(typ, TokenTypeRefreshToken)
}

// IsLegacyRefreshTokenType reports whether a typ header is the generic type that refresh tokens
// minted before rt+jwt carry. Such a token is a refresh token only if it also has access_token_sub.
//
// TODO: Remove this and its callers on the next major version, once no pre-rt+jwt refresh token
// can still be valid; refresh tokens will then be identified by their typ header alone.
func IsLegacyRefreshTokenType(typ string) bool {
	return strings.EqualFold(typ, TokenTypeJWT)
}
