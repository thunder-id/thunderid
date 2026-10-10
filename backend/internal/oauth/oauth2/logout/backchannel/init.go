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
	// Logout tokens are posted to tenant-configured logout endpoints, so the
	// SSRF dial guard is applied only where the tenant opted in via
	// RejectsPrivateAddresses; redirects are never followed so the token is
	// not handed to an attacker-controlled hop.
	httpClient := syshttp.NewHTTPClient(syshttp.HTTPClientConfig{
		Timeout:          time.Duration(bcl.RequestTimeout) * time.Second,
		DisableRedirects: true,
		GuardSSRF:        bcl.RejectsPrivateAddresses(),
	})
	return newDispatcher(bcl, tokens, clients, httpClient, events)
}
