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

// Package resourceindicators provides shared helpers for RFC 8707 resource indicator
// processing across OAuth 2.0 grant handlers and the authorization endpoint.
package resourceindicators

import (
	"context"
	"net/url"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/serverconfig"
)

// ValidateResourceURIs returns an error response when any resource URI is not absolute
// or contains a fragment component (RFC 8707 §2, RFC 3986 §4.3).
func ValidateResourceURIs(resources []string) *model.ErrorResponse {
	for _, res := range resources {
		parsedURI, err := url.Parse(res)
		if err != nil || parsedURI.Scheme == "" {
			return &model.ErrorResponse{
				Error:            constants.ErrorInvalidTarget,
				ErrorDescription: "Invalid resource parameter: must be an absolute URI",
			}
		}
		if parsedURI.Fragment != "" {
			return &model.ErrorResponse{
				Error:            constants.ErrorInvalidTarget,
				ErrorDescription: "Invalid resource parameter: must not contain a fragment component",
			}
		}
	}
	return nil
}

// ResolveTargetResourceServer resolves the single target Resource Server for a token request.
// At most one resource is allowed; more than one is invalid_target. When exactly one resource is
// supplied it is resolved by identifier. When none is supplied the deployment's configured
// defaultResourceServer is used; if no default is configured the request is rejected
// (invalid_target) — token issuance is bound to exactly one resource server.
func ResolveTargetResourceServer(
	ctx context.Context,
	resourceService providers.ResourceServerProvider,
	serverConfigService serverconfig.ServerConfigService,
	resources []string,
) (*providers.ResourceServer, *model.ErrorResponse) {
	if errResp := ValidateResourceURIs(resources); errResp != nil {
		return nil, errResp
	}
	if len(resources) > 1 {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidTarget,
			ErrorDescription: "Only a single resource parameter is supported",
		}
	}

	if len(resources) == 1 {
		rs, svcErr := resourceService.GetResourceServerByIdentifier(ctx, resources[0])
		if svcErr != nil {
			return nil, resolveTargetError(svcErr,
				"The resource parameter does not match any registered resource server")
		}
		return rs, nil
	}

	return resolveDefaultResourceServer(ctx, resourceService, serverConfigService)
}

// ResolveAudienceBinding decides the single resource server an access token binds to. It returns
// (nil, nil) when the request carries neither a resource nor any permission scope, for example an
// OIDC-only or scopeless request: such a token is not bound to a resource server, and the caller
// sets its audience to the client_id. Otherwise it resolves the single target resource server via
// ResolveTargetResourceServer (rejecting with invalid_target when none can be determined).
func ResolveAudienceBinding(
	ctx context.Context,
	resourceService providers.ResourceServerProvider,
	serverConfigService serverconfig.ServerConfigService,
	resources []string,
	permissionScopes []string,
) (*providers.ResourceServer, *model.ErrorResponse) {
	if len(resources) == 0 && len(permissionScopes) == 0 {
		return nil, nil
	}
	return ResolveTargetResourceServer(ctx, resourceService, serverConfigService, resources)
}

// resolveDefaultResourceServer resolves the deployment's configured default resource server.
func resolveDefaultResourceServer(
	ctx context.Context,
	resourceService providers.ResourceServerProvider,
	serverConfigService serverconfig.ServerConfigService,
) (*providers.ResourceServer, *model.ErrorResponse) {
	// No server-config service (e.g. the embedded engine) means no default can be configured, so an
	// implicit (no-resource) request cannot be bound to a resource server.
	if serverConfigService == nil {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidTarget,
			ErrorDescription: "No resource parameter supplied and no default resource server is configured",
		}
	}
	merged, svcErr := serverConfigService.GetMergedConfig(ctx, string(serverconfig.ConfigNameDefaultResourceServer))
	if svcErr != nil {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to resolve default resource server",
		}
	}
	cfg, _ := merged.(resource.DefaultResourceServerConfig)
	if cfg.ResourceServerID == "" {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidTarget,
			ErrorDescription: "No resource parameter supplied and no default resource server is configured",
		}
	}
	rs, svcErr := resourceService.GetResourceServer(ctx, cfg.ResourceServerID)
	if svcErr != nil {
		return nil, resolveTargetError(svcErr, "The configured default resource server does not exist")
	}
	return rs, nil
}

// resolveTargetError maps a resource-service error to invalid_target (client) or server_error.
func resolveTargetError(svcErr *tidcommon.ServiceError, invalidTargetDescription string) *model.ErrorResponse {
	if svcErr.Type == tidcommon.ServerErrorType {
		return &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to resolve resource server",
		}
	}
	return &model.ErrorResponse{
		Error:            constants.ErrorInvalidTarget,
		ErrorDescription: invalidTargetDescription,
	}
}

// ResolveResourceServers resolves each resource identifier to its registered Resource Server.
// Returns invalid_target on any unknown identifier (RFC 8707 §2.2).
// The returned slice preserves the order of the input identifiers.
func ResolveResourceServers(
	ctx context.Context,
	resourceService providers.ResourceServerProvider,
	resources []string,
) ([]*providers.ResourceServer, *model.ErrorResponse) {
	if len(resources) == 0 {
		return nil, nil
	}
	resolved := make([]*providers.ResourceServer, 0, len(resources))
	for _, identifier := range resources {
		rs, svcErr := resourceService.GetResourceServerByIdentifier(ctx, identifier)
		if svcErr != nil {
			if svcErr.Type == tidcommon.ServerErrorType {
				return nil, &model.ErrorResponse{
					Error:            constants.ErrorServerError,
					ErrorDescription: "Failed to resolve resource server",
				}
			}
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorInvalidTarget,
				ErrorDescription: "The resource parameter does not match any registered resource server",
			}
		}
		resolved = append(resolved, rs)
	}
	return resolved, nil
}

// ResolveAndDownscope resolves each resource identifier to its registered Resource Server and
// returns the subset of requestedScopes that are defined as permissions on at least one resolved
// RS (RFC 6749 §3.3). Unknown identifiers surface as invalid_target (RFC 8707 §2.2); scopes not
// defined on any RS are silently dropped. The downscoped slice preserves the order of
// requestedScopes. When resources is empty or requestedScopes is empty, scopes are returned
// unchanged.
func ResolveAndDownscope(
	ctx context.Context,
	resourceService providers.ResourceServerProvider,
	resources []string,
	requestedScopes []string,
) ([]*providers.ResourceServer, []string, *model.ErrorResponse) {
	resolvedRSes, errResp := ResolveResourceServers(ctx, resourceService, resources)
	if errResp != nil {
		return nil, nil, errResp
	}
	if len(resolvedRSes) == 0 || len(requestedScopes) == 0 {
		return resolvedRSes, requestedScopes, nil
	}
	rsValidScopes, errResp := ComputeRSValidScopes(ctx, resourceService, resolvedRSes, requestedScopes)
	if errResp != nil {
		return nil, nil, errResp
	}
	allowed := make(map[string]struct{}, len(requestedScopes))
	for _, scopes := range rsValidScopes {
		for _, s := range scopes {
			allowed[s] = struct{}{}
		}
	}
	downscoped := make([]string, 0, len(requestedScopes))
	for _, s := range requestedScopes {
		if _, ok := allowed[s]; ok {
			downscoped = append(downscoped, s)
			delete(allowed, s)
		}
	}
	return resolvedRSes, downscoped, nil
}

// ComputeRSValidScopes returns, for each resolved Resource Server, the subset of requested
// scopes that are defined as permissions on that RS. Scopes not defined on any RS are absent
// from the union of the per-RS slices (downscoping per RFC 6749 §3.3).
func ComputeRSValidScopes(
	ctx context.Context,
	resourceService providers.ResourceServerProvider,
	resolvedRSes []*providers.ResourceServer,
	requestedScopes []string,
) (map[string][]string, *model.ErrorResponse) {
	rsValidScopes := make(map[string][]string, len(resolvedRSes))
	if len(requestedScopes) == 0 || len(resolvedRSes) == 0 {
		return rsValidScopes, nil
	}
	for _, rs := range resolvedRSes {
		invalid, valErr := resourceService.ValidatePermissions(ctx, rs.ID, requestedScopes)
		if valErr != nil {
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to validate permissions",
			}
		}
		invalidSet := make(map[string]struct{}, len(invalid))
		for _, p := range invalid {
			invalidSet[p] = struct{}{}
		}
		valid := make([]string, 0, len(requestedScopes))
		for _, p := range requestedScopes {
			if _, isInvalid := invalidSet[p]; !isInvalid {
				valid = append(valid, p)
			}
		}
		rsValidScopes[rs.ID] = valid
	}
	return rsValidScopes, nil
}

// DownscopeToResourceServer returns the subset of scopes that are defined as permissions on the
// given resource server (RFC 6749 §3.3). Scopes not defined on the RS are dropped. Order of the
// input scopes is preserved. When scopes is empty it is returned unchanged.
func DownscopeToResourceServer(
	ctx context.Context,
	resourceService providers.ResourceServerProvider,
	resourceServerID string,
	scopes []string,
) ([]string, *model.ErrorResponse) {
	if len(scopes) == 0 {
		return scopes, nil
	}
	invalid, svcErr := resourceService.ValidatePermissions(ctx, resourceServerID, scopes)
	if svcErr != nil {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to validate permissions",
		}
	}
	invalidSet := make(map[string]struct{}, len(invalid))
	for _, p := range invalid {
		invalidSet[p] = struct{}{}
	}
	valid := make([]string, 0, len(scopes))
	for _, s := range scopes {
		if _, isInvalid := invalidSet[s]; !isInvalid {
			valid = append(valid, s)
		}
	}
	return valid, nil
}
