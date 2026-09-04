// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package providers

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// IsAllowedGrantType reports whether the given grant type is in the allowed list.
func IsAllowedGrantType(grantTypes []GrantType, grantType GrantType) bool {
	if grantType == "" {
		return false
	}
	return slices.Contains(grantTypes, grantType)
}

// IsAllowedResponseType reports whether the given response type is in the allowed list.
func IsAllowedResponseType(responseTypes []ResponseType, responseType string) bool {
	if responseType == "" {
		return false
	}
	return slices.Contains(responseTypes, ResponseType(responseType))
}

// IsAllowedGrantType reports whether the given grant type is allowed for this client.
func (o *OAuthClient) IsAllowedGrantType(grantType GrantType) bool {
	return IsAllowedGrantType(o.GrantTypes, grantType)
}

// IsAllowedResponseType reports whether the given response type is allowed for this client.
func (o *OAuthClient) IsAllowedResponseType(responseType string) bool {
	return IsAllowedResponseType(o.ResponseTypes, responseType)
}

// IsAllowedTokenEndpointAuthMethod reports whether the given auth method is the one configured for this client.
func (o *OAuthClient) IsAllowedTokenEndpointAuthMethod(method TokenEndpointAuthMethod) bool {
	return o.TokenEndpointAuthMethod == method
}

// ValidateRedirectURI validates the given redirect URI against this client's registered URIs.
func (o *OAuthClient) ValidateRedirectURI(ctx context.Context, redirectURI string) error {
	return ValidateRedirectURI(ctx, o.RedirectURIs, redirectURI)
}

// ValidatePostLogoutRedirectURI validates the given post-logout redirect URI against this client's
// registered post-logout redirect URIs.
func (o *OAuthClient) ValidatePostLogoutRedirectURI(ctx context.Context, postLogoutRedirectURI string) error {
	return ValidatePostLogoutRedirectURI(ctx, o.PostLogoutRedirectURIs, postLogoutRedirectURI)
}

// RequiresPKCE reports whether PKCE is required for this client.
func (o *OAuthClient) RequiresPKCE() bool {
	return o.PKCERequired || o.PublicClient
}

// RequiresPAR reports whether pushed authorization requests are required for this client.
func (o *OAuthClient) RequiresPAR() bool {
	return o.RequirePushedAuthorizationRequests || config.GetServerRuntime().Config.OAuth.PAR.RequirePAR
}

// ShouldAppendActorClaim reports whether an implicit OBO act claim should be added to
// user access tokens issued through this client. Agents always do; applications opt in.
func (o *OAuthClient) ShouldAppendActorClaim() bool {
	return o.EntityCategory == EntityCategoryAgent ||
		(o.EntityCategory == EntityCategoryApp && o.IncludeActClaim)
}

// UserAccessTokenConfig returns the access token sub-config for user-subject tokens
// (authorization_code, refresh_token, token_exchange, ciba), or nil if unset.
func (o *OAuthClient) UserAccessTokenConfig() *AccessTokenSubConfig {
	if o == nil || o.Token == nil || o.Token.AccessToken == nil {
		return nil
	}
	return o.Token.AccessToken.UserConfig
}

// ClientAccessTokenConfig returns the access token sub-config for client-subject tokens
// (client_credentials only), or nil if unset.
func (o *OAuthClient) ClientAccessTokenConfig() *AccessTokenSubConfig {
	if o == nil || o.Token == nil || o.Token.AccessToken == nil {
		return nil
	}
	return o.Token.AccessToken.ClientConfig
}

// ResolveDefaultAudience returns the aud claim for an access token that is not bound to a
// resource server (an OIDC-only or scopeless request). It returns the application's configured
// default audience when set; otherwise it falls back to the given client_id.
func (o *OAuthClient) ResolveDefaultAudience(clientID string) string {
	if o != nil && o.Token != nil && o.Token.AccessToken != nil &&
		o.Token.AccessToken.DefaultAudience != "" {
		return o.Token.AccessToken.DefaultAudience
	}
	return clientID
}

// ValidateRedirectURI validates the provided redirect URI against the registered list.
func ValidateRedirectURI(ctx context.Context, redirectURIs []string, redirectURI string) error {
	logger := log.GetLogger()

	if redirectURI == "" {
		if len(redirectURIs) != 1 {
			return fmt.Errorf("redirect URI is required in the authorization request")
		}
		if strings.Contains(redirectURIs[0], "*") {
			return fmt.Errorf("redirect URI is required in the authorization request")
		}
		parsed, err := url.Parse(redirectURIs[0])
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("registered redirect URI is not fully qualified")
		}
		if strings.EqualFold(parsed.Scheme, "http") {
			hostname := parsed.Hostname()
			if !strings.EqualFold(hostname, "localhost") && hostname != "127.0.0.1" && hostname != "::1" {
				return fmt.Errorf("http redirect URI must use localhost, 127.0.0.1, or [::1]")
			}
		}
		return nil
	}

	parsedRedirectURI, err := utils.ParseURL(redirectURI)
	if err != nil {
		logger.Error(ctx, "Failed to parse redirect URI", log.Error(err))
		return fmt.Errorf("invalid redirect URI: %s", err.Error())
	}
	if strings.EqualFold(parsedRedirectURI.Scheme, "http") {
		hostname := parsedRedirectURI.Hostname()
		if !strings.EqualFold(hostname, "localhost") && hostname != "127.0.0.1" && hostname != "::1" {
			return fmt.Errorf("http redirect URI must use localhost, 127.0.0.1, or [::1]")
		}
	}
	if parsedRedirectURI.Fragment != "" {
		return fmt.Errorf("redirect URI must not contain a fragment component")
	}
	if !matchAnyRedirectURIPattern(redirectURIs, redirectURI) {
		return fmt.Errorf("your application's redirect URL does not match with the registered redirect URLs")
	}

	return nil
}

// ValidatePostLogoutRedirectURI validates a post-logout redirect URI against the registered list.
// An empty URI is allowed (the logout endpoint then lands the user on a default page); a supplied
// URI must match one of the registered post-logout redirect URIs.
func ValidatePostLogoutRedirectURI(ctx context.Context, postLogoutRedirectURIs []string,
	postLogoutRedirectURI string) error {
	if postLogoutRedirectURI == "" {
		return nil
	}
	if !matchAnyRedirectURIPattern(postLogoutRedirectURIs, postLogoutRedirectURI) {
		return fmt.Errorf("post_logout_redirect_uri does not match any registered post-logout redirect URI")
	}
	if _, err := utils.ParseURL(postLogoutRedirectURI); err != nil {
		log.GetLogger().Error(ctx, "Failed to parse post-logout redirect URI", log.Error(err))
		return fmt.Errorf("invalid post_logout_redirect_uri: %s", err.Error())
	}
	return nil
}

func matchAnyRedirectURIPattern(patterns []string, redirectURI string) bool {
	wildcardEnabled := config.GetServerRuntime().Config.OAuth.AllowWildcardRedirectURI
	for _, pattern := range patterns {
		if !wildcardEnabled || !strings.Contains(pattern, "*") {
			if pattern == redirectURI {
				return true
			}
			continue
		}
		matched, err := utils.MatchURIPattern(pattern, redirectURI)
		if err != nil {
			continue
		}
		if matched {
			return true
		}
	}
	return false
}
