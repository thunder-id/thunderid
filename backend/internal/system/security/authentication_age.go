// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// RequireRecentAuthentication refuses the token when its auth_time is more than maxAge seconds old, or when it
// has no auth_time. A maxAge of zero or less sets no limit.
func RequireRecentAuthentication(ctx context.Context, maxAge int64) *tidcommon.ServiceError {
	if !authenticationExceedsMaxAge(ctx, maxAge, time.Now().Unix()) {
		return nil
	}
	return apierror.ErrRecentAuthenticationRequired.WithParams(
		map[string]string{apierror.ErrorParamMaxAge: strconv.FormatInt(maxAge, 10)})
}

// authenticationExceedsMaxAge reports whether the token's auth_time is more than maxAge seconds before now, in Unix
// seconds. A token without auth_time exceeds every limit; a maxAge of zero or less sets none.
func authenticationExceedsMaxAge(ctx context.Context, maxAge int64, now int64) bool {
	if maxAge <= 0 {
		return false
	}
	authTime, ok := GetAttribute(ctx, claimAuthTime).(float64)
	if !ok {
		return true
	}
	return now-int64(authTime) > maxAge
}

// SetAuthenticationAgeChallenge sets the RFC 9470 challenge asking for an authentication within maxAge seconds.
// The caller answers 401.
func SetAuthenticationAgeChallenge(w http.ResponseWriter, maxAge string) {
	w.Header().Set(serverconst.WWWAuthenticateHeaderName, fmt.Sprintf(
		`%s error="insufficient_user_authentication", `+
			`error_description="A more recent authentication is required", max_age="%s"`,
		serverconst.TokenTypeBearer, maxAge))
}
