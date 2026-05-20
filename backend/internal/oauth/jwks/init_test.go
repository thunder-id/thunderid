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
	"github.com/thunder-id/thunderid/internal/system/cryptolab"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
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
	keys := []kmprovider.PublicKeyInfo{
		{
			KeyID:          "test-kid",
			Algorithm:      cryptolab.AlgorithmRS256,
			PublicKey:      &rsaKey.PublicKey,
			Thumbprint:     "test-kid",
			CertificateDER: []byte("raw-cert"),
		},
	}
	cryptoMock.EXPECT().GetPublicKeys(mock.Anything, kmprovider.PublicKeyFilter{}).Return(keys, nil)

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
