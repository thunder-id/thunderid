// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package jwks

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
)

type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (suite *InitTestSuite) SetupTest() {
	testConfig := &config.Config{}
	_ = config.InitializeServerRuntime("test", testConfig)
}

func (suite *InitTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *InitTestSuite) TestInitialize() {
	mux := http.NewServeMux()
	cryptoMock := cryptomock.NewRuntimeCryptoProviderMock(suite.T())

	service := Initialize(mux, cryptoMock)

	assert.NotNil(suite.T(), service)
	assert.Implements(suite.T(), (*JWKSServiceInterface)(nil), service)
}

func (suite *InitTestSuite) TestInitialize_RegistersRoutes() {
	mux := http.NewServeMux()
	cryptoMock := cryptomock.NewRuntimeCryptoProviderMock(suite.T())

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(suite.T(), err)
	keys := []providers.PublicKeyInfo{
		{
			KeyID:          "test-kid",
			Algorithm:      string(cryptolib.AlgorithmRS256),
			PublicKey:      &rsaKey.PublicKey,
			Thumbprint:     "test-kid",
			CertificateDER: []byte("raw-cert"),
		},
	}
	cryptoMock.EXPECT().GetPublicKeys(mock.Anything, providers.PublicKeyFilter{}).Return(keys, nil)

	_ = Initialize(mux, cryptoMock)

	req := httptest.NewRequest("GET", "/oauth2/jwks", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.NotEqual(suite.T(), http.StatusNotFound, w.Code)

	req = httptest.NewRequest("OPTIONS", "/oauth2/jwks", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(suite.T(), http.StatusNoContent, w.Code)
}

func (suite *InitTestSuite) TestRegisterRoutes_SetsCorrelationID() {
	cryptoMock := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	cryptoMock.EXPECT().GetPublicKeys(mock.Anything, providers.PublicKeyFilter{}).
		Return([]providers.PublicKeyInfo{}, nil)

	mux := http.NewServeMux()
	registerRoutes(mux, newJWKSHandler(newJWKSService(cryptoMock)))

	req := httptest.NewRequest(http.MethodGet, "/oauth2/jwks", nil)
	req.Header.Set(serverconst.CorrelationIDHeaderName, "trace-abcd")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	suite.Equal("trace-abcd", rec.Header().Get(serverconst.CorrelationIDHeaderName))
}
