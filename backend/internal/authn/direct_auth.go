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
	"crypto/subtle"
	"net/http"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// directAuthHeaderName is the request header carrying the Direct Auth Secret on Direct API requests.
const directAuthHeaderName = "Direct-Auth-Secret"

// DirectAuthGuardInterface enforces the Direct Auth Secret on the Direct API endpoints
// (/auth/**, /register/passkey/**, /access/**).
type DirectAuthGuardInterface interface {
	Wrap(next http.HandlerFunc) http.HandlerFunc
}

// directAuthGuard is secure by default: an empty secret blocks every wrapped endpoint.
type directAuthGuard struct {
	secret string
}

// newDirectAuthGuard creates the Direct Auth Secret guard.
func newDirectAuthGuard(secret string) DirectAuthGuardInterface {
	return &directAuthGuard{secret: secret}
}

// Wrap admits the request only when the configured secret matches the Direct-Auth-Secret header
// (constant-time compare); otherwise it responds 401 with an RFC 6750 Bearer challenge.
func (g *directAuthGuard) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provided := r.Header.Get(directAuthHeaderName)
		if g.secret == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(g.secret)) != 1 {
			g.writeUnauthorized(w, r)
			return
		}
		next(w, r)
	}
}

func (g *directAuthGuard) writeUnauthorized(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(serverconst.WWWAuthenticateHeaderName, serverconst.TokenTypeBearer)
	sysutils.WriteErrorResponse(r.Context(), w, http.StatusUnauthorized, apierror.ErrUnauthorized)
}
