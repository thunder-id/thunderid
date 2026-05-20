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

package jwksresolver

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	certmodel "github.com/thunder-id/thunderid/internal/cert"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/tests/mocks/httpmock"
)

const testJWKSURI = "https://rp.example.com/jwks" //nolint:gosec // test URI

// rsaJWKS builds a JWKS JSON string with a single RSA key.
// Pass use="" to omit the "use" field.
func rsaJWKS(pub *rsa.PublicKey, use, kid string) string {
	eBytes := big.NewInt(int64(pub.E)).Bytes()
	key := map[string]interface{}{
		"kty": "RSA",
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(eBytes),
	}
	if use != "" {
		key["use"] = use
	}
	if kid != "" {
		key["kid"] = kid
	}
	b, _ := json.Marshal(map[string]interface{}{"keys": []interface{}{key}})
	return string(b)
}

// multiKeyJWKS builds a JWKS with multiple RSA keys.
func multiKeyJWKS(keys ...map[string]interface{}) string {
	b, _ := json.Marshal(map[string]interface{}{"keys": keys})
	return string(b)
}

func rsaKeyEntry(pub *rsa.PublicKey, kid, alg string) map[string]interface{} {
	eBytes := big.NewInt(int64(pub.E)).Bytes()
	m := map[string]interface{}{
		"kty": "RSA",
		"use": "enc",
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(eBytes),
	}
	if kid != "" {
		m["kid"] = kid
	}
	if alg != "" {
		m["alg"] = alg
	}
	return m
}

type ResolverTestSuite struct {
	suite.Suite
}

func TestResolverTestSuite(t *testing.T) {
	suite.Run(t, new(ResolverTestSuite))
}

// ---------------------------------------------------------------------------
// ResolveEncryptionKey — nil / unsupported cert
// ---------------------------------------------------------------------------

func (suite *ResolverTestSuite) TestResolveEncryptionKey_NilCertificate() {
	r := newJWKSResolver(nil)
	pub, kid, svcErr := r.ResolveEncryptionKey(context.Background(), nil, "RSA-OAEP-256", KeyUseLenientEnc)
	assert.Nil(suite.T(), pub)
	assert.Empty(suite.T(), kid)
	assert.NotNil(suite.T(), svcErr)
}

func (suite *ResolverTestSuite) TestResolveEncryptionKey_EmptyCertType() {
	r := newJWKSResolver(nil)
	cert := &inboundmodel.Certificate{Type: "", Value: "{}"}
	pub, kid, svcErr := r.ResolveEncryptionKey(context.Background(), cert, "RSA-OAEP-256", KeyUseLenientEnc)
	assert.Nil(suite.T(), pub)
	assert.Empty(suite.T(), kid)
	assert.NotNil(suite.T(), svcErr)
}

func (suite *ResolverTestSuite) TestResolveEncryptionKey_UnsupportedCertType() {
	r := newJWKSResolver(nil)
	cert := &inboundmodel.Certificate{Type: "UNKNOWN", Value: "{}"}
	pub, kid, svcErr := r.ResolveEncryptionKey(context.Background(), cert, "RSA-OAEP-256", KeyUseLenientEnc)
	assert.Nil(suite.T(), pub)
	assert.Empty(suite.T(), kid)
	assert.NotNil(suite.T(), svcErr)
}

// ---------------------------------------------------------------------------
// ResolveEncryptionKey — inline JWKS (CertificateTypeJWKS)
// ---------------------------------------------------------------------------

func (suite *ResolverTestSuite) TestResolveEncryptionKey_InlineJWKS_LenientPolicy_EncKey() {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	suite.Require().NoError(err)
	jwks := rsaJWKS(&priv.PublicKey, "enc", "k1")

	r := newJWKSResolver(nil)
	cert := &inboundmodel.Certificate{Type: certmodel.CertificateTypeJWKS, Value: jwks}
	pub, kid, svcErr := r.ResolveEncryptionKey(context.Background(), cert, "RSA-OAEP-256", KeyUseLenientEnc)
	assert.NotNil(suite.T(), pub)
	assert.Equal(suite.T(), "k1", kid)
	assert.Nil(suite.T(), svcErr)
}

func (suite *ResolverTestSuite) TestResolveEncryptionKey_InlineJWKS_LenientPolicy_AbsentUse() {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	suite.Require().NoError(err)
	jwks := rsaJWKS(&priv.PublicKey, "", "k2") // no "use" field

	r := newJWKSResolver(nil)
	cert := &inboundmodel.Certificate{Type: certmodel.CertificateTypeJWKS, Value: jwks}
	pub, kid, svcErr := r.ResolveEncryptionKey(context.Background(), cert, "RSA-OAEP-256", KeyUseLenientEnc)
	assert.NotNil(suite.T(), pub)
	assert.Equal(suite.T(), "k2", kid)
	assert.Nil(suite.T(), svcErr)
}

func (suite *ResolverTestSuite) TestResolveEncryptionKey_InlineJWKS_StrictPolicy_EncKey() {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	suite.Require().NoError(err)
	jwks := rsaJWKS(&priv.PublicKey, "enc", "")

	r := newJWKSResolver(nil)
	cert := &inboundmodel.Certificate{Type: certmodel.CertificateTypeJWKS, Value: jwks}
	pub, _, svcErr := r.ResolveEncryptionKey(context.Background(), cert, "RSA-OAEP-256", KeyUseStrictEnc)
	assert.NotNil(suite.T(), pub)
	assert.Nil(suite.T(), svcErr)
}

func (suite *ResolverTestSuite) TestResolveEncryptionKey_StrictPolicy_AbsentUse_KeySkipped() {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	suite.Require().NoError(err)
	jwks := rsaJWKS(&priv.PublicKey, "", "") // no "use" field

	r := newJWKSResolver(nil)
	cert := &inboundmodel.Certificate{Type: certmodel.CertificateTypeJWKS, Value: jwks}
	pub, _, svcErr := r.ResolveEncryptionKey(context.Background(), cert, "RSA-OAEP-256", KeyUseStrictEnc)
	assert.Nil(suite.T(), pub) // strict mode: absent "use" is skipped
	assert.NotNil(suite.T(), svcErr)
}

// ---------------------------------------------------------------------------
// ResolveEncryptionKey — JWKS URI (CertificateTypeJWKSURI)
// ---------------------------------------------------------------------------

func (suite *ResolverTestSuite) TestResolveEncryptionKey_JWKSURI_Success() {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	suite.Require().NoError(err)
	jwks := rsaJWKS(&priv.PublicKey, "enc", "remote-kid")

	mockHTTP := httpmock.NewHTTPClientInterfaceMock(suite.T())
	mockHTTP.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.URL.String() == testJWKSURI
	})).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(jwks)),
	}, nil)

	r := newJWKSResolver(mockHTTP)
	cert := &inboundmodel.Certificate{Type: certmodel.CertificateTypeJWKSURI, Value: testJWKSURI}
	pub, kid, svcErr := r.ResolveEncryptionKey(context.Background(), cert, "RSA-OAEP-256", KeyUseLenientEnc)
	assert.NotNil(suite.T(), pub)
	assert.Equal(suite.T(), "remote-kid", kid)
	assert.Nil(suite.T(), svcErr)
	mockHTTP.AssertExpectations(suite.T())
}

// ---------------------------------------------------------------------------
// fetchJWKS — error paths
// ---------------------------------------------------------------------------

func (suite *ResolverTestSuite) TestFetchJWKS_NilHTTPClient() {
	r := newJWKSResolver(nil)
	body, svcErr := r.fetchJWKS(context.Background(), testJWKSURI)
	assert.Nil(suite.T(), body)
	assert.NotNil(suite.T(), svcErr)
}

func (suite *ResolverTestSuite) TestFetchJWKS_SSRFUnsafeURI() {
	r := newJWKSResolver(httpmock.NewHTTPClientInterfaceMock(suite.T()))
	body, svcErr := r.fetchJWKS(context.Background(), "http://169.254.169.254/latest/meta-data/")
	assert.Nil(suite.T(), body)
	assert.NotNil(suite.T(), svcErr)
}

func (suite *ResolverTestSuite) TestFetchJWKS_HTTPDoFailure() {
	mockHTTP := httpmock.NewHTTPClientInterfaceMock(suite.T())
	mockHTTP.On("Do", mock.Anything).Return((*http.Response)(nil), assert.AnError)

	r := newJWKSResolver(mockHTTP)
	body, svcErr := r.fetchJWKS(context.Background(), testJWKSURI)
	assert.Nil(suite.T(), body)
	assert.NotNil(suite.T(), svcErr)
	mockHTTP.AssertExpectations(suite.T())
}

func (suite *ResolverTestSuite) TestFetchJWKS_Non200Status() {
	mockHTTP := httpmock.NewHTTPClientInterfaceMock(suite.T())
	mockHTTP.On("Do", mock.Anything).Return(&http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader("")),
	}, nil)

	r := newJWKSResolver(mockHTTP)
	body, svcErr := r.fetchJWKS(context.Background(), testJWKSURI)
	assert.Nil(suite.T(), body)
	assert.NotNil(suite.T(), svcErr)
	mockHTTP.AssertExpectations(suite.T())
}

func (suite *ResolverTestSuite) TestFetchJWKS_BodyExceedsLimit() {
	oversized := strings.Repeat("x", (1<<20)+1)
	mockHTTP := httpmock.NewHTTPClientInterfaceMock(suite.T())
	mockHTTP.On("Do", mock.Anything).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(oversized)),
	}, nil)

	r := newJWKSResolver(mockHTTP)
	body, svcErr := r.fetchJWKS(context.Background(), testJWKSURI)
	assert.Nil(suite.T(), body)
	assert.NotNil(suite.T(), svcErr)
	mockHTTP.AssertExpectations(suite.T())
}

// ---------------------------------------------------------------------------
// parseEncryptionKeyFromJWKS — error / filter paths
// ---------------------------------------------------------------------------

func (suite *ResolverTestSuite) TestParseEncryptionKeyFromJWKS_InvalidJSON() {
	r := newJWKSResolver(nil)
	pub, kid, svcErr := r.parseEncryptionKeyFromJWKS([]byte("not-json"), "RSA-OAEP-256", KeyUseLenientEnc)
	assert.Nil(suite.T(), pub)
	assert.Empty(suite.T(), kid)
	assert.NotNil(suite.T(), svcErr)
}

func (suite *ResolverTestSuite) TestParseEncryptionKeyFromJWKS_NoRSAKeys() {
	jwks := `{"keys":[{"kty":"EC","use":"enc","crv":"P-256","x":"x","y":"y"}]}`
	r := newJWKSResolver(nil)
	pub, _, svcErr := r.parseEncryptionKeyFromJWKS([]byte(jwks), "RSA-OAEP-256", KeyUseLenientEnc)
	assert.Nil(suite.T(), pub)
	assert.NotNil(suite.T(), svcErr)
}

func (suite *ResolverTestSuite) TestParseEncryptionKeyFromJWKS_AlgMismatch() {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	suite.Require().NoError(err)
	// Key explicitly declares alg=RSA-OAEP, but we ask for RSA-OAEP-256.
	entry := rsaKeyEntry(&priv.PublicKey, "", "RSA-OAEP")
	jwks := multiKeyJWKS(entry)

	r := newJWKSResolver(nil)
	pub, _, svcErr := r.parseEncryptionKeyFromJWKS([]byte(jwks), "RSA-OAEP-256", KeyUseLenientEnc)
	assert.Nil(suite.T(), pub)
	assert.NotNil(suite.T(), svcErr)
}

func (suite *ResolverTestSuite) TestParseEncryptionKeyFromJWKS_NoKid_ReturnsEmptyKid() {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	suite.Require().NoError(err)
	jwks := rsaJWKS(&priv.PublicKey, "enc", "") // no kid

	r := newJWKSResolver(nil)
	pub, kid, svcErr := r.parseEncryptionKeyFromJWKS([]byte(jwks), "RSA-OAEP-256", KeyUseLenientEnc)
	assert.NotNil(suite.T(), pub)
	assert.Empty(suite.T(), kid)
	assert.Nil(suite.T(), svcErr)
}

func (suite *ResolverTestSuite) TestParseEncryptionKeyFromJWKS_MultipleKeys_FirstWrongAlg_SecondMatches() {
	priv1, err := rsa.GenerateKey(rand.Reader, 2048)
	suite.Require().NoError(err)
	priv2, err := rsa.GenerateKey(rand.Reader, 2048)
	suite.Require().NoError(err)
	// First key has wrong explicit alg; second matches.
	k1 := rsaKeyEntry(&priv1.PublicKey, "k1", "RSA-OAEP")
	k2 := rsaKeyEntry(&priv2.PublicKey, "k2", "RSA-OAEP-256")
	jwks := multiKeyJWKS(k1, k2)

	r := newJWKSResolver(nil)
	pub, kid, svcErr := r.parseEncryptionKeyFromJWKS([]byte(jwks), "RSA-OAEP-256", KeyUseLenientEnc)
	assert.NotNil(suite.T(), pub)
	assert.Equal(suite.T(), "k2", kid)
	assert.Nil(suite.T(), svcErr)
}

func (suite *ResolverTestSuite) TestParseEncryptionKeyFromJWKS_TwoValidKeys_FirstWins() {
	priv1, err := rsa.GenerateKey(rand.Reader, 2048)
	suite.Require().NoError(err)
	priv2, err := rsa.GenerateKey(rand.Reader, 2048)
	suite.Require().NoError(err)
	k1 := rsaKeyEntry(&priv1.PublicKey, "first", "")
	k2 := rsaKeyEntry(&priv2.PublicKey, "second", "")
	jwks := multiKeyJWKS(k1, k2)

	r := newJWKSResolver(nil)
	pub, kid, svcErr := r.parseEncryptionKeyFromJWKS([]byte(jwks), "RSA-OAEP-256", KeyUseLenientEnc)
	assert.NotNil(suite.T(), pub)
	assert.Equal(suite.T(), "first", kid) // deterministic: first matching key wins
	assert.Nil(suite.T(), svcErr)
}

func (suite *ResolverTestSuite) TestParseEncryptionKeyFromJWKS_SigUseKey_LenientSkips() {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	suite.Require().NoError(err)
	jwks := rsaJWKS(&priv.PublicKey, "sig", "")

	r := newJWKSResolver(nil)
	pub, _, svcErr := r.parseEncryptionKeyFromJWKS([]byte(jwks), "RSA-OAEP-256", KeyUseLenientEnc)
	assert.Nil(suite.T(), pub) // use="sig" is explicitly non-enc; skipped in both policies
	assert.NotNil(suite.T(), svcErr)
}
