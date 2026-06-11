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
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"hash"
	"math/big"
)

// Sign errors.
var (
	ErrUnsupportedAlgorithm = errors.New("unsupported signature algorithm")
	ErrInvalidPrivateKey    = errors.New("invalid private key type for algorithm")
	ErrInvalidPublicKey     = errors.New("invalid public key type for algorithm")
	ErrInvalidSignature     = errors.New("signature verification failed")
)

// Generate hashes data according to alg and returns the digital signature using privateKey.
func Generate(data []byte, alg SignAlgorithm, privateKey crypto.PrivateKey) ([]byte, error) {
	hashed, hashFunc := hashData(data, alg)

	switch alg {
	case RSASHA256, RSASHA512:
		return newRSASign(hashed, hashFunc, privateKey)
	case RSAPSSSHA256:
		return newRSAPSSSign(hashed, hashFunc, privateKey)
	case ECDSASHA256, ECDSASHA384, ECDSASHA512:
		return newECDSASign(hashed, privateKey)
	case ED25519:
		return newED25519Sign(data, privateKey)
	default:
		return nil, ErrUnsupportedAlgorithm
	}
}

// Verify hashes data according to alg and verifies the signature using publicKey.
func Verify(data []byte, signature []byte, alg SignAlgorithm, publicKey crypto.PublicKey) error {
	hashed, hashFunc := hashData(data, alg)

	switch alg {
	case RSASHA256, RSASHA512:
		return verifyRSA(hashed, signature, hashFunc, publicKey)
	case RSAPSSSHA256:
		return verifyRSAPSS(hashed, signature, hashFunc, publicKey)
	case ECDSASHA256, ECDSASHA384, ECDSASHA512:
		return verifyECDSA(hashed, signature, publicKey)
	case ED25519:
		return verifyED25519(data, signature, publicKey)
	default:
		return ErrUnsupportedAlgorithm
	}
}

// hashData hashes data using the hash function implied by alg.
// For ED25519, no pre-hashing is performed and the original data is returned.
func hashData(data []byte, alg SignAlgorithm) ([]byte, crypto.Hash) {
	var h hash.Hash
	var hashFunc crypto.Hash

	switch alg {
	case RSASHA256, RSAPSSSHA256, ECDSASHA256:
		h = sha256.New()
		hashFunc = crypto.SHA256
	case RSASHA512, ECDSASHA512:
		h = sha512.New()
		hashFunc = crypto.SHA512
	case ECDSASHA384:
		h = sha512.New384()
		hashFunc = crypto.SHA384
	case ED25519:
		return data, crypto.Hash(0)
	default:
		return nil, crypto.Hash(0)
	}

	h.Write(data)
	return h.Sum(nil), hashFunc
}

func newRSASign(hashed []byte, hashFunc crypto.Hash, privateKey crypto.PrivateKey) ([]byte, error) {
	rsaKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, ErrInvalidPrivateKey
	}
	return rsa.SignPKCS1v15(rand.Reader, rsaKey, hashFunc, hashed)
}

func verifyRSA(hashed, signature []byte, hashFunc crypto.Hash, publicKey crypto.PublicKey) error {
	rsaPub, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		return ErrInvalidPublicKey
	}
	if err := rsa.VerifyPKCS1v15(rsaPub, hashFunc, hashed, signature); err != nil {
		return ErrInvalidSignature
	}
	return nil
}

// newRSAPSSSign creates an RSA-PSS signature.
// Salt length equals the hash output size as required by RFC 7518 Section 3.5.
func newRSAPSSSign(hashed []byte, hashFunc crypto.Hash, privateKey crypto.PrivateKey) ([]byte, error) {
	rsaKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, ErrInvalidPrivateKey
	}
	opts := &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: hashFunc}
	return rsa.SignPSS(rand.Reader, rsaKey, hashFunc, hashed, opts)
}

// verifyRSAPSS verifies an RSA-PSS signature.
// Salt length equals the hash output size as required by RFC 7518 Section 3.5.
func verifyRSAPSS(hashed, signature []byte, hashFunc crypto.Hash, publicKey crypto.PublicKey) error {
	rsaPub, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		return ErrInvalidPublicKey
	}
	opts := &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: hashFunc}
	if err := rsa.VerifyPSS(rsaPub, hashFunc, hashed, signature, opts); err != nil {
		return ErrInvalidSignature
	}
	return nil
}

func newECDSASign(hashed []byte, privateKey crypto.PrivateKey) ([]byte, error) {
	ecdsaKey, ok := privateKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, ErrInvalidPrivateKey
	}
	r, s, err := ecdsa.Sign(rand.Reader, ecdsaKey, hashed)
	if err != nil {
		return nil, err
	}
	// RFC 7518 §3.4: encode as fixed-size R || S (each zero-padded to curve coordinate size).
	coordSize := (ecdsaKey.Curve.Params().BitSize + 7) / 8
	sig := make([]byte, 2*coordSize)
	r.FillBytes(sig[:coordSize])
	s.FillBytes(sig[coordSize:])
	return sig, nil
}

func verifyECDSA(hashed, signature []byte, publicKey crypto.PublicKey) error {
	ecdsaPub, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return ErrInvalidPublicKey
	}
	// RFC 7518 §3.4: signature is fixed-size R || S (each zero-padded to curve coordinate size).
	coordSize := (ecdsaPub.Curve.Params().BitSize + 7) / 8
	if len(signature) != 2*coordSize {
		return ErrInvalidSignature
	}
	r := new(big.Int).SetBytes(signature[:coordSize])
	s := new(big.Int).SetBytes(signature[coordSize:])
	if !ecdsa.Verify(ecdsaPub, hashed, r, s) {
		return ErrInvalidSignature
	}
	return nil
}

func newED25519Sign(data []byte, privateKey crypto.PrivateKey) ([]byte, error) {
	ed25519Key, ok := privateKey.(ed25519.PrivateKey)
	if !ok {
		return nil, ErrInvalidPrivateKey
	}
	return ed25519.Sign(ed25519Key, data), nil
}

func verifyED25519(data, signature []byte, publicKey crypto.PublicKey) error {
	ed25519Pub, ok := publicKey.(ed25519.PublicKey)
	if !ok {
		return ErrInvalidPublicKey
	}
	if !ed25519.Verify(ed25519Pub, data, signature) {
		return ErrInvalidSignature
	}
	return nil
}
