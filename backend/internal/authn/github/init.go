// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"time"

	authnoauth "github.com/thunder-id/thunderid/internal/authn/oauth"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
)

// Initialize initializes the GitHub OAuth authentication service.
func Initialize(oauthSvc authnoauth.OAuthAuthnServiceInterface,
	clients syshttp.OutboundClientProviderInterface) GithubOAuthAuthnServiceInterface {
	// The requests carry credentials (the client secret and code, or the access token), so a redirect is
	// not followed: it could resend them to another host.
	httpClient := clients.NewHTTPClientWithoutRedirects(30 * time.Second)
	return newGithubOAuthAuthnService(oauthSvc, httpClient)
}
