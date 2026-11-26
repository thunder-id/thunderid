// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package github

import (
	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnoauth "github.com/thunder-id/thunderid/internal/authn/oauth"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
)

// Initialize initializes the GitHub OAuth authentication service.
func Initialize(oauthSvc authnoauth.OAuthAuthnServiceInterface,
	clients syshttp.OutboundClientFactoryInterface) GithubOAuthAuthnServiceInterface {
	// The requests carry credentials (the client secret and code, or the access token), so a redirect is
	// not followed: it could resend them to another host.
	httpClient := clients.NewHTTPClientWithoutRedirects(authncm.IDPRequestTimeout)
	return newGithubOAuthAuthnService(oauthSvc, httpClient)
}
