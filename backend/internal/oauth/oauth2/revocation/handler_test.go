// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
)

type RevocationHandlerTestSuite struct {
	suite.Suite
	serviceMock *RevocationServiceInterfaceMock
	handler     *revocationHandler
}

func TestRevocationHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(RevocationHandlerTestSuite))
}

func (s *RevocationHandlerTestSuite) SetupTest() {
	s.serviceMock = NewRevocationServiceInterfaceMock(s.T())
	s.handler = newRevocationHandler(s.serviceMock, discardingBus{})
}

// newRevokeRequest builds a POST /oauth2/revoke request with a form body and an authenticated client.
func newRevokeRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/oauth2/revoke", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := context.WithValue(req.Context(), clientauth.OAuthClientKey,
		&clientauth.OAuthClientInfo{ClientID: testClientID})
	return req.WithContext(ctx)
}

func (s *RevocationHandlerTestSuite) TestHandleRevoke_Success() {
	s.serviceMock.On("RevokeToken", mock.Anything, "tok", "", testClientID).
		Return(RevokeOutcomeRevoked, nil)

	rec := httptest.NewRecorder()
	s.handler.HandleRevoke(rec, newRevokeRequest("token=tok"))

	assert.Equal(s.T(), http.StatusOK, rec.Code)
	s.serviceMock.AssertExpectations(s.T())
}

func (s *RevocationHandlerTestSuite) TestHandleRevoke_PassesTokenTypeHintAndClient() {
	s.serviceMock.On("RevokeToken", mock.Anything, "tok", "refresh_token", testClientID).
		Return(RevokeOutcomeRevoked, nil)

	rec := httptest.NewRecorder()
	s.handler.HandleRevoke(rec, newRevokeRequest("token=tok&token_type_hint=refresh_token"))

	assert.Equal(s.T(), http.StatusOK, rec.Code)
	s.serviceMock.AssertExpectations(s.T())
}

func (s *RevocationHandlerTestSuite) TestHandleRevoke_MissingToken() {
	rec := httptest.NewRecorder()
	s.handler.HandleRevoke(rec, newRevokeRequest(""))

	assert.Equal(s.T(), http.StatusBadRequest, rec.Code)
	assert.Contains(s.T(), rec.Body.String(), "invalid_request")
	s.serviceMock.AssertNotCalled(s.T(), "RevokeToken", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *RevocationHandlerTestSuite) TestHandleRevoke_NotOwnedReturnsInvalidGrant() {
	s.serviceMock.On("RevokeToken", mock.Anything, "tok", "", testClientID).
		Return(RevokeOutcomeNotOwned, nil)

	rec := httptest.NewRecorder()
	s.handler.HandleRevoke(rec, newRevokeRequest("token=tok"))

	assert.Equal(s.T(), http.StatusBadRequest, rec.Code)
	assert.Contains(s.T(), rec.Body.String(), "invalid_grant")
}

func (s *RevocationHandlerTestSuite) TestHandleRevoke_ServerError() {
	s.serviceMock.On("RevokeToken", mock.Anything, "tok", "", testClientID).
		Return(RevokeOutcomeRevoked, errors.New("db down"))

	rec := httptest.NewRecorder()
	s.handler.HandleRevoke(rec, newRevokeRequest("token=tok"))

	assert.Equal(s.T(), http.StatusInternalServerError, rec.Code)
	assert.Contains(s.T(), rec.Body.String(), "server_error")
}
