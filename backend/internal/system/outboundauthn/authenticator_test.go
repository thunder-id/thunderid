// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outboundauthn

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewAppliesNoAuthentication(t *testing.T) {
	authenticator, err := NewRequestAuthenticator(Config{Scheme: SchemeNone})
	require.NoError(t, err)

	req := httptestRequest(t)
	authenticator.ApplyAuthentication(req)

	require.Empty(t, req.Header)
}

func TestNewAppliesBearerAuthentication(t *testing.T) {
	authenticator, err := NewRequestAuthenticator(Config{Scheme: SchemeBearer, BearerToken: "token"})
	require.NoError(t, err)

	req := httptestRequest(t)
	authenticator.ApplyAuthentication(req)

	require.Equal(t, "Bearer token", req.Header.Get("Authorization"))
}

func TestNewAppliesAPIKeyAuthentication(t *testing.T) {
	authenticator, err := NewRequestAuthenticator(Config{
		Scheme: SchemeAPIKey,
		APIKeyHeaders: map[string]string{
			"X-API-Key": "secret",
			"X-Tenant":  "tenant-1",
		},
	})
	require.NoError(t, err)

	req := httptestRequest(t)
	authenticator.ApplyAuthentication(req)

	require.Equal(t, "secret", req.Header.Get("X-API-Key"))
	require.Equal(t, "tenant-1", req.Header.Get("X-Tenant"))
	require.Empty(t, req.Header.Get("Authorization"))
}

func TestNewRejectsAPIKeyWithoutHeader(t *testing.T) {
	_, err := NewRequestAuthenticator(Config{Scheme: SchemeAPIKey})
	require.EqualError(t, err, "at least one API key header is required")
}

func TestNewRejectsInvalidExplicitAuthenticationConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		error  string
	}{
		{
			name:   "unsupported scheme without credentials",
			config: Config{Scheme: "CUSTOM"},
			error:  `unsupported outbound authentication scheme "CUSTOM"`,
		},
		{
			name:   "unsupported scheme with bearer token",
			config: Config{Scheme: "CUSTOM", BearerToken: "token"},
			error:  `unsupported outbound authentication scheme "CUSTOM"`,
		},
		{
			name:   "bearer scheme without token",
			config: Config{Scheme: SchemeBearer},
			error:  "bearer token is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewRequestAuthenticator(test.config)
			require.EqualError(t, err, test.error)
		})
	}
}

func TestValidateAPIKeyHeader(t *testing.T) {
	tests := []struct {
		name        string
		headerName  string
		headerValue string
		valid       bool
	}{
		{name: "valid", headerName: "x-api-key", headerValue: "secret", valid: true},
		{name: "invalid name", headerName: "X Key", headerValue: "secret"},
		{name: "invalid separator", headerName: "X:Key", headerValue: "secret"},
		{name: "line break in value", headerName: "X-API-Key", headerValue: "secret\r\nX-Other: value"},
		{name: "content type", headerName: "Content-Type", headerValue: "text/plain"},
		{name: "content length", headerName: "Content-Length", headerValue: "10"},
		{name: "host", headerName: "Host", headerValue: "pdp.example.com"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			name, err := ValidateAPIKeyHeader(test.headerName, test.headerValue)
			if test.valid {
				require.NoError(t, err)
				require.Equal(t, "X-Api-Key", name)
				return
			}
			require.Error(t, err)
		})
	}
}

func TestNewInfersAuthenticationScheme(t *testing.T) {
	authenticator, err := NewRequestAuthenticator(Config{BearerToken: "token"})
	require.NoError(t, err)

	req := httptestRequest(t)
	authenticator.ApplyAuthentication(req)

	require.Equal(t, "Bearer token", req.Header.Get("Authorization"))
}

func httptestRequest(t *testing.T) *http.Request {
	t.Helper()
	return httptest.NewRequest(http.MethodGet, "https://example.com", nil)
}
