// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ciba

import (
	"maps"
	"slices"
	"strings"

	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// getRequiredOptionalAttributes determines the space-separated optional user attributes to resolve
// for a CIBA request, so the auth assertion caches them and the token grant can embed them.
//
// This mirrors the authorization_code attribute selection (authz.getRequiredAttributes) for the
// code-flow case, which is the only case CIBA exercises: CIBA always issues an access token and
// never carries an OIDC `claims` request parameter. Because essential attributes only originate from
// the claims parameter, the CIBA essential set is always empty and is therefore set to "" at the
// call site; the optional set is the access-token attributes plus the scope-derived OIDC attributes
// allow-listed for either the ID token or the UserInfo endpoint. The output therefore matches
// authorization_code for the same client config and scope. The format (space-separated) is what the
// assertion executor expects via strings.Fields on the required-attribute runtime keys.
func getRequiredOptionalAttributes(scopes []string, app *providers.OAuthClient) string {
	if app == nil {
		return ""
	}

	optionalAttributes := make(map[string]bool)

	if app.Token != nil && app.Token.AccessToken != nil && app.Token.AccessToken.UserConfig != nil {
		for _, attr := range app.Token.AccessToken.UserConfig.Attributes {
			optionalAttributes[attr] = true
		}
	}

	if slices.Contains(scopes, oauth2const.ScopeOpenID) {
		var idTokenAllowed map[string]bool
		if app.Token != nil {
			idTokenAllowed = buildIDTokenAllowedSet(app.Token.IDToken)
		}
		userInfoAllowed := buildUserInfoAllowedSet(app.UserInfo)
		for _, scope := range scopes {
			for _, attr := range resolveScopeAttributes(scope, app.ScopeClaims) {
				if idTokenAllowed[attr] || userInfoAllowed[attr] {
					optionalAttributes[attr] = true
				}
			}
		}
	}

	return strings.Join(slices.Collect(maps.Keys(optionalAttributes)), " ")
}

// buildIDTokenAllowedSet creates a set of attributes the ID token is allowed to carry.
func buildIDTokenAllowedSet(idTokenConfig *providers.IDTokenConfig) map[string]bool {
	if idTokenConfig == nil || len(idTokenConfig.UserAttributes) == 0 {
		return nil
	}
	allowedSet := make(map[string]bool, len(idTokenConfig.UserAttributes))
	for _, attr := range idTokenConfig.UserAttributes {
		allowedSet[attr] = true
	}
	return allowedSet
}

// buildUserInfoAllowedSet creates a set of attributes the UserInfo endpoint is allowed to return.
func buildUserInfoAllowedSet(userInfoConfig *providers.UserInfoConfig) map[string]bool {
	if userInfoConfig == nil || len(userInfoConfig.UserAttributes) == 0 {
		return nil
	}
	allowedSet := make(map[string]bool, len(userInfoConfig.UserAttributes))
	for _, attr := range userInfoConfig.UserAttributes {
		allowedSet[attr] = true
	}
	return allowedSet
}

// resolveScopeAttributes resolves the attributes mapped to a scope, preferring app-specific
// scope-to-claims mappings and falling back to the standard OIDC scope definitions.
func resolveScopeAttributes(scope string, scopeAttributesMapping map[string][]string) []string {
	if scopeAttributesMapping != nil {
		if appAttributes, exists := scopeAttributesMapping[scope]; exists {
			return appAttributes
		}
	}
	if standardScope, exists := oauth2const.StandardOIDCScopes[scope]; exists {
		return standardScope.Claims
	}
	return nil
}
