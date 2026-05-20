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
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // RSA-OAEP with SHA-1 is required by RFC 7518 §4.3
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"strings"

	cryptohash "github.com/thunder-id/thunderid/internal/system/cryptolab/hash"
	"github.com/thunder-id/thunderid/internal/system/jose/jws"
)

// encryptKey encrypts or derives the content encryption key (CEK) for a recipient.
// For ECDH-ES, the CEK is derived from the shared secret and written to the cek parameter
// slice in-place. For other algorithms, the CEK is treated as input and encrypted using
// the recipient's public key.
func encryptKey(cek []byte, alg KeyEncAlgorithm, recipientPubKey crypto.PublicKey,
	enc ContentEncAlgorithm) ([]byte, map[string]interface{}, error) {
	switch alg {
	case RSAOAEP:
		encryptedKey, err := encryptWithRSAOAEP(cek, recipientPubKey)
		return encryptedKey, nil, err

	case RSAOAEP256:
		encryptedKey, err := encryptWithRSAOAEP256(cek, recipientPubKey)
		return encryptedKey, nil, err

	case A128KW, A192KW, A256KW:
		encryptedKey, err := encryptWithAESKW(cek, recipientPubKey, alg)
		return encryptedKey, nil, err

	case ECDHES:
		// For ECDH-ES, the CEK is directly derived from the shared secret
		return encryptWithECDHES(cek, recipientPubKey, enc)

	case ECDHESA128KW, ECDHESA192KW, ECDHESA256KW:
		// Derive KEK using ECDH-ES, then wrap CEK
		return encryptWithECDHESKW(cek, recipientPubKey, alg)

	case A128GCMKW, A192GCMKW, A256GCMKW:
		return encryptWithAESGCMKW(cek, recipientPubKey, alg)

	default:
		return nil, nil, fmt.Errorf("unsupported JWE algorithm: %s", alg)
	}
}

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

// decryptKey decrypts the encrypted content encryption key (CEK) using the recipient's private key.
func decryptKey(encryptedKey []byte, alg KeyEncAlgorithm, privateKey crypto.PrivateKey,
	header map[string]interface{}, enc ContentEncAlgorithm) ([]byte, error) {
	switch alg {
	case RSAOAEP:
		return decryptWithRSAOAEP(encryptedKey, privateKey)

	case RSAOAEP256:
		return decryptWithRSAOAEP256(encryptedKey, privateKey)

	case A128KW, A192KW, A256KW:
		return decryptWithAESKW(encryptedKey, privateKey, alg)

	case ECDHES:
		return decryptWithECDHES(privateKey, header, enc)

	case ECDHESA128KW, ECDHESA192KW, ECDHESA256KW:
		return decryptWithECDHESKW(encryptedKey, privateKey, header, alg)

	case A128GCMKW, A192GCMKW, A256GCMKW:
		return decryptWithAESGCMKW(encryptedKey, privateKey, header, alg)

	default:
		return nil, fmt.Errorf("unsupported JWE algorithm: %s", alg)
	}
}

// computeSharedSecretForRecipient computes the ECDH shared secret Z from the recipient's private key.
func computeSharedSecretForRecipient(priv crypto.PrivateKey, pub crypto.PublicKey) ([]byte, error) {
	ecdsaPriv, ok := priv.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not an ECDSA key")
	}
	ecdhPriv, err := ecdsaPriv.ECDH()
	if err != nil {
		return nil, err
	}
	return computeSharedSecret(ecdhPriv, pub)
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

// concatKDF implements the Concat KDF function (RFC 7518 Section 4.6.2).
func concatKDF(z []byte, algID string, keyLen int) []byte {
	hasher, _ := cryptohash.GetHash(cryptohash.GenericSHA256)
	key := make([]byte, 0, keyLen)

	// SuppPubInfo is the key length in bits
	suppPubInfo := make([]byte, 4)
	binary.BigEndian.PutUint32(suppPubInfo, uint32(uint64(keyLen)*8)) // nolint:gosec // G115

	// OtherInfo = AlgorithmID || PartyUInfo || PartyVInfo || SuppPubInfo || SuppPrivInfo
	// For simplicity, we assume empty PartyUInfo, PartyVInfo and SuppPrivInfo as often done in JOSER
	algorithmID := lengthPrefixed([]byte(algID))
	partyUInfo := lengthPrefixed(nil)
	partyVInfo := lengthPrefixed(nil)
	suppPrivInfo := lengthPrefixed(nil)

	otherInfo := append(algorithmID, partyUInfo...) // nolint:gocritic
	otherInfo = append(otherInfo, partyVInfo...)
	otherInfo = append(otherInfo, suppPubInfo...)
	otherInfo = append(otherInfo, suppPrivInfo...)

	for counter := uint32(1); len(key) < keyLen; counter++ {
		hasher.Reset()
		counterBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(counterBuf, counter)

		hasher.Write(counterBuf)
		hasher.Write(z)
		hasher.Write(otherInfo)

		key = append(key, hasher.Sum(nil)...)
	}

	return key[:keyLen]
}

// lengthPrefixed returns the input data prefixed with its length as a 4-byte big-endian integer.
func lengthPrefixed(data []byte) []byte {
	res := make([]byte, 4+len(data))
	binary.BigEndian.PutUint32(res, uint32(uint64(len(data)))) // nolint:gosec // G115
	copy(res[4:], data)
	return res
}

// aesKeyWrap wraps the content encryption key (CEK) with the key encryption key (KEK).
// Implements RFC 3394 AES Key Wrap algorithm.
func aesKeyWrap(kek, cek []byte) ([]byte, error) {
	if len(cek)%8 != 0 {
		return nil, errors.New("CEK length must be a multiple of 8")
	}

	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, err
	}

	n := len(cek) / 8
	r := make([]byte, (n+1)*8)
	copy(r[8:], cek)

	// Default IV for AES Key Wrap
	copy(r[:8], []byte{0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6})

	for j := 0; j <= 5; j++ {
		for i := 1; i <= n; i++ {
			b := make([]byte, 16)
			copy(b[:8], r[:8])
			copy(b[8:], r[i*8:i*8+8])

			block.Encrypt(b, b)

			t := uint64(j)*uint64(n) + uint64(i) // nolint:gosec // G115
			for k := 0; k < 8; k++ {
				b[7-k] ^= byte(t >> (8 * k))
			}

			copy(r[:8], b[:8])
			copy(r[i*8:i*8+8], b[8:])
		}
	}

	return r, nil
}

// aesKeyUnwrap unwraps the wrapped key using the key encryption key (KEK).
// Implements RFC 3394 AES Key Wrap algorithm.
func aesKeyUnwrap(kek, wrapped []byte) ([]byte, error) {
	if len(wrapped)%8 != 0 || len(wrapped) < 16 {
		return nil, errors.New("invalid wrapped key length")
	}

	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, err
	}

	n := (len(wrapped) / 8) - 1
	r := make([]byte, (n+1)*8)
	copy(r, wrapped)

	for j := 5; j >= 0; j-- {
		for i := n; i >= 1; i-- {
			t := uint64(j)*uint64(n) + uint64(i) // nolint:gosec // G115
			b := make([]byte, 16)
			copy(b[:8], r[:8])
			for k := 0; k < 8; k++ {
				b[7-k] ^= byte(t >> (8 * k))
			}
			copy(b[8:], r[i*8:i*8+8])

			block.Decrypt(b, b)

			copy(r[:8], b[:8])
			copy(r[i*8:i*8+8], b[8:])
		}
	}

	// Verify IV
	for i := 0; i < 8; i++ {
		if r[i] != 0xA6 {
			return nil, errors.New("IV mismatch during AES Key Unwrap")
		}
	}

	return r[8:], nil
}

// generateEphemeralKey generates an ephemeral EC key pair for the given public key's curve.
func generateEphemeralKey(recipientPubKey crypto.PublicKey) (crypto.PrivateKey, crypto.PublicKey, error) {
	ecdsaPub, ok := recipientPubKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, nil, errors.New("recipient public key is not an ECDSA key")
	}

	var curve ecdh.Curve
	switch ecdsaPub.Curve.Params().Name {
	case jws.P256:
		curve = ecdh.P256()
	case jws.P384:
		curve = ecdh.P384()
	case jws.P521:
		curve = ecdh.P521()
	default:
		return nil, nil, fmt.Errorf("unsupported curve: %s", ecdsaPub.Curve.Params().Name)
	}

	priv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	pub := priv.PublicKey()
	return priv, pub, nil
}

// computeSharedSecret computes the ECDH shared secret Z.
func computeSharedSecret(privKey crypto.PrivateKey, pubKey crypto.PublicKey) ([]byte, error) {
	ecdhPriv, ok := privKey.(*ecdh.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not an ECDH private key")
	}

	var ecdhPub *ecdh.PublicKey
	switch p := pubKey.(type) {
	case *ecdh.PublicKey:
		ecdhPub = p
	case *ecdsa.PublicKey:
		var err error
		ecdhPub, err = p.ECDH()
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unsupported public key type for ECDH")
	}

	return ecdhPriv.ECDH(ecdhPub)
}

// encryptWithRSAOAEP encrypts the CEK using RSA-OAEP with SHA-1 (RFC 7518 §4.3).
func encryptWithRSAOAEP(cek []byte, recipientPubKey crypto.PublicKey) ([]byte, error) {
	rsaPub, ok := recipientPubKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("unsupported public key type for JWE key encryption")
	}
	return rsa.EncryptOAEP(sha1.New(), rand.Reader, rsaPub, cek, nil) //nolint:gosec
}

// encryptWithRSAOAEP256 encrypts the CEK using RSA-OAEP-256 algorithm.
func encryptWithRSAOAEP256(cek []byte, recipientPubKey crypto.PublicKey) ([]byte, error) {
	rsaPub, ok := recipientPubKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("unsupported public key type for JWE key encryption")
	}
	h, err := cryptohash.GetHash(cryptohash.GenericSHA256)
	if err != nil {
		return nil, err
	}
	return rsa.EncryptOAEP(h, rand.Reader, rsaPub, cek, nil)
}

// decryptWithRSAOAEP decrypts the encrypted key using RSA-OAEP with SHA-1 (RFC 7518 §4.3).
func decryptWithRSAOAEP(encryptedKey []byte, privateKey crypto.PrivateKey) ([]byte, error) {
	rsaPriv, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("unsupported private key type for JWE key decryption")
	}
	return rsa.DecryptOAEP(sha1.New(), rand.Reader, rsaPriv, encryptedKey, nil) //nolint:gosec
}

// encryptWithECDHES derives the CEK using ECDH-ES algorithm.
func encryptWithECDHES(cek []byte, recipientPubKey crypto.PublicKey,
	enc ContentEncAlgorithm) ([]byte, map[string]interface{}, error) {
	ephemeralPriv, ephemeralPub, err := generateEphemeralKey(recipientPubKey)
	if err != nil {
		return nil, nil, err
	}

	z, err := computeSharedSecret(ephemeralPriv, recipientPubKey)
	if err != nil {
		return nil, nil, err
	}

	// Derive key length based on enc
	keyLen := 0
	switch enc {
	case A128GCM:
		keyLen = 16
	case A192GCM:
		keyLen = 24
	case A256GCM:
		keyLen = 32
	default:
		return nil, nil, fmt.Errorf("unsupported encryption algorithm for ECDH-ES: %s", enc)
	}

	derivedKey := concatKDF(z, string(enc), keyLen)
	copy(cek, derivedKey)

	// Set epk in header
	epkMap, err := epkToMap(ephemeralPub)
	if err != nil {
		return nil, nil, err
	}
	headerExtras := map[string]interface{}{"epk": epkMap}
	return []byte{}, headerExtras, nil
}

// encryptWithECDHESKW encrypts the CEK using ECDH-ES with AES key wrap.
func encryptWithECDHESKW(cek []byte, recipientPubKey crypto.PublicKey,
	alg KeyEncAlgorithm) ([]byte, map[string]interface{}, error) {
	ephemeralPriv, ephemeralPub, err := generateEphemeralKey(recipientPubKey)
	if err != nil {
		return nil, nil, err
	}

	z, err := computeSharedSecret(ephemeralPriv, recipientPubKey)
	if err != nil {
		return nil, nil, err
	}

	kekLen := 16
	switch alg {
	case ECDHESA192KW:
		kekLen = 24
	case ECDHESA256KW:
		kekLen = 32
	}

	kek := concatKDF(z, string(alg), kekLen)
	wrappedKey, err := aesKeyWrap(kek, cek)
	if err != nil {
		return nil, nil, err
	}

	epkMap, err := epkToMap(ephemeralPub)
	if err != nil {
		return nil, nil, err
	}
	headerExtras := map[string]interface{}{"epk": epkMap}
	return wrappedKey, headerExtras, nil
}

// decryptWithRSAOAEP256 decrypts the encrypted key using RSA-OAEP-256 algorithm.
func decryptWithRSAOAEP256(encryptedKey []byte, privateKey crypto.PrivateKey) ([]byte, error) {
	rsaPriv, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("unsupported private key type for JWE key decryption")
	}
	h, err := cryptohash.GetHash(cryptohash.GenericSHA256)
	if err != nil {
		return nil, err
	}
	return rsa.DecryptOAEP(h, rand.Reader, rsaPriv, encryptedKey, nil)
}

// decryptWithECDHES derives the CEK using ECDH-ES algorithm.
func decryptWithECDHES(privateKey crypto.PrivateKey, header map[string]interface{},
	enc ContentEncAlgorithm) ([]byte, error) {
	epkMap, ok := header["epk"].(map[string]interface{})
	if !ok {
		return nil, errors.New("missing epk in header")
	}
	ephemeralPub, err := jws.JWKToECPublicKey(epkMap)
	if err != nil {
		return nil, err
	}

	z, err := computeSharedSecretForRecipient(privateKey, ephemeralPub)
	if err != nil {
		return nil, err
	}

	keyLen := 0
	switch enc {
	case A128GCM:
		keyLen = 16
	case A192GCM:
		keyLen = 24
	case A256GCM:
		keyLen = 32
	default:
		return nil, fmt.Errorf("unsupported encryption algorithm for ECDH-ES: %s", enc)
	}

	return concatKDF(z, string(enc), keyLen), nil
}

// decryptWithECDHESKW decrypts the wrapped key using ECDH-ES with AES key wrap.
func decryptWithECDHESKW(encryptedKey []byte, privateKey crypto.PrivateKey,
	header map[string]interface{}, alg KeyEncAlgorithm) ([]byte, error) {
	epkMap, ok := header["epk"].(map[string]interface{})
	if !ok {
		return nil, errors.New("missing epk in header")
	}
	ephemeralPub, err := jws.JWKToECPublicKey(epkMap)
	if err != nil {
		return nil, err
	}

	z, err := computeSharedSecretForRecipient(privateKey, ephemeralPub)
	if err != nil {
		return nil, err
	}

	kekLen := 16
	switch alg {
	case ECDHESA192KW:
		kekLen = 24
	case ECDHESA256KW:
		kekLen = 32
	}

	kek := concatKDF(z, string(alg), kekLen)
	return aesKeyUnwrap(kek, encryptedKey)
}

// encryptWithAESKW wraps the CEK using the symmetric KEK via RFC 3394 AES Key Wrap (RFC 7518 §4.4).
// The KEK is passed as a []byte stored in crypto.PublicKey.
func encryptWithAESKW(cek []byte, kek crypto.PublicKey, alg KeyEncAlgorithm) ([]byte, error) {
	keyBytes, ok := kek.([]byte)
	if !ok {
		return nil, fmt.Errorf("AES Key Wrap requires a symmetric key ([]byte), got %T", kek)
	}
	expectedLen := 16
	switch alg {
	case A192KW:
		expectedLen = 24
	case A256KW:
		expectedLen = 32
	}
	if len(keyBytes) != expectedLen {
		return nil, fmt.Errorf("AES Key Wrap (%s) requires a %d-byte key, got %d", alg, expectedLen, len(keyBytes))
	}
	return aesKeyWrap(keyBytes, cek)
}

// decryptWithAESKW unwraps the encrypted CEK using the symmetric KEK via RFC 3394 AES Key Unwrap (RFC 7518 §4.4).
// The KEK is passed as a []byte stored in crypto.PrivateKey.
func decryptWithAESKW(encryptedKey []byte, kek crypto.PrivateKey, alg KeyEncAlgorithm) ([]byte, error) {
	keyBytes, ok := kek.([]byte)
	if !ok {
		return nil, fmt.Errorf("AES Key Wrap requires a symmetric key ([]byte), got %T", kek)
	}
	expectedLen := 16
	switch alg {
	case A192KW:
		expectedLen = 24
	case A256KW:
		expectedLen = 32
	}
	if len(keyBytes) != expectedLen {
		return nil, fmt.Errorf("AES Key Wrap (%s) requires a %d-byte key, got %d", alg, expectedLen, len(keyBytes))
	}
	return aesKeyUnwrap(keyBytes, encryptedKey)
}

// encryptWithAESGCMKW encrypts the CEK using AES-GCM key wrap (RFC 7518 §4.7).
// The KEK is passed as a []byte stored in crypto.PublicKey.
// The generated IV and authentication tag are returned as headerExtras ("iv" and "tag").
func encryptWithAESGCMKW(
	cek []byte, kek crypto.PublicKey, alg KeyEncAlgorithm,
) ([]byte, map[string]interface{}, error) {
	keyBytes, ok := kek.([]byte)
	if !ok {
		return nil, nil, fmt.Errorf("AES-GCM Key Wrap requires a symmetric key ([]byte), got %T", kek)
	}
	expectedLen := 16
	switch alg {
	case A192GCMKW:
		expectedLen = 24
	case A256GCMKW:
		expectedLen = 32
	}
	if len(keyBytes) != expectedLen {
		return nil, nil, fmt.Errorf(
			"AES-GCM Key Wrap (%s) requires a %d-byte key, got %d", alg, expectedLen, len(keyBytes),
		)
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	iv := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(iv); err != nil {
		return nil, nil, err
	}

	sealed := gcm.Seal(nil, iv, cek, nil)
	tagSize := gcm.Overhead()
	encryptedKey := sealed[:len(sealed)-tagSize]
	tag := sealed[len(sealed)-tagSize:]

	headerExtras := map[string]interface{}{
		"iv":  base64.RawURLEncoding.EncodeToString(iv),
		"tag": base64.RawURLEncoding.EncodeToString(tag),
	}
	return encryptedKey, headerExtras, nil
}

// decryptWithAESGCMKW decrypts the wrapped CEK using AES-GCM key wrap (RFC 7518 §4.7).
// The KEK is passed as a []byte stored in crypto.PrivateKey.
// The IV and tag are read from the JWE protected header.
func decryptWithAESGCMKW(encryptedKey []byte, kek crypto.PrivateKey,
	header map[string]interface{}, alg KeyEncAlgorithm) ([]byte, error) {
	keyBytes, ok := kek.([]byte)
	if !ok {
		return nil, fmt.Errorf("AES-GCM Key Wrap requires a symmetric key ([]byte), got %T", kek)
	}
	expectedLen := 16
	switch alg {
	case A192GCMKW:
		expectedLen = 24
	case A256GCMKW:
		expectedLen = 32
	}
	if len(keyBytes) != expectedLen {
		return nil, fmt.Errorf("AES-GCM Key Wrap (%s) requires a %d-byte key, got %d", alg, expectedLen, len(keyBytes))
	}

	ivStr, ok := header["iv"].(string)
	if !ok {
		return nil, errors.New("missing iv in header for AES-GCM Key Wrap")
	}
	tagStr, ok := header["tag"].(string)
	if !ok {
		return nil, errors.New("missing tag in header for AES-GCM Key Wrap")
	}

	iv, err := base64.RawURLEncoding.DecodeString(ivStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode iv from header: %w", err)
	}
	tag, err := base64.RawURLEncoding.DecodeString(tagStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode tag from header: %w", err)
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(iv) != gcm.NonceSize() {
		return nil, fmt.Errorf("invalid GCM nonce length for key wrap: got %d, want %d", len(iv), gcm.NonceSize())
	}

	return gcm.Open(nil, iv, append(encryptedKey, tag...), nil)
}
