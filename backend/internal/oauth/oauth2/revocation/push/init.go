// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"time"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
)

// Initialize builds the dispatcher, or returns nil when revocation push is disabled. The composition
// root registers it on the revocation event hook, starts it once the server is about to serve, and
// stops it during graceful shutdown.
func Initialize(jwtService jwt.JWTServiceInterface, cfg oauthconfig.Config) DispatcherInterface {
	pushCfg := cfg.OAuth.TokenRevocation.Push
	if !pushCfg.IsEnabled() {
		return nil
	}
	httpClient := syshttp.NewHTTPClientWithoutRedirects(
		time.Duration(pushCfg.RequestTimeout)*time.Second, pushCfg.RejectsPrivateAddresses())
	return newDispatcher(pushCfg, jwtService, httpClient)
}
