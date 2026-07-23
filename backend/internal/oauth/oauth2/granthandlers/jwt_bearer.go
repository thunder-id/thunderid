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

package granthandlers

import (
	"context"
	"slices"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/resourceindicators"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// jwtBearerGrantHandler handles the jwt-bearer grant type used to present an ID-JAG assertion
// (draft-ietf-oauth-identity-assertion-authz-grant) issued by a trusted external IdP.
type jwtBearerGrantHandler struct {
	tokenBuilder        tokenservice.TokenBuilderInterface
	tokenValidator      tokenservice.TokenValidatorInterface
	resourceService     providers.ResourceServerProvider
	serverConfigService serverconfig.ServerConfigService
}

// newJWTBearerGrantHandler creates a new instance of jwtBearerGrantHandler.
func newJWTBearerGrantHandler(
	tokenBuilder tokenservice.TokenBuilderInterface,
	tokenValidator tokenservice.TokenValidatorInterface,
	resourceService providers.ResourceServerProvider,
	serverConfigService serverconfig.ServerConfigService,
) GrantHandlerInterface {
	return &jwtBearerGrantHandler{
		tokenBuilder:        tokenBuilder,
		tokenValidator:      tokenValidator,
		resourceService:     resourceService,
		serverConfigService: serverConfigService,
	}
}

// ValidateGrant validates the jwt-bearer grant type request.
func (h *jwtBearerGrantHandler) ValidateGrant(ctx context.Context, tokenRequest *model.TokenRequest,
	oauthApp *providers.OAuthClient) *model.ErrorResponse {
	if providers.GrantType(tokenRequest.GrantType) != providers.GrantTypeJWTBearer {
		return &model.ErrorResponse{
			Error:            constants.ErrorUnsupportedGrantType,
			ErrorDescription: "Unsupported grant type",
		}
	}

	if tokenRequest.Assertion == "" {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "Missing required parameter: assertion",
		}
	}

	// An ID-JAG is a bearer assertion whose only anti-theft protection is its client_id binding, which
	// is worthless for a public client. Restrict the grant to confidential clients.
	if oauthApp.TokenEndpointAuthMethod == providers.TokenEndpointAuthMethodNone {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidClient,
			ErrorDescription: "The jwt-bearer grant requires a confidential client",
		}
	}

	return nil
}

// HandleGrant handles the jwt-bearer grant type. It validates an ID-JAG assertion issued by a trusted
// external IdP, binds it to the authenticated client, and issues a ThunderID access token whose
// subject is the assertion's subject. No refresh token is issued. The inbound ID-JAG assertion is not
// sender-constrained (DPoP binding of the assertion is out of scope), but the issued access token is
// DPoP-bound when the request carries a DPoP proof, like every other grant.
func (h *jwtBearerGrantHandler) HandleGrant(ctx context.Context, tokenRequest *model.TokenRequest,
	oauthApp *providers.OAuthClient) (
	*model.TokenResponseDTO, *model.ErrorResponse) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "JWTBearerGrantHandler"))

	assertionClaims, err := h.tokenValidator.ValidateIDJAGAssertion(
		ctx, tokenRequest.Assertion, tokenRequest.ClientID)
	if err != nil {
		logger.Debug(ctx, "Failed to validate ID-JAG assertion", log.Error(err))
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "Invalid assertion",
		}
	}

	// Granted scopes start from the assertion's scope claim, narrowed by the request scope parameter
	// when present. The app's registered scopes are intentionally NOT intersected here: no other grant
	// enforces oauthApp.Scopes (the scope validator is a passthrough), and resource-server-scoped
	// narrowing below (when a resource claim is present) is the correct authorization boundary. Per-app
	// resource authorization is expected to be handled by app-resource subscription once implemented.
	grantedScopes := assertionClaims.Scopes
	if tokenRequest.Scope != "" {
		grantedScopes = intersectScopes(grantedScopes, tokenservice.ParseScopes(tokenRequest.Scope))
	}

	// The issued access token is bound to at most one resource server (RFC 8707). A resource
	// parameter on the request may narrow the assertion's resource claim but must not widen it; an
	// assertion that authorizes more than one resource requires the request to select one. When
	// permission scopes are present and neither the assertion nor the request carries a resource,
	// the configured defaultResourceServer is used, and if none is configured the request is rejected
	// with invalid_target. OIDC-only no-resource requests are not resource-server bound.
	resources := assertionClaims.Resources
	if len(tokenRequest.Resources) > 0 {
		if len(assertionClaims.Resources) > 0 {
			for _, r := range tokenRequest.Resources {
				if !slices.Contains(assertionClaims.Resources, r) {
					return nil, &model.ErrorResponse{
						Error: constants.ErrorInvalidTarget,
						ErrorDescription: "The resource parameter must be a subset of the " +
							"assertion's resource claim",
					}
				}
			}
		}
		resources = tokenRequest.Resources
	}

	// Retain OIDC scopes; downscope permission scopes to those defined on the target resource server.
	oidcScopes, permissionScopes := oauth2utils.SeparateOIDCAndNonOIDCScopes(
		tokenservice.JoinScopes(grantedScopes), oauthApp.ScopeClaims)
	targetRS, errResp := resourceindicators.ResolveAudienceBinding(
		ctx, h.resourceService, h.serverConfigService, resources, permissionScopes)
	if errResp != nil {
		return nil, errResp
	}

	var audiences []string
	if targetRS == nil {
		// OIDC-only assertion with no resource: the token is not bound to a resource server, so its
		// audience is the app's configured default audiences (falling back to the client_id) and it
		// carries only the OIDC scopes.
		audiences = []string{oauthApp.ResolveDefaultAudience(tokenRequest.ClientID)}
		grantedScopes = oidcScopes
	} else {
		permissionScopes, errResp = resourceindicators.DownscopeToResourceServer(
			ctx, h.resourceService, targetRS.ID, permissionScopes)
		if errResp != nil {
			return nil, errResp
		}
		grantedScopes = make([]string, 0, len(oidcScopes)+len(permissionScopes))
		grantedScopes = append(grantedScopes, oidcScopes...)
		grantedScopes = append(grantedScopes, permissionScopes...)
		audiences = []string{targetRS.Identifier}
	}

	// The subject is the external IdP's identifier carried in the assertion; no local user resolution
	// or attribute mapping is performed in v1. The access token carries the source IdP as the `idp`
	// claim so that this external `sub` is not mistaken for a local user id by downstream consumers.
	accessToken, err := h.tokenBuilder.BuildAccessToken(ctx, &tokenservice.AccessTokenBuildContext{
		Subject:           assertionClaims.Sub,
		Audiences:         audiences,
		ClientID:          tokenRequest.ClientID,
		Scopes:            grantedScopes,
		SubjectAttributes: make(map[string]interface{}),
		GrantType:         string(providers.GrantTypeJWTBearer),
		OAuthApp:          oauthApp,
		SourceIDP:         assertionClaims.Iss,
		DPoPJkt:           dpop.GetJkt(ctx),
	})
	if err != nil {
		logger.Error(ctx, "Failed to generate token", log.Error(err))
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to generate token",
		}
	}

	return &model.TokenResponseDTO{
		AccessToken: *accessToken,
	}, nil
}

// intersectScopes returns the scopes present in both a and b, preserving the order of a.
func intersectScopes(a, b []string) []string {
	if len(a) == 0 || len(b) == 0 {
		return []string{}
	}
	set := make(map[string]struct{}, len(b))
	for _, s := range b {
		set[s] = struct{}{}
	}
	out := make([]string, 0, len(a))
	for _, s := range a {
		if _, ok := set[s]; ok {
			out = append(out, s)
		}
	}
	return out
}
