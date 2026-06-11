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

package openid4vp

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
)

func newSignerMock(
	t *testing.T, key *ecdsa.PrivateKey, info kmprovider.PublicKeyInfo,
) *cryptomock.RuntimeCryptoProviderMock {
	t.Helper()
	m := cryptomock.NewRuntimeCryptoProviderMock(t)
	m.EXPECT().GetPublicKeys(mock.Anything, mock.Anything).Return([]kmprovider.PublicKeyInfo{info}, nil).Maybe()
	m.EXPECT().Sign(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(
			_ context.Context, _ kmprovider.KeyRef, _ cryptolib.SignAlgorithm, content []byte,
		) ([]byte, error) {
			return cryptolib.Generate(content, cryptolib.ECDSASHA256, key)
		}).Maybe()
	return m
}

func TestRequestSignerSignsVerifiableJAR(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	info := kmprovider.PublicKeyInfo{
		KeyID:          "vp-signing",
		Algorithm:      cryptolib.AlgorithmES256,
		PublicKey:      &key.PublicKey,
		Thumbprint:     "thumb-1",
		CertificateDER: []byte{0x30, 0x82, 0x01, 0x02, 0x03},
	}
	m := newSignerMock(t, key, info)

	signer, err := newRequestSigner(context.Background(), m, "vp-signing")
	require.NoError(t, err)

	jar, err := signer.signRequestObject(context.Background(), map[string]interface{}{
		"response_type": "vp_token",
		"client_id":     "x509_hash:abc",
	})
	require.NoError(t, err)

	parts := strings.Split(jar, ".")
	require.Len(t, parts, 3)

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	var header map[string]interface{}
	require.NoError(t, json.Unmarshal(headerJSON, &header))
	assert.Equal(t, "ES256", header["alg"])
	assert.Equal(t, requestObjectType, header["typ"])
	assert.Equal(t, "thumb-1", header["kid"])
	x5c := header["x5c"].([]interface{})
	require.Len(t, x5c, 1)
	assert.Equal(t, base64.StdEncoding.EncodeToString(info.CertificateDER), x5c[0])

	// Signature is in JWS P1363 format (r||s, 32 bytes each for P-256).
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	require.NoError(t, err)
	require.Len(t, sig, 64)
	signingInput := parts[0] + "." + parts[1]
	hashed := sha256.Sum256([]byte(signingInput))
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	assert.True(t, ecdsa.Verify(&key.PublicKey, hashed[:], r, s))
}

func TestNewRequestSignerErrors(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	t.Run("nil provider", func(t *testing.T) {
		_, err := newRequestSigner(context.Background(), nil, "k")
		assert.ErrorIs(t, err, ErrPolicy)
	})

	t.Run("no key found", func(t *testing.T) {
		m := cryptomock.NewRuntimeCryptoProviderMock(t)
		m.EXPECT().GetPublicKeys(mock.Anything, mock.Anything).Return(nil, nil)
		_, err := newRequestSigner(context.Background(), m, "missing")
		assert.ErrorIs(t, err, ErrPolicy)
	})

	t.Run("missing certificate", func(t *testing.T) {
		info := kmprovider.PublicKeyInfo{
			KeyID:     "vp-signing",
			Algorithm: cryptolib.AlgorithmES256,
			PublicKey: &key.PublicKey,
		}
		m := cryptomock.NewRuntimeCryptoProviderMock(t)
		m.EXPECT().GetPublicKeys(mock.Anything, mock.Anything).Return([]kmprovider.PublicKeyInfo{info}, nil)
		_, err := newRequestSigner(context.Background(), m, "vp-signing")
		assert.ErrorIs(t, err, ErrPolicy)
	})
}
