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

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
)

type OpenID4VPSignerTestSuite struct {
	suite.Suite
}

func TestOpenID4VPSignerTestSuite(t *testing.T) {
	suite.Run(t, new(OpenID4VPSignerTestSuite))
}

func newSignerMock(
	t *testing.T, key *ecdsa.PrivateKey, info kmprovider.PublicKeyInfo,
) *cryptomock.RuntimeCryptoProviderMock {
	t.Helper()
	m := cryptomock.NewRuntimeCryptoProviderMock(t)
	m.EXPECT().GetPublicKeys(mock.Anything, mock.Anything).Return([]kmprovider.PublicKeyInfo{info}, nil).Maybe()
	m.EXPECT().Sign(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(
			_ context.Context, _ kmprovider.KeyRef, _ string, content []byte,
		) ([]byte, error) {
			return cryptolib.Generate(content, cryptolib.ECDSASHA256, key)
		}).Maybe()
	return m
}

func (suite *OpenID4VPSignerTestSuite) TestRequestSignerSignsVerifiableJAR() {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	suite.Require().NoError(err)
	info := kmprovider.PublicKeyInfo{
		KeyID:          "vp-signing",
		Algorithm:      cryptolib.AlgorithmES256,
		PublicKey:      &key.PublicKey,
		Thumbprint:     "thumb-1",
		CertificateDER: []byte{0x30, 0x82, 0x01, 0x02, 0x03},
	}
	m := newSignerMock(suite.T(), key, info)

	signer, err := newRequestSigner(context.Background(), m, "vp-signing")
	suite.Require().NoError(err)

	jar, err := signer.signRequestObject(context.Background(), map[string]interface{}{
		"response_type": "vp_token",
		"client_id":     "x509_hash:abc",
	})
	suite.Require().NoError(err)

	parts := strings.Split(jar, ".")
	suite.Require().Len(parts, 3)

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	suite.Require().NoError(err)
	var header map[string]interface{}
	suite.Require().NoError(json.Unmarshal(headerJSON, &header))
	suite.Equal("ES256", header["alg"])
	suite.Equal(requestObjectType, header["typ"])
	// No kid header: the wallet authenticates via x5c for the x509 client scheme.
	_, hasKid := header["kid"]
	suite.False(hasKid, "request object header must not carry a kid alongside x5c")
	x5c := header["x5c"].([]interface{})
	suite.Require().Len(x5c, 1)
	suite.Equal(base64.StdEncoding.EncodeToString(info.CertificateDER), x5c[0])

	// Signature is in JWS P1363 format (r||s, 32 bytes each for P-256).
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	suite.Require().NoError(err)
	suite.Require().Len(sig, 64)
	signingInput := parts[0] + "." + parts[1]
	hashed := sha256.Sum256([]byte(signingInput))
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	suite.True(ecdsa.Verify(&key.PublicKey, hashed[:], r, s))
}

func (suite *OpenID4VPSignerTestSuite) TestEcdsaDERToJWS() {
	digest := sha256.Sum256([]byte("signing input"))

	tests := []struct {
		name    string
		curve   elliptic.Curve
		alg     string
		wantLen int
	}{
		{"ES256", elliptic.P256(), "ES256", 64},
		{"ES384", elliptic.P384(), "ES384", 96},
		{"ES512", elliptic.P521(), "ES512", 132},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			key, err := ecdsa.GenerateKey(tt.curve, rand.Reader)
			suite.Require().NoError(err)
			der, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
			suite.Require().NoError(err)

			raw := ecdsaDERToJWS(der, tt.alg)
			suite.Len(raw, tt.wantLen)
		})
	}
}

func (suite *OpenID4VPSignerTestSuite) TestNewRequestSignerErrors() {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	suite.Require().NoError(err)

	suite.Run("nil provider", func() {
		_, err := newRequestSigner(context.Background(), nil, "k")
		suite.ErrorIs(err, ErrPolicy)
	})

	suite.Run("no key found", func() {
		m := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
		m.EXPECT().GetPublicKeys(mock.Anything, mock.Anything).Return(nil, nil)
		_, err := newRequestSigner(context.Background(), m, "missing")
		suite.ErrorIs(err, ErrPolicy)
	})

	suite.Run("missing certificate", func() {
		info := kmprovider.PublicKeyInfo{
			KeyID:     "vp-signing",
			Algorithm: cryptolib.AlgorithmES256,
			PublicKey: &key.PublicKey,
		}
		m := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
		m.EXPECT().GetPublicKeys(mock.Anything, mock.Anything).Return([]kmprovider.PublicKeyInfo{info}, nil)
		_, err := newRequestSigner(context.Background(), m, "vp-signing")
		suite.ErrorIs(err, ErrPolicy)
	})
}
