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
	"sort"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
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

// ResolveResourceServers resolves each resource identifier to its registered Resource Server.
// Returns invalid_target on any unknown identifier (RFC 8707 §2.2).
// The returned slice preserves the order of the input identifiers.
func ResolveResourceServers(
	ctx context.Context,
	resourceService resource.ResourceServiceInterface,
	resources []string,
) ([]*resource.ResourceServer, *model.ErrorResponse) {
	if len(resources) == 0 {
		return nil, nil
	}
	resolved := make([]*resource.ResourceServer, 0, len(resources))
	for _, identifier := range resources {
		rs, svcErr := resourceService.GetResourceServerByIdentifier(ctx, identifier)
		if svcErr != nil {
			if svcErr.Type == serviceerror.ServerErrorType {
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
	resourceService resource.ResourceServiceInterface,
	resources []string,
	requestedScopes []string,
) ([]*resource.ResourceServer, []string, *model.ErrorResponse) {
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
	resourceService resource.ResourceServiceInterface,
	resolvedRSes []*resource.ResourceServer,
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

// UnionScopes returns the union of all per-RS valid scope slices in deterministic order.
func UnionScopes(rsValidScopes map[string][]string) []string {
	keys := make([]string, 0, len(rsValidScopes))
	for k := range rsValidScopes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	seen := make(map[string]struct{})
	union := []string{}
	for _, k := range keys {
		for _, s := range rsValidScopes[k] {
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			union = append(union, s)
		}
	}
	return union
}

// ComposeAudiences builds the aud claim from the resolved Resource Servers. When resolvedRSes is
// non-nil (explicit resource parameter), all resolved RS identifiers are included. When
// resolvedRSes is nil and granted scopes are present, contributors are discovered via the resource
// service (implicit audience). If no RS contributes, clientID is returned as the sole fallback
// audience. If clientID is also empty, an empty slice is returned.
func ComposeAudiences(
	ctx context.Context,
	resourceService resource.ResourceServiceInterface,
	clientID string,
	resolvedRSes []*resource.ResourceServer,
	grantedScopes []string,
) ([]string, *model.ErrorResponse) {
	var rsIdentifiers []string
	if resolvedRSes != nil {
		rsIdentifiers = ContributingAudiences(resolvedRSes)
	} else if len(grantedScopes) > 0 {
		implicit, svcErr := resourceService.FindResourceServersByPermissions(ctx, grantedScopes)
		if svcErr != nil {
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to resolve resource servers for granted scopes",
			}
		}
		rsIdentifiers = make([]string, 0, len(implicit))
		for _, rs := range implicit {
			if rs.Identifier != "" {
				rsIdentifiers = append(rsIdentifiers, rs.Identifier)
			}
		}
		sort.Strings(rsIdentifiers)
	}

	if len(rsIdentifiers) > 0 {
		seen := make(map[string]struct{}, len(rsIdentifiers))
		deduped := make([]string, 0, len(rsIdentifiers))
		for _, id := range rsIdentifiers {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			deduped = append(deduped, id)
		}
		return deduped, nil
	}

	if clientID != "" {
		return []string{clientID}, nil
	}
	return []string{}, nil
}

// FilterByIdentifiers returns the subset of resolvedRSes whose Identifier is in identifiers.
// Preserves the order of resolvedRSes.
func FilterByIdentifiers(resolvedRSes []*resource.ResourceServer, identifiers []string) []*resource.ResourceServer {
	idSet := make(map[string]struct{}, len(identifiers))
	for _, id := range identifiers {
		idSet[id] = struct{}{}
	}
	filtered := make([]*resource.ResourceServer, 0, len(identifiers))
	for _, rs := range resolvedRSes {
		if _, ok := idSet[rs.Identifier]; ok {
			filtered = append(filtered, rs)
		}
	}
	return filtered
}

// ContributingAudiences returns the identifiers of all explicitly resolved Resource Servers.
// When the client explicitly requests resource targets, those targets form the token audience.
// Preserves the order of resolvedRSes.
func ContributingAudiences(
	resolvedRSes []*resource.ResourceServer,
) []string {
	if len(resolvedRSes) == 0 {
		return nil
	}
	auds := make([]string, 0, len(resolvedRSes))
	for _, rs := range resolvedRSes {
		if rs.Identifier != "" {
			auds = append(auds, rs.Identifier)
		}
	}
	return auds
}
