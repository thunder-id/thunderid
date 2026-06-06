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
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
)

// parseTimeField parses a time field from the database result. It preserves any timezone offset
// present in the stored string so a value written in one zone reads back as the same instant.
func parseTimeField(field interface{}, fieldName string) (time.Time, error) {
	switch v := field.(type) {
	case string:
		date, offset := splitTimeAndOffset(v)
		if offset != "" {
			if parsedTime, err := time.Parse("2006-01-02 15:04:05.999999999 -0700", date+" "+offset); err == nil {
				return parsedTime, nil
			}
		}
		// No offset present: treat the wall-clock value as UTC (write side normalizes to UTC).
		if parsedTime, err := time.Parse("2006-01-02 15:04:05.999999999", date); err == nil {
			return parsedTime.UTC(), nil
		}
		parsedTime, err := time.Parse("2006-01-02T15:04:05Z07:00", v)
		if err != nil {
			return time.Time{}, fmt.Errorf("error parsing %s: %w", fieldName, err)
		}
		return parsedTime, nil
	case time.Time:
		return v, nil
	default:
		return time.Time{}, fmt.Errorf("unexpected type for %s", fieldName)
	}
}

// splitTimeAndOffset splits a database time string into its "date time" portion and timezone
// offset token (e.g. "+0530"), if present. Go's time.Time.String() renders values such as
// "2026-06-02 21:57:49.157215 +0530 +0530 m=+595..."; the third space-separated token is the
// numeric offset that must be retained to read the value back as the same instant.
func splitTimeAndOffset(timeStr string) (date, offset string) {
	parts := strings.SplitN(timeStr, " ", 4)
	if len(parts) < 2 {
		return timeStr, ""
	}
	date = parts[0] + " " + parts[1]
	if len(parts) >= 3 && isNumericOffset(parts[2]) {
		offset = parts[2]
	}
	return date, offset
}

// isNumericOffset reports whether the token is a numeric timezone offset like "+0530" or "-0700".
func isNumericOffset(token string) bool {
	if len(token) != 5 || (token[0] != '+' && token[0] != '-') {
		return false
	}
	for _, c := range token[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

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
func getRequiredOptionalAttributes(scopes []string, app *inboundmodel.OAuthClient) string {
	if app == nil {
		return ""
	}

	optionalAttributes := make(map[string]bool)

	if app.Token != nil && app.Token.AccessToken != nil {
		for _, attr := range app.Token.AccessToken.UserAttributes {
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
func buildUserInfoAllowedSet(userInfoConfig *inboundmodel.UserInfoConfig) map[string]bool {
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
