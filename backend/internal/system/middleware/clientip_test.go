// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/clientip"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

func TestClientIPMiddleware_PutsResolvedIPOnContext(t *testing.T) {
	mw, err := ClientIPMiddleware([]engineconfig.TrustedProxyConfig{{CIDR: "10.0.0.0/24"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = clientip.FromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "10.0.0.5:443"
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if got != "203.0.113.10" {
		t.Errorf("expected client IP 203.0.113.10, got %q", got)
	}
}

func TestClientIPMiddleware_SetsConnectingAddressWithoutProxies(t *testing.T) {
	mw, err := ClientIPMiddleware(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = clientip.FromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "198.51.100.7:443"
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if got != "198.51.100.7" {
		t.Errorf("expected the connecting address 198.51.100.7, got %q", got)
	}
}

func TestClientIPMiddleware_InvalidConfig(t *testing.T) {
	if _, err := ClientIPMiddleware([]engineconfig.TrustedProxyConfig{{CIDR: "not-a-cidr"}}); err == nil {
		t.Error("expected an error for an invalid cidr")
	}
}
