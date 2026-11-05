// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocationcache

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/revocation"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

var testIssuer = TrustedIssuer{
	Issuer: "https://idp.example.com", JWKSURL: "https://idp.example.com/jwks", Audience: "https://cp.example.com",
}

// eventToken builds a token shaped like a signed event token. Verification is mocked, so the
// signature segment is a placeholder.
func eventToken(typ string, claims map[string]interface{}) string {
	header, _ := json.Marshal(map[string]interface{}{"alg": "RS256", "typ": typ})
	payload, _ := json.Marshal(claims)
	return base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func push(handler http.HandlerFunc, contentType, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, EventsPath, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set(serverconst.ContentTypeHeaderName, contentType)
	}
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func newTestReceiver(t *testing.T) (*eventReceiver, *revokedCache, *jwtmock.JWTServiceInterfaceMock) {
	jwtService := jwtmock.NewJWTServiceInterfaceMock(t)
	cache := newRevokedCache()
	return newEventReceiver(testIssuer, jwtService, cache), cache, jwtService
}

func subjectEventClaims() map[string]interface{} {
	event := revocation.Event{
		Criterion: revocation.Criterion{Type: revocation.CriterionTypeSubject, Value: "user-1"},
		Reason:    revocation.ReasonUserDeleted, RevokedAt: time.Now(), ExpiryTime: time.Now().Add(time.Hour),
	}
	return event.Claims()
}

func TestEventReceiver_AcceptsAVerifiedEvent(t *testing.T) {
	receiver, cache, jwtService := newTestReceiver(t)
	token := eventToken(revocation.SecurityEventTokenType, subjectEventClaims())
	jwtService.On("VerifyJWTWithJWKS", mock.Anything, token, testIssuer.JWKSURL, testIssuer.Audience,
		testIssuer.Issuer).Return((*tidcommon.ServiceError)(nil))

	rec := push(receiver.handlePush, serverconst.ContentTypeSecurityEventToken+"; charset=utf-8", token)

	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.True(t, cache.isSubjectRevoked("user-1", time.Time{}))
}

func TestEventReceiver_EnforcesAfterAPush(t *testing.T) {
	receiver, cache, jwtService := newTestReceiver(t)
	token := eventToken("SECEVENT+JWT", subjectEventClaims())
	jwtService.On("VerifyJWTWithJWKS", mock.Anything, token, mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	enforcer := newEnforcer(cache)
	identity := security.RevocationIdentity{Subject: "user-1"}
	require.NoError(t, enforcer.EnsureNotRevoked(context.Background(), identity))

	rec := push(receiver.handlePush, serverconst.ContentTypeSecurityEventToken, token)

	require.Equal(t, http.StatusAccepted, rec.Code)
	assert.ErrorIs(t, enforcer.EnsureNotRevoked(context.Background(), identity), errTokenRevoked)
}

func TestEventReceiver_IgnoresAnUnsupportedEventType(t *testing.T) {
	receiver, cache, jwtService := newTestReceiver(t)
	token := eventToken(revocation.SecurityEventTokenType, map[string]interface{}{
		"events": map[string]interface{}{"urn:example:other": map[string]interface{}{}}})
	jwtService.On("VerifyJWTWithJWKS", mock.Anything, token, mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))

	rec := push(receiver.handlePush, serverconst.ContentTypeSecurityEventToken, token)

	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.Empty(t, cache.subjects)
}

func TestEventReceiver_Rejects(t *testing.T) {
	validToken := eventToken(revocation.SecurityEventTokenType, subjectEventClaims())

	tests := []struct {
		name        string
		contentType string
		body        string
		verifyErr   *tidcommon.ServiceError
		wantCode    string
	}{
		{"missing content type", "", validToken, nil, pushErrInvalidRequest},
		{"wrong content type", serverconst.ContentTypeJSON, validToken, nil, pushErrInvalidRequest},
		{"body not a jwt", serverconst.ContentTypeSecurityEventToken, "not-a-jwt", nil, pushErrInvalidRequest},
		{"wrong typ", serverconst.ContentTypeSecurityEventToken,
			eventToken("JWT", subjectEventClaims()), nil, pushErrInvalidRequest},
		{"verification fails", serverconst.ContentTypeSecurityEventToken, validToken,
			&tidcommon.InternalServerError, pushErrAuthenticationFailed},
		{"malformed event", serverconst.ContentTypeSecurityEventToken,
			eventToken(revocation.SecurityEventTokenType, map[string]interface{}{
				"events": map[string]interface{}{revocation.EventTypeCriterionRevoked: map[string]interface{}{}}}),
			nil, pushErrInvalidRequest},
		{"oversized body", serverconst.ContentTypeSecurityEventToken,
			strings.Repeat("a", maxEventBodyBytes+1), nil, pushErrInvalidRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receiver, cache, jwtService := newTestReceiver(t)
			jwtService.On("VerifyJWTWithJWKS", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
				mock.Anything).Return(tt.verifyErr).Maybe()

			rec := push(receiver.handlePush, tt.contentType, tt.body)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			var body pushErrorResponse
			require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
			assert.Equal(t, tt.wantCode, body.Err)
			assert.Empty(t, cache.subjects, "a rejected push changes nothing")
		})
	}
}
