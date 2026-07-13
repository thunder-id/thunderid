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

package revocation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	serviceerror "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/observability/observabilitymock"
)

const testClientID = "test-client-id"

type RevocationServiceTestSuite struct {
	suite.Suite
	jwtServiceMock *jwtmock.JWTServiceInterfaceMock
	storeMock      *RevokedTokenStoreInterfaceMock
	obsMock        *observabilitymock.ObservabilityServiceInterfaceMock
	service        RevocationServiceInterface
}

func TestRevocationServiceTestSuite(t *testing.T) {
	suite.Run(t, new(RevocationServiceTestSuite))
}

func (s *RevocationServiceTestSuite) SetupTest() {
	s.jwtServiceMock = jwtmock.NewJWTServiceInterfaceMock(s.T())
	s.storeMock = NewRevokedTokenStoreInterfaceMock(s.T())
	s.obsMock = observabilitymock.NewObservabilityServiceInterfaceMock(s.T())
	s.service = newRevocationService(s.jwtServiceMock, s.storeMock, s.obsMock)
}

// buildToken constructs a JWT-shaped string with the given claims. DecodeJWT only base64-decodes the
// header/payload (signature verification is mocked), so a dummy signature segment is sufficient.
func buildToken(claims map[string]interface{}) string {
	header, _ := json.Marshal(map[string]interface{}{"alg": "RS256", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	return base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func (s *RevocationServiceTestSuite) TestRevokeToken_Success() {
	token := buildToken(map[string]interface{}{
		"jti":       "jti-123",
		"client_id": testClientID,
		"exp":       float64(time.Now().Add(time.Hour).Unix()),
	})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)
	s.storeMock.On("InsertRevokedToken", mock.Anything, mock.MatchedBy(func(rt RevokedToken) bool {
		return rt.JTI == "jti-123" && rt.RevocationReason == RevocationReasonExplicit
	})).Return(nil)
	s.obsMock.On("IsEnabled").Return(false)

	revokeOutcome, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, revokeOutcome)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_PublishesAuditEvent() {
	token := buildToken(map[string]interface{}{"jti": "jti-evt", "client_id": testClientID})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)
	s.storeMock.On("InsertRevokedToken", mock.Anything, mock.Anything).Return(nil)
	s.obsMock.On("IsEnabled").Return(true)
	s.obsMock.On("PublishEvent", mock.Anything, mock.Anything).Return()

	revokeOutcome, err := s.service.RevokeToken(context.Background(), token, "refresh_token", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, revokeOutcome)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_InvalidSignatureIsNoOp() {
	token := buildToken(map[string]interface{}{"jti": "jti-123", "client_id": testClientID})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(&serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType, Code: "INVALID_SIGNATURE",
	})

	revokeOutcome, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, revokeOutcome)
	s.storeMock.AssertNotCalled(s.T(), "InsertRevokedToken", mock.Anything, mock.Anything)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_ExpiredTokenStillRevocable() {
	token := buildToken(map[string]interface{}{
		"jti":       "jti-expired",
		"client_id": testClientID,
		"exp":       float64(time.Now().Add(-time.Hour).Unix()),
	})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)
	s.storeMock.On("InsertRevokedToken", mock.Anything, mock.Anything).Return(nil)
	s.obsMock.On("IsEnabled").Return(false)

	revokeOutcome, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, revokeOutcome)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_NotOwnedByClient() {
	token := buildToken(map[string]interface{}{"jti": "jti-123", "client_id": "another-client"})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	revokeOutcome, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeNotOwned, revokeOutcome)
	s.storeMock.AssertNotCalled(s.T(), "InsertRevokedToken", mock.Anything, mock.Anything)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_NoJtiIsNoOp() {
	token := buildToken(map[string]interface{}{"client_id": testClientID})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	revokeOutcome, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, revokeOutcome)
	s.storeMock.AssertNotCalled(s.T(), "InsertRevokedToken", mock.Anything, mock.Anything)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_StoreErrorReturnsError() {
	token := buildToken(map[string]interface{}{"jti": "jti-123", "client_id": testClientID})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)
	s.storeMock.On("InsertRevokedToken", mock.Anything, mock.Anything).Return(errors.New("db error"))

	revokeOutcome, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.Error(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, revokeOutcome)
	assert.Contains(s.T(), err.Error(), "failed to record token revocation")
}

func (s *RevocationServiceTestSuite) TestRevokeRefreshToken_RecordsWithRotationReason() {
	revoker := s.service.(RefreshTokenRevokerInterface)
	expiry := time.Now().Add(time.Hour).UTC()
	s.storeMock.On("InsertRevokedToken", mock.Anything, mock.MatchedBy(func(rt RevokedToken) bool {
		return rt.JTI == "rotated-jti" &&
			rt.RevocationReason == RevocationReasonRefreshRotation &&
			rt.ExpiryTime.Equal(expiry)
	})).Return(nil)

	err := revoker.RevokeRefreshToken(context.Background(), "rotated-jti", expiry)
	assert.NoError(s.T(), err)
}

func (s *RevocationServiceTestSuite) TestRevokeRefreshToken_EmptyJTIIsNoOp() {
	revoker := s.service.(RefreshTokenRevokerInterface)

	err := revoker.RevokeRefreshToken(context.Background(), "", time.Now().UTC())
	assert.NoError(s.T(), err)
	s.storeMock.AssertNotCalled(s.T(), "InsertRevokedToken", mock.Anything, mock.Anything)
}

func (s *RevocationServiceTestSuite) TestRevokeRefreshToken_StoreErrorPropagates() {
	revoker := s.service.(RefreshTokenRevokerInterface)
	s.storeMock.On("InsertRevokedToken", mock.Anything, mock.Anything).
		Return(errors.New("operation database unavailable"))

	err := revoker.RevokeRefreshToken(context.Background(), "jti-x", time.Now().UTC())
	assert.Error(s.T(), err)
}
