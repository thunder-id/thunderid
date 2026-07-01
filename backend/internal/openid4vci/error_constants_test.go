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
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
)

type OID4VCIErrorTestSuite struct {
	suite.Suite
}

func TestOID4VCIErrorTestSuite(t *testing.T) {
	suite.Run(t, new(OID4VCIErrorTestSuite))
}

func (s *OID4VCIErrorTestSuite) TestToOID4VCIError() {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"invalid token", ErrInvalidToken, http.StatusUnauthorized, errCodeInvalidToken},
		{"user not found", ErrUserNotFound, http.StatusUnauthorized, errCodeInvalidToken},
		{"invalid dpop", ErrInvalidDPoP, http.StatusUnauthorized, errCodeInvalidDPoPProof},
		{"invalid nonce", ErrInvalidNonce, http.StatusBadRequest, errCodeInvalidNonce},
		{"invalid proof", ErrInvalidProof, http.StatusBadRequest, errCodeInvalidProof},
		{"unsupported", ErrUnsupportedCredential, http.StatusBadRequest, errCodeUnsupportedCredentialType},
		{"invalid request", ErrInvalidRequest, http.StatusBadRequest, errCodeInvalidCredentialRequest},
		{"wrapped proof", fmt.Errorf("decode: %w", ErrInvalidProof), http.StatusBadRequest, errCodeInvalidProof},
		{"unknown", errors.New("boom"), http.StatusInternalServerError, errCodeServerError},
	}
	for _, c := range cases {
		s.Run(c.name, func() {
			got := toOID4VCIError(c.err)
			s.Equal(c.status, got.Status)
			s.Equal(c.code, got.Code)
		})
	}
}
