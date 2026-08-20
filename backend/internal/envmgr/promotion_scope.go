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

package envmgr

import (
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// requirePromotionScope refuses a request whose caller does not hold the promotion scope.
//
// A token is issued for the organization, and every member of it may edit configuration, capture a
// version, and apply or revert an environment. Promotion is the exception: moving a version towards
// production is a release decision, so it is gated on a scope the token either carries or does not.
//
// The scope name is configurable because the authorization server issuing these tokens is not always
// this one, and its naming is its own.
func requirePromotionScope(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !callerMayPromote(r) {
			writeErrorStatus(w, http.StatusForbidden,
				"promoting requires the "+promotionScope()+" scope, which this token does not carry")
			return
		}
		next(w, r)
	}
}

// callerMayPromote reports whether the request's token carries the promotion scope.
func callerMayPromote(r *http.Request) bool {
	wanted := promotionScope()
	for _, granted := range security.GetPermissions(r.Context()) {
		if strings.EqualFold(strings.TrimSpace(granted), wanted) {
			return true
		}
	}
	return false
}

// promotionScope is the scope a caller must hold to promote.
func promotionScope() string {
	if !config.IsServerRuntimeInitialized() {
		return config.DefaultPromotionScope
	}
	return config.GetServerRuntime().Config.Promotion.PromotionScope()
}
