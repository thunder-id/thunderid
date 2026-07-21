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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
)

type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (s *InitTestSuite) TestRegisterRoutes() {
	svc := NewOpenID4VCIServiceInterfaceMock(s.T())
	svc.EXPECT().GetMetadata(mock.Anything).Return(map[string]interface{}{"credential_issuer": "https://i"}).Maybe()
	svc.EXPECT().GenerateNonce(mock.Anything).Return("nonce", nil).Maybe()

	mux := http.NewServeMux()
	registerRoutes(mux, newOpenID4VCIHandler(svc, nil, "https://i/credential", time.Minute))

	cases := []struct {
		method string
		path   string
		status int
	}{
		{http.MethodGet, metadataPath, http.StatusOK},
		{http.MethodPost, noncePath, http.StatusOK},
		{http.MethodOptions, metadataPath, http.StatusNoContent},
		{http.MethodOptions, credentialOfferPath + "/abc", http.StatusNoContent},
	}
	for _, c := range cases {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(c.method, c.path, nil))
		s.Equal(c.status, rr.Code, "%s %s", c.method, c.path)
	}
}

// Initialize disables the issuer engine (nil service) when no signing key is configured.
func (s *InitTestSuite) TestInitializeDisabledWithoutSigningKey() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("", &config.Config{}))
	defer config.ResetServerRuntime()

	svc, err := Initialize(http.NewServeMux(), nil, nil, nil, nil, nil, nil)
	s.Require().NoError(err)
	s.Nil(svc)
}
