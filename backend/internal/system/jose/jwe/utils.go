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
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/jose/jws"
)

// epkToMap converts an ephemeral public key to a JWK-like map representation.
func epkToMap(pub crypto.PublicKey) (map[string]interface{}, error) {
	ecdhPub, ok := pub.(*ecdh.PublicKey)
	if !ok {
		return nil, fmt.Errorf("unsupported ephemeral public key type: %T", pub)
	}

	raw := ecdhPub.Bytes()
	var crv string
	var x, y []byte

	switch len(raw) {
	case 65:
		crv = jws.P256
		x = raw[1:33]
		y = raw[33:]
	case 97:
		crv = jws.P384
		x = raw[1:49]
		y = raw[49:]
	case 133:
		crv = jws.P521
		x = raw[1:67]
		y = raw[67:]
	default:
		return nil, fmt.Errorf("unsupported ephemeral public key curve (raw length %d)", len(raw))
	}

	return map[string]interface{}{
		"kty": "EC",
		"crv": crv,
		"x":   base64.RawURLEncoding.EncodeToString(x),
		"y":   base64.RawURLEncoding.EncodeToString(y),
	}, nil
}

// encryptContent encrypts the payload using the content encryption key (CEK).
func encryptContent(payload []byte, cek []byte, enc ContentEncAlgorithm, aad []byte) ([]byte, []byte, []byte, error) {
	switch enc {
	case A128CBCHS256, A192CBCHS384, A256CBCHS512:
		return encryptWithCBC(payload, cek, enc, aad)
	case A128GCM, A192GCM, A256GCM:
		return encryptWithGCM(payload, cek, enc, aad)
	default:
		return nil, nil, nil, fmt.Errorf("unsupported encryption algorithm: %s", enc)
	}
}

// decryptContent decrypts the ciphertext using the content encryption key (CEK).
func decryptContent(ciphertext, iv, tag []byte, cek []byte, enc ContentEncAlgorithm, aad []byte) ([]byte, error) {
	switch enc {
	case A128CBCHS256, A192CBCHS384, A256CBCHS512:
		return decryptWithCBC(ciphertext, iv, tag, cek, enc, aad)
	case A128GCM, A192GCM, A256GCM:
		return decryptWithGCM(ciphertext, iv, tag, cek, aad)
	default:
		return nil, fmt.Errorf("unsupported encryption algorithm: %s", enc)
	}
}

// encryptWithGCM encrypts the payload using AES-GCM.
func encryptWithGCM(payload []byte, cek []byte, enc ContentEncAlgorithm, aad []byte) ([]byte, []byte, []byte, error) {
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, nil, nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, nil, err
	}

	_ = enc // key size already enforced by CEK length

	iv := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(iv); err != nil {
		return nil, nil, nil, err
	}

	ciphertextWithTag := gcm.Seal(nil, iv, payload, aad)
	tagSize := gcm.Overhead()
	ciphertext := ciphertextWithTag[:len(ciphertextWithTag)-tagSize]
	tag := ciphertextWithTag[len(ciphertextWithTag)-tagSize:]

	return iv, ciphertext, tag, nil
}

// decryptWithGCM decrypts the ciphertext using AES-GCM.
func decryptWithGCM(ciphertext, iv, tag []byte, cek []byte, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(iv) != gcm.NonceSize() {
		return nil, fmt.Errorf("invalid GCM nonce length: got %d, want %d", len(iv), gcm.NonceSize())
	}

	return gcm.Open(nil, iv, append(ciphertext, tag...), aad)
}

// encryptWithCBC encrypts the payload using AES-CBC + HMAC per RFC 7518 §5.2.
// Supports A128CBC-HS256 (32-byte CEK), A192CBC-HS384 (48-byte CEK), A256CBC-HS512 (64-byte CEK).
func encryptWithCBC(payload []byte, cek []byte, enc ContentEncAlgorithm, aad []byte) ([]byte, []byte, []byte, error) {
	halfLen, hashAlg, err := cbcParams(enc)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(cek) != halfLen*2 {
		return nil, nil, nil, fmt.Errorf("%s requires a %d-byte CEK, got %d", enc, halfLen*2, len(cek))
	}
	macKey := cek[:halfLen]
	encKey := cek[halfLen:]

	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return nil, nil, nil, err
	}

	padded := pkcs7Pad(payload, aes.BlockSize)
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, nil, nil, err
	}
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, padded)

	tag := cbcHMACTag(hashAlg, macKey, aad, iv, ciphertext, halfLen)
	return iv, ciphertext, tag, nil
}

// decryptWithCBC decrypts AES-CBC + HMAC per RFC 7518 §5.2.
// Supports A128CBC-HS256, A192CBC-HS384, and A256CBC-HS512.
func decryptWithCBC(ciphertext, iv, tag []byte, cek []byte, enc ContentEncAlgorithm, aad []byte) ([]byte, error) {
	halfLen, hashAlg, err := cbcParams(enc)
	if err != nil {
		return nil, err
	}
	if len(cek) != halfLen*2 {
		return nil, fmt.Errorf("%s requires a %d-byte CEK, got %d", enc, halfLen*2, len(cek))
	}
	macKey := cek[:halfLen]
	encKey := cek[halfLen:]

	expected := cbcHMACTag(hashAlg, macKey, aad, iv, ciphertext, halfLen)
	if !hmac.Equal(tag, expected) {
		return nil, fmt.Errorf("%s authentication tag mismatch", enc)
	}

	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, err
	}
	if len(iv) != aes.BlockSize {
		return nil, fmt.Errorf("invalid CBC IV length: got %d, want %d", len(iv), aes.BlockSize)
	}
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext length is not a multiple of AES block size")
	}
	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, ciphertext)
	return pkcs7Unpad(plaintext)
}

// cbcParams returns the half-CEK length and HMAC hash constructor for the given CBC enc algorithm.
// Per RFC 7518 §5.2: CEK = MAC key || ENC key, each of halfLen bytes; tag = HMAC[:halfLen].
func cbcParams(enc ContentEncAlgorithm) (halfLen int, newHash func() hash.Hash, err error) {
	switch enc {
	case A128CBCHS256:
		return 16, sha256.New, nil
	case A192CBCHS384:
		return 24, sha512.New384, nil
	case A256CBCHS512:
		return 32, sha512.New, nil
	default:
		return 0, nil, fmt.Errorf("unsupported CBC enc algorithm: %s", enc)
	}
}

// cbcHMACTag computes the authentication tag per RFC 7518 §5.2.2.1.
// MAC input: AAD || IV || ciphertext || AAD_length_bits (64-bit big-endian). Tag = HMAC[:tagLen].
func cbcHMACTag(newHash func() hash.Hash, macKey, aad, iv, ciphertext []byte, tagLen int) []byte {
	aadLenBits := make([]byte, 8)
	binary.BigEndian.PutUint64(aadLenBits, uint64(len(aad))*8) //nolint:gosec // G115: safe cast

	mac := hmac.New(newHash, macKey)
	mac.Write(aad)
	mac.Write(iv)
	mac.Write(ciphertext)
	mac.Write(aadLenBits)
	return mac.Sum(nil)[:tagLen]
}

// pkcs7Pad pads the data to a multiple of blockSize using PKCS#7.
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	return append(data, bytes.Repeat([]byte{byte(padding)}, padding)...)
}

// pkcs7Unpad removes PKCS#7 padding.
func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data for PKCS#7 unpadding")
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > aes.BlockSize {
		return nil, errors.New("invalid PKCS#7 padding")
	}
	for _, b := range data[len(data)-padding:] {
		if int(b) != padding {
			return nil, errors.New("invalid PKCS#7 padding bytes")
		}
	}
	return data[:len(data)-padding], nil
}

// jwkToECPublicKey converts a JWK to an ECDH public key for key agreement (e.g. ECDH-ES).
func jwkToECPublicKey(jwk map[string]interface{}) (*ecdh.PublicKey, error) {
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

	uncompressed := make([]byte, 1+len(xBytes)+len(yBytes))
	uncompressed[0] = 0x04
	copy(uncompressed[1:], xBytes)
	copy(uncompressed[1+len(xBytes):], yBytes)

	return curve.NewPublicKey(uncompressed)
}

// getECCurveInfo returns the ECDH curve and expected coordinate byte size for a given curve name.
func getECCurveInfo(crv string) (ecdh.Curve, int, error) {
	switch crv {
	case jws.P256:
		return ecdh.P256(), 32, nil
	case jws.P384:
		return ecdh.P384(), 48, nil
	case jws.P521:
		return ecdh.P521(), 66, nil
	default:
		return nil, 0, fmt.Errorf("unsupported EC curve: %s", crv)
	}
}

// extractEPKFromHeader parses the "epk" field from the JWE protected header and returns
// the ephemeral public key as *ecdh.PublicKey required by cryptolib.
func extractEPKFromHeader(header map[string]interface{}) (crypto.PublicKey, error) {
	epkMap, ok := header["epk"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid epk in JWE header")
	}
	ecdhPub, err := jwkToECPublicKey(epkMap)
	if err != nil {
		return nil, fmt.Errorf("invalid epk in header: %w", err)
	}
	return ecdhPub, nil
}

// decodeAPUAPV extracts and base64url-decodes the apu or apv field from the JWE
// protected header. Returns nil when the field is absent or not a string.
func decodeAPUAPV(header map[string]interface{}, key string) []byte {
	v, ok := header[key].(string)
	if !ok || v == "" {
		return nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		return nil
	}
	return decoded
}

// DecodeJWE decodes a JWE compact serialization into its five parts.
func DecodeJWE(jweToken string) (header map[string]interface{}, headerBase64 string,
	encryptedKey, iv, ciphertext, tag []byte, err error) {
	parts := strings.Split(jweToken, ".")
	if len(parts) != 5 {
		return nil, "", nil, nil, nil, nil, errors.New("invalid JWE format")
	}

	headerBase64 = parts[0]
	headerBytes, err := base64.RawURLEncoding.DecodeString(headerBase64)
	if err != nil {
		return nil, "", nil, nil, nil, nil, fmt.Errorf("failed to decode header: %w", err)
	}

	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, "", nil, nil, nil, nil, fmt.Errorf("failed to unmarshal header: %w", err)
	}

	encryptedKey, err = base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, "", nil, nil, nil, nil, fmt.Errorf("failed to decode encrypted key: %w", err)
	}

	iv, err = base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, "", nil, nil, nil, nil, fmt.Errorf("failed to decode IV: %w", err)
	}

	ciphertext, err = base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return nil, "", nil, nil, nil, nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	tag, err = base64.RawURLEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, "", nil, nil, nil, nil, fmt.Errorf("failed to decode tag: %w", err)
	}

	return header, headerBase64, encryptedKey, iv, ciphertext, tag, nil
}
