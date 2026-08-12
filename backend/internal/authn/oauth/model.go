// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package oauth

// OAuthEndpoints represents the default OAuth endpoints for an OAuth identity provider.
type OAuthEndpoints struct {
	AuthorizationEndpoint string
	TokenEndpoint         string
	UserInfoEndpoint      string
	UserEmailEndpoint     string
	JwksEndpoint          string
}

// OAuthClientConfig holds the OAuth client configuration details.
type OAuthClientConfig struct {
	ClientID         string
	ClientSecret     string
	RedirectURI      string
	Scopes           []string
	OAuthEndpoints   OAuthEndpoints
	AdditionalParams map[string]string
}

// TokenResponse represents the token endpoint response body.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}
