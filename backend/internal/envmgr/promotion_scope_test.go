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
	"net/http/httptest"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/security"
)

// requestWithScopes builds a request whose caller holds the given scopes.
func requestWithScopes(scopes ...string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/promotions", nil)
	authCtx := security.NewSecurityContextForTest("user-1", "ou-1", "token", scopes, nil)
	return r.WithContext(security.WithSecurityContextTest(r.Context(), authCtx))
}

// A token carrying the promotion scope promotes.
func TestPromotionIsAllowedWithTheScope(t *testing.T) {
	called := false
	handler := requirePromotionScope(func(http.ResponseWriter, *http.Request) { called = true })

	w := httptest.NewRecorder()
	handler(w, requestWithScopes("system:promote"))

	if !called {
		t.Fatal("a caller holding the promotion scope must be allowed through")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected the handler's own status, got %d", w.Code)
	}
}

// Every other scope leaves promotion refused. A member of the organization can still edit, capture,
// apply and revert; only the release decision is gated.
func TestPromotionIsRefusedWithoutTheScope(t *testing.T) {
	called := false
	handler := requirePromotionScope(func(http.ResponseWriter, *http.Request) { called = true })

	w := httptest.NewRecorder()
	handler(w, requestWithScopes("system", "applications:manage"))

	if called {
		t.Fatal("a caller without the promotion scope must not reach the handler")
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

// A request with no security context at all is refused rather than treated as unscoped-and-allowed.
func TestPromotionIsRefusedWithNoScopes(t *testing.T) {
	handler := requirePromotionScope(func(http.ResponseWriter, *http.Request) {
		t.Fatal("an unauthenticated caller must not reach the handler")
	})

	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest(http.MethodPost, "/promotions", nil))

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}
