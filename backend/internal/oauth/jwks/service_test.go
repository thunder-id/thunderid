/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package jwks

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cryptolab"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
)

type JWKSServiceTestSuite struct {
	suite.Suite
	jwksService JWKSServiceInterface
	cryptoMock  *cryptomock.RuntimeCryptoProviderMock
}

func TestJWKSServiceSuite(t *testing.T) {
	suite.Run(t, new(JWKSServiceTestSuite))
}

func (suite *JWKSServiceTestSuite) SetupTest() {
	suite.cryptoMock = cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	suite.jwksService = newJWKSService(suite.cryptoMock)
}

func (suite *JWKSServiceTestSuite) TestGetJWKS_RSA_Success() {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	info := kmprovider.PublicKeyInfo{
		KeyID:          "kid-1",
		Algorithm:      cryptolab.AlgorithmRS256,
		PublicKey:      &key.PublicKey,
		Thumbprint:     "kid-1",
		CertificateDER: []byte("rsa-cert-raw"),
	}
	suite.cryptoMock.EXPECT().GetPublicKeys(mock.Anything, kmprovider.PublicKeyFilter{}).
		Return([]kmprovider.PublicKeyInfo{info}, nil)

	resp, svcErr := suite.jwksService.GetJWKS()
	assert.Nil(suite.T(), svcErr)
	assert.NotNil(suite.T(), resp)
	assert.Len(suite.T(), resp.Keys, 1)
	k := resp.Keys[0]
	assert.Equal(suite.T(), "RSA", k.Kty)
	assert.Equal(suite.T(), "RS256", k.Alg)
	assert.NotEmpty(suite.T(), k.N)
	assert.NotEmpty(suite.T(), k.E)
	assert.NotEmpty(suite.T(), k.X5c)
	assert.NotEmpty(suite.T(), k.X5t)
	assert.NotEmpty(suite.T(), k.X5tS256)
}

func (suite *JWKSServiceTestSuite) TestGetJWKS_ECDSA_P256_Success() {
	ecdsaKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	info := kmprovider.PublicKeyInfo{
		KeyID:          "kid-1",
		Algorithm:      cryptolab.AlgorithmES256,
		PublicKey:      &ecdsaKey.PublicKey,
		Thumbprint:     "kid-1",
		CertificateDER: []byte("ec-cert-raw"),
	}
	suite.cryptoMock.EXPECT().GetPublicKeys(mock.Anything, kmprovider.PublicKeyFilter{}).
		Return([]kmprovider.PublicKeyInfo{info}, nil)

	resp, svcErr := suite.jwksService.GetJWKS()
	assert.Nil(suite.T(), svcErr)
	assert.NotNil(suite.T(), resp)
	assert.Len(suite.T(), resp.Keys, 1)
	k := resp.Keys[0]
	assert.Equal(suite.T(), "EC", k.Kty)
	assert.Equal(suite.T(), "ES256", k.Alg)
	assert.Equal(suite.T(), "P-256", k.Crv)
	assert.NotEmpty(suite.T(), k.X)
	assert.NotEmpty(suite.T(), k.Y)
	assert.NotEmpty(suite.T(), k.X5c)
	assert.NotEmpty(suite.T(), k.X5t)
	assert.NotEmpty(suite.T(), k.X5tS256)
}

func (suite *JWKSServiceTestSuite) TestGetJWKS_EdDSA_Success() {
	_, edPriv, _ := ed25519.GenerateKey(rand.Reader)
	info := kmprovider.PublicKeyInfo{
		KeyID:          "kid-1",
		Algorithm:      cryptolab.AlgorithmEdDSA,
		PublicKey:      edPriv.Public(),
		Thumbprint:     "kid-1",
		CertificateDER: []byte("ed-cert-raw"),
	}
	suite.cryptoMock.EXPECT().GetPublicKeys(mock.Anything, kmprovider.PublicKeyFilter{}).
		Return([]kmprovider.PublicKeyInfo{info}, nil)

	resp, svcErr := suite.jwksService.GetJWKS()
	assert.Nil(suite.T(), svcErr)
	assert.NotNil(suite.T(), resp)
	assert.Len(suite.T(), resp.Keys, 1)
	k := resp.Keys[0]
	assert.Equal(suite.T(), "OKP", k.Kty)
	assert.Equal(suite.T(), "EdDSA", k.Alg)
	assert.Equal(suite.T(), "Ed25519", k.Crv)
	assert.NotEmpty(suite.T(), k.X)
	assert.NotEmpty(suite.T(), k.X5c)
	assert.NotEmpty(suite.T(), k.X5t)
	assert.NotEmpty(suite.T(), k.X5tS256)
}

func (suite *JWKSServiceTestSuite) TestGetJWKS_GetPublicKeysError() {
	suite.cryptoMock.EXPECT().GetPublicKeys(mock.Anything, kmprovider.PublicKeyFilter{}).
		Return(nil, errors.New("provider error"))

	resp, svcErr := suite.jwksService.GetJWKS()
	assert.Nil(suite.T(), resp)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, svcErr.Code)
}

func (suite *JWKSServiceTestSuite) TestGetJWKS_NoCertificatesFound() {
	suite.cryptoMock.EXPECT().GetPublicKeys(mock.Anything, kmprovider.PublicKeyFilter{}).
		Return([]kmprovider.PublicKeyInfo{}, nil)

	resp, svcErr := suite.jwksService.GetJWKS()
	assert.Nil(suite.T(), resp)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, svcErr.Code)
}

func (suite *JWKSServiceTestSuite) TestGetJWKS_UnsupportedPublicKeyType() {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	keys := []kmprovider.PublicKeyInfo{
		{
			KeyID:      "kid-1",
			PublicKey:  "unsupported-key-type",
			Thumbprint: "kid-1",
		},
		{
			KeyID:          "kid-2",
			Algorithm:      cryptolab.AlgorithmRS256,
			PublicKey:      &rsaKey.PublicKey,
			Thumbprint:     "kid-2",
			CertificateDER: []byte("rsa-cert-raw"),
		},
	}
	suite.cryptoMock.EXPECT().GetPublicKeys(mock.Anything, kmprovider.PublicKeyFilter{}).
		Return(keys, nil)

	resp, svcErr := suite.jwksService.GetJWKS()
	assert.Nil(suite.T(), svcErr)
	assert.NotNil(suite.T(), resp)
	assert.Len(suite.T(), resp.Keys, 1)
}

func (suite *JWKSServiceTestSuite) TestGetJWKS_OnlyUnsupportedKeys() {
	keys := []kmprovider.PublicKeyInfo{
		{KeyID: "kid-1", PublicKey: "unsupported-key-type", Thumbprint: "kid-1"},
	}
	suite.cryptoMock.EXPECT().GetPublicKeys(mock.Anything, kmprovider.PublicKeyFilter{}).
		Return(keys, nil)

	resp, svcErr := suite.jwksService.GetJWKS()
	assert.Nil(suite.T(), resp)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, svcErr.Code)
}

func (suite *JWKSServiceTestSuite) TestGetJWKS_MultipleCertificates() {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	ecdsaKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	keys := []kmprovider.PublicKeyInfo{
		{
			KeyID:          "rsa-kid",
			Algorithm:      cryptolab.AlgorithmRS256,
			PublicKey:      &rsaKey.PublicKey,
			Thumbprint:     "rsa-kid",
			CertificateDER: []byte("rsa-cert-raw"),
		},
		{
			KeyID:          "ec-kid",
			Algorithm:      cryptolab.AlgorithmES256,
			PublicKey:      &ecdsaKey.PublicKey,
			Thumbprint:     "ec-kid",
			CertificateDER: []byte("ec-cert-raw"),
		},
	}
	suite.cryptoMock.EXPECT().GetPublicKeys(mock.Anything, kmprovider.PublicKeyFilter{}).
		Return(keys, nil)

	resp, svcErr := suite.jwksService.GetJWKS()
	assert.Nil(suite.T(), svcErr)
	assert.NotNil(suite.T(), resp)
	assert.Len(suite.T(), resp.Keys, 2)

	rsaFound := false
	ecFound := false
	for _, k := range resp.Keys {
		if k.Kty == "RSA" {
			rsaFound = true
			assert.Equal(suite.T(), "RS256", k.Alg)
		}
		if k.Kty == "EC" {
			ecFound = true
			assert.Equal(suite.T(), "ES256", k.Alg)
		}
	}
	assert.True(suite.T(), rsaFound, "RSA key not found in JWKS")
	assert.True(suite.T(), ecFound, "EC key not found in JWKS")
}

func (suite *JWKSServiceTestSuite) TestGetJWKS_ECDSA_AdditionalCurves() {
	tests := []struct {
		name  string
		curve elliptic.Curve
		alg   cryptolab.Algorithm
		crv   string
	}{
		{name: "P-384", curve: elliptic.P384(), alg: cryptolab.AlgorithmES384, crv: "P-384"},
		{name: "P-521", curve: elliptic.P521(), alg: cryptolab.AlgorithmES512, crv: "P-521"},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			ecdsaKey, _ := ecdsa.GenerateKey(tt.curve, rand.Reader)
			info := kmprovider.PublicKeyInfo{
				KeyID:          "kid-" + tt.name,
				Algorithm:      tt.alg,
				PublicKey:      &ecdsaKey.PublicKey,
				Thumbprint:     "kid-" + tt.name,
				CertificateDER: []byte("ec-cert-raw-" + tt.name),
			}
			suite.cryptoMock.EXPECT().GetPublicKeys(mock.Anything, kmprovider.PublicKeyFilter{}).
				Return([]kmprovider.PublicKeyInfo{info}, nil).Once()

			resp, svcErr := suite.jwksService.GetJWKS()
			assert.Nil(suite.T(), svcErr)
			assert.NotNil(suite.T(), resp)
			assert.Len(suite.T(), resp.Keys, 1)
			k := resp.Keys[0]
			assert.Equal(suite.T(), "EC", k.Kty)
			assert.Equal(suite.T(), string(tt.alg), k.Alg)
			assert.Equal(suite.T(), tt.crv, k.Crv)
			assert.NotEmpty(suite.T(), k.X)
			assert.NotEmpty(suite.T(), k.Y)
		})
	}
}

func (suite *JWKSServiceTestSuite) TestGetJWKS_RSA_ZeroExponent() {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	key.PublicKey.E = 0
	info := kmprovider.PublicKeyInfo{
		KeyID:          "kid-zero",
		Algorithm:      cryptolab.AlgorithmRS256,
		PublicKey:      &key.PublicKey,
		Thumbprint:     "kid-zero",
		CertificateDER: []byte("rsa-cert-raw-zero"),
	}
	suite.cryptoMock.EXPECT().GetPublicKeys(mock.Anything, kmprovider.PublicKeyFilter{}).
		Return([]kmprovider.PublicKeyInfo{info}, nil)

	resp, svcErr := suite.jwksService.GetJWKS()
	assert.Nil(suite.T(), svcErr)
	assert.NotNil(suite.T(), resp)
	assert.Len(suite.T(), resp.Keys, 1)
	k := resp.Keys[0]
	assert.Equal(suite.T(), "RSA", k.Kty)
	assert.Equal(suite.T(), "AA", k.E)
}

func (suite *JWKSServiceTestSuite) TestGetJWKS_NoCertificateDER() {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	info := kmprovider.PublicKeyInfo{
		KeyID:      "kid-1",
		Algorithm:  cryptolab.AlgorithmRS256,
		PublicKey:  &rsaKey.PublicKey,
		Thumbprint: "kid-1",
		// CertificateDER intentionally nil
	}
	suite.cryptoMock.EXPECT().GetPublicKeys(mock.Anything, kmprovider.PublicKeyFilter{}).
		Return([]kmprovider.PublicKeyInfo{info}, nil)

	resp, svcErr := suite.jwksService.GetJWKS()
	assert.Nil(suite.T(), svcErr)
	assert.NotNil(suite.T(), resp)
	assert.Len(suite.T(), resp.Keys, 1)
	k := resp.Keys[0]
	assert.Equal(suite.T(), "RSA", k.Kty)
	assert.Empty(suite.T(), k.X5c)
	assert.Empty(suite.T(), k.X5t)
	assert.Empty(suite.T(), k.X5tS256)
}
