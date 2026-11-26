// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
)

type AuthenticationAgeTestSuite struct {
	suite.Suite
}

func TestAuthenticationAgeTestSuite(t *testing.T) {
	suite.Run(t, new(AuthenticationAgeTestSuite))
}

func contextWithAuthTime(authTime any) context.Context {
	attributes := map[string]any{}
	if authTime != nil {
		attributes[claimAuthTime] = authTime
	}
	return withSecurityContext(context.Background(),
		newSecurityContext("user-1", "ou-1", "token", nil, attributes))
}

func (s *AuthenticationAgeTestSuite) TestAuthenticationExceedsMaxAge() {
	const now = int64(1700000000)
	tests := []struct {
		name     string
		authTime any
		maxAge   int64
		expected bool
	}{
		{name: "no limit", authTime: nil, maxAge: 0, expected: false},
		{name: "negative limit", authTime: nil, maxAge: -1, expected: false},
		{name: "within the limit", authTime: float64(now - 10), maxAge: 300, expected: false},
		{name: "at the limit", authTime: float64(now - 300), maxAge: 300, expected: false},
		{name: "older than the limit", authTime: float64(now - 301), maxAge: 300, expected: true},
		{name: "no auth_time", authTime: nil, maxAge: 300, expected: true},
		{name: "auth_time not a number", authTime: "recent", maxAge: 300, expected: true},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.expected, authenticationExceedsMaxAge(contextWithAuthTime(tt.authTime), tt.maxAge, now))
		})
	}
}

func (s *AuthenticationAgeTestSuite) TestAuthenticationExceedsMaxAge_NoSecurityContext() {
	s.True(authenticationExceedsMaxAge(context.Background(), 300, 1700000000))
}

func (s *AuthenticationAgeTestSuite) TestRequireRecentAuthentication() {
	tests := []struct {
		name       string
		authAge    int64
		noAuthTime bool
		maxAge     int64
		wantErr    bool
	}{
		{name: "recent", authAge: 10, maxAge: 300},
		{name: "check off", noAuthTime: true, maxAge: 0},
		{name: "old", authAge: 3600, maxAge: 300, wantErr: true},
		{name: "no auth_time", noAuthTime: true, maxAge: 300, wantErr: true},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			var authTime any
			if !tt.noAuthTime {
				authTime = float64(time.Now().Unix() - tt.authAge)
			}

			svcErr := RequireRecentAuthentication(contextWithAuthTime(authTime), tt.maxAge)

			if !tt.wantErr {
				s.Nil(svcErr)
				return
			}
			s.Require().NotNil(svcErr)
			s.Equal(apierror.ErrRecentAuthenticationRequired.Code, svcErr.Code)
			s.Equal("300", svcErr.ErrorDescription.Params[apierror.ErrorParamMaxAge])
		})
	}
}

func (s *AuthenticationAgeTestSuite) TestSetAuthenticationAgeChallenge() {
	w := httptest.NewRecorder()

	SetAuthenticationAgeChallenge(w, "120")

	s.Equal(`Bearer error="insufficient_user_authentication", `+
		`error_description="A more recent authentication is required", max_age="120"`,
		w.Header().Get("WWW-Authenticate"))
}
