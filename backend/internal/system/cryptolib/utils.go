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

import (
	"crypto/aes"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	ecCurveP256 = "P-256"
	ecCurveP384 = "P-384"
	ecCurveP521 = "P-521"
)

// ecdhGenerateEphemeralKeyPair generates a fresh ECDH ephemeral key pair on the same curve
// as the recipient's ECDSA public key.
func ecdhGenerateEphemeralKeyPair(recipientPub *ecdsa.PublicKey) (*ecdh.PrivateKey, *ecdh.PublicKey, error) {
	var curve ecdh.Curve
	switch recipientPub.Curve.Params().Name {
	case ecCurveP256:
		curve = ecdh.P256()
	case ecCurveP384:
		curve = ecdh.P384()
	case ecCurveP521:
		curve = ecdh.P521()
	default:
		return nil, nil, fmt.Errorf("unsupported EC curve: %s", recipientPub.Curve.Params().Name)
	}

	priv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	pub := priv.PublicKey()
	return priv, pub, nil
}

// ecdhComputeSharedSecret computes Z = ECDH(ephemeralPriv, recipientPub). Used during Encrypt.
func ecdhComputeSharedSecret(ephemeralPriv *ecdh.PrivateKey, recipientPub *ecdsa.PublicKey) ([]byte, error) {
	ecdhPub, err := recipientPub.ECDH()
	if err != nil {
		return nil, err
	}
	return ephemeralPriv.ECDH(ecdhPub)
}

// ecdhComputeSharedSecretForRecipient computes Z = ECDH(recipientPriv, ephemeralPub).
// Used during Decrypt; ephemeralPub must be an *ecdh.PublicKey.
func ecdhComputeSharedSecretForRecipient(
	recipientPriv *ecdsa.PrivateKey, ephemeralPub *ecdh.PublicKey,
) ([]byte, error) {
	ecdhPriv, err := recipientPriv.ECDH()
	if err != nil {
		return nil, err
	}
	return ecdhPriv.ECDH(ephemeralPub)
}

// ecdhConcatKDF implements the Concat KDF (RFC 7518 Section 4.6.2) using SHA-256.
// apu and apv are the raw (already base64url-decoded) header values; pass nil when absent.
func ecdhConcatKDF(z []byte, algID string, keyLen int, apu, apv []byte) ([]byte, error) {
	suppPubInfo := make([]byte, 4)
	binary.BigEndian.PutUint32(suppPubInfo, uint32(uint64(keyLen)*8)) // nolint:gosec // G115

	algorithmIDBytes := lengthPrefixedBytes([]byte(algID))
	partyUInfo := lengthPrefixedBytes(apu)
	partyVInfo := lengthPrefixedBytes(apv)

	// OtherInfo per RFC 7518 §4.6.2: AlgID || PartyUInfo || PartyVInfo || SuppPubInfo
	// SuppPrivInfo is empty and must not be included.
	otherInfo := append(algorithmIDBytes, partyUInfo...) // nolint:gocritic
	otherInfo = append(otherInfo, partyVInfo...)
	otherInfo = append(otherInfo, suppPubInfo...)

	key := make([]byte, 0, keyLen)
	for counter := uint32(1); len(key) < keyLen; counter++ {
		h := sha256.New()
		counterBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(counterBuf, counter)
		h.Write(counterBuf)
		h.Write(z)
		h.Write(otherInfo)
		key = append(key, h.Sum(nil)...)
	}
	return key[:keyLen], nil
}

// lengthPrefixedBytes returns data prefixed with its 4-byte big-endian length.
func lengthPrefixedBytes(data []byte) []byte {
	res := make([]byte, 4+len(data))
	binary.BigEndian.PutUint32(res, uint32(uint64(len(data)))) // nolint:gosec // G115
	copy(res[4:], data)
	return res
}

// ecdhAESKeyWrap wraps cek with kek using RFC 3394 AES Key Wrap.
func ecdhAESKeyWrap(kek, cek []byte) ([]byte, error) {
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

// ecdhAESKeyUnwrap unwraps wrapped with kek using RFC 3394 AES Key Unwrap.
func ecdhAESKeyUnwrap(kek, wrapped []byte) ([]byte, error) {
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

	for i := 0; i < 8; i++ {
		if r[i] != 0xA6 {
			return nil, errors.New("IV mismatch during AES Key Unwrap")
		}
	}
	return r[8:], nil
}

// ecdhContentEncKeyLen returns the CEK length in bytes for the given content encryption algorithm.
func ecdhContentEncKeyLen(alg Algorithm) (int, error) {
	switch alg {
	case "A128GCM":
		return 16, nil
	case "A192GCM":
		return 24, nil
	case "A256GCM":
		return 32, nil
	case "A128CBC-HS256":
		return 32, nil
	case "A192CBC-HS384":
		return 48, nil
	case "A256CBC-HS512":
		return 64, nil
	default:
		return 0, fmt.Errorf("unsupported content encryption algorithm: %s", alg)
	}
}
