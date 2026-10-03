// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package jws

import (
	"crypto/mldsa"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type JWSUtilsTestSuite struct {
	suite.Suite
}

func TestJWSUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(JWSUtilsTestSuite))
}

func (suite *JWSUtilsTestSuite) TestDecodeHeaderValidToken() {
	header := map[string]interface{}{
		"alg": "RS256",
		"typ": "JWT",
	}
	headerJSON, _ := json.Marshal(header)
	headerBase64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	token := headerBase64 + ".payload.signature"

	decodedHeader, err := DecodeHeader(token)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), decodedHeader)
	assert.Equal(suite.T(), "RS256", decodedHeader["alg"])
	assert.Equal(suite.T(), "JWT", decodedHeader["typ"])
}

func (suite *JWSUtilsTestSuite) TestDecodeHeaderInvalidFormat() {
	testCases := []struct {
		name          string
		token         string
		errorContains string
	}{
		{"TooFewParts", "part1.part2", "invalid JWS token format"},
		{"EmptyToken", "", "invalid JWS token format"},
		{"OnlyDots", "...", "invalid JWS token format"},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			header, err := DecodeHeader(tc.token)

			assert.Error(t, err)
			assert.Nil(t, header)
			assert.Contains(t, err.Error(), tc.errorContains)
		})
	}
}

func (suite *JWSUtilsTestSuite) TestDecodeHeaderInvalidBase64() {
	token := "invalid_base64!@#.payload.signature"

	header, err := DecodeHeader(token)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), header)
	assert.Contains(suite.T(), err.Error(), "failed to decode JWS header")
}

func (suite *JWSUtilsTestSuite) TestDecodeHeaderInvalidJSON() {
	headerBase64 := base64.RawURLEncoding.EncodeToString([]byte(`{invalid json}`))
	token := headerBase64 + ".payload.signature"

	header, err := DecodeHeader(token)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), header)
	assert.Contains(suite.T(), err.Error(), "failed to unmarshal JWS header")
}

func (suite *JWSUtilsTestSuite) TestIsValidJKT() {
	testCases := []struct {
		name  string
		input string
		want  bool
	}{
		{"Valid43CharBase64URL", "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQ", true},
		{"ValidWithDashAndUnderscore", "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKL-_012", true},
		{"ValidAllDigits", "0123456789012345678901234567890123456789012", true},
		{"TooShort", "abc", false},
		{"TooLong", "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQR", false},
		{"Empty", "", false},
		{"WithPadding", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa=", false},
		{"WithStandardBase64Slash", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/", false},
		{"WithStandardBase64Plus", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa+", false},
		{"WithWhitespace", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa ", false},
		{"WithSpecialChar", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa!", false},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsValidJKT(tc.input))
		})
	}
}

func TestComputeJKTAKPMissingMembers(t *testing.T) {
	_, err := ComputeJKT(map[string]interface{}{"kty": "AKP", "alg": "ML-DSA-65"})
	assert.ErrorContains(t, err, "AKP JWK missing required members alg/pub")

	_, err = ComputeJKT(map[string]interface{}{"kty": "AKP", "pub": "AAAA"})
	assert.ErrorContains(t, err, "AKP JWK missing required members alg/pub")
}

func TestComputeJKTAKP(t *testing.T) {
	signer, err := mldsa.GenerateKey(mldsa.MLDSA65())
	require.NoError(t, err)
	pubBytes := signer.PublicKey().Bytes()

	jwk := map[string]interface{}{
		"kty": "AKP",
		"alg": "ML-DSA-65",
		"pub": base64.RawURLEncoding.EncodeToString(pubBytes),
	}
	jkt, err := ComputeJKT(jwk)
	require.NoError(t, err)
	assert.True(t, IsValidJKT(jkt))
}
