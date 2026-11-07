// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package github

import (
	authnoauth "github.com/thunder-id/thunderid/internal/authn/oauth"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
)

// Initialize initializes the GitHub OAuth authentication service.
func Initialize(oauthSvc authnoauth.OAuthAuthnServiceInterface) GithubOAuthAuthnServiceInterface {
	// The user-email endpoint comes from the connection config, so it gets the
	// same SSRF dial guard as the other OAuth endpoints.
	httpClient := syshttp.NewHTTPClient(syshttp.HTTPClientConfig{
		GuardSSRF: true,
	})
	return newGithubOAuthAuthnService(oauthSvc, httpClient)
}
