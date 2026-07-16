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
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/asn1"

	"github.com/cloudflare/circl/sign/schemes"
)

var mldsaAlgorithms = []Algorithm{AlgorithmMLDSA44, AlgorithmMLDSA65, AlgorithmMLDSA87}

func (suite *SignUtilsTestSuite) TestMLDSAPKCS8SeedRoundTrip() {
	for _, alg := range mldsaAlgorithms {
		scheme := mldsaSchemeFor(alg)
		suite.Require().NotNil(scheme)

		seed := bytes.Repeat([]byte{0x2a}, scheme.SeedSize())
		der, err := marshalMLDSAPKCS8Seed(alg, seed)
		suite.Require().NoError(err)

		// The peek helper must recognize the algorithm.
		peeked, ok := MLDSAAlgFromPKCS8(der)
		suite.True(ok)
		suite.Equal(alg, peeked)

		sk, parsedAlg, err := ParseMLDSAPKCS8(der)
		suite.Require().NoError(err)
		suite.Equal(alg, parsedAlg)

		// The reconstructed key must match the key derived directly from the seed.
		_, want := scheme.DeriveKey(seed)
		suite.True(sk.Equal(want))
	}
}

func (suite *SignUtilsTestSuite) TestMarshalMLDSAPKCS8SeedInvalidSeed() {
	_, err := marshalMLDSAPKCS8Seed(AlgorithmMLDSA65, []byte("short"))
	suite.Error(err)
}

// TestParseMLDSAPKCS8BothFormat verifies the parser accepts the "both" CHOICE
// (seed + expandedKey), which is what OpenSSL 3.x emits by default.
func (suite *SignUtilsTestSuite) TestParseMLDSAPKCS8BothFormat() {
	scheme := schemes.ByName(string(AlgorithmMLDSA65))
	suite.Require().NotNil(scheme)

	seed := bytes.Repeat([]byte{0x11}, scheme.SeedSize())
	_, sk := scheme.DeriveKey(seed)
	expanded, err := sk.MarshalBinary()
	suite.Require().NoError(err)

	inner, err := asn1.Marshal(struct {
		Seed        []byte
		ExpandedKey []byte
	}{Seed: seed, ExpandedKey: expanded})
	suite.Require().NoError(err)

	var k pkcs8ML
	k.Algo.Algorithm = oidMLDSA65
	k.PrivateKey = inner
	der, err := asn1.Marshal(k)
	suite.Require().NoError(err)

	parsed, alg, err := ParseMLDSAPKCS8(der)
	suite.Require().NoError(err)
	suite.Equal(AlgorithmMLDSA65, alg)
	suite.True(parsed.Equal(sk))
}

func (suite *SignUtilsTestSuite) TestParseMLDSAPKCS8NotMLDSA() {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	suite.Require().NoError(err)
	der, err := x509.MarshalPKCS8PrivateKey(ecKey)
	suite.Require().NoError(err)

	_, _, err = ParseMLDSAPKCS8(der)
	suite.ErrorIs(err, errNotMLDSAKey)

	_, ok := MLDSAAlgFromPKCS8(der)
	suite.False(ok)
}

func (suite *SignUtilsTestSuite) TestMLDSAPublicKeyBytesRoundTrip() {
	for _, alg := range mldsaAlgorithms {
		signer, err := GenerateMLDSAKey(alg)
		suite.Require().NoError(err)

		gotAlg, ok := MLDSAAlgForPublicKey(signer.Public())
		suite.Require().True(ok)
		suite.Equal(alg, gotAlg)

		pubBytes, ok := MLDSAPublicKeyBytes(signer.Public())
		suite.Require().True(ok)

		restored, err := MLDSAPublicKeyFromBytes(alg, pubBytes)
		suite.Require().NoError(err)
		suite.True(restored.Equal(signer.Public()))
	}
}

func (suite *SignUtilsTestSuite) TestMLDSAHelpersRejectNonMLDSA() {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	suite.Require().NoError(err)

	_, ok := MLDSAAlgForPublicKey(ecKey.Public())
	suite.False(ok)
	_, ok = MLDSAPublicKeyBytes(ecKey.Public())
	suite.False(ok)
	suite.Nil(mldsaSchemeFor(AlgorithmRS256))
}

func (suite *SignUtilsTestSuite) TestMLDSAOIDForAlgUnsupported() {
	_, ok := mldsaOIDForAlg(AlgorithmRS256)
	suite.False(ok)
}

func (suite *SignUtilsTestSuite) TestMLDSAAlgFromPKCS8Malformed() {
	_, ok := MLDSAAlgFromPKCS8([]byte("not valid asn1"))
	suite.False(ok)
}

func (suite *SignUtilsTestSuite) TestParseMLDSAPKCS8Malformed() {
	_, _, err := ParseMLDSAPKCS8([]byte("not valid asn1"))
	suite.Error(err)
}

func (suite *SignUtilsTestSuite) TestGenerateMLDSAKeyUnsupportedAlgorithm() {
	_, err := GenerateMLDSAKey(AlgorithmRS256)
	suite.ErrorIs(err, ErrUnsupportedAlgorithm)
}

func (suite *SignUtilsTestSuite) TestMLDSAPublicKeyFromBytesUnsupportedAlgorithm() {
	_, err := MLDSAPublicKeyFromBytes(AlgorithmRS256, []byte("pub"))
	suite.ErrorIs(err, ErrUnsupportedAlgorithm)
}

func (suite *SignUtilsTestSuite) TestMarshalMLDSAPKCS8SeedUnsupportedAlgorithm() {
	_, err := marshalMLDSAPKCS8Seed(AlgorithmRS256, []byte("seed"))
	suite.ErrorIs(err, ErrUnsupportedAlgorithm)
}

func (suite *SignUtilsTestSuite) TestDecodeMLDSAPrivateKeyChoiceInvalidSeedLength() {
	scheme := schemes.ByName(string(AlgorithmMLDSA65))
	suite.Require().NotNil(scheme)

	inner, err := asn1.MarshalWithParams([]byte("short"), "tag:0")
	suite.Require().NoError(err)

	_, _, err = decodeMLDSAPrivateKeyChoice(inner, scheme.SeedSize())
	suite.ErrorContains(err, "invalid ML-DSA seed length")
}

func (suite *SignUtilsTestSuite) TestDecodeMLDSAPrivateKeyChoiceInvalidBothSeedLength() {
	scheme := schemes.ByName(string(AlgorithmMLDSA65))
	suite.Require().NotNil(scheme)

	inner, err := asn1.Marshal(struct {
		Seed        []byte
		ExpandedKey []byte
	}{Seed: []byte("short"), ExpandedKey: bytes.Repeat([]byte{0x01}, 10)})
	suite.Require().NoError(err)

	_, _, err = decodeMLDSAPrivateKeyChoice(inner, scheme.SeedSize())
	suite.ErrorContains(err, "invalid ML-DSA seed length")
}

func (suite *SignUtilsTestSuite) TestDecodeMLDSAPrivateKeyChoiceUnsupportedEncoding() {
	inner, err := asn1.Marshal(true)
	suite.Require().NoError(err)

	_, _, err = decodeMLDSAPrivateKeyChoice(inner, 32)
	suite.ErrorContains(err, "unsupported ML-DSA private key CHOICE encoding")
}

func (suite *SignUtilsTestSuite) TestDecodeMLDSAPrivateKeyChoiceMalformed() {
	_, _, err := decodeMLDSAPrivateKeyChoice([]byte("not valid asn1"), 32)
	suite.Error(err)
}
