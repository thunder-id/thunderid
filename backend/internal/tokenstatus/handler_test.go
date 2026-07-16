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

package tokenstatus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// testSignedToken is the canned Status List Token the handler test double returns.
const testSignedToken = "signed.status.list"

// fakeService is a ServiceInterface test double for the handler tests.
type fakeService struct {
	token string
	ttl   int
	err   error
}

func (f fakeService) IssueReference(context.Context) (int64, string, error) { return 0, "", nil }
func (f fakeService) SetStatus(context.Context, string, int64, int, time.Time) error {
	return nil
}
func (f fakeService) GetStatus(context.Context, string, int64) (int, error) { return 0, nil }
func (f fakeService) Produce(context.Context, string) (string, int, error) {
	return f.token, f.ttl, f.err
}

func newRequest(id, accept string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/statuslists/"+id, nil)
	req.SetPathValue("id", id)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	return req
}

func TestHandlerServesSignedToken(t *testing.T) {
	h := newHandler(fakeService{token: testSignedToken, ttl: 3600})
	rec := httptest.NewRecorder()

	h.handleGet(rec, newRequest("abc", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != statusListMediaTypeJWT {
		t.Fatalf("Content-Type = %q, want %q", ct, statusListMediaTypeJWT)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "max-age=3600" {
		t.Fatalf("Cache-Control = %q, want max-age=3600", cc)
	}
	if rec.Body.String() != testSignedToken {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestHandlerNotFound(t *testing.T) {
	h := newHandler(fakeService{err: ErrListNotFound})
	rec := httptest.NewRecorder()

	h.handleGet(rec, newRequest("missing", ""))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandlerRejectsCWTOnly(t *testing.T) {
	h := newHandler(fakeService{token: "unused"})
	rec := httptest.NewRecorder()

	h.handleGet(rec, newRequest("abc", statusListMediaTypeCWT))

	if rec.Code != http.StatusNotAcceptable {
		t.Fatalf("status = %d, want 406", rec.Code)
	}
}

func TestHandlerProduceError(t *testing.T) {
	h := newHandler(fakeService{err: errAllocationExhausted})
	rec := httptest.NewRecorder()

	h.handleGet(rec, newRequest("abc", ""))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestRegisterRoutes(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, fakeService{token: testSignedToken, ttl: 3600})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/statuslists/abc", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != testSignedToken {
		t.Fatalf("body = %q", rec.Body.String())
	}

	orec := httptest.NewRecorder()
	mux.ServeHTTP(orec, httptest.NewRequest(http.MethodOptions, "/statuslists/abc", nil))
	if orec.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS status = %d, want 204", orec.Code)
	}
}

func TestAcceptsJWT(t *testing.T) {
	tests := []struct {
		accept string
		want   bool
	}{
		{"", true},
		{statusListMediaTypeJWT, true},
		{"*/*", true},
		{"application/*", true},
		{statusListMediaTypeJWT + ", " + statusListMediaTypeCWT, true},
		{statusListMediaTypeCWT, false},
	}
	for _, tt := range tests {
		if got := acceptsJWT(tt.accept); got != tt.want {
			t.Fatalf("acceptsJWT(%q) = %v, want %v", tt.accept, got, tt.want)
		}
	}
}
