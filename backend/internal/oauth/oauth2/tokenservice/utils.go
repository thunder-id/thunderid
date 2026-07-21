/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package tokenservice

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/thunder-id/thunderid/internal/attributecache"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// ParseScopes parses a space-separated scope string into a slice of scope strings.
func ParseScopes(scopeString string) []string {
	trimmed := strings.TrimSpace(scopeString)
	if trimmed == "" {
		return []string{}
	}

	// Split by space and filter out empty strings
	parts := strings.Split(trimmed, " ")
	scopes := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			scopes = append(scopes, part)
		}
	}
	return scopes
}

// JoinScopes joins a slice of scope strings into a space-separated string.
func JoinScopes(scopes []string) string {
	return strings.Join(scopes, " ")
}

// ResolveTokenConfig resolves the token configuration from the OAuth app or falls back to global config.
// accessValidityPeriod is the token subject's configured access-token validity (0 to use the
// global default); it is only consulted for TokenTypeAccess.
func ResolveTokenConfig(
	cfg oauthconfig.Config, oauthApp *providers.OAuthClient, tokenType TokenType,
	accessValidityPeriod int64,
) *TokenConfig {
	tokenConfig := &TokenConfig{
		Issuer:         cfg.JWT.Issuer,
		ValidityPeriod: cfg.JWT.ValidityPeriod,
	}

	// Override with token-type specific configuration if available
	switch tokenType {
	case TokenTypeAccess:
		if accessValidityPeriod > 0 {
			tokenConfig.ValidityPeriod = accessValidityPeriod
		}
	case TokenTypeID:
		if oauthApp != nil && oauthApp.Token != nil && oauthApp.Token.IDToken != nil {
			if oauthApp.Token.IDToken.ValidityPeriod > 0 {
				tokenConfig.ValidityPeriod = oauthApp.Token.IDToken.ValidityPeriod
			}
		}
	case TokenTypeRefresh:
		if cfg.OAuth.RefreshToken.ValidityPeriod > 0 {
			tokenConfig.ValidityPeriod = cfg.OAuth.RefreshToken.ValidityPeriod
		}
		if oauthApp != nil && oauthApp.Token != nil && oauthApp.Token.RefreshToken != nil {
			if oauthApp.Token.RefreshToken.ValidityPeriod > 0 {
				tokenConfig.ValidityPeriod = oauthApp.Token.RefreshToken.ValidityPeriod
			}
		}
	}

	return tokenConfig
}

// extractStringClaim safely extracts a non-empty string claim from a claims map.
func extractStringClaim(claims map[string]interface{}, key string) (string, error) {
	value, ok := claims[key]
	if !ok {
		return "", fmt.Errorf("missing claim: %s", key)
	}

	strValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("claim %s is not a string", key)
	}

	if strValue == "" {
		return "", fmt.Errorf("claim %s is empty", key)
	}

	return strValue, nil
}

// extractAudiences returns the JWT "aud" claim as a []string, accepting either
// the RFC 7519 §4.1.3 string form or the array form. Returns an error when the
// claim is missing, empty, or has an unsupported type.
func extractAudiences(claims map[string]interface{}) ([]string, error) {
	value, ok := claims["aud"]
	if !ok {
		return nil, fmt.Errorf("missing claim: aud")
	}

	switch v := value.(type) {
	case string:
		if v == "" {
			return nil, fmt.Errorf("claim aud is empty")
		}
		return []string{v}, nil
	case []interface{}:
		if len(v) == 0 {
			return nil, fmt.Errorf("claim aud is an empty array")
		}
		result := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok || s == "" {
				return nil, fmt.Errorf("claim aud array contains a non-string or empty element")
			}
			result = append(result, s)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("claim aud has unsupported type")
	}
}

// extractInt64Claim safely extracts an int64 claim from a claims map.
func extractInt64Claim(claims map[string]interface{}, key string) (int64, error) {
	value, ok := claims[key]
	if !ok {
		return 0, fmt.Errorf("missing claim: %s", key)
	}

	// JSON numbers are decoded as float64
	switch v := value.(type) {
	case float64:
		return int64(v), nil
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("claim %s is not a number", key)
	}
}

// extractStringSliceClaim extracts a claim that may be a string or a JSON array of strings.
// Returns nil if the claim is missing or not a recognized type.
func extractStringSliceClaim(claims map[string]interface{}, key string) []string {
	value, ok := claims[key]
	if !ok {
		return nil
	}

	switch v := value.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				result = append(result, s)
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result
	default:
		return nil
	}
}

// extractScopesFromClaims extracts and parses scopes from a claims map.
func extractScopesFromClaims(claims map[string]interface{}, isAuthAssertion bool) []string {
	scopeValue, ok := claims["scope"]
	if ok {
		scopeString, ok := scopeValue.(string)
		if ok && scopeString != "" {
			return ParseScopes(scopeString)
		}
	}

	// This allows auth assertions with authorized_permissions to be used in token exchange
	if isAuthAssertion {
		authorizedPermsValue, ok := claims["authorized_permissions"]
		if ok {
			authorizedPermsString, ok := authorizedPermsValue.(string)
			if ok && authorizedPermsString != "" {
				return ParseScopes(authorizedPermsString)
			}
		}
	}

	return []string{}
}

// getStandardJWTClaims returns the standard JWT claims that should be excluded from user attributes.
func getStandardJWTClaims() map[string]bool {
	return map[string]bool{
		"sub":       true,
		"iss":       true,
		"aud":       true,
		"exp":       true,
		"nbf":       true,
		"iat":       true,
		"jti":       true,
		"scope":     true,
		"client_id": true,
		"act":       true,
	}
}

// ExtractUserAttributes extracts user attributes from JWT claims by filtering out standard claims.
func ExtractUserAttributes(claims map[string]interface{}) map[string]interface{} {
	standardClaims := getStandardJWTClaims()

	userAttributes := make(map[string]interface{})
	for key, value := range claims {
		if !standardClaims[key] {
			userAttributes[key] = value
		}
	}

	return userAttributes
}

// FetchUserAttributes fetches user attributes and merges default claims and groups into the return map.
// Callers should log errors with their own context.
func FetchUserAttributes(
	ctx context.Context,
	attrCacheService attributecache.AttributeCacheServiceInterface,
	allowedClaims []string,
	attributeCacheKey string,
) (map[string]interface{}, error) {
	attrs := make(map[string]interface{})

	// Helper to check if a claim should be included
	shouldInclude := func(claimName string) bool {
		if len(allowedClaims) == 0 {
			return false // Only add special claims if explicitly allowed
		}
		return slices.Contains(allowedClaims, claimName)
	}

	if attributeCacheKey != "" {
		attrCache, err := attrCacheService.GetAttributeCache(ctx, attributeCacheKey)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch attribute cache: %s", err.Error)
		}
		if attrCache == nil || attrCache.Attributes == nil {
			return nil, fmt.Errorf("attribute cache not found for key: %s", attributeCacheKey)
		}
		for key, value := range attrCache.Attributes {
			if shouldInclude(key) {
				attrs[key] = value
			}
		}
	}

	return attrs, nil
}

// BuildClaims builds claims by merging scope-based claims with explicit claims request.
// Explicit claims override scope claims. Returns empty if allowedUserAttributes is not configured.
// The requestedClaims should contain only the relevant claims map (IDToken or UserInfo) for the target.
func BuildClaims(
	scopes []string,
	requestedClaims map[string]*model.IndividualClaimRequest,
	userAttributes map[string]interface{},
	scopeClaimsMapping map[string][]string,
	allowedUserAttributes []string,
) map[string]interface{} {
	result := make(map[string]interface{})

	// Check for openid scope first
	hasOpenIDScope := slices.Contains(scopes, constants.ScopeOpenID)
	if !hasOpenIDScope || userAttributes == nil {
		return result
	}

	// Build scope claims
	scopeClaims := buildClaimsFromScopes(scopes, userAttributes, scopeClaimsMapping, allowedUserAttributes)

	// Process explicit claims request if present
	if requestedClaims != nil {
		explicitClaims := buildClaimsFromRequest(requestedClaims, userAttributes, allowedUserAttributes)

		// Add scope claims that are not explicitly requested
		for k, v := range scopeClaims {
			if _, explicitlyRequested := requestedClaims[k]; !explicitlyRequested {
				result[k] = v
			}
		}

		// Add validated explicit claims (takes precedence over scope claims)
		for claimName, value := range explicitClaims {
			result[claimName] = value
		}
	} else {
		// No explicit claims request, add all scope claims
		for k, v := range scopeClaims {
			result[k] = v
		}
	}

	return result
}

// buildClaimsFromScopes builds claims from OIDC scopes based on scope-to-claims mapping.
func buildClaimsFromScopes(
	scopes []string,
	userAttributes map[string]interface{},
	scopeClaimsMapping map[string][]string,
	allowedUserAttributes []string,
) map[string]interface{} {
	claims := make(map[string]interface{})

	if len(allowedUserAttributes) == 0 || userAttributes == nil || len(scopes) == 0 {
		return claims
	}

	// For each scope, get the claims associated with that scope
	for _, scope := range scopes {
		var scopeClaims []string

		// Check app-specific scope claims first
		if scopeClaimsMapping != nil {
			if appClaims, exists := scopeClaimsMapping[scope]; exists {
				scopeClaims = appClaims
			}
		}

		// Fall back to standard OIDC scopes if no app-specific mapping
		if scopeClaims == nil {
			if standardScope, exists := constants.StandardOIDCScopes[scope]; exists {
				scopeClaims = standardScope.Claims
			}
		}

		// Add claims if they're in user attributes and allowed in config
		for _, claim := range scopeClaims {
			if slices.Contains(allowedUserAttributes, claim) {
				if value, ok := userAttributes[claim]; ok && value != nil {
					claims[claim] = value
				}
			}
		}
	}

	return claims
}

// buildClaimsFromRequest builds claims from explicit claims parameter.
// Returns empty if allowedUserAttributes is not configured.
// Filters claims by availability, allowed attributes, and value/values constraints.
func buildClaimsFromRequest(
	requestedClaims map[string]*model.IndividualClaimRequest,
	userAttributes map[string]interface{},
	allowedUserAttributes []string,
) map[string]interface{} {
	result := make(map[string]interface{})

	if requestedClaims == nil || userAttributes == nil {
		return result
	}

	// Return empty if no allowed attributes configured
	if len(allowedUserAttributes) == 0 {
		return result
	}

	// Process each requested claim
	for claimName, claimReq := range requestedClaims {
		// Check if claim value is available in user attributes
		value, exists := userAttributes[claimName]
		if !exists || value == nil {
			// Per OIDC spec, it's not an error to not return a requested claim
			continue
		}

		// Check if this claim is allowed by app config
		if !slices.Contains(allowedUserAttributes, claimName) {
			continue
		}

		// Check value/values constraints if specified
		if claimReq != nil && !claimReq.MatchesValue(value) {
			// Value doesn't match the requested constraint, skip this claim
			continue
		}

		// TODO: Revisit "essential" claim handling if needed.

		result[claimName] = value
	}

	return result
}

// ReservedAccessTokenClaimNames returns the claim names set by the token builder itself.
// Configured attribute allow-lists (userAttributes, clientAttributes) must not be allowed to use these.
func ReservedAccessTokenClaimNames() map[string]bool {
	reserved := getStandardJWTClaims()
	reserved["grant_type"] = true
	reserved["aci"] = true
	reserved["cnf"] = true
	reserved[constants.ClaimOUID] = true
	reserved[constants.ClaimOUName] = true
	reserved[constants.ClaimOUHandle] = true
	reserved[constants.ClaimClaimsRequest] = true
	reserved[constants.ClaimClaimsLocales] = true
	return reserved
}

// FilterAttributesByAllowList returns the subset of attrs whose keys are listed in the given
// sub-config's Attributes allow-list. Returns an empty map when the sub-config or its allow-list
// is empty. Used by grant handlers to resolve a user subject's access token attributes.
func FilterAttributesByAllowList(
	attrs map[string]interface{}, subConfig *providers.AccessTokenSubConfig,
) map[string]interface{} {
	filtered := make(map[string]interface{})
	if subConfig == nil || len(subConfig.Attributes) == 0 {
		return filtered
	}
	for _, attr := range subConfig.Attributes {
		if val, ok := attrs[attr]; ok {
			filtered[attr] = val
		}
	}
	return filtered
}

// surfaceableClientSystemClaims is the fixed set of entity system-attribute keys that may be
// surfaced as client-token claims. Deliberately excludes clientId/description/clientSecret.
var surfaceableClientSystemClaims = map[string]bool{
	constants.ClaimName:  true,
	constants.ClaimOwner: true,
}

// BuildClientAttributes gathers all OAuth client/application-scoped attributes that should be added
// to an access token for the given OAuth application.
func BuildClientAttributes(
	ctx context.Context,
	oauthApp *providers.OAuthClient,
	ouService providers.OrganizationUnitProvider,
	actorProvider providers.ActorProvider,
) (map[string]interface{}, error) {
	claims := make(map[string]interface{})

	ouClaims, err := resolveClientOUAttributes(ctx, oauthApp, ouService)
	if err != nil {
		return nil, err
	}
	for k, v := range ouClaims {
		claims[k] = v
	}

	entity, err := fetchClientEntity(oauthApp, actorProvider)
	if err != nil {
		return nil, err
	}

	entityClaims, err := resolveClientEntityAttributes(oauthApp, entity)
	if err != nil {
		return nil, err
	}
	for k, v := range entityClaims {
		claims[k] = v
	}

	// Merge system attributes last so they win over a colliding schema attribute.
	systemClaims, err := resolveClientSystemAttributes(oauthApp, entity)
	if err != nil {
		return nil, err
	}
	for k, v := range systemClaims {
		claims[k] = v
	}

	groupRoleClaims, err := resolveClientGroupRoleClaims(oauthApp, actorProvider)
	if err != nil {
		return nil, err
	}
	for k, v := range groupRoleClaims {
		claims[k] = v
	}

	if len(claims) == 0 {
		return nil, nil
	}
	return claims, nil
}

// isClientOUClaim reports whether name is one of the OU claims resolved from the OU service
// (not from the entity).
func isClientOUClaim(name string) bool {
	return name == constants.ClaimOUID || name == constants.ClaimOUName || name == constants.ClaimOUHandle
}

// resolvedWithoutEntity reports whether name is a claim resolved from a source other than the
// entity record (OU service, or the actor's group/role assignments).
func resolvedWithoutEntity(name string) bool {
	return isClientOUClaim(name) ||
		name == constants.UserAttributeGroups || name == constants.UserAttributeRoles
}

// fetchClientEntity loads the backing entity for the OAuth client once, so the schema- and
// system-attribute resolvers can share it. Returns nil when no requested attribute needs the
// entity: an empty allow-list, or one that lists only claims resolved elsewhere (OU, groups, roles).
func fetchClientEntity(
	oauthApp *providers.OAuthClient,
	actorProvider providers.ActorProvider,
) (*providers.Entity, error) {
	if oauthApp == nil || actorProvider == nil {
		return nil, nil
	}

	needsEntity := false
	for _, name := range clientConfigAttributeNames(oauthApp) {
		if !resolvedWithoutEntity(name) {
			needsEntity = true
			break
		}
	}
	if !needsEntity {
		return nil, nil
	}

	entity, svcErr := actorProvider.GetActor(oauthApp.ID)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to fetch entity %s for client attributes: %s", oauthApp.ID, svcErr.Error)
	}
	return entity, nil
}

// clientConfigAttributeNames returns the client-token attribute allow-list, or nil when unset.
func clientConfigAttributeNames(oauthApp *providers.OAuthClient) []string {
	if oauthApp == nil || oauthApp.Token == nil || oauthApp.Token.AccessToken == nil ||
		oauthApp.Token.AccessToken.ClientConfig == nil {
		return nil
	}
	return oauthApp.Token.AccessToken.ClientConfig.Attributes
}

// resolveClientOUAttributes returns the OAuth client's organization unit claims (ouId, ouName,
// ouHandle) selected by ClientConfig.Attributes. Opt-in for every client (agent and application).
func resolveClientOUAttributes(
	ctx context.Context,
	oauthApp *providers.OAuthClient,
	ouService providers.OrganizationUnitProvider,
) (map[string]interface{}, error) {
	if oauthApp == nil || oauthApp.OUID == "" || ouService == nil {
		return nil, nil
	}

	attrNames := clientConfigAttributeNames(oauthApp)
	wantOUID := slices.Contains(attrNames, constants.ClaimOUID)
	wantOUName := slices.Contains(attrNames, constants.ClaimOUName)
	wantOUHandle := slices.Contains(attrNames, constants.ClaimOUHandle)
	if !wantOUID && !wantOUName && !wantOUHandle {
		return nil, nil
	}

	orgUnit, svcErr := ouService.GetOrganizationUnit(ctx, oauthApp.OUID)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to fetch organization unit %s for app %s: %s",
			oauthApp.OUID, oauthApp.ID, svcErr.Error)
	}

	claims := make(map[string]interface{})
	if wantOUID {
		claims[constants.ClaimOUID] = orgUnit.ID
	}
	if wantOUName && orgUnit.Name != "" {
		claims[constants.ClaimOUName] = orgUnit.Name
	}
	if wantOUHandle && orgUnit.Handle != "" {
		claims[constants.ClaimOUHandle] = orgUnit.Handle
	}
	return claims, nil
}

// resolveClientEntityAttributes returns the subset of the OAuth client's own schema attributes
// selected by ClientConfig.Attributes. Resolves generically off Entity.Attributes regardless of
// EntityCategory — populated only for Agent entities (the only category with a schema and
// stored attributes); a no-op for plain Application clients, which have neither.
func resolveClientEntityAttributes(
	oauthApp *providers.OAuthClient,
	entity *providers.Entity,
) (map[string]interface{}, error) {
	if entity == nil || len(entity.Attributes) == 0 {
		return nil, nil
	}

	var allAttrs map[string]interface{}
	if err := json.Unmarshal(entity.Attributes, &allAttrs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal entity attributes for %s: %w", oauthApp.ID, err)
	}

	reserved := ReservedAccessTokenClaimNames()
	filtered := make(map[string]interface{})
	for _, name := range clientConfigAttributeNames(oauthApp) {
		if reserved[name] {
			continue
		}
		if val, ok := allAttrs[name]; ok {
			filtered[name] = val
		}
	}
	return filtered, nil
}

// resolveClientSystemAttributes returns the agent system attributes (name, owner) selected by
// ClientConfig.Attributes, read from Entity.SystemAttributes. Restricted to agent entities so an
// application client can never surface these claims regardless of its stored allow-list.
func resolveClientSystemAttributes(
	oauthApp *providers.OAuthClient,
	entity *providers.Entity,
) (map[string]interface{}, error) {
	if entity == nil || entity.Category != providers.EntityCategoryAgent || len(entity.SystemAttributes) == 0 {
		return nil, nil
	}

	var sysAttrs map[string]interface{}
	if err := json.Unmarshal(entity.SystemAttributes, &sysAttrs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal entity system attributes for %s: %w", oauthApp.ID, err)
	}

	filtered := make(map[string]interface{})
	for _, name := range clientConfigAttributeNames(oauthApp) {
		if !surfaceableClientSystemClaims[name] {
			continue
		}
		if val, ok := sysAttrs[name]; ok {
			filtered[name] = val
		}
	}
	return filtered, nil
}

// resolveClientGroupRoleClaims returns the client's groups and/or roles claims selected by
// ClientConfig.Attributes. Groups come from the actor's transitive memberships; roles are resolved
// from those groups plus direct assignments. The shared group lookup runs once for both.
func resolveClientGroupRoleClaims(
	oauthApp *providers.OAuthClient,
	actorProvider providers.ActorProvider,
) (map[string]interface{}, error) {
	if oauthApp == nil || actorProvider == nil {
		return nil, nil
	}

	attrNames := clientConfigAttributeNames(oauthApp)
	wantGroups := slices.Contains(attrNames, constants.UserAttributeGroups)
	wantRoles := slices.Contains(attrNames, constants.UserAttributeRoles)
	if !wantGroups && !wantRoles {
		return nil, nil
	}

	groups, svcErr := actorProvider.GetActorGroups(oauthApp.ID)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to fetch groups for client %s: %s", oauthApp.ID, svcErr.Error)
	}

	claims := make(map[string]interface{})
	if wantGroups {
		names := make([]string, 0, len(groups))
		for _, g := range groups {
			if g.Name != "" {
				names = append(names, g.Name)
			}
		}
		if len(names) > 0 {
			claims[constants.UserAttributeGroups] = names
		}
	}
	if wantRoles {
		groupIDs := make([]string, 0, len(groups))
		for _, g := range groups {
			if g.ID != "" {
				groupIDs = append(groupIDs, g.ID)
			}
		}
		roles, roleErr := actorProvider.GetActorRoles(oauthApp.ID, groupIDs)
		if roleErr != nil {
			return nil, fmt.Errorf("failed to fetch roles for client %s: %s", oauthApp.ID, roleErr.Error)
		}
		if len(roles) > 0 {
			claims[constants.UserAttributeRoles] = roles
		}
	}
	return claims, nil
}
