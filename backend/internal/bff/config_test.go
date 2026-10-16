// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package bff

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/config"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/actorprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/runtimestoreprovidermock"
)

func TestConsoleConfigFromServerRuntime(t *testing.T) {
	require.NoError(t, config.InitializeServerRuntime("", &config.Config{
		Server:     engineconfig.ServerConfig{PublicURL: "https://id.example.com/", HTTPOnly: false},
		GateClient: engineconfig.GateClientConfig{Path: "/gate"},
		ConsoleClient: config.ConsoleClientConfig{
			ClientID: "CONSOLE", Scopes: []string{"openid"}, Resource: "https://id.example.com/mcp",
		},
	}))
	t.Cleanup(config.ResetServerRuntime)

	cfg := consoleConfigFromServerRuntime()

	assert.Equal(t, "https://id.example.com", cfg.PublicURL)
	assert.Equal(t, "https://id.example.com/mcp", cfg.Resource)
	assert.Equal(t, "CONSOLE", cfg.ClientID)
	assert.Equal(t, "/console/auth", cfg.BasePath)
	assert.True(t, cfg.SecureCookies)
	assert.Equal(t, []string{"/console/", "/gate/"}, cfg.BlockedProxyPrefixes)
}

func TestEndedSessions_StoreFailureIsNotEnded(t *testing.T) {
	store := runtimestoreprovidermock.NewRuntimeStoreProviderMock(t)
	store.EXPECT().Get(mock.Anything, providers.NamespaceConsoleEnded, "s1").Return(nil, errors.New("down"))
	store.EXPECT().Get(mock.Anything, providers.NamespaceConsoleEnded, "s2").
		Return(nil, providers.ErrRuntimeStoreKeyNotFound)

	ended := newEndedSessions(store)

	assert.False(t, ended.isEnded(context.Background(), "s1"))
	assert.False(t, ended.isEnded(context.Background(), "s2"))
	assert.False(t, ended.isEnded(context.Background(), ""))
}

func TestOIDCClient_DiscoveryFailureIsRetried(t *testing.T) {
	calls := 0
	upstream := &syshttp.InProcessTransport{}
	upstream.SetHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"authorization_endpoint":"a","token_endpoint":"t"}`))
	}))
	client := &oidcClient{httpClient: &http.Client{Transport: upstream}, issuerURL: "https://id.example.com"}

	_, err := client.getMetadata(context.Background())
	require.Error(t, err)

	md, err := client.getMetadata(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "t", md.TokenEndpoint)

	_, err = client.getMetadata(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
}

func TestIsTrustedOrigin_ClientLookupFailureTrustsOnlyTheServer(t *testing.T) {
	clients := actorprovidermock.NewActorProviderMock(t)
	clients.EXPECT().GetOAuthClientByClientID(mock.Anything, "CONSOLE").
		Return(nil, &common.ServiceError{Code: "X"})
	h, err := newHandler(Config{ClientID: "CONSOLE", PublicURL: "https://id.example.com",
		SessionMode: SessionModeStateless}, &syshttp.InProcessTransport{}, nil, nil, nil, clients, nil)
	require.NoError(t, err)

	assert.True(t, h.isTrustedOrigin(context.Background(), "https://id.example.com"))
	assert.False(t, h.isTrustedOrigin(context.Background(), "https://localhost:5191"))
	assert.False(t, h.isTrustedOrigin(context.Background(), ""))
	assert.Equal(t, "", originOf("/relative"))
}

func TestNewHandler_RejectsBadSessionConfig(t *testing.T) {
	_, err := newHandler(Config{SessionMode: "sticky"}, &syshttp.InProcessTransport{}, nil, nil, nil, nil, nil)
	assert.ErrorContains(t, err, "unknown session mode")

	_, err = newHandler(Config{SessionMode: SessionModeStateful}, &syshttp.InProcessTransport{}, nil, nil, nil, nil,
		nil)
	assert.ErrorContains(t, err, "idle timeout")
}
