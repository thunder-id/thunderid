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

package authn

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
)

// DirectAuthGuardTestSuite exercises the Direct Auth Secret guard.
type DirectAuthGuardTestSuite struct {
	suite.Suite
}

func TestDirectAuthGuardTestSuite(t *testing.T) {
	suite.Run(t, new(DirectAuthGuardTestSuite))
}

// serve wraps a next handler with the guard, drives the request, and reports whether next ran.
func (suite *DirectAuthGuardTestSuite) serve(
	secret, providedHeader string, setHeader bool) (*httptest.ResponseRecorder, bool) {
	nextCalled := false
	next := func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/credentials/authenticate", nil)
	if setHeader {
		req.Header.Set(directAuthHeaderName, providedHeader)
	}
	rec := httptest.NewRecorder()

	newDirectAuthGuard(secret).Wrap(next)(rec, req)
	return rec, nextCalled
}

// assertRejected asserts the response is a 401 carrying the RFC 6750 Bearer challenge.
func (suite *DirectAuthGuardTestSuite) assertRejected(rec *httptest.ResponseRecorder) {
	suite.Equal(http.StatusUnauthorized, rec.Code)
	suite.Equal(serverconst.TokenTypeBearer, rec.Header().Get(serverconst.WWWAuthenticateHeaderName))
}

func (suite *DirectAuthGuardTestSuite) TestValidSecretIsAdmitted() {
	rec, nextCalled := suite.serve("s3cr3t-value", "s3cr3t-value", true)

	suite.True(nextCalled)
	suite.Equal(http.StatusOK, rec.Code)
	suite.Empty(rec.Header().Get(serverconst.WWWAuthenticateHeaderName))
}

func (suite *DirectAuthGuardTestSuite) TestMissingSecretIsRejected() {
	rec, nextCalled := suite.serve("s3cr3t-value", "", false)

	suite.False(nextCalled)
	suite.assertRejected(rec)
}

func (suite *DirectAuthGuardTestSuite) TestWrongSecretIsRejected() {
	rec, nextCalled := suite.serve("s3cr3t-value", "wrong", true)

	suite.False(nextCalled)
	suite.assertRejected(rec)
}

// TestUnconfiguredSecretBlocksEndpoint verifies the secure-by-default behavior: no configured secret
// blocks the endpoint even when a header value is sent.
func (suite *DirectAuthGuardTestSuite) TestUnconfiguredSecretBlocksEndpoint() {
	rec, nextCalled := suite.serve("", "anything", true)

	suite.False(nextCalled)
	suite.assertRejected(rec)
}

func (suite *DirectAuthGuardTestSuite) TestUnconfiguredSecretBlocksEndpointWithoutHeader() {
	rec, nextCalled := suite.serve("", "", false)

	suite.False(nextCalled)
	suite.assertRejected(rec)
}
