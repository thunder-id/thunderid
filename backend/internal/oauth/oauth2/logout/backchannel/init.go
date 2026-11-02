// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package backchannel

import (
	"time"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize builds the dispatcher, or returns nil when back-channel logout is disabled. The
// composition root registers it on the session termination hook, starts it once the server is about
// to serve, and stops it during graceful shutdown.
func Initialize(
	tokens tokenservice.TokenBuilderInterface,
	clients providers.ActorProvider,
	events providers.ObservabilityProvider,
	cfg oauthconfig.Config,
) DispatcherInterface {
	bcl := cfg.OAuth.Logout.Backchannel
	if !bcl.IsEnabled() {
		return nil
	}
	httpClient := syshttp.NewHTTPClientWithoutRedirects(
		time.Duration(bcl.RequestTimeout)*time.Second, bcl.RejectsPrivateAddresses())
	return newDispatcher(bcl, tokens, clients, httpClient, events)
}
