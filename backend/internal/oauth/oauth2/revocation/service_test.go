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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	serviceerror "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/observability/observabilitymock"
)

const (
	testClientID = "test-client-id"
	testListURI  = "https://issuer.example/statuslists/abc"
)

// fakeStatusWriter is a hand-rolled TokenStatusWriter capturing the last call.
type fakeStatusWriter struct {
	called    bool
	gotURI    string
	gotIdx    int64
	gotStatus int
	gotExpiry time.Time
	err       error
}

func (f *fakeStatusWriter) SetStatus(_ context.Context, uri string, idx int64, status int, expiry time.Time) error {
	f.called, f.gotURI, f.gotIdx, f.gotStatus, f.gotExpiry = true, uri, idx, status, expiry
	return f.err
}

type RevocationServiceTestSuite struct {
	suite.Suite
	jwtServiceMock *jwtmock.JWTServiceInterfaceMock
	writer         *fakeStatusWriter
	obsMock        *observabilitymock.ObservabilityServiceInterfaceMock
	service        RevocationServiceInterface
}

func TestRevocationServiceTestSuite(t *testing.T) {
	suite.Run(t, new(RevocationServiceTestSuite))
}

func (s *RevocationServiceTestSuite) SetupTest() {
	s.jwtServiceMock = jwtmock.NewJWTServiceInterfaceMock(s.T())
	s.writer = &fakeStatusWriter{}
	s.obsMock = observabilitymock.NewObservabilityServiceInterfaceMock(s.T())
	s.service = newRevocationService(s.jwtServiceMock, s.writer, s.obsMock)
}

// buildToken constructs a JWT-shaped string with the given claims. DecodeJWT only base64-decodes the
// header/payload (signature verification is mocked), so a dummy signature segment is sufficient.
func buildToken(claims map[string]interface{}) string {
	header, _ := json.Marshal(map[string]interface{}{"alg": "RS256", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	return base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

// buildStatusToken builds a token carrying a status_list reference (uri, idx).
func buildStatusToken(jti string, idx int64, extra map[string]interface{}) string {
	claims := map[string]interface{}{
		"jti":       jti,
		"client_id": testClientID,
		"status": map[string]interface{}{
			"status_list": map[string]interface{}{"idx": float64(idx), "uri": testListURI},
		},
	}
	for k, v := range extra {
		claims[k] = v
	}
	return buildToken(claims)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_FlipsStatusBit() {
	exp := float64(time.Now().Add(time.Hour).Unix())
	token := buildStatusToken("jti-123", 42, map[string]interface{}{"exp": exp})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)
	s.obsMock.On("IsEnabled").Return(false)

	outcome, err := s.service.RevokeToken(context.Background(), token, "", testClientID)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, outcome)
	assert.True(s.T(), s.writer.called)
	assert.Equal(s.T(), testListURI, s.writer.gotURI)
	assert.Equal(s.T(), int64(42), s.writer.gotIdx)
	assert.Equal(s.T(), statusRevoked, s.writer.gotStatus)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_PublishesAuditEvent() {
	token := buildStatusToken("jti-evt", 1, nil)
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)
	s.obsMock.On("IsEnabled").Return(true)
	s.obsMock.On("PublishEvent", mock.Anything, mock.Anything).Return()

	outcome, err := s.service.RevokeToken(context.Background(), token, "refresh_token", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, outcome)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_InvalidSignatureIsNoOp() {
	token := buildStatusToken("jti-123", 1, nil)
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(&serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType, Code: "INVALID_SIGNATURE",
	})

	outcome, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, outcome)
	assert.False(s.T(), s.writer.called)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_ExpiredTokenStillRevocable() {
	exp := float64(time.Now().Add(-time.Hour).Unix())
	token := buildStatusToken("jti-expired", 5, map[string]interface{}{"exp": exp})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)
	s.obsMock.On("IsEnabled").Return(false)

	outcome, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, outcome)
	assert.True(s.T(), s.writer.called)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_NotOwnedByClient() {
	token := buildToken(map[string]interface{}{"jti": "jti-123", "client_id": "another-client"})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	outcome, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeNotOwned, outcome)
	assert.False(s.T(), s.writer.called)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_NoJtiIsNoOp() {
	token := buildToken(map[string]interface{}{"client_id": testClientID})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	outcome, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, outcome)
	assert.False(s.T(), s.writer.called)
}

// A token without a status reference has no revocation channel and is a no-op success per RFC 7009.
func (s *RevocationServiceTestSuite) TestRevokeToken_NoStatusRefIsNoOp() {
	token := buildToken(map[string]interface{}{"jti": "jti-noref", "client_id": testClientID})
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	outcome, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, outcome)
	assert.False(s.T(), s.writer.called)
}

// With the Token Status List feature disabled (nil writer), revocation is a no-op success.
func (s *RevocationServiceTestSuite) TestRevokeToken_DisabledWriterIsNoOp() {
	svc := newRevocationService(s.jwtServiceMock, nil, s.obsMock)
	token := buildStatusToken("jti-disabled", 3, nil)
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	outcome, err := svc.RevokeToken(context.Background(), token, "", testClientID)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, outcome)
	assert.False(s.T(), s.writer.called)
}

func (s *RevocationServiceTestSuite) TestRevokeToken_StatusWriterErrorReturnsError() {
	s.writer.err = assert.AnError
	token := buildStatusToken("jti-123", 9, nil)
	s.jwtServiceMock.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	outcome, err := s.service.RevokeToken(context.Background(), token, "", testClientID)
	assert.Error(s.T(), err)
	assert.Equal(s.T(), RevokeOutcomeRevoked, outcome)
	assert.Contains(s.T(), err.Error(), "failed to record token revocation")
}

func (s *RevocationServiceTestSuite) TestRevokeRefreshToken_FlipsStatusBit() {
	revoker := s.service.(RefreshTokenRevokerInterface)
	expiry := time.Now().Add(time.Hour).UTC()

	err := revoker.RevokeRefreshToken(context.Background(), testListURI, 7, "jti-x", expiry)

	assert.NoError(s.T(), err)
	assert.True(s.T(), s.writer.called)
	assert.Equal(s.T(), int64(7), s.writer.gotIdx)
	assert.Equal(s.T(), statusRevoked, s.writer.gotStatus)
	assert.True(s.T(), s.writer.gotExpiry.Equal(expiry))
}

// An empty status URI (pre-feature token, or the feature is off) leaves nothing to record.
func (s *RevocationServiceTestSuite) TestRevokeRefreshToken_EmptyURIIsNoOp() {
	revoker := s.service.(RefreshTokenRevokerInterface)

	err := revoker.RevokeRefreshToken(context.Background(), "", 0, "jti-x", time.Now().UTC())
	assert.NoError(s.T(), err)
	assert.False(s.T(), s.writer.called)
}

func (s *RevocationServiceTestSuite) TestRevokeRefreshToken_WriterErrorPropagates() {
	s.writer.err = assert.AnError
	revoker := s.service.(RefreshTokenRevokerInterface)

	err := revoker.RevokeRefreshToken(context.Background(), testListURI, 1, "jti-x", time.Now().UTC())
	assert.Error(s.T(), err)
}
