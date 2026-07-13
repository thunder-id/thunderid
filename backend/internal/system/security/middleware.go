/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

package security

import (
	"context"
	"errors"
	"net/http"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// wwwAuthChallengeInvalidToken is the RFC 6750 §3 challenge returned when a presented access token is
// rejected. Per RFC 6750 §3.1 a rejected token (expired, malformed, or revoked) is signaled as
// invalid_token; the description is intentionally generic and does not disclose which of those
// occurred, so the response leaks nothing about the token's state.
const wwwAuthChallengeInvalidToken = `Bearer error="invalid_token", ` +
	`error_description="The access token is invalid, expired, or malformed"`

// middleware returns an HTTP middleware function that applies security checks to requests.
func middleware(service SecurityServiceInterface) (func(http.Handler) http.Handler, error) {
	if service == nil {
		return nil, errors.New("security service cannot be nil")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Process the security checks
			ctx, err := service.Process(r)
			if err != nil {
				// Write error response and stop request processing
				writeSecurityError(ctx, w, err)
				return
			}

			// Continue with the enriched context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
}

// writeSecurityError writes an appropriate HTTP error response based on the security error, including
// the RFC 6750 WWW-Authenticate challenge.
func writeSecurityError(ctx context.Context, w http.ResponseWriter, err error) {
	// A 403 is an authorization failure: the caller authenticated successfully but lacks the required
	// permission. It is not an authentication challenge, so no WWW-Authenticate header is emitted.
	// (A future change may add the RFC 6750 insufficient_scope challenge here.)
	if errors.Is(err, errForbidden) || errors.Is(err, errInsufficientPermissions) {
		utils.WriteErrorResponse(ctx, w, http.StatusForbidden, apierror.ErrForbidden)
		return
	}

	// A presented-but-rejected token is challenged with invalid_token (RFC 6750 §3.1); a request with
	// no token gets the bare Bearer challenge (RFC 6750 §3: no error code for unauthenticated requests).
	if errors.Is(err, errInvalidToken) {
		w.Header().Set(serverconst.WWWAuthenticateHeaderName, wwwAuthChallengeInvalidToken)
	} else {
		w.Header().Set(serverconst.WWWAuthenticateHeaderName, serverconst.TokenTypeBearer)
	}

	utils.WriteErrorResponse(ctx, w, http.StatusUnauthorized, apierror.ErrUnauthorized)
}
