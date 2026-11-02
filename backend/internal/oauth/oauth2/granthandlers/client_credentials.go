// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package granthandlers

import (
	"context"
	"encoding/json"
	"slices"

	authzdetailcommon "github.com/thunder-id/thunderid/internal/authzdetail/common"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/resourceindicators"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// clientCredentialsGrantHandler handles the client credentials grant type.
type clientCredentialsGrantHandler struct {
	tokenBuilder    tokenservice.TokenBuilderInterface
	ouService       providers.OrganizationUnitProvider
	authzService    providers.AuthorizationProvider
	actorProvider   providers.ActorProvider
	resourceService providers.ResourceServerProvider
	governance      providers.IdentityGovernanceProvider
	authzDetails    providers.AuthorizationDetailProvider
}

// newClientCredentialsGrantHandler creates a new instance of ClientCredentialsGrantHandler.
func newClientCredentialsGrantHandler(
	tokenBuilder tokenservice.TokenBuilderInterface,
	ouService providers.OrganizationUnitProvider,
	authzService providers.AuthorizationProvider,
	actorProvider providers.ActorProvider,
	resourceService providers.ResourceServerProvider,
	governance providers.IdentityGovernanceProvider,
	authzDetails providers.AuthorizationDetailProvider,
) GrantHandlerInterface {
	return &clientCredentialsGrantHandler{
		tokenBuilder:    tokenBuilder,
		ouService:       ouService,
		authzService:    authzService,
		actorProvider:   actorProvider,
		resourceService: resourceService,
		governance:      governance,
		authzDetails:    authzDetails,
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

	// The client is the resource owner, so its authorization details are processed without consent.
	var details []providers.AuthorizationDetail
	if tokenRequest.AuthorizationDetails != "" {
		var svcErr *tidcommon.ServiceError
		details, svcErr = h.authzDetails.PrepareAuthorizationDetails(ctx,
			json.RawMessage(tokenRequest.AuthorizationDetails))
		if svcErr != nil {
			return nil, oauth2utils.AuthorizationDetailsErrorResponse(svcErr)
		}
	}

	// A client_credentials token carries no OIDC scopes, so every requested scope is a permission
	// scope. Bind the token to a single resource server (RFC 8707 resource or the configured
	// default). A request with no scope, authorization details or resource is not bound to a resource
	// server: its audience is the app's configured default audiences (falling back to the client_id) and
	// it carries no scopes.
	targetRS, errResp := resourceindicators.ResolveAudienceBinding(
		ctx, h.resourceService, tokenRequest.Resources, scopes, len(details) > 0)
	if errResp != nil {
		return nil, errResp
	}

	// The processors are called only for a request that binds to a resource server.
	var authorizationDetails json.RawMessage
	if len(details) > 0 {
		processed, svcErr := h.processAuthorizationDetails(ctx, details, oauthApp, targetRS.Identifier)
		if svcErr != nil {
			return nil, oauth2utils.AuthorizationDetailsErrorResponse(svcErr)
		}
		authorizationDetails = processed
	}

	audiences := []string{oauthApp.ResolveDefaultAudience(tokenRequest.ClientID)}
	if targetRS != nil {
		audiences = []string{targetRS.Identifier}

		// Downscope requested scopes to permissions defined on the target resource server.
		scopes, errResp = resourceindicators.DownscopeToResourceServer(ctx, h.resourceService, targetRS.ID, scopes)
		if errResp != nil {
			return nil, errResp
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

			// The policy engine decides only the scopes. The details the processors approved are passed as
			// context, so a scope is not granted on the strength of a detail the token does not carry.
			var evaluationContext map[string]interface{}
			if len(authorizationDetails) > 0 {
				evaluationContext = map[string]interface{}{
					providers.AccessEvaluationContextAuthorizationDetails: authorizationDetails}
			}
			evaluationsRequest := tokenservice.BuildAccessEvaluationsRequest(
				oauthApp.ID, oauthApp.EntityCategory.String(), groupIDs, nil, scopes, targetRS.ID, evaluationContext)
			// Only the roles the application holds in its own organization unit count, not those an
			// organization unit it is shared with assigned it, even when the token is for that unit.
			for i := range evaluationsRequest.Evaluations {
				evaluationsRequest.Evaluations[i].OUID = oauthApp.OUID
			}
			authzResp, svcErr := h.authzService.EvaluateAccessBatch(ctx, evaluationsRequest)
			if svcErr != nil {
				logger.Error(ctx, "Failed to get authorized permissions for app",
					log.String("appID", oauthApp.ID), log.String("error", svcErr.Error.DefaultValue))
				return nil, &model.ErrorResponse{
					Error:            constants.ErrorServerError,
					ErrorDescription: "Failed to generate token",
				}
			}

			scopes = tokenservice.FilterAuthorizedScopes(scopes, authzResp.Evaluations)
		}
	}

	clientAttributes, clientAttrErr := tokenservice.BuildClientAttributes(oauthApp, h.actorProvider)
	if clientAttrErr != nil {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to generate token",
		}
	}

	// The organization claims are resolved apart from the rest and handed to the builder in a field
	// of their own.
	ouAttributes, ouAttrErr := tokenservice.BuildClientEffectiveOUAttributes(ctx, oauthApp, h.ouService)
	if ouAttrErr != nil {
		logger.Error(ctx, "Failed to resolve organization unit claims",
			log.String("appID", oauthApp.ID), log.Error(ouAttrErr))
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to generate token",
		}
	}

	accessToken, err := h.tokenBuilder.BuildAccessToken(ctx, &tokenservice.AccessTokenBuildContext{
		Subject:              oauthApp.ID,
		Audiences:            audiences,
		ClientID:             tokenRequest.ClientID,
		Scopes:               scopes,
		SubjectAttributes:    clientAttributes,
		OUAttributes:         ouAttributes,
		GrantType:            string(providers.GrantTypeClientCredentials),
		OAuthApp:             oauthApp,
		ValidityPeriod:       oauthApp.ClientAccessTokenConfig().ValidityPeriodOrZero(),
		DPoPJkt:              dpop.GetJkt(ctx),
		AuthorizationDetails: authorizationDetails,
	})
	if err != nil {
		return nil, tokenBuildErrorResponse(ctx, logger, err, "Failed to generate token")
	}

	// The client was granted access, so its last login is stamped. A failed stamp is logged by the
	// authority and never fails the response.
	if h.governance != nil {
		_ = h.governance.RecordLogin(ctx, oauthApp.ID)
	}

	return &model.TokenResponseDTO{
		AccessToken: *accessToken,
	}, nil
}

// processAuthorizationDetails has the authorization detail provider decide the prepared details with the client
// itself as the subject, and returns the admitted details as the token carries them.
func (h *clientCredentialsGrantHandler) processAuthorizationDetails(ctx context.Context,
	details []providers.AuthorizationDetail, client *providers.OAuthClient,
	resourceServer string) (json.RawMessage, *tidcommon.ServiceError) {
	subject := providers.EntityReference{EntityID: client.ID, EntityCategory: string(client.EntityCategory),
		OUID: client.OUID}
	admitted, svcErr := h.authzDetails.ProcessAuthorizationDetails(ctx, details, subject, client.ClientID, subject,
		resourceServer)
	if svcErr != nil {
		return nil, svcErr
	}
	encoded, err := authzdetailcommon.EncodeDetailValues(admitted)
	if err != nil {
		log.GetLogger().Error(ctx, "Failed to encode authorization details", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return encoded, nil
}
