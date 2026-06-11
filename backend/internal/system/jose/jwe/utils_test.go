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

package jwe

import (
	"crypto/aes"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

type JEWUtilsTestSuite struct {
	suite.Suite
}

func TestJEWUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(JEWUtilsTestSuite))
}

func (s *JEWUtilsTestSuite) TestEncryptDecryptContent() {
	payload := []byte("Hello, JWE!")
	aad := []byte("additional-authenticated-data")

	testCases := []struct {
		name    string
		enc     ContentEncAlgorithm
		cekSize int
	}{
		{"A128GCM", A128GCM, 16},
		{"A192GCM", A192GCM, 24},
		{"A256GCM", A256GCM, 32},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			cek := make([]byte, tc.cekSize)
			_, _ = rand.Read(cek)

			iv, ciphertext, tag, err := encryptContent(payload, cek, tc.enc, aad)
			s.NoError(err)

			decryptedPayload, err := decryptContent(ciphertext, iv, tag, cek, tc.enc, aad)
			s.NoError(err)
			s.Equal(payload, decryptedPayload)
		})
	}
}

func (s *JEWUtilsTestSuite) TestContentEncryption_Errors() {
	payload := []byte("payload")
	cek16 := make([]byte, 16)
	cekInvalid := []byte("too-short")

	// Encrypt errors
	_, _, _, err := encryptContent(payload, cekInvalid, A128GCM, nil)
	s.Error(err)
	_, _, _, err = encryptContent(payload, cek16, "INVALID", nil)
	s.Error(err)

	// Decrypt errors
	_, err = decryptContent(payload, cek16, cek16, cekInvalid, A128GCM, nil)
	s.Error(err)
	_, err = decryptContent(payload, cek16, cek16, cek16, "INVALID", nil)
	s.Error(err)
}

func (s *JEWUtilsTestSuite) TestDecodeJWE() {
	header := `{"alg":"RSA-OAEP-256","enc":"A128GCM"}`
	headerBase64 := base64.RawURLEncoding.EncodeToString([]byte(header))
	encryptedKey := base64.RawURLEncoding.EncodeToString([]byte("encrypted-key"))
	iv := base64.RawURLEncoding.EncodeToString([]byte("iv-iv-iv-iv-"))
	ciphertext := base64.RawURLEncoding.EncodeToString([]byte("ciphertext"))
	tag := base64.RawURLEncoding.EncodeToString([]byte("tag-tag-tag-tag-"))

	jweToken := fmt.Sprintf("%s.%s.%s.%s.%s", headerBase64, encryptedKey, iv, ciphertext, tag)

	decodedHeader, _, decodedEncryptedKey, _, _, _, err := DecodeJWE(jweToken)
	s.NoError(err)
	s.Equal("RSA-OAEP-256", decodedHeader["alg"])
	s.Equal([]byte("encrypted-key"), decodedEncryptedKey)
}

func (s *JEWUtilsTestSuite) TestDecodeJWE_Errors() {
	// Wrong number of parts
	_, _, _, _, _, _, err := DecodeJWE("a.b.c.d")
	s.Error(err)

	// Invalid base64
	_, _, _, _, _, _, err = DecodeJWE("@@.b.c.d.e")
	s.Error(err)
	_, _, _, _, _, _, err = DecodeJWE("YQ.@@.c.d.e")
	s.Error(err)
	_, _, _, _, _, _, err = DecodeJWE("YQ.YQ.@@.d.e")
	s.Error(err)
	_, _, _, _, _, _, err = DecodeJWE("YQ.YQ.YQ.@@.e")
	s.Error(err)
	_, _, _, _, _, _, err = DecodeJWE("YQ.YQ.YQ.YQ.@@")
	s.Error(err)

	// Invalid JSON header
	_, _, _, _, _, _, err = DecodeJWE(base64.RawURLEncoding.EncodeToString([]byte("{invalid}")) + ".b.c.d.e")
	s.Error(err)
}

func (s *JEWUtilsTestSuite) TestIsSupportedCurve() {
	tests := []struct {
		name     string
		curve    elliptic.Curve
		expected bool
	}{
		{"P-256", elliptic.P256(), true},
		{"P-384", elliptic.P384(), true},
		{"P-521", elliptic.P521(), true},
		{"P-224", elliptic.P224(), false}, // Not supported
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := isSupportedCurve(tt.curve)
			s.Equal(tt.expected, result)
		})
	}
}

func (s *JEWUtilsTestSuite) TestInvalidAlgorithmCombinations() {
	cek := make([]byte, 32)
	_, _ = rand.Read(cek)

	s.Run("UnsupportedContentEncryptionAlgorithm", func() {
		_, _, _, err := encryptContent([]byte("test"), cek, ContentEncAlgorithm("INVALID-ENC"), nil)
		s.Error(err)
		s.Contains(err.Error(), "unsupported encryption algorithm")
	})
}

func (s *JEWUtilsTestSuite) TestInvalidDecryptionScenarios() {
	s.Run("InvalidContentDecryption", func() {
		cek := make([]byte, 16)
		iv := make([]byte, 12)
		tag := make([]byte, 16)
		ciphertext := []byte("fake-ciphertext")

		_, err := decryptContent(ciphertext, iv, tag, cek, ContentEncAlgorithm("INVALID-ENC"), nil)
		s.Error(err)
		s.Contains(err.Error(), "unsupported encryption algorithm")
	})
}

func (s *JEWUtilsTestSuite) TestEpkToMapEdgeCases() {
	// Test with P-384 curve
	privKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	s.NoError(err)

	// Convert ECDSA public key to ECDH public key
	ecdhPub, err := privKey.PublicKey.ECDH()
	s.NoError(err)

	epkMap, err := epkToMap(ecdhPub)
	s.NoError(err)
	s.NotNil(epkMap)
	s.Equal("P-384", epkMap["crv"])

	// Test with P-521 curve
	privKey521, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	s.NoError(err)

	ecdhPub521, err := privKey521.PublicKey.ECDH()
	s.NoError(err)

	epkMap521, err := epkToMap(ecdhPub521)
	s.NoError(err)
	s.NotNil(epkMap521)
	s.Equal("P-521", epkMap521["crv"])

	// Test with non-ECDH key
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	s.NoError(err)

	epkMap, err = epkToMap(&rsaKey.PublicKey)
	s.Error(err)
	s.Nil(epkMap)
}

func (s *JEWUtilsTestSuite) TestDecodeJWEWithDifferentHeaders() {
	// Test with ECDH-ES header containing epk
	epkHeader := map[string]interface{}{
		"alg": "ECDH-ES",
		"enc": "A256GCM",
		"epk": map[string]interface{}{
			"kty": "EC",
			"crv": "P-256",
			"x":   "WKn-ZIGevcwGIyyrzFoZNBdaq9_TsqzGHwHitJBcBmQ",
			"y":   "y77As5vbZdIgh9BzxPztXDBhKwuDiAv6rU9xDPVv3rI",
		},
	}

	headerJSON, _ := json.Marshal(epkHeader)
	headerBase64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	encryptedKey := base64.RawURLEncoding.EncodeToString([]byte{})
	iv := base64.RawURLEncoding.EncodeToString([]byte("123456789012"))
	ciphertext := base64.RawURLEncoding.EncodeToString([]byte("encrypted-data"))
	tag := base64.RawURLEncoding.EncodeToString([]byte("auth-tag-here!!!"))

	jweToken := fmt.Sprintf("%s.%s.%s.%s.%s", headerBase64, encryptedKey, iv, ciphertext, tag)

	decodedHeader, headerExtras, _, _, _, _, err := DecodeJWE(jweToken)
	s.NoError(err)
	s.Equal("ECDH-ES", decodedHeader["alg"])
	s.NotNil(headerExtras)

	// Test header missing mandatory fields
	incompleteHeader := map[string]interface{}{
		"alg": "RSA-OAEP-256",
		// missing "enc"
	}
	headerJSON, _ = json.Marshal(incompleteHeader)
	headerBase64 = base64.RawURLEncoding.EncodeToString(headerJSON)
	incompleteJWE := fmt.Sprintf("%s.%s.%s.%s.%s", headerBase64, encryptedKey, iv, ciphertext, tag)

	// DecodeJWE should succeed even with missing fields - validation happens later
	_, _, _, _, _, _, err = DecodeJWE(incompleteJWE)
	s.NoError(err)
}

// isSupportedCurve checks if the elliptic curve is supported.
func isSupportedCurve(curve elliptic.Curve) bool {
	switch curve {
	case elliptic.P256(), elliptic.P384(), elliptic.P521():
		return true
	default:
		return false
	}
}

func (s *JEWUtilsTestSuite) TestEncryptDecryptContent_CBC() {
	payload := []byte("Hello, CBC JWE!")
	aad := []byte("additional-authenticated-data")

	testCases := []struct {
		name    string
		enc     ContentEncAlgorithm
		cekSize int
	}{
		{"A128CBC-HS256", A128CBCHS256, 32},
		{"A192CBC-HS384", A192CBCHS384, 48},
		{"A256CBC-HS512", A256CBCHS512, 64},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			cek := make([]byte, tc.cekSize)
			_, _ = rand.Read(cek)

			iv, ciphertext, tag, err := encryptContent(payload, cek, tc.enc, aad)
			s.NoError(err)

			decryptedPayload, err := decryptContent(ciphertext, iv, tag, cek, tc.enc, aad)
			s.NoError(err)
			s.Equal(payload, decryptedPayload)
		})
	}
}

func (s *JEWUtilsTestSuite) TestCBCEncryptErrors() {
	payload := []byte("test payload for CBC")
	aad := []byte("aad")

	// A128CBCHS256 requires a 32-byte CEK; 10 bytes should fail
	_, _, _, err := encryptWithCBC(payload, make([]byte, 10), A128CBCHS256, aad)
	s.Error(err)
	s.Contains(err.Error(), "requires a")
}

func (s *JEWUtilsTestSuite) TestCBCDecryptErrors() {
	payload := []byte("test payload for CBC")
	aad := []byte("aad")
	cek := make([]byte, 32)
	_, _ = rand.Read(cek)

	iv, ciphertext, tag, encErr := encryptWithCBC(payload, cek, A128CBCHS256, aad)
	s.Require().NoError(encErr)

	// Wrong CEK size
	_, err := decryptWithCBC(ciphertext, iv, tag, make([]byte, 10), A128CBCHS256, aad)
	s.Error(err)
	s.Contains(err.Error(), "requires a")

	// Tag mismatch
	wrongTag := make([]byte, len(tag))
	_, _ = rand.Read(wrongTag)
	_, err = decryptWithCBC(ciphertext, iv, wrongTag, cek, A128CBCHS256, aad)
	s.Error(err)
	s.Contains(err.Error(), "authentication tag mismatch")

	// Invalid IV length — compute a matching tag for the short IV to bypass the HMAC check
	shortIV := make([]byte, 8)
	halfLen, hashAlg, _ := cbcParams(A128CBCHS256)
	tagForShortIV := cbcHMACTag(hashAlg, cek[:halfLen], aad, shortIV, ciphertext, halfLen)
	_, err = decryptWithCBC(ciphertext, shortIV, tagForShortIV, cek, A128CBCHS256, aad)
	s.Error(err)
	s.Contains(err.Error(), "invalid CBC IV length")

	// Non-block-aligned ciphertext — compute a matching tag for the misaligned ciphertext
	nonAligned := make([]byte, 17)
	_, _ = rand.Read(nonAligned)
	tagForNonAligned := cbcHMACTag(hashAlg, cek[:halfLen], aad, iv, nonAligned, halfLen)
	_, err = decryptWithCBC(nonAligned, iv, tagForNonAligned, cek, A128CBCHS256, aad)
	s.Error(err)
	s.Contains(err.Error(), "not a multiple of AES block size")
}

func (s *JEWUtilsTestSuite) TestCbcParams_Unsupported() {
	_, _, err := cbcParams("UNSUPPORTED")
	s.Error(err)
	s.Contains(err.Error(), "unsupported CBC enc algorithm")
}

func (s *JEWUtilsTestSuite) TestPKCS7Padding() {
	// Non-aligned: 5 bytes padded to one full block
	data := []byte("hello")
	padded := pkcs7Pad(data, aes.BlockSize)
	s.Equal(aes.BlockSize, len(padded))
	unpadded, err := pkcs7Unpad(padded)
	s.NoError(err)
	s.Equal(data, unpadded)

	// Block-aligned: 16 bytes gets a full extra block of padding
	data16 := make([]byte, 16)
	padded16 := pkcs7Pad(data16, aes.BlockSize)
	s.Equal(32, len(padded16))
	unpadded16, err := pkcs7Unpad(padded16)
	s.NoError(err)
	s.Equal(data16, unpadded16)

	// Empty input
	_, err = pkcs7Unpad([]byte{})
	s.Error(err)
	s.Contains(err.Error(), "empty data")

	// Zero padding byte
	_, err = pkcs7Unpad([]byte{0x00, 0x00})
	s.Error(err)
	s.Contains(err.Error(), "invalid PKCS#7 padding")

	// Padding byte > aes.BlockSize (17 = 0x11)
	_, err = pkcs7Unpad([]byte{0x11})
	s.Error(err)
	s.Contains(err.Error(), "invalid PKCS#7 padding")

	// Inconsistent padding bytes
	badPad := make([]byte, 16)
	badPad[15] = 0x03 // claims 3 bytes of padding
	badPad[14] = 0x03 // correct
	badPad[13] = 0x01 // wrong — should be 0x03
	_, err = pkcs7Unpad(badPad)
	s.Error(err)
	s.Contains(err.Error(), "invalid PKCS#7 padding bytes")
}

func (s *JEWUtilsTestSuite) TestEpkToMap_P256() {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	s.Require().NoError(err)

	ecdhPub, err := privKey.PublicKey.ECDH()
	s.Require().NoError(err)

	epkMap, err := epkToMap(ecdhPub)
	s.NoError(err)
	s.NotNil(epkMap)
	s.Equal("EC", epkMap["kty"])
	s.Equal("P-256", epkMap["crv"])
	s.NotEmpty(epkMap["x"])
	s.NotEmpty(epkMap["y"])
}

func (s *JEWUtilsTestSuite) TestDecryptWithGCM_InvalidNonce() {
	cek := make([]byte, 16) // A128GCM key size
	_, _ = rand.Read(cek)

	// GCM nonce size is 12; supply 8 to trigger the length check
	_, err := decryptWithGCM([]byte("ciphertext"), make([]byte, 8), []byte("tag"), cek, []byte("aad"))
	s.Error(err)
	s.Contains(err.Error(), "invalid GCM nonce length")
}

func (s *JEWUtilsTestSuite) TestExtractEPKFromHeader() {
	// Valid EPK
	privKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ecdhPub, _ := privKey.PublicKey.ECDH()
	epkMap, _ := epkToMap(ecdhPub)

	pub, err := extractEPKFromHeader(map[string]interface{}{"epk": epkMap})
	s.NoError(err)
	s.NotNil(pub)

	// Missing "epk" key
	_, err = extractEPKFromHeader(map[string]interface{}{})
	s.Error(err)
	s.Contains(err.Error(), "missing or invalid epk")

	// "epk" is a string, not a map
	_, err = extractEPKFromHeader(map[string]interface{}{"epk": "not-a-map"})
	s.Error(err)
	s.Contains(err.Error(), "missing or invalid epk")

	// "epk" is a map with invalid JWK (bad x/y coordinates)
	_, err = extractEPKFromHeader(map[string]interface{}{
		"epk": map[string]interface{}{
			"kty": "EC",
			"crv": "P-256",
			"x":   "!invalid-base64!",
			"y":   "!invalid-base64!",
		},
	})
	s.Error(err)
	s.Contains(err.Error(), "invalid epk in header")
}

func (s *JEWUtilsTestSuite) TestGetECCurveInfoP256() {
	curve, keySize, err := getECCurveInfo("P-256")
	s.NoError(err)
	s.Equal(ecdh.P256(), curve)
	s.Equal(32, keySize)
}

func (s *JEWUtilsTestSuite) TestGetECCurveInfoP384() {
	curve, keySize, err := getECCurveInfo("P-384")
	s.NoError(err)
	s.Equal(ecdh.P384(), curve)
	s.Equal(48, keySize)
}

func (s *JEWUtilsTestSuite) TestGetECCurveInfoP521() {
	curve, keySize, err := getECCurveInfo("P-521")
	s.NoError(err)
	s.Equal(ecdh.P521(), curve)
	s.Equal(66, keySize)
}

func (s *JEWUtilsTestSuite) TestGetECCurveInfoUnsupported() {
	curve, keySize, err := getECCurveInfo("P-999")
	s.Error(err)
	s.Nil(curve)
	s.Equal(0, keySize)
	s.Contains(err.Error(), "unsupported EC curve")
}

func (s *JEWUtilsTestSuite) TestJWKToECPublicKeyMissingParams() {
	_, err := jwkToECPublicKey(map[string]interface{}{"x": "val", "y": "val"})
	s.Error(err)
	s.Contains(err.Error(), "JWK missing EC parameters")
}

func (s *JEWUtilsTestSuite) TestJWKToECPublicKeyUnsupportedCurve() {
	_, err := jwkToECPublicKey(map[string]interface{}{
		"crv": "P-999",
		"x":   base64.RawURLEncoding.EncodeToString([]byte("value")),
		"y":   base64.RawURLEncoding.EncodeToString([]byte("value")),
	})
	s.Error(err)
	s.Contains(err.Error(), "unsupported EC curve")
}

func (s *JEWUtilsTestSuite) TestJWKToECPublicKeyInvalidCoordinateLength() {
	_, err := jwkToECPublicKey(map[string]interface{}{
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString([]byte("short")),
		"y":   base64.RawURLEncoding.EncodeToString([]byte("short")),
	})
	s.Error(err)
	s.Contains(err.Error(), "invalid EC coordinate length")
}

func (s *JEWUtilsTestSuite) TestJWKToECPublicKeyValid() {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	s.NoError(err)
	ecdhPub, err := privKey.PublicKey.ECDH()
	s.NoError(err)

	raw := ecdhPub.Bytes() // 0x04 || x || y
	jwk := map[string]interface{}{
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(raw[1:33]),
		"y":   base64.RawURLEncoding.EncodeToString(raw[33:]),
	}

	pub, err := jwkToECPublicKey(jwk)
	s.NoError(err)
	s.NotNil(pub)
	s.Equal(ecdhPub, pub)
}
