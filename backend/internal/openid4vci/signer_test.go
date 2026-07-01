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

package openid4vci

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
)

type SignerTestSuite struct {
	suite.Suite
}

func TestSignerTestSuite(t *testing.T) {
	suite.Run(t, new(SignerTestSuite))
}

func selfSignedCertDER(t *testing.T, key *ecdsa.PrivateKey) []byte {
	t.Helper()
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-issuer"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	return der
}

func (s *SignerTestSuite) TestNewIssuerSignerNilProvider() {
	_, err := newIssuerSigner(context.Background(), nil, "kid")
	s.ErrorIs(err, ErrPolicy)
}

func (s *SignerTestSuite) TestNewIssuerSignerLoadError() {
	provider := cryptomock.NewRuntimeCryptoProviderMock(s.T())
	provider.EXPECT().GetPublicKeys(mock.Anything, mock.Anything).Return(nil, errors.New("load failed"))
	_, err := newIssuerSigner(context.Background(), provider, "kid")
	s.Error(err)
}

func (s *SignerTestSuite) TestNewIssuerSignerNoKey() {
	provider := cryptomock.NewRuntimeCryptoProviderMock(s.T())
	provider.EXPECT().GetPublicKeys(mock.Anything, mock.Anything).Return(nil, nil)
	_, err := newIssuerSigner(context.Background(), provider, "kid")
	s.ErrorIs(err, ErrPolicy)
}

func (s *SignerTestSuite) TestNewIssuerSignerUnsupportedAlg() {
	provider := cryptomock.NewRuntimeCryptoProviderMock(s.T())
	provider.EXPECT().GetPublicKeys(mock.Anything, mock.Anything).Return([]kmprovider.PublicKeyInfo{
		{KeyID: "kid", Algorithm: "RUBBISH"},
	}, nil)
	_, err := newIssuerSigner(context.Background(), provider, "kid")
	s.ErrorIs(err, ErrPolicy)
}

func (s *SignerTestSuite) TestNewIssuerSignerNotCertBacked() {
	provider := cryptomock.NewRuntimeCryptoProviderMock(s.T())
	provider.EXPECT().GetPublicKeys(mock.Anything, mock.Anything).Return([]kmprovider.PublicKeyInfo{
		{KeyID: "kid", Algorithm: "ES256"},
	}, nil)
	_, err := newIssuerSigner(context.Background(), provider, "kid")
	s.ErrorIs(err, ErrPolicy)
}

func (s *SignerTestSuite) TestNewIssuerSignerSuccess() {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	certDER := selfSignedCertDER(s.T(), key)
	provider := cryptomock.NewRuntimeCryptoProviderMock(s.T())
	provider.EXPECT().GetPublicKeys(mock.Anything, mock.Anything).Return([]kmprovider.PublicKeyInfo{
		{KeyID: "kid", Algorithm: "ES256", Thumbprint: "tp", CertificateDER: certDER},
	}, nil)

	signer, err := newIssuerSigner(context.Background(), provider, "kid")
	s.Require().NoError(err)
	s.Equal("ES256", signer.jwsAlg)
	s.Equal("tp", signer.kid)
	s.Require().Len(signer.x5c, 1)

	header := signer.header("dc+sd-jwt")
	s.Equal("ES256", header["alg"])
	s.Equal("dc+sd-jwt", header["typ"])
	s.Equal("tp", header["kid"])
	s.NotNil(header["x5c"])
}

func (s *SignerTestSuite) TestNewIssuerSignerCertChain() {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	leaf := selfSignedCertDER(s.T(), key)
	provider := cryptomock.NewRuntimeCryptoProviderMock(s.T())
	provider.EXPECT().GetPublicKeys(mock.Anything, mock.Anything).Return([]kmprovider.PublicKeyInfo{
		{KeyID: "kid", Algorithm: "ES256", CertificateDER: leaf, CertificateChainDER: [][]byte{leaf, leaf}},
	}, nil)

	signer, err := newIssuerSigner(context.Background(), provider, "kid")
	s.Require().NoError(err)
	s.Len(signer.x5c, 2)
	// No thumbprint -> no kid header.
	_, hasKid := signer.header("typ")["kid"]
	s.False(hasKid)
}

func (s *SignerTestSuite) TestSign() {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	provider := cryptomock.NewRuntimeCryptoProviderMock(s.T())
	provider.EXPECT().Sign(mock.Anything, mock.Anything, "ES256", mock.Anything).
		RunAndReturn(func(
			_ context.Context, _ kmprovider.KeyRef, _ string, content []byte,
		) ([]byte, error) {
			digest := sha256.Sum256(content)
			return ecdsa.SignASN1(rand.Reader, key, digest[:])
		})
	signer := &issuerSigner{cryptoProvider: provider, signAlg: cryptolib.ECDSASHA256, jwsAlg: "ES256"}

	sig, err := signer.sign(context.Background(), "input")
	s.Require().NoError(err)
	s.Len(sig, 64) // ES256 P1363 r||s
}

func (s *SignerTestSuite) TestSignError() {
	provider := cryptomock.NewRuntimeCryptoProviderMock(s.T())
	provider.EXPECT().Sign(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("sign failed"))
	signer := &issuerSigner{cryptoProvider: provider, signAlg: cryptolib.ECDSASHA256}

	_, err := signer.sign(context.Background(), "input")
	s.Error(err)
}

func (s *SignerTestSuite) TestECDSADERToJWS() {
	der, err := asn1.Marshal(struct{ R, S *big.Int }{big.NewInt(1234567), big.NewInt(7654321)})
	s.Require().NoError(err)

	jws := ecdsaDERToJWS(der, cryptolib.ECDSASHA256)
	s.Len(jws, 64)

	jws = ecdsaDERToJWS(der, cryptolib.ECDSASHA384)
	s.Len(jws, 96)

	jws = ecdsaDERToJWS(der, cryptolib.ECDSASHA512)
	s.Len(jws, 132)

	// Non-DER input is returned as-is.
	raw := []byte("not-der-at-all")
	s.Equal(raw, ecdsaDERToJWS(raw, cryptolib.ECDSASHA256))

	// Unknown algorithm leaves DER untouched.
	s.Equal(der, ecdsaDERToJWS(der, cryptolib.SignAlgorithm("unknown")))
}
