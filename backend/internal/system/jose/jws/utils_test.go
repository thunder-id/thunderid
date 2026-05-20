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

package jws

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cryptolab"
)

type JWSUtilsTestSuite struct {
	suite.Suite
	rsaPrivateKey *rsa.PrivateKey
	rsaPublicKey  *rsa.PublicKey
	ecPrivateKey  *ecdsa.PrivateKey
	ecPublicKey   *ecdsa.PublicKey
	edPrivateKey  ed25519.PrivateKey
	edPublicKey   ed25519.PublicKey
}

func TestJWSUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(JWSUtilsTestSuite))
}

func (suite *JWSUtilsTestSuite) SetupSuite() {
	// Generate RSA key pair
	var err error
	suite.rsaPrivateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(suite.T(), err)
	suite.rsaPublicKey = &suite.rsaPrivateKey.PublicKey

	// Generate EC key pair
	suite.ecPrivateKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.NoError(suite.T(), err)
	suite.ecPublicKey = &suite.ecPrivateKey.PublicKey

	// Generate Ed25519 key pair
	suite.edPublicKey, suite.edPrivateKey, err = ed25519.GenerateKey(rand.Reader)
	assert.NoError(suite.T(), err)
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

func (suite *JWSUtilsTestSuite) TestMapAlgorithmToSignAlgAllSupported() {
	testCases := []struct {
		name        string
		alg         Algorithm
		expectedAlg cryptolab.SignAlgorithm
	}{
		{"RS256", RS256, cryptolab.RSASHA256},
		{"RS512", RS512, cryptolab.RSASHA512},
		{"PS256", PS256, cryptolab.RSAPSSSHA256},
		{"ES256", ES256, cryptolab.ECDSASHA256},
		{"ES384", ES384, cryptolab.ECDSASHA384},
		{"ES512", ES512, cryptolab.ECDSASHA512},
		{"EdDSA", EdDSA, cryptolab.ED25519},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			alg, err := MapAlgorithmToSignAlg(tc.alg)

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedAlg, alg)
		})
	}
}

func (suite *JWSUtilsTestSuite) TestMapAlgorithmToSignAlgUnsupported() {
	testCases := []struct {
		name string
		alg  Algorithm
	}{
		{"HS256", Algorithm("HS256")},
		{"HS512", Algorithm("HS512")},
		{"Unknown", Algorithm("UNKNOWN")},
		{"Empty", Algorithm("")},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			alg, err := MapAlgorithmToSignAlg(tc.alg)

			assert.Error(t, err)
			assert.Equal(t, cryptolab.SignAlgorithm(""), alg)
			assert.Contains(t, err.Error(), "unsupported JWS alg")
		})
	}
}

func (suite *JWSUtilsTestSuite) TestJWKToPublicKeyRSA() {
	n := suite.rsaPublicKey.N
	e := suite.rsaPublicKey.E

	jwk := map[string]interface{}{
		"kty": "RSA",
		"n":   base64.RawURLEncoding.EncodeToString(n.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(new(big.Int).SetInt64(int64(e)).Bytes()),
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), publicKey)
	rsaKey, ok := publicKey.(*rsa.PublicKey)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), n, rsaKey.N)
	assert.Equal(suite.T(), e, rsaKey.E)
}

func (suite *JWSUtilsTestSuite) TestJWKToPublicKeyEd25519() {
	jwk := map[string]interface{}{
		"kty": "OKP",
		"crv": "Ed25519",
		"x":   base64.RawURLEncoding.EncodeToString(suite.edPublicKey),
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), publicKey)
	edKey, ok := publicKey.(ed25519.PublicKey)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), suite.edPublicKey, edKey)
}

func (suite *JWSUtilsTestSuite) TestJWKToPublicKeyMissingKty() {
	jwk := map[string]interface{}{
		"n": "value",
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
	assert.Contains(suite.T(), err.Error(), "JWK missing kty")
}

func (suite *JWSUtilsTestSuite) TestJWKToPublicKeyInvalidKty() {
	jwk := map[string]interface{}{
		"kty": "UNKNOWN",
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
	assert.Contains(suite.T(), err.Error(), "unsupported JWK kty")
}

func (suite *JWSUtilsTestSuite) TestJWKToRSAPublicKeyMissingModulus() {
	jwk := map[string]interface{}{
		"kty": "RSA",
		"e":   "AQAB",
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
	assert.Contains(suite.T(), err.Error(), "JWK missing RSA modulus or exponent")
}

func (suite *JWSUtilsTestSuite) TestJWKToRSAPublicKeyMissingExponent() {
	jwk := map[string]interface{}{
		"kty": "RSA",
		"n":   base64.RawURLEncoding.EncodeToString(suite.rsaPublicKey.N.Bytes()),
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
}

func (suite *JWSUtilsTestSuite) TestJWKToRSAPublicKeyInvalidExponent() {
	jwk := map[string]interface{}{
		"kty": "RSA",
		"n":   base64.RawURLEncoding.EncodeToString(suite.rsaPublicKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString([]byte{0, 0, 0, 0}),
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
}

func (suite *JWSUtilsTestSuite) TestJWKToECPublicKeyMissingCurve() {
	jwk := map[string]interface{}{
		"kty": "EC",
		"x":   "value",
		"y":   "value",
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
	assert.Contains(suite.T(), err.Error(), "JWK missing EC parameters")
}

func (suite *JWSUtilsTestSuite) TestJWKToECPublicKeyUnsupportedCurve() {
	jwk := map[string]interface{}{
		"kty": "EC",
		"crv": "P-999",
		"x":   base64.RawURLEncoding.EncodeToString([]byte("value")),
		"y":   base64.RawURLEncoding.EncodeToString([]byte("value")),
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
	assert.Contains(suite.T(), err.Error(), "unsupported EC curve")
}

func (suite *JWSUtilsTestSuite) TestJWKToECPublicKeyInvalidCoordinateLength() {
	jwk := map[string]interface{}{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString([]byte("short")),
		"y":   base64.RawURLEncoding.EncodeToString([]byte("short")),
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
	assert.Contains(suite.T(), err.Error(), "invalid EC coordinate length")
}

func (suite *JWSUtilsTestSuite) TestJWKToOKPPublicKeyMissingCurve() {
	jwk := map[string]interface{}{
		"kty": "OKP",
		"x":   base64.RawURLEncoding.EncodeToString(suite.edPublicKey),
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
	assert.Contains(suite.T(), err.Error(), "JWK missing OKP parameters")
}

func (suite *JWSUtilsTestSuite) TestJWKToOKPPublicKeyUnsupportedCurve() {
	jwk := map[string]interface{}{
		"kty": "OKP",
		"crv": "X25519",
		"x":   base64.RawURLEncoding.EncodeToString(suite.edPublicKey),
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
	assert.Contains(suite.T(), err.Error(), "unsupported OKP curve")
}

func (suite *JWSUtilsTestSuite) TestJWKToOKPPublicKeyInvalidKeyLength() {
	jwk := map[string]interface{}{
		"kty": "OKP",
		"crv": "Ed25519",
		"x":   base64.RawURLEncoding.EncodeToString([]byte("short")),
	}

	publicKey, err := JWKToPublicKey(jwk)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), publicKey)
	assert.Contains(suite.T(), err.Error(), "invalid Ed25519 public key length")
}

func (suite *JWSUtilsTestSuite) TestGetECCurveInfoP256() {
	curve, keySize, err := getECCurveInfo(P256)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), ecdh.P256(), curve)
	assert.Equal(suite.T(), 32, keySize)
}

func (suite *JWSUtilsTestSuite) TestGetECCurveInfoP384() {
	curve, keySize, err := getECCurveInfo(P384)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), ecdh.P384(), curve)
	assert.Equal(suite.T(), 48, keySize)
}

func (suite *JWSUtilsTestSuite) TestGetECCurveInfoP521() {
	curve, keySize, err := getECCurveInfo(P521)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), ecdh.P521(), curve)
	assert.Equal(suite.T(), 66, keySize)
}

func (suite *JWSUtilsTestSuite) TestGetECCurveInfoUnsupported() {
	curve, keySize, err := getECCurveInfo("P-999")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), curve)
	assert.Equal(suite.T(), 0, keySize)
	assert.Contains(suite.T(), err.Error(), "unsupported EC curve")
}

func (suite *JWSUtilsTestSuite) TestJWKToPublicKeyInvalidEC() {
	// Use a known valid point on P-256
	validX := new(big.Int).SetBytes([]byte{
		0x6b, 0x17, 0xd1, 0xf2, 0xe1, 0x2c, 0x42, 0x47, 0xf8, 0xbc, 0xe6, 0xe5, 0x63, 0xa4, 0x40, 0xf2,
		0x77, 0x03, 0x7d, 0x81, 0x2d, 0xeb, 0x33, 0xa0, 0xf4, 0xa1, 0x39, 0x45, 0xd8, 0x98, 0xc2, 0x96,
	})
	validY := new(big.Int).SetBytes([]byte{
		0x4f, 0xe3, 0x42, 0xe2, 0xfe, 0x61, 0xa7, 0xf5, 0x73, 0xb3, 0x5b, 0x0b, 0x82, 0x41, 0xa8, 0xc2,
		0x8e, 0x4f, 0xb5, 0x35, 0x86, 0x4a, 0xf3, 0xd4, 0x0f, 0x55, 0xd5, 0x96, 0xb6, 0x7f, 0x4c, 0x8b,
	})

	jwk := map[string]interface{}{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(validX.Bytes()),
		"y":   base64.RawURLEncoding.EncodeToString(validY.Bytes()),
	}

	publicKey, err := JWKToPublicKey(jwk)
	assert.Contains(suite.T(), err.Error(), "point not on curve")
	assert.Nil(suite.T(), publicKey)
}
