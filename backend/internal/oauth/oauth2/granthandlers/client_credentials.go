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

package granthandlers

import (
	"context"
	"slices"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/resourceindicators"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// clientCredentialsGrantHandler handles the client credentials grant type.
type clientCredentialsGrantHandler struct {
	tokenBuilder    tokenservice.TokenBuilderInterface
	ouService       providers.OrganizationUnitProvider
	authzService    providers.AuthorizationProvider
	actorProvider   providers.ActorProvider
	resourceService providers.ResourceServerProvider
}

// newClientCredentialsGrantHandler creates a new instance of ClientCredentialsGrantHandler.
func newClientCredentialsGrantHandler(
	tokenBuilder tokenservice.TokenBuilderInterface,
	ouService providers.OrganizationUnitProvider,
	authzService providers.AuthorizationProvider,
	actorProvider providers.ActorProvider,
	resourceService providers.ResourceServerProvider,
) GrantHandlerInterface {
	return &clientCredentialsGrantHandler{
		tokenBuilder:    tokenBuilder,
		ouService:       ouService,
		authzService:    authzService,
		actorProvider:   actorProvider,
		resourceService: resourceService,
	}
}

// ValidateGrant validates the client credentials grant type.
func (h *clientCredentialsGrantHandler) ValidateGrant(ctx context.Context, tokenRequest *model.TokenRequest,
	oauthApp *providers.OAuthClient) *model.ErrorResponse {
	if providers.GrantType(tokenRequest.GrantType) != providers.GrantTypeClientCredentials {
		return &model.ErrorResponse{
			Error:            constants.ErrorUnsupportedGrantType,
			ErrorDescription: "Unsupported grant type",
		}
	}

	if errResp := resourceindicators.ValidateResourceURIs(tokenRequest.Resources); errResp != nil {
		return errResp
	}

	return nil
}

// HandleGrant handles the client credentials grant type.
func (h *clientCredentialsGrantHandler) HandleGrant(ctx context.Context, tokenRequest *model.TokenRequest,
	oauthApp *providers.OAuthClient) (
	*model.TokenResponseDTO, *model.ErrorResponse) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ClientCredentialsGrantHandler"))

	scopes := tokenservice.ParseScopes(tokenRequest.Scope)
	hasResourceParam := len(tokenRequest.Resources) > 0

	// Resolve each requested resource identifier to an internal Resource Server.
	// Unknown identifiers cause a 400 invalid_target.
	resolvedRSes, errResp := resourceindicators.ResolveResourceServers(ctx, h.resourceService, tokenRequest.Resources)
	if errResp != nil {
		return nil, errResp
	}

	// Per-RS valid scopes (intersection of requested scopes with the RS's defined permissions).
	// Scopes not defined on any requested RS are silently dropped.
	rsValidScopes, errResp := resourceindicators.ComputeRSValidScopes(ctx, h.resourceService, resolvedRSes, scopes)
	if errResp != nil {
		return nil, errResp
	}

	if hasResourceParam {
		scopes = resourceindicators.UnionScopes(rsValidScopes)
	}

	if len(scopes) > 0 {
		var groupIDs []string
		if h.actorProvider != nil {
			groups, groupErr := h.actorProvider.GetActorGroups(oauthApp.ID)
			if groupErr != nil {
				logger.Error(ctx, "Failed to resolve app group memberships",
					log.String("appID", oauthApp.ID), log.String("error", groupErr.Error.DefaultValue))
				return nil, &model.ErrorResponse{
					Error:            constants.ErrorServerError,
					ErrorDescription: "Failed to generate token",
				}
			} else {
				for _, group := range groups {
					if group.ID != "" && !slices.Contains(groupIDs, group.ID) {
						groupIDs = append(groupIDs, group.ID)
					}
				}
			}
		}

		authzResp, svcErr := h.authzService.EvaluateAccessBatch(ctx,
			buildAccessEvaluationsRequest(oauthApp.ID, groupIDs, scopes))
		if svcErr != nil {
			logger.Error(ctx, "Failed to get authorized permissions for app",
				log.String("appID", oauthApp.ID), log.String("error", svcErr.Error.DefaultValue))
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to generate token",
			}
		}

		scopes = filterAuthorizedScopes(scopes, authzResp.Evaluations)
	}

	// aud is composed by resourceindicators.ComposeAudiences: RS identifiers when any RS contributes
	// (explicit resource params or implicit discovery via granted scopes), else clientID fallback.
	audiences, errResp := resourceindicators.ComposeAudiences(ctx, h.resourceService, tokenRequest.ClientID,
		resolvedRSes, scopes)
	if errResp != nil {
		return nil, errResp
	}

	clientAttributes, clientAttrErr := tokenservice.BuildClientAttributes(ctx, oauthApp, h.ouService, h.actorProvider)
	if clientAttrErr != nil {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to generate token",
		}
	}

	accessToken, err := h.tokenBuilder.BuildAccessToken(ctx, &tokenservice.AccessTokenBuildContext{
		Subject:           oauthApp.ID,
		Audiences:         audiences,
		ClientID:          tokenRequest.ClientID,
		Scopes:            scopes,
		SubjectAttributes: clientAttributes,
		GrantType:         string(providers.GrantTypeClientCredentials),
		OAuthApp:          oauthApp,
		ValidityPeriod:    oauthApp.ClientAccessTokenConfig().ValidityPeriodOrZero(),
		DPoPJkt:           dpop.GetJkt(ctx),
	})
	if err != nil {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to generate token",
		}
	}

	return &model.TokenResponseDTO{
		AccessToken: *accessToken,
	}, nil
}

func buildAccessEvaluationsRequest(
	entityID string,
	groupIDs []string,
	permissions []string,
) providers.AccessEvaluationsRequest {
	evaluations := make([]providers.AccessEvaluationRequest, 0, len(permissions))
	for _, permission := range permissions {
		evaluations = append(evaluations, providers.AccessEvaluationRequest{
			Subject: providers.Subject{
				ID:       entityID,
				GroupIDs: groupIDs,
			},
			Permission: providers.Permission{Name: permission},
		})
	}
	return providers.AccessEvaluationsRequest{Evaluations: evaluations}
}

func filterAuthorizedScopes(scopes []string, evaluations []providers.AccessEvaluationResponse) []string {
	authorizedScopes := make([]string, 0, len(evaluations))
	for i, evaluation := range evaluations {
		if evaluation.Decision && i < len(scopes) {
			authorizedScopes = append(authorizedScopes, scopes[i])
		}
	}
	return authorizedScopes
}
