// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

func TestInProcessTransport_NotReady(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://id.example.com/users", nil)

	_, err := (&InProcessTransport{}).RoundTrip(req)

	assert.ErrorIs(t, err, ErrInProcessHandlerNotSet)
}

func TestInProcessTransport_ServesWithHandler(t *testing.T) {
	transport := &InProcessTransport{}
	var seen *http.Request
	transport.SetHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello"))
	}))

	req, _ := http.NewRequest(http.MethodGet, "https://id.example.com/users?x=1", nil)
	resp, err := transport.RoundTrip(req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "hello", string(body))
	assert.Equal(t, "/users?x=1", seen.RequestURI)
	assert.Equal(t, "id.example.com", seen.Host)
}

func TestNewInProcessHTTPClient_DoesNotFollowRedirects(t *testing.T) {
	transport := &InProcessTransport{}
	transport.SetHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/elsewhere", http.StatusFound)
	}))
	client := NewInProcessHTTPClient(time.Second, transport)

	resp, err := client.Get("https://id.example.com/start")

	require.NoError(t, err)
	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Equal(t, "/elsewhere", resp.Header.Get("Location"))
}

func TestIsServerOrigin(t *testing.T) {
	config.ResetServerRuntime()
	t.Cleanup(config.ResetServerRuntime)
	require.NoError(t, config.InitializeServerRuntime("", &config.Config{
		Server: engineconfig.ServerConfig{Hostname: "localhost", Port: 8090},
	}))

	cases := map[string]bool{
		"https://localhost:8090/console/auth/backchannel-logout": true,
		"HTTPS://LOCALHOST:8090/x":                               true,
		"https://localhost:8091/x":                               false,
		"http://localhost:8090/x":                                false,
		"https://127.0.0.1:8090/x":                               false,
		"https://localhost/x":                                    false,
		"/relative":                                              false,
		"::":                                                     false,
	}
	for raw, want := range cases {
		assert.Equal(t, want, IsServerOrigin(raw), raw)
	}
}

func TestIsServerOrigin_DefaultPort(t *testing.T) {
	config.ResetServerRuntime()
	t.Cleanup(config.ResetServerRuntime)
	require.NoError(t, config.InitializeServerRuntime("", &config.Config{
		Server: engineconfig.ServerConfig{PublicURL: "https://id.example.com"},
	}))

	assert.True(t, IsServerOrigin("https://id.example.com:443/x"))
	assert.True(t, IsServerOrigin("https://id.example.com/x"))
	assert.False(t, IsServerOrigin("https://id.example.com:8443/x"))
}
