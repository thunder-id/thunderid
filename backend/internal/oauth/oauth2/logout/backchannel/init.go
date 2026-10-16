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
// to serve, and stops it during graceful shutdown. Deliveries to the server's own origin, such as the
// Console's, go through inProcess instead of the network.
func Initialize(
	tokens tokenservice.TokenBuilderInterface,
	clients providers.ActorProvider,
	events providers.ObservabilityProvider,
	cfg oauthconfig.Config,
	inProcess *syshttp.InProcessTransport,
) DispatcherInterface {
	bcl := cfg.OAuth.Logout.Backchannel
	if !bcl.IsEnabled() {
		return nil
	}
	httpClient := syshttp.NewHTTPClientWithoutRedirects(
		time.Duration(bcl.RequestTimeout)*time.Second, bcl.RejectsPrivateAddresses())
	d := newDispatcher(bcl, tokens, clients, httpClient, events)
	if inProcess != nil {
		d.self = syshttp.NewInProcessHTTPClient(time.Duration(bcl.RequestTimeout)*time.Second, inProcess)
	}
	return d
}
