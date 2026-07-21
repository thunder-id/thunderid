/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
// filtered against the UserInfo allowed set. The output therefore matches authorization_code for the
// same client config and scope. The format (space-separated) is what the assertion executor expects
// via strings.Fields on the required-attribute runtime keys.
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
		userInfoAllowed := buildUserInfoAllowedSet(app.UserInfo)
		if userInfoAllowed != nil {
			for _, scope := range scopes {
				for _, attr := range resolveScopeAttributes(scope, app.ScopeClaims) {
					if userInfoAllowed[attr] {
						optionalAttributes[attr] = true
					}
				}
			}
		}
	}

	return strings.Join(slices.Collect(maps.Keys(optionalAttributes)), " ")
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
