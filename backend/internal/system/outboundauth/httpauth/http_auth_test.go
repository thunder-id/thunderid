// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpauth

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/outboundauth"
)

func TestAuthenticationMethods(t *testing.T) {
	tests := []struct {
		name   string
		config outboundauth.Config
		check  func(*testing.T, *http.Request)
	}{
		{"none", outboundauth.Config{Type: outboundauth.TypeNone}, func(t *testing.T, req *http.Request) {
			require.Empty(t, req.Header.Get("Authorization"))
		}},
		{"bearer", outboundauth.Config{Type: outboundauth.TypeBearer, Properties: map[string]string{"token": "secret"}},
			func(t *testing.T, req *http.Request) {
				require.Equal(t, "Bearer secret", req.Header.Get("Authorization"))
			}},
		{"basic", outboundauth.Config{Type: outboundauth.TypeBasic, Properties: map[string]string{
			"username": "client", "password": "secret",
		}}, func(t *testing.T, req *http.Request) {
			username, password, ok := req.BasicAuth()
			require.True(t, ok)
			require.Equal(t, "client", username)
			require.Equal(t, "secret", password)
		}},
		{"multiple API keys", outboundauth.Config{Type: outboundauth.TypeAPIKey, Properties: map[string]string{
			"X-API-Key": "first", "X-Tenant": "second",
		}}, func(t *testing.T, req *http.Request) {
			require.Equal(t, "first", req.Header.Get("X-API-Key"))
			require.Equal(t, "second", req.Header.Get("X-Tenant"))
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authenticator, err := New(test.config)
			require.NoError(t, err)
			req, err := http.NewRequest(http.MethodPost, "https://example.com", nil)
			require.NoError(t, err)
			require.NoError(t, authenticator.Authenticate(context.Background(), req))
			test.check(t, req)
		})
	}
}

func TestRejectsInvalidAPIKeyHeaders(t *testing.T) {
	for _, header := range []struct{ name, value string }{
		{"X Key", "value"}, {"X.Api.Key", "value"}, {"Content-Type", "value"}, {"Content-Length", "value"},
		{"Host", "value"},
		{"Transfer-Encoding", "value"}, {"Connection", "value"}, {"TE", "value"},
		{"Trailer", "value"}, {"Upgrade", "value"}, {"Proxy-Connection", "value"},
		{"Keep-Alive", "value"}, {"Proxy-Authorization", "value"},
		{"X-Key", "line\nnext"}, {"X-Key", "******"},
	} {
		_, err := New(outboundauth.Config{Type: outboundauth.TypeAPIKey,
			Properties: map[string]string{header.name: header.value}})
		require.Error(t, err)
	}
}

func TestRejectsMaskedBearerAndBasicCredentials(t *testing.T) {
	tests := []struct {
		name   string
		config outboundauth.Config
	}{
		{"bearer token", outboundauth.Config{Type: outboundauth.TypeBearer,
			Properties: map[string]string{outboundauth.FieldBearerToken: "******"}}},
		{"basic password", outboundauth.Config{Type: outboundauth.TypeBasic,
			Properties: map[string]string{
				outboundauth.FieldBasicUsername: "client", outboundauth.FieldBasicPassword: "******",
			}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Canonicalize(test.config)
			require.ErrorContains(t, err, "masked")
			_, err = New(test.config)
			require.ErrorContains(t, err, "masked")
		})
	}
}

func TestAllowsBasicUsernameMatchingMask(t *testing.T) {
	_, err := Canonicalize(outboundauth.Config{Type: outboundauth.TypeBasic,
		Properties: map[string]string{
			outboundauth.FieldBasicUsername: "******", outboundauth.FieldBasicPassword: "secret",
		}})
	require.NoError(t, err)
}

func TestAllowsAuthorizationAPIKeyHeader(t *testing.T) {
	authenticator, err := New(outboundauth.Config{
		Type: outboundauth.TypeAPIKey, Properties: map[string]string{"Authorization": "ApiKey secret"},
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, "https://example.com", nil)
	require.NoError(t, err)
	require.NoError(t, authenticator.Authenticate(context.Background(), req))
	require.Equal(t, "ApiKey secret", req.Header.Get("Authorization"))
}

func TestCredentialTransport(t *testing.T) {
	configs := []struct {
		name       string
		config     outboundauth.Config
		headerName string
	}{
		{"bearer", outboundauth.Config{Type: outboundauth.TypeBearer,
			Properties: map[string]string{"token": "secret"}}, "Authorization"},
		{"basic", outboundauth.Config{Type: outboundauth.TypeBasic,
			Properties: map[string]string{"username": "client", "password": "secret"}}, "Authorization"},
		{"API key", outboundauth.Config{Type: outboundauth.TypeAPIKey,
			Properties: map[string]string{"X-API-Key": "secret"}}, "X-API-Key"},
	}
	endpoints := []struct {
		name    string
		url     string
		allowed bool
	}{
		{"remote HTTP", "http://example.com/send", false},
		{"remote HTTPS", "https://example.com/send", true},
		{"localhost HTTP", "http://localhost:8080/send", true},
		{"IPv4 loopback HTTP", "http://127.0.0.1:8080/send", true},
		{"IPv6 loopback HTTP", "http://[::1]:8080/send", true},
	}
	for _, config := range configs {
		t.Run(config.name, func(t *testing.T) {
			authenticator, err := New(config.config)
			require.NoError(t, err)
			for _, endpoint := range endpoints {
				t.Run(endpoint.name, func(t *testing.T) {
					request, err := http.NewRequest(http.MethodPost, endpoint.url, nil)
					require.NoError(t, err)
					err = authenticator.Authenticate(context.Background(), request)
					if endpoint.allowed {
						require.NoError(t, err)
						require.NotEmpty(t, request.Header.Get(config.headerName))
					} else {
						require.ErrorContains(t, err, "HTTPS is required")
						require.Empty(t, request.Header.Get(config.headerName))
					}
				})
			}
		})
	}
}
