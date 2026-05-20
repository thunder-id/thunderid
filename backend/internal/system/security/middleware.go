/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"errors"
	"net/http"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

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
				writeSecurityError(w, err)
				return
			}

			// Continue with the enriched context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
}

// writeSecurityError writes an appropriate HTTP error response based on the security error.
func writeSecurityError(w http.ResponseWriter, err error) {
	w.Header().Set(serverconst.WWWAuthenticateHeaderName, serverconst.TokenTypeBearer)

	if errors.Is(err, errForbidden) || errors.Is(err, errInsufficientPermissions) {
		utils.WriteErrorResponse(w, http.StatusForbidden, apierror.ErrForbidden)
		return
	}

	utils.WriteErrorResponse(w, http.StatusUnauthorized, apierror.ErrUnauthorized)
}
