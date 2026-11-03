// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package granthandlers

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	authzdetailcommon "github.com/thunder-id/thunderid/internal/authzdetail/common"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// excludeRevokedScopes returns the OIDC scopes and the permission scopes not revoked since a grant was
// established, refusing the grant when it was revoked itself, when revocation leaves no scope to grant, or
// when the deny list cannot be read.
func excludeRevokedScopes(ctx context.Context, enforcementService revocation.EnforcementServiceInterface,
	grant revocation.RevocationIdentity, subject string, audiences, oidcScopes, scopes []string,
) ([]string, *model.ErrorResponse) {
	// Nil only when oauth.token_revocation is disabled, as the token validator also treats it.
	if enforcementService == nil {
		return slices.Concat(oidcScopes, scopes), nil
	}
	// Second precision, like a token's iat, so a revocation within the grant's issuing second applies.
	grant.EstablishedAt = grant.EstablishedAt.UTC().Truncate(time.Second)

	// One lookup answers the common case; only a match is narrowed to the grant or the revoked scopes.
	err := enforcementService.EnsureNotRevoked(ctx, revocation.RevocationIdentity{
		EstablishedAt: grant.EstablishedAt,
		Criteria:      append(revocation.ScopeCriteria(subject, audiences, scopes), grant.Criteria...),
	})
	if err == nil {
		return slices.Concat(oidcScopes, scopes), nil
	}
	if !errors.Is(err, revocation.ErrTokenRevoked) {
		return nil, grantRevocationError(err)
	}
	if err := enforcementService.EnsureNotRevoked(ctx, grant); err != nil {
		return nil, grantRevocationError(err)
	}

	kept := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		err := enforcementService.EnsureNotRevoked(ctx, revocation.RevocationIdentity{
			EstablishedAt: grant.EstablishedAt,
			Criteria:      revocation.ScopeCriteria(subject, audiences, []string{scope}),
		})
		if err == nil {
			kept = append(kept, scope)
		} else if !errors.Is(err, revocation.ErrTokenRevoked) {
			return nil, grantRevocationError(err)
		}
	}
	// A response without scope reads as granted as requested (RFC 6749 §5.1), so nothing left is a refusal.
	if len(oidcScopes) == 0 && len(kept) == 0 {
		return nil, grantRevocationError(revocation.ErrTokenRevoked)
	}
	return slices.Concat(oidcScopes, kept), nil
}

// grantRevocationError maps a deny-list result to the token error refusing the grant.
func grantRevocationError(err error) *model.ErrorResponse {
	if errors.Is(err, revocation.ErrTokenRevoked) {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "The authorization grant has been revoked",
		}
	}
	return &model.ErrorResponse{
		Error:            constants.ErrorServerError,
		ErrorDescription: "Token revocation status could not be verified",
	}
}

// refreshableAuthorizationDetails returns the granted details a refresh token may carry.
func refreshableAuthorizationDetails(ctx context.Context,
	authzDetails providers.AuthorizationDetailProvider,
	granted []providers.AuthorizationDetail) ([]providers.AuthorizationDetail, *model.ErrorResponse) {
	refreshable, svcErr := authzDetails.RefreshableAuthorizationDetails(ctx, granted)
	if svcErr != nil {
		return nil, oauth2utils.AuthorizationDetailsErrorResponse(svcErr)
	}
	return refreshable, nil
}

// modifyGrantedAuthorizationDetails returns the values of the requested subset of the granted details,
// or of the whole grant, as the access token carries them.
func modifyGrantedAuthorizationDetails(ctx context.Context,
	authzDetails providers.AuthorizationDetailProvider, granted []providers.AuthorizationDetail,
	requested string) (json.RawMessage, *model.ErrorResponse) {
	if requested == "" {
		values, err := authzdetailcommon.EncodeDetailValues(granted)
		if err != nil {
			log.GetLogger().With(log.String(log.LoggerKeyComponentName, "GrantHandler")).Error(ctx,
				"Failed to encode the granted authorization details", log.Error(err))
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to generate token",
			}
		}
		return values, nil
	}
	modified, svcErr := authzDetails.ModifyAuthorizationDetails(ctx, granted, requested)
	if svcErr != nil {
		return nil, oauth2utils.AuthorizationDetailsErrorResponse(svcErr)
	}
	return modified, nil
}

// oidcScopeSet returns the subset of scopes that are OIDC scopes for this application, as a set for
// repeated lookup.
func oidcScopeSet(scopes []string, scopeClaims map[string][]string) map[string]struct{} {
	oidcScopes, _ := oauth2utils.SeparateOIDCAndNonOIDCScopes(tokenservice.JoinScopes(scopes), scopeClaims)
	set := make(map[string]struct{}, len(oidcScopes))
	for _, scope := range oidcScopes {
		set[scope] = struct{}{}
	}
	return set
}

// grantPermits allows exact scopes and permission descendants within the carried grant.
// Requested OIDC scopes require exact grants; held OIDC scopes cannot authorize descendants.
func grantPermits(requested string, granted []string, oidcScopes map[string]struct{}, delimiter string) bool {
	if _, isOIDC := oidcScopes[requested]; isOIDC {
		return slices.Contains(granted, requested)
	}
	for _, grant := range granted {
		if grant == requested {
			return true
		}
		if _, isOIDC := oidcScopes[grant]; isOIDC {
			continue
		}
		if security.PermissionCovers(grant, requested, delimiter) {
			return true
		}
	}
	return false
}

// hierarchyDelimiter returns the delimiter a hierarchical resource server's permissions nest on, and
// the empty delimiter, which matches exactly, for any other or for no resource server.
func hierarchyDelimiter(server *providers.ResourceServer) string {
	if server == nil || !server.HierarchicalPermissions {
		return ""
	}
	return server.Delimiter
}

// compactPermissionScopes removes covered permission strings after authorization and consent checks.
func compactPermissionScopes(scopes, oidcScopes []string,
	server *providers.ResourceServer,
) []string {
	if server == nil || !server.HierarchicalPermissions || server.Delimiter == "" {
		return scopes
	}
	result := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		if slices.Contains(oidcScopes, scope) {
			result = append(result, scope)
			continue
		}
		covered := false
		for _, parent := range scopes {
			if slices.Contains(oidcScopes, parent) {
				continue
			}
			if parent != scope && security.PermissionCovers(parent, scope, server.Delimiter) {
				covered = true
				break
			}
		}
		if !covered && !slices.Contains(result, scope) {
			result = append(result, scope)
		}
	}
	return result
}
