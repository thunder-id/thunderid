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
		{"X Key", "value"}, {"Content-Type", "value"}, {"Content-Length", "value"}, {"Host", "value"},
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
